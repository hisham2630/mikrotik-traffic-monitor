package models

import (
	"fmt"
	"strings"
	"time"

	"mikrotik-monitor/internal/crypto"
)

func (d *DB) ListDevices() ([]Device, error) {
	rows, err := d.Query(`SELECT id, name, host, port, username, password_encrypted, polling_interval_sec, enabled, created_at, updated_at FROM devices ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Device
	for rows.Next() {
		dev, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *dev)
	}
	return list, rows.Err()
}

func scanDevice(rows interface {
	Scan(dest ...any) error
}) (*Device, error) {
	var dev Device
	var enabled int
	var created, updated string
	err := rows.Scan(&dev.ID, &dev.Name, &dev.Host, &dev.Port, &dev.Username, &dev.PasswordEncrypted,
		&dev.PollingIntervalSec, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	dev.Enabled = enabled == 1
	dev.CreatedAt = parseTime(created)
	dev.UpdatedAt = parseTime(updated)
	return &dev, nil
}

func (d *DB) GetDevice(id int64) (*Device, error) {
	row := d.QueryRow(`SELECT id, name, host, port, username, password_encrypted, polling_interval_sec, enabled, created_at, updated_at FROM devices WHERE id = ?`, id)
	return scanDevice(row)
}

func (d *DB) GetDevicePassword(id int64) (string, error) {
	var enc string
	if err := d.QueryRow(`SELECT password_encrypted FROM devices WHERE id = ?`, id).Scan(&enc); err != nil {
		return "", err
	}
	plain, err := crypto.Decrypt(enc, d.secret)
	if err != nil && strings.Contains(err.Error(), "message authentication failed") {
		return "", fmt.Errorf("device password cannot be decrypted; edit the device and re-enter the MikroTik password")
	}
	return plain, err
}

func (d *DB) CreateDevice(in DeviceInput) (*Device, error) {
	if in.Port == 0 {
		in.Port = 8728
	}
	if in.PollingIntervalSec <= 0 {
		in.PollingIntervalSec = 5
	}
	enc, err := crypto.Encrypt(in.Password, d.secret)
	if err != nil {
		return nil, err
	}
	enabled := 0
	if in.Enabled {
		enabled = 1
	}
	res, err := d.Exec(`INSERT INTO devices (name, host, port, username, password_encrypted, polling_interval_sec, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Host, in.Port, in.Username, enc, in.PollingIntervalSec, enabled)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetDevice(id)
}

func (d *DB) UpdateDevice(id int64, in DeviceInput) (*Device, error) {
	existing, err := d.GetDevice(id)
	if err != nil {
		return nil, err
	}
	enc := existing.PasswordEncrypted
	if in.Password != "" {
		enc, err = crypto.Encrypt(in.Password, d.secret)
		if err != nil {
			return nil, err
		}
	}
	if in.Port == 0 {
		in.Port = 8728
	}
	if in.PollingIntervalSec <= 0 {
		in.PollingIntervalSec = 5
	}
	enabled := 0
	if in.Enabled {
		enabled = 1
	}
	_, err = d.Exec(`UPDATE devices SET name=?, host=?, port=?, username=?, password_encrypted=?, polling_interval_sec=?, enabled=?, updated_at=datetime('now') WHERE id=?`,
		in.Name, in.Host, in.Port, in.Username, enc, in.PollingIntervalSec, enabled, id)
	if err != nil {
		return nil, err
	}
	return d.GetDevice(id)
}

func (d *DB) DeleteDevice(id int64) error {
	_, err := d.Exec(`DELETE FROM devices WHERE id = ?`, id)
	return err
}

func (d *DB) CopyDevice(id int64, newHost, newName string) (*Device, error) {
	src, err := d.GetDevice(id)
	if err != nil {
		return nil, err
	}
	pw, err := d.GetDevicePassword(id)
	if err != nil {
		return nil, err
	}
	if newName == "" {
		newName = src.Name + " (copy)"
	}
	if newHost == "" {
		return nil, fmt.Errorf("new host is required")
	}
	dev, err := d.CreateDevice(DeviceInput{
		Name:               newName,
		Host:               newHost,
		Port:               src.Port,
		Username:           src.Username,
		Password:           pw,
		PollingIntervalSec: src.PollingIntervalSec,
		Enabled:            src.Enabled,
	})
	if err != nil {
		return nil, err
	}
	ifaces, _ := d.ListMonitoredInterfaces(id)
	for _, iface := range ifaces {
		_, _ = d.AddMonitoredInterface(dev.ID, iface.InterfaceName, iface.InterfaceType)
	}
	rules, _ := d.ListAlertRulesByDevice(id)
	for _, r := range rules {
		var ifaceID *int64
		if r.InterfaceID != nil {
			for _, ni := range ifaces {
				if ni.ID == *r.InterfaceID {
					newIfaces, _ := d.ListMonitoredInterfaces(dev.ID)
					for _, nif := range newIfaces {
						if nif.InterfaceName == ni.InterfaceName {
							idCopy := nif.ID
							ifaceID = &idCopy
							break
						}
					}
					break
				}
			}
		}
		_, _ = d.CreateAlertRule(AlertRuleInput{
			DeviceID:     dev.ID,
			InterfaceID:  ifaceID,
			Direction:    r.Direction,
			Condition:    r.Condition,
			ThresholdBps: r.ThresholdBps,
			DurationSec:  r.DurationSec,
			CooldownSec:  r.CooldownSec,
			Enabled:      r.Enabled,
		})
	}
	return dev, nil
}

func (d *DB) GetAppSettings() (*AppSettings, error) {
	var s AppSettings
	err := d.QueryRow(`SELECT id, retention_days, server_secret FROM app_settings WHERE id = 1`).
		Scan(&s.ID, &s.RetentionDays, &s.ServerSecret)
	return &s, err
}

func (d *DB) UpdateAppSettings(retentionDays int) error {
	_, err := d.Exec(`UPDATE app_settings SET retention_days = ? WHERE id = 1`, retentionDays)
	return err
}

func (d *DB) PruneSamples(before time.Time) error {
	_, err := d.Exec(`DELETE FROM traffic_samples WHERE sampled_at < ?`, before.Format("2006-01-02 15:04:05"))
	return err
}
