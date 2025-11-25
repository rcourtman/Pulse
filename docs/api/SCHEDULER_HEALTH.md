# 🩺 Scheduler Health API

**Endpoint**: `GET /api/monitoring/scheduler/health`
**Auth**: Required (Bearer token or Cookie)

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
  "instances": [
    {
      "key": "pve::pve-a",
      "type": "pve",
      "displayName": "Pulse PVE Cluster",
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
        "failureCount": 3
      },
      "deadLetter": {
        "present": false
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
*   **`deadLetter`**:
    *   `present`: `true` if the instance is in the DLQ (stopped polling).
    *   `reason`: Why it was moved to DLQ (e.g., `permanent_failure`).

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
