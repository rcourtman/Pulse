# Settings Control Plane Decomposition Progress Tracker

Linked plan:
- `docs/architecture/settings-control-plane-decomposition-plan-2026-02.md`

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

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| 00 | Surface Inventory and Decomposition Cut-Map | DONE | Codex | Claude | APPROVED | See Packet 00 evidence below |
| 01 | Tab Schema and Metadata Extraction | DONE | Codex | Claude | APPROVED | See Packet 01 evidence below |
| 02 | Feature Gate Engine Extraction | DONE | Codex | Claude | APPROVED | See Packet 02 evidence below |
| 03 | Navigation and Deep-Link Orchestration Extraction | TODO | Unassigned | Unassigned | PENDING | |
| 04 | System Settings State Slice Extraction | TODO | Unassigned | Unassigned | PENDING | |
| 05 | Infrastructure and Node Workflow Extraction | TODO | Unassigned | Unassigned | PENDING | |
| 06 | Backup Import/Export and Passphrase Flow Extraction | TODO | Unassigned | Unassigned | PENDING | |
| 07 | Panel Registry and Render Dispatch Extraction | TODO | Unassigned | Unassigned | PENDING | |
| 08 | Contract Test Hardening (Settings Routing + Gates) | TODO | Unassigned | Unassigned | PENDING | |
| 09 | Architecture Guardrails for Settings Monolith Regression | TODO | Unassigned | Unassigned | PENDING | |
| 10 | Final Certification | TODO | Unassigned | Unassigned | PENDING | |

## Packet 00 Checklist: Surface Inventory and Decomposition Cut-Map

### Discovery
- [x] Tab schema/meta ownership inventory completed.
- [x] Route/deep-link/legacy redirect inventory completed with anchors.
- [x] State cluster inventory completed (system, infra, backup transfer, modal state).
- [x] High-risk flows identified and mapped.

### Deliverables
- [x] Plan appendix inventory updated.
- [x] Risk register packet mappings validated.
- [x] Rollback notes included for high-severity risks.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 00 Review Evidence

```
Files changed:
- docs/architecture/settings-control-plane-decomposition-plan-2026-02.md: Appended Appendices C (Tab Schema Inventory), D (Route/Deep-Link/Redirect Inventory with D1-D4 subsections), E (State Cluster Inventory with 6 clusters), F (High-Risk Flow Register with packet mapping and rollback notes)
- docs/architecture/settings-control-plane-decomposition-progress-2026-02.md: Updated Packet 00 status, checklist, and evidence

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 tests passed)

Gate checklist:
- P0: PASS (plan doc updated with 4 appendices; no source code modified; both required commands rerun by reviewer with exit 0)
- P1: PASS (discovery/documentation-only packet; no behavioral changes; line-number spot-checks verified against source for tabFeatureRequirements L886-894, isFeatureLocked/isTabLocked L896-905, baseTabGroups L907+, URL sync effect L458-517, deriveTabFromPath L30-102, deriveAgentFromPath L105-107)
- P2: PASS (progress tracker updated; checklist complete; evidence recorded)

Verdict: APPROVED

Commit:
- `2418cfeb` (docs(settings): Packet 00 — surface inventory and decomposition cut-map)

Residual risk:
- None. Discovery-only packet with no source code changes.

Rollback:
- Revert appendices C-F from the plan doc (delete content after Appendix B line 399).
- Reset progress tracker Packet 00 row and checklist to initial state.
```

## Packet 01 Checklist: Tab Schema and Metadata Extraction

### Implementation
- [x] Tab type/schema extracted to dedicated modules.
- [x] Header metadata extracted to dedicated module.
- [x] Nav ordering and labels preserved.
- [x] `Settings.tsx` no longer owns inline schema/meta constants.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 01 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/settingsTypes.ts (new): SettingsTab re-export, SettingsNavItem, SettingsNavGroup, SettingsHeaderMeta types
- frontend-modern/src/components/Settings/settingsTabs.ts (new): baseTabGroups array with all 6 groups, 23 tabs, same ordering/labels/icons/features
- frontend-modern/src/components/Settings/settingsHeaderMeta.ts (new): SETTINGS_HEADER_META map with all 23 tab entries
- frontend-modern/src/components/Settings/Settings.tsx: Removed inline definitions, now imports from new modules

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 passed)

Gate checklist:
- P0: PASS (all 4 files verified; both commands rerun by reviewer with exit 0)
- P1: PASS (tab ordering, labels, icons, features all preserved; Settings.tsx imports only, no inline definitions remain)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Commit:
- `d84f747a` (feat(settings): Packet 01 — extract tab schema and header metadata)

Residual risk:
- None

Rollback:
- Delete settingsTypes.ts, settingsTabs.ts, settingsHeaderMeta.ts
- Restore inline definitions in Settings.tsx from checkpoint commit 2418cfeb
```

## Packet 02 Checklist: Feature Gate Engine Extraction

### Implementation
- [x] Gate decisions extracted to helper module.
- [x] Multi-tenant visibility behavior preserved.
- [x] License lock behavior preserved.
- [x] Notification/fallback behavior preserved.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 02 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/settingsFeatureGates.ts (new): tabFeatureRequirements, isFeatureLocked(features, hasFeature, licenseLoaded), isTabLocked(tab, hasFeature, licenseLoaded)
- frontend-modern/src/components/Settings/Settings.tsx: Removed inline gate definitions, imports from settingsFeatureGates.ts, uses thin wrappers injecting license store

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 passed)

Gate checklist:
- P0: PASS (new file verified, Settings.tsx imports confirmed, both commands rerun with exit 0)
- P1: PASS (gate logic identical, parameter injection pattern enables testability, multi-tenant/lock/fallback behavior preserved)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- None

Rollback:
- Delete settingsFeatureGates.ts, restore inline gate logic from d84f747a
```

## Packet 03 Checklist: Navigation and Deep-Link Orchestration Extraction

### Implementation
- [ ] URL sync/tab activation extracted to helper hook.
- [ ] Legacy redirects extracted and preserved.
- [ ] Canonical tab mapping behavior preserved.
- [ ] `/settings` landing behavior remains no-flicker.

### Required Tests
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 04 Checklist: System Settings State Slice Extraction

### Implementation
- [ ] System settings state extracted to dedicated hook(s).
- [ ] Backup polling state and summaries extracted.
- [ ] Save/load payload semantics preserved.
- [ ] Error/notification behavior preserved.

### Required Tests
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 05 Checklist: Infrastructure and Node Workflow Extraction

### Implementation
- [ ] Node orchestration flow extracted to dedicated hook(s).
- [ ] Agent-specific behaviors preserved (PVE/PBS/PMG).
- [ ] Mutation/refresh semantics preserved.
- [ ] Existing node-related panel contracts preserved.

### Required Tests
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/UnifiedAgents.test.tsx` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 06 Checklist: Backup Import/Export and Passphrase Flow Extraction

### Implementation
- [ ] Import/export request flow extracted.
- [ ] Passphrase modal state machine extracted.
- [ ] Validation and warning semantics preserved.
- [ ] Success/error behavior preserved.

### Required Tests
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/SuggestProfileModal.test.tsx` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 07 Checklist: Panel Registry and Render Dispatch Extraction

### Implementation
- [ ] Panel registry introduced and used for dispatch.
- [ ] Inline render chain reduced substantially.
- [ ] Panel prop contracts preserved.
- [ ] Accessibility and nav behavior preserved.

### Required Tests
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 08 Checklist: Contract Test Hardening (Settings Routing + Gates)

### Implementation
- [ ] Canonical path mapping contract cases added/updated.
- [ ] Legacy alias/redirect contract cases added/updated.
- [ ] Organization route contract cases added/updated.
- [ ] Feature-gated path contract cases added/updated.

### Required Tests
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 09 Checklist: Architecture Guardrails for Settings Monolith Regression

### Implementation
- [ ] Architecture guard tests added for externalized modules.
- [ ] Guardrails are low-noise and not brittle.
- [ ] Exceptions policy documented.
- [ ] CI behavior validated.

### Required Tests
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsArchitecture.test.ts src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `go build ./...` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

## Packet 10 Checklist: Final Certification

### Certification
- [ ] Global validation baseline completed.
- [ ] Before/after module ownership map attached.
- [ ] Residual risk and rollback notes documented.
- [ ] Progress tracker state fully reconciled.

### Required Tests
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts src/components/Settings/__tests__/UnifiedAgents.test.tsx src/components/Settings/__tests__/SuggestProfileModal.test.tsx` passed.
- [ ] `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` passed.
- [ ] `go build ./...` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`
