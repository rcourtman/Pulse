# Pulse API Documentation

## Overview

Pulse provides a REST API for monitoring and managing Proxmox VE and PBS instances. All API endpoints are prefixed with `/api`.

## Authentication

API authentication is optional but recommended for production use.

### Setting up API Authentication

```bash
# Systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=your-secure-token-here"

# Docker
docker run -e API_TOKEN=your-secure-token rcourtman/pulse:latest
```

### Using API Authentication

Include the token in the `X-API-Token` header:

```bash
curl -H "X-API-Token: your-secure-token" http://localhost:7655/api/health
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
  "version": "v4.2.1",
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

## Configuration

### Node Management
Manage Proxmox VE and PBS nodes.

```bash
GET /api/config/nodes                    # List all nodes
POST /api/config/nodes                   # Add new node
PUT /api/config/nodes/<node-id>         # Update node
DELETE /api/config/nodes/<node-id>      # Remove node
POST /api/config/nodes/test-connection  # Test node connection
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
POST /api/config/system  # Update system config
```

### Export/Import Configuration
Backup and restore Pulse configuration.

```bash
POST /api/config/export  # Export encrypted config
POST /api/config/import  # Import encrypted config
```

**Note**: Requires API_TOKEN or ALLOW_UNPROTECTED_EXPORT=true

## Notifications

### Email Configuration
Manage email notification settings.

```bash
GET /api/notifications/email          # Get email config
POST /api/notifications/email         # Update email config
POST /api/notifications/email/test    # Send test email
GET /api/notifications/email-providers # List email providers
```

### Alert Thresholds
Manage alert thresholds and overrides.

```bash
GET /api/notifications/thresholds            # Get thresholds
POST /api/notifications/thresholds           # Update thresholds
GET /api/notifications/thresholds/overrides  # Get overrides
POST /api/notifications/thresholds/overrides # Set overrides
```

### Alert History
View alert history and manage alerts.

```bash
GET /api/notifications/alerts         # Get recent alerts
POST /api/notifications/alerts/clear  # Clear alert history
```

## Auto-Registration

### Setup Script
Generate setup script for automatic node configuration.

```bash
POST /api/setup-script
```

Request:
```json
{
  "type": "pve",
  "host": "https://192.168.1.100:8006"
}
```

### Auto-Register Node
Register a node automatically (used by setup scripts).

```bash
POST /api/auto-register
```

With registration token (if required):
```bash
curl -X POST http://localhost:7655/api/auto-register \
  -H "X-Registration-Token: PULSE-REG-xxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "pve",
    "host": "https://node.local:8006",
    "name": "Node Name",
    "username": "monitor@pam",
    "tokenId": "token-id",
    "tokenValue": "token-secret"
  }'
```

## Registration Tokens

Manage registration tokens for secure node registration.

```bash
POST /api/tokens/generate  # Generate new token
GET /api/tokens/list       # List active tokens
DELETE /api/tokens/revoke  # Revoke a token
```

### Generate Token Example
```bash
curl -X POST http://localhost:7655/api/tokens/generate \
  -H "X-API-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "validityMinutes": 30,
    "maxUses": 5,
    "allowedTypes": ["pve", "pbs"],
    "description": "Production cluster setup"
  }'
```

## Updates

### Check for Updates
Check if a new version is available.

```bash
GET /api/updates/check
```

### Apply Update
Download and apply an available update.

```bash
POST /api/updates/apply
```

### Update Status
Get current update operation status.

```bash
GET /api/updates/status
```

## WebSocket

Real-time updates are available via WebSocket connection.

```javascript
const ws = new WebSocket('ws://localhost:7655/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Update received:', data);
};
```

The WebSocket broadcasts state updates every few seconds with the complete system state.

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