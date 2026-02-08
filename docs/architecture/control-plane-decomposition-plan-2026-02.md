# Control Plane Decomposition and Contract Hardening Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/control-plane-decomposition-progress-2026-02.md`

## Product Intent

Pulse's API control plane must be modular enough to evolve without turning `router.go` and `config_handlers.go` into permanent high-risk bottlenecks.

This plan has two top-level goals:
1. Decompose control-plane wiring into bounded modules with stable seams.
2. Lock route/auth/tenant contracts so refactors cannot silently drift behavior.

## Non-Negotiable Contracts

1. Route compatibility contract:
- Existing method+path behavior remains stable unless a packet explicitly declares a contract migration.
- Public endpoint behavior remains unchanged.

2. Auth/scope contract:
- No endpoint can lose auth middleware, permission checks, or scope checks during extraction.
- Token/admin behavior must stay equivalent.

3. Tenant isolation contract:
- Multi-tenant request binding and org authorization semantics remain intact.
- No packet may weaken tenant middleware or org handler protections.

4. Handler side-effect contract:
- Config writes, monitor wiring, websocket broadcasts, and reload callbacks must preserve current side effects.
- No duplicate side effects introduced by wrapper/delegate extraction.

5. In-flight work coexistence contract:
- Alerts and multi-tenant in-flight work are treated as parallel streams; packet scopes must avoid opportunistic rewrites outside packet intent.
- Out-of-scope failures must be documented with evidence, not "fixed" by broad edits.

6. Rollback contract:
- Every packet includes a reversible rollback path at file granularity.
- No packet requires destructive git operations to rollback.

## Current Baseline (Code-Derived)

1. `internal/api/router.go`: 9389 LOC.
2. `internal/api/config_handlers.go`: 6052 LOC.
3. `type Router struct` starts at `internal/api/router.go:63`.
4. `setupRoutes()` starts at `internal/api/router.go:249`.
5. `ConfigHandlers` constructor starts at `internal/api/config_handlers.go:113`.
6. Route and allowlist contracts are currently guarded by `internal/api/route_inventory_test.go`.

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

1. `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouteInventory" -v`
2. `go test ./internal/api/... -run "Router|ConfigHandlers|Contract|Scope|Tenant|OrgHandlers" -v`
3. `go build ./...`

When frontend settings files are touched, also run:

4. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
5. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`

Notes:
- `go build` alone is never sufficient for approval.
- Empty, timed-out, or truncated command output without exit code is invalid evidence.

## Execution Packets

### Packet 00: Surface Inventory and Cut-Map

Objective:
- Produce an explicit decomposition map for router and config handler responsibilities before code movement.

Scope:
- `internal/api/router.go`
- `internal/api/config_handlers.go`
- `internal/api/route_inventory_test.go`
- `docs/architecture/control-plane-decomposition-plan-2026-02.md` (appendix updates only)

Implementation checklist:
1. Build route domain inventory: auth/security, monitoring/resources, config/system, AI/relay, org/license.
2. Build config handler domain inventory: node lifecycle, setup/auto-register, discovery, system settings, import/export.
3. Record extraction cut points with file+function anchors.
4. Create risk register with packet mapping.

Required tests:
1. `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouteInventory" -v`
2. `go test ./internal/api/... -run "ConfigHandlers|Router" -v`

Exit criteria:
- Every high-severity risk is mapped to a packet with rollback notes.

### Packet 01: Router Registration Skeleton

Objective:
- Convert `setupRoutes` into a thin orchestrator that delegates to domain route registration methods without behavior changes.

Scope:
- `internal/api/router.go`
- `internal/api/router_routes_registration.go` (new)
- `internal/api/router_routes_additional_test.go`

Implementation checklist:
1. Add registration methods partitioned by domain (public/auth, monitoring, config, AI, org/license).
2. Keep route strings, middleware wrappers, and handler targets unchanged.
3. Keep initialization ordering deterministic.
4. Add tests/assertions that route inventory remains identical.

Required tests:
1. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v`
2. `go test ./internal/api/... -run "RouterRoutes|RouterGeneral" -v`

Exit criteria:
- `setupRoutes` is orchestration-only and route contract tests remain green.

### Packet 02: Extract Auth + Security + Install Route Group

Objective:
- Move authentication/security/install route wiring into a dedicated route module.

Scope:
- `internal/api/router.go`
- `internal/api/router_routes_auth_security.go` (new)
- `internal/api/router_auth_additional_test.go`
- `internal/api/router_proxy_auth_security_test.go`

Implementation checklist:
1. Move login/logout/security/setup/install registrations from monolithic section.
2. Preserve CSRF skip behavior and protected/bare route intent.
3. Preserve scope requirements for privileged routes.
4. Add/extend negative-path tests for missing scope and unauthorized access.

Required tests:
1. `go test ./internal/api/... -run "Auth|Security|CSRF|Proxy" -v`
2. `go test ./internal/api/... -run "TestRouterRouteInventory|TestRouterCSRFMiddleware" -v`

Exit criteria:
- Auth/security/install behavior is contract-equivalent after extraction.

### Packet 03: Extract Monitoring + Resource Route Group

Objective:
- Move monitoring/resource/alerts/charts route wiring into a dedicated route module with parity preserved.

Scope:
- `internal/api/router.go`
- `internal/api/router_routes_monitoring.go` (new)
- `internal/api/charts_test.go`
- `internal/api/resources_v2_test.go`
- `internal/api/alerts_endpoints_test.go`

Implementation checklist:
1. Move charts/storage/resources/backups/alerts route registrations to module.
2. Preserve route aliases and compatibility endpoints (`/api/backups`, `/api/backups/unified`, `/api/v2/resources`).
3. Preserve scope and auth wrappers.
4. Add/extend route-contract tests for compatibility paths.

Required tests:
1. `go test ./internal/api/... -run "Charts|Resources|Backups|AlertsEndpoints" -v`
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v`

Exit criteria:
- Monitoring and resource routes are module-owned with no contract drift.

### Packet 04: Extract AI + Relay + Sessions Route Group

Objective:
- Move AI and relay route registration into a dedicated module and lock stream/session route contracts.

Scope:
- `internal/api/router.go`
- `internal/api/router_routes_ai_relay.go` (new)
- `internal/api/ai_handlers_stream_test.go`
- `internal/api/router_handlers_additional_test.go`

Implementation checklist:
1. Move AI route registrations (chat, sessions, approvals, providers, patrol, findings).
2. Move relay route registrations.
3. Preserve scope and permission gates for each route.
4. Add assertions for legacy AI endpoints that must remain.

Required tests:
1. `go test ./internal/api/... -run "AI|Patrol|Chat|Relay|Contract" -v`
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouterHandlers" -v`

Exit criteria:
- AI and relay routes are module-owned with stream contract parity.

### Packet 05: Extract Org + License + Audit Route Group

Objective:
- Isolate org/license/audit/reporting registrations and lock authorization boundaries.

Scope:
- `internal/api/router.go`
- `internal/api/router_routes_org_license.go` (new)
- `internal/api/org_handlers_test.go`
- `internal/api/audit_reporting_scope_test.go`
- `internal/api/license_handlers_test.go`

Implementation checklist:
1. Move org/license/audit/report route registration section into module.
2. Preserve feature gating semantics (multi-tenant and license checks).
3. Preserve required settings scopes on audit/report routes.
4. Add coverage for deny paths and feature-disabled behavior.

Required tests:
1. `go test ./internal/api/... -run "OrgHandlers|License|Audit|Reporting|Scope" -v`
2. `go test ./internal/api/... -run "TestRouterRouteInventory|RouteInventory" -v`

Exit criteria:
- Org/license/audit registrations are module-owned without authz regressions.

### Packet 06: ConfigHandlers Node Lifecycle Extraction

Objective:
- Extract node CRUD/test/refresh logic from `ConfigHandlers` into a dedicated node service with wrapper compatibility.

Scope:
- `internal/api/config_handlers.go`
- `internal/api/config_node_handlers.go` (new)
- `internal/api/config_handlers_add_test.go`
- `internal/api/config_handlers_delete_test.go`
- `internal/api/config_handlers_connection_test.go`

Implementation checklist:
1. Introduce node lifecycle component (`GetNodes`, `AddNode`, `UpdateNode`, `DeleteNode`, `TestConnection`, `RefreshClusterNodes`).
2. Keep existing exported handler methods as thin delegates initially.
3. Preserve validation, normalization, and side effects.
4. Preserve websocket broadcast/reload behavior.

Required tests:
1. `go test ./internal/api/... -run "ConfigHandlers(Add|Delete|Update|Connection|Cluster)" -v`
2. `go test ./internal/api/... -run "Router|RouteInventory" -v`

Exit criteria:
- Node lifecycle behavior is unchanged and isolated behind dedicated component.

### Packet 07: ConfigHandlers Setup + Auto-Register Extraction

Objective:
- Isolate setup script and auto-register flows into a dedicated setup service without changing contract semantics.

Scope:
- `internal/api/config_handlers.go`
- `internal/api/config_setup_handlers.go` (new)
- `internal/api/config_handlers_setup_script_test.go`
- `internal/api/config_handlers_setup_url_test.go`
- `internal/api/config_handlers_auto_register_test.go`
- `internal/api/config_handlers_secure_auto_register_test.go`

Implementation checklist:
1. Move setup script generation and setup URL logic to service module.
2. Move auto-register and secure auto-register logic to service module.
3. Preserve security constraints (transport guard, token/scope rules, IP checks).
4. Add parity tests for payload/response shapes.

Required tests:
1. `go test ./internal/api/... -run "SetupScript|SetupURL|AutoRegister|SecureAutoRegister|TransportGuard" -v`
2. `go test ./internal/api/... -run "Contract|RouteInventory" -v`

Exit criteria:
- Setup and auto-register behavior remains contract-stable and isolated.

### Packet 08: ConfigHandlers System + Discovery + Import/Export Extraction

Objective:
- Isolate system settings, discovery, and config import/export flows into dedicated modules and lock side effects.

Scope:
- `internal/api/config_handlers.go`
- `internal/api/config_system_handlers.go` (new)
- `internal/api/config_discovery_handlers.go` (new)
- `internal/api/config_export_import_handlers.go` (new)
- `internal/api/config_handlers_discovery_test.go`
- `internal/api/export_test.go`

Implementation checklist:
1. Move system settings read/write and SSH verification handlers to system module.
2. Move discovery endpoints to discovery module.
3. Move export/import to dedicated module.
4. Preserve persistence and reload sequencing behavior.

Required tests:
1. `go test ./internal/api/... -run "SystemSettings|Discovery|Export|Import|TemperatureSSH" -v`
2. `go test ./internal/api/... -run "Scope|Authorization|RouteInventory" -v`

Exit criteria:
- ConfigHandlers retains only composition/delegation logic for extracted domains.

### Packet 09: Architecture Guardrails and Drift Tests

Objective:
- Prevent regression into new monoliths by adding enforceable architecture tests.

Scope:
- `internal/api/code_standards_test.go`
- `internal/api/route_inventory_test.go`
- `internal/api/router_decomposition_contract_test.go` (new)
- `internal/api/config_handlers_helpers_additional_test.go`

Implementation checklist:
1. Add decomposition guard that fails if route registration is re-centralized into one giant block.
2. Add guard for handler delegation boundaries in `ConfigHandlers`.
3. Add contract tests for route protection consistency.
4. Keep guards implementation-agnostic enough to avoid brittle false positives.

Required tests:
1. `go test ./internal/api/... -run "CodeStandards|RouteInventory|Decomposition|Contract" -v`
2. `go build ./...`

Exit criteria:
- CI has explicit tests to catch future architectural regressions.

### Packet 10: Final Certification

Objective:
- Certify control-plane decomposition as behavior-preserving and future-safe.

Implementation checklist:
1. Run global validation baseline and collect explicit exit codes.
2. Produce before/after inventory showing ownership of route groups and config domains.
3. Record residual risks and deferred items.
4. Update progress tracker and final verdict.

Required tests:
1. `go build ./...`
2. `go test ./internal/api/... -v`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` (if frontend scope touched)
4. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts` (if frontend scope touched)

Exit criteria:
- Reviewer signs `APPROVED` with all gates passing and checkpoint evidence recorded.

## Acceptance Definition

Plan is complete only when:
1. Packet 00-10 are `DONE` in the linked progress tracker.
2. Every packet includes reviewer evidence and verdict.
3. Route/auth/tenant contracts are verified unchanged where no migration is declared.
4. `router.go` and `config_handlers.go` are composition-oriented, with domain logic owned by extracted modules.

## Appendix A: Risk Register

| Risk ID | Surface | Description | Severity | Mapped Packet ID(s) | Mitigation | Rollback Notes (File-Level) |
| --- | --- | --- | --- | --- | --- | --- |
| CP-001 | `internal/api/router.go:249` (`setupRoutes`) | Single giant route registration block increases auth/scope regression risk during edits. | HIGH | CP-01, CP-02, CP-03, CP-04, CP-05 | Incremental domain extraction + route inventory parity tests each packet. | Revert extracted router route module file(s) and corresponding `setupRoutes` delegation lines in `internal/api/router.go`; rerun `TestRouterRouteInventory` before merge. |
| CP-002 | `internal/api/router.go` mixed concerns | Router mixes route registration, runtime wiring, streaming lifecycle, and utility logic. | HIGH | CP-01, CP-02, CP-03, CP-04, CP-05, CP-09 | Separate registration modules and enforce decomposition guards. | Revert new `router_routes_*.go` files and restore prior wiring in `internal/api/router.go`; keep behavioral helpers unchanged. |
| CP-003 | `internal/api/config_handlers.go` node/setup/system coupling | One file owns too many handler domains, increasing blast radius for any change. | HIGH | CP-06, CP-07, CP-08 | Extract domain services with wrapper compatibility and targeted tests. | Revert extracted config domain files (`config_*_handlers.go`) and restore delegate method bodies in `internal/api/config_handlers.go`. |
| CP-004 | Auth wrapper drift | Refactors can accidentally drop `RequireAuth`/`RequireScope`/`RequirePermission` or inline `ensureScope` checks. | HIGH | CP-01, CP-02, CP-03, CP-04, CP-05, CP-09 | Route protection contract tests and explicit deny-path tests. | Revert affected route registration module and its call-site in `internal/api/router.go`; rerun scope/auth deny-path tests. |
| CP-005 | Tenant/feature gate drift | Org and license routes could regress under route movement (`RequireLicenseFeature`, `RequirePermission`). | HIGH | CP-05 | Preserve gate chains; validate with org/license/audit tests. | Revert `router_routes_org_license.go` and associated `setupRoutes` registration changes in `internal/api/router.go`. |
| CP-006 | Setup/auto-register security drift | Setup and auto-register logic is high-risk and easy to weaken while extracting. | HIGH | CP-07 | Keep security tests mandatory (`SecureAutoRegister`, transport guard, scopes). | Revert `config_setup_handlers.go` and restore setup/auto-register handlers in `internal/api/config_handlers.go`; rerun setup security tests. |
| CP-007 | Side-effect duplication/loss | Delegate wrappers may double-trigger or miss reload/broadcast side effects. | MEDIUM | CP-06, CP-07, CP-08 | Add side-effect assertions and keep wrappers thin/explicit. | Revert only the extracted handler file for the affected domain and rebind the original method implementation in `internal/api/config_handlers.go`. |
| CP-008 | Re-monolith over time | Without guardrails, future edits can re-centralize logic. | MEDIUM | CP-09 | Add code standards and decomposition contract tests. | Revert decomposition guard test files if they are overly brittle; keep route inventory test baselines intact. |
| CP-009 | Global auth/public/CSRF middleware (`internal/api/router.go:4045-4192`) | Public route allowlist and CSRF skip rules are separate from `setupRoutes`; moving routes can silently desync protection semantics. | HIGH | CP-01, CP-02, CP-04, CP-09 | Keep route inventory + public/bare allowlists + CSRF middleware tests in lockstep for each extraction packet. | Revert middleware/public-path edits in `internal/api/router.go` and the corresponding extracted route module in same packet; rerun route inventory and CSRF tests. |
| CP-010 | Inline verb multiplexers in route handlers | Many routes use inline `switch req.Method` and path-suffix branching; extraction can alter verb/path dispatch behavior. | HIGH | CP-01, CP-03, CP-04, CP-05 | Preserve inline verb routers initially; only refactor internals after parity tests exist. | Revert the extracted route registration block in the module file and restore original inline handler closure in `internal/api/router.go`. |
| CP-011 | Target-map ownership gap | Appendix B has no dedicated router module for config/system/settings routes, increasing ambiguity and churn risk. | MEDIUM | CP-01 (decision), CP-09 | Document temporary ownership in inventory; decide whether to add `router_routes_config_system.go` in a follow-up packet. | Revert only route delegation call ordering in `internal/api/router.go` if ownership split creates regressions; keep handlers unchanged. |

## Appendix B: Domain Extraction Target Map

| Target Module | Primary Ownership | Source Anchor |
| --- | --- | --- |
| `router_routes_auth_security.go` | auth/security/setup/install routes | `internal/api/router.go` auth/security sections |
| `router_routes_monitoring.go` | charts/resources/backups/alerts routes | `internal/api/router.go` monitoring/resource sections |
| `router_routes_ai_relay.go` | AI/patrol/chat/relay routes | `internal/api/router.go` AI/relay sections |
| `router_routes_org_license.go` | org/license/audit/report routes | `internal/api/router.go` org/license/audit sections |
| `config_node_handlers.go` | node CRUD/test/refresh | `internal/api/config_handlers.go` node methods |
| `config_setup_handlers.go` | setup script + auto-register | `internal/api/config_handlers.go` setup/auto-register methods |
| `config_system_handlers.go` | system settings + SSH verify | `internal/api/config_handlers.go` system/ssh methods |
| `config_discovery_handlers.go` | discovery flows | `internal/api/config_handlers.go` discovery methods |
| `config_export_import_handlers.go` | config export/import | `internal/api/config_handlers.go` export/import methods |

## Appendix C: Route Domain Inventory

### auth/security/install

| Method | Path Pattern | Handler Function | Auth Wrapper(s) / Public/CSRF | Target Extraction Module |
| --- | --- | --- | --- | --- |
| ANY | `/api/health` (`internal/api/router.go:277`) | `r.handleHealth` | public | `router_routes_auth_security.go` |
| ANY | `/api/version` (`internal/api/router.go:337`) | `r.handleVersion` | public | `router_routes_auth_security.go` |
| ANY | `/api/install/install-docker.sh` (`internal/api/router.go:349`) | `r.handleDownloadDockerInstallerScript` | public | `router_routes_auth_security.go` |
| ANY | `/api/install/install.sh` (`internal/api/router.go:350`) | `r.handleDownloadUnifiedInstallScript` | none | `router_routes_auth_security.go` |
| ANY | `/api/install/install.ps1` (`internal/api/router.go:351`) | `r.handleDownloadUnifiedInstallScriptPS` | none | `router_routes_auth_security.go` |
| ANY | `/api/security/validate-bootstrap-token` (`internal/api/router.go:513`) | `r.handleValidateBootstrapToken` | public; CSRF skip | `router_routes_auth_security.go` |
| ANY | `/api/security/change-password` (`internal/api/router.go:633`) | `r.handleChangePassword` | none | `router_routes_auth_security.go` |
| ANY | `/api/logout` (`internal/api/router.go:634`) | `r.handleLogout` | none | `router_routes_auth_security.go` |
| ANY | `/api/login` (`internal/api/router.go:635`) | `r.handleLogin` | public; CSRF skip | `router_routes_auth_security.go` |
| ANY | `/api/security/reset-lockout` (`internal/api/router.go:636`) | `r.handleResetLockout` | none | `router_routes_auth_security.go` |
| ANY | `/api/security/oidc` (`internal/api/router.go:637`) | `r.handleOIDCConfig` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/oidc/login` (`internal/api/router.go:638`) | `r.handleOIDCLogin` | public | `router_routes_auth_security.go` |
| ANY | `config.DefaultOIDCCallbackPath` (`internal/api/router.go:639`) | `r.handleOIDCCallback` | public | `router_routes_auth_security.go` |
| ANY | `/api/security/sso/providers/test` (`internal/api/router.go:640`) | `inline func -> r.handleTestSSOProvider` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| ANY | `/api/security/sso/providers/metadata/preview` (`internal/api/router.go:646`) | `inline func -> r.handleMetadataPreview` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| GET/POST | `/api/security/sso/providers` (`internal/api/router.go:652`) | `inline func -> r.handleSSOProviders` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| GET/PUT/DELETE | `/api/security/sso/providers/` (`internal/api/router.go:668`) | `inline func -> r.handleSSOProvider` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| GET/POST | `/api/security/tokens` (`internal/api/router.go:684`) | `inline func -> r.handleCreateAPIToken, r.handleListAPITokens` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| ANY | `/api/security/tokens/` (`internal/api/router.go:700`) | `inline func -> r.handleDeleteAPIToken` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers) | `router_routes_auth_security.go` |
| GET | `/api/security/status` (`internal/api/router.go:706`) | `inline func` | public | `router_routes_auth_security.go` |
| ANY | `/api/security/quick-setup` (`internal/api/router.go:881`) | `handleQuickSecuritySetupFixed(r)` | public; CSRF skip | `router_routes_auth_security.go` |
| ANY | `/api/security/regenerate-token` (`internal/api/router.go:884`) | `r.HandleRegenerateAPIToken` | none | `router_routes_auth_security.go` |
| ANY | `/api/security/validate-token` (`internal/api/router.go:887`) | `r.HandleValidateAPIToken` | none | `router_routes_auth_security.go` |
| POST | `/api/security/apply-restart` (`internal/api/router.go:891`) | `inline func` | CSRF skip | `router_routes_auth_security.go` |
| GET/POST | `/api/security/recovery` (`internal/api/router.go:983`) | `inline func` | none | `router_routes_auth_security.go` |
| ANY | `/api/agent/ws` (`internal/api/router.go:1822`) | `r.handleAgentWebSocket` | public | `router_routes_auth_security.go` |
| ANY | `/install-docker-agent.sh` (`internal/api/router.go:1825`) | `r.handleDownloadInstallScript` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/install-container-agent.sh` (`internal/api/router.go:1826`) | `r.handleDownloadContainerAgentInstallScript` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/download/pulse-docker-agent` (`internal/api/router.go:1827`) | `r.handleDownloadAgent` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/install-host-agent.sh` (`internal/api/router.go:1830`) | `r.handleDownloadHostAgentInstallScript` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/install-host-agent.ps1` (`internal/api/router.go:1831`) | `r.handleDownloadHostAgentInstallScriptPS` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/download/pulse-host-agent` (`internal/api/router.go:1834`) | `r.handleDownloadHostAgent` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/download/pulse-host-agent.sha256` (`internal/api/router.go:1835`) | `r.handleDownloadHostAgent` | r.downloadLimiter.Middleware | `router_routes_auth_security.go` |
| ANY | `/install.sh` (`internal/api/router.go:1838`) | `r.handleDownloadUnifiedInstallScript` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/install.ps1` (`internal/api/router.go:1839`) | `r.handleDownloadUnifiedInstallScriptPS` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/download/pulse-agent` (`internal/api/router.go:1840`) | `r.handleDownloadUnifiedAgent` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/api/agent/version` (`internal/api/router.go:1842`) | `r.handleAgentVersion` | public | `router_routes_auth_security.go` |
| ANY | `/api/server/info` (`internal/api/router.go:1843`) | `r.handleServerInfo` | public | `router_routes_auth_security.go` |
| ANY | `/ws` (`internal/api/router.go:1846`) | `r.handleWebSocket` | none | `router_routes_auth_security.go` |
| ANY | `/socket.io/` (`internal/api/router.go:1849`) | `r.handleSocketIO` | none | `router_routes_auth_security.go` |

### monitoring/resources/charts/backups/alerts

| Method | Path Pattern | Handler Function | Auth Wrapper(s) / Public/CSRF | Target Extraction Module |
| --- | --- | --- | --- | --- |
| ANY | `/api/monitoring/scheduler/health` (`internal/api/router.go:278`) | `r.handleSchedulerHealth` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/api/state` (`internal/api/router.go:279`) | `r.handleState` | none | `router_routes_monitoring.go` |
| ANY | `/api/storage/` (`internal/api/router.go:338`) | `r.handleStorage` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/storage-charts` (`internal/api/router.go:339`) | `r.handleStorageCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/charts` (`internal/api/router.go:340`) | `r.handleCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/charts/workloads` (`internal/api/router.go:341`) | `r.handleWorkloadCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/charts/infrastructure` (`internal/api/router.go:342`) | `r.handleInfrastructureCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/charts/infrastructure-summary` (`internal/api/router.go:343`) | `r.handleInfrastructureSummaryCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/charts/workloads-summary` (`internal/api/router.go:344`) | `r.handleWorkloadsSummaryCharts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/metrics-store/stats` (`internal/api/router.go:345`) | `r.handleMetricsStoreStats` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/metrics-store/history` (`internal/api/router.go:346`) | `r.handleMetricsHistory` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/backups` (`internal/api/router.go:353`) | `r.handleBackups` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/backups/` (`internal/api/router.go:354`) | `r.handleBackups` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/backups/unified` (`internal/api/router.go:355`) | `r.handleBackups` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/backups/pve` (`internal/api/router.go:356`) | `r.handleBackupsPVE` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/backups/pbs` (`internal/api/router.go:357`) | `r.handleBackupsPBS` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/snapshots` (`internal/api/router.go:358`) | `r.handleSnapshots` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/resources` (`internal/api/router.go:361`) | `r.resourceHandlers.HandleGetResources` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/resources/stats` (`internal/api/router.go:362`) | `r.resourceHandlers.HandleGetResourceStats` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/resources/` (`internal/api/router.go:363`) | `r.resourceHandlers.HandleGetResource` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/v2/resources` (`internal/api/router.go:366`) | `r.resourceV2Handlers.HandleListResources` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/v2/resources/stats` (`internal/api/router.go:367`) | `r.resourceV2Handlers.HandleStats` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/v2/resources/` (`internal/api/router.go:368`) | `r.resourceV2Handlers.HandleResourceRoutes` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/guests/metadata` (`internal/api/router.go:371`) | `guestMetadataHandler.HandleGetMetadata` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/guests/metadata/` (`internal/api/router.go:372`) | `inline func -> guestMetadataHandler.HandleDeleteMetadata, guestMetadataHandler.HandleGetMetadata, guestMetadataHandler.HandleUpdateMetadata` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/api/docker/metadata` (`internal/api/router.go:395`) | `dockerMetadataHandler.HandleGetMetadata` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/docker/metadata/` (`internal/api/router.go:396`) | `inline func -> dockerMetadataHandler.HandleDeleteMetadata, dockerMetadataHandler.HandleGetMetadata, dockerMetadataHandler.HandleUpdateMetadata` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/api/docker/hosts/metadata` (`internal/api/router.go:419`) | `dockerMetadataHandler.HandleGetHostMetadata` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/docker/hosts/metadata/` (`internal/api/router.go:420`) | `inline func -> dockerMetadataHandler.HandleDeleteHostMetadata, dockerMetadataHandler.HandleGetHostMetadata, dockerMetadataHandler.HandleUpdateHostMetadata` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/api/hosts/metadata` (`internal/api/router.go:443`) | `hostMetadataHandler.HandleGetMetadata` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/hosts/metadata/` (`internal/api/router.go:444`) | `inline func -> hostMetadataHandler.HandleDeleteMetadata, hostMetadataHandler.HandleGetMetadata, hostMetadataHandler.HandleUpdateMetadata` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/api/infra-updates` (`internal/api/router.go:477`) | `infraUpdateHandlers.HandleGetInfraUpdates` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/infra-updates/summary` (`internal/api/router.go:478`) | `infraUpdateHandlers.HandleGetInfraUpdatesSummary` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/infra-updates/check` (`internal/api/router.go:479`) | `infraUpdateHandlers.HandleTriggerInfraUpdateCheck` | RequireAuth; RequireScope(config.ScopeMonitoringWrite) | `router_routes_monitoring.go` |
| ANY | `/api/infra-updates/host/` (`internal/api/router.go:480`) | `inline func -> infraUpdateHandlers.HandleGetInfraUpdatesForHost` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/infra-updates/` (`internal/api/router.go:490`) | `inline func -> infraUpdateHandlers.HandleGetInfraUpdateForResource` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/alerts/` (`internal/api/router.go:1371`) | `r.alertHandlers.HandleAlerts` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/notifications/` (`internal/api/router.go:1374`) | `r.notificationHandlers.HandleNotifications` | RequireAdmin | `router_routes_monitoring.go` |
| GET | `/api/notifications/dlq` (`internal/api/router.go:1380`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_monitoring.go` |
| GET | `/api/notifications/queue/stats` (`internal/api/router.go:1387`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_monitoring.go` |
| POST | `/api/notifications/dlq/retry` (`internal/api/router.go:1394`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_monitoring.go` |
| POST/DELETE | `/api/notifications/dlq/delete` (`internal/api/router.go:1401`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_monitoring.go` |
| ANY | `/api/discovery` (`internal/api/router.go:1727`) | `r.discoveryHandlers.HandleListDiscoveries` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/discovery/status` (`internal/api/router.go:1728`) | `r.discoveryHandlers.HandleGetStatus` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/discovery/settings` (`internal/api/router.go:1729`) | `r.discoveryHandlers.HandleUpdateSettings` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_monitoring.go` |
| ANY | `/api/discovery/info/` (`internal/api/router.go:1730`) | `r.discoveryHandlers.HandleGetInfo` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| ANY | `/api/discovery/type/` (`internal/api/router.go:1731`) | `r.discoveryHandlers.HandleListByType` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/discovery/host/` (`internal/api/router.go:1732`) | `inline func -> r.discoveryHandlers.HandleDeleteDiscovery, r.discoveryHandlers.HandleGetDiscovery, r.discoveryHandlers.HandleGetProgress, r.discoveryHandlers.HandleListByHost, r.discoveryHandlers.HandleTriggerDiscovery, r.discoveryHandlers.HandleUpdateNotes` | RequireAuth | `router_routes_monitoring.go` |
| GET/POST/PUT/DELETE | `/api/discovery/` (`internal/api/router.go:1785`) | `inline func -> r.discoveryHandlers.HandleDeleteDiscovery, r.discoveryHandlers.HandleGetDiscovery, r.discoveryHandlers.HandleGetProgress, r.discoveryHandlers.HandleTriggerDiscovery, r.discoveryHandlers.HandleUpdateNotes` | RequireAuth | `router_routes_monitoring.go` |
| ANY | `/simple-stats` (`internal/api/router.go:1852`) | `r.handleSimpleStats` | RequireAuth | `router_routes_monitoring.go` |

### config/system/settings

| Method | Path Pattern | Handler Function | Auth Wrapper(s) / Public/CSRF | Target Extraction Module |
| --- | --- | --- | --- | --- |
| ANY | `/api/logs/stream` (`internal/api/router.go:282`) | `r.logHandlers.HandleStreamLogs` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/logs/download` (`internal/api/router.go:283`) | `r.logHandlers.HandleDownloadBundle` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| GET/POST | `/api/logs/level` (`internal/api/router.go:284`) | `inline func` | none | `router_routes_auth_security.go` |
| ANY | `/api/agents/docker/report` (`internal/api/router.go:294`) | `r.dockerAgentHandlers.HandleReport` | RequireAuth; RequireScope(config.ScopeDockerReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/kubernetes/report` (`internal/api/router.go:295`) | `r.kubernetesAgentHandlers.HandleReport` | RequireAuth; RequireScope(config.ScopeKubernetesReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/host/report` (`internal/api/router.go:296`) | `r.hostAgentHandlers.HandleReport` | RequireAuth; RequireScope(config.ScopeHostReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/host/lookup` (`internal/api/router.go:297`) | `r.hostAgentHandlers.HandleLookup` | RequireAuth; RequireScope(config.ScopeHostReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/host/uninstall` (`internal/api/router.go:298`) | `r.hostAgentHandlers.HandleUninstall` | RequireAuth; RequireScope(config.ScopeHostReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/host/unlink` (`internal/api/router.go:300`) | `r.hostAgentHandlers.HandleUnlink` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/agents/host/link` (`internal/api/router.go:301`) | `r.hostAgentHandlers.HandleLink` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| PATCH/DELETE | `/api/agents/host/` (`internal/api/router.go:303`) | `inline func -> r.hostAgentHandlers.HandleConfig, r.hostAgentHandlers.HandleDeleteHost` | RequireAuth | `router_routes_auth_security.go` |
| ANY | `/api/agents/docker/commands/` (`internal/api/router.go:333`) | `r.dockerAgentHandlers.HandleCommandAck` | RequireAuth; RequireScope(config.ScopeDockerReport) | `router_routes_auth_security.go` |
| ANY | `/api/agents/docker/hosts/` (`internal/api/router.go:334`) | `r.dockerAgentHandlers.HandleDockerHostActions` | RequireAdmin; RequireScope(config.ScopeDockerManage) | `router_routes_auth_security.go` |
| ANY | `/api/agents/docker/containers/update` (`internal/api/router.go:335`) | `r.dockerAgentHandlers.HandleContainerUpdate` | RequireAdmin; RequireScope(config.ScopeDockerManage) | `router_routes_auth_security.go` |
| ANY | `/api/agents/kubernetes/clusters/` (`internal/api/router.go:336`) | `r.kubernetesAgentHandlers.HandleClusterActions` | RequireAdmin; RequireScope(config.ScopeKubernetesManage) | `router_routes_auth_security.go` |
| ANY | `/api/diagnostics` (`internal/api/router.go:347`) | `r.handleDiagnostics` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/diagnostics/docker/prepare-token` (`internal/api/router.go:348`) | `r.handleDiagnosticsDockerPrepareToken` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/config` (`internal/api/router.go:352`) | `r.handleConfig` | RequireAuth; RequireScope(config.ScopeMonitoringRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/check` (`internal/api/router.go:467`) | `updateHandlers.HandleCheckUpdates` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/apply` (`internal/api/router.go:468`) | `updateHandlers.HandleApplyUpdate` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/updates/status` (`internal/api/router.go:469`) | `updateHandlers.HandleUpdateStatus` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/stream` (`internal/api/router.go:470`) | `updateHandlers.HandleUpdateStream` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/plan` (`internal/api/router.go:471`) | `updateHandlers.HandleGetUpdatePlan` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/history` (`internal/api/router.go:472`) | `updateHandlers.HandleListUpdateHistory` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/updates/history/entry` (`internal/api/router.go:473`) | `updateHandlers.HandleGetUpdateHistoryEntry` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| GET/POST | `/api/config/nodes` (`internal/api/router.go:503`) | `inline func` | none | `router_routes_auth_security.go` |
| POST | `/api/config/nodes/test-config` (`internal/api/router.go:516`) | `inline func` | none | `router_routes_auth_security.go` |
| POST | `/api/config/nodes/test-connection` (`internal/api/router.go:525`) | `inline func` | none | `router_routes_auth_security.go` |
| POST/PUT/DELETE | `/api/config/nodes/` (`internal/api/router.go:532`) | `inline func` | none | `router_routes_auth_security.go` |
| ANY | `/api/admin/profiles/` (`internal/api/router.go:555`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsWrite); RequireLicenseFeature(license.FeatureAgentProfiles) | `router_routes_auth_security.go` |
| GET | `/api/config/system` (`internal/api/router.go:560`) | `inline func` | none | `router_routes_auth_security.go` |
| GET/POST/PUT | `/api/system/mock-mode` (`internal/api/router.go:574`) | `inline func` | none | `router_routes_auth_security.go` |
| POST | `/api/config/export` (`internal/api/router.go:1102`) | `inline func -> r.configHandlers.HandleExportConfig` | r.exportLimiter.Middleware | `router_routes_auth_security.go` |
| POST | `/api/config/import` (`internal/api/router.go:1216`) | `inline func -> r.configHandlers.HandleImportConfig` | r.exportLimiter.Middleware | `router_routes_auth_security.go` |
| ANY | `/api/setup-script` (`internal/api/router.go:1332`) | `r.configHandlers.HandleSetupScript` | public | `router_routes_auth_security.go` |
| ANY | `/api/setup-script-url` (`internal/api/router.go:1335`) | `r.configHandlers.HandleSetupScriptURL` | RequireAdmin; RequireScope(config.ScopeSettingsWrite); CSRF skip | `router_routes_auth_security.go` |
| ANY | `/api/agent-install-command` (`internal/api/router.go:1338`) | `r.configHandlers.HandleAgentInstallCommand` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/auto-register` (`internal/api/router.go:1341`) | `r.configHandlers.HandleAutoRegister` | public | `router_routes_auth_security.go` |
| ANY | `/api/discover` (`internal/api/router.go:1343`) | `r.configHandlers.HandleDiscoverServers` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| POST | `/api/test-notification` (`internal/api/router.go:1347`) | `inline func` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/system/settings` (`internal/api/router.go:1411`) | `r.systemSettingsHandler.HandleGetSystemSettings` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_auth_security.go` |
| ANY | `/api/system/settings/update` (`internal/api/router.go:1412`) | `r.systemSettingsHandler.HandleUpdateSystemSettings` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_auth_security.go` |
| ANY | `/api/system/ssh-config` (`internal/api/router.go:1413`) | `r.handleSSHConfig` | public with valid setup token | `router_routes_auth_security.go` |
| ANY | `/api/system/verify-temperature-ssh` (`internal/api/router.go:1414`) | `r.handleVerifyTemperatureSSH` | public with valid setup token | `router_routes_auth_security.go` |
| ANY | `/uninstall-host-agent.sh` (`internal/api/router.go:1832`) | `r.handleDownloadHostAgentUninstallScript` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |
| ANY | `/uninstall-host-agent.ps1` (`internal/api/router.go:1833`) | `r.handleDownloadHostAgentUninstallScriptPS` | r.downloadLimiter.Middleware; public | `router_routes_auth_security.go` |

### AI/relay/patrol/chat/sessions

| Method | Path Pattern | Handler Function | Auth Wrapper(s) / Public/CSRF | Target Extraction Module |
| --- | --- | --- | --- | --- |
| ANY | `/api/settings/ai` (`internal/api/router.go:1526`) | `r.aiSettingsHandler.HandleGetAISettings` | RequirePermission(auth.ActionRead,auth.ResourceSettings); RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| ANY | `/api/settings/ai/update` (`internal/api/router.go:1527`) | `r.aiSettingsHandler.HandleUpdateAISettings` | RequirePermission(auth.ActionWrite,auth.ResourceSettings); RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/test` (`internal/api/router.go:1528`) | `r.aiSettingsHandler.HandleTestAIConnection` | RequirePermission(auth.ActionWrite,auth.ResourceSettings); RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/test/{provider}` (`internal/api/router.go:1529`) | `r.aiSettingsHandler.HandleTestProvider` | RequirePermission(auth.ActionWrite,auth.ResourceSettings); RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/models` (`internal/api/router.go:1531`) | `r.aiSettingsHandler.HandleListModels` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/execute` (`internal/api/router.go:1532`) | `r.aiSettingsHandler.HandleExecute` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/execute/stream` (`internal/api/router.go:1533`) | `r.aiSettingsHandler.HandleExecuteStream` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/kubernetes/analyze` (`internal/api/router.go:1534`) | `r.aiSettingsHandler.HandleAnalyzeKubernetesCluster` | RequireAdmin; RequireScope(config.ScopeAIExecute); RequireLicenseFeature(license.FeatureKubernetesAI) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/investigate-alert` (`internal/api/router.go:1535`) | `r.aiSettingsHandler.HandleInvestigateAlert` | RequireAdmin; RequireScope(config.ScopeAIExecute); RequireLicenseFeature(license.FeatureAIAlerts) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/run-command` (`internal/api/router.go:1537`) | `r.aiSettingsHandler.HandleRunCommand` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge` (`internal/api/router.go:1539`) | `r.aiSettingsHandler.HandleGetGuestKnowledge` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge/save` (`internal/api/router.go:1540`) | `r.aiSettingsHandler.HandleSaveGuestNote` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge/delete` (`internal/api/router.go:1541`) | `r.aiSettingsHandler.HandleDeleteGuestNote` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge/export` (`internal/api/router.go:1542`) | `r.aiSettingsHandler.HandleExportGuestKnowledge` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge/import` (`internal/api/router.go:1543`) | `r.aiSettingsHandler.HandleImportGuestKnowledge` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/knowledge/clear` (`internal/api/router.go:1544`) | `r.aiSettingsHandler.HandleClearGuestKnowledge` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/debug/context` (`internal/api/router.go:1546`) | `r.aiSettingsHandler.HandleDebugContext` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/agents` (`internal/api/router.go:1548`) | `r.aiSettingsHandler.HandleGetConnectedAgents` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/cost/summary` (`internal/api/router.go:1550`) | `r.aiSettingsHandler.HandleGetAICostSummary` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/cost/reset` (`internal/api/router.go:1551`) | `r.aiSettingsHandler.HandleResetAICostHistory` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/cost/export` (`internal/api/router.go:1552`) | `r.aiSettingsHandler.HandleExportAICostHistory` | RequireAdmin; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/oauth/start` (`internal/api/router.go:1555`) | `r.aiSettingsHandler.HandleOAuthStart` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/oauth/exchange` (`internal/api/router.go:1556`) | `r.aiSettingsHandler.HandleOAuthExchange` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/oauth/callback` (`internal/api/router.go:1557`) | `r.aiSettingsHandler.HandleOAuthCallback` | public | `router_routes_ai_relay.go` |
| ANY | `/api/ai/oauth/disconnect` (`internal/api/router.go:1558`) | `r.aiSettingsHandler.HandleOAuthDisconnect` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| GET | `/api/settings/relay` (`internal/api/router.go:1561`) | `r.handleGetRelayConfig` | RequireAdmin; RequireScope(config.ScopeSettingsRead); RequireLicenseFeature(license.FeatureRelay) | `router_routes_ai_relay.go` |
| PUT | `/api/settings/relay` (`internal/api/router.go:1562`) | `r.handleUpdateRelayConfig` | RequireAdmin; RequireScope(config.ScopeSettingsWrite); RequireLicenseFeature(license.FeatureRelay) | `router_routes_ai_relay.go` |
| GET | `/api/settings/relay/status` (`internal/api/router.go:1563`) | `r.handleGetRelayStatus` | RequireAdmin; RequireScope(config.ScopeSettingsRead); RequireLicenseFeature(license.FeatureRelay) | `router_routes_ai_relay.go` |
| GET | `/api/onboarding/qr` (`internal/api/router.go:1564`) | `r.handleGetOnboardingQR` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| POST | `/api/onboarding/validate` (`internal/api/router.go:1565`) | `r.handleValidateOnboardingConnection` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| GET | `/api/onboarding/deep-link` (`internal/api/router.go:1566`) | `r.handleGetOnboardingDeepLink` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/status` (`internal/api/router.go:1573`) | `r.aiSettingsHandler.HandleGetPatrolStatus` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/stream` (`internal/api/router.go:1574`) | `r.aiSettingsHandler.HandlePatrolStream` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| GET/DELETE | `/api/ai/patrol/findings` (`internal/api/router.go:1575`) | `inline func -> r.aiSettingsHandler.HandleClearAllFindings, r.aiSettingsHandler.HandleGetPatrolFindings` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/history` (`internal/api/router.go:1587`) | `r.aiSettingsHandler.HandleGetFindingsHistory` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/run` (`internal/api/router.go:1588`) | `r.aiSettingsHandler.HandleForcePatrol` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/acknowledge` (`internal/api/router.go:1591`) | `r.aiSettingsHandler.HandleAcknowledgeFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/dismiss` (`internal/api/router.go:1594`) | `r.aiSettingsHandler.HandleDismissFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/findings/note` (`internal/api/router.go:1595`) | `r.aiSettingsHandler.HandleSetFindingNote` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/suppress` (`internal/api/router.go:1596`) | `r.aiSettingsHandler.HandleSuppressFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/snooze` (`internal/api/router.go:1597`) | `r.aiSettingsHandler.HandleSnoozeFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/resolve` (`internal/api/router.go:1598`) | `r.aiSettingsHandler.HandleResolveFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/runs` (`internal/api/router.go:1599`) | `r.aiSettingsHandler.HandleGetPatrolRunHistory` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| GET/POST | `/api/ai/patrol/suppressions` (`internal/api/router.go:1601`) | `inline func -> r.aiSettingsHandler.HandleAddSuppressionRule, r.aiSettingsHandler.HandleGetSuppressionRules` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/suppressions/` (`internal/api/router.go:1611`) | `r.aiSettingsHandler.HandleDeleteSuppressionRule` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/patrol/dismissed` (`internal/api/router.go:1612`) | `r.aiSettingsHandler.HandleGetDismissedFindings` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| GET/PUT | `/api/ai/patrol/autonomy` (`internal/api/router.go:1615`) | `inline func -> r.aiSettingsHandler.HandleGetPatrolAutonomy, r.aiSettingsHandler.HandleUpdatePatrolAutonomy` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/findings/` (`internal/api/router.go:1628`) | `inline func -> r.aiSettingsHandler.HandleGetInvestigation, r.aiSettingsHandler.HandleGetInvestigationMessages, r.aiSettingsHandler.HandleReinvestigateFinding` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence` (`internal/api/router.go:1648`) | `r.aiSettingsHandler.HandleGetIntelligence` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/patterns` (`internal/api/router.go:1650`) | `r.aiSettingsHandler.HandleGetPatterns` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/predictions` (`internal/api/router.go:1651`) | `r.aiSettingsHandler.HandleGetPredictions` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/correlations` (`internal/api/router.go:1652`) | `r.aiSettingsHandler.HandleGetCorrelations` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/changes` (`internal/api/router.go:1653`) | `r.aiSettingsHandler.HandleGetRecentChanges` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/baselines` (`internal/api/router.go:1654`) | `r.aiSettingsHandler.HandleGetBaselines` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/remediations` (`internal/api/router.go:1655`) | `r.aiSettingsHandler.HandleGetRemediations` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/anomalies` (`internal/api/router.go:1656`) | `r.aiSettingsHandler.HandleGetAnomalies` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/intelligence/learning` (`internal/api/router.go:1657`) | `r.aiSettingsHandler.HandleGetLearningStatus` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/unified/findings` (`internal/api/router.go:1659`) | `r.aiSettingsHandler.HandleGetUnifiedFindings` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/forecast` (`internal/api/router.go:1662`) | `r.aiSettingsHandler.HandleGetForecast` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/forecasts/overview` (`internal/api/router.go:1663`) | `r.aiSettingsHandler.HandleGetForecastOverview` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/learning/preferences` (`internal/api/router.go:1664`) | `r.aiSettingsHandler.HandleGetLearningPreferences` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/proxmox/events` (`internal/api/router.go:1665`) | `r.aiSettingsHandler.HandleGetProxmoxEvents` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/proxmox/correlations` (`internal/api/router.go:1666`) | `r.aiSettingsHandler.HandleGetProxmoxCorrelations` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| GET | `/api/ai/remediation/plans` (`internal/api/router.go:1668`) | `inline func -> r.aiSettingsHandler.HandleGetRemediationPlans` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/remediation/plan` (`internal/api/router.go:1676`) | `r.aiSettingsHandler.HandleGetRemediationPlan` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/remediation/approve` (`internal/api/router.go:1678`) | `r.aiSettingsHandler.HandleApproveRemediationPlan` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/remediation/execute` (`internal/api/router.go:1679`) | `r.aiSettingsHandler.HandleExecuteRemediationPlan` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/remediation/rollback` (`internal/api/router.go:1680`) | `r.aiSettingsHandler.HandleRollbackRemediationPlan` | RequireAdmin; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/circuit/status` (`internal/api/router.go:1682`) | `r.aiSettingsHandler.HandleGetCircuitBreakerStatus` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/incidents` (`internal/api/router.go:1685`) | `r.aiSettingsHandler.HandleGetRecentIncidents` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/incidents/` (`internal/api/router.go:1686`) | `r.aiSettingsHandler.HandleGetIncidentData` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/chat/sessions` (`internal/api/router.go:1689`) | `r.aiSettingsHandler.HandleListAIChatSessions` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| GET/PUT/DELETE | `/api/ai/chat/sessions/` (`internal/api/router.go:1690`) | `inline func -> r.aiSettingsHandler.HandleDeleteAIChatSession, r.aiSettingsHandler.HandleGetAIChatSession, r.aiSettingsHandler.HandleSaveAIChatSession` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/status` (`internal/api/router.go:1704`) | `r.aiHandler.HandleStatus` | RequireAuth | `router_routes_ai_relay.go` |
| ANY | `/api/ai/chat` (`internal/api/router.go:1705`) | `r.aiHandler.HandleChat` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| GET/POST | `/api/ai/sessions` (`internal/api/router.go:1706`) | `inline func -> r.aiHandler.HandleCreateSession, r.aiHandler.HandleSessions` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/sessions/` (`internal/api/router.go:1716`) | `r.routeAISessions` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/approvals` (`internal/api/router.go:1720`) | `r.aiSettingsHandler.HandleListApprovals` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/approvals/` (`internal/api/router.go:1721`) | `r.routeApprovals` | RequireAuth; RequireScope(config.ScopeAIExecute) | `router_routes_ai_relay.go` |
| ANY | `/api/ai/question/` (`internal/api/router.go:1724`) | `r.routeQuestions` | RequireAuth; RequireScope(config.ScopeAIChat) | `router_routes_ai_relay.go` |

### org/license/audit/reporting

| Method | Path Pattern | Handler Function | Auth Wrapper(s) / Public/CSRF | Target Extraction Module |
| --- | --- | --- | --- | --- |
| ANY | `/api/license/status` (`internal/api/router.go:588`) | `r.licenseHandlers.HandleLicenseStatus` | RequireAdmin | `router_routes_org_license.go` |
| ANY | `/api/license/features` (`internal/api/router.go:589`) | `r.licenseHandlers.HandleLicenseFeatures` | RequireAuth | `router_routes_org_license.go` |
| ANY | `/api/license/activate` (`internal/api/router.go:590`) | `r.licenseHandlers.HandleActivateLicense` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| ANY | `/api/license/clear` (`internal/api/router.go:591`) | `r.licenseHandlers.HandleClearLicense` | RequireAdmin; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| GET | `/api/orgs` (`internal/api/router.go:594`) | `orgHandlers.HandleListOrgs` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| POST | `/api/orgs` (`internal/api/router.go:595`) | `orgHandlers.HandleCreateOrg` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| GET | `/api/orgs/{id}` (`internal/api/router.go:596`) | `orgHandlers.HandleGetOrg` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| PUT | `/api/orgs/{id}` (`internal/api/router.go:597`) | `orgHandlers.HandleUpdateOrg` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| DELETE | `/api/orgs/{id}` (`internal/api/router.go:598`) | `orgHandlers.HandleDeleteOrg` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| GET | `/api/orgs/{id}/members` (`internal/api/router.go:599`) | `orgHandlers.HandleListMembers` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| POST | `/api/orgs/{id}/members` (`internal/api/router.go:600`) | `orgHandlers.HandleInviteMember` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| DELETE | `/api/orgs/{id}/members/{userId}` (`internal/api/router.go:601`) | `orgHandlers.HandleRemoveMember` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| GET | `/api/orgs/{id}/shares` (`internal/api/router.go:602`) | `orgHandlers.HandleListShares` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| GET | `/api/orgs/{id}/shares/incoming` (`internal/api/router.go:603`) | `orgHandlers.HandleListIncomingShares` | RequireAuth; RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| POST | `/api/orgs/{id}/shares` (`internal/api/router.go:604`) | `orgHandlers.HandleCreateShare` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| DELETE | `/api/orgs/{id}/shares/{shareId}` (`internal/api/router.go:605`) | `orgHandlers.HandleDeleteShare` | RequireAuth; RequireScope(config.ScopeSettingsWrite) | `router_routes_org_license.go` |
| GET | `/api/audit` (`internal/api/router.go:609`) | `auditHandlers.HandleListAuditEvents` | RequirePermission(auth.ActionRead,auth.ResourceAuditLogs); RequireLicenseFeature(license.FeatureAuditLogging); RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| GET | `/api/audit/` (`internal/api/router.go:610`) | `auditHandlers.HandleListAuditEvents` | RequirePermission(auth.ActionRead,auth.ResourceAuditLogs); RequireLicenseFeature(license.FeatureAuditLogging); RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| GET | `/api/audit/{id}/verify` (`internal/api/router.go:611`) | `auditHandlers.HandleVerifyAuditEvent` | RequirePermission(auth.ActionRead,auth.ResourceAuditLogs); RequireLicenseFeature(license.FeatureAuditLogging); RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| ANY | `/api/admin/roles` (`internal/api/router.go:614`) | `rbacHandlers.HandleRoles` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers); RequireLicenseFeature(license.FeatureRBAC) | `router_routes_org_license.go` |
| ANY | `/api/admin/roles/` (`internal/api/router.go:615`) | `rbacHandlers.HandleRoles` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers); RequireLicenseFeature(license.FeatureRBAC) | `router_routes_org_license.go` |
| ANY | `/api/admin/users` (`internal/api/router.go:616`) | `rbacHandlers.HandleGetUsers` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers); RequireLicenseFeature(license.FeatureRBAC) | `router_routes_org_license.go` |
| ANY | `/api/admin/users/` (`internal/api/router.go:617`) | `rbacHandlers.HandleUserRoleActions` | RequirePermission(auth.ActionAdmin,auth.ResourceUsers); RequireLicenseFeature(license.FeatureRBAC) | `router_routes_org_license.go` |
| ANY | `/api/admin/reports/generate` (`internal/api/router.go:620`) | `r.reportingHandlers.HandleGenerateReport` | RequirePermission(auth.ActionRead,auth.ResourceNodes); RequireLicenseFeature(license.FeatureAdvancedReporting); RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| ANY | `/api/admin/reports/generate-multi` (`internal/api/router.go:621`) | `r.reportingHandlers.HandleGenerateMultiReport` | RequirePermission(auth.ActionRead,auth.ResourceNodes); RequireLicenseFeature(license.FeatureAdvancedReporting); RequireScope(config.ScopeSettingsRead) | `router_routes_org_license.go` |
| GET | `/api/admin/webhooks/audit` (`internal/api/router.go:624`) | `inline func` | RequirePermission(auth.ActionAdmin,auth.ResourceAuditLogs); RequireLicenseFeature(license.FeatureAuditLogging) | `router_routes_org_license.go` |

## Appendix D: ConfigHandlers Domain Inventory

### node lifecycle (CRUD, test, refresh)

| Method | Line Anchor | Key Side Effects | Target Extraction Module |
| --- | --- | --- | --- |
| `HandleGetNodes` | `internal/api/config_handlers.go:1133` | Read path only; aggregates node state from config/monitor/persistence; no persistence writes. | `config_node_handlers.go` |
| `HandleAddNode` | `internal/api/config_handlers.go:1474` | Persists node lists via `SaveNodesConfig`; reloads monitor via `h.reloadFunc`. | `config_node_handlers.go` |
| `HandleTestConnection` | `internal/api/config_handlers.go:1976` | No persistence writes; performs outbound connectivity checks to PVE/PBS/PMG endpoints. | `config_node_handlers.go` |
| `HandleUpdateNode` | `internal/api/config_handlers.go:2275` | Persists via `SaveNodesConfig`; preserves alert overrides (`LoadAlertConfig` + `UpdateConfig`); reloads monitor; broadcasts discovery update via `wsHub`. | `config_node_handlers.go` |
| `HandleDeleteNode` | `internal/api/config_handlers.go:2651` | Persists via `SaveNodesConfigAllowEmpty`; reloads monitor; broadcasts `node_deleted`; triggers async discovery refresh. | `config_node_handlers.go` |
| `HandleRefreshClusterNodes` | `internal/api/config_handlers.go:2774` | Persists refreshed endpoints via `SaveNodesConfig`; best-effort monitor reload; broadcasts `nodes_updated`. | `config_node_handlers.go` |
| `HandleTestNodeConfig` | `internal/api/config_handlers.go:2890` | No persistence writes; validates/tests proposed node configuration against remote APIs. | `config_node_handlers.go` |
| `HandleTestNode` | `internal/api/config_handlers.go:3042` | No persistence writes; tests existing configured node connectivity. | `config_node_handlers.go` |

### setup/auto-register

| Method | Line Anchor | Key Side Effects | Target Extraction Module |
| --- | --- | --- | --- |
| `HandleSetupScript` | `internal/api/config_handlers.go:3711` | Generates setup script payload; may generate/load SSH keys via `getOrGenerateSSHKeys` (filesystem writes in `~/.ssh`). | `config_setup_handlers.go` |
| `HandleSetupScriptURL` | `internal/api/config_handlers.go:4790` | Creates one-time setup token and writes to in-memory `setupCodes` map with expiry/metadata. | `config_setup_handlers.go` |
| `HandleAutoRegister` | `internal/api/config_handlers.go:5008` | Persists node config via `SaveNodesConfig`; async monitor reload; discovery refresh; broadcasts `node_auto_registered` and `discovery_update`. | `config_setup_handlers.go` |
| `HandleAgentInstallCommand` | `internal/api/config_handlers.go:5957` | Appends/sorts in-memory API token records; persists via `SaveAPITokens`; rolls back in-memory append on persistence failure. | `config_setup_handlers.go` |

### discovery

| Method | Line Anchor | Key Side Effects | Target Extraction Module |
| --- | --- | --- | --- |
| `HandleDiscoverServers` | `internal/api/config_handlers.go:3584` | Reads cached discovery state and/or performs manual discovery scans; no config persistence writes. | `config_discovery_handlers.go` |

### system settings / SSH verify

| Method | Line Anchor | Key Side Effects | Target Extraction Module |
| --- | --- | --- | --- |
| `HandleGetSystemSettings` | `internal/api/config_handlers.go:3255` | Reads persisted settings and runtime overrides; no writes. | `config_system_handlers.go` |
| `HandleVerifyTemperatureSSH` | `internal/api/config_handlers.go:3329` | Performs outbound SSH probes for temperature monitoring verification; no persistence writes. | `config_system_handlers.go` |
| `HandleGetMockMode` | `internal/api/config_handlers.go:4892` | Read-only mock mode/config response. | `config_system_handlers.go` |
| `HandleUpdateMockMode` | `internal/api/config_handlers.go:4920` | Updates runtime mock state via `mock.SetMockConfig` and `monitor.SetMockMode`/`mock.SetEnabled`; no persistence writes. | `config_system_handlers.go` |

### import/export

| Method | Line Anchor | Key Side Effects | Target Extraction Module |
| --- | --- | --- | --- |
| `HandleExportConfig` | `internal/api/config_handlers.go:3424` | Reads encrypted export payload from persistence via `ExportConfig`; no writes. | `config_export_import_handlers.go` |
| `HandleImportConfig` | `internal/api/config_handlers.go:3471` | Imports via `ImportConfig`; reloads config and monitor; reloads alert/webhook/email configs; mutates notification manager state; reloads guest metadata. | `config_export_import_handlers.go` |

## Appendix E: Extraction Cut Points

| Target Module | Source File + Line Range(s) | Dependencies Required | Initialization Ordering Constraints | Known Coupling Points |
| --- | --- | --- | --- | --- |
| `router_routes_auth_security.go` | `internal/api/router.go:277-303`, `internal/api/router.go:333-352`, `internal/api/router.go:503-574`, `internal/api/router.go:633-706`, `internal/api/router.go:881-983`, `internal/api/router.go:1102-1347`, `internal/api/router.go:1411-1414`, `internal/api/router.go:1822-1849` | `Router` fields: `config`, `authorizer`, `configHandlers`, `systemSettingsHandler`, `downloadLimiter`, `exportLimiter`, `wsHub`, agent handlers, install/download handlers. | Keep handler construction before registration. Preserve `systemSettingsHandler` initialization before `/api/system/settings*` registrations. | Global public path list and CSRF skip logic in `ServeHTTP` (`internal/api/router.go:4045-4192`); inline `ensureScope` checks and method-switch handlers. |
| `router_routes_monitoring.go` | `internal/api/router.go:278-279`, `internal/api/router.go:338-490`, `internal/api/router.go:1371-1401`, `internal/api/router.go:1727-1785`, `internal/api/router.go:1852` | `monitor`, `mtMonitor`, `alertHandlers`, `notificationHandlers`, `notificationQueueHandlers`, `resourceHandlers`, `resourceV2Handlers`, metadata handlers, `discoveryHandlers`. | Ensure monitoring/alert/discovery handlers are instantiated before route registration. | Path-suffix dispatch routes (`/api/discovery/host/`, `/api/discovery/`, `/api/infra-updates/`), mixed scope checks (`RequireScope` + inline `ensureScope`). |
| `router_routes_ai_relay.go` | `internal/api/router.go:1526-1724` | `aiSettingsHandler`, `aiHandler`, `licenseHandlers`, `authorizer`, helper routers (`routeAISessions`, `routeApprovals`, `routeQuestions`). | Keep AI handler wiring/callback setup complete before route registration (`SetChatHandler`, control callbacks, resource providers). | Heavy use of inline method multiplexers and license-feature gates within closures; session sub-route behavior in helper routers. |
| `router_routes_org_license.go` | `internal/api/router.go:588-624` | `licenseHandlers`, `authorizer`, `reportingHandlers`, local `orgHandlers`, `auditHandlers`, `rbacHandlers`. | Keep `SetLicenseServiceProvider(r.licenseHandlers)` before route registration so license middleware resolves correctly. | Nested gate chains (`RequirePermission` + `RequireLicenseFeature` + `RequireScope`), method-prefixed routes (`GET /api/orgs`, etc.). |
| `config_node_handlers.go` | `internal/api/config_handlers.go:1133-3221` (plus node API projection helper at `internal/api/config_handlers.go:1037-1130`) | `getConfig`, `getPersistence`, `getMonitor`, `reloadFunc`, `wsHub`, `guestMetadataHandler`, Proxmox/PBS/PMG clients and cluster detection helpers. | Preserve current write->reload->broadcast sequencing and existing goroutine timing for discovery broadcasts. | Node ID/path parsing logic shared with router suffix routes; alert override preservation (`LoadAlertConfig`/`UpdateConfig`) tightly coupled to update flow. |
| `config_setup_handlers.go` | `internal/api/config_handlers.go:3711-4773`, `internal/api/config_handlers.go:4790-4889`, `internal/api/config_handlers.go:5008-5659`, `internal/api/config_handlers.go:5957-6052` | Setup-token store (`setupCodes`, `codeMutex`), script/key helpers (`generateSetupCode`, `getOrGenerateSSHKeys`), persistence, monitor reload, websocket hub, token utilities. | Keep setup token generation and validation semantics identical; preserve async reload and broadcast behavior after auto-register. | Large embedded script templates and security checks; shared auth/setup token flow with router public-route handling for `/api/setup-script` and `/api/auto-register`. |
| `config_system_handlers.go` | `internal/api/config_handlers.go:3255-3405`, `internal/api/config_handlers.go:4892-4989` | Persistence-backed settings loading, runtime config overlay, monitor pointer for mock mode, mock package state. | Maintain runtime-state-first semantics (mock mode updates can bypass persistence). | `/api/system/*` routes are partly bare with conditional public setup-token access enforced in router middleware. |
| `config_discovery_handlers.go` | `internal/api/config_handlers.go:3584-3708` | Discovery service access via `h.getMonitor(ctx).GetDiscoveryService()`, request payload parsing helpers. | Ensure monitor/discovery service availability checks remain defensive (nil-safe) before reads/scans. | Overlaps conceptually with router-level `/api/discovery*` route family owned by `discoveryHandlers` in `router.go`. |
| `config_export_import_handlers.go` | `internal/api/config_handlers.go:3424-3581` | Persistence export/import APIs, config reload (`config.Load`), `reloadFunc`, monitor alert/notification managers, guest metadata reload. | Preserve import sequence: persist import -> reload config -> reload monitor -> reload alert/webhook/email/metadata. | Routes are bare (`/api/config/export`, `/api/config/import`) and rely on inline auth/scope checks in router handler closure plus handler-level `ensureScope`. |
