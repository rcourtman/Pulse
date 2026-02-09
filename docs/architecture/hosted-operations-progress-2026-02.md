# Hosted Operations Progress (HOP Lane) — 2026-02

Status: Active
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
| HOP-00 | Scope freeze + condition reconciliation | DONE | (pending) | Plan + progress docs created, build passes |
| HOP-01 | Hosted mode rollout policy | PENDING | — | — |
| HOP-02 | Tenant lifecycle safety drills | PENDING | — | — |
| HOP-03 | Billing-state controls + metrics wiring | PENDING | — | — |
| HOP-04 | SLO/alert tuning + incident playbooks | PENDING | — | — |
| HOP-05 | Final operational verdict | PENDING | — | — |

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
