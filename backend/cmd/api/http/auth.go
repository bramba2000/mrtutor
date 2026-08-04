package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bramba2000/mrtutor/backend/auth"
)

type authHandler struct {
	Login    http.HandlerFunc
	Register http.HandlerFunc
}

const sessionCookieName = "session"
const sessionCookieMaxAge = 7 * 24 * time.Hour // 7 days

func encodeSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, new(http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		HttpOnly: true,
		MaxAge:   int(sessionCookieMaxAge),
	}))
}

func NewAuthHandler(svc auth.Service, logger *slog.Logger) authHandler {
	return authHandler{
		Login: wrap(
			bodyDecoder[auth.LoginIn],
			svc.Login,
			func(w http.ResponseWriter, out string) error {
				encodeSessionCookie(w, out)
				w.WriteHeader(http.StatusOK)
				return nil
			},
			logger,
		),
		Register: wrap(
			bodyDecoder[auth.RegisterIn],
			svc.Register,
			func(w http.ResponseWriter, out auth.RegisterOut) error {
				encodeSessionCookie(w, out.SessionToken)
				return noContent(w, struct{}{})
			},
			logger,
		),
	}
}
