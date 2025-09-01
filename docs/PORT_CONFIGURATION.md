# Port Configuration Guide

Pulse supports multiple ways to configure the frontend port (default: 7655).

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
- `API_TOKEN` - API authentication token (hashed)
- `PULSE_AUTH_USER` - Web UI username
- `PULSE_AUTH_PASS` - Web UI password (hashed)

Keeping application configuration separate from authentication credentials:
- Makes it clear what's a secret vs what's configuration
- Allows different permission models if needed
- Follows the principle of separation of concerns
- Makes it easier to backup/share configs without exposing credentials

## Troubleshooting

### Port not changing after configuration?
1. Check which service name is in use:
   ```bash
   systemctl list-units | grep pulse
   ```
   It might be `pulse` or `pulse-backend` depending on your installation method.

2. Verify the configuration is loaded:
   ```bash
   sudo systemctl show pulse | grep Environment
   ```

3. Check if another process is using the port:
   ```bash
   sudo lsof -i :8080
   ```

