package wshub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mikrotik-monitor/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:      func(r *http.Request) bool { return true },
	HandshakeTimeout: 10 * time.Second,
}

// SnapshotFunc returns the latest samples for the given device IDs.
// Set on Hub by the poller manager after construction.
type SnapshotFunc func(deviceIDs []int64) []models.TrafficSample

// StatusFunc returns current online/error state for the given device IDs.
type StatusFunc func(deviceIDs []int64) []models.DeviceStatus

type Hub struct {
	mu          sync.RWMutex
	clients     map[*client]struct{}
	GetSnapshot SnapshotFunc // optional; called when a client subscribes
	GetStatus   StatusFunc   // optional; called when a client subscribes
}

type client struct {
	hub       *Hub
	conn      *websocket.Conn
	deviceIDs map[int64]struct{}
	send      chan []byte
}

func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	// Keep the connection alive with ping/pong.
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	c := &client{
		hub:       h,
		conn:      conn,
		deviceIDs: make(map[int64]struct{}),
		send:      make(chan []byte, 512),
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump()
}

func (c *client) readPump() {
	defer func() {
		c.hub.mu.Lock()
		delete(c.hub.clients, c)
		c.hub.mu.Unlock()
		close(c.send)
		c.conn.Close()
	}()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Reset read deadline on any message received.
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var sub struct {
			Action    string  `json:"action"`
			DeviceIDs []int64 `json:"device_ids"`
		}
		if json.Unmarshal(msg, &sub) != nil || sub.Action != "subscribe" {
			continue
		}

		c.deviceIDs = make(map[int64]struct{})
		for _, id := range sub.DeviceIDs {
			c.deviceIDs[id] = struct{}{}
		}

		// Push snapshot immediately so the client sees current data without
		// waiting for the next poll tick.
		if len(sub.DeviceIDs) == 0 {
			continue
		}
		if c.hub.GetStatus != nil {
			for _, st := range c.hub.GetStatus(sub.DeviceIDs) {
				data, err := json.Marshal(map[string]interface{}{"type": "status", "payload": st})
				if err != nil {
					continue
				}
				select {
				case c.send <- data:
				default:
					log.Printf("ws: status snapshot buffer full, dropping status")
				}
			}
		}
		if c.hub.GetSnapshot != nil {
			for _, sample := range c.hub.GetSnapshot(sub.DeviceIDs) {
				data, err := json.Marshal(sample)
				if err != nil {
					continue
				}
				select {
				case c.send <- data:
				default:
					log.Printf("ws: snapshot buffer full, dropping sample")
				}
			}
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) Broadcast(sample models.TrafficSample) {
	data, err := json.Marshal(sample)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if len(c.deviceIDs) > 0 {
			if _, ok := c.deviceIDs[sample.DeviceID]; !ok {
				continue
			}
		}
		select {
		case c.send <- data:
		default:
			log.Printf("ws: client buffer full, dropping broadcast")
		}
	}
}

func (h *Hub) BroadcastStatus(deviceID int64, online bool, errMsg string) {
	st := models.DeviceStatus{DeviceID: deviceID, Online: online, Error: errMsg}
	data, _ := json.Marshal(map[string]interface{}{"type": "status", "payload": st})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if len(c.deviceIDs) > 0 {
			if _, ok := c.deviceIDs[deviceID]; !ok {
				continue
			}
		}
		select {
		case c.send <- data:
		default:
		}
	}
}
