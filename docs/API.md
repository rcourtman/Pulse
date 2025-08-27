# Pulse API Documentation

## Overview

Pulse provides a REST API for monitoring and managing Proxmox VE and PBS instances. All API endpoints are prefixed with `/api`.

## Authentication

Pulse supports multiple authentication methods that can be used independently or together:

### Password Authentication
Set a username and password for web UI access. Passwords are hashed with bcrypt (cost 12) for security.

```bash
# Systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="PULSE_AUTH_USER=admin"
Environment="PULSE_AUTH_PASS=your-secure-password"

# Docker
docker run -e PULSE_AUTH_USER=admin -e PULSE_AUTH_PASS=your-password rcourtman/pulse:latest
```

Once set, users must login via the web UI. The password can be changed from Settings → Security.

### API Token Authentication
For programmatic API access and automation. Tokens can be generated via the web UI (Settings → Security → Generate API Token).

**API-Only Mode**: If only API_TOKEN is configured (no password auth), the UI remains accessible in read-only mode while API modifications require the token.

```bash
# Systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=your-48-char-hex-token"

# Docker
docker run -e API_TOKEN=your-48-char-hex-token rcourtman/pulse:latest
```

### Using Authentication

```bash
# With API Token (header)
curl -H "X-API-Token: your-secure-token" http://localhost:7655/api/health

# With API Token (query parameter, for export/import)
curl "http://localhost:7655/api/export?token=your-secure-token"

# With session cookie (after login)
curl -b cookies.txt http://localhost:7655/api/health
```

### Security Features

When authentication is enabled, Pulse provides enterprise-grade security:

- **CSRF Protection**: All state-changing requests require a CSRF token
- **Rate Limiting**: 500 req/min general, 10 attempts/min for authentication
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

### Version Information
Get current Pulse version and build info.

```bash
GET /api/version
```

Response:
```json
{
  "version": "v4.8.0",
  "build": "release",
  "runtime": "go",
  "channel": "stable",
  "isDocker": false,
  "isDevelopment": false
}
```

### System State
Get complete system state including all nodes and their metrics.

```bash
GET /api/state
```

Response includes all monitored nodes, VMs, containers, storage, and backups.

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
GET /api/system/settings         # Get current system settings
POST /api/system/settings/update  # Update system settings
```

## Configuration

### Node Management
Manage Proxmox VE and PBS nodes.

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

Request body:
```json
{
  "username": "admin",
  "password": "secure-password",
  "generateApiToken": true
}
```

#### API Token Management
Manage API tokens for programmatic access.

```bash
POST /api/security/regenerate-token   # Generate or regenerate API token
```

Note: The old `/api/system/api-token` endpoints have been deprecated in favor of the simplified regenerate-token endpoint.

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
```

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
  "type": "pve",        // "pve" or "pbs"
  "host": "https://192.168.1.100:8006",
  "backupPerms": true   // Optional: add backup management permissions (PVE only)
}
```

Response:
```json
{
  "url": "http://pulse.local:7655/api/setup-script?type=pve&host=...",
  "command": "curl -sSL \"http://pulse.local:7655/api/setup-script?...\" | bash",
  "setupCode": "A7K9P2",  // 6-character one-time code
  "expires": 1755123456    // Unix timestamp when code expires (5 minutes)
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
4. Prompt for the setup code
5. Auto-register with Pulse if a valid code is provided

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
Check if a new version is available. Returns version info and deployment-specific update instructions.

```bash
GET /api/updates/check
```

Response includes `deploymentType` field indicating how to update:
- `proxmoxve`: Type `update` in LXC console
- `docker`: Pull new image and recreate container  
- `systemd`: Re-run install script
- `manual`: Re-run install script

### Apply Update (Deprecated)
**⚠️ DEPRECATED**: This endpoint exists for backwards compatibility but is no longer used.
Updates cannot be performed through the API due to security constraints (no sudo access, 
containers can't restart themselves). Use deployment-specific update methods instead.

```bash
POST /api/updates/apply
```

### Update Status (Deprecated)
**⚠️ DEPRECATED**: Since updates are no longer performed through the API, this endpoint
is not used by the UI.

```bash
GET /api/updates/status
```

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

Some endpoints have rate limiting:
- Export/Import: 5 requests per minute
- Test email: 10 requests per minute
- Update check: 10 requests per hour

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