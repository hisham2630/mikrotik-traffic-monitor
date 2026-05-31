package alerter

import (
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

func (e *Engine) fire(rule models.AlertRule, s models.TrafficSample, val int64, dir, msg string) {
	ruleID := rule.ID
	_ = e.db.InsertAlertHistory(&ruleID, s.DeviceID, s.InterfaceName, dir, val, msg, false)

	cfg, err := e.db.GetNotificationConfig()
	if err != nil || !cfg.Enabled || cfg.APIURLTemplate == "" {
		return
	}
	phones := strings.Split(cfg.PhoneNumbers, ",")
	text := strings.ReplaceAll(cfg.MessageTemplate, "{message}", msg)
	client := &http.Client{Timeout: 10 * time.Second}
	notified := false
	for _, phone := range phones {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}
		u := strings.ReplaceAll(cfg.APIURLTemplate, "{phone}", url.QueryEscape(phone))
		u = strings.ReplaceAll(u, "{message}", url.QueryEscape(text))
		if err := sendGET(client, u); err != nil {
			log.Printf("alert notify failed: %v", err)
			time.Sleep(5 * time.Second)
			if err2 := sendGET(client, u); err2 == nil {
				notified = true
			}
		} else {
			notified = true
		}
	}
	if notified {
		_ = e.db.InsertAlertHistory(&ruleID, s.DeviceID, s.InterfaceName, dir, val, msg, true)
	}
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

func (e *Engine) SendTestNotification(message string) error {
	cfg, err := e.db.GetNotificationConfig()
	if err != nil {
		return err
	}
	if cfg.APIURLTemplate == "" {
		return fmt.Errorf("notification URL not configured")
	}
	phones := strings.Split(cfg.PhoneNumbers, ",")
	text := strings.ReplaceAll(cfg.MessageTemplate, "{message}", message)
	client := &http.Client{Timeout: 10 * time.Second}
	for _, phone := range phones {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}
		u := strings.ReplaceAll(cfg.APIURLTemplate, "{phone}", url.QueryEscape(phone))
		u = strings.ReplaceAll(u, "{message}", url.QueryEscape(text))
		if err := sendGET(client, u); err != nil {
			return err
		}
	}
	return nil
}
