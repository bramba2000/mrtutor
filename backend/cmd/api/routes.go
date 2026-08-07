package main

import (
	"log/slog"

	"github.com/bramba2000/mrtutor/backend/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

func RegisterRoutes(services Services, router *httpx.Router, logger *slog.Logger, authCfg authhttp.Config) {
	authhttp.NewHandler(services.Auth, authCfg, logger).Mount(router)
}

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
