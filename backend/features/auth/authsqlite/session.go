package authsqlite

import (
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type SessionStore struct {
	r gen.DBTX
	w gen.DBTX
}

// GetByID implements [auth.SessionStore].
func (s *SessionStore) GetByID(ctx context.Context, sessionId [32]byte) (auth.Session, error) {
	session, err := gen.New(s.r).GetAuthSessionByToken(ctx, sessionId[:])
	if err != nil {
		return auth.Session{}, sqlite.TranslateSQLError("get session by id ", err, auth.ErrSessionNotFound, nil)
	}
	return sessionFromDB(session), nil
}

// Touch implements [auth.SessionStore].
func (s *SessionStore) Touch(ctx context.Context, sessionId [32]byte, at time.Time) error {
	err := gen.New(s.w).TouchAuthSession(ctx, gen.TouchAuthSessionParams{
		TokenHash:  sessionId[:],
		LastSeenAt: at.UTC(),
	})
	if err != nil {
		return sqlite.TranslateSQLError("touch session ", err, auth.ErrSessionNotFound, nil)
	}
	return nil
}

// Delete implements [auth.SessionStore].
func (s *SessionStore) Delete(ctx context.Context, sessionId [32]byte) error {
	err := gen.New(s.w).DeleteAuthSession(ctx, sessionId[:])
	if err != nil {
		return sqlite.TranslateSQLError("delete session ", err, auth.ErrSessionNotFound, nil)
	}
	return nil
}

// Create implements [auth.SessionStore].
func (s *SessionStore) Create(ctx context.Context, session auth.Session) (auth.Session, error) {
	err := gen.New(s.w).CreateAuthSession(ctx, gen.CreateAuthSessionParams{
		UserID:     int64(session.UserID),
		TokenHash:  session.TokenHash[:],
		CreatedAt:  session.CreatedAt,
		LastSeenAt: session.LastSeenAt,
	})
	if err != nil {
		return auth.Session{}, sqlite.TranslateSQLError("create session ", err, auth.ErrSessionNotFound, auth.ErrSessionConflict)
	}
	return session, nil
}

func (s *SessionStore) DeleteExpired(ctx context.Context, createdBefore, lastSeenBefore time.Time) error {
	err := gen.New(s.w).DeleteExpiredSessions(ctx, gen.DeleteExpiredSessionsParams{
		CreatedBefore:  createdBefore.UTC(),
		LastSeenBefore: lastSeenBefore.UTC(),
	})
	if err != nil {
		return sqlite.TranslateSQLError("delete expired sessions ", err, auth.ErrSessionNotFound, nil)
	}
	return nil
}

var _ auth.SessionStore = (*SessionStore)(nil)

func sessionFromDB(session gen.Session) auth.Session {
	var tokenHash [32]byte
	copy(tokenHash[:], session.ID)
	return auth.Session{
		TokenHash:  tokenHash,
		UserID:     int(session.UserID),
		CreatedAt:  session.CreatedAt,
		LastSeenAt: session.LastSeenAt,
	}
}
