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

var (
	ErrNotFound = errs.Domain("tutors.notFound", "Tutor not found", errs.NotFound)
)

type Repository interface {
	// GetByID returns a tutor by ID, or ErrNotFound if not found.
	GetByID(ctx context.Context, id int) (Tutor, error)
	// GetAll returns all tutors, or an empty slice if none found.
	GetAll(ctx context.Context) ([]Tutor, error)
	// Save saves a tutor. Try to save a tutor with an existing ID will update the tutor,
	// otherwise it will create a new tutor. CreatedAt and ModifiedAt will be set to the current time in UTC.
	Save(ctx context.Context, tutor Tutor) (Tutor, error)
	// Delete deletes a tutor by ID, or ErrNotFound if not found.
	Delete(ctx context.Context, id int) error
}
