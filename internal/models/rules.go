package models

import "database/sql"

func (d *DB) ListAlertRules() ([]AlertRule, error) {
	rows, err := d.Query(`SELECT id, device_id, interface_id, direction, condition, threshold_bps, duration_sec, cooldown_sec, enabled, created_at, updated_at FROM alert_rules ORDER BY device_id, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlertRules(rows)
}

func (d *DB) ListAlertRulesByDevice(deviceID int64) ([]AlertRule, error) {
	rows, err := d.Query(`SELECT id, device_id, interface_id, direction, condition, threshold_bps, duration_sec, cooldown_sec, enabled, created_at, updated_at FROM alert_rules WHERE device_id = ?`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlertRules(rows)
}

func (d *DB) ListEnabledAlertRules() ([]AlertRule, error) {
	rows, err := d.Query(`SELECT id, device_id, interface_id, direction, condition, threshold_bps, duration_sec, cooldown_sec, enabled, created_at, updated_at FROM alert_rules WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlertRules(rows)
}

func scanAlertRules(rows *sql.Rows) ([]AlertRule, error) {
	var list []AlertRule
	for rows.Next() {
		var r AlertRule
		var ifaceID sql.NullInt64
		var enabled int
		var created, updated string
		if err := rows.Scan(&r.ID, &r.DeviceID, &ifaceID, &r.Direction, &r.Condition, &r.ThresholdBps,
			&r.DurationSec, &r.CooldownSec, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		if ifaceID.Valid {
			r.InterfaceID = &ifaceID.Int64
		}
		r.Enabled = enabled == 1
		r.CreatedAt = parseTime(created)
		r.UpdatedAt = parseTime(updated)
		list = append(list, r)
	}
	return list, rows.Err()
}

func (d *DB) GetAlertRule(id int64) (*AlertRule, error) {
	row := d.QueryRow(`SELECT id, device_id, interface_id, direction, condition, threshold_bps, duration_sec, cooldown_sec, enabled, created_at, updated_at FROM alert_rules WHERE id = ?`, id)
	var r AlertRule
	var ifaceID sql.NullInt64
	var enabled int
	var created, updated string
	err := row.Scan(&r.ID, &r.DeviceID, &ifaceID, &r.Direction, &r.Condition, &r.ThresholdBps,
		&r.DurationSec, &r.CooldownSec, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	if ifaceID.Valid {
		r.InterfaceID = &ifaceID.Int64
	}
	r.Enabled = enabled == 1
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return &r, nil
}

func (d *DB) CreateAlertRule(in AlertRuleInput) (*AlertRule, error) {
	enabled := 0
	if in.Enabled {
		enabled = 1
	}
	if in.DurationSec <= 0 {
		in.DurationSec = 30
	}
	if in.CooldownSec <= 0 {
		in.CooldownSec = 300
	}
	res, err := d.Exec(`INSERT INTO alert_rules (device_id, interface_id, direction, condition, threshold_bps, duration_sec, cooldown_sec, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.DeviceID, in.InterfaceID, in.Direction, in.Condition, in.ThresholdBps, in.DurationSec, in.CooldownSec, enabled)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetAlertRule(id)
}

func (d *DB) UpdateAlertRule(id int64, in AlertRuleInput) (*AlertRule, error) {
	enabled := 0
	if in.Enabled {
		enabled = 1
	}
	_, err := d.Exec(`UPDATE alert_rules SET device_id=?, interface_id=?, direction=?, condition=?, threshold_bps=?, duration_sec=?, cooldown_sec=?, enabled=?, updated_at=datetime('now') WHERE id=?`,
		in.DeviceID, in.InterfaceID, in.Direction, in.Condition, in.ThresholdBps, in.DurationSec, in.CooldownSec, enabled, id)
	if err != nil {
		return nil, err
	}
	return d.GetAlertRule(id)
}

func (d *DB) DeleteAlertRule(id int64) error {
	_, err := d.Exec(`DELETE FROM alert_rules WHERE id = ?`, id)
	return err
}

func (d *DB) GetNotificationConfig() (*NotificationConfig, error) {
	var c NotificationConfig
	var enabled int
	err := d.QueryRow(`SELECT id, api_url_template, phone_numbers, message_template, enabled FROM notification_config WHERE id = 1`).
		Scan(&c.ID, &c.APIURLTemplate, &c.PhoneNumbers, &c.MessageTemplate, &enabled)
	if err != nil {
		return nil, err
	}
	c.Enabled = enabled == 1
	return &c, nil
}

func (d *DB) UpdateNotificationConfig(c NotificationConfig) error {
	enabled := 0
	if c.Enabled {
		enabled = 1
	}
	_, err := d.Exec(`UPDATE notification_config SET api_url_template=?, phone_numbers=?, message_template=?, enabled=? WHERE id=1`,
		c.APIURLTemplate, c.PhoneNumbers, c.MessageTemplate, enabled)
	return err
}

func (d *DB) InsertAlertHistory(ruleID *int64, deviceID int64, iface, direction string, value int64, msg string, notified bool) error {
	n := 0
	if notified {
		n = 1
	}
	_, err := d.Exec(`INSERT INTO alert_history (rule_id, device_id, interface_name, triggered_value_bps, direction, message, notified) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ruleID, deviceID, iface, value, direction, msg, n)
	return err
}

func (d *DB) ListAlertHistory(limit int) ([]AlertHistory, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.Query(`SELECT id, rule_id, device_id, interface_name, triggered_value_bps, direction, message, fired_at, notified FROM alert_history ORDER BY fired_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AlertHistory
	for rows.Next() {
		var h AlertHistory
		var ruleID sql.NullInt64
		var notified int
		var fired string
		if err := rows.Scan(&h.ID, &ruleID, &h.DeviceID, &h.InterfaceName, &h.TriggeredValueBps, &h.Direction, &h.Message, &fired, &notified); err != nil {
			return nil, err
		}
		if ruleID.Valid {
			h.RuleID = &ruleID.Int64
		}
		h.FiredAt = parseTime(fired)
		h.Notified = notified == 1
		list = append(list, h)
	}
	return list, rows.Err()
}
