package studentssqlite_test

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

func newRepo(t *testing.T) *studentssqlite.Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db := sqlitetest.OpenTemp(t)
	return studentssqlite.NewRepository(db)
}

func seedStudent(t *testing.T, repo students.Repository, displayName string) students.Student {
	t.Helper()
	age := rand.IntN(50)

	student := students.Student{
		DisplayName: displayName,
		BirthDate:   time.Now().UTC().AddDate(-age, 0, 0).Format(time.DateOnly),
		Email:       strings.ReplaceAll(displayName, " ", ".") + "@example.com",
	}

	saved, err := repo.Save(t.Context(), student)
	if err != nil {
		t.Fatalf("failed to seed student: %v", err)
	}
	return saved
}

func TestStudentRepository(t *testing.T) {
	t.Run("Save", func(t *testing.T) {
		t.Run("Successful create", func(t *testing.T) {
			repo := newRepo(t)
			student := students.Student{
				DisplayName:  "test",
				BirthDate:    time.Now().UTC().AddDate(-20, 0, 0).Format(time.DateOnly),
				Email:        "test@example.com",
				Phone:        "3334455666",
				School:       "Test school",
				StudyProgram: "Test student program",
				Class:        "Test class",
			}
			created, err := repo.Save(t.Context(), student)
			if err != nil {
				t.Fatalf("failed to create student: %v", err)
			}
			if created.ID == 0 {
				t.Fatalf("expected non-zero id, got %d", created.ID)
			}
			if created.CreatedAt.IsZero() {
				t.Errorf("expected CreatedAt to be set, got zero")
			}
			if !created.ModifiedAt.IsZero() {
				t.Errorf("expected ModifiedAt to be zero on create, got %v", created.ModifiedAt)
			}
			if created.DisplayName != student.DisplayName ||
				created.BirthDate != student.BirthDate ||
				created.Email != student.Email ||
				created.Phone != student.Phone ||
				created.School != student.School ||
				created.StudyProgram != student.StudyProgram ||
				created.Class != student.Class {
				t.Errorf("expected fields to round-trip, got %+v", created)
			}
		})

		t.Run("Successful update preserves CreatedAt", func(t *testing.T) {
			repo := newRepo(t)
			created := seedStudent(t, repo, "update me")

			updated, err := repo.Save(t.Context(), students.Student{
				ID:          created.ID,
				DisplayName: "updated name",
			})
			if err != nil {
				t.Fatalf("failed to update student: %v", err)
			}
			if updated.ID != created.ID {
				t.Errorf("expected id to be preserved, got %d, want %d", updated.ID, created.ID)
			}
			if updated.DisplayName != "updated name" {
				t.Errorf("expected DisplayName to be updated, got %q", updated.DisplayName)
			}
			if !updated.CreatedAt.Equal(created.CreatedAt) {
				t.Errorf("expected CreatedAt to be preserved, got %v, want %v", updated.CreatedAt, created.CreatedAt)
			}
			if updated.ModifiedAt.IsZero() {
				t.Errorf("expected ModifiedAt to be set after update")
			}
			if updated.ModifiedAt.Before(updated.CreatedAt) {
				t.Errorf("expected ModifiedAt (%v) not to be before CreatedAt (%v)", updated.ModifiedAt, updated.CreatedAt)
			}
		})

		t.Run("Save with unknown id creates a row with that id", func(t *testing.T) {
			repo := newRepo(t)
			saved, err := repo.Save(t.Context(), students.Student{
				ID:          12345,
				DisplayName: "upserted",
			})
			if err != nil {
				t.Fatalf("failed to save student: %v", err)
			}
			if saved.ID != 12345 {
				t.Errorf("expected id 12345, got %d", saved.ID)
			}
			got, err := repo.GetByID(t.Context(), 12345)
			if err != nil {
				t.Fatalf("failed to get created student: %v", err)
			}
			if got.DisplayName != "upserted" {
				t.Errorf("expected DisplayName 'upserted', got %q", got.DisplayName)
			}
		})

		t.Run("Empty BirthDate round-trips as empty string", func(t *testing.T) {
			repo := newRepo(t)
			saved, err := repo.Save(t.Context(), students.Student{DisplayName: "no birthdate"})
			if err != nil {
				t.Fatalf("failed to save student: %v", err)
			}
			if saved.BirthDate != "" {
				t.Errorf("expected empty BirthDate, got %q", saved.BirthDate)
			}
			got, err := repo.GetByID(t.Context(), saved.ID)
			if err != nil {
				t.Fatalf("failed to get student: %v", err)
			}
			if got.BirthDate != "" {
				t.Errorf("expected empty BirthDate after reload, got %q", got.BirthDate)
			}
		})

		t.Run("Malformed BirthDate is rejected as invalid", func(t *testing.T) {
			repo := newRepo(t)
			_, err := repo.Save(t.Context(), students.Student{
				DisplayName: "bad birthdate",
				BirthDate:   "not-a-date",
			})
			if !errors.Is(err, errs.Invalid) {
				t.Fatalf("expected errs.Invalid, got %v", err)
			}
		})

		t.Run("Empty optional fields round-trip as empty strings", func(t *testing.T) {
			repo := newRepo(t)
			saved, err := repo.Save(t.Context(), students.Student{DisplayName: "only name"})
			if err != nil {
				t.Fatalf("failed to save student: %v", err)
			}
			if saved.Email != "" || saved.Phone != "" || saved.School != "" || saved.StudyProgram != "" || saved.Class != "" {
				t.Errorf("expected optional fields to be empty, got %+v", saved)
			}
		})
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("Successful get student by id", func(t *testing.T) {
			repo := newRepo(t)
			student := seedStudent(t, repo, "test get student by id")
			got, err := repo.GetByID(t.Context(), student.ID)
			if err != nil {
				t.Fatalf("failed to get student by id: %v", err)
			}
			if got != student {
				t.Fatalf("\nwant:%+v\ngot :%+v", student, got)
			}
		})

		t.Run("Missing id returns ErrNotFound", func(t *testing.T) {
			repo := newRepo(t)
			_, err := repo.GetByID(t.Context(), 999999)
			if !errors.Is(err, students.ErrNotFound) {
				t.Errorf("expected students.ErrNotFound, got %v", err)
			}
			if !errors.Is(err, errs.NotFound) {
				t.Errorf("expected errs.NotFound, got %v", err)
			}
		})
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Run("Successful get all students", func(t *testing.T) {
			repo := newRepo(t)
			student1 := seedStudent(t, repo, "test get all students 1")
			student2 := seedStudent(t, repo, "test get all students 2")

			got, err := repo.GetAll(t.Context())
			if err != nil {
				t.Fatalf("failed to get all students: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("expected 2 students, got %d", len(got))
			}
			found1 := false
			found2 := false
			for _, s := range got {
				if s.ID == student1.ID {
					found1 = true
				}
				if s.ID == student2.ID {
					found2 = true
				}
			}
			if !found1 || !found2 {
				t.Fatalf("expected to find both seeded students in the result")
			}
		})

		t.Run("Empty returns empty slice", func(t *testing.T) {
			repo := newRepo(t)
			got, err := repo.GetAll(t.Context())
			if err != nil {
				t.Fatalf("failed to get all students: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("expected 0 students, got %d", len(got))
			}
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Successful delete", func(t *testing.T) {
			repo := newRepo(t)
			student := seedStudent(t, repo, "to delete")

			if err := repo.Delete(t.Context(), student.ID); err != nil {
				t.Fatalf("failed to delete student: %v", err)
			}
			_, err := repo.GetByID(t.Context(), student.ID)
			if !errors.Is(err, students.ErrNotFound) {
				t.Errorf("expected the student to be gone after delete, got %v", err)
			}
		})

		t.Run("Missing id returns ErrNotFound", func(t *testing.T) {
			repo := newRepo(t)
			err := repo.Delete(t.Context(), 999999)
			if !errors.Is(err, students.ErrNotFound) {
				t.Errorf("expected students.ErrNotFound, got %v", err)
			}
		})
	})
}
