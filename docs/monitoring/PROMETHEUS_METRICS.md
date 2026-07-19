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

## 🔄 Polling & Nodes

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_monitor_poll_duration_seconds` | Histogram | Per-instance poll latency. |
| `pulse_monitor_poll_total` | Counter | Success/error counts per instance (`result` label). |
| `pulse_monitor_poll_errors_total` | Counter | Poll failures by `error_type`. |
| `pulse_monitor_poll_last_success_timestamp` | Gauge | Unix timestamp of last success. |
| `pulse_monitor_poll_staleness_seconds` | Gauge | Seconds since last success (`-1` if never succeeded). |
| `pulse_monitor_poll_queue_depth` | Gauge | Global queue depth. |
| `pulse_monitor_poll_inflight` | Gauge | In-flight polls by `instance_type`. |
| `pulse_monitor_node_poll_duration_seconds` | Histogram | Per-node poll latency. |
| `pulse_monitor_node_poll_total` | Counter | Success/error counts per node (`result` label). |
| `pulse_monitor_node_poll_errors_total` | Counter | Node poll failures by `error_type`. |
| `pulse_monitor_node_poll_last_success_timestamp` | Gauge | Unix timestamp of last node success. |
| `pulse_monitor_node_poll_staleness_seconds` | Gauge | Seconds since last node success (`-1` if never succeeded). |

## 🧠 Scheduler Health

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_scheduler_queue_due_soon` | Gauge | Tasks due within the next 12 seconds. |
| `pulse_scheduler_queue_depth` | Gauge | Queue depth per `instance_type`. |
| `pulse_scheduler_queue_wait_seconds` | Histogram | Wait time between task readiness and execution. |
| `pulse_scheduler_dead_letter_depth` | Gauge | DLQ depth by `instance_type` and `instance`. |
| `pulse_scheduler_breaker_state` | Gauge | `0`=Closed, `1`=Half-Open, `2`=Open, `-1`=Unknown. |
| `pulse_scheduler_breaker_failure_count` | Gauge | Consecutive failure count. |
| `pulse_scheduler_breaker_retry_seconds` | Gauge | Seconds until next retry allowed. |

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
- **Stale Node**: `pulse_monitor_node_poll_staleness_seconds > 300`
- **Breaker Open**: `pulse_scheduler_breaker_state == 2`
