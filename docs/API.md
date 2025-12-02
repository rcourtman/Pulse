# 🔌 Pulse API Reference

Pulse provides a comprehensive REST API for automation and integration.

**Base URL**: `http://<your-pulse-ip>:7655/api`

## 🔐 Authentication

All API requests require authentication via one of the following methods:

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

---

## 📊 Metrics & Charts

### Chart Data
`GET /api/charts?range=1h`
Returns time-series data for CPU, Memory, and Storage.
**Ranges**: `1h`, `24h`, `7d`, `30d`

### Storage Stats
`GET /api/storage/`
Detailed storage usage per node and pool.

### Backup History
`GET /api/backups/unified`
Combined view of PVE and PBS backups.

---

## 🔔 Notifications

### Send Test Notification
`POST /api/notifications/test`
Triggers a test alert to all configured channels.

### Manage Webhooks
- `GET /api/notifications/webhooks`
- `POST /api/notifications/webhooks`
- `DELETE /api/notifications/webhooks/<id>`

---

## 🛡️ Security

### List API Tokens
`GET /api/security/tokens`

### Create API Token
`POST /api/security/tokens`
```json
{ "name": "ansible-script", "scopes": ["monitoring:read"] }
```

### Revoke Token
`DELETE /api/security/tokens/<id>`

---

---
## 🖥️ Host Agent

### Submit Report
`POST /api/agents/host/report`
Used by the Pulse Host Agent to push system metrics.

### Lookup Agent
`POST /api/agents/host/lookup`
Check if a host agent is already registered.

### Delete Host
`DELETE /api/agents/host/<id>`
Remove a host agent from monitoring.

---

## ⚙️ System Settings

### Get Settings
`GET /api/config/system`
Retrieve current system configuration.

### Toggle Mock Mode
`POST /api/system/mock-mode`
Enable or disable mock data generation (dev/demo only).

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

## 🐳 Docker Agent

### Submit Report
`POST /api/agents/docker/report`
Used by the Pulse Docker Agent to push container metrics.

### Download Agent
`GET /download/pulse-docker-agent`
Downloads the binary for the current platform.

---

> **Note**: This is a summary of the most common endpoints. For a complete list, inspect the network traffic of the Pulse dashboard or check the source code in `internal/api/router.go`.
