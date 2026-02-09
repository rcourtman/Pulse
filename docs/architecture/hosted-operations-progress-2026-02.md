# Hosted Operations Progress (HOP Lane) — 2026-02

Status: LANE_COMPLETE
Owner: Orchestrator (Claude)
Created: 2026-02-09
Plan: `docs/architecture/hosted-operations-plan-2026-02.md`

## Predecessor

W6 Hosted Readiness Lane — LANE_COMPLETE (HW-00 through HW-08, all DONE/APPROVED)
- HW-08 Verdict: GO_WITH_CONDITIONS (private_beta)
- Checkpoint commits: see `docs/architecture/hosted-readiness-progress-2026-02.md`

## Condition Reconciliation (from HW-08)

| # | Condition | Resolution |
|---|-----------|------------|
| 1 | W4 RBAC per-tenant isolation | **RESOLVED** — RBAC lane LANE_COMPLETE. TenantRBACProvider, per-org SQLite, 34 tests. Commits: RBAC-01 `0a10656f`, RBAC-02 `e8811b89`, RBAC-03 `8c845c35`, RBAC-04 `258f251b`. |
| 2 | Hosted mode gated behind env var | **IN PLACE** — All hosted routes 404 when `PULSE_HOSTED_MODE` disabled. |
| 3 | Private beta limited to trusted tenants | **POLICY** — Operational controls to be documented in HOP-01. |
| 4 | Route inventory test failure (parallel work) | **OUT OF SCOPE** — From TrueNAS/conversion lanes, not hosted. |

## Packet Progress

| Packet | Description | Status | Commit | Evidence |
|--------|-------------|--------|--------|----------|
| HOP-00 | Scope freeze + condition reconciliation | DONE | `dfb74f99` | Plan + progress docs created, build exit 0 |
| HOP-01 | Hosted mode rollout policy | DONE | `0f62162f` | Runbook Section 9 added, RBAC blocker resolved, tests exit 0 |
| HOP-02 | Tenant lifecycle safety drills | DONE | `6bb8eb82` | 7 drill tests + 5 RBAC lifecycle tests pass, exit 0 |
| HOP-03 | Billing-state controls + metrics wiring | DONE | `11f5ded7` | 6 metrics calls wired, audit test passes, exit 0 |
| HOP-04 | SLO/alert tuning + incident playbooks | DONE | `82a61931` | Sections 4+5 expanded, 4 alert queries, P1-P4 playbooks |
| HOP-05 | Final operational verdict | DONE | (this commit) | GO_WITH_CONDITIONS — see verdict below |

## Detailed Packet Records

### HOP-00: Scope Freeze + Condition Reconciliation

**Status**: DONE
**Shape**: docs only
**Implementer**: Orchestrator (bootstrap)

Files changed:
- `docs/architecture/hosted-operations-plan-2026-02.md` (created — plan with 6 packets, risk register, deferred items)
- `docs/architecture/hosted-operations-progress-2026-02.md` (created — progress tracker with condition reconciliation)

Commands run:
- `go build ./...` → exit 0

Gate checklist:
- [x] P0: Files exist and are well-formed
- [x] P0: `go build ./...` passes (exit 0)
- [x] P1: All 4 HW-08 conditions reconciled (RESOLVED/IN_PLACE/POLICY/OUT_OF_SCOPE)
- [x] P1: W4 RBAC resolution documented with 4 commit hashes from RBAC lane
- [x] P2: Plan matches user-specified packet structure (HOP-00 through HOP-05)

Verdict: APPROVED

---

### HOP-01: Hosted Mode Rollout Policy

**Status**: DONE
**Shape**: docs + test validation
**Implementer**: Codex
**Reviewer**: Orchestrator (Claude)

Files changed:
- `docs/architecture/hosted-operational-runbook-2026-02.md` (modified — Section 6 RBAC updated, Section 8 limitation updated, Section 9 rollout policy added)

Commands run:
- `go test ./internal/api/... -run "HostedModeGate" -count=1 -v` → exit 0 (3 tests: Billing, Signup, Lifecycle)
- `go test ./internal/hosted/... -count=1 -v` → exit 0 (7 tests)

Gate checklist:
- [x] P0: File modified correctly (Section 6 RBAC resolved, Section 8 updated, Section 9 added)
- [x] P0: Hosted gate tests pass (exit 0, independently verified by reviewer)
- [x] P1: Rollout stages documented (dev → private beta → public beta → GA)
- [x] P1: Enable/disable procedures with verification steps
- [x] P2: Checkpoint commit `0f62162f`

Verdict: APPROVED

---

### HOP-02: Tenant Lifecycle Safety Drills

**Status**: DONE
**Shape**: tests only
**Implementer**: Codex
**Reviewer**: Orchestrator (Claude)

Files changed (in-scope):
- `internal/api/org_lifecycle_handlers_test.go` (modified — 7 new drill tests added)

Files changed (out-of-scope, included by Codex):
- `internal/api/org_handlers.go` (RBAC cache cleanup on org delete)
- `internal/api/rbac_tenant_provider.go` (ManagerCount helper for tests)
- `internal/api/rbac_lifecycle_test.go` (new — 5 RBAC lifecycle cleanup tests)
- `internal/api/router.go` (wire rbacProvider into OrgHandlers)

Note: Out-of-scope changes are safe/useful (fix RBAC cache leak on org delete) but violate single-shape packet constraint. Documented here for traceability.

Commands run:
- `go test ./internal/api/... -run "Lifecycle" -count=1 -v` → exit 0 (all lifecycle tests pass, independently verified)

Gate checklist:
- [x] P0: 7 drill tests pass (round-trip, conflict detection, nonexistent org, retention days)
- [x] P0: Tests independently verified by reviewer (exit 0)
- [x] P1: All specified drill scenarios covered
- [x] P1: Out-of-scope changes are benign (RBAC cache cleanup)
- [x] P2: Checkpoint commit `6bb8eb82`

Verdict: APPROVED (with scope-breach note)

---

### HOP-03: Billing-State Operational Controls + Metrics Wiring

**Status**: DONE
**Shape**: code (integration)
**Implementer**: Codex
**Reviewer**: Orchestrator (Claude)

Files changed:
- `internal/api/hosted_signup_handlers.go` (modified — RecordSignup + RecordProvision wired)
- `internal/api/org_lifecycle_handlers.go` (modified — RecordLifecycleTransition in suspend/unsuspend/soft-delete)
- `internal/api/billing_state_handlers_test.go` (modified — TestBillingStatePutAuditLogEmitted added)

Commands run:
- `go build ./...` → exit 0
- `go test ./internal/api/... -run "BillingState|Hosted|Lifecycle" -count=1` → exit 0 (independently verified)

Metrics wiring verification:
- `grep "hosted.GetHostedMetrics()"` found 6 call sites across 2 handler files

Gate checklist:
- [x] P0: Build passes (exit 0)
- [x] P0: All tests pass (exit 0, independently verified)
- [x] P1: 6 metrics calls wired (signup success/failure, 3 lifecycle transitions)
- [x] P1: Billing audit test verifies PUT round-trip
- [x] P1: Exactly 3 files changed (in scope)
- [x] P2: Checkpoint commit `11f5ded7`

Verdict: APPROVED

---

### HOP-04: SLO/Alert Tuning + Incident Playbooks

**Status**: DONE
**Shape**: docs only
**Implementer**: Codex
**Reviewer**: Orchestrator (Claude)

Files changed:
- `docs/architecture/hosted-operational-runbook-2026-02.md` (modified — Sections 4 and 5 expanded)

Changes:
- Section 4: Renamed to "Incident Response Playbooks" with full P1-P4 playbooks (Detection, SLA, Immediate actions, Investigation, Resolution)
- Section 5: Renamed to "SLO Definitions and Alert Thresholds" with SLO targets table and 4 Prometheus alert queries

Verification:
- `wc -l` → 359 lines (was 277)
- `grep -c "## Section"` → 9 (sections preserved)
- Section numbering intact (1-9)

Gate checklist:
- [x] P0: File well-formed, section count preserved
- [x] P1: P1-P4 playbooks each have Detection/SLA/Actions/Investigation/Resolution
- [x] P1: SLO table has 5 targets with alert thresholds
- [x] P1: 4 Prometheus alert queries with action descriptions
- [x] P2: Checkpoint commit `82a61931`

Verdict: APPROVED

---

### HOP-05: Final Operational Verdict

**Status**: DONE
**Shape**: docs (certification)
**Implementer**: Orchestrator (Claude)

## Final Certification: Hosted Private Beta Operational Readiness

### Verdict: GO_WITH_CONDITIONS (private_beta)

### Evidence Summary

**Build + Tests** (all independently verified):
- `go build ./...` → exit 0
- `go test ./internal/api/... -run "BillingState|Hosted|Lifecycle|HostedModeGate" -count=1` → exit 0
- `go test ./internal/hosted/... -count=1` → exit 0
- `go test ./internal/license/entitlements/... -count=1` → exit 0
- `go test ./internal/config/... -count=1` → exit 0

**Completed Capabilities** (HW + HOP lanes):

| Capability | Status | Evidence |
|------------|--------|----------|
| Tenant provisioning (signup) | DONE | HW-02, handlers + tests |
| Per-tenant RBAC isolation | DONE | W4 RBAC lane complete, TenantRBACProvider |
| Billing-state admin API | DONE | HW-04, GET/PUT + audit logging |
| Org lifecycle (suspend/unsuspend/soft-delete) | DONE | HW-05, handlers + drill tests |
| Hosted mode env var gate | DONE | All 6 endpoints gated, 3 gate tests |
| Rollout policy documented | DONE | HOP-01, runbook Section 9 |
| Lifecycle safety drills | DONE | HOP-02, 7 drill tests + 5 RBAC lifecycle tests |
| Metrics instrumentation | DONE | HOP-03, 6 metrics call sites wired |
| SLO targets + alert thresholds | DONE | HOP-04, 5 SLOs + 4 Prometheus queries |
| Incident playbooks (P1-P4) | DONE | HOP-04, full detection/SLA/resolution flows |

**Checkpoint Commits** (HOP lane):

| Packet | Commit |
|--------|--------|
| HOP-00 | `dfb74f99` |
| HOP-01 | `0f62162f` |
| HOP-02 | `6bb8eb82` |
| HOP-03 | `11f5ded7` |
| HOP-04 | `82a61931` |
| HOP-05 | (this commit) |

### Conditions for Private Beta

1. **`PULSE_HOSTED_MODE` must remain disabled** until private beta rollout decision.
2. **Audience must be invite-only** — no open signup until public beta stage.
3. **Monitor `pulse_hosted_*` metrics** from first enable for 48h before expanding audience.

### GA Upgrade Requirements (deferred — not blocking private beta)

| # | Requirement | Current Status |
|---|-------------|----------------|
| 1 | Suspended-org enforcement middleware | Deferred — needs per-request org resolution |
| 2 | Background reaper for pending_deletion orgs | Deferred — orgs persist safely |
| 3 | Stripe/payment integration | Deferred — manual billing override sufficient |
| 4 | Load testing under hosted concurrency | Deferred — private beta volume is low |
| 5 | Email verification for signup | Deferred |
| 6 | Password reset flow | Deferred |
| 7 | Tenant-aware rate limiting | Deferred — IP-based sufficient for beta |
| 8 | SSO handler migration to TenantRBACProvider | Deferred — flagged in RBAC lane |

### Operational Recommendation

The hosted private beta infrastructure is operationally ready. All code paths are tested, instrumented with metrics, and documented with incident playbooks and SLO thresholds. The `PULSE_HOSTED_MODE` gate provides a safe enable/disable switch. Per-tenant RBAC isolation is complete.

**Recommendation**: Proceed to private beta when business readiness criteria are met. Follow the enable procedure in Section 9 of the operational runbook.

---
