# Automatic Updates

Pulse 5.0 introduces one-click updates for supported deployment types, making it easy to keep your monitoring system up to date.

## Supported Deployment Types

| Deployment | Auto-Update | Method |
|------------|-------------|--------|
| **ProxmoxVE LXC** | ✅ Yes | In-app update button |
| **Systemd Service** | ✅ Yes | In-app update button |
| **Docker** | ❌ Manual | Pull new image |
| **Source Build** | ❌ Manual | Git pull + rebuild |

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

1. **Download**: New version is downloaded
2. **Backup**: Current installation is backed up
3. **Apply**: Files are updated
4. **Restart**: Service restarts automatically
5. **Verify**: Health check confirms success

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
| **Update Channel** | Stable (recommended) or Release Candidate |
| **Auto-Check** | Background update check interval (hours); `0` disables |

### Stored Settings (system.json)

Auto-update preferences are stored in `system.json` and edited via the UI.

```json
{
  "autoUpdateEnabled": false,
  "updateChannel": "stable",
  "autoUpdateCheckInterval": 24,
  "autoUpdateTime": "03:00"
}
```

**Note:** `autoUpdateTime` is stored for UI reference. The systemd timer still runs on its own schedule (02:00 + jitter). Background update checks follow `autoUpdateCheckInterval`.

## Manual Update Methods

### Docker

```bash
# Pull latest image
docker pull rcourtman/pulse:latest

# Restart container
docker compose down && docker compose up -d
```

If you use the legacy `docker-compose` binary, replace `docker compose` with `docker-compose`.

### ProxmoxVE LXC (Manual)

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

This script installs/updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Agents → Installation commands**.

### Systemd Service (Manual)

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

This script installs/updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Agents → Installation commands**.

### Source Build

```bash
cd /path/to/pulse
git pull
make build
sudo systemctl restart pulse
```

## Rollback

If an update causes issues:

### Automatic Rollback
Pulse creates a backup before updating. If the update fails:
1. The previous version is automatically restored
2. Service restarts with the old version
3. Error details are logged

### Manual Rollback
Backups created by in-app updates are stored as `backup-<timestamp>/` folders inside the Pulse data directory (`/etc/pulse` or `/data`). If that directory is not writable, Pulse falls back to `/tmp/pulse-backup-<timestamp>`.
There is no rollback UI. To revert, stop Pulse, restore the backup contents to `/opt/pulse`, then restart.

Example (systemd/LXC):
```bash
sudo systemctl stop pulse
sudo cp -a /etc/pulse/backup-<timestamp>/pulse /opt/pulse/pulse
sudo cp -a /etc/pulse/backup-<timestamp>/VERSION /opt/pulse/VERSION
sudo rm -rf /opt/pulse/data /opt/pulse/config
sudo cp -a /etc/pulse/backup-<timestamp>/data /opt/pulse/data
sudo cp -a /etc/pulse/backup-<timestamp>/config /opt/pulse/config
sudo cp -a /etc/pulse/backup-<timestamp>/.env /opt/pulse/.env
sudo systemctl start pulse
```

## Update History

History entries are stored in `update-history.jsonl` under the Pulse data directory (`/etc/pulse` or `/data`), and exposed via `GET /api/updates/history` (admin auth required).

Systemd/LXC update runs write detailed logs to `/var/log/pulse/update-<timestamp>.log`.

## Troubleshooting

### Update button not showing
1. Check if your deployment supports auto-update
2. Verify an update is actually available
3. Ensure you have the latest frontend loaded (hard refresh)

### Update failed
1. Check the error message in the progress modal
2. Review logs: `journalctl -u pulse -n 100` or `/var/log/pulse/update-<timestamp>.log`
3. Verify disk space is available
4. Check network connectivity to GitHub

### Service won't restart after update
1. Check systemd status: `systemctl status pulse`
2. View recent logs: `journalctl -u pulse -f`
3. Manually restore from backup if needed
