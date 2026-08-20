package studentssqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Repository struct {
	R *gen.Queries
	W *gen.Queries
}

// Delete implements [students.Repository].
func (r Repository) Delete(ctx context.Context, id int) error {
	affected, err := r.W.DeleteStudent(ctx, int64(id))
	if err != nil {
		return sqlite.TranslateSQLError("students.Delete", err, students.ErrNotFound, nil)
	}
	if affected == 0 {
		return fmt.Errorf("students.Delete: %w", students.ErrNotFound)
	}
	return nil
}

// GetAll implements [students.Repository].
func (r Repository) GetAll(ctx context.Context) ([]students.Student, error) {
	got, err := r.R.GetAllStudents(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("students.GetAll", err, nil, nil)
	}
	result := make([]students.Student, len(got))
	for i, s := range got {
		result[i] = toStudent(s)
	}
	return result, nil
}

// GetDistinctSchools implements [students.Repository].
func (r Repository) GetDistinctSchools(ctx context.Context) ([]string, error) {
	got, err := r.R.GetDistinctSchools(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("students.GetDistinctSchools", err, nil, nil)
	}
	return toStrings(got), nil
}

// GetDistinctStudyPrograms implements [students.Repository].
func (r Repository) GetDistinctStudyPrograms(ctx context.Context) ([]string, error) {
	got, err := r.R.GetDistinctStudyPrograms(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("students.GetDistinctStudyPrograms", err, nil, nil)
	}
	return toStrings(got), nil
}

// GetDistinctClasses implements [students.Repository].
func (r Repository) GetDistinctClasses(ctx context.Context) ([]string, error) {
	got, err := r.R.GetDistinctClasses(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("students.GetDistinctClasses", err, nil, nil)
	}
	return toStrings(got), nil
}

// GetByID implements [students.Repository].
func (r Repository) GetByID(ctx context.Context, id int) (students.Student, error) {
	got, err := r.R.GetStudentById(ctx, int64(id))
	if err != nil {
		return students.Student{}, sqlite.TranslateSQLError("students.GetByID", err, students.ErrNotFound, nil)
	}
	return toStudent(got), nil
}

// Save implements [students.Repository].
func (r Repository) Save(ctx context.Context, student students.Student) (students.Student, error) {
	var birthDate time.Time
	if student.BirthDate != "" {
		var err error
		birthDate, err = time.Parse(time.DateOnly, student.BirthDate)
		if err != nil {
			return students.Student{}, fmt.Errorf("students.Save: %w: %w", errs.Invalid, err)
		}
	}

	model, err := r.W.SaveStudent(ctx, gen.SaveStudentParams{
		ID:           int64(student.ID),
		DisplayName:  student.DisplayName,
		Email:        sql.NullString{Valid: student.Email != "", String: student.Email},
		Phone:        sql.NullString{Valid: student.Phone != "", String: student.Phone},
		School:       sql.NullString{Valid: student.School != "", String: student.School},
		StudyProgram: sql.NullString{Valid: student.StudyProgram != "", String: student.StudyProgram},
		Class:        sql.NullString{Valid: student.Class != "", String: student.Class},
		BirthDate:    sql.NullTime{Valid: !birthDate.IsZero(), Time: birthDate},
	})

	if err != nil {
		return students.Student{}, sqlite.TranslateSQLError("students.Save", err, nil, nil)
	}
	return toStudent(model), nil
}

func NewRepository(r *sqlite.DB) *Repository {
	return &Repository{
		R: gen.New(r.R),
		W: gen.New(r.W),
	}
}

var _ students.Repository = Repository{}

// toStrings drops any non-valid (SQL NULL) entries; the queries already
// filter those out, so this only guards against an empty result set.
func toStrings(values []sql.NullString) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if v.Valid {
			result = append(result, v.String)
		}
	}
	return result
}

func toStudent(s gen.Student) students.Student {
	var birthDate string
	if s.BirthDate.Valid {
		birthDate = s.BirthDate.Time.Format(time.DateOnly)
	}
	return students.Student{
		ID:           int(s.ID),
		DisplayName:  s.DisplayName,
		Email:        s.Email.String,
		Phone:        s.Phone.String,
		School:       s.School.String,
		StudyProgram: s.StudyProgram.String,
		Class:        s.Class.String,
		BirthDate:    birthDate,
		CreatedAt:    s.CreatedAt,
		ModifiedAt:   s.ModifiedAt.Time,
	}
}
