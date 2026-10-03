package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

type Container struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Image       string  `json:"image"`
	State       string  `json:"state"`
	Status      string  `json:"status"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
	MemoryLimit int64   `json:"memory_limit"`
	RXBytes     int64   `json:"rx_bytes"`
	TXBytes     int64   `json:"tx_bytes"`
}

type Info struct {
	Available  bool        `json:"available"`
	Error      string      `json:"error,omitempty"`
	Containers []Container `json:"containers"`
}

const maxStats = 32

type cpuSample struct {
	usage  uint64
	system uint64
	cores  int
}

type listEntry struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

type statsResponse struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     int    `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage int64 `json:"usage"`
		Limit int64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RXBytes int64 `json:"rx_bytes"`
		TXBytes int64 `json:"tx_bytes"`
	} `json:"networks"`
}

type Collector struct {
	client *http.Client
	base   string
	procs  int
	prev   map[string]cpuSample
}

func New(socketPath string) *Collector {
	d := net.Dialer{Timeout: 2 * time.Second}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "unix", socketPath)
		},
		DisableKeepAlives: true,
	}
	return newCollector(&http.Client{Transport: tr, Timeout: 3 * time.Second}, "http://docker")
}

func newCollector(client *http.Client, base string) *Collector {
	return &Collector{
		client: client,
		base:   base,
		procs:  runtime.NumCPU(),
		prev:   map[string]cpuSample{},
	}
}

func (c *Collector) Collect() Info {
	entries, err := c.list()
	if err != nil {
		return Info{Available: false, Error: err.Error(), Containers: []Container{}}
	}
	out := make([]Container, 0, len(entries))
	for _, e := range entries {
		ctr := Container{
			ID:     e.ID,
			Name:   containerName(e.Names),
			Image:  e.Image,
			State:  e.State,
			Status: e.Status,
		}
		if len(out) < maxStats && e.State == "running" {
			c.enrich(&ctr)
		}
		out = append(out, ctr)
	}
	return Info{Available: true, Containers: out}
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "unnamed"
	}
	return strings.TrimPrefix(names[0], "/")
}

func (c *Collector) list() ([]listEntry, error) {
	resp, err := c.client.Get(c.base + "/containers/json?all=true")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker api: unexpected status %s", resp.Status)
	}
	var entries []listEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Collector) enrich(ctr *Container) {
	resp, err := c.client.Get(c.base + "/containers/" + ctr.ID + "/stats?no-stream=true")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var s statsResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return
	}
	ctr.MemoryBytes = s.MemoryStats.Usage
	ctr.MemoryLimit = s.MemoryStats.Limit
	for _, n := range s.Networks {
		ctr.RXBytes += n.RXBytes
		ctr.TXBytes += n.TXBytes
	}
	cores := s.CPUStats.OnlineCPUs
	if cores <= 0 {
		cores = c.procs
	}
	cur := cpuSample{usage: s.CPUStats.CPUUsage.TotalUsage, system: s.CPUStats.SystemCPUUsage, cores: cores}
	if p, ok := c.prev[ctr.ID]; ok {
		dCPU := int64(cur.usage) - int64(p.usage)
		dSystem := int64(cur.system) - int64(p.system)
		if dCPU >= 0 && dSystem > 0 {
			ctr.CPUPercent = 100 * float64(dCPU) / float64(dSystem) * float64(cores)
			if ctr.CPUPercent < 0 {
				ctr.CPUPercent = 0
			}
		}
	}
	c.prev[ctr.ID] = cur
}
