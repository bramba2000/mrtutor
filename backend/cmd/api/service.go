package main

import (
	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Services struct {
	Auth auth.Service
}

func createServices(db *sqlite.DB) Services {
	principalStore := sqlite.NewPrincipalStore(db)
	sessionStore := sqlite.NewSessionStore(db)
	auth := auth.NewService(principalStore, sessionStore)

	return Services{
		Auth: auth,
	}
}
