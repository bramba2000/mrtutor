package studentssqlite_test

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

func seedStudent(t *testing.T, repo students.Repository, displayName string) students.Student {
	age := rand.IntN(50)

	student := students.Student{
		DisplayName: displayName,
		CreatedAt:   time.Now().UTC(),
		BirthDate:   time.Now().UTC().AddDate(-age, 0, 0).Format(time.DateOnly),
		Email:       strings.ReplaceAll(displayName, " ", ".") + "@example.com",
	}

	id, err := repo.Create(t.Context(), student)
	if err != nil {
		t.Fatalf("failed to seed student: %v", err)
	}
	t.Cleanup(func() {
		err := repo.Delete(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to cleanup student: %v", err)
		}
	})
	student.ID = id
	return student
}

func TestStudentRepository(t *testing.T) {
	db := sqlitetest.OpenTemp(t)
	repo := studentssqlite.NewRepository(db)

	t.Run("Successful create student", func(t *testing.T) {
		student := students.Student{
			DisplayName:  "test",
			CreatedAt:    time.Now().UTC(),
			BirthDate:    time.Now().UTC().AddDate(-20, 0, 0).Format(time.DateOnly),
			Email:        "test@example.com",
			Phone:        "3334455666",
			School:       "Test school",
			StudyProgram: "Test student program",
			Class:        "Test class",
		}
		id, err := repo.Create(t.Context(), student)
		if err != nil {
			t.Fatalf("failed to create student: %v", err)
		}
		if id == 0 {
			t.Fatalf("expected non-zero id, got %d", id)
		}
		t.Cleanup(func() {
			err := repo.Delete(context.Background(), id)
			if err != nil {
				t.Fatalf("failed to cleanup student: %v", err)
			}
		})
	})

	t.Run("Successful get student by id", func(t *testing.T) {
		student := seedStudent(t, repo, "test get student by id")
		got, err := repo.GetByID(t.Context(), student.ID)
		if err != nil {
			t.Fatalf("failed to get student by id: %v", err)
		}
		if got != student {
			t.Fatalf("\nwant:%+v\ngot :%+v", student, got)
		}
	})

	t.Run("Successful get all students", func(t *testing.T) {
		student1 := seedStudent(t, repo, "test get all students 1")
		student2 := seedStudent(t, repo, "test get all students 2")

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("failed to get all students: %v", err)
		}
		if len(got) < 2 {
			t.Fatalf("expected at least 2 students, got %d", len(got))
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

	t.Run("Successful get all students when empty", func(t *testing.T) {
		db.W.Exec("DELETE FROM STUDENTS")
		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("failed to get all students: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 students, got %d", len(got))
		}
	})
}
