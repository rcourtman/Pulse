# Pulse for Proxmox

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

**Real-time monitoring for Proxmox VE and PBS with alerts and webhooks.**

![Dashboard](docs/images/01-dashboard.png)

## 💖 Support This Project

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?style=social&label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## Features

- **Auto-Discovery**: Finds Proxmox nodes on your network, one-liner setup via generated scripts
- **Cluster Support**: Configure one node, monitor entire cluster
- **Enterprise Security**: 
  - Credentials encrypted at rest, masked in logs, never sent to frontend
  - CSRF protection for all state-changing operations
  - Rate limiting (500 req/min general, 10 attempts/min for auth)
  - Account lockout after failed login attempts
  - Secure session management with HttpOnly cookies
  - bcrypt password hashing (cost 12) - passwords NEVER stored in plain text
  - SHA3-256 API token hashing - tokens NEVER stored in plain text
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
- Live monitoring of VMs, containers, nodes, storage
- Alerts with email and webhooks (Discord, Slack, Telegram, Teams, ntfy.sh, Gotify)
- Unified view of PBS backups, PVE backups, and snapshots
- PBS push mode for firewalled servers
- Config export/import with encryption and authentication
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

### Configure Nodes

1. Open `http://<your-server>:7655`
2. Go to Settings → Nodes
3. Discovered nodes appear automatically
4. Click "Setup Script" next to any node
5. Run the generated one-liner on that node
6. Node is configured and monitoring starts

The script handles user creation, permissions, token generation, and registration automatically.

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
      # Network discovery
      # - DISCOVERY_SUBNET=192.168.1.0/24  # Auto-discovery subnet (default: auto-detect)
      
      # Ports
      # - PORT=7655                         # Backend port (default: 7655)
      # - FRONTEND_PORT=7655                # Frontend port (default: 7655)
      
      # Security (all optional - runs open by default)
      # - PULSE_AUTH_USER=admin             # Username for web UI login
      # - PULSE_AUTH_PASS='$2a$12$...'      # Bcrypt hashed password (use Quick Security Setup)
      # - API_TOKEN=<sha3-256-hash>         # SHA3-256 hashed API token (64 hex chars)
      # - ALLOW_UNPROTECTED_EXPORT=false    # Allow export without auth (default: false)
      
      # Polling & timeouts
      # - POLLING_INTERVAL=5                # Seconds between node checks (default: 5)
      # - CONNECTION_TIMEOUT=10             # Connection timeout in seconds (default: 10)
      
      # Updates
      # - UPDATE_CHANNEL=stable             # Update channel: stable or rc (default: stable)
      # - AUTO_UPDATE_ENABLED=true          # Enable auto-updates (default: true)
      # - AUTO_UPDATE_CHECK_INTERVAL=24     # Hours between update checks (default: 24)
      # - AUTO_UPDATE_TIME=03:00            # Time to install updates HH:MM (default: 03:00)
      
      # CORS & logging
      # - ALLOWED_ORIGINS=https://app.example.com  # CORS origins (default: none, same-origin only)
      # - LOG_LEVEL=info                    # Log level: debug/info/warn/error (default: info)
    restart: unless-stopped

volumes:
  pulse_data:
```


## PBS Agent (Push Mode)

For isolated PBS servers, see [PBS Agent documentation](docs/PBS-AGENT.md)

## Security

- **Authentication is optional** - Run open for homelab or secured for production
- **Multiple auth methods**: Password authentication, API tokens, or both
- **Enterprise-grade protection**:
  - Credentials encrypted at rest (AES-256-GCM)
  - CSRF tokens for state-changing operations
  - Rate limiting and account lockout protection
  - Secure session management with HttpOnly cookies
  - bcrypt password hashing (cost 12) - passwords NEVER stored in plain text
  - SHA3-256 API token hashing - tokens NEVER stored in plain text (cost 12)
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
- **Security by design**:
  - Frontend never receives node credentials
  - API tokens visible only to authenticated users
  - Export/import requires authentication when configured

See [Security Documentation](docs/SECURITY.md) for details.

## Configuration

Quick start - most settings are in the web UI:
- **Settings → Nodes**: Add/remove Proxmox instances
- **Settings → System**: Polling intervals, CORS settings
- **Alerts**: Thresholds and notifications

### Email Alerts Configuration
Configure email notifications in **Settings → Alerts → Email Destinations**

#### Supported Providers
- **Gmail/Google Workspace**: Requires app-specific password
- **Outlook/Office 365**: Requires app-specific password  
- **Custom SMTP**: Any SMTP server

#### Recommended Settings
- **Port 587 with STARTTLS** (recommended for most providers)
- **Port 465** for SSL/TLS
- **Port 25** for unencrypted (not recommended)

#### Gmail Setup
1. Enable 2-factor authentication
2. Generate app-specific password at https://myaccount.google.com/apppasswords
3. Use your email as username and app password as password
4. Server: smtp.gmail.com, Port: 587, Enable STARTTLS

#### Outlook Setup
1. Generate app password at https://account.microsoft.com/security
2. Use your email as username and app password as password
3. Server: smtp-mail.outlook.com, Port: 587, Enable STARTTLS

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
curl http://localhost:7655/api/health

# Metrics (default time range: 1h)
curl http://localhost:7655/api/charts

# With authentication (if configured)
curl -H "X-API-Token: your-token" http://localhost:7655/api/health
```

📖 **[Full API Documentation →](docs/API.md)** - Complete endpoint reference with examples

## Reverse Proxy

Using Pulse behind a reverse proxy? **WebSocket support is required for real-time updates.**

See [Reverse Proxy Configuration Guide](docs/REVERSE_PROXY.md) for nginx, Caddy, Apache, Traefik, HAProxy, and Cloudflare Tunnel configurations.

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

## Documentation

- [Configuration Guide](docs/CONFIGURATION.md) - Complete setup and configuration
- [API Reference](docs/API.md) - REST API endpoints and examples
- [Webhook Guide](docs/WEBHOOKS.md) - Setting up webhooks and custom payloads
- [Reverse Proxy Setup](docs/REVERSE_PROXY.md) - nginx, Caddy, Apache, Traefik configs
- [PBS Agent](docs/PBS-AGENT.md) - Monitoring isolated PBS servers
- [Security](docs/SECURITY.md) - Security features and best practices
- [FAQ](docs/FAQ.md) - Common questions and troubleshooting
- [Migration Guide](docs/MIGRATION.md) - Backup and migration procedures
- [v3 to v4 Upgrade](docs/MIGRATION_V3_TO_V4.md) - Upgrading from v3 to v4

## Security

- Credentials stored encrypted (AES-256-GCM)
- Optional API token authentication
- Export/import requires passphrase
- [Security Details →](docs/SECURITY.md)

## Development

### Quick Start - Hot Reload (Recommended)
```bash
# Best development experience with instant frontend updates
./hot-dev.sh
# Frontend: http://localhost:5173 (hot reload)
# Backend: http://localhost:7655
```

### Production-like Development
```bash
# Watches files and rebuilds/embeds frontend into Go binary
./dev.sh
# Access at: http://localhost:7655
```

### Manual Development
```bash
# Frontend only
cd frontend-modern
npm install
npm run dev

# Backend only
go build -o pulse ./cmd/pulse
./pulse

# Or use make for full rebuild
make dev
```

## Links

- [Releases](https://github.com/rcourtman/Pulse/releases)
- [Docker Hub](https://hub.docker.com/r/rcourtman/pulse)
- [Issues](https://github.com/rcourtman/Pulse/issues)

## License

MIT - See [LICENSE](LICENSE)