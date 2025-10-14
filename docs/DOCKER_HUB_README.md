# Pulse

[![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

**Real-time monitoring for Proxmox VE, Proxmox Mail Gateway, PBS, and Docker infrastructure with alerts and webhooks.**

Monitor your hybrid Proxmox and Docker estate from a single dashboard. Get instant alerts when nodes go down, containers misbehave, backups fail, or storage fills up. Supports email, Discord, Slack, Telegram, and more.

**[Try the live demo →](https://demo.pulserelay.pro)** (read-only with mock data)

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
- **Adaptive Thresholds**: Hysteresis-based trigger/clear levels, fractional network thresholds, per-metric search, reset-to-defaults, and Custom overrides with inline audit trail
- **Alert Timeline Analytics**: Rich history explorer with acknowledgement/clear markers, escalation breadcrumbs, and quick filters for noisy resources
- **Ceph Awareness**: Surface Ceph health, pool utilisation, and daemon status automatically when Proxmox exposes Ceph-backed storage
- Unified view of PBS backups, PVE backups, and snapshots
- **Interactive Backup Explorer**: Cross-highlighted bar chart + grid with quick time-range pivots (24h/7d/30d/custom) and contextual tooltips for the busiest jobs
- Proxmox Mail Gateway analytics: mail volume, spam/virus trends, quarantine health, and cluster node status
- Optional Docker container monitoring via lightweight agent
- Config export/import with encryption and authentication
- Automatic stable updates with safe rollback (opt-in)
- Dark/light themes, responsive design
- Built with Go for minimal resource usage

[View screenshots and full documentation on GitHub →](https://github.com/rcourtman/Pulse)

## Privacy

**Pulse respects your privacy:**
- No telemetry or analytics collection
- No phone-home functionality
- No external API calls (except for configured webhooks)
- All data stays on your server
- Open source - verify it yourself

Your infrastructure data is yours alone.

## Quick Start with Docker

### Basic Setup

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Then open `http://localhost:7655` and complete the security setup wizard.

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

### Automated Deployment with Pre-configured Auth

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

## Initial Setup

1. Open `http://<your-server>:7655`
2. **Complete the mandatory security setup** (first-time only)
3. Create your admin username and password
4. Use **Settings → Security → API tokens** to issue dedicated tokens for automation (one token per integration makes revocation painless)

## Configure Proxmox/PBS Nodes

After logging in:

1. Go to Settings → Nodes
2. Discovered nodes appear automatically
3. Click "Setup Script" next to any node
4. Click "Generate Setup Code" button (creates a 6-character code valid for 5 minutes)
5. Copy and run the provided one-liner on your Proxmox/PBS host
6. Node is configured and monitoring starts automatically

**Example setup command:**
```bash
curl -sSL "http://pulse:7655/api/setup-script?type=pve&host=https://pve:8006&auth_token=ABC123" | bash
```

## Docker Updates

```bash
# Latest stable
docker pull rcourtman/pulse:latest

# Latest RC/pre-release
docker pull rcourtman/pulse:rc

# Specific version
docker pull rcourtman/pulse:v4.22.0

# Then recreate your container
docker stop pulse && docker rm pulse
# Run your docker run or docker-compose command again
```

## Security

- **Authentication required** - Protects your Proxmox infrastructure credentials
- **Quick setup wizard** - Secure your installation in under a minute
- **Multiple auth methods**: Password authentication, API tokens, proxy auth (SSO), or combinations
- **Proxy/SSO support** - Integrate with Authentik, Authelia, and other authentication proxies
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

See [Security Documentation](https://github.com/rcourtman/Pulse/blob/main/docs/SECURITY.md) for details.

## HTTPS/TLS Configuration

Enable HTTPS by setting these environment variables:

```bash
docker run -d -p 7655:7655 \
  -e HTTPS_ENABLED=true \
  -e TLS_CERT_FILE=/data/cert.pem \
  -e TLS_KEY_FILE=/data/key.pem \
  -v pulse_data:/data \
  -v /path/to/certs:/data/certs:ro \
  rcourtman/pulse:latest
```

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
- Container (LXC) disk stats always work (no guest agent needed)

### Connection Issues
- Check Proxmox API is accessible (port 8006/8007)
- Verify credentials have PVEAuditor role plus VM.GuestAgent.Audit (PVE 9) or VM.Monitor (PVE 8); the setup script applies these via the PulseMonitor role (adds Sys.Audit when available)
- For PBS: ensure API token has Datastore.Audit permission

### Logs
```bash
# View logs
docker logs pulse

# Follow logs
docker logs -f pulse
```

## Documentation

Full documentation available on GitHub:

- [Complete Installation Guide](https://github.com/rcourtman/Pulse/blob/main/docs/INSTALL.md)
- [Configuration Guide](https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md)
- [VM Disk Monitoring](https://github.com/rcourtman/Pulse/blob/main/docs/VM_DISK_MONITORING.md) - Set up QEMU Guest Agent for accurate VM disk usage
- [Troubleshooting](https://github.com/rcourtman/Pulse/blob/main/docs/TROUBLESHOOTING.md)
- [API Reference](https://github.com/rcourtman/Pulse/blob/main/docs/API.md)
- [Webhook Guide](https://github.com/rcourtman/Pulse/blob/main/docs/WEBHOOKS.md)
- [Proxy Authentication](https://github.com/rcourtman/Pulse/blob/main/docs/PROXY_AUTH.md) - SSO integration with Authentik, Authelia, etc.
- [Reverse Proxy Setup](https://github.com/rcourtman/Pulse/blob/main/docs/REVERSE_PROXY.md) - nginx, Caddy, Apache, Traefik configs
- [Security](https://github.com/rcourtman/Pulse/blob/main/docs/SECURITY.md)
- [FAQ](https://github.com/rcourtman/Pulse/blob/main/docs/FAQ.md)

## Links

- [GitHub Repository](https://github.com/rcourtman/Pulse)
- [Releases & Changelog](https://github.com/rcourtman/Pulse/releases)
- [Issues & Feature Requests](https://github.com/rcourtman/Pulse/issues)
- [Live Demo](https://demo.pulserelay.pro)

## License

MIT - See [LICENSE](https://github.com/rcourtman/Pulse/blob/main/LICENSE)
