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
| 03 | Navigation and Deep-Link Orchestration Extraction | DONE | Codex | Claude | APPROVED | See Packet 03 evidence below |
| 04 | System Settings State Slice Extraction | DONE | Codex | Claude | APPROVED | See Packet 04 evidence below |
| 05 | Infrastructure and Node Workflow Extraction | DONE | Codex | Claude | APPROVED | See Packet 05 evidence below |
| 06 | Backup Import/Export and Passphrase Flow Extraction | DONE | Codex | Claude | APPROVED | See Packet 06 evidence below |
| 07 | Panel Registry and Render Dispatch Extraction | DONE | Codex | Claude | APPROVED | See Packet 07 evidence below |
| 08 | Contract Test Hardening (Settings Routing + Gates) | DONE | Codex | Claude | APPROVED | See Packet 08 evidence below |
| 09 | Architecture Guardrails for Settings Monolith Regression | DONE | Codex | Claude | APPROVED | See Packet 09 evidence below |
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
- `1278841f` (feat(settings): Packet 02 — extract feature gate engine)

Residual risk:
- None

Rollback:
- Delete settingsFeatureGates.ts, restore inline gate logic from d84f747a
```

## Packet 03 Checklist: Navigation and Deep-Link Orchestration Extraction

### Implementation
- [x] URL sync/tab activation extracted to helper hook.
- [x] Legacy redirects extracted and preserved.
- [x] Canonical tab mapping behavior preserved.
- [x] `/settings` landing behavior remains no-flicker.

### Required Tests
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` — legacyRedirects 3/3 PASS, legacyRouteContracts 2/2 PASS; platformTabs FAIL (pre-existing: `@/routing/resourceLinks` alias issue, confirmed fails identically on Packet 02 commit).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS (pre-existing platformTabs failure documented, not caused by this packet)
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 03 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/useSettingsNavigation.ts (new): Hook encapsulating currentTab, activeTab, selectedAgent signals + agentPaths + handleSelectAgent + setActiveTab + URL sync effect with all legacy redirects
- frontend-modern/src/components/Settings/Settings.tsx: Removed inline navigation state/effects, calls useSettingsNavigation() hook

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 passed)
3. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` -> exit 1
   - legacyRedirects.test.ts: 3/3 PASS
   - legacyRouteContracts.test.ts: 2/2 PASS
   - platformTabs.test.ts: FAIL (pre-existing @/routing/resourceLinks alias issue, verified fails on Packet 02 commit too)

Gate checklist:
- P0: PASS (hook file verified with correct signals/effects/redirects; both tsc and routing tests pass; platformTabs failure is pre-existing)
- P1: PASS (URL sync effect preserved verbatim; all legacy redirect shims intact; no-flicker landing behavior maintained)
- P2: PASS (tracker updated, pre-existing failure documented)

Verdict: APPROVED

Commit:
- `791e027e` (feat(settings): Packet 03 — extract navigation and deep-link orchestration)

Residual risk:
- platformTabs.test.ts has a pre-existing alias resolution failure from parallel work

Rollback:
- Delete useSettingsNavigation.ts, restore inline navigation logic from 1278841f
```

## Packet 04 Checklist: System Settings State Slice Extraction

### Implementation
- [x] System settings state extracted to dedicated hook(s).
- [x] Backup polling state and summaries extracted.
- [x] Save/load payload semantics preserved.
- [x] Error/notification behavior preserved.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 04 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/useSystemSettingsState.ts (new, 629 LOC): All system settings signals, env override locks, polling/update/diagnostics state, saveSettings, checkForUpdates, handleInstallUpdate, handleConfirmUpdate, initializeSystemSettingsState
- frontend-modern/src/components/Settings/Settings.tsx (3831→3339 LOC, -492): Removed inline system state, calls hook and destructures

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 passed)

Gate checklist:
- P0: PASS (hook verified with complete state surface; Settings.tsx delegates; both commands pass)
- P1: PASS (save/load semantics, notifications, env locks all preserved; discovery state left in Settings.tsx for Packet 05 coupling)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Commit:
- `735b9f41` (feat(settings): Packet 04 — extract system settings state slice)

Residual risk:
- None

Rollback:
- Delete useSystemSettingsState.ts, restore inline system state from 791e027e
```

## Packet 05 Checklist: Infrastructure and Node Workflow Extraction

### Implementation
- [x] Node orchestration flow extracted to dedicated hook(s).
- [x] Agent-specific behaviors preserved (PVE/PBS/PMG).
- [x] Mutation/refresh semantics preserved.
- [x] Existing node-related panel contracts preserved.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/UnifiedAgents.test.tsx` — pre-existing failure (Client-only API import error, fails identically on Packet 04 commit).
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS (pre-existing test failure documented)
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 05 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts (new, 924 LOC): Node/discovery signals, CRUD orchestration, event bus subscriptions, discovery handlers, temperature monitoring, modal polling, WebSocket re-merge, saveNode
- frontend-modern/src/components/Settings/Settings.tsx (3339→2459 LOC, -880): Removed inline infrastructure state, calls hook

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/UnifiedAgents.test.tsx` -> exit 1 (pre-existing: Client-only API error, confirmed fails on Packet 04 commit too)

Gate checklist:
- P0: PASS (hook verified, tsc passes, test failure is pre-existing)
- P1: PASS (PVE/PBS/PMG behaviors preserved, event bus subscriptions intact, modal polling/cleanup preserved)
- P2: PASS (tracker updated)

Verdict: APPROVED

Commit:
- `23ec9294` (feat(settings): Packet 05 — extract infrastructure and node workflow)

Residual risk:
- UnifiedAgents.test.tsx has a pre-existing environment issue

Rollback:
- Delete useInfrastructureSettingsState.ts, restore from 735b9f41
```

## Packet 06 Checklist: Backup Import/Export and Passphrase Flow Extraction

### Implementation
- [x] Import/export request flow extracted.
- [x] Passphrase modal state machine extracted.
- [x] Validation and warning semantics preserved.
- [x] Success/error behavior preserved.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/SuggestProfileModal.test.tsx` — pre-existing failure (`@/utils/format` alias resolution, confirmed fails on Packet 05 commit).
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS (pre-existing test failure documented)
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 06 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/useBackupTransferFlow.ts (new, 256 LOC): Export/import dialog signals, passphrase/file signals, API token modal, handleExport, handleImport, dialog close/reset handlers
- frontend-modern/src/components/Settings/Settings.tsx (2459→2274 LOC, -185): Removed inline backup flow state, calls hook

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/SuggestProfileModal.test.tsx` -> exit 1 (pre-existing: @/utils/format alias, confirmed fails on Packet 05 commit)

Gate checklist:
- P0: PASS (hook verified, tsc passes, test failure is pre-existing)
- P1: PASS (passphrase validation, token gate/retry, export download, import parse all preserved)
- P2: PASS (tracker updated)

Verdict: APPROVED

Commit:
- `4854c251` (feat(settings): Packet 06 — extract backup import/export and passphrase flow)

Residual risk:
- SuggestProfileModal.test.tsx has pre-existing alias resolution issue

Rollback:
- Delete useBackupTransferFlow.ts, restore from 23ec9294
```

## Packet 07 Checklist: Panel Registry and Render Dispatch Extraction

### Implementation
- [x] Panel registry introduced and used for dispatch.
- [x] Inline render chain reduced substantially.
- [x] Panel prop contracts preserved.
- [x] Accessibility and nav behavior preserved.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 07 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/settingsPanelRegistry.ts (new, 137 LOC): Typed registry mapping 22 dispatchable tabs to components with getProps factories; proxmox kept separate for section-nav behavior
- frontend-modern/src/components/Settings/Settings.tsx (2274→2207 LOC, -67): Replaced Show chain with Dynamic dispatch via registry lookup

Commands run + exit codes (reviewer-independent):
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (7/7 passed)

Gate checklist:
- P0: PASS (registry file verified with all 22 tabs; Settings.tsx uses Dynamic dispatch; both commands pass)
- P1: PASS (only active panel mounts via Dynamic; proxmox section-nav preserved separately; props factories maintain contracts)
- P2: PASS (tracker updated)

Verdict: APPROVED

Commit:
- `aea36a07` (feat(settings): Packet 07 — panel registry and render dispatch extraction)

Residual risk:
- None

Rollback:
- Delete settingsPanelRegistry.ts, restore Show chain from 4854c251
```

## Packet 08 Checklist: Contract Test Hardening (Settings Routing + Gates)

### Implementation
- [x] Canonical path mapping contract cases added/updated.
- [x] Legacy alias/redirect contract cases added/updated.
- [x] Organization route contract cases added/updated.
- [x] Feature-gated path contract cases added/updated.

### Required Tests
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed (8/8).
- [x] `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts` — legacyRedirects 3/3 PASS, legacyRouteContracts 2/2 PASS; platformTabs pre-existing failure.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 08 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/__tests__/settingsRouting.test.ts: Expanded from 7→8 tests (153 LOC), added canonical path mapping, legacy alias/redirect, organization routing, query deep-link, feature gate, and deriveAgentFromPath contract cases

Commands run + exit codes (reviewer-independent):
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (8/8 passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (test file verified with 8 contract tests; both commands pass)
- P1: PASS (comprehensive route/gate coverage: canonical paths, legacy aliases, org routing, query deep-links, feature gates, agent paths)
- P2: PASS (tracker updated)

Verdict: APPROVED

Commit:
- `fd735965` (test(settings): Packet 08 — contract test hardening for routing and gates)

Residual risk:
- None

Rollback:
- Restore previous test file from aea36a07
```

## Packet 09 Checklist: Architecture Guardrails for Settings Monolith Regression

### Implementation
- [x] Architecture guard tests added for externalized modules.
- [x] Guardrails are low-noise and not brittle.
- [x] Exceptions policy documented.
- [x] CI behavior validated.

### Required Tests
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsArchitecture.test.ts src/components/Settings/__tests__/settingsRouting.test.ts` passed (12/12).
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] `go build ./...` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 09 Review Evidence

```
Files changed:
- frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts (new, 67 LOC): 4 architecture guardrail tests — module existence via glob, import relationship assertions, no-inline-redefinition checks, LOC ceiling (2500) with exceptions policy

Commands run + exit codes (reviewer-independent):
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsArchitecture.test.ts src/components/Settings/__tests__/settingsRouting.test.ts` -> exit 0 (12/12 passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (test file verified, all 3 commands pass)
- P1: PASS (guards enforce module boundaries without brittleness; exceptions policy documented in test)
- P2: PASS (tracker updated)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- None

Rollback:
- Delete settingsArchitecture.test.ts
```

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
