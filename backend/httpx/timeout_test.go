package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	t.Run("Attaches a deadline to the request context", func(t *testing.T) {
		var deadlineSet bool
		handler := Timeout(time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, deadlineSet = r.Context().Deadline()
		}))

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		if !deadlineSet {
			t.Error("expected the request context to carry a deadline")
		}
	})

	t.Run("Context is cancelled once the timeout elapses", func(t *testing.T) {
		var ctxErr error
		handler := Timeout(time.Microsecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			ctxErr = r.Context().Err()
		}))

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		if !errors.Is(ctxErr, context.DeadlineExceeded) {
			t.Errorf("ctx.Err() = %v, want context.DeadlineExceeded", ctxErr)
		}
	})

	t.Run("Zero or negative duration disables the middleware", func(t *testing.T) {
		for _, d := range []time.Duration{0, -time.Second} {
			var deadlineSet bool
			handler := Timeout(d)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, deadlineSet = r.Context().Deadline()
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			if deadlineSet {
				t.Errorf("Timeout(%v): expected no deadline, got one", d)
			}
		}
	})
}
