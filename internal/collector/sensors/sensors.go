package sensors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Sensor struct {
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
}

type Collector struct {
	root string
}

func New(root string) *Collector {
	return &Collector{root: root}
}

func (c *Collector) Collect() []Sensor {
	out := []Sensor{}
	entries, err := os.ReadDir(filepath.Join(c.root, "sys", "class", "thermal"))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		base := filepath.Join(c.root, "sys", "class", "thermal", e.Name())
		b, err := os.ReadFile(filepath.Join(base, "temp"))
		if err != nil {
			continue
		}
		milli, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			continue
		}
		name := e.Name()
		if tb, err := os.ReadFile(filepath.Join(base, "type")); err == nil {
			if s := strings.TrimSpace(string(tb)); s != "" {
				name = s
			}
		}
		out = append(out, Sensor{Name: name, Temperature: float64(milli) / 1000})
	}
	return out
}
