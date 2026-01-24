# Multi-Tenant Feature Documentation

## Status: Disabled by Default

This feature is gated behind a feature flag and license check. It will not affect existing users unless explicitly enabled.

---

## How to Enable

### Requirements

1. **Feature flag**: Set environment variable
   ```bash
   PULSE_MULTI_TENANT_ENABLED=true
   ```

2. **License**: Enterprise license with `multi_tenant` feature enabled

### Behavior Without Enablement

| Condition | HTTP Response | WebSocket Response |
|-----------|---------------|-------------------|
| Feature flag disabled | 501 Not Implemented | 501 Not Implemented |
| Flag enabled, no license | 402 Payment Required | 402 Payment Required |
| Flag enabled + licensed | Normal operation | Normal operation |

The "default" organization always works regardless of feature flag or license status.

---

## What's Implemented

### Tenant Isolation

| Component | Status | Details |
|-----------|--------|---------|
| State/Monitor | ✅ | Each org gets its own `Monitor` instance via `MultiTenantMonitor` |
| WebSocket | ✅ | Clients bound to tenant, broadcasts filtered by org |
| Audit Logs | ✅ | `LogAuditEventForTenant()` writes to per-org audit DB |
| Resources | ✅ | Per-tenant resource stores with `PopulateFromSnapshotForTenant()` |
| Persistence | ✅ | `MultiTenantPersistence` provides per-org config directories |

### Gating & Authorization

| Component | Status | Details |
|-----------|--------|---------|
| Feature flag | ✅ | `PULSE_MULTI_TENANT_ENABLED` env var (default: false) |
| License check | ✅ | Requires `multi_tenant` feature in Enterprise license |
| HTTP middleware | ✅ | `TenantMiddleware` extracts org ID, validates access |
| WebSocket gating | ✅ | `MultiTenantChecker` validates before upgrade |
| Token authorization | ✅ | `AuthorizationChecker.TokenCanAccessOrg()` |
| User authorization | ✅ | `AuthorizationChecker.UserCanAccessOrg()` via org membership |

### Tenant-Aware Endpoints

All user-facing data endpoints use `getTenantMonitor(ctx)`:

- `/api/state`
- `/api/charts`
- `/api/storage/{id}`
- `/api/backups`, `/api/backups/pve`, `/api/backups/pbs`
- `/api/snapshots`
- `/api/resources/*`
- `/api/metrics/*`

---

## Intentionally Global (Admin-Level)

These endpoints show system-wide data regardless of tenant context:

| Endpoint | Rationale |
|----------|-----------|
| `/api/health` | System uptime, not tenant-specific |
| `/api/scheduler/health` | Process-level scheduler status |
| `/api/diagnostics/*` | Admin diagnostics for full system |

Also global:
- `security_setup_fix.go` - Clears unauthenticated agents on default monitor

---

## Architecture

### Key Files

| File | Purpose |
|------|---------|
| `internal/api/middleware_tenant.go` | Extracts org ID, validates access, injects context |
| `internal/api/middleware_license.go` | Feature flag, license check, 501/402 responses |
| `internal/api/authorization.go` | `AuthorizationChecker` interface, token/user access checks |
| `internal/monitoring/multi_tenant_monitor.go` | Per-org monitor instances |
| `internal/config/multi_tenant.go` | Per-org persistence (config directories) |
| `internal/websocket/hub.go` | Tenant-aware client tracking, `MultiTenantChecker` |
| `pkg/server/server.go` | Wires up org loader, multi-tenant checker |

### Request Flow

```
Request
  │
  ├─► TenantMiddleware
  │     ├─► Extract org ID (header/cookie/default)
  │     ├─► Feature flag check (501 if disabled)
  │     ├─► License check (402 if unlicensed)
  │     ├─► Authorization check (403 if denied)
  │     └─► Inject org ID into context
  │
  ├─► Handler
  │     └─► getTenantMonitor(ctx) → org-specific Monitor
  │
  └─► Response (org-scoped data)
```

### Org ID Sources (Priority Order)

1. `X-Pulse-Org-ID` header (API clients/agents)
2. `pulse_org_id` cookie (browser sessions)
3. Fallback: `"default"`

---

## Data Model

### Organization

```go
type Organization struct {
    ID          string
    DisplayName string
    OwnerUserID string               // Creator/owner
    Members     []OrganizationMember // User membership
}

type OrganizationMember struct {
    UserID  string
    Role    string // "owner", "admin", "member"
    AddedAt time.Time
    AddedBy string
}
```

### API Token Binding

```go
type APITokenRecord struct {
    // ... existing fields ...
    OrgID  string   // Single org binding
    OrgIDs []string // Multi-org access (MSP tokens)
}
```

Legacy tokens (empty `OrgID`) have wildcard access during migration period.

---

## TODO / Deferred Items

### High Priority (Before GA)

- [ ] **Config deep copy**: `multi_tenant_monitor.go:59` does shallow copy; credential slices may be shared
- [ ] **Migration script**: Move existing data to `/orgs/default/` with symlinks for backward compatibility
- [ ] **UI integration**: Org switcher, org management screens

### Medium Priority

- [ ] **Per-tenant node credentials**: Load tenant-specific `nodes.enc` instead of inheriting base config
- [ ] **Org CRUD endpoints**: Create/update/delete organizations via API
- [ ] **Member management**: Add/remove users from organizations

### Low Priority / Policy Decisions

- [ ] Decide if diagnostics should be org-scoped or super-admin only
- [ ] Decide if `security_setup_fix.go` agent cleanup should be org-scoped

---

## Testing Checklist

### Unit Tests

```bash
# Tenant middleware tests
go test ./internal/api -run TestTenantMiddleware

# WebSocket multi-tenant tests
go test ./internal/websocket -run TestHandleWebSocket_MultiTenant
```

### Manual Testing

1. **Default behavior (flag disabled)**
   - Start Pulse without `PULSE_MULTI_TENANT_ENABLED`
   - Verify normal operation
   - Attempt `X-Pulse-Org-ID: test-org` header → expect 501

2. **Flag enabled, no license**
   - Set `PULSE_MULTI_TENANT_ENABLED=true`
   - No Enterprise license
   - Attempt non-default org → expect 402

3. **Full multi-tenant**
   - Enable flag + Enterprise license
   - Create org "test-a" with PVE node A
   - Create org "test-b" with PVE node B
   - Open browser tabs for each org
   - Verify each sees only their nodes
   - Verify WebSocket updates are isolated
   - Attempt header spoofing with wrong token → expect 403

### Integration Test Script

```bash
# 1. Verify default org works without flag
curl -u admin:admin http://localhost:7655/api/state
# → 200 OK

# 2. Verify non-default org blocked without flag
curl -u admin:admin -H "X-Pulse-Org-ID: test-org" http://localhost:7655/api/state
# → 501 Not Implemented

# 3. With flag enabled but no license
export PULSE_MULTI_TENANT_ENABLED=true
curl -u admin:admin -H "X-Pulse-Org-ID: test-org" http://localhost:7655/api/state
# → 402 Payment Required
```

---

## Rollout

### Verification Status

| Component | Status | Method | Notes |
|-----------|--------|--------|-------|
| **Feature Flag** | ✅ Verified | Unit Test | Flag disables/enables multi-tenant access correctly |
| **Licensing** | ✅ Verified | Unit Test | Unlicensed access blocked with 402 Payment Required |
| **Migration** | ✅ Verified | Unit Test | Legacy data moves to default org; symlinks created |
| **Isolation** | ✅ Verified | Unit Test | API State, WebSockets, and Resources respect tenant context |
| **Security** | ✅ Verified | Code Audit | API Tokens and Audit Logs enforce tenant binding |

### Readiness Checklist

- [ ] Enterprise license loaded for the orgs that will access multi-tenant features
- [ ] `PULSE_MULTI_TENANT_ENABLED=true` configured in the runtime environment
- [ ] Config migration has run on startup (verify tenant layout exists in data dir)
- [ ] Org membership loader is available for session users
- [ ] API tokens for non-default orgs are bound to the org(s)
- [ ] Per-tenant audit logging is enabled (tenant audit DBs present and writable)
- [ ] Tenant config loading uses per-org nodes and credentials (no shared secrets)

### Rollout Steps

1. Enable the feature flag in staging
2. Confirm enterprise license activation for a test org
3. Create a non-default org and bind a test API token to it
4. Validate:
   - `501`/`402` behavior for disabled/unlicensed org access
   - Success for licensed access (HTTP + WebSocket)
   - Data isolation across orgs
5. Roll out to production with monitoring for 4xx/5xx spikes

### Rollback

Disable `PULSE_MULTI_TENANT_ENABLED` to revert non-default org access (default org unaffected).

---

## Response Codes Reference

| Code | Meaning | When |
|------|---------|------|
| 200 | Success | Valid org access |
| 400 | Bad Request | Invalid org ID format |
| 402 | Payment Required | Feature enabled but not licensed |
| 403 | Forbidden | Token/user not authorized for org |
| 501 | Not Implemented | Feature flag disabled |

---

## Changelog

- **2024-01**: Initial implementation
  - Feature flag gating
  - License enforcement
  - Per-tenant state isolation
  - WebSocket tenant binding
  - Audit log isolation
  - Authorization framework
