package students

import (
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/validation"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return Service{
		repo: repo,
	}
}

type StudentData struct {
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	School       string `json:"school"`
	StudyProgram string `json:"studyProgram"`
	Class        string `json:"class"`
	BirthDate    string `json:"birthDate"`
}

func validateStudentData(student StudentData) error {
	return validation.Errors{
		"displayName":  validation.Validate(student.DisplayName, validation.NotBlank, validation.MaxLength[string](256)),
		"email":        validation.Optional(student.Email, validation.NotBlank, validation.MaxLength[string](256), validation.Email),
		"phone":        validation.Optional(student.Phone, validation.NotBlank, validation.Phone),
		"birthday":     validation.Optional(student.BirthDate, validation.NotBlank, validation.Date),
		"school":       validation.Optional(student.School, validation.NotBlank, validation.MaxLength[string](256)),
		"studyProgram": validation.Optional(student.StudyProgram, validation.NotBlank, validation.MaxLength[string](256)),
		"class":        validation.Optional(student.Class, validation.NotBlank, validation.MaxLength[string](256)),
	}.Err()
}

type CreateIn StudentData

func (in CreateIn) Validate() error {
	return validateStudentData(StudentData(in))
}

// Create creates a new student and returns the created student with its ID.
func (s Service) Create(ctx context.Context, in CreateIn) (Student, error) {
	student := Student{
		DisplayName:  in.DisplayName,
		Email:        in.Email,
		Phone:        in.Phone,
		School:       in.School,
		StudyProgram: in.StudyProgram,
		Class:        in.Class,
		BirthDate:    in.BirthDate,
		ID:           0,
		CreatedAt:    time.Now().UTC(),
		ModifiedAt:   time.Time{},
	}

	id, err := s.repo.Create(ctx, student)
	if err != nil {
		return Student{}, err
	}

	student.ID = id
	return student, nil
}

// GetByID retrieves a student by its ID.
func (s Service) GetByID(ctx context.Context, id int) (Student, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll retrieves all students.
func (s Service) GetAll(ctx context.Context) ([]Student, error) {
	return s.repo.GetAll(ctx)
}

type UpdateIn struct {
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	School       string `json:"school"`
	StudyProgram string `json:"studyProgram"`
	Class        string `json:"class"`
	BirthDate    string `json:"birthDate"`
	ID           int    `json:"-"`
}

func (in UpdateIn) Validate() error {
	err := validation.Errors{
		"id": validation.Validate(in.ID, validation.Min(1)),
	}
	studentData := StudentData{
		DisplayName:  in.DisplayName,
		Email:        in.Email,
		Phone:        in.Phone,
		School:       in.School,
		StudyProgram: in.StudyProgram,
		Class:        in.Class,
		BirthDate:    in.BirthDate,
	}
	err.Merge(validation.Errors(validateStudentData(studentData).(validation.Errors)))
	return err.Err()
}

// Update updates an existing student and returns the updated student.
func (s Service) Update(ctx context.Context, in UpdateIn) (Student, error) {
	student := Student{
		ID:           in.ID,
		DisplayName:  in.DisplayName,
		Email:        in.Email,
		Phone:        in.Phone,
		School:       in.School,
		StudyProgram: in.StudyProgram,
		Class:        in.Class,
		BirthDate:    in.BirthDate,
		ModifiedAt:   time.Now().UTC(),
	}

	updated, err := s.repo.Update(ctx, student)
	if err != nil {
		return Student{}, err
	}

	return updated, nil
}

// Delete removes a student by its ID.
func (s Service) Delete(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	return nil
}
