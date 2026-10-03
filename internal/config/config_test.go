package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 8090 {
		t.Errorf("server = %+v", cfg.Server)
	}
	if cfg.ListenAddr() != "127.0.0.1:8090" {
		t.Errorf("listen = %q", cfg.ListenAddr())
	}
	if d := cfg.Interval(); d != 2*time.Second {
		t.Errorf("interval = %v", d)
	}
	if cfg.Root != "/" {
		t.Errorf("root = %q", cfg.Root)
	}
	if len(cfg.Storage.IgnoredFilesystems) == 0 {
		t.Error("expected default ignored filesystems")
	}
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr() != "127.0.0.1:8090" {
		t.Errorf("listen = %q", cfg.ListenAddr())
	}
}

func TestLoadEmptyYieldDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8090 {
		t.Errorf("port = %d", cfg.Server.Port)
	}
}

func TestLoadFileOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server:
  host: 0.0.0.0
  port: 9999
monitor:
  interval: 500ms
storage:
  ignored_filesystems:
    - proc
    - overlay
health:
  cpu:
    warning: 60
    critical: 90
docker:
  socket_path: /tmp/test.sock
services:
  - name: Pi-hole
    type: process
    match: pihole
  - name: Web
    type: http
    url: http://127.0.0.1:8080
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 9999 {
		t.Errorf("server = %+v", cfg.Server)
	}
	if cfg.ListenAddr() != "0.0.0.0:9999" {
		t.Errorf("listen = %q", cfg.ListenAddr())
	}
	if d := cfg.Interval(); d != 500*time.Millisecond {
		t.Errorf("interval = %v", d)
	}
	if len(cfg.Storage.IgnoredFilesystems) != 2 {
		t.Errorf("ignored = %v", cfg.Storage.IgnoredFilesystems)
	}
	if cfg.Health.CPU.Warning != 60 || cfg.Health.CPU.Critical != 90 {
		t.Errorf("cpu thresholds = %+v", cfg.Health.CPU)
	}
	if cfg.Health.Memory.Warning != 85 {
		t.Errorf("memory thresholds should keep default, got %+v", cfg.Health.Memory)
	}
	if cfg.Docker.SocketPath != "/tmp/test.sock" {
		t.Errorf("socket = %q", cfg.Docker.SocketPath)
	}
	if len(cfg.Services) != 2 || cfg.Services[0].Match != "pihole" || cfg.Services[1].Type != "http" {
		t.Errorf("services = %+v", cfg.Services)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("server: [broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestInvalidValuesReverted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server:
  host: ""
  port: 99999
monitor:
  interval: bogus
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 8090 {
		t.Errorf("server = %+v", cfg.Server)
	}
	if d := cfg.Interval(); d != 2*time.Second {
		t.Errorf("interval = %v", d)
	}
}

func TestServiceTimeout(t *testing.T) {
	if d := (Service{Timeout: "1s"}).TimeoutDuration(); d != time.Second {
		t.Errorf("timeout = %v", d)
	}
	if d := (Service{}).TimeoutDuration(); d != 3*time.Second {
		t.Errorf("default timeout = %v", d)
	}
}
