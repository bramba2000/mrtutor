package tutorssqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Repository struct {
	R *gen.Queries
	W *gen.Queries
}

// Delete implements [tutors.Repository].
func (r Repository) Delete(ctx context.Context, id int) error {
	affected, err := r.W.DeleteTutor(ctx, int64(id))
	if err != nil {
		return sqlite.TranslateSQLError("tutors.Delete", err, tutors.ErrNotFound, nil)
	}
	if affected == 0 {
		return fmt.Errorf("tutors.Delete: %w", tutors.ErrNotFound)
	}
	return nil
}

// GetAll implements [tutors.Repository].
func (r Repository) GetAll(ctx context.Context) ([]tutors.Tutor, error) {
	got, err := r.R.GetAllTutors(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("tutors.GetAll", err, nil, nil)
	}
	result := make([]tutors.Tutor, len(got))
	for i, t := range got {
		result[i] = toTutor(t)
	}
	return result, nil
}

// GetByID implements [tutors.Repository].
func (r Repository) GetByID(ctx context.Context, id int) (tutors.Tutor, error) {
	got, err := r.R.GetTutorById(ctx, int64(id))
	if err != nil {
		return tutors.Tutor{}, sqlite.TranslateSQLError("tutors.GetByID", err, tutors.ErrNotFound, nil)
	}
	return toTutor(got), nil
}

// Save implements [tutors.Repository].
func (r Repository) Save(ctx context.Context, tutor tutors.Tutor) (tutors.Tutor, error) {
	model, err := r.W.SaveTutor(ctx, gen.SaveTutorParams{
		ID:          int64(tutor.ID),
		DisplayName: tutor.DisplayName,
		Email:       sql.NullString{Valid: tutor.Email != "", String: tutor.Email},
		Phone:       sql.NullString{Valid: tutor.Phone != "", String: tutor.Phone},
		AboutMe:     sql.NullString{Valid: tutor.AboutMe != "", String: tutor.AboutMe},
	})

	if err != nil {
		return tutors.Tutor{}, sqlite.TranslateSQLError("tutors.Save", err, nil, nil)
	}
	return toTutor(model), nil
}

func NewRepository(r *sqlite.DB) *Repository {
	return &Repository{
		R: gen.New(r.R),
		W: gen.New(r.W),
	}
}

var _ tutors.Repository = Repository{}

func toTutor(t gen.Tutor) tutors.Tutor {
	return tutors.Tutor{
		ID:          int(t.ID),
		DisplayName: t.DisplayName,
		Email:       t.Email.String,
		Phone:       t.Phone.String,
		AboutMe:     t.AboutMe.String,
		CreatedAt:   t.CreatedAt,
		ModifiedAt:  t.ModifiedAt.Time,
	}
}
