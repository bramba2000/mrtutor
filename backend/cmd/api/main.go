package main

import (
	"context"
	"net/http"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
)

var isShuttingDown atomic.Bool

func main() {
	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	srv := NewServer(config.Address(), mux)
	srv.Start()

	<-rootCtx.Done()
	cancel()
	isShuttingDown.Store(true)
	time.Sleep(config.ReadinessDrainPeriod)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()
	err := srv.Shutdown(shutdownCtx)

	if err != nil {
		time.Sleep(config.ShutdownHardTimeout)
	}
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
