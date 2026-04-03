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

## Canonical Files

1. `internal/alerts/specs/types.go`
2. `internal/alerts/specs/evaluator.go`
3. `internal/alerts/canonical_metric.go`
4. `internal/alerts/canonical_lifecycle.go`
5. `internal/alerts/unified_incidents.go`
6. `frontend-modern/src/components/Alerts/RecentAlertsPanel.tsx`
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
23. `frontend-modern/src/utils/alertThresholdsPresentation.ts`
24. `frontend-modern/src/utils/alertThresholdsSectionPresentation.ts`
25. `internal/alerts/history.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`
2. Add typed collector/builders in `internal/alerts/alerts.go`
3. Add identity/persistence updates through canonical alert helpers only
4. Add or change alert history persistence through `internal/alerts/history.go` using normalized owned storage roots and fixed storage leaves only

## Forbidden Paths

1. New ad hoc `Check*`-local evaluator logic
2. Reintroducing runtime legacy alert-ID contracts
3. Reintroducing per-family threshold/override merge logic outside the shared path

## Completion Obligations

1. Update alert spec/evaluator tests when a new rule kind is added
2. Update this contract if alert truth or identity rules change
3. Route runtime changes through the explicit alert proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten or add guardrails when an old alert path is removed

## Current State

Canonical alert identity and evaluation are the live runtime model. Remaining
legacy references should exist only in explicit migration or negative test
boundaries.
Guest metric canonical state remains resource-backed and therefore node-scoped
for Proxmox guests, so node moves must not strand active alert state on the
previous resource ID. When a guest metric alert survives a node move, alerts
runtime must migrate the active alert, history entry, acknowledgment record,
suppression/rate-limit/flapping tracking, and guest per-disk metric identity
to the current canonical state instead of reopening a duplicate alert or
resolving only the stale node-scoped identity.
That same alerts runtime also owns instance-scoped node display-name
resolution. Raw node names are not globally unique across configured
infrastructure instances, so cached node display names must key on instance +
node identity whenever the alert carries instance context. Alert updates,
incident rebuilds, and guest-metric migrations may fall back to the legacy
name-only cache only for instance-less resources like standalone host agents.

Alert history persistence is also part of that canonical boundary. The history
manager may choose the owned runtime data directory, but it must normalize that
directory once and then resolve only the fixed `alert-history.json` and
`alert-history.backup.json` leaves through the shared storage-path helper
before any filesystem read, write, rename, or delete. Future history-persistence
changes must not reintroduce raw `filepath.Join(dataDir, ...)` joins from
caller-supplied directories or ad hoc history filenames.

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
status labels. Future alert configuration or history presentation work should
extend those helpers instead of rebuilding alert-specific semantics in pages,
dashboard surfaces, feature hooks, or thresholds shells.

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
That schedule surface now also follows the same shell/runtime split as the
other feature tabs: `frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx`
stays the render shell, while
`frontend-modern/src/features/alerts/useAlertScheduleState.ts` owns schedule
reset behavior, quiet-hours day/category toggles, cooldown/grouping/escalation
update policy, and the canonical defaults handoff. Future schedule control-flow
work should extend that hook instead of rebuilding those mutations inline in
the tab shell.

Incident-event filter chip and filter-action styling now routes through
`frontend-modern/src/utils/alertIncidentPresentation.ts` for both
`frontend-modern/src/pages/Alerts.tsx` and
`frontend-modern/src/features/alerts/OverviewTab.tsx` instead of allowing
those alert timeline surfaces to fork their own filter presentation.

Alert incident acknowledged badges, timeline event cards, and note-editor
presentation now also route through
`frontend-modern/src/utils/alertIncidentPresentation.ts` instead of remaining
duplicated inline across the alerts page and overview timeline surfaces.

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
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`, and
`frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`. New
threshold row grouping, override-ID compatibility, resource normalization,
thresholds-table controller logic, or per-tab runtime should land in those
feature hooks and tab owners rather than being rebuilt inside the shell.
The shell-owned thresholds sub-routes are now the neutral user-facing paths
`/alerts/thresholds/infrastructure`, `/alerts/thresholds/systems`,
`/alerts/thresholds/mail-gateway`, and `/alerts/thresholds/containers`.
Legacy `/alerts/thresholds/proxmox` and `/alerts/thresholds/agents` links
must redirect to the neutral infrastructure and systems routes so API-backed
platforms like TrueNAS do not remain stranded behind provider-specific deep
links.
Within the infrastructure tab, render-heavy ownership now further routes through
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
alert surfaces can evaluate, show, and drill into vSphere alarm and health
signals on canonical `agent`, `vm`, and `storage` resources, with related
event/task context routed through the shared incident and resource links.
Pulse must not grow a VMware-only alert shell, alarm editor, or direct alarm-
control surface in phase 1.
That same VMware alert rule now also includes the topology boundary. Alarm
context that originates on a datacenter, cluster, folder, or resource pool may
inform a shared incident, but it must still resolve onto canonical `agent`,
`vm`, or `storage` investigation paths rather than creating synthetic
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
resource-table empty states, badge labels, offline-state wording, note
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
timeline control flow should land in that feature hook instead of being
forked back into either alert surface.

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
notification feedback for both
`frontend-modern/src/features/alerts/useAlertOverviewState.ts` and
`frontend-modern/src/components/Alerts/RecentAlertsPanel.tsx`.
`frontend-modern/src/features/alerts/useAlertOverviewState.ts` now owns the
derived alert read-model and Last 24 Hours stat refresh for
`frontend-modern/src/features/alerts/OverviewTab.tsx`, while composing that
shared acknowledgement owner instead of keeping its own alert mutation fork.
Future overview or dashboard recent-alert action behavior should extend that
shared acknowledgement hook instead of putting acknowledge mutations back into
either render shell.
Render-heavy alert overview ownership now routes through
`frontend-modern/src/features/alerts/AlertOverviewStatsCards.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewActiveAlertsSection.tsx`,
and `frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx` instead
of rebuilding stats-card, active-alert, and timeline-card presentation inline
inside `frontend-modern/src/features/alerts/OverviewTab.tsx`.
Dashboard recent-alert rendering and dashboard alert summary/tone copy now
route through that same alert overview presentation owner and the alert-owned
`frontend-modern/src/components/Alerts/RecentAlertsPanel.tsx` surface instead
of living as a dashboard-page-local panel plus a second dashboard-only alert
presentation helper.

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
normalization plus resource-backed override projection, and
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
alerts beyond storage-only cases. `internal/alerts/alerts.go`,
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
