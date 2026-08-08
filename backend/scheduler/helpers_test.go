package scheduler_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// capturedLogs is a logger paired with the buffer its records are written
// to, mirroring httpx/helpers_test.go's fixture of the same name.
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
