package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
	"mikrotik-monitor/internal/models"
)

type Client struct {
	Host     string
	Port     int
	Username string
	Password string
}

// InterfaceCounters holds cumulative byte counters from the router.
type InterfaceCounters struct {
	RxBytes uint64
	TxBytes uint64
}

func (c *Client) Connect() (*routeros.Client, error) {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	return routeros.Dial(addr, c.Username, c.Password)
}

// Ping verifies an existing API session with a lightweight command.
func Ping(conn *routeros.Client) error {
	_, err := conn.Run("/system/identity/print")
	return err
}

// SystemResource holds CPU load and uptime from /system/resource/print.
type SystemResource struct {
	CPULoad int
	Uptime  string
}

// GetSystemResourceOn reads cpu-load and uptime using an existing session.
func GetSystemResourceOn(conn *routeros.Client) (SystemResource, error) {
	reply, err := conn.Run("/system/resource/print")
	if err != nil {
		return SystemResource{}, err
	}
	if len(reply.Re) == 0 {
		return SystemResource{}, fmt.Errorf("empty resource reply")
	}
	m := reply.Re[0].Map
	cpu := parseInt(m["cpu-load"])
	return SystemResource{
		CPULoad: cpu,
		Uptime:  strings.TrimSpace(m["uptime"]),
	}, nil
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

// Reboot issues /system/reboot on a new short-lived connection.
func (c *Client) Reboot() error {
	conn, err := c.Connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Run("/system/reboot")
	return err
}

func (c *Client) TestConnection() error {
	client, err := c.Connect()
	if err != nil {
		return err
	}
	defer client.Close()
	return Ping(client)
}

func (c *Client) ListInterfaces() ([]models.DiscoveredInterface, error) {
	client, err := c.Connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	reply, err := client.Run("/interface/print")
	if err != nil {
		return nil, err
	}
	var list []models.DiscoveredInterface
	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			continue
		}
		ifaceType := re.Map["type"]
		if ifaceType == "" {
			ifaceType = classifyFromName(name)
		} else {
			ifaceType = normalizeType(ifaceType)
		}
		list = append(list, models.DiscoveredInterface{Name: name, Type: ifaceType})
	}
	if list == nil {
		list = []models.DiscoveredInterface{}
	}
	return list, nil
}

// FetchCounters opens a short-lived API session (login + logout on the router).
// Prefer FetchCountersOn with a persistent connection when polling repeatedly.
func (c *Client) FetchCounters(names []string) (map[string]InterfaceCounters, error) {
	conn, err := c.Connect()
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	return FetchCountersOn(conn, names)
}

// FetchCountersOn reads cumulative rx-byte/tx-byte counters using an existing session.
// It tries three strategies in order to cover different RouterOS versions (6/7) and
// interface types (ethernet, VLAN, bridge, etc.).
func FetchCountersOn(conn *routeros.Client, names []string) (map[string]InterfaceCounters, error) {
	if len(names) == 0 {
		return map[string]InterfaceCounters{}, nil
	}
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}

	// Strategy 1: explicit proplist – most reliable cross-version approach.
	reply, err := conn.Run("/interface/print", "=.proplist=name,rx-byte,tx-byte")
	if err == nil {
		out := extractCounters(reply, want)
		if hasNonZero(out) {
			return out, nil
		}
		log.Printf("[mikrotik] proplist returned zero byte counts, trying stats")
	} else {
		log.Printf("[mikrotik] proplist attempt failed (%v), trying stats", err)
	}

	// Strategy 2: stats sub-command (RouterOS 6.x explicitly exposes this).
	reply2, err2 := conn.Run("/interface/print", "stats")
	if err2 == nil {
		out2 := extractCounters(reply2, want)
		if hasNonZero(out2) {
			return out2, nil
		}
		log.Printf("[mikrotik] stats returned zero byte counts, trying plain print")
	} else {
		log.Printf("[mikrotik] stats attempt failed (%v), trying plain print", err2)
	}

	// Strategy 3: plain /interface/print without extra args.
	reply3, err3 := conn.Run("/interface/print")
	if err3 != nil {
		return nil, fmt.Errorf("all counter fetch strategies failed; last: %w", err3)
	}
	out3 := extractCounters(reply3, want)
	if len(out3) == 0 {
		return nil, fmt.Errorf("interfaces not found on router: %v", names)
	}
	if !hasNonZero(out3) {
		log.Printf("[mikrotik] WARNING: all rx-byte/tx-byte values are 0 for %v – check if counters are enabled", names)
	}
	return out3, nil
}

// extractCounters maps a RouterOS reply into InterfaceCounters keyed by name.
// It logs the raw rx-byte/tx-byte values so mismatches can be spotted quickly.
func extractCounters(reply *routeros.Reply, want map[string]struct{}) map[string]InterfaceCounters {
	out := make(map[string]InterfaceCounters)
	for _, re := range reply.Re {
		name := re.Map["name"]
		if _, ok := want[name]; !ok {
			continue
		}
		rxRaw := re.Map["rx-byte"]
		txRaw := re.Map["tx-byte"]
		rx := parseUint64(rxRaw)
		tx := parseUint64(txRaw)
		log.Printf("[mikrotik] iface=%q rx-byte=%q→%d tx-byte=%q→%d", name, rxRaw, rx, txRaw, tx)
		out[name] = InterfaceCounters{RxBytes: rx, TxBytes: tx}
	}
	return out
}

func hasNonZero(m map[string]InterfaceCounters) bool {
	for _, c := range m {
		if c.RxBytes > 0 || c.TxBytes > 0 {
			return true
		}
	}
	return false
}

func parseUint64(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func normalizeType(t string) string {
	t = strings.ToLower(t)
	switch {
	case strings.Contains(t, "ether"):
		return "ethernet"
	case strings.Contains(t, "vlan"):
		return "vlan"
	case strings.Contains(t, "bridge"):
		return "bridge"
	case strings.Contains(t, "bond"):
		return "bonding"
	case strings.Contains(t, "wlan"), strings.Contains(t, "wireless"):
		return "wireless"
	case strings.Contains(t, "ppp"):
		return "ppp"
	default:
		return "other"
	}
}

func classifyFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "ether"), strings.HasPrefix(lower, "sfp"):
		return "ethernet"
	case strings.HasPrefix(lower, "vlan"), strings.HasPrefix(lower, "vl-"):
		return "vlan"
	case strings.HasPrefix(lower, "bridge"):
		return "bridge"
	case strings.HasPrefix(lower, "bond"):
		return "bonding"
	case strings.HasPrefix(lower, "wlan"):
		return "wireless"
	case strings.HasPrefix(lower, "ppp"):
		return "ppp"
	default:
		return "other"
	}
}

// BytesToBps converts byte counter delta to bits per second.
func BytesToBps(deltaBytes uint64, elapsed time.Duration) int64 {
	if elapsed <= 0 {
		return 0
	}
	secs := elapsed.Seconds()
	if secs <= 0 {
		return 0
	}
	return int64(float64(deltaBytes) * 8 / secs)
}
