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
API_TOKEN=abc123...                  # Optional: seed a primary API token (auto-hashed)
API_TOKENS=token-one,token-two       # Optional: comma-separated list of API tokens

# Security settings
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
- API tokens are stored as SHA3-256 hashes on disk; plain tokens listed in `API_TOKEN` or `API_TOKENS` are auto-hashed at startup
- Multiple tokens can be pre-seeded via `API_TOKENS` (comma separated). Every token—plain text or pre-hashed—becomes a distinct credential.
- This file should have restricted permissions (600)
- Never commit this file to version control
- ProxmoxVE installations may pre-configure `API_TOKEN`; you can now add additional tokens without touching the original value
- Changes to this file are applied immediately without restart (v4.3.9+)
- **DO NOT** put port configuration here - use system.json or systemd overrides
- Copy `.env.example` from the repository for a ready-to-edit template
- Locked out? Create `<data-path>/.auth_recovery`, restart Pulse, and sign in from localhost to reset credentials. Remove the file afterwards.

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
  "pbsPollingInterval": 60,            // Seconds between PBS refreshes (PVE polling fixed at 10s)
  "pmgPollingInterval": 60,            // Seconds between PMG refreshes (mail analytics and health)
  "connectionTimeout": 60,             // Seconds before node connection timeout
  "autoUpdateEnabled": false,          // Systemd timer toggle for automatic updates
  "autoUpdateCheckInterval": 24,       // Hours between auto-update checks
  "autoUpdateTime": "03:00",           // Preferred update window (combined with randomized delay)
  "updateChannel": "stable",           // Update channel: stable or rc
  "allowedOrigins": "",                // CORS allowed origins (empty = same-origin only)
  "allowEmbedding": false,             // Allow iframe embedding
  "allowedEmbedOrigins": "",           // Comma-separated origins allowed to embed Pulse
  "temperatureMonitoringEnabled": true,// Global temperature polling toggle (Settings → Proxmox → Edit node → Advanced monitoring)
  "backendPort": 3000,                 // Internal API listen port (not normally changed)
  "frontendPort": 7655,                // Public port exposed by the service
  "logLevel": "info",                  // Log level: debug, info, warn, error
  "logFormat": "auto",                 // auto, json, or console output
  "logFile": "",                       // Optional file path for mirrored logs
  "logMaxSize": 100,                   // Log rotation threshold (MB) when logFile is set
  "logMaxAge": 30,                     // Days to retain rotated files
  "logCompress": true,                 // Compress rotated log files
  "adaptivePollingEnabled": false,     // Toggle adaptive scheduler (v4.24.0+)
  "adaptivePollingBaseInterval": 10,   // Target cadence (seconds)
  "adaptivePollingMinInterval": 5,     // Fastest cadence (seconds)
  "adaptivePollingMaxInterval": 300,   // Slowest cadence (seconds)
  "discoveryEnabled": false,           // Enable/disable network discovery for Proxmox/PBS servers
  "discoverySubnet": "auto",           // CIDR to scan ("auto" discovers common ranges)
  "theme": ""                          // UI theme preference: "", "light", or "dark"
}
```

**Important Notes:**
- User-editable via Settings UI
- Environment variable overrides (e.g., `DISCOVERY_ENABLED`, `ALLOWED_ORIGINS`) take precedence and lock the corresponding UI controls
- Can be safely backed up without exposing secrets
- Missing file results in defaults being used
- Changes take effect immediately (no restart required)
- API tokens are no longer managed in system.json (moved to .env in v4.3.9+)
- **Adaptive polling controls** (`adaptivePollingEnabled`, `adaptivePolling*Interval`) map directly to the Scheduler Health API and adjust queue/backoff behaviour in real time.
- **Runtime logging controls** (`logLevel`, `logFormat`, `logFile`, `logMaxSize`, `logMaxAge`, `logCompress`) can be tuned from the UI or system.json; updates are applied immediately so you can raise verbosity, switch to structured JSON, or stream logs to disk without restarting Pulse.

### Adaptive Polling Settings (v4.24.0+)

- `adaptivePollingEnabled`: Enables the adaptive scheduler that prioritises stale or failing instances. Toggle it in **Settings → System → Adaptive polling** or set the flag in system.json.
- `adaptivePollingBaseInterval`: Target cadence (seconds) when an instance is healthy. Defaults to 10 seconds.
- `adaptivePollingMinInterval`: Lower bound when Pulse needs to poll aggressively (for example, 5 seconds for busy clusters).
- `adaptivePollingMaxInterval`: Upper bound for idle instances. Setting this to a small value (≤15s) automatically engages the low-latency backoff profile (750 ms initial delay, 20 % jitter, 10 s breaker windows).
- The adaptive scheduler feeds the `/api/monitoring/scheduler/health` endpoint and priority queue. Shorter intervals reduce queue depth; longer intervals trade freshness for fewer calls. All three intervals are stored in seconds in system.json; environment overrides accept Go duration strings such as `15s` or `5m`.

### Logging Configuration (v4.24.0+)

- `logLevel`: Runtime log verbosity (`debug`, `info`, `warn`, `error`). Raise it to `debug` temporarily when troubleshooting, then drop back to `info`.
- `logFormat`: `auto` switches between human-friendly console output (interactive TTY) and JSON when Pulse runs under a service. Override with `json` to stream machine-readable logs everywhere, or `console` to force colourised output.
- `logFile`: Optional absolute path. When populated, Pulse mirrors logs to this file as well as stdout. Rotation honours `logMaxSize` (MB), `logMaxAge` (days), and `logCompress` (gzip rotated files).
- Logging changes made via the UI or system.json take effect immediately, so you can capture verbose traces or structured logs without scheduling downtime.

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
    "disk": { "trigger": 85, "clear": 75 },
    "restartCount": 3,
    "restartWindow": 300,
    "memoryWarnPct": 90,
    "memoryCriticalPct": 95,
    "serviceWarnGapPercent": 10,
    "serviceCriticalGapPercent": 50,
    "stateDisableConnectivity": false,
    "statePoweredOffSeverity": "warning"
  },
  "dockerIgnoredContainerPrefixes": [
    "runner-",
    "ci-temp-"
  ],
  "pmgThresholds": {
    "queueTotalWarning": 500,
    "oldestMessageWarnMins": 30
  },
  "timeThresholds": { "guest": 90, "node": 60, "storage": 180, "pbs": 120 },
  "metricTimeThresholds": {
    "guest": { "disk": 120, "networkOut": 240 }
  },
  "overrides": {
    "example.lan/qemu/101": {
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
- Set a metric to `-1` to disable it globally or per-resource (the UI shows "Off" and adds a **Custom** badge).
- `timeThresholds` apply a grace period (in seconds) before an alert fires, with separate defaults per resource type (guest, node, storage, pbs).
- `metricTimeThresholds` provide **per-metric alert delays**, allowing you to configure different wait times for different metrics. See "Alert Delay Configuration" below for details.
- `overrides` are indexed by the stable resource ID returned from `/api/state` (VMs: `instance/qemu/vmid`, containers: `instance/lxc/ctid`, nodes: `instance/node`).
- `dockerIgnoredContainerPrefixes` lets you silence state/metric/restart alerts for ephemeral containers whose names or IDs share a common, case-insensitive prefix. The Containers tab in the UI keeps this list in sync.
- Swarm service alerts track missing replicas: `serviceWarnGapPercent` defines when a warning fires, and `serviceCriticalGapPercent` must be greater than or equal to the warning gap (Pulse automatically clamps the critical value upward if an older client submits something smaller).
- Docker container state controls live in `dockerDefaults`: flip `stateDisableConnectivity` to silence exit/offline alerts globally, or change `statePoweredOffSeverity` to `critical` when you want exiting containers to page immediately. Per-container overrides still take precedence.
- `dockerDefaults.disk` defines the writable-layer usage threshold (% of the container's upper filesystem compared to its base image). Defaults trigger at 85% and clear at 80%, and can be overridden per container or host when noisy workloads need a different window.
- Quiet hours, escalation, deduplication, and restart loop detection are all managed here, and the UI keeps the JSON in sync automatically.

#### Alert Delay Configuration

Alert delays prevent spurious notifications by requiring a threshold to remain exceeded for a specified duration before triggering an alert. Pulse supports **per-metric delay configuration**, allowing you to fine-tune delays for different types of alerts.

**Configuration Levels** (in order of precedence):

1. **Metric-specific delay for resource type**: `metricTimeThresholds[resourceType][metricName]`
2. **Resource-type default**: `timeThresholds[resourceType]` (e.g., `guest`, `node`, `storage`, `pbs`)
3. **Global metric delay**: `metricTimeThresholds["all"][metricName]` (only applies when no resource-type default exists)
4. **Legacy global delay**: `timeThreshold` (deprecated, use `timeThresholds` instead)

**Examples:**

```json
{
  "timeThresholds": {
    "guest": 60,
    "node": 30,
    "storage": 180
  },
  "metricTimeThresholds": {
    "guest": {
      "cpu": 300,
      "memory": 60,
      "disk": 120,
      "networkout": 240
    },
    "node": {
      "cpu": 120,
      "temperature": 300
    },
    "docker": {
      "restartcount": 10,
      "cpu": 120
    },
    "all": {
      "networkout": 300
    }
  }
}
```

**How it works:**

- **CPU alerts for VMs** wait 300 seconds (5 minutes) because CPU spikes are often transient
- **Memory alerts for VMs** wait 60 seconds (1 minute) because memory pressure is typically persistent
- **Disk alerts for VMs** wait 120 seconds (2 minutes), balancing urgency with stability
- **Network Out for VMs** waits 240 seconds (4 minutes) because backups and migrations create temporary spikes
- **Temperature alerts for nodes** wait 300 seconds (5 minutes) to allow fans time to respond
- **Docker restart count** alerts trigger after only 10 seconds for immediate attention
- **Storage usage** with no specific override uses the `storage` default (180 seconds)

**UI Access:**

In the Alerts page, the "Global Defaults" row for each resource table shows an expandable "Alert Delay (s)" sub-row. Each metric column has an input where you can configure per-metric delays. Empty fields inherit from the resource-type default shown in the placeholder.

**Common Use Cases:**

| Metric | Recommended Delay | Reasoning |
|--------|-------------------|-----------|
| CPU | 2-5 minutes | Transient spikes during load balancing, backups, or startup |
| Memory | 30-60 seconds | Persistent issue that needs attention quickly |
| Disk | 1-3 minutes | Gradual fill-up, not usually urgent |
| Network | 3-5 minutes | Backups, migrations, and replication cause temporary spikes |
| Temperature | 5+ minutes | Fans need time to ramp up; short spikes are normal |
| Restart Count | 10-30 seconds | Container crashes need immediate attention |

> Tip: Back up `alerts.json` alongside `.env` during exports. Restoring it preserves all overrides, quiet-hour schedules, and webhook routing.

### `pulse-sensor-proxy/config.yaml`

The sensor proxy reads `/etc/pulse-sensor-proxy/config.yaml` (or the path supplied via `PULSE_SENSOR_PROXY_CONFIG`). Key fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `allowed_source_subnets` | list(string) | auto-detected host CIDRs | Restrict which networks can reach the UNIX socket listener. |
| `allowed_peer_uids` / `allowed_peer_gids` | list(uint32) | empty | Required when Pulse runs in a container; use mapped UID/GID. |
| `allow_idmapped_root` | bool | `true` | Governs acceptance of ID-mapped root callers. |
| `allowed_idmap_users` | list(string) | `["root"]` | Restricts which ID-mapped usernames are accepted. |
| `metrics_address` | string | `default` (maps to `127.0.0.1:9127`) | Set to `"disabled"` to turn metrics off. |
| `rate_limit.per_peer_interval_ms` | int | `1000` | Milliseconds between allowed RPCs per UID. Set `>=100` in production. |
| `rate_limit.per_peer_burst` | int | `5` | Number of requests allowed in a burst; should meet or exceed node count. |

Example (also shipped as `cmd/pulse-sensor-proxy/config.example.yaml`):
```yaml
rate_limit:
  per_peer_interval_ms: 500   # 2 rps
  per_peer_burst: 10          # allow 10-node sweep
```

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

### Update Backups & History (v4.24.0+)

- Every self-update or rollback writes an entry to `<DATA_PATH>/update-history.jsonl` (defaults to `/var/lib/pulse` for systemd installs and `/data` in Docker). Review the log via **Settings → System → Updates**, or query `/api/updates/history` for automation.
- The install script prints the configuration backup it creates (for example `/etc/pulse.backup.20251020-130500`). That path is captured in the history entry as `backup_path` so rollbacks know which snapshot to restore.
- Update logs live under `/var/log/pulse/update-*.log`; grab the most recent file when filing support tickets or analysing failures.

---

## Configuration Priority

Settings are loaded in this order (later overrides earlier):

1. **Built-in defaults** - Hardcoded application defaults
2. **system.json file** - Settings configured via UI
3. **Environment variables** - Override both defaults and system.json

### Environment Variables

#### Configuration Variables (override system.json)
These env vars override system.json values. When set, the UI will show a warning and disable the affected fields:

- `DISCOVERY_ENABLED` - Enable/disable network discovery (default: false)
- `DISCOVERY_SUBNET` - Custom network to scan (default: auto-scans common networks)
- `CONNECTION_TIMEOUT` - API timeout in seconds (default: 10)
- `ALLOWED_ORIGINS` - CORS origins (default: same-origin only)
- `LOG_LEVEL` - Log verbosity: debug/info/warn/error (default: info). Switching levels takes effect immediately.
- `LOG_FORMAT` - Override output format (`auto`, `json`, or `console`).
- `LOG_FILE` - Mirror logs to this absolute path in addition to stdout (empty = stdout only).
- `LOG_MAX_SIZE` - Rotate the log file after it grows beyond this many megabytes (default: 100).
- `LOG_MAX_AGE` - Delete rotated log files older than this many days (default: 30).
- `LOG_COMPRESS` - When `true` (default) gzip-compresses rotated log files.
- `ADAPTIVE_POLLING_ENABLED` - Enable/disable the adaptive scheduler without touching system.json (`true`/`false`).
- `ADAPTIVE_POLLING_BASE_INTERVAL` - Override the target polling cadence (accepts Go durations, e.g. `15s`).
- `ADAPTIVE_POLLING_MIN_INTERVAL` - Override the minimum cadence (Go duration or seconds).
- `ADAPTIVE_POLLING_MAX_INTERVAL` - Override the maximum cadence (Go duration or seconds). Values ≤`15s` engage the low-latency backoff profile.
- `ENABLE_BACKUP_POLLING` - Set to `false` to disable polling of Proxmox backup/snapshot APIs (default: true)
- `BACKUP_POLLING_INTERVAL` - Override the backup polling cadence. Accepts Go duration syntax (e.g. `30m`, `6h`) or seconds. Use `0` for Pulse's default (~90s) cadence.
- `ENABLE_TEMPERATURE_MONITORING` - Force-enable or disable SSH temperature polling for all nodes (`true`/`false`)
- `PULSE_PUBLIC_URL` - Full URL to access Pulse (e.g., `http://192.168.1.100:7655`)
  - **Auto-detected** if not set (except inside Docker where detection is disabled)
  - Used in webhook notifications for "View in Pulse" links
  - Set explicitly when running in containers or whenever auto-detection picks the wrong address
  - Example: `PULSE_PUBLIC_URL="http://192.168.1.100:7655"`

> **Log file behaviour:** When `LOG_FILE` is set, Pulse continues to write logs to stderr while also appending to the specified file. Rotation occurs when the file exceeds `LOG_MAX_SIZE` megabytes (default 100 MB). Rotated files older than `LOG_MAX_AGE` days (default 30) are deleted, and compressing is enabled by default (`LOG_COMPRESS=true`), producing `.gz` archives for rotated files.

#### Authentication Variables (from .env file)
These should be set in the .env file for security:

- `PULSE_AUTH_USER`, `PULSE_AUTH_PASS` - Basic authentication
- `API_TOKEN` - Primary API token (auto-hashed if you supply the raw value)
- `API_TOKENS` - Comma-separated list of additional API tokens (plain or SHA3-256 hashed)

> Locked out? Create `<data-path>/.auth_recovery`, restart Pulse, and sign in from localhost. Delete the flag file and restart again to restore normal authentication.

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

**Option 1: API Tokens (single or multiple)**
```bash
# Start Pulse with API tokens - setup screen is skipped
API_TOKENS="$ANSIBLE_TOKEN,$DOCKER_AGENT_TOKEN" ./pulse

# Each token is hashed and stored securely on startup
curl -H "X-API-Token: $ANSIBLE_TOKEN" http://localhost:7655/api/nodes

# Legacy fallback (not recommended for new installs)
# API_TOKEN=your-secure-api-token ./pulse
```

> **Tip:** Generate a distinct token for each automation workflow (Ansible, Docker agents, host agents, CI runners, etc.) so you can revoke one credential without affecting the others.

### Token Scopes

API tokens created in the UI can be restricted to the smallest set of permissions required by an integration:

| Scope | Typical use |
|-------|-------------|
| `docker:report` | Docker agent submitting host/container telemetry |
| `docker:manage` | Docker agent lifecycle commands (restart, stop, etc.) |
| `host-agent:report` | Pulse host agent reporting OS metrics |
| `monitoring:read` | Read-only access to dashboards, state API, and alert history |
| `monitoring:write` | Acknowledge, silence, or clear alerts |
| `settings:read` | Fetch configuration snapshots and diagnostics |
| `settings:write` | Modify configuration, manage tokens, trigger updates |

Leaving the scope list empty (or legacy tokens without scopes) grants full access. Tokens generated from specific panels (e.g. **Settings → Agents → Host agents**) automatically apply the relevant scope presets.

> **Upgrade note:** After upgrading, existing tokens are treated as full-access (`*`). Visit **Settings → Security** to edit each legacy token and assign narrower scopes.

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
Set `PRIMARY_TOKEN` to the token value you want to reuse (plain text or SHA3-256 hash) before starting Pulse:
```bash
# Configure both authentication methods
API_TOKENS="$PRIMARY_TOKEN" \
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
# Generate dedicated tokens for each integration
ANSIBLE_TOKEN=$(openssl rand -hex 32)
DOCKER_AGENT_TOKEN=$(openssl rand -hex 32)

# Deploy with authentication pre-configured
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -e API_TOKENS="$ANSIBLE_TOKEN,$DOCKER_AGENT_TOKEN" \
  -v pulse-data:/data \
  rcourtman/pulse:latest

echo "Pulse deployed!"
echo "  Ansible token: $ANSIBLE_TOKEN"
echo "  Docker agent token: $DOCKER_AGENT_TOKEN"

# Immediately use the API - no setup needed
curl -H "X-API-Token: $ANSIBLE_TOKEN" http://localhost:7655/api/nodes
```

Remember to store each token securely; the plain values above are displayed only once.

### Managing tokens via the REST API

Infrastructure-as-code workflows (Ansible, Terraform, etc.) can drive token lifecycle directly through the new `/api/security/tokens` endpoints:

- `GET /api/security/tokens` – list existing tokens (metadata only)
- `POST /api/security/tokens` – create a new token; the raw value is returned once in the response
- `DELETE /api/security/tokens/{id}` – revoke a token by its identifier

Example: create a token named `ansible` and capture the secret for later use.

```bash
NEW_TOKEN_JSON=$(curl -sS -X POST http://localhost:7655/api/security/tokens \
  -H "Content-Type: application/json" \
  -H "X-API-Token: $ADMIN_TOKEN" \
  -d '{"name":"ansible"}')

NEW_TOKEN=$(echo "$NEW_TOKEN_JSON" | jq -r '.token')
TOKEN_ID=$(echo "$NEW_TOKEN_JSON" | jq -r '.record.id')
echo "New token value: $NEW_TOKEN"
echo "Token id: $TOKEN_ID"
```

Store `NEW_TOKEN` securely; future GET requests only expose token hints (`prefix`/`suffix`). To revoke the credential later, call `DELETE /api/security/tokens/$TOKEN_ID`.

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
