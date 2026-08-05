package config

import "strings"

// AppMode selects the application's runtime mode.
type AppMode string

const (
	AppModeDev  AppMode = "development"
	AppModeProd AppMode = "production"
)

// parseAppMode normalizes known aliases ("dev", "prod") to their canonical
// value. Unknown input is passed through unchanged so validation can name
// the offending value; this function never fails.
func parseAppMode(s string) (AppMode, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "dev", string(AppModeDev):
		return AppModeDev, nil
	case "prod", string(AppModeProd):
		return AppModeProd, nil
	default:
		return AppMode(v), nil
	}
}

// LogFormat selects the logger's output encoding.
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// parseLogFormat normalizes case; unknown input passes through unchanged so
// validation can name the offending value. This function never fails.
func parseLogFormat(s string) (LogFormat, error) {
	return LogFormat(strings.ToLower(strings.TrimSpace(s))), nil
}
