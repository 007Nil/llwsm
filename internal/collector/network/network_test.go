package network

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestCollectWith(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "eth0", "operstate"), "up\n")
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "eth0", "statistics", "rx_bytes"), "1000\n")
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "eth0", "statistics", "tx_bytes"), "500\n")
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "wlan0", "operstate"), "down\n")

	ifaces := []net.Interface{
		{Name: "lo", Flags: net.FlagLoopback | net.FlagUp},
		{Name: "eth0", Flags: net.FlagUp | net.FlagRunning},
		{Name: "wlan0", Flags: 0},
	}

	c := New(root)
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c.prevTime = t0
	c.prev = map[string][2]uint64{"eth0": {900, 400}}
	c.now = func() time.Time { return t0.Add(10 * time.Second) }

	out := c.CollectWith(ifaces)
	if len(out) != 2 {
		t.Fatalf("expected 2 interfaces (loopback skipped), got %d", len(out))
	}
	eth := out[0]
	if eth.Name != "eth0" || eth.State != "up" || !eth.Up {
		t.Errorf("eth0 = %+v", eth)
	}
	if eth.RXRate != 10 || eth.TXRate != 10 {
		t.Errorf("rates = %f %f, want 10 10", eth.RXRate, eth.TXRate)
	}
	if eth.RXTotal != 1000 || eth.TXTotal != 500 {
		t.Errorf("totals = %d %d", eth.RXTotal, eth.TXTotal)
	}
	wlan := out[1]
	if wlan.State != "down" || wlan.Up {
		t.Errorf("wlan0 = %+v", wlan)
	}
	if len(wlan.IPv4) != 0 || len(wlan.IPv6) != 0 {
		t.Errorf("wlan0 addrs = %v %v", wlan.IPv4, wlan.IPv6)
	}
}

func TestFirstSampleHasNoRates(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "eth0", "statistics", "rx_bytes"), "1000\n")
	mustWrite(t, filepath.Join(root, "sys", "class", "net", "eth0", "statistics", "tx_bytes"), "500\n")

	c := New(root)
	out := c.CollectWith([]net.Interface{
		{Name: "eth0", Flags: net.FlagUp},
	})
	if len(out) != 1 {
		t.Fatalf("got %d interfaces", len(out))
	}
	if out[0].RXRate != 0 || out[0].TXRate != 0 {
		t.Errorf("first sample rates = %f %f, want 0 0", out[0].RXRate, out[0].TXRate)
	}
	if out[0].RXTotal != 1000 || out[0].TXTotal != 500 {
		t.Errorf("totals = %d %d", out[0].RXTotal, out[0].TXTotal)
	}
}

func TestSplitAddrs(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.60"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("fd00::9"), Mask: net.CIDRMask(64, 128)},
	}
	v4, v6 := splitAddrs(addrs)
	if len(v4) != 2 || v4[0] != "192.168.1.60" || v4[1] != "10.0.0.5" {
		t.Errorf("v4 = %v", v4)
	}
	if len(v6) != 2 || v6[0] != "fe80::1" || v6[1] != "fd00::9" {
		t.Errorf("v6 = %v", v6)
	}
	if a, b := splitAddrs(nil); len(a) != 0 || len(b) != 0 {
		t.Errorf("empty addrs = %v %v", a, b)
	}
}

func TestCounterReset(t *testing.T) {
	if r := rate(100, 50, 1); r != 0 {
		t.Errorf("rate = %f, want 0 after counter reset", r)
	}
	if r := rate(50, 150, 0); r != 0 {
		t.Errorf("rate = %f, want 0 for zero delta time", r)
	}
}
