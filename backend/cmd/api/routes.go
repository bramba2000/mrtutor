package main

import (
	"log/slog"
	"net/http"

	ehttp "github.com/bramba2000/mrtutor/backend/cmd/api/http"
)

func RegisterRoutes(services Services, mux *http.ServeMux, logger *slog.Logger) {
	auth := ehttp.NewAuthHandler(services.Auth, logger)
	apiMux := http.NewServeMux()
	apiMux.Handle("POST /auth/login", auth.Login)
	apiMux.Handle("POST /auth/register", auth.Register)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMux))

	mux.HandleFunc("GET /health", healthHandler)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if isShuttingDown.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service is shutting down"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
