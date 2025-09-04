# Pulse Troubleshooting Guide

## Common Issues and Solutions

### Authentication Problems

#### Forgot Password / Password Reset

**Problem**: Can't remember the password and are locked out of the Pulse interface.

**Solutions**:

**Option 1: Use the password reset script (Native Install)**
```bash
# Run the password reset script
sudo /opt/pulse/scripts/reset-password.sh

# Choose option 1 to reset password (keeps username)
# Choose option 2 to set new username and password
# Choose option 3 to disable authentication temporarily
# Choose option 4 to check current status
```

**Option 2: Manually reset via .env file (Native Install)**
```bash
# Edit the .env file directly
sudo nano /opt/pulse/.env

# Option A: Disable authentication temporarily
# Add or modify this line:
DISABLE_AUTH=true
# Then restart: sudo systemctl restart pulse

# Option B: Set new credentials
# Add or modify these lines:
PULSE_AUTH_USER=yournewusername
PULSE_AUTH_PASS=yournewpassword
# Then restart: sudo systemctl restart pulse
```

**Option 3: Docker - Reset authentication**
```bash
# Option A: Disable auth temporarily
docker exec pulse sh -c "echo 'DISABLE_AUTH=true' > /data/.env"
docker restart pulse

# Option B: Set new credentials
docker exec pulse sh -c "cat > /data/.env << 'EOF'
PULSE_AUTH_USER=yournewusername  
PULSE_AUTH_PASS=yournewpassword
EOF"
docker restart pulse

# Option C: Remove auth completely and use Quick Setup
docker exec pulse rm -f /data/.env
docker restart pulse
# Then access the UI and use Quick Security Setup
```

**Option 4: ProxmoxVE LXC - Reset from console**
```bash
# Access the container console in Proxmox
# Then run:
/opt/pulse/scripts/reset-password.sh

# Or manually edit:
nano /opt/pulse/.env
# Add: DISABLE_AUTH=true
# Then: systemctl restart pulse
```

**Important Notes:**
- Passwords are hashed with bcrypt when Pulse starts (you'll see a 60-character hash starting with $2)
- Minimum password length is 8 characters
- The .env file is located at `/opt/pulse/.env` (native) or `/data/.env` (Docker)
- After resetting, you can re-enable authentication through the UI's security settings

#### Cannot login after setting up security
**Symptoms**: "Invalid username or password" error despite correct credentials

**Common causes and solutions:**

1. **Truncated bcrypt hash** (most common)
   - Check hash is exactly 60 characters: `echo -n "$PULSE_AUTH_PASS" | wc -c`
   - Look for error in logs: `Bcrypt hash appears truncated!`
   - Solution: Use full 60-character hash or Quick Security Setup

2. **Docker Compose $ character issue**
   - Docker Compose interprets `$` as variable expansion
   - **Wrong**: `PULSE_AUTH_PASS='$2a$12$hash...'`
   - **Right**: `PULSE_AUTH_PASS='$$2a$$12$$hash...'` (escape with $$)
   - Alternative: Use a .env file where no escaping is needed

3. **Environment variable not loaded**
   - Check if variable is set: `docker exec pulse env | grep PULSE_AUTH`
   - Verify quotes around hash: Must use single quotes
   - Restart container after changes

#### Password change fails
**Error**: `exec: "sudo": executable file not found`

**Solution**: Update to v4.3.8+ which removes sudo requirement. For older versions:
```bash
# Manually update .env file
docker exec pulse sh -c "echo \"PULSE_AUTH_PASS='new-hash'\" >> /data/.env"
docker restart pulse
```

#### Can't access Pulse - stuck at login
**Symptoms**: Can't access Pulse after upgrade, no credentials work

**Solution**: 
- If upgrading from pre-v4.5.0, you need to complete security setup first
- Clear browser cache and cookies
- Access http://your-ip:7655 to see setup wizard
- Complete setup, then restart container

### Docker-Specific Issues

#### No .env file in /data
**This is expected behavior** when using environment variables. The .env file is only created by:
- Quick Security Setup wizard
- Password change through UI
- Manual creation

If you provide auth via `-e` flags or docker-compose environment section, no .env is created.

#### Container won't start
Check logs: `docker logs pulse`

Common issues:
- Port already in use: Change port mapping
- Volume permissions: Ensure volume is writable
- Invalid environment variables: Check syntax

### Installation Issues

#### Binary not found (v4.3.7)
**Error**: `/opt/pulse/pulse: No such file or directory`

**Cause**: v4.3.7 install script bug

**Solution**: Update to v4.3.8 or manually fix:
```bash
sudo mkdir -p /opt/pulse/bin
sudo mv /opt/pulse/pulse /opt/pulse/bin/pulse
sudo systemctl daemon-reload
sudo systemctl restart pulse
```

#### Service name confusion
Pulse uses different service names depending on installation method:
- **ProxmoxVE Script**: `pulse`
- **Manual Install**: `pulse-backend`
- **Docker**: N/A (container name)

To check which you have:
```bash
systemctl status pulse 2>/dev/null || systemctl status pulse-backend
```

### Notification Issues

#### Emails not sending
1. Check email configuration in Settings → Alerts
2. Verify SMTP settings and credentials
3. Check logs for errors: `docker logs pulse | grep -i email`
4. Test with a simple webhook first

#### Webhook not working
- Verify URL is accessible from Pulse server
- Check for SSL certificate issues
- Try a test service like webhook.site
- Check logs for response codes

### Performance Issues

#### High CPU usage
- Polling interval is fixed at 10 seconds (matches Proxmox update cycle)
- Check number of monitored nodes
- Disable unused features (snapshots, backups monitoring)

#### High memory usage
- Normal for monitoring many nodes
- Check metrics retention settings
- Restart container to clear any memory leaks

### Network Issues

#### Cannot connect to Proxmox nodes
1. Verify Proxmox API is accessible:
   ```bash
   curl -k https://proxmox-ip:8006
   ```
2. Check credentials have proper permissions (PVEAuditor minimum)
3. Verify network connectivity between Pulse and Proxmox
4. Check for firewall rules blocking port 8006

#### PBS connection issues
- Ensure API token has Datastore.Audit permission
- Check PBS is accessible on port 8007
- Verify token format: `user@realm!tokenid=secret`

### Update Issues

#### Updates not showing
- Check update channel in Settings → System
- Verify internet connectivity
- Check GitHub API rate limits
- Manual update: Pull latest Docker image or run install script

#### Update fails to apply
**Docker**: Pull new image and recreate container
**Native**: Run install script again or check logs

### Data Recovery

#### Lost authentication
If you've lost access and need to reset:

**Native Install (Recommended)**:
```bash
# Use the password reset script
sudo /opt/pulse/scripts/reset-password.sh
# Follow the prompts to reset or disable auth
```

**Docker**:
```bash
# Option 1: Reset credentials
docker exec pulse sh -c "echo 'DISABLE_AUTH=true' > /data/.env"
docker restart pulse
# Access UI and set new credentials

# Option 2: Remove auth completely
docker exec pulse rm /data/.env
docker restart pulse
# Access UI and use Quick Security Setup
```

**Manual Reset**:
```bash
# Native install
sudo nano /opt/pulse/.env
# Add: DISABLE_AUTH=true
sudo systemctl restart pulse  # or pulse-backend

# Docker
docker exec pulse sh -c "echo 'DISABLE_AUTH=true' > /data/.env"
docker restart pulse
```

#### Corrupt configuration
Restore from backup or delete config files to start fresh:
```bash
# Docker
docker exec pulse rm /data/*.json /data/*.enc
docker restart pulse

# Native
sudo rm /etc/pulse/*.json /etc/pulse/*.enc
sudo systemctl restart pulse
```

## Getting Help

### Collect diagnostic information
```bash
# Version
curl http://localhost:7655/api/version

# Logs (last 100 lines)
docker logs --tail 100 pulse  # Docker
journalctl -u pulse -n 100    # Native

# Environment
docker exec pulse env | grep -E "PULSE|API"  # Docker
systemctl show pulse --property=Environment  # Native
```

### Report issues
When reporting issues, include:
1. Pulse version
2. Deployment type (Docker/LXC/Manual)
3. Error messages from logs
4. Steps to reproduce
5. Expected vs actual behavior

Report at: https://github.com/rcourtman/Pulse/issues