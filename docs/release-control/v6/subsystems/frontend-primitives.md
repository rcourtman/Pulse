# Frontend Primitives Contract

## Contract Metadata

```json
{
  "subsystem_id": "frontend-primitives",
  "lane": "L8",
  "contract_file": "docs/release-control/v6/subsystems/frontend-primitives.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own reusable frontend primitives and presentational patterns so feature work
extends shared components instead of creating new local variants.

## Canonical Files

1. `frontend-modern/src/components/shared/`
2. `frontend-modern/src/components/shared/PageControls.guardrails.test.ts`
3. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
4. `frontend-modern/src/features/`

## Shared Boundaries

1. None.

## Extension Points

1. Add shared primitives in `frontend-modern/src/components/shared/`
2. Add feature-specific presentation only when no shared primitive should own it
3. Add guardrail tests when a new shared pattern is introduced

## Forbidden Paths

1. Reinventing table/filter/toggle primitives when a shared version exists
2. Feature-local styling forks of canonical shared components without explicit justification
3. Direct imports that bypass shared presentation helpers where guardrails exist

## Completion Obligations

1. Update guardrail tests when new shared primitives are added
2. Update this contract when a new canonical UI pattern is adopted
3. Remove local forks after the shared primitive is introduced

## Current State

The frontend already has several guardrail tests. The next step is to keep
turning repeated local patterns into explicit shared primitives with hard usage
bounds.

The subsystem registry now also requires explicit proof-policy coverage for all
shared runtime files, and shared-component guardrails fail if raw table
composition is reintroduced in new shared components outside the canonical
allowlist.

General settings segmented selectors for theme preference and temperature unit
must now also route through the shared `FilterButtonGroup` primitive instead of
maintaining local button-group styling forks inside
`frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`.

Reporting time-range/export selectors and General settings Proxmox VE polling
presets must now also route through the shared `FilterButtonGroup` prominent
variant instead of maintaining local blue segmented-control styling forks in
feature components.
That same shared `FilterButtonGroup` primitive must stay CSP-safe: touch-scroll
overflow behavior must come from canonical CSS classes rather than inline
`style` attributes so settings and reporting selectors do not reintroduce
browser console CSP violations under the release build policy.

Selectable settings cards for compact provider pickers and detail choice panels
must now route through the shared `SelectionCardGroup` primitive instead of
duplicating border-2 active-card styling in feature components.

Settings informational callouts with icon-plus-copy layouts must now route
through the shared `CalloutCard` primitive instead of maintaining feature-local
blue bordered wrappers.

Alert incident-event filter containers, labels, and chips must now route
through the shared presentation helpers in
`frontend-modern/src/utils/alertIncidentPresentation.ts` instead of allowing
`frontend-modern/src/pages/Alerts.tsx` and
`frontend-modern/src/features/alerts/OverviewTab.tsx` to fork their own filter
button styling.

Alert incident acknowledged badges, event cards, and note-editor controls must
also route through `frontend-modern/src/utils/alertIncidentPresentation.ts`
instead of letting the alerts page and overview timeline maintain duplicate
inline incident-detail styling.

Alert incident meta-row and detail-text presentation must also route through
`frontend-modern/src/utils/alertIncidentPresentation.ts` instead of letting
the alerts page and overview timeline maintain duplicated inline incident
typography rules.

Resource incident panel card and summary-row presentation must also route
through `frontend-modern/src/utils/alertIncidentPresentation.ts` instead of
maintaining page-local incident panel styling inside
`frontend-modern/src/pages/Alerts.tsx`.

Active alert card state and action-button presentation must also route through
`frontend-modern/src/utils/alertOverviewPresentation.ts` instead of leaving
feature-local alert card styling inside
`frontend-modern/src/features/alerts/OverviewTab.tsx`.
