package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/features/auth"
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

// TestNewHandler drives the real composition newHandler builds: the API
// under /api/v1 and the embedded SPA at everything else, on the single
// http.ServeMux run.go actually serves. It documents a deliberate behaviour
// change from before this test existed: an unmatched non-/api/ path no
// longer 404s — it now returns the SPA's index.html, since the router can no
// longer tell a missing resource from a client-side route.
//
// Whether backend/web/dist holds a real frontend build or just the committed
// .gitkeep placeholder is ambient state this test doesn't control — task
// build populates it, a fresh clone doesn't — so the root-fallback assertion
// below branches on newHandler's own error return rather than assuming one
// or the other.
func TestNewHandler(t *testing.T) {
	db := sqlitetest.OpenTemp(t)
	svcs := createServices(db)
	logger := slog.New(slog.DiscardHandler)
	readiness := &httpx.Readiness{}

	handler, err := newHandler(svcs, readiness, logger, config.Config{})

	t.Run("API routes still work under /api/v1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", encodeJSON(t, auth.RegisterIn{
			Username: "handleruser",
			Email:    "handleruser@example.com",
			Password: "Password00!",
		}))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("POST /api/v1/auth/register: status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
		}
	})

	t.Run("unknown API version 404s, not the SPA", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v99/whatever", nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if ct := w.Header().Get("Content-Type"); ct == "text/html" || ct == "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, an unknown API version must not be answered with HTML", ct)
		}
	})

	t.Run("root falls through to the SPA handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		// web.ErrNotBuilt means no frontend build exists in this checkout
		// (a fresh clone, before "task build" ever ran): the SPA handler
		// answers 503 to everything. Otherwise a real build is present and
		// root must serve it.
		want := http.StatusOK
		if err != nil {
			want = http.StatusServiceUnavailable
		}
		if w.Code != want {
			t.Errorf("status = %d, want %d (err = %v)", w.Code, want, err)
		}
	})

	t.Run("readiness is still reachable under /api/v1/healthz", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
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
