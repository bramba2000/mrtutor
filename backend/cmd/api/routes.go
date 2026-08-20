package main

import (
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/students/studentshttp"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorshttp"
	"github.com/bramba2000/mrtutor/backend/httpx"
	"github.com/bramba2000/mrtutor/backend/web"
)

func registerRoutes(services Services, router *httpx.Router, logger *slog.Logger, cfg config.Config) {
	handlers := []interface {
		Mount(*httpx.Router)
	}{
		authhttp.NewHandler(services.Auth, authhttp.Config{Secure: cfg.AppMode == config.AppModeProd}, logger),
		studentshttp.NewHandler(services.Students, services.Auth, logger),
		tutorshttp.NewHandler(services.Tutors, services.Auth, logger),
	}

	for _, h := range handlers {
		h.Mount(router)
	}
}

// newHandler builds the binary's complete HTTP surface: the JSON API under
// /api/v1 and the embedded SPA at /. httpx.Router is left untouched — it
// cannot express an unprefixed route, and editing httpx/ for a single
// feature is the layout smell docs/architecture-review.md's scaling gate
// warns about — so a plain http.ServeMux is composed above it instead.
//
// It returns web.ErrNotBuilt (with a still-usable handler) when the frontend
// has not been built, leaving the fatal-in-production/warn-in-development
// call to run.
func newHandler(services Services, readiness *httpx.Readiness, logger *slog.Logger, cfg config.Config) (http.Handler, error) {
	api := httpx.NewRouter("/api/v1", httpx.RequestID(), httpx.AccessLog(logger), httpx.Recover(logger), httpx.MaxBytes(), httpx.Timeout(cfg.Server.RequestTimeout))
	registerRoutes(services, api, logger, cfg)
	api.Handle("/healthz", readiness.Handler())
	api.Handle("/livez", httpx.Liveness())

	spa, err := web.Handler(logger)

	mux := http.NewServeMux()
	// "/api/", not "/api/v1/": an unknown API version must 404 from the API
	// router, not be swallowed by the SPA's catch-all fallback below.
	mux.Handle("/api/", api)
	// MaxBytes and Timeout are deliberately omitted here — a request-body
	// cap and a handler deadline are API concerns, not static-asset ones.
	mux.Handle("/", httpx.Chain(spa, httpx.RequestID(), httpx.AccessLog(logger), httpx.Recover(logger)))

	return mux, err
}
