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

### Isolation, Gating, and Authorization

| Component | Status | Details |
|-----------|--------|---------|
| State/Monitor isolation | Implemented | Each org gets its own `Monitor` instance via `MultiTenantMonitor` |
| WebSocket isolation | Implemented | Clients bound to tenant, broadcasts filtered by org |
| Audit log isolation | Implemented | `LogAuditEventForTenant()` writes to per-org audit DB |
| Resource isolation | Implemented | Per-tenant resource stores with `PopulateFromSnapshotForTenant()` |
| Persistence isolation | Implemented | `MultiTenantPersistence` provides per-org config directories |
| Feature flag gate | Implemented | `PULSE_MULTI_TENANT_ENABLED` env var (default: false) |
| License gate | Implemented | Requires `multi_tenant` feature in Enterprise license |
| HTTP middleware gate | Implemented | `TenantMiddleware` extracts org ID and validates access |
| WebSocket gate | Implemented | `MultiTenantChecker` validates before upgrade |
| Token authorization | Implemented | `AuthorizationChecker.TokenCanAccessOrg()` |
| User authorization | Implemented | `AuthorizationChecker.UserCanAccessOrg()` via org membership |

### Organization Management and Sharing

| Component | Status | Details |
|-----------|--------|---------|
| Organization CRUD | Implemented | Create/read/update/delete orgs via API and UI |
| Member management | Implemented | Invite/remove members, role changes, ownership transfer |
| Member roles | Implemented | `owner`, `admin`, `editor`, `viewer` (`member` alias normalizes to `viewer`) |
| Cross-org sharing | Implemented | Share resources between orgs with role-based access (`viewer`/`editor`/`admin`) |

### Frontend UX and State Isolation

| Component | Status | Details |
|-----------|--------|---------|
| OrgSwitcher | Implemented | Header dropdown for active organization selection |
| Settings panels | Implemented | Organization panels: Overview, Access, Sharing, Billing & Plan |
| Org switch state reset | Implemented | Settings panels re-fetch, org caches invalidated, AI chat session/context reset |

### Tenant-Aware Endpoints

All user-facing data endpoints use tenant context (`X-Pulse-Org-ID`, cookie, or default fallback):

- `/api/state`
- `/api/charts`
- `/api/storage/{id}`
- `/api/backups`, `/api/backups/pve`, `/api/backups/pbs`
- `/api/snapshots`
- `/api/resources/*`
- `/api/metrics/*`
- `/api/orgs`
- `/api/orgs/{id}`
- `/api/orgs/{id}/members`
- `/api/orgs/{id}/members/{userId}`
- `/api/orgs/{id}/shares`
- `/api/orgs/{id}/shares/incoming`
- `/api/orgs/{id}/shares/{shareId}`

---

## Organizations & Members

- Each organization is an isolated environment with its own monitor instance, resource state, config storage, and audit scope.
- The `default` organization always exists and is used when multi-tenant is disabled.
- `default` is intentionally immutable for org admin operations:
  - It cannot be deleted.
  - It cannot be renamed through org update APIs.
  - Member management on `default` is blocked.
- Roles:
  - `owner`: full control, can transfer ownership
  - `admin`: can manage members and shares, and update/delete org (non-default)
  - `editor`: read/write org resources, cannot manage members/shares
  - `viewer`: read-only access
- Org lifecycle:
  - Create org: API `POST /api/orgs` or UI
  - Read/update/delete org: API `/api/orgs/{id}` or UI
  - Add/update/remove members: API `/api/orgs/{id}/members*` or UI
  - Transfer ownership: set a member role to `owner` (current owner only)

---

## Resource Sharing

- Sharing allows one source org to grant a target org access to selected resources.
- A share record includes:
  - Source org
  - Target org
  - Resource type
  - Resource ID
  - Access role
- Valid resource types:
  - `vm`, `container`, `host`, `storage`, `pbs`, `pmg`
- Share access roles:
  - `viewer`, `editor`, `admin`
- Sharing endpoints:
  - Outgoing shares: `GET /api/orgs/{id}/shares`
  - Incoming shares: `GET /api/orgs/{id}/shares/incoming`
  - Create/update share: `POST /api/orgs/{id}/shares`
  - Delete share: `DELETE /api/orgs/{id}/shares/{shareId}`

---

## Frontend Behavior

- Header:
  - `OrgSwitcher` is shown when multi-tenant is enabled.
- Org switch behavior:
  - Emits `org_switched` event for cross-component reactivity.
  - Organization settings panels re-fetch data on switch.
  - AI chat state is reset on switch (messages/session/context cleared).
  - Org-scoped caches are invalidated on switch (including metadata and infrastructure summary caches).
- Settings path:
  - `Settings -> Organization`
  - Panels: `Overview`, `Access`, `Sharing`, `Billing & Plan`
- Error handling:
  - `402` maps to "Multi-tenant requires an Enterprise license"
  - `501` maps to "Multi-tenant is not enabled on this server"

---

## Storage Layout and Migration

- The **default** org uses the root data dir for backward compatibility.
- Non-default orgs store data in `/orgs/<org-id>/`.
- When multi-tenant is enabled, legacy single-tenant data is migrated into `/orgs/default/` and symlinks are created in the root data dir for compatibility.
- Organization metadata is stored in `org.json` inside each org directory.

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
| `internal/api/org_handlers.go` | Organization CRUD, member management, and sharing APIs |
| `internal/monitoring/multi_tenant_monitor.go` | Per-org monitor instances |
| `internal/config/multi_tenant.go` | Per-org persistence (config directories) |
| `internal/websocket/hub.go` | Tenant-aware client tracking, `MultiTenantChecker` |
| `pkg/server/server.go` | Wires up org loader and multi-tenant checker |
| `frontend-modern/src/components/OrgSwitcher.tsx` | Organization switcher dropdown in app header |
| `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx` | Organization Overview settings panel |
| `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx` | Organization Access settings panel |
| `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` | Organization Sharing settings panel |
| `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` | Organization Billing & Plan settings panel |
| `frontend-modern/src/stores/events.ts` | `org_switched` event bus contract for org-change reactivity |

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

```go
type Organization struct {
    ID              string
    DisplayName     string
    OwnerUserID     string               // Creator/owner
    Members         []OrganizationMember // User membership
    SharedResources []OrganizationShare  // Outgoing shares from this org
}

type OrganizationMember struct {
    UserID  string    // Member user ID
    Role    string    // "owner", "admin", "editor", "viewer"
    AddedAt time.Time
    AddedBy string
}

type OrganizationShare struct {
    ID           string
    SourceOrgID  string
    TargetOrgID  string
    ResourceType string // "vm", "container", "host", "storage", "pbs", "pmg"
    ResourceID   string
    ResourceName string
    AccessRole   string // "viewer", "editor", "admin"
    CreatedAt    time.Time
    CreatedBy    string
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

## API Reference

For full request/response examples, see the Organizations section in `docs/API.md`.

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

## Testing Checklist

### Unit Tests

```bash
# Tenant middleware tests
go test ./internal/api -run TestTenantMiddleware

# WebSocket multi-tenant tests
go test ./internal/websocket -run TestHandleWebSocket_MultiTenant
```

### Frontend (Vitest)

- Run org-related frontend tests with the Vitest suite (OrgSwitcher, org header propagation, AI chat reset behavior, and org-scoped cache behavior).

### Manual Testing

1. **Default behavior (flag disabled)**
   - Start Pulse without `PULSE_MULTI_TENANT_ENABLED`
   - Verify normal operation
   - Attempt `X-Pulse-Org-ID: test-org` header -> expect 501

2. **Flag enabled, no license**
   - Set `PULSE_MULTI_TENANT_ENABLED=true`
   - No Enterprise license
   - Attempt non-default org -> expect 402

3. **Full multi-tenant**
   - Enable flag + Enterprise license
   - Create org "test-a" with PVE node A
   - Create org "test-b" with PVE node B
   - Open browser tabs for each org
   - Verify each sees only their nodes
   - Verify WebSocket updates are isolated
   - Verify org switch resets org-scoped UI state
   - Attempt header spoofing with wrong token -> expect 403

### Integration Test Script

```bash
# 1. Verify default org works without flag
curl -u admin:admin http://localhost:7655/api/state
# -> 200 OK

# 2. Verify non-default org blocked without flag
curl -u admin:admin -H "X-Pulse-Org-ID: test-org" http://localhost:7655/api/state
# -> 501 Not Implemented

# 3. With flag enabled but no license
export PULSE_MULTI_TENANT_ENABLED=true
curl -u admin:admin -H "X-Pulse-Org-ID: test-org" http://localhost:7655/api/state
# -> 402 Payment Required
```

---

## Rollout

### Verification Status

| Component | Status | Method | Notes |
|-----------|--------|--------|-------|
| **Feature Flag** | Verified | Unit Test | Flag disables/enables multi-tenant access correctly |
| **Licensing** | Verified | Unit Test | Unlicensed access blocked with 402 Payment Required |
| **Migration** | Verified | Unit Test | Legacy data moves to default org; symlinks created |
| **Isolation** | Verified | Unit Test | API State, WebSockets, and Resources respect tenant context |
| **Security** | Verified | Code Audit | API Tokens and Audit Logs enforce tenant binding |

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

## Changelog

- **2024-01**: Initial implementation
  - Feature flag gating
  - License enforcement
  - Per-tenant state isolation
  - WebSocket tenant binding
  - Audit log isolation
  - Authorization framework
- **2026-02**: Multi-tenant productization
  - Organization CRUD via `/api/orgs/*` (API + UI)
  - Member management with `owner`/`admin`/`editor`/`viewer`
  - Cross-org resource sharing
  - Organization settings panels (Overview, Access, Sharing, Billing & Plan)
  - Frontend org-switch state isolation (panel refresh, cache invalidation, AI chat reset)
  - Frontend org-aware cache scoping and invalidation on switch
  - License features endpoint consumption in org/billing UI flow
