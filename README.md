# LLWSM — Linux Light Weight System Monitor

`sysmon` is a small, self-hosted Linux system monitor: one static Go binary,
a minimal dark-mode dashboard, and a clean JSON API. No databases, no cloud,
no telemetry, no plugins. It answers one question:

> Is my machine healthy, what is it doing right now, and are any important
> services, storage, or network components having problems?

Designed for low-resource devices (ARM64 phones, Raspberry Pi, small home
servers), but runs anywhere on Linux: x86_64, ARM64, ARMv7.

## Features

- CPU: overall + per-core usage, core count, load average, model, frequency
- Memory: total / used / available (Linux available-memory semantics),
  buffers, cached, swap usage and percent
- Uptime and boot time
- Storage: dynamically discovered mounts (no hardcoded paths), capacity,
  used, available, percent; pseudo-filesystems filtered out
- Network: auto-discovered interfaces (no assumed names), operational state,
  IPv4/IPv6, live RX/TX rates and totals
- Temperature: `/sys/class/thermal` when available, `N/A` otherwise
- Docker containers: optional, via the Unix socket; works without Docker
- Service monitoring: simple process-name match and HTTP checks, configured
  in YAML; no systemd assumption
- Health status: HEALTHY / WARNING / CRITICAL with configurable thresholds
  and an explanation of what triggered each warning
- Real-time dashboard: Server-Sent Events with automatic HTTP-polling
  fallback; 1-2 s configurable refresh
- Single static binary, embedded web UI, small idle footprint

## Quick start

```bash
# build (Go 1.24+ required)
go build -o sysmon ./cmd/sysmon

# run — binds to 127.0.0.1:8090 by default
./sysmon
```

Open http://127.0.0.1:8090 in a browser.

With Docker:

```bash
docker compose up -d
```

then open http://127.0.0.1:8090 (or the machine's LAN address).

## Configuration

A small YAML file is optional. Locations checked, in order:

1. `--config /path/to/config.yaml`
2. `$SYSMON_CONFIG`
3. `/etc/sysmon/config.yaml`
4. `./config.yaml`

With no config file at all, sensible defaults are used:

| Setting | Default |
|---|---|
| listen | `127.0.0.1:8090` |
| refresh interval | `2s` |
| docker socket | `/var/run/docker.sock` |
| ignored filesystems | proc, sysfs, tmpfs, devtmpfs, ... (see `config.example.yaml`) |
| health thresholds | CPU 80/95 %, memory 85/95 %, swap 50/80 %, storage 85/95 % |

Flags:

```
--config PATH   path to a YAML config file
--listen ADDR   host:port to listen on (overrides config)
--root PATH     host filesystem root ("/host" in the documented Docker setup)
--version       print version and exit
```

See `config.example.yaml` for a fully commented example, including service
monitoring:

```yaml
services:
  - name: Pi-hole
    type: process
    match: pihole
  - name: Web
    type: http
    url: http://127.0.0.1:8080
```

## Security

- Binds to `127.0.0.1` unless explicitly changed with `--listen` or config.
- **No authentication is implemented.** If you expose it beyond localhost
  (for example `--listen 0.0.0.0:8090`), do it only on a trusted LAN.
- The web UI and API never execute shell commands and never accept
  user-controlled commands. All endpoints are read-only.
- Docker integration only calls the read-only containers list and stats
  endpoints over the socket; no container management is exposed.
- All host mounts for the Docker setup are read-only.

## API

All endpoints return JSON. Schemas are stable; new fields may be added.

| Endpoint | Description |
|---|---|
| `GET /api/status` | Aggregated dashboard payload (system + cpu + memory + storage + network + sensors + containers + services + health) |
| `GET /api/system` | Hostname, OS, arch, kernel, model, boot time, uptime |
| `GET /api/cpu` | Usage, cores, per-core usage, load, model, frequency |
| `GET /api/memory` | Total/used/available/buffers/cached, swap, percents |
| `GET /api/storage` | Discovered mounted filesystems with capacity |
| `GET /api/network` | Interfaces: state, addresses, RX/TX rates and totals |
| `GET /api/containers` | Docker availability and container list |
| `GET /api/services` | Configured service checks with up/down status |
| `GET /api/health` | Overall status (HEALTHY/WARNING/CRITICAL) with reasons |
| `GET /api/version` | Version, commit, build date |
| `GET /api/stream` | Server-Sent Events: pushes the `/api/status` payload on every refresh |

`/api/status` example (abridged):

```json
{
  "hostname": "qcom-msm89x7",
  "model": "Xiaomi Redmi 4A",
  "kernel": "6.1.0-postmarketos",
  "arch": "arm64",
  "uptime": 319200,
  "status": "HEALTHY",
  "reasons": [],
  "cpu": { "usage": 23.4, "cores": 4, "per_core": [20, 25, 26, 22], "load": [0.42, 0.38, 0.31] },
  "memory": { "total": 1932735283, "used": 657000000, "available": 900000000, "percent": 35.0, "swap_used": 39400000 },
  "storage": [ { "mount": "/", "fstype": "ext4", "total": 16000000000, "used": 8200000000, "available": 7800000000, "percent": 51.2, "mounted": true } ],
  "network": [ { "name": "wlan0", "state": "up", "ipv4": ["192.168.1.60"], "rx_rate": 1200000, "tx_rate": 120000 } ],
  "sensors": [ { "name": "CPU", "temperature": 48 } ],
  "containers": { "available": true, "containers": [] },
  "services": [ { "name": "Pi-hole", "type": "process", "status": "up" } ]
}
```

When a metric is unavailable (no sensors, no Docker, ...), the field is
`N/A`, an empty array, or `available: false` — the application never fails
because of missing kernel interfaces.

Real-time updates use `/api/stream` (SSE). If the browser cannot open the
stream, the dashboard falls back to polling `/api/status` at the configured
interval (default 2 s).

## Deployment

`sysmon` deploys as a single static binary with zero runtime dependencies —
no interpreters, no shared libraries, no data directories. Pick one path.

### Path 1 — Native binary + systemd (recommended)

Works on any systemd Linux (Debian, Ubuntu, Fedora, Arch, Raspberry Pi OS):

```bash
# 1. Build a stripped static binary for the target arch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o sysmon ./cmd/sysmon
#    GOARCH=amd64 for x86_64, GOARCH=arm GOARM=7 for 32-bit ARM

# 2. Transfer and install
scp sysmon user@target:
sudo install -m 0755 sysmon /usr/local/bin/sysmon

# 3. Optional: customize config (works without one too)
sudo mkdir -p /etc/sysmon
sudo cp config.example.yaml /etc/sysmon/config.yaml

# 4. Install the service
sudo cp deploy/systemd/sysmon.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sysmon

# 5. Check
systemctl status sysmon
journalctl -u sysmon -f        # live log
curl -s localhost:8090/api/health
```

### Path 2 — Docker

Works anywhere with a Docker daemon; no Go toolchain needed on the host:

```bash
git clone <repo> && cd llwsm
docker compose up -d
curl -s localhost:8090/api/health
```

The compose file mounts the host root read-only at `/host` and runs
`sysmon --root /host`. This is required: `/proc` inside the container
reflects the container itself, not the host. To also monitor host
containers, add the optional Docker socket mount documented in
`deploy/docker/README.md`.

Build an image for another architecture:

```bash
docker buildx build --platform linux/arm64 -t llwsm/sysmon:arm64 .
```

### Path 3 — No systemd (OpenRC, runit, s6, phones, ...)

Run the binary from whatever supervisor your system has. No systemd is
assumed or used by sysmon:

```bash
# OpenRC:  /etc/init.d/sysmon (launch: exec /usr/local/bin/sysmon)
# runit:   /etc/sv/sysmon/run -> #!/bin/sh \n exec /usr/local/bin/sysmon
# s6:      s6-svcstart after s6-rc install
# cron:    @reboot /usr/local/bin/sysmon --config /etc/sysmon/config.yaml
```

### Deploying to a target device (e.g. a phone)

1. Cross-compile on your dev machine (pure Go, no cgo — trivial):

   ```bash
   GOOS=linux GOARCH=arm64 go build -o dist/sysmon-linux-arm64 ./cmd/sysmon
   # arch variants: GOARCH=amd64 -> dist/sysmon-linux-amd64,
   #                GOARCH=arm GOARM=7 -> dist/sysmon-linux-armv7
   ```

2. Copy the binary over (`scp`, a USB cable, or `adb push` on postmarketOS).

3. Run in the foreground first to sanity-check:

   ```bash
   ./sysmon --listen 0.0.0.0:8090
   ```

4. Point a browser on the LAN at `http://<device-ip>:8090`.

### LAN access

Default is `127.0.0.1:8090` (localhost only). To serve the LAN:

- config: `server.host: 0.0.0.0` (see `config.example.yaml`), or
- flag: `./sysmon --listen 0.0.0.0:8090`

There is **no authentication** — only do this on a trusted network.

### First-run validation checklist

```bash
curl -s localhost:8090/api/health   # expect {"status":"HEALTHY","reasons":[]}
curl -s localhost:8090/api/status   # full dashboard payload
curl -sN localhost:8090/api/stream  # live SSE stream, one event per refresh
```

Then open `http://<host>:8090/` in a browser and confirm the dashboard
updates every couple of seconds without a refresh.

## Health status

`GET /api/health` returns:

```json
{
  "status": "WARNING",
  "reasons": [
    { "level": "WARNING", "source": "storage",
      "message": "filesystem \"/\" is 88% full (warning threshold 85%)" }
  ]
}
```

Thresholds are configurable (see `config.example.yaml`). A downed configured
service produces a warning reason. A `HEALTHY` result always means no check
fired, and every non-healthy result lists exactly what triggered it.

## Resource usage

- Single goroutine refresh loop: all collectors run once per interval
  (default 2 s) and serve cached snapshots to any number of API clients.
- No persistent data, no history, no background retention.
- Idle footprint is dominated by the Go runtime; well under the ~50 MB
  target on an ARM64 device with a few monitors.
- CPU/network rates are deltas between samples — no extra high-frequency
  reads.

## Project layout

```
cmd/sysmon/          main entry point (flags, server, refresh loop)
internal/collector/  cpu, memory, storage, network, sensors, docker, services, system
internal/api/        REST + SSE handlers, status payload
internal/config/     YAML config with defaults
internal/health/     threshold evaluation
internal/host/       snapshot aggregation (clean collector/API boundary)
internal/version/    version metadata
web/                 embedded dashboard (templates + static css/js)
deploy/              systemd unit, docker notes
```

The collector layer is fully decoupled from the API layer through the
`host.Snapshot` type, which is what makes a future remote-agent mode
(a central dashboard querying remote `sysmon` instances over their HTTP API)
a straightforward extension.

## Limitations / not in the first version

- squashfs mounts (snap images) are ignored by default because read-only
  compressed images always report themselves as full; they are listed in
  `config.example.yaml` if you want to display them.
- No per-container restart count (not exposed by the lightweight stats
  endpoint used here).
- No historical data, no alerting, no authentication, no user accounts.
- In Docker, host network monitoring requires `network_mode: host`.
- Docker stats are sampled for at most 32 running containers per refresh.

## Testing

```bash
go test ./...
```

Unit tests cover the CPU, memory, storage, network, sensor, Docker, and
service collectors, the JSON API, configuration parsing, and health
calculation. They use fake `/proc` and `/sys` trees and in-process HTTP
servers — no Docker, no root, no real system state required.
