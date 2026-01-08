# ⚙️ Configuration Guide

Pulse uses a split-configuration model to ensure security and flexibility.

| File | Purpose | Security Level |
|------|---------|----------------|
| `.env` | Authentication & Secrets | 🔒 **Critical** (Read-only by owner) |
| `.encryption.key` | Encryption key for `.enc` files | 🔒 **Critical** |
| `system.json` | General Settings | 📝 Standard |
| `nodes.enc` | Node Credentials | 🔒 **Encrypted** (AES-256-GCM) |
| `alerts.json` | Alert Rules | 📝 Standard |
| `email.enc` | SMTP settings | 🔒 **Encrypted** |
| `webhooks.enc` | Webhook URLs + headers | 🔒 **Encrypted** |
| `apprise.enc` | Apprise notification config | 🔒 **Encrypted** |
| `oidc.enc` | OIDC provider config | 🔒 **Encrypted** |
| `api_tokens.json` | API token records (hashed) | 🔒 **Sensitive** |
| `env_token_suppressions.json` | Suppressed legacy env tokens (migration aid) | 📝 Standard |
| `ai.enc` | AI settings and credentials | 🔒 **Encrypted** |
| `ai_findings.json` | AI Patrol findings | 📝 Standard |
| `ai_patrol_runs.json` | AI Patrol run history | 📝 Standard |
| `ai_usage_history.json` | AI usage history | 📝 Standard |
| `license.enc` | Pulse Pro license key | 🔒 **Encrypted** |
| `host_metadata.json` | Host notes, tags, and AI command overrides | 📝 Standard |
| `docker_metadata.json` | Docker metadata cache | 📝 Standard |
| `guest_metadata.json` | Guest notes and metadata | 📝 Standard |
| `agent_profiles.json` | Agent configuration profiles (Pulse Pro) | 📝 Standard |
| `agent_profile_assignments.json` | Agent profile assignments (Pulse Pro) | 📝 Standard |
| `recovery_tokens.json` | Recovery tokens (short-lived) | 🔒 **Sensitive** |
| `sessions.json` | Persistent sessions (includes OIDC refresh tokens) | 🔒 **Sensitive** |
| `update-history.jsonl` | Update history log (in-app updates) | 📝 Standard |
| `metrics.db` | Persistent metrics history (SQLite) | 📝 Standard |

All files are located in `/etc/pulse/` (Systemd) or `/data/` (Docker/Kubernetes) by default.

Path overrides:
- `PULSE_DATA_DIR` sets the base directory for `system.json`, encrypted files, and the bootstrap token.
- `PULSE_AUTH_CONFIG_DIR` sets the directory for `.env` (auth-only) if you need auth on a separate volume.

---

## 🔐 Authentication (`.env`)

This file controls access to Pulse. It is **never** exposed to the UI.

```bash
# /etc/pulse/.env

# Admin Credentials (bcrypt hashed; plain text auto-hashes on startup)
PULSE_AUTH_USER='admin'
PULSE_AUTH_PASS='$2a$12$...' 

# Legacy API tokens (deprecated, auto-migrated to api_tokens.json)
API_TOKEN='token1'
API_TOKENS='token2,token3'
```

<details>
<summary><strong>Advanced: Automated Setup (Skip UI)</strong></summary>

You can pre-configure Pulse by setting environment variables. Plain text credentials are automatically hashed on startup.

```bash
# Docker Example
docker run -d \
  -e PULSE_AUTH_USER=admin \
  -e PULSE_AUTH_PASS=secret123 \
  -e API_TOKENS=ci-token,agent-token \
  rcourtman/pulse:latest
```
</details>

<details>
<summary><strong>Advanced: OIDC / SSO</strong></summary>

Configure Single Sign-On in **Settings → Security → Single Sign-On**, or use environment variables to lock the configuration.

See [OIDC Documentation](OIDC.md) and [Proxy Auth](PROXY_AUTH.md) for details.

Environment overrides (lock the corresponding UI fields):

| Variable | Description |
|----------|-------------|
| `OIDC_ENABLED` | Enable OIDC (`true`/`false`) |
| `OIDC_ISSUER_URL` | Issuer URL from your IdP |
| `OIDC_CLIENT_ID` | Client ID |
| `OIDC_CLIENT_SECRET` | Client secret |
| `OIDC_REDIRECT_URL` | Override redirect URL (defaults to `<public-url>/api/oidc/callback`) |
| `OIDC_LOGOUT_URL` | Optional logout URL |
| `OIDC_SCOPES` | Space or comma-separated scopes |
| `OIDC_USERNAME_CLAIM` | Claim for username (default: `preferred_username`) |
| `OIDC_EMAIL_CLAIM` | Claim for email (default: `email`) |
| `OIDC_GROUPS_CLAIM` | Claim for groups |
| `OIDC_ALLOWED_GROUPS` | Allowed groups (space or comma-separated) |
| `OIDC_ALLOWED_DOMAINS` | Allowed email domains (space or comma-separated) |
| `OIDC_ALLOWED_EMAILS` | Allowed emails (space or comma-separated) |
| `OIDC_CA_BUNDLE` | Custom CA bundle path |
</details>

> **Note**: `API_TOKEN` / `API_TOKENS` are legacy and will be migrated into `api_tokens.json` on startup.
> Manage API tokens in the UI for long-term support.

---

## 🖥️ System Settings (`system.json`)

Controls runtime behavior like ports, logging, and polling intervals. Most of these can be changed in **Settings → System**.

<details>
<summary><strong>Example system.json</strong></summary>

```json
{
  "pvePollingInterval": 10,       // Seconds
  "backendPort": 3000,            // Legacy (unused)
  "frontendPort": 7655,           // Public port
  "logLevel": "info",             // debug, info, warn, error
  "autoUpdateEnabled": false,     // Enable auto-update checks
  "adaptivePollingEnabled": false, // Smart polling for large clusters
  "allowedOrigins": "",           // CORS allowlist (single origin or "*")
  "allowEmbedding": false,        // Allow iframe embedding
  "allowedEmbedOrigins": "",      // Comma-separated origins for iframe embedding
  "webhookAllowedPrivateCIDRs": "" // Allowlist for private webhook targets
}
```

> **Note**: `logFormat` is only configurable via the `LOG_FORMAT` environment variable, not in `system.json`.
> **Note**: `autoUpdateTime` is stored by the UI, but the systemd timer uses its own schedule.
</details>

### Common Overrides (Environment Variables)
Environment variables take precedence over `system.json`.

| Variable | Description | Default |
|----------|-------------|---------|
| `FRONTEND_PORT` | Public listening port | `7655` |
| `PORT` | Legacy alias for `FRONTEND_PORT` | *(unset)* |
| `BACKEND_HOST` | Bind host for the HTTP server and metrics listener (advanced) | *(unset)* |
| `BACKEND_PORT` | Legacy internal API port (unused) | `3000` |
| `LOG_LEVEL` | Log verbosity (see below) | `info` |
| `LOG_FORMAT` | Log output format (`auto`, `json`, `console`) | `auto` |

#### Log Levels

| Level | Description |
|-------|-------------|
| `error` | Only errors and critical issues |
| `warn` | Errors + warnings (recommended for minimal logging) |
| `info` | Standard operational messages (startup, connections, alerts) |
| `debug` | Verbose output including per-guest/storage polling details |

> **Tip**: If your syslog is being flooded with Pulse messages, set `LOG_LEVEL=warn` to significantly reduce log volume while still capturing important events.

| Variable | Description | Default |
|----------|-------------|---------|
| `PULSE_PUBLIC_URL` | URL for UI links, notifications, and OIDC. For reverse proxies, keep this as the public URL and use `PULSE_AGENT_CONNECT_URL` for agent installs if you need a direct/internal address. | Auto-detected |
| `PULSE_AGENT_CONNECT_URL` | Dedicated direct URL for agents (overrides `PULSE_PUBLIC_URL` for agent install commands). Alias: `PULSE_AGENT_URL`. | *(unset)* |
| `ALLOWED_ORIGINS` | CORS allowed origin (`*` or a single origin). Empty = same-origin only. | *(unset)* |
| `DISCOVERY_ENABLED` | Auto-discover nodes | `false` |
| `DISCOVERY_SUBNET` | CIDR or `auto` | `auto` |
| `DISCOVERY_ENVIRONMENT_OVERRIDE` | Force discovery environment (`auto`, `native`, `docker_host`, `docker_bridge`, `lxc_privileged`, `lxc_unprivileged`) | `auto` |
| `DISCOVERY_SUBNET_ALLOWLIST` | Comma-separated CIDRs allowed for discovery | *(empty)* |
| `DISCOVERY_SUBNET_BLOCKLIST` | Comma-separated CIDRs excluded from discovery | `169.254.0.0/16` |
| `DISCOVERY_MAX_HOSTS_PER_SCAN` | Max hosts to scan per run | `1024` |
| `DISCOVERY_MAX_CONCURRENT` | Max concurrent discovery probes | `50` |
| `DISCOVERY_ENABLE_REVERSE_DNS` | Enable reverse DNS lookup (`true`/`false`) | `true` |
| `DISCOVERY_SCAN_GATEWAYS` | Include gateway IPs in discovery (`true`/`false`) | `true` |
| `DISCOVERY_DIAL_TIMEOUT_MS` | TCP dial timeout (ms) | `1000` |
| `DISCOVERY_HTTP_TIMEOUT_MS` | HTTP probe timeout (ms) | `2000` |
| `PULSE_ENABLE_SENSOR_PROXY` | Enable legacy `pulse-sensor-proxy` endpoints (deprecated, unsupported) | `false` |
| `PULSE_AUTH_HIDE_LOCAL_LOGIN` | Hide username/password form | `false` |
| `DEMO_MODE` | Enable read-only demo mode | `false` |
| `PULSE_TRUSTED_PROXY_CIDRS` | Comma-separated IPs/CIDRs trusted to supply `X-Forwarded-For`/`X-Real-IP` | *(unset)* |
| `PULSE_TRUSTED_NETWORKS` | Comma-separated CIDRs treated as trusted local networks (does not bypass auth) | *(unset)* |
| `PULSE_SENSOR_PROXY_SOCKET` | Legacy sensor-proxy socket override (deprecated) | *(unset)* |

### Iframe Embedding (system.json)

Embedding is controlled by `system.json` and the UI (**Settings → System → Network**):

- `allowEmbedding` (boolean): enables iframe embedding
- `allowedEmbedOrigins` (comma-separated): restricts `frame-ancestors` when embedding is enabled

When `allowEmbedding` is `false`, Pulse sends `X-Frame-Options: DENY` and `frame-ancestors 'none'`.

### Monitoring Overrides

| Variable | Description | Default |
|----------|-------------|---------|
| `PVE_POLLING_INTERVAL` | PVE metrics polling frequency | `10s` |
| `PBS_POLLING_INTERVAL` | PBS metrics polling frequency | `60s` |
| `PMG_POLLING_INTERVAL` | PMG metrics polling frequency | `60s` |
| `CONCURRENT_POLLING` | Enable concurrent polling for multi-node clusters | `true` |
| `CONNECTION_TIMEOUT` | API connection timeout | `60s` |
| `BACKUP_POLLING_CYCLES` | Poll cycles between backup checks | `10` |
| `ENABLE_BACKUP_POLLING` | Enable backup job monitoring | `true` |
| `BACKUP_POLLING_INTERVAL` | Backup polling frequency | `0` (Auto) |
| `ENABLE_TEMPERATURE_MONITORING` | Enable temperature monitoring (where supported) | `true` |
| `SSH_PORT` | SSH port for legacy SSH-based temperature collection | `22` |
| `ADAPTIVE_POLLING_ENABLED` | Enable smart polling for large clusters | `false` |
| `ADAPTIVE_POLLING_BASE_INTERVAL` | Base interval for adaptive polling | `10s` |
| `ADAPTIVE_POLLING_MIN_INTERVAL` | Minimum adaptive polling interval | `5s` |
| `ADAPTIVE_POLLING_MAX_INTERVAL` | Maximum adaptive polling interval | `5m` |
| `GUEST_METADATA_MIN_REFRESH_INTERVAL` | Minimum refresh for guest metadata | `2m` |
| `GUEST_METADATA_REFRESH_JITTER` | Jitter for guest metadata refresh | `45s` |
| `GUEST_METADATA_RETRY_BACKOFF` | Retry backoff for guest metadata | `30s` |
| `GUEST_METADATA_MAX_CONCURRENT` | Max concurrent guest metadata fetches | `4` |
| `DNS_CACHE_TIMEOUT` | Cache TTL for DNS lookups | `5m` |
| `MAX_POLL_TIMEOUT` | Maximum time per polling cycle | `3m` |
| `WEBHOOK_BATCH_DELAY` | Delay before sending batched webhooks | `10s` |
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Hide Docker update buttons (read-only mode) | `false` |

### Logging Overrides

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_FILE` | Log file path (empty = stdout) | *(unset)* |
| `LOG_MAX_SIZE` | Log file max size (MB) | `100` |
| `LOG_MAX_AGE` | Log file retention (days) | `30` |
| `LOG_COMPRESS` | Compress rotated logs | `true` |

### Update Settings (system.json)

These are stored in `system.json` and managed via the UI.

| Key | Description | Default |
|-----|-------------|---------|
| `updateChannel` | Update channel (`stable` or `rc`) | `stable` |
| `autoUpdateEnabled` | Allow one-click updates | `false` |
| `autoUpdateCheckInterval` | Stored UI preference (server currently checks hourly) | `24` |
| `autoUpdateTime` | Stored UI preference (systemd timer has its own schedule) | `03:00` |

### Auto-Import (Bootstrap)

You can auto-import an encrypted backup on first startup. This is useful for automated provisioning and test environments.

| Variable | Description |
|----------|-------------|
| `PULSE_INIT_CONFIG_DATA` | Base64 or raw contents of an export bundle (auto-imports on first start) |
| `PULSE_INIT_CONFIG_FILE` | Path to an export bundle on disk (auto-imports on first start) |
| `PULSE_INIT_CONFIG_PASSPHRASE` | Passphrase for the export bundle (required) |

> **Note**: `PULSE_INIT_CONFIG_URL` is only supported by the hidden `pulse config auto-import` command, not by the server startup auto-import.

### Developer/Test Overrides (Environment Variables)

These are primarily for development or test harnesses and should not be used in production.

| Variable | Description | Default |
|----------|-------------|---------|
| `PULSE_UPDATE_SERVER` | Override update server base URL (testing only) | *(unset)* |
| `PULSE_UPDATE_STAGE_DELAY_MS` | Adds artificial delays between update stages (testing only) | *(unset)* |
| `PULSE_ALLOW_DOCKER_UPDATES` | Expose update UI/actions in Docker (debug only) | `false` |
| `PULSE_AI_ALLOW_LOOPBACK` | Allow AI tool HTTP fetches to loopback addresses | `false` |
| `PULSE_LICENSE_PUBLIC_KEY` | Override embedded license public key (base64, dev only) | *(unset)* |
| `PULSE_LICENSE_DEV_MODE` | Skip license verification (development only) | `false` |

### Metrics Retention (Tiered)

Persistent metrics history uses tiered retention windows. These values are stored in `system.json` and can be adjusted for storage vs history depth:

- `metricsRetentionRawHours`
- `metricsRetentionMinuteHours`
- `metricsRetentionHourlyDays`
- `metricsRetentionDailyDays`

See [METRICS_HISTORY.md](METRICS_HISTORY.md) for details.

---

## 🔔 Alerts (`alerts.json`)

Pulse uses a powerful alerting engine with hysteresis (separate trigger/clear thresholds) to prevent flapping.

**Managed via UI**: Alerts → Thresholds

<details>
<summary><strong>Manual Configuration (JSON)</strong></summary>

```json
{
  "guestDefaults": {
    "cpu": { "trigger": 90, "clear": 80 },
    "memory": { "trigger": 85, "clear": 72.5 }
  },
  "schedule": {
    "quietHours": {
      "enabled": true,
      "start": "22:00",
      "end": "06:00"
    }
  }
}
```
</details>

---

## 🔒 HTTPS / TLS

Enable HTTPS by providing certificate files via environment variables.

```bash
# Systemd
HTTPS_ENABLED=true
TLS_CERT_FILE=/etc/pulse/cert.pem
TLS_KEY_FILE=/etc/pulse/key.pem

# Docker
docker run -e HTTPS_ENABLED=true \
  -v /path/to/certs:/certs \
  -e TLS_CERT_FILE=/certs/cert.pem \
  -e TLS_KEY_FILE=/certs/key.pem ...
```

---

## 🛡️ Security Best Practices

1. **Permissions**: Ensure `.env` and `nodes.enc` are `600` (read/write by owner only).
2. **Backups**: Back up `.env` separately from `system.json`.
3. **Tokens**: Use scoped API tokens for agents instead of the admin password.

---

## 🔑 API Tokens

API tokens provide scoped, revocable access to Pulse. Manage tokens in **Settings → Security → API Tokens**.

### Token Scopes

| Scope | Description |
|-------|-------------|
| `*` (Full access) | All permissions (legacy, not recommended) |
| `monitoring:read` | View dashboards, metrics, alerts |
| `monitoring:write` | Acknowledge/silence alerts |
| `docker:report` | Container agent telemetry submission |
| `docker:manage` | Container lifecycle actions (restart, stop) |
| `kubernetes:report` | Kubernetes agent telemetry submission |
| `kubernetes:manage` | Kubernetes cluster management |
| `host-agent:report` | Host agent metrics submission |
| `settings:read` | Read configuration |
| `settings:write` | Modify configuration |

### Presets

The UI offers quick presets for common use cases:

| Preset | Scopes | Use Case |
|--------|--------|----------|
| **Kiosk / Dashboard** | `monitoring:read` | Read-only dashboard displays |
| **Host agent** | `host-agent:report` | Host agent authentication |
| **Container report** | `docker:report` | Container agent (read-only) |
| **Container manage** | `docker:report`, `docker:manage` | Container agent with actions |
| **Settings read** | `settings:read` | Read-only config access |
| **Settings admin** | `settings:read`, `settings:write` | Full config access |

### Kiosk Mode

For unattended displays (wall monitors, dashboards), use a kiosk token to avoid cookie persistence issues:

1. Go to **Settings → Security → API Tokens**
2. Click **New token** and select the **Kiosk / Dashboard** preset
3. Copy the generated token
4. Access Pulse via URL with token:
   ```
   https://your-pulse-url/?token=YOUR_TOKEN_HERE
   ```

**Kiosk tokens:**
- Grant read-only dashboard access (`monitoring:read` scope)
- Hide the Settings tab automatically
- Work without cookies (token in URL)
- Can be revoked anytime from the UI

> **Security note**: URL tokens appear in browser history and server logs. Use only for read-only dashboard access on trusted networks.
