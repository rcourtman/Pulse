# 📉 Adaptive Polling

Pulse uses an adaptive scheduler to optimize polling based on instance health and activity.

## 🧠 Architecture
*   **Scheduler**: Calculates intervals based on health/staleness.
*   **Priority Queue**: Min-heap keyed by `NextRun`.
*   **Circuit Breaker**: Prevents hot loops on failing instances using success/failure counters.
*   **Backoff**: Exponential retry delays (5s min to 5m max).
*   **Worker Pool**: One worker per configured instance (PVE/PBS/PMG), capped at 10.
*   **Global Concurrency Cap**: At most 2 polling cycles run at once to avoid resource spikes.

## 🔬 Implementation Details (Developer Info)

### Staleness Scoring
The `AdaptiveScheduler` (`internal/monitoring/scheduler.go`) relies on the `StalenessTracker` to compute a `StalenessScore` (0.0 to 1.0) based on **how long it has been since the last successful poll**.

- `0.0` = fresh (recent success)
- `1.0` = very stale or never succeeded

The staleness score is normalized against `AdaptivePollingMaxInterval` (default 5 minutes).

The scheduler applies **Exponential Smoothing** (alpha 0.6) and a small jitter (5%) to avoid oscillation.

Additional influences:
- **Error penalty**: retries tighten the interval based on the error count.
- **Queue stretch**: large queues gently stretch intervals to avoid overload.

### Circuit Breaker Recovery
The `circuitBreaker` (`internal/monitoring/circuit_breaker.go`) follows a standard state machine:
1. **Closing the Circuit**: One successful poll moves *Half-Open* → *Closed* and resets failure count.
2. **Backoff Calculation**: Retries use exponential backoff starting at 5s (multiplier 2, jitter 0.2) capped at 5m.
3. **Transient vs. Permanent**:
   - **Transient** errors (retryable) are retried up to 5 times before moving to the Dead Letter Queue.
   - **Permanent** errors move directly to the Dead Letter Queue.

**Note:** When `AdaptivePollingMaxInterval` is set to 15 seconds or less, the retry backoff is shortened (750ms initial, 6s max) to keep fast feedback loops during tight polling windows.

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
| `pulse_scheduler_queue_due_soon` | Gauge | Tasks due in the next 12 seconds. |

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
