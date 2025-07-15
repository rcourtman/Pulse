# Pulse Troubleshooting Guide

## Quick Diagnostics

**First step for any issue:** Run the web diagnostics tool
```
http://<pulse-ip>:7655/diagnostics.html
```

This tool automatically checks:
- ✓ Configuration validity
- ✓ API connectivity
- ✓ Permission levels
- ✓ Data collection status

## Top 5 Most Common Issues

### 1. Empty Dashboard / No VMs Showing

**Symptom:** Pulse loads but shows no VMs or containers

**Cause:** Missing API token permissions (90% of cases)

**Quick Check:**
```bash
# Use Pulse's automated permission checker
cd /opt/pulse
./scripts/check-pve-permissions.sh
```

**Manual Solution:**
```bash
# Check if token has permissions
pveum user permissions user@pam!token

# If missing, grant permissions to the USER (not token)
pveum acl modify / --users user@pam --roles PVEAuditor
```

**Key points:**
- Permissions must be on path `/` (root), not `/access`
- "Propagate" must be checked
- With privilege separation disabled, set permissions on USER only

### 2. Backups Not Showing

**PVE Backups Missing:**
```bash
# Use Pulse's automated permission checker
cd /opt/pulse
./scripts/check-pve-permissions.sh

# Automatically fix permission issues
./scripts/check-pve-permissions.sh --fix

# Manual fix - need additional permission for storage
pveum acl modify /storage --users user@pam --roles PVEDatastoreAdmin
```

**PBS Backups Missing:**
```bash
# Use Pulse's automated permission checker
cd /opt/pulse
./scripts/check-pbs-permissions.sh

# Automatically fix permission issues (requires admin credentials)
./scripts/check-pbs-permissions.sh --fix
```
- Verify PBS connection is configured in settings
- Check PBS permissions: `DatastoreAudit` on `/datastore`

### 3. Can't Access Pulse Web Interface

**Check if service is running:**
```bash
# Systemd
sudo systemctl status pulse-monitor
sudo netstat -tlnp | grep 7655

# Docker
docker ps | grep pulse
docker logs pulse
```

**Common causes:**
- Wrong IP address (use container/VM IP, not Proxmox host)
- Firewall blocking port 7655
- Service not started

### 4. API Connection Failed

**In diagnostics, seeing "Connection refused" or "403 Forbidden"**

**Check:**
1. Correct URL format: `https://proxmox-ip:8006` (not http)
2. Token format: `user@realm!tokenname`
3. Self-signed certificates: Enable in settings
4. Network connectivity: `curl https://proxmox-ip:8006`

### 5. Updates Not Working

**Docker:** Updates must be done via Docker, not web UI
```bash
docker compose pull && docker compose up -d
```

**LXC/Manual:** 
```bash
# Try manual update
sudo /opt/pulse/scripts/install-pulse.sh --update

# Check update logs
cat /var/log/pulse_update.log
```

## Detailed Troubleshooting

### Permission Issues

**Automated Permission Checking:**

Pulse includes scripts to automatically verify and fix API token permissions:

**For Proxmox VE:**
```bash
# Run from Pulse installation directory
cd /opt/pulse
./scripts/check-pve-permissions.sh

# Automatically fix permission issues
./scripts/check-pve-permissions.sh --fix

# Or specify token details directly
./scripts/check-pve-permissions.sh --url https://proxmox:8006 \
  --token-id user@pam!token --token-secret your-secret

# Specify token details with automatic fix
./scripts/check-pve-permissions.sh --url https://proxmox:8006 \
  --token-id user@pam!token --token-secret your-secret --fix
```

**For Proxmox Backup Server:**
```bash
# Run from Pulse installation directory
cd /opt/pulse
./scripts/check-pbs-permissions.sh

# Automatically fix permission issues (requires admin credentials)
./scripts/check-pbs-permissions.sh --fix

# Or specify token details directly
./scripts/check-pbs-permissions.sh --url https://pbs:8007 \
  --token-id user@pbs!token --token-secret your-secret

# Specify token details with automatic fix
./scripts/check-pbs-permissions.sh --url https://pbs:8007 \
  --token-id user@pbs!token --token-secret your-secret --fix \
  --admin-user admin@pbs --admin-password your-admin-password
```

These scripts will:
- ✓ Test API connectivity
- ✓ Verify token authentication
- ✓ Check all required permissions
- ✓ Provide specific fixes for any issues found
- ✓ Automatically apply fixes with --fix flag
- ✓ Work without jq installed (important for Docker environments)
- ✓ Correctly parse and validate permissions

**Understanding Token Permissions:**

1. **Token WITH privilege separation (default):**
   ```bash
   # Must set permissions on BOTH user AND token
   pveum acl modify / --users user@pam --roles PVEAuditor
   pveum acl modify / --tokens user@pam!token --roles PVEAuditor
   ```

2. **Token WITHOUT privilege separation (recommended):**
   ```bash
   # Only set permissions on user
   pveum acl modify / --users user@pam --roles PVEAuditor
   ```

**How to check privilege separation:**
```bash
pveum user token list user@pam --output-format json-pretty
# Look for "privsep": 0 (disabled) or 1 (enabled)
```

### Network Issues

**Test connectivity:**
```bash
# From Pulse host
curl -k https://proxmox-ip:8006
curl -k https://pbs-ip:8007

# Check DNS
nslookup proxmox-hostname
```

**Common network problems:**
- Proxmox using private CA certificate
- Firewall blocking ports 8006/8007
- DNS not resolving hostnames
- Proxy intercepting connections

### Performance Issues

**High Memory Usage:**
- Reduce `BACKUP_HISTORY_DAYS` (default 365)
- Normal memory usage: 100-200MB
- With many VMs: up to 500MB

**Slow Loading:**
- Check network latency to Proxmox
- Verify Proxmox host performance
- Consider reducing polling frequency

### Alert Issues

**Alerts not triggering:**
1. Check alert configuration in settings
2. Verify thresholds are exceeded for duration
3. Check notification configuration
4. Review alert history in UI

**Too many alerts:**
- Increase threshold values
- Increase duration requirements
- Use per-VM custom thresholds

### Notification Problems

**Webhooks not working:**
```bash
# Test webhook manually
curl -X POST https://discord.com/api/webhooks/... \
  -H "Content-Type: application/json" \
  -d '{"content":"Test from Pulse"}'
```

**Email not sending:**
- Gmail: Must use App Password, not regular password
- Check SMTP port (587 for TLS, 465 for SSL)
- Verify firewall allows outbound SMTP

## Log Locations

### Systemd Installation
```bash
# Service logs
sudo journalctl -u pulse-monitor.service -f

# Application logs
/opt/pulse/logs/pulse.log  # If configured

# Update logs
/var/log/pulse_update.log
```

### Docker Installation
```bash
# Container logs
docker logs pulse -f

# Or with docker-compose
docker compose logs -f
```

## Advanced Debugging

### Enable Debug Mode

**Environment variable:**
```bash
DEBUG=pulse:* NODE_ENV=development
```

**Docker compose:**
```yaml
environment:
  - DEBUG=pulse:*
  - NODE_ENV=development
```

### API Token Testing

**Test token manually:**
```bash
# Test PVE token
curl -k -H "Authorization: PVEAPIToken=user@pam!token=secret" \
  https://proxmox:8006/api2/json/cluster/resources

# Test PBS token  
curl -k -H "Authorization: PBSAPIToken=user@pbs!token=secret" \
  https://pbs:8007/api2/json/nodes
```

### Database Issues

**Reset alert state:**
```bash
# Stop Pulse first
rm /opt/pulse/data/active-alerts.json
rm /opt/pulse/data/alert-rules.json
# Restart Pulse
```

## Getting Help

1. **Run diagnostics first** - Provides info needed for support
2. **Check existing issues** - https://github.com/rcourtman/Pulse/issues
3. **Create new issue with:**
   - Pulse version
   - Installation method
   - Diagnostic export (sanitized)
   - Relevant logs

## FAQ

**Q: Why does Pulse need PVEDatastoreAdmin for read-only access?**
A: Proxmox API limitation - viewing backup contents requires this permission even for read-only access. Unfortunately, PVEDatastoreAdmin includes write permissions. See our [Security Guide](../SECURITY.md#api-token-permissions-and-security) for details and mitigation strategies.

**Q: Can I run multiple Pulse instances?**
A: Yes, each instance is independent. Use different ports and storage locations.

**Q: Does Pulse support Proxmox clusters?**
A: Yes, connect to any cluster node and Pulse discovers all nodes automatically.

**Q: How do I monitor PBS behind a firewall?**
A: Use PBS Push Mode - see [PBS Push Mode Guide](PBS_PUSH_MODE.md).