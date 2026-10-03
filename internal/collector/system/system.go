package system

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	Hostname string    `json:"hostname"`
	OS       string    `json:"os"`
	Arch     string    `json:"arch"`
	Kernel   string    `json:"kernel"`
	Model    string    `json:"model"`
	BootTime time.Time `json:"boot_time"`
	UptimeS  int64     `json:"uptime"`
}

type Collector struct {
	root string
}

func New(root string) *Collector {
	return &Collector{root: root}
}

func (c *Collector) Collect() Info {
	info := Info{OS: "linux", Arch: runtime.GOARCH, Model: "N/A"}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	} else {
		info.Hostname = "unknown"
	}
	info.Kernel = c.readTrim("proc/sys/kernel/osrelease")
	if m := c.readTrim("proc/device-tree/model"); m != "" {
		info.Model = m
	} else if m := c.readTrim("sys/devices/virtual/dmi/id/product_name"); m != "" {
		info.Model = m
	}
	if uptime, ok := c.uptime(); ok {
		info.UptimeS = int64(uptime)
		info.BootTime = time.Now().Add(-time.Duration(uptime * float64(time.Second)))
	}
	return info
}

func (c *Collector) uptime() (float64, bool) {
	b, err := os.ReadFile(filepath.Join(c.root, "proc", "uptime"))
	if err != nil {
		return 0, false
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (c *Collector) readTrim(rel string) string {
	b, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
