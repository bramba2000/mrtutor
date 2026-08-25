package enrollmentssqlite_test

import (
	"testing"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/features/enrollments/enrollmentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

// testDB bundles a temp sqlite DB with the enrollments repository under test
// plus the tutors/students/principal repositories needed to seed foreign keys.
type testDB struct {
	enrollments *enrollmentssqlite.Repository
	tutors      *tutorssqlite.Repository
	students    *studentssqlite.Repository
	principals  auth.PrincipalStore
}

func newTestDB(t *testing.T) testDB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db := sqlitetest.OpenTemp(t)
	return testDB{
		enrollments: enrollmentssqlite.NewRepository(db),
		tutors:      tutorssqlite.NewRepository(db),
		students:    studentssqlite.NewRepository(db),
		principals:  authsqlite.Build(db).PrincipalStore,
	}
}

// seedTutor creates a tutor owned by a freshly seeded user. username
// distinguishes the backing user across multiple calls in the same test.
func seedTutor(t *testing.T, db testDB, username string) tutors.Tutor {
	t.Helper()
	principal, err := db.principals.Create(t.Context(), auth.Principal{
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: []byte("passwordHash"),
	})
	if err != nil {
		t.Fatalf("failed to seed principal: %v", err)
	}
	tutor, err := db.tutors.Create(t.Context(), tutors.TutorFields{DisplayName: "tutor"}, principal.ID)
	if err != nil {
		t.Fatalf("failed to seed tutor: %v", err)
	}
	return tutor
}

func seedStudent(t *testing.T, repo *studentssqlite.Repository, displayName string) students.Student {
	t.Helper()
	student, err := repo.Save(t.Context(), students.Student{DisplayName: displayName})
	if err != nil {
		t.Fatalf("failed to seed student: %v", err)
	}
	return student
}

func TestEnrollmentRepository(t *testing.T) {
	t.Run("Link", func(t *testing.T) {
		t.Run("Successful create", func(t *testing.T) {
			db := newTestDB(t)
			tutor := seedTutor(t, db, "tutor1")
			student := seedStudent(t, db.students, "John")

			got, err := db.enrollments.Link(t.Context(), tutor.ID, student.ID)
			if err != nil {
				t.Fatalf("failed to link tutor and student: %v", err)
			}
			if got.ID == 0 {
				t.Errorf("expected a non-zero id, got 0")
			}
			if got.TutorID != tutor.ID || got.StudentID != student.ID {
				t.Errorf("expected TutorID %d and StudentID %d, got %+v", tutor.ID, student.ID, got)
			}
			if got.CreatedAt.IsZero() {
				t.Errorf("expected CreatedAt to be set")
			}
		})

		t.Run("Linking an already-enrolled pair returns the existing row", func(t *testing.T) {
			db := newTestDB(t)
			tutor := seedTutor(t, db, "tutor1")
			student := seedStudent(t, db.students, "John")

			first, err := db.enrollments.Link(t.Context(), tutor.ID, student.ID)
			if err != nil {
				t.Fatalf("failed to link tutor and student: %v", err)
			}
			second, err := db.enrollments.Link(t.Context(), tutor.ID, student.ID)
			if err != nil {
				t.Fatalf("failed to re-link tutor and student: %v", err)
			}
			if first.ID != second.ID {
				t.Errorf("expected the same enrollment id, got %d and %d", first.ID, second.ID)
			}

			got, err := db.enrollments.GetByTutorID(t.Context(), tutor.ID)
			if err != nil {
				t.Fatalf("failed to get enrollments by tutor id: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected exactly one enrollment, got %d", len(got))
			}
		})
	})

	t.Run("GetByTutorID", func(t *testing.T) {
		t.Run("Returns every enrollment for the tutor", func(t *testing.T) {
			db := newTestDB(t)
			tutor := seedTutor(t, db, "tutor1")
			otherTutor := seedTutor(t, db, "tutor2")
			student1 := seedStudent(t, db.students, "John")
			student2 := seedStudent(t, db.students, "Jane")

			if _, err := db.enrollments.Link(t.Context(), tutor.ID, student1.ID); err != nil {
				t.Fatalf("failed to link: %v", err)
			}
			if _, err := db.enrollments.Link(t.Context(), tutor.ID, student2.ID); err != nil {
				t.Fatalf("failed to link: %v", err)
			}
			if _, err := db.enrollments.Link(t.Context(), otherTutor.ID, student1.ID); err != nil {
				t.Fatalf("failed to link: %v", err)
			}

			got, err := db.enrollments.GetByTutorID(t.Context(), tutor.ID)
			if err != nil {
				t.Fatalf("failed to get enrollments by tutor id: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("expected 2 enrollments, got %d", len(got))
			}
		})

		t.Run("Empty returns empty slice", func(t *testing.T) {
			db := newTestDB(t)
			tutor := seedTutor(t, db, "tutor1")

			got, err := db.enrollments.GetByTutorID(t.Context(), tutor.ID)
			if err != nil {
				t.Fatalf("failed to get enrollments by tutor id: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("expected 0 enrollments, got %d", len(got))
			}
		})
	})
}
