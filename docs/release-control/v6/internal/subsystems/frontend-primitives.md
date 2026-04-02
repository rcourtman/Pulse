# Frontend Primitives Contract

## Contract Metadata

```json
{
  "subsystem_id": "frontend-primitives",
  "lane": "L8",
  "contract_file": "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["agent-lifecycle", "storage-recovery"]
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
25. `frontend-modern/src/utils/discoveryPresentation.ts`
26. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
27. `frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`
28. `frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`
29. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
30. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`
31. `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`
32. `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
33. `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`
34. `frontend-modern/src/components/Settings/useAISettingsState.ts`
35. `frontend-modern/src/components/Settings/useDiagnosticsPanelState.ts`
36. `frontend-modern/src/components/Settings/useSettingsShellState.ts`
37. `frontend-modern/src/components/Settings/useSSOProvidersState.ts`
38. `frontend-modern/src/components/Settings/ssoProvidersModel.ts`
39. `frontend-modern/src/utils/ssoProviderPresentation.ts`
40. `frontend-modern/src/utils/systemSettingsPresentation.ts`
41. `frontend-modern/src/utils/aiSettingsPresentation.ts`
42. `frontend-modern/src/utils/settingsShellPresentation.ts`
43. `frontend-modern/src/utils/textPresentation.ts`
44. `frontend-modern/src/components/Settings/UpdateInstallGuide.tsx`
45. `frontend-modern/src/components/Settings/updatesSettingsModel.ts`
46. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
47. `frontend-modern/src/components/Settings/ReportingPanel.tsx`
48. `frontend-modern/src/components/Settings/reportingPanelModel.ts`
49. `frontend-modern/src/components/Settings/reportingInventoryExportModel.ts`
50. `frontend-modern/src/components/Settings/useReportingPanelState.ts`
51. `frontend-modern/src/utils/reportingPresentation.ts`
52. `frontend-modern/src/utils/updatesPresentation.ts`
53. `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
54. `tests/integration/tests/15-settings-shell-consistency.spec.ts`
55. `frontend-modern/src/components/shared/PageControls.guardrails.test.ts`
56. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
57. `frontend-modern/src/features/`
58. `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
59. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
60. `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
61. `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
62. `frontend-modern/src/components/SetupWizard/__tests__/SetupWizard.test.tsx`
63. `frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx`
64. `frontend-modern/src/components/SetupWizard/__tests__/WelcomeStep.test.tsx`
65. `frontend-modern/src/components/shared/MonitoredSystemLimitWarningBanner.tsx`
66. `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
67. `frontend-modern/src/components/Settings/useSystemLogsPanelState.ts`
68. `frontend-modern/src/utils/systemLogsPresentation.ts`
69. `frontend-modern/src/components/Settings/__tests__/SystemLogsPanel.test.tsx`
70. `frontend-modern/src/features/operations/OperationsPageSurface.tsx`
71. `frontend-modern/src/features/operations/operationsPageModel.ts`
72. `frontend-modern/src/pages/Operations.tsx`
73. `frontend-modern/src/components/Settings/ResourcePicker.tsx`
74. `frontend-modern/src/components/Settings/reportingResourceTypes.ts`
75. `frontend-modern/src/utils/reportableResourceTypes.ts`
76. `frontend-modern/src/utils/reportingResourceTypes.ts`
77. `frontend-modern/src/utils/problemResourcePresentation.ts`
78. `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`
79. `frontend-modern/src/utils/dashboardGuestPresentation.ts`
80. `frontend-modern/src/utils/dashboardKpiPresentation.ts`
81. `frontend-modern/src/utils/dashboardTrendPresentation.ts`
82. `frontend-modern/src/components/Toast/Toast.tsx`
83. `frontend-modern/src/utils/toast.ts`
84. `frontend-modern/src/utils/semanticTonePresentation.ts`
85. `frontend-modern/src/utils/emptyStatePresentation.ts`
86. `frontend-modern/src/utils/typeColumnPresentation.ts`
87. `frontend-modern/src/pages/__tests__/Operations.helpers.test.ts`
88. `frontend-modern/src/components/Settings/NetworkDiscoverySection.tsx`
89. `frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
90. `frontend-modern/src/components/Settings/networkSettingsModel.ts`
91. `frontend-modern/src/components/Settings/useDiscoverySettingsState.ts`
92. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
93. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
94. `frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts`
95. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
96. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`
97. `frontend-modern/src/components/Settings/settingsNavVisibility.ts`
98. `frontend-modern/src/components/Settings/settingsRouting.ts`
99. `frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts`
100. `frontend-modern/src/components/Settings/settingsTypes.ts`
101. `frontend-modern/src/components/Settings/useSettingsNavigation.ts`
102. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
103. `frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx`
104. `frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx`
105. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
106. `frontend-modern/src/utils/environmentLockPresentation.ts`
107. `frontend-modern/src/utils/docsLinks.ts`
108. `tests/integration/tests/20-local-doc-links.spec.ts`
109. `frontend-modern/src/index.css`
110. `frontend-modern/src/components/shared/SummaryScopeBar.tsx`
111. `frontend-modern/src/components/shared/summaryScopePresentation.ts`
112. `frontend-modern/src/components/shared/summaryInteractionA11y.ts`
113. `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`

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
5. Keep shared platform-connections shell state on the reusable settings boundary: `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`, `frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`, and `frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx` must continue to derive provider counts, availability, and shared subtab copy from one infrastructure-settings source instead of creating provider-local summary fetches or VMware-only shell vocabulary.
6. Keep shared storage feature presenters on canonical platform truth. When reusable storage presenters under `frontend-modern/src/features/storageBackups/` classify canonical resources for the shared storage route, API-backed virtualization datastores such as VMware must stay inventory-only datastores instead of inheriting PBS-specific backup-repository or protected-target copy from older fallback branches.
7. Keep top-of-page summary interaction on shared primitives. Infrastructure, workloads, and storage summary cards must route sticky-shell behavior through `frontend-modern/src/components/shared/StickySummarySection.tsx` and route row-hover or focused-series rendering through shared chart primitives such as `frontend-modern/src/components/shared/InteractiveSparkline.tsx` and `frontend-modern/src/components/shared/DensityMap.tsx`, rather than page-local sticky wrappers or metric-card-specific hover logic. The shared summary-card contract must also own stable summary-card geometry for chart-backed cards so row hover, focus, synchronized readouts, or idle header metadata cannot ratchet the sticky summary taller across rerenders.
8. Keep summary chart interaction identity on one shared helper. Summary surfaces that expose row-hover, group-hover, chart-hover, or route-focus-driven chart emphasis must derive page/group/entity scope through `frontend-modern/src/components/shared/summaryCardInteraction.ts` and pass that same resolved scope into card-state, sparkline, and density-map primitives, rather than letting cards read `hovered || focused` while charts listen to a different page-local ID source. Hovering one summary chart must promote that series into the shared active entity so sibling cards highlight the same object instead of keeping chart-local hover islands, and hovering or pinning a workload group header, infrastructure cluster header, or storage pool-group header must scope the matching summary cards through that same shared contract instead of forking a page-local summary filter path. Sibling cards should surface that synchronized hover as one compact header readout through the shared summary-card contract, while the chart under the pointer keeps the only floating tooltip. `frontend-modern/src/components/Recovery/RecoverySummary.tsx` is explicitly outside this interaction dialect: recovery posture cards may share summary framing, but they must not silently grow row/group/chart hover behavior without a separate governed product decision.
9. Keep page summaries page-scoped when table rows enter contextual focus. Route-backed row selection may add a focused label and shared series emphasis, but infrastructure, workloads, and storage summary cards must continue to render the page-level series set instead of collapsing the summary down to the selected row or replacing the global trend view with row-local empty states.
10. Keep contextual row focus on the shared summary primitive. Summary surfaces and same-route table drill-ins must reuse `frontend-modern/src/components/shared/contextualFocus.ts` for interactive-series filtering, focused-name lookup, active-series derivation, local scroll preservation, and deliberate inline-detail reveal instead of rebuilding page-local `Set` filters, focused-label scans, drawer-aware scroll math, or ad hoc scroll restoration in each surface.
11. Keep summary-to-table coordination deliberate, explicit, and reversible. Shared summary hover may highlight the matching table row when it is already visible, but transient chart hover must not auto-filter tables, auto-scroll the page, or reshuffle table ordering. When a page/group/entity scope is active on workloads, infrastructure, or storage, the page shell must surface that state through the shared `frontend-modern/src/components/shared/SummaryScopeBar.tsx` plus `frontend-modern/src/components/shared/summaryScopePresentation.ts` contract instead of inventing page-local chips, breadcrumbs, or hidden route-only focus. That shared scope presenter must read like native page context, not a pill/badge widget: it should distinguish transient preview from pinned focus with quiet inline language, keep the current scope visible even when the sticky summary is collapsed or off-screen, and expose a clear reset path for pinned scope so touch-only operators do not depend on hover or “click the same row again” discovery. When the active row is off-screen, page owners must still route through `frontend-modern/src/components/shared/summaryTableFocus.ts` and surface a lightweight `Jump to row` affordance that reveals and scrolls only on explicit user action. Deliberate row focus may reveal inline detail automatically, but that reveal must be drawer-aware: preserve already-good positions, avoid hard centering, and scroll only enough to keep the row header plus the top of the inline detail visible.
    Shared summary-linked rows and group headers must also route their preview
    semantics through
    `frontend-modern/src/components/shared/summaryInteractionA11y.ts`.
    Leaf rows and any explicit row-level control chrome must route deliberate
    pin/open ownership through
    `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`, so
    `aria-expanded`, `aria-controls`, `aria-pressed`, focus treatment, and
    `Escape` preview clearing stay on the shared control instead of focusable-
    table-row shims. Group headers are different: they may use the header row
    itself as the deliberate pin target when that keeps the table chrome
    native, but they must not grow separate scope/pinned pill buttons beyond
    the shared `SummaryScopeBar.tsx` reset path. Workloads, infrastructure,
    and storage must not rebuild row-as-button keyboard handling or trailing
    one-off expand columns once the shared action primitive exists.
12. Keep summary-linked table row emphasis on the shared primitive contract. Workloads, infrastructure, and storage rows that mirror the active summary entity must expose that state through `data-summary-row-active` and let the shared presentation in `frontend-modern/src/index.css` render the row emphasis, rather than carrying page-local sky or blue fill classes inside each row renderer.
13. Keep retained-value data loading honest at the ownership boundary. Helpers
    that prevent a feature surface from falling through the app-level Suspense
    boundary during in-flight refresh belong under that feature's
    `frontend-modern/src/features/` owner until multiple surfaces truly share
    the behavior; do not promote them into `components/shared/` or a generic
    global hook namespace just because more than one local hook calls them
    inside the same feature area.

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
5. Keep shared feature-level presenters on capability truth. When reusable
   presenters under `frontend-modern/src/features/` explain why a control,
   chart, or detail surface is unavailable, they must describe the owned
   identity or capability gap instead of prescribing a provider-local install
   path that conflicts with API-backed platforms like TrueNAS.
6. When a settings route header and a top-level settings shell describe the same
   commercial surface, keep them on the same shared presentation owner instead
   of allowing route metadata in `settingsHeaderMeta.ts` or labels in
   `settingsNavCatalog.ts` to drift into independent title or description copy,
   and keep adjacent settings-shell referrals such as
   `InfrastructureWorkspace.tsx` on that same shared owner instead of
   reintroducing local “go to Pulse Pro” variants.
7. Keep hosted settings-shell framing imports safe for bundle initialization.
   Self-hosted billing titles, descriptions, and referral copy used by
   `settingsHeaderMeta.ts`, `settingsNavCatalog.ts`, and adjacent settings
   shells must flow through
   `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
   instead of importing generic commercial presentation helpers directly into
   hosted settings route shells.
8. Keep first-session dashboard empty-state copy on
   `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`, and make
   infrastructure setup guidance name the canonical destination explicitly
   instead of falling back to generic settings CTA labels.
9. Keep the live first-session wizard on the canonical three-step runtime
   shape in `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
   (`Welcome`, `Security`, then `Install`), and keep the step indicator plus
   completion CTA language aligned with the governed infrastructure install
   workspace instead of regressing to a route jump that leaves the next action
   implicit.
10. Keep shared filter primitives coherent with route-owned option hydration.
    Feature shells such as `frontend-modern/src/features/infrastructure/`
    must keep a route-owned canonical option visible in shared selects like
    `LabeledFilterSelect` even when current results do not contain that
    option, so provider-scoped handoffs do not flash back to `All`.
11. Keep the first welcome screen in
    `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
    explicit about operator context. The shell must explain that the bootstrap
    token only unlocks first-run setup, state where the command should run, and
    adapt command/help text to detected Docker or containerized deployments
    instead of assuming the operator already knows which host or container owns
    the Pulse install.
12. Keep the settings-shell infrastructure landing path aligned with that same
    first-session story. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
    must treat `/settings` and the infrastructure settings tab as the canonical
    path to `/settings/infrastructure/install`, not to reporting/control, so
    the shell does not send first-time operators to the wrong infrastructure
    subview by default.
13. Keep dashboard onboarding copy on the shared presentation owner in
    `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`. Both the
    infrastructure empty state and the dashboard route's no-resources state
    must name the canonical install workspace explicitly, keep `Platform
connections` visible as the API-backed alternative for Proxmox and
    TrueNAS, and expose the same first-host next step instead of falling back
    to passive “nothing here yet” wording.
14. Keep cross-surface investigation handoffs on shared route ownership.
    Feature shells such as Alerts and Patrol may decide which governed
    destination chips to render, but canonical href, label, dedupe, and
    infrastructure-fallback truth must stay in
    `frontend-modern/src/routing/resourceLinks.ts` instead of freezing raw
    route strings or provider-local link builders inside feature panels.
15. Keep shared summary-card emphasis coherent. When shared summary primitives enter an `inactive` state, `SummaryMetricCard`, `InteractiveSparkline`, and `DensityMap` must all demote background context together so storage, infrastructure, and workloads read as one interaction model instead of mixing page-local opacity, sticky-shell, or highlight rules.
16. Keep density-map summaries overview-first. When a shared summary density map receives row focus or chart-hover emphasis, `frontend-modern/src/components/shared/DensityMap.tsx`, `frontend-modern/src/components/shared/useDensityMapState.ts`, and `frontend-modern/src/components/shared/densityMapModel.ts` must preserve the multi-entity overview rows and keep focused-entity detail in the hover tooltip instead of swapping the card into a single-series chart, dimming the rest of the map into unusable background noise, duplicating cursor-value tooltip copy, or adding persistent card chrome that steals heatmap space. The card body must stay overview-first; the tooltip may carry the active entity identity, current value, and peak, shared tooltip shells must follow semantic surface tokens instead of forcing a dark palette in light mode, the tooltip header must let long entity names consume the available width before truncating rather than clipping against an arbitrary fixed label cap, numeric metric readouts such as `16.9 MB/s` or `37.4 MB/s` must stay single-line instead of wrapping the unit onto a second row, and density-map detail that cannot fit cleanly inside the canonical tooltip shell must be omitted rather than introducing tooltip-specific chrome or a secondary chart inside the hover surface.
17. Keep sparkline scrubbing source-local and sibling-sync timestamp-based. The chart a user is actively scrubbing in `frontend-modern/src/components/shared/InteractiveSparkline.tsx` and `frontend-modern/src/components/shared/useInteractiveSparklineState.ts` must keep its dashed hover cursor on the real local mouse `x`, while sibling cards may map the shared hover timestamp onto their own timelines. Shared cursor sync must not snap the source chart back onto the nearest sample timestamp, the rendered SVG/canvas hover cursor must bind to the actual numeric cursor coordinate rather than a boolean guard state, the time cursor must span the chart viewport instead of collapsing to the series height, and the hover tooltip must track the pointer instead of anchoring to the chart top edge while following the active theme rather than a hardcoded dark shell.
18. Keep shared contextual focus canonical after adoption. Once a summary or table surface enters route-backed contextual focus, future additions must extend `frontend-modern/src/components/shared/contextualFocus.ts` and its guardrail tests rather than forking another helper for workload IDs, resource IDs, or scroll-preserving same-route selection.
19. Keep shared infrastructure/resource selectors on the canonical agent-facet
    truth. Shared primitives and settings-facing selector helpers must treat
    top-level TrueNAS appliances as agent-facet infrastructure via shared
    helper ownership instead of reviving a direct `resource.type === 'truenas'`
    branch inside page shells, selectors, or reporting-resource type helpers.
20. Keep shared feature-shell Patrol run fixtures on the canonical run-record
    contract. When `frontend-modern/src/features/patrol/` consumes Patrol run
    history, the shared normalized record must preserve provider-backed counts
    such as `truenas_checked` instead of letting feature-local fixtures or
    fallback objects collapse API-backed TrueNAS systems back into generic
    agent-host presentation.
21. Keep the authenticated app root aligned with that same first-session path.
    That same shared-primitive ownership now includes contextual row focus.
    `frontend-modern/src/components/shared/contextualFocus.ts` is the canonical
    owner for interactive-series filtering, focused-label lookup, active-series
    resolution, and nearest-scrollable-ancestor preservation across page-scoped
    summary surfaces. Dashboard row focus, infrastructure summary emphasis,
    storage summary emphasis, and workloads summary emphasis must all route through
    that helper instead of maintaining page-local copies of the same hover/focus
    rules.
    `frontend-modern/src/App.tsx` must land `/` on the dashboard shell and let
    the governed dashboard empty state route first-time operators into
    Infrastructure Install, instead of preserving a separate root-only jump to
    `/infrastructure` that drifts from the rest of the onboarding contract.
22. Keep relay settings shell copy on the shared presentation owner in
    `frontend-modern/src/utils/relayPresentation.ts`. The route metadata in
    `settingsHeaderMeta.ts` and the leading `SettingsPanel` in
    `RelaySettingsPanel.tsx` must reuse the same description and availability
    copy instead of drifting into separate rollout or pairing wording.
23. Keep shared settings-shell legal and docs referrals on
    `frontend-modern/src/utils/docsLinks.ts`. Shared settings surfaces such as
    `AIRuntimeControlsSection.tsx` must not hardcode GitHub `main` doc URLs for
    privacy, security, proxy-auth, scope-reference, or Terms-of-Service links.
24. Keep shared settings-shell telemetry transparency controls on the governed
    general settings panel. Preview/reset affordances for anonymous telemetry
    must stay rendered inside
    `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
    instead of drifting into route-local modals, hidden dev tools, or shell
    chrome that operators would not naturally inspect.
25. Keep the short telemetry/privacy summary copy on that same shared surface
    accurate to the governed privacy doc. If the trust boundary depends on a
    specific retention window or on “IP addresses are not stored” rather than
    “IPs are never seen,” the summary copy in
    `GeneralSettingsPanel.tsx` must state those facts plainly instead of
    reverting to a stronger but inaccurate shorthand.
26. Keep shared storage-route feature presentation on neutral capability truth.
    Reusable mappers and presenters in `frontend-modern/src/features/storageBackups/`
    must distinguish inventory datastores from backup repositories so VMware
    rows on the shared storage route stay canonical to the admitted phase-1 floor instead of
    reviving backup-target, protected-target, or recovery-local semantics on a
    shared page.
27. Keep infrastructure settings-shell API alternatives on the shared shell
    contract. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
    `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
    `frontend-modern/src/components/Settings/settingsNavigationModel.ts`, and
    shared empty-state/setup guidance must
    present `Platform connections` as the canonical API-backed alternative for
    Proxmox, TrueNAS, and future provider integrations instead of reviving
    top-level `Direct Proxmox` wording or shell-local provider routes.
28. Keep the infrastructure settings platform-connections summary and provider
    workspaces on one shared state source. `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`,
    `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`,
    `frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`,
    `frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx`, and `frontend-modern/src/components/Settings/TrueNASSettingsPanel.tsx` must
    derive TrueNAS connection counts and availability from the shared
    infrastructure settings state instead of letting the reporting summary and
    the provider-specific panel issue separate connection fetches.
29. Keep alert-history feature composition on the current owned state contract.
    `frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` must react to the
    shared `alertData()` history state instead of reviving deleted aliases, and
    it must pass unified-resource resolution through to
    `frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx` so
    the panel can render shared route chips without creating another page-local
    resource lookup or provider-specific handoff layer.
30. Keep the alert-thresholds containers surface on the canonical shared owner.
    `alertOverridesModel.ts`, `useAlertOverridesState.ts`, and
    `useAlertsConfigurationState.ts` must surface API-backed `app-container`
    parents such as TrueNAS as first-class `Container Runtimes`, while
    `ThresholdsTab.tsx` must bridge function-valued selectors into
    `ThresholdsTable.tsx` explicitly instead of relying on spread-based adapter
    props that can collapse functions on the live Solid surface. Docker-only
    controls in `ThresholdsTableDockerTab.tsx` must remain gated to real
    `docker-host` resources instead of leaking onto platform-managed runtimes.

## Current State

The frontend already has several guardrail tests. The next step is to keep
turning repeated local patterns into explicit shared primitives with hard usage
bounds, including provider-backed alert-history wording. `frontend-modern/src/features/alerts/helpers.ts`,
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`, and
`frontend-modern/src/features/alerts/OverviewTab.tsx` must present VMware-
backed host and VM incidents with the shared `resource-incident` vocabulary
and existing alert-history shells instead of introducing VMware-only labels,
badges, or panel copy just because the underlying signal came from vSphere.
Storage disk drawers now also sit on that same shared-primitives floor.
`frontend-modern/src/components/Storage/DiskDetail.tsx` must render physical-
disk read, write, and busy charts through `HistoryChart` plus
`useHistoryChartState`, using the canonical physical-disk history resource id,
instead of reviving `diskMetricsHistory`, a page-local ring buffer, or another
storage-only live chart primitive for the same telemetry.

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
That same settings-shell boundary now also owns operator-facing docs referrals
for governed security panels. `APIAccessPanel.tsx` and
`SecurityOverviewPanel.tsx` must route scope and proxy-auth guidance through
the shared shipped-doc helper in `frontend-modern/src/utils/docsLinks.ts`
instead of hardcoding GitHub `main` URLs that can drift from the running
build, and `tests/integration/tests/20-local-doc-links.spec.ts` must keep
browser proof on those settings-shell surfaces.
That same settings-shell boundary now also owns the remediation framing for
Security Overview itself. `SecurityOverviewPanel.tsx` may not stop at a score
card and static best-practices copy once low-risk security debt has been
demoted out of the global banner; it must render explicit next-step hardening
actions on the canonical settings shell, source those actions from the shared
security presentation owner, and keep direct operator links pointed at the
owning auth, API-access, or shipped security-guide surface. The canonical
proof for that shell framing remains
`frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`.
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
That same reporting shell must also route failed catalog/report/export
responses through the shared API error extractor in `frontend-modern/src/utils/apiClient.ts`
rather than surfacing raw JSON payload text from `response.text()` directly in
warning UI.
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
That same shell must also stay usable against older Pulse backends that do not
yet expose `/api/admin/reports/catalog`. When that specific metadata route
returns `404`, `useReportingPanelState.ts` may fall back to the governed legacy
performance-report transport (`/api/reporting` and `/api/reporting/generate-multi`)
so the reporting panel does not go dead on mixed-version installs, but that
compatibility path is intentionally report-only and must not invent the newer
catalog-owned VM inventory export surface.
`ReportingPanel.tsx` must therefore treat `vmInventoryExport` as optional when
it renders a governed reporting catalog. A legacy compatibility catalog with no
inventory export still owns a valid enabled reporting surface and must continue
to render the performance-report workflow instead of collapsing back to the
unavailable shell.
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
That same summary shell should also keep the shared page-card base neutral:
severity belongs in compact header accents, icon chips, and badges rather
than turning the entire full-width summary into a tinted warning banner that
breaks the surrounding Pulse surface language.
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
The same applies to Patrol operational context during active execution: the
shared feature surface may add an explicit run-in-progress badge, but any
activity support surface or integrated summary panel must remain factual
activity copy rather than shifting into a second Patrol verdict label while a
run is underway.
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
That same owner also holds generic settings-paywall CTA labels. Runtime shells
such as `AIRuntimeControlsSection.tsx` and `RelaySettingsPanel.tsx` must source
shared labels like `Upgrade to Pro` and `Start free trial` from
`upgradePresentation.ts` rather than embedding local CTA strings in the panel
markup.
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
That same shared sparkline boundary now also owns active-series isolation
metadata. The shell may expose `data-active-series-display` and
`data-rendered-series-count` for proof and inspection, but only the shared
runtime/model owners may decide whether a hovered or focused series is merely
emphasized or fully isolated; feature shells must not fork their own row-hover
line filtering.
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
math, hover target selection, focused-series tooltip detail, and density-cell
opacity rules. Future density-map work should extend those owners instead of
pushing canvas lifecycle, tooltip shaping, or chart math back into the shared
shell.
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
The shared page-controls bar now follows that same owner split.
`frontend-modern/src/components/shared/PageControls.tsx` stays the render shell
for canonical page-level control composition, while
`frontend-modern/src/components/shared/FilterToolbar.tsx` owns the shared
search-row, filter-row, and inline-leading-slot layout surface. Monitoring
pages that need workspace tabs or count chips next to search should route that
through the shared `searchLeading` slot instead of recreating a second local
header strip above the control bar.
That same shared filter-toolbar boundary also owns controlled select continuity
when filter options materialize asynchronously. `LabeledFilterSelect` must keep
the caller-owned `value` visibly selected after option children arrive so
dashboard, recovery, and other canonical filter bars do not drop their active
selection until the operator reopens the control.
That same boundary also owns live option propagation through shared page-control
composition. Callers such as storage and recovery must pass source/filter
option collections through reactive accessors instead of snapshot arrays when
those options depend on post-load unified-resource state, so the shared toolbar
can reconcile late-arriving options and preserved route selections without
requiring page-local reset hacks.
When those workspace tabs need an embedded control-bar treatment, they should
still stay on the one canonical `frontend-modern/src/components/shared/Subtabs.tsx`
primitive and reuse the established shell, list, and button class pattern
already proven on owning surfaces like operations rather than introducing new
variant APIs on the primitive.
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
product copy, or external links back into the shared shell. Internal product
navigation from that shell should still route through canonical shared helpers
such as `frontend-modern/src/routing/resourceLinks.ts` rather than freezing raw
`/recovery?...` route strings into the modal itself.
Canonical customer disclosures inside those shared shells now route through
`frontend-modern/src/utils/docsLinks.ts`, so settings and what's-new privacy
links resolve to shipped `/docs/...` assets instead of hard-coded GitHub
`main` URLs that can drift from the running build.
The shared summary strip primitives now follow that same owner split.
`frontend-modern/src/components/shared/SummaryPanel.tsx` and
`frontend-modern/src/components/shared/SummaryMetricCard.tsx` stay the render
shells for summary-frame spacing and card density, while monitoring surfaces
such as recovery, infrastructure, workloads, and storage only choose from the
owned shared density modes instead of forking summary spacing with feature-
local padding hacks. Future summary-density work should extend those shared
primitives rather than hard-coding compact card chrome inside one surface.
The shared tooltip now follows that same owner split.
`frontend-modern/src/components/shared/Tooltip.tsx` stays the render shell and
singleton API boundary, `frontend-modern/src/components/shared/useTooltipState.ts`
owns tooltip positioning lifecycle, RAF scheduling, and singleton visibility
state, and `frontend-modern/src/components/shared/tooltipModel.ts` owns tooltip
sanitization plus viewport-clamped positioning math. Future tooltip work should
extend those owners instead of pushing singleton state, DOM measurement, or
sanitization logic back into the shared shell. Shared portal-mounted tooltip
shells such as `frontend-modern/src/components/shared/TooltipPortal.tsx` must
use the same semantic surface tokens as the canonical tooltip instead of
introducing light-mode-inverted palettes.
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
recovery-local switcher pattern. When recovery embeds that switcher inside the
page shell, it should follow the same ordering already used by storage: shared
subtabs row first, shared controls card second, and data card after that. The
contained styling should come from the same canonical subtabs shell, list, and
button class treatment already used by established Pulse surfaces rather than
from a recovery-only variant boundary, adjacent chip row, or recovery-local
filter-row embedding.
The shared table primitives now also need to preserve caller-owned separator
styling. `TableHeader` and `TableBody` may provide canonical default borders
and dividers, but when a caller supplies explicit border or divide classes the
shared primitive must defer to that local contract instead of silently forcing
the default separator treatment back into the rendered DOM.
That same shared-boundary rule applies to summary density. The shared compact
mode on `SummaryPanel.tsx` and `SummaryMetricCard.tsx` exists for genuinely
dense monitoring surfaces, but pages that are trying to align with the normal
Pulse monitoring scan path should stay on the default shared density instead of
using page-local compact overrides by habit.
That same recovery shell boundary now also owns one canonical top-level filter
controller in
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`. Route-backed
recovery filters such as the provider-neutral `itemType` selector must be
derived, normalized, and fanned out to inventory, history, activity, facets,
and series consumers from that shared state owner rather than being recreated
as page-local toolbar state inside individual recovery sections.
That same shared recovery filter boundary also owns canonical recovery
item-type derivation through
`frontend-modern/src/utils/recoveryItemTypePresentation.ts`. Recovery shell
state, tables, summaries, and point-detail surfaces must resolve rollup and
point item types through the shared presenter helpers instead of repeating
`display.itemType` / `subjectType` / `subjectRef.type` fallback chains in
page-local consumers.
That same shared recovery decode boundary also owns canonical recovery display
shape. `frontend-modern/src/utils/recoveryPlatformModel.ts`,
`frontend-modern/src/hooks/useRecoveryPoints.ts`, and
`frontend-modern/src/hooks/useRecoveryRollups.ts` must normalize legacy
transport display aliases like `subjectLabel` and `subjectType` into canonical
runtime `itemLabel` and `itemType` fields before recovery presenters consume
the model.
The same shared recovery-column boundary must keep legacy `subject` and
`source` column ids at migration-only scope once
`frontend-modern/src/hooks/useColumnVisibility.ts` owns alias rewrites.
Recovery table runtime helpers and render switches should operate on canonical
`item` and `platform` ids rather than carrying the deleted ids as live cases.
That same shared recovery state owner now also keeps `platform` as the
canonical route and transport filter name for operator-facing recovery links,
while any accepted legacy `provider` aliases remain parser compatibility only.
Caller-facing shared recovery route builders must therefore stay
platform-first as well: compatibility `provider` aliases may be accepted while
parsing legacy links, but they should not remain a first-class input on new
recovery link construction helpers.
Recovery frontend decode and derived option builders must treat payload
`platform` / `platforms` as the canonical response fields and only fall back
to legacy `provider` / `providers` aliases for compatibility, so route,
filter, and table state do not keep backend-era vocabulary alive as the
default client model.
That normalization belongs at the shared recovery transport boundary in
`frontend-modern/src/hooks/useRecoveryPoints.ts` and
`frontend-modern/src/hooks/useRecoveryRollups.ts`, not in individual tables,
drawers, or summary cards. Recovery components should receive canonical
platform-first runtime models rather than re-deriving legacy alias fallback
locally.
Recovery section owners under `frontend-modern/src/components/Recovery/` must
consume that shared `platform` filter surface directly. They must not keep
recovery-local `provider` route/query vocabulary alive behind renamed labels,
or the UI will drift back to backend-shaped navigation even when the copy says
`Platform`.
That same shared recovery filter owner must also preserve route-owned platform
visibility while transport-backed options are still hydrating. If
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts` restores a
canonical `platform` selection such as `truenas` from the route before the
rollups, points, or facets payloads arrive, it must keep that selected
platform present in the option set so the shared `LabeledFilterSelect` shows
the owned value immediately instead of flashing back to `All Platforms` until
recovery data warms.
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
When the same governed run-history contract shows a recent full patrol plus
same-day scoped follow-up work, that summary shell should also carry a compact
activity-mix explanation rather than forcing operators to infer why Patrol
looked busy from a second competing status band.
That explanation belongs on the verification surface itself when operators are
reconciling `Recently verified` copy against same-day scoped Patrol bursts; the
supporting activity context may complement the readout, but it is not
sufficient as the only explanation path.
That same shell rule also owns Patrol recency labels. Shared Patrol header and
status-shell surfaces must keep `Last full patrol` tied only to the full-sweep
transport fact and use `Last activity` for scoped or verification work instead
of collapsing both timestamps back into a generic `Last run` label.
That same Patrol shell should make scoped trigger policy legible without
another navigation step. `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
should present alert-triggered and anomaly-triggered Patrol toggles as distinct
controls, and `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`
should render compact activity breakdown and scoped-trigger-state copy from the
shared transport rather than leaving busy Patrol periods as unexplained noise.
On the main Patrol page, though, that same governed activity context belongs
inside `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`
alongside the verification readout rather than as a second full-width band
above the findings workspace. If `PatrolStatusBar.tsx` is reused elsewhere, it
must stay a compact factual support surface and must not reintroduce a parallel
page-level verdict strip once the summary shell already owns that explanation.
That same composition rule applies to `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx`:
once the summary shell carries the operator-facing verification and activity
story, the workspace should move directly into findings and run history instead
of repeating that same runtime context through a second pre-tab status strip.

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
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`
owns the neutral thresholds sub-route contract:
`/alerts/thresholds/infrastructure`, `/alerts/thresholds/systems`,
`/alerts/thresholds/mail-gateway`, and `/alerts/thresholds/containers`.
Legacy `/alerts/thresholds/proxmox` and `/alerts/thresholds/agents` links
must redirect to the neutral infrastructure and systems routes so API-backed
platforms such as TrueNAS stay on canonical page language rather than
provider-specific aliases.
The infrastructure tab is itself now a shell that composes
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxNodesSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxPBSSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxGuestFilteringSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx`,
and `frontend-modern/src/components/Alerts/ThresholdsTableProxmoxStorageSection.tsx`
using the shared contract in
`frontend-modern/src/features/alerts/thresholds/thresholdsTableSectionProps.ts`.
Future infrastructure-thresholds presentation changes should extend those section
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
The systems tab now follows that same composition pattern through
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx`
and `frontend-modern/src/components/Alerts/ThresholdsTableAgentDisksSection.tsx`.
Future systems-thresholds presentation changes should extend those section
surfaces rather than restoring mixed JSX ownership to
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`.
The thresholds tab adapter contract now lives in
`frontend-modern/src/features/alerts/thresholds/thresholdsTabModel.ts`, so
`frontend-modern/src/features/alerts/tabs/ThresholdsTab.tsx` stays a thin shell
instead of carrying a duplicate table adapter contract inline. That adapter
must bridge function-valued selectors and mutation props into
`frontend-modern/src/components/Alerts/ThresholdsTable.tsx` explicitly; spread-
based table prop adapters are not allowed here because they can collapse
function props on the live Solid surface and break thresholds runtime state.
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
Within that alerts configuration runtime, canonical container-runtime projection
now belongs to `alertOverridesModel.ts`,
`useAlertOverridesState.ts`, and `useAlertsConfigurationState.ts`. The
thresholds `Containers` workspace must treat API-backed `app-container`
parents such as TrueNAS as first-class `Container Runtimes`, while Docker-only
controls in `ThresholdsTableDockerTab.tsx` remain gated to real
`docker-host` resources instead of leaking onto platform-managed runtimes.
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
That same feature shell now owns the resource-resolution handoff into the
resource-incident panel. `frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`
must pass the unified-resource resolver through to
`frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx`, and the
tab shell itself should only react to the current `alertData()` contract rather
than reviving deleted history-state aliases such as `filteredAlerts()`. The
panel may render compact route chips, but it must stay on shared route helpers
and feature-owned composition instead of growing provider-local routing logic
or another page-local resource lookup path.
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
The self-hosted Pulse Pro settings navigation item and route header metadata
for `frontend-modern/src/components/Settings/settingsNavCatalog.ts` and
`frontend-modern/src/components/Settings/settingsHeaderMeta.ts` are part of
that same shell boundary as
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` and the shared
settings billing presentation owner in
`frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`;
the `system-billing` navigation label plus header title and description must reuse
`SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle` and
`SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription` so the route header and
the billing shell do not narrate the same commercial surface differently.
That same settings-shell framing boundary also covers adjacent top-level
settings references to the self-hosted commercial surface. When
`InfrastructureWorkspace.tsx` or other settings-shell surfaces point operators
toward Pulse Pro for billing, monitored-system limits, or license status, they
must reuse the shared referral copy from
`SELF_HOSTED_PRO_BILLING_PRESENTATION` rather than drafting local “go there
for billing” variants.
That same shell boundary also has to stay safe for hosted tenant bundles.
Settings-shell framing copy for self-hosted billing must route through
`selfHostedBillingPresentation.ts`, with `settingsNavCatalog.ts`,
`settingsHeaderMeta.ts`, and adjacent hosted settings shells consuming that
settings-owned adapter instead of importing generic commercial presentation
helpers in ways that can reintroduce top-level bundle-init cycles.
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
Shared infrastructure action-link framing now also owns recovery entry wording
for service resources. `frontend-modern/src/components/Infrastructure/serviceDetailLinks.ts`
must keep platform-service recovery links on canonical recovery-events
framing and route state, so upstream service surfaces do not drift back to
PBS-backup wording or inherit the page-default inventory workspace when they
are actually deep-linking into recovery activity.
That same shared primitive boundary also owns resource handoff chip framing for
cross-surface investigation UI. Alerts, Patrol, and similar feature shells may
choose which governed surfaces to show, but they must build those links through
the shared resolved-resource route helpers in
`frontend-modern/src/routing/resourceLinks.ts` instead of freezing raw route
strings, local link dedupe, or provider-specific link chips inside feature
panels. Shared chip styling belongs in the feature shell; canonical href and
label truth belongs in the shared route helper.
That same shared primitive boundary now also owns persisted column-identity
migration for governed surfaces. When a v6 surface canonicalizes saved column
IDs, `frontend-modern/src/hooks/useColumnVisibility.ts` must accept explicit
legacy-to-canonical aliases so existing local preferences migrate forward
without resetting user choices or forcing the runtime to keep deleted column
IDs alive indefinitely.
That same shared primitive boundary now also owns environment-lock
presentation. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
stays the reusable badge shell,
`frontend-modern/src/utils/environmentLockPresentation.ts` owns the canonical
badge label, title, and lock-button copy, and
`frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx` stays
the settings-shell consumer for environment-variable-locked container-update
controls. Future environment-lock UX should extend those owners instead of
reintroducing panel-local lock labels, badge styling, or title copy.
The release-ready shell proof now also includes a representative desktop
Playwright rehearsal in
`tests/integration/tests/15-settings-shell-consistency.spec.ts` so general,
organization, billing, relay, security, AI, updates, and recovery panels are
all exercised through the built app shell under a seeded multi-tenant runtime.
The security-facing settings panels within that shell now also follow an
explicit shared boundary with `security-privacy` so shell framing stays here
while auth posture, token controls, and privacy semantics remain governed as a
trust surface instead of generic UX copy.
That shared shell boundary now also covers version-matched docs-link framing:
customer-facing privacy disclosures in shared settings surfaces must route
through `frontend-modern/src/utils/docsLinks.ts` rather than panel-local
external URLs.
That same docs-link boundary also governs local legal docs surfaced from the
settings shell: shared settings surfaces such as
`AIRuntimeControlsSection.tsx` must route Terms-of-Service links through the
shipped `TERMS.md` asset instead of hardcoding GitHub `main` URLs that can
drift from the running build.
The same shell boundary now also owns shared relay route framing copy:
`frontend-modern/src/utils/relayPresentation.ts` is the canonical owner for
the top-level relay settings description and availability copy used by both
`settingsHeaderMeta.ts` and `RelaySettingsPanel.tsx`, so the route shell and
its first `SettingsPanel` cannot drift into separate rollout or pairing
descriptions.

Single-surface settings pages that only render one canonical `SettingsPanel`
must stay rooted directly at that panel instead of wrapping it in an extra
page-level `space-y-*` container. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
`frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`, and
`frontend-modern/src/components/Settings/AuditLogPanel.tsx` are the current
reference cases, and
`frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
locks that direct-root contract so single-surface pages do not quietly regain
redundant outer spacing chrome.
The same shared settings-shell boundary now also owns the API-backed
alternative path inside Infrastructure Operations. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
`frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`, `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
`frontend-modern/src/components/Settings/settingsNavigationModel.ts`, `frontend-modern/src/utils/dashboardEmptyStatePresentation.ts`,
`frontend-modern/src/utils/infrastructureEmptyStatePresentation.ts`, and adjacent setup guidance must
treat `Platform connections` as the canonical API-backed alternative for
Proxmox, TrueNAS, and future provider integrations instead of reviving
top-level `Direct Proxmox` wording or shell-local provider routes.
That same settings-shell contract also owns the shared platform-connections
summary state. `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`,
`frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`,
`frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`,
`frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx`, `frontend-modern/src/components/Settings/TrueNASSettingsPanel.tsx`, and
`frontend-modern/src/components/Settings/VMwareSettingsPanel.tsx` must derive Proxmox/PBS/PMG/TrueNAS/VMware counts
and availability from one shared infrastructure settings state source instead
of letting the reporting summary and the provider-specific panels fetch the
same connection state separately.
That same shared settings-shell boundary also owns provider parity inside the
platform workspace. Adding VMware to the shared `Platform connections`
subtabs may extend the same card, empty-state, dialog, and summary-shell
patterns used by TrueNAS, but it must not introduce a VMware-only outer page
shell, alternate settings route hierarchy, or another summary vocabulary for
connection health and contribution counts.
That same shared filter-presentation boundary also owns infrastructure
route-filter continuity. `frontend-modern/src/features/infrastructure/`
must keep a route-owned canonical source option such as `truenas` visible in
the shared `LabeledFilterSelect` even when current unified-resource results do
not contain that source, so platform handoffs from settings and other
surfaces do not flash back to `All` while the operator is still in a
provider-scoped investigation flow.
That same shared feature-presentation boundary also owns storage disk-detail
fallback messaging in `frontend-modern/src/features/storageBackups/`. Shared
detail presenters must describe the actual capability or identity gap that
prevents history from rendering, rather than reviving agent-install guidance
on API-backed platforms like TrueNAS when the canonical disk metrics target is
already the owning history path.
That same shared chart primitive boundary now also owns physical-disk live I/O
drawers. `frontend-modern/src/components/Storage/DiskDetail.tsx` must render
read, write, and busy charts through `HistoryChart` plus
`useHistoryChartState`, using the canonical physical-disk history resource id,
instead of reviving `diskMetricsHistory`, a page-local ring buffer, or another
storage-only live chart primitive for the same telemetry.
