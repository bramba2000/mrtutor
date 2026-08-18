package studentshttp_test

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
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/students/studentshttp"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

// Package studentshttp_test is external, so these tests only reach the
// exported surface — the handler is driven through the real router, with a
// fake Service and a fake Authenticator standing in for the database.

// fakeService lets each test control what the domain layer returns, without
// a database in the loop.
type fakeService struct {
	createFn  func(context.Context, students.CreateIn) (students.Student, error)
	getByIDFn func(context.Context, int) (students.Student, error)
	getAllFn  func(context.Context) ([]students.Student, error)
	updateFn  func(context.Context, students.UpdateIn) (students.Student, error)
	deleteFn  func(context.Context, int) error

	createCalled  bool
	getByIDCalled bool
	getAllCalled  bool
	updateCalled  bool
	deleteCalled  bool

	gotCreate  students.CreateIn
	gotGetByID int
	gotUpdate  students.UpdateIn
	gotDelete  int
}

func (f *fakeService) Create(ctx context.Context, in students.CreateIn) (students.Student, error) {
	f.createCalled = true
	f.gotCreate = in
	return f.createFn(ctx, in)
}

func (f *fakeService) GetByID(ctx context.Context, id int) (students.Student, error) {
	f.getByIDCalled = true
	f.gotGetByID = id
	return f.getByIDFn(ctx, id)
}

func (f *fakeService) GetAll(ctx context.Context) ([]students.Student, error) {
	f.getAllCalled = true
	return f.getAllFn(ctx)
}

func (f *fakeService) Update(ctx context.Context, in students.UpdateIn) (students.Student, error) {
	f.updateCalled = true
	f.gotUpdate = in
	return f.updateFn(ctx, in)
}

func (f *fakeService) Delete(ctx context.Context, id int) error {
	f.deleteCalled = true
	f.gotDelete = id
	return f.deleteFn(ctx, id)
}

var (
	_ studentshttp.Service = (*fakeService)(nil)
	_ studentshttp.Service = students.Service{}
	_ httpx.Validable      = students.CreateIn{}
	_ httpx.Validable      = students.UpdateIn{}
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

func mounted(t *testing.T, svc studentshttp.Service) *httpx.Router {
	t.Helper()
	r := httpx.NewRouter("")
	l := slog.New(slog.NewTextHandler(t.Output(), nil))
	studentshttp.NewHandler(svc, fakeAuthenticator{}, l).Mount(r)
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
	t.Run("Success returns the students as JSON", func(t *testing.T) {
		svc := &fakeService{
			getAllFn: func(context.Context) ([]students.Student, error) {
				return []students.Student{{ID: 1, DisplayName: "John"}}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/students/", nil)))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		var got []students.Student
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != 1 || got[0].DisplayName != "John" {
			t.Errorf("expected the service's students in the body, got %+v", got)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/students/", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.getAllCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})

	t.Run("Fail when the service returns an unclassified error", func(t *testing.T) {
		svc := &fakeService{
			getAllFn: func(context.Context) ([]students.Student, error) {
				return nil, errors.New("db is on fire")
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/students/", nil)))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "db is on fire") {
			t.Errorf("expected the error cause to be withheld, got body %q", w.Body.String())
		}
	})
}

func TestGetByID(t *testing.T) {
	t.Run("Success returns the student", func(t *testing.T) {
		svc := &fakeService{
			getByIDFn: func(ctx context.Context, id int) (students.Student, error) {
				return students.Student{ID: id, DisplayName: "John"}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/students/7", nil)))

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
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/students/abc", nil)))

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

	t.Run("Fail when the student does not exist", func(t *testing.T) {
		svc := &fakeService{
			getByIDFn: func(context.Context, int) (students.Student, error) {
				return students.Student{}, students.ErrNotFound
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodGet, "/students/999", nil)))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
		body := decodeErrorBody(t, w)
		if body.Code != "students.notFound" {
			t.Errorf("expected code %q, got %q", "students.notFound", body.Code)
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/students/7", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("Success creates the student", func(t *testing.T) {
		svc := &fakeService{
			createFn: func(ctx context.Context, in students.CreateIn) (students.Student, error) {
				return students.Student{ID: 1, DisplayName: in.DisplayName}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPost, "/students/", students.CreateIn{DisplayName: "John"})))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
		}
		if svc.gotCreate.DisplayName != "John" {
			t.Errorf("expected the decoded CreateIn to reach the service, got %+v", svc.gotCreate)
		}
		var got students.Student
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.ID != 1 {
			t.Errorf("expected the created student in the body, got %+v", got)
		}
	})

	t.Run("Fail when the display name is blank", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPost, "/students/", students.CreateIn{})))

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

		buf, _ := json.Marshal(students.CreateIn{DisplayName: "John"})
		req := authed(httptest.NewRequest(http.MethodPost, "/students/", bytes.NewReader(buf)))
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

		req := authed(httptest.NewRequest(http.MethodPost, "/students/", strings.NewReader("{not json")))
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
		h.ServeHTTP(w, newJSONRequest(http.MethodPost, "/students/", students.CreateIn{DisplayName: "John"}))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.createCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("Success updates the student and the path id wins over the body", func(t *testing.T) {
		svc := &fakeService{
			updateFn: func(ctx context.Context, in students.UpdateIn) (students.Student, error) {
				return students.Student{ID: in.ID, DisplayName: in.DisplayName}, nil
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/students/7", students.UpdateIn{
			ID: 999, DisplayName: "John",
		})))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}
		if svc.gotUpdate.ID != 7 {
			t.Errorf("expected the path id (7) to override the body id (999), got %d", svc.gotUpdate.ID)
		}
	})

	t.Run("Fail when the body is invalid", func(t *testing.T) {
		// Regression test: PUT used to run on WrapUnvalidated, so
		// UpdateIn.Validate never ran — and when it did run elsewhere it
		// panicked on a valid input. This proves PUT now validates and
		// returns a normal 400, not a 500.
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/students/7", students.UpdateIn{
			DisplayName: "",
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
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/students/abc", students.UpdateIn{DisplayName: "John"})))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.updateCalled {
			t.Error("expected the service not to be called with an invalid id")
		}
	})

	t.Run("Fail when the student does not exist", func(t *testing.T) {
		svc := &fakeService{
			updateFn: func(context.Context, students.UpdateIn) (students.Student, error) {
				return students.Student{}, students.ErrNotFound
			},
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(newJSONRequest(http.MethodPut, "/students/999", students.UpdateIn{DisplayName: "John"})))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newJSONRequest(http.MethodPut, "/students/7", students.UpdateIn{DisplayName: "John"}))

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
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/students/7", nil)))

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
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/students/abc", nil)))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
		if svc.deleteCalled {
			t.Error("expected the service not to be called with an invalid id")
		}
	})

	t.Run("Fail when the student does not exist", func(t *testing.T) {
		svc := &fakeService{
			deleteFn: func(context.Context, int) error { return students.ErrNotFound },
		}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodDelete, "/students/999", nil)))

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("Fail when unauthenticated", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/students/7", nil))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
		}
		if svc.deleteCalled {
			t.Error("expected the service not to be called when unauthenticated")
		}
	})
}

func TestRouting(t *testing.T) {
	t.Run("An unregistered method on /students/{id} is rejected", func(t *testing.T) {
		svc := &fakeService{}
		h := mounted(t, svc)

		w := httptest.NewRecorder()
		h.ServeHTTP(w, authed(httptest.NewRequest(http.MethodPatch, "/students/7", nil)))

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d: %s", http.StatusMethodNotAllowed, w.Code, w.Body.String())
		}
	})
}
