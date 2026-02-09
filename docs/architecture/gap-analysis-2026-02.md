# Pulse v2.0 Implementation Status Report (2026-02)

**Date:** 2026-02-09
**Status:** COMPLETE (All Lanes Closed)

This document provides a factual assessment of the codebase state relative to the v2.0 architectural vision. It identifies implemented components and closure evidence across all implementation lanes.

## Executive Summary

All implementation lanes are now closed with certification evidence recorded in their progress trackers: W0 Monetization Foundation is COMPLETE (MON-01..MON-09); W1 Unified Resources is COMPLETE (URF-00..URF-08); W2 TrueNAS GA is COMPLETE (TN-00..TN-11 GO) and rollout readiness is also COMPLETE (TRR-00..TRR-07 GO); W3 Mobile is COMPLETE (Packets 00-04); W4 Multi-Tenant is COMPLETE (Productization 00-08, RBAC-00..RBAC-05, RBO-00..RBO-08); W5 Conversion is COMPLETE (CNV-01..CNV-06, CVO-00..CVO-05 GO); and W6 Hosted is COMPLETE (HW-00..HW-08, HOP-00..HOP-09, GO_WITH_CONDITIONS private_beta).

| Workstream | Status | Implementation Notes |
| :--- | :--- | :--- |
| **W0: Monetization** | **100%** | **W0-A (Enforcement):** Complete — token parsing, feature gates, `max_nodes` enforcement active. **W0-B (Advanced Primitives):** COMPLETE — B1-B6 implemented, legacy compatibility wiring complete, and final certification approved (MON-01..MON-09). |
| **W1: Unified Resources** | **100%** | **Complete:** URF-00 through URF-08 are DONE/APPROVED; first-party runtime is unified-resource-native. Compatibility bridges remain explicitly bounded. |
| **W2: TrueNAS** | **100%** | **COMPLETE:** TN-00..TN-11 DONE/APPROVED (GO verdict). Rollout readiness lane (TRR-00..TRR-07) also COMPLETE (GO reaffirmed). Plan: `truenas-ga-plan-2026-02.md`. Progress: `truenas-ga-progress-2026-02.md`, `truenas-rollout-readiness-progress-2026-02.md`. |
| **W3: Mobile App** | **100%** | **Complete:** Packets 00-04 are DONE/APPROVED (pairing, connection hardening, push+deep linking, approvals, biometrics/app lock). |
| **W4: Multi-Tenant** | **100%** | **COMPLETE:** Productization (Packets 00-08) certified. Per-tenant RBAC (RBAC-00..RBAC-05 LANE_COMPLETE). User limit enforcement (`max_users`) implemented. RBAC operations hardening (RBO-00..RBO-08 LANE_COMPLETE, GO_WITH_CONDITIONS burn-down complete). Plan: `multi-tenant-rbac-operations-plan-2026-02.md`. Progress: `multi-tenant-rbac-operations-progress-2026-02.md`. |
| **W5: Conversion** | **100%** | **COMPLETE:** CNV-01..CNV-06 DONE/APPROVED. Backend conversion event contract, frontend emission, trial lifecycle wiring, dynamic upgrade reasons, real usage in limits. Plan: `conversion-readiness-plan-2026-02.md`. Progress: `conversion-readiness-progress-2026-02.md`. |
| **W6: Hosted** | **100%** | **COMPLETE:** HW-00..HW-08 DONE/APPROVED (GO_WITH_CONDITIONS private_beta). Operations lane (HOP-00..HOP-09) LANE_COMPLETE. Tenant provisioning, per-tenant RBAC, billing-state API, lifecycle operations, metrics, SLOs, rate limiting, and runbooks are delivered. Plan: `hosted-operations-plan-2026-02.md`. Progress: `hosted-operations-progress-2026-02.md`. |

---

## Detailed Workstream Analysis

### W0: Monetization Foundation
**Goal:** Generic license entitlement system decoupled from hardcoded tiers.

#### W0-A: Basic Enforcement (COMPLETE)

*   **Release Readiness Audit (W0 - Monetization Enforcement):**
    *   **Status: COMPLETE** (Corrected 2026-02-08)
    *   **Evidence:** Token parsing, feature gates, and `max_nodes` enforcement are all implemented.
        *   Claims parsing and storage:
            *   `internal/license/license.go:58` defines `Claims.MaxNodes`.
            *   `internal/license/license.go:450` stores parsed claims in `License`.
            *   `internal/license/license.go:146` stores active license in `Service.license`.
            *   `internal/api/license_handlers.go:21` stores per-org `*license.Service` in `LicenseHandlers.services`.
            *   `internal/api/router.go:269` wires `LicenseHandlers` into middleware context via `SetLicenseServiceProvider`.
        *   Feature gates exist for endpoints:
            *   `internal/api/license_handlers.go:353` defines `RequireLicenseFeature(...)`.
            *   `internal/api/router_routes_org_license.go:33` uses `RequireLicenseFeature` (audit endpoints).
            *   `internal/api/router_routes_registration.go:153` uses `RequireLicenseFeature` (agent profiles endpoint).
        *   **Node limit enforcement is ACTIVE on ALL registration paths:**
            *   `internal/api/license_node_limit.go` defines enforcement helpers:
                *   `enforceNodeLimitForConfigRegistration` - blocks new config nodes when limit reached.
                *   `enforceNodeLimitForHostReport` - blocks new host agent registrations.
                *   `enforceNodeLimitForDockerReport` - blocks new Docker agent registrations.
                *   `enforceNodeLimitForKubernetesReport` - blocks new Kubernetes agent registrations.
            *   Enforcement call sites (verified 2026-02-08):
                *   `internal/api/host_agents.go:64` - calls `enforceNodeLimitForHostReport` BEFORE `ApplyHostReport`.
                *   `internal/api/docker_agents.go:85` - calls `enforceNodeLimitForDockerReport` BEFORE `ApplyDockerReport`.
                *   `internal/api/kubernetes_agents.go:48` - calls `enforceNodeLimitForKubernetesReport` BEFORE `ApplyKubernetesReport`.
                *   `internal/api/config_node_handlers.go:427` - calls `enforceNodeLimitForConfigRegistration` (PVE).
                *   `internal/api/config_node_handlers.go:558` - calls `enforceNodeLimitForConfigRegistration` (PBS).
                *   `internal/api/config_node_handlers.go:643` - calls `enforceNodeLimitForConfigRegistration` (PMG).
                *   `internal/api/config_setup_handlers.go:1723` - calls `enforceNodeLimitForConfigRegistration` (auto-register PVE).
                *   `internal/api/config_setup_handlers.go:1777` - calls `enforceNodeLimitForConfigRegistration` (auto-register PBS).
    *   **Verdict:** Basic license enforcement is fully implemented. Token parsing, feature gating, and `max_nodes` enforcement are all active.
    *   **Implementation Checkpoint (2026-02-08):** `2ee37eed` (`api(license): enforce max_nodes limits on registration paths`).

#### W0-B: Advanced Entitlement Primitives (COMPLETE)

**Completed 2026-02-08. Plan: `monetization-foundation-plan-2026-02.md`. Progress: `monetization-foundation-progress-2026-02.md`.**

All B1-B6 primitives from the Release Readiness Guiding Light are implemented and contract-tested, including legacy compatibility wiring and final certification (MON-01 through MON-09).

| Blueprint | Requirement | Status | Evidence |
| :--- | :--- | :--- | :--- |
| **B1: Claims Schema** | `capabilities: string[]` | ✅ IMPLEMENTED | `Claims.Capabilities` + `EffectiveCapabilities()` with legacy derivation (MON-01) |
| | `limits: map[string]int64` | ✅ IMPLEMENTED | `Claims.Limits` + `EffectiveLimits()` with MaxNodes/MaxGuests derivation (MON-01) |
| | `meters_enabled: string[]` | ✅ IMPLEMENTED | `Claims.MetersEnabled` field (MON-01) |
| | `plan_version: string` | ✅ IMPLEMENTED | `Claims.PlanVersion` field (MON-01) |
| | `subscription_state: string` | ✅ IMPLEMENTED | `Claims.SubState` with 5-state enum (MON-01) |
| **B2: Canonical Evaluator** | `HasCapability(key)` with alias resolution | ✅ IMPLEMENTED | `entitlements.Evaluator.HasCapability()` with `LegacyAliases` + `DeprecatedCapabilities` (MON-02, MON-03) |
| | `GetLimit(key)` | ✅ IMPLEMENTED | `entitlements.Evaluator.GetLimit()` (MON-02) |
| | `CheckLimit(key, value)` → `allowed`/`soft_block`/`hard_block` | ✅ IMPLEMENTED | 90% soft threshold, 100% hard (MON-02) |
| | `MeterEnabled(key)` | ✅ IMPLEMENTED | `entitlements.Evaluator.MeterEnabled()` (MON-02) |
| **B3: Metering Pipeline** | Event types + windowed aggregation | ✅ IMPLEMENTED | `metering.Event`, `WindowedAggregator` with idempotency + cardinality limits (MON-05) |
| **B4: State Machine** | `subscription_state` enum + transitions | ✅ IMPLEMENTED | `subscription.StateMachine` with 9 valid transitions, 16 invalid rejected, per-state behavior, downgrade policy (MON-04) |
| **B5: Frontend Contract** | Normalized entitlement payload | ✅ IMPLEMENTED | `GET /api/license/entitlements` with capabilities, limits+usage, subscription_state, upgrade_reasons (MON-07) |
| **B6: Revocation/CRL** | CRL + fail-open | ✅ IMPLEMENTED | `revocation.CRLCache` (72h staleness), `SafeEvaluator` (panic recovery), `EnrollmentRateLimit` types (MON-06) |

**Implementation Summary (post W0-B):**
- ✅ JWT-based license validation (Ed25519 signatures)
- ✅ Basic feature gating with `HasFeature()` + evaluator delegation parity wiring (MON-08)
- ✅ Grace period handling (`DefaultGracePeriod = 7 days`)
- ✅ `max_nodes` enforcement on all registration paths
- ✅ Tier-based feature mapping via `internal/license/features.go`
- ✅ Extensible entitlement primitives (capabilities, limits, meters, plan_version, subscription_state)
- ✅ Canonical evaluator with alias resolution and soft/hard limit checks
- ✅ Subscription state machine (trial/active/grace/expired/suspended)
- ✅ Metering event pipeline with windowed aggregation and cardinality protection
- ✅ CRL revocation cache with fail-open bounded staleness
- ✅ Panic-safe evaluator wrapper (fail-open on crash)
- ✅ Normalized frontend entitlement API endpoint
- ✅ Contract parity tests: evaluator produces identical results to legacy tier logic across all tiers

**Deferred (real residuals, as of 2026-02-09):**
- SQLite metering persistence + daily aggregation
- Stripe/billing webhook handlers
- Frontend UI components consuming entitlement API
- Full evaluator wiring to all 5 gating layers (evaluator is opt-in via SetEvaluator)
- CRL fetch transport mechanism
- Enrollment rate limit runtime enforcement

**Verdict:** W0-A and W0-B are COMPLETE. Monetization foundation lane is closed with certification evidence recorded.

### W1: Unified Resource Completion
**Goal:** Single API surface for all infrastructure resources.

#### Split-Brain Audit (2026-02-08)

**Status:** `COMPLETE` (URF lane complete; runtime migration done with bounded compatibility surfaces retained)

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
1. ~~Migrate `OrganizationSharingPanel.tsx` to use unified resources for resource picker~~ **DONE** (URF-01, `061f1ebd`)
2. ~~Cut over Alerts runtime off legacy conversion hook~~ **DONE** (URF-02, `acc50cb2`)
3. ~~Cut over AI chat runtime off legacy conversion hook~~ **DONE** (URF-03, `748007bf`)
4. ~~Pass SB5 gate and delete `useResourcesAsLegacy`~~ **DONE** (URF-04 `097ed341`, URF-05 `d6f40b29`)
5. ~~Migrate AI backend contract/provider to unified path~~ **DONE** (URF-06 `7557ded8`, URF-07 `f83043a4`)
6. ~~Final certification~~ **DONE** (URF-08 `9df70e89`)

*   **Implemented:**
    *   `internal/unifiedresources`: Core structures and ingestion interfaces active.
    *   `internal/api/resources_v2.go`: Full v2 API implementation (list/get/stats/link/unlink/report-merge).
    *   `frontend-modern`: All first-party runtime resource consumers migrated to unified hooks/selectors.
*   **Bounded compatibility retained (intentional):**
    *   Legacy `/api/resources` endpoints remain as compatibility facade over unified registry data.
    *   Some AI compatibility interfaces retain `LegacyResource` typing while unified provider is primary.

### W2: TrueNAS Integration
**Goal:** First-party support for TrueNAS storage arrays.

*   **Implemented:**
    *   `internal/truenas`: Provider path upgraded from fixture-only behavior to live fetcher-backed runtime ingestion while fixture snapshots remain for deterministic contract testing (TN-04).
    *   `internal/truenas/client.go`: REST API client scaffold with auth/TLS/error handling (TN-01).
    *   `internal/config/truenas.go` + persistence wiring: encrypted TrueNAS connection model (TN-02).
    *   Setup API endpoints for add/list/delete/test are implemented and admin-gated (TN-03).
    *   Runtime registration with periodic polling is implemented (TN-05).
    *   Frontend source badge and Infrastructure filter integration are implemented (TN-06).
    *   Exhaustive backend ZFS health/error state mapping is implemented (TN-07).
    *   Frontend ZFS health tag resolution and display are implemented (TN-08).
    *   Alert and AI context compatibility for TrueNAS resources is implemented (TN-09).
    *   Integration test matrix and end-to-end validation are complete (TN-10).
    *   Final GA certification is complete with GO verdict (TN-11).
    *   Rollout readiness lane is complete: operational runbook (TRR-01), telemetry + alert thresholds (TRR-02), canary rollout controls (TRR-03), soak/failure-injection validation (TRR-04), final rollout verdict GO (TRR-05), typed error classification hardening (TRR-06), and phase-1 lifecycle integration tests (TRR-07).
    *   `PULSE_ENABLE_TRUENAS`: feature flag remains the operational rollout control.

#### TrueNAS Integration Audit (2026-02-09)

*   **Status: COMPLETE (GA Ready — GO Verdict Issued)**
*   **Evidence:**
    *   `internal/truenas/provider.go`: Runtime provider wiring is complete and uses the live client/fetcher path for ingestion; fixture snapshots remain intentionally available for deterministic test coverage.
    *   `internal/truenas/client.go`: Real HTTP client path is active in runtime polling flow.
    *   `internal/api/truenas_handlers.go` + route wiring: setup lifecycle endpoints are implemented (TN-03).
    *   `internal/monitoring/truenas_poller.go`: periodic polling, telemetry, and failure-mode handling are implemented (TN-05, TRR-02, TRR-04).
    *   `docs/architecture/truenas-ga-progress-2026-02.md`: TN-00..TN-11 all DONE/APPROVED with final GO verdict.
    *   `docs/architecture/truenas-rollout-readiness-progress-2026-02.md`: TRR-00..TRR-07 all DONE/APPROVED with GO verdict reaffirmed.
*   **Verdict:** TrueNAS integration lane is complete and GA-ready. Runtime wiring is fully implemented, certification passed, and rollout-readiness controls are complete.

### W3: Mobile App Operational
**Goal:** Companion app for approvals and "pocket" monitoring.

*   **Implemented:**
    *   **Repository:** `pulse-mobile`
    *   **Biometrics:** `authStore.ts` handles FaceID/TouchID locking.
    *   **Relay Protocol:** `relay/client.ts` implements the secure WebSocket handshake.
    *   **Approvals UI:** Screens for viewing and approving findings are built.
    *   **QR Scanner:** `QRScanner.tsx` is implemented and expects a specific JSON payload.
*   **Recently Completed:**
    *   **Packet 00:** Web UI pairing source (`7e280a50`)
    *   **Packet 01:** Connection hardening and limits (`a279b18`)
    *   **Packet 02:** Push notifications and deep linking (`993bd23`)
    *   **Packet 03:** Secure approval workflow (`822dbe4`)
    *   **Packet 04:** Biometrics and app lock (`eaa82e5`)
*   **Certification Follow-up:**
    *   Lane-level manual real-device evidence artifacts (if required by release certification policy).

#### W3 Pairing Flow Audit (2026-02-08)

| End | Status | Notes |
| :--- | :--- | :--- |
| **Mobile (Sink)** | **COMPLETE** | QR scanner, relay client, push/deep-link, approvals, and app lock packetized and approved |
| **Web UI (Source)** | **COMPLETE** | `RelaySettingsPanel.tsx` QR source implemented and lane validated through Packet 04 closeout |

**Mobile App Status: COMPLETE**
*   `QRScanner.tsx` (lines 1-97): Working `expo-camera` integration with QR barcode scanning.
*   `qrCodeParser.ts` (lines 1-59): Expects JSON payload with `relay_url`, `instance_id`, `auth_token`, plus optional identity fields.
*   `add-instance.tsx` (lines 1-204): Complete onboarding screen with QR scan and manual entry tabs.
*   `relay/client.ts` (lines 1-418): Full WebSocket relay implementation with CONNECT handshake, key exchange, E2E encryption, and identity verification.

**Web UI Status: COMPLETE** (packetized lane closeout through `eaa82e5`)
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

**Verdict:** W3 Mobile Operational lane is **COMPLETE** (all five packets DONE/APPROVED).

### W4: Multi-Tenancy
**Goal:** Secure, resource-capped isolation for multiple organizations.

*   **Implemented:**
    *   **Middleware:** `internal/api/middleware_tenant.go` successfully segregates requests by Organization ID.
    *   **Data Separation:** Monitoring (`MultiTenantMonitor`) and Resource (`SQLiteResourceStore`) layers use distinct file-based storage per tenant.
    *   **Node Limit Enforcement:** `max_nodes` limits are enforced per-tenant via `getLicenseServiceForContext()` which returns tenant-specific license services.
    *   **Per-Tenant RBAC:** `TenantRBACProvider` in `internal/api/rbac_tenant_provider.go` provisions per-org `rbac.db` instances via `SQLiteManager` (RBAC-01).
    *   **RBAC Handler Wiring:** RBAC handlers resolve tenant-specific managers from request context (RBAC-02).
    *   **User Limit Enforcement:** `max_users` is enforced on member-add paths via `enforceUserLimitForMemberAdd()` (RBAC-03).
    *   **Cross-Tenant Isolation Coverage:** RBAC and user-limit isolation test matrix is complete (RBAC-04).
    *   **RBAC Operations Hardening:** org deletion lifecycle cleanup (RBO-01), integrity verification + break-glass recovery (RBO-02), Prometheus metrics (RBO-03), load/soak benchmarks (RBO-04), final operational verdict GO_WITH_CONDITIONS (RBO-05), and condition burn-down packets (RBO-06..RBO-08).

#### Multi-Tenant Isolation Audit (2026-02-09, Updated)

**Status:** `COMPLETE` (Full Isolation)

| Layer | Status | Evidence |
| :--- | :--- | :--- |
| **Request Handling** | **COMPLETE** | `TenantMiddleware` correctly extracts OrgID and injects it into context. |
| **Monitoring Data** | **COMPLETE** | `MultiTenantMonitor` instantiates separate `Monitor` instances with distinct `DataPath`. |
| **Resource Data** | **COMPLETE** | `ResourceV2Handlers` initialize tenant-specific `SQLiteResourceStore` files. |
| **RBAC / Auth** | **COMPLETE** | `TenantRBACProvider` creates per-org `rbac.db` instances through `SQLiteManager`. Roles and assignments are scoped to OrgID, and users can hold different roles per org. |
| **Node Limit Enforcement** | **COMPLETE** | `getLicenseServiceForContext()` returns per-tenant license service; `enforceNodeLimitFor*` functions use this for tenant-aware limits. |
| **User Limit Enforcement** | **COMPLETE** | `enforceUserLimitForMemberAdd()` enforces `max_users` per tenant on member-add flows with tenant-scoped license context. |

**Key Findings:**
*   **RBAC isolation is resolved:** Users, roles, and assignments now resolve through per-tenant RBAC managers, enabling different role assignments for the same user across organizations.
*   **Node Limits ARE Enforced:** Contrary to previous assessment, `max_nodes` limits ARE enforced per-tenant. The `LicenseServiceProvider` pattern in `middleware_license.go` provides context-aware license services, which `enforceNodeLimitFor*` functions use.
*   **User limits are now enforced:** `max_users` checks are active on tenant member-add paths and covered by isolation tests.
*   **Operations hardening is complete:** lifecycle cleanup, integrity/recovery helpers, metrics, load/soak validation, and post-verdict burn-down fixes are complete with lane status `LANE_COMPLETE`.
*   **Data Safety:** Monitoring/resource storage and IAM controls are now both tenant-scoped.

**Verdict:**
Multi-tenant isolation is complete across request handling, storage, RBAC/auth, and plan-limit enforcement. W4 productization and residual RBAC lanes are closed; operations hardening is `LANE_COMPLETE` (RBO-00..RBO-08) with `GO_WITH_CONDITIONS` recommendation and burn-down follow-ups closed.


### W5: Conversion Readiness
**Goal:** In-app triggers for upsell opportunities.

*   **Implemented:**
    *   **Backend Conversion Events:** `internal/license/conversion/` package with 8 event types, `Recorder` wrapping metering aggregator, and `POST /api/conversion/events` endpoint (CNV-02, `d21b2ece`).
    *   **Frontend Event Emission:** `conversionEvents.ts` with fire-and-forget tracking. `paywall_viewed` at 3 surfaces (HistoryChart, AIIntelligence, Settings). `upgrade_clicked` on 4 upgrade links (CNV-03, `c8e00aa2`).
    *   **Trial Lifecycle Wiring:** `Service.SubscriptionState()` derives trial/active/grace/expired from claims. `EntitlementPayload` includes `TrialExpiresAt` and `TrialDaysRemaining` (CNV-04, `c5e32db2`).
    *   **Dynamic Upgrade Reasons:** `UpgradeReasonMatrix` with 10 entries covering all Pro-only features. Entitlement payload returns context-aware reasons with UTM-parameterized URLs (CNV-05, `31ba1a87`).
    *   **Real Usage in Limits:** `LimitStatus.Current` populated from actual node/guest counts via persistence (CNV-05).
*   **Deferred (incremental follow-ups):**
    *   Backend emission of `trial_started`, `license_activated`, `limit_blocked` in respective handlers.
    *   `checkout_started`/`checkout_completed` (requires Stripe, out of scope per Guiding Light).
    *   Frontend `limit_warning_shown` emission (needs entitlements store).

#### Conversion Audit (2026-02-09, Updated)

**Status:** `COMPLETE` (Lane certified, CNV-01..CNV-06 DONE/APPROVED)

| Area | Status | Evidence |
| :--- | :--- | :--- |
| **Backend Events** | **COMPLETE** | `internal/license/conversion/` — 8 event types, idempotent recorder, HTTP ingestion endpoint. |
| **Frontend Events** | **COMPLETE** | `conversionEvents.ts` — paywall_viewed at 3 surfaces, upgrade_clicked at 4 links, 60s idempotency dedup. |
| **Trial Logic** | **COMPLETE** | `Service.SubscriptionState()` wired to subscription state machine. Entitlement payload includes trial countdown fields. |
| **Upgrade Reasons** | **COMPLETE** | 10-entry `UpgradeReasonMatrix` with UTM URLs. Entitlement payload returns dynamic reasons per tier. |
| **Usage Awareness** | **COMPLETE** | `LimitStatus.Current` uses real node/guest counts from persistence. |

**Verdict:**
Conversion instrumentation is implemented end-to-end. Event contracts are defined for all Guiding Light events. Frontend emits at existing paywall surfaces. Entitlement payload is complete with dynamic upgrade reasons and real usage data. Incremental emission wiring for backend-emitted events is low-risk follow-up work.

### W6: Hosted Readiness (SaaS)
**Goal:** Automated tenant provisioning and lifecycle management.

*   **Implemented:**
    *   Public signup endpoint with hosted-mode gate and rate limiting (HW-01).
    *   Tenant provisioning service layer with idempotency and rollback (HW-02).
    *   Entitlement `DatabaseSource` billing integration seam (HW-03).
    *   Billing-state admin API with per-org billing persistence (HW-04).
    *   Tenant lifecycle operations (suspend/unsuspend/soft-delete) with audit trails (HW-05).
    *   Hosted observability metrics package and instrumentation lane closeout (HW-06 + HOP-03).
    *   Operational runbook, security baseline, and SLO definitions (HW-07 + HOP-04).
    *   Final certification complete with `GO_WITH_CONDITIONS (private_beta)` (HW-08).
    *   Hosted operations lane complete: rollout policy (HOP-01), lifecycle safety drills (HOP-02), billing-state controls + metrics wiring (HOP-03), SLO/alert tuning + incident playbooks (HOP-04), final operational verdict `GO_WITH_CONDITIONS` (HOP-05), suspended-org enforcement middleware (HOP-06), pending-deletion reaper (HOP-07), tenant-aware rate limiting (HOP-08), and follow-up verdict checkpoint (HOP-09).

#### Provisioning Audit (2026-02-09)

**Status:** `COMPLETE` (Private Beta Ready)

| Feature | Status | Implementation Details |
| :--- | :--- | :--- |
| **Provisioning API** | **COMPLETE** | `POST /api/public/signup` is implemented with hosted-mode 404 gate and dedicated signup rate limiting (HW-01). |
| **Database Automation** | **COMPLETE** | Provisioning service orchestrates org creation with idempotency and rollback, backed by per-org file isolation and billing-state persistence (HW-02/HW-04). |
| **Lifecycle Management** | **COMPLETE** | Admin lifecycle endpoints (`suspend`, `unsuspend`, `soft-delete`) plus safety drills and RBAC cleanup are implemented (HW-05, HOP-02). |
| **Host Provisioning** | **COMPLETE (Private Beta Scope)** | Tenant provisioning, entitlement/billing-state wiring, metrics, rollout controls, suspended-org protection, reaper automation, and tenant-aware rate limiting are complete for shared-app hosted private beta under `PULSE_HOSTED_MODE` (HW-08, HOP-09). |

**Key Findings:**
*   **Architecture:** Pulse uses a "Shared Application, Independent Storage" model. Each tenant gets a directory, not a database schema.
*   **Hosted Control Plane:** Public signup, provisioning orchestration, billing-state administration, lifecycle operations, and hosted metrics are implemented and validated in packetized lanes.
*   **Operational Readiness:** Rollout policy, alert/SLO tuning, and incident runbooks are complete; `PULSE_HOSTED_MODE` remains the explicit safety gate.
*   **Verdict:** Hosted lane is private-beta ready with `GO_WITH_CONDITIONS` posture and documented GA-upgrade follow-ups.

---

## Technical Debt & Cleanup

*   **Legacy Code:** Legacy tier maps remain for backward compatibility; evaluator parity wiring is complete. Future deprecation timing should align with commercial migration policy and customer-token transition windows.
*   **Frontend Consistency:** First-party runtime is unified-resource-native; remaining legacy compatibility is intentionally bounded to compatibility APIs/contracts.
