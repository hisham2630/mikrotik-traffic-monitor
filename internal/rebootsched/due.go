package rebootsched

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultCatchup is the post-slot window during which a missed reboot still fires.
const DefaultCatchup = 15 * time.Minute

// Due reports whether a reboot slot is active at now.
// days is a bitmask Sun=1, Mon=2, … Sat=64. lastRun nil means never fired.
// Also checks yesterday’s slot so a midnight wrap (e.g. 23:55 → 00:05) still catches up.
func Due(now time.Time, days int, timeHHMM string, lastRun *time.Time, catchup time.Duration) (slot time.Time, ok bool) {
	if days == 0 || catchup <= 0 {
		return time.Time{}, false
	}
	hour, min, err := parseHHMM(timeHHMM)
	if err != nil {
		return time.Time{}, false
	}
	for _, dayOffset := range []int{0, -1} {
		day := now.AddDate(0, 0, dayOffset)
		bit := 1 << uint(day.Weekday()) // Sunday=0 → 1
		if days&bit == 0 {
			continue
		}
		cand := time.Date(day.Year(), day.Month(), day.Day(), hour, min, 0, 0, now.Location())
		if now.Before(cand) || !now.Before(cand.Add(catchup)) {
			continue
		}
		if lastRun != nil && !lastRun.Before(cand) {
			continue
		}
		return cand, true
	}
	return time.Time{}, false
}

func parseHHMM(s string) (hour, min int, err error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time")
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	min, err = strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return hour, min, nil
}
