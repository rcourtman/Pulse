# Frontend Primitives Contract

## Contract Metadata

```json
{
  "subsystem_id": "frontend-primitives",
  "lane": "L8",
  "contract_file": "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "agent-lifecycle",
    "cloud-paid",
    "storage-recovery"
  ]
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
55. `frontend-modern/src/components/shared/FilterBar/FilterBar.tsx`
56. `frontend-modern/src/components/shared/FilterBar/FilterChip.tsx`
57. `frontend-modern/src/components/shared/FilterBar/AddFilterMenu.tsx`
58. `frontend-modern/src/components/shared/FilterBar/filterCatalog.ts`
59. `frontend-modern/src/components/shared/FilterBar/index.ts`
59a. `frontend-modern/src/components/shared/FilterBar/SavedViewsMenu.tsx`
59b. `frontend-modern/src/components/shared/FilterBar/useSavedViews.ts`
56. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
57. `frontend-modern/src/features/`
58. `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
59. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
60. `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
61. `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
62. `frontend-modern/src/components/SetupWizard/__tests__/SetupWizard.test.tsx`
63. `frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx`
64. `frontend-modern/src/components/SetupWizard/__tests__/WelcomeStep.test.tsx`
66. `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
67. `frontend-modern/src/components/Settings/useSystemLogsPanelState.ts`
68. `frontend-modern/src/utils/systemLogsPresentation.ts`
69. `frontend-modern/src/components/Settings/__tests__/SystemLogsPanel.test.tsx`
70. `frontend-modern/src/pages/Operations.tsx`
71. `frontend-modern/src/components/Settings/ResourcePicker.tsx`
72. `frontend-modern/src/components/Settings/reportingResourceTypes.ts`
73. `frontend-modern/src/utils/reportableResourceTypes.ts`
74. `frontend-modern/src/utils/reportingResourceTypes.ts`
75. `frontend-modern/src/utils/problemResourcePresentation.ts`
76. `frontend-modern/src/utils/workloadEmptyStatePresentation.ts`
77. `frontend-modern/src/utils/workloadGuestPresentation.ts`
78. `frontend-modern/src/utils/emptyStatePresentation.ts`
79. `frontend-modern/src/utils/semanticTonePresentation.ts`
80. `frontend-modern/src/components/Toast/Toast.tsx`
81. `frontend-modern/src/utils/toast.ts`
82. `frontend-modern/src/utils/semanticTonePresentation.ts`
83. `frontend-modern/src/utils/emptyStatePresentation.ts`
84. `frontend-modern/src/utils/typeColumnPresentation.ts`
85. `frontend-modern/src/pages/__tests__/Operations.helpers.test.ts`
86. `frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
87. `frontend-modern/src/components/Settings/networkSettingsModel.ts`
88. `frontend-modern/src/components/Settings/useDiscoverySettingsState.ts`
89. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
90. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
91. `frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts`
92. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
93. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`
94. `frontend-modern/src/components/Settings/settingsNavVisibility.ts`
95. `frontend-modern/src/components/Settings/settingsRouting.ts`
96. `frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts`
97. `frontend-modern/src/components/Settings/settingsTypes.ts`
98. `frontend-modern/src/components/Settings/useSettingsNavigation.ts`
99. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
100. `frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx`
101. `frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx`
102. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
103. `frontend-modern/src/utils/environmentLockPresentation.ts`
104. `frontend-modern/src/utils/docsLinks.ts`
105. `tests/integration/tests/20-local-doc-links.spec.ts`
106. `frontend-modern/src/index.css`
107. `frontend-modern/src/components/shared/summaryInteractionA11y.ts`
108. `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`
109. `frontend-modern/src/hooks/createNonSuspendingQuery.ts`
110. `frontend-modern/src/components/shared/TableCardHeader.tsx`
111. `frontend-modern/src/components/shared/SummaryTableCardHeader.tsx`
112. `frontend-modern/src/components/shared/UpgradeLink.tsx`
113. `frontend-modern/src/components/shared/useUpgradeNavigation.ts`
114. `frontend-modern/src/utils/upgradeNavigation.ts`
115. `frontend-modern/src/components/DemoBanner.tsx`
116. `frontend-modern/src/components/Login.tsx`
117. `frontend-modern/src/stores/demoMode.ts`
118. `frontend-modern/src/stores/sessionCapabilities.ts`
119. `frontend-modern/src/stores/sessionPresentationPolicy.ts`
120. `frontend-modern/src/stores/licenseCommercial.ts`
121. `frontend-modern/src/useAppRuntimeState.ts`
122. `frontend-modern/src/routing/routePreload.ts`
123. `frontend-modern/src/stores/aiChat.ts`
124. `frontend-modern/scripts/header-audit.mjs`
125. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx`
126. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts`
127. `frontend-modern/scripts/canonical-platform-audit.mjs`
128. `frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs`
129. `frontend-modern/src/utils/platformSupportManifest.generated.ts`
130. `frontend-modern/src/utils/platformSupportManifest.ts`
131. `frontend-modern/src/utils/sourcePlatformOptions.ts`
132. `frontend-modern/src/utils/sourcePlatforms.ts`
133. `frontend-modern/src/utils/infrastructureOnboardingPresentation.ts`

## Shared Boundaries

1. `frontend-modern/src/components/Settings/APIAccessPanel.tsx` shared with `security-privacy`: the API Access settings intro is both a security/privacy token-management trust surface and a canonical settings-shell presentation boundary.
   The panel may own shell placement and local action layout, but
   token-specific Docker / Podman copy must come from
   `frontend-modern/src/utils/apiTokenPresentation.ts` rather than page-local
   text.
2. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` shared with `security-privacy`: the data-handling settings surface is both a security/privacy trust surface and a canonical settings-shell presentation boundary.
3. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` shared with `security-privacy`: the data-handling settings model is both a security/privacy posture projection and a canonical settings-shell presentation boundary.
4. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `security-privacy`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
5. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `security-privacy`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
6. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `security-privacy`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
7. `frontend-modern/src/routing/routePreload.ts` shared with `performance-and-scalability`: the app-shell route preload registry is both a canonical frontend shell boundary and an authenticated hot-path performance boundary.
8. `frontend-modern/src/stores/aiChat.ts` shared with `ai-runtime`: the assistant drawer and session store is both an AI runtime control surface and a canonical app-shell presentation boundary.
   Assistant session pickers and reloads must restore only safe
   `handoff_summary` presentation state from the session list. Loading a plain
   session or starting a new conversation must clear stale scoped handoff
   briefing state so Patrol and alert context does not visually leak between
   conversations. Browser-originated model handoff payloads are one-shot
   request seeds: after the first successful chat send, this store must clear
   `handoffContext`, `handoffResources`, `handoffActions`, and safe
   `handoffMetadata` while preserving the safe visible briefing and scoped
   approval-required posture, so later turns rely on backend session hydration
   instead of resending stale browser context. Patrol handoffs must not include
   safe next-step labels, action kinds, or whitelisted app-route hrefs in
   `handoffMetadata`; the drawer must treat
   `handoff_summary.requires_approval` as a current pending-decision flag, not a
   historical action marker, so completed or rejected handoff actions render as
   action context rather than pending approval. A restored Patrol run summary
   must remain visibly sourced to Pulse Patrol, restore a `patrol-run` target
   plus run ID/type/status/runtime-failure presentation only, and must not
   rehydrate model-only runtime failure detail into browser context. New Patrol
   run requests follow the same drawer boundary: source-owned context and
   briefing copy may show classified, redacted failure summaries for operator
   review, but `handoffContext`, `handoffResources`, and `handoffActions` for
   run-history context must stay absent from the browser request so the backend
   can rebuild model-bound context from the stored Patrol run. Restored
   Patrol assessment, Patrol finding, and Patrol configuration-failure sessions
   follow the same safe-summary rule: the drawer may restore source label,
   title, target type, status badge, bounded resource facts, and approval/action
   status from `handoff_summary`, but
   it must not infer a finding target from bounded action references or
   reconstruct hidden model context, provider details, retry payloads, commands,
   preflight output, or action results in the browser. If the safe summary
   was created by a legacy build that stored Patrol next-step metadata,
   recommendation detail, action labels, safe action kind, or whitelisted
   app-route href, the session picker plus restored drawer must ignore those
   fields rather than carrying them forward as hidden context or visible
   recommendation copy.
   Session-load and new-conversation transitions must be success-bound: if the
   underlying session operation fails, the shared drawer store must not clear or
   replace the current scoped handoff context.
   Live Patrol assessment drawer opens must use that same
   `patrol-assessment`/`pulse-patrol-assessment` target identity rather than a
   retired dashboard target, so first-open and restored-session chrome remain
   source-named.
9. `frontend-modern/src/utils/platformSupportManifest.generated.ts` shared with `unified-resources`: the generated platform support projection is both a canonical unified-resource platform union boundary and a shared frontend source/platform vocabulary boundary.
10. `frontend-modern/src/utils/sourcePlatforms.ts` shared with `unified-resources`: the source platform normalizer is both a canonical unified-resource source adapter boundary and a shared frontend source/platform vocabulary boundary.
    That shared boundary must preserve `availability` as the agentless
    infrastructure source for `network-endpoint` resources and settings
    presets, so source badges and platform/source type resolution do not fall
    back to `generic` when an endpoint is represented by ping, TCP, or HTTP
    probe data rather than an installed agent or provider API.

## Extension Points

SSO provider settings changes must preserve the shared Community-tier action
path: SAML and OIDC provider creation stay on the same settings-shell control
surface, while paid-plan copy and compatibility feature probes stay out of the
frontend primitive boundary.

Feature surfaces under `frontend-modern/src/features/` may own product-specific
assessment semantics, but they must keep those semantics in their governed
presentation helpers and render them inside the shared neutral Pulse surface
language rather than introducing page-local verdict bands or nested cards.
Feature-owned table drawers use shared disclosure and inline-detail primitives
as local interaction state. Unless a surface has a separately governed deep-link
write contract, opening or closing a row drawer must preserve the current
document and URL instead of writing route state or reloading the page shell.
Feature-owned sortable table headers must render real button controls inside
the shared `TableHead` primitive and expose column state through `aria-sort`;
the feature owner may define the sort keys and data comparator, but the header
interaction must update that canonical owner state rather than forking
table-local sort state or making header labels look clickable while inert.

Shared filter/search primitives may provide the common shell, keyboard behavior,
history, and reset mechanics, but the owning page or table must supply
domain-specific visible copy, scope filters, status labels, and searchable
field coverage. Platform pages must not surface generic "rows/resources"
search affordances when the visible table is actually pods, VMs, datastores,
apps, mail gateways, storage pools, backup jobs, or another product-owned
object model. Add-filter controls that sit beside a page-level search box must
prefer direct selectable filter values over a second nested search affordance;
a page that needs filter-value search must keep that search inside the active
filter chip or an explicit page-owned advanced selector. Platform-owned filter
selectors must also exclude facet options from other platform scopes, even when
the underlying shared surface is mounted from the same Workloads or Storage
component.
Patrol's primary assessment strip is descriptive only; it must not render a
Patrol-authored recommended next step, suggested prompt chips, or a secondary
action band inside the assessment shell. If the same assessment opens
Assistant, the Patrol-to-Assistant handoff must carry only bounded evidence,
resource references, and factual governed approval/action metadata as model-only
context. Feature-owned Assistant handoffs may provide source context and safe
metadata, but the shared drawer boundary must not turn those handoffs into
frontend-authored prompts, tool routes, or remediation plans; the configured
model owns tool choice and diagnostic reasoning after the request reaches the
AI runtime.

1. Add shared primitives in `frontend-modern/src/components/shared/`
   Framed product table surfaces must consume the shared `TableCard` frame and
   `TableCardHeader` title/action band instead of composing page-local `Card`
   border, background, overflow, or table-title chrome. Feature owners may own
   the table data, filters, columns, and row behavior, but the outer
   product-table frame, section header band, and light/dark border treatment
   belong to frontend primitives so Infrastructure, Workloads, Storage, and
   Recovery do not drift visually. The shared `Table` primitive owns the
   horizontal scroll shell (`overflow-x-auto` plus touch scrolling); feature
   tables must not wrap it in page-local scroll containers just to restore
   table sides or mobile overflow. Headerless product tables, including alert
   history, still use `TableCard` for the outer frame instead of hand-coded
   rounded/bordered wrappers. Product tables already inside a canonical
   section frame, including storage pools, physical disks, and infrastructure
   settings source/configured-node tables, must use `Table` directly rather than
   nesting another card or scroll wrapper. If a framed table needs bounded
   vertical height, that constraint belongs on `Table.wrapperClass` so the
   shared table shell still owns overflow behavior. Resource-detail drawer
   tables that consume `Table`, including Docker Swarm services, Kubernetes
   namespaces/deployments, and PMG detail tables, inherit the same scroll-shell
   owner instead of carrying drawer-local `overflow-x-auto` wrappers. Other
   product table surfaces, including deploy wizard target tables, AI cost
   tables, Ceph tables, PMG resource panels, and `PulseDataGrid`, must follow
   the same rule: feature owners may pass `wrapperClass` for bounded height,
   border, radius, or scrollbar hiding, but they must not add raw table markup
   or local scroll wrappers around the shared table primitive. `PulseDataGrid`
   also owns its root frame variants: feature surfaces embedded directly inside
   an existing panel/card frame must use the shared `frame="flush"` mode rather
   than caller-local border overrides, horizontal-scroll wrappers, or negative
   margin compensation.
   Product-table subgroup/header rows must likewise consume the shared
   `frontend-modern/src/components/shared/groupedTableRowPresentation.ts`
   helper and `.grouped-table-row` CSS token contract instead of local
   `bg-surface-alt` or page-specific hover fills. This applies to grouped
   rows across Infrastructure, Workloads, Storage, Recovery, alert history,
   alert threshold tables, and Infrastructure Settings source-manager tables;
   feature owners may own group content and behavior, but not duplicate the
   subgroup band styling.
   Shared progress and metric-fill motion belongs to the frontend primitive
   CSS contract in `frontend-modern/src/index.css`. Generic progress bars must
   keep the CSP-safe `ProgressBar` / `foreignObject` shape and use the shared
   `.progress-fill-frame` and `.progress-fill` classes for width and color
   transitions instead of inline styles or page-local animation wrappers. The
   same global CSS owner must provide the `prefers-reduced-motion` disable path
   for these fills so feature surfaces inherit one accessibility policy.
   Numeric readout motion belongs to
   `frontend-modern/src/components/shared/AnimatedNumber.tsx` and its
   `useAnimatedNumberState` owner. Feature surfaces may opt metric labels and
   compact counters into that primitive, but must not create local counter
   timers, page-specific easing, or independent reduced-motion policy.
   Shared primitives must not reintroduce app-shell monitored-system capacity
   banners. Monitored-system grouping and ledger presentation belongs in the
   owned settings surfaces, while commercial plan explanation belongs in
   `cloud-paid` plan surfaces.
   Mobile navigation under the same shared boundary owns tab accessible names:
   icon components may keep their standalone labels, but the nav must treat
   those icons as decorative inside tab buttons so names come from the tab
   label plus meaningful badge counts, not duplicated icon titles.
   Shared grouped-resource presentation primitives must keep grouped resource
   labels operator-readable: count-led labels may aggregate repeated resources,
   but uncountable or category-like resource types such as storage must use
   resource wording instead of naive pluralization.
   Infrastructure filter chrome under `frontend-modern/src/features/infrastructure/`
   must use mode-oriented labels for table presentation controls: grouped table
   mode is `Grouped`, not `Cluster`, because cluster remains a
   platform/resource concept for Proxmox, Kubernetes, and similar inventory
   details.
   Settings shell search copy belongs to
   `frontend-modern/src/utils/settingsShellPresentation.ts`. Shared settings
   search must not display non-actionable shortcut chips such as `Any key`;
   if the shell exposes a shortcut hint, it must name an actual key chord,
   otherwise the hint must remain unset so the shared `SearchInput` renders no
   shortcut chip.
   Native select state belongs to the shared
   `frontend-modern/src/components/shared/FormSelect.tsx` primitive. It must
   apply controlled `value` props after options are mounted so settings panels
   such as workload discovery show the persisted option instead of falling back
   to the first option while the collapsed summary shows a different value.
2. Route new top-level settings surfaces through the canonical settings shell
   instead of introducing page-local framing.
   When a new operator-facing concern is closely related to an
   existing tab's intent, prefer adding it as a sibling
   `SettingsPanel` inside that tab's container component over
   minting a new top-level tab. The agent-integrations surface
   added in slice 59 follows this pattern:
   `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`
   ships under the existing API Access tab via
   `APIAccessPanel.tsx`'s composition, not as its own tab. This
   keeps the tab inventory bounded, avoids touching
   `settingsNavigationModel.ts`, the registry, the loaders, and
   the routing tests for additive sub-surfaces, and presents
   "tokens + what those tokens unlock" as one operator-facing
   story.
   Shared shells and primitives that need websocket or dark-mode context must
   consume `frontend-modern/src/contexts/appRuntime.ts`; they must not import
   `frontend-modern/src/App.tsx`, because `App.tsx` owns provider placement
   while frontend primitives own reusable consumption.
   That same shared shell boundary now also owns thin public-route handoff
   presentation in `frontend-modern/src/App.tsx`: compatibility routes such as
   `/pricing` may stay outside authenticated chrome, but they must remain
   minimal handoff shells that defer destination truth to the owning subsystem
   instead of embedding a second copy of public marketing or checkout UI inside
   the product runtime.
   Cloud acquisition follows that same app-shell rule: ordinary self-hosted
   frontend primitives must not register `/cloud` or `/cloud/signup` as public
   product-runtime routes, because Cloud signup belongs to Pulse Account and
   the Cloud control plane rather than a local in-product trial page.
   The same settings-shell boundary owns read-only landing posture: when the
   session presentation policy says the operator cannot manage setup, `/settings`
   and sidebar navigation must land on the canonical reporting/control surface
   instead of setup-oriented install routes.
   Route normalization must wait until that presentation policy has resolved
   before stripping infrastructure onboarding queries such as `?add=pick`,
   `?add=agent`, or `?add=detect`, so first-session and explicit add-flow
   handoffs do not lose their modal target during session bootstrap.
   When those infrastructure modal targets are preserved, the settings shell
   must keep the owning add-flow copy intact: the detect target is an API
   platform probe, not a generic infrastructure-source detector, and its outer
   dialog frame should match the management-API endpoint language owned by the
   agent-lifecycle onboarding contract.
   That same settings-shell boundary also owns explicit organization-route
   stability. Deep links such as `/settings/organization`,
   `/settings/organization/access`, and adjacent organization shells must keep
   their canonical header and page frame when the route itself is allowed,
   even while runtime capabilities or presentation policy are still settling.
   Shared shell filtering may hide the sidebar item until the governing state
   resolves, but `settingsHeaderMeta.ts`, `useSettingsAccess.ts`, and the
   canonical settings-shell tests must not bounce an allowed organization
   route back to `Infrastructure` just because nav filtering has not yet
   surfaced that tab.
   That same settings-shell boundary also owns authenticated operator identity
   propagation into organization panels.
   `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
   must derive the effective username from the resolved security status
   (`proxyAuthUsername`, then `ssoSessionUsername`, then `authUsername`) and
   pass that identity into the organization overview, access, and sharing
   panels so deep-linked organization routes do not collapse into anonymous
   read-only shells under proxy-auth, SSO, or local-auth sessions.
   That same shared session-presentation boundary also owns alerts read-only
   posture: `/alerts` may continue exposing reporting tabs such as overview and
   history, but activation controls plus configuration routes must collapse out
   of the public-demo shell instead of advertising blocked management actions.
   That same public-demo presentation boundary also owns Settings support
   posture: the authenticated demo shell must not advertise `Diagnostics &
   Health`, `Data & Reports`, or `System Logs` in the Settings navigation, and
   legacy `/operations/*` links must resolve through the canonical Settings
   routing boundary instead of reviving a standalone Operations utility tab or
   route-local support shell.
   Because `Data & Reports` is the `advanced_reporting` capability surface
   rather than a general diagnostics page, `settingsNavCatalog.ts` must hide it
   when that feature is unavailable; ordinary self-hosted users should see
   support diagnostics and logs without being shown a Pro-locked reporting tab,
   while paid instances keep the canonical `/settings/support/reporting` route
   and panel.
   Resource Privacy/Data Handling is a route-backed trust surface, not a
   commercial surface or default settings destination. The Settings shell must
   keep it governed by the Security registry/header/navigation model without
   advertising it in the normal sidebar while it remains informational only,
   and it must avoid trial, upgrade, paid-plan, or monitoring-limit copy when
   commercial presentation is hidden.
   General settings runtime cards that present source-platform actions must
   consume `frontend-modern/src/utils/systemSettingsPresentation.ts` and the
   shared source-platform vocabulary rather than card-local product names.
   `frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx`
   owns the local toggle layout, but Docker and Podman update-action titles,
   descriptions, environment variable display, and failure copy belong to
   `systemSettingsPresentation.ts` so settings surfaces do not drift back to
   generic container wording.
   Shared sparkline primitives must also stay CSP-safe by construction:
   `frontend-modern/src/components/shared/InteractiveSparkline.tsx` may use SVG
   attributes and shared state/model helpers for cursor, axis-label, and
   tooltip positioning, but it must not write inline `style=` attributes for
   tick labels, tooltip placement, or per-series transitions on the public
   shell. Axis labels must render in fixed-size SVG shells or another
   non-scaled primitive boundary; the shared sparkline must not put axis glyphs
   inside `preserveAspectRatio="none"` label viewBoxes that stretch text as the
   chart resizes.
   The same shared presentation boundary also owns reusable scroll containers:
   `frontend-modern/src/components/shared/Table.tsx` must keep touch-scroll
   behavior on classes and shared CSS in `frontend-modern/src/index.css`
   instead of reintroducing inline `style=` attributes for overflow or mobile
   scroll behavior. `frontend-modern/src/components/shared/PulseDataGrid.tsx`
   inherits that same boundary: shared data-grid shells must route scrollbar
   hiding and table-width sizing through shared classes plus HTML attributes,
   not inline overflow or min-width styles.
3. Add feature-specific presentation only when no shared primitive should own it.
   Feature surfaces under `frontend-modern/src/features/` that display
   product labels must consume the owning subsystem's presentation utilities
   rather than hard-coding divergent page-local copy. Shared primitives and
   feature shells may compose those labels, but they must not become a second
   source of truth for alert, storage, recovery, infrastructure, workload, or
   adjacent product vocabulary. Table-mode segmented controls that expose a
   grouped/list view mode must use
   `frontend-modern/src/components/shared/GroupedTableModeSegmentedControl.tsx`
   so the shared primitive owns the `Group by` accessible label, `Grouped` and
   `List` visible labels, tooltip titles, and icons instead of each resource
   surface rebuilding that language with subtly different resource-specific
   concepts. Shared
   `PageControls` owns trailing filter-row actions such as toolbar display
   controls, utility buttons, Columns, and Reset. Controls that should wrap
   with the column/reset cluster must enter through `toolbarTrailing` instead
   of staying as loose filter-row children, and those controls must stay
   grouped when dense toolbars wrap so popovers remain viewport-safe instead of
   drifting off-screen from page-local flex behavior. The shared action rail
   must align to the trailing edge at wrapped desktop widths and remain
   separate from the filter-control wrap zone instead of waiting for a wide
   breakpoint, so Recovery events, Workloads, Storage, Infrastructure, and
   future dense toolbars do not strand Filter/Columns/Reset actions as an
   isolated second-row fragment. Shared `FilterToolbarPanel` owns
   default filter-popover geometry, and `FilterToolbar` owns the shared chart
   visibility display action: Workloads, Storage, Infrastructure, and future
   summary-bearing pages must use `ChartVisibilityToggleButton` so the
   affordance exposes one `Show charts` / `Hide charts` pressed-state contract
   instead of rebuilding a one-option segmented control or an in-summary
   collapse chevron page by page. Feature state hooks under
   `frontend-modern/src/features/` own route-backed query state, selected item
   state, and data-window selection for their product surfaces; shared
   primitives and reusable presentation helpers may own viewport-safe chrome,
   focus treatment, pressed-state affordances, and accessible label builders
   for repeated controls. Recovery timeline columns follow that split:
   storage/recovery owns the range, selected day, chart/table transport
   windows, and bucket data, while the shared frontend boundary owns the
   reusable button focus/selected styling and ARIA wording so columns expose
   singular/plural recovery-point labels plus selected state consistently.
   Frontend primitives must not fetch recovery data, infer recovery date
   ranges, or carry a parallel selected-day store just to render timeline
   columns. Compact, stable, mutually-exclusive filters
   with two to five options should use `LabeledFilterToggleGroup` as a
   responsive control: toggle buttons at wide desktop widths and the native
   select fallback below that. Dynamic, user/environment-sized, or six-plus
   option filters remain `LabeledFilterSelect` surfaces so estate-sized lists
   such as nodes never become button groups. Filters that change which other
   filters exist, such as Workloads Type, must stay in a stable primary filter
   band ahead of the dependent estate/data filters so changing the parent
   filter does not move its own click target or the adjacent primary filters;
   when multiple filters are expanded into button groups at wide desktop widths,
   each expanded group must have its own row rather than sitting immediately
   after another expanded group. User-facing filter options must use operator
   mental models rather than implementation categories: Workloads Type exposes a
   single `Containers` bucket while the `system-container` / `app-container`
   distinction remains an internal data/deep-link compatibility detail.
   `PageControls` owns the default stacked control deck for page-level filters:
   filter controls, display/chart controls, Columns, and Reset inherit one
   shared structured command deck with visible section boundaries instead of
   each page passing local `controlDeckClass`, action-rail, border, or
   background strings. Pages that have multiple semantic filter groups may set
   the shared `filterControlsVariant="sectioned-children"` mode and wrap those
   groups with `pageControlsFilterSectionClass`, but the deck chrome and
   trailing action section remain frontend-primitives owned. Those structured
   decks must give each semantic section a visible boundary so adjacent radio
   groups, scope filters, and display actions do not collapse into one
   hard-to-scan strip. Narrow consumers such as
   `ColumnPicker` must opt into their panel width through that primitive rather
   than layering competing width classes page by page.
4. Add guardrail tests when a new shared pattern is introduced.
   Shared monitored-system primitives must prove they remain informational
   grouping or ledger surfaces rather than admission-freeze banners, cap
   summaries, or `current / limit` quota math.
   Shared modal scroll containment follows that same owner split. The dialog
   shell in `frontend-modern/src/components/shared/dialogModel.ts` must keep
   shared panels `min-h-0`, and page-owned modal bodies may use
   `overflow-y-auto` only under shrinkable flex columns instead of clipping
   lower fields behind a fixed-height shell.
   Shared filter popovers follow the same primitive-level ownership. The
   shared `FilterToolbar` panel class must render above nested card, table,
   and empty-state shells, and feature pages embedding those controls must
   make only their immediate control shell overflow-visible rather than
   forking local z-index or popover positioning rules.
   The shared navigation guide owns route-aware first focus: when it opens
   from a top-level product route such as `/recovery`, the first highlighted
   step should match that route instead of always restarting at Dashboard.
5. Keep shared infrastructure shell state on the reusable settings boundary: `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts` and `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx` must continue to derive provider counts, availability, and shared subtab copy from one infrastructure-settings source — via the unified aggregator through `frontend-modern/src/components/Settings/useConnectionsLedger.ts` — instead of creating provider-local summary fetches or VMware-only shell vocabulary. Phase 9 retired the old `PlatformConnectionsWorkspace` per-type shell; setup guidance should now use `Add infrastructure` plus source-strategy language for API-backed onboarding. The standalone connections-table presenter is retired; `frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx` is the only landing-ledger presenter for configured infrastructure rows.
   The first-run setup wizard inherits that same source-strategy vocabulary:
   step labels and completion copy must frame the final setup step as choosing
   the first infrastructure source, not installing a host. Successful token
   validation and security setup transitions should rely on the wizard progress
   state instead of transient success toasts that can cover the credential
   handoff. Generated first-run admin passwords must use browser cryptographic
   randomness rather than `Math.random`.
   That same shared shell boundary now owns the first-run posture for
   `/settings/infrastructure`: the landing route should read as one
   source-manager workspace with configured infrastructure instances first
   and no redundant monitored-systems ledger beneath it. The landing route may
   include a dedicated first-viewport toolbar that explains platform APIs and
   Pulse Agent telemetry as infrastructure sources and exposes `Add
   infrastructure`, `Run discovery`, and `Discovery settings` inside the
   source manager. Per-source add actions, including `Install Pulse Agent`,
   belong on the governed source rows, and `Detect address` stays inside the
   single add-flow probe path instead of a duplicate toolbar action. It may
   also show a compact coverage strip derived from the same unified connection
   rows and discovered candidates so operators can confirm connected-system
   count, API coverage,
   agent coverage, sources that still need an agent, and discovery review state
   without opening a tour or second ledger. Existing sources stay visible in
   stable source-catalog order, and add, detect, install, review, and manage
   flows open as
   secondary interactions from that same destination instead of taking over the
   whole page.
   The same source-manager workspace may show a compact fleet-governance strip
   and row-level fleet attention badges, but those badges must be presentation
   of the canonical `/api/connections` `fleet` object rather than another
   frontend-owned lifecycle classifier.
   Those secondary views must stay under the same single `Infrastructure`
   sidebar destination, but they may open in governed modal/dialog chrome when
   that preserves the persistent source-manager page behind them.
   That governed dialog chrome must also preserve inner form scrolling:
   `InfrastructureWorkspace.tsx` and `ConnectionEditor.tsx` keep the add/edit
   shell on `min-h-0` flex columns so long credential forms scroll inside the
   modal body instead of trapping the lower fields below the fold.
   The same shared shell boundary now also owns grouped source-row composition.
   `useConnectionsLedger.ts`, `InfrastructureSourceManager.tsx`, and
   `InfrastructureWorkspace.tsx` must render attached collection methods as a
   plain-language row subtitle on the owning row (`via platform API`, `via
   Pulse Agent`, or `via platform API and Pulse Agent`), with fuller detail in
   the edit dialog, instead of duplicating the same machine across multiple
   peer groups or forcing operators to decode badge jargon.
   The table-level product/system group rows in
   `InfrastructureSourceManager.tsx` must also use the shared grouped table row
   presentation helper, not local table-background classes, so source-manager
   grouping stays visually consistent with the product tables.
   That same shared shell boundary owns the landing taxonomy too: the primary
   grouping labels in the infrastructure manager must describe real
   platform/system owners, not collection methods. Agent-only machines belong
   in a standalone-host bucket, while `Pulse Agent` remains a collection-
   method label, install path, and detail-surface concept rather than a peer
   top-level
   pseudo-platform beside Proxmox, VMware, and TrueNAS.
   That same shared shell boundary also owns compact version visibility for
   agent-backed rows. The infrastructure source table must not grow a dedicated
   always-on version column for Pulse Agent; exact version text belongs in the
   edit/detail surfaces, while the landing table only surfaces a compact
   warning badge when an attached or standalone agent actually has an update
   available. That same table boundary must reuse the existing `System` and
   `Endpoint` cells for compact standalone-agent identity such as
   `Unraid 7.1.0` plus a reported host address; it must not add a new
   diagnostics column just to surface host facts the unified agent already
   reports.
   That same shared shell boundary now owns one canonical infrastructure
   destination in the Settings sidebar. `InfrastructureWorkspace.tsx` owns the
   source-manager landing inside that destination, while route-backed add flows
   and local edit flows stay single-purpose instead of stacking multiple
   page-level workspaces at once.
   The source-manager landing now also owns the explicit discovery strip for
   that destination. `InfrastructureSourceManager.tsx` may expose a compact
   discovery status line plus `Run discovery` and `Discovery settings`
   actions from the shared landing shell, but it must not start a network scan
   just because the page rendered. New-source admission belongs on the table's
   per-platform `Add` actions or the compact first-run/readiness actions rather
   than in the discovery strip, and the direct address-probe utility may appear
   as first-run setup guidance while header discovery actions remain dedicated
   to saved network scanning.
   Discovered API-backed candidates stay visible in the same platform-group
   table as configured sources, using the existing tree/table hierarchy
   instead of spawning a second discovery-only page or card stack.
   `InfrastructureWorkspace.tsx` must still open a new connection through
   `frontend-modern/src/components/Settings/ConnectionEditor/ConnectionEditor.tsx`,
   but the editor now serves as governed dialog content under the source
   manager rather than replacing the page inline. The `?add=pick` route owns
   the search-first infrastructure source finder, `?add=detect` owns the
   detect-from-address utility, and typed add routes jump straight into the
   matching credential
   slot through `initialType`.
   The picker must keep that first choice in recognizable system/service
   vocabulary, while the typed add dialog may use the shared source-strategy
   vocabulary after selection to explain API inventory, Agent telemetry, or
   API + Agent coverage. Agent-backed typed add routes keep the same governed
   dialog shell, but their embedded installer surface should stay focused on
   the selected system so the first visible command path does not re-expand
   into irrelevant platform choices. When an already-configured source is an
   agent-backed host profile, the source manager must group it under the
   operator-facing profile family and keep its add action on the same typed
   route instead of collapsing it back into generic standalone-agent copy.
   Credential slots are dispatched by the detected or manually-selected type
   and must still reach the canonical form body rather than diverging into a
   revived provider-specific workspace.
   For PVE, PBS, and PMG, the credential slot is
   `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`
   and it must compose `NodeModalBasicInfoSection`,
   `NodeModalAuthenticationSection`, `NodeModalMonitoringSection`, and
   `NodeModalStatusFooter` inline under the editor shell rather than
   embedding the full Proxmox workspace (discovery card, configured
   nodes table, delete dialog, node modal stack). For TrueNAS and VMware
   the credential slots are
   `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx`
   and
   `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx`
   and they must render only the connection form body inline under the
   editor shell — no connection list, no row actions, no surrounding
   panel chrome. Showing a ledger of other systems inside the credential
   slot is exactly the ledger-inside-editor drift this contract forbids.
   The configured-connections summary and source-manager rows themselves must
   render exclusively from the aggregator.
   `InfrastructureWorkspace.tsx` composes the platform-banded systems table
   from `frontend-modern/src/components/Settings/useConnectionsLedger.ts`
   (polling `GET /api/connections`). Table rows may open a governed edit
   dialog for mutable sources, but pause, resume, remove, last-error detail,
   and agent uninstall commands must live inside that owned edit surface or
   the shared row-action owner rather than returning to inline landing-page
   action clutter or a revived provider-specific detail page. When the
   backend marks a grouped Proxmox row with canonical cluster identity, the
   table primitive must render that cluster moniker as the row title instead
   of falling back to one sibling node hostname or reopening a standalone-host
   presentation for cluster-member agents. When that grouped row also carries
   backend-authored cluster members, the table primitive must render those
   nodes as child composition beneath the cluster row rather than flattening
   them back into peer top-level systems or hiding them entirely.
   The same table shell must keep fulfilled rows visible across polling and
   manual reloads by using a retained-value query boundary, not app-level
   Suspense or a blank loading replacement, so configured infrastructure does
   not disappear while the next `/api/connections` request is in flight.
   The systems table and setup summary must count the same visible posture
   highlights they render, not hidden raw fleet signals. Passive attached-agent
   config or rollout handshakes whose only cause is a missing comparable
   applied configuration fingerprint may stay in the raw row model for deeper
   diagnostics, but they must not create duplicate cluster-parent badges or a
   `Needs attention` count when the visible row/member posture is otherwise
   healthy.
   That same landing-shell boundary also owns represented-host dedupe between
   the unified ledger and the discovery strip. `InfrastructureWorkspace.tsx`,
   `frontend-modern/src/components/Settings/useConnectionsLedger.ts`, and
   `frontend-modern/src/components/Settings/infrastructureSettingsModel.ts`
   must treat backend-authored hostname/IP aliases as canonical identity so an
   already-represented platform row, attached agent augmentation, or grouped
   member suppresses the matching discovered candidate instead of showing the
   same machine twice under hostname-versus-IP drift.
   Phase 9 retired the
   parallel reporting/inventory surface entirely:
   `useInfrastructureReportingState`, `InfrastructureOperationsController`,
   `InfrastructureInventorySection`, `InfrastructureActiveRowDetails`,
   `InfrastructureIgnoredRowDetails`, `InfrastructureStopMonitoringDialog`,
   and the per-type shells `PlatformConnectionsWorkspace`,
   `ProxmoxSettingsPanel`, `ProxmoxDirectWorkspace`, `NodeModal.tsx`,
   `TrueNASSettingsPanel`, and `VMwareSettingsPanel` no longer exist.
   The aggregator plus `ConnectionEditor` is the only path; no
   parallel reporting state, stop-surface dialog, ignored-row fallback,
   or per-type workspace may be reintroduced. `connectionsTableModel.ts`
   carries only the connection-scoped `SystemManageAction` variant —
   `inventory-active` / `inventory-ignored` manage kinds must not
   return.
   Frontend infrastructure feature surfaces inherit that same source/platform
   vocabulary. `frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
   must keep the operator-facing resource filter on `Platform`, not `Source`,
   while the infrastructure table labels its primary identity column as
   `System`. Lower-level unified-resource contracts preserve merged-source
   detail for tooltips, accessibility metadata, and routing. Collection methods
   such as Pulse Agent and runtime capabilities such as Docker may appear as
   option or detail labels, but they must not become the primary top-level
   system wording when a provider/API platform or reported host OS/appliance
   identity better explains what the operator is looking at.
6. Keep Proxmox deep-link route selection on the shared settings-navigation boundary. `frontend-modern/src/components/Settings/settingsNavigationModel.ts` and `frontend-modern/src/components/Settings/useSettingsNavigation.ts` must treat the canonical PBS and PMG Proxmox deep links as agent-selection authority even though those URLs resolve to the shared `infrastructure-operations` tab. Reloading or remounting on a PBS or PMG deep link must not silently fall back to the PVE selector state.
7. Keep shared storage feature presenters on canonical platform truth. When reusable storage presenters under `frontend-modern/src/features/storageBackups/` classify canonical resources for the shared storage route, API-backed virtualization datastores such as VMware must stay inventory-only datastores instead of inheriting PBS-specific backup-repository or protected-target copy from older fallback branches.
   Those reusable storage presenters must also keep primary issue copy separate
   from contextual impact copy. Composite posture fields may include dependent
   resource or protected workload impact, but shared table/presenter primitives
   must derive primary issue labels and summaries from explicit incidents,
   storage risk summaries, or storage-risk reasons so healthy rows do not
   render impact text as a warning.
   The shared `resolveResourcePlatformType(resource)` helper in
   `frontend-modern/src/utils/sourcePlatforms.ts` is the canonical reader for
   "what platform family does this unified resource belong to" and must be
   used by every frontend consumer that buckets unified resources by family
   (platform pages, filter resolvers, presentation pickers). The helper
   prefers `resource.platformType` when present and falls back to the
   resource's `sources` array via the existing source-platform normalization,
   so client-side family grouping behaves identically against mock fixtures
   and live backends that leave `platformType` empty on a subset of
   canonical resource types.
8. Keep shared source/platform vocabulary on the governed manifest boundary. `frontend-modern/src/utils/platformSupportManifest.generated.ts` must be the tracked frontend projection of `docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json`, `frontend-modern/src/utils/platformSupportManifest.ts`, `frontend-modern/src/utils/sourcePlatforms.ts`, and `frontend-modern/src/utils/sourcePlatformOptions.ts` must consume that generated projection instead of embedding divergent future-label lists, setup/onboarding path allowlists, host-profile labels, or presentation-only guesses, and `frontend-modern/scripts/canonical-platform-audit.mjs` must fail when the generated projection drifts from the governed manifest. The generic `docker` source-platform label is "Docker / Podman" in shared selectors, badges, and filter options so v5 Docker users can find the runtime surface while Podman-backed rows are not mislabeled as Docker-only; "Container runtime" remains the governed platform family, not the primary customer-facing label. Identity colour is semantic, not page-local decoration: shared source/platform badges, host identity badges, and container runtime badges must use the shared presentation helpers so Docker remains on the Docker/Podman blue runtime tone, Podman uses its distinct runtime tone, Proxmox PVE remains orange, and those meanings do not drift across table rows, filters, drawers, or platform pages. Agent host-profile entries, including Unraid, stay in the generated `agentHostProfiles` projection and shared wrapper helpers; frontend primitives may render those labels for Pulse Agent install/identity copy but must not add them to the first-class platform union.
   The generated host-profile projection also carries runtime platform fallback
   metadata for shared explanation and parity with backend normalization, but
   frontend primitives must still render host-profile labels through
   explicit backend profile fields such as `agentIdentity.hostProfile` and
   unified-resource `platformData.agent.hostProfile` rather than prettifying
   presentation-only ids as platform values. Raw appliance identity aliases
   such as `unraid-os` belong only in the generated host-profile token list so
   shared helpers resolve them to `unraid` before presentation. Infrastructure
   `System` badges must append the platform runtime version when the payload
   proves that version belongs to the displayed platform identity, such as PVE
   `pveVersion` or a Pulse Agent report whose OS identity resolves to Unraid or
   Proxmox VE. They must omit the version rather than showing unrelated
   collector OS versions, such as Debian 12, beside an API-backed PVE badge.
   Shared row primitives that render Proxmox node identity, including
   `frontend-modern/src/components/shared/NodeGroupHeader.tsx`, must route raw
   PVE manager payloads through
   `frontend-modern/src/utils/proxmoxVersion.ts` rather than inlining
   page-local parsing or falling back to unrelated agent OS versions.
   System title metadata must apply the same identity rule: once the primary
   system badge names a platform with its version, source/method context may
   still add collection labels such as Pulse Agent, but it must not repeat the
   same platform again as an unversioned source badge.
9. Keep top-of-page summary interaction on shared primitives. Infrastructure, workloads, and storage summary cards must route sticky-shell behavior through `frontend-modern/src/components/shared/StickySummarySection.tsx` and route row-hover or focused-series rendering through shared chart primitives such as `frontend-modern/src/components/shared/InteractiveSparkline.tsx` and `frontend-modern/src/components/shared/DensityMap.tsx`, rather than page-local sticky wrappers or metric-card-specific hover logic. When a page keeps summary charts visible below the desktop breakpoint, it must use the shared `stickyDesktopOnly` mode instead of adding page-local media queries, so wrapped two-column summaries scroll as normal content and only become sticky once the large-screen layout is active. The shared summary-card contract must also own stable summary-card geometry for chart-backed cards so row hover, focus, synchronized readouts, or idle header metadata cannot ratchet the sticky summary taller across rerenders. Shared chart slot geometry belongs in `frontend-modern/src/components/shared/summaryChartLayout.ts` so `SummaryMetricCard` and governed non-summary chart sections can consume the same normal and compact chart heights instead of re-declaring page-local `h-*` sizing.
   Storage's top-of-page summary scope is limited to the stable chart grid and
   shared row/group focus affordances. A rolling-history capacity planner must
   not be bolted onto the sticky summary shell as an extra frontend primitive;
   if capacity planning returns, it needs an owner-level model that can keep
   the summary geometry and visible card set stable across refreshes.
10. Keep summary chart interaction identity on one shared helper. Summary surfaces that expose row-hover, group-hover, chart-hover, or route-focus-driven chart emphasis must derive page/group/entity scope through `frontend-modern/src/components/shared/summaryCardInteraction.ts` and pass that same resolved scope into card-state, sparkline, and density-map primitives, rather than letting cards read `hovered || focused` while charts listen to a different page-local ID source. Hovering one summary chart must promote that series into the shared active entity so sibling cards highlight the same object instead of keeping chart-local hover islands, and hovering or pinning a workload group header, infrastructure cluster header, or storage pool-group header must scope the matching summary cards through that same shared contract instead of forking a page-local summary filter path. Sibling cards should surface that synchronized hover as one compact header readout through the shared summary-card contract, while the chart under the pointer keeps the only floating tooltip. Recovery is explicitly outside this interaction dialect: its retired posture-card strip must not return with row/group/chart hover behavior without a separate governed product decision.
11. Keep page summaries page-scoped when table rows enter contextual focus. Route-backed row selection may add a focused label and shared series emphasis, but infrastructure, workloads, and storage summary cards must continue to render the page-level series set instead of collapsing the summary down to the selected row or replacing the global trend view with row-local empty states.
12. Keep contextual row focus on the shared summary primitive. Summary surfaces and same-route table drill-ins must reuse `frontend-modern/src/components/shared/contextualFocus.ts` for interactive-series filtering, focused-name lookup, active-series derivation, local scroll preservation, and deliberate inline-detail reveal instead of rebuilding page-local `Set` filters, focused-label scans, drawer-aware scroll math, or ad hoc scroll restoration in each surface.
13. Keep summary-to-table coordination deliberate, explicit, and reversible. Shared summary hover may highlight the matching table row when it is already visible, but transient chart hover must not auto-filter tables, auto-scroll the page, or reshuffle table ordering. Pinned page/group/entity scope on workloads, infrastructure, or storage must stay row-first: the pinned row or group header is the visible scoped state, not a second strip or search-row widget. Page shells therefore must not reintroduce always-on scope banners, preview bars, page-local chips, breadcrumbs, or search/filter-row scope accessories just to explain pinned state. When the active row is off-screen, page owners must still route through `frontend-modern/src/components/shared/summaryTableFocus.ts` and surface a lightweight `Jump to row` affordance that reveals and scrolls only on explicit user action. That same shared table-focus owner now also owns reversible clearing: pinned scope may clear only from governed neutral interaction-surface space or the shared `Escape` path, with page owners binding a broader clear-surface root separately from the row-lookup table root when needed and supplying one page-level reset callback for filters plus summary-linked selections. Row cells, group headers, inline detail, summary cards, and explicit controls must not accidentally clear pinned scope, while governed table/card clear surfaces must still allow real user clicks on neutral whitespace to clear it. Deliberate row focus may reveal inline detail automatically, but that reveal must be drawer-aware: infrastructure and workload row toggles that already have the row in view must hand the current `.app-scroll-shell` position through `frontend-modern/src/utils/appShellScrollRestoration.ts` so the remounted shell in `frontend-modern/src/App.tsx` can reopen the inline detail without looking like a page refresh, and then still route through the shared reveal helper whenever the opened drawer would otherwise land below the fold. Same-route drawers must therefore scroll only enough to keep the row header plus the top of the inline detail visible, never hard-center the row just because the route state changed. The same-route state scheduler owns the lifecycle for its deferred scroll-restore timers and animation frames; page owners must clean it up on unmount instead of leaving replay work attached to a torn-down route.
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
    native, but they must not grow separate scope/pinned pill buttons or
    off-screen fallback strips. Workloads, infrastructure,
    and storage must not rebuild row-as-button keyboard handling or trailing
    one-off expand columns once the shared action primitive exists.
    When pinned page, group, or entity scope needs a local explicit reset,
    the only shared table-chrome owner is
    `frontend-modern/src/components/shared/SummaryTableCardHeader.tsx`: the
    reset action stays as one compact header-level `Clear` control with an
    accessible `Clear selection` label, not a second page-level scope strip,
    search-row accessory, or filter-bar badge.
13. Keep summary-linked table row emphasis on the shared primitive contract. Workloads, infrastructure, and storage rows that mirror the active summary entity must expose that state through `data-summary-row-active` and let the shared presentation in `frontend-modern/src/index.css` render the row emphasis, rather than carrying page-local sky or blue fill classes inside each row renderer. Group-scoped preview and pin must use that same shared presentation boundary: child rows that belong to a hovered or pinned summary group should expose `data-summary-group-member-active="preview|pinned"` so the block-level emphasis stays subtle, consistent, and reversible instead of each table inventing its own outline, badge, or full-strength fill treatment. Static grouped row headers on workloads, infrastructure, storage, recovery, and future grouped tables must use `frontend-modern/src/components/shared/groupedTableRowPresentation.ts` plus the `.grouped-table-row` CSS contract in `frontend-modern/src/index.css`, rather than rebuilding local `bg-surface-alt` variants with subtly different light/dark behavior or page-local left-accent markers. That shared grouped-table primitive owns the subgroup cell padding, typography, small metadata, and badge treatment as well as the row background token, so a future adjustment to the subgroup visual language changes every grouped product table from one owner. Storage-backed reusable row presenters under `frontend-modern/src/features/storageBackups/` must also keep row height and alert accents on class/data-attribute presentation instead of runtime inline style maps, so the shared table contract stays CSP-safe on both steady-state and alert-highlighted routes.
14. Keep retained-value data loading honest at the ownership boundary. Helpers
    that prevent a feature surface from falling through the app-level Suspense
    boundary during in-flight refresh should stay feature-local until multiple
    governed surfaces truly share the behavior. Once that boundary is shared,
    promote the helper into an explicit shared hook owner such as
    `frontend-modern/src/hooks/createNonSuspendingQuery.ts` rather than
    re-copying suspense-escape logic into each feature area or burying it
    inside one feature's private state model.
15. Keep shared commercial warning banners truthful about destination intent.
    When a shared banner renders both explanatory and commercial CTAs, those
    labels must resolve to distinct owned destinations or section anchors
    instead of presenting two different labels that land on the same
    unscoped billing screen. Monitored-system capacity warning banners are
    retired; shared commercial banners must not render stale `current/limit`
    counts, paid-plan CTAs, usage summaries, or upgrade-impression telemetry
    from legacy monitored-system limit payloads. When a banner does need a
    review destination for a current paid feature, it must scope the operator into the
    usage-owned policy ledger rather than plan-selection intent or CTA copy
    that frames the flow as monitored-system-cap expansion.
16. Keep assistant availability bootstrap on the shared app-shell boundary.
    `frontend-modern/src/useAppRuntimeState.ts`,
    `frontend-modern/src/App.tsx`,
    `frontend-modern/src/stores/aiChat.ts`, and
    `frontend-modern/src/components/AI/Chat/index.tsx` must consume the
    backend-owned `/api/security/status.sessionCapabilities.assistantEnabled`
    fact instead of probing `/api/settings/ai` or `/api/ai/*` during ordinary
    route bootstrap. Closed assistant chrome and non-AI settings panels may
    not initialize assistant runtime state until an owned assistant or Patrol
    surface is actually open. `frontend-modern/src/stores/aiChat.ts` is the
    shared drawer shell owner for assistant open/close state, focus handoff,
    and tenant-local context/session persistence; the app shell must not fork
    that state across `App.tsx`, `AppLayout.tsx`, or page-level helpers.
    The same shared shell boundary must keep Pulse Assistant coherent while a
    blocking shared dialog owns the viewport: closed launcher affordances must
    hide until the dialog clears, and the shell must close any already-open
    assistant drawer instead of leaving background assistant controls visibly
    active behind the modal.
    AI-owned frontend surfaces that need shared settings or model-catalog
    truth must route those reads through
    `frontend-modern/src/stores/aiRuntimeState.ts` rather than each feature
    bootstrapping `/api/settings/ai` or `/api/ai/models` independently.
    Non-AI settings panels such as
    `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
    must stay on the app-shell assistant-availability fact instead of
    re-reading raw AI settings just to decide whether assistant affordances
    should render.
17. Keep optional shared selectors honest about data ownership. Reusable
    shells such as `frontend-modern/src/components/shared/InfrastructureSelector.tsx`
    must gate their runtime data hooks on actual surface visibility, passing
    explicit disabled/null inputs to shared data owners when the selector is
    hidden instead of hydrating background summary data for chrome the page is
    not rendering.
18. Keep Patrol shell composition and product-first provider vocabulary on the
    shared feature-presentation boundary.
    `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`,
    `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx`,
    `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`,
    `frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`,
    `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`,
    `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`,
    `frontend-modern/src/features/patrol/patrolSupportingContextPresentation.ts`,
    `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`, and
    `frontend-modern/src/utils/patrolRuntimeActions.ts` must keep
    Patrol assessment, verification, and findings primary; surface recent
    changes, learned correlations, and policy coverage only as explicitly
    secondary supporting context when degraded or incomplete verification,
    active findings, or selected-run investigation makes that evidence
    relevant; and use Patrol/provider wording for the shared provider settings,
    provider model, and provider circuit-breaker affordances instead of
    generic AI labels inside Patrol-owned shells. The shared app shell in
    `frontend-modern/src/App.tsx` and `frontend-modern/src/AppLayout.tsx` must
    likewise expose `/patrol` as the canonical route and navigation target,
    keeping legacy `/ai` entry points as thin compatibility redirects rather
    than a second Patrol-branded primary route. `PatrolIntelligenceHeader.tsx`
    must also keep the page heading's accessible name singular: when the
    `PulsePatrolLogo` appears beside visible Patrol heading text, it is
    decorative rather than a second label source. The Patrol-owned supporting-context
    presenter must also keep the disclosure toggle plus evidence-boundary copy
    centralized instead of letting the workspace reintroduce inline shell-local
    trust wording, while the Patrol investigation-context owner normalizes
    same-state recent-change records into changed-substate wording before the
    workspace or Assistant handoff renders them. The same shared feature-shell
    boundary owns the
    commercial-facing Patrol capability language: Pro-locked helper copy,
    autonomy segmented controls, and run-history/result labels must use
    `Remediate`, `remediated`, or safe remediation wording while legacy API
    names remain hidden from operators. The Patrol configuration popover is part
    of that shared feature-presentation boundary: it must stay viewport-bounded,
    expose an accessible dialog label, and pass backend save rejection reasons
    through as inline dialog state instead of replacing them with generic toast
    copy. When the failure includes Patrol readiness context, the inline state
    must expose the provider, model, and readiness summary next to a direct
    provider-settings action instead of hiding that diagnosis behind Assistant
    alone. The provider-model selector in that popover must stay bound to the
    shared runtime settings/model catalog even when the popover mounts after
    async catalog loading, so a saved direct-provider Patrol model renders as
    that model instead of visually falling back to the default selection.
    Successful provider-model saves that return a not-ready Patrol
    readiness snapshot must use that same inline surface with `needs attention`
    wording, while Assistant receives a saved configuration issue rather than a
    failed-save handoff. When safe remediation is locked, the same popover
    state owner must
    clear stale full-mode unlock state before Apply Configuration submits the
    monitor-only autonomy payload, so disabled paid controls cannot leak stale
    permission into a save. If that inline state opens Assistant, the Patrol
    feature must hand off
    a source-named, model-only briefing and close the popover so the shared
    Assistant drawer is not visually hidden behind feature chrome. When a Patrol
    assessment handoff is attached, the shared Assistant drawer empty state must
    stay aligned with that source-named briefing and must not render generic
    cluster/system starter prompts below the Patrol-owned context. The Patrol
    feature shell must also consume the Patrol-owned findings source for its
    findings tab, run-scoped findings panels, and tab badges so shared feature
    composition does not rebuild Patrol state by filtering the cross-product
    unified findings feed.
19. Keep the shared `system-ai` settings shell product-first.
    `frontend-modern/src/components/Settings/AISettings.tsx`,
    `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
    `frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
    `frontend-modern/src/components/Settings/useAISettingsState.ts`, and
    `frontend-modern/src/utils/aiSettingsPresentation.ts` must present that
    surface to operators as `Assistant & Patrol` plus provider/model
    configuration rather than as a generic `AI Services` shell. Settings-save
    feedback must preserve provider-specific preflight failures and successful
    save responses that carry Patrol readiness warnings, including the provider,
    selected Patrol model, failure cause, safe recommendation, and readiness
    summary when those fields are present. The settings shell may compose that
    safe backend diagnostic for display, but it must not infer provider
    remediation by parsing raw upstream error strings in the browser. Provider
    setup cards must describe provider families through the current
    backend-owned provider contract; DeepSeek setup copy is the V4 family and
    must not regress to old V3 or compatibility-alias wording.
    Runtime controls inside `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`
    must likewise describe discovery as workload discovery that supplies
    concrete service context to Pulse Assistant and Patrol, not as a generic
    AI context feature. `frontend-modern/src/components/Settings/useAISettingsState.ts`
    must save workload discovery enablement and interval as one explicit
    settings pair so selecting "Every 6 hours" or "Manual only" round-trips
    through `/api/settings/ai` without depending on stale read-side diffing.
    The same workload-discovery settings section must expose a manual
    "Run discovery now" action wired through `/api/discovery/run` when
    workload discovery is enabled in manual-only mode, while resource-drawer
    discovery remains the forced single-resource refresh path.
    Assistant-only controls inside the shared shell, such
    as execution permissions and session maintenance, must stay explicitly
    labeled as Pulse Assistant controls, while Patrol schedule and autonomy
    continue to live on Patrol-owned surfaces rather than drifting back into
    the shared settings shell. Shared/default model choices may remain on the
    combined shell only when Assistant, Patrol, and Discovery overrides are
    presented as explicit per-surface overrides instead of a generic advanced
    AI bucket. Each per-surface override must fall back to the shared default
    when left empty rather than silently using a hard-coded backend default,
    so the shared shell stays the single place an operator picks the model
    for any of the three surfaces.
    The shared shell must not show Pro-only autonomous execution as a default
    free-user control when upgrade prompts are suppressed; it may surface that
    option only when the entitlement is present, commercial prompts are
    explicitly allowed, or the current saved setting already uses autonomous
    mode and needs to remain visible for operator review.
    Provider model catalogs must remain curated on that same shell:
    `frontend-modern/src/components/shared/AIModelPicker.tsx` owns the
    searchable, notable-first model picker pattern, and
    `AIModelSelectionSection.tsx` must feed it configured-provider models plus
    the current manual selection instead of rendering raw provider catalogs as
    plain select options. The picker must also constrain its dropdown and
    internal result list to the available viewport height so settings model
    catalogs remain usable on mobile and tablet layouts with bottom navigation.
    Platform-first top-level pages registered through
    `frontend-modern/src/App.tsx` must stay chrome-only and route through the
    canonical app shell: each per-platform surface owns navigation and sub-tab
    chrome, then embeds the canonical `WorkloadsSurface`, `StorageSurface`,
    `RecoverySurface`, or `UnifiedResourceTable` in `embedded tableOnly` mode
    with a forced platform or source filter. Per-platform features must not
    fork their own table primitives, header layouts, or summary cards when a
    shared canonical surface already exists; new shared platform-page
    primitives live under `frontend-modern/src/features/platformPage/` so the
    chrome stays reusable across families.
    `frontend-modern/src/AppLayout.tsx` may extend the `PlatformTab` list with
    new family entries, but primary navigation is a support-and-evidence-gated
    surface: rendered platform tabs, command/search destinations, keyboard
    shortcuts, and authenticated landing fallbacks must derive from the
    governed support manifest plus current runtime resource evidence.
    Supported platform families appear when evidence proves they are present;
    admitted-only, presentation-only, unsupported, or absent families stay
    hidden rather than rendering as disabled placeholders.
    The `MOBILE_NAV_PLATFORM_PRIORITY` ordering in
    `frontend-modern/src/components/shared/mobileNavBarModel.ts` mirrors
    that platform-first set so mobile and desktop primary navigation stay
    aligned; legacy Infrastructure / Workloads / Storage / Recovery entries
    are intentionally absent from that priority list.

## Forbidden Paths

1. Reinventing table/filter/toggle primitives when a shared version exists
2. Feature-local styling forks of canonical shared components without explicit justification
3. Direct imports that bypass shared presentation helpers where guardrails exist
4. Top-level settings panels introducing bespoke page-level headers or outer
   framing instead of the canonical settings shell and `SettingsPanel`
   contract
5. User-facing diagnostics or settings panels rendering maintainer/admin
   analytics such as commercial funnel, sales funnel, pricing/checkout
   conversion, or infrastructure onboarding telemetry. Those signals belong in
   admin-owned metrics surfaces, not the product diagnostics UI or customer
   frontend event emission.

## Completion Obligations

1. Update guardrail tests when new shared primitives are added, including
   new Settings controls that drive backend verification surfaces (for
   example the Verify Patrol button in
   `AIModelSelectionSection.tsx`, which must drive the typed
   `runPatrolPreflight` client through `useAISettingsState.ts` rather
   than inlining fetch calls in the section component, must hydrate
   its result panel from the `patrol_preflight` snapshot on
   `/api/settings/ai` so the "last verified" state survives page
   reloads without forcing a re-click, must pass the form's pending
   `patrolModel` as the model override so the click tests the
   operator's unsaved dropdown selection rather than whatever was
   previously saved, and must surface a stale-cache warning when the
   form's selection differs from the cached result's model so the
   green badge cannot silently mislead)
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
   That same shared owner must also support explicit IA/title separation for
   self-hosted commercial settings: the nav label may stay product-IA-first
   (`Plans`) while the page shell stays task-first (`Self-hosted plan`), and
   the owned plan shell must foreground the active plan name plus available
   capabilities before secondary billing or recovery detail so paid upgrades
   can confirm their entitlement immediately after activation without making
   default Community look like it is missing an activation key.
7. Keep hosted settings-shell framing imports safe for bundle initialization.
   Self-hosted billing titles, descriptions, and referral copy used by
   `settingsHeaderMeta.ts`, `settingsNavCatalog.ts`, and adjacent settings
   shells must flow through
   `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
8. Keep shared settings-shell AI control copy capability-scoped rather than
   upsell-scoped. `AIRuntimeControlsSection.tsx` may describe read-only,
   approval-required, and autonomous action posture, but option labels and
   helper text must avoid tier labels or broad "executes everything" wording;
   paid capability availability belongs to entitlement-backed visibility and
   lock state, not local select copy.
   instead of importing generic commercial presentation helpers directly into
   hosted settings route shells.
   Contextual settings feature gates must use capability-owned presentation
   helpers and neutral paid-plan copy. They must not reintroduce `Pro feature`
   badge titles, Pro-suffixed option labels, monitored-system limit claims, or
   browser-local commercial/onboarding metrics wrappers in SSO, audit,
   reporting, AI controls, agent profiles, or shared warning banners.
8. Keep first-session dashboard empty-state copy on
   `frontend-modern/src/utils/workloadEmptyStatePresentation.ts`, and make
   infrastructure setup guidance name the canonical destination explicitly
   instead of falling back to generic settings CTA labels.
9. Keep the live first-session wizard on the canonical three-step runtime
   shape in `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
   (`Welcome`, `Security`, then `Install`), and keep the step indicator plus
   completion CTA language aligned with the governed infrastructure install
   workspace instead of regressing to a route jump that leaves the next action
   implicit. Preview-only follow-up surfaces such as
   `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
   must stay deterministic and scenario-driven: they may not poll the live
   `/api/state` runtime or inherit whatever connected systems happen to exist
   on the current backend, and browser proof for `/preview/setup-complete`
   must select explicit preview scenarios instead of ambient runtime state.
10. Keep AI settings setup UI backend-driven:
    `frontend-modern/src/components/Settings/useAISettingsState.ts` and
    `frontend-modern/src/components/Settings/AISettingsDialogs.tsx` may collect
    provider credentials or runtime URLs, but they must not bake vendor model
    IDs into setup payloads. The shared settings shell should let the backend
    resolve the effective BYOK model and then render that returned state rather
    than guessing a model in the modal.
    Scoped Assistant handoffs must keep request-local execution overrides in
    drawer context. Dashboard and other route-owned entry points may open the
    Assistant drawer with source context and `autonomousMode:false`, but they
    must not pre-fill or auto-submit a prompt, mutate persistent AI control-level
    settings or trigger background Assistant settings/model bootstrap before
    the drawer is open. Patrol finding handoffs that add structured
    investigation-record framing must derive that context through
    `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
    so shared drawer primitives stay shell-owned rather than becoming a
    Patrol-specific diagnosis formatter; shared drawer primitives must not
    branch on intent themselves. Patrol-page
    surfaces must not add standalone trust strips to shared header or
    workspace chrome; high-signal trust facts may feed the Patrol-owned
    assessment readout, but shared drawer/chrome primitives stay free of
    the `FindingsTrustSummary` shape so adding new trust signals goes
    through the contract first rather than per-shell branching.
    Patrol header refresh controls stay on that same feature-owned shell
    boundary: `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`
    must make the refresh affordance generation-aware and timeout-bounded, so
    a slow supporting intelligence read cannot permanently disable the shared
    Patrol header control while Patrol findings and status remain visible.
    That feature-owned
    presentation helper is the single emitter for investigation-record
    `impact` and `rollback` fields: when an investigation record exists but those fields
    are empty, the helper emits explicit `Impact not assessed` and
    `Rollback not specified` lines into the model-only Patrol finding
    prompt context so the operator-visible gap is surfaced to Assistant
    rather than hidden, and shared chat primitives stay free of that
    placeholder logic. Patrol assessment-level handoffs must use
    that same feature helper to attach bounded model-only assessment,
    verification, latest-run, supporting-context evidence, active-finding, and
    resource reference context while forcing request-local approval-required
    mode. Patrol run-history handoffs must also use that feature helper rather
    than a row-local Assistant prompt, so the shared drawer receives only a
    generic visible briefing plus bounded model-only run context, scoped
    resource references, runtime failure summary/detail, and
    `autonomousMode:false` while the Patrol feature remains the source of run
    copy and retry/configuration guidance. Active-finding entries in that
    assessment handoff may add live pending
    approval posture only as safe structured metadata: approval ID, pending
    status, risk, target, requested/expiry timestamps, action plan identity,
    requester identity, approval policy, plan expiry, dry-run posture, and
    command count. Those entries may be passed through shared chat transport as
    `handoff_actions` for
    model-only refresh, but the shared drawer stays a generic shell rather than
    a Patrol summary prompt builder. The Patrol helper may turn those same safe
    references into visible action labels and safety notes for assessment and
    finding-level handoffs, but it must not produce Patrol-authored suggested
    prompt chips, recommendation titles, recommendation reasons, or route-owned
    next-step actions. Assessment-level Patrol prompts, action labels, and
    safety notes must describe active findings, pending approvals, governed
    action references, and coverage caveats as evidence for the configured
    model, not as a frontend-authored decision tree.
    Finding-level drawer opens may also pass one bounded
    model-only finding context, one target resource reference, and one
    `handoff_actions` reference for a live approval or proposed fix. It must not
    expose raw command or execution payloads.
    The
    drawer may render a generic
    context-briefing band from `frontend-modern/src/stores/aiChat.ts`, but
    feature-owned helpers must provide compact source labels, primary subject,
    status, and governed approval/action artifact metadata while keeping
    detailed evidence, safety notes, and model-only finding context outside
    drawer chrome. Prompt suggestions, attention-reason copy, and
    operator-decision framing must stay out of Patrol drawer chrome.
    Patrol finding and action-artifact handoffs must not render suggested prompt
    chips in the drawer and must not become another primitive path for raw
    approval, command, or rollback command payload text. Missing-detail
    queued-fix recovery actions must still provide the feature-owned Patrol
    briefing and request-local approval-required posture rather than opening the
    shared drawer as context-free generic Assistant chat. If a feature-owned
    expired-approval recovery action still has structured action artifact metadata,
    the shared drawer may receive only safe summary fields and command counts;
    raw command text remains outside shared Assistant primitives.
    When those feature-owned helpers attach backend model-only context, the
    drawer store may carry only bounded handoff text and structured resource
    references for the shared chat transport; approval, lifecycle, and command
    authority remain with the owning runtime surfaces.
    Patrol finding handoffs should still provide that briefing from current
    finding facts when a durable Patrol investigation record is not attached
    yet, rather than opening the shared drawer as empty generic chat.
    When the feature helper adds live approval state to the generic drawer
    briefing, it may pass only safe approval metadata into
    `AIChatContextBriefing`, including generated approval summaries and command
    counts when available; raw approval commands remain owned by the governed
    approval/remediation panels. If the generic finding-level helper hydrates
    latest investigation detail to recover action artifact context, it may pass only
    safe summary fields and command counts into the drawer briefing. Shared
    approval-required posture must derive its subject from that briefing or
    structured finding context, so Patrol handoffs render as Patrol handoffs or
    Patrol findings, and alert handoffs render as alert investigations, rather
    than generic dashboard briefs. Patrol approval-row Assistant prompts must
    route through the same
    feature-owned finding handoff helper rather than hand-written prompt-only
    drawer opens: safe approval metadata, action artifact summaries, resource
    references, and bounded `handoff_actions` may enter the prompt and context,
    but raw command text stays out and the scoped request must pass
    `autonomousMode:false` instead of changing the user's persistent Assistant
    control level. Patrol remediation-plan drawer handoffs must use the same
    primitive boundary: plan title/status/risk, step labels, and command counts
    may enter Assistant context; raw command and rollback command payloads must
    stay in the governed remediation/action panel. All Patrol finding
    discussion handoffs, including context-only findings without a live approval
    or proposed fix, must pass `autonomousMode:false` as a request-local
    override so the drawer shows approval-required posture without mutating the
    persistent Assistant control setting.
11. Keep shared filter primitives coherent with route-owned option hydration.
    Feature shells such as `frontend-modern/src/features/infrastructure/`
    must keep a route-owned canonical option visible in shared selects like
    `LabeledFilterSelect` even when current results do not contain that
    option, so provider-scoped handoffs do not flash back to `All`.
12. Keep the first welcome screen in
    `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
    explicit about operator context. The shell must explain that the bootstrap
    token only unlocks first-run setup, state where the command should run, and
    adapt command/help text to detected Docker or containerized deployments
    instead of assuming the operator already knows which host or container owns
    the Pulse install.
13. Keep the settings-shell infrastructure landing path aligned with that same
    first-session story. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
    must treat `/settings` and the infrastructure settings tab as the canonical
    path to the bare `/settings/infrastructure`, which renders the unified
    Connections table, not to a separate install subview or to reporting/
    control. The first-session story is owned by that table's own empty state
    and the `Add infrastructure` entry point on it, not by a second landing route,
    so first-time operators and returning operators see one consistent
    infrastructure surface by default.
14. Keep Infrastructure and Workloads onboarding copy on the shared
    presentation owner in
    `frontend-modern/src/utils/workloadEmptyStatePresentation.ts`. Both the
    infrastructure empty state and the Workloads no-resources state must route
    first-time operators into the canonical
    `/settings/infrastructure?add=pick` source picker, let operators choose by
    recognizable system/service names instead of collection-method taxonomy,
    and avoid falling back to either passive “nothing here yet” wording or the
    retired install-first / `Platform connections` split. Workloads routes that already
    have canonical unified-resource infrastructure sources but no workload
    inventory must use a distinct no-inventory presentation that points
    operators at credentials, permissions, and collection status in the
    canonical infrastructure workspace instead of reusing first-run onboarding
    copy.
15. Keep cross-surface investigation handoffs on shared route ownership.
    Feature shells such as Alerts and Patrol may decide which governed
    destination chips to render, but canonical href, label, dedupe, and
    infrastructure-fallback truth must stay in
    `frontend-modern/src/routing/resourceLinks.ts` instead of freezing raw
    route strings or provider-local link builders inside feature panels.
16. Keep shared summary-card emphasis coherent. When shared summary primitives enter an `inactive` state, `SummaryMetricCard`, `InteractiveSparkline`, and `DensityMap` must all demote background context together so storage, infrastructure, and workloads read as one interaction model instead of mixing page-local opacity, sticky-shell, or highlight rules.
17. Keep density-map summaries overview-first. When a shared summary density map receives row focus or chart-hover emphasis, `frontend-modern/src/components/shared/DensityMap.tsx`, `frontend-modern/src/components/shared/useDensityMapState.ts`, and `frontend-modern/src/components/shared/densityMapModel.ts` must preserve the multi-entity overview rows and keep focused-entity detail in the hover tooltip instead of swapping the card into a single-series chart, dimming the rest of the map into unusable background noise, duplicating cursor-value tooltip copy, or adding persistent card chrome that steals heatmap space. The card body must stay overview-first; the tooltip may carry the active entity identity, current value, and peak, shared tooltip shells must follow semantic surface tokens instead of forcing a dark palette in light mode, the tooltip header must let long entity names consume the available width before truncating rather than clipping against an arbitrary fixed label cap, numeric metric readouts such as `16.9 MB/s` or `37.4 MB/s` must stay single-line instead of wrapping the unit onto a second row, and density-map detail that cannot fit cleanly inside the canonical tooltip shell must be omitted rather than introducing tooltip-specific chrome or a secondary chart inside the hover surface.
18. Keep retired self-hosted hosted-model and trial acquisition surfaces out of
    normal v6 GA runtime. Shared shells and helper-driven badges may continue to
    parse legacy payload fields, but ordinary self-hosted Assistant, Patrol, and
    settings flows must present provider setup as BYOK/local/self-managed and
    must not surface hosted-model credits, in-app trial starts, or generic
    managed-model claims.
19. Keep sparkline scrubbing source-local and sibling-sync timestamp-based. The chart a user is actively scrubbing in `frontend-modern/src/components/shared/InteractiveSparkline.tsx` and `frontend-modern/src/components/shared/useInteractiveSparklineState.ts` must keep its dashed hover cursor on the real local mouse `x`, while sibling cards may map the shared hover timestamp onto their own timelines. Shared cursor sync must not snap the source chart back onto the nearest sample timestamp, the rendered SVG/canvas hover cursor must bind to the actual numeric cursor coordinate rather than a boolean guard state, the time cursor must span the chart viewport instead of collapsing to the series height, and the hover tooltip must track the pointer instead of anchoring to the chart top edge while following the active theme rather than a hardcoded dark shell. The hover tooltip must stay side-offset from the active scrub cursor and flip to the available side near viewport edges so it does not cover the highlighted guide or graph point.
20. Keep shared contextual focus canonical after adoption. Once a summary or table surface enters route-backed contextual focus, future additions must extend `frontend-modern/src/components/shared/contextualFocus.ts` and its guardrail tests rather than forking another helper for workload IDs, resource IDs, or scroll-preserving same-route selection.
21. Keep shared infrastructure/resource selectors on the canonical agent-facet
    truth. Shared primitives and settings-facing selector helpers must treat
    top-level TrueNAS appliances as agent-facet infrastructure via shared
    helper ownership instead of reviving a direct `resource.type === 'truenas'`
    branch inside page shells, selectors, or reporting-resource type helpers.
22. Keep shared feature-shell Patrol run fixtures on the canonical run-record
    contract. When `frontend-modern/src/features/patrol/` consumes Patrol run
    history, the shared normalized record must preserve provider-backed counts
    such as `truenas_checked` instead of letting feature-local fixtures or
    fallback objects collapse API-backed TrueNAS systems back into generic
    agent-host presentation.
    That same shared route-shell boundary also owns header-composition audit.
    `frontend-modern/scripts/header-audit.mjs`,
    `.github/workflows/release-dry-run.yml`, and
    `.github/workflows/create-release.yml` must prove the same shared
    top-level page-header contract before publication. The audit may follow
    local imports when a route shell composes `PageHeader` through a nested
    surface, and settings coverage must stay limited to top-level registry
    panels rather than every helper `*Panel.tsx` file. The canonical Settings
    shell therefore owns the shared `PageHeader` for support tools, and
    `frontend-modern/src/pages/Operations.tsx` must stay a redirect-only
    compatibility handoff instead of regrowing a second route-local heading,
    tab strip, or page shell for diagnostics, reporting, or logs. Because the
    dashboard route is retired, that audit must also discover live top-level
    pages from `src/pages/` and may not keep a hard required-header entry for
    `frontend-modern/src/pages/Dashboard.tsx`.
23. Keep the authenticated app root aligned with that same first-session path.
    That same shared-primitive ownership now includes contextual row focus.
    `frontend-modern/src/components/shared/contextualFocus.ts` is the canonical
    owner for interactive-series filtering, focused-label lookup, active-series
    resolution, and nearest-scrollable-ancestor preservation across page-scoped
    summary surfaces. Dashboard row focus, infrastructure summary emphasis,
    storage summary emphasis, and workloads summary emphasis must all route through
    that helper instead of maintaining page-local copies of the same hover/focus
    rules.
    `frontend-modern/src/App.tsx` must land `/` on the Infrastructure route and
    let the governed Infrastructure empty state route first-time operators into
    the `Add infrastructure` source picker, instead of preserving a separate
    root-only compatibility shell or an agent-only install jump that drifts from
    the rest of the onboarding contract.
    The authenticated app shell's boot-time route preloads must be owned by
    `frontend-modern/src/routing/routePreload.ts` so top-level cold-tab
    readiness cannot drift from the route-module preloader. Workloads,
    Recovery, Patrol, Alerts, Storage, and Settings are part of that shared
    preload contract.
    Route-module preloads and idle chart-cache prewarming are separate shell
    responsibilities: the shared route preload inventory must stay module-only,
    while chart payload warming must route through the owning summary-cache
    utilities instead of mounting hidden pages or adding page-local boot logic.
    The same entry-shell contract must also canonicalize authenticated
    `/login`: once auth succeeds, the shared shell must resolve that route back
    onto the governed Infrastructure landing path instead of rendering a page-local
    not-found state inside the authenticated chrome.
24. Keep relay settings shell copy on the shared presentation owner in
    `frontend-modern/src/utils/relayPresentation.ts`. The route metadata in
    `settingsHeaderMeta.ts` and the leading `SettingsPanel` in
    `RelaySettingsPanel.tsx` must reuse the same description and availability
    copy instead of drifting into separate rollout or pairing wording. Relay
    availability copy must describe the Relay tier boundary as Relay and higher
    plans rather than collapsing Remote Access back into a Pro-only feature.
25. Keep shared settings-shell legal and docs referrals on
    `frontend-modern/src/utils/docsLinks.ts`. Shared settings surfaces such as
    `AIRuntimeControlsSection.tsx` must not hardcode GitHub `main` doc URLs for
    privacy, security, proxy-auth, scope-reference, or Terms-of-Service links.
26. Keep shared settings-shell telemetry transparency controls on the governed
    general settings panel. Preview/reset affordances for anonymous telemetry
    must stay rendered inside
    `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
    instead of drifting into route-local modals, hidden dev tools, or shell
    chrome that operators would not naturally inspect.
27. Keep the short telemetry/privacy summary copy on that same shared surface
    accurate to the governed privacy doc. If the trust boundary depends on a
    specific retention window or on “IP addresses are not stored” rather than
    “IPs are never seen,” the summary copy in
    `GeneralSettingsPanel.tsx` must state those facts plainly instead of
    reverting to a stronger but inaccurate shorthand.
28. Keep maintainer commercial-event controls out of customer settings.
    The shared general settings privacy panel may expose anonymous outbound
    telemetry controls, preview, and reset affordances, but it must not render
    local commercial handoff event toggles, `PULSE_DISABLE_LOCAL_UPGRADE_METRICS`,
    or other commercial-debug controls as normal customer-facing preferences.
29. Keep shared storage-route feature presentation on neutral capability truth.
    Reusable mappers and presenters in `frontend-modern/src/features/storageBackups/`
    must distinguish inventory datastores from backup repositories so VMware
    rows on the shared storage route stay canonical to the admitted phase-1 floor instead of
    reviving backup-target, protected-target, or recovery-local semantics on a
    shared page.
30. Keep infrastructure settings-shell API alternatives on the shared shell
    contract. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
    `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`, and
    `frontend-modern/src/components/Settings/settingsNavigationModel.ts` must
    present the unified add flow as the canonical API-backed entry for
    Proxmox, TrueNAS, VMware, and future provider integrations instead of
    reviving top-level `Direct Proxmox` wording or shell-local provider
    routes. Phase 9 retired the `Platform connections` nomenclature along
    with the shells that owned it — there is no `PlatformConnectionsWorkspace`
    and no per-type `ProxmoxSettingsPanel` / `TrueNASSettingsPanel` /
    `VMwareSettingsPanel` to route through; the provider is a field inside
    one `ConnectionEditor`, not a destination.
31. Keep the infrastructure settings connection inventory on one shared
    source. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
    composes rows exclusively from
    `frontend-modern/src/components/Settings/useConnectionsLedger.ts`,
    which polls `GET /api/connections`. Provider connection counts and
    availability must derive from that aggregator, not from a top-level
    ledger plus parallel provider-specific fetches. The retired
    `PlatformConnectionsWorkspace` / `TrueNASSettingsPanel` /
    `VMwareSettingsPanel` panels must not be reintroduced as a second
    fetch path.
32. Keep alert-history feature composition on the current owned state contract.
    `frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` must react to the
    shared `alertData()` history state instead of reviving deleted aliases, and
    it must pass unified-resource resolution through to
    `frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx` so
    the panel can render shared route chips without creating another page-local
    resource lookup or provider-specific handoff layer.
33. Keep the alert-thresholds containers surface on the canonical shared owner.
    `alertOverridesModel.ts`, `useAlertOverridesState.ts`, and
    `useAlertsConfigurationState.ts` must surface API-backed `app-container`
    parents such as TrueNAS as first-class `Container Runtimes`, while
    `ThresholdsTab.tsx` must bridge function-valued selectors into
    `ThresholdsTable.tsx` explicitly instead of relying on spread-based adapter
    props that can collapse functions on the live Solid surface. Docker-only
    controls in `ThresholdsTableDockerTab.tsx` must remain gated to real
    `docker-host` resources instead of leaking onto platform-managed runtimes.
34. Keep shared commercial upgrade navigation typed and destination-aware.
    Shared paywall shells and upgrade actions must route internal billing or
    cloud destinations through `frontend-modern/src/utils/upgradeNavigation.ts`,
    `frontend-modern/src/components/shared/UpgradeLink.tsx`, and
    `frontend-modern/src/components/shared/useUpgradeNavigation.ts` instead of
    guessing from labels, hardcoding `target="_blank"`, or calling
    `window.open(...)` from each feature surface.
35. Keep same-shell infrastructure route transitions on retained shared state.
    `frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
    may show its full-page loading shell only before the first compatible
    resource snapshot exists; once a fresh canonical snapshot is already
    present in the shared app shell, top-level tab switches must reuse that
    state boundary instead of flashing a transient infrastructure page
    takeover between tabs.
36. Keep self-hosted paid-service prompts opt-in at the shared shell layer.
    `settingsNavCatalog.ts`, `settingsNavVisibility.ts`, shared upgrade link
    primitives, trial banners, monitored-system warning banners, history-lock
    overlays, and Patrol lock helpers must honor `presentationPolicy.hideUpgrade`
    by hiding paid prompts by default on ordinary self-hosted installs. Direct
    activation/recovery routes may render their owned content, but sidebar
    discovery, trial CTAs, plan upsells, monitored-system limit pressure, and
    feature upgrade links must require hosted mode, explicit handoff, or active
    entitlement. Cloud interest links from self-hosted plan surfaces must hand
    off to Pulse Account/public Cloud ownership rather than route to an
    in-product Cloud trial/signup page.
37. Keep the identified-service reducer on `discoveryPresentation.ts`. Any
    surface that wants to label a workload with the AI-identified service
    (drawer overview card, future row chips, MCP capability payloads) must
    consume `getDiscoveryIdentifiedSummary` rather than re-implement the
    empty/low-signal gate. The helper returns null when the stored record
    has no useful identification — mirroring the Discovery tab's
    `hasValidDiscovery` — so the same record either renders in all
    surfaces or hides in all surfaces, preventing "Unknown" rows or
    zero-confidence noise from drifting into peripheral UI.
    Discovery is an opt-in observed-context layer, not an automatic row-link
    owner. The reducer must carry provenance, observed time, service version,
    endpoint candidates, and URL-source copy so drawer surfaces can show
    "Observed by Discovery" context and pass suggested URLs into the shared
    web-interface field. Persisted/manual web-interface metadata remains the
    only row-link source until the operator explicitly adopts a suggested URL.

## Current State

Cross-jump chip strips on alert and Patrol surfaces were retired on
2026-05-16 alongside the platform-first migration. The
`buildResolvedResourceSurfaceLinks` and `buildResourceSurfaceLinksForResource`
helpers (and the per-surface builders for Infrastructure / Workloads /
Storage / Recovery hrefs) were deleted from
`frontend-modern/src/routing/resourceLinks.ts`; the alert resource-incidents
panel and Patrol findings panel that consumed them now keep investigation
in-place through their existing handoff buttons and inline actions. Future
cross-surface drilldown chips must not reanimate the legacy helpers.

Command palette and keyboard shortcuts moved to platform-first on 2026-05-16
(`frontend-modern/src/components/shared/commandPaletteModel.ts`,
`frontend-modern/src/components/shared/useCommandPaletteState.ts`,
`frontend-modern/src/components/shared/KeyboardShortcutsModal.tsx`,
`frontend-modern/src/hooks/useKeyboardShortcuts.ts`,
`frontend-modern/src/routing/routePreload.ts`,
`frontend-modern/src/routing/navigation.ts`). The legacy `nav-infrastructure`,
`nav-workloads`, `nav-storage`, and `nav-recovery` palette entries — together
with the `g i` / `g w` / `g s` / `g b` chord bindings — were retired and
replaced with `nav-proxmox`, `nav-docker`, `nav-kubernetes`, `nav-truenas`,
`nav-vmware` (chords `g p` / `g d` / `g k` / `g n` / `g v`) plus a dedicated
`nav-kubernetes-pods` entry that lands on `/kubernetes/pods`. The shell
preload set and `getActiveTabForPath` matcher no longer recognize the legacy
top-level routes. New palette commands and shortcut chords must therefore
anchor on canonical platform routes and must flow through the same platform
visibility model as primary navigation; do not reintroduce hidden platform
families or top-level Infrastructure / Workloads / Storage / Recovery entries
by reanimating the legacy paths.

The shared table chrome now allows `TableCardHeader` to expose a right-aligned
action slot, currently used by the Workloads/Proxmox metric display control.
That slot belongs to the table header band and must not reintroduce nested
cards or page-local toolbar wrappers inside `TableCard`. Proxmox host grouping
also extends the shared `NodeGroupHeader` row pattern: host metrics may align
with workload table columns, but the shared primitive owns the header/table
shell boundary rather than platform pages copying their own card headers.
Compact PVE version text in that header must come from the shared Proxmox
version formatter so raw `pve-manager/...` payloads and platform-page host
version cells stay consistent.
Mobile navigation now recognizes `proxmox` as a first-class platform tab in
the shared priority model so app-shell ordering remains centralized.

`ResourceOperatorStateSection.tsx` on the resource detail drawer
overview tab uses `createNonSuspendingQuery` to fetch
`/api/resources/{id}/operator-state` so the drawer's parent
Suspense boundary does not flicker the page-level "Loading view…"
fallback while operator-set state is in flight. New self-fetching
sections inside the drawer must follow the same pattern (or wrap in
their own local Suspense) rather than relying on `createResource`,
which propagates suspension to the closest ancestor.

The Patrol page header copy lives in a single canonical helper at
`frontend-modern/src/utils/patrolPagePresentation.ts`. The page-title
tooltip on `PatrolIntelligenceHeader.tsx` must read from
`PATROL_PAGE_TITLE_TOOLTIP` exported alongside the description rather
than carrying an inline copy, so hover and inline never drift apart on
what Patrol actually owns: scheduled probing, context assembly for the
configured model, and approval-bound action.
The same `PatrolIntelligenceHeader.tsx` shell also renders a compact
trust-at-a-glance summary directly under the page title (a
render-only consumer of `state.patrolStatus()?.trust`), gated on at
least one non-zero trust signal so fresh installs render no header
strip. The detailed breakdown stays in
`PatrolIntelligenceWorkspace.tsx` for the canonical view; the header
line is the entry-point summary so operators see active, regressed,
and verified-fix counts before scrolling into the workspace tabs.
The recency line beside the header actions also renders coverage
alongside time when the canonical `getPatrolRecencyPresentation` helper
returns `resourcesCheckedLabel` from the latest completed run. Render code
must gate on `<Show when={recency().resourcesCheckedLabel}>` (truthy) so
zero-coverage runs do not surface a misleading coverage phrase, failed or
scoped runs use neutral checked wording, and only successful full patrols read
as verified. The primary Patrol assessment shell must pass the same run-history
facts into `getPatrolAssessmentPresentation` so assessment coverage caveats do
not contradict the header's verified full-run coverage state.

`frontend-modern/src/utils/discoveryPresentation.ts` owns resource discovery
command guidance targets. Discovery surfaces that need to tell operators where
to enable command execution or verify `agent:exec` scope must use that helper's
canonical `Settings → Infrastructure` and `Settings → API Access` handoffs
instead of hard-coding legacy settings labels or old route paths.
Shared frontend empty states, thresholds empty states, and discovery guidance
that mention the Infrastructure settings destination now consume
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts` for the
canonical `Settings → Infrastructure` label and source-strategy copy. Shared
primitives must not fork that string or revive removed nested route labels.
The shared Assistant drawer owns compact source-named approval posture for
governed handoffs. Patrol handoffs render as Patrol, and alert plus alert
incident timeline handoffs render as alert investigations rather than dashboard
briefs. Those same drawer handoffs may carry model-only chat context and
resource references to the backend, but the drawer remains a presentation and
transport owner rather than the source of approval or execution truth. Patrol
briefings must stay simple: source, status, one primary subject, and an optional
safe route link. They must not render remediation step lists, evidence chips,
command summaries, or suggested-prompt chips as drawer chrome. If a
feature-owned briefing includes a safe route-owned `actionHref`, the drawer may
render the briefing action label as a normal app link; that link is navigation
guidance only and must not become approval or execution authority.

`SettingsTab` no longer includes `infrastructure-connections` or
`infrastructure-install`. The single `infrastructure-systems` entry in
`settingsNavCatalog.ts`, `settingsPanelRegistry.ts`, and
`settingsNavigationModel.ts` replaces both. Panel routing within the
infrastructure area uses `InfrastructurePanelStep` in-page state.
The shared monitored-system warning banner has been retired. Ordinary hosted
and self-hosted sessions must not render app-shell monitored-system capacity
warnings, plan-review links, or upgrade-impression telemetry from stale finite
policy data.
Shared alert presentation surfaces (`OverviewTab.tsx`, `HistoryTab.tsx`,
`AlertOverviewActiveAlertsSection.tsx`, `AlertHistoryTableSection.tsx`,
`AlertHistoryTableAlertRow.tsx`, `AlertOverviewAlertCard.tsx`) no longer accept
`hasAIAlertsFeature` or `runtimeCapabilitiesLoading` props. Feature gating for
AI alerts flows through the shared entitlements layer; surfaces must not
re-introduce per-surface capability fetch props.
Recovery's retired posture-card strip remains outside the shared
hover-synchronization dialect. Per the Extension Points constraint, Recovery
must not introduce a new summary-card component with row/group/chart hover
wiring without a separate governed product decision.
`frontend-modern/src/components/Storage/useStorageSummaryCharts.ts` now owns
the reusable polling/caching state for storage summary history, while
`frontend-modern/src/features/storageBackups/storageCapacityDeltaPresentation.ts`
keeps pool-growth label/tone formatting inside the shared feature presentation
layer. The storage page must keep reusing those shared owners instead of
rebuilding storage-history timers or byte-delta formatting inside row
components.
That same shared alerts feature boundary now also owns legacy shared-storage
override migration. `frontend-modern/src/features/alerts/alertOverridesModel.ts`
and `frontend-modern/src/features/alerts/useAlertOverridesState.ts` must
canonicalize per-node shared-storage override keys such as
`Main-pve1-ceph-pool`, hashed `/api/resources` storage ids, and Ceph pool
storage rows onto the storage metrics target id before the thresholds table
derives rows, so old Ceph override records and newly projected Ceph pool
overrides survive the v6 feature-shell path instead of silently disappearing
from the live editor.

The frontend already has several guardrail tests. The next step is to keep
turning repeated local patterns into explicit shared primitives with hard usage
bounds, including provider-backed alert-history wording. `frontend-modern/src/features/alerts/helpers.ts`,
`frontend-modern/src/features/alerts/tabs/HistoryTab.tsx`, and
`frontend-modern/src/features/alerts/OverviewTab.tsx` must present VMware-
backed host and VM incidents with the shared `resource-incident` vocabulary
and existing alert-history shells instead of introducing VMware-only labels,
badges, or panel copy just because the underlying signal came from vSphere.
That same shared settings and modal boundary now also owns the public usage-data
vocabulary. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/useSystemSettingsState.ts`, and
`frontend-modern/src/utils/systemSettingsPresentation.ts` must present one
explicit `Usage data and privacy` model centered on `Anonymous outbound
telemetry`; maintainer commercial-event controls, upgrade-metrics labels, and
sales/onboarding reporting language must not appear in customer-facing Settings
or support diagnostics, and public configuration docs must not list their
internal compatibility switches as ordinary operator settings. Customer
frontend code must also not import, define, or call `upgradeMetrics`,
`conversionEvents`, infrastructure onboarding metrics wrappers, or POST those
events to `/api/upgrade-metrics/events`.
The telemetry copy must describe normalized release identity rather than
falling back to ambiguous `telemetry`, `upgrade metrics`, or raw-version
wording.
Shared table, disclosure, and form primitives must also stay explicitly typed
at the browser edge. Summary rows may memoize repeated pending-update reads,
shared buttons must preserve discriminated disclosure props, toggle and a11y
helpers must expose exact event signatures, shared rows must accept typed
`data-*` props, and reporting-panel helpers must remain ES2020-safe instead of
depending on feature-local casts or newer string helpers.
That same shared settings-shell and banner boundary now also owns demo-mode
commercial suppression. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
`frontend-modern/src/components/Settings/settingsNavVisibility.ts`,
`frontend-modern/src/stores/sessionCapabilities.ts`,
`frontend-modern/src/stores/sessionPresentationPolicy.ts`,
`frontend-modern/src/stores/demoMode.ts`,
`frontend-modern/src/stores/license.ts`,
`frontend-modern/src/stores/licenseCommercial.ts`,
`frontend-modern/src/useAppRuntimeState.ts`,
`frontend-modern/src/components/shared/HistoryChartOverlay.tsx`,
`frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`, and
`frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
must consume one shared bootstrap truth from `/api/security/status`. The
backend capability fact `sessionCapabilities.demoMode` remains part of that
payload, but the browser-owned shared primitive is now the resolved
`sessionPresentationPolicy` contract, which hides billing tabs, trial nudges,
monitored-system warning banners, dashboard upsells, Patrol upgrade CTAs,
history-lock paywalls, and other public-demo commercial affordances when the
browser is rendering a public demo runtime.
That same shared settings-shell boundary also owns demo-mode organization
suppression. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`,
`frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
`frontend-modern/src/components/Settings/settingsNavVisibility.ts`,
`frontend-modern/src/stores/sessionPresentationPolicy.ts`, and
`frontend-modern/src/useAppRuntimeState.ts` must fail closed on organization
navigation and app-shell org chrome until the resolved presentation policy is
known, then keep org switchers, visible `Default Organization` labels, and
organization-scoped settings groups hidden when the browser is rendering a
public demo runtime.
Shared primitives must not perform their own ad hoc `/api/health` polling,
response-header inference, hostname heuristics, or per-banner demo branching;
the runtime bootstrap, shared presentation-policy store, and shared banner
hooks stay on one canonical owner so suppression stays coherent across
customer-facing surfaces.
That same shared primitive boundary now also treats runtime capability reads
and commercial reads as separate stores. Shared settings shells and banner
hooks may read feature truth from `frontend-modern/src/stores/license.ts`, but
commercial identity, upgrade routing, and trial state must stay in
`frontend-modern/src/stores/licenseCommercial.ts`, which suppresses public-demo
loads locally and defers its first fetch until the presentation policy has
resolved instead of depending on route-local guards.
That same shared primitive boundary now also centralizes authenticated-shell
commercial posture bootstrap. `frontend-modern/src/useAppRuntimeState.ts`
owns the first shared `loadCommercialPosture()` read after authenticated app
runtime has mounted, while `frontend-modern/src/AppLayout.tsx`,
`frontend-modern/src/components/Settings/Settings.tsx`, Patrol state hooks,
and settings-panel state hooks must consume the
resolved store state instead of reissuing mount-time posture fetches from each
surface. Shared commercial posture loading may still dedupe or force-refresh
through the store for governed billing or first-run flows, but route-local or
panel-local bootstrap ownership is forbidden.
Storage disk drawers now also sit on that same shared-primitives floor.
`frontend-modern/src/components/Storage/DiskDetail.tsx` must render physical-
disk read, write, and busy charts through `HistoryChart` plus
`useHistoryChartState`, using the canonical physical-disk history resource id,
instead of reviving `diskMetricsHistory`, a page-local ring buffer, or another
storage-only live chart primitive for the same telemetry.
That same shared-primitive floor now also owns upgrade-navigation semantics.
`frontend-modern/src/utils/upgradeNavigation.ts` is the canonical typed
internal-vs-external destination helper, while
`frontend-modern/src/components/shared/UpgradeLink.tsx` and
`frontend-modern/src/components/shared/useUpgradeNavigation.ts` own how shared
paywall surfaces navigate those destinations. Feature shells may request a
commercial destination, but they must not re-decide whether that destination
opens in-app or in a new tab once the shared primitive exists.
That same shared-primitive floor now also owns prerelease shell guidance.
`frontend-modern/src/AppLayout.tsx` is the canonical authenticated-shell owner
for prerelease presentation, and the remaining user-facing treatment is the
compact `Preview` badge keyed from resolved release metadata. Feature pages,
settings panels, shared components, and route-local shells must not add a
second release-candidate banner, hardcoded GitHub release or feedback links,
or page-local prerelease notices once that shared shell contract exists.
Browser proof for that shell rule now lives in
`tests/integration/tests/57-release-candidate-shell.spec.ts`, which must keep
rc-channel builds banner-free while preserving the compact preview badge.

The subsystem registry now also requires explicit proof-policy coverage for all
shared runtime files, and shared-component guardrails fail if raw table
composition is reintroduced in new shared components outside the canonical
allowlist.
Retained-value query ownership is now part of that shared floor too.
`frontend-modern/src/hooks/createNonSuspendingQuery.ts` is the canonical
shared helper for page-local fetches that must stay inside the mounted
surface instead of falling through the app-level `Loading view...` fallback.
Feature slices such as recovery and infrastructure drawers may consume that
helper, but they must not fork new suspense-escape helpers once the shared
contract exists.
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
That same contract also includes the locked teaser copy itself. The reporting
catalog owns report-builder identity once the feature is available, but a locked
session must render neutral feature-gate copy from the reporting panel state so
Community users do not see enabled report-builder language before advanced
reporting is available.
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
runtime state instead of surfacing stale auxiliary counters as primary status.
Legacy hosted-model credit counters may remain parseable on transport payloads,
but shared Patrol headers must not render them as customer-facing badges in the
normal self-hosted GA app.
The same shared shell rule applies to retired hosted availability copy: when an
older backend payload describes hosted model activation, credit exhaustion, or
account-backed AI availability, shared feature shells must normalize the
operator-facing guidance back to provider setup or local model setup rather than
rendering the retired offer.
The same primitive boundary now also owns the first AI enable control in
`AISettings.tsx`: the primary toggle must remain explicitly addressable with a
stable accessible label and route unconfigured installs into the canonical
provider setup modal instead of falling back to generic "first pressed toggle"
selectors, provider-model-load heuristics, legacy hosted-model enablement, or
in-app trial acquisition.
That same route-owned presentation rule also governs Patrol findings empty
states: shared section shells under `frontend-modern/src/features/patrol/`
must not render a green healthy empty state from `0 active findings` alone
when the owning Patrol runtime or overall-health summary is degraded, blocked,
or not fully verified.
The same hierarchy also applies inside the Patrol summary shell: once the
primary assessment strip states Patrol's current risk and verification basis,
supporting metrics under that strip must stay metric-oriented and must not
repeat assessment or verification labels as a second compact verdict row.
The collapsed Patrol assessment strip itself must remain a compact readout
rather than a headline-plus-paragraph block; explanatory assessment and
recommendation copy belongs in the owning Findings, Runs, Supporting context,
or Assistant chat surfaces rather than a normal-path summary details expansion.
That readout should lead with current operator state and score rather than
mixing a reassuring grade label with issue-state copy in the same line.
That same summary shell should also keep the shared Pulse surface neutral:
severity belongs in compact accents, inline readouts, and badges rather than
turning the whole assessment into a tinted warning banner, nested card, or
hero-style block that breaks the surrounding operator workflow.
That same summary-shell rule also applies to timing metadata: if the header,
verification card, or findings footer already presents the governed Patrol
activity timestamp, the summary chip row must not add another recency badge
that competes with those owned timing surfaces.
The same ownership split applies to supporting counts: if the Patrol summary
surface renders the metric strip for active findings, warnings, criticals, and
fixes, the primary assessment strip should not repeat those same counts in badge
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
canonical tag-badge primitive. Workload rows and the unified-resource detail
drawer must import that shared owner instead of keeping a workload-local tag
badge variant or importing a feature-local path into infrastructure
surfaces.
That same owner now also holds the CSP-safe tag-dot rendering contract: tag
color and active-state emphasis must travel through SVG fill/stroke attributes
or stable classes, not inline `background-color`, `box-shadow`, or other
`style=` mutations that break the hosted demo CSP.
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
Self-hosted trial CTA removal is now part of that same primitive boundary for
settings and shared paywalls. Shared/settings runtime owners may derive neutral
plan and entitlement posture from the canonical commercial contracts, but
ordinary self-hosted feature gates must not present in-app trial starts,
trial-status banners, or trial-specific rate-limit copy. Operator-facing paid
handoff remains limited to explicit Plans, hosted, activation, recovery, or
support surfaces; feature gates may show neutral "View plans" links through
`frontend-modern/src/utils/upgradePresentation.ts` only where presentation
policy allows commercial discovery.
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
The infrastructure feature state owner may opt into websocket-first unified
resource hydration only when it also schedules canonical REST revalidation
after the first-paint settle window; shared route composition must not re-route
the table through a blocking resource fetch just to confirm infrastructure that
the realtime store has already reported. Authenticated cold starts must render
from retained realtime or unified-resource state without falling back to
first-run/welcome posture or replaying stale setup success notifications, and
background revalidation may update rows in place but may not blank the page.
Realtime resource adapters must defensively coalesce split host identities by
the same source-bridge rule as the API boundary so a transient backend rebuild
cannot surface duplicate infrastructure rows while the next canonical REST
snapshot is settling.
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
The shared infrastructure summary row may consume alert-backed metric
thresholds from the table state, but threshold selection itself remains
alerts-owned through `frontend-modern/src/stores/alertsActivation.ts` and
`frontend-modern/src/utils/metricThresholds.ts`; shared primitive cells and rows
must only pass resolved warning/critical values into metric presentation.
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
That same sparkline boundary now also owns floating tooltip shell routing:
local hover tooltips must derive viewport anchor coordinates from the shared
runtime/model path, keep the tooltip beside rather than on top of the scrub
cursor, and render through
`frontend-modern/src/components/shared/TooltipPortal.tsx`, not as HTML
`foreignObject` shells inside the `preserveAspectRatio="none"` chart SVG where
cross-browser scaling can stretch the tooltip surface or drop its semantic
shell styling.
That same shared sparkline boundary now also owns active-series isolation
metadata. The shell may expose `data-active-series-display` and
`data-rendered-series-count` for proof and inspection, but only the shared
runtime/model owners may decide whether a hovered or focused series is merely
emphasized or fully isolated; feature shells must not fork their own row-hover
line filtering.
The retired dashboard overview route must not regain feature-local trend,
KPI, problem-resource, or card shells. Workload-table and guest-row fallback
copy that lives under `frontend-modern/src/components/Workloads/` must keep
using `frontend-modern/src/utils/workloadEmptyStatePresentation.ts` and
`frontend-modern/src/utils/workloadGuestPresentation.ts`. New route-level empty
states, tone mapping, or compact issue copy must extend the shared
`emptyStatePresentation`, `semanticTonePresentation`, and
`problemResourcePresentation` helpers instead of reviving deleted
dashboard-only KPI, metric, storage, recovery, or trend presentation helpers.
That shell must also stay passive with respect to data ownership: future
overview trend cards may render summary-range controls and operator-facing
empty or error copy only after they have a governed owner, and they must not
reintroduce route-local metrics-history fetch loops for CPU and memory
sparklines when the canonical infrastructure summary surface already owns the
chart contract.
The shared density map now follows that same owner split.
`frontend-modern/src/components/shared/DensityMap.tsx` stays the render shell,
`frontend-modern/src/components/shared/useDensityMapState.ts` owns hover
signals, canvas draw lifecycle, and resize handling, and
`frontend-modern/src/components/shared/densityMapModel.ts` owns bucket/window
math, hover target selection, focused-series tooltip detail, and density-cell
opacity rules. Future density-map work should extend those owners instead of
pushing canvas lifecycle, tooltip shaping, or chart math back into the shared
shell.
The shared trial banner is retired for self-hosted v6 GA. Future commercial
notification work must start from the explicit Plans, hosted, activation,
recovery, or support surfaces rather than reviving a global authenticated-shell
trial banner.
The shared column picker now follows that same owner split.
`frontend-modern/src/components/shared/ColumnPicker.tsx` stays the render
shell, `frontend-modern/src/components/shared/useColumnPickerState.ts` owns
dropdown open state and outside-click listener lifecycle, and
`frontend-modern/src/components/shared/columnPickerModel.ts` owns hidden-column
count, reset visibility policy, and column-option text-class/copy policy.
Column-picker trigger badges must describe what the count means, such as
`N hidden`, rather than exposing a bare number or ratio that competing table
surfaces can interpret differently. Shared column-picker tests must cover that
copy alongside the owner split so governed product tables do not regress to
ambiguous utility badges.
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
Filter-toolbar segmented controls must delegate to this primitive rather than
calling `segmentedButtonClass` directly, and icon+text labels must render as
one inline-flex button label so compact bars keep the v5 single-line control
language across Type/Status, grouped/list, bars/trends, columns, and reset.
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
body-scroll lock, previous-focus restoration, shared blocking-dialog
visibility, and backdrop-close runtime, and
`frontend-modern/src/components/shared/dialogModel.ts` owns focusable-element
lookup plus layout and panel class policy. Future dialog work should extend
those owners instead of pushing focus-trap lifecycle or layout policy back into
the shared shell.
App-shell consumers such as `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` may read that shared blocking-dialog state
to suppress background affordances, but they must not reimplement their own
parallel modal-stack bookkeeping.
The shared history chart now follows the same owner shape.
`frontend-modern/src/components/shared/HistoryChart.tsx` stays the render
shell, `frontend-modern/src/components/shared/useHistoryChartState.ts` owns
license gating, history fetch/refresh, canvas draw lifecycle, and hover state,
and `frontend-modern/src/components/shared/historyChartModel.ts` owns tooltip
formatting, scale and axis math, and closest-point selection. Lock overlays in
ordinary self-hosted surfaces must stay informational rather than presenting
trial-start or upgrade-link actions. Future history-chart work should extend
those owners instead of pushing fetch, license, commercial trial actions, or
canvas math back into the shared component shell.
The shared history range catalog is also owned here. The canonical product
range sequence is `24h`, `7d`, `14d`, `30d`, and `90d`, with `14d` preserved
as the Relay entitlement surface rather than hidden behind the Pro-only
long-range controls. Lock copy must derive its target days and tier label from
the selected range instead of assuming every locked history selection is a
30-day or 90-day Pro ask.
The remaining header, overlay, and tooltip render surfaces now live in
`frontend-modern/src/components/shared/HistoryChartHeader.tsx`,
`frontend-modern/src/components/shared/HistoryChartOverlay.tsx`, and
`frontend-modern/src/components/shared/HistoryChartTooltip.tsx` instead of
re-accumulating those sections inline in the shell.
That tooltip owner now also holds the CSP-safe hover contract: chart tooltips
must render inside the chart surface with model-owned layout and SVG/attribute
positioning, not through fixed portals or inline `left`/`top` style attributes
that violate the public demo CSP.
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
of pushing tab-order or DOM lifecycle logic back into the shared shell. With
support/admin tools moved under Settings, that utility ordering must no longer
reserve a standalone `operations` slot; alerts, Patrol, and Settings are the
remaining authenticated utility tabs.
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

Pages that filter a list-of-resources surface (Infrastructure, Workloads,
Storage, Recovery Protection coverage, Recovery events) compose the chip-based
`frontend-modern/src/components/shared/FilterBar/FilterBar.tsx` shell instead
of `PageControls`. Each page declares a `FilterDef[]` catalog (label, options,
value, defaultValue, group); `FilterBar` renders chips for active filters and
exposes the rest behind a "+ Filter" menu, with type-ahead at both the menu
and chip popovers (`AddFilterMenu` and `FilterChip`). View options
(grouping segmented control, charts toggle, columns picker, sort key) sit in
the `viewOptionsTrailing` slot and are not chips. Recovery is event-first and
does not use equal workspace subtabs for protected rollups versus event
history; Storage subtabs (Pools / Physical Disks) sit above the bar as
navigation, not filters.
Primary filters with small, stable option sets should stay one-click controls
inside that same `FilterDef[]` catalog by setting `inline: true`; `FilterBar`
renders those as unlabeled compact segmented controls in the same second-row
rail as view options, matching the v5 filter-bar pattern, and keeps longer or
dynamic scope filters in the menu/chip path. Feature surfaces must not fork
local filter rows or bury high-frequency Type, Status, or Group-by filters
behind an extra menu just to regain one-click behavior.
Pages that have not yet migrated (the alert-history filter card,
Kubernetes deployments drawer) keep using `PageControls` and
`LabeledFilterSelect`, but new resource-list filter surfaces should reach for
`FilterBar` with a catalog rather than reintroducing a per-page select row.

Pages may opt into saved views by passing `savedViewsKey` to
`FilterBar`. The `useSavedViews` hook owns the localStorage IO + URL
navigation (`pulse:filterbar:saved-views:<key>`); `SavedViewsMenu` owns
the dropdown chrome. A "view" is the page's URL query string at save
time, so saved views double as shareable links: copying the bar URL
after applying a view gives someone else the exact filtered state.
Implicit "remember last filters" is intentionally not added — defaulting
to yesterday's filter state on a monitoring page hides real problems.
That same shared filter-toolbar boundary also owns controlled select continuity
when filter options materialize asynchronously. `LabeledFilterSelect` must keep
the caller-owned `value` visibly selected after option children arrive so
dashboard, recovery, and other canonical filter bars do not drop their active
selection until the operator reopens the control. The same primitive must keep
its `<label for>` association reactive when a route-owned filter swaps the
select `id`, label, and option set in place, so controls such as the workloads
node/K8s cluster filter remain accessible after mode changes.
That same boundary also owns live option propagation through shared page-control
composition. Callers such as storage and recovery must pass source/filter
option collections through reactive accessors instead of snapshot arrays when
those options depend on post-load unified-resource state, so the shared toolbar
can reconcile late-arriving options and preserved route selections without
requiring page-local reset hacks.
Shared default filter labels must also stay on the same primitive-level
contract. Generic `All …` option text should route through
`frontend-modern/src/components/shared/filterOptionPresentation.ts`, with
domain presentation helpers supplying the noun phrase, so storage, alerts,
recovery, settings, and future filter bars do not drift between title-case and
sentence-case local strings.
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
Canonical customer disclosures inside shared shells route through
`frontend-modern/src/utils/docsLinks.ts`, so settings privacy links resolve to
shipped `/docs/...` assets instead of hard-coded GitHub `main` URLs that can
drift from the running build.
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
That same tooltip owner now also holds the CSP-safe portal contract: shared
tooltip shells must render through SVG/attribute positioning and viewport-
clamped layout helpers rather than fixed inline `left`/`top` style attributes.
When a shared portal tooltip is already visible, that same owner must
reschedule positioning on live coordinate and viewport changes so chart hover
tooltips keep following the active pointer instead of sticking to their first
anchor.
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
runtime logic inline. Audit-log filter option labels must come from
`frontend-modern/src/utils/auditLogPresentation.ts` and the shared filter-option
label primitive instead of hard-coded title-case strings in the settings shell.
When the shared runtime-capabilities store reports `paid_runtime_required` for
`audit_logging`, Settings navigation must keep the Audit Log surface reachable
and the panel must render paid-runtime-required copy with the private Pulse Pro
download action. The runtime mismatch is not a plan upsell and must not be
hidden by ordinary missing-feature navigation filtering.
That shared filter-option primitive is also the canonical owner for default
`All <scope>` option wording wherever a product surface exposes filter selects
or segmented filter choices. Workloads filters, storage source
filters, recovery history and platform/type filters, Kubernetes namespace
drawers, resource-change timeline filters, and alert configuration options must
call `frontend-modern/src/components/shared/filterOptionPresentation.ts` through
their nearest presentation/model owner instead of hard-coding page-local `All
...` labels.

The audit webhook settings surface now follows that same owner split.
`frontend-modern/src/components/Settings/AuditWebhookPanel.tsx` stays the
canonical `SettingsPanel` shell, while
`frontend-modern/src/components/Settings/useAuditWebhookPanelState.ts` owns the
license/paywall lifecycle, webhook fetch/save flow, validation, paywall
tracking, and hidden-upgrade copy posture. The shell must not re-accumulate API
calls or paywall tracking inline.
The same paid-runtime-required route applies to Audit Webhooks: missing
`audit_logging` caused by a community runtime must keep the panel reachable,
hide normal upgrade-plan prompts, and present the private Pulse Pro runtime
download action instead of describing the feature as an unlicensed Pro upsell.

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
That same diagnostics owner split also keeps maintainer analytics out of the
customer diagnostics surface. `DiagnosticsResultsPanel.tsx`,
`diagnosticsModel.ts`, and the diagnostics export path must not render or
preserve commercial funnel, sales funnel, pricing/checkout conversion, or
infrastructure onboarding telemetry from `/api/diagnostics`; those signals
belong in admin-owned metrics surfaces instead of Settings support UI.
`frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs`, called by
the canonical frontend audit runner, enforces that boundary by failing if the
diagnostics API, diagnostics results panel, or diagnostics payload model
reintroduce those analytics fields outside the defensive strip helper, and by
failing if production customer frontend source reintroduces the retired
commercial/onboarding analytics wrappers or `/api/upgrade-metrics/events`
calls. That same audit also fails if the retired conversion/funnel or metering
packages return under the compiled product licensing path, because Settings
support diagnostics cannot be the only customer-facing guard if the normal
product binary still carries the maintainer analytics pipeline.
Diagnostics cards that summarize Docker and Podman agent coverage must use the
shared `docker` source-platform label from
`frontend-modern/src/utils/sourcePlatforms.ts` for their heading and body copy,
so diagnostics results stay aligned with the governed settings/source-platform
vocabulary instead of inventing a local runtime family label.

The settings shell registry now also treats extracted feature prop contracts as
canonical shell inputs instead of reaching back into feature panels for type
ownership. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
must consume the direct Proxmox panel contract through
`frontend-modern/src/components/Settings/proxmoxSettingsModel.ts`, so the
registry stays a shell/composition owner and does not depend on
`ProxmoxSettingsPanel.tsx` as though the panel still owned the runtime model.

The retired `/operations` route is now a thin compatibility redirect only.
`frontend-modern/src/pages/Operations.tsx` may normalize legacy `/operations/*`
links into the canonical Settings support routes, but diagnostics, reports,
and logs now belong to the shared Settings shell instead of a bespoke page-
local tab surface. Support-only navigation must therefore route through the
shared settings owners rather than rebuilding a second route-level shell, and
public demo posture must keep those support entries hidden from the Settings
navigation instead of reviving a standalone operations page.
that are unavailable in demo mode.

The dashboard overview route and its feature-owned summary surfaces are
retired. Authenticated root entry now lands on Infrastructure, so first-
viewport estate orientation belongs to the Infrastructure page and Add
infrastructure flow rather than a separate dashboard shell. Future overview or
brief-style surfaces must be governed as new product surfaces before they add
route-level data orchestration, section anchors, or Assistant prompt handoffs;
they must not restore `frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/features/dashboardOverview/`, or deleted dashboard-only
presentation helpers as compatibility paths.
The primary navigation active-tab contract follows that retirement boundary:
retired or unknown routes such as `/dashboard` must not be coerced into the
Infrastructure tab just because Infrastructure is the authenticated landing
surface. Shared desktop and mobile navigation must tolerate a missing active tab
for those paths while still highlighting canonical active routes such as
Infrastructure, Workloads, Storage, Recovery, Alerts, Patrol, and Settings.
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
That same shared table boundary now owns CSP-safe sizing for infrastructure
tables and metric bars. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
and `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
must express table layout and column sizing as shared class/attribute
presentation instead of inline `style=` maps, and
`frontend-modern/src/components/shared/ProgressBar.tsx` must render fill width
through DOM attributes rather than inline width styles. Infrastructure host and
service tables may still vary by breakpoint and column family, but they must do
so through the shared presentation owner instead of lane-local style objects
that break the public demo CSP.
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
Problem-resource table readability belongs to that same owner. Repeated rows
may collapse only when they share the same governed display label, resource
type, and problem signal; the header count and Pulse Brief counts must continue
to represent the underlying affected resources, and grouped links must route to
the broad owning surface rather than inventing a synthetic resource target.
Problem Resources and Pulse Brief wording must not amplify generic
status-shaped names such as `storage (offline)` into first-viewport prose or
grouped-row sublabels; when the resource name is only a type plus status, the
surface should summarize the type-level issue in operator language instead of
repeating raw backend-shaped labels.
The retired dashboard action queue must not be reintroduced as a compact
Patrol or infrastructure issue panel. Patrol-owned runtime findings remain
governed by `frontend-modern/src/utils/aiFindingPresentation.ts` and their
own Patrol route/store surfaces; any future cross-surface issue queue needs a
new governed owner rather than reviving dashboard action-panel files.

Feature-owned alert shells under `frontend-modern/src/features/alerts/` now
also treat shared action runtime as a first-class feature owner instead of
rebuilding it per surface. The overview shell must compose
`frontend-modern/src/features/alerts/useAlertAcknowledgementState.ts` for
acknowledge/restore behavior rather than keeping duplicate API and notification
logic inline in `useAlertOverviewState.ts` or a revived dashboard recent-alert
panel.
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
including the route-owned billing focus contract where
`/settings/system/billing/plan` is the canonical settings-tab destination,
`/settings/system/billing/usage` is a same-tab child state, and legacy billing
base/hash links are compatibility inputs rather than primary runtime routes.
That same route-sync owner must also preserve Proxmox platform-selection truth
across canonical deep links such as
`/settings/infrastructure/platforms/proxmox/pbs` and
`/settings/infrastructure/platforms/proxmox/pmg`: even though those routes
collapse into the shared `infrastructure-operations` tab, the selected
platform state must still be derived from the path instead of silently
falling back to `pve` on reload or remount.
That same settings access boundary must keep route eligibility separate from
sidebar visibility. Panel-owned feature gates such as Relay, Reporting, RBAC,
Audit Log, and Audit Webhooks may be hidden from the navigation on Community
installs, but their direct settings routes must stay routeable so the owning
panel can render its locked, non-flashing state instead of being bounced to the
default Infrastructure tab.
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
That same settings-routing contract now also owns the Support group for
`Diagnostics & Health`, `Data & Reports`, and `System Logs`: the navigation
model must normalize both `/settings/operations/*` and legacy `/operations/*`
compatibility links into `/settings/support/*`, and the catalog plus visibility
owners must treat those support surfaces as Settings-native pages rather than
as a second top-level utility destination.

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
owns the pure investigation-context summary and Patrol-to-Assistant operator
briefing derivation, including the rule that active findings, pending
approvals, and governed action references outrank secondary coverage caveats
when building the Assistant prompt, action label, and safety note,
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
of collapsing both timestamps back into a generic `Last run` label. Coverage
phrases on those recency surfaces must come from the Patrol recency presenter
instead of hardcoding verified wording in the shell.
That same run-history ownership applies to assessment caveats: Patrol summary
shells should not present `Recent coverage is incomplete` when the shared
recency/verification helpers already prove a successful full patrol with
non-zero resource coverage.
That same Patrol shell ownership includes refresh affordance state:
`frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts` must keep
operator refresh controls generation-aware and timeout-bounded, so a slow
supporting intelligence read cannot permanently disable the shared Patrol header
Refresh button while Patrol findings and status remain visible.
That same Patrol shell should make scoped trigger policy legible without
another navigation step. `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
should present alert-triggered and anomaly-triggered Patrol toggles as distinct
controls, and `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`
should render compact activity breakdown and scoped-trigger-state copy from the
shared transport rather than leaving busy Patrol periods as unexplained noise.
That same Patrol-facing primitive vocabulary must stay product-first. Patrol
summary actions, runtime banners, run-history runtime-failure actions,
runtime-finding actions, circuit-breaker copy, and Patrol configuration controls
may point at the shared provider settings route or model catalog, but they
should describe those controls as Patrol/provider surfaces through
`frontend-modern/src/utils/patrolRuntimeActions.ts` rather than falling back to
generic `AI Settings`, `AI Model`, or `AI circuit breaker` copy inside the
Patrol shell itself.
That same product-first naming rule also applies to the shared `system-ai`
settings shell: `frontend-modern/src/components/Settings/AISettings.tsx`,
`frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
`frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
`frontend-modern/src/components/Settings/useAISettingsState.ts`, and
`frontend-modern/src/utils/aiSettingsPresentation.ts` must present that surface
to operators as `Assistant & Patrol` plus provider/model configuration rather
than as a generic `AI Services` shell.
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
Supporting context follows that same composition rule. Recent changes, learned
correlations, and policy coverage belong behind an explicitly secondary
supporting-context disclosure that only appears when Patrol has active
findings, degraded or incomplete verification, or a selected run that needs
explanation; healthy fully verified Patrol states must not advertise that
supporting evidence as a peer workflow. When that disclosure expands, the
workspace must explicitly label findings and run history as Patrol verification
evidence and frame the supporting cards as explanatory context rather than as a
fresh Patrol result. `frontend-modern/src/features/patrol/patrolSupportingContextPresentation.ts`
must own that disclosure copy and toggle wording so the Patrol workspace does
not regress into inline shell-local trust language.

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
shared banner or shared settings shell would explain monitored-system plan
caps, the correct primitive decision is absence: monitored-system volume is not
a current paid-capacity surface. Shared headers and descriptions may use
monitored-system language for inventory grouping and support ledgers, but they
must not talk about monitored-system limits, cap pressure, plan capacity,
admission freezes, or upgrade actions. Future work must not recreate
`MonitoredSystemLimitWarningBanner`, its state hook, or banner-local
monitored-system copy strings.
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
for override normalization and resource-backed projection. That shared
feature-model boundary must also canonicalize legacy shared-storage override
keys and hashed storage resource ids onto the storage metrics target id before
thresholds rows are derived, so migrated Ceph/shared-datastore overrides and
Ceph pool overrides survive the feature-shell path instead of dropping out of
the live editor, and
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
empty, OAuth, action/error, shell-description, and workload-discovery copy
for the settings shell stays on one governed helper instead of drifting back
into section-local strings.
`frontend-modern/src/components/Settings/AuditLogPanel.tsx`,
`frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`,
`frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`,
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
That boundary must keep SAML creation on the same first-class action path as
OIDC. `SSOProvidersPanel.tsx` may show read-only state from settings
capabilities, but it must not render a self-hosted Pro upsell, `UpgradeLink`,
or `advanced_sso` feature probe before opening the SAML provider modal.
`useSSOProvidersState.ts` must treat provider type as form state only; SSO
entitlement truth belongs to the backend/runtime capability contract, where
OIDC, SAML, and multi-provider SSO are Community-tier capabilities.
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
the `system-billing` navigation label, header title/description, and billing
shell framing must all route through `SELF_HOSTED_PRO_BILLING_PRESENTATION`
instead of drifting independently. The owned split is now explicit: the
navigation label comes from `navLabel`, while the route header and billing
shell reuse `shellTitle` plus `shellDescription`, so the settings IA can stay
plan-owned (`Plans`) while the page itself still names the concrete job
(`Self-hosted plan`) without reintroducing local label drift.
That same settings-shell framing boundary also covers adjacent top-level
settings references to the self-hosted commercial surface. When
`InfrastructureWorkspace.tsx` or other settings-shell surfaces point operators
toward Plans for billing, license status, or paid feature activation, they
must reuse the shared referral copy from
`SELF_HOSTED_PRO_BILLING_PRESENTATION` rather than drafting local “go there
for billing” variants.
That same shared presentation owner now also carries the entitlement-first
commercial summary contract for self-hosted settings. The top-level navigation
entry stays product-IA owned through `navLabel` (`Plans`), while the page
header and shell title stay task-owned through `shellTitle`
(`Self-hosted plan`), and the billing shell must foreground the active plan
name plus available capabilities before secondary billing or recovery detail.
Paid upgrades should be able to confirm “Current plan: Pulse Pro” immediately
after activation without hunting through generic billing language or a second
page-local summary card model.
That same shell boundary also has to stay safe for hosted tenant bundles.
Settings-shell framing copy for self-hosted billing must route through
`selfHostedBillingPresentation.ts`, with `settingsNavCatalog.ts`,
`settingsHeaderMeta.ts`, and adjacent hosted settings shells consuming that
settings-owned adapter instead of importing generic commercial presentation
helpers in ways that can reintroduce top-level bundle-init cycles.
`frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx` is now a
shell only for network-boundary controls.
`frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
owns the public URL, CORS, embedding, and webhook-boundary UI, while the
editable discovery configuration entry point is owned by the infrastructure
workspace instead of the System/Network route. Shared prop contracts for the
network-boundary surface must extend
`frontend-modern/src/components/Settings/networkSettingsModel.ts` instead of
re-expanding the shell or reintroducing page-local section types.
`frontend-modern/src/utils/discoveryPresentation.ts` now owns the
customer-facing discovery-section framing copy, scan-scope labels, subnet
guidance, command-execution settings targets, API Access handoff labels, and
environment-lock messaging so
`frontend-modern/src/components/Settings/DiscoverySettingsForm.tsx` stays a
shared presentation shell instead of re-accumulating that wording inline.
Resource discovery command guidance must use that same presentation owner for
settings handoffs. `frontend-modern/src/utils/discoveryPresentation.ts` owns
the shared command-execution and `agent:exec` token-scope links; discovery
surfaces may explain those states, but the visible links must remain
`Settings → Infrastructure` and `Settings → API Access` through that helper,
not inline legacy labels or old settings paths.
That same presentation owner also packages the identified-service summary
consumed by surfaces outside the Discovery sub-tab.
`getDiscoveryIdentifiedSummary` is the canonical reducer that turns a stored
`ResourceDiscovery` into the compact card payload (service name, category,
confidence percent, port and path counts, cli access hint, service version,
observed timestamp, provenance label, and suggested web-interface URL
metadata). New surfaces that want to label a workload with its identified
service or offer a Discovery-sourced endpoint candidate must read through that
helper rather than re-implementing the empty/low-signal gate, so the
Discovery tab and out-of-tab surfaces collapse the same records and avoid
surfacing "Unknown" rows or zero-confidence noise. Manual/persisted
web-interface URLs still win: Discovery suggestions may be copied, opened, or
adopted through the shared `WebInterfaceUrlField`, but they must not silently
replace metadata or make row-name links active until the operator saves them.
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
descriptions or describe Relay as a Pro-only feature after Relay became its
own self-hosted paid tier.

Single-surface settings pages that only render one canonical `SettingsPanel`
must stay rooted directly at that panel instead of wrapping it in an extra
page-level `space-y-*` container. `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
`frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx`, and
`frontend-modern/src/components/Settings/AuditLogPanel.tsx` are the current
reference cases, and
`frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
locks that direct-root contract so single-surface pages do not quietly regain
redundant outer spacing chrome.
The same shared settings-shell boundary now also owns the API-backed source
path inside Infrastructure.
`frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
`frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
`frontend-modern/src/components/Settings/settingsNavigationModel.ts`,
`frontend-modern/src/utils/workloadEmptyStatePresentation.ts`,
`frontend-modern/src/utils/infrastructureEmptyStatePresentation.ts`, and
adjacent setup guidance must use `Add infrastructure` as the operator-facing
first-run label for API-backed onboarding, resolve that label to the shared
`Infrastructure` destination and its inline `ConnectionEditor` add flow, and
avoid reviving a standalone platform shell, `Platform connections` label, or
provider-local route.
That same settings-shell contract also owns the shared infrastructure summary
state. `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`,
`frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`,
`frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
`frontend-modern/src/components/Settings/useTrueNASSettingsPanelState.ts`, and
`frontend-modern/src/components/Settings/useVMwareSettingsPanelState.ts` must
derive Proxmox/PBS/PMG/TrueNAS/VMware counts and availability from one shared
infrastructure settings state source instead of letting the top-level ledger
and inline credential flows fetch the same connection state separately. Phase
9 retired the standalone `PlatformConnectionsWorkspace.tsx`,
`TrueNASSettingsPanel.tsx`, and `VMwareSettingsPanel.tsx` shells; they remain
labels and proof history, not live presentation surfaces.
That same shared settings-shell boundary also owns provider parity inside the
inline add flow. Adding VMware may extend the same card, empty-state, dialog,
and summary-shell patterns used by TrueNAS, but it must not introduce a
VMware-only outer page shell, alternate settings route hierarchy, or another
summary vocabulary for connection health and contribution counts.
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
The shared shell boundary now also includes
`frontend-modern/src/contexts/appRuntime.ts` as the only neutral owner for
app-level websocket and dark-mode consumption. Shared shells and primitives
such as `frontend-modern/src/components/Settings/Settings.tsx`,
`frontend-modern/src/components/shared/TagBadges.tsx`, and
`frontend-modern/src/components/shared/useInfrastructureSummaryTableState.ts`
may consume that module, but they must not import `@/App` or recreate shell
providers. `frontend-modern/src/App.tsx` owns provider placement; primitives
own reusable consumption only.
That same shared settings-shell and banner boundary now also owns demo-mode
commercial suppression. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
`frontend-modern/src/components/Settings/settingsNavVisibility.ts`,
`frontend-modern/src/stores/sessionCapabilities.ts`,
`frontend-modern/src/stores/demoMode.ts`,
`frontend-modern/src/useAppRuntimeState.ts`,
`frontend-modern/src/components/shared/HistoryChartOverlay.tsx`,
`frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`, and
`frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
must consume one shared bootstrap truth from
`/api/security/status.sessionCapabilities.demoMode` and hide billing tabs,
trial nudges, monitored-system warning banners, dashboard upsells, Patrol
upgrade CTAs, history-lock paywalls, and other public-demo commercial
affordances when the browser is rendering a public demo runtime.
Shared primitives must not perform their own ad hoc `/api/health` polling,
response-header inference, hostname heuristics, or per-banner demo branching;
the runtime bootstrap, shared session-capability store, and shared banner
hooks stay on one canonical owner so suppression stays coherent across
customer-facing surfaces.
That same session-presentation boundary owns the non-promotional self-hosted
v6 app posture. Settings navigation, shared upgrade links, trial banners,
monitored-system warning banners, history-lock overlays, and paid-feature gate
primitives must honor resolved `presentationPolicy.hideUpgrade` by hiding
prompts by default on ordinary self-hosted installs. Direct
activation/recovery routes may still render their owned content, but sidebar
discovery, trial CTAs, plan-review links, plan upsells, and feature upgrade
links must not appear unless an explicit handoff, hosted-mode policy, or active
entitlement says they should.
That same shared app-shell boundary now also owns assistant bootstrap silence
on non-AI routes. `frontend-modern/src/useAppRuntimeState.ts`,
`frontend-modern/src/App.tsx`,
`frontend-modern/src/stores/aiChat.ts`,
`frontend-modern/src/components/AI/Chat/index.tsx` must treat
`/api/security/status.sessionCapabilities.assistantEnabled` as the only
general-route assistant availability fact, while closed assistant chrome and
non-AI settings panels stay off `/api/settings/ai` and `/api/ai/*` until an
owned assistant or Patrol surface is actually open. `frontend-modern/src/stores/aiChat.ts`
must therefore stay presentation-only with respect to assistant bootstrap:
org-switch cleanup, keyboard focus, drawer state, and local context/session
persistence belong there, while backend settings/model reads stay on
`frontend-modern/src/stores/aiRuntimeState.ts`. The governed browser proof in
`tests/integration/tests/11-first-session.spec.ts` must continue to assert
that plain settings routes render without assistant bootstrap traffic or
console noise.
When an owned Patrol or alert surface attaches a source-named Assistant
handoff, that same drawer shell must keep the empty conversation state aligned
with the attached briefing as neutral `Context attached` copy instead of
rendering generic cluster/system starter prompts or feature-authored suggested
prompt chips below the source-owned context.
Shared table, disclosure, and form primitives must also stay explicitly typed
at the browser edge. Summary rows may memoize repeated pending-update reads,
shared buttons must preserve discriminated disclosure props, toggle and a11y
helpers must expose exact event signatures, shared rows must accept typed
`data-*` props, and reporting-panel helpers must remain ES2020-safe instead of
depending on feature-local casts or newer string helpers.
The settings navigation model now exposes a single `infrastructure-systems`
sidebar entry for the infrastructure settings area. The former
`infrastructure-connections` and `infrastructure-install` entries have been
removed from `SettingsTab`, `settingsNavCatalog.ts`, `settingsPanelRegistry.ts`,
and `settingsNavigationModel.ts`. All canonical redirects and tab-derivation
logic that previously mapped to those two entries now collapse to
`infrastructure-systems`. No future additions to the settings nav may restore
`infrastructure-connections` or `infrastructure-install` as independent tab
identifiers; panel routing within the infrastructure area must use
`InfrastructurePanelStep` in-page state instead of URL sub-routes.
`frontend-modern/src/components/Settings/settingsNavigationModel.ts` now uses
the normalised (not canonical) path when resolving Proxmox agent and path
checks so that deep links such as `/settings/infrastructure/platforms/proxmox/pbs`
resolve to the correct agent before the canonical-redirect fires, rather than
after it has already collapsed the path.
The shared frontend source/platform vocabulary now also includes
`availability` as an agentless infrastructure source and `network-endpoint` as
the canonical resource projection. Picker cards, source labels, badges, and
settings add-flow copy must use the shared onboarding and source-platform
helpers instead of feature-local wording, so availability probes stay visually
aligned with the single Infrastructure settings surface without pretending to
be a host agent install.
Availability setup presets for pingable devices, MQTT, ESPHome, or similar
agentless endpoints must also stay on the shared settings form vocabulary:
presets may fill protocol, port, and path defaults, but display badges and
drawers still derive `Availability` and `Network Endpoint` labels from the
shared resource presentation helpers rather than from preset-local copy.
Infrastructure rows for those same agentless endpoints must surface probe
evidence directly in the row, not just as a green status dot or an
`Availability` badge. The shared row presentation must expose the probe method
and latest latency or failure result once, inline in the agentless endpoint's
metric slot, while keeping recent check timing and fuller failure context in
the tooltip or drawer so operators can understand what was measured without
duplicated row chrome.
