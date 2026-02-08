# Unified Resource Convergence Phase 2 Plan (Alerts + AI + Legacy Payload Retirement)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md`

## Intent

Converge remaining runtime-critical surfaces onto the unified resource model, then retire redundant legacy payload dependencies without destabilizing parallel streams.

Primary outcomes:
1. Alerts rendering and grouping do not rely on direct legacy websocket arrays.
2. AI chat frontend context synthesis does not rely on direct legacy websocket arrays.
3. Websocket legacy payload retirement is staged with explicit safety gates.

## Active Parallel Work Constraints (Mandatory)

The following lanes are currently active and must not be disrupted:
1. Storage GA hardening (`docs/architecture/storage-page-ga-hardening-progress-2026-02.md`)
2. Settings stabilization (`docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md`)

Execution constraints for this plan:
1. Do not edit storage/backups migration files currently owned by storage GA packets.
2. Do not edit settings stabilization packet-owned files while that lane is active.
3. `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` is explicitly deferred until settings lane completes.

## Packet 00 Scope Boundaries (Frozen)

### In-Scope Surfaces

The following surfaces are in-scope for Unified Resource Convergence Phase 2 packets and are the only allowed migration touchpoints in this lane:

1. Alerts consumer legacy reads:
- `frontend-modern/src/pages/Alerts.tsx` (legacy array reads anchored at `:765`, `:919`, `:3111`)
2. AI chat consumer legacy reads:
- `frontend-modern/src/components/AI/Chat/index.tsx` (legacy array reads anchored at `:334`, `:338`)
3. Frontend websocket dual-payload state handling:
- `frontend-modern/src/stores/websocket.ts` (dual payload handling anchored at `:478`)
4. Frontend adapter fallback boundary:
- `frontend-modern/src/hooks/useResources.ts` (`useResourcesAsLegacy` anchored at `:238`)
5. Unified selector/query surface:
- `frontend-modern/src/hooks/useUnifiedResources.ts`
6. Associated test coverage surfaces:
- `frontend-modern/src/pages/__tests__/`
- `frontend-modern/src/components/AI/__tests__/`
- `frontend-modern/src/hooks/__tests__/`
7. Backend contract touchpoints:
- `internal/api/` resource/websocket contract paths only
- `internal/ai/` context/routing paths only

### Out-of-Scope Surfaces (Protected by Active Workers)

The following remain protected and must not be edited in this lane while worker ownership is active:

1. All Storage GA packet-owned files:
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md` lane scope
2. All Settings stabilization packet-owned files:
- `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md` lane scope
3. Explicit deferred file:
- `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`

## Packet 00 Legacy Dependency Baseline Matrix (Drift Baseline)

### Alerts Legacy Array Reads (`frontend-modern/src/pages/Alerts.tsx`)

| Legacy Array | Code Anchors | Current Purpose |
|---|---|---|
| `state.nodes` | `:765`, `:919`, `:3111` | Guard override-hydration effect execution, resolve node override keys, and pass node list into `ThresholdsTable` for thresholds UI. |
| `state.vms` | `:766`, `:945`, `:1508` | Guard override-hydration effect execution, resolve guest override keys as VM matches, and compose `allGuests` list used by thresholds/edit flows. |
| `state.containers` | `:767`, `:946`, `:1509` | Guard override-hydration effect execution, resolve guest override keys as container matches, and compose `allGuests` list used by thresholds/edit flows. |
| `state.storage` | `:768`, `:931`, `:3113` | Guard override-hydration effect execution, resolve storage override keys, and pass storage list into `ThresholdsTable`. |
| `state.hosts` | `:785`, `:2197` | Build host-agent map for host/disk override resolution and pass hosts list into `ThresholdsTab` for host thresholds editing/rendering. |
| `state.dockerHosts` | `:773`, `:3114` | Build docker host/container lookup maps for override resolution and pass docker host list into `ThresholdsTable`. |

### AI Chat Legacy Array Reads (`frontend-modern/src/components/AI/Chat/index.tsx`)

| Legacy Array | Code Anchors | Current Purpose |
|---|---|---|
| `state.nodes` | `:334`, `:380` | Set cluster-mode signal from node count and add node mention entries to chat autocomplete context. |
| `state.vms` | `:338` | Build VM mention entries for autocomplete/context synthesis. |
| `state.containers` | `:349` | Build LXC/container mention entries for autocomplete/context synthesis. |
| `state.dockerHosts` | `:360` | Build docker host and nested docker container mention entries for autocomplete/context synthesis. |
| `state.hosts` | `:390` | Build standalone host-agent mention entries for autocomplete/context synthesis. |

### WebSocket Dual Payload Baseline (`frontend-modern/src/stores/websocket.ts`)

| Surface | Code Anchors | Current Shape / Behavior |
|---|---|---|
| Store state shape includes both contracts | `frontend-modern/src/stores/websocket.ts:99`, `frontend-modern/src/stores/websocket.ts:151`, `frontend-modern/src/types/api.ts:5`, `frontend-modern/src/types/api.ts:33`, `frontend-modern/src/types/api.ts:1100` | `State` includes legacy arrays (`nodes`, `vms`, `containers`, `dockerHosts`, `hosts`, `storage`, `pbs`, `pmg`, etc.) plus optional unified `resources[]`; websocket `initialState` and `rawData` both carry `State`. |
| Legacy array payload application | `frontend-modern/src/stores/websocket.ts:358`, `frontend-modern/src/stores/websocket.ts:365`, `frontend-modern/src/stores/websocket.ts:368`, `frontend-modern/src/stores/websocket.ts:406`, `frontend-modern/src/stores/websocket.ts:448`, `frontend-modern/src/stores/websocket.ts:451`, `frontend-modern/src/stores/websocket.ts:452` | Incoming `message.data` updates legacy arrays individually (reconciled by `id`) for nodes/VMs/containers/hosts/docker hosts/storage/PBS/PMG and other legacy state fields. |
| Unified resources payload application | `frontend-modern/src/stores/websocket.ts:478` | In parallel with legacy fields, `message.data.resources` is reconciled into `state.resources` as unified resource objects keyed by `id`. |

### Adapter Fallback Baseline (`frontend-modern/src/hooks/useResources.ts`)

| Surface | Code Anchors | Current Fallback / Conversion Behavior |
|---|---|---|
| Adapter definition and fallback policy | `:237` (docstring), `:246` (function def) | `useResourcesAsLegacy` explicitly prefers legacy arrays when present and only synthesizes legacy objects from unified resources when legacy fields are absent/empty. |
| VM conversion | `:273` (asVMs def), `:276` (fallback branch) | `asVMs` returns `state.vms` until unified-only condition is met; otherwise maps unified `vm` resources into legacy VM shape. |
| Container conversion | `:335` (asContainers def), `:338` (fallback branch) | `asContainers` returns `state.containers` unless unified-only path is required; maps `container` and `oci-container` resources to legacy container shape. |
| Host conversion | `:406` (asHosts def), `:409` (fallback branch) | `asHosts` returns `state.hosts` unless unified-only path is required; maps unified `host` resources to legacy host shape. |
| Node conversion | `:500` (asNodes def), `:503` (fallback branch) | `asNodes` returns `state.nodes` unless unified-only path is required; maps unified `node` resources to legacy node shape (including synthesized legacy temperature struct). |
| Docker host conversion | `:569` (asDockerHosts def), `:572` (fallback branch) | `asDockerHosts` returns `state.dockerHosts` unless unified-only path is required; maps `docker-host` plus child `docker-container` resources to legacy nested docker host shape. |
| PBS conversion | `:817` (asPBS def), `:819` (fallback branch) | `asPBS` returns `state.pbs` when unified resources are absent/inapplicable; otherwise maps unified `pbs`/`datastore` resources into legacy PBS instance/datastore structures. |
| Storage conversion | `:964` (asStorage def) | `asStorage` merges legacy storage with synthesized storage/datastore records, including PBS datastore derivations. |
| PMG conversion | `:1100` (asPMG def), `:1102` (fallback branch) | `asPMG` returns `state.pmg` when unified resources are absent/inapplicable; otherwise maps unified `pmg` resources into legacy PMG structures. |

## Code-Derived Baseline (Authoritative)

### A. Unified foundation already exists

1. v2 resource API routes and handlers exist and are active:
- `internal/api/router_routes_monitoring.go:39`
- `internal/api/resources_v2.go:52`

2. Unified registry/adapters are wired in router:
- `internal/api/router.go:183`
- `internal/api/router.go:334`

3. Infrastructure/workloads already consume v2 resources directly:
- `frontend-modern/src/pages/Infrastructure.tsx:33`
- `frontend-modern/src/hooks/useV2Workloads.ts:5`

### B. Remaining legacy-heavy frontend consumers

1. Alerts still reads direct legacy arrays (`state.nodes`, `state.vms`, `state.containers`, `state.storage`, `state.hosts`, `state.dockerHosts`):
- `frontend-modern/src/pages/Alerts.tsx:765`
- `frontend-modern/src/pages/Alerts.tsx:919`
- `frontend-modern/src/pages/Alerts.tsx:3111`

2. AI chat still synthesizes UI context from legacy arrays:
- `frontend-modern/src/components/AI/Chat/index.tsx:334`
- `frontend-modern/src/components/AI/Chat/index.tsx:338`

3. Websocket broadcasts both unified resources and legacy arrays in parallel:
- `frontend-modern/src/stores/websocket.ts:478`
- `frontend-modern/src/hooks/useResources.ts:238`

## Risk Register

| ID | Severity | Finding | Evidence |
|---|---|---|---|
| URC2-001 | High | Alerts UI still couples core behavior to legacy arrays. | `frontend-modern/src/pages/Alerts.tsx:765` |
| URC2-002 | High | AI chat summary/context still couples to legacy arrays. | `frontend-modern/src/components/AI/Chat/index.tsx:334` |
| URC2-003 | High | Dual websocket shape increases drift risk and long-term maintenance cost. | `frontend-modern/src/stores/websocket.ts:478` |
| URC2-004 | Medium | Adapter fallback (`useResourcesAsLegacy`) can hide migration regressions if left unbounded. | `frontend-modern/src/hooks/useResources.ts:238` |
| URC2-005 | Medium | Settings-owned org sharing migration cannot be safely executed while settings lane is active. | `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx:133` |

## Packet Execution Model

Use fixed roles per packet:
- Implementer: Codex
- Reviewer: Claude

A packet is `DONE` only when:
1. all packet checkboxes are complete,
2. required commands have explicit exit codes,
3. reviewer gate checklist passes,
4. verdict is `APPROVED`.

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
2. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts`

On packets that change backend websocket/resource contracts, also run:

3. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers" -count=1`
4. `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1`

Notes:
- `go build ./...` alone is insufficient.
- Missing exit code evidence is a failed gate.

## Packets

### Packet 00: Contract Freeze, Scope Fences, and Drift Baseline

Objective:
- Freeze exact migration scope and create measurable baseline signals before behavior changes.

Scope:
- `docs/architecture/unified-resource-convergence-phase-2-plan-2026-02.md`
- `docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md`
- Test scaffolds only (no production behavior change)

Implementation checklist:
1. Record explicit in-scope/out-of-scope boundaries with active worker protection.
2. Add baseline matrix for alerts/ai current data dependencies.
3. Add placeholder integration specs for expected unified-selector behavior.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Baseline and guardrails are explicit and reviewer-approved.

### Packet 01: Introduce Unified Selector Layer (No Behavior Change)

Objective:
- Introduce a reusable selector layer that resolves resources from unified state first, with compatibility fallback isolated behind one boundary.

Scope:
- `frontend-modern/src/hooks/useResources.ts`
- `frontend-modern/src/hooks/useUnifiedResources.ts`
- `frontend-modern/src/hooks/__tests__/useResources.test.ts`
- `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts`

Implementation checklist:
1. Create explicit selector APIs for alert and AI consumers.
2. Keep output semantics equivalent to current consumer expectations.
3. Maintain strict type contracts and avoid duplicated conversion logic.

Required tests:
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Selector layer exists and is production-ready without behavior drift.

### Packet 02: Alerts Consumer Migration to Unified Selectors

Objective:
- Move alerts page read paths onto the selector layer and remove direct dependency on legacy arrays for core decisions.

Scope:
- `frontend-modern/src/pages/Alerts.tsx`
- `frontend-modern/src/pages/__tests__/Alerts.helpers.test.ts`
- Optional new alerts integration tests under `frontend-modern/src/pages/__tests__/`

Implementation checklist:
1. Replace direct legacy array reads with unified selector outputs.
2. Preserve filtering, grouping, override mapping, and incident timeline behavior.
3. Keep one packet of compatibility fallback for rollback safety.

Required tests:
1. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Alerts behavior remains equivalent, with selectors as the primary data source.

### Packet 03: AI Chat UI Context Migration to Unified Selectors

Objective:
- Move AI chat frontend context synthesis (cluster/workload/resource summary inputs) to unified selectors.

Scope:
- `frontend-modern/src/components/AI/Chat/index.tsx`
- `frontend-modern/src/components/AI/__tests__/aiChatUtils.test.ts`
- Optional tests under `frontend-modern/src/stores/__tests__/aiChat.test.ts`

Implementation checklist:
1. Replace direct legacy array loops with selector outputs.
2. Preserve mention, summary, and session-context behavior.
3. Keep output semantics stable for downstream prompt composition.

Required tests:
1. `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- AI chat UI context path is unified-selector driven.

### Packet 04: WebSocket Legacy Payload Deprecation Gates

Objective:
- Add explicit backend/frontend gating for legacy payload retirement to prevent accidental hard cuts.

Scope:
- `internal/websocket/` (targeted contract paths only)
- `internal/api/` resource-state contract touchpoints only
- `frontend-modern/src/stores/websocket.ts`
- Contract tests covering payload shape and compatibility modes

Implementation checklist:
1. Define compatibility mode switch for legacy arrays in payload.
2. Add telemetry/logging to detect remaining legacy consumers.
3. Preserve default compatibility mode until Packet 06 certification.

Required tests:
1. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Retirement is controlled by explicit gates with verified compatibility path.

### Packet 05: Legacy Compatibility Narrowing

Objective:
- Narrow fallback behavior in selector/adapter layer to only documented transitional paths.

Scope:
- `frontend-modern/src/hooks/useResources.ts`
- `frontend-modern/src/stores/websocket.ts`
- Associated tests

Implementation checklist:
1. Remove opportunistic fallbacks that can mask regressions.
2. Keep rollback path explicit and bounded.
3. Document exact remaining fallback branches.

Required tests:
1. `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts`
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Fallback matrix is minimal, explicit, and test-locked.

### Packet 06: Contract Test Hardening and Regression Net

Objective:
- Lock migration behavior with explicit contract tests across alerts, AI, and websocket payload modes.

Scope:
- `frontend-modern/src/pages/__tests__/`
- `frontend-modern/src/components/AI/__tests__/`
- `internal/api/*_test.go` (targeted resource/websocket contract tests)
- `internal/ai/*_test.go` (targeted context/routing tests)

Implementation checklist:
1. Add parity tests for pre/post migration behavior.
2. Add payload contract tests for compatibility mode behavior.
3. Add negative-path assertions for stale legacy-only consumers.

Required tests:
1. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts`
2. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1`

Exit criteria:
- Migration behavior is guarded by deterministic tests and contracts.

### Packet 07: Final Certification and Release Recommendation

Objective:
- Produce final go/no-go certification for this lane.

Scope:
- Plan/progress docs plus any missing validation-only fixes discovered in-scope.

Implementation checklist:
1. Execute full validation baseline with explicit exit codes.
2. Summarize residual risks and rollback instructions.
3. Issue final verdict for this lane.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts src/hooks/__tests__/useResources.test.ts src/hooks/__tests__/useUnifiedResources.test.ts`
3. `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1`
4. `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1`

Exit criteria:
- Reviewer confirms lane is complete with explicit APPROVED evidence.

## Deferred Follow-On (Blocked by Active Settings Lane)

Item: Organization Sharing endpoint migration (`/api/resources` -> `/api/v2/resources`)

Blocked scope:
- `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`

Unblock condition:
- Settings stabilization lane reaches completion and releases file ownership.

Follow-on packet preview:
1. Replace resource picker source endpoint with v2 query path.
2. Preserve role/resource validation behavior.
3. Add settings-panel regression tests and lock endpoint contract.
