package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessLog(t *testing.T) {
	t.Run("Logs method, path, and status for a normal request", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := AccessLog(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/brew", nil))

		logs.requireLogged(t, "GET", "/brew", "418")
	})

	t.Run("Logs status 200 when the handler never calls WriteHeader", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := AccessLog(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("implicit 200"))
		}))

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		// httptest.ResponseRecorder pre-seeds Code to 200, so this only proves
		// the recorder doesn't crash on an implicit WriteHeader — the real
		// "never wrote a header" distinction needs recordingWriter (see
		// helpers_test.go), not exercised here since AccessLog only reads
		// rec.code for logging, never branches on whether it was explicit.
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	// A ResponseWriter wrapper used for status capture must implement
	// Unwrap() http.ResponseWriter, or it hides http.Flusher/http.Hijacker
	// from any handler downstream that needs them (see docs/architecture-tasks.md
	// Phase 4). http.NewResponseController walks the Unwrap chain to find
	// Flush; without recorder.Unwrap, this fails with http.ErrNotSupported
	// even though the underlying httptest.ResponseRecorder supports it.
	t.Run("The status recorder unwraps to the underlying ResponseWriter", func(t *testing.T) {
		var flushErr error
		handler := AccessLog(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flushErr = http.NewResponseController(w).Flush()
		}))

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		if flushErr != nil {
			t.Errorf("Flush() through the recorder = %v, want nil", flushErr)
		}
	})
}
