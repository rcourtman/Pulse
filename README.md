<h1 align="center">
  <img src="docs/images/pulse-logo.svg" alt="Pulse Logo" width="32" style="vertical-align: middle;" /> Pulse
</h1>

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)
[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)

**Real-time monitoring for Proxmox VE, Proxmox Mail Gateway, PBS, and Docker infrastructure with alerts and webhooks.**

Monitor your hybrid Proxmox and Docker estate from a single dashboard. Get instant alerts when nodes go down, containers misbehave, backups fail, or storage fills up. Supports email, Discord, Slack, Telegram, and more.

**[Try the live demo →](https://demo.pulserelay.pro)** (read-only with mock data, login: `demo` / `demo`)

Pulse is maintained by one person after work. [Sponsorships](https://github.com/sponsors/rcourtman) keep the servers online and free up time for bug fixes and new features.

**[Full documentation →](docs/README.md)**

## Table of Contents

- [Why Pulse?](#why-pulse)
- [Features](#features)
- [Privacy](#privacy)
- [Install Options](#install-options-overview)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Security](#security)
- [Updates](#updates)
- [API](#api)
- [Troubleshooting](#troubleshooting)
- [Documentation](#documentation)
- [Development](#development)
- [Visual Tour](#visual-tour)
- [Support Pulse Development](#support-pulse-development)
- [Links](#links)
- [License](#license)

## Why Pulse?

- **Who it’s for** – Homelab admins juggling multiple PVE nodes, sysadmins consolidating PBS/PMG telemetry, and MSPs watching mixed Proxmox + Docker estates who want a single pane with Proxmox-aware alerts.
- **What it solves** – Pulse pulls together VE, PBS, PMG, host agents, and Docker metadata so you can see node health, backups, storage, and notifications without flipping between UIs. It discovers new nodes, enforces scoped credentials, and keeps secrets server-side.
- **Where it fits** – Pulse complements the native Proxmox UI and tools like Zabbix/Netdata by focusing on cross-platform dashboards, webhook-rich alerting, and agent coverage. Keep using those tools for deep cluster admin or infrastructure metrics; add Pulse when you want shared alerting workflows, multi-site visibility, and Proxmox-fluent onboarding scripts.
- **When to reach for Pulse**
  - You need one dashboard for Proxmox VE, PBS, PMG, Docker, and standalone hosts.
  - You want alert policies with hysteresis, webhooks, and scoped API tokens.
  - You prefer onboarding scripts that apply least-privilege roles automatically.
  - You’re supporting teams who read webhooks (Discord, Slack, Teams, etc.) instead of email-only notifications.

<img width="2872" height="1502" alt="Pulse dashboard showing Proxmox cluster health, node metrics, and alert timeline" src="https://github.com/user-attachments/assets/41ac125c-59e3-4bdc-bfd2-e300109aa1f7" />

## Features

- **Auto-Discovery**: Infers subnets from local bridges/OVS, then scans those ranges (with a small RFC1918 `/24` fallback) to find Proxmox nodes; one-liner scripts onboard anything it finds
- **Cluster Support**: Configure one node, monitor entire cluster
- **Security defaults**:
  - Credentials encrypted at rest, masked in logs, never sent to frontend
  - CSRF protection for all state-changing operations
  - Rate limiting (500 req/min general, 10 attempts/min for auth)
  - Account lockout after failed login attempts
  - Secure session management with HttpOnly cookies
  - bcrypt password hashing (cost 12) - passwords NEVER stored in plain text
  - API tokens stored securely with restricted file permissions
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
  - **Temperature proxy safety notes** ([2025-11-07 audit](docs/SECURITY_AUDIT_2025-11-07.md)):
    - SSH keys isolated on host (never in containers)
    - SSRF protection via node allowlists
    - DoS-resistant with connection deadlines
    - Container-aware rate limiting
    - Capability-based authorization
- Live monitoring of VMs, containers, nodes, storage
- **Smart Alerts**: Email and webhooks (Discord, Slack, Telegram, Teams, ntfy.sh, Gotify)
  - Example: "VM 'webserver' is down on node 'pve1'"
  - Example: "Storage 'local-lvm' at 85% capacity"
  - Example: "VM 'database' is back online"
- **Adaptive Thresholds**: Hysteresis-based trigger/clear levels, fractional network thresholds, per-metric search, reset-to-defaults, and Custom overrides with inline audit trail
- **Alert Timeline Analytics**: Rich history explorer with acknowledgement/clear markers, escalation breadcrumbs, and quick filters for noisy resources
- **Ceph Awareness**: Surface Ceph health, pool utilisation, and daemon status automatically when Proxmox exposes Ceph-backed storage
- Unified view of PBS backups, PVE backups, and snapshots
- **Interactive Backup Explorer**: Cross-highlighted bar chart + grid with quick time-range pivots (7d/30d/90d/1y) and contextual tooltips for the busiest jobs
- Proxmox Mail Gateway analytics: mail volume, spam/virus trends, quarantine health, and cluster node status
- Optional Docker container monitoring via lightweight agent
- Standalone host agent for Linux, macOS, and Windows servers to capture uptime, OS metadata, and capacity metrics
- Config export/import with encryption and authentication
- Optional auto-updates with rollback helpers (systemd installs only; containers redeploy tags)
- Dark/light themes, responsive design
- Built with Go for minimal resource usage

[Screenshots →](docs/SCREENSHOTS.md)

## Support Pulse Development

The big expense isn’t infrastructure, it’s the AI tooling that helps keep up with bug reports and feature work. Monthly costs look like this:

- `demo.pulserelay.pro` domain + cert: ~£10/year (about £0.80/month)
- Hetzner VPS for the public demo: £6/month
- Development tooling and services: ~£200/month

So keeping Pulse healthy costs about £206/month (~$260 USD) plus 10–15 hours of evenings each week. Sponsorships cover those bills and let me spend that time coding instead of freelancing. If Pulse saves you time, please consider helping keep it sustainable:

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

**Not ready to sponsor?** Star the repo, share it with your homelab friends, or send a bug report—that feedback keeps the project moving.

## Privacy

**Pulse respects your privacy:**
- No telemetry or analytics collection
- No phone-home functionality
- No external API calls (except for configured webhooks)
- All data stays on your server
- Open source - verify it yourself

Your infrastructure data is yours alone.

## Install Options Overview

| Method | Best For | Prerequisites | Quick Start Link |
|--------|----------|---------------|------------------|
| LXC installer (one-liner) | Proxmox users who want Pulse inside a managed container | Proxmox VE node with container storage, curl | `install.sh` script (see [Quick Start](#quick-start)) |
| Docker container | Users standardizing on Docker/Podman or composing Pulse into existing stacks | Docker Engine or Podman, persistent volume | [Quick Start](#quick-start) / [Docker](#docker) |
| Kubernetes/Helm | Clusters needing HA, ingress, GitOps | Kubernetes cluster with storage class + Helm 3 | [docs/KUBERNETES.md](docs/KUBERNETES.md) |
| Bare metal/systemd | Minimal installs or environments without containers | Go-supported Linux host, systemd access | [docs/INSTALL.md](docs/INSTALL.md) and `scripts/build-release.sh` |

## Quick Start

### Install

```bash
# Recommended: Official installer (auto-detects Proxmox and creates container)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash

# Need to roll back to a previous release? Pass the tag you want
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.20.0

# Alternative: Docker
docker run -d -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest

# Testing: Install from main branch source (for testing latest fixes)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --source

# Alternative: Kubernetes (Helm)
helm registry login ghcr.io
helm install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version $(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/VERSION) \
  --namespace pulse \
  --create-namespace
# Replace the VERSION lookup with a specific release if you need to pin. For local development, see docs/KUBERNETES.md.
```

**Proxmox users**: The installer detects PVE hosts and automatically creates an optimized LXC container. Choose Quick mode for one-minute setup.

[Advanced installation options →](docs/INSTALL.md)

### Updating

**Automatic Updates (New!, systemd installs only):** Enable during installation or via Settings UI to stay current automatically  
**Standard Install:** Re-run the installer  
**Docker:** `docker pull rcourtman/pulse:latest` then recreate container

### Initial Setup

**Option A: Interactive Setup (UI)**
1. On the host, read the bootstrap token that protects the first-time setup screen:
   - Standard install: `cat /etc/pulse/.bootstrap_token`
   - Docker/Helm: `cat /data/.bootstrap_token` inside the container or mounted volume
2. Open `http://<your-server>:7655`
3. When prompted, paste the bootstrap token to unlock the wizard, then **complete the mandatory security setup** (first-time only)
4. Create your admin username and password and store the generated API token safely (the token file is removed after setup)
5. Use **Settings → Security → API tokens** to mint dedicated tokens for automation. Assign scopes so each token only has the permissions it needs (e.g. `docker:report`, `host-agent:report`). Legacy tokens default to full access until you edit and save new scopes.

**Option B: Automated Setup (No UI)**
For automated deployments, configure authentication via environment variables:
```bash
# Start Pulse with auth pre-configured - skips setup screen
API_TOKENS="ansible-token,docker-agent-token" ./pulse

# Or use basic auth
PULSE_AUTH_USER=admin PULSE_AUTH_PASS=password ./pulse

# Plain text credentials are automatically hashed for security
# `API_TOKEN` is still accepted for back-compat, but `API_TOKENS` lets you manage multiple credentials
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
5. Copy and run the provided one-liner on your Proxmox/PBS host (the script prompts for your setup token securely)
6. Node is configured and monitoring starts automatically

**Example:**
```bash
curl -sSL "http://pulse:7655/api/setup-script?type=pve&host=https://pve:8006" | bash
```
> Tip: For non-interactive installs, export `PULSE_SETUP_TOKEN` before running the script or supply `auth_token=YOUR_API_TOKEN` as shown in Method 2.

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

### Monitor Docker Containers (optional)

Deploy the lightweight [Pulse Docker agent](docs/DOCKER_MONITORING.md) on any host running Docker to stream container status and resource data back to Pulse. Install the agent alongside your stack, point it at your Pulse URL and API token, and the **Docker** workspace lights up with host summaries, restart loop detection, per-container CPU/memory charts, and quick filters for stacks and unhealthy workloads.

### Monitor Standalone Servers (optional)

Install the [Pulse host agent](docs/HOST_AGENT.md) on Linux, macOS, or Windows machines that sit outside your Proxmox or Docker estate. Generate an API token scoped to `host-agent:report`, drop it into the install command, and the **Servers** workspace will populate with uptime, OS metadata, and capacity metrics.

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
- 192.168.1.0/24 (common home gateway range)
- 192.168.0.0/24 (legacy home networks)
- 192.168.2.0/24 (consumer mesh gear)
- 10.0.0.0/24 (lab/private networks)
- 172.16.0.0/24 (Docker/internal bridge defaults)

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
  -e API_TOKENS="ansible-token,docker-agent-token" \
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
      # - API_TOKENS=token-a,token-b        # Comma-separated tokens (plain or SHA3-256 hashed)
      # - API_TOKEN=legacy-token            # Optional single-token fallback
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

- **Authentication required** protects your Proxmox infrastructure credentials; the quick setup wizard locks down a new install in under a minute.
- **Multiple auth methods**: Password authentication, API tokens, proxy auth (SSO), or combinations with Authentik/Authelia ([docs](docs/PROXY_AUTH.md)).
- **Setup script authentication**:
  - **Setup codes**: Temporary 6-character codes for manual setup (expire in 5 minutes)
  - **API tokens**: Permanent tokens for automation and scripting
  - Use setup codes when delegating access without exposing long-lived tokens; use scoped API tokens for automation.
- **Enterprise-grade protection**:
  - Credentials encrypted at rest (AES-256-GCM), masked in logs, and never sent to the frontend
  - CSRF protection for all state-changing operations
  - Rate limiting (500 req/min general, 10 attempts/min for auth) and account lockout after failed login attempts
  - Secure session management with HttpOnly cookies
  - bcrypt password hashing (cost 12) - passwords NEVER stored in plain text
  - API tokens stored securely with restricted file permissions
  - Security headers (CSP, X-Frame-Options, etc.)
  - Comprehensive audit logging
- **Other safety notes**:
  - Frontend never receives node credentials
  - API tokens visible only to authenticated users
  - Export/import requires authentication when configured

See [Security Documentation](docs/SECURITY.md) and [security audits](docs/SECURITY_AUDIT_2025-11-07.md) for details.

## Updates

Pulse checks for updates, surfaces notifications in the UI, and always requires a deliberate update so you stay in control of maintenance windows. The optional auto-update timer/service applies only to systemd installs; Docker/Helm deployments continue to update by redeploying the desired image/tag manually.

### Manual Installation (systemd)
```bash
# Update to latest stable
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash

# Update to latest RC/pre-release  
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --rc

# Install specific version
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.8.0-rc.1
```

### Docker
```bash
# Latest stable
docker pull rcourtman/pulse:latest

docker stop pulse
docker rm pulse
# Run docker run command again with your settings

# Latest RC
docker pull rcourtman/pulse:rc

# Specific version (example: stable tag)
docker pull rcourtman/pulse:v4.27.2
```

The UI detects your deployment type and reminds you which command sequence to run when a new version is available.

## Configuration

Quick start - most settings are in the web UI:
- **Settings → Nodes**: Add/remove Proxmox instances
- **Settings → System**: Polling intervals, timeouts, update settings
- **Settings → Security**: Authentication and API tokens
- **Alerts**: Thresholds and notifications

### Apprise Notifications

Pulse can broadcast grouped alerts through [Apprise](https://github.com/caronc/apprise) using either the local CLI or a remote Apprise API gateway. Configure everything under **Alerts → Notifications → Apprise**.

- **Local CLI** – Install Apprise on the Pulse host (for example `pip install apprise`) and enter one Apprise URL per line in the delivery targets field. You can override the CLI path and timeout if the executable lives outside of `$PATH`. Pulse skips CLI delivery automatically when no targets are configured.
- **Remote API** – Point Pulse at an Apprise API server by providing the base URL (such as `https://apprise-api.local:8000`). Optionally include a configuration key (for `/notify/{key}` routes), an API key header/value pair, and allow self-signed certificates for lab deployments. Targets remain optional in API mode—leave the list empty to let the Apprise server use its stored defaults.

For both modes, delivery targets accept any Apprise URL (Discord, Slack, email, SMS, etc.). The timeout applies to the CLI process or HTTP request respectively.

### Configuration Files

Pulse persists different domains into dedicated files so secrets and policies stay isolated:
- `.env` – Bootstrap authentication credentials created via the setup wizard.
- `system.json` – Runtime settings (polling, discovery, updates, feature flags).
- `nodes.enc` – Encrypted node credentials (AES-256-GCM).
- `alerts.json` – Thresholds, overrides, quiet hours, escalation schedule.
- `email.enc` / `webhooks.enc` / `apprise.enc` – Notification secrets per channel.
- `oidc.enc` – OIDC client configuration when SSO is enabled.
- `api_tokens.json` – Hashed API token metadata (scopes, hints, timestamps).

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
# Systemd (service: pulse; legacy installs may use pulse-backend): sudo systemctl edit pulse
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
# Systemd (service: pulse; legacy installs may use pulse-backend): sudo systemctl edit pulse
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

### VM Disk Stats Show "-"
- VMs require QEMU Guest Agent to report disk usage (Proxmox API returns 0 for VMs)
- Install guest agent in VM: `apt install qemu-guest-agent` (Linux) or virtio-win tools (Windows)
- Enable in VM Options → QEMU Guest Agent, then restart VM
- See [VM Disk Monitoring Guide](docs/VM_DISK_MONITORING.md) for setup
- Container (LXC) disk stats always work (no guest agent needed)

### Connection Issues
- Check Proxmox API is accessible (port 8006/8007)
- Verify credentials have PVEAuditor role plus VM.GuestAgent.Audit (PVE 9) or VM.Monitor (PVE 8); the setup script applies these via the PulseMonitor role (adds Sys.Audit when available)
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

## Development

### Quick Start - Hot Reload (Recommended)
```bash
# Launch Vite + Go with automatic frontend proxying
make dev-hot
# Frontend HMR: http://127.0.0.1:5173
# Backend API:   http://127.0.0.1:7655 (served via the Go app)
# Ports come from FRONTEND_PORT/PULSE_DEV_API_PORT (loaded from .env*. Override there if you need a different port.)
```

The backend now detects `FRONTEND_DEV_SERVER` and proxies requests straight to the Vite dev server. Edit files under `frontend-modern/src/` and the browser refreshes instantly—no manual rebuilds or service restarts required. Use `CTRL+C` to stop both processes.

### Mock Mode - Develop Without Real Infrastructure

Work on Pulse without needing Proxmox servers! Mock mode generates realistic test data and auto-reloads when toggled. The `mock.env` configuration file is **included in the repository**, and the helper script below switches the backend between synthetic data and your local dev config.

```bash
./scripts/toggle-mock.sh on      # Enable mock mode (~7 nodes, ~90 guests)
./scripts/toggle-mock.sh off     # Return to local/dev data
./scripts/toggle-mock.sh status  # Show current mode + data dir
./scripts/toggle-mock.sh edit    # Edit mock.env (node counts, randomness, etc.)
```

The script rewrites `mock.env`, restarts the Go backend, and flips `PULSE_DATA_DIR` so datasets stay isolated:

- Mock mode: `/opt/pulse/tmp/mock-data`
- Standard dev mode: `/opt/pulse/tmp/dev-config` (keeps `/etc/pulse` untouched)

Create personal overrides with `cp mock.env mock.env.local` and rerun `toggle-mock.sh` to apply them.

See [docs/development/MOCK_MODE.md](docs/development/MOCK_MODE.md) for full details.

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
