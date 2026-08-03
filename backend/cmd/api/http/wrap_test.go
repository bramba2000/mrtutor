package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
)

// ctxKey marks the value used to prove the request context reaches the handler.
type ctxKey struct{}

func TestWrap(t *testing.T) {
	t.Run("Success when decode, handler and encode succeed", func(t *testing.T) {
		w := newRecordingWriter()
		gotPath := ""
		gotInput := payload{}

		decode := func(r *http.Request) (payload, error) {
			gotPath = r.URL.Path
			return payload{Name: "alice"}, nil
		}
		fn := func(ctx context.Context, in payload) (payload, error) {
			gotInput = in
			return payload{Name: in.Name, Count: 7}, nil
		}

		h := wrap(decode, fn, ok[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/principals", nil))

		if gotPath != "/principals" {
			t.Errorf("expected the decoder to receive the request, got path %q", gotPath)
		}
		if gotInput.Name != "alice" {
			t.Errorf("expected the handler to receive the decoded input, got %+v", gotInput)
		}
		if w.status != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.status)
		}
		want := `{"name":"alice","count":7}` + "\n"
		if got := w.body.String(); got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
	})

	t.Run("Success when handler receives the request context", func(t *testing.T) {
		w := newRecordingWriter()
		gotValue := any(nil)

		fn := func(ctx context.Context, _ payload) (payload, error) {
			gotValue = ctx.Value(ctxKey{})
			return payload{}, nil
		}

		ctx := context.WithValue(t.Context(), ctxKey{}, "carried")
		r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)

		h := wrap(decodeZero, fn, noContent[payload], discardLogger())
		h(w, r)

		if gotValue != "carried" {
			t.Errorf("expected the request context to reach the handler, got %v", gotValue)
		}
	})

	t.Run("Fail when decode returns an error", func(t *testing.T) {
		w := newRecordingWriter()
		fnCalled, encodeCalled := false, false

		decode := func(r *http.Request) (payload, error) {
			return payload{}, errBoom
		}
		fn := func(ctx context.Context, _ payload) (payload, error) {
			fnCalled = true
			return payload{}, nil
		}
		encode := func(w http.ResponseWriter, _ payload) error {
			encodeCalled = true
			return nil
		}

		h := wrap(decode, fn, encode, discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		// errBoom is unclassified, so it falls through to the 500 default; see
		// TestWrap/"Fail when decode returns a classified error" for the case
		// that proves wrap forwards the error faithfully instead of flattening
		// it to a fixed status.
		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if fnCalled {
			t.Error("expected the handler not to run after a decode failure")
		}
		if encodeCalled {
			t.Error("expected the encoder not to run after a decode failure")
		}
	})

	t.Run("Fail when decode returns a classified error", func(t *testing.T) {
		w := newRecordingWriter()

		decode := func(r *http.Request) (payload, error) {
			return payload{}, errs.Domain("bad", "bad input", errs.Invalid)
		}

		h := wrap(decode, handlerZero, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		// Proves wrap hands the decode error to writeError untouched, rather
		// than mapping every decode failure to a fixed status itself.
		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
	})

	t.Run("Fail when input validation fails", func(t *testing.T) {
		w := newRecordingWriter()
		calls := 0
		fnCalled := false

		errValidation := errors.New("name is required")
		decode := func(r *http.Request) (validableValue, error) {
			return validableValue{err: errValidation, calls: &calls}, nil
		}
		fn := func(ctx context.Context, _ validableValue) (payload, error) {
			fnCalled = true
			return payload{}, nil
		}

		h := wrap(decode, fn, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		// A bare Validate error is classified as errs.Invalid by wrap itself
		// (not by writeError), guaranteeing a 400 regardless of what the
		// caller's Validate returns.
		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
		if calls != 1 {
			t.Errorf("expected Validate to be called once, got %d", calls)
		}
		if fnCalled {
			t.Error("expected the handler not to run after a validation failure")
		}
	})

	t.Run("Fail when input validation fails with validation.Errors", func(t *testing.T) {
		w := newRecordingWriter()

		validationErr := validation.Errors{"name": []error{errors.New("is required")}}.Err()
		decode := func(r *http.Request) (validableValue, error) {
			return validableValue{err: validationErr}, nil
		}
		fn := func(ctx context.Context, _ validableValue) (payload, error) {
			return payload{}, nil
		}

		h := wrap(decode, fn, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		// validation.Errors already reports Is(errs.Invalid) (see
		// validation/errors.go), so wrap's classification must not rewrap it —
		// doing so would sever the type chain writeError relies on to recover
		// the per-field detail.
		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
		var body errorBody
		if err := json.Unmarshal(w.body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if len(body.Fields) != 1 || body.Fields[0].Name != "name" {
			t.Errorf("expected the validation.Errors field detail to survive, got %+v", body.Fields)
		}
	})

	t.Run("Success when input validation passes", func(t *testing.T) {
		w := newRecordingWriter()
		calls := 0
		fnCalled := false

		decode := func(r *http.Request) (validableValue, error) {
			return validableValue{calls: &calls}, nil
		}
		fn := func(ctx context.Context, _ validableValue) (payload, error) {
			fnCalled = true
			return payload{}, nil
		}

		h := wrap(decode, fn, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if calls != 1 {
			t.Errorf("expected Validate to be called once, got %d", calls)
		}
		if !fnCalled {
			t.Error("expected the handler to run after successful validation")
		}
		if w.status != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, w.status)
		}
	})

	t.Run("Success when Validate is declared on the pointer type and input is a value", func(t *testing.T) {
		w := newRecordingWriter()
		calls := 0
		fnCalled := false

		// validablePointer declares Validate on *validablePointer, which is not in
		// the method set of the value type. any(in).(Validable) therefore fails and
		// validation is skipped entirely, despite Validate returning an error.
		decode := func(r *http.Request) (validablePointer, error) {
			return validablePointer{err: errBoom, calls: &calls}, nil
		}
		fn := func(ctx context.Context, _ validablePointer) (payload, error) {
			fnCalled = true
			return payload{}, nil
		}

		h := wrap(decode, fn, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if calls != 0 {
			t.Errorf("expected Validate to be skipped, got %d calls", calls)
		}
		if !fnCalled {
			t.Error("expected the handler to run when validation is skipped")
		}
		if w.status != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, w.status)
		}
	})

	t.Run("Fail when Validate is declared on the pointer type and input is a pointer", func(t *testing.T) {
		w := newRecordingWriter()
		calls := 0
		fnCalled := false

		decode := func(r *http.Request) (*validablePointer, error) {
			return &validablePointer{err: errBoom, calls: &calls}, nil
		}
		fn := func(ctx context.Context, _ *validablePointer) (payload, error) {
			fnCalled = true
			return payload{}, nil
		}

		h := wrap(decode, fn, noContent[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if calls != 1 {
			t.Errorf("expected Validate to be called once, got %d", calls)
		}
		if fnCalled {
			t.Error("expected the handler not to run after a validation failure")
		}
		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
	})

	t.Run("Fail when handler returns an error", func(t *testing.T) {
		w := newRecordingWriter()
		encodeCalled := false

		fn := func(ctx context.Context, _ payload) (payload, error) {
			return payload{}, errBoom
		}
		encode := func(w http.ResponseWriter, _ payload) error {
			encodeCalled = true
			return nil
		}

		h := wrap(decodeZero, fn, encode, discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if encodeCalled {
			t.Error("expected the encoder not to run after a handler failure")
		}
	})

	t.Run("Fail when encode returns an error", func(t *testing.T) {
		w := newRecordingWriter()

		encode := func(w http.ResponseWriter, _ payload) error {
			return errBoom
		}

		h := wrap(decodeZero, handlerZero, encode, discardLogger())
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
	})

	t.Run("Success when logger is nil and the handler fails", func(t *testing.T) {
		w := newRecordingWriter()

		fn := func(ctx context.Context, _ payload) (payload, error) {
			return payload{}, errBoom
		}

		h := wrap(decodeZero, fn, noContent[payload], nil)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
	})
}

func TestWrapNoInput(t *testing.T) {
	t.Run("Success when handler and encode succeed", func(t *testing.T) {
		w := newRecordingWriter()

		fn := func(ctx context.Context) (payload, error) {
			return payload{Name: "alice", Count: 7}, nil
		}

		h := wrapNoInput(fn, ok[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.status)
		}
		want := `{"name":"alice","count":7}` + "\n"
		if got := w.body.String(); got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
	})

	t.Run("Success when request body is invalid JSON", func(t *testing.T) {
		w := newRecordingWriter()

		fn := func(ctx context.Context) (payload, error) {
			return payload{Name: "alice"}, nil
		}

		// Proof that the internal decoder never reads the body.
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not json at all`))

		h := wrapNoInput(fn, ok[payload], discardLogger())
		h(w, r)

		if w.status != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.status)
		}
	})

	t.Run("Success when handler receives the request context", func(t *testing.T) {
		w := newRecordingWriter()
		gotValue := any(nil)

		fn := func(ctx context.Context) (payload, error) {
			gotValue = ctx.Value(ctxKey{})
			return payload{}, nil
		}

		ctx := context.WithValue(t.Context(), ctxKey{}, "carried")
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		h := wrapNoInput(fn, noContent[payload], discardLogger())
		h(w, r)

		if gotValue != "carried" {
			t.Errorf("expected the request context to reach the handler, got %v", gotValue)
		}
	})

	t.Run("Fail when handler returns an error", func(t *testing.T) {
		w := newRecordingWriter()

		fn := func(ctx context.Context) (payload, error) {
			return payload{}, errBoom
		}

		h := wrapNoInput(fn, ok[payload], discardLogger())
		h(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
	})

	t.Run("Fail when encode returns an error", func(t *testing.T) {
		w := newRecordingWriter()

		fn := func(ctx context.Context) (payload, error) {
			return payload{}, nil
		}
		encode := func(w http.ResponseWriter, _ payload) error {
			return errBoom
		}

		h := wrapNoInput(fn, encode, discardLogger())
		h(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
	})
}

// decodeZero is a decoder that always succeeds with the zero payload.
func decodeZero(r *http.Request) (payload, error) {
	return payload{}, nil
}

// handlerZero is a handler that always succeeds with the zero payload.
func handlerZero(ctx context.Context, _ payload) (payload, error) {
	return payload{}, nil
}
