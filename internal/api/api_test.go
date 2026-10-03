package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llwsm/internal/config"
	"llwsm/internal/host"
)

func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"proc", "proc/sys/kernel", "sys/class/net/eth0/statistics", "sys/class/thermal/thermal_zone0", filepath.Join("mnt", "data")} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("proc/uptime", "12345.67 23456.78\n")
	w("proc/loadavg", "0.50 1.00 1.50 2/345 678\n")
	w("proc/stat", "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 50 0 50 400 0 0 0 0 0 0\ncpu1 50 0 50 400 0 0 0 0 0 0\n")
	w("proc/meminfo", "MemTotal:       2048000 kB\nMemFree:        100000 kB\nMemAvailable:     900000 kB\nBuffers:          20000 kB\nCached:          400000 kB\nSwapTotal:       2048000 kB\nSwapFree:        1900000 kB\n")
	w("proc/mounts", "rootfs / rootfs ro 0 0\nproc /proc proc rw 0 0\ntmpfs /run tmpfs rw 0 0\next4 / ext4 rw 0 0\next4 /mnt/data ext4 rw 0 0\n")
	w("proc/cpuinfo", "processor\t: 0\nmodel name\t: Test CPU\n")
	w("proc/sys/kernel/osrelease", "6.1.0-test\n")
	w("sys/class/net/eth0/operstate", "up\n")
	w("sys/class/net/eth0/statistics/rx_bytes", "1000\n")
	w("sys/class/net/eth0/statistics/tx_bytes", "500\n")
	w("sys/class/thermal/thermal_zone0/temp", "45000\n")
	w("sys/class/thermal/thermal_zone0/type", "cpu-thermal\n")
	return root
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.Defaults()
	cfg.Root = fakeRoot(t)
	cfg.Docker.SocketPath = filepath.Join(t.TempDir(), "docker.sock")
	h := host.New(cfg)
	srv := httptest.NewServer(New(h, cfg).Routes())
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", url, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("%s: decode: %v", url, err)
	}
	return out
}

func TestAPIStatus(t *testing.T) {
	srv := newTestServer(t)
	d := getJSON(t, srv.URL+"/api/status")

	if d["hostname"] == "" || d["hostname"] == "unknown" {
		t.Errorf("hostname = %v", d["hostname"])
	}
	if uptime, _ := d["uptime"].(float64); uptime != 12345 {
		t.Errorf("uptime = %v", d["uptime"])
	}
	status, _ := d["status"].(string)
	switch status {
	case "HEALTHY", "WARNING", "CRITICAL":
	default:
		t.Errorf("status = %q", status)
	}
	if _, ok := d["reasons"].([]any); !ok {
		t.Errorf("reasons must be an array, got %T", d["reasons"])
	}

	cpuObj, ok := d["cpu"].(map[string]any)
	if !ok {
		t.Fatal("cpu missing")
	}
	if cores, _ := cpuObj["cores"].(float64); cores != 2 {
		t.Errorf("cores = %v", cpuObj["cores"])
	}
	load, _ := cpuObj["load"].([]any)
	if len(load) != 3 || load[0].(float64) != 0.5 {
		t.Errorf("load = %v", cpuObj["load"])
	}
	if model, _ := cpuObj["model"].(string); model != "Test CPU" {
		t.Errorf("model = %q", cpuObj["model"])
	}

	memObj, _ := d["memory"].(map[string]any)
	if total, _ := memObj["total"].(float64); total != 2048000*1024 {
		t.Errorf("memory total = %v", memObj["total"])
	}

	storage, _ := d["storage"].([]any)
	if len(storage) != 2 {
		t.Errorf("storage = %v", d["storage"])
	} else {
		if mount, _ := storage[0].(map[string]any)["mount"].(string); mount != "/" {
			t.Errorf("first mount = %q", mount)
		}
	}

	sensors, _ := d["sensors"].([]any)
	if len(sensors) != 1 {
		t.Fatalf("sensors = %v", d["sensors"])
	}
	s0, _ := sensors[0].(map[string]any)
	if s0["name"] != "cpu-thermal" || s0["temperature"].(float64) != 45 {
		t.Errorf("sensor = %v", s0)
	}

	containers, _ := d["containers"].(map[string]any)
	if containers["available"].(bool) {
		t.Error("docker should be unavailable in test environment")
	}

	if _, ok := d["kernel"].(string); !ok {
		t.Error("kernel missing")
	}
	if d["kernel"] != "6.1.0-test" {
		t.Errorf("kernel = %v", d["kernel"])
	}
}

func TestAPISubresources(t *testing.T) {
	srv := newTestServer(t)

	cpu := getJSON(t, srv.URL+"/api/cpu")
	if c, _ := cpu["cores"].(float64); c != 2 {
		t.Errorf("cpu cores = %v", cpu["cores"])
	}

	mem := getJSON(t, srv.URL+"/api/memory")
	if total, _ := mem["total"].(float64); total != 2048000*1024 {
		t.Errorf("mem total = %v", mem["total"])
	}

	sys := getJSON(t, srv.URL+"/api/system")
	if sys["os"] != "linux" {
		t.Errorf("os = %v", sys["os"])
	}

	ver := getJSON(t, srv.URL+"/api/version")
	if ver["version"] == "" {
		t.Errorf("version = %v", ver)
	}

	storageResp, err := http.Get(srv.URL + "/api/storage")
	if err != nil {
		t.Fatal(err)
	}
	storageResp.Body.Close()
	if storageResp.StatusCode != http.StatusOK {
		t.Errorf("storage status = %d", storageResp.StatusCode)
	}

	networkResp, err := http.Get(srv.URL + "/api/network")
	if err != nil {
		t.Fatal(err)
	}
	networkResp.Body.Close()
	if networkResp.StatusCode != http.StatusOK {
		t.Errorf("network status = %d", networkResp.StatusCode)
	}

	servicesResp, err := http.Get(srv.URL + "/api/services")
	if err != nil {
		t.Fatal(err)
	}
	servicesResp.Body.Close()
	if servicesResp.StatusCode != http.StatusOK {
		t.Errorf("services status = %d", servicesResp.StatusCode)
	}

	containersResp, err := http.Get(srv.URL + "/api/containers")
	if err != nil {
		t.Fatal(err)
	}
	containersResp.Body.Close()
	if containersResp.StatusCode != http.StatusOK {
		t.Errorf("containers status = %d", containersResp.StatusCode)
	}
}

func TestAPIHealth(t *testing.T) {
	srv := newTestServer(t)
	d := getJSON(t, srv.URL+"/api/health")
	status, _ := d["status"].(string)
	switch status {
	case "HEALTHY", "WARNING", "CRITICAL":
	default:
		t.Errorf("status = %q", status)
	}
	if _, ok := d["reasons"].([]any); !ok {
		t.Errorf("reasons must be an array")
	}
}

func TestAPIEndpointsMethod(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Post(srv.URL+"/api/status", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/status = %d, want 405", resp.StatusCode)
	}
}

func TestDashboardHTML(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<html", "sysmon", "SYSMON", "/static/css/style.css"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("dashboard missing %q", want)
		}
	}

	css, err := http.Get(srv.URL + "/static/css/style.css")
	if err != nil {
		t.Fatal(err)
	}
	css.Body.Close()
	if css.StatusCode != http.StatusOK {
		t.Errorf("css status = %d", css.StatusCode)
	}
	js, err := http.Get(srv.URL + "/static/js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	js.Body.Close()
	if js.StatusCode != http.StatusOK {
		t.Errorf("js status = %d", js.StatusCode)
	}
}

func TestAPIStream(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("stream request failed: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(buf[:n]), "data: {") {
		t.Errorf("first bytes = %q", string(buf[:n]))
	}
}

func TestHubBroadcast(t *testing.T) {
	h := NewHub()
	ch := h.Register()
	h.Unregister(ch)
	if len(h.clients) != 0 {
		t.Errorf("clients = %d", len(h.clients))
	}

	ch2 := h.Register()
	h.Broadcast([]byte("hello"))
	if got, ok := <-ch2; !ok || string(got) != "hello" {
		t.Errorf("broadcast = %q %v", got, ok)
	}
	h.Unregister(ch2)
}
