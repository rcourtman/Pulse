# Multi-Tenant Operational Runbook

Status: Active
Owner: Operations
Last Updated: 2026-02-08

## Purpose

This runbook defines the production hardening and rollback process for multi-tenant operations in Pulse.

## 1. Staged Rollout Strategy

### Stage Matrix

| Stage | Scope | Entry Criteria | Owner Checkpoints | Success Metrics | Rollback Trigger |
|---|---|---|---|---|---|
| Stage 0: Development/Testing | Development and test environments only | `PULSE_MULTI_TENANT_ENABLED=true` in dev/test; all required backend and frontend tests green | Engineering owner verifies gate behavior (`501`/`402`/`403`) and UI gating behavior before internal exposure | 100% pass on targeted multi-tenant tests; no known P0 or P1 regressions | Any failing multi-tenant gate test, authz bypass, or UI leakage in single-tenant mode |
| Stage 1: Internal Pilot | Single internal organization | Stage 0 complete; pilot org selected; monitoring dashboards and log queries active | Product + engineering checkpoint at 24h and 48h | Stable API error profile, no cross-org access violations, no tenant data leakage, no critical incidents for 48h | Any confirmed cross-org leakage, sustained elevated `5xx`, or inability to recover quickly via kill switch |
| Stage 2: Limited GA | Up to 5 organizations | Stage 1 success for full 48h; support/on-call briefed on rollback procedure | Daily owner review for 1 week; support incident review | Error/latency within baseline bands, no unresolved Sev-1/Sev-2 tenant incidents, successful operator verification checks for 1 week | Repeated authz failures, materially elevated `403`/`5xx`, tenant isolation uncertainty, or rollback drill failure |
| Stage 3: General Availability | All customers | Stage 2 stable for 1 week with no blocking incidents | Weekly operational review with continuous monitoring and alert response | Sustained stability under full tenant load, no tenant isolation incidents, acceptable request/error/latency SLOs | Any production incident that threatens tenant isolation or broad service reliability |

### Stage Exit Rule

Do not advance stages unless all prior stage success metrics are met and no unresolved high-severity risks remain.

## 2. Kill Switch Operation

### Kill Switch

`PULSE_MULTI_TENANT_ENABLED=false`

### Immediate System Behavior After Flip + Restart

When the kill switch is set to `false` and Pulse is restarted:

1. All non-default org API requests immediately return `501 Not Implemented`.
2. `IsMultiTenantEnabled()` in `internal/api/middleware_license.go` reads the `multiTenantEnabled` variable and returns `false`.
3. Every org handler calls `requireMultiTenantGate()` first, which checks `IsMultiTenantEnabled()` and `hasMultiTenantFeatureForContext()`.
4. Frontend `isMultiTenantEnabled()` resolves to `false`; `OrgSwitcher` is hidden, org settings tabs are removed, and org panels render: `This feature is not available.`
5. Default org (`default`) continues working normally for backward compatibility.
6. WebSocket hub `MultiTenantChecker` denies non-default org tenant access; tenant-scoped behavior falls back to global/default-org behavior.
7. No data is deleted; org data remains on disk under `<dataDir>/orgs/<orgId>/`.

### Restart Procedure

1. Set `PULSE_MULTI_TENANT_ENABLED=false`.
2. Restart Pulse process.
3. Verify all `/api/orgs/*` endpoints return `501`.
4. Verify frontend has no org switcher and no org settings tabs.

## 3. Rollback Steps

### Rollback Procedure

1. Set `PULSE_MULTI_TENANT_ENABLED=false`.
2. Restart all Pulse instances.
3. Verify default org still works (existing configs, nodes, and AI settings intact).
4. Monitor for 30 minutes.
5. Confirm no `500` errors in logs.

### Expected Post-Rollback State

- All users see single-tenant UI (no org switcher, no org settings).
- All API calls to `/api/orgs/*` return `501`.
- All WebSocket broadcasts are global (no tenant scoping).
- Data in org directories is preserved but inaccessible while feature is disabled.
- No impact to default org operations.

## 4. Metrics Inventory

### HTTP Metrics (existing)

Source: `internal/api/http_metrics.go`

- `pulse_http_requests_total{method, route, status}`
  Tracks all API requests, including denied responses (`403`, `501`, `402`).
- `pulse_http_request_errors_total{method, route, status_class}`
  Tracks client and server error classes.
- `pulse_http_request_duration_seconds{method, route, status}`
  Latency histogram for API requests.

### Logging (existing)

Source: `internal/api/middleware_tenant.go`

- `log.Warn` with `org_id`, `user_id`, `reason` on unauthorized access attempts (lines 123-127).
- `log.Warn` for legacy token access to non-default orgs (lines 134-137).

### Key Monitoring Queries

- Denied access rate:
  `rate(pulse_http_requests_total{status="403"}[5m])`
- Feature-disabled hits:
  `rate(pulse_http_requests_total{status="501", route=~".*orgs.*"}[5m])`
- License-required hits:
  `rate(pulse_http_requests_total{status="402", route=~".*orgs.*"}[5m])`

## 5. Alerting Thresholds

Recommended starting thresholds:

- WARNING:
  `rate(pulse_http_requests_total{status="403"}[5m]) > 10`
- CRITICAL:
  `rate(pulse_http_requests_total{status="403"}[5m]) > 50`
- INFO:
  `rate(pulse_http_requests_total{status="501", route=~".*orgs.*"}[5m]) > 0`

Operational interpretation:

- WARNING likely indicates user confusion, permission drift, or bad client routing.
- CRITICAL may indicate brute-force probing or severe authorization misconfiguration.
- INFO indicates callers are hitting multi-tenant endpoints while feature is disabled.

## Incident Severity and Response

- P1: Data loss, security breach, or broad outage. Response: immediate acknowledgement and containment start in < 15 minutes.
- P2: Degraded functionality or single-feature outage. Response: acknowledge and begin mitigation in < 1 hour.
- P3: Non-blocking degradation or monitoring gaps. Response: triage and mitigation plan in < 4 hours.
- P4: Cosmetic or documentation-only issue. Response: address by next business day.

## 6. Kill Switch Validation Evidence

Existing coverage that validates kill-switch behavior:

- `TestOrgHandlersMultiTenantGate`
  Confirms org handlers return `501` when multi-tenant is disabled.
- Tenant middleware test suite (`internal/api/middleware_tenant*_test.go`)
  Confirms non-default org requests return `501` when feature flag is off.
- All 12 org handlers call `requireMultiTenantGate()` at the beginning of handler execution.
- Frontend `isMultiTenantEnabled()` gates org UI surfaces, including switcher and org settings panels.

## Operator Verification Checklist

Use this checklist during rollout and rollback execution:

1. Confirm `PULSE_MULTI_TENANT_ENABLED` value in runtime environment.
2. Confirm process restart completed on all target instances.
3. Confirm `/api/orgs/*` behavior matches expected stage (`200/403/402`) or rollback (`501`).
4. Confirm default org flows remain operational.
5. Confirm frontend org switcher/settings visibility matches expected mode.
6. Confirm no new `500` spikes and no unresolved security warnings in logs.
