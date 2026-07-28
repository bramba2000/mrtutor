package config

import (
	"net"
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
