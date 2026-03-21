# Performance And Scalability Contract

## Contract Metadata

```json
{
  "subsystem_id": "performance-and-scalability",
  "lane": "L10",
  "contract_file": "docs/release-control/v6/internal/subsystems/performance-and-scalability.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
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
7. `frontend-modern/src/components/Dashboard/useDashboardState.ts`
8. `frontend-modern/src/components/Dashboard/DashboardFilter.tsx`
9. `frontend-modern/src/components/Dashboard/dashboardFilterModel.ts`
10. `frontend-modern/src/components/Dashboard/useDashboardFilterState.ts`
11. `frontend-modern/src/components/Dashboard/ThresholdSlider.tsx`
12. `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts`
13. `frontend-modern/src/components/Dashboard/useThresholdSliderState.ts`
14. `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`
15. `frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts`
16. `frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts`
17. `frontend-modern/src/components/Dashboard/DiskList.tsx`
18. `frontend-modern/src/components/Dashboard/diskListModel.ts`
19. `frontend-modern/src/components/Dashboard/useDiskListState.ts`
20. `frontend-modern/src/components/Dashboard/GuestRow.tsx`
21. `frontend-modern/src/components/Dashboard/guestRowModel.tsx`
22. `frontend-modern/src/components/Dashboard/useGuestRowState.ts`
23. `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`
24. `frontend-modern/src/components/Dashboard/guestDrawerModel.ts`
25. `frontend-modern/src/components/Dashboard/useGuestDrawerState.ts`
26. `frontend-modern/src/components/Dashboard/workloadSelectors.ts`
27. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
28. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
29. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts`
30. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
31. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
32. `frontend-modern/src/components/Dashboard/__tests__/DashboardFilter.test.tsx`
33. `frontend-modern/src/components/Dashboard/__tests__/useDashboardFilterState.test.ts`
34. `frontend-modern/src/components/Dashboard/ThresholdSlider.test.tsx`
35. `frontend-modern/src/components/Dashboard/__tests__/useThresholdSliderState.test.ts`
36. `frontend-modern/src/components/Dashboard/__tests__/StackedDiskBar.test.tsx`
37. `frontend-modern/src/components/Dashboard/__tests__/useStackedDiskBarState.test.tsx`
38. `frontend-modern/src/components/Dashboard/__tests__/DiskList.test.tsx`
39. `frontend-modern/src/components/Dashboard/__tests__/GuestRow.test.tsx`
40. `frontend-modern/src/components/Dashboard/GuestDrawer.test.tsx`
41. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `unified-resources`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `unified-resources`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `unified-resources`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
4. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts` shared with `unified-resources`: unified resource table state, grouping, and windowing are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
5. `internal/api/slo.go` shared with `api-contracts`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add performance budgets through SLO or contract tests
2. Add query-plan guardrails for DB-backed hot paths
3. Optimize hot paths only when backed by benchmarks or proven query issues
4. Extend dashboard hot-path selectors through `frontend-modern/src/components/Dashboard/workloadSelectors.ts` rather than duplicating filtering/grouping logic in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
5. Normalize dashboard workload view-mode aliases through `frontend-modern/src/utils/workloads.ts` instead of keeping local URL/storage parsing in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
6. Deduplicate dashboard workload rows by canonical workload ID from `frontend-modern/src/utils/workloads.ts` rather than via local pass-through wrappers in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
7. Render dashboard row identity directly from the shared canonical workload helper so row selection, hover, and fallback metadata lookup stay aligned with the same workload contract
8. Format infrastructure sensor labels through the shared `frontend-modern/src/utils/textPresentation.ts` presentation helper instead of maintaining a local title-casing implementation in `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
9. Extend dashboard row contract and per-row hot-path derivations through `frontend-modern/src/components/Dashboard/guestRowModel.tsx` and `frontend-modern/src/components/Dashboard/useGuestRowState.ts` rather than rebuilding column metadata, row identity, or anomaly correlation inside `frontend-modern/src/components/Dashboard/GuestRow.tsx`
10. Extend dashboard drawer derivations and runtime wiring through `frontend-modern/src/components/Dashboard/guestDrawerModel.ts` and `frontend-modern/src/components/Dashboard/useGuestDrawerState.ts` rather than rebuilding canonical guest identity, discovery routing, or drawer-local normalization inside `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`
11. Extend dashboard disk-list derivations and fallback runtime wiring through `frontend-modern/src/components/Dashboard/diskListModel.ts` and `frontend-modern/src/components/Dashboard/useDiskListState.ts` rather than rebuilding usage math, progress-state mapping, or tooltip fallback logic inside `frontend-modern/src/components/Dashboard/DiskList.tsx`
12. Extend dashboard filter defaults, active-filter counting, reset semantics, and mobile toolbar state through `frontend-modern/src/components/Dashboard/dashboardFilterModel.ts` and `frontend-modern/src/components/Dashboard/useDashboardFilterState.ts`, and keep dashboard-owned filter-config assembly in `frontend-modern/src/components/Dashboard/useDashboardState.ts`, rather than rebuilding filter-local state inside `frontend-modern/src/components/Dashboard/DashboardFilter.tsx` or inline config IIFEs in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
13. Extend threshold-slider value-position math, title/label derivation, and drag scroll-lock runtime through `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts` and `frontend-modern/src/components/Dashboard/useThresholdSliderState.ts` rather than rebuilding slider-local state and pointer lifecycle inside `frontend-modern/src/components/Dashboard/ThresholdSlider.tsx`
14. Extend stacked disk-bar capacity math, segment/tooltip derivation, and resize-observer runtime through `frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts` and `frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts` rather than rebuilding disk-bar-local state, mode branching, and tooltip shaping inside `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`

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

All governed performance-owned runtime files now require explicit registry
path-policy coverage, so new protected hot paths must be mapped to a concrete
proof route instead of falling back to subsystem-default verification.

The dashboard workload selector path and the dashboard runtime that consumes it
are now part of the protected performance surface rather than proof-only
context. Future hot-path filter/group/sort/windowing changes must route through
the explicit dashboard performance proof policy in the subsystem registry.
That runtime state owner now lives in
`frontend-modern/src/components/Dashboard/useDashboardState.ts`, so guest
metadata persistence, workload-route synchronization, grouping/windowing, and
filter/sort state must extend through that owner instead of accreting back into
`frontend-modern/src/components/Dashboard/Dashboard.tsx`.
The dashboard guest-row path now follows the same pattern: the render shell
stays in `frontend-modern/src/components/Dashboard/GuestRow.tsx`, while the
canonical row contract and per-row hot-path derivations live in
`frontend-modern/src/components/Dashboard/guestRowModel.tsx` and
`frontend-modern/src/components/Dashboard/useGuestRowState.ts`. Future row
identity, column, anomaly-correlation, and link-state changes must extend
through those owners instead of rebuilding row-local state inside the shell.
The dashboard guest drawer now follows that same ownership rule: the shell
stays in `frontend-modern/src/components/Dashboard/GuestDrawer.tsx`, while
drawer-local normalization, backup/tag formatting, discovery identity wiring,
and workload-derived navigation state live in
`frontend-modern/src/components/Dashboard/guestDrawerModel.ts` and
`frontend-modern/src/components/Dashboard/useGuestDrawerState.ts`. Future
drawer runtime changes must extend through those owners instead of adding
more mixed state and helper drift back into the shell.
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
slider bounds math, thumb-position transforms, and title/label derivations
live in `frontend-modern/src/components/Dashboard/thresholdSliderModel.ts`
and drag scroll-lock lifecycle lives in
`frontend-modern/src/components/Dashboard/useThresholdSliderState.ts`.
Future slider runtime changes must extend through those owners instead of
reintroducing mixed drag state and formatting logic into the shell.
The dashboard stacked disk bar now follows that same pattern: the shell stays
in `frontend-modern/src/components/Dashboard/StackedDiskBar.tsx`, while
disk-capacity math, segment and tooltip derivation, max-disk labeling, and
mode-specific presentation live in
`frontend-modern/src/components/Dashboard/stackedDiskBarModel.ts` and
resize-observer plus tooltip lifecycle live in
`frontend-modern/src/components/Dashboard/useStackedDiskBarState.ts`.
Future disk-bar runtime changes must extend through those owners instead of
reintroducing mixed resize state and presentation branching into the shell.

The unified resource table hot path is now also governed as explicit
performance-owned runtime, with shared ownership against the unified-resource
consumer boundary. The remaining performance work is no longer top-level
ownership ambiguity on the main dashboard or infrastructure tables.
The table's sort, grouping, row-windowing, and viewport-sync owner now lives
in `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`,
so future hot-path table-state changes must route through that state owner
instead of rebuilding selector and scroll coordination inside the render shell.
That hot-path contract now includes policy badge rendering on resource rows.
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
