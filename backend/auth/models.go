package auth

import (
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
)

type Principal struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash []byte    `json:"-"`
	CreatedAt    time.Time `json:"createdAt,omitzero"`
	UpdatedAt    time.Time `json:"updatedAt,omitzero"`
}

type Session struct {
	TokenHash [32]byte   `json:"id"`
	UserID    int        `json:"userId"`
	CreatedAt time.Time  `json:"createdAt,omitzero"`
	RevokedAt *time.Time `json:"revokedAt,omitzero"`
}

var (
	ErrPrincipalNotFound  = errs.Domain("principal.notFound", "principal not found", errs.NotFound)
	ErrSessionNotFound    = errs.Domain("session.notFound", "session not found", errs.NotFound)
	ErrConflictPrincipal  = errs.Domain("principal.conflict", "principal already exists", errs.Conflict)
	ErrInvalidCredentials = errs.Domain("invalidCredentials", "invalid credentials", errs.Unauthenticated)
	ErrSessionConflict    = errs.Domain("session.conflict", "session already exists", errs.Conflict)
)
