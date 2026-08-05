package httpx

import (
	"net/http"
	"sync/atomic"
)

// Readiness reports whether the server is ready to accept traffic.
// Zero value is ready: can accept traffic. Do not copy after first use
type Readiness struct {
	draining atomic.Bool
}

// Ready returns true if the server is ready to accept traffic.
func (r *Readiness) Ready() bool {
	return !r.draining.Load()
}

// Shutdown marks the server as not ready to accept traffic.
func (r *Readiness) Shutdown() {
	r.draining.Store(true)
}

// Handler returns an HTTP handler that reports the readiness of the server.
func (r *Readiness) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !r.Ready() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("OK"))
	})
}

func Liveness() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})
}
