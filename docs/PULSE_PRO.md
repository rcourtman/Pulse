# Pulse Plans and Entitlements (Community / Pro / Cloud)

This document explains Pulse's user-facing plan structure and how it maps to runtime feature gates.

For the canonical, code-aligned entitlement table (including internal tier names), see:
- `docs/architecture/ENTITLEMENT_MATRIX.md`

## Plan Mapping (User-Facing -> Code Tiers)

Pulse uses capability keys (for example, `ai_autofix`) to gate features at runtime. Those capabilities are bundled into
internal tiers in `internal/license/features.go`.

User-facing plans map to internal tiers as follows:
- **Community**: `free`
- **Pro**: `pro`, `pro_annual`, `lifetime` (same entitlements)
- **Cloud**: `msp` or `enterprise` (volume/hosted keys)

Notes:
- Items marked **Cloud\*** require the `enterprise` tier (not included in `msp`).
- If you are self-hosting, you can still use the capability keys and `GET /api/license/features` to discover exactly
  what is active in your instance.

## Feature Matrix (Community / Pro / Cloud)

Legend:
- Included: `Y` / `N`
- `Y*`: Cloud Enterprise only (`enterprise` tier)

This matrix is derived from the canonical table in `docs/architecture/ENTITLEMENT_MATRIX.md`.

| Constant | Capability Key | Display Name | Community | Pro | Cloud | Primary Gating Mechanism / Notes |
|---|---|---|:---:|:---:|:---:|---|
| `FeatureAIPatrol` | `ai_patrol` | Pulse Patrol (Background Health Checks) | Y | Y | Y | Patrol itself is available on Community (BYOK). Higher autonomy outcomes and fix execution are separately gated; auto-fix and some mutation endpoints require `ai_autofix` (see API routing in `internal/api/router_routes_ai_relay.go`). |
| `FeatureAIAlerts` | `ai_alerts` | Alert Analysis | N | Y | Y | API route gating via `RequireLicenseFeature(..., ai_alerts, ...)` (for example `/api/ai/investigate-alert` in `internal/api/router_routes_ai_relay.go`). |
| `FeatureAIAutoFix` | `ai_autofix` | Pulse Patrol Auto-Fix | N | Y | Y | Required for fix execution and higher-autonomy actions; enforced via `RequireLicenseFeature` in AI routes (for example `/api/ai/findings/.../reapprove`). |
| `FeatureKubernetesAI` | `kubernetes_ai` | Kubernetes Analysis | N | Y | Y | API route gating via `RequireLicenseFeature(..., kubernetes_ai, ...)` (for example `/api/ai/kubernetes/analyze`). |
| `FeatureAgentProfiles` | `agent_profiles` | Centralized Agent Profiles | N | Y | Y | API route gating via `RequireLicenseFeature(..., agent_profiles, ...)` (for example `/api/admin/profiles/` in `internal/api/router_routes_registration.go`). |
| `FeatureUpdateAlerts` | `update_alerts` | Update Alerts (Container/Package Updates) | Y | Y | Y | Capability exists and is exposed on license status; feature-specific enforcement is implementation-dependent. Included in Community tier per `TierFeatures[TierFree]`. |
| `FeatureRBAC` | `rbac` | Role-Based Access Control (RBAC) | N | Y | Y | API route gating via `RequireLicenseFeature(..., rbac, ...)` for RBAC endpoints (see `internal/api/router_routes_org_license.go`). |
| `FeatureAuditLogging` | `audit_logging` | Enterprise Audit Logging | N | Y | Y | API route gating for audit query/verify/export endpoints via `RequireLicenseFeature(..., audit_logging, ...)` (see `internal/api/router_routes_org_license.go`). |
| `FeatureSSO` | `sso` | Basic SSO (OIDC) | Y | Y | Y | Basic SSO (OIDC) is included in Community tier. Advanced SSO functionality is separately paid via `advanced_sso`. |
| `FeatureAdvancedSSO` | `advanced_sso` | Advanced SSO (SAML/Multi-Provider) | N | Y | Y | Used to gate advanced SSO capabilities such as SAML and multi-provider flows. Frontend currently uses `advanced_sso` to show Advanced SSO UI (see `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`). |
| `FeatureAdvancedReporting` | `advanced_reporting` | Advanced Infrastructure Reporting (PDF/CSV) | N | Y | Y | API route gating via `RequireLicenseFeature(..., advanced_reporting, ...)` for report generation endpoints (see `internal/api/router_routes_org_license.go`). |
| `FeatureLongTermMetrics` | `long_term_metrics` | 90-Day Metric History | N | Y | Y | Used to gate long-range history queries; for example, history durations beyond 7 days are blocked without `long_term_metrics` (see `internal/api/router.go` around the history handler). |
| `FeatureRelay` | `relay` | Remote Access (Mobile Relay, App Coming Soon) | N | Y | Y | API route gating via `RequireLicenseFeature(..., relay, ...)` for relay settings endpoints (see `internal/api/router_routes_ai_relay.go`). |
| `FeatureMultiUser` | `multi_user` | Multi-User Mode | N | N | Y* | Capability key exists for Cloud Enterprise multi-user mode; current API surface may additionally rely on `rbac` for day-to-day role/user operations. |
| `FeatureWhiteLabel` | `white_label` | White-Label Branding | N | N | Y* | Capability key exists; marked as not implemented in `internal/license/features.go`. |
| `FeatureMultiTenant` | `multi_tenant` | Multi-Tenant Mode | N | N | Y* | Requires both a feature flag (`PULSE_MULTI_TENANT_ENABLED=true`) and the `multi_tenant` capability for non-default orgs (see `internal/api/middleware_license.go`). |
| `FeatureUnlimited` | `unlimited` | Unlimited Instances | N | N | Y | Used for volume/instance limit removal (MSP/Enterprise); enforcement is limit-check dependent. |

## Autonomy Levels (AI Safety)

Patrol and the Assistant support tiered autonomy:

| Mode | Behavior | Plan |
|---|---|---|
| **Monitor** | Detect issues only. No investigation or fixes. | Community (BYOK) |
| **Investigate** | Investigates findings and proposes fixes. All fixes require approval. | Community (BYOK) |
| **Auto-fix** | Automatically fixes issues and verifies. Critical findings require approval by default. | Pro / Cloud |
| **Full autonomy** | Auto-fix for all findings including critical, without approval (explicit toggle). | Pro / Cloud |

## What You Get (By Plan)

### Community
- Pulse Patrol (BYOK): scheduled background analysis and findings.
- Basic SSO (OIDC).
- Update alerts (container/package update signals).

### Pro
- Everything in Community, plus:
- Alert-triggered analysis (`ai_alerts`)
- Auto-fix and higher autonomy (`ai_autofix`)
- Kubernetes AI analysis (`kubernetes_ai`)
- Centralized agent profiles (`agent_profiles`)
- Advanced SSO (`advanced_sso`)
- RBAC (`rbac`)
- Audit logging (`audit_logging`)
- Advanced reporting (`advanced_reporting`)
- Long-term metrics history (`long_term_metrics`)
- Remote access via relay (`relay`) with staged mobile app rollout

### Cloud
- Everything in Pro, plus:
- Unlimited instances (`unlimited`)
- Cloud Enterprise only: multi-tenant orgs, multi-user mode, and (future) white-labeling

## License Activation and Introspection

Pulse plan upgrades are activated locally with a license key.

- License key storage: `license.enc` under the Pulse config directory (encrypted; requires `.encryption.key` to decrypt).
- Export/import note: license files are not included in exports, so you typically re-activate after migrations.

### Feature Status API

You can inspect active feature gates via:
- `GET /api/license/features` (authenticated)

This returns a feature map including keys like `ai_alerts`, `ai_autofix`, `kubernetes_ai`, and `multi_tenant` so you can
conditionally enable Pro/Cloud-only workflows safely.

## Deep Dives

- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md)
- [Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md)
- [Pulse AI overview](AI.md)
