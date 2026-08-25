package enrollments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bramba2000/mrtutor/backend/features/enrollments"
)

// fakeRepo is a configurable in-memory [enrollments.Repository]. Link mirrors
// the real upsert (LinkTutorStudent): linking an already-enrolled pair
// returns the existing row instead of creating a duplicate.
type fakeRepo struct {
	db     []enrollments.Enrollment
	nextID int

	GetByTutorIDFn func(context.Context, int) ([]enrollments.Enrollment, error)
	LinkFn         func(context.Context, int, int) (enrollments.Enrollment, error)

	GetByTutorIDCalled bool
	LinkCalled         bool
	GotGetByTutorID    int
	GotLinkTutorID     int
	GotLinkStudentID   int
}

func (r *fakeRepo) GetByTutorID(ctx context.Context, tutorID int) ([]enrollments.Enrollment, error) {
	r.GetByTutorIDCalled = true
	r.GotGetByTutorID = tutorID
	if r.GetByTutorIDFn != nil {
		return r.GetByTutorIDFn(ctx, tutorID)
	}
	var result []enrollments.Enrollment
	for _, e := range r.db {
		if e.TutorID == tutorID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *fakeRepo) Link(ctx context.Context, tutorID, studentID int) (enrollments.Enrollment, error) {
	r.LinkCalled = true
	r.GotLinkTutorID = tutorID
	r.GotLinkStudentID = studentID
	if r.LinkFn != nil {
		return r.LinkFn(ctx, tutorID, studentID)
	}
	for _, e := range r.db {
		if e.TutorID == tutorID && e.StudentID == studentID {
			return e, nil
		}
	}
	r.nextID++
	e := enrollments.Enrollment{ID: r.nextID, TutorID: tutorID, StudentID: studentID}
	r.db = append(r.db, e)
	return e, nil
}

var _ enrollments.Repository = (*fakeRepo)(nil)

func TestService(t *testing.T) {
	t.Run("GetByTutorID", func(t *testing.T) {
		t.Run("passes the returned enrollments through", func(t *testing.T) {
			repo := &fakeRepo{}
			svc := enrollments.NewService(repo)

			want := []enrollments.Enrollment{{ID: 1, TutorID: 5, StudentID: 9}}
			repo.GetByTutorIDFn = func(context.Context, int) ([]enrollments.Enrollment, error) {
				return want, nil
			}

			got, err := svc.GetByTutorID(t.Context(), 5)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != want[0] {
				t.Errorf("expected %+v, got %+v", want, got)
			}
			if repo.GotGetByTutorID != 5 {
				t.Errorf("expected tutorID 5 to reach the repository, got %d", repo.GotGetByTutorID)
			}
		})

		t.Run("propagates a repository error", func(t *testing.T) {
			repo := &fakeRepo{
				GetByTutorIDFn: func(context.Context, int) ([]enrollments.Enrollment, error) {
					return nil, errors.New("db is on fire")
				},
			}
			svc := enrollments.NewService(repo)

			_, err := svc.GetByTutorID(t.Context(), 5)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	})

	t.Run("Link", func(t *testing.T) {
		t.Run("enrolls a student with a tutor", func(t *testing.T) {
			repo := &fakeRepo{}
			svc := enrollments.NewService(repo)

			got, err := svc.Link(t.Context(), 5, 9)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.TutorID != 5 || got.StudentID != 9 {
				t.Errorf("expected TutorID 5 and StudentID 9, got %+v", got)
			}
			if repo.GotLinkTutorID != 5 || repo.GotLinkStudentID != 9 {
				t.Errorf("expected (5, 9) to reach the repository, got (%d, %d)", repo.GotLinkTutorID, repo.GotLinkStudentID)
			}
		})

		t.Run("linking an already-enrolled pair is a no-op", func(t *testing.T) {
			repo := &fakeRepo{}
			svc := enrollments.NewService(repo)

			first, err := svc.Link(t.Context(), 5, 9)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			second, err := svc.Link(t.Context(), 5, 9)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if first.ID != second.ID {
				t.Errorf("expected the same enrollment to be returned, got %+v and %+v", first, second)
			}
			if len(repo.db) != 1 {
				t.Errorf("expected exactly one enrollment row, got %d", len(repo.db))
			}
		})

		t.Run("propagates a repository error", func(t *testing.T) {
			repo := &fakeRepo{
				LinkFn: func(context.Context, int, int) (enrollments.Enrollment, error) {
					return enrollments.Enrollment{}, errors.New("db is on fire")
				},
			}
			svc := enrollments.NewService(repo)

			_, err := svc.Link(t.Context(), 5, 9)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	})
}
