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

// Create implements [tutors.Repository].
func (r Repository) Create(ctx context.Context, t tutors.TutorFields) (tutors.Tutor, error) {
	got, err := r.W.CreateTutor(ctx, gen.CreateTutorParams{
		DisplayName: t.DisplayName,
		Email:       sql.NullString{String: t.Email, Valid: t.Email != ""},
		Phone:       sql.NullString{String: t.Phone, Valid: t.Phone != ""},
		AboutMe:     sql.NullString{String: t.AboutMe, Valid: t.AboutMe != ""},
	})
	if err != nil {
		return tutors.Tutor{}, sqlite.TranslateSQLError("tutors.Create", err, nil, nil)
	}
	return toTutor(got), nil
}

// Update implements [tutors.Repository].
func (r Repository) Update(ctx context.Context, id int, t tutors.TutorFields) (tutors.Tutor, error) {
	got, err := r.W.UpdateTutor(ctx, gen.UpdateTutorParams{
		ID:          int64(id),
		DisplayName: t.DisplayName,
		Email:       sql.NullString{String: t.Email, Valid: t.Email != ""},
		Phone:       sql.NullString{String: t.Phone, Valid: t.Phone != ""},
		AboutMe:     sql.NullString{String: t.AboutMe, Valid: t.AboutMe != ""},
	})
	if err != nil {
		return tutors.Tutor{}, sqlite.TranslateSQLError("tutors.Update", err, tutors.ErrNotFound, nil)
	}
	return toTutor(got), nil
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
