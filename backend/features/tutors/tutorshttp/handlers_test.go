package tutorshttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/enrollments"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/features/tutors/tutorshttp"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

// Package tutorshttp_test is external, so these tests only reach the
// exported surface — the handler is driven through the real router, with a
// fake Service and a fake Authenticator standing in for the database.

// fakeService lets each test control what the domain layer returns, without
// a database in the loop.
type fakeService struct {
	createFn      func(context.Context, tutors.CreateIn) (tutors.Tutor, error)
	getByIDFn     func(context.Context, int) (tutors.Tutor, error)
	getAllFn      func(context.Context) ([]tutors.Tutor, error)
	updateFn      func(context.Context, tutors.UpdateIn) (tutors.Tutor, error)
	deleteFn      func(context.Context, int) error
	getByUserIDFn func(context.Context, int) (tutors.Tutor, error)

	createCalled      bool
	getByIDCalled     bool
	getAllCalled      bool
	updateCalled      bool
	deleteCalled      bool
	getByUserIDCalled bool

	gotCreate      tutors.CreateIn
	gotGetByID     int
	gotUpdate      tutors.UpdateIn
	gotDelete      int
	gotGetByUserID int
}

func (f *fakeService) Create(ctx context.Context, in tutors.CreateIn) (tutors.Tutor, error) {
	f.createCalled = true
	f.gotCreate = in
	return f.createFn(ctx, in)
}

func (f *fakeService) GetByID(ctx context.Context, id int) (tutors.Tutor, error) {
	f.getByIDCalled = true
	f.gotGetByID = id
	return f.getByIDFn(ctx, id)
}

func (f *fakeService) GetAll(ctx context.Context) ([]tutors.Tutor, error) {
	f.getAllCalled = true
	return f.getAllFn(ctx)
}

func (f *fakeService) Update(ctx context.Context, in tutors.UpdateIn) (tutors.Tutor, error) {
	f.updateCalled = true
	f.gotUpdate = in
	return f.updateFn(ctx, in)
}

func (f *fakeService) Delete(ctx context.Context, id int) error {
	f.deleteCalled = true
	f.gotDelete = id
	return f.deleteFn(ctx, id)
}

func (f *fakeService) GetByUserID(ctx context.Context, userId int) (tutors.Tutor, error) {
	f.getByUserIDCalled = true
	f.gotGetByUserID = userId
	return f.getByUserIDFn(ctx, userId)
}

var (
	_ tutorshttp.Service = (*fakeService)(nil)
	_ tutorshttp.Service = tutors.Service{}
	_ httpx.Validable    = tutors.CreateIn{}
	_ httpx.Validable    = tutors.UpdateIn{}
)

// fakeEnrollmentsService implements [tutorshttp.EnrollmentsService].
type fakeEnrollmentsService struct {
	getByTutorIDFn func(context.Context, int) ([]enrollments.Enrollment, error)

	getByTutorIDCalled bool
	gotGetByTutorID    int
}

func (f *fakeEnrollmentsService) GetByTutorID(ctx context.Context, tutorID int) ([]enrollments.Enrollment, error) {
	f.getByTutorIDCalled = true
	f.gotGetByTutorID = tutorID
	return f.getByTutorIDFn(ctx, tutorID)
}

// fakeStudentsService implements [tutorshttp.StudentsService].
type fakeStudentsService struct {
	getByIDFn func(context.Context, int) (students.Student, error)

	getByIDCalled bool
	gotGetByID    int
}

func (f *fakeStudentsService) GetByID(ctx context.Context, id int) (students.Student, error) {
	f.getByIDCalled = true
	f.gotGetByID = id
	return f.getByIDFn(ctx, id)
}

var (
	_ tutorshttp.EnrollmentsService = (*fakeEnrollmentsService)(nil)
	_ tutorshttp.StudentsService    = (*fakeStudentsService)(nil)
)

// fakeAuthenticator implements [authhttp.Authenticator] without a database.
type fakeAuthenticator struct{}

const validToken = "valid-token"

func (fakeAuthenticator) Authenticate(ctx context.Context, token string) (auth.Principal, error) {
	if token == validToken {
		return auth.Principal{ID: 1, Username: "test"}, nil
	}
	return auth.Principal{}, auth.ErrUnauthenticated
}

// errorBody mirrors httpx's unexported wire shape. Declared here rather than
// exported from httpx, because this test pins the JSON contract, not the Go
// type — an internal rename to httpx shouldn't have to touch this file.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fields  []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields,omitempty"`
}

func newJSONRequest(method, target string, body any) *http.Request {
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(method, target, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func authed(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: "session", Value: validToken})
	return req
}

func mounted(t *testing.T, svc tutorshttp.Service) *httpx.Router {
	t.Helper()
	return mountedWithDeps(t, svc, &fakeEnrollmentsService{}, &fakeStudentsService{})
}

func mountedWithDeps(t *testing.T, svc tutorshttp.Service, enrollmentsSvc tutorshttp.EnrollmentsService, studentsSvc tutorshttp.StudentsService) *httpx.Router {
	t.Helper()
	r := httpx.NewRouter("")
	l := slog.New(slog.NewTextHandler(t.Output(), nil))
	tutorshttp.NewHandler(svc, enrollmentsSvc, studentsSvc, fakeAuthenticator{}, l).Mount(r)
	return r
}

func decodeErrorBody(t *testing.T, w *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response body: %v", err)
	}
	return body
}

func hasField(body errorBody, field string) bool {
	for _, f := range body.Fields {
		if f.Field == field {
			return true
		}
	}
	return false
}

func TestGetAll(t *testing.T) {
	t.Run("Success returns the tutors as JSON", func(t *testing.T) {
		svc := &fakeService{
			getAllFn: func(context.Context) ([]tutors.Tutor, error) {
				return []tutors.Tutor{{ID: 1, DisplayName: "John"}}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/", nil)))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		var got []tutors.Tutor
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != 1 || got[0].DisplayName != "John" {
			t.Errorf("expected the service's tutors in the body, got %+v", got)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tutors/", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.getAllCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})

	t.Run("Fail when the service returns an unclassified error", func(t *testing.T) {
		svc := &fakeService{
			getAllFn: func(context.Context) ([]tutors.Tutor, error) {
				return nil, errors.New("db is on fire")
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/", nil)))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "db is on fire") {
			t.Errorf("expected the error cause to be withheld, got body %q", w.Body.String())
		}
	})
}

func TestGetByID(t *testing.T) {
	t.Run("Success returns the tutor", func(t *testing.T) {
		svc := &fakeService{
			getByIDFn: func(ctx context.Context, id int) (tutors.Tutor, error) {
				return tutors.Tutor{ID: id, DisplayName: "John"}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/7", nil)))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if svc.gotGetByID != 7 {
			t.Errorf("expected the decoded id 7 to reach the service, got %d", svc.gotGetByID)
		}
	})

	t.Run("Fail when the id is not numeric", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/abc", nil)))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.getByIDCalled {
			t.Error("expected the service not to be called with an invalid id")
		}
		body := decodeErrorBody(t, w)
		if body.Code != "invalid" {
			t.Errorf("expected code %q, got %q", "invalid", body.Code)
		}
	})

	t.Run("Fail when the tutor does not exist", func(t *testing.T) {
		svc := &fakeService{
			getByIDFn: func(context.Context, int) (tutors.Tutor, error) {
				return tutors.Tutor{}, tutors.ErrNotFound
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/999", nil)))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
		body := decodeErrorBody(t, w)
		if body.Code != "tutors.notFound" {
			t.Errorf("expected code %q, got %q", "tutors.notFound", body.Code)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tutors/7", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("Success creates the tutor", func(t *testing.T) {
		svc := &fakeService{
			createFn: func(ctx context.Context, in tutors.CreateIn) (tutors.Tutor, error) {
				return tutors.Tutor{ID: 1, DisplayName: in.DisplayName}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPost, "/tutors/", tutors.TutorFields{DisplayName: "John"})))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
		}
		if svc.gotCreate.DisplayName != "John" {
			t.Errorf("expected the decoded CreateIn to reach the service, got %+v", svc.gotCreate)
		}
		var got tutors.Tutor
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.ID != 1 {
			t.Errorf("expected the created tutor in the body, got %+v", got)
		}
	})

	t.Run("Fail when the display name is blank", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPost, "/tutors/", tutors.CreateIn{})))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.createCalled {
			t.Error("expected the service not to be called when validation fails")
		}
		body := decodeErrorBody(t, w)
		if !hasField(body, "displayName") {
			t.Errorf("expected a field error for %q, got %+v", "displayName", body.Fields)
		}
	})

	t.Run("Fail when Content-Type is missing", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		buf, _ := json.Marshal(tutors.TutorFields{DisplayName: "John"})
		req := authed(httptest.NewRequest(http.MethodPost, "/tutors/", bytes.NewReader(buf)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnsupportedMediaType, w.Code, w.Body.String())
		}
		if svc.createCalled {
			t.Error("expected the service not to be called")
		}
	})

	t.Run("Fail when the body is malformed JSON", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		req := authed(httptest.NewRequest(http.MethodPost, "/tutors/", strings.NewReader("{not json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		body := decodeErrorBody(t, w)
		if body.Code != "invalid.json" {
			t.Errorf("expected code %q, got %q", "invalid.json", body.Code)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/tutors/", tutors.TutorFields{DisplayName: "John"}))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.createCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("Success updates the tutor and the path id wins over the body", func(t *testing.T) {
		svc := &fakeService{
			updateFn: func(ctx context.Context, in tutors.UpdateIn) (tutors.Tutor, error) {
				return tutors.Tutor{ID: in.ID, DisplayName: in.DisplayName}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/tutors/7", tutors.UpdateIn{
			ID: 999, TutorFields: tutors.TutorFields{DisplayName: "John"},
		})))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if svc.gotUpdate.ID != 7 {
			t.Errorf("expected the path id (7) to override the body id (999), got %d", svc.gotUpdate.ID)
		}
	})

	t.Run("Fail when the body is invalid", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/tutors/7", tutors.UpdateIn{
			TutorFields: tutors.TutorFields{DisplayName: ""},
		})))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.updateCalled {
			t.Error("expected the service not to be called when validation fails")
		}
		body := decodeErrorBody(t, w)
		if !hasField(body, "displayName") {
			t.Errorf("expected a field error for %q, got %+v", "displayName", body.Fields)
		}
	})

	t.Run("Fail when the path id is not numeric", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/tutors/abc", tutors.UpdateIn{TutorFields: tutors.TutorFields{DisplayName: "John"}})))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.updateCalled {
			t.Error("expected the service not to be called with an invalid id")
		}
	})

	t.Run("Fail when the tutor does not exist", func(t *testing.T) {
		svc := &fakeService{
			updateFn: func(context.Context, tutors.UpdateIn) (tutors.Tutor, error) {
				return tutors.Tutor{}, tutors.ErrNotFound
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/tutors/999", tutors.UpdateIn{TutorFields: tutors.TutorFields{DisplayName: "John"}})))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPut, "/tutors/7", tutors.UpdateIn{TutorFields: tutors.TutorFields{DisplayName: "John"}}))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.updateCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("Success deletes with an empty body", func(t *testing.T) {
		svc := &fakeService{
			deleteFn: func(context.Context, int) error { return nil },
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/tutors/7", nil)))

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
		}
		if w.Body.Len() != 0 {
			t.Errorf("expected an empty body, got %q", w.Body.String())
		}
		if svc.gotDelete != 7 {
			t.Errorf("expected id 7 to reach the service, got %d", svc.gotDelete)
		}
	})

	t.Run("Fail when the id is not numeric", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/tutors/abc", nil)))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.deleteCalled {
			t.Error("expected the service not to be called with an invalid id")
		}
	})

	t.Run("Fail when the tutor does not exist", func(t *testing.T) {
		svc := &fakeService{
			deleteFn: func(context.Context, int) error { return tutors.ErrNotFound },
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/tutors/999", nil)))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/tutors/7", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.deleteCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})
}

func TestRouting(t *testing.T) {
	t.Run("An unregistered method on /tutors/{id} is rejected", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodPatch, "/tutors/7", nil)))

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d: %s", http.StatusMethodNotAllowed, w.Code, w.Body.String())
		}
	})
}

func TestGetMe(t *testing.T) {
	t.Run("Success returns the tutor for the authenticated user", func(t *testing.T) {
		svc := &fakeService{
			getByUserIDFn: func(ctx context.Context, userId int) (tutors.Tutor, error) {
				return tutors.Tutor{ID: 1, DisplayName: "John"}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/me", nil)))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if svc.gotGetByUserID != 1 {
			t.Errorf("expected the authenticated user's id (1) to reach the service, got %d", svc.gotGetByUserID)
		}
		var got tutors.Tutor
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.DisplayName != "John" {
			t.Errorf("expected the service's tutor in the body, got %+v", got)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tutors/me", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.getByUserIDCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})

	t.Run("Fail when authenticated user is not a tutor", func(t *testing.T) {
		svc := fakeService{
			getByUserIDFn: func(ctx context.Context, i int) (tutors.Tutor, error) {
				return tutors.Tutor{}, tutors.ErrNotFound
			},
		}
		h := mounted(t, &svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/me", nil)))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})
}

func TestGetStudentsByTutorID(t *testing.T) {
	t.Run("Success returns the tutor's students", func(t *testing.T) {
		enr := &fakeEnrollmentsService{
			getByTutorIDFn: func(ctx context.Context, tutorID int) ([]enrollments.Enrollment, error) {
				return []enrollments.Enrollment{{StudentID: 9}}, nil
			},
		}
		std := &fakeStudentsService{
			getByIDFn: func(ctx context.Context, id int) (students.Student, error) {
				return students.Student{ID: id, DisplayName: "Jane"}, nil
			},
		}
		h := mountedWithDeps(t, &fakeService{}, enr, std)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/7/students", nil)))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if enr.gotGetByTutorID != 7 {
			t.Errorf("expected tutor id 7 to reach the enrollments service, got %d", enr.gotGetByTutorID)
		}
		if std.gotGetByID != 9 {
			t.Errorf("expected student id 9 to reach the students service, got %d", std.gotGetByID)
		}
		var got []students.Student
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != 1 || got[0].DisplayName != "Jane" {
			t.Errorf("expected Jane in the body, got %+v", got)
		}
	})

	t.Run("Fail when the id is not numeric", func(t *testing.T) {
		enr := &fakeEnrollmentsService{}
		h := mountedWithDeps(t, &fakeService{}, enr, &fakeStudentsService{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/abc/students", nil)))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if enr.getByTutorIDCalled {
			t.Error("expected the enrollments service not to be called with an invalid id")
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		enr := &fakeEnrollmentsService{}
		h := mountedWithDeps(t, &fakeService{}, enr, &fakeStudentsService{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tutors/7/students", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if enr.getByTutorIDCalled {
			t.Error("expected the enrollments service not to be called when unauthenticated")
		}
	})

	t.Run("Propagates an enrollments service error", func(t *testing.T) {
		enr := &fakeEnrollmentsService{
			getByTutorIDFn: func(context.Context, int) ([]enrollments.Enrollment, error) {
				return nil, errors.New("db is on fire")
			},
		}
		h := mountedWithDeps(t, &fakeService{}, enr, &fakeStudentsService{})

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/tutors/7/students", nil)))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}
