package models

import (
	"errors"
	"fmt"
	"strings"
)

const (
	NotifyChannelWhatsApp = "whatsapp"
	NotifyChannelTelegram = "telegram"
)

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ValidateTelegramBotToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("telegram bot token is required")
	}
	botID, secret, ok := strings.Cut(token, ":")
	if !ok || botID == "" || secret == "" {
		return errors.New("paste the full bot token from BotFather (format: 123456789:AAH...)")
	}
	for _, c := range botID {
		if c < '0' || c > '9' {
			return errors.New("bot token must start with the numeric bot id, then colon, then secret")
		}
	}
	if len(secret) < 20 {
		return errors.New("bot token looks incomplete; paste the entire string from BotFather")
	}
	return nil
}

func (d *DB) ValidateRuleNotifyChannels(in AlertRuleInput) error {
	if !in.NotifyWhatsApp && !in.NotifyTelegram {
		return errors.New("select at least one notification channel (WhatsApp and/or Telegram)")
	}

	cfg, err := d.GetNotificationConfig()
	if err != nil {
		return err
	}

	if in.NotifyWhatsApp {
		if !cfg.WhatsAppEnabled {
			return errors.New("WhatsApp notifications are disabled in Settings")
		}
		if strings.TrimSpace(cfg.APIURLTemplate) == "" {
			return errors.New("WhatsApp API URL is not configured in Settings")
		}
		if len(splitCSV(cfg.PhoneNumbers)) == 0 {
			return errors.New("WhatsApp phone numbers are not configured in Settings")
		}
	}
	if in.NotifyTelegram {
		if !cfg.TelegramEnabled {
			return errors.New("Telegram notifications are disabled in Settings")
		}
		if err := ValidateTelegramBotToken(cfg.TelegramBotToken); err != nil {
			return fmt.Errorf("telegram: %w", err)
		}
		if len(splitCSV(cfg.TelegramChatIDs)) == 0 {
			return errors.New("Telegram chat IDs are not configured in Settings")
		}
	}
	return nil
}

func RuleNotifyChannelsLabel(wa, tg bool) string {
	var parts []string
	if wa {
		parts = append(parts, NotifyChannelWhatsApp)
	}
	if tg {
		parts = append(parts, NotifyChannelTelegram)
	}
	return strings.Join(parts, ",")
}

func normalizeAlertRuleInput(in *AlertRuleInput) {
	if !in.NotifyWhatsApp && !in.NotifyTelegram {
		in.NotifyWhatsApp = true
	}
}
