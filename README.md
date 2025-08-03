# <img src="frontend-modern/public/pulse-icon.svg" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

**Real-time monitoring for Proxmox VE and PBS with alerts, webhooks, and a clean web interface.**

![Pulse Dashboard](docs/images/01-dashboard.png)

## Key Features

- **Real-time Monitoring** - Live updates for VMs, containers, nodes, and storage via WebSockets
- **Smart Alerts** - Configurable thresholds with email and webhook notifications (Discord, Slack, Gotify, Telegram, ntfy.sh, Teams)
- **Alert History** - Persistent storage of alert events with detailed metrics and timeline
- **Unified Backups** - Single view for PBS backups, PVE backups, and snapshots
- **PBS Push Mode** - Monitor isolated/firewalled PBS servers without inbound connections
- **Modern UI** - Responsive design with dark/light themes, virtual scrolling, and expandable charts
- **Performance** - Built with Go for minimal resource usage, stops polling when no clients connected
- **Secure by Default** - Encrypted configuration storage with flexible credential management

[View Screenshots →](docs/SCREENSHOTS.md)

## Support Development

Pulse is a solo hobby project developed in my free time. If you find it useful, your support helps keep me motivated and covers hosting costs.

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsors&logo=GitHub%20Sponsors&style=for-the-badge)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## Quick Start (2 minutes)

### Prerequisites
- Proxmox VE 7.0+ or PBS 2.0+
- Network access to Proxmox API (ports 8006/8007)

### Install Pulse

Choose **one** method:

```bash
# Option A: Automated LXC Container (Easiest)
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"

# Option B: Docker (For existing Docker hosts)
docker run -d -p 7655:7655 -v pulse_config:/etc/pulse -v pulse_data:/data rcourtman/pulse:latest

# Option C: Manual Install (For existing LXC/VMs)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh | sudo bash
```

### Configure Pulse

1. Open Pulse in your browser: `http://<your-server>:7655`
2. Go to **Settings** → **Nodes** → **Add Node**
3. Enter your Proxmox credentials
4. Click **Save** - Pulse will start monitoring immediately

## Configuration

### Web UI (Recommended)

All configuration can be done through the web interface:
- **Settings** → **Nodes**: Add/remove Proxmox instances
- **Settings** → **General**: Configure ports, intervals, themes
- **Alerts**: Set up thresholds and notification channels

### Configuration File

For advanced users or automation, you can use `/etc/pulse/pulse.yml`:

```yaml
# Basic configuration
server:
  backend:
    port: 3000
    host: "0.0.0.0"
  frontend:
    port: 7655
    host: "0.0.0.0"

# Proxmox nodes
nodes:
  pve:
    - name: my-cluster
      host: https://proxmox.example.com:8006
      user: pulse@pam
      password: your-password
      # Or use token auth:
      # token_name: pulse-token
      # token_value: secret-token-value
      verifySSL: false

# Monitoring settings
monitoring:
  pollingInterval: 5000      # milliseconds
  backupPollingCycles: 10    # poll backups every N cycles

# Alert settings
alerts:
  enabled: true
  cpuThreshold: 80
  memoryThreshold: 80
  diskThreshold: 80
```

### Environment Variables

All settings can be overridden with environment variables:

```bash
PULSE_SERVER_FRONTEND_PORT=8080
PULSE_MONITORING_POLLING_INTERVAL=10000
PULSE_NODES_PVE_0_HOST=https://proxmox.local:8006
PULSE_NODES_PVE_0_USER=monitor@pve
PULSE_NODES_PVE_0_PASSWORD=secret
```

### Security Options

See [Security Guide](docs/SECURITY.md) for credential management options:
- Encrypted file storage (default)
- Environment variable references
- External secret files
- Integration with secret management tools

## Webhooks

Pulse supports multiple webhook providers for alerts:

- **Discord** - Native Discord webhooks with rich embeds
- **Slack** - Slack incoming webhooks
- **Gotify** - Self-hosted push notifications
- **Telegram** - Telegram bot notifications
- **ntfy.sh** - Simple pub-sub notifications
- **Teams** - Microsoft Teams incoming webhooks
- **Generic** - Any webhook endpoint (JSON POST)

Configure webhooks in **Alerts** → **Destinations** → **Webhooks**.

## PBS Agent (Push Mode)

For isolated PBS servers that can't be reached directly:

```bash
# On the PBS server:
cd /opt
wget https://github.com/rcourtman/Pulse/releases/latest/download/pulse-pbs-agent.tar.gz
tar xzf pulse-pbs-agent.tar.gz
cd pulse-pbs-agent
./install.sh
```

Configure the agent to push data to your Pulse instance. See [PBS Agent Guide](docs/PBS-AGENT.md).

## Docker Compose

```yaml
version: '3.8'

services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_config:/etc/pulse
      - pulse_data:/data
    environment:
      - PULSE_NODES_PVE_0_HOST=https://proxmox.local:8006
      - PULSE_NODES_PVE_0_USER=monitor@pve
      - PULSE_NODES_PVE_0_PASSWORD=${PROXMOX_PASSWORD}
    restart: unless-stopped

volumes:
  pulse_config:
  pulse_data:
```

## Building from Source

```bash
# Clone repository
git clone https://github.com/rcourtman/Pulse.git
cd Pulse

# Build backend
go build -o pulse ./cmd/pulse

# Build frontend
cd frontend-modern
npm install
npm run build

# Run
./pulse
```

## API Documentation

Pulse provides a REST API for integration:

- `GET /api/state` - Current state of all resources
- `GET /api/nodes` - Node configuration
- `GET /api/alerts` - Alert status and history
- `POST /api/alerts/acknowledge` - Acknowledge alerts
- `GET /api/backups` - Backup information
- `GET /api/charts/:type/:id` - Historical metrics

WebSocket endpoint: `ws://your-server:3000/ws` for real-time updates.

## Troubleshooting

### Common Issues

1. **"Connection refused" error**
   - Check firewall rules for ports 7655 (UI) and 3000 (API)
   - Verify Pulse is running: `systemctl status pulse`

2. **"Invalid credentials" error**
   - Ensure user has at least `PVEAuditor` role
   - For PBS, user needs `DatastoreReader` permissions
   - Try token authentication instead of password

3. **No data showing**
   - Check browser console for errors
   - Verify Proxmox API is accessible from Pulse server
   - Check Pulse logs: `journalctl -u pulse -f`

### Getting Help

- [Report Issues](https://github.com/rcourtman/Pulse/issues)
- [Discussions](https://github.com/rcourtman/Pulse/discussions)
- [FAQ](docs/FAQ.md)

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Go](https://golang.org/) and [SolidJS](https://www.solidjs.com/)
- Icons by [Lucide](https://lucide.dev/)
- Inspired by the Proxmox community's monitoring needs

---

Made with ❤️ for the Proxmox community