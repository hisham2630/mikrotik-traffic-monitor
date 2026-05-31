package models

import "strings"

func (d *DB) ListMonitoredInterfaces(deviceID int64) ([]MonitoredInterface, error) {
	rows, err := d.Query(`SELECT id, device_id, interface_name, interface_type, enabled, created_at FROM monitored_interfaces WHERE device_id = ? ORDER BY interface_type, interface_name`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MonitoredInterface
	for rows.Next() {
		var m MonitoredInterface
		var enabled int
		var created string
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.InterfaceName, &m.InterfaceType, &enabled, &created); err != nil {
			return nil, err
		}
		m.Enabled = enabled == 1
		m.CreatedAt = parseTime(created)
		list = append(list, m)
	}
	return list, rows.Err()
}

func (d *DB) AddMonitoredInterface(deviceID int64, name, ifaceType string) (*MonitoredInterface, error) {
	if ifaceType == "" {
		ifaceType = classifyInterface(name)
	}
	res, err := d.Exec(`INSERT OR REPLACE INTO monitored_interfaces (device_id, interface_name, interface_type, enabled) VALUES (?, ?, ?, 1)`,
		deviceID, name, ifaceType)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		var existingID int64
		_ = d.QueryRow(`SELECT id FROM monitored_interfaces WHERE device_id = ? AND interface_name = ?`, deviceID, name).Scan(&existingID)
		id = existingID
	}
	return d.getMonitoredInterface(id)
}

func (d *DB) getMonitoredInterface(id int64) (*MonitoredInterface, error) {
	var m MonitoredInterface
	var enabled int
	var created string
	err := d.QueryRow(`SELECT id, device_id, interface_name, interface_type, enabled, created_at FROM monitored_interfaces WHERE id = ?`, id).
		Scan(&m.ID, &m.DeviceID, &m.InterfaceName, &m.InterfaceType, &enabled, &created)
	if err != nil {
		return nil, err
	}
	m.Enabled = enabled == 1
	m.CreatedAt = parseTime(created)
	return &m, nil
}

func (d *DB) SetMonitoredInterfaces(deviceID int64, names []string, types map[string]string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM monitored_interfaces WHERE device_id = ?`, deviceID); err != nil {
		return err
	}
	for _, name := range names {
		t := types[name]
		if t == "" {
			t = classifyInterface(name)
		}
		if _, err := tx.Exec(`INSERT INTO monitored_interfaces (device_id, interface_name, interface_type, enabled) VALUES (?, ?, ?, 1)`,
			deviceID, name, t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) DeleteMonitoredInterface(id int64) error {
	_, err := d.Exec(`DELETE FROM monitored_interfaces WHERE id = ?`, id)
	return err
}

func classifyInterface(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "ether"), strings.HasPrefix(lower, "sfp"):
		return "ethernet"
	case strings.HasPrefix(lower, "vlan"), strings.HasPrefix(lower, "vl-"):
		return "vlan"
	case strings.HasPrefix(lower, "bridge"):
		return "bridge"
	case strings.HasPrefix(lower, "bond"):
		return "bonding"
	case strings.HasPrefix(lower, "wlan"):
		return "wireless"
	case strings.HasPrefix(lower, "ppp"):
		return "ppp"
	default:
		return "other"
	}
}
