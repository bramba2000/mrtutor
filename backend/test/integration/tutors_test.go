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
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorshttp"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorssqlite"
	"github.com/bramba2000/mrtutor/backend/httpx"
	"github.com/bramba2000/mrtutor/backend/sqlite/sqlitetest"
)

func TestTutors(t *testing.T) {
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

	repo := tutorssqlite.NewRepository(db)
	svc := tutors.NewService(repo)

	router := httpx.NewRouter("")
	tutorshttp.NewHandler(svc, authService, logger).Mount(router)

	principal := seedPrincipal(t, authStorage.PrincipalStore, "tutors", "Test00!")

	t.Run("Cannot create a new tutor when unauthenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPost, "/tutors/", tutors.CreateIn{
			DisplayName: "John",
			Email:       "john@example.com",
			Phone:       "+393334455667",
			AboutMe:     "I teach math.",
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot list tutors when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tutors/", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot update a tutor when unauthenticated", func(t *testing.T) {
		req := newJSONRequest(t, http.MethodPut, "/tutors/1", tutors.UpdateIn{
			TutorFields: tutors.TutorFields{DisplayName: "John"},
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Cannot delete a tutor when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/tutors/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Full lifecycle when authenticated", func(t *testing.T) {
		var id int

		t.Run("Create", func(t *testing.T) {
			req := newJSONRequest(t, http.MethodPost, "/tutors/", tutors.CreateIn{
				DisplayName: "John",
				Email:       "john@example.com",
				Phone:       "+393334455667",
				AboutMe:     "I teach math and physics.",
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("Expected status code %d, got %d", http.StatusCreated, w.Code)
			}
			var body tutors.Tutor
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if body.DisplayName != "John" {
				t.Errorf("Expected DisplayName to be 'John', got '%s'", body.DisplayName)
			}
			if body.ID == 0 {
				t.Errorf("Expected ID to be set, got 0")
			}
			if body.AboutMe != "I teach math and physics." {
				t.Errorf("Expected AboutMe to be 'I teach math and physics.', got '%s'", body.AboutMe)
			}
			if body.CreatedAt.IsZero() {
				t.Errorf("Expected CreatedAt to be set")
			}
			id = body.ID
		})

		t.Run("Get by id", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tutors/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body tutors.Tutor
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if body.ID != id {
				t.Errorf("Expected ID %d, got %d", id, body.ID)
			}
		})

		t.Run("Appears in the list", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tutors/", nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body []tutors.Tutor
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			found := false
			for _, tt := range body {
				if tt.ID == id {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected the created tutor to appear in the list, got %+v", body)
			}
		})

		t.Run("Update", func(t *testing.T) {
			req := newJSONRequest(t, http.MethodPut, "/tutors/"+strconv.Itoa(id), tutors.UpdateIn{
				TutorFields: tutors.TutorFields{
					DisplayName: "John Updated",
					Email:       "john.updated@example.com",
					Phone:       "+393334455667",
					AboutMe:     "I teach math, physics, and chemistry.",
				},
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
			var body tutors.Tutor
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
			req := newJSONRequest(t, http.MethodPut, "/tutors/"+strconv.Itoa(id), tutors.UpdateIn{
				TutorFields: tutors.TutorFields{
					DisplayName: "",
				},
			})
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/tutors/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Fatalf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
			}
		})

		t.Run("Get by id now returns 404", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tutors/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
			}
		})

		t.Run("Delete again now returns 404", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/tutors/"+strconv.Itoa(id), nil)
			authenticateRequest(t, authStorage.SessionStore, principal, req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
			}
		})
	})
}
