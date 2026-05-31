package poller

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mikrotik-monitor/internal/mikrotik"
	"mikrotik-monitor/internal/models"
	"mikrotik-monitor/internal/wshub"
)

type counterSnap struct {
	rxBytes uint64
	txBytes uint64
	at      time.Time
}

type Manager struct {
	db       *models.DB
	hub      *wshub.Hub
	out      chan<- models.TrafficSample
	mu       sync.RWMutex
	workers  map[int64]chan struct{}
	status   map[int64]bool
	lastErr  map[int64]string
	prev     map[int64]map[string]counterSnap
	latest   map[string]models.TrafficSample // key: "deviceID:interface"
}

func New(db *models.DB, hub *wshub.Hub, out chan<- models.TrafficSample) *Manager {
	return &Manager{
		db:      db,
		hub:     hub,
		out:     out,
		workers: make(map[int64]chan struct{}),
		status:  make(map[int64]bool),
		lastErr: make(map[int64]string),
		prev:    make(map[int64]map[string]counterSnap),
		latest:  make(map[string]models.TrafficSample),
	}
}

func (m *Manager) Start() {
	// Let the hub call LatestSamples when a new WS client subscribes,
	// so charts populate immediately without waiting for the next poll tick.
	m.hub.GetSnapshot = func(deviceIDs []int64) []models.TrafficSample {
		var out []models.TrafficSample
		for _, id := range deviceIDs {
			out = append(out, m.LatestSamples(id)...)
		}
		return out
	}
	go m.syncLoop()
}

func (m *Manager) syncLoop() {
	m.sync()
	ticker := time.NewTicker(15 * time.Second)
	for range ticker.C {
		m.sync()
	}
}

func (m *Manager) sync() {
	devs, err := m.db.ListDevices()
	if err != nil {
		return
	}
	active := make(map[int64]struct{})
	for _, d := range devs {
		if !d.Enabled {
			continue
		}
		active[d.ID] = struct{}{}
		m.mu.Lock()
		if _, ok := m.workers[d.ID]; !ok {
			stop := make(chan struct{})
			m.workers[d.ID] = stop
			go m.runDevice(d.ID, stop)
		}
		m.mu.Unlock()
	}
	m.mu.Lock()
	for id, stop := range m.workers {
		if _, ok := active[id]; !ok {
			close(stop)
			delete(m.workers, id)
			delete(m.status, id)
			delete(m.lastErr, id)
			delete(m.prev, id)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) ReloadDevice(id int64) {
	m.mu.Lock()
	if stop, ok := m.workers[id]; ok {
		close(stop)
		delete(m.workers, id)
	}
	delete(m.prev, id)
	m.mu.Unlock()
	m.sync()
}

func (m *Manager) IsOnline(id int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status[id]
}

func (m *Manager) LastError(id int64) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr[id]
}

func (m *Manager) LatestSamples(deviceID int64) []models.TrafficSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	prefix := fmt.Sprintf("%d:", deviceID)
	var out []models.TrafficSample
	for k, s := range m.latest {
		if strings.HasPrefix(k, prefix) {
			out = append(out, s)
		}
	}
	if out == nil {
		out = []models.TrafficSample{}
	}
	return out
}

func fmtDeviceKey(deviceID int64, iface string) string {
	if iface == "" {
		return fmt.Sprintf("%d:", deviceID)
	}
	return fmt.Sprintf("%d:%s", deviceID, iface)
}

func (m *Manager) setStatus(deviceID int64, online bool, errMsg string) {
	m.mu.Lock()
	prev := m.status[deviceID]
	m.status[deviceID] = online
	if errMsg != "" {
		m.lastErr[deviceID] = errMsg
	} else if online {
		m.lastErr[deviceID] = ""
	}
	m.mu.Unlock()
	if prev != online || errMsg != "" {
		m.hub.BroadcastStatus(deviceID, online, errMsg)
	}
}

func (m *Manager) storeSample(s models.TrafficSample) {
	key := fmtDeviceKey(s.DeviceID, s.InterfaceName)
	m.mu.Lock()
	m.latest[key] = s
	m.mu.Unlock()
	m.hub.Broadcast(s)
	select {
	case m.out <- s:
	default:
	}
}

func (m *Manager) runDevice(deviceID int64, stop <-chan struct{}) {
	backoff := time.Second
	for {
		select {
		case <-stop:
			return
		default:
		}

		dev, err := m.db.GetDevice(deviceID)
		if err != nil || !dev.Enabled {
			return
		}
		pw, err := m.db.GetDevicePassword(deviceID)
		if err != nil {
			m.setStatus(deviceID, false, err.Error())
			time.Sleep(backoff)
			backoff = min(backoff*2, 60*time.Second)
			continue
		}
		ifaces, err := m.db.ListMonitoredInterfaces(deviceID)
		if err != nil || len(ifaces) == 0 {
			m.setStatus(deviceID, false, "no monitored interfaces — select interfaces and save")
			time.Sleep(time.Duration(dev.PollingIntervalSec) * time.Second)
			continue
		}
		var names []string
		for _, i := range ifaces {
			if i.Enabled {
				names = append(names, i.InterfaceName)
			}
		}
		if len(names) == 0 {
			m.setStatus(deviceID, false, "no enabled interfaces")
			time.Sleep(time.Duration(dev.PollingIntervalSec) * time.Second)
			continue
		}

		client := &mikrotik.Client{
			Host:     dev.Host,
			Port:     dev.Port,
			Username: dev.Username,
			Password: pw,
		}
		if err := client.TestConnection(); err != nil {
			m.setStatus(deviceID, false, "connection failed: "+err.Error())
			time.Sleep(backoff)
			backoff = min(backoff*2, 60*time.Second)
			continue
		}

		m.setStatus(deviceID, true, "")
		backoff = time.Second
		interval := time.Duration(dev.PollingIntervalSec) * time.Second
		if interval < 2*time.Second {
			interval = 2 * time.Second
		}

		pollOnce := func() bool {
			counters, err := client.FetchCounters(names)
			if err != nil {
				log.Printf("device %d poll error: %v", deviceID, err)
				m.setStatus(deviceID, false, err.Error())
				return false
			}

			now := time.Now()
			m.mu.Lock()
			if m.prev[deviceID] == nil {
				m.prev[deviceID] = make(map[string]counterSnap)
			}
			prevMap := m.prev[deviceID]

			for _, name := range names {
				cnt, ok := counters[name]
				if !ok {
					continue
				}
				var txBps, rxBps int64
				prev, hasPrev := prevMap[name]
				if hasPrev {
					elapsed := now.Sub(prev.at)
					if cnt.TxBytes >= prev.txBytes {
						txBps = mikrotik.BytesToBps(cnt.TxBytes-prev.txBytes, elapsed)
					}
					if cnt.RxBytes >= prev.rxBytes {
						rxBps = mikrotik.BytesToBps(cnt.RxBytes-prev.rxBytes, elapsed)
					}
				}
				prevMap[name] = counterSnap{rxBytes: cnt.RxBytes, txBytes: cnt.TxBytes, at: now}
				m.mu.Unlock()

				m.storeSample(models.TrafficSample{
					DeviceID:      deviceID,
					InterfaceName: name,
					TxBps:         txBps,
					RxBps:         rxBps,
					SampledAt:     now,
				})
				m.mu.Lock()
			}
			m.mu.Unlock()
			m.setStatus(deviceID, true, "")
			return true
		}

		pollOnce()

		ticker := time.NewTicker(interval)
	pollLoop:
		for {
			select {
			case <-stop:
				ticker.Stop()
				return
			case <-ticker.C:
				if !pollOnce() {
					ticker.Stop()
					break pollLoop
				}
			}
		}
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// SampleWriter batches DB writes
type SampleWriter struct {
	db     *models.DB
	in     <-chan models.TrafficSample
	stop   chan struct{}
	buffer []models.TrafficSample
}

func NewSampleWriter(db *models.DB, in <-chan models.TrafficSample) *SampleWriter {
	return &SampleWriter{db: db, in: in, stop: make(chan struct{})}
}

func (w *SampleWriter) Run() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case s, ok := <-w.in:
			if !ok {
				w.flush()
				return
			}
			w.buffer = append(w.buffer, s)
			if len(w.buffer) >= 100 {
				w.flush()
			}
		case <-ticker.C:
			w.flush()
		case <-w.stop:
			w.flush()
			return
		}
	}
}

func (w *SampleWriter) flush() {
	if len(w.buffer) == 0 {
		return
	}
	if err := w.db.InsertTrafficSamplesBatch(w.buffer); err != nil {
		log.Printf("sample write error: %v", err)
	}
	w.buffer = w.buffer[:0]
}

func (w *SampleWriter) Stop() { close(w.stop) }

func RunPruner(db *models.DB) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		for range ticker.C {
			settings, err := db.GetAppSettings()
			if err != nil {
				continue
			}
			before := time.Now().Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour)
			if err := db.PruneSamples(before); err != nil {
				log.Printf("prune error: %v", err)
			}
		}
	}()
}
