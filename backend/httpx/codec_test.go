package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyDecoder(t *testing.T) {
	t.Run("Success when body is valid JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice","count":3}`))
		r.Header.Set("Content-Type", "application/json")

		decoded, err := BodyDecoder[payload](r)
		if err != nil {
			t.Fatal("failed to decode valid body", err)
		}

		if decoded.Name != "alice" {
			t.Errorf("expected name %q, got %q", "alice", decoded.Name)
		}
		if decoded.Count != 3 {
			t.Errorf("expected count %d, got %d", 3, decoded.Count)
		}
	})

	t.Run("Success when body has unknown fields", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice","nope":true}`))
		r.Header.Set("Content-Type", "application/json")

		decoded, err := BodyDecoder[payload](r)
		if err != nil {
			t.Fatal("expected unknown fields to be ignored", err)
		}

		if decoded.Name != "alice" {
			t.Errorf("expected name %q, got %q", "alice", decoded.Name)
		}
	})

	t.Run("Success when body has trailing content", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"first"}{"name":"second"}`))
		r.Header.Set("Content-Type", "application/json")

		decoded, err := BodyDecoder[payload](r)
		if err != nil {
			t.Fatal("expected trailing content to be ignored", err)
		}

		// Only the first JSON value is consumed; the rest is never inspected.
		if decoded.Name != "first" {
			t.Errorf("expected name %q, got %q", "first", decoded.Name)
		}
	})

	t.Run("Fail when body is malformed JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
		r.Header.Set("Content-Type", "application/json")

		if _, err := BodyDecoder[payload](r); err == nil {
			t.Fatal("expected error when decoding malformed JSON, got nil")
		}
	})

	t.Run("Fail when body is empty", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Content-Type", "application/json")

		_, err := BodyDecoder[payload](r)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF for an empty body, got %v", err)
		}
	})

	t.Run("Fail when field type mismatches", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"count":"not a number"}`))
		r.Header.Set("Content-Type", "application/json")

		_, err := BodyDecoder[payload](r)
		if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
			t.Fatalf("expected *json.UnmarshalTypeError, got %v", err)
		}

		// This is why wrap withholds decoder errors from clients: the message
		// names the Go struct field and its type.
		for _, leaked := range []string{"payload.count", "of type int"} {
			if !strings.Contains(err.Error(), leaked) {
				t.Errorf("expected the decoder error to leak %q, got %q", leaked, err.Error())
			}
		}
	})

	t.Run("Fail when body is empty and zero value is returned", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Content-Type", "application/json")

		decoded, err := BodyDecoder[payload](r)
		if err == nil {
			t.Fatal("expected error for an empty body, got nil")
		}

		if decoded != (payload{}) {
			t.Errorf("expected the zero value alongside the error, got %+v", decoded)
		}
	})

	t.Run("Fail when Content-Type is absent", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))

		_, err := BodyDecoder[payload](r)
		if !errors.Is(err, ErrContentTypeNotJSON) {
			t.Fatalf("expected ErrContentTypeNotJSON, got %v", err)
		}
	})

	t.Run("Fail when Content-Type is not JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		r.Header.Set("Content-Type", "text/plain")

		_, err := BodyDecoder[payload](r)
		if !errors.Is(err, ErrContentTypeNotJSON) {
			t.Fatalf("expected ErrContentTypeNotJSON, got %v", err)
		}
	})
}

func TestNoContent(t *testing.T) {
	t.Run("Success when value is discarded", func(t *testing.T) {
		w := newRecordingWriter()

		if err := NoContent(w, payload{Name: "ignored"}); err != nil {
			t.Fatal("NoContent should never fail", err)
		}

		if w.status != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, w.status)
		}
		if w.body.Len() != 0 {
			t.Errorf("expected an empty body, got %q", w.body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "" {
			t.Errorf("expected no Content-Type, got %q", got)
		}
	})
}

func TestOk(t *testing.T) {
	t.Run("Success when value is marshalable", func(t *testing.T) {
		w := newRecordingWriter()

		if err := OK(w, payload{Name: "alice", Count: 3}); err != nil {
			t.Fatal("failed to encode a valid value", err)
		}

		if w.status != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.status)
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", got)
		}
		// json.Encoder terminates every value with a newline.
		want := `{"name":"alice","count":3}` + "\n"
		if got := w.body.String(); got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
	})
}

func TestCreated(t *testing.T) {
	t.Run("Success when value is marshalable", func(t *testing.T) {
		w := newRecordingWriter()

		if err := Created(w, payload{Name: "alice", Count: 3}); err != nil {
			t.Fatal("failed to encode a valid value", err)
		}

		if w.status != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.status)
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", got)
		}
		want := `{"name":"alice","count":3}` + "\n"
		if got := w.body.String(); got != want {
			t.Errorf("expected body %q, got %q", want, got)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("Fail when value is not marshalable", func(t *testing.T) {
		w := newRecordingWriter()

		err := writeJSON(w, http.StatusOK, unmarshalable{})
		if err == nil {
			t.Fatal("expected error when encoding an unmarshalable value, got nil")
		}

		// The response must be left completely untouched, so that the caller can
		// still turn the failure into an error response.
		if w.wroteHeader {
			t.Errorf("expected no status to be committed, got %d", w.status)
		}
		if w.body.Len() != 0 {
			t.Errorf("expected an empty body, got %q", w.body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "" {
			t.Errorf("expected no Content-Type, got %q", got)
		}
	})

	t.Run("Fail when the writer fails", func(t *testing.T) {
		w := newRecordingWriter()
		w.writeErr = errWrite

		err := writeJSON(w, http.StatusOK, payload{Name: "alice"})
		if !errors.Is(err, errWrite) {
			t.Fatalf("expected the write error to be propagated, got %v", err)
		}

		// Unrecoverable, unlike the case above: the status is already sent.
		if !w.wroteHeader || w.status != http.StatusOK {
			t.Errorf("expected status %d to be committed, got %d", http.StatusOK, w.status)
		}
	})

	t.Run("Success when status is forwarded", func(t *testing.T) {
		w := newRecordingWriter()

		if err := writeJSON(w, http.StatusAccepted, payload{Name: "alice"}); err != nil {
			t.Fatal("failed to encode a valid value", err)
		}

		if w.status != http.StatusAccepted {
			t.Errorf("expected status %d, got %d", http.StatusAccepted, w.status)
		}
	})
}
