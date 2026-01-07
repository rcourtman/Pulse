# 🩺 Scheduler Health API

**Endpoint**: `GET /api/monitoring/scheduler/health`
**Auth**: Required (`Authorization: Bearer <token>`, `X-API-Token`, or session cookie)

Returns a real-time snapshot of the adaptive scheduler, including queue state, circuit breakers, and dead-letter tasks.

## 📦 Response Format

```json
{
  "updatedAt": "2025-10-20T13:05:42Z",
  "enabled": true,
  "queue": {
    "depth": 7,
    "dueWithinSeconds": 2,
    "perType": { "pve": 4, "pbs": 2 }
  },
  "deadLetter": {
    "count": 1,
    "tasks": [
      {
        "instance": "pbs-main",
        "type": "pbs",
        "nextRun": "2025-10-20T13:06:40Z",
        "lastError": "connection timeout",
        "failures": 5
      }
    ]
  },
  "breakers": [
    {
      "instance": "pve-a",
      "type": "pve",
      "state": "half_open",
      "failures": 3,
      "retryAt": "2025-10-20T13:06:15Z"
    }
  ],
  "staleness": [
    {
      "instance": "pve-a",
      "type": "pve",
      "lastSuccess": "2025-10-20T13:05:10Z",
      "stalenessSeconds": 32,
      "stalenessScore": 0.12
    }
  ],
  "instances": [
    {
      "key": "pve::pve-a",
      "type": "pve",
      "displayName": "Pulse PVE Cluster",
      "instance": "pve-a",
      "connection": "https://pve-a:8006",
      "pollStatus": {
        "lastSuccess": "2025-10-20T13:05:10Z",
        "lastError": {
          "at": "2025-10-20T13:05:40Z",
          "message": "connection timeout",
          "category": "transient"
        },
        "consecutiveFailures": 2,
        "firstFailureAt": "2025-10-20T13:05:20Z"
      },
      "breaker": {
        "state": "half_open", // closed, open, half_open
        "retryAt": "2025-10-20T13:06:15Z",
        "failureCount": 3,
        "since": "2025-10-20T12:58:10Z",
        "lastTransition": "2025-10-20T13:05:40Z"
      },
      "deadLetter": {
        "present": false,
        "reason": "",
        "retryCount": 0
      }
    }
  ]
}
```

## 🔍 Key Fields

### Instances (`instances`)
The authoritative source for per-instance health.

*   **`pollStatus`**:
    *   `lastSuccess`: Timestamp of last successful poll.
    *   `lastError`: Details of the last error (message, category).
    *   `consecutiveFailures`: Current failure streak.
*   **`breaker`**:
    *   `state`: `closed` (healthy), `open` (failing), `half_open` (recovering).
    *   `retryAt`: Next retry time if open/half-open.
    *   `since`: When the current breaker state started.
    *   `lastTransition`: Timestamp of the last state transition.
*   **`deadLetter`**:
    *   `present`: `true` if the instance is in the DLQ (stopped polling).
    *   `reason`: Why it was moved to DLQ (e.g., `permanent_failure`).
    *   `retryCount`: DLQ retry attempts.
    *   `nextRetry`: Next scheduled retry (if any).

### Top-Level Queue and DLQ
*   **`queue`**: Snapshot of the active task queue (depth + per-type counts).
*   **`deadLetter`**: Aggregate DLQ summary plus up to 25 queued tasks.

### Optional Summaries
*   **`breakers`**: Only breakers that are not in default `closed`/zero-failure state.
*   **`staleness`**: Snapshot of staleness scores (if the tracker is enabled).

## 🛠️ Common Queries (jq)

**Find Failing Instances:**
```bash
curl -s http://HOST:7655/api/monitoring/scheduler/health | \
jq '.instances[] | select(.pollStatus.consecutiveFailures > 0) | {key, failures: .pollStatus.consecutiveFailures}'
```

**Check Dead Letter Queue:**
```bash
curl -s http://HOST:7655/api/monitoring/scheduler/health | \
jq '.instances[] | select(.deadLetter.present) | {key, reason: .deadLetter.reason}'
```

**Find Open Breakers:**
```bash
curl -s http://HOST:7655/api/monitoring/scheduler/health | \
jq '.instances[] | select(.breaker.state != "closed") | {key, state: .breaker.state}'
```
