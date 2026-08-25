package main

import (
	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/features/enrollments"
	"github.com/bramba2000/mrtutor/backend/features/enrollments/enrollmentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Services struct {
	Auth        auth.Service
	Students    students.Service
	Tutors      tutors.Service
	Enrollments enrollments.Service
}

func createServices(db *sqlite.DB) Services {
	authStorage := authsqlite.Build(db)
	auth := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)

	studentsStorage := studentssqlite.NewRepository(db)
	students := students.NewService(studentsStorage)

	tutorsStorage := tutorssqlite.NewRepository(db)
	tutors := tutors.NewService(tutorsStorage)

	enrollmentsStorage := enrollmentssqlite.NewRepository(db)
	enrollments := enrollments.NewService(enrollmentsStorage)

	return Services{
		Auth:        auth,
		Students:    students,
		Tutors:      tutors,
		Enrollments: enrollments,
	}
}
