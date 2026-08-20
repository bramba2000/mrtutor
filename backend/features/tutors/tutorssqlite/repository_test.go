package tutorssqlite_test

import (
	"errors"
	"testing"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

func newRepo(t *testing.T) *tutorssqlite.Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db := sqlitetest.OpenTemp(t)
	return tutorssqlite.NewRepository(db)
}

func seedTutor(t *testing.T, repo tutors.Repository, displayName string) tutors.Tutor {
	t.Helper()
	saved, err := repo.Save(t.Context(), tutors.Tutor{DisplayName: displayName})
	if err != nil {
		t.Fatalf("failed to seed tutor: %v", err)
	}
	return saved
}

func TestTutorRepository(t *testing.T) {
	t.Run("Save", func(t *testing.T) {
		t.Run("Successful create", func(t *testing.T) {
			repo := newRepo(t)
			tutor := tutors.Tutor{
				DisplayName: "test",
				Email:       "test@example.com",
				Phone:       "+393334455666",
				AboutMe:     "I teach math and physics.",
			}
			created, err := repo.Save(t.Context(), tutor)
			if err != nil {
				t.Fatalf("failed to create tutor: %v", err)
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
			if created.DisplayName != tutor.DisplayName ||
				created.Email != tutor.Email ||
				created.Phone != tutor.Phone ||
				created.AboutMe != tutor.AboutMe {
				t.Errorf("expected fields to round-trip, got %+v", created)
			}
		})

		t.Run("Successful update preserves CreatedAt", func(t *testing.T) {
			repo := newRepo(t)
			created := seedTutor(t, repo, "update me")

			updated, err := repo.Save(t.Context(), tutors.Tutor{
				ID:          created.ID,
				DisplayName: "updated name",
			})
			if err != nil {
				t.Fatalf("failed to update tutor: %v", err)
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
			saved, err := repo.Save(t.Context(), tutors.Tutor{
				ID:          12345,
				DisplayName: "upserted",
			})
			if err != nil {
				t.Fatalf("failed to save tutor: %v", err)
			}
			if saved.ID != 12345 {
				t.Errorf("expected id 12345, got %d", saved.ID)
			}
			got, err := repo.GetByID(t.Context(), 12345)
			if err != nil {
				t.Fatalf("failed to get created tutor: %v", err)
			}
			if got.DisplayName != "upserted" {
				t.Errorf("expected DisplayName 'upserted', got %q", got.DisplayName)
			}
		})

		t.Run("Empty optional fields round-trip as empty strings", func(t *testing.T) {
			repo := newRepo(t)
			saved, err := repo.Save(t.Context(), tutors.Tutor{DisplayName: "only name"})
			if err != nil {
				t.Fatalf("failed to save tutor: %v", err)
			}
			if saved.Email != "" || saved.Phone != "" || saved.AboutMe != "" {
				t.Errorf("expected optional fields to be empty, got %+v", saved)
			}
			got, err := repo.GetByID(t.Context(), saved.ID)
			if err != nil {
				t.Fatalf("failed to get tutor: %v", err)
			}
			if got.Email != "" || got.Phone != "" || got.AboutMe != "" {
				t.Errorf("expected optional fields to be empty after reload, got %+v", got)
			}
		})
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("Successful get tutor by id", func(t *testing.T) {
			repo := newRepo(t)
			tutor := seedTutor(t, repo, "test get tutor by id")
			got, err := repo.GetByID(t.Context(), tutor.ID)
			if err != nil {
				t.Fatalf("failed to get tutor by id: %v", err)
			}
			if got != tutor {
				t.Fatalf("\nwant:%+v\ngot :%+v", tutor, got)
			}
		})

		t.Run("Missing id returns ErrNotFound", func(t *testing.T) {
			repo := newRepo(t)
			_, err := repo.GetByID(t.Context(), 999999)
			if !errors.Is(err, tutors.ErrNotFound) {
				t.Errorf("expected tutors.ErrNotFound, got %v", err)
			}
			if !errors.Is(err, errs.NotFound) {
				t.Errorf("expected errs.NotFound, got %v", err)
			}
		})
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Run("Successful get all tutors", func(t *testing.T) {
			repo := newRepo(t)
			tutor1 := seedTutor(t, repo, "test get all tutors 1")
			tutor2 := seedTutor(t, repo, "test get all tutors 2")

			got, err := repo.GetAll(t.Context())
			if err != nil {
				t.Fatalf("failed to get all tutors: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("expected 2 tutors, got %d", len(got))
			}
			found1 := false
			found2 := false
			for _, tt := range got {
				if tt.ID == tutor1.ID {
					found1 = true
				}
				if tt.ID == tutor2.ID {
					found2 = true
				}
			}
			if !found1 || !found2 {
				t.Fatalf("expected to find both seeded tutors in the result")
			}
		})

		t.Run("Empty returns empty slice", func(t *testing.T) {
			repo := newRepo(t)
			got, err := repo.GetAll(t.Context())
			if err != nil {
				t.Fatalf("failed to get all tutors: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("expected 0 tutors, got %d", len(got))
			}
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Successful delete", func(t *testing.T) {
			repo := newRepo(t)
			tutor := seedTutor(t, repo, "to delete")

			if err := repo.Delete(t.Context(), tutor.ID); err != nil {
				t.Fatalf("failed to delete tutor: %v", err)
			}
			_, err := repo.GetByID(t.Context(), tutor.ID)
			if !errors.Is(err, tutors.ErrNotFound) {
				t.Errorf("expected the tutor to be gone after delete, got %v", err)
			}
		})

		t.Run("Missing id returns ErrNotFound", func(t *testing.T) {
			repo := newRepo(t)
			err := repo.Delete(t.Context(), 999999)
			if !errors.Is(err, tutors.ErrNotFound) {
				t.Errorf("expected tutors.ErrNotFound, got %v", err)
			}
		})
	})
}
