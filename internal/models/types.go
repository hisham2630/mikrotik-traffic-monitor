package models

import "time"

type User struct {
	ID                 int64     `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	Role               string    `json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
}

type Device struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	Username           string    `json:"username"`
	PasswordEncrypted  string    `json:"-"`
	PollingIntervalSec int       `json:"polling_interval_sec"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	// Runtime only (not stored in DB)
	Online    bool   `json:"online,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

type DeviceInput struct {
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	PollingIntervalSec int    `json:"polling_interval_sec"`
	Enabled            bool   `json:"enabled"`
}

type MonitoredInterface struct {
	ID            int64     `json:"id"`
	DeviceID      int64     `json:"device_id"`
	InterfaceName string    `json:"interface_name"`
	InterfaceType string    `json:"interface_type"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
}

type DiscoveredInterface struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type AlertRule struct {
	ID           int64     `json:"id"`
	DeviceID     int64     `json:"device_id"`
	InterfaceID  *int64    `json:"interface_id"`
	Direction    string    `json:"direction"`
	Condition    string    `json:"condition"`
	ThresholdBps int64     `json:"threshold_bps"`
	DurationSec  int       `json:"duration_sec"`
	CooldownSec    int       `json:"cooldown_sec"`
	NotifyWhatsApp bool      `json:"notify_whatsapp"`
	NotifyTelegram bool      `json:"notify_telegram"`
	Enabled        bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AlertRuleInput struct {
	DeviceID      int64  `json:"device_id"`
	InterfaceID   *int64 `json:"interface_id"`
	Direction     string `json:"direction"`
	Condition     string `json:"condition"`
	ThresholdBps  int64  `json:"threshold_bps"`
	DurationSec   int    `json:"duration_sec"`
	CooldownSec    int  `json:"cooldown_sec"`
	NotifyWhatsApp bool `json:"notify_whatsapp"`
	NotifyTelegram bool `json:"notify_telegram"`
	Enabled        bool `json:"enabled"`
}

type NotificationConfig struct {
	ID               int    `json:"id"`
	APIURLTemplate   string `json:"api_url_template"`
	PhoneNumbers     string `json:"phone_numbers"`
	MessageTemplate  string `json:"message_template"`
	WhatsAppEnabled  bool   `json:"whatsapp_enabled"`
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatIDs  string `json:"telegram_chat_ids"`
	TelegramEnabled  bool   `json:"telegram_enabled"`
}

type AppSettings struct {
	ID             int    `json:"id"`
	RetentionDays  int    `json:"retention_days"`
	ServerSecret   string `json:"-"`
}

type TrafficSample struct {
	DeviceID      int64     `json:"device_id"`
	InterfaceName string    `json:"interface"`
	TxBps         int64     `json:"tx_bps"`
	RxBps         int64     `json:"rx_bps"`
	SampledAt     time.Time `json:"ts"`
}

type AlertHistory struct {
	ID                int64     `json:"id"`
	RuleID            *int64    `json:"rule_id"`
	DeviceID          int64     `json:"device_id"`
	InterfaceName     string    `json:"interface_name"`
	TriggeredValueBps int64     `json:"triggered_value_bps"`
	Direction         string    `json:"direction"`
	Message           string    `json:"message"`
	NotifyChannel     string    `json:"notify_channel"`
	FiredAt           time.Time `json:"fired_at"`
	Notified          bool      `json:"notified"`
}

type DeviceStatus struct {
	DeviceID int64  `json:"device_id"`
	Online   bool   `json:"online"`
	Error    string `json:"error,omitempty"`
}

type DeviceStats struct {
	DeviceID int64  `json:"device_id"`
	CPULoad  int    `json:"cpu_load"`
	Uptime   string `json:"uptime"`
}
