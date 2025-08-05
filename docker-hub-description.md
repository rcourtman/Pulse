# Pulse for Proxmox

Real-time monitoring for Proxmox VE and PBS with alerts, webhooks, and a clean web interface.

## ⚠️ IMPORTANT: Upgrading from v3?

**DO NOT pull latest if running v3!** Pulse v4 is a complete rewrite from Node.js to Go. Automatic upgrades will break your installation.

See the [Migration Guide](https://github.com/rcourtman/Pulse/blob/main/docs/MIGRATION_V3_TO_V4.md) for upgrade instructions.

## Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse-data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Access at: `http://localhost:7655`

## Key Changes in v4
- Port changed: 3000 → **7655**
- Complete rewrite in Go (no Node.js)
- Configuration via web UI (no .env files)
- Improved performance and lower resource usage

## Environment Variables (Optional)

Configure initial node via environment variables:
- `PROXMOX_HOST` - PVE host (e.g., https://192.168.1.100:8006)
- `PROXMOX_USER` - API user (e.g., monitor@pve)
- `PROXMOX_TOKEN_NAME` - API token name
- `PROXMOX_TOKEN_VALUE` - API token secret

Or configure everything through the web UI after starting.

## Documentation

- [GitHub Repository](https://github.com/rcourtman/Pulse)
- [Migration Guide (v3→v4)](https://github.com/rcourtman/Pulse/blob/main/docs/MIGRATION_V3_TO_V4.md)
- [Latest Release](https://github.com/rcourtman/Pulse/releases/latest)