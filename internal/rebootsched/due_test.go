package rebootsched

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	loc := time.FixedZone("test", 0)
	// Wednesday 2026-07-22
	wed := time.Date(2026, 7, 22, 3, 5, 0, 0, loc)
	wedBit := 1 << int(time.Wednesday) // 8
	thuBit := 1 << int(time.Thursday)  // 16
	catchup := 15 * time.Minute

	tests := []struct {
		name    string
		now     time.Time
		days    int
		hhmm    string
		lastRun *time.Time
		wantOK  bool
	}{
		{
			name:   "in window",
			now:    wed,
			days:   wedBit,
			hhmm:   "03:00",
			wantOK: true,
		},
		{
			name:   "past 15m",
			now:    time.Date(2026, 7, 22, 3, 16, 0, 0, loc),
			days:   wedBit,
			hhmm:   "03:00",
			wantOK: false,
		},
		{
			name: "last_run blocks",
			now:  wed,
			days: wedBit,
			hhmm: "03:00",
			lastRun: func() *time.Time {
				t := time.Date(2026, 7, 22, 3, 0, 0, 0, loc)
				return &t
			}(),
			wantOK: false,
		},
		{
			name:   "midnight wrap",
			now:    time.Date(2026, 7, 23, 0, 5, 0, 0, loc), // Thu 00:05
			days:   wedBit,                                   // Wed schedule
			hhmm:   "23:55",
			wantOK: true,
		},
		{
			name:   "disabled bits",
			now:    wed,
			days:   0,
			hhmm:   "03:00",
			wantOK: false,
		},
		{
			name:   "wrong weekday",
			now:    wed,
			days:   thuBit,
			hhmm:   "03:00",
			wantOK: false,
		},
		{
			name: "last_run before slot allows",
			now:  wed,
			days: wedBit,
			hhmm: "03:00",
			lastRun: func() *time.Time {
				t := time.Date(2026, 7, 21, 3, 0, 0, 0, loc)
				return &t
			}(),
			wantOK: true,
		},
		{
			name:   "exactly at slot",
			now:    time.Date(2026, 7, 22, 3, 0, 0, 0, loc),
			days:   wedBit,
			hhmm:   "03:00",
			wantOK: true,
		},
		{
			name:   "exactly at catchup end exclusive",
			now:    time.Date(2026, 7, 22, 3, 15, 0, 0, loc),
			days:   wedBit,
			hhmm:   "03:00",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot, ok := Due(tt.now, tt.days, tt.hhmm, tt.lastRun, catchup)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want %v (slot=%v)", ok, tt.wantOK, slot)
			}
			if ok && slot.IsZero() {
				t.Fatal("expected non-zero slot")
			}
		})
	}
}
