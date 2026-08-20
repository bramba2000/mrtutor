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

// TestUnitOfWork_RunInTx_RollsBackOnFailure is the regression test the
// architecture review's Phase 6 called for: force the session insert to
// fail inside a transaction that already created a principal, and assert
// the principal does not survive. Register (auth/service.go) depends on
// this property to avoid orphaning an account when session creation fails.
func TestUnitOfWork_RunInTx_RollsBackOnFailure(t *testing.T) {
	skipIfShort(t)
	db := sqlitetest.OpenTemp(t)
	storage := authsqlite.Build(db)

	const username = "atomictest"
	err := storage.UnitOfWork.RunInTx(t.Context(), func(stores auth.Stores) error {
		_, err := stores.Principal.Create(t.Context(), auth.Principal{
			Username:     username,
			Email:        "atomictest@example.com",
			PasswordHash: []byte("hash"),
			CreatedAt:    time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		// Force the session insert to fail: user_id references a
		// principal that doesn't exist, violating the FK constraint on
		// sessions even though the principal insert above succeeded
		// within this same, uncommitted transaction.
		_, err = stores.Session.Create(t.Context(), auth.Session{
			TokenHash: sha256.Sum256([]byte("token")),
			UserID:    999999,
			CreatedAt: time.Now().UTC(),
		})
		return err
	})
	if err == nil {
		t.Fatal("expected the transaction to fail")
	}

	_, err = storage.PrincipalStore.GetByUsernameOrEmail(t.Context(), username)
	if !errors.Is(err, auth.ErrPrincipalNotFound) {
		t.Fatalf("expected the principal to not survive the rolled-back transaction, got: %v", err)
	}
}
