package host

import (
	"sync"
	"time"

	"llwsm/internal/collector/cpu"
	"llwsm/internal/collector/docker"
	"llwsm/internal/collector/memory"
	"llwsm/internal/collector/network"
	"llwsm/internal/collector/sensors"
	"llwsm/internal/collector/services"
	"llwsm/internal/collector/storage"
	"llwsm/internal/collector/system"
	"llwsm/internal/config"
)

type Snapshot struct {
	Time       time.Time            `json:"time"`
	System     system.Info          `json:"system"`
	CPU        cpu.Info             `json:"cpu"`
	Memory     memory.Info          `json:"memory"`
	Storage    []storage.Filesystem `json:"storage"`
	Network    []network.Interface  `json:"network"`
	Sensors    []sensors.Sensor     `json:"sensors"`
	Containers docker.Info          `json:"containers"`
	Services   []services.Service   `json:"services"`
}

type Host struct {
	mu       sync.RWMutex
	snap     Snapshot
	sys      *system.Collector
	cpu      *cpu.Collector
	mem      *memory.Collector
	stor     *storage.Collector
	net      *network.Collector
	sens     *sensors.Collector
	dock     *docker.Collector
	services *services.Collector
}

func New(cfg *config.Config) *Host {
	h := &Host{
		sys:      system.New(cfg.Root),
		cpu:      cpu.New(cfg.Root),
		mem:      memory.New(cfg.Root),
		stor:     storage.New(cfg.Root, cfg.Storage.IgnoredFilesystems, nil),
		net:      network.New(cfg.Root),
		sens:     sensors.New(cfg.Root),
		dock:     docker.New(cfg.Docker.SocketPath),
		services: services.New(cfg.Root, cfg.Services),
	}
	h.Refresh()
	return h
}

func (h *Host) Snapshot() Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snap
}

func (h *Host) Refresh() {
	snap := Snapshot{
		Time:       time.Now(),
		System:     h.sys.Collect(),
		CPU:        h.cpu.Collect(),
		Memory:     h.mem.Collect(),
		Storage:    h.stor.Collect(),
		Network:    h.net.Collect(),
		Sensors:    h.sens.Collect(),
		Containers: h.dock.Collect(),
		Services:   h.services.Collect(),
	}
	if snap.Storage == nil {
		snap.Storage = []storage.Filesystem{}
	}
	if snap.Network == nil {
		snap.Network = []network.Interface{}
	}
	if snap.Sensors == nil {
		snap.Sensors = []sensors.Sensor{}
	}
	if snap.Services == nil {
		snap.Services = []services.Service{}
	}
	h.mu.Lock()
	h.snap = snap
	h.mu.Unlock()
}
