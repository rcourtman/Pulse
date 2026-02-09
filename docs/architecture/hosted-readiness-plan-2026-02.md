# W6 Hosted Readiness Lane Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/hosted-readiness-progress-2026-02.md`

Related lanes:
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (W6 exit criteria)
- `docs/architecture/gap-analysis-2026-02.md` (W6 current state assessment)
- `docs/architecture/multi-tenant-productization-plan-2026-02.md` (W4 RBAC dependency)
- `docs/architecture/monetization-foundation-plan-2026-02.md` (W0 entitlement primitives, complete)

## Intent

Graduate Pulse from a self-hosted-only product to a hosted-ready platform. The hosted path must support public signup, automated tenant provisioning, external billing-state integration, tenant lifecycle management, and operational observability — all without breaking the existing self-hosted deployment model.

This lane does NOT build full Stripe checkout or payment processing. It builds the integration seams and operational contracts required for a manual/external-compatible hosted offering (consistent with the Release Readiness Guiding Light: "Billing Integration: Stripe/Paddle integration is manual/external for v1. No in-app checkout code.").

Primary outcomes:
1. Hosted mode is an explicit, gated operational posture (`PULSE_HOSTED_MODE`).
2. Public signup creates trial tenants with validated input and abuse-resistant rate limiting.
3. Tenant provisioning is atomic, idempotent, and orchestrated (org + user + trial license).
4. Billing state flows through a `DatabaseSource` entitlement implementation compatible with external billing systems.
5. Tenant lifecycle operations (suspend/unsuspend/soft-delete) are safe and auditable.
6. Hosted-specific observability metrics and operational runbook exist.
7. Security baseline and data handling policy are publishable.

## Definition of Done

This lane is complete only when all are true:
1. `PULSE_HOSTED_MODE` feature flag gates all hosted-only endpoints and behavior.
2. A public signup endpoint creates trial tenants with input validation and rate limiting.
3. Tenant provisioning orchestrates org creation + admin user + trial license atomically.
4. `DatabaseSource` implements `EntitlementSource` for SaaS entitlement lookup.
5. An admin API allows manual billing-state override per tenant (external billing compatible).
6. Suspend/unsuspend/soft-delete operations exist with safety guards.
7. Hosted observability metrics are registered and emitting.
8. Operational runbook with SLOs, escalation paths, and security baseline is publishable.
9. Final certification packet approved with explicit go/no-go verdict.
10. If full GA is not achievable, lane explicitly labels hosted as `private_beta` or `waitlist` with concrete unblock path.

## Code-Derived Baseline (Current State)

### A. Org/Tenant Provisioning — PARTIAL

1. `internal/config/multi_tenant.go`: `MultiTenantPersistence` with `GetPersistence(orgID)`, `OrgExists()`, `ListOrganizations()`, `DeleteOrganization()`. Directory-based isolation: default org uses root data dir, non-default orgs use `data/orgs/<org-id>/`.
2. `internal/api/org_handlers.go`: Admin-only CRUD — `POST /api/orgs`, `GET /api/orgs`, `GET/PUT/DELETE /api/orgs/{id}`, member management, resource sharing. All require authenticated session.
3. `internal/models/models.go`: `Organization` struct with ID, DisplayName, OwnerUserID, CreatedAt, Members, SharedResources. No lifecycle state field.

### B. Authentication / RBAC — GLOBAL (W4 Dependency)

1. `pkg/auth/sqlite_manager.go`: Single global `rbac.db` shared across all tenants.
2. Users and roles are not scoped to OrgID. This is a known W4 blocker.
3. W6 can build provisioning and lifecycle ops, but production security requires W4 RBAC isolation.

### C. Rate Limiting — IP-BASED

1. `internal/api/ratelimit.go`: `RateLimiter` with per-IP token bucket.
2. `internal/api/rate_limit_config.go`: Endpoint-specific limits (auth: 10/min, general: 500/min).
3. No tenant-aware rate limiting.

### D. Entitlement System — TOKEN-ONLY

1. `internal/license/entitlements/source.go`: `EntitlementSource` interface with 5 methods.
2. `internal/license/entitlements/token_source.go`: JWT-based `TokenSource` (self-hosted only).
3. `DatabaseSource` commented as "future" — not implemented.
4. Subscription state enum defined (`trial/active/grace/expired/suspended`) but not enforced at runtime.

### E. Billing / Signup — NONE

1. Zero Stripe/payment/billing code in codebase.
2. No public signup endpoints.
3. No billing state persistence per tenant.

### F. Observability — INFRASTRUCTURE-ONLY

1. Prometheus metrics endpoint on port 9091.
2. `internal/metrics/incident_recorder.go`: infrastructure-level metrics.
3. No SaaS-specific metrics (tenant count, signup rate, lifecycle transitions).

## Non-Negotiable Contracts

1. Hosted mode isolation:
- All hosted-only endpoints are gated behind `PULSE_HOSTED_MODE=true`.
- Self-hosted deployments must not expose signup, billing-state, or hosted lifecycle APIs.
- Hosted mode requires `PULSE_MULTI_TENANT_ENABLED=true` as a prerequisite.

2. Packet scope contract:
- No packet crosses more than one subsystem boundary (backend, frontend, infra, docs).
- No packet combines abstraction creation + rewiring + deletion in one step.
- Max 3-5 files changed target per packet.

3. Evidence contract:
- No packet is APPROVED without explicit command exit codes.
- Timeout, empty output, truncated output, or missing exit code is an automatic failed gate.

4. Rollback contract:
- Each packet has file-level rollback guidance.
- No destructive git operations.

5. Security contract:
- No plaintext credential storage.
- Signup rate limiting must prevent abuse (bot registration, DoS).
- Tenant provisioning must be idempotent (retry-safe).
- Billing state changes must be auditable.

6. Lane isolation contract:
- Do not modify active lane files: `truenas-ga-*`, `multi-tenant-rbac-user-limits-*`, `conversion-readiness-*`.
- Stage only packet-scoped files in commits.

7. W4 dependency contract:
- W6 proceeds with building hosted plumbing in parallel with W4 RBAC isolation.
- W6 final certification (HW-08) MUST note W4 RBAC as a hard prerequisite for production hosted deployment.
- No W6 packet claims production-ready security without W4 RBAC completion evidence.

## Risk Register

| ID | Severity | Risk | Mitigation Packet |
|---|---|---|---|
| HW-R001 | Critical | RBAC is global — cross-tenant auth bypass possible in hosted mode. | External (W4). HW-08 records dependency. |
| HW-R002 | High | Signup abuse — bots registering thousands of trial tenants. | HW-01 (strict rate limiter: 5/hr per IP). |
| HW-R003 | High | Billing state bypass — tenant manually overrides own billing state. | HW-04 (admin-only API, not tenant-accessible). |
| HW-R004 | High | Provisioning race condition — concurrent signups with same email. | HW-02 (idempotent provisioner with email uniqueness). |
| HW-R005 | Medium | Soft-delete data retention — deleted tenant data lingers indefinitely. | HW-05 (configurable retention period, documented cleanup). |
| HW-R006 | Medium | DatabaseSource availability — billing DB down blocks entitlement evaluation. | HW-03 (fail-open with cached state, bounded staleness). |
| HW-R007 | Low | Hosted metrics cardinality — unbounded tenant labels in Prometheus. | HW-06 (aggregate counters, not per-tenant labels). |

## Orchestrator Operating Model

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

Run after every backend packet unless explicitly waived:

1. `go build ./...`
2. `go test ./internal/api/... -run "Hosted|Signup|BillingState|OrgLifecycle|OrgHandlers" -count=1`

When entitlement code is touched, additionally run:

3. `go test ./internal/license/entitlements/... -count=1`

When provisioner is touched, additionally run:

4. `go test ./internal/hosted/... -count=1`

When metrics are touched, additionally run:

5. `go test ./internal/metrics/... -count=1`

Milestone boundary baselines (HW-05, HW-08):

6. `go build ./... && go test ./internal/api/... ./internal/hosted/... ./internal/license/entitlements/... ./internal/metrics/... -count=1`

## Execution Packets

### HW-00: Scope Freeze + Hosted Threat Model + Boundary Definition (Docs-Only)

Objective:
- Freeze the hosted readiness gap baseline, threat model, trust boundaries, and packet gates.

Scope:
- `docs/architecture/hosted-readiness-plan-2026-02.md`
- `docs/architecture/hosted-readiness-progress-2026-02.md`

Implementation checklist:
1. Audit current hosted-related code and document gaps.
2. Define hosted threat model: trust boundaries, attack surface, data flow.
3. Document W4 RBAC dependency and hosted-mode prerequisite chain.
4. Freeze packet boundaries and dependency gates.
5. Record definition-of-done contracts.

Required tests:
1. `go build ./...`

Exit criteria:
- Gap baseline, threat model, and packet gates are reviewer-approved.

### HW-01: Public Signup Endpoint + Hosted Mode Gate (Backend — API)

Objective:
- Implement a public signup endpoint gated behind hosted mode, with input validation and abuse-resistant rate limiting.

Scope (max 5 files):
1. `internal/api/hosted_signup_handlers.go` (new)
2. `internal/api/hosted_signup_handlers_test.go` (new)
3. `internal/api/router_routes_hosted.go` (new — route registration)
4. `internal/api/router.go` (wire hosted handlers + hosted mode check)

Implementation checklist:
1. Add `PULSE_HOSTED_MODE` env var detection to router config (simple `os.Getenv` check, consistent with `PULSE_MULTI_TENANT_ENABLED` pattern).
2. Create `POST /api/public/signup` endpoint with JSON payload: `email`, `password`, `org_name`.
3. Validate input: email format, password minimum strength (8+ chars, not in common list), org_name length (3-64 chars, path-safe).
4. Apply signup-specific rate limiter: 5 signups per hour per IP (stricter than general API limit).
5. On success: call provisioner (HW-02) or inline provisioning if HW-02 not yet available — create org + admin user + return org ID and session token.
6. Gate endpoint: return 404 if `PULSE_HOSTED_MODE` is not enabled.
7. Add handler tests: success path, validation failures, rate limit enforcement, hosted mode gate.

Required tests:
1. `go test ./internal/api/... -run "Hosted|Signup" -count=1`
2. `go build ./...`

Exit criteria:
- Public signup creates a trial tenant with validated input behind hosted mode gate.

### HW-02: Tenant Provisioning Service Layer (Backend — New Package)

Objective:
- Create an orchestration layer that atomically provisions a new hosted tenant (org + admin user + trial entitlement).

Scope (max 3 files):
1. `internal/hosted/provisioner.go` (new)
2. `internal/hosted/provisioner_test.go` (new)

Implementation checklist:
1. Define `Provisioner` struct with dependencies: `MultiTenantPersistence`, `AuthManager`, `LicenseService`.
2. Implement `ProvisionTenant(ctx, email, password, orgName) (ProvisionResult, error)`.
3. Orchestration steps: validate → check email uniqueness → create org dir → create admin user in org → assign trial entitlement state → emit audit log entry.
4. Make provisioning idempotent: if org with same owner email exists, return existing org (no duplicate creation).
5. Return `ProvisionResult` with OrgID, UserID, and status.
6. Handle partial failure: if user creation fails after org creation, clean up org dir.
7. Add unit tests with mock dependencies.

Required tests:
1. `go test ./internal/hosted/... -count=1`
2. `go build ./...`

Exit criteria:
- Provisioner atomically creates tenant with rollback on partial failure.

### HW-03: Billing-State Integration Seam: DatabaseSource (Backend — Entitlements)

Objective:
- Implement `DatabaseSource` fulfilling the `EntitlementSource` interface for SaaS/hosted entitlement lookup, with fail-open bounded staleness.

Scope (max 3 files):
1. `internal/license/entitlements/database_source.go` (new)
2. `internal/license/entitlements/database_source_test.go` (new)
3. `internal/license/entitlements/billing_store.go` (new — store interface)

Implementation checklist:
1. Define `BillingStore` interface: `GetBillingState(orgID) (*BillingState, error)` where `BillingState` contains capabilities, limits, meters_enabled, plan_version, subscription_state.
2. Implement `DatabaseSource` struct wrapping `BillingStore` + in-memory cache with configurable TTL.
3. `DatabaseSource` implements all 5 `EntitlementSource` methods by reading from cached billing state.
4. Fail-open: if `BillingStore.GetBillingState()` fails, use last cached state (bounded staleness, default 1 hour).
5. If no cached state exists and store is unavailable, return trial-equivalent defaults (fail-open, not fail-closed).
6. Add unit tests: happy path, cache hit, cache miss with store failure (fail-open), cache expiry, default behavior.

Required tests:
1. `go test ./internal/license/entitlements/... -count=1`
2. `go build ./...`

Exit criteria:
- DatabaseSource produces entitlements from a pluggable store with fail-open caching.

### HW-04: Billing-State Admin API + Org Billing Persistence (Backend — API)

Objective:
- Expose admin-only endpoints for querying and setting tenant billing state, with persistence in the org directory.

Scope (max 5 files):
1. `internal/api/billing_state_handlers.go` (new)
2. `internal/api/billing_state_handlers_test.go` (new)
3. `internal/config/billing_state.go` (new — persistence helpers)
4. `internal/api/router_routes_hosted.go` (extend with billing routes)

Implementation checklist:
1. `GET /api/admin/orgs/{id}/billing-state` — return current billing state for org.
2. `PUT /api/admin/orgs/{id}/billing-state` — set billing state (capabilities, limits, subscription_state, plan_version). Admin-only.
3. Persist billing state as `billing.json` in org directory (consistent with file-based tenant isolation).
4. Gate behind `PULSE_HOSTED_MODE` + `RequireAdmin`.
5. Validate subscription_state against known enum values.
6. Log billing state changes for audit trail.
7. Wire `BillingStore` interface from HW-03 to read from this persistence layer.
8. Add handler tests: get/set success, validation, auth gate, hosted mode gate.

Required tests:
1. `go test ./internal/api/... -run "BillingState" -count=1`
2. `go test ./internal/config/... -run "BillingState" -count=1`
3. `go build ./...`

Exit criteria:
- Admin can manually set per-tenant billing state; DatabaseSource reads from it.

### HW-05: Tenant Lifecycle Operations (Backend — API/Models)

Objective:
- Add suspend/unsuspend/soft-delete operations with safety guards and lifecycle state tracking.

Scope (max 5 files):
1. `internal/api/org_lifecycle_handlers.go` (new)
2. `internal/api/org_lifecycle_handlers_test.go` (new)
3. `internal/models/models.go` (extend Organization with Status field)
4. `internal/api/router_routes_hosted.go` (extend with lifecycle routes)

Implementation checklist:
1. Add `Status` field to `Organization` model: `active` (default), `suspended`, `pending_deletion`.
2. `POST /api/admin/orgs/{id}/suspend` — set org status to `suspended`, record reason and timestamp.
3. `POST /api/admin/orgs/{id}/unsuspend` — set org status back to `active`.
4. `DELETE /api/admin/orgs/{id}/soft-delete` — set org status to `pending_deletion` with configurable retention period (default 30 days). Does NOT immediately delete data.
5. Guard: cannot suspend/delete the default org.
6. Guard: suspended orgs reject all non-admin API requests (middleware check on org status).
7. Emit audit log entries for all lifecycle state changes.
8. Add handler tests: suspend/unsuspend/soft-delete success, default org guard, auth gate.

Required tests:
1. `go test ./internal/api/... -run "OrgLifecycle|Suspend|Unsuspend" -count=1`
2. `go build ./...`

Exit criteria:
- Tenant lifecycle operations are safe, auditable, and guard the default org.

### HW-06: Hosted Observability Metrics (Backend — Metrics)

Objective:
- Register SaaS-specific Prometheus counters for hosted operational visibility.

Scope (max 3 files):
1. `internal/metrics/hosted_metrics.go` (new)
2. `internal/metrics/hosted_metrics_test.go` (new)
3. Integration point in signup/provisioner/lifecycle handlers (instrument calls)

Implementation checklist:
1. Define Prometheus counters: `pulse_hosted_signups_total`, `pulse_hosted_provisions_total` (with `status` label: success/failure), `pulse_hosted_lifecycle_transitions_total` (with `from`/`to` labels), `pulse_hosted_active_tenants` (gauge).
2. Register metrics on init.
3. Instrument signup handler (HW-01), provisioner (HW-02), and lifecycle handlers (HW-05) to increment counters.
4. Use aggregate labels only — no per-tenant labels to avoid cardinality explosion (HW-R007).
5. Add unit tests verifying counter registration and increment behavior.

Required tests:
1. `go test ./internal/metrics/... -run "Hosted" -count=1`
2. `go build ./...`

Exit criteria:
- Hosted metrics are emitting and visible on Prometheus endpoint.

### HW-07: Hosted Operational Runbook + Security Baseline (Docs)

Objective:
- Produce the operational runbook, SLO definitions, escalation paths, and security baseline required by W6 exit criteria.

Scope:
- `docs/architecture/hosted-operational-runbook-2026-02.md` (new)
- `docs/architecture/hosted-security-baseline-2026-02.md` (new)

Implementation checklist:
1. Operational runbook:
   - Tenant provisioning procedures (manual and automated).
   - Suspend/unsuspend/delete procedures with safety checks.
   - Billing state override procedures.
   - Incident escalation paths.
   - Backup and restore per-tenant procedures.
2. SLO definitions:
   - API availability target (99.9%).
   - Provisioning latency target (< 5s p95).
   - Billing state propagation delay (< 60s).
3. Security baseline:
   - Trust boundary diagram (public internet → LB → Pulse API → tenant stores).
   - Data handling policy (encryption at rest, isolation model, retention).
   - Authentication and authorization model (with W4 RBAC dependency noted).
   - Hosted-specific attack surface: signup abuse, billing state manipulation, cross-tenant access.
4. Data handling policy:
   - Per-tenant data isolation guarantees.
   - Data retention and deletion policy.
   - Export/portability requirements.

Required tests:
1. `go build ./...` (verify no regressions)

Exit criteria:
- Runbook, SLOs, and security baseline are publishable.

### HW-08: Final Certification + Go/No-Go Verdict (Docs)

Objective:
- Certify lane completion with explicit go/no-go evidence and hosted posture decision.

Scope:
- `docs/architecture/hosted-readiness-progress-2026-02.md`

Implementation checklist:
1. Verify HW-00 through HW-07 are all `DONE/APPROVED`.
2. Execute full milestone validation commands with explicit exit codes.
3. Record W4 RBAC dependency status and impact on hosted posture.
4. Determine hosted launch posture: `GA`, `private_beta`, or `waitlist`.
5. Produce rollback runbook (per-packet revert instructions).
6. Record final verdict: `GO`, `GO_WITH_CONDITIONS`, or `NO_GO` with explicit rationale.

Required tests:
1. `go build ./... && go test ./internal/api/... ./internal/hosted/... ./internal/license/entitlements/... ./internal/metrics/... -count=1`

Exit criteria:
- Final certification approved with explicit hosted posture decision.

## Dependency Graph

```
HW-00 (scope freeze + threat model)
  │
  ├── HW-01 (public signup + hosted mode gate)
  │     │
  │     └── HW-02 (tenant provisioner)
  │           │
  │           └── HW-06 (hosted metrics — instruments provisioner)
  │
  ├── HW-03 (DatabaseSource entitlements)
  │     │
  │     └── HW-04 (billing-state admin API)
  │
  ├── HW-05 (tenant lifecycle ops)
  │     │
  │     └── HW-06 (hosted metrics — instruments lifecycle)
  │
  └── HW-07 (runbook + security baseline)

HW-08 (final certification) — depends on all
```

Critical path: HW-00 → HW-01 → HW-02 → HW-06 → HW-08

Parallel track A: HW-03 → HW-04 (can run parallel to HW-01/HW-02)
Parallel track B: HW-05 (can run parallel to HW-01/HW-02 after HW-00)
Parallel track C: HW-07 (can run parallel to all implementation packets after HW-00)

## External Dependencies

| Dependency | Owner | Status | Impact on W6 |
|---|---|---|---|
| W4 RBAC Isolation | Multi-Tenant lane | IN PROGRESS | HARD BLOCKER for production hosted. W6 can build in parallel; final cert notes dependency. |
| W0 EntitlementSource Interface | Monetization lane | COMPLETE | HW-03 implements DatabaseSource against this interface. |
| W5 Conversion Events | Conversion lane | NOT STARTED | Signup events could feed W5 instrumentation. Loose coupling only. |

## Explicitly Deferred Beyond W6 Lane

1. Full Stripe/Paddle checkout integration — manual/external billing only for v1.
2. Email verification on signup — deferred to post-MVP hosted launch.
3. Password reset / account recovery flows — deferred.
4. Tenant-aware rate limiting (per-org quotas) — IP-based is sufficient for private beta.
5. Distributed rate limiting (Redis-backed) — single-instance is sufficient initially.
6. Automated tenant infrastructure provisioning (VMs/Pods) — shared application model for v1.
7. Usage-based billing metering integration — metering pipeline exists (W0-B MON-05) but billing connection deferred.
8. Frontend hosted admin dashboard — backend APIs only for this lane.
9. Multi-region deployment support — single-region for initial hosted offering.
10. Custom domain / white-labeling — explicitly out of scope per Release Readiness Guiding Light.
