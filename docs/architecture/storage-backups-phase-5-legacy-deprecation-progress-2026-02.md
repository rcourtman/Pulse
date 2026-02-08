# Storage + Backups Phase 5: Legacy Deprecation Progress Tracker

Linked plan:
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-plan-2026-02.md` (authoritative execution spec)
- `docs/architecture/storage-backups-v2-plan.md` (Phase 5: Route switch + deprecation — strategic north star)
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md` (predecessor — all packets DONE/APPROVED)
- `docs/architecture/program-closeout-certification-plan-2026-02.md` (DL-001 origin)

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

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| SB5-00 | Legacy Artifact Discovery and Removal Plan | DONE | Codex + Claude | Claude | APPROVED | See SB5-00 Review Evidence |
| SB5-01 | Routing Mode Contract Hardening | DONE | Codex | Claude | APPROVED | See SB5-01 Review Evidence |
| SB5-02 | App Router Integration Wiring | DONE | Codex | Claude | APPROVED | See SB5-02 Review Evidence |
| SB5-03 | Backups Legacy Shell Decoupling | DONE | Codex | Claude | APPROVED | See SB5-03 Review Evidence |
| SB5-04 | Storage Legacy Shell Decoupling | DONE | Codex | Claude | APPROVED | See SB5-04 Review Evidence |
| SB5-05 | Legacy Route and Shell Deletion | DONE | Codex | Claude | APPROVED | See SB5-05 Review Evidence |
| SB5-06 | Final Certification and DL-001 Closure | TODO | Claude | Claude | — | — |

---

## SB5-00: Legacy Artifact Discovery and Removal Plan

### Discovery Checklist
- [x] Legacy frontend page shells identified with file paths.
- [x] Legacy backend API endpoints identified and classified.
- [x] Feature flag/toggle infrastructure inventoried.
- [x] Legacy-only test files identified.
- [x] Shared infrastructure dependencies mapped (not removable).
- [x] Packetized removal sequence produced with acceptance checks.

### Required Tests
- [x] `go build ./...` → exit 0 (no modifications made)
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0 (no modifications made)

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-00 Review Evidence

```
Files changed:
- docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md: Created progress tracker with inventory and removal plan

Commands run + exit codes:
1. `go build ./...` → exit 0 (Codex evidence, verified by reviewer)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0 (Codex evidence, verified by reviewer)

Gate checklist:
- P0: PASS (comprehensive inventory produced with file anchors; no code modifications; both validation commands pass)
- P1: PASS (dependency analysis correctly identifies non-removable shared infrastructure)
- P2: PASS (progress tracker created, packetized removal sequence defined)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete this progress tracker file.
```

---

## Appendix A: Legacy Artifact Inventory

### Category A: Pure Legacy — Safely Removable

These artifacts serve ONLY the legacy pages and have no V2 or shared-system consumers.

#### A.1 Legacy Page Shells

| File | Description | LOC (approx) | Consumers |
|---|---|---|---|
| `frontend-modern/src/components/Storage/Storage.tsx` | Legacy storage page shell | ~1645 | App.tsx `StorageRoute` only |
| `frontend-modern/src/components/Backups/UnifiedBackups.tsx` | Legacy backups page shell | ~600 | App.tsx `BackupsRoute` only |

#### A.2 Feature Flag / Toggle Infrastructure

| File | Artifact | Description |
|---|---|---|
| `frontend-modern/src/utils/featureFlags.ts` | Entire file | `isStorageBackupsV2Enabled()`, `isBackupsV2RolledBack()`, `isStorageV2RolledBack()` |
| `frontend-modern/src/routing/storageBackupsMode.ts` | Entire file | `StorageBackupsDefaultMode`, `StorageBackupsRoutingPlan`, `resolveStorageBackupsRoutingPlan()`, `buildStorageBackupsRoutingPlan()`, rollback resolution |
| `frontend-modern/src/utils/localStorage.ts` | 3 keys | `STORAGE_BACKUPS_V2_ENABLED`, `STORAGE_V2_ROLLBACK`, `BACKUPS_V2_ROLLBACK` |
| `frontend-modern/src/App.tsx` | Routing wrappers | `StorageRoute`, `BackupsRoute`, `StorageV2Route`, `BackupsV2Route` wrapper components; `storageBackupsRoutingPlan` memo (lines ~978-1024, ~1401-1406) |

#### A.3 App.tsx Dual Route Registrations

| Route | Current Behavior | Post-Removal Target |
|---|---|---|
| `/storage` | Renders legacy or V2 depending on routing plan | Always renders `StorageV2` directly |
| `/storage-v2` | Redirects to `/storage` if V2 is default, else renders V2 | Remove entirely (V2 is `/storage`) |
| `/backups` | Renders legacy or V2 depending on routing plan | Always renders `BackupsV2` directly |
| `/backups-v2` | Redirects to `/backups` if V2 is default, else renders V2 | Remove entirely (V2 is `/backups`) |

#### A.4 Legacy-Only Test Files

| File | Description | Action |
|---|---|---|
| `frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx` | Legacy `/storage` routing contract | Delete (V2 routing tests in `StorageV2.test.tsx` cover V2 contract) |
| `frontend-modern/src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx` | Legacy `/backups` routing contract | Delete (V2 routing tested separately) |
| `frontend-modern/src/components/Backups/__tests__/PBSEnhancementBanner.test.ts` | Legacy PBS passthrough banner | Delete |
| `frontend-modern/src/routing/__tests__/storageBackupsMode.test.ts` | V2 mode/plan resolution tests | Delete (mode model itself removed) |

#### A.5 Platform Tabs Dual-Mode Logic

| File | Artifact | Description |
|---|---|---|
| `frontend-modern/src/routing/platformTabs.ts` | `buildStorageBackupsTabSpecs()` | Dual-branch tab generation based on `StorageBackupsRoutingPlan`; legacy branches produce `(Legacy)` labels and extra V2 tabs |
| `frontend-modern/src/routing/platformTabs.ts` | Type `StorageBackupsTabSpec` | IDs `storage-v2` and `backups-v2` become dead after removal |

#### A.6 V2 Route Path Constants and Builders

| File | Artifact | Description |
|---|---|---|
| `frontend-modern/src/routing/resourceLinks.ts:9` | `STORAGE_V2_PATH` | `/storage-v2` constant |
| `frontend-modern/src/routing/resourceLinks.ts:10` | `BACKUPS_V2_PATH` | `/backups-v2` constant |
| `frontend-modern/src/routing/resourceLinks.ts:200` | `buildStorageV2Path()` | Builder for `/storage-v2?...` |
| `frontend-modern/src/routing/resourceLinks.ts:265` | `buildBackupsV2Path()` | Builder for `/backups-v2?...` |

Consumers of V2 paths (must be updated in SB5-04):
- `StorageV2.tsx:195` — `isActiveStorageRoute()` checks `STORAGE_V2_PATH`
- `BackupsV2.tsx:368` — `isActiveBackupsRoute()` checks `BACKUPS_V2_PATH`
- `platformTabs.ts:57,83` — tab route values
- `platformTabs.test.ts` — test assertions
- `resourceLinks.test.ts:189-190` — builder tests

### Category B: Shared Infrastructure — NOT Removable in Phase 5

These artifacts are consumed by non-storage/backups systems and must remain.

| Artifact | File | Active Consumers | Reason |
|---|---|---|---|
| `useResourcesAsLegacy()` hook | `frontend-modern/src/hooks/useResources.ts:275` | `useAlertsResources()` → `Alerts.tsx`, `useAIChatResources()` → `AI/Chat/index.tsx` | Alerts and AI chat still depend on legacy resource conversion |
| Legacy websocket state arrays (`state.storage`, `state.backups`, etc.) | `frontend-modern/src/stores/websocket.ts` | `useResourcesAsLegacy()`, V2 adapter fallback paths | Live data pipeline for alerts/AI; V2 fallback for partial unified-resource coverage |
| Legacy backend API endpoints (`/api/storage/`, `/api/backups`, etc.) | `internal/api/router_routes_monitoring.go:17-31` | External API consumers, possibly AI tools | Out of scope per storage-backups-v2-plan.md |
| Backend legacy adapter/contract | `internal/unifiedresources/legacy_adapter.go`, `legacy_contract.go` | `/api/resources` shape, websocket, AI tools | Serves non-storage consumers |

### Category C: Assess Before Removal — Deferred

| Artifact | File | Concern |
|---|---|---|
| V2 adapter legacy fallback paths (`legacy-storage`, `legacy-pbs-datastore`) | `storageAdapters.ts:320-344` | Provides data for sources not yet in unified resources; removal could cause data gaps |
| V2 adapter legacy backup paths (`legacy-pve-snapshots`, etc.) | `backupAdapters.ts:998-1048` | Same concern — fallback data coverage |
| Legacy origin precedence | `models.ts:26-30` | Tied to adapter legacy paths |
| Backend system settings V2 toggle | `internal/config/config.go` (if exists) | Needs investigation; may be server-side gate for `disableLegacyRouteRedirects` (different concern) |

---

## Appendix B: Packetized Removal Sequence

> Note: This appendix mirrors the authoritative packet definitions in
> `docs/architecture/storage-backups-phase-5-legacy-deprecation-plan-2026-02.md`.
> If the two diverge, the plan file is authoritative.

### SB5-01: Routing Mode Contract Hardening (No Shell Changes) — DONE

**Change shape:** test hardening + contract annotation
**Subsystem boundary:** frontend routing
**Status:** APPROVED (commit `2967c0de`)

Simplified and test-locked routing-mode semantics. Added GA contract regression gates
(10 new tests) and `@deprecated` annotations on rollback-only mode branches and alias
tab IDs. No runtime behavior changes.

### SB5-02: App Router Integration Wiring

**Change shape:** integration/wiring
**Subsystem boundary:** frontend routing + App shell
**Target files (3-5):**
1. `frontend-modern/src/App.tsx` — EDIT: integrate hardened routing decisions
2. `frontend-modern/src/routing/storageBackupsMode.ts` — EDIT (if helper extraction needed)
3. `frontend-modern/src/routing/__tests__/storageBackupsMode.test.ts` — EDIT (if test updates needed)
4. `frontend-modern/src/routing/__tests__/platformTabs.test.ts` — EDIT (if test updates needed)

**Constraints:**
- Keep `/storage` and `/backups` canonical.
- Keep `/storage-v2` and `/backups-v2` behavior explicit (redirect or serve) per mode contract.
- Do NOT delete `Storage.tsx` or `UnifiedBackups.tsx` in this packet.
- Do NOT remove `useResourcesAsLegacy` or feature-flag infrastructure.

**Acceptance checks:**
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/resourceLinks.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

### SB5-03: Backups Legacy Shell Decoupling

**Change shape:** integration/wiring
**Subsystem boundary:** frontend components (Backups)
**Target files (3-4):**
1. `frontend-modern/src/components/Backups/BackupsV2.tsx`
2. `frontend-modern/src/components/Backups/UnifiedBackups.tsx`
3. `frontend-modern/src/components/Backups/__tests__/BackupsV2.test.tsx`
4. `frontend-modern/src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx`

**Constraints:**
- Preserve `/backups` query canonicalization on BackupsV2.
- Narrow UnifiedBackups responsibilities to explicit compatibility behavior.
- Do NOT delete UnifiedBackups.tsx in this packet.

**Acceptance checks:**
1. `cd frontend-modern && npx vitest run src/components/Backups/__tests__/BackupsV2.test.tsx src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

### SB5-04: Storage Legacy Shell Decoupling

**Change shape:** integration/wiring
**Subsystem boundary:** frontend components (Storage)
**Target files (3-4):**
1. `frontend-modern/src/components/Storage/StorageV2.tsx`
2. `frontend-modern/src/components/Storage/Storage.tsx`
3. `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`
4. `frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx`

**Constraints:**
- Preserve `/storage` query canonicalization and filter semantics.
- Narrow Storage.tsx to compatibility behavior.
- Do NOT delete Storage.tsx in this packet.

**Acceptance checks:**
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

### SB5-05: Legacy Route and Shell Deletion (Deletion Only)

**Change shape:** deletion
**Subsystem boundary:** frontend routing + components
**Target files (5+):**
1. `frontend-modern/src/App.tsx` — EDIT: remove legacy shell imports and route wrappers
2. `frontend-modern/src/routing/platformTabs.ts` — EDIT: simplify tabs
3. `frontend-modern/src/routing/navigation.ts` — EDIT: consolidate alias tab IDs
4. `frontend-modern/src/components/Storage/Storage.tsx` — DELETE (or isolate)
5. `frontend-modern/src/components/Backups/UnifiedBackups.tsx` — DELETE (or isolate)

**Constraints:**
- Delete only code proven unused by SB5-02/03/04 evidence.
- Keep `useResourcesAsLegacy` untouched (Alerts/AI consumers).
- Update/trim tests for removed routes/tabs/components.

**Acceptance checks:**
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts src/components/Storage/__tests__/StorageV2.test.tsx src/components/Backups/__tests__/BackupsV2.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

### SB5-06: Final Certification and DL-001 Closure

**Change shape:** docs only
**Subsystem boundary:** docs
**Target files (2-3):**
1. `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` — final verdict
2. `docs/architecture/storage-backups-v2-plan.md` — update status to Complete
3. `docs/architecture/program-closeout-certification-plan-2026-02.md` — DL-001 update

**Acceptance checks:**
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run`
3. `go build ./...`

---

## SB5-01 Checklist: Routing Mode Contract Hardening

### Implementation
- [x] `storageBackupsMode.ts` annotated with GA deprecation markers on rollback modes.
- [x] `storageBackupsMode.test.ts` GA contract regression gates added (4 tests).
- [x] `platformTabs.ts` annotated with deprecation markers on legacy branches and alias tab IDs.
- [x] `platformTabs.test.ts` GA contract regression gates added (3 tests).
- [x] `navigation.test.ts` alias path tab mapping tests added (3 tests).

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts` → exit 0 (28 tests passed).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0.
- [x] Exit codes recorded.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-01 Review Evidence

```
Files changed:
- frontend-modern/src/routing/storageBackupsMode.ts: Added GA state comment block and @deprecated annotations on legacy-default, backups-v2-default, storage-v2-default modes. No logic changes.
- frontend-modern/src/routing/__tests__/storageBackupsMode.test.ts: Added 'GA contract regression gates' describe block with 4 tests (default resolution, v2 primary views, V2 alias redirects, rollback isolation).
- frontend-modern/src/routing/platformTabs.ts: Added @deprecated annotations on storage-v2/backups-v2 tab IDs and legacy else-branches. Added boolean compat removal note. No logic changes.
- frontend-modern/src/routing/__tests__/platformTabs.test.ts: Added 'GA contract regression gates' describe block with 3 tests (v2-default tab count/routes, no legacy labels, tab identity stability).
- frontend-modern/src/routing/__tests__/navigation.test.ts: Added 'alias path tab mapping' describe block with 3 tests documenting /storage-v2 and /backups-v2 alias mappings scheduled for consolidation.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts` → exit 0 (3 files, 28 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0

Gate checklist:
- P0: PASS (all 5 files verified with expected edits; both commands rerun by reviewer with exit 0)
- P1: PASS (10 new GA contract regression tests lock v2-default production behavior; rollback isolation explicitly tested; no behavioral changes to runtime)
- P2: PASS (progress tracker updated with correct SB5-01 scope and plan-aligned packet sequence)

Verdict: APPROVED

Commit:
- `2967c0de` (refactor(storage-phase5): SB5-01 — routing mode contract hardening)

Residual risk:
- None. All changes are annotation + test-only. No runtime behavior was modified.

Rollback:
- Revert checkpoint commit; restore original JSDoc-free source files and original test files without GA contract blocks.
```

---

## SB5-02 Checklist: App Router Integration Wiring

### Implementation
- [x] SB5-01 routing decisions integrated into App route handlers — removed `storageBackupsRoutingPlan` memo, `StorageRoute`/`BackupsRoute` wrappers, feature-flag imports; hard-wired V2 components directly on canonical routes.
- [x] `/storage` and `/backups` canonical routes preserved — `/storage` → `StorageV2Component`, `/backups` → `BackupsV2Component`.
- [x] `/storage-v2` and `/backups-v2` always redirect to canonical paths — `StorageV2Route` and `BackupsV2Route` simplified to unconditional `<Navigate>`.
- [x] No deletion of `Storage.tsx` or `UnifiedBackups.tsx` in this packet — confirmed via grep: both files untouched.
- [x] Platform tabs simplified: `buildStorageBackupsTabSpecs(true)` (boolean compat path → v2-default tabs).

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/resourceLinks.test.ts` → exit 0 (3 files, 29 tests passed).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0.
- [x] Exit codes recorded.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-02 Review Evidence

```
Files changed:
- frontend-modern/src/App.tsx: Removed imports for resolveStorageBackupsRoutingPlan,
  shouldRedirectStorageV2Route, shouldRedirectBackupsV2Route (from storageBackupsMode),
  isStorageBackupsV2Enabled, isStorageV2RolledBack, isBackupsV2RolledBack (from featureFlags),
  and lazy imports for StorageComponent and UnifiedBackups. Removed storageBackupsRoutingPlan
  memo (App body and RootLayout). Removed StorageRoute/BackupsRoute conditional wrapper
  components. Simplified StorageV2Route/BackupsV2Route to unconditional <Navigate> redirects.
  Hard-wired /storage → StorageV2Component and /backups → BackupsV2Component directly.
  Platform tabs: buildStorageBackupsTabSpecs(true) instead of routing plan memo.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts
   src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/resourceLinks.test.ts`
   → exit 0 (3 files, 29 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
   → exit 0

Gate checklist:
- P0: PASS (only App.tsx modified; no out-of-scope files changed; both commands rerun by
  reviewer with exit 0; git diff confirms no stray changes)
- P1: PASS (canonical routes /storage and /backups serve V2 directly; alias routes
  /storage-v2 and /backups-v2 always redirect; all 29 routing tests pass; Storage.tsx
  and UnifiedBackups.tsx confirmed untouched; useResourcesAsLegacy not referenced in diff;
  feature-flag infrastructure files not modified)
- P2: PASS (progress tracker updated; packet scope matches plan SB5-02 definition;
  checklist items align to plan constraints)

Verdict: APPROVED

Commit:
- `8a9836e9` (refactor(storage-phase5): SB5-02 — hard-wire V2 routes in App router)
  - Note: `2ef31e6d` was superseded by amend that recorded the commit hash.

Residual risk:
- Storage.tsx and UnifiedBackups.tsx lazy imports removed from App.tsx but files still
  exist on disk. They are now unreachable dead code. Deletion is deferred to SB5-05
  per plan sequencing.

Rollback:
- Revert checkpoint commit to restore App.tsx with conditional routing wrappers and
  feature-flag-driven mode resolution.
```

---

## SB5-03 Checklist: Backups Legacy Shell Decoupling

### Implementation
- [x] `/backups` query canonicalization preserved on `BackupsV2` — simplified `isActiveBackupsRoute()` to canonical-only; switched query sync from `buildBackupsV2Path` to `buildBackupsPath`; removed `BACKUPS_V2_PATH` and `buildBackupsV2Path` imports.
- [x] `UnifiedBackups` responsibilities narrowed to explicit compatibility behavior — added file-level JSDoc deprecation comment documenting unrouted status (no App.tsx route since SB5-02) and SB5-05 deletion schedule.
- [x] Residual fallback dependencies that block deletion captured — `useResourcesAsLegacy` is the shared dependency (Alerts.tsx, AI/Chat); hook itself is NOT removable in Phase 5. UnifiedBackups.tsx file itself has no shared consumers and is safe for SB5-05 deletion.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/components/Backups/__tests__/BackupsV2.test.tsx src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx` → exit 0 (2 files, 11 tests passed).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0 on committed tree; exit 2 with parallel in-flight untracked files (`Dashboard.performance.contract.test.tsx`) which are out-of-scope. SB5-03 changes introduce zero type errors.
- [x] Exit codes recorded.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-03 Review Evidence

```
Files changed:
- frontend-modern/src/components/Backups/BackupsV2.tsx: Removed BACKUPS_V2_PATH and
  buildBackupsV2Path imports. Simplified isActiveBackupsRoute() to check only
  buildBackupsPath() (canonical). Switched query-sync managed path from
  buildBackupsV2Path() to buildBackupsPath(). No other changes.
- frontend-modern/src/components/Backups/UnifiedBackups.tsx: Added file-level JSDoc
  deprecation comment documenting unrouted status and SB5-05 deletion schedule.
  No logic changes.
- frontend-modern/src/components/Backups/__tests__/BackupsV2.test.tsx: Changed default
  mockLocationPath from '/backups-v2' to '/backups'. Added GA contract canonical-path
  test verifying BackupsV2 at /backups is the only canonical path.
- frontend-modern/src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx: Added
  JSDoc deprecation comment above describe block documenting legacy compatibility status
  and SB5-05 deletion schedule.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/components/Backups/__tests__/BackupsV2.test.tsx
   src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx`
   → exit 0 (2 files, 11 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
   → exit 0 on committed tree (verified via git stash round-trip)
   → exit 2 with untracked parallel-work files; errors are in
   Dashboard.performance.contract.test.tsx (untracked, out-of-scope)

Gate checklist:
- P0: PASS (only 4 in-scope files changed; git diff verified no out-of-scope
  modifications; both commands pass for in-scope code; parallel-work tsc failure
  documented and isolated)
- P1: PASS (/backups canonical query canonicalization preserved and tested;
  BackupsV2 fully decoupled from /backups-v2 alias semantics; UnifiedBackups
  logic untouched with deprecation annotation only; useResourcesAsLegacy not
  modified; no storage-related files changed; GA contract test added)
- P2: PASS (progress tracker updated; packet scope matches plan SB5-03 definition;
  checklist items align to plan constraints)

Verdict: APPROVED

Commit:
- `65d78e52` (refactor(storage-phase5): SB5-03 — decouple BackupsV2 from V2 alias path)

Residual risk:
- Parallel in-flight tsc failure in Dashboard.performance.contract.test.tsx is
  not caused by SB5 work and will resolve when that parallel session commits.
- UnifiedBackups.tsx is dead code (no route) but file persists on disk until SB5-05.

Rollback:
- Revert checkpoint commit to restore BackupsV2 dual-path awareness and remove
  deprecation annotations.
```

---

## SB5-04 Checklist: Storage Legacy Shell Decoupling

### Implementation
- [x] `/storage` query canonicalization and filter semantics preserved — simplified `isActiveStorageRoute()` to canonical-only; switched `useStorageRouteState` buildPath from `buildStorageV2Path` to `buildStoragePath`; removed `STORAGE_V2_PATH` and `buildStorageV2Path` imports.
- [x] `Storage.tsx` narrowed to compatibility behavior — added file-level JSDoc deprecation comment documenting unrouted status (no App.tsx route since SB5-02) and SB5-05 deletion schedule. No logic changes.
- [x] Residual blocking dependencies captured — `useResourcesAsLegacy` is the shared dependency (Alerts.tsx, AI/Chat); hook itself NOT removable in Phase 5. Storage.tsx file has no shared consumers and is safe for SB5-05 deletion.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx` → exit 0 (2 files, 18 tests passed).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0 (Codex evidence).
- [x] Exit codes recorded.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-04 Review Evidence

```
Files changed:
- frontend-modern/src/components/Storage/StorageV2.tsx: Removed STORAGE_V2_PATH and
  buildStorageV2Path imports, added buildStoragePath. Simplified isActiveStorageRoute()
  to canonical /storage only. Updated useStorageRouteState buildPath to buildStoragePath.
- frontend-modern/src/components/Storage/Storage.tsx: Added file-level JSDoc deprecation
  comment. No logic changes.
- frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx: Changed default
  mockLocationPath from '/storage-v2' to '/storage'. Added GA contract canonical-path test.
- frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx: Added JSDoc
  deprecation comment above describe block.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx
   src/components/Storage/__tests__/Storage.routing.test.tsx`
   → exit 0 (2 files, 18 tests passed) — independently verified by reviewer
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
   → exit 0 (Codex evidence)

Gate checklist:
- P0: PASS (only 4 in-scope files changed; vitest exit 0 independently verified)
- P1: PASS (/storage canonical query canonicalization preserved; StorageV2 decoupled
  from /storage-v2 alias; Storage.tsx logic untouched; useResourcesAsLegacy not modified;
  GA contract test added)
- P2: PASS (progress tracker updated; scope matches plan SB5-04 definition)

Verdict: APPROVED

Commit:
- `1992ff36` (refactor(storage-phase5): SB5-04 — decouple StorageV2 from V2 alias path)

Rollback:
- Revert checkpoint commit to restore StorageV2 dual-path awareness.
```

---

## SB5-05 Checklist: Legacy Route and Shell Deletion

### Implementation
- [x] Only code proven unused by SB5-02/03/04 evidence deleted — Storage.tsx, UnifiedBackups.tsx, 3 legacy test files deleted; App.tsx preloads and V2 alias routes removed; platformTabs.ts simplified to v2-default only; navigation.ts alias tab IDs removed.
- [x] Alerts/AI compatibility (`useResourcesAsLegacy`) untouched — verified not referenced in any diff.
- [x] Tests updated/trimmed for removed routes/tabs/components — platformTabs.test.ts simplified to 2 tests; navigation.test.ts alias block removed; legacy routing tests deleted with their shells.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts src/components/Storage/__tests__/StorageV2.test.tsx src/components/Backups/__tests__/BackupsV2.test.tsx` → exit 0 (5 files, 43 tests passed).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` → exit 0 (Codex evidence after App.tsx cleanup).
- [x] Exit codes recorded.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### SB5-05 Review Evidence

```
Files deleted:
- frontend-modern/src/components/Storage/Storage.tsx (legacy shell, unrouted since SB5-02)
- frontend-modern/src/components/Backups/UnifiedBackups.tsx (legacy shell, unrouted since SB5-02)
- frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx (legacy test)
- frontend-modern/src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx (legacy test)
- frontend-modern/src/components/Backups/__tests__/PBSEnhancementBanner.test.ts (legacy test)

Files modified:
- frontend-modern/src/App.tsx: Removed preload entries for deleted shells; removed
  STORAGE_V2_PATH/BACKUPS_V2_PATH imports; removed StorageV2Route/BackupsV2Route
  redirect components and their Route registrations.
- frontend-modern/src/routing/platformTabs.ts: Simplified to always return canonical
  2-tab set (storage + backups). Removed all legacy/rollback branches, StorageBackupsRoutingPlan
  import, and V2 alias tab IDs.
- frontend-modern/src/routing/navigation.ts: Removed storage-v2 and backups-v2 from
  AppTabId type and getActiveTabForPath cases.
- frontend-modern/src/routing/__tests__/platformTabs.test.ts: Simplified to 2 tests
  (canonical tabs + backward compat argument ignored).
- frontend-modern/src/routing/__tests__/navigation.test.ts: Removed alias path tab
  mapping describe block and storage-v2/backups-v2 assertions.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run [5 test files]` → exit 0 (43 tests passed)
   — independently verified by reviewer
2. `tsc --noEmit` → exit 0 (Codex evidence after App.tsx sub-delegation)

Gate checklist:
- P0: PASS (10 files changed total; all in plan scope; both commands pass;
  deletion targets proven unused by SB5-02/03/04 deprecation annotations)
- P1: PASS (canonical /storage and /backups routes intact; 43 tests pass;
  useResourcesAsLegacy not referenced in any diff; no active route depends
  on deleted shells)
- P2: PASS (progress tracker updated; scope matches plan SB5-05)

Verdict: APPROVED

Commit:
- PENDING

Rollback:
- Revert checkpoint commit to restore deleted shells, alias routes, and
  legacy tab/mode branches.
```

---

## SB5-06 Checklist: Final Certification and DL-001 Closure

### Certification
- [ ] All packets SB5-00 through SB5-05 are `DONE` and `APPROVED`.
- [ ] Checkpoint commit hashes recorded for each approved packet.
- [ ] Storage + Backups V2 plan status updated to Complete.
- [ ] Debt ledger DL-001 updated to `CLOSED` with evidence.

### Final Validation Gate
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `cd frontend-modern && npx vitest run` passed.
- [ ] `go build ./...` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded
