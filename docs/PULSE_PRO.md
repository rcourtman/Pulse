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
- `pro_plus` remains a legacy compatibility tier for existing holders. It is not a current public self-hosted plan because monitored-system volume is no longer the paid boundary.
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
| Pro | $8.99/mo or $79/yr | Unlimited | 90 days | AI operations and advanced admin features |

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
- API-backed monitoring and agent-backed monitoring use the same counted-system
  model. Self-hosted public plans leave core monitoring unlimited; finite
  capacity policies apply only where a hosted, enterprise, or explicit
  compatibility policy says so.
- If the same system is seen through both paths, it counts once.
- Deduplication follows canonical unified-resource identity rather than transport-specific state.

Migration policy:
- Legacy recurring Pulse Pro subscriptions already active before the public v6 pricing cutover keep their grandfathered recurring price until cancellation. Self-hosted monitoring and child-resource volume remain uncapped under the current v6 policy.
- Existing lifetime license holders remain valid and uncapped.
- Supported legacy paid v5 migrations outside that recurring grandfathered path can still exchange into the v6 activation model without losing self-hosted monitoring access. Migration metadata can preserve the original cohort for support and audit, but monitored-system volume is no longer the paid gate.

### Paid Customer Continuity Matrix

| Customer cohort | What happens in v6 | Pricing and capacity outcome |
|---|---|---|
| Legacy recurring subscriber from a v5 or earlier Pulse Pro monthly/annual plan, already active before the public v6 pricing cutover | The install can migrate into the v6 activation model without forcing a repurchase. | The existing recurring price stays in place while the subscription remains continuously active; self-hosted monitoring and child-resource volume remain uncapped under the current v6 policy. |
| Existing lifetime license holder | The license remains valid through the v6 licensing transition. | Lifetime remains permanently valid; self-hosted monitoring and child-resource volume stay uncapped. |
| Legacy paid v5 license migrated into v6 outside the recurring grandfathered path | The install can still exchange into the v6 activation model without forcing a repurchase. Migration records can still preserve the original cohort for support and audit. | Self-hosted monitoring stays available; monitored-system volume is no longer sold as a paid gate on current v6 self-hosted plans. |
| Former recurring subscriber who already canceled or later lapses/cancels | A later return is treated as a new paid purchase, not as a grandfathered renewal. | The old grandfathered price does not resume automatically; current public v6 pricing applies for paid features while self-hosted monitoring remains unlimited. |
| New self-hosted v6 purchase | The purchase uses the current Community / Relay / Pro self-hosted plans. | Core monitoring stays unlimited; paid value comes from convenience, AI, history, and advanced admin features. |

Support rule:
- If any self-hosted v6 install shows a bounded monitored-system, guest, or child-resource volume cap after activation or migration, treat it as a bug rather than as intended policy.

## V6 Product Classification

Pulse keeps some entitlement keys for compatibility, but not every Pro
capability key is a primary v6 product pillar.

### Build On In v6

These are the current self-hosted Pro pillars that Pulse should keep
investing in, surfacing, and marketing:
- Alert-triggered root-cause analysis.
- Safe remediation workflows through Patrol fix execution and autonomy controls.
- 90-day history.
- Included team/admin extras: Advanced SSO (SAML), RBAC, audit logging,
  reporting, and agent profiles.

### Compatibility-Only In v6

These remain valid runtime gates for backwards compatibility, but should not
be elevated into headline Pro marketing or generic upgrade prompts:
- `FeatureKubernetesAI` / `kubernetes_ai`
  - Keeps the legacy `/api/ai/kubernetes/analyze` route gate intact.
  - Do not present it as a primary Pulse Pro pillar on current v6 surfaces.

### Legacy / Retired Claims

These should not appear as current v6 Pro promises unless they are rebuilt
into first-class product surfaces:
- `incident memory` as a standalone feature name
- `scheduled remediations`
- `execution audit trail`

## Feature Matrix

Legend:
- Included: `Y` / `N`
- `Y*`: Cloud Enterprise only (`enterprise` tier)

This matrix is derived from the canonical table in `docs/architecture/ENTITLEMENT_MATRIX.md` plus runtime history/limit semantics exposed through entitlements.

| Constant | Capability Key | Display Name | Community | Relay | Pro | Cloud | Primary Gating Mechanism / Notes |
|---|---|---|:---:|:---:|:---:|:---:|---|
| `FeatureAIPatrol` | `ai_patrol` | Pulse Patrol (Background Health Checks) | Y | Y | Y | Y | Patrol itself is available on Community with your own provider or local model. Higher-autonomy outcomes and fix execution are separately gated. |
| `FeatureRelay` | `relay` | Remote Access (Mobile Relay) | N | Y | Y | Y | API route gating via `RequireLicenseFeature(..., relay, ...)` for relay settings and onboarding endpoints. |
| `FeatureAIAlerts` | `ai_alerts` | Alert-Triggered Root-Cause Analysis | N | N | Y | Y | API route gating via `RequireLicenseFeature(..., ai_alerts, ...)`. |
| `FeatureAIAutoFix` | `ai_autofix` | Safe Remediation Workflows | N | N | Y | Y | Required for governed fix execution and autonomous remediation actions. |
| `FeatureKubernetesAI` | `kubernetes_ai` | Kubernetes AI Analysis (Compatibility) | N | N | Y | Y | Legacy compatibility gate for `/api/ai/kubernetes/analyze`; not a primary marketed v6 Pro plan pillar. |
| `FeatureAgentProfiles` | `agent_profiles` | Centralized Agent Profiles | N | N | Y | Y | API route gating via `RequireLicenseFeature(..., agent_profiles, ...)`. |
| `FeatureUpdateAlerts` | `update_alerts` | Update Alerts (Container/Package Updates) | Y | Y | Y | Y | Included in Community tier per `TierFeatures[TierFree]`. |
| `FeatureSSO` | `sso` | Basic SSO (OIDC) | Y | Y | Y | Y | Basic SSO is included in Community tier. |
| `FeatureAdvancedSSO` | `advanced_sso` | Advanced SSO (SAML/Multi-Provider) | N | N | Y | Y | Used to gate advanced SSO capabilities such as SAML and multi-provider flows. |
| `FeatureRBAC` | `rbac` | Role-Based Access Control (RBAC) | N | N | Y | Y | API route gating via `RequireLicenseFeature(..., rbac, ...)`. |
| `FeatureAuditLogging` | `audit_logging` | Audit Logging | N | N | Y | Y | API route gating for audit query, verify, and export endpoints. |
| `FeatureAdvancedReporting` | `advanced_reporting` | PDF/CSV Reporting | N | N | Y | Y | API route gating via `RequireLicenseFeature(..., advanced_reporting, ...)`. |
| `FeatureLongTermMetrics` | `long_term_metrics` | Extended Metric History | N | Y | Y | Y | Runtime history limits are tier-aware through `max_history_days`: Community `7`, Relay `14`, Pro `90`. |
| `FeatureMultiUser` | `multi_user` | Multi-User Mode | N | N | N | Y* | Cloud Enterprise only. |
| `FeatureMultiTenant` | `multi_tenant` | Multi-Tenant Mode | N | N | N | Y* | Requires both `PULSE_MULTI_TENANT_ENABLED=true` and the `multi_tenant` capability for non-default orgs. |
| `FeatureUnlimited` | `unlimited` | Hosted Capacity Policy | N | N | N | Y | Hosted/enterprise capacity policy only; not a self-hosted core monitoring gate. |

## Autonomy Levels (AI Safety)

Patrol and the Assistant support tiered autonomy:

| Mode | Behavior | Plan |
|---|---|---|
| **Monitor** | Detect issues only. No investigation or remediation execution. | Community / Relay |
| **Investigate** | Investigates findings and proposes fixes. All remediation actions require approval. | Pro / hosted Cloud |
| **Remediate** | Runs approved safe remediation actions and verifies the outcome. Critical findings require approval by default. | Pro / hosted Cloud |
| **Full autonomy** | Allows critical remediation actions without approval when explicitly enabled. | Pro / hosted Cloud |

## What You Get (By Plan)

### Community
- Unlimited self-hosted core monitoring.
- 7-day history.
- Pulse Patrol with your own provider or local model.
- Basic SSO and update alerts.

### Relay
- Everything in Community, plus:
- 14-day history.
- Remote access via Relay.
- Mobile app access and push notifications.

### Pro
- Everything in Relay, plus:
- Alert-triggered root-cause analysis.
- Safe remediation workflows and autonomy controls.
- Centralized agent profiles.
- Advanced SSO, RBAC, audit logging, and advanced reporting.
- 90-day history.

### Legacy Pro+
- Existing Pro+ entitlements remain supported for current holders, but Pro+ is no longer presented as a public self-hosted plan because monitored-system volume is no longer the paid boundary.

### Cloud
- Hosted Pulse with Pro-level capabilities and hosted lifecycle management.
- Cloud Enterprise adds multi-tenant orgs and multi-user mode.

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

This returns a feature map including keys like `relay`, `ai_alerts`, `ai_autofix`, `agent_profiles`, and `multi_tenant` so you can conditionally enable paid workflows safely.

## Deep Dives

- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md)
- [Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md)
- [Pulse AI overview](AI.md)
