# Pulse Configuration Guide

## Configuration Methods

Pulse uses different methods for different types of settings:

### 1. Web UI Configuration
Most settings are configured through the web interface:

- **Nodes**: Auto-discovery, one-click setup scripts, cluster detection
- **Alerts**: Thresholds and notification rules  
- **Updates**: Update channels and auto-update settings
- **Security**: Export/import encrypted configurations

### 2. Environment Variables (Deployment Overrides)
Environment variables override UI settings. Use them for Docker/systemd deployments:

**Docker Example:**
```bash
docker run -d \
  -e UPDATE_CHANNEL=rc \
  -e POLLING_INTERVAL=10 \
  -e API_TOKEN=your-secure-token \
  rcourtman/pulse:latest
```

**Systemd Example:**
```bash
# Edit service
sudo systemctl edit pulse-backend

# Add overrides:
[Service]
Environment="UPDATE_CHANNEL=rc"
Environment="POLLING_INTERVAL=10"
```

**.env File (Optional):**
You can use a .env file for convenience, but UI settings take precedence:
```bash
# Create .env for deployment overrides
sudo nano /etc/pulse/.env
# Add: UPDATE_CHANNEL=rc
sudo systemctl restart pulse-backend
```

**Available variables:**
- `FRONTEND_PORT` - Web UI port
- `POLLING_INTERVAL` - Node check interval (seconds)
- `CONNECTION_TIMEOUT` - Connection timeout (seconds)
- `UPDATE_CHANNEL` - stable or rc
- `AUTO_UPDATE_ENABLED` - true/false
- `AUTO_UPDATE_CHECK_INTERVAL` - Hours between checks
- `AUTO_UPDATE_TIME` - Update time (HH:MM)
- `ALLOWED_ORIGINS` - CORS origins
- `LOG_LEVEL` - debug/info/warn/error
- `DISCOVERY_SUBNET` - Network subnet for auto-discovery (default: auto, e.g., 192.168.0.0/24)

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
- **Configuration**: 
  - Standard install: `/etc/pulse/`
  - Docker: `/data` (mounted volume)
  - Fallback: `./data` if not writable
- **Files**:
  - `system.json` - UI-managed settings
  - `.encryption.key` - Auto-generated encryption key (do not share!)
  - `nodes.enc` - Encrypted node credentials
  - `email.enc` - Encrypted email settings
  - `.env` - Optional deployment overrides (if created)
- **Metrics**: `<data-dir>/metrics/`
- **Logs**: `<data-dir>/pulse.log`

> **Docker Note**: All configuration is persisted in the `/data` volume to survive container restarts

## Common Configuration Tasks

### Change the Web Port
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

Pulse v4 requires WebSocket support for real-time updates. If using a reverse proxy (nginx, Apache, Caddy, etc.), you **MUST** enable WebSocket proxying.

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