# Pulse Sensor Proxy Audit Log Rotation

The sensor proxy writes a tamper-evident audit trail to
`/var/log/pulse/sensor-proxy/audit.log`. Every entry includes the SHA-256 hash
of the previous entry, so any modification becomes obvious. Because the process
keeps the file open and maintains the running hash in memory, rotation requires
special handling.

## Rotation Strategy

Use `logrotate` to rotate the file once it reaches 100 MB. After each rotation,
restart the proxy so it opens a new file and starts a fresh hash chain.

Create `/etc/logrotate.d/pulse-sensor-proxy` with the following contents:

```conf
/var/log/pulse/sensor-proxy/audit.log {
    daily
    size 100M
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    create 0640 pulse pulse
    sharedscripts
    postrotate
        systemctl restart pulse-sensor-proxy.service >/dev/null 2>&1 || true
    endscript
}
```

### Why a Restart Is Mandatory

`copytruncate` and similar tricks break the chain integrity. Restarting the
service ensures:

1. The proxy releases the old file descriptor.
2. A new hash chain starts at sequence 1 with an all-zero `prev_hash`.

If the proxy is not restarted, it will continue writing to the renamed file and
the rotation will have no effect.

### Chain Continuity Across Rotations

Each rotated log (`audit.log.1.gz`, `audit.log.2.gz`, …) is self-contained. To
prove continuity between files:

1. After each rotation, record the final `event_hash` from the rotated file (for
   example, store it in the filename or a checksum manifest).
2. When reviewing logs, verify the `prev_hash` of the first entry in the new
   file is the zero hash, and reconcile the recorded final hash from the prior
   file to show no entries were removed.

Maintaining this “final hash ledger” allows auditors to stitch the rotated files
together chronologically while preserving the tamper-evident guarantees.

### Permissions

Adjust the `create` directive to match the user and group that run the sensor
proxy. The example assumes both user and group are `pulse`.

---

## Post-Rotation Health Checks (v4.24.0+)

**After rotating audit logs and restarting pulse-sensor-proxy, verify adaptive polling health:**

### 1. Check Scheduler Health

```bash
curl -s http://localhost:7655/api/monitoring/scheduler/health | jq
```

**Verify:**
- Temperature proxy pollers appear in `instances[]` array
- `pollStatus.lastSuccess` is recent (within last 60 seconds)
- No new entries in `deadLetter` queue for proxy instances
- `breaker.state` is `closed` for proxy nodes

**Example check for proxy instances:**
```bash
curl -s http://localhost:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.type == "proxy" or .connection | contains("proxy")) | {key, lastSuccess: .pollStatus.lastSuccess, breaker: .breaker.state}'
```

### 2. Monitor Metrics (10-15 minutes)

Watch these metrics to ensure proxy restart didn't cause issues:

```bash
# Queue depth should remain stable
curl -s http://localhost:7655/api/monitoring/scheduler/health | jq '.queue.depth'

# Check staleness for proxy instances
curl -s http://localhost:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.type == "proxy") | {key, staleness: .pollStatus.lastSuccess}'
```

**Expected behavior:**
- Queue depth: No significant spike (< 10 temporary increase acceptable)
- Staleness: Proxy instances show fresh polls within 30-60 seconds
- No circuit breaker trips for proxy instances
- No new DLQ entries

### 3. Cross-Reference Audit Logs

**Link rotation events with scheduler health for security review:**

```bash
# Check Pulse audit log for rotation timing
journalctl -u pulse-sensor-proxy --since "10 minutes ago" | grep -E "restart|rotation"

# Check update history for any concurrent events
curl -s http://localhost:7655/api/updates/history?limit=5 | jq '.entries[] | {action, timestamp, status}'
```

**Why this matters:**
- Security auditors can correlate proxy restarts with scheduler behavior
- Update rollbacks may be concurrent with log rotations
- Rollback metadata (new in v4.24.0) provides full operational context
- Ensures restart didn't mask polling failures or breaker trips

### 4. Troubleshooting Rotation Issues

**If proxy instances don't rejoin queue:**

1. **Check service status**
   ```bash
   systemctl status pulse-sensor-proxy
   ```

2. **Verify scheduler sees the proxy**
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.type == "proxy")'
   ```

3. **Check for circuit breakers**
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.breaker.state != "closed") | {key, state: .breaker.state, retryAt: .breaker.retryAt}'
   ```

4. **Review logs for errors**
   ```bash
   journalctl -u pulse-sensor-proxy -n 50
   journalctl -u pulse | grep -E "proxy|temperature"
   ```

**Recovery actions:**
- If breakers are stuck: Restart main Pulse service (`systemctl restart pulse`)
- If DLQ entries persist: Check proxy credentials and network connectivity
- If polling doesn't resume: Verify proxy configuration in **Settings → Sensors**

---

## Related Documentation

- [Scheduler Health API](../api/SCHEDULER_HEALTH.md) - Complete API reference
- [Adaptive Polling Operations](ADAPTIVE_POLLING_ROLLOUT.md) - Health monitoring procedures
- [Pulse Sensor Proxy Hardening](../security/pulse-sensor-proxy-hardening.md) - Security configuration
- [Temperature Monitoring Security](../TEMPERATURE_MONITORING_SECURITY.md) - Proxy-specific security notes
