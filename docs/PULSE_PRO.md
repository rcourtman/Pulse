# Pulse Plans and Entitlements (Community / Relay / Pro / Cloud)

This document explains Pulse's user-facing plan structure, the locked self-hosted commercial model, and how those plans map to runtime feature gates.

For the canonical, code-aligned entitlement table (including internal tier names), see:
- `docs/architecture/ENTITLEMENT_MATRIX.md`

## Plan Mapping (User-Facing -> Code Tiers)

Pulse uses capability keys (for example, `ai_autofix`) to gate features at runtime. Those capabilities are bundled into internal tiers in `pkg/licensing/features.go`.

User-facing plans map to internal tiers as follows:
- **Community**: `free`
- **Relay**: `relay`
- **Pro**: `pro`, `pro_annual`, `lifetime`
- **Cloud**: `msp` or `enterprise`

Notes:
- `lifetime` keeps the same runtime feature set as Pro, but lifetime and grandfathered recurring legacy entitlements remain uncapped for monitored systems and guest access. Other migrated legacy paid installs can still carry cohort continuity metadata for support and audit, but self-hosted monitoring volume is no longer the paid gate.
- Items marked **Cloud*** require the `enterprise` tier rather than the base `msp` tier.
- If you are self-hosting, you can use capability keys and `GET /api/license/features` to discover exactly what is active in your instance.

## Self-Hosted Commercial Model

Pulse does not monetize self-hosted users on monitored-system volume. The counted
unit remains a monitored system for product understanding, migrations, and
inventory truth, but self-hosted core monitoring is not the paid gate.

Self-hosted pricing is:

| Plan | Price | Core monitoring | Metric history | Purpose |
|---|---:|---|---:|---|
| Community | Free | Unlimited | 7 days | Full self-hosted monitoring for normal homelab use |
| Relay | $4.99/mo or $39/yr | Unlimited | 14 days | Remote access, mobile, push, and convenience |
| Pro | $8.99/mo or $79/yr | Unlimited | 90 days | AI operations, automation, and advanced admin features |

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
- Legacy recurring Pulse Pro subscriptions already active before the public v6 pricing cutover keep their grandfathered recurring price and uncapped monitored-system and guest capacity until cancellation.
- Existing lifetime license holders remain valid and uncapped.
- Supported legacy paid v5 migrations outside that recurring grandfathered path can still exchange into the v6 activation model without losing self-hosted monitoring access. Migration metadata can preserve the original cohort for support and audit, but monitored-system volume is no longer the paid gate.

### Paid Customer Continuity Matrix

| Customer cohort | What happens in v6 | Pricing and capacity outcome |
|---|---|---|
| Legacy recurring subscriber from a v5 or earlier Pulse Pro monthly/annual plan, already active before the public v6 pricing cutover | The install can migrate into the v6 activation model without forcing a repurchase. | The existing recurring price stays in place and monitored-system plus guest capacity remain uncapped while the subscription remains continuously active. |
| Existing lifetime license holder | The license remains valid through the v6 licensing transition. | Lifetime remains permanently valid and monitored-system plus guest capacity stay uncapped. |
| Legacy paid v5 license migrated into v6 outside the recurring grandfathered path | The install can still exchange into the v6 activation model without forcing a repurchase. Migration records can still preserve the original cohort for support and audit. | Self-hosted monitoring stays available; monitored-system volume is no longer sold as a paid gate on current v6 self-hosted plans. |
| Former recurring subscriber who already canceled or later lapses/cancels | A later return is treated as a new paid purchase, not as a grandfathered renewal. | The old grandfathered price and uncapped capacity do not resume automatically; current public v6 pricing applies. |
| New self-hosted v6 purchase | The purchase uses the current Community / Relay / Pro self-hosted plans. | Core monitoring stays unlimited; paid value comes from convenience, AI, history, and advanced admin features. |

Support rule:
- If any self-hosted v6 install shows a bounded monitored-system cap after activation or migration, treat it as a bug rather than as intended policy. Guest limits still follow the active tier or continuity contract.

## Feature Matrix

Legend:
- Included: `Y` / `N`
- `Y*`: Cloud Enterprise only (`enterprise` tier)

This matrix is derived from the canonical table in `docs/architecture/ENTITLEMENT_MATRIX.md` plus runtime history/limit semantics exposed through entitlements.

| Constant | Capability Key | Display Name | Community | Relay | Pro | Pro+ | Cloud | Primary Gating Mechanism / Notes |
|---|---|---|:---:|:---:|:---:|:---:|:---:|---|
| `FeatureAIPatrol` | `ai_patrol` | Pulse Patrol (Background Health Checks) | Y | Y | Y | Y | Y | Patrol itself is available on Community with BYOK. Activated or trial-backed installs can use 25 Patrol quickstart runs with no API key for first-run activation. Higher-autonomy outcomes and fix execution are separately gated. |
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
- Unlimited self-hosted core monitoring.
- 7-day history.
- Pulse Patrol with BYOK.
- Patrol quickstart after activation or trial: 25 Patrol runs with no API key on a server-verified install.
- Quickstart is Patrol-only activation support, not a general hosted chat entitlement.
- Basic SSO and update alerts.

### Relay
- Everything in Community, plus:
- 14-day history.
- Remote access via Relay.
- Mobile app access and push notifications.

### Pro
- Everything in Relay, plus:
- AI alert analysis.
- Auto-fix and higher autonomy.
- Kubernetes AI analysis.
- Centralized agent profiles.
- Advanced SSO, RBAC, audit logging, and advanced reporting.
- 90-day history.

### Legacy Pro+
- Existing Pro+ entitlements remain supported for current holders, but Pro+ is no longer presented as a public self-hosted plan because monitored-system volume is no longer the paid boundary.

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
