package authsqlite_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

// skipIfShort skips a test that needs a real sqlite database under `go test
// -short` — sqlitetest.OpenTemp itself fails the test rather than skipping
// it, so callers that want to be part of the unit-test suite's -short subset
// must check this first.
func skipIfShort(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// seedTestPrincipal creates a principal to satisfy sessions' FK constraint.
func seedTestPrincipal(t testing.TB, store auth.PrincipalStore, username string) auth.Principal {
	t.Helper()
	principal, err := store.Create(t.Context(), auth.Principal{
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: []byte("hash"),
	})
	if err != nil {
		t.Fatalf("failed to seed principal: %v", err)
	}
	return principal
}

func TestSessionStore_CreateGetTouch(t *testing.T) {
	skipIfShort(t)
	db := sqlitetest.OpenTemp(t)
	storage := authsqlite.Build(db)
	principal := seedTestPrincipal(t, storage.PrincipalStore, "sessionowner")

	tokenHash := sha256.Sum256([]byte("test-token"))
	createdAt := time.Now().UTC().Truncate(time.Second)

	created, err := storage.SessionStore.Create(t.Context(), auth.Session{
		TokenHash:  tokenHash,
		UserID:     principal.ID,
		CreatedAt:  createdAt,
		LastSeenAt: createdAt,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if created.TokenHash != tokenHash {
		t.Errorf("expected the returned session to carry the token hash, got %x", created.TokenHash)
	}

	got, err := storage.SessionStore.GetByID(t.Context(), tokenHash)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if got.UserID != principal.ID {
		t.Errorf("expected user id %d, got %d", principal.ID, got.UserID)
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("expected created_at %v, got %v", createdAt, got.CreatedAt)
	}
	if !got.LastSeenAt.Equal(createdAt) {
		t.Errorf("expected last_seen_at %v, got %v", createdAt, got.LastSeenAt)
	}

	touchedAt := createdAt.Add(10 * time.Minute)
	if err := storage.SessionStore.Touch(t.Context(), tokenHash, touchedAt); err != nil {
		t.Fatalf("failed to touch session: %v", err)
	}

	got, err = storage.SessionStore.GetByID(t.Context(), tokenHash)
	if err != nil {
		t.Fatalf("failed to get session after touch: %v", err)
	}
	if !got.LastSeenAt.Equal(touchedAt) {
		t.Errorf("expected last_seen_at %v after touch, got %v", touchedAt, got.LastSeenAt)
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("expected created_at to be unaffected by touch, got %v", got.CreatedAt)
	}
}

func TestSessionStore_GetByID_NotFound(t *testing.T) {
	skipIfShort(t)
	db := sqlitetest.OpenTemp(t)
	storage := authsqlite.Build(db)

	_, err := storage.SessionStore.GetByID(t.Context(), sha256.Sum256([]byte("does-not-exist")))
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	skipIfShort(t)
	db := sqlitetest.OpenTemp(t)
	storage := authsqlite.Build(db)
	principal := seedTestPrincipal(t, storage.PrincipalStore, "logoutuser")

	tokenHash := sha256.Sum256([]byte("logout-token"))
	now := time.Now().UTC()
	if _, err := storage.SessionStore.Create(t.Context(), auth.Session{
		TokenHash: tokenHash, UserID: principal.ID, CreatedAt: now, LastSeenAt: now,
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := storage.SessionStore.Delete(t.Context(), tokenHash); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	_, err := storage.SessionStore.GetByID(t.Context(), tokenHash)
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected the session to be gone after delete, got %v", err)
	}
}

func TestSessionStore_DeleteExpired(t *testing.T) {
	skipIfShort(t)
	db := sqlitetest.OpenTemp(t)
	storage := authsqlite.Build(db)
	principal := seedTestPrincipal(t, storage.PrincipalStore, "cleanupuser")

	now := time.Now().UTC()
	seed := func(name string, createdAt, lastSeenAt time.Time) [32]byte {
		tokenHash := sha256.Sum256([]byte(name))
		if _, err := storage.SessionStore.Create(t.Context(), auth.Session{
			TokenHash: tokenHash, UserID: principal.ID, CreatedAt: createdAt, LastSeenAt: lastSeenAt,
		}); err != nil {
			t.Fatalf("failed to seed session %q: %v", name, err)
		}
		return tokenHash
	}

	fresh := seed("fresh", now.Add(-time.Hour), now.Add(-time.Minute))
	idleExpired := seed("idle-expired", now.Add(-time.Hour), now.Add(-4*time.Hour))
	capExpired := seed("cap-expired", now.Add(-8*24*time.Hour), now.Add(-time.Minute))

	err := storage.SessionStore.DeleteExpired(t.Context(), now.Add(-7*24*time.Hour), now.Add(-3*time.Hour))
	if err != nil {
		t.Fatalf("failed to delete expired sessions: %v", err)
	}

	if _, err := storage.SessionStore.GetByID(t.Context(), fresh); err != nil {
		t.Errorf("expected the fresh session to survive, got %v", err)
	}
	if _, err := storage.SessionStore.GetByID(t.Context(), idleExpired); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected the idle-expired session to be deleted, got %v", err)
	}
	if _, err := storage.SessionStore.GetByID(t.Context(), capExpired); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected the cap-expired session to be deleted, got %v", err)
	}
}
