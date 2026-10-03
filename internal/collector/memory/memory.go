package memory

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Info struct {
	Total       int64   `json:"total"`
	Used        int64   `json:"used"`
	Available   int64   `json:"available"`
	Buffers     int64   `json:"buffers"`
	Cached      int64   `json:"cached"`
	Percent     float64 `json:"percent"`
	SwapTotal   int64   `json:"swap_total"`
	SwapUsed    int64   `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

type Collector struct {
	root string
	read func() (string, error)
}

func New(root string) *Collector {
	c := &Collector{root: root}
	c.read = c.readMeminfo
	return c
}

func (c *Collector) readMeminfo() (string, error) {
	b, err := os.ReadFile(filepath.Join(c.root, "proc", "meminfo"))
	return string(b), err
}

func parseMeminfo(raw string) map[string]int64 {
	out := map[string]int64{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if len(fields) > 1 && fields[1] == "kB" {
			v *= 1024
		}
		out[strings.TrimSpace(key)] = v
	}
	return out
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := 100 * float64(used) / float64(total)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return p
}

func (c *Collector) Collect() Info {
	var info Info
	raw, err := c.read()
	if err != nil {
		return info
	}
	m := parseMeminfo(raw)
	info.Total = m["MemTotal"]
	available := m["MemAvailable"]
	if available == 0 {
		available = m["MemFree"]
	}
	info.Available = available
	info.Used = info.Total - info.Available
	if info.Used < 0 {
		info.Used = 0
	}
	info.Buffers = m["Buffers"]
	info.Cached = m["Cached"]
	info.Percent = percent(info.Used, info.Total)
	info.SwapTotal = m["SwapTotal"]
	info.SwapUsed = m["SwapTotal"] - m["SwapFree"]
	if info.SwapUsed < 0 {
		info.SwapUsed = 0
	}
	info.SwapPercent = percent(info.SwapUsed, info.SwapTotal)
	return info
}
