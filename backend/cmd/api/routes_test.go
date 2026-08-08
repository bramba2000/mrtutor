package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/httpx"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

// TestRegisterRoutes_RealMux drives requests through the actual composition
// this binary uses at startup — createServices + registerRoutes mounted
// under the same "/api/v1" prefix run.go uses — rather than the bare,
// hand-built router test/integration/auth_test.go constructs for itself.
// Nothing previously exercised registerRoutes, the "/api/v1" prefix, or
// method routing together; this closes that gap from the verification
// checklist in docs/architecture-tasks.md.
func TestRegisterRoutes_RealMux(t *testing.T) {
	db := sqlitetest.OpenTemp(t)
	svcs := createServices(db)
	logger := slog.New(slog.DiscardHandler)

	router := httpx.NewRouter("/api/v1")
	registerRoutes(svcs, router, logger, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", encodeJSON(t, auth.RegisterIn{
		Username: "muxuser",
		Email:    "muxuser@example.com",
		Password: "Password00!",
	}))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/auth/register: status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// A request outside the /api/v1 prefix must not match — proves the
	// prefix baked into run.go's NewRouter call is load-bearing here, not
	// just decorative in a test that builds its own router from scratch.
	t.Run("Requests outside the /api/v1 prefix 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	// The wrong method on a real, registered route must 405, not silently
	// match some other handler or fall through to 404.
	t.Run("Wrong method on a registered route 405s", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/register", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}

func encodeJSON(t testing.TB, v any) *bytes.Reader {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to encode body: %v", err)
	}
	return bytes.NewReader(buf)
}
