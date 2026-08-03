package auth

import "context"

type Stores struct {
	Principals PrincipalStore
}

type PrincipalStore interface {
	// Create creates a new principal in the repository.
	Create(ctx context.Context, principal Principal) (Principal, error)
	// GetByUsernameOrEmail retrieves a principal by its username or email.
	GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (Principal, error)
}

type SessionStore interface {
	// Create creates a new session in the repository.
	Create(ctx context.Context, session Session) (Session, error)
}

type UnitOfWork interface {
	RunInTx(ctx context.Context, fn func(repos Stores) error) error
}
