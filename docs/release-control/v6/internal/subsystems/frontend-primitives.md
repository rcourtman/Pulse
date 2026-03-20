# Frontend Primitives Contract

## Contract Metadata

```json
{
  "subsystem_id": "frontend-primitives",
  "lane": "L8",
  "contract_file": "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own reusable frontend primitives and canonical page-shell patterns so feature
work extends shared components instead of creating new local variants.

## Canonical Files

1. `frontend-modern/src/components/shared/`
2. `frontend-modern/src/components/Settings/Settings.tsx`
3. `frontend-modern/src/components/Settings/SettingsPageShell.tsx`
4. `frontend-modern/src/components/Settings/settingsPanelRegistry.ts`
5. `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
6. `frontend-modern/src/components/Settings/AISettings.tsx`
7. `frontend-modern/src/components/Settings/AuditLogPanel.tsx`
8. `frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`
9. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
10. `frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`
11. `frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`
12. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
13. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`
14. `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`
15. `frontend-modern/src/components/Settings/SettingsPageShell.tsx`
16. `frontend-modern/src/components/Settings/settingsPanelRegistry.ts`
17. `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`
18. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
19. `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
20. `tests/integration/tests/15-settings-shell-consistency.spec.ts`
21. `frontend-modern/src/components/shared/PageControls.guardrails.test.ts`
22. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
23. `frontend-modern/src/features/`
24. `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
25. `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
26. `frontend-modern/src/components/SetupWizard/__tests__/SetupWizard.test.tsx`
27. `frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx`
28. `frontend-modern/src/components/shared/MonitoredSystemLimitWarningBanner.tsx`

## Shared Boundaries

1. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `security-privacy`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
2. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `security-privacy`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
3. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `security-privacy`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.

## Extension Points

1. Add shared primitives in `frontend-modern/src/components/shared/`
2. Route new top-level settings surfaces through the canonical settings shell
   instead of introducing page-local framing
3. Add feature-specific presentation only when no shared primitive should own it
4. Add guardrail tests when a new shared pattern is introduced

## Forbidden Paths

1. Reinventing table/filter/toggle primitives when a shared version exists
2. Feature-local styling forks of canonical shared components without explicit justification
3. Direct imports that bypass shared presentation helpers where guardrails exist
4. Top-level settings panels introducing bespoke page-level headers or outer
   framing instead of the canonical settings shell and `SettingsPanel`
   contract

## Completion Obligations

1. Update guardrail tests when new shared primitives are added
2. Keep top-level settings surfaces routed through the canonical settings shell
   and maintain both `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
   plus `tests/integration/tests/15-settings-shell-consistency.spec.ts`
3. Update this contract when a new canonical UI pattern is adopted
4. Remove local forks after the shared primitive is introduced

## Current State

The frontend already has several guardrail tests. The next step is to keep
turning repeated local patterns into explicit shared primitives with hard usage
bounds.

The subsystem registry now also requires explicit proof-policy coverage for all
shared runtime files, and shared-component guardrails fail if raw table
composition is reintroduced in new shared components outside the canonical
allowlist.
Infrastructure summary and detail surfaces now also use the shared normalized
identity lookup helper from `frontend-modern/src/utils/resourceIdentity.ts`
so dotted hostnames and alias variants stay consistent between the shared
table, drawer, and detail views instead of each component carrying its own
identifier-variant logic.
Those same surfaces also share the trimmed-string helper from
`frontend-modern/src/utils/stringUtils.ts` so shared components do not keep
their own copy of the same whitespace-trimming identity logic.

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

Alert incident timeline event card structure must also route through
`frontend-modern/src/components/Alerts/IncidentTimelineEventCard.tsx` so the
alerts page and overview timeline share one canonical event-card renderer
instead of reimplementing the same summary/detail/output block twice.

The full expanded alert incident detail panel and event-filter controls must
also route through `frontend-modern/src/components/Alerts/IncidentTimelinePanel.tsx`
and `frontend-modern/src/components/Alerts/IncidentEventFilters.tsx` rather
than rebuilding loading/error copy, filter controls, note-editor wiring, or
event-card composition separately inside the alerts page and overview tab.

Resource incident panel card and summary-row presentation must also route
through `frontend-modern/src/utils/alertIncidentPresentation.ts` instead of
maintaining page-local incident panel styling inside
`frontend-modern/src/pages/Alerts.tsx`.

The resource incident panel's collapsed activity summary is now part of that
same shared primitive boundary. Event-type count chips, visible-event copy,
and the summary-ordering helper in `frontend-modern/src/features/alerts/types.ts`
must stay shared across alert timeline surfaces instead of rebuilding
page-local event summaries or bespoke incident-card markup.

Shared primitive consumers that split status-dot tone and status-text tone
must now keep both values routed through the same exported presentation helper.
Feature cards such as RAID status may not call shadow local aliases that drift
from the canonical shared class/variant helpers.

Active alert card state and action-button presentation must also route through
`frontend-modern/src/utils/alertOverviewPresentation.ts` instead of leaving
feature-local alert card styling inside
`frontend-modern/src/features/alerts/OverviewTab.tsx`.

Alert resource display labels used by the thresholds editor and alerts page
must now route through the shared helper in
`frontend-modern/src/features/alerts/helpers.ts` instead of rebuilding
resource display-name fallback chains inline. Governed resources must preserve
their canonical policy-aware label across grouped node headers, docker host
grouping, and saved override rows rather than collapsing back to raw names or
friendly-name truncation.

Shared search inputs must now keep their forwarded keyboard, blur, and clear
handlers as explicit callable functions instead of relying on loose Solid
event-handler unions. Shared search primitives still need to accept the real
input/button event targets, but direct invocation inside the primitive must
stay type-safe so consumers do not reintroduce union-call regressions while
adding history, shortcut, or trailing-control behavior.

Shared shared-shell primitives that expose semantic `title` or value-level
`onChange` props must now explicitly omit the conflicting DOM attribute names
from their inherited HTML props. `CalloutCard`, `FilterSegmentedControl`, and
`Subtabs` may still forward ordinary div attributes, but their canonical API
must preserve JSX element titles and value-callback handlers instead of
widening back to raw DOM attribute unions.

Shared entitlement/migration warning banners that live under
`frontend-modern/src/components/shared/` must also keep their counted fleet
surface on the Pulse Unified Agent term. Shared primitive copy may describe
legacy/API-connected resources separately, but it may not regress the primary
banner label or CTA text back to host-agent product language.
The self-hosted commercial paywall copy on those shared warning surfaces is
now also explicitly locked to monitored systems rather than agents. When a
shared banner or shared settings shell is explaining self-hosted plan caps,
the operator-facing commercial term must follow the monitored-system model even
if explicit legacy-v5 compatibility helpers still decode older alias fields at
import boundaries.
That banner boundary now also owns the canonical monitored-system naming
surface directly: the shared warning component path and exported symbol are
`MonitoredSystemLimitWarningBanner`, and future work may not reintroduce an
agent-era banner filename or component name as the primary primitive.
First-session educational surfaces must also stay brief, flat, and model-led.
When Pulse needs to teach a user how a flow works, the primary on-screen
guidance should collapse to a few short descriptions of the real product
mental model instead of a logo wall, feature brochure, or verbose internal
mechanics dump. The runtime wizard itself now stays on the two-step
`Welcome -> Security` path, while the separate setup-completion preview owns
the brief three-step explanation: install the Unified Agent, get the first
Pulse resource, then layer on additional context.

The settings shell is now also a governed frontend primitive boundary.

The alerts page shell now follows that same page-shell rule for feature tabs:
`frontend-modern/src/pages/Alerts.tsx` owns navigation and cross-surface
routing, while feature-owned tab surfaces such as
`frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx` and
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` plus
`frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx` and
`frontend-modern/src/features/alerts/tabs/ThresholdsTab.tsx` own their
tab-local rendering and interaction logic. Future alert tab cleanup should
continue by extracting page-local tab blocks into feature modules rather than
expanding the top-level page file again, and history-table behavior or
thresholds-table adapter logic should stay feature-owned unless it graduates
into a shared primitive used by more than one alert surface.
Within that thresholds surface, `frontend-modern/src/components/Alerts/ThresholdsTable.tsx`
is now explicitly a feature consumer rather than the data owner. Canonical
threshold row shaping, override-ID compatibility, and grouped resource
normalization live in
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`,
so future cleanup should extend that feature hook instead of rebuilding
resource normalization inside the table component.

The alerts page now also applies the same shell-versus-feature rule to
configuration orchestration. `frontend-modern/src/pages/Alerts.tsx` is the page
shell, while `frontend-modern/src/features/alerts/AlertsConfigurationSurface.tsx`
owns alert config load/save behavior, notification-config reloads, defaults,
and threshold-override normalization for the destinations, schedule, and
thresholds tabs. Future cleanup should continue by moving page-local config
control flow into that feature surface or a narrower shared primitive, not back
into the top-level page shell.
Top-level settings surfaces must route through `Settings.tsx`,
`SettingsPageShell.tsx`, and
`frontend-modern/src/components/shared/SettingsPanel.tsx` instead of
reintroducing bespoke outer page headers or one-off top-level panel framing.
The shell metadata driving those surfaces is part of the same boundary as
well: `frontend-modern/src/components/Settings/settingsHeaderMeta.ts` and
representative top-level panels such as
`frontend-modern/src/components/Settings/APIAccessPanel.tsx`,
`frontend-modern/src/components/Settings/AISettings.tsx`,
`frontend-modern/src/components/Settings/AuditLogPanel.tsx`,
`frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`,
`frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx` must keep
`frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` must keep
`frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`,
`frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`,
`frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`, and
`frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx` must keep
page-shell titles, descriptions, and lead panel framing aligned instead of
letting navigation/header labels drift away from the actual settings surface.
First-session runtime framing is now part of that same owned primitive story.
`frontend-modern/src/components/SetupWizard/SetupWizard.tsx` must stay on the
real two-step runtime shape (`Welcome`, then `Security`) and hand successful
setup directly into the canonical Infrastructure Operations install route
instead of regrowing a second setup-only completion step. The standalone
preview surface in
`frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx` may
still exist for guarded preview coverage, but it must remain explicitly
separate from the runtime wizard so first-session UX does not split into two
competing product flows again.
The release-ready shell proof now also includes a representative desktop
Playwright rehearsal in
`tests/integration/tests/15-settings-shell-consistency.spec.ts` so general,
organization, billing, relay, security, AI, updates, and recovery panels are
all exercised through the built app shell under a seeded multi-tenant runtime.
The security-facing settings panels within that shell now also follow an
explicit shared boundary with `security-privacy` so shell framing stays here
while auth posture, token controls, and privacy semantics remain governed as a
trust surface instead of generic UX copy.

Single-surface settings pages that only render one canonical `SettingsPanel`
must stay rooted directly at that panel instead of wrapping it in an extra
page-level `space-y-*` container. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
`frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`, and
`frontend-modern/src/components/Settings/AuditLogPanel.tsx` are the current
reference cases, and
`frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
locks that direct-root contract so single-surface pages do not quietly regain
redundant outer spacing chrome.
