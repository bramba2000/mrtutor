package authsqlite

import (
	"cmp"
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type PrincipalStore struct {
	r gen.DBTX
	w gen.DBTX
}

// GetByID implements [auth.PrincipalStore].
func (p *PrincipalStore) GetByID(ctx context.Context, principalId int) (auth.Principal, error) {
	principal, err := gen.New(p.r).GetPrincipalById(ctx, int64(principalId))
	if err != nil {
		return auth.Principal{}, sqlite.TranslateSQLError("get principal by id ", err, auth.ErrPrincipalNotFound, nil)
	}
	return principalFromDB(principal), nil
}

// Create implements [auth.PrincipalStore].
func (p *PrincipalStore) Create(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
	id, err := gen.New(p.w).CreatePrincipal(ctx, gen.CreatePrincipalParams{
		Username:     principal.Username,
		Email:        principal.Email,
		PasswordHash: principal.PasswordHash,
		CreatedAt:    cmp.Or(principal.CreatedAt, time.Now().UTC()),
	})
	if err != nil {
		return auth.Principal{}, sqlite.TranslateSQLError("create principal ", err, nil, auth.ErrConflictPrincipal)
	}
	principal.ID = int(id)
	return principal, nil
}

// GetByUsernameOrEmail implements [auth.PrincipalStore].
func (p *PrincipalStore) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (auth.Principal, error) {
	principal, err := gen.New(p.r).GetPrincipalByUsernameOrEmail(ctx, usernameOrEmail)
	if err != nil {
		return auth.Principal{}, sqlite.TranslateSQLError("get principal by token "+usernameOrEmail, err, auth.ErrPrincipalNotFound, nil)
	}
	return principalFromDB(principal), nil
}

var _ auth.PrincipalStore = (*PrincipalStore)(nil)

func principalFromDB(principal gen.Principal) auth.Principal {
	return auth.Principal{
		ID:           int(principal.ID),
		Username:     principal.Username,
		Email:        principal.Email,
		PasswordHash: principal.PasswordHash,
		CreatedAt:    principal.CreatedAt,
		UpdatedAt:    principal.UpdatedAt.Time,
	}
}
