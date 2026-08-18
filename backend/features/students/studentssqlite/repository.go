package studentssqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Repository struct {
	R *gen.Queries
	W *gen.Queries
}

// Create implements [students.Repository].
func (r Repository) Create(ctx context.Context, student students.Student) (int, error) {
	birthDate, err := time.Parse(time.DateOnly, student.BirthDate)
	if err != nil && student.BirthDate != "" {
		return 0, err
	}

	id, err := r.W.CreateStudent(ctx, gen.CreateStudentParams{
		DisplayName:  student.DisplayName,
		Email:        sql.NullString{String: student.Email, Valid: student.Email != ""},
		Phone:        sql.NullString{String: student.Phone, Valid: student.Phone != ""},
		School:       sql.NullString{String: student.School, Valid: student.School != ""},
		StudyProgram: sql.NullString{String: student.StudyProgram, Valid: student.StudyProgram != ""},
		Class:        sql.NullString{String: student.Class, Valid: student.Class != ""},
		BirthDate:    sql.NullTime{Time: birthDate, Valid: student.BirthDate != ""},
		CreatedAt:    student.CreatedAt,
	})
	return int(id), err
}

// Delete implements [students.Repository].
func (r Repository) Delete(ctx context.Context, id int) error {
	return r.W.DeleteStudent(ctx, int64(id))
}

// GetAll implements [students.Repository].
func (r Repository) GetAll(ctx context.Context) ([]students.Student, error) {
	got, err := r.R.GetAllStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]students.Student, len(got))
	for i, s := range got {
		result[i] = toStudent(s)
	}
	return result, nil
}

// GetByID implements [students.Repository].
func (r Repository) GetByID(ctx context.Context, id int) (students.Student, error) {
	got, err := r.R.GetStudentById(ctx, int64(id))
	if err != nil {
		return students.Student{}, err
	}
	return toStudent(got), nil
}

// Update implements [students.Repository].
func (r Repository) Update(ctx context.Context, student students.Student) (students.Student, error) {
	birthDate, err := time.Parse(time.DateOnly, student.BirthDate)
	if err != nil {
		return students.Student{}, err
	}

	model, err := r.W.UpdateStudent(ctx, gen.UpdateStudentParams{
		DisplayName:  student.DisplayName,
		Email:        sql.NullString{String: student.Email, Valid: student.Email != ""},
		Phone:        sql.NullString{String: student.Phone, Valid: student.Phone != ""},
		School:       sql.NullString{String: student.School, Valid: student.School != ""},
		StudyProgram: sql.NullString{String: student.StudyProgram, Valid: student.StudyProgram != ""},
		Class:        sql.NullString{String: student.Class, Valid: student.Class != ""},
		BirthDate:    sql.NullTime{Time: birthDate, Valid: student.BirthDate != ""},
		ModifiedAt:   sql.NullTime{Time: student.ModifiedAt, Valid: true},
		ID:           0,
	})
	if err != nil {
		return students.Student{}, err
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

func toStudent(s gen.Student) students.Student {
	return students.Student{
		ID:           int(s.ID),
		DisplayName:  s.DisplayName,
		Email:        s.Email.String,
		Phone:        s.Phone.String,
		School:       s.School.String,
		StudyProgram: s.StudyProgram.String,
		Class:        s.Class.String,
		BirthDate:    s.BirthDate.Time.Format(time.DateOnly),
		CreatedAt:    s.CreatedAt,
		ModifiedAt:   s.ModifiedAt.Time,
	}
}
