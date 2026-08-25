package enrollments

import (
	"context"
	"time"
)

// Enrollment records that a student is enrolled with a tutor. The pair
// (TutorID, StudentID) is unique — enrolling an already-enrolled pair is a
// no-op, not a duplicate row.
type Enrollment struct {
	ID        int       `json:"id"`
	TutorID   int       `json:"tutorId"`
	StudentID int       `json:"studentId"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository interface {
	// GetByTutorID returns every enrollment linking the given tutor to a student.
	GetByTutorID(ctx context.Context, tutorID int) ([]Enrollment, error)
	// Link enrolls a student with a tutor. Linking an already-enrolled pair
	// is a no-op that returns the existing enrollment.
	Link(ctx context.Context, tutorID, studentID int) (Enrollment, error)
}
