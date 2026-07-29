package sqlite_test

import (
	"log/slog"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/bramba2000/mrtutor/backend/sqlite"
)

func TestOpen(t *testing.T) {
	t.Run("Success when existing path", func(t *testing.T) {
		path := filepath.Join(t.ArtifactDir(), "test.db")
		db, err := sqlite.Open(t.Context(), path, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err := db.Close()
			if err != nil {
				t.Fatal(err)
			}
		})

		if err := db.W.Ping(); err != nil {
			t.Error("failed to ping write connection", err)
		}

		if err := db.R.Ping(); err != nil {
			t.Error("failed to ping read connection", err)
		}
	})
	t.Run("Fail when non-existing dir", func(t *testing.T) {
		path := filepath.Join(t.ArtifactDir(), "nonExisting", "test.db")
		_, err := sqlite.Open(t.Context(), path, nil)
		if err == nil {
			t.Fatal("expected error when opening database in non-existing directory, got nil")
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("Success when open", func(t *testing.T) {
		path := filepath.Join(t.ArtifactDir(), "test.db")
		db, err := sqlite.Open(t.Context(), path, nil)
		if err != nil {
			t.Fatal(err)
		}

		if err := db.Close(); err != nil {
			t.Fatal("failed to close database", err)
		}
	})
}

func TestRunMigrations(t *testing.T) {
	t.Run("Success when open db and correct migrations", func(t *testing.T) {
		path := filepath.Join(t.ArtifactDir(), "test.db")
		db, err := sqlite.Open(t.Context(), path, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err := db.Close()
			if err != nil {
				t.Fatal(err)
			}
		})

		migrations := fstest.MapFS{
			"001_create_users_table.sql": &fstest.MapFile{
				Data: []byte(`-- +goose Up
					CREATE TABLE users (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name TEXT NOT NULL
					);
				`),
			},
			"002_add_email_to_users.sql": &fstest.MapFile{
				Data: []byte(`-- +goose Up
					ALTER TABLE users ADD COLUMN email TEXT;
				`),
			},
		}

		if err := db.RunMigrations(t.Context(), migrations, "."); err != nil {
			t.Fatal("failed to run migrations", err)
		}
	})
}
