# ⚠️ CRITICAL UPGRADE NOTICE - Pulse v4.3.9

## Breaking Change: Authentication is Now Mandatory

### What Changed
Starting with v4.3.9, Pulse **requires** authentication to be configured. This is a security requirement, not a feature - Pulse stores Proxmox API tokens that often have write permissions, and leaving these exposed on your network is a serious security risk.

### What This Means for You

#### If upgrading from v4.3.8 or earlier:
1. **Your configuration is safe** - All nodes, alerts, and settings are preserved
2. You'll see a one-time security setup wizard when accessing Pulse
3. The setup takes about 30 seconds
4. After setup, everything works exactly as before, just secured

#### For Docker users:
```bash
# Pull latest
docker pull rcourtman/pulse:latest

# Stop and recreate (your data in pulse_data volume is safe)
docker stop pulse
docker rm pulse
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

#### For systemd users:
```bash
# Re-run the install script
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash

# Or manually update
cd /opt/pulse
sudo ./install.sh
```

#### For ProxmoxVE Helper Script users:
```bash
# In your Pulse container console
update
```

### What Happens During Upgrade

1. **First access after upgrade**: You'll see the security setup wizard
2. **Create credentials**: Choose a username and password (or use generated password)
3. **API token generated**: Automatically creates a secure API token
4. **Immediate access**: No restart needed - you're logged in immediately
5. **All settings preserved**: Your nodes, alerts, webhooks - everything is still there

### Why We Made This Change

We discovered that many users were running Pulse exposed on their networks without authentication, unknowingly exposing Proxmox API tokens that could have write access to their infrastructure. Rather than make this an optional security warning that could be dismissed, we made the difficult decision to enforce security by default.

We understand this is disruptive, but we believe protecting your infrastructure is worth the one-time inconvenience.

### Need to Export Your Config First?

If you want to backup your configuration before upgrading (not necessary, but for peace of mind):

#### For v4.3.8 users without auth:
```bash
# Export your config (works without auth in v4.3.8)
curl http://your-pulse:7655/api/export -o pulse-backup.json
```

#### For users with auth:
```bash
# Export with API token
curl -H "X-API-Token: your-token" http://your-pulse:7655/api/export -o pulse-backup.json
```

### Questions or Issues?

- GitHub Issues: https://github.com/rcourtman/Pulse/issues
- The upgrade preserves all data - if something goes wrong, your config is still in `/etc/pulse/` or `/data/`

### For API/Automation Users

If you're using Pulse's API for automation:
1. Complete the security setup via web UI first
2. Generate an API token in Settings → Security
3. Update your scripts to include the token:
   ```bash
   curl -H "X-API-Token: your-token" http://pulse:7655/api/state
   ```

---

We apologize for the disruption, but this change was necessary to protect your infrastructure. The setup takes less than a minute, and your Proxmox environments will be much more secure.