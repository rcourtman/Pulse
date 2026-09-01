# Alert Engine Evolution

Status: IMPLEMENTED (phases 0–3 engine side complete 2026-08-27: `internal/alerts/alert_policy.go`, `history_projection.go`, `lifecycle_contract_test.go`; the rules-first UI migration stays deferred and demand-gated through FEATURE_REQUESTS.md, as recorded under Phase 3 below). Last reviewed 2026-09-01.
Date: 2026-08-26
Scope: `pulse` only
Predecessor: `docs/CANONICAL_ALERT_ENGINE_MIGRATION_2026-03-10.md`

## Purpose

The March 2026 canonical migration correctly rejected a full alert-engine
rewrite and began moving alert *inputs* onto canonical identity
(`internal/unifiedresources`) and specs (`internal/alerts/specs`). It
deliberately froze two layers: transition state stays in the manager's
tracking maps, and notification-suppression rules stay where they are.

The post-March issue record shows the recurring regression classes live in
exactly those frozen layers: config saves emitting resolved notifications
(#1682), grouping silently dropping N−1 of N alerts (#1683), resolves sent
for suppressed firings (#1553), self-resolving fire/resolve churn deleting
history (#1693), chronic re-fires on terminal container state (#1724).
Cleaner inputs feeding the same fragile core does not retire these classes.

This document extends the migration's end-state one layer deeper. The
mechanism stays the same — strangler migration inside one engine, no
big-bang rewrite — applied to the transition core and the suppression path.

## Target model (delta over the March doc)

The March doc's layers 1 (unified resource identity), 2 (canonical specs),
and 5 (notification fan-out boundary in `monitor_alerts.go`) stand
unchanged. Two layers are added or replaced:

### Event log (new, Phase 0)

An append-only alert event log, SQLite-backed, owned by `internal/alerts`.
Every lifecycle transition (pending, firing, resolved, acknowledged,
escalated) and every notification decision — including suppressions, with
the mechanism and reason that held them — is an event. History, the
delivery log, frequency analytics, and `AlertDeliveryDiagnosis` become
projections of one log instead of separately maintained structures.
Transparency stops being a feature and becomes a property of the
architecture.

Phase 0 is strictly additive: the existing manager writes events alongside
its current behavior. No lifecycle semantics change.

### Incident reducer (replaces the frozen transition core, Phases 1–2)

A pure transition function — `next(state, signal, rules, clock) →
(state', events)` — replaces the manager's string-keyed tracking maps.
Identity is computed in exactly one place (canonical resource ID + spec
ID); lifecycle is an explicit state machine; acknowledgement, snooze, and
flapping are typed fields on one incident record. Because the reducer is
pure, the lifecycle contract suite (config save ⇒ zero events; suppressed
firing ⇒ no resolve; N grouped ⇒ N rendered) runs exhaustively and gates
releases.

The March doc's "stable behaviors that must not change" list is the seed
of the reducer's characterization suite: ack preservation, cooldown
re-notify, escalation carry-over, and confirmation semantics are pinned by
parity tests against the live manager before any cutover, not rewritten
from memory.

## Phases

- **Phase 0 — event log, additive.** Event store + manager wiring for
  transition and suppression events + API + UI surfacing of the existing
  delivery diagnosis and held-notification rows. Zero lifecycle risk.
  Registered as coverage gap `alert-engine-suppression-observability`.
- **Phase 1 — reducer in shadow mode.** Built beside the manager, fed the
  same inputs, outputs diffed continuously (the
  `unified_eval_parity_test.go` pattern). No user-visible change.
- **Phase 2 — family-by-family cutover.** Worst identity offenders first
  (Docker, PBS/storage). Each family cutover deletes its tracking maps and
  the imperative bulk of its `Check*` path. Alert IDs and override keys
  stay stable until an explicit config migration.
  **Status: complete (2026-08-27).** All four per-observation transition
  families run on the reducer core as the authoritative state:
  match-spec lifecycle (`evaluateCanonicalLifecycleAlert`), legacy
  `checkMetric`, canonical metric threshold, and the stateful family.
  The legacy tracking maps (`offlineConfirmations`,
  `offlineRecoveryConfirmations`, `nodeOfflineCount`,
  `connectionDegradedCount`, `dockerOfflineCount`, `dockerStateConfirm`,
  `pendingAlerts`) are deleted; their cleanup-loop hygiene moved into the
  core (`PruneStalePending`, pending reaping on container removal,
  healthy observations from the online handlers).
  *Deliberately scoped out:* the unified-incidents reconciler
  (`unifiedIncident*` in the manager). It is a batch reconciliation
  family, not a per-observation confirmation run — its provider hands it
  complete incident sets and its `firstSeen` bookkeeping is already
  identity-correct — so forcing it through the per-signal reducer would
  add translation without retiring a defect class. It stays a follow-on:
  if the event log becomes the sole history authority (Phase 3+), it
  should emit reducer-shaped events at that boundary instead.
- **Phase 3 — declarative rule model.** Scope selector + condition +
  policy replaces the per-platform config blocks and `DisableAll*`
  booleans, with a translator from the existing `AlertConfig` so persisted
  user configs keep working. UI migrates tab by tab.
  **Status: engine side complete (2026-08-27).** The effective alert
  policy for a resource is answered by one ordered fold
  (`internal/alerts/alert_policy.go`): type default block → the type's
  `DisableAll` switches → custom rules (guest-scoped, priority order) →
  the per-resource override through the identity-aware lookup for the
  kind. The persisted `AlertConfig` is the translator's input and keeps
  its shape; the engine no longer reads it piecemeal — every scattered
  `DisableAll*` read and threshold lookup routes through the fold, pinned
  by characterization tests against the legacy resolution before any call
  site moved.
  *UI migration: deliberately deferred, demand-gated.* The per-platform
  thresholds presentation is what users ask in — "VM CPU threshold", not
  "scope selector" — and the demand ledger holds no signal for a
  rules-first editing surface (the nearest entry warns against bolting
  schedules onto alert rules). The tabs now sit on a single resolution
  surface, so a rules-first UI, an effective-policy inspector ("why is
  this alert off?"), or both can be built without further engine work
  when a signal lands. Re-scope through
  `repos/pulse-pro/FEATURE_REQUESTS.md` at that point.

## What is kept

The delivery pipeline (`internal/notifications` queue, DLQ, receipts,
templates, delivery health), the operator-intent fabric, unified
resources, the specs/evaluator layer, the UI shell, and the
`monitor_alerts.go` fan-out boundary — which becomes an event consumer.

## What is retired (by the end of Phase 2–3)

The manager's per-family tracking maps, the imperative per-platform
check bodies, the scattered suppression checks, and — once the event log
is authoritative — the JSON snapshot history file.

Status (2026-08-27): the tracking maps are deleted; the check bodies'
imperative transition bulk is replaced by the reducer core (what remains
of them is evidence assembly for the spec evaluator); suppression
decisions emit typed, reasoned events through the Phase 0 log; policy
reads go through the Phase 3 fold.

Post-plan hardening (2026-08-27, same-day follow-through):

- **The lifecycle contract suite is a release gate.** Engine contracts
  (config-save silence, resolve-exactly-once, ack halts escalation,
  acknowledged recovery suppressed) in
  `internal/alerts/lifecycle_contract_test.go`; grouped-rendering
  contracts (N grouped ⇒ N rendered on email, apprise, and every
  webhook service template; cancel-before-delivery holds the recovery)
  in `internal/notifications/grouped_rendering_contract_test.go`.
- **The event log is the alert history authority.** Lifecycle events
  carry full alert snapshots; history is projected from the log
  (`history_projection.go`), the legacy JSON entries migrate in once
  and the files retire to `*.imported`, clears are append-only
  tombstones, and every alert family — reconciler and system alerts
  included — records its fired event. The JSON snapshot history file is
  retired. Parity with the JSON model is pinned by
  `history_projection_parity_test.go`.
- **The identity/config migration is complete.** Alert configuration now
  carries an additive identity schema version and the monitor runs a
  non-mutating, fail-closed migration plan against the live resource registry
  before persisting it. Proven guest, guest-disk, storage, Docker, and
  provider-declared succession keys move to their single write identity;
  ambiguous, conflicting, unknown, and temporarily absent rows are retained.
  Active-alert snapshots (including acknowledgement fields) are rewritten with
  canonical state IDs after restore. The planner is idempotent across rollback
  and re-upgrade, rejects future schema versions, and the frontend round-trips
  the marker on ordinary configuration saves.
