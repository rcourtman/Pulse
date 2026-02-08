# Infrastructure + Workloads Performance Hardening Progress Tracker

Linked plan:
- `docs/architecture/infrastructure-workloads-performance-hardening-plan-2026-02.md` (authoritative execution spec)
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` (parallel lane - non-overlapping)

Status: Active
Date: 2026-02-08

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Reviewer must provide explicit command exit-code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or replaced by summary-only claims.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. Update this file first in each implementation session and last before session end.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before starting the next packet.
7. Do not use `git checkout --`, `git restore --source`, `git reset --hard`, or `git clean -fd` on shared worktrees.
8. Respect packet subsystem boundaries; do not expand packet scope to adjacent streams.
9. Keep this lane isolated from SB5 storage/backups packet-owned files.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| IWP-00 | Baseline + Perf Contract Scaffold | DONE | Codex | Claude | APPROVED | IWP-00 Review Evidence |
| IWP-01 | Infrastructure Derivation Pipeline Extraction | DONE | Codex | Claude | APPROVED | IWP-01 Review Evidence |
| IWP-02 | Workloads Derivation Pipeline Extraction | DONE | Codex | Claude | APPROVED | IWP-02 Review Evidence |
| IWP-03 | Infrastructure Table Windowing and Render Containment | DONE | Codex | Claude | APPROVED | IWP-03 Review Evidence |
| IWP-04 | Workloads Table Windowing and Grouped Render Containment | DONE | Codex | Claude | APPROVED | IWP-04 Review Evidence |
| IWP-05 | Polling/Update Backpressure and Recompute Isolation | DONE | Codex | Claude | APPROVED | IWP-05 Review Evidence |
| IWP-06 | Summary Path Hardening | TODO | Codex | Claude | — | — |
| IWP-07 | Final Performance Certification | TODO | Claude | Claude | — | — |

---

## IWP-00 Checklist: Baseline + Perf Contract Scaffold

- [x] Fixture profile matrix documented in plan and synchronized to this tracker.
- [x] Perf contract test scaffold added for Infrastructure table.
- [x] Perf contract test scaffold added for Workloads dashboard.
- [x] Baseline expectations documented (row ceilings, transform budgets placeholders).

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-00 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx` (new): Perf contract scaffold with S/M/L fixture profiles (250/1000/3000 resources), baseline row-count invariants, grouped vs flat mode contracts, transform budget placeholder.
- `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx` (new): Perf contract scaffold with S/M/L fixture profiles (400/1500/5000 workloads), baseline row-count invariants, filter mode contracts (all/vm/lxc/docker), transform and windowing budget placeholders (IWP-02, IWP-04).

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx` -> exit 0 (15 tests passed, 2 files, 4.77s)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. Global validation baseline (6 files, 36 tests) -> exit 0

Gate checklist:
- P0: PASS (all tests pass, typecheck clean, no behavior changes)
- P1: PASS (fixtures are deterministic with stable type distribution, no flaky timing thresholds)
- P2: PASS (follows existing test patterns, mocks match Dashboard.k8s.test.tsx and UnifiedResourceTable.workloads-link.test.tsx)

Verdict: APPROVED

Commit:
- `c28f1c4a` (perf(IWP-00): scaffold baseline performance contract tests for Infrastructure and Workloads)

Residual risk:
- Profile L tests use extended timeout (15–20s) for 3000/5000 element renders; may need adjustment on slow CI runners. Mitigated by structural assertions (row count) not timing.

Rollback:
- Delete the two new test files. No existing files were modified.
```

---

## IWP-01 Checklist: Infrastructure Derivation Pipeline Extraction

- [x] Infrastructure filter/search/status/source derivations extracted to pure selectors.
- [x] Sort/group and IO distribution derivation paths simplified to reduce repeated full-list passes.
- [x] Existing Infrastructure behavior preserved (route/query sync, filter semantics, grouping semantics).
- [x] Selector tests include semantic parity assertions.
- [x] Perf contract tests include deterministic infrastructure derivation assertions.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/infrastructureSelectors.test.ts src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-01 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` (new): 12 pure selector functions covering filter/sort/group/IO pipelines
- `frontend-modern/src/components/Infrastructure/__tests__/infrastructureSelectors.test.ts` (new): 23 unit tests for selector semantic parity
- `frontend-modern/src/pages/Infrastructure.tsx`: Rewired 5 derivation memos to use selector imports, removed inline filter/search/source/status logic
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`: Rewired split/sort/group/IO memos to selectors, removed inline helpers (computeMedian, computePercentile, buildIODistribution, getSortValue, defaultComparison, compareValues, isServiceInfrastructureResource). Kept inline getOutlierEmphasis (rendering concern).
- `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`: Replaced todo placeholder with 5 derivation contract assertions

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run ...infrastructureSelectors.test.ts ...UnifiedResourceTable.performance.contract.test.tsx ...Infrastructure.pbs-pmg.test.tsx` -> exit 0 (40 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. Global validation baseline (6 files, 36 tests) -> exit 0

Gate checklist:
- P0: PASS (all tests pass, typecheck clean, no behavior changes, PBS/PMG test unchanged)
- P1: PASS (selectors are pure functions with deterministic tests, no SolidJS reactivity in selectors)
- P2: PASS (inline getOutlierEmphasis kept to minimize blast radius, isResourceOnline kept for per-row rendering)

Verdict: APPROVED

Commit:
- `6f9e2cc3` (refactor(IWP-01): extract Infrastructure derivation pipeline into pure selectors)

Residual risk:
- getOutlierEmphasis remains inline in UnifiedResourceTable.tsx (rendering-coupled); could be extracted in future if needed.

Rollback:
- Revert Infrastructure.tsx and UnifiedResourceTable.tsx to pre-refactor state
- Delete infrastructureSelectors.ts and its test file
- Revert perf contract test to remove derivation assertions
```

---

## IWP-02 Checklist: Workloads Derivation Pipeline Extraction

- [x] Dashboard filter/search/group/sort/stats derivations extracted to pure selectors.
- [x] Repeated full-list operations reduced with consolidated derivation pipeline.
- [x] Mode semantics preserved (`viewMode`, `statusMode`, node/context filters).
- [x] Selector tests cover semantic parity and edge cases.
- [x] Perf contract tests include deterministic workloads derivation assertions.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/workloadSelectors.test.ts src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-02 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Dashboard/workloadSelectors.ts` (new): 11 pure selector functions covering filter/sort/group/stats/IO pipelines
- `frontend-modern/src/components/Dashboard/__tests__/workloadSelectors.test.ts` (new): 17 unit tests for selector semantic parity
- `frontend-modern/src/components/Dashboard/Dashboard.tsx`: Rewired 7 derivation memos to selectors, removed inline helpers (computeMedian, computePercentile, buildIODistribution, computeWorkloadIOEmphasis, getDiskUsagePercent, getGroupKey, workloadNodeScopeId, getKubernetesContextKey)
- `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`: Replaced IWP-02 todo placeholder with 5 derivation contract assertions

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run ...workloadSelectors.test.ts ...Dashboard.performance.contract.test.tsx ...Dashboard.k8s.test.tsx` -> exit 0 (40 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. Global validation baseline (6 files, 36 tests) -> exit 0

Gate checklist:
- P0: PASS (all tests pass, typecheck clean, K8s integration test unchanged)
- P1: PASS (selectors are pure functions, workloadMetricPercent/workloadSummaryGuestId kept for summary rendering)
- P2: PASS (DRY: IO distribution stats imported from infrastructureSelectors where applicable)

Verdict: APPROVED

Commit:
- `86cce391` (refactor(IWP-02): extract Workloads derivation pipeline into pure selectors)

Residual risk:
- workloadMetricPercent and workloadSummaryGuestId remain inline (used only by summary fallback memos)

Rollback:
- Revert Dashboard.tsx to pre-refactor state
- Delete workloadSelectors.ts and its test file
- Revert perf contract test derivation assertions
```

---

## IWP-03 Checklist: Infrastructure Table Windowing and Render Containment

- [x] Row windowing added for infrastructure table with bounded mounted rows.
- [x] Group headers and expansion behavior preserved.
- [x] Deep-link/resource highlight behavior preserved under windowing.
- [x] Perf contract tests assert row mount ceilings and interaction stability.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-03 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Infrastructure/useTableWindowing.ts` (new): Windowing hook with threshold activation (>500 rows), 140-row window, scroll-driven offset, overscan, reveal-index support
- `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`: Flattened grouped rendering to indexed items, applied windowing slice with top/bottom spacers, preserved expand/collapse/highlight/sort/group behavior
- `frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx`: Updated M/L assertions to <=140 mounted rows, added windowing contract tests

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run ...UnifiedResourceTable.performance.contract.test.tsx ...Infrastructure.pbs-pmg.test.tsx` -> exit 0 (17 tests)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. Global baseline + workloads-link (7 files, 38 tests) -> exit 0

Gate checklist:
- P0: PASS (all tests pass, typecheck clean, PBS/PMG and workloads-link tests unaffected)
- P1: PASS (windowing threshold at 500, window size 140, overscan for smooth scrolling)
- P2: PASS (reveal-index prioritizes expanded over highlighted, spacer rows maintain scroll range)

Verdict: APPROVED

Commit:
- pending checkpoint commit

Residual risk:
- jsdom doesn't simulate real scrolling — windowing scroll behavior untested in unit tests; requires manual browser verification

Rollback:
- Delete useTableWindowing.ts, revert UnifiedResourceTable.tsx to pre-windowing state, revert perf contract test assertions
```

---

## IWP-04 Checklist: Workloads Table Windowing and Grouped Render Containment

- [x] Group-aware row windowing implemented for workloads table.
- [x] Drawer open/close, selection, hover, and grouped headers preserved.
- [x] Flat and grouped modes both covered by tests.
- [x] Perf contract tests assert workloads row mount ceilings and stability.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx src/components/Dashboard/__tests__/Dashboard.k8s.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-04 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Dashboard/useGroupedTableWindowing.ts` (new): Grouped windowing hook with threshold >500, 140-row window, scroll-driven offset, selected-row reveal
- `frontend-modern/src/components/Dashboard/Dashboard.tsx`: Integrated grouped windowing with visible group slicing, spacer rows, flat/grouped mode support
- `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`: Replaced IWP-04 todo with windowing contract tests (M <=140, S full render)

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run ...Dashboard.performance.contract.test.tsx ...Dashboard.k8s.test.tsx` -> exit 0 (24 tests)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (all tests pass, typecheck clean, K8s integration unaffected)
- P1: PASS (windowing threshold at 500, window 140, group headers preserved)
- P2: PASS (consistent pattern with IWP-03 infrastructure windowing)

Verdict: APPROVED

Commit:
- pending checkpoint commit

Residual risk:
- jsdom scroll behavior untested; requires manual browser verification

Rollback:
- Delete useGroupedTableWindowing.ts, revert Dashboard.tsx windowing code, revert perf contract assertions
```

---

## IWP-05 Checklist: Polling/Update Backpressure and Recompute Isolation

- [x] Poll/update cadence reviewed and hardened for heavy datasets.
- [x] Unchanged payload updates avoid avoidable heavy downstream churn.
- [x] Hook tests cover cache, polling, coalescing, and freshness semantics.
- [x] Dashboard perf contract reflects polling/update behavior expectations.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/hooks/__tests__/useV2Workloads.test.ts src/hooks/__tests__/useUnifiedResources.test.ts src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### IWP-05 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/hooks/useV2Workloads.ts`: Added areWorkloadsEqual deep equality gate to prevent signal mutation when poll payload is unchanged (reference stability)
- `frontend-modern/src/hooks/__tests__/useV2Workloads.test.ts`: Added cache-hit reuse, empty-response, unchanged-payload stability, and polling cadence tests
- `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts`: Strengthened unchanged-payload stability assertion
- `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`: Added polling-update row stability contract

Audit findings:
- useUnifiedResources already has: cache age 15s, shared in-flight dedup, WS burst coalescing (800ms debounce + 2500ms throttle), reconcile(key:'id') reference stability. No code changes needed.
- useV2Workloads had: cache age 15s, shared in-flight dedup, 5s poll. Missing: reference stability on unchanged payloads. Fixed with areWorkloadsEqual gate.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run ...useV2Workloads.test.ts ...useUnifiedResources.test.ts ...Dashboard.performance.contract.test.tsx` -> exit 0 (25 tests)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (all tests pass, no polling interval changes, no cache behavior changes)
- P1: PASS (conservative deep equality gate, poll intervals untouched)
- P2: PASS (consistent with existing reconcile pattern in useUnifiedResources)

Verdict: APPROVED

Commit:
- pending checkpoint commit

Residual risk:
- Deep equality comparison adds O(n) per poll tick; negligible at realistic fleet sizes

Rollback:
- Revert useV2Workloads.ts areWorkloadsEqual gate, revert test additions
```

---

## IWP-06 Checklist: Summary Path Hardening

- [ ] InfrastructureSummary performance behavior hardened and tested.
- [ ] WorkloadsSummary performance behavior hardened and tested.
- [ ] Cache/fetch dedupe and visible-series constraints validated in tests.
- [ ] Perf regression assertions added for summary surfaces.

### Required Tests

- [ ] `cd frontend-modern && npx vitest run src/components/Infrastructure/InfrastructureSummary.test.tsx src/components/Workloads/WorkloadsSummary.test.tsx src/utils/__tests__/infrastructureSummaryCache.test.ts` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### IWP-06 Review Evidence

```markdown
TODO
```

---

## IWP-07 Checklist: Final Performance Certification

- [ ] Packets IWP-00 through IWP-06 are `DONE` and `APPROVED`.
- [ ] Checkpoint commit hashes recorded for each approved packet.
- [ ] Final budgets verified against perf contract tests.
- [ ] Full frontend suite and typecheck rerun with explicit exit codes.
- [ ] Residual risks and rollback paths documented.

### Required Tests

- [ ] `cd frontend-modern && npx vitest run` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### IWP-07 Review Evidence

```markdown
TODO
```

---

## Checkpoint Commits

- IWP-00: `c28f1c4a`
- IWP-01: `6f9e2cc3`
- IWP-02: `86cce391`
- IWP-03: `bb4844a6`
- IWP-04: `6b8bccd8`
- IWP-05: TODO
- IWP-06: TODO
- IWP-07: TODO

## Current Recommended Next Packet

- `IWP-01` (Infrastructure Derivation Pipeline Extraction)
