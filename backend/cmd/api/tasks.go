package main

import (
	"errors"
	"time"

	"github.com/bramba2000/mrtutor/backend/scheduler"
)

// registerTasks registers every background job with the scheduler.
func registerTasks(s *scheduler.Scheduler, services Services) error {
	return errors.Join(
		s.Register(scheduler.Task{
			Name:     "cleanup sessions",
			Schedule: scheduler.Every(24 * time.Hour),
			Run:      services.Auth.CleanupExpiredSessions,
		}),
	)
}
