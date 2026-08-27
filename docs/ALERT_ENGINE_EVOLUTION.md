# Alert Engine Evolution

Status: Active
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

## What is kept

The delivery pipeline (`internal/notifications` queue, DLQ, receipts,
templates, delivery health), the operator-intent fabric, unified
resources, the specs/evaluator layer, the UI shell, and the
`monitor_alerts.go` fan-out boundary — which becomes an event consumer.

## What is retired (by the end of Phase 2–3)

The manager's per-family tracking maps, the imperative per-platform
check bodies, the scattered suppression checks, and — once the event log
is authoritative — the JSON snapshot history file.
