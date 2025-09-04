# Pulse Troubleshooting Guide

## Common Issues and Solutions

### Authentication Problems

#### Forgot Password / Lost Access

**Solution: Start Fresh**

If you've forgotten your password, the recommended approach is to simply start fresh. Pulse is designed for quick setup - it takes just 2-3 minutes to be fully operational again.

**Why no password recovery?**
- Adding recovery mechanisms creates security vulnerabilities
- Pulse setup is intentionally simple and fast
- You're not losing important data (Pulse only tracks current state, not history)
- Your nodes will immediately repopulate with all VMs/containers

**Steps to start fresh:**
1. Stop Pulse
2. Delete your configuration/data directory (`/etc/pulse`, `/data`, or wherever you configured it)
3. Restart Pulse
4. Run Quick Security Setup (30 seconds)
5. Add your nodes back (another 30 seconds)

That's it. Your infrastructure will be fully visible again immediately.

**Prevention:**
- Use a password manager
- Document your credentials securely
- Consider using API tokens for automation

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
See [Forgot Password / Lost Access](#forgot-password--lost-access) section above.

**Recommended approach**: Start fresh. Delete your Pulse data and restart - takes 2 minutes to set up again.

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