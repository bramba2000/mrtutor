package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/httpx"
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

func seedSession(t testing.TB, repo auth.SessionStore, sessionToken string, principalID int) auth.Session {
	now := time.Now()
	session, err := repo.Create(context.Background(), auth.Session{
		TokenHash:  sha256.Sum256([]byte(sessionToken)),
		UserID:     principalID,
		CreatedAt:  now,
		LastSeenAt: now,
	})
	if err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}
	t.Cleanup(func() {
		repo.Delete(context.Background(), session.TokenHash)
	})
	return session
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

func authenticateRequest(t testing.TB, sessionStore auth.SessionStore, principal auth.Principal, req *http.Request) func(*http.Request) {
	t.Helper()
	sessionToken := t.Name()
	now := time.Now()

	session, err := sessionStore.Create(context.Background(), auth.Session{TokenHash: sha256.Sum256([]byte(sessionToken)), UserID: principal.ID, CreatedAt: now, LastSeenAt: now})
	if err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}
	t.Cleanup(func() {
		sessionStore.Delete(context.Background(), session.TokenHash)
	})

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionToken,
	})
	return func(r *http.Request) {
		r.AddCookie(&http.Cookie{
			Name:  "session",
			Value: sessionToken,
		})
	}
}

func TestAuth(t *testing.T) {
	skipIfNotIntegration(t)

	db := sqlitetest.OpenTemp(t)
	authStorage := authsqlite.Build(db)
	svc := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)
	logger := slog.New(slog.NewTextHandler(t.Output(), nil))

	router := httpx.NewRouter("")
	authHandler := authhttp.NewHandler(svc, authhttp.Config{Secure: false}, logger)
	authHandler.Mount(router)

	const password = "testpassword"
	principal := seedPrincipal(t, authStorage.PrincipalStore, "testlogin", password)

	const sessionToken = "testsessiontoken"
	seedSession(t, authStorage.SessionStore, sessionToken, principal.ID)

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
	t.Run("Unknown user and wrong password are indistinguishable", func(t *testing.T) {
		unknownUserReq := newJSONRequest(t, http.MethodPost, "/auth/login", auth.LoginIn{
			Token:    "does-not-exist",
			Password: "whatever",
		})
		wrongPasswordReq := newJSONRequest(t, http.MethodPost, "/auth/login", auth.LoginIn{
			Token:    principal.Username,
			Password: "wrongpassword",
		})

		unknownUserW := httptest.NewRecorder()
		router.ServeHTTP(unknownUserW, unknownUserReq)
		wrongPasswordW := httptest.NewRecorder()
		router.ServeHTTP(wrongPasswordW, wrongPasswordReq)

		if unknownUserW.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 for unknown user, got %d: %s", unknownUserW.Code, unknownUserW.Body.String())
		}
		if unknownUserW.Code != wrongPasswordW.Code {
			t.Errorf("expected identical status codes, got %d (unknown user) and %d (wrong password)", unknownUserW.Code, wrongPasswordW.Code)
		}
		if unknownUserW.Body.String() != wrongPasswordW.Body.String() {
			t.Errorf("expected byte-identical bodies, got %q (unknown user) and %q (wrong password)", unknownUserW.Body.String(), wrongPasswordW.Body.String())
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
	t.Run("Successful logout when authenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		authenticateRequest(t, authStorage.SessionStore, principal, req)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
		}
		sessionCookie(t, w)
		if w.Result().Cookies()[0].MaxAge != -1 {
			t.Errorf("expected the session cookie to be cleared, got MaxAge %d", w.Result().Cookies()[0].MaxAge)
		}
	})
	t.Run("Succcessful get current principal when authenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		authenticateRequest(t, authStorage.SessionStore, principal, req)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var got auth.Principal
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got.ID != principal.ID || got.Username != principal.Username || got.Email != principal.Email {
			t.Errorf("expected the current principal in the body, got %+v", got)
		}
	})
	t.Run("Fail to get current principal when not authenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("Fail to get current principal when session is idle-expired", func(t *testing.T) {
		const idleSessionToken = "idle-expired-session-token"
		now := time.Now()
		session, err := authStorage.SessionStore.Create(context.Background(), auth.Session{
			TokenHash:  sha256.Sum256([]byte(idleSessionToken)),
			UserID:     principal.ID,
			CreatedAt:  now,
			LastSeenAt: now.Add(-4 * time.Hour), // older than auth.SessionIdleTimeout (3h)
		})
		if err != nil {
			t.Fatalf("failed to seed session: %v", err)
		}
		t.Cleanup(func() {
			authStorage.SessionStore.Delete(context.Background(), session.TokenHash)
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: idleSessionToken})
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("Session lifecycle: logout invalidates the session for later requests", func(t *testing.T) {
		meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		addCookie := authenticateRequest(t, authStorage.SessionStore, principal, meReq)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, meReq)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 before logout, got %d: %s", w.Code, w.Body.String())
		}

		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		addCookie(logoutReq)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, logoutReq)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status 204 for logout, got %d: %s", w.Code, w.Body.String())
		}

		secondMeReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		addCookie(secondMeReq)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, secondMeReq)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 for the same session after logout, got %d: %s", w.Code, w.Body.String())
		}
	})
}
