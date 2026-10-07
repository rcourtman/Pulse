# ⚙️ Configuration Guide

Pulse uses a split-configuration model to ensure security and flexibility.

| File | Purpose | Security Level |
| ------ | --------- | ---------------- |
| `.env` | Authentication & Secrets | 🔒 **Critical** (Owner access only) |
| `.encryption.key` | Encryption key for `.enc` files | 🔒 **Critical** |
| `audit/.audit-signing.key` | Encrypted signing key for the default audit store; preserve with its history and matching encryption key | 🔒 **Sensitive** |
| `system.json` | General Settings | 📝 Standard |
| `nodes.enc` | Node Credentials | 🔒 **Encrypted** (AES-256-GCM) |
| `alerts.json` | Alert Rules | 📝 Standard |
| `email.enc` | SMTP settings | 🔒 **Encrypted** |
| `webhooks.enc` | Webhook URLs + headers | 🔒 **Encrypted** |
| `apprise.enc` | Apprise notification config | 🔒 **Encrypted** |
| `oidc.enc` | OIDC provider config | 🔒 **Encrypted** |
| `sso.enc` | SAML/SSO provider config | 🔒 **Encrypted** |
| `api_tokens.json` | API token records (hashed) | 🔒 **Sensitive** |
| `ai.enc` | AI settings and credentials | 🔒 **Encrypted** |
| `ai_findings.json` | AI Patrol findings | 📝 Standard |
| `ai_patrol_runs.json` | AI Patrol run history | 📝 Standard |
| `ai_usage_history.json` | AI usage history | 📝 Standard |
| `ai_chat_sessions.json` | Legacy AI chat sessions (UI sync) | 📝 Standard |
| `license.enc` | Relay/Pro/legacy Pro+/Cloud license key | 🔒 **Encrypted** |
| `report_schedules.json` | Scheduled report definitions, recipients, and last-run metadata | 🔒 **Sensitive** (encrypted when data-dir encryption is enabled) |
| `host_metadata.json` | Host notes, tags, and AI command overrides | 📝 Standard |
| `docker_metadata.json` | Docker metadata cache | 📝 Standard |
| `guest_metadata.json` | Guest notes and metadata | 📝 Standard |
| `agent_profiles.json` | Agent configuration profiles (Pro/legacy Pro+/Cloud) | 📝 Standard |
| `agent_profile_assignments.json` | Agent profile assignments (Pro/legacy Pro+/Cloud) | 📝 Standard |
| `profile-versions.json` | Agent profile version history (Pro/legacy Pro+/Cloud) | 📝 Standard |
| `profile-deployments.json` | Agent profile deployment status (Pro/legacy Pro+/Cloud) | 📝 Standard |
| `profile-changelog.json` | Agent profile change log (Pro/legacy Pro+/Cloud) | 📝 Standard |
| `recovery_tokens.json` | Recovery tokens (short-lived) | 🔒 **Sensitive** |
| `sessions.json` | Persistent sessions (includes OIDC refresh tokens) | 🔒 **Sensitive** |
| `update-history.jsonl` | Update history log (in-app updates) | 📝 Standard |
| `metrics.db` | Persistent metrics history (SQLite) | 📝 Standard |
| `audit/audit.db` | Default audit database; capture on all plans, licensed query/export | 🔒 **Sensitive** |
| `baselines.json` | AI baseline data for anomaly detection | 📝 Standard |
| `ai_correlations.json` | AI correlation analysis cache | 📝 Standard |
| `ai_patterns.json` | AI pattern detection data | 📝 Standard |
| `ai_remediations.json` | AI remediation suggestions | 📝 Standard |
| `ai_incidents.json` | AI incident tracking | 📝 Standard |
| `org.json` | Organization metadata (multi-tenant) | 📝 Standard |

Guest metadata entries are keyed by the canonical guest ID format `instance:node:vmid` (for example, `pve1:node1:100`). Legacy dash-separated keys are migrated automatically.

Paths are relative to `/etc/pulse/` (Systemd) or `/data/` (Docker/Kubernetes) by
default. The audit database and default signing key are in the `audit/`
subdirectory. Runtime-specific audit storage and non-default organisation
paths can differ; see [Audit storage and safe recovery](AUDIT_LOGGING.md#storage).
Do not reset signing or encryption keys as a troubleshooting step.

Path overrides:
- `PULSE_DATA_DIR` sets the base directory for `system.json`, encrypted files, and the bootstrap token.
- `PULSE_METRICS_DB_PATH` sets only the metrics SQLite database path. Use this
  for tmpfs-backed metrics history without moving secrets or config off the
  persistent data directory.

Multi-tenant layout:
- Default org uses the root data directory for backward compatibility.
- Non-default orgs store data under `/orgs/<org-id>/`.
- Migration may create `/orgs/default/` and symlinks in the root data directory.

---

## 🔐 Authentication (`.env`)

For a new install, leave `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` unset and
complete the bootstrap-token setup in your browser. See
[First login](INSTALL.md#step-1-get-the-token). Do not deploy a shared example password.

The Pulse-generated `.env` controls local authentication and is not exposed
to the UI. Preserve its owner-only permissions. A bcrypt hash is sensitive too:
someone who obtains it can attempt offline password cracking.

```dotenv
# Pulse-generated file, not a command to paste into a shell
PULSE_AUTH_USER='admin'
PULSE_AUTH_PASS='<complete-bcrypt-hash>'
```

Deployment-supplied environment values take precedence over this file. Changing
the generated file or using Pulse's password-change UI does not replace a
password supplied by your deployment manager; update that managed source too.

### Private Docker authentication file

If automation must skip browser setup, keep the credentials in a private file,
not a `docker run -e PULSE_AUTH_PASS=...` command, shell history or a shared
Compose file. On the Docker host, prepare the file below. These commands do not
erase an existing file; edit it locally, without printing its contents:

```bash
(
  set -e
  umask 077
  auth_dir="$HOME/.config/pulse"
  auth_file="$auth_dir/docker-auth.env"
  if [ -L "$auth_dir" ] || [ -L "$auth_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  mkdir -p "$auth_dir"
  chmod 700 "$auth_dir"
  touch "$auth_file"
  chmod 600 "$auth_file"
  vi "$auth_file"
)
```

In that editor, enter these two settings and replace the placeholder with a
**complete bcrypt hash** generated by a trusted local tool with an interactive
password prompt. Do not put the password in that tool's command arguments.
Unlike Pulse's own quoted `.env`, Docker's `--env-file` input uses unquoted
values; keep the hash's `$` characters literal, with no `$$` substitution:

```dotenv
PULSE_AUTH_USER=admin
PULSE_AUTH_PASS=<complete-bcrypt-hash>
```

For a **new Community container**, replace `vX.Y.Z` with the exact published
tag you intend to install, then run:

```bash
(
  set -e
  if [ ! -s "$HOME/.config/pulse/docker-auth.env" ]; then
    printf 'Prepare the private authentication file first.\n' >&2
    exit 1
  fi
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --env-file "$HOME/.config/pulse/docker-auth.env" \
  -e PULSE_DEPLOYMENT_METHOD=docker_run \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
)
```

This keeps credential values out of the command arguments, **not out of Docker's
container environment**: administrators with Docker access can still read them.
Pulse hashes a plaintext `PULSE_AUTH_PASS` for authentication at startup, but
does not scrub its original value from Docker or your deployment file. Keep the
file private, outside the repository, and do not share `docker inspect` or
resolved Compose configuration. Do not source the file into your shell.

For an existing or paid Pro deployment, retain its image, mounts and managed
configuration; this new-container example is not an upgrade procedure. Compose
interpolation and Docker `--env-file` are different formats. Prefer bootstrap
setup for Compose unless your deployment manager already has a private
credential mechanism. See the [Docker guide](DOCKER.md) for setup and updates.

<details>
<summary><strong>Advanced: OIDC / SSO</strong></summary>

Configure Single Sign-On in **Settings → Security → Single Sign-On**, or use environment variables to lock the configuration.

See [OIDC Documentation](OIDC.md) and [Proxy Auth](PROXY_AUTH.md) for details.

Environment overrides (lock the corresponding UI fields):

| Variable | Description |
| ---------- | ------------- |
| `OIDC_ENABLED` | Enable OIDC (`true`/`false`) |
| `OIDC_ISSUER_URL` | Issuer URL from your IdP |
| `OIDC_CLIENT_ID` | Client ID |
| `OIDC_CLIENT_SECRET` | Client secret |
| `OIDC_REDIRECT_URL` | Override redirect URL (defaults to `<public-url>/api/oidc/<provider-id>/callback`) |
| `OIDC_LOGOUT_URL` | Optional logout URL |
| `OIDC_SCOPES` | Space or comma-separated scopes |
| `OIDC_USERNAME_CLAIM` | Claim for username (default: `preferred_username`) |
| `OIDC_EMAIL_CLAIM` | Claim for email (default: `email`) |
| `OIDC_GROUPS_CLAIM` | Claim for groups |
| `OIDC_ALLOWED_GROUPS` | Allowed groups (space or comma-separated) |
| `OIDC_ALLOWED_DOMAINS` | Allowed email domains (space or comma-separated) |
| `OIDC_ALLOWED_EMAILS` | Allowed emails (space or comma-separated) |
| `OIDC_GROUP_ROLE_MAPPINGS` | Comma-separated group=role mappings (built-in roles on every plan; custom-role administration requires Pro RBAC) |
| `OIDC_CA_BUNDLE` | Custom CA bundle path |

</details>

> **Note**: `API_TOKEN` / `API_TOKENS` in `.env` are legacy and ignored at runtime in v6.
> Manage API tokens in the UI (`api_tokens.json`) for supported behavior.

---

## 🖥️ System Settings (`system.json`)

Controls runtime behavior like logging, polling intervals, and UI preferences. Legacy port fields in `system.json` are ignored; use `FRONTEND_PORT` instead.

<details>
<summary><strong>Example system.json</strong></summary>

```json
{
  "pvePollingInterval": 10,       // Seconds
  "backendPort": 3000,            // Legacy (unused)
  "frontendPort": 7655,           // Legacy (ignored; use FRONTEND_PORT)
  "logLevel": "info",             // debug, info, warn, error
  "autoUpdateEnabled": false,     // Enable auto-update checks
  "adaptivePollingEnabled": false, // Smart polling for large clusters
  "allowedOrigins": "",           // CORS: empty or comma-separated exact origins
  "allowEmbedding": false,        // Allow iframe embedding
  "allowedEmbedOrigins": "",      // Comma-separated origins for iframe embedding
  "webhookAllowedPrivateCIDRs": "" // Allowlist for private webhook targets
}
```

> **Note**: `logFormat` is only configurable via the `LOG_FORMAT` environment variable, not in `system.json`.
</details>

### Supported system.json Keys

Numeric intervals are **seconds** unless noted otherwise.

| Key | Description |
| ----- | ----------- |
| `pvePollingInterval` | PVE polling interval |
| `pbsPollingInterval` | PBS polling interval |
| `pmgPollingInterval` | PMG polling interval |
| `backupPollingInterval` | Backup polling interval (`0` = auto) |
| `backupPollingEnabled` | Enable backup polling |
| `adaptivePollingEnabled` | Enable adaptive polling |
| `adaptivePollingBaseInterval` | Base interval for adaptive polling |
| `adaptivePollingMinInterval` | Minimum adaptive polling interval |
| `adaptivePollingMaxInterval` | Maximum adaptive polling interval |
| `connectionTimeout` | API connection timeout |
| `logLevel` | Server log level (`debug`, `info`, `warn`, `error`) |
| `allowedOrigins` | CORS: empty grants no cross-origin browser permission; comma-separated exact origins allow credentialed browser requests. `*` allows any origin without credentials. |
| `allowEmbedding` | Allow iframe embedding |
| `allowedEmbedOrigins` | Comma-separated `frame-ancestors` allowlist |
| `webhookAllowedPrivateCIDRs` | Allowlist for private webhook targets |
| `updateChannel` | Update channel (`stable` or `rc`) |
| `autoUpdateEnabled` | Allow one-click updates |
| `publicURL` | Public URL used in links/notifications |
| `hideLocalLogin` | Hide username/password login form |
| `temperatureMonitoringEnabled` | Enable temperature monitoring (where supported) |
| `dnsCacheTimeout` | DNS cache timeout |
| `sshPort` | Default SSH port for temperature collection |
| `discoveryEnabled` | Enable auto-discovery |
| `discoverySubnet` | CIDR or `auto` |
| `discoveryConfig` | Discovery tuning object (see below) |
| `theme` | UI theme (`light`, `dark`, or empty for system) |
| `fullWidthMode` | UI layout preference |
| `metricsRetentionRawHours` | Raw metrics retention (hours) |
| `metricsRetentionMinuteHours` | Minute metrics retention (hours) |
| `metricsRetentionHourlyDays` | Hourly metrics retention (days) |
| `metricsRetentionDailyDays` | Daily metrics retention (days) |
| `disableDockerUpdateActions` | Hide Docker update actions in UI |
| `backendPort` | Legacy (unused) |
| `frontendPort` | Legacy (ignored; use `FRONTEND_PORT`) |

`discoveryConfig` supports:
- `environmentOverride`, `subnetAllowlist`, `subnetBlocklist`
- `maxHostsPerScan`, `maxConcurrent`, `enableReverseDns`, `scanGateways`
- `dialTimeoutMs`, `httpTimeoutMs`

### Common Overrides (Environment Variables)
Environment variables take precedence over `system.json`.

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `FRONTEND_PORT` | Public listening port (web UI, API, and agent ingest) | `7655` |
| `PORT` | **Deprecated** legacy alias for `FRONTEND_PORT`, honored only when `FRONTEND_PORT` is unset. Logs a deprecation warning at startup; switch to `FRONTEND_PORT`. | *(unset)* |
| `PULSE_AGENT_INGEST_PORT` | Optional dedicated port for the complete agent control plane: reports/config (`/api/agents/*`), command admission (`/api/agent/ws`), version checks, and bootstrap downloads. The web UI and management API stay isolated. `0` = disabled (single port). See [Split-Port Agent Ingest](#split-port-agent-ingest-network-isolation). | `0` |
| `LOG_LEVEL` | Log verbosity (see below) | `info` |
| `LOG_FORMAT` | Log output format (`auto`, `json`, `console`) | `auto` |
| `LOG_FILE` | Log file path (enables file logging) | *(unset)* |
| `LOG_MAX_SIZE` | Log rotation size (MB) | `100` |
| `LOG_MAX_AGE` | Keep rotated logs for N days (`0` disables cleanup) | `30` |
| `LOG_COMPRESS` | Gzip rotated logs | `true` |

#### Log Levels

| Level | Description |
| ------- | ------------- |
| `error` | Only errors and critical issues |
| `warn` | Errors + warnings (recommended for minimal logging) |
| `info` | Standard operational messages (startup, connections, alerts) |
| `debug` | Verbose output including per-guest/storage polling details |

> **Tip**: If your syslog is being flooded with Pulse messages, set `LOG_LEVEL=warn` to significantly reduce log volume while still capturing important events.

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `PULSE_PUBLIC_URL` | URL for UI links, notifications, and OIDC. For reverse proxies, keep this as the public URL and use `PULSE_AGENT_CONNECT_URL` for agent installs if you need a direct/internal address. | Auto-detected |
| `PULSE_PRO_TRIAL_SIGNUP_URL` | Legacy hosted commercial base URL retained for hosted entitlement refresh compatibility. The path is ignored for refresh and normal self-hosted v6 UI must not surface trial signup. Must be absolute `http(s)` URL. | `https://cloud.pulserelay.pro` |
| `PULSE_AGENT_CONNECT_URL` | Dedicated direct URL for agents (overrides `PULSE_PUBLIC_URL` for agent install commands). Alias: `PULSE_AGENT_URL`. | *(unset)* |
| `PULSE_AGENT_CONFIG_SIGNING_KEY` | Base64 Ed25519 private key used to sign remote agent config payloads. | *(unset)* |
| `PULSE_AGENT_CONFIG_PUBLIC_KEYS` | Comma-separated base64 Ed25519 public keys (raw 32-byte or PKIX-encoded) trusted by agents. | *(unset)* |
| `PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED` | Require signed remote config payloads (set on Pulse and agents). | `false` |
| `ALLOWED_ORIGINS` | Overrides the saved CORS allowlist with comma-separated exact origins. `*` permits any origin without credentialed browser access; not a login/proxy repair. | *(unset)* |
| `DISCOVERY_ENABLED` | Auto-discover nodes | `false` |
| `DISCOVERY_SUBNET` | CIDR or `auto` | `auto` |
| `DISCOVERY_ENVIRONMENT_OVERRIDE` | Force discovery environment (`auto`, `native`, `docker-host`, `docker-bridge`, `lxc-privileged`, `lxc-unprivileged`) | `auto` |
| `DISCOVERY_SUBNET_ALLOWLIST` | Comma-separated CIDRs allowed for discovery | *(empty)* |
| `DISCOVERY_SUBNET_BLOCKLIST` | Comma-separated CIDRs excluded from discovery | `169.254.0.0/16` |
| `DISCOVERY_MAX_HOSTS_PER_SCAN` | Max hosts to scan per run | `1024` |
| `DISCOVERY_MAX_CONCURRENT` | Max concurrent discovery probes | `50` |
| `DISCOVERY_ENABLE_REVERSE_DNS` | Enable reverse DNS lookup (`true`/`false`) | `true` |
| `DISCOVERY_SCAN_GATEWAYS` | Include gateway IPs in discovery (`true`/`false`) | `true` |
| `DISCOVERY_DIAL_TIMEOUT_MS` | TCP dial timeout (ms) | `1000` |
| `DISCOVERY_HTTP_TIMEOUT_MS` | HTTP probe timeout (ms) | `2000` |
| `PULSE_AUTH_HIDE_LOCAL_LOGIN` | Hide username/password form | `false` |
| `DEMO_MODE` | Enable read-only demo mode | `false` |
| `PULSE_TRUSTED_PROXY_CIDRS` | Comma-separated immediate proxy IPs/CIDRs trusted for forwarded client IP, scheme, host and port. Use the peer seen by Pulse; wildcard ranges are rejected. See [Reverse Proxy](REVERSE_PROXY.md#before-configuring-the-proxy). | *(unset)* |
| `PULSE_TRUSTED_NETWORKS` | Comma-separated CIDRs treated as trusted local networks (does not bypass auth) | *(unset)* |
| `ALLOW_UNPROTECTED_EXPORT` | Allow unauthenticated config export on public networks when no auth is configured (use with caution) | `false` |

### Split-Port Agent Ingest (Network Isolation)

By default Pulse serves the web UI, the REST API, and the agent control plane together on `FRONTEND_PORT`. For deployments that expose Pulse to monitored hosts across an untrusted network (for example, a managed service provider whose clients' Proxmox nodes reach a central Pulse server over the internet), you can expose the agent control plane on its own dedicated port and keep the web UI and management API on a separate, firewalled port.

Set `PULSE_AGENT_INGEST_PORT` to a port other than `FRONTEND_PORT`:

```bash
PULSE_AGENT_INGEST_PORT=7656
```

When enabled:

- The dedicated port serves only the agent-owned routes required for a complete lifecycle: `/api/agents/*`, `/api/agent/ws`, `/api/agent/version`, `/api/server/info`, `/install.sh`, `/install.ps1`, and `/download/pulse-agent`. Every other path, including the web UI, login, and management APIs, returns `404`. A host that can reach the agent port cannot pivot to the management interface.
- The main `FRONTEND_PORT` listener is unchanged and still serves everything (including agent ingest), so existing single-port installs keep working. The dedicated listener is purely additive.
- The value is validated at startup: it must be between 1 and 65535 and must differ from `FRONTEND_PORT` and the HTTP redirect port. An invalid value is rejected.

Expose only `PULSE_AGENT_INGEST_PORT` to your monitored hosts and keep `FRONTEND_PORT` on a private network or behind your firewall/VPN. Point agents at the dedicated port by setting `PULSE_AGENT_CONNECT_URL` to that port's public address, so generated agent install commands send check-ins there:

```bash
PULSE_AGENT_INGEST_PORT=7656
PULSE_AGENT_CONNECT_URL=https://agents.example.com:7656
```

Agents then post telemetry to `https://agents.example.com:7656/api/agents/agent/report` and establish their command channel at `wss://agents.example.com:7656/api/agent/ws`, while the web UI and management API remain reachable only on the private `FRONTEND_PORT` listener. If command execution is enabled, both routes must traverse the same proxy/firewall path; a successful report does not prove that the WebSocket is admitted.

### Proxmox Cluster Node Display Names

Nodes discovered through one Proxmox VE cluster connection can have an
optional Pulse display name. Open **Settings → Infrastructure**, edit the
Proxmox VE connection, and set **Display name** beside a cluster member.

- Display names are presentation only. They do not change the Proxmox node
  name, API address, credentials, TLS fingerprint, routing, or action target.
- An empty display name uses the current native Proxmox node name.
- Names are trimmed, may contain Unicode, are limited to 128 characters, and
  cannot contain control characters. Duplicate display names are allowed
  because they are cosmetic; Pulse still uses a separate immutable identity.
- The override is stored with that Proxmox connection and survives reloads,
  restarts, member address changes, native node renames, and temporary cluster
  membership loss. Confirmed removed members retain their identity metadata so
  a later reappearance can recover the override.
- Pulse keeps the native Proxmox node name, prior native names, numeric node ID
  when available, and immutable Pulse identity for diagnostics and search.
  These fields are also available to API and mobile clients.

For legacy configuration first seen before Pulse records a Proxmox numeric node
ID, changing both the native node name and every known member address at once
is intentionally treated as a new member. Pulse does not guess across
ambiguous evidence because that could transfer an override to the wrong node.

### Iframe Embedding (system.json)

Embedding is controlled by `system.json` and the UI (**Settings → System → Network**):

- `allowEmbedding` (boolean): enables iframe embedding
- `allowedEmbedOrigins` (comma-separated): restricts `frame-ancestors` when embedding is enabled

When `allowEmbedding` is `false`, Pulse sends `X-Frame-Options: DENY` and `frame-ancestors 'none'`.

### Monitoring Overrides

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `PVE_POLLING_INTERVAL` | PVE metrics polling frequency | `10s` |
| `PBS_POLLING_INTERVAL` | PBS metrics polling frequency | `60s` |
| `PMG_POLLING_INTERVAL` | PMG metrics polling frequency | `60s` |
| `CONNECTION_TIMEOUT` | API connection timeout | `60s` |
| `BACKUP_POLLING_CYCLES` | Poll cycles between backup checks | `10` |
| `ENABLE_BACKUP_POLLING` | Enable backup job monitoring | `true` |
| `BACKUP_POLLING_INTERVAL` | Backup polling frequency | `0` (Auto) |
| `ENABLE_TEMPERATURE_MONITORING` | Enable temperature monitoring (where supported) | `true` |
| `SSH_PORT` | SSH port for temperature collection over SSH | `22` |
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
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Hide Docker update buttons (read-only mode) | `false` |
| `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION` | Allow Proxmox-side LXC Docker socket hinting with `pct exec` | `false` |
| `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY` | Allow Proxmox-side minimal LXC Docker inventory collection with `pct exec`; collects Docker host/container summary, not inspect/env/mount/process data. Admins can also toggle this in Settings → System → General; setting the env var locks the toggle | `false` |
| `PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS` | Optional comma-separated VMID allowlist for Proxmox-side LXC Docker discovery; when set, only these guests are socket-probed and inventoried. Empty means all running LXCs are eligible when detection or inventory is enabled | *(unset)* |
| `PULSE_TELEMETRY` | Outbound usage telemetry ([details](PRIVACY.md)); set `false` to disable | `true` |
| `PULSE_DEPLOYMENT_METHOD` | Optional closed telemetry label: `docker_compose`, `docker_run`, `container_other`, `systemd`, `binary_other`, or `other`; invalid values are reported only as the safe runtime fallback | Inferred as `container_other` or `binary_other` |

### Logging Overrides

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `LOG_FILE` | Log file path (empty = stderr only) | *(unset)* |
| `LOG_MAX_SIZE` | Log file max size (MB) | `100` |
| `LOG_MAX_AGE` | Log file retention (days, `0` disables cleanup) | `30` |
| `LOG_COMPRESS` | Compress rotated logs | `true` |


### Update Settings (system.json)

These are stored in `system.json` and managed via the UI.

| Key | Description | Default |
| ----- | ------------- | --------- |
| `updateChannel` | Update channel (`stable` or `rc`) | `stable` |
| `autoUpdateEnabled` | Allow one-click updates | `false` |

> **Note**: Update settings are stored in `system.json`. Legacy `.env` entries (`UPDATE_CHANNEL`, `AUTO_UPDATE_ENABLED`) are kept in sync for backwards compatibility but are not read at runtime. The former `autoUpdateCheckInterval` / `autoUpdateTime` fields were never consumed and are ignored if present; the update schedule lives in the systemd timer.
>
> `stable` is the default and recommended production channel. `rc` is an
> opt-in preview channel. In v6, unattended systemd auto-updates remain
> `stable`-only even when `updateChannel` is set to `rc`.

### Auto-Import (Bootstrap)

You can auto-import an encrypted backup on first startup. This is useful for automated provisioning and test environments.

| Variable | Description |
| ---------- | ------------- |
| `PULSE_INIT_CONFIG_DATA` | Base64 or raw contents of an export bundle (auto-imports on first start) |
| `PULSE_INIT_CONFIG_FILE` | Path to an export bundle on disk (auto-imports on first start) |
| `PULSE_INIT_CONFIG_PASSPHRASE` | Passphrase for the export bundle (required) |

> **Note**: `PULSE_INIT_CONFIG_URL` is only supported by the hidden `pulse config auto-import` command, not by the server startup auto-import.

Configuration import restores server-side agent credentials, but it cannot
change the primary Pulse URL persisted on remote agents. When restoring onto a
server with a different address, retarget those agents after import. See
[Moving Pulse to a new address](UNIFIED_AGENT.md#moving-pulse-to-a-new-address).

### Developer/Test Overrides (Environment Variables)

These are primarily for development or test harnesses and should not be used in production.

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `PULSE_UPDATE_SERVER` | Override update server base URL (testing only) | *(unset)* |
| `PULSE_UPDATE_STAGE_DELAY_MS` | Adds artificial delays between update stages (testing only) | *(unset)* |
| `PULSE_ALLOW_DOCKER_UPDATES` | Expose update UI/actions in Docker (debug only) | `false` |
| `PULSE_DEV_ALLOW_CONTAINER_SSH` | Allow SSH-based temperature collection from containers (dev/test only) | `false` |
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

### Prometheus Metrics Endpoint

The `/metrics` listener is separate from the main UI/API listener and binds to loopback by default.

| Variable | Description | Default |
| ---------- | ------------- | --------- |
| `PULSE_METRICS_PORT` | Metrics listener port | `9091` |
| `PULSE_METRICS_BIND_ADDRESS` | Metrics listener bind address | `127.0.0.1` |
| `PULSE_METRICS_TOKEN` | Optional bearer token for `/metrics` | *(empty)* |
| `PULSE_METRICS_ALLOW_INSECURE_REMOTE` | Explicit opt-in to serve a metrics bearer token over non-loopback plaintext HTTP | `false` |

For remote scraping with `PULSE_METRICS_TOKEN`, prefer a local scraper, tunnel, VPN-private path, or TLS/mTLS reverse proxy. Pulse refuses non-loopback plaintext token scraping unless `PULSE_METRICS_ALLOW_INSECURE_REMOTE=true` is set.

---

## 🔔 Alerts (`alerts.json`)

Manage alert rules in **Alerts → Thresholds**. Choose the platform and resource
before editing; a group default can affect many resources, while a custom
resource override takes precedence. Finish the edit and use **Save Changes**;
an unsaved value is not the running policy. Reload after a successful save to
check that the intended value persisted.

### Metric thresholds, Off and inheritance

For numeric metric rules such as CPU, memory and disk usage:

| Setting | Meaning |
| --- | --- |
| Positive metric threshold | Enables that metric rule at the entered value, subject to other alert policies. Read the column's unit: percentages, temperatures and throughput are not interchangeable. |
| Metric **Off** | Disables that metric rule, not collection of its readings. Use the On/Off control rather than an empty input to disable it. |
| Blank per-resource metric value | Inherits the group default; it does **not** mean Off. An inherited Off default remains Off. |
| Saved numeric metric trigger `0` or a negative value | Disables that metric rule. Zero does **not** mean “alert on any usage”. Older saved rules may use `0`; the current metric Off control writes `-1`. |

**Zero has different meanings in different fields.** A zero *metric trigger*
disables the rule, but zero *powered-off tolerance* below means immediate
eligibility on an authoritative stopped observation. Do not copy one field's
meaning into another. Platform/resource alert switches, offline alerts and
notification delivery have their own controls.

Usage rules have separate trigger and clear values (hysteresis), so a small
drop below the trigger does not repeatedly close and reopen the alert. In the
manual CPU example below, `trigger: 90` and `clear: 80` mean:

- A fresh evaluated value of **90% or higher** is eligible to activate the alert.
- An already active alert stays active **above 80%**, even at 85%.
- A fresh evaluated value of **80% or lower** is eligible to clear it.

Configured evaluation windows and activation/recovery delays still apply;
these are not promises of immediate notifications. The excerpt illustrates a
saved rule, not the clear value produced by every UI edit.

Disabling a metric can close its existing alert even while usage remains high.
That disappearance is a policy change, **not measured recovery**. Missing or
stale readings are not a healthy zero either. Check fresh readings and the
workload before judging recovery; do not lower thresholds, create load or stop
a workload just to test the rule. To quiet notifications without disabling a
metric rule, review [Quiet hours](#quiet-hours-and-notification-holds) and their
critical-alert exceptions. Check [Recent delivery activity](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
separately: a saved threshold or a successful destination Test does not prove
ordinary alert delivery.

### VM and container powered-off tolerance

Open **Alerts → Thresholds → Alert intent & grace** to configure how long a
Proxmox VM or LXC container may remain stopped before Pulse raises its
powered-off alert.

- Leave the VM/container default blank to inherit the existing policy. An
  installation with no applicable policy keeps the legacy two-poll behavior.
- Set it to `0` to alert on the first authoritative stopped observation.
- Set a positive number of seconds (for example, `300`) to tolerate a short
  stop regardless of polling cadence. Per-resource values override the
  VM/container default; a blank per-resource value inherits it.
- **Extend offline grace during Proxmox backups** applies only while Pulse has
  fresh matching evidence of an active backup. The maximum deferral is a hard
  cap, so a stale or stuck backup cannot hide a sustained outage.

This tolerance delays activation; it does not disable powered-off monitoring.
Use the existing guest offline-alert toggle when a guest should never produce
powered-off alerts.

### Acknowledge and snooze existing alerts

In **Alerts → Overview**, these actions apply to an existing incident, not its
threshold rule. Monitoring continues; neither action repairs the workload or
confirms recovery.

| Action | Effect |
| --- | --- |
| **Acknowledge** | Marks the incident as seen. While acknowledged, further firing notifications, escalation and its recovery notification are suppressed. The underlying alert can remain active. |
| **Unacknowledge** | Removes that acknowledgement. Normal notification policies apply again; this is not a guaranteed immediate resend. |
| **Snooze** | Pauses notifications and escalation for this incident until the selected time, including critical notifications. Monitoring can still detect recovery during the snooze. |
| **Resume** | Ends this incident's snooze early. It does not remove an acknowledgement or clear the underlying alert. |

Acknowledged alerts are hidden from the default active list and excluded from
its **Active** count. Use **Show acknowledged** to inspect them and
**Unacknowledge** only when they need attention again. **No unacknowledged
alerts** does not mean every workload recovered. Before **Acknowledge all**, read
the count and scope: the toolbar applies to all unacknowledged active alerts;
a group's button applies to that group, including its collapsed related alerts.
Use the individual action when only one incident has been reviewed.

Choose a **Snooze** duration on the incident's card and check its **Snoozed until**
time. **Until tomorrow at 9:00** uses the browser's local timezone, not the
[quiet-hours timezone](#quiet-hours-and-notification-holds). On expiry or
**Resume**, an acknowledged incident remains acknowledged; remove that hold
separately if intended. Other schedules, routing, cooldowns and destination
settings still apply, so ending a snooze is not proof of delivery or a promise
to replay missed notifications.

These incident actions are not **Dismiss retained failures** or **Retry retained
deliveries**. They do not retry a failed notification. Check fresh readings and
the workload for recovery, and [Recent delivery activity](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
for delivery. Do not create an outage or lower thresholds to test an action.

### Quiet hours and notification holds

Open **Alerts → Schedule → Quiet hours** to enable a notification quiet period.
Choose the start and end times, an IANA timezone such as `Europe/London`, and
**Quiet days**. No selected days means no quiet period, even when enabled.
Times are interpreted in the selected timezone, not your browser's timezone.

Quiet hours hold **non-critical notifications**; they do not stop monitoring,
clear the alert or confirm recovery. Critical notifications remain eligible
unless you explicitly select their **Suppress categories** (Performance,
Storage or Offline). Selecting a category also holds its critical
notifications, including urgent failures; leave it unchecked when those must
still reach you. Other routing, mute and delivery policies still apply.

A window can cross midnight. Days refer to the **current local calendar day**:
with only Monday selected, `22:00`–`06:00` covers Monday's early morning and
late evening, not Tuesday's early morning. Select Tuesday too if that part of
the night must be quiet. The configured end minute is included: an end of
`06:00` remains quiet through `06:00:59`.

Eligible queued notifications are held for re-evaluation when the quiet period
ends. This is not a promise to send every held item at that instant: current
alert state, destination settings, other policies and provider failures still
matter. Use [Recent delivery activity](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
to distinguish a quiet-hours hold from a failed delivery; a settings Test skips
the queue and does not validate this schedule. Do not create an outage to test
it.

The example below makes every day quiet from `22:00` through the `06:00` end
minute in `Europe/London`, while leaving critical categories unsuppressed.
Quiet hours are **off by default**. This is an illustrative excerpt, not a
complete `alerts.json`: prefer the UI, and preserve existing rules and settings
rather than replacing the file with this example.

<details>
<summary><strong>Manual Configuration (JSON excerpt)</strong></summary>

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
      "end": "06:00",
      "timezone": "Europe/London",
      "days": {
        "monday": true,
        "tuesday": true,
        "wednesday": true,
        "thursday": true,
        "friday": true,
        "saturday": true,
        "sunday": true
      },
      "suppress": {
        "performance": false,
        "storage": false,
        "offline": false
      }
    }
  }
}
```
</details>

---

## Availability Checks (`availability_targets.enc`)

Availability checks are agentless probes for devices and services where Pulse
cannot install an agent or does not need full machine telemetry. Use them for
simple ping monitoring, TCP service checks, and HTTP/HTTPS status checks.

**Managed via UI**: Settings -> Monitoring -> Availability checks

Supported protocols:

| Protocol | Use case | Required fields |
| ---------- | ---------- | ---------------- |
| `icmp` | Ping-only reachability for devices, computers, and appliances | `address` |
| `ping` | API input alias for `icmp`; saved targets return `icmp` | `address` |
| `tcp` | A reachable port such as MQTT, SSH, or a custom service | `address`, `port` |
| `udp` | A request/response UDP service, or an open-or-filtered observation | `address`, `port`, UDP result policy |
| `http` / `https` | Web UI or health endpoint availability | `address`, optional `port`, optional `path` |

Saved targets include `name`, `targetKind` (`machine`, `service`, or
`device`), `address`, `protocol`, `enabled`, polling interval, timeout,
failure threshold, and an optional `linkedResourceId`. UDP targets select
`response_required` (send up to 512 UTF-8 bytes and require a response) or
`open_or_filtered` (silence is indeterminate, not proof of reachability); an
optional expected response must match exactly. HTTPS targets monitor
certificate validity by default and warn 30 days before expiry unless the
target overrides or disables that check. ICMP ping is the default probe for
new targets. Availability targets publish `network-endpoint` resources and can
raise downtime alerts after the configured failure threshold and certificate
alerts inside the configured expiry window.

Example API payload for simple ping monitoring:

```json
{
  "name": "Garage temperature sensor",
  "targetKind": "device",
  "address": "garage-sensor.local",
  "protocol": "ping",
  "enabled": true
}
```

Pulse stores and returns that target as `protocol: "icmp"` so dashboards,
alerts, and resource projections keep one canonical protocol value.

### External probes (Pro)

External probes change **where a check runs**, not where alerts are sent.
By default, checks run on the Pulse server. With the Pro `external_probe`
entitlement, select one or more connected agents under **Observation locations**
in the check editor. An agent on a cloud VM or at another site can observe a
service from outside its local network. Keep **This Pulse server** selected if
you also want a local observation; deselect it for agent-only checks. Pulse keeps
each location's evidence separate, so a failure on one path is not a universal
outage. The API uses `observationLocationIds`; see the
[target fields](API.md#availability-checks).

The assigned agent receives signed configuration, runs the check and reports
results back to Pulse. Only agents assigned to that target may submit results;
the server runs a local check only when **This Pulse server** is selected.
**The agent does not send
notifications directly.** The Pulse server must be running and able to reach
the notification destination to evaluate results and send alerts.

| Situation | What the check can tell you |
| --- | --- |
| Pulse is running and receives probe results | Target observations are evaluated through the normal alert and notification policies. |
| Pulse is running but probe results stop arriving | The affected location becomes indeterminate: "no recent report from probe agent". This is missing evidence, not proof the target is down. Other locations retain their own evidence. |
| Pulse is stopped or cannot reach its notification destination | An external agent cannot deliver Pulse alerts in its place. Use an independent external watchdog for this failure. |

The missing-report window is the longer of **five minutes or three check
intervals**, measured from server receipt time, not the agent's clock. During
normal server evaluation, eligible stale probes raise one warning per agent,
not per assigned check. Alert and connectivity policies still apply. If the
agent heartbeat is also offline, the host-offline alert owns the incident
instead of a duplicate probe warning. See the
[agent probe guide](UNIFIED_AGENT.md#external-probes-pro) for buffering limits.

For an outage of Pulse itself, configure an
[external watchdog](TROUBLESHOOTING.md#no-alert-when-pulse-power-or-internet-goes-down)
outside the power and network failure you need to detect, with an independently
reachable notification destination. Verify actual receipt in an authorised test
environment; a successful destination test does not prove outage coverage.

Existing paired Pulse Mobile/Relay users retain their current push path until
**31 March 2027**. Relay is no longer sold. Its instance-disconnect push does not
evaluate individual targets while Pulse is offline, and is not a permanent
substitute for the watchdog. See [Mobile retirement](RELAY.md).

If the entitlement lapses, the check resumes running from the Pulse server:
this changes its observation location and may change its result. Checks without
an agent assignment remain available in every edition.

### ICMP probe privileges

ICMP probes run the system `ping` binary, which needs the `CAP_NET_RAW`
capability. The systemd unit written by the installer hardens the service
with `NoNewPrivileges=true`, which strips ping's setuid bit and file
capabilities, so the unit also grants the capability directly with
`AmbientCapabilities=CAP_NET_RAW`. Units written by older versions of the
installer lack that line, and ICMP probes fail with
`icmp probe failed: ping: socktype: SOCK_RAW ... missing cap_net_raw+p capability`.

To fix an existing install, either re-run the install script (it rewrites
the unit) or add the capability as an override:

```bash
systemctl edit pulse   # pulse-backend on ProxmoxVE community-script installs
```

```ini
[Service]
AmbientCapabilities=CAP_NET_RAW
```

Then `systemctl daemon-reload && systemctl restart pulse`.

Docker installs are unaffected: Docker's default capability set includes
`NET_RAW`. If you run the container with `--cap-drop=ALL`, add
`--cap-add=NET_RAW` to keep ICMP probes working. TCP and HTTP/HTTPS probes
need no special privileges.

---

## 🔒 HTTPS / TLS

Enable HTTPS by providing certificate files via environment variables.

```bash
# Systemd
HTTPS_ENABLED=true
TLS_CERT_FILE=/etc/pulse/cert.pem
TLS_KEY_FILE=/etc/pulse/key.pem

# Docker
docker run --init -e HTTPS_ENABLED=true \
  -v /path/to/certs:/certs \
  -e TLS_CERT_FILE=/certs/cert.pem \
  -e TLS_KEY_FILE=/certs/key.pem ...
```

> **Important (Docker with HTTPS)**: Always use `--init` (or `init: true` in docker-compose) when enabling HTTPS. The Alpine-based healthcheck uses busybox `wget`, which spawns `ssl_client` subprocesses. Without an init process to reap them, these become zombie processes over time.

---

## 🛡️ Security Best Practices

1. **Permissions**: Ensure `.env` and `nodes.enc` are `600` (read/write by owner only).
2. **Backup hygiene**: Back up `.env` separately from `system.json`.
3. **Tokens**: Use scoped API tokens for agents instead of the admin password.

---

## 🔑 API Tokens

API tokens provide scoped, revocable access to Pulse. Manage them in
**Settings → API Access**, in the **Security** group. An account without token
management permission can review the inventory but cannot create or revoke tokens.

The token shown during first-run setup is the primary automation API token for
that Pulse instance, not your web login password. Do not reuse a full-access
setup token for every consumer. Give each agent, script, integration or display
only the scopes it needs, with a name that identifies its use. Tokens are shown
once; later rows show hints and metadata, not a recoverable secret. Store the
secret privately before dismissing it.

### Replace or revoke a token

Revoking a token stops consumers still using it from authenticating. For a
planned rotation, create a least-privilege replacement, install it privately
in each affected consumer and verify a fresh authenticated result before
revoking the old token. Last-used metadata alone does not account for every
consumer. Do not reinstall an agent or delete its saved identity just to
replace a credential; use its private token-file or managed configuration.

**If a token or kiosk link has been exposed, revoke it promptly**, even if that
interrupts monitoring or a display. Replace it through the same private setup
path. Do not leave a leaked token active while arranging a gradual rotation.
For a server-address change without credential exposure, follow
[agent retargeting](UNIFIED_AGENT.md#moving-pulse-to-a-new-address) instead.

Never put a token or session cookie in a diagnostic command, screenshot or
issue thread. For scripted API access, use the
[private header-file procedure](API.md#api-token-recommended), keep TLS
verification enabled and do not share verbose or trace output.

### Token Scopes

| Scope | Description |
| ------- | ------------- |
| `*` (Full access) | All permissions (legacy, not recommended) |
| `monitoring:read` | View dashboards, metrics, alerts |
| `monitoring:write` | Acknowledge, silence and clear alerts |
| `docker:report` | Docker / Podman agent telemetry submission |
| `docker:manage` | Docker / Podman container lifecycle actions (restart, stop) |
| `kubernetes:report` | Kubernetes agent telemetry submission |
| `kubernetes:manage` | Kubernetes cluster management |
| `agent:report` | Agent host telemetry submission |
| `agent:config:read` | Read agent config payloads |
| `agent:manage` | Agent lifecycle/configuration changes and unregistering; not needed by the reporting preset |
| `agent:exec` | Establish agent command WebSocket connections; not a reporting permission |
| `ai:chat` | Use Pulse Assistant chat and read knowledge |
| `ai:execute` | Use governed Patrol plans, approvals, actions, and history |
| `settings:read` | Read configuration |
| `settings:write` | Modify configuration, manage tokens and trigger updates |
| `audit:read` | Read audit events, verification results, summaries, and exports |

### Presets

Use **New token**, enter a name under **Create token**, select a **Quick preset**
and choose **Generate**. The current presets are:

| Preset | Scopes | Use Case |
| -------- | -------- | ---------- |
| **Kiosk / Monitoring** | `monitoring:read` | Read-only dashboard displays |
| **Agent** | `agent:report`, `agent:config:read` | Host telemetry and the agent's bound configuration |
| **Docker / Podman report** | `docker:report` | Docker / Podman agent (read-only) |
| **Docker / Podman manage** | `docker:report`, `docker:manage` | Docker / Podman agent with actions |
| **Settings read** | `settings:read` | Read-only config access |
| **Settings admin** | `settings:read`, `settings:write` | Full config access |
| **Audit read** | `audit:read` | Audit events, verification history, summaries and exports |

**Reporting is not remote execution.** The Agent preset does not grant
`agent:manage` or `agent:exec`; Docker / Podman report does not grant
`docker:manage`. Add lifecycle or action authority only for the operation you
intend, not to fix a missing reading. A settings-read token can read sensitive
configuration and diagnostics; it is not a public-display credential. See
[agent security](AGENT_SECURITY.md) for the separate collector and action-runner
requirements.

If **Patrol external agent** is offered, its scopes come from the current Patrol
requirements. Use the displayed set rather than guessing or adding **Full access**.
The separate **Full access** choice grants the legacy `*` wildcard, not a
least-privilege preset. Hiding controls does not reduce a token's permissions.

### Kiosk Mode

For a wall monitor, create a dedicated **Kiosk / Monitoring** token with only
`monitoring:read`. Never use **Full access**, a settings token or your
administrator's logged-in browser profile on an unattended display.

1. Open Pulse at its trusted **HTTPS** address and go to **Settings → API Access**.
2. Use **New token**, name the display, select **Kiosk / Monitoring** and
   choose **Generate**. Store the secret privately.
3. After closing the token-reveal dialog, the **Magic Kiosk Link** offers
   **Copy Link** for this monitoring-only token. Transfer that link privately
   to the display; do not put it in shared bookmarks, screenshots or reports.
4. Check the display using the link in a separate browser profile with no
   administrator session. Confirm it shows the intended monitoring data and
   does not offer settings or write actions. A hidden control alone is not
   proof of restricted access.

The link is a **bearer credential**: anyone with it can read the permitted
infrastructure data until the token expires or is revoked. The `kiosk=1`
option only hides navigation and filters; it does not authenticate a browser
or change the token's scopes.

Pulse removes the token from the page URL after loading and keeps it in that
tab's session storage. **The initial request already carried the token**;
HTTPS and the cleaned address bar do not remove copies from proxy/server logs,
browser history or the clipboard. Do not rely on this session surviving a
closed tab, browser restart or cleared site data. Keep the original link private
if the display needs it again, and clear clipboard copies after setup.

If the display or link is lost or exposed, revoke its dedicated token in
**API Access** and create a replacement. Revoking a display token should not
require rotating unrelated agents' credentials.

---

## TrueNAS

Pulse v6 supports first-class TrueNAS SCALE and CORE monitoring.

### Adding a TrueNAS Instance

1. Go to **Settings → TrueNAS**.
2. Click **Add Connection**.
3. Enter the URL (e.g., `https://truenas.local`) and an API key.
4. Click **Test Connection** to verify, then **Save**.

### Creating a TrueNAS API Key

On your TrueNAS system:
1. Navigate to the TrueNAS UI → **Settings → API Keys**.
2. Click **Add** and create a new read-only key.
3. Copy the key value and paste it into Pulse.

### What Gets Monitored

| Data | Where it appears |
|---|---|
| System info (CPU, memory, uptime) | Infrastructure page |
| Virtual machines | TrueNAS Overview |
| Apps | TrueNAS Overview |
| ZFS Pools & datasets | Storage page |
| Physical disks | Storage page |
| ZFS Snapshots | Recovery page |
| Replication tasks | Recovery page |
| TrueNAS alerts | Alerts page |

TrueNAS connections are stored encrypted in `truenas.enc`.

---

## Relay

The relay protocol provides end-to-end encrypted remote access foundations for Pulse mobile connectivity.

> Supported Pulse Mobile clients pair here using the generated QR code or deep link once relay is enabled for this instance.

### Configuration

1. Go to **Settings → Relay**.
2. Toggle relay **On**.
3. Use the **QR Code** or **Deep Link** to pair a supported Pulse Mobile client.

### Environment Overrides

For headless / container deployments that need to bootstrap relay without
going through the UI, two environment variables override the persisted
`relay.enc` values at load time:

| Variable | Description | Default |
|---|---|---|
| `PULSE_RELAY_ENABLED` | Enable/disable relay (`true`/`false`/`yes`/`no`/`1`/`0`). Unset or unrecognized values leave the file value untouched. | *(unset)* |
| `PULSE_RELAY_SERVER` | Override relay server URL. Must be a valid `ws://` or `wss://` URL with no userinfo, query, or fragment. Invalid values are logged and ignored. | `wss://relay.pulserelay.pro/ws/instance` |

Precedence: env vars beat the file. If you set `PULSE_RELAY_ENABLED=true`,
saving the relay form in **Settings → Relay** will then persist the
env-effective state to disk, so removing the env var later does not
automatically revert relay back to its previous file-stored state — clear
relay in the UI as well if you want to fully disable it.

### Security

- All data is encrypted end-to-end using ECDH key exchange.
- The relay server never sees plaintext monitoring data.
- Each mobile session has its own encryption channel.
- Requires a valid Relay, Pro, legacy Pro+, or Cloud license (gated by the `relay` feature key).

Relay config is stored encrypted in `relay.enc`.
