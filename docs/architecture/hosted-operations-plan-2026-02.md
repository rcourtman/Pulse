# Hosted Operations Plan (HOP Lane) — 2026-02

Status: Active
Owner: Orchestrator (Claude)
Created: 2026-02-09
Predecessor: W6 Hosted Readiness Lane (HW-00 through HW-08, LANE_COMPLETE)

## 1. Purpose

Resolve the GO_WITH_CONDITIONS items from HW-08 certification and operationalize hosted mode for private beta rollout. This lane bridges the gap between "code exists" (HW lane) and "safe to enable in production" (operational readiness).

## 2. Predecessor Reconciliation

### HW-08 Verdict: GO_WITH_CONDITIONS (private_beta)

| # | Condition | Current Status | HOP Disposition |
|---|-----------|----------------|-----------------|
| 1 | W4 RBAC per-tenant isolation | **RESOLVED** — RBAC lane LANE_COMPLETE (RBAC-00 through RBAC-05 DONE/APPROVED). TenantRBACProvider with per-org SQLite, lazy-loading, 34 tests. | No HOP action needed. Record as resolved in HOP-00. |
| 2 | Hosted mode gated behind `PULSE_HOSTED_MODE` env var | **IN PLACE** — All hosted routes return 404 when disabled. | Verified. Document rollout policy in HOP-01. |
| 3 | Private beta limited to trusted tenants | **POLICY** — No technical enforcement yet (signup rate limit only). | Document operational controls in HOP-01. |
| 4 | TestRouterRouteInventory failure from parallel work | **OUT OF SCOPE** — Caused by parallel TrueNAS/conversion lanes, not hosted code. | No HOP action needed. |

### HW-08 GA Upgrade Conditions

| # | GA Condition | HOP Action |
|---|-------------|------------|
| 1 | W4 RBAC per-tenant isolation | **DONE** (RBAC lane complete). |
| 2 | Suspended-org enforcement middleware | **IN PROGRESS** — W4 RBAC now complete, per-request org resolution available. Target HOP-06. |
| 3 | Background reaper for soft-deleted orgs | **IN PROGRESS** — Safety path with dry-run default. Target HOP-07. |
| 4 | Stripe/payment integration | Deferred (manual billing override is sufficient for private beta). Document gap in HOP-05. |
| 5 | Handler instrumentation wired to hosted metrics | Wire existing HostedMetrics calls into hosted handlers. Target HOP-03. |
| 6 | Load testing | Deferred to pre-GA phase. Document in HOP-05. |

## 3. Packet Definitions

### HOP-00: Scope Freeze + Condition Reconciliation
- **Shape**: docs only
- **Goal**: Create plan + progress docs. Record resolved/deferred conditions.
- **Scope**: `docs/architecture/hosted-operations-{plan,progress}-2026-02.md`
- **Acceptance**: Docs exist, `go build ./...` passes, checkpoint commit.

### HOP-01: Hosted Mode Rollout Policy
- **Shape**: docs + lightweight code validation
- **Goal**: Document rollout policy: environments, enable/disable criteria, operational procedures for toggling `PULSE_HOSTED_MODE`.
- **Scope**: `docs/architecture/hosted-operational-runbook-2026-02.md` (append rollout policy section), validation that hosted gate works correctly.
- **Acceptance**: Runbook updated with rollout section, `go test ./internal/hosted/... ./internal/api/... -run Hosted -count=1` passes.

### HOP-02: Tenant Lifecycle Safety Drills
- **Shape**: tests only
- **Goal**: Add integration-style tests that rehearse suspend/unsuspend/soft-delete flows end-to-end, including rollback scenarios and default-org guard.
- **Scope**: `internal/api/org_lifecycle_handlers_test.go` (extend existing tests)
- **Acceptance**: New drill tests pass, `go test ./internal/api/... -run Lifecycle -count=1` passes.

### HOP-03: Billing-State Operational Controls + Metrics Wiring
- **Shape**: code (integration)
- **Goal**: (a) Wire HostedMetrics.RecordProvision/RecordLifecycleTransition into hosted handlers. (b) Add billing-state audit verification test.
- **Scope**: `internal/api/billing_state_handlers.go`, `internal/api/org_lifecycle_handlers.go`, `internal/api/hosted_signup_handler.go` (if exists), tests.
- **Acceptance**: Metrics calls present in handlers, `go test ./internal/api/... ./internal/hosted/... -count=1` passes.

### HOP-04: SLO/Alert Tuning + Incident Playbooks
- **Shape**: docs only
- **Goal**: Refine SLO definitions from runbook, add alert threshold recommendations, create incident response playbooks for P1-P4 scenarios.
- **Scope**: `docs/architecture/hosted-operational-runbook-2026-02.md` (extend existing sections)
- **Acceptance**: Runbook sections 4-6 updated with concrete thresholds and playbooks.

### HOP-05: Final Operational Verdict
- **Shape**: docs (certification)
- **Goal**: Final GO/GO_WITH_CONDITIONS/NO_GO verdict for hosted private beta operational readiness.
- **Scope**: Progress doc updated with final verdict, gap register for GA.
- **Acceptance**: Verdict recorded with evidence chain.

### HOP-06: Suspended-Org Enforcement Middleware
- **Shape**: code (middleware + tests)
- **Goal**: Add org status check to TenantMiddleware. If org is suspended or pending_deletion, reject non-admin API requests with 403. Admin users retain access for lifecycle operations.
- **Scope**: `internal/api/middleware_tenant.go`, `internal/api/middleware_tenant_test.go`
- **Constraints**: Must not break existing TenantMiddleware behavior for active orgs. Default org exempt. Admin bypass required.
- **Acceptance**: `go build ./...` exit 0, `go test ./internal/api/... -run "Suspend|HostedModeGate" -count=1` exit 0.

### HOP-07: Pending-Deletion Background Reaper Safety Path
- **Shape**: code (scaffold + tests)
- **Goal**: Background reaper that scans for pending_deletion orgs past retention period. Safety-first: default dry-run mode logs what would be purged without deleting. Live mode requires explicit opt-in via `PULSE_REAPER_LIVE=true`.
- **Scope**: `internal/hosted/reaper.go`, `internal/hosted/reaper_test.go`
- **Constraints**: Default org never reaped. Configurable scan interval. Graceful shutdown via context cancellation.
- **Acceptance**: `go build ./...` exit 0, `go test ./internal/hosted/... -count=1` exit 0.

### HOP-08: Tenant-Aware Rate Limiting Baseline
- **Shape**: code (enhancement + tests)
- **Goal**: Add per-org rate limiting alongside existing IP-based limiting. Key on org ID from TenantMiddleware context. Applies after TenantMiddleware resolves org.
- **Scope**: `internal/api/ratelimit_tenant.go`, `internal/api/ratelimit_tenant_test.go`
- **Constraints**: Must not interfere with existing IP-based rate limiting. Default org gets higher limits. Separate from signup rate limiter.
- **Acceptance**: `go build ./...` exit 0, `go test ./internal/api/... -run "RateLimit" -count=1` exit 0.

### HOP-09: Follow-Up Verdict + Checkpoint
- **Shape**: docs (certification)
- **Goal**: Rerun full validation suite, update progress with HOP-06 through HOP-08 evidence, record updated verdict.
- **Scope**: Progress doc, plan doc deferred-items update.
- **Acceptance**: All validation commands exit 0, verdict recorded.

## 4. Non-Negotiable Contracts

1. **`PULSE_HOSTED_MODE` stays disabled** until HOP-05 produces a GO or GO_WITH_CONDITIONS verdict.
2. **No direct code writes** — all implementation delegated via Codex MCP.
3. **Evidence-based approvals** — reviewer independently reruns commands and records exit codes.
4. **Git safety** — no destructive commands, path-specific staging only.
5. **Parallel work awareness** — do not touch files outside packet scope.

## 5. Risk Register

| # | Risk | Mitigation |
|---|------|------------|
| R1 | Hosted handlers lack metrics instrumentation | HOP-03 wires existing HostedMetrics into handlers |
| R2 | Lifecycle safety untested beyond unit level | HOP-02 adds integration-style drill tests |
| R3 | Rollout policy undocumented | HOP-01 documents enable/disable procedures |
| R4 | SLO thresholds not validated | HOP-04 adds concrete alert recommendations |
| R5 | GA conditions remain open | HOP-05 explicitly catalogs deferred items |
| R6 | Suspended orgs can still access API | HOP-06 adds enforcement middleware |
| R7 | Soft-deleted orgs persist indefinitely | HOP-07 adds reaper with dry-run safety |
| R8 | Single tenant can exhaust API capacity | HOP-08 adds per-org rate limiting |

## 6. Explicitly Deferred (Post-HOP / Pre-GA)

1. ~~Suspended-org enforcement middleware~~ → **Addressed in HOP-06**
2. ~~Background reaper for pending_deletion orgs~~ → **Addressed in HOP-07**
3. Stripe/payment integration
4. Load testing under hosted concurrency
5. Email verification for signup flow
6. Password reset flow
7. ~~Tenant-aware rate limiting (beyond IP-based)~~ → **Addressed in HOP-08**
8. SSO handler migration from global auth to TenantRBACProvider
