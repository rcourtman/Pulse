# Adaptive Polling Operations Runbook

**GA in v4.24.0 - Enabled by Default**

This runbook guides operators managing the adaptive polling scheduler in production. Adaptive polling is enabled by default in v4.24.0 and can be toggled via **Settings → System → Monitoring** (no restart) or environment variables (restart required).

Follow these operational procedures for steady-state monitoring, troubleshooting, and rollback scenarios.

---

## 1. Preparation (Before v4.24.0 Deployment)

**For new deployments or upgrades to v4.24.0:**

1. **Monitoring readiness**
   - Set up Grafana dashboard with:
     - `pulse_monitor_poll_queue_depth` (gauge)
     - `pulse_monitor_poll_staleness_seconds` (gauge, per instance)
     - `pulse_monitor_poll_total` and `pulse_monitor_poll_errors_total` (rate panels)
     - `pulse_monitor_poll_last_success_timestamp` (new in v4.24.0)
     - Alerting panels for circuit breaker state (via scheduler health API)
   - Configure alerts (see §4)

2. **Baseline metrics**
   - Record pre-upgrade metrics if upgrading from < v4.24.0:
     - Typical polling frequency
     - Average response times
     - Current alert volumes
   - These help assess adaptive polling impact

3. **Rollback readiness**
   - Verify update rollback workflow: **Settings → System → Updates → Restore previous version**
   - Test rollback in staging environment
   - Confirm `/api/monitoring/scheduler/health` accessible
   - Document emergency disable procedure (see §5)

4. **Configuration review**
   - Review `system.json` or environment variables for adaptive polling tunables:
     - `ADAPTIVE_POLLING_BASE_INTERVAL` (default: 10s)
     - `ADAPTIVE_POLLING_MIN_INTERVAL` (default: 5s)
     - `ADAPTIVE_POLLING_MAX_INTERVAL` (default: 5m)
   - Adjust if needed for your environment (e.g., high-frequency monitoring)

---

## 2. Post-Deployment Verification (v4.24.0+)

**Adaptive polling is enabled by default. Verify it's working correctly:**

1. **Check scheduler health**
   ```bash
   curl -s http://<host>:7655/api/monitoring/scheduler/health | jq
   ```

   **Expected response:**
   - `"enabled": true`
   - `queue.depth` reasonable (< instances × 1.5)
   - `deadLetter.count` = 0 (or only known failing instances)
   - `instances[]` array populated with your nodes
   - No `breaker.state` stuck in `open` (except known issues)

2. **Verify UI access**
   - Navigate to **Settings → System → Monitoring**
   - Confirm "Adaptive Polling" toggle is ON
   - Review queue depth and recent poll status

3. **Check Grafana metrics** (if configured)
   - `pulse_monitor_poll_queue_depth` shows reasonable values
   - `pulse_monitor_poll_staleness_seconds` < 60s for healthy instances
   - `pulse_monitor_poll_errors_total` not rapidly increasing
   - `pulse_monitor_poll_last_success_timestamp` updating regularly

4. **Monitor update history**
   - Check **Settings → System → Updates** for update entry
   - Verify rollback button is available
   - Confirm update status shows "completed"

   **Via API:**
   ```bash
   curl -s http://<host>:7655/api/updates/history | jq '.entries[0]'
   ```

---

## 3. Steady-State Operations

**Ongoing monitoring and SLO checks:**

1. **Daily health checks**
   - Review scheduler health dashboard or API endpoint
   - Check for:
     - Queue depth < 50 (alert if > 50 for 10+ minutes)
     - Staleness < 120s for critical instances
     - DLQ count stable (not growing)
     - Circuit breakers mostly `closed`

2. **Weekly reviews**
   - Analyze trends in Grafana:
     - Poll success rates
     - Average queue depth over time
     - Circuit breaker trip frequency
     - Dead-letter queue patterns
   - Document any recurring issues

3. **SLO targets**
   - **Queue depth**: < 1.5× instance count (< 50 typical)
   - **Staleness**: < 60s for healthy instances, < 120s for critical instances
   - **Poll success rate**: > 99% for healthy infrastructure
   - **DLQ growth**: < 5% per week (excluding known failures)
   - **Circuit breaker recovery**: < 5 minutes for transient failures

4. **Log correlation**
   - Cross-reference scheduler health with update history
   - Check `/api/updates/history` for rollback events correlated with scheduler issues
   - Review audit logs for adaptive polling configuration changes

---

## 4. Grafana & Alert Configuration

1. **Dashboard panels**
   - **Queue Depth**: `pulse_monitor_poll_queue_depth`
     - Use single-stat with alert if > 1.5× active instances for > 10 min
   - **Instance Staleness**: panel per instance type using `pulse_monitor_poll_staleness_seconds`
     - Alert threshold: > 60 s for > 5 min (excluding known failing instances)
   - **Polling Throughput**: rate of `pulse_monitor_poll_total{result="success"}` vs `result="error"`
   - **Circuit Breakers / DLQ**: table from scheduler health API (via scripted datasource) highlighting non-closed breakers or DLQ entries
   - **Last Success Timestamp** (v4.24.0+): `pulse_monitor_poll_last_success_timestamp` to detect polling gaps

2. **Alerts**
   - Queue depth > threshold for >10 min (Warning), >20 min (Critical)
   - Staleness > 60 s for >5 min (Critical)
   - Dead-letter count increase > N (based on baseline) triggers Warning
   - Any breaker stuck in `open` for >10 min triggers Critical
   - Permanent failures (`pulse_monitor_poll_errors_total{category="permanent"}`) trigger immediate Critical

3. **Notification routing**
   - Ensure alerts route to on-call + feature owner

---

## 5. Rollback Procedures

### Option A: Disable Adaptive Polling (Keep v4.24.0)

**If adaptive polling causes issues but you want to keep v4.24.0:**

1. **Via UI (No restart required)**
   - Navigate to **Settings → System → Monitoring**
   - Toggle "Adaptive Polling" OFF
   - Changes apply immediately

2. **Via environment variables (Restart required)**
   ```bash
   # Systemd
   sudo systemctl edit pulse
   # Add:
   [Service]
   Environment="ADAPTIVE_POLLING_ENABLED=false"

   # Then restart
   sudo systemctl restart pulse
   ```

3. **Verification**
   ```bash
   curl -s http://<host>:7655/api/monitoring/scheduler/health | jq '.enabled'
   # Should return: false
   ```

### Option B: Full Version Rollback

**If v4.24.0 causes broader issues:**

1. **Via UI**
   - Navigate to **Settings → System → Updates**
   - Click **"Restore previous version"**
   - Confirm rollback
   - Pulse restarts automatically with previous version

2. **Via CLI**
   ```bash
   # Systemd installations
   sudo /opt/pulse/pulse config rollback

   # LXC containers
   pct exec <ctid> -- bash -c "cd /opt/pulse && ./pulse config rollback"
   ```

3. **Verification**
   ```bash
   # Check version
   curl -s http://<host>:7655/api/version | jq '.version'

   # Check update history
   curl -s http://<host>:7655/api/updates/history | jq '.entries[0]'
   # Should show action="rollback", status="completed"
   ```

4. **Post-rollback**
   - Verify rollback logged in update history
   - Check journal: `journalctl -u pulse | grep rollback`
   - Monitor for 15-30 minutes to ensure stability
   - Document rollback reason and notify stakeholders

---

## 6. Troubleshooting

| Symptom | Possible Cause | Action |
|---------|----------------|--------|
| Queue depth remains high (> 2× usual) | Insufficient workers, hidden breaker, misconfigured flag | Check scheduler health API for breaker states; consider increasing workers or reverting flag |
| Staleness spikes across many instances | Backend API slowdown or connectivity issues | Inspect backend logs, network health; revert flag if duration > 15 min |
| Dead-letter count climbs rapidly | Downstream API failures | Investigate specific instances via scheduler health API; fix credential/connectivity issues or rollback |
| Circuit breakers stuck half-open/open | Persistent transient failures | Review error logs, ensure backoff/rate limits not starving retries; rollback if unresolved quickly |
| Grafana panels flatline | Metrics exporter or job issue | Ensure Prometheus scraping working; verify service restarted with flag |

### Accessing Scheduler Health API

```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health | jq
```

Key sections to inspect:

- `queue.depth`, `queue.perType`
- `instances[].pollStatus` (success/failure streaks and last error)
- `instances[].breaker` (current breaker state, retry windows)
- `instances[].deadLetter` (reason, retry counts, schedules)
- `staleness` (normalized freshness score)

Common queries:

**Instances with errors:**
```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.pollStatus.lastError != null) | {key, lastError: .pollStatus.lastError}'
```

**Current dead-letter entries:**
```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.deadLetter.present) | {key, reason: .deadLetter.reason, retryCount: .deadLetter.retryCount}'
```

**Breakers not closed:**
```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.breaker.state != "closed") | {key, breaker: .breaker}'
```

### When to Roll Back

Rollback immediately if any of the following occurs:
- Queue depth > 3× baseline for > 15 min
- Staleness > 120 s on majority of instances
- Dead-letter count doubles without clear cause
- Customer-facing alerts or latency regressions attributed to adaptive polling

Document the incident and notify stakeholders after rollback.

---

## 7. Related Documentation

- [Scheduler Health API](../api/SCHEDULER_HEALTH.md) - Complete API reference
- [Adaptive Polling Architecture](../monitoring/ADAPTIVE_POLLING.md) - Technical details
- [Management Endpoints](ADAPTIVE_POLLING_MANAGEMENT_ENDPOINTS.md) - Circuit breaker/DLQ controls
- [Configuration Guide](../CONFIGURATION.md) - Adaptive polling settings
