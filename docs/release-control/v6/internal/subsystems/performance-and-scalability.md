# Performance And Scalability Contract

## Contract Metadata

```json
{
  "subsystem_id": "performance-and-scalability",
  "lane": "L10",
  "contract_file": "docs/release-control/v6/internal/subsystems/performance-and-scalability.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["api-contracts", "frontend-primitives"]
}
```

## Purpose

Own measurable performance budgets, query-plan guarantees, and hot-path
regression protection.

## Canonical Files

1. `pkg/metrics/store.go`
2. `pkg/metrics/store_query_plan_test.go`
3. `pkg/metrics/store_slo_test.go`
4. `pkg/metrics/store_additional_test.go`
4. `internal/api/slo.go`
5. `internal/api/slo_bench_test.go`
6. `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
7. `frontend-modern/src/components/Workloads/WorkloadsStateCards.tsx`
8. `frontend-modern/src/components/Workloads/WorkloadsStatsStrip.tsx`
9. `frontend-modern/src/components/Workloads/WorkloadsTable.tsx`
10. `frontend-modern/src/components/Workloads/WorkloadPanel.tsx`
11. `frontend-modern/src/components/Workloads/WorkloadTableHeader.tsx`
12. `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
13. `frontend-modern/src/components/Workloads/useWorkloadsControlsState.ts`
14. `frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`
15. `frontend-modern/src/components/Workloads/useWorkloadGuestMetadataState.ts`
16. `frontend-modern/src/components/Workloads/useWorkloadSelectionState.ts`
17. `frontend-modern/src/components/Workloads/useWorkloadRouteState.ts`
18. `frontend-modern/src/components/Workloads/useWorkloadUrlSync.ts`
19. `frontend-modern/src/components/Workloads/WorkloadsFilter.tsx`
20. `frontend-modern/src/components/Workloads/workloadsFilterModel.ts`
22. `frontend-modern/src/components/Workloads/ThresholdSlider.tsx`
23. `frontend-modern/src/components/Workloads/thresholdSliderModel.ts`
24. `frontend-modern/src/components/Workloads/useThresholdSliderState.ts`
25. `frontend-modern/src/components/Workloads/StackedDiskBar.tsx`
26. `frontend-modern/src/components/Workloads/stackedDiskBarModel.ts`
27. `frontend-modern/src/components/Workloads/useStackedDiskBarState.ts`
28. `frontend-modern/src/components/Workloads/StackedMemoryBar.tsx`
29. `frontend-modern/src/components/Workloads/stackedMemoryBarModel.ts`
30. `frontend-modern/src/components/Workloads/useStackedMemoryBarState.ts`
31. `frontend-modern/src/components/Workloads/MetricBar.tsx`
32. `frontend-modern/src/components/Workloads/metricBarModel.ts`
33. `frontend-modern/src/components/Workloads/useMetricBarState.ts`
34. `frontend-modern/src/components/Workloads/EnhancedCPUBar.tsx`
35. `frontend-modern/src/components/Workloads/enhancedCpuBarModel.ts`
36. `frontend-modern/src/components/Workloads/useEnhancedCPUBarState.ts`
37. `frontend-modern/src/components/Workloads/DiskList.tsx`
38. `frontend-modern/src/components/Workloads/diskListModel.ts`
39. `frontend-modern/src/components/Workloads/useDiskListState.ts`
40. `frontend-modern/src/components/Workloads/GuestRow.tsx`
41. `frontend-modern/src/components/Workloads/GuestRowCells.tsx`
42. `frontend-modern/src/components/Workloads/guestRowModel.tsx`
43. `frontend-modern/src/components/Workloads/useGuestRowState.ts`
44. `frontend-modern/src/components/Workloads/GuestDrawer.tsx`
45. `frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx`
46. `frontend-modern/src/components/Workloads/guestDrawerModel.ts`
47. `frontend-modern/src/components/Workloads/useGuestDrawerState.ts`
48. `frontend-modern/src/components/Workloads/useGroupedTableWindowing.ts`
49. `frontend-modern/src/components/Workloads/workloadSelectors.ts`
50. `frontend-modern/src/components/Workloads/workloadTopology.ts`
51. `frontend-modern/src/components/Workloads/workloadSelectionModel.ts`
52. `frontend-modern/src/components/Workloads/workloadRouteModel.ts`
53. `frontend-modern/src/components/Workloads/workloadFilterConfigModel.ts`
54. `frontend-modern/src/components/Workloads/workloadRouteStateModel.ts`
55. `frontend-modern/src/components/Workloads/workloadUrlSyncModel.ts`
56. `frontend-modern/src/components/Workloads/useWorkloadFilterOptions.ts`
57. `frontend-modern/src/components/Workloads/__tests__/workloadSelectionModel.test.ts`
58. `frontend-modern/src/components/Workloads/__tests__/workloadFilterConfigModel.test.ts`
59. `frontend-modern/src/components/Workloads/__tests__/workloadRouteModel.test.ts`
60. `frontend-modern/src/components/Workloads/__tests__/workloadRouteStateModel.test.ts`
61. `frontend-modern/src/components/Workloads/__tests__/workloadUrlSyncModel.test.ts`
62. `frontend-modern/src/components/Workloads/__tests__/workloadTopology.test.ts`
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
76. `frontend-modern/src/components/Workloads/__tests__/WorkloadsSurface.performance.contract.test.tsx`
77. `frontend-modern/src/components/Workloads/__tests__/WorkloadsFilter.test.tsx`
79. `frontend-modern/src/components/Workloads/__tests__/useWorkloadSelectionState.test.ts`
80. `frontend-modern/src/components/Workloads/MetricBar.test.tsx`
81. `frontend-modern/src/components/Workloads/__tests__/useMetricBarState.test.tsx`
82. `frontend-modern/src/components/Workloads/__tests__/EnhancedCPUBar.test.tsx`
83. `frontend-modern/src/components/Workloads/__tests__/useEnhancedCPUBarState.test.tsx`
84. `frontend-modern/src/components/Workloads/ThresholdSlider.test.tsx`
85. `frontend-modern/src/components/Workloads/__tests__/useThresholdSliderState.test.ts`
86. `frontend-modern/src/components/Workloads/__tests__/StackedDiskBar.test.tsx`
87. `frontend-modern/src/components/Workloads/__tests__/useStackedDiskBarState.test.tsx`
88. `frontend-modern/src/components/Workloads/StackedMemoryBar.test.tsx`
89. `frontend-modern/src/components/Workloads/__tests__/useStackedMemoryBarState.test.tsx`
90. `frontend-modern/src/components/Workloads/__tests__/DiskList.test.tsx`
91. `frontend-modern/src/components/Workloads/__tests__/GuestRow.test.tsx`
92. `frontend-modern/src/components/Workloads/GuestDrawer.test.tsx`
93. `frontend-modern/src/components/Workloads/__tests__/useGroupedTableWindowing.test.ts`
94. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`
95. `frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts`
96. `frontend-modern/src/components/Workloads/__tests__/useWorkloadViewportSync.test.tsx`
97. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
98. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
99. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts`
100. `frontend-modern/src/utils/workloadsSummaryCache.ts`
101. `frontend-modern/src/routing/routePreload.ts`
102. `frontend-modern/src/useAppRuntimeState.ts`
103. `frontend-modern/src/components/Storage/StorageSummary.tsx`
104. `frontend-modern/src/utils/storageSummaryCache.ts`
105. `frontend-modern/src/pages/Workloads.tsx`

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
15. `frontend-modern/src/routing/routePreload.ts` shared with `frontend-primitives`: the app-shell route preload registry is both a canonical frontend shell boundary and an authenticated hot-path performance boundary.
16. `frontend-modern/src/useAppRuntimeState.ts` shared with `cloud-paid`: the authenticated app runtime bootstrap is both a hosted commercial org-context boundary and a protected app-shell performance boundary.
17. `internal/api/slo.go` shared with `api-contracts`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add performance budgets through SLO or contract tests
2. Add query-plan guardrails for DB-backed hot paths
3. Optimize hot paths only when backed by benchmarks or proven query issues
4. Keep shared auth gating in `internal/api/router.go` cheap and local: pre-auth quick-setup and recovery routing may short-circuit on loopback/session/token checks, but they must not trigger chart, metrics, or broad persistence fan-out on the protected request hot path.
   The same rule applies to public setup-script lifecycle routes: `/api/auto-register`
   and `/api/auto-unregister` may bypass the global auth wall so their handlers can
   validate request-body setup tokens, but the router-level public-path check must
   stay O(1) and must not front-run persistence loads, monitor refresh, or other
   teardown/register side effects before the canonical handler owns the request.
   `/api/config/export` and `/api/config/import` follow the same hot-path rule:
   router auth bypass exists only to let their handlers make route-local auth and
   public-network decisions, and the bypass check must remain a constant-time
   path comparison rather than a persistence-backed authorization probe.
   Agent-version signaling follows that same hot-path rule. Shared helpers used
   by `/api/agent/version` and adjacent settings payloads may read the cached
   process version once, but they must not add per-request release lookups,
   filesystem walks, or other heavy work just to compute whether an attached
   agent is current.
   Container runtime migration token minting follows that same rule: adding
   server-derived owner metadata in `internal/api/router.go` must reuse the
   already-authenticated request context or caller token and must not add
   monitor scans, persistence reads, or other broad hot-path work before the
   route-local handler owns the mutation.
   Patrol investigation-record fan-out through shared router callbacks follows
   the same bounded-work rule: the callback may copy the already-materialized
   durable record into unified findings, but it must not add broad resource
   scans, model calls, or persistence walks to protected request setup paths.
   The same rule covers the operator-facing `impact` and `recommendation`
   fields copied from Finding to UnifiedFinding through that router callback:
   the callback may pass them through as already-materialized strings but must
   not invoke models, persistence walks, or evidence aggregation to derive
   them on the protected hot path. The `previous_resolved_fix_summary`
   operational-memory field follows the same constraint: the router callback
   passes the already-captured string from `FindingsStore.Add` rather than
   reaching back into investigation history or persistence to re-derive it
   inside the request setup path.
   Patrol run Assistant handoff wiring in `internal/api/router.go` follows the
   same protected hot-path rule: the shared callback may resolve one requested
   Patrol run ID from the already-owned Patrol service and strip tool traces
   before returning it to AI runtime, but it must not scan run history broadly,
   hydrate resource inventories, call models, or perform persistence fan-out as
   part of router setup or generic request admission.
   Retiring self-hosted trial acquisition follows that same rule: removing
   `/auth/trial-activate` and `POST /api/license/trial/start` from public-path
   and CSRF inventories must stay as constant-time route-table absence rather
   than replacing the old callback with persistence-backed router probes.
   Infrastructure host-table health explanations follow the same hot-path
   rule. `UnifiedResourceHostTableCard.tsx` may render compact issue labels
   from the already-materialized resource incident and storage-risk fields, but
   it must not introduce per-row API reads, broad resource scans, storage
   topology recomputation, or layout-measuring work just to explain warning or
   degraded rows. Infrastructure table source derivation follows the same
   bounded-work rule: `unifiedResourceTableStateModel.ts` must read the
   already-materialized top-level `resource.sources` array before legacy
   `platformData.sources` hints, and must not reconstruct platform identity by
   scanning sibling resources or making row-time API calls.
5. Extend workload hot-path filter, sort, grouping, and stats math through `frontend-modern/src/components/Workloads/workloadSelectors.ts`, and extend workload identity, discovery routing, and node-topology helpers through `frontend-modern/src/components/Workloads/workloadTopology.ts`, rather than duplicating selector or topology logic in `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
   The retired dashboard overview route must not return as a hot-path
   orientation shortcut. First-viewport system count, health, source coverage,
   and freshness now belong to Infrastructure and Add infrastructure, derived
   from their existing runtime/resource state rather than a dashboard-specific
   compact summary fallback. Any future brief-style Assistant handoff needs a
   newly governed owner and must not add route-global polling, model/settings
   fetches, or chart/history reads to the authenticated root path.
6. Normalize workload view-mode aliases through `frontend-modern/src/utils/workloads.ts` instead of keeping local URL/storage parsing in `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
7. Deduplicate workload rows by canonical workload ID from `frontend-modern/src/utils/workloads.ts` rather than via local pass-through wrappers in `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
8. Render workload row identity directly from the shared canonical workload helper so row selection, hover, and fallback metadata lookup stay aligned with the same workload contract
9. Format infrastructure sensor labels through the shared `frontend-modern/src/utils/textPresentation.ts` presentation helper instead of maintaining a local title-casing implementation in `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
10. Extend workload row contract and per-row hot-path derivations through `frontend-modern/src/components/Workloads/guestRowModel.tsx` and `frontend-modern/src/components/Workloads/useGuestRowState.ts`, and extend tooltip-backed row cell presentation through `frontend-modern/src/components/Workloads/GuestRowCells.tsx`, rather than rebuilding column metadata, row identity, cell tooltips, or anomaly correlation inside `frontend-modern/src/components/Workloads/GuestRow.tsx`
11. Extend workload drawer derivations and runtime wiring through `frontend-modern/src/components/Workloads/guestDrawerModel.ts` and `frontend-modern/src/components/Workloads/useGuestDrawerState.ts`, and extend drawer overview rendering through `frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx`, rather than rebuilding canonical guest identity, discovery routing, or drawer-local normalization inside `frontend-modern/src/components/Workloads/GuestDrawer.tsx`
12. Extend workload disk-list derivations and fallback runtime wiring through `frontend-modern/src/components/Workloads/diskListModel.ts` and `frontend-modern/src/components/Workloads/useDiskListState.ts` rather than rebuilding usage math, progress-state mapping, or tooltip fallback logic inside `frontend-modern/src/components/Workloads/DiskList.tsx`
13. Extend workload guest metadata cache persistence, metadata refresh, org-scope switching, and optimistic custom-URL updates through `frontend-modern/src/components/Workloads/useWorkloadGuestMetadataState.ts` rather than rebuilding workload-local storage caches, event listeners, or guest metadata API wiring inside `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
14. Extend workload deep-link selection and hovered-row continuity semantics through `frontend-modern/src/components/Workloads/workloadSelectionModel.ts`, and extend table scroll preservation plus reactive selection state through `frontend-modern/src/components/Workloads/useWorkloadSelectionState.ts`, rather than rebuilding resource-query parsing, selected-row scroll pinning, or hovered-row invalidation inside `frontend-modern/src/components/Workloads/useWorkloadsState.ts`; canonical typed workload IDs such as `app-container:<host>:<provider-id>` must remain exact route/selection keys and must not be reinterpreted into synthetic node scopes
15. Extend workload route ownership, route-driven option catalogs, and toolbar filter config through `frontend-modern/src/components/Workloads/useWorkloadRouteState.ts`, `frontend-modern/src/components/Workloads/useWorkloadFilterOptions.ts`, `frontend-modern/src/components/Workloads/workloadRouteModel.ts`, `frontend-modern/src/components/Workloads/workloadFilterConfigModel.ts`, and `frontend-modern/src/components/Workloads/workloadRouteStateModel.ts`, and extend query-param synchronization plus managed workload URL semantics through `frontend-modern/src/components/Workloads/useWorkloadUrlSync.ts` and `frontend-modern/src/components/Workloads/workloadUrlSyncModel.ts`, rather than rebuilding route sync, alias parsing, option derivation, toolbar callback/config wiring, reset policy, node-selection compatibility rules, param precedence, or managed workload URLs inside `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
    Workloads route host scopes must resolve through `frontend-modern/src/components/Workloads/workloadTopology.ts`: app-container scopes use the canonical Docker/runtime host id before host labels or node fallbacks, while VM and system-container scopes keep the instance-node key. Route `agent` filters and option catalogs must consume that shared scope so Infrastructure related-workload links cannot drift from Workloads filtering.
    The user-facing Workloads Type filter must expose the stable operator
    buckets `All`, `VMs`, `Containers`, and `Pods`. The internal
    `system-container` / `app-container` distinction may continue to parse for
    legacy deep links and exact runtime scopes, but broad container filtering
    must resolve through shared workload view-mode helpers so route sync,
    platform option derivation, row filtering, and column defaults do not
    duplicate hot-path type matching.
16. Extend grouped workload derivation, summary fallbacks, and grouped/windowed table presentation through `frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`, extend viewport-driven grouped table synchronization through `frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts`, and extend node parent mapping through `frontend-modern/src/components/Workloads/workloadTopology.ts`, rather than rebuilding grouped selectors, summary snapshot math, scroll listeners, or topology lookups inside `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
17. Extend workload control defaults, persistent view preferences, keyboard reset behavior, column-visibility ownership, and tag-search flow through `frontend-modern/src/components/Workloads/useWorkloadsControlsState.ts` and `frontend-modern/src/components/Workloads/workloadsFilterModel.ts` rather than rebuilding sort/search/grouping state, reset drift, or column-toggle plumbing inside `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
18. Extend workload filter active-count, reset semantics, and mobile toolbar state through `frontend-modern/src/components/Workloads/workloadsFilterModel.ts` (defaults, `countActiveWorkloadsFilters`, `hasActiveWorkloadsFilters`) rather than rebuilding filter-local state inside `frontend-modern/src/components/Workloads/WorkloadsFilter.tsx`. Workloads filter presentation now composes the chip-based shared `FilterBar` (`frontend-modern/src/components/shared/FilterBar/FilterBar.tsx`) with a per-page `FilterDef[]` catalog rather than the legacy `PageControls` structured control deck. The xl segmented↔select swap retired with the migration; type-ahead in the "+ Filter" menu and chip popovers covers the power-user speed that the segmented controls used to give. View options (grouped/list, charts, columns) sit in the shared `viewOptionsTrailing` slot.
19. Extend threshold-slider value-position math, title/label derivation, and drag scroll-lock runtime through `frontend-modern/src/components/Workloads/thresholdSliderModel.ts` and `frontend-modern/src/components/Workloads/useThresholdSliderState.ts` rather than rebuilding slider-local state and pointer lifecycle inside `frontend-modern/src/components/Workloads/ThresholdSlider.tsx`
20. Extend stacked disk-bar capacity math, segment/tooltip derivation, and resize-observer runtime through `frontend-modern/src/components/Workloads/stackedDiskBarModel.ts` and `frontend-modern/src/components/Workloads/useStackedDiskBarState.ts` rather than rebuilding disk-bar-local state, mode branching, and tooltip shaping inside `frontend-modern/src/components/Workloads/StackedDiskBar.tsx`
21. Extend stacked memory-bar capacity math, balloon/swap derivation, and resize-observer runtime through `frontend-modern/src/components/Workloads/stackedMemoryBarModel.ts` and `frontend-modern/src/components/Workloads/useStackedMemoryBarState.ts` rather than rebuilding memory-bar-local state, tooltip shaping, and label-fit logic inside `frontend-modern/src/components/Workloads/StackedMemoryBar.tsx`
22. Extend metric-bar width, label-fit logic, and resize-observer runtime through `frontend-modern/src/components/Workloads/metricBarModel.ts` and `frontend-modern/src/components/Workloads/useMetricBarState.ts` rather than rebuilding metric-local state and threshold mapping inside `frontend-modern/src/components/Workloads/MetricBar.tsx`
23. Extend enhanced CPU bar usage/anomaly presentation and tooltip runtime through `frontend-modern/src/components/Workloads/enhancedCpuBarModel.ts` and `frontend-modern/src/components/Workloads/useEnhancedCPUBarState.ts` rather than rebuilding tooltip-local state and CPU-threshold formatting inside `frontend-modern/src/components/Workloads/EnhancedCPUBar.tsx`
    Workload and unified-resource metric bars may accept resolved alert display
    threshold props, but they must not choose alert thresholds locally. Scope and
    override resolution belongs to the alerts-owned activation store and
    `frontend-modern/src/utils/metricThresholds.ts`, while the hot-path bar
    models remain presentation-only consumers.
23. Extend grouped workload row windowing, reveal-index clamping, overscan math, and per-group visible-slice derivation through `frontend-modern/src/components/Workloads/useGroupedTableWindowing.ts`, and extend viewport event wiring through `frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts` rather than rebuilding scroll handlers, mounted-row budgets, viewport listeners, or group-slice math inside `frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`
24. Extend Workloads shell rendering through `frontend-modern/src/components/Workloads/WorkloadsStateCards.tsx`, `frontend-modern/src/components/Workloads/WorkloadsTable.tsx`, and `frontend-modern/src/components/Workloads/WorkloadsStatsStrip.tsx` rather than accreting loading cards, workload table markup, or stats-strip presentation back into `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
25. Extend workload table shell ownership through `frontend-modern/src/components/Workloads/WorkloadTableHeader.tsx` and `frontend-modern/src/components/Workloads/WorkloadPanel.tsx` rather than rebuilding sortable header markup, grouped node rows, row expansion, or guest-drawer rendering inside `frontend-modern/src/components/Workloads/WorkloadsTable.tsx`
    Compact icon headers inside `WorkloadTableHeader.tsx` may stay visually
    dense for responsive workload tables, but the icon must be decorative and
    the column label must remain present through an `sr-only` label so the
    header row exposes names such as Uptime, Image, Context, and Node instead
    of collapsing to only the visible text columns.
    Pod-mode workload filters must use the workload-owned
    `K8s cluster` / `All K8s clusters` labels for Kubernetes context selection,
    rather than a generic `Cluster` label that can be confused with Proxmox
    cluster terminology on adjacent infrastructure surfaces, and all workload
    default filter-option labels must flow through the shared all-option
    presentation helper via the workload filter config model instead of the
    hot-path shell hard-coding local `All ...` strings.
26. Keep long-range workload chart capping time-proportional across `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`, `frontend-modern/src/api/charts.ts`, and `internal/api/router.go`: when the workload hot path caps mixed-cadence history for top cards, it must bucket by time window rather than raw point index so 7-day and 30-day workload cards stay visually even without relaxing the protected payload budget.
    Workload summary chart caching and app-shell prewarming belong to
    `frontend-modern/src/utils/workloadsSummaryCache.ts`, not to page-local
    component state. `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
    may hydrate from that shared in-memory/local cache and refresh it, while
    `frontend-modern/src/useAppRuntimeState.ts` may warm the same cache after
    authenticated bootstrap without mounting the Workloads route or starting a
    second polling loop.
27. Keep summary hover/focus and sticky-card behavior on shared hot paths: infrastructure, workloads, and storage summary shells must reuse one page/group/entity scope model plus `frontend-modern/src/components/shared/StickySummarySection.tsx` inside the app scroll shell instead of per-page scroll listeners or per-card hover derivations, so row scrubbing highlights all cards, workload group headers, infrastructure cluster headers, and storage pool-group headers scope the summary coherently, pinned group focus remains route-backed and reversible, and the hot path does not multiply render or scroll work. Pages that keep chart summaries visible below the desktop breakpoint must use the shared desktop-sticky mode on that primitive rather than page-local viewport listeners, so wrapped chart grids do not pin most of the scroll container on narrow screens. That hot path stays row-first rather than adding fallback chrome: the on-screen row or group header is the scoped state, and any explicit reset belongs to one compact shared table-header action plus the shared `Escape` path, not to page-level scope strips, search-row widgets, or filter-bar badges. Background whitespace clearing may exist as a convenience, but the hot path must not depend on brittle dead-space hit testing as the only reversible control. The same hot path must therefore keep one page-level reset owner for filters plus pinned summary selections, and it must keep chart-backed summary-card geometry explicit and stable so hover rerenders, synchronized readouts, or idle header metadata cannot feed layout loops that grow or shrink the top cards over time. Recovery’s summary rail is not part of this interactive hot path; it may share summary-card framing, but it must remain non-interactive until a separately governed model says otherwise.
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
    Static grouped-row header emphasis on the same workload and infrastructure
    tables must route through
    `frontend-modern/src/components/shared/groupedTableRowPresentation.ts` and
    the shared `.grouped-table-row` CSS contract in `frontend-modern/src/index.css`,
    rather than local `bg-surface-alt` or hover-fill variants that drift between
    pages.
    Framed product-table cards on the same hot-path surfaces must consume the
    frontend-primitives-owned `TableCard` wrapper and `TableCardHeader`
    title/action band so Workloads, Infrastructure, Storage, and Recovery keep
    one border/background/overflow frame and table-section header treatment
    instead of introducing page-local table shells with different light/dark
    contrast. The shared `Table` primitive owns the horizontal scroll container
    for those hot tables; page-local nested `overflow-x-auto` wrappers are not
    allowed because they add extra scroll and paint surfaces on the row-heavy
    path.
    Infrastructure table responsive behavior belongs to that same hot-path
    owner. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
    must derive column visibility from the measured table surface width, with
    `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
    owning the column-priority breakpoints. It must not rely on global viewport
    width alone, because app chrome can leave the table with tablet-sized space
    while the window still reports a desktop breakpoint. Live resize proof must
    show the host and service tables dropping lower-priority columns without
    introducing horizontal overflow.
    Dashboard workload table responsive behavior belongs to the same hot-path
    owner. `frontend-modern/src/components/Workloads/useWorkloadsControlsState.ts`
    must derive workload table layout stages from live viewport width, and
    `frontend-modern/src/components/Workloads/guestRowModel.tsx` must own the
    responsive workload column priority and width model. `WorkloadsTable`,
    `WorkloadTableHeader`, and `WorkloadPanel` must consume one shared
    layout-visible column set so headers, colgroups, and rows stay aligned during
    live resize. View-mode-specific column sets in
    `frontend-modern/src/components/Workloads/guestRowModel.tsx` must retain
    core monitoring metrics when narrowing scope: app-container mode keeps CPU,
    memory, disk, and the responsive Net I/O and Disk I/O columns available,
    while the layout priority model decides when detail-heavy I/O columns are
    rendered. Filtering to a workload type must not silently remove capacity or
    I/O observability that remains available in the all-workloads table. Tablet
    and compact workload stages must normalize active column widths against the
    currently visible column IDs, show higher-priority workload information
    before exposing detail-heavy Net I/O and Disk I/O columns, and avoid
    horizontal overflow at mobile, tablet, compact, and full desktop widths.
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
    That same shared callback path must also keep install-version attribution
    O(1): `internal/api/router.go` may pass the cached process
    `serverVersion` into `internal/api/licensing_handlers.go`, but
    performance work must not add filesystem reads, GitHub release lookups, or
    other per-request version discovery on activation, legacy exchange, or
    grant-refresh traffic just to stamp authenticated install metadata.
30. Keep retired dashboard summary-chart paths absent rather than replacing
them with new hot-path fetches. Infrastructure summary cards must continue to
hydrate through `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
and the canonical infrastructure-summary route owned by
`internal/api/router_routes_monitoring.go` and `internal/api/router.go`; storage
summary cards must use the storage summary cache owners. The authenticated
root path in `frontend-modern/src/App.tsx` must not prewarm a deleted
dashboard-specific overview, trend, or summary transport.
    The same hot path must keep mock/demo chart identity on the canonical
    unified snapshot too: when mock mode is enabled, `internal/api/router.go`
    may not read `/api/charts`, `/api/charts/infrastructure`, or
    `/api/storage-charts` from the live store-backed `ReadState` if that would
    omit provider-backed resources already present in the canonical mock
    unified-resource graph.
    The same app-shell bootstrap boundary now also governs demo-hidden
    organization context. When presentation policy suppresses organization
    chrome, `frontend-modern/src/useAppRuntimeState.ts` may retain a hidden
    default org scope for route-safe API calls, but it must skip browser org
    list hydration and must not turn dashboard landing on `frontend-modern/src/App.tsx`
    into protected pre-auth churn. On public login entrypoints (`/` and
    `/login`), if auth is configured but the browser has no local auth hint
    yet, the app shell must stop at the login-needed state instead of probing
    `/api/state` and paying an avoidable `401`; once a local-login hint exists
    or the operator is on a protected route, that shared state probe remains
    the canonical runtime detector.
    That same app-shell performance boundary treats global commercial banners as
    non-hot-path UI. Retired self-hosted trial or managed-model acquisition
    surfaces must not be reintroduced into `frontend-modern/src/App.tsx` as
    always-mounted chrome that preloads commercial posture or adds route-global
    render work to ordinary monitoring sessions. The authenticated app shell
    must also avoid background commercial-posture bootstrap while
    `presentationPolicy.hideUpgrade` suppresses upgrade prompts.
    Cloud signup retirement follows the same route-table rule: `/cloud` and
    `/cloud/signup` must stay absent from the ordinary self-hosted app shell
    rather than being replaced with public-route wrappers that preload checkout
    or hosted-account state.
    Invitation accept/revoke and other org-membership changes follow that same
    hot-path contract: `frontend-modern/src/useAppRuntimeState.ts` may reload
    the org list from the shared `organizations_changed` event, but that refresh
    must stay event-driven and route-safe rather than expanding into a second
    full app bootstrap, a pre-auth org probe, or a deleted dashboard-route prewarm
    that duplicates the canonical summary fetch path or expands into another
    summary-fetch or org-bootstrap hot path.
    The same protected hot path also owns Patrol route compatibility: if
    `frontend-modern/src/App.tsx` keeps `/ai` as a legacy alias while `/patrol`
    is canonical, that alias must stay a thin redirect and must not mount a
    second Patrol shell or duplicate app bootstrap work before navigation
    settles on the canonical route.
    The same rule now applies to the retired `/operations` surface: legacy
    `/operations/*` links may redirect into Settings support routes, but they
    must not mount a second diagnostics/reporting shell or pay extra bootstrap
    work before the canonical Settings URL takes over.
    Authenticated `/login` recovery belongs to that same app-shell boundary:
    `frontend-modern/src/App.tsx` must redirect that route back to the
    canonical Infrastructure landing path instead of leaving the freshly
    authenticated shell on a not-found page that immediately pays another cold
    bootstrap.
    App-shell route module preloading belongs to
    `frontend-modern/src/routing/routePreload.ts` and the delayed authenticated
    preload call in `frontend-modern/src/App.tsx`. `frontend-modern/src/useAppRuntimeState.ts`
    must not keep page-local dynamic imports or a second route-preload cache,
    because the runtime bootstrap already owns state, chart cache warming, org
    hydration, and health probing.
    The same protected hot path now also owns proof harness steadiness.
    Store-backed chart SLO and benchmark helpers in `pkg/metrics/store_slo_test.go`,
    `internal/api/slo_bench_test.go`, and `internal/monitoring/monitor_metrics_slo_test.go`
    must wait for deferred metrics-store startup maintenance to quiesce before
    timing steady-state reads, so one-time retention or auto-vacuum cleanup does
    not masquerade as summary-route or chart-batch regression latency.
31. Keep the retired dashboard overview route absent from the protected hot
path. `frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/hooks/useDashboardOverview.ts`, and
`/api/resources/dashboard-summary` must not be restored as compatibility
surfaces for KPI cards, problem-resource rows, or top-infrastructure
identities. New summary cards must live on their owning product route and
prove their data path there.
32. Keep infrastructure and assistant consumers off deleted dashboard summary
state. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
and globally mounted helpers such as `frontend-modern/src/components/AI/Chat/index.tsx`
must read the live websocket snapshot or existing unified-resource cache
rather than forcing root navigation to pay for a replacement dashboard
overview transport. When the assistant shell changes presentation,
`frontend-modern/src/utils/aiChatPresentation.ts` must remain the canonical
owner for launcher, drawer, session-menu, and empty-state copy so hot-path
consumers do not grow one-off inline strings or extra state branches alongside
the mounted shell. Blocking shared dialogs must also suppress closed assistant
affordances through the shared dialog runtime instead of leaving the mounted
shell clickable behind another overlay.
    Approval presentation inside that mounted assistant shell must stay
    state-local to the existing drawer/session state and backend approval
    endpoints. Deny/skip failure handling may preserve the pending approval
    card, but it must not add polling, resource hydration, or mounted-shell work
    to recover UI state.
    That same mounted-shell hot path must protect usable width on constrained
    viewports. When the shared assistant drawer opens inside
    `frontend-modern/src/components/AI/Chat/index.tsx`, it may not shrink the
    infrastructure or dashboard operating surface below a workable narrow-width
    floor just because the assistant stays mounted in the app shell; below the
    canonical dock threshold, the assistant must switch to an overlay drawer so
    table filters, grouped rows, and other hot-path controls keep their
    existing layout budget instead of paying a second collapse cost.
33. Keep hidden workload-route selector shells off the hot path. When the
    workloads route keeps `frontend-modern/src/components/shared/InfrastructureSelector.tsx`
    mounted only for layout parity, `frontend-modern/src/components/shared/useInfrastructureSelectorState.ts`
    must not hydrate `all-resources` or recovery rollups behind a hidden node
    summary; selector-owned data hooks must be explicitly visibility-gated so
    `/workloads` only pays for workload-owned transports.
34. Keep the retired dashboard page-header path absent from the compact hot
    path. New page headers must stay pure presentation on their owning route
    and must not introduce a second data load, widen suspense ownership, or
    force deleted dashboard summaries back through full-resource fetch paths
    just to satisfy page
    chrome.
35. Keep the Workloads table CSP-safe on the hot path. The renderers
    in `frontend-modern/src/components/Workloads/WorkloadsTable.tsx`,
    `frontend-modern/src/components/Workloads/WorkloadTableHeader.tsx`,
    `frontend-modern/src/components/Workloads/GuestRow.tsx`,
    `frontend-modern/src/components/Workloads/EnhancedCPUBar.tsx`,
    `frontend-modern/src/components/Workloads/StackedMemoryBar.tsx`, and
    `frontend-modern/src/components/Workloads/StackedDiskBar.tsx` may still
    use shared models plus SVG or HTML attributes for widths, offsets, and
    colors, but they must not fall back to inline `style=` attributes on the
    public shell just to express virtualization spacers, alert accents, or
    workload metric bars.
36. Keep the unified connections ledger off the polling hot path. Changes to
    `internal/api/router.go` that register `/api/connections` and
    `/api/connections/probe` must keep those routes off the monitoring fan-out
    budget: `GET /api/connections` must resolve purely from
    `monitoring.Monitor.SchedulerHealth()` plus existing per-type config
    stores without triggering any live network probes, and blocked metadata,
    link-local, multicast, and unspecified probe destinations must be
    rejected during request validation so the route does not spend dial budget
    or become a side-channel scanner against forbidden targets. The probe path
    must also keep the validated destination pinned through the restricted
    outbound client so DNS rebinding cannot swap targets after admission, and
    `POST /api/connections/probe` must remain bounded at 3s total /
    2s dial / 1s read with at most 5 concurrent fingerprints so the probe
    endpoint cannot be repurposed into a slow-leak scanner that starves the
    dashboard hot path.

## Forbidden Paths

1. Speculative micro-optimizations without evidence
2. New N+1 data loading paths on dashboard/resource views
3. Hot-path query changes without updating plan or SLO guardrails

## Completion Obligations

1. Update benchmarks, SLOs, or query-plan tests when hot-path behavior changes
2. Update this contract when a new protected hot path is adopted
3. Route runtime changes through the explicit performance proof policies in `registry.json`; default fallback proof routing is not allowed
4. Record the evidence source for any claimed performance improvement
5. Keep wide-desktop infrastructure table layout proof on the shared owner.
   Changes to `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
   that affect host, PBS, or PMG desktop column widths must ship with shared
   model verification plus desktop Playwright proof that full-width shells
   distribute surplus width across peer columns instead of stretching only the
   `Resource` column.
6. Keep entitlement and runtime capability lookups on shared router/settings
   hot paths bounded and request-local. AI control-level clamping in
   `internal/api/router.go` may consult the already-wired runtime entitlement
   service, but it must not add broad persistence scans, metrics fan-out, or
   external network calls to protected settings or chat request paths.

## Current State

The new-finding hot path now consults a per-resource operator-state
provider when one is wired (slice 31). The provider call is gated on
`f.ResourceID != ""` and a non-nil provider so deployments without
the feature wired pay no extra work; the provider implementation
itself is a single SQLite point lookup keyed on `canonical_id`, no
join, no scan. Maintenance-window suppression therefore adds at most
one read per new finding, not per existing-finding update.

Summary cards for Infrastructure, Storage, Workloads, and Recovery now surface
health-state counts (offline, degraded, alerting) instead of raw online/offline
splits. `InfrastructureSummary.tsx` and `infrastructureSummaryModel.ts` add
`degraded` and `alerting` resource counts; `StorageSummary.tsx` and
`useStoragePageSummary.ts` add `poolsDegraded` and `disksFailing` indicators;
`WorkloadsSummary.tsx` and `useWorkloadsDerivedState.ts` add an
alerting count derived from `activeAlerts`. These additions must remain
read-only projections from existing websocket state — they must not introduce
new polling loops or widen fetch scope on the hot-path boundary.
Agentless availability endpoints participate in the same unified-resource
consumer hot path as other infrastructure resources. Adding
`network-endpoint` to resource queries, filters, and summary counts must reuse
the existing unified-resource hydration and websocket paths; frontend
consumers must not add an endpoint-specific polling loop or a second table
model just to show availability status.

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
Monitored-system grouping preview now also sits on a protected backend hot
path. The removed monitored-system limit-enforcement path must not return;
unified-resource projection helpers should reuse one current monitored-system
snapshot plus prospective candidate/preview projection for settings/support
explanation instead of rescanning platform inventories per handler or falling
back to zero when monitor state is unavailable.
That protected path now also includes supplemental-inventory settlement.
Performance work may not shortcut monitored-system grouping readiness to
"store exists" once provider-owned platforms such as TrueNAS or VMware suppress
snapshot-owned sources; the hot path must report preview unavailability until
the monitor has both observed an initial baseline for every active connection
and rebuilt the canonical store at or after the latest provider watermark.

The Workloads selector path and the Workloads runtime that consumes it
are now part of the protected performance surface rather than proof-only
context. Future hot-path filter/group/sort/windowing changes must route through
the explicit Workloads performance proof policy in the subsystem registry.
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
REST fetch. Route-owned trend loading must also key off stable target identity
and selected range only; reconciled resource snapshots that do not change the
effective target set must not trigger duplicate infrastructure or storage chart
requests.
That same protected metrics-store boundary now also owns selected-series batch
queries. Compact route consumers that request only CPU/memory or only storage
`used`/`avail` capacity must keep that metric-type filter all the way through
`pkg/metrics/store.go` instead of fetching every series for every resource and
discarding the extra payload in higher layers.
That same protected workload-chart hot path now also owns rendered-metric
budgeting. `internal/api/router.go` may parallelize VM, container, pod, and
docker-container workload batch reads, but `/api/charts/workloads` and
`/api/charts/workloads-summary` must request only the canonical five workload
metrics they actually render instead of widening back to disk read/write or
fetch-all history queries.
That same chart-batch hot path now also owns long-range in-memory coverage.
`internal/monitoring/monitor_metrics.go` may skip SQLite for guest and node
chart batches when `metrics_history` can prove the requested window is
already covered in memory, and performance work must preserve that
coverage-gated fast path rather than treating every long-duration request as
store-backed by default.
That same hot path now also covers mock-mode cache warmth. The canonical
24-hour `/api/charts/storage-summary` dashboard transport must stay prewarmed
across live mock sampler ticks so the first dashboard request after a refresh
does not pay the full aggregate-storage synthesis cost on the operator path.
That same chart-client hot path also owns canonical Kubernetes target typing.
`frontend-modern/src/api/charts.ts` may normalize Kubernetes history requests
onto the shared backend `resourceType=k8s` transport, but it must preserve the
canonical frontend target types for clusters, nodes, pods, and deployments so
the workloads and drawer hot paths do not silently drop deployment history or
split Kubernetes charts across incompatible cache keys.
That same protected metrics-store boundary also owns tiered history range
parsing as pre-query work. Day-based ranges such as `14d`, longer day ranges,
and equivalent duration syntax must be normalized and checked against the
licensed `max_history_days` before the store read is planned, so denied history
windows fail without widening DB scans or letting duration strings bypass the
Relay 14-day and Pro 90-day budgets.
That same protected metrics-store boundary also owns write-path churn on local
instances. `pkg/metrics/store.go` must prefer fewer, larger SQLite write
transactions over tiny frequent commits: the default write buffer should stay
large enough to absorb one poll cycle on modest multi-node installs, queued
flush batches should coalesce before commit when the worker is already behind,
and WAL auto-checkpointing should avoid tiny segment rewrite loops that keep
rewriting `metrics.db` on SSD-backed systems. Duplicate samples for the same
resource type, resource id, metric type, timestamp, and tier must be treated as
idempotent writes against the metrics table's unique key rather than as noisy
SQLite constraint failures; for raw writes, the latest buffered value wins and
aggregate-only min/max columns stay unset. Rollups must not be hard-coded to a
5-minute disk-write cadence: the default cadence should favor fewer, larger
rollup transactions, remain bounded by raw retention so data is aggregated
before pruning, and stay overrideable for operators who deliberately trade
freshness for lower write frequency. The runtime must also allow an explicit
metrics database path so SSD-sensitive Docker/LXC installs can place only the
metrics SQLite store on tmpfs while keeping secrets and general configuration on
durable storage. Any future change that reduces that batching headroom, makes
WAL checkpoints more aggressive again, reopens duplicate-write failures, or
removes the metrics DB path/cadence controls must re-prove the metrics-store
hot path with the owned store tests rather than assuming the earlier vacuum
fixes are sufficient.
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
`frontend-modern/src/components/Workloads/useWorkloadSelectionState.ts` must
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
page-local focus overlay. The shared same-path scheduler must also own cleanup
for every deferred scroll-restore timeout and animation frame it creates, so
route-state cleanup cannot leave hot-path replay work running after the owning
surface unmounts.
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
That same protected hot path keeps storage trend loading route-owned after the
dashboard overview retirement. `internal/api/router.go` must continue serving
the compact `/api/charts/storage-summary` request backed by
`GetStorageMetricsForChartBatch(...)` for the surfaces that still own storage
summary presentation, but no deleted dashboard trend hook may reopen the full
storage-page `/api/storage-charts` payload or an N+1 per-pool
`/api/metrics-store/history` fan-out.
That same Workloads shell boundary also owns empty-state action routing in
`frontend-modern/src/components/Workloads/WorkloadsStateCards.tsx`. When the
Workloads route has no connected infrastructure sources, the CTA must hand
operators directly to the canonical source picker through
`buildInfrastructureOnboardingPath('pick')` instead of bouncing through a generic
settings landing page. When canonical unified-resource source presence shows
that infrastructure exists but no workload inventory is available, the route
must not reuse the first-run no-source state; it must send operators to the
canonical infrastructure workspace through `buildInfrastructureWorkspacePath()`
so credential, permission, and collection status can be reviewed from the source
of truth.
That workload route now also treats readiness as a route-owned contract instead
of a raw websocket proxy signal: `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`
and `frontend-modern/src/components/Workloads/useWorkloadsState.ts` must keep
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
fleet-scale workload path.
That runtime is now intentionally split by concern:
`frontend-modern/src/components/Workloads/useWorkloadsState.ts` owns
top-level Workloads orchestration, workload loading, and composition across
the canonical Workloads owners, while
`frontend-modern/src/components/Workloads/useWorkloadsControlsState.ts`
owns persistent control defaults, keyboard-reset semantics, sort/search/tag
behavior, column visibility, and summary display preferences, while
`frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`
owns grouped workload derivation, summary fallbacks, parent-node mapping,
and grouped/windowed table math, while
`frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts`
owns grouped workload viewport synchronization and the scroll/resize listener
lifecycle, while
`frontend-modern/src/components/Workloads/useWorkloadGuestMetadataState.ts`
owns guest metadata cache persistence, optimistic custom-URL updates,
org-scope switching, and metadata refresh, and
`frontend-modern/src/components/Workloads/useWorkloadRouteState.ts`
owns workload-route synchronization, deep-link normalization, and route-scoped
filter contracts. Future workload hot-path changes must extend through those
owners within the Workloads surface, and new overview-route work must not
accrete back into `frontend-modern/src/components/Workloads/WorkloadsSurface.tsx`.
That same route-owned filter contract now also includes canonical workload
`platform` scoping for API-backed runtimes. Workloads URL-sync, filter-option
assembly, and workload drill-down routes must preserve
`platform=<owned-source-key>` for surfaces such as TrueNAS app-containers,
while treating host or cluster identity as secondary scope instead of
collapsing those routes back to generic agent-only semantics. The derived
workload hot path must also tolerate transient async route/data gaps by
treating missing filtered-workload collections as empty until the route and
resource snapshots converge, rather than crashing Workloads between sync
phases.
That same workload hot path also owns the split between canonical
app-container routing and Docker-only actions. `frontend-modern/src/hooks/useWorkloads.ts`,
`frontend-modern/src/components/Workloads/workloadTopology.ts`, and
`frontend-modern/src/components/Workloads/useGuestRowState.ts` must preserve a
canonical app-container navigation path while keeping Docker runtime action
identifiers explicit. Discovery affordances on the Workloads drawer must follow
the canonical `discoveryTarget` contract, not generic app-container host
fallbacks. TrueNAS app-containers may reuse runtime metadata such as image and
runtime strings, but they must not inherit Docker-only update affordances or
agent-only Discovery tabs on the Workloads row path unless the unified
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
The same protected table path treats the visible system column as
identity-first presentation over canonical merged-source data. Sort derivation
for that column must use the same displayed system identity as the render path,
while the render path may keep full merged-source detail in tooltips. When a
row contains both `agent` and a provider/API platform such as Proxmox, the table
must render the provider platform as the compact visible badge rather than
adding extra Agent badge width or sorting primarily by the telemetry method.
When a row is only known through an agent or container runtime, the table must
prefer reported OS/appliance identity before falling back to Docker/runtime
capability labels.
When the displayed system badge already includes the platform version, row title
metadata must not spend extra badge/title budget repeating that same platform as
an unversioned source; it may keep non-duplicate collection context such as
Pulse Agent.
That derived workload owner now also routes grouped row windowing through
`frontend-modern/src/components/Workloads/useGroupedTableWindowing.ts`, which
owns row-window thresholds, overscan behavior, reveal-index clamping, and
per-group visible-slice derivation. Future Workloads table windowing changes
must extend through that hook instead of rebuilding scroll math or mounted-row
budgets inline inside `frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`.
Viewport-driven grouped table synchronization now also routes through
`frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts`,
which owns the Workloads table body measurement and the scroll/resize listener
lifecycle. Future viewport sync changes must extend through that hook rather
than rebuilding browser-event wiring or table-body geometry reads inside
`frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts`.
The workload guest-row path now follows the same pattern: the render shell
stays in `frontend-modern/src/components/Workloads/GuestRow.tsx`, tooltip-backed
cell presentation lives in `frontend-modern/src/components/Workloads/GuestRowCells.tsx`,
and the canonical row contract and per-row hot-path derivations live in
`frontend-modern/src/components/Workloads/guestRowModel.tsx` and
`frontend-modern/src/components/Workloads/useGuestRowState.ts`. Future row
identity, column, cell-tooltip, anomaly-correlation, and link-state changes
must extend through those owners instead of rebuilding row-local state inside
the shell.
That per-row link state now also consumes the shared
`frontend-modern/src/routing/resourceLinks.ts` workload-to-infrastructure
helper instead of a workload-local routing shim. Future infrastructure-link
changes for workload rows must extend through the shared routing owner rather
than recreating feature-local path builders.
That shell now also routes tag-dot rendering through the shared
`frontend-modern/src/components/shared/TagBadges.tsx` primitive instead of
keeping a workload-local badge helper. Future guest-row tag presentation
changes must extend through that shared owner rather than reintroducing a
workload-only tag-badge variant.
The Workloads guest drawer now follows that same ownership rule: the shell
stays in `frontend-modern/src/components/Workloads/GuestDrawer.tsx`, the
overview card surface lives in
`frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx`, and
drawer-local normalization, backup/tag formatting, discovery identity wiring,
and workload-derived navigation state live in
`frontend-modern/src/components/Workloads/guestDrawerModel.ts` and
`frontend-modern/src/components/Workloads/useGuestDrawerState.ts`. Future
drawer runtime and overview-surface changes must extend through those owners
instead of adding more mixed state and helper drift back into the shell.
That drawer state now also consumes the same shared
`frontend-modern/src/routing/resourceLinks.ts` workload-to-infrastructure
helper, so row and drawer navigation stay aligned without a second
workload-local link-mapping file.
The shared infrastructure mapper hot path now stays intentionally narrow:
`frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
continues to own sensor-label presentation and hot-path host/agent projection,
while canonical drawer discovery-target derivation now lives in
`frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts`
under `unified-resources`. Future discovery-config or target-resolution
changes must extend through that unified-resource owner instead of
re-accumulating discovery heuristics back into the performance hot-path mapper.
The PBS service-table hot path now follows that same split: raw job arrays may
cross the transport boundary, but
`frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts`
must collapse them into one shared `Activity` presentation via the canonical
service-model helpers, while
`frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
stays a thin render shell. Future running-task visibility changes must extend
through the shared model and accepted performance proof file instead of adding
per-row PBS status parsing or inline job scans in the table render path.
The Workloads disk list now follows the same pattern: the shell stays in
`frontend-modern/src/components/Workloads/DiskList.tsx`, while disk-row
presentation derivations and fallback tooltip/runtime wiring live in
`frontend-modern/src/components/Workloads/diskListModel.ts` and
`frontend-modern/src/components/Workloads/useDiskListState.ts`. Future disk
usage math, threshold-color routing, and fallback handling must extend
through those owners instead of accreting back into the shell.
The Workloads filter now follows that same ownership rule: the shell
stays in `frontend-modern/src/components/Workloads/WorkloadsFilter.tsx`, which
composes the shared `FilterBar` chip primitive (with `savedViewsKey="workloads"`
so operators can save and recall named filter combos through the shared
`SavedViewsMenu`), while toolbar defaults,
active-filter counting, and reset semantics live in
`frontend-modern/src/components/Workloads/workloadsFilterModel.ts`. The legacy
`useWorkloadsFilterState.ts` hook retired with the FilterBar migration; its
`countActiveWorkloadsFilters` and `hasActiveWorkloadsFilters` helpers stay on
`workloadsFilterModel.ts`.
Workloads table-mode controls must also keep their accessible group name aligned
with the shared table presentation contract by using `Group by` for the
grouped/list selector instead of reintroducing local `Group By` casing or
platform-specific cluster wording. Dense workload toolbar variants must keep
that same row wrap-capable so optional runtime, chart, column, and reset
controls remain reachable on desktop instead of forcing a single no-wrap row
that clips trailing actions. The workload shell now routes table display
actions such as grouped/list mode and chart visibility through the shared
`FilterBar.viewOptionsTrailing` slot, leaving filter chips as primary toolbar
children so the display-action cluster wraps together with Columns and the
"+ Filter" menu across narrow desktop widths. Workload chart visibility is a display preference, not
an in-summary collapse affordance: the toolbar action must expose explicit
`Show charts` / `Hide charts` pressed state, and hiding charts must remove the
summary section rather than leaving an empty collapsed summary band on screen.
The Workloads-owned filter-config assembly now lives in
`frontend-modern/src/components/Workloads/useWorkloadsState.ts`, so future
filter runtime changes must extend through those owners instead of
reintroducing workload-local state, reset drift, or inline config assembly
into the shell.
Workload type option labels are part of that filter-config model ownership:
`WorkloadsFilter.tsx` must render the exported workload type option catalog
instead of embedding its own `System containers` / `App containers` wording in
the shell.
The dashboard threshold slider now follows that same pattern: the shell stays
in `frontend-modern/src/components/Workloads/ThresholdSlider.tsx`, while
metric-type text and fill presentation live in
`frontend-modern/src/utils/thresholdSliderPresentation.ts`, while
slider bounds math, thumb-position transforms, and title/label derivations
live in `frontend-modern/src/components/Workloads/thresholdSliderModel.ts`
and drag scroll-lock lifecycle lives in
`frontend-modern/src/components/Workloads/useThresholdSliderState.ts`.
Future slider runtime changes must extend through those owners instead of
reintroducing mixed drag state, type-color formatting, and presentation logic
into the shell.
The dashboard stacked disk bar now follows that same pattern: the shell stays
in `frontend-modern/src/components/Workloads/StackedDiskBar.tsx`, while
disk-capacity math, segment and tooltip derivation, max-disk labeling, and
mode-specific presentation live in
`frontend-modern/src/components/Workloads/stackedDiskBarModel.ts` and
resize-observer plus tooltip lifecycle live in
`frontend-modern/src/components/Workloads/useStackedDiskBarState.ts`.
Future disk-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state and presentation branching into the shell.
The dashboard stacked memory bar now follows that same pattern: the shell
stays in `frontend-modern/src/components/Workloads/StackedMemoryBar.tsx`,
while memory-capacity math, balloon/swap tooltip derivation, anomaly label
presentation, and sublabel-fit logic live in
`frontend-modern/src/components/Workloads/stackedMemoryBarModel.ts` and
resize-observer plus tooltip lifecycle live in
`frontend-modern/src/components/Workloads/useStackedMemoryBarState.ts`.
Future memory-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state, balloon branching, and tooltip shaping into
the shell.
The dashboard metric bar now follows that same pattern: the shell stays in
`frontend-modern/src/components/Workloads/MetricBar.tsx`, while width,
show-label, sublabel-fit, and threshold-color derivation live in
`frontend-modern/src/components/Workloads/metricBarModel.ts` and
resize-observer lifecycle lives in
`frontend-modern/src/components/Workloads/useMetricBarState.ts`. Future
metric-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state and label-fit logic into the shell.
The dashboard enhanced CPU bar now follows that same pattern: the shell stays
in `frontend-modern/src/components/Workloads/EnhancedCPUBar.tsx`, while usage
formatting, anomaly presentation, tooltip load-average formatting, and
threshold-driven label state live in
`frontend-modern/src/components/Workloads/enhancedCpuBarModel.ts` and
tooltip lifecycle lives in
`frontend-modern/src/components/Workloads/useEnhancedCPUBarState.ts`. Future
CPU-bar runtime changes must extend through those owners instead of
reintroducing mixed tooltip state and formatting logic into the shell.

The unified resource table hot path is now also governed as explicit
performance-owned runtime, with shared ownership against the unified-resource
consumer boundary. The remaining performance work is no longer top-level
ownership ambiguity on the main Infrastructure or Workloads tables.
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
That same hot-path boundary now also owns CSP-safe table sizing. Infrastructure
host, PBS, and PMG table shells must take their layout and column sizing from
the shared presentation owner in
`frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
and apply those values through classes and width/height attributes, not inline
`style=` maps on the live table DOM. Future hot-path table work must not
reintroduce inline width, min/max width, or row-height styles into the render
shell just to land a local layout tweak.
That same shared sizing contract now also owns wide-desktop width
distribution. In full-width shells, host, PBS, and PMG tables must keep an
explicit desktop `Resource` column width in the shared presentation owner so
surplus width is redistributed across peer metric, source, uptime, and action
columns instead of being dumped into the first column and wasting operator
visible table density.
That hot-path contract now includes policy badge rendering on resource rows.
Policy-rich table rows must only surface a single inline summary chip for
blocking `local-only`/`restricted` posture; non-blocking `sensitive` +
`local-first` and redaction-only metadata belongs in Data Handling, detail, and
AI/governance surfaces instead of spending default row visual budget.
Agentless availability evidence belongs on that same bounded row path.
Infrastructure `network-endpoint` rows may replace otherwise empty host metric
slots with one compact inline target/result readout, while the System column
owns the protocol identity badge (`ICMP`, `TCP`, or `HTTP`). Recent check
timing, latency history, and fuller failure context may stay in bounded
tooltip or drawer detail, but the table must not duplicate the same probe
protocol and result text across the resource identity cell and metric cells.
That text must derive from the existing resource payload and shared
presentation helper instead of adding a per-row fetch, extra hydration pass, or
unbounded badge stack.
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
It now also includes compact resource-facet summary chips rendered next to
policy metadata, and table-row uses must set an explicit visible chip limit
with overflow disclosure instead of allowing mock-rich resource rows to wrap
an unbounded badge list. Those chips must stay within the same bounded
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
The same infrastructure hot path now also depends on the shared
`frontend-modern/src/components/shared/ProgressBar.tsx` primitive for metric
fill rendering. Performance-sensitive metric bars may vary by value and color,
but they must render width through shared attribute-driven progress geometry
instead of per-row inline width styles that break the hosted demo CSP.
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
Relationship-map detail now stays on that same drawer-only path: canonical
relationships come from the bundled resource facet payload when a resource is
opened, not from additional table-row hydration or a second all-resources pass.
The detail drawer now follows the same default posture by collapsing its
history overview down to timeline counts and timeline-summary chips, so the
performance-sensitive shared presentation path stays aligned with the
investigation-first product contract instead of rendering low-signal generic
facet sections by default.
Governance metadata such as sensitivity and routing scope may be visible in
the table, but it must remain on the same bounded row-windowing and mounted-row
budget proved by `UnifiedResourceTable.performance.contract.test.tsx` rather
than creating a separate unbounded rendering path for policy-rich fleets.
Infrastructure cluster-header presentation must stay inside that same bounded
row model. `unifiedResourceTableStateModel.ts` owns whether the compact
`Cluster` chip is rendered, including suppressing it when the visible group
name already ends in `Cluster`, so label cleanup does not add a second
row-rendering path or per-page DOM workaround.
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
That same workloads-link path and the workload projection now also
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
Those budgets also assume Kubernetes pod history lookups hit one canonical
series key. Pod chart and history consumers must normalize bare pod IDs onto
`k8s:<cluster>:pod:<uid>` before lookup; otherwise demo and mock workloads pay
repeated empty-store misses and redundant refetch churn even while the shared
workload charts already hold the same pod metrics in memory.
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
That same ownership includes local SQLite artifact permissions. The metrics
store must create or re-harden the selected database directory as owner-only
and must reject symlink or non-regular database targets before opening SQLite;
the `.db`, `.db-wal`, and `.db-shm` artifacts must be chmodded owner-only even
under a permissive process umask.
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
consumption. `frontend-modern/src/components/Workloads/useWorkloadsState.ts`
may read connection and alert state only through
`frontend-modern/src/contexts/appRuntime.ts`; it must not import `@/App` or
create a reverse dependency into the root shell just to read websocket state.
The same hot-path discipline now applies to public-demo commercial reads.
Shared shells may consume the small `/api/license/runtime-capabilities`
contract for feature truth, but commercial demo routes stay hidden and the
browser must not keep retrying `/api/license/commercial-posture`,
`/api/license/entitlements`, `/auth/license-purchase-start`,
`/api/upgrade-metrics/*`, `/api/admin/upgrade-metrics-funnel`, or other hidden
commercial endpoints from performance-sensitive settings or route shells. The
retired local commercial analytics routes must not become route-shell polling
fallbacks or background bootstrap work after their normal product API removal.
Paid-runtime block records are allowed on that same small
runtime-capabilities response, but route shells must treat them as already
loaded runtime identity facts. They must not start polling billing
entitlements, checkout, or commercial posture endpoints just to decide whether
a community runtime should show private Pulse Pro download guidance.
Workloads and infrastructure summary consumers now also keep null-tolerant read
models on the shared hot path. Guest rows, stacked bars, anomaly summaries, and
resource detail mappers may accept partial platform metadata or undefined
ratios, but they must normalize those values once in the shared model layer
instead of scattering non-null assertions or per-component count coercion
through the Workloads runtime.
That same workload-table hot path now also owns a single width contract.
`frontend-modern/src/components/Workloads/WorkloadsTable.tsx`,
`WorkloadTableHeader.tsx`, `GuestRow.tsx`, and `guestRowModel.tsx` must remain
the canonical owners for desktop and mobile workload column sizing. Global CSS
must not reintroduce competing `.workload-table [data-workload-col=…]` width
rules or `min-width: max-content` fallbacks that can blow the table out
horizontally on Firefox or other desktop browsers.
The same `internal/api/router.go` payload boundary also keeps the
will_fix_later remind-at deadline scoped to a single optional pointer
(`*time.Time`) per finding on both API write paths, so adding the
operational-commitment field does not regress the unified-findings hot
path with a per-row allocation when the dismissal reason is anything
other than `will_fix_later`. The newer `AutoResolved` attribution flag
on the same UnifiedFinding shape stays a fixed-size `bool` in the same
struct, so the operator-vs-Pulse closure attribution adds no per-row
allocation pressure on the same hot path.
