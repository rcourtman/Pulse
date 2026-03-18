# Pulse Plans and Entitlements (Community / Relay / Pro / Pro+ / Cloud)

This document explains Pulse's user-facing plan structure, the locked self-hosted commercial model, and how those plans map to runtime feature gates.

For the canonical, code-aligned entitlement table (including internal tier names), see:
- `docs/architecture/ENTITLEMENT_MATRIX.md`

## Plan Mapping (User-Facing -> Code Tiers)

Pulse uses capability keys (for example, `ai_autofix`) to gate features at runtime. Those capabilities are bundled into internal tiers in `internal/license/features.go`.

User-facing plans map to internal tiers as follows:
- **Community**: `free`
- **Relay**: `relay`
- **Pro**: `pro`, `pro_annual`, `lifetime`
- **Pro+**: `pro_plus`
- **Cloud**: `msp` or `enterprise`

Notes:
- `lifetime` keeps the same runtime entitlements as Pro.
- Items marked **Cloud*** require the `enterprise` tier rather than the base `msp` tier.
- If you are self-hosting, you can use capability keys and `GET /api/license/features` to discover exactly what is active in your instance.

## Self-Hosted Commercial Model

Pulse sells monitored coverage. The counted unit is a monitored system, not an installed agent.

Self-hosted pricing is locked to:

| Plan | Price | Included monitored systems | Metric history | Purpose |
|---|---:|---:|---:|---|
| Community | Free | 5 | 7 days | One real small lab end to end |
| Relay | $4.99/mo or $39/yr | 8 | 14 days | Cheap headroom plus remote access |
| Pro | $8.99/mo or $79/yr | 15 | 90 days | Automation and operations tier |
| Pro+ | $14.99/mo or $129/yr | 50 | 90 days | Larger self-hosted labs |

Counted examples:
- Proxmox PVE node
- PBS or PMG server
- Standalone Linux, Windows, or macOS host
- Docker host
- TrueNAS or Unraid system
- Kubernetes cluster

Not counted separately:
- VMs
- containers
- pods
- disks
- pools
- datastores
- backup jobs
- other child resources under a counted top-level system

Runtime rules:
- API-backed monitoring and agent-backed monitoring consume the same cap.
- If the same system is seen through both paths, it counts once.
- Deduplication follows canonical unified-resource identity rather than transport-specific state.

Migration policy:
- Existing paid v5 customers keep their grandfathered recurring continuity until cancellation.
- Existing free users above the new Community cap are not hard-broken on rollout day.
- During grace, existing monitoring keeps working.
- During grace, only new counted-system additions are blocked until the user removes systems or upgrades.

## Feature Matrix

Legend:
- Included: `Y` / `N`
- `Y*`: Cloud Enterprise only (`enterprise` tier)

This matrix is derived from the canonical table in `docs/architecture/ENTITLEMENT_MATRIX.md` plus runtime history/limit semantics exposed through entitlements.

| Constant | Capability Key | Display Name | Community | Relay | Pro | Pro+ | Cloud | Primary Gating Mechanism / Notes |
|---|---|---|:---:|:---:|:---:|:---:|:---:|---|
| `FeatureAIPatrol` | `ai_patrol` | Pulse Patrol (Background Health Checks) | Y | Y | Y | Y | Y | Patrol itself is available on Community with BYOK. Higher-autonomy outcomes and fix execution are separately gated. |
| `FeatureRelay` | `relay` | Remote Access (Mobile Relay) | N | Y | Y | Y | Y | API route gating via `RequireLicenseFeature(..., relay, ...)` for relay settings and onboarding endpoints. |
| `FeatureAIAlerts` | `ai_alerts` | Alert Analysis | N | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., ai_alerts, ...)`. |
| `FeatureAIAutoFix` | `ai_autofix` | Pulse Patrol Auto-Fix | N | N | Y | Y | Y | Required for fix execution and higher-autonomy actions. |
| `FeatureKubernetesAI` | `kubernetes_ai` | Kubernetes Analysis | N | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., kubernetes_ai, ...)`. |
| `FeatureAgentProfiles` | `agent_profiles` | Centralized Agent Profiles | N | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., agent_profiles, ...)`. |
| `FeatureUpdateAlerts` | `update_alerts` | Update Alerts (Container/Package Updates) | Y | Y | Y | Y | Y | Included in Community tier per `TierFeatures[TierFree]`. |
| `FeatureSSO` | `sso` | Basic SSO (OIDC) | Y | Y | Y | Y | Y | Basic SSO is included in Community tier. |
| `FeatureAdvancedSSO` | `advanced_sso` | Advanced SSO (SAML/Multi-Provider) | N | N | Y | Y | Y | Used to gate advanced SSO capabilities such as SAML and multi-provider flows. |
| `FeatureRBAC` | `rbac` | Role-Based Access Control (RBAC) | N | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., rbac, ...)`. |
| `FeatureAuditLogging` | `audit_logging` | Audit Logging | N | N | Y | Y | Y | API route gating for audit query, verify, and export endpoints. |
| `FeatureAdvancedReporting` | `advanced_reporting` | PDF/CSV Reporting | N | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., advanced_reporting, ...)`. |
| `FeatureLongTermMetrics` | `long_term_metrics` | Extended Metric History | N | Y | Y | Y | Y | Runtime history limits are tier-aware through `max_history_days`: Community `7`, Relay `14`, Pro/Pro+ `90`. |
| `FeatureMultiUser` | `multi_user` | Multi-User Mode | N | N | N | N | Y* | Cloud Enterprise only. |
| `FeatureWhiteLabel` | `white_label` | White-Label Branding | N | N | N | N | Y* | Capability key exists but is still marked not implemented in `internal/license/features.go`. |
| `FeatureMultiTenant` | `multi_tenant` | Multi-Tenant Mode | N | N | N | N | Y* | Requires both `PULSE_MULTI_TENANT_ENABLED=true` and the `multi_tenant` capability for non-default orgs. |
| `FeatureUnlimited` | `unlimited` | Unlimited Instances | N | N | N | N | Y | Used for hosted volume and instance limit removal. |

## Autonomy Levels (AI Safety)

Patrol and the Assistant support tiered autonomy:

| Mode | Behavior | Plan |
|---|---|---|
| **Monitor** | Detect issues only. No investigation or fixes. | Community / Relay |
| **Investigate** | Investigates findings and proposes fixes. All fixes require approval. | Community / Relay |
| **Auto-fix** | Automatically fixes issues and verifies. Critical findings require approval by default. | Pro / Pro+ / Cloud |
| **Full autonomy** | Auto-fix for all findings including critical, without approval (explicit toggle). | Pro / Pro+ / Cloud |

## What You Get (By Plan)

### Community
- Core monitoring for up to 5 monitored systems.
- 7-day history.
- Pulse Patrol with BYOK.
- Basic SSO and update alerts.

### Relay
- Everything in Community, plus:
- 8 monitored systems.
- 14-day history.
- Remote access via Relay.
- Mobile app access and push notifications.

### Pro
- Everything in Relay, plus:
- 15 monitored systems.
- AI alert analysis.
- Auto-fix and higher autonomy.
- Kubernetes AI analysis.
- Centralized agent profiles.
- Advanced SSO, RBAC, audit logging, and advanced reporting.
- 90-day history.

### Pro+
- Everything in Pro, with 50 monitored systems for larger self-hosted labs.

### Cloud
- Hosted Pulse with Pro-level capabilities and hosted lifecycle management.
- Cloud Enterprise adds multi-tenant orgs, multi-user mode, and future white-labeling where licensed.

## License Activation and Introspection

Pulse plan upgrades are activated locally with a license key.

- License key storage: `license.enc` under the Pulse config directory (encrypted; requires `.encryption.key` to decrypt).
- Export/import note: license files are not included in exports, so you typically re-activate after migrations.
- Pulse v6 prefers v6 activation keys, but it can migrate valid Pulse v5 Pro or Lifetime JWT-style licenses into the v6 activation model.
- If a v5 license is already persisted on disk during upgrade and no v6 activation state exists yet, Pulse will try to auto-exchange it on startup.
- If you are activating manually in v6, paste the v6 activation key shown on the hosted checkout success page. A backup copy is also sent by email. You can also paste a valid v5 Pro or Lifetime license key and Pulse will try to exchange it automatically.
- If the exchange cannot complete, retry from the v6 license panel or use the self-serve retrieval flow to fetch the current v6 activation key.

### Feature Status API

You can inspect active feature gates via:
- `GET /api/license/features` (authenticated)

This returns a feature map including keys like `relay`, `ai_alerts`, `ai_autofix`, `kubernetes_ai`, and `multi_tenant` so you can conditionally enable paid workflows safely.

## Deep Dives

- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md)
- [Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md)
- [Pulse AI overview](AI.md)
