# Deployment Models

Pulse supports multiple deployment models. This page clarifies what differs between them and where “truth” lives (paths, updates, and operational constraints).

## Summary

| Model | Recommended for | Data/config path | Updates |
| --- | --- | --- | --- |
| Proxmox VE LXC (installer) | Proxmox-first deployments | `/etc/pulse` | In-app updates supported |
| systemd (bare metal / VM) | Traditional Linux hosts | `/etc/pulse` | In-app updates supported |
| Docker | Quick evaluation and container stacks | `/data` (bind mount / volume) | Image pull + restart |
| Kubernetes (Helm) | Cluster operators | `/data` (PVC) | Helm upgrade |

## Common Ports

- UI/API: `7655/tcp`
- Prometheus metrics: `9091/tcp` (`/metrics` on a separate listener)

Docker and Kubernetes do not publish `9091` unless you explicitly expose it.

## Where Configuration Lives

Pulse uses a split config model:

- **Local auth and secrets**: `.env` (managed by Quick Security Setup or environment overrides, not shown in the UI)
- **System settings**: `system.json` (editable in the UI unless locked by env)
- **Nodes and credentials**: `nodes.enc` (encrypted)
- **AI config**: `ai.enc` (encrypted)
- **Metrics history**: `metrics.db` (SQLite)

Path mapping:

- systemd/LXC: `/etc/pulse/*`
- Docker/Helm: `/data/*`

## Updates by Model

### systemd and Proxmox LXC

Use the UI:

- **Settings → System → Updates**

These deployments can apply updates by downloading a release and swapping binaries/config safely with backups and history.

### Docker

Pull a new image and restart:

```bash
docker pull rcourtman/pulse:latest
docker compose up -d
```

### Kubernetes (Helm)

Upgrade the chart (OCI):

```bash
helm upgrade pulse oci://ghcr.io/rcourtman/pulse-chart -n pulse
```
