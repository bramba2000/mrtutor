package auth

import (
	"context"
	"time"
)

type Stores struct {
	Principal PrincipalStore
	Session   SessionStore
}

type PrincipalStore interface {
	// Create creates a new principal in the repository.
	Create(ctx context.Context, principal Principal) (Principal, error)
	// GetByUsernameOrEmail retrieves a principal by its username or email.
	GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (Principal, error)
	// GetByID retrieves a principal by its ID.
	GetByID(ctx context.Context, principalId int) (Principal, error)
}

type SessionStore interface {
	// Create creates a new session in the repository.
	Create(ctx context.Context, session Session) (Session, error)
	// Revoke revokes a session by its ID.
	Revoke(ctx context.Context, sessionId [32]byte) error
	// GetByID retrieves a session by its ID.
	GetByID(ctx context.Context, sessionId [32]byte) (Session, error)
	// DeleteExpired deletes all sessions that are revoked or created before the specified expiration time.
	DeleteExpired(ctx context.Context, expirationTime time.Time) error
}

type UnitOfWork interface {
	RunInTx(ctx context.Context, fn func(repos Stores) error) error
}
