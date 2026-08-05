package sqlitetest

import (
	"log/slog"
	"testing"

	"github.com/bramba2000/mrtutor/backend/sqlite"
)

func OpenTemp(t testing.TB) *sqlite.DB {
	path := t.TempDir() + "/test.db"
	db, err := sqlite.Open(t.Context(), sqlite.Options{
		Path:   path,
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	err = db.RunEmbeddedMigrations(t.Context())
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}
