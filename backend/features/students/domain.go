package students

import (
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
)

type Student struct {
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	School       string `json:"school"`
	StudyProgram string `json:"studyProgram"`
	Class        string `json:"class"`
	// BirthDate is in the format of YYYY-MM-DD
	BirthDate string `json:"birthDate"`

	CreatedAt  time.Time `json:"createdAt"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

var (
	ErrNotFound = errs.Domain("students.notFound", "Student not found", errs.NotFound)
)

type Repository interface {
	// GetByID returns a student by ID, or ErrStudentNotFound if not found.
	GetByID(ctx context.Context, id int) (Student, error)
	// GetAll returns all students, or an empty slice if none found.
	GetAll(ctx context.Context) ([]Student, error)
	// Save saves a student. Try to save a student with an existing ID will update the student,
	// otherwise it will create a new student. CreatedAt and ModifiedAt will be set to the current time in UTC.
	Save(ctx context.Context, student Student) (Student, error)
	// Delete deletes a student by ID, or ErrStudentNotFound if not found.
	Delete(ctx context.Context, id int) error
}
