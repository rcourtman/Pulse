# 🔌 Pulse API Reference

Pulse provides a comprehensive REST API for automation and integration.

**Base URL**: `http://<your-pulse-ip>:7655/api`

## 🔐 Authentication

Most API requests require authentication via one of the following methods:

**1. API Token (Recommended)**
Pass the token in the `X-API-Token` header.
```bash
curl -H "X-API-Token: your-token" http://localhost:7655/api/health
```

**2. Bearer Token**
```bash
curl -H "Authorization: Bearer your-token" http://localhost:7655/api/health
```

**3. Session Cookie**
Standard browser session cookie (used by the UI).

Public endpoints include:
- `GET /api/health`
- `GET /api/version`

## 🔏 Scopes and Admin Access

Some endpoints require admin privileges and/or scopes. Common scopes include:
- `monitoring:read`
- `settings:read`
- `settings:write`

Endpoints that require admin access are noted below.

---

## 📡 Core Endpoints

### System Health
`GET /api/health`
Check if Pulse is running.
```json
{ "status": "healthy", "uptime": 3600 }
```

### System State
`GET /api/state`
Returns the complete state of your infrastructure (Nodes, VMs, Containers, Storage, Alerts). This is the main endpoint used by the dashboard.

### Version Info
`GET /api/version`
Returns version, build time, and update status.

---

## 🖥️ Nodes & Config

### List Nodes
`GET /api/config/nodes`

### Add Node
`POST /api/config/nodes`
```json
{
  "type": "pve",
  "name": "Proxmox 1",
  "host": "https://192.168.1.10:8006",
  "user": "root@pam",
  "password": "password"
}
```

### Test Connection
`POST /api/config/nodes/test-connection`
Validate credentials before saving.

### Export Configuration
`POST /api/config/export` (admin or API token)
Request body:
```json
{ "passphrase": "use-a-strong-passphrase" }
```
Returns an encrypted export bundle in `data`. Passphrases must be at least 12 characters.

### Import Configuration
`POST /api/config/import` (admin)
Request body:
```json
{
  "data": "<exported-bundle>",
  "passphrase": "use-a-strong-passphrase"
}
```

---

## 📊 Metrics & Charts

### Chart Data
`GET /api/charts?range=1h`
Returns time-series data for CPU, Memory, and Storage.
**Ranges**: `5m`, `15m`, `30m`, `1h`, `4h`, `12h`, `24h`, `7d`

### Storage Charts
`GET /api/storage-charts`
Returns storage chart data.

### Storage Stats
`GET /api/storage/`
Detailed storage usage per node and pool.

### Backup History
`GET /api/backups/unified`
Combined view of PVE and PBS backups.

Other backup endpoints:
- `GET /api/backups`
- `GET /api/backups/pve`
- `GET /api/backups/pbs`

---

## 🔔 Notifications

### Send Test Notification
`POST /api/notifications/test` (admin)
Triggers a test alert to all configured channels.

### Email, Apprise, and Webhooks
- `GET /api/notifications/email` (admin)
- `PUT /api/notifications/email` (admin)
- `GET /api/notifications/apprise` (admin)
- `PUT /api/notifications/apprise` (admin)
- `GET /api/notifications/webhooks` (admin)
- `POST /api/notifications/webhooks` (admin)
- `PUT /api/notifications/webhooks/<id>` (admin)
- `DELETE /api/notifications/webhooks/<id>` (admin)
- `POST /api/notifications/webhooks/test` (admin)
- `GET /api/notifications/webhook-templates` (admin)
- `GET /api/notifications/webhook-history` (admin)
- `GET /api/notifications/email-providers` (admin)
- `GET /api/notifications/health` (admin)

### Queue and Dead-Letter Tools
- `GET /api/notifications/queue/stats` (admin)
- `GET /api/notifications/dlq` (admin)
- `POST /api/notifications/dlq/retry` (admin)
- `POST /api/notifications/dlq/delete` (admin)

---

## 🛡️ Security

### Security Status
`GET /api/security/status`
Returns authentication status, proxy auth state, and security posture flags.

### List API Tokens
`GET /api/security/tokens`

### Create API Token
`POST /api/security/tokens`
```json
{ "name": "ansible-script", "scopes": ["monitoring:read"] }
```

### Revoke Token
`DELETE /api/security/tokens/<id>`

### Recovery (Localhost or Recovery Token)
`POST /api/security/recovery`
Supports actions:
- `generate_token` (localhost only)
- `disable_auth`
- `enable_auth`

`GET /api/security/recovery` returns recovery mode status.

### Reset Account Lockout (Admin)
`POST /api/security/reset-lockout`
```json
{ "identifier": "admin" }
```
Identifier can be a username or IP address.

### Regenerate API Token (Admin)
`POST /api/security/regenerate-token`

Returns a new raw token (shown once) and updates stored hashes:
```json
{
  "success": true,
  "token": "raw-token",
  "deploymentType": "systemd",
  "requiresRestart": false,
  "message": "New API token generated and active immediately! Save this token - it won't be shown again."
}
```

### Validate API Token (Admin)
`POST /api/security/validate-token`
```json
{ "token": "raw-token" }
```
Returns:
```json
{ "valid": true, "message": "Token is valid" }
```

### Bootstrap Token Validation (Public)
`POST /api/security/validate-bootstrap-token`

Provide the token via header `X-Setup-Token` or JSON body:
```json
{ "token": "bootstrap-token" }
```

Returns `204 No Content` on success.

### Quick Security Setup (Public, bootstrap token required)
`POST /api/security/quick-setup`

Requires a valid bootstrap token (header `X-Setup-Token`) or an authenticated session.

```json
{
  "username": "admin",
  "password": "StrongPass!1",
  "apiToken": "token",
  "enableNotifications": false,
  "darkMode": false,
  "force": false,
  "setupToken": "optional-bootstrap-token"
}
```

---

## ⚙️ System Settings

### Get Settings
`GET /api/system/settings`
Retrieve current system settings.

### Update Settings
`POST /api/system/settings/update`
Update system settings. Requires admin + `settings:write`.

### Toggle Mock Mode
`POST /api/system/mock-mode`
Enable or disable mock data generation (dev/demo only).

### Scheduler Health
`GET /api/monitoring/scheduler/health`
Returns scheduler health, DLQ, and breaker status. Requires `monitoring:read`.

### Updates (Admin)
- `GET /api/updates/check`
- `POST /api/updates/apply`
- `GET /api/updates/status`
- `GET /api/updates/stream`
- `GET /api/updates/plan?version=vX.Y.Z` (optional `channel`)
- `GET /api/updates/history`
- `GET /api/updates/history/entry?id=<event_id>`

---

## 🔑 OIDC / SSO

### Get OIDC Config
`GET /api/security/oidc`
Retrieve current OIDC provider settings.

### Update OIDC Config
`POST /api/security/oidc`
Configure OIDC provider details (Issuer, Client ID, etc).

### Login
`GET /api/oidc/login`
Initiate OIDC login flow.

---

## 💳 License (Pulse Pro)

### License Status (Admin)
`GET /api/license/status`

### License Features (Authenticated)
`GET /api/license/features`

### Activate License (Admin)
`POST /api/license/activate`
```json
{ "license_key": "PASTE_KEY_HERE" }
```

### Clear License (Admin)
`POST /api/license/clear`

---

## 🤖 Pulse AI *(v5)*

**Pro gating:** endpoints labeled "(Pro)" require a Pulse Pro license and return `402 Payment Required` if the feature is not licensed.

### Get AI Settings
`GET /api/settings/ai`
Returns current AI configuration (providers, models, patrol status). Requires admin + `settings:read`.

### Update AI Settings
`PUT /api/settings/ai/update` (or `POST /api/settings/ai/update`)
Configure AI providers, API keys, and preferences. Requires admin + `settings:write`.

### List Models
`GET /api/ai/models`
Lists models available to the configured providers (queried live from provider APIs).

### OAuth (Anthropic)
- `POST /api/ai/oauth/start` (admin)
- `POST /api/ai/oauth/exchange` (admin, manual code input)
- `GET /api/ai/oauth/callback` (public, IdP redirect)
- `POST /api/ai/oauth/disconnect` (admin)

### Execute (Chat + Tools)
`POST /api/ai/execute`
Runs an AI request which may return tool calls, findings, or suggested actions.

### Execute (Streaming)
`POST /api/ai/execute/stream`
Streaming variant of execute (used by the UI for incremental responses).

### Kubernetes AI Analysis (Pro)
`POST /api/ai/kubernetes/analyze`
```json
{ "cluster_id": "cluster-id" }
```
Requires a Pulse Pro license with the `kubernetes_ai` feature enabled.

### Patrol
- `GET /api/ai/patrol/status`
- `GET /api/ai/patrol/findings`
- `DELETE /api/ai/patrol/findings` (clear all findings)
- `GET /api/ai/patrol/history`
- `GET /api/ai/patrol/runs`
- `GET /api/ai/patrol/stream` (Pro)
- `POST /api/ai/patrol/run` (admin, Pro)
- `POST /api/ai/patrol/acknowledge` (Pro)
- `POST /api/ai/patrol/dismiss`
- `POST /api/ai/patrol/resolve`
- `POST /api/ai/patrol/snooze` (Pro)
- `POST /api/ai/patrol/suppress` (Pro)
- `GET /api/ai/patrol/suppressions` (Pro)
- `POST /api/ai/patrol/suppressions` (Pro)
- `DELETE /api/ai/patrol/suppressions/{id}` (Pro)
- `GET /api/ai/patrol/dismissed` (Pro)

### Cost Tracking
- `GET /api/ai/cost/summary`
- `POST /api/ai/cost/reset` (admin)
- `GET /api/ai/cost/export` (admin)

## 📈 Metrics Store (v5)

Auth required: `monitoring:read`.

### Store Stats
`GET /api/metrics-store/stats`
Returns stats for the persistent metrics store (SQLite-backed).

### History
`GET /api/metrics-store/history`
Returns historical metric series for a resource and time range.

---

## 🤖 Agent Endpoints

### Unified Agent (Recommended)
`GET /download/pulse-agent`
Downloads the unified agent binary. Without `arch`, Pulse serves the local binary on the server host.

Optional query:
- `?arch=linux-amd64` (supported: `linux-amd64`, `linux-arm64`, `linux-armv7`, `linux-armv6`, `linux-386`, `darwin-amd64`, `darwin-arm64`, `windows-amd64`, `windows-arm64`, `windows-386`)

The response includes `X-Checksum-Sha256` for verification.

The unified agent combines host, Docker, and Kubernetes monitoring. Use `--enable-docker` or `--enable-kubernetes` to enable additional metrics.

See [UNIFIED_AGENT.md](UNIFIED_AGENT.md) for installation instructions.

### Unified Agent Installer Script
`GET /install.sh`
Serves the universal `install.sh` used to install `pulse-agent` on target machines.

### Unified Agent Installer (Windows)
`GET /install.ps1`
Serves the PowerShell installer for Windows.

### Legacy Agents (Deprecated)
`GET /download/pulse-host-agent` - *Deprecated, use pulse-agent*
`GET /download/pulse-docker-agent` - *Deprecated, use pulse-agent --enable-docker*

Host-agent downloads accept `?platform=<os>&arch=<arch>` and expose a checksum endpoint:
- `/download/pulse-host-agent.sha256?platform=linux&arch=amd64`

Legacy install/uninstall scripts:
- `GET /install-docker-agent.sh`
- `GET /install-container-agent.sh`
- `GET /install-host-agent.sh`
- `GET /install-host-agent.ps1`
- `GET /uninstall-host-agent.sh`
- `GET /uninstall-host-agent.ps1`

### Submit Reports
`POST /api/agents/host/report` - Host metrics
`POST /api/agents/docker/report` - Docker container metrics
`POST /api/agents/kubernetes/report` - Kubernetes cluster metrics

### Host Agent Management
`GET /api/agents/host/lookup?id=<host_id>`  
`GET /api/agents/host/lookup?hostname=<hostname>`  
Looks up a host by ID or hostname/display name. Requires `host-agent:report`.

`POST /api/agents/host/uninstall`  
Host agent self-unregister during uninstall. Requires `host-agent:report`.

`POST /api/agents/host/unlink` (admin, `host-agent:manage`)  
Unlinks a host agent from a node.

`DELETE /api/agents/host/{host_id}` (admin, `host-agent:manage`)  
Removes a host agent from state.

### Agent Remote Config
`GET /api/agents/host/{agent_id}/config`  
Returns the server-side config payload for an agent (used by remote config and debugging). Requires `host-agent:report`.

`PATCH /api/agents/host/{agent_id}/config` (admin, `host-agent:manage`)  
Updates server-side config for an agent (e.g., `commandsEnabled`).

### Agent Profiles (Pro)
`GET /api/admin/profiles` (admin, Pro)
`POST /api/admin/profiles` (admin, Pro)
`PUT /api/admin/profiles/{id}` (admin, Pro)
`DELETE /api/admin/profiles/{id}` (admin, Pro)
`GET /api/admin/profiles/assignments` (admin, Pro)
`POST /api/admin/profiles/assignments` (admin, Pro)
`DELETE /api/admin/profiles/assignments/{agent_id}` (admin, Pro)

---

## 🌡️ Temperature Proxy (Legacy)

These endpoints are only available when legacy `pulse-sensor-proxy` support is enabled.

- `POST /api/temperature-proxy/register` (proxy registration)
- `GET /api/temperature-proxy/authorized-nodes` (proxy sync)
- `DELETE /api/temperature-proxy/unregister` (admin)
- `GET /api/temperature-proxy/install-command` (admin, `settings:write`)
- `GET /api/temperature-proxy/host-status` (admin, `settings:read`)

Legacy migration helper:
- `GET /api/install/migrate-temperature-proxy.sh`

> **Note**: This is a summary of the most common endpoints. For a complete list, inspect the network traffic of the Pulse dashboard or check the source code in `internal/api/router.go`.
