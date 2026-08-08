package scheduler_test

import (
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/scheduler"
)

// A Schedule.Next must be pure, so every row below stands on its own: there
// is no ordering dependency between them, unlike a stateful cursor would
// require.
func TestSchedule_Next(t *testing.T) {
	start := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC) // a Friday

	tt := []struct {
		name     string
		schedule scheduler.Schedule
		last     time.Time
		wantNext time.Time
		wantOK   bool
	}{
		{
			name:     "Once fires delay after start when it has never run",
			schedule: scheduler.Once(5 * time.Minute),
			last:     time.Time{},
			wantNext: start.Add(5 * time.Minute),
			wantOK:   true,
		},
		{
			name:     "Once never fires again once last is set",
			schedule: scheduler.Once(5 * time.Minute),
			last:     start.Add(5 * time.Minute),
			wantOK:   false,
		},
		{
			name:     "OnceAt fires at the absolute instant when it has never run",
			schedule: scheduler.OnceAt(start.Add(48 * time.Hour)),
			last:     time.Time{},
			wantNext: start.Add(48 * time.Hour),
			wantOK:   true,
		},
		{
			name:     "OnceAt never fires again once last is set",
			schedule: scheduler.OnceAt(start.Add(48 * time.Hour)),
			last:     start.Add(48 * time.Hour),
			wantOK:   false,
		},
		{
			name:     "Every fires at start on the first run",
			schedule: scheduler.Every(time.Hour),
			last:     time.Time{},
			wantNext: start,
			wantOK:   true,
		},
		{
			name:     "Every fires interval after the previous fire time",
			schedule: scheduler.Every(time.Hour),
			last:     start.Add(3 * time.Hour),
			wantNext: start.Add(4 * time.Hour),
			wantOK:   true,
		},
		{
			name:     "Every.After delays only the first run",
			schedule: scheduler.Every(time.Hour).After(10 * time.Minute),
			last:     time.Time{},
			wantNext: start.Add(10 * time.Minute),
			wantOK:   true,
		},
		{
			name:     "Every.After does not affect subsequent runs",
			schedule: scheduler.Every(time.Hour).After(10 * time.Minute),
			last:     start.Add(10 * time.Minute),
			wantNext: start.Add(70 * time.Minute),
			wantOK:   true,
		},
		{
			name: "Within shifts a fire time outside the window to its next opening",
			// start is a Friday; Every fires at start (Friday 00:00), which
			// falls outside a Monday-Friday 09:00-18:00 window and outside
			// business hours even on a weekday, so it shifts to Friday 09:00.
			schedule: scheduler.Every(24 * time.Hour).Within(scheduler.Weekdays(9, 18)),
			last:     time.Time{},
			wantNext: start.Add(9 * time.Hour),
			wantOK:   true,
		},
		{
			name:     "Within shifts a Saturday fire time to the following Monday",
			schedule: scheduler.Every(24 * time.Hour).Within(scheduler.Weekdays(9, 18)),
			// last is Friday 09:00, so the raw next fire (one day later) is
			// Saturday 09:00 — outside the window, shifted to Monday 09:00.
			last:     start.Add(9 * time.Hour),
			wantNext: start.Add(9*time.Hour).AddDate(0, 0, 3), // Monday 09:00
			wantOK:   true,
		},
		{
			name:     "Within leaves a fire time already inside the window untouched",
			schedule: scheduler.Every(time.Hour).Within(scheduler.Weekdays(9, 18)),
			// last is Friday 09:00, plus one hour lands at Friday 10:00 — inside the window.
			last:     start.Add(9 * time.Hour),
			wantNext: start.Add(10 * time.Hour),
			wantOK:   true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			next, ok := tc.schedule.Next(start, tc.last)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (next = %v)", ok, tc.wantOK, next)
			}
			if !tc.wantOK {
				return
			}
			if !next.Equal(tc.wantNext) {
				t.Errorf("next = %v, want %v", next, tc.wantNext)
			}
		})
	}
}

func TestPeriodic_Validate(t *testing.T) {
	tt := []struct {
		name     string
		schedule scheduler.Periodic
		wantErr  bool
	}{
		{name: "valid interval", schedule: scheduler.Every(time.Minute), wantErr: false},
		{name: "zero interval", schedule: scheduler.Every(0), wantErr: true},
		{name: "negative interval", schedule: scheduler.Every(-time.Minute), wantErr: true},
		{name: "negative initial delay", schedule: scheduler.Every(time.Minute).After(-time.Second), wantErr: true},
		{
			name:     "window with From after To",
			schedule: scheduler.Every(time.Minute).Within(scheduler.Daily(18*time.Hour, 9*time.Hour)),
			wantErr:  true,
		},
		{
			name:     "window with From equal to To",
			schedule: scheduler.Every(time.Minute).Within(scheduler.Daily(9*time.Hour, 9*time.Hour)),
			wantErr:  true,
		},
		{
			name:     "window with an out-of-range offset",
			schedule: scheduler.Every(time.Minute).Within(scheduler.Daily(-time.Hour, 9*time.Hour)),
			wantErr:  true,
		},
		{
			name:     "valid window",
			schedule: scheduler.Every(time.Minute).Within(scheduler.Weekdays(9, 18)),
			wantErr:  false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schedule.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
