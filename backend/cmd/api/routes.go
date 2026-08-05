package main

import (
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/auth/authhttp"
)

func RegisterRoutes(services Services, mux *http.ServeMux, logger *slog.Logger, authCfg authhttp.Config) {
	auth := authhttp.NewHandler(services.Auth, authCfg, logger)
	apiMux := http.NewServeMux()
	apiMux.Handle("POST /auth/login", auth.Login)
	apiMux.Handle("POST /auth/register", auth.Register)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMux))
}
