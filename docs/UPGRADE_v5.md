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

This installer updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Unified Agents → Installation commands**.

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

### Sensor proxy removal

The `pulse-sensor-proxy` from v4 is no longer needed — temperature monitoring is now handled by the unified agent. If you had the sensor proxy installed on your Proxmox hosts, remove it **on each host** after upgrading. See the [Legacy Cleanup](TEMPERATURE_MONITORING.md#legacy-cleanup-if-upgrading) section in the temperature monitoring docs for the full cleanup commands.

Skipping this step will leave a selfheal timer running on the host that generates recurring `TASK ERROR` entries in the Proxmox task log.

### Temperature monitoring in containers

If Pulse runs in a container and you are relying on SSH-based temperature collection, move to the agent or run Pulse on the host. SSH-based collection from containers is intended for dev/test only (use `PULSE_DEV_ALLOW_CONTAINER_SSH=true` if you must).

Preferred option:

- Install the unified agent (`pulse-agent`) on Proxmox hosts with `--enable-proxmox`

Alternative option:

- Run Pulse outside a container and use SSH-based temperature collection (restricted `sensors -j` keys)

### Backups not showing (PVE)

If local PVE backups aren't appearing in Pulse, your API token may be missing the `PVEDatastoreAdmin` permission required for backup visibility.

This can happen if:
- You upgraded from v4 (older setup scripts didn't include this permission)
- You set up nodes via the unified agent before v5.1.x (the agent wasn't granting this permission)
- You created the API token manually without the storage permission

**Quick fix** (run on each Proxmox host):
```bash
pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin
```

**Alternative** (re-run setup):
1. Delete the node from Pulse Settings
2. Re-run the setup (either the UI-generated script or agent with `--enable-proxmox`)
3. The new token will have correct permissions

Note: The "re-run setup" option only works on v5.1.x or later, which includes the fix for agent-based setups.
