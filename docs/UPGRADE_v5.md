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
  sudo bash -s -- --version vX.Y.Z
```

This installer updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Agents → Installation commands**.

### Docker

```bash
docker pull rcourtman/pulse:latest
docker compose up -d
```

### Kubernetes (Helm)

```bash
helm repo update
helm upgrade pulse pulse/pulse -n pulse
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

If Pulse runs in a container and you are relying on SSH-based temperature collection, move to the agent or run Pulse on the host. SSH-based collection from containers is intended for dev/test only (use `PULSE_DEV_ALLOW_CONTAINER_SSH=true` if you must).

Preferred option:

- Install the unified agent (`pulse-agent`) on Proxmox hosts with `--enable-proxmox`

Alternative option:

- Run Pulse outside a container and use SSH-based temperature collection (restricted `sensors -j` keys)

### Backups not showing after upgrade (v4 → v5)

If your backups stop appearing after upgrading from v4, your existing API token may be missing the `PVEDatastoreAdmin` permission required for backup visibility.

**Quick fix** (run on each Proxmox host):
```bash
pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin
```

**Alternative** (re-run agent setup):
1. Delete the node from Pulse Settings
2. Re-run the agent setup command from Settings → Proxmox → Add Node
3. The new token will have correct permissions

This happens because v5's agent setup grants broader permissions than the v4 manual setup scripts did.
