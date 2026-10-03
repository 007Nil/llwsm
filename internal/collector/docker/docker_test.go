package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

const containersList = `[
	{"Id":"abc123","Names":["/pihole"],"Image":"pihole:latest","State":"running","Status":"Up 2 hours"},
	{"Id":"def456","Names":["test"],"Image":"busybox","State":"exited","Status":"Exited (0) 3 days ago"}
]`

func TestCollectUnavailable(t *testing.T) {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, errors.New("dial unix /nonexistent.sock: connection refused")
		},
	}
	c := newCollector(&http.Client{Transport: tr}, "http://docker")
	info := c.Collect()
	if info.Available {
		t.Error("expected docker to be unavailable")
	}
	if info.Error == "" {
		t.Error("expected an error message")
	}
}

func TestCollect(t *testing.T) {
	statsCalls := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("all") != "true" {
			t.Errorf("expected all=true query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, containersList)
	})
	mux.HandleFunc("/containers/abc123/stats", func(w http.ResponseWriter, r *http.Request) {
		statsCalls["abc123"]++
		if statsCalls["abc123"] == 1 {
			io.WriteString(w, `{"cpu_stats":{"cpu_usage":{"total_usage":1000000},"system_cpu_usage":390000000,"online_cpus":4},"precpu_stats":{"cpu_usage":{"total_usage":500000},"system_cpu_usage":380000000},"memory_stats":{"usage":31000000,"limit":1000000000},"networks":{"eth0":{"rx_bytes":100,"tx_bytes":50}}}`)
		} else {
			io.WriteString(w, `{"cpu_stats":{"cpu_usage":{"total_usage":2000000},"system_cpu_usage":400000000,"online_cpus":4},"precpu_stats":{"cpu_usage":{"total_usage":1000000},"system_cpu_usage":390000000},"memory_stats":{"usage":32000000,"limit":1000000000},"networks":{"eth0":{"rx_bytes":200,"tx_bytes":100}}}`)
		}
	})
	mux.HandleFunc("/containers/def456/stats", func(w http.ResponseWriter, r *http.Request) {
		t.Error("stats endpoint should not be called for stopped containers")
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newCollector(&http.Client{Transport: &http.Transport{}}, srv.URL)

	info := c.Collect()
	if !info.Available {
		t.Fatalf("expected docker available, error: %s", info.Error)
	}
	if len(info.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(info.Containers))
	}
	pihole := info.Containers[0]
	if pihole.Name != "pihole" || pihole.State != "running" || pihole.Status != "Up 2 hours" {
		t.Errorf("pihole = %+v", pihole)
	}
	if pihole.MemoryBytes != 31000000 || pihole.MemoryLimit != 1000000000 {
		t.Errorf("memory = %d / %d", pihole.MemoryBytes, pihole.MemoryLimit)
	}
	if pihole.RXBytes != 100 || pihole.TXBytes != 50 {
		t.Errorf("network = %d / %d", pihole.RXBytes, pihole.TXBytes)
	}
	if pihole.CPUPercent != 0 {
		t.Errorf("first sample cpu = %f, want 0", pihole.CPUPercent)
	}
	stopped := info.Containers[1]
	if stopped.Name != "test" || stopped.State != "exited" {
		t.Errorf("stopped = %+v", stopped)
	}

	info = c.Collect()
	pihole = info.Containers[0]
	if pihole.CPUPercent != 40 {
		t.Errorf("second sample cpu = %f, want 40", pihole.CPUPercent)
	}
	if pihole.RXBytes != 200 {
		t.Errorf("rx = %d", pihole.RXBytes)
	}
}

func TestCollectListError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newCollector(&http.Client{Transport: &http.Transport{}}, srv.URL)
	info := c.Collect()
	if info.Available {
		t.Error("expected unavailable on list error")
	}
	if info.Error == "" {
		t.Error("expected error message")
	}
}

func TestContainerName(t *testing.T) {
	if n := containerName([]string{"/pihole"}); n != "pihole" {
		t.Errorf("name = %q", n)
	}
	if n := containerName(nil); n != "unnamed" {
		t.Errorf("name = %q", n)
	}
}
