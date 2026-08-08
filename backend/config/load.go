package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/bramba2000/mrtutor/backend/validation"
)

// Load builds a Config from the environment, using lookup to read each
// variable (os.LookupEnv if lookup is nil, so tests can inject a map-backed
// stand-in via t.Setenv or a plain closure).
//
// Every setting has a default; supplying a value that fails to parse or
// fails validation does not fall back to the default silently — it is
// recorded and Load returns a non-nil error accumulating every problem
// found, keyed by environment variable name. The returned error satisfies
// errors.Is(err, errs.Invalid) (via validation.Errors), so callers that
// already map that to a "misconfiguration" exit code need no extra wiring.
func Load(lookup func(string) (string, bool)) (Config, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	l := &loader{lookup: lookup, errs: validation.Errors{}}

	logFile := get(l, "LOG_FILE", "", parseString, validation.NotBlank)

	// Preserve prior behaviour: file logs default to JSON, console logs
	// default to text. LOG_FORMAT can still override either default.
	defaultFormat := LogFormatText
	if logFile != "" {
		defaultFormat = LogFormatJSON
	}

	cfg := Config{
		AppMode: get(l, "APP_MODE", AppModeDev, parseAppMode,
			validation.OneOf(AppModeDev, AppModeProd)),
		Log: Log{
			Level: get(l, "LOG_LEVEL", slog.LevelInfo, parseLevel),
			File:  logFile,
			Format: get(l, "LOG_FORMAT", defaultFormat, parseLogFormat,
				validation.OneOf(LogFormatText, LogFormatJSON)),
		},
		DB: DB{
			File:         get(l, "DATABASE_FILE", "data.db", parseString, validation.NotBlank),
			ReadPoolSize: get(l, "READ_POOL_SIZE", 4, parseInt, validation.Min(1)),
		},
		Server: Server{
			// HOST intentionally has no NotBlank validator: an empty host
			// is meaningful (net.JoinHostPort("", port) binds all
			// interfaces), not a missing value.
			Host: get(l, "HOST", "localhost", parseString),
			// PORT intentionally allows 0 (ephemeral port), hence the
			// range starts at 0 rather than 1.
			Port: get(l, "PORT", 8080, parseInt, validation.MinMax(0, 65535)),
			ShutdownTimeout: get(l, "SHUTDOWN_TIMEOUT", 15*time.Second, parseDuration,
				validation.Min(time.Duration(0))),
			ReadinessDrainPeriod: get(l, "READINESS_DRAIN_PERIOD", 5*time.Second, parseDuration,
				validation.Min(time.Duration(0))),
		},
		// SCHEDULER_ prefixed, unlike every setting above: DRAIN_PERIOD and
		// SHUTDOWN_TIMEOUT are already taken by the HTTP server's own
		// settings, and the two components' values are independent.
		Scheduler: Scheduler{
			Location: get(l, "SCHEDULER_LOCATION", time.UTC, parseLocation),
			DrainPeriod: get(l, "SCHEDULER_DRAIN_PERIOD", 5*time.Second, parseDuration,
				validation.Min(time.Duration(0))),
			ShutdownTimeout: get(l, "SCHEDULER_SHUTDOWN_TIMEOUT", 15*time.Second, parseDuration,
				validation.Min(time.Duration(0))),
		},
	}

	return cfg, l.errs.Err()
}
