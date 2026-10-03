package network

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Interface struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	Up      bool     `json:"up"`
	IPv4    []string `json:"ipv4"`
	IPv6    []string `json:"ipv6"`
	RXRate  float64  `json:"rx_rate"`
	TXRate  float64  `json:"tx_rate"`
	RXTotal uint64   `json:"rx_total"`
	TXTotal uint64   `json:"tx_total"`
}

type Collector struct {
	root     string
	prev     map[string][2]uint64
	prevTime time.Time
	now      func() time.Time
}

func New(root string) *Collector {
	return &Collector{root: root, prev: map[string][2]uint64{}, now: time.Now}
}

func (c *Collector) Collect() []Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []Interface{}
	}
	return c.CollectWith(ifaces)
}

func (c *Collector) CollectWith(ifaces []net.Interface) []Interface {
	now := c.now()
	dt := 0.0
	if !c.prevTime.IsZero() {
		dt = now.Sub(c.prevTime).Seconds()
	}
	out := make([]Interface, 0, len(ifaces))
	next := map[string][2]uint64{}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		in := Interface{
			Name:  ifi.Name,
			Up:    ifi.Flags&net.FlagUp != 0,
			State: c.state(ifi),
			IPv4:  []string{},
			IPv6:  []string{},
		}
		if addrs, err := ifi.Addrs(); err == nil {
			in.IPv4, in.IPv6 = splitAddrs(addrs)
		}
		rx, tx, ok := c.counters(ifi.Name)
		in.RXTotal, in.TXTotal = rx, tx
		if ok {
			next[ifi.Name] = [2]uint64{rx, tx}
			if p, had := c.prev[ifi.Name]; had && dt > 0 {
				in.RXRate = rate(p[0], rx, dt)
				in.TXRate = rate(p[1], tx, dt)
			}
		}
		out = append(out, in)
	}
	c.prev = next
	c.prevTime = now
	return out
}

func splitAddrs(addrs []net.Addr) (v4, v6 []string) {
	v4, v6 = []string{}, []string{}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.To4() != nil {
			v4 = append(v4, ipnet.IP.String())
		} else {
			v6 = append(v6, ipnet.IP.String())
		}
	}
	return
}

func rate(prev, cur uint64, dt float64) float64 {
	if cur < prev || dt <= 0 {
		return 0
	}
	return float64(cur-prev) / dt
}

func (c *Collector) state(ifi net.Interface) string {
	b, err := os.ReadFile(filepath.Join(c.root, "sys", "class", "net", ifi.Name, "operstate"))
	if err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s
		}
	}
	if ifi.Flags&net.FlagUp != 0 {
		return "up"
	}
	return "down"
}

func (c *Collector) counters(name string) (uint64, uint64, bool) {
	base := filepath.Join(c.root, "sys", "class", "net", name, "statistics")
	rx, err1 := readCounter(filepath.Join(base, "rx_bytes"))
	tx, err2 := readCounter(filepath.Join(base, "tx_bytes"))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return rx, tx, true
}

func readCounter(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}
