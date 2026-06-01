package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mikrotik-monitor/internal/models"
)

type Engine struct {
	db       *models.DB
	samples  <-chan models.TrafficSample
	devNames map[int64]string
	rules    []models.AlertRule
	ifaces   map[int64]map[int64]string // device -> interface_id -> name
	mu       sync.RWMutex
	state    map[int64]*ruleState
}

type ruleState struct {
	conditionSince *time.Time
	cooldownUntil  time.Time
}

func New(db *models.DB, samples <-chan models.TrafficSample) *Engine {
	e := &Engine{
		db:       db,
		samples:  samples,
		devNames: make(map[int64]string),
		ifaces:   make(map[int64]map[int64]string),
		state:    make(map[int64]*ruleState),
	}
	e.reload()
	return e
}

func (e *Engine) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case s, ok := <-e.samples:
			if !ok {
				return
			}
			e.evaluate(s)
		case <-ticker.C:
			e.reload()
		}
	}
}

func (e *Engine) reload() {
	devs, err := e.db.ListDevices()
	if err != nil {
		return
	}
	rules, err := e.db.ListEnabledAlertRules()
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.devNames = make(map[int64]string)
	for _, d := range devs {
		e.devNames[d.ID] = d.Name
	}
	e.rules = rules
	e.ifaces = make(map[int64]map[int64]string)
	for _, d := range devs {
		ifaces, _ := e.db.ListMonitoredInterfaces(d.ID)
		m := make(map[int64]string)
		for _, i := range ifaces {
			m[i.ID] = i.InterfaceName
		}
		e.ifaces[d.ID] = m
	}
}

func (e *Engine) evaluate(s models.TrafficSample) {
	e.mu.RLock()
	rules := e.rules
	devName := e.devNames[s.DeviceID]
	e.mu.RUnlock()

	for _, rule := range rules {
		if rule.DeviceID != s.DeviceID {
			continue
		}
		if rule.InterfaceID != nil {
			e.mu.RLock()
			name, ok := e.ifaces[s.DeviceID][*rule.InterfaceID]
			e.mu.RUnlock()
			if !ok || name != s.InterfaceName {
				continue
			}
		}
		tx, rx := s.TxBps, s.RxBps
		met := false
		var triggered int64
		var dir string

		check := func(val int64, direction string) bool {
			if rule.Condition == "above" {
				return val > rule.ThresholdBps
			}
			return val < rule.ThresholdBps
		}

		switch rule.Direction {
		case "tx":
			met = check(tx, "tx")
			triggered = tx
			dir = "tx"
		case "rx":
			met = check(rx, "rx")
			triggered = rx
			dir = "rx"
		case "both":
			metTx := check(tx, "tx")
			metRx := check(rx, "rx")
			met = metTx || metRx
			if metTx {
				triggered = tx
				dir = "tx"
			} else if metRx {
				triggered = rx
				dir = "rx"
			}
		}

		e.mu.Lock()
		st, ok := e.state[rule.ID]
		if !ok {
			st = &ruleState{}
			e.state[rule.ID] = st
		}
		now := time.Now()
		if met {
			if st.conditionSince == nil {
				t := now
				st.conditionSince = &t
			} else if now.Sub(*st.conditionSince) >= time.Duration(rule.DurationSec)*time.Second {
				if now.After(st.cooldownUntil) {
					msg := formatAlert(devName, s.InterfaceName, dir, triggered, rule.ThresholdBps, rule.DurationSec)
					e.fire(rule, s, triggered, dir, msg)
					st.cooldownUntil = now.Add(time.Duration(rule.CooldownSec) * time.Second)
					st.conditionSince = nil
				}
			}
		} else {
			st.conditionSince = nil
		}
		e.mu.Unlock()
	}
}

func formatAlert(dev, iface, dir string, val, threshold int64, dur int) string {
	return fmt.Sprintf("⚠ [%s] %s %s at %s (threshold: %s for %ds)",
		dev, iface, strings.ToUpper(dir), formatBps(val), formatBps(threshold), dur)
}

func formatBps(bps int64) string {
	if bps >= 1_000_000_000 {
		return fmt.Sprintf("%.1fGbps", float64(bps)/1e9)
	}
	if bps >= 1_000_000 {
		return fmt.Sprintf("%.1fMbps", float64(bps)/1e6)
	}
	if bps >= 1_000 {
		return fmt.Sprintf("%.1fKbps", float64(bps)/1e3)
	}
	return fmt.Sprintf("%dbps", bps)
}

func applyMessageTemplate(template, msg string) string {
	if template == "" {
		template = "{message}"
	}
	return strings.ReplaceAll(template, "{message}", msg)
}

func (e *Engine) fire(rule models.AlertRule, s models.TrafficSample, val int64, dir, msg string) {
	ruleID := rule.ID
	channels := models.RuleNotifyChannelsLabel(rule.NotifyWhatsApp, rule.NotifyTelegram)
	_ = e.db.InsertAlertHistory(&ruleID, s.DeviceID, s.InterfaceName, dir, val, msg, channels, false)

	cfg, err := e.db.GetNotificationConfig()
	if err != nil {
		return
	}
	text := applyMessageTemplate(cfg.MessageTemplate, msg)
	client := &http.Client{Timeout: 10 * time.Second}

	var notified bool
	if rule.NotifyWhatsApp {
		if sendWhatsApp(client, cfg, text) {
			notified = true
		}
	}
	if rule.NotifyTelegram {
		if sendTelegram(client, cfg, text) {
			notified = true
		}
	}
	if notified {
		_ = e.db.InsertAlertHistory(&ruleID, s.DeviceID, s.InterfaceName, dir, val, msg, channels, true)
	}
}

func sendWhatsApp(client *http.Client, cfg *models.NotificationConfig, text string) bool {
	if !cfg.WhatsAppEnabled || cfg.APIURLTemplate == "" {
		return false
	}
	phones := splitList(cfg.PhoneNumbers)
	if len(phones) == 0 {
		return false
	}
	notified := false
	for _, phone := range phones {
		u := strings.ReplaceAll(cfg.APIURLTemplate, "{phone}", url.QueryEscape(phone))
		u = strings.ReplaceAll(u, "{message}", url.QueryEscape(text))
		if err := sendGET(client, u); err != nil {
			log.Printf("alert whatsapp notify failed: %v", err)
			time.Sleep(5 * time.Second)
			if err2 := sendGET(client, u); err2 == nil {
				notified = true
			}
		} else {
			notified = true
		}
	}
	return notified
}

// normalizeTelegramChatID fixes common supergroup IDs copied without a leading minus
// (e.g. 1002405693501 → -1002405693501). Private chats keep their positive numeric id.
func normalizeTelegramChatID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.HasPrefix(id, "-") {
		return id
	}
	if strings.HasPrefix(id, "100") && len(id) > 10 {
		return "-" + id
	}
	return id
}

type telegramAPIResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func telegramSendMessage(client *http.Client, token, chatID, text string) error {
	chatID = normalizeTelegramChatID(chatID)
	if len(text) > 4096 {
		text = text[:4096]
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	body, err := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tr telegramAPIResponse
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &tr)
	}
	if tr.OK {
		return nil
	}
	if tr.Description != "" {
		return fmt.Errorf("telegram: %s", tr.Description)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram: HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("telegram: request failed")
}

func sendTelegram(client *http.Client, cfg *models.NotificationConfig, text string) bool {
	if !cfg.TelegramEnabled || strings.TrimSpace(cfg.TelegramBotToken) == "" {
		return false
	}
	chatIDs := splitList(cfg.TelegramChatIDs)
	if len(chatIDs) == 0 {
		return false
	}
	notified := false
	for _, chatID := range chatIDs {
		if err := telegramSendMessage(client, cfg.TelegramBotToken, chatID, text); err != nil {
			log.Printf("alert telegram notify failed (chat %s): %v", chatID, err)
			time.Sleep(5 * time.Second)
			if err2 := telegramSendMessage(client, cfg.TelegramBotToken, chatID, text); err2 == nil {
				notified = true
			}
		} else {
			notified = true
		}
	}
	return notified
}

func sendTelegramTest(client *http.Client, cfg *models.NotificationConfig, text string) error {
	chatIDs := splitList(cfg.TelegramChatIDs)
	var lastErr error
	sent := false
	for _, chatID := range chatIDs {
		if err := telegramSendMessage(client, cfg.TelegramBotToken, chatID, text); err != nil {
			lastErr = fmt.Errorf("chat %s: %w", chatID, err)
			log.Printf("telegram test failed (chat %s): %v", chatID, err)
			continue
		}
		sent = true
	}
	if sent {
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("telegram test send failed")
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func sendGET(client *http.Client, u string) error {
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func sendPOST(client *http.Client, u string, body []byte) error {
	resp, err := client.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (e *Engine) SendTest(channel string) error {
	cfg, err := e.db.GetNotificationConfig()
	if err != nil {
		return err
	}
	msg := "MikroTik Monitor test notification"
	text := applyMessageTemplate(cfg.MessageTemplate, msg)
	client := &http.Client{Timeout: 10 * time.Second}

	switch strings.ToLower(strings.TrimSpace(channel)) {
	case models.NotifyChannelTelegram:
		if !cfg.TelegramEnabled {
			return fmt.Errorf("telegram notifications are disabled")
		}
		if strings.TrimSpace(cfg.TelegramBotToken) == "" {
			return fmt.Errorf("telegram bot token not configured")
		}
		if err := models.ValidateTelegramBotToken(cfg.TelegramBotToken); err != nil {
			return err
		}
		if len(splitList(cfg.TelegramChatIDs)) == 0 {
			return fmt.Errorf("telegram chat IDs not configured")
		}
		return sendTelegramTest(client, cfg, text)
	case models.NotifyChannelWhatsApp:
		if !cfg.WhatsAppEnabled {
			return fmt.Errorf("whatsapp notifications are disabled")
		}
		if cfg.APIURLTemplate == "" {
			return fmt.Errorf("whatsapp API URL not configured")
		}
		if len(splitList(cfg.PhoneNumbers)) == 0 {
			return fmt.Errorf("whatsapp phone numbers not configured")
		}
		if !sendWhatsApp(client, cfg, text) {
			return fmt.Errorf("whatsapp test send failed")
		}
		return nil
	default:
		return fmt.Errorf("unknown notification channel: %s", channel)
	}
}
