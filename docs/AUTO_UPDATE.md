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

1. Navigate to **Settings → System Updates**
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

In **Settings → System Updates**:

| Setting | Description |
|---------|-------------|
| **Update Channel** | Stable (recommended) or Release Candidate |
| **Auto-Check** | Automatically check for updates daily |

### Environment Variables

```bash
# Disable auto-update check
PULSE_AUTO_UPDATE_CHECK=false

# Use release candidate channel
PULSE_UPDATE_CHANNEL=rc
```

## Manual Update Methods

### Docker

```bash
# Pull latest image
docker pull ghcr.io/rcourtman/pulse:latest

# Restart container
docker-compose down && docker-compose up -d
```

### ProxmoxVE LXC (Manual)

```bash
# Inside the container
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install.sh | bash
```

### Systemd Service (Manual)

```bash
# Download new release
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install.sh | bash
```

### Source Build

```bash
cd /opt/pulse
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
```bash
# Backups are stored in /etc/pulse/backups/
ls /etc/pulse/backups/

# Restore a specific backup
sudo /opt/pulse/scripts/restore-backup.sh /etc/pulse/backups/pulse-backup-20250101.tar.gz
```

## Update History

View past updates in **Settings → System Updates → Update History**:
- Previous versions installed
- Update timestamps
- Success/failure status

## Troubleshooting

### Update button not showing
1. Check if your deployment supports auto-update
2. Verify an update is actually available
3. Ensure you have the latest frontend loaded (hard refresh)

### Update failed
1. Check the error message in the progress modal
2. Review logs: `journalctl -u pulse -n 100`
3. Verify disk space is available
4. Check network connectivity to GitHub

### Service won't restart after update
1. Check systemd status: `systemctl status pulse`
2. View recent logs: `journalctl -u pulse -f`
3. Manually restore from backup if needed
