package config

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

func parseString(s string) (string, error) {
	return s, nil
}

func parseInt(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	return v, nil
}

func parseDuration(s string) (time.Duration, error) {
	v, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return 0, errors.New(`must be a duration such as "15s" or "500ms"`)
	}
	return v, nil
}

// parseLevel accepts either a numeric slog level (e.g. "-4", as used by
// Taskfile.yml) or a level name (e.g. "DEBUG", "warn"). The numeric form is
// tried first: slog.Level.UnmarshalText only accepts a "+"/"-" offset after a
// name, so it rejects a bare "-4" that the numeric branch must handle.
func parseLevel(s string) (slog.Level, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return slog.Level(n), nil
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(s)); err != nil {
		return 0, errors.New("must be a level name (DEBUG, INFO, WARN, ERROR) or a numeric slog level")
	}
	return lvl, nil
}

// parseLocation loads an IANA time zone name (e.g. "Europe/Rome"). It relies
// on the system zoneinfo database, which the Phase 8 glibc-based image
// provides; a scratch image would need a blank import of time/tzdata.
func parseLocation(s string) (*time.Location, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(s))
	if err != nil {
		return nil, errors.New("must be a valid IANA time zone name, such as \"UTC\" or \"Europe/Rome\"")
	}
	return loc, nil
}
