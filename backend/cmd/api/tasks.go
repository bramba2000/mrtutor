package main

import (
	"github.com/bramba2000/mrtutor/backend/scheduler"
)

// registerTasks registers every background job with the scheduler, mirroring
// registerRoutes — the one place a future feature adds a line. No jobs exist
// yet; session GC is a natural first one once Phase 7 lands
// SessionStore.DeleteExpired.
func registerTasks(s *scheduler.Scheduler, services Services) error {
	return nil
}
