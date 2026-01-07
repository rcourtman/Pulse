# 📊 Prometheus Metrics

Pulse exposes metrics at `/metrics` (default port `9091`).

Example scrape target:

- `http://<pulse-host>:9091/metrics`

This listener is separate from the main UI/API port (`7655`). In Docker and Kubernetes you must expose `9091` explicitly if you want to scrape it from outside the container/pod.

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

## 🚨 Alerting Examples
*   **High Error Rate**: `rate(pulse_http_request_errors_total[5m]) > 0.05`
*   **Stale Node**: `pulse_monitor_node_poll_staleness_seconds > 300`
*   **Breaker Open**: `pulse_scheduler_breaker_state == 2` 
