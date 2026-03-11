# Performance And Scalability Contract

## Purpose

Own measurable performance budgets, query-plan guarantees, and hot-path
regression protection.

## Canonical Files

1. `pkg/metrics/store.go`
2. `pkg/metrics/store_query_plan_test.go`
3. `pkg/metrics/store_slo_test.go`
4. `internal/api/slo.go`
5. `internal/api/slo_bench_test.go`
6. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`

## Extension Points

1. Add performance budgets through SLO or contract tests
2. Add query-plan guardrails for DB-backed hot paths
3. Optimize hot paths only when backed by benchmarks or proven query issues

## Forbidden Paths

1. Speculative micro-optimizations without evidence
2. New N+1 data loading paths on dashboard/resource views
3. Hot-path query changes without updating plan or SLO guardrails

## Completion Obligations

1. Update benchmarks, SLOs, or query-plan tests when hot-path behavior changes
2. Update this contract when a new protected hot path is adopted
3. Record the evidence source for any claimed performance improvement

## Current State

This lane already has strong evidence and guardrails, but it still trails on
score because critical hot paths need more complete protection and verification.
