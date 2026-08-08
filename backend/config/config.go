package config

import (
	"log/slog"
	"net"
	"strconv"
	"time"
)

// Config is the fully resolved, validated application configuration.
type Config struct {
	AppMode   AppMode
	Log       Log
	DB        DB
	Server    Server
	Scheduler Scheduler
}

// Log configures the application logger.
type Log struct {
	Level  slog.Level
	File   string
	Format LogFormat
}

// DB configures the SQLite database connection.
type DB struct {
	File         string
	ReadPoolSize int
}

// Server configures the HTTP server.
type Server struct {
	Host                 string
	Port                 int
	ShutdownTimeout      time.Duration
	ReadinessDrainPeriod time.Duration
}

// Address returns the host:port pair the server should listen on.
func (s Server) Address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

// Scheduler configures the background task scheduler.
type Scheduler struct {
	// Location is the time.Location schedules are evaluated in.
	Location *time.Location
	// DrainPeriod is how long an in-flight task run may finish on a live
	// context after shutdown begins, before it is cancelled.
	DrainPeriod time.Duration
	// ShutdownTimeout bounds the scheduler's entire shutdown.
	ShutdownTimeout time.Duration
}
