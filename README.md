# Pulse

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

**Real-time monitoring for Proxmox VE and PBS with alerts and webhooks.**

Monitor your entire Proxmox infrastructure from a single dashboard. Get instant alerts when nodes go down, backups fail, or storage fills up. Supports email, Discord, Slack, Telegram, and more.

![Dashboard](docs/images/01-dashboard.png)

## Support Pulse Development

Pulse is built by a solo developer in evenings and weekends. Your support helps:
- Keep me motivated to add new features
- Prioritize bug fixes and user requests
- Ensure Pulse stays 100% free and open-source forever

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?style=social&label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

**Not ready to sponsor?** Star the project or share it with your homelab community!

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
  - API tokens stored securely with restricted file permissions
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
- Live monitoring of VMs, containers, nodes, storage
- **Smart Alerts**: Email and webhooks (Discord, Slack, Telegram, Teams, ntfy.sh, Gotify)
  - Example: "VM 'webserver' is down on node 'pve1'"
  - Example: "Storage 'local-lvm' at 85% capacity"
  - Example: "VM 'database' is back online"
- Unified view of PBS backups, PVE backups, and snapshots
- Config export/import with encryption and authentication
- Automatic stable updates with safe rollback (opt-in)
- Dark/light themes, responsive design
- Built with Go for minimal resource usage

[Screenshots →](docs/SCREENSHOTS.md)

## Privacy

**Pulse respects your privacy:**
- No telemetry or analytics collection
- No phone-home functionality
- No external API calls (except for configured webhooks)
- All data stays on your server
- Open source - verify it yourself

Your infrastructure data is yours alone.

## Quick Start

### Install

```bash
# Recommended: Official installer (auto-detects Proxmox and creates container)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash

# Alternative: Docker
docker run -d -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest

# Testing: Install from main branch source (for testing latest fixes)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --main
```

**Proxmox users**: The installer detects PVE hosts and automatically creates an optimized LXC container. Choose Quick mode for one-minute setup.

[Advanced installation options →](docs/INSTALL.md)

### Updating

**Automatic Updates (New!):** Enable during installation or via Settings UI to stay current automatically  
**Standard Install:** Re-run the installer  
**Docker:** `docker pull rcourtman/pulse:latest` then recreate container

### Initial Setup

**Option A: Interactive Setup (UI)**
1. Open `http://<your-server>:7655`
2. **Complete the mandatory security setup** (first-time only)
3. Create your admin username and password
4. Save the generated API token for automation

**Option B: Automated Setup (No UI)**
For automated deployments, configure authentication via environment variables:
```bash
# Start Pulse with auth pre-configured - skips setup screen
API_TOKEN=your-api-token ./pulse

# Or use basic auth
PULSE_AUTH_USER=admin PULSE_AUTH_PASS=password ./pulse

# Plain text credentials are automatically hashed for security
# You can also provide pre-hashed values if preferred
```
See [Configuration Guide](docs/CONFIGURATION.md#automated-setup-skip-ui) for details.

### Configure Nodes

**Two authentication methods available:**

#### Method 1: Manual Setup (Recommended for interactive use)
1. After login, go to Settings → Nodes
2. Discovered nodes appear automatically
3. Click "Setup Script" next to any node
4. Click "Generate Setup Code" button (creates a 6-character code valid for 5 minutes)
5. Copy and run the provided one-liner on your Proxmox/PBS host
6. Node is configured and monitoring starts automatically

**Example:**
```bash
curl -sSL "http://pulse:7655/api/setup-script?type=pve&host=https://pve:8006&auth_token=ABC123" | bash
```

#### Method 2: Automated Setup (For scripts/automation)
Use your permanent API token directly in the URL for automation:

```bash
# For Proxmox VE
curl -sSL "http://pulse:7655/api/setup-script?type=pve&host=https://pve:8006&auth_token=YOUR_API_TOKEN" | bash

# For Proxmox Backup Server
curl -sSL "http://pulse:7655/api/setup-script?type=pbs&host=https://pbs:8007&auth_token=YOUR_API_TOKEN" | bash
```

**Parameters:**
- `type`: `pve` for Proxmox VE, `pbs` for Proxmox Backup Server
- `host`: Full URL of your Proxmox/PBS server (e.g., https://192.168.1.100:8006)
- `auth_token`: Either a 6-character setup code (expires in 5 min) or your permanent API token
- `backup_perms=true` (optional): Add backup management permissions
- `pulse_url` (optional): Pulse server URL if different from where script is downloaded

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

### Network Discovery

Pulse automatically discovers Proxmox nodes on your network! By default, it scans:
- 192.168.0.0/16 (home networks)
- 10.0.0.0/8 (private networks)
- 172.16.0.0/12 (Docker/internal networks)

To scan a custom subnet instead:
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e DISCOVERY_SUBNET="192.168.50.0/24" \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Automated Deployment
```bash
# Deploy with authentication pre-configured
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e API_TOKEN="your-secure-token" \
  -e PULSE_AUTH_USER="admin" \
  -e PULSE_AUTH_PASS="your-password" \
  --restart unless-stopped \
  rcourtman/pulse:latest

# Plain text credentials are automatically hashed for security
# No setup required - API works immediately
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
      # NOTE: Env vars override UI settings. Remove env var to allow UI configuration.
      
      # Network discovery (usually not needed - auto-scans common networks)
      # - DISCOVERY_SUBNET=192.168.50.0/24  # Only for non-standard networks
      
      # Ports
      # - PORT=7655                         # Backend port (default: 7655)
      # - FRONTEND_PORT=7655                # Frontend port (default: 7655)
      
      # Security (all optional - runs open by default)
      # - PULSE_AUTH_USER=admin             # Username for web UI login
      # - PULSE_AUTH_PASS=your-password     # Plain text or bcrypt hash (auto-hashed if plain)
      # - API_TOKEN=your-token              # Plain text or SHA3-256 hash (auto-hashed if plain)
      # - ALLOW_UNPROTECTED_EXPORT=false    # Allow export without auth (default: false)
      
      # Security: Plain text credentials are automatically hashed
      # You can provide either:
      # 1. Plain text (auto-hashed): PULSE_AUTH_PASS=mypassword
      # 2. Pre-hashed (advanced): PULSE_AUTH_PASS='$$2a$$12$$...'
      #    Note: Escape $ as $$ in docker-compose.yml for pre-hashed values
      
      # Performance
      # - CONNECTION_TIMEOUT=10             # Connection timeout in seconds (default: 10)
      
      # CORS & logging
      # - ALLOWED_ORIGINS=https://app.example.com  # CORS origins (default: none, same-origin only)
      # - LOG_LEVEL=info                    # Log level: debug/info/warn/error (default: info)
    restart: unless-stopped

volumes:
  pulse_data:
```


## Security

- **Authentication required** - Protects your Proxmox infrastructure credentials
- **Quick setup wizard** - Secure your installation in under a minute
- **Multiple auth methods**: Password authentication, API tokens, proxy auth (SSO), or combinations
- **Proxy/SSO support** - Integrate with Authentik, Authelia, and other authentication proxies ([docs](docs/PROXY_AUTH.md))
- **Enterprise-grade protection**:
  - Credentials encrypted at rest (AES-256-GCM)
  - CSRF tokens for state-changing operations
  - Rate limiting and account lockout protection
  - Secure session management with HttpOnly cookies
  - bcrypt password hashing (cost 12) - passwords NEVER stored in plain text
  - API tokens stored securely with restricted file permissions
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
- **Security by design**:
  - Frontend never receives node credentials
  - API tokens visible only to authenticated users
  - Export/import requires authentication when configured

See [Security Documentation](docs/SECURITY.md) for details.

## Updating

### Update Notifications
Pulse checks for updates and displays notifications in the UI when new versions are available. For security reasons, updates must be installed manually using the appropriate method for your deployment.

### Manual Installation (systemd)
```bash
# Update to latest stable
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash

# Update to latest RC/pre-release  
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --rc

# Install specific version
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.8.0-rc.1
```

### Docker Updates
```bash
# Latest stable
docker pull rcourtman/pulse:latest

# Latest RC
docker pull rcourtman/pulse:rc

# Specific version
docker pull rcourtman/pulse:v4.8.0-rc.1
```

## Configuration

Quick start - most settings are in the web UI:
- **Settings → Nodes**: Add/remove Proxmox instances
- **Settings → System**: Polling intervals, timeouts, update settings
- **Settings → Security**: Authentication and API tokens
- **Alerts**: Thresholds and notifications

### Configuration Files

Pulse uses three separate configuration files with clear separation of concerns:
- `.env` - Authentication credentials only
- `system.json` - Application settings
- `nodes.enc` - Encrypted node credentials

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for detailed documentation on configuration structure and management.

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

### Alert Configuration

Pulse provides two complementary approaches for managing alerts:

#### Custom Alert Rules (Permanent Policy)
Configure persistent alert policies in **Settings → Alerts → Custom Rules**:
- Define thresholds for specific VMs/containers based on name patterns
- Set different thresholds for production vs development environments
- Create complex rules with AND/OR logic
- Manage all rules through the UI with priority ordering

**Use for:** Long-term alert policies like "all database VMs should alert at 90%"


### HTTPS/TLS Configuration
Enable HTTPS by setting these environment variables:
```bash
# Systemd: sudo systemctl edit pulse-backend
Environment="HTTPS_ENABLED=true"
Environment="TLS_CERT_FILE=/etc/pulse/cert.pem"
Environment="TLS_KEY_FILE=/etc/pulse/key.pem"

# Docker
docker run -d -p 7655:7655 \
  -e HTTPS_ENABLED=true \
  -e TLS_CERT_FILE=/data/cert.pem \
  -e TLS_KEY_FILE=/data/key.pem \
  -v pulse_data:/data \
  -v /path/to/certs:/data/certs:ro \
  rcourtman/pulse:latest
```

For deployment overrides (ports, etc), use environment variables:
```bash
# Systemd: sudo systemctl edit pulse-backend
Environment="FRONTEND_PORT=8080"

# Docker: -e FRONTEND_PORT=8080
```

**[Full Configuration Guide →](docs/CONFIGURATION.md)**

### Backup/Restore

**Via UI (recommended):**
- Settings → Security → Backup & Restore
- Export: Choose login password or custom passphrase for encryption
- Import: Upload backup file with passphrase
- Includes all settings, nodes, and custom console URLs

**Via CLI:**
```bash
# Export (v4.0.3+)
pulse config export -o backup.enc

# Import
pulse config import -i backup.enc
```

## Updates

Pulse shows when updates are available and provides deployment-specific instructions:

### Docker
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
# Run docker run command again with your settings
```

### Manual Install
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

The UI will detect your deployment type and show the appropriate update method when a new version is available.

## API

```bash
# Status
curl http://localhost:7655/api/health

# Metrics (default time range: 1h)
curl http://localhost:7655/api/charts

# With authentication (if configured)
curl -H "X-API-Token: your-token" http://localhost:7655/api/health
```

**[Full API Documentation →](docs/API.md)** - Complete endpoint reference with examples

## Reverse Proxy & SSO

Using Pulse behind a reverse proxy? **WebSocket support is required for real-time updates.**

**NEW: Proxy Authentication Support** - Integrate with Authentik, Authelia, and other SSO providers. See [Proxy Authentication Guide](docs/PROXY_AUTH.md).

See [Reverse Proxy Configuration Guide](docs/REVERSE_PROXY.md) for nginx, Caddy, Apache, Traefik, HAProxy, and Cloudflare Tunnel configurations.

## Troubleshooting

### Authentication Issues

#### Cannot login after setting up security
- **Docker**: Ensure bcrypt hash is exactly 60 characters and wrapped in single quotes
- **Docker Compose**: MUST escape $ characters as $$ (e.g., `$$2a$$12$$...`)
- **Example (docker run)**: `PULSE_AUTH_PASS='$2a$12$YTZXOCEylj4TaevZ0DCeI.notayQZ..b0OZ97lUZ.Q24fljLiMQHK'`
- **Example (docker-compose.yml)**: `PULSE_AUTH_PASS='$$2a$$12$$YTZXOCEylj4TaevZ0DCeI.notayQZ..b0OZ97lUZ.Q24fljLiMQHK'`
- If hash is truncated or mangled, authentication will fail
- Use Quick Security Setup in the UI to avoid manual configuration errors

#### .env file not created (Docker)
- **Expected behavior**: When using environment variables, no .env file is created in /data
- The .env file is only created when using Quick Security Setup or password changes
- If you provide credentials via environment variables, they take precedence
- To use Quick Security Setup: Start container WITHOUT auth environment variables

### VM Disk Stats Show 0% or Dash
- **Known Proxmox 9 limitation**: API tokens cannot access guest agent data
- See [VM Disk Stats Troubleshooting Guide](docs/VM_DISK_STATS_TROUBLESHOOTING.md) for details
- Container (LXC) disk stats work normally
- This is a Proxmox API limitation, not a Pulse bug

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

- [Docker Guide](docs/DOCKER.md) - Complete Docker deployment guide
- [Configuration Guide](docs/CONFIGURATION.md) - Complete setup and configuration
- [VM Disk Monitoring](docs/VM_DISK_MONITORING.md) - Set up QEMU Guest Agent for accurate VM disk usage
- [Port Configuration](docs/PORT_CONFIGURATION.md) - How to change the default port
- [Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues and solutions
- [API Reference](docs/API.md) - REST API endpoints and examples
- [Webhook Guide](docs/WEBHOOKS.md) - Setting up webhooks and custom payloads
- [Proxy Authentication](docs/PROXY_AUTH.md) - SSO integration with Authentik, Authelia, etc.
- [Reverse Proxy Setup](docs/REVERSE_PROXY.md) - nginx, Caddy, Apache, Traefik configs
- [Security](docs/SECURITY.md) - Security features and best practices
- [FAQ](docs/FAQ.md) - Common questions and troubleshooting
- [Migration Guide](docs/MIGRATION.md) - Backup and migration procedures

## Security

- **Mandatory authentication** protects your infrastructure
- Credentials stored encrypted (AES-256-GCM)
- API token support for automation
- Export/import requires authentication
- **Setup script authentication**:
  - **Setup codes**: Temporary 6-character codes for manual setup (expire in 5 minutes)
  - **API tokens**: Permanent tokens for automation and scripting
  - Use setup codes when giving access to others without sharing your API token
  - Use API tokens for your own automation or trusted environments
- [Security Details →](docs/SECURITY.md)

## Development

### Quick Start - Hot Reload (Recommended)
```bash
# Best development experience with instant frontend updates
./scripts/hot-dev.sh
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

## Visual Tour

See Pulse in action with our [complete screenshot gallery →](docs/SCREENSHOTS.md)

### Core Features

| Dashboard | Storage | Backups |
|-----------|---------|---------|
| ![Dashboard](docs/images/01-dashboard.png) | ![Storage](docs/images/02-storage.png) | ![Backups](docs/images/03-backups.png) |
| *Real-time monitoring of nodes, VMs & containers* | *Storage pool usage across all nodes* | *Unified backup management & PBS integration* |

### Alerts & Configuration

| Alert Configuration | Alert History | Settings |
|---------------------|---------------|----------|
| ![Alerts](docs/images/04-alerts.png) | ![Alert History](docs/images/05-alert-history.png) | ![Settings](docs/images/06-settings.png) |
| *Configure thresholds & notifications* | *Track patterns with visual timeline* | *Manage nodes & authentication* |

### Mobile Experience

| Mobile Dashboard |
|------------------|
| ![Mobile](docs/images/08-mobile.png) |
| *Fully responsive interface for monitoring on the go* |

## Links

- [Releases](https://github.com/rcourtman/Pulse/releases)
- [Docker Hub](https://hub.docker.com/r/rcourtman/pulse)
- [Issues](https://github.com/rcourtman/Pulse/issues)

## License

MIT - See [LICENSE](LICENSE)