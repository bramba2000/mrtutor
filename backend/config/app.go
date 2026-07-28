package config

import "log/slog"

var (
	LogLevel = getInt("LOG_LEVEL", slog.LevelInfo)
	LogFile  = getString("LOG_FILE", "")
)
