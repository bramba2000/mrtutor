package config

import (
	"os"
	"strconv"
	"time"
)

func getString[T ~string](key string, defaultValue T) T {
	if value, ok := os.LookupEnv(key); ok {
		return T(value)
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getInt[T ~int](key string, defaultValue T) T {
	if value, ok := os.LookupEnv(key); ok {
		if intValue, err := strconv.Atoi(value); err == nil {
			return T(intValue)
		}
	}
	return defaultValue
}
