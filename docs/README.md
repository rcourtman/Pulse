# 📚 Pulse Documentation

Welcome to the Pulse documentation portal. Here you'll find everything you need to install, configure, and master Pulse.

---

## 🚀 Getting Started

- **[Installation Guide](INSTALL.md)**
  Step-by-step guides for Docker, Kubernetes, and bare metal.
- **[Configuration](CONFIGURATION.md)**  
  Learn how to configure authentication, notifications (Email, Discord, etc.), and system settings.
- **[Deployment Models](DEPLOYMENT_MODELS.md)**  
  Where config lives, how updates work, and what differs per deployment.
- **[Migration Guide](MIGRATION.md)**  
  Moving to a new server? Here's how to export and import your data safely.
- **[Upgrade to v5](UPGRADE_v5.md)**  
  Practical upgrade guidance and post-upgrade checks.
- **[FAQ](FAQ.md)**  
  Common questions and quick answers.

## 🛠️ Deployment & Operations

- **[Docker Guide](DOCKER.md)** – Advanced Docker & Compose configurations.
- **[Kubernetes](KUBERNETES.md)** – Helm charts, ingress, and HA setups.
- **[Reverse Proxy](REVERSE_PROXY.md)** – Nginx, Caddy, Traefik, and Cloudflare Tunnel recipes.
- **[Troubleshooting](TROUBLESHOOTING.md)** – Deep dive into common issues and logs.

## 🔐 Security

- **[Security Policy](../SECURITY.md)** – The core security model (Encryption, Auth, API Scopes).

## ✨ New in 5.0

- **[Pulse AI](AI.md)** – Optional assistant for chat, patrol findings, alert analysis, and execution workflows.
- **[Metrics History](METRICS_HISTORY.md)** – Persistent metrics storage with configurable retention.
- **[Mail Gateway](MAIL_GATEWAY.md)** – Proxmox Mail Gateway (PMG) monitoring.
- **[Auto Updates](AUTO_UPDATE.md)** – One-click updates for supported deployments.
- **[Kubernetes](KUBERNETES.md)** – Helm deployment (ingress, persistence, HA patterns).

## 📡 Monitoring & Agents

- **[Unified Agent](UNIFIED_AGENT.md)** – Single binary for Host and Docker monitoring.
- **[VM Disk Monitoring](VM_DISK_MONITORING.md)** – Enabling QEMU Guest Agent for disk stats.
- **[Temperature Monitoring](TEMPERATURE_MONITORING.md)** – Setting up hardware sensors.
- **[Webhooks](WEBHOOKS.md)** – Custom notification payloads.

## 💻 Development

- **[API Reference](API.md)** – Complete REST API documentation.
- **[Dev Quick Start](../DEV-QUICK-START.md)** – Hot-reload workflow for contributors.
- **[Architecture](../ARCHITECTURE.md)** – System design and component interaction.
- **[Contributing](../CONTRIBUTING.md)** – How to contribute to Pulse.
- **[Mock Mode](development/MOCK_MODE.md)** – Developing without real infrastructure.

---

<div align="center">
  <p>Found a bug or have a suggestion?</p>
  <a href="https://github.com/rcourtman/Pulse/issues">
    <img src="https://img.shields.io/badge/GitHub-Issues-green" alt="GitHub Issues" />
  </a>
</div>
