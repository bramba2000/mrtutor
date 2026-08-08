package scheduler

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Schedule reports when a task should run.
//
// start is the time Scheduler.Run began — fixed for the process lifetime, so
// schedules relative to startup need no internal state. last is the previous
// fire time, or the zero Time if the task has never run. ok is false when
// the task must never run again: a Once that has fired, or a window that has
// permanently closed.
//
// Implementations must be pure: the same arguments must always produce the
// same result, and Next must never mutate the receiver. Purity is what makes
// a Schedule safe to share between tasks, testable as an unordered table,
// and safe for the run loop to re-query after a skipped fire time (see
// Scheduler.nextFireTime). A future Cron(expr) satisfies this interface by
// using last as its base, falling back to start on the first call.
type Schedule interface {
	Next(start, last time.Time) (next time.Time, ok bool)
}

// ScheduleFunc adapts a function to Schedule.
type ScheduleFunc func(start, last time.Time) (time.Time, bool)

// Next implements [Schedule].
func (f ScheduleFunc) Next(start, last time.Time) (time.Time, bool) {
	return f(start, last)
}

// once is shared by Once and OnceAt: fire exactly once, at whatever at
// reports, then never again.
type once struct {
	at func(start time.Time) time.Time
}

// Next implements [Schedule].
func (o once) Next(start, last time.Time) (time.Time, bool) {
	if !last.IsZero() {
		return time.Time{}, false
	}
	return o.at(start), true
}

// Once returns a Schedule that fires exactly once, delay after the scheduler
// starts.
func Once(delay time.Duration) Schedule {
	return once{at: func(start time.Time) time.Time { return start.Add(delay) }}
}

// OnceAt returns a Schedule that fires exactly once, at the given absolute
// instant.
func OnceAt(t time.Time) Schedule {
	return once{at: func(start time.Time) time.Time { return t }}
}

// Periodic is a Schedule that fires at a fixed interval, optionally delayed
// on its first run and optionally restricted to a Window.
type Periodic struct {
	interval time.Duration
	delay    time.Duration
	// window is a pointer so the zero Periodic means "unrestricted" without
	// an ambiguous zero Window (From==To==0 could otherwise mean either
	// "always open" or "never open").
	window *Window
}

// Every returns a Periodic firing every interval, starting when the
// scheduler starts.
func Every(interval time.Duration) Periodic {
	return Periodic{interval: interval}
}

// After delays p's first run by delay past the scheduler's start time. It
// does not affect the interval between subsequent runs.
func (p Periodic) After(delay time.Duration) Periodic {
	p.delay = delay
	return p
}

// Within restricts p to run only inside w, the mechanism this package uses
// to express suspension ("every 5 minutes, but only weekdays 09:00-18:00")
// without any runtime Suspend/Resume state: a fire time that lands outside w
// is shifted forward to w's next opening rather than skipped.
func (p Periodic) Within(w Window) Periodic {
	p.window = &w
	return p
}

// Next implements [Schedule].
func (p Periodic) Next(start, last time.Time) (time.Time, bool) {
	next := start.Add(p.delay)
	if !last.IsZero() {
		next = last.Add(p.interval)
	}
	if p.window == nil {
		return next, true
	}
	return p.window.shift(next)
}

// Validate reports whether p is well-formed. Scheduler.Register calls it
// (via a type assertion — Periodic is the only built-in Schedule that can be
// misconfigured) so a bad interval or window fails at registration instead
// of silently never firing.
func (p Periodic) Validate() error {
	if p.interval <= 0 {
		return errors.New("interval must be positive")
	}
	if p.delay < 0 {
		return errors.New("initial delay must not be negative")
	}
	if p.window != nil {
		return p.window.validate()
	}
	return nil
}

// Window is a recurring wall-clock span, used by Periodic.Within to restrict
// a schedule to certain days and hours.
type Window struct {
	// Days restricts the window to these weekdays. Empty means every day.
	Days []time.Weekday
	// From and To are offsets from local midnight; the window is open on
	// [From, To). Both must be within [0, 24h], and From must be before To.
	From, To time.Duration
	// Loc is the location From/To and Days are evaluated in. Nil defaults to
	// the location of the fire time being tested, which — since start and
	// every fire time derived from it carry the location time.Now().In(loc)
	// gave them — is ordinarily the Scheduler's own Config.Location.
	Loc *time.Location
}

// Daily returns a Window open every day, from midnight+from to midnight+to.
// Chain .In(loc) to pin it to a specific location; otherwise it inherits the
// location of whatever fire time it is tested against (see Window.Loc).
func Daily(from, to time.Duration) Window {
	return Window{From: from, To: to}
}

// Weekdays returns a Window open Monday-Friday, from fromHour to toHour
// (0-24). See Daily for how its location is decided.
func Weekdays(fromHour, toHour int) Window {
	return Daily(time.Duration(fromHour)*time.Hour, time.Duration(toHour)*time.Hour).
		On(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
}

// On restricts w to the given weekdays, replacing any it already had.
func (w Window) On(days ...time.Weekday) Window {
	w.Days = slices.Clone(days) // decouple from the caller's slice, as httpx.Router.Group does for its middleware slice
	return w
}

// In sets the location w is evaluated in.
func (w Window) In(loc *time.Location) Window {
	w.Loc = loc
	return w
}

func (w Window) validate() error {
	if w.From < 0 || w.From > 24*time.Hour {
		return errors.New("window From must be between 0 and 24h")
	}
	if w.To < 0 || w.To > 24*time.Hour {
		return errors.New("window To must be between 0 and 24h")
	}
	if w.From >= w.To {
		return errors.New("window From must be before To")
	}
	for _, d := range w.Days {
		if d < time.Sunday || d > time.Saturday {
			return fmt.Errorf("window has invalid weekday %v", d)
		}
	}
	return nil
}

// maxScanDays bounds how far ahead shift looks for the window's next
// opening. A week plus a buffer day is enough for any Days/From/To
// combination that validate accepts.
const maxScanDays = 8

// shift returns the earliest instant at or after t that falls inside w,
// scanning at most maxScanDays days ahead. It assumes w has already passed
// validate. In practice ok is always true for such a w: validate requires
// From < To and restricts Days to real weekdays, and every weekday recurs at
// least once within any maxScanDays-day span, so a validated window always
// finds an opening. The bool return exists so a future refinement of Window
// (e.g. an excluded-dates list) can express "closed forever" without
// changing Schedule's shape.
func (w Window) shift(t time.Time) (time.Time, bool) {
	loc := w.Loc
	if loc == nil {
		loc = t.Location()
	}
	local := t.In(loc)

	for i := 0; i <= maxScanDays; i++ {
		day := local.AddDate(0, 0, i)
		midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
		if !w.dayAllowed(midnight.Weekday()) {
			continue
		}

		from := midnight.Add(w.From)
		to := midnight.Add(w.To)

		if i == 0 {
			switch {
			case local.Before(from):
				return from, true
			case local.Before(to):
				return t, true // already inside the window; don't shift it
			default:
				continue // past today's window, try the next allowed day
			}
		}
		return from, true
	}
	return time.Time{}, false
}

func (w Window) dayAllowed(d time.Weekday) bool {
	if len(w.Days) == 0 {
		return true
	}
	return slices.Contains(w.Days, d)
}
