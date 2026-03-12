# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert identity, alert specs, evaluation, persistence semantics, and
notification behavior for live runtime alerts.

## Canonical Files

1. `internal/alerts/specs/types.go`
2. `internal/alerts/specs/evaluator.go`
3. `internal/alerts/canonical_metric.go`
4. `internal/alerts/canonical_lifecycle.go`
5. `internal/alerts/unified_incidents.go`

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

Alert incident event meta rows and detail text treatments now also route
through `frontend-modern/src/utils/alertIncidentPresentation.ts` instead of
keeping duplicate summary/detail typography inline in the alerts page and
overview timelines.

Resource incident panel cards, summary rows, and toggle-button presentation
now also route through `frontend-modern/src/utils/alertIncidentPresentation.ts`
instead of remaining inline inside `frontend-modern/src/pages/Alerts.tsx`.

Active alert card state, acknowledged badge, and primary/secondary action
button presentation now route through
`frontend-modern/src/utils/alertOverviewPresentation.ts` instead of remaining
inline in `frontend-modern/src/features/alerts/OverviewTab.tsx`.
