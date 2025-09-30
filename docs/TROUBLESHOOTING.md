# Pulse Troubleshooting Guide

## Common Issues and Solutions

### Authentication Problems

#### Forgot Password / Lost Access

**Solution: Start Fresh**

If you've forgotten your password, the recommended approach is to simply start fresh. Pulse is designed for quick setup.

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

### VM Disk Monitoring Issues

#### VMs show "-" for disk usage

**This is normal and expected** - VMs require QEMU Guest Agent to report disk usage.

**Quick fix:**
1. Install guest agent in VM: `apt install qemu-guest-agent` (Linux) or virtio-win tools (Windows)
2. Enable in Proxmox: VM → Options → QEMU Guest Agent → Enable
3. Restart the VM
4. Wait 10 seconds for Pulse to poll again

**Detailed troubleshooting:**

See [VM Disk Monitoring Guide](VM_DISK_MONITORING.md) for full setup instructions.

#### How to diagnose VM disk issues

**Step 1: Check if guest agent is running**

On Proxmox host:
```bash
# Check if agent is enabled in VM config
qm config <VMID> | grep agent

# Test if agent responds
qm agent <VMID> ping

# Get filesystem info (what Pulse uses)
qm agent <VMID> get-fsinfo
```

Inside the VM:
```bash
# Linux
systemctl status qemu-guest-agent

# Windows (PowerShell)
Get-Service QEMU-GA
```

**Step 2: Run diagnostic script**

```bash
# On Proxmox host
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/test-vm-disk.sh | bash
```

Or if Pulse is installed:
```bash
/opt/pulse/scripts/test-vm-disk.sh
```

**Step 3: Check Pulse logs**

```bash
# Docker
docker logs pulse | grep -i "guest agent\|fsinfo"

# Systemd
journalctl -u pulse -f | grep -i "guest agent\|fsinfo"
```

Look for specific error reasons:
- `agent-not-running` - Agent service not started in VM
- `agent-disabled` - Not enabled in VM config
- `agent-timeout` - Agent not responding (may need restart)
- `permission-denied` - Check permissions (see below)
- `no-filesystems` - Agent returned no usable filesystem data

#### Permission denied errors

If Pulse logs show permission denied when querying guest agent:

**Check permissions:**
```bash
# On Proxmox host
pveum user permissions pulse-monitor@pam
```

**Required permissions:**
- **Proxmox 9:** `PVEAuditor` role (includes `VM.GuestAgent.Audit`)
- **Proxmox 8:** `VM.Monitor` permission

**Fix permissions:**

Re-run the Pulse setup script on the Proxmox node:
```bash
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/setup-pve.sh | bash
```

Or manually:
```bash
# Proxmox 9
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor

# Proxmox 8
pveum role add PulseMonitor -privs VM.Monitor
pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
```

**Important:** Both API tokens and passwords work fine for guest agent access. If you see permission errors, it's a permission configuration issue, not an authentication method limitation.

#### Guest agent installed but no disk data

If agent responds to ping but returns no filesystem info:

1. **Check agent version** - Update to latest:
   ```bash
   # Linux
   apt update && apt install --only-upgrade qemu-guest-agent
   systemctl restart qemu-guest-agent
   ```

2. **Check filesystem permissions** - Agent needs read access to filesystem data

3. **Windows VMs** - Ensure VirtIO drivers are up to date from latest virtio-win ISO

4. **Special filesystems only** - If VM only has special filesystems (tmpfs, ISO mounts), this is normal for Live systems

#### Specific VM types

**Cloud images:**
- Most have guest agent pre-installed but disabled
- Enable with: `systemctl enable --now qemu-guest-agent`

**Windows VMs:**
- Must install VirtIO guest tools
- Ensure "QEMU Guest Agent" service is running
- May need "QEMU Guest Agent VSS Provider" for full functionality

**Container-based VMs (Docker/Kubernetes hosts):**
- Will show high disk usage due to container layers
- This is accurate - containers consume real disk space
- Consider monitoring container disk separately

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

**Recommended approach**: Start fresh. Delete your Pulse data and restart.

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