package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
	handlers "github.com/bramba2000/mrtutor/backend/cmd/api/http"
	"github.com/bramba2000/mrtutor/backend/sqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
	"golang.org/x/crypto/bcrypt"
)

func skipIfNotIntegration(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func seedPrincipal(t testing.TB, repo auth.PrincipalStore, username, password string) auth.Principal {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	principal, err := repo.Create(context.Background(), auth.Principal{
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to seed principal: %v", err)
	}
	return principal
}

func encodeBody(t testing.TB, input any) io.Reader {
	t.Helper()
	buf, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to encode body: %v", err)
	}
	return bytes.NewReader(buf)
}

func TestAuth(t *testing.T) {
	skipIfNotIntegration(t)
	db := sqlitetest.OpenTemp(t)
	principalRepo := sqlite.NewPrincipalStore(db)
	const password = "testpassword"
	svc := auth.NewService(principalRepo, sqlite.NewSessionStore(db))
	handler := handlers.NewAuthHandler(svc, slog.New(slog.NewTextHandler(t.Output(), nil)))
	principal := seedPrincipal(t, principalRepo, "testlogin", password)
	t.Run("Sucessful login when valid credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", encodeBody(t, auth.LoginIn{
			Token:    principal.Username,
			Password: password,
		}))
		w := httptest.NewRecorder()
		handler.Login.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("Fail to login when wrong credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", encodeBody(t, auth.LoginIn{
			Token:    principal.Username,
			Password: "wrongpassword",
		}))
		w := httptest.NewRecorder()
		handler.Login.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("Successul registration when valid credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", encodeBody(t, auth.RegisterIn{
			Username: "newuser",
			Email:    "newuser@example.com",
			Password: "Password00!",
		}))
		w := httptest.NewRecorder()
		handler.Register.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		var found bool
		for _, c := range w.Result().Cookies() {
			if c.Name == "session" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected session cookie to be set")
		}
	})
	t.Run("Fail registration when username already registered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", encodeBody(t, auth.RegisterIn{
			Username: principal.Username,
			Email:    "testregister@example.com",
			Password: "Password00!",
		}))
		w := httptest.NewRecorder()
		handler.Register.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
		}
	})
}
