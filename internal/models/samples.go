package models

import "time"

func (d *DB) InsertTrafficSample(s TrafficSample) error {
	_, err := d.Exec(`INSERT INTO traffic_samples (device_id, interface_name, tx_bps, rx_bps, sampled_at) VALUES (?, ?, ?, ?, ?)`,
		s.DeviceID, s.InterfaceName, s.TxBps, s.RxBps, s.SampledAt.Format("2006-01-02 15:04:05"))
	return err
}

func (d *DB) InsertTrafficSamplesBatch(samples []TrafficSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO traffic_samples (device_id, interface_name, tx_bps, rx_bps, sampled_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, s := range samples {
		if _, err := stmt.Exec(s.DeviceID, s.InterfaceName, s.TxBps, s.RxBps, s.SampledAt.Format("2006-01-02 15:04:05")); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) GetTrafficHistory(deviceID int64, iface string, since time.Time) ([]TrafficSample, error) {
	rows, err := d.Query(`SELECT device_id, interface_name, tx_bps, rx_bps, sampled_at FROM traffic_samples
		WHERE device_id = ? AND interface_name = ? AND sampled_at >= ? ORDER BY sampled_at ASC`,
		deviceID, iface, since.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TrafficSample
	for rows.Next() {
		var s TrafficSample
		var sampled string
		if err := rows.Scan(&s.DeviceID, &s.InterfaceName, &s.TxBps, &s.RxBps, &sampled); err != nil {
			return nil, err
		}
		s.SampledAt = parseTime(sampled)
		list = append(list, s)
	}
	return list, rows.Err()
}
