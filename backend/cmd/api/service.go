package main

import (
	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Services struct {
	Auth     auth.Service
	Students students.Service
}

func createServices(db *sqlite.DB) Services {
	authStorage := authsqlite.Build(db)
	auth := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)

	studentsStorage := studentssqlite.NewRepository(db)
	students := students.NewService(studentsStorage)

	return Services{
		Auth:     auth,
		Students: students,
	}
}
