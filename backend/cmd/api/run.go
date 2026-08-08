package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/httpx"
	"github.com/bramba2000/mrtutor/backend/scheduler"
	"github.com/bramba2000/mrtutor/backend/sqlite"
	"golang.org/x/sync/errgroup"
)

func run(ctx context.Context, stderr io.Writer, lookupEnv func(string) (string, bool)) (err error) {
	cfg, err := config.Load(lookupEnv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rootCtx, cancelRoot := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancelRoot()

	logger, closeLogger, err := newLogger(cfg.Log, stderr)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { err = errors.Join(err, closeLogger()) }()

	db, err := sqlite.Open(rootCtx, sqlite.Options{
		Path:         cfg.DB.File,
		Logger:       logger,
		ReadPoolSize: cfg.DB.ReadPoolSize,
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	err = db.RunEmbeddedMigrations(rootCtx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	svcs := createServices(db)
	readiness := &httpx.Readiness{}

	handler, err := newHandler(svcs, readiness, logger, cfg)
	if err != nil {
		if cfg.AppMode == config.AppModeProd {
			return fmt.Errorf("serve frontend: %w", err)
		}
		logger.Warn("frontend assets not built, serving 503 at /", "error", err)
	}

	srv := httpx.NewServer(httpx.Config{
		Address:         cfg.Server.Address(),
		Handler:         handler,
		Logger:          logger,
		LogLevel:        cfg.Log.Level,
		ShutdownTimeout: cfg.Server.ShutdownTimeout,
		DrainPeriod:     cfg.Server.ReadinessDrainPeriod,
		OnShuttingDown:  readiness.Shutdown,
	})

	sched := scheduler.New(scheduler.Config{
		Logger:          logger,
		Location:        cfg.Scheduler.Location,
		DrainPeriod:     cfg.Scheduler.DrainPeriod,
		ShutdownTimeout: cfg.Scheduler.ShutdownTimeout,
	})
	if err := registerTasks(sched, svcs); err != nil {
		return fmt.Errorf("register scheduled tasks: %w", err)
	}

	g, gctx := errgroup.WithContext(rootCtx)
	g.Go(func() error { return srv.Run(gctx) })
	g.Go(func() error { return sched.Run(gctx) })
	return g.Wait()
}

func newLogger(cfg config.Log, fallback io.Writer) (*slog.Logger, func() error, error) {
	options := &slog.HandlerOptions{Level: cfg.Level}

	w := fallback
	closeFn := func() error { return nil }
	if cfg.File == "" {
		options.ReplaceAttr = stripTimestamp
	} else {
		f, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %q: %w", cfg.File, err)
		}
		w, closeFn = f, f.Close
	}

	var handler slog.Handler = slog.NewTextHandler(w, options)
	if cfg.Format == config.LogFormatJSON {
		handler = slog.NewJSONHandler(w, options)
	}
	return slog.New(handler), closeFn, nil
}

func stripTimestamp(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}
