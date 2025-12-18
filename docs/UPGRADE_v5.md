# Upgrade to Pulse v5

This is a practical guide for upgrading an existing Pulse install to v5.

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Backups → Create Backup**
- Confirm you can access the host/container console (for rollback and bootstrap token retrieval)
- Review the v5 release notes on GitHub before upgrading

## Upgrade Paths

### systemd and Proxmox LXC installs

Preferred path:

- **Settings → System → Updates**

If you prefer CLI, use the official installer for the target version:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | \
  sudo bash -s -- --version v5.0.0
```

### Docker

```bash
docker pull rcourtman/pulse:latest
docker compose up -d
```

### Kubernetes (Helm)

```bash
helm upgrade pulse oci://ghcr.io/rcourtman/pulse-chart -n pulse
```

## Post-Upgrade Checklist

- Confirm version: `GET /api/version`
- Confirm scheduler health: `GET /api/monitoring/scheduler/health`
- Confirm nodes are polling and no breakers are stuck open
- Confirm notifications still send (send a test)
- Confirm agents are connected (if used)

## Notes and Common Gotchas

### Bootstrap token on fresh auth setup

If you reset auth (for example by deleting `.env`), Pulse may require a bootstrap token before you can complete setup.

- Docker: `docker exec pulse /app/pulse bootstrap-token`
- systemd/LXC: `sudo pulse bootstrap-token`

### Temperature monitoring in containers

If Pulse runs in a container and you are relying on SSH-based temperature collection, v5 blocks that in hardened configurations.

Preferred option:

- Install the unified agent (`pulse-agent`) on Proxmox hosts with `--enable-proxmox`

Deprecated option (existing installs only):

- `pulse-sensor-proxy` continues to work for now, but it is deprecated in v5 and not recommended for new installs. Plan to migrate to the unified agent.
