package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

var isShuttingDown atomic.Bool

func main() {
	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	logger := newLogger()
	logger.Debug("Starting bootstrap")

	db, err := sqlite.Open(rootCtx, config.DatabaseFile, logger)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		return
	}
	defer func() {
		err := db.Close()
		if err != nil {
			logger.Error("Failed to close database", "error", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	srv := NewServer(config.Address(), mux, logger)
	logger.Debug("Boostrap completed")

	srv.Start()

	<-rootCtx.Done()
	cancel()
	logger.Info("Received shutdown signal, starting graceful shutdown")
	isShuttingDown.Store(true)
	time.Sleep(config.ReadinessDrainPeriod)

	logger.Debug("Readiness probe drained")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		logger.Error("Error during shutdown, forcing exit", "error", err)
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

func newLogger() *slog.Logger {
	var handler slog.Handler
	options := &slog.HandlerOptions{
		Level: config.LogLevel,
	}
	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		handler = slog.NewJSONHandler(logFile, options)
	} else {
		options.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Remove the timestamp from the log output
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
		handler = slog.NewTextHandler(os.Stderr, options)
	}
	return slog.New(handler)
}
