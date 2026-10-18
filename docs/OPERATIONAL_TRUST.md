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

1. Record the running version, edition and deployment configuration. Keep a
   consistent private backup of the actual data directory, matching encryption
   and audit signing keys, and any external stores; use the
   [full-state recovery guidance](MIGRATION.md#full-state-recovery), not a copy
   of a live database file or a configuration-only export.
2. Check the existing service account and persistent mounts through the normal
   deployment configuration. Do not broaden permissions or replace keys to
   make an upgrade check pass.
3. Keep independent monitoring and alert coverage available during the update.
   For affected freeze-enabled Proxmox backups, preserve the separate
   [guest-safety precaution](VM_DISK_MONITORING.md#backup-safety).
4. Keep the previous binary or image and its matching data backup until
   recovery is checked. Reverting a binary does not reverse data migrations;
   follow the deployment's [update and rollback guidance](AUTO_UPDATE.md#rollback).

### Read-only checks after upgrading

Use an existing authenticated browser session and ordinary collection. These
checks inspect existing evidence; they do not require a new incident or a
change to a workload, notification queue or alert lifecycle.

1. Check the running server version and edition, then the normal connection
   health and observation times. Compare the same connection, resource and time
   range before and after the upgrade; reused names or VMIDs are not identity.
2. Compare the Patrol navigation count with the active queue using the same
   organisation, access scope and filters. Open an existing item, if present,
   and inspect its resource, lifecycle, evidence and protection detail. An
   empty queue does not establish complete or healthy collection.
3. Confirm that retained evidence still shows its observation time and any
   stale, denied, partial or unknown state. Do not disconnect a collector or
   induce a fault to obtain a test case. A successful task or action is not
   fresh detector evidence of recovery, guest thaw or a tested restore.
4. Inspect **Recent delivery activity** in Alert History and existing failed
   delivery details, if any. Compare the original alert, queued attempt and
   recipient's actual message separately; Test success does not establish
   ordinary, grouped or resolved delivery. Use the
   [notification checks](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
   for the actual symptom.
5. If using Pro actions, inspect existing execution and verification records
   separately. A successful execution does not establish verified recovery;
   an unavailable action offer is not by itself a monitoring failure.

**Do not acknowledge, suppress, retry, dismiss or clear records, send a Test,
or approve or run an action merely to validate an upgrade.** Retry can send a
real notification, suppression changes active attention, and an offered Docker
restart changes a workload. These controls remain available for intentional
operator work, not passive diagnostics. If a check needs an event that has not
occurred, record it as unverified rather than manufacture one. Exercise those
paths only in an isolated non-production fixture with synthetic destinations
and disposable workloads, never against live resources.

Keep any existing metrics listener private; reading through an already
protected scraper is optional, not a reason to expose a new listener or widen
network access. Keep authentication, TLS verification and least-privilege
collection unchanged. Preserve errors and existing delivery/action records;
share only the affected view, version, time and locally redacted explanation,
not credentials, full inventories, notification payloads or Debug exports. See
[safe issue reporting](TROUBLESHOOTING.md#-getting-help).

If a migration fails, stop the upgraded process, preserve the data directory
and private logs, and use the deployment's recovery procedure with the prior
release and its matching consistent pre-upgrade data backup. A failed read or
an empty queue alone is not a reason to restore data. Do not delete lifecycle,
evidence, notification, recovery or action records to force startup.
