package tutors

import (
	"context"

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

type TutorData struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AboutMe     string `json:"aboutMe"`
}

func validateTutorData(tutor TutorData) validation.Errors {
	return validation.Errors{
		"displayName": validation.Validate(tutor.DisplayName, validation.NotBlank, validation.MaxLength[string](256)),
		"email":       validation.Optional(tutor.Email, validation.NotBlank, validation.MaxLength[string](256), validation.Email),
		"phone":       validation.Optional(tutor.Phone, validation.NotBlank, validation.Phone),
		"aboutMe":     validation.Optional(tutor.AboutMe, validation.NotBlank, validation.MaxLength[string](2000)),
	}
}

type CreateIn TutorData

func (in CreateIn) Validate() error {
	return validateTutorData(TutorData(in)).Err()
}

// Create creates a new tutor and returns the created tutor with its ID.
func (s Service) Create(ctx context.Context, in CreateIn) (Tutor, error) {
	tutor := Tutor{
		DisplayName: in.DisplayName,
		Email:       in.Email,
		Phone:       in.Phone,
		AboutMe:     in.AboutMe,
		ID:          0,
	}

	return s.repo.Save(ctx, tutor)
}

// GetByID retrieves a tutor by its ID.
func (s Service) GetByID(ctx context.Context, id int) (Tutor, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll retrieves all tutors.
func (s Service) GetAll(ctx context.Context) ([]Tutor, error) {
	return s.repo.GetAll(ctx)
}

type UpdateIn struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AboutMe     string `json:"aboutMe"`
	ID          int    `json:"-"`
}

func (in UpdateIn) Validate() error {
	err := validation.Errors{
		"id": validation.Validate(in.ID, validation.Min(1)),
	}
	tutorData := TutorData{
		DisplayName: in.DisplayName,
		Email:       in.Email,
		Phone:       in.Phone,
		AboutMe:     in.AboutMe,
	}
	err.Merge(validateTutorData(tutorData))
	return err.Err()
}

// Update updates an existing tutor and returns the updated tutor.
func (s Service) Update(ctx context.Context, in UpdateIn) (Tutor, error) {
	tutor := Tutor{
		DisplayName: in.DisplayName,
		Email:       in.Email,
		Phone:       in.Phone,
		AboutMe:     in.AboutMe,
		ID:          in.ID,
	}
	return s.repo.Save(ctx, tutor)
}

// Delete removes a tutor by its ID.
func (s Service) Delete(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	return nil
}
