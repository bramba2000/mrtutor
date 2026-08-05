package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DefaultReadPoolSize is used by Open when Options.ReadPoolSize is zero or
// negative, so the package stands alone without requiring a caller to know
// a sensible pool size.
const DefaultReadPoolSize = 4

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

// Options configures Open.
type Options struct {
	// Path is the filesystem path to the SQLite database file.
	Path string
	// Logger receives debug logging; a nil Logger falls back to slog.Default().
	Logger *slog.Logger
	// ReadPoolSize bounds the read connection pool. Zero or negative falls
	// back to DefaultReadPoolSize.
	ReadPoolSize int
}

// Open opens a connection to the SQLite database at opts.Path and returns
// the *DB object for both read and write operations.
func Open(ctx context.Context, opts Options) (*DB, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "sqlite")

	readPoolSize := opts.ReadPoolSize
	if readPoolSize <= 0 {
		readPoolSize = DefaultReadPoolSize
	}

	writeDSN, err := dsn(opts.Path, true)
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

	readDSN, err := dsn(opts.Path, false)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to build read DSN: %w", err)
	}
	logger.Debug("Opening read connection", "dsn", readDSN, "readPoolSize", readPoolSize)

	r, err := sql.Open("sqlite3", readDSN)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to open read connection: %w", err)
	}
	r.SetMaxIdleConns(readPoolSize)
	r.SetMaxOpenConns(readPoolSize)
	r.SetConnMaxIdleTime(time.Minute)

	if err := r.PingContext(ctx); err != nil {
		w.Close()
		r.Close()
		return nil, fmt.Errorf("failed to ping read connection: %w", err)
	}

	return &DB{W: w, R: r, logger: logger}, nil
}
