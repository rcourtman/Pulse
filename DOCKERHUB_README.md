# Pulse - Real-time monitoring for Proxmox

**Real-time monitoring dashboard for Proxmox VE and PBS with alerts and webhooks**

[![Version](https://img.shields.io/docker/v/rcourtman/pulse?sort=semver)](https://github.com/rcourtman/Pulse/releases)
[![Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![GitHub](https://img.shields.io/github/license/rcourtman/Pulse)](https://github.com/rcourtman/Pulse)

## Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Then open http://localhost:7655

## Features

- 📊 **Real-time Monitoring** - Live updates via WebSocket
- 🔔 **Smart Alerts** - Email, Discord, Slack, Telegram, Teams, ntfy.sh, Gotify
- 🎯 **Auto-Discovery** - Finds your Proxmox nodes automatically
- 🔒 **Secure** - Encrypted credential storage, read-only access
- 📱 **Mobile Friendly** - Responsive design works on any device
- 🌙 **Dark Mode** - Easy on the eyes
- 🚀 **Lightweight** - Single binary, minimal resource usage
- 🔄 **Auto-Updates** - Built-in update notifications

## What's Monitored

### Proxmox VE
- VMs and Containers (CPU, RAM, status)
- Storage pools and usage
- Backup jobs and status
- Cluster health

### Proxmox Backup Server
- Datastore usage
- Sync, verify, prune jobs
- Backup retention
- Garbage collection

## Configuration

### Environment Variables

```yaml
environment:
  - TZ=America/New_York           # Your timezone
  - API_TOKEN=your-secure-token   # Optional API authentication
  - FRONTEND_PORT=7655            # Change port if needed
```

### Docker Compose

```yaml
version: '3.8'
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - TZ=America/New_York
    restart: unless-stopped

volumes:
  pulse_data:
```

## Setup

1. **Open Pulse** at http://localhost:7655
2. **Go to Settings → Nodes**
3. **Click "Setup Script"** next to any discovered node
4. **Run the script** on your Proxmox node - it handles everything automatically

No manual token creation needed! The setup script:
- Creates a read-only monitoring user
- Sets proper permissions
- Generates API tokens
- Registers with Pulse

## Alert Configuration

Configure alerts in **Settings → Alerts**:

- **Email** - Gmail, Outlook, or any SMTP server
- **Discord** - Via webhooks
- **Slack** - Via webhooks  
- **Telegram** - Via bot API
- **Teams** - Via webhooks
- **ntfy.sh** - Self-hosted or cloud
- **Gotify** - Self-hosted notifications

Set thresholds for CPU, RAM, and storage - get notified before issues occur.

## Security

- **Encrypted Storage** - Credentials encrypted at rest (AES-256-GCM)
- **Read-Only Access** - Monitor without modification rights
- **API Authentication** - Optional token-based API security
- **Registration Tokens** - Secure node auto-registration

## Supported Architectures

Multi-architecture image supports:
- `linux/amd64` - Standard x86-64
- `linux/arm64` - ARM 64-bit (Raspberry Pi 4/5, Apple Silicon)
- `linux/arm/v7` - ARM 32-bit (older Raspberry Pi)

## Updating

```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
# Run the docker run command again
```

Or enable auto-updates with Watchtower.

## Links

- 📖 [Documentation](https://github.com/rcourtman/Pulse)
- 🐛 [Report Issues](https://github.com/rcourtman/Pulse/issues)
- 💬 [Discussions](https://github.com/rcourtman/Pulse/discussions)
- 📦 [GitHub Releases](https://github.com/rcourtman/Pulse/releases)

## Requirements

- Proxmox VE 7.0+ or PBS 2.0+
- Docker or Podman
- 1 CPU core, 256MB RAM minimum

## License

MIT License - See [LICENSE](https://github.com/rcourtman/Pulse/blob/main/LICENSE)

---

**Latest Version**: v4.2.1 | **Updated**: 2025-08-12