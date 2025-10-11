# Pulse Configuration Guide

## Key Features

- **🔒 Auto-Hashing Security** (v4.5.0+): Plain text credentials provided via environment variables are hashed before being persisted
- **📁 Separated Configuration**: Authentication (.env), runtime settings (system.json), and node credentials (nodes.enc) stay isolated
- **⚙️ UI-First Provisioning**: Nodes and infrastructure settings are managed through the web UI to prevent accidental wipes
- **🔐 Enterprise Security**: Credentials encrypted at rest, hashed in memory
- **🎯 Hysteresis Thresholds**: `alerts.json` stores trigger/clear pairs, fractional network limits, per-metric delays, and overrides that match the Alert Thresholds UI

## Configuration File Structure

Pulse uses three separate configuration files, each with a specific purpose. This separation ensures security, clarity, and proper access control.

### File Locations
All configuration files are stored in `/etc/pulse/` (or `/data/` in Docker containers).

```
/etc/pulse/
├── .env          # Authentication credentials ONLY
├── system.json   # Application settings (ports, intervals, etc.)
├── nodes.enc     # Encrypted node credentials
├── oidc.enc      # Encrypted OIDC client configuration (issuer, client ID/secret)
├── alerts.json   # Alert thresholds and rules
└── webhooks.enc  # Encrypted webhook configurations (v4.1.9+)
```

---

## 📁 `.env` - Authentication & Security

**Purpose:** Contains authentication credentials and security settings ONLY.

**Format:** Environment variables (KEY=VALUE)

**Contents:**
```bash
# User authentication
PULSE_AUTH_USER='admin'              # Admin username
PULSE_AUTH_PASS='$2a$12$...'        # Bcrypt hashed password (keep quotes!)
API_TOKEN=abc123...                  # API token (Pulse hashes it automatically)

# Security settings
DISABLE_AUTH=true                    # Disable authentication entirely
PULSE_AUDIT_LOG=true                # Enable security audit logging

# Proxy/SSO Authentication (see docs/PROXY_AUTH.md for full details)
PROXY_AUTH_SECRET=secret123          # Shared secret between proxy and Pulse
PROXY_AUTH_USER_HEADER=X-Username    # Header containing authenticated username
PROXY_AUTH_ROLE_HEADER=X-Groups      # Header containing user roles/groups
PROXY_AUTH_ADMIN_ROLE=admin          # Role that grants admin access
PROXY_AUTH_LOGOUT_URL=/logout        # URL for SSO logout
```

**Important Notes:**
- Password hash MUST be in single quotes to prevent shell expansion
- API tokens are stored as SHA3-256 hashes on disk; provide a plain token and Pulse hashes it automatically
- This file should have restricted permissions (600)
- Never commit this file to version control
- ProxmoxVE installations may pre-configure API_TOKEN
- Changes to this file are applied immediately without restart (v4.3.9+)
- **DO NOT** put port configuration here - use system.json or systemd overrides
- Copy `.env.example` from the repository for a ready-to-edit template

---

## 📁 `oidc.enc` - OIDC Single Sign-On

**Purpose:** Stores OpenID Connect (OIDC) client configuration for single sign-on.

**Format:** Encrypted JSON (AES-256-GCM via Pulse crypto manager)

**Contents:**
```json
{
  "enabled": true,
  "issuerUrl": "https://login.example.com/realms/pulse",
  "clientId": "pulse",
  "clientSecret": "s3cr3t",
  "redirectUrl": "https://pulse.example.com/api/oidc/callback",
  "scopes": ["openid", "profile", "email"],
  "usernameClaim": "preferred_username",
  "emailClaim": "email",
  "groupsClaim": "groups",
  "allowedGroups": ["pulse-admins"],
  "allowedDomains": ["example.com"],
  "allowedEmails": []
}
```

**Important Notes:**
- Managed through **Settings → Security → Single sign-on (OIDC)** in the UI.
- Secrets are encrypted at rest; client secrets are never exposed back to the browser.
- Optional environment variables (`OIDC_*`) can override individual fields and lock the UI.
- Redirect URL defaults to `<PUBLIC_URL>/api/oidc/callback` if not specified.

---

## 📁 `system.json` - Application Settings

**Purpose:** Contains all application behavior settings and configuration.

**Format:** JSON

**Contents:**
```json
{
  "pbsPollingInterval": 60,        // Seconds between PBS refreshes (PVE polling fixed at 10s)
  "pmgPollingInterval": 60,        // Seconds between PMG refreshes (mail analytics and health)
  "connectionTimeout": 60,         // Seconds before node connection timeout
  "autoUpdateEnabled": false,      // Systemd timer toggle for automatic updates
  "autoUpdateCheckInterval": 24,   // Hours between auto-update checks
  "autoUpdateTime": "03:00",       // Preferred update window (combined with randomized delay)
  "updateChannel": "stable",       // Update channel: stable or rc
  "allowedOrigins": "",            // CORS allowed origins (empty = same-origin only)
  "allowEmbedding": false,         // Allow iframe embedding
  "allowedEmbedOrigins": "",       // Comma-separated origins allowed to embed Pulse
  "backendPort": 3000,             // Internal API listen port (not normally changed)
  "frontendPort": 7655,            // Public port exposed by the service
  "logLevel": "info",              // Log level: debug, info, warn, error
  "discoveryEnabled": true,        // Enable/disable network discovery for Proxmox/PBS servers
  "discoverySubnet": "auto",       // CIDR to scan ("auto" discovers common ranges)
  "theme": ""                      // UI theme preference: "", "light", or "dark"
}
```

**Important Notes:**
- User-editable via Settings UI
- Environment variable overrides (e.g., `DISCOVERY_ENABLED`, `ALLOWED_ORIGINS`) take precedence and lock the corresponding UI controls
- Can be safely backed up without exposing secrets
- Missing file results in defaults being used
- Changes take effect immediately (no restart required)
- API tokens are no longer managed in system.json (moved to .env in v4.3.9+)

---

## 📁 `nodes.enc` - Encrypted Node Credentials

**Purpose:** Stores encrypted credentials for Proxmox VE and PBS nodes.

**Format:** Encrypted JSON (AES-256-GCM)

**Structure (when decrypted):**
```json
{
  "pveInstances": [
    {
      "name": "pve-node1",
      "url": "https://192.168.1.10:8006",
      "username": "root@pam",
      "password": "encrypted_password_here",
      "token": "optional_api_token"
    }
  ],
  "pbsInstances": [
    {
      "name": "backup-server",
      "url": "https://192.168.1.20:8007",
      "username": "admin@pbs",
      "password": "encrypted_password_here"
    }
  ]
}
```

**Important Notes:**
- Encrypted at rest using system-generated key
- Credentials never exposed in UI (only "•••••" shown)
- Export/import requires authentication
- Automatic re-encryption on each save

---

## 📁 `alerts.json` - Alert Thresholds & Scheduling

**Purpose:** Captures the full alerting policy – default thresholds, per-resource overrides, suppression windows, and delivery preferences – exactly as shown in **Alerts → Thresholds**.

**Format:** JSON with hysteresis-aware thresholds (`trigger` and `clear`) and nested configuration blocks.

**Example (trimmed):**

```json
{
  "enabled": true,
  "guestDefaults": {
    "cpu": { "trigger": 90, "clear": 80 },
    "memory": { "trigger": 85, "clear": 72.5 },
    "networkOut": { "trigger": 120.5, "clear": 95 }
  },
  "nodeDefaults": {
    "cpu": { "trigger": 85, "clear": 70 },
    "temperature": { "trigger": 80, "clear": 70 },
    "disableConnectivity": false
  },
  "storageDefault": { "trigger": 85, "clear": 75 },
  "dockerDefaults": {
    "cpu": { "trigger": 75, "clear": 60 },
    "restartCount": 3,
    "restartWindow": 300
  },
  "pmgThresholds": {
    "queueTotalWarning": 500,
    "oldestMessageWarnMins": 30
  },
  "timeThresholds": { "guest": 90, "node": 60, "storage": 180, "pbs": 120 },
  "metricTimeThresholds": {
    "guest": { "disk": 120, "networkOut": 240 }
  },
  "overrides": {
    "delly.lan/qemu/101": {
      "memory": { "trigger": 92, "clear": 80 },
      "networkOut": -1,
      "poweredOffSeverity": "warning"
    }
  },
  "aggregation": {
    "enabled": true,
    "timeWindow": 120,
    "countThreshold": 3,
    "similarityWindow": 90
  },
  "flapping": {
    "enabled": true,
    "threshold": 5,
    "window": 300,
    "suppressionTime": 600,
    "minStability": 180
  },
  "schedule": {
    "quietHours": {
      "enabled": true,
      "start": "22:00",
      "end": "06:00",
      "timezone": "Europe/London",
      "days": { "monday": true, "tuesday": true, "sunday": true },
      "suppress": { "performance": true, "storage": false, "offline": true }
    },
    "cooldown": 15,
    "grouping": { "enabled": true, "window": 120, "byNode": true }
  }
}
```

**Key behaviours:**

- Thresholds use hysteresis pairs (`trigger` / `clear`) to avoid flapping. Use decimals for fine-grained network and IO limits.
- Set a metric to `-1` to disable it globally or per-resource (the UI shows “Off” and adds a **Custom** badge).
- `timeThresholds` apply a grace period before an alert fires; `metricTimeThresholds` allow per-metric overrides (e.g., delay network alerts longer than CPU).
- `overrides` are indexed by the stable resource ID returned from `/api/state` (VMs: `instance/qemu/vmid`, containers: `instance/lxc/ctid`, nodes: `instance/node`).
- Quiet hours, escalation, deduplication, and restart loop detection are all managed here, and the UI keeps the JSON in sync automatically.

> Tip: Back up `alerts.json` alongside `.env` during exports. Restoring it preserves all overrides, quiet-hour schedules, and webhook routing.

---

## 🔄 Automatic Updates

Pulse can automatically install stable updates to keep your installation secure and current.

### How It Works
- **Systemd Timer**: Runs daily at 2 AM with 4-hour random delay
- **Stable Only**: Never installs release candidates automatically
- **Safe Rollback**: Creates backup before updating, restores on failure
- **Respects Config**: Checks `autoUpdateEnabled` in system.json

### Enable/Disable
```bash
# Enable during installation
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --enable-auto-updates

# Enable after installation
systemctl enable --now pulse-update.timer

# Disable auto-updates
systemctl disable --now pulse-update.timer

# Check status
systemctl status pulse-update.timer
systemctl list-timers pulse-update

# View logs
journalctl -u pulse-update
```

### Configuration
Set `autoUpdateEnabled: true` in system.json or toggle in Settings UI.

**Note**: Docker installations do not support automatic updates (use Docker's update mechanisms instead).

---

## Configuration Priority

Settings are loaded in this order (later overrides earlier):

1. **Built-in defaults** - Hardcoded application defaults
2. **system.json file** - Settings configured via UI
3. **Environment variables** - Override both defaults and system.json

### Environment Variables

#### Configuration Variables (override system.json)
These env vars override system.json values. When set, the UI will show a warning and disable the affected fields:

- `DISCOVERY_ENABLED` - Enable/disable network discovery (default: true)
- `DISCOVERY_SUBNET` - Custom network to scan (default: auto-scans common networks)
- `CONNECTION_TIMEOUT` - API timeout in seconds (default: 10)
- `ALLOWED_ORIGINS` - CORS origins (default: same-origin only)
- `LOG_LEVEL` - Log verbosity: debug/info/warn/error (default: info)
- `PULSE_PUBLIC_URL` - Full URL to access Pulse (e.g., `http://192.168.1.100:7655`)
  - **Auto-detected** if not set (except inside Docker where detection is disabled)
  - Used in webhook notifications for "View in Pulse" links
  - Set explicitly when running in containers or whenever auto-detection picks the wrong address
  - Example: `PULSE_PUBLIC_URL="http://192.168.1.100:7655"`

#### Authentication Variables (from .env file)
These should be set in the .env file for security:

- `PULSE_AUTH_USER`, `PULSE_AUTH_PASS` - Basic authentication
- `API_TOKEN` - API token for authentication
- `DISABLE_AUTH` - Set to `true` to disable authentication entirely

#### OIDC Variables (optional overrides)
Set these environment variables to manage single sign-on without using the UI. When present, the OIDC form is locked read-only.

- `OIDC_ENABLED` - `true` / `false`
- `OIDC_ISSUER_URL` - Provider issuer URL
- `OIDC_CLIENT_ID` - Registered client ID
- `OIDC_CLIENT_SECRET` - Client secret (plain text)
- `OIDC_REDIRECT_URL` - Override default redirect callback (use `https://` when behind TLS proxy)
- `OIDC_LOGOUT_URL` - End-session URL for proper OIDC logout (e.g., `https://auth.example.com/application/o/pulse/end-session/`)
- `OIDC_SCOPES` - Space/comma separated scopes (e.g. `openid profile email`)
- `OIDC_USERNAME_CLAIM` - Claim used for the Pulse username
- `OIDC_EMAIL_CLAIM` - Claim that contains the email address
- `OIDC_GROUPS_CLAIM` - Claim that lists group memberships
- `OIDC_ALLOWED_GROUPS` - Allowed group names (comma/space separated)
- `OIDC_ALLOWED_DOMAINS` - Allowed email domains
- `OIDC_ALLOWED_EMAILS` - Explicit email allowlist
- `PULSE_PUBLIC_URL` **(strongly recommended)** - The externally reachable base URL Pulse should advertise. This is used to generate the default redirect URI. If you expose Pulse on multiple hostnames, list each one in your IdP configuration because OIDC callbacks must match exactly.

> **Authentik note:** Assign an RSA signing key to the application so ID tokens use `RS256`. Without it Authentik falls back to `HS256`, which Pulse rejects. See [Authentik setup details](OIDC.md#authentik) for the exact menu path.

#### Proxy/SSO Authentication Variables
For integration with authentication proxies (Authentik, Authelia, etc):

- `PROXY_AUTH_SECRET` - Shared secret between proxy and Pulse (required for proxy auth)
- `PROXY_AUTH_USER_HEADER` - Header containing authenticated username (default: none)
- `PROXY_AUTH_ROLE_HEADER` - Header containing user roles/groups (default: none)
- `PROXY_AUTH_ROLE_SEPARATOR` - Separator for multiple roles (default: |)
- `PROXY_AUTH_ADMIN_ROLE` - Role name that grants admin access (default: admin)
- `PROXY_AUTH_LOGOUT_URL` - URL to redirect for SSO logout (default: none)

See [Proxy Authentication Guide](PROXY_AUTH.md) for detailed configuration examples.

#### Port Configuration
Port configuration should be done via one of these methods:

1. **systemd override** (Recommended for production):
   ```bash
   sudo systemctl edit pulse
   # Add: Environment="FRONTEND_PORT=8080"
   ```

2. **system.json** (For persistent configuration):
   ```json
   {"frontendPort": 8080}
   ```

3. **Environment variable** (For Docker/testing):
   - `FRONTEND_PORT` - Port to listen on (default: 7655)
   - `PORT` - Legacy port variable (use FRONTEND_PORT instead)

#### TLS/HTTPS Configuration
- `HTTPS_ENABLED` - Enable HTTPS (true/false)
- `TLS_CERT_FILE`, `TLS_KEY_FILE` - Paths to TLS certificate files

> **⚠️ UI Override Warning**: When configuration env vars are set (like `ALLOWED_ORIGINS`), the corresponding UI fields will be disabled with a warning message. Remove the env var and restart to enable UI configuration.

---

## Automated Setup (Skip UI)

For automated deployments (CI/CD, infrastructure as code, ProxmoxVE scripts), you can configure Pulse authentication via environment variables, completely bypassing the UI setup screen.

### Simple Automated Setup

**Option 1: API Token Authentication**
```bash
# Start Pulse with API token - setup screen is skipped
API_TOKEN=your-secure-api-token ./pulse

# The token is hashed and stored securely
# Use this same token for all API calls
curl -H "X-API-Token: your-secure-api-token" http://localhost:7655/api/nodes
```

**Option 2: Basic Authentication**
```bash
# Start Pulse with username/password - setup screen is skipped
PULSE_AUTH_USER=admin \
PULSE_AUTH_PASS=your-secure-password \
./pulse

# Password is bcrypt hashed and stored securely
# Use these credentials for UI login or API calls
```

**Option 3: Both (API + Basic Auth)**
```bash
# Configure both authentication methods
API_TOKEN=your-api-token \
PULSE_AUTH_USER=admin \
PULSE_AUTH_PASS=your-password \
./pulse
```

### Security Notes

- **Automatic hashing**: Plain text credentials are automatically hashed when provided via environment variables
  - API tokens → SHA3-256 hash
  - Passwords → bcrypt hash (cost 12)
- **Pre-hashed credentials supported**: Advanced users can provide pre-hashed values:
  - API tokens: 64-character hex string (SHA3-256 hash)
  - Passwords: bcrypt hash starting with `$2a$`, `$2b$`, or `$2y$` (60 characters)
- **No plain text in memory**: All credentials are hashed before use
- Once configured, the setup screen is automatically skipped
- Credentials work immediately - no additional setup required

### Example: Docker Automated Deployment

```bash
#!/bin/bash
# Generate secure token
API_TOKEN=$(openssl rand -hex 32)

# Deploy with authentication pre-configured
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -e API_TOKEN="$API_TOKEN" \
  -v pulse-data:/data \
  rcourtman/pulse:latest

echo "Pulse deployed! Use API token: $API_TOKEN"

# Immediately use the API - no setup needed
curl -H "X-API-Token: $API_TOKEN" http://localhost:7655/api/nodes
```

---

## Security Best Practices

1. **File Permissions**
   ```bash
   chmod 600 /etc/pulse/.env        # Only readable by owner
   chmod 644 /etc/pulse/system.json # Readable by all, writable by owner
   chmod 600 /etc/pulse/nodes.enc   # Only readable by owner
   ```

2. **Backup Strategy**
   - `.env` - Backup separately and securely (contains auth)
   - `system.json` - Safe to include in regular backups
   - `nodes.enc` - Backup with .env (contains encrypted credentials)

3. **Version Control**
   - **NEVER** commit `.env` or `nodes.enc`
   - `system.json` can be committed if it doesn't contain sensitive data
   - Use `.gitignore` to exclude sensitive files
