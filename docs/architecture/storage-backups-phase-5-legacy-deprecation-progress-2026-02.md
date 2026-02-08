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
| SB5-02 | App Router Integration Wiring | TODO | Codex | Claude | — | — |
| SB5-03 | Backups Legacy Shell Decoupling | TODO | Codex | Claude | — | — |
| SB5-04 | Storage Legacy Shell Decoupling | TODO | Codex | Claude | — | — |
| SB5-05 | Legacy Route and Shell Deletion | TODO | Codex | Claude | — | — |
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
- [ ] SB5-01 routing decisions integrated into App route handlers.
- [ ] `/storage` and `/backups` canonical routes preserved.
- [ ] `/storage-v2` and `/backups-v2` behavior explicit (redirect or serve) per mode contract.
- [ ] No deletion of `Storage.tsx` or `UnifiedBackups.tsx` in this packet.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/resourceLinks.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

---

## SB5-03 Checklist: Backups Legacy Shell Decoupling

### Implementation
- [ ] `/backups` query canonicalization preserved on `BackupsV2`.
- [ ] `UnifiedBackups` responsibilities narrowed to explicit compatibility behavior.
- [ ] Residual fallback dependencies that block deletion captured.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Backups/__tests__/BackupsV2.test.tsx src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

---

## SB5-04 Checklist: Storage Legacy Shell Decoupling

### Implementation
- [ ] `/storage` query canonicalization and filter semantics preserved.
- [ ] `Storage.tsx` narrowed to compatibility behavior until deletion packet.
- [ ] Residual blocking dependencies captured.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

---

## SB5-05 Checklist: Legacy Route and Shell Deletion

### Implementation
- [ ] Only code proven unused by SB5-02/03/04 evidence deleted.
- [ ] Alerts/AI compatibility (`useResourcesAsLegacy`) untouched.
- [ ] Tests updated/trimmed for removed routes/tabs/components.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts src/components/Storage/__tests__/StorageV2.test.tsx src/components/Backups/__tests__/BackupsV2.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

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
