# 🌡️ Temperature Monitoring

Monitor real-time CPU and NVMe temperatures for your Proxmox nodes.

## 🚀 Quick Start

### 1. Enable in Pulse
Go to **Settings → Nodes → [Node] → Advanced Monitoring** and enable "Temperature Monitoring".

### 2. Install Sensor Proxy
The setup depends on your deployment:

| Deployment | Recommended Method |
| :--- | :--- |
| **LXC (Pulse)** | Run the **Setup Script** in Pulse UI. It auto-installs the proxy on the host. |
| **Docker (Pulse)** | Install proxy on host + bind mount socket. (See below) |
| **Remote Node** | Install proxy in **HTTP Mode** on the remote node. |

## 📦 Docker Setup (Manual)

If running Pulse in Docker, you must install the proxy on the host and share the socket.

1.  **Install Proxy on Host**:
    ```bash
    curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
      sudo bash -s -- --standalone --pulse-server http://<pulse-ip>:7655
    ```

2.  **Update `docker-compose.yml`**:
    Add the socket volume to your Pulse service:
    ```yaml
    volumes:
      - /mnt/pulse-proxy:/run/pulse-sensor-proxy:ro
    ```
    > **Note**: The standalone installer creates the socket at `/mnt/pulse-proxy` on the host. Map it to `/run/pulse-sensor-proxy` inside the container.

3.  **Restart Pulse**: `docker compose up -d`

## 🌐 Multi-Server Proxmox Setup

If you have Pulse running on **Server A** and want to monitor temperatures on **Server B** (a separate Proxmox host without Pulse):

1.  **Run Installer on Server B** (the remote Proxmox host):
    ```bash
    curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
      sudo bash -s -- --ctid <PULSE_CONTAINER_ID> --pulse-server http://<pulse-ip>:7655
    ```
    Replace `<PULSE_CONTAINER_ID>` with the LXC container ID where Pulse runs on Server A (e.g., `100`).

2.  The installer will detect that the container doesn't exist locally and install in **host monitoring only** mode:
    ```
    [WARN] Container 100 does not exist on this node
    [WARN] Will install sensor-proxy for host temperature monitoring only
    ```

3.  **Verify**: `systemctl status pulse-sensor-proxy`

> **Note**: The `--standalone --http-mode` flags shown in the Pulse UI quick-setup are for Docker deployments, not bare Proxmox hosts. For multi-server Proxmox setups, use the `--ctid` approach above.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **No Data** | Check **Settings → Diagnostics → Temperature Proxy**. |
| **Proxy Unreachable** | Ensure port `8443` is open on the remote node. |
| **"Permission Denied"** | Re-run the installer to fix permissions or SSH keys. |
| **LXC Issues** | Ensure the container has the bind mount: `lxc.mount.entry: /run/pulse-sensor-proxy ...` |

### Check Proxy Status
On the Proxmox host:
```bash
systemctl status pulse-sensor-proxy
```

### View Logs
```bash
journalctl -u pulse-sensor-proxy -f
```

## 🧠 How It Works

1.  **Pulse Sensor Proxy**: A lightweight service runs on the Proxmox host.
2.  **Secure Access**: It reads sensors (via `lm-sensors`) and exposes them securely.
3.  **Transport**:
    *   **Local**: Uses a Unix socket (`/run/pulse-sensor-proxy`) for zero-latency, secure access.
    *   **Remote**: Uses mutual TLS over HTTPS (port 8443).
4.  **No SSH Keys**: Pulse containers no longer need SSH keys to read temperatures.

---

## 🔧 Advanced Configuration

#### Manual Configuration (No Script)

If you can't run the installer script, create the configuration manually:

**1. Download binary:**
```bash
curl -L https://github.com/rcourtman/Pulse/releases/latest/download/pulse-sensor-proxy-linux-amd64 \
  -o /tmp/pulse-sensor-proxy
install -D -m 0755 /tmp/pulse-sensor-proxy /opt/pulse/sensor-proxy/bin/pulse-sensor-proxy
```

**2. Create service user:**
```bash
useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
usermod -aG www-data pulse-sensor-proxy  # For pvecm access
```

**3. Create directories:**
```bash
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 /var/lib/pulse-sensor-proxy/ssh
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0755 /etc/pulse-sensor-proxy
```

**4. Create config (optional, for Docker):**
```yaml
# /etc/pulse-sensor-proxy/config.yaml
allowed_nodes_file: /etc/pulse-sensor-proxy/allowed_nodes.yaml
allowed_peer_uids: [1000]  # Docker container UID
allow_idmapped_root: true
allowed_idmap_users:
  - root
```
Allowed nodes live in `/etc/pulse-sensor-proxy/allowed_nodes.yaml`; change them via `pulse-sensor-proxy config set-allowed-nodes` so the proxy can lock and validate the file safely. Control-plane settings are added automatically when you register via Pulse, but you can supply them manually if you cannot reach the API (`pulse_control_plane.url`, `.token_file`, `.refresh_interval`).

**5. Install systemd service:**
```bash
# Download from: https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh
# Extract the systemd unit from the installer (ExecStartPre/ExecStart use /opt/pulse/sensor-proxy/bin)
systemctl daemon-reload
systemctl enable --now pulse-sensor-proxy
```

**6. Verify:**
```bash
systemctl status pulse-sensor-proxy
ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock
```

#### Configuration File Format

The proxy reads `/etc/pulse-sensor-proxy/config.yaml` plus an allow-list in `/etc/pulse-sensor-proxy/allowed_nodes.yaml`:

```yaml
allowed_source_subnets:
  - 192.168.1.0/24
  - 10.0.0.0/8

# Capability-based access control (legacy UID/GID lists still work)
allowed_peers:
  - uid: 0
    capabilities: [read, write, admin]
  - uid: 1000
    capabilities: [read]
allowed_peer_uids: []
allowed_peer_gids: []
allow_idmapped_root: true
allowed_idmap_users:
  - root

log_level: info
metrics_address: default
read_timeout: 5s
write_timeout: 10s
max_ssh_output_bytes: 1048576
require_proxmox_hostkeys: false

# Allow list persistence (managed by installer/control-plane/CLI)
allowed_nodes_file: /etc/pulse-sensor-proxy/allowed_nodes.yaml
strict_node_validation: false

# Rate limiting (per calling UID)
rate_limit:
  per_peer_interval_ms: 1000
  per_peer_burst: 5

# HTTPS mode (for remote nodes)
http_enabled: false
http_listen_addr: ":8443"
http_tls_cert: /etc/pulse-sensor-proxy/tls/server.crt
http_tls_key: /etc/pulse-sensor-proxy/tls/server.key
http_auth_token: ""  # Populated during registration

# Control-plane sync (keeps allowed_nodes.yaml updated)
pulse_control_plane:
  url: https://pulse.example.com:7655
  token_file: /etc/pulse-sensor-proxy/.pulse-control-token
  refresh_interval: 60
  insecure_skip_verify: false
```

`allowed_nodes.yaml` is the source of truth for valid nodes. Avoid editing it directly—use `pulse-sensor-proxy config set-allowed-nodes` so the proxy can lock, dedupe, and write atomically. `allowed_peers` scopes socket access; legacy UID/GID lists remain for backward compatibility and imply full capabilities.

**Environment Variable Overrides:**

Config values can also be set via environment variables (useful for containerized proxy deployments):

```bash
# Add allowed subnets (comma-separated, appends to config file values)
PULSE_SENSOR_PROXY_ALLOWED_SUBNETS=192.168.1.0/24,10.0.0.0/8

# Allow/disallow ID-mapped root (overrides config file)
PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT=true

# HTTP listener controls
PULSE_SENSOR_PROXY_HTTP_ENABLED=true
PULSE_SENSOR_PROXY_HTTP_ADDR=":8443"
PULSE_SENSOR_PROXY_HTTP_TLS_CERT=/etc/pulse-sensor-proxy/tls/server.crt
PULSE_SENSOR_PROXY_HTTP_TLS_KEY=/etc/pulse-sensor-proxy/tls/server.key
PULSE_SENSOR_PROXY_HTTP_AUTH_TOKEN="$(cat /etc/pulse-sensor-proxy/.http-auth-token)"
```
Additional overrides include `PULSE_SENSOR_PROXY_ALLOWED_PEER_UIDS`, `PULSE_SENSOR_PROXY_ALLOWED_PEER_GIDS`, `PULSE_SENSOR_PROXY_ALLOWED_NODES`, `PULSE_SENSOR_PROXY_READ_TIMEOUT`, `PULSE_SENSOR_PROXY_WRITE_TIMEOUT`, `PULSE_SENSOR_PROXY_METRICS_ADDR`, and `PULSE_SENSOR_PROXY_STRICT_NODE_VALIDATION`.

Example systemd override:
```ini
# /etc/systemd/system/pulse-sensor-proxy.service.d/override.conf
[Service]
Environment="PULSE_SENSOR_PROXY_ALLOWED_SUBNETS=192.168.1.0/24"
```

**Note:** Socket path, SSH key directory, and audit log path are configured via command-line flags (see main.go), not the YAML config file.

#### Re-running After Changes

The installer is idempotent and safe to re-run:

```bash
# After adding a new Proxmox node to cluster
bash install-sensor-proxy.sh --standalone --pulse-server http://pulse:7655 --quiet

# After upgrading Pulse version
bash install-sensor-proxy.sh --standalone --pulse-server http://pulse:7655 --version v4.27.0 --quiet

# Verify installation
systemctl status pulse-sensor-proxy
```

### Legacy Security Concerns (Pre-v4.24.0)

Older versions stored SSH keys inside the container, creating security risks:

- Compromised container = exposed SSH keys
- Even with forced commands, keys could be extracted
- Required manual hardening (key rotation, IP restrictions, etc.)

### Hardening Recommendations (Legacy/Native Installs Only)

#### 1. Key Rotation
Rotate SSH keys periodically (e.g., every 90 days):

```bash
# On Pulse server
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_new -N ""

# Update all nodes' authorized_keys
# Test connectivity
ssh -i ~/.ssh/id_ed25519_new node "sensors -j"

# Replace old key
mv ~/.ssh/id_ed25519_new ~/.ssh/id_ed25519
```

#### 2. Secret Mounts (Docker)
Mount SSH keys from secure volumes:

```yaml
version: '3'
services:
  pulse:
    image: rcourtman/pulse:latest
    volumes:
      - pulse-ssh-keys:/home/pulse/.ssh:ro  # Read-only
      - pulse-data:/data
volumes:
  pulse-ssh-keys:
    driver: local
    driver_opts:
      type: tmpfs  # Memory-only, not persisted
      device: tmpfs
```

#### 3. Monitoring & Alerts
Enable SSH audit logging on Proxmox nodes:

```bash
# Install auditd
apt-get install auditd

# Watch SSH access
auditctl -w /root/.ssh -p wa -k ssh_access

# Monitor for unexpected commands
tail -f /var/log/audit/audit.log | grep ssh
```

#### 4. IP Restrictions
Limit SSH access to your Pulse server IP in `/etc/ssh/sshd_config`:

```ssh
Match User root Address 192.168.1.100
    ForceCommand sensors -j
    PermitOpen none
    AllowAgentForwarding no
    AllowTcpForwarding no
```

### Verifying Proxy Installation

To check if your deployment is using the secure proxy:

```bash
# On Proxmox host - check proxy service
systemctl status pulse-sensor-proxy

# Check if socket exists
ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# View proxy logs
journalctl -u pulse-sensor-proxy -f
```

Forward these logs off-host for retention by following
[operations/SENSOR_PROXY_LOGS.md](operations/SENSOR_PROXY_LOGS.md).

In the Pulse container, check the logs at startup:
```bash
# Should see: "Temperature proxy detected - using secure host-side bridge"
journalctl -u pulse | grep -i proxy
```

### Disabling Temperature Monitoring

To remove SSH access:

```bash
# On each Proxmox node
sed -i '/pulse@/d' /root/.ssh/authorized_keys

# Or remove just the forced command entry
sed -i '/command="sensors -j"/d' /root/.ssh/authorized_keys
```

Temperature data will stop appearing in the dashboard after the next polling cycle.

## Operations & Troubleshooting

### Managing the Proxy Service

The pulse-sensor-proxy service runs on the Proxmox host (outside the container).

**Service Management:**
```bash
# Check service status
systemctl status pulse-sensor-proxy

# Restart the proxy
systemctl restart pulse-sensor-proxy

# Stop the proxy (disables temperature monitoring)
systemctl stop pulse-sensor-proxy

# Start the proxy
systemctl start pulse-sensor-proxy

# Enable proxy to start on boot
systemctl enable pulse-sensor-proxy

# Disable proxy autostart
systemctl disable pulse-sensor-proxy
```

### Log Locations

**Proxy Logs (on Proxmox host):**
```bash
# Follow proxy logs in real-time
journalctl -u pulse-sensor-proxy -f

# View last 50 lines
journalctl -u pulse-sensor-proxy -n 50

# View logs since last boot
journalctl -u pulse-sensor-proxy -b

# View logs with timestamps
journalctl -u pulse-sensor-proxy --since "1 hour ago"
```

**Pulse Logs (in container):**
```bash
# Check if proxy is being used
journalctl -u pulse | grep -i "proxy\|temperature"

# Should see: "Temperature proxy detected - using secure host-side bridge"
```

### SSH Key Rotation

Rotate SSH keys periodically for security (recommended every 90 days).

**Automated Rotation (Recommended):**

The `/opt/pulse/scripts/pulse-proxy-rotate-keys.sh` script handles rotation safely with staging, verification, and rollback support:

```bash
# 1. Dry-run first (recommended)
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh --dry-run

# 2. Perform rotation
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh
```

**What the script does:**
- Generates new Ed25519 keypair in staging directory
- Pushes new key to all cluster nodes via proxy RPC
- Verifies SSH connectivity with new key on each node
- Atomically swaps keys (current → backup, staging → active)
- Preserves old keys for rollback

**If rotation fails, rollback:**
```bash
sudo /opt/pulse/scripts/pulse-proxy-rotate-keys.sh --rollback
```

**Manual Rotation (Fallback):**

If the automated script fails or is unavailable:

```bash
# 1. On Proxmox host, backup old keys
cd /var/lib/pulse-sensor-proxy/ssh/
cp id_ed25519 id_ed25519.backup
cp id_ed25519.pub id_ed25519.pub.backup

# 2. Generate new keypair
ssh-keygen -t ed25519 -f id_ed25519 -N "" -C "pulse-sensor-proxy-rotated"

# 3. Re-run setup to push keys to cluster
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
  bash -s -- --ctid <your-container-id>

# 4. Verify temperature data still works in Pulse UI
```

### Automatic Cleanup When Nodes Are Removed (v4.26.0+)

Starting in v4.26.0, SSH keys are **automatically removed** when you delete a node from Pulse:

1. **When you remove a node** in Pulse Settings → Nodes, Pulse signals the temperature proxy
2. **The proxy creates a cleanup request** file at `/var/lib/pulse-sensor-proxy/cleanup-request.json`
3. **A systemd path unit detects the request** and triggers the cleanup service
4. **The cleanup script automatically:**
   - SSHs to the specified node (or localhost if it's local)
   - Removes the SSH key entries (`# pulse-managed-key` and `# pulse-proxy-key`)
   - Logs the cleanup action via syslog

**Automatic cleanup works for:**
- ✅ **Cluster nodes** - Full automatic cleanup (Proxmox clusters have unrestricted passwordless SSH)
- ⚠️ **Standalone nodes** - Cannot auto-cleanup due to forced command security (see below)

**Standalone Node Limitation:**

Standalone nodes use forced commands (`command="sensors -j"`) for security. This same restriction prevents the cleanup script from running `sed` to remove keys. This is a **security feature, not a bug** - adding a workaround would defeat the forced command protection.

For standalone nodes:
- Keys remain after removal (but they're **read-only** - only `sensors -j` access)
- **Low security risk** - no shell access, no write access, no port forwarding
- **Auto-cleanup on re-add** - Setup script removes old keys when node is re-added
- **Manual cleanup if needed:**
  ```bash
  ssh root@standalone-node "sed -i '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys"
  ```

**Monitoring Cleanup:**
```bash
# Watch cleanup operations in real-time
journalctl -u pulse-sensor-cleanup -f

# View cleanup history
journalctl -u pulse-sensor-cleanup --since "1 week ago"

# Check if cleanup system is active
systemctl status pulse-sensor-cleanup.path
```

**Manual Cleanup (if needed):**

If automatic cleanup fails or you need to manually revoke access:

```bash
# On the node being removed, remove all Pulse SSH keys
ssh root@old-node "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys"

# Or remove them locally
sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys

# No restart needed - proxy will fail gracefully for that node
# Temperature monitoring will continue for remaining nodes
```

### Failure Modes

**Proxy Not Running:**
- Symptom: No temperature data in Pulse UI
- Check: `systemctl status pulse-sensor-proxy` on Proxmox host
- Fix: `systemctl start pulse-sensor-proxy`

**Socket Not Accessible in Container:**
- Symptom: Pulse logs show "Temperature proxy not available - using direct SSH"
- Check: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock` in container
- Fix: Verify bind mount in LXC config (`/etc/pve/lxc/<CTID>.conf`)
- Should have: `lxc.mount.entry: /run/pulse-sensor-proxy run/pulse-sensor-proxy none bind,create=dir 0 0`

**pvecm Not Available:**
- Symptom: Proxy fails to discover cluster nodes
- Cause: Pulse runs on non-Proxmox host
- Fallback: Use legacy direct SSH method (native installation)

**Pulse Running Off-Cluster:**
- Symptom: Proxy discovers local host but not remote cluster nodes
- Limitation: Proxy requires passwordless SSH between cluster nodes
- Solution: Ensure Proxmox host running Pulse has SSH access to all cluster nodes

**Unauthorized Connection Attempts:**
- Symptom: Proxy logs show "Unauthorized connection attempt"
- Cause: Process with non-root UID trying to access socket
- Normal: Only root (UID 0) or proxy's own user can access socket
- Check: Look for suspicious processes trying to access the socket

### Monitoring the Proxy

**Manual Monitoring (v1):**

The proxy service includes systemd restart-on-failure, which handles most issues automatically. For additional monitoring:

```bash
# Check proxy health
systemctl is-active pulse-sensor-proxy && echo "Proxy is running" || echo "Proxy is down"

# Monitor logs for errors
journalctl -u pulse-sensor-proxy --since "1 hour ago" | grep -i error

# Verify socket exists and is accessible
test -S /run/pulse-sensor-proxy/pulse-sensor-proxy.sock && echo "Socket OK" || echo "Socket missing"
```

**Alerting:**
- Rely on systemd's automatic restart (`Restart=on-failure`)
- Monitor via journalctl for persistent failures
- Check Pulse UI for missing temperature data

**Future:** Integration with pulse-watchdog is planned for automated health checks and alerting (see #528).

### Known Limitations

**Single Proxy = Single Point of Failure:**
- Each Proxmox host runs one pulse-sensor-proxy instance
- If the proxy service dies, temperature monitoring stops for all containers on that host
- This is acceptable for read-only telemetry, but be aware of the failure mode
- Systemd auto-restart (`Restart=on-failure`) mitigates most outages
- If multiple Pulse containers run on same host, they share the same proxy

**Sensors Output Parsing Brittleness:**
- Pulse depends on `sensors -j` JSON output format from lm-sensors
- Changes to sensor names, structure, or output format could break parsing
- Consider adding schema validation and instrumentation to detect issues early
- Monitor proxy logs for parsing errors: `journalctl -u pulse-sensor-proxy | grep -i "parse\|error"`

**Cluster Discovery Limitations:**
- Proxy uses `pvecm status` to discover cluster nodes (requires Proxmox IPC access)
- If Proxmox hardens IPC access or cluster topology changes unexpectedly, discovery may fail
- Standalone Proxmox nodes work but only monitor that single node
- Fallback: Re-run setup script manually to reconfigure cluster access

**Rate Limiting & Scaling** (updated in commit 46b8b8d):

**What changed:** pulse-sensor-proxy now defaults to 1 request per second with a burst of 5 per calling UID. Earlier builds throttled after two calls every five seconds, which caused temperature tiles to flicker or fall back to `--` as soon as clusters reached three or more nodes.

**Symptoms of saturation:**
- Temperature widgets flicker between values and `--`, or entire node rows disappear after adding new hardware
- `Settings → System → Updates` shows no proxy restarts, yet scheduler health reports breaker openings for temperature pollers
- Proxy logs include `limiter.rejection` or `Rate limit exceeded` entries for the container UID

**Diagnose:**
1. Check scheduler health for temperature pollers:
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.key | contains("temperature")) \
       | {key, lastSuccess: .pollStatus.lastSuccess, breaker: .breaker.state, deadLetter: .deadLetter.present}'
   ```
   Breakers that remain `open` or repeated dead letters indicate the proxy is rejecting calls.
2. Inspect limiter metrics on the host:
   ```bash
   curl -s http://127.0.0.1:9127/metrics \
     | grep -E 'pulse_proxy_limiter_(rejects|penalties)_total'
   ```
   A rising counter confirms the limiter is backing off callers.
3. Review logs for throttling:
   ```bash
   journalctl -u pulse-sensor-proxy -n 100 | grep -i "rate limit"
   ```

**Tuning guidance:** Add a `rate_limit` block to `/etc/pulse-sensor-proxy/config.yaml` (see `cmd/pulse-sensor-proxy/config.example.yaml`) when clusters grow beyond the defaults. Use the formula `per_peer_interval_ms = polling_interval_ms / node_count` and set `per_peer_burst ≥ node_count` to allow one full sweep per polling window.

| Deployment size | Nodes | 10 s poll interval → interval_ms | Suggested burst | Notes |
| --- | --- | --- | --- | --- |
| Small | 1–3 | 1000 (default) | 5 | Works for most single Proxmox hosts. |
| Medium | 4–10 | 500 | 10 | Halves wait time; keep burst ≥ node count. |
| Large | 10–20 | 250 | 20 | Monitor CPU on proxy; consider staggering polls. |
| XL | 30+ | 100–150 | 30–50 | Only enable after validating proxy host capacity. |

**Security note:** Lower intervals increase throughput and reduce UI staleness, but they also allow untrusted callers to issue more RPCs per second. Keep `per_peer_interval_ms ≥ 100` in production and continue to rely on UID allow-lists plus audit logs when raising limits.

**SSH latency monitoring:**
- Monitor SSH latency metrics: `curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy_ssh_latency`

**Requires Proxmox Cluster Membership:**
- Proxy requires passwordless root SSH between cluster nodes
- Standard for Proxmox clusters, but hardened environments may differ
- Alternative: Create dedicated service account with sudo access to `sensors`

**No Cross-Cluster Support:**
- Proxy only manages the cluster its host belongs to
- Cannot bridge temperature monitoring across multiple disconnected clusters
- Each cluster needs its own Pulse instance with its own proxy

### Common Issues

**Temperature Data Stops Appearing:**
1. Check proxy service: `systemctl status pulse-sensor-proxy`
2. Check proxy logs: `journalctl -u pulse-sensor-proxy -n 50`
3. Test SSH manually: `ssh root@node "sensors -j"`
4. Verify socket exists: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock`

**New Cluster Node Not Showing Temperatures:**
1. Ensure lm-sensors installed: `ssh root@new-node "sensors -j"`
2. Proxy auto-discovers on next poll (may take up to 1 minute)
3. Re-run the setup script to configure SSH keys on the new node: `curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | bash -s -- --ctid <CTID>`

**Permission Denied Errors:**
1. Verify socket permissions: `ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock`
2. Should be: `srw-rw---- 1 root root`
3. Check Pulse runs as root in container: `pct exec <CTID> -- whoami`

**Proxy Service Won't Start:**
1. Check logs: `journalctl -u pulse-sensor-proxy -n 50`
2. Verify binary exists: `ls -l /opt/pulse/sensor-proxy/bin/pulse-sensor-proxy`
3. Test manually: `/opt/pulse/sensor-proxy/bin/pulse-sensor-proxy --version`
4. Check socket directory: `ls -ld /var/run`

### Future Improvements

**Potential Enhancements (Roadmap):**

1. **Proxmox API Integration**
   - If future Proxmox versions expose temperature telemetry via API, retire SSH approach
   - Would eliminate SSH key management and improve security posture
   - Monitor Proxmox development for metrics/RRD temperature endpoints

2. **Agent-Based Architecture**
   - Deploy lightweight agents on each node for richer telemetry
   - Reduces SSH fan-out overhead for large clusters
   - Trade-off: Adds deployment/update complexity
   - Consider only if demand for additional metrics grows

3. **SNMP/IPMI Support**
   - Optional integration for baseboard management controllers
   - Better for hardware-level sensors (baseboard temps, fan speeds)
   - Requires hardware/firmware support, so keep as optional add-on

4. **Schema Validation**
   - Add JSON schema validation for `sensors -j` output
   - Detect format changes early with instrumentation
   - Log warnings when unexpected sensor formats appear

5. **Caching & Throttling**
   - Implement result caching for large clusters (10+ nodes)
   - Reduce SSH overhead with configurable TTL
   - Add request throttling to prevent SSH rate limiting

6. **Automated Key Rotation**
   - Systemd timer for automatic 90-day rotation
   - Already supported via `/opt/pulse/scripts/pulse-proxy-rotate-keys.sh`
   - Just needs timer unit configuration (documented in hardening guide)

7. **Health Check Endpoint**
   - Add `/health` endpoint separate from Prometheus metrics
   - Enable external monitoring systems (Nagios, Zabbix, etc.)
   - Return proxy status, socket accessibility, and last successful poll

**Contributions Welcome:** If any of these improvements interest you, open a GitHub issue to discuss implementation!

## Configuration Management

Starting with v4.31.1, the sensor proxy includes a built-in CLI for safe configuration management. This prevents config corruption that caused 99% of temperature monitoring failures.

### Quick Reference

```bash
# Validate config files
pulse-sensor-proxy config validate

# Add nodes to allowed list
pulse-sensor-proxy config set-allowed-nodes --merge 192.168.0.1 --merge node1.local

# Replace entire allowed list
pulse-sensor-proxy config set-allowed-nodes --replace --merge 192.168.0.1
```

**Key benefits:**
- Atomic writes with file locking prevent corruption
- Automatic deduplication and normalization
- systemd validation prevents startup with bad config
- Installer uses CLI (no more shell/Python divergence)

**See also:**
- [Sensor Proxy Config Management Guide](operations/SENSOR_PROXY_CONFIG.md) - Complete runbook
- [Sensor Proxy CLI Reference](/opt/pulse/cmd/pulse-sensor-proxy/README.md) - Full command documentation

## Control-Plane Sync & Migration

As of v4.32 the sensor proxy registers with Pulse and syncs its authorized node list via `/api/temperature-proxy/authorized-nodes`. No more manual `allowed_nodes` maintenance or `/etc/pve` access is required.

### New installs

Always pass the Pulse URL when installing:

```bash
curl -sSL https://pulse.example.com/api/install/install-sensor-proxy.sh \
  | sudo bash -s -- --ctid 108 --pulse-server http://192.168.0.149:7655
```

The installer now:

- Registers the proxy with Pulse (even for socket-only mode)
- Saves `/etc/pulse-sensor-proxy/.pulse-control-token`
- Appends a `pulse_control_plane` block to `/etc/pulse-sensor-proxy/config.yaml`

### Migrating existing hosts

If you installed before v4.32, run the migration helper on each host:

```bash
curl -sSL https://pulse.example.com/api/install/migrate-sensor-proxy-control-plane.sh \
  | sudo bash -s -- --pulse-server http://192.168.0.149:7655
```

The script registers the existing proxy, writes the control token, updates the config, and restarts the service (use `--skip-restart` if you prefer to bounce it yourself). Once migrated, temperatures for every node defined in Pulse will continue working even if the proxy can’t reach `/etc/pve` or Corosync IPC.

After migration you should see `Temperature data fetched successfully` entries for each node in `journalctl -u pulse-sensor-proxy`, and Settings → Diagnostics will show the last control-plane sync time.

### Getting Help

If temperature monitoring isn't working:

1. **Collect diagnostic info:**
   ```bash
   # On Proxmox host
   systemctl status pulse-sensor-proxy
   journalctl -u pulse-sensor-proxy -n 100 > /tmp/proxy-logs.txt
   ls -la /run/pulse-sensor-proxy/pulse-sensor-proxy.sock

   # In Pulse container
   journalctl -u pulse -n 100 | grep -i temp > /tmp/pulse-temp-logs.txt
   ```

2. **Test manually:**
   ```bash
   # On Proxmox host - test SSH to a cluster node
   ssh root@cluster-node "sensors -j"
   ```

3. **Check GitHub Issues:** https://github.com/rcourtman/Pulse/issues
4. **Include in bug report:**
   - Pulse version
   - Deployment type (LXC/Docker/native)
   - Proxy logs
   - Pulse logs
   - Output of manual SSH test
