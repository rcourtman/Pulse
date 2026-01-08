<div align="center">
  <img src="docs/images/pulse-logo.svg" alt="Pulse Logo" width="120" />
  <h1>Pulse</h1>
  <p><strong>Real-time monitoring for Proxmox, Docker, and Kubernetes infrastructure.</strong></p>

  [![GitHub Stars](https://img.shields.io/github/stars/rcourtman/Pulse?style=flat&logo=github)](https://github.com/rcourtman/Pulse)
  [![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
  [![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
  [![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

  [Live Demo](https://demo.pulserelay.pro) • [Pulse Pro](https://pulserelay.pro) • [Documentation](docs/README.md) • [Report Bug](https://github.com/rcourtman/Pulse/issues)
</div>

---

## 🚀 Overview

Pulse is a modern, unified dashboard for monitoring your **infrastructure** across Proxmox, Docker, and Kubernetes. It consolidates metrics, alerts, and AI-powered insights from all your systems into a single, beautiful interface.

Designed for homelabs, sysadmins, and MSPs who need a "single pane of glass" without the complexity of enterprise monitoring stacks.

![Pulse Dashboard](docs/images/01-dashboard.jpg)

## ✨ Features

### Core Monitoring
- **Unified Monitoring**: View health and metrics for PVE, PBS, PMG, Docker, and Kubernetes in one place
- **Smart Alerts**: Get notified via Discord, Slack, Telegram, Email, and more
- **Auto-Discovery**: Automatically finds Proxmox nodes on your network
- **Metrics History**: Persistent storage with configurable retention
- **Backup Explorer**: Visualize backup jobs and storage usage

### AI-Powered
- **Chat Assistant**: Ask questions about your infrastructure in natural language
- **Patrol**: Background health checks that generate findings on a schedule
- **Alert Analysis**: Optional AI analysis when alerts fire
- **Cost Tracking**: Track usage and costs per provider/model

### Multi-Platform
- **Proxmox VE/PBS/PMG**: Full monitoring and management
- **Kubernetes**: Complete K8s cluster monitoring via agents
- **Docker/Podman**: Container and Swarm service monitoring
- **OCI Containers**: Proxmox 9.1+ native container support

### Security & Operations
- **Secure by Design**: Credentials encrypted at rest, strict API scoping
- **One-Click Updates**: Easy upgrades for supported deployments
- **OIDC/SSO**: Enterprise authentication support
- **Privacy Focused**: No telemetry, all data stays on your server

## ⚡ Quick Start

### Option 1: Proxmox LXC (Recommended)
Run this one-liner on your Proxmox host to create a lightweight LXC container:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

Note: this installs the Pulse **server**. Agent installs use the command generated in **Settings → Agents → Installation commands** (served from `/install.sh` on your Pulse server).

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
- **[Agent Security](docs/AGENT_SECURITY.md)**: Details on checksum-verified updates and verification.
- **[Docker Monitoring](docs/DOCKER.md)**: Setup and management of Docker agents.

## 🌐 Community Integrations

Community-maintained integrations and addons:

- **[Home Assistant Addons](https://github.com/Kosztyk/homeassistant-addons)** - Run Pulse Agent and Pulse Server as Home Assistant addons.

## 🚀 Pulse Pro

**[Pulse Pro](https://pulserelay.pro)** unlocks **LLM-backed AI Patrol** — automated background monitoring that catches issues before they become outages.

| Feature | Free | Pro |
|---------|------|-----|
| Real-time dashboard | ✅ | ✅ |
| Threshold alerts | ✅ | ✅ |
| AI Chat (BYOK) | ✅ | ✅ |
| Heuristic Patrol (local rules) | ✅ | ✅ |
| **LLM-backed AI Patrol** | — | ✅ |
| Alert-triggered AI analysis | — | ✅ |
| Kubernetes AI analysis | — | ✅ |
| Auto-fix + autonomous mode | — | ✅ |
| Centralized agent profiles | — | ✅ |
| Priority support | — | ✅ |

AI Patrol runs on your schedule (every 10 minutes to every 7 days, default 6 hours) and finds:
- ZFS pools approaching capacity
- Backup jobs that silently failed
- VMs stuck in restart loops
- Clock drift across cluster nodes
- Container health check failures

Technical highlights:
- Cross-system context (nodes, VMs, backups, containers, and metrics history)
- LLM analysis for high-impact findings + alert-triggered deep dives
- Optional auto-fix with command safety policies and audit trail
- Centralized agent profiles for consistent fleet settings

**[Try the live demo →](https://demo.pulserelay.pro)** or **[learn more at pulserelay.pro](https://pulserelay.pro)**

Pulse Pro technical details: [docs/PULSE_PRO.md](docs/PULSE_PRO.md)

## ❤️ Support Pulse Development

Pulse is maintained by one person. Sponsorships help cover the costs of the demo server, development tools, and domains. If Pulse saves you time, please consider supporting the project!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 📄 License

MIT © [Richard Courtman](https://github.com/rcourtman). Use of Pulse Pro is subject to the [Terms of Service](TERMS.md).
