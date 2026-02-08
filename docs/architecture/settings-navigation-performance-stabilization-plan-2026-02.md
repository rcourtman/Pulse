# Settings Navigation and Performance Stabilization Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md`

## Product Intent

Settings navigation must feel immediate and reliable:
1. sidebar clicks always load the intended panel,
2. tab transitions do not stall under background polling/load,
3. Settings remains modular and regression-resistant as new panels are added.

This plan is a stabilization track for post-decomposition regressions.

## Non-Negotiable Contracts

1. Route contract:
- Existing canonical settings routes remain stable.
- Existing legacy aliases/redirect behavior remains compatible.
- `/settings/*` must continue to resolve every defined tab ID deterministically.

2. Interaction contract:
- Sidebar clicks should update visible content predictably with no silent no-op states.
- Locked/feature-gated tabs must be visually and behaviorally explicit.

3. Performance contract:
- Initial settings load must avoid duplicate bootstrap fetch bursts.
- Polling and refresh loops must not degrade tab interaction responsiveness.

4. Architecture contract:
- Settings shell remains composition-first.
- Heavy panel payloads should not block core navigation interaction.

5. Safety contract:
- Packet scopes are confined to Settings/routing-related files unless explicitly stated.
- Out-of-scope failures from parallel streams are documented, not absorbed.

6. Rollback contract:
- Every packet has file-level rollback instructions.
- No destructive git operations required.

## Code-Derived Audit Baseline

### A. Complexity and interaction surface

1. Settings shell remains large:
- `frontend-modern/src/components/Settings/Settings.tsx`: 2207 LOC.

2. Settings route orchestration is centralized:
- `frontend-modern/src/components/Settings/useSettingsNavigation.ts`.

3. Sidebar click behavior depends on route navigation + active-tab synchronization:
- `setActiveTab(...)` in `useSettingsNavigation.ts`.

### B. Bootstrap and polling pressure

1. Duplicate bootstrap load calls exist:
- Parent `Settings.tsx` `onMount()` calls `loadNodes()`, `loadDiscoveredNodes()`, `loadSecurityStatus()`.
- `useInfrastructureSettingsState.ts` `onMount()` also calls `loadSecurityStatus()`, `loadNodes()`, `loadDiscoveredNodes()`, and `initializeSystemSettingsState()`.

2. Concurrent polling loops during settings usage:
- Diagnostics polling from `useSystemSettingsState.ts` when current tab is proxmox.
- Discovery polling and modal polling from `useInfrastructureSettingsState.ts`.

3. These overlapping flows increase UI churn and can degrade click responsiveness.

### C. Coverage gaps vs reported symptom

1. Current tests validate routing helpers and architecture boundaries, but not sidebar click-to-panel rendering behavior:
- `settingsRouting.test.ts` validates pure route helpers.
- `settingsArchitecture.test.ts` validates extraction/size guardrails.
- No integration test currently locks sidebar click reliability across full tab set.

2. No explicit performance guardrails for duplicate bootstrap/polling orchestration.

## Audit Findings (Priority)

| ID | Severity | Finding | Evidence |
|---|---|---|---|
| SNP-001 | High | Duplicate startup fetch orchestration likely causes initial churn and responsiveness degradation. | `Settings.tsx` + `useInfrastructureSettingsState.ts` `onMount` flows |
| SNP-002 | High | Sidebar click reliability is not protected by integration-level tests. | Settings tests currently focus on routing/helpers only |
| SNP-003 | Medium | Settings shell still eagerly wires large panel graph, increasing transition burden. | `Settings.tsx`, `settingsPanelRegistry.ts` |
| SNP-004 | Medium | Polling lifecycle and tab lifecycle are not explicitly coordinated for interaction priority. | `useSystemSettingsState.ts`, `useInfrastructureSettingsState.ts` |
| SNP-005 | Medium | User-visible locked-tab behavior can be interpreted as "did not load" without stronger affordances. | `Settings.tsx` tab click + license lock redirect flow |

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
2. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsRouting.test.ts src/components/Settings/__tests__/settingsArchitecture.test.ts`
3. `cd frontend-modern && npx vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts`

On packets that change Settings component behavior, additionally run:

4. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`

On backend-touching packets (if any):

5. `go build ./...`

Notes:
- `go build` alone is never sufficient.
- Missing exit code evidence is a failed gate.

## Execution Packets

### Packet 00: Repro Matrix and Baseline Instrumentation

Objective:
- Convert reported sidebar/load instability into deterministic repro cases and baseline metrics.

Scope:
- `docs/architecture/settings-navigation-performance-stabilization-plan-2026-02.md` (appendix updates only)
- `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md`
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx` (new scaffold)

Implementation checklist:
1. Define reproducible scenario matrix (tab clicks, route transitions, locked tabs, slow bootstrap).
2. Add baseline integration test scaffold that currently demonstrates observed failure modes (if reproducible).
3. Capture startup request/polling baseline in plan appendix.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsRouting.test.ts src/components/Settings/__tests__/settingsArchitecture.test.ts`

Exit criteria:
- Repro and baseline are explicit and ratified.

### Packet 01: Startup Orchestration De-duplication

Objective:
- Eliminate duplicate bootstrap fetches and establish single ownership for initial settings hydration.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`
- `frontend-modern/src/components/Settings/useSystemSettingsState.ts`

Implementation checklist:
1. Define one bootstrap owner and remove redundant initial calls.
2. Preserve security/nodes/discovery/system-settings initialization correctness.
3. Ensure `initialLoadComplete` semantics remain accurate.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/settingsRouting.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Startup load graph is non-duplicated and behaviorally equivalent.

### Packet 02: Navigation State Machine Hardening

Objective:
- Make sidebar-to-route-to-active-panel flow explicit and robust under rapid clicks.

Scope:
- `frontend-modern/src/components/Settings/useSettingsNavigation.ts`
- `frontend-modern/src/components/Settings/settingsRouting.ts`
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`
- `frontend-modern/src/components/Settings/__tests__/settingsRouting.test.ts`

Implementation checklist:
1. Harden tab activation and redirect order to avoid transient no-op states.
2. Ensure route-to-tab synchronization is deterministic for all defined tabs.
3. Preserve legacy alias behavior.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/settingsRouting.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Sidebar clicks reliably resolve to intended panel state.

### Packet 03: Polling Lifecycle Isolation and Interaction Priority

Objective:
- Prevent background polling from degrading active navigation interactions.

Scope:
- `frontend-modern/src/components/Settings/useSystemSettingsState.ts`
- `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`

Implementation checklist:
1. Gate polling cadence by visibility/active tab where safe.
2. Prevent overlapping interval churn on rapid state changes.
3. Preserve data freshness expectations for infrastructure panels.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Polling no longer competes with core navigation responsiveness.

### Packet 04: Panel Loading Strategy and Transition Performance

Objective:
- Reduce interaction latency by decoupling heavy panel payload from core sidebar navigation.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsPanelRegistry.ts`
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`

Implementation checklist:
1. Introduce lazy loading/Suspense boundaries for heavy non-proxmox panels.
2. Keep navigation shell interactive while panel content resolves.
3. Preserve route contracts and panel mapping.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/settingsArchitecture.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Sidebar interactions remain responsive across panel transitions.

### Packet 05: Locked Tab UX Clarity and Non-Loading Affordance Fix

Objective:
- Eliminate ambiguity where users interpret lock redirects as "tab failed to load".

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsFeatureGates.ts`
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`

Implementation checklist:
1. Make locked behavior explicit at click point.
2. Prevent confusing silent state transitions for gated tabs.
3. Preserve licensing contract and redirect behavior.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/settingsRouting.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Locked tab behavior is explicit and cannot be mistaken for broken loading.

### Packet 06: Contract Test Hardening and Guardrails

Objective:
- Lock sidebar navigation reliability and startup orchestration invariants against regression.

Scope:
- `frontend-modern/src/components/Settings/__tests__/settingsNavigation.integration.test.tsx`
- `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts`
- `frontend-modern/src/components/Settings/__tests__/settingsRouting.test.ts`

Implementation checklist:
1. Add click-to-panel integration coverage for representative tab set.
2. Add assertions for no duplicate bootstrap orchestration paths.
3. Keep architecture guardrails aligned with stabilized design.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/settingsArchitecture.test.ts src/components/Settings/__tests__/settingsRouting.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Regressions in sidebar load behavior and startup churn are test-blocked.

### Packet 07: Final Certification and Release Recommendation

Objective:
- Certify Settings stabilization complete with explicit evidence and recommendation.

Scope:
- `docs/architecture/settings-navigation-performance-stabilization-plan-2026-02.md` (final appendices)
- `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md`

Implementation checklist:
1. Verify all packet evidence and checkpoint commits.
2. Run final frontend gate.
3. Record final verdict and residual risks.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run`

Exit criteria:
- Final verdict recorded: `GO` | `GO_WITH_CONDITIONS` | `NO_GO`.

## Risk Register

| Risk ID | Description | Severity | Packet Owner | Rollback Strategy |
|---|---|---|---|---|
| SNP-001 | Startup orchestration changes cause missing initial data in specific panels | High | 01 | Revert bootstrap ownership changes and restore prior call graph |
| SNP-002 | Navigation hardening introduces redirect loops | High | 02 | Revert useSettingsNavigation changes and restore prior route handling |
| SNP-003 | Polling isolation reduces required freshness in proxmox panels | Medium | 03 | Revert polling lifecycle gating and restore previous cadence |
| SNP-004 | Lazy panel loading introduces perceived blank states | Medium | 04 | Revert lazy loading while retaining de-duped bootstrap changes |
| SNP-005 | Lock UX changes drift from license contract | Medium | 05 | Revert UX layer changes; keep gate logic untouched |
| SNP-006 | Test guardrails become noisy/flaky | Low | 06 | Revert failing guard subset and replace with deterministic assertions |

## Appendix A: Repro Scenario Matrix

| Scenario ID | Description | Steps | Expected | Actual/Status |
|---|---|---|---|---|
| RSM-01 | Sidebar click loads intended panel for all canonical tabs | Open `/settings/infrastructure`; click each sidebar item once across Resources, Organization, Integrations, Operations, System, Security. | Active panel always matches clicked canonical tab route. | Baseline defined; pending packet execution evidence. |
| RSM-02 | Rapid sidebar clicks resolve to last-clicked tab | From any settings tab, click 4-6 different tabs rapidly (including cross-group transitions). | Final rendered panel + route match the last click, with no stale intermediate lock. | Baseline defined; pending packet execution evidence. |
| RSM-03 | Locked/gated tab click shows explicit lock state (not silent no-op) | Use a license lacking required features; click `system-relay`, `reporting`, `security-webhooks`, and `organization-*` tabs. | UI communicates lock/gate state explicitly and does not appear broken. | Baseline defined; pending packet execution evidence. |
| RSM-04 | Initial Settings load does not fire duplicate bootstrap fetches | Hard refresh `/settings/infrastructure` with network inspector open; capture initial API burst. | Single-owner startup hydration without duplicate calls for shared resources. | Known issue at baseline: duplicate bootstrap overlap present. |
| RSM-05 | Tab transitions are not blocked by background polling | Enter proxmox settings and leave polling active; repeatedly switch between proxmox, operations, and security tabs. | Transitions remain responsive while polling continues in background. | Baseline defined; pending packet execution evidence. |

## Appendix B: Startup Request Baseline

Current startup call graph (2026-02-08 baseline):

- `Settings.tsx` `onMount` (`frontend-modern/src/components/Settings/Settings.tsx:385`) calls:
  - `loadLicenseStatus()`
  - `loadNodes()`
  - `loadDiscoveredNodes()`
  - `loadSecurityStatus()`
- `useInfrastructureSettingsState.ts` `onMount` async bootstrap (`frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts:849`) calls:
  - `loadSecurityStatus()`
  - `loadNodes()`
  - `loadDiscoveredNodes()`
  - `initializeSystemSettingsState()`
- Overlap between both startup locations:
  - `loadSecurityStatus()`
  - `loadNodes()`
  - `loadDiscoveredNodes()`
- `useSystemSettingsState.ts` diagnostics startup/polling (`frontend-modern/src/components/Settings/useSystemSettingsState.ts:266` and `frontend-modern/src/components/Settings/useSystemSettingsState.ts:280`):
  - `runDiagnostics()` on mount
  - 60s polling interval when `currentTab() === 'proxmox'`
