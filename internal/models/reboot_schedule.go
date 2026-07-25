package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var rebootTimeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// ValidateRebootSchedule requires ≥1 day bit and HH:MM when the schedule is enabled.
func ValidateRebootSchedule(in DeviceInput) error {
	if !in.RebootScheduleEnabled {
		return nil
	}
	if in.RebootDays&0b1111111 == 0 {
		return errors.New("select at least one reboot day")
	}
	t := strings.TrimSpace(in.RebootTime)
	if t == "" {
		t = "03:00"
	}
	if !rebootTimeRe.MatchString(t) {
		return fmt.Errorf("reboot_time must be HH:MM (00:00–23:59), got %q", in.RebootTime)
	}
	return nil
}
