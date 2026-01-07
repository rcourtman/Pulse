# Pulse Security

This document is the canonical security policy for Pulse. It combines our
ongoing hardening guidance with the operational checklists that previously lived
in `docs/SECURITY.md`.

For a high-level overview of the system design and data flow, please refer to
[`ARCHITECTURE.md`](ARCHITECTURE.md).

---

## Critical Security Notice for Container Deployments

### Container SSH Key Policy (BREAKING CHANGE)

**Effective immediately, SSH-based temperature monitoring is blocked in
containerized Pulse deployments.**

#### Why This Change?

Storing SSH private keys inside Docker/LXC containers creates an unacceptable
risk in production environments:

- **Container compromise = infrastructure compromise** – if an attacker gains
  shell access to the Pulse container they obtain the SSH private keys used to
  reach your Proxmox hosts.
- **Keys persist in images** – private keys survive in image layers and can leak
  when images are pushed to registries or shared.
- **No key rotation** – long-lived keys inside containers are difficult to
  rotate safely.
- **Violates least-privilege** – monitoring containers should not hold
  credentials that grant host-level access to the infrastructure they observe.

#### Affected Deployments

✅ **Not affected** – Pulse installed directly on a VM or bare-metal host (no
containers), or homelab environments where you explicitly accept the risk.

❌ **Blocked** – Pulse running in Docker containers, LXC containers, or any
environment where `PULSE_DOCKER=true`/`/.dockerenv` is detected.

#### Migration Path (Production)

Preferred option (no SSH keys, no proxy wiring):

1. Install the unified agent (`pulse-agent`) on each Proxmox host with Proxmox integration enabled.
   - Use the UI to generate an install command in **Settings → Agents**, or run:
     ```bash
     curl -fsSL http://pulse.example.com:7655/install.sh | \
       sudo bash -s -- --url http://pulse.example.com:7655 --token <api-token> --enable-proxmox
     ```

Deprecated option (existing installs only):

- `pulse-sensor-proxy` is deprecated in Pulse v5 and is not recommended for new deployments. In v5, legacy sensor-proxy endpoints are disabled by default unless `PULSE_ENABLE_SENSOR_PROXY=true` is set on the Pulse server.
- Existing installs continue to work during the migration window, but plan to move to `pulse-agent --enable-proxmox`.
- Canonical temperature docs: `docs/TEMPERATURE_MONITORING.md`

#### Removing Old SSH Keys

If you previously generated SSH keys inside containers:

```bash
# On each Proxmox host
sed -i '/# pulse-/d' /root/.ssh/authorized_keys

# Inside the Pulse container (or rebuild the container)
docker exec pulse rm -rf /home/pulse/.ssh/id_ed25519*
```

#### Security Boundary

```
┌─────────────────────────────────────┐
│  Proxmox Host                       │
│  ┌───────────────────────────────┐  │
│  │  pulse-agent                  │  │
│  │  · Reads sensors locally      │  │
│  │  · Sends metrics via HTTPS    │  │
│  └───────────────────────────────┘  │
│            │                         │
│            │ HTTPS + API token       │
│            │                         │
│  ┌─────────▼─────────────────────┐  │
│  │  Pulse (Docker/LXC container) │  │
│  │  · No SSH keys                │  │
│  │  · No host root privileges    │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

#### Homelab Exception

If you fully understand the risk and are **not** containerized (VM/bare-metal
install), the legacy SSH flow still works. Use a dedicated monitoring user,
restrict the key with `command="sensors -j"` and `from="<pulse-ip>"`, and
rotate keys regularly.

#### Auditing Your Deployment

```bash
# Detect vulnerable containers
ls /home/pulse/.ssh/id_ed25519* 2>/dev/null && echo "⚠️  SSH keys present"
```

Verify temperature collection is agent-based:

- UI: **Settings → Agents** shows each Proxmox host connected and reporting.
- On each Proxmox host:
  ```bash
  systemctl status pulse-agent
  journalctl -u pulse-agent -n 200 --no-pager
  ```

**Documentation:** https://github.com/rcourtman/Pulse/blob/main/SECURITY.md#critical-security-notice-for-container-deployments
**Issues:** https://github.com/rcourtman/pulse/issues
**Private disclosures:** security@pulseapp.io

---

## Mandatory Authentication

Authentication setup is prompted for all new Pulse installations. This protects your Proxmox API credentials from unauthorized
access.

> **Service name note:** systemd deployments use `pulse.service`. If you're
> upgrading from an older install that still registers `pulse-backend.service`,
> substitute that name in the commands below.

### First-Run Security Setup
When you first access Pulse, you'll be guided through a mandatory security
setup:
- Create your admin username and password
- Automatic API token generation for automation
- Settings are applied immediately without restart
- **Your existing nodes and settings are preserved**

## Smart Security Context

### Public Access Detection
Pulse automatically detects when it's being accessed from public networks:
- **Private networks**: local/RFC1918 addresses (192.168.x.x, 10.x.x.x, etc.)
- **Public networks**: any non-private IP address
- **Stronger warnings**: red alerts when accessed from public IPs without
  authentication

### Trusted Networks Configuration (Deprecated)
**Note:** authentication is now mandatory regardless of network location.

Legacy configuration (no longer applicable):
```bash
# Environment variable (comma-separated CIDR blocks)
PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24

# Or in systemd
sudo systemctl edit pulse
[Service]
Environment="PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24"
```

When configured:
- Access from trusted networks: no auth required
- Access from outside: authentication enforced
- Useful for: mixed home/remote access scenarios

## Security Warning System

Pulse includes a non-intrusive security warning system that helps you
understand your security posture.

### Security Score
Your instance receives a score from 0‑5 based on:
- ✅ Credentials encrypted at rest (always enabled)
- ✅ Export/import protection
- ⚠️ Authentication enabled
- ⚠️ HTTPS connection
- ⚠️ Audit logging

### Dismissing Warnings
If you're comfortable with your security setup, you can dismiss warnings:
- **For 1 day** – reminder tomorrow
- **For 1 week** – reminder next week
- **Forever** – won't show again

## Credential Security

### Encrypted at Rest (AES-256-GCM)
- **Node credentials**: passwords and API tokens (`/etc/pulse/nodes.enc`)
- **Email settings**: SMTP passwords (`/etc/pulse/email.enc`)
- **Webhook data**: URLs and auth headers (`/etc/pulse/webhooks.enc`)
- **Encryption key**: auto-generated (`/etc/pulse/.encryption.key`)

### Security Features
- **Logs**: token values masked with `***` in all outputs
- **API**: frontend receives only `hasToken: true`, never actual values
- **Export**: requires a valid API token (`X-API-Token` header or `token`
  parameter) to extract credentials
- **Migration**: use passphrase-protected export/import (see
  [Migration Guide](docs/MIGRATION.md))
- **Auto-migration**: unencrypted configs automatically migrate to encrypted
  format

## Export/Import Protection

By default, configuration export/import is blocked. You have two options:

### Option 1: Create an API Token (Recommended)
Create a token in **Settings → Security → API Tokens**, then use it for exports.
For automation-only environments, you can seed tokens via environment variables (legacy) and
they will be persisted to `api_tokens.json` on startup.

Legacy environment seeding:
```bash
# Using systemd (secure)
sudo systemctl edit pulse
# Add:
[Service]
Environment="API_TOKENS=ansible-token,docker-agent-token"
Environment="API_TOKEN=legacy-token"

# Then restart:
sudo systemctl restart pulse

# Docker
docker run -e API_TOKENS=ansible-token,docker-agent-token rcourtman/pulse:latest
```

### Option 2: Allow Unprotected Export (Homelab)
```bash
# Using systemd
sudo systemctl edit pulse
# Add:
[Service]
Environment="ALLOW_UNPROTECTED_EXPORT=true"

# Docker
docker run -e ALLOW_UNPROTECTED_EXPORT=true rcourtman/pulse:latest
```

**Note:** for production, prefer Docker secrets or systemd environment files
for sensitive data.

## Security Features

### Core Protection
- **Encryption**: credentials encrypted at rest (AES-256-GCM)
- **Export protection**: exports always encrypted with a passphrase
- **Minimum passphrase**: 12 characters required for exports
- **Security tab**: check status in *Settings → Security*

### Enterprise Security (When Authentication Enabled)
- **Password security**
  - Bcrypt hashing with cost factor 12 (60‑character hash)
  - Passwords never stored in plain text
  - Automatic hashing during security setup
  - **Critical**: bcrypt hashes must be exactly 60 characters
- **API token security**
  - 64‑character hex tokens (32 bytes entropy)
  - SHA3-256 hashed before storage (64‑character hash)
  - Raw token shown only once
  - Tokens never stored in plain text
  - Stored in `api_tokens.json` and managed via the UI
  - API-only mode supported (no password auth required)
- **CSRF protection**: all state-changing operations require CSRF tokens
- **Rate limiting**
  - Auth endpoints: 10 attempts/minute per IP (returns `Retry-After` header)
  - General API: 500 requests/minute per IP
  - Real-time endpoints exempt for functionality
  - All responses include rate limit headers:
    - `X-RateLimit-Limit`: Maximum requests per window
    - `X-RateLimit-Remaining`: Requests remaining in current window
    - `Retry-After`: Seconds to wait before retrying (on 429 responses)
- **Account lockout**
  - Locks after 5 failed login attempts
  - 15-minute automatic lockout duration
  - Clear feedback showing remaining attempts
  - Time remaining displayed when locked
  - Manual reset available via API for admins
- **Session management**
  - Secure HttpOnly cookies
  - 24-hour session expiry
  - Session invalidation on password change
- **Security headers**
  - Content-Security-Policy
  - X-Frame-Options: DENY
  - X-Content-Type-Options: nosniff
  - X-XSS-Protection: 1; mode=block
  - Referrer-Policy: strict-origin-when-cross-origin
  - Permissions-Policy restricting sensitive APIs
- **Audit logging**
  - Authentication events include IP addresses
  - Rollback actions are logged with timestamps and metadata
  - Scheduler health escalations recorded in audit trail
  - Runtime logging configuration changes tracked

### What's Encrypted in Exports
- Node credentials (passwords, API tokens)
- PBS credentials
- Email settings passwords
- Webhook URLs and authentication headers

### What's **Not** Encrypted
- Node hostnames and IPs
- Threshold settings
- General configuration
- Alert rules and schedules

## Authentication Workflows

Pulse supports multiple authentication methods that can be used independently or
together.

> **Note**: `DISABLE_AUTH` is deprecated and no longer disables authentication. Remove it from your environment and restart if it's still present.

### Password Authentication

#### Quick Security Setup (Recommended)
1. Navigate to *Settings → Security*.
2. Click **Enable Security Now**.
3. Enter username and password.
4. Save the generated API token (shown only once!).
5. Security is enabled immediately (no restart needed).

This automatically:
- Generates a secure random password
- Hashes it with bcrypt (cost factor 12)
- Creates secure API token (SHA3-256 hashed, raw token shown once)
- For systemd: Configures systemd with hashed credentials
- For Docker: Saves to `/data/.env` with hashed credentials (properly quoted to prevent shell expansion)
- Applies credentials immediately and persists them for future restarts

#### Manual Setup (Advanced)
```bash
# Using systemd (password will be hashed automatically)
sudo systemctl edit pulse
# Add:
[Service]
Environment="PULSE_AUTH_USER=admin"
Environment="PULSE_AUTH_PASS=$2a$12$..."  # Use bcrypt hash, not plain text!

# Docker (credentials persist in volume via .env file)
# IMPORTANT: Always quote bcrypt hashes to prevent shell expansion!
docker run -e PULSE_AUTH_USER=admin -e PULSE_AUTH_PASS='$2a$12$...' rcourtman/pulse:latest
# Or use Quick Security Setup and restart container
```

**Important**: Always use hashed passwords in configuration. Use the Quick Security Setup or generate bcrypt hashes manually.

#### Features
- Web UI login required when authentication enabled
- Change/remove password from Settings → Security  
- Passwords ALWAYS hashed with bcrypt (cost 12)
- Session-based authentication with secure HttpOnly cookies
- 24-hour session expiry
- CSRF protection for all state-changing operations
- Session invalidation on password change

### API Token Authentication  
For programmatic access and automation. API tokens are SHA3-256 hashed for security.

#### Token Setup via Quick Security
The Quick Security Setup automatically:
- Generates a cryptographically secure token
- Hashes it with SHA3-256
- Stores only the 64-character hash
- Adds the token to the managed token list

#### Manual Token Setup (Legacy Seeding)
```bash
# Using systemd (plain text values are auto-hashed on startup)
sudo systemctl edit pulse
# Add:
[Service]
Environment="API_TOKENS=ansible-token,docker-agent-token"

# Docker
docker run -e API_TOKENS=ansible-token,docker-agent-token rcourtman/pulse:latest

# To provide pre-hashed tokens instead, list the SHA3-256 hashes
# Environment="API_TOKENS=83c8...,b1de..."
```

**Security Note**: Tokens defined via environment variables are hashed with SHA3-256 before being stored in `api_tokens.json`. Plain values never persist beyond startup.

#### Token Management (Settings → API Tokens)
- Issue dedicated tokens for automation/agents without sharing a global credential
- View prefixes/suffixes and last-used timestamps for auditing
- Revoke tokens individually without downtime
- Regenerate tokens when rotating credentials (new value displayed once)
- All tokens stored as SHA3-256 hashes

#### Usage
```bash
# Include the ORIGINAL token (not hash) in X-API-Token header
curl -H "X-API-Token: your-original-token" http://localhost:7655/api/health

# or in Authorization header (preferred for shared tooling)
curl -H "Authorization: Bearer your-original-token" http://localhost:7655/api/export
```

### Auto-Registration Security

#### Default Mode
- All access requires authentication
- Nodes can auto-register with the API token
- Setup scripts work without additional configuration

#### Secure Mode
- Require API token for all operations
- Protects auto-registration endpoint
- Enable by creating at least one API token (UI or legacy env seeding)

### Runtime Logging Configuration

Pulse supports configurable logging (level, format, optional file output, rotation) via environment variables.

#### Security Benefits
- Enable debug logging temporarily for incident investigation
- Switch to JSON format for SIEM integration
- Adjust verbosity based on security posture
- Control file rotation to manage audit log retention

#### Configuration Options

**Via environment variables:**
```bash
# Systemd
sudo systemctl edit pulse
[Service]
Environment="LOG_LEVEL=info"
Environment="LOG_FORMAT=json"
Environment="LOG_MAX_SIZE=100"        # MB per log file
Environment="LOG_MAX_AGE=30"          # Days to retain logs
Environment="LOG_COMPRESS=true"       # Compress rotated logs

# Docker
docker run \
  -e LOG_LEVEL=info \
  -e LOG_FORMAT=json \
  -e LOG_MAX_SIZE=100 \
  -e LOG_MAX_AGE=30 \
  -e LOG_COMPRESS=true \
  rcourtman/pulse:latest
```

**Security Considerations:**
- Debug logs may contain sensitive data—enable only when needed
- JSON format recommended for security monitoring and SIEM
- Adjust retention based on compliance requirements
- Changes take effect on restart

## CORS (Cross-Origin Resource Sharing)

By default, Pulse allows all origins (`ALLOWED_ORIGINS=*`). This is convenient for local setups,
but should be restricted in production.

### Configuring CORS for External Access

If you need to access Pulse API from a different domain:

```bash
# Docker
docker run -e ALLOWED_ORIGINS="https://app.example.com" rcourtman/pulse:latest

# systemd
sudo systemctl edit pulse
[Service]
Environment="ALLOWED_ORIGINS=https://app.example.com"

# Development (allow localhost)
ALLOWED_ORIGINS="http://localhost:5173"
```

Notes:

- `ALLOWED_ORIGINS` supports a single origin or `*` (it is written directly to `Access-Control-Allow-Origin`).
- In production, set a specific origin to avoid exposing the API to arbitrary sites.

## Monitoring and Observability

### Scheduler Health API

#### Endpoint
```bash
curl -s http://localhost:7655/api/monitoring/scheduler/health | jq
```

#### Security Use Cases
1. **Anomaly Detection**
   - Watch for unusual queue depths (possible DoS)
   - Monitor circuit breaker trips (connectivity issues or attacks)
   - Track backoff patterns (rate limiting, potential probes)

2. **Performance Monitoring**
   - Identify performance degradation
   - Detect resource exhaustion
   - Track API response times

3. **Incident Response**
   - Real-time visibility into system health
   - Historical metrics for post-incident analysis
   - Circuit breaker status for failover decisions

#### Key Security Metrics
- **Queue Depth**: High values may indicate attack or overload
- **Circuit Breaker Status**: Half-open/open states suggest connectivity issues
- **Backoff Delays**: Increased backoff may indicate rate limiting or errors
- **Error Rates**: Track failed API calls and authentication attempts

There is currently no dedicated scheduler-health UI in v5. Use the API endpoint above (or export diagnostics from **Settings → Diagnostics**) when troubleshooting.

## Security Best Practices

### Credential Storage
- ✅ **DO**: Use Quick Security Setup for automatic hashing
- ✅ **DO**: Store only bcrypt hashes for passwords
- ✅ **DO**: Store only SHA3-256 hashes for API tokens
- ❌ **DON'T**: Store plain text passwords in config files
- ❌ **DON'T**: Store plain text API tokens in config files
- ❌ **DON'T**: Log credentials or include them in backups

### Authentication Setup
- ✅ **DO**: Use strong, unique passwords (16+ characters)
- ✅ **DO**: Rotate API tokens periodically
- ✅ **DO**: Use HTTPS in production environments
- ❌ **DON'T**: Share API tokens between users/services
- ❌ **DON'T**: Embed credentials in client-side code

### Verification Checklist
Manually verify your deployment follows security best practices:
- No hardcoded credentials in environment files
- No credentials exposed in logs (check `docker logs pulse`)
- All passwords stored as bcrypt hashes (60 characters, starting with `$2a$` or `$2b$`)
- All API tokens stored as SHA3-256 hashes (64 characters)
- Secure file permissions on `/etc/pulse/.env` (600)
- No credential leaks in API responses (test with `curl`)

## Account Lockout and Recovery

### Lockout Behavior
- After **5 failed login attempts**, the account is locked for **15 minutes**
- Lockout applies to both username and IP address
- Login form shows remaining attempts after each failure
- Clear message when locked with time remaining

### Automatic Recovery
- Lockouts automatically expire after 15 minutes
- No action needed - just wait for the timer to expire
- Successful login clears all failed attempt counters

### Manual Recovery (Admin)
Administrators with API access can manually reset lockouts:

```bash
# Reset lockout for a specific username
curl -X POST http://localhost:7655/api/security/reset-lockout \
  -H "X-API-Token: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"identifier":"username"}'

# Reset lockout for an IP address
curl -X POST http://localhost:7655/api/security/reset-lockout \
  -H "X-API-Token: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"identifier":"192.168.1.100"}'
```

## Troubleshooting

**Account locked?** Wait 15 minutes or contact admin for manual reset  
**Export blocked?** You're on a public network – login with password, create an API token, or set `ALLOW_UNPROTECTED_EXPORT=true`  
**Rate limited?** Wait 1 minute and try again  
**Can't login?** Check `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` environment variables  
**API access denied?** Verify the token you supplied matches one of the values created in *Settings → API Tokens* (use the original token, not the hash)  
**CORS errors?** Configure `ALLOWED_ORIGINS` for your domain  
**Forgot password?** Start fresh – delete your Pulse data and restart

---
