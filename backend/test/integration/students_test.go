package integration

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authsqlite"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentshttp"
	"github.com/bramba2000/mrtutor/backend/features/students/studentssqlite"
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

	router := httpx.NewRouter("")
	studentshttp.NewHandler(svc, authService, logger).Mount(router)

	principal := seedPrincipal(t, authStorage.PrincipalStore, "students", "Test00!")

	t.Run("Cannot create a new students when unauthenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/students/", students.Student{
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

	t.Run("Can create a student when authenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/students/", students.Student{
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

	})
}
