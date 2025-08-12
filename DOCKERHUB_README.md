# Pulse

Real-time monitoring dashboard for Proxmox Virtual Environment and Proxmox Backup Server.

## Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Access the dashboard at `http://localhost:7655`

## Overview

Pulse provides comprehensive monitoring for Proxmox environments with real-time updates, intelligent alerting, and automatic node discovery. Built with performance and security in mind, it requires minimal resources while delivering instant insights into your infrastructure.

### Key Features

- **Real-time Monitoring** - WebSocket-based live updates every 2 seconds
- **Multi-node Support** - Monitor multiple Proxmox VE and PBS instances
- **Intelligent Alerts** - Customizable thresholds with multiple notification channels
- **Automatic Discovery** - Detects nodes on your network automatically
- **Secure by Design** - Encrypted credential storage, read-only access
- **Responsive Interface** - Full mobile support with dark mode
- **Lightweight** - Single Go binary with embedded frontend
- **Auto-updates** - Built-in update notifications and one-click updates

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

## Initial Setup

1. Access Pulse at `http://localhost:7655`
2. Navigate to Settings → Nodes
3. Click "Setup Script" next to any discovered node
4. Execute the generated script on your Proxmox node

The setup script automatically:
- Creates a dedicated monitoring user with minimal permissions
- Configures API token authentication
- Registers the node with Pulse
- Applies security best practices

No manual configuration required.

## Alert Configuration

Pulse supports comprehensive alerting through multiple channels:

- **Email** (SMTP with STARTTLS/TLS support)
- **Discord** webhooks
- **Slack** webhooks
- **Telegram** bot API
- **Microsoft Teams** webhooks
- **ntfy.sh** (cloud or self-hosted)
- **Gotify** (self-hosted)

Configure thresholds for CPU, memory, and storage utilization to receive proactive notifications before issues impact your infrastructure.

## Security

Pulse implements multiple security layers:

- Encrypted credential storage using AES-256-GCM
- Read-only access to Proxmox APIs
- Optional API token authentication
- Registration tokens for controlled node enrollment
- No root access required

## Architecture Support

This image supports multiple architectures:

- `linux/amd64` - Intel/AMD 64-bit processors
- `linux/arm64` - ARM 64-bit (Raspberry Pi 4/5, AWS Graviton)
- `linux/arm/v7` - ARM 32-bit (Raspberry Pi 2/3)

## Updates

To update to the latest version:

```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
# Re-run the docker run command
```

Alternatively, use Watchtower for automatic updates.

## Documentation

- [GitHub Repository](https://github.com/rcourtman/Pulse)
- [Release Notes](https://github.com/rcourtman/Pulse/releases)
- [Issue Tracker](https://github.com/rcourtman/Pulse/issues)
- [API Documentation](https://github.com/rcourtman/Pulse/blob/main/docs/API.md)

## System Requirements

- Proxmox VE 7.0+ or PBS 2.0+
- 256MB RAM minimum
- 1 CPU core
- Docker 20.10+ or Podman

## License

MIT License

## Support

For bugs and feature requests, please use the [GitHub issue tracker](https://github.com/rcourtman/Pulse/issues). For questions and discussions, visit the [GitHub Discussions](https://github.com/rcourtman/Pulse/discussions) page.