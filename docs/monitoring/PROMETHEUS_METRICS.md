# 📊 Prometheus Metrics

Pulse exposes metrics at `/metrics` (default port `9091`).

Example scrape target:

- `http://<pulse-host>:9091/metrics`

This listener is separate from the main UI/API port (`7655`). In Docker and Kubernetes you must expose `9091` explicitly if you want to scrape it from outside the container/pod.

## 🌐 HTTP Ingress
| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_http_request_duration_seconds` | Histogram | Latency buckets by `method`, `route`, `status`. |
| `pulse_http_requests_total` | Counter | Total requests. |
| `pulse_http_request_errors_total` | Counter | 4xx/5xx errors. |

## 🔄 Polling & Nodes
| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_monitor_node_poll_duration_seconds` | Histogram | Per-node poll latency. |
| `pulse_monitor_node_poll_total` | Counter | Success/error counts per node. |
| `pulse_monitor_node_poll_staleness_seconds` | Gauge | Seconds since last success. |
| `pulse_monitor_poll_queue_depth` | Gauge | Global queue depth. |

## 🧠 Scheduler Health
| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_scheduler_queue_depth` | Gauge | Queue depth per instance type. |
| `pulse_scheduler_dead_letter_depth` | Gauge | DLQ depth per instance. |
| `pulse_scheduler_breaker_state` | Gauge | `0`=Closed, `1`=Half-Open, `2`=Open. |

## ⚡ Diagnostics Cache
| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_diagnostics_cache_hits_total` | Counter | Cache hits. |
| `pulse_diagnostics_refresh_duration_seconds` | Histogram | Refresh latency. |

## 🚨 Alerting Examples
*   **High Error Rate**: `rate(pulse_http_request_errors_total[5m]) > 0.05`
*   **Stale Node**: `pulse_monitor_node_poll_staleness_seconds > 300`
*   **Breaker Open**: `pulse_scheduler_breaker_state == 2` 
