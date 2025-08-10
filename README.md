# Pulse for Proxmox

Real-time monitoring for Proxmox VE and PBS with alerts and webhooks.

> **⚠️ Upgrading from v3?** See [Migration Guide](docs/MIGRATION_V3_TO_V4.md) - automatic upgrades will break.

![Dashboard](docs/images/01-dashboard.png)

## Features

- **Auto-Discovery**: Automatically finds and configures Proxmox nodes
- **Cluster Support**: Detects and monitors entire Proxmox clusters
- Live monitoring of VMs, containers, nodes, storage
- Alerts with email and webhooks (Discord, Slack, Telegram, Teams, ntfy.sh, Gotify)
- Unified view of PBS backups, PVE backups, and snapshots
- PBS push mode for firewalled servers
- **Secure Config Export/Import**: Encrypted backup and restore
- Dark/light themes, responsive design
- Built with Go for minimal resource usage

[Screenshots →](docs/SCREENSHOTS.md)

## Quick Start

### Install

```bash
# Option A: Proxmox Helper Script (creates LXC container)
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"

# Option B: Docker
docker run -d -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest

# Option C: Manual (existing systems)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash
```

### Configure

**Option 1: Auto-Registration (NEW)**
1. Open `http://<your-server>:7655`
2. Settings → Nodes → Shows discovered nodes automatically
3. Click "Setup Script" to copy one-liner
4. Run script on Proxmox node - auto-configures everything

**Option 2: Manual**
1. Settings → Nodes → Add Node
2. Enter credentials manually
3. Save

## Docker

### Basic
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### With Network Discovery
```bash
# Specify your LAN subnet for auto-discovery
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e DISCOVERY_SUBNET=192.168.1.0/24 \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Docker Compose
```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      # Optional: specify your LAN subnet for auto-discovery
      # - DISCOVERY_SUBNET=192.168.1.0/24
    restart: unless-stopped

volumes:
  pulse_data:
```


## PBS Agent (Push Mode)

For isolated PBS servers, see [PBS Agent documentation](docs/PBS-AGENT.md)

## Configuration

Quick start - most settings are in the web UI:
- **Settings → Nodes**: Add/remove Proxmox instances
- **Settings → System**: Polling intervals, CORS settings
- **Alerts**: Thresholds and notifications

For deployment overrides (ports, etc), use environment variables:
```bash
# Systemd: sudo systemctl edit pulse-backend
Environment="FRONTEND_PORT=8080"

# Docker: -e FRONTEND_PORT=8080
```

📖 **[Full Configuration Guide →](docs/CONFIGURATION.md)**

### Backup/Restore

```bash
# Export (v4.0.3+)
pulse config export -o backup.enc

# Import
pulse config import -i backup.enc
```

Or use Settings → Security tab in UI.

## Updates

### Docker
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
# Run docker run command again
```

### Manual Install
Settings → System → Check for Updates

After updates complete, refresh your browser (Ctrl+F5 or Cmd+Shift+R) to load the new version.

## API

```bash
# Status
curl http://localhost:7655/api/status

# Metrics
curl http://localhost:7655/api/metrics

# With authentication (if configured)
curl -H "X-API-Token: your-token" http://localhost:7655/api/status
```

## Troubleshooting

### Connection Issues
- Check Proxmox API is accessible (port 8006/8007)
- Verify credentials have PVEAuditor role minimum
- For PBS: ensure API token has Datastore.Audit permission

### High CPU/Memory
- Reduce polling interval in Settings
- Check number of monitored nodes
- Disable unused features (backups, snapshots)

### Logs
```bash
# Docker
docker logs pulse

# Manual
journalctl -u pulse -f
```

## Security

- Credentials stored encrypted (AES-256-GCM)
- Optional API token authentication
- Export/import requires passphrase
- [Security Details →](docs/SECURITY.md)

## Development

```bash
# Frontend
cd frontend-modern
npm install
npm run dev

# Backend
go run cmd/pulse/*.go
```

## Links

- [Releases](https://github.com/rcourtman/Pulse/releases)
- [Docker Hub](https://hub.docker.com/r/rcourtman/pulse)
- [Issues](https://github.com/rcourtman/Pulse/issues)

## License

MIT - See [LICENSE](LICENSE)