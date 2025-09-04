# Pulse Troubleshooting Guide

## Common Issues and Solutions

### Authentication Problems

#### Forgot Password / Lost Access

**⚠️ SECURITY WARNING**: Password reset requires server-level access. If someone can reset your Pulse password, they already have root/admin access to your server. **Secure your server access first!**

**Problem**: Can't remember the password and are locked out of the Pulse interface.

**Important Security Considerations**:
- These methods require server access (SSH/console)
- If an attacker can perform these steps, they already have server control
- Use a password manager to avoid needing password resets
- **Re-enable authentication immediately after regaining access**

## Secure Recovery Methods

### Method 1: Recovery Token (Most Secure)

Pulse includes a secure recovery token system for emergency access:

**Step 1: Generate Recovery Token (from server)**
```bash
# SSH into your server and generate a time-limited recovery token
curl -X POST http://localhost:7655/api/security/recovery \
  -H "Content-Type: application/json" \
  -d '{"action": "generate_token", "duration": 30}'

# This returns a token valid for 30 minutes (max 60)
# Save this token immediately - it's shown only once!
```

**Step 2: Use Recovery Token (from any browser)**
```bash
# Use the token to access the recovery endpoint
curl -X POST http://your-server:7655/api/security/recovery \
  -H "X-Recovery-Token: your-token-here" \
  -H "Content-Type: application/json" \
  -d '{"action": "disable_auth"}'

# Now you can access the UI and reset your password
```

**Step 3: Re-enable Authentication**
```bash
# After setting new password in the UI
curl -X POST http://localhost:7655/api/security/recovery \
  -H "Content-Type: application/json" \
  -d '{"action": "enable_auth"}'
```

**Security Features:**
- Tokens are single-use only
- Time-limited (30-60 minutes)
- Cryptographically secure (32 bytes of entropy)
- Logged for audit purposes
- Constant-time validation to prevent timing attacks

### Method 2: Emergency Recovery Mode (Localhost Only)

If you have direct server access but can't use tokens:

**Native Install:**
```bash
# Enable recovery mode (localhost access only)
echo "Recovery mode enabled at $(date)" | sudo tee /etc/pulse/.auth_recovery

# Access Pulse from the server itself (localhost)
curl http://localhost:7655  # or use local browser/port forward

# Set new credentials through the UI

# Disable recovery mode
sudo rm /etc/pulse/.auth_recovery
sudo systemctl restart pulse
```

**Docker:**
```bash
# Enable recovery mode
docker exec pulse sh -c "echo 'Recovery mode' > /data/.auth_recovery"

# Access from localhost (port forward if needed)
# Set new credentials

# Disable recovery mode
docker exec pulse rm /data/.auth_recovery
docker restart pulse
```

### Method 3: Manual Override (Last Resort)

⚠️ **Only use if other methods fail:**

```bash
# Temporarily bypass auth (native install)
echo "DISABLE_AUTH=true" | sudo tee -a /opt/pulse/.env
sudo systemctl restart pulse

# IMMEDIATELY set new credentials in UI
# Then remove the DISABLE_AUTH line and restart

# For Docker:
docker exec pulse sh -c "echo 'DISABLE_AUTH=true' >> /data/.env"
docker restart pulse
# Set credentials, then remove the line
```

**Best Practices to Avoid This Situation**:
1. **Use a password manager** - Store credentials securely
2. **Document credentials** - Keep in a secure location
3. **Set up API tokens** - Alternative authentication method
4. **Regular backups** - Include .env file in backups
5. **Secure server access** - Use SSH keys, disable root login, use fail2ban

**Alternative: API Token Access**:
If you have an API token configured, you can still access the API:
```bash
curl -H "X-API-Token: your-token-here" http://localhost:7655/api/config/system
```

**Security Reminder**: 
After regaining access:
1. Set a strong password (use a password generator)
2. Save credentials in a password manager
3. Review server access logs for unauthorized access
4. Consider implementing additional server security measures

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
See [Forgot Password / Lost Access](#forgot-password--lost-access) section above for detailed recovery instructions.

**⚠️ Security Note**: Password recovery requires root access. If someone can reset your password, they already have full control of your server. Focus on securing server access (SSH keys, firewall rules, etc.) rather than worrying about Pulse password resets.

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