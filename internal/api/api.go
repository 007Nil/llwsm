package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"llwsm/internal/collector/cpu"
	"llwsm/internal/collector/docker"
	"llwsm/internal/collector/memory"
	"llwsm/internal/collector/network"
	"llwsm/internal/collector/sensors"
	"llwsm/internal/collector/services"
	"llwsm/internal/collector/storage"
	"llwsm/internal/config"
	"llwsm/internal/health"
	"llwsm/internal/host"
	"llwsm/internal/version"
	"llwsm/web"
)

type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[chan []byte]struct{}{}}
}

func (h *Hub) Register() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unregister(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
	}
	h.mu.Unlock()
}

// Broadcast sends data to every client, dropping slow ones.
func (h *Hub) Broadcast(data []byte) {
	h.mu.Lock()
	var dead []chan []byte
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			delete(h.clients, ch)
			dead = append(dead, ch)
		}
	}
	h.mu.Unlock()
	for _, ch := range dead {
		close(ch)
	}
}

type statusPayload struct {
	Hostname   string               `json:"hostname"`
	Model      string               `json:"model"`
	Kernel     string               `json:"kernel"`
	OS         string               `json:"os"`
	Arch       string               `json:"arch"`
	BootTime   time.Time            `json:"boot_time"`
	Uptime     int64                `json:"uptime"`
	Updated    time.Time            `json:"updated"`
	Status     string               `json:"status"`
	Reasons    []health.Reason      `json:"reasons"`
	CPU        cpu.Info             `json:"cpu"`
	Memory     memory.Info          `json:"memory"`
	Storage    []storage.Filesystem `json:"storage"`
	Network    []network.Interface  `json:"network"`
	Sensors    []sensors.Sensor     `json:"sensors"`
	Containers docker.Info          `json:"containers"`
	Services   []services.Service   `json:"services"`
}

type Handler struct {
	host *host.Host
	cfg  *config.Config
	hub  *Hub
	tmpl *template.Template
}

func New(h *host.Host, cfg *config.Config) *Handler {
	a := &Handler{host: h, cfg: cfg, hub: NewHub()}
	if tmpl, err := template.New("index.html").ParseFS(web.Templates, "templates/index.html"); err == nil {
		a.tmpl = tmpl
	}
	return a
}

func thresholdsFromConfig(c *config.Config) health.Thresholds {
	return health.Thresholds{
		CPUWarning:      c.Health.CPU.Warning,
		CPUCritical:     c.Health.CPU.Critical,
		MemoryWarning:   c.Health.Memory.Warning,
		MemoryCritical:  c.Health.Memory.Critical,
		SwapWarning:     c.Health.Swap.Warning,
		SwapCritical:    c.Health.Swap.Critical,
		StorageWarning:  c.Health.Storage.Warning,
		StorageCritical: c.Health.Storage.Critical,
	}
}

func (a *Handler) statusPayload() statusPayload {
	snap := a.host.Snapshot()
	res := health.Evaluate(&snap, thresholdsFromConfig(a.cfg))
	return statusPayload{
		Hostname:   snap.System.Hostname,
		Model:      snap.System.Model,
		Kernel:     snap.System.Kernel,
		OS:         snap.System.OS,
		Arch:       snap.System.Arch,
		BootTime:   snap.System.BootTime,
		Uptime:     snap.System.UptimeS,
		Updated:    snap.Time,
		Status:     res.Status,
		Reasons:    res.Reasons,
		CPU:        snap.CPU,
		Memory:     snap.Memory,
		Storage:    snap.Storage,
		Network:    snap.Network,
		Sensors:    snap.Sensors,
		Containers: snap.Containers,
		Services:   snap.Services,
	}
}

// StatusJSON returns the current dashboard payload as JSON.
func (a *Handler) StatusJSON() []byte {
	b, err := json.Marshal(a.statusPayload())
	if err != nil {
		b = []byte(`{"status":"HEALTHY"}`)
	}
	return b
}

// Publish pushes the current status to all stream clients.
func (a *Handler) Publish() {
	a.hub.Broadcast(a.StatusJSON())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (a *Handler) handleJSON(f func(host.Snapshot) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := a.host.Snapshot()
		writeJSON(w, f(snap))
	}
}

func (a *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.statusPayload())
}

func (a *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap := a.host.Snapshot()
	writeJSON(w, health.Evaluate(&snap, thresholdsFromConfig(a.cfg)))
}

func (a *Handler) handleSystem(w http.ResponseWriter, r *http.Request) {
	snap := a.host.Snapshot()
	writeJSON(w, snap.System)
}

func (a *Handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, version.Get())
}

func (a *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := a.hub.Register()
	defer a.hub.Unregister(ch)

	writeEvent := func(data []byte) {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
	}

	writeEvent(a.StatusJSON())

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(data)
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Version":         version.Version,
		"IntervalSeconds": int(a.cfg.Interval().Seconds()),
	}
	if a.tmpl == nil {
		http.Error(w, "dashboard template unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.Execute(w, data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (a *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/system", a.handleSystem)
	mux.HandleFunc("GET /api/cpu", a.handleJSON(func(s host.Snapshot) any { return s.CPU }))
	mux.HandleFunc("GET /api/memory", a.handleJSON(func(s host.Snapshot) any { return s.Memory }))
	mux.HandleFunc("GET /api/storage", a.handleJSON(func(s host.Snapshot) any { return s.Storage }))
	mux.HandleFunc("GET /api/network", a.handleJSON(func(s host.Snapshot) any { return s.Network }))
	mux.HandleFunc("GET /api/containers", a.handleJSON(func(s host.Snapshot) any { return s.Containers }))
	mux.HandleFunc("GET /api/services", a.handleJSON(func(s host.Snapshot) any { return s.Services }))
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/version", a.handleVersion)
	mux.HandleFunc("GET /api/stream", a.handleStream)
	mux.HandleFunc("GET /", a.handleIndex)
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		staticFS = web.Static
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	return mux
}
