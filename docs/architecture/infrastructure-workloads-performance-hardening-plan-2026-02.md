# Infrastructure + Workloads Performance Hardening Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/infrastructure-workloads-performance-hardening-progress-2026-02.md`

## Product Intent

Infrastructure and Workloads must stay responsive at realistic fleet sizes, not just small mock datasets.

This lane hardens performance for:
1. initial page responsiveness,
2. filter/search/sort interaction latency,
3. sustained polling updates under load,
4. regression prevention through explicit performance contracts.

## Scope and Boundaries

In scope:
- `frontend-modern/src/pages/Infrastructure.tsx`
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
- `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx`
- `frontend-modern/src/components/Dashboard/GuestRow.tsx` (only if required for render containment)
- `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
- `frontend-modern/src/hooks/useUnifiedResources.ts`
- `frontend-modern/src/hooks/useV2Workloads.ts`
- Targeted tests and new perf contract tests under `frontend-modern/src/**/__tests__`

Out of scope (this lane):
- Storage/Backups SB5 packets and files
- Settings decomposition/stabilization files
- Backend historical metrics-store query optimization
- Websocket payload-shape removals unrelated to Infrastructure/Workloads rendering

Parallel worker guardrail:
- This lane is safe to run in parallel with SB5 because it does not require edits to storage/backups packet-owned files.

## Non-Negotiable Contracts

1. Correctness contract:
- No regression in displayed counts, grouping semantics, filters, sort order, or route/query sync behavior.

2. UX contract:
- No visible UI jank from polling updates at medium/large data volumes.

3. Scale contract:
- Design for real fleets larger than current mock mode. Optimization decisions must hold at 3k+ resources and 5k+ workloads.

4. Test contract:
- Every optimization packet ships with regression tests and explicit perf contract evidence.

5. Architecture contract:
- Prefer deterministic, pure derivation utilities + memoized selectors over ad hoc in-component recomputation.

6. Safety contract:
- Do not broaden packet scope into unrelated pages/components.

7. Rollback contract:
- Every packet has file-level rollback guidance and checkpoint commit hash in tracker evidence.

## Code-Derived Baseline (Current)

### A. Large reactive surfaces and repeated transforms

1. Infrastructure page builds filtered arrays in-component and passes them to multiple heavy consumers:
- `frontend-modern/src/pages/Infrastructure.tsx:328`
- `frontend-modern/src/pages/Infrastructure.tsx:422`
- `frontend-modern/src/pages/Infrastructure.tsx:560`

2. Infrastructure table performs multiple full-list passes (host/service split, sort, group, IO distribution) and renders all rows:
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx:290`
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx:297`
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx:319`
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx:351`
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx:490`

3. Workloads dashboard has high transform density and repeated filtering/counting over full guest sets:
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:986`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1149`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1186`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1198`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1325`

4. Workloads table renders full grouped rows; there is no row windowing/virtualization layer:
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1556`
- `frontend-modern/src/components/Dashboard/Dashboard.tsx:1592`

### B. Existing strengths to preserve

1. Unified resources hook already has pagination caps, shared fetch dedupe, cache age checks, websocket burst coalescing:
- `frontend-modern/src/hooks/useUnifiedResources.ts`
- `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts`

2. V2 workloads hook already has shared fetch, cache, polling without Suspense thrash:
- `frontend-modern/src/hooks/useV2Workloads.ts`
- `frontend-modern/src/hooks/__tests__/useV2Workloads.test.ts`

3. Summary charts already include cache + adaptive point limits:
- `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
- `frontend-modern/src/components/Workloads/WorkloadsSummary.test.tsx`
- `frontend-modern/src/utils/infrastructureSummaryCache.ts`

### C. Complexity baseline

- `frontend-modern/src/components/Dashboard/Dashboard.tsx`: 1715 LOC
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`: 1281 LOC
- `frontend-modern/src/components/Dashboard/GuestRow.tsx`: 1138 LOC
- Combined key surface: 7296 LOC across main infra/workloads render paths

## Performance Budget Matrix (Target)

Define three synthetic fleet profiles for perf contracts:

1. Profile S (small):
- 250 infrastructure resources
- 400 workloads

2. Profile M (medium):
- 1000 infrastructure resources
- 1500 workloads

3. Profile L (large):
- 3000 infrastructure resources
- 5000 workloads

Required budget targets after packet completion:

1. Derivation budget (pure selector transforms):
- Infrastructure filter/sort/group recompute P95:
  - Profile M <= 120ms
  - Profile L <= 250ms
- Workloads filter/sort/group recompute P95:
  - Profile M <= 150ms
  - Profile L <= 300ms

2. Render budget (table row mount pressure):
- For datasets > 500 rows, initial render and filter transitions must cap mounted rows to a viewport-windowed slice (target <= 140 mounted rows).

3. Poll update budget:
- Polling ticks with unchanged selection/filter state should avoid full table remount behavior.
- Contracted by stable identity tests and row-render-count assertions in new perf tests.

4. Summary chart budget:
- Maintain adaptive point limits for large workload counts and avoid fetching/rendering non-visible series.

Note:
- CI timing can be noisy; all perf contracts must include both timing envelopes and deterministic structural invariants (row-count/windowing/reference stability).

## Risk Register

| ID | Severity | Risk | Mitigation Packet |
|---|---|---|---|
| IWP-001 | High | Windowing changes break expand/collapse or keyboard/scroll behavior. | IWP-03, IWP-04 |
| IWP-002 | High | Filter/sort semantics regress during selector extraction. | IWP-01, IWP-02 |
| IWP-003 | Medium | Perf tests become flaky and block delivery. | IWP-00, IWP-07 |
| IWP-004 | Medium | Over-throttled polling causes stale perceived data. | IWP-05 |
| IWP-005 | Medium | Summary chart optimization hides important visible-series data. | IWP-06 |

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: delegated coding agent
- Reviewer: orchestrator

A packet can be marked DONE only when:
- all packet checkboxes are checked,
- required commands run with explicit exit codes,
- reviewer gate checklist passes,
- verdict is `APPROVED`.

## Required Review Output (Every Packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit 0
2. `<command>` -> exit 0

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<short-hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Global Validation Baseline

Run after every packet unless explicitly waived:

1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run src/hooks/__tests__/useUnifiedResources.test.ts src/hooks/__tests__/useV2Workloads.test.ts src/components/Workloads/WorkloadsSummary.test.tsx src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx src/components/Infrastructure/InfrastructureSummary.test.tsx`

Milestone boundary suites (run at end of IWP-03, IWP-06, IWP-07):

3. `cd frontend-modern && npx vitest run`

## Execution Packets

### IWP-00: Baseline + Perf Contract Scaffold (Docs + Tests Scaffold)

Objective:
- Establish deterministic perf harness strategy and fixture profiles before behavior changes.

Scope:
- `docs/architecture/infrastructure-workloads-performance-hardening-plan-2026-02.md`
- `docs/architecture/infrastructure-workloads-performance-hardening-progress-2026-02.md`
- `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx` (new scaffold)
- `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx` (new scaffold)

Implementation checklist:
1. Add fixture-profile matrix and explicit pass/fail budgets.
2. Add perf contract scaffolds with deterministic assertions placeholders (no flaky hard timing yet).
3. Record baseline behavior expectations for row counts and transform paths.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Harness exists and is ready to absorb packet-by-packet assertions.

### IWP-01: Infrastructure Derivation Pipeline Extraction and Single-Pass Optimization

Objective:
- Remove repeated in-component full-list work in Infrastructure path while preserving behavior.

Scope (max 5 files):
1. `frontend-modern/src/pages/Infrastructure.tsx`
2. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
3. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` (new)
4. `frontend-modern/src/components/Infrastructure/__tests__/infrastructureSelectors.test.ts` (new)
5. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`

Implementation checklist:
1. Extract filter/sort/group derivations into pure selector helpers.
2. Consolidate redundant array passes where possible.
3. Keep existing filter/search semantics identical.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/infrastructureSelectors.test.ts src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Infrastructure selectors are covered with semantic + perf contract tests.

### IWP-02: Workloads Derivation Pipeline Extraction and Filter Cost Reduction

Objective:
- Reduce repeated full-list passes in Dashboard filtering/grouping/stat computation.

Scope (max 5 files):
1. `frontend-modern/src/components/Dashboard/Dashboard.tsx`
2. `frontend-modern/src/components/Dashboard/workloadSelectors.ts` (new)
3. `frontend-modern/src/components/Dashboard/__tests__/workloadSelectors.test.ts` (new)
4. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
5. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx`

Implementation checklist:
1. Extract filter/search/sort/group/stat derivations into testable selectors.
2. Ensure single-pass aggregation where feasible.
3. Preserve all mode semantics (`viewMode`, `statusMode`, node/context filters).

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/workloadSelectors.test.ts src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Workloads derivations are deterministic, tested, and regression-safe.

### IWP-03: Infrastructure Table Windowing and Render Containment

Objective:
- Cap mounted infrastructure row count for medium/large datasets.

Scope (max 4 files):
1. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
2. `frontend-modern/src/components/Infrastructure/useTableWindowing.ts` (new)
3. `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`
4. `frontend-modern/src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx` (update only if routing/interaction assertions needed)

Implementation checklist:
1. Add deterministic row-windowing for host table rendering.
2. Preserve grouping headers, expansion behavior, and deep-link reveal behavior.
3. Add row-mount-count assertions in perf contract test.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Infrastructure table obeys row window budget and behavior contracts.

### IWP-04: Workloads Table Windowing and Grouped Render Containment

Objective:
- Cap mounted workload rows and avoid full group render pressure.

Scope (max 5 files):
1. `frontend-modern/src/components/Dashboard/Dashboard.tsx`
2. `frontend-modern/src/components/Dashboard/useGroupedTableWindowing.ts` (new)
3. `frontend-modern/src/components/Dashboard/GuestRow.tsx` (only if required for stable keys/containment hooks)
4. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
5. `frontend-modern/src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx`

Implementation checklist:
1. Apply group-aware row windowing (flat + grouped modes).
2. Preserve selection, drawer open/close, hover linking, and node group headers.
3. Add structural assertions for mounted-row ceilings.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Workloads table render cost is bounded for large datasets.

### IWP-05: Polling/Update Backpressure and Recompute Isolation

Objective:
- Prevent poll/update cadence from triggering avoidable heavy recomputation.

Scope (max 5 files):
1. `frontend-modern/src/hooks/useV2Workloads.ts`
2. `frontend-modern/src/hooks/useUnifiedResources.ts`
3. `frontend-modern/src/components/Dashboard/Dashboard.tsx`
4. `frontend-modern/src/hooks/__tests__/useV2Workloads.test.ts`
5. `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts`

Implementation checklist:
1. Verify/refine polling update cadence under heavy datasets.
2. Ensure unchanged payload updates avoid unnecessary downstream churn.
3. Harden tests around cache/poll semantics and update coalescing.

Required tests:
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useV2Workloads.test.ts src/hooks/__tests__/useUnifiedResources.test.ts src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Polling behavior remains fresh without causing broad rerender churn.

### IWP-06: Summary Path Hardening (InfrastructureSummary + WorkloadsSummary)

Objective:
- Keep summary cards performant and bounded under large visible sets.

Scope (max 5 files):
1. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
2. `frontend-modern/src/components/Workloads/WorkloadsSummary.tsx`
3. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.test.tsx`
4. `frontend-modern/src/components/Workloads/WorkloadsSummary.test.tsx`
5. `frontend-modern/src/utils/__tests__/infrastructureSummaryCache.test.ts`

Implementation checklist:
1. Validate fetch/cache cadence under large profiles.
2. Preserve visible-series-only behavior and adaptive chart point limits.
3. Add/strengthen perf-oriented contract tests.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Infrastructure/InfrastructureSummary.test.tsx src/components/Workloads/WorkloadsSummary.test.tsx src/utils/__tests__/infrastructureSummaryCache.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Summary surfaces remain bounded and regression-tested.

### IWP-07: Final Performance Certification

Objective:
- Certify all budgets and close this lane with documented evidence.

Scope:
- `docs/architecture/infrastructure-workloads-performance-hardening-progress-2026-02.md`
- Any packet-created perf contract tests (assertion tightening only)

Implementation checklist:
1. Verify IWP-00 through IWP-06 are DONE/APPROVED with checkpoint hashes.
2. Run full frontend test suite and typecheck.
3. Record final P0/P1/P2 verdict and residual risk register.

Required tests:
1. `cd frontend-modern && npx vitest run`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Lane certified with explicit evidence and rollback traceability.

## Explicitly Deferred Beyond This Lane

1. Backend API payload minimization for `/api/v2/resources` and charts endpoints.
2. Websocket protocol redesign and payload field retirement.
3. Native browser-e2e perf lab automation (Playwright/Lighthouse) if introduced later.
