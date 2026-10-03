package cpu

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Info struct {
	Usage        float64    `json:"usage"`
	Cores        int        `json:"cores"`
	PerCore      []float64  `json:"per_core"`
	Load         [3]float64 `json:"load"`
	Model        string     `json:"model"`
	FrequencyMHz float64    `json:"frequency_mhz"`
}

type sample struct {
	total uint64
	idle  uint64
}

type Collector struct {
	root     string
	stat     func() (string, error)
	prev     []sample
	havePrev bool
	last     []float64
}

func New(root string) *Collector {
	c := &Collector{root: root}
	c.stat = c.readStat
	return c
}

func (c *Collector) readStat() (string, error) {
	b, err := os.ReadFile(filepath.Join(c.root, "proc", "stat"))
	return string(b), err
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseProcStat(raw string) []sample {
	var out []sample
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		if name != "cpu" && (strings.TrimPrefix(name, "cpu") == "" || !isDigits(name[3:])) {
			continue
		}
		var vals []uint64
		for _, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				vals = nil
				break
			}
			vals = append(vals, v)
		}
		if len(vals) < 4 {
			continue
		}
		s := sample{}
		n := len(vals)
		if n > 8 {
			n = 8
		}
		for _, v := range vals[:n] {
			s.total += v
		}
		s.idle = vals[3]
		if len(vals) > 4 {
			s.idle += vals[4]
		}
		out = append(out, s)
	}
	return out
}

func usageFrom(prev, cur sample) (float64, bool) {
	dTotal := int64(cur.total) - int64(prev.total)
	dIdle := int64(cur.idle) - int64(prev.idle)
	if dTotal <= 0 || dIdle < 0 {
		return 0, false
	}
	u := 100 * (1 - float64(dIdle)/float64(dTotal))
	if u < 0 {
		u = 0
	}
	if u > 100 {
		u = 100
	}
	return u, true
}

func (c *Collector) Collect() Info {
	info := Info{Cores: runtime.NumCPU(), Model: c.model(), FrequencyMHz: c.frequency()}
	raw, err := c.stat()
	if err != nil {
		info.PerCore = []float64{}
		if len(c.last) > 0 {
			info.Usage = c.last[0]
		}
		info.Load = c.load()
		return info
	}
	samples := parseProcStat(raw)
	if len(samples) == 0 {
		info.PerCore = []float64{}
		if len(c.last) > 0 {
			info.Usage = c.last[0]
		}
		info.Load = c.load()
		return info
	}
	usage := make([]float64, len(samples))
	for i, s := range samples {
		if c.havePrev && i < len(c.prev) {
			if v, ok := usageFrom(c.prev[i], s); ok {
				usage[i] = v
			} else if i < len(c.last) {
				usage[i] = c.last[i]
			}
		}
	}
	c.prev = samples
	c.havePrev = true
	c.last = usage
	info.Usage = usage[0]
	info.PerCore = append([]float64{}, usage[1:]...)
	if len(usage) > 1 {
		info.Cores = len(usage) - 1
	}
	info.Load = c.load()
	return info
}

func (c *Collector) load() [3]float64 {
	b, err := os.ReadFile(filepath.Join(c.root, "proc", "loadavg"))
	if err != nil {
		return [3]float64{}
	}
	f := strings.Fields(string(b))
	var out [3]float64
	for i := 0; i < 3 && i < len(f); i++ {
		v, err := strconv.ParseFloat(f[i], 64)
		if err == nil {
			out[i] = v
		}
	}
	return out
}

func (c *Collector) model() string {
	b, err := os.ReadFile(filepath.Join(c.root, "proc", "cpuinfo"))
	if err != nil {
		return "N/A"
	}
	model, hardware := "", ""
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "model name":
			if model == "" {
				model = v
			}
		case "Hardware":
			if hardware == "" {
				hardware = v
			}
		}
	}
	if model != "" {
		return model
	}
	if hardware != "" {
		return hardware
	}
	return "N/A"
}

func (c *Collector) frequency() float64 {
	for _, rel := range []string{
		"sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq",
		"sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
	} {
		b, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err == nil && v > 0 {
			return v / 1000
		}
	}
	return 0
}
