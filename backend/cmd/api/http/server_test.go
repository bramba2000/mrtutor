package http_test

import (
	"context"
	"log/slog"
	"net"
	httpstdlib "net/http"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/cmd/api/http"
	"golang.org/x/net/nettest"
)

func TestServer_Run(t *testing.T) {
	blockedAddr := "127.0.0.1:6666"
	blocker, err := net.Listen("tcp", blockedAddr)
	if err != nil {
		t.Fatalf("Failed to listen on %s: %v", blockedAddr, err)
	}
	defer blocker.Close()

	tt := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "Address already in use",
			address: blockedAddr,
			wantErr: true,
		},
		{
			name:    "Valid address",
			address: "127.0.0.1:7777",
			wantErr: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var shutdownChan chan struct{} = make(chan struct{})
			var errChan chan error = make(chan error, 1)
			srv := http.NewServer(http.Config{
				Address:         tc.address,
				Logger:          slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
				LogLevel:        slog.LevelDebug,
				ShutdownTimeout: 5 * time.Second,
				OnShuttingDown:  func() { close(shutdownChan) },
			})
			ctx, cancel := context.WithCancel(t.Context())
			go func() {
				errChan <- srv.Run(ctx)
			}()

			cancel() // Ensure we cancel the context to avoid goroutine leaks

			var runErr error
			select {
			case <-shutdownChan:
				select {
				case runErr = <-errChan:
				case <-t.Context().Done():
					t.Fatalf("Test timed out waiting for server to return an error")
				}
			case runErr = <-errChan:
			case <-t.Context().Done():
				t.Fatalf("Test timed out waiting for server to shut down")
			}
			if tc.wantErr != (runErr != nil) {
				t.Errorf("Expected error: %v, got: %v", tc.wantErr, runErr)
			}
		})
	}

}

// waitServerReady blocks until either address accepts connections or the
// server has already exited with a result on errChan (e.g. because the
// listener was invalid to begin with). It returns the early error, if any,
// and whether one was observed.
func waitServerReady(t *testing.T, ctx context.Context, address string, errChan <-chan error) (earlyErr error, exited bool) {
	t.Helper()
	var readyChan = make(chan struct{})
	go func() {
		dialer := net.Dialer{}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				conn.Close()
				close(readyChan)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	select {
	case <-readyChan:
		return nil, false
	case err := <-errChan:
		return err, true
	case <-t.Context().Done():
		t.Fatalf("Server did not become ready in time")
		return nil, false
	}
}

func TestServer_Serve(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listner func(t *testing.T) net.Listener
		wantErr bool
	}{
		{
			name: "Successful start when valid listener is provided",
			listner: func(t *testing.T) net.Listener {
				ln, err := nettest.NewLocalListener("tcp")
				if err != nil {
					t.Fatalf("Failed to create local listener: %v", err)
				}
				return ln
			},
			wantErr: false,
		},
		{
			name: "Error when listener is closed",
			listner: func(t *testing.T) net.Listener {
				ln, err := nettest.NewLocalListener("tcp")
				if err != nil {
					t.Fatalf("Failed to create local listener: %v", err)
				}
				ln.Close() // Close the listener to simulate an error
				return ln
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var shutdownChan chan struct{} = make(chan struct{})
			var ln net.Listener = tc.listner(t)
			srv := http.NewServer(http.Config{
				Address:         ln.Addr().String(),
				Logger:          slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
				LogLevel:        slog.LevelDebug,
				ShutdownTimeout: 5 * time.Second,
				OnShuttingDown:  func() { shutdownChan <- struct{}{} },
			})

			var ctx, cancel = context.WithCancel(t.Context())
			defer cancel()

			var errChan chan error = make(chan error, 1)
			go func() {
				errChan <- srv.Serve(ctx, ln)
			}()

			if earlyErr, exited := waitServerReady(t, ctx, ln.Addr().String(), errChan); exited {
				if tc.wantErr != (earlyErr != nil) {
					t.Errorf("Serve() error = %v, wantErr %v", earlyErr, tc.wantErr)
				}
				return
			}
			cancel() // stop gracefully

			var err error
			select {
			case <-shutdownChan:
				select {
				case err = <-errChan:
				case <-t.Context().Done():
					t.Fatalf("Test timed out waiting for server to return an error")
				}
			case err = <-errChan:
			case <-t.Context().Done():
				t.Fatalf("Test timed out waiting for server to shut down")
			}
			if tc.wantErr != (err != nil) {
				t.Errorf("Serve() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}

	t.Run("Drains an in-flight request before shutting down", func(t *testing.T) {
		ln, err := nettest.NewLocalListener("tcp")
		if err != nil {
			t.Fatalf("Failed to create local listener: %v", err)
		}

		const (
			shutdownTimeout = 500 * time.Millisecond
			handlerDelay    = 100 * time.Millisecond
		)
		handlerStarted := make(chan struct{})
		srv := http.NewServer(http.Config{
			Address:         ln.Addr().String(),
			Logger:          slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
			LogLevel:        slog.LevelDebug,
			ShutdownTimeout: shutdownTimeout,
			Handler: httpstdlib.HandlerFunc(func(w httpstdlib.ResponseWriter, r *httpstdlib.Request) {
				close(handlerStarted)
				// Finishes well within shutdownTimeout, so a graceful drain should let it complete.
				time.Sleep(handlerDelay)
				w.WriteHeader(httpstdlib.StatusOK)
			}),
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		errChan := make(chan error, 1)
		go func() { errChan <- srv.Serve(ctx, ln) }()

		if _, exited := waitServerReady(t, ctx, ln.Addr().String(), errChan); exited {
			t.Fatalf("Server exited before becoming ready")
		}

		type result struct {
			status int
			err    error
		}
		resultChan := make(chan result, 1)
		go func() {
			resp, err := httpstdlib.Get("http://" + ln.Addr().String())
			if err != nil {
				resultChan <- result{err: err}
				return
			}
			defer resp.Body.Close()
			resultChan <- result{status: resp.StatusCode}
		}()
		select {
		case <-handlerStarted:
		case <-t.Context().Done():
			t.Fatalf("Test timed out waiting for the in-flight request to start")
		}

		cancel() // stop gracefully; the in-flight request should be allowed to finish

		select {
		case err = <-errChan:
		case <-t.Context().Done():
			t.Fatalf("Test timed out waiting for server to shut down")
		}
		if err != nil {
			t.Fatalf("Serve() error = %v, want nil for a graceful drain", err)
		}

		select {
		case res := <-resultChan:
			if res.err != nil {
				t.Fatalf("In-flight request failed: %v", res.err)
			}
			if res.status != httpstdlib.StatusOK {
				t.Errorf("In-flight request status = %d, want %d", res.status, httpstdlib.StatusOK)
			}
		case <-t.Context().Done():
			t.Fatalf("Test timed out waiting for the in-flight request to complete")
		}
	})

	t.Run("Force closed when shutdown timeout", func(t *testing.T) {
		ln, err := nettest.NewLocalListener("tcp")
		if err != nil {
			t.Fatalf("Failed to create local listener: %v", err)
		}

		const shutdownTimeout = 100 * time.Millisecond
		handlerStarted := make(chan struct{})
		srv := http.NewServer(http.Config{
			Address:         ln.Addr().String(),
			Logger:          slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
			LogLevel:        slog.LevelDebug,
			ShutdownTimeout: shutdownTimeout,
			Handler: httpstdlib.HandlerFunc(func(w httpstdlib.ResponseWriter, r *httpstdlib.Request) {
				close(handlerStarted)
				// Outlives shutdownTimeout so the graceful drain can never finish in time.
				time.Sleep(10 * shutdownTimeout)
			}),
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		errChan := make(chan error, 1)
		go func() { errChan <- srv.Serve(ctx, ln) }()

		if _, exited := waitServerReady(t, ctx, ln.Addr().String(), errChan); exited {
			t.Fatalf("Server exited before becoming ready")
		}

		// Fire a request in the background and wait for it to actually reach the
		// handler before triggering shutdown, so the connection is genuinely
		// in-flight (not a race against a request that hasn't been sent yet).
		go func() {
			resp, err := httpstdlib.Get("http://" + ln.Addr().String())
			if err == nil {
				resp.Body.Close()
			}
		}()
		select {
		case <-handlerStarted:
		case <-t.Context().Done():
			t.Fatalf("Test timed out waiting for the in-flight request to start")
		}

		cancel() // stop gracefully; the hung request should force the shutdown timeout

		select {
		case err = <-errChan:
		case <-t.Context().Done():
			t.Fatalf("Test timed out waiting for server to return an error")
		}
		if err == nil {
			t.Fatalf("Serve() error = nil, want an error from a forced close after the shutdown timeout")
		}
	})
}
