# Scheduler Health API

Endpoint: `GET /api/monitoring/scheduler/health`

Returns a snapshot of the adaptive polling scheduler, queue state, circuit breakers, and per-instance status. Requires authentication (session cookie or bearer token).

---

## Request

```
GET /api/monitoring/scheduler/health
Authorization: Bearer <token>
```

No query parameters are needed.

---

## Response Overview

```json
{
  "updatedAt": "2025-10-20T13:05:42Z",
  "enabled": true,
  "queue": {...},
  "deadLetter": {...},
  "breakers": [...],          // legacy summary
  "staleness": [...],         // legacy summary
  "instances": [ ... ]       // enhanced per-instance view
}
```

### Queue Snapshot (`queue`)

| Field | Type | Description |
|-------|------|-------------|
| `depth` | integer | Current queue size |
| `dueWithinSeconds` | integer | Items scheduled within the next 12 seconds |
| `perType` | object | Counts per instance type, e.g. `{"pve":4}` |

### Dead-letter Snapshot (`deadLetter`)

| Field | Type | Description |
|-------|------|-------------|
| `count` | integer | Total items in the dead-letter queue |
| `tasks` | array | Top entries (legacy format; limited set) |

### Instances (`instances`)

Each element gives a complete view of one instance.

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Unique key `type::name` |
| `type` | string | Instance type (`pve`, `pbs`, `pmg`, etc.) |
| `displayName` | string | Friendly name (falls back to host/name) |
| `instance` | string | Raw instance identifier |
| `connection` | string | Connection URL or host |
| `pollStatus` | object | Recent poll outcomes |
| `breaker` | object | Circuit breaker state |
| `deadLetter` | object | Dead-letter insight for this instance |

#### Poll Status (`pollStatus`)

| Field | Type | Description |
|-------|------|-------------|
| `lastSuccess` | timestamp nullable | Most recent successful poll |
| `lastError` | object nullable | `{ at, message, category }` (`category` is `transient` or `permanent`) |
| `consecutiveFailures` | integer | Failure streak length |
| `firstFailureAt` | timestamp nullable | When the streak began |

#### Breaker (`breaker`)

| Field | Type | Description |
|-------|------|-------------|
| `state` | string | `closed`, `open`, `half_open`, or `unknown` |
| `since` | timestamp nullable | When current state began |
| `lastTransition` | timestamp nullable | Last transition time |
| `retryAt` | timestamp nullable | Scheduled retry time when applicable |
| `failureCount` | integer | Failures counted in the current breaker cycle |

#### Dead-letter (`deadLetter`)

| Field | Type | Description |
|-------|------|-------------|
| `present` | boolean | `true` if instance is in the DLQ |
| `reason` | string | `max_retry_attempts` or `permanent_failure` |
| `firstAttempt` | timestamp nullable | First time the instance hit DLQ |
| `lastAttempt` | timestamp nullable | Most recent DLQ enqueue |
| `retryCount` | integer | Number of DLQ attempts |
| `nextRetry` | timestamp nullable | Next scheduled retry time |

---

## Example Response

```json
{
  "updatedAt": "2025-10-20T13:05:42Z",
  "enabled": true,
  "queue": {
    "depth": 7,
    "dueWithinSeconds": 2,
    "perType": { "pve": 4, "pbs": 2, "pmg": 1 }
  },
  "deadLetter": {
    "count": 1,
    "tasks": [
      {
        "instance": "pbs-b",
        "type": "pbs",
        "nextRun": "2025-10-20T13:30:00Z",
        "lastError": "401 unauthorized",
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
      "score": 0.42,
      "lastSuccess": "2025-10-20T13:05:10Z",
      "lastError": "2025-10-20T13:05:40Z"
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
        "state": "half_open",
        "since": "2025-10-20T13:05:40Z",
        "lastTransition": "2025-10-20T13:05:40Z",
        "retryAt": "2025-10-20T13:06:15Z",
        "failureCount": 3
      },
      "deadLetter": {
        "present": false
      }
    },
    {
      "key": "pbs::pbs-b",
      "type": "pbs",
      "displayName": "Backup PBS",
      "instance": "pbs-b",
      "connection": "https://pbs-b:8007",
      "pollStatus": {
        "lastSuccess": "2025-10-20T12:55:00Z",
        "lastError": {
          "at": "2025-10-20T13:00:01Z",
          "message": "401 unauthorized",
          "category": "permanent"
        },
        "consecutiveFailures": 5,
        "firstFailureAt": "2025-10-20T12:58:30Z"
      },
      "breaker": {
        "state": "open",
        "since": "2025-10-20T13:00:01Z",
        "lastTransition": "2025-10-20T13:00:01Z",
        "retryAt": "2025-10-20T13:02:01Z",
        "failureCount": 5
      },
      "deadLetter": {
        "present": true,
        "reason": "max_retry_attempts",
        "firstAttempt": "2025-10-20T12:58:30Z",
        "lastAttempt": "2025-10-20T13:00:01Z",
        "retryCount": 5,
        "nextRetry": "2025-10-20T13:30:00Z"
      }
    }
  ]
}
```

---

## Useful `jq` Queries

### Instances with recent errors

```
curl -s http://HOST:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.pollStatus.lastError != null) | {key, lastError: .pollStatus.lastError}'
```

### Current dead-letter queue entries

```
curl -s http://HOST:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.deadLetter.present) | {key, reason: .deadLetter.reason, retryCount: .deadLetter.retryCount}'
```

### Breakers not closed

```
curl -s http://HOST:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.breaker.state != "closed") | {key, breaker: .breaker}'
```

### Stale instances (score > 0.5)

```
curl -s http://HOST:7655/api/monitoring/scheduler/health \
  | jq '.staleness[] | select(.score > 0.5)'
```

### Instances sorted by failure streak

```
curl -s http://HOST:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.pollStatus.consecutiveFailures > 0) | {key, failures: .pollStatus.consecutiveFailures}'
```

---

## Migration Notes

| Legacy Field | Status | Replacement |
|--------------|--------|-------------|
| `breakers` array | retains summary | use `instances[].breaker` for detailed view |
| `deadLetter.tasks` | retains summary | use `instances[].deadLetter` for per-instance enrichment |
| `staleness` array | unchanged | combined with `pollStatus.lastSuccess` gives precise timestamps |

The `instances` array centralizes per-instance telemetry; existing integrations can migrate at their own pace.

---

## Troubleshooting Examples

1. **Transient outages:** look for `pollStatus.lastError.category == "transient"` to confirm network hiccups; check `breaker.retryAt` to see when retries resume.
2. **Permanent failures:** `deadLetter.present == true` with `reason == "permanent_failure"` indicates credential or configuration issues.
3. **Breaker stuck:** `breaker.state != "closed"` with `since` > 5 minutes suggests manual intervention or rollback.
4. **Staleness spike:** compare `pollStatus.lastSuccess` with `updatedAt` to estimate data age; cross-reference `staleness.score` for alert thresholds.

Use Grafana dashboards for historical trends; the API complements dashboards by revealing instant state and precise failure context.
