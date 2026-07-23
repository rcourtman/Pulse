# Operational Trust

Operational Trust is Pulse v6's shared model for answering five operator
questions:

1. What needs attention now?
2. What evidence supports that conclusion?
3. Is the affected resource protected by usable recovery history?
4. What changed in the issue lifecycle?
5. Can Pulse offer a narrow action, and did fresh evidence verify its result?

Alerts, the Patrol attention queue, navigation counts, attached availability
checks, notifications, protection posture, and governed actions project the
same canonical records. None of those views owns a second writable lifecycle.

## Lifecycle states

| State | Operator meaning |
| :--- | :--- |
| `observing` | Evidence is being confirmed. This is not yet active work. |
| `open` | Current evidence supports an operational issue. |
| `acknowledged` | An operator has seen the issue. The issue is not resolved. |
| `suppressed` | The issue is temporarily removed from active attention with an actor, reason, and bounded expiry. |
| `resolving` | Recovery evidence exists, but the detector has not yet confirmed normal health. |
| `resolved` | Fresh detector evidence confirms that the issue no longer applies. |
| `stale` | The prior issue remains relevant, but its collection evidence is no longer current. |
| `unknown` | Permissions, completeness, provider state, or identity prevent a stronger conclusion. |

Missing observations never resolve an open record. Collector disconnects move
existing work to `stale`; permission or provider uncertainty moves it to
`unknown`. A successful action result also does not close a record. Only fresh
detector evidence can confirm recovery.

Acknowledgement is reversible and does not reduce the active issue truth.
Suppression requires a non-empty reason and an expiry no more than 30 days in
the future. The Patrol UI offers shorter 1-hour, 24-hour, and 7-day choices by
default.

## Evidence

Every evidence envelope records:

- a stable opaque evidence ID;
- provider, collector, and optional provider instance;
- one canonical resource ID or one unresolved provider-scoped reference;
- observation, ingestion, and optional validity times;
- completeness, confidence, and permission state;
- an optional bounded payload reference and identity-correlation proof.

`fresh`, `complete`, and `confirmed` are independent dimensions. Partial,
denied, unavailable, stale, inferred, ambiguous, and unknown evidence is
represented explicitly and never upgraded to healthy by a client.

The attention detail response contains the retained envelopes needed for the
normal operator journey. An authorized client can request one exact envelope
through the evidence endpoint. If the record still links the evidence ID but
its bounded detail has expired, the endpoint returns `410
attention_evidence_detail_expired`; it does not pretend the evidence never
existed.

## Protection posture

Protection posture is evaluated server-side per canonical subject resource:

- `protected`: current usable protection satisfies policy;
- `attention`: protection exists but freshness, verification, or provider
  outcome needs attention;
- `unprotected`: sufficient evidence confirms that required protection is
  absent;
- `unknown`: identity, history, permissions, or collection coverage cannot
  support a stronger claim.

The response explains the conclusion, preserves provider-specific state, and
links the recovery and repository resources involved. Platform tables fetch
posture in batches of at most 200 resource IDs. They must not issue one network
request per row.

## Availability

An availability check attaches to an existing unified resource only when an
explicit link resolves or identity correlation yields exactly one candidate.
The relationship carries a stable relationship ID and the evidence ID that
supports it. Every configured check remains a source-owned inventory resource,
including an attached check. Correlation additionally projects its bounded
facet onto the matched platform row and detail; it never replaces the check
identity or copies the check's incident and service identity into the machine.
Ambiguous and unresolved checks remain visible with their typed correlation
state.

Availability success is time-bounded. A stale successful observation is
`stale`, not healthy. A failure enters the same alert lifecycle and Patrol
attention queue as other detectors.

## Patrol workflow

The normal operator path is:

1. Open Patrol from the monitor shell.
2. Review the urgency-ordered active queue.
3. Select an item for impact, next step, resource, evidence, protection, and
   lifecycle detail.
4. Acknowledge it, or temporarily suppress it with a reason and expiry.
5. If an eligible Pulse Pro action is offered, review the server-owned plan,
   approve it, run it, and inspect execution and verification separately.

A calm state appears only when the lifecycle evaluation succeeded, coverage is
current, and no active item exists. A failed read or partial coverage never
becomes a calm claim.

The first governed action is a Docker container restart. It is offered only
for a uniquely identified container with fresh confirmed unhealthy evidence,
declared executor readiness, the required authorization scope, and the
`ai_autofix` entitlement. The action framework owns plan hashing, approval,
idempotent execution, durable audit, restart reconciliation, and verification.

## API and authorization

Read routes require `monitoring:read`:

- `GET /api/ai/patrol/attention`
- `GET /api/ai/patrol/attention/summary`
- `GET /api/ai/patrol/attention/{id}`
- `GET /api/ai/patrol/attention/{id}/evidence/{evidenceId}`

Lifecycle mutations require `monitoring:write`:

- `POST /api/ai/patrol/attention/{id}/acknowledge`
- `POST /api/ai/patrol/attention/{id}/unacknowledge`
- `POST /api/ai/patrol/attention/{id}/suppress`
- `POST /api/ai/patrol/attention/{id}/unsuppress`

Suppression body:

```json
{
  "reason": "Planned host maintenance",
  "expiresAt": "2026-07-20T08:00:00Z"
}
```

Planning an offered restart requires the action scopes enforced by the
canonical action API and an active `ai_autofix` entitlement:

```text
POST /api/ai/patrol/attention/{id}/actions/restart/plan
```

Action decision, execution, detail, and audit use `/api/actions`.

All IDs are opaque. Clients must path-escape operational-record and evidence
IDs because canonical IDs can contain `/`, `:`, and provider-specific
segments.

## Metrics

The `/metrics` listener exposes Operational Trust counters and histograms under
`pulse_operational_trust_*`. Labels use closed, low-cardinality vocabularies;
resource IDs, evidence IDs, provider-instance names, actors, and destination
IDs never appear as labels.

Useful alerts include:

- sustained growth in `active_count_mismatch_total`;
- notification `failed` or `dead_letter` outcomes;
- growing stale, unavailable, denied, or partial evidence observations;
- identity `ambiguous` or `unresolved` outcomes;
- action verification `contradicted`, `inconclusive`, or `timed_out` outcomes.

## Upgrade and compatibility

Operational Trust migrations are additive. Existing alert, notification,
recovery, relationship, availability, and action records are normalized on
read or migrated in their owning stores. Read-side compatibility fields remain
supported where older clients need them, but new writes go only through the
canonical lifecycle, recovery, unified-resource, notification, and action
owners.

The Pulse Mobile primary backlog uses `/api/ai/patrol/attention` and preserves
operational record, evidence, and action-verification identity. Its old finding
shape is now a local display adapter, not a writable source of truth.

Before upgrading:

1. back up the Pulse data directory;
2. confirm that the v6 process can write its alert, notification, recovery, and
   action database directories;
3. confirm supported clients can read additive JSON fields;
4. expose the metrics listener to a protected scraper if rollout telemetry is
   required;
5. verify Pulse Pro entitlement connectivity before relying on action offers.

After upgrading:

1. confirm the Patrol navigation count matches the active queue;
2. inspect an active item through its deepest evidence and protection detail;
3. acknowledge and unacknowledge a test item;
4. verify a bounded suppression returns to active attention;
5. confirm stale collection remains visible rather than resolving;
6. exercise notification retry/dead-letter monitoring;
7. if using Pulse Pro actions, complete a review/approve/run/verify journey.

If a migration fails, stop the upgraded process, preserve the data directory
and logs, and restore the prior release with the pre-upgrade data backup. Do
not delete lifecycle, evidence, notification, recovery, or action records to
force startup.
