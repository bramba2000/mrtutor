package tutors_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
)

// fakeRepo is a configurable in-memory [tutors.Repository]. Its default Save
// mirrors the real SQL upsert (SaveTutor in tutorssqlite): a zero ID assigns
// a new one and sets CreatedAt, a non-zero ID preserves any existing row's
// CreatedAt and sets ModifiedAt, and an unknown non-zero ID inserts a new row
// under that ID. Each method can be overridden per test via its *Fn field,
// and every call is recorded so tests can assert on the exact argument the
// service passed down.
type fakeRepo struct {
	db     map[int]tutors.Tutor
	nextID int

	CreateFn  func(context.Context, tutors.TutorFields) (tutors.Tutor, error)
	UpdateFn  func(context.Context, int, tutors.TutorFields) (tutors.Tutor, error)
	GetByIDFn func(context.Context, int) (tutors.Tutor, error)
	GetAllFn  func(context.Context) ([]tutors.Tutor, error)
	DeleteFn  func(context.Context, int) error

	CreateCalled   bool
	UpdateCalled   bool
	GetByIDCalled  bool
	GetAllCalled   bool
	DeleteCalled   bool
	GotCreate      tutors.TutorFields
	GotUpdateID    int
	GotUpdateTutor tutors.TutorFields
	GotGetByIDID   int
	GotDeleteID    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{db: make(map[int]tutors.Tutor)}
}

// Save implements [tutors.Repository].
func (r *fakeRepo) Create(ctx context.Context, tutor tutors.TutorFields) (tutors.Tutor, error) {
	r.CreateCalled = true
	r.GotCreate = tutor
	if r.CreateFn != nil {
		return r.CreateFn(ctx, tutor)
	}

	r.nextID++
	r.db[r.nextID] = tutors.Tutor{
		ID:          r.nextID,
		DisplayName: tutor.DisplayName,
		Email:       tutor.Email,
		Phone:       tutor.Phone,
		AboutMe:     tutor.AboutMe,
		CreatedAt:   time.Now().UTC(),
	}
	return r.db[r.nextID], nil
}

// Update implements [tutors.Repository].
func (r *fakeRepo) Update(ctx context.Context, id int, tutor tutors.TutorFields) (tutors.Tutor, error) {
	r.UpdateCalled = true
	r.GotUpdateID = id
	r.GotUpdateTutor = tutor
	if r.UpdateFn != nil {
		return r.UpdateFn(ctx, id, tutor)
	}

	if existing, ok := r.db[id]; ok {
		existing.DisplayName = tutor.DisplayName
		existing.Email = tutor.Email
		existing.Phone = tutor.Phone
		existing.AboutMe = tutor.AboutMe
		existing.ModifiedAt = time.Now().UTC()
		r.db[id] = existing
		return existing, nil
	} else {
		return tutors.Tutor{}, tutors.ErrNotFound
	}

}

// GetByID implements [tutors.Repository].
func (r *fakeRepo) GetByID(ctx context.Context, id int) (tutors.Tutor, error) {
	r.GetByIDCalled = true
	r.GotGetByIDID = id
	if r.GetByIDFn != nil {
		return r.GetByIDFn(ctx, id)
	}
	if tutor, ok := r.db[id]; ok {
		return tutor, nil
	}
	return tutors.Tutor{}, tutors.ErrNotFound
}

// GetAll implements [tutors.Repository].
func (r *fakeRepo) GetAll(ctx context.Context) ([]tutors.Tutor, error) {
	r.GetAllCalled = true
	if r.GetAllFn != nil {
		return r.GetAllFn(ctx)
	}
	return slices.Collect(maps.Values(r.db)), nil
}

// Delete implements [tutors.Repository].
func (r *fakeRepo) Delete(ctx context.Context, id int) error {
	r.DeleteCalled = true
	r.GotDeleteID = id
	if r.DeleteFn != nil {
		return r.DeleteFn(ctx, id)
	}
	if _, ok := r.db[id]; !ok {
		return tutors.ErrNotFound
	}
	delete(r.db, id)
	return nil
}

var _ tutors.Repository = (*fakeRepo)(nil)

func TestService(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		t.Run("Correctly sets id and timestamps", func(t *testing.T) {
			repo := newFakeRepo()
			svc := tutors.NewService(repo)

			created, err := svc.Create(t.Context(), tutors.CreateIn{DisplayName: "John Doe"})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if created.ID == 0 {
				t.Errorf("Create() id = %v, want non-zero", created.ID)
			}
			if created.CreatedAt.IsZero() {
				t.Errorf("Create() CreatedAt = %v, want non-zero", created.CreatedAt)
			}
			if !created.ModifiedAt.IsZero() {
				t.Errorf("Create() ModifiedAt = %v, want zero", created.ModifiedAt)
			}
			if created.DisplayName != "John Doe" {
				t.Errorf("Create() DisplayName = %v, want 'John Doe'", created.DisplayName)
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.CreateFn = func(context.Context, tutors.TutorFields) (tutors.Tutor, error) {
				return tutors.Tutor{}, wantErr
			}
			svc := tutors.NewService(repo)

			_, err := svc.Create(t.Context(), tutors.CreateIn{DisplayName: "John Doe"})
			if !errors.Is(err, wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("CreateIn.Validate", func(t *testing.T) {
		base := tutors.CreateIn{DisplayName: "John Doe"}

		tests := []struct {
			name    string
			in      func(in *tutors.CreateIn)
			wantErr bool
		}{
			{
				name:    "Succeeds with just a display name",
				in:      func(in *tutors.CreateIn) {},
				wantErr: false,
			},
			{
				name: "Succeeds with all optional fields populated",
				in: func(in *tutors.CreateIn) {
					in.Email = "john@example.com"
					in.Phone = "+393334455666"
					in.AboutMe = "I love teaching math."
				},
				wantErr: false,
			},
			{
				name: "Fails with a blank display name",
				in: func(in *tutors.CreateIn) {
					in.DisplayName = ""
				},
				wantErr: true,
			},
			{
				name: "Fails with a display name over 256 characters",
				in: func(in *tutors.CreateIn) {
					in.DisplayName = string(make([]byte, 257))
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed email",
				in: func(in *tutors.CreateIn) {
					in.Email = "not-an-email"
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed phone",
				in: func(in *tutors.CreateIn) {
					in.Phone = "not-a-phone"
				},
				wantErr: true,
			},
			{
				name: "Succeeds with an about me exactly 2000 characters long",
				in: func(in *tutors.CreateIn) {
					in.AboutMe = strings.Repeat("a", 2000)
				},
				wantErr: false,
			},
			{
				name: "Fails with an about me over 2000 characters",
				in: func(in *tutors.CreateIn) {
					in.AboutMe = strings.Repeat("a", 2001)
				},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				in := base
				tt.in(&in)
				gotErr := in.Validate()
				if gotErr != nil {
					if !tt.wantErr {
						t.Errorf("Validate() failed: %v", gotErr)
					}
					if !errors.Is(gotErr, errs.Invalid) {
						t.Errorf("Validate() error is not classified as errs.Invalid: %v", gotErr)
					}
					return
				}
				if tt.wantErr {
					t.Fatal("Validate() succeeded unexpectedly")
				}
			})
		}

		t.Run("Reports the malformed email under the email field", func(t *testing.T) {
			in := base
			in.Email = "not-an-email"
			err := in.Validate()
			if err == nil {
				t.Fatal("Validate() succeeded unexpectedly")
			}
			if !strings.Contains(err.Error(), "email") {
				t.Errorf("Validate() error = %q, want it to mention field 'email'", err.Error())
			}
		})
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("Passes through the returned value", func(t *testing.T) {
			repo := newFakeRepo()
			want := tutors.Tutor{ID: 1, DisplayName: "John Doe"}
			repo.db[1] = want
			svc := tutors.NewService(repo)

			got, err := svc.GetByID(t.Context(), 1)
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			if got != want {
				t.Errorf("GetByID() = %+v, want %+v", got, want)
			}
			if repo.GotGetByIDID != 1 {
				t.Errorf("expected id 1 to reach the repository, got %d", repo.GotGetByIDID)
			}
		})

		t.Run("Propagates ErrNotFound", func(t *testing.T) {
			repo := newFakeRepo()
			svc := tutors.NewService(repo)

			_, err := svc.GetByID(t.Context(), 999)
			if !errors.Is(err, tutors.ErrNotFound) {
				t.Fatalf("GetByID() error = %v, want tutors.ErrNotFound", err)
			}
		})
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Run("Passes through the returned slice", func(t *testing.T) {
			repo := newFakeRepo()
			repo.db[1] = tutors.Tutor{ID: 1, DisplayName: "A"}
			repo.db[2] = tutors.Tutor{ID: 2, DisplayName: "B"}
			svc := tutors.NewService(repo)

			got, err := svc.GetAll(t.Context())
			if err != nil {
				t.Fatalf("GetAll() error = %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("GetAll() returned %d tutors, want 2", len(got))
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.GetAllFn = func(context.Context) ([]tutors.Tutor, error) {
				return nil, wantErr
			}
			svc := tutors.NewService(repo)

			_, err := svc.GetAll(t.Context())
			if !errors.Is(err, wantErr) {
				t.Fatalf("GetAll() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Correctly sets modified at and preserves created at", func(t *testing.T) {
			repo := newFakeRepo()
			createdAt := time.Now().UTC().Add(-time.Hour)
			created := tutors.Tutor{
				ID:          1,
				DisplayName: "John Doe",
				Email:       "john@example.com",
				Phone:       "+393498581656",
				CreatedAt:   createdAt,
			}
			repo.db[created.ID] = created

			updateIn := tutors.UpdateIn{
				ID: created.ID,
				TutorFields: tutors.TutorFields{
					DisplayName: "John",
					Email:       created.Email,
					Phone:       created.Phone,
					AboutMe:     created.AboutMe,
				},
			}

			svc := tutors.NewService(repo)
			updated, err := svc.Update(t.Context(), updateIn)

			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if updated.ModifiedAt.IsZero() {
				t.Errorf("Update() ModifiedAt = %v, want non-zero", updated.ModifiedAt)
			}
			if !updated.CreatedAt.Equal(createdAt) {
				t.Errorf("Update() CreatedAt = %v, want %v", updated.CreatedAt, createdAt)
			}
			if updated.DisplayName != "John" {
				t.Errorf("Update() DisplayName = %v, want 'John'", updated.DisplayName)
			}
			if repo.GotUpdateID != created.ID {
				t.Errorf("expected Save to receive id %d, got %d", created.ID, repo.GotUpdateID)
			}
		})

		t.Run("Fail to update row when the id is unknown", func(t *testing.T) {
			repo := newFakeRepo()
			svc := tutors.NewService(repo)

			_, err := svc.Update(t.Context(), tutors.UpdateIn{
				ID:          42,
				TutorFields: tutors.TutorFields{DisplayName: "New"},
			})
			if err == nil {
				t.Fatal("Update() expect error, got nil")
			}
			if slices.ContainsFunc(slices.Collect(maps.Values(repo.db)), func(t tutors.Tutor) bool {
				return t.DisplayName == "New" || t.ID == 42
			}) {
				t.Errorf("Update() created a new row in the repository, want no new rows")
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.UpdateFn = func(_ context.Context, id int, in tutors.TutorFields) (tutors.Tutor, error) {
				return tutors.Tutor{}, wantErr
			}
			svc := tutors.NewService(repo)

			_, err := svc.Update(t.Context(), tutors.UpdateIn{
				ID: 1,
				TutorFields: tutors.TutorFields{
					DisplayName: "John",
				}})
			if !errors.Is(err, wantErr) {
				t.Fatalf("Update() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("UpdateIn.Validate", func(t *testing.T) {
		base := tutors.UpdateIn{
			ID:          1,
			TutorFields: tutors.TutorFields{DisplayName: "John Doe"},
		}

		t.Run("Succeeds with valid input", func(t *testing.T) {
			// Regression test: validateTutorData returns a nil error when
			// every field is valid, and UpdateIn.Validate used to blindly
			// type-assert that nil error to validation.Errors, which panics
			// on a nil interface. A valid UpdateIn must not panic and must
			// return nil.
			if err := base.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})

		tests := []struct {
			name    string
			in      func(in *tutors.UpdateIn)
			wantErr bool
		}{
			{
				name: "Fails with id less than 1",
				in: func(in *tutors.UpdateIn) {
					in.ID = 0
				},
				wantErr: true,
			},
			{
				name: "Fails with a blank display name",
				in: func(in *tutors.UpdateIn) {
					in.DisplayName = ""
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed email",
				in: func(in *tutors.UpdateIn) {
					in.Email = "not-an-email"
				},
				wantErr: true,
			},
			{
				name: "Fails with an about me over 2000 characters",
				in: func(in *tutors.UpdateIn) {
					in.AboutMe = strings.Repeat("a", 2001)
				},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				in := base
				tt.in(&in)
				gotErr := in.Validate()
				if gotErr != nil {
					if !tt.wantErr {
						t.Errorf("Validate() failed: %v", gotErr)
					}
					if !errors.Is(gotErr, errs.Invalid) {
						t.Errorf("Validate() error is not classified as errs.Invalid: %v", gotErr)
					}
					return
				}
				if tt.wantErr {
					t.Fatal("Validate() succeeded unexpectedly")
				}
			})
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Passes the id through and returns nil on success", func(t *testing.T) {
			repo := newFakeRepo()
			repo.db[1] = tutors.Tutor{ID: 1}
			svc := tutors.NewService(repo)

			if err := svc.Delete(t.Context(), 1); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			if repo.GotDeleteID != 1 {
				t.Errorf("expected id 1 to reach the repository, got %d", repo.GotDeleteID)
			}
		})

		t.Run("Propagates ErrNotFound", func(t *testing.T) {
			repo := newFakeRepo()
			svc := tutors.NewService(repo)

			err := svc.Delete(t.Context(), 999)
			if !errors.Is(err, tutors.ErrNotFound) {
				t.Fatalf("Delete() error = %v, want tutors.ErrNotFound", err)
			}
		})
	})
}
