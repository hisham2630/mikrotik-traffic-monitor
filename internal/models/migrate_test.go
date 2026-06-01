package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationsNotificationChannels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg, err := db.GetNotificationConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MessageTemplate != "{message}" {
		t.Fatalf("message template: %q", cfg.MessageTemplate)
	}

	in := AlertRuleInput{
		DeviceID:       1,
		Direction:      "rx",
		Condition:      "above",
		ThresholdBps:   1000,
		NotifyWhatsApp: true,
		Enabled:        true,
	}
	err = db.ValidateRuleNotifyChannels(in)
	if err == nil {
		t.Fatal("expected validation error when whatsapp not configured")
	}

	_, err = db.Exec(`INSERT INTO devices (name, host, port, username, password_encrypted, polling_interval_sec, enabled) VALUES ('d','h',8728,'u','',30,1)`)
	if err != nil {
		t.Fatal(err)
	}

	cfg.WhatsAppEnabled = true
	cfg.APIURLTemplate = "http://example.com/?phone={phone}&text={message}"
	cfg.PhoneNumbers = "123"
	if err := db.UpdateNotificationConfig(*cfg); err != nil {
		t.Fatal(err)
	}
	if err := db.ValidateRuleNotifyChannels(in); err != nil {
		t.Fatal(err)
	}

	rule, err := db.CreateAlertRule(in)
	if err != nil {
		t.Fatal(err)
	}
	if !rule.NotifyWhatsApp || rule.NotifyTelegram {
		t.Fatalf("channels: wa=%v tg=%v", rule.NotifyWhatsApp, rule.NotifyTelegram)
	}

	label := RuleNotifyChannelsLabel(rule.NotifyWhatsApp, rule.NotifyTelegram)
	if err := db.InsertAlertHistory(&rule.ID, 1, "eth1", "rx", 5000, "test", label, true); err != nil {
		t.Fatal(err)
	}
	hist, err := db.ListAlertHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].NotifyChannel != NotifyChannelWhatsApp {
		t.Fatalf("history: %+v", hist)
	}

	inBoth := AlertRuleInput{
		DeviceID:       1,
		Direction:      "rx",
		Condition:      "above",
		ThresholdBps:   2000,
		NotifyWhatsApp: true,
		NotifyTelegram: true,
		Enabled:        true,
	}
	cfg.TelegramEnabled = true
	cfg.TelegramBotToken = "123456789:AAHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	cfg.TelegramChatIDs = "-100123"
	if err := db.UpdateNotificationConfig(*cfg); err != nil {
		t.Fatal(err)
	}
	if err := db.ValidateRuleNotifyChannels(inBoth); err != nil {
		t.Fatal(err)
	}
	rule2, err := db.CreateAlertRule(inBoth)
	if err != nil {
		t.Fatal(err)
	}
	if !rule2.NotifyWhatsApp || !rule2.NotifyTelegram {
		t.Fatalf("both channels: wa=%v tg=%v", rule2.NotifyWhatsApp, rule2.NotifyTelegram)
	}
	if got := RuleNotifyChannelsLabel(rule2.NotifyWhatsApp, rule2.NotifyTelegram); got != "whatsapp,telegram" {
		t.Fatalf("label: %q", got)
	}

	_ = os.Remove(path)
}
