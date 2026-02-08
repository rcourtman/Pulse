# Storage + Backups Phase 5 Legacy Deprecation Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md`

## Intent

Complete the remaining Storage/Backups follow-on work by retiring legacy route/shell wiring after GA stabilization, without regressing canonical URL contracts or cross-surface behavior.

This lane explicitly closes the remaining Storage/Backups debt recorded as `DL-001` in closeout.

## Authoritative Inputs

1. `docs/architecture/storage-page-ga-hardening-plan-2026-02.md`
2. `docs/architecture/storage-page-ga-hardening-progress-2026-02.md`
3. `docs/architecture/storage-backups-v2-plan.md`
4. `docs/architecture/program-closeout-certification-plan-2026-02.md` (Debt Ledger: `DL-001`)
5. `docs/architecture/delegation-review-rubric.md`
6. `CLAUDE.md` (delegation packet sizing + review gate policy)

## Code-Derived Baseline (Verified)

### A. Canonical and alias route wiring is still dual-capable

- App route wiring has both canonical and V2 alias routes:
  - `frontend-modern/src/App.tsx` (`/storage`, `/storage-v2`, `/backups`, `/backups-v2`)
- Routing mode decision still supports 4 modes:
  - `frontend-modern/src/routing/storageBackupsMode.ts`
  - `legacy-default`, `backups-v2-default`, `storage-v2-default`, `v2-default`

### B. Legacy shells still exist and still consume legacy adapter surface

- Storage legacy shell:
  - `frontend-modern/src/components/Storage/Storage.tsx`
  - uses `useResourcesAsLegacy`
- Backups legacy shell:
  - `frontend-modern/src/components/Backups/UnifiedBackups.tsx`
  - uses `useResourcesAsLegacy`

### C. Critical constraint: legacy adapter cannot be globally removed in this lane

- `useAlertsResources()` and `useAIChatResources()` are thin wrappers over `useResourcesAsLegacy()`:
  - `frontend-modern/src/hooks/useResources.ts`
  - consumed by:
    - `frontend-modern/src/pages/Alerts.tsx`
    - `frontend-modern/src/components/AI/Chat/index.tsx`

Conclusion:
- Phase 5 can remove Storage/Backups legacy route/shell usage.
- Phase 5 must not remove `useResourcesAsLegacy` itself.

## Non-Negotiable Contracts

1. URL contract:
- `/storage` and `/backups` remain canonical stable entry points.
- `q` remains canonical search param; `search` legacy alias remains parse-compatible where required.
- Deep-link params remain compatible: `tab`, `group`, `source`, `status`, `node`, `resource`, `sort`, `order`, `backupType`, `namespace`.

2. Behavior contract:
- No regressions in filter sync, canonicalization, route redirects, or tab highlighting.
- `BackupsV2` and `StorageV2` preserve canonical query-sync behavior when served via `/backups` and `/storage`.

3. Scope contract:
- Do not modify Alerts or AI migration behavior in this lane.
- `useResourcesAsLegacy` remains in place for non-Storage/Backups consumers.

4. Deletion safety contract:
- No legacy file deletion until integration and regression gates are green.
- Deletion packets are deletion-only (no concurrent rewiring).

5. Review and evidence contract:
- Follow fail-closed gates from rubric and `CLAUDE.md`.
- No packet may be marked `APPROVED` without explicit rerun exit-code evidence.

## Packet Sizing Policy (Lane-Specific)

Hard limits per delegated packet:
1. Max 1 change shape.
2. Max 1 subsystem boundary (frontend + docs only in this lane).
3. Max 3-5 files touched target.
4. Max 2 required validation commands in packet.

Required sequencing:
1. Discovery + scaffold
2. Integration/wiring
3. Test hardening + cleanup
4. Deletion
5. Final certification

## Risk Register

| ID | Severity | Risk | Owner Packet | Mitigation | Rollback |
|---|---|---|---|---|---|
| SB5-001 | High | Route-mode simplification changes redirect behavior unexpectedly. | SB5-01, SB5-02 | Keep mode decisions centralized with explicit test matrix. | Revert routing helper + App wiring commits. |
| SB5-002 | High | Canonical `/storage` or `/backups` query sync behavior regresses while decoupling legacy shells. | SB5-03, SB5-04 | Keep route/query contract tests mandatory per packet. | Revert shell-specific packet commit and restore previous route effects. |
| SB5-003 | High | Legacy deletion removes code paths still needed for rollback flags. | SB5-05 | Gate deletion behind prior green integrations and explicit route matrix evidence. | Revert deletion packet only. |
| SB5-004 | Medium | Navigation/tab metadata drifts from route behavior (`storage-v2`, `backups-v2`). | SB5-01, SB5-05 | Keep `platformTabs` and `navigation` tests in each relevant packet. | Revert metadata change set only. |
| SB5-005 | Medium | Attempted global `useResourcesAsLegacy` removal breaks Alerts/AI consumers. | Scope fence | Explicitly out-of-scope in SB5; preserve wrappers unchanged. | N/A (prohibited by scope). |

## Global Validation Baseline

Run at milestone boundaries (`SB5-02`, `SB5-05`, `SB5-06`):

1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run`

Notes:
- In-packet required checks remain capped at 2 commands.
- Milestone baselines are in addition to packet checks and require explicit exit codes.

## Packet Plan

### SB5-00: Discovery and Scope Freeze (Scaffold Only)

Objective:
- Freeze exact deprecation scope and deletion prerequisites.

Scope:
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-plan-2026-02.md`
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md`

Implementation checklist:
1. Record route/shell inventory and explicit keep/remove candidates.
2. Document non-goal: do not remove `useResourcesAsLegacy`.
3. Define packet-level acceptance commands and rollback notes.

Required commands:
1. None (docs-only packet).

Exit criteria:
- Scope boundaries are explicit and reviewer-approved.

### SB5-01: Routing Mode Contract Hardening (No Shell Changes)

Objective:
- Simplify and lock routing-mode semantics before shell decoupling.

Scope:
- `frontend-modern/src/routing/storageBackupsMode.ts`
- `frontend-modern/src/routing/__tests__/storageBackupsMode.test.ts`
- `frontend-modern/src/routing/platformTabs.ts`
- `frontend-modern/src/routing/__tests__/platformTabs.test.ts`
- `frontend-modern/src/routing/__tests__/navigation.test.ts`

Implementation checklist:
1. Keep canonical route decision model deterministic.
2. Remove ambiguous/dead mode branches only if covered by tests.
3. Preserve legacy-compat tab behavior where still required for remaining packets.

Required commands:
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Routing mode matrix is explicit, deterministic, and test-locked.

### SB5-02: App Router Integration Wiring

Objective:
- Integrate SB5-01 routing decisions into App route handlers without deleting legacy shells.

Scope:
- `frontend-modern/src/App.tsx`
- `frontend-modern/src/routing/storageBackupsMode.ts` (if helper extraction is needed)
- `frontend-modern/src/routing/__tests__/storageBackupsMode.test.ts`
- `frontend-modern/src/routing/__tests__/platformTabs.test.ts`

Implementation checklist:
1. Keep `/storage` and `/backups` canonical.
2. Keep `/storage-v2` and `/backups-v2` behavior explicit (redirect or serve) per mode contract.
3. Do not delete `Storage.tsx` or `UnifiedBackups.tsx` in this packet.

Required commands:
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/resourceLinks.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- App route behavior matches hardened routing contract with no shell deletion.

### SB5-03: Backups Legacy Shell Decoupling

Objective:
- Ensure canonical Backups behavior is fully V2-ready; legacy shell becomes compatibility-only.

Scope:
- `frontend-modern/src/components/Backups/BackupsV2.tsx`
- `frontend-modern/src/components/Backups/UnifiedBackups.tsx`
- `frontend-modern/src/components/Backups/__tests__/BackupsV2.test.tsx`
- `frontend-modern/src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx`

Implementation checklist:
1. Preserve `/backups` query canonicalization on `BackupsV2`.
2. Narrow `UnifiedBackups` responsibilities to explicit compatibility behavior.
3. Capture any residual fallback dependency that blocks deletion.

Required commands:
1. `cd frontend-modern && npx vitest run src/components/Backups/__tests__/BackupsV2.test.tsx src/components/Backups/__tests__/UnifiedBackups.routing.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Backups V2 canonical path behavior is independent and fully validated.

### SB5-04: Storage Legacy Shell Decoupling

Objective:
- Ensure canonical Storage behavior is fully V2-ready; legacy shell becomes compatibility-only.

Scope:
- `frontend-modern/src/components/Storage/StorageV2.tsx`
- `frontend-modern/src/components/Storage/Storage.tsx`
- `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`
- `frontend-modern/src/components/Storage/__tests__/Storage.routing.test.tsx`

Implementation checklist:
1. Preserve `/storage` query canonicalization and filter semantics.
2. Narrow `Storage.tsx` to compatibility behavior until deletion packet.
3. Capture residual blocking dependencies, if any.

Required commands:
1. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Storage V2 canonical path behavior is independent and fully validated.

### SB5-05: Legacy Route and Shell Deletion (Deletion Only)

Objective:
- Remove legacy Storage/Backups route/shell wiring that is proven unused by canonical flow.

Scope:
- `frontend-modern/src/App.tsx`
- `frontend-modern/src/routing/platformTabs.ts`
- `frontend-modern/src/routing/navigation.ts`
- `frontend-modern/src/components/Storage/Storage.tsx` (delete or isolate if still needed)
- `frontend-modern/src/components/Backups/UnifiedBackups.tsx` (delete or isolate if still needed)

Implementation checklist:
1. Delete only code proven unused by SB5-02/03/04 evidence.
2. Keep compatibility constraints for Alerts/AI (`useResourcesAsLegacy`) untouched.
3. Update/trim tests for removed routes/tabs/components.

Required commands:
1. `cd frontend-modern && npx vitest run src/routing/__tests__/storageBackupsMode.test.ts src/routing/__tests__/platformTabs.test.ts src/routing/__tests__/navigation.test.ts src/components/Storage/__tests__/StorageV2.test.tsx src/components/Backups/__tests__/BackupsV2.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- No active route/tab depends on removed legacy Storage/Backups shells.

### SB5-06: Final Certification and DL-001 Closure

Objective:
- Certify SB5 lane and close debt ledger item `DL-001`.

Scope:
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md`
- `docs/architecture/program-closeout-certification-plan-2026-02.md` (DL-001 update)
- Any required route/status docs touched by packet evidence

Implementation checklist:
1. Verify SB5-00..SB5-05 all `DONE/APPROVED` with checkpoint hashes.
2. Run final frontend validation gate.
3. Update debt ledger entry `DL-001` to `CLOSED` with evidence.

Required commands:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run`

Exit criteria:
- SB5 lane approved, debt item closed, residual risks documented.

## Explicit Out-of-Scope Follow-On

The following is intentionally deferred beyond SB5:

1. Global removal of `useResourcesAsLegacy`.
2. Alerts/AI migration to eliminate `useAlertsResources` and `useAIChatResources` wrappers.
3. WebSocket legacy payload retirement outside Storage/Backups surfaces.
