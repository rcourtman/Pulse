# Multi-Tenant Surface Inventory and Risk Register

Status: Active
Date: 2026-02-08
Packet: 00

## 1. API Endpoint Inventory

All endpoints are under `internal/api/org_handlers.go` and registered in `internal/api/router.go`.

| Method | Path | Handler | Auth Model | MT Gate | Notes |
|--------|------|---------|-----------|---------|-------|
| GET | `/api/orgs` | `HandleListOrgs` | RequireAuth + ScopeSettingsRead; `canAccessOrg` filter | `requireMultiTenantGate` | Lists orgs user can access (member/default/token-org) |
| POST | `/api/orgs` | `HandleCreateOrg` | RequireAuth + ScopeSettingsWrite; session-only (token forbidden) | `requireMultiTenantGate` | Creates org; requires authenticated username |
| GET | `/api/orgs/{id}` | `HandleGetOrg` | RequireAuth + ScopeSettingsRead; `canAccessOrg` | `requireMultiTenantGate` | |
| PUT | `/api/orgs/{id}` | `HandleUpdateOrg` | RequireAuth + ScopeSettingsWrite; session-only; `org.CanUserManage`; default org immutable | `requireMultiTenantGate` | |
| DELETE | `/api/orgs/{id}` | `HandleDeleteOrg` | RequireAuth + ScopeSettingsWrite; session-only; `org.CanUserManage`; default org immutable | `requireMultiTenantGate` | |
| GET | `/api/orgs/{id}/members` | `HandleListMembers` | RequireAuth + ScopeSettingsRead; `canAccessOrg` | `requireMultiTenantGate` | |
| POST | `/api/orgs/{id}/members` | `HandleInviteMember` | RequireAuth + ScopeSettingsWrite; session-only; `org.CanUserManage`; default org member mgmt blocked; owner-transfer constraints | `requireMultiTenantGate` | |
| DELETE | `/api/orgs/{id}/members/{userId}` | `HandleRemoveMember` | RequireAuth + ScopeSettingsWrite; session-only; `org.CanUserManage`; default org member mgmt blocked; owner removal blocked | `requireMultiTenantGate` | |
| GET | `/api/orgs/{id}/shares` | `HandleListShares` | RequireAuth + ScopeSettingsRead; `canAccessOrg` | `requireMultiTenantGate` | |
| GET | `/api/orgs/{id}/shares/incoming` | `HandleListIncomingShares` | RequireAuth + ScopeSettingsRead; `canAccessOrg` on target | `requireMultiTenantGate` | |
| POST | `/api/orgs/{id}/shares` | `HandleCreateShare` | RequireAuth + ScopeSettingsWrite; session-only; `sourceOrg.CanUserManage` | `requireMultiTenantGate` | |
| DELETE | `/api/orgs/{id}/shares/{shareId}` | `HandleDeleteShare` | RequireAuth + ScopeSettingsWrite; session-only; `sourceOrg.CanUserManage` | `requireMultiTenantGate` | |

### Multi-Tenant Gate Mechanism

All org endpoints use `requireMultiTenantGate` middleware which checks:
1. `IsMultiTenantEnabled` config flag
2. `hasMultiTenantFeatureForContext` license check

When either check fails, the endpoint returns a feature-disabled response before reaching the handler.

### Multi-Tenant Config Structure (`internal/config/multi_tenant.go`)

```go
type MultiTenantPersistence struct {
    baseDataDir string
    mu          sync.RWMutex
    tenants     map[string]*ConfigPersistence
}
```

- `default` org uses `baseDataDir` directly (legacy compatibility)
- Non-default orgs use `filepath.Join(baseDataDir, "orgs", orgID)`
- Org ID validation: `filepath.Base(orgID) == orgID`, non-empty, not `.`/`..`, length ≤64

## 2. Frontend Surface Inventory

| File Path | Type | Description | Gated When MT Off? | Risk |
|-----------|------|-------------|---------------------|------|
| `src/App.tsx` | Component | Loads org list via `OrgsAPI.list()` after auth; persists selected org (`setOrgID`); renders `OrgSwitcher` | No | **HIGH** |
| `src/components/OrgSwitcher.tsx` | Component | Organization selector/header label; renders org context even in single-org mode | No | **HIGH** |
| `src/components/Settings/Settings.tsx` | Component | Org tabs (overview/access/sharing/billing) with `features: ['multi_tenant']`; uses `isFeatureLocked`/`isTabLocked` to disable+badge; redirects locked tab to `system-pro` | Partially | **HIGH** |
| `src/components/Settings/settingsRouting.ts` | Route | Route/query mapping for org surfaces (`/settings/organization*`, `/settings/billing`, `tab=organization`, `tab=org`); no intrinsic MT check | Partially (guarded in Settings.tsx) | MEDIUM |
| `src/components/Settings/OrganizationOverviewPanel.tsx` | Component | Org metadata + membership panel via `OrgsAPI`; keyed by `getOrgID()` | Partially (parent-tab gated) | MEDIUM |
| `src/components/Settings/OrganizationAccessPanel.tsx` | Component | Org membership management (invite/update/remove roles) via `OrgsAPI` | Partially (parent-tab gated) | MEDIUM |
| `src/components/Settings/OrganizationSharingPanel.tsx` | Component | Cross-org sharing management (outgoing/incoming shares) via `OrgsAPI` | Partially (parent-tab gated) | MEDIUM |
| `src/components/Settings/OrganizationBillingPanel.tsx` | Component | Billing panel; includes org/member counts via `OrgsAPI` | Partially (parent-tab gated) | MEDIUM |
| `src/components/Settings/ProLicensePanel.tsx` | Component | Exposes MT capability label (`multi_tenant: 'Multi-tenant Mode'`) in feature display | No | **HIGH** |
| `src/api/orgs.ts` | API Client | Canonical org/membership/sharing API client (`/api/orgs*`); all requests use `skipOrgContext: true` | No (client-side, no gating) | **HIGH** |
| `src/utils/apiClient.ts` | Utility | Org context transport: `setOrgID`/`getOrgID`, session/cookie persistence, `X-Pulse-Org-ID` header injection, invalid-org recovery | No | **HIGH** |
| `src/stores/license.ts` | Store | Canonical MT feature helper: `hasFeature(feature)` checks `licenseStatus()?.features.includes(feature)` | Yes (when consumer uses it) | LOW |
| `src/features/storageBackupsV2/models.ts` | Utility | Contains `BackupScope.scope` value `'tenant'` (data taxonomy, not org gating) | No | LOW |

### Canonical Feature Flag Helper

- **Function:** `hasFeature(feature)` in `src/stores/license.ts`
- **Checks:** `licenseStatus()?.features.includes(feature)`
- **Consumed by:** `Settings.tsx` via `isFeatureLocked(['multi_tenant'])`
- **Gap:** No single top-level guard prevents `OrgSwitcher` and `App.tsx` org loading from running in single-tenant mode.

## 3. Realtime / Event Emission Inventory

| Source File | Function | Event Type | Org-Scoped? | Isolation Risk |
|-------------|----------|-----------|-------------|----------------|
| `websocket/hub.go` | `HandleWebSocket` | WS connect (`/ws`) | Partial | Org from `X-Pulse-Org-ID`/cookie/query/default; auth checkers present but not wired in production |
| `websocket/hub.go` | `BroadcastState` | `rawData` | **No** | Global broadcast to all clients |
| `websocket/hub.go` | `BroadcastStateToTenant` | `rawData` | Yes | Tenant-targeted |
| `websocket/hub.go` | `BroadcastAlert` | `alert` | **No** | Global broadcast |
| `websocket/hub.go` | `BroadcastAlertToTenant` | `alert` | Yes | Tenant-targeted |
| `websocket/hub.go` | `BroadcastAlertResolved` | `alertResolved` | **No** | Global broadcast |
| `websocket/hub.go` | `BroadcastAlertResolvedToTenant` | `alertResolved` | Yes | Tenant-targeted |
| `websocket/hub.go` | `BroadcastMessage` | Arbitrary typed | **No** | Global typed message broadcast |
| `monitoring/monitor.go` | `broadcastState` | `rawData` | Conditional | Uses `GetOrgID()`: tenant path → `BroadcastStateToTenant`; fallback → global |
| `monitoring/monitor_alerts.go` | `raiseAlert` | `alert` | Yes | Uses `BroadcastAlertToTenant` with monitor org |
| `monitoring/monitor_alerts.go` | `resolveAlert` | `alertResolved` | Yes | Uses `BroadcastAlertResolvedToTenant` with monitor org |
| `monitoring/monitor.go` | `escalateToAttention` | `alert` | Yes | Escalation alerts emitted tenant-scoped |
| `api/alerts.go` | `broadcastStateForContext` | `rawData` | Yes | Uses request org + `BroadcastStateToTenant` |
| `api/agent_handlers_base.go` | `broadcastAgentData` | `rawData` | **No** | Reads tenant monitor but publishes via global `BroadcastState` — **cross-tenant data fan-out** |
| `api/config_handlers.go` | `handleDiscoveryUpdate` | `discovery_update` | **No** | Global `BroadcastMessage` |
| `api/config_handlers.go` | `deleteNode` | `node_deleted` | **No** | Global `BroadcastMessage` |
| `api/config_handlers.go` | `bulkNodeUpdate` | `nodes_updated` | **No** | Global `BroadcastMessage` |
| `api/config_handlers.go` | `handleNodeAutoRegister` | `node_auto_registered`, `discovery_update` | **No** | Global `BroadcastMessage` |
| `api/system_settings.go` | `UpdateSettings` | `settingsUpdate` | **No** | Global settings broadcast |
| `api/router.go` | AI discovery progress callback | `ai_discovery_progress` | **No** | Global hub broadcast |
| `api/log_handlers.go` | `HandleStreamLogs` | SSE log stream | **No** | Subscribes to global log broadcaster; no org filter |
| `api/updates.go` | `HandleUpdateStream` | SSE update stream | **No** | Global update manager SSE |
| `api/ai_handlers.go` | `HandleExecuteStream` | SSE AI execute stream | Yes | Uses `GetAIService(r.Context())` (tenant-keyed) |
| `api/ai_handlers.go` | `HandleInvestigateAlert` | SSE AI investigation | Yes | Same tenant service resolution |
| `api/ai_handlers.go` | `HandlePatrolStream` | SSE patrol stream | Yes | Tenant AI service from request context |
| `api/ai_handler.go` | `HandleChat` | SSE chat stream | Yes | Tenant-scoped via `GetOrgID(ctx)` |
| `ai/patrol_run.go` | `SubscribeToStreamFrom` | In-process patrol events | Depends | Per `PatrolRunner` instance; isolation depends on tenant-specific runner wiring |
| `ai/patrol_findings.go` | `CreateFinding` push path | Push notification (finding) | **Unclear/weak** | Push payload has no explicit org field in relay protocol |
| `ai/patrol_findings.go` | Investigation outcome push | Push notification (fix/approval) | **Unclear/weak** | Same; no org in relay payload schema |
| `api/router.go` | `SetPushNotifyCallback` | Push dispatch to relay | **Unclear/weak** | Sends to relay; no visible org tagging |
| `discovery/service.go` | `broadcastEvent` | Discovery WS events | **No** | Uses global hub `Broadcast` |

### Key Findings

1. **WebSocket org resolution is client-advisory:** Org determined from `X-Pulse-Org-ID` header, `pulse_org_id` cookie, `org_id` query param, or default. Hub has optional org authorization checkers but they are **not wired in production**.
2. **Global broadcast functions exist and are actively used:** `BroadcastState`, `BroadcastAlert`, `BroadcastAlertResolved`, `BroadcastMessage` all fan out to every connected client regardless of org.
3. **Agent data broadcast is cross-tenant:** `broadcastAgentData` uses global `BroadcastState` even when reading from tenant-specific monitor state.
4. **Push notifications lack org tagging:** Patrol finding and investigation outcome push payloads do not include org identifiers in the relay protocol.
5. **AI services are tenant-isolated via service map:** Chat, execute, investigate, and patrol SSE streams all resolve a tenant-specific AI service from request context.
6. **Shared AI stores risk cross-contamination:** Multiple AI component stores (baseline, change detector, remediation log, etc.) may be shared across tenant services.

## 4. Background Jobs and Caches

| Component | Tenant-Isolated? | Risk |
|-----------|-------------------|------|
| AI component stores (baseline, change detector, remediation log, incident/pattern/correlation/discovery) | Unclear — may be shared across tenant services | **HIGH** |
| Discovery service event broadcast | No — global hub broadcast | MEDIUM |
| Log broadcaster | No — global subscriber list | LOW (logs are system-level) |
| Update manager SSE | No — global client list | LOW (updates are system-level) |

## 5. Risk Register

| ID | Surface | Category | Severity | Current State | Guard Strategy | Remediation Packet | Notes |
|----|---------|----------|----------|--------------|----------------|-------------------|-------|
| R01 | `App.tsx` org loading + `OrgSwitcher` | Frontend visibility | **HIGH** | Org list loaded and OrgSwitcher rendered regardless of MT flag | Add top-level gate: skip org load + hide switcher when MT disabled | Packet 01 | Non-negotiable contract #1 violation |
| R02 | `ProLicensePanel.tsx` MT label | Frontend visibility | **HIGH** | "Multi-tenant Mode" label shown in license features unconditionally | Gate display behind MT feature check | Packet 02 | Single-tenant users see MT terminology |
| R03 | `broadcastAgentData` global fan-out | Realtime isolation | **HIGH** | Agent data broadcast uses global `BroadcastState`, leaking tenant monitor data cross-org | Replace with `BroadcastStateToTenant` | Packet 05 | Cross-org data leakage vector |
| R04 | WebSocket org auth not wired | Realtime isolation | **HIGH** | Hub has org authorization hooks but they are not connected in production | Wire org auth checkers into `HandleWebSocket` | Packet 05 | Client can join any org's event stream |
| R05 | Push notifications lack org tag | Realtime isolation | **HIGH** | Patrol finding/investigation push payloads have no org identifier in relay protocol | Add org field to push relay payload; filter delivery by org membership | Packet 05 | Push events could reach wrong org's users |
| R06 | Global `BroadcastState`/`BroadcastAlert`/`BroadcastMessage` usage | Realtime isolation | **HIGH** | Multiple callers use global broadcast instead of tenant-targeted variants | Audit all callers; replace with tenant-targeted equivalents | Packet 05 | Systematic cross-org event leakage |
| R07 | Shared AI component stores | Data isolation | **HIGH** | Baseline store, change detector, remediation log, etc. may mix data across tenants | Ensure stores are per-tenant instances or keyed by org | Packet 05 | Cross-org AI analysis contamination |
| R08 | Settings org tabs partial gating | Frontend visibility | MEDIUM | Org tabs use `isFeatureLocked` which disables/badges but route still exists and redirects | Ensure clean redirect with no org-specific content shown | Packet 01 | Tabs visible but locked — leaks MT concept |
| R09 | `settingsRouting.ts` no intrinsic guard | Frontend visibility | MEDIUM | Route definitions exist without MT check; guarded only by parent component | Add route-level guards or ensure parent always blocks | Packet 01 | Defense-in-depth gap |
| R10 | Org panels (Overview/Access/Sharing/Billing) parent-only gating | Frontend visibility | MEDIUM | Panels rely solely on parent Settings tab lock | Panels should independently check MT feature as defense-in-depth | Packet 02 | Could render if directly navigated |
| R11 | `apiClient.ts` always injects `X-Pulse-Org-ID` | Context propagation | MEDIUM | Header injected on every request even in single-tenant mode | Make header injection conditional on MT enabled | Packet 04 | Unnecessary header in ST mode; spoof surface |
| R12 | Discovery events global broadcast | Realtime isolation | MEDIUM | Discovery WS events use global hub broadcast | Scope to tenant when MT enabled | Packet 05 | Discovery results visible cross-org |
| R13 | Config handler broadcasts (node CRUD, auto-register) | Realtime isolation | MEDIUM | Node lifecycle events use global `BroadcastMessage` | Scope to tenant | Packet 05 | Node operations visible cross-org |
| R14 | `api/orgs.ts` no client-side gating | Frontend visibility | MEDIUM | API client module available regardless of MT feature | Acceptable — backend gate prevents actual data access | Packet 01 | Low risk due to backend gate |
| R15 | `BackupScope.scope = 'tenant'` | Frontend visibility | LOW | Data taxonomy value; not org-membership gating | No action needed | N/A | Cosmetic; no user-facing impact |
| R16 | `license.ts` `hasFeature` helper | Frontend gating | LOW | Canonical helper exists and works; consumers must use it | Ensure all MT-gated surfaces use this helper | Packet 01 | Foundational; no issue with the helper itself |
| R17 | Log stream SSE | Realtime isolation | LOW | Global log broadcast; no org filter | System-level logs; acceptable in current model | Packet 07 | Consider org filtering if logs become tenant-scoped |
| R18 | Update stream SSE | Realtime isolation | LOW | Global update status; no org filter | System-level updates; acceptable | N/A | No action needed |

### Severity Distribution

- **HIGH:** 7 items (R01–R07) — cross-org data leakage, missing authz, unscoped broadcasts, UI contract violations
- **MEDIUM:** 7 items (R08–R14) — partial gating, defense-in-depth gaps, unnecessary context propagation
- **LOW:** 4 items (R15–R18) — cosmetic, foundational helpers, system-level streams

### Remediation Sequencing (HIGH Severity)

1. **Packet 01** (R01): Top-level frontend gate — prevents MT UI from appearing in single-tenant mode
2. **Packet 04** (R11 feeds into R04, R05): Header spoofing hardening — prevents context escalation
3. **Packet 05** (R03, R04, R05, R06, R07, R12, R13): Realtime isolation — largest HIGH cluster; requires systematic broadcast audit and AI store isolation
4. **Packet 02** (R02, R10): UX parity sweep — clean up remaining visibility leaks

### Guard Strategy Summary

Every surface has one of these guard strategies assigned:
1. **Backend gate (`requireMultiTenantGate`)** — all API endpoints (in place)
2. **Frontend feature gate (`hasFeature/isFeatureLocked`)** — settings tabs (partially in place)
3. **Top-level frontend gate** — App.tsx/OrgSwitcher (NOT in place — Packet 01)
4. **Tenant-targeted broadcast** — realtime events (partially in place — Packet 05)
5. **Org-scoped service resolution** — AI streams (in place)
6. **No action needed** — system-level streams, data taxonomy (acceptable)

## 6. Test Coverage Summary

| Test Name | Status | Coverage Area |
|-----------|--------|--------------|
| `TestOrgHandlersCRUDLifecycle` | PASS | Org CRUD operations |
| `TestOrgHandlersMemberCannotManageOrg` | PASS | Role-based access denial |
| `TestOrgHandlersTokenListAllowedButWriteForbidden` | PASS | Token auth write restrictions |
| `TestOrgHandlersMultiTenantGate` | PASS | Feature gate enforcement |
| `TestOrgHandlersNormalizesLegacyMemberRoleToViewer` | PASS | Legacy role migration |
| `TestOrgHandlersOwnershipTransfer` | PASS | Owner transfer constraints |
| `TestOrgHandlersRemoveMember` | PASS | Member removal constraints |
| `TestOrgHandlersShareLifecycle` | PASS | Share CRUD operations |
| `TestOrgHandlersCrossOrgIsolation` | PASS | Cross-org read/write denial |
| `TestOrgHandlersShareIsolationAcrossOrganizations` | PASS | Cross-org share isolation |
| `TestContract_FindingJSONSnapshot` | PASS | Finding contract shape |
| `TestContract_ApprovalJSONSnapshot` | PASS | Approval contract shape |
| `TestContract_ChatStreamEventJSONSnapshots` | PASS | Chat SSE event contracts |
| `TestContract_PushNotificationJSONSnapshots` | PASS | Push notification contracts |
| `TestContract_AlertJSONSnapshot` | PASS | Alert contract shape |
| `TestContract_AlertResourceTypeConsistency` | PASS | Alert resource types |

## 7. Endpoint Authorization Matrix (Packet 03 Evidence)

| Endpoint | Read/Write | Membership Check | Role Required | Token Allowed | Default Org Immutable | Input Validation | Test Coverage |
|----------|-----------|-----------------|--------------|--------------|----------------------|-----------------|--------------|
| GET `/api/orgs` | Read | `canAccessOrg` filter | Any member | Yes (read) | N/A | N/A | CRUDLifecycle |
| POST `/api/orgs` | Write | Creator becomes owner | N/A (creation) | No (session-only) | N/A | `isValidOrganizationID`, MaxBytesReader | CRUDLifecycle |
| GET `/api/orgs/{id}` | Read | `canAccessOrg` | Any member | Yes (read) | N/A | `loadOrganization` validates | CRUDLifecycle |
| PUT `/api/orgs/{id}` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | Yes | MaxBytesReader, displayName required | CRUDLifecycle, MemberCannotManageOrg |
| DELETE `/api/orgs/{id}` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | Yes | `loadOrganization` validates | CRUDLifecycle, MemberCannotManageOrg |
| GET `/api/orgs/{id}/members` | Read | `canAccessOrg` | Any member | Yes (read) | N/A | `loadOrganization` validates | CRUDLifecycle |
| POST `/api/orgs/{id}/members` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | Yes (member mgmt blocked) | MaxBytesReader, owner-transfer constraints | OwnershipTransfer, TokenListAllowedButWriteForbidden |
| DELETE `/api/orgs/{id}/members/{userId}` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | Yes (member mgmt blocked) | Owner removal blocked | RemoveMember |
| GET `/api/orgs/{id}/shares` | Read | `canAccessOrg` | Any member | Yes (read) | N/A | `loadOrganization` validates | ShareLifecycle |
| GET `/api/orgs/{id}/shares/incoming` | Read | `canAccessOrg` on target | Any member | Yes (read) | N/A | `loadOrganization` validates | ShareIsolationAcrossOrganizations |
| POST `/api/orgs/{id}/shares` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | N/A | MaxBytesReader, target org exists check | ShareLifecycle, ShareIsolationAcrossOrganizations |
| DELETE `/api/orgs/{id}/shares/{shareId}` | Write | `loadOrganization` | `CanUserManage` (admin/owner) | No (session-only) | N/A | Share existence check | ShareLifecycle |

### Org ID Validation (`isValidOrganizationID`)
- Rejects: empty string, `.`, `..`, path traversal (`filepath.Base(orgID) != orgID`), length > 64
- Applied in: `HandleCreateOrg`, `loadOrganization` (used by all handlers referencing `{id}`)
