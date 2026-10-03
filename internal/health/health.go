package health

import (
	"fmt"

	"llwsm/internal/host"
)

const (
	Healthy  = "HEALTHY"
	Warning  = "WARNING"
	Critical = "CRITICAL"
)

type Thresholds struct {
	CPUWarning, CPUCritical         float64
	MemoryWarning, MemoryCritical   float64
	SwapWarning, SwapCritical       float64
	StorageWarning, StorageCritical float64
}

type Reason struct {
	Level   string `json:"level"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

type Result struct {
	Status  string   `json:"status"`
	Reasons []Reason `json:"reasons"`
}

func Evaluate(s *host.Snapshot, t Thresholds) Result {
	reasons := []Reason{}
	status := Healthy

	flag := func(level, source, message string) {
		if level == Critical {
			status = Critical
		} else if status != Critical {
			status = Warning
		}
		reasons = append(reasons, Reason{Level: level, Source: source, Message: message})
	}

	if t.CPUCritical > 0 && s.CPU.Usage >= t.CPUCritical {
		flag(Critical, "cpu", fmt.Sprintf("CPU usage %.1f%% is at or above the critical threshold %.0f%%", s.CPU.Usage, t.CPUCritical))
	} else if t.CPUWarning > 0 && s.CPU.Usage >= t.CPUWarning {
		flag(Warning, "cpu", fmt.Sprintf("CPU usage %.1f%% is at or above the warning threshold %.0f%%", s.CPU.Usage, t.CPUWarning))
	}

	if t.MemoryCritical > 0 && s.Memory.Percent >= t.MemoryCritical {
		flag(Critical, "memory", fmt.Sprintf("memory usage %.1f%% is at or above the critical threshold %.0f%%", s.Memory.Percent, t.MemoryCritical))
	} else if t.MemoryWarning > 0 && s.Memory.Percent >= t.MemoryWarning {
		flag(Warning, "memory", fmt.Sprintf("memory usage %.1f%% is at or above the warning threshold %.0f%%", s.Memory.Percent, t.MemoryWarning))
	}

	if s.Memory.SwapTotal > 0 {
		if t.SwapCritical > 0 && s.Memory.SwapPercent >= t.SwapCritical {
			flag(Critical, "swap", fmt.Sprintf("swap usage %.1f%% is at or above the critical threshold %.0f%%", s.Memory.SwapPercent, t.SwapCritical))
		} else if t.SwapWarning > 0 && s.Memory.SwapPercent >= t.SwapWarning {
			flag(Warning, "swap", fmt.Sprintf("swap usage %.1f%% is at or above the warning threshold %.0f%%", s.Memory.SwapPercent, t.SwapWarning))
		}
	}

	for _, fs := range s.Storage {
		if fs.Total <= 0 {
			continue
		}
		if t.StorageCritical > 0 && fs.Percent >= t.StorageCritical {
			flag(Critical, "storage", fmt.Sprintf("filesystem %q is %.0f%% full (critical threshold %.0f%%)", fs.Mount, fs.Percent, t.StorageCritical))
		} else if t.StorageWarning > 0 && fs.Percent >= t.StorageWarning {
			flag(Warning, "storage", fmt.Sprintf("filesystem %q is %.0f%% full (warning threshold %.0f%%)", fs.Mount, fs.Percent, t.StorageWarning))
		}
	}

	for _, svc := range s.Services {
		if svc.Status == "down" {
			flag(Warning, "service", fmt.Sprintf("service %q is not running (%s)", svc.Name, svc.Detail))
		}
	}

	return Result{Status: status, Reasons: reasons}
}
