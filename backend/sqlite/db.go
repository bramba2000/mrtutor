package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var EmbeddedMigrations embed.FS

type DB struct {
	W      *sql.DB
	R      *sql.DB
	logger *slog.Logger
}

// Close closes both the write and read connections to the SQLite database.
//
// The returned error support Unwrap() []error to retrieve the underlying errors from closing the write and read connections.
func (db *DB) Close() error {
	rErr := db.R.Close()
	wErr := db.W.Close()

	if wErr != nil {
		wErr = fmt.Errorf("failed to close write connection: %w", wErr)
	}
	if rErr != nil {
		rErr = fmt.Errorf("failed to close read connection: %w", rErr)
	}
	return errors.Join(wErr, rErr)
}

// InTx executes the provided function within a database transaction.
//
// If the function returns an error, the transaction is rolled back; otherwise, it is committed.
// The context is used for managing the transaction's lifetime and cancellation. If reaching the [db.W] while in a transaction, it will be blocked until the transaction is completed (committed or rolled back).
func (db *DB) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.W.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rbErr, err)
		}
		return fmt.Errorf("transaction function returned an error: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

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

// dsn returns the Data Source Name (DSN) for connecting to a SQLite database at the given path.
// The write parameter determines whether the DSN is for a read-write connection (true) or a read-only connection (false).
func dsn(path string, write bool) (string, error) {
	params := []string{
		"_journal=WAL",
		"_fk=on",
		"_timeout=5000",
		"_sync=NORMAL",
	}
	if write {
		params = append(params, "mode=rwc")
		params = append(params, "_query_only=0")
		params = append(params, "_txlock=immediate")
	} else {
		params = append(params, "mode=ro")
		params = append(params, "_query_only=1")
		params = append(params, "_txlock=deferred")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %q: %w", path, err)
	}

	url := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(abs),
		RawQuery: strings.Join(params, "&"),
	}

	return url.String(), nil
}

// Given the path to the SQLite database, open a connection to it and return the *sql.DB object for both read and write operations.
func Open(ctx context.Context, path string, logger *slog.Logger) (*DB, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "sqlite")

	writeDSN, err := dsn(path, true)
	if err != nil {
		return nil, fmt.Errorf("failed to build write DSN: %w", err)
	}
	logger.Debug("Opening write connection", "dsn", writeDSN)

	w, err := sql.Open("sqlite3", writeDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open write connection: %w", err)
	}
	w.SetMaxIdleConns(1)
	w.SetMaxOpenConns(1)
	w.SetConnMaxIdleTime(0)
	w.SetConnMaxLifetime(0)

	if err := w.PingContext(ctx); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to ping write connection: %w", err)
	}

	readDSN, err := dsn(path, false)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to build read DSN: %w", err)
	}
	logger.Debug("Opening read connection", "dsn", readDSN, "readPoolSize", config.ReadPoolSize)

	r, err := sql.Open("sqlite3", readDSN)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to open read connection: %w", err)
	}
	r.SetMaxIdleConns(config.ReadPoolSize)
	r.SetMaxOpenConns(config.ReadPoolSize)
	r.SetConnMaxIdleTime(time.Minute)

	if err := r.PingContext(ctx); err != nil {
		w.Close()
		r.Close()
		return nil, fmt.Errorf("failed to ping read connection: %w", err)
	}

	return &DB{W: w, R: r, logger: logger}, nil
}
