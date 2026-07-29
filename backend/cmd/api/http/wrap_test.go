package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		logs := newCapturedLogs()
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

		h := wrap(decode, fn, encode, logs.logger)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
		if got, want := w.body.String(), msgInvalidBody+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		// Decoder errors name Go types and fields, so they must stay internal.
		if strings.Contains(w.body.String(), errBoom.Error()) {
			t.Errorf("expected the decoder error to be withheld, got body %q", w.body.String())
		}
		if fnCalled {
			t.Error("expected the handler not to run after a decode failure")
		}
		if encodeCalled {
			t.Error("expected the encoder not to run after a decode failure")
		}
		logs.requireLogged(t, "failed to decode request", errBoom.Error())
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

		if w.status != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
		}
		// Validation messages are written for clients, so they are forwarded.
		if got, want := w.body.String(), errValidation.Error()+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		if calls != 1 {
			t.Errorf("expected Validate to be called once, got %d", calls)
		}
		if fnCalled {
			t.Error("expected the handler not to run after a validation failure")
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
		logs := newCapturedLogs()
		encodeCalled := false

		fn := func(ctx context.Context, _ payload) (payload, error) {
			return payload{}, errBoom
		}
		encode := func(w http.ResponseWriter, _ payload) error {
			encodeCalled = true
			return nil
		}

		h := wrap(decodeZero, fn, encode, logs.logger)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if got, want := w.body.String(), msgInternalError+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		if strings.Contains(w.body.String(), errBoom.Error()) {
			t.Errorf("expected the handler error to be withheld, got body %q", w.body.String())
		}
		if encodeCalled {
			t.Error("expected the encoder not to run after a handler failure")
		}
		logs.requireLogged(t, "request handler failed", errBoom.Error())
	})

	t.Run("Fail when encode returns an error without writing", func(t *testing.T) {
		w := newRecordingWriter()
		logs := newCapturedLogs()

		encode := func(w http.ResponseWriter, _ payload) error {
			return errBoom
		}

		h := wrap(decodeZero, handlerZero, encode, logs.logger)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if got, want := w.body.String(), msgInternalError+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		logs.requireLogged(t, "failed to encode response", errBoom.Error())
	})

	t.Run("Fail when encode cannot marshal the value", func(t *testing.T) {
		w := newRecordingWriter()
		logs := newCapturedLogs()

		fn := func(ctx context.Context, _ payload) (unmarshalable, error) {
			return unmarshalable{}, nil
		}

		// ok buffers the encoding, so a value it cannot marshal leaves the
		// response untouched and the failure becomes a real 500.
		h := wrap(decodeZero, fn, ok[unmarshalable], logs.logger)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if got, want := w.body.String(), msgInternalError+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Errorf("expected the JSON Content-Type to be replaced, got %q", got)
		}
		logs.requireLogged(t, "failed to encode response", "unsupported type")
	})

	t.Run("Fail when encode fails after committing the status", func(t *testing.T) {
		w := newRecordingWriter()
		w.writeErr = errWrite
		logs := newCapturedLogs()

		h := wrap(decodeZero, handlerZero, ok[payload], logs.logger)
		h(w, httptest.NewRequest(http.MethodPost, "/", nil))

		// Unavoidable, and distinct from the case above: the body could not be
		// written, but 200 was already sent, so no 500 can replace it.
		if w.status != http.StatusOK {
			t.Errorf("expected the committed status %d to be retained, got %d", http.StatusOK, w.status)
		}
		logs.requireLogged(t, "failed to encode response", errWrite.Error())
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
		logs := newCapturedLogs()

		fn := func(ctx context.Context) (payload, error) {
			return payload{}, errBoom
		}

		h := wrapNoInput(fn, ok[payload], logs.logger)
		h(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		if got, want := w.body.String(), msgInternalError+"\n"; got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
		logs.requireLogged(t, "request handler failed", errBoom.Error())
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
