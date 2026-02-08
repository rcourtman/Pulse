# Pulse v2.0 Implementation Status Report (2026-02)

**Date:** 2026-02-08
**Status:** In Progress

This document provides a factual assessment of the codebase state relative to the v2.0 architectural vision. It identifies implemented components, work in progress, and areas that have not yet been started.

## Executive Summary

Progress is well underway on server-side primitives and core mobile logic. Significant work remains to connect these systems (Mobile Pairing, Entitlement Enforcement) and to finalize the migration to the Unified Resource Model.

| Workstream | Status | Implementation Notes |
| :--- | :--- | :--- |
| **W0: Monetization** | **60%** | **Server:** Complete (Flexible tokens, keys). **Client:** Logic exists but enforcement hooks are pending. |
| **W1: Unified Resources** | **85%** | **Backend:** Core model V2 exists. **Frontend:** All first-party runtime surfaces now use V2/unified hooks. Legacy conversion hooks (`useResourcesAsLegacy`) remain for Alerts/AI/Storage paths pending URF-02..URF-05. |
| **W2: TrueNAS** | **10%** | **Prototype:** Fixture-based ingestion is working. Network client implementation is pending. |
| **W3: Mobile App** | **75%** | **Core:** Relay, Biometrics, Approvals are built. **Pairing:** READY — Backend API, mobile scanner, and Web UI QR generator all implemented. |
| **W4: Multi-Tenant** | **30%** | **Isolation:** Data & Monitoring isolated. **RBAC:** BROKEN (Global). **Limits:** MISSING. |
| **W5: Conversion** | **0%** | **Pending:** Conversion events and trial lifecycle logic need to be started. |
| **W6: Hosted** | **Partial** | **Backend:** Admin APIs exist using file-based isolation. **SaaS:** Public signup/billing missing. |

---

## Detailed Workstream Analysis

### W0: Monetization Foundation
**Goal:** Generic license entitlement system decoupled from hardcoded tiers.

*   **Release Readiness Audit (W0 - Monetization Enforcement, Client-Side):**
    *   **Status: PARTIAL**
    *   **Evidence:** Token parsing exists, but `max_nodes` is never enforced on registration paths.
        *   Claims parsing and storage:
            *   `internal/license/license.go:58` defines `Claims.MaxNodes`.
            *   `internal/license/license.go:450` stores parsed claims in `License`.
            *   `internal/license/license.go:146` stores active license in `Service.license`.
            *   `internal/api/license_handlers.go:21` stores per-org `*license.Service` in `LicenseHandlers.services`.
            *   `internal/api/router.go:269` wires `LicenseHandlers` into middleware context via `SetLicenseServiceProvider`.
        *   Feature gates exist for some endpoints:
            *   `internal/api/license_handlers.go:353` defines `RequireLicenseFeature(...)`.
            *   `internal/api/router_routes_org_license.go:33` uses `RequireLicenseFeature` (audit endpoints).
            *   `internal/api/router_routes_registration.go:153` uses `RequireLicenseFeature` (agent profiles endpoint).
        *   Registration flows are not license-gated:
            *   `internal/api/router_routes_registration.go:48` `/api/agents/docker/report` uses auth+scope only.
            *   `internal/api/router_routes_registration.go:49` `/api/agents/kubernetes/report` uses auth+scope only.
            *   `internal/api/router_routes_registration.go:50` `/api/agents/host/report` uses auth+scope only.
            *   `internal/api/router_routes_registration.go:108` `/api/config/nodes` POST uses admin+scope only.
            *   `internal/api/router_routes_registration.go:422` `/api/auto-register` has no auth wrapper and no license wrapper.
            *   `internal/api/host_agents.go:65` directly calls `ApplyHostReport(...)` with no license check.
            *   `internal/api/docker_agents.go:86` directly calls `ApplyDockerReport(...)` with no license check.
            *   `internal/api/kubernetes_agents.go:49` directly calls `ApplyKubernetesReport(...)` with no license check.
            *   `internal/api/config_node_handlers.go:427` appends new PVE nodes directly.
            *   `internal/api/config_node_handlers.go:555` appends new PBS nodes directly.
            *   `internal/api/config_setup_handlers.go:1723` and `internal/api/config_setup_handlers.go:1774` append auto-registered nodes directly.
        *   `max_nodes` usage is display-only:
            *   `internal/license/license.go:343` copies `Claims.MaxNodes` into status output.
            *   No call site in `internal/api` or `internal/agentexec` reads `Claims.MaxNodes` to allow/deny registration.
    *   **Verdict:** The license is parsed and stored, and feature-level gating exists for selected routes, but agent/node registration paths bypass any `max_nodes` enforcement hook. Call site is missing.

### W1: Unified Resource Completion
**Goal:** Single API surface for all infrastructure resources.

#### Split-Brain Audit (2026-02-08)

**Status:** `CONVERGING` (No first-party v1 runtime callers remain; legacy conversion hooks pending removal)

| Metric | Value |
| :--- | :--- |
| Modern (v2) Files | 5 |
| Legacy (v1) Files | 0 |
| Mixed v1+v2 Files | 0 |
| Risk Assessment | **Low** |

##### Backend Routes (Confirmed)
- **v2 (New):** `/api/v2/resources`, `/api/v2/resources/stats`, `/api/v2/resources/{id}/*`
- **v1 (Legacy):** `/api/resources`, `/api/resources/stats`, `/api/resources/{id}`
- **Note:** v1 handlers now read from v2 registry data (facade pattern) - no actual data divergence.

##### Frontend Files - Modern (v2 API / unified hooks) ✅
| File | Route |
| :--- | :--- |
| `src/hooks/useV2Workloads.ts` | `/api/v2/resources?type=vm,lxc,docker_container,pod` |
| `src/hooks/useUnifiedResources.ts` | `/api/v2/resources` (base) |
| `src/components/Infrastructure/ReportMergeModal.tsx` | `/api/v2/resources/{id}/report-merge` |
| `src/components/Settings/OrganizationSharingPanel.tsx` | `useResources()` unified hook (via WebSocket) |

##### Frontend Files - Legacy (v1 API) ✅
| File | Route | Risk |
| :--- | :--- | :--- |
| *(none — all first-party v1 runtime callers migrated)* | — | — |

##### Key Finding: apiClient.ts
The `apiClient.ts` reference to `/api/resources` on line 402 is **configuration metadata** (redirect skip list), **NOT** a hardcoded API base URL. This is **NOT CRITICAL** and is deferred to URF-08 final certification.

##### Remediation Status
1. ~~Migrate `OrganizationSharingPanel.tsx` to use unified resources for resource picker~~ **DONE** (URF-01, commit `061f1ebd`)
2. Once frontend legacy conversion hooks are removed (URF-02..URF-05), deprecate v1 routes in backend

*   **Implemented:**
    *   `internal/unifiedresources`: Core Go structures and ingestion interfaces active.
    *   `internal/api/resources_v2.go`: Full v2 API implementation with list, get, stats, link, unlink, report-merge.
    *   `internal/api/resource_handlers.go`: v1 facade that proxies to v2 registry data.
    *   **Frontend (85%):** All first-party runtime surfaces use v2/unified hooks exclusively. No remaining direct `/api/resources` callers.
*   **Recently Completed:**
    *   **OrganizationSharingPanel Migration:** Cutover from `/api/resources` to `useResources()` unified hook (URF-01, commit `061f1ebd`). Regression tests added (11 tests across OrganizationSharingPanel + ResourcePicker).
*   **Pending:**
    *   **Alerts/AI Legacy Conversion Hooks:** `useResourcesAsLegacy()` wrappers still used by Alerts and AI Chat paths (URF-02, URF-03).
    *   **SB5 Gate + Legacy Hook Deletion:** `useResourcesAsLegacy` cannot be removed until SB5 storage/backups migration completes (URF-04, URF-05).
    *   **AI Backend Contract:** AI resource context provider still typed on `LegacyResource` (URF-06, URF-07).
    *   **Route Deprecation:** Once all legacy conversion paths are removed, mark v1 routes as deprecated.

### W2: TrueNAS Integration
**Goal:** First-party support for TrueNAS storage arrays.

*   **Implemented:**
    *   `internal/truenas`: Fixture-based provider proving the ingestion interface works.
    *   `PULSE_ENABLE_TRUENAS`: Feature flag scaffolding.
*   **Pending:**
    *   **Network Client:** Implementation of the HTTP/SSH client to fetch real data from TrueNAS devices.

#### TrueNAS Integration Audit (2026-02-08)

*   **Status: MOCK ONLY**
*   **Evidence:**
    *   `internal/truenas/provider.go`: Logic is strictly fixture-based.
        *   `NewProvider` takes a `FixtureSnapshot`.
        *   `Records` method iterates over `p.fixtures.Pools` and `p.fixtures.Datasets`.
        *   No HTTP client, SSH client, or external connection logic exists in the package.
    *   `internal/truenas/fixtures.go`: Contains hardcoded static data (e.g., "truenas-main", "pool-tank").
    *   `grep` search for `net/http` or `crypto/ssh` returned 0 results in `internal/truenas`.
*   **Verdict:** The current implementation is a data-shape prototype only. No real integration with TrueNAS devices exists.

### W3: Mobile App Operational
**Goal:** Companion app for approvals and "pocket" monitoring.

*   **Implemented:**
    *   **Repository:** `pulse-mobile`
    *   **Biometrics:** `authStore.ts` handles FaceID/TouchID locking.
    *   **Relay Protocol:** `relay/client.ts` implements the secure WebSocket handshake.
    *   **Approvals UI:** Screens for viewing and approving findings are built.
    *   **QR Scanner:** `QRScanner.tsx` is implemented and expects a specific JSON payload.
*   **Recently Completed:**
    *   **Web UI Pairing:** `RelaySettingsPanel.tsx` now includes a "Pair Mobile Device" section with QR code generation (via `qrcode` library), diagnostics display, and copy-to-clipboard fallback. API client at `frontend-modern/src/api/onboarding.ts` calls `GET /api/onboarding/qr`. Commit `7e280a50`.
*   **Pending:**
    *   **Connection Hardening:** Exponential backoff, network transition handling, data saver mode.
    *   **Push Notifications:** Deep linking from push notifications to correct resource context.
    *   **Secure Approval Workflow:** Mobile approval actions with biometric re-auth.
    *   **Biometrics & App Lock:** Configurable lock on background/foreground transition.

#### W3 Pairing Flow Audit (2026-02-08)

| End | Status | Notes |
| :--- | :--- | :--- |
| **Mobile (Sink)** | **READY** | QR Scanner + Relay client fully implemented |
| **Web UI (Source)** | **READY** | `RelaySettingsPanel.tsx` generates QR from `/api/onboarding/qr` deep_link via `qrcode` library |

**Mobile App Status: READY**
*   `QRScanner.tsx` (lines 1-97): Working `expo-camera` integration with QR barcode scanning.
*   `qrCodeParser.ts` (lines 1-59): Expects JSON payload with `relay_url`, `instance_id`, `auth_token`, plus optional identity fields.
*   `add-instance.tsx` (lines 1-204): Complete onboarding screen with QR scan and manual entry tabs.
*   `relay/client.ts` (lines 1-418): Full WebSocket relay implementation with CONNECT handshake, key exchange, E2E encryption, and identity verification.

**Web UI Status: IMPLEMENTED** (commit `7e280a50`)
*   `onboarding_handlers.go` (backend): `GET /api/onboarding/qr` endpoint generates the required payload:
    ```json
    {
      "schema": "pulse-mobile-onboarding-v1",
      "instance_url": "...",
      "instance_id": "...",
      "relay": { "enabled": true, "url": "wss://...", "identity_fingerprint": "...", "identity_public_key": "..." },
      "auth_token": "...",
      "deep_link": "pulse://connect?..."
    }
    ```
*   `frontend-modern/src/api/onboarding.ts`: Typed API client (`OnboardingAPI.getQRPayload()`) for the onboarding endpoint.
*   `RelaySettingsPanel.tsx` (frontend): Now includes "Pair Mobile Device" section (shown when relay is enabled + connected):
    *   Imports `qrcode` library (pure JS, framework-agnostic) for QR generation.
    *   Calls `/api/onboarding/qr` and renders QR code from `deep_link` field via `QRCode.toDataURL()`.
    *   Displays diagnostics (warnings/errors) from the backend response.
    *   "Copy Payload" button copies the full JSON payload to clipboard for manual entry fallback.
*   Contract tests at `frontend-modern/src/components/Settings/__tests__/RelaySettingsPanel.test.ts` (3 tests, all pass).

**Verdict:** W3 Pairing is **READY**. Both ends (mobile scanner + web UI QR source) are implemented. Manual end-to-end scan verification is pending.

### W4: Multi-Tenancy
**Goal:** Secure, resource-capped isolation for multiple organizations.

*   **Implemented:**
    *   **Middleware:** `internal/api/middleware_tenant.go` successfully segregates requests by Organization ID.
    *   **Data Separation:** Monitoring (`MultiTenantMonitor`) and Resource (`SQLiteResourceStore`) layers use distinct file-based storage per tenant.
*   **Pending:**
    *   **RBAC Isolation:** Authentication and Authorization are global. Users and roles are shared across all tenants (Critical Security Gap).
    *   **Limit Enforcement:** Logic to reject resource creation when an Organization exceeds its plan limits (e.g., max nodes, max users).

#### Multi-Tenant Isolation Audit (2026-02-08)

**Status:** `PARTIAL` (Data Isolated, Auth Global)

| Layer | Status | Evidence |
| :--- | :--- | :--- |
| **Request Handling** | **COMPLETE** | `TenantMiddleware` correctly extracts OrgID and injects it into context. |
| **Monitoring Data** | **COMPLETE** | `MultiTenantMonitor` instantiates separate `Monitor` instances with distinct `DataPath`. |
| **Resource Data** | **COMPLETE** | `ResourceV2Handlers` initialize tenant-specific `SQLiteResourceStore` files. |
| **RBAC / Auth** | **BROKEN** | `pkg/auth/sqlite_manager.go` uses a single global `rbac.db`. Roles and assignments are not scoped to OrgID. |
| **Limit Enforcement** | **MISSING** | No logic in `Monitor.ApplyDockerReport` or `MultiTenantMonitor.GetMonitor` to enforce N+1 limits. |

**Key Findings:**
*   **RBAC is Global:** A user created in one organization exists globally. Roles assigned to a user apply to all organizations they can access. The system currently lacks the concept of "User X is Admin in Org A but Viewer in Org B".
*   **Missing Limits:** There are no checks to prevent a tenant from registering an infinite number of agents or nodes.
*   **Data Safety:** Actual monitoring data and resource inventories are safely isolated on disk. The risk is primarily unauthorized access via global RBAC and resource exhaustion.

**Verdict:**
Storage and processing are multi-tenant ready, but the Identity and Access Management (IAM) layer is fundamentally single-tenant. **This is a blocker for secure multi-tenancy.**


### W5: Conversion Readiness
**Goal:** In-app triggers for upsell opportunities.

*   **Implemented:**
    *   None.
*   **Pending:**
    *   **Instrumentation:** Telemetry hooks for `paywall_viewed`.
    *   **Trial Logic:** State machine for handling trial expiration and downgrades.

#### Conversion Audit (2026-02-08)

**Status:** `NOT STARTED`

| Area | Status | Evidence |
| :--- | :--- | :--- |
| **Backend Events** | **MISSING** | 0 results for `paywall`, `upgrade`, `trial`, `conversion` in `internal/`. 0 analytics SDKs in `go.mod`. |
| **Frontend Events** | **MISSING** | 0 results for tracking keywords or analytics SDKs (PostHog, Segment, etc.) in `frontend-modern`. |
| **Trial Logic** | **MISSING** | No schemas, migrations, or state machines found for subscription lifecycles. |

**Verdict:**
Zero instrumentation exists. The application is completely blind to conversion flows. There is no code to handle trial expirations or upgrade paths.

### W6: Hosted Readiness (SaaS)
**Goal:** Automated tenant provisioning and lifecycle management.

#### Provisioning Audit (2026-02-08)

**Status:** `PARTIAL` (Admin API Only)

| Feature | Status | Implementation Details |
| :--- | :--- | :--- |
| **Provisioning API** | **PARTIAL** | `POST /api/orgs` exists (internal/admin only). No public signup endpoint. |
| **Database Automation** | **AUTOMATED** | File-based isolation via `internal/config/multi_tenant.go`. `EnsureConfigDir` creates `data/orgs/<ID>`. No external DB required. |
| **Lifecycle Management** | **AUTOMATED** | `DELETE /api/orgs/{id}` removes tenant directory. |
| **Host Provisioning** | **MISSING** | No code to provision underlying infrastructure (VMs/Pods) for tenants. |

**Key Findings:**
*   **Architecture:** Pulse uses a "Shared Application, Independent Storage" model. Each tenant gets a directory, not a database schema.
*   **SaaS Gap:** The core plumbing (`CreateOrg`, `DeleteOrg`) exists, but there is no "Service Layer" to handle public sign-ups, billing, or orchestration.
*   **Verdict:** The engine supports multi-tenancy, but the car has no doors (public signup) or ignition (billing).

---

## Technical Debt & Cleanup

*   **Legacy Code:** The legacy hardcoded tier logic in `internal/license` should be deprecated in favor of the dynamic token evaluator once W0 enforcement is complete.
*   **Frontend Consistency:** All first-party runtime surfaces now use V2/unified hooks. Remaining work is removing transitional `useResourcesAsLegacy` conversion wrappers (tracked in URF-02..URF-05) and AI backend contract migration (URF-06..URF-07).
