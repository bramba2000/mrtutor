package http

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

var (
	// errBoom stands in for any failure raised by a collaborator.
	errBoom = errors.New("boom")
	// errWrite stands in for a broken connection mid-response.
	errWrite = errors.New("write failed")
)

// recordingWriter is an http.ResponseWriter that reports whether WriteHeader was
// called at all.
//
// httptest.ResponseRecorder cannot do this: its Code field is pre-seeded to 200,
// so "never wrote a header" and "wrote 200" are indistinguishable. Telling them
// apart is the whole point of the encode-failure tests, since leaving the
// response untouched is what lets the caller substitute an error response.
type recordingWriter struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
	// writeErr, when set, is returned by Write instead of consuming the bytes.
	writeErr error
}

func newRecordingWriter() *recordingWriter {
	return &recordingWriter{header: make(http.Header)}
}

func (w *recordingWriter) Header() http.Header {
	return w.header
}

func (w *recordingWriter) WriteHeader(status int) {
	if w.wroteHeader {
		// Mirror net/http, which ignores a superfluous WriteHeader.
		return
	}
	w.wroteHeader = true
	w.status = status
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.body.Write(b)
}

// unmarshalable holds a func field, which encoding/json always rejects.
type unmarshalable struct {
	Fn func()
}

// payload is the ordinary value used to check successful encoding and decoding.
type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// validableValue declares Validate on its value type, so it satisfies Validable
// both as validableValue and as *validableValue.
type validableValue struct {
	err   error
	calls *int
}

func (v validableValue) Validate() error {
	// calls is a pointer so that the count survives the receiver being copied.
	if v.calls != nil {
		*v.calls++
	}
	return v.err
}

// validablePointer declares Validate on its pointer type only, so a
// validablePointer value does not satisfy Validable.
type validablePointer struct {
	err   error
	calls *int
}

func (v *validablePointer) Validate() error {
	if v.calls != nil {
		*v.calls++
	}
	return v.err
}

// capturedLogs is a logger paired with the buffer its records are written to.
type capturedLogs struct {
	logger *slog.Logger
	buf    *bytes.Buffer
}

func newCapturedLogs() *capturedLogs {
	buf := &bytes.Buffer{}
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return &capturedLogs{logger: slog.New(handler), buf: buf}
}

// requireLogged fails the test unless every fragment appears in the log output.
func (c *capturedLogs) requireLogged(t *testing.T, fragments ...string) {
	t.Helper()
	logged := c.buf.String()
	for _, fragment := range fragments {
		if !strings.Contains(logged, fragment) {
			t.Errorf("expected log to contain %q, got %q", fragment, logged)
		}
	}
}

// discardLogger returns a logger that drops every record.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
