package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadiness(t *testing.T) {
	t.Run("Ready returns true and the handler returns 200 for the zero value", func(t *testing.T) {
		var r Readiness

		if !r.Ready() {
			t.Error("expected the zero value to be ready")
		}

		w := httptest.NewRecorder()
		r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("Ready returns false and the handler returns 503 after Shutdown", func(t *testing.T) {
		var r Readiness
		r.Shutdown()

		if r.Ready() {
			t.Error("expected Ready to be false after Shutdown")
		}

		w := httptest.NewRecorder()
		r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
		}
	})
}

func TestLiveness(t *testing.T) {
	w := httptest.NewRecorder()
	Liveness().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
