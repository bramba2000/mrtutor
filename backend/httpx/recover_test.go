package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecover(t *testing.T) {
	t.Run("No panic leaves the response and the log untouched", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := Recover(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		w := newRecordingWriter()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusOK {
			t.Errorf("status = %d, want %d", w.status, http.StatusOK)
		}
		if w.body.String() != "OK" {
			t.Errorf("body = %q, want %q", w.body.String(), "OK")
		}
		if logs.buf.Len() != 0 {
			t.Errorf("expected no log output, got %q", logs.buf.String())
		}
	})

	t.Run("A string panic is recovered, logged, and mapped to a 500", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := Recover(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))

		w := newRecordingWriter()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.status, http.StatusInternalServerError)
		}
		logs.requireLogged(t, "panic recovered", "boom")
	})

	t.Run("An error panic is recovered, logged, and mapped via WriteError", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := Recover(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(errBoom)
		}))

		w := newRecordingWriter()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.status, http.StatusInternalServerError)
		}
		logs.requireLogged(t, "panic recovered", errBoom.Error())
	})

	t.Run("A non-string, non-error panic is recovered as an unknown panic", func(t *testing.T) {
		logs := newCapturedLogs()
		handler := Recover(logs.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(42)
		}))

		w := newRecordingWriter()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.status != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.status, http.StatusInternalServerError)
		}
		// The panic value (42) isn't a publicError, so WriteError falls back
		// to its generic body; "unknown panic" only reaches the log, via
		// WriteError's own error-logging of the errors.New("unknown panic")
		// recover.go constructs.
		var body errorBody
		if err := json.NewDecoder(&w.body).Decode(&body); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if body.Code != "internal" {
			t.Errorf("code = %q, want %q", body.Code, "internal")
		}
		logs.requireLogged(t, "panic recovered", "unknown panic")
	})

	t.Run("http.ErrAbortHandler is re-panicked, not recovered", func(t *testing.T) {
		handler := Recover(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}))

		defer func() {
			rec := recover()
			if rec == nil {
				t.Fatal("expected the panic to propagate, but it was recovered")
			}
			err, ok := rec.(error)
			if !ok || !errors.Is(err, http.ErrAbortHandler) {
				t.Fatalf("recovered value = %v, want http.ErrAbortHandler", rec)
			}
		}()

		handler.ServeHTTP(newRecordingWriter(), httptest.NewRequest(http.MethodGet, "/", nil))
		t.Fatal("expected ServeHTTP to panic")
	})
}
