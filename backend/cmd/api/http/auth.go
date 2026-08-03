package http

import (
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/auth"
)

type authHandler struct {
	Login http.HandlerFunc
}

const sessionCookieName = "session"

func NewAuthHandler(svc auth.Service, logger *slog.Logger) authHandler {
	return authHandler{
		Login: wrap(
			bodyDecoder[auth.LoginIn],
			svc.Login,
			func(w http.ResponseWriter, out string) error {
				http.SetCookie(w, new(http.Cookie{
					Name:     sessionCookieName,
					Value:    out,
					HttpOnly: true,
				}))
				w.WriteHeader(http.StatusOK)
				return nil
			},
			logger,
		),
	}
}
