package services

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llwsm/internal/config"
)

type Service struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type Collector struct {
	procRoot string
	client   *http.Client
	defs     []config.Service
}

func New(procRoot string, defs []config.Service) *Collector {
	return &Collector{
		procRoot: procRoot,
		client:   &http.Client{Timeout: 3 * time.Second},
		defs:     defs,
	}
}

func (c *Collector) Collect() []Service {
	out := make([]Service, 0, len(c.defs))
	for _, d := range c.defs {
		switch d.Type {
		case "process":
			out = append(out, c.checkProcess(d))
		case "http":
			out = append(out, c.checkHTTP(d))
		default:
			out = append(out, Service{Name: d.Name, Type: d.Type, Status: "down", Detail: "unknown service type"})
		}
	}
	return out
}

func isPID(s string) bool {
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

func (c *Collector) checkProcess(d config.Service) Service {
	s := Service{Name: d.Name, Type: "process", Status: "down"}
	if d.Match == "" {
		s.Detail = "no match configured"
		return s
	}
	entries, err := os.ReadDir(c.procRoot)
	if err != nil {
		s.Detail = "cannot read /proc"
		return s
	}
	for _, e := range entries {
		if !isPID(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(c.procRoot, e.Name(), "comm"))
		if err != nil {
			continue
		}
		if strings.Contains(strings.TrimSpace(string(b)), d.Match) {
			s.Status = "up"
			s.Detail = "process found"
			return s
		}
	}
	s.Detail = "no process matching " + d.Match
	return s
}

func (c *Collector) checkHTTP(d config.Service) Service {
	s := Service{Name: d.Name, Type: "http", Status: "down"}
	if d.URL == "" {
		s.Detail = "no url configured"
		return s
	}
	client := *c.client
	client.Timeout = d.TimeoutDuration()
	resp, err := client.Get(d.URL)
	if err != nil {
		s.Detail = "unreachable"
		return s
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		s.Detail = "http " + resp.Status
		return s
	}
	s.Status = "up"
	s.Detail = "http " + resp.Status
	return s
}
