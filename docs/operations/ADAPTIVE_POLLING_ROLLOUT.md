# Adaptive Polling Rollout Runbook

Adaptive polling (v4.24.0+) lets the scheduler dynamically adjust poll
intervals per resource. This runbook documents the safe way to enable, monitor,
and, if needed, disable the feature across environments.

## Scope & Prerequisites

- Pulse **v4.24.0 or newer**
- Admin access to **Settings → System → Monitoring**
- Prometheus access to `pulse_monitor_*` metrics
- Ability to run authenticated `curl` commands against the Pulse API

## Change Windows

Run rollouts during a maintenance window where transient alert jitter is
acceptable. Adaptive polling touches every monitor queue; give yourself at least
15 minutes to observe steady state metrics.

## Rollout Steps

1. **Snapshot current health**
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health | jq '.enabled, .queue.depth'
   ```
   Record queue depth, breaker count, and dead-letter entries.

2. **Enable adaptive polling**
   - UI: toggle **Settings → System → Monitoring → Adaptive Polling** → Enable
   - CLI: `jq '.AdaptivePollingEnabled=true' /var/lib/pulse/system.json > tmp && mv tmp system.json`
   - Env override: `ADAPTIVE_POLLING_ENABLED=true` before starting Pulse (for
     containers/k8s)

3. **Watch metrics (first 5 minutes)**
   ```bash
   watch -n 5 'curl -s http://localhost:9091/metrics | grep -E "pulse_monitor_(poll_queue_depth|poll_staleness_seconds)" | head'
   ```
   Targets:
   - `pulse_monitor_poll_queue_depth < 50`
   - `pulse_monitor_poll_staleness_seconds` under your SLA (typically < 60 s)
   - No spikes in `pulse_monitor_poll_errors_total{category="permanent"}`

4. **Validate scheduler state**
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '{enabled, queue: .queue.depth, breakers: [.breakers[]?.instance], deadLetter: .deadLetter.count}'
   ```
   Expect `enabled: true`, empty breaker list, and `deadLetter.count == 0`.

5. **Document overrides**
   - Note any instances moved to manual polling (Settings → Nodes → Polling)
   - Capture Grafana screenshots for queue depth/staleness widgets

## Rollback

If queue depth climbs uncontrollably or breakers remain open for >10 minutes:

1. Disable the feature the same way you enabled it (UI/environment).
2. Restart Pulse if environment overrides were used, otherwise hot toggle is
   immediate.
3. Continue monitoring until queue depth and staleness return to baseline.

## Canary Strategy Suggestions

| Stage | Action | Acceptance Criteria |
| --- | --- | --- |
| Dev | Enable flag in hot-dev (scripts/hot-dev.sh) | No scheduler panics, UI reflects flag instantly |
| Staging | Enable on one Pulse instance per region | `queue.depth` within ±20 % of baseline after 15 min |
| Production | Enable per cluster with 30 min soak | No more than 5 breaker openings per hour |

## Instrumentation Checklist

- Grafana dashboard with `queue.depth`, `poll_staleness_seconds`,
  `poll_errors_total` by type
- Alert rule: `rate(pulse_monitor_poll_errors_total{category="permanent"}[5m]) > 0`
- Alert rule: `max_over_time(pulse_monitor_poll_queue_depth[5m]) > 75`
- JSON log search for `"scheduler":` warnings immediately after enablement

## References

- [Architecture doc](../monitoring/ADAPTIVE_POLLING.md)
- [Scheduler Health API](../api/SCHEDULER_HEALTH.md)
- [Kubernetes guidance](../KUBERNETES.md#adaptive-polling-configuration-v4250)
