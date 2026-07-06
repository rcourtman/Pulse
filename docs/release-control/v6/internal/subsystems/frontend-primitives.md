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
    "api-contracts",
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
   6a. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`
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
    42a. `frontend-modern/src/i18n/`
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
    59c. `frontend-modern/src/components/shared/FilterBar/filterOptionPresentation.tsx`
60. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
61. `frontend-modern/src/features/` (including Patrol presentation, where Patrol control starter counts are context only even when mirrored through `patrolAutonomy*` compatibility fields, successful direct Patrol control saves may record the content-free `patrol_control` starter only when paid control is available and the effective control posture changes and must then refresh Patrol status, findings, approvals, and run history before the operator waits for polling, legacy `proActivation*` starter aliases must not render a separate proof strip by themselves, Patrol control completed/resolved counts may only project backend-owned terminal proof, current active findings and pending approvals outrank historical completion proof in the primary operator state, selected run history must read as a Patrol run record rather than a findings filter or snapshot workflow, terminal verified/rejected outcomes with no active finding or pending approval must stay history detail without rendering a no-op proof strip, resolved-only issue history must not be promoted into current-work copy or actions, compact recurrence/trust counters must read as historical evidence rather than current issue state, Patrol-owned status/history evidence must keep the assessment visible when the broader intelligence summary is missing, Patrol work-group chips may group current approvals, failed actions, failed checks, recurring active issues, and stale scheduled protection but must not become a separate status/trust/proof strip, the Patrol route and page title must lead with Patrol while the default workspace underneath may use Open work and run history stays a deliberate secondary review surface, setup-only Patrol runtime failures must instead use `Fix Patrol setup` framing with a dedicated setup task and direct provider-settings action while suppressing generic issue-row chips and filter chrome, Patrol must not expose a generic Details/supporting-context panel for nearby activity, related patterns, or policy limits, locked-control copy must state the watch-only boundary in positive capability language by saying Patrol checks infrastructure and shows current issues, avoid repeating the same sentence across the header and control, and avoid repeatedly restating infrastructure-unchanged caveats or relying on disabled controls, compact Pro badges, Limits controls, or manual-review framing, `patrolControlValueState` decides whether a terminal decision is partial review context or verified value proof while `patrolAutonomyValueState` remains a compatibility mirror, legacy `proActivation*` fields are compatibility fallback only, native Patrol state must not load the operations-loop status projection to decide current work, local Patrol state must expose issue-backed `patrolWork*` evidence rather than legacy proof naming, Patrol mode labels remain domain copy that must describe backend-owned risk policy without creating page-local safety thresholds, the selected Patrol mode sentence must state the approval and policy boundary without adding a second limits panel or proof strip, the Patrol schedule and model drawer must stay separate from the always-visible Patrol mode selector instead of duplicating the four control choices or reintroducing save/apply configuration framing, and Patrol header refresh controls must call an explicit operator-refresh handler whose spinning/disabled state is separate from background polling and initial data loads)
62. `frontend-modern/src/components/SetupWizard/SetupWizard.tsx`
63. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
64. `frontend-modern/src/components/SetupWizard/SetupCompletionPreview.tsx`
65. `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
66. `frontend-modern/src/components/SetupWizard/__tests__/SetupWizard.test.tsx`
67. `frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPreview.test.tsx`
68. `frontend-modern/src/components/SetupWizard/__tests__/WelcomeStep.test.tsx`
69. `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
70. `frontend-modern/src/components/Settings/useSystemLogsPanelState.ts`
71. `frontend-modern/src/utils/systemLogsPresentation.ts`
72. `frontend-modern/src/components/Settings/__tests__/SystemLogsPanel.test.tsx`
73. `frontend-modern/src/components/Settings/ResourcePicker.tsx`
74. `frontend-modern/src/components/Settings/reportingResourceTypes.ts`
75. `frontend-modern/src/utils/reportableResourceTypes.ts`
76. `frontend-modern/src/utils/reportingResourceTypes.ts`
77. `frontend-modern/src/utils/workloadEmptyStatePresentation.ts`
78. `frontend-modern/src/utils/workloadGuestPresentation.ts`
79. `frontend-modern/src/utils/emptyStatePresentation.ts`
80. `frontend-modern/src/utils/semanticTonePresentation.ts`
81. `frontend-modern/src/components/Toast/Toast.tsx`
82. `frontend-modern/src/utils/toast.ts`
83. `frontend-modern/src/utils/semanticTonePresentation.ts`
84. `frontend-modern/src/utils/emptyStatePresentation.ts`
85. `frontend-modern/src/utils/typeColumnPresentation.ts`
86. `frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx`
87. `frontend-modern/src/components/Settings/networkSettingsModel.ts`
88. `frontend-modern/src/components/Settings/useDiscoverySettingsState.ts`
89. `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts`
90. `frontend-modern/src/components/Settings/AvailabilitySettingsPanel.tsx`
91. `frontend-modern/src/components/Settings/availabilitySettingsModel.ts`
92. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
93. `frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts`
94. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
95. `frontend-modern/src/components/Settings/settingsNavCatalog.ts`
96. `frontend-modern/src/components/Settings/settingsNavVisibility.ts`
97. `frontend-modern/src/components/Settings/settingsRouting.ts`
98. `frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts`
99. `frontend-modern/src/components/Settings/settingsTypes.ts`
100. `frontend-modern/src/components/Settings/useSettingsNavigation.ts`
101. `frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx`
102. `frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx`
103. `frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx`
104. `frontend-modern/src/components/shared/EnvironmentLockBadge.tsx`
105. `frontend-modern/src/utils/environmentLockPresentation.ts`
106. `frontend-modern/src/utils/docsLinks.ts`
107. `tests/integration/tests/20-local-doc-links.spec.ts`
108. `frontend-modern/src/index.css`
109. `frontend-modern/src/components/shared/summaryInteractionA11y.ts`
110. `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`
111. `frontend-modern/src/hooks/createNonSuspendingQuery.ts`
112. `frontend-modern/src/components/shared/TableCardHeader.tsx`
113. `frontend-modern/src/components/shared/UpgradeLink.tsx`
114. `frontend-modern/src/components/shared/useUpgradeNavigation.ts`
115. `frontend-modern/src/utils/upgradeNavigation.ts`
116. `frontend-modern/src/components/DemoBanner.tsx`
     116a. `frontend-modern/src/components/CommercialMigrationBanner.tsx`
     116b. `frontend-modern/src/components/GitHubStarBanner.tsx`
117. `frontend-modern/src/components/Login.tsx`
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
129. `frontend-modern/scripts/shared-template-audit.mjs`
130. `frontend-modern/scripts/shared-template-registry.json`
131. `frontend-modern/src/features/platformPage/sharedPlatformPage.tsx`
     131a. `frontend-modern/src/features/platformPage/PlatformResourceDetailTableRow.tsx`
     131b. `frontend-modern/src/features/platformPage/PlatformOutdatedAgentNotice.tsx`
     131c. `frontend-modern/src/features/platformPage/PlatformOutdatedSensorSetupNotice.tsx`
132. `frontend-modern/src/utils/platformSupportManifest.generated.ts`
133. `frontend-modern/src/utils/platformSupportManifest.ts`
134. `frontend-modern/src/utils/sourcePlatformOptions.ts`
135. `frontend-modern/src/utils/sourcePlatforms.ts`
136. `frontend-modern/src/utils/infrastructureOnboardingPresentation.ts`
137. `frontend-modern/src/components/shared/Button.tsx`
138. `frontend-modern/src/components/shared/buttonModel.ts`
139. `frontend-modern/src/components/shared/Button.test.tsx`
     139a. `frontend-modern/src/components/shared/InlineNotice.tsx`
     139b. `frontend-modern/src/components/shared/InlineNotice.test.tsx`
     139c. `frontend-modern/src/components/shared/ExternalTextLink.tsx`
     139d. `frontend-modern/src/components/shared/ExternalTextLink.test.tsx`
140. `frontend-modern/src/components/shared/CopyableCodeRow.tsx`
141. `frontend-modern/src/components/shared/DetailSectionTable.tsx`
142. `frontend-modern/src/components/shared/detailSectionModel.ts`
143. `frontend-modern/src/components/Settings/__tests__/settingsLocalization.test.ts`
144. `frontend-modern/src/i18n/__tests__/i18n.test.ts`

## Shared Boundaries

Settings navigation discoverability is part of the shared settings-shell
boundary. A settings route that is available in normal commercial presentation
must be reachable through the sidebar unless it is explicitly a hidden
deep-link flow. The self-hosted `Plans` page is not a deep-link-only flow:
`system-billing` stays visible whenever commercial presentation is allowed,
while demo or commercial-suppressed sessions may still hide it.

Candidate import-plan presentation inside the Infrastructure settings dialog is
a shared primitive composition boundary. `NodeCandidateImportPlan.tsx` may use
shared `Button`, checkbox styling, lucide icons, and
`MonitoredSystemImpactPreview`, while `InfrastructureWorkspace.tsx` owns the
route-backed dialog state that feeds probe or Discovery candidates into the
credential slot. That surface must keep the approval card readable inside the
existing dialog body and must not introduce a second nested modal, detached
wizard shell, or page-local preview renderer for monitored-system impact.

Frontend localization is a shared primitive boundary. Locale support must flow
through typed message catalogs with an English fallback and explicit seed
locale coverage rather than page-local string switches.
`frontend-modern/src/i18n/locales.ts` owns locale normalization, the supported
locale registry, and fallback chains; `frontend-modern/src/i18n/messages.ts`
owns the typed catalog shape; and `frontend-modern/src/i18n/policy.ts` owns the
first-wave non-translatable token rules. The active app locale is a shared user
preference initialized from stored or browser language and exposed through
Settings > General; individual surfaces must consume that shared preference
instead of creating local language toggles. Customer-facing shell, navigation,
settings, first-run, empty-state, commercial handoff, and alert copy may be
localized through this catalog — including the alert-to-Patrol action surface ("Have Patrol
investigate" and its targeted-check menu hint) that is primary on
resource-backed active alert cards, plus the secondary Assistant explanation
handoff strings — but
machine-facing values must remain stable: commands, environment variables, API
fields, config keys, log lines, error codes, hostnames, resource names, product
identifiers, and vendor object names stay untranslated unless the owning
runtime contract explicitly says otherwise. Shared settings-shell header copy
for the self-hosted plan must keep first-wave locale catalogs aligned with the
English product stance: on Pro, the operator chooses how autonomous Patrol
should be. The settings shell must not teach a separate activation loop, MCP
readiness, or operations-loop proof model as the default plan setup story.
Pulse Intelligence external-agent setup uses the same shell language with a
`Choose Patrol mode` handoff before scoped-token setup, and the expanded setup
checklist must say to set how autonomous Patrol should be before connected
agents request work rather than repeating internal automation/proof wording.
Developer-only external-agent posture uses `External agents` plus Patrol mode
before surfacing MCP or workflow prompt wire names.
Migrated settings surfaces must render customer-facing copy through the catalog
and shared presentation helpers rather than reintroducing panel-local English.
Migrated first-session surfaces,
including the Setup Wizard shell, welcome/security steps, setup completion
handoff, and runtime-home loading handoff, follow the same catalog path; their
guardrails must fail if the migrated journey returns to page-local English or
translates commands, URLs, generated credentials, product/source identifiers, or
reported resource names. Migrated Alerts Overview surfaces, including the page
shell, overview stat cards, active-alert triage list, acknowledgement actions,
incident timeline panel/filter controls, and Pulse Assistant alert handoff
briefing, must also route user-visible copy through the catalog and
alert-owned presentation helpers. Alert IDs, alert types, resource IDs,
resource names, node names, source messages, event payloads, commands, command
output, logs, and Assistant model-context labels stay machine-stable and
untranslated.
The legacy pricing handoff page may also route its visible redirect title and
manual-link copy through the catalog, but `Pulse Account`, route paths, feature
keys, query parameters, public URLs, and purchase-return state remain
machine-stable and untranslated.

Alert thresholds consume the shared FilterBar primitive and route state, while
the alerts subsystem owns the resource data and platform-specific threshold
tabs. The thresholds platform IA is platform-shaped: Proxmox, Docker,
Kubernetes, TrueNAS, vSphere, PBS, PMG, and Systems. Frontend primitives own
the chip, reset, "+ Filter", and route-backed shell pattern; alerts must not
replace that with page-local search/tab chrome or legacy neutral buckets.
Native multiline form fields are also a shared primitive boundary.
`FormTextarea` owns label/id/help wiring, controlled value synchronization, and
textarea chrome for alert, settings, and infrastructure runtime surfaces; those
surfaces must not recreate raw native `<textarea>` shells locally.

Toast notification chrome is a shared primitive boundary. `Toast` owns the
global notification stack shell, status icon placement, and dismiss action
chrome; status glyphs must come from the shared library icon set and dismiss
controls must compose `ActionIconButton`. Consumer-specific raw SVG status
icons, toast-local close-button class strings, and page-local toast stacks are
forbidden unless the shared-template registry is intentionally extended first.

The System Updates install guide is a shared settings primitive, not a
deployment-lane-only panel. `UpdateInstallGuide` must render the canonical
update-plan readiness verdict inline with the update action, and a blocked
readiness status must make automatic install visibly unavailable until the
blocking check is resolved.

`frontend-modern/src/components/shared/DiscoveryReadinessBadge.tsx` is the
shared presentation primitive for discovery freshness/readiness indicators.
It may render the canonical presentation model from
`frontend-modern/src/utils/resourceDiscoveryReadiness.ts`, but it must remain
presentational: no local storage, network reads, discovery fetches, or
resource-target inference belongs inside the badge. Workload rows, drawers,
and Assistant handoff surfaces must share that primitive instead of inventing
local freshness chips.

Platform page subnavigation is a shared frontend primitive. Docker / Podman
and Kubernetes platform pages may add native API-backed sections, but the tabs
must use `PlatformSectionTabs`, canonical table alignment helpers, and shared
resource type presentation/reporting helpers rather than page-local tab shells,
alignment classes, or ad hoc report-category coercion. Platform tabs are
feature-owned consumers of canonical resource payloads: Docker page model
helpers may prefer backend-authored `DockerData` stack and Podman metadata for
search/display while shared primitives continue to own only the tab shell,
filter controls, and reusable presentation affordances. Platform tabs are
workflow-level navigation, not one visible tab per API resource kind, and they
must be evidence-gated by their owning row or signal model. `Overview` is the
stable landing surface; supporting workflow tabs appear only when the current
setup has native inventory or signal for that workflow, and legacy object URLs
resolve to their owning workflow only when that workflow is visible. Docker /
Podman may expose `Overview`, `Images`, `Storage`, `Networks`, and `Swarm`,
while legacy `/docker/containers` resolves to the Overview landing surface
rather than remaining a separate visible tab. Kubernetes may expose `Overview`,
`Nodes`, `Workloads`, `Services`, `Storage`, `Configuration`, and `Events`;
TrueNAS and vSphere follow the same evidence-gated primitive for native
storage, service, app, VM, protection, datastore, network, health, and activity
workflows. API-native tables remain bespoke under those workflows, so Docker
`Overview` owns runtime hosts plus primary container workloads, Docker `Storage`
owns engine disk usage plus volumes, and Docker `Swarm` owns services, tasks,
nodes, secrets, and configs; Kubernetes
`Workloads` owns Pods, Deployments, controllers, and autoscaling, `Services`
owns Services plus ingress/endpoint inventory, and `Configuration` owns config
plus policy inventory. Backup and recovery platform pages follow the same
navigation primitive boundary: source-specific evidence tables may be exposed as
secondary drilldowns under an owning workflow tab, but the shared tab shell must
not grow one top-level tab for each API source merely because that source has a
table. Legacy object-specific URLs may resolve to the owning
workflow tab, but they must not reappear as top-level platform navigation unless
the product IA is intentionally changed. Overview tabs must stay deliberately
shaped around the primary operator job instead of repeating every detail table:
Docker / Podman Overview owns runtime hosts and primary container workloads in
the proven host-then-workloads pattern, while Kubernetes Overview owns
cluster/control-plane rollup; supporting object tables live in their dedicated
workflow tabs. Docker / Podman native subsections now
include runtime containers, engine storage usage, Swarm node inventory, and
metadata-only Swarm secret/config inventory where the documented Docker APIs
report those resources; Podman-only libpod pod inventory must not be represented
until the collector has a libpod-native source. Kubernetes config
inventory must preserve the same trust boundary for metadata-only ConfigMap
and Secret rows: the shared table wording may indicate metadata-only inventory,
but must not imply payload fields were read when the agent used Kubernetes
metadata-only API responses, and the unified-resource owner supplies the
Namespace, ConfigMap, Secret, and ServiceAccount-specific columns. Kubernetes
Node inventory must also be reachable through a dedicated native tab, not only
the overview stack, while retaining the shared `PlatformSectionTabs` shell.
Primary app-shell navigation consumes unified-resources-owned resource evidence:
empty compatibility facets such as `docker: {}` do not admit runtime-lens tabs
on their own.
Feature-owned Docker / Podman action controls may render backend
`actionReadiness` disabled reasons, but the shared primitive layer owns only the
button/table affordance shell; it must not invent action availability, command
agent state, or alternate execution routes.
Kubernetes Service inventory must likewise stay on the shared tab, toolbar,
table, table-alignment, and inline-detail primitives while the unified-resource
owner supplies Service type, virtual IP, published port, and selector columns.
Kubernetes storage inventory follows that same primitive boundary while the
unified-resource owner supplies StorageClass, PersistentVolume, and
PersistentVolumeClaim-specific columns. Kubernetes networking inventory also
follows that same primitive boundary while the unified-resource owner supplies
Service, Ingress, and EndpointSlice-specific columns.
Outdated-agent notices on platform pages are part of this same shared
frontend/platform primitive boundary and must compose
`frontend-modern/src/components/shared/InlineNotice.tsx` for the dense notice
shell, icon/content layout, and action-link chrome. Agent-backed and hybrid
platform pages may
surface a compact stale-agent cue when their row model carries Pulse agent
identity and version evidence, but the CTA must route to the canonical
Infrastructure settings update-command surface with scoped agent IDs instead of
duplicating installer command assembly, tokens, or lifecycle copy in each
platform page. These notices must compare against the API-owned
`agentUpdateTargetVersion` rather than the app build version, so development
builds can show their dirty server version without implying agents can or
should update toward that build. Agentless API-only platforms such as vSphere
must not grow this notice unless a concrete guest or monitored system row
actually carries a Pulse agent identity. On vSphere specifically, stale-agent
copy belongs to correlated in-guest VM agents and must not describe ESXi hosts
as Pulse-agent update targets just because phase-1 host resources use the
canonical `agent` resource type. Kubernetes is cluster-agent-backed: canonical
`k8s-node` rows may be pure Kubernetes API rows rather than merged `agent`
rows, so the unified-resource owner must project the cluster agent identity and
cluster-scoped agent version onto those node rows before the shared stale-agent
collector can decide whether the node inventory is gated by an older agent.
Global dismissible notice bars are also part of the shared `InlineNotice`
boundary. `frontend-modern/src/components/DemoBanner.tsx` and
`frontend-modern/src/components/CommercialMigrationBanner.tsx` must compose
`InlineNotice` with the `banner` layout, the shared icon library, action slots,
and the primitive's dismiss slot instead of carrying local colored notice
shells, raw SVG status icons, or page-local action and close button classes.
Kubernetes policy inventory follows that same primitive boundary while the
unified-resource owner supplies NetworkPolicy policy type and rule-count
columns, PodDisruptionBudget budget and observed health columns, ResourceQuota
hard/used quota columns, and LimitRange item-type columns.
Kubernetes autoscaling inventory follows that same primitive boundary while the
unified-resource owner supplies HorizontalPodAutoscaler scale target, replica
bounds, current/desired replicas, and metric source columns.
Kubernetes events inventory follows that same primitive boundary while the
unified-resource owner supplies Event type, reason, involved-object, count,
observed-time, and message columns.
Docker empty-state guidance on platform pages follows the same shared platform
primitive boundary: it may use the route-specific Docker / Podman vocabulary,
but it must distinguish standalone Docker host installation from the Proxmox
LXC Docker host-side inventory path without adding page-local installer command
assembly or token handling.
Docker / Podman inventory follows that same primitive boundary while the
unified-resource owner supplies API-object-specific container, image, volume,
network, Swarm node, task, secret, and config columns through dedicated native
tables. The Docker containers tab must use the native
`DockerContainersTable` for container state, health, restart, image, port,
network, mount, update, governed lifecycle actions, and host/runtime columns
rather than embedding `WorkloadsSurface`. The lifecycle action column is a
compact icon-button primitive over unified-resource capabilities and the shared
resource-action API client; it must not grow Docker/Podman shell, SSH, or
provider calls inside the table component. The same governed lifecycle controls
may appear in the resource detail header, while the platform table and drawer
shells remain presentation owners only: they may pass a post-success refresh
callback to their existing resource query, but must not own execution,
approval, policy, or provider dispatch. Swarm services must surface the
API-reported rollout/update
state in the native services table, and engine storage rows must expose a
stable row hook so platform-page browser proof can verify the storage tab is
hydrated from runtime disk-usage data. Kubernetes deployments must surface the
API-native observed generation and metadata age columns through the shared
table shell without page-local alignment helpers or generic infrastructure
columns.
Docker network rows use the same shared table/detail primitive split:
the default table columns prioritize attached workloads, attention state,
subnets, driver, and host, while lower-level scope, addressing, flags, and
network id details belong in the inline row disclosure. Attached container
names, network addresses, image, health/state, and published ports are
feature-owned data, but search and disclosure behavior must remain inside the
shared table chrome rather than a card deck, nested card, or route-changing
object browser. Dense networks must keep the inline disclosure bounded by
default and provide local attached-container search, status grouping, and
attention/running/other filters so large bridge or overlay networks remain
scan-friendly without hiding any container from drilldown.

1. `frontend-modern/src/components/CommercialMigrationBanner.tsx` shared with `cloud-paid`: the global commercial migration notice is both a cloud-paid entitlement recovery surface and a shared app-shell notice primitive consumer.
2. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx` shared with `ai-runtime`, `api-contracts`: the External agents settings panel is the optional settings-shell projection of Pulse MCP onboarding, the AI runtime connected-agent onboarding surface, and a presentation consumer of the shared agent capabilities frontend client.
3. `frontend-modern/src/components/Settings/APIAccessPanel.tsx` shared with `security-privacy`: the API Access settings intro is both a security/privacy token-management trust surface and a canonical settings-shell presentation boundary.
   The panel may own shell placement and local action layout, but
   token-specific Docker / Podman copy must come from
   `frontend-modern/src/utils/apiTokenPresentation.ts` rather than page-local
   text.
   `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx` stays
   under Pulse Intelligence Assistant settings because connected agents are an
   optional access path to Patrol work, while API Access remains the token
   minting surface linked from setup. Its setup copy must present one shared
   `pulse-mcp` runtime contract with client-native wrappers: OpenCode's top-level
   `opencode.json` / `mcp` shape and the common `mcpServers` shape for
   Claude-style clients. The server name, command, base URL flag/default,
   token environment variable, supported config families, and copied config
   snippets must flow through `frontend-modern/src/api/agentCapabilities.ts`
   from `/api/agent/capabilities.mcpAdapter`; the component may arrange and
   label them, but must keep setup mechanics, raw client config snippets, and
   developer details behind deliberate disclosures so the default Assistant
   settings view stays focused on chat command access and optional external
   access rather than raw JSON. External-agent posture should present
   `External agents` as the visible surface and reserve `Pulse MCP` or
   `pulse_operations_loop` for wire-name/debugging detail. The Patrol control handoff must route to the
   Patrol operator surface where the inline control level is configured. When
   setup is opened, the setup order must put Patrol control before scoped-token
   creation and client connection, while installer commands, client config
   snippets, and developer details remain deliberate disclosures.
   It must not frame
   Pulse MCP as a Claude-only surface, force OpenCode through a Claude-style
   wrapper, or duplicate a client-specific tool list. Full-surface token guidance in that panel must render the manifest-provided
   `requiredScopes` list through
   `frontend-modern/src/api/agentCapabilities.ts`; it must not hardcode a
   partial scope set in browser copy. Capability category order, labels, and
   descriptions must also come from the same manifest client; the settings
   panel may provide compatibility fallback rendering through that client, but
   it must not own a local category presentation table or
   `/api/agent/capabilities` fetcher. The panel's Pulse Intelligence surface
   summary is the same manifest projection: Pulse Intelligence Core, Patrol,
   Assistant, and MCP labels/descriptions plus surface affordance badges must flow through
   `frontend-modern/src/api/agentCapabilities.ts` from
   `/api/agent/capabilities.surfaceContract`, leaving the component to own only
   settings-shell layout. The visible external-adapter label in onboarding copy
   must also come from that manifest-derived capability posture instead of a
   hard-coded panel-local product label, so the settings shell cannot drift from
   the published agent surface contract. MCP capability-posture chips in that same panel must also
   flow through `getAgentManifestSurfaceToolContract(manifest,
AGENT_SURFACE_ID_PULSE_MCP)` and `getAgentSurfaceToolPosturePresentation`,
   with the static inventory read only from
   `/api/agent/capabilities.surfaceToolContracts`; missing
   `surfaceToolContracts` entries must not make the browser infer MCP tools
   from raw capabilities. The panel must not own request / response capability
   filtering, call a per-surface projection alias, know the `subscribe_events`
   streaming exception, or maintain a local MCP tool count. The shared frontend
   manifest client must keep MCP onboarding on the generic surface resolver
   rather than exporting a Pulse-MCP-specific tool-contract helper. The panel's
   settings shell may arrange the external-agent setup hierarchy, but it must
   keep connected agents framed as optional access to Pulse context and Patrol
   work, with Patrol as the built-in operator that checks infrastructure,
   follows Patrol mode before acting, asks when approval is required, verifies
   outcomes, and records history, state that external agents use that same boundary, and make the canonical
   `/settings/pulse-intelligence/assistant#external-agent-setup` route land on
   and briefly focus this panel with setup open rather than leaving the user at
   the generic API token inventory. Legacy `/settings/security/api#external-agent-setup` and
   `/settings/security/api#pulse-mcp-setup` links must remain accepted and may
   redirect to the canonical Pulse Intelligence Assistant route. Normal API
   Access visits remain token-management first, and external-agent setup must
   not reorder the API token inventory because it no longer lives on that page.
   The Assistant settings default must keep setup mechanics behind a `Show connector setup`
   disclosure so it does not introduce a separate external-agent operator
   journey or make copied MCP config blocks or tool-contract proof badges the
   default visual weight of Pulse Intelligence settings. Its Developer details
   disclosure may show posture and policy summaries behind Patrol
   access model, but prompt, scope, failure-code, and tool inventories must sit
   behind a nested Live manifest details disclosure so the advanced surface
   remains navigable. The panel's
   manifest-owned `pulse_operations_loop` prompt row may expose the stable
   prompt id for client builders, but its visible label, description, and badge
   must keep the user-facing Patrol framing from the manifest instead
   of reintroducing operations-loop proof wording in the settings shell. The panel's
   explanatory onboarding copy must name published manifest-owned surface
   contracts as the publication boundary rather than suggesting raw backend
   capabilities become visible automatically.
   The visible token preset name in that setup must be `Patrol external agent`,
   not `Pulse Intelligence agent`; the latter may survive only as a route/model
   compatibility id.
   Manifest-backed stable failure-code
   summaries may use settings-shell chips or compact lists, but code selection
   and capability attribution stay owned by the API manifest client. Token
   setup handoff buttons or anchors in
   the Agent integrations panel are settings-shell chrome only: they may route
   to the API Access token creation section, but token preset semantics and
   required-scope derivation remain owned by the API/security boundary.
4. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` shared with `security-privacy`: the data-handling settings surface is both a security/privacy trust surface and a canonical settings-shell presentation boundary.
5. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` shared with `security-privacy`: the data-handling settings model is both a security/privacy posture projection and a canonical settings-shell presentation boundary.
6. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `security-privacy`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
   The panel owns compact settings-shell framing for outbound usage telemetry, but
   its vocabulary must stay aligned with `security-privacy`: aggregate
   self-hosted adoption counts, coarse feature flags, and coarse Patrol,
   Assistant, and external-agent usage counters may be named, while
   hostnames, credentials, infrastructure identifiers, prompts, chat messages,
   command text, action output, token values, and personal information must
   stay explicitly excluded.
7. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `security-privacy`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
8. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `security-privacy`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
9. `frontend-modern/src/routing/routePreload.ts` shared with `performance-and-scalability`: the app-shell route preload registry is both a canonical frontend shell boundary and an authenticated hot-path performance boundary.
10. `frontend-modern/src/stores/aiChat.ts` shared with `ai-runtime`: the assistant drawer and session store is both an AI runtime control surface and a canonical app-shell presentation boundary.
    Assistant session pickers and reloads must restore only safe
    `handoff_summary` presentation state from the session list. Loading a plain
    session or starting a new conversation must clear stale scoped handoff
    briefing state so Patrol and alert context does not visually leak between
    conversations. Browser-originated model handoff payloads are one-shot
    request seeds: after the first successful chat send, this store must clear
    `handoffContext`, `handoffResources`, `handoffActions`, and safe
    `handoffMetadata`; it must also clear any preferred workflow prompt request
    once the manifest-rendered starter has seeded the composer and the first
    scoped send succeeds. The store must preserve the safe visible briefing and
    scoped approval-required posture, so later turns rely on backend session
    hydration instead of resending stale browser context. Patrol handoffs must not include
    Persisted Assistant redo availability is safe session chrome, not transcript
    content. The drawer may consume `ChatSession.can_redo` from the backend
    session list to re-enable Redo after reload or session refresh, but it must
    not read or duplicate the redo stack in frontend state. Undo restores the
    editable prompt draft and safe structured send metadata into the composer;
    Redo clears that recovered draft only after the backend restores the turn.
    Browser-owned session-management comments, command request handling, and
    question-answer plumbing in this store and the adjacent chat hook must name
    the native drawer surface as Pulse Assistant rather than reviving the
    retired generic `Pulse AI` label. Shared model/provider settings guidance
    still belongs to Pulse Intelligence > Provider & Models.
    Patrol handoffs must not include
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
    Patrol assessment, Patrol finding, and Patrol control save-failure sessions
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
    The shared `frontend-modern/src/components/shared/AIModelPicker.tsx`
    primitive must keep model route presentation delegated to the AI runtime
    label helpers. Pulse-owned local Assistant routes such as
    `pulse:local-inventory` and `pulse:mock-assistant` are implementation
    routes and must render as named choices without secondary raw route IDs,
    while external provider route IDs may remain visible where they disambiguate
    catalog entries.
11. `frontend-modern/src/utils/platformSupportManifest.generated.ts` shared with `unified-resources`: the generated platform support projection is both a canonical unified-resource platform union boundary and a shared frontend source/platform vocabulary boundary.
    It must expose the manifest `surface_kind` field so runtime lenses such as
    `docker` are not collapsed back into owning platform semantics.
    It must also preserve canonical projection lists from the governed manifest
    without page-local narrowing; for example, TrueNAS exposes both native
    `vm`, `network-share`, and `app-container` workloads through the same
    generated platform projection used by route helpers, badges, source
    filters, reportable-resource pickers, and type unions.
12. `frontend-modern/src/utils/sourcePlatforms.ts` shared with `unified-resources`: the source platform normalizer is both a canonical unified-resource source adapter boundary and a shared frontend source/platform vocabulary boundary.
    That shared boundary must preserve `availability` as the agentless
    monitoring source for `network-endpoint` resources and settings presets,
    so source badges and platform/source type resolution do not fall back to
    `generic` when an endpoint is represented by ping, TCP, or HTTP probe data
    rather than an installed agent or provider API.

## Extension Points

Assistant shell entry changes must keep Assistant contextual rather than
generic: `AppLayout.tsx` and the command palette may expose a compact launcher,
but that launcher must attach current-view context before opening the drawer
and label the action around the current monitoring, Patrol, Alerts, or Settings
view.
The app shell may surface Patrol current-work pressure as a secondary count on
the `Patrol` navigation tab, but it must not rename that tab to `Needs
Attention`, create a Home-like action queue, or route the operator away from the
Patrol route. Desktop and mobile accessible names should combine the stable
`Patrol` label with concise open-work count context when a count is present.

SSO provider settings changes must preserve the shared Community-tier action
path: SAML and OIDC provider creation stay on the same settings-shell control
surface, while paid-plan copy and compatibility feature probes stay out of the
frontend primitive boundary. The SSO provider settings shell is a fully
migrated shared-action consumer: add, test, preview, copy, close, cancel, save,
delete, edit, and dismiss controls must compose the shared `Button`,
`ActionIconButton`, and `CopyValueButton` primitives rather than restoring
panel-local `<button>` shells.

Feature surfaces under `frontend-modern/src/features/` may own product-specific
assessment semantics, but they must keep those semantics in their governed
presentation helpers and render them inside the shared neutral Pulse surface
language rather than introducing page-local verdict bands or nested cards.
For Patrol, that includes the Open work description: it may use concise
row-level guidance such as review evidence, approve a change, inspect automatic
actions, or review verification results, but it must remain descriptive copy
rather than another card, strip, or proof counter above the findings list.
Feature-owned runtime hooks may also own non-visual side effects when those
effects are part of the governed feature workflow. For Patrol, current-work
action chrome must keep active findings in the Patrol findings workflow first;
Assistant handoffs stay contextual actions on selected findings, approvals, or
history records rather than the primary current-work CTA. When provider/model
readiness blocks manual Patrol, the feature header must render the shared
primary-action chrome as a Provider & Models setup link instead of a disabled
run button that looks primary but cannot act. The Patrol hook owns the
content-free `pulse_operations_loop` starter marker so render components do not
fork telemetry or privacy behavior.
When the shared operations-loop status projection reports contextual
Assistant/external-agent collaboration inside the Assistant step, Assistant or
Pulse MCP starter counts, Patrol control starter evidence, or Patrol control
completed-loop or resolved-loop proof, feature presentation helpers may render
those facts as compact title or step detail inside the existing journey layout.
They must read primary `patrolControl*` fields first, then fall back to legacy
`patrolAutonomy*` compatibility fields, and may fall back to legacy
`proActivation*` fields only when the Patrol-control projections are absent.
They must keep the neutral shared surface chrome stable and must not add
page-local badges, nested cards, or alternate progress widgets for the same
evidence.
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
Alert configuration tables follow the same primitive boundary: the alerts
owner supplies platform-specific threshold groups and filter catalog values,
while the shared FilterBar owns the chip, reset, and "+ Filter" interaction
shape so thresholds do not reintroduce page-local search/tab chrome.
Platform sub-routes that add native provider inventory must stay on the shared
platform page and table primitives. The vSphere Networks surface routes through
`/vmware/networks`, the shared platform tab model, the command palette
navigation model, and the canonical table/detail primitives rather than a card
deck or VMware-local page shell. Its rows are canonical `network` resources in
the shared reportable/resource vocabulary, so source badges, resource pickers,
command-palette search, table chrome, and detail disclosure must all consume
shared primitives before VMware-specific presentation logic.
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
The compact Patrol assessment strip may include factual recent activity mix and
trigger-mode labels when those values are derived from the Patrol run-history
and status payloads. Those labels are summary context inside the same strip,
not a replacement status card, CTA band, or page-local nested card.

1. Add shared primitives in `frontend-modern/src/components/shared/`
   Standard command buttons and button-styled route actions belong to the
   shared `Button` primitive family. Feature pages may choose the action label,
   icon, route, click handler, and contextual layout, but secondary/primary/
   danger/outline/ghost button chrome, sizes, focus rings, disabled/loading
   behavior, and safe new-tab link behavior must come from `Button`,
   `ButtonLink`, and `buttonModel.ts`. Empty states, platform page notices,
   Patrol controls, settings panel actions, infrastructure setup controls,
   compact row actions, and other repeated command affordances must not copy
   local `inline-flex ... rounded-md ...` Tailwind shells just because the page
   needs a one-off action. The shared `mdCompact` size owns the common
   settings/action `px-3 py-2 text-sm` shape, the shared `xs` size owns the
   compact settings row-action `px-2.5 py-1 text-xs` shape, the shared
   `settingsActionXs` size owns compact settings/privacy `px-3 py-2 text-xs`
   action controls such as telemetry preview/reset buttons, and the shared
   `iconMd` size owns the settings dialog close-button `h-9 w-9` icon shape.
   Positive completion or continuation actions such as infrastructure handoff
   and reporting exports use the shared `success`, `successOutline`, and
   `successGhost` Button variants instead of carrying page-local emerald action
   shells.
   Patrol approval and remediation controls use that same primitive family:
   Patrol owns approval/reapproval/denial/review/Assistant handoff behavior,
   while `Button` owns success, warning-solid, primary, secondary, ghost,
   disabled, focus, and compact action chrome.
   Shared error-boundary fallback actions are also command buttons: reset,
   reload, and retry controls must compose `Button` so emergency UI does not
   become a separate local button vocabulary.
   Update confirmation and progress modal actions are part of the same command
   boundary: cancel, start, retry, close, history, reload-now, and close-icon
   controls must compose `Button` or `ActionIconButton` instead of carrying
   modal-local blue, neutral, or icon-button class strings.
   Compact icon-only row, inline, and floating action controls belong to
   `ActionIconButton`. Feature surfaces may own the icon choice, click handler,
   label text, and layout slot, but icon-button size, tone, focus ring,
   disabled treatment, title fallback, and accessible name wiring must come
   from that shared primitive rather than page-local `<button>` plus inline SVG
   shells.
   Standalone machine row action triggers follow the same rule: the Machines
   table owns remove-agent semantics and menu placement, while
   `ActionIconButton` owns the compact muted trigger chrome.
   AI Chat follows that same boundary for drawer header controls, session row
   actions, transcript fallback close/download actions, activity-dock queued
   follow-up controls, composer send, footer help/route actions, and compact
   dismiss controls: the Assistant owns chat/session behavior and copy, while
   `ActionIconButton` owns h-5/h-6/h-7/h-8/h-9 sizing, outline, primary, accent,
   warning, info, danger, disabled, title, and focus chrome.
   AI Chat message and tool copy controls follow the copy-action boundary:
   MessageItem and ToolExecutionBlock own the copied text and timer semantics,
   while `CopyValueButton` owns copied-state iconography, disabled handling,
   focus, and embedded-row propagation behavior.
   Global app-shell prompts are part of the same action boundary.
   `frontend-modern/src/components/GitHubStarBanner.tsx` may own its display
   timing, product copy, and GitHub destination, but its primary, defer, and
   dismiss controls must compose `Button` and `ActionIconButton` instead of
   carrying local floating-prompt button shells.
   Settings selection helpers such as `ResourcePicker` must use the same
   `Button` primitive for select-all, clear, and chip remove actions instead of
   restoring footer-local action shells.
   Reporting surfaces must use the same primitive for retry, generate, and
   export actions rather than restoring large local CTA button shells.
   Self-hosted commercial plan, retry, activation, and clear-key actions follow
   the same shared Button boundary: commercial surfaces own the labels,
   entitlement state, and click handlers, while `Button`, `ButtonLink`, and
   `UpgradeButtonLink` own the primary, outline, warning, and upgrade/link
   chrome.
   Manual self-hosted key recovery is a secondary detail in that same boundary:
   settings surfaces may expose the fallback, but they must label it as license
   recovery/key recovery and keep normal checkout plus the Pro Patrol-mode setup
   path ahead of recovery mechanics or activation-key terminology.
   Hosted billing admin organization row actions follow the same boundary:
   cloud-paid surfaces own Suspend, Activate, Reload, tenant state, and mutation
   semantics, while `Button` owns the row-action chrome through the secondary
   `sm` and `xs` sizes.
   Security authentication settings actions follow the same boundary:
   security/privacy owns auth setup, password-change, credential-rotation, and
   read-only semantics, while `Button` owns the warning, primary, secondary,
   and settings-action chrome.
   Organization RBAC settings actions follow the same boundary: organization
   settings owns role creation, role editing/deletion, user-access assignment,
   feature-gate, and row-action semantics, while `Button` / `ActionIconButton`
   own primary, ghost, accent, danger, focus, disabled, and settings-action
   chrome.
   Organization overview, access, invitation, member, and sharing actions stay
   in that same primitive family: organization settings owns membership and
   share semantics, while `Button` owns primary, danger-outline, success-ghost,
   danger-ghost, disabled, focus, and row-action chrome.
   If a new surface needs a variant that the shared primitive does not expose,
   extend the primitive and registry guard rather than adding a page-local
   class string.
   Drawer header command and icon actions belong to that same shared Button
   primitive family. Workload and infrastructure drawers may own which actions
   appear and the action labels, but the `h-8` Assistant, copy-context, close,
   and future drawer-header action chrome must compose
   `DrawerHeaderActionGroup`, `DrawerHeaderActionButton`, or
   `DrawerHeaderIconButton` instead of copying drawer-local button classes.
   Copy-value affordances belong to the same shared button family. Feature
   surfaces may own the copied value, success/error notification, and adjacent
   product copy, but icon/chip copy controls must use `CopyValueButton`, and
   copyable command/path/value rows must use `CopyableCodeRow` instead of
   recreating local copy icons, copied-state checks, disabled empty-value
   handling, or `font-mono` code-row shells.
   Compact information-card frames in drawer overviews, discovery summaries,
   resource detail sections, web-interface URL editors, shared overview cards,
   and storage backup empty states belong to `InfoCardFrame`. Feature surfaces
   own the title, rows, actions, and sizing context, but the bordered
   `bg-surface p-3 shadow-sm` frame must be composed through
   `InfoCardFrame`, `getInfoCardFrameClass`, or `INFO_CARD_FRAME_CLASS` rather
   than copied as a page-local class string.
   Read-only metadata chips belong to `MetadataBadge` and domain wrappers over
   it. Organization role and share-status chips must use
   `OrganizationRoleBadge` and `OrganizationShareStatusBadge`, so role/status
   tone mapping, pill shape, fit behavior, and whitespace handling do not drift
   across organization overview, access, and sharing surfaces.
   Patrol finding, investigation, approval risk/state, tool-call result,
   run-history/status-bar resource, outcome, snapshot, scoped-run,
   workspace-tab count, and contextual metadata chips must also compose
   `MetadataBadge`; Patrol remains the label/count/semantics owner, but the
   visible badge shell, sizing, tone vocabulary, and whitespace behavior stay in
   the shared primitive and `shared-template-registry.json`.
   Proxmox backup source/state chips follow the same boundary: storage/recovery
   owns backup-source labels and state semantics, while the visible chip shell
   and tone vocabulary route through `MetadataBadge`.
   Inline detail content belongs to the shared detail-section primitive family.
   Feature surfaces may own the platform-specific rows, section labels, and
   source model, but section row shaping, empty-row compaction, value tone
   classes, table rendering, and inline close-action chrome must come from
   `detailSectionModel.ts`, `DetailSectionTable`, and `InlineDetailPanel`
   instead of local `DetailField` grids or provider-named reusable primitives.
   Resource-detail drawer byte labels, integer labels, and count pluralization
   are part of that same primitive family: provider drawer models choose the
   fields and domain labels, but numeric detail values must route through
   `formatDetailBytesValue`, `formatDetailIntegerValue`, and
   `formatDetailCountValue` in `detailSectionModel.ts`.
   VMware vSphere drawer detail sections follow the same boundary as TrueNAS
   and Kubernetes: vSphere owns which rows are meaningful, while row shape,
   section shape, tone classes, and table rendering must stay on
   `DetailSection`, `DetailRow`, `makeDetailRow`, `compactDetailRows`,
   `compactDetailSections`, and `DetailSectionTable` rather than provider-local
   row/section aliases or custom vSphere card loops.
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
   Infrastructure table chrome on active platform/runtime pages must use
   mode-oriented labels for table presentation controls: grouped table mode is
   `Grouped`, not `Cluster`, because cluster remains a platform/resource
   concept for Proxmox, Kubernetes, and similar inventory details. The retired
   top-level infrastructure feature directory must not be recreated for table
   chrome ownership.
   Settings shell search copy belongs to
   `frontend-modern/src/utils/settingsShellPresentation.ts`. Shared settings
   search must not display non-actionable shortcut chips such as `Any key`;
   if the shell exposes a shortcut hint, it must name an actual key chord,
   otherwise the hint must remain unset so the shared `SearchInput` renders no
   shortcut chip.
   Native select state belongs to the shared
   `frontend-modern/src/components/shared/FormSelect.tsx` primitive. It must
   apply controlled `value` props after options are mounted so settings panels
   such as Discovery show the persisted option instead of falling back
   to the first option while the collapsed summary shows a different value.
   The Assistant runtime controls in
   `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx` — e.g. the
   service context scan `Toggle` — are settings-shell chrome bound to the canonical
   `useAISettingsState` form and `/api/settings/ai` payload, not local browser
   state. Each must bind to a `state.form.*` field, round-trip through the
   field-by-field settings payload, and source its label, help, and summary copy
   from `frontend-modern/src/utils/aiSettingsPresentation.ts` rather than inlining
   strings or reaching for a bespoke fetch outside the canonical payload. There is
   no cloud-context-privacy control here: cloud context behavior is a fixed posture
   (see `ai-runtime`), not an operator setting.
2. Add feature-specific presentation only when no shared primitive should own it.
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
3. Add guardrail tests when a new shared pattern is introduced.
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
   from a platform-owned product surface such as a recovery tab, the first
   highlighted step should match that route instead of always restarting at
   Dashboard.
4. Keep shared infrastructure shell state on the reusable settings boundary: `frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts` and `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx` must continue to derive provider counts and shared subtab copy from one infrastructure-settings source — via the unified aggregator through `frontend-modern/src/components/Settings/useConnectionsLedger.ts` — instead of creating provider-local summary fetches or VMware-only shell vocabulary. Phase 9 retired the old `PlatformConnectionsWorkspace` per-type shell; setup guidance should now use `Add infrastructure` plus source-strategy language for API-backed onboarding. The standalone connections-table presenter is retired; `frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx` is the only landing-ledger presenter for configured infrastructure rows, and it must exclude agentless availability probes because those belong to `frontend-modern/src/components/Settings/AvailabilitySettingsPanel.tsx`.
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
   single API-platform probe path instead of a duplicate toolbar action. It may
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
   that destination. `InfrastructureSourceManager.tsx` exposes one Network
   discovery status/action band from the shared landing shell with scan state,
   saved scope, last result metadata, errors, `Run discovery`, `Discovery
settings`, and candidate review when discovered sources are waiting. It must
   not start a network scan just because the page rendered. New-source
   admission belongs on the table's per-platform `Add` actions, the compact
   first-run/readiness actions, or the discovery band's explicit review action,
   and the direct address-probe utility may appear as first-run setup guidance
   instead of a second saved-network-scan command.
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
   Active infrastructure settings and platform/runtime surfaces inherit that
   same source/platform vocabulary. Settings may label configured ingestion
   entries and endpoint probes as `Source`, while resource tables label their
   primary identity column as `System`. Lower-level unified-resource contracts
   preserve merged-source detail for tooltips, accessibility metadata, and
   routing. Collection methods such as Pulse Agent and runtime capabilities
   such as Docker may appear as option or detail labels, but they must not
   become the primary top-level system wording when a provider/API platform or
   reported host OS/appliance identity better explains what the operator is
   looking at.
5. Keep settings deep-link route selection on the shared settings-navigation boundary. `frontend-modern/src/components/Settings/settingsNavigationModel.ts` and `frontend-modern/src/components/Settings/useSettingsNavigation.ts` must treat the canonical PBS and PMG Proxmox deep links as agent-selection authority even though those URLs resolve to the shared `infrastructure-operations` tab. Reloading or remounting on a PBS or PMG deep link must not silently fall back to the PVE selector state. Assistant OAuth callback compatibility queries such as `ai_oauth_error` and `ai_oauth_success` must route the bare settings root to Pulse Intelligence > Provider & Models while preserving the query long enough for `useAISettingsState` to consume and clear it, rather than normalizing the user back to Infrastructure and dropping the callback result.
6. Keep shared storage feature presenters on canonical platform truth. When reusable storage presenters under `frontend-modern/src/features/storageBackups/` classify canonical resources for the shared storage route, API-backed virtualization datastores such as VMware must stay inventory-only datastores instead of inheriting PBS-specific backup-repository or protected-target copy from older fallback branches.
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
   Workload page membership must use the canonical `platformScopes` list when
   present instead of treating `platformType` as exclusive ownership. A Docker
   or Podman app-container can therefore carry both the container runtime lens
   and its owning platform page in routing/filter context, such as Proxmox
   when the runtime is detected inside a PVE LXC, while TrueNAS app containers
   stay scoped to TrueNAS even when their runtime metadata uses the shared
   Docker facet. That membership overlap is not permission to duplicate the
   detailed container table into every platform overview: Proxmox Overview
   keeps the default Workloads peer table to VMs and LXCs, and Docker-in-LXC
   evidence belongs as LXC drawer detail while `/docker`
   remains the canonical detailed Docker / Podman container lens. The overview
   table should not add peer rows, badges, or child rows for Docker containers;
   those signals compete with VM/LXC state and belong one click down. The
   default row may carry only a quiet icon/count cue beside the guest name to
   show nested runtime presence; names, metrics, state, and actions stay in the
   drawer or Docker lens.
   Drawer-to-runtime navigation is still part of the shared platform-table
   affordance contract: when the LXC drawer exposes an `Open Docker` action for
   nested containers, that action must use the Docker host facet route state so
   the target Docker Overview opens scoped to the same runtime instead of a
   broad, visually unrelated container list.
   Primary navigation uses that same membership model: the Docker route is
   the container-runtime lens and may be labelled `Containers` in the shell,
   while shared source badges, filters, and runtime management copy continue
   to use `Docker / Podman` where the capability itself is being named.
   Kubernetes workload rows on `/kubernetes/workloads` must render through the
   Kubernetes-native workload tables rather than a generic infrastructure or
   inventory table. Pods render through
   `frontend-modern/src/features/kubernetes/KubernetesPodsTable.tsx` with
   Pod-native phase, readiness, restart, owner, node, image, and age columns;
   legacy `/kubernetes/pods` resolves to the same workflow. Controller rows
   render through
   `frontend-modern/src/features/kubernetes/KubernetesControllersTable.tsx`;
   legacy `/kubernetes/controllers` resolves to the same workflow. The table
   boundary preserves platform-native API fields for StatefulSets, DaemonSets,
   Jobs, and CronJobs, including targets, active/current counts,
   ready/succeeded counts, availability, exceptions, service names, schedules,
   and last run metadata.
7. Keep shared source/platform vocabulary on the governed manifest boundary. `frontend-modern/src/utils/platformSupportManifest.generated.ts` must be the tracked frontend projection of `docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json`, `frontend-modern/src/utils/platformSupportManifest.ts`, `frontend-modern/src/utils/sourcePlatforms.ts`, and `frontend-modern/src/utils/sourcePlatformOptions.ts` must consume that generated projection instead of embedding divergent future-label lists, setup/onboarding path allowlists, host-profile labels, surface-kind guesses, readiness-state guesses, or presentation-only guesses, and `frontend-modern/scripts/canonical-platform-audit.mjs` must fail when the generated projection drifts from the governed manifest. The generated governance/readiness split is authoritative: supported platform arrays drive current support claims, while admitted platform arrays may keep route/navigation and add-flow vocabulary available without turning `first-lab-ready` entries such as VMware into supported-source copy. Kubernetes manifest projections must enumerate the native API-backed page sections the shared tab shell can expose, including controllers, networking, storage, config, policy, autoscaling, and events, so platform pages do not invent local support claims outside the governed JSON. The generated `surface_kind` is the machine-readable boundary between owning platform entries and runtime lenses: `docker` is a `runtime-lens`, not a `platform`, even when the container-runtime route stays available as a primary shell destination. The generic `docker` source-platform label is "Docker / Podman" in shared selectors, badges, and filter options so v5 Docker users can find the runtime surface while Podman-backed rows are not mislabeled as Docker-only; "Container runtime" remains the governed runtime family, not the primary customer-facing label. Identity colour is semantic, not page-local decoration: shared source/platform badges, host identity badges, and container runtime badges must use the shared presentation helpers so Docker remains on the Docker/Podman blue runtime tone, Podman uses its distinct runtime tone, Proxmox PVE remains orange, and those meanings do not drift across table rows, filters, drawers, or platform pages. Agent host-profile entries, including Unraid, stay in the generated `agentHostProfiles` projection and shared wrapper helpers; frontend primitives may render those labels for Pulse Agent install/identity copy but must not add them to the first-class platform union.
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
8. Keep summary chart interaction identity on one shared helper. Summary surfaces that expose row-hover, group-hover, chart-hover, or route-focus-driven chart emphasis must derive page/group/entity scope through `frontend-modern/src/components/shared/summaryCardInteraction.ts` and pass that same resolved scope into card-state, sparkline, and density-map primitives, rather than letting cards read `hovered || focused` while charts listen to a different page-local ID source. Hovering one summary chart must promote that series into the shared active entity so sibling cards highlight the same object instead of keeping chart-local hover islands, and hovering or pinning a workload group header, infrastructure cluster header, or storage pool-group header must scope the matching summary cards through that same shared contract instead of forking a page-local summary filter path. Sibling cards should surface that synchronized hover as one compact header readout through the shared summary-card contract, while the chart under the pointer keeps the only floating tooltip. Recovery is explicitly outside this interaction dialect: its retired posture-card strip must not return with row/group/chart hover behavior without a separate governed product decision.
9. Keep page summaries page-scoped when table rows enter contextual focus. Route-backed row selection may add a focused label and shared series emphasis, but infrastructure, workloads, and storage summary cards must continue to render the page-level series set instead of collapsing the summary down to the selected row or replacing the global trend view with row-local empty states.
10. Keep contextual row focus on the shared summary primitive. Summary surfaces and same-route table drill-ins must reuse `frontend-modern/src/components/shared/contextualFocus.ts` for interactive-series filtering, focused-name lookup, active-series derivation, local scroll preservation, and deliberate inline-detail reveal instead of rebuilding page-local `Set` filters, focused-label scans, drawer-aware scroll math, or ad hoc scroll restoration in each surface.
11. Keep summary-linked table row emphasis on the shared primitive contract. Workloads, infrastructure, and storage rows that mirror the active summary entity must expose that state through `data-summary-row-active` and let the shared presentation in `frontend-modern/src/index.css` render the row emphasis, rather than carrying page-local sky or blue fill classes inside each row renderer. Group-scoped preview and pin must use that same shared presentation boundary: child rows that belong to a hovered or pinned summary group should expose `data-summary-group-member-active="preview|pinned"` so the block-level emphasis stays subtle, consistent, and reversible instead of each table inventing its own outline, badge, or full-strength fill treatment. Static grouped row headers on workloads, infrastructure, storage, recovery, and future grouped tables must use `frontend-modern/src/components/shared/groupedTableRowPresentation.ts` plus the `.grouped-table-row` CSS contract in `frontend-modern/src/index.css`, rather than rebuilding local `bg-surface-alt` variants with subtly different light/dark behavior or page-local left-accent markers. That shared grouped-table primitive owns the subgroup cell padding, typography, small metadata, and badge treatment as well as the row background token, so a future adjustment to the subgroup visual language changes every grouped product table from one owner. Inline table detail rows on platform, workload, and infrastructure tables must compose `frontend-modern/src/components/shared/InlineDetailTableRow.tsx` for the full-width row, surface-alt cell, detail padding, and row-click containment instead of rebuilding page-local `TableRow` / `TableCell` / `div` shells around each drawer. Storage-backed reusable row presenters under `frontend-modern/src/features/storageBackups/` must also keep row height and alert accents on class/data-attribute presentation instead of runtime inline style maps, so the shared table contract stays CSP-safe on both steady-state and alert-highlighted routes.
12. Keep retained-value data loading honest at the ownership boundary. Helpers
    that prevent a feature surface from falling through the app-level Suspense
    boundary during in-flight refresh should stay feature-local until multiple
    governed surfaces truly share the behavior. Once that boundary is shared,
    promote the helper into an explicit shared hook owner such as
    `frontend-modern/src/hooks/createNonSuspendingQuery.ts` rather than
    re-copying suspense-escape logic into each feature area or burying it
    inside one feature's private state model.
13. Keep shared commercial warning banners truthful about destination intent.
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
14. Keep assistant availability bootstrap on the shared app-shell boundary.
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
    active behind the modal. Non-critical app-shell prompts, including
    promotional or feedback prompts, must not use the shared blocking dialog
    stack because they must not suppress Pulse Assistant access or look like
    required operational acknowledgement.
    AI-owned frontend surfaces that need shared settings or model-catalog
    truth must route those reads through
    `frontend-modern/src/stores/aiRuntimeState.ts` rather than each feature
    bootstrapping `/api/settings/ai` or `/api/ai/models` independently.
    Non-AI settings panels such as
    `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
    must stay on the app-shell assistant-availability fact instead of
    re-reading raw AI settings just to decide whether assistant affordances
    should render.
15. Keep Patrol shell composition and product-first provider vocabulary on the
    shared feature-presentation boundary.
    `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx`,
    `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`,
    `frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`,
    `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`,
    `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`,
    `frontend-modern/src/components/patrol/RunHistoryEntry.tsx`, and
    `frontend-modern/src/utils/patrolRuntimeActions.ts` must keep
    Patrol assessment, verification, and findings primary; surface recent
    changes, learned correlations, and policy coverage only as backend,
    Assistant, selected-finding, or selected-run context when investigation
    makes that evidence relevant; and use Patrol/provider wording for the shared provider settings,
    provider model, and provider circuit-breaker affordances instead of
    generic AI labels inside Patrol-owned shells. The shared app shell in
    `frontend-modern/src/App.tsx` and `frontend-modern/src/AppLayout.tsx` must
    likewise expose `/patrol` as the canonical route and navigation target,
    while retired `/ai` browser entry points stay unregistered rather than a
    second Patrol-branded primary route. `PatrolIntelligenceHeader.tsx`
    must also keep the page heading's accessible name singular: when the
    `PulsePatrolLogo` appears beside visible Patrol heading text, it is
    decorative rather than a second label source. The Patrol workspace must not
    expose a generic Details/supporting-context panel for nearby activity,
    learned correlations, or policy buckets; those payloads stay backend and
    Assistant context rather than a default page section. Patrol initial data
    refresh failures must stay inside the Patrol feature shell as one compact
    stale-data retry banner; they must not replace the route with Suspense,
    blank loading, raw transport errors, or page-local diagnostic panels. The Patrol
    investigation-context owner normalizes same-state recent-change records into
    changed-substate wording before Assistant handoff renders them. The same shared feature-shell
    boundary owns the
    commercial-facing Patrol capability language: autonomy segmented controls
    and run-history/result labels must present the operator-facing policy levels
    as `Watch only`, `Ask first`, `Safe auto-fix`, and `Autopilot`, while legacy
    API names remain hidden from operators and compact controls do not collapse
    into unexplained shorthand. Patrol run-history rows must lead with what
    Patrol did or could not do before exposing trigger, token, tool-call, or raw
    trace details. Plan-locked Patrol controls must keep the free watch-only
    surface clean and must not render a Pro-absence explainer, a disabled
    paid-level matrix, compact Pro badges, or any paid-mode disclosure. The free
    Patrol working surface stays clear of paid-feature surfacing entirely; Pro
    discovery belongs in Settings, website/docs, and contextual at-need prompts,
    not beside the daily-use selector. The one allowed at-need prompt is a
    single finding-level Pulse Pro capability line in the expanded finding
    primary-action area for plan-locked installs on active critical or warning
    findings, with its upgrade action gated by the upgrade-prompt policy. Visible product copy calls the selector `Patrol mode`; compatibility route and wire identifiers may keep stable names
    such as `patrol_control` and `patrolControl*`. The always-visible Patrol mode selector must stay on
    the selected mode and one plain summary, without a separate `Limits`
    disclosure or hard-limit matrix beside the picker. Shared feature shells must not invent their own Patrol safety
    thresholds, policy labels, or disabled-control explanations. The Patrol page
    header must consume the same effective control state, using watch-and-report
    copy for locked or `Watch only` mode and full governed-operations copy only
    for modes where that capability is actually available. Paid-control
    availability and commercial-plan copy must describe the same decision as
    choosing what Patrol may handle automatically; they must not ask users to
    decide how far Patrol can go or how much control Patrol has. The
    Open work workspace copy belongs to that same product-facing boundary:
    empty and descriptive text must explain what Patrol-found problems will
    appear there, what the selected control level allows, and the next useful
    operator action; it must not fall back to activation-loop, proof, queue, or
    verification-accounting language. Active Patrol issue rows may use shared
    definition-list and muted text primitives to show problem, affected
    resource, checked evidence, next step, and verification state inside the
    row, but that scaffold must not become a nested card, status strip, trust
    strip, or page-level proof block. A calm Patrol queue must not use shared
    compact-list or badge primitives to create protection-current,
    verification-waiting, schedule-freshness, drift, trust, or proof strips;
    empty work belongs to the plain empty-state and deliberate History affordance.
    Monitor-context Patrol coverage posture must not use the shared compact list
    and badge primitives as a generic Proxmox overview or monitor-first
    launch-page proof strip. A future scoped
    monitor affordance may use these primitives only when it is attached to an
    operator action or selected Patrol context, uses distinct monitor labels,
    and does not become a nested card, generic dashboard strip, trust summary,
    or duplicate Patrol empty-work list. The
    Patrol schedule and model drawer is part of that shared
    feature-presentation boundary: it must stay viewport-bounded, expose an
    accessible dialog label, keep the four-level control policy on the default
    Patrol header, and keep provider model, schedule, trigger tuning, and
    readiness validation inside the secondary disclosure. Backend save rejection reasons must pass
    through as inline dialog state instead of being replaced with generic toast
    copy, and that advanced disclosure must open when the inline state exists.
    When the failure includes
    Patrol readiness context, the inline state must expose the provider, model,
    and readiness summary next to a direct provider-settings action instead of
    hiding that diagnosis behind Assistant alone. The provider-model selector in
    that popover must stay bound to the shared runtime settings/model catalog
    even when the popover mounts after async catalog loading, but the full
    catalog must stay behind an explicit change action so the default advanced
    drawer leads with the current effective model summary rather than a raw
    provider route list. A saved direct-provider Patrol model still renders as
    that model instead of visually falling back to the default selection.
    Successful provider-model saves that return a not-ready Patrol
    readiness snapshot must use that same inline surface with `needs attention`
    wording, while Assistant receives a saved configuration issue rather than a
    failed-save handoff. When governed fixes are locked, the same Patrol state
    owner must clear stale full-mode unlock state before persisting the
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
16. Keep Pulse Intelligence settings product-first and page-scoped.
    `frontend-modern/src/components/Settings/AISettings.tsx`,
    `frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
    `frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
    `frontend-modern/src/components/Settings/settingsNavigationModel.ts`,
    `frontend-modern/src/components/Settings/settingsPanelRegistry.ts`,
    `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`,
    `frontend-modern/src/components/Settings/useAISettingsState.ts`, and
    `frontend-modern/src/utils/aiSettingsPresentation.ts` must present that
    surface to operators under the `Pulse Intelligence` settings group as
    separate focused pages rather than as a generic `AI Services` shell or one
    oversized mixed form. `Provider & Models` owns API keys, default model
    selection, provider health/preflight, provider runtime budget/timeout, and
    usage/cost visibility. `Patrol` owns schedule, alert/anomaly triggers,
    runtime readiness, Patrol model override, and a simple `Open Patrol` handoff
    to the `/patrol` operator page; the actual watch/investigate/act/verify/record
    operator loop stays on `/patrol`. `Assistant` owns chat/tool permission,
    command-access, model override, and session maintenance. Service context may
    exist under Pulse Intelligence only for model-backed or continuous service
    discovery that supplies Assistant and Patrol context; normal infrastructure
    discovery and onboarding remain under Infrastructure, and the Pulse
    Intelligence navigation item, route header, model override, reset/save
    affordances, and setup copy must use the `Service Context` label so
    operators do not confuse it with infrastructure discovery. The
    `Provider & Models` page must not carry a discovery summary,
    Patrol-control banner, or Patrol CTA; the Patrol-control handoff belongs on
    the `Patrol` settings page and `/patrol`, where copy describes Patrol
    autonomy in plain operator terms rather than exposing an internal
    `operations policy` concept. Settings-save
    feedback must preserve provider-specific preflight failures and successful
    save responses that carry Patrol readiness warnings, including the provider,
    selected Patrol model, failure cause, safe recommendation, and readiness
    summary when those fields are present. The settings shell may compose that
    safe backend diagnostic for display, but it must not infer provider
    remediation by parsing raw upstream error strings in the browser. Provider
    setup cards must describe provider families through the current
    backend-owned provider contract; DeepSeek setup copy is the V4 family and
    must not regress to old V3 or compatibility-alias wording. First-class
    provider cards on `Provider & Models` must remain model-driven through
    `AI_PROVIDERS`, `AI_PROVIDER_CONFIGS`, and the `useAISettingsState`
    provider payload mapping: adding direct chat-compatible providers such as
    Z.ai, Groq, Mistral, Cerebras, Together, or Fireworks extends those shared
    arrays/maps and the backend registry projection instead of introducing
    provider-specific JSX branches, local configured-state inference, or
    browser-owned default endpoint facts.
    Provider connection controls are page-scoped: the global Pulse
    Intelligence enable toggle, provider readiness strip, and Test Connection
    action belong to `Provider & Models`; Patrol, Assistant, and Service Context
    subpages keep their focused settings plus reset/save actions without
    repeating provider health chrome. Those reset/save actions and save
    notifications must be page-scoped as well: saving `Patrol`, `Assistant`,
    or `Service Context` settings must not report
    `Provider & Models settings saved` or render a generic `Save changes`
    affordance that hides which Pulse
    Intelligence page owns the change.
    Runtime controls inside `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`
    must stay split by page-specific exports: provider runtime controls on
    `Provider & Models`, Assistant chat actions on `Assistant`, and service
    context controls on `Service Context`. Assistant chat-action copy
    must name Patrol control as configured on the Patrol page, not as an
    Assistant command mode, because `/patrol` remains the operator surface for
    choosing how much autonomy Patrol has. Service context copy must describe
    the model-backed loop that supplies concrete service facts to Pulse
    Assistant and Patrol, not as generic discovery or AI context.
    `frontend-modern/src/components/Settings/useAISettingsState.ts`
    must save service context scan enablement and interval as one explicit
    settings pair so selecting "Every 6 hours" or "Manual only" round-trips
    through `/api/settings/ai` without depending on stale read-side diffing.
    The same Service Context settings section must expose a manual
    context-scan action wired through `/api/discovery/run` when
    service context scanning is enabled in manual-only mode, while resource-drawer
    discovery remains the forced single-resource refresh path. The collapsed
    section and run-action copy must make automatic scheduling visible by
    distinguishing `Auto <interval>`, `Manual only`, and `Off`, and the run
    action must describe whether it is running the scheduled scan or a
    one-off manual-only sweep rather than implying recurring scans were
    enabled.
    Assistant-only controls such as execution permissions and session
    maintenance must stay explicitly labeled as Pulse Assistant controls, while
    Patrol schedule and trigger readiness live on the Patrol settings page and
    Patrol autonomy/control level lives on the `/patrol` operator page rather
    than drifting back into the provider shell. Session maintenance is limited to
    Pulse-owned conversation operations such as summarization; OpenCode-style
    file diff, revert, or unrevert actions must not appear in Settings unless
    Pulse owns a real governed infrastructure action-history/reversal contract
    for the affected resources. Shared/default model choice belongs on
    `Provider & Models`, while Assistant, Patrol, and service context model
    overrides belong on their respective settings pages instead of a generic
    advanced AI bucket. Each per-surface override must fall back to the shared
    default when
    left empty rather than silently using a hard-coded backend default, so
    `Provider & Models` stays the single place an operator picks the default
    model for all three surfaces. Assistant model copy must describe chat,
    explanation, and review support; it must not present Assistant as the
    approved-fix executor because Patrol is the hands-on operator for checks,
    governed fixes, and verification.
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
    catalogs remain usable on mobile and tablet layouts with bottom navigation,
    and it must flip above its trigger when prompt/composer chrome leaves
    insufficient room below. Caller-owned alignment may choose left or right
    anchoring, but the shared picker still owns the fixed-position dropdown,
    viewport cap, search shell, result list sizing, and keyboard navigation
    model. Chat-owned selectors must reuse this shared picker instead of
    carrying a parallel dropdown implementation. Recent/priority model
    sections, external open-and-focus requests, selected older model visibility,
    route labels, explicit `provider:model` custom-route validation, and
    catalog-disclosure rows belong to the shared picker so Assistant, settings,
    and future model-selection surfaces do not drift apart. Unknown custom or
    recent routes may remain visible only when
    they have a valid non-empty provider and model segment; malformed route
    strings such as empty provider/model values, URL-shaped text, whitespace,
    or path-only payloads must be dropped instead of becoming selectable model
    routes. The shared picker must also mark the selected catalog, recent,
    override, custom, or inherited-default route as the current row with
    visible `Current` metadata and `aria-selected`; selected model state must
    not be communicated by background color alone. The model picker dropdown is
    a named search/listbox surface: opening it must focus search, the trigger
    must expose its owned listbox while expanded, and keyboard movement from
    search through the option rows must support current-row focus,
    filtered-result focus, catalog-disclosure focus, up/down, page, home/end,
    Enter/Space activation, and Escape return to the trigger so model choice and
    catalog expansion do not depend on mouse interaction. Picker-owned
    navigation keys, including Escape, must be consumed by the picker so parent
    shells do not also treat the same keypress as drawer or page-level Escape.
    Gateway-routed model choices must not look like direct-provider choices:
    the shared picker, System AI settings status strip, and per-surface
    inherited-default descriptions must render OpenRouter-hosted provider
    models with an explicit `via OpenRouter` route label while leaving direct
    DeepSeek/OpenAI/Anthropic/Gemini/Ollama selections unqualified. When a
    selected route also carries a shared-default or override badge, the shared
    picker owns that badge as separate metadata in both visible text and the
    button accessible name; labels must render as `model via OpenRouter ·
default` instead of fusing provider and badge text such as
    `OpenRouterdefault`.
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
    Source-specific platform product surfaces under
    `frontend-modern/src/features/`, such as the Proxmox Backups tab, may own
    domain IA and row models in their product subsystem while consuming shared
    primitives for table shells, alignment, filter buttons, charts, and empty
    states. Frontend-primitives owns those reusable controls and guardrails,
    not the storage/recovery semantics that decide which PBS, PVE archive,
    snapshot, task, or workload-coverage rows are shown. Storage/recovery-owned
    Proxmox backup subcomponents may live under `frontend-modern/src/features/`
    when they are listed in the subsystem registry and continue to compose the
    shared filter, table, chart, and empty-state primitives rather than local
    shell variants.
    `frontend-modern/src/AppLayout.tsx` may extend the `PrimaryTab` list with
    new platform or runtime-family entries, but primary navigation is a
    support-and-evidence-gated surface: rendered tabs, command/search
    destinations, keyboard shortcuts, and authenticated landing fallbacks must
    derive from the governed support manifest plus current runtime resource
    evidence. Supported platform/runtime families appear when evidence proves
    they are present; admitted-only, presentation-only, unsupported, or absent
    families stay hidden rather than rendering as disabled placeholders.
    The `MOBILE_NAV_PLATFORM_PRIORITY` ordering in
    `frontend-modern/src/components/shared/mobileNavBarModel.ts` mirrors
    that platform-first set only, so mobile and desktop navigation stay aligned
    without reintroducing aggregate Workloads / Storage / Recovery workspace
    tabs or the legacy Infrastructure entry.
    Frontend primitives owns the sole user-facing `Machines` IA contract for
    the support-manifest `agent` platform and agentless availability endpoints.
    The compatibility route, internal navigation id, and builders remain
    `standalone` / `buildStandalonePath()`; adjacent subsystem contracts may
    reference this owner for dependencies but must not restate the route,
    navigation, or landing semantics. Its primary tab, mobile priority,
    command-palette destination, and keyboard shortcut must all route through
    `buildStandalonePath()` and the `PrimaryInfrastructureNavId` `standalone`
    evidence gate; they must not create a generic Hosts, Nodes, Other, or
    mixed-systems bucket, and they must not include provider-owned platform
    nodes that are not canonical machine-page resources. The Machines page is a
    platform/runtime page, not a legacy Infrastructure page: it must use the
    shared platform tab, toolbar, table-card, and kind-aligned column
    primitives, and it must not reintroduce the old top-of-page
    InfrastructureSummary chart strip. The Machines surface must also remain
    secondary in the shell hierarchy when provider/runtime platform evidence exists:
    `PRIMARY_INFRASTRUCTURE_NAV_IDS`, desktop primary tabs, mobile primary
    priority, app-shell preload order, authenticated landing fallback, and
    command-palette ordering must prefer Proxmox, Docker, Kubernetes,
    TrueNAS, and vSphere ahead of Machines. The Machines surface may win those
    first/default positions only when the current estate has standalone Pulse
    Agent machines or agentless availability endpoints and no
    provider/runtime platform evidence.
    Patrol workflow components under
    `frontend-modern/src/features/patrol/` may compose shared `Button` and
    `ButtonLink` chrome for issue actions, but the workflow state, route
    anchors, single-finding direct-action selection, Assistant handoff, autonomy
    label, and multi-finding fallback semantics stay with `patrol-intelligence`;
    the canonical Patrol control anchor belongs on the visible selector, not the
    workspace shell. Setup-only readiness may hide run, schedule, model, trigger,
    and provider-repair controls, but it must not hide that selector or replace
    it with a setup/status explainer. Shared primitives must not grow
    Patrol-specific activation, autonomy, provider-settings, or
    Assistant-routing behavior. The default loop is a
    Patrol-owned watch / investigate / act under policy / verify / record loop.
    Active current-issue expansion is a task surface, not a history transcript:
    raw finding lifecycle rows may render in explicit all/resolved/history or
    selected-run review states, but not in the default active Patrol issue
    expansion.
    Compact Patrol status chrome may render work/health evidence passed by
    Patrol, but trigger/scheduling status remains header/control context and
    must not be repeated by shared default status primitives. Shared primitives
    must preserve that plain visible label instead of exposing internal
    assessment terminology on the default page.
    External-agent readiness from Pulse MCP may remain compact optional context
    derived from the shared manifest-client contract verdict and backend
    operations-loop `externalAgentReady` signal, but shared primitives must not
    create page-local MCP setup constants, token-scope checks, tool filters,
    readiness shortcuts, MCP readiness props, or a visible external-agent stage
    as the primary first-party loop. The journey's loaded progress state must come from the
    canonical operations-loop status projection exposed by the shared
    agent-capabilities frontend client, while shared primitives remain passive
    renderers of the state Patrol passes them. Patrol control starter,
    completed-loop, or resolved-loop evidence may change compact journey copy
    only after Patrol has derived it from that projection; shared primitives must
    not infer Patrol control or legacy Pro activation state from route anchors,
    billing state, generic Patrol recency, or MCP readiness alone.
    Shared presentation
    helpers may render the operations-loop state they receive, but they must
    not infer operations-loop progress from a generic Patrol run, recency
    timestamp, or MCP readiness alone; Patrol owns the issue-backed evidence
    model that decides when the loop can advance through Assistant, approval,
    rejected no-execution terminal decisions, approved-action verification, and
    external-agent parity.

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
6. User-facing diagnostics panels rendering the native Pulse Assistant runtime
   as an MCP connection. Settings diagnostics must consume
   `assistantRuntimeConnected` and label it as Assistant runtime availability;
   `mcpConnected`, `mcpToolCount`, and "MCP Connection" are forbidden on the
   first-party diagnostics surface.
7. Settings route models normalizing retired aliases such as
   `/settings/operations/*`, `/settings/integrations/api`,
   `/settings/system-pro`, `/settings/workloads/*`, or nested
   `/settings/infrastructure/*` paths back into current settings panels.
   Retired aliases must fail route eligibility instead of being kept as
   compatibility redirects.

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
3. Keep Settings loading placeholders on the shared
   `SettingsLoadingSkeleton` primitive and the
   `settings-loading-skeleton-shell` registry rule instead of local
   `animate-pulse` block templates.
   Pure Settings loading indicators that are spinners rather than skeleton
   placeholders must stay on `LoadingSpinner` and the `loading-spinner-shell`
   registry rule instead of local `border-t-transparent` or `border-b-2`
   animate-spin shells.
4. Update this contract when a new canonical UI pattern is adopted
5. Remove local forks after the shared primitive is introduced
6. Keep shared feature-level presenters on capability truth. When reusable
   presenters under `frontend-modern/src/features/` explain why a control,
   chart, or detail surface is unavailable, they must describe the owned
   identity or capability gap instead of prescribing a provider-local install
   path that conflicts with API-backed platforms like TrueNAS.
7. When a settings route header and a top-level settings shell describe the same
   commercial surface, keep them on the same shared presentation owner instead
   of allowing route metadata in `settingsHeaderMeta.ts` or labels in
   `settingsNavCatalog.ts` to drift into independent title or description copy,
   and keep adjacent settings-shell referrals such as
   `InfrastructureWorkspace.tsx` on that same shared owner instead of
   reintroducing local “go to Pulse Pro” variants.
   That same shared owner must also keep self-hosted commercial settings
   discoverable: the nav label and page shell title use the shared
   `Plans & Billing` label, and the owned plan shell must foreground the active
   plan name plus available
   capabilities before secondary billing or recovery detail so paid upgrades
   can confirm their entitlement immediately after activation without making
   default Community look like it is missing an activation key.
   Routine plan and capability-status copy must stay product-facing: describe
   what is available on the instance, point failed capability checks to refresh
   the plan or open recovery, and avoid raw entitlement-payload phrasing or
   activation language as the normal setup story.
8. When settings surfaces need informational, warning, success, or danger
   callouts, compose `frontend-modern/src/components/shared/CalloutCard.tsx`
   and register the consumer in
   `frontend-modern/scripts/shared-template-registry.json` instead of adding
   feature-local colored panel shells. Compact settings notices must use the
   shared `scale="compact"` density and keep proof in both
   `frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts`
   and
   `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`.
   Connection-editor status, feature-disabled, delete-error, and probe-result
   notices are part of the same settings callout boundary: the editor and
   credential slots own the source-specific lifecycle or API meaning, while
   `CalloutCard` owns the warning/success/danger shell and compact density.
   Update confirmation and progress modal notices share that same primitive
   boundary. The update flow owns version, prerequisite, root-access, restart,
   and error copy; `CalloutCard` owns the info/warning/danger shell, spacing,
   dark-mode tone, and icon layout.
9. Keep hosted settings-shell framing imports safe for bundle initialization.
   Self-hosted billing titles, descriptions, and referral copy used by
   `settingsHeaderMeta.ts`, `settingsNavCatalog.ts`, and adjacent settings
   shells must flow through
   `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
   instead of importing generic commercial presentation helpers directly into
   hosted settings route shells.
   Contextual settings feature gates must use capability-owned presentation
   helpers and neutral paid-plan copy. They must not reintroduce `Pro feature`
   badge titles, Pro-suffixed option labels, monitored-system limit claims, or
   browser-local commercial/onboarding metrics wrappers in SSO, audit,
   reporting, AI controls, agent profiles, or shared warning banners.
10. Keep shared settings-shell AI control copy capability-scoped rather than
    upsell-scoped. `AIRuntimeControlsSection.tsx` may describe read-only,
    approval-required, and autonomous action posture, but option labels and
    helper text must avoid tier labels or broad "executes everything" wording;
    paid capability availability belongs to entitlement-backed visibility and
    lock state, not local select copy. Provider & Models settings copy must keep
    Patrol autonomy distinct from Assistant chat actions: Patrol's
    hands-on control level belongs on the Patrol page, while the shared settings
    shell may only describe whether Assistant chat can run eligible chat
    actions.
11. Keep first-session dashboard empty-state copy on
    `frontend-modern/src/utils/workloadEmptyStatePresentation.ts`, and make
    infrastructure setup guidance name the canonical destination explicitly
    instead of falling back to generic settings CTA labels.
12. Keep the live first-session wizard on the canonical three-step runtime
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
13. Keep AI settings setup UI backend-driven:
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
14. Keep shared filter primitives coherent with source-owned option hydration.
    Active platform/runtime pages and Settings infrastructure surfaces must keep
    canonical options visible in shared filter controls even when current
    results do not contain that option, so provider- or endpoint-scoped
    handoffs do not flash back to generic host-only language.
15. Keep the first welcome screen in
    `frontend-modern/src/components/SetupWizard/steps/WelcomeStep.tsx`
    explicit about operator context. The shell must explain that the bootstrap
    token only unlocks first-run setup, state where the command should run, and
    adapt command/help text to detected Docker or containerized deployments
    instead of assuming the operator already knows which host or container owns
    the Pulse install.
16. Keep the settings-shell infrastructure landing path aligned with that same
    first-session story. `frontend-modern/src/components/Settings/settingsNavigationModel.ts`
    must treat `/settings` and the infrastructure settings tab as the canonical
    path to the bare `/settings/infrastructure`, which renders the unified
    Connections table, not to a separate install subview or to reporting/
    control. The first-session story is owned by that table's own empty state
    and the `Add infrastructure` entry point on it, not by a second landing route,
    so first-time operators and returning operators see one consistent
    infrastructure surface by default.
17. Keep Infrastructure and Workloads onboarding copy on the shared
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
18. Keep cross-surface investigation handoffs on shared route ownership.
    Feature shells such as Alerts and Patrol may decide which governed
    destination chips to render, but canonical href, label, dedupe, and
    infrastructure-fallback truth must stay in
    `frontend-modern/src/routing/resourceLinks.ts` instead of freezing raw
    route strings or provider-local link builders inside feature panels.
    Patrol workflow handoffs follow the same rule: start/continue Patrol
    control links must compose the route-backed `patrol_control` helper from
    `resourceLinks.ts`, single-finding direct action links must use canonical
    finding-presentation destinations such as the Patrol provider-settings
    route, while `patrol_autonomy` and legacy Pro activation URLs remain parser
    aliases only and verified review links use the plain Patrol history anchor.
    UI surfaces must not duplicate the `patrolControlStarter` query string or
    write Patrol control or legacy entry-point starter telemetry from local
    click handlers.
19. Keep shared summary-card emphasis coherent. When shared summary primitives enter an `inactive` state, `SummaryMetricCard`, `InteractiveSparkline`, and `DensityMap` must all demote background context together so storage, infrastructure, and workloads read as one interaction model instead of mixing page-local opacity, sticky-shell, or highlight rules.
20. Keep density-map summaries overview-first. When a shared summary density map receives row focus or chart-hover emphasis, `frontend-modern/src/components/shared/DensityMap.tsx`, `frontend-modern/src/components/shared/useDensityMapState.ts`, and `frontend-modern/src/components/shared/densityMapModel.ts` must preserve the multi-entity overview rows and keep focused-entity detail in the hover tooltip instead of swapping the card into a single-series chart, dimming the rest of the map into unusable background noise, duplicating cursor-value tooltip copy, or adding persistent card chrome that steals heatmap space. The card body must stay overview-first; the tooltip may carry the active entity identity, current value, and peak, shared tooltip shells must follow semantic surface tokens instead of forcing a dark palette in light mode, the tooltip header must let long entity names consume the available width before truncating rather than clipping against an arbitrary fixed label cap, numeric metric readouts such as `16.9 MB/s` or `37.4 MB/s` must stay single-line instead of wrapping the unit onto a second row, and density-map detail that cannot fit cleanly inside the canonical tooltip shell must be omitted rather than introducing tooltip-specific chrome or a secondary chart inside the hover surface.
21. Keep retired self-hosted hosted-model and trial acquisition surfaces out of
    normal v6 GA runtime. Shared shells and helper-driven badges may continue to
    parse legacy payload fields, but ordinary self-hosted Assistant, Patrol, and
    settings flows must present provider setup as BYOK/local/self-managed and
    must not surface hosted-model credits, in-app trial starts, or generic
    managed-model claims.
22. Keep sparkline scrubbing source-local and sibling-sync timestamp-based. The chart a user is actively scrubbing in `frontend-modern/src/components/shared/InteractiveSparkline.tsx` and `frontend-modern/src/components/shared/useInteractiveSparklineState.ts` must keep its dashed hover cursor on the real local mouse `x`, while sibling cards may map the shared hover timestamp onto their own timelines. Shared cursor sync must not snap the source chart back onto the nearest sample timestamp, the rendered SVG/canvas hover cursor must bind to the actual numeric cursor coordinate rather than a boolean guard state, the time cursor must span the chart viewport instead of collapsing to the series height, and the hover tooltip must track the pointer instead of anchoring to the chart top edge while following the active theme rather than a hardcoded dark shell. The hover tooltip must stay side-offset from the active scrub cursor and flip to the available side near viewport edges so it does not cover the highlighted guide or graph point.
23. Keep shared contextual focus canonical after adoption. Once a summary or table surface enters route-backed contextual focus, future additions must extend `frontend-modern/src/components/shared/contextualFocus.ts` and its guardrail tests rather than forking another helper for workload IDs, resource IDs, or scroll-preserving same-route selection.
24. Keep shared infrastructure/resource selectors on the canonical agent-facet
    truth. Shared primitives and settings-facing selector helpers must treat
    top-level TrueNAS appliances as agent-facet infrastructure via shared
    helper ownership instead of reviving a direct `resource.type === 'truenas'`
    branch inside page shells, selectors, or reporting-resource type helpers.
25. Keep shared feature-shell Patrol run fixtures on the canonical run-record
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
    shell therefore owns the shared `PageHeader` for support tools, and the
    retired top-level `/operations/*` browser path must not regrow a route-local
    heading, tab strip, or page shell for diagnostics, reporting, or logs. Because the
    dashboard route is retired, that audit must also discover live top-level
    pages from `src/pages/` and may not keep a hard required-header entry for
    `frontend-modern/src/pages/Dashboard.tsx`.
26. Keep the authenticated app root aligned with that same first-session path.
    That same shared-primitive ownership now includes contextual row focus.
    `frontend-modern/src/components/shared/contextualFocus.ts` is the canonical
    owner for interactive-series filtering, focused-label lookup, active-series
    resolution, and nearest-scrollable-ancestor preservation across page-scoped
    summary surfaces. Dashboard row focus, infrastructure summary emphasis,
    storage summary emphasis, and workloads summary emphasis must all route through
    that helper instead of maintaining page-local copies of the same hover/focus
    rules.
    `frontend-modern/src/App.tsx` must land authenticated `/` and `/login`
    handoffs through this subsystem's provider-first platform landing contract:
    the first visible provider/runtime platform wins, and the Machines surface
    is eligible only when the current estate has standalone Pulse Agent
    machines or agentless availability endpoints and no provider/runtime
    evidence. The
    retired Infrastructure aggregate route and nested settings infrastructure
    aliases are not compatibility commitments:
    first-time operator setup must enter through the canonical Settings →
    Infrastructure workspace and its query-backed add flow, while provider
    evidence still owns the operational landing surface.
    `frontend-modern/src/components/Login.tsx` is part of that same
    pre-authenticated and first-session shell ownership: auth-check, setup
    fallback, and submit-pending loading indicators must compose
    `frontend-modern/src/components/shared/LoadingSpinner.tsx` through the
    shared-template registry instead of recreating page-local spinner shells.
    The subsystem registry must keep `Login.tsx` covered by the
    `first-session-runtime-and-preview` proof policy so login loading
    affordances cannot drift from the shared Settings, Patrol, AI, and
    primitive spinner contract.
    The authenticated app shell's boot-time route preloads must be owned by
    `frontend-modern/src/routing/routePreload.ts` so top-level cold-tab
    readiness cannot drift from the route-module preloader. Workloads,
    Recovery, Patrol, Alerts, Storage, and Settings are part of that shared
    preload contract.
    Route-module preloads and chart-cache fetches are separate shell
    responsibilities: the shared route preload inventory must stay module-only,
    while chart payload warming must route through the route or interaction that
    renders the chart. `frontend-modern/src/useAppRuntimeState.ts` must not
    prewarm retired Infrastructure summary-chart caches or eager Workloads
    chart caches as a generic authenticated-shell side effect.
27. Keep relay settings shell copy on the shared presentation owner in
    `frontend-modern/src/utils/relayPresentation.ts`. The route metadata in
    `settingsHeaderMeta.ts` and the leading `SettingsPanel` in
    `RelaySettingsPanel.tsx` must reuse the same description and availability
    copy instead of drifting into separate rollout or pairing wording. Relay
    availability copy must describe the Relay tier boundary as Relay and higher
    plans rather than collapsing Remote Access back into a Pro-only feature.
28. Keep shared settings-shell legal and docs referrals on
    `frontend-modern/src/utils/docsLinks.ts`. Shared settings surfaces such as
    `AIRuntimeControlsSection.tsx` must not hardcode GitHub `main` doc URLs for
    privacy, security, proxy-auth, scope-reference, or Terms-of-Service links.
29. Keep shared settings-shell telemetry transparency controls on the governed
    general settings panel. Preview/reset affordances for outbound usage telemetry
    must stay rendered inside
    `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
    instead of drifting into route-local modals, hidden dev tools, or shell
    chrome that operators would not naturally inspect.
30. Keep the short telemetry/privacy summary copy on that same shared surface
    accurate to the governed privacy doc. If the trust boundary depends on a
    specific retention window or on “IP addresses are not stored” rather than
    “IPs are never seen,” the summary copy in
    `GeneralSettingsPanel.tsx` must state those facts plainly instead of
    reverting to a stronger but inaccurate shorthand.
31. Keep maintainer commercial-event controls out of customer settings.
    The shared general settings privacy panel may expose outbound usage
    telemetry controls, preview, and reset affordances, but it must not render
    local commercial handoff event toggles, `PULSE_DISABLE_LOCAL_UPGRADE_METRICS`,
    or other commercial-debug controls as normal customer-facing preferences.
32. Keep shared storage-route feature presentation on neutral capability truth.
    Reusable mappers and presenters in `frontend-modern/src/features/storageBackups/`
    must distinguish inventory datastores from backup repositories so VMware
    rows on the shared storage route stay canonical to the admitted phase-1 floor instead of
    reviving backup-target, protected-target, or recovery-local semantics on a
    shared page. Those presenters must also source ZFS pool health from the
    canonical `details.zfsPool` payload (meta-first `storage.zfsPool`, flat
    `platformData.zfsPool` fallback) when building pool detail and bar
    summaries, rather than re-deriving device-level health from risk-reason
    strings or presenting flattened pool-state scalars as the full report.
    Ceph dedup is part of the same shared-presenter truth: cluster-internal
    pool rows must be consolidated into their mounting storage rows through
    `consolidateCephClusterPoolRecords` in
    `frontend-modern/src/features/storageBackups/cephRecordPresentation.ts`
    (lifting worse health onto the survivor) before shared storage tables
    render, instead of each table double-listing the same Ceph storage with
    conflicting raw-pool versus mounted-capacity accounting. Storage row
    models built by
    `frontend-modern/src/features/storageBackups/storagePoolRowPresentation.ts`
    must only carry fields the row actually renders; per-row source-platform
    badges and other identical-on-every-row decorations belong in the row
    expansion, not in `StoragePoolRowModel`.
33. Keep infrastructure settings-shell API alternatives on the shared shell
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
34. Keep the infrastructure settings connection inventory on one shared
    source. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
    composes rows exclusively from
    `frontend-modern/src/components/Settings/useConnectionsLedger.ts`,
    which polls `GET /api/connections`. Provider connection counts and
    availability must derive from that aggregator, not from a top-level
    ledger plus parallel provider-specific fetches. The retired
    `PlatformConnectionsWorkspace` / `TrueNASSettingsPanel` /
    `VMwareSettingsPanel` panels must not be reintroduced as a second
    fetch path.
35. Keep alert-history feature composition on the current owned state contract.
    `frontend-modern/src/features/alerts/tabs/HistoryTab.tsx` must react to the
    shared `alertData()` history state instead of reviving deleted aliases, and
    it must pass unified-resource resolution through to
    `frontend-modern/src/features/alerts/AlertResourceIncidentsPanel.tsx` so
    the panel can render shared route chips without creating another page-local
    resource lookup or provider-specific handoff layer.
36. Keep the alert-thresholds containers surface on the canonical shared owner.
    `alertOverridesModel.ts`, `useAlertOverridesState.ts`, and
    `useAlertsConfigurationState.ts` must surface API-backed `app-container`
    parents such as TrueNAS as first-class `Container Runtimes`, while
    `ThresholdsTab.tsx` must bridge function-valued selectors into
    `ThresholdsTable.tsx` explicitly instead of relying on spread-based adapter
    props that can collapse functions on the live Solid surface. Docker-only
    controls in `ThresholdsTableDockerTab.tsx` must remain gated to real
    `docker-host` resources instead of leaking onto platform-managed runtimes.
37. Keep shared commercial upgrade navigation typed and destination-aware.
    Shared paywall shells and upgrade actions must route internal billing or
    cloud destinations through `frontend-modern/src/utils/upgradeNavigation.ts`,
    `frontend-modern/src/components/shared/UpgradeLink.tsx`, and
    `frontend-modern/src/components/shared/useUpgradeNavigation.ts` instead of
    guessing from labels, hardcoding `target="_blank"`, or calling
    `window.open(...)` from each feature surface. Inline upgrade links may use
    `UpgradeLink`; button-styled upgrade CTAs must use `UpgradeButtonLink` so
    width, tone, focus, route/new-tab behavior, and opener preservation stay on
    the shared `ButtonLink` primitive instead of page-local Tailwind anchors or
    commercial helper class strings.
38. Keep same-shell platform/runtime route transitions on retained shared state.
    Active infrastructure consumers may show full-page loading only before the
    first compatible resource snapshot exists; once a fresh canonical snapshot
    is already present in the shared app shell, top-level platform/runtime tab
    switches must reuse that state boundary instead of flashing a transient
    page takeover between tabs.
39. Keep self-hosted paid-service prompts opt-in at the shared shell layer.
    `settingsNavCatalog.ts`, `settingsNavVisibility.ts`, shared upgrade link
    primitives, trial banners, monitored-system warning banners, history-lock
    overlays, and Patrol lock helpers must honor `presentationPolicy.hideUpgrade`
    by hiding paid prompts by default on ordinary self-hosted installs. Direct
    activation/recovery routes may render their owned content, but sidebar
    discovery, trial CTAs, plan upsells, monitored-system limit pressure,
    feature upgrade links, and plan-lock Patrol banners must require hosted
    mode, explicit handoff, or active entitlement. Cloud interest links from
    self-hosted plan surfaces must hand
    off to Pulse Account/public Cloud ownership rather than route to an
    in-product Cloud trial/signup page.
40. Keep the identified-service reducer on `discoveryPresentation.ts`. Any
    surface that wants to label a workload with the AI-identified service
    (drawer overview card, future row chips, MCP capability payloads) must
    consume `getDiscoveryIdentifiedSummary` rather than re-implement the
    empty/low-signal gate. The helper returns null when the stored record
    has no useful identification — mirroring the Discovery tab's
    `hasValidDiscovery` — so the same record either renders in all
    surfaces or hides in all surfaces, preventing "Unknown" rows or
    zero-confidence noise from drifting into peripheral UI.
    CLI access, confidence fields, and no-URL diagnostics are support
    metadata; they must not by themselves promote a record into the
    identified-service summary when the service name, category, version,
    paths, ports, facts, and suggested URL are all absent or placeholders,
    including generic workload types such as `service` or `container` and
    diagnostic facts such as metadata-only status, config-availability
    failures, or missing-config errors.
    Discovery is an opt-in observed-context layer, not an automatic row-link
    owner. The reducer must carry provenance, observed time, service version,
    endpoint candidates, and URL-source copy so drawer surfaces can show
    "Observed by Discovery" context and pass suggested URLs into the shared
    web-interface field. Persisted/manual web-interface metadata remains the
    only row-link source until the operator explicitly adopts a suggested URL.
    Discovery-sourced values rendered outside the Discovery tab must carry the
    shared compact provenance marker from
    `frontend-modern/src/components/shared/DiscoveryProvenanceMarker.tsx`, so
    operators can distinguish opt-in Discovery context from API-owned resource
    facts without reading a drawer-specific explanation.

## Current State

Patrol `fix_rejected` presentation is owned by the Patrol/AI finding surfaces
that render the governed-action loop, while the surrounding badge, button,
loading, and icon composition still uses shared frontend primitives. Shared
primitive code must not special-case rejected Patrol fixes outside the standard
metadata badge, button, and lucide-icon contracts.
Setup-only Patrol action chrome is also governed by the shared Patrol workspace
composition contract: provider-blocked states may show only the setup task and
direct `Open Provider & Models` action, must suppress the duplicate readiness
banner, and must not expose run-history buttons as a competing primary action
before Patrol can check infrastructure.
Patrol page setup banners must stay at operator level: render Patrol readiness
payloads as `Patrol setup issue` or `Patrol setup warning`, keep provider/model
context visible, and keep preflight/tool-call diagnostic wording inside
Provider & Models rather than the first-party Patrol header.
Pulse Intelligence settings now keep `Provider & Models` focused on provider
setup, default model selection, health, budget, usage, and provider checks; it
must not reintroduce the Patrol-control banner or `Open Patrol control` CTA
that belongs to the `Patrol` settings page and `/patrol` operator surface.
The Patrol settings page may still hydrate the cached Patrol diagnostic
snapshot, but the rendered model-check panel must summarize it as model
readiness rather than exposing preflight/tool-call implementation wording.
The canonical Provider & Models browser route is
`/settings/pulse-intelligence/provider`; `/settings/system-ai` remains a
routeable compatibility alias for old deep links, while new settings
navigation, OAuth callback redirects, Assistant repair actions, and Patrol
provider-repair CTAs must emit the Pulse Intelligence route.

Shared loading indicators are part of the active frontend primitive contract.
`LoadingSpinner` owns pure loading and action-pending spinner shells for shared
primitive internals such as `Button`, `PulseDataGrid`, and
`HistoryChartOverlay`, as well as Login, Settings, Patrol, and AI finding
surfaces; local `animate-spin` spinner shells in those consumers are governed
by the shared-template registry rather than page-local discretion.
Update progress status indicators are included in that loading boundary:
progress-stage loading must compose `LoadingSpinner` rather than local spinner
SVGs.
`DiscoveryLoadingFallback` owns the discovery-tab Suspense fallback row for
resource, workload, and Docker host drawers: centered row layout, status
semantics, discovery loading copy, and canonical `LoadingSpinner` composition
live there rather than in drawer-local fallback markup.
`FilterButtonGroup` owns feature table view toggles as well as settings
segmented selectors: page-specific labels and selected values stay in the
owning feature model, but the visible segmented selector shell must come from
the shared primitive rather than a local bordered button group.

AI settings provider fields are a governed frontend primitive, not a
provider-local form fork. The shared provider configuration section must render
provider-specific controls from `aiSettingsModel.ts` `extraFields`, including
Ollama `keep_alive` and the Z.ai custom base URL override, so Assistant and
Patrol keep one settings shape across
labeling, help affordances, helper copy, and persistence binding.
The shared AI model picker owns model route search and presentation for
Assistant surfaces. External open requests may seed an initial search query,
but filtering, current/default route badges, recent routes, and custom route
selection must remain inside `AIModelPicker`; callers should not duplicate that
logic in command handlers or feature-local model selectors. Optional provider
management actions belong in the same picker header as model refresh so
Assistant and settings surfaces can expose provider repair without forking the
model-list shell; callers own the destination, while the shared picker owns the
button placement, labeling, close behavior, and keyboard-safe dropdown state.

The Patrol alert-trigger severity selector under
`frontend-modern/src/features/patrol/` is built on the shared `FormSelect`
primitive (label-for/id wiring, `selectBaseClass` styling hook) rather than a
hand-rolled `<select>`, so its labeling and disabled-state affordances stay
consistent with the rest of the AI settings surface. That selector belongs in the
advanced Patrol settings disclosure, below the control policy that defines what
Patrol may do.

The advanced Patrol control toggles in the same surface bind each `Toggle`
primitive's accessible name through `ariaLabelledBy` pointing at the row's
heading span, so renaming a control's visible label (for example
"Alert-Triggered Analysis" to "Container Update Risk") updates the accessible
name automatically without a parallel `aria-label` string. New Patrol toggle
copy must keep using the shared `Toggle` `ariaLabelledBy` wiring rather than
hard-coding a divergent accessible name.

Kubernetes RBAC inventory (Roles, ClusterRoles, RoleBindings,
ClusterRoleBindings) is part of the existing Kubernetes platform-page
Configuration tab, not a new sidebar entry or top-level route, and the
reporting-resource-type mapping at
`frontend-modern/src/utils/reportingResourceTypes.ts` folds all four RBAC
kinds into the existing `k8s` transport token alongside ConfigMaps, Secrets,
and ServiceAccounts. Configuration tab rendering keeps RBAC summary fields
(rule count, role kind / role name, subject count, subject Kinds, aggregation
labels) bounded — individual subject names and full PolicyRule contents stay
outside the rendered surface, mirroring the agent and unified-resource
contracts.

Embedded Recovery workspace controls now use the shared filter-toolbar
primitive boundary. Platform pages may choose a default Recovery workspace,
such as TrueNAS opening on protection coverage, but the compact
protection/events selector must use `FilterSegmentedControl` and the
recovery-owned `useRecoverySurfaceState` owner rather than page-local tabs,
nested cards, or independent protection/event state in the embedding surface.

Cross-jump chip strips on alert and Patrol surfaces were retired on
2026-05-16 alongside the platform-first migration. The
`buildResolvedResourceSurfaceLinks` and `buildResourceSurfaceLinksForResource`
helpers (and the per-surface builders for Infrastructure / Workloads /
Storage / Recovery hrefs) were deleted from
`frontend-modern/src/routing/resourceLinks.ts`; the alert resource-incidents
panel and Patrol findings panel that consumed them now keep investigation
in-place through their existing handoff buttons and inline actions. Future
cross-surface drilldown chips must not reanimate the legacy helpers.

Command palette and keyboard shortcuts moved to platform-first on 2026-05-16,
and top-level aggregate workspace routes were retired on 2026-05-25
(`frontend-modern/src/components/shared/commandPaletteModel.ts`,
`frontend-modern/src/components/shared/useCommandPaletteState.ts`,
`frontend-modern/src/components/shared/KeyboardShortcutsModal.tsx`,
`frontend-modern/src/hooks/useKeyboardShortcuts.ts`,
`frontend-modern/src/routing/routePreload.ts`,
`frontend-modern/src/routing/navigation.ts`). The legacy
`nav-infrastructure` palette entry and `g i` chord remain retired with the
unregistered Infrastructure route. `nav-workloads`, `nav-storage`,
`nav-recovery`, and the `g w` / `g s` aggregate chords are also retired rather
than hidden as compatibility commands. Platform commands remain `nav-proxmox`,
`nav-docker`, `nav-kubernetes`, `nav-truenas`, `nav-vmware` (chords `g p` /
`g d` / `g k` / `g n` / `g v`) plus a dedicated `nav-kubernetes-workloads`
entry that lands on `/kubernetes/workloads`. The route-module preload registry
and `getActiveTabForPath` matcher must not recognize aggregate workspace URLs
as owned shell destinations. New palette commands and shortcut chords must
flow through the same shell owners; do not reintroduce hidden platform
families or retired top-level aggregate routes by reanimating legacy paths.
The shared route-state helpers follow the same boundary: workload, storage,
and recovery helpers in `frontend-modern/src/routing/resourceLinks.ts` may
build query strings for an already-owned platform/runtime route, but must not
export pathname builders for `/workloads`, `/storage`, or `/recovery`.

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
what Patrol actually owns: watching infrastructure, detecting issues,
recording findings, and escalating into governed investigation/action
only when the selected Patrol mode allows it.
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
The same header row may surface `Trigger status` when
`getPatrolTriggerStatusSummary` returns a runtime-relevant value from the
Patrol status payload. That text is page-owned operational metadata inside the
existing header row, not a new shared primitive, nested status card, or
secondary verdict band.

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
explicit `Usage data and privacy` model centered on `Outbound usage telemetry`;
maintainer commercial-event controls, upgrade-metrics labels, and
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
Platform stale-agent notices are also command-style upgrade affordances:
`frontend-modern/src/features/platformPage/PlatformOutdatedAgentNotice.tsx`
must stay hidden while the resolved presentation policy marks the session
read-only, including public demo mode, even though the same notice remains
available for ordinary customer installs that report outdated agents.
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
The same empty-state helper must consume Patrol trust-history evidence so a
historical regression reads as history review context, not as a current issue
and not as a healthy all-clear.
The same hierarchy also applies inside the Patrol summary shell: once the
primary assessment strip states Patrol's current risk and verification basis,
supporting metrics under that strip must stay metric-oriented and must not
repeat assessment or verification labels as a second compact verdict row.
The collapsed Patrol assessment strip itself must remain a compact readout
rather than a headline-plus-paragraph block; explanatory assessment and
recommendation copy belongs in the owning Findings, Runs, `Details`,
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
The same default-readout rule applies to collapsed Patrol issue rows:
`MetadataBadge` may carry severity, recurrence, and active decision/work states,
but the default Patrol page must not render raw lifecycle or investigation
process badges such as `detected`, `review finding`, loop state, status, outcome,
or confidence as row chrome. Those details belong in expansion, run history,
Assistant handoff context, or diagnostics.
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
TagBadges also owns Proxmox tag color fidelity. When a caller supplies a
source instance, the primitive must read that instance's `pveTagStyles` entry
before the legacy aggregate `pveTagColors` map, and it must honor the Proxmox
`caseSensitive` flag for both override lookup and deterministic fallback color
generation. Feature rows may pass instance identity into the primitive, but
they must not rebuild Proxmox color-map lookup locally.
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
Top-level route files are now expected to stay thin when a feature owns the
real product surface, but the former `/infrastructure` surface is not one of
those compatibility cases. It never shipped as a stable v6 route, so
`frontend-modern/src/App.tsx` must not register it and future feature surfaces
must extend Settings infrastructure, platform/runtime pages, or shared
Infrastructure components instead of recreating
`frontend-modern/src/features/infrastructure/` as a hidden page shell.
Infrastructure resource consumers may opt into websocket-first unified
resource hydration only when they also schedule canonical REST revalidation
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
Binary on/off controls are registry-backed too. Product surfaces must compose
`Toggle` or `TogglePrimitive` for shared track/knob styling, disabled behavior,
label/description wiring, and synthetic checked events instead of recreating
local `role="switch"` buttons with `aria-checked` and page-local classes.
Ordinary checkboxes, radio groups, and row-selection controls are separate
affordances and must not be forced through this toggle primitive.
The shared status badge now follows that same owner split.
`frontend-modern/src/components/shared/StatusBadge.tsx` stays the render shell,
`frontend-modern/src/components/shared/useStatusBadgeState.ts` owns disabled
gating and click runtime, and
`frontend-modern/src/components/shared/statusBadgeModel.ts` owns size padding,
label/title fallback policy, and status-badge class selection. Future status
badge work should extend those owners instead of pushing label/title policy or
disabled click handling back into the shell.
Read-only health/state badges are a separate shared primitive.
`frontend-modern/src/components/shared/StatusIndicatorBadge.tsx` owns
status-to-tone mapping, optional dot wiring, sizing, shape, and label
presentation for product surfaces that display state rather than toggle it.
Product components must compose `StatusIndicatorBadge` instead of calling
`getStatusIndicatorBadgeToneClasses` directly; low-level status utilities may
still expose the tone mapping for that primitive and utility-level tests.
Platform alert severity indicators are a governed specialization of that same
primitive family. `frontend-modern/src/components/shared/AlertSeverityBadge.tsx`
owns the alert severity badge and dot shells, while
`frontend-modern/src/utils/alertSeverityPresentation.ts` owns alert severity
label formatting, severity-bucket-to-status-indicator mapping, and
severity-bucket-to-detail-row tone mapping. Docker, Kubernetes, TrueNAS,
vSphere, and future platform alert tables must compose `AlertSeverityBadge`,
`AlertSeverityDot`, and `getAlertSeverityDetailTone`; they must not recreate
`severityVariant`, `severityTextClass`, `alertTone`, or severity badge spans
locally.
Platform alert severity filters follow the same shared-template rule.
`frontend-modern/src/features/platformPage/platformAlertSeverityFilterOptions.tsx`
owns the canonical All/Critical/Warning/Info option labels, tones, and leading
dots for platform alert table toolbars. Platform alert tables must call
`getPlatformAlertSeverityFilterOptions` instead of declaring local severity
filter arrays or calling `filterChipStatusDot` directly for those filters.
Platform alert detail formatting follows the same by-construction primitive
rule. `frontend-modern/src/utils/alertDetailPresentation.ts` owns the provider
code labels, provider-specific resource-type labels, vSphere alert entity
labels, started-at row labels, and full detail timestamp labels consumed by
Docker, Kubernetes, TrueNAS, and vSphere alert tables. Those tables must call
`formatPlatformAlertCode`, `formatPlatformAlertResourceType`,
`formatPlatformAlertEntityType`, `formatPlatformAlertStartedAt`, and
`formatPlatformAlertDetailDateTime` instead of declaring local formatter
helpers.
Read-only metadata badges follow the same primitive-owned shell rule.
`frontend-modern/src/components/shared/MetadataBadge.tsx` owns filled and
outlined appearances, compact sizing, shape, typed tone vocabulary, fit
behavior, and whitespace handling. Product surfaces such as Patrol findings
may own the labels and state-to-tone mapping in their presentation helpers, but
they must render visible metadata chips through `MetadataBadge` instead of
recreating local bordered xs spans.
Neutral and muted badge treatments must use semantic surface/text tokens rather
than hardcoded gray palettes; non-gray typed tones may retain their state color
vocabulary so success, warning, danger, info, and platform-adjacent metadata do
not collapse into visually identical chips.
Patrol run-history labels follow this state-badge boundary:
Patrol may derive the status label and typed variant in
`patrolRunPresentation.ts` or `patrolSummaryPresentation.ts`, but
`RunHistoryEntry.tsx` must render visible state badges through
`StatusIndicatorBadge` rather than `runStatus.badgeClass` or a local span.
The shared segmented selector now follows that same owner split.
`frontend-modern/src/components/shared/FilterButtonGroup.tsx` stays the render
shell, `frontend-modern/src/components/shared/useFilterButtonGroupState.ts`
owns variant resolution plus disabled selection/change runtime, and
`frontend-modern/src/components/shared/filterButtonGroupModel.ts` owns the
variant class catalog, compact-label policy, and segmented button class
selection. Future filter-button-group work should extend those owners instead
of pushing label truncation or segmented variant policy back into the shell.
Pressed/unpressed selector pills follow the same primitive rule.
`frontend-modern/src/components/shared/SelectablePillButton.tsx` owns the
pressed button shell and `aria-pressed` wiring, while
`frontend-modern/src/components/shared/selectablePillModel.ts` owns the active
and inactive pill class catalog. API token scope surfaces may own the security
scope labels and click handlers, but must not recreate rounded-full selector
pill class strings locally.
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
Tooltip shell chrome must follow semantic surface, text, and border tokens
rather than hardcoded dark palette utilities so light and dark themes share one
primitive-owned contrast contract.
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
Missing-suggested-URL diagnostics remain useful only when the operator has no
saved or entered URL; once a custom web-interface URL is present, the shared
field must suppress "no suggested URL" warnings so Discovery does not make a
valid manual endpoint look broken.
Saved web-interface launch affordances must also stay on a shared primitive
instead of page-local table columns or one-off external-link anchors.
`frontend-modern/src/components/shared/WebInterfaceNameLink.tsx` owns the
resource-name link shell, new-tab safety attributes, row-click/key propagation
containment, fallback text, and accessible launch labels. Workload guest rows,
grouped node headers, standalone machine rows, Proxmox node rows, and alert
resource rows/group headers compose that primitive so a saved or inferred URL
is opened by clicking the resource name on every comparable runtime table.
Runtime/platform tables must not add separate `Web` columns, page-local
external-link anchors, or duplicated new-tab safety handling for that launch
affordance.
Shared-template drift enforcement is registry-backed:
`frontend-modern/scripts/shared-template-registry.json` is the canonical list of
standardized repeated affordances, required consumers, and forbidden local
patterns, while `frontend-modern/scripts/shared-template-audit.mjs` enforces
that registry. Future repeated-affordance migrations must add or extend a
registry rule as part of the same change that extracts or adopts the shared
primitive.
Button-styled commercial upgrade CTAs are one of those registry-backed
templates. `frontend-modern/src/components/shared/UpgradeLink.tsx` owns
`UpgradeButtonLink`, and the registry requires gated settings, audit, agent
profiles, and self-hosted plan CTA surfaces to compose it rather than styling
`UpgradeLink` or anchors locally. Commercial presentation helpers may own the
label and destination intent; they must not own CTA button chrome.
Platform table frames are one of those registry-backed templates.
`frontend-modern/src/features/platformPage/sharedPlatformPage.tsx` owns
`PlatformTableShell`, including the canonical table card, header row, and body
divide styling. Platform table frames now have no local-frame exceptions in
`shared-template-registry.json`: new and existing platform tables must compose
`PlatformTableShell` instead of recreating the `TableCard`, header row, or body
divide frame locally. Platform table consumers must preserve the owner split:
frontend-primitives owns the repeated `PlatformTableShell` frame and guardrail
registry, while platform and unified-resource consumers own the source-specific
row fields, drawers, and resource semantics.
Platform table empty states follow the same registry-backed ownership.
`PlatformTableEmptyState` owns the repeated table-card empty-state shell for
Docker, Kubernetes, Proxmox, Standalone, TrueNAS, vSphere, and future platform
feature tables; source-specific consumers own only the empty-state icon, title,
description, and actions. Platform feature tables must not import
`EmptyState` directly or recreate a `Card`-wrapped empty-state shell.
Embedded settings and patrol panel empty states follow the same
shared-template registry, but compose `EmptyState` directly with
`variant="panel"` when they are not platform table/card empty states. The
primitive owns compact spacing, icon treatment, text hierarchy,
framed-versus-panel density, and action-slot layout; feature panels own only
the empty-state copy, icon choice, and callbacks. Migrated panels such as
Agent profiles, Audit Webhooks, Audit Log, Availability, Diagnostics, SSO
providers, and Patrol Run History must not recreate local `text-center` icon
stacks, dashed empty-state frames, or page-local empty-state action buttons.
Platform table loading states are registry-backed too.
`PlatformTableLoadingState` owns the repeated table-card compact
`role="status"` loading row for platform pages and tables; platform consumers
own only the title and description copy. Platform feature surfaces must not
recreate the `TableCard` plus compact status-row shell locally.
Platform table text-cell fallback formatting is a shared primitive as well.
`formatPlatformTableTextValue` in
`frontend-modern/src/features/platformPage/sharedPlatformPage.tsx` owns the
trimmed-string plus canonical empty-cell marker behavior. Kubernetes platform
tables must compose that helper instead of declaring local `textValue` helpers
or inlining `asTrimmedString(...) || '—'` fallback expressions for table text
cells.
Platform table title-case fallback formatting follows the same rule.
`formatPlatformTableTitleCaseValue` owns the repeated trimmed-string plus
`Unknown` fallback behavior for state/status labels that need simple title
case. TrueNAS platform tables must compose that helper instead of declaring
local `titleCase` helpers.
Platform table compact list summaries follow the same rule.
`summarizePlatformTableValues` owns the repeated trimming, empty-marker label,
visible-value count, `+N` overflow suffix, full-title text, and normalized
value-list behavior for dense platform table cells. Kubernetes
service/network/config/policy/autoscaling tables, vSphere datastore/network
tables, and TrueNAS network-share tables must compose that helper instead of
declaring local `compactList` or `summarizeValues` helpers.
Platform table uptime formatting follows that rule too.
`formatPlatformTableUptimeValue` owns the repeated compact/full uptime label
selection plus canonical empty-cell marker behavior for dense platform table
cells. Docker / Podman hosts, Kubernetes nodes, Proxmox nodes and backup
server rows, Proxmox Mail Gateway instance and drawer node rows, Standalone
machines, TrueNAS systems, and vSphere ESXi hosts must compose that helper
instead of declaring local `formatUptime` helpers or importing the generic
formatter in table files for the same days/hours/minutes fallback.
Platform table byte-size formatting follows the same rule.
`formatPlatformTableBytesValue` owns the repeated positive-byte formatting plus
canonical empty-cell marker behavior for dense platform table cells. Docker /
Podman native storage cells, Docker / Podman engine storage-usage cells,
Kubernetes node and storage capacity cells, Proxmox backup server, Ceph,
coverage, and recoverable-artifact size cells, and TrueNAS system, VM,
storage-topology, and protection byte cells must compose that helper instead
of declaring local `formatBytes` wrappers, importing the generic formatter in
table files, or reimplementing byte-unit precision there.
Compact platform table timestamps follow the same rule.
`PlatformTableDateTimeValue` and `formatPlatformTableDateTimeValue` own compact
date-time parsing, invalid/empty markers, optional minimum-year filtering, Intl
format options, and tabular-number styling for dense timestamp cells. TrueNAS
protection completed-time cells and vSphere activity "When" cells must compose
that primitive instead of declaring local compact `toLocaleString` helpers, and
timestamp columns use the canonical `numeric-value` alignment kind because they
are scannable scalar values.
Relative timestamp-age cells follow the same rule.
`PlatformTableRelativeTimeValue` and `formatPlatformTableRelativeTimeValue` own
the repeated `formatRelativeTime` composition, compact-label default,
invalid/empty markers, and tabular-number styling for dense platform table
cells. Docker / Podman volume created-at cells, Kubernetes deployment age
cells, Kubernetes event observed-time cells, Proxmox backup created-age cells,
Proxmox replication last-sync cells, Standalone machine last-seen cells, and
Standalone availability-check checked-at cells must compose that primitive
instead of importing `formatRelativeTime` directly or declaring local
timestamp-age helpers in table files.
Duration and interval cells follow the same rule.
`PlatformTableDurationValue` and `formatPlatformTableDurationValue` own
seconds/minutes/hours duration labels, explicit fallback text, canonical
empty-cell markers, and tabular-number styling for dense platform table cells.
Proxmox replication last-duration cells and Standalone availability-check poll
interval cells must compose that primitive instead of declaring local
seconds/minutes helpers.
Responsive platform table width normalization is registry-backed too.
`getPlatformTableWeightedColumnWidthStyle` owns visible-column weight
normalization, zero-width fallback, and stable four-decimal percentage
formatting for dense platform table models. Docker / Podman container columns
and Proxmox host columns must keep their domain column IDs, layout breakpoints,
and weight maps local, but must call that shared helper instead of declaring
local `formatPercentage` / `toFixed(4)` width helpers.
Platform table numeric fallback rendering is registry-backed too.
`PlatformTableNumberValue` owns finite-number checking, tabular-number styling,
custom empty-marker support, and caller-owned number formatting for dense
optional numeric table cells. Docker / Podman native count helpers, Kubernetes
optional count cells, Docker Swarm service desired/running counts, Kubernetes
Deployment replica counts, Proxmox Mail Gateway count columns, and TrueNAS
system share/service and storage-topology disk count cells must compose that
primitive instead of declaring local `numberValue`, `numericValue`,
`replicaCount`, `countCell`, `diskCountLabel`, or cell-level `tabular-nums`
variants. If a scheduler, service-domain, or inventory count is intentionally
zero-defaulted, the consuming table owns that field/default choice and still
renders through `PlatformTableNumberValue`.
Locale-formatted integer count labels share the same primitive boundary.
`formatPlatformTableIntegerValue` owns rounded integer formatting, locale
grouping, and empty-marker behavior for dense platform table and drawer count
cells. Kubernetes namespace drawers, Proxmox Backup Server backup counts,
Ceph pool object counts, and Proxmox Mail Gateway table/drawer counts must
compose that helper, usually through `PlatformTableNumberValue`, instead of
declaring local `formatInteger`, `formatLocaleCount`, `formatNumber`, or direct
`toLocaleString()` count formatting.
`PlatformTableCountRatioValue` owns the companion healthy/total or ready/total
count-ratio skeleton: numerator, slash, muted denominator, tabular styling, and
empty marker behavior. `formatPlatformTableCountRatioValue` owns the same
zero-default and suffix behavior for string-only table summaries and titles.
Kubernetes cluster child counts compose the component instead of keeping a
table-local `childCountCell` renderer, and Kubernetes networking endpoint
summaries compose the formatter instead of hand-building `3/3 ready` strings;
the consuming table owns only which current/total values, suffix, and warning
tone apply.
One-decimal percent and positive Celsius cells are also shared platform-table
value primitives. `PlatformTablePercentValue` owns percent formatting,
tabular-number styling, and empty markers; `formatPlatformTablePercentValue`
owns the same one-decimal percent string for overlay labels, titles, and
sparkline labels, including caller-selected ratio normalization and clamping.
`PlatformTableTemperatureValue` owns finite positive Celsius validation,
one-decimal `°C` formatting, tabular-number styling, and empty markers. Docker
/ Podman host, Proxmox backup/Ceph/node/mail-gateway, and TrueNAS
system/storage-topology tables or drawers must compose those primitives instead
of carrying local `formatPercent`, `formatPercentLabel`, `toFixed(1)%`, or
temperature label helpers.
Platform table metric fallback rendering is also shared.
`PlatformTableMetricFallback` owns the centered muted empty marker used in
metric bar cells plus optional caller-owned fallback label/title text, and
`getPlatformTableFiniteMetric` owns finite-number normalization for CPU and
memory, disk, and capacity values. Docker / Podman, Kubernetes, Proxmox,
Standalone, TrueNAS, and vSphere platform tables and their table-model helpers
must compose those helpers instead of declaring local `metricFallback` /
`finiteMetric` helpers or inlining centered muted dash fallback markup in
metric cells.
Platform load-failure states are registry-backed as well.
`PlatformErrorState` owns the repeated table-card error shell, warning icon,
and Refresh action for platform page and table load failures; platform
consumers own only the failure title, description, and refresh callback.
Platform feature surfaces must not recreate an `EmptyState` plus local Refresh
button for `Could not load...` states.
Platform section tabs are registry-backed too. `PlatformSectionTabs` owns the
workflow tab shell, hidden-single-tab behavior, active-link styling, link
targeting, and active-page aria state for platform pages; platform page
surfaces own only tab specs, the active tab choice, and aria-label copy.
Platform feature surfaces must not rebuild local nav tab bars with
`aria-current` and border-tab styling.
Filter bars are registry-backed too. `FilterBar` owns resource-list filtering
as a catalog of `FilterDef` entries, while `filterChipStatusDot` owns the
small leading status-dot glyph used by filter options. Page and feature
surfaces must not copy the chip-dot `<span>` factory or import the legacy
`PageControls` deck for resource-list filtering; those drift checks live in
`shared-template-registry.json` and run through `shared-template-audit.mjs`.
Status indicator dots are registry-backed too. `StatusDot` owns the shared
size, color-token, pulse, title, aria, and decorative-status behavior for
resource and health dots, while feature owners supply only the status
semantics. Storage linked-disk health rows must derive a `StatusDot` variant
through `getLinkedDiskHealthDotVariant` and must not recreate local rounded
green/yellow span classes in storage components or storage-backup presentation
helpers.
Loading indicators are registry-backed too. `LoadingSpinner` owns the shared
border-based spinner shell, size catalog, tone catalog, decorative status, and
accessible status label behavior. Shared primitive internals such as `Button`,
`PulseDataGrid`, and `HistoryChartOverlay`, plus Login, Settings, Patrol, and
AI finding surfaces, must compose that primitive for pure loading and
action-pending spinners; icon-specific refresh rotation remains local icon
state, not a loading-spinner shell.
Native select controls are registry-backed too. `FormSelect` owns label/id
wiring, helper-text description merging, value synchronization, default select
chrome, dynamic-option value synchronization, and compact styling hooks for
native selects. Product components and shared filter/menu internals must
compose `FormSelect` rather than recreating screen-reader labels, native
`<select>` shells, value-reapply effects, or compact select chrome locally; the
only raw native select in frontend runtime code should live inside that
primitive.
Native textarea controls follow the same contract. `FormTextarea` owns
label/id wiring, helper-text description merging, value synchronization, default
textarea chrome, and compact styling hooks for multi-line text fields. Alert
destination fields, incident notes, infrastructure merge reports, commercial
recovery input, and agent-profile prompt/description fields must compose
`FormTextarea`; alert, settings, and infrastructure runtime code must not
recreate raw native `<textarea>` shells outside that primitive.
Search controls are registry-backed too. `SearchField` owns simple search
input chrome, clear affordance, keyboard forwarding, focus handling, aria
labels, and trailing-control padding, while `SearchInput` owns resource-list
search enhancements such as history, tips, and type-to-search wiring. Product
surfaces must compose those primitives instead of rendering native
`type="search"` inputs or recreating search icon/clear/input classes locally.
Settings resource selectors must use `SearchField` for both primary text
search and secondary tag/resource filters instead of restoring native
`type="text"` filter inputs beside a shared search field.
Segmented selectors are registry-backed too. `FilterButtonGroup` owns the
settings, prominent, compact, and equal segmented selector shells, including
active-button tone, disabled-option behavior, pressed-state semantics,
compact labels, and horizontal scroll treatment through the shared
shell/state/model split. Settings and compact feature surfaces must compose
that primitive instead of copying active-button selector styling locally.
Selectable pill buttons are registry-backed too. `SelectablePillButton` owns
rounded pressed/unpressed selector pills, including active tone, disabled
treatment, focus ring, and `aria-pressed`; settings and security surfaces must
compose that primitive instead of copying rounded-full active selector styling.
`ResourcePicker` report-domain filters are part of that boundary: the picker
owns the reportable resource categories and labels, but the type selector shell
must come from `FilterButtonGroup`.
Chart visibility display actions are registry-backed too.
`ChartVisibilityToggleButton` owns the `Show charts` / `Hide charts` label,
pressed-state, title, icon, and toolbar action styling for summary-bearing
filter surfaces. Pages must compose that primitive instead of recreating local
chart visibility buttons or one-option segmented controls.
Column visibility controls are registry-backed too. `ColumnPicker` owns the
column chooser trigger, panel title, reset action, empty-state copy, hidden
count badge, dropdown width, and outside-click lifecycle through the shared
shell/state/model split. Table surfaces must compose that primitive instead of
recreating local column chooser buttons or panels.
Selection-card groups are registry-backed too. `SelectionCardGroup` owns the
compact/detail card grid, active-card tone, disabled selection behavior,
pressed-state semantics, title/description styling, and icon container
treatment through the shared shell/state/model split. Settings and feature
surfaces must compose that primitive instead of copying compact/detail
border-card styling locally.
Grouped/list table-mode controls are registry-backed as well.
`GroupedTableModeSegmentedControl` owns the shared `Group by` label,
`Grouped` / `List` labels, tooltip titles, and icons for table mode switching.
Resource surfaces must compose that primitive instead of copying grouped/list
segmented-control labels locally.
Product table cards are registry-backed too. `TableCard` owns the shared
bordered, no-padding, card-tone table frame, while `TableCardHeader` owns the
title/action/clear chrome, clear button copy, aria label, and propagation
containment for table-card headers. Product table surfaces must compose those
primitives instead of recreating local overflow-hidden bordered wrappers or
retired summary-table header aliases.
Inline detail table rows are also registry-backed. `InlineDetailTableRow`
owns the row/cell/content shell and row-click containment for platform,
workload, and infrastructure inline drawers; callers may pass row-specific
`data-*` attributes, colspan, and content classes, but they must not recreate
the surface-alt detail row shell locally.
Inline detail section content is registry-backed separately from the row shell.
`DetailSectionTable`, `InlineDetailPanel`, and `detailSectionModel.ts` own
detail row compaction, section-table rendering, value-tone classes, and the
inline close action for platform alert/activity/protection/service detail
panels; consumers may own the platform-specific section data, but they must not
recreate local `DetailField` grids or route platform-neutral detail tables
through a provider-named primitive.
Platform row-detail disclosure controls are also registry-backed templates.
`frontend-modern/src/features/platformPage/PlatformResourceDetailTableRow.tsx`
owns `PlatformResourceDetailToggleButton`, which composes
`SummaryRowActionButton` for the canonical row-detail affordance, accessible
label, `aria-expanded`, `aria-controls`, and propagation containment. Platform
tables that use `createPlatformResourceDetailState` or render local inline
detail rows must compose that toggle; they may still own the detail row content,
drawer payload, post-success refresh callbacks, and platform-specific fields.
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
support/admin controls moved under Settings, that utility ordering must no longer
reserve a standalone `operations` slot; alerts, Patrol, and Settings are the
remaining authenticated utility tabs.
The shared command palette now follows that same owner split.
`frontend-modern/src/components/shared/CommandPaletteModal.tsx` stays the
render shell, `frontend-modern/src/components/shared/useCommandPaletteState.ts`
owns query state, selected-row keyboard state, open-reset/focus lifecycle,
route-path wiring, and command selection, and
`frontend-modern/src/components/shared/commandPaletteModel.ts`
owns canonical command construction plus query normalization and filtering
policy. Future command-palette work should extend those owners instead of
pushing route construction or search policy back into the shared shell.
The OpenCode reference for this interaction is
`packages/opencode/src/cli/cmd/run/footer.command.tsx` at `origin/dev`
`e82542b8023a8374f29c23b70ec019c8f256354e`, where `RunCommandMenuBody`
builds and filters command rows, holds selected menu state via
`createFooterMenuState`, resets selection as the query changes, and routes
keyboard movement and selection through `handleKey`. Pulse adapts that contract
by keeping command construction in the shared palette model and command
execution in the shared palette state / Assistant store instead of moving
editor-specific command routing into the render shell.
Assistant command-palette actions are shell requests, not duplicated chat
logic. New session, session picker, model picker, Undo, and Redo commands must
flow through the shared `frontend-modern/src/stores/aiChat.ts` command request
contract so the drawer owns disabled/loading state, prompt restoration, and
notifications while the palette remains only a searchable command surface.
The command-palette Assistant open command is also contextual shell routing:
`frontend-modern/src/components/shared/useCommandPaletteState.ts` must derive
current-view context through `frontend-modern/src/utils/assistantPageContext.ts`
and pass that context into `aiChatStore.open(...)`, while
`frontend-modern/src/components/shared/commandPaletteModel.ts` owns the
corresponding `Ask about <view>` label. It must not fall back to an empty
generic Assistant open action.
The shared search field now follows that same owner split.
`frontend-modern/src/components/shared/SearchField.tsx` stays the render shell,
`frontend-modern/src/components/shared/useSearchFieldState.ts` owns focused-
Escape clear/blur behavior and input-ref lifecycle, and
`frontend-modern/src/components/shared/searchFieldModel.ts` owns clear/shortcut
visibility rules plus trailing-control padding policy. Future search-field work
should extend those owners instead of pushing event behavior or layout policy
back into the shared shell. Forwarded keyboard and blur events must preserve
native browser event getters and methods while normalizing `currentTarget` and
`target`; shared search-field wrappers must not proxy native event properties or
methods through a receiver that can break `KeyboardEvent`/`FocusEvent` getters,
`preventDefault()`, or `stopPropagation()` in live browser surfaces.
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
Legacy `PageControls` and labelled select/toggle primitives are not the
resource-list filter shape. If a future surface needs a new filtering
affordance, it should extend the FilterBar catalog model or add a new
registry-backed shared primitive rather than reintroducing a per-page select
row.

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
Audit-log fetch failures must preserve the structured backend error object
through `apiErrorFromResponse` and render customer-facing copy from
`frontend-modern/src/utils/auditLogPresentation.ts`; the settings shell may
own refresh and pagination state, but it must not show raw `Internal Server
Error` strings or unbounded page sizes as local hook behavior.
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

The retired `/operations` route is unregistered rather than a compatibility
redirect. Diagnostics, reports, and logs belong to the shared Settings shell
instead of a bespoke page-local tab surface. Support-only navigation must
therefore route through the shared settings owners rather than rebuilding a
second route-level shell, and public demo posture must keep those support
entries hidden from the Settings navigation instead of reviving a standalone
operations page.
that are unavailable in demo mode.

The dashboard overview route and its feature-owned summary surfaces are
retired. Authenticated root entry now lands on the first visible
provider/runtime platform, so first-viewport estate orientation belongs to that
platform page plus the Add infrastructure flow rather than a separate dashboard
or legacy Infrastructure shell. Future overview or brief-style surfaces must
be governed as new product surfaces before they add route-level data
orchestration, section anchors, or Assistant prompt handoffs; they must not
restore `frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/features/dashboardOverview/`, or deleted dashboard-only
presentation helpers as compatibility paths.
The primary navigation active-tab contract follows that retirement boundary:
retired or unknown routes such as `/dashboard` must not be coerced into the
nearest platform tab just because the authenticated shell has a provider-first
landing fallback. Shared desktop and mobile navigation must tolerate a missing
active tab for those paths while still highlighting canonical active routes
such as Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, Alerts,
Patrol, and Settings.
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
update-surface wording inline. `CopyCommandBlock` must use the shared
`copyToClipboard` helper so install/update/agent snippets keep the same
Clipboard API fallback path and only report copied state after the shared copy
path succeeds.

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
of that same reporting boundary. Native Kubernetes inventory-only resource
types, including ReplicaSets, EndpointSlices, NetworkPolicies, StorageClasses,
ConfigMaps, Secrets, ServiceAccounts, Roles, ClusterRoles, RoleBindings,
ClusterRoleBindings, ResourceQuotas, LimitRanges, PodDisruptionBudgets, and
HorizontalPodAutoscalers, must map to the existing reporting `k8s` transport
token at this edge rather than widening the metric-report picker into
platform-object inventory. Kubernetes RBAC inventory (Roles, ClusterRoles,
RoleBindings, ClusterRoleBindings) joins the existing K8s inventory bucket on
that transport without introducing a separate reporting token: from the
reporting transport's point of view it is platform-object inventory the same
way ConfigMaps and ServiceAccounts already are, even though the rendered
Configuration tab surfaces it through RBAC-specific lifecycle/data-shape
columns inside `KubernetesConfigTable`. The shell must not
re-accumulate license
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

Settings informational, warning, success, and danger callouts with icon-plus-copy
layouts must now route through the shared `CalloutCard` primitive instead of
maintaining feature-local colored bordered wrappers. The primitive owns the tone
palette and the `scale="compact"` density used by smaller settings notices, and
the `settings-callout-card-shell` shared-template registry rule requires current
settings consumers to compose it instead of reintroducing local panel shells.
Connection-editor status, feature-disabled, delete-error, and probe-result
notices are part of that same settings callout boundary: the editor and
credential slots own the source-specific lifecycle or API meaning, while
`CalloutCard` owns the warning/success/danger shell and compact density.
The `settings-connection-editor-local-*-callout-shell` pattern guards block
future connection-editor files from reintroducing amber, red, or rose local
notice shells.
Shared error-boundary fallbacks use the same boundary: the fallback owns error
copy and reset/reload handlers, while `CalloutCard` owns danger tone, spacing,
dark-mode styling, and alert layout instead of inline red panels or raw SVG
alert glyphs.
Update confirmation and progress modals use the same shared boundary: the
modal flow owns update state and copy, while `CalloutCard`, `Button`,
`ActionIconButton`, `LoadingSpinner`, and lucide icons own the colored notice,
command, icon-only close, and status indicator chrome.

Settings loading placeholders must route through the shared
`SettingsLoadingSkeleton` primitive instead of local `animate-pulse` blocks.
Feature panels may choose the loading shape and row counts, but the shared
primitive owns pulse animation, skeleton fill tokens, metric-card grids,
progress-card rows, table header/body shells, and labelled status semantics.
The `settings-loading-skeleton-shell` registry rule covers the current
organization, security overview, and resource data policy loading surfaces, the
`settings-loading-state-shared-skeleton-required` guard requires future
Settings `*LoadingState` files to compose that primitive, and the
`settings-local-loading-skeleton-block-shell` guard blocks local pulse skeleton
blocks from returning inside Settings components.

Settings external documentation text links must route through
`ExternalTextLink`, while button-styled external actions route through
`ButtonLink`/`UpgradeButtonLink`. Shared primitives own new-tab safety, rel
policy, link tone, compact action density, and focus styling; settings panels
must not hand-code raw `<a target="_blank">` anchors for documentation links.
The `settings-external-text-link-shell` and
`settings-external-text-link-local-anchor` registry entries enforce that split,
and the Button registry owns the `info` variant for blue documentation CTAs.

Platform inline notices that sit inside platform pages but are not settings
callouts must route through the shared `InlineNotice` primitive. Platform owners
provide only the affected-resource copy, render predicate, and destination; the
shared primitive owns the dense warning/info/danger/success tone palette,
icon/content layout, and action-link chrome. The
`platform-inline-notice-shell` registry rule covers current outdated-agent and
outdated-sensor notices, and the
`platform-inline-notice-local-amber-shell` pattern guard blocks future
`platformPage` files from reintroducing page-local amber notice shells.

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
settings tab identity, canonical route derivation, route eligibility, and
retired infrastructure/workloads alias rejection. `settingsRouting.ts` and
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
Infrastructure settings no longer has route-level platform-selection state:
`/settings/infrastructure` and `/settings/infrastructure?add=<step>` are the
only routeable Infrastructure settings entry points for platform/API and
agent-backed source setup. Agentless ping/TCP/HTTP checks are monitoring
availability settings at `/settings/monitoring/availability`, with
`/settings/monitoring/availability?add=target` as the route-owned add dialog.
Machine entry points must add `targetKind=machine`, while the focused
availability checks entry points for services and devices must add
`targetKind=service`, so deep links open the same owned dialog with the correct
bounded target kind already selected. That target kind only scopes the
availability form copy and payload; it must not make agentless reachability
targets eligible for the Machines table.
Former nested aliases such as `/settings/infrastructure/install`,
`/settings/infrastructure/platforms/proxmox/pbs`,
`/settings/infrastructure/api/pve`, and `/settings/workloads/docker` must fail
route eligibility instead of being normalized back into the Infrastructure
workspace.
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
model must reject old `/settings/operations/*` settings paths instead of
normalizing them into `/settings/support/*`, and the top-level `/operations/*`
browser path must stay unregistered. The catalog plus visibility owners must
still treat support surfaces as Settings-native pages rather than as a second
top-level utility destination.

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
operator refresh controls generation-aware, timeout-bounded, and separate from
background polling state, so a slow supporting intelligence read cannot make the
shared Patrol header Refresh Patrol action spin indefinitely or stay disabled
while Patrol findings and status remain visible.
That same Patrol shell should make scoped trigger policy legible without
another navigation step. `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
should keep actionable scoped-trigger state legible without promoting
background-only policy pauses as default operator guidance; run-trigger details
belong in run history or explicit secondary context.
That same Patrol-facing primitive vocabulary must stay product-first. Patrol
summary actions, runtime banners, run-history runtime-failure actions,
runtime-finding actions, circuit-breaker copy, and Patrol control/provider controls
may point at the shared provider settings route or model catalog, but they
should describe those controls as Patrol/provider surfaces through
`frontend-modern/src/utils/patrolRuntimeActions.ts` rather than falling back to
generic `AI Settings`, `AI Model`, or `AI circuit breaker` copy inside the
Patrol shell itself.
That same product-first naming rule also applies to Pulse Intelligence settings:
`frontend-modern/src/components/Settings/AISettings.tsx`,
`frontend-modern/src/components/Settings/settingsHeaderMeta.ts`,
`frontend-modern/src/components/Settings/settingsNavCatalog.ts`,
`frontend-modern/src/components/Settings/useAISettingsState.ts`, and
`frontend-modern/src/utils/aiSettingsPresentation.ts` must present the provider
surface to operators under the `Pulse Intelligence` settings group as
`Provider & Models` provider/model configuration rather than as a generic
`AI Services` shell. The canonical browser route for that surface is
`/settings/pulse-intelligence/provider`; legacy `/settings/system-ai` links may
remain routeable compatibility aliases, but new navigation, setup, Assistant,
and Patrol repair CTAs must emit the Pulse Intelligence route.
On the main Patrol page, though, governed activity context belongs inside
`frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx` as current
work, selected run history, or explicit details. Do not reintroduce a
parallel page-level status strip above the current-work workspace.
That same composition rule applies to the workspace: the default path should
move directly into findings and run history instead of repeating runtime
context through a second pre-tab status strip.
`Details` follows that same composition rule. Recent changes,
learned correlations, and policy coverage belong behind an explicitly secondary
supporting-context affordance that only appears when Patrol has active findings
or a selected run that needs explanation; healthy fully verified Patrol states
and degraded summary health by themselves must not advertise that supporting
evidence as a peer workflow. The default workspace may show the compact
`Details` control, but the full panel must render only after the operator
opens it. When that disclosure expands, the workspace must explicitly label the
selected finding or run as Patrol's record and frame the supporting cards as
explanatory context rather than as a fresh Patrol result or raw evidence
console.
Selected-run history should also suppress generic findings filter chrome and
read as a Patrol run record. Missing legacy `finding_ids` remains an internal
fail-closed scoping condition, but the visible caveat should say the finding
record is unavailable rather than exposing snapshot/filter vocabulary.
Workspace section descriptions must use Patrol-owned mode presentation copy
that reflects the selected mode and lock state; they must not hardcode
an all-mode sentence that tells watch-only users Patrol can investigate, ask for
approval, or fix issues. The Patrol workspace must not add a generic Details
panel to explain learned correlations, recent changes, or policy buckets; those
signals may support Assistant and backend reasoning without becoming default
operator chrome. Setup-only Patrol runtime failures must use the Patrol-owned
`Fix Patrol setup` workspace title and setup description plus a dedicated setup
task with the direct provider-settings action, rather than presenting that state
as a normal `Current issues` infrastructure queue, and they must suppress
generic issue-row chips, expand chevrons, and `Active` / `All` / `Resolved`
filter chrome in that setup-only state. That setup-only workspace must also be
the only visible provider-repair CTA for that state: it should use the
`Open Provider & Models` action and suppress the readiness banner so setup does
not appear twice. Run history must stay hidden as a competing action until
Patrol can check infrastructure or an operator is already reviewing a specific
run record.

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
`frontend-modern/src/components/Alerts/ThresholdsTableAgentsTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableDockerTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableKubernetesTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableTrueNASTab.tsx`,
`frontend-modern/src/components/Alerts/ThresholdsTableVMwareTab.tsx`, and
`frontend-modern/src/components/Alerts/ThresholdsTablePBSTab.tsx`.
`frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts`
owns the platform-shaped thresholds sub-route contract:
`/alerts/thresholds/proxmox`, `/alerts/thresholds/docker`,
`/alerts/thresholds/kubernetes`, `/alerts/thresholds/truenas`,
`/alerts/thresholds/vmware`, `/alerts/thresholds/pbs`,
`/alerts/thresholds/pmg`, and `/alerts/thresholds/systems`. Legacy neutral
links such as `/alerts/thresholds/infrastructure`,
`/alerts/thresholds/containers`, and `/alerts/thresholds/mail-gateway` must
redirect to the matching platform-shaped route; legacy
`/alerts/thresholds/agents` links must continue to resolve to Systems.
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
save/test actions route through `AISettingsStatusAndActions.tsx`.
`AISettingsStatusAndActions.tsx` may expose provider connection status and
test actions only for the Provider & Models page; section pages reuse the save
bar without making Patrol, Assistant, or Discovery look like provider setup
screens. Future AI settings work must extend those section owners instead of
re-inlining large runtime subsections into the shell.
Provider-specific settings fields inside
`AIProviderConfigurationSection.tsx` must remain model-driven through
`aiSettingsModel.ts` `extraFields`, including Ollama `keep_alive`, so the
shared provider panel owns framing, labels, help affordances, helper copy, and
form binding instead of adding provider-local bespoke controls.
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
shell reuse `shellTitle` plus `shellDescription`, so the settings IA and page
shell stay aligned on `Plans & Billing` without reintroducing local label drift.
That same settings-shell framing boundary also covers adjacent top-level
settings references to the self-hosted commercial surface. When
`InfrastructureWorkspace.tsx` or other settings-shell surfaces point operators
toward Plans & Billing for billing, license status, Patrol mode, or paid feature access, they
must reuse the shared referral copy from
`SELF_HOSTED_PRO_BILLING_PRESENTATION` rather than drafting local “go there
for billing” variants.
That same shared presentation owner now also carries the entitlement-first
commercial summary contract for self-hosted settings. The top-level navigation
entry stays product-IA owned through `navLabel` (`Plans & Billing`), while the
page header and shell title stay owned through `shellTitle`
(`Plans & Billing`), and the billing shell must foreground the active plan
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
No-URL diagnostics and command access hints are not endpoint candidates; they
can explain a Discovery result inside Discovery-owned surfaces, but they must
not trigger out-of-tab identified-service cards or suggested-URL panels without
another meaningful service signal.
The visible provenance marker for those values is the shared
`DiscoveryProvenanceMarker`; local surfaces may choose the labelled or
icon-only variant, but must not invent alternate Discovery badges or hide the
source on compact cards.
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
That same shared-shell framing also covers the concise telemetry summary in
General settings. The shell may present the privacy contract in compact product
copy, but the vocabulary for outbound usage telemetry must stay aligned
with `security-privacy`: aggregate self-hosted adoption counts, coarse feature
flags, and coarse Patrol, Assistant, and external-agent usage counters are
allowed, while hostnames, credentials, infrastructure identifiers, prompts,
chat messages, command text, action output, token values, and personal
information are not.
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
summary vocabulary for connection health and contribution counts. While VMware
remains admitted rather than supported, shared settings primitives must render
its source-picker card with the manifest-derived preview badge and keep
supported-source empty-state copy from listing VMware as available now.
That same shared filter-presentation boundary also owns infrastructure source
continuity on active surfaces. Settings infrastructure and platform/runtime
pages must keep known canonical source options such as `truenas` and
`availability` visible when configuration or route context establishes them,
even when current unified-resource results do not contain that source, so
platform handoffs from settings and other surfaces do not flash back to
generic host-only language while the operator is still in a provider- or
endpoint-scoped investigation flow.
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
The global Assistant launcher and the command-palette Assistant open command
follow the same contextual rule. They must derive current-view context through
`frontend-modern/src/utils/assistantPageContext.ts`, label the action as asking
about the current monitoring, Patrol, alerts, or settings view, and open the
drawer with that `pulse-view` context attached. They must not call
`aiChatStore.toggle()` or `aiChatStore.open()` without context from the
authenticated shell, because that reintroduces a generic Assistant front door.
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
and `settingsNavigationModel.ts`. No future additions to the settings nav may restore
`infrastructure-connections` or `infrastructure-install` as independent tab
identifiers; panel routing within the infrastructure area must use
`InfrastructurePanelStep` in-page state instead of URL sub-routes.
`frontend-modern/src/components/Settings/settingsNavigationModel.ts` owns the
explicit routeability check that rejects retired infrastructure/workloads
aliases before the settings shell mounts. `useSettingsNavigation.ts` may
redirect `/settings` and still canonicalize current settings destinations, but
it must not translate removed infrastructure subpaths into onboarding queries or
derive Proxmox platform state from those paths.
The shared frontend source/platform vocabulary now also includes
`availability` as an agentless monitoring source and `network-endpoint` as the
canonical resource projection. Source labels, badges, settings add-flow copy,
and availability management copy must use shared presentation helpers instead
of feature-local wording, so availability probes stay visually aligned with
the Monitoring availability settings surface without pretending to be a host
agent install or a platform API connection.
Availability setup presets for pingable machines/devices, MQTT, ESPHome, or
similar agentless endpoints must also stay on the shared settings form
vocabulary: presets may fill target kind, protocol, port, and path defaults,
but display badges and drawers still derive `Availability` and
`Network Endpoint` labels from the shared resource presentation helpers rather
than from preset-local copy.
Infrastructure rows for those same agentless endpoints must surface probe
evidence directly in the row, not just as a green status dot or an
`Availability` badge. The shared row presentation must expose the probe method
and latest latency or failure result once, inline in the agentless endpoint's
metric slot, while keeping recent check timing and fuller failure context in
the tooltip or drawer so operators can understand what was measured without
duplicated row chrome.
Operational navigation for those agentless endpoints belongs to the
frontend-primitives-owned Machines surface as a focused Availability checks tab
rather than a new primary nav item. The page may show availability checks beside
standalone Pulse Agent machines, but Settings remains the add/edit owner and
the app shell must not add a separate top-level Availability destination.
The Machines page must not pretend its machine list is a generic overview:
the default tab is `Machines`, the Machines table is only for Pulse Agent-backed
resources with host telemetry, and the full availability-check row list belongs
to the `Availability checks` tab. Its default disk column follows the platform
host-table scan pattern: multi-disk machines render compact per-disk mini-bars
so operators can quickly see disk count and pressure distribution, while sorting
still uses the highest-usage operational filesystem, platform plumbing stays out
of the visible disk set, and full per-filesystem labels remain in hover/detail
affordances rather than turning the row into a raw mount browser. Servers,
laptops, desktops, and comparable
computers monitored only by agentless reachability checks may use
`targetKind=machine` in the availability form, but they stay in Availability
checks until a Pulse Agent registers and supplies CPU, memory, disk, and network
telemetry. Machines empty and handoff actions must lead to Pulse Agent install
or the Availability checks tab, not to an agentless machine row in Machines.
