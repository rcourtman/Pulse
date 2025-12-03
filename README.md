<div align="center">
  <img src="docs/images/pulse-logo.svg" alt="Pulse Logo" width="120" />
  <h1>Pulse</h1>
  <p><strong>Real-time monitoring for Proxmox VE, Proxmox Mail Gateway, PBS, and Docker infrastructure.</strong></p>

  [![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
  [![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
  [![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)
  [![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)

  [Live Demo](https://demo.pulserelay.pro) • [Documentation](docs/README.md) • [Report Bug](https://github.com/rcourtman/Pulse/issues)
</div>

---

## 🚀 Overview

Pulse is a modern, unified dashboard for your **Proxmox** and **Docker** estate. It consolidates metrics, logs, and alerts from Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, and standalone Docker hosts into a single, beautiful interface.

Designed for homelabs, sysadmins, and MSPs who need a "single pane of glass" without the complexity of enterprise monitoring stacks.

![Pulse Dashboard](docs/images/01-dashboard.png)

## ✨ Features

- **Unified Monitoring**: View health and metrics for PVE, PBS, PMG, and Docker containers in one place.
- **Smart Alerts**: Get notified via Discord, Slack, Telegram, Email, and more when things go wrong (e.g., "VM down", "Storage full").
- **Auto-Discovery**: Automatically finds Proxmox nodes on your network.
- **Secure by Design**: Credentials encrypted at rest, no external dependencies, and strict API scoping.
- **Backup Explorer**: Visualize backup jobs and storage usage across your entire infrastructure.
- **Privacy Focused**: No telemetry, no phone-home, all data stays on your server.
- **Lightweight**: Built with Go and React, running as a single binary or container.

## ⚡ Quick Start

### Option 1: Proxmox LXC (Recommended)
Run this one-liner on your Proxmox host to create a lightweight LXC container:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

### Option 2: Docker
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Access the dashboard at `http://<your-ip>:7655`.

## 📚 Documentation

- **[Installation Guide](docs/INSTALL.md)**: Detailed instructions for Docker, Kubernetes, and bare metal.
- **[Configuration](docs/CONFIGURATION.md)**: Setup authentication, notifications, and advanced settings.
- **[Security](SECURITY.md)**: Learn about Pulse's security model and best practices.
- **[API Reference](docs/API.md)**: Integrate Pulse with your own tools.
- **[Architecture](ARCHITECTURE.md)**: High-level system design and data flow.
- **[Troubleshooting](docs/TROUBLESHOOTING.md)**: Solutions to common issues.

## 🌐 Community Integrations

Community-maintained integrations and addons:

- **[Home Assistant Addon](https://github.com/Kosztyk/pulse-docker-agent-addon)** - Run the Pulse Docker Agent as a Home Assistant addon.

## ❤️ Support Pulse Development

Pulse is maintained by one person. Sponsorships help cover the costs of the demo server, development tools, and domains. If Pulse saves you time, please consider supporting the project!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 📄 License

MIT © [Richard Courtman](https://github.com/rcourtman)
