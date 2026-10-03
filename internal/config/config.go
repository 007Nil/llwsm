package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Monitor struct {
	Interval string `yaml:"interval"`
}

type Storage struct {
	IgnoredFilesystems []string `yaml:"ignored_filesystems"`
}

type Threshold struct {
	Warning  float64 `yaml:"warning"`
	Critical float64 `yaml:"critical"`
}

type Health struct {
	CPU     Threshold `yaml:"cpu"`
	Memory  Threshold `yaml:"memory"`
	Swap    Threshold `yaml:"swap"`
	Storage Threshold `yaml:"storage"`
}

type Docker struct {
	SocketPath string `yaml:"socket_path"`
}

type Service struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Match   string `yaml:"match"`
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

func (s Service) TimeoutDuration() time.Duration {
	d, err := time.ParseDuration(s.Timeout)
	if err != nil || d <= 0 {
		return 3 * time.Second
	}
	return d
}

type Config struct {
	Root     string    `yaml:"root"`
	Server   Server    `yaml:"server"`
	Monitor  Monitor   `yaml:"monitor"`
	Storage  Storage   `yaml:"storage"`
	Health   Health    `yaml:"health"`
	Docker   Docker    `yaml:"docker"`
	Services []Service `yaml:"services"`
}

var defaultIgnoredFilesystems = []string{
	"proc", "sysfs", "devtmpfs", "tmpfs", "devpts", "mqueue",
	"cgroup", "cgroup2", "securityfs", "pstore", "efivarfs",
	"bpf", "debugfs", "tracefs", "fusectl", "configfs",
	"ramfs", "autofs", "binfmt_misc", "nsfs", "selinuxfs", "rootfs",
	"squashfs", "hugetlbfs",
}

func Defaults() *Config {
	return &Config{
		Root:    "/",
		Server:  Server{Host: "127.0.0.1", Port: 8090},
		Monitor: Monitor{Interval: "2s"},
		Storage: Storage{IgnoredFilesystems: append([]string{}, defaultIgnoredFilesystems...)},
		Health: Health{
			CPU:     Threshold{Warning: 80, Critical: 95},
			Memory:  Threshold{Warning: 85, Critical: 95},
			Swap:    Threshold{Warning: 50, Critical: 80},
			Storage: Threshold{Warning: 85, Critical: 95},
		},
		Docker: Docker{SocketPath: "/var/run/docker.sock"},
	}
}

func Load(path string) (*Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.validate()
	return cfg, nil
}

func (c *Config) validate() {
	if c.Root == "" {
		c.Root = "/"
	}
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		c.Server.Port = 8090
	}
	if c.Monitor.Interval == "" {
		c.Monitor.Interval = "2s"
	}
	if c.Docker.SocketPath == "" {
		c.Docker.SocketPath = "/var/run/docker.sock"
	}
	if len(c.Storage.IgnoredFilesystems) == 0 {
		c.Storage.IgnoredFilesystems = append([]string{}, defaultIgnoredFilesystems...)
	}
	if c.Health.CPU == (Threshold{}) {
		c.Health.CPU = Threshold{Warning: 80, Critical: 95}
	}
	if c.Health.Memory == (Threshold{}) {
		c.Health.Memory = Threshold{Warning: 85, Critical: 95}
	}
	if c.Health.Swap == (Threshold{}) {
		c.Health.Swap = Threshold{Warning: 50, Critical: 80}
	}
	if c.Health.Storage == (Threshold{}) {
		c.Health.Storage = Threshold{Warning: 85, Critical: 95}
	}
}

func (c *Config) Interval() time.Duration {
	d, err := time.ParseDuration(c.Monitor.Interval)
	if err != nil || d <= 0 {
		return 2 * time.Second
	}
	return d
}

func (c *Config) ListenAddr() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}
