package sqlite_test

import (
	"database/sql"
	"log"
	"testing"

	"github.com/bramba2000/mrtutor/backend/sqlite"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

type testLogger struct {
	t testing.TB
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func (l *testLogger) Fatalf(format string, v ...interface{}) {
	l.t.Fatalf(format, v...)
}

// Integration test to ensure that all migrations can be applied
// and rolled back without errors.
func TestAllMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	goose.SetLogger(&testLogger{t})
	goose.SetBaseFS(sqlite.EmbeddedMigrations)
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}
	if err := goose.Down(db, "migrations"); err != nil {
		t.Fatalf("failed to rollback migrations: %v", err)
	}
}
