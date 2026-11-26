# 📉 Adaptive Polling

Pulse uses an adaptive scheduler to optimize polling based on instance health and activity.

## 🧠 Architecture
- **Planning**: The monitor puts scheduled polls in a queue ordered by when
  they are due. The limit of two concurrent planning passes per monitor does
  not limit the number of polls workers can run.
- **Workers**: By default each monitor sizes its worker pool from the targets
  it can poll at startup, with at least one and at most ten workers. Workers
  take due tasks from the queue. A worker is not reserved for each connection.
  A positive `POLL_TASK_WORKERS` override instead caps concurrent poll tasks
  across the whole Pulse process, including across workspaces. The override
  is capped at 128. Both modes apply whether adaptive timing is on or off.
- **Retries**: Backoff spaces out attempts after returned errors. A circuit
  breaker can defer work for one connection without pausing the other
  connections.

### Check where a poll stopped progressing

Queue depth and the number of tasks due soon describe work still in the queue
at the time of the snapshot. A due poll that a worker or dispatcher has already
taken is no longer counted. A snapshot with nothing due can therefore coexist
with an overdue poll.

[Compare recorded poll results](../api/SCHEDULER_HEALTH.md#compare-two-poll-snapshots)
for the affected connection. A newer error without a newer success means Pulse
recorded a failed poll. A newer success with a partial error means some
readings are still missing. Neither a quiet queue nor a flat result counter
shows that the target is down.

If your build exposes [overdue poll stages](PROMETHEUS_METRICS.md#-scheduler-health)
and you already scrape Pulse, a positive `waiting_for_slot` value means
overdue work is waiting for a shared worker slot. A positive `running` value
means a worker took the poll and has not returned within its time budget.
Those are server-wide signals, not one connection's result. The queue alone cannot
tell these stages apart. Do not change worker limits, timeouts or polling
intervals just to identify the held stage.

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
The scheduler's retry delay and the circuit breaker's retry deadline are
separate. For a retryable error, the scheduler uses exponential backoff with
jitter, starting from a base of 5 seconds and capped at 5 minutes. With
adaptive timing enabled and a maximum interval of 15 seconds or less, the
scheduler uses a 750ms base and a 4-second cap. An open breaker can defer
execution independently of that delay. Read the connection's recorded
`breaker.retryAt` rather than calculating it from the scheduler backoff.

## ⚙️ Configuration
Adaptive polling is **disabled by default**.

### UI
There is currently no dedicated UI for adaptive polling. Use environment variables or `system.json`.

### Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `ADAPTIVE_POLLING_ENABLED` | `false` | Enable/disable. |
| `ADAPTIVE_POLLING_BASE_INTERVAL` | `10s` | Starting interval before an instance has a schedule. |
| `ADAPTIVE_POLLING_MIN_INTERVAL` | `5s` | Shortest interval, approached as data goes unrefreshed. |
| `ADAPTIVE_POLLING_MAX_INTERVAL` | `5m` | Longest interval. Healthy instances approach it. |

### system.json
You can also set `adaptivePollingEnabled` (and related interval fields) in `system.json` and restart Pulse.

### What changes when it is on
Adaptive polling replaces the Proxmox VE, PBS and PMG polling intervals
(Settings → General → Monitoring cadence, `PBS_POLLING_INTERVAL`,
`PMG_POLLING_INTERVAL`) for those platforms. Each successful poll resets an
instance's staleness score, so the scheduler stretches a healthy instance's
interval toward `ADAPTIVE_POLLING_MAX_INTERVAL`. With the defaults, healthy
instances are polled every few minutes, not every 10 seconds.

Freshness follows that cadence. A Proxmox VE, PBS or PMG resource is shown as
stale only when its last sighting from a poll is older than twice the maximum
interval (10 minutes with the defaults). A Proxmox node that stops answering can
keep showing online for the same window after it was last seen online, although
a cluster's own report that a member is offline applies immediately. Offline
nodes and lost data therefore take longer to show than with fixed polling. For a
lighter but predictable load, raise the polling intervals instead: freshness and
offline detection then scale with the interval you choose.

## 📊 Metrics
The separate metrics listener defaults to port 9091; `METRICS_PORT` can override
it. Use an existing authorised scrape if available, not a newly exposed port for
diagnostics. A missing scrape is not evidence of zero errors.

| Metric | Type | Description |
| :--- | :--- | :--- |
| `pulse_monitor_poll_total` | Counter | Recorded poll outcomes, split by result. |
| `pulse_monitor_poll_duration_seconds` | Histogram | Poll latency. |
| `pulse_monitor_poll_staleness_seconds` | Gauge | Age since the last success. Older builds update it only when a result is recorded. |
| `pulse_monitor_poll_queue_depth` | Gauge | Total number of tasks still queued at scrape time. |
| `pulse_monitor_poll_errors_total` | Counter | Error counts by category. |
| `pulse_scheduler_queue_due_soon` | Gauge | Tasks still queued that are due now or within 12 seconds at scrape time. Older builds report the last recorded queue sample. |

If a scrape still works but polling stops producing results, use the
[last-success timestamp](PROMETHEUS_METRICS.md#check-whether-polling-is-still-running)
to check age on either type of exporter. A staleness gauge that stays at zero
on an older build does not rule out a stall. If a circuit breaker is open,
[check its recorded retry time](PROMETHEUS_METRICS.md#check-whether-backoff-still-explains-the-wait)
before attributing the gap to backoff.

## ⚡ Circuit Breaker

| State | Trigger | Recovery |
| :--- | :--- | :--- |
| **Closed** | Normal operation. | — |
| **Open** | Three consecutive poll failures while closed. | The recorded retry deadline must pass before the breaker admits a poll. |
| **Half-open** | The breaker admits a poll after the open deadline. | A success closes the breaker and resets its failure count. A failure opens it again. |

### Read the dead-letter entry alongside poll results

A permanent failure or five consecutive retryable failures moves the failed
task to the Dead Letter Queue. Pulse records a retry time 30 minutes after
that entry, but this is not a countdown to the next poll. The normal scheduler
can poll the same connection sooner. A later scheduled success clears the
entry for that connection. If a read is partial, the old terminal entry clears
but the current partial error remains. Older builds can keep an entry even
after a later poll succeeds.

Use a [saved scheduler snapshot](../api/SCHEDULER_HEALTH.md#-common-queries-jq).
This query shows the times for connections with a dead-letter entry. It makes
no additional requests. Keep the output private because the key can contain
the connection name.

```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '.instances[] | select(.deadLetter.present == true) |
  {key, deadLetterAt: .deadLetter.lastAttempt, recordedRetryAt: .deadLetter.nextRetry,
   lastSuccess: .pollStatus.lastSuccess, currentErrorAt: .pollStatus.lastError.at,
   currentErrorCategory: .pollStatus.lastError.category}' "$snapshot"
```

If `lastSuccess` is newer than `deadLetterAt` and there is no current error,
Pulse recorded a later successful poll with no outstanding error. If a
`partial` error remains, some readings are still missing. No rows means no
entries in that snapshot, not that polling is healthy. To check a gap now,
[compare two poll results](../api/SCHEDULER_HEALTH.md#compare-two-poll-snapshots)
over the normal interval. Do not wait for the recorded retry time or reset
the breaker just to find out whether polling has resumed.

## 🩺 Health API
`GET /api/monitoring/scheduler/health` (Auth required)

Returns:
- Queue depth & breakdown.
- Dead-letter tasks.
- Circuit breaker states.
- Per-instance staleness.

Use the [authenticated snapshot and local queries](../api/SCHEDULER_HEALTH.md#-common-queries-jq)
to inspect it without putting credentials in command arguments or repeatedly
requesting the same data. The response's `enabled` flag describes adaptive
timing. You can still [compare poll results](../api/SCHEDULER_HEALTH.md#compare-two-poll-snapshots)
when it is false. Follow the [rollout and rollback checks](../operations/ADAPTIVE_POLLING_ROLLOUT.md)
when changing the flag. Queue depth alone is not a monitoring-health verdict.
