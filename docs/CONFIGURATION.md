# ⚙️ Configuration Guide

Pulse uses a split-configuration model to ensure security and flexibility.

| File | Purpose | Security Level |
|------|---------|----------------|
| `.env` | Authentication & Secrets | 🔒 **Critical** (Read-only by owner) |
| `system.json` | General Settings | 📝 Standard |
| `nodes.enc` | Node Credentials | 🔒 **Encrypted** (AES-256-GCM) |
| `alerts.json` | Alert Rules | 📝 Standard |
| `email.enc` | SMTP settings | 🔒 **Encrypted** |
| `webhooks.enc` | Webhook URLs + headers | 🔒 **Encrypted** |
| `apprise.enc` | Apprise notification config | 🔒 **Encrypted** |
| `oidc.enc` | OIDC provider config | 🔒 **Encrypted** |
| `api_tokens.json` | API token records (hashed) | 🔒 **Sensitive** |
| `ai.enc` | AI settings and credentials | 🔒 **Encrypted** |
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

# Admin Credentials (bcrypt hashed)
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
</details>

> **Note**: `API_TOKEN` / `API_TOKENS` are legacy and will be migrated into `api_tokens.json` on startup.
> Manage API tokens in the UI for long-term support.

---

## 🖥️ System Settings (`system.json`)

Controls runtime behavior like ports, logging, and polling intervals. Most of these can be changed in **Settings → System**.

<details>
<summary><strong>Full Configuration Reference</strong></summary>

```json
{
  "pvePollingInterval": 10,       // Seconds
  "backendPort": 3000,            // Internal port (default: 3000)
  "frontendPort": 7655,           // Public port
  "logLevel": "info",             // debug, info, warn, error
  "autoUpdateEnabled": false,     // Enable auto-update checks
  "adaptivePollingEnabled": false // Smart polling for large clusters
}
```

> **Note**: `logFormat` is only configurable via the `LOG_FORMAT` environment variable, not in `system.json`.
</details>

### Common Overrides (Environment Variables)
Environment variables take precedence over `system.json`.

| Variable | Description | Default |
|----------|-------------|---------|
| `FRONTEND_PORT` | Public listening port | `7655` |
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
| `PULSE_PUBLIC_URL` | URL for UI links, notifications, and OIDC. **Reverse proxy setups**: set this to the direct/internal Pulse URL (e.g., `http://192.168.1.10:7655`) so agents connect directly instead of via the proxy. | Auto-detected |
| `PULSE_AGENT_CONNECT_URL` | Dedicated direct URL for agents (overrides `PULSE_PUBLIC_URL` for agent install commands). Alias: `PULSE_AGENT_URL`. | *(unset)* |
| `ALLOWED_ORIGINS` | CORS allowed domains | `*` |
| `IFRAME_EMBEDDING_ALLOW` | Iframe embedding policy (`SAMEORIGIN`, `ALLOWALL`, etc.) | `SAMEORIGIN` |
| `DISCOVERY_ENABLED` | Auto-discover nodes | `false` |
| `DISCOVERY_SUBNET` | CIDR or `auto` | `auto` |
| `PULSE_ENABLE_SENSOR_PROXY` | Enable legacy `pulse-sensor-proxy` endpoints (deprecated, unsupported) | `false` |
| `PULSE_AUTH_HIDE_LOCAL_LOGIN` | Hide username/password form | `false` |
| `DEMO_MODE` | Enable read-only demo mode | `false` |
| `PULSE_TRUSTED_PROXY_CIDRS` | Comma-separated IPs/CIDRs trusted to supply `X-Forwarded-For`/`X-Real-IP` | *(unset)* |
| `PULSE_TRUSTED_NETWORKS` | Comma-separated CIDRs treated as trusted local networks | *(unset)* |

### Monitoring Overrides

| Variable | Description | Default |
|----------|-------------|---------|
| `PVE_POLLING_INTERVAL` | PVE metrics polling frequency | `10s` |
| `PBS_POLLING_INTERVAL` | PBS metrics polling frequency | `60s` |
| `PMG_POLLING_INTERVAL` | PMG metrics polling frequency | `60s` |
| `CONCURRENT_POLLING` | Enable concurrent polling for multi-node clusters | `true` |
| `CONNECTION_TIMEOUT` | API connection timeout | `45s` |
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

### Update Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `UPDATE_CHANNEL` | Update channel (`stable` or `rc`) | `stable` |
| `AUTO_UPDATE_ENABLED` | Allow one-click updates | `false` |
| `AUTO_UPDATE_CHECK_INTERVAL` | Auto-check interval | `24h` |
| `AUTO_UPDATE_TIME` | Scheduled check time (HH:MM) | `03:00` |

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

**Managed via UI**: Settings → Alerts → Thresholds

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
