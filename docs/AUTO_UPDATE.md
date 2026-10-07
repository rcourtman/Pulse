# Pulse Server Automatic Updates

Pulse supports one-click server updates for supported deployment types. This
document describes the Pulse server runtime, not installed Pulse Agents.

See [Releases and update channels](RELEASE_PROCESS.md) for Stable and Preview
expectations, beta and RC testing stages, and the checks required before a release
becomes stable.

Eligible v6 agents update asynchronously through their own update client. A
server update changes their target version but does not prove fleet convergence.
For v5, PVE, disabled, or failed agent updates, use **Agent Doctor** at
`/settings/infrastructure?agentDoctor=1` or the installer in
**Settings → Infrastructure → Install on a host**. See
[Unified Agent](UNIFIED_AGENT.md#auto-update).

## Supported Deployment Types

| Deployment | Auto-Update | Method |
|------------|-------------|--------|
| **ProxmoxVE LXC** | ✅ Yes | In-app update button |
| **Systemd Service** | ✅ Yes | In-app update button |
| **Docker** | ❌ Manual | Pull new image |
| **Source Build** | ❌ Manual | Build, then install through the existing deployment procedure |

## Using One-Click Updates

### When an Update is Available

1. Navigate to **Settings → System → Updates**
2. If an update is available, you'll see an **"Install Update"** button
3. Click the button to open the confirmation dialog
4. Review the update details:
   - Current version → New version
   - Estimated time
   - Changelog highlights
5. Click **"Install Update"** to begin

### Update Process

1. **Download**: New version is downloaded, its signature and checksum are verified
2. **Validate**: The new binary is executed with `--version` to prove it runs on this host and reports the expected version, before anything is touched
3. **Backup**: Current installation is backed up
4. **Apply**: Files are updated
5. **Restart**: Service restarts automatically
6. **Verify**: Health check confirms success

### Progress Tracking

A real-time progress modal shows:
- Current step
- Download progress
- Any warnings or errors
- Automatic page reload on success

## Configuration

### Update Preferences

In **Settings → System → Updates**:

| Setting | Description |
|---------|-------------|
| **Update Channel** | Stable (recommended for production) or Pre-release (opt-in preview) |
| **Automatic Stable Updates** | Allow unattended stable updates on supported host installations with an installed update timer |

The update timer must already be installed and enabled for unattended updates.
Saving the UI preference does not provision or start a missing timer. Check the
existing deployment rather than editing `system.json` or creating another
update service to make the preference take effect.

### Stored Settings (system.json)

Auto-update preferences are stored in `system.json` and edited via the UI.

```json
{
  "autoUpdateEnabled": false,
  "updateChannel": "stable"
}
```

**Note:** The update schedule itself lives in the systemd timer (daily at 02:00 plus up to 4 hours of random delay), not in `system.json`. The legacy `autoUpdateCheckInterval` and `autoUpdateTime` fields were never consumed by anything and are ignored if present in older files.

**Channel policy note:** `stable` is the default and only recommended channel for paid or production environments. `rc` remains the internal channel key, but the user-facing meaning is an explicit pre-release preview path. In v6, unattended systemd auto-updates remain `stable`-only even if `updateChannel` is set to `rc`.

## Manual Update Methods

### Docker

Use [Docker server updates](DOCKER.md#-updates). Change the configured image to
the exact release first, then pull and recreate only the Pulse service with the
same mounted data. Pulling another tag alone does not change the image in your
Compose file. Do not run `docker compose down` for a Pulse-only update or remove
its data volume.

Private Pro installs must keep the private image supplied by the
[download page](https://pulserelay.pro/download.html); replacing it with
`rcourtman/pulse` changes the runtime edition. Server updates and monitored
container updates are separate operations.

### ProxmoxVE LXC (Manual)

```bash
sudo /bin/update
```

`/bin/update` is installed by the supported Pulse server installer and preserves the signed-installer trust chain. If your host does not have it yet, use the signed server-installer flow in [INSTALL.md](INSTALL.md). Agent updates still use the `/install.sh` command generated in **Settings → Infrastructure → Install on a host**.

### Systemd Service (Manual)

```bash
sudo /bin/update
```

`/bin/update` is installed by the supported Pulse server installer and preserves the signed-installer trust chain. If your host does not have it yet, use the signed server-installer flow in [INSTALL.md](INSTALL.md). Agent updates still use the `/install.sh` command generated in **Settings → Infrastructure → Install on a host**.

### Source Build

`make build` writes a `pulse` binary in the source checkout. It does not install
that binary into the running service's executable path. Restarting the service
after a build can therefore restart the old installed binary; if the service
runs directly from the checkout, building there can instead overwrite its live
executable. Do not build in the active installation directory.

For a production update, prefer the exact signed release and the
[existing-installation update procedure](INSTALL.md#-updates).
Keep private Pro installations on their private runtime; a public source build
is not a replacement for it.

If you intentionally maintain a source-built deployment:

1. Select the intended source tag or commit explicitly in a separate build
   checkout, preserving local changes. A moving `main` branch is development
   source, not the latest published stable release.
2. Use that checkout's declared toolchain and locked dependencies. Build and
   check the resulting binary's version before stopping the running service;
   a successful build alone is not an installed update.
3. Install through your existing deployment procedure, preserving its service
   identity, executable path, configuration, credentials and data. Stage the
   replacement separately; do not copy onto a running executable or remove
   the previous binary and recovery snapshots first. If the active executable
   path or recovery procedure is unknown, stop before changing the service.
4. Restore only a service that was active before the update, then confirm the
   running version, ordinary collection and notification delivery. Retain the
   previous binary and state backup until recovery is verified; a restart or
   version check alone does not establish healthy monitoring.

Do not pull, rebuild or restart just to reproduce an update failure. Follow
[update preparation and recovery limits](DEPLOYMENT_MODELS.md#updates-by-model) before another attempt.

## Rollback

An update error or a stuck progress modal does not prove that the old version
is still running. Check the running version and service health before retrying
or restoring anything; preserve the update error, logs and history. Do not run
another update just to reproduce a failure.

Before any rollback, take a consistent private backup of the current state and
check the proposed backup's scope. Keep the failed installation and update
snapshots until recovery is verified. Returning to an older binary is not a
full-state recovery. Restoring a matching data snapshot can discard settings,
alert changes and history written since that snapshot.

### Automatic Rollback

The in-app updater creates an installation snapshot before applying files. If
applying files fails, it attempts to restore that snapshot and logs any restore
error. This is not a guarantee that every update failure restores all state or
restarts successfully. Confirm the running version, connections and service
health afterwards; a progress message alone is not recovery evidence.

### Roll back from Update History

If your installed version offers it, open **Settings → System → Updates →
Update History**. **Roll back** is offered for a successful in-app update with
a retained backup. The confirmation names the version before that update and
Pulse restarts to use the restored binary. Check that version and the actual
backup contents before confirming.

**Update History is not a full-state recovery tool.** Its restore scope depends
on the installed updater. Older updaters may replace `data/`, `config/` and
`.env` under the installation directory, but omit active data stored elsewhere.
Do not assume later settings and alert changes will be reverted, even if the
confirmation says so. If active data shares those installation paths, do not
restore it while Pulse is running; use the stopped-service procedure in
[Manual Rollback](#manual-rollback).

Older versions may not have this control. An absent button or backup does not
justify deleting data or creating an empty configuration. Docker server installs
use their previous image and matching data backup, not this in-app rollback.

### What an update snapshot contains

In-app snapshots normally live at `backup-<timestamp>/` under the runtime data
directory (usually `/etc/pulse`), with `/tmp/pulse-backup-<timestamp>` as a
low-space fallback. Use the `backup_path` recorded for the actual update in
`update-history.jsonl`, rather than guessing a timestamp. The updater retains
the most recent three snapshots; a `/tmp` snapshot may also be lost on reboot.

Snapshot scope is version-specific. The published **v6.4.5** and
**v6.4.6-rc.1** in-app updaters use the limited installation snapshot described
below. Do not assume a backup made by another updater version has the same
contents or restore behaviour.

The limited snapshot copies the running server binary, available `VERSION` and
`.env` files, and available `data/` and `config/` directories under
`PULSE_INSTALL_DIR` (default `/opt/pulse`). It is not necessarily a complete
backup of the active `PULSE_DATA_DIR`, external metrics/audit stores, service
units, bundled agents or update helpers. Copy errors can leave a partial
snapshot: a recorded path is not proof that every file was saved.

A backup's existence, or the presence of a database file, does not establish
completeness, consistency or that Update History will restore it. A snapshot
that includes active data still needs a verified, stopped-service recovery
procedure; its presence is not permission to rewind live runtime stores.

Verify the backup against your effective service paths and update logs. Preserve
the active data directory, matching `.encryption.key`, audit history and its
signing key together, including custom paths and organisation directories. Use
a stopped, consistent filesystem/volume backup; copying a live `.db` file alone
can omit SQLite sidecar files. Keep these backups private, not in an issue or
diagnostic attachment. See [configuration paths](CONFIGURATION.md) and
[audit storage and safe recovery](AUDIT_LOGGING.md#storage).

### Manual Rollback

If Pulse cannot start, work inside the Pulse host or LXC, not on the Proxmox host
itself. Start with read-only service discovery:

```bash
systemctl is-active pulse
systemctl show pulse --property=FragmentPath --property=DropInPaths
```

An inactive service makes `is-active` return non-zero; that is useful evidence,
not a reason to reset it. Inspect the named unit and drop-ins locally for the
effective executable, `PULSE_INSTALL_DIR` and `PULSE_DATA_DIR`. Do not post full
service environments or `.env` contents; they may contain credentials.

1. Stop Pulse before taking the failed-state backup or replacing files. Preserve
   the actual executable, distribution files, unit/drop-ins and every active
   data path, not just a guessed `/opt/pulse/data` directory.
2. Match the retained backup to the intended pre-update version. Check its
   files and the update log for missing/copy-failed entries. If the backup is
   missing, partial or has the wrong scope, stop here; do not delete live
   directories to make a restore fit.
3. Prefer restoring a verified pre-update filesystem/volume snapshot with your
   host backup tooling. Check it in an isolated recovery instance first, with
   no access to monitored systems or notification destinations. Do not start a
   second connected copy with the same agent identities.
4. For a file-level repair, stage verified replacements and keep reversible
   copies of the failed state. Restore only the confirmed paths, ownership and
   permissions for this deployment. Do not merge unrelated snapshots, delete
   data/configuration directories, or regenerate encryption or signing keys.
5. Start Pulse and verify the running version and edition, service health,
   saved connections, agent identities and retained history. If any check fails,
   preserve the error and backups rather than retrying the update blindly.

For Docker, change the configured image back to the recorded previous tag or
digest and recreate only Pulse with its existing volume. An older image may not
understand data migrated by the newer version; use a verified matching data
backup when required. Never remove the volume or initialise a fresh one as a
rollback shortcut. An image change alone is not a data rollback.

## Update History

History entries are stored in `update-history.jsonl` under the Pulse data directory (`/etc/pulse` or `/data`), and exposed via `GET /api/updates/history` (admin auth required).

Systemd/LXC update runs write detailed logs to `/var/log/pulse/update-<timestamp>.log`.

## Troubleshooting

### Update button not showing
1. Check if your deployment supports auto-update
2. Verify an update is actually available
3. Ensure you have the latest frontend loaded (hard refresh)

### Update failed

First identify what was updated: this guide covers the **Pulse server**.
An installed agent has its [own update path](UNIFIED_AGENT.md#auto-update).
For a monitored Docker workload, use the [failed or pending workload checks](DOCKER.md#check-a-failed-or-pending-workload-update)
on that workload's host, not inside Pulse.

Keep the original target version, attempt time, last displayed step and redacted
error from the progress modal or updater log. If Pulse still opens, check
**Settings → System → Updates** for its running version and retain the existing
**Update History** entry when available. Check the ordinary UI and monitoring
freshness too. A failed updater exit, disconnected progress stream or stuck
modal does not establish which version is running or whether rollback succeeded.
Do not rerun an update, restart or reinstall merely to obtain evidence.

For server logs, use the [bounded Pulse log readers](TROUBLESHOOTING.md#inspect-notification-logs)
for the actual deployment. Run them on the Pulse host, or inside the Pulse LXC,
not on a monitored guest. They have a deadline as well as time and record limits;
do not substitute a log-follow command or an unbounded reader if they fail.
Inspect the relevant part of the original `/var/log/pulse/update-<timestamp>.log`
privately when that systemd/LXC update produced one; a server journal alone may
not contain the installer failure. Missing logs are unavailable evidence, not
proof that no change occurred. Do not enable Debug or repeat the failing action.

If the error names disk space, check headroom on the actual staging, installation
and data filesystems before planning another attempt; the release payload and
rollback snapshot both need space. Do not delete configuration backups, keys or
history to clear the error. A download error needs its original HTTP, connectivity
or certificate evidence; do not disable TLS verification or switch a paid Pro
installation to the public Community runtime as a workaround.

Share only the relevant times, versions, failed step and redacted error. Keep
full logs, service environments, configuration, credentials and private backup
paths out of public reports.

### Service won't restart after update

Distinguish the **Pulse service** from the separate updater service: a failed
`pulse-update.service` is not proof that `pulse` is stopped. Use the read-only
service discovery in [Manual Rollback](#manual-rollback), substituting the actual
service name (`pulse-backend` on some older installs), and the bounded log readers
above. For Docker, use the [current container-state and image checks](DOCKER.md#check-a-failed-or-pending-workload-update)
with the Pulse server's container name. A running process or container alone is
not proof that the UI, saved connections and history are healthy.

If recovery is needed, follow [Manual Rollback](#manual-rollback): preserve the
failed state, verify the intended backup's version and complete scope, and use
the stopped-service recovery procedure. An absent or partial backup is a reason
to stop, not to overwrite live data, create an empty data directory or retry the
update. Do not restore a guessed snapshot just because the progress modal failed.
