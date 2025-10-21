# Pulse Prometheus Metrics (v4.24.0+)

Pulse exposes multiple metric families that cover HTTP ingress, per-node poll execution, scheduler health, and diagnostics caching. Use the following reference when wiring dashboards or alert rules.

---

## HTTP Request Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `pulse_http_request_duration_seconds` | Histogram | `method`, `route`, `status` | Request latency buckets. `route` is a normalised path (dynamic segments collapsed to `:id`, `:uuid`, etc.). |
| `pulse_http_requests_total` | Counter | `method`, `route`, `status` | Total requests handled. |
| `pulse_http_request_errors_total` | Counter | `method`, `route`, `status_class` | Counts 4xx/5xx responses. |

**Alert suggestion:**  
`rate(pulse_http_request_errors_total{status_class="server_error"}[5m]) > 0.05` (more than ~3 server errors/min) should page ops.

---

## Per-Node Poll Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `pulse_monitor_node_poll_duration_seconds` | Histogram | `instance_type`, `instance`, `node` | Wall-clock duration for each node poll. |
| `pulse_monitor_node_poll_total` | Counter | `instance_type`, `instance`, `node`, `result` | Success/error counts per node. |
| `pulse_monitor_node_poll_errors_total` | Counter | `instance_type`, `instance`, `node`, `error_type` | Error type breakdown (connection, auth, internal, etc.). |
| `pulse_monitor_node_poll_last_success_timestamp` | Gauge | `instance_type`, `instance`, `node` | Unix timestamp of last successful poll. |
| `pulse_monitor_node_poll_staleness_seconds` | Gauge | `instance_type`, `instance`, `node` | Seconds since last success (−1 means no success yet). |

**Alert suggestion:**  
`max_over_time(pulse_monitor_node_poll_staleness_seconds{node!=""}[10m]) > 300` indicates a node has been stale for 5+ minutes.

---

## Scheduler Health Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `pulse_scheduler_queue_due_soon` | Gauge | — | Number of tasks due within 12 seconds. |
| `pulse_scheduler_queue_depth` | Gauge | `instance_type` | Queue depth per instance type (PVE, PBS, PMG). |
| `pulse_scheduler_queue_wait_seconds` | Histogram | `instance_type` | Wait time between when a task should run and when it actually executes. |
| `pulse_scheduler_dead_letter_depth` | Gauge | `instance_type`, `instance` | Dead-letter queue depth per monitored instance. |
| `pulse_scheduler_breaker_state` | Gauge | `instance_type`, `instance` | Circuit breaker state: `0`=closed, `1`=half-open, `2`=open, `-1`=unknown. |
| `pulse_scheduler_breaker_failure_count` | Gauge | `instance_type`, `instance` | Consecutive failures tracked by the breaker. |
| `pulse_scheduler_breaker_retry_seconds` | Gauge | `instance_type`, `instance` | Seconds until the breaker will allow the next attempt. |

**Alert suggestions:**
- Queue saturation: `max_over_time(pulse_scheduler_queue_depth[10m]) > <instance count * 1.5>`
- DLQ growth: `increase(pulse_scheduler_dead_letter_depth[10m]) > 0`
- Breaker stuck open: `pulse_scheduler_breaker_state == 2` for > 10 minutes.

---

## Diagnostics Cache Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `pulse_diagnostics_cache_hits_total` | Counter | — | Diagnostics requests served from cache. |
| `pulse_diagnostics_cache_misses_total` | Counter | — | Requests that triggered a fresh probe. |
| `pulse_diagnostics_refresh_duration_seconds` | Histogram | — | Time taken to refresh diagnostics payload. |

**Alert suggestion:**  
`rate(pulse_diagnostics_cache_misses_total[5m])` spiking alongside `pulse_diagnostics_refresh_duration_seconds` > 20s can signal upstream slowness.

---

## Existing Instance-Level Poll Metrics (for completeness)

The following metrics pre-date v4.24.0 but remain essential:

| Metric | Type | Description |
| --- | --- | --- |
| `pulse_monitor_poll_duration_seconds` | Histogram | Poll duration per instance. |
| `pulse_monitor_poll_total` | Counter | Success/error counts per instance. |
| `pulse_monitor_poll_errors_total` | Counter | Error counts per instance. |
| `pulse_monitor_poll_last_success_timestamp` | Gauge | Last successful poll timestamp. |
| `pulse_monitor_poll_staleness_seconds` | Gauge | Seconds since last successful poll (instance-level). |
| `pulse_monitor_poll_queue_depth` | Gauge | Current queue depth. |
| `pulse_monitor_poll_inflight` | Gauge | Polls currently running. |

Refer to this document whenever you build dashboards or craft alert policies. Scrape all metrics from the Pulse backend `/metrics` endpoint (9091 by default for systemd installs). 
