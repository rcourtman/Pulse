# Storage Page GA Hardening Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md`

## Product Intent

Storage should ship at the same architecture quality bar as Infrastructure and Settings:
1. deterministic data contracts,
2. predictable route/query behavior,
3. modular page composition,
4. hard regression coverage for parity and future source expansion.

This plan graduates Storage from "working preview implementation" to a stable, testable GA-grade surface.

## Non-Negotiable Contracts

1. Route and deep-link contract:
- `/storage` remains the canonical entry point.
- Existing storage query params (`tab`, `group`, `source`, `status`, `node`, `q`, `resource`, `sort`, `order`) remain compatible.
- Legacy `search` alias remains supported until explicit removal packet.

2. Behavior parity contract:
- No regression to current useful behavior from legacy storage page:
  - alert highlighting semantics,
  - storage resource deep-link expansion/highlight,
  - reconnect/disconnected/waiting states,
  - filter and sorting semantics.

3. Data contract:
- Storage row identity must be deterministic across unified-resource and legacy fallback paths.
- Resource-origin data must win when both origins describe the same entity.
- No duplicate logical storage rows from mismatched IDs across origins.

4. Architecture contract:
- Storage domain semantics (health normalization, Ceph/ZFS labeling, source mapping) cannot remain duplicated in page shells.
- `StorageV2.tsx` must become composition-first; domain transforms belong in dedicated modules/hooks.

5. Cross-track safety contract:
- Packet scopes must avoid opportunistic rewrites in Backups, Alerts, Multi-tenant, or unrelated Settings work.
- Out-of-scope failures are documented with evidence, not "fixed" by broad edits.

6. Rollback contract:
- Every packet has file-granular rollback instructions.
- No destructive git operations are required for rollback.

## Code-Derived Audit Baseline

### A. Monolith and duplication hotspots

1. Storage shells are still large and parallel:
- `frontend-modern/src/components/Storage/Storage.tsx`: 1645 LOC
- `frontend-modern/src/components/Storage/StorageV2.tsx`: 1080 LOC

2. Duplicated storage-domain logic in both shells:
- `isCephType`, `getCephHealthLabel`, `getCephHealthStyles` exist in both files.
- Route/query synchronization logic is duplicated in both files.

3. Storage V2 still signals preview status while functioning as default route:
- `Storage V2 Preview` copy exists in `frontend-modern/src/components/Storage/StorageV2.tsx`.

### B. Data-shape and merge weaknesses

1. Storage V2 adapter merge currently deduplicates by `record.id` only:
- `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts`
- This is fragile when resource-origin and legacy-origin IDs differ for the same logical storage target.

2. Legacy-bridge storage synthesis contains correctness smells:
- Duplicate `enabled` assignment appears in `frontend-modern/src/hooks/useResources.ts` (`asStorage` object synthesis).
- Indicates merge/precedence behavior needs explicit contract hardening.

3. Unified resource storage payload is too thin for frontend V2 parity:
- `internal/unifiedresources/adapters.go` `resourceFromStorage(...)` maps only high-level node/instance + metrics.
- Storage-specific attributes (type/content/shared/ZFS cues) are not first-class in unified payload, forcing frontend heuristics/fallback coupling.

### C. Parity and test coverage gaps

1. Legacy Storage behavior testing is thin:
- Legacy storage coverage is mostly routing contract plus source option unit tests.
- No deep behavior tests for legacy alert/deep-link interactions.

2. V2 tests focus on render/state happy paths, not full parity contracts:
- Missing hard contract tests for canonical identity merge behavior across mixed source IDs.
- Missing explicit architecture guardrails to prevent logic re-inlining and monolith regression.

## Audit Findings (Priority)

| ID | Severity | Finding | Evidence |
|---|---|---|---|
| SP-001 | High | Two parallel storage shells have overlapping domain logic and inconsistent behavior contracts. | `frontend-modern/src/components/Storage/Storage.tsx`, `frontend-modern/src/components/Storage/StorageV2.tsx` |
| SP-002 | High | Storage identity merge is not canonicalized across origins; duplicate logical rows are possible. | `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts` |
| SP-003 | High | Unified storage resource payload lacks storage-specific fields, forcing fragile frontend fallbacks. | `internal/unifiedresources/adapters.go` |
| SP-004 | Medium | V2 default route still presents preview semantics and inconsistent UX parity. | `frontend-modern/src/components/Storage/StorageV2.tsx` |
| SP-005 | Medium | Legacy bridge synthesis has quality debt (`enabled` duplicated) and unclear precedence. | `frontend-modern/src/hooks/useResources.ts` |
| SP-006 | Medium | Contract tests do not yet lock identity merge and parity behavior strongly enough. | Storage tests in `frontend-modern/src/components/Storage/__tests__` and `frontend-modern/src/features/storageBackupsV2/__tests__` |

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: delegated coding agent.
- Reviewer: orchestrator.

A packet can be marked DONE only when:
- all packet checkboxes are checked,
- all listed commands are run with explicit exit codes,
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
2. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx src/components/Storage/__tests__/storageSourceOptions.test.ts`
3. `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageAdapters.test.ts src/routing/__tests__/resourceLinks.test.ts src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts`

When backend packets are touched, additionally run:

4. `go test ./internal/unifiedresources/... ./internal/api/... -run "ResourcesV2|ResourceHandlers|Storage|RouteInventory" -count=1`
5. `go build ./...`

Notes:
- `go build` alone is never sufficient for approval.
- Empty/timed-out/truncated output without explicit exit code is invalid evidence.

## Execution Packets

### Packet 00: Storage Architecture Inventory and Contract Freeze

Objective:
- Freeze a precise storage architecture baseline and map risks to implementation packets.

Scope:
- `docs/architecture/storage-page-ga-hardening-plan-2026-02.md` (appendices only)
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md`

Implementation checklist:
1. Record file/function anchors for all high-risk storage flows.
2. Map each SP risk to exactly one owner packet.
3. Document rollback notes per high-severity risk.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Risk register is packet-mapped with explicit rollback notes.

### Packet 01: Route/Query Orchestration Extraction

Objective:
- Eliminate duplicated route/query synchronization logic between storage shells.

Scope:
- `frontend-modern/src/components/Storage/Storage.tsx`
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/components/Storage/useStorageRouteState.ts` (new)
- `frontend-modern/src/routing/__tests__/resourceLinks.test.ts`

Implementation checklist:
1. Extract route parse/write orchestration into shared hook/helper.
2. Preserve all query param canonicalization behavior.
3. Preserve shareable URL behavior for storage filters/search.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/Storage.routing.test.tsx src/components/Storage/__tests__/StorageV2.test.tsx src/routing/__tests__/resourceLinks.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Storage shells no longer own duplicated query sync logic.

### Packet 02: Shared Storage Domain Semantics Extraction

Objective:
- Move duplicated storage-domain primitives into a shared, tested module.

Scope:
- `frontend-modern/src/components/Storage/Storage.tsx`
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/features/storageBackupsV2/storageDomain.ts` (new)
- `frontend-modern/src/features/storageBackupsV2/__tests__/storageDomain.test.ts` (new)

Implementation checklist:
1. Extract Ceph/ZFS/source/status helper logic into shared module.
2. Remove duplicate helper definitions from both page shells.
3. Add deterministic unit tests for health/status/source normalization.

Required tests:
1. `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageDomain.test.ts src/components/Storage/__tests__/StorageV2.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Domain primitives are single-sourced and test-locked.

### Packet 03: Storage V2 View-Model Decomposition

Objective:
- Move filtering/sorting/grouping/summary/ceph-aggregation from `StorageV2.tsx` into dedicated hooks.

Scope:
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/components/Storage/useStorageV2Model.ts` (new)
- `frontend-modern/src/components/Storage/useStorageV2CephModel.ts` (new)
- `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`

Implementation checklist:
1. Extract all non-render data transforms into hooks.
2. Keep render shell focused on composition and interaction wiring.
3. Preserve behavioral parity for filters, grouping, sorting, and ceph drawer.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- `StorageV2.tsx` is composition-driven, not transformation-heavy.

### Packet 04: Storage V2 UX Parity Uplift

Objective:
- Bring V2 interaction semantics to parity with mature pages and retire preview posture.

Scope:
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/components/Storage/StorageFilter.tsx`
- `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`
- `frontend-modern/src/routing/platformTabs.ts` (only if label/tooltips need parity correction)

Implementation checklist:
1. Replace preview-only copy with GA-oriented semantics.
2. Add persistent filter/sort/view state behavior equivalent to legacy expectations.
3. Ensure empty/loading/disconnected states match UX standards used elsewhere.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/routing/__tests__/platformTabs.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- V2 no longer reads as preview and preserves state/UX parity expectations.

### Packet 05: Alert and Deep-Link Parity Hardening

Objective:
- Restore alert semantics and deep-link row focus behavior parity in V2.

Scope:
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/components/Storage/useStorageV2AlertState.ts` (new)
- `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`

Implementation checklist:
1. Introduce alert style/state integration equivalent to legacy behavior.
2. Support resource-driven row expansion/highlighting for storage deep-links.
3. Preserve disconnect/reconnect and stale-data cues.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Alert/deep-link behavior parity is test-locked in V2.

### Packet 06: Adapter Identity and Merge Contract Hardening

Objective:
- Guarantee canonical storage identity and deterministic precedence across origins.

Scope:
- `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts`
- `frontend-modern/src/features/storageBackupsV2/models.ts`
- `frontend-modern/src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`

Implementation checklist:
1. Add canonical identity key strategy (not raw `id` only).
2. Enforce deterministic precedence (resource > legacy) per canonical identity.
3. Preserve capability/details union without duplicate logical records.

Required tests:
1. `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Mixed-origin duplicate logical rows are prevented by contract.

### Packet 07: Backend Unified Storage Metadata Enrichment

Objective:
- Enrich unified storage resources so frontend can stop relying on fragile legacy heuristics.

Scope:
- `internal/unifiedresources/types.go`
- `internal/unifiedresources/adapters.go`
- `internal/api/resources_v2.go`
- `internal/unifiedresources/*_test.go` and/or `internal/api/resources_v2_test.go` (targeted updates only)

Implementation checklist:
1. Add storage-specific metadata fields to unified resource payload contract.
2. Populate type/content/shared/ZFS-relevant storage hints from snapshot models.
3. Preserve route/auth/tenant behavior and existing response compatibility.

Required tests:
1. `go test ./internal/unifiedresources/... ./internal/api/... -run "ResourcesV2|storage|Storage|registry" -count=1`
2. `go build ./...`

Exit criteria:
- Unified storage payload carries enough metadata for frontend parity without legacy-only heuristics.

### Packet 08: Frontend Consumption of Enriched Storage Metadata

Objective:
- Consume backend storage metadata contract and simplify frontend fallback logic.

Scope:
- `frontend-modern/src/hooks/useResources.ts`
- `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts`
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`

Implementation checklist:
1. Replace brittle inferred fields with enriched unified metadata usage.
2. Remove duplicated/ambiguous merge assignments in `asStorage` synthesis.
3. Keep fallback behavior for truly missing unified coverage only.

Required tests:
1. `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageAdapters.test.ts src/components/Storage/__tests__/StorageV2.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Frontend storage model is unified-resource-first with narrow, explicit fallback.

### Packet 09: Contract Test Hardening and Monolith Regression Guardrails

Objective:
- Add tests/guardrails that prevent storage architecture drift back into monolith/duplication.

Scope:
- `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`
- `frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx`
- `frontend-modern/src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`
- `frontend-modern/src/components/Storage/code_standards_test.ts` or equivalent targeted guard file (new)

Implementation checklist:
1. Add contract tests for identity merge, deep-link expansion, alert parity, and query canonicalization.
2. Add architecture guardrails for duplicated storage-domain helper definitions.
3. Keep tests focused on storage domain only.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Storage contracts are regression-protected across route, data, and interaction layers.

### Packet 10: Final Certification and GA Readiness Verdict

Objective:
- Certify storage GA hardening completion with explicit Go/No-Go evidence.

Scope:
- `docs/architecture/storage-page-ga-hardening-plan-2026-02.md` (final appendices)
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md`

Implementation checklist:
1. Verify packet evidence and checkpoint commits for 00-09.
2. Run final validation gate (frontend + backend where touched).
3. Record release recommendation and residual risk (if any).

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run`
3. `go build ./...`

Exit criteria:
- Final verdict recorded as `GO` or `GO_WITH_CONDITIONS` with explicit blocker list.

## Risk Register

| Risk ID | Description | Severity | Packet Owner | Rollback Strategy |
|---|---|---|---|---|
| SP-001 | Dual shell drift and inconsistent behavior | High | 01-05 | Revert extracted hooks/modules and restore previous shell-local logic packet-by-packet |
| SP-002 | Cross-origin identity collisions/duplicates | High | 06 | Revert canonical identity changes in adapters and tests only |
| SP-003 | Unified payload missing storage detail fields | High | 07 | Revert added metadata fields and adapter mappings in backend only |
| SP-004 | Frontend fallback simplification hides missing data | Medium | 08 | Restore prior fallback merge behavior and re-enable conservative fallback path |
| SP-005 | Regression to preview/unfinished UX semantics | Medium | 04-05 | Revert V2 UX packet commits while preserving contract extraction packets |
| SP-006 | Future re-monolith of storage logic | Medium | 09 | Keep guardrails/tests; rollback only if false-positive, with targeted rule adjustments |

## Appendix A: High-Risk Storage Flow Anchors (Packet 00 Baseline)

Captured: 2026-02-08

### A.1 Route/Query Synchronization (SP-001 → Packets 01, 04)

| Shell | Read Effect (URL → signals) | Write Effect (signals → URL) |
|---|---|---|
| `Storage.tsx` (legacy) | Lines 105-137: `createEffect` parsing `location.search` via `parseStorageLinkSearch` → `setTabView`, `setViewMode`, `setSourceFilter`, `setStatusFilter`, `setSelectedNode`, `setSearchTerm` | Lines 139-173: `createEffect` building URL via `buildStoragePath`, merging unmanaged params |
| `StorageV2.tsx` (V2) | Lines 484-511: `createEffect` parsing `location.search` via `parseStorageLinkSearch` → `setView`, `setSourceFilter`, `setHealthFilter`, `setSelectedNodeId`, `setGroupBy`, `setSortKey`, `setSortDirection`, `setSearch` | Lines 513-540: `createEffect` building URL via `buildStorageV2Path`, merging unmanaged params |

V2 supports additional params: `sort`, `order`, `group`. Both shells use `parseStorageLinkSearch` for `query`/`search` alias canonicalization.

### A.2 Ceph/ZFS Domain Helpers (SP-001 → Packet 02)

| Helper | Storage.tsx | StorageV2.tsx |
|---|---|---|
| `isCephType(type?: string)` | Lines 209-212 | Lines 117-120 |
| `getCephHealthLabel(health?: string)` | Lines 214-218 | Lines 133-137 |
| `getCephHealthStyles(health?: string)` | Lines 220-232 | Lines 139-151 |

ZFS rendering uses `ZFSHealthMap` component in both shells (no dedicated helper extraction needed).

### A.3 Storage Identity Merge (SP-002 → Packet 06)

- **File**: `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts`
- **Function**: `buildStorageRecordsV2()` at lines 263-293
- **Identity key**: `record.id` only (line 277, 283, 285, 288)
- **Merge function**: `mergeStorageRecords()` at lines 263-271 — prefers `resource` origin; shallow-merges `details`; deduplicates capabilities
- **Risk**: If resource-origin and legacy-origin produce different `id` values for the same logical storage target, duplicates appear.

### A.4 Legacy Bridge Synthesis (SP-005 → Packet 08)

- **File**: `frontend-modern/src/hooks/useResources.ts`
- **Function**: `asStorage` memo at lines 877-1000
- **Duplicate `enabled` assignments**:
  - Line 899: `enabled: true` (hardcoded for PBS datastores from `asPBS()`)
  - Line 952: `enabled: true` (hardcoded for `resource.type === 'datastore'`)
  - Line 980: `enabled: enabled ?? (resource.status !== 'offline' && resource.status !== 'stopped')` (computed fallback)
- **Risk**: PBS datastores may appear from both `asPBS()` and `resources()`, causing merge ambiguity.

### A.5 Unified Resource Storage Adapter (SP-003 → Packet 07)

- **File**: `internal/unifiedresources/adapters.go`
- **Function**: `resourceFromStorage(storage models.Storage)` at lines 292-318
- **Fields currently mapped**: Type, Name (fallback to ID), Status (via `statusFromStorage`), LastSeen, UpdatedAt, Metrics (via `metricsFromStorage`), Proxmox.NodeName, Proxmox.Instance, Hostnames
- **Missing for frontend parity**: storage type, content type, shared flag, Ceph/ZFS indicators, pool/datastore metadata

### A.6 V2 Preview Semantics (SP-005 → Packet 04)

- **File**: `frontend-modern/src/components/Storage/StorageV2.tsx` line 553
- **Copy**: `"Storage V2 Preview"` heading + `"Source-agnostic storage view model with capability-first normalization."` subtitle
- **Risk**: Default route presents as "preview" while functioning as GA path.

### A.7 Test Coverage Baseline (SP-006 → Packet 09)

| Test File | Domain |
|---|---|
| `frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx` | Route contracts |
| `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx` | V2 render/state |
| `frontend-modern/src/components/Storage/__tests__/storageSourceOptions.test.ts` | Source filters |
| `frontend-modern/src/features/storageBackupsV2/__tests__/storageAdapters.test.ts` | Adapter merge |
| `frontend-modern/src/features/storageBackupsV2/__tests__/platformBlueprint.test.ts` | Platform blueprint |
| `frontend-modern/src/features/storageBackupsV2/__tests__/backupAdapters.test.ts` | Backup adapters |

Gap: No contract tests for identity merge across mixed-origin IDs, no architecture guardrails for duplicated helpers.
