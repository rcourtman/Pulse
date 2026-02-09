# W6 Hosted Readiness Lane Progress Tracker

Linked plan:
- `docs/architecture/hosted-readiness-plan-2026-02.md` (authoritative execution spec)

Related lanes:
- `docs/architecture/multi-tenant-productization-plan-2026-02.md` (W4 RBAC dependency)
- `docs/architecture/monetization-foundation-plan-2026-02.md` (W0 entitlements, complete)
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (W6 exit criteria)

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
8. Respect packet subsystem boundaries; do not expand packet scope to adjacent streams.
9. All hosted-only endpoints must be gated behind `PULSE_HOSTED_MODE`.
10. Do not modify active lane files: `truenas-ga-*`, `multi-tenant-rbac-user-limits-*`, `conversion-readiness-*`.
11. W6 final certification must record W4 RBAC isolation status as a hard production dependency.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| HW-00 | Scope Freeze + Hosted Threat Model | DONE | Claude | Claude | APPROVED | HW-00 Review Evidence |
| HW-01 | Public Signup Endpoint + Hosted Mode Gate | TODO | Codex | Claude | — | — |
| HW-02 | Tenant Provisioning Service Layer | TODO | Codex | Claude | — | — |
| HW-03 | Billing-State Integration Seam: DatabaseSource | TODO | Codex | Claude | — | — |
| HW-04 | Billing-State Admin API + Org Billing Persistence | TODO | Codex | Claude | — | — |
| HW-05 | Tenant Lifecycle Operations | TODO | Codex | Claude | — | — |
| HW-06 | Hosted Observability Metrics | TODO | Codex | Claude | — | — |
| HW-07 | Hosted Operational Runbook + Security Baseline | TODO | Codex | Claude | — | — |
| HW-08 | Final Certification + Go/No-Go Verdict | TODO | Claude | Claude | — | — |

---

## HW-00 Checklist: Scope Freeze + Hosted Threat Model + Boundary Definition

- [x] Current hosted-related code audited and gaps documented.
- [x] Hosted threat model defined: trust boundaries, attack surface, data flow.
- [x] W4 RBAC dependency documented with hosted-mode prerequisite chain.
- [x] Packet boundaries and dependency gates frozen.
- [x] Definition-of-done contracts recorded.

### Required Tests

- [x] `go build ./...` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### HW-00 Review Evidence

```markdown
Files changed:
- `docs/architecture/hosted-readiness-plan-2026-02.md` (new): Full execution spec with 9 packets (HW-00 through HW-08), risk register (7 risks), dependency graph, validation baseline, non-negotiable contracts (7 contracts), and deferred items (10 items).
- `docs/architecture/hosted-readiness-progress-2026-02.md` (new): Progress tracker with per-packet checklists, checkpoint commit tracking, rules (11 rules), and review evidence sections.

Audit findings (verified):
1. Org CRUD exists (admin-only): POST/GET/PUT/DELETE /api/orgs in org_handlers.go.
2. File-based tenant isolation: multi_tenant.go with per-org directories (data/orgs/<id>/).
3. Rate limiting exists (IP-based): ratelimit.go + rate_limit_config.go. Not tenant-aware.
4. EntitlementSource interface defined with TokenSource only. DatabaseSource commented as "future".
5. Subscription state enum defined (trial/active/grace/expired/suspended) but not enforced.
6. RBAC is global: single rbac.db, not per-tenant. HARD BLOCKER for production (W4 dependency).
7. Zero billing/Stripe/payment code in codebase.
8. Zero public signup endpoints.
9. Prometheus metrics infrastructure on port 9091. No SaaS-specific metrics.
10. Organization model has no Status/lifecycle field.

Hosted threat model:
- Trust boundary: public internet → load balancer → Pulse API → per-tenant file stores
- Attack surface: signup abuse (bot registration), billing state manipulation, cross-tenant data access (via global RBAC), credential stuffing on public endpoints
- Data flow: signup request → provisioner → org dir + admin user + trial license → billing state sync → entitlement evaluation → access control
- Critical dependency: W4 RBAC isolation must complete before hosted mode is production-safe

Dependency gates frozen:
- HW-01 depends on HW-00 (scope freeze must exist before implementation)
- HW-02 depends on HW-01 (signup calls provisioner)
- HW-04 depends on HW-03 (admin API wires DatabaseSource to persistence)
- HW-06 depends on HW-01, HW-02, HW-05 (instruments all handler layers)
- HW-08 depends on all (final certification)

Commands run + exit codes:
1. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (plan/progress docs created, audit verified with explicit search evidence across 7 areas)
- P1: N/A (docs-only packet, no behavioral changes)
- P2: PASS (progress tracker initialized, packet board populated, threat model documented, dependency gates frozen)

Verdict: APPROVED

Residual risk:
- None. Docs-only packet.

Rollback:
- Delete both plan and progress docs.
```

---

## HW-01 Checklist: Public Signup Endpoint + Hosted Mode Gate

- [ ] `PULSE_HOSTED_MODE` env var detection added to router config.
- [ ] `POST /api/public/signup` endpoint with JSON payload (email, password, org_name).
- [ ] Input validation: email format, password strength, org_name path-safety.
- [ ] Signup-specific rate limiter (5/hr per IP).
- [ ] Hosted mode gate: 404 when `PULSE_HOSTED_MODE` is not enabled.
- [ ] Handler tests: success, validation failures, rate limit, hosted mode gate.

### Required Tests

- [ ] `go test ./internal/api/... -run "Hosted|Signup" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-01 Review Evidence

```markdown
TODO
```

---

## HW-02 Checklist: Tenant Provisioning Service Layer

- [ ] `Provisioner` struct with dependencies (MultiTenantPersistence, AuthManager, LicenseService).
- [ ] `ProvisionTenant()` orchestrates: validate → check uniqueness → create org → create user → assign trial.
- [ ] Idempotent: existing org with same owner email returns existing.
- [ ] Partial failure rollback: clean up org dir if user creation fails.
- [ ] Unit tests with mock dependencies.

### Required Tests

- [ ] `go test ./internal/hosted/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-02 Review Evidence

```markdown
TODO
```

---

## HW-03 Checklist: Billing-State Integration Seam: DatabaseSource

- [ ] `BillingStore` interface defined: `GetBillingState(orgID) (*BillingState, error)`.
- [ ] `DatabaseSource` implements `EntitlementSource` with cached billing store lookup.
- [ ] Fail-open: use last cached state on store failure (bounded staleness, default 1hr).
- [ ] Default behavior: trial-equivalent entitlements when no cache and store unavailable.
- [ ] Unit tests: happy path, cache hit, cache miss with store failure, cache expiry, defaults.

### Required Tests

- [ ] `go test ./internal/license/entitlements/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-03 Review Evidence

```markdown
TODO
```

---

## HW-04 Checklist: Billing-State Admin API + Org Billing Persistence

- [ ] `GET /api/admin/orgs/{id}/billing-state` returns current billing state.
- [ ] `PUT /api/admin/orgs/{id}/billing-state` sets billing state (admin-only).
- [ ] Billing state persisted as `billing.json` in org directory.
- [ ] Gated behind `PULSE_HOSTED_MODE` + `RequireAdmin`.
- [ ] Subscription_state validated against known enum.
- [ ] Audit logging for billing state changes.
- [ ] `BillingStore` wired to read from file persistence.
- [ ] Handler tests: get/set success, validation, auth gate, hosted mode gate.

### Required Tests

- [ ] `go test ./internal/api/... -run "BillingState" -count=1` -> exit 0
- [ ] `go test ./internal/config/... -run "BillingState" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-04 Review Evidence

```markdown
TODO
```

---

## HW-05 Checklist: Tenant Lifecycle Operations

- [ ] `Status` field added to Organization model (active/suspended/pending_deletion).
- [ ] `POST /api/admin/orgs/{id}/suspend` with reason and timestamp.
- [ ] `POST /api/admin/orgs/{id}/unsuspend` restores active status.
- [ ] `DELETE /api/admin/orgs/{id}/soft-delete` sets pending_deletion with retention period.
- [ ] Default org guard: cannot suspend/delete default org.
- [ ] Suspended org middleware check: reject non-admin API requests.
- [ ] Audit log entries for all lifecycle state changes.
- [ ] Handler tests: suspend/unsuspend/soft-delete success, default org guard, auth gate.

### Required Tests

- [ ] `go test ./internal/api/... -run "OrgLifecycle|Suspend|Unsuspend" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-05 Review Evidence

```markdown
TODO
```

---

## HW-06 Checklist: Hosted Observability Metrics

- [ ] Prometheus counters defined: signups_total, provisions_total, lifecycle_transitions_total, active_tenants gauge.
- [ ] Metrics registered on init.
- [ ] Signup, provisioner, and lifecycle handlers instrumented.
- [ ] Aggregate labels only (no per-tenant labels).
- [ ] Unit tests for counter registration and increment.

### Required Tests

- [ ] `go test ./internal/metrics/... -run "Hosted" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-06 Review Evidence

```markdown
TODO
```

---

## HW-07 Checklist: Hosted Operational Runbook + Security Baseline

- [ ] Operational runbook: provisioning, suspend/unsuspend/delete, billing override, escalation.
- [ ] SLO definitions: API availability, provisioning latency, billing propagation delay.
- [ ] Security baseline: trust boundaries, data handling, auth model, attack surface.
- [ ] Data handling policy: isolation, retention, deletion, export.

### Required Tests

- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-07 Review Evidence

```markdown
TODO
```

---

## HW-08 Checklist: Final Certification + Go/No-Go Verdict

- [ ] HW-00 through HW-07 are all `DONE` and `APPROVED`.
- [ ] Full milestone validation commands rerun with explicit exit codes.
- [ ] W4 RBAC dependency status recorded with production impact assessment.
- [ ] Hosted launch posture determined (`GA`, `private_beta`, or `waitlist`).
- [ ] Rollback runbook recorded.
- [ ] Final readiness verdict recorded (`GO`, `GO_WITH_CONDITIONS`, or `NO_GO`).

### Required Tests

- [ ] `go build ./... && go test ./internal/api/... ./internal/hosted/... ./internal/license/entitlements/... ./internal/metrics/... -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### HW-08 Review Evidence

```markdown
TODO
```

---

## Checkpoint Commits

- HW-00: TODO
- HW-01: TODO
- HW-02: TODO
- HW-03: TODO
- HW-04: TODO
- HW-05: TODO
- HW-06: TODO
- HW-07: TODO
- HW-08: TODO

## Current Recommended Next Packet

- `HW-01` (Public Signup Endpoint + Hosted Mode Gate)
