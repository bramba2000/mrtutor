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
	BirthDate string `json:"birthday"`

	CreatedAt  time.Time `json:"create_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

var (
	ErrNotFound = errs.Domain("students.notFound", "Student not found", errs.NotFound)
)

type Repository interface {
	// GetByID returns a student by ID, or ErrStudentNotFound if not found.
	GetByID(ctx context.Context, id int) (Student, error)
	// GetAll returns all students, or an empty slice if none found.
	GetAll(ctx context.Context) ([]Student, error)
	// Create creates a new student and returns the created student with ID.
	Create(ctx context.Context, student Student) (int, error)
	// Update updates an existing student by its ID, or ErrStudentNotFound if not found.
	Update(ctx context.Context, student Student) (Student, error)
	// Delete deletes a student by ID, or ErrStudentNotFound if not found.
	Delete(ctx context.Context, id int) error
}
