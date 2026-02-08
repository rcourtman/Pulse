# Settings Control Plane Decomposition Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/settings-control-plane-decomposition-progress-2026-02.md`

## Product Intent

Settings must behave as a stable control plane, not a monolithic page implementation.

This plan has two top-level goals:
1. Turn `Settings.tsx` into a composition shell with clear module boundaries.
2. Lock route/deep-link/feature-gate contracts so refactors cannot silently break navigation or licensing behavior.

## Non-Negotiable Contracts

1. Route and deep-link contract:
- Existing settings URLs must keep their canonical tab resolution behavior.
- Legacy aliases and redirects must remain compatible until explicitly removed by migration packet.

2. Feature gate contract:
- License-gated tabs remain hidden/locked exactly as before.
- Multi-tenant organization tabs remain unavailable when tenant mode is disabled.

3. UX and behavior contract:
- Save/load behavior for system, infrastructure, and backup flows remains behaviorally equivalent.
- No regressions to loading/error/notification semantics.

4. Cross-track safety contract:
- Packet scopes must avoid opportunistic edits to alerts page architecture work unless explicitly required.
- Out-of-scope failures from other streams must be documented, not silently absorbed.

5. Rollback contract:
- Every packet has file-granular rollback steps.
- No destructive restore/reset commands are required for rollback.

## Current Baseline (Code-Derived)

1. `frontend-modern/src/components/Settings/Settings.tsx`: 4285 LOC.
2. `frontend-modern/src/components/Settings/settingsRouting.ts`: 209 LOC.
3. Existing settings tests:
- `frontend-modern/src/components/Settings/__tests__/settingsRouting.test.ts`
- `frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx`
- `frontend-modern/src/components/Settings/__tests__/SuggestProfileModal.test.tsx`
4. Routing contract tests exist in:
- `frontend-modern/src/routing/__tests__/legacyRedirects.test.ts`
- `frontend-modern/src/routing/__tests__/legacyRouteContracts.test.ts`
- `frontend-modern/src/routing/__tests__/platformTabs.test.ts`

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
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`
3. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/UnifiedAgents.test.tsx src/components/Settings/__tests__/SuggestProfileModal.test.tsx`
4. `go build ./...`

Run routing compatibility tests on packets touching path/redirect logic:

5. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts`

Notes:
- `go build` alone is never sufficient for approval.
- Empty, timed-out, or truncated output without exit code is invalid evidence.

## Execution Packets

### Packet 00: Surface Inventory and Decomposition Cut-Map

Objective:
- Produce a concrete inventory of Settings responsibilities and map them to extraction targets.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsRouting.ts`
- `docs/architecture/settings-control-plane-decomposition-plan-2026-02.md` (appendix updates only)

Implementation checklist:
1. Inventory tab metadata ownership (labels, descriptions, category structure, features).
2. Inventory route/deep-link/legacy redirect logic with function anchors.
3. Inventory state clusters (system settings, infrastructure nodes, backup flows, modals, org usage).
4. Build risk register and packet mapping.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Exit criteria:
- Every high-severity risk has packet mapping and rollback notes.

### Packet 01: Tab Schema and Metadata Extraction

Objective:
- Move tab/category/header metadata from `Settings.tsx` into dedicated schema modules.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsTabs.ts` (new)
- `frontend-modern/src/components/Settings/settingsHeaderMeta.ts` (new)
- `frontend-modern/src/components/Settings/settingsTypes.ts` (new or expanded)

Implementation checklist:
1. Extract tab IDs/types into shared types module.
2. Extract section/tab tree with feature tags into tab schema module.
3. Extract header title/description metadata into dedicated map.
4. Keep existing visible nav ordering and labels unchanged.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Exit criteria:
- `Settings.tsx` no longer owns inline tab schema/meta constants.

### Packet 02: Feature Gate Engine Extraction

Objective:
- Centralize feature and license gating decisions behind testable helpers.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsFeatureGates.ts` (new)
- `frontend-modern/src/stores/license.ts` (read-only usage unless explicit fix required)

Implementation checklist:
1. Extract `isFeatureLocked` and tab visibility decisions into helper module.
2. Preserve multi-tenant tab hiding and license lock behavior.
3. Keep notifications and fallback behavior unchanged.
4. Add tests for representative lock/hide combinations.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Exit criteria:
- Gating logic is centralized and behaviorally equivalent.

### Packet 03: Navigation and Deep-Link Orchestration Extraction

Objective:
- Isolate URL sync, redirects, and tab activation flow into orchestration helpers/hooks.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/useSettingsNavigation.ts` (new)
- `frontend-modern/src/components/Settings/settingsRouting.ts`

Implementation checklist:
1. Extract initial tab resolution and URL synchronization flow.
2. Extract legacy path redirect mappings into dedicated orchestration helper.
3. Preserve `settingsTabPath`/`deriveTabFromPath` semantics.
4. Maintain no-flicker behavior around `/settings` landing route handling.

Required tests:
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`
2. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Navigation/deep-link orchestration no longer lives inline in `Settings.tsx`.

### Packet 04: System Settings State Slice Extraction

Objective:
- Move system settings load/save state (general/network/updates/backups) into dedicated hook(s).

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/useSystemSettingsState.ts` (new)
- `frontend-modern/src/components/Settings/BackupsSettingsPanel.tsx` (touch only if needed for prop contracts)

Implementation checklist:
1. Extract backup polling state and derived summaries.
2. Extract system settings load path and save payload assembly.
3. Keep immediate-save vs explicit-save semantics unchanged.
4. Preserve notifications and error handling behavior.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Exit criteria:
- System settings lifecycle logic is hook-owned; `Settings.tsx` delegates.

### Packet 05: Infrastructure and Node Workflow Extraction

Objective:
- Isolate infrastructure node orchestration and agent-specific flow logic.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts` (new)
- `frontend-modern/src/components/Settings/ConfiguredNodeTables.tsx` (if interface updates required)

Implementation checklist:
1. Extract node list normalization and derived instance maps.
2. Extract node create/update/delete/test orchestration flow.
3. Preserve agent-specific behavior (PVE/PBS/PMG) and UI contracts.
4. Preserve websocket/state refresh behavior after mutations.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/UnifiedAgents.test.tsx`

Exit criteria:
- Infrastructure workflow logic is extracted behind dedicated hook(s).

### Packet 06: Backup Import/Export and Passphrase Flow Extraction

Objective:
- Isolate backup import/export modal and passphrase orchestration from main settings shell.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/useBackupTransferFlow.ts` (new)
- `frontend-modern/src/components/Settings/BackupsSettingsPanel.tsx`

Implementation checklist:
1. Extract backup export/import request assembly and validation rules.
2. Extract passphrase modal states and transitions.
3. Preserve warnings, error copy, and success semantics.
4. Add tests for invalid payload guard and modal transition basics.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/SuggestProfileModal.test.tsx`

Exit criteria:
- Backup transfer flow no longer managed inline by `Settings.tsx`.

### Packet 07: Panel Registry and Render Dispatch Extraction

Objective:
- Replace large inline `<Show when={activeTab() === ...}>` render chain with a registry/dispatcher model.

Scope:
- `frontend-modern/src/components/Settings/Settings.tsx`
- `frontend-modern/src/components/Settings/settingsPanelRegistry.ts` (new)
- `frontend-modern/src/components/Settings/SettingsSectionNav.tsx` (if contracts need updates)

Implementation checklist:
1. Introduce panel registry keyed by tab IDs.
2. Move render dispatch to registry lookup with typed fallbacks.
3. Preserve panel props and side-effect behavior.
4. Keep accessibility and section nav behavior unchanged.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Exit criteria:
- `Settings.tsx` render branch complexity is substantially reduced and registry-driven.

### Packet 08: Contract Test Hardening (Settings Routing + Gates)

Objective:
- Lock route/gate/tab contracts with explicit tests to prevent refactor drift.

Scope:
- `frontend-modern/src/components/Settings/__tests__/settingsRouting.test.ts`
- `frontend-modern/src/routing/__tests__/legacyRedirects.test.ts`
- `frontend-modern/src/routing/__tests__/legacyRouteContracts.test.ts`
- `frontend-modern/src/components/Settings/settingsRouting.ts` (only if test-driven contract fixes are needed)

Implementation checklist:
1. Add contract cases for canonical tab path mapping.
2. Add contract cases for legacy aliases and redirects.
3. Add contract cases for organization tab routing behavior.
4. Add contract cases for feature-gated paths.

Required tests:
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`
2. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Route/gate regressions fail fast through explicit contract tests.

### Packet 09: Architecture Guardrails for Settings Monolith Regression

Objective:
- Add enforceable guardrails so Settings logic cannot silently collapse back into one file.

Scope:
- `frontend-modern/eslint.config.js` (if needed)
- `frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts` (new)
- `frontend-modern/src/components/Settings/Settings.tsx`

Implementation checklist:
1. Add architecture test(s) asserting externalized schema/registry/hook usage.
2. Add size/structure guardrails suitable for CI (implementation-agnostic, low false positive risk).
3. Ensure guardrails enforce boundaries, not code style preferences.
4. Document exceptions policy for future justified changes.

Required tests:
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsArchitecture.test.ts src/components/Settings/__tests__/settingsRouting.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
3. `go build ./...`

Exit criteria:
- CI has explicit protections against re-monolith drift.

### Packet 10: Final Certification

Objective:
- Certify Settings control-plane decomposition as contract-safe and production-ready.

Implementation checklist:
1. Run global validation baseline and collect explicit exit-code evidence.
2. Produce before/after ownership map (schema, gates, navigation, state hooks, panel registry).
3. Document residual risks and deferred improvements.
4. Update progress tracker and final verdict.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts src/components/Settings/__tests__/UnifiedAgents.test.tsx src/components/Settings/__tests__/SuggestProfileModal.test.tsx`
3. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/legacyRedirects.test.ts src/routing/__tests__/legacyRouteContracts.test.ts src/routing/__tests__/platformTabs.test.ts`
4. `go build ./...`

Exit criteria:
- Reviewer signs `APPROVED` with all gates passing and checkpoint evidence recorded.

## Acceptance Definition

Plan is complete only when:
1. Packet 00-10 are `DONE` in the linked progress tracker.
2. Every packet includes explicit reviewer evidence and verdict.
3. Route/gate/behavior contracts are validated where no intentional migration is declared.
4. `Settings.tsx` functions as composition shell rather than a behavior monolith.

## Appendix A: Risk Register

| Risk ID | Surface | Description | Severity | Mapped Packet | Mitigation |
| --- | --- | --- | --- | --- | --- |
| SC-001 | `Settings.tsx` inline navigation orchestration | Route synchronization and redirects are embedded in page logic, making regressions easy during unrelated edits. | HIGH | 03, 08 | Extract navigation orchestration + contract tests for redirects/deep links. |
| SC-002 | `Settings.tsx` inline tab schema and header metadata | Tab definitions and metadata are coupled to render logic, increasing change blast radius. | HIGH | 01 | Extract typed schema/meta modules and keep order/labels contract-locked. |
| SC-003 | Distributed feature gate decisions in page | Gate logic mixed with nav/render paths can drift from license/tenant semantics. | HIGH | 02, 08 | Centralized gate engine + explicit test matrix. |
| SC-004 | System settings state mixed with infrastructure state | Large mixed state surface makes save/load changes risky and hard to reason about. | HIGH | 04, 05 | Extract domain hooks and preserve side effects through contract tests. |
| SC-005 | Backup transfer flow in main shell | Import/export + passphrase flows carry security/UX risk when tangled with unrelated logic. | MEDIUM | 06 | Dedicated flow hook and validation tests. |
| SC-006 | Large render branch chain | `<Show>` chain is error-prone and discourages modular additions. | MEDIUM | 07 | Registry/dispatcher extraction with typed tab IDs. |
| SC-007 | Contract drift across legacy aliases | Legacy settings aliases can silently break without dedicated tests. | HIGH | 08 | Harden routing contract tests across canonical and legacy paths. |
| SC-008 | Re-monolith risk after refactor | Without guardrails, new features can re-centralize into `Settings.tsx`. | MEDIUM | 09 | Architecture test guardrails and documented exceptions policy. |

## Appendix B: Target Extraction Map

| Target Module | Ownership |
| --- | --- |
| `settingsTypes.ts` | Tab and settings composition types |
| `settingsTabs.ts` | Tab/category schema |
| `settingsHeaderMeta.ts` | Header title/description map |
| `settingsFeatureGates.ts` | Feature/license/tenant gate decisions |
| `useSettingsNavigation.ts` | URL sync, redirects, active tab orchestration |
| `useSystemSettingsState.ts` | System settings load/save state |
| `useInfrastructureSettingsState.ts` | Node/infrastructure orchestration state |
| `useBackupTransferFlow.ts` | Import/export/passphrase flow orchestration |
| `settingsPanelRegistry.ts` | Tab -> panel registry and dispatch |

## Appendix C: Tab Schema and Metadata Inventory

Tab definitions are owned primarily by `baseTabGroups` with header copy in `SETTINGS_HEADER_META`.
Feature lock enforcement is centralized through `tabFeatureRequirements` + `isFeatureLocked`/`isTabLocked`.

| Tab ID / Constant Key | Display Label | Category / Section Group | Feature Gate | License Requirement | Multi-Tenant Requirement | Approx. Line Range in `Settings.tsx` |
| --- | --- | --- | --- | --- | --- | --- |
| `'proxmox'` | Infrastructure | Resources | None | None | None | 912; header meta 269-273 |
| `'agents'` | Workloads | Resources | None | None | None | 913; header meta 279-282 |
| `'docker'` | Docker | Resources | None | None | None | 914; header meta 274-278 |
| `'organization-overview'` | Overview | Organization | `multi_tenant` | Requires license feature `multi_tenant` | Yes (`isMultiTenantEnabled()`) | 922-927; feature lock map 890 |
| `'organization-access'` | Access | Organization | `multi_tenant` | Requires license feature `multi_tenant` | Yes (`isMultiTenantEnabled()`) | 929-934; feature lock map 891 |
| `'organization-sharing'` | Sharing | Organization | `multi_tenant` | Requires license feature `multi_tenant` | Yes (`isMultiTenantEnabled()`) | 936-941; feature lock map 892 |
| `'organization-billing'` | Billing | Organization | `multi_tenant` | Requires license feature `multi_tenant` | Yes (`isMultiTenantEnabled()`) | 943-948; feature lock map 893 |
| `'api'` | API Access | Integrations | None | None | None | 954; header meta 327-331 |
| `'diagnostics'` | Diagnostics | Operations | None | None | None | 961-965; header meta 360-364 |
| `'reporting'` | Reporting | Operations | `advanced_reporting` | Requires Pulse Pro capability `advanced_reporting` | None | 967-972; feature lock map 888 |
| `'system-logs'` | System Logs | Operations | None | None | None | 974-978; header meta 369-372 |
| `'system-general'` | General | System | None | None | None | 986-990; header meta 283-286 |
| `'system-network'` | Network | System | None | None | None | 992-996; header meta 287-290 |
| `'system-updates'` | Updates | System | None | None | None | 998-1002; header meta 291-294 |
| `'system-backups'` | Backups | System | None | None | None | 1004-1008; header meta 295-298 |
| `'system-ai'` | AI | System | None | None | None | 1010-1014; header meta 299-302 |
| `'system-relay'` | Remote Access | System | `relay` | Requires Pulse Pro capability `relay` | None | 1016-1021; feature lock map 887 |
| `'system-pro'` | Pulse Pro | System | None (acts as lock fallback destination) | None | None | 1023-1026; header meta 307-310 |
| `'security-overview'` | Overview | Security | None | None | None | 1034-1038; header meta 332-335 |
| `'security-auth'` | Authentication | Security | None | None | None | 1040-1044; header meta 336-339 |
| `'security-sso'` | Single Sign-On | Security | None | None | None | 1046-1050; header meta 340-343 |
| `'security-roles'` | Roles | Security | None | None | None | 1052-1056; header meta 344-347 |
| `'security-users'` | Users | Security | None | None | None | 1058-1062; header meta 348-351 |
| `'security-audit'` | Audit Log | Security | None | None | None | 1064-1068; header meta 352-355 |
| `'security-webhooks'` | Audit Webhooks | Security | `audit_logging` | Requires Pulse Pro capability `audit_logging` | None | 1070-1075; feature lock map 889 |

## Appendix D: Route, Deep-Link, and Legacy Redirect Inventory

### D1. Canonical Path Emitters

| Path Pattern | Target Tab / Section | Function Anchor (Approx. Line) | Canonical / Alias / Legacy Redirect | Source File |
| --- | --- | --- | --- | --- |
| `/settings/infrastructure` | `proxmox` | `settingsTabPath` (180-181) | Canonical | `settingsRouting.ts` |
| `/settings/workloads` | `agents` | `settingsTabPath` (182-183) | Canonical | `settingsRouting.ts` |
| `/settings/workloads/docker` | `docker` | `settingsTabPath` (184-185) | Canonical | `settingsRouting.ts` |
| `/settings/backups` | `system-backups` | `settingsTabPath` (186-187) | Canonical | `settingsRouting.ts` |
| `/settings/organization` | `organization-overview` | `settingsTabPath` (188-189) | Canonical | `settingsRouting.ts` |
| `/settings/organization/access` | `organization-access` | `settingsTabPath` (190-191) | Canonical | `settingsRouting.ts` |
| `/settings/organization/sharing` | `organization-sharing` | `settingsTabPath` (192-193) | Canonical | `settingsRouting.ts` |
| `/settings/billing` | `organization-billing` | `settingsTabPath` (194-195) | Canonical | `settingsRouting.ts` |
| `/settings/integrations/api` | `api` | `settingsTabPath` (196-197) | Canonical | `settingsRouting.ts` |
| `/settings/integrations/relay` | `system-relay` | `settingsTabPath` (198-199) | Canonical | `settingsRouting.ts` |
| `/settings/operations/diagnostics` | `diagnostics` | `settingsTabPath` (200-201) | Canonical | `settingsRouting.ts` |
| `/settings/operations/reporting` | `reporting` | `settingsTabPath` (202-203) | Canonical | `settingsRouting.ts` |
| `/settings/operations/logs` | `system-logs` | `settingsTabPath` (204-205) | Canonical | `settingsRouting.ts` |
| `/settings/<tab-id>` | Remaining tab IDs (default switch branch) | `settingsTabPath` (206-207) | Canonical fallback emitter | `settingsRouting.ts` |
| `/settings/infrastructure/pve` | Proxmox subsection `pve` | `agentPaths` + `handleSelectAgent` (420-435) | Canonical subsection | `Settings.tsx` |
| `/settings/infrastructure/pbs` | Proxmox subsection `pbs` | `agentPaths` + `handleSelectAgent` (420-435) | Canonical subsection | `Settings.tsx` |
| `/settings/infrastructure/pmg` | Proxmox subsection `pmg` | `agentPaths` + `handleSelectAgent` (420-435) | Canonical subsection | `Settings.tsx` |

### D2. Path Resolution Matrix (`deriveTabFromPath` / `deriveAgentFromPath`)

| Path Pattern | Target Tab / Section | Function Anchor (Approx. Line) | Canonical / Alias / Legacy Redirect | Source File |
| --- | --- | --- | --- | --- |
| `/settings/workloads/docker` | `docker` | `deriveTabFromPath` (31) | Canonical resolver | `settingsRouting.ts` |
| `/settings/infrastructure` | `proxmox` | `deriveTabFromPath` (32) | Canonical resolver | `settingsRouting.ts` |
| `/settings/workloads` | `agents` | `deriveTabFromPath` (34) | Canonical resolver | `settingsRouting.ts` |
| `/settings/storage` | `proxmox` | `deriveTabFromPath` (33) | Legacy alias | `settingsRouting.ts` |
| `/settings/proxmox` | `proxmox` | `deriveTabFromPath` (36) | Legacy alias | `settingsRouting.ts` |
| `/settings/agent-hub` | `proxmox` | `deriveTabFromPath` (37) | Legacy alias (also runtime redirected) | `settingsRouting.ts` |
| `/settings/docker` | `docker` | `deriveTabFromPath` (38) | Legacy alias | `settingsRouting.ts` |
| `/settings/hosts` | `agents` | `deriveTabFromPath` (41-49) | Alias | `settingsRouting.ts` |
| `/settings/host-agents` | `agents` | `deriveTabFromPath` (41-49) | Alias | `settingsRouting.ts` |
| `/settings/servers` | `agents` | `deriveTabFromPath` (41-49) | Alias (overridden by runtime redirect to infrastructure) | `settingsRouting.ts` |
| `/settings/linuxServers` | `agents` | `deriveTabFromPath` (41-49) | Legacy alias (also runtime redirected to workloads) | `settingsRouting.ts` |
| `/settings/windowsServers` | `agents` | `deriveTabFromPath` (41-49) | Legacy alias (also runtime redirected to workloads) | `settingsRouting.ts` |
| `/settings/macServers` | `agents` | `deriveTabFromPath` (41-49) | Legacy alias (also runtime redirected to workloads) | `settingsRouting.ts` |
| `/settings/agents` | `agents` | `deriveTabFromPath` (41-49) | Alias | `settingsRouting.ts` |
| `/settings/system-general` | `system-general` | `deriveTabFromPath` (52) | Canonical | `settingsRouting.ts` |
| `/settings/system-network` | `system-network` | `deriveTabFromPath` (53) | Canonical | `settingsRouting.ts` |
| `/settings/system-updates` | `system-updates` | `deriveTabFromPath` (54) | Canonical | `settingsRouting.ts` |
| `/settings/backups` | `system-backups` | `deriveTabFromPath` (55) | Canonical | `settingsRouting.ts` |
| `/settings/system-backups` | `system-backups` | `deriveTabFromPath` (56) | Alias | `settingsRouting.ts` |
| `/settings/system-ai` | `system-ai` | `deriveTabFromPath` (57) | Canonical | `settingsRouting.ts` |
| `/settings/integrations/relay` | `system-relay` | `deriveTabFromPath` (58) | Canonical | `settingsRouting.ts` |
| `/settings/system-relay` | `system-relay` | `deriveTabFromPath` (59) | Alias | `settingsRouting.ts` |
| `/settings/system-pro` | `system-pro` | `deriveTabFromPath` (60) | Canonical | `settingsRouting.ts` |
| `/settings/organization/access` | `organization-access` | `deriveTabFromPath` (61) | Canonical | `settingsRouting.ts` |
| `/settings/organization/sharing` | `organization-sharing` | `deriveTabFromPath` (62) | Canonical | `settingsRouting.ts` |
| `/settings/billing` | `organization-billing` | `deriveTabFromPath` (63) | Canonical | `settingsRouting.ts` |
| `/settings/plan` | `organization-billing` | `deriveTabFromPath` (64) | Alias | `settingsRouting.ts` |
| `/settings/organization/billing` | `organization-billing` | `deriveTabFromPath` (65) | Alias | `settingsRouting.ts` |
| `/settings/organization` | `organization-overview` | `deriveTabFromPath` (66) | Canonical | `settingsRouting.ts` |
| `/settings/operations/logs` | `system-logs` | `deriveTabFromPath` (67) | Canonical | `settingsRouting.ts` |
| `/settings/system-logs` | `system-logs` | `deriveTabFromPath` (68) | Alias | `settingsRouting.ts` |
| `/settings/integrations/api` | `api` | `deriveTabFromPath` (70) | Canonical | `settingsRouting.ts` |
| `/settings/api` | `api` | `deriveTabFromPath` (71) | Alias | `settingsRouting.ts` |
| `/settings/security-overview` | `security-overview` | `deriveTabFromPath` (73) | Canonical | `settingsRouting.ts` |
| `/settings/security-auth` | `security-auth` | `deriveTabFromPath` (74) | Canonical | `settingsRouting.ts` |
| `/settings/security-sso` | `security-sso` | `deriveTabFromPath` (75) | Canonical | `settingsRouting.ts` |
| `/settings/security-roles` | `security-roles` | `deriveTabFromPath` (76) | Canonical | `settingsRouting.ts` |
| `/settings/security-users` | `security-users` | `deriveTabFromPath` (77) | Canonical | `settingsRouting.ts` |
| `/settings/security-audit` | `security-audit` | `deriveTabFromPath` (78) | Canonical | `settingsRouting.ts` |
| `/settings/security-webhooks` | `security-webhooks` | `deriveTabFromPath` (79) | Canonical | `settingsRouting.ts` |
| `/settings/security` | `security-overview` | `deriveTabFromPath` (80) | Alias | `settingsRouting.ts` |
| `/settings/operations/updates` | `system-updates` | `deriveTabFromPath` (82) | Alias | `settingsRouting.ts` |
| `/settings/updates` | `system-updates` | `deriveTabFromPath` (83) | Alias | `settingsRouting.ts` |
| `/settings/operations/diagnostics` | `diagnostics` | `deriveTabFromPath` (84) | Canonical | `settingsRouting.ts` |
| `/settings/diagnostics` | `diagnostics` | `deriveTabFromPath` (85) | Alias | `settingsRouting.ts` |
| `/settings/operations/reporting` | `reporting` | `deriveTabFromPath` (86) | Canonical | `settingsRouting.ts` |
| `/settings/reporting` | `reporting` | `deriveTabFromPath` (87) | Alias | `settingsRouting.ts` |
| `/settings/pve`, `/settings/pbs`, `/settings/pmg`, `/settings/containers` | `proxmox` | `deriveTabFromPath` legacy block (89-100) | Legacy alias fallback | `settingsRouting.ts` |
| Any unmatched `/settings/*` path | `proxmox` | `deriveTabFromPath` default return (102) | Fallback | `settingsRouting.ts` |
| `/settings/infrastructure/pve` | Agent subsection `pve` | `deriveAgentFromPath` (106) | Canonical subsection | `settingsRouting.ts` |
| `/settings/infrastructure/pbs` | Agent subsection `pbs` | `deriveAgentFromPath` (107) | Canonical subsection | `settingsRouting.ts` |
| `/settings/infrastructure/pmg` | Agent subsection `pmg` | `deriveAgentFromPath` (108) | Canonical subsection | `settingsRouting.ts` |
| `/settings/pve`, `/settings/pbs`, `/settings/pmg` | Agent subsection `pve`/`pbs`/`pmg` | `deriveAgentFromPath` (110-112) | Legacy subsection alias | `settingsRouting.ts` |
| `/settings/storage` | Agent subsection `pbs` | `deriveAgentFromPath` (114) | Legacy subsection alias | `settingsRouting.ts` |

### D3. Query Deep-Link Mapping (`deriveTabFromQuery` + `/settings` landing handling)

| Path / Query Pattern | Target Tab / Section | Function Anchor (Approx. Line) | Canonical / Alias / Legacy Redirect | Source File |
| --- | --- | --- | --- | --- |
| `/settings` or `/settings/` with no `tab` query | `proxmox` | URL sync effect (462-465) | Canonical landing fallback | `Settings.tsx` |
| `/settings` or `/settings/` + `?tab=<value>` | Canonical path of resolved tab | URL sync effect (463-470) + `settingsTabPath` | Deep-link alias -> canonical redirect | `Settings.tsx` + `settingsRouting.ts` |
| `?tab=infrastructure|proxmox` | `proxmox` | `deriveTabFromQuery` (124-126) | Alias | `settingsRouting.ts` |
| `?tab=workloads|agents` | `agents` | `deriveTabFromQuery` (127-129) | Alias | `settingsRouting.ts` |
| `?tab=docker` | `docker` | `deriveTabFromQuery` (130-131) | Alias | `settingsRouting.ts` |
| `?tab=backups` | `system-backups` | `deriveTabFromQuery` (132-133) | Alias | `settingsRouting.ts` |
| `?tab=updates` | `system-updates` | `deriveTabFromQuery` (134-135) | Alias | `settingsRouting.ts` |
| `?tab=network` | `system-network` | `deriveTabFromQuery` (136-137) | Alias | `settingsRouting.ts` |
| `?tab=general` | `system-general` | `deriveTabFromQuery` (138-139) | Alias | `settingsRouting.ts` |
| `?tab=api` | `api` | `deriveTabFromQuery` (140-141) | Alias | `settingsRouting.ts` |
| `?tab=organization|org` | `organization-overview` | `deriveTabFromQuery` (142-144) | Alias | `settingsRouting.ts` |
| `?tab=organization-access|org-access` | `organization-access` | `deriveTabFromQuery` (145-147) | Alias | `settingsRouting.ts` |
| `?tab=organization-sharing|sharing` | `organization-sharing` | `deriveTabFromQuery` (148-150) | Alias | `settingsRouting.ts` |
| `?tab=billing|plan` | `organization-billing` | `deriveTabFromQuery` (151-153) | Alias | `settingsRouting.ts` |
| `?tab=security|security-overview` | `security-overview` | `deriveTabFromQuery` (154-156) | Alias | `settingsRouting.ts` |
| `?tab=security-auth` | `security-auth` | `deriveTabFromQuery` (157-158) | Alias | `settingsRouting.ts` |
| `?tab=security-sso` | `security-sso` | `deriveTabFromQuery` (159-160) | Alias | `settingsRouting.ts` |
| `?tab=security-roles` | `security-roles` | `deriveTabFromQuery` (161-162) | Alias | `settingsRouting.ts` |
| `?tab=security-users` | `security-users` | `deriveTabFromQuery` (163-164) | Alias | `settingsRouting.ts` |
| `?tab=security-audit` | `security-audit` | `deriveTabFromQuery` (165-166) | Alias | `settingsRouting.ts` |
| `?tab=security-webhooks` | `security-webhooks` | `deriveTabFromQuery` (167-168) | Alias | `settingsRouting.ts` |
| `?tab=diagnostics` | `diagnostics` | `deriveTabFromQuery` (169-170) | Alias | `settingsRouting.ts` |
| `?tab=reporting` | `reporting` | `deriveTabFromQuery` (171-172) | Alias | `settingsRouting.ts` |

### D4. Runtime Redirect Shims in `Settings.tsx`

| Path Pattern | Target Tab / Section | Function Anchor (Approx. Line) | Canonical / Alias / Legacy Redirect | Source File |
| --- | --- | --- | --- | --- |
| `/settings/agent-hub*` | Replace prefix with `/settings/infrastructure*` | URL sync effect (484-489) | Legacy redirect | `Settings.tsx` |
| `/settings/servers*` | Replace prefix with `/settings/infrastructure*` | URL sync effect (492-497) | Legacy redirect | `Settings.tsx` |
| `/settings/containers*` | Replace prefix with `/settings/workloads/docker*` | URL sync effect (500-505) | Legacy redirect | `Settings.tsx` |
| `/settings/linuxServers*`, `/settings/windowsServers*`, `/settings/macServers*` | Force `/settings/workloads` | URL sync effect (508-517) | Legacy redirect | `Settings.tsx` |

## Appendix E: State Cluster Inventory

### 1. System Settings State (General, Network, Updates, Theme, etc.)

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| `hasUnsavedChanges`, `setHasUnsavedChanges` | Signal | 533 | 04 |
| `pvePollingInterval`, `pvePollingSelection`, `pvePollingCustomSeconds` | Signal | 575-579 | 04 |
| `allowedOrigins`, `allowEmbedding`, `allowedEmbedOrigins`, `webhookAllowedPrivateCIDRs`, `publicURL` | Signal | 580, 749-756 | 04 |
| `discoveryEnabled`, `discoverySubnet`, `discoveryMode`, `discoverySubnetDraft`, `lastCustomSubnet`, `discoverySubnetError`, `savingDiscoverySettings` | Signal | 581-587 | 04 |
| `envOverrides` + lock helpers (`temperatureMonitoringLocked`, `hideLocalLoginLocked`, `backupPollingEnvLocked`, etc.) | Signal/Derived | 588, 598-609, 774-784 | 04 |
| `temperatureMonitoringEnabled`, `savingTemperatureSetting`, `hideLocalLogin`, `savingHideLocalLogin`, `disableDockerUpdateActions`, `savingDockerUpdateActions` | Signal | 589-596 | 04 |
| `versionInfo`, `updateInfo`, `checkingForUpdates`, `updateChannel`, `autoUpdateEnabled`, `autoUpdateCheckInterval`, `autoUpdateTime`, `updatePlan`, `isInstallingUpdate` | Signal | 759-769 | 04 |
| `backupPollingEnabled`, `backupPollingInterval`, `backupPollingCustomMinutes`, `backupPollingUseCustom` | Signal | 770-773 | 04 |
| `_diagnosticsData`, `_runningDiagnostics` | Signal | 806-807 | 04 |
| `updateStore` usage (`checkForUpdates`, dismissed state coordination) | Store | 100, 1863-1878, 2068-2092 | 04 |
| `createEffect` diagnostics poll loop tied to active tab | `createEffect` + `onCleanup` | 846-861 | 04 |
| System settings bootstrap (`SettingsAPI.getSystemSettings`, `UpdatesAPI.getVersion`) | `onMount` async workflow | 1657-1895 | 04 |
| `saveSettings`, `checkForUpdates`, `handleInstallUpdate`, `handleConfirmUpdate` | Async orchestration functions | 1934-2125 | 04 |

### 2. Infrastructure / Node State (Node Lists, CRUD Flows, Test Connections)

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| `nodes`, `discoveredNodes`, `editingNode`, `currentNodeType`, `modalResetKey`, `initialLoadComplete`, `discoveryScanStatus` | Signal | 537-556 | 05 |
| `showNodeModal`, `showDeleteNodeModal`, `nodePendingDelete`, `deleteNodeLoading` | Signal | 539, 544-546 | 05 |
| `pveNodes`, `pbsNodes`, `pmgNodes` | Memo | 560-562 | 05 |
| WebSocket-backed infrastructure data `state.nodes/state.hosts/state.pmg/...` | Store | 409, 563-572, 1900-1929 | 05 |
| `loadNodes`, `updateDiscoveredNodesFromServers`, `loadDiscoveredNodes`, `triggerDiscoveryScan` | Async orchestration | 1130-1350, 1352-1389 | 05 |
| `handleDiscoveryEnabledChange`, `commitDiscoverySubnet`, `handleDiscoveryModeChange` | Async orchestration | 1391-1654 | 05 |
| `handleNodeTemperatureMonitoringChange` | Async orchestration (optimistic update + rollback) | 1555-1605 | 05 |
| Event bus subscriptions (`node_auto_registered`, `refresh_nodes`, `discovery_updated`, `discovery_status`) | `onMount` workflow | 1659-1730 | 05 |
| Modal-open poll effect + unmount cleanup intervals | `createEffect` + `onCleanup` | 1734-1765 | 05 |
| WebSocket temperature re-merge effect | `createEffect` | 1898-1932 | 05 |
| `requestDeleteNode`, `cancelDeleteNode`, `deleteNode`, `testNodeConnection`, `refreshClusterNodes` | Mutation/test orchestration | 1994-2066 | 05 |
| Per-type `NodeModal` save handlers (`pve`/`pbs`/`pmg`) | Modal mutation orchestration | 3775-3979 | 05, 07 |

### 3. Backup Transfer State (Import / Export, Passphrase Modals)

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| `exportPassphrase`, `useCustomPassphrase`, `importPassphrase`, `importFile` | Signal | 815-818 | 06 |
| `showExportDialog`, `showImportDialog`, `showApiTokenModal`, `apiTokenInput`, `apiTokenModalSource` | Signal | 819-824 | 06 |
| API token helpers `getApiClientToken` / `setApiClientToken` in transfer flow | Store helper usage | 20-22, 2154-2157, 2225-2228, 4179-4189 | 06 |
| `handleExport` (passphrase validation, token gate, export payload/download) | Async orchestration | 2127-2209 | 06 |
| `handleImport` (file parse variants, token gate, import payload/reload) | Async orchestration | 2211-2290 | 06 |
| Export/import/API-token dialog close/reset handlers | Dialog transitions | 4111-4115, 4168-4185, 4251-4254 | 06 |

### 4. Modal / Dialog State (All Modal Show/Hide Signals)

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| `showNodeModal`, `editingNode`, `modalResetKey`, `currentNodeType` | Signal | 539-542 | 05 |
| `showDeleteNodeModal`, `nodePendingDelete`, `deleteNodeLoading` | Signal | 544-546 | 05 |
| `showPasswordModal`, `showQuickSecuritySetup`, `showQuickSecurityWizard` | Signal | 543, 826, 828 | 07 |
| `showUpdateConfirmation`, `isInstallingUpdate` | Signal | 768-769 | 04 |
| `showExportDialog`, `showImportDialog`, `showApiTokenModal` | Signal | 819-821 | 06 |
| Auth-disabled guard that auto-hides quick security setup | `createEffect` | 1200-1204 | 07 |
| Delete-node modal render + confirm/cancel wiring | Dialog render flow | 3708-3773 | 05, 07 |
| Node modal triad (`pve`/`pbs`/`pmg`) open/close/save lifecycle | Dialog render flow | 3775-3979 | 05, 07 |
| Update confirmation modal open/close/confirm wiring | Dialog render flow | 3981-3994 | 04, 07 |
| Export/import/API-token dialogs + close/reset + retry | Dialog render flow | 3996-4271 | 06, 07 |
| Change password modal close -> `loadSecurityStatus()` | Dialog side-effect | 4273-4279 | 07 |

### 5. Organization / Multi-Tenant State

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| License store functions `hasFeature`, `isMultiTenantEnabled`, `licenseLoaded`, `loadLicenseStatus` | Store | 101, 1122-1124 | 02 |
| `tabFeatureRequirements` multi-tenant + pro feature map | Config object | 886-894 | 01, 02 |
| `isFeatureLocked`, `isTabLocked` | Gate helpers | 896-905 | 02 |
| `tabGroups` filter/hide logic for `multi_tenant` items and locked badges | Memo | 1080-1100 | 01, 02 |
| Active-tab protection (`organization-*` fallback, locked tab fallback to `system-pro`) | `createEffect` | 1104-1120 | 02, 03 |
| `orgNodeUsage`, `orgGuestUsage` | Memo | 563-572 | 05 |
| `currentSettingsUser` (used by organization panels) | Memo | 812-814 | 07 |

### 6. Navigation / Routing State

| Reactive State / Effect | Kind | Approx. Line(s) | Extraction Packet |
| --- | --- | --- | --- |
| `currentTab`, `activeTab`, `selectedAgent` | Signal | 413-418 | 03 |
| `location` + `navigate` router handles | Router reactive source | 410-411 | 03 |
| `agentPaths`, `handleSelectAgent`, `setActiveTab` | Navigation orchestration | 420-449 | 03 |
| URL synchronization + landing/legacy redirects + tab/agent derivation | `createEffect(on(...))` | 458-531 | 03, 08 |
| `sidebarCollapsed` | Navigation UI state | 536 | 07 |
| `tabGroups`, `flatTabs` derived navigation model | Memo | 1080-1103 | 01, 03 |
| `setActiveTab` wiring in desktop/mobile nav button handlers | Render-driven navigation dispatch | 2410-2490 | 03, 07 |
| Routing contract helpers (`deriveTabFromPath`, `deriveAgentFromPath`, `deriveTabFromQuery`, `settingsTabPath`) | Pure routing functions | `settingsRouting.ts` 30-209 | 03, 08 |

## Appendix F: High-Risk Flow Register with Packet Mapping

| Flow Description | Severity | Mapped Extraction Packet(s) | Rollback Notes | Key Functions / Line Ranges |
| --- | --- | --- | --- | --- |
| `/settings` landing + `?tab=` deep-link canonicalization and no-flicker initialization | HIGH | 03, 08 | If extracted hook regresses first-load behavior, restore the original URL sync effect block from the packet checkpoint and keep `settingsRouting.ts` unchanged until tests pass. | `createEffect(on([location.pathname, location.search]))` 458-531; `deriveTabFromQuery` 118-176; `settingsTabPath` 178-209 |
| Legacy path redirect shims (`agent-hub`, `servers`, `containers`, `*Servers`) and alias precedence | HIGH | 03, 08 | Revert redirect-map extraction only (do not touch panel/state hooks), then replay routing contract tests before reattempting refactor. | Redirect branches 484-517; `deriveTabFromPath` 30-103 |
| Feature gate + multi-tenant enforcement (`organization-*` visibility, lock-to-Pro fallback) | HIGH | 02, 03, 08 | Roll back gate engine module and reinstate inline `tabGroups` filter + lock effect in `Settings.tsx` if fallback target changes unexpectedly. | `tabFeatureRequirements` 886-894; `isFeatureLocked`/`isTabLocked` 896-905; gate effect 1104-1120 |
| Infrastructure discovery lifecycle (REST load, POST scan trigger, event bus streaming updates, polling) | HIGH | 05 | Revert infrastructure state hook extraction as a unit; restore `loadDiscoveredNodes`, `triggerDiscoveryScan`, and event-bus subscriptions together to avoid partial lifecycle mismatch. | 1206-1350, 1352-1460, 1659-1765 |
| Node CRUD and connection workflows duplicated across PVE/PBS/PMG modal save paths | HIGH | 05, 07 | If extracted modal controller breaks type-specific behavior, restore per-type `NodeModal` handlers first, then reintroduce shared abstraction behind tests. | `deleteNode`/`testNodeConnection`/`refreshClusterNodes` 2005-2066; node modal `onSave` blocks 3799-3977 |
| Optimistic temperature-monitoring updates (global + per-node) with rollback-on-error | MEDIUM | 05 | Restore inline optimistic update logic before touching API client wiring; verify revert semantics on failed update calls. | `handleTemperatureMonitoringChange` 1526-1553; `handleNodeTemperatureMonitoringChange` 1555-1605 |
| System settings bootstrap/load/save payload assembly with env override reconciliation and delayed page reload | HIGH | 04 | Roll back `useSystemSettingsState` extraction in one commit and restore inline load/save block to maintain ordering and notifications. | onMount load path 1784-1895; `saveSettings` 1934-1971 |
| Update check/install orchestration (store sync, update plan fetch, confirmation modal, apply call) | MEDIUM | 04, 07 | Revert update-specific state slice first; keep modal view unchanged until `checkForUpdates` and `handleConfirmUpdate` parity is re-established. | `checkForUpdates` 2068-2103; `handleInstallUpdate`/`handleConfirmUpdate` 2105-2125; update modal 3981-3994 |
| Backup export/import security flow (passphrase rules, token fallback modal, payload format compatibility, reload) | HIGH | 06 | Revert backup flow extraction and restore inline handlers + modal transitions together so token retry path and validation remain aligned. | `handleExport` 2127-2209; `handleImport` 2211-2290; token modal retry 4177-4191 |
| Security-auth modal interplay (quick setup/wizard/password modal + auth-disabled guard) | MEDIUM | 07 | If UI state machine breaks, roll back security modal controller extraction and keep only structural panel registry changes. | `createEffect` auth-disabled guard 1200-1204; security panel props 3645-3662; password modal close 4273-4279 |
