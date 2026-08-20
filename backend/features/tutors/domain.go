package tutors

import (
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
)

type Tutor struct {
	ID          int    `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AboutMe     string `json:"aboutMe"`

	CreatedAt  time.Time `json:"createdAt"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type TutorFields struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AboutMe     string `json:"aboutMe"`
}

var (
	ErrNotFound = errs.Domain("tutors.notFound", "Tutor not found", errs.NotFound)
)

type Repository interface {
	// GetByID returns a tutor by ID, or ErrNotFound if not found.
	GetByID(ctx context.Context, id int) (Tutor, error)
	// GetAll returns all tutors, or an empty slice if none found.
	GetAll(ctx context.Context) ([]Tutor, error)
	// Create creates a new tutor and returns the created tutor with ID set.
	Create(ctx context.Context, tutor TutorFields) (Tutor, error)
	// Update updates a tutor by ID, or ErrNotFound if not found.
	Update(ctx context.Context, id int, tutor TutorFields) (Tutor, error)
	// Delete deletes a tutor by ID, or ErrNotFound if not found.
	Delete(ctx context.Context, id int) error
}
