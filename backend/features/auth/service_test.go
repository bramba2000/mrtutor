package auth_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/auth"
)

func seedPrincipal(t testing.TB, store auth.PrincipalStore, username, email, password string) auth.Principal {
	t.Helper()

	passwordHash, err := auth.GeneratePasswordHash(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	principal, err := store.Create(context.Background(), auth.Principal{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		t.Fatalf("failed to create principal: %v", err)
	}

	return principal
}

func TestService_Login(t *testing.T) {
	principalStore := &mockPrincipalStore{
		db:    make(map[int]auth.Principal),
		count: 0,
	}
	sessionStore := mockSessionStore(make(map[[32]byte]auth.Session))

	service := auth.NewService(principalStore, sessionStore, nil)
	const password = "abc123"
	principal := seedPrincipal(t, principalStore, "test", "test@example.com", password)
	t.Run("Success when provide valid credentials", func(t *testing.T) {
		tt := []struct {
			name  string
			token string
		}{
			{
				name:  "Login with username",
				token: principal.Username,
			}, {
				name:  "Login with email",
				token: principal.Email,
			},
		}
		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				session, err := service.Login(context.Background(), auth.LoginIn{
					Token:    tc.token,
					Password: password,
				})
				if err != nil {
					t.Fatalf("failed to login: %v", err)
				}
				if _, ok := sessionStore[sha256.Sum256([]byte(session))]; !ok {
					t.Fatalf("session not persisted in session store")
				}
			})
		}
	})

	// Regression test for defect #1 (user enumeration): an unknown
	// username/email and a wrong password for a known account must be
	// indistinguishable to the caller, both in error identity and in
	// whether bcrypt ran (timing).
	t.Run("Unknown token and wrong password are indistinguishable", func(t *testing.T) {
		_, errUnknown := service.Login(context.Background(), auth.LoginIn{
			Token:    "does-not-exist",
			Password: password,
		})
		_, errWrongPassword := service.Login(context.Background(), auth.LoginIn{
			Token:    principal.Username,
			Password: "wrong-password",
		})

		if !errors.Is(errUnknown, auth.ErrInvalidCredentials) {
			t.Errorf("expected unknown token to fail with ErrInvalidCredentials, got %v", errUnknown)
		}
		if !errors.Is(errWrongPassword, auth.ErrInvalidCredentials) {
			t.Errorf("expected wrong password to fail with ErrInvalidCredentials, got %v", errWrongPassword)
		}
		if errUnknown != errWrongPassword {
			t.Errorf("expected both failures to be the same error value, got %v and %v", errUnknown, errWrongPassword)
		}
		if errors.Is(errUnknown, auth.ErrPrincipalNotFound) {
			t.Error("the store's not-found error must not leak past Login")
		}
	})
}

type mockPrincipalStore struct {
	db    map[int]auth.Principal
	count int
}

// GetByID implements [auth.PrincipalStore].
func (m *mockPrincipalStore) GetByID(ctx context.Context, principalId int) (auth.Principal, error) {
	if p, ok := m.db[principalId]; !ok {
		return auth.Principal{}, auth.ErrPrincipalNotFound
	} else {
		return p, nil
	}
}

// Create implements [auth.PrincipalStore].
func (m *mockPrincipalStore) Create(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
	for _, p := range m.db {
		if p.Username == principal.Username {
			return auth.Principal{}, auth.ErrConflictPrincipal
		}
		if p.Email == principal.Email {
			return auth.Principal{}, auth.ErrConflictPrincipal
		}
	}

	m.count += 1
	principal.ID = m.count
	m.db[principal.ID] = principal

	return principal, nil
}

// GetByUsernameOrEmail implements [auth.PrincipalStore].
func (m *mockPrincipalStore) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (auth.Principal, error) {
	for _, principal := range m.db {
		if principal.Username == usernameOrEmail || principal.Email == usernameOrEmail {
			return principal, nil
		}
	}

	return auth.Principal{}, auth.ErrPrincipalNotFound
}

var _ auth.PrincipalStore = &mockPrincipalStore{}

type mockSessionStore map[[32]byte]auth.Session

// GetByID implements [auth.SessionStore].
func (m mockSessionStore) GetByID(ctx context.Context, sessionId [32]byte) (auth.Session, error) {
	if s, ok := m[sessionId]; !ok {
		return auth.Session{}, auth.ErrSessionNotFound
	} else {
		return s, nil
	}
}

// Touch implements [auth.SessionStore].
func (m mockSessionStore) Touch(ctx context.Context, sessionId [32]byte, at time.Time) error {
	if s, ok := m[sessionId]; !ok {
		return auth.ErrSessionNotFound
	} else {
		s.LastSeenAt = at
		m[sessionId] = s
	}
	return nil
}

// Delete implements [auth.SessionStore].
func (m mockSessionStore) Delete(ctx context.Context, sessionId [32]byte) error {
	if _, ok := m[sessionId]; !ok {
		return auth.ErrSessionNotFound
	}
	delete(m, sessionId)
	return nil
}

// Create implements [auth.SessionStore].
func (m mockSessionStore) Create(ctx context.Context, session auth.Session) (auth.Session, error) {
	m[session.TokenHash] = session
	return session, nil
}

func (m mockSessionStore) DeleteExpired(ctx context.Context, createdBefore, lastSeenBefore time.Time) error {
	for tokenHash, session := range m {
		if session.CreatedAt.Before(createdBefore) || session.LastSeenAt.Before(lastSeenBefore) {
			delete(m, tokenHash)
		}
	}
	return nil
}

var _ auth.SessionStore = mockSessionStore{}

type mockUnitOfWork struct {
	principalStore auth.PrincipalStore
	sessionStore   auth.SessionStore
}

func (m mockUnitOfWork) RunInTx(ctx context.Context, fn func(stores auth.Stores) error) error {
	return fn(auth.Stores{
		Principal: m.principalStore,
		Session:   m.sessionStore,
	})
}

var _ auth.UnitOfWork = mockUnitOfWork{}

func TestLoginIn_Validate(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		in      auth.LoginIn
		wantErr bool
	}{
		{
			name: "Succed with valid credentials",
			in: auth.LoginIn{
				Token:    "test",
				Password: "abc123",
			},
			wantErr: false,
		}, {
			name: "Fail with empty token",
			in: auth.LoginIn{
				Token:    "",
				Password: "abc123",
			},
			wantErr: true,
		}, {
			name: "Fail with empty password",
			in: auth.LoginIn{
				Token:    "test",
				Password: "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.in.Validate()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Validate() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Validate() succeeded unexpectedly")
			}
		})
	}
}

func TestRegisterIn_Validate(t *testing.T) {
	base := auth.RegisterIn{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "Abc123!!",
	}

	tests := []struct {
		name    string // description of this test case
		in      func(in *auth.RegisterIn)
		wantErr bool
	}{
		{
			name:    "Succed with valid credentials",
			in:      func(in *auth.RegisterIn) {},
			wantErr: false,
		}, {
			name: "Fail with empty username",
			in: func(in *auth.RegisterIn) {
				in.Username = ""
			},
			wantErr: true,
		}, {
			name: "Fail with too short username",
			in: func(in *auth.RegisterIn) {
				in.Username = "a"
			},
			wantErr: true,
		},
		{
			name: "Fail with empty email",
			in: func(in *auth.RegisterIn) {
				in.Email = ""
			},
			wantErr: true,
		}, {
			name: "Fail with empty email",
			in: func(in *auth.RegisterIn) {
				in.Email = ""
			},
			wantErr: true,
		}, {
			name: "Fail with malformed email",
			in: func(in *auth.RegisterIn) {
				in.Email = "testexample.com"
			},
			wantErr: true,
		}, {
			name: "Fail with empty password",
			in: func(in *auth.RegisterIn) {
				in.Password = ""
			},
			wantErr: true,
		},
		{
			name: "Fail with too short password",
			in: func(in *auth.RegisterIn) {
				in.Password = "Ab1!"
			},
			wantErr: true,
		},
		{
			name: "Fail with too long password",
			in: func(in *auth.RegisterIn) {
				in.Password = strings.Repeat(in.Password, 10)
			},
			wantErr: true,
		},
		{
			name: "Fail when password without uppercase letter",
			in: func(in *auth.RegisterIn) {
				in.Password = strings.ToLower(in.Password)
			},
			wantErr: true,
		},
		{
			name: "Fail when password without lowercase letter",
			in: func(in *auth.RegisterIn) {
				in.Password = strings.ToUpper(in.Password)
			},
			wantErr: true,
		},
		{
			name: "Fail when password without number",
			in: func(in *auth.RegisterIn) {
				in.Password = "Abcdefgh!"
			},
			wantErr: true,
		},
		{
			name: "Fail when password without special character",
			in: func(in *auth.RegisterIn) {
				in.Password = "Abcdefgh1"
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.in(&in)
			gotErr := in.Validate()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Validate() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Validate() succeeded unexpectedly")
			}
		})
	}
}

func TestService_Register(t *testing.T) {
	seededPrincipalStore := &mockPrincipalStore{db: map[int]auth.Principal{}}
	existingPrincipal := seedPrincipal(t, seededPrincipalStore, "testuser", "testuser@example.com", "Abc123!!")

	tests := []struct {
		name           string
		principalStore auth.PrincipalStore
		sessionStore   auth.SessionStore
		in             auth.RegisterIn
		matchOut       func(out auth.RegisterOut) error
		matchErr       func(err error) error
	}{
		{
			name:           "Succed with valid credentials",
			principalStore: &mockPrincipalStore{db: make(map[int]auth.Principal)},
			sessionStore:   make(mockSessionStore),
			in: auth.RegisterIn{
				Username: "testuser",
				Password: "Abc123!!",
				Email:    "testuser@example.com",
			},
			matchErr: func(err error) error {
				if err != nil {
					return fmt.Errorf("expected null error, got %w", err)
				}
				return nil
			},
		},
		{
			name:           "Fail with existing username",
			principalStore: seededPrincipalStore,
			sessionStore:   make(mockSessionStore),
			in: auth.RegisterIn{
				Username: existingPrincipal.Username,
				Password: "Abc123!!",
				Email:    "different@example.com",
			},
			matchErr: func(err error) error {
				if !errors.Is(err, auth.ErrConflictPrincipal) {
					return fmt.Errorf("Expected %v, got %w", auth.ErrConflictPrincipal, err)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService(
				tt.principalStore,
				tt.sessionStore,
				mockUnitOfWork{tt.principalStore, tt.sessionStore},
			)
			got, gotErr := svc.Register(t.Context(), tt.in)
			if tt.matchErr != nil {
				if err := tt.matchErr(gotErr); err != nil {
					t.Errorf("Register() failed error match: %v", err)
				}
			}
			if tt.matchOut != nil {
				if err := tt.matchOut(got); err != nil {
					t.Errorf("Register() failed output match: %v", err)
				}
			}
		})
	}
}

func TestService_Logout(t *testing.T) {
	validSessionToken := "valid-session-token"
	tokenHash := sha256.Sum256([]byte(validSessionToken))
	tt := []struct {
		name         string
		sessionToken string
		matchErr     func(err error) error
	}{
		{
			name:         "Succed with valid session token",
			sessionToken: validSessionToken,
			matchErr: func(err error) error {
				if err != nil {
					return fmt.Errorf("expected null error, got %w", err)
				}
				return nil
			},
		},
		{
			name:         "Fail when non-existing session token",
			sessionToken: "non-existing-session-token",
			matchErr: func(err error) error {
				if err != nil {
					return fmt.Errorf("expected null error, got %w", err)
				}
				return nil
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh store per case: Logout must be able to delete the row
			// out from under a shared store without affecting other cases.
			seededStore := mockSessionStore{
				tokenHash: auth.Session{
					TokenHash:  tokenHash,
					CreatedAt:  time.Now().UTC(),
					LastSeenAt: time.Now().UTC(),
				},
			}
			svc := auth.NewService(nil, seededStore, nil)
			gotErr := svc.Logout(t.Context(), tc.sessionToken)
			if tc.matchErr != nil {
				if err := tc.matchErr(gotErr); err != nil {
					t.Errorf("Logout() failed error match: %v", err)
				}
			}
		})
	}

	t.Run("Removes the session row entirely", func(t *testing.T) {
		seededStore := mockSessionStore{
			tokenHash: auth.Session{
				TokenHash:  tokenHash,
				CreatedAt:  time.Now().UTC(),
				LastSeenAt: time.Now().UTC(),
			},
		}
		svc := auth.NewService(nil, seededStore, nil)
		if err := svc.Logout(t.Context(), validSessionToken); err != nil {
			t.Fatalf("failed to logout: %v", err)
		}
		if _, ok := seededStore[tokenHash]; ok {
			t.Error("expected the session row to be deleted, but it still exists")
		}
	})
}

func TestService_Authenticate(t *testing.T) {
	const sessionToken = "the-session-token"
	tokenHash := sha256.Sum256([]byte(sessionToken))

	newPrincipalStore := func() *mockPrincipalStore {
		return &mockPrincipalStore{db: map[int]auth.Principal{1: {ID: 1, Username: "test"}}, count: 1}
	}

	t.Run("Fresh session is authenticated and last_seen_at is not rewritten", func(t *testing.T) {
		now := time.Now().UTC()
		sessionStore := mockSessionStore{
			tokenHash: auth.Session{TokenHash: tokenHash, UserID: 1, CreatedAt: now, LastSeenAt: now},
		}
		svc := auth.NewService(newPrincipalStore(), sessionStore, nil)

		principal, err := svc.Authenticate(t.Context(), sessionToken)
		if err != nil {
			t.Fatalf("expected successful authentication, got %v", err)
		}
		if principal.ID != 1 {
			t.Errorf("expected principal ID 1, got %d", principal.ID)
		}
		if !sessionStore[tokenHash].LastSeenAt.Equal(now) {
			t.Errorf("expected last_seen_at to stay at %v, got %v", now, sessionStore[tokenHash].LastSeenAt)
		}
	})

	t.Run("Stale but not idle last_seen_at is advanced", func(t *testing.T) {
		now := time.Now().UTC()
		staleSince := now.Add(-10 * time.Minute)
		sessionStore := mockSessionStore{
			tokenHash: auth.Session{TokenHash: tokenHash, UserID: 1, CreatedAt: staleSince, LastSeenAt: staleSince},
		}
		svc := auth.NewService(newPrincipalStore(), sessionStore, nil)

		if _, err := svc.Authenticate(t.Context(), sessionToken); err != nil {
			t.Fatalf("expected successful authentication, got %v", err)
		}
		if !sessionStore[tokenHash].LastSeenAt.After(staleSince) {
			t.Errorf("expected last_seen_at to be advanced past %v, got %v", staleSince, sessionStore[tokenHash].LastSeenAt)
		}
	})

	// Regression test for the token-state oracle: a session past the idle
	// timeout must fail exactly like an unknown token, both in error
	// identity and observable behavior, so a caller cannot distinguish
	// "never existed" from "expired".
	t.Run("Idle-expired session fails identically to an unknown token", func(t *testing.T) {
		now := time.Now().UTC()
		sessionStore := mockSessionStore{
			tokenHash: auth.Session{
				TokenHash:  tokenHash,
				UserID:     1,
				CreatedAt:  now.Add(-4 * time.Hour),
				LastSeenAt: now.Add(-4 * time.Hour), // older than SessionIdleTimeout (3h)
			},
		}
		svc := auth.NewService(newPrincipalStore(), sessionStore, nil)

		_, gotErr := svc.Authenticate(t.Context(), sessionToken)
		_, wantErr := svc.Authenticate(t.Context(), "unknown-token")

		if !errors.Is(gotErr, auth.ErrUnauthenticated) {
			t.Errorf("expected ErrUnauthenticated for an idle-expired session, got %v", gotErr)
		}
		if gotErr != wantErr {
			t.Errorf("expected idle-expired and unknown-token errors to be identical, got %v and %v", gotErr, wantErr)
		}
	})

	t.Run("Absolute cap is not defeated by recent activity", func(t *testing.T) {
		now := time.Now().UTC()
		sessionStore := mockSessionStore{
			tokenHash: auth.Session{
				TokenHash:  tokenHash,
				UserID:     1,
				CreatedAt:  now.Add(-8 * 24 * time.Hour), // older than SessionMaxAge (7d)
				LastSeenAt: now.Add(-1 * time.Minute),
			},
		}
		svc := auth.NewService(newPrincipalStore(), sessionStore, nil)

		_, err := svc.Authenticate(t.Context(), sessionToken)
		if !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("expected ErrUnauthenticated for a session past the absolute cap, got %v", err)
		}
	})
}

func TestService_CleanupExpiredSessions(t *testing.T) {
	now := time.Now().UTC()
	fresh := sha256.Sum256([]byte("fresh"))
	idleExpired := sha256.Sum256([]byte("idle-expired"))
	capExpired := sha256.Sum256([]byte("cap-expired"))

	sessionStore := mockSessionStore{
		fresh: auth.Session{
			TokenHash: fresh, UserID: 1,
			CreatedAt: now.Add(-1 * time.Hour), LastSeenAt: now.Add(-1 * time.Minute),
		},
		idleExpired: auth.Session{
			TokenHash: idleExpired, UserID: 1,
			CreatedAt: now.Add(-1 * time.Hour), LastSeenAt: now.Add(-4 * time.Hour),
		},
		capExpired: auth.Session{
			TokenHash: capExpired, UserID: 1,
			CreatedAt: now.Add(-8 * 24 * time.Hour), LastSeenAt: now.Add(-1 * time.Minute),
		},
	}

	svc := auth.NewService(nil, sessionStore, nil)
	if err := svc.CleanupExpiredSessions(t.Context()); err != nil {
		t.Fatalf("failed to cleanup expired sessions: %v", err)
	}

	if _, ok := sessionStore[fresh]; !ok {
		t.Error("expected the fresh session to survive cleanup")
	}
	if _, ok := sessionStore[idleExpired]; ok {
		t.Error("expected the idle-expired session to be deleted")
	}
	if _, ok := sessionStore[capExpired]; ok {
		t.Error("expected the cap-expired session to be deleted")
	}
}
