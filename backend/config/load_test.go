package config_test

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/config"
	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
)

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.AppMode != config.AppModeDev {
		t.Errorf("AppMode = %q, want %q", cfg.AppMode, config.AppModeDev)
	}
	if cfg.Log.Level != slog.LevelInfo {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, slog.LevelInfo)
	}
	if cfg.Log.File != "" {
		t.Errorf("Log.File = %q, want empty", cfg.Log.File)
	}
	if cfg.Log.Format != config.LogFormatText {
		t.Errorf("Log.Format = %q, want %q (console default)", cfg.Log.Format, config.LogFormatText)
	}
	if cfg.DB.File != "data.db" {
		t.Errorf("DB.File = %q, want %q", cfg.DB.File, "data.db")
	}
	if cfg.DB.ReadPoolSize != 4 {
		t.Errorf("DB.ReadPoolSize = %d, want 4", cfg.DB.ReadPoolSize)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "localhost")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("Server.ShutdownTimeout = %v, want 15s", cfg.Server.ShutdownTimeout)
	}
	if cfg.Server.ReadinessDrainPeriod != 5*time.Second {
		t.Errorf("Server.ReadinessDrainPeriod = %v, want 5s", cfg.Server.ReadinessDrainPeriod)
	}
	if got := cfg.Server.Address(); got != "localhost:8080" {
		t.Errorf("Server.Address() = %q, want %q", got, "localhost:8080")
	}
}

func TestLoadDefaultFormatFollowsLogFile(t *testing.T) {
	cfg, err := config.Load(lookupFrom(map[string]string{"LOG_FILE": "/tmp/app.log"}))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Log.Format != config.LogFormatJSON {
		t.Errorf("Log.Format = %q, want %q (file default)", cfg.Log.Format, config.LogFormatJSON)
	}
}

func TestLoadAllValidValues(t *testing.T) {
	cfg, err := config.Load(lookupFrom(map[string]string{
		"APP_MODE":               "prod",
		"LOG_LEVEL":              "DEBUG",
		"LOG_FILE":               "/tmp/app.log",
		"LOG_FORMAT":             "text",
		"DATABASE_FILE":          "app.db",
		"READ_POOL_SIZE":         "8",
		"HOST":                   "0.0.0.0",
		"PORT":                   "9090",
		"SHUTDOWN_TIMEOUT":       "30s",
		"READINESS_DRAIN_PERIOD": "10s",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.AppMode != config.AppModeProd {
		t.Errorf("AppMode = %q, want %q", cfg.AppMode, config.AppModeProd)
	}
	if cfg.Log.Level != slog.LevelDebug {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, slog.LevelDebug)
	}
	if cfg.Log.File != "/tmp/app.log" {
		t.Errorf("Log.File = %q, want %q", cfg.Log.File, "/tmp/app.log")
	}
	if cfg.Log.Format != config.LogFormatText {
		t.Errorf("Log.Format = %q, want %q (explicit override)", cfg.Log.Format, config.LogFormatText)
	}
	if cfg.DB.File != "app.db" {
		t.Errorf("DB.File = %q, want %q", cfg.DB.File, "app.db")
	}
	if cfg.DB.ReadPoolSize != 8 {
		t.Errorf("DB.ReadPoolSize = %d, want 8", cfg.DB.ReadPoolSize)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeout != 30*time.Second {
		t.Errorf("Server.ShutdownTimeout = %v, want 30s", cfg.Server.ShutdownTimeout)
	}
	if cfg.Server.ReadinessDrainPeriod != 10*time.Second {
		t.Errorf("Server.ReadinessDrainPeriod = %v, want 10s", cfg.Server.ReadinessDrainPeriod)
	}
}

func TestLoadLogLevelForms(t *testing.T) {
	cases := map[string]slog.Level{
		"-4":     slog.LevelDebug,
		"0":      slog.LevelInfo,
		"DEBUG":  slog.LevelDebug,
		"debug":  slog.LevelDebug,
		"WARN":   slog.LevelWarn,
		"WARN+2": slog.LevelWarn + 2,
		"ERROR":  slog.LevelError,
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			cfg, err := config.Load(lookupFrom(map[string]string{"LOG_LEVEL": raw}))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if cfg.Log.Level != want {
				t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, want)
			}
		})
	}
}

func TestLoadTaskfileDefault(t *testing.T) {
	// Taskfile.yml sets LOG_LEVEL: -4 for `task run`; pin that it still works.
	cfg, err := config.Load(lookupFrom(map[string]string{"LOG_LEVEL": "-4"}))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Log.Level != slog.LevelDebug {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, slog.LevelDebug)
	}
}

func TestLoadInvalidValues(t *testing.T) {
	cases := map[string]map[string]string{
		"bad READ_POOL_SIZE":   {"READ_POOL_SIZE": "abc"},
		"zero READ_POOL_SIZE":  {"READ_POOL_SIZE": "0"},
		"bad SHUTDOWN_TIMEOUT": {"SHUTDOWN_TIMEOUT": "abc"},
		"negative timeout":     {"SHUTDOWN_TIMEOUT": "-5s"},
		"bad APP_MODE":         {"APP_MODE": "staging"},
		"bad LOG_FORMAT":       {"LOG_FORMAT": "yaml"},
		"bad LOG_LEVEL":        {"LOG_LEVEL": "nonsense"},
		"port too large":       {"PORT": "70000"},
		"port negative":        {"PORT": "-1"},
		"blank DATABASE_FILE":  {"DATABASE_FILE": "   "},
		"blank LOG_FILE":       {"LOG_FILE": "   "},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(lookupFrom(env))
			if err == nil {
				t.Fatalf("Load() error = nil, want error")
			}
			if !errors.Is(err, errs.Invalid) {
				t.Errorf("errors.Is(err, errs.Invalid) = false, want true (err: %v)", err)
			}
		})
	}
}

func TestLoadAccumulatesAllErrors(t *testing.T) {
	_, err := config.Load(lookupFrom(map[string]string{
		"READ_POOL_SIZE": "abc",
		"APP_MODE":       "staging",
		"LOG_FORMAT":     "yaml",
	}))
	if err == nil {
		t.Fatalf("Load() error = nil, want error")
	}

	verrs, ok := errors.AsType[validation.Errors](err)
	if !ok {
		t.Fatalf("errors.AsType[validation.Errors](err) = false, want true (err type %T)", err)
	}

	for _, key := range []string{"READ_POOL_SIZE", "APP_MODE", "LOG_FORMAT"} {
		if _, ok := verrs[key]; !ok {
			t.Errorf("expected an error for %q, fields present: %v", key, verrs.Fields())
		}
	}
}

func TestLoadUnsetLookupFallsBackToOSLookupEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Server.Port = %d, want 9999", cfg.Server.Port)
	}
}
