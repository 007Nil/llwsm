package health

import (
	"testing"

	"llwsm/internal/collector/cpu"
	"llwsm/internal/collector/memory"
	"llwsm/internal/collector/services"
	"llwsm/internal/collector/storage"
	"llwsm/internal/host"
)

func thresholds() Thresholds {
	return Thresholds{
		CPUWarning: 80, CPUCritical: 95,
		MemoryWarning: 85, MemoryCritical: 95,
		SwapWarning: 50, SwapCritical: 80,
		StorageWarning: 85, StorageCritical: 95,
	}
}

func TestHealthy(t *testing.T) {
	s := &host.Snapshot{
		CPU:      cpu.Info{Usage: 10},
		Memory:   memory.Info{Percent: 30, SwapTotal: 0, SwapPercent: 0},
		Storage:  []storage.Filesystem{{Mount: "/", Percent: 50, Total: 1000}},
		Services: []services.Service{{Name: "X", Status: "up"}},
	}
	r := Evaluate(s, thresholds())
	if r.Status != Healthy {
		t.Fatalf("status = %s, reasons = %+v", r.Status, r.Reasons)
	}
	if len(r.Reasons) != 0 {
		t.Errorf("reasons = %+v", r.Reasons)
	}
}

func TestWarningMemoryStorageService(t *testing.T) {
	s := &host.Snapshot{
		CPU:      cpu.Info{Usage: 50},
		Memory:   memory.Info{Percent: 90, SwapTotal: 1000, SwapPercent: 10},
		Storage:  []storage.Filesystem{{Mount: "/mnt/data", Percent: 90, Total: 1000}},
		Services: []services.Service{{Name: "Pi-hole", Status: "down", Detail: "no process matching pihole"}},
	}
	r := Evaluate(s, thresholds())
	if r.Status != Warning {
		t.Fatalf("status = %s, reasons = %+v", r.Status, r.Reasons)
	}
	sources := map[string]bool{}
	for _, reason := range r.Reasons {
		sources[reason.Source] = true
		if reason.Level != Warning {
			t.Errorf("reason level = %s, want warning: %+v", reason.Level, reason)
		}
		if reason.Message == "" {
			t.Error("reason must carry an explanation")
		}
	}
	for _, want := range []string{"memory", "storage", "service"} {
		if !sources[want] {
			t.Errorf("missing reason for %q: %+v", want, r.Reasons)
		}
	}
}

func TestCriticalCPU(t *testing.T) {
	s := &host.Snapshot{CPU: cpu.Info{Usage: 99}}
	r := Evaluate(s, thresholds())
	if r.Status != Critical {
		t.Fatalf("status = %s", r.Status)
	}
	found := false
	for _, reason := range r.Reasons {
		if reason.Source == "cpu" && reason.Level == Critical {
			found = true
		}
	}
	if !found {
		t.Errorf("expected critical cpu reason: %+v", r.Reasons)
	}
}

func TestCriticalBeatsWarning(t *testing.T) {
	s := &host.Snapshot{
		CPU:    cpu.Info{Usage: 96},
		Memory: memory.Info{Percent: 90},
	}
	r := Evaluate(s, thresholds())
	if r.Status != Critical {
		t.Fatalf("status = %s, reasons = %+v", r.Status, r.Reasons)
	}
}

func TestSwapOnlyWhenPresent(t *testing.T) {
	s := &host.Snapshot{
		CPU:    cpu.Info{Usage: 0},
		Memory: memory.Info{Percent: 10, SwapTotal: 0, SwapPercent: 100},
	}
	r := Evaluate(s, thresholds())
	if r.Status != Healthy {
		t.Fatalf("no swap total should not trigger swap warnings: %+v", r.Reasons)
	}
}

func TestZeroThresholdDisablesCheck(t *testing.T) {
	t0 := thresholds()
	t0.CPUWarning = 0
	t0.CPUCritical = 0
	s := &host.Snapshot{CPU: cpu.Info{Usage: 100}}
	r := Evaluate(s, t0)
	if r.Status != Healthy {
		t.Fatalf("disabled check should not fire: %+v", r.Reasons)
	}
}

func TestStorageSkipsZeroTotal(t *testing.T) {
	s := &host.Snapshot{
		Storage: []storage.Filesystem{{Mount: "/tmp", Percent: 100, Total: 0}},
	}
	r := Evaluate(s, thresholds())
	if r.Status != Healthy {
		t.Fatalf("zero total fs should be skipped: %+v", r.Reasons)
	}
}
