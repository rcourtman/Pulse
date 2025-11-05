# Pulse Troubleshooting Guide

## Common Issues and Solutions

### Correlate API Calls with Logs

Every API response includes an `X-Request-ID` header. When escalating issues, capture that value and use it to search the backend logs or log file. The same identifier is emitted as `request_id` in structured logs.

```bash
# Capture a request ID
curl -i https://pulse.example.com/api/state | grep X-Request-ID

# Search the rotating log file
grep 'request_id=abc123' /var/log/pulse/pulse.log

# Docker / kubectl example
docker logs pulse | grep 'request_id=abc123'
```

Include the `X-Request-ID` in support tickets or incident notes so responders can jump straight to the relevant log lines.

### Authentication Problems

#### Forgot Password / Lost Access

**Solution: Use the built-in recovery endpoint**

Pulse ships with a guarded recovery API that lets you regain access without wiping configuration.

1. **From the Pulse host (localhost only)**  
   Generate a short-lived recovery token or temporarily disable auth:
   ```bash
   # Create a 30 minute recovery token (returns JSON with the token value)
   curl -s -X POST http://localhost:7655/api/security/recovery \
     -H 'Content-Type: application/json' \
     -d '{"action":"generate_token","duration":30}'

   # OR force local-only recovery access (writes .auth_recovery in the data dir)
   curl -s -X POST http://localhost:7655/api/security/recovery \
     -H 'Content-Type: application/json' \
     -d '{"action":"disable_auth"}'
   ```

2. **If you generated a token**, use it from a trusted workstation:
   ```bash
   curl -s -X POST https://pulse.example.com/api/security/recovery \
     -H 'Content-Type: application/json' \
     -H 'X-Recovery-Token: YOUR_TOKEN' \
     -d '{"action":"disable_auth"}'
   ```
   The token is single-use and expires automatically.

3. **Log in and reset credentials** using Settings → Security, then re-enable auth:
   ```bash
   curl -s -X POST http://localhost:7655/api/security/recovery \
     -H 'Content-Type: application/json' \
     -d '{"action":"enable_auth"}'
   ```
   Alternatively, delete `/etc/pulse/.auth_recovery` (or `/data/.auth_recovery` for Docker) and restart Pulse.

Only fall back to nuking `/etc/pulse` if the recovery endpoint is unreachable.

**Prevention:**
- Use a password manager
- Store exported configuration backups securely
- Generate API tokens for automation instead of sharing passwords

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

### Port change didn't take effect
1. **Confirm which service name is active**
   ```bash
   systemctl status pulse 2>/dev/null \\
     || systemctl status pulse-backend 2>/dev/null \\
     || systemctl status pulse-hot-dev
   ```
   - Docker: the container port mapping controls the public port (`-p host:7655`).
   - Kubernetes (Helm chart): service is `svc/pulse`; update `service.port` in your values file and run `helm upgrade`.

2. **Verify configuration/environment overrides**
   ```bash
   sudo systemctl show pulse --property=Environment
   ```
   Helm users can run `kubectl get svc pulse -n <namespace> -o yaml` to confirm the current port.

3. **Check for port conflicts**
   ```bash
   sudo lsof -i :8080
   ```

4. **Post-change validation**
   - Restart the service (`systemctl restart`, `docker restart`, or `helm upgrade`).
   - v4.24.0 logs these restarts/upgrades in **Settings → System → Updates** and `/api/updates/history`; capture the `event_id` for your change notes.

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
- **Default systemd install**: `pulse`
- **Legacy installs (pre-v4.7)**: `pulse-backend`
- **Hot dev environment**: `pulse-hot-dev`
- **Docker**: N/A (container name)

To check which you have:
```bash
systemctl status pulse 2>/dev/null \
  || systemctl status pulse-backend 2>/dev/null \
  || systemctl status pulse-hot-dev
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
- Check logs for response codes (temporarily set `LOG_LEVEL=debug` via **Settings → System → Logging** or export `LOG_LEVEL=debug` and restart; review `webhook.delivery` entries, then revert to `info`)

### HTTPS/TLS Configuration Issues

#### Pulse fails to start after enabling HTTPS
**Symptoms**: Service exits with status code 1, continuous restart attempts, or permission denied errors in logs

**Common causes and solutions:**

1. **Certificate files not readable by pulse user** (most common)
   - Pulse runs as the `pulse` user and needs read access to certificate files
   - Check file ownership: `ls -l /etc/pulse/*.pem`
   - Solution:
     ```bash
     sudo chown pulse:pulse /etc/pulse/cert.pem /etc/pulse/key.pem
     sudo chmod 644 /etc/pulse/cert.pem   # Certificate
     sudo chmod 600 /etc/pulse/key.pem    # Private key
     ```

2. **Invalid certificate or key file**
   - Verify certificate format: `openssl x509 -in /etc/pulse/cert.pem -text -noout`
   - Verify private key: `openssl rsa -in /etc/pulse/key.pem -check -noout`
   - Ensure certificate and key match:
     ```bash
     openssl x509 -noout -modulus -in /etc/pulse/cert.pem | openssl md5
     openssl rsa -noout -modulus -in /etc/pulse/key.pem | openssl md5
     # Both should output the same hash
     ```

3. **File paths incorrect**
   - Verify paths in environment variables match actual file locations
   - Check for typos in `TLS_CERT_FILE` and `TLS_KEY_FILE`
   - Use absolute paths, not relative paths

4. **Check startup logs**
   ```bash
   # View recent service logs
   journalctl -u pulse -n 50

   # Follow logs in real-time during restart
   journalctl -u pulse -f
   ```

**Complete HTTPS setup example:**
```bash
# 1. Place certificate files
sudo cp mycert.pem /etc/pulse/cert.pem
sudo cp mykey.pem /etc/pulse/key.pem

# 2. Set ownership and permissions
sudo chown pulse:pulse /etc/pulse/cert.pem /etc/pulse/key.pem
sudo chmod 644 /etc/pulse/cert.pem
sudo chmod 600 /etc/pulse/key.pem

# 3. Configure systemd service
sudo systemctl edit pulse
# Add:
# [Service]
# Environment="HTTPS_ENABLED=true"
# Environment="TLS_CERT_FILE=/etc/pulse/cert.pem"
# Environment="TLS_KEY_FILE=/etc/pulse/key.pem"

# 4. Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart pulse

# 5. Verify service started successfully
sudo systemctl status pulse
```

See [Configuration Guide](CONFIGURATION.md#tlshttps-configuration) for complete HTTPS setup documentation.

### Temperature Monitoring Issues

#### Temperature data flickers after adding nodes

**Symptoms:** Dashboard temperatures alternate between values and `--`, or new nodes never show readings. Proxy logs contain `limiter.rejection` messages.

**Diagnosis:**
1. Confirm you are running a build with commit 46b8b8d or later (defaults are 1 rps, burst 5). Older binaries throttle multi-node clusters aggressively.
2. Check limiter metrics:
   ```bash
   curl -s http://127.0.0.1:9127/metrics \
     | grep -E 'pulse_proxy_limiter_(rejects|penalties)_total'
   ```
   Any recent increment indicates rate-limit saturation.
3. Inspect scheduler health for temperature pollers (`breaker.state` should be `closed` and `deadLetter.present` must be `false`).

**Fix:** Increase the proxy burst/interval in `/etc/pulse-sensor-proxy/config.yaml`:
```yaml
rate_limit:
  per_peer_interval_ms: 500   # medium cluster (≈10 nodes)
  per_peer_burst: 10
```
Restart `pulse-sensor-proxy`, verify limiter counters stop increasing, and confirm the dashboard stabilises. Document the change in your operations log.

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

### Ceph Cluster Data Missing

**Symptoms**: Ceph pools or health section missing in Storage view even though the cluster uses Ceph.

**Checklist:**
1. Confirm the Proxmox node exposes Ceph-backed storage (`Datacenter → Storage`). Types must be `rbd`, `cephfs`, or `ceph`.
2. Ensure Pulse has permission to call `/cluster/ceph/status` (Pulse’s Proxmox account needs `Sys.Audit` as part of `PVEAuditor`, provided by the setup script).
3. Check the backend logs for `Ceph status unavailable – preserving previous Ceph state`. Intermittent errors are usually network timeouts; steady errors point to permissions.
4. Run from the Pulse host:
   ```bash
   curl -sk https://pve-node:8006/api2/json/cluster/ceph/status \
     -H "Authorization: PVEAPIToken=pulse-monitor@pam!token=<value>"
   ```
   If this fails, verify firewall / token scope.

**Tip**: Pulse polls Ceph after storage refresh. If you recently added Ceph storage, wait one poll cycle or restart the backend to force detection.

### Backup View Filters Not Working

**Symptoms**: Backup chart does not highlight the selected time range or the grid ignores the picker.

**Checklist:**
1. Make sure you are running Pulse v4.29.0 or newer (the interactive picker was introduced alongside the new timeline). Check **Settings → System → About**.
2. Verify your browser is not forcing Legacy mode – if the top-right toggle shows “Lightweight UI”, switch back to default.
3. When filters appear stuck:
   - Click **Reset Filters** in the toolbar.
   - Clear any search chips under the chart.
   - Pick a preset (24h / 7d / 30d) to re-seed the view, then move back to Custom.
4. If the grid still shows stale data, open DevTools console and ensure no errors mentioning `chartsSelection` appear. Any error here usually means a stale service worker; hard refresh (Ctrl+Shift+R) clears it.

**Tip**: Selecting bars in the chart cross-highlights matching rows. If that does not happen, confirm you do not have browser extensions that block pointer events on canvas elements.

### Container Agent Shows Hosts Offline

**Symptoms**: `/containers` tab marks hosts as offline or missing container metrics.

**Checklist:**
1. Run the agent manually with verbose logs:
   ```bash
   sudo /usr/local/bin/pulse-docker-agent --interval 15s --debug
   ```
   Look for HTTP 401 (token mismatch) or socket errors.
2. Confirm the host sees Docker:
   ```bash
   sudo docker info | head -n 20
   ```
3. Make sure the agent ID is stable. If running inside transient containers, set `--agent-id` explicitly so Pulse does not treat each restart as a new host.
4. Verify Pulse shows a recent heartbeat (`lastSeen`) in `/api/state` → `dockerHosts`. Hosts are marked offline after 4× the configured interval with no update.
5. For reverse proxies/TLS issues, append `--insecure` temporarily to confirm whether certificate validation is the culprit.

**Restart loops**: The Containers workspace Issues column lists the last exit codes. Investigate recurring non-zero codes in `docker logs <container>` and adjust restart policy if needed.

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
- **Proxmox 9:** `VM.GuestAgent.Audit` privilege (Pulse setup adds this via the `PulseMonitor` role)
- **Proxmox 8:** `VM.Monitor` privilege (Pulse setup adds this via the `PulseMonitor` role)
- **All versions:** `Sys.Audit` is recommended for Ceph metrics and applied when available

**Fix permissions:**

Re-run the Pulse setup script on the Proxmox node:
```bash
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/setup-pve.sh | bash
```

Or manually:
```bash
# Shared read-only access
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor

# Extra privileges for guest metrics and Ceph
EXTRA_PRIVS=()

# Sys.Audit (Ceph, cluster status)
if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
  EXTRA_PRIVS+=(Sys.Audit)
else
  if pveum role add PulseTmpSysAudit -privs Sys.Audit 2>/dev/null; then
    EXTRA_PRIVS+=(Sys.Audit)
    pveum role delete PulseTmpSysAudit 2>/dev/null
  fi
fi

# VM guest agent / monitor privileges
VM_PRIV=""
if pveum role list 2>/dev/null | grep -q "VM.Monitor"; then
  VM_PRIV="VM.Monitor"
elif pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
  VM_PRIV="VM.GuestAgent.Audit"
else
  if pveum role add PulseTmpVMMonitor -privs VM.Monitor 2>/dev/null; then
    VM_PRIV="VM.Monitor"
    pveum role delete PulseTmpVMMonitor 2>/dev/null
  elif pveum role add PulseTmpGuestAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
    VM_PRIV="VM.GuestAgent.Audit"
    pveum role delete PulseTmpGuestAudit 2>/dev/null
  fi
fi

if [ -n "$VM_PRIV" ]; then
  EXTRA_PRIVS+=("$VM_PRIV")
fi

if [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then
  PRIV_STRING="${EXTRA_PRIVS[*]}"
  pveum role delete PulseMonitor 2>/dev/null
  pveum role add PulseMonitor -privs "$PRIV_STRING"
  pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
fi
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
