# Pulse API Documentation

This document describes the REST API endpoints available in Pulse, including authentication requirements and CSRF protection details.

## Authentication

Pulse supports two authentication methods depending on the security mode:

### Public Mode
- No authentication required
- All endpoints are accessible without credentials
- CSRF protection is disabled

### Private Mode (Default)
- Authentication required for all endpoints except `/api/health`
- Two authentication methods supported:
  1. **Session-based**: Login via web UI, uses cookies
  2. **HTTP Basic Auth**: For API integrations

#### Session-based Authentication

```bash
# Login
curl -X POST http://pulse:7655/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}'

# Response includes session cookie and CSRF token
{
  "success": true,
  "user": { "username": "admin" },
  "csrfToken": "..."
}
```

#### HTTP Basic Auth

```bash
# Example API request with Basic Auth
curl -u admin:your-password http://pulse:7655/api/health
```

## CSRF Protection

When using session-based authentication in Private mode:

1. **CSRF tokens are required** for all state-changing requests (POST, PUT, DELETE)
2. **Token delivery**: Include the CSRF token in the `X-CSRF-Token` header
3. **Token retrieval**: Get the token from:
   - Login response (`csrfToken` field)
   - Any authenticated API response (`X-CSRF-Token` response header)
   - Settings endpoint

### Example with CSRF Token

```bash
# Get CSRF token from login
TOKEN=$(curl -X POST http://pulse:7655/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}' \
  -c cookies.txt | jq -r .csrfToken)

# Use token in subsequent requests
curl -X POST http://pulse:7655/api/alerts/test \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $TOKEN" \
  -b cookies.txt \
  -d '{"type": "webhook"}'
```

### CSRF-Exempt Endpoints

The following endpoints do not require CSRF tokens:
- `/api/auth/login` - Login endpoint
- `/api/health` - Health check
- `/api/config/test` - Connection testing (during setup)
- All GET requests

## API Endpoints

### Authentication

#### POST /api/auth/login
Login to create a session.

**Request:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response:**
```json
{
  "success": true,
  "user": { "username": "admin" },
  "csrfToken": "..."
}
```

#### POST /api/auth/logout
Logout and destroy session. Requires CSRF token.

#### GET /api/auth/status
Check authentication status.

### Health & Status

#### GET /api/health
Health check endpoint. No authentication required.

**Response:**
```json
{
  "status": "healthy",
  "uptime": 12345,
  "version": "3.42.0",
  "mode": "private"
}
```

### Configuration

#### GET /api/config
Get current configuration (sanitized, no secrets).

#### POST /api/config
Update configuration. Requires CSRF token.

**Request:**
```json
{
  "proxmox": {
    "host": "https://pve.example.com:8006",
    "tokenId": "user@pam!token",
    "tokenSecret": "..."
  }
}
```

#### POST /api/config/test
Test connection settings without saving.

### Alerts

#### GET /api/alerts
Get all alert rules and active alerts.

#### POST /api/alerts
Update alert configuration. Requires CSRF token.

#### POST /api/alerts/test
Test notification configuration. Requires CSRF token.

#### DELETE /api/alerts/:alertId
Clear a specific alert. Requires CSRF token.

### Custom Thresholds

#### GET /api/thresholds
Get all custom VM/LXC thresholds.

#### POST /api/thresholds
Create or update a custom threshold. Requires CSRF token.

#### DELETE /api/thresholds/:vmid
Delete a custom threshold. Requires CSRF token.

### Metrics & Snapshots

#### GET /api/snapshots
Get historical metrics snapshots.

**Query Parameters:**
- `hours`: Number of hours of history (default: 24, max: 168)

#### POST /api/snapshots/export
Export metrics data. Requires CSRF token.

### Updates

#### GET /api/updates/check
Check for available updates.

#### POST /api/updates/install
Install an update (non-Docker only). Requires CSRF token.

### PBS Push Mode

#### POST /api/push/:serverId
Receive metrics from a PBS server in push mode.

**Headers:**
- `X-API-Key`: Required, must match `PULSE_PUSH_API_KEY`

**Request:**
```json
{
  "timestamp": "2024-01-20T10:00:00Z",
  "version": "1.0",
  "server": { ... },
  "datastores": [ ... ],
  "tasks": [ ... ]
}
```

## Error Responses

All endpoints use consistent error responses:

```json
{
  "error": "Error type",
  "message": "Human-readable error message"
}
```

Common HTTP status codes:
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Invalid CSRF token or insufficient permissions
- `404 Not Found` - Resource not found
- `422 Unprocessable Entity` - Validation error
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## Rate Limiting

API endpoints are rate-limited to prevent abuse:

- **Default limit**: 100 requests per minute
- **Strict endpoints** (login): 5 requests per minute
- **API endpoints**: 200 requests per minute

Rate limit headers are included in responses:
- `X-RateLimit-Limit`: Request limit
- `X-RateLimit-Remaining`: Remaining requests
- `X-RateLimit-Reset`: Reset timestamp

## WebSocket API

Real-time updates are available via WebSocket at `/socket.io/`.

### Events

#### Client → Server
- `subscribe`: Subscribe to updates
- `unsubscribe`: Unsubscribe from updates

#### Server → Client
- `metricsUpdate`: Real-time metrics data
- `alertsUpdate`: Alert status changes
- `configUpdate`: Configuration changes

### Example

```javascript
const socket = io('http://pulse:7655', {
  auth: {
    token: 'session-token'
  }
});

socket.on('connect', () => {
  socket.emit('subscribe');
});

socket.on('metricsUpdate', (data) => {
  console.log('Metrics updated:', data);
});
```

## Security Headers

All API responses include security headers:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` (unless iframe embedding is enabled)
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy: default-src 'self'; ...`

## API Client Example

### Python

```python
import requests

# Configure session
session = requests.Session()
base_url = 'http://pulse:7655'

# Login
response = session.post(f'{base_url}/api/auth/login', json={
    'username': 'admin',
    'password': 'your-password'
})
csrf_token = response.json()['csrfToken']

# Make authenticated request with CSRF token
headers = {'X-CSRF-Token': csrf_token}
response = session.post(f'{base_url}/api/alerts/test', 
                       headers=headers,
                       json={'type': 'webhook'})
```

### JavaScript

```javascript
// Login and get CSRF token
const loginResponse = await fetch('/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  credentials: 'include',
  body: JSON.stringify({
    username: 'admin',
    password: 'your-password'
  })
});

const { csrfToken } = await loginResponse.json();

// Make authenticated request
const response = await fetch('/api/config', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-CSRF-Token': csrfToken
  },
  credentials: 'include',
  body: JSON.stringify({ ... })
});
```

## API Versioning

The API currently does not use versioning. Breaking changes are avoided, and new features are added in a backward-compatible manner.

## Support

For API-related issues:
- Check the response error messages
- Review the browser console for client-side errors
- Check Pulse logs for server-side errors
- Open an issue on [GitHub](https://github.com/rcourtman/Pulse/issues)