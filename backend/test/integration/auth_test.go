package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/httpx"
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

// newJSONRequest builds a request with a JSON-encoded body and the
// Content-Type header bodyDecoder requires; a bare httptest.NewRequest sets
// no Content-Type and is rejected with 415 before the body is ever read.
func newJSONRequest(t testing.TB, method, target string, body any) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, encodeBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func sessionCookie(t testing.TB, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("expected a session cookie to be set")
	return nil
}

func TestAuth(t *testing.T) {
	skipIfNotIntegration(t)

	db := sqlitetest.OpenTemp(t)
	principalRepo := sqlite.NewPrincipalStore(db)
	svc := auth.NewService(principalRepo, sqlite.NewSessionStore(db))
	logger := slog.New(slog.NewTextHandler(t.Output(), nil))

	router := httpx.NewRouter("")
	authHandler := authhttp.NewHandler(svc, authhttp.Config{Secure: false}, logger)
	authHandler.Mount(router)

	const password = "testpassword"
	principal := seedPrincipal(t, principalRepo, "testlogin", password)

	t.Run("Sucessful login when valid credentials", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/auth/login", auth.LoginIn{
			Token:    principal.Username,
			Password: password,
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		cookie := sessionCookie(t, w)
		if !cookie.HttpOnly {
			t.Error("expected the session cookie to be HttpOnly")
		}
		if cookie.Path != "/" {
			t.Errorf("expected cookie path %q, got %q", "/", cookie.Path)
		}
		if cookie.MaxAge != 604800 {
			t.Errorf("expected cookie MaxAge %d (7 days in seconds), got %d", 604800, cookie.MaxAge)
		}
	})
	t.Run("Fail to login when wrong credentials", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/auth/login", auth.LoginIn{
			Token:    principal.Username,
			Password: "wrongpassword",
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("Successul registration when valid credentials", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/auth/register", auth.RegisterIn{
			Username: "newuser",
			Email:    "newuser@example.com",
			Password: "Password00!",
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		var got auth.Principal
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got.Username != "newuser" || got.Email != "newuser@example.com" {
			t.Errorf("expected the created principal in the body, got %+v", got)
		}
		if strings.Contains(w.Body.String(), "passwordHash") {
			t.Errorf("expected the password hash not to be in the response, got %s", w.Body.String())
		}

		cookie := sessionCookie(t, w)
		if !cookie.HttpOnly {
			t.Error("expected the session cookie to be HttpOnly")
		}
		if cookie.Path != "/" {
			t.Errorf("expected cookie path %q, got %q", "/", cookie.Path)
		}
		if cookie.MaxAge != 604800 {
			t.Errorf("expected cookie MaxAge %d (7 days in seconds), got %d", 604800, cookie.MaxAge)
		}
	})
	t.Run("Fail registration when username already registered", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/auth/register", auth.RegisterIn{
			Username: principal.Username,
			Email:    "testregister@example.com",
			Password: "Password00!",
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
		}
	})
}
