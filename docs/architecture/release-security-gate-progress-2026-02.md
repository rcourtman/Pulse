# Release Security Gate Progress Tracker

Linked plan:
- `docs/architecture/release-security-gate-plan-2026-02.md`

Status: In Progress
Date: 2026-02-09

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Reviewer must provide explicit command exit-code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or replaced by summary-only claims.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. After every `APPROVED` packet, create a checkpoint commit and record the hash.
6. Do not use destructive git commands on shared worktrees.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| SEC-00 | Scope Freeze + Threat Replay Contract | DONE | Claude | Claude | APPROVED | SEC-00 section below |
| SEC-01 | AuthZ + Tenant Isolation Replay | PENDING | Codex | Claude | — | — |
| SEC-02 | Secret Exposure + Dependency Risk Audit | PENDING | Codex | Claude | — | — |
| SEC-03 | Security Runbook + Incident Readiness Ratification | PENDING | Codex | Claude | — | — |
| SEC-04 | Final Security Verdict | PENDING | Claude | Claude | — | — |

---

## SEC-00 Checklist: Scope Freeze + Threat Replay Contract

- [x] Scope boundaries and in-scope systems declared.
- [x] Severity rubric (`P0`/`P1`/`P2`) documented.
- [x] Dependency order and blocking criteria recorded.

### In-Scope Security Surfaces

| Surface | Package(s) | Test File Count | Test Function Count |
|---------|-----------|-----------------|---------------------|
| API Authentication | `internal/api`, `pkg/auth` | 20+ | 125+ |
| API Authorization / Scope | `internal/api` | 15+ | 95+ |
| Tenant Isolation (API) | `internal/api` | 12+ | 27+ |
| RBAC Isolation | `internal/api`, `pkg/auth` | 12+ | 35+ |
| WebSocket Tenant Isolation | `internal/websocket` | 3 | 9 |
| Monitoring Tenant Isolation | `internal/monitoring` | 2 | 3 |
| Notification Security | `internal/notifications` | 4 | 19 |
| Audit Logging & Integrity | `pkg/audit` | 6 | 34 |
| Security Hardening (OIDC, tokens, recovery) | `internal/api` | 10+ | 35+ |

**Out of scope:** AI/patrol autonomy, frontend-only rendering, mock data generation, Docker image build pipeline.

### Severity Rubric

| Severity | Definition | Release Impact |
|----------|-----------|----------------|
| **P0** | Auth bypass, tenant data leak, scope enforcement failure, unmitigated secret exposure, exploitable dependency CVE | **Blocks public release.** Must be resolved or accepted with explicit owner + follow-up date. |
| **P1** | Missing regression coverage for critical path, stale runbook section, dependency vuln with no known exploit path | **Must be triaged.** Acceptable with documented risk acceptance and remediation timeline. |
| **P2** | Documentation drift, cosmetic test naming, advisory-level dependency findings | **Does not block.** Tracked for post-release follow-up. |

### Packet Dependency Order

```
SEC-00 (scope freeze) ──blocks──► SEC-01 (auth/tenant replay)
SEC-00 ──blocks──► SEC-02 (secret/dependency audit)
SEC-00 ──blocks──► SEC-03 (runbook ratification)
SEC-01 + SEC-02 + SEC-03 ──all block──► SEC-04 (final verdict)
```

- SEC-01, SEC-02, SEC-03 may execute in parallel after SEC-00 is approved.
- SEC-04 requires all three predecessor packets to be `DONE/APPROVED`.
- Any `P0` finding in SEC-01..SEC-03 blocks SEC-04 verdict as `GO`.

### Required Commands

- [x] `go build ./...` -> exit 0

### Review Evidence

```
go build ./... -> exit 0 (2026-02-09, verified by orchestrator)
```

### Review Gates

- [x] P0 PASS — No execution integrity issues; scope and rubric are explicit.
- [x] P1 PASS — N/A for docs-only packet; no behavioral paths to validate.
- [x] P2 PASS — Progress tracker updated accurately; packet status matches evidence.
- [x] Verdict recorded

### SEC-00 Review Record

```
Files changed:
- docs/architecture/release-security-gate-progress-2026-02.md: scope freeze, severity rubric, dependency order

Commands run + exit codes:
1. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (scope and rubric documented; build passes)
- P1: N/A (docs-only packet)
- P2: PASS (tracker updated accurately)

Verdict: APPROVED

Commit:
- <pending checkpoint>

Residual risk:
- None

Rollback:
- Revert checkpoint commit
```

## SEC-01 Checklist: AuthZ + Tenant Isolation Replay

- [ ] API security/scope/tenant suites rerun.
- [ ] Websocket isolation suites rerun.
- [ ] Monitoring isolation suites rerun.
- [ ] Missing critical regressions covered by tests.

### Required Commands

- [ ] `go build ./...` -> exit 0
- [ ] `go test ./pkg/auth/... -count=1` -> exit 0
- [ ] `go test ./internal/api/... -run "Security|Scope|Authorization|Spoof|Tenant|RBAC|Org" -count=1` -> exit 0
- [ ] `go test ./internal/websocket/... -run "Tenant|Isolation|Alert" -count=1` -> exit 0
- [ ] `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation" -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

## SEC-02 Checklist: Secret Exposure + Dependency Risk Audit

- [ ] Secret scan executed and triaged.
- [ ] Go vuln scan executed and triaged.
- [ ] Frontend dependency scan executed and triaged.
- [ ] No unaccepted `P0` findings remain.

### Required Commands

- [ ] `gitleaks detect --no-git --source .` -> exit 0
- [ ] `govulncheck ./...` -> exit 0
- [ ] `cd frontend-modern && npm audit --omit=dev` -> exit 0 or triaged non-blocking findings

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

## SEC-03 Checklist: Security Runbook + Incident Readiness Ratification

- [ ] Incident playbooks verified against current architecture.
- [ ] Rollback and kill-switch paths verified for W2/W4/W6.
- [ ] Escalation and ownership sections verified.

### Required Commands

- [ ] `rg -n "rollback|kill-switch|incident|SLA|severity" docs/architecture/*runbook*.md` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded

## SEC-04 Checklist: Final Security Verdict

- [ ] SEC-00 through SEC-03 are `DONE` and `APPROVED`.
- [ ] Milestone command set rerun with explicit exit codes.
- [ ] Residual risks documented with owner and follow-up date.
- [ ] Final verdict recorded (`GO` / `GO_WITH_CONDITIONS` / `NO_GO`).

### Required Commands

- [ ] `go build ./...` -> exit 0
- [ ] `go test ./pkg/auth/... -count=1` -> exit 0
- [ ] `go test ./internal/api/... -run "Security|Scope|Authorization|Spoof|Tenant|RBAC|Org" -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded
