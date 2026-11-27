# 🩺 Scheduler Health API

**Endpoint**: `GET /api/monitoring/scheduler/health`
**Auth**: Required. Use a signed-in Pulse browser or an API token in a private
header file; do not copy a session cookie or token into a command.

Returns a snapshot of polling, including queue state, circuit breakers, and
dead-letter tasks. `enabled: false` means adaptive timing is off. Configured
connections can still poll at fixed intervals, and their queue and results still
appear here. Neither `enabled` nor the queue depth is a health verdict.

## 📦 Response Format

```json
{
  "updatedAt": "2025-10-20T13:05:42Z",
  "enabled": true,
  "queue": {
    "depth": 7,
    "dueWithinSeconds": 2,
    "perType": { "pve": 4, "pbs": 3 },
    "oldestOverdueSeconds": 90
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
      "score": 0.12
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
The scheduler's record of each connection's recent poll results.

*   **`pollStatus`**: `lastSuccess` timestamp, the current outstanding `lastError` if present, and `consecutiveFailures`. A complete success clears the error. A partial read can record both a success and a `partial` error with no consecutive failures. Proxmox VE startup and reconnect construction failures populate `pollStatus`, breaker state, and staleness tracking immediately. TrueNAS and VMware pollers drop explicit shutdown cancellations without fabricating poll failures while still reporting genuine poll timeouts, and fence in-flight completions against active provider ownership so deleted, paused, or unreadable-config fail-closed connections cannot resurrect cached state or overwrite a replacement provider.
*   **`breaker`**: `state` (`closed`/`open`/`half_open`), `retryAt` next retry, `since` state start, `lastTransition` timestamp.
*   **`deadLetter`**: `present` flag, `reason` (e.g., `permanent_failure`),
    `retryCount` and the recorded `nextRetry`. This time does not schedule a
    poll. When a later scheduled result succeeds, Pulse removes the terminal
    entry. A partial read can clear that entry and still leave a
    `pollStatus.lastError` with category `partial`.

### Top-Level Queue and DLQ
*   **`queue`**: Snapshot of the active task queue (depth + per-type counts).
    `dueWithinSeconds` counts tasks due now or within 12 seconds. It is not a
    duration. `oldestOverdueSeconds` is how far past its scheduled time the
    most overdue timestamped poll still in the queue was at capture. Zero
    means none was past its scheduled time. It does not count future work
    (including a future scheduled retry), claimed work or dead-letter entries.
    Deleting a saved availability check retires its queued poll on catalogue
    refresh without disturbing unrelated providers, while a failed catalogue
    read leaves existing scheduled work unchanged.
*   **`deadLetter`**: Aggregate DLQ summary plus up to 25 queued tasks.

### Optional Summaries
*   **`breakers`**: Only breakers that are not in default `closed`/zero-failure state.
*   **`staleness`**: Snapshot of staleness scores (if the tracker is enabled).

## 🛠️ Common Queries (jq)

For a one-off check, open `/api/monitoring/scheduler/health` in your signed-in
browser on that same Pulse instance. For curl, first prepare the private header
file from [API authentication](../API.md#api-token-recommended), using a token
with only the access needed for your monitoring task. The recipe requires curl
7.55 or later and jq. It makes one read-only request and keeps its response in
a new private directory:

```bash
(
  set -eu
  umask 077
  snapshot_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulse-scheduler.XXXXXX")"
  snapshot="$snapshot_dir/health.json"
  status="$(curl --disable --silent --connect-timeout 5 --max-time 15 \
    --header "@$HOME/.config/pulse/api-header" --output "$snapshot" \
    --write-out '%{http_code}' http://127.0.0.1:7655/api/monitoring/scheduler/health)" || {
    printf 'Scheduler snapshot unavailable (curl exit %s). No poll result.\n' "$?" >&2
    exit 1
  }
  case "$status" in
    200) ;;
    401|403)
      printf 'Scheduler snapshot unavailable (HTTP %s). Check access to this Pulse instance.\n' "$status" >&2
      exit 1 ;;
    *)
      printf 'Scheduler snapshot unavailable (HTTP %s). No poll result.\n' "$status" >&2
      exit 1 ;;
  esac
  if ! jq -es 'length == 1 and (.[0] | type == "object" and
    (.updatedAt | type == "string") and (.enabled | type == "boolean") and
    (.deadLetter.count | type == "number") and (.queue.depth | type == "number") and
    (.instances | type == "array"))' "$snapshot" >/dev/null 2>&1; then
    printf 'Scheduler snapshot unavailable. The response was not a scheduler snapshot.\n' >&2
    exit 1
  fi
  printf 'Saved scheduler snapshot: %s\n' "$snapshot"
)
```

The loopback URL is only for curl running on the Pulse host. Elsewhere, use your
Pulse HTTPS URL with certificate verification enabled, never `--insecure`.
Keep `--disable` first to ignore curl configuration that could enable trace
output. A 401 or 403 is an access failure on this request, not a poll failure.
A redirect, other non-200 status, curl error or unexpected response leaves no
snapshot to compare. Do not use a partial file or treat it as an empty queue.
Only continue if the recipe prints **Saved scheduler snapshot**. Do not enable
verbose or trace output or share the header file.

Use the printed filename in the following local queries. They make no further
requests. Keep the raw response private: it includes infrastructure names,
addresses and error text; redact these before sharing selected observations.
`updatedAt` is the snapshot time, not the time of the last successful poll.
Use the per-connection timestamps to check for progress. Do not enable adaptive
polling or restart Pulse just to collect this evidence.

**Summarise the Snapshot:**
```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '{updatedAt, enabled, queueDepth: .queue.depth, deadLetterCount: .deadLetter.count,
  oldestQueuedOverdueSeconds: .queue.oldestOverdueSeconds,
  instanceCount: (.instances | length)}' "$snapshot"
```

**Find incomplete and failed polls:**

A poll can record a new `lastSuccess` and still leave a `partial` error.
Pulse reached the connection, but some readings were unavailable. The
consecutive failure count is zero in this case. Check which read failed in
your private snapshot rather than treating the whole connection as down.

```bash
snapshot='/absolute/path/to/health.json' # use the printed filename
jq '.instances[] | select(.pollStatus.lastError != null or .pollStatus.consecutiveFailures > 0) |
  {key, lastSuccess: .pollStatus.lastSuccess, failures: .pollStatus.consecutiveFailures,
   lastErrorAt: .pollStatus.lastError.at, lastErrorCategory: .pollStatus.lastError.category}' "$snapshot"
```

No rows means the listed connections have no recorded poll error or consecutive
failure in this snapshot. It does not show that their polls have run. Compare
the last-success timestamps over the connection's normal poll interval.

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

### Compare two poll snapshots

Use two snapshots from the same running Pulse instance. Allow enough time for
the connection's normal poll interval and any retry delay. If a snapshot
request times out, keep that result and stop rather than repeatedly requesting
it. Put the earlier capture first. This local query joins the saved results by
connection key even if the row order changes. It leaves out addresses and error
messages. Keep its output private too: the connection keys can include names.

```bash
earlier='/absolute/path/to/earlier/health.json'
later='/absolute/path/to/later/health.json'
jq -ne --slurpfile earlier "$earlier" --slurpfile later "$later" '
  def snapshot:
    if length == 1 and (.[0] | type == "object" and
      (.updatedAt | type == "string" and length > 0) and
      (.enabled | type == "boolean") and
      (.instances | type == "array" and all(.[];
        (.key | type == "string" and length > 0) and (.pollStatus | type == "object"))))
    then .[0] else error("Expected one saved scheduler snapshot per file") end;
  def by_key:
    INDEX(.instances[]; .key) as $rows |
    if ($rows | length) == (.instances | length)
    then $rows else error("Duplicate connection key in a snapshot") end;
  def poll:
    if . == null then null else .pollStatus |
      {lastSuccess, lastErrorAt: .lastError.at, lastErrorCategory: .lastError.category,
       consecutiveFailures}
    end;
  def retry_context:
    if . == null then null else
      {breaker: {state: .breaker.state, retryAt: .breaker.retryAt},
       deadLetter: {present: .deadLetter.present, nextRetry: .deadLetter.nextRetry}}
    end;
  ($earlier | snapshot) as $a | ($later | snapshot) as $b |
  if $a.updatedAt == $b.updatedAt then error("Use two different capture times") else . end |
  ($a | by_key) as $old | ($b | by_key) as $new |
  {earlier: ($a | {updatedAt, enabled}), later: ($b | {updatedAt, enabled}),
   oldestQueuedOverdueSeconds: {earlier: $a.queue.oldestOverdueSeconds, later: $b.queue.oldestOverdueSeconds},
   polls: [($old + $new | keys[]) as $key |
     ($old[$key] | poll) as $before | ($new[$key] | poll) as $after |
     {key: $key, before: $before, after: $after,
      retry: {before: ($old[$key] | retry_context), after: ($new[$key] | retry_context)},
      changes: (if $before == null or $after == null then null else
        [$before | keys[] as $field | select(.[$field] != $after[$field]) | $field]
      end)}]}'
```

Check the two `updatedAt` times before using the result. In each row, `changes`
names the fields that differ. It does not judge whether a timestamp advanced.
An empty list means the recorded fields are unchanged. A null `before` or
`after` means the connection was absent from that capture, not that its
poll succeeded.

An increasing `oldestQueuedOverdueSeconds` shows the queue has work further
past its scheduled time, even if the task counts did not change. It does not
show that the same task was late in both snapshots, or identify a connection
or cause. `null` means the capture did not include this measurement (for
example, from an older Pulse build). Zero does not rule out a poll already
taken by a worker. If you have Prometheus, check the separate
[claimed-poll stages](../monitoring/PROMETHEUS_METRICS.md#-scheduler-health).

If no result advanced, read `retry.after` beside the later snapshot's
`updatedAt`. For an `open` or `half_open` breaker, `retryAt` is the earliest
time the breaker would permit another attempt, not a scheduled start. After
that time the breaker may still show the same state until work reaches it.
If `deadLetter.present` is true, the scheduler has recorded this connection
in its dead-letter queue. Older builds may keep the entry after recovery.
A null `nextRetry` means this snapshot has no recorded retry time for it.
A retry timestamp changing or passing does not mean a poll ran.
Only the poll fields are included in `changes`. If neither result moved
after the normal polling interval and any recorded wait, keep this as a
gap in recorded results. Don't reset the breaker or retry just to check it.

| What changes | What it tells you |
| --- | --- |
| `lastSuccess` appears or advances | Pulse recorded a successful poll. A new `partial` error can still mean some readings were unavailable. |
| `lastErrorAt` appears or advances without a newer success | Pulse is still recording failures for that connection. Check the error in your local snapshot. |
| `lastSuccess` moves backwards or disappears | Check the capture order and whether Pulse restarted or its clock changed. This is not a newly recorded success. |
| Neither advances during an expected poll window | You have no new recorded result. A slow poll, a pause or a wait for retry can look this way. Do not assume the target is offline. |

A missing success timestamp is not a fresh reading. A complete success can
also clear an outstanding error. Check agent reports and alert evaluation
separately. If you already scrape Prometheus, compare
[completed poll outcomes](../monitoring/PROMETHEUS_METRICS.md#check-whether-polling-is-still-running)
alongside these snapshots.

If poll results advance but a chart does not, compare
[Bars, Trends and drawer History](../monitoring/METRICS_DATA_FLOW.md#if-the-views-disagree).
They do not share the same read, and a successful poll does not prove a
particular metric was stored or that the browser received it.
