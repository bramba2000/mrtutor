package authhttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

// Package authhttp_test is external, so these tests only reach the exported
// surface — the whole point of moving the handlers out of cmd/api/http and
// behind the Service seam.

// fakeService lets each test control what the domain layer returns, without a
// database or bcrypt anywhere in the loop.
type fakeService struct {
	loginFn    func(context.Context, auth.LoginIn) (string, error)
	registerFn func(context.Context, auth.RegisterIn) (auth.RegisterOut, error)

	loginCalled, registerCalled bool
	gotLogin                    auth.LoginIn
	gotRegister                 auth.RegisterIn
}

func (f *fakeService) Login(ctx context.Context, in auth.LoginIn) (string, error) {
	f.loginCalled = true
	f.gotLogin = in
	return f.loginFn(ctx, in)
}

func (f *fakeService) Register(ctx context.Context, in auth.RegisterIn) (auth.RegisterOut, error) {
	f.registerCalled = true
	f.gotRegister = in
	return f.registerFn(ctx, in)
}

var (
	_ authhttp.Service = (*fakeService)(nil)
	_ authhttp.Service = auth.Service{}
	// Positive counterpart to the #17 fix: Wrap[auth.LoginIn]/Wrap[auth.RegisterIn]
	// only compile because both satisfy Validable.
	_ httpx.Validable = auth.LoginIn{}
	_ httpx.Validable = auth.RegisterIn{}
)

// errorBody mirrors httpx's unexported wire shape. Declared here rather than
// exported from httpx, because this test pins the JSON contract, not the Go
// type — an internal rename to httpx shouldn't have to touch this file.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fields  []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields,omitempty"`
}

func newJSONRequest(method, target string, body any) *http.Request {
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(method, target, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func findSessionCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

func mounted(t *testing.T, svc authhttp.Service, cfg authhttp.Config) *httpx.Router {
	t.Helper()
	r := httpx.NewRouter("")
	h := authhttp.NewHandler(svc, cfg, nil)
	h.Mount(r)
	return r
}

func TestLogin(t *testing.T) {
	t.Run("Success when credentials are valid", func(t *testing.T) {
		svc := &fakeService{
			loginFn: func(ctx context.Context, in auth.LoginIn) (string, error) {
				return "the-session-token", nil
			},
		}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/login", auth.LoginIn{
			Token: "alice", Password: "correct-password",
		}))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if w.Body.Len() != 0 {
			t.Errorf("expected an empty body, got %q", w.Body.String())
		}
		if svc.gotLogin.Token != "alice" || svc.gotLogin.Password != "correct-password" {
			t.Errorf("expected the decoded LoginIn to reach the service, got %+v", svc.gotLogin)
		}

		cookie := findSessionCookie(w)
		if cookie == nil {
			t.Fatal("expected a session cookie to be set")
		}
		if cookie.Value != "the-session-token" {
			t.Errorf("expected cookie value %q, got %q", "the-session-token", cookie.Value)
		}
		if !cookie.HttpOnly {
			t.Error("expected the cookie to be HttpOnly")
		}
		if cookie.Path != "/" {
			t.Errorf("expected cookie path %q, got %q", "/", cookie.Path)
		}
		if cookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("expected SameSite=Lax, got %v", cookie.SameSite)
		}
		// Direct regression guard for the nanoseconds bug: MaxAge used to be
		// int(7*24*time.Hour) = 604800000000000, not 604800 seconds.
		if cookie.MaxAge != 604800 {
			t.Errorf("expected MaxAge %d, got %d", 604800, cookie.MaxAge)
		}
		if cookie.Secure {
			t.Error("expected Secure to be unset for a zero-value Config")
		}
	})

	t.Run("Secure cookie is set when Config.Secure is true", func(t *testing.T) {
		svc := &fakeService{
			loginFn: func(ctx context.Context, in auth.LoginIn) (string, error) {
				return "tok", nil
			},
		}
		h := mounted(t, svc, authhttp.Config{Secure: true})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/login", auth.LoginIn{
			Token: "alice", Password: "correct-password",
		}))

		cookie := findSessionCookie(w)
		if cookie == nil {
			t.Fatal("expected a session cookie to be set")
		}
		if !cookie.Secure {
			t.Error("expected Secure to be set when Config.Secure is true")
		}
	})

	t.Run("Fail when credentials are invalid", func(t *testing.T) {
		svc := &fakeService{
			loginFn: func(ctx context.Context, in auth.LoginIn) (string, error) {
				return "", auth.ErrInvalidCredentials
			},
		}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/login", auth.LoginIn{
			Token: "alice", Password: "wrong",
		}))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if body.Code != "invalidCredentials" {
			t.Errorf("expected code %q, got %q", "invalidCredentials", body.Code)
		}
		if findSessionCookie(w) != nil {
			t.Error("expected no session cookie on a failed login")
		}
	})

	t.Run("Fail when the service returns an unclassified error", func(t *testing.T) {
		svc := &fakeService{
			loginFn: func(ctx context.Context, in auth.LoginIn) (string, error) {
				return "", errors.New("db is on fire")
			},
		}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/login", auth.LoginIn{
			Token: "alice", Password: "whatever",
		}))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "db is on fire") {
			t.Errorf("expected the error cause to be withheld, got body %q", w.Body.String())
		}
	})

	t.Run("Fail when Content-Type is missing", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc, authhttp.Config{})

		buf, _ := json.Marshal(auth.LoginIn{Token: "alice", Password: "whatever"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(buf))

		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnsupportedMediaType, w.Code, w.Body.String())
		}
		if svc.loginCalled {
			t.Error("expected the service not to be called")
		}
	})

	t.Run("Fail when the body is empty", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc, authhttp.Config{})

		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if body.Code != "invalid.json" {
			t.Errorf("expected code %q, got %q", "invalid.json", body.Code)
		}
	})

	t.Run("Fail when token and password are blank", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/login", auth.LoginIn{}))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.loginCalled {
			t.Error("expected the service not to be called when validation fails")
		}
		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		gotFields := map[string]bool{}
		for _, f := range body.Fields {
			gotFields[f.Field] = true
		}
		for _, want := range []string{"token", "password"} {
			if !gotFields[want] {
				t.Errorf("expected a field error for %q, got %+v", want, body.Fields)
			}
		}
	})
}

func TestRegister(t *testing.T) {
	validIn := auth.RegisterIn{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "Password00!",
	}

	t.Run("Success when the input is valid", func(t *testing.T) {
		svc := &fakeService{
			registerFn: func(ctx context.Context, in auth.RegisterIn) (auth.RegisterOut, error) {
				return auth.RegisterOut{
					Principal: auth.Principal{
						ID:           3,
						Username:     in.Username,
						Email:        in.Email,
						PasswordHash: []byte("$2a$10$notarealhash"),
					},
					SessionToken: "the-session-token",
				}, nil
			},
		}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/register", validIn))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
		}
		if svc.gotRegister != validIn {
			t.Errorf("expected the decoded RegisterIn to reach the service, got %+v", svc.gotRegister)
		}

		var got auth.Principal
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got.ID != 3 || got.Username != "newuser" || got.Email != "newuser@example.com" {
			t.Errorf("expected the created principal in the body, got %+v", got)
		}

		body := w.Body.String()
		for _, leaked := range []string{"passwordHash", "$2a$", "the-session-token"} {
			if strings.Contains(body, leaked) {
				t.Errorf("expected %q not to appear in the response body, got %q", leaked, body)
			}
		}

		if cookie := findSessionCookie(w); cookie == nil || cookie.Value != "the-session-token" {
			t.Errorf("expected a session cookie with the returned token, got %+v", cookie)
		}
	})

	t.Run("Fail when the username is already registered", func(t *testing.T) {
		svc := &fakeService{
			registerFn: func(ctx context.Context, in auth.RegisterIn) (auth.RegisterOut, error) {
				return auth.RegisterOut{}, auth.ErrConflictPrincipal
			},
		}
		h := mounted(t, svc, authhttp.Config{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/register", validIn))

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
		}
		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if body.Code != "principal.conflict" {
			t.Errorf("expected code %q, got %q", "principal.conflict", body.Code)
		}
		if findSessionCookie(w) != nil {
			t.Error("expected no session cookie on a failed registration")
		}
	})

	t.Run("Fail when the password is too weak", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc, authhttp.Config{})

		in := validIn
		in.Password = "weak"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/register", in))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.registerCalled {
			t.Error("expected the service not to be called when validation fails")
		}
		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		var sawPassword bool
		for _, f := range body.Fields {
			if f.Field == "password" {
				sawPassword = true
			}
		}
		if !sawPassword {
			t.Errorf("expected a field error for %q, got %+v", "password", body.Fields)
		}
	})

	t.Run("Fail when the email is malformed", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc, authhttp.Config{})

		in := validIn
		in.Email = "not-an-email"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/auth/register", in))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.registerCalled {
			t.Error("expected the service not to be called when validation fails")
		}
	})
}
