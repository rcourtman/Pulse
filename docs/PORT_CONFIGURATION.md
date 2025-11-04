# Port Configuration Guide

Pulse supports multiple ways to configure the frontend port (default: 7655).

> **Development tip:** The hot-reload workflow (`scripts/hot-dev.sh` or `make dev-hot`) loads `.env`, `.env.local`, and `.env.dev`. Set `FRONTEND_PORT` or `PULSE_DEV_API_PORT` there to run the backend on a different port while keeping the generated `curl` commands and Vite proxy in sync.

## Recommended Methods

### 1. During Installation (Easiest)
The installer prompts for the port. To skip the prompt, use:
```bash
FRONTEND_PORT=8080 curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

### 2. Using systemd override (For existing installations)
```bash
sudo systemctl edit pulse
```
Add these lines:
```ini
[Service]
Environment="FRONTEND_PORT=8080"
```
Then restart: `sudo systemctl restart pulse`

### 3. Using system.json (Alternative method)
Edit `/etc/pulse/system.json`:
```json
{
  "frontendPort": 8080
}
```
Then restart: `sudo systemctl restart pulse`

### 4. Using environment variables (Docker)
For Docker deployments:
```bash
docker run -e FRONTEND_PORT=8080 -p 8080:8080 rcourtman/pulse:latest
```

## Priority Order

Pulse checks for port configuration in this order:
1. `FRONTEND_PORT` environment variable
2. `PORT` environment variable (legacy)
3. `frontendPort` in system.json
4. Default: 7655

Environment variables always override configuration files.

## Why not .env?

The `/etc/pulse/.env` file is reserved exclusively for authentication credentials:
- `API_TOKENS` - One or more API authentication tokens (hashed)
- `API_TOKEN` - Legacy single API token (hashed)
- `PULSE_AUTH_USER` - Web UI username
- `PULSE_AUTH_PASS` - Web UI password (hashed)

Keeping application configuration separate from authentication credentials:
- Makes it clear what's a secret vs what's configuration
- Allows different permission models if needed
- Follows the principle of separation of concerns
- Makes it easier to backup/share configs without exposing credentials

## Service Name Variations

**Important:** Pulse uses different service names depending on the deployment environment:

- **Systemd (default):** `pulse.service` or `pulse-backend.service` (legacy)
- **Hot-dev scripts:** `pulse-hot-dev` (development only)
- **Kubernetes/Helm:** Deployment `pulse`, Service `pulse` (port configured via Helm values)

**To check the active service:**
```bash
# Systemd
systemctl list-units | grep pulse
systemctl status pulse

# Kubernetes
kubectl -n pulse get svc pulse
kubectl -n pulse get deploy pulse
```

## Change Tracking (v4.24.0+)

Port changes via environment variables or `system.json` take effect immediately after restart. **v4.24.0 records configuration changes in update history**—useful for audit trails and troubleshooting.

**To view change history:**
```bash
# Via UI
# Navigate to Settings → System → Updates

# Via API
curl -s http://localhost:7655/api/updates/history | jq '.entries[] | {timestamp, action, status}'
```

## Troubleshooting

### Port not changing after configuration?
1. **Check which service name is in use:**
   ```bash
   systemctl list-units | grep pulse
   ```
   It might be `pulse` (default), `pulse-backend` (legacy), or `pulse-hot-dev` (dev environment) depending on your installation method.

2. **Verify the configuration is loaded:**
   ```bash
   # Systemd
   sudo systemctl show pulse | grep Environment

   # Kubernetes
   kubectl -n pulse get deploy pulse -o jsonpath='{.spec.template.spec.containers[0].env}' | jq
   ```

3. **Check if another process is using the port:**
   ```bash
   sudo lsof -i :8080
   ```

4. **Verify post-restart** (v4.24.0+):
   ```bash
   # Check actual listening port
   curl -s http://localhost:7655/api/version | jq

   # Check update history for restart event
   curl -s http://localhost:7655/api/updates/history?limit=5 | jq
   ```
