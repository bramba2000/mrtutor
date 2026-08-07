package sqlite

import (
	"cmp"
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
	gen "github.com/bramba2000/mrtutor/backend/sqlite/internal"
)

type PrincipalStore struct {
	db *DB
}

// GetByID implements [auth.PrincipalStore].
func (p *PrincipalStore) GetByID(ctx context.Context, principalId int) (auth.Principal, error) {
	principal, err := gen.New(p.db.R).GetPrincipalById(ctx, int64(principalId))
	if err != nil {
		return auth.Principal{}, translateSQLError("get principal by id ", err, auth.ErrPrincipalNotFound, nil)
	}
	return principalFromDB(principal), nil
}

// Create implements [auth.PrincipalStore].
func (p *PrincipalStore) Create(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
	id, err := gen.New(p.db.W).CreatePrincipal(ctx, gen.CreatePrincipalParams{
		Username:     principal.Username,
		Email:        principal.Email,
		PasswordHash: principal.PasswordHash,
		CreatedAt:    cmp.Or(principal.CreatedAt, time.Now().UTC()),
	})
	if err != nil {
		return auth.Principal{}, translateSQLError("create principal ", err, nil, auth.ErrConflictPrincipal)
	}
	principal.ID = int(id)
	return principal, nil
}

// GetByUsernameOrEmail implements [auth.PrincipalStore].
func (p *PrincipalStore) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (auth.Principal, error) {
	principal, err := gen.New(p.db.R).GetPrincipalByUsernameOrEmail(ctx, usernameOrEmail)
	if err != nil {
		return auth.Principal{}, translateSQLError("get principal by token "+usernameOrEmail, err, auth.ErrPrincipalNotFound, nil)
	}
	return principalFromDB(principal), nil
}

var _ auth.PrincipalStore = (*PrincipalStore)(nil)

type SessionStore struct {
	db *DB
}

// GetByID implements [auth.SessionStore].
func (s *SessionStore) GetByID(ctx context.Context, sessionId [32]byte) (auth.Session, error) {
	session, err := gen.New(s.db.R).GetAuthSessionByToken(ctx, sessionId[:])
	if err != nil {
		return auth.Session{}, translateSQLError("get session by id ", err, auth.ErrSessionNotFound, nil)
	}
	return sessionFromDB(session), nil
}

// Revoke implements [auth.SessionStore].
func (s *SessionStore) Revoke(ctx context.Context, sessionId [32]byte) error {
	err := gen.New(s.db.W).RevokeSession(ctx, sessionId[:])
	if err != nil {
		return translateSQLError("revoke session ", err, auth.ErrSessionNotFound, nil)
	}
	return nil
}

// Create implements [auth.SessionStore].
func (s *SessionStore) Create(ctx context.Context, session auth.Session) (auth.Session, error) {
	err := gen.New(s.db.W).CreateAuthSession(ctx, gen.CreateAuthSessionParams{
		UserID:    int64(session.UserID),
		TokenHash: session.TokenHash[:],
		CreatedAt: session.CreatedAt,
	})
	if err != nil {
		return auth.Session{}, translateSQLError("create session ", err, auth.ErrSessionNotFound, auth.ErrSessionConflict)
	}
	return session, nil
}

var _ auth.SessionStore = (*SessionStore)(nil)

func NewPrincipalStore(db *DB) *PrincipalStore {
	return &PrincipalStore{
		db: db,
	}
}

func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{
		db: db,
	}
}

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

func sessionFromDB(session gen.Session) auth.Session {
	var tokenHash [32]byte
	copy(tokenHash[:], session.ID)
	return auth.Session{
		TokenHash: tokenHash,
		UserID:    int(session.UserID),
		CreatedAt: session.CreatedAt,
		RevokedAt: nullTimeToPointer(session.RevokedAt),
	}
}
