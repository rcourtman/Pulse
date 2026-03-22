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

## Shared Boundaries

1. None.

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`
2. Add typed collector/builders in `internal/alerts/alerts.go`
3. Add identity/persistence updates through canonical alert helpers only

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
That threshold editor data shaping now lives under
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`,
while `frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`
owns threshold-table route sync, edit state, bulk-edit flow, and override
mutation control. `frontend-modern/src/components/Alerts/ThresholdsTable.tsx`
is now limited to table interaction and presentation. New threshold row
grouping, override-ID compatibility, resource normalization, or thresholds-table
controller logic should land in those hooks rather than being rebuilt inside
the table component.
The alert resource thresholds editor now follows the same shape: shared metric
normalization, bounds, value-resolution, and override-label logic live in
`frontend-modern/src/components/Alerts/alertResourceTableModel.ts`, render-heavy
desktop row ownership lives in
`frontend-modern/src/components/Alerts/AlertResourceTableRow.tsx`, and
selection state, delay-row toggling, and inline metric-input focus live in
`frontend-modern/src/components/Alerts/useAlertResourceTableState.ts`. Future
resource-table threshold semantics should land in those owners instead of
being rebuilt inline in `frontend-modern/src/components/Alerts/ResourceTable.tsx`.

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
`frontend-modern/src/features/alerts/useAlertHistoryState.ts`, which owns alert
history fetch, persistent filter state, trend-bucket derivation, grouped-row
projection, resource-incident panel loading, and history-clear flow. Future
alert history control-flow work should extend that feature hook instead of
putting data fetch or resource-incident state back into
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`.

Alert configuration load/save state, notification config reloads, and threshold
override normalization now route through
`frontend-modern/src/features/alerts/AlertsConfigurationSurface.tsx` instead of
living inline in `frontend-modern/src/pages/Alerts.tsx`. The page shell owns
navigation, activation chrome, and cross-surface routing; the configuration
surface is now a shell that composes the destinations, schedule, and thresholds
tabs. The canonical alert-policy runtime owner is now
`frontend-modern/src/features/alerts/useAlertsConfigurationState.ts`, while
notification destination reload and persistence now route through
`frontend-modern/src/features/alerts/useAlertDestinationsState.ts`. Future
config cleanup should extend the alert-policy hook or the destinations hook
based on which subsystem actually owns the behavior instead of letting the
broader configuration hook absorb both concerns again.

Alert filter metadata and grouped header consumers must also preserve the
canonical `agent` and `node` header boundary when reusing shared filter
primitives. Frontend alert tables may not drift back to ad hoc host-key
grouping or narrow filter key predicates that drop optional hostname values
before alert group metadata is derived.
