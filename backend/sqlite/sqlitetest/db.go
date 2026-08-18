package sqlitetest

import (
	"log/slog"
	"testing"

	"github.com/bramba2000/mrtutor/backend/sqlite"
)

func OpenTemp(t testing.TB) *sqlite.DB {
	if testing.Short() {
		t.Fatalf("cannot use real db when in short test mode")
	}
	path := t.TempDir() + "/test.db"
	db, err := sqlite.Open(t.Context(), sqlite.Options{
		Path:   path,
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Logf("DB open at %s", path)
	t.Cleanup(func() {
		db.Close()
	})
	err = db.RunEmbeddedMigrations(t.Context())
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}
