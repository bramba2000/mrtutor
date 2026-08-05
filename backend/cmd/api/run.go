package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

func run(ctx context.Context, stderr io.Writer, _ func(string) (string, bool)) (err error) {
	rootCtx, cancelRoot := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancelRoot()

	logger, closeLogger, err := newLogger(stderr)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { err = errors.Join(err, closeLogger()) }()

	db, err := sqlite.Open(rootCtx, config.DatabaseFile, logger)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	err = db.RunEmbeddedMigrations(rootCtx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	mux := http.NewServeMux()
	svcs := createServices(db)
	RegisterRoutes(svcs, mux, logger)

	srv := NewServer(config.Address(), mux, logger)

	srv.Start()

	<-rootCtx.Done()
	cancelRoot()
	logger.Info("Received shutdown signal, starting graceful shutdown")
	isShuttingDown.Store(true)
	time.Sleep(config.ReadinessDrainPeriod)

	logger.Debug("Readiness probe drained")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancelShutdown()

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		logger.Error("Error during shutdown, forcing exit", "error", err)
		time.Sleep(config.ShutdownHardTimeout)
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}

func newLogger(fallback io.Writer) (*slog.Logger, func() error, error) {
	options := &slog.HandlerOptions{Level: config.LogLevel}

	if config.LogFile == "" {
		options.ReplaceAttr = stripTimestamp
		return slog.New(slog.NewTextHandler(fallback, options)), func() error { return nil }, nil
	}

	f, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", config.LogFile, err)
	}
	return slog.New(slog.NewJSONHandler(f, options)), f.Close, nil
}

func stripTimestamp(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}
