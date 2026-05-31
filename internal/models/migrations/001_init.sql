PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    must_change_password INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 8728,
    username TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    polling_interval_sec INTEGER NOT NULL DEFAULT 5,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS monitored_interfaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    interface_name TEXT NOT NULL,
    interface_type TEXT NOT NULL DEFAULT 'other',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(device_id, interface_name)
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    interface_id INTEGER REFERENCES monitored_interfaces(id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('tx', 'rx', 'both')),
    condition TEXT NOT NULL CHECK (condition IN ('above', 'below')),
    threshold_bps INTEGER NOT NULL,
    duration_sec INTEGER NOT NULL DEFAULT 30,
    cooldown_sec INTEGER NOT NULL DEFAULT 300,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notification_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    api_url_template TEXT NOT NULL DEFAULT '',
    phone_numbers TEXT NOT NULL DEFAULT '',
    message_template TEXT NOT NULL DEFAULT '{message}',
    enabled INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS app_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    retention_days INTEGER NOT NULL DEFAULT 7,
    server_secret TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS traffic_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    interface_name TEXT NOT NULL,
    tx_bps INTEGER NOT NULL,
    rx_bps INTEGER NOT NULL,
    sampled_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_traffic_samples_lookup
    ON traffic_samples(device_id, interface_name, sampled_at);

CREATE TABLE IF NOT EXISTS alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER REFERENCES alert_rules(id) ON DELETE SET NULL,
    device_id INTEGER NOT NULL,
    interface_name TEXT NOT NULL,
    triggered_value_bps INTEGER NOT NULL,
    direction TEXT NOT NULL,
    message TEXT NOT NULL,
    fired_at TEXT NOT NULL DEFAULT (datetime('now')),
    notified INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_alert_history_fired ON alert_history(fired_at);
