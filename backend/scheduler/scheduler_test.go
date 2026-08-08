// Tests below use testing/synctest so timing-dependent behaviour (fires,
// skips, drain/cancel) is deterministic instead of racing real wall-clock
// sleeps — the same reason the package itself introduces no Clock seam
// (see auth/service.go's direct time.Now().UTC() calls for the posture this
// keeps): a fake clock inside a bubble makes one unnecessary here too.
package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/scheduler"
)

func TestScheduler_Register(t *testing.T) {
	validRun := func(ctx context.Context) error { return nil }
	validSchedule := scheduler.Every(time.Minute)

	tt := []struct {
		name    string
		task    scheduler.Task
		setup   func(t *testing.T, s *scheduler.Scheduler)
		wantErr bool
	}{
		{
			name: "valid task",
			task: scheduler.Task{Name: "t", Run: validRun, Schedule: validSchedule},
		},
		{
			name:    "empty name",
			task:    scheduler.Task{Name: "", Run: validRun, Schedule: validSchedule},
			wantErr: true,
		},
		{
			name:    "nil Run",
			task:    scheduler.Task{Name: "t", Schedule: validSchedule},
			wantErr: true,
		},
		{
			name:    "nil Schedule",
			task:    scheduler.Task{Name: "t", Run: validRun},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			task:    scheduler.Task{Name: "t", Run: validRun, Schedule: validSchedule, Timeout: -time.Second},
			wantErr: true,
		},
		{
			name:    "invalid schedule",
			task:    scheduler.Task{Name: "t", Run: validRun, Schedule: scheduler.Every(0)},
			wantErr: true,
		},
		{
			name: "duplicate name",
			task: scheduler.Task{Name: "dup", Run: validRun, Schedule: validSchedule},
			setup: func(t *testing.T, s *scheduler.Scheduler) {
				t.Helper()
				if err := s.Register(scheduler.Task{Name: "dup", Run: validRun, Schedule: validSchedule}); err != nil {
					t.Fatalf("seeding the duplicate: Register() error = %v", err)
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := scheduler.New(scheduler.Config{Logger: discardLogger()})
			if tc.setup != nil {
				tc.setup(t, s)
			}

			err := s.Register(tc.task)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Register() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, errs.Invalid) {
				t.Errorf("error %v does not satisfy errors.Is(err, errs.Invalid)", err)
			}
		})
	}
}

func TestScheduler_Register_AfterRunStarted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()
		synctest.Wait()

		err := s.Register(scheduler.Task{
			Name:     "late",
			Run:      func(ctx context.Context) error { return nil },
			Schedule: scheduler.Every(time.Minute),
		})
		if !errors.Is(err, errs.Invalid) {
			t.Errorf("Register() after Run started: error = %v, want errs.Invalid", err)
		}

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_AlreadyRunning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()
		synctest.Wait()

		if err := s.Run(ctx); !errors.Is(err, scheduler.ErrAlreadyRunning) {
			t.Errorf("second Run() error = %v, want ErrAlreadyRunning", err)
		}

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_FiresRepeatedly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var runs atomic.Int32
		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		mustRegister(t, s, scheduler.Task{
			Name:     "hourly",
			Run:      func(ctx context.Context) error { runs.Add(1); return nil },
			Schedule: scheduler.Every(time.Hour),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		// Fires land at 0h, 1h, 2h; stop short of the 3h fire so the count
		// is unambiguous.
		time.Sleep(2*time.Hour + 30*time.Minute)
		synctest.Wait()

		if got := runs.Load(); got != 3 {
			t.Errorf("runs = %d, want 3", got)
		}

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_FatalErrorStopsScheduler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var siblingRuns atomic.Int32

		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		mustRegister(t, s, scheduler.Task{
			Name:     "boom",
			Run:      func(ctx context.Context) error { return fmt.Errorf("boom: %w", scheduler.ErrFatal) },
			Schedule: scheduler.Once(30 * time.Second),
		})
		mustRegister(t, s, scheduler.Task{
			Name: "sibling",
			Run:  func(ctx context.Context) error { siblingRuns.Add(1); return nil },
			// Fires immediately (once) and again an hour later — the
			// second fire must never happen once boom goes fatal.
			Schedule: scheduler.Every(time.Hour),
		})

		ctx := context.Background()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		time.Sleep(45 * time.Second) // past boom's fire at 30s

		select {
		case err := <-errCh:
			if !errors.Is(err, scheduler.ErrFatal) {
				t.Fatalf("Run() error = %v, want it to satisfy errors.Is(err, scheduler.ErrFatal)", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("scheduler did not stop after a fatal task error")
		}

		if got := siblingRuns.Load(); got != 1 {
			t.Errorf("sibling ran %d times, want exactly 1 (its immediate first run) — a fatal error must stop further scheduling immediately", got)
		}
	})
}

func TestScheduler_Run_PanicRecovered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := newCapturedLogs()
		var runs atomic.Int32

		s := scheduler.New(scheduler.Config{Logger: logs.logger})
		mustRegister(t, s, scheduler.Task{
			Name: "flaky",
			Run: func(ctx context.Context) error {
				if runs.Add(1) == 1 {
					panic("boom")
				}
				return nil
			},
			Schedule: scheduler.Every(time.Hour),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		time.Sleep(time.Hour + time.Minute)
		synctest.Wait()

		if got := runs.Load(); got != 2 {
			t.Fatalf("runs = %d, want 2 — a panic must not stop the scheduler", got)
		}
		logs.requireLogged(t, "task panicked", "flaky", "boom")

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_OverlapIsSkipped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := newCapturedLogs()
		var runs, active, maxActive atomic.Int32

		s := scheduler.New(scheduler.Config{Logger: logs.logger})
		mustRegister(t, s, scheduler.Task{
			Name: "slow",
			Run: func(ctx context.Context) error {
				n := active.Add(1)
				defer active.Add(-1)
				for {
					m := maxActive.Load()
					if n <= m || maxActive.CompareAndSwap(m, n) {
						break
					}
				}
				// Structurally, this package can never actually run this
				// task twice at once (one goroutine per task) — maxActive
				// is a spot check of that guarantee, not the mechanism
				// enforcing it.
				if runs.Add(1) == 1 {
					time.Sleep(90 * time.Minute) // spans the 1h fire entirely
				}
				return nil
			},
			Schedule: scheduler.Every(time.Hour),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		// Fires would land at 0h, 1h, 2h, 3h; the slow run occupying
		// [0h, 1h30m) swallows the 1h fire, so only 0h, 2h, 3h actually run.
		time.Sleep(3*time.Hour + 5*time.Minute)
		synctest.Wait()

		if got := maxActive.Load(); got != 1 {
			t.Errorf("max concurrent runs = %d, want 1 — overlapping runs must never happen", got)
		}
		if got := runs.Load(); got != 3 {
			t.Errorf("runs = %d, want 3 (one skipped)", got)
		}
		logs.requireLogged(t, "runs skipped", "slow")

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

// TestScheduler_Run_FirstFireIsNeverSkipped pins a bug caught only by
// running the real binary: every production Every()/Once() schedule with no
// initial delay computes its first fire as exactly start, and by the time a
// task's goroutine actually samples time.Now() — after Run has spawned every
// task's goroutine — real wall-clock time has already ticked past it, however
// slightly. The skip-catch-up logic in nextFireTime was treating that as an
// overrun and discarding the task's first, legitimate run. It cannot be
// reproduced by simply using a real Schedule under testing/synctest, because
// the fake clock never advances during plain CPU work — so this uses a
// ScheduleFunc that fabricates the same "first fire already due" condition
// on purpose.
func TestScheduler_Run_FirstFireIsNeverSkipped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := newCapturedLogs()
		var runs atomic.Int32

		alreadyDue := scheduler.ScheduleFunc(func(start, last time.Time) (time.Time, bool) {
			if !last.IsZero() {
				return time.Time{}, false // run exactly once
			}
			return start.Add(-time.Hour), true
		})

		s := scheduler.New(scheduler.Config{Logger: logs.logger})
		mustRegister(t, s, scheduler.Task{
			Name:     "backdated",
			Run:      func(ctx context.Context) error { runs.Add(1); return nil },
			Schedule: alreadyDue,
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()

		if got := runs.Load(); got != 1 {
			t.Fatalf("runs = %d, want 1 — the first fire must run even if already overdue", got)
		}
		if strings.Contains(logs.buf.String(), "runs skipped") {
			t.Error("the first fire must never be logged as skipped")
		}

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_TaskTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var timedOut atomic.Bool

		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		mustRegister(t, s, scheduler.Task{
			Name: "slow",
			Run: func(ctx context.Context) error {
				<-ctx.Done()
				timedOut.Store(errors.Is(ctx.Err(), context.DeadlineExceeded))
				return ctx.Err()
			},
			Schedule: scheduler.Once(0),
			Timeout:  time.Second,
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		time.Sleep(2 * time.Second)
		synctest.Wait()

		if !timedOut.Load() {
			t.Error("task context was not cancelled by its Timeout")
		}

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_DrainsBeforeCancelling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sawCancel atomic.Bool
		started := make(chan struct{})

		s := scheduler.New(scheduler.Config{Logger: discardLogger(), DrainPeriod: time.Minute})
		mustRegister(t, s, scheduler.Task{
			Name: "in-flight",
			Run: func(ctx context.Context) error {
				close(started)
				select {
				case <-ctx.Done():
					sawCancel.Store(true)
				case <-time.After(30 * time.Second): // finishes well inside DrainPeriod
				}
				return nil
			},
			Schedule: scheduler.Once(0),
		})

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		<-started
		cancel() // shutdown begins while the task is in flight

		requireCleanShutdown(t, errCh)
		if sawCancel.Load() {
			t.Error("task observed cancellation during the drain period")
		}
	})
}

func TestScheduler_Run_CancelsAfterDrainPeriodElapses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sawCancel atomic.Bool
		started := make(chan struct{})

		s := scheduler.New(scheduler.Config{
			Logger:          discardLogger(),
			DrainPeriod:     time.Minute,
			ShutdownTimeout: 5 * time.Minute,
		})
		mustRegister(t, s, scheduler.Task{
			Name: "outlasts-drain",
			Run: func(ctx context.Context) error {
				close(started)
				<-ctx.Done() // only unblocks once the drain period elapses and taskCtx is cancelled
				sawCancel.Store(true)
				return ctx.Err()
			},
			Schedule: scheduler.Once(0),
		})

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		<-started
		cancel()

		requireCleanShutdown(t, errCh)
		if !sawCancel.Load() {
			t.Error("task never observed cancellation after the drain period elapsed")
		}
	})
}

func TestScheduler_Run_ExhaustedScheduleStillBlocksUntilCancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var runs atomic.Int32
		s := scheduler.New(scheduler.Config{Logger: discardLogger()})
		mustRegister(t, s, scheduler.Task{
			Name:     "once",
			Run:      func(ctx context.Context) error { runs.Add(1); return nil },
			Schedule: scheduler.Once(0),
		})

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		time.Sleep(time.Minute) // long after the task's only run, and its own exit
		synctest.Wait()

		if got := runs.Load(); got != 1 {
			t.Fatalf("runs = %d, want 1", got)
		}
		requireStillRunning(t, errCh)

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func TestScheduler_Run_ZeroTasksStillBlocksUntilCancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := scheduler.New(scheduler.Config{Logger: discardLogger()})

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()

		synctest.Wait()
		requireStillRunning(t, errCh)

		cancel()
		requireCleanShutdown(t, errCh)
	})
}

func mustRegister(t *testing.T, s *scheduler.Scheduler, task scheduler.Task) {
	t.Helper()
	if err := s.Register(task); err != nil {
		t.Fatalf("Register(%q) error = %v", task.Name, err)
	}
}

func requireStillRunning(t *testing.T, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		t.Fatalf("Run() returned early with err = %v; it must block until ctx is cancelled", err)
	default:
	}
}

// shutdownWatchdog must exceed the longest drain/cancel sequence any test
// below deliberately runs (currently CancelsAfterDrainPeriodElapses's
// DrainPeriod+ShutdownTimeout, 6 minutes) — firing it early would kill the
// synctest main goroutine while a task's goroutine is still legitimately
// durably blocked, which synctest reports as a deadlock rather than as this
// helper's own timeout.
const shutdownWatchdog = 10 * time.Minute

func requireCleanShutdown(t *testing.T, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(shutdownWatchdog):
		t.Fatal("scheduler did not shut down")
	}
}
