# Adaptive Polling Rollout Playbook

This playbook guides operators through enabling the adaptive polling scheduler in
production. Follow the steps sequentially and record key checkpoints in the run
sheet for audit purposes.

---

## 1. Prerequisites

1. **Test suite status**
   - `go test ./...` and `go test -tags=integration ./internal/monitoring -run TestAdaptiveSchedulerIntegration`
   - Adaptive polling soak test:
     ```
     HARNESS_SOAK_MINUTES=15 go test -tags=integration ./internal/monitoring -run TestAdaptiveSchedulerSoak -soak -timeout 30m
     ```
   - All tests must pass within the last 24 hours.

2. **Monitoring readiness**
   - Grafana dashboard updated with:
     - `pulse_monitor_poll_queue_depth` (gauge)
     - `pulse_monitor_poll_staleness_seconds` (gauge, per instance)
     - `pulse_monitor_poll_total` and `pulse_monitor_poll_errors_total` (rate panels)
     - Alerting panels for circuit breaker state (via scheduler health API).
   - Alerts configured (see §4).

3. **Configuration management**
   - Ensure staging and production environments are managed via `system.json` or appropriate env vars.
   - Identify the operator owning flag toggles and service restarts.

4. **Rollback plan**
   - Confirm ability to set `ADAPTIVE_POLLING_ENABLED=false` and restart `pulse-hot-dev` or equivalent service within 5 minutes.
   - Document the `systemctl restart pulse-hot-dev` command path or container restart procedure.

5. **Stakeholder sign-off**
   - Adaptive polling feature owner approves rollout window.
   - SRE and on-call engineer acknowledge the playbook.

---

## 2. Staging Rollout

1. **Enable feature flag**
   - Update staging configuration:
     ```
     export ADAPTIVE_POLLING_ENABLED=true
     ```
     or edit `system.json` and set `"adaptivePollingEnabled": true`.
   - Restart hot-dev service / container to apply:
     ```
     systemctl restart pulse-hot-dev
     ```
     (Adapt to your env if using Docker/K8s.)

2. **Verification**
   - `curl -s http://<staging-host>:7655/api/monitoring/scheduler/health | jq`
     - Expect `"enabled": true`.
   - Check Grafana dashboard for the staging cluster:
     - Queue depth should stabilise near historic baseline (< instances × 1.5).
     - Staleness gauges should stay below 60 s for healthy instances.
     - No persistent circuit breakers (`state != "closed"`) except known failing endpoints.

3. **Observation window**
   - Monitor for 24–48 hours.
   - Success criteria:
     - No increase in polling failures or alert volume.
     - Queue depth and staleness metrics remain within SLO (queue depth < 1.5× instance count, staleness < 60 s).
     - Scheduler health API shows empty dead-letter queue or expected entries only.
   - Record key metric snapshots at 0 h, 12 h, 24 h.

4. **Sign-off**
   - If criteria met, proceed to production. Otherwise revert flag to false and investigate (§6).

---

## 3. Production Rollout

1. **Rollout strategy**
   - Perform during low-traffic maintenance window.
   - Enable flag gradually by cluster or instance group (e.g., 25 % of nodes every 2 hours):
     1. Update config (`ADAPTIVE_POLLING_ENABLED=true`) for first subset.
     2. Restart service on those nodes.
     3. Watch metrics for at least 30 minutes before continuing.

2. **Monitoring during rollout**
   - Grafana dashboard per cluster:
     - `poll_queue_depth`
     - `poll_staleness_seconds`
     - `poll_total` success/error ratio
   - Scheduler health API:
     ```
     curl -s http://<prod-host>:7655/api/monitoring/scheduler/health | jq
     ```
     - Confirm `enabled: true`, `deadLetter.count` stable, `breakers` mostly empty.

3. **Success criteria**
   - Queue depth rises temporarily but settles within threshold (< 1.5× instance count).
   - Staleness stays below 60 s for healthy instances.
   - No unexplained increase in alert volume or API error rate.
   - Dead-letter queue holds only known failing targets.

4. **Completion**
   - After all nodes enabled, monitor for an additional 24 h.
   - Record final metric snapshot.

---

## 4. Grafana & Alert Configuration

1. **Dashboard panels**
   - **Queue Depth**: `pulse_monitor_poll_queue_depth`.
     - Use single-stat with alert if > 1.5× active instances for > 10 min.
   - **Instance Staleness**: panel per instance type using `pulse_monitor_poll_staleness_seconds`.
     - Alert threshold: > 60 s for > 5 min (excluding known failing instances).
   - **Polling Throughput**: rate of `pulse_monitor_poll_total{result="success"}` vs `result="error"`.
   - **Circuit Breakers / DLQ**: table from scheduler health API (via scripted datasource) highlighting non-closed breakers or DLQ entries.

2. **Alerts**
   - Queue depth > threshold for >10 min (Warning), >20 min (Critical).
   - Staleness > 60 s for >5 min (Critical).
   - Dead-letter count increase > N (based on baseline) triggers Warning.
   - Any breaker stuck in `open` for >10 min triggers Critical.

3. **Notification routing**
   - Ensure alerts route to on-call + feature owner.

---

## 5. Rollback Procedure

1. **Disable adaptive polling**
   - Set `ADAPTIVE_POLLING_ENABLED=false` (env or `system.json`).
   - Restart service (`systemctl restart pulse-hot-dev` or equivalent).

2. **Verification**
   - Scheduler health API should show `"enabled": false`.
   - Queue depth returns to pre-feature baseline within 10–15 minutes.
   - Staleness/queue alerts clear.

3. **Post-rollback actions**
   - Notify stakeholders, capture metric snapshots showing recovery.
   - File incident report if rollback triggered by outage.

---

## 6. Troubleshooting

| Symptom | Possible Cause | Action |
|---------|----------------|--------|
| Queue depth remains high (> 2× usual) | Insufficient workers, hidden breaker, misconfigured flag | Check scheduler health API for breaker states; consider increasing workers or reverting flag. |
| Staleness spikes across many instances | Backend API slowdown or connectivity issues | Inspect backend logs, network health; revert flag if duration > 15 min. |
| Dead-letter count climbs rapidly | Downstream API failures | Investigate specific instances via scheduler health API; fix credential/connectivity issues or rollback. |
| Circuit breakers stuck half-open/open | Persistent transient failures | Review error logs, ensure backoff/rate limits not starving retries; rollback if unresolved quickly. |
| Grafana panels flatline | Metrics exporter or job issue | Ensure Prometheus scraping working; verify service restarted with flag. |

### Accessing Scheduler Health API

```
curl -s http://<host>:7655/api/monitoring/scheduler/health | jq
```

Key fields:
- `queue.depth`, `queue.perType`
- `deadLetter.count`, task list
- `breakers` array with states (`closed`, `open`, `half_open`)
- `staleness` per instance

### When to Roll Back

Rollback immediately if any of the following occurs:
- Queue depth > 3× baseline for > 15 min.
- Staleness > 120 s on majority of instances.
- Dead-letter count doubles without clear cause.
- Customer-facing alerts or latency regressions attributed to adaptive polling.

Document the incident and notify stakeholders after rollback.
