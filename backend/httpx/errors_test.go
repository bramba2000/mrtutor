package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
)

// TestWriteError covers status and body shape: which errs sentinel (or
// unclassified error) produces which status code, code string, message, and
// field list.
func TestWriteError(t *testing.T) {
	tt := []struct {
		name               string
		err                error
		expectedStatusCode int
		expectedCode       string
		expectedMessage    string
		expectedFields     []fieldEntry
		matchErrorResponse func(errorResponse map[string]any) error
	}{
		{
			name:               "invalid json",
			err:                json.Unmarshal([]byte("{\"name\":\"field\""), &struct{}{}),
			expectedStatusCode: 400,
			expectedCode:       "invalid.json",
			expectedMessage:    "invalid JSON",
		},
		{
			// BodyDecoder sees io.EOF for an empty request body.
			// isJsonDecodingError matches it (defect #11), so this
			// client-caused case is a 400, not a 500.
			name:               "empty body",
			err:                io.EOF,
			expectedStatusCode: 400,
			expectedCode:       "invalid.json",
			expectedMessage:    "invalid JSON",
		},
		{
			// Same regression guard, for a body that ends mid-value.
			name:               "truncated body",
			err:                io.ErrUnexpectedEOF,
			expectedStatusCode: 400,
			expectedCode:       "invalid.json",
			expectedMessage:    "invalid JSON",
		},
		{
			// BodyDecoder's Content-Type gate reports a distinct status and
			// code from a JSON decoding failure.
			name:               "content type not JSON",
			err:                ErrContentTypeNotJSON,
			expectedStatusCode: 415,
			expectedCode:       "invalid.contentType",
			expectedMessage:    "content type is not JSON",
		},
		{
			name:               "not found",
			err:                errs.Domain("notFound", "testing not found", errs.NotFound),
			expectedStatusCode: 404,
			expectedCode:       "notFound",
			expectedMessage:    "testing not found",
		},
		{
			name:               "unauthenticated",
			err:                errs.Domain("unauthenticated", "testing unauthenticated", errs.Unauthenticated),
			expectedStatusCode: 401,
			expectedCode:       "unauthenticated",
			expectedMessage:    "testing unauthenticated",
		},
		{
			name:               "forbidden",
			err:                errs.Domain("forbidden", "testing forbidden", errs.Forbidden),
			expectedStatusCode: 403,
			expectedCode:       "forbidden",
			expectedMessage:    "testing forbidden",
		},
		{
			name:               "precondition failed",
			err:                errs.Domain("preconditionFailed", "testing precondition failed", errs.FailedPrecondition),
			expectedStatusCode: 412,
			expectedCode:       "preconditionFailed",
			expectedMessage:    "testing precondition failed",
		},
		{
			name:               "unique constraint violation",
			err:                errs.Domain("uniqueConstraintViolation", "testing unique constraint violation", errs.Conflict),
			expectedStatusCode: 409,
			expectedCode:       "uniqueConstraintViolation",
			expectedMessage:    "testing unique constraint violation",
		},
		{
			name:               "validation error",
			err:                errs.Domain("validationError", "testing validation error", errs.Invalid),
			expectedStatusCode: 400,
			expectedCode:       "validationError",
			expectedMessage:    "testing validation error",
		},
		{
			name:               "wrapped sentinel error",
			err:                fmt.Errorf("%s: %w", "testentity", errs.Domain("test", "not found", errs.NotFound)),
			expectedStatusCode: 404,
			matchErrorResponse: func(errResp map[string]any) error {
				if val, ok := errResp["message"]; !ok {
					return errors.New("error field not found in response")
				} else if valS, ok := val.(string); !ok {
					return errors.New("error field is not a string")
				} else if valS != "not found" {
					return fmt.Errorf("unexpected error message: %s", valS)
				}
				return nil
			},
		},
		{
			name: "struct validation error",
			err: validation.Errors{
				"test": []error{errors.New("validation error")},
			}.Err(),
			expectedStatusCode: 400,
			expectedFields: []fieldEntry{
				{Name: "test", Message: "validation error"},
			},
		},
		{
			// Unclassified errors default to a generic 500 body, and their
			// cause must not leak into the response.
			name:               "unclassified error",
			err:                errors.New("boom, do not leak me"),
			expectedStatusCode: 500,
			expectedCode:       "internal",
			expectedMessage:    "internal error",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", http.NoBody)
			WriteError(w, r, tc.err, slog.New(slog.NewTextHandler(t.Output(), nil)))

			if tc.expectedStatusCode != 0 && w.Code != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, w.Code)
			}

			var body errorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response body: %v", err)
			}

			if tc.expectedCode != "" && body.Code != tc.expectedCode {
				t.Errorf("expected code %q, got %q", tc.expectedCode, body.Code)
			}
			if tc.expectedMessage != "" && body.Message != tc.expectedMessage {
				t.Errorf("expected message %q, got %q", tc.expectedMessage, body.Message)
			}
			if tc.expectedFields != nil {
				if got, want := body.Fields, tc.expectedFields; len(got) != len(want) {
					t.Errorf("expected fields %+v, got %+v", want, got)
				} else {
					for i := range want {
						if got[i] != want[i] {
							t.Errorf("expected fields %+v, got %+v", want, got)
							break
						}
					}
				}
			}
			if msg := tc.err.Error(); tc.expectedCode == "internal" && strings.Contains(w.Body.String(), msg) {
				t.Errorf("expected the error cause to be withheld, got body %q", w.Body.String())
			}

			if tc.matchErrorResponse != nil {
				var errorResponse map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &errorResponse); err != nil {
					t.Fatalf("failed to unmarshal response body: %v", err)
				}
				if err := tc.matchErrorResponse(errorResponse); err != nil {
					t.Logf("error response: %v", errorResponse)
					t.Errorf("error response validation failed: %v", err)
				}
			}
		})
	}
}

// TestWriteErrorLogging covers WriteError's logging: level and message are
// chosen from the response's status class, and the log always carries the
// request method, path, and the underlying error.
func TestWriteErrorLogging(t *testing.T) {
	t.Run("logs at ERROR with \"request failed\" for a 5xx", func(t *testing.T) {
		logs := newCapturedLogs()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/boom", http.NoBody)

		WriteError(w, r, errBoom, logs.logger)

		logs.requireLogged(t, "level=ERROR", "request failed", errBoom.Error(), http.MethodPost, "/boom")
	})

	t.Run("logs at INFO with \"request rejected\" for a 4xx", func(t *testing.T) {
		logs := newCapturedLogs()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/rejected", http.NoBody)

		err := errs.Domain("notFound", "testing not found", errs.NotFound)
		WriteError(w, r, err, logs.logger)

		logs.requireLogged(t, "level=INFO", "request rejected", err.Error(), http.MethodPost, "/rejected")
	})
}

// TestWriteErrorWriter covers WriteError's interaction with the
// http.ResponseWriter itself: content type, and the cases where the response
// cannot be written as intended. It uses recordingWriter rather than
// httptest.ResponseRecorder because the latter pre-seeds Code to 200, making
// "never wrote a header" indistinguishable from "wrote 200" — the distinction
// the already-committed-status case depends on.
func TestWriteErrorWriter(t *testing.T) {
	t.Run("sets a JSON Content-Type", func(t *testing.T) {
		w := newRecordingWriter()
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

		WriteError(w, r, errBoom, discardLogger())

		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", got)
		}
	})

	t.Run("retains an already-committed status", func(t *testing.T) {
		w := newRecordingWriter()
		w.WriteHeader(http.StatusOK)
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

		WriteError(w, r, errBoom, discardLogger())

		if w.status != http.StatusOK {
			t.Errorf("expected the committed status %d to be retained, got %d", http.StatusOK, w.status)
		}
	})

	t.Run("logs when the response cannot be written", func(t *testing.T) {
		w := newRecordingWriter()
		w.writeErr = errWrite
		logs := newCapturedLogs()
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

		WriteError(w, r, errBoom, logs.logger)

		logs.requireLogged(t, "failed to write error response", errWrite.Error())
	})

	t.Run("does not panic with a nil logger, and still writes the response", func(t *testing.T) {
		w := newRecordingWriter()
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

		WriteError(w, r, errBoom, nil)

		if w.status != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
		}
		var body errorBody
		if err := json.Unmarshal(w.body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal response body: %v", err)
		}
		if body.Code != "internal" {
			t.Errorf("expected code %q, got %q", "internal", body.Code)
		}
	})
}
