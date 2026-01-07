# 📉 Adaptive Polling

Pulse uses an adaptive scheduler to optimize polling based on instance health and activity.

## 🧠 Architecture
*   **Scheduler**: Calculates intervals based on health/staleness.
*   **Priority Queue**: Min-heap keyed by `NextRun`.
*   **Circuit Breaker**: Prevents hot loops on failing instances using success/failure counters.
*   **Backoff**: Exponential retry delays (5s min to 5m max).
*   **Worker Pool**: Controlled concurrency (default 10) to limit host resource usage.

## 🔬 Implementation Details (Developer Info)

### Staleness Weighting
The `AdaptiveScheduler` (`internal/monitoring/scheduler.go`) calculates a `StalenessScore` (0.0 to 1.0) for every instance type. This score is weighted to prioritize active resources:
- **PVE (Proxmox nodes)**: High weight (1.0). Missing node data is critical.
- **VMs/Containers**: Medium weight (0.7).
- **Storage/Backups**: Lower weight (0.4). They change less frequently.

The scheduler uses **Exponential Smoothing** on the intervals to prevent rapid "bobbing" between `MinInterval` and `MaxInterval` when sensors fluctuate.

### Circuit Breaker Recovery
The `circuitBreaker` (`internal/monitoring/circuit_breaker.go`) follows the standard state machine but with Pulse-specific thresholds:
1. **Closing the Circuit**: It requires **one single successful poll** to transition from *Half-Open* back to *Closed*.
2. **Backoff Calculation**: Retries use `2^failures * 5s` up to the configured `MaxInterval`.
3. **Transient vs. Permanent**:
   - **Transient (Network, Timeout)**: Retried 5 times before moving to DLQ.
   - **Permanent (Auth 401, Forbidden 403)**: Bypasses immediate retries and moves straight to the Dead Letter Queue to avoid triggering IP lockouts on the target host.

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
