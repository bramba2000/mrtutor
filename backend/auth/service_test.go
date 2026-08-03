package auth_test

import (
	"context"
	"crypto/sha256"
	"testing"

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

func TestService(t *testing.T) {
	principalStore := mockPrincipalStore{
		db:    make(map[int]auth.Principal),
		count: 0,
	}
	sessionStore := mockSessionStore(make(map[[32]byte]auth.Session))

	service := auth.NewService(principalStore, sessionStore)
	t.Run("Login", func(t *testing.T) {
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
	})
}

type mockPrincipalStore struct {
	db    map[int]auth.Principal
	count int
}

// Create implements [auth.PrincipalStore].
func (m mockPrincipalStore) Create(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
	m.count += 1
	principal.ID = m.count
	m.db[principal.ID] = principal

	return principal, nil
}

// GetByUsernameOrEmail implements [auth.PrincipalStore].
func (m mockPrincipalStore) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (auth.Principal, error) {
	for _, principal := range m.db {
		if principal.Username == usernameOrEmail || principal.Email == usernameOrEmail {
			return principal, nil
		}
	}

	return auth.Principal{}, auth.ErrPrincipalNotFound
}

var _ auth.PrincipalStore = mockPrincipalStore{}

type mockSessionStore map[[32]byte]auth.Session

// Create implements [auth.SessionStore].
func (m mockSessionStore) Create(ctx context.Context, session auth.Session) (auth.Session, error) {
	m[session.TokenHash] = session
	return session, nil
}

var _ auth.SessionStore = mockSessionStore{}
