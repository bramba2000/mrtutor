package auth_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
	"golang.org/x/crypto/bcrypt"
)

func seedPrincipal(t testing.TB, store auth.PrincipalStore, username, email, password string) auth.Principal {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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

// Revoke implements [auth.SessionStore].
func (m mockSessionStore) Revoke(ctx context.Context, sessionId [32]byte) error {
	if s, ok := m[sessionId]; !ok {
		return auth.ErrSessionNotFound
	} else {
		s.RevokedAt = new(time.Now().UTC())
		m[sessionId] = s
	}
	return nil
}

// Create implements [auth.SessionStore].
func (m mockSessionStore) Create(ctx context.Context, session auth.Session) (auth.Session, error) {
	m[session.TokenHash] = session
	return session, nil
}

func (m mockSessionStore) DeleteExpired(ctx context.Context, expirationTime time.Time) error {
	for tokenHash, session := range m {
		if session.CreatedAt.Before(expirationTime) {
			delete(m, tokenHash)
		} else if session.RevokedAt != nil {
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
	seededStore := mockSessionStore{
		tokenHash: auth.Session{
			TokenHash: tokenHash,
			CreatedAt: time.Now().UTC(),
		},
	}
	tt := []struct {
		name         string
		sessionStore auth.SessionStore
		sessionToken string
		matchErr     func(err error) error
	}{
		{
			name:         "Succed with valid session token",
			sessionStore: seededStore,
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
			sessionStore: seededStore,
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
			svc := auth.NewService(nil, tc.sessionStore, nil)
			gotErr := svc.Logout(t.Context(), tc.sessionToken)
			if tc.matchErr != nil {
				if err := tc.matchErr(gotErr); err != nil {
					t.Errorf("Logout() failed error match: %v", err)
				}
			}
		})
	}
}
