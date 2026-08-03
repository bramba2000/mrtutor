package sqlite

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

type gooseLoggerAdapter struct {
	logger slog.Logger
}

func (g gooseLoggerAdapter) Printf(format string, v ...interface{}) {
	g.logger.Debug(fmt.Sprintf(format, v...))
}

func (g gooseLoggerAdapter) Fatalf(format string, v ...interface{}) {
	g.logger.Error(fmt.Sprintf(format, v...))
}

func (db *DB) RunMigrations(ctx context.Context, fs fs.FS, path string) error {
	goose.SetLogger(gooseLoggerAdapter{logger: *db.logger})
	goose.SetBaseFS(fs)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db.W, path); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	db.logger.Debug("Migrations completed successfully")
	return nil
}

func (db *DB) RunEmbeddedMigrations(ctx context.Context) error {
	return db.RunMigrations(ctx, EmbeddedMigrations, "migrations")
}
