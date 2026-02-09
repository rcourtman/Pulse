# Hosted Operational Runbook + Security Baseline (2026-02)

Status: Active  
Owner: Operations  
Last Updated: 2026-02-09

## Section 1: Provisioning Operations

### Public signup flow (`POST /api/public/signup`)

1. Client submits JSON payload:
   - `email`
   - `password`
   - `org_name`
2. API validates input format and constraints.
3. Hosted mode gate is enforced:
   - If `PULSE_HOSTED_MODE` is disabled, return `404`.
4. Signup rate limiting is enforced:
   - Current policy: `5` attempts per hour per source IP.
5. On success, provisioning returns tenant identity (`org_id`) and operator-facing success status.

### Tenant provisioning steps

Provisioning is executed as an orchestration sequence:

1. Create tenant org directory under `data/orgs/<org-id>/`.
2. Persist organization metadata (`org.json`) including owner/member linkage.
3. Assign RBAC admin role to the signup owner in tenant auth store.
4. Set initial billing subscription state to `trial`.
5. Emit provisioning audit log/event with actor and tenant identifiers.

### Idempotency behavior

Provisioning requests are idempotent for duplicate owner email:

- If the same signup email already owns an existing org, return that existing org (`status=existing`) instead of creating a duplicate tenant.

### Partial failure rollback

Provisioning rollback is mandatory on partial failure:

- If org directory and metadata are created, but auth user/RBAC assignment fails, delete the newly created org directory immediately.
- Rollback failure must be logged as a high-priority operational warning.

## Section 2: Lifecycle Operations

### Suspend org (`POST /api/admin/orgs/{id}/suspend`)

1. Accept optional `reason`.
2. Set status to `suspended`.
3. Set `suspended_at` timestamp (UTC).
4. Persist reason and emit lifecycle audit log entry (actor, old status, new status, reason).

### Unsuspend org (`POST /api/admin/orgs/{id}/unsuspend`)

1. Require current status `suspended`.
2. Set status to `active`.
3. Clear `suspended_at`.
4. Clear suspend reason.
5. Emit lifecycle audit log entry.

### Soft-delete org (`POST /api/admin/orgs/{id}/soft-delete`)

1. Accept optional `retention_days`.
2. Default `retention_days` to `30` when not provided.
3. Set status to `pending_deletion`.
4. Set `deletion_requested_at` timestamp.
5. Emit lifecycle audit log entry.

### Default org guard

The `default` org is immutable for destructive lifecycle operations:

- `default` cannot be suspended.
- `default` cannot be soft-deleted.

### Conflict detection

Lifecycle handlers return `409 Conflict` for invalid transition retries:

- `already_suspended` when suspend is requested for an already suspended org.
- `already_pending_deletion` when soft-delete is requested for an org already in pending deletion.

## Section 3: Billing Override

### Read billing state (`GET /api/admin/orgs/{id}/billing-state`)

1. Validate org ID.
2. Read persisted billing state.
3. If no state exists, return default `trial` state payload.

### Update billing state (`PUT /api/admin/orgs/{id}/billing-state`)

1. Validate org ID and request payload.
2. Validate `subscription_state` against allowed enum values.
3. Persist updated billing state.
4. Emit audit log containing before/after state diff.

### Valid `subscription_state` values

- `trial`
- `active`
- `grace`
- `expired`
- `suspended`

### Default billing state

When no persisted billing record exists for an org, runtime default is:

- `subscription_state=trial`
- `plan_version=trial`
- Empty `capabilities`, `limits`, `meters_enabled`

## Section 4: Escalation Procedures

### P1: Cross-tenant data leak

1. Immediately suspend affected org(s) using lifecycle admin endpoint.
2. Freeze sensitive admin changes until scope is known.
3. Notify security response owner immediately.
4. Preserve logs and audit trail for incident timeline.
5. Treat W4 RBAC isolation status as first triage checkpoint.

### P2: Billing state inconsistency

1. Compare effective entitlement behavior vs persisted billing payload.
2. Apply manual billing override via admin billing-state API.
3. Investigate `DatabaseSource` cache staleness/TTL behavior.
4. Confirm corrected state after cache propagation window.

### P3: Provisioning failures

1. Check available disk space on hosted data volume.
2. Verify tenant directory permissions for `data/orgs/`.
3. Check RBAC database lock/contention conditions.
4. Retry provisioning only after root cause is resolved.

### P4: Signup abuse

1. Review spike pattern by source IP and request volume.
2. Tune signup limiter settings (baseline: `5/hr/IP`).
3. Add temporary source blocking at edge/LB where needed.
4. Preserve abuse telemetry for rate-limit policy tuning.

## Section 5: SLO Definitions

- API availability:
  - `99.9%` for hosted admin endpoints.
  - `99.5%` for public signup endpoint.
- Provisioning latency:
  - P95 `< 2s` for tenant creation.
- Billing propagation delay:
  - `< 1 hour` (bounded by `DatabaseSource` cache TTL).
- Lifecycle transition latency:
  - P95 `< 500ms`.

## Section 6: Security Baseline

### Trust boundaries

`Public Internet -> Load Balancer -> Pulse API -> Per-tenant file stores`

### Auth model

- Lifecycle and billing endpoints are admin-only.
- Public signup is unauthenticated but rate-limited and hosted-mode-gated.

### Hosted mode gate

- Guard variable: `PULSE_HOSTED_MODE`.
- Hosted-only routes return `404` when hosted mode is disabled.

### Primary attack surface

- Signup abuse / bot registration pressure.
- Billing-state manipulation attempts.
- Cross-tenant access attempts (depends on W4 RBAC isolation hardening).

### W4 RBAC isolation dependency

- `HARD BLOCKER` for production hosted rollout.
- Current blocker statement: RBAC still treated as globally shared (`rbac.db`) rather than fully isolated per tenant in production threat posture.

## Section 7: Data Handling Policy

### Tenant isolation

- Tenant data is stored under `data/orgs/<org-id>/`.

### Data at rest

- Configuration persists as JSON/encrypted JSON files.
- Write path uses temp-file plus atomic rename for crash-safety semantics where implemented.

### Retention

- Soft-delete uses configurable `retention_days` (default `30`).
- Org status during retention is `pending_deletion`.

### Deletion

- No background reaper currently exists.
- `pending_deletion` orgs persist until explicit purge flow is implemented/executed.

### Export

- Tenant export flow is not implemented yet (future requirement).

### Encryption

- Sensitive config uses per-tenant `.encryption.key` material.
- Sensitive payload files include `nodes.enc` and `ai.enc`.

## Section 8: Known Limitations and Dependencies

- W4 RBAC isolation remains a production hard blocker; currently treated as global `rbac.db` in hosted security posture.
- Suspended-org enforcement middleware is deferred pending full per-request org resolution path from W4.
- Signup flow does not implement password hashing directly; credential handling is delegated to existing auth system integration.
- `ListOrganizations` owner-email uniqueness scan is `O(n)`; acceptable for private beta, not long-term scale.
- No background reaper exists for soft-deleted orgs (`pending_deletion` persists).
- No Stripe/payment integration exists yet; billing operations are manual/admin override driven.
