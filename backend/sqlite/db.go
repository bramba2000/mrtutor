package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	W *sql.DB
	R *sql.DB
}

// Close closes both the write and read connections to the SQLite database.
//
// The returned error support Unwrap() []error to retrieve the underlying errors from closing the write and read connections.
func (db *DB) Close() error {
	wErr := db.W.Close()
	rErr := db.R.Close()

	if wErr != nil {
		return fmt.Errorf("failed to close write connection: %w", wErr)
	}
	if rErr != nil {
		return fmt.Errorf("failed to close read connection: %w", rErr)
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
func dsn(path string, write bool) string {
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

	p := filepath.ToSlash(path)
	if p[0] != '/' {
		p = "/" + p
	}

	url := url.URL{
		Scheme:   "file",
		Path:     p,
		RawQuery: strings.Join(params, "&"),
	}

	return url.String()
}

// Given the path to the SQLite database, open a connection to it and return the *sql.DB object for both read and write operations.
func Open(ctx context.Context, path string) (*DB, error) {
	w, err := sql.Open("sqlite3", dsn(path, true))
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

	r, err := sql.Open("sqlite3", dsn(path, false))
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

	return &DB{W: w, R: r}, nil
}
