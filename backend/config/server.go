package config

import (
	"net"
	"os"
	"time"
)

var (
	Port                 = getString("PORT", "8080")
	Host                 = getString("HOST", "localhost")
	ShutdownTimeout      = getDuration("SHUTDOWN_TIMEOUT", 15*time.Second)
	ShutdownHardTimeout  = getDuration("SHUTDOWN_HARD_TIMEOUT", 3*time.Second)
	ReadinessDrainPeriod = getDuration("READINESS_DRAIN_PERIOD", 5*time.Second)
)

func Address() string {
	return net.JoinHostPort(Host, Port)
}

func getString(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
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
