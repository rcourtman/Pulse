## v6 Canonical Alert-Engine Migration

Status: Draft
Date: 2026-03-10
Scope: `pulse` only

## Purpose

This document defines the canonical v6 alert-engine target for this repo and the migration path inside the current codebase.

It is intentionally not a rewrite plan. The current system already has the pieces that matter:

- `internal/unifiedresources` already owns canonical resource identity, parent-child relationships, metrics targets, and provider-native incidents.
- `internal/alerts/unified_eval.go` already centralizes threshold-based metric evaluation for unified resources.
- `internal/alerts/unified_incidents.go` already mirrors canonical resource incidents into the live alert manager.
- `internal/monitoring/monitor_alert_sync.go` already syncs unified incidents into shared alert state.
- `internal/monitoring/monitor_alerts.go` already owns the downstream fan-out to tenant websocket broadcast, notifications, incident recording, and AI callbacks.

What is still legacy is responsibility placement: most `Check*` methods in `internal/alerts/alerts.go` still mix collection, threshold resolution, transition rules, alert construction, and notification entry.

## Canonical Target Model

The target model keeps one alert engine and splits it into explicit layers.

### 1. Unified Resource Identity

Source of truth: `internal/unifiedresources`

Canonical resource identity comes from `unifiedresources.Resource`:

- `Resource.ID` is the long-term alert `resource_id`.
- `Resource.Type` is the long-term alert `resource_type`.
- `Resource.Canonical`, `ParentID`, `ParentName`, `MetricsTarget`, and `DiscoveryTarget` are the only allowed inputs for display naming, grouping, and source lookup.
- `Resource.Incidents` is the only canonical provider-native incident feed.

Important migration bridge:

- Today, alert `resource_id` and threshold override key are not always the same key.
- Example: host alerts emit `agent:<hostID>` resource IDs, while override lookup still resolves on raw `host.ID`.
- The migration must therefore carry both a `ResourceID` and a `ThresholdKey` until config storage is migrated safely.

### 2. Canonical Alert Specs

Add a spec layer inside `internal/alerts` and make it the only input to transition logic.

Minimum spec families:

- `MetricSpec`
- `StateSpec`
- `IncidentSpec`

Minimum fields every spec must carry:

- stable `AlertID`
- canonical `ResourceID`
- explicit `ThresholdKey`
- canonical `ResourceType`
- `Node`, `Instance`, `ResourceName`
- `AlertType`
- current value or incident payload
- resolved message and metadata
- transition policy fields such as `RequiredConfirmations`, `MonitorOnly`, and `DisableConnectivity`

This is the contract that replaces ad hoc alert assembly inside each `Check*` method.

### 3. Evaluator Engine

Source of truth: `internal/alerts`

The evaluator owns only:

- threshold resolution
- time-threshold delay handling
- hysteresis trigger/clear behavior
- active/resolved transition rules
- acknowledgement and escalation preservation
- rate limiting
- quiet-hours suppression
- flapping suppression

`checkMetric`, `preserveAlertState`, `dispatchAlert`, and the resolved-alert flow already do most of this. The migration should reuse those mechanics, not replace them.

### 4. Transition Persistence

Transition state remains in the existing manager until the migration is complete:

- `activeAlerts`
- `recentlyResolved`
- `recentAlerts`
- `pendingAlerts`
- `offlineConfirmations`
- `ackState`
- flapping and rate-limit maps
- `historyManager`

Do not create a second persistence path. The migration succeeds by moving callers onto the same state machine.

### 5. Notification Fan-Out

`internal/monitoring/monitor_alerts.go` remains the notification boundary:

- tenant websocket broadcasts
- notification manager sends/cancels
- incident timeline recording
- AI alert callbacks

The evaluator emits alert transitions. Monitoring owns delivery.

## Why a Full Rewrite Is the Wrong Move

A full rewrite is the wrong move here for four concrete reasons.

1. The hard part already exists.
The current manager already preserves ack state, start times, quiet-hours behavior, cooldown re-notify logic, rate limiting, flapping suppression, and resolved notification behavior. Rebuilding that from scratch would duplicate the highest-risk code path.

2. The codebase already has a working canonical seam.
`unified_eval.go` and `unified_incidents.go` prove the repo can migrate incrementally by moving input builders first and leaving transition state alone.

3. Pollers are still source-shaped.
The monitor layer still fans out typed models from Proxmox, agents, Docker, PBS, PMG, storage, backups, and snapshots. Replacing the engine outright would force every poller and every alert family to move at once.

4. Downstream contracts already depend on the current manager.
`monitor_alerts.go` assumes one live alert manager feeds websocket, notifications, incident recording, and AI. Swapping the engine wholesale would multiply migration surfaces instead of reducing them.

The correct move is to keep one manager and migrate input assembly until legacy `Check*` methods are thin builders only.

## Stable Behaviors That Must Not Change Mid-Migration

These behaviors are already relied on by tests, persisted state, or user workflow and must remain stable until a dedicated migration phase says otherwise.

- Existing alert ID formats stay stable for migrated families. Do not change IDs while `preserveAlertState`, ack carry-over, and history continuity still depend on them.
- Existing threshold override keys stay stable until config migration is implemented explicitly.
- `checkMetric` hysteresis, per-type time thresholds, per-metric time thresholds, cooldown re-notify, and critical escalation-on-level-change stay unchanged.
- Ack, start time, escalation metadata, and last notification timestamps continue to survive alert rebuilds through `preserveAlertState`.
- Notification suppression rules stay unchanged: quiet hours, rate limiting, flapping, monitor-only, activation state, and resolved-notification rules.
- Current confirmation semantics stay unchanged:
  - node offline: 3 polls
  - PBS offline: 3 polls
  - PMG offline: 3 polls
  - storage offline: 2 polls
  - guest powered-off: 2 polls
- Unified incident suppression semantics stay unchanged, including parent-child de-dup in `internal/alerts/unified_incidents.go`.
- Tenant broadcast routing stays in monitoring and remains tenant-scoped.

## Migration Phases

### Phase 1: Introduce Canonical Builder Contracts

Goal:
Create explicit canonical alert-spec builders without changing downstream alert behavior.

Code moves:

- Add internal spec types in `internal/alerts` for metric, state, and incident evaluation.
- Give every spec both `ResourceID` and `ThresholdKey`.
- Add builders that project `unifiedresources.Resource` into evaluator input.
- Add builder helpers for typed source models where unified resources are not yet the direct caller.
- Keep `CheckUnifiedResource` and `SyncUnifiedResourceIncidents` as the first-class entry points for canonical metric and incident inputs.

What stays:

- `checkMetric`
- `preserveAlertState`
- existing notification callbacks
- existing manager maps and history persistence

Delete at end of phase:

- Nothing. This phase is structure-only.

Exit criteria:

- New evaluator helpers accept specs rather than raw source models.
- New alert logic is forbidden from being added directly inside typed `Check*` methods.

### Phase 2: Convert Metric Families First

Goal:
Make typed metric checks collectors/builders only.

Code moves:

- Convert `CheckGuest`, `CheckNode`, `CheckHost`, `CheckPBS`, and `CheckStorage` into:
  - source-specific collection and suppression
  - canonical spec building
  - shared evaluator calls
- Reuse `evaluateUnifiedMetrics` and `checkMetric` for the final transition decision.
- Keep source-specific metadata assembly in the builders.
- For host-agent metrics, explicitly bridge the current mismatch between raw host override keys and emitted `agent:<id>` alert resource IDs.

Required parity gates:

- extend `internal/alerts/unified_eval_parity_test.go`
- keep `internal/alerts/threshold_resolution_shared_test.go` green
- keep `internal/alerts/override_normalization_test.go` green

Delete at end of phase:

- Inline metric trigger/clear code inside the migrated `Check*` methods.
- Duplicated threshold-resolution branches that now live in shared builder/evaluator helpers.

Exit criteria:

- Those five `Check*` methods no longer create metric alerts directly.
- Metric alert construction happens only through shared spec-to-evaluator code.

### Phase 3: Move State and Event Families Onto the Same Engine

Goal:
Stop building `Alert` structs directly inside typed methods for non-metric resource alerts.

Code moves:

- Convert offline and power-state handling to `StateSpec` with per-family confirmation policy.
- Convert provider-specific event families to builders:
  - host SMART risk and wearout
  - RAID degradation/rebuild state
  - ZFS pool and device health
  - Docker container state, health, restart-loop, OOM, memory-limit, and image-update events
  - PMG queue depth, oldest message, backlog, anomaly, and node queue events
- Keep the current confirmation counts, alert IDs, messages, and metadata fields stable.
- Keep `SyncUnifiedResourceIncidents` as the canonical incident path, but make it emit `IncidentSpec` into shared transition helpers instead of assembling `Alert` objects directly.

Delete at end of phase:

- Direct `Alert{...}` construction in migrated event branches of `internal/alerts/alerts.go`.
- Resource-family-specific state transition duplication that now exists only to manage confirmations and resolved handling.

Exit criteria:

- Typed `Check*` methods act as collectors/builders for both metric and event families.
- Shared transition helpers own all create/update/resolve behavior for migrated families.

### Phase 4: Make Monitoring Call the Canonical Path First

Goal:
Make unified resources the primary alert input surface without breaking domains that still need typed collection.

Code moves:

- After unified resource refresh, evaluate canonical resources first.
- Expand `internal/monitoring/monitor_alert_sync.go` from incident-only sync into the canonical unified alert-sync entry point.
- Keep typed poller calls only for domains not yet represented well enough in unified resources.
- Current expected holdouts:
  - backups from `internal/monitoring/monitor_backups.go`
  - snapshots from `internal/monitoring/monitor_backups.go`
- For those holdouts, move them to spec builders even if they are not yet unified-resource-backed.

Delete at end of phase:

- Direct typed metric fan-out from monitor loops for resource families already covered by unified resources.
- Compatibility wrappers in monitoring that only exist to feed old evaluator entry points.

Exit criteria:

- The primary evaluation path for guest, node, host, PBS, storage, and unified incidents starts from unified resource state.
- Typed monitor calls remain only for documented exceptions.

### Phase 5: Collapse the Identity Bridge and Remove Legacy Entrypoints

Goal:
Finish the migration by making unified identity the only alert identity contract.

Code moves:

- Migrate threshold override/config storage from legacy keys to canonical unified resource IDs.
- Remove explicit `ThresholdKey` bridging once config and alert state agree on the same ID contract.
- Convert backups and snapshots into canonical builder specs with stable resource references.
- Retire compatibility wrappers once no monitor path depends on them.

Delete at end of phase:

- legacy key-translation helpers that only exist for override compatibility
- typed `Check*` evaluator logic that survived as wrappers
- any duplicated incident or metric entrypoints made obsolete by the canonical path

Exit criteria:

- `unifiedresources.Resource.ID` is the single alert identity contract for evaluation and config.
- Remaining `Check*` methods are either deleted or are thin adapters used only at external boundaries.

## How Legacy `Check*` Methods Should Look After Migration

After Phase 3, a legacy `Check*` method should do only this:

1. Read source-specific models.
2. Apply source-specific suppression or shaping that cannot live in the generic evaluator.
3. Build canonical specs with explicit resource identity, threshold key, metadata, and transition policy.
4. Hand the specs to shared evaluator helpers.

It should not:

- resolve trigger/clear transitions inline
- mutate `activeAlerts` directly
- construct resolved-alert flows directly
- send notifications directly
- own history or acknowledgement persistence

## Deletion Rules By Area

Delete code only when the replacement path is already live and parity-tested.

- Metric-family code can be deleted once the shared metric builder/evaluator path produces identical IDs, thresholds, and metadata.
- Event-family code can be deleted once the shared state/incident helpers preserve confirmation counts, ack carry-over, and resolution behavior.
- Monitor fan-out code can be deleted only after unified resource evaluation is the first caller in the poll loop.
- Key-translation helpers can be deleted only after config override storage is migrated to canonical unified IDs.

## Highest-Risk Migration Edges

These are the places most likely to create regressions if the migration is done sloppily.

1. Identity drift between `ResourceID` and override key.
This is already visible in host-agent alerts. Do not collapse those keys until config migration is real.

2. Alert ID churn.
Changing IDs too early will break ack continuity, history continuity, re-notify cooldown state, and flapping history.

3. Partial transition rewrites.
If a builder starts creating `Alert` structs directly again, transition policy will drift across families.

4. Backup and snapshot exceptions.
Those flows still come from recovery/backups rather than unified resources. They should join the canonical spec layer before any attempt to delete their typed paths.

5. Incident duplication.
`SyncUnifiedResourceIncidents` already suppresses redundant parent/child incidents. Any replacement path must preserve that exact behavior.

## Execution Order

Do the work in this order:

1. builder/spec layer
2. metric-family migration
3. state/event-family migration
4. monitor unified-first routing
5. config identity migration and final deletions

Any attempt to start with Phase 4 or Phase 5 first will create a temporary second alert engine, which this repo does not need.
