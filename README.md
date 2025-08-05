# <img src="docs/images/pulse-logo.svg" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

**Real-time monitoring for Proxmox VE and PBS with alerts, webhooks, and a clean web interface.**

> **⚠️ IMPORTANT: Upgrading from v3?** See the [Migration Guide](docs/MIGRATION_V3_TO_V4.md) - automatic upgrades will break your installation!

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
# Option A: Docker (Multi-arch: AMD64, ARM64, ARMv7) - RECOMMENDED
docker run -d -p 7655:7655 -v pulse_data:/data --restart unless-stopped rcourtman/pulse:latest

# Option B: Manual Install (For existing LXC/VMs)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash

# Option C: Proxmox Helper Script (Fix in progress - PR submitted)
# The helper script is being updated for v4. Until merged, use Option A or B
# bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

### Configure Pulse

1. Open Pulse in your browser: `http://<your-server>:7655`
2. Go to **Settings** → **Nodes** → **Add Node**
3. Enter your Proxmox credentials
4. Click **Save** - Pulse will start monitoring immediately

## Configuration

Pulse uses a modern, secure configuration system similar to popular apps like Radarr and Sonarr:

### Everything Through the UI

- **All configuration is done via the web interface** - no manual file editing needed
- **Settings** → **Nodes**: Add/remove Proxmox instances with a simple form
- **Settings** → **General**: Configure ports, intervals, themes, and more
- **Alerts**: Set up thresholds and notification channels
- **Configuration is encrypted** and stored securely with proper permissions

### Zero Configuration Files

Unlike traditional monitoring tools:
- **No YAML/JSON files to edit**
- **No environment variables to set**
- **No complex configurations**
- **Works immediately** after installation

Just open the web UI, add your Proxmox nodes through the interface, and you're done!

### For Docker Users

Pulse provides multi-architecture Docker images supporting:
- **linux/amd64** - Intel/AMD 64-bit servers
- **linux/arm64** - 64-bit ARM (Raspberry Pi 4/5, Apple Silicon)
- **linux/arm/v7** - 32-bit ARM (Raspberry Pi 2/3)

The only environment variables needed are for the initial ports if you want to change defaults:

```bash
docker run -d -p 8080:8080 \
  -e PULSE_SERVER_FRONTEND_PORT=8080 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Available Docker tags:
- `rcourtman/pulse:latest` - Latest stable release
- `rcourtman/pulse:4` - Latest v4.x release
- `rcourtman/pulse:4.0.0` - Specific version

Once running, all configuration is done through the web UI.

### Security

Pulse automatically encrypts and secures all configuration:
- Credentials are encrypted using AES-256-GCM
- Configuration files have restricted permissions (0600)
- No plaintext passwords in config files
- Encryption keys derived from machine ID

See [Security Guide](docs/SECURITY.md) for additional security options.

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
    image: rcourtman/pulse:latest  # Multi-arch: AMD64, ARM64, ARMv7
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    restart: unless-stopped

volumes:
  pulse_data:
```

After starting, configure everything through the web UI at `http://localhost:7655`.

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

## Troubleshooting

### Common Issues

1. **"Connection refused" error**
   - Check firewall rules for port 7655
   - Verify Pulse is running: `systemctl status pulse-backend`

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