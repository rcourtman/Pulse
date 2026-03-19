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
7. `frontend-modern/src/components/Dashboard/workloadSelectors.ts`
8. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
9. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts`
10. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`
11. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
12. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `unified-resources`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `unified-resources`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `unified-resources`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
4. `internal/api/slo.go` shared with `api-contracts`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add performance budgets through SLO or contract tests
2. Add query-plan guardrails for DB-backed hot paths
3. Optimize hot paths only when backed by benchmarks or proven query issues
4. Extend dashboard hot-path selectors through `frontend-modern/src/components/Dashboard/workloadSelectors.ts` rather than duplicating filtering/grouping logic in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
5. Normalize dashboard workload view-mode aliases through `frontend-modern/src/utils/workloads.ts` instead of keeping local URL/storage parsing in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
6. Deduplicate dashboard workload rows by canonical workload ID from `frontend-modern/src/utils/workloads.ts` rather than via local pass-through wrappers in `frontend-modern/src/components/Dashboard/Dashboard.tsx`
7. Render dashboard row identity directly from the shared canonical workload helper so row selection, hover, and fallback metadata lookup stay aligned with the same workload contract
8. Format infrastructure sensor labels through the shared `frontend-modern/src/utils/textPresentation.ts` presentation helper instead of maintaining a local title-casing implementation in `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`

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

The unified resource table hot path is now also governed as explicit
performance-owned runtime, with shared ownership against the unified-resource
consumer boundary. The remaining performance work is no longer top-level
ownership ambiguity on the main dashboard or infrastructure tables.
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
share the canonical Kubernetes context prefix helper, so route labels and pod
grouping keep using the same cluster-context source of truth instead of
rebuilding the `clusterName`/`context`/`clusterId` prefix locally.
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
