package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	principalStore PrincipalStore
	sessionStore   SessionStore
	unitOfWork     UnitOfWork
}

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

func (svc Service) Login(ctx context.Context, in LoginIn) (string, error) {
	principal, err := svc.principalStore.GetByUsernameOrEmail(ctx, in.Token)
	if err != nil && !errors.Is(err, errs.NotFound) {
		return "", err
	}

	if errors.Is(err, errs.NotFound) {
		checkPasswordAndHash(in.Password, []byte("$2a$10$KorBjJKn1XhwziiSfTZA4OBJngdfphnBL6a7rIdVJk6QsgYFy1aki")) // Dummy hash for timing attack mitigation
		return "", ErrInvalidCredentials
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

type RegisterIn struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (in RegisterIn) Validate() error {
	return validation.Errors{
		"username": validation.Validate(in.Username, validation.Required, validation.MinLength[string](6)),
		"email":    validation.Validate(in.Email, validation.Required, validation.NotBlank, validation.Email),
		"password": validation.Validate(in.Password, validation.Required, validation.NotBlank, passwordValidator),
	}.Err()
}

var passwordValidator validation.Validator[string] = func(s string) error {
	const minPasswordLength = 8
	const maxPasswordLength = 72
	if len(s) < minPasswordLength {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	if len(s) > maxPasswordLength {
		return fmt.Errorf("password must be at most 72 characters long")
	}
	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, c := range s {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()-_=+[]{}|;:',.<>?/", c):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return fmt.Errorf("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	}
	return nil
}

type RegisterOut struct {
	Principal    Principal
	SessionToken string
}

func (svc Service) Register(ctx context.Context, in RegisterIn) (RegisterOut, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return RegisterOut{}, err
	}

	token, err := newSessionToken()
	if err != nil {
		return RegisterOut{}, err
	}
	tokenHash := sha256.Sum256([]byte(token))

	var principal Principal
	err = svc.unitOfWork.RunInTx(ctx, func(stores Stores) error {
		var err error
		principal, err = stores.Principal.Create(ctx, Principal{
			Username:     in.Username,
			Email:        in.Email,
			PasswordHash: passwordHash,
		})
		if err != nil {
			return err
		}

		_, err = stores.Session.Create(ctx, Session{
			TokenHash: tokenHash,
			UserID:    principal.ID,
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return RegisterOut{}, err
	}

	return RegisterOut{Principal: principal, SessionToken: token}, nil
}

func (svc Service) Logout(ctx context.Context, sessionToken string) error {
	tokenHash := sha256.Sum256([]byte(sessionToken))
	err := svc.sessionStore.Revoke(ctx, tokenHash)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	return nil
}

func (svc Service) Authenticate(ctx context.Context, sessionToken string) (Principal, error) {
	tokenHash := sha256.Sum256([]byte(sessionToken))
	session, err := svc.sessionStore.GetByID(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, errs.NotFound) {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{}, err
	}
	if session.RevokedAt != nil {
		return Principal{}, ErrSessionRevoked
	}

	principal, err := svc.principalStore.GetByID(ctx, session.UserID)
	if err != nil {
		return Principal{}, fmt.Errorf("failed to get principal for session %v: %w", session, err)
	}

	return principal, nil
}

func (svc Service) CleanupExpiredSessions(ctx context.Context) error {
	timeout := time.Now().Add(-10 * 24 * time.Hour) // 10 days
	err := svc.sessionStore.DeleteExpired(ctx, timeout)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}

func NewService(principalStore PrincipalStore, sessionStore SessionStore, uow UnitOfWork) Service {
	return Service{
		principalStore: principalStore,
		sessionStore:   sessionStore,
		unitOfWork:     uow,
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
