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

## Section 4: Incident Response Playbooks

### P1: Cross-tenant data leak

**Detection**: User report, anomalous cross-org API access patterns, or RBAC isolation test failure.

**Response SLA**: Acknowledge within 15 minutes, contain within 1 hour.

**Immediate actions**:
1. Suspend affected org(s) via `POST /api/admin/orgs/{id}/suspend` with reason `security_incident`.
2. Freeze all admin API access except read-only operations.
3. Notify security response owner immediately (out-of-band communication).
4. Preserve all logs and audit entries — do NOT rotate or clean up during incident.

**Investigation**:
1. Verify TenantRBACProvider isolation — confirm per-org SQLite database boundaries.
2. Check for shared-state leaks in monitoring, metrics, or config paths.
3. Review API access logs for cross-org resource access patterns.
4. Verify `data/orgs/<org-id>/` directory isolation.

**Resolution**:
1. Patch isolation gap and deploy fix.
2. Unsuspend affected orgs only after fix is verified.
3. Notify affected tenants of data exposure scope.
4. Post-incident review within 48 hours.

### P2: Billing state inconsistency

**Detection**: Tenant reports unexpected feature gating, or admin observes billing/entitlement mismatch.

**Response SLA**: Acknowledge within 1 hour, resolve within 4 hours.

**Immediate actions**:
1. Read current billing state: `GET /api/admin/orgs/{id}/billing-state`.
2. Compare with expected entitlement behavior in the application.
3. Check `DatabaseSource` cache age — TTL is 1 hour.

**Investigation**:
1. Verify `billing.json` file contents in `data/orgs/<org-id>/billing.json`.
2. Check for file corruption or write failures in billing store.
3. Review recent billing state PUT audit log entries.

**Resolution**:
1. Apply manual billing override: `PUT /api/admin/orgs/{id}/billing-state` with correct state.
2. Force cache refresh by restarting Pulse process if TTL wait is unacceptable.
3. Verify corrected behavior from tenant perspective.

### P3: Provisioning failures

**Detection**: `pulse_hosted_provisions_total{status="failure"}` counter increasing, or user signup returning 500.

**Response SLA**: Acknowledge within 2 hours, resolve within 8 hours.

**Immediate actions**:
1. Check available disk space on hosted data volume.
2. Verify `data/orgs/` directory exists and is writable.

**Investigation**:
1. Check RBAC database lock/contention in `TenantRBACProvider`.
2. Review Pulse process logs for `tenant_init_failed` or `create_failed` error codes.
3. Verify file system permissions on the data directory.
4. Check for hitting OS file descriptor limits.

**Resolution**:
1. Resolve root cause (disk space, permissions, locks).
2. Retry provisioning only after root cause is confirmed fixed.
3. If partial provisioning occurred, verify rollback cleaned up orphan directories.

### P4: Signup abuse

**Detection**: `rate(pulse_hosted_signups_total[5m]) > 2` alert, or abnormal IP patterns in access logs.

**Response SLA**: Acknowledge within 4 hours.

**Immediate actions**:
1. Review source IP distribution in access logs.
2. Confirm abuse pattern (same IP, rotating IPs, bot signatures).

**Investigation**:
1. Check current rate limiter effectiveness (baseline: `5/hr/IP`).
2. Identify whether legitimate users are being affected.

**Resolution**:
1. Tune signup rate limiter settings if needed.
2. Add temporary IP blocking at load balancer for confirmed abusers.
3. Consider CAPTCHA or email verification for future mitigation.
4. Preserve abuse telemetry for rate-limit policy tuning.

## Section 5: SLO Definitions and Alert Thresholds

### SLO targets

| SLO | Target | Measurement window | Alert threshold |
|-----|--------|-------------------|-----------------|
| Admin API availability | 99.9% | Rolling 7d | < 99.5% over 1h triggers P2 |
| Public signup availability | 99.5% | Rolling 7d | < 99.0% over 1h triggers P3 |
| Provisioning latency (P95) | < 2s | Rolling 24h | > 5s over 15min triggers P3 |
| Billing propagation delay | < 1 hour | Per-event | > 2h triggers P2 |
| Lifecycle transition latency (P95) | < 500ms | Rolling 24h | > 2s over 15min triggers P3 |

### Prometheus alert queries (private beta)

**Signup rate anomaly** (possible abuse):
```
rate(pulse_hosted_signups_total[5m]) > 2
```
Action: Review source IPs. If bot pattern confirmed, trigger P4 signup abuse playbook.

**Provisioning failure rate**:
```
rate(pulse_hosted_provisions_total{status="failure"}[15m]) / rate(pulse_hosted_provisions_total[15m]) > 0.1
```
Action: If > 10% failure rate sustained 15min, trigger P3 provisioning failure playbook.

**Active tenant count regression**:
```
delta(pulse_hosted_active_tenants[1h]) < -5
```
Action: Investigate tenant data loss or accidental bulk deletion. Trigger P1 if confirmed.

**Lifecycle transition volume anomaly**:
```
rate(pulse_hosted_lifecycle_transitions_total{to_status="suspended"}[1h]) > 5
```
Action: Review if automated suspension is occurring. Verify operator intent.

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

### W4 RBAC isolation status

- **RESOLVED** (2026-02) — W4 RBAC lane LANE_COMPLETE.
- TenantRBACProvider implements per-org SQLite databases with lazy-loading.
- Default-org backward compatibility preserved for self-hosted deployments.
- 34 isolation tests pass, including 9 cross-tenant isolation assertions.
- Reference: `docs/architecture/multi-tenant-rbac-user-limits-progress-2026-02.md`

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

- W4 RBAC per-tenant isolation is resolved (TenantRBACProvider with per-org SQLite). SSO handlers still use global `auth.GetManager()` — flagged for future work but not a blocker for private beta.
- Suspended-org enforcement middleware is deferred pending full per-request org resolution path from W4.
- Signup flow does not implement password hashing directly; credential handling is delegated to existing auth system integration.
- `ListOrganizations` owner-email uniqueness scan is `O(n)`; acceptable for private beta, not long-term scale.
- No background reaper exists for soft-deleted orgs (`pending_deletion` persists).
- No Stripe/payment integration exists yet; billing operations are manual/admin override driven.

## Section 9: Hosted Mode Rollout Policy

### Environment gate

- **Guard variable**: `PULSE_HOSTED_MODE` (environment variable)
- **Values**: `"true"` = enabled, anything else = disabled
- **Read at**: Router initialization (`internal/api/router.go`, `os.Getenv("PULSE_HOSTED_MODE") == "true"`)
- **Enforcement**: Per-handler first-operation check; returns HTTP 404 when disabled

### Gated endpoints

| Endpoint | Handler | Auth |
|----------|---------|------|
| `POST /api/public/signup` | HostedSignupHandlers | Unauthenticated (rate-limited) |
| `GET /api/admin/orgs/{id}/billing-state` | BillingStateHandlers | Admin + Scope |
| `PUT /api/admin/orgs/{id}/billing-state` | BillingStateHandlers | Admin + Scope |
| `POST /api/admin/orgs/{id}/suspend` | OrgLifecycleHandlers | Admin |
| `POST /api/admin/orgs/{id}/unsuspend` | OrgLifecycleHandlers | Admin |
| `POST /api/admin/orgs/{id}/soft-delete` | OrgLifecycleHandlers | Admin |

### Rollout stages

| Stage | `PULSE_HOSTED_MODE` | Audience | Criteria to advance |
|-------|---------------------|----------|---------------------|
| Development | `true` (dev only) | Engineering team | All HOP packets DONE |
| Private beta | `true` (staging + prod) | Trusted tenants (invite-only) | HOP-05 GO or GO_WITH_CONDITIONS |
| Public beta | `true` (prod) | Open signup | Stripe integration, suspended-org middleware, load testing |
| GA | `true` (prod, default) | All users | All GA upgrade conditions met |

### Enable procedure

1. Verify all prerequisite conditions for the target stage are met.
2. Set `PULSE_HOSTED_MODE=true` in the target environment's configuration.
3. Restart the Pulse process to pick up the environment variable.
4. Verify hosted endpoints respond (not 404) by hitting `GET /api/admin/orgs/default/billing-state` with admin credentials.
5. Monitor `pulse_hosted_signups_total` and `pulse_hosted_provisions_total` metrics for initial traffic.

### Disable procedure (emergency)

1. Remove or set `PULSE_HOSTED_MODE=false` in the target environment.
2. Restart the Pulse process.
3. Verify all hosted endpoints return 404.
4. Existing tenants retain their data — disabling the gate only prevents new hosted operations.
5. File an incident report if disable was triggered by a security or stability event.

### Private beta controls

- **Tenant limiting**: No technical enforcement beyond signup rate limit (`5/hr/IP`). Operational control via invite-only distribution of signup URL.
- **Tenant cap**: Monitor `pulse_hosted_active_tenants` gauge. Manual intervention if count exceeds operational comfort threshold.
- **Rollback**: Disable `PULSE_HOSTED_MODE` to immediately halt all hosted operations without data loss.
