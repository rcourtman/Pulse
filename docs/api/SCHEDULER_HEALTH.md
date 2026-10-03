# 🩺 Scheduler Health API

**Endpoint**: `GET /api/monitoring/scheduler/health`
**Auth**: Required. Use a signed-in Pulse browser or an API token in a private
header file; do not copy a session cookie or token into a command.

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
        "state": "half_open",
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

*   **`pollStatus`**: `lastSuccess` timestamp, `lastError` details, `consecutiveFailures` count.
*   **`breaker`**: `state` (`closed`/`open`/`half_open`), `retryAt` next retry, `since` state start, `lastTransition` timestamp.
*   **`deadLetter`**: `present` flag, `reason` (e.g., `permanent_failure`), `retryCount`, `nextRetry` if scheduled.

### Top-Level Queue and DLQ
*   **`queue`**: Snapshot of the active task queue (depth + per-type counts).
*   **`deadLetter`**: Aggregate DLQ summary plus up to 25 queued tasks.

### Optional Summaries
*   **`breakers`**: Only breakers that are not in default `closed`/zero-failure state.
*   **`staleness`**: Snapshot of staleness scores (if the tracker is enabled).

## 🛠️ Common Queries (jq)

For a one-off check, open `/api/monitoring/scheduler/health` in your signed-in
browser on that same Pulse instance. For curl, first prepare the private header
file from [API authentication](../API.md#api-token-recommended), using a token
with only the access needed for your monitoring task. These commands require
curl 7.76 or later and jq. They make one read-only request and keep its response
in a new private directory:

```bash
(
  set -eu
  umask 077
  snapshot_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulse-scheduler.XXXXXX")"
  snapshot="$snapshot_dir/health.json"
  status="$(curl --disable --fail-with-body --connect-timeout 5 --max-time 15 \
    --header "@$HOME/.config/pulse/api-header" --output "$snapshot" \
    --write-out '%{http_code}' http://127.0.0.1:7655/api/monitoring/scheduler/health)"
  if [ "$status" != 200 ]; then
    printf 'Scheduler snapshot unavailable (HTTP %s); no health conclusion.\n' "$status" >&2
    exit 1
  fi
  jq -e 'type == "object" and (.updatedAt | type == "string") and
    (.enabled | type == "boolean") and (.deadLetter.count | type == "number") and
    (.queue.depth | type == "number") and (.instances | type == "array")' \
    "$snapshot" >/dev/null
  printf 'Saved scheduler snapshot: %s\n' "$snapshot"
)
```

The loopback URL is only for curl running on the Pulse host. Elsewhere, use your
Pulse HTTPS URL with certificate verification enabled, never `--insecure`.
Keep `--disable` first to ignore curl configuration that could enable trace
output. HTTP errors, transport failures and unexpected response shapes stop the
recipe; none is a healthy or empty scheduler result. A 401/403 concerns access,
not polling health. Do not use verbose/trace output or share the header file.

Use the printed filename in the following local queries. They make no further
requests. Keep the raw response private: it includes infrastructure names,
addresses and error text; redact these before sharing selected observations.
Check `enabled` first: a disabled scheduler's empty queue is not a health pass.
`updatedAt` is the snapshot time; per-instance `pollStatus.lastSuccess` records
the last successful poll. Compare freshness with the effective poll intervals,
not a universal queue-depth threshold.

**Summarise the Snapshot:**
```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '{updatedAt, enabled, queueDepth: .queue.depth, deadLetterCount: .deadLetter.count,
  instanceCount: (.instances | length)}' "$snapshot"
```

**Find Failing Instances:**
```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '.instances[] | select(.pollStatus.consecutiveFailures > 0) |
  {key, lastSuccess: .pollStatus.lastSuccess, failures: .pollStatus.consecutiveFailures}' "$snapshot"
```

**Check Dead Letter Queue:**
```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '.instances[] | select(.deadLetter.present) |
  {key, reason: .deadLetter.reason}' "$snapshot"
```

**Find Open Breakers:**
```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '.instances[] | select(.breaker.state != "closed") |
  {key, state: .breaker.state}' "$snapshot"
```
