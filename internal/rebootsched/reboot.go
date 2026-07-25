package rebootsched

import (
	"strings"

	"mikrotik-monitor/internal/mikrotik"
	"mikrotik-monitor/internal/models"
)

// Reloader is the poller hook used after a reboot command.
type Reloader interface {
	ReloadDevice(id int64)
}

// RebootDevice issues /system/reboot. Connection drops (EOF/closed) count as success.
func RebootDevice(db *models.DB, p Reloader, id int64) error {
	dev, err := db.GetDevice(id)
	if err != nil {
		return err
	}
	pw, err := db.GetDevicePassword(id)
	if err != nil {
		return err
	}
	port := dev.Port
	if port == 0 {
		port = 8728
	}
	client := &mikrotik.Client{
		Host:     dev.Host,
		Port:     port,
		Username: dev.Username,
		Password: pw,
	}
	if err := client.Reboot(); err != nil {
		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "connection") &&
			!strings.Contains(errStr, "closed") &&
			!strings.Contains(errStr, "eof") &&
			!strings.Contains(errStr, "broken") {
			return err
		}
	}
	if p != nil {
		p.ReloadDevice(id)
	}
	return nil
}
