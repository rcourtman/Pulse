# Pulse Configuration Guide

This guide covers all configuration options for Pulse. Most users should use the web-based configuration interface (Settings menu) rather than editing files directly.

## Table of Contents

- [Web-Based Configuration](#web-based-configuration)
- [Environment Variables](#environment-variables)
- [Alert System](#alert-system)
- [Notifications](#notifications)
- [Multiple Environments](#multiple-environments)
- [Advanced Options](#advanced-options)
- [Security Best Practices](#security-best-practices)

## Web-Based Configuration

The recommended way to configure Pulse is through the web interface:

1. Click the settings icon (⚙️) in the top-right corner
2. Configure your connections and preferences
3. Test connections before saving
4. Changes apply immediately without restart

## Environment Variables

For advanced deployments, Pulse can be configured using environment variables in a `.env` file. The web interface will override these settings.

### Proxmox VE Configuration

**Required:**
```env
PROXMOX_HOST=https://192.168.1.10:8006
PROXMOX_TOKEN_ID=user@pam!tokenid
PROXMOX_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

**Optional:**
```env
PROXMOX_NODE_NAME=My Proxmox Cluster    # Display name in UI
PROXMOX_ALLOW_SELF_SIGNED_CERTS=true   # For self-signed certificates
PORT=7655                               # Pulse web server port
BACKUP_HISTORY_DAYS=365                 # Days of backup history to show
```

### Proxmox Backup Server Configuration

**Optional - for PBS monitoring:**
```env
PBS_HOST=https://192.168.1.11:8007
PBS_TOKEN_ID=user@pbs!tokenid
PBS_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
PBS_ALLOW_SELF_SIGNED_CERTS=true
PBS_PORT=8007
```

## Alert System

### Global Alert Thresholds

Configure when alerts should trigger:

```env
# Enable/disable alert types
ALERT_CPU_ENABLED=true
ALERT_MEMORY_ENABLED=true
ALERT_DISK_ENABLED=true
ALERT_DOWN_ENABLED=true

# Threshold percentages
ALERT_CPU_THRESHOLD=85          # Alert when CPU > 85%
ALERT_MEMORY_THRESHOLD=90       # Alert when Memory > 90%
ALERT_DISK_THRESHOLD=95         # Alert when Disk > 95%

# Duration before alerting (milliseconds)
ALERT_CPU_DURATION=300000       # 5 minutes
ALERT_MEMORY_DURATION=300000    # 5 minutes
ALERT_DISK_DURATION=600000      # 10 minutes
ALERT_DOWN_DURATION=60000       # 1 minute
```

### Per-VM/LXC Custom Thresholds

Configure custom thresholds for specific VMs through the web interface:

1. Navigate to Settings → Custom Thresholds
2. Click "Add Custom Threshold"
3. Select VM/LXC and configure thresholds
4. Custom thresholds override global settings

**Use Cases:**
- Storage VMs with high memory usage for caching
- Critical applications needing lower thresholds
- Development VMs with relaxed limits

## Notifications

### Webhook Notifications

Supports Discord, Slack, Teams, and generic webhooks:

```env
WEBHOOK_URL=https://discord.com/api/webhooks/...
WEBHOOK_ENABLED=true
```

**Setting up webhooks:**
- **Discord**: Server Settings → Integrations → Webhooks → New Webhook
- **Slack**: Apps → Incoming Webhooks → Add to Slack
- **Teams**: Channel → Connectors → Incoming Webhook

### Email Notifications

Configure SMTP for email alerts:

```env
SMTP_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password
SMTP_FROM=your-email@gmail.com
SMTP_TO=alerts@example.com,admin@example.com
SMTP_USE_SSL=true
```

**Gmail Setup:**
1. Enable 2-factor authentication
2. Generate App Password: Google Account → Security → App passwords
3. Use app password, not regular password

## Multiple Environments

Monitor multiple Proxmox clusters or PBS instances:

### Multiple PVE Clusters

```env
# Cluster 1 (Production)
PROXMOX_HOST=https://prod-pve.example.com:8006
PROXMOX_TOKEN_ID=monitor@pam!pulse
PROXMOX_TOKEN_SECRET=xxx-xxx-xxx
PROXMOX_NODE_NAME=Production Cluster

# Cluster 2 (Development)
PROXMOX_HOST_2=https://dev-pve.example.com:8006
PROXMOX_TOKEN_ID_2=monitor@pam!pulse
PROXMOX_TOKEN_SECRET_2=yyy-yyy-yyy
PROXMOX_NODE_NAME_2=Development Cluster

# Cluster 3 (DR Site)
PROXMOX_HOST_3=https://dr-pve.example.com:8006
PROXMOX_TOKEN_ID_3=monitor@pam!pulse
PROXMOX_TOKEN_SECRET_3=zzz-zzz-zzz
PROXMOX_NODE_NAME_3=DR Site
```

### Multiple PBS Instances

```env
# PBS Instance 1
PBS_HOST=https://pbs1.example.com:8007
PBS_TOKEN_ID=monitor@pbs!pulse
PBS_TOKEN_SECRET=aaa-aaa-aaa

# PBS Instance 2
PBS_HOST_2=https://pbs2.example.com:8007
PBS_TOKEN_ID_2=monitor@pbs!pulse
PBS_TOKEN_SECRET_2=bbb-bbb-bbb
```

## Advanced Options

### Performance Tuning

```env
# Reduce backup history for better performance
BACKUP_HISTORY_DAYS=90          # Default: 365

# Enable debug logging (development only)
NODE_ENV=development
DEBUG=pulse:*
```

### Security Options

```env
# Security mode: public (no auth) or private (auth required)
SECURITY_MODE=private           # Default: private

# Admin password for private mode
ADMIN_PASSWORD=your-secure-password

# Session configuration
SESSION_SECRET=random-64-char-string
SESSION_TIMEOUT_HOURS=24        # Default: 24

# Allow embedding in iframes (for dashboards)
ALLOW_EMBEDDING=true            # Default: false

# Specify allowed origins for iframe embedding
# Comma-separated list of origins that can embed Pulse
# Only applies when ALLOW_EMBEDDING=true
# HTTP is allowed for local networks (192.168.x.x, 10.x.x.x, *.local, *.lan)
# HTTPS is required for public addresses
ALLOWED_EMBED_ORIGINS=http://homepage.lan:3000,https://dashboard.example.com

# PBS Push Mode (for isolated servers)
PULSE_PUSH_API_KEY=secure-random-key

# Audit logging
AUDIT_LOG=true                  # Default: true
```

## Iframe Embedding

Pulse can be embedded in other dashboards using iframes. This is useful for integrating Pulse into Homepage, Organizr, or custom dashboards.

### Configuration

1. **Enable embedding** - Set `ALLOW_EMBEDDING=true` in your `.env` file or enable "Allow iframe embedding" in Settings → Advanced
2. **Add allowed origins** - Set `ALLOWED_EMBED_ORIGINS` with comma-separated origins that can embed Pulse

### Security Notes

- **HTTP allowed for local networks**: 192.168.x.x, 10.x.x.x, 172.16-31.x.x, *.local, *.lan
- **HTTPS required for public addresses**: Any non-local addresses must use HTTPS
- **Exact origin matching**: Origins must match exactly (including protocol and port)
- **CSP frame-ancestors**: Modern browsers use Content Security Policy for iframe control
- **X-Frame-Options fallback**: Legacy browsers fall back to X-Frame-Options header

### Example: Homepage Integration

1. In Pulse, add your Homepage origin:
   ```env
   ALLOWED_EMBED_ORIGINS=http://homepage.lan:3000
   ```

2. In Homepage's `services.yaml`:
   ```yaml
   - Monitoring:
       - Pulse Monitor:
           icon: mdi-monitor-dashboard
           href: http://pulse.lan:7655/
           description: Proxmox monitoring
           widget:
             type: iframe
             src: http://pulse.lan:7655
             height: 400
   ```

### Troubleshooting

- **"Refused to frame" error**: Check that the origin in the error message exactly matches what's in `ALLOWED_EMBED_ORIGINS`
- **Use browser DevTools**: The console will show the exact CSP policy and which origin is being blocked
- **Hostname vs IP**: If accessing Homepage via `homepage.lan`, add `http://homepage.lan:3000`, not the IP address

### Update System

```env
# Force update channel preference
UPDATE_CHANNEL_PREFERENCE=stable # or 'rc'
UPDATE_TEST_MODE=false          # Enable test mode
```

## Security Configuration

Pulse supports two security modes:

1. **Public Mode** (`SECURITY_MODE=public`)
   - No authentication required
   - Only use on fully trusted networks
   - Suitable for home labs with no external access

2. **Private Mode** (`SECURITY_MODE=private`) - Default
   - Authentication required for all access
   - Username: `admin`
   - Password: Set via `ADMIN_PASSWORD` environment variable
   - Supports both web login and HTTP Basic Auth

For detailed security configuration, see the [Security Guide](../SECURITY.md).

## Security Best Practices

### API Token Security

1. **Never use root tokens** - Create dedicated monitoring users
2. **Disable privilege separation** for simpler permission management
3. **Use minimal permissions**:
   - PVE: `PVEAuditor` + `PVEDatastoreAdmin` (for backups)
   - PBS: `DatastoreAudit` only

### Example: Secure PVE Setup

```bash
# Create monitoring user
pveum user add pulse@pam --comment "Pulse monitoring"

# Create token without privilege separation
pveum user token add pulse@pam monitoring --privsep 0

# Grant minimal permissions
pveum acl modify / --users pulse@pam --roles PVEAuditor
pveum acl modify /storage --users pulse@pam --roles PVEDatastoreAdmin
```

### Example: Secure PBS Setup

```bash
# Create monitoring user
proxmox-backup-manager user create pulse@pbs --password 'ChangeMe123'

# Generate token
proxmox-backup-manager user generate-token pulse@pbs monitoring

# Grant read-only access
proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse@pbs!monitoring'
```

### Network Security

- Use HTTPS for all connections
- Restrict API access with firewalls
- Consider VPN for external monitoring
- Regularly rotate API tokens

## Configuration File Locations

- **Docker**: `/usr/src/app/config/.env` (inside container)
- **LXC/Manual**: `/opt/pulse/.env`
- **Data files**: `/data/` directory:
  - `metrics-snapshot.json.gz` - Historical metrics
  - `alert-rules.json` - Alert configuration
  - `custom-thresholds.json` - Per-VM thresholds
  - `active-alerts.json` - Current alert state

## Troubleshooting Configuration

1. **Test connections** using the web diagnostics: `http://pulse:7655/diagnostics.html`
2. **Check logs** for configuration errors:
   - Docker: `docker logs pulse`
   - LXC/Manual: `sudo journalctl -u pulse-monitor.service -f`
3. **Verify permissions** match the requirements above
4. **Ensure network connectivity** between Pulse and Proxmox APIs