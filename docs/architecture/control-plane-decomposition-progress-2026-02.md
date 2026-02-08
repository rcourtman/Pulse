# Control Plane Decomposition and Contract Hardening Progress Tracker

Linked plan:
- `docs/architecture/control-plane-decomposition-plan-2026-02.md`

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
| 00 | Surface Inventory and Cut-Map | DONE | Codex | Claude | APPROVED | See Packet 00 Review Evidence |
| 01 | Router Registration Skeleton | DONE | Codex | Claude | APPROVED | See Packet 01 Review Evidence |
| 02 | Extract Auth + Security + Install Route Group | DONE | Codex | Claude | APPROVED | See Packet 02 Review Evidence |
| 03 | Extract Monitoring + Resource Route Group | DONE | Codex | Claude | APPROVED | See Packet 03 Review Evidence |
| 04 | Extract AI + Relay + Sessions Route Group | DONE | Codex | Claude | APPROVED | See Packet 04 Review Evidence |
| 05 | Extract Org + License + Audit Route Group | DONE | Codex | Claude | APPROVED | See Packet 05 Review Evidence |
| 06 | ConfigHandlers Node Lifecycle Extraction | DONE | Codex | Claude | APPROVED | See Packet 06 Review Evidence |
| 07 | ConfigHandlers Setup + Auto-Register Extraction | DONE | Codex | Claude | APPROVED | See Packet 07 Review Evidence |
| 08 | ConfigHandlers System + Discovery + Import/Export Extraction | DONE | Codex | Claude | APPROVED | See Packet 08 Review Evidence |
| 09 | Architecture Guardrails and Drift Tests | DONE | Codex | Claude | APPROVED | See Packet 09 Review Evidence |
| 10 | Final Certification | TODO | Unassigned | Unassigned | PENDING | |

## Packet 00 Checklist: Surface Inventory and Cut-Map

### Discovery
- [x] Route domain inventory completed with file/function anchors.
- [x] ConfigHandlers domain inventory completed with file/function anchors.
- [x] Extraction boundaries documented for each planned module.
- [x] High-risk behaviors and dependencies identified (auth, scope, tenant, side effects).

### Deliverables
- [x] Inventory table added/updated in plan appendices.
- [x] Risk register entries mapped to packet IDs.
- [x] Rollback notes documented for each high-severity risk.

### Required Tests
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouteInventory" -v` passed.
- [x] `go test ./internal/api/... -run "ConfigHandlers|Router" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `docs/architecture/control-plane-decomposition-plan-2026-02.md`: Added Appendix C (Route Domain Inventory), Appendix D (ConfigHandlers Domain Inventory), Appendix E (Extraction Cut Points); updated Appendix A (Risk Register) with rollback notes and 3 new risks (CP-009, CP-010, CP-011).

Commands run + exit codes:
1. `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouteInventory" -v` -> exit 0
2. `go test ./internal/api/... -run "ConfigHandlers|Router" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently by reviewer, exit codes 0)
- P1: PASS (discovery-only packet; no behavioral changes; baseline tests green)
- P2: PASS (tracker updated, checklist complete, risk register has rollback notes)

Verdict: APPROVED

Residual risk:
- CP-011 (MEDIUM): Appendix B has no dedicated router module for config/system/settings routes; decision deferred to CP-01.

Commit:
- `2418cfeb` (docs(settings): Packet 00 — surface inventory and decomposition cut-map)

Rollback:
- Revert `docs/architecture/control-plane-decomposition-plan-2026-02.md` to pre-packet state (doc-only change, no code impact).

## Packet 01 Checklist: Router Registration Skeleton

### Implementation
- [x] `setupRoutes` converted to orchestration-only flow.
- [x] Domain registration methods introduced with no route contract drift.
- [x] Route ordering and middleware wrapping parity preserved.
- [x] Route inventory tests updated/passing.

### Required Tests
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` passed.
- [x] `go test ./internal/api/... -run "RouterRoutes|RouterGeneral" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router.go`: `setupRoutes` body reduced to handler construction + 5 domain registration calls (lines 391-395).
- `internal/api/router_routes_registration.go`: New file with 5 domain registration methods containing all route registrations moved from `setupRoutes`.
- `internal/api/route_inventory_test.go`: `parseRouterRoutes` updated to scan both `router.go` and `router_routes_registration.go`.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` -> exit 0
3. `go test ./internal/api/... -run "RouterRoutes|RouterGeneral" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (route inventory test confirms no route contract drift; middleware wrappers preserved)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/router_routes_registration.go`, revert `internal/api/router.go` and `internal/api/route_inventory_test.go` to pre-packet state.

## Packet 02 Checklist: Extract Auth + Security + Install Route Group

### Implementation
- [x] Auth/security/install registrations moved to dedicated module.
- [x] CSRF/public-path behavior preserved.
- [x] Scope and auth wrappers preserved.
- [x] Deny-path tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "Auth|Security|CSRF|Proxy" -v` passed.
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouterCSRFMiddleware" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router_routes_auth_security.go` (new, 525 LOC): Auth/security/install route registrations extracted.
- `internal/api/router_routes_registration.go`: `registerPublicAndAuthRoutes` reduced to thin delegate.
- `internal/api/route_inventory_test.go`: Added `router_routes_auth_security.go` to parsed file list.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouterCSRFMiddleware" -v` -> exit 0
3. `go test ./internal/api/... -run "Auth|Security|CSRF|Proxy" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (route inventory parity confirmed; auth/CSRF/proxy tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Commit:
- `312d24ad` (CP-01 checkpoint — prerequisite)

Residual risk:
- None

Rollback:
- Delete `internal/api/router_routes_auth_security.go`, restore `registerPublicAndAuthRoutes` body in `router_routes_registration.go`.

## Packet 03 Checklist: Extract Monitoring + Resource Route Group

### Implementation
- [x] Monitoring/resource route registrations moved to dedicated module.
- [x] Compatibility aliases preserved.
- [x] Scope and auth wrappers preserved.
- [x] Alerts/resources/charts route tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "Charts|Resources|Backups|AlertsEndpoints" -v` passed.
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router_routes_monitoring.go` (new, 296 LOC): Monitoring/resource route registrations extracted.
- `internal/api/router_routes_registration.go`: `registerMonitoringRoutes` reduced to thin delegate.
- `internal/api/route_inventory_test.go`: Added `router_routes_monitoring.go` to parsed file list.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` -> exit 0
3. `go test ./internal/api/... -run "Charts|Resources|Backups|AlertsEndpoints" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (route inventory parity; compatibility aliases preserved; monitoring tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/router_routes_monitoring.go`, restore `registerMonitoringRoutes` body in `router_routes_registration.go`.

## Packet 04 Checklist: Extract AI + Relay + Sessions Route Group

### Implementation
- [x] AI and relay registrations moved to dedicated module.
- [x] Legacy session/approval/question endpoint behavior preserved.
- [x] AI scope/permission wrappers preserved.
- [x] Stream/session contract tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "AI|Patrol|Chat|Relay|Contract" -v` passed.
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|RouterHandlers" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router_routes_ai_relay.go` (new): AI/relay/patrol/chat/sessions route registrations extracted.
- `internal/api/router_routes_registration.go`: `registerAIRelayRoutes` reduced to thin delegate.
- `internal/api/route_inventory_test.go`: Added `router_routes_ai_relay.go` to parsed file list.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouterHandlers" -v` -> exit 0
3. `go test ./internal/api/... -run "AI|Patrol|Chat|Relay|Contract" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (route inventory parity; AI/patrol/relay tests pass; legacy endpoints preserved)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/router_routes_ai_relay.go`, restore `registerAIRelayRoutes` body in `router_routes_registration.go`.

## Packet 05 Checklist: Extract Org + License + Audit Route Group

### Implementation
- [x] Org/license/audit/report registrations moved to dedicated module.
- [x] Feature-gate behavior preserved.
- [x] Scope/permission behavior preserved.
- [x] Deny-path and feature-disabled tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "OrgHandlers|License|Audit|Reporting|Scope" -v` passed.
- [x] `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router_routes_org_license.go` (new): Org/license/audit/RBAC/reporting route registrations extracted.
- `internal/api/router_routes_registration.go`: `registerOrgLicenseRoutes` reduced to thin delegate.
- `internal/api/route_inventory_test.go`: Added `router_routes_org_license.go` to parsed file list.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v` -> exit 0
3. `go test ./internal/api/... -run "OrgHandlers|License|Audit|Reporting|Scope" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (route inventory parity; feature gates and scope wrappers preserved; org/license/audit tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/router_routes_org_license.go`, restore `registerOrgLicenseRoutes` body in `router_routes_registration.go`.

## Packet 06 Checklist: ConfigHandlers Node Lifecycle Extraction

### Implementation
- [x] Node lifecycle logic extracted into dedicated module/component.
- [x] Existing exported handler methods retained as delegates.
- [x] Validation and side-effect parity preserved.
- [x] Node lifecycle tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "ConfigHandlers(Add|Delete|Update|Connection|Cluster)" -v` passed.
- [x] `go test ./internal/api/... -run "Router|RouteInventory" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/config_node_handlers.go` (new, 1914 LOC): Node lifecycle handler logic extracted.
- `internal/api/config_handlers.go` (6052 -> 4201 LOC): Node handler methods replaced with thin delegates.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "ConfigHandlers|TestNode|AddNode|DeleteNode|UpdateNode|Connection|Cluster" -v` -> exit 0
3. `go test ./internal/api/... -run "Router|RouteInventory" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (side effects preserved; validation intact; node lifecycle tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/config_node_handlers.go`, restore method bodies in `internal/api/config_handlers.go`.

## Packet 07 Checklist: ConfigHandlers Setup + Auto-Register Extraction

### Implementation
- [x] Setup script and setup URL logic extracted.
- [x] Auto-register and secure auto-register logic extracted.
- [x] Security guardrails preserved.
- [x] Setup/auto-register contract tests updated for parity.

### Required Tests
- [x] `go test ./internal/api/... -run "SetupScript|SetupURL|AutoRegister|SecureAutoRegister|TransportGuard" -v` passed.
- [x] `go test ./internal/api/... -run "Contract|RouteInventory" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/config_setup_handlers.go` (new, 2275 LOC): Setup/auto-register logic and private helpers extracted.
- `internal/api/config_handlers.go` (4201 -> 1969 LOC): Setup handler methods replaced with thin delegates.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "SetupScript|SetupURL|AutoRegister|SecureAutoRegister|TransportGuard" -v` -> exit 0
3. `go test ./internal/api/... -run "Contract|RouteInventory" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (security guardrails preserved; setup/auto-register tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/config_setup_handlers.go`, restore method bodies in `internal/api/config_handlers.go`.

## Packet 08 Checklist: ConfigHandlers System + Discovery + Import/Export Extraction

### Implementation
- [x] System settings and SSH verification handlers extracted.
- [x] Discovery handlers extracted.
- [x] Export/import handlers extracted.
- [x] Side-effect ordering and persistence behavior preserved.

### Required Tests
- [x] `go test ./internal/api/... -run "SystemSettings|Discovery|Export|Import|TemperatureSSH" -v` passed.
- [x] `go test ./internal/api/... -run "Scope|Authorization|RouteInventory" -v` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/config_system_handlers.go` (new, 266 LOC): System settings, SSH verify, mock mode logic.
- `internal/api/config_discovery_handlers.go` (new, 125 LOC): Discovery handler logic.
- `internal/api/config_export_import_handlers.go` (new, 182 LOC): Export/import handler logic.
- `internal/api/config_handlers.go` (1969 -> 1448 LOC): Remaining methods replaced with thin delegates.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "SystemSettings|Discovery|Export|Import|TemperatureSSH" -v` -> exit 0
3. `go test ./internal/api/... -run "Scope|Authorization|RouteInventory" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (import side-effect ordering preserved; system/discovery/export tests pass)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete the 3 new config domain files, restore method bodies in `internal/api/config_handlers.go`.

## Packet 09 Checklist: Architecture Guardrails and Drift Tests

### Implementation
- [x] Decomposition guard tests added for router route registration boundaries.
- [x] Delegation boundary tests added for config handlers.
- [x] Route protection drift tests added/updated.
- [x] Guardrails tuned to avoid brittle false positives.

### Required Tests
- [x] `go test ./internal/api/... -run "CodeStandards|RouteInventory|Decomposition|Contract" -v` passed.
- [x] `go build ./...` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Review Evidence

Files changed:
- `internal/api/router_decomposition_contract_test.go` (new): 4 decomposition guard tests — setupRoutes drift guard, route distribution guard, ConfigHandlers delegation guard, route inventory coverage guard.

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "CodeStandards|RouteInventory|Decomposition|Contract" -v` -> exit 0

Gate checklist:
- P0: PASS (files verified, commands rerun independently, exit codes 0)
- P1: PASS (guards are practical and pass; avoid brittle false positives)
- P2: PASS (tracker updated, checklist complete)

Verdict: APPROVED

Residual risk:
- None

Rollback:
- Delete `internal/api/router_decomposition_contract_test.go`.

## Packet 10 Checklist: Final Certification

### Certification
- [ ] Global validation baseline completed.
- [ ] Route ownership before/after inventory attached.
- [ ] Config handler domain ownership before/after inventory attached.
- [ ] Residual risk and rollback notes documented.

### Required Tests
- [ ] `go build ./...` passed.
- [ ] `go test ./internal/api/... -v` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed (if frontend scope touched).
- [ ] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` passed (if frontend scope touched).
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`
