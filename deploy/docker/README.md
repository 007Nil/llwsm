# Running sysmon in Docker

The container needs to see host kernel state. A normal container's /proc
reflects the container itself, so the host root filesystem must be mounted
read-only:

```yaml
volumes:
  - /:/host:ro
```

Then run with `--root /host` (the compose file does this). This single mount
covers /host/proc, /host/sys, and host mount points (/host/mnt/...), which is
all the collectors need.

Optional:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

enables container monitoring. The socket only supports the read-only
containers list and stats endpoints; no container management is exposed.

Network interfaces: by default the container sees its own network namespace.
To monitor the host's interfaces and traffic, use `network_mode: host` and
drop the `ports` mapping.

Security note: mounting the host root read-only inside the container gives
the container read access to host files. Anyone who can execute code inside
the container can read them. Run this only on machines you control.
