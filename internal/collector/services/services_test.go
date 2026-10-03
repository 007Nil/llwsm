package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"llwsm/internal/config"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcessCheck(t *testing.T) {
	proc := t.TempDir()
	mustWrite(t, filepath.Join(proc, "123", "comm"), "pihole\n")
	mustWrite(t, filepath.Join(proc, "456", "comm"), "mopidy\n")
	mustWrite(t, filepath.Join(proc, "notapid", "comm"), "pihole\n")

	defs := []config.Service{
		{Name: "Pi-hole", Type: "process", Match: "pihole"},
		{Name: "Mopidy", Type: "process", Match: "mopidy"},
		{Name: "Missing", Type: "process", Match: "nomatch"},
	}
	c := New(proc, defs)
	out := c.Collect()
	if len(out) != 3 {
		t.Fatalf("got %d services", len(out))
	}
	if out[0].Status != "up" {
		t.Errorf("pihole = %+v", out[0])
	}
	if out[1].Status != "up" {
		t.Errorf("mopidy = %+v", out[1])
	}
	if out[2].Status != "down" || out[2].Detail == "" {
		t.Errorf("missing = %+v", out[2])
	}
}

func TestProcessCheckNoMatchConfigured(t *testing.T) {
	c := New(t.TempDir(), []config.Service{{Name: "X", Type: "process"}})
	out := c.Collect()
	if out[0].Status != "down" {
		t.Errorf("expected down, got %+v", out[0])
	}
}

func TestHTTPCheck(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okSrv.Close()
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer badSrv.Close()

	defs := []config.Service{
		{Name: "Ok", Type: "http", URL: okSrv.URL},
		{Name: "Bad", Type: "http", URL: badSrv.URL},
		{Name: "Dead", Type: "http", URL: "http://127.0.0.1:1"},
	}
	c := New(t.TempDir(), defs)
	out := c.Collect()
	if out[0].Status != "up" {
		t.Errorf("ok = %+v", out[0])
	}
	if out[1].Status != "down" {
		t.Errorf("bad = %+v", out[1])
	}
	if out[2].Status != "down" {
		t.Errorf("dead = %+v", out[2])
	}
}

func TestUnknownType(t *testing.T) {
	c := New(t.TempDir(), []config.Service{{Name: "X", Type: "database"}})
	out := c.Collect()
	if out[0].Status != "down" {
		t.Errorf("expected down, got %+v", out[0])
	}
}
