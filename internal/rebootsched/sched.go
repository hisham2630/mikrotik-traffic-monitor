package rebootsched

import (
	"fmt"
	"log"
	"time"

	"mikrotik-monitor/internal/models"
)

// Notifier sends a free-form message to enabled notification channels.
type Notifier interface {
	Notify(msg string)
}

// Start launches a minute ticker that reboots due devices. Non-blocking.
// ponytail: single goroutine + sequential reboots; fan-out if device count grows large.
func Start(db *models.DB, p Reloader, n Notifier) {
	go run(db, p, n)
}

func run(db *models.DB, p Reloader, n Notifier) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		tick(db, p, n, time.Now())
	}
}

func tick(db *models.DB, p Reloader, n Notifier, now time.Time) {
	devs, err := db.ListDevices()
	if err != nil {
		log.Printf("rebootsched: list devices: %v", err)
		return
	}
	for _, dev := range devs {
		if !dev.Enabled || !dev.RebootScheduleEnabled {
			continue
		}
		if _, ok := Due(now, dev.RebootDays, dev.RebootTime, dev.RebootLastRunAt, DefaultCatchup); !ok {
			continue
		}
		if err := db.MarkRebootLastRun(dev.ID, now); err != nil {
			log.Printf("rebootsched: mark last_run device %d: %v", dev.ID, err)
			continue
		}
		err := RebootDevice(db, p, dev.ID)
		msg := fmt.Sprintf("Scheduled reboot of %s (%s): success", dev.Name, dev.Host)
		if err != nil {
			msg = fmt.Sprintf("Scheduled reboot of %s (%s): failed — %v", dev.Name, dev.Host, err)
			log.Printf("rebootsched: %s", msg)
		} else {
			log.Printf("rebootsched: %s", msg)
		}
		if n != nil {
			n.Notify(msg)
		}
	}
}
