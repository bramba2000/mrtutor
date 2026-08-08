// Package scheduler runs registered background tasks on their own
// goroutines according to a Schedule, mirroring how httpx.Server owns its
// own lifecycle: bind/validate synchronously, then a blocking Run(ctx) error
// that returns nil on a clean shutdown.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
)

const (
	// DefaultDrainPeriod is used by New when Config.DrainPeriod is zero or
	// negative.
	DefaultDrainPeriod = 5 * time.Second
	// DefaultShutdownTimeout is used by New when Config.ShutdownTimeout is
	// zero or negative.
	DefaultShutdownTimeout = 15 * time.Second
)

// ErrAlreadyRunning is returned by Run if it is called more than once on the
// same Scheduler.
var ErrAlreadyRunning = errors.New("scheduler: Run already called")

// Config configures a Scheduler.
type Config struct {
	// Logger receives lifecycle and task logging. A nil Logger discards
	// everything.
	Logger *slog.Logger
	// Location is the time.Location Run's start time is evaluated in, which
	// every fire time derived from it inherits (see Window.Loc). Nil
	// defaults to time.UTC.
	Location *time.Location
	// DrainPeriod is how long in-flight runs may finish on a live context
	// after shutdown begins, before the Scheduler cancels them. Zero or
	// negative falls back to DefaultDrainPeriod.
	DrainPeriod time.Duration
	// ShutdownTimeout bounds the entire shutdown: draining, and — if that
	// doesn't finish in time — the subsequent wait for cancellation to take
	// effect. Zero or negative falls back to DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
}

// Scheduler runs a fixed set of Tasks, one goroutine per Task, according to
// each Task's Schedule. Tasks must be Registered before Run is called; Run
// blocks until ctx is cancelled or a task returns an error wrapping
// ErrFatal. The zero Scheduler is not usable — construct one with New.
type Scheduler struct {
	logger          *slog.Logger
	location        *time.Location
	drainPeriod     time.Duration
	shutdownTimeout time.Duration

	mu      sync.Mutex
	tasks   []Task
	started bool
}

// New builds a Scheduler from cfg, applying defaults for anything left
// unset. New never fails; Register reports mistakes made registering a task,
// and Run reports failures at runtime.
func New(cfg Config) *Scheduler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger = logger.With("component", "scheduler")

	location := cfg.Location
	if location == nil {
		location = time.UTC
	}

	drainPeriod := cfg.DrainPeriod
	if drainPeriod <= 0 {
		drainPeriod = DefaultDrainPeriod
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}

	return &Scheduler{
		logger:          logger,
		location:        location,
		drainPeriod:     drainPeriod,
		shutdownTimeout: shutdownTimeout,
	}
}

// Register adds t to the set of tasks Run will schedule. It must be called
// before Run; calling it afterwards returns an error classified as
// errs.Invalid, the same as any other misconfiguration Register rejects.
func (s *Scheduler) Register(t Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errs.Domain("scheduler.alreadyStarted", "cannot register a task after Run has started", errs.Invalid)
	}

	fieldErrs := validation.Errors{}
	fieldErrs.Add("name", validation.Validate(t.Name, validation.NotBlank)...)
	if t.Run == nil {
		fieldErrs.Add("run", errors.New("must not be nil"))
	}
	if t.Schedule == nil {
		fieldErrs.Add("schedule", errors.New("must not be nil"))
	} else if v, ok := t.Schedule.(interface{ Validate() error }); ok {
		fieldErrs.Add("schedule", v.Validate())
	}
	if t.Timeout < 0 {
		fieldErrs.Add("timeout", errors.New("must not be negative"))
	}
	for _, existing := range s.tasks {
		if existing.Name == t.Name {
			fieldErrs.Add("name", fmt.Errorf("task %q is already registered", t.Name))
			break
		}
	}

	if err := fieldErrs.Err(); err != nil {
		return err
	}

	s.tasks = append(s.tasks, t)
	return nil
}

// Run schedules every registered task and blocks until ctx is cancelled or a
// task reports a fatal error — including when there are no tasks, or when
// every task's Schedule has been exhausted, since Run is meant to share an
// errgroup with other long-lived components (see httpx.Server.Run) and
// returning early would tear those down too.
//
// On shutdown, in-flight runs are given up to Config.DrainPeriod to finish on
// a live context before their context is cancelled; the whole shutdown is
// bounded by Config.ShutdownTimeout. A clean shutdown returns nil, never
// ctx.Err() — mirroring the httpx.Server rule that a cancelled context is
// not itself a failure.
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.started = true
	tasks := slices.Clone(s.tasks)
	s.mu.Unlock()

	start := time.Now().In(s.location)
	s.logger.Info("scheduler starting", "tasks", len(tasks))

	// schedCtx stops the run loops from scheduling further runs. It is
	// cancelled either by the caller (ctx) or, the moment a task reports a
	// fatal error, by us below — so a fatal failure stops every other task
	// from firing again immediately, rather than waiting for the next
	// external cancellation.
	schedCtx, cancelSched := context.WithCancel(ctx)
	defer cancelSched()

	// taskCtx is what Task.Run actually receives. It is detached from ctx
	// (context.WithoutCancel) so an in-flight run can keep going during the
	// drain period even though the caller has already cancelled ctx; it is
	// cancelled explicitly, and only, once the drain period elapses below.
	taskCtx, cancelTasks := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelTasks()

	var wg sync.WaitGroup
	fatalCh := make(chan error, 1)
	for _, t := range tasks {
		wg.Add(1)
		go s.runTask(schedCtx, taskCtx, start, t, &wg, fatalCh)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var fatalErr error
	select {
	case <-ctx.Done():
	case fatalErr = <-fatalCh:
		cancelSched()
	}

	s.logger.Info("scheduler shutting down, draining in-flight runs", "drainPeriod", s.drainPeriod)
	shutdownStart := time.Now()

	if !s.awaitDone(done, s.drainPeriod) {
		s.logger.Warn("drain period elapsed, cancelling in-flight runs")
		cancelTasks()
		if !s.awaitDone(done, s.shutdownTimeout-time.Since(shutdownStart)) {
			s.logger.Error("tasks still running after shutdown timeout, abandoning")
			return errors.Join(fatalErr, errors.New("scheduler: shutdown timed out waiting for tasks"))
		}
	}

	return fatalErr
}

// awaitDone waits for done to close, up to timeout. A non-positive timeout
// still gets one non-blocking check, since time.NewTimer treats it as due
// immediately.
func (s *Scheduler) awaitDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// runTask drives a single task for the scheduler's whole lifetime: exactly
// one goroutine per task, so two runs of the same task can never overlap —
// skipping a missed fire time is a consequence of this loop's shape, not a
// policy it has to check for.
func (s *Scheduler) runTask(schedCtx, taskCtx context.Context, start time.Time, t Task, wg *sync.WaitGroup, fatalCh chan<- error) {
	defer wg.Done()

	var last time.Time
	for {
		next, ok := s.nextFireTime(t, start, &last)
		if !ok {
			s.logger.Info("task has no further runs", "task", t.Name)
			return
		}

		timer := time.NewTimer(time.Until(next))
		select {
		case <-schedCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		last = next
		if err := s.runOnce(taskCtx, t); err != nil && errors.Is(err, ErrFatal) {
			select {
			case fatalCh <- fmt.Errorf("task %q: %w", t.Name, err):
			default:
			}
			return
		}
	}
}

// nextFireTime returns the next time t should run at or after now, skipping
// (and, if any were skipped, logging once at Warn) every fire time already
// in the past — e.g. because the previous run overran its own interval.
// *last is advanced past every skipped fire time. This re-query is only
// correct because Schedule.Next is required to be pure.
//
// The very first fire (last still zero on entry) is never skipped, however
// far in the past it computes to. Skipping exists to catch up after a real
// run overran its interval; a task's first run has no previous run to have
// overrun. Treating it as skippable would misfire in production: start is
// captured once, before any task goroutine is spawned, so by the time this
// runs, real wall-clock time has already advanced past a same-instant next
// (e.g. Every with no initial delay) by whatever scheduling latency the Go
// runtime happened to introduce — nothing this package's tests can pin down
// with testing/synctest's fake clock, since it never advances during plain
// CPU work. Losing every task's first run to that latency would be a much
// worse bug than the one skipping was added to fix.
func (s *Scheduler) nextFireTime(t Task, start time.Time, last *time.Time) (time.Time, bool) {
	next, ok := t.Schedule.Next(start, *last)
	if !ok || last.IsZero() {
		return next, ok
	}

	skipped := 0
	now := time.Now()
	for next.Before(now) {
		skipped++
		*last = next
		next, ok = t.Schedule.Next(start, *last)
		if !ok {
			return next, false
		}
	}
	if skipped > 0 {
		s.logger.Warn("runs skipped, previous run overran", "task", t.Name, "skipped", skipped)
	}
	return next, true
}

// runOnce executes a single run of t.Run, applying Task.Timeout and
// recovering any panic. A panic is logged like any other failure and is
// never fatal — only an error wrapping ErrFatal is (see runTask).
func (s *Scheduler) runOnce(ctx context.Context, t Task) (err error) {
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	started := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			s.logger.Error("task panicked", "task", t.Name, "panic", rec, "stack", string(debug.Stack()))
			err = fmt.Errorf("task %q panicked: %v", t.Name, rec)
			return
		}
		if err != nil {
			s.logger.Error("task run failed", "task", t.Name, "error", err, "duration", time.Since(started))
			return
		}
		s.logger.Debug("task run succeeded", "task", t.Name, "duration", time.Since(started))
	}()

	return t.Run(ctx)
}
