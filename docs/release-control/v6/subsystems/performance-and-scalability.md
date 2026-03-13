# Performance And Scalability Contract

## Contract Metadata

```json
{
  "subsystem_id": "performance-and-scalability",
  "lane": "L10",
  "contract_file": "docs/release-control/v6/subsystems/performance-and-scalability.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
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
The aggregate `/api/charts/workloads-summary` endpoint now also has its own
explicit API p95 budget constant, aligned with the per-workload charts budget,
and `internal/api/slo_bench_test.go` must fail if that aggregate budget or its
store-backed mixed-workload benchmark coverage drifts.

Infrastructure selector status ordering must now tolerate arbitrary filter-set
strings without widening the canonical hot-path order tuple. Unknown statuses
must sort after the governed status order instead of forcing the selector path
to abandon the typed canonical order used by the infrastructure table and its
performance proof surface.
