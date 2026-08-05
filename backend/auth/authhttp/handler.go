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
}

var _ Service = auth.Service{}

// Handler holds the auth endpoints. Exported: NewHandler previously returned
// an unexported type, which callers could not name in a signature or field.
type Handler struct {
	Login    http.HandlerFunc
	Register http.HandlerFunc
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

func encodeSessionCookie(w http.ResponseWriter, cfg Config, token string) {
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

func NewHandler(svc Service, cfg Config, logger *slog.Logger) Handler {
	return Handler{
		Login: httpx.Wrap(
			httpx.BodyDecoder[auth.LoginIn],
			svc.Login,
			func(w http.ResponseWriter, out string) error {
				encodeSessionCookie(w, cfg, out)
				w.WriteHeader(http.StatusOK)
				return nil
			},
			logger,
		),
		Register: httpx.Wrap(
			httpx.BodyDecoder[auth.RegisterIn],
			svc.Register,
			func(w http.ResponseWriter, out auth.RegisterOut) error {
				encodeSessionCookie(w, cfg, out.SessionToken)
				return httpx.Created(w, out.Principal)
			},
			logger,
		),
	}
}
