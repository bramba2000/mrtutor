package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Address  string
	Handler  http.Handler
	Logger   *slog.Logger
	LogLevel slog.Level

	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	ShutdownTimeout time.Duration
	DrainPeriod     time.Duration
	OnShuttingDown  func()
}

type Server struct {
	server          *http.Server
	cancelBaseCtx   context.CancelFunc
	logger          *slog.Logger
	shutdownTimeout time.Duration
	drainPeriod     time.Duration
	onShuttingDown  func()
}

func NewServer(cfg Config) *Server {
	baseCtx, cancelBaseCtx := context.WithCancel(context.Background())
	logger := cfg.Logger.With("component", "server")

	drainPeriod := cfg.DrainPeriod
	if drainPeriod >= cfg.ShutdownTimeout {
		logger.Warn("drain period exceeds shutdown timeout, clamping to make room for shutdown",
			"drainPeriod", drainPeriod, "shutdownTimeout", cfg.ShutdownTimeout)
		drainPeriod = cfg.ShutdownTimeout / 2
	}

	return &Server{
		server: &http.Server{
			Addr:              cfg.Address,
			Handler:           cfg.Handler,
			BaseContext:       func(_ net.Listener) context.Context { return baseCtx },
			ErrorLog:          slog.NewLogLogger(logger.Handler(), cfg.LogLevel),
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		cancelBaseCtx:   cancelBaseCtx,
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
		drainPeriod:     drainPeriod,
		onShuttingDown:  cfg.OnShuttingDown,
	}
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.server.Addr, err)
	}
	return s.Serve(ctx, ln)
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	defer s.cancelBaseCtx()
	s.logger.Info("server listening", "address", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}

	shutdownStart := time.Now()
	s.logger.Info("shutdown signal received, draining", "drainPeriod", s.drainPeriod)
	if s.onShuttingDown != nil {
		s.onShuttingDown()
	}

	// The listener stays open for the whole drain period, so a load balancer
	// or Kubernetes readiness probe has time to observe onShuttingDown's 503
	// and stop routing new traffic before Shutdown closes the listener. This
	// deliberately does not select on ctx: ctx is already cancelled (that's
	// why we're here), so waiting on it again would return immediately and
	// skip the drain entirely.
	if s.drainPeriod > 0 {
		drainTimer := time.NewTimer(s.drainPeriod)
		defer drainTimer.Stop()
		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("server error: %w", err)
		case <-drainTimer.C:
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout-time.Since(shutdownStart))
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Error("graceful shutdown timed out, forcing close", "error", err)
		if closeErr := s.server.Close(); closeErr != nil {
			return errors.Join(
				fmt.Errorf("graceful shutdown: %w", err),
				fmt.Errorf("force close: %w", closeErr),
			)
		}
		return fmt.Errorf("graceful shutdown exceeded timeout: %w", err)
	}
	return nil
}
