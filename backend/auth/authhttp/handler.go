package authhttp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

// Service is the slice of the auth service the HTTP layer uses. Declared here
// rather than depending on auth.Service directly, mirroring the convention
// auth.PrincipalStore already establishes — and this is what makes these
// handlers unit-testable against a fake.
//
// It declares only what auth.Service implements today. Phase 7 adds
// Authenticate and Logout, here and in the fake.
type Service interface {
	Login(ctx context.Context, in auth.LoginIn) (string, error)
	Register(ctx context.Context, in auth.RegisterIn) (auth.RegisterOut, error)
	Logout(ctx context.Context, sessionToken string) error
	Authenticate(ctx context.Context, sessionToken string) (auth.Principal, error)
}

var _ Service = auth.Service{}

// Handler holds the auth endpoints. Exported: NewHandler previously returned
// an unexported type, which callers could not name in a signature or field.
type Handler struct {
	svc    Service
	logger *slog.Logger
	cfg    Config
}

func (h Handler) Mount(r *httpx.Router) {
	requireSession := RequireSession(h.svc, h.logger)

	r.Handle("POST /auth/login", httpx.Wrap(
		httpx.BodyDecoder[auth.LoginIn],
		h.svc.Login,
		func(w http.ResponseWriter, out string) error {
			encodeSessionCookie(w, out, h.cfg)
			w.WriteHeader(http.StatusOK)
			return nil
		},
		h.logger,
	))
	r.Handle("POST /auth/register", httpx.Wrap(
		httpx.BodyDecoder[auth.RegisterIn],
		h.svc.Register,
		func(w http.ResponseWriter, out auth.RegisterOut) error {
			encodeSessionCookie(w, out.SessionToken, h.cfg)
			return httpx.Created(w, out.Principal)
		},
		h.logger,
	))
	r.Handle("POST /auth/logout", httpx.WrapUnvalidated(
		func(r *http.Request) (string, error) {
			session, _ := decodeSessionCookie(r)
			return session, nil
		},
		func(ctx context.Context, sessionToken string) (struct{}, error) {
			return struct{}{}, h.svc.Logout(ctx, sessionToken)
		},
		func(w http.ResponseWriter, out struct{}) error {
			// Clear the session cookie by setting it to an empty value and
			// MaxAge=0, which tells the browser to delete it.
			http.SetCookie(w, &http.Cookie{
				Name:   sessionCookieName,
				Value:  "",
				MaxAge: -1,
			})
			return httpx.NoContent(w, out)
		},
		h.logger,
	))
	r.Handle("GET /auth/me", requireSession(httpx.WrapNoInput(
		func(ctx context.Context) (auth.Principal, error) {
			principal, ok := auth.FromContext(ctx)
			if !ok {
				return auth.Principal{}, auth.ErrUnauthenticated
			}
			return principal, nil
		},
		httpx.OK,
		h.logger,
	)))
}

// DefaultCookieMaxAge is the session cookie lifetime used when Config.MaxAge
// is left at its zero value.
const DefaultCookieMaxAge = 7 * 24 * time.Hour

const sessionCookieName = "session"

// Config configures the auth handlers' session cookie.
type Config struct {
	// Secure marks the session cookie Secure, so browsers withhold it over
	// plain HTTP. Set for production; leave unset for local development,
	// which typically runs over http:// on a LAN host (browsers treat
	// http://localhost as trustworthy regardless of this flag).
	Secure bool
	// MaxAge is the session cookie lifetime. Zero means DefaultCookieMaxAge.
	MaxAge time.Duration
}

func (c Config) maxAge() time.Duration {
	if c.MaxAge == 0 {
		return DefaultCookieMaxAge
	}
	return c.MaxAge
}

func encodeSessionCookie(w http.ResponseWriter, token string, cfg Config) {
	http.SetCookie(w, new(http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cfg.maxAge().Seconds()),
	}))
}

func decodeSessionCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", auth.ErrUnauthenticated
	}
	return cookie.Value, nil
}

func NewHandler(svc Service, cfg Config, logger *slog.Logger) Handler {
	return Handler{
		svc:    svc,
		logger: logger,
		cfg:    cfg,
	}
}
