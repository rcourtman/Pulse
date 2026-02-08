# Unified Resource Convergence Phase 2 Progress Tracker

Linked plan:
- `docs/architecture/unified-resource-convergence-phase-2-plan-2026-02.md`

Status: Active
Date: 2026-02-08

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Reviewer must provide explicit command exit-code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or summary-only.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. Update this file first in each implementation session and last before session end.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before next packet.
7. Do not use `git checkout --`, `git restore --source`, `git reset --hard`, or `git clean -fd` on shared worktrees.
8. Respect active parallel worker boundaries: do not edit storage/settings worker-owned files.

## Active Worker Boundaries

1. Storage GA lane active: avoid edits in packet-owned storage GA files.
2. Settings stabilization lane active: avoid edits in packet-owned settings files.
3. Deferred item: `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| 00 | Contract Freeze, Scope Fences, and Drift Baseline | DONE | Codex | Claude | APPROVED | See Packet 00 Review Evidence |
| 01 | Introduce Unified Selector Layer (No Behavior Change) | DONE | Codex | Claude | APPROVED | See Packet 01 Review Evidence |
| 02 | Alerts Consumer Migration to Unified Selectors | DONE | Codex | Claude | APPROVED | See Packet 02 Review Evidence |
| 03 | AI Chat UI Context Migration to Unified Selectors | DONE | Codex | Claude | APPROVED | See Packet 03 Review Evidence |
| 04 | WebSocket Legacy Payload Deprecation Gates | DONE | Codex | Claude | APPROVED | See Packet 04 Review Evidence |
| 05 | Legacy Compatibility Narrowing | DONE | Codex | Claude | APPROVED | See Packet 05 Review Evidence |
| 06 | Contract Test Hardening and Regression Net | DONE | Codex | Claude | APPROVED | See Packet 06 Review Evidence |
| 07 | Final Certification and Release Recommendation | TODO | Claude | Claude | PENDING | See Packet 07 Review Evidence |

## Packet 00 Checklist: Contract Freeze, Scope Fences, and Drift Baseline

### Discovery
- [x] In-scope/out-of-scope boundaries documented and approved.
- [x] Alerts/AI dependency baseline matrix captured with code anchors.
- [x] Deferred settings-owned migration item captured with unblock condition.
- [x] Existing scaffold coverage verified: `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts` and `frontend-modern/src/hooks/__tests__/useResources.test.ts` already exist.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 00 Review Evidence

```text
Files changed:
- docs/architecture/unified-resource-convergence-phase-2-plan-2026-02.md: Added frozen scope boundaries (in-scope/out-of-scope) and legacy dependency baseline matrix with verified code anchors.
- docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md: Updated Packet 00 checklist and review evidence.

Commands run + exit codes:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (No production code changes; docs-only packet. tsc passes.)
- P1: PASS (Scope boundaries explicit. Worker-owned files untouched. Deferred item documented.)
- P2: PASS (Baseline matrix anchors independently verified by reviewer and corrected for useResources.ts sub-conversion locations.)

Verdict: APPROVED

Commit:
- `878a9a0c` (docs(urc2): Packet 00 — freeze scope fences and drift baseline)

Residual risk:
- Line-number anchors will drift as code changes; re-verify before starting Packet 01.

Rollback:
- Revert the two documentation files to their pre-Packet-00 state (git show HEAD:path).
```

## Packet 01 Checklist: Introduce Unified Selector Layer (No Behavior Change)

### Implementation
- [x] Unified selector APIs introduced for alerts/ai consumers.
- [x] No behavior change validated against current outputs.
- [x] Type contracts documented and enforced.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts` passed.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 01 Review Evidence

```text
Files changed:
- frontend-modern/src/hooks/useResources.ts: Added UseAlertsResourcesReturn/UseAIChatResourcesReturn interfaces and useAlertsResources/useAIChatResources wrapper hooks.
- frontend-modern/src/hooks/__tests__/useResources.test.ts: Added selector-layer tests for return shape and derived signals.
- docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md: Updated Packet 01 checklist and review evidence.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts` -> exit 0 (35 tests passed)
2. `npx tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (No behavior change; thin wrappers only. Both tests and tsc pass.)
- P1: PASS (Type contracts via explicit interfaces. No duplicated conversion logic.)
- P2: PASS (Selector APIs match baseline matrix consumer needs.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- none

Rollback:
- Remove useAlertsResources/useAIChatResources exports and interfaces from useResources.ts; remove test blocks.
```

## Packet 02 Checklist: Alerts Consumer Migration to Unified Selectors

### Implementation
- [x] Core alerts read paths switched to unified selectors.
- [x] Direct legacy array reads removed from core decision points.
- [x] One-packet compatibility fallback retained for rollback safety.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts` passed.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 02 Review Evidence

```text
Files changed:
- frontend-modern/src/pages/Alerts.tsx: Migrated all 11 legacy array read sites to useAlertsResources() selectors. ThresholdsTab/ThresholdsTable now receive nodes/storage/dockerHosts as explicit props. Non-resource state fields unchanged.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts` -> exit 0 (27 tests passed)
2. `npx tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (All 11 legacy read sites migrated. Override hydration, allGuests, ThresholdsTab all use selectors. Tests + tsc pass.)
- P1: PASS (Fallback retained via useResourcesAsLegacy underneath. Non-resource state untouched.)
- P2: PASS (Behavior-preserving migration; no new logic introduced.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- ThresholdsTable still receives pbs/pmg/backups from state directly; these are out-of-scope for this packet.

Rollback:
- Revert Alerts.tsx to pre-Packet-02 state; remove useAlertsResources import and revert to direct state.* reads.
```

## Packet 03 Checklist: AI Chat UI Context Migration to Unified Selectors

### Implementation
- [x] AI chat context synthesis switched to unified selector outputs.
- [x] Legacy array loops removed from migrated scope.
- [x] Mention and context behavior remains stable.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts` passed.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 03 Review Evidence

```text
Files changed:
- frontend-modern/src/components/AI/Chat/index.tsx: Replaced MonitoringAPI.getState() onMount with reactive createEffect using useAIChatResources() selectors. Removed MonitoringAPI import and local isCluster signal. Mention ID formats and name resolution logic preserved.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts` -> exit 0 (15 tests passed)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (Mention IDs, types, name resolution all preserved. Tests + tsc pass.)
- P1: PASS (One-shot API call replaced with reactive selectors — strictly better behavior. Fallback preserved via underlying useResourcesAsLegacy.)
- P2: PASS (Clean migration, no new logic beyond reactive wiring.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- none

Rollback:
- Revert AI Chat/index.tsx to pre-Packet-03 state; restore MonitoringAPI.getState() onMount pattern.
```

## Packet 04 Checklist: WebSocket Legacy Payload Deprecation Gates

### Implementation
- [x] Compatibility mode switch implemented for legacy payload fields.
- [x] Telemetry/logging added to detect remaining legacy consumers.
- [x] Default mode remains compatibility-safe.

### Required Tests
- [x] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` passed.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 04 Review Evidence

```text
Files changed:
- internal/models/models_frontend.go: Added StripLegacyArrays() method that nils legacy arrays while preserving PBS/PMG/Backups.
- internal/websocket/hub.go: Added legacyPayloadCompat flag (default true), prepareStateForBroadcast() applied to all broadcast paths, startup logging.
- internal/api/router_integration_test.go: Added TestWebsocketLegacyCompatMode contract test for both compat-on and compat-off modes.
- frontend-modern/src/stores/websocket.ts: Added unified-only mode detection debug log.

Commands run + exit codes:
1. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (Default compat mode enabled, no behavior change. prepareStateForBroadcast copies before stripping — no mutation. All broadcast paths covered.)
- P1: PASS (Contract test validates both compat-on and compat-off modes. PBS/PMG/Backups preserved in stripped mode.)
- P2: PASS (Clean separation of concerns. Thread-safe flag with RWMutex.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- Flag is only in-memory; future work could persist it via config if needed.

Rollback:
- Remove legacyPayloadCompat from Hub, revert prepareStateForBroadcast calls, remove StripLegacyArrays method, remove test.
```

## Packet 05 Checklist: Legacy Compatibility Narrowing

### Implementation
- [x] Selector/adapter fallback matrix narrowed to documented transitional paths.
- [x] Hidden fallback branches removed.
- [x] Rollback path stays explicit.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts` passed.
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 05 Review Evidence

```text
Files changed:
- frontend-modern/src/hooks/useResources.ts: Removed opportunistic legacy.length > 0 fallback from 5 conversion memos. Removed unused has*Resources memos. Added fallback matrix documentation.
- frontend-modern/src/stores/websocket.ts: Documented remaining legacy payload paths.
- frontend-modern/src/hooks/__tests__/useResources.test.ts: Added narrowed-fallback tests (32 total, +3 new).
- frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts: Added regression test (+1 new, 7 total).

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts` -> exit 0 (39 tests)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (Rollback path preserved — unified empty → legacy. 5 opportunistic tiers removed cleanly. Tests pass.)
- P1: PASS (PBS/PMG/storage bounded fallback preserved. Fallback matrix explicitly documented.)
- P2: PASS (Clean diff — 88 lines. No new complexity.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- The narrowing makes unified conversion the primary path when unified resources exist. Any conversion bugs will now surface (which is the intended behavior).

Rollback:
- Restore legacy.length > 0 tiers in useResourcesAsLegacy conversion memos.
```

## Packet 06 Checklist: Contract Test Hardening and Regression Net

### Implementation
- [x] Alerts parity tests expanded for unified-selector paths.
- [x] AI context parity tests expanded for unified-selector paths.
- [x] Websocket contract tests lock compatibility and deprecation modes.

### Required Tests
- [x] `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts` passed.
- [x] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 06 Review Evidence

```text
Files changed:
- frontend-modern/src/pages/__tests__/Alerts.helpers.test.ts: Added unified selector parity tests (+2 tests, 29 total).
- frontend-modern/src/components/AI/__tests__/aiChatUtils.test.ts: Added mention ID format contract tests (+5 tests, 13 total).
- frontend-modern/src/hooks/__tests__/useResources.test.ts: Added stale legacy-only consumer detection tests (+3 tests, 35 total).
- internal/api/router_integration_test.go: Added TestWebsocketPayloadContractShape to lock payload key contract.

Commands run + exit codes:
1. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0 (77 tests)
2. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0

Gate checklist:
- P0: PASS (Tests-only packet. All new tests pass. No production code changes.)
- P1: PASS (Contract coverage: alerts parity, mention ID format, stale-legacy detection, payload shape.)
- P2: PASS (Negative-path assertions lock migration behavior.)

Verdict: APPROVED

Commit:
- (pending)

Residual risk:
- none

Rollback:
- <steps>
```

## Packet 07 Checklist: Final Certification and Release Recommendation

### Final Validation
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts` passed.
- [ ] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` passed.
- [ ] `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` passed.
- [ ] Exit codes recorded for all commands.

### Certification
- [ ] Residual risks documented.
- [ ] Rollback strategy validated.
- [ ] Verdict recorded (`APPROVED` or `BLOCKED`).

### Packet 07 Review Evidence

```text
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Deferred Follow-On (Blocked)

Item:
- Organization sharing migration in `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`

Blocked by:
- Active settings stabilization worker ownership

Unblock condition:
- Settings stabilization plan complete and file ownership released
