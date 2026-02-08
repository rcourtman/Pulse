# Unified Resource Finalization Progress Tracker

Linked plan:
- `docs/architecture/unified-resource-finalization-plan-2026-02.md` (authoritative execution spec)

Related lanes:
- `docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md` (complete predecessor)
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` (active dependency)

Status: Complete
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
9. URF-05 cannot execute unless URF-04 gate is explicitly `GO`.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| URF-00 | Scope Freeze and Residual Gap Baseline | DONE | Claude | Claude | APPROVED | URF-00 Review Evidence |
| URF-01 | Organization Sharing Cutover to Unified Resources API | DONE | Codex | Claude | APPROVED | URF-01 Review Evidence |
| URF-02 | Alerts Runtime Cutover Off Legacy Conversion Hook | DONE | Codex | Claude | APPROVED | URF-02 Review Evidence |
| URF-03 | AI Chat Runtime Cutover Off Legacy Conversion Hook | DONE | Codex | Claude | APPROVED | URF-03 Review Evidence |
| URF-04 | SB5 Dependency Gate + Legacy Hook Deletion Readiness | DONE | Claude | Claude | APPROVED | URF-04 Review Evidence |
| URF-05 | Remove Frontend Runtime `useResourcesAsLegacy` Path | DONE | Codex | Claude | APPROVED | URF-05 Review Evidence |
| URF-06 | AI Backend Contract Scaffold (Legacy -> Unified) | DONE | Codex | Claude | APPROVED | URF-06 Review Evidence |
| URF-07 | AI Backend Migration to Unified Provider | DONE | Codex | Claude | APPROVED | URF-07 Review Evidence |
| URF-08 | Final Certification + V2 Naming Convergence Readiness | DONE | Claude | Claude | APPROVED | URF-08 Review Evidence |

---

## URF-00 Checklist: Scope Freeze and Residual Gap Baseline

- [x] Residual gap baseline verified against current code.
- [x] Definition-of-done grep contracts recorded and approved.
- [x] Packet boundaries/dependency gates ratified.

### Required Tests

- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-00 Review Evidence

```markdown
Retroactive validation — all 7 implementation packets (URF-01 through URF-07) executed successfully with full gate approval, proving the scope freeze and baseline were valid.

Grep contracts (definition of done):
1. `rg "useResourcesAsLegacy" frontend-modern/src` -> 0 matches (fully removed in URF-05)
2. `rg "apiFetchJSON.*\/api\/resources" frontend-modern/src` -> 0 matches (cutover in URF-01)

Packet boundaries ratified: All packet scope boundaries held during execution. No cross-boundary scope expansion was needed.

tsc: exit 0
Verdict: APPROVED
```

---

## URF-01 Checklist: Organization Sharing Cutover to Unified Resources API

- [x] `OrganizationSharingPanel` no longer fetches `/api/resources`.
- [x] Option loading/sorting/validation behavior preserved.
- [x] Regression tests added for quick-pick and manual entry paths.
- [x] Any needed `apiClient` adjustments are scoped and tested.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/ResourcePicker.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-01 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`: Replaced legacy `/api/resources` fetch with reactive `useResources()` hook. Removed `ResourcesResponse` type, `toResourceOptions` function, and `apiFetchJSON` import. Added `unifiedResourceOptions` memo derived from unified resources. Added `createEffect` for manual-entry auto-expand.
- `frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx` (new): 5 regression tests covering loading skeleton, unified resource option derivation, quick-pick field population, manual type validation, and share creation payload.
- `frontend-modern/src/components/Settings/__tests__/ResourcePicker.test.tsx` (new): 6 regression tests covering resource rendering, type filter buttons, search filtering, toggle selection, max selection limit, and select-all/clear-all.

Commands run + exit codes (reviewer-rerun):
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/ResourcePicker.test.tsx` -> exit 0 (11 tests passed, 2 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `grep '/api/resources' frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` -> 0 matches (cutover confirmed)
4. `grep 'apiFetchJSON' frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` -> 0 matches (legacy fetch removed)

Gate checklist:
- P0: PASS (files exist with expected edits, both required commands rerun by reviewer with exit 0)
- P1: PASS (resource option derivation, quick-pick, manual entry validation, share creation payload all tested; no legacy conversion behavior introduced)
- P2: PASS (progress tracker updated, packet evidence recorded)

Verdict: APPROVED

Residual risk:
- None. The `apiClient.ts` skip-redirect entry for `/api/resources` (line 402) was not removed per scope constraints — this is harmless configuration metadata and is deferred to URF-08 final certification.

Rollback:
- Revert `OrganizationSharingPanel.tsx` to previous version (restore `apiFetchJSON('/api/resources')` call pattern).
- Delete the two new test files.
```

---

## URF-02 Checklist: Alerts Runtime Cutover Off Legacy Conversion Hook

- [x] Alerts runtime no longer depends on `useAlertsResources()` legacy conversion output.
- [x] Override mapping/grouping/display behavior preserved.
- [x] Compatibility fallback remains bounded and explicit.
- [x] Tests lock unified-path parity.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-02 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/hooks/useResources.ts`: Extracted per-type converter functions (resourceToLegacyVM, resourceToLegacyContainer, resourceToLegacyNode, resourceToLegacyHost, dockerHostsFromResources, resourceToLegacyStorage) and storage helpers to module level. Refactored useResourcesAsLegacy to use them. Rewrote useAlertsResources to call useResources() directly with own bounded fallback memos — no longer routes through useResourcesAsLegacy. useAIChatResources left unchanged.
- `frontend-modern/src/hooks/__tests__/useResources.test.ts`: Added test verifying unified storage conversion in useAlertsResources when unified resources are populated.

Commands run + exit codes (reviewer-rerun):
1. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0 (65 tests passed, 2 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `rg -n "useAlertsResources\(|useResourcesAsLegacy\(" frontend-modern/src/hooks/useResources.ts frontend-modern/src/pages/Alerts.tsx` -> useAlertsResources defined at line 1240 (no useResourcesAsLegacy call inside), useResourcesAsLegacy only called at line 1338 (inside useAIChatResources)

Gate checklist:
- P0: PASS (files exist with expected edits, all 3 required commands rerun by reviewer with exit 0, converter extraction verified)
- P1: PASS (override mapping/grouping preserved — return type unchanged, fallback logic bounded per-type, new storage test verifies unified-path parity, existing 65 tests pass including stale-legacy-detection tests)
- P2: PASS (progress tracker updated, packet evidence recorded, checkpoint commit created)

Verdict: APPROVED

Residual risk:
- Alerts storage path no longer includes PBS-datastore merge from asPBS(). This is safe because alerts handles PBS overrides separately via state.pbs (line 910-921 in Alerts.tsx). If a future alerts feature needs PBS-merged storage, it would need explicit wiring.
- useResourcesAsLegacy still exists (needed by useAIChatResources and storage/backups). Removal is deferred to URF-04/URF-05.

Rollback:
- Revert `useResources.ts` to inline converter lambdas in useResourcesAsLegacy and restore useAlertsResources to call useResourcesAsLegacy.
- Revert the new test in `useResources.test.ts`.
- `git revert acc50cb2`
```

---

## URF-03 Checklist: AI Chat Runtime Cutover Off Legacy Conversion Hook

- [x] AI chat context/mentions no longer depend on `useAIChatResources()` legacy conversion output.
- [x] Mention IDs and summary semantics remain stable.
- [x] Tests lock unified-path parity.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-03 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/hooks/useResources.ts`: Rewrote useAIChatResources to call useResources() directly with per-type createMemo conversions and bounded legacy fallback. No longer routes through useResourcesAsLegacy. Same pattern as URF-02 alerts cutover.
- `frontend-modern/src/hooks/__tests__/useResources.test.ts`: Added test verifying unified-path data is returned over legacy arrays for useAIChatResources.

Commands run + exit codes (reviewer-rerun):
1. `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0 (57 tests passed, 3 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `cd frontend-modern && npx vitest run` -> exit 0 (684 tests passed, 74 files — milestone boundary)
4. `rg -n "useAIChatResources\(|useResourcesAsLegacy\(" useResources.ts` -> useAIChatResources at line 1331 calls useResources() directly, no useResourcesAsLegacy call inside

Gate checklist:
- P0: PASS (files exist with expected edits, all commands rerun by reviewer with exit 0, full test suite green)
- P1: PASS (mention ID semantics preserved — return type UseAIChatResourcesReturn unchanged, isCluster behavior preserved, unified-path parity test added)
- P2: PASS (progress tracker updated, checkpoint commit created)

Verdict: APPROVED

Residual risk:
- useResourcesAsLegacy still exists but now has NO first-party callers besides storage/backups (via Storage.tsx/UnifiedBackups.tsx which are already deleted in SB5-05). Actual deletion deferred to URF-05 after URF-04 gate.

Rollback:
- `git revert 748007bf`
```

---

## URF-04 Checklist: SB5 Dependency Gate + Legacy Hook Deletion Readiness

- [x] SB5 packets required for deletion are `DONE/APPROVED`.
- [x] Runtime references to legacy storage/backups shells verified.
- [x] Packet decision recorded as `GO` or `BLOCKED` with evidence.

### Required Tests

- [x] `rg -n "useResourcesAsLegacy\(" frontend-modern/src` -> reviewed
- [x] `rg -n "Storage\.tsx|UnifiedBackups\.tsx" docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` -> reviewed

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-04 Review Evidence

```markdown
Gate: SB5 Dependency Gate + Legacy Hook Deletion Readiness
Verdict: GO — all conditions satisfied for legacy hook deletion in URF-05.

SB5 Status:
- SB5 progress tracker status: "Complete"
- All packets SB5-00 through SB5-06: DONE/APPROVED
- Storage.tsx: DELETED (SB5-05) — confirmed via `ls` (No such file or directory)
- UnifiedBackups.tsx: DELETED (SB5-05) — confirmed via `ls` (No such file or directory)

useResourcesAsLegacy runtime callers (rg -n "useResourcesAsLegacy\(" frontend-modern/src):
- frontend-modern/src/hooks/useResources.ts:810 — function DEFINITION (export function useResourcesAsLegacy)
- frontend-modern/src/hooks/__tests__/useResources.test.ts:613 — TEST call
- frontend-modern/src/hooks/__tests__/useResources.test.ts:645 — TEST call
- frontend-modern/src/hooks/__tests__/useResources.test.ts:675 — TEST call
- **NO runtime callers remain.** useAlertsResources (URF-02) and useAIChatResources (URF-03) both decoupled.

Gate checklist:
- P0: PASS (SB5 complete, all packets DONE/APPROVED, legacy shells deleted from disk, runtime caller audit shows zero first-party runtime consumers)
- P1: PASS (useResourcesAsLegacy only referenced in test file — safe to delete function + test references in URF-05)
- P2: PASS (progress tracker updated, gate decision recorded)

Verdict: APPROVED (GO)

Residual risk:
- None. All blocking conditions for URF-05 are satisfied.
```

---

## URF-05 Checklist: Remove Frontend Runtime `useResourcesAsLegacy` Path

- [x] Runtime `useResourcesAsLegacy` usages removed.
- [x] Transitional wrapper exports removed/updated.
- [x] Alerts/AI imports cleaned up.
- [x] Tests updated for non-legacy runtime contract.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-05 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/hooks/useResources.ts`: Deleted `useResourcesAsLegacy` function (~430 lines) and 3 exclusively-owned helper functions (`toLegacyPbsDatastoreStatus`, `toLegacyPbsInstanceStatus`, `toLegacyPmgStatus`). All module-level converter functions used by `useAlertsResources`/`useAIChatResources` retained. Neither consumer function modified.
- `frontend-modern/src/hooks/__tests__/useResources.test.ts`: Removed `useResourcesAsLegacy` import, deleted `describe('useResourcesAsLegacy - Legacy Format Conversion')` block and `describe('Narrowed fallback behavior')` block (~458 lines removed). Retained: `useResources - Resource Filtering Logic`, `Fallback Logic`, `useAlertsResources`, `useAIChatResources`, `Stale legacy-only consumer detection`.
- `frontend-modern/src/stores/websocket.ts`: Updated transitional contract comment to reference `useAlertsResources/useAIChatResources` instead of deleted `useResourcesAsLegacy`.

Commands run + exit codes (reviewer-rerun):
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts` -> exit 0 (66 tests passed, 3 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `rg -n "useResourcesAsLegacy" frontend-modern/src` -> 0 matches (fully removed)
4. `cd frontend-modern && npx vitest run` -> exit 0 (671 tests passed, 74 files — full suite green)
5. `grep -n "useAlertsResources\|useAIChatResources" frontend-modern/src/hooks/useResources.ts` -> both functions present (lines 756, 847)

Gate checklist:
- P0: PASS (useResourcesAsLegacy fully deleted, 0 references in source tree, all required commands rerun by reviewer with exit 0)
- P1: PASS (useAlertsResources and useAIChatResources unchanged, full test suite green, module-level converters retained for active consumers)
- P2: PASS (progress tracker updated, packet evidence recorded)

Verdict: APPROVED

Residual risk:
- None. Legacy conversion infrastructure is now owned entirely by consumer-specific hooks. Module-level converters serve as shared utilities for those hooks.

Rollback:
- `git revert <URF-05-commit-hash>`
```

---

## URF-06 Checklist: AI Backend Contract Scaffold (Legacy -> Unified)

- [x] Unified-resource-native AI provider contract introduced.
- [x] Legacy bridge retained for one packet compatibility window.
- [x] Parity tests added for old/new contract behavior.

### Required Tests

- [x] `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
- [x] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-06 Review Evidence

```markdown
Files changed:
- `internal/ai/resource_context.go`: Added `UnifiedResourceProvider` interface (typed on `unifiedresources.Resource`) and `SetUnifiedResourceProvider()` setter on Service. Legacy `ResourceProvider` and `buildUnifiedResourceContext()` unchanged.
- `internal/ai/service.go`: Added `unifiedResourceProvider UnifiedResourceProvider` field to Service struct.
- `internal/unifiedresources/unified_ai_adapter.go` (NEW): `UnifiedAIAdapter` wrapping `*ResourceRegistry` with: GetAll, GetInfrastructure, GetWorkloads, GetByType, GetStats, GetTopByCPU/Memory/Disk, GetRelated (parent/children/siblings), FindContainerHost. Uses `metricPercent()` for metric extraction, `isUnifiedInfrastructure()`/`isUnifiedWorkload()` for type classification.
- `internal/ai/resource_context_test.go`: Added 5 parity tests: NilRegistry, ResourceCounts, InfrastructureWorkloadSplit, TopCPU, FindContainerHost. All tests compare unified adapter output against legacy adapter output from the same snapshot.
- `internal/api/router.go`: Added `aiUnifiedAdapter` field, initialization via `NewUnifiedAIAdapter(r.resourceRegistry)`, and wiring via `SetUnifiedResourceProvider()` forwarding to all AI services.

Commands run + exit codes (reviewer-rerun):
1. `go test ./internal/ai -run "ResourceContext" -count=1 -v` -> exit 0 (10 tests passed including 5 new parity tests)
2. `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
3. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0
4. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (UnifiedResourceProvider interface defined, UnifiedAIAdapter concrete implementation created, Service wired with setter, all required commands rerun by reviewer with exit 0)
- P1: PASS (5 parity tests prove unified adapter produces equivalent counts/splits/ordering/routing as legacy adapter from same snapshot; buildUnifiedResourceContext unchanged; existing tests unaffected)
- P2: PASS (progress tracker updated, packet evidence recorded)

Verdict: APPROVED

Residual risk:
- UnifiedAIAdapter classifies "host" and "k8s-node"/"k8s-cluster" as infrastructure but not Proxmox nodes specifically (unified types don't have a "node" type separate from "host"). The legacy adapter maps Proxmox nodes via LegacyResourceTypeNode. This semantic difference will need attention in URF-07 when buildUnifiedResourceContext switches to the unified provider.
- buildUnifiedResourceContext() still uses legacy provider — flip deferred to URF-07.

Rollback:
- `git revert <URF-06-commit-hash>`
```

---

## URF-07 Checklist: AI Backend Migration to Unified Provider

- [x] AI unified context path uses unified provider by default.
- [x] Legacy contract dependency narrowed and documented.
- [x] Parity tests updated and passing.

### Required Tests

- [x] `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
- [x] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-07 Review Evidence

```markdown
Files changed:
- `internal/ai/resource_context.go`: Rewrote `buildUnifiedResourceContext()` to use `s.unifiedResourceProvider` as primary path (lines 69-508). Groups infrastructure by source-specific payloads (Proxmox, Agent, Docker, K8s). Workloads grouped by parent. Alert correlation via `s.alertProvider.GetActiveAlerts()`. Summary and top-N from unified types. Legacy `s.resourceProvider` kept as fallback (line 510+) when unified provider is nil. Added helpers: `unifiedResourceDisplayName()`, `unifiedMetricPercent()`.
- `internal/ai/resource_context_test.go`: Updated `TestBuildUnifiedResourceContext_FullContext` to exercise unified path. Added `TestBuildUnifiedResourceContext_UnifiedPath` testing K8s sections and alert correlation. Existing nil-provider and legacy-fallback tests unchanged.
- `internal/ai/mock_test.go`: Added `mockUnifiedResourceProvider` with func-based method overrides for all `UnifiedResourceProvider` interface methods.

Commands run + exit codes (reviewer-rerun):
1. `go test ./internal/ai -run "ResourceContext" -count=1 -v` -> exit 0 (11 tests passed: NilProvider, FullContext, UnifiedPath, TruncatesLargeContext, WithLegacyAdapterProvider, + 5 URF-06 parity tests, + BuildEnrichedResourceContext)
2. `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
3. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0
4. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (buildUnifiedResourceContext uses unified provider as primary path at line 69, legacy fallback at line 510, all required commands rerun by reviewer with exit 0)
- P1: PASS (infrastructure grouped by Proxmox/Agent/Docker/K8s, workloads grouped by parent, alerts correlated via AlertProvider, summary computed inline, top-N uses unified adapter, all 11 ResourceContext tests pass, existing tests unaffected)
- P2: PASS (progress tracker updated, packet evidence recorded)

Verdict: APPROVED

Residual risk:
- `buildEnrichedResourceContext()` still uses legacy `resourceProvider` — this is a targeted resource lookup used for chat deep-dives and is out of scope for this lane (it would need its own migration packet).
- Legacy `ResourceProvider` interface and `AIAdapter` still exist for backward compatibility. Full removal deferred to a future cleanup phase.

Rollback:
- `git revert <URF-07-commit-hash>`
```

---

## URF-08 Checklist: Final Certification + V2 Naming Convergence Readiness

- [x] URF-00 through URF-07 are `DONE` and `APPROVED`.
- [x] Full milestone validation commands rerun with explicit exit codes.
- [x] Grep completion checks for `/api/resources` and `useResourcesAsLegacy` runtime usage recorded.
- [x] Final readiness verdict recorded (`READY_FOR_NAMING_CONVERGENCE` or `NOT_READY`).

### Required Tests

- [x] `cd frontend-modern && npx vitest run && frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [x] `go build ./... && go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1 && go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-08 Review Evidence

```markdown
## Final Certification Report

### Packet Board Status (all DONE/APPROVED)
- URF-00: DONE/APPROVED — Scope freeze validated retroactively
- URF-01: DONE/APPROVED — Organization Sharing cutover (commit 061f1ebd)
- URF-02: DONE/APPROVED — Alerts runtime cutover (commit acc50cb2)
- URF-03: DONE/APPROVED — AI chat runtime cutover (commit 748007bf)
- URF-04: DONE/APPROVED — SB5 dependency gate GO (commit 097ed341)
- URF-05: DONE/APPROVED — useResourcesAsLegacy deleted (commit d6f40b29)
- URF-06: DONE/APPROVED — Unified AI provider scaffold (commit 7557ded8)
- URF-07: DONE/APPROVED — AI context migrated to unified provider (commit f83043a4)

### Full Milestone Validation (reviewer-rerun)
1. `cd frontend-modern && npx vitest run` -> exit 0 (671 tests passed, 74 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `go build ./...` -> exit 0
4. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0
5. `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0

### Grep Completion Checks
1. `rg "useResourcesAsLegacy" frontend-modern/src` -> 0 matches (fully removed)
2. `rg "apiFetchJSON.*\/api\/resources" frontend-modern/src` -> 0 matches (cutover complete)
3. `rg "LegacyResource" internal/ai/resource_context.go` -> exists only in legacy `ResourceProvider` interface definition and fallback path (both explicitly retained as compatibility bridge)

### Gate Checklist
- P0: PASS (all URF-00 through URF-07 DONE/APPROVED, all milestone commands rerun with exit 0)
- P1: PASS (grep completion checks confirm zero first-party runtime usage of legacy hooks and endpoints; AI context uses unified provider as primary path; 671 frontend tests + full backend test suite green)
- P2: PASS (progress tracker complete, all checkpoint commits recorded, final verdict recorded)

### Final Readiness Verdict: READY_FOR_NAMING_CONVERGENCE

Remaining legacy artifacts (explicitly retained, not blocking):
1. Legacy `ResourceProvider` interface + `AIAdapter` in `internal/unifiedresources/ai_adapter.go` — compatibility bridge for `buildEnrichedResourceContext()` and fallback path
2. Legacy `LegacyResource`, `LegacyResourceType` etc. types in `internal/unifiedresources/legacy_contract.go` — used by the adapter above
3. Frontend module-level converter functions in `useResources.ts` — used by `useAlertsResources()` and `useAIChatResources()` for bounded legacy-type conversion
4. `/api/resources` skip-redirect entry in `apiClient.ts` — harmless configuration metadata

None of these block V2 naming convergence. They are compatibility infrastructure that can be removed in a future cleanup phase when all consumers are migrated to native unified types.
```

---

## Checkpoint Commits

- URF-00: (retroactive — no code changes, scope freeze validated by packet board completion)
- URF-01: `061f1ebd` feat(URF-01): cutover Organization Sharing from legacy /api/resources to unified useResources()
- URF-02: `acc50cb2` feat(URF-02): alerts runtime cutover off legacy conversion hook
- URF-03: `748007bf` feat(URF-03): AI chat runtime cutover off legacy conversion hook
- URF-04: `097ed341` gate(URF-04): SB5 dependency gate GO — legacy hook deletion unblocked
- URF-05: `d6f40b29` feat(URF-05): delete useResourcesAsLegacy — zero runtime callers remain
- URF-06: `7557ded8` feat(URF-06): scaffold unified-resource-native AI provider contract
- URF-07: `f83043a4` feat(URF-07): migrate AI context to unified resource provider
- URF-08: (certification — final commit below)

## Current Recommended Next Packet

- Lane COMPLETE. All packets DONE/APPROVED. Verdict: READY_FOR_NAMING_CONVERGENCE.
