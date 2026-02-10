# Entitlement Matrix (Canonical)

This document is the canonical, single source of truth for feature entitlements by tier.

If this document disagrees with runtime enforcement, update this document *and* fix the code to match
`internal/license/features.go` (tier -> capability keys). Keep them aligned.

## Source Of Truth

- Capability keys and tier mapping: `internal/license/features.go` (see `TierFeatures`, starting at line 52).
- Display names: `internal/license/features.go` (see `GetFeatureDisplayName`, starting at line 210).
- Runtime checks: primarily `license.Service.HasFeature()` / `RequireFeature()` and API middleware wrappers such as
  `internal/api/license_handlers.go:RequireLicenseFeature`.

## Tiers

Pulse tiers in code:
- `free`
- `pro`
- `pro_annual`
- `lifetime`
- `msp`
- `enterprise`

## Matrix

Legend:
- Included in tier: `Y` / `N` (derived from `TierFeatures` in `internal/license/features.go`).
- Feature definition line: where the capability key is declared in `internal/license/features.go`.

| Constant | Key | Display Name | Feature Line | free | pro | pro_annual | lifetime | msp | enterprise | Primary Gating Mechanism / Notes |
|---|---|---|---:|:---:|:---:|:---:|:---:|:---:|:---:|---|
| `FeatureAIPatrol` | `ai_patrol` | Pulse Patrol (Background Health Checks) | 10 | Y | Y | Y | Y | Y | Y | Patrol itself is available on Free (BYOK). Higher autonomy outcomes and fix execution are separately gated; auto-fix and some mutation endpoints require `ai_autofix` (see API routing in `internal/api/router_routes_ai_relay.go`). |
| `FeatureAIAlerts` | `ai_alerts` | Alert Analysis | 11 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., ai_alerts, ...)` (for example `/api/ai/investigate-alert` in `internal/api/router_routes_ai_relay.go`). |
| `FeatureAIAutoFix` | `ai_autofix` | Pulse Patrol Auto-Fix | 12 | N | Y | Y | Y | Y | Y | Required for fix execution and higher-autonomy actions; enforced via `RequireLicenseFeature` in AI routes (for example `/api/ai/findings/.../reapprove`). |
| `FeatureKubernetesAI` | `kubernetes_ai` | Kubernetes Analysis | 13 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., kubernetes_ai, ...)` (for example `/api/ai/kubernetes/analyze`). |
| `FeatureAgentProfiles` | `agent_profiles` | Centralized Agent Profiles | 16 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., agent_profiles, ...)` (for example `/api/admin/profiles/` in `internal/api/router_routes_registration.go`). |
| `FeatureUpdateAlerts` | `update_alerts` | Update Alerts (Container/Package Updates) | 19 | Y | Y | Y | Y | Y | Y | Capability exists and is exposed on license status; feature-specific enforcement is implementation-dependent. Included in Free tier per `TierFeatures[TierFree]`. |
| `FeatureRBAC` | `rbac` | Role-Based Access Control (RBAC) | 22 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., rbac, ...)` for RBAC endpoints (see `internal/api/router_routes_org_license.go`). |
| `FeatureAuditLogging` | `audit_logging` | Enterprise Audit Logging | 23 | N | Y | Y | Y | Y | Y | API route gating for audit query/verify/export endpoints via `RequireLicenseFeature(..., audit_logging, ...)` (see `internal/api/router_routes_org_license.go`). |
| `FeatureSSO` | `sso` | Basic SSO (OIDC) | 24 | Y | Y | Y | Y | Y | Y | Basic SSO (OIDC) is included in Free tier. Advanced SSO functionality is separately paid via `advanced_sso`. |
| `FeatureAdvancedSSO` | `advanced_sso` | Advanced SSO (SAML/Multi-Provider) | 25 | N | Y | Y | Y | Y | Y | Used to gate advanced SSO capabilities such as SAML and multi-provider flows. Frontend currently uses `advanced_sso` to show Advanced SSO UI (see `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`). |
| `FeatureAdvancedReporting` | `advanced_reporting` | Advanced Infrastructure Reporting (PDF/CSV) | 26 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., advanced_reporting, ...)` for report generation endpoints (see `internal/api/router_routes_org_license.go`). |
| `FeatureLongTermMetrics` | `long_term_metrics` | 90-Day Metric History | 27 | N | Y | Y | Y | Y | Y | Used to gate long-range history queries; for example, history durations beyond 7 days are blocked without `long_term_metrics` (see `internal/api/router.go` around the history handler). |
| `FeatureRelay` | `relay` | Remote Access (Mobile Relay) | 30 | N | Y | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., relay, ...)` for relay settings endpoints (see `internal/api/router_routes_ai_relay.go`). |
| `FeatureMultiUser` | `multi_user` | Multi-User Mode | 33 | N | N | N | N | N | Y | Capability key exists for Enterprise-only multi-user mode; current API surface may additionally rely on `rbac` for day-to-day role/user operations. |
| `FeatureWhiteLabel` | `white_label` | White-Label Branding | 34 | N | N | N | N | N | Y | Capability key exists; marked as not implemented in `internal/license/features.go`. |
| `FeatureMultiTenant` | `multi_tenant` | Multi-Tenant Mode | 35 | N | N | N | N | N | Y | Requires both a feature flag (`PULSE_MULTI_TENANT_ENABLED=true`) and the `multi_tenant` capability for non-default orgs (see `internal/api/middleware_license.go`). |
| `FeatureUnlimited` | `unlimited` | Unlimited Instances | 36 | N | N | N | N | Y | Y | Used for volume/instance limit removal (MSP/Enterprise); enforcement is limit-check dependent. |

