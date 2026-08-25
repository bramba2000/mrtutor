package integration

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/features/enrollments"
	"github.com/bramba2000/mrtutor/backend/features/enrollments/enrollmentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentshttp"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorshttp"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite"
	"github.com/bramba2000/mrtutor/backend/httpx"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

func TestStudents(t *testing.T) {
	skipIfNotIntegration(t)

	logger := slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	db := sqlitetest.OpenTemp(t)

	authStorage := authsqlite.Build(db)
	authService := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)

	repo := studentssqlite.NewRepository(db)
	svc := students.NewService(repo)

	tutorsRepo := tutorssqlite.NewRepository(db)
	tutorsSvc := tutors.NewService(tutorsRepo)

	enrollmentsRepo := enrollmentssqlite.NewRepository(db)
	enrollmentsSvc := enrollments.NewService(enrollmentsRepo)

	router := httpx.NewRouter("")
	studentshttp.NewHandler(svc, tutorsSvc, enrollmentsSvc, authService, logger).Mount(router)
	tutorshttp.NewHandler(tutorsSvc, enrollmentsSvc, svc, authService, logger).Mount(router)

	principal := seedPrincipal(t, authStorage.PrincipalStore, "students", "Test00!")

	tutor, err := tutorsRepo.Create(t.Context(), tutors.TutorFields{DisplayName: "Tutor"}, principal.ID)
	if err != nil {
		t.Fatalf("failed to seed tutor for principal: %v", err)
	}

	t.Run("Cannot create a new student when unauthenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/students/", students.CreateIn{
			DisplayName:  "John",
			Email:        "john@example.com",
			Phone:        "333 445566777",
			School:       "School",
			StudyProgram: "Study program",
			Class:        "Class",
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot list students when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/students/", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot update a student when unauthenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPut, "/students/1", students.UpdateIn{DisplayName: "John"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot delete a student when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/students/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Full lifecycle when authenticated", func(t *testing.T) {
		var id int

		t.Run("Create", func(t *testing.T) {
			req := newJSONRequest(t, http.MethodPost, "/students/", students.CreateIn{
				DisplayName:  "John",
				Email:        "john@example.com",
				Phone:        "+393333445566777",
				School:       "School",
				StudyProgram: "Study program",
				Class:        "Class",
				BirthDate:    "2000-01-01",
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("Expected status code %d, got %d", http.StatusCreated, w.Code)
			}
			var body students.Student
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if body.DisplayName != "John" {
				t.Errorf("Expected DisplayName to be 'John', got '%s'", body.DisplayName)
			}
			if body.ID == 0 {
				t.Errorf("Expected ID to be set, got 0")
			}
			if body.BirthDate != "2000-01-01" {
				t.Errorf("Expected BirthDate to be '2000-01-01', got '%s'", body.BirthDate)
			}
			if body.CreatedAt.IsZero() {
				t.Errorf("Expected CreatedAt to be set")
			}
			id = body.ID
		})

		t.Run("Get by id", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/students/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body students.Student
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if body.ID != id {
				t.Errorf("Expected ID %d, got %d", id, body.ID)
			}
		})

		t.Run("Appears in the list", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/students/", nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body []students.Student
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			found := false
			for _, s := range body {
				if s.ID == id {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected the created student to appear in the list, got %+v", body)
			}
		})

		t.Run("Appears in the signed-in tutor's student list", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tutors/"+strconv.Itoa(tutor.ID)+"/students", nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body []students.Student
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			found := false
			for _, s := range body {
				if s.ID == id {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected the created student to appear in the tutor's student list, got %+v", body)
			}
		})

		t.Run("Update", func(t *testing.T) {
			req := newJSONRequest(t, http.MethodPut, "/students/"+strconv.Itoa(id), students.UpdateIn{
				DisplayName:  "John Updated",
				Email:        "john.updated@example.com",
				Phone:        "+393333445566777",
				School:       "School",
				StudyProgram: "Study program",
				Class:        "Class",
				BirthDate:    "2000-01-01",
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body students.Student
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if body.DisplayName != "John Updated" {
				t.Errorf("Expected DisplayName to be 'John Updated', got '%s'", body.DisplayName)
			}
			if body.CreatedAt.IsZero() {
				t.Errorf("Expected CreatedAt to survive the update")
			}
			if body.ModifiedAt.IsZero() {
				t.Errorf("Expected ModifiedAt to be set after the update")
			}
		})

		t.Run("Update with an invalid body fails validation", func(t *testing.T) {
			req := newJSONRequest(t, http.MethodPut, "/students/"+strconv.Itoa(id), students.UpdateIn{
				DisplayName: "",
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/students/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Fatalf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
			}
		})

		t.Run("Get by id now returns 404", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/students/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
			}
		})

		t.Run("Delete again now returns 404", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/students/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
			}
		})
	})
}
