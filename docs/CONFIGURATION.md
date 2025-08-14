# Pulse Configuration Guide

## Configuration Methods by Deployment Type

### Docker Deployments

**Configuration location:** `/data` (volume mount)
- All settings stored in the mounted volume
- Environment variables passed with `-e` flag or via `/data/.env` file
- The security wizard creates `/data/.env` for auth credentials
- Configuration persists in the volume across container restarts

**Setting environment variables:**
```bash
# Direct run
docker run -d \
  -e FRONTEND_PORT=8080 \
  -e UPDATE_CHANNEL=rc \
  -e API_TOKEN=your-secure-token \
  -v pulse_data:/data \
  rcourtman/pulse:latest

# Or use docker-compose.yml (see README)
```

### LXC/Systemd Deployments (Native Install)

**Configuration location:** `/etc/pulse`
- Settings stored in encrypted JSON files
- Environment variables can be set via systemd or .env file
- .env file at `/etc/pulse/.env` is auto-loaded if present

**Setting environment variables - Option 1: Systemd override**
```bash
# Edit service
sudo systemctl edit pulse-backend

# Add overrides:
[Service]
Environment="FRONTEND_PORT=8080"
Environment="UPDATE_CHANNEL=rc"
```

**Setting environment variables - Option 2: .env file**
```bash
# Create/edit .env file
sudo nano /etc/pulse/.env

# Add variables:
FRONTEND_PORT=8080
UPDATE_CHANNEL=rc

# Restart service
sudo systemctl restart pulse-backend
```

### Web UI Configuration (Both Deployments)
Most settings are configured through the web interface at `http://<server>:7655/settings`:

- **Nodes**: Auto-discovery, one-click setup scripts, cluster detection
- **Alerts**: Thresholds and notification rules  
- **Updates**: Update channels and auto-update settings
- **Security**: Export/import encrypted configurations

## Understanding .env vs .enc Files

Pulse uses two different file types for configuration, each serving a specific purpose:

### .env Files (Authentication Only)
- **Purpose**: Store authentication credentials (username, password, API token)
- **Format**: Plain text environment variables with hashed values
- **Location**: `/data/.env` (Docker) or `/etc/pulse/.env` (native)
- **When used**: Loaded at startup before encryption keys are available
- **Contents**: Only auth-related variables (PULSE_AUTH_USER, PULSE_AUTH_PASS, API_TOKEN)
- **Security**: Passwords and tokens are bcrypt-hashed, not plaintext

**Example .env file:**
```bash
PULSE_AUTH_USER='admin'
PULSE_AUTH_PASS='$2a$12$YTZXOCEylj4TaevZ0DCeI.notayQZ..b0OZ97lUZ.Q24fljLiMQHK'
API_TOKEN='e6e9fcfb4662d2b485000cc5faf2f7e5d8b75e0492b4877c36dadb085f12e57b'
```

**CRITICAL**: The bcrypt hash MUST be exactly 60 characters and enclosed in single quotes!

### .enc Files (Sensitive Configuration)
- **Purpose**: Store sensitive configuration like Proxmox node credentials
- **Format**: Encrypted JSON using AES-256-GCM
- **Location**: `/data/*.enc` (Docker) or `/etc/pulse/*.enc` (native)
- **When used**: After authentication, requires encryption key
- **Contents**: Node credentials (tokens, passwords), email settings, webhooks
- **Security**: Fully encrypted at rest, decrypted only in memory

### Why Both?
This split architecture exists because:
1. **Authentication must work before encryption** - You need to authenticate to access the encryption key
2. **Docker persistence** - Containers need auth to persist across restarts
3. **Security layers** - Node credentials (the highest risk) get maximum protection in .enc files
4. **Simplicity** - Auth can be managed with standard environment variables

## Environment Variables

**Available variables:**

Variables that ALWAYS override UI settings:
- `FRONTEND_PORT` or `PORT` - Web UI port (default: 7655)
- `API_TOKEN` - Token for API authentication (overrides UI)
- `PULSE_AUTH_USER` - Username for web UI authentication (overrides UI)
- `PULSE_AUTH_PASS` - Bcrypt password hash - MUST be 60 chars in single quotes! (overrides UI)
- `UPDATE_CHANNEL` - stable or rc (overrides UI)
- `AUTO_UPDATE_ENABLED` - true/false (overrides UI)
- `AUTO_UPDATE_CHECK_INTERVAL` - Hours between checks (overrides UI)
- `AUTO_UPDATE_TIME` - Update time HH:MM (overrides UI)
- `CONNECTION_TIMEOUT` - Connection timeout in seconds (overrides UI)
- `ALLOWED_ORIGINS` - CORS origins (overrides UI, default: empty = same-origin only)
- `LOG_LEVEL` - debug/info/warn/error (overrides UI)

Variables that only work if no system.json exists:
- `POLLING_INTERVAL` - Node check interval in seconds (default: 3)

Other variables:
- `DISCOVERY_SUBNET` - Network subnet for auto-discovery (default: auto-detect)
- `ALLOW_UNPROTECTED_EXPORT` - Allow export without auth (default: false)
- `PULSE_DEV` - Enable development mode features (default: false)

### 3. Secure Environment Variables
For sensitive data like API tokens and passwords:

```bash
# Edit systemd service
sudo systemctl edit pulse-backend

# Add secure environment variables:
[Service]
Environment="API_TOKEN=your-secure-token"
Environment="ALLOW_UNPROTECTED_EXPORT=true"

# Restart service
sudo systemctl restart pulse-backend
```

**Docker users:**
```bash
docker run -e API_TOKEN=secure-token -p 7655:7655 rcourtman/pulse:latest
```

## Data Storage

### Encrypted Storage
All sensitive data is automatically encrypted at rest using AES-256-GCM:
- Node passwords and API tokens
- Email server passwords  
- PBS credentials

The encryption key is auto-generated and stored in the data directory with restricted permissions.

### File Locations

**Docker Container:**
- Base directory: `/data` (mounted volume)
- Config files: `/data/*.json`, `/data/*.enc`
- Encryption key: `/data/.encryption.key`
- Auth config: `/data/.env` (created by security wizard)
- Metrics: `/data/metrics/`
- Logs: Container logs (`docker logs pulse`)

**LXC/Native Install:**
- Base directory: `/etc/pulse`
- Config files: `/etc/pulse/*.json`, `/etc/pulse/*.enc`
- Encryption key: `/etc/pulse/.encryption.key`
- Metrics: `/etc/pulse/metrics/`
- Logs: `/etc/pulse/pulse.log` or journalctl
- Optional: `/etc/pulse/.env` for env overrides

**Files created (both deployments):**
- `system.json` - UI-managed settings
- `.encryption.key` - Auto-generated encryption key (do not share!)
- `nodes.enc` - Encrypted node credentials
- `email.enc` - Encrypted email settings

## Common Configuration Tasks

### Change the Web Port

**Docker:**
```bash
# Stop existing container
docker stop pulse

# Run with new port
docker run -d --name pulse \
  -e FRONTEND_PORT=8080 \
  -p 8080:8080 \
  -v pulse_data:/data \
  rcourtman/pulse:latest
```

**LXC/Systemd:**
```bash
echo "FRONTEND_PORT=8080" >> /etc/pulse/.env
sudo systemctl restart pulse-backend
```

### Enable API Authentication
```bash
sudo systemctl edit pulse-backend
# Add: Environment="API_TOKEN=your-secure-token"
sudo systemctl restart pulse-backend
```

### Configure for Reverse Proxy

**Docker:**
```bash
docker run -d --name pulse \
  -e ALLOWED_ORIGINS="https://pulse.example.com" \
  -p 7655:7655 \
  -v pulse_data:/data \
  rcourtman/pulse:latest
```

**LXC/Systemd:**
```bash
echo "ALLOWED_ORIGINS=https://pulse.example.com" >> /etc/pulse/.env
sudo systemctl restart pulse-backend
```

### Enable Debug Logging
```bash
echo "LOG_LEVEL=debug" >> /etc/pulse/.env
sudo systemctl restart pulse-backend
tail -f /etc/pulse/pulse.log
```

### Configure Discovery Subnet (Docker)
By default, Docker containers may only discover nodes on the Docker bridge network. To scan your actual network:
```bash
docker run -d \
  -e DISCOVERY_SUBNET=192.168.1.0/24 \
  -p 7655:7655 \
  rcourtman/pulse:latest
```
Replace `192.168.1.0/24` with your actual network subnet.

## Security Notes

⚠️ **Never put sensitive data in .env files!**
- .env files are not encrypted
- Use systemd environment variables for API_TOKEN
- Node credentials are always stored encrypted

## Node Setup Details

### Auto-Registration Script
The setup script generated for each discovered node:
1. Creates monitoring user (`pulse-monitor@pam` or `pulse-monitor@pbs`)
2. Sets minimal permissions (PVEAuditor or Datastore.Audit)
3. Generates API token with timestamp
4. Registers with Pulse automatically
5. Optionally cleans up old tokens

Example:
```bash
curl -sSL "http://pulse:7655/api/setup-script?type=pve&host=https%3A%2F%2F192.168.1.10%3A8006" | bash
```

### Manual Setup

If auto-registration isn't suitable, you can still set up manually:

**Proxmox VE:**
```bash
pveum user add pulse-monitor@pam
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor
pveum user token add pulse-monitor@pam pulse-token --privsep 0
```

**PBS:**
```bash
proxmox-backup-manager user create pulse-monitor@pbs
proxmox-backup-manager acl update / Admin --auth-id pulse-monitor@pbs
proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token
```

## Reverse Proxy Configuration

Pulse requires WebSocket support for real-time updates. If using a reverse proxy (nginx, Apache, Caddy, etc.), you **MUST** enable WebSocket proxying.

See the [Reverse Proxy Guide](REVERSE_PROXY.md) for detailed configurations.

## Troubleshooting

### Port Already in Use
Check what's using the port:
```bash
sudo lsof -i :7655
```

### Permission Denied
Ensure Pulse has write access:
```bash
sudo chown -R pulse:pulse /etc/pulse
```

### Changes Not Taking Effect
Always restart after configuration changes:
```bash
sudo systemctl restart pulse-backend
```