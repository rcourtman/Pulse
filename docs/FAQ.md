# FAQ

## Installation

### What's the easiest way to install?
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

### System requirements?
- 1 vCPU, 512MB RAM (1GB recommended), 1GB disk
- Network access to Proxmox API

## Configuration

### How do I add a node?
**Auto-discovery (Easiest)**: Settings → Nodes → Click "Setup Script" on discovered node → Run on Proxmox
**Manual**: Settings → Nodes → Add Node → Enter credentials → Save

![Node Configuration](images/06-settings.png)

### How do I disable network discovery?
Settings → System → Network Settings → Toggle "Enable Discovery" off → Save
Or set environment variable `DISCOVERY_ENABLED=false`

### How do I change the port?
Systemd: `sudo systemctl edit pulse`, add `Environment="FRONTEND_PORT=8080"`, restart
Docker: Use `-e FRONTEND_PORT=8080 -p 8080:8080` in your run command
See [Port Configuration Guide](PORT_CONFIGURATION.md) for details

### Why can't I change settings in the UI?
If a setting is disabled with an amber warning, it's being overridden by an environment variable. 
Remove the env var (check `sudo systemctl show pulse | grep Environment`) and restart to enable UI configuration.

### What permissions needed?
- PVE core API access: `PVEAuditor`
- PVE guest metrics: `VM.GuestAgent.Audit` (PVE 9+) or `VM.Monitor` (PVE 8) plus `Sys.Audit` for Ceph — Pulse setup script adds these to the `PulseMonitor` role automatically
- PBS: `DatastoreReader` minimum

### API tokens vs passwords?
API tokens are more secure. Create in Proxmox: Datacenter → Permissions → API Tokens

### Where are settings stored?
See [Configuration Guide](CONFIGURATION.md) for details

### How do I backup my configuration?
Settings → Security → Backup & Restore → Export Backup
- If logged in with password: Just enter your password or a custom passphrase
- If using API token only: Provide the API token when prompted
- Includes all settings, nodes, credentials (encrypted), and custom console URLs

### Can I filter backup history or focus on a specific time window?
Yes. The **Backups** workspace exposes a time-range picker above the chart (Last 24 h / 7 d / 30 d / Custom). Selecting a range reflows the chart, highlights matching bars, and filters the grid below. Hovering the chart shows tooltips with the top jobs inside that window so you can jump directly to a backup task or snapshot.
Trouble with the picker? See [Troubleshooting → Backup View Filters Not Working](TROUBLESHOOTING.md#backup-view-filters-not-working).

### Can Pulse detect Proxmox clusters?
Yes! When you add one cluster node, Pulse automatically discovers and monitors all nodes

## Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006/8007)
- Verify credentials
- Check logs: `journalctl -u pulse -f`

### Connection refused?
- Check port 7655 is open
- Verify Pulse is running: `systemctl status pulse`

### PBS connection issues?
- PBS requires HTTPS (not HTTP) - use `https://your-pbs:8007`
- Default PBS port is 8007 (not 8006)
- Check firewall allows port 8007

### Invalid credentials?
- Check username includes realm (@pam, @pve)
- Verify API token not expired
- Confirm user has required permissions

### CORS errors in browser?
- By default, Pulse only allows same-origin requests
- Set `ALLOWED_ORIGINS` environment variable for cross-origin access
- Example: `ALLOWED_ORIGINS=https://app.example.com`
- Never use `*` in production

### Authentication issues?
- Password auth: Check `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` environment variables
- API tokens: Ensure `API_TOKENS` includes an active credential (or `API_TOKEN` for legacy setups)
- Session expired: Log in again via web UI
- Account locked: Wait 15 minutes after 5 failed attempts

### High memory usage?
Reduce `metricsRetentionDays` in settings and restart

### How do I monitor adaptive polling?
**New in v4.24.0:** Pulse includes adaptive polling that automatically adjusts polling intervals based on system load.

**Monitor adaptive polling:**
- **Dashboard**: Settings → System → Monitoring shows scheduler health status
- **API**: `/api/monitoring/scheduler/health` provides detailed metrics including:
  - Queue depths and processing times
  - Circuit breaker status
  - Backoff states
  - Instance metadata
- **Logging**: Enable debug logging to see detailed polling behavior

**Key metrics to watch:**
- Queue depth (alerts if backlog builds up)
- Circuit breaker trips (indicates connectivity issues)
- Backoff delays (shows throttling behavior)

See [Adaptive Polling Documentation](monitoring/ADAPTIVE_POLLING.md) for complete details.

### What's new about rate limiting in v4.24.0?
Pulse now returns standard rate limit headers with all API responses:

**Response Headers:**
- `X-RateLimit-Limit`: Maximum requests allowed per window (e.g., 500)
- `X-RateLimit-Remaining`: Requests remaining in current window
- `Retry-After`: Seconds to wait before retrying (on 429 responses)

**Rate Limits:**
- **Auth endpoints**: 10 attempts/minute per IP
- **General API**: 500 requests/minute per IP
- **Real-time endpoints**: No limits (WebSocket, SSE)

**Example Response:**
```
HTTP/1.1 200 OK
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 487
```

When you hit the limit:
```
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 0
Retry-After: 60
```

## Features

### Why do VMs show "-" for disk usage?

VMs show "-" because the QEMU Guest Agent is not installed or not working. This is normal and expected.

**How VM disk monitoring works:**
- Proxmox API always returns `disk=0` for VMs (this is normal, not a bug)
- To get real disk usage, Pulse queries the QEMU Guest Agent inside each VM
- Both API tokens and passwords work fine for this (no authentication method limitation)
- If guest agent is missing or not responding, Pulse shows "-" with a tooltip explaining why

**To get VM disk usage showing:**

1. **Install QEMU Guest Agent in the VM:**
   - Linux: `apt install qemu-guest-agent && systemctl enable --now qemu-guest-agent`
   - Windows: Install virtio-win guest tools

2. **Enable in VM config:**
   - Proxmox UI: VM → Options → QEMU Guest Agent → Enable
   - Or CLI: `qm set <VMID> --agent enabled=1`

3. **Restart the VM** for changes to take effect

4. **Verify it works:**
   ```bash
   qm agent <VMID> ping
   qm agent <VMID> get-fsinfo
   ```

5. **Check Pulse has permissions:**
   - Proxmox 9: `VM.GuestAgent.Audit` privilege (Pulse setup adds via `PulseMonitor`)
   - Proxmox 8: `VM.Monitor` privilege (Pulse setup adds via `PulseMonitor`)
   - `Sys.Audit` is recommended for Ceph metrics and included when available
   - The setup script applies all of the above automatically

**Note:** Container (LXC) disk usage always works without guest agent because containers share the host kernel.

**Still not working?** See [Troubleshooting Guide - VM Disk Monitoring](TROUBLESHOOTING.md#vm-disk-monitoring-issues) for detailed diagnostics.

### How do I see real disk usage for VMs?
See the previous question "Why do VMs show '-' for disk usage?" or the [VM Disk Monitoring Guide](VM_DISK_MONITORING.md) for full details.

### Multiple clusters?
Yes, add multiple nodes in Settings

### PBS push mode?
No, PBS push mode is not currently supported. PBS monitoring requires network connectivity from Pulse to the PBS server.

### Webhook providers?
Discord, Slack, Gotify, Telegram, ntfy.sh, Teams, generic JSON

### Works with reverse proxy?
Yes, ensure WebSocket support is enabled

### How do I disable alerts for specific metrics?
Go to **Alerts → Thresholds**, then set any threshold to `-1` to disable alerts for that metric.

**Examples:**
- Don't care about disk I/O alerts? Set "Disk R MB/s" and "Disk W MB/s" to `-1`
- Want to ignore network alerts on a specific VM? Set "Net In MB/s" and "Net Out MB/s" to `-1`
- Need to disable CPU alerts for a maintenance node? Set "CPU %" to `-1`

**To re-enable:** Click on any disabled threshold showing "Off" and it will restore to a default value. The trash icon beside **Global Defaults** resets that row instantly, and the search bar at the top of the tab filters resources live.

**Per-resource customization:** You can disable metrics globally (affects all resources) or individually (just one VM, container, node, etc.). Resources with custom settings show a blue "Custom" badge so you can spot overrides quickly.

### Can I set fractional thresholds or specify different trigger/clear values?
Yes. Pulse stores hysteresis thresholds in pairs: `trigger` (when to fire) and `clear` (when to recover). Both values accept decimal precision – for example, set network thresholds to `12.5` / `9.5` MB/s. The UI shows the trigger value in the table and reveals the clear threshold in the sidebar drawer.

### How do I interpret the alert timeline graph?
Open **Alerts → History** and click an entry. The right-hand panel now shows a context timeline that plots alert start, acknowledgement, clearance, and any escalations so you can see at a glance how long the condition lasted and when notifications were sent. Hovering each marker reveals the exact timestamp and value Pulse captured at that step.

### Does Pulse monitor Ceph clusters?
Yes. When Ceph-backed storage (RBD or CephFS) is detected, Pulse queries `/cluster/ceph/status` and `/cluster/ceph/df` and surfaces the results on the **Storage → Ceph** drawer and via `/api/state` → `cephClusters`. You get cluster health, daemon counts, placement groups, and per-pool capacity without any additional configuration.
If those sections stay empty, follow [Troubleshooting → Ceph Cluster Data Missing](TROUBLESHOOTING.md#ceph-cluster-data-missing).

### Why does a Docker host show as offline in the Docker tab?
First, confirm the agent is still running (`systemctl status pulse-docker-agent` or `docker ps`). If it is, check the Issues column for restart-loop notes and verify the host’s last heartbeat under **Details**. Still stuck? Walk through [Troubleshooting → Docker Agent Shows Hosts Offline](TROUBLESHOOTING.md#docker-agent-shows-hosts-offline) for a step-by-step checklist.

## Updates

### How to update?
- **Docker**: Pull latest image, recreate container
- **Manual/systemd**: Run the install script again: `curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash`

### Can I roll back if an update misbehaves?
**New in v4.24.0:** Yes! Pulse now retains previous versions and provides easy rollback.

**Via UI (Recommended):**
1. Navigate to **Settings → System → Updates**
2. Click **"Restore previous version"**
3. Confirm rollback
4. Pulse restarts with the previous working version

**Via CLI:**
```bash
# Systemd installations
sudo /opt/pulse/pulse config rollback

# LXC containers
pct exec <ctid> -- bash -c "cd /opt/pulse && ./pulse config rollback"
```

**What gets rolled back:**
- Pulse binary and frontend assets
- System configuration (preserved from previous version)
- Rollback history tracked in Updates view

**What stays the same:**
- Your node configurations
- Alert settings
- User credentials
- Historical metrics data

Check rollback logs: `journalctl -u pulse | grep rollback`

### How do I install an older release (downgrade)?
- **Manual/systemd installs**: rerun the installer and pass the tag you want, e.g. `curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.24.0`
- **Proxmox LXC appliance**: `pct exec <ctid> -- bash -lc "curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.24.0"`
- **Docker**: launch with a versioned tag instead of `latest`, e.g. `docker run -d --name pulse -p 7655:7655 rcourtman/pulse:v4.24.0`

### How do I adjust logging without restarting?
**New in v4.24.0:** Pulse supports runtime logging configuration—no restart required!

**Via UI:**
1. Navigate to **Settings → System → Logging**
2. Adjust:
   - **Log Level**: debug, info, warn, error
   - **Log Format**: json, text
   - **File Rotation**: size limits, retention
3. Changes apply immediately

**Via Environment Variables:**
```bash
# Systemd
sudo systemctl edit pulse
[Service]
Environment="LOG_LEVEL=debug"
Environment="LOG_FORMAT=json"

# Docker
docker run -e LOG_LEVEL=debug -e LOG_FORMAT=json rcourtman/pulse:latest
```

**Use cases:**
- Enable debug logging temporarily for troubleshooting
- Switch to JSON format for log aggregation
- Adjust file rotation to manage disk usage

### Why can't I update from the UI?
For security reasons, Pulse cannot self-update. The UI will notify you when updates are available and show the appropriate update command for your deployment type.

### Will updates break config?
No, configuration is preserved
