package enrollments

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return Service{repo: repo}
}

// GetByTutorID retrieves every enrollment for the given tutor.
func (s Service) GetByTutorID(ctx context.Context, tutorID int) ([]Enrollment, error) {
	return s.repo.GetByTutorID(ctx, tutorID)
}

// Link enrolls a student with a tutor. Linking an already-enrolled pair is a
// no-op that returns the existing enrollment.
func (s Service) Link(ctx context.Context, tutorID, studentID int) (Enrollment, error) {
	return s.repo.Link(ctx, tutorID, studentID)
}
