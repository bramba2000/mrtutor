package authhttp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (principal auth.Principal, err error)
}

func RequireSession(a Authenticator, logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := decodeSessionCookie(r)
			if err != nil {
				httpx.WriteError(w, r, err, logger)
				return
			}
			p, err := a.Authenticate(r.Context(), token)
			if err != nil {
				httpx.WriteError(w, r, err, logger)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
		})
	}
}
