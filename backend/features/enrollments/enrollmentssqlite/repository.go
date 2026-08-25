package enrollmentssqlite

import (
	"context"

	"github.com/bramba2000/mrtutor/backend/features/enrollments"
	"github.com/bramba2000/mrtutor/backend/features/enrollments/enrollmentssqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Repository struct {
	R *gen.Queries
	W *gen.Queries
}

func NewRepository(r *sqlite.DB) *Repository {
	return &Repository{
		R: gen.New(r.R),
		W: gen.New(r.W),
	}
}

var _ enrollments.Repository = Repository{}

// GetByTutorID implements [enrollments.Repository].
func (r Repository) GetByTutorID(ctx context.Context, tutorID int) ([]enrollments.Enrollment, error) {
	got, err := r.R.GetEnrollmentsByTutorID(ctx, int64(tutorID))
	if err != nil {
		return nil, sqlite.TranslateSQLError("enrollments.GetByTutorID", err, nil, nil)
	}
	result := make([]enrollments.Enrollment, len(got))
	for i, v := range got {
		result[i] = toEnrollment(v)
	}
	return result, nil
}

// Link implements [enrollments.Repository].
func (r Repository) Link(ctx context.Context, tutorID, studentID int) (enrollments.Enrollment, error) {
	got, err := r.W.LinkTutorStudent(ctx, gen.LinkTutorStudentParams{
		TutorID:   int64(tutorID),
		StudentID: int64(studentID),
	})
	if err != nil {
		return enrollments.Enrollment{}, sqlite.TranslateSQLError("enrollments.Link", err, nil, nil)
	}
	return toEnrollment(got), nil
}

func toEnrollment(m gen.Enrollment) enrollments.Enrollment {
	return enrollments.Enrollment{
		ID:        int(m.ID),
		TutorID:   int(m.TutorID),
		StudentID: int(m.StudentID),
		CreatedAt: m.CreatedAt,
	}
}
