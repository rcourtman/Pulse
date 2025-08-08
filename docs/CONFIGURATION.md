# Pulse Configuration Guide

## Configuration Methods

Pulse uses different methods for different types of settings:

### 1. Web UI Configuration
Most settings are configured through the web interface:
- **Nodes**: 
  - Auto-discovers nodes on your network
  - One-click setup with generated scripts
  - Automatic cluster detection
  - Manual add/remove also available
- **Alerts**: Set thresholds and notification rules  
- **Updates**: Configure update channels and auto-update
- **Security**: Export/import encrypted configurations

### 2. Environment File (.env)
For non-sensitive system settings that require a restart:

```bash
# Copy the example file
sudo cp /opt/pulse/.env.example /etc/pulse/.env

# Edit settings
sudo nano /etc/pulse/.env

# Restart to apply changes
sudo systemctl restart pulse-backend
```

**Available .env settings:**
- `FRONTEND_PORT` - Web UI port (default: 7655)
- `ALLOWED_ORIGINS` - CORS settings for reverse proxies
- `POLLING_INTERVAL` - How often to check nodes (seconds)
- `LOG_LEVEL` - Logging verbosity (debug/info/warn/error)
- `UPDATE_CHANNEL` - stable/beta/dev
- `METRICS_RETENTION_DAYS` - How long to keep metrics

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

The encryption key is auto-generated and stored at `/etc/pulse/.encryption.key` with restricted permissions.

### File Locations
- **Configuration**: `/etc/pulse/` (or `./data` if not writable)
  - `.env` - Non-sensitive settings
  - `.encryption.key` - Auto-generated encryption key (do not share!)
  - `nodes.enc` - Encrypted node credentials
  - `email.enc` - Encrypted email settings
  - `system.json` - General settings
- **Metrics**: `/etc/pulse/metrics/`
- **Logs**: `/etc/pulse/pulse.log`

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

## Security Notes

⚠️ **Never put sensitive data in .env files!**
- .env files are not encrypted
- Use systemd environment variables for API_TOKEN
- Node credentials are always stored encrypted

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