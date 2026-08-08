package main

import (
	"log/slog"

	"github.com/bramba2000/mrtutor/backend/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

func registerRoutes(services Services, router *httpx.Router, logger *slog.Logger, cfg config.Config) {
	handlers := []interface {
		Mount(*httpx.Router)
	}{
		authhttp.NewHandler(services.Auth, authhttp.Config{Secure: cfg.AppMode == config.AppModeProd}, logger),
	}

	for _, h := range handlers {
		h.Mount(router)
	}
}
