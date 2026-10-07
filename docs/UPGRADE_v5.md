# Upgrade to Pulse v5

This historical guide covers the v4-to-v5 transition. For an upgrade to a
current release, follow [Updating Pulse](AUTO_UPDATE.md) and the
[current installation and recovery guidance](INSTALL.md); do not downgrade
to v5 to follow this page.

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Recovery → Create Backup** (older versions labeled this **Backups**)
- Confirm you can access the host/container console (for rollback and bootstrap token retrieval)
- Review the v5 release notes on GitHub before upgrading

## Upgrade Paths

### systemd and Proxmox LXC installs

Preferred path:

- **Settings → System → Updates**

If you prefer CLI, use the installed update helper for the target version:

```bash
sudo /bin/update --version vX.Y.Z
```

`/bin/update` is installed by the supported systemd and Proxmox LXC server installer. If your host does not have it yet, follow the signed server-installer flow in [INSTALL.md](INSTALL.md). Agent updates still use the `/install.sh` command generated in **Settings → Infrastructure → Install on a host**.

### Docker

```bash
docker pull rcourtman/pulse:vX.Y.Z
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

An upgrade is not a reason to reset authentication. **Do not delete `.env` or
repeat first-time setup to recover an existing login.** Follow
[password recovery](TROUBLESHOOTING.md#i-forgot-my-password), preserving the
existing configuration and data.

A bootstrap token is for an installation that genuinely needs initial setup,
not a replacement for an existing password. Retrieve it locally only when the
setup screen asks for it:

- Docker: `docker exec pulse /app/pulse bootstrap-token`
- systemd/LXC: `sudo pulse bootstrap-token`

Keep the token private and enter it in the setup screen; do not post it in a
report or put it in a URL.

### Sensor proxy removal

The `pulse-sensor-proxy` from v4 is no longer needed — temperature monitoring is now handled by the unified agent. If you had the sensor proxy installed on your Proxmox hosts, remove it **on each host** after upgrading:

```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh | \
  sudo bash -s -- --uninstall --purge --local-only
```

If you deleted the old node from Pulse and want the cleanup to also remove the old `pulse-monitor@pam` API user and tokens before reinstalling, add `--remove-proxmox-access`.

Run the local-only command on every Proxmox node that carried the proxy. The
optional cluster-wide mode uses strict OpenSSH host-key verification and
requires already-provisioned user/system known_hosts trust (or an explicit
`--ssh-known-hosts /path/to/known_hosts` file); it never accepts unknown keys.

See the [Legacy Cleanup](TEMPERATURE_MONITORING.md#legacy-cleanup-if-upgrading) section in the temperature monitoring docs for the full cleanup details.

Skipping this step will leave a selfheal timer running on the host that generates recurring `TASK ERROR` entries in the Proxmox task log.

### Temperature monitoring in containers

If Pulse runs in a container and you are relying on SSH-based temperature collection, move to the agent or run Pulse on the host. SSH-based collection from containers is intended for dev/test only (use `PULSE_DEV_ALLOW_CONTAINER_SSH=true` if you must).

Preferred option:

- Install the unified agent (`pulse-agent`) on Proxmox hosts with `--enable-proxmox`

Alternative option:

- Run Pulse outside a container and use SSH-based temperature collection (restricted `sensors -j` keys)

### Backups not showing (PVE)

An empty backup table does not establish a permission failure. Pulse's server
reads PVE backup inventory through the saved Proxmox API connection; a working
host agent does not prove that this separate collection is current.

1. Compare an **existing** archive in native PVE with **Proxmox → Backups →
   By date**, retaining its node, storage, guest type/ID and time. Check the
   selected connection and filters. Direct PBS and PVE passthrough are distinct
   sources, and Coverage posture is not the same as archive presence. A failed
   or partial read is not an empty inventory.
2. Retain the existing collection status, original redacted error and time.
   Connection liveness and a successful connection test do not establish backup
   freshness. Do not run another backup, restore, guest-agent probe or restart
   just to obtain evidence. An OK backup task does not prove guest thaw; follow
   the [backup safety precaution](VM_DISK_MONITORING.md#backup-safety).
3. If a request was rejected, have the Proxmox administrator check the **actual
   rejected endpoint**, installed PVE version and configured service user/token.
   With privilege separation, both user and token ACLs must permit that request
   at the relevant scope and inheritance. Authentication failure is not proof
   of a missing storage role, and audit-only access is not proof of access to
   every storage-content endpoint. Do not grant blanket storage-administrator
   access, disable privilege separation or substitute an administrator token
   merely because backups are missing.

**Do not delete the monitored node, remove registration state or rerun setup as
a diagnostic.** Setup can rotate an existing API token and change permissions.
Preserve the connection, credentials, registration state and backup/history data.
Make any evidenced repair through normal maintenance outside backups, then
observe ordinary polling rather than repeating **Test Connection**.

Use the [complete missing-PVE-backup checks](UNIFIED_AGENT.md#pve-backups-not-showing-recovery)
and [Proxmox permission guidance](TROUBLESHOOTING.md#check-permissions-proxmox).
Keep token secrets, full ACL listings and private infrastructure details out of
public reports; follow [Getting Help](TROUBLESHOOTING.md#-getting-help).
