# W4 Follow-Up: RBAC Operations Hardening Plan

Status: Active
Owner: Pulse
Date: 2026-02-09

Progress tracker:
- `docs/architecture/multi-tenant-rbac-operations-progress-2026-02.md`

Predecessor:
- W4 Residual RBAC + max_users Lane: LANE_COMPLETE (RBAC-00..RBAC-05)
- W4 Multi-Tenant Productization (Packets 00-08): COMPLETE
- W0-B Monetization Foundation (MON-01..MON-09): COMPLETE

Reference:
- `docs/architecture/multi-tenant-rbac-user-limits-plan-2026-02.md` (completed RBAC isolation plan)
- `docs/architecture/multi-tenant-rbac-user-limits-progress-2026-02.md` (completed RBAC isolation progress)
- `docs/architecture/multi-tenant-operational-runbook.md` (operational runbook)
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (W4 exit criteria)
- `docs/architecture/delegation-review-rubric.md` (review gates)

## Intent

Harden the per-tenant RBAC system for production operations. The RBAC isolation lane delivered functional correctness (per-org databases, tenant-scoped handlers, max_users enforcement, isolation tests). This follow-up lane closes operational gaps: lifecycle cleanup, data integrity verification, observability, recovery paths, and load validation.

Primary outcomes:
1. Org deletion properly cleans up cached RBAC managers and audit loggers (fix lifecycle wiring gap).
2. RBAC data integrity is verifiable at runtime (schema presence, built-in roles, db health).
3. Break-glass admin recovery exists for RBAC lockout scenarios.
4. RBAC-specific Prometheus metrics provide operational visibility into cache behavior and mutation rates.
5. Load benchmarks validate RBAC manager cache behavior under many-tenant scenarios.
6. Production operations recommendation is evidence-backed.

## Residual Risks from RBAC Lane (Code-Derived)

These are gaps discovered during codebase exploration that were not addressed in the RBAC isolation lane:

| ID | Severity | Gap | File | Evidence |
|---|---|---|---|---|
| RBO-R01 | High | Org deletion doesn't call `rbacProvider.RemoveTenant()` — cached SQLiteManager stays in memory, SQLite connection open to deleted db | `org_handlers.go:283-294` | Only calls `persistence.DeleteOrganization()` and `mtMonitor.RemoveTenant()` |
| RBO-R02 | Medium | `rbacProvider` not stored on Router struct — can't access from deletion handler | `router.go:283-284` | Created locally, only passed to RBACHandlers |
| RBO-R03 | Medium | Audit logger not cleaned up on org deletion | `org_handlers.go:283-294` | `RemoveTenantLogger()` exists in `pkg/audit/tenant_logger.go:151` but never called |
| RBO-R04 | Low | No RBAC-specific Prometheus metrics — only HTTP-level observability | `internal/api/http_metrics.go` | No `pulse_rbac_*` metrics exist |
| RBO-R05 | Low | No backup/restore helpers for per-tenant RBAC data | — | No backup function exists |
| RBO-R06 | Low | No break-glass admin recovery for RBAC lockout | `recovery_tokens.go` | Recovery tokens exist but no RBAC-specific reset path |
| RBO-R07 | Low | No load validation for many-tenant RBAC cache | — | No benchmarks exist |

## Current Baseline (Code-Derived)

### RBAC Manager Cache (`internal/api/rbac_tenant_provider.go`)

- `TenantRBACProvider` caches `map[string]*auth.SQLiteManager` with `sync.RWMutex`
- Double-check locking for lazy initialization
- `RemoveTenant(orgID)` closes and removes cached manager — but never called from org deletion
- `Close()` closes all cached managers — called on Router shutdown

### Org Deletion (`internal/api/org_handlers.go:251-297`)

Current cleanup sequence:
1. `persistence.DeleteOrganization(orgID)` — removes org directory via `os.RemoveAll()`
2. `mtMonitor.RemoveTenant(orgID)` — stops monitoring

Missing:
3. `rbacProvider.RemoveTenant(orgID)` — close RBAC db, remove from cache
4. `auditLoggerManager.RemoveTenantLogger(orgID)` — close audit logger

### SQLiteManager Lifecycle (`pkg/auth/sqlite_manager.go`)

- Constructor creates tables + indexes + built-in roles on first open
- `Close()` closes db connection pool
- WAL journaling mode with 30s busy timeout
- `MaxOpenConns=1` / `MaxIdleConns=1` — single-writer model
- File-based migration from legacy JSON (`MigrateFromFiles` config option)

### Existing Observability (`internal/api/http_metrics.go`)

- `pulse_http_requests_total{method, route, status}` — all API requests
- `pulse_http_request_errors_total{method, route, status_class}` — errors by class
- `pulse_http_request_duration_seconds{method, route, status}` — latency histogram
- Route normalization strips IDs/UUIDs for cardinality control

### Recovery Infrastructure (`internal/api/recovery_tokens.go`)

- `RecoveryTokenStore` with constant-time validation and replay protection
- Hourly cleanup of expired tokens
- Persisted to `recovery_tokens.json`

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: Codex
- Reviewer: Claude (orchestrator)

A packet is `DONE` only when:
1. All packet checkboxes are complete.
2. Required commands have explicit exit codes.
3. Reviewer gate checklist passes.
4. Verdict is `APPROVED`.
5. Checkpoint commit hash is recorded.

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

1. `go build ./...`
2. `go test ./internal/api/... -run "RBAC|TenantRBAC" -count=1`

Milestone boundary baselines (RBO-04, RBO-05):

3. `go test ./internal/api/... -count=1`
4. `go test ./pkg/auth/... -count=1`

## Execution Packets

### Phase 1: Scope + Lifecycle Fixes

#### RBO-00: Scope Freeze + Residual Risk Reconciliation

Objective:
- Document all operational gaps from codebase exploration.
- Reconcile residual risks from the completed RBAC isolation lane.
- Freeze scope for the operations hardening lane.

Scope (docs only):
- This plan document (already includes risk table and gap enumeration).

Implementation checklist:
1. All residual risks from RBAC lane enumerated with file references.
2. All operational gaps documented with severity.
3. Packet-to-risk mapping defined.
4. Scope freeze: in-scope and out-of-scope items declared.

Evidence:
- This plan document exists with all required sections.

Exit criteria:
- All known operational gaps have documented mitigations mapped to packets.

Rollback:
- Delete this plan document.

#### RBO-01: Org Deletion RBAC Lifecycle Cleanup

Objective:
- Fix the critical gap where org deletion leaves RBAC managers cached in memory with open database connections.

Scope (max 4 files):
1. `internal/api/router.go` — Store `rbacProvider` on Router struct so it's accessible from deletion handlers.
2. `internal/api/org_handlers.go` — Wire `rbacProvider.RemoveTenant(orgID)` into `HandleDeleteOrg` cleanup sequence. Also wire audit logger cleanup if manager is accessible.
3. `internal/api/rbac_lifecycle_test.go` (new) — Tests: org deletion cleans RBAC cache, deleted org manager not reused, new org after deletion gets fresh db.
4. `internal/api/rbac_tenant_provider.go` — Add `ManagerCount()` method for test observability of cache size.

Implementation checklist:
1. Add `rbacProvider *TenantRBACProvider` field to Router struct.
2. Store provider reference during router initialization.
3. Pass provider to OrgHandlers (or make accessible via Router method).
4. Call `rbacProvider.RemoveTenant(orgID)` in `HandleDeleteOrg` after `persistence.DeleteOrganization()`.
5. Add `ManagerCount() int` to TenantRBACProvider for test assertions.
6. Tests: verify cache size decreases after org deletion, verify fresh manager after re-creation.

Required tests:
1. `go test ./internal/api/... -run "RBACLifecycle|RBACIsolation" -count=1` -> exit 0
2. `go build ./...` -> exit 0

Exit criteria:
- Org deletion cleans up RBAC cache.
- No stale managers remain after org deletion.

Rollback:
- Revert Router struct change, revert HandleDeleteOrg wiring, delete test file.

### Phase 2: Recovery + Observability

#### RBO-02: RBAC Data Integrity Verification + Break-Glass Recovery

Objective:
- Add RBAC data integrity verification helpers and a break-glass admin role reset function.

Scope (max 4 files):
1. `internal/api/rbac_admin_recovery.go` (new) — `VerifyRBACIntegrity(provider, orgID)` checks: db exists, schema tables present, built-in roles exist. `ResetAdminRole(provider, orgID)` re-creates admin role + assigns to specified user. `BackupRBACData(provider, orgID, destPath)` copies db file.
2. `internal/api/rbac_admin_recovery_test.go` (new) — Tests for integrity verification, admin reset, and backup.
3. `internal/api/rbac_admin_handlers.go` (new) — `HandleRBACIntegrityCheck` endpoint (GET, admin-only). `HandleRBACAdminReset` endpoint (POST, admin-only, requires recovery token).

Implementation checklist:
1. `VerifyRBACIntegrity()` returns structured result: db accessible, tables exist, built-in roles count, total roles, total assignments.
2. `ResetAdminRole()` ensures admin role exists with full permissions, assigns specified user, logs to changelog.
3. `BackupRBACData()` copies current rbac.db to timestamped backup path.
4. Integrity check endpoint returns JSON health status.
5. Admin reset endpoint requires valid recovery token (existing `RecoveryTokenStore`).
6. Tests: verify integrity check detects healthy vs missing db, verify admin reset restores access, verify backup creates file.

Required tests:
1. `go test ./internal/api/... -run "RBACAdminRecovery|RBACIntegrity" -count=1` -> exit 0
2. `go build ./...` -> exit 0

Exit criteria:
- Integrity verification detects healthy and broken RBAC state.
- Break-glass admin reset restores access.
- Backup creates usable copy of RBAC data.

Rollback:
- Delete new files.

#### RBO-03: RBAC Observability Metrics

Objective:
- Add RBAC-specific Prometheus metrics for operational visibility into tenant RBAC cache behavior and mutation rates.

Scope (max 3 files):
1. `internal/api/rbac_metrics.go` (new) — RBAC metrics registration: `pulse_rbac_managers_active` (gauge), `pulse_rbac_role_mutations_total{org_id, action}` (counter), `pulse_rbac_access_denials_total{org_id}` (counter), `pulse_rbac_integrity_checks_total{org_id, result}` (counter).
2. `internal/api/rbac_tenant_provider.go` — Add metric recording to `GetManager()` (gauge increment), `RemoveTenant()` (gauge decrement).
3. `internal/api/rbac_metrics_test.go` (new) — Tests verifying metric registration and recording.

Implementation checklist:
1. Define RBAC metrics following existing `http_metrics.go` pattern (namespace `pulse`, subsystem `rbac`).
2. `pulse_rbac_managers_active` gauge tracks current cache size.
3. `pulse_rbac_role_mutations_total` counter tracks role CRUD and assignment changes by org.
4. `pulse_rbac_access_denials_total` counter tracks RBAC-related 403s by org.
5. `pulse_rbac_integrity_checks_total` counter tracks integrity check outcomes.
6. Wire gauge updates into `GetManager()` and `RemoveTenant()`.
7. Tests: verify gauge increments/decrements match manager lifecycle.

Required tests:
1. `go test ./internal/api/... -run "RBACMetrics" -count=1` -> exit 0
2. `go build ./...` -> exit 0

Exit criteria:
- RBAC metrics are registered and recording.
- Cache size is observable via Prometheus.
- Role mutations are countable per org.

Rollback:
- Delete metrics files. Revert provider instrumentation.

### Phase 3: Validation + Certification

#### RBO-04: Load/Soak Validation for Tenant RBAC Manager Cache

Objective:
- Benchmark test validating RBAC manager cache behavior under many-tenant load.
- Verify no connection leaks, excessive memory growth, or deadlocks.

Scope (max 2 files):
1. `internal/api/rbac_tenant_provider_bench_test.go` (new) — Benchmark: create N orgs (10, 50, 100), get managers concurrently, verify all return valid managers, close all, verify zero open connections. Stress test: concurrent GetManager + RemoveTenant interleaving.
2. `internal/api/rbac_tenant_provider.go` — Add `OpenManagerCount()` for leak detection if not already present.

Implementation checklist:
1. `BenchmarkTenantRBACProvider_ManyOrgs` — measures allocation and time for 100 concurrent orgs.
2. `TestTenantRBACProvider_ConcurrentAccess` — goroutine stress test with GetManager + RemoveTenant interleaving.
3. `TestTenantRBACProvider_NoConnectionLeak` — create and close 50 managers, verify no residual db handles.
4. Verify `Close()` properly releases all resources.

Required tests:
1. `go test ./internal/api/... -run "RBACProvider.*Concurrent|RBACProvider.*Leak" -count=1` -> exit 0
2. `go test ./internal/api/... -bench "TenantRBACProvider" -benchtime=3s` -> exit 0
3. `go build ./...` -> exit 0

Exit criteria:
- 100-org benchmark completes without deadlock.
- No connection leaks detected.
- Concurrent access does not panic or corrupt state.

Rollback:
- Delete benchmark test file.

#### RBO-05: Final Operational Verdict

Objective:
- Certify operations lane completion with full validation evidence.
- Issue production operations recommendation: GO / GO_WITH_CONDITIONS / NO_GO.

Scope:
- `docs/architecture/multi-tenant-rbac-operations-progress-2026-02.md`

Implementation checklist:
1. Verify RBO-00 through RBO-04 all `DONE/APPROVED`.
2. Execute full milestone validation commands with explicit exit codes.
3. Verify org deletion lifecycle cleanup is wired.
4. Verify integrity check and admin recovery endpoints exist and test-covered.
5. Verify RBAC metrics are registered and recording.
6. Verify load benchmarks complete without leaks.
7. Issue production recommendation with conditions (if any).

Required tests:
1. `go build ./...`
2. `go test ./pkg/auth/... -count=1`
3. `go test ./internal/api/... -run "RBAC|TenantRBAC|UserLimit|RBACLifecycle|RBACAdminRecovery|RBACIntegrity|RBACMetrics" -count=1`
4. `go test ./internal/api/... -bench "TenantRBACProvider" -benchtime=3s`

Exit criteria:
- Final verdict recorded with evidence-backed production recommendation.

## Milestones

M1 complete when RBO-00 and RBO-01 are approved (scope + lifecycle fix).
M2 complete when RBO-02 and RBO-03 are approved (recovery + observability).
M3 complete when RBO-04 and RBO-05 are approved (validation + certification).

## Definition of Done

This lane is complete only when all are true:

1. Org deletion properly cleans up RBAC cache and audit loggers.
2. RBAC data integrity is verifiable via endpoint and programmatic helper.
3. Break-glass admin recovery exists with recovery token protection.
4. RBAC-specific Prometheus metrics are registered and recording.
5. Load benchmarks validate RBAC cache under 100+ tenant scenarios.
6. Production operations recommendation is issued with evidence.
7. RBO-00 through RBO-05 all `DONE/APPROVED` in progress tracker.

### Phase 4: GO_WITH_CONDITIONS Burn-Down

#### RBO-06: Role Mutation Metric Wiring in HTTP Handlers

Objective:
- Wire `RecordRBACRoleMutation()` calls into all RBAC mutation paths in HTTP handlers.
- Burns down GO_WITH_CONDITIONS condition #1.

Scope (1 file):
1. `internal/api/rbac_handlers.go` — Add `RecordRBACRoleMutation("create")` after successful POST in HandleRoles, `RecordRBACRoleMutation("update")` after successful PUT, `RecordRBACRoleMutation("delete")` after successful DELETE, `RecordRBACRoleMutation("assign")` after successful PUT/POST in HandleUserRoleActions.

Implementation checklist:
1. Add `RecordRBACRoleMutation("create")` after successful role creation (after audit log line).
2. Add `RecordRBACRoleMutation("update")` after successful role update (after audit log line).
3. Add `RecordRBACRoleMutation("delete")` after successful role deletion (after audit log line).
4. Add `RecordRBACRoleMutation("assign")` after successful role assignment (after audit log line).

Required tests:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "RBACMetrics|Role" -count=1` -> exit 0

Exit criteria:
- All four RBAC mutation paths emit Prometheus counter increments.

Rollback:
- Remove the four `RecordRBACRoleMutation` calls from `rbac_handlers.go`.

#### RBO-07: Audit Logger Tenant Cleanup on Org Deletion

Objective:
- Wire `RemoveTenantLogger(orgID)` into org deletion cleanup sequence.
- Burns down GO_WITH_CONDITIONS condition #3.

Scope (max 2 files):
1. `internal/api/org_handlers.go` — Add `GetTenantAuditManager().RemoveTenantLogger(orgID)` after RBAC provider cleanup in `HandleDeleteOrg`.
2. `internal/api/rbac_lifecycle_test.go` — Add test verifying audit logger cleanup during org deletion.

Implementation checklist:
1. Add nil-guarded `GetTenantAuditManager().RemoveTenantLogger(orgID)` after RBAC cleanup in `HandleDeleteOrg`.
2. Add test: create org, trigger deletion, verify audit logger manager state.

Required tests:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "RBACLifecycle|OrgLifecycle" -count=1` -> exit 0

Exit criteria:
- Org deletion cleans up audit logger for the deleted tenant.

Rollback:
- Remove cleanup call from `org_handlers.go`. Remove test.

#### RBO-08: RBAC Integrity + Admin Reset HTTP Endpoints

Objective:
- Expose `VerifyRBACIntegrity` and `ResetAdminRole` via safe admin-only HTTP endpoints.
- Burns down GO_WITH_CONDITIONS condition #2.

Scope (max 3 files):
1. `internal/api/rbac_admin_handlers.go` (new) — `HandleRBACIntegrityCheck` (GET `/api/admin/rbac/integrity`, admin-only, license-gated, returns JSON integrity result). `HandleRBACAdminReset` (POST `/api/admin/rbac/reset-admin`, admin-only, requires recovery token, calls ResetAdminRole).
2. `internal/api/router_routes_org_license.go` — Register two new routes under RBAC section.
3. `internal/api/rbac_admin_handlers_test.go` (new) — Tests for both endpoints: healthy integrity check, admin reset with valid token, admin reset without token rejected, invalid org handling.

Implementation checklist:
1. `HandleRBACIntegrityCheck`: GET handler, extracts `org_id` query param (default "default"), calls `VerifyRBACIntegrity(h.rbacProvider, orgID)`, returns JSON.
2. `HandleRBACAdminReset`: POST handler, decodes body `{org_id, username, recovery_token}`, validates recovery token via `GetRecoveryTokenStore().ValidateRecoveryTokenConstantTime()`, calls `ResetAdminRole(h.rbacProvider, orgID, username)`, returns 200 on success.
3. Route registration: admin-only, RBAC license-gated, settings-write scope for reset.
4. Tests: verify integrity endpoint returns healthy result for valid org, verify admin reset requires recovery token, verify admin reset succeeds with valid token.

Required tests:
1. `go build ./...` -> exit 0
2. `go test ./internal/api/... -run "RBACIntegrity|ResetAdminRole" -count=1` -> exit 0

Exit criteria:
- RBAC integrity check accessible via HTTP admin endpoint.
- Admin reset requires recovery token and functions correctly.

Rollback:
- Delete new handler and test files. Revert route registration.

## Milestones (Updated)

M1 complete when RBO-00 and RBO-01 are approved (scope + lifecycle fix).
M2 complete when RBO-02 and RBO-03 are approved (recovery + observability).
M3 complete when RBO-04 and RBO-05 are approved (validation + certification).
M4 complete when RBO-06, RBO-07, and RBO-08 are approved (conditions burn-down).

## Explicitly Deferred

1. **LRU eviction for RBAC managers** — optimization for extremely high org counts (1000+). Current lane validates 100-org behavior.
2. **RBAC data export/import CLI** — future operational tooling.
3. **Automated backup scheduling** — future ops automation. This lane provides the backup primitive.
4. **RBAC metrics dashboard** — Grafana/alerting dashboard. This lane provides the metrics.
5. **SSO handler tenant scoping** — OIDC/SAML group-role auto-assignment still uses global manager. Flagged in RBAC-02, separate scope.

## Scope Freeze

### In Scope
- Org deletion RBAC lifecycle cleanup (RBO-R01, RBO-R02, RBO-R03)
- RBAC data integrity verification
- Break-glass admin role reset
- RBAC backup helper
- RBAC-specific Prometheus metrics (RBO-R04)
- Load/soak benchmarks for RBAC cache (RBO-R07)
- Production operations GO/NO_GO verdict

### Out of Scope
- RBAC UI changes
- SSO tenant scoping
- RBAC data migration tooling beyond integrity checks
- Automated backup scheduling
- LRU cache eviction policy
- Changes to non-RBAC subsystems (TrueNAS, conversion, hosted)
