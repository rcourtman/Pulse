# Upgrade to Pulse v5

This historical guide covers the v4-to-v5 transition. For an upgrade to a
current release, follow [Updating Pulse](AUTO_UPDATE.md) and the
[current installation and recovery guidance](INSTALL.md); do not downgrade
to v5 to follow this page.

## Before You Upgrade

- Keep a private, consistent backup of the existing installation, including its
  active data, encryption key and deployment configuration. **Create Backup** in
  **Settings → System → Recovery** (older versions labelled this **Backups**) is
  a configuration export, **not a full-state backup**: it excludes history and
  agent enrolment state. Follow [full-state recovery](MIGRATION.md#full-state-recovery)
  for the backup scope; do not copy live data or assume an updater snapshot is complete.
- Confirm existing administrator access to Pulse and its host/container console.
  Preserve login, agent identities, credentials and connection state; an upgrade
  does not require another first-time setup.
- Choose an actually [published software release](https://github.com/rcourtman/Pulse/releases)
  and read its notes and recovery instructions. A historical v5 packet, tag,
  draft or Helm-chart release is not the current server download. Keep the
  installation's private Pro runtime if it uses one.

## Upgrade Paths

### systemd and Proxmox LXC installs

Use **Settings → System → Updates** where supported. For CLI updates, follow
[the current manual update procedure](INSTALL.md#manual-update): it checks the
helper's ownership and preserves the existing service and data/config paths.
The name `/bin/update` alone does not identify the helper; do not execute an
absent or unknown helper or install over a custom deployment with default paths.
Work inside the existing Pulse LXC, not on the Proxmox host.

If a community helper selects a chart instead of server software, follow
[chart-selection recovery](AUTO_UPDATE.md#a-helper-selects-a-helm-chart-release).
Do not recreate the LXC, supply a GitHub token or bypass signatures. Agent
installation and updates are separate: use the [trusted-instance agent
procedure](UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas), not the server's top-level installer.

### Docker

Follow [Docker updates](DOCKER.md#-updates) for the existing Compose service and
an explicit published image tag. Preserve its mounted data, credentials and
collector options. A successful pull does not replace the running container;
do not replace a private Pro image with a public one or recreate unrelated services.

### Kubernetes (Helm)

Follow [the current Helm installation guidance](INSTALL.md#1-kubernetes-helm),
preserving the existing release, namespace, values and persistent volumes.
Chart packaging and the server image are separate versions; do not use a chart
asset to update a systemd or LXC server.

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

The retired `pulse-sensor-proxy` is separate from the unified agent and Pulse
server. Upgrading the server does not remove it. Existing selfheal timers can
produce recurring task errors, but a missing temperature is not a reason to
run cleanup, remove a node or repeat setup.

**Cleanup can interrupt workloads, not just temperature collection.** It edits
affected LXC configurations and can stop and start running LXCs, including
Pulse. `--local-only` prevents cluster SSH; it does not restrict cleanup to one
container. An authorised Proxmox administrator must identify the affected
workloads, preserve private recovery evidence and schedule their maintenance
before running the inspected local helper. Do not pipe a web response into a
privileged shell.

Use the [complete legacy cleanup procedure](TEMPERATURE_MONITORING.md#legacy-cleanup-if-upgrading)
for private download/inspection, prior-active-state recording, local execution
and verified restoration. It deliberately preserves proxy state, configuration,
logs and the service account. Do not add `--purge` or `--remove-proxmox-access`
to make cleanup succeed: they can erase recovery evidence or delete the
`pulse-monitor@pam` API user and tokens still used by current connections.
Neither is required to stop the retired proxy. Cleanup is not a cluster-wide
uninstall; retain unrelated SSH trust and monitoring access.

### Temperature monitoring in containers

For current installations, follow [verified HTTPS and token-file agent setup](TEMPERATURE_MONITORING.md#recommended-pulse-agent-proxmox)
on the monitored Proxmox host. Keep the server's deployment separate from the
collector's. Do not move a working server, enable development-only container
SSH or reinstall an agent just to diagnose a missing temperature. Existing
restricted SSH collection is a separate path; the [temperature guide](TEMPERATURE_MONITORING.md)
explains its trust and missing-reading limits.

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
