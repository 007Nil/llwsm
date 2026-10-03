package cpu

import (
	"os"
	"path/filepath"
	"testing"
)

const procStat = "cpu  1000 0 500 8000 100 0 50 0 0 0\n" +
	"cpu0 500 0 250 4000 50 0 25 0 0 0\n" +
	"cpu1 500 0 250 4000 50 0 25 0 0 0\n" +
	"intr 12345\n" +
	"ctxt 999\n"

func TestParseProcStat(t *testing.T) {
	samples := parseProcStat(procStat)
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	if samples[0].total != 9650 {
		t.Errorf("aggregate total = %d, want 9650", samples[0].total)
	}
	if samples[0].idle != 8100 {
		t.Errorf("aggregate idle = %d, want 8100", samples[0].idle)
	}
	if samples[1].total != 4825 {
		t.Errorf("core0 total = %d, want 4825", samples[1].total)
	}
}

func TestUsageFrom(t *testing.T) {
	prev := sample{total: 9650, idle: 8100}
	cur := sample{total: 11650, idle: 8100}
	u, ok := usageFrom(prev, cur)
	if !ok {
		t.Fatal("expected ok")
	}
	if u != 100 {
		t.Errorf("usage = %f, want 100", u)
	}
	same := sample{total: 9650, idle: 8100}
	if _, ok := usageFrom(prev, same); ok {
		t.Error("expected no change to be invalid")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollect(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "proc", "loadavg"), "0.42 0.38 0.31 2/50 123\n")
	writeFile(t, filepath.Join(root, "proc", "cpuinfo"), "processor\t: 0\nmodel name\t: Test CPU Model\n")
	writeFile(t, filepath.Join(root, "sys", "devices", "system", "cpu", "cpu0", "cpufreq", "scaling_cur_freq"), "1800000\n")

	first := "cpu  100 0 100 900 0 0 0 0 0 0\ncpu0 50 0 50 450 0 0 0 0 0 0\ncpu1 50 0 50 450 0 0 0 0 0 0\n"
	second := "cpu  200 0 200 900 0 0 0 0 0 0\ncpu0 100 0 100 450 0 0 0 0 0 0\ncpu1 100 0 100 450 0 0 0 0 0 0\n"

	current := first
	c := &Collector{root: root}
	c.stat = func() (string, error) { return current, nil }

	previous := c.Collect()
	if previous.Usage != 0 {
		t.Errorf("first sample usage = %f, want 0", previous.Usage)
	}

	current = second
	info := c.Collect()

	if info.Cores != 2 {
		t.Errorf("cores = %d, want 2", info.Cores)
	}
	if info.Model != "Test CPU Model" {
		t.Errorf("model = %q", info.Model)
	}
	if info.FrequencyMHz != 1800 {
		t.Errorf("frequency = %f, want 1800", info.FrequencyMHz)
	}
	if info.Load != [3]float64{0.42, 0.38, 0.31} {
		t.Errorf("load = %v", info.Load)
	}
	if info.Usage != 100 {
		t.Errorf("usage = %f, want 100", info.Usage)
	}
	if len(info.PerCore) != 2 || info.PerCore[0] != 100 || info.PerCore[1] != 100 {
		t.Errorf("per core = %v", info.PerCore)
	}
}

func TestCollectMissingFiles(t *testing.T) {
	c := New(t.TempDir())
	info := c.Collect()
	if info.Model != "N/A" {
		t.Errorf("model = %q, want N/A", info.Model)
	}
	if info.FrequencyMHz != 0 {
		t.Errorf("frequency = %f, want 0", info.FrequencyMHz)
	}
	if info.Usage != 0 {
		t.Errorf("usage = %f, want 0", info.Usage)
	}
}
