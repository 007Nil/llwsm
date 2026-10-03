package sensors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollect(t *testing.T) {
	root := t.TempDir()
	zone := filepath.Join(root, "sys", "class", "thermal", "thermal_zone0")
	if err := os.MkdirAll(zone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zone, "temp"), []byte("45000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zone, "type"), []byte("xchg-thermal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "sys", "class", "thermal", "thermal_zone1")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "temp"), []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(root)
	out := c.Collect()
	if len(out) != 1 {
		t.Fatalf("expected 1 sensor, got %d: %+v", len(out), out)
	}
	if out[0].Name != "xchg-thermal" || out[0].Temperature != 45.0 {
		t.Errorf("sensor = %+v", out[0])
	}
}

func TestCollectNoThermalDir(t *testing.T) {
	c := New(t.TempDir())
	if out := c.Collect(); len(out) != 0 {
		t.Errorf("expected no sensors, got %d", len(out))
	}
}
