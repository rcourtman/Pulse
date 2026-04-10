# Performance And Scalability Contract

## Contract Metadata

```json
{
  "subsystem_id": "performance-and-scalability",
  "lane": "L10",
  "contract_file": "docs/release-control/v6/internal/subsystems/performance-and-scalability.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["ai-runtime", "api-contracts", "cloud-paid", "frontend-primitives", "storage-recovery", "unified-resources"]
}
```

## Purpose

Own measurable performance budgets, query-plan guarantees, and hot-path
regression protection.

## Canonical Files

1. `pkg/metrics/store.go`
2. `pkg/metrics/store_query_plan_test.go`
3. `pkg/metrics/store_slo_test.go`
4. `internal/api/slo.go`
5. `internal/api/slo_bench_test.go`
6. `frontend-modern/src/components/Dashboard/Dashboard.tsx`
7. `frontend-modern/src/components/Dashboard/DashboardStateCards.tsx`
8. `frontend-modern/src/components/Dashboard/DashboardStatsStrip.tsx`
9. `frontend-modern/src/components/Dashboard/DashboardWorkloadTable.tsx`
10. `frontend-modern/src/components/Dashboard/WorkloadPanel.tsx`
11. `frontend-modern/src/components/Dashboard/WorkloadTableHeader.tsx`
12. `frontend-modern/src/components/Dashboard/useDashboardState.ts`
13. `frontend-modern/src/components/Dashboard/useDashboardControlsState.ts`
14. `frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`
15. `frontend-modern/src/components/Dashboard/useDashboardGuestMetadataState.ts`
16. `frontend-modern/src/components/Dashboard/useDashboardSelectionState.ts`
17. `frontend-modern/src/components/Dashboard/useDashboardWorkloadRouteState.ts`
18. `frontend-modern/src/components/Dashboard/useDashboardWorkloadUrlSync.ts`
19. `frontend-modern/src/components/Dashboard/DashboardFilter.tsx`
20. `frontend-modern/src/components/Dashboard/dashboardFilterModel.ts`
21. `frontend-modern/src/components/Dashboard/useDashboardFilterState.ts`
22. `frontend-modern/src/components/Dashboard/ThresholdSlider.tsx`
23. `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts`
24. `frontend-modern/src/components/Dashboard/useThresholdSliderState.ts`
25. `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`
26. `frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts`
27. `frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts`
28. `frontend-modern/src/components/Dashboard/StackedMemoryBar.tsx`
29. `frontend-modern/src/components/Dashboard/stackedMemoryBarModel.ts`
30. `frontend-modern/src/components/Dashboard/useStackedMemoryBarState.ts`
31. `frontend-modern/src/components/Dashboard/MetricBar.tsx`
32. `frontend-modern/src/components/Dashboard/metricBarModel.ts`
33. `frontend-modern/src/components/Dashboard/useMetricBarState.ts`
34. `frontend-modern/src/components/Dashboard/EnhancedCPUBar.tsx`
35. `frontend-modern/src/components/Dashboard/enhancedCpuBarModel.ts`
36. `frontend-modern/src/components/Dashboard/useEnhancedCPUBarState.ts`
37. `frontend-modern/src/components/Dashboard/DiskList.tsx`
38. `frontend-modern/src/components/Dashboard/diskListModel.ts`
39. `frontend-modern/src/components/Dashboard/useDiskListState.ts`
40. `frontend-modern/src/components/Dashboard/GuestRow.tsx`
41. `frontend-modern/src/components/Dashboard/GuestRowCells.tsx`
42. `frontend-modern/src/components/Dashboard/guestRowModel.tsx`
43. `frontend-modern/src/components/Dashboard/useGuestRowState.ts`
44. `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`
45. `frontend-modern/src/components/Dashboard/GuestDrawerOverview.tsx`
46. `frontend-modern/src/components/Dashboard/guestDrawerModel.ts`
47. `frontend-modern/src/components/Dashboard/useGuestDrawerState.ts`
48. `frontend-modern/src/components/Dashboard/useGroupedTableWindowing.ts`
49. `frontend-modern/src/components/Dashboard/workloadSelectors.ts`
50. `frontend-modern/src/components/Dashboard/workloadTopology.ts`
51. `frontend-modern/src/components/Dashboard/dashboardSelectionModel.ts`
52. `frontend-modern/src/components/Dashboard/dashboardWorkloadRouteModel.ts`
53. `frontend-modern/src/components/Dashboard/dashboardWorkloadFilterConfigModel.ts`
54. `frontend-modern/src/components/Dashboard/dashboardWorkloadRouteStateModel.ts`
55. `frontend-modern/src/components/Dashboard/dashboardWorkloadUrlSyncModel.ts`
56. `frontend-modern/src/components/Dashboard/useDashboardWorkloadFilterOptions.ts`
57. `frontend-modern/src/components/Dashboard/__tests__/dashboardSelectionModel.test.ts`
58. `frontend-modern/src/components/Dashboard/__tests__/dashboardWorkloadFilterConfigModel.test.ts`
59. `frontend-modern/src/components/Dashboard/__tests__/dashboardWorkloadRouteModel.test.ts`
60. `frontend-modern/src/components/Dashboard/__tests__/dashboardWorkloadRouteStateModel.test.ts`
61. `frontend-modern/src/components/Dashboard/__tests__/dashboardWorkloadUrlSyncModel.test.ts`
62. `frontend-modern/src/components/Dashboard/__tests__/workloadTopology.test.ts`
63. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
64. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
65. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
66. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`
67. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`
68. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
69. `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
70. `frontend-modern/src/utils/throughputPresentation.ts`
71. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx`
72. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx`
73. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts`
74. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts`
75. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
76. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
77. `frontend-modern/src/components/Dashboard/__tests__/DashboardFilter.test.tsx`
78. `frontend-modern/src/components/Dashboard/__tests__/useDashboardFilterState.test.ts`
79. `frontend-modern/src/components/Dashboard/__tests__/useDashboardSelectionState.test.ts`
80. `frontend-modern/src/components/Dashboard/MetricBar.test.tsx`
81. `frontend-modern/src/components/Dashboard/__tests__/useMetricBarState.test.tsx`
82. `frontend-modern/src/components/Dashboard/__tests__/EnhancedCPUBar.test.tsx`
83. `frontend-modern/src/components/Dashboard/__tests__/useEnhancedCPUBarState.test.tsx`
84. `frontend-modern/src/components/Dashboard/ThresholdSlider.test.tsx`
85. `frontend-modern/src/components/Dashboard/__tests__/useThresholdSliderState.test.ts`
86. `frontend-modern/src/components/Dashboard/__tests__/StackedDiskBar.test.tsx`
87. `frontend-modern/src/components/Dashboard/__tests__/useStackedDiskBarState.test.tsx`
88. `frontend-modern/src/components/Dashboard/StackedMemoryBar.test.tsx`
89. `frontend-modern/src/components/Dashboard/__tests__/useStackedMemoryBarState.test.tsx`
90. `frontend-modern/src/components/Dashboard/__tests__/DiskList.test.tsx`
91. `frontend-modern/src/components/Dashboard/__tests__/GuestRow.test.tsx`
92. `frontend-modern/src/components/Dashboard/GuestDrawer.test.tsx`
93. `frontend-modern/src/components/Dashboard/__tests__/useGroupedTableWindowing.test.ts`
94. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`
95. `frontend-modern/src/components/Dashboard/useDashboardWorkloadViewportSync.ts`
96. `frontend-modern/src/components/Dashboard/__tests__/useDashboardWorkloadViewportSync.test.tsx`
97. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
98. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
99. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts`
100. `frontend-modern/src/hooks/useDashboardTrends.ts`
101. `frontend-modern/src/hooks/__tests__/useDashboardTrends.test.ts`
102. `frontend-modern/src/components/Storage/DashboardStoragePanel.tsx`
103. `frontend-modern/src/components/Storage/StorageSummary.tsx`
104. `frontend-modern/src/utils/storageSummaryCache.ts`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `unified-resources`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx` shared with `unified-resources`: the infrastructure summary surface is both a canonical unified-resource consumer and a fleet-scale summary chart hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts` shared with `unified-resources`: infrastructure summary chart matching, focused-summary view derivation, and metric-series shaping are both a canonical unified-resource consumer surface and a fleet-scale summary chart hot-path boundary.
4. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `unified-resources`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
5. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx` shared with `unified-resources`: the unified resource host table card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
6. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx` shared with `unified-resources`: the unified resource PBS section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
7. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx` shared with `unified-resources`: the unified resource PMG section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
8. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx` shared with `unified-resources`: the unified resource service infrastructure card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
9. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `unified-resources`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
10. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts` shared with `unified-resources`: unified resource service row shaping and I/O emphasis are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
11. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts` shared with `unified-resources`: unified resource table state derivation, sort-cycle policy, service sorting, and responsive column layout are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
12. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts` shared with `unified-resources`: infrastructure summary chart polling, cache hydration, and summary-state orchestration are both a canonical unified-resource consumer surface and a fleet-scale summary chart hot-path boundary.
13. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts` shared with `unified-resources`: unified resource table state, grouping, and windowing are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
14. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts` shared with `unified-resources`: unified resource table viewport sync and selected-row reveal are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
15. `internal/api/slo.go` shared with `api-contracts`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add performance budgets through SLO or contract tests
2. Add query-plan guardrails for DB-backed hot paths
3. Optimize hot paths only when backed by benchmarks or proven query issues
4. Extend dashboard hot-path filter, sort, grouping, and stats math through `frontend-modern/src/components/Dashboard/workloadSelectors.ts`, and extend workload identity, discovery routing, and node-topology helpers through `frontend-modern/src/components/Dashboard/workloadTopology.ts`, rather than duplicating selector or topology logic in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
5. Normalize dashboard workload view-mode aliases through `frontend-modern/src/utils/workloads.ts` instead of keeping local URL/storage parsing in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
6. Deduplicate dashboard workload rows by canonical workload ID from `frontend-modern/src/utils/workloads.ts` rather than via local pass-through wrappers in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
7. Render dashboard row identity directly from the shared canonical workload helper so row selection, hover, and fallback metadata lookup stay aligned with the same workload contract
8. Format infrastructure sensor labels through the shared `frontend-modern/src/utils/textPresentation.ts` presentation helper instead of maintaining a local title-casing implementation in `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
9. Extend dashboard row contract and per-row hot-path derivations through `frontend-modern/src/components/Dashboard/guestRowModel.tsx` and `frontend-modern/src/components/Dashboard/useGuestRowState.ts`, and extend tooltip-backed row cell presentation through `frontend-modern/src/components/Dashboard/GuestRowCells.tsx`, rather than rebuilding column metadata, row identity, cell tooltips, or anomaly correlation inside `frontend-modern/src/components/Dashboard/GuestRow.tsx`
10. Extend dashboard drawer derivations and runtime wiring through `frontend-modern/src/components/Dashboard/guestDrawerModel.ts` and `frontend-modern/src/components/Dashboard/useGuestDrawerState.ts`, and extend drawer overview rendering through `frontend-modern/src/components/Dashboard/GuestDrawerOverview.tsx`, rather than rebuilding canonical guest identity, discovery routing, or drawer-local normalization inside `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`
11. Extend dashboard disk-list derivations and fallback runtime wiring through `frontend-modern/src/components/Dashboard/diskListModel.ts` and `frontend-modern/src/components/Dashboard/useDiskListState.ts` rather than rebuilding usage math, progress-state mapping, or tooltip fallback logic inside `frontend-modern/src/components/Dashboard/DiskList.tsx`
12. Extend dashboard guest metadata cache persistence, metadata refresh, org-scope switching, and optimistic custom-URL updates through `frontend-modern/src/components/Dashboard/useDashboardGuestMetadataState.ts` rather than rebuilding dashboard-local storage caches, event listeners, or guest metadata API wiring inside `frontend-modern/src/components/Dashboard/useDashboardState.ts`
13. Extend dashboard deep-link selection and hovered-row continuity semantics through `frontend-modern/src/components/Dashboard/dashboardSelectionModel.ts`, and extend table scroll preservation plus reactive selection state through `frontend-modern/src/components/Dashboard/useDashboardSelectionState.ts`, rather than rebuilding resource-query parsing, selected-row scroll pinning, or hovered-row invalidation inside `frontend-modern/src/components/Dashboard/useDashboardState.ts`; canonical typed workload IDs such as `app-container:<host>:<provider-id>` must remain exact route/selection keys and must not be reinterpreted into synthetic node scopes
14. Extend dashboard workload route ownership, route-driven option catalogs, and toolbar filter config through `frontend-modern/src/components/Dashboard/useDashboardWorkloadRouteState.ts`, `frontend-modern/src/components/Dashboard/useDashboardWorkloadFilterOptions.ts`, `frontend-modern/src/components/Dashboard/dashboardWorkloadRouteModel.ts`, `frontend-modern/src/components/Dashboard/dashboardWorkloadFilterConfigModel.ts`, and `frontend-modern/src/components/Dashboard/dashboardWorkloadRouteStateModel.ts`, and extend query-param synchronization plus managed workload URL semantics through `frontend-modern/src/components/Dashboard/useDashboardWorkloadUrlSync.ts` and `frontend-modern/src/components/Dashboard/dashboardWorkloadUrlSyncModel.ts`, rather than rebuilding route sync, alias parsing, option derivation, toolbar callback/config wiring, reset policy, node-selection compatibility rules, param precedence, or managed workload URLs inside `frontend-modern/src/components/Dashboard/useDashboardState.ts`
15. Extend grouped dashboard workload derivation, summary fallbacks, and grouped/windowed table presentation through `frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`, extend viewport-driven grouped table synchronization through `frontend-modern/src/components/Dashboard/useDashboardWorkloadViewportSync.ts`, and extend node parent mapping through `frontend-modern/src/components/Dashboard/workloadTopology.ts`, rather than rebuilding grouped selectors, summary snapshot math, scroll listeners, or topology lookups inside `frontend-modern/src/components/Dashboard/useDashboardState.ts`
16. Extend dashboard control defaults, persistent view preferences, keyboard reset behavior, column-visibility ownership, and tag-search flow through `frontend-modern/src/components/Dashboard/useDashboardControlsState.ts` and `frontend-modern/src/components/Dashboard/dashboardFilterModel.ts` rather than rebuilding sort/search/grouping state, reset drift, or column-toggle plumbing inside `frontend-modern/src/components/Dashboard/useDashboardState.ts`
17. Extend dashboard filter active-count, reset semantics, and mobile toolbar state through `frontend-modern/src/components/Dashboard/dashboardFilterModel.ts` and `frontend-modern/src/components/Dashboard/useDashboardFilterState.ts`, rather than rebuilding filter-local state inside `frontend-modern/src/components/Dashboard/DashboardFilter.tsx`
18. Extend threshold-slider value-position math, title/label derivation, and drag scroll-lock runtime through `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts` and `frontend-modern/src/components/Dashboard/useThresholdSliderState.ts` rather than rebuilding slider-local state and pointer lifecycle inside `frontend-modern/src/components/Dashboard/ThresholdSlider.tsx`
19. Extend stacked disk-bar capacity math, segment/tooltip derivation, and resize-observer runtime through `frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts` and `frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts` rather than rebuilding disk-bar-local state, mode branching, and tooltip shaping inside `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`
20. Extend stacked memory-bar capacity math, balloon/swap derivation, and resize-observer runtime through `frontend-modern/src/components/Dashboard/stackedMemoryBarModel.ts` and `frontend-modern/src/components/Dashboard/useStackedMemoryBarState.ts` rather than rebuilding memory-bar-local state, tooltip shaping, and label-fit logic inside `frontend-modern/src/components/Dashboard/StackedMemoryBar.tsx`
21. Extend metric-bar width, label-fit logic, and resize-observer runtime through `frontend-modern/src/components/Dashboard/metricBarModel.ts` and `frontend-modern/src/components/Dashboard/useMetricBarState.ts` rather than rebuilding metric-local state and threshold mapping inside `frontend-modern/src/components/Dashboard/MetricBar.tsx`
22. Extend enhanced CPU bar usage/anomaly presentation and tooltip runtime through `frontend-modern/src/components/Dashboard/enhancedCpuBarModel.ts` and `frontend-modern/src/components/Dashboard/useEnhancedCPUBarState.ts` rather than rebuilding tooltip-local state and CPU-threshold formatting inside `frontend-modern/src/components/Dashboard/EnhancedCPUBar.tsx`
23. Extend grouped dashboard row windowing, reveal-index clamping, overscan math, and per-group visible-slice derivation through `frontend-modern/src/components/Dashboard/useGroupedTableWindowing.ts`, and extend viewport event wiring through `frontend-modern/src/components/Dashboard/useDashboardWorkloadViewportSync.ts` rather than rebuilding scroll handlers, mounted-row budgets, viewport listeners, or group-slice math inside `frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`
24. Extend dashboard shell rendering through `frontend-modern/src/components/Dashboard/DashboardStateCards.tsx`, `frontend-modern/src/components/Dashboard/DashboardWorkloadTable.tsx`, and `frontend-modern/src/components/Dashboard/DashboardStatsStrip.tsx` rather than accreting loading cards, workload table markup, or stats-strip presentation back into `frontend-modern/src/components/Dashboard/Dashboard.tsx`
25. Extend dashboard workload table shell ownership through `frontend-modern/src/components/Dashboard/WorkloadTableHeader.tsx` and `frontend-modern/src/components/Dashboard/WorkloadPanel.tsx` rather than rebuilding sortable header markup, grouped node rows, row expansion, or guest-drawer rendering inside `frontend-modern/src/components/Dashboard/DashboardWorkloadTable.tsx`
26. Keep long-range workload chart capping time-proportional across `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`, `frontend-modern/src/api/charts.ts`, and `internal/api/router.go`: when the workload hot path caps mixed-cadence history for top cards, it must bucket by time window rather than raw point index so 7-day and 30-day workload cards stay visually even without relaxing the protected payload budget.
27. Keep summary hover/focus and sticky-card behavior on shared hot paths: infrastructure, workloads, and storage summary shells must reuse one page/group/entity scope model plus `frontend-modern/src/components/shared/StickySummarySection.tsx` inside the app scroll shell instead of per-page scroll listeners or per-card hover derivations, so row scrubbing highlights all cards, workload group headers, infrastructure cluster headers, and storage pool-group headers scope the summary coherently, pinned group focus remains route-backed and reversible, and the hot path does not multiply render or scroll work. That hot path stays row-first rather than adding fallback chrome: the on-screen row or group header is the scoped state, and any explicit reset belongs to one compact shared table-header action plus the shared `Escape` path, not to page-level scope strips, search-row widgets, or filter-bar badges. Background whitespace clearing may exist as a convenience, but the hot path must not depend on brittle dead-space hit testing as the only reversible control. The same hot path must therefore keep one page-level reset owner for filters plus pinned summary selections, and it must keep chart-backed summary-card geometry explicit and stable so hover rerenders, synchronized readouts, or idle header metadata cannot feed layout loops that grow or shrink the top cards over time. Recovery’s summary rail is not part of this interactive hot path; it may share summary-card framing, but it must remain non-interactive until a separately governed model says otherwise.
    The input path for that hot summary contract must stay shared too:
    `frontend-modern/src/components/shared/summaryInteractionA11y.ts` owns
    fine-pointer preview and focus-preview continuity, while
    `frontend-modern/src/components/shared/SummaryRowActionButton.tsx` owns
    deliberate open/pin controls for summary-linked leaf rows and explicit row
    chrome. Group headers may pin through the header row itself when that
    keeps the hot path visually native, but they must not reintroduce local
    scope/pinned pill buttons that compete with the summary shell. Workloads,
    infrastructure, and storage do not rebuild mouse-only hover branches,
    focusable-row toggles, or touch-hostile synthetic hover behavior inside
    individual row renderers. The same hot path must also carry block-level
    group feedback through one shared row-state contract: when a summary group
    is previewed or pinned, member rows should take a restrained shared
    preview/pinned wash via `data-summary-group-member-active` rather than
    per-surface outlines, secondary buttons, or full-strength row fills.
28. Keep summary-card hover emphasis on one bounded rendering budget: when a summary row is active, shared sparkline and density-map primitives must promote the selected series and demote background series through the same active-series ID rather than layering a second page-local highlight pass, so zoom-range and hover scrubbing stay visually coherent without reintroducing multi-series overdraw on the hot summary cards. Density maps on that hot path must stay overview-first under focus: preserve the multi-entity heatmap rows, layer focused-entity detail inside the card, and avoid swapping transient hover into a separate single-series chart path.
29. Keep public self-hosted checkout handoff endpoints on the adjacent
    commercial/router boundary, not the summary-chart hot path. When
    `internal/api/router.go`, `internal/api/router_routes_cloud.go`, or
    `internal/api/licensing_handlers.go` evolve
    `/auth/license-purchase-start` or `/auth/license-purchase-activate`,
    performance work may keep those routes
    cheap and redirect-safe, but it must not treat purchase-return callbacks as
    chart-transport hot paths, fold summary-card caching into commercial
    callback behavior, or reuse those public auth endpoints as a justification
    for relaxing the protected history payload budgets that belong elsewhere.
30. Keep dashboard summary-chart fetches scope-owned rather than page-churn-owned: `frontend-modern/src/hooks/useDashboardTrends.ts` must hydrate infrastructure and storage summaries once per org/range scope from the canonical summary caches and recompute card presentation locally as the compact dashboard overview changes, rather than refetching the infrastructure-summary transport in `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`, the dashboard storage-summary trend transport in `frontend-modern/src/utils/storageSummaryTrendCache.ts`, or the storage-page summary transport in `frontend-modern/src/utils/storageSummaryCache.ts` for every top-resource or card reshuffle on the same dashboard load. That dashboard infrastructure path must also request only the metrics it renders through the canonical infrastructure-summary route owned by `internal/api/router_routes_monitoring.go` and `internal/api/router.go`; the dashboard may not pay for disk or network summary series when it only renders CPU and memory. App-shell prewarm in `frontend-modern/src/useAppRuntimeState.ts` must not front-run that dashboard-specific route while the operator is already on the root dashboard route owned by `frontend-modern/src/App.tsx`.
31. Keep the dashboard overview hot path compact and route-owned. `frontend-modern/src/pages/Dashboard.tsx`, `frontend-modern/src/api/resources.ts`, and `frontend-modern/src/hooks/useDashboardOverview.ts` must hydrate KPI cards, problem-resource rows, and top-infrastructure identities through the compact dashboard-summary API contract owned by the adjacent `api-contracts` and `unified-resources` surfaces, rather than booting the full unfiltered paginated unified-resource list just to derive summary cards.
32. Keep infrastructure summary consumers on the compact dashboard overview rather than reopening the all-resources hook. `frontend-modern/src/hooks/useDashboardTrends.ts`, `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`, and adjacent dashboard summary consumers may derive chart identity and storage presence from the overview payload they were already given, but they must not call `useResources()` or mount a second unfiltered unified-resource fetch path inside the dashboard hot path. That rule also applies to globally mounted helpers such as `frontend-modern/src/components/AI/Chat/index.tsx`: closed assistant surfaces must read the live websocket snapshot or existing unified-resource cache rather than forcing the dashboard to pay for `all-resources` just because the shell component is mounted.
33. Keep hidden workload-route selector shells off the hot path. When the
    workloads route keeps `frontend-modern/src/components/shared/InfrastructureSelector.tsx`
    mounted only for layout parity, `frontend-modern/src/components/shared/useInfrastructureSelectorState.ts`
    must not hydrate `all-resources` or recovery rollups behind a hidden node
    summary; selector-owned data hooks must be explicitly visibility-gated so
    `/workloads` only pays for workload-owned transports.

## Forbidden Paths

1. Speculative micro-optimizations without evidence
2. New N+1 data loading paths on dashboard/resource views
3. Hot-path query changes without updating plan or SLO guardrails

## Completion Obligations

1. Update benchmarks, SLOs, or query-plan tests when hot-path behavior changes
2. Update this contract when a new protected hot path is adopted
3. Route runtime changes through the explicit performance proof policies in `registry.json`; default fallback proof routing is not allowed
4. Record the evidence source for any claimed performance improvement

## Current State

This lane already has strong evidence and guardrails, but it still trails on
score because critical hot paths need more complete protection and verification.
Compact physical-disk drawer charts now also belong to the protected hot path.
Thirty-minute storage detail charts must query the same in-memory plus
store-backed disk history path as longer-term disk charts, with the backend
owning range selection and fallback. Feature-local polling loops or
browser-side disk ring buffers are forbidden because they duplicate live
sampling work and drift out of sync with the governed history timeline.

All governed performance-owned runtime files now require explicit registry
path-policy coverage, so new protected hot paths must be mapped to a concrete
proof route instead of falling back to subsystem-default verification.
Monitored-system admission and entitlement usage now also sit on a protected
backend hot path. `internal/api/monitored_system_limit_enforcement.go` and
the unified-resource projection helpers must reuse one current monitored-
system snapshot plus prospective candidate/preview projection for add and
update checks instead of rescanning platform inventories per handler or
falling back to zero when monitor state is unavailable.
That protected path now also includes supplemental-inventory settlement.
Performance work may not shortcut monitored-system usage readiness to "store
exists" once provider-owned platforms such as TrueNAS or VMware suppress
snapshot-owned sources; the hot path must fail closed until the monitor has
both observed an initial baseline for every active connection and rebuilt the
canonical store at or after the latest provider watermark.

The dashboard workload selector path and the dashboard runtime that consumes it
are now part of the protected performance surface rather than proof-only
context. Future hot-path filter/group/sort/windowing changes must route through
the explicit dashboard performance proof policy in the subsystem registry.
Route-backed workload `resource` focus on that hot path is contextual state
only, not inferred filter state: opening or closing an inline drawer must not
invent, retain, or clear `agent` or node-scope filters unless those filters
were already explicit in the managed workload URL.
That same hot-path ownership now covers top-of-page summary emphasis: infrastructure
and workloads summary cards must treat row hover, chart hover, and route focus
as one shared active-series contract so time-range switches, row scrubbing, and
chart cross-highlighting reuse one existing chart path instead of repainting
page-local “selected row” overlays on top of already downsampled summary
history. Hovering a sparkline or density map for one entity must promote that
entity into the shared active series so sibling cards highlight the same object
at once rather than maintaining chart-local hover state, and the synchronized
hover timestamp must remain visible across those sibling cards even when the
active entity has no samples for one metric in the current range. Those
sibling cards should expose the synchronized value through one compact
header-level readout, not by spawning duplicate floating tooltips on every
chart.
Infrastructure cluster-header hover now belongs to that same bounded hot path:
hovering a grouped infrastructure header must scope the top cards to that
cluster's unified-resource members through the shared group/entity interaction
That same hot path now also owns dashboard freshness discipline. Supported
unified-resource dashboard reads may hydrate from REST for first paint, but
once websocket `state.resources` is available they must consume that canonical
live snapshot directly instead of turning each websocket update into a second
REST fetch. Dashboard trend loading must also key off stable target identity
and selected range only; reconciled resource snapshots that do not change the
effective target set must not trigger duplicate infrastructure or storage
chart requests.
That same protected metrics-store boundary now also owns selected-series batch
queries. Compact dashboard routes that request only CPU/memory or only
storage `used`/`avail` capacity must keep that metric-type filter all the way
through `pkg/metrics/store.go` instead of fetching every series for every
resource and discarding the extra payload in higher layers.
That same hot path now also covers mock-mode cache warmth. The canonical
24-hour `/api/charts/storage-summary` dashboard transport must stay prewarmed
across live mock sampler ticks so the first dashboard request after a refresh
does not pay the full aggregate-storage synthesis cost on the operator path.
contract instead of inventing an infrastructure-local summary filter branch.
For shared line charts on that hot path, the shared sparkline primitive may
isolate the selected series inside the existing render budget, but that
isolation must still reuse the same summary series set and timeline data rather
than triggering a second page-local chart recomputation.
That same hot-path rule now covers contextual row focus on those pages.
That same hot-path ownership now also forbids hover-driven table movement.
When a summary chart promotes one active entity, the matching row may highlight
in place if it is already mounted and visible, but the hot path must not
auto-scroll the page or rebuild the table into a one-row filtered view on
transient hover. Off-screen reveal must stay behind an explicit `Jump to row`
action routed through the shared summary-table focus bridge.
That same hot-path contract also owns the row-emphasis paint. Dashboard guest
rows and infrastructure resource rows must expose summary-linked activity
through the shared `data-summary-row-active` marker and let the shared frontend
primitive render the emphasis, instead of layering lane-local row-fill classes
that diverge across pages or wash out inline metric bars.
`frontend-modern/src/components/Dashboard/useDashboardSelectionState.ts` must
write workload selection back into the workloads route through the shared
same-path route-state scheduler, but the actual shell-position handoff for
query-only row focus must go through `frontend-modern/src/utils/appShellScrollRestoration.ts`
plus the root `frontend-modern/src/App.tsx` shell so opening a focused
workload does not look like a full page reload, and the governed infrastructure and
workloads summary surfaces must keep the summary page-scoped while that focus
reuses the shared highlight contract; density maps may retain page-level
context, but they must now also surface focused-entity detail inside the same
card instead of dimming the rest of the map into unusable background noise or
swapping the hot path into a transient single-series chart. Line-card
isolation must still flow through the shared sparkline runtime instead of a
page-local focus overlay.
That same hot-path rule now also covers infrastructure drawer hydration. Row
focus may mount drawer-local facet and intelligence fetches, but those
requests must retain the mounted page shell through the shared non-suspending
query helper instead of bubbling a transient `Loading view...` fallback through
`AppLayout.tsx` when an inline infrastructure drawer opens.
That same hot-path rule now has one shared runtime boundary. Interactive-series
filtering, active-series derivation, focused-label lookup, and local
scroll-preserving row focus must extend
`frontend-modern/src/components/shared/contextualFocus.ts` instead of
rebuilding page-local `Set` scans or scroll repair logic in dashboard,
infrastructure, or workloads hot paths.
That same hot-path ownership now also covers deliberate inline-detail reveal.
When a focused workload or infrastructure row opens its inline detail, the hot
path may preserve scroll across same-route state writes, but the actual reveal
must still flow through the shared contextual-focus and summary-table helpers.
Direct row toggles that already have the row in view must capture the current
app-shell scroll position before the focus write and let the remounted root
shell restore that position, so the interaction stays anchored instead of
looking like a full refresh. Once the drawer is mounted, the shared reveal
helper must still take over whenever the opened detail would land below the
fold, marking that movement as deliberate so route-state restore does not
replay over it and then scrolling only enough to keep the row header plus the
top of the detail visible instead of hard-centering every expansion.
That same hot-path ownership now includes summary cache invalidation.
Infrastructure and workload summary caches may hydrate charts for fast remounts,
but when the summary chart timeline contract changes, the cache version must
advance and stale payloads must be purged on read so long-lived browser sessions
cannot keep rendering pre-fix sparkline shapes after the backend timeline model
has been corrected.
That same cache contract also applies to same-tab in-memory hydration on the
protected summary hot path. Infrastructure and workload summary shells must
version their module-scoped remount caches alongside the chart contract so a
hot-reloaded or long-lived browser tab cannot immediately rehydrate an older
summary series set before the next live fetch completes.
That same hot-path rule now applies to infrastructure summary resource
filtering: `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
must include API-backed systems such as top-level TrueNAS appliances through
the shared `isAgentFacetInfrastructureResource(...)` helper instead of a local
`resource.type` branch, so the summary poll/cache path stays on one canonical
infrastructure selector contract.
That same protected dashboard hot path now includes storage trend loading too.
`frontend-modern/src/hooks/useDashboardTrends.ts` and
`internal/api/router.go` must keep dashboard storage trends on one compact
`/api/charts/storage-summary` request backed by
`GetStorageMetricsForChartBatch(...)`, so the dashboard does not pull the full
storage-page `/api/storage-charts` payload or reopen an N+1 per-pool
`/api/metrics-store/history` fan-out just to compute the shared 24-hour
storage capacity delta.
That same dashboard shell boundary also owns empty-state action routing in
`frontend-modern/src/components/Dashboard/DashboardStateCards.tsx`. When the
dashboard has no connected infrastructure, the CTA must hand operators
directly to the canonical infrastructure install route via
`buildInfrastructureWorkspacePath('install')` instead of bouncing through a
generic settings landing page.
That workload route now also treats readiness as a route-owned contract instead
of a raw websocket proxy signal: `frontend-modern/src/components/Dashboard/Dashboard.tsx`
and `frontend-modern/src/components/Dashboard/useDashboardState.ts` must keep
filters, stats, and table visibility driven by the workload route's own
REST-backed health so websocket churn does not hide already-fetched workloads
or swap the protected hot path into a false disconnected shell. The first
websocket connect on that route is not a reconnect and must not trigger an
extra workload REST refetch; only later false-to-true recoveries may pay that
refresh cost.
The same performance ownership now applies to the downsampled
`pkg/metrics/store.go` batched query path. `QueryAllBatch` may drop the global
SQLite `ORDER BY resource_id, metric_type, bucket_ts` sort from the grouped
query only if the runtime preserves ascending timestamps plus correct
avg/min/max bucket aggregates within every returned resource/metric series, and
the hot-path proof keeps latency, ordering, and aggregate correctness guarded
together in the explicit metrics SLO surface. That protected surface should
That same workload-summary hot path now also owns chart-cache invalidation
whenever the shaped timeline contract changes. `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
must version-bust cached summary payloads in the same slice that changes
workload chart bucket semantics or timestamp precision, so operators are not
served stale mixed-cadence chart shapes after the backend timeline model has
already been corrected.
stay on one ordered index scan plus Go-side bucket aggregation rather than
forcing SQLite to `GROUP BY` computed buckets through a temp B-tree on the
fleet-scale dashboard path.
That runtime is now intentionally split by concern:
`frontend-modern/src/components/Dashboard/useDashboardState.ts` owns
top-level dashboard orchestration, workload loading, and composition across
the canonical dashboard owners, while
`frontend-modern/src/components/Dashboard/useDashboardControlsState.ts`
owns persistent control defaults, keyboard-reset semantics, sort/search/tag
behavior, column visibility, and summary display preferences, while
`frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`
owns grouped workload derivation, summary fallbacks, parent-node mapping,
and grouped/windowed table math, while
`frontend-modern/src/components/Dashboard/useDashboardWorkloadViewportSync.ts`
owns grouped workload viewport synchronization and the scroll/resize listener
lifecycle, while
`frontend-modern/src/components/Dashboard/useDashboardGuestMetadataState.ts`
owns guest metadata cache persistence, optimistic custom-URL updates,
org-scope switching, and metadata refresh, and
`frontend-modern/src/components/Dashboard/useDashboardWorkloadRouteState.ts`
owns workload-route synchronization, deep-link normalization, and route-scoped
filter contracts. Future dashboard hot-path changes must extend through those
owners instead of accreting back into `frontend-modern/src/components/Dashboard/Dashboard.tsx`.
That same route-owned filter contract now also includes canonical workload
`platform` scoping for API-backed runtimes. Dashboard URL-sync, filter-option
assembly, and workload drill-down routes must preserve
`platform=<owned-source-key>` for surfaces such as TrueNAS app-containers,
while treating host or cluster identity as secondary scope instead of
collapsing those routes back to generic agent-only semantics. The derived
workload hot path must also tolerate transient async route/data gaps by
treating missing filtered-workload collections as empty until the route and
resource snapshots converge, rather than crashing the dashboard between sync
phases.
That same workload hot path also owns the split between canonical
app-container routing and Docker-only actions. `frontend-modern/src/hooks/useWorkloads.ts`,
`frontend-modern/src/components/Dashboard/workloadTopology.ts`, and
`frontend-modern/src/components/Dashboard/useGuestRowState.ts` must preserve a
canonical app-container navigation path while keeping Docker runtime action
identifiers explicit. Discovery affordances on the dashboard drawer must follow
the canonical `discoveryTarget` contract, not generic app-container host
fallbacks. TrueNAS app-containers may reuse runtime metadata such as image and
runtime strings, but they must not inherit Docker-only update affordances or
agent-only Discovery tabs on the dashboard row path unless the unified
resource contract explicitly supplies discovery ownership.
That same row-and-selection boundary also requires canonical app-container
identity to stay intact on the workload surface. `/workloads` rows, deep links,
drawer selection, anomaly correlation, and route-owned filters must key on the
unified resource ID such as `app-container:truenas-main:nextcloud`, while
Docker-native runtime IDs stay in a separate action-only field for Docker
controls like image updates. Performance work must not collapse API-backed
platforms back onto runtime-native container IDs or derive fake node scopes
from typed canonical resource IDs.
That same summary hot path must also keep workload hover identity canonical
across provider-backed history. Row hover and focus plus top-card isolation on
`/workloads` must resolve against the same canonical workload ID even when the
backing history is stored under a provider metrics target, so provider-backed
VM rows do not silently drop out of summary emphasis.
The shared infrastructure table hot path now also treats operator-facing
resource identity as a protected boundary: sorting, searching, summary-series
matching, and row titles on the infrastructure page must use the canonical
local instance identity rather than governed AI-summary text, so performance
work cannot “optimize” the table into ambiguous labels that collapse multiple
resources into the same visible name.
That derived workload owner now also routes grouped row windowing through
`frontend-modern/src/components/Dashboard/useGroupedTableWindowing.ts`, which
owns row-window thresholds, overscan behavior, reveal-index clamping, and
per-group visible-slice derivation. Future dashboard table windowing changes
must extend through that hook instead of rebuilding scroll math or mounted-row
budgets inline inside `frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`.
Viewport-driven grouped table synchronization now also routes through
`frontend-modern/src/components/Dashboard/useDashboardWorkloadViewportSync.ts`,
which owns the dashboard table body measurement and the scroll/resize listener
lifecycle. Future viewport sync changes must extend through that hook rather
than rebuilding browser-event wiring or table-body geometry reads inside
`frontend-modern/src/components/Dashboard/useDashboardWorkloadDerivedState.ts`.
The dashboard guest-row path now follows the same pattern: the render shell
stays in `frontend-modern/src/components/Dashboard/GuestRow.tsx`, tooltip-backed
cell presentation lives in `frontend-modern/src/components/Dashboard/GuestRowCells.tsx`,
and the canonical row contract and per-row hot-path derivations live in
`frontend-modern/src/components/Dashboard/guestRowModel.tsx` and
`frontend-modern/src/components/Dashboard/useGuestRowState.ts`. Future row
identity, column, cell-tooltip, anomaly-correlation, and link-state changes
must extend through those owners instead of rebuilding row-local state inside
the shell.
That per-row link state now also consumes the shared
`frontend-modern/src/routing/resourceLinks.ts` workload-to-infrastructure
helper instead of a dashboard-local routing shim. Future infrastructure-link
changes for dashboard rows must extend through the shared routing owner rather
than recreating feature-local path builders.
That shell now also routes tag-dot rendering through the shared
`frontend-modern/src/components/shared/TagBadges.tsx` primitive instead of
keeping a dashboard-local badge helper. Future guest-row tag presentation
changes must extend through that shared owner rather than reintroducing a
dashboard-only tag-badge variant.
The dashboard guest drawer now follows that same ownership rule: the shell
stays in `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`, the
overview card surface lives in
`frontend-modern/src/components/Dashboard/GuestDrawerOverview.tsx`, and
drawer-local normalization, backup/tag formatting, discovery identity wiring,
and workload-derived navigation state live in
`frontend-modern/src/components/Dashboard/guestDrawerModel.ts` and
`frontend-modern/src/components/Dashboard/useGuestDrawerState.ts`. Future
drawer runtime and overview-surface changes must extend through those owners
instead of adding more mixed state and helper drift back into the shell.
That drawer state now also consumes the same shared
`frontend-modern/src/routing/resourceLinks.ts` workload-to-infrastructure
helper, so row and drawer navigation stay aligned without a second
dashboard-local link-mapping file.
The shared infrastructure mapper hot path now stays intentionally narrow:
`frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
continues to own sensor-label presentation and hot-path host/agent projection,
while canonical drawer discovery-target derivation now lives in
`frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts`
under `unified-resources`. Future discovery-config or target-resolution
changes must extend through that unified-resource owner instead of
re-accumulating discovery heuristics back into the performance hot-path mapper.
The dashboard disk list now follows the same pattern: the shell stays in
`frontend-modern/src/components/Dashboard/DiskList.tsx`, while disk-row
presentation derivations and fallback tooltip/runtime wiring live in
`frontend-modern/src/components/Dashboard/diskListModel.ts` and
`frontend-modern/src/components/Dashboard/useDiskListState.ts`. Future disk
usage math, threshold-color routing, and fallback handling must extend
through those owners instead of accreting back into the shell.
The dashboard filter now follows that same ownership rule: the shell
stays in `frontend-modern/src/components/Dashboard/DashboardFilter.tsx`, while
toolbar defaults, active-filter counting, and reset semantics live in
`frontend-modern/src/components/Dashboard/dashboardFilterModel.ts` and
`frontend-modern/src/components/Dashboard/useDashboardFilterState.ts`.
The dashboard-owned filter-config assembly now lives in
`frontend-modern/src/components/Dashboard/useDashboardState.ts`, so future
filter runtime changes must extend through those owners instead of
reintroducing dashboard-local state, reset drift, or inline config assembly
into the shell.
The dashboard threshold slider now follows that same pattern: the shell stays
in `frontend-modern/src/components/Dashboard/ThresholdSlider.tsx`, while
metric-type text and fill presentation live in
`frontend-modern/src/utils/thresholdSliderPresentation.ts`, while
slider bounds math, thumb-position transforms, and title/label derivations
live in `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts`
and drag scroll-lock lifecycle lives in
`frontend-modern/src/components/Dashboard/useThresholdSliderState.ts`.
Future slider runtime changes must extend through those owners instead of
reintroducing mixed drag state, type-color formatting, and presentation logic
into the shell.
The dashboard stacked disk bar now follows that same pattern: the shell stays
in `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`, while
disk-capacity math, segment and tooltip derivation, max-disk labeling, and
mode-specific presentation live in
`frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts` and
resize-observer plus tooltip lifecycle live in
`frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts`.
Future disk-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state and presentation branching into the shell.
The dashboard stacked memory bar now follows that same pattern: the shell
stays in `frontend-modern/src/components/Dashboard/StackedMemoryBar.tsx`,
while memory-capacity math, balloon/swap tooltip derivation, anomaly label
presentation, and sublabel-fit logic live in
`frontend-modern/src/components/Dashboard/stackedMemoryBarModel.ts` and
resize-observer plus tooltip lifecycle live in
`frontend-modern/src/components/Dashboard/useStackedMemoryBarState.ts`.
Future memory-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state, balloon branching, and tooltip shaping into
the shell.
The dashboard metric bar now follows that same pattern: the shell stays in
`frontend-modern/src/components/Dashboard/MetricBar.tsx`, while width,
show-label, sublabel-fit, and threshold-color derivation live in
`frontend-modern/src/components/Dashboard/metricBarModel.ts` and
resize-observer lifecycle lives in
`frontend-modern/src/components/Dashboard/useMetricBarState.ts`. Future
metric-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state and label-fit logic into the shell.
The dashboard enhanced CPU bar now follows that same pattern: the shell stays
in `frontend-modern/src/components/Dashboard/EnhancedCPUBar.tsx`, while usage
formatting, anomaly presentation, tooltip load-average formatting, and
threshold-driven label state live in
`frontend-modern/src/components/Dashboard/enhancedCpuBarModel.ts` and
tooltip lifecycle lives in
`frontend-modern/src/components/Dashboard/useEnhancedCPUBarState.ts`. Future
CPU-bar runtime changes must extend through those owners instead of
reintroducing mixed tooltip state and formatting logic into the shell.

The unified resource table hot path is now also governed as explicit
performance-owned runtime, with shared ownership against the unified-resource
consumer boundary. The remaining performance work is no longer top-level
ownership ambiguity on the main dashboard or infrastructure tables.
The table's reactive runtime, grouping, and row-windowing owner now lives in
`frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`,
while pure table-state derivation, service sorting, sort-cycle policy, and
responsive column layout now live in
`frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`,
and viewport-sync plus selected-row reveal behavior now live in
`frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`,
so future hot-path table-state changes must not fold selector derivation,
layout policy, and scroll coordination back into one mixed owner or the render
shell.
That hot-path contract now includes policy badge rendering on resource rows.
The infrastructure summary hot path is now explicit shared ownership too:
`InfrastructureSummary.tsx` stays a render shell,
`useInfrastructureSummaryState.ts` owns chart polling and cache lifecycle, and
`infrastructureSummaryModel.ts` owns chart matching, focused-summary display
selection, empty-state wording, and summary-series/metric derivation. Future
summary-chart work must not put polling, cache hydration, and series math
back into the shell.
The summary API feeding that hot path must also normalize mixed-resolution
history into equal-time summary buckets before it reaches the shell/runtime
owners, so long-range cards do not bunch recent higher-resolution samples at
the right edge.
It now also includes the compact resource-facet summary chips rendered next
to policy metadata, and those chips must stay within the same bounded
windowing and mounted-row budget proved by
`UnifiedResourceTable.performance.contract.test.tsx`.
The same facet summary contract applies to the service-resource rows inside
the unified table as well, so PBS and PMG entries must keep the same bounded
presentation and verification surface as the primary fleet rows. The shared
`ResourceFacetSummary` component now owns that chip rendering path, so any
future summary changes must preserve the same bounded row budget instead of
forking separate table-only presentation logic. That component now also
consumes the shared `frontend-modern/src/utils/resourceChangePresentation.ts`
label helper for canonical change kinds, source types, and adapter provenance
so the chip wording stays consistent without adding extra hot-path branching.
The default table hot path now scopes those summary chips to timeline and
change-provenance badges only. Generic capability and relationship badges are
removed from the default row surface entirely until the underlying data is
proven populated, which preserves the fleet-table scan path and avoids
spending hot-path visual budget on model nouns that do not yet clear the
product bar.
Row summaries now also prefer canonical `facetCounts` on each resource when
they are available, so the hot path can stay within the same budget while
still reading totals from the shared resource contract. The drawer history
surface reuses the same governed resource route helpers for relationship and
related-resource links, so cross-resource navigation stays within the existing
infrastructure surface rather than branching into custom detail-only routing.
The detail drawer now follows the same default posture by collapsing its
history overview down to timeline counts and timeline-summary chips, so the
performance-sensitive shared presentation path stays aligned with the
investigation-first product contract instead of rendering low-signal generic
facet sections by default.
Governance metadata such as sensitivity and routing scope may be visible in
the table, but it must remain on the same bounded row-windowing and mounted-row
budget proved by `UnifiedResourceTable.performance.contract.test.tsx` rather
than creating a separate unbounded rendering path for policy-rich fleets.
The shared table and detail drawer now also render governed resource labels
through the shared identity/display contract, which routes policy-aware
resources through the canonical policy-aware helper and suppresses the raw
alternate name when policy requires governed handling. That keeps the
policy-aware label path inside the same hot-row rendering budget instead of
adding a second display branch for redacted fleets, and the proof for that
behavior lives in `UnifiedResourceTable.performance.contract.test.tsx`.
The shared table now also passes the same canonical resource-label resolver
into the detail drawer so related-resource chips in the timeline/history path
can resolve through the canonical catalog without adding a separate
detail-only lookup branch to the hot-row path.
The same detail drawer also uses that resolver for correlation dependency and
dependent chips, so the investigation path does not fall back to raw IDs in
the drawer while the AI page keeps its broader no-catalog fallback.
The shared infrastructure selector search path now also routes through that
same preferred resource display contract, so governed resources do not
reappear via raw-name search candidates while the selector stays on the same
hot-path budget.
The shared workloads-link helper used by the resource drawer and table now
also routes its Kubernetes-cluster fallback through the same preferred
resource display contract, so navigation context does not leak raw
`displayName` values for governed clusters.
That same workloads-link path and the dashboard workload projection now also
share the canonical cluster-name helpers in the shared agent-resource layer,
so route labels, pod grouping, and cluster-name fetch keys keep using the
same source of truth instead of rebuilding the `clusterName`/`context`/
`clusterId` prefix locally.
The infrastructure host-table hot path now also suppresses the default
`Internal` + `Cloud Summary` policy pair in row chrome. That baseline posture
still belongs to the canonical policy contract, but repeating it on every host
burns row-density budget without adding operator-grade signal.
The shared node adapter also uses that same cluster-name helper for the
infrastructure summary surface, so Proxmox node projections stay aligned with
the same canonical cluster label instead of carrying a raw adapter-local
cluster string.
The drawer's Kubernetes namespace/deployment tabs use the canonical
cluster-name helper for fetch keys, so the visible navigation label stays
separate from the backend cluster lookup contract.
The workloads projection in `useWorkloads` also uses that same helper for pod
context labels, keeping the dashboard's Kubernetes grouping aligned with the
same canonical cluster-name boundary.
The drawer's discovery mapper also reuses that helper for pod fallback agent
IDs, so the resource-detail hot path and the dashboard selector path stay
aligned on the same cluster-name source of truth.
The unified-resource projection also uses that same helper for Kubernetes
`clusterId`, so the shared store, dashboard grouping, and detail-navigation
surfaces all see the same cluster-context prefix before any surface-specific
fallback applies.
The aggregate `/api/charts/workloads-summary` endpoint now also has its own
explicit API p95 budget constant, aligned with the per-workload charts budget,
and `internal/api/slo_bench_test.go` must fail if that aggregate budget or its
store-backed mixed-workload benchmark coverage drifts.
The infrastructure and workload summary cards now share a canonical
throughput-rate formatter in `frontend-modern/src/utils/throughputPresentation.ts`,
so bytes-per-second labels stay consistent between the two summary surfaces
instead of each component carrying its own rate string builder.
That shared throughput boundary is now also explicitly governed here.
`frontend-modern/src/components/Workloads/WorkloadsSummary.tsx` is the
canonical workload-summary hot-path surface, and
`frontend-modern/src/utils/throughputPresentation.ts` is the canonical
bytes-per-second formatter shared by workload and infrastructure summary
cards. Future throughput wording or workload-summary hot-path changes must
extend these performance-owned surfaces instead of leaving the formatter or
summary shell unowned in registry coverage.

Infrastructure selector status ordering must now tolerate arbitrary filter-set
strings without widening the canonical hot-path order tuple. Unknown statuses
must sort after the governed status order instead of forcing the selector path
to abandon the typed canonical order used by the infrastructure table and its
performance proof surface.

The Infrastructure page now also normalizes source filter keys through the
shared `frontend-modern/src/utils/sourcePlatforms.ts` helper directly, so the
selector boundary keeps using the canonical source-platform contract instead of
maintaining a local source-normalization alias.

Resource detail mappers now also use the shared
`frontend-modern/src/utils/textPresentation.ts` title-case helper for sensor
labels, so the canonical presentation layer owns that wording instead of the
mapper carrying its own title-casing branch.

Dashboard, workload-summary, infrastructure-summary, and org-scoped cache-key
paths now normalize org scope through the shared
`frontend-modern/src/utils/orgScope.ts` helper instead of each file carrying
its own `getOrgID() || 'default'` fallback. That keeps cache isolation and
multi-tenant row-scoping aligned across the dashboard and resource-summary hot
paths.

GitHub-hosted runner proof for the API performance surface now intentionally
uses a looser budget envelope than local/staging benchmark runs for the
mixed-endpoint load test and the infrastructure/workload chart p95 checks.
Those CI targets remain regression guardrails, but they are calibrated to the
observed contention and CPU variability of the public release workflow rather
than to workstation-class latency.

The metrics store write boundary now fails closed on malformed samples. Empty
resource identifiers, empty metric names, unsupported tiers, and legacy
resource-type writes must be dropped before they reach SQLite so the governed
hot-path query surface cannot silently accumulate unqueryable garbage rows that
inflate store size and distort downstream performance evidence.
Canonical metrics resource types now also normalize case at the store boundary.
`agent`, `vm`, and other governed resource-type keys must not drift into
uppercase or mixed-case variants that write successfully but fragment the
metrics hot path into effectively separate query spaces.
The query boundary must use the same canonicalization rules. Resource
identifiers and metric names passed back into `Query`, `QueryAll`, and
`QueryAllBatch` must be normalized the same way writes are, so whitespace- or
case-polluted callers cannot manufacture false "missing metrics" results,
split one governed metric stream into mixed-case query buckets, or trigger
redundant batch work against otherwise valid stored samples.
That same metrics-store boundary also owns the persistent DB file path. The
runtime must normalize the owned metrics directory and resolve the selected DB
filename through the shared storage-path helper before it creates directories
or opens SQLite, instead of trusting raw caller-built paths.
That same metrics hot path must also keep startup maintenance off the
constructor critical path. `NewStore` may initialize schema and return a usable
store, but restart-time retention cleanup and one-time auto-vacuum migration
must run through the background worker so large historical databases do not
block application startup before the metrics store is even available.
That same metrics hot path now also owns compact physical-disk drawer charts.
Thirty-minute storage detail charts must query the same in-memory plus
store-backed disk history path as longer-term disk charts, with the backend
doing range selection and fallback. Feature-local polling loops or browser-side
disk ring buffers are forbidden because they duplicate live sampling work and
drift out of sync with the governed history timeline.
That same hot-path ownership now also requires lazy-load-safe websocket
consumption. `frontend-modern/src/components/Dashboard/useDashboardState.ts`
may read connection and alert state only through
`frontend-modern/src/contexts/appRuntime.ts`; it must not import `@/App` or
create a reverse dependency into the root shell just to read websocket state.
The same hot-path discipline now applies to public-demo commercial reads.
Shared shells may consume the small `/api/license/runtime-capabilities`
contract for feature truth, but commercial demo routes stay hidden and the
browser must not keep retrying `/api/license/commercial-posture`,
`/api/license/entitlements`, `/auth/license-purchase-start`, or other hidden
commercial endpoints from performance-sensitive settings or dashboard shells.
Dashboard and infrastructure summary consumers now also keep null-tolerant read
models on the shared hot path. Guest rows, stacked bars, anomaly summaries, and
resource detail mappers may accept partial platform metadata or undefined
ratios, but they must normalize those values once in the shared model layer
instead of scattering non-null assertions or per-component count coercion
through the dashboard runtime.
