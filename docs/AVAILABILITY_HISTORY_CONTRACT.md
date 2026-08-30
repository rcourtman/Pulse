# Availability History and Fleet View Contract

Status: Build-ready
Date: 2026-08-30
Scope: `pulse` only
Demand owner: `pulse-pro/FEATURE_REQUESTS.md`, "Fleet-scale machine
availability and service probes"

## Decision

Build source-owned availability observation history before adding a compact
fleet view. The first user-visible slice is a bounded recent state and latency
shape for the existing `network-endpoint` resources. It is not a second
availability-check inventory, a public status page, or an Uptime Kuma
configuration clone.

This order is required for honest output. The current poller keeps only the
latest `AvailabilityProbeStatus`; the browser cannot reconstruct a timeline
from resource refreshes. Pulse's numeric metrics rollups also cannot preserve
the difference between `reachable`, `unreachable`, `indeterminate`, and an
unobserved gap. A tile, sparkline, or uptime percentage built on either source
would invent evidence.

## Why now

The demand ledger records five independent signals: a fresh request from an
operator with roughly 20-50 targets, three public self-hosting threads where
Pulse is paired with a separate availability product or loses the combined
workflow to Zabbix, and an allowlisted aggregate telemetry read. On the clean
latest-ping basis, 339 of 6,578 persistent active installs had 2,449 configured
availability targets on 2026-08-29, up from 296 of 6,102 installs and 2,118
targets in the comparable prior week. Seventy installs already had at least
ten targets.

The category baseline is also clear without dictating Pulse's shape:

- [Gatus](https://github.com/TwiN/gatus) exposes endpoint uptime and response
  time history through its read API.
- [Uptime Kuma](https://github.com/louislam/uptime-kuma) covers a much broader
  set of monitor types and public status pages.

Pulse should meet the recent-history monitoring outcome, but keep its existing
attention-first resource model and resist the unsupported breadth and
status-page asks.

## Product outcome

An operator with tens of machine, service, and device checks can answer these
questions without opening each row:

1. Which targets need attention now?
2. Was this a brief transition or a sustained outage?
3. Is latency changing while the target remains reachable?
4. Does Pulse have enough observation coverage to support the conclusion?

The existing table remains the management and investigation surface. A compact
fleet mode is an alternate presentation over the same source-owned resources,
filters, and detail path.

## Observation contract

### Owner and identity

- The configured availability target ID owns history. Every configured check
  remains a distinct `network-endpoint`, including checks correlated to a VM,
  host, or device.
- Correlation may project the check's current facet onto another resource, but
  must not duplicate, move, or re-key its history.
- Renaming a target retains history. Deleting a target deletes its history.
- Execution-defining edits increment a non-secret configuration revision.
  Address, protocol, port, path, UDP policy, timeout, polling interval, and
  probe-agent assignment are revision-defining; display-name and resource-link
  edits are not. History responses expose revision boundaries so a chart does
  not imply one unchanged check across materially different configurations.

### One record per scheduled observation

Record only completed scheduled local polls and accepted assigned-agent
reports. Unsaved **Test** requests, stale-state synthesis, UI refreshes, and
resource projections never write history.

Each raw observation contains:

- target ID and configuration revision;
- closed outcome: `reachable`, `unreachable`, or `indeterminate`;
- observation time and server ingestion time;
- bounded latency in milliseconds when measured;
- the validity window derived from the effective polling/staleness contract;
- a closed execution source (`local` or `assigned_agent`), without an agent
  name, address, target address, or raw error.

For local probes, server-authored check time orders the timeline. For assigned
probes, server receipt time orders the timeline and determines coverage; the
agent's observation time remains evidence metadata but cannot author future
or backdated availability. Duplicate assigned-agent reports must be
idempotent.

`indeterminate` is first-class. In particular, silent UDP
`open_or_filtered` checks are neither reachable nor unreachable. A failed
probe is `unreachable` even before its alert failure threshold is met; the
threshold governs incident creation, not the observed endpoint state.

### Coverage and uptime

Each observation governs the interval from its authoritative timeline time to
the earlier of the next observation or its validity expiry. Time outside a
governed interval is `unknown`. Disabled periods, process downtime, missing
agent reports, and retention gaps therefore reduce coverage rather than
silently counting as healthy or failed.

For a requested window, return duration totals for:

- reachable;
- unreachable;
- indeterminate;
- unknown.

`coveragePercent` is `(reachable + unreachable + indeterminate) / window`.
`availabilityPercent` is `reachable / (reachable + unreachable)` and is
returned only when the window contains determinate time, and always beside
`coveragePercent`. Indeterminate and unknown time are not in the availability
denominator. The UI must not label this value an SLA, and must not show a
standalone percentage when known coverage is below 90%; show "insufficient
coverage" and the observed duration instead.

Latency summaries and lines use reachable observations only. Timeout duration
is not response latency, and an unreachable or indeterminate point must create
a state segment rather than a numeric zero or timeout-height latency point.

## Storage and rollup

Use a categorical availability-history owner backed by SQLite. It may share
the existing `metrics.db` lifecycle, permissions, writer serialization, and
retention scheduler, but it must not encode outcomes as ordinary numeric
metric rows: the current average/min/max rollup has no sample counts or gap
semantics and would lose the observation contract.

Raw observations roll up into buckets that retain, at minimum, state durations,
observation counts by outcome, unknown duration, and reachable-latency
count/sum/min/max. Rollups must preserve totals across raw -> minute -> hourly
-> daily transitions. The existing history access limits apply: Community 7
days, Relay 14 days, and Pro/eligible legacy plans 90 days. This capability
does not introduce a new entitlement. Assigned execution continues to require
the existing `external_probe` entitlement.

Retention and deletion must remove both raw and rolled-up rows. Stored history
must not contain target addresses, URLs, paths, agent names, raw probe errors,
certificate details, or customer/account identity.

## Read API

Expose one `monitoring:read` batch endpoint over target IDs. The exact route
may follow the existing metrics-history router, but the response contract is:

- at most 200 unique target IDs per request;
- an allowlisted range bounded by the active history entitlement;
- at most 120 chronological buckets per target for the fleet view;
- one summary and one state/latency series per target;
- per-target `not_found` / `forbidden` results without failing the whole batch;
- no N+1 store query per target.

Conceptual response shape:

```json
{
  "start": "2026-08-29T12:00:00Z",
  "end": "2026-08-30T12:00:00Z",
  "targets": [
    {
      "targetId": "opaque-target-id",
      "summary": {
        "reachableSeconds": 82320,
        "unreachableSeconds": 1800,
        "indeterminateSeconds": 0,
        "unknownSeconds": 2280,
        "coveragePercent": 97.36,
        "availabilityPercent": 97.86,
        "reachableLatencyMillis": { "average": 18, "min": 8, "max": 92 }
      },
      "buckets": [
        {
          "start": "2026-08-30T11:48:00Z",
          "end": "2026-08-30T12:00:00Z",
          "reachableSeconds": 720,
          "unreachableSeconds": 0,
          "indeterminateSeconds": 0,
          "unknownSeconds": 0,
          "latencyMillis": { "average": 17, "min": 12, "max": 24 }
        }
      ],
      "revisionBoundaries": []
    }
  ]
}
```

All percentages are derived server-side from duration totals. Clients do not
recalculate them from rounded buckets.

## Fleet presentation

Add a URL-owned `table` / `fleet` view choice to **Machines -> Availability**.
Keep search and health filters shared between both views and retain the current
attention ordering: active failures first, then stale/indeterminate checks,
then healthy checks.

Each compact tile shows:

- target name and current status;
- method and local/assigned-probe source;
- checked time and current reachable latency, when available;
- a bounded 24-hour state strip with unknown and indeterminate visually
  distinct from down;
- a latency line over reachable segments only;
- coverage-aware availability text.

Selecting a tile opens the existing resource detail. Configuration stays in
the existing Settings flow. The table remains the default until fleet mode has
rendered-browser evidence at 50 targets on laptop and phone widths; after that,
preference may persist locally without changing shared URLs.

## Non-goals for this slice

- DNS, keyword/JSON, push, game-server, or scripted monitors.
- Public or authenticated status pages.
- Incident communications or SLA reporting.
- Replacing notification policy, failure thresholds, or maintenance intent.
- Per-view browser polling or a second target inventory endpoint.
- Backfilling history from alert events, current `LastSuccess`, or resource
  snapshots.

## Release gates

The history slice is complete only when tests prove:

1. local polls and assigned-agent reports produce identical state semantics;
2. agent clock skew cannot move coverage into the future or past;
3. duplicate reports are idempotent and restarts retain history;
4. UDP indeterminate time is never counted as reachable or unreachable;
5. disabled periods, process downtime, and missing remote reports become
   unknown coverage;
6. configuration revisions produce explicit boundaries;
7. retention, entitlement limits, and target deletion cover raw and rollups;
8. rollups preserve state-duration and latency aggregates;
9. a 200-target batch read is bounded and performs no target-by-target query;
10. stored rows and API responses contain no raw errors or target/agent
    addresses beyond the separately authorized current target inventory.

The fleet slice additionally requires a rendered-browser contract at 50
targets, keyboard access to every tile, non-color state labels, and proof that
an API failure renders "history unavailable" without changing current target
health.
