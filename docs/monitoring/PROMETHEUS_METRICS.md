# 📊 Prometheus Metrics

Pulse exposes metrics at `/metrics` (default port `9091`).

Example scrape target:

- `http://127.0.0.1:9091/metrics`

This listener is separate from the main UI/API port (`7655`) and binds to loopback by default. Use `PULSE_METRICS_BIND_ADDRESS` only when a scraper must reach Pulse from another host.

When `PULSE_METRICS_TOKEN` is set, Pulse refuses to serve that bearer token on a non-loopback plaintext listener unless `PULSE_METRICS_ALLOW_INSECURE_REMOTE=true` is explicitly set. Prefer a local Prometheus agent, SSH tunnel, VPN-private scrape path, or TLS/mTLS reverse proxy for remote scraping.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PULSE_METRICS_PORT` | Metrics listener port. | `9091` |
| `PULSE_METRICS_BIND_ADDRESS` | Metrics listener bind address. Keep loopback unless a protected remote scraper needs direct access. | `127.0.0.1` |
| `PULSE_METRICS_TOKEN` | Optional bearer token required for `/metrics`. | *(empty)* |
| `PULSE_METRICS_ALLOW_INSECURE_REMOTE` | Explicit opt-in to serve `PULSE_METRICS_TOKEN` on a non-loopback plaintext listener. Use only behind a trusted private network or TLS-terminating proxy. | `false` |

In Docker and Kubernetes you must expose `9091` explicitly if you want to scrape it from outside the container/pod, and the container/pod should set `PULSE_METRICS_BIND_ADDRESS` to the intended listener address.

**Helm note:** the current chart exposes only port `7655`, so Prometheus scraping requires an additional Service that targets `9091` (and a matching ServiceMonitor).

## 🌐 HTTP Ingress

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_http_request_duration_seconds` | Histogram | Latency buckets by `method`, `route`, `status`. |
| `pulse_http_requests_total` | Counter | Total requests by `method`, `route`, `status`. |
| `pulse_http_request_errors_total` | Counter | Error totals by `method`, `route`, `status_class` (`client_error`, `server_error`, `none`). |

`route` is the registered route pattern that serves the request (for example `/api/state` or `/api/orgs/{id}`), not the requested path. Requests that match no pattern, including frontend pages and assets, are recorded as `unmatched`. `method` is a standard HTTP method or `other`.

## 🔄 Polling & Nodes

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_monitor_poll_duration_seconds` | Histogram | Per-instance poll latency. |
| `pulse_monitor_poll_total` | Counter | Success/error counts per instance (`result` label). |
| `pulse_monitor_poll_errors_total` | Counter | Poll failures by `error_type`. |
| `pulse_monitor_poll_last_success_timestamp` | Gauge | Unix timestamp of last success. |
| `pulse_monitor_poll_staleness_seconds` | Gauge | Age since success at last result. |
| `pulse_monitor_poll_queue_depth` | Gauge | Global queue depth. |
| `pulse_monitor_poll_inflight` | Gauge | In-flight polls by `instance_type`. |
| `pulse_monitor_node_poll_duration_seconds` | Histogram | Per-node poll latency. |
| `pulse_monitor_node_poll_total` | Counter | Success/error counts per node (`result` label). |
| `pulse_monitor_node_poll_errors_total` | Counter | Node poll failures by `error_type`. |
| `pulse_monitor_node_poll_last_success_timestamp` | Gauge | Unix timestamp of last node success. |
| `pulse_monitor_node_poll_staleness_seconds` | Gauge | Age since success at last result. |

### Check whether polling is still running

Use your existing scrape. A successful scrape means Pulse served metrics, not
that polling succeeded.

First match the Pulse server and the monitored connection. With Prometheus's
default `honor_labels: false`, the `instance` label on scraped poll metrics
identifies the **Pulse scrape target**. Pulse's own `instance` label is renamed
to `exported_instance` and identifies the **monitored connection**. Filter on
both, especially if two Pulse servers use the same connection name. For
example, check the scrape and the PVE poll results separately over the same
window, replacing the example labels with yours:

```promql
up{job="pulse", instance="pulse-a.example:9091"}
```

```promql
pulse_monitor_poll_total{
  job="pulse", instance="pulse-a.example:9091",
  instance_type="pve", exported_instance="home-pve"
}
```

Keep the `result` series separate. For a PBS connection, select its `pbs`
type and name. For the per-node counters, also select the expected `node`.
If the scrape failed (`up` is `0`) or the matching `up` series is missing,
you cannot tell from a flat poll graph whether collection stopped. If you
preserve exporter labels (`honor_labels: true`) or relabel metrics, inspect
the actual series labels instead. A poll metric's `instance` may then be the
connection name, not a label you can match to `up`.

The staleness gauges above update when a poll result is recorded. The gauge
is `-1` after a failure before the first success. If polls stop finishing
after a success, the gauge can stay at zero.
Instead, subtract the last successful poll time from the query time:

- **Per connection:** `time() - pulse_monitor_poll_last_success_timestamp`
- **Per node:** `time() - pulse_monitor_node_poll_last_success_timestamp`

Look at the connections and nodes you still expect Pulse to poll. Allow for
their configured cadence, including adaptive polling, and normal poll duration.
An age that keeps rising means no new success has been recorded. An absent
series is not an age of zero: Pulse may not have completed a successful poll
since it started, or Prometheus may not have a current scrape. Check the
expected series and scrape gaps separately.

If the age keeps rising during successful scrapes, plot the matching
`pulse_monitor_poll_total` for each connection, split by `result`. For a
Proxmox node, use `pulse_monitor_node_poll_total` instead. Compare several
scrapes of the same source without a Pulse restart. Choose a window longer
than its usual time between results, including poll duration and retry
backoff. Do not fill absent series with zero. For example, the
`result="error"` series may not exist until the first failure is recorded.

| What you see | What it means |
| :--- | :--- |
| No new success, but the `result="error"` counter rises | Pulse is still recording outcomes for that source. Check why its polls are failing. |
| No new outcomes across several sources that normally poll in that window, despite fresh scrapes | Check for intentional pauses and normal backoff. If neither explains the gap, note when the last readings advanced and whether the monitored systems were still responding. Flat counters alone do not distinguish slow polls from stopped work. |

When comparing runs before and after an update, check the same connections and
nodes in both windows. A new poll timestamp does not prove every guest or
backup datastore refreshed, that agent reports were accepted, or that alerts
were evaluated. Check those paths separately, and treat a gap as unverified
rather than a memory saving.

## 🧠 Scheduler Health

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_scheduler_queue_due_soon` | Gauge | Tasks still in the scheduler queue that are due now or within 12 seconds, at the last queue snapshot. |
| `pulse_scheduler_queue_depth` | Gauge | Queue depth per `instance_type`. |
| `pulse_scheduler_queue_wait_seconds` | Histogram | Wait time between task readiness and execution. |
| `pulse_scheduler_claimed_poll_overdue_seconds` | Gauge | Seconds past one poll task timeout for the most overdue claimed poll, by stage. Zero if no claimed poll is over that budget. |
| `pulse_scheduler_dead_letter_depth` | Gauge | DLQ depth by `instance_type` and `instance`. |
| `pulse_scheduler_breaker_state` | Gauge | `0`=Closed, `1`=Half-Open, `2`=Open, `-1`=Unknown. |
| `pulse_scheduler_breaker_failure_count` | Gauge | Consecutive failure count. |
| `pulse_scheduler_breaker_retry_seconds` | Gauge | Seconds until next retry allowed. |

When expected results stop, check the two stages on successive scrapes:

- `waiting_for_slot`: a due poll left the scheduler queue but is still waiting
  for a process-wide worker slot (`POLL_TASK_WORKERS`). The budget starts at
  the poll's scheduled time.
- `running`: a worker took the poll but has not returned, including task
  preparation, provider calls and rescheduling. The budget starts at worker
  admission. An execution deadline can expire before the work actually returns.

Use this signal with the [result counters](#check-whether-polling-is-still-running).
A returned error waiting for its next retry is not overdue work. The gauges
cover claimed polls across the Pulse process, not individual connections or
tasks still in the queue. Zero does not prove polling is healthy, and a
missing series is not zero. This does not measure agent reports or rule
evaluation, decide that a target is down, or send an alert.

## ⚡ Diagnostics Cache

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_diagnostics_cache_hits_total` | Counter | Cache hits. |
| `pulse_diagnostics_cache_misses_total` | Counter | Cache misses. |
| `pulse_diagnostics_refresh_duration_seconds` | Histogram | Refresh latency. |

## 🚨 Alert Lifecycle

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_alerts_active` | Gauge | Active alerts by `level` and `type`. |
| `pulse_alerts_fired_total` | Counter | Total alerts fired by `level` and `type`. |
| `pulse_alerts_resolved_total` | Counter | Total alerts resolved by `type`. |
| `pulse_alerts_acknowledged_total` | Counter | Total alerts acknowledged. |
| `pulse_alerts_suppressed_total` | Counter | Alerts suppressed by `reason` (quiet_hours, rate_limit, duplicate, etc.). |
| `pulse_alerts_rate_limited_total` | Counter | Alerts suppressed due to rate limiting. |
| `pulse_alert_duration_seconds` | Histogram | Time from alert fire to resolve (by `type`). |

## Operational Trust

Operational Trust metrics use closed low-cardinality labels. They never label
series with resource, operational-record, evidence, actor, provider-instance,
or notification-destination IDs.

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_operational_trust_observation_to_open_seconds` | Histogram | Time from the first canonical observation to an open operational record. |
| `pulse_operational_trust_open_to_notification_enqueue_seconds` | Histogram | Time from an open transition to durable notification enqueue. |
| `pulse_operational_trust_evidence_observations_total` | Counter | Evidence observations by bounded `source` and state (`current`, `stale`, `unknown`, `partial`, `unavailable`, `partial_permission`, `denied`, or `other`). |
| `pulse_operational_trust_identity_correlations_total` | Counter | Resource correlation outcomes (`attached`, `standalone`, `ambiguous`, `unresolved`, or `other`). |
| `pulse_operational_trust_protection_posture_evaluations_total` | Counter | Protection posture evaluations by state. |
| `pulse_operational_trust_protection_posture_evaluation_failures_total` | Counter | Posture evaluation failures by bounded reason. |
| `pulse_operational_trust_notification_delivery_total` | Counter | Transition-linked delivery outcomes (`queued`, `retry`, `sent`, `failed`, `dead_letter`, `cancelled`, or `other`). |
| `pulse_operational_trust_active_count_mismatch_total` | Counter | Detected disagreement between the canonical active records and attention projection. |
| `pulse_operational_trust_action_offers_total` | Counter | Action-offer projections by eligibility. |
| `pulse_operational_trust_action_verification_total` | Counter | Verification outcomes, including confirmed, contradicted, inconclusive, timed out, and not attempted. |

Example rollout alerts:

- `increase(pulse_operational_trust_active_count_mismatch_total[15m]) > 0`
- `increase(pulse_operational_trust_notification_delivery_total{outcome="dead_letter"}[15m]) > 0`
- `increase(pulse_operational_trust_action_verification_total{outcome=~"contradicted|timed_out"}[30m]) > 0`

## 🚨 Alerting Examples
- **High Error Rate**: `rate(pulse_http_request_errors_total[5m]) > 0.05`
- **Overdue node poll with a recorded success**: `time() - pulse_monitor_node_poll_last_success_timestamp > 300`. Five minutes is an example, not a default for every cadence. The check does not alert on a missing series. Use the [poll and scrape checks](#check-whether-polling-is-still-running) above.
- **Breaker Open**: `pulse_scheduler_breaker_state == 2`
