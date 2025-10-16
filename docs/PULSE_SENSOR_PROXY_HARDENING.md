# Pulse Temperature Proxy - Security Hardening Guide

## Overview

The `pulse-sensor-proxy` is a host-side service that provides secure temperature monitoring for containerized Pulse deployments. It addresses a critical security concern: SSH keys stored inside LXC containers can be exfiltrated if the container is compromised.

**Architecture:**
- Host-side proxy runs with minimal privileges on each Proxmox node
- Containerized Pulse communicates via Unix socket (inside the container at `/mnt/pulse-proxy/pulse-sensor-proxy.sock`, backed by `/run/pulse-sensor-proxy/pulse-sensor-proxy.sock` on the host)
- Proxy authenticates containers using Linux `SO_PEERCRED` (UID/PID verification)
- SSH keys never leave the host filesystem

```mermaid
flowchart LR
    subgraph Host Node
        direction TB
        PulseProxy["pulse-sensor-proxy service\n(systemd, user=pulse-sensor-proxy)"]
        HostSocket["/run/pulse-sensor-proxy/\npulse-sensor-proxy.sock"]
        PulseProxy -- Unix socket --> HostSocket
    end

    subgraph LXC Container (Pulse)
        direction TB
        MountPoint["/mnt/pulse-proxy/\npulse-sensor-proxy.sock"]
        PulseBackend["Pulse backend (Go)\n-hot-dev / production"]
        MountPoint --> PulseBackend
    end

    HostSocket == Proxmox mp mount ==> MountPoint
    PulseBackend -.-> Sensors[(Cluster nodes via SSH 'sensors -j')]
    PulseProxy -.-> Sensors
```

**Threat Model:**
- ✅ Container compromise cannot access SSH keys
- ✅ Container cannot directly SSH to cluster nodes
- ✅ Rate limiting prevents abuse via socket
- ✅ IP restrictions on SSH keys limit lateral movement
- ✅ Audit logging tracks all temperature requests

## Prerequisites

- Proxmox VE 7.0+ or Proxmox Backup Server 2.0+
- LXC container running Pulse (unprivileged recommended)
- Root access to Proxmox host(s)
- `lm-sensors` installed on all nodes
- Cluster SSH access configured (root passwordless SSH between nodes)

## Host Hardening

### Service Account

The proxy runs as the `pulse-sensor-proxy` user with these characteristics:
- System account (no login shell: `/usr/sbin/nologin`)
- No home directory
- Dedicated group: `pulse-sensor-proxy`
- Owns `/var/lib/pulse-sensor-proxy` and `/run/pulse-sensor-proxy`

**Verify service account:**
```bash
# Check user exists
id pulse-sensor-proxy

# Expected output:
# uid=XXX(pulse-sensor-proxy) gid=XXX(pulse-sensor-proxy) groups=XXX(pulse-sensor-proxy)

# Check shell (should be /usr/sbin/nologin)
getent passwd pulse-sensor-proxy | cut -d: -f7
```

### Systemd Unit Security

The systemd unit includes comprehensive hardening directives:

**Key security features:**
- `User=pulse-sensor-proxy` / `Group=pulse-sensor-proxy` - Unprivileged execution
- `NoNewPrivileges=true` - Prevents privilege escalation
- `ProtectSystem=strict` - Read-only `/usr`, `/boot`, `/efi`
- `ProtectHome=true` - Inaccessible `/home`, `/root`, `/run/user`
- `PrivateTmp=true` - Isolated `/tmp` and `/var/tmp`
- `SystemCallFilter=@system-service` - Restricted syscalls
- `CapabilityBoundingSet=` - No capabilities granted
- `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6` - Socket restrictions

**Verify systemd security:**
```bash
# Check service status
systemctl status pulse-sensor-proxy

# Verify user/group
ps aux | grep pulse-sensor-proxy | grep -v grep

# Expected: pulse-sensor-proxy user, not root

# Check systemd security settings
systemctl show pulse-sensor-proxy | grep -E '(User=|NoNewPrivileges|ProtectSystem|CapabilityBoundingSet)'
```

### File Permissions

**Critical paths and ownership:**
```
/var/lib/pulse-sensor-proxy/          pulse-sensor-proxy:pulse-sensor-proxy  0750
├── ssh/                            pulse-sensor-proxy:pulse-sensor-proxy  0700
│   ├── id_ed25519                  pulse-sensor-proxy:pulse-sensor-proxy  0600
│   └── id_ed25519.pub              pulse-sensor-proxy:pulse-sensor-proxy  0640
└── ssh.d/                          pulse-sensor-proxy:pulse-sensor-proxy  0750
    ├── next/                       pulse-sensor-proxy:pulse-sensor-proxy  0750
    └── prev/                       pulse-sensor-proxy:pulse-sensor-proxy  0750

/run/pulse-sensor-proxy/              pulse-sensor-proxy:pulse-sensor-proxy  0775
└── pulse-sensor-proxy.sock           pulse-sensor-proxy:pulse-sensor-proxy  0777
/mnt/pulse-proxy/                     nobody:nogroup (id-mapped)            0777
└── pulse-sensor-proxy.sock           nobody:nogroup                         0777
```

**Verify permissions:**
```bash
# Check base directory
ls -ld /var/lib/pulse-sensor-proxy/
# Expected: drwxr-x--- pulse-sensor-proxy pulse-sensor-proxy

# Check SSH keys
ls -l /var/lib/pulse-sensor-proxy/ssh/
# Expected:
# -rw------- pulse-sensor-proxy pulse-sensor-proxy id_ed25519
# -rw-r----- pulse-sensor-proxy pulse-sensor-proxy id_ed25519.pub

# Check socket directory on host (note: 0775 for container access)
ls -ld /run/pulse-sensor-proxy/
# Expected: drwxrwxr-x pulse-sensor-proxy pulse-sensor-proxy

# Check socket directory inside container
ls -ld /mnt/pulse-proxy/
# Expected: drwxrwxrwx nobody nogroup (id-mapped)
```

**Why 0775 on socket directory?**
The socket directory needs `0775` (not `0770`) to allow the container's unprivileged UID (e.g., 1001) to traverse into the directory and access the socket. The socket itself is `0777` as access control is enforced via `SO_PEERCRED`.

## LXC Container Requirements

### Configuration Summary

| Setting | Value | Purpose |
|---------|-------|---------|
| `lxc.idmap` | `u 0 100000 65536`<br>`g 0 100000 65536` | Unprivileged UID/GID mapping |
| `lxc.apparmor.profile` | `generated` or custom | AppArmor confinement |
| `lxc.cap.drop` | `sys_admin` (optional) | Drop dangerous capabilities |
| `mpX` | `/run/pulse-sensor-proxy,mp=/mnt/pulse-proxy` | Socket access from container |

### Sample LXC Configuration

**In `/etc/pve/lxc/<VMID>.conf`:**
```ini
# Unprivileged container (required)
unprivileged: 1

# AppArmor profile (recommended)
lxc.apparmor.profile: generated

# Drop CAP_SYS_ADMIN if feasible (optional but recommended)
# WARNING: May break some container management operations
lxc.cap.drop: sys_admin

# Bind mount proxy socket directory (REQUIRED)
# Note: Use an mp entry so Proxmox manages the bind mount automatically
mp0: /run/pulse-sensor-proxy,mp=/mnt/pulse-proxy
```

**Key points:**
- **Directory-level mount**: Mount `/run/pulse-sensor-proxy` directory, not the socket file itself
- **Why directory mount?** Systemd recreates the socket on restart; socket-level mounts break on recreation
- **Mode 0775**: Socket directory needs group+other execute permissions for container UID traversal
- **Socket 0777**: Actual socket is world-writable; security enforced via `SO_PEERCRED` authentication

### Upgrading Existing Installations

If you previously followed the legacy guide (manual `lxc.mount.entry` and `/run/pulse-sensor-proxy` inside the container), upgrade by **removing each node in Pulse and then re-adding it using the “Copy install script” flow in Settings → Nodes**. The script you copy from the UI now:

- Cleans up any old `lxc.mount.entry` rows and replaces them with the managed `mp` mount.
- Ensures the socket is mounted at `/mnt/pulse-proxy/pulse-sensor-proxy.sock` inside the container.
- Adds the systemd override so the container backend (or hot-dev) automatically uses the mounted socket.

This is the same workflow you used originally—no extra commands are required. Just remove the node from Pulse, click “Copy install script,” run it on the Proxmox host, and add the node again.

> **Advanced option**: If you’d rather refresh in place without removing the node, you can rerun the host-side installer directly (e.g. `sudo /opt/pulse/scripts/install-sensor-proxy.sh --ctid <id>`). The script is idempotent, but re-adding through the UI guarantees the full host + Pulse configuration is rebuilt.

### Runtime Verification

**Check container is unprivileged:**
```bash
# On host
pct config <VMID> | grep unprivileged
# Expected: unprivileged: 1

# Inside container
cat /proc/self/uid_map
# Expected: 0 100000 65536 (or similar)
# NOT: 0 0 4294967295 (privileged)
```

**Check AppArmor confinement:**
```bash
# Inside container
cat /proc/self/attr/current
# Expected: lxc-<vmid>_</var/lib/lxc> (enforcing) or similar
# NOT: unconfined
```

**Check namespace isolation:**
```bash
# Inside container
ls -li /proc/self/ns/
# Each namespace should have a unique inode number, different from host
```

**Check capabilities:**
```bash
# Inside container
capsh --print | grep Current
# Should show limited capability set
# If lxc.cap.drop: sys_admin is set, CAP_SYS_ADMIN should be absent
```

**Check bind mount:**
```bash
# Inside container
ls -la /mnt/pulse-proxy/
# Expected: pulse-sensor-proxy.sock visible

# Test socket access (requires Pulse to attempt connection)
socat - UNIX-CONNECT:/mnt/pulse-proxy/pulse-sensor-proxy.sock
# Should connect (may timeout waiting for input, but connection succeeds)
```

## Key Management

### SSH Key Restrictions

All SSH keys deployed to cluster nodes include these restrictions:
- `command="sensors -j"` - Forced command (only sensors allowed)
- `from="<subnets>"` - IP address restrictions
- `no-port-forwarding` - Disable port forwarding
- `no-X11-forwarding` - Disable X11 forwarding
- `no-agent-forwarding` - Disable agent forwarding
- `no-pty` - Disable PTY allocation

**Example authorized_keys entry:**
```
from="192.168.0.0/24,10.0.0.0/8",command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-ed25519 AAAA... pulse-sensor-proxy
```

**Configure allowed subnets:**

Create `/etc/pulse-sensor-proxy/config.yaml`:
```yaml
allowed_source_subnets:
  - "192.168.0.0/24"    # LAN subnet
  - "10.0.0.0/8"        # VPN subnet
```

Or use environment variable:
```bash
# In /etc/default/pulse-sensor-proxy (loaded by systemd)
PULSE_SENSOR_PROXY_ALLOWED_SUBNETS="192.168.0.0/24,10.0.0.0/8"
```

**Auto-detection:**
If no subnets are configured, the proxy auto-detects host IP addresses and uses them as `/32` (IPv4) or `/128` (IPv6) CIDRs. This is secure but brittle (breaks if host IP changes). Explicit configuration is recommended.

**Verify SSH restrictions:**
```bash
# On any cluster node
grep pulse-sensor-proxy /root/.ssh/authorized_keys

# Expected format:
# from="...",command="sensors -j",no-* ssh-ed25519 AAAA... pulse-sensor-proxy
```

### Key Rotation

**Rotation cadence:**
- Recommended: Every 90 days
- Minimum: Every 180 days
- After incident: Immediately

**Rotation workflow:**

The `pulse-sensor-proxy-rotate-keys.sh` script performs staged rotation with verification:

1. **Dry-run (recommended first):**
   ```bash
   /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --dry-run
   ```
   Shows what would happen without making changes.

2. **Perform rotation:**
   ```bash
   /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh
   ```

   **What happens:**
   - Generates new Ed25519 keypair in `/var/lib/pulse-sensor-proxy/ssh.d/next/`
   - Pushes new key to all cluster nodes (via RPC `ensure_cluster_keys`)
   - Verifies SSH connectivity with new key on each node
   - Atomically swaps keys:
     - Current `/ssh/` → `/ssh.d/prev/` (backup)
     - Staging `/ssh.d/next/` → `/ssh/` (active)
   - Old keys preserved in `/ssh.d/prev/` for rollback

3. **If rotation fails, rollback:**
   ```bash
   /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --rollback
   ```

   Restores previous keypair from `/ssh.d/prev/` and re-pushes to cluster nodes.

**Post-rotation verification:**
```bash
# Check new key timestamp
stat /var/lib/pulse-sensor-proxy/ssh/id_ed25519

# Verify all nodes have new key
for node in pve1 pve2 pve3; do
  echo "=== $node ==="
  ssh root@$node "grep pulse-sensor-proxy /root/.ssh/authorized_keys | tail -1"
done

# Test temperature fetch via proxy
curl -s --unix-socket /run/pulse-sensor-proxy/pulse-sensor-proxy.sock \
  -d '{"correlation_id":"test","method":"get_temp","params":{"node":"pve1"}}' \
  | jq .
```

### Automated Rotation (Optional)

**Create systemd timer:**

`/etc/systemd/system/pulse-sensor-proxy-key-rotation.service`:
```ini
[Unit]
Description=Rotate pulse-sensor-proxy SSH keys
After=pulse-sensor-proxy.service
Requires=pulse-sensor-proxy.service

[Service]
Type=oneshot
ExecStart=/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh
StandardOutput=journal
StandardError=journal
```

`/etc/systemd/system/pulse-sensor-proxy-key-rotation.timer`:
```ini
[Unit]
Description=Rotate pulse-sensor-proxy SSH keys every 90 days
Requires=pulse-sensor-proxy-key-rotation.service

[Timer]
OnCalendar=quarterly
RandomizedDelaySec=1h
Persistent=true

[Install]
WantedBy=timers.target
```

**Enable timer:**
```bash
systemctl daemon-reload
systemctl enable --now pulse-sensor-proxy-key-rotation.timer

# Check next run
systemctl list-timers pulse-sensor-proxy-key-rotation.timer
```

## Monitoring & Auditing

### Metrics Endpoint

The proxy exposes Prometheus metrics on `127.0.0.1:9127` by default.

**Available metrics:**
- `pulse_proxy_rpc_requests_total{method, result}` - RPC request counter
- `pulse_proxy_rpc_latency_seconds{method}` - RPC handler latency histogram
- `pulse_proxy_ssh_requests_total{node, result}` - SSH request counter per node
- `pulse_proxy_ssh_latency_seconds{node}` - SSH latency histogram per node
- `pulse_proxy_queue_depth` - Concurrent RPC requests (gauge)
- `pulse_proxy_rate_limit_hits_total` - Rejected requests due to rate limiting
- `pulse_proxy_build_info{version}` - Build metadata

**Configure metrics address:**

In `/etc/default/pulse-sensor-proxy`:
```bash
# Listen on all interfaces (WARNING: exposes metrics externally)
PULSE_SENSOR_PROXY_METRICS_ADDR="0.0.0.0:9127"

# Disable metrics
PULSE_SENSOR_PROXY_METRICS_ADDR="disabled"
```

**Test metrics endpoint:**
```bash
curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy
```

### Prometheus Integration

**Sample scrape configuration:**

```yaml
scrape_configs:
  - job_name: 'pulse-sensor-proxy'
    static_configs:
      - targets:
          - 'pve1:9127'
          - 'pve2:9127'
          - 'pve3:9127'
    relabel_configs:
      - source_labels: [__address__]
        regex: '([^:]+):.+'
        target_label: instance
```

### Alert Rules

**Recommended Prometheus alerts:**

```yaml
groups:
  - name: pulse-sensor-proxy
    rules:
      # High SSH failure rate
      - alert: PulseProxySSHFailureRate
        expr: |
          rate(pulse_proxy_ssh_requests_total{result="error"}[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High SSH failure rate on {{ $labels.instance }}"
          description: "{{ $value | humanize }} SSH requests/sec failing"

      # Rate limiting active
      - alert: PulseProxyRateLimiting
        expr: |
          rate(pulse_proxy_rate_limit_hits_total[5m]) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Rate limiting active on {{ $labels.instance }}"
          description: "Proxy rejecting requests due to rate limits"

      # High queue depth
      - alert: PulseProxyQueueDepth
        expr: pulse_proxy_queue_depth > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High RPC queue depth on {{ $labels.instance }}"
          description: "{{ $value }} concurrent requests (threshold: 5)"

      # Proxy down
      - alert: PulseProxyDown
        expr: up{job="pulse-sensor-proxy"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Pulse proxy down on {{ $labels.instance }}"
```

### Audit Logging

**Log format:**
All RPC requests are logged with structured fields:
- `corr_id` - Correlation ID (UUID, tracks request lifecycle)
- `uid` / `pid` - Peer credentials from `SO_PEERCRED`
- `method` - RPC method called (`get_temp`, `register_nodes`, `ensure_cluster_keys`)

**Example log entries:**
```json
{"level":"info","corr_id":"a7f3d..","uid":1001,"pid":12345,"method":"get_temp","node":"pve1","msg":"RPC request"}
{"level":"info","corr_id":"a7f3d..","node":"pve1","latency_ms":245,"msg":"Temperature fetch successful"}
```

**Query logs:**
```bash
# All RPC requests in last hour
journalctl -u pulse-sensor-proxy --since "1 hour ago" -o json | \
  jq -r 'select(.corr_id != null) | [.corr_id, .uid, .method, .node] | @tsv'

# Failed SSH requests
journalctl -u pulse-sensor-proxy --since today | grep -E '(SSH.*failed|error)'

# Rate limit hits
journalctl -u pulse-sensor-proxy --since today | grep "rate limit"

# Specific correlation ID
journalctl -u pulse-sensor-proxy | grep "corr_id=a7f3d"
```

### Rate Limiting

**Current limits (per peer UID+PID):**
- **Rate**: 20 requests/minute (token bucket with burst)
- **Burst**: 10 requests
- **Concurrency**: 10 simultaneous requests

**Behavior on limit exceeded:**
- Request rejected immediately (no queuing)
- `pulse_proxy_rate_limit_hits_total` metric incremented
- Log entry: `"Rate limit exceeded"`
- HTTP-like semantics: Similar to 429 Too Many Requests

**Adjust limits:**

Limits are hardcoded in `throttle.go`. To adjust, modify and rebuild:
```go
// cmd/pulse-sensor-proxy/throttle.go
const (
    requestsPerMin  = 20     // Change this
    requestBurst    = 10     // Change this
    maxConcurrent   = 10     // Change this
)
```

Then rebuild and restart:
```bash
go build -v ./cmd/pulse-sensor-proxy
systemctl restart pulse-sensor-proxy
```

## Incident Response

### Suspected Compromise Checklist

**If the proxy or host is suspected compromised:**

1. **Isolate immediately:**
   ```bash
   # Stop proxy service
   systemctl stop pulse-sensor-proxy

   # Block outbound SSH from host (if applicable)
   iptables -A OUTPUT -p tcp --dport 22 -j REJECT
   ```

2. **Rotate all keys:**
   ```bash
   # Remove compromised keys from all nodes
   for node in pve1 pve2 pve3; do
     ssh root@$node "sed -i '/pulse-sensor-proxy/d' /root/.ssh/authorized_keys"
   done

   # Generate new keys (don't use rotation script - may be compromised)
   rm -rf /var/lib/pulse-sensor-proxy/ssh*
   mkdir -p /var/lib/pulse-sensor-proxy/ssh
   ssh-keygen -t ed25519 -N '' -C "pulse-sensor-proxy emergency $(date -u +%Y%m%dT%H%M%SZ)" \
     -f /var/lib/pulse-sensor-proxy/ssh/id_ed25519
   chown -R pulse-sensor-proxy:pulse-sensor-proxy /var/lib/pulse-sensor-proxy/ssh
   chmod 0700 /var/lib/pulse-sensor-proxy/ssh
   chmod 0600 /var/lib/pulse-sensor-proxy/ssh/id_ed25519
   chmod 0640 /var/lib/pulse-sensor-proxy/ssh/id_ed25519.pub
   ```

3. **Audit logs:**
   ```bash
   # Export all proxy logs
   journalctl -u pulse-sensor-proxy --since "7 days ago" > /tmp/proxy-audit-$(date +%s).log

   # Look for anomalies:
   # - Unusual correlation IDs
   # - High rate limit hits
   # - Unexpected UIDs/PIDs
   # - SSH errors to unexpected nodes
   ```

4. **Reinstall proxy:**
   ```bash
   # Re-run installation script
   /opt/pulse/scripts/install-sensor-proxy.sh

   # Verify service status
   systemctl status pulse-sensor-proxy
   ```

5. **Re-push keys:**
   ```bash
   # Use proxy RPC to push new keys
   /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh
   ```

6. **Verify no persistence mechanisms:**
   ```bash
   # Check for unexpected systemd units
   systemctl list-units --all | grep -i proxy

   # Check for unexpected cron jobs
   crontab -l -u pulse-sensor-proxy

   # Check for unauthorized files in /var/lib/pulse-sensor-proxy
   find /var/lib/pulse-sensor-proxy -type f ! -path '*/ssh/*' ! -path '*/ssh.d/*'
   ```

### Post-Incident Hardening

After an incident, consider:
- **Audit all LXC containers** for unexpected privilege escalation
- **Review bind mounts** on all containers (check for unauthorized mounts)
- **Enable full syscall auditing** (`auditd`) on host
- **Restrict network access** to proxy metrics endpoint (firewall `127.0.0.1:9127`)
- **Implement log aggregation** (forward `journald` to central SIEM)

## Testing & Rollout

### Development Testing

Before deploying to production, verify the implementation with these safe tests:

**1. Build Verification:**
```bash
# Compile proxy
cd /opt/pulse
go build -v ./cmd/pulse-sensor-proxy

# Verify binary
./pulse-sensor-proxy version
# Expected: pulse-sensor-proxy dev (or version number)

# Check help output
./pulse-sensor-proxy --help
```

**2. Rotation Script Syntax:**
```bash
# Syntax check
bash -n /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh

# Help output
/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --help

# Dry-run (requires root and socket)
sudo /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --dry-run
```

**3. Configuration Validation:**
```bash
# Test config file parsing
cat > /tmp/test-config.yaml <<EOF
allowed_source_subnets:
  - "192.168.0.0/24"
  - "10.0.0.0/8"
metrics_address: "127.0.0.1:9127"
EOF

# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('/tmp/test-config.yaml'))"
```

### Production Rollout Checklist

**Phase 1: Pre-Deployment (on host)**

1. **Backup current state:**
   ```bash
   # Backup systemd unit
   cp /etc/systemd/system/pulse-sensor-proxy.service \
      /etc/systemd/system/pulse-sensor-proxy.service.backup

   # Backup SSH keys
   tar -czf /tmp/pulse-sensor-proxy-keys-backup-$(date +%s).tar.gz \
     /var/lib/pulse-sensor-proxy/ssh/

   # Note current service status
   systemctl status pulse-sensor-proxy > /tmp/pulse-sensor-proxy-status-before.txt
   ```

2. **Create service account:**
   ```bash
   # Run install script or manually create
   if ! id -u pulse-sensor-proxy >/dev/null 2>&1; then
     useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
   fi
   ```

3. **Update file ownership:**
   ```bash
   chown -R pulse-sensor-proxy:pulse-sensor-proxy /var/lib/pulse-sensor-proxy/
   chmod 0750 /var/lib/pulse-sensor-proxy/
   chmod 0700 /var/lib/pulse-sensor-proxy/ssh/
   chmod 0600 /var/lib/pulse-sensor-proxy/ssh/id_ed25519
   chmod 0640 /var/lib/pulse-sensor-proxy/ssh/id_ed25519.pub
   ```

**Phase 2: Deploy Hardened Version**

1. **Build and install binary:**
   ```bash
   cd /opt/pulse
   go build -v -o /tmp/pulse-sensor-proxy ./cmd/pulse-sensor-proxy

   # Verify build
   /tmp/pulse-sensor-proxy version

   # Install
   sudo install -m 0755 -o root -g root /tmp/pulse-sensor-proxy /usr/local/bin/pulse-sensor-proxy
   ```

2. **Install hardened systemd unit:**
   ```bash
   # Copy hardened unit
   sudo cp /opt/pulse/scripts/pulse-sensor-proxy.service /etc/systemd/system/

   # Verify syntax
   systemd-analyze verify /etc/systemd/system/pulse-sensor-proxy.service

   # Reload systemd
   sudo systemctl daemon-reload
   ```

3. **Update RuntimeDirectoryMode for LXC access:**
   ```bash
   # Ensure socket directory is accessible from container
   sudo mkdir -p /etc/systemd/system/pulse-sensor-proxy.service.d/
   cat | sudo tee /etc/systemd/system/pulse-sensor-proxy.service.d/lxc-access.conf <<'EOF'
[Service]
RuntimeDirectoryMode=0775
EOF

   sudo systemctl daemon-reload
   ```

**Phase 3: Restart and Verify**

1. **Restart service:**
   ```bash
   sudo systemctl restart pulse-sensor-proxy

   # Check status
   sudo systemctl status pulse-sensor-proxy
   ```

2. **Verify service user:**
   ```bash
   ps aux | grep pulse-sensor-proxy | grep -v grep
   # Expected: pulse-sensor-proxy user, not root
   ```

3. **Check socket permissions:**
   ```bash
   ls -ld /run/pulse-sensor-proxy/
   # Expected: drwxrwxr-x pulse-sensor-proxy pulse-sensor-proxy

   ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock
   # Expected: srwxrwxrwx pulse-sensor-proxy pulse-sensor-proxy
   ```

4. **Test from container:**
   ```bash
   # Inside LXC container running Pulse
   ls -la /run/pulse-sensor-proxy/
   # Should show socket

   # Check Pulse logs for connection success
   journalctl -u pulse -n 50 | grep -i temperature
   ```

**Phase 4: End-to-End Validation**

1. **Test RPC methods:**
   ```bash
   # On host, test socket connectivity
   echo '{"correlation_id":"test-001","method":"register_nodes","params":{}}' | \
     sudo socat - UNIX-CONNECT:/run/pulse-sensor-proxy/pulse-sensor-proxy.sock | jq .

   # Should return cluster nodes list
   ```

2. **Test temperature fetch:**
   ```bash
   # From container or via socket
   echo '{"correlation_id":"test-002","method":"get_temp","params":{"node":"pve1"}}' | \
     socat - UNIX-CONNECT:/run/pulse-sensor-proxy/pulse-sensor-proxy.sock | jq .

   # Should return sensors JSON data
   ```

3. **Verify metrics endpoint:**
   ```bash
   curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy

   # Should show metrics like:
   # pulse_proxy_rpc_requests_total{method="get_temp",result="success"} N
   # pulse_proxy_queue_depth 0
   ```

4. **Test SSH key rotation:**
   ```bash
   # Dry-run first
   sudo /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --dry-run

   # Full rotation (if confident)
   sudo /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh

   # Verify all nodes updated
   for node in pve1 pve2 pve3; do
     ssh root@$node "tail -1 /root/.ssh/authorized_keys"
   done
   ```

5. **Audit logging verification:**
   ```bash
   # Check logs include correlation IDs and peer credentials
   sudo journalctl -u pulse-sensor-proxy --since "5 minutes ago" -o json | \
     jq -r 'select(.corr_id != null) | [.corr_id, .uid, .method] | @tsv'

   # Should show structured logging with UIDs
   ```

**Phase 5: Monitoring Setup**

1. **Configure Prometheus scraping:**
   ```yaml
   # Add to prometheus.yml
   scrape_configs:
     - job_name: 'pulse-sensor-proxy'
       static_configs:
         - targets: ['localhost:9127']
   ```

2. **Import alert rules:**
   ```bash
   # Copy alert rules from docs to Prometheus alerts directory
   # Reload Prometheus configuration
   ```

3. **Verify alerts fire (optional stress test):**
   ```bash
   # Generate rate limit hits (test alert)
   for i in {1..50}; do
     echo '{"correlation_id":"stress-'$i'","method":"register_nodes","params":{}}' | \
       socat - UNIX-CONNECT:/run/pulse-sensor-proxy/pulse-sensor-proxy.sock &
   done
   wait

   # Check rate limit metric increased
   curl -s http://127.0.0.1:9127/metrics | grep rate_limit_hits
   ```

### Rollback Procedure

If issues occur during rollout:

1. **Stop new service:**
   ```bash
   sudo systemctl stop pulse-sensor-proxy
   ```

2. **Restore backup:**
   ```bash
   sudo cp /etc/systemd/system/pulse-sensor-proxy.service.backup \
          /etc/systemd/system/pulse-sensor-proxy.service
   sudo systemctl daemon-reload
   ```

3. **Restore SSH keys (if rotated):**
   ```bash
   # If rotation was performed and failed
   sudo /opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --rollback
   ```

4. **Restart with old configuration:**
   ```bash
   sudo systemctl restart pulse-sensor-proxy
   sudo systemctl status pulse-sensor-proxy
   ```

5. **Verify Pulse connectivity:**
   ```bash
   # Check Pulse can still fetch temperatures
   # Monitor Pulse logs
   ```

### Known Limitations

- **No automated unit tests**: Code verification relies on build success and manual testing
- **Key rotation requires manual trigger**: Automated timer setup is optional
- **Metrics require Prometheus**: No built-in alerting without external monitoring
- **LXC bind mount required**: Container must have directory-level bind mount configured
- **Root required for rotation script**: Script needs root to run `ensure_cluster_keys` RPC

### Future Improvements

- Add Go unit tests for validation, throttling, and metrics logic
- Implement health check endpoint (e.g., `/health`) separate from metrics
- Add support for TLS on metrics endpoint
- Create automated integration test suite
- Add `--check` flag to rotation script for pre-flight validation
- Support for multiple LXC containers accessing same proxy instance

## Appendix

### Quick Verification Checklist

**Host:**
- [ ] Service running as `pulse-sensor-proxy` user (not root)
- [ ] Keys in `/var/lib/pulse-sensor-proxy/ssh/` owned by `pulse-sensor-proxy:pulse-sensor-proxy`
- [ ] Private key permissions: `0600`
- [ ] Socket directory permissions: `0775` (not `0770`)
- [ ] Metrics endpoint accessible: `curl http://127.0.0.1:9127/metrics`

**Container:**
- [ ] Container is unprivileged (`unprivileged: 1` in config)
- [ ] Bind mount exists: `ls /mnt/pulse-proxy/pulse-sensor-proxy.sock`
- [ ] AppArmor enforced: `cat /proc/self/attr/current` shows confinement
- [ ] Pulse can connect to socket (check Pulse logs)

**SSH Keys:**
- [ ] All nodes have `pulse-sensor-proxy` key in `/root/.ssh/authorized_keys`
- [ ] Keys include `from="..."` restrictions
- [ ] Keys include `command="sensors -j"` forced command
- [ ] Keys include `no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty`

**Monitoring:**
- [ ] Prometheus scraping metrics successfully
- [ ] Alerts configured for SSH failures, rate limiting, queue depth
- [ ] Logs forwarded to central logging (optional but recommended)

### Reference Commands

**Service Management:**
```bash
systemctl status pulse-sensor-proxy          # Check service status
systemctl restart pulse-sensor-proxy         # Restart service
journalctl -u pulse-sensor-proxy -f          # Tail logs
```

**Key Management:**
```bash
/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --dry-run    # Dry-run rotation
/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh              # Perform rotation
/opt/pulse/scripts/pulse-sensor-proxy-rotate-keys.sh --rollback   # Rollback
```

**Metrics:**
```bash
curl http://127.0.0.1:9127/metrics                         # Fetch all metrics
curl -s http://127.0.0.1:9127/metrics | grep pulse_proxy   # Filter proxy metrics
```

**Manual RPC (Testing):**
```bash
# Using socat (inline JSON)
echo '{"correlation_id":"test","method":"get_temp","params":{"node":"pve1"}}' | \
  socat - UNIX-CONNECT:/run/pulse-sensor-proxy/pulse-sensor-proxy.sock

# Using Python (proper JSON-RPC client)
python3 <<'PY'
import json, socket, uuid
payload = {
    "correlation_id": str(uuid.uuid4()),
    "method": "get_temp",
    "params": {"node": "pve1"}
}
with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as s:
    s.connect("/run/pulse-sensor-proxy/pulse-sensor-proxy.sock")
    s.sendall((json.dumps(payload) + "\n").encode())
    s.shutdown(socket.SHUT_WR)
    print(s.recv(65536).decode())
PY
```

**Verification:**
```bash
# Check service user
ps aux | grep pulse-sensor-proxy | grep -v grep

# Check file ownership
ls -lR /var/lib/pulse-sensor-proxy/

# Check bind mount in container
pct enter <VMID>
ls -la /run/pulse-sensor-proxy/

# Check SSH keys on nodes
for node in pve1 pve2 pve3; do
  echo "=== $node ==="
  ssh root@$node "grep pulse-sensor-proxy /root/.ssh/authorized_keys"
done
```

---

**Document Version:** 1.0
**Last Updated:** 2025-10-13
**Applies To:** pulse-sensor-proxy v1.0+
