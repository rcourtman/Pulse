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
3. `frontend-modern/src/components/Settings/SettingsDialogs.tsx`
4. `frontend-modern/src/components/Settings/SettingsPageShell.tsx`
5. `frontend-modern/src/components/Settings/settingsPanelRegistry.ts`
6. `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
7. `frontend-modern/src/components/Settings/AIChatMaintenanceSection.tsx`
8. `frontend-modern/src/components/Settings/AIModelSelectionSection.tsx`
9. `frontend-modern/src/components/Settings/AIProviderConfigurationSection.tsx`
10. `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`
11. `frontend-modern/src/components/Settings/AISettings.tsx`
12. `frontend-modern/src/components/Settings/AISettingsDialogs.tsx`
13. `frontend-modern/src/components/Settings/AISettingsStatusAndActions.tsx`
14. `frontend-modern/src/components/Settings/aiSettingsModel.ts`
15. `frontend-modern/src/components/Settings/AuditLogPanel.tsx`
16. `frontend-modern/src/components/Settings/useAuditLogPanelState.ts`
17. `frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`
18. `frontend-modern/src/components/Settings/useAuditWebhookPanelState.ts`
19. `frontend-modern/src/components/Settings/CopyCommandBlock.tsx`
20. `frontend-modern/src/components/Settings/diagnosticsModel.ts`
21. `frontend-modern/src/components/Settings/DiagnosticsPanel.tsx`
22. `frontend-modern/src/components/Settings/DiagnosticsResultsPanel.tsx`
23. `frontend-modern/src/components/Settings/OperationsPanel.tsx`
24. `frontend-modern/src/utils/diagnosticsPresentation.ts`
24. `frontend-modern/src/utils/discoveryPresentation.ts`
24. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
25. `frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`
26. `frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`
27. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
28. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`
29. `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`
30. `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`
31. `frontend-modern/src/components/Settings/useAISettingsState.ts`
32. `frontend-modern/src/components/Settings/useDiagnosticsPanelState.ts`
33. `frontend-modern/src/components/Settings/useSettingsShellState.ts`
34. `frontend-modern/src/components/Settings/useSSOProvidersState.ts`
35. `frontend-modern/src/components/Settings/ssoProvidersModel.ts`
36. `frontend-modern/src/utils/ssoProviderPresentation.ts`
37. `frontend-modern/src/utils/systemSettingsPresentation.ts`
38. `frontend-modern/src/utils/aiSettingsPresentation.ts`
39. `frontend-modern/src/utils/settingsShellPresentation.ts`
40. `frontend-modern/src/utils/textPresentation.ts`
37. `frontend-modern/src/components/Settings/UpdateInstallGuide.tsx`
38. `frontend-modern/src/components/Settings/updatesSettingsModel.ts`
39. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
40. `frontend-modern/src/components/Settings/ReportingPanel.tsx`
41. `frontend-modern/src/components/Settings/reportingPanelModel.ts`
42. `frontend-modern/src/components/Settings/reportingInventoryExportModel.ts`
43. `frontend-modern/src/components/Settings/useReportingPanelState.ts`
44. `frontend-modern/src/utils/reportingPresentation.ts`
45. `frontend-modern/src/utils/updatesPresentation.ts`
44. `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
45. `tests/integration/tests/15-settings-shell-consistency.spec.ts`
46. `frontend-modern/src/components/shared/PageControls.guardrails.test.ts`
47. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
48. `frontend-modern/src/features/`
49. `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
50. `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
51. `frontend-modern/src/components/SetupWizard/__tests__/SetupWizard.test.tsx`
52. `frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx`
53. `frontend-modern/src/components/shared/MonitoredSystemLimitWarningBanner.tsx`
54. `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
55. `frontend-modern/src/components/Settings/useSystemLogsPanelState.ts`
56. `frontend-modern/src/utils/systemLogsPresentation.ts`
57. `frontend-modern/src/components/Settings/__tests__/SystemLogsPanel.test.tsx`
58. `frontend-modern/src/features/operations/OperationsPageSurface.tsx`
59. `frontend-modern/src/features/operations/operationsPageModel.ts`
60. `frontend-modern/src/pages/Operations.tsx`
61. `frontend-modern/src/components/Settings/ResourcePicker.tsx`
62. `frontend-modern/src/components/Settings/reportingResourceTypes.ts`
63. `frontend-modern/src/utils/reportableResourceTypes.ts`
64. `frontend-modern/src/utils/reportingResourceTypes.ts`
65. `frontend-modern/src/utils/problemResourcePresentation.ts`
66. `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`
67. `frontend-modern/src/utils/dashboardGuestPresentation.ts`
68. `frontend-modern/src/utils/dashboardKpiPresentation.ts`
69. `frontend-modern/src/utils/dashboardTrendPresentation.ts`
70. `frontend-modern/src/components/Toast/Toast.tsx`
71. `frontend-modern/src/utils/toast.ts`
72. `frontend-modern/src/utils/semanticTonePresentation.ts`
73. `frontend-modern/src/utils/emptyStatePresentation.ts`
74. `frontend-modern/src/utils/typeColumnPresentation.ts`
61. `frontend-modern/src/pages/__tests__/Operations.helpers.test.ts`
59. `frontend-modern/src/components/Settings/NetworkDiscoverySection.tsx`
60. `frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
61. `frontend-modern/src/components/Settings/networkSettingsModel.ts`
62. `frontend-modern/src/components/Settings/useDiscoverySettingsState.ts`
63. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
64. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
65. `frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts`
66. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
67. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`
68. `frontend-modern/src/components/Settings/settingsNavVisibility.ts`
69. `frontend-modern/src/components/Settings/settingsRouting.ts`
70. `frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts`
71. `frontend-modern/src/components/Settings/settingsTypes.ts`
72. `frontend-modern/src/components/Settings/useSettingsNavigation.ts`
73. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
74. `frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx`
75. `frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx`
76. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
77. `frontend-modern/src/utils/environmentLockPresentation.ts`

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
The settings reporting shell now also owns a deliberate split between
historical performance reports and current-state VM inventory export.
`frontend-modern/src/components/Settings/ReportingPanel.tsx`,
`frontend-modern/src/components/Settings/useReportingPanelState.ts`,
`frontend-modern/src/components/Settings/reportingCatalogModel.ts`,
`frontend-modern/src/components/Settings/reportingPanelModel.ts`, and
`frontend-modern/src/components/Settings/reportingInventoryExportModel.ts` must
keep those as separate operator jobs with separate request builders and success
copy, rather than collapsing inventory export back into the metrics-report
controls.
That same settings shell must now also render both historical performance
options and VM inventory schema from the backend-owned reporting catalog rather
than hardcoding panel copy, routes, or range presets in the frontend. The
frontend models may validate and present the catalog, but the canonical panel
title, descriptions, endpoints, filename prefixes, range windows, and column
list belong to the API reporting contract.
The same reporting catalog ownership now also governs the operator resource-
selection cap for performance reports. `ReportingPanel.tsx` and
`ResourcePicker.tsx` may present or enforce that limit, but they must receive
it from the backend-owned `multiResourceMax` definition rather than hardcoding
the reporting cap in frontend-local constants.
That same catalog-owned capability contract also governs which optional
performance-report controls appear at all. The settings shell and reporting
request builder may not assume metric filtering or custom titles are always
available; they must honor `supportsMetricFilter` and `supportsCustomTitle`
from the backend catalog and avoid emitting unsupported controls or request
parameters from frontend-local defaults.
That same backend-owned catalog also owns the initial reporting selections and
transport details. `useReportingPanelState.ts`,
`reportingPanelModel.ts`, and `reportingInventoryExportModel.ts` may not seed
format/range selections from legacy frontend constants or invent fallback
report endpoints, filename prefixes, default report titles, or range windows
or fallback filename date-stamp styles
when the catalog is
present; the first valid selection and all request semantics must come from the
parsed backend definition.
The same rule applies to VM inventory export transport details: request
builders and fallback filenames must derive the export format and extension
from the parsed inventory definition instead of hardcoding `csv` in frontend
helpers.
That same fallback contract also includes the single-report filename subject,
so frontend download builders may not substitute resource display names when
the catalog says fallback attachment names are keyed off canonical resource IDs.
That same reporting transport contract also means the frontend download path
must prefer the backend `Content-Disposition` filename over any locally built
fallback name when a report or inventory export response arrives.
That same settings shell must also read the reporting catalog for locked users,
not just entitled users, so the paywalled reporting panel does not drift onto a
separate frontend-owned title or description contract.
That same settings shell must now also treat the reporting catalog as the
feature-identity source once it loads. `ReportingPanel.tsx` and
`useReportingPanelState.ts` may use a generic loading or error shell before the
catalog is available, but they must not hardcode the reporting feature key or
the entitled and locked panel title and description once the catalog has
loaded.
The same metadata route is readable without the reporting feature gate, so the
settings shell must not delay the catalog fetch on `licenseLoaded()` before it
can render its canonical loading, locked, or entitled states.
That same catalog load must also remain retryable after transient failure.
`useReportingPanelState.ts` may memoize or dedupe in-flight work, but it must
not permanently latch a failed first fetch and force operators to reload the
entire settings page before the reporting shell can recover.
That same catalog-owned contract also includes the locked teaser copy itself:
`ReportingPanel.tsx` may style or place the paywall content, but the locked
title and description must come from the parsed reporting catalog instead of
hardcoded component strings.
That same reporting catalog also owns the enabled-shell guidance callout that
explains when to use performance reports versus VM inventory export.
`ReportingPanel.tsx` may choose the presentation primitive, but the callout
title and description must come from the parsed catalog instead of a
frontend-local explainer paragraph.
The shared updates settings owner also defines the user-facing framing for
rc-tagged builds. `frontend-modern/src/components/Settings/updatesSettingsModel.ts`
and `frontend-modern/src/utils/updatesPresentation.ts` must present that
channel as a prerelease or preview path with manual validation expectations,
not as a near-ready release candidate promise.
The root app shell now also treats backend availability as distinct from
websocket liveness: `frontend-modern/src/AppLayout.tsx` and
`frontend-modern/src/useAppRuntimeState.ts` must keep the top-right connection
badge aligned to overall backend availability so a healthy dev/runtime backend
does not present the whole shell as reconnecting just because the live stream
is transiently renegotiating. That shell badge must now stay on an explicit
state model as well: healthy runtime, backend-healthy-but-stream-degraded,
full reconnect, and full disconnect are distinct operator states, and the
shared shell may not collapse them back into one generic reconnect label.
Shared feature presentation helpers under `frontend-modern/src/features/` now
also need to preserve route-owned page-health semantics when the owning surface
is REST-backed: operators should only see reconnect or disconnected shells when
the route's own data contract is unhealthy, not because a global websocket
singleton is transiently reconnecting.
Those same feature-owned header badges must also stay aligned to the owning
runtime state instead of surfacing stale auxiliary counters as primary status;
an exhausted quickstart-credit badge cannot override an otherwise active Patrol
runtime unless quickstart exhaustion is the active blocker.
That same route-owned presentation rule also governs Patrol findings empty
states: shared section shells under `frontend-modern/src/features/patrol/`
must not render a green healthy empty state from `0 active findings` alone
when the owning Patrol runtime or overall-health summary is degraded, blocked,
or not fully verified.
The same hierarchy also applies inside the Patrol summary shell: once the
primary summary card states Patrol's assessment and verification basis,
supporting metric strips under that card must stay metric-oriented and must
not repeat assessment or verification labels as a second compact verdict row.
That same summary-shell rule also applies to timing metadata: if the header,
verification card, or findings footer already presents the governed Patrol
activity timestamp, the summary chip row must not add another recency badge
that competes with those owned timing surfaces.
The same ownership split applies to supporting counts: if the Patrol summary
surface renders the metric strip for active findings, warnings, criticals, and
fixes, the primary summary card should not repeat those same counts in badge
form beside the assessment and verification copy.
That same ownership rule applies to empty-state timing metadata. When the
Patrol page header already carries schedule and recency context, the findings
empty state should not add its own footer for `Last activity`, `Next run`, or
run interval text.
Those supporting cards must also keep their content factual and count-based:
active findings, critical findings, warnings, and fixes are valid secondary
readouts, while labels such as `Issues detected` or `Partial verification`
belong only to the primary Patrol assessment and verification surfaces.
The same applies to the Patrol activity strip during active execution: the
shared feature surface may add an explicit run-in-progress badge, but the main
status-strip label remains factual activity copy rather than shifting into a
second Patrol verdict label while a run is underway.
`frontend-modern/src/components/shared/TagBadges.tsx` is now also the
canonical tag-badge primitive. Dashboard workload rows and the unified-resource
detail drawer must import that shared owner instead of keeping a dashboard-local
tag badge variant or importing a feature-local path into infrastructure
surfaces.
`frontend-modern/src/components/Settings/OperationsPanel.tsx` is now also the
canonical shared settings wrapper for operations-style panels such as
diagnostics, reporting, and system logs. Those surfaces must extend that owner
instead of rebuilding a local `SettingsPanel` wrapper, panel-header action
slot, or divided content-body framing inline.
The system logs operations surface now follows the same shell/runtime split as
the other modernized settings panels: `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
owns the operations framing and consumes the canonical stream-copy/status
helpers from `frontend-modern/src/utils/systemLogsPresentation.ts`, while
`frontend-modern/src/components/Settings/useSystemLogsPanelState.ts` owns the
stream lifecycle, buffering, level updates, and download action. Future system
logs work must extend that split instead of pulling `EventSource`, API calls,
notification flow, or customer-facing system-log copy back into the panel
render shell.
Shared trial CTA handling is now part of that same primitive boundary for
settings and shared paywalls. Shared/settings runtime owners must derive trial
eligibility from the canonical entitlements payload, including
`trial_eligible`, and route operator-facing failure copy through
`frontend-modern/src/utils/upgradePresentation.ts`. The trial-start runtime
handoff itself is now centralized in
`frontend-modern/src/utils/trialStartAction.ts`; settings/shared paywalls and
onboarding surfaces must use that owner for redirect, success-notification, and
canonical denial handling instead of open-coding local `startProTrial()`
branches or re-interpreting backend status codes.
Top-level route files are now also expected to stay thin when a feature owns
the real product surface. `frontend-modern/src/pages/Infrastructure.tsx` now
acts only as the route boundary, while
`frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
owns the shell, `frontend-modern/src/features/infrastructure/useInfrastructurePageState.ts`
owns page-control composition, persistence, and route composition,
`frontend-modern/src/features/infrastructure/infrastructurePageModel.ts`
owns filter/search/catalog derivation, and
`frontend-modern/src/features/infrastructure/useInfrastructurePageRouteState.ts`
owns infrastructure route/deep-link synchronization. Future feature
surfaces under `frontend-modern/src/features/` should follow that same pattern
instead of letting page files accumulate route sync, filter, and modal
orchestration inline.
Infrastructure summary and detail surfaces now also use the shared normalized
identity lookup helper from `frontend-modern/src/utils/resourceIdentity.ts`
so dotted hostnames and alias variants stay consistent between the shared
table, drawer, and detail views instead of each component carrying its own
identifier-variant logic.
Those same surfaces also share the trimmed-string helper from
`frontend-modern/src/utils/stringUtils.ts` so shared components do not keep
their own copy of the same whitespace-trimming identity logic.
The shared infrastructure summary table now also follows the same
shell/runtime/model shape as the rest of the modernized primitives.
`frontend-modern/src/components/shared/InfrastructureSummaryTable.tsx` stays
the table shell, `frontend-modern/src/components/shared/useInfrastructureSummaryTableState.ts`
owns alert wiring, sort state, breakpoint state, and expanded-row lifecycle,
`frontend-modern/src/components/shared/infrastructureSummaryTableModel.ts`
owns sorting, count, identity-alias, and linked-agent derivation, and
`frontend-modern/src/components/shared/InfrastructureSummaryTableRow.tsx`
owns the per-row render/runtime surface. Future work should extend those
owners instead of pushing websocket, alert, or identity plumbing back into the
shared table shell.
The shared infrastructure selector now follows that same owner split.
`frontend-modern/src/components/shared/InfrastructureSelector.tsx` stays the
render shell, `frontend-modern/src/components/shared/useInfrastructureSelectorState.ts`
owns selected-node state, tab-reset and escape-key lifecycle, plus hook-backed
resource and recovery composition, and
`frontend-modern/src/components/shared/infrastructureSelectorModel.ts` owns
resource-family counts, agent-backed node-summary projection, unified-node and
PBS-instance projection, and recovery backup-count derivation. Future
infrastructure-selector work should extend those owners instead of pushing
resource aggregation or selection lifecycle back into the shared shell.
That shared selector projection must also preserve canonical local operator
identity for agent-backed infrastructure labels. Governed or AI-safe resource
summaries may inform policy/detail surfaces, but the selector's summary and
drawer-facing agent labels must continue to use the same local instance
identity boundary as the operator-facing infrastructure tables so multiple PBS,
PMG, or other governed resources remain distinguishable.
The shared infrastructure details drawer now follows that same owner split.
`frontend-modern/src/components/shared/InfrastructureDetailsDrawer.tsx` stays
the render shell, `frontend-modern/src/components/shared/useInfrastructureDetailsDrawerState.ts`
owns tab-selection runtime, and
`frontend-modern/src/components/shared/infrastructureDetailsDrawerModel.ts`
owns canonical metadata-id and discovery-hostname derivation. Future
infrastructure-details-drawer work should extend those owners instead of
pushing tab state or resource-identity normalization back into the shared
shell.
The shared interactive sparkline now follows that same split.
`frontend-modern/src/components/shared/InteractiveSparkline.tsx` stays the
render shell, `frontend-modern/src/components/shared/useInteractiveSparklineState.ts`
owns hover state, RAF throttling, canvas draw scheduling, and resize lifecycle,
and `frontend-modern/src/components/shared/interactiveSparklineModel.ts` owns
sparkline downsampling, gap segmentation, axis-tick math, and hover-selection
policy. Future sparkline work should extend those owners instead of pushing
canvas scheduling or chart-shape math back into the shared component shell.
The dashboard overview trend cards now also have an explicit shared-shell
obligation: `frontend-modern/src/features/dashboardOverview/TrendCharts.tsx`
must treat missing infrastructure history as a governed empty state rather than
as a silent blank sparkline box. Error copy and empty-history copy belong to
the feature shell, while the data path and chart-shaping logic must stay in the
owned hook/model layers that feed it.
That same dashboard boundary now also owns the shared dashboard presentation
helpers through `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`,
`frontend-modern/src/utils/dashboardGuestPresentation.ts`,
`frontend-modern/src/utils/dashboardKpiPresentation.ts`,
`frontend-modern/src/utils/dashboardMetricPresentation.ts`, and
`frontend-modern/src/utils/dashboardTrendPresentation.ts`. Dashboard loading,
disconnect, and empty states; guest backup/disk fallback copy; KPI card
framing; status-badge, delta, percent, and action-priority formatting; and
trend palette/error copy must extend those helpers instead of being redefined
inline in route shells, guest rows, or overview cards.
That shell must also stay passive with respect to data ownership: dashboard
trend cards may render the summary-range controls and operator-facing empty or
error copy, but they must not reintroduce route-local metrics-history fetch
loops for CPU and memory sparklines when the canonical infrastructure summary
surface already owns the chart contract.
The shared density map now follows that same owner split.
`frontend-modern/src/components/shared/DensityMap.tsx` stays the render shell,
`frontend-modern/src/components/shared/useDensityMapState.ts` owns hover
signals, canvas draw lifecycle, and resize handling, and
`frontend-modern/src/components/shared/densityMapModel.ts` owns bucket/window
math, hover target selection, tooltip time formatting, and density-cell
opacity rules. Future density-map work should extend those owners instead of
pushing canvas lifecycle or chart math back into the shared shell.
The shared active-use trial nudge now follows that same owner split.
`frontend-modern/src/components/shared/ActiveUseTrialNudge.tsx` stays the
render shell, `frontend-modern/src/components/shared/useActiveUseTrialNudgeState.ts`
owns first-seen persistence, snooze state, hourly age refresh, and trial-start
runtime, and `frontend-modern/src/components/shared/activeUseTrialNudgeModel.ts`
owns the eligibility policy, age threshold, and nudge copy/config. Future
active-use trial work should extend those owners instead of pushing storage
policy, timers, or commercial action flow back into the shared shell.
The shared trial banner now follows that same owner split.
`frontend-modern/src/components/shared/TrialBanner.tsx` stays the render
shell, `frontend-modern/src/components/shared/useTrialBannerState.ts` owns
entitlement load, snooze lifecycle, and upgrade-link runtime, and
`frontend-modern/src/components/shared/trialBannerModel.ts` owns day-count
normalization, tone policy, and display labels. Future trial-banner work
should extend those owners instead of pushing entitlement orchestration,
snooze state, or tone math back into the shared shell.
The shared column picker now follows that same owner split.
`frontend-modern/src/components/shared/ColumnPicker.tsx` stays the render
shell, `frontend-modern/src/components/shared/useColumnPickerState.ts` owns
dropdown open state and outside-click listener lifecycle, and
`frontend-modern/src/components/shared/columnPickerModel.ts` owns hidden-column
count, reset visibility policy, and column-option text-class/copy policy.
Future column-picker work should extend those owners instead of pushing
document-level listener logic or column-count policy back into the shell.
The shared tag input now follows that same owner split.
`frontend-modern/src/components/shared/TagInput.tsx` stays the render shell,
`frontend-modern/src/components/shared/useTagInputState.ts` owns input state,
container-focus runtime, and tag add/remove/backspace orchestration, and
`frontend-modern/src/components/shared/tagInputModel.ts` owns delimiter keys,
placeholder policy, remove-title copy, and canonical next-tag derivation.
Future tag-input work should extend those owners instead of pushing DOM reach-in
or tag-mutation policy back into the shell.
The shared scroll-to-top button now follows that same owner split.
`frontend-modern/src/components/shared/ScrollToTopButton.tsx` stays the render
shell, `frontend-modern/src/components/shared/useScrollToTopButtonState.ts`
owns scroll-listener lifecycle, visible state, and smooth-scroll runtime, and
`frontend-modern/src/components/shared/scrollToTopButtonModel.ts` owns
scrollable-ancestor discovery, visibility threshold policy, aria label, and
button class policy. Future scroll-to-top work should extend those owners
instead of pushing scroll-container discovery or listener lifecycle back into
the shell.
The shared toggle now follows that same owner split.
`frontend-modern/src/components/shared/Toggle.tsx` stays the render shell,
`frontend-modern/src/components/shared/useToggleState.ts` owns disabled gating
plus the synthetic toggle change-event runtime, and
`frontend-modern/src/components/shared/toggleModel.ts` owns size resolution,
track/knob/container class policy, and the canonical toggle event type.
Future toggle work should extend those owners instead of pushing synthetic
event behavior or size/class policy back into the shell.
The shared status badge now follows that same owner split.
`frontend-modern/src/components/shared/StatusBadge.tsx` stays the render shell,
`frontend-modern/src/components/shared/useStatusBadgeState.ts` owns disabled
gating and click runtime, and
`frontend-modern/src/components/shared/statusBadgeModel.ts` owns size padding,
label/title fallback policy, and status-badge class selection. Future status
badge work should extend those owners instead of pushing label/title policy or
disabled click handling back into the shell.
The shared segmented selector now follows that same owner split.
`frontend-modern/src/components/shared/FilterButtonGroup.tsx` stays the render
shell, `frontend-modern/src/components/shared/useFilterButtonGroupState.ts`
owns variant resolution plus disabled selection/change runtime, and
`frontend-modern/src/components/shared/filterButtonGroupModel.ts` owns the
variant class catalog, compact-label policy, and segmented button class
selection. Future filter-button-group work should extend those owners instead
of pushing label truncation or segmented variant policy back into the shell.
The shared selection-card primitive now follows that same owner split.
`frontend-modern/src/components/shared/SelectionCardGroup.tsx` stays the render
shell, `frontend-modern/src/components/shared/useSelectionCardGroupState.ts`
owns variant resolution plus disabled selection/change runtime, and
`frontend-modern/src/components/shared/selectionCardGroupModel.ts` owns the
tone fallback, group/button class catalog, and title/description presentation
policy. Future selection-card-group work should extend those owners instead of
pushing tone or active-card presentation logic back into the shell.
The shared dialog now follows that same owner split.
`frontend-modern/src/components/shared/Dialog.tsx` stays the render shell,
`frontend-modern/src/components/shared/useDialogState.ts` owns focus trap,
body-scroll lock, previous-focus restoration, and backdrop-close runtime, and
`frontend-modern/src/components/shared/dialogModel.ts` owns focusable-element
lookup plus layout and panel class policy. Future dialog work should extend
those owners instead of pushing focus-trap lifecycle or layout policy back into
the shared shell.
The shared history chart now follows the same owner shape.
`frontend-modern/src/components/shared/HistoryChart.tsx` stays the render
shell, `frontend-modern/src/components/shared/useHistoryChartState.ts` owns
license gating, trial actions, history fetch/refresh, canvas draw lifecycle,
and hover state, and `frontend-modern/src/components/shared/historyChartModel.ts`
owns tooltip formatting, scale and axis math, and closest-point selection.
Future history-chart work should extend those owners instead of pushing fetch,
license, or canvas math back into the shared component shell.
The remaining header, overlay, and tooltip render surfaces now live in
`frontend-modern/src/components/shared/HistoryChartHeader.tsx`,
`frontend-modern/src/components/shared/HistoryChartOverlay.tsx`, and
`frontend-modern/src/components/shared/HistoryChartTooltip.tsx` instead of
re-accumulating those sections inline in the shell.
The shared container update badge now follows that same owner split.
`frontend-modern/src/components/shared/ContainerUpdateBadge.tsx` stays the
render surface for the badge, icon, and update button shells,
`frontend-modern/src/components/shared/useContainerUpdateButtonState.ts` owns
Docker update mutation flow, persistent update-store state, settings gating,
and button lifecycle, and
`frontend-modern/src/components/shared/containerUpdateBadgeModel.ts` owns badge
and button tooltip formatting, class selection, and label/state presentation.
Future container-update work should extend those owners instead of pushing
store wiring, settings reads, or mutation flow back into the shared shell.
The shared web interface URL field now follows that same owner split.
`frontend-modern/src/components/shared/WebInterfaceUrlField.tsx` stays the
render shell, `frontend-modern/src/components/shared/useWebInterfaceUrlFieldState.ts`
owns metadata fetch/save/remove lifecycle, success/error state, and suggested
URL runtime, and `frontend-modern/src/components/shared/webInterfaceUrlFieldModel.ts`
owns URL validation, target-label normalization, and suggested-URL presentation
rules. The shared primitive now also supports an embedded mode with a caller-
owned title so feature drawers can place web-interface controls inside a larger
access surface without forking the save/remove/runtime behavior. Future
web-interface URL work should extend those owners instead of pushing metadata
transport or validation back into the shared shell.
The shared help icon now follows that same owner split.
`frontend-modern/src/components/shared/HelpIcon.tsx` stays the render shell,
`frontend-modern/src/components/shared/useHelpIconState.ts` owns open state,
popover-position lifecycle, and global click/escape listeners, and
`frontend-modern/src/components/shared/helpIconModel.ts` owns help-content
resolution, icon sizing, missing-content warnings, and popover-position math.
Future help-icon work should extend those owners instead of pushing registry
lookups or DOM listener lifecycle back into the shared shell.
The shared mobile nav now follows that same owner split.
`frontend-modern/src/components/shared/MobileNavBar.tsx` stays the render
shell, `frontend-modern/src/components/shared/useMobileNavBarState.ts` owns
fade signals, scroll and resize listeners, active-tab centering, and click
handoff lifecycle, and
`frontend-modern/src/components/shared/mobileNavBarModel.ts` owns platform and
utility tab ordering, alert badge counts, fade-state derivation, and tab
button class policy. Future mobile-nav work should extend those owners instead
of pushing tab-order or DOM lifecycle logic back into the shared shell.
The shared command palette now follows that same owner split.
`frontend-modern/src/components/shared/CommandPaletteModal.tsx` stays the
render shell, `frontend-modern/src/components/shared/useCommandPaletteState.ts`
owns query state, open-reset/focus lifecycle, route-path wiring, and Enter-key
selection, and `frontend-modern/src/components/shared/commandPaletteModel.ts`
owns canonical command construction plus query normalization and filtering
policy. Future command-palette work should extend those owners instead of
pushing route construction or search policy back into the shared shell.
The shared search field now follows that same owner split.
`frontend-modern/src/components/shared/SearchField.tsx` stays the render shell,
`frontend-modern/src/components/shared/useSearchFieldState.ts` owns focused-
Escape clear/blur behavior and input-ref lifecycle, and
`frontend-modern/src/components/shared/searchFieldModel.ts` owns clear/shortcut
visibility rules plus trailing-control padding policy. Future search-field work
should extend those owners instead of pushing event behavior or layout policy
back into the shared shell.
The shared search input now follows that same owner split.
`frontend-modern/src/components/shared/SearchInput.tsx` stays the render shell,
`frontend-modern/src/components/shared/useSearchInputState.ts` owns input-ref
lifecycle, type-to-search registration, and enhancement runtime composition,
and `frontend-modern/src/components/shared/searchInputModel.ts` owns the shared
search-input contract plus shortcut-hint and trailing-control policy. Future
search-input work should extend those owners instead of pushing type-to-search
or enhancement wiring back into the shared shell.
The search-input enhancement surfaces now follow that same owner split.
`frontend-modern/src/components/shared/SearchInputEnhancements.tsx` stays the
render shell, `frontend-modern/src/components/shared/useSearchInputEnhancements.ts`
owns search-history persistence, menu-open lifecycle, blur commit policy, and
tips/history interaction runtime, and
`frontend-modern/src/components/shared/searchInputEnhancementsModel.ts` owns
history-toggle copy plus history-menu button and row class policy. Future
search-input-enhancement work should extend those owners instead of pushing
history copy or menu presentation policy back into the shell.
The shared search tips popover now follows that same owner split.
`frontend-modern/src/components/shared/SearchTipsPopover.tsx` stays the render
shell, `frontend-modern/src/components/shared/useSearchTipsPopoverState.ts`
owns open-state, pointer/focus continuity, and outside-click/Escape listener
runtime, and `frontend-modern/src/components/shared/searchTipsPopoverModel.ts`
owns trigger variant, label/id defaults, hover policy, and trigger/popover
class selection. Future search-tips work should extend those owners instead of
pushing listener lifecycle or trigger policy back into the shared shell.
The shared what's-new modal now follows that same owner split.
`frontend-modern/src/components/shared/WhatsNewModal.tsx` stays the render
shell, `frontend-modern/src/components/shared/useWhatsNewModalState.ts` owns
local-storage dismissal, session dismissal, and close behavior, and
`frontend-modern/src/components/shared/whatsNewModalModel.ts` owns the feature
card catalog, telemetry copy, labels, and canonical docs/privacy links. Future
what's-new work should extend those owners instead of pushing dismissal state,
product copy, or external links back into the shared shell.
The shared tooltip now follows that same owner split.
`frontend-modern/src/components/shared/Tooltip.tsx` stays the render shell and
singleton API boundary, `frontend-modern/src/components/shared/useTooltipState.ts`
owns tooltip positioning lifecycle, RAF scheduling, and singleton visibility
state, and `frontend-modern/src/components/shared/tooltipModel.ts` owns tooltip
sanitization plus viewport-clamped positioning math. Future tooltip work should
extend those owners instead of pushing singleton state, DOM measurement, or
sanitization logic back into the shared shell.
The shared collapsible search input now follows that same owner split.
`frontend-modern/src/components/shared/CollapsibleSearchInput.tsx` stays the
render shell, `frontend-modern/src/components/shared/useCollapsibleSearchInputState.ts`
owns expand/collapse state, focus choreography, and type-to-search handoff, and
`frontend-modern/src/components/shared/collapsibleSearchInputModel.ts` owns
trigger-label, expanded-visibility, and full-width layout policy. Future
collapsible-search work should extend those owners instead of pushing
expand/collapse runtime or layout rules back into the shared shell.
The shared pulse data grid now follows that same owner split.
`frontend-modern/src/components/shared/PulseDataGrid.tsx` stays the render
shell, `frontend-modern/src/components/shared/usePulseDataGridState.ts` owns
breakpoint-driven min-width selection and stable-row reconciliation, and
`frontend-modern/src/components/shared/pulseDataGridModel.ts` owns alignment
class policy plus interactive-target row-click protection. Future pulse-data-
grid work should extend those owners instead of pushing breakpoint lifecycle or
interaction policy back into the shared shell.

The audit log settings surface now follows that same owner split.
`frontend-modern/src/components/Settings/AuditLogPanel.tsx` stays the canonical
`SettingsPanel` shell, while
`frontend-modern/src/components/Settings/useAuditLogPanelState.ts` owns the
license/paywall lifecycle, persisted filters, verification flow, and audit-log
fetch orchestration. The shell must not re-accumulate localStorage or API
runtime logic inline.

The audit webhook settings surface now follows that same owner split.
`frontend-modern/src/components/Settings/AuditWebhookPanel.tsx` stays the
canonical `SettingsPanel` shell, while
`frontend-modern/src/components/Settings/useAuditWebhookPanelState.ts` owns the
license/paywall lifecycle, webhook fetch/save flow, validation, and trial
startup orchestration. The shell must not re-accumulate API calls or paywall
tracking inline.

The diagnostics settings surface now follows that same owner split.
`frontend-modern/src/components/Settings/DiagnosticsPanel.tsx` stays the
top-level diagnostics shell, while
`frontend-modern/src/components/Settings/useDiagnosticsPanelState.ts`,
`frontend-modern/src/components/Settings/DiagnosticsResultsPanel.tsx`,
`frontend-modern/src/components/Settings/diagnosticsModel.ts`, and
`frontend-modern/src/utils/diagnosticsPresentation.ts` own the diagnostics
run/export lifecycle, results rendering, sanitization/model helpers, and
customer-facing diagnostics copy. The shell must not re-accumulate inline API
calls, export-download plumbing, diagnostics-card composition, or diagnostics
surface copy.

The settings shell registry now also treats extracted feature prop contracts as
canonical shell inputs instead of reaching back into feature panels for type
ownership. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
must consume the direct Proxmox panel contract through
`frontend-modern/src/components/Settings/proxmoxSettingsModel.ts`, so the
registry stays a shell/composition owner and does not depend on
`ProxmoxSettingsPanel.tsx` as though the panel still owned the runtime model.

The operations route now follows the same thin-route pattern as infrastructure,
storage, and Patrol. `frontend-modern/src/pages/Operations.tsx` stays the route
shell, `frontend-modern/src/features/operations/OperationsPageSurface.tsx` owns
the tabbed operations surface, and
`frontend-modern/src/features/operations/operationsPageModel.ts` owns the tab
and path contract. The operations route must keep its navigation routed through
the shared `frontend-modern/src/components/shared/Subtabs.tsx` primitive rather
than rebuilding a bespoke page-local tab bar.

The dashboard overview route now follows that same feature-owner pattern for
its dashboard-specific summary surfaces. `frontend-modern/src/pages/Dashboard.tsx`
stays the route shell, while `frontend-modern/src/features/dashboardOverview/`
owns the dashboard-specific action, KPI, problem-resource, trend, and
customization surfaces. Lane-owned widgets like recent alerts, storage,
and recovery must continue to route through their own subsystem owners instead
of drifting back into a page-local dashboard panel cluster.
The recovery feature shell now also depends on the shared
`frontend-modern/src/components/shared/Subtabs.tsx` primitive for its primary
protected-items versus recovery-events workspace switch. The recovery lane may
own the active view and route-state semantics, but the top-level tab framing
must stay on the canonical shared subtabs control instead of reviving a
recovery-local switcher pattern.
That same recovery shell boundary now also owns one canonical top-level filter
controller in
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`. Route-backed
recovery filters such as the provider-neutral `itemType` selector must be
derived, normalized, and fanned out to inventory, history, activity, facets,
and series consumers from that shared state owner rather than being recreated
as page-local toolbar state inside individual recovery sections.
That same shared recovery state owner now also keeps `platform` as the
canonical route and transport filter name for operator-facing recovery links,
while any accepted legacy `provider` aliases remain parser compatibility only.
Recovery section owners under `frontend-modern/src/components/Recovery/` must
consume that shared `platform` filter surface directly. They must not keep
recovery-local `provider` route/query vocabulary alive behind renamed labels,
or the UI will drift back to backend-shaped navigation even when the copy says
`Platform`.
`frontend-modern/src/utils/problemResourcePresentation.ts` now also belongs to
that same dashboard overview boundary so the problem-resource severity contract
stays shared with `ProblemResourcesTable.tsx` instead of floating as an
unowned helper.
That same dashboard overview boundary must consume the governed Patrol finding
presentation helpers when it surfaces Patrol findings in compact form. In
`frontend-modern/src/features/dashboardOverview/ActionRequiredPanel.tsx`,
Patrol-owned runtime findings must use the shared compact badge, title, and
primary-action/manual-control contracts from
`frontend-modern/src/utils/aiFindingPresentation.ts` rather than rendering raw
`Pulse Patrol:` titles or generic snooze/dismiss controls that the Patrol
runtime lifecycle rejects.
That same boundary must also consume the shared attention-queue ordering from
`frontend-modern/src/utils/aiFindingPresentation.ts` through
`frontend-modern/src/hooks/useDashboardActions.ts`, so Patrol-blinding runtime
issues sort ahead of same-severity infrastructure findings in the dashboard
action panel instead of inheriting arbitrary store order.

Feature-owned alert shells under `frontend-modern/src/features/alerts/` now
also treat shared action runtime as a first-class feature owner instead of
rebuilding it per surface. The overview shell and dashboard recent-alerts panel
must both compose
`frontend-modern/src/features/alerts/useAlertAcknowledgementState.ts` for
acknowledge/restore behavior rather than keeping duplicate API and notification
logic inline in `useAlertOverviewState.ts` or
`frontend-modern/src/components/Alerts/RecentAlertsPanel.tsx`.
The same feature-owner rule now applies to the alert scheduling surface:
`frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx` must remain the
schedule render shell, while
`frontend-modern/src/features/alerts/useAlertScheduleState.ts` owns schedule
reset/update policy and canonical default application. The tab should not
re-accumulate quiet-hours, cooldown, grouping, or escalation mutation logic
inline.
The thresholds editor now follows that same split more tightly:
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`
must stay the table-shell owner for route sync and local UI state, while
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`
stays the composition shell for threshold resource-family projectors,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsRecoveryDefaultsState.ts`
owns backup/snapshot default sanitization and factory-drift policy, and
`frontend-modern/src/features/alerts/thresholds/thresholdsOverrideMutationModel.ts`
owns pure override upsert/hysteresis/state-strip helpers,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsOverrideMutations.ts`
owns threshold-save and backup/snapshot override persistence, and
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsAvailabilityMutations.ts`
owns availability-state policy and alert-removal side effects. The table-shell
hook should not re-accumulate raw override mutation logic,
recovery-threshold defaults policy, or resource-family projection engines
inline.

The updates settings surface now follows the same presentation-owner rule.
`frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx` stays the
top-level settings shell, while
`frontend-modern/src/components/Settings/UpdateInstallGuide.tsx`,
`frontend-modern/src/components/Settings/CopyCommandBlock.tsx`, and
`frontend-modern/src/components/Settings/updatesSettingsModel.ts` plus
`frontend-modern/src/utils/updatesPresentation.ts` own the
deployment-specific install guide, copy-command block, and update-channel/install
model data plus customer-facing update status/action copy. The panel shell must
not rebuild copy-to-clipboard command cards, deployment instruction trees, or
update-surface wording inline.

The reporting operations surface now follows the same shell-state-model rule.
`frontend-modern/src/components/Settings/ReportingPanel.tsx` stays the
operations-panel shell, while
`frontend-modern/src/components/Settings/useReportingPanelState.ts` owns the
license/trial lifecycle and report generation flow,
`frontend-modern/src/components/Settings/reportingPanelModel.ts` plus
`frontend-modern/src/utils/reportingResourceTypes.ts` own the
request/range/filename model and reporting-type API mapping,
`frontend-modern/src/components/Settings/ResourcePicker.tsx` plus
`frontend-modern/src/utils/reportableResourceTypes.ts` own the reportable
resource selection, filter, sort, and empty-state contract, and
`frontend-modern/src/utils/reportingPresentation.ts` owns the user-facing
range/status copy. The compatibility re-export in
`frontend-modern/src/components/Settings/reportingResourceTypes.ts` stays part
of that same reporting boundary. The shell must not re-accumulate license
bootstrapping, inline report API requests, blob-download plumbing, or local
resource-type filter and reporting-token maps.

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

The settings shell now also has an explicit five-way ownership split.
`frontend-modern/src/components/Settings/useDiscoverySettingsState.ts` owns the
shared discovery draft and subnet-validation state,
`frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
owns infrastructure workspace prop assembly and resource-derived infrastructure
read-model shaping for the shell,
`frontend-modern/src/components/Settings/settingsNavigationModel.ts` owns
settings tab identity, canonical route derivation, legacy alias normalization,
and Proxmox agent route metadata. `settingsRouting.ts` and
`settingsTypes.ts` remain thin compatibility re-export shims only, so external
consumers can bridge to the canonical owner without reintroducing a second
settings navigation model. `settingsNavCatalog.ts` owns settings navigation
metadata and item lookup, `settingsNavVisibility.ts` owns
feature/capability visibility and lock policy for settings navigation,
`useSettingsNavigation.ts` owns reactive URL sync and canonical tab-selection
state, `SettingsDialogs.tsx` owns shared settings modal composition,
`useSettingsShellState.ts` owns shell-local sidebar/search/password-modal
state, and `settingsTabSaveBehavior.ts` owns settings tab save-behavior lookup,
`frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx` owns
system panel prop assembly for general, network, updates, and recovery, and
`frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx` owns
registry context assembly for dispatchable settings tabs while
`frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts` owns
the lazy settings panel loader table and route-to-panel import boundary, and
`frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx` owns the
final memoized registry composition only. `frontend-modern/src/components/Settings/Settings.tsx`
must stay a shell that wires those owners together instead of re-accumulating
infrastructure workspace props, registry context maps, system panel prop maps,
lazy loader definitions, or discovery draft state inline.

The resource incident panel's collapsed activity summary is now part of that
same shared primitive boundary. Event-type count chips, visible-event copy,
and the summary-ordering helper in `frontend-modern/src/features/alerts/types.ts`
must stay shared across alert timeline surfaces instead of rebuilding
page-local event summaries or bespoke incident-card markup.

Feature-owned route surfaces under `frontend-modern/src/features/` must also
keep their shell/runtime split explicit once a subsystem grows real transport
or polling lifecycle. The Patrol feature is the current reference shape:
`frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` stays the
feature shell, `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`
owns the runtime state machine, `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
owns the pure investigation-context summary derivation,
`frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` owns canonical AI
summary normalization at the shared store boundary, and the Patrol-owned
header/banner/summary/workspace section files under
`frontend-modern/src/features/patrol/` own the heavy render surfaces. Shared
shell governance should reinforce that pattern instead of letting feature render
surfaces re-accumulate API and timer orchestration inline.
That same route-owned page-health rule now also applies to Patrol: a feature
surface may not present a green or all-clear primary summary when the owning
runtime contract says the page is blocked or unavailable, even if the last
successful snapshot was healthy.
That same rule also applies to compact Patrol summary fragments inside the
feature surface: count-only strips or metric cards must not emit `No issues
found` or other reassuring copy when the owning overall-health summary is
degraded or not fully verified.
That same summary shell should also surface verification scope from the
owning run-history contract. Operators should be able to see, inside the same
summary surface, whether Patrol recently completed a full verification pass or
whether recent activity was limited to scoped/erroring patrol runs.

Shared primitive consumers that split status-dot tone and status-text tone
must now keep both values routed through the same exported presentation helper.
Feature cards such as RAID status may not call shadow local aliases that drift
from the canonical shared class/variant helpers.

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
That same settings-shell framing must stay in customer language. Shared headers
and descriptions should talk about monitored-system limits, plan limits, and
subscription or license status instead of reviving legacy `installed-agent`
terms or vague internal nouns like `allocation`.
That banner boundary now also owns the canonical monitored-system naming
surface directly: the shared warning component path and exported symbol are
`MonitoredSystemLimitWarningBanner`, and future work may not reintroduce an
agent-era banner filename or component name as the primary primitive.
That shared monitored-system warning banner now also follows the shell/runtime/model
owner split. `frontend-modern/src/components/shared/MonitoredSystemLimitWarningBanner.tsx`
stays the render shell, `frontend-modern/src/components/shared/useMonitoredSystemLimitWarningBannerState.ts`
owns entitlement load, warning metric emission, migration/upgrade click tracking,
and upgrade-link runtime, and
`frontend-modern/src/components/shared/monitoredSystemLimitWarningBannerModel.ts`
owns monitored-system warning policy, count aggregation, and tone/text-class
policy while sourcing customer-facing monitored-system copy from the canonical
`frontend-modern/src/utils/monitoredSystemPresentation.ts` helper. Future
warning-banner work should extend those owners instead of pushing entitlement
orchestration, tracking, or naming math back into the shared shell or
reintroducing banner-local monitored-system copy strings.
Shared frontend label-formatting helpers now also have an explicit owner here.
`frontend-modern/src/utils/textPresentation.ts` is the canonical shared owner
for token humanization, identifier label formatting, title-casing, and
arrow-delimited label presentation used across AI, Patrol, Storage/Recovery,
and other feature surfaces. Feature contracts may depend on that helper, but
they should not re-home or fork those generic text-formatting rules into
feature-local utilities.
That same shared presentation boundary now also owns operator feedback and
shared table-label semantics. `frontend-modern/src/components/Toast/Toast.tsx`
stays the render shell for the global toast stack,
`frontend-modern/src/utils/toast.ts` owns the app-level trigger helper,
`frontend-modern/src/utils/semanticTonePresentation.ts` owns canonical toast
and diagnostics tone classes, `frontend-modern/src/utils/emptyStatePresentation.ts`
owns the shared empty-state tone styling consumed by `EmptyState`, and
`frontend-modern/src/utils/typeColumnPresentation.ts` owns the single
canonical type-column label used across dashboard and alert tables. Future
feedback, empty-state, or shared type-column work should extend those helpers
instead of reintroducing panel-local tone classes, app-local toast wiring, or
copy drift between tables.
First-session educational surfaces must also stay brief, flat, and model-led.
When Pulse needs to teach a user how a flow works, the primary on-screen
guidance should collapse to a few short descriptions of the real product
mental model instead of a logo wall, feature brochure, or verbose internal
mechanics dump. The runtime wizard itself now stays on the two-step
`Welcome -> Security` path, while the separate setup-completion preview owns
the brief three-step explanation: install the Unified Agent, get the first
Pulse resource, then layer on additional context.

The settings shell is now also a governed frontend primitive boundary.
`frontend-modern/src/utils/settingsShellPresentation.ts` now owns the
customer-facing settings-shell framing copy for navigation, search, loading,
and unsaved-change banners so `SettingsPageShell.tsx` stays a render shell
instead of re-accumulating product wording inline.

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
is now explicitly a shell consumer rather than the data or controller owner,
and the tab render owners live in
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTablePMGTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`, and
`frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`.
The Proxmox tab is itself now a shell that composes
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxNodesSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxPBSSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestFilteringSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx`,
and `frontend-modern/src/components/Alerts/ThresholdsTableProxmoxStorageSection.tsx`
using the shared contract in
`frontend-modern/src/features/alerts/thresholds/thresholdsTableSectionProps.ts`.
Future Proxmox thresholds presentation changes should extend those section
surfaces rather than restoring mixed JSX ownership to
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxTab.tsx`.
The Docker tab now follows that same composition pattern through
`frontend-modern/src/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerServiceGapSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerHostsSection.tsx`,
and `frontend-modern/src/components/Alerts/ThresholdsTableDockerContainersSection.tsx`.
Future Docker thresholds presentation changes should extend those section
surfaces rather than restoring mixed JSX ownership to
`frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`.
The agents tab now follows that same composition pattern through
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx`
and `frontend-modern/src/components/Alerts/ThresholdsTableAgentDisksSection.tsx`.
Future agent thresholds presentation changes should extend those section
surfaces rather than restoring mixed JSX ownership to
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`.
The thresholds tab adapter contract now lives in
`frontend-modern/src/features/alerts/thresholds/thresholdsTabModel.ts`, so
`frontend-modern/src/features/alerts/tabs/ThresholdsTab.tsx` stays a thin shell
instead of carrying a duplicate table adapter contract inline.
Canonical threshold row shaping now routes through
`frontend-modern/src/features/alerts/thresholds/thresholdsResourceModel.ts`
plus the family-owned feature hooks
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsHostData.ts`,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsDockerData.ts`,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsGuestData.ts`,
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsInfrastructureData.ts`,
with `frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsData.ts`
limited to composing them. Thresholds-table controller state lives in
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`,
so future cleanup should extend those feature hooks or tab owners instead of
rebuilding resource normalization, tab render surfaces, or thresholds-table
runtime state inside the shell component.

The alerts page now also applies the same shell-versus-feature rule to
configuration orchestration. `frontend-modern/src/pages/Alerts.tsx` is the page
shell, while `frontend-modern/src/features/alerts/AlertsConfigurationSurface.tsx`
is the feature shell. The canonical runtime owner is now
`frontend-modern/src/features/alerts/useAlertsConfigurationState.ts` for alert
config transport and org-switch reload orchestration,
`frontend-modern/src/features/alerts/useAlertsConfigurationSnapshotState.ts`
for the default-backed mutable configuration snapshot plus apply/capture/reset
ownership,
`frontend-modern/src/features/alerts/alertsConfigurationModel.ts` for config
normalization, factory defaults, docker-gap validation, and payload
serialization, `frontend-modern/src/features/alerts/alertOverridesModel.ts`
for override normalization and resource-backed projection, and
`frontend-modern/src/features/alerts/useAlertOverridesState.ts`
for reactive override state and thresholds-facing resource selectors, and
`frontend-modern/src/features/alerts/alertDestinationsModel.ts` for
destination config normalization and payload shaping, and
`frontend-modern/src/features/alerts/useAlertDestinationsState.ts` for
notification destination reload and persistence orchestration.
`frontend-modern/src/features/alerts/useAlertWebhookDestinationsState.ts` now
owns webhook runtime, and
`frontend-modern/src/components/Alerts/ResourceTable.tsx` now follows the same
shell rule: the shell only picks desktop vs mobile render ownership and bulk-edit
composition, while
`frontend-modern/src/components/Alerts/AlertResourceTableDesktop.tsx`,
`frontend-modern/src/components/Alerts/AlertResourceTableMobile.tsx`, and
`frontend-modern/src/components/Alerts/AlertResourceGroupHeader.tsx` own the
render-heavy table/card/group-header surfaces. Shared runtime state remains in
`frontend-modern/src/components/Alerts/useAlertResourceTableState.ts`, shared row
rendering remains in
`frontend-modern/src/components/Alerts/AlertResourceTableRow.tsx`, and shared
metric normalization remains in
`frontend-modern/src/components/Alerts/alertResourceTableModel.ts`.
`frontend-modern/src/features/alerts/useAlertDestinationsTabState.ts` now owns
destination test actions plus retry orchestration while
`frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx` stays the
render shell and should compose the dedicated email, Apprise, webhook, and
load/error section owners instead of carrying those panels inline. Future
cleanup should extend the transport hook, config model, override hook, or
destinations runtime hook based on the true owner, not move config control
flow back into the top-level page shell.
The alert email provider picker now also follows the shell/runtime split:
`frontend-modern/src/components/Alerts/useEmailProviderSelectState.ts` owns
provider-catalog loading and provider-default application, while
`frontend-modern/src/components/Alerts/EmailProviderSelect.tsx` stays the
render shell and should not re-accumulate `NotificationsAPI.getEmailProviders`
or a second local email-config contract inline.
The alert scheduling surface now follows the same shell-versus-section split:
`frontend-modern/src/features/alerts/tabs/ScheduleTab.tsx` should compose the
dedicated quiet-hours, cooldown, grouping, recovery, escalation, and summary
section owners while `frontend-modern/src/features/alerts/useAlertScheduleState.ts`
remains the canonical runtime owner.
The same rule now also covers cross-tab incident timelines: the shared runtime
owner is `frontend-modern/src/features/alerts/useAlertIncidentTimelineState.ts`,
while `frontend-modern/src/features/alerts/OverviewTab.tsx` and
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` stay focused on
surface composition. Future incident timeline fetch, note-save, or expansion
control flow should extend that feature hook rather than forking back into
either tab surface.
Overview alert runtime now follows that same shell-versus-runtime split. The
shell stays in `frontend-modern/src/features/alerts/OverviewTab.tsx`, while
`frontend-modern/src/features/alerts/useAlertOverviewState.ts` owns derived
alert stats, filtered ordering, and single/bulk acknowledge runtime behavior.
Future overview control flow should extend that hook rather than restoring
action timers or acknowledge mutations to the tab shell.
Render-heavy overview ownership now lives in
`frontend-modern/src/features/alerts/AlertOverviewStatsCards.tsx`,
`frontend-modern/src/features/alerts/AlertOverviewActiveAlertsSection.tsx`,
and `frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx`, so
future card-list or timeline-card presentation work should extend those
surfaces rather than expanding `frontend-modern/src/features/alerts/OverviewTab.tsx`
back into a mixed shell.
Alert history runtime now follows that same pattern. The shell stays in
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`, while
`frontend-modern/src/features/alerts/useAlertHistoryState.ts` owns history
fetch, persistent filters, history-clear behavior, and composition of the
derived history owners. Resource-incident panel runtime now lives in
`frontend-modern/src/features/alerts/useAlertResourceIncidentsState.ts`, while
`frontend-modern/src/features/alerts/alertHistoryModel.ts` owns grouped/trend
derivation and the bucket/range analytics contract. The render-heavy surfaces
now route through
`frontend-modern/src/features/alerts/AlertHistoryFrequencyCard.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryFiltersCard.tsx`,
`frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableSection.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableGroupRow.tsx`,
`frontend-modern/src/features/alerts/AlertHistoryTableAlertRow.tsx`, and
`frontend-modern/src/features/alerts/AlertHistoryAdministrationCard.tsx`.
Future alert-history control flow should extend the hook, pure history analytics
should extend the model, and section rendering should extend those owners
rather than rebuilding any of those concerns in the tab shell.
Top-level settings surfaces must route through `Settings.tsx`,
`SettingsPageShell.tsx`, and
`frontend-modern/src/components/shared/SettingsPanel.tsx` instead of
reintroducing bespoke outer page headers or one-off top-level panel framing.
The shell metadata driving those surfaces is part of the same boundary as
well: `frontend-modern/src/components/Settings/settingsHeaderMeta.ts` and
representative top-level panels such as
`frontend-modern/src/components/Settings/APIAccessPanel.tsx`,
`frontend-modern/src/components/Settings/AISettings.tsx`,
`frontend-modern/src/components/Settings/AIModelSelectionSection.tsx`,
`frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`,
`frontend-modern/src/components/Settings/AIChatMaintenanceSection.tsx`,
`frontend-modern/src/components/Settings/AISettingsStatusAndActions.tsx`,
`frontend-modern/src/components/Settings/AIProviderConfigurationSection.tsx`,
`frontend-modern/src/components/Settings/AISettingsDialogs.tsx`, and
`frontend-modern/src/components/Settings/aiSettingsModel.ts` now also define
the canonical AI settings runtime boundary. `AISettings.tsx` is the shell,
`frontend-modern/src/components/Settings/useAISettingsState.ts` owns the
runtime lifecycle and persistence flow, model/provider setup now routes
through `AIModelSelectionSection.tsx`, discovery, budget, timeout, and
permission controls route through `AIRuntimeControlsSection.tsx`, chat
maintenance routes through `AIChatMaintenanceSection.tsx`, and readiness plus
save/test actions route through `AISettingsStatusAndActions.tsx`. Future AI
settings work must extend those section owners instead of re-inlining large
runtime subsections into the shell.
That same AI settings boundary now also owns
`frontend-modern/src/utils/aiSettingsPresentation.ts`, so shared loading,
empty, OAuth, and action/error copy for the settings shell stays on one
governed helper instead of drifting back into section-local strings.
`frontend-modern/src/components/Settings/AuditLogPanel.tsx`,
`frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`,
`frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/NetworkDiscoverySection.tsx`,
`frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`,
`frontend-modern/src/components/Settings/networkSettingsModel.ts`,
`frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`,
`frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`,
`frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`,
`frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`,
`frontend-modern/src/components/Settings/useSSOProvidersState.ts`, and
`frontend-modern/src/components/Settings/ssoProvidersModel.ts` now also define
the canonical SSO provider settings runtime boundary: `SSOProvidersPanel.tsx`
is the shell, `useSSOProvidersState.ts` owns the reactive/API lifecycle, and
`ssoProvidersModel.ts` owns provider-form normalization and payload building.
`frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx` must keep
page-shell titles, descriptions, and lead panel framing aligned instead of
letting navigation/header labels drift away from the actual settings surface.
`frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx` is now a
shell only. `frontend-modern/src/components/Settings/NetworkDiscoverySection.tsx`
owns discovery controls and shared subnet presets, while
`frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
owns the public URL, CORS, embedding, and webhook-boundary UI. Shared prop
contracts for that surface must extend
`frontend-modern/src/components/Settings/networkSettingsModel.ts` instead of
re-expanding the shell or reintroducing page-local section types.
`frontend-modern/src/utils/discoveryPresentation.ts` now owns the
customer-facing discovery-section framing copy, scan-scope labels, subnet
guidance, and environment-lock messaging so
`frontend-modern/src/components/Settings/NetworkDiscoverySection.tsx` stays a
settings section shell instead of re-accumulating that wording inline.
That same settings-shell boundary now also owns the shared settings
presentation helpers that those panels consume. `frontend-modern/src/utils/systemSettingsPresentation.ts`
is the canonical owner for shared system-settings presets, summaries, and
customer-facing action copy, while
`frontend-modern/src/utils/ssoProviderPresentation.ts` owns the shared SSO
provider labels, empty states, and action/status messaging. Future settings
copy changes in those areas should extend these helpers instead of inlining
panel-local strings inside the shell or reactive state owners.
That same shared primitive boundary now also owns environment-lock
presentation. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
stays the reusable badge shell,
`frontend-modern/src/utils/environmentLockPresentation.ts` owns the canonical
badge label, title, and lock-button copy, and
`frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx` stays
the settings-shell consumer for environment-variable-locked container-update
controls. Future environment-lock UX should extend those owners instead of
reintroducing panel-local lock labels, badge styling, or title copy.
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
