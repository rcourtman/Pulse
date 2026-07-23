# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert identity, alert specs, evaluation, persistence semantics, and
operator-facing alert routing behavior for live runtime alerts.
Docker and Podman container CPU thresholds evaluate host-capacity-normalized
CPU percent, not Docker's runtime-native per-core percent. Alert metadata may
carry the raw per-core value and reporting host CPU count for evidence, but the
threshold value and canonical `cpuPercent` metadata remain normalized.
Docker and Podman OOM alerts require authoritative runtime evidence: the
container must be stopped (`exited` or `dead`) and its reported `OOMKilled`
state must be explicitly true. Exit code 137 alone is only SIGKILL evidence;
explicit false and unavailable/legacy OOM state both fail closed without an OOM
alert. Recovery clears an existing OOM alert when the authoritative predicate
is no longer true.

## Canonical Files

1. `internal/alerts/specs/types.go`
2. `internal/alerts/specs/evaluator.go`
3. `internal/alerts/canonical_metric.go`
4. `internal/alerts/canonical_lifecycle.go`
5. `internal/alerts/unified_incidents.go`
6. `frontend-modern/src/features/alerts/AlertOverviewActiveAlertsSection.tsx`
7. `frontend-modern/src/utils/alertOverviewPresentation.ts`
8. `frontend-modern/src/utils/alertResourceTablePresentation.ts`
9. `frontend-modern/src/utils/alertDestinationsPresentation.ts`
10. `frontend-modern/src/utils/alertIncidentPresentation.ts`
11. `frontend-modern/src/utils/alertSchedulePresentation.ts`
12. `frontend-modern/src/utils/alertWebhookPresentation.ts`
13. `frontend-modern/src/utils/alertActivationPresentation.ts`
14. `frontend-modern/src/utils/alertAdministrationPresentation.ts`
15. `frontend-modern/src/utils/alertBulkEditPresentation.ts`
16. `frontend-modern/src/utils/alertConfigPresentation.ts`
17. `frontend-modern/src/utils/alertEmailPresentation.ts`
18. `frontend-modern/src/utils/alertFrequencyPresentation.ts`
19. `frontend-modern/src/utils/alertGroupingPresentation.ts`
20. `frontend-modern/src/utils/alertHistoryPresentation.ts`
21. `frontend-modern/src/utils/alertSeverityPresentation.ts`
22. `frontend-modern/src/utils/alertTabsPresentation.ts`
23. `frontend-modern/src/features/alerts/types.ts`
24. `frontend-modern/src/utils/alertThresholdsPresentation.ts`
25. `frontend-modern/src/utils/alertThresholdsSectionPresentation.ts`
26. `internal/alerts/history.go`
27. `frontend-modern/src/stores/alertsActivation.ts`
28. `frontend-modern/src/utils/alertThresholdDefaults.ts`
29. `frontend-modern/src/utils/metricThresholds.ts`
30. `internal/alerts/config_facade.go`
31. `internal/alerts/constants.go`
32. `internal/alerts/model.go`
33. `internal/alerts/metric_hooks.go`
34. `internal/alerts/manager.go`
35. `internal/alerts/default_config.go`
36. `internal/alerts/lifecycle.go`
37. `internal/alerts/escalation.go`
38. `internal/alerts/callbacks.go`
39. `internal/alerts/config/types.go`
40. `internal/alerts/config/normalize.go`
41. `internal/alerts/config/identity.go`
42. `internal/alerts/notification_policy.go`
43. `internal/alerts/read_model.go`
44. `internal/alerts/pmg.go`
45. `internal/alerts/docker.go`
46. `internal/alerts/pbs.go`
47. `internal/alerts/storage.go`
48. `internal/alerts/node.go`
49. `internal/alerts/host.go`
50. `internal/alerts/backup_snapshot.go`
51. `internal/alerts/disk_health.go`
52. `internal/alerts/metric_runtime.go`
53. `internal/alerts/health_assessment.go`
54. `internal/alerts/guest.go`
55. `internal/alerts/config_runtime.go`
56. `internal/alerts/active_persistence.go`
57. `internal/alerts/tracking_cleanup.go`
58. `internal/alerts/active_lifecycle.go`
59. `internal/alerts/active_cleanup.go`
60. `frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx`
61. `frontend-modern/src/components/Alerts/alertAssistantHandoffModel.ts`
62. `frontend-modern/src/components/Alerts/IncidentAssistantHandoffButton.tsx`
63. `frontend-modern/src/components/Alerts/incidentAssistantHandoffModel.ts`
64. `internal/alerts/storage_override_identity.go`
65. `internal/alerts/unified_eval.go`
66. `frontend-modern/src/components/Alerts/ThresholdsTable.tsx`
67. `frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`
68. `frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsPlatformData.ts`
69. `frontend-modern/src/utils/alertTargetTypes.ts`
70. `frontend-modern/src/types/alerts.ts`
71. `frontend-modern/src/stores/websocket.ts`
72. `frontend-modern/src/utils/alerts.ts`
73. `frontend-modern/src/utils/alertsActivation.ts`
74. `internal/operationaltrust/contracts.go`
75. `internal/alerts/operational_contract.go`

## Shared Boundaries

1. `internal/operationaltrust/contracts.go` shared with `notifications`: the operational trust contract is jointly consumed by canonical alert lifecycle ownership and notification delivery linkage without making delivery state operational truth.
2. `internal/proxmoxidentity/backup_identity.go` shared with `monitoring`, `storage-recovery`: Proxmox PBS backup subject identity is a shared runtime boundary for monitoring backup freshness, backup-age alert attribution, and recovery-point guest mapping.
Alert multiline field presentation is shared with frontend-primitives:
notification, timeline, threshold ignored-prefix, and resource threshold note
editors must compose the shared `FormTextarea` primitive for label/id/help
wiring and textarea chrome instead of rendering raw native `<textarea>` shells
in alert-owned runtime components.
Alert resource threshold action presentation is also shared with
frontend-primitives: row, mobile-card, global-default, and bulk-selection
icon-only actions must compose `ActionIconButton` for shared size, tone, focus,
title, and accessible-name behavior instead of rendering local `<button>` plus
inline SVG shells in alert-owned runtime components.
Alert runtime state has one explicit ownership boundary: `AlertConfig.enabled`
controls detector evaluation and in-product alert visibility, while
`AlertConfig.activationState` controls external notification delivery only.
The websocket store, resource-row presentation, navigation counts, and Alerts
overview must preserve active alert truth while notification delivery is
pending review or snoozed. Notification activation must not clear the browser
active-alert store, suppress resource indicators, lock threshold/history
configuration, or claim that monitoring has stopped.
Operational evidence and lifecycle identity are typed through
`internal/operationaltrust`. Evidence envelopes distinguish completeness,
confidence, permissions, freshness, correlation, and bounded provider detail.
Evidence and transition identifiers are deterministic under retry. Active and
resolved alert compatibility payloads carry an additive canonical operational
record, latest transition, and bounded evidence envelopes; legacy alert paths
must migrate through `internal/alerts/operational_contract.go` and name their
limited provenance honestly rather than inventing confirmed provider evidence.
Acknowledgement remains distinct from resolution, and every resolution
transition references recovery evidence separate from its trigger evidence.

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`
2. Add typed collector/builders in the resource-specific checker owner or
   `internal/alerts/metric_runtime.go`
3. Add identity/persistence updates through canonical alert helpers only
4. Add or change alert history persistence through `internal/alerts/history.go` using normalized owned storage roots and fixed storage leaves only
5. Add or change locked alert-investigation commercial handoff behavior through
   `frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx` while
   preserving the shared upgrade-navigation contract; the alert surface may
   route to the canonical destination, but must not emit browser-local upgrade
   metrics or present Pro-required copy when prompt suppression applies.
6. Add or change frontend metric color thresholds through
   `frontend-modern/src/utils/metricThresholds.ts`,
   `frontend-modern/src/utils/alertThresholdDefaults.ts`, and
   `frontend-modern/src/stores/alertsActivation.ts` so browser display colors
   consume the same configured alert thresholds and override identity chain as
   the alert runtime instead of hard-coded per-surface thresholds.
7. Add or change alert investigation handoffs through
   `frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx` and
   `frontend-modern/src/components/Alerts/alertAssistantHandoffModel.ts`.
   On resource-backed active alert cards, Patrol is the primary doer: the
   visible primary action must run a manual scoped Patrol trigger such as
   "Have Patrol investigate" through the `ai-runtime` manual Patrol route
   contract. Pulse Assistant remains a secondary context-only explanation path:
   Assistant handoffs must preserve alert context, force request-scoped
   approval mode, send bounded model-only handoff context plus structured
   resource references through the shared Assistant chat transport, and render a
   compact Alerts-owned briefing in the Assistant drawer without transferring
   raw command payloads or synthesizing, pre-filling, or auto-submitting a chat
   prompt. The Patrol trigger and the context-only Assistant open path must stay
   distinct.
8. Add or change Pulse Assistant incident timeline handoffs through
   `frontend-modern/src/components/Alerts/IncidentAssistantHandoffButton.tsx`
   and `frontend-modern/src/components/Alerts/incidentAssistantHandoffModel.ts`;
   these handoffs must preserve sanitized incident facts and timeline event
   summaries, force request-scoped approval mode, send the same sanitized facts
   as model-only handoff context plus structured resource references through
   the shared Assistant chat transport, and keep raw command/output details in
   the incident or approval surface rather than the chat handoff. Incident
   handoffs must not add suggested prompt chips or route-owned remediation
   instructions; the configured model owns investigation and next-step
   reasoning after it receives the context.
9. Add or change alert target resource types through
   `internal/alerts/specs/types.go`, `internal/alerts/config/identity.go`,
   `internal/alerts/unified_eval.go`, and
   `frontend-modern/src/utils/alertTargetTypes.ts`. Supported target types
   must share the unified evaluator, the canonical threshold/override identity
   chain, and the standard notification delivery path.
10. Add or change the alert notification destinations catalog through
    `frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx` and
    `frontend-modern/src/utils/alertDestinationsPresentation.ts`. The
    destinations surface presents mobile push (Pulse Mobile paired through
    Relay) alongside email, Apprise, and webhooks so phone delivery is
    discoverable where alert routing is configured. It stays a pointer
    surface: it routes setup to the canonical `/settings/system-relay` Remote
    Access panel rather than duplicating relay pairing state or relay API
    calls, and when the `relay` feature is absent it gates through the shared
    `FeatureGateSection` and upgrade-navigation contract, rendering no upgrade
    call-to-action when prompt suppression applies.

## Forbidden Paths

1. New ad hoc `Check*`-local evaluator logic
2. Reintroducing runtime legacy alert-ID contracts
3. Reintroducing per-family threshold/override merge logic outside the shared path

## Completion Obligations

1. Update alert spec/evaluator tests when a new rule kind is added
2. Update this contract if alert truth or identity rules change
3. Route runtime changes through the explicit alert proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten or add guardrails when an old alert path is removed

### Attention projection source contract

Canonical alert operational records, evidence envelopes, and lifecycle
transitions are the only writable source for Patrol attention. The legacy alert
adapter must preserve an existing provider-authored recommended next step, use
the canonical `incidentAction` when present, and otherwise add the safe
operator instruction to open the affected resource and verify current state
before changing it. This is migration guidance, not action authority.

The attention read model may project and filter alert lifecycle state, but it
must not reinterpret acknowledgement as resolution, omit suppressed state from
inspectability, or convert missing/stale evidence into health.

## Current State

The alert resource-incident panel
(`frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx`)
dropped its "Open in Infrastructure / Workloads / Storage / Recovery"
cross-jump chip strip on 2026-05-16 when the surrounding platform-first
migration retired broad surface-link chips. The panel now keeps
investigation flow in-place through `IncidentAssistantHandoffButton` and the
shared incident-timeline cards; it must not reintroduce a chip strip that
links to the retired top-level routes, and the supporting
`buildResolvedResourceSurfaceLinks` helper was deleted from
`frontend-modern/src/routing/resourceLinks.ts` as part of the same pass.

Alert browser surfaces no longer manage their own runtime-capabilities fetch or
`hasAIAlertsFeature` prop chain. `frontend-modern/src/pages/Alerts.tsx` and the
shared alert overview surfaces (`OverviewTab.tsx`, `HistoryTab.tsx`,
`AlertOverviewActiveAlertsSection.tsx`, `AlertHistoryTableSection.tsx`,
`AlertHistoryTableAlertRow.tsx`, `AlertOverviewAlertCard.tsx`) must source AI
alert feature gating from the shared entitlements layer, not from a per-surface
`loadRuntimeCapabilities` fetch. Alert surfaces must not re-introduce their own
`hasAIAlertsFeature`, `runtimeCapabilitiesLoading`, or direct
`/api/license/runtime-capabilities` reads.

Canonical alert identity and evaluation are the live runtime model. Remaining
legacy references should exist only in explicit migration or negative test
boundaries.
TrueNAS per-resource threshold overrides use the unified resource's current
canonical ID as their persistence and evaluation key. API-backed TrueNAS
systems are keyed by configured connection, never reported hostname or DMI
serial, so repolls, result reordering, missing serials, DR clones, and multiple
same-hostname appliances cannot transfer or strand an override. The browser
may read provider-declared superseded IDs and metric-target IDs while
projecting a legacy `alerts.json`, but an edit must remove those candidates
and persist exactly one row under the current canonical ID. While an input is
active, the thresholds projector must retain the edited row across WebSocket
resource refreshes; blur commits that retained row into the unsaved
configuration, and the global configuration save remains the only API
persistence boundary.

The monitoring bridge may migrate only provider-declared, unambiguous
canonical-ID successions before evaluation. It must persist the migrated
configuration before installing it in the alert manager, let a current-key
override win while removing its retired duplicate, and retain unknown or
temporarily absent override rows until a provider proves succession. The same
persisted override must drive trigger, clear, derived critical severity, and
notification dispatch before and after restart. Regression ownership is
`frontend-modern/src/features/alerts/thresholds/hooks/__tests__/truenasThresholdPersistence.test.tsx`,
`frontend-modern/src/features/alerts/__tests__/useAlertsConfigurationState.test.tsx`,
`internal/alerts/canonical_override_migration_test.go`, and
`internal/monitoring/monitor_alert_override_migration_test.go`.
All v6 platform alert targets must enter runtime threshold evaluation through
`UnifiedResourceInput` and `internal/alerts/unified_eval.go`: Proxmox guests/
nodes/storage, Docker hosts/containers/services, Kubernetes clusters/nodes/
namespaces/deployments/pods, TrueNAS systems/pools/datasets/disks, VMware
vSphere hosts/VMs/datastores/networks, PBS, PMG, and standalone host agents.
Adding a platform-local evaluator branch for these resource families is
forbidden.
Per-platform defaults, per-resource overrides, global disables, active-alert
reevaluation, history persistence, and notification delivery must use the same
alert configuration shape rather than a platform-specific sidecar.
Notification cadence is part of that runtime contract. `Schedule.Cooldown` and
`Schedule.MaxAlertsHour` apply to already-active alert re-notifications as well
as first-fire creation, including canonical metric alerts, legacy metric paths,
and severity-change re-notifications. Accepted alert dispatch must record
`LastNotified` back onto the live active-alert state before persistence, even
when a restored or replayed alert is dispatched through a clone, so reloads do
not reopen the same alert's notification window.
Recently resolved alerts are an operator-facing transition window, not an
unbounded history store. `recentlyResolved` must prune expired entries and cap
the newest retained entries on insert as well as during cleanup, so monitor
sync and websocket state snapshots remain bounded; durable resolved-alert
history belongs in the alert history store, not in this live transition cache.
`recentlyResolved` and `resolvedAlias` have one lock owner:
`Manager.resolvedMutex`. Every access holds that mutex, including alias-repair
lookups, which require the write lock. When an operation needs both manager
state and resolved state, the only permitted nested order is
`Manager.mu` then `Manager.resolvedMutex`; no path may acquire `Manager.mu`
while holding `Manager.resolvedMutex`. Resolved critical sections are limited
to map access and must not dispatch, persist history, invoke callbacks, or
perform notification work. Canonical lifecycle and stateful cooldown refires
consume resolved state through the shared lock-order-aware helper, preserve the
original alert `StartTime`, and keep the five-minute refire/history semantics.
The browser thresholds surface is also platform-shaped: Proxmox, Docker,
Kubernetes, TrueNAS, vSphere, PBS, PMG, and Systems. It must use the shared
FilterBar chip and "+ Filter" pattern for resource filtering, and alert tables
must use the canonical platform table column-kind alignment helpers from
`frontend-modern/src/features/platformPage/` rather than hard-coded table
alignment classes.
Alert filter option semantics stay alert-owned, but FilterBar chip
presentation is frontend-primitives-owned: severity filter leading dots must
use `filterChipStatusDot` rather than alert-local span factories.
Alert notification and timeline form textareas are also shared-primitive
consumers. Email recipient lists, Apprise target lists, webhook payload
templates, threshold ignored-prefix input, and incident timeline notes must
compose `FormTextarea` for label/id/help wiring and textarea chrome instead of
recreating raw labelled `<textarea>` shells locally. Alert resource row/mobile
note editors now follow the same primitive contract.
Guest metric canonical state remains resource-backed and therefore node-scoped
for Proxmox guests, so node moves must not strand active alert state on the
previous resource ID. When a guest metric alert survives a node move, alerts
runtime must migrate the active alert, history entry, acknowledgment record,
suppression/rate-limit/flapping tracking, and guest per-disk metric identity
to the current canonical state instead of reopening a duplicate alert or
resolving only the stale node-scoped identity.
That same guest-threshold owner also governs guest-derived lifecycle and
posture alerts. Snapshot age, backup age, powered-off state, and
configuration-change reevaluation must all construct a canonical lightweight
guest snapshot and route threshold resolution through the shared
guest-defaults → filter-driven custom rules → guest-override chain.
That canonical guest context must preserve the live guest name and tags for
snapshot and backup posture evaluation. Ignored prefixes, `pulse-no-alerts`,
configured ignored tags, and required-tag filtering must resolve through the
same guest alert policy before any guest-derived alert is created; posture
pollers may not downgrade that context to a name-only lookup that bypasses the
operator's suppression policy.
Passing `nil` guest context or resolving only overrides/defaults is forbidden
because it silently bypasses custom guest rules and makes guest lifecycle
alerting diverge from running-guest metric truth.
That same guest-alert owner also has to retire per-disk guest alerts when the
guest stops, disk alerting is disabled, or the reported disk set changes.
Canonical guest disk identity is only valid while the guest still exposes that
disk resource under the current thresholds, so runtime cleanup must remove
stale `guestID-disk-*` state instead of leaving orphaned per-disk incidents in
active alerts, resolved history, or later UI projections.
That same alerts runtime also owns instance-scoped node display-name
resolution. Raw node names are not globally unique across configured
infrastructure instances, so cached node display names must key on instance +
node identity whenever the alert carries instance context. Alert updates,
incident rebuilds, and guest-metric migrations may fall back to the legacy
name-only cache only for instance-less resources like standalone host agents.
That same host-alert boundary also owns vendor-managed NAS RAID suppression as
an alert-lifecycle concern. Shared storagehealth rules decide which Synology
or QNAP md arrays are vendor-managed system volumes rather than customer-facing
storage, and alerts runtime must use those shared rules both to suppress new
RAID incidents and to clear stale suppressed alert IDs even after monitoring
has already normalized those arrays out of canonical host state.
Storage alert runtime also owns operator-facing resource labels for storage
incidents. ZFS device alert labels must preserve raw device names such as
`/dev/sda4`, but must not join pool and device labels with a raw slash because
device paths can already begin with `/`; browser alert surfaces consume the
runtime `resourceName` as authored rather than patching storage labels locally.
Ceph pool storage threshold resolution is also source-alias aware. Storage
alerts must evaluate the normalized pool storage id while accepting legacy
`agent:<host>-ceph-pool-<name>` override keys as aliases, so operators do not
lose saved thresholds when the same physical Ceph pool moves between
host-agent-only fallback and Proxmox API canonical discovery.
Active alert reevaluation after threshold or config changes must use canonical
resource type metadata before the legacy node fallback. Host-agent Ceph pool
alerts may carry no-colon resource ids with `Instance == Node`, but when
metadata or resource type says storage they must keep using storage threshold
resolution and source-alias overrides instead of node defaults.

Browser metric severity colors are also alert-backed. Workloads,
Infrastructure, and Storage may pass resolved display thresholds into their
local bars, but threshold selection must flow through the shared alert activation
store and `frontend-modern/src/utils/metricThresholds.ts`, including configured
hysteresis, disabled thresholds, storage usage defaults, and guest/Docker
override identity candidates. Static metric-color defaults are only fallback
presentation behavior for callers that do not have alert configuration in
scope.

Docker container image-update alerts are lifecycle-governed by the alerts
runtime. Disabling Docker update alerts globally, disabling alerts for a
specific Docker container, ignoring a Docker container prefix, or disabling all
Docker container alerts must clear the active image-update alert plus resource
and identity first-seen tracking. Generic threshold reevaluation must not keep
or resurrect image-update alerts after their owning Docker alert configuration
has disabled them.

Backup orphan evaluation is also inventory-scoped. The alerts runtime may
evaluate recovery rollups for backup age, but unresolved Proxmox PVE backup
subjects must not be treated as orphaned until monitoring has supplied the
matching per-instance guest-type inventory readiness signal. Known Proxmox
template subjects are valid backup subjects, not orphaned workload backups,
even though templates remain excluded from normal runtime workload resources.

Alert history persistence is also part of that canonical boundary. The history
manager may choose the owned runtime data directory, but it must normalize that
directory once and then resolve only the fixed `alert-history.json` and
`alert-history.backup.json` leaves through the shared storage-path helper
before any filesystem read, write, rename, or delete. Future history-persistence
changes must not reintroduce raw `filepath.Join(dataDir, ...)` joins from
caller-supplied directories or ad hoc history filenames.
Agentless availability incidents now enter alerts through the same unified
resource incident bridge as storage, PBS, VM, and host resource incidents.
Standalone `network-endpoint` resources and any canonical resource carrying an
attached availability facet must create canonical `resource-incident` alerts
with provider display `Availability`; availability alerting must not introduce
a second endpoint-only evaluator or alert identity family outside
`internal/alerts/unified_incidents.go`. When a resource carries multiple
checks, the incident `NativeID` selects the exact check evidence envelope that
is copied into the alert and its `OperationalRecord`; the singular
compatibility summary must never substitute evidence from a different target.
The same lifecycle transition then projects into Patrol like every other
canonical operational record.

Notification transport, provider delivery, queue safety, and notification API
transport now live under the explicit `notifications` subsystem inside the
current architecture lane. The alerts surface still owns operator-facing alert
pages and routing UX, but it does not implicitly own the delivery engine.
That includes the webhook settings editor: alert UI may present provider setup,
but canonical service-field ownership such as Pushover `token` / `user`
normalization belongs to `internal/notifications/` and persistence boundaries,
not to alert-surface runtime delivery code.

The alert webhook editor now mirrors that canonical Pushover field rule through
`frontend-modern/src/utils/alertWebhookPresentation.ts`, so the UI shares the
same alias, preset, and custom-field input mapping instead of carrying its own
local webhook-field normalization fork.
The alert manager callback layer now also has to stay fan-out-safe. Monitor
delivery, the unified alert bridge, and Patrol-adjacent AI listeners must
compose through additive fired/resolved subscriptions instead of overwriting a
single callback slot, and alert-triggered Patrol enqueueing must stay on the
canonical unified alert bridge plus trigger-manager path rather than reviving
duplicate callback-side Patrol shortcuts.
That runtime callback boundary is now factored into
`internal/alerts/callbacks.go` as same-package Manager support code so it can
own callback mutexes, subscription IDs, legacy Set* slots, and fan-out snapshot
helpers without creating an import cycle around the `Alert` runtime type.
Alert configuration types, pure normalization, and resource-type identity
helpers now live under `internal/alerts/config/`; the parent `alerts` package
may re-export aliases and wrappers for compatibility, but consumer packages
must keep importing `internal/alerts` unless they are explicitly taking
ownership of alert configuration internals.
Alert configuration runtime now lives in `internal/alerts/config_runtime.go`.
That file owns `UpdateConfig` normalization and activation-state migration,
global disable cleanup, active alert reevaluation after threshold changes,
threshold override cloning and merge behavior, and hysteresis defaults; future
config-driven runtime behavior should extend that owner rather than expanding
the central Manager file.
Active-alert persistence now lives in `internal/alerts/active_persistence.go`.
That file owns active-alert save/load, secure active-alert storage leaves,
startup restoration and legacy active-alert ID migration, and periodic active
alert persistence; persistence changes should extend that owner rather than
adding save/load logic back to the central Manager file.
Tracking-map cleanup now lives in `internal/alerts/tracking_cleanup.go`. That
file owns stale flapping, suppression, pending-alert, offline-confirmation,
Docker tracking, rate-limit, recent-alert, acknowledgement, and stale active
alert cleanup; future cleanup rules should extend that owner rather than
mixing cleanup into resource evaluators.
Active-alert lifecycle now lives in `internal/alerts/active_lifecycle.go`.
That file owns acknowledgement and unacknowledgement, manual active-alert
clearing, preserved alert state during rebuilds, poll-confirmed offline
recovery clears, resolved-alert registration, and no-lock active-alert removal
helpers; future active-alert lifecycle changes should extend that owner.
Active-alert cleanup now lives in `internal/alerts/active_cleanup.go`. That
file owns TTL cleanup, auto-acknowledgement cleanup, stale acknowledgement
retention cleanup, node-retirement cleanup, and full active-alert state reset;
future cleanup policy changes should extend that owner.
The old central `internal/alerts/alerts.go` file is intentionally gone. The
residual manager surface is now split by ownership: `config_facade.go` owns
compatibility aliases and wrapper functions for the leaf config package,
`model.go` owns alert runtime data structures and clone semantics,
`constants.go` owns package-wide cleanup and storage constants,
`metric_hooks.go` owns Prometheus integration callbacks, `manager.go` owns
Manager state and construction, `default_config.go` owns the default runtime
configuration literal, `lifecycle.go` owns shutdown, and `escalation.go` owns
the escalation loop and escalation state mutation. Future changes must extend
the owning file rather than reintroducing a central catch-all manager file.
Alert notification policy now lives in `internal/alerts/notification_policy.go`.
That file owns dispatch suppression, flapping suppression, quiet-hours
suppression, monitor-only notification suppression, cooldown decisions, and
per-alert rate limiting; future notification-gating changes should extend that
policy owner rather than burying new checks inside metric or resource-specific
evaluators.
The same policy owner also exposes the read-only alert delivery diagnosis
projection used by `/api/alerts/delivery-diagnosis`; that projection may explain
current gating state, quiet-hours replay timing, cooldown timing, rate-limit
counts, and flapping suppression, but it must not dispatch callbacks or mutate
flapping/rate-limit tracking maps.
The same dispatch policy owns firing-notification evidence on active alerts:
any alert that passes notification suppression and enters the fired callback
fan-out must carry `LastNotified` before the callback clone is emitted. Resolved
notification gating depends on that field to distinguish alerts whose firing
notification actually entered delivery from alerts that were never sent.
Alert read-side output now lives in `internal/alerts/read_model.go`. That file
owns active alert projection and sorting, metadata coercion helpers,
recently-resolved and history output wrappers, and notify-existing redispatch;
future output-ordering or metadata coercion changes should extend that owner
rather than adding another read path inside resource-specific evaluators.
PMG alert evaluation now lives in `internal/alerts/pmg.go`. That file owns PMG
connectivity evaluation, PMG queue and per-node queue checks, quarantine growth
tracking, and mail-rate anomaly detection; future Proxmox Mail Gateway alert
behavior should extend that resource checker owner rather than adding more PMG
logic to the central Manager file.
Docker alert evaluation now lives in `internal/alerts/docker.go`. That file
owns Docker host connectivity, container state and health, container metric
projection, service gap/update-state checks, image-update timing, and Docker
tracking cleanup; future Docker alert behavior should extend that resource
checker owner rather than expanding the central Manager file. It must not keep
shadow last-exit-code state or infer an OOM kill from exit 137; the accepted
container model's nullable runtime-authored `OOMKilled` field is the sole OOM
classification input.
PBS alert evaluation now lives in `internal/alerts/pbs.go`. That file owns PBS
connectivity normalization, PBS metric projection, PBS metric cleanup, and PBS
offline lifecycle handling; future PBS alert behavior should extend that
resource checker owner rather than expanding the central Manager file.
Storage alert evaluation now lives in `internal/alerts/storage.go`. That file
owns storage connectivity handling, storage usage projection, ZFS pool/device
health checks, and storage offline lifecycle handling; future storage alert
behavior should extend that resource checker owner while shared storage-health
assessment helpers remain package-level until host and storage health paths are
separated cleanly.
Proxmox node alert evaluation now lives in `internal/alerts/node.go`. That file
owns node metric and temperature projection, node offline lifecycle handling,
host-agent deduplication bookkeeping, and instance-scoped node display-name
cache updates; future Proxmox node alert behavior should extend that resource
checker owner rather than expanding the central Manager file.
Host-agent alert evaluation now lives in `internal/alerts/host.go`. That file
owns host identity, host-agent metric projection, host disk/SMART/RAID/Unraid
health handling, host cleanup, and host offline lifecycle handling; future host
agent alert behavior should extend that resource checker owner while shared
health-assessment evaluation remains package-level until all storage-health
callers can be separated behind a narrower owner.
Host long-running storage-operation alerts require fresh operation evidence.
An accepted Unraid cancellation/completion or terminal Linux RAID report
resolves the corresponding operation alert immediately. A normal polling gap
does not imply completion; after the monitoring-owned reporting lease expires,
alerts must re-evaluate with only transient operation/progress fields cleared,
retain alerts backed by static degraded/topology evidence when that last-known
topology remains available, and progress the separate confirmed
host-connectivity alert. Persisted operation alerts also obey the durable
accepted-telemetry lease after server restart, so restart cannot make a stale
parity or rebuild alert immortal and missing telemetry is never represented as
healthy reporting.
Snapshot and backup-age alert evaluation now lives in
`internal/alerts/backup_snapshot.go`. That file owns snapshot age/size
evaluation, backup rollup age evaluation, backup inventory readiness, PVE
template subject matching, namespace disambiguation, and snapshot/backup active
alert cleanup; future backup or snapshot alert behavior should extend that
owner rather than expanding the central Manager file.
Proxmox disk health alert evaluation now lives in
`internal/alerts/disk_health.go`. That file owns Proxmox disk canonical
identity, disk health assessment alerts, known-firmware health suppression, and
SSD wearout alerts; future Proxmox disk-health behavior should extend that
checker owner rather than expanding the central Manager file.
Shared metric threshold runtime now lives in
`internal/alerts/metric_runtime.go`. That file owns metric threshold lookup,
per-metric delay resolution, legacy metric alert creation/update/clear
behavior, metric runtime options, alert key sanitation, and metric delta
helpers; future metric-threshold behavior should extend that owner rather than
adding shared metric logic back to the central Manager file.
Shared storage-health assessment alerting now lives in
`internal/alerts/health_assessment.go`. That file owns storage-health reason
normalization, ZFS pool/device reason filtering, canonical health-assessment
alert synchronization, and ZFS device assessment construction for host and
storage checkers; future shared health-assessment behavior should extend that
owner rather than reappearing inside resource-specific evaluators or the
central Manager file.
Proxmox guest alert evaluation now lives in `internal/alerts/guest.go`. That
file owns guest metric projection, per-disk guest metric alerts, guest
powered-off lifecycle alerts, Pulse tag controls, relaxed guest thresholds, and
guest suppression cleanup; future guest-specific alert behavior should extend
that checker owner rather than expanding the central Manager file.
Commercial alert handoffs now follow the same shared navigation boundary.
`frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx` may resolve
the canonical `ai_alerts` destination from the shared license/commercial
contract, but it must delegate the actual open behavior to the
`frontend-primitives` typed upgrade-navigation owner instead of reintroducing
alert-local `window.open(...)` or raw external-tab assumptions.
That same alert button must also honor the ordinary self-hosted prompt
suppression policy: when `presentationPolicy.hideUpgrade` is true, a locked
alert-investigation action may remain visibly unavailable, but it must not
show Pro-required tooltip copy, track upgrade clicks, or open the commercial
handoff route.
Unlocked alert-investigation Assistant handoffs are contextual explanation and
triage entries, not autonomous execution grants. `InvestigateAlertButton.tsx`
must pass `autonomousMode: false` when it opens Pulse Assistant, and it must
open the drawer with context only rather than seeding a product-authored prompt
or choosing a diagnostic/remediation route. The
visible drawer briefing for that same handoff is Alerts-owned presentation
context: alert identifier, severity, metric, resource, threshold, duration,
node label, and message may be shown, while raw diagnostic or remediation
commands remain outside the handoff.
Alert-adjacent shared helpers also inherit the runtime-versus-commercial split
now carried by the shared licensing stores. Alert pages may consume runtime
feature truth from `frontend-modern/src/stores/license.ts`, but any
upgrade/trial posture must come from the dedicated commercial-posture
contract, and public-demo suppression must flow from the shared resolved
`presentationPolicy` contract instead of alert-local demo checks or
entitlement reads.
That same shared read-only presentation contract now also owns the public-demo
alerts shell posture. When `presentationPolicy.readOnly` is true, the alerts
page must behave as a reporting surface: overview/history remain available,
alert activation controls stay hidden, configuration tabs must not render or
remain navigable, and the overview empty state must not tell public demo users
to toggle alerting back on when demo mode already blocks write requests.
The alerts page also owns its mobile tab-shell presentation directly.
`frontend-modern/src/pages/Alerts.tsx` may keep alert-specific active and
disabled tab styling, but horizontal tab scrolling must route through the
shared `touch-scroll` / `scrollbar-hide` class contract instead of writing
inline overflow styles that break CSP on the public shell.
Alert tab routing is part of that same presentation boundary.
`frontend-modern/src/features/alerts/types.ts` owns the canonical mapping
between visible alert tabs and URLs. The operator-facing Notifications tab
must use `/alerts/notifications` as its canonical route because the visible
navigation label is Notifications; `/alerts/destinations` is a legacy alias
only and must normalize through `tabFromPath` / `pathForTab` instead of being
reintroduced as canonical UI vocabulary. The page header for that tab must use
`Notifications` as its title; destination wording may describe concrete email,
Apprise, or webhook endpoints, but it must not reappear as the primary tab or
page identity.
That shared alert presentation boundary now also has explicit alerts ownership.
`frontend-modern/src/utils/alertWebhookPresentation.ts` is the canonical owner
for webhook setup copy, service labels, mention-help phrasing, custom-field
presets, and add/test/update/delete action wording; 
`frontend-modern/src/utils/alertSchedulePresentation.ts` owns quiet-hours day
and suppress-category card styling; and
`frontend-modern/src/utils/alertIncidentPresentation.ts` owns incident badge,
timeline, filter-chip, note-editor, and resource-incident panel presentation.
Future alert presentation work must extend those helpers through the alerts
contract instead of leaving alert-facing wording or styling inlined in page or
feature shells while the registry treats the helpers as unowned.
German and Spanish localization for the Alerts Overview operator journey is
owned by the canonical i18n layer plus the alert presentation helpers:
`frontend-modern/src/i18n/messages.ts` and
`frontend-modern/src/i18n/policy.ts` own the catalog and non-translation
policy, while `frontend-modern/src/utils/alertOverviewPresentation.ts`,
`frontend-modern/src/utils/alertActivationPresentation.ts`, and
`frontend-modern/src/utils/alertTabsPresentation.ts` own the translated
operator-facing alert overview, activation, tab, timeline, and acknowledgement
copy consumed by `frontend-modern/src/pages/Alerts.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewStatsCards.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewActiveAlertsSection.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx`,
`frontend-modern/src/features/alerts/useAlertAcknowledgementState.ts`,
`frontend-modern/src/components/Alerts/IncidentTimelinePanel.tsx`,
`frontend-modern/src/components/Alerts/IncidentEventFilters.tsx`,
`frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx`, and
`frontend-modern/src/components/Alerts/alertAssistantHandoffModel.ts`.
Machine-facing alert identifiers, alert types, resource IDs, resource names,
node names, source-system messages, commands, command output, event payloads,
log text, and the English Assistant model-context labels must remain
untranslated; only the user-visible briefing labels and alert controls may use
the active app locale.

The remaining alert configuration and history presentation helpers now also
have explicit alerts ownership. `frontend-modern/src/utils/alertActivationPresentation.ts`,
`frontend-modern/src/utils/alertAdministrationPresentation.ts`,
`frontend-modern/src/utils/alertBulkEditPresentation.ts`,
`frontend-modern/src/utils/alertConfigPresentation.ts`,
`frontend-modern/src/utils/alertEmailPresentation.ts`,
`frontend-modern/src/utils/alertFrequencyPresentation.ts`,
`frontend-modern/src/utils/alertGroupingPresentation.ts`,
`frontend-modern/src/utils/alertHistoryPresentation.ts`,
`frontend-modern/src/utils/alertSeverityPresentation.ts`,
`frontend-modern/src/utils/alertTabsPresentation.ts`,
`frontend-modern/src/utils/alertThresholdsPresentation.ts`, and
`frontend-modern/src/utils/alertThresholdsSectionPresentation.ts` are the
canonical owners for alert enablement copy, history administration wording,
bulk-edit labels, schedule/configuration text, email-destination field labels,
frequency chips, grouping card styling, history source and resource badges,
severity badges, tab labels, thresholds empty states, and thresholds section
status labels. Overview stat-card labels must also route through the alert
overview presentation helper, and user-facing configuration or thresholds copy
must use workload, VM, and container vocabulary instead of exposing internal
guest override/filter names unless the UI is naming a backend field directly.
Alert severity presentation has a split ownership boundary: alerts owns
`formatAlertSeverityLabel`, compact severity labels, legacy alert severity
class helpers, severity-bucket-to-`StatusIndicatorVariant` mapping, and
severity-bucket-to-detail-row tone mapping in
`frontend-modern/src/utils/alertSeverityPresentation.ts`, while
frontend-primitives owns the visible platform severity badge and dot shells in
`frontend-modern/src/components/shared/AlertSeverityBadge.tsx`. Platform alert
tables must consume that split instead of recreating provider-local severity
label, text-class, detail-tone, or variant helpers.
Platform alert detail field formatting is also a shared alerts presentation
boundary. `frontend-modern/src/utils/alertDetailPresentation.ts` owns provider
code labels, provider-specific resource-type labels, vSphere alert entity
labels, row timestamp labels, and detail timestamp labels for Docker,
Kubernetes, TrueNAS, and vSphere alert tables. Platform alert tables must call
those helpers instead of recreating local `formatCode`, `formatResourceType`,
`formatEntityType`, `formatStartedAt`, or `detailDateTime` helpers.
Alert history filter defaults such as the all-time period option must likewise
come from the alert overview/history presentation helper and the shared
filter-option label primitive rather than hard-coded title-case strings in the
history filter card.
Alert configuration select options share that same rule: all-channel
escalation labels must come from `alertConfigPresentation.ts` plus the shared
filter-option primitive, not a schedule-page-local `All Channels` string.
Thresholds empty states that hand operators to Infrastructure settings must use
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts` for the
canonical `Settings → Infrastructure` label instead of hard-coding generic
`Settings` copy or removed nested settings paths.
Future alert configuration or history presentation work should
extend those helpers instead of rebuilding alert-specific semantics in pages,
dashboard surfaces, feature hooks, or thresholds shells.

Alert history and threshold resource tables also inherit the shared product
table subgroup-row contract. `AlertHistoryTableGroupRow` and grouped rows in
`AlertResourceTableDesktop` must route their date/resource group bands through
`frontend-modern/src/components/shared/groupedTableRowPresentation.ts` instead
of local `bg-surface-alt` fills, so alert subgroup hierarchy stays visually
consistent with Infrastructure, Workloads, Storage, and Recovery tables.
Alert history table shells must also rely on the shared `TableCard` frame and
the shared `Table` primitive for horizontal overflow rather than adding
alert-local bordered or `overflow-x-auto` wrappers inside the history section.

The alert webhook service chooser also now derives its service set from the
backend webhook template registry, rather than keeping a second frontend-only
list of services, labels, descriptions, and mention-copy metadata.
The WebhookConfig editor now imports the shared webhook template API type
directly so it does not retain a local duplicate shape for chooser metadata.
That webhook editor now also keeps runtime ownership in
`frontend-modern/src/components/Alerts/useWebhookConfigState.ts`, while
`frontend-modern/src/components/Alerts/WebhookConfigList.tsx` owns the
existing-webhook list surface and
`frontend-modern/src/components/Alerts/WebhookConfigForm.tsx` owns the
add/edit form surface. Future webhook template loading, form normalization,
custom-field preset handling, or webhook editor state transitions should land
in those owners instead of being rebuilt inline in
`frontend-modern/src/components/Alerts/WebhookConfig.tsx`.

Alert spec validation still accepts the explicit migration-bridge resource
types (`node`, `agent-disk`, `docker-host`, `backup-subject`,
`proxmox-disk`), but any other non-canonical type string is rejected before
it can reach alert persistence. That keeps alert routing aligned with the
canonical unified resource model instead of silently normalizing legacy type
aliases inside the alert layer.

Frontend alert surfaces and backend alert-support files now require explicit
registry path-policy coverage, so new alert-owned runtime files must be mapped
to a concrete proof route instead of silently inheriting subsystem-default
verification.

The alerts schedule surface now also routes quiet-hour suppress-category card
styling through `frontend-modern/src/utils/alertSchedulePresentation.ts`
instead of leaving that selectable-card presentation inline in
`frontend-modern/src/pages/Alerts.tsx`.
That same schedule/runtime boundary also owns quiet-hours clock semantics.
Backend quiet hours are minute-granular user input, so runtime evaluation must
treat the configured start minute and end minute as inclusive and therefore
keep schedules such as `00:00` to `23:59` active through the full final
minute instead of expiring at `23:59:00`. Alert quiet-hours proofs should
control time through the alert manager clock hook instead of depending on wall
clock execution at whatever second the test runner happens to hit.
Quiet-hours suppression also applies to alert delivery lifecycle, not only the
initial raised notification. Resolved notifications must not fan out when the
alert was never notified or was already acknowledged, and monitoring-driven
escalation delivery must consult the same quiet-hours suppression path while
still letting canonical escalation state reach websocket consumers.
That schedule surface now also follows the same shell/runtime split as the
other feature tabs: `frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx`
stays the render shell, while
`frontend-modern/src/features/alerts/useAlertScheduleState.ts` owns schedule
reset behavior, quiet-hours day/category toggles, cooldown/grouping/escalation
update policy, and the canonical defaults handoff. Future schedule control-flow
work should extend that hook instead of rebuilding those mutations inline in
the tab shell.
The backend cooldown gate is part of that same schedule contract. A disabled
cooldown (`0` or negative) means "do not send periodic re-notifications for
the same active alert"; it still allows the first notification for a new alert
occurrence, while level-escalation delivery remains owned by the separate
escalation path. Runtime evaluation must not treat disabled cooldown as
"always notify" because the alert loop runs every metric tick.

Incident-event filter chip and filter-action styling now routes through
`frontend-modern/src/utils/alertIncidentPresentation.ts` for both
`frontend-modern/src/pages/Alerts.tsx` and
`frontend-modern/src/features/alerts/OverviewTab.tsx` instead of allowing
those alert timeline surfaces to fork their own filter presentation.

Alert incident acknowledged badges, timeline event cards, and note-editor
presentation now also route through
`frontend-modern/src/utils/alertIncidentPresentation.ts` instead of remaining
duplicated inline across the alerts page and overview timeline surfaces.

Poll-driven connectivity recovery is also part of canonical alert truth.
Resources that clear an offline alert from later healthy polls must require
repeated healthy confirmations before resolving that alert instead of clearing
on the first recovered sample; otherwise transient poll recovery reopens the
same regression as false "back online" notifications and missing downtime
signal. Nodes, PBS, and PMG require three healthy confirmations before
resolution, while storage requires two.

Host-agent threshold ownership now follows the linked resource model.
Explicit agent overrides still win, but when no host-agent override exists the
alerts runtime must inherit linked node or guest overrides for that agent so
metric and connectivity behavior match the logical machine the agent augments.
Persisted host alerts must carry enough linked-resource metadata for
reevaluation after threshold changes to honor that same inheritance rule.

Alert resource tables, grouped node headers, and alert override reconstruction
now route resource-backed names through the shared policy-aware alerts helper
so governed resources do not fall back to raw names when the thresholds editor
rebuilds, saves, or re-renders override rows.
Alert threshold tables now route their visible resource row labels, search
labels, and persisted override display names through the same shared helper
so governed agent, guest, and storage rows do not leak raw names when the
threshold editor saves or re-renders them.
That threshold editor data shaping now routes through
`frontend-modern/src/features/alerts/thresholds/thresholdsResourceModel.ts`
for shared override-ID compatibility, grouped resource normalization, and
storage status policy, while
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`
stays the composition owner for the family-specific threshold projectors in
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsHostData.ts`,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsDockerData.ts`,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsGuestData.ts`,
and
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsInfrastructureData.ts`.
backup and snapshot default sanitization plus factory-drift policy now live in
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsRecoveryDefaultsState.ts`,
while `frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`
owns threshold-table route sync, section collapse state, search/edit shell
state, and bulk-edit dialog control. Pure override upsert and hysteresis-entry
helpers now live in
`frontend-modern/src/features/alerts/thresholds/thresholdsOverrideMutationModel.ts`.
Threshold edit persistence, bulk threshold application, and backup/snapshot
override toggles now route through
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsOverrideMutations.ts`,
while powered-off/connectivity state transitions plus alert-removal side
effects now route through
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsAvailabilityMutations.ts`.
That same thresholds host-data boundary now treats top-level TrueNAS appliances
as canonical `agent` resources with `platformType: 'truenas'`. System-disk
group headers must use agent-owned header metadata instead of guest/node-
friendly header metadata, so appliance labels like `TrueNAS Main` do not
collapse to vendor-only `TrueNAS` inside thresholds or override surfaces.
The thresholds tab adapter contract now lives in
`frontend-modern/src/features/alerts/thresholds/thresholdsTabModel.ts`, so
`frontend-modern/src/features/alerts/tabs/ThresholdsTab.tsx` stays a shell
instead of carrying a duplicate table-prop interface and hand-mapped adapter
layer.
`frontend-modern/src/components/Alerts/ThresholdsTable.tsx` is now limited to
shell composition for search/help/nav plus bulk-edit dialog flow, while the
tab render owners live in
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTablePMGTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableKubernetesTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableTrueNASTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableVMwareTab.tsx`, and
`frontend-modern/src/components/Alerts/ThresholdsTablePBSTab.tsx`. New
threshold row grouping, override-ID compatibility, resource normalization,
thresholds-table controller logic, or per-tab runtime should land in those
feature hooks and tab owners rather than being rebuilt inside the shell.
The shell-owned thresholds sub-routes are platform-shaped user-facing paths:
`/alerts/thresholds/proxmox`, `/alerts/thresholds/docker`,
`/alerts/thresholds/kubernetes`, `/alerts/thresholds/truenas`,
`/alerts/thresholds/vmware`, `/alerts/thresholds/pbs`,
`/alerts/thresholds/pmg`, and `/alerts/thresholds/systems`. Legacy neutral
links like `/alerts/thresholds/infrastructure`,
`/alerts/thresholds/containers`, and `/alerts/thresholds/mail-gateway` must
redirect to the matching platform-shaped route. Legacy
`/alerts/thresholds/agents` links must continue to resolve to Systems.
Within the Proxmox tab, render-heavy ownership now further routes through
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxNodesSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxPBSSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestFilteringSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx`,
and `frontend-modern/src/components/Alerts/ThresholdsTableProxmoxStorageSection.tsx`
with the shared section contract in
`frontend-modern/src/features/alerts/thresholds/thresholdsTableSectionProps.ts`.
Future infrastructure-thresholds presentation work should extend those section owners
instead of expanding `frontend-modern/src/components/Alerts/ThresholdsTableProxmoxTab.tsx`
back into a mixed render surface.
The Docker tab now follows that same section-owner shape through
`frontend-modern/src/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerServiceGapSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerHostsSection.tsx`,
and `frontend-modern/src/components/Alerts/ThresholdsTableDockerContainersSection.tsx`.
The containers thresholds surface must consume canonical `app-container`
parents through the shared alert-overrides state rather than assuming
`docker-host` is the only runtime shape. API-backed TrueNAS parents belong in
the same `Container Runtimes` / `Containers` surface, while Docker-specific
controls like ignored prefixes and Swarm service gap settings must stay gated
to real Docker runtimes instead of leaking onto non-Docker platforms.
At the current support floor, TrueNAS alert support means the shared alert
surfaces can evaluate, show, and drill into incidents on TrueNAS-backed
systems, disks, and app parents using the canonical resource model and related
links into infrastructure, workloads, storage, and recovery. Pulse does not
promise a TrueNAS-only alert workflow or provider-specific alert management
surface beyond the shared alerts product.
At the current locked VMware floor, alert support must mean the same shared
alert surfaces can evaluate, show, and drill into vSphere alarm, health, and
threshold signals on canonical `agent`, `vm`, `storage`, and `network`
resources, with related event/task context routed through the shared incident
and resource links. This is the alerts support floor for admitted vSphere
resources; it does not by itself promote the broader VMware platform readiness
state beyond the separately governed platform-admission floor. Pulse must not
grow a VMware-only alert shell, alarm editor, or direct alarm-control surface
in phase 1.
That same VMware alert rule now also includes the topology boundary. Alarm
context that originates on a datacenter, cluster, folder, or resource pool may
inform a shared incident, but it must still resolve onto canonical `agent`,
`vm`, `storage`, or `network` investigation paths rather than creating synthetic
top-level VMware incident resources. If that attachment cannot be done
honestly for a given signal, the signal should remain supporting context
instead of inflating the support claim.
That same VMware alert rule now also includes the timeline boundary. Related
VMware event and task context may enrich shared alert and incident views, but
it must do so through the canonical incident and resource-history paths rather
than through a VMware-only history browser, event drill-down route, or alarm
management shell.
Future Docker thresholds presentation work should extend those section owners
instead of expanding `frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`
back into a mixed render surface.
The systems tab now follows that same shell-versus-section pattern through
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx`
and `frontend-modern/src/components/Alerts/ThresholdsTableAgentDisksSection.tsx`.
Future systems-thresholds presentation work should extend those section owners
instead of expanding `frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`
back into a mixed render surface.
The alert resource thresholds editor now follows the same shape: shared metric
normalization, bounds, value-resolution, and override-label logic live in
`frontend-modern/src/components/Alerts/alertResourceTableModel.ts`, shared group
header presentation lives in
`frontend-modern/src/components/Alerts/AlertResourceGroupHeader.tsx`, desktop
table ownership lives in
`frontend-modern/src/components/Alerts/AlertResourceTableDesktop.tsx`, mobile
card ownership lives in
`frontend-modern/src/components/Alerts/AlertResourceTableMobile.tsx`, render-heavy
desktop row ownership lives in
`frontend-modern/src/components/Alerts/AlertResourceTableRow.tsx`, and selection
state, delay-row toggling, and inline metric-input focus live in
`frontend-modern/src/components/Alerts/useAlertResourceTableState.ts`. Shared
web-interface launch affordances inside alert resource rows and grouped agent
headers must compose
`frontend-modern/src/components/shared/WebInterfaceNameLink.tsx`; alerts own
only the alert/resource data and URL availability decision, not the external
anchor shell, new-tab safety attributes, row-click containment, or accessible
launch-label semantics.
Resource-table empty states, badge labels, offline-state wording, note
placeholders, and metric input titles now route through
`frontend-modern/src/utils/alertResourceTablePresentation.ts` instead of
remaining duplicated across the desktop and mobile thresholds surfaces.
`frontend-modern/src/components/Alerts/ResourceTable.tsx` is now limited to the
shell boundary for breakpoint selection and bulk-edit composition. Future
resource-table threshold semantics should land in those owners instead of
being rebuilt inline in the shell.

Alert incident timeline event cards now route through
`frontend-modern/src/components/Alerts/IncidentTimelineEventCard.tsx`,
while their meta-row, heading, detail, command, and output typography still
route through `frontend-modern/src/utils/alertIncidentPresentation.ts`
instead of keeping duplicate timeline card structure inline in the alerts
page and overview timelines.

Expanded alert incident detail now also routes through
`frontend-modern/src/components/Alerts/IncidentTimelinePanel.tsx` and
`frontend-modern/src/components/Alerts/IncidentEventFilters.tsx` so the
overview surface and the history table share the same loading/error states,
canonical timeline meta row, note editor, and event-filter controls instead
of maintaining two independent incident-detail implementations.
That shared timeline runtime state now routes through
`frontend-modern/src/features/alerts/useAlertIncidentTimelineState.ts`, which
owns incident timeline fetch, expansion state, note-save flow, and shared
event-filter state for both `frontend-modern/src/features/alerts/OverviewTab.tsx`
and `frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`. Future incident
timeline control flow should land in that feature hook instead of being forked
back into either alert surface. Alert incident timeline handoffs into Pulse
Assistant are now owned by the Alerts incident handoff model and carry only
sanitized incident facts plus event summaries into both the visible drawer
briefing and the backend model-only handoff context; raw command and output
details stay in the incident timeline or approval surface.

Resource incident panel cards, summary rows, and toggle-button presentation
now also route through `frontend-modern/src/utils/alertIncidentPresentation.ts`
instead of remaining inline inside `frontend-modern/src/pages/Alerts.tsx`.

That same resource incident panel now treats collapsed incident activity as a
canonical alert read-model summary rather than a page-local sentence. The
collapsed row must summarize filtered incident events by canonical event type
order and reuse the shared event-card renderer for expanded incident detail,
so the alert history page does not drift away from the overview timeline when
canonical lifecycle or remediation events are added.

Active alert card state, acknowledged badge, and primary/secondary action
button presentation now route through
`frontend-modern/src/utils/alertOverviewPresentation.ts` instead of remaining
inline in `frontend-modern/src/features/alerts/OverviewTab.tsx`.
The canonical shared alert-acknowledgement runtime owner is now
`frontend-modern/src/features/alerts/useAlertAcknowledgementState.ts`, which
owns optimistic single/bulk acknowledge control flow, restore behavior, and
notification feedback for `frontend-modern/src/features/alerts/useAlertOverviewState.ts`
and the alert overview render shells.
`frontend-modern/src/features/alerts/useAlertOverviewState.ts` now owns the
derived alert read-model and Last 24 Hours stat refresh for
`frontend-modern/src/features/alerts/OverviewTab.tsx`, while composing that
shared acknowledgement owner instead of keeping its own alert mutation fork.
Future overview action behavior should extend that shared acknowledgement hook
instead of putting acknowledge mutations back into render shells.
Render-heavy alert overview ownership now routes through
`frontend-modern/src/features/alerts/AlertOverviewStatsCards.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewActiveAlertsSection.tsx`,
and `frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx` instead
of rebuilding stats-card, active-alert, and timeline-card presentation inline
inside `frontend-modern/src/features/alerts/OverviewTab.tsx`.
The retired dashboard recent-alert panel must not be reintroduced as a
parallel alert surface. Alert summary/tone copy belongs to the alert overview
presentation owner, and any future compact alert surface must compose the
shared alert read-model and acknowledgement hook rather than creating a
dashboard-only panel.
The Alerts Overview description must stay monitor-first: it names active
incidents and current coverage across monitored resources, and must not imply
that the overview itself owns installation-wide alert activation controls.
The exported English fallback in `alertOverviewPresentation.ts`, localized
message catalog, and header metadata proof must remain textually aligned.

Alert threshold and schedule surfaces must now also treat
`discoveryTarget` as optional frontend input and keep grouping-card state on
the canonical `node` group-header contract. Frontend alert pages may not
assume discovery metadata is always present when deriving override IDs or
toggle styling.

The alerts page shell in `frontend-modern/src/pages/Alerts.tsx` must now keep
destinations, history, schedule, and thresholds rendering feature-owned under
`frontend-modern/src/features/alerts/tabs/`. New alert tab surfaces should be
extracted as feature modules instead of remaining page-local function blocks,
so the page owns navigation and cross-surface routing while tab files own their
runtime presentation, tab-local interaction logic, and any history-table
presentation or thresholds-table adapter logic that does not belong in a shared
primitive.

The history tab itself now follows the same shell-versus-runtime rule. The
canonical history runtime owner is
`frontend-modern/src/features/alerts/useAlertHistoryState.ts`, which now owns
alert-history fetch, persistent filter state, history-clear flow, and
composition of the derived history owners. Resource-incident panel loading,
refresh, and expansion state now live in
`frontend-modern/src/features/alerts/useAlertResourceIncidentsState.ts`, while
the pure analytics model for history-item projection, trend buckets, group
labels, axis ticks, and selected bucket detail now lives in
`frontend-modern/src/features/alerts/alertHistoryModel.ts`. The tab shell in
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` now composes
`frontend-modern/src/features/alerts/AlertHistoryFrequencyCard.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryFiltersCard.tsx`,
`frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableSection.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableGroupRow.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableAlertRow.tsx`, and
`frontend-modern/src/features/alerts/AlertHistoryAdministrationCard.tsx`.
Future alert-history control-flow work should extend the feature hook, new
grouping or trend semantics should extend the history model, and render-heavy
history surfaces should extend those section owners instead of putting fetch,
resource-incident state, or table rendering back into the shell.
That same history surface now also owns the canonical resource-incident
handoff. `frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx`
must treat the selected incident resource as a unified-resource consumer,
linking back into canonical infrastructure/resource detail first and then into
shared workloads, storage, and recovery surfaces through
`frontend-modern/src/routing/resourceLinks.ts` rather than leaving the panel
as a dead-end investigation card or rebuilding provider-local route strings for
platforms such as TrueNAS.
That same alert handoff must now stay on the shared resolved-resource link
builder. `AlertResourceIncidentsPanel.tsx` must resolve its chip set through
`buildResolvedResourceSurfaceLinks(...)`, which owns exact unified-resource
handoffs plus the infrastructure fallback when alert history still references a
resource ID before the backing unified record has hydrated. Future incident-link
work must not reintroduce local infrastructure-link assembly, local dedupe, or
provider-local route strings inside the alert feature shell.

Alert configuration load/save state, notification config reloads, and threshold
override normalization now route through
`frontend-modern/src/features/alerts/AlertsConfigurationSurface.tsx` instead of
living inline in `frontend-modern/src/pages/Alerts.tsx`. The page shell owns
navigation, activation chrome, and cross-surface routing; the configuration
surface is now a shell that composes the destinations, schedule, and thresholds
tabs. The canonical alert-policy runtime owner is now
`frontend-modern/src/features/alerts/useAlertsConfigurationState.ts` for
config transport, notification-config reloads, and save/load orchestration,
`frontend-modern/src/features/alerts/useAlertsConfigurationSnapshotState.ts`
for default-backed mutable config snapshot state plus apply/capture/reset
ownership,
`frontend-modern/src/features/alerts/alertsConfigurationModel.ts` for backend
config normalization, factory defaults, docker-gap validation, and save-payload
serialization,
`frontend-modern/src/features/alerts/alertOverridesModel.ts` for raw override
normalization plus resource-backed override projection. That override owner
must canonicalize legacy per-node shared-storage keys and hashed storage
resource ids onto the storage metrics target id before the thresholds surface
rebuilds, so old Ceph/shared-datastore overrides and newly projected Ceph pool
overrides still surface on the live v6 editor instead of disappearing after
the feature-shell migration, and
`frontend-modern/src/features/alerts/useAlertOverridesState.ts` for reactive
override state, derived resource lists, and overview handoff, and
`frontend-modern/src/features/alerts/alertDestinationsModel.ts` for email and
Apprise config normalization plus outbound payload shaping, and
`frontend-modern/src/features/alerts/useAlertDestinationsState.ts` for
notification destination reload and persistence orchestration.
`frontend-modern/src/features/alerts/useAlertWebhookDestinationsState.ts` now
owns webhook load/mutate/test flow,
`frontend-modern/src/features/alerts/useAlertDestinationsTabState.ts` now owns
destination test actions plus retry orchestration around that webhook runtime,
while
`frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx` stays the
destinations render shell and composes
`frontend-modern/src/features/alerts/AlertEmailDestinationsSection.tsx`,
`frontend-modern/src/features/alerts/AlertAppriseDestinationsSection.tsx`,
`frontend-modern/src/features/alerts/AlertWebhookDestinationsSection.tsx`, and
the dedicated load/error wrappers. Future config cleanup should extend the
config transport hook, the config model, the override-projection hook, or the
shared `frontend-modern/src/utils/alertDestinationsPresentation.ts` helper for
customer-facing destinations copy instead of reviving inline retry, test, and
error text across the feature tabs.
destinations runtime hook based on which subsystem actually owns the behavior
instead of letting the broader configuration hook absorb all four concerns
again.
The email destination provider picker now follows that same split:
`frontend-modern/src/components/Alerts/useEmailProviderSelectState.ts` owns
provider-catalog loading and provider-default application, while
`frontend-modern/src/components/Alerts/EmailProviderSelect.tsx` stays the
render shell and consumes the canonical `UIEmailConfig` feature type instead of
keeping a second local email-config interface.
The alert scheduling surface now follows the same shell/section split:
`frontend-modern/src/features/alerts/useAlertScheduleState.ts` owns schedule
runtime and default/reset policy, while
`frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx` stays the shell and
composes the dedicated quiet-hours, cooldown, grouping, recovery, escalation,
and summary section owners instead of carrying those panels inline.

Alert filter metadata and grouped header consumers must also preserve the
canonical `agent` and `node` header boundary when reusing shared filter
primitives. Frontend alert tables may not drift back to ad hoc host-key
grouping or narrow filter key predicates that drop optional hostname values
before alert group metadata is derived.
That same shared alert boundary now also owns provider-backed `resource-incident`
alerts beyond storage-only cases. `internal/alerts/model.go`,
`internal/alerts/unified_incidents.go`, and
`frontend-modern/src/utils/alertIncidentPresentation.ts` must treat VMware-
backed host and VM incidents as the same canonical `resource-incident`
vocabulary used everywhere else, with quiet-hours routing derived from the
shared incident category and provider context carried only as shared alert and
timeline metadata. Alert history may surface VMware alarm, task, and snapshot
context inside that shared model, but it must not fork into VMware-only alert
types, badges, or incident chrome.
That same alert-shell boundary now also treats websocket access as a shared
app-runtime dependency rather than an alerts-owned provider. Alert shells such
as `frontend-modern/src/pages/Alerts.tsx` and
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` may consume live
state only through `frontend-modern/src/contexts/appRuntime.ts`; they must not
import `@/App` or create reverse dependencies into the root shell chunk,
because alerts surfaces must remain lazy-load safe and must not blank the app
before auth/bootstrap finishes.
Alert commercial handoffs now also follow the runtime-versus-commercial split.
`frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx` may resolve
upgrade destinations through the shared commercial-posture store, but alert
runtime availability and chat enablement must stay governed by the
non-commercial app runtime and assistant state instead of reusing the same
commercial payload as feature truth.
Alert schedule and incident-timeline surfaces now also keep their browser state
typed through one feature-owned contract. Quiet-hour suppress options must be
cloned into mutable feature props before crossing section boundaries, quiet-day
callbacks must preserve the canonical weekday key union, and incident timeline
expansion/note-saving state must remain `Set<string>`-owned instead of drifting
to untyped browser-local collections.
That same alerts runtime boundary also owns canonical identity derivation and
active-alert persistence. Shared canonical identity helpers may infer resource,
spec, and state ids from legacy alerts, but they must do so without mutating
live in-memory alert instances unless the caller explicitly backfills that
state. Persisted active-alert snapshots must therefore clone alerts under lock,
backfill canonical identity on the clone, and serialize that snapshot instead
of mutating the live alert map during async saves or incident rebuilds.
That same ownership also governs acknowledgement and manual-clear cleanup.
Clearing an alert through the canonical alerts runtime must remove both legacy
public-id tracking and canonical-state acknowledgement records so old aliases
cannot keep an alert acknowledged after the canonical alert has been removed.

### Operational Trust writable lifecycle

`internal/alerts/active_lifecycle.go` is the single writable owner for
acknowledge, unacknowledge, suppress, unsuppress, collection-stale,
collection-unknown, resolving, and collection-restored transitions. Every
writer is idempotent under retry, retains new evidence without duplicating a
same-state transition, persists through the active-alert store, and survives
restart. Explicit stale, unknown, resolving, or suppressed state takes
precedence over the legacy `Alert.Acknowledged` projection. The canonical
record's `LastObservedAt` is the maximum of retained record, alert, and evidence
timestamps so a fresh outage observation cannot be rolled back by an older
legacy alert timestamp.

Suppression is bounded and reasoned, leaves the default active queue, and
remains inspectable. Expiry or explicit unsuppression returns the record to its
detector-owned state; it never resolves it. Only fresh sufficient recovery
evidence may enter resolving, and only detector recovery may resolve.

### Canonical threshold-override succession

Alert threshold overrides are persisted under the current canonical resource
ID. `internal/alerts/canonical_override_migration.go` may re-home an override
from a retired ID only when the unified-resource owner explicitly publishes
that ID in `SupersededCanonicalIDs`. Display aliases, hostnames, metric
targets, connection labels, and other lookup conveniences are not persistence
authority. A still-live old ID or one old ID claimed by multiple current
resources fails closed and remains untouched; when both keys exist, the current
canonical override wins.

Monitoring owns the synchronization point that applies this migration to the
active alert configuration and persists it before later reloads. The frontend
may read legacy connection-target keys as compatibility candidates, but save,
edit, and delete operations target the resource's current canonical ID and
retain that ID across refetches or reported-identity changes. This migration
changes alert configuration only and does not create customer-infrastructure
mutation authority outside canonical Actions.

`internal/alerts/canonical_override_migration_test.go` proves the
unambiguous/live/ambiguous decision matrix,
`internal/monitoring/monitor_alert_override_migration_test.go` proves durable
reload behavior, and
`frontend-modern/src/features/alerts/thresholds/hooks/__tests__/truenasThresholdPersistence.test.tsx`
proves the browser-side TrueNAS save/refetch contract.

### Versioned alert-intent policy

The alerts runtime owns one versioned alert-intent document and its durable
pending-condition state. Stable signal keys are `*`, `state.offline`,
`incident.availability`, and `metric.<name>`. Effective fields resolve
independently from legacy metric behavior, document defaults, resource-type
overrides, and canonical-resource overrides, in that order. Keys are
normalized before validation and collisions fail closed; updates use revision
compare-and-swap so a stale browser cannot overwrite a newer document.

Intent affects when detector truth becomes eligible for an active alert; it
does not mutate the underlying observation or create a second alert identity.
Without an explicit applicable rule, established alert behavior remains
compatible. With a rule, the first matched time is durable across restart and
becomes the canonical alert start time once eligible. Preview evaluates the
same resolver but restores pending state before returning, so it is read-only.
Invalid documents, persistence failures, or revision conflicts must leave the
prior in-memory and durable policy active.

Operator maintenance and intentionally-offline state are read only through the
canonical unified-resource identity. Backup-aware offline deferral consumes
fresh, matching, active task evidence, applies the configured post-backup grace,
and always terminates at its hard cap. Missing, stale, future-skewed,
finished, or mismatched backup evidence cannot suppress an outage. This policy
changes alert activation only: notification delivery, recovery assurance, and
customer-infrastructure mutation retain their existing owners.

`internal/alerts/intent_policy_test.go` proves precedence, normalization,
operator and backup contexts, preview immutability, restart continuity, and
first-match lifecycle identity.
