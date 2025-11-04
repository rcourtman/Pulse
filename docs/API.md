# Pulse API Documentation

## Overview

Pulse provides a REST API for monitoring and managing Proxmox VE and PBS instances. All API endpoints are prefixed with `/api`.

## Authentication

Pulse supports multiple authentication methods that can be used independently or together:

> **Service name note:** Systemd deployments use `pulse.service`. If your host still uses the legacy `pulse-backend.service`, substitute that name in the commands below.

### Password Authentication
Set a username and password for web UI access. Passwords are hashed with bcrypt (cost 12) for security.

```bash
# Systemd
sudo systemctl edit pulse
# Add:
[Service]
Environment="PULSE_AUTH_USER=admin"
Environment="PULSE_AUTH_PASS=your-secure-password"

# Docker
docker run -e PULSE_AUTH_USER=admin -e PULSE_AUTH_PASS=your-password rcourtman/pulse:latest
```

Once set, users must login via the web UI. The password can be changed from Settings → Security.

### API Token Authentication
For programmatic API access and automation. Manage tokens via **Settings → Security → API tokens** or the `/api/security/tokens` endpoints.

**API-Only Mode**: If at least one API token is configured (no password auth), the UI remains accessible in read-only mode while API modifications require a valid token.

```bash
# Systemd
sudo systemctl edit pulse
# Add:
[Service]
Environment="API_TOKENS=token-a,token-b"

# Docker
docker run -e API_TOKENS=token-a,token-b rcourtman/pulse:latest
```

### Using Authentication

```bash
# With API token (header)
curl -H "X-API-Token: your-secure-token" http://localhost:7655/api/health

# With API token (Authorization header)
curl -H "Authorization: Bearer your-secure-token" http://localhost:7655/api/health

# (Query parameters are rejected to avoid leaking tokens in logs or referrers.)

# With session cookie (after login)
curl -b cookies.txt http://localhost:7655/api/health
```

> Legacy note: The `API_TOKEN` environment variable is still honored for backwards compatibility. When both `API_TOKEN` and `API_TOKENS` are supplied, Pulse merges them and prefers the newest token when presenting hints.

### Security Features

When authentication is enabled, Pulse provides enterprise-grade security:

- **CSRF Protection**: All state-changing requests require a CSRF token
- **Rate Limiting** (enhanced in v4.24.0): 500 req/min general, 10 attempts/min for authentication
  - **New**: All responses include rate limit headers:
    - `X-RateLimit-Limit`: Maximum requests per window
    - `X-RateLimit-Remaining`: Requests remaining in current window
    - `X-RateLimit-Reset`: Unix timestamp when the limit resets
    - `Retry-After`: Seconds to wait before retrying (on 429 responses)
- **Account Lockout**: Locks after 5 failed attempts (15 minute cooldown) with clear feedback
- **Secure Sessions**: HttpOnly cookies, 24-hour expiry
- **Security Headers**: CSP, X-Frame-Options, X-Content-Type-Options, etc.
- **Audit Logging**: All security events are logged

### CSRF Token Usage

When using session authentication, include the CSRF token for state-changing requests:

```javascript
// Get CSRF token from cookie
const csrfToken = getCookie('pulse_csrf');

// Include in request header
fetch('/api/nodes', {
  method: 'POST',
  headers: {
    'X-CSRF-Token': csrfToken,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify(data)
});
```

## Common Response Headers

Most endpoints emit a pair of diagnostic headers to help with troubleshooting:

- `X-Request-ID` &mdash; unique identifier assigned to each HTTP request. The same value appears in Pulse logs, enabling quick correlation when raising support tickets or hunting through log files.
- `X-RateLimit-*` family (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`) &mdash; surfaced when rate limiting is enabled (default in v4.24.0+).
- `X-Diagnostics-Cached-At` &mdash; returned only by `/api/diagnostics`; indicates when the current diagnostics payload was generated.

## Core Endpoints

### Health Check
Check if Pulse is running and healthy.

```bash
GET /api/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": 1754995749,
  "uptime": 166.187561244
}
```

**Optional fields** (v4.24.0+, appear when relevant):
```json
{
  "status": "healthy",
  "timestamp": 1754995749,
  "uptime": 166.187561244,
  "proxyInstallScriptAvailable": true,
  "devModeSSH": false
}
```

### Version Information
Get current Pulse version and build info.

```bash
GET /api/version
```

Response (v4.24.0+):
```json
{
  "version": "v4.24.0",
  "build": "release",
  "buildTime": "2025-10-20T10:30:00Z",
  "runtime": "go",
  "goVersion": "1.23.2",
  "channel": "stable",
  "deploymentType": "systemd",
  "isDocker": false,
  "isDevelopment": false,
  "updateAvailable": false,
  "latestVersion": "v4.24.0"
}
```

### System State
Get complete system state including all nodes and their metrics.

```bash
GET /api/state
```

Response payload includes dedicated collections for each subsystem:

- `nodes`: Proxmox VE nodes with live resource metrics and connection health
- `vms` / `containers`: Guest workloads with CPU, memory, disk, network, and power state
- `dockerHosts`: Hosts that report through the Docker agent, including container inventory
  - Each host entry includes `issues` (restart loops, health check failures), `lastSeen`, `agentVersion`, and a flattened list of labelled containers so you can display the same insights the UI shows.
- `storage`: Per-node storage with capacity and usage metadata
- `cephClusters`: Ceph health summaries, daemon counts, and pool capacity (see below)
- `physicalDisks`: SMART/enclosure telemetry when physical disk monitoring is enabled
- `pbs`: Proxmox Backup Server inventory, job status, and datastore utilisation
- `pmg`: Proxmox Mail Gateway health and analytics (mail totals, queues, spam distribution)
- `pveBackups` / `pbsBackups`: Backup history across snapshots, storage jobs, and PBS
- `stats`: System-wide aggregates (uptime, versions, counts)
- `activeAlerts`: Currently firing alerts with hysteresis-aware metadata
- `performance`: Cached chart series for the dashboard

#### Ceph Cluster Data

When Pulse detects Ceph-backed storage (RBD, CephFS, etc.), the `cephClusters` array surfaces detailed health information gathered via `/cluster/ceph/status` and `/cluster/ceph/df`:

```json
{
  "cephClusters": [
    {
      "id": "pve-cluster-4f7c...",
      "instance": "pve-cluster",
      "health": "HEALTH_OK",
      "healthMessage": "All OSDs are running",
      "totalBytes": 128178802368000,
      "usedBytes": 87236608000000,
      "availableBytes": 40942194432000,
      "usagePercent": 68.1,
      "numMons": 3,
      "numMgrs": 2,
      "numOsds": 12,
      "numOsdsUp": 12,
      "numOsdsIn": 12,
      "numPGs": 768,
      "pools": [
        { "id": 1, "name": "cephfs_data", "storedBytes": 7130316800000, "availableBytes": 1239814144000, "objects": 1024, "percentUsed": 64.2 }
      ],
      "services": [
        { "type": "mon", "running": 3, "total": 3 },
        { "type": "mgr", "running": 2, "total": 2 }
      ],
      "lastUpdated": 1760219854
    }
  ]
}
```

Each service entry lists offline daemons in `message` when present (for example, `Offline: mgr.x@pve2`), making it easy to highlight degraded components in custom tooling.

### Scheduler Health

Monitor Pulse's internal adaptive polling scheduler and circuit breaker status.

```bash
GET /api/monitoring/scheduler/health
```

This endpoint provides detailed metrics about:
- Task queue depths and processing times
- Circuit breaker states per node
- Backoff delays and retry schedules
- Dead-letter queue entries (tasks that repeatedly fail)
- Instance-level staleness tracking

See [Scheduler Health API Documentation](api/SCHEDULER_HEALTH.md) for complete response schema and examples.

**Key use cases:**
- Monitor for polling backlogs
- Detect connectivity issues via circuit breaker trips
- Track node health and responsiveness
- Identify failing tasks in the dead-letter queue

#### PMG Mail Gateway Data

When PMG instances are configured, the `pmg` array inside `/api/state` surfaces consolidated health and mail analytics for each gateway:

- `status`/`connectionHealth` reflect reachability (`online` + `healthy` when the API responds).
- `nodes` lists discovered cluster members and their reported role.
- `mailStats` contains rolling totals for the configured timeframe (default: last 24 hours).
- `mailCount` provides hourly buckets for the last day; useful for charting trends.
- `spamDistribution` captures spam score buckets as returned by PMG.
- `quarantine` aggregates queue counts for spam and virus categories.

Snippet:

```json
{
  "pmg": [
    {
      "id": "pmg-primary",
      "name": "primary",
      "host": "https://pmg.example.com",
      "status": "online",
      "version": "8.3.1",
      "connectionHealth": "healthy",
      "lastSeen": "2025-10-10T09:30:00Z",
      "lastUpdated": "2025-10-10T09:30:05Z",
      "nodes": [
        { "name": "pmg01", "status": "master", "role": "master" }
      ],
      "mailStats": {
        "timeframe": "day",
        "countTotal": 100,
        "countIn": 60,
        "countOut": 40,
        "spamIn": 5,
        "spamOut": 2,
        "virusIn": 1,
        "virusOut": 0,
        "rblRejects": 2,
        "pregreetRejects": 1,
        "greylistCount": 7,
        "averageProcessTimeMs": 480,
        "updatedAt": "2025-10-10T09:30:05Z"
      },
      "mailCount": [
        {
          "timestamp": "2025-10-10T09:00:00Z",
          "count": 100,
          "countIn": 60,
          "countOut": 40,
          "spamIn": 5,
          "spamOut": 2,
          "virusIn": 1,
          "virusOut": 0,
          "rblRejects": 2,
          "pregreet": 1,
          "greylist": 7,
          "index": 0,
          "timeframe": "hour"
        }
      ],
      "spamDistribution": [
        { "score": "low", "count": 10 }
      ],
      "quarantine": { "spam": 5, "virus": 2 }
    }
  ]
}
```

### Docker Agent Integration
Accept reports from the optional Docker agent to track container workloads outside Proxmox.

```bash
POST /api/agents/docker/report        # Submit agent heartbeat payloads (JSON)
DELETE /api/agents/docker/hosts/<id>  # Remove a Docker host that has gone offline
GET /api/agent/version                # Retrieve the bundled Docker agent version
GET /install-docker-agent.sh          # Download the installation convenience script
GET /download/pulse-docker-agent      # Download the standalone Docker agent binary
```

Agent routes require authentication. Use an API token or an authenticated session when calling them from automation. When authenticating with tokens, grant `docker:report` for `POST /api/agents/docker/report`, `docker:manage` for Docker host lifecycle endpoints, and `host-agent:report` for host agent submissions. The payload reports restart loops, exit codes, memory pressure, and health probes per container, and Pulse de-duplicates heartbeats per agent ID so you can fan out to multiple Pulse instances safely. Host responses mirror the `/api/state` data, including `issues`, `recentExitCodes`, and `lastSeen` timestamps so external tooling can mimic the built-in Docker workspace.

## Monitoring Data

### Charts Data
Get time-series data for charts (CPU, memory, storage).

```bash
GET /api/charts
GET /api/charts?range=1h  # Last hour (default)
GET /api/charts?range=24h # Last 24 hours
GET /api/charts?range=7d  # Last 7 days
```

### Storage Information
Get detailed storage information for all nodes.

```bash
GET /api/storage/
GET /api/storage/<node-id>
```

### Storage Charts
Get storage usage trends over time.

```bash
GET /api/storage-charts
```

### Backup Information
Get backup information across all nodes.

```bash
GET /api/backups          # All backups
GET /api/backups/unified  # Unified view
GET /api/backups/pve      # PVE backups only
GET /api/backups/pbs      # PBS backups only
```

### Snapshots
Get snapshot information for VMs and containers.

```bash
GET /api/snapshots
```

### Guest Metadata
Manage custom metadata for VMs and containers (e.g., console URLs).

```bash
GET /api/guests/metadata              # Get all guest metadata
GET /api/guests/metadata/<guest-id>   # Get metadata for specific guest
PUT /api/guests/metadata/<guest-id>   # Update guest metadata
DELETE /api/guests/metadata/<guest-id> # Remove guest metadata
```

### Network Discovery
Discover Proxmox nodes on your network.

```bash
GET /api/discover     # Get cached discovery results (updates every 5 minutes)
```

Note: Manual subnet scanning via POST is currently not available through the API.

### System Settings
Manage system-wide settings.

```bash
GET /api/system/settings         # Get current system settings (includes env overrides)
POST /api/system/settings/update # Update system settings (admin only)
```

## Configuration

### Node Management
Manage Proxmox VE, Proxmox Mail Gateway, and PBS nodes.

```bash
GET /api/config/nodes                    # List all nodes
POST /api/config/nodes                   # Add new node
PUT /api/config/nodes/<node-id>         # Update node
DELETE /api/config/nodes/<node-id>      # Remove node
POST /api/config/nodes/test-connection  # Test node connection
POST /api/config/nodes/test-config      # Test node configuration (for new nodes)
POST /api/config/nodes/<node-id>/test   # Test existing node
```

#### Add Node Example
```bash
curl -X POST http://localhost:7655/api/config/nodes \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "type": "pve",
    "name": "My PVE Node",
    "host": "https://192.168.1.100:8006",
    "user": "monitor@pve",
    "password": "password",
    "verifySSL": false
  }'
```

### System Configuration
Get and update system configuration.

```bash
GET /api/config/system   # Get system config
PUT /api/config/system   # Update system config
```

### Mock Mode Control
Toggle mock data generation used for demos and development.

```bash
GET /api/system/mock-mode       # Report current mock mode status
POST /api/system/mock-mode      # Enable/disable mock mode (admin only)
PUT /api/system/mock-mode       # Same as POST, but idempotent for tooling
```

These endpoints back the `npm run mock:on|off|status` scripts and trigger the same hot reload behavior. Responses include both `enabled` and the full mock configuration so tooling can preview generated node/guest counts before flipping the switch.

### Security Configuration

#### Security Status
Check current security configuration status.

```bash
GET /api/security/status
```

Returns information about:
- Authentication configuration
- API token status  
- Network context (private/public)
- HTTPS status
- Audit logging status

#### Password Management
Manage user passwords.

```bash
POST /api/security/change-password
```

Request body:
```json
{
  "currentPassword": "old-password",
  "newPassword": "new-secure-password"
}
```

#### Quick Security Setup
Quick setup for authentication (first-time setup).

```bash
POST /api/security/quick-setup
```

Authentication:
- Provide the bootstrap token in the `X-Setup-Token` header (or in the JSON payload) when no auth is configured yet.
- Once credentials exist, an authenticated admin session or API token with `settings:write` is required.

Request body:
```json
{
  "username": "admin",
  "password": "secure-password",
  "apiToken": "raw-token-value",
  "setupToken": "<bootstrap-token>"
}
```

The bootstrap token can be read from `/.bootstrap_token` in the data directory (for example `/etc/pulse/.bootstrap_token` on bare metal or `/data/.bootstrap_token` in Docker). The token file is removed automatically after a successful setup run.

#### API Token Management
Manage API tokens for automation workflows, Docker agents, and tool integrations.

Authentication: Requires an admin session or an API token with the scope(s) below:
- `settings:read` for `GET /api/security/tokens`
- `settings:write` for `POST /api/security/tokens` and `DELETE /api/security/tokens/{id}`

**List tokens**
```bash
GET /api/security/tokens
```

Response:
```json
{
  "tokens": [
    {
      "id": "9bf9aa59-3b85-4fd8-9aad-3f19b2c9b6f0",
      "name": "ansible",
      "prefix": "pulse_1a2b",
      "suffix": "c3d4",
      "createdAt": "2025-10-14T12:12:34Z",
      "lastUsedAt": "2025-10-14T12:21:05Z",
      "scopes": ["docker:report", "monitoring:read"]
    }
  ]
}
```

**Create a token**
```bash
POST /api/security/tokens
Content-Type: application/json
{
  "name": "ansible",
  "scopes": ["monitoring:read"]
}
```

> Omit the `scopes` field to mint a full-access token (`["*"]`). When present, the array must include one or more known scopes—see `docs/CONFIGURATION.md` for the canonical list and descriptions.

Response (token value is returned once):
```json
{
  "token": "pulse_1a2b3c4d5e6f7g8h9i0j",
  "record": {
    "id": "9bf9aa59-3b85-4fd8-9aad-3f19b2c9b6f0",
    "name": "ansible",
    "prefix": "pulse_1a2b",
    "suffix": "c3d4",
    "createdAt": "2025-10-14T12:12:34Z",
    "lastUsedAt": null,
    "scopes": ["monitoring:read"]
  }
}
```

**Delete a token**
```bash
DELETE /api/security/tokens/{id}
```

Returns `204 No Content` when the token is revoked.

> Legacy compatibility: `POST /api/security/regenerate-token` is still available but now replaces the entire token list with a single regenerated token. Prefer the endpoints above for multi-token environments.

#### Login
Enhanced login endpoint with lockout feedback.

```bash
POST /api/login
```

Request body:
```json
{
  "username": "admin",
  "password": "your-password"
}
```

Response includes:
- Remaining attempts after failed login
- Lockout status and duration when locked
- Clear error messages with recovery guidance

#### Logout
End the current session.

```bash
POST /api/logout
```

#### Account Lockout Recovery
Reset account lockouts (requires authentication).

```bash
POST /api/security/reset-lockout
```

Request body:
```json
{
  "identifier": "username-or-ip"  // Can be username or IP address
}
```

This endpoint allows administrators to manually reset lockouts before the 15-minute automatic expiration.

### Export/Import Configuration
Backup and restore Pulse configuration with encryption.

```bash
POST /api/config/export  # Export encrypted config
POST /api/config/import  # Import encrypted config
```

**Authentication**: Requires one of:
- Active session (when logged in with password)
- API token via X-API-Token header  
- Private network access (automatic for homelab users on 192.168.x.x, 10.x.x.x, 172.16.x.x)
- ALLOW_UNPROTECTED_EXPORT=true (to explicitly allow on public networks)

**Export includes**: 
- All nodes and their credentials (encrypted)
- Alert configurations
- Webhook configurations  
- Email settings
- System settings (polling intervals, UI preferences)
- Guest metadata (custom console URLs)

**NOT included** (for security):
- Authentication settings (passwords, API tokens)
- Each instance should have its own authentication

## Notifications

### Email Configuration
Manage email notification settings.

```bash
GET /api/notifications/email          # Get email config
PUT /api/notifications/email          # Update email config (Note: Uses PUT, not POST)
GET /api/notifications/email-providers # List email providers
```

### Test Notifications
Test notification delivery.

```bash
POST /api/notifications/test          # Send test notification to all configured channels
```

### Webhook Configuration
Manage webhook notification endpoints.

```bash
GET /api/notifications/webhooks                    # List all webhooks
POST /api/notifications/webhooks                   # Create new webhook
PUT /api/notifications/webhooks/<id>               # Update webhook
DELETE /api/notifications/webhooks/<id>            # Delete webhook
POST /api/notifications/webhooks/test              # Test webhook
GET /api/notifications/webhook-templates           # Get service templates
GET /api/notifications/webhook-history             # Get webhook notification history
```

#### Create Webhook Example
```bash
curl -X POST http://localhost:7655/api/notifications/webhooks \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "name": "Discord Alert",
    "url": "https://discord.com/api/webhooks/xxx/yyy",
    "method": "POST",
    "service": "discord",
    "enabled": true
  }'
```

#### Custom Payload Template Example
```bash
curl -X POST http://localhost:7655/api/notifications/webhooks \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "name": "Custom Webhook",
    "url": "https://my-service.com/webhook",
    "method": "POST",
    "service": "generic",
    "enabled": true,
    "template": "{\"alert\": \"{{.Level}}: {{.Message}}\", \"value\": {{.Value}}}"
  }'
```

#### Test Webhook
```bash
curl -X POST http://localhost:7655/api/notifications/webhooks/test \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "name": "Test",
    "url": "https://example.com/webhook",
    "service": "generic"
  }'
```


### Alert Management
Comprehensive alert management system.

```bash
# Alert Configuration
GET /api/alerts/                     # Get alert configuration and status
POST /api/alerts/                    # Update alert settings

# Alert Monitoring
GET /api/alerts/active                # Get currently active alerts
GET /api/alerts/history               # Get alert history
DELETE /api/alerts/history            # Clear alert history

# Alert Actions
POST /api/alerts/<id>/acknowledge    # Acknowledge an alert
POST /api/alerts/<id>/clear          # Clear a specific alert
POST /api/alerts/<id>/unacknowledge  # Remove acknowledgement
```

Alert configuration responses model Pulse's hysteresis thresholds and advanced behaviour:

- `guestDefaults`, `nodeDefaults`, `storageDefault`, `dockerDefaults`, `pmgThresholds` expose the baseline trigger/clear values applied globally. Each metric uses `{ "trigger": 90, "clear": 85 }`, so fractional thresholds (e.g. `12.5`) are supported.
- `overrides` is keyed by resource ID for bespoke thresholds. Setting a threshold to `-1` disables that signal for that resource.
- `timeThresholds` and `metricTimeThresholds` provide per-resource/per-metric grace periods, reducing alert noise on bursty workloads.
- `dockerIgnoredContainerPrefixes` suppresses alerts for ephemeral containers whose name or ID begins with a listed prefix. Matching is case-insensitive and controlled through the Alerts UI.
- `aggregation`, `flapping`, `schedule` configure deduplication, cooldown, and quiet hours. These values are shared with the notification pipeline.
- Active and historical alerts include `metadata.clearThreshold`, `resourceType`, and other context so UIs can render the trigger/clear pair and supply timeline explanations.

### Notification Management
Manage notification destinations and history.

```bash
GET /api/notifications/               # Get notification configuration
POST /api/notifications/              # Update notification settings
GET /api/notifications/history        # Get notification history
```

## Auto-Registration

Pulse provides a secure auto-registration system for adding Proxmox nodes using one-time setup codes.

### Generate Setup Code and URL
Generate a one-time setup code and URL for node configuration. This endpoint requires authentication.

```bash
POST /api/setup-script-url
```

Request:
```json
{
  "type": "pve",        // "pve", "pmg", or "pbs"
  "host": "https://192.168.1.100:8006",
  "backupPerms": true   // Optional: add backup management permissions (PVE only)
}
```

Response:
```json
{
  "url": "http://pulse.local:7655/api/setup-script?type=pve&host=...",
  "command": "curl -sSL \"http://pulse.local:7655/api/setup-script?...\" | bash",
  "setupToken": "4c7f3e8c1c5f4b0da580c4477f4b1c2d",
  "tokenHint": "4c7…c2d",
  "expires": 1755123456    // Unix timestamp when token expires (5 minutes)
}
```

### Setup Script
Download the setup script for automatic node configuration. This endpoint is public but the script will prompt for a setup code.

```bash
GET /api/setup-script?type=pve&host=<encoded-url>&pulse_url=<encoded-url>
```

The script will:
1. Create a monitoring user (pulse-monitor@pam or pulse-monitor@pbs)
2. Generate an API token for that user
3. Set appropriate permissions
4. Prompt for the setup token (or read `PULSE_SETUP_TOKEN` if set)
5. Auto-register with Pulse if a valid token is provided

### Auto-Register Node
Register a node automatically (used by setup scripts). Requires either a valid setup code or API token.

```bash
POST /api/auto-register
```

Request with setup code (preferred):
```json
{
  "type": "pve",
  "host": "https://node.local:8006",
  "serverName": "node-hostname",
  "tokenId": "pulse-monitor@pam!token-name",
  "tokenValue": "token-secret-value",
  "setupCode": "A7K9P2"  // One-time setup code from UI
}
```

Request with API token (legacy):
```bash
curl -X POST http://localhost:7655/api/auto-register \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-api-token" \
  -d '{
    "type": "pve",
    "host": "https://node.local:8006",
    "serverName": "node-hostname",
    "tokenId": "pulse-monitor@pam!token-name",
    "tokenValue": "token-secret-value"
  }'
```

### Security Management
Additional security endpoints.

```bash
# Apply security settings and restart service
POST /api/security/apply-restart

# Recovery mode (localhost only)
GET /api/security/recovery           # Check recovery status
POST /api/security/recovery          # Enable/disable recovery mode
  Body: {"action": "disable_auth" | "enable_auth"}
```

### Security Features

The setup code system provides multiple layers of security:

- **One-time use**: Each code can only be used once
- **Time-limited**: Codes expire after 5 minutes
- **Hashed storage**: Codes are stored as SHA3-256 hashes
- **Validation**: Codes are validated against node type and host URL
- **No secrets in URLs**: Setup URLs contain no authentication tokens
- **Interactive entry**: Codes are entered interactively, not passed in URLs

### Alternative: Environment Variable

For automation, the setup code can be provided via environment variable:

```bash
PULSE_SETUP_CODE=A7K9P2 curl -sSL "http://pulse:7655/api/setup-script?..." | bash
```


## Guest Metadata

Manage custom metadata for VMs and containers, such as console URLs.

```bash
# Get all guest metadata
GET /api/guests/metadata

# Get metadata for specific guest
GET /api/guests/metadata/<node>/<vmid>

# Update guest metadata
PUT /api/guests/metadata/<node>/<vmid>
POST /api/guests/metadata/<node>/<vmid>

# Delete guest metadata
DELETE /api/guests/metadata/<node>/<vmid>
```

Example metadata update:
```bash
curl -X PUT http://localhost:7655/api/guests/metadata/pve-node/100 \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "consoleUrl": "https://custom-console.example.com/vm/100",
    "notes": "Production database server"
  }'
```

## System Information

### Current Configuration
Get the current Pulse configuration.

```bash
GET /api/config
```

Returns the complete configuration including nodes, settings, and system parameters.

### Diagnostics
Get comprehensive system diagnostics information.

```bash
GET /api/diagnostics
```

Returns detailed information about:
- System configuration
- Node connectivity status
- Error logs
- Performance metrics
- Service health

> **Caching (v4.24.0+):** Diagnostics results are cached for 45 seconds to protect upstream systems. If the cache is fresh it is returned immediately; otherwise a new probe runs, replacing the cache once complete. Inspect the `X-Diagnostics-Cached-At` header to see when the payload was generated. Probe failures surface in the `errors` array and are tracked by Prometheus metrics (`pulse_diagnostics_*`).

### Network Discovery
Discover Proxmox servers on the network.

```bash
GET /api/discover
```

Response:
```json
{
  "servers": [
    {
      "host": "192.168.1.100",
      "port": 8006,
      "type": "pve",
      "name": "pve-node-1"
    }
  ],
  "errors": [],
  "scanning": false,
  "updated": 1755123456
}
```

### Simple Statistics
Get simplified statistics (lightweight endpoint).

```bash
GET /simple-stats
```

## Session Management

### Logout
End the current user session.

```bash
POST /api/logout
```

## Settings Management

### UI Settings
Manage user interface preferences.

```bash
# Get current UI settings
GET /api/settings

# Update UI settings
POST /api/settings/update
```

Settings include:
- Theme preferences
- Dashboard layout
- Refresh intervals
- Display options

### System Settings
Manage system-wide settings.

```bash
# Get system settings
GET /api/system/settings

# Update system settings
POST /api/system/settings/update
```

System settings include:
- Polling intervals
- Performance tuning
- Feature flags
- Global configurations

## Updates

### Check for Updates
Check if a new version is available. Returns version info, release notes, and deployment-specific instructions.

```bash
GET /api/updates/check
GET /api/updates/check?channel=rc   # Override channel (stable/rc)
```

The response includes `deploymentType` so the UI/automation can decide whether a self-service update is possible (`systemd`, `proxmoxve`, `aur`) or if a manual Docker image pull is required.

### Prepare Update Plan
Fetch scripted steps for a target version. Useful when presenting the release picker in the UI.

```bash
GET /api/updates/plan?version=v4.30.0
GET /api/updates/plan?version=v4.30.0&channel=rc
```

Response example (systemd deployment, v4.24.0+):

```json
{
  "version": "v4.30.0",
  "channel": "stable",
  "canAutoUpdate": true,
  "requiresRoot": true,
  "rollbackSupport": true,
  "estimatedTime": "2-3 minutes",
  "downloadUrl": "https://github.com/rcourtman/Pulse/releases/download/v4.30.0/pulse-linux-amd64.tar.gz",
  "instructions": "Run the installer script with --version flag",
  "prerequisites": ["systemd", "root access"],
  "steps": [
    "curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.30.0"
  ]
}
```

### Apply Update
Kick off an update using the download URL returned by the release metadata. Pulse runs the install script asynchronously and streams progress via WebSocket.

```bash
POST /api/updates/apply
Content-Type: application/json

{ "downloadUrl": "https://github.com/rcourtman/Pulse/releases/download/v4.30.0/pulse-linux-amd64.tar.gz" }
```

Only deployments that can self-update (systemd, Proxmox VE appliance, AUR) will honour this call. Docker users should continue to pull a new image manually.

### Update Status
Retrieve the last known update status or in-flight progress. Possible values: `idle`, `checking`, `downloading`, `installing`, `completed`, `error`.

```bash
GET /api/updates/status
```

### Update History
Pulse captures each self-update attempt in a local history file.

```bash
GET /api/updates/history                 # List recent update attempts (optional ?limit=&status=)
GET /api/updates/history/entry?id=<uuid> # Inspect a specific update event
```

**Response format (v4.24.0+):**
```json
{
  "entries": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "action": "update",
      "version": "v4.24.0",
      "fromVersion": "v4.23.0",
      "channel": "stable",
      "status": "completed",
      "timestamp": "2025-10-20T10:30:00Z",
      "initiated_via": "ui",
      "related_event_id": null,
      "backup_path": "/opt/pulse/backups/pre-update-v4.23.0.tar.gz",
      "duration_seconds": 120,
      "error": null
    },
    {
      "id": "650e8400-e29b-41d4-a716-446655440001",
      "action": "rollback",
      "version": "v4.23.0",
      "fromVersion": "v4.24.0",
      "channel": "stable",
      "status": "completed",
      "timestamp": "2025-10-20T11:00:00Z",
      "initiated_via": "api",
      "related_event_id": "550e8400-e29b-41d4-a716-446655440000",
      "backup_path": null,
      "duration_seconds": 45,
      "error": null
    }
  ]
}
```

Entries include:
- `action`: "update" | "rollback"
- `status`: "pending" | "in_progress" | "completed" | "failed"
- `initiated_via`: How the action was started (ui, api, auto)
- `related_event_id`: Links rollback to original update
- `backup_path`: Location of pre-update backup
- Error details for failed attempts

## Real-time Updates

### WebSocket
Real-time updates are available via WebSocket connection.

```javascript
const ws = new WebSocket('ws://localhost:7655/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Update received:', data);
};
```

The WebSocket broadcasts state updates every few seconds with the complete system state.

### Socket.IO Compatibility
For Socket.IO clients, a compatibility endpoint is available:

```bash
GET /socket.io/
```

### Test Notifications
Test WebSocket notifications:

```bash
POST /api/test-notification
```

## Simple Statistics

Lightweight statistics endpoint for monitoring.

```bash
GET /simple-stats
```

Returns simplified metrics without authentication requirements.

## Rate Limiting

**v4.24.0:** All responses include rate limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`). 429 responses add `Retry-After`.

**Rate limits by endpoint category:**
- **Authentication**: 10 attempts/minute per IP
- **Config writes**: 30 requests/minute
- **Exports**: 5 requests per 5 minutes
- **Recovery operations**: 3 requests per 10 minutes
- **Update operations**: 20 requests/minute
- **WebSocket connections**: 5 connections/minute per IP
- **General API**: 500 requests/minute per IP
- **Public endpoints**: 1000 requests/minute per IP

**Exempt endpoints** (no rate limits):
- `/api/state` (real-time monitoring)
- `/api/guests/metadata` (frequent polling)
- WebSocket message streaming (after connection established)

**Example response with rate limit headers:**
```
HTTP/1.1 200 OK
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 487
X-RateLimit-Reset: 1754995800
Content-Type: application/json
```

**When rate limited:**
```
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1754995800
Retry-After: 60
Content-Type: application/json

{
  "error": "Rate limit exceeded. Please retry after 60 seconds."
}
```

## Error Responses

All endpoints return standard HTTP status codes:
- `200 OK` - Success
- `400 Bad Request` - Invalid request data
- `401 Unauthorized` - Missing or invalid API token
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limited
- `500 Internal Server Error` - Server error

Error response format:
```json
{
  "error": "Error message description"
}
```

## Examples

### Full Example: Monitor a New Node

```bash
# 1. Test connection to node
curl -X POST http://localhost:7655/api/config/nodes/test-connection \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "type": "pve",
    "host": "https://192.168.1.100:8006",
    "user": "root@pam",
    "password": "password"
  }'

# 2. Add the node if test succeeds
curl -X POST http://localhost:7655/api/config/nodes \
  -H "Content-Type: application/json" \
  -H "X-API-Token: your-token" \
  -d '{
    "type": "pve",
    "name": "pve-node-1",
    "host": "https://192.168.1.100:8006",
    "user": "root@pam",
    "password": "password",
    "verifySSL": false
  }'

# 3. Get monitoring data
curl -H "X-API-Token: your-token" http://localhost:7655/api/state

# 4. Get chart data
curl -H "X-API-Token: your-token" http://localhost:7655/api/charts?range=1h
```

### PowerShell Example

```powershell
# Set variables
$apiUrl = "http://localhost:7655/api"
$apiToken = "your-secure-token"
$headers = @{ "X-API-Token" = $apiToken }

# Check health
$health = Invoke-RestMethod -Uri "$apiUrl/health" -Headers $headers
Write-Host "Status: $($health.status)"

# Get all nodes
$nodes = Invoke-RestMethod -Uri "$apiUrl/config/nodes" -Headers $headers
$nodes | ForEach-Object { Write-Host "Node: $($_.name) - $($_.status)" }
```

### Python Example

```python
import requests

API_URL = "http://localhost:7655/api"
API_TOKEN = "your-secure-token"
headers = {"X-API-Token": API_TOKEN}

# Check health
response = requests.get(f"{API_URL}/health", headers=headers)
health = response.json()
print(f"Status: {health['status']}")

# Get monitoring data
response = requests.get(f"{API_URL}/state", headers=headers)
state = response.json()
for node in state.get("nodes", []):
    print(f"Node: {node['name']} - {node['status']}")
```
