package memory

import (
	"errors"
	"testing"
)

const meminfo = `MemTotal:       1932735 kB
MemFree:         500000 kB
MemAvailable:    900000 kB
Buffers:          50000 kB
Cached:          400000 kB
SwapTotal:       2752000 kB
SwapFree:        2700000 kB
HugePages_Total: 4
`

func TestParseMeminfo(t *testing.T) {
	m := parseMeminfo(meminfo)
	if m["MemTotal"] != 1932735*1024 {
		t.Errorf("MemTotal = %d", m["MemTotal"])
	}
	if m["HugePages_Total"] != 4 {
		t.Errorf("HugePages_Total = %d", m["HugePages_Total"])
	}
}

func TestCollect(t *testing.T) {
	c := &Collector{}
	c.read = func() (string, error) { return meminfo, nil }
	info := c.Collect()

	if info.Total != 1932735*1024 {
		t.Errorf("total = %d", info.Total)
	}
	if info.Available != 900000*1024 {
		t.Errorf("available = %d", info.Available)
	}
	wantUsed := int64(1932735-900000) * 1024
	if info.Used != wantUsed {
		t.Errorf("used = %d, want %d (total - available)", info.Used, wantUsed)
	}
	if info.SwapTotal != 2752000*1024 || info.SwapUsed != 52000*1024 {
		t.Errorf("swap = %+v", info)
	}
	if info.Percent <= 0 || info.Percent >= 100 {
		t.Errorf("percent = %f", info.Percent)
	}
	wantPercent := 100 * float64(wantUsed) / float64(info.Total)
	if info.Percent < wantPercent-0.001 || info.Percent > wantPercent+0.001 {
		t.Errorf("percent = %f, want %f", info.Percent, wantPercent)
	}
}

func TestCollectLegacyWithoutMemAvailable(t *testing.T) {
	c := &Collector{}
	c.read = func() (string, error) {
		return "MemTotal:       1000 kB\nMemFree:        400 kB\n", nil
	}
	info := c.Collect()
	if info.Available != 400*1024 {
		t.Errorf("available = %d, want free fallback", info.Available)
	}
	if info.Used != 600*1024 {
		t.Errorf("used = %d", info.Used)
	}
}

func TestCollectReadError(t *testing.T) {
	c := &Collector{}
	c.read = func() (string, error) { return "", errors.New("no such file") }
	info := c.Collect()
	if info.Total != 0 || info.Percent != 0 {
		t.Errorf("expected zero info, got %+v", info)
	}
}
