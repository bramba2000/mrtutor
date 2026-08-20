package students_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/students"
)

// fakeRepo is a configurable in-memory [students.Repository]. Its default
// Save mirrors the real SQL upsert (SaveStudent in studentssqlite): a zero
// ID assigns a new one and sets CreatedAt, a non-zero ID preserves any
// existing row's CreatedAt and sets ModifiedAt, and an unknown non-zero ID
// inserts a new row under that ID. Each method can be overridden per test
// via its *Fn field, and every call is recorded so tests can assert on the
// exact argument the service passed down.
type fakeRepo struct {
	db     map[int]students.Student
	nextID int

	SaveFn                     func(context.Context, students.Student) (students.Student, error)
	GetByIDFn                  func(context.Context, int) (students.Student, error)
	GetAllFn                   func(context.Context) ([]students.Student, error)
	DeleteFn                   func(context.Context, int) error
	GetDistinctSchoolsFn       func(context.Context) ([]string, error)
	GetDistinctStudyProgramsFn func(context.Context) ([]string, error)
	GetDistinctClassesFn       func(context.Context) ([]string, error)

	SaveCalled                     bool
	GetByIDCalled                  bool
	GetAllCalled                   bool
	DeleteCalled                   bool
	GetDistinctSchoolsCalled       bool
	GetDistinctStudyProgramsCalled bool
	GetDistinctClassesCalled       bool
	GotSave                        students.Student
	GotGetByIDID                   int
	GotDeleteID                    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{db: make(map[int]students.Student)}
}

// Save implements [students.Repository].
func (r *fakeRepo) Save(ctx context.Context, student students.Student) (students.Student, error) {
	r.SaveCalled = true
	r.GotSave = student
	if r.SaveFn != nil {
		return r.SaveFn(ctx, student)
	}

	if existing, ok := r.db[student.ID]; ok {
		student.CreatedAt = existing.CreatedAt
		student.ModifiedAt = time.Now().UTC()
	} else {
		if student.ID == 0 {
			r.nextID++
			student.ID = r.nextID
		}
		student.CreatedAt = time.Now().UTC()
		student.ModifiedAt = time.Time{}
	}
	r.db[student.ID] = student
	return student, nil
}

// GetByID implements [students.Repository].
func (r *fakeRepo) GetByID(ctx context.Context, id int) (students.Student, error) {
	r.GetByIDCalled = true
	r.GotGetByIDID = id
	if r.GetByIDFn != nil {
		return r.GetByIDFn(ctx, id)
	}
	if student, ok := r.db[id]; ok {
		return student, nil
	}
	return students.Student{}, students.ErrNotFound
}

// GetAll implements [students.Repository].
func (r *fakeRepo) GetAll(ctx context.Context) ([]students.Student, error) {
	r.GetAllCalled = true
	if r.GetAllFn != nil {
		return r.GetAllFn(ctx)
	}
	return slices.Collect(maps.Values(r.db)), nil
}

// GetDistinctSchools implements [students.Repository].
func (r *fakeRepo) GetDistinctSchools(ctx context.Context) ([]string, error) {
	r.GetDistinctSchoolsCalled = true
	if r.GetDistinctSchoolsFn != nil {
		return r.GetDistinctSchoolsFn(ctx)
	}
	return nil, nil
}

// GetDistinctStudyPrograms implements [students.Repository].
func (r *fakeRepo) GetDistinctStudyPrograms(ctx context.Context) ([]string, error) {
	r.GetDistinctStudyProgramsCalled = true
	if r.GetDistinctStudyProgramsFn != nil {
		return r.GetDistinctStudyProgramsFn(ctx)
	}
	return nil, nil
}

// GetDistinctClasses implements [students.Repository].
func (r *fakeRepo) GetDistinctClasses(ctx context.Context) ([]string, error) {
	r.GetDistinctClassesCalled = true
	if r.GetDistinctClassesFn != nil {
		return r.GetDistinctClassesFn(ctx)
	}
	return nil, nil
}

// Delete implements [students.Repository].
func (r *fakeRepo) Delete(ctx context.Context, id int) error {
	r.DeleteCalled = true
	r.GotDeleteID = id
	if r.DeleteFn != nil {
		return r.DeleteFn(ctx, id)
	}
	if _, ok := r.db[id]; !ok {
		return students.ErrNotFound
	}
	delete(r.db, id)
	return nil
}

var _ students.Repository = (*fakeRepo)(nil)

func TestService(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		t.Run("Correctly sets id and timestamps", func(t *testing.T) {
			repo := newFakeRepo()
			svc := students.NewService(repo)

			created, err := svc.Create(t.Context(), students.CreateIn{DisplayName: "John Doe"})
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

		t.Run("Passes a zero id to the repository", func(t *testing.T) {
			repo := newFakeRepo()
			svc := students.NewService(repo)

			_, err := svc.Create(t.Context(), students.CreateIn{DisplayName: "Jane Doe"})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if !repo.SaveCalled {
				t.Fatalf("expected Save to be called")
			}
			if repo.GotSave.ID != 0 {
				t.Errorf("expected Create to pass id=0 to Save, got %d", repo.GotSave.ID)
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.SaveFn = func(context.Context, students.Student) (students.Student, error) {
				return students.Student{}, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.Create(t.Context(), students.CreateIn{DisplayName: "John Doe"})
			if !errors.Is(err, wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("CreateIn.Validate", func(t *testing.T) {
		base := students.CreateIn{DisplayName: "John Doe"}

		tests := []struct {
			name    string
			in      func(in *students.CreateIn)
			wantErr bool
		}{
			{
				name:    "Succeeds with just a display name",
				in:      func(in *students.CreateIn) {},
				wantErr: false,
			},
			{
				name: "Succeeds with all optional fields populated",
				in: func(in *students.CreateIn) {
					in.Email = "john@example.com"
					in.Phone = "+393334455666"
					in.School = "School"
					in.StudyProgram = "Program"
					in.Class = "Class"
					in.BirthDate = "2000-01-01"
				},
				wantErr: false,
			},
			{
				name: "Fails with a blank display name",
				in: func(in *students.CreateIn) {
					in.DisplayName = ""
				},
				wantErr: true,
			},
			{
				name: "Fails with a display name over 256 characters",
				in: func(in *students.CreateIn) {
					in.DisplayName = string(make([]byte, 257))
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed email",
				in: func(in *students.CreateIn) {
					in.Email = "not-an-email"
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed phone",
				in: func(in *students.CreateIn) {
					in.Phone = "not-a-phone"
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed birth date",
				in: func(in *students.CreateIn) {
					in.BirthDate = "not-a-date"
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

		t.Run("Reports the malformed birth date under the birthDate field", func(t *testing.T) {
			in := base
			in.BirthDate = "not-a-date"
			err := in.Validate()
			if err == nil {
				t.Fatal("Validate() succeeded unexpectedly")
			}
			if !strings.Contains(err.Error(), "birthDate") {
				t.Errorf("Validate() error = %q, want it to mention field 'birthDate'", err.Error())
			}
		})
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("Passes through the returned value", func(t *testing.T) {
			repo := newFakeRepo()
			want := students.Student{ID: 1, DisplayName: "John Doe"}
			repo.db[1] = want
			svc := students.NewService(repo)

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
			svc := students.NewService(repo)

			_, err := svc.GetByID(t.Context(), 999)
			if !errors.Is(err, students.ErrNotFound) {
				t.Fatalf("GetByID() error = %v, want students.ErrNotFound", err)
			}
		})
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Run("Passes through the returned slice", func(t *testing.T) {
			repo := newFakeRepo()
			repo.db[1] = students.Student{ID: 1, DisplayName: "A"}
			repo.db[2] = students.Student{ID: 2, DisplayName: "B"}
			svc := students.NewService(repo)

			got, err := svc.GetAll(t.Context())
			if err != nil {
				t.Fatalf("GetAll() error = %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("GetAll() returned %d students, want 2", len(got))
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.GetAllFn = func(context.Context) ([]students.Student, error) {
				return nil, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.GetAll(t.Context())
			if !errors.Is(err, wantErr) {
				t.Fatalf("GetAll() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("GetDistinctSchools", func(t *testing.T) {
		t.Run("Passes through the returned slice", func(t *testing.T) {
			repo := newFakeRepo()
			want := []string{"A School", "B School"}
			repo.GetDistinctSchoolsFn = func(context.Context) ([]string, error) {
				return want, nil
			}
			svc := students.NewService(repo)

			got, err := svc.GetDistinctSchools(t.Context())
			if err != nil {
				t.Fatalf("GetDistinctSchools() error = %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("GetDistinctSchools() = %v, want %v", got, want)
			}
			if !repo.GetDistinctSchoolsCalled {
				t.Error("expected the repository to be called")
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.GetDistinctSchoolsFn = func(context.Context) ([]string, error) {
				return nil, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.GetDistinctSchools(t.Context())
			if !errors.Is(err, wantErr) {
				t.Fatalf("GetDistinctSchools() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("GetDistinctStudyPrograms", func(t *testing.T) {
		t.Run("Passes through the returned slice", func(t *testing.T) {
			repo := newFakeRepo()
			want := []string{"Program A", "Program B"}
			repo.GetDistinctStudyProgramsFn = func(context.Context) ([]string, error) {
				return want, nil
			}
			svc := students.NewService(repo)

			got, err := svc.GetDistinctStudyPrograms(t.Context())
			if err != nil {
				t.Fatalf("GetDistinctStudyPrograms() error = %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("GetDistinctStudyPrograms() = %v, want %v", got, want)
			}
			if !repo.GetDistinctStudyProgramsCalled {
				t.Error("expected the repository to be called")
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.GetDistinctStudyProgramsFn = func(context.Context) ([]string, error) {
				return nil, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.GetDistinctStudyPrograms(t.Context())
			if !errors.Is(err, wantErr) {
				t.Fatalf("GetDistinctStudyPrograms() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("GetDistinctClasses", func(t *testing.T) {
		t.Run("Passes through the returned slice", func(t *testing.T) {
			repo := newFakeRepo()
			want := []string{"Class A", "Class B"}
			repo.GetDistinctClassesFn = func(context.Context) ([]string, error) {
				return want, nil
			}
			svc := students.NewService(repo)

			got, err := svc.GetDistinctClasses(t.Context())
			if err != nil {
				t.Fatalf("GetDistinctClasses() error = %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("GetDistinctClasses() = %v, want %v", got, want)
			}
			if !repo.GetDistinctClassesCalled {
				t.Error("expected the repository to be called")
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.GetDistinctClassesFn = func(context.Context) ([]string, error) {
				return nil, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.GetDistinctClasses(t.Context())
			if !errors.Is(err, wantErr) {
				t.Fatalf("GetDistinctClasses() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Correctly sets modified at and preserves created at", func(t *testing.T) {
			repo := newFakeRepo()
			createdAt := time.Now().UTC().Add(-time.Hour)
			created := students.Student{
				ID:          1,
				DisplayName: "John Doe",
				Email:       "john@example.com",
				Phone:       "3498581656",
				CreatedAt:   createdAt,
			}
			repo.db[created.ID] = created

			updateIn := students.UpdateIn{
				ID:           created.ID,
				DisplayName:  "John",
				Email:        created.Email,
				Phone:        created.Phone,
				School:       created.School,
				StudyProgram: created.StudyProgram,
				Class:        created.Class,
				BirthDate:    created.BirthDate,
			}

			svc := students.NewService(repo)
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
			if repo.GotSave.ID != created.ID {
				t.Errorf("expected Save to receive id %d, got %d", created.ID, repo.GotSave.ID)
			}
		})

		t.Run("Creates a row when the id is unknown", func(t *testing.T) {
			repo := newFakeRepo()
			svc := students.NewService(repo)

			updated, err := svc.Update(t.Context(), students.UpdateIn{ID: 42, DisplayName: "New"})
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if updated.ID != 42 {
				t.Errorf("Update() ID = %v, want 42", updated.ID)
			}
			if updated.CreatedAt.IsZero() {
				t.Errorf("Update() CreatedAt = %v, want non-zero for a newly created row", updated.CreatedAt)
			}
		})

		t.Run("Propagates repository errors", func(t *testing.T) {
			repo := newFakeRepo()
			wantErr := errors.New("boom")
			repo.SaveFn = func(context.Context, students.Student) (students.Student, error) {
				return students.Student{}, wantErr
			}
			svc := students.NewService(repo)

			_, err := svc.Update(t.Context(), students.UpdateIn{ID: 1, DisplayName: "John"})
			if !errors.Is(err, wantErr) {
				t.Fatalf("Update() error = %v, want %v", err, wantErr)
			}
		})
	})

	t.Run("UpdateIn.Validate", func(t *testing.T) {
		base := students.UpdateIn{ID: 1, DisplayName: "John Doe"}

		t.Run("Succeeds with valid input", func(t *testing.T) {
			// Regression test: validateStudentData returns a nil error when
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
			in      func(in *students.UpdateIn)
			wantErr bool
		}{
			{
				name: "Fails with id less than 1",
				in: func(in *students.UpdateIn) {
					in.ID = 0
				},
				wantErr: true,
			},
			{
				name: "Fails with a blank display name",
				in: func(in *students.UpdateIn) {
					in.DisplayName = ""
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed email",
				in: func(in *students.UpdateIn) {
					in.Email = "not-an-email"
				},
				wantErr: true,
			},
			{
				name: "Fails with a malformed birth date",
				in: func(in *students.UpdateIn) {
					in.BirthDate = "not-a-date"
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
			repo.db[1] = students.Student{ID: 1}
			svc := students.NewService(repo)

			if err := svc.Delete(t.Context(), 1); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			if repo.GotDeleteID != 1 {
				t.Errorf("expected id 1 to reach the repository, got %d", repo.GotDeleteID)
			}
		})

		t.Run("Propagates ErrNotFound", func(t *testing.T) {
			repo := newFakeRepo()
			svc := students.NewService(repo)

			err := svc.Delete(t.Context(), 999)
			if !errors.Is(err, students.ErrNotFound) {
				t.Fatalf("Delete() error = %v, want students.ErrNotFound", err)
			}
		})
	})
}
