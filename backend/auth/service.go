package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/bramba2000/mrtutor/backend/validation"
	"golang.org/x/crypto/bcrypt"
)

type LoginIn struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (in LoginIn) Validate() error {
	return validation.Errors{
		"token":    validation.Validate(in.Token, validation.Required, validation.NotBlank),
		"password": validation.Validate(in.Password, validation.Required, validation.NotBlank),
	}.Err()
}

type Service struct {
	principalStore PrincipalStore
	sessionStore   SessionStore
}

func (svc Service) Login(ctx context.Context, in LoginIn) (string, error) {
	principal, err := svc.principalStore.GetByUsernameOrEmail(ctx, in.Token)
	if err != nil {
		return "", err
	}

	if ok := checkPasswordAndHash(in.Password, principal.PasswordHash); !ok {
		return "", ErrInvalidCredentials
	}

	sessionToken, err := newSessionToken()
	if err != nil {
		return "", err
	}
	tokenHash := sha256.Sum256([]byte(sessionToken))

	_, err = svc.sessionStore.Create(ctx, Session{
		TokenHash: tokenHash,
		UserID:    principal.ID,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}

	return sessionToken, nil
}

func NewService(principalStore PrincipalStore, sessionStore SessionStore) Service {
	return Service{
		principalStore: principalStore,
		sessionStore:   sessionStore,
	}
}

func newSessionToken() (string, error) {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func checkPasswordAndHash(password string, hash []byte) bool {
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}
