package students_test

import (
	"context"
	"maps"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/features/students"
)

type memoryStudentRepo struct {
	db    map[int]students.Student
	count atomic.Int64
}

// Create implements [students.Repository].
func (r *memoryStudentRepo) Create(ctx context.Context, student students.Student) (int, error) {
	id := int(r.count.Add(1))
	student.ID = id
	r.db[id] = student
	return id, nil
}

// Delete implements [students.Repository].
func (r *memoryStudentRepo) Delete(ctx context.Context, id int) error {
	if _, ok := r.db[id]; !ok {
		return students.ErrNotFound
	}
	delete(r.db, id)
	return nil
}

// GetAll implements [students.Repository].
func (r *memoryStudentRepo) GetAll(ctx context.Context) ([]students.Student, error) {
	return slices.Collect(maps.Values(r.db)), nil
}

// GetByID implements [students.Repository].
func (r *memoryStudentRepo) GetByID(ctx context.Context, id int) (students.Student, error) {
	if student, ok := r.db[id]; ok {
		return student, nil
	}
	return students.Student{}, students.ErrNotFound
}

// Update implements [students.Repository].
func (r *memoryStudentRepo) Update(ctx context.Context, student students.Student) (students.Student, error) {
	if _, ok := r.db[student.ID]; !ok {
		return students.Student{}, students.ErrNotFound
	}
	student.CreatedAt = r.db[student.ID].CreatedAt
	r.db[student.ID] = student
	return r.db[student.ID], nil
}

var _ students.Repository = &memoryStudentRepo{}

func TestService(t *testing.T) {
	repo := &memoryStudentRepo{
		db: make(map[int]students.Student),
	}
	svc := students.NewService(repo)

	t.Run("Create", func(t *testing.T) {
		t.Run("Correctly set id and timestamps", func(t *testing.T) {
			created, err := svc.Create(context.Background(),
				students.CreateIn{DisplayName: "John Doe"})
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
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Logf("Skipped, just a wrapper of repository")
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Logf("Skipped, just a wrapper of repository")
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Correctly set modified at", func(t *testing.T) {
			createdAt := time.Now().UTC()
			created := students.Student{
				ID:          1,
				DisplayName: "John Doe",
				Email:       "john@example.com",
				Phone:       "3498581656",
				CreatedAt:   createdAt,
			}
			repo.db[created.ID] = created

			// use all data from created to update, but change DisplayName
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

			updated, err := svc.Update(context.Background(),
				updateIn)

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
		})
	})

}
