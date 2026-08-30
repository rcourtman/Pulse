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
Alert-history investigations on narrow layouts render in a shared drawer
outside the virtualized history row. The drawer title comes from the canonical
alert-incident presentation vocabulary. A resource-incidents panel embedded in
that drawer suppresses its standalone panel title so the investigation has one
accessible heading, while the desktop inline panel retains its own title.
Alert schedule delivery routing is one persisted, normalized contract.
`schedule.initialNotify` accepts `all`, `email`, `webhook`, or `apprise`;
missing, legacy, and unknown values preserve the backward-compatible `all`
target. The selected initial target owns firing, grouped, and recovery delivery,
while every escalation level retains its own independently normalized target.
Saving the alert configuration must update the live notification manager as
well as persistence so delivery does not differ before and after restart.
Configuration-change reevaluation may resolve metric-backed alerts against
their updated thresholds and may apply explicit resource-disable policies, but
it must not treat provider-owned incidents as missing thresholds. Unrelated
configuration saves preserve those incidents and their acknowledgement state
until their provider evaluator supplies recovery evidence.
When a VM or container stops, guest evaluation resolves only metric-threshold
alerts whose observations are no longer meaningful. Backup-age and snapshot
posture remain owned by their posture evaluator and may stay active while the
guest is powered off; stopped state is not backup-recovery evidence.
Docker and Podman container CPU thresholds evaluate host-capacity-normalized
CPU percent, not Docker's runtime-native per-core percent. Alert metadata may
carry the raw per-core value and reporting host CPU count for evidence, but the
threshold value and canonical `cpuPercent` metadata remain normalized.
Docker and Podman container health is live evidence only while the container
state is `running`. Runtime APIs retain the last health-check result after exit
and suspend health checks in other non-running states, so those stale values
must clear rather than raise `docker-container-health`; the canonical runtime
state lifecycle owns the non-running condition.
Proxmox VM and LXC CPU thresholds consume the same canonical guest CPU-percent
normalizer as unified live state and guest history. Proxmox guest CPU is the
authoritative observation for that guest; a Pulse host agent running inside the
guest may retain its own agent alert identity, but its host CPU observation
must not replace or renormalize the Proxmox guest value.
Docker and Podman OOM alerts require authoritative runtime evidence: the
container must be stopped (`exited` or `dead`) and its reported `OOMKilled`
state must be explicitly true. Exit code 137 alone is only SIGKILL evidence;
explicit false and unavailable/legacy OOM state both fail closed without an OOM
alert. Recovery clears an existing OOM alert when the authoritative predicate
is no longer true.
Backup-age alert attribution of an unlinked recovery rollup treats the
subject ref's namespace field as a connection label, not a PBS namespace: it
may match a candidate guest's instance or node only by exact normalized
equality. Suffix matching is reserved for real PBS namespaces inside the
shared identity helpers; applying it to a PBS connection name
cross-attributes clusters that share a VMID, so an unlinked backup whose
label identifies no guest exactly stays on its generic rollup key. The PBS
submission-source learner that shares those identity helpers is backup
attribution state, not alert identity: it is built per evaluation from
positively attributed snapshots, never persists across evaluations, and is
inconclusive by default, so it can only narrow which guest an already-linked
backup belongs to and can never widen, merge, or move an alert's subject.
Availability incident and alert identity belongs to the source-owned
`network-endpoint` check. Correlation may project probe evidence onto a matched
machine, but it must not copy the check incident onto that machine or create a
second alert lifecycle. Failure, recovery, history, acknowledgement, and
notification routing therefore remain stable under relinking and restart.
Certificate validity uses that same source-owned incident lifecycle. Codes
prefixed `certificate_` map to alert type `certificate`: expiry-window incidents
are warning, while expired, not-yet-valid, and untrusted incidents are critical.
A self-signed certificate is not an untrusted-chain alert. Certificate alerts
reuse canonical confirmation, acknowledgement, history, recovery, schedule,
intent, and notification routing and must not create a parallel delivery path.
Linux memory thresholds consume only canonical cache-aware usage. An explicit
usage-unavailable sample, a non-finite percentage, or contradictory
used/free/total evidence must not open or clear a memory alert. If such a
sample follows an active alert, the alert remains active with its last trusted
value until a subsequent trusted sample proves recovery; missing evidence is
not evidence that pressure disappeared.
Proxmox guest disk-read, disk-write, network-in, and network-out thresholds
consume only monitoring-owned valid rate observations. A valid idle interval
is explicit zero and may prove recovery; a first sample, missing/null counter,
partial response, or rejected out-of-order sample is unknown and must not
start, clear, or match a custom metric filter. Alert units remain MiB/s at the
threshold boundary (`bytes/s / 1024 / 1024`); that display/threshold conversion
must not be applied to the upstream cumulative-counter divisor.


Disk wearout is an evidence boundary, not a plain threshold. The field carries
two distinct absent states and one real zero: `-1` is the canonical unreported
sentinel, and `0` is genuine evidence only from a device that reports endurance
at all. Rotational disks never do, so a `0` from one is absence. Alert
evaluation must gate on `storagehealth.WearoutReported` rather than carrying its
own inline boundary. A wearout arm keyed on `> 0` silently exempts the single
worst reading a disk can publish, which let a spent SSD read critical on the
Physical Disks surface while raising no alert at all.

Host SMART counter growth is an event boundary rather than a warning on every
historical non-zero value. For an agent-only disk, the first reported UDMA CRC
error count establishes an in-memory baseline and raises no alert. A later
increase contributes `crc_errors_increased` as a warning reason to that disk's
canonical `disk-health` assessment; the next stable sample resolves the active
growth event while normal alert and notification history retain it. A lower
counter, including a reset or disk replacement, establishes a new baseline and
must not alert. Missing or negative evidence cannot create a baseline or prove
growth. Baselines are bounded tracking state, are discarded by a full alert
state reset, and expire after the standard stale-tracking window. Linked
Proxmox host agents continue to defer SMART risk alert ownership to the
provider-backed disk surface rather than creating a duplicate host alert.
`TestCheckHostAlertsWhenSMARTCRCCountIncreases`,
`TestCheckHostSMARTCRCCounterResetEstablishesNewBaseline`, and
`TestCleanupRemovesOnlyStaleSMARTCounterSnapshots` in
`internal/alerts/alerts_test.go` pin the baseline, growth, reset, recovery, and
retention boundaries.

Host SMART alert policy is configured through the resolved agent threshold
chain, not embedded inside the disk evaluator. Missing SMART rule fields seed
the backward-compatible policy: failed health is enabled; reallocated,
pending, uncorrectable, media-error, and CRC-growth counters trigger at one;
remaining-life warning/critical thresholds are 10/5 percent; and NVMe spare
warning/critical thresholds are 20/10 percent. An explicit zero disables that
individual rule and must survive normalization, cloning, persistence, and
per-machine override resolution. Counter rules trigger when current evidence
reaches the configured value, while the CRC rule applies the configured
minimum increase to two successive reports. Endurance and spare values remain
evidence-aware, and invalid percentage inputs clamp to 0..100 with critical no
higher than its warning boundary. The Machines threshold surface must
round-trip every rule without changing these defaults. Linked Proxmox host
agents continue to defer risk alert ownership even when their resolved SMART
policy differs.

Host SMART recovery is field-evidence-aware. A standby report preserves disk
identity but carries no health authority, so it must not open or clear either
the canonical `disk-health` or `disk-wearout` assessment. For an active SMART
alert, every still-enabled reason that raised the occurrence must be observed
again before recovery evaluation; an omitted counter or endurance field is
unknown rather than a healthy zero. Explicitly disabling the owning SMART rule
remains authoritative and may clear the alert without waking the disk.
`TestCheckHostSMARTDiskAlertRequiresRelevantRecoveryEvidence` in
`internal/alerts/alerts_test.go` pins the fire, standby hold, partial-evidence
hold, and authoritative recovery sequence.

Threshold sections are keyed by override identity, not by resource type. The
Virtualization Hosts section reads and writes overrides on the bare resource id,
while the Machines section resolves through the agent-derived identity
candidates. A resource that appears in both sections therefore has two different
override keys, and a threshold saved from the wrong one is written where
alerting never reads it, leaving the machine on the global default with nothing
on screen to explain it. A resource must appear in exactly one threshold section,
the one whose identity alerting honours. Standalone Pulse-agent machines belong
to Machines, so the Virtualization Hosts section is fed by provider-owned
Proxmox PVE nodes rather than by every resource of type `agent`. Canonical
TrueNAS and vSphere hosts are also represented as `agent` resources, but they
belong only to their platform threshold sections; admitting either to
Virtualization Hosts would normalize edits against Proxmox node defaults and
could silently discard a valid platform override whose value happens to equal
the Proxmox default.

QEMU guest-agent filesystems have their own threshold identity:
`guest-disk:<stable-guest-key>/disk:<mount-device-key>`. The stable guest key
must use the same cluster-aware alias chain as guest thresholds so a VM node
move does not strand its filesystem settings. A filesystem override may disable
that mount or supply its own disk threshold. Once a mount has a dedicated
override, it is evaluated only against that override and is excluded from the
guest aggregate disk alert; otherwise the aggregate would still fire at the
guest default and silently defeat a disabled or relaxed mount. The visible
threshold row keeps the live `guestID-disk-*` alert resource identity for
active-state and intent-policy linkage while persisting through the stable
`guest-disk:*` override identity.

A per-metric threshold is off whenever its trigger is `<= 0`. That boundary is
engine truth (`internal/alerts/canonical_metric.go`,
`internal/alerts/config_runtime.go`), not a display convention: `-1` is the
value Pulse writes, and `0` disables the metric just as completely because
earlier builds advertised `0` as the disable value and those overrides are still
on disk. Every threshold editor reads that same `<= 0` rule through
`frontend-modern/src/components/Alerts/alertResourceTableModel.ts` rather than
testing for `-1` inline: read cells, the row override editor, the global
defaults row, and the bulk edit dialog, on both the desktop and the mobile
surface. A surface that recognises only `-1` reports On for a metric the engine
is not alerting on at all, and an unset global default carries the same
obligation, because absent is disabled rather than enabled.

Turning a metric back on must write a value the save path will actually
persist. Clearing an override only re-inherits the default, so it re-enables
nothing when the inherited default is itself off, and an undefined entry in a
bulk edit reads as unchanged rather than as on; in both cases the editor stages
the metric's enabled default instead, and only clears the override when the
inherited default is already enabled. Editors must also not manufacture an off
state out of an empty input: a cleared box is mid-edit, and coercing it to `0`
disabled the metric in the engine while the row still showed On. Disabling is
the toggle's job, and the toggle writes the canonical `-1`.

External availability-probe reporting loss owns one canonical
`external-probe-unavailable` alert per assigned agent, not one alert per target.
The lifecycle waits for the assignment grace floor before firing, keeps target
IDs as bounded supporting metadata, yields to the existing host-offline
lifecycle when the agent heartbeat is unhealthy, and resolves only when a
fresh result from the currently assigned agent arrives. Notification delivery,
acknowledgement, history, and recovery reuse the normal alert pipeline.

Storage capacity forecasting is an alert-grade evidence boundary, not an AI
prose feature. It requires at least 24 hours of valid history, hourly median
normalization, a fresh terminal sample, agreement between full-window and
recent positive slopes, and confidence of at least 0.80 before it can fire.
Raw poll count alone must never manufacture confidence. Forecast risk opens
only when projected exhaustion is within seven days (critical within one day)
and recovers only after the projection moves beyond fourteen days or trusted
evidence proves growth stopped. Missing, shallow, stale, or low-confidence
history is unknown rather than recovery for an already-active forecast.
Predictive and static usage policy share `metric-threshold:usage` as one
canonical occurrence: a forecast that later crosses the percentage threshold
keeps its start time, acknowledgement, history, and timeline instead of
emitting a forecast recovery plus a second capacity alert. The same contract
applies to Proxmox, Ceph, TrueNAS pools/datasets, and vSphere datastores through
their existing platform thresholds and disable policy.

Rolling metric evaluation is alert policy over monitoring-owned history, not a
second incident family. `metricEvaluationWindows` stores seconds by canonical
resource type and metric, with an explicit zero meaning current-value
evaluation. The Thresholds surface exposes both the global `all` rule and the
canonical `guest` workload fallback; concrete workloads inherit through the
same runtime chain (`vm` and `app-container` through `guest`, host-like agents
through `node`, then `all`) and must label the effective parent duration rather
than always presenting the global value. CPU seeds a five-minute default; only
CPU and burst-prone disk/network rate metrics may use
rolling averages, while memory, capacity, and temperature remain instantaneous
evidence boundaries. A window requires at least three samples, at least 80%
temporal coverage, and no gap larger than the bounded cadence allowance. Weak,
stale, or unavailable history is unknown: it cannot open or recover an
incident, and an existing incident retains its last trusted value. Ready
windows use time-weighted averaging so polling jitter cannot bias the result.
The evaluated value drives trigger, hysteresis, severity, delay, and recovery;
the current value, sample count, coverage, and window remain alert metadata and
operator-visible message evidence. Window changes never change the canonical
`metric-threshold:<metric>` identity, so start time, acknowledgement, history,
timeline, escalation, and recovery stay one occurrence.

Active-alert restore is opt-out at construction. `NewManagerWithDataDir` accepts
`ManagerOption` values, and `WithoutPersistedAlertRestore` starts the manager
with an empty active-alert set instead of reading `active-alerts.json`. Mock
mode selects it: switching the mock toggle already clears active alerts, but a
process that boots with mock mode already enabled never runs that path and would
otherwise restore alerts raised against real infrastructure and serve them
beside fixture data. The option changes startup restore only. Alert evaluation,
persistence, notification, acknowledgement, and history are unaffected, and the
default construction path still restores.

## Canonical Files

1. `internal/alerts/specs/types.go`
2. `internal/alerts/specs/evaluator.go`
3. `internal/alerts/canonical_metric.go`
4. `internal/alerts/canonical_lifecycle.go`
5. `internal/alerts/unified_incidents.go`
5a. `internal/alerts/incident_synthesis.go`
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
47a. `internal/alerts/capacity_forecast.go`
47b. `internal/alerts/windowed_metric.go`
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
76. `internal/alerts/issue1497_test.go`
77. `internal/alerts/system_alert.go`
78. `internal/alerts/event_emission.go`
79. `internal/alerts/eventlog/eventlog.go`
80. `internal/alerts/history_projection.go`
81. `internal/alerts/history_migration.go`
82. `internal/alerts/active_state_bootstrap.go`
83. `internal/alerts/eventlog/active_state.go`
84. `frontend-modern/src/features/alerts/AlertDeadManDestinationSection.tsx`

## Shared Boundaries

1. `frontend-modern/src/stores/websocket.ts` shared with `performance-and-scalability`: the connection-owned realtime store is both the canonical alert truth boundary and the fleet-scale resource reconciliation hot path.
   That shared store normalizes slimmed broadcast resources at ingestion —
   expanding `capabilitiesRef` through the state `capabilityCatalog` and
   synthesizing the default policy posture for resources published without one
   — before any alert consumer resolves alert-to-resource identity or policy
   display. Alert override and threshold identity matching must consult
   `canonicalIdentity.supersededIds` alongside aliases, because broadcast
   aliases no longer duplicate superseded canonical ids.
2. `internal/operationaltrust/contracts.go` shared with `notifications`: the operational trust contract is jointly consumed by canonical alert lifecycle ownership and notification delivery linkage without making delivery state operational truth.
3. `internal/proxmoxidentity/backup_identity.go` shared with `monitoring`, `storage-recovery`: Proxmox PBS backup subject identity is a shared runtime boundary for monitoring backup freshness, backup-age alert attribution, and recovery-point guest mapping.
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
The notification-delivery health card is an alerts-owned presentation over
notification-owned queue truth. When retained terminal failures exist, the
Destinations surface exposes explicit retry and dismiss actions with
consequence confirmations, refreshes health and delivery history after either
action, and never instructs the operator to delete queue storage.
Platform `connection-degraded` alerts are availability observations of their
owning PVE, PBS, PMG, VMware, or TrueNAS resource, not a separate policy
surface. They must honor that resource's disabled and connectivity-disabled
override, the platform-wide alert and offline-alert switches, offline intent
and quiet-hours policy, and must clear immediately when that policy becomes
disabled. The connection snapshot must carry the owning monitor resource ID
used by registry alias resolution; the ledger's display ID is not a substitute
for that policy identity. A second connection detector must never notify around
a resource's offline-alert toggle.
Alert runtime state has one explicit ownership boundary: `AlertConfig.enabled`
controls detector evaluation and in-product alert visibility, while
`AlertConfig.activationState` controls external notification delivery only.
The websocket store, resource-row presentation, navigation counts, and Alerts
overview must preserve active alert truth while notification delivery is
pending review or snoozed. Notification activation must not clear the browser
active-alert store, suppress resource indicators, lock threshold/history
configuration, or claim that monitoring has stopped.
The websocket store's raw resource-delta baseline is connection-scoped. Closing
or replacing a socket invalidates that baseline and its recovery throttle; a
delta from the replacement connection must not patch the previous connection's
raw snapshot. Only a full snapshot delivered over that socket may establish
the new delta baseline. The same lineage rule applies independently to the
connected-infrastructure and active-alert keyed projections. Dropping an
oversized state frame invalidates all three raw baselines and their queued
projection work. Oversized-state REST recovery may refresh the current
connection's displayed resources, infrastructure, and alerts, but it is
independently built and must remain delta-free; a later keyed delta without a
socket-owned baseline is ignored and requests the shared throttled recovery
path. A marker or baseline-less delta observed during hydration must coalesce
one trailing REST refresh after the throttle window so the latest invalidation
is not lost. Alert deltas still apply immediately when their socket baseline
exists. Late socket callbacks and REST responses from a retired connection
must be ignored, while a current oversized connection remains free to hydrate
without waiting for the retired connection's request to settle.
Cold active-alert hydration has a narrower recovery path than oversized estate
state. Until the canonical store has accepted either a socket-owned alert
snapshot or a successful `GET /api/alerts/active` response, the Alerts overview
must expose `pending` or `unavailable` truth and must not turn an empty local
store into a "No active alerts" all-clear. An unintentional socket close before
that first alert snapshot starts one throttled active-alert REST recovery from
the canonical store, and an open connection that remains alert-snapshot-free
for five seconds starts the same recovery rather than waiting for the
ninety-second heartbeat timeout; pages must not own an independent alert fetch
or cache.
That response may refresh display truth but must leave `rawActiveAlerts`
baseline-free, and an alert revision plus request-generation fence must discard
it if newer socket truth, a URL-scope switch, or disposal wins the race. Once a
snapshot is known, reconnects retain it as the last confirmed projection rather
than replacing known alert state with transport uncertainty. A failed recovery
must remain visibly unavailable and provide an explicit operator retry.
While the document is hidden, that same connection-scoped baseline must keep
accepting resource deltas without reconciling the visible resource store on
every message. The store accumulates changed resource IDs (and their per-key
change shapes, unioned across the hidden ticks with unknown-shape
contamination) and performs one canonical catch-up reconciliation on
`visibilitychange` to visible. Alert and
resolved-alert truth continues to update normally while hidden; this resource
optimization must not defer, clear, or reinterpret alert lifecycle state.
Active operator input applies the same separation: scroll, wheel, pointer
press, and key activity may defer resource deltas, reporting projection, and
the shared realtime tick token until the input-idle flush, but alert and
resolved-alert payloads must still commit on the message that carries them.
The input gate must never delay acknowledgement, recovery, navigation counts,
resource alert indicators, or notification-facing alert truth merely because
the resource projection is waiting for an idle window.
The visible-tick resource commit may apply metrics-only rows as per-key
subtree writes instead of whole-row reconciles, but alert truth is outside
that fast path entirely: active-alert and resolved-alert stores keep their
own commit path, resource `alerts` facets are not in the fast-path
allow-list, and a row whose patch touches alert-relevant structure always
takes the full canonical merge.
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
4. Add or change alert history persistence through `internal/alerts/history.go`,
   `internal/alerts/history_projection.go`, and `internal/alerts/eventlog/`
   using normalized owned storage roots and fixed storage leaves only.
   Event-log projection changes must preserve parity with the in-memory
   JSON-history model, and migration changes must retain a retryable source or
   backup until every legacy entry is durably imported.
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
    discoverable where alert routing is configured. It may read and update the
    Relay destination's alert minimum severity, but routes pairing and
    connectivity setup to the canonical `/settings/system-relay` Remote Access
    panel rather than duplicating Relay runtime state. When the `relay` feature
    is absent it gates through the shared
    `FeatureGateSection` and upgrade-navigation contract, rendering no upgrade
    call-to-action when prompt suppression applies.
11. Add or change system-scoped alerts through
    `internal/alerts/system_alert.go`. A system-scoped alert reports on Pulse
    itself rather than on a monitored resource, for conditions the operator
    cannot observe from outside the product. Notification delivery is the
    founding case: the channel that would carry the warning is the thing that
    failed, so the alert list and navigation badge are the only escalation
    path that does not depend on delivery working. System alerts carry the
    stable `pulse-system-` identity prefix, set no `ResourceID` so
    resource-linked affordances are skipped rather than pointed at nothing,
    and stamp `systemAlert` metadata. Raising must stay idempotent for an
    unchanged condition so a timer-driven evaluator cannot become a
    notification storm; only a change of level or message re-notifies. New
    system alert types must be raised through this helper rather than by
    constructing bare alerts, so identity and the idempotence guarantee stay
    in one place.

## Forbidden Paths

1. New ad hoc `Check*`-local evaluator logic
2. Reintroducing runtime legacy alert-ID contracts
3. Reintroducing per-family threshold/override merge logic outside the shared path

## Completion Obligations

1. Update alert spec/evaluator tests when a new rule kind is added
2. Update this contract if alert truth or identity rules change
3. Route runtime changes through the explicit alert proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten or add guardrails when an old alert path is removed
5. Update the event-log schema upgrade and history-projection parity proofs
   when lifecycle snapshots, occurrence folding, or alert-history authority
   changes

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

### Alert-quality telemetry folds only canonical durable lifecycle truth

`internal/alerts/telemetry_quality.go` folds active alerts and the canonical
30-day history projection into identity-free aggregate counts. Severity totals,
active-age buckets, resolution-duration buckets, repeated occurrences, and
snooze outcomes are computed locally. Canonical state and occurrence time may
be used transiently for folding, but neither leaves the process. The snapshot
also exposes tenant-level configuration and event/active-state authority counts
so downstream rates have denominators and persistence failures remain visible.

The contract deliberately does not classify automatic versus operator
resolution because alert resolution currently records recovery evidence, not a
resolution actor or operator-resolve action. It does not claim maintenance
suppression effectiveness because an intent policy can suppress a candidate
before an alert occurrence exists. It also excludes detected flapping episode
counts because that diagnostic event path may drop under pressure. Flapping
configuration adoption remains measurable. `telemetry_quality_test.go` pins
occurrence folding, snooze sequencing, and exact bucket boundaries.

The alert webhook editor exposes the delivery contract's language-neutral
`MessageKey`, event, resource type and node display name alongside the existing
type, severity and metric fields. Presentation must advertise the backend-owned
template data rather than asking operators to parse the English alert message
or reach into metadata for canonical alert identity.

### Agent custom sensors use canonical health-assessment alerts

Typed `HostSensorSummary.Custom` numeric, boolean, or timestamp readings with
`warning` or `critical` status map to one canonical `custom-sensor` alert per
host and sensor ID. Alert evidence preserves the optional group, subgroup,
kind, and event time alongside the evaluated numeric value.
Collection errors map to warning only when the local definition reports
`alertOnError`; healthy reports, removed definitions, disabled host thresholds,
host removal, and telemetry expiry clear the same identity. Alert evaluation
consumes the agent-authored typed status and never executes or receives the
local command or REST URL. `TestHostCustomSensorAlertLifecycle` and
`TestHostCustomSensorErrorCanBeReportOnly` in
`internal/alerts/host_unraid_lifecycle_test.go` pin creation, recovery,
opt-out, and cleanup.

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

On desktop that panel renders inline under the history table row that opened
it (`AlertHistoryTableAlertRow.tsx`). On phone layouts, Resource and Timeline
investigations render through
`MobileAlertHistoryInvestigationDialog.tsx` as one full-height, independently
scrollable investigation drawer. They must not expand inside
`AlertHistoryMobileList.tsx`: the history is a fixed-estimate virtualized list,
so changing one mounted card to incident-timeline height invalidates its spacer
math and makes the app scroll shell jump or pull the operator back up the page.
The mobile drawer keeps that list height and scroll position stable, contains
overscroll within the evidence surface, supports Escape and explicit close,
and returns focus to the originating action or the history list fallback.
It must not become a page-level sibling in `tabs/HistoryTab.tsx`, which would
again open away from a reader deep in history (#1687). Because several alerts
can share one resource, `resourceIncidentPanel` still carries the originating
`rowKey`, and the mobile investigation state allows exactly one Timeline or
Resource surface at a time. Resource incident expansion and filtering remain
reactive accessors inside each keyed incident row; snapshotting the expanded
set or filter set would let state update while the visible Events timeline
stays closed or stale.

The alert history filter bar carries no saved-views affordance.
`AlertHistoryFiltersCard.tsx` must not pass a `savedViewsKey` to the shared
`FilterBar`; that prop and the localStorage view library behind it were
removed because a saved view was only ever the page's URL query string, which
the browser's own bookmarks already capture and share. History narrowing stays
URL-owned so a filtered history page remains a shareable, bookmarkable link.

The alert history severity facet derives each option count through
`useAlertHistoryState.countForSeverity`, using the same
`filterAlertHistoryItems` predicate that supplies the rendered list for the
current fetched period and search term. It must not count an unfiltered or
separately reduced collection that can disagree with the selected chip's
result. These counts follow the shared Inventory totals visibility preference;
the Period facet remains uncounted because it selects the fetched time scope
rather than filtering the already-fetched rows.

Alert severity is one closed, end-to-end vocabulary: `info`, `warning`, and
`critical`. Alert creation, restored active state, history projection,
incidents, operational-trust projections, API responses, sorting, resource
highlighting, overview cards, badges, and history filters must preserve those
three meanings. The legacy history value `error` normalizes to `critical`;
empty or unknown runtime values fail safe to `warning` and must never become
low-urgency information accidentally. Informational alerts use the shared blue
presentation and sort below warnings, while the history facet exposes a real
Info option whose count and filtered rows use the same predicate as every
other severity.

Alert history row timestamps render clock time in the viewer's own locale and
must carry the absolute date and time as a title. The date otherwise lives
only in the day group header, which scrolls out of sight, and a hardcoded
`en-US` format misreported the time of day to everyone outside the US
(#1687, #1685). `useAlertHistoryState` owns both formatters
(`formatAlertRowTime`, `formatAlertRowTimestamp`) so the desktop table and the
mobile list cannot drift apart. The day group `fullLabel` built in
`frontend-modern/src/features/alerts/alertHistoryModel.ts` follows the same
rule, because a header reading "Thursday, August 6, 2026" to a reader whose
rows are ordered day-month is the same defect one level up. See the
localization boundary in the frontend-primitives contract for the shared
formatting rule and the audit that enforces it.

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
The reducer core owns transitions for the canonical stateful family: health
assessments, posture thresholds, and change thresholds, including ZFS pool
state, backup and snapshot age, and Docker update-delay state. Legacy
pending-since maps are read-only mirrors during migration and must not decide
activation. Stateful occurrences retain their established timestamp boundary:
the current observation, or a caller-supplied override such as the backup
timestamp, rather than the pending-run start. A manual clear must pass its
clear time into the reducer, record the firing incident as recently resolved,
and start acknowledgement retention so a refire inside five minutes restores
the same occurrence without duplicating history. Regression ownership is
`internal/alerts/canonical_stateful_test.go`.
The browser thresholds surface is also platform-shaped: Proxmox, Docker,
Kubernetes, TrueNAS, vSphere, PBS, PMG, and Systems. Route-backed platform
choices must use the shared `Subtabs` navigation above the shared `FilterBar`;
they are not resource-filter facets and must remain visible when the mobile
filter shell is collapsed. Resource filtering must use the shared FilterBar
chip and "+ Filter" pattern, and alert tables must use the canonical platform
table column-kind alignment helpers from
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
The Recovery and Snapshot Age threshold sections expose exactly one Global
Defaults surface: the always-live editor row (table layout) or card (card
layout) that the shared resource table renders when global defaults are
supplied. They must not add a synthetic read-only resource row mirroring the
same record; a second surface invites edits that no override mutation path
persists, which is how #1680's silently-dropped edits happened. Snapshot size
thresholds belong to the Snapshot Age section, not Recovery, because
`BackupAlertConfig` has no size dimension. The recovery-defaults records carry
the normalized `warningSizeGiB` and `criticalSizeGiB` metric keys that the
column editor resolves for the size columns, and the configuration snapshot
layer must round-trip those fields: `applyAlertsConfigToSnapshot` and
`buildAlertsConfigurationPayload` carry them between the UI snapshot and
`/api/alerts/config` rather than stripping them. Guests in the VMs &
Containers section order by display name (numeric-aware, case-insensitive)
with vmid as tiebreaker, because the rows render the name and an invisible
sort key reads as an unsorted list. Regression ownership is
`frontend-modern/src/utils/__tests__/metricThresholds.test.ts` and
`frontend-modern/src/features/alerts/thresholds/hooks/__tests__/useThresholdsRecoveryDefaultsState.test.tsx`.
Guest metric canonical state remains resource-backed and therefore node-scoped
for Proxmox guests, so node moves must not strand active alert state on the
previous resource ID. When a guest metric alert survives a node move, alerts
runtime must migrate the active alert, history entry, acknowledgment record,
suppression/rate-limit/flapping tracking, and guest per-disk metric identity
to the current canonical state instead of reopening a duplicate alert or
resolving only the stale node-scoped identity.
Metric alerts stored under a canonical state key must also clear through that
same identity. Hysteresis resolution must derive the active-alert storage key
from the existing alert and canonical state ID; removing only the legacy
`<resourceID>-<metric>` ID can emit a resolved notification while leaving the
canonical alert active to resolve again on every poll.

`internal/alerts/reducer` is the deterministic lifecycle core. It accepts an
explicit observation time and models enablement, hysteresis, sustained delay,
intent gating, acknowledgement, and warning/critical severity without
persistence or notification side effects. The canonical lifecycle family and
the shared metric-threshold runtime now use reducer incidents as transition
truth; the alert manager projects those incidents into active alerts, history,
persistence, callbacks, and notification policy. Metric incidents are keyed by
the canonical metric spec ID, while the metric name remains the severity
classification input. Families not yet cut over continue to use the parity and
shadow harnesses; a divergence is a reducer bug unless a focused manager
regression first proves otherwise.

The confirmation/discrete-state slice uses the same boundary for connectivity,
powered-state, and generic discrete matches. It activates after the resolved
number of consecutive matches, clears pending state or firing state on a
non-match at the evaluator layer, and re-derives spec-carried severity on every
firing observation. Both manager and reducer must date first activation at the
first matched observation, not the final confirming poll. Alerts runtime owns
that first-match timestamp alongside the confirmation counter and must clear it
whenever the run resets or its tracking state is removed. A direct reset of the
confirmation counter also begins a new run: the next matched observation must
replace any stale first-match timestamp so a later alert cannot be backdated to
an earlier outage.

Poll-driven offline recovery composes an additional confirmation gate over that
evaluator-layer lifecycle. A firing incident resolves only after consecutive
healthy observations (three for nodes, PBS, and PMG; two for storage), and an
offline observation resets the healthy run. Pending incidents still clear on
the first healthy observation, while disabling the rule bypasses recovery
confirmation and resolves immediately. The reducer's recovery state and the
manager composition must remain step-for-step equivalent under the parity
harness.

A discrete incident that reactivates within the five-minute recently-resolved
retention is the same occurrence: it restores the original first-activation
time and emits the reducer's `EventRefired`, matching the manager path that
reactivates without adding a second history occurrence. The resolved record is
consumed by reactivation; after the retention window, activation starts a new
occurrence at the new matched observation. The reducer measures retention on
the explicit observation clock. The manager's temporary wall-clock comparison
remains reference behavior only until cutover and is covered by a parity
harness that aligns the two clocks. Additional alert families and downstream
delivery behavior require separate contract slices.

Acknowledgement is canonical incident state across reducer families. It may be
set only on a firing incident, remains attached through per-observation alert
rebuilds, and is removed explicitly by unacknowledge. Resolution makes the
acknowledgement record inactive rather than deleting it, so a reactivation
within one hour restores the original user and acknowledgement time; after that
inactive retention, reactivation is unacknowledged. The reducer enforces this
boundary on its observation clock, matching the manager's one-hour cleanup TTL,
and parity must cover acknowledgement, unacknowledgement, rebuild, short
resolve/re-fire restoration, and expiry for both metric and discrete families.

The intent-policy gate composes at activation time with both discrete
confirmation and shared metric thresholds. An explicit policy grace period or
resolved operator suppression keeps a matched condition pending while
preserving its first active observation and, for discrete state, clamping its
confirmation count at the configured requirement. For metrics, the explicit
`metric.<name>` policy replaces the legacy time-threshold delay and accrues
grace only from monotonic runtime ticks; a below-trigger observation clears the
pending run and resets grace. Grace and operator suppression accrue
concurrently; once both release, activation uses the preserved first
observation as the incident start. Maintenance, muted or retired monitoring,
and expected-offline state may hold a new activation when the resolved policy
honors operator state, but must not clear or suppress an incident that is
already firing. The reducer applies caller-resolved intent on its observation
clock, and parity must exercise the manager's real intent-policy and
maintenance-window composition. Backup-offline deferral remains outside this
characterized slice.
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
immutable node identity whenever the alert carries instance context. Current
and prior native Proxmox node names may resolve that same entry so an override
or native rename updates active alerts, resolved alerts, and history
presentation without changing the stored diagnostic native resource name or
alert identity. Read-model presentation must snapshot resolved state before
acquiring the active-alert lock to preserve the established lock hierarchy.
Alert updates, incident rebuilds, and guest-metric migrations may fall back to
the legacy name-only cache only for instance-less resources like standalone
host agents. Display names never become acknowledgement, suppression,
rate-limit, flapping, escalation, or notification routing identity.
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
ZFS pool and device alerts follow the storage's pool attachment lifecycle. A
storage checked without an attached ZFS pool must shed any previously raised
zfs-pool-state, zfs-pool-errors, and zfs-device alerts on that check rather
than waiting for the stale-alert cleanup, because the health path can only
clear its own alerts while the attachment exists (#1731).
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
That reevaluation boundary distinguishes metric-backed alert types from
provider-owned incidents. A nil metric threshold may resolve only a known
threshold-backed metric; it is not recovery evidence for resource incidents,
and must not erase their acknowledgement state during a schedule-only save.

Browser metric severity colors are also alert-backed. Workloads,
Infrastructure, Storage, and the platform-page tables (Docker hosts and
containers, Proxmox nodes, Kubernetes clusters and nodes, TrueNAS systems and
apps, vSphere hosts) may pass resolved display thresholds into their local
bars, but threshold selection must flow through the shared alert activation
store and `frontend-modern/src/utils/metricThresholds.ts`, including configured
hysteresis, disabled thresholds, storage usage defaults, and guest/Docker
override identity candidates. The display resolver mirrors the runtime's
per-platform default scopes — guest, node, pbs, agent, docker, storage, plus
kubernetes, truenas, and vmware backed by `kubernetesDefaults`,
`truenasDefaults`, and `vmwareDefaults` — and unified platform resources
resolve overrides through the shared
`unifiedPlatformOverrideIdCandidates` chain in
`frontend-modern/src/features/alerts/alertOverridesModel.ts` (the same
candidate set `buildProjectedOverrides` indexes platform alert rows under).
Static metric-color defaults are only fallback presentation behavior for
callers that do not have alert configuration in scope.

Docker container image-update alerts are lifecycle-governed by the alerts
runtime. Disabling Docker update alerts globally, disabling alerts for a
specific Docker container, ignoring a Docker container prefix, or disabling all
Docker container alerts must clear the active image-update alert plus resource
and identity first-seen tracking. Generic threshold reevaluation must not keep
or resurrect image-update alerts after their owning Docker alert configuration
has disabled them.

Docker container alert overrides key on stable container identity. The
canonical override key is `docker:{hostID}/{containerName}`; container IDs
change on every recreate, so runtime-ID keys orphaned each image update and
accumulated dead entries in alerts.json (#1601). The evaluator resolves the
name key first and falls back to the legacy `docker:{hostID}/{containerID}`
key for pre-migration entries, `MigrateDockerContainerOverrideKeys`
(`internal/alerts/docker_override_migration.go`) re-homes live legacy and
unified-hash keys onto the name key on the monitor sync cadence and prunes
orphaned ID-shaped entries while keeping name-keyed overrides for absent
containers, and the UI writes only the name key while binding rows through the
shared `dockerContainerOverrideIdCandidates` chain (name first; container-ID,
short-ID, unified-hash, and slash-tail forms as trailing lookup candidates).
Container override work must not reintroduce a runtime-container-ID
persistence key.

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
An incident with no backing availability check still carries first-class
evidence: the bridge derives a complete/confirmed envelope from the directly
observed incident payload (provider and collector from the incident, digest
payload ref) instead of leaving the alert evidence-less for the legacy partial
shim to backfill. Because each sync re-observes the incident, the bridge merges
the cycle's freshly observed envelopes into the already-active alert, so
raise-time evidence (including availability envelopes with validity windows)
does not age out while the condition is still directly observed.
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
The destination editor also exposes resource-tag routing for email and each
alert webhook. Alert producers must carry canonical resource tags into alert
metadata: Proxmox VM/container tags remain literal, Docker container/service
labels become `key:value` tags (or `key` for empty values), host-agent tags
remain literal, and unified resources forward their canonical `Tags`. The
alerts surface owns this producer metadata and routing UX; destination
matching, persistence, and receipt-aware delivery remain notifications-owned.
The same editor exposes a minimum-severity policy for email, Apprise, each
webhook, and Relay mobile push. The alerts surface owns the coherent routing UX;
notifications owns provider filtering and recovery receipts, while Relay owns
privacy-safe mobile projection and persisted mobile routing policy.
Email and Apprise expose all three destination choices: all alerts, warnings
and critical alerts, or critical alerts only. Relay deliberately exposes its
smaller persisted policy of all alerts or critical alerts only; hiding the
warning option there is protocol honesty, not permission to collapse a stored
warning floor on notification-owned destinations.
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
Persistent managers establish the SQLite alert store during construction,
before escalation, periodic persistence, shadow evaluation, or monitoring can
act on restored state. `alert_active_state` is the restart authority and is
independent of event retention, so a still-active occurrence never disappears
because it predates the 90-day history window. Fired, refired, acknowledged,
unacknowledged, escalated, and resolved lifecycle transitions update that
projection in the same transaction as their immutable lifecycle event. A
resolution matches alert identity plus occurrence start time so a delayed old
resolution cannot delete a newer occurrence that reused the canonical ID.
Full checkpoints capture mutable fields such as `LastSeen`, `LastNotified`, and
escalation state with revision compare-and-swap; a checkpoint that raced a
lifecycle transition retries instead of overwriting it. SQLite uses full
synchronous durability for this authority, and the recovery mirror fsyncs its
temporary file plus a platform-native durable rename barrier (parent-directory
sync on Unix and write-through replacement on Windows), so the contract covers
host power loss rather than only orderly process restart.
`active-alerts.json` remains an atomic recovery mirror, not a competing healthy
read authority. A new or recreated database imports the readable mirror. A
failed SQLite checkpoint writes a durable degraded marker, and the next startup
uses the mirror once to repair SQLite before clearing that marker. Without a
degraded marker, an initialized SQLite projection wins over a stale mirror.
Lifecycle append failure first checkpoints a lock-independent active-state
projection to the recovery mirror synchronously, then writes the degraded
marker. The projection is updated at the canonical set/remove and lifecycle
event seams so event emission remains safe even when its caller holds the
manager lock. A checkpoint that overlaps a lifecycle failure must observe the
failure epoch and retry before it can restore SQLite authority or clear the
marker; stale periodic work can never overwrite the failure recovery snapshot.
An unreadable or malformed mirror is write-blocked and preserved for manual
recovery even when healthy SQLite can continue serving current state; startup
must never silently replace a recoverable source with an empty snapshot.
Acknowledgement and incident age are never implicit resolution evidence:
restart restores long-running and long-acknowledged occurrences until fresh
healthy evidence or an explicit operator action resolves them.
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
Omitting the alert identifier returns one diagnosis for every active alert from
one manager read pass, ordered by alert identifier, and an empty active set is
encoded as `[]` rather than `null`. The Alerts Overview consumes that bulk
projection once per refresh and may render its current delivery outcome on an
unacknowledged active-alert card. Missing or failed diagnosis reads degrade to
no delivery line; they must not hide alert truth or trigger per-card requests.
`internal/alerts/eventlog/` and `internal/alerts/event_emission.go` own the
canonical append-only alert event record. Persistent managers enable a
SQLite-backed store under the alerts data directory; ephemeral managers record
nothing unless a store is installed explicitly. Resolution, acknowledgement,
unacknowledgement, escalation, flapping detection, dispatch, quiet-hours
deferral, and suppression append immutable events without changing lifecycle
or delivery behavior. Suppression and deferral are recorded as outcome
episodes: the first decision is immutable, identical reevaluations of that
unchanged outcome append no duplicate row, and a changed reason, details,
resource presentation, intervening lifecycle/dispatch event, or later return
to that outcome opens a new episode. Lifecycle, escalation, and dispatch
events are never coalesced. This keeps delivery activity explanatory instead
of poll-frequency-shaped, prevents unchanged reevaluations from crowding a
real outcome change out of the non-blocking diagnostic buffer, and bounds
diagnostic growth without erasing a meaningful decision transition. A failed
write forgets its admission key so a recovered store can accept the outcome
again. Reads apply the same episode projection to
redundant diagnostic rows written by older versions, so upgraded installations
become readable immediately without rewriting their immutable event records.
Snapshot-bearing lifecycle transitions commit
synchronously before downstream lifecycle projections run; they never share
the droppable diagnostic buffer because alert history is reconstructed from
them. High-volume notification decisions remain non-blocking and fail-open for
alert evaluation: a full diagnostic buffer counts and drops that delivery
evidence. Failed asynchronous batches do not advance the successful-write
counter, and `Flush` must report the failure or timeout rather than falsely
claiming the batch landed. An unavailable or failed lifecycle store degrades
history reads to the recovery model without hiding live active-alert truth.
Events retain for 90 days and prune hourly. Fired and
refired lifecycle events come only from the reducer core's explicit activation
events: canonical lifecycle reactivation maps `EventRefired` separately, while
shared metric activation records one fired event when its pending incident
becomes firing. Repeated firing observations and persisted-alert restore must
not append another activation event, and the active-alert storage funnel must
never invent one.
Event-log reads accept a caller limit but normalize it to the store's bounded
maximum before it reaches SQLite. Result-slice allocation is independent of
that caller value: a request-provided limit is a row-count preference, never a
memory-allocation hint. This remains defense in depth even when an authenticated
API handler validates the query parameter, because non-HTTP manager callers use
the same store boundary.
Alert-history reconstruction is the explicit exception to a single bounded
query: it walks every matching lifecycle event oldest-first through bounded
keyset pages, releasing the SQLite rows between pages. The public/API query cap
therefore remains intact while a noisy installation with more than 1,000
lifecycle events in the requested window cannot silently lose older history.
Lifecycle events now carry a full alert snapshot and
`internal/alerts/history_projection.go` can fold those snapshots into one
history row per occurrence. Existing pre-snapshot SQLite stores upgrade in
place, and rows without snapshots remain readable but cannot contribute to the
projection. The event log becomes the alert-history read authority only after
legacy history is absent or its complete import and retirement succeeds.
Managers without an authoritative event store, and stores that report a
lifecycle write failure, retain the in-memory JSON-history model as their
fail-soft read fallback.
`internal/alerts/history_projection_parity_test.go` characterizes the event-log
projection against that fallback for active, resolved, acknowledged,
multi-resource, and repeated occurrences. Live active-alert state overlays the
latest projected snapshot so acknowledgement and current values do not become
stale.
`internal/alerts/history_migration.go` imports every legacy JSON entry through
the event log's synchronous, non-droppable import path before renaming the
fixed history and backup files to their `.imported` backups. The primary or
backup source's presence keeps migration active until both leaves retire.
Imported events are retry-idempotent by immutable alert ID, occurrence time,
and snapshot, so a committed SQLite transaction followed by a failed or
partial file rename cannot duplicate history on the next startup. An import
failure leaves JSON history authoritative for the next attempt. An unreadable
or malformed source, or any entry that cannot produce an identified complete
snapshot, blocks both retirement and further JSON writes so startup cannot
silently replace recoverable user history with an empty file. Successful
retirement stops further JSON writes. Retired `.imported` leaves remain bounded
recovery sources: startup loads and idempotently replays them if `events.db` is
recreated, so loss or quarantine of the database cannot turn a successful
migration into silent empty history. Clearing history appends a
`history_cleared` tombstone rather than deleting log rows; the projection
ignores earlier lifecycle events but overlays still-active alerts as current
state. Unified-incident and system-alert activation must emit snapshot-bearing
fired events so their projected history does not begin at acknowledgement or
resolution.
`SubscribeLifecycleCallback` is the delivery-independent projection seam for
that canonical stream. It runs for lifecycle transitions regardless of
activation, quiet hours, grouping, rate limits, or destination state; delivery
callbacks remain policy-controlled consumers. Monitoring uses this seam to
materialize canonical resource-history breadcrumbs and incident shells. On
startup it replays durable lifecycle transitions and migrated legacy-history
snapshots oldest first, then idempotently reconciles restored active alerts
that predate the event store. Because monitor construction precedes attachment
of the API-owned durable unified-resource store, that attachment repeats the
same idempotent replay so canonical resource history is repaired as well as the
incident fallback cache. A migrated final snapshot expands only facts it
actually carries: the occurrence start, saved acknowledgement, and known end;
it does not invent delivery, escalation, analysis, or remediation events.
The incident store retains those snapshot events as a fallback only when the
canonical resource timeline lacks the equivalent event, so a partial durable
projection cannot erase the occurrence start while complete canonical history
still wins. The occurrence-qualified incident API performs the same repair
from the active/history read model when retention or an older migration left a
shell absent or empty. This repairs resolved as well as active incident
timelines without replaying any delivery side effect, and ensures an alert
already visible in Overview or History cannot keep returning an unavailable
timeline merely because its notification was held or it predates the event
store. Occurrence lookup allows only one second of start-time precision drift;
it must not merge recurring or flapping incidents merely because they share an
alert identifier and started within the same former ten-minute window.
Snapshot events stored at equal timestamp precision order by lifecycle meaning,
with detection first and resolution last, rather than by generated event ID;
a malformed legacy end before its own start clamps to the start boundary.
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

### Backup-age attribution follows live identity

Backup rollup subject refs are historical hints, not unconditional proof of a
guest's current placement. An exact `instance:node:vmid` ref is authoritative
only while it resolves to a live guest of the same type. Otherwise backup-age
evaluation retries typed VMID attribution and prefers live candidates carrying
a resource ID over last-known display metadata. A PVE-owned ref can still name
its authoritative orphan after the PVE inventory is ready; a stale PBS ref that
cannot be resolved uniquely remains unattributed instead of pinning an alert to
an old or wrong PVE node. `TestCheckBackupsRemapsStaleSubjectRefToUniqueLiveGuest`
and the PVE orphan/inventory-readiness tests in `internal/alerts/alerts_test.go`
pin both sides of this boundary.
Per-guest backup and snapshot overrides are sparse, not frozen copies. A
zero-valued threshold field in an override (`warningDays`, `criticalDays`,
`freshHours`, `staleHours`, and the snapshot size pair) inherits the current
global default at evaluation time through the merge helpers in
`internal/alerts/backup_snapshot.go`; explicit non-zero values win, and
`alertOrphaned` plus `ignoreVMIDs` always resolve from the globals because they
are instance-wide filters. `NormalizeRecoveryOverrides` in
`internal/alerts/config/normalize.go` (called from `UpdateConfig` in
`internal/alerts/config_runtime.go` and from the shared persistence
normalization in `internal/config/persistence.go`) rewrites stored overrides
whose threshold tuple exactly matches the current globals into sparse
enabled-only form. Those full copies were artifacts of the legacy per-guest
toggle, and the equality condition makes the rewrite behavior-preserving at
the moment it runs while letting the guest track later global edits (#1126).
Overrides whose thresholds differ from the current globals are deliberate
per-guest values and must never be rewritten.
`TestGuestBackupOverrideInheritsGlobalThresholds`,
`TestFrozenBackupOverrideMigratesToSparse`,
`TestDeliberateBackupOverridePreserved`, and
`TestMergeSnapshotOverrideInheritsZeroFields` in
`internal/alerts/alerts_test.go` pin the merge, migration, and preservation
behaviors.
Proxmox disk health alert evaluation now lives in
`internal/alerts/disk_health.go`. That file owns Proxmox disk canonical
identity, disk health assessment alerts, known-firmware health suppression, and
SSD wearout alerts; future Proxmox disk-health behavior should extend that
checker owner rather than expanding the central Manager file.
Shared metric threshold runtime now lives in
`internal/alerts/metric_runtime.go`. That file owns metric threshold lookup,
per-metric delay and intent resolution, reducer input composition, active-alert
projection, delivery side effects, metric runtime options, alert key sanitation,
and metric delta helpers. The reducer core owns pending, firing, severity, and
hysteresis transitions; future metric-threshold behavior should extend these
owners rather than adding shared metric logic back to the central Manager file.
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
inline overflow styles that break CSP on the public shell. The selected tab
must expose canonical current-page state and compose the shared active
horizontal-rail visibility owner, so direct navigation and viewport changes
bring Thresholds, Notifications, or Schedule fully into view instead of
leaving the active destination clipped beyond the mobile rail.
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
Resource-tag filtering must not re-evaluate mutable tags for a resolved event.
The notification owner uses the firing delivery receipt to select recovery
destinations, so a tag change between firing and recovery cannot suppress a
clear from a destination that received the original alert.
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
The backup/snapshot override toggles in that mutations owner write sparse
enabled-only overrides (zero-valued threshold fields) rather than copying the
current global defaults into the override, because zero-valued fields inherit
the globals at evaluation time and a copied value freezes the guest against
later global edits (#1126). Toggling a guest that already carries explicit
override thresholds must preserve those values while flipping enabled.
The Backups and Snapshots global-defaults editors in
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx`
and
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx`
reconcile warning/critical pairs through `reconcileWarningCriticalEdit` in
`frontend-modern/src/features/alerts/thresholds/helpers.ts`: a single-field
edit that creates a conflict adjusts the field the user did not touch, so a
32-day warning edit raises critical alongside it instead of being silently
clamped back down by the sanitizers before save.
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
`frontend-modern/src/components/Alerts/ThresholdsTable.tsx` must expose those
peer platform routes through the shared `Subtabs` primitive rather than an
inline FilterBar group. Clearing search or override filters must preserve the
active platform route.
The thresholds shell is overview-first: every resource section starts
collapsed, persisted operator expansion choices still win, and Expand all /
Collapse all remain available on every platform tab. Because collapsed tables
cannot carry the discovery burden, the shell must surface a filter-independent
custom-override summary above the sections; choosing a summary item selects
the Custom-only filter, opens its owning section, and moves that section into
view. Resources not represented there continue to inherit their group defaults.
Metric enablement is an explicit On/Off interaction in row, global-default,
mobile, and bulk editors. Enabled numeric inputs accept positive trigger values
only and user-facing copy must not expose the persisted disable sentinel.
Internally, the canonical `<= 0` read rule remains intact for legacy data, while
new Off actions write `-1`, so existing configuration and the API contract
continue to round-trip without making `0` a second customer-facing disable path.
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
`frontend-modern/src/components/shared/WebInterfaceLink.tsx`; alerts own only
the alert/resource data and URL availability decision, not URL safety
classification, the adjacent launch-control shell, new-tab safety attributes,
row-click containment, invalid-URL warnings, or accessible launch-label
semantics. Alert resource names remain inert row identity so opening a saved
web interface cannot also toggle selection or the thresholds editor.
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
The shared hook may expose Timeline for a displayed mock alert because mock
mode now guarantees an occurrence-qualified incident through the same typed
API. A mock fixture missing that incident is a broken alert read-model contract,
not an empty-state case for the frontend to disguise; genuine unknown real
occurrences may still render the existing no-timeline state.

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
`AlertOverviewAlertCard.tsx` also owns the compact delivery-status placement,
while `deliveryDiagnosisPresentation.ts` owns the reason-to-copy and tone
mapping. Acknowledged cards keep their canonical badge and omit that redundant
line; held delivery states that can surprise an operator use the attention
tone, while cooldown, quiet-hours, monitor-only, and successful/pending states
remain neutral.
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
Alert History toolbar state is one route-owned unit. Search, period, and
severity are read and written by `useAlertHistoryState.ts`; its active-filter
count includes search, and `AlertHistoryFiltersCard.tsx` delegates contextual
Clear filters to one composite hook mutation instead of letting `FilterBar`
issue sequential route writes. That reset removes all three query parameters
in one navigation and clears any transient chart-bucket selection, so a
search-only result set remains visibly resettable and an older URL write
cannot resurrect another filter.
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
destination test actions, retry orchestration, and notification delivery-health
loading around that webhook runtime,
while
`frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx` stays the
destinations render shell and composes
`frontend-modern/src/features/alerts/AlertDeliveryHealthCard.tsx`,
`frontend-modern/src/features/alerts/AlertEmailDestinationsSection.tsx`,
`frontend-modern/src/features/alerts/AlertAppriseDestinationsSection.tsx`,
`frontend-modern/src/features/alerts/AlertWebhookDestinationsSection.tsx`, and
the dedicated load/error wrappers. The delivery card must warn for retained
terminal failures, present an unavailable queue-health read as unverified
rather than healthy, and give the operator a refresh action plus configuration
and test guidance. Its warning copy must explain that expired delivery records
are removed hourly and that, absent another terminal failure, the warning
clears only after the last retained record reaches its configured retention
limit. Recoverable retry attempts must not produce this warning.
`frontend-modern/src/features/alerts/AlertDeliveryHealthCard.test.tsx` and
`frontend-modern/src/features/alerts/__tests__/useAlertDestinationsTabState.test.tsx`
are the focused render and runtime proofs for that operator surface.
Future config cleanup should extend the
config transport hook, the config model, the override-projection hook, or the
shared `frontend-modern/src/utils/alertDestinationsPresentation.ts` helper for
customer-facing destinations copy instead of reviving inline retry, test, and
error text across the feature tabs. Runtime cleanup should extend the
destinations hook owned by the subsystem that carries the behavior instead of
letting the broader configuration hook absorb all four concerns again.
Local Apprise delivery is a shipped server-runtime capability, not an
operator-installed optional. The shared presentation helper must show the
Telegram `chat_id:topic` form, the notification backend must preserve each
target as one CLI argument, and the deployment-installability contract must
keep the pinned `apprise` executable available in every Pulse server image.
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
backfill canonical identity on the clone, replace the persisted public ID with
the canonical state ID, and serialize that snapshot instead of mutating the
live alert map during async saves or incident rebuilds. Restore must preserve
acknowledgement fields and rewrite a successfully derived legacy snapshot
atomically after the in-memory restore; unrecognized records remain readable.
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

Per-alert snooze is the customer-facing bounded suppression contract, not a
page-local timer. `SnoozeAlert` writes the same canonical operational record,
requires a future expiry no more than 30 days away, and appends a durable
`snoozed` lifecycle snapshot. While active, notification dispatch, recovery
delivery, delivery diagnosis, and escalation all consult that one record.
Expiry runs as an explicit `unsnoozed` transition and restores `acknowledged`
or `open` according to the underlying alert without resolving the incident.
Skipped escalation levels are not replayed in a burst: the remaining schedule
resumes from the unsnooze transition while the incident's original start time
and duration remain intact. The alerts overview exposes preset bounded holds,
the exact local expiry, and an explicit Resume action over this API-owned state.

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

### Versioned persisted alert identity

`AlertConfig.identitySchemaVersion` is the additive persisted identity marker;
the current schema is version 1. `PlanAlertIdentityMigration` is the mandatory
dry-run boundary: it operates on a copied config and reports source/target
versions, removed and added keys, deferred rows, and unsupported future
versions before any write. `ApplyAlertIdentityMigration` may install that exact
plan only while its source version is still current. The monitoring bridge must
persist the result before updating the live manager.

Version 1 folds provider-declared canonical succession, Docker container
recreation IDs, Proxmox guest and guest-disk aliases, and live storage aliases
onto the single write identity already used by the editor/evaluator. A current
row wins over retired duplicates. Multiple legacy rows claiming one target,
ambiguous live ownership, unknown rows, and temporarily absent resources fail
closed and stay in `alerts.json` for a later resource snapshot that can prove
the mapping. Alias readers remain compatibility inputs for those deferred rows,
not persistence authority. Storage alias provenance is carried on the unified
resource `StorageMeta` so the plan does not reconstruct ownership from display
names alone.

The marker is rollback-safe: older binaries ignore the additive JSON field and
may omit it on a later save; after re-upgrade the idempotent planner runs again.
Newer binaries preserve an already-held marker when a stale browser payload
omits it, while the alert editor explicitly round-trips the marker. A config
whose marker is newer than the running binary is never rewritten. Regression
ownership is `internal/alerts/alert_identity_migration_test.go`,
`internal/monitoring/monitor_alert_override_migration_test.go`, and
`frontend-modern/src/features/alerts/__tests__/alertsConfigurationModel.snapshot.test.ts`.

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

Resource-type inheritance is field-wise from general to specific before the
canonical-resource rule: for example `guest` supplies a VM/LXC default and
`vm` or `system-container` may replace individual fields. A guest policy must
not leak into node or agent connectivity. Omitted grace preserves inherited or
factory behavior, explicit zero is a deliberate no-wait policy, and a positive
powered-off grace is duration authority rather than a poll-count threshold.
The no-policy powered-state path retains the legacy two-confirmation contract
for migration compatibility; an applicable powered-state duration activates
on the first stopped observation at or after elapsed eligibility.

Runtime eligibility uses server receipt time plus process-monotonic elapsed
progress, never client/provider timestamps or poll counts. Duplicate reports
advance no time, delayed polls count actual elapsed time, and wall-clock
forward/backward changes do not change the decision. Pending state persists
accumulated elapsed progress. Restart re-baselines the monotonic clock without
counting unobserved process downtime, while legacy timestamp-only pending files
are imported once into the elapsed representation. Recovery, explicit guest
suppression, disabling offline alerts, and tracking cleanup remove both
durable pending state and its transient monotonic baseline before a later
outage can start.

Operator maintenance and intentionally-offline state are read only through the
canonical unified-resource identity. One-shot and recurring maintenance use
the same effective occurrence boundary. Exact-resource maintenance always
applies; ancestor maintenance applies only when its persisted scope is
`resource_and_descendants`. Overlapping exact and inherited windows remain
suppressed until the latest active end, and every operator-state mutation
reconciles the active-alert set immediately so a host window cannot leave child
alerts visible until another detector write. Backup-aware offline deferral consumes
fresh, matching, active task evidence, applies the configured post-backup grace,
and always terminates at its hard cap. Missing, stale, future-skewed,
finished, or mismatched backup evidence cannot suppress an outage. This policy
changes alert activation only: notification delivery, recovery assurance, and
customer-infrastructure mutation retain their existing owners.

`internal/alerts/intent_policy_test.go` and
`internal/alerts/powered_off_tolerance_test.go` prove field-wise type
precedence, normalization, exact zero/positive duration boundaries, VM/LXC
coverage, duplicate and delayed reports, wall-clock changes, suppression
reset, migration/restart continuity, backup hard-cap behavior, preview
immutability, and first-match lifecycle identity.

The thresholds resource rows expose that same policy owner directly through a
per-resource alert-delay action. The action carries the canonical resource id
and the first supported CPU, memory, or disk signal into the versioned
intent-policy editor, expands it, and leaves persistence and inheritance with
`/api/alerts/intent-policies`. Resources without a supported metric open on
`state.offline`. Threshold rows must not create a parallel delay field in the
legacy threshold override document or save intent changes through the
threshold-config endpoint.

### Provider-observed storage and workload incident lifecycle

Provider observations use one stable alert identity composed from provider,
connection-local resource identity, native signal identity, and canonical
incident code. Synthetic TrueNAS pool, vdev, dataset, disk, app, and container
conditions require two consecutive observations before activation and two
consecutive healthy observations before recovery. A transient poll therefore
does not fire or clear an alert. Active state, acknowledgement, suppression,
severity changes, recovery, and history remain owned by the shared alerts
runtime and survive restart; an absent resource or unknown collection result is
not recovery evidence.

Native TrueNAS alerts remain authoritative for their equivalent condition.
Their stable native ID suppresses only the duplicate synthetic pool/vdev state,
missing-member, or scan condition; distinct read/write/checksum evidence
continues to participate. Pulse does not send a second TrueNAS email stream.
Intentional stopped apps and other stable conditions remain suppressible
through the canonical per-resource or global alert configuration, and readonly
replication targets do not produce incidents when native replication evidence
classifies them as healthy.

Ceph contributes one cluster-scoped provider incident only for native
`HEALTH_WARN` or `HEALTH_ERR` evidence. It shares the same confirmation,
recovery, history, and suppression machinery but cannot manufacture a disk
incident from cluster health.

`internal/alerts/unified_incident_confirmation_test.go`,
`internal/truenas/provider_pool_health_contract_test.go`, and
`internal/unifiedresources/ceph_pool_health_contract_test.go` prove the
lifecycle and deduplication matrix.

### Per-resource monitoring policy is an alert-runtime invariant

The canonical unified-resource operator state is honored without requiring an
explicit versioned alert-intent rule. Factory behavior enables operator-state
evaluation, and a signal rule cannot weaken a persisted resource monitoring or
lifecycle policy. `expected_offline` suppresses only `state.offline` and
`incident.availability`; `muted`, `retired`, and active maintenance suppress all
signals. Unknown alert families fail safe to the default signal and cannot be
hidden by expected-offline.

Every active-alert writer passes through the same operator-state gate, and
notification delivery consults the same decision before quiet-hours policy.
Operator suppression is never converted into a quiet-hours replay, including
for recovery notifications carrying stale replay metadata.
After a persisted policy mutation, `ReconcileResourceOperatorState` resolves
already-active records for that resource immediately; later detector writes
cannot recreate them while suppression remains active. Existing prefix, tag,
and `pulse-no-alerts` bulk rules remain compatible inputs and do not create a
second per-resource state store. `internal/alerts/intent_policy_test.go` pins
factory operator-state evaluation, writer gating, and active reconciliation.

### Object drawers expose the active problem across identity aliases

An object drawer with active alerts must show the exact problem in its default
Overview, not only an alert accent or aggregate status already present in the
table row. Frontend matching accepts a bounded ordered set containing the
canonical resource id and provider-authored, metrics-target, alias, and
superseded identities. The same candidate set drives row alert decoration and
drawer message selection so opening a highlighted object cannot lose the alert
because the provider stream and unified-resource projection use different
keys. This is a read-only projection of the existing active-alert map; it does
not create alert truth, change acknowledgement, or add a detector or delivery
path.

### Large alert collections stay continuously scrollable

Resource alert tables, active-alert lists, incident timelines, and resolved
history must preserve full filter, group, count, selection, and action semantics
without mounting the full estate. Desktop tables use the shared
`PlatformWindowedRows` owner; phone cards use `PlatformWindowedList` with
feature budgets no larger than 32 items. Native scroll extent comes from spacer
geometry, and wheel/touch projection must maintain a mounted directional runway
so rapid scrolling never exposes a loading-looking blank section. Alert
windowing is presentation-only and must not change alert truth, grouping,
acknowledgement, resolution, or delivery state.

### Destinations tab carries delivery evidence

The alert destinations tab is where the belief "delivery works" is formed, so
it must carry delivery evidence, not only configuration. It renders the
notifications-owned delivery log (`AlertDeliveryLogCard` fed by
`useNotificationDeliveryLog` over `GET /api/notifications/delivery-log`) as a
newest-first record of real alert delivery attempts with plain-language
outcome labels, destination names resolved from the loaded webhook configs,
failure classes, and secret-redacted error text. The card names its retention
window and says that test sends skip the queue, and an unreadable log renders
as unavailable rather than empty. The same card interleaves alert-owned
`notification_suppressed` and `notification_deferred` evidence from
`GET /api/alerts/events` with those attempts, ordered by the recorded event or
attempt time. Held rows name the affected resource, alert type, and
plain-language policy reason even when no delivery attempt exists. The event
read is independent and fail-soft: an unavailable event log removes held rows
without delaying, hiding, or marking the notifications-owned attempt log as
unavailable. Refreshing delivery evidence starts both reads from one operator
action while preserving that failure isolation. Test-send results that report
`deliveryPaused` surface as a warning toast, never plain success, across the
email, Apprise, and webhook test actions in `useAlertDestinationsTabState`
and `useAlertWebhookDestinationsState`. Delivery evidence remains
notification truth: the delivery log card must not resolve, suppress, or
re-evaluate alerts, and it does not alter the `AlertConfig.enabled` versus
`activationState` ownership boundary above.

### External watchdog is Pulse-availability evidence, not notification delivery

The Alerts destinations surface owns the operator contract for an external
dead-man watchdog that detects loss of Pulse itself. It presents masked
configuration, live heartbeat state, canonical-monitor progress, consecutive
delivery failures, and the last restart interruption. The credential-bearing
ping URL must never return to the browser after save; the configured state uses
an explicit replacement placeholder and removal action. The watchdog remains
active independently of alert activation, snooze, quiet hours, and notification
delivery pause because those policies must not disable observation of Pulse.

Watchdog transport and monitoring progress remain notifications- and
monitoring-owned respectively. Alerts owns the system-alert projection:
delivery failure, canonical-loop stall, restart interruption, and durable-state
failure use stable system-alert identities, normal lifecycle/event history, and
idempotent fingerprints. A restart interruption is raised into history even
when the first successful external report immediately resolves it, so recovery
does not erase the outage record. The watchdog must never invent resource
identity or become a second lifecycle for infrastructure alerts.

### Escalation targets are exact and critical repeats are bounded

An escalation level may address credential-free logical destination IDs:
`email`, `apprise`, or `webhook:<id>`. Once exact IDs are present they are the
authority for that level; a deleted or unknown ID fails closed and must never
widen to a channel or every webhook. The legacy `notify` channel remains a
mixed-version fallback for configurations that do not yet carry exact IDs.

Optional critical-repeat policy applies only after the final escalation level.
It sends at most one repeat per checker pass, uses a normalized 5–180 minute
interval, and stops while the incident is acknowledged, resolved, snoozed, or
global detection or delivery controls are paused. A repeat is durable escalation
evidence with an explicit repeat marker rather than a new incident or a replay
of missed intervals.

Every escalation level delay uses the same 5–180 minute safety boundary at the
frontend state owner and the backend configuration-normalization boundary.
Malformed API payloads and manually entered values therefore cannot turn a
level into an immediate escalation or an unreachable timer beyond the supported
schedule range.

The Schedule surface loads the same email, Apprise, and webhook catalog as the
Destinations surface. It shows disabled and deleted selections explicitly,
prevents an escalation level from becoming destinationless, and explains the
critical-repeat stop conditions before save.

### Recovery authority and operator qualification fail closed

If a lifecycle append or active-state checkpoint cannot reach SQLite, the
degradation marker is the crash-safe restart authority. The marker is an
atomically replaced, file-synced, directory-synced envelope containing the
complete active-alert snapshot; the JSON mirror remains a compatible recovery
aid but is not required for a new marker. While the marker exists, later saves
refresh that envelope and must not make SQLite authoritative again in the same
process. Bootstrap repairs SQLite from the envelope and only returns authority
after durably removing the marker. Legacy text markers may use a readable JSON
mirror, while malformed or source-less markers defer authority rather than
silently trusting a potentially stale projection.

`tests/integration/scripts/run-alert-qualification.mjs` is the alerts-owned
operator qualification entrypoint. One run must cover crash-safe active-state
recovery, snooze and escalation policy, destination severity and exact-target
routing, a signed real HTTP webhook, durable delivery receipts and recovery,
dead-man progress and restart-gap reporting, plus the production frontend
against a managed local backend. The browser phase must exercise active-alert
snooze, delivery diagnosis, restart persistence, unsnooze, incident evidence,
history migration, virtualization, and clear tombstones without replacing the
backend APIs with route mocks.

### Noisy gauges use one trust-stable lifecycle

Memory, temperature, and disk-temperature warnings use one continuous
stability window on both lifecycle edges: an unchanged factory configuration
requires five minutes above the trigger to open and five minutes at or below
the recovery threshold to close. A return to the hysteresis band or a renewed
breach resets the recovery run. Critical metric evidence may bypass only this
factory or legacy activation delay; an explicit alert-intent policy remains
authoritative. A metric-specific delay, including zero, remains the simple
operator override and becomes the matching recovery window for these gauges.

Recovery timing uses the reducer's monotonic runtime evidence when supplied,
so wall-clock changes cannot prematurely close an incident. Missing or unknown
observations do not constitute healthy evidence and therefore cannot advance a
recovery run. Restarts conservatively restart an in-progress recovery window
rather than manufacturing elapsed health.

For guest disk capacity, valid per-filesystem evidence owns the alert identity
and suppresses the less actionable guest-wide aggregate. This guarantees one
incident for one full filesystem while retaining the aggregate only when no
usable filesystem evidence exists. OS-managed transient mounts, including
macOS Gatekeeper App Translocation volumes, are removed at the shared
filesystem collection boundary before they can enter capacity inventory or
alert evaluation.

### Shared-system correlation never merges detector lifecycles

An active alert may carry the optional typed `correlation` object with a
`shared-system` kind, stable key, `primary` or `supporting` role, and an
auditable reason. This is presentation-only incident context. Every correlated
signal retains its own canonical spec, confirmation state, acknowledgement,
recovery evidence, history, and notification decision; recovery of one signal
must never clear another.

Proxmox connection availability is the primary signal for its `pve:<instance>`
system. PVE node connectivity is a supporting signal through provider-owned
instance membership. A linked host-agent connectivity alert is supporting only
when monitoring proves the reciprocal node ID and agent ID relationship and
the node supplies the same non-empty instance. Missing, one-sided, duplicate,
or conflicting identity fails open and leaves alerts separate. Resource-path
truncation, hostname similarity, and timing coincidence are forbidden as
correlation evidence.

The overview groups exact-resource detector signals and explicit shared-system
signals. It selects a declared primary when one is present, describes the rest
as linked signals, and acknowledges each underlying alert individually. Older
alerts and clients remain compatible because the field is additive and
optional. `internal/alerts/correlation_test.go`,
`internal/monitoring/alert_correlation_test.go`, and
`frontend-modern/src/features/alerts/__tests__/useAlertOverviewState.test.tsx`
pin lifecycle independence, fail-open identity, and client grouping.

### Infrastructure incident synthesis is deterministic and evidence preserving

The same optional alert `correlation` boundary also admits the
`infrastructure-incident` kind. It adds one of six typed failure classes
(`runtime`, `network-path`, `application-response`, `certificate`,
`dependency`, or `evidence-coverage`), an inference state
(`supported-cause` or `observation-set`), the primary alert and resource IDs,
sorted affected resource IDs, and a bounded observation list carrying alert,
resource, severity, observation time, and evidence IDs. It never copies
provider response bodies, credentials, arbitrary metadata, or error payloads.

`internal/alerts/incident_synthesis.go` derives this context from the complete
post-recovery active detector set plus active canonical relationships from
`ResourceRelationshipsWithCanonicalParent`. It may traverse at most eight
dependency edges and records at most 100 observations. Exact relationship
direction, confidence of at least 0.8, a current primary failure, and bounded
event ordering are required for `supported-cause`. Healthy or disagreeing
primary resource state, weaker topology, late timing, cycles, or incomplete
support must remain an `observation-set`. Name similarity, resource-ID prefix
truncation, message parsing, and coincident timestamps remain forbidden causal
evidence.

Every grouped alert still owns its canonical spec, acknowledgement, history,
timeline, evidence, and recovery. Synthesis is recomputed on each unified
incident reconciliation and clears stale synthesized links, so primary-first
recovery leaves the still-failing symptom visible as standalone immediately.
Existing `shared-system` correlations remain authoritative and are not
overwritten. Resource maintenance and monitoring intent continue through the
canonical resource-policy and intent gates before synthesis; no group-local
mute or maintenance state is allowed.

Only a supporting member of a `supported-cause` group suppresses duplicate
notification delivery, with the typed `correlated_primary` reason and primary
alert ID in the event log. The primary remains the one delivery owner.
`observation-set` members continue to notify independently because Pulse has
not established causality. `incident_synthesis_test.go` pins classification,
contradiction downgrade, bounded evidence, duplicate-delivery suppression, and
partial-recovery behavior.
