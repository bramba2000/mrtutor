package scheduler

import (
	"context"
	"errors"
	"time"
)

// ErrFatal marks a failure the process cannot continue past. A task whose
// Run returns an error wrapping ErrFatal stops the Scheduler; Run returns
// that error, which callers typically propagate up to a non-zero process
// exit. A panic is never fatal — it is recovered and logged like any other
// run failure, since a task cannot decide the whole process should die from
// inside a panic.
var ErrFatal = errors.New("fatal task error")

// Task is a unit of work the Scheduler runs on its own Schedule.
type Task struct {
	// Name identifies the task in logs and error messages. Required, and
	// must be unique within a Scheduler.
	Name string
	// Run is the work to perform. It receives a context that is live for
	// the whole run except during a shutdown that outlasts
	// Config.DrainPeriod, at which point it is cancelled; Run should return
	// promptly once ctx is done. Required.
	Run func(ctx context.Context) error
	// Schedule decides when Run is called. Required.
	Schedule Schedule
	// Timeout bounds a single run of Run. Zero means no per-run timeout.
	Timeout time.Duration
}
