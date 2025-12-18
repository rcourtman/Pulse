# 📉 Adaptive Polling

Pulse uses an adaptive scheduler to optimize polling based on instance health and activity.

## 🧠 Architecture
*   **Scheduler**: Calculates intervals based on health/staleness.
*   **Priority Queue**: Min-heap keyed by `NextRun`.
*   **Circuit Breaker**: Prevents hot loops on failing instances.
*   **Backoff**: Exponential retry delays (5s to 5m).

## ⚙️ Configuration
Adaptive polling is **disabled by default**.

### UI
There is currently no dedicated UI for adaptive polling in v5.

### Environment Variables
| Variable | Default | Description |
| :--- | :--- | :--- |
| `ADAPTIVE_POLLING_ENABLED` | `false` | Enable/disable. |
| `ADAPTIVE_POLLING_BASE_INTERVAL` | `10s` | Healthy poll rate. |
| `ADAPTIVE_POLLING_MIN_INTERVAL` | `5s` | Active/busy rate. |
| `ADAPTIVE_POLLING_MAX_INTERVAL` | `5m` | Idle/backoff rate. |

### system.json
You can also set `adaptivePollingEnabled` (and related interval fields) in `system.json` and restart Pulse.

## 📊 Metrics
Exposed at `:9091/metrics`.

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_monitor_poll_total` | Counter | Total poll attempts. |
| `pulse_monitor_poll_duration_seconds` | Histogram | Poll latency. |
| `pulse_monitor_poll_staleness_seconds` | Gauge | Age since last success. |
| `pulse_monitor_poll_queue_depth` | Gauge | Queue size. |
| `pulse_monitor_poll_errors_total` | Counter | Error counts by category. |

## ⚡ Circuit Breaker
| State | Trigger | Recovery |
| :--- | :--- | :--- |
| **Closed** | Normal operation. | — |
| **Open** | ≥3 failures. | Backoff (max 5m). |
| **Half-open** | Retry window elapsed. | Success = Closed; Fail = Open. |

**Dead Letter Queue**: After 5 transient or 1 permanent failure, tasks move to DLQ (30m retry).

## 🩺 Health API
`GET /api/monitoring/scheduler/health` (Auth required)

Returns:
*   Queue depth & breakdown.
*   Dead-letter tasks.
*   Circuit breaker states.
*   Per-instance staleness.
