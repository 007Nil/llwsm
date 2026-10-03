# Build: Lightweight Cross-Platform Linux Monitoring System 

LLWSM
Linux Light Weight System Monitor

## Objective

Build a lightweight, self-hosted Linux system monitoring application designed for low-resource machines such as ARM64 phones, Raspberry Pis, small servers, and home lab devices.

The primary target is a Redmi 4A running postmarketOS Linux with approximately 1.8 GB RAM, but the application **must not contain Redmi-specific assumptions**.

The same application should work on common Linux distributions and architectures including:

* x86_64
* ARM64 / aarch64
* ARMv7 where practical

Target operating systems include:

* postmarketOS / Alpine
* Debian
* Ubuntu
* Fedora
* Arch
* Raspberry Pi OS
* other standard Linux systems

The application should be completely self-hosted. No cloud service, account, telemetry, or external dependency should be required.

---

# Product Philosophy

Do NOT try to replicate Netdata, Grafana, or Prometheus.

The goal is a **small, attractive, intelligent system-status dashboard**.

The dashboard should answer:

> "Is my machine healthy, what is it doing right now, and are any important services/storage/network components having problems?"

It should be useful at a glance.

Avoid excessive charts, metrics, configuration screens, databases, plugins, and unnecessary complexity.

---

# Recommended Architecture

Use:

* **Go** for the backend/agent
* Lightweight HTTP server
* HTML/CSS/JavaScript frontend
* Prefer vanilla JS and CSS unless a frontend framework provides a clear benefit
* No Node.js runtime required in production
* No Python runtime required in production
* No database for basic operation

The resulting application should ideally be deployable as a **single static Go binary**.

Example:

```text
sysmon
```

Running:

```bash
./sysmon
```

should start the monitoring server and web UI.

---

# Operating Modes

Design the architecture to support two modes.

## 1. Standalone mode

The machine runs both collector and dashboard:

```text
Linux machine
    |
    +-- sysmon
          |
          +-- collectors
          +-- API
          +-- Web UI
```

This is the primary mode for the Redmi 4A.

Example:

```text
http://192.168.1.60:8090
```

## 2. Agent / Central Dashboard mode

Design the internal architecture so that a future version can separate:

```text
                 Central Dashboard
                       |
          +------------+------------+
          |            |            |
        Agent        Agent        Agent
        Redmi        Laptop       Server
```

Do not implement a complicated distributed system unless necessary for the MVP.

However, keep the collector/API boundaries clean enough that remote agents can be added later.

---

# Core System Metrics

The application must collect the following where the Linux system exposes them.

## CPU

Display:

* overall CPU utilization
* per-core utilization
* number of CPU cores
* load average
* CPU model/name if available
* CPU frequency if available

Example:

```text
CPU
23%
4 cores
Load: 0.42 0.38 0.31
```

Do not assume `/proc/cpuinfo` has a particular format.

---

# Memory

Display:

* total RAM
* used RAM
* available RAM
* cached/buffered memory where useful
* percentage used
* total swap
* used swap
* swap percentage

Example:

```text
MEMORY

628 MB / 1.8 GB
35%

Swap
38 MB / 2.7 GB
1%
```

Use Linux's available-memory semantics rather than simply:

```text
total - free
```

where appropriate.

---

# Uptime

Display:

* human-readable uptime
* optionally boot time

Example:

```text
Uptime
3d 12h 24m
```

---

# Storage

Discover mounted filesystems dynamically.

Do NOT hardcode:

```text
/mnt/music
/mnt/media
```

Those are examples from the development machine only.

For every relevant mounted filesystem display:

* mount point
* filesystem type
* total capacity
* used capacity
* available capacity
* percentage used
* mounted/unmounted status

Example:

```text
STORAGE

/
8.2 GB / 16 GB
51%

/mnt/music
312 GB / 466 GB
67%

/mnt/media
184 GB / 466 GB
39%
```

Avoid displaying pseudo-filesystems such as:

* `/proc`
* `/sys`
* `/dev`
* `/run`

unless explicitly configured.

Do not scan the entire filesystem just to discover mounts.

Use appropriate Linux system calls such as `statfs/statvfs`.

---

# Network

Automatically discover network interfaces.

For each useful interface display:

* interface name
* operational state
* IPv4 address
* IPv6 address if available
* RX rate
* TX rate
* optionally total RX/TX

Example:

```text
NETWORK

wlan0
192.168.1.60

↓ 1.2 MB/s
↑ 120 KB/s

eth0
DOWN
```

Do not assume interfaces are named:

```text
eth0
wlan0
```

Modern Linux systems may use names such as:

```text
enp3s0
ens18
wlp2s0
usb0
```

---

# Temperature / Sensors

Discover available thermal zones and sensors.

On systems exposing:

```text
/sys/class/thermal/
```

use them.

Display useful temperatures.

Example:

```text
TEMPERATURE

CPU       48°C
Battery   34°C
```

Sensor names vary significantly between devices.

Do not crash if sensors are unavailable.

Display:

```text
N/A
```

when appropriate.

---

# Docker Integration

Docker support is important but optional.

Detect whether Docker is available.

If Docker is available, collect:

* container name
* container ID
* running/stopped state
* CPU usage
* memory usage
* memory limit if available
* network RX/TX if available
* restart count where available
* uptime/status

Example:

```text
CONTAINERS

● pihole
  Running
  CPU 0.6%
  RAM 31 MB

● mopidy
  Running
  CPU 2.1%
  RAM 85 MB

○ test-container
  Stopped
```

Use the Docker Unix socket when available:

```text
/var/run/docker.sock
```

Make Docker integration optional.

The application must work perfectly on systems without Docker.

Never require Docker to run the monitoring application itself.

---

# Service Monitoring

Provide lightweight service/process health checks.

At minimum support detecting common Linux services/processes.

For systemd systems, optionally integrate with:

```bash
systemctl
```

For non-systemd systems, do not assume systemd exists.

The monitoring application must remain functional on:

* systemd
* OpenRC
* other Linux init systems

Allow users to define simple monitored services, for example:

```yaml
services:
  - name: Pi-hole
    type: process
    match: pihole

  - name: Mopidy
    type: process
    match: mopidy
```

For HTTP services optionally support:

```yaml
services:
  - name: Pi-hole
    type: http
    url: http://127.0.0.1:8080
```

Display:

```text
SERVICES

● Pi-hole       Running
● Mopidy        Running
● Docker        Running
```

Keep this feature simple.

---

# Health Status

Create a simple overall status calculation.

Possible states:

```text
HEALTHY
WARNING
CRITICAL
```

Do not attempt to create a complicated monitoring algorithm.

Examples:

Healthy:

```text
● ONLINE
```

Warning:

```text
● WARNING
```

Critical:

```text
● CRITICAL
```

Potential warning conditions:

* filesystem nearly full
* unusually high RAM usage
* swap usage increasing significantly
* service unavailable
* network interface unexpectedly down

Thresholds must be configurable.

Do NOT claim that a machine is "healthy" based on arbitrary metrics without explaining what triggered a warning.

---

# Dashboard UI

The UI should be clean, modern, responsive, and intentionally minimal.

Target:

* desktop browser
* mobile browser
* tablet

Prefer dark mode by default.

Example layout:

```text
┌──────────────────────────────────────────────────────┐
│ REDMI SERVER                              ● ONLINE   │
│ qcom-msm89x7 • uptime 3d 12h                         │
├────────────────────────┬─────────────────────────────┤
│ CPU                    │ MEMORY                      │
│                        │                             │
│       23%              │       628 MB / 1.8 GB       │
│    █████░░░░            │       █████░░░░             │
│                        │                             │
│ Load 0.42 0.38 0.31    │ Swap 38 MB / 2.7 GB         │
├────────────────────────┴─────────────────────────────┤
│ STORAGE                                              │
│                                                      │
│ /             51%  ███████████░░░  8.2 / 16 GB      │
│ /mnt/music    67%  █████████████░ 312 / 466 GB      │
│ /mnt/media    39%  ████████░░░░░░ 184 / 466 GB      │
├──────────────────────────────────────────────────────┤
│ NETWORK                                              │
│                                                      │
│ wlan0     192.168.1.60       ↓ 1.2 MB/s ↑ 120 KB/s  │
├──────────────────────────────────────────────────────┤
│ SERVICES                                             │
│                                                      │
│ ● Docker       ● Pi-hole       ● Mopidy             │
└──────────────────────────────────────────────────────┘
```

Do not fill the interface with dozens of graphs.

Graphs can be added later.

---

# Real-Time Updates

The dashboard should update without full page refresh.

Preferred:

```text
WebSocket
```

or lightweight:

```text
Server-Sent Events
```

Fallback:

```text
HTTP polling
```

Target update interval:

```text
1–2 seconds
```

Make this configurable.

Do not generate unnecessary traffic.

---

# Resource Requirements

This is extremely important.

The target machine is a Redmi 4A with approximately:

```text
CPU: 4 cores
RAM: ~1.8 GB
Swap: ~2.7 GB
ARM64
```

The monitoring application should be extremely lightweight.

Goals:

* minimal idle CPU
* ideally <50 MB RAM for the entire application
* no persistent database
* no background data retention
* no telemetry
* no cloud communication
* no unnecessary filesystem scanning
* no high-frequency expensive operations

Do not implement expensive monitoring operations every second.

Use cached data where appropriate.

---

# Security

Default behavior:

* bind to `127.0.0.1` unless explicitly configured otherwise

Allow:

```text
--listen 0.0.0.0:8090
```

for LAN access.

Do not expose Docker socket functionality through arbitrary user-controlled HTTP endpoints.

Never allow the web UI to execute arbitrary shell commands.

Never accept arbitrary commands from clients.

If authentication is not implemented in the MVP, clearly document that the application should only be exposed to trusted LAN networks.

---

# Configuration

Support a small configuration file.

Example:

```yaml
server:
  host: 0.0.0.0
  port: 8090

monitor:
  interval: 2s

storage:
  ignored_filesystems:
    - proc
    - sysfs
    - tmpfs
    - devtmpfs

services:
  - name: Pi-hole
    type: process
    match: pihole

  - name: Mopidy
    type: process
    match: mopidy
```

Don't make configuration unnecessarily complicated.

Sensible defaults should allow the application to start with zero configuration.

---

# API

Create a clean REST API.

At minimum:

```text
GET /api/status
GET /api/system
GET /api/cpu
GET /api/memory
GET /api/storage
GET /api/network
GET /api/containers
GET /api/services
GET /api/health
```

Example:

```json
{
  "hostname": "qcom-msm89x7",
  "uptime": 123456,
  "cpu": {
    "usage": 23.4,
    "cores": 4,
    "load": [0.42, 0.38, 0.31]
  },
  "memory": {
    "total": 1932735283,
    "used": 657000000,
    "available": 900000000,
    "swap_used": 39400000
  }
}
```

Use stable schemas.

---

# Docker

Provide a Dockerfile.

The image should be small.

Prefer a multi-stage build:

```text
Go build stage
        ↓
minimal runtime image
```

If practical, use:

```text
scratch
```

or:

```text
distroless
```

for the final image.

However, if runtime filesystem access requires utilities, do not sacrifice functionality merely to achieve an extremely small image.

The container must support host monitoring through explicitly documented mounts.

Example:

```yaml
volumes:
  - /proc:/host/proc:ro
  - /sys:/host/sys:ro
  - /etc:/host/etc:ro
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

Do not assume `/proc` inside a container represents the host unless configured appropriately.

Document the required Docker configuration.

---

# Native Linux Installation

Also provide a native binary installation.

The application should support:

```bash
./sysmon
```

and optionally provide:

```text
sysmon.service
```

for systemd.

Do not assume systemd is present.

Document both systemd and non-systemd usage where practical.

---

# Architecture / Code Organization

Keep the code modular.

Suggested structure:

```text
cmd/
    sysmon/

internal/
    collector/
        cpu/
        memory/
        storage/
        network/
        sensors/
        docker/
        services/

    api/

    config/

    health/

web/
    templates/
    static/
        css/
        js/

deploy/
    docker/
    systemd/

Dockerfile
docker-compose.yml
README.md
```

Do not blindly follow this structure if a better Go architecture is appropriate.

The important requirement is clear separation between:

```text
collectors
API
health evaluation
frontend
configuration
deployment
```

---

# Cross-Platform Requirements

Never assume:

* CPU count
* network interface names
* filesystem paths
* systemd
* Docker
* temperature sensors
* battery
* architecture
* distribution

Use feature detection.

If a metric is unavailable:

```text
N/A
```

instead of crashing.

---

# Testing

Provide tests for:

* CPU collector
* memory collector
* storage collector
* network collector
* JSON API
* configuration
* health calculation

Do not require Docker to run the unit tests.

Add architecture/build checks for:

```text
linux/amd64
linux/arm64
linux/arm/v7
```

where supported by dependencies.

---

# Observability of the Monitoring Application

The application itself should expose:

```text
/api/health
```

which returns something like:

```json
{
  "status": "ok"
}
```

Also expose version information:

```text
/api/version
```

Example:

```json
{
  "version": "0.1.0",
  "commit": "...",
  "build_date": "..."
}
```

---

# No Unnecessary Features

Do NOT implement in the first version:

* cloud accounts
* user registration
* SaaS integration
* Prometheus
* Grafana
* time-series database
* alerting email system
* Telegram integration
* complex permissions
* Kubernetes monitoring
* log aggregation
* distributed tracing
* AI analysis
* dozens of configurable dashboards

Those can be considered later.

The first release should be **small, fast, reliable, and attractive**.

---

# Deliverables

Produce a complete working repository containing:

1. Go backend
2. Linux collectors
3. REST API
4. Responsive web dashboard
5. Dockerfile
6. Docker Compose example
7. Native binary build instructions
8. systemd service example
9. Configuration example
10. README
11. Unit tests
12. Cross-compilation instructions

The final application should be runnable on the Redmi with approximately:

```bash
docker compose up -d
```

or:

```bash
./sysmon
```

and immediately provide the basic system dashboard.

---

# Development Priority

Implement in this order:

### Phase 1

* CPU
* RAM
* swap
* uptime
* load
* storage
* network
* responsive dashboard

### Phase 2

* temperature/sensors
* Docker containers
* service monitoring
* health status

### Phase 3

* configuration
* systemd integration
* Docker packaging
* ARM64/ARMv7/x86_64 builds

### Phase 4

* prepare architecture for remote agents and centralized monitoring

Do not jump to Phase 4 complexity while Phase 1 is incomplete.

---

# Important Final Requirement

Before considering the project complete, test it on a constrained ARM64 Linux environment similar to the Redmi 4A.

The application must remain useful even if:

* Docker is unavailable
* temperature sensors are unavailable
* systemd is unavailable
* some `/sys` entries are missing
* network interfaces change names
* storage devices are added/removed
* the host has very little RAM

The guiding principle is:

> **"Give me a beautiful, useful view of my Linux machine without turning monitoring into another server workload."**

