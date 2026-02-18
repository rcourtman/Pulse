# 🔌 Pulse API Reference

Pulse provides a comprehensive REST API for automation and integration.

**Base URL**: `http://<your-pulse-ip>:7655/api`

## 🔐 Authentication

Most API requests require authentication via one of the following methods:

### API Token (Recommended)
Pass the token in the `X-API-Token` header.
```bash
curl -H "X-API-Token: your-token" http://localhost:7655/api/health
```

### Bearer Token
```bash
curl -H "Authorization: Bearer your-token" http://localhost:7655/api/health
```

### Session Cookie
Standard browser session cookie (used by the UI).

Session endpoints:
- `POST /api/login` (sets `pulse_session` + `pulse_csrf`)
- `POST /api/logout` (clears session)

Login body:
```json
{ "username": "admin", "password": "secret", "rememberMe": true }
```

Public endpoints include:
- `GET /api/health`
- `GET /api/version`
- `GET /api/agent/version` (agent update checks)
- `GET /api/setup-script` (requires a setup token)

## 🔏 Scopes and Admin Access

Some endpoints require admin privileges and/or scopes. Common scopes include:
- `monitoring:read`
- `settings:read`
- `settings:write`
- `host-agent:config:read`
- `host-agent:manage`

Endpoints that require admin access are noted below.

---

## 📡 Core Endpoints

### System Health
`GET /api/health`
Check if Pulse is running.
```json
{
  "status": "healthy",
  "timestamp": 1700000000,
  "uptime": 3600,
  "devModeSSH": false
}
```

### System State
`GET /api/state`
Returns the complete state of your infrastructure (Nodes, VMs, Containers, Storage, Alerts). This is the main endpoint used by the dashboard.

### Simple Stats (HTML)
`GET /simple-stats`
Lightweight HTML status page for quick checks.

### Unified Resources
`GET /api/resources`
Returns the unified resource list with pagination + aggregations. Requires `monitoring:read`.

Query params:
- `type`: comma-separated list (e.g., `host`, `vm`, `lxc`, `container`, `docker-service`, `storage`, `pbs`, `pmg`, `k8s-cluster`, `k8s-node`, `pod`, `k8s-deployment`, `physical_disk`, `ceph`)
- `source`: comma-separated list (e.g., `proxmox`, `agent`, `docker`, `pbs`, `pmg`, `kubernetes`)
- `status`: comma-separated list (`online`, `offline`, `warning`, `unknown`)
- `parent`: parent resource ID
- `cluster`: cluster name
- `namespace`: Kubernetes namespace (filters Kubernetes resources only)
- `q`: name search (contains match)
- `tags`: comma-separated tags
- `page`: page number (default `1`)
- `limit`: page size (default `50`, max `100`)
- `sort`: `name` (default), `status`, `type`, `lastSeen`
- `order`: `asc` (default) or `desc`

Note: `GET /api/resources` is optimized for list views. Some large, platform-specific fields may be omitted from the list response and are only returned by `GET /api/resources/{id}`.

`GET /api/resources/stats`
Returns aggregations (counts + health rollups).

`GET /api/resources/k8s/namespaces?cluster=<clusterName>`
Returns namespace-level rollups (pods + deployments) for a Kubernetes cluster. Requires `monitoring:read`.
```json
{
  "cluster": "prod-k8s",
  "data": [
    {
      "namespace": "default",
      "pods": { "total": 12, "online": 10, "warning": 2, "offline": 0, "unknown": 0 },
      "deployments": { "total": 3, "online": 3, "warning": 0, "offline": 0, "unknown": 0 }
    }
  ]
}
```

`GET /api/resources/{id}`
Fetch a single resource by ID.

`GET /api/resources/{id}/children`
Returns child resources for the parent ID.

`GET /api/resources/{id}/metrics`
Returns the resource metrics payload.

`POST /api/resources/{id}/link`
Manually link two resources.
```json
{ "targetId": "resource-id", "reason": "optional note" }
```

`POST /api/resources/{id}/unlink`
Manually unlink two resources.
```json
{ "targetId": "resource-id", "reason": "optional note" }
```

`POST /api/resources/{id}/report-merge`
Report an incorrect merge (creates exclusions).
```json
{ "sources": ["proxmox", "agent"], "notes": "optional note" }
```

### Resource Metadata
User notes, tags, and custom URLs for resources.

- `GET /api/hosts/metadata` (admin or `monitoring:read`)
- `GET /api/hosts/metadata/{hostId}` (admin or `monitoring:read`)
- `PUT /api/hosts/metadata/{hostId}` (admin or `monitoring:write`)
- `DELETE /api/hosts/metadata/{hostId}` (admin or `monitoring:write`)

- `GET /api/guests/metadata` (admin or `monitoring:read`)
- `GET /api/guests/metadata/{guestId}` (admin or `monitoring:read`)
- `PUT /api/guests/metadata/{guestId}` (admin or `monitoring:write`)
- `DELETE /api/guests/metadata/{guestId}` (admin or `monitoring:write`)

- `GET /api/docker/metadata` (admin or `monitoring:read`)
- `GET /api/docker/metadata/{containerId}` (admin or `monitoring:read`)
- `PUT /api/docker/metadata/{containerId}` (admin or `monitoring:write`)
- `DELETE /api/docker/metadata/{containerId}` (admin or `monitoring:write`)

- `GET /api/docker/hosts/metadata` (admin or `monitoring:read`)
- `GET /api/docker/hosts/metadata/{hostId}` (admin or `monitoring:read`)
- `PUT /api/docker/hosts/metadata/{hostId}` (admin or `monitoring:write`)
- `DELETE /api/docker/hosts/metadata/{hostId}` (admin or `monitoring:write`)

### Version Info
`GET /api/version`
Returns version, build time, and update status.
Example response:
```json
{
  "version": "5.0.16",
  "buildTime": "2026-01-19T22:20:18Z",
  "channel": "stable",
  "deploymentType": "systemd",
  "updateAvailable": true,
  "latestVersion": "5.0.17"
}
```
Version fields are returned as plain semantic versions (no leading `v`).

---

## 🖥️ Nodes & Config

### Public Config
`GET /api/config`
Returns a small public config payload (update channel, auto-update enabled).

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

### Test Node Config (Validation Only)
`POST /api/config/nodes/test-config`
Validates node config without saving.

### Update Node
`PUT /api/config/nodes/{id}`

### Delete Node
`DELETE /api/config/nodes/{id}`

### Test Node (Legacy)
`POST /api/config/nodes/{id}/test`

### Refresh Cluster Nodes
`POST /api/config/nodes/{id}/refresh-cluster`

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

## 🧭 Setup & Discovery

### Setup Script (Public)
`GET /api/setup-script`
Returns the Proxmox/PBS setup script. Requires a temporary setup token (`auth_token`) in the query.

### Setup Script URL
`POST /api/setup-script-url` (auth)
Generates a one-time setup token and URL for `/api/setup-script`.

### Auto-Register (Public)
`POST /api/auto-register`
Auto-registers a node using the temporary setup token.

### Agent Install Command
`POST /api/agent-install-command` (auth)
Generates an API token and install command for agent-based Proxmox setup.

### Discovery
`GET /api/discover` (auth)
Runs network discovery.

### AI Discovery (Service Discovery)
Service discovery is used by Pulse Assistant and the UI to inventory web services and enrich links.

- `GET /api/discovery` (list summaries)
- `GET /api/discovery/status`
- `PUT /api/discovery/settings` (admin, `settings:write`)
- `GET /api/discovery/type/{type}`
- `GET /api/discovery/host/{host}`
- `GET /api/discovery/{type}/{host}/{id}`
- `POST /api/discovery/{type}/{host}/{id}` (trigger discovery, optional `force`)
- `DELETE /api/discovery/{type}/{host}/{id}`
- `GET /api/discovery/{type}/{host}/{id}/progress`
- `PUT /api/discovery/{type}/{host}/{id}/notes`

### Test Notification
`POST /api/test-notification` (auth)
Broadcasts a WebSocket test event.

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

### Recovery (formerly Backups / Snapshots)
Pulse v6 uses the recovery API to provide a platform-agnostic view of backup and snapshot artifacts.
See `docs/RECOVERY_CONTRACT.md` for the provider-neutral contract (subjects, points, rollups, and filter semantics).

- `GET /api/recovery/points`
  - Query params:
    - Core filters: `provider`, `kind`, `mode`, `outcome`, `subjectResourceId`, `rollupId`
    - Time window: `from` (RFC3339), `to` (RFC3339)
    - Paging: `page`, `limit`
    - Normalized filters: `q`, `cluster`, `node`, `namespace`, `scope=workload`, `verification` (`verified` | `unverified` | `unknown`)
- `GET /api/recovery/rollups`
  - Query params: `provider`, `kind`, `mode`, `outcome`, `subjectResourceId`, `rollupId`, `from` (RFC3339), `to` (RFC3339), `page`, `limit`
- `GET /api/recovery/series`
  - Returns per-day counts for the activity chart.
  - Query params: same filters as `/api/recovery/points` (except paging), plus `tzOffsetMinutes` (integer; UTC offset minutes for day bucketing)
- `GET /api/recovery/facets`
  - Returns distinct filter values (clusters/nodes/namespaces) and capability flags (size/verification/entity id present).
  - Query params: same filters as `/api/recovery/points` (except paging)

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

### Audit Webhooks (Pro)
- `GET /api/admin/webhooks/audit` (admin, `settings:read`)
- `POST /api/admin/webhooks/audit` (admin, `settings:write`)

### Advanced Reporting (Pro)
- `GET /api/admin/reports/generate` (admin, `settings:read`)
  - Query params: `format` (pdf/csv, default `pdf`), `resourceType`, `resourceId`, `metricType` (optional), `start`/`end` (RFC3339, optional; defaults to last 24h), `title` (optional)

### Queue and Dead-Letter Tools
- `GET /api/notifications/queue/stats` (admin)
- `GET /api/notifications/dlq` (admin)
- `POST /api/notifications/dlq/retry` (admin)
- `POST /api/notifications/dlq/delete` (admin)

---

## 🚨 Alerts

Alert configuration and history (requires `monitoring:read`/`monitoring:write`).

- `GET /api/alerts/config`
- `PUT /api/alerts/config`
- `POST /api/alerts/activate`
- `GET /api/alerts/active`
- `GET /api/alerts/history`
- `DELETE /api/alerts/history`
- `GET /api/alerts/incidents`
- `POST /api/alerts/incidents/note`
- `POST /api/alerts/bulk/acknowledge`
- `POST /api/alerts/bulk/clear`
- `POST /api/alerts/acknowledge` (body: `{ "id": "alert-id" }`)
- `POST /api/alerts/unacknowledge` (body: `{ "id": "alert-id" }`)
- `POST /api/alerts/clear` (body: `{ "id": "alert-id" }`)
- Legacy path-based endpoints: `POST /api/alerts/{id}/acknowledge`, `/unacknowledge`, `/clear`

---

## 🛡️ Security

### Security Status
`GET /api/security/status`
Returns authentication status, proxy auth state, and security posture flags.

### Change Password
`POST /api/security/change-password`
```json
{ "currentPassword": "old-pass", "newPassword": "new-pass" }
```
In Docker installs, the response includes a restart notice.

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

---

## 🧾 Audit Log (Pro)

These endpoints require admin access and the `settings:read` scope. On Community, the list endpoint returns an empty set and `persistentLogging: false`.

### List Audit Events
`GET /api/audit?limit=100&event=login&user=admin&success=true&startTime=2024-01-01T00:00:00Z&endTime=2024-01-31T23:59:59Z`

Response:
```json
{
  "events": [
    {
      "id": "6b3c9c3c-9a2f-4b3c-9a3b-3d0e8c5c5d45",
      "timestamp": "2024-01-12T10:15:30Z",
      "event": "login",
      "user": "admin",
      "ip": "10.0.0.10",
      "path": "/api/login",
      "success": true,
      "details": "Successful login",
      "signature": "..."
    }
  ],
  "total": 1,
  "persistentLogging": true
}
```

### Verify Audit Event Signature
`GET /api/audit/<id>/verify`

Response:
```json
{
  "available": true,
  "verified": true,
  "message": "Event signature verified"
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

### Apply Security Restart (Systemd Only)
`POST /api/security/apply-restart`
Applies auth changes by restarting the service (systemd deployments only).

---

## ⚙️ System Settings

### Get Settings
`GET /api/system/settings`
Retrieve current system settings.

### Update Settings
`POST /api/system/settings/update`
Update system settings. Requires admin + `settings:write`.

### Legacy System Settings (Read Only)
`GET /api/config/system`
Legacy system settings endpoint (read-only).

### Toggle Mock Mode
`GET /api/system/mock-mode`
`POST /api/system/mock-mode`
`PUT /api/system/mock-mode`
Enable or disable mock data generation (dev/demo only).

### SSH Config (Temperature Monitoring)
`POST /api/system/ssh-config`
Writes the SSH config used for temperature collection (requires setup token or auth).

### Verify Temperature SSH
`POST /api/system/verify-temperature-ssh`
Tests SSH connectivity for temperature collection (requires setup token or auth).

### Scheduler Health
`GET /api/monitoring/scheduler/health`
Returns scheduler health, DLQ, and breaker status. Requires `monitoring:read`.

### Updates (Admin)
- `GET /api/updates/check`
- `POST /api/updates/apply`
- `GET /api/updates/status`
- `GET /api/updates/stream`
- `GET /api/updates/plan?version=X.Y.Z` (optional `channel`, accepts `v` prefix)
- `GET /api/updates/history`
- `GET /api/updates/history/entry?id=<event_id>`

### Infrastructure Updates
- `GET /api/infra-updates` (requires `monitoring:read`)
- `GET /api/infra-updates/summary` (requires `monitoring:read`)
- `POST /api/infra-updates/check` (requires `monitoring:write`)
- `GET /api/infra-updates/host/{hostId}` (requires `monitoring:read`)
- `GET /api/infra-updates/{resourceId}` (requires `monitoring:read`)

### Diagnostics
- `GET /api/diagnostics` (auth)
- `POST /api/diagnostics/docker/prepare-token` (admin, `settings:write`)

### Logs (Admin)
- `GET /api/logs/stream` (server-sent stream)
- `GET /api/logs/download` (bundled logs)
- `GET /api/logs/level`
- `POST /api/logs/level` (set log level)

### Server Info
`GET /api/server/info`
Returns minimal server info for installer scripts.

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

## 💳 License (Pro / Cloud)

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

## 👥 RBAC / Role Management (Pro)

Role-based access control endpoints for managing roles and user assignments. Requires admin access and the `rbac` license feature.

### List Roles
`GET /api/admin/roles`
Returns all defined roles.

### Create Role
`POST /api/admin/roles`
```json
{
  "id": "operator",
  "name": "Operator",
  "description": "Can view and manage alerts",
  "permissions": [
    { "action": "read", "resource": "alerts" },
    { "action": "write", "resource": "alerts" }
  ]
}
```

### Update Role
`PUT /api/admin/roles/{id}`
Update an existing role's name, description, or permissions.

### Delete Role
`DELETE /api/admin/roles/{id}`

### List Users
`GET /api/admin/users`
Returns all users with their role assignments.

### Assign Role to User
`POST /api/admin/users/{username}/roles`
```json
{ "role_id": "operator" }
```

### Remove Role from User
`DELETE /api/admin/users/{username}/roles/{role_id}`

> **Note**: OIDC group-to-role mapping can automatically assign roles on login. See [OIDC.md](OIDC.md) for configuration.

---

## 🏢 Organizations (Enterprise)

Multi-tenant organization management. Requires `PULSE_MULTI_TENANT_ENABLED=true` and an Enterprise license with the `multi_tenant` feature. All endpoints require authentication.

See [MULTI_TENANT.md](MULTI_TENANT.md) for setup and architecture details.

### List Organizations
`GET /api/orgs` (requires `settings:read`)
Returns organizations accessible to the authenticated user.

### Create Organization
`POST /api/orgs` (requires `settings:write`, session auth only)
```json
{ "id": "acme-corp", "displayName": "Acme Corporation" }
```
The creator becomes the owner and first member. Organization IDs must be lowercase alphanumeric with hyphens, 3-64 characters.

### Get Organization
`GET /api/orgs/{id}` (requires `settings:read`)
Returns organization details. User must be a member.

### Update Organization
`PUT /api/orgs/{id}` (requires `settings:write`, session auth only)
```json
{ "displayName": "Updated Name" }
```
Admin or owner role required. The default organization cannot be updated.

### Delete Organization
`DELETE /api/orgs/{id}` (requires `settings:write`, session auth only)
Admin or owner role required. The default organization cannot be deleted.

### List Members
`GET /api/orgs/{id}/members` (requires `settings:read`)
Returns all members with their roles. User must be a member of the org.

### Add or Update Member
`POST /api/orgs/{id}/members` (requires `settings:write`, session auth only)
```json
{ "userId": "jane", "role": "editor" }
```
Roles: `owner`, `admin`, `editor`, `viewer`. Admin or owner role required. Setting role to `owner` transfers ownership (only current owner can do this). Default org members cannot be managed.

### Remove Member
`DELETE /api/orgs/{id}/members/{userId}` (requires `settings:write`, session auth only)
Admin or owner role required. The organization owner cannot be removed.

### List Outgoing Shares
`GET /api/orgs/{id}/shares` (requires `settings:read`)
Returns resources shared outbound from this organization to others.

### List Incoming Shares
`GET /api/orgs/{id}/shares/incoming` (requires `settings:read`)
Returns resources shared inbound to this organization from other organizations.

### Create Share
`POST /api/orgs/{id}/shares` (requires `settings:write`, session auth only)
```json
{
  "targetOrgId": "partner-org",
  "resourceType": "vm",
  "resourceId": "vm-101",
  "resourceName": "Web Server",
  "accessRole": "viewer"
}
```
Share a resource with another organization. Valid resource types: `vm`, `container`, `host`, `storage`, `pbs`, `pmg`. Access roles: `viewer`, `editor`, `admin`. Admin or owner role required on the source org.

### Delete Share
`DELETE /api/orgs/{id}/shares/{shareId}` (requires `settings:write`, session auth only)
Revoke a resource share. Admin or owner role required.

---

## 🤖 Pulse AI *(v5)*

**Pro gating:** endpoints labeled "(Pro)" require the relevant Pro/Cloud capability and return `402 Payment Required` if the feature is not licensed.

### Get AI Settings
`GET /api/settings/ai`
Returns current AI configuration (providers, models, patrol status). Requires admin + `settings:read`.

### Update AI Settings
`PUT /api/settings/ai/update` (or `POST /api/settings/ai/update`)
Configure AI providers, API keys, and preferences. Requires admin + `settings:write`.

### List Models
`GET /api/ai/models`
Lists models available to the configured providers (queried live from provider APIs).

### Provider Tests (Admin)
- `POST /api/ai/test`
- `POST /api/ai/test/{provider}`

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

### Assistant Chat & Sessions
- `GET /api/ai/status`
- `POST /api/ai/chat` (streaming)
- `GET /api/ai/sessions`
- `POST /api/ai/sessions`
- `DELETE /api/ai/sessions/{id}`
- `GET /api/ai/sessions/{id}/messages`
- `POST /api/ai/sessions/{id}/abort`
- `POST /api/ai/sessions/{id}/summarize`
- `GET /api/ai/sessions/{id}/diff`
- `POST /api/ai/sessions/{id}/fork`
- `POST /api/ai/sessions/{id}/revert`
- `POST /api/ai/sessions/{id}/unrevert`

### Legacy Chat Sessions (UI Sync)
- `GET /api/ai/chat/sessions`
- `GET /api/ai/chat/sessions/{id}`
- `PUT /api/ai/chat/sessions/{id}`
- `DELETE /api/ai/chat/sessions/{id}`

### Question Answers
- `POST /api/ai/question/{id}/answer`

### Kubernetes AI Analysis (Pro)
`POST /api/ai/kubernetes/analyze`
```json
{ "cluster_id": "cluster-id" }
```
Requires Pro/Cloud with the `kubernetes_ai` feature enabled.

### Alert Investigation (Pro)
`POST /api/ai/investigate-alert`
Runs a focused investigation for an alert payload (used by the UI).

### Patrol
- `GET /api/ai/patrol/autonomy`
- `PUT /api/ai/patrol/autonomy`
- `GET /api/ai/patrol/status`
- `GET /api/ai/patrol/findings`
- `DELETE /api/ai/patrol/findings` (clear all findings)
- `GET /api/ai/patrol/history`
- `GET /api/ai/patrol/runs`
- `GET /api/ai/patrol/stream` (Pro)
- `POST /api/ai/patrol/run` (admin, Pro)
- `POST /api/ai/patrol/acknowledge` (Pro)
- `POST /api/ai/patrol/dismiss`
- `POST /api/ai/patrol/findings/note`
- `POST /api/ai/patrol/resolve`
- `POST /api/ai/patrol/snooze` (Pro)
- `POST /api/ai/patrol/suppress` (Pro)
- `GET /api/ai/patrol/suppressions` (Pro)
- `POST /api/ai/patrol/suppressions` (Pro)
- `DELETE /api/ai/patrol/suppressions/{id}` (Pro)
- `GET /api/ai/patrol/dismissed` (Pro)

### Findings & Investigations
- `GET /api/ai/unified/findings`
- `GET /api/ai/findings/{id}/investigation`
- `GET /api/ai/findings/{id}/investigation/messages`
- `POST /api/ai/findings/{id}/reinvestigate`
- `POST /api/ai/findings/{id}/reapprove` (Pro)

### Approvals & Command Execution (Pro)
- `GET /api/ai/approvals`
- `GET /api/ai/approvals/{id}`
- `POST /api/ai/approvals/{id}/approve`
- `POST /api/ai/approvals/{id}/deny`
- `POST /api/ai/run-command` (execute an approved command)
- `GET /api/ai/agents` (connected agents via `/api/agent/ws`)

### Remediation Plans (Pro)
- `GET /api/ai/remediation/plans`
- `GET /api/ai/remediation/plan?plan_id=<id>`
- `POST /api/ai/remediation/approve`
- `POST /api/ai/remediation/execute`
- `POST /api/ai/remediation/rollback`

Request bodies:
- `approve`: `{ "plan_id": "...", "approved_by": "api" }`
- `execute`: `{ "execution_id": "...", "plan_id": "..." }`
- `rollback`: `{ "execution_id": "..." }`

### Intelligence & Forecasting
- `GET /api/ai/intelligence`
- `GET /api/ai/intelligence/patterns`
- `GET /api/ai/intelligence/predictions`
- `GET /api/ai/intelligence/correlations`
- `GET /api/ai/intelligence/changes`
- `GET /api/ai/intelligence/baselines`
- `GET /api/ai/intelligence/remediations`
- `GET /api/ai/intelligence/anomalies`
- `GET /api/ai/intelligence/learning`
- `GET /api/ai/forecast` (params: `resource_id`, `metric`, optional `resource_name`, `horizon_hours`, `threshold`)
- `GET /api/ai/forecasts/overview` (params: `metric`, `horizon_hours`, `threshold`)
- `GET /api/ai/learning/preferences` (optional `resource_id`)
- `GET /api/ai/proxmox/events`
- `GET /api/ai/proxmox/correlations`
- `GET /api/ai/incidents` (optional `resource_id`, `limit`)
- `GET /api/ai/incidents/{resourceId}` (optional `limit`)
- `GET /api/ai/circuit/status`

### Knowledge Base
- `GET /api/ai/knowledge?guest_id=<id>`
- `POST /api/ai/knowledge/save`
- `POST /api/ai/knowledge/delete`
- `GET /api/ai/knowledge/export?guest_id=<id>`
- `POST /api/ai/knowledge/import`
- `POST /api/ai/knowledge/clear`

### Debug
- `GET /api/ai/debug/context` (admin)

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

Query params:
- `resourceType` (required): `node`, `vm`, `container`, `storage`, `dockerHost`, `dockerContainer`
- `resourceId` (required)
- `metric` (optional): `cpu`, `memory`, `disk`, etc. Omit for all metrics
- `range` (optional): `1h`, `6h`, `12h`, `24h`, `1d`, `7d`, `30d`, `90d` (default `24h`; duration strings also accepted)
- `maxPoints` (optional): Downsample to a target number of points

> **License**: Requests beyond `7d` require the Pro/Cloud `long_term_metrics` feature. Unlicensed requests return `402 Payment Required`.
> **Aliases**: `guest` (VM/LXC) and `docker` (Docker container) are accepted, but persistent store data uses the canonical types above.

---

## 🤖 Agent Endpoints

### Unified Agent (Recommended)
`GET /download/pulse-agent`
Downloads the unified agent binary. Without `arch`, Pulse serves the local binary on the server host.

Optional query:
- `?arch=linux-amd64` (supported: `linux-amd64`, `linux-arm64`, `linux-armv7`, `linux-armv6`, `linux-386`, `darwin-amd64`, `darwin-arm64`, `freebsd-amd64`, `freebsd-arm64`, `windows-amd64`, `windows-arm64`, `windows-386`)

The response includes `X-Checksum-Sha256` for verification.

The unified agent combines host, Docker, and Kubernetes monitoring. Use `--enable-docker` or `--enable-kubernetes` to enable additional metrics.

See [UNIFIED_AGENT.md](UNIFIED_AGENT.md) for installation instructions.

### Agent Version
`GET /api/agent/version`
Returns the current server version for agent update checks.

### Unified Agent Installer Script
`GET /install.sh`
Serves the universal `install.sh` used to install `pulse-agent` on target machines.

`GET /api/install/install.sh`
API-prefixed alias for the unified agent installer script.

### Unified Agent Installer (Windows)
`GET /install.ps1`
Serves the PowerShell installer for Windows.

`GET /api/install/install.ps1`
API-prefixed alias for the unified agent PowerShell installer.

### Docker Server Installer Script
`GET /api/install/install-docker.sh`
Serves the turnkey Docker installer script that generates a `docker-compose.yml` and `.env`.

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

### Host Agent Linking (Admin)
- `POST /api/agents/host/link` (admin, `host-agent:manage`)
- `POST /api/agents/host/unlink` (admin, `host-agent:manage`)

### Agent Remote Config
`GET /api/agents/host/{agent_id}/config`  
Returns the server-side config payload for an agent (used by remote config and debugging). Requires `host-agent:config:read`.

`PATCH /api/agents/host/{agent_id}/config` (admin, `host-agent:manage`)  
Updates server-side config for an agent (e.g., `commandsEnabled`).

### Docker Agent Management (Admin)
- `POST /api/agents/docker/commands/{commandId}/ack` (`docker:report`)
- `DELETE /api/agents/docker/hosts/{hostId}` (`docker:manage`, supports `?hide=true` or `?force=true`)
- `POST /api/agents/docker/hosts/{hostId}/allow-reenroll` (`docker:manage`)
- `PUT /api/agents/docker/hosts/{hostId}/unhide` (`docker:manage`)
- `PUT /api/agents/docker/hosts/{hostId}/pending-uninstall` (`docker:manage`)
- `PUT /api/agents/docker/hosts/{hostId}/display-name` (`docker:manage`)
- `POST /api/agents/docker/hosts/{hostId}/check-updates` (`docker:manage`)
- `POST /api/agents/docker/hosts/{hostId}/update-all` (`docker:manage`)
- `POST /api/agents/docker/containers/update` (`docker:manage`)

### Kubernetes Agent Management (Admin)
- `DELETE /api/agents/kubernetes/clusters/{clusterId}` (`kubernetes:manage`, supports `?hide=true` or `?force=true`)
- `POST /api/agents/kubernetes/clusters/{clusterId}/allow-reenroll` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/unhide` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/pending-uninstall` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/display-name` (`kubernetes:manage`)

### Agent Profiles (Pro)
`GET /api/admin/profiles` (admin, Pro)
`POST /api/admin/profiles` (admin, Pro)
`GET /api/admin/profiles/{id}` (admin, Pro)
`PUT /api/admin/profiles/{id}` (admin, Pro)
`DELETE /api/admin/profiles/{id}` (admin, Pro)
`GET /api/admin/profiles/schema` (admin, Pro)
`POST /api/admin/profiles/validate` (admin, Pro)
`POST /api/admin/profiles/suggestions` (admin, Pro)
`GET /api/admin/profiles/changelog` (admin, Pro)
`GET /api/admin/profiles/deployments` (admin, Pro)
`POST /api/admin/profiles/deployments` (admin, Pro)
`GET /api/admin/profiles/{id}/versions` (admin, Pro)
`POST /api/admin/profiles/{id}/rollback/{version}` (admin, Pro)
`GET /api/admin/profiles/assignments` (admin, Pro)
`POST /api/admin/profiles/assignments` (admin, Pro)
`DELETE /api/admin/profiles/assignments/{agent_id}` (admin, Pro)

---

## 🔌 WebSocket Endpoints

- `GET /ws` – Primary UI WebSocket (browser sessions).
- `GET /socket.io/` – Legacy Socket.IO compatibility endpoint.
- `GET /api/agent/ws` – Agent WebSocket used for AI command execution.

---

> **Note**: This is a summary of the most common endpoints. For a complete list, inspect the network traffic of the Pulse dashboard or check the source code in `internal/api/router.go`.
