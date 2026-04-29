# Pulse v6 Pricing & Tiering (Canonical)

> Consolidation Notice (2026-02-27):
> Primary v6 execution authority is `docs/release-control/v6/internal/SOURCE_OF_TRUTH.md` (+ `docs/release-control/v6/internal/status.json`).
> This file remains the detailed pricing evidence/spec and must stay aligned with the release-control source.

> **Status:** APPROVED — Final structure for v6 launch.
> **Date:** 2026-02-25
> **Replaces:** All previous pricing documents and v5 pricing structure.

This document is the single source of truth for Pulse v6 pricing, tiering, feature
allocation, and conversion mechanics. All code, UI, marketing, and documentation must
align with this document. If there is a conflict, this document wins.

---

## Design Principles

1. **Free attracts, paid converts.** The free tier must be good enough to get users in the
   door, while paid tiers must add obvious operational value for serious users.
2. **Gate on action, not information.** AI Patrol on self-hosted installs uses the operator's
   configured provider or local model. We never cap how many times users can run
   Patrol through their own provider. The paid gate is on auto-execution of fixes,
   not on analysis or suggestions.
3. **Smooth upgrade ladder.** No large price gaps. Every step up has a clear reason.
4. **Simple to understand.** A homelabber should know which tier is right for them in
   under 10 seconds.

---

## Counted Unit

**Rule:** Pulse tracks a **monitored system** as the canonical counted unit for product
understanding, migrations, and any hosted or legacy continuity policy that still uses
capacity semantics. On current self-hosted v6 plans, monitored systems are not the paid
gate.

**Counts as one monitored system:**
- Proxmox PVE node
- PBS / PMG server
- Standalone Linux, Windows, or macOS host
- Docker host
- TrueNAS / Unraid system
- Kubernetes cluster
- Any other top-level infrastructure system Pulse actively monitors as a first-class root

**Does NOT count separately:**
- VMs
- containers
- pods
- services
- disks
- pools
- datastores
- backup jobs
- any child resource discovered under a counted top-level system

**Collection path does not matter:**
- Agent-backed monitoring and API-backed monitoring resolve to the same monitored-system identity
- If the same underlying system is seen by both an agent and an API connection, it is represented once
- Deduplication must follow canonical unified-resource identity rather than transport-specific state

**Why this model:** counting by monitored systems matches the product Pulse is actually
becoming, keeps inventory honest across collection paths, and avoids turning child-resource
sprawl into self-hosted upsell pressure. Any remaining hosted or continuity capacity logic
should still use this canonical unit rather than transport-specific counts.

**Definition (shown on pricing page, add-system UI, and docs):**
> "A counted system is a top-level machine or cluster Pulse actively monitors. Each monitored
> system counts once in Pulse's inventory, no matter how Pulse collects it. Everything under
> that system — VMs, containers, pods, disks, backups, and services — is included."

**Counting stability:** when a hosted, MSP, or legacy continuity policy uses monitored-system
capacity, a monitored system should only count after it is stable enough to appear as a
durable monitored root. Existing offline systems should release their slot only after a
deliberate grace period, not on transient disconnects.

**Transparent ledger:** in-product UI must show the exact counted systems, their collection
path, and their first-seen / last-seen state so users can understand Pulse's inventory truth.

**Implementation transition note:** any remaining `max_agents` or agent-backed counting paths
are compatibility boundaries for hosted, MSP, or legacy continuity logic, not the self-hosted
commercial model. The canonical v6 destination is monitored-system identity and ledger truth,
with self-hosted commercial surfaces treating core monitoring as included rather
than sold by monitored-system volume.

**Examples:**
- A 3-node Proxmox cluster monitored node-by-node counts as **3 monitored systems**
- A 3-node Proxmox cluster plus 1 PBS server plus 1 TrueNAS system counts as **5 monitored systems**
- One Docker host with 15 containers counts as **1 monitored system**
- One Kubernetes cluster with 40 pods counts as **1 monitored system**
- The same host connected by agent and API still counts as **1 monitored system**

---

## Self-Hosted Tiers

### Community (Free) — $0

| Element | Value |
|---|---|
| Monitoring scope | **Core self-hosted monitoring included** |
| Monitoring | Full (Proxmox, Docker, K8s, agents) |
| Alerts | Threshold-based |
| SSO | Basic OIDC |
| Update notifications | Yes |
| Metrics history | **7 days** |
| AI Patrol | Monitor + root-cause summaries + fix suggestions with the operator's configured provider or local model |
| Safe remediation workflows | **No** (must apply fixes manually) |
| Alert-triggered root-cause analysis | No |
| Relay | No |
| Push notifications | No |
| Custom URL | No |
| RBAC | No |
| Audit logging | No |
| SAML SSO | No |
| Agent profiles | No |
| PDF/CSV reporting | No |

**AI Patrol in free tier:** Users provide their own provider connection
(OpenAI/Anthropic/etc.) or a local model endpoint. No commercial quota applies:
it's their provider, their key or local runtime, and their operating cost. Pulse
applies technical guardrails such as anti-loop protection and rate protection.

In both modes, the paid gate is governed execution through Pulse: free users see the
full analysis and fix suggestions but apply fixes manually. Pro users can run approved
remediation actions through Pulse.

### Relay — $4.99/month or $39/year

| Element | Value |
|---|---|
| Monitoring scope | **Core self-hosted monitoring included** |
| Everything in Free | Yes |
| Relay remote access | **Yes** |
| Mobile app access | **Yes** |
| Push notifications | **Yes** |
| Custom URL | **yourlab.pulserelay.pro** |
| Metrics history | **14 days** |
| Safe remediation workflows | No |
| Alert-triggered root-cause analysis | No |
| RBAC/Audit/SAML | No |
| Reporting | No |

**Positioning:** The convenience tier. It should feel cheap enough to buy on the spot when
someone wants secure remote access, mobile checks, push notifications, and longer history
without changing their self-hosted monitoring scope.

### Pro — $8.99/month or $79/year

| Element | Value |
|---|---|
| Monitoring scope | **Core self-hosted monitoring included** |
| Everything in Relay | Yes |
| Safe remediation workflows | **Yes** (approved execution, safety preflight, rollback) |
| Alert-triggered root-cause analysis | **Yes** |
| Metrics history | **90 days** |
| RBAC | **Yes** |
| Audit logging | **Yes** |
| SAML SSO | **Yes** |
| Agent profiles | **Yes** |
| PDF/CSV reporting | **Yes** |
| Trial | **14-day, no credit card** |

**Positioning:** For serious self-hosted operators who want Pulse to move from monitoring
into operations. The marketing pitch focuses on three things:
1. "Explain what broke" (alert-triggered root-cause analysis)
2. "Fix it safely" (safe remediation workflows)
3. "Keep longer operating memory" (90-day history)

Relay convenience and the team extras (RBAC, audit logging, SAML, reporting, and agent
profiles) are included, but they are supporting value rather than the headline.

### Pro+ — Legacy continuity tier only

Existing Pro+ entitlements remain supported for continuity, but Pro+ is no longer part of
the public v6 self-hosted ladder because monitored-system volume is not the paid boundary.
Runtime feature access matches Pro, while grandfathered recurring or lifetime continuity can
still preserve uncapped self-hosted monitoring and child-resource volume where applicable.

---

## Cloud Tiers (Hosted — separate page)

All Cloud tiers include everything in Pro + managed hosting + daily automated backups.
Cloud launches alongside v6 (not behind a waitlist).

### Cloud Starter — $29/month or $249/year

| Element | Value |
|---|---|
| Agents | **10** |
| All Pro features | Yes |
| Managed hosting | Yes |
| Daily backups | Yes |
| Support | Community |
| Founding rate | **$19/month** (first 100 signups, locked while subscription active) |
| Trial | **14-day, no credit card** |

### Cloud Power — $49/month or $449/year

| Element | Value |
|---|---|
| Agents | **30** |
| All Pro features | Yes |
| Managed hosting | Yes |
| Daily backups | Yes |
| Support | **Priority** |
| Trial | **14-day, no credit card** |

### Cloud Max — $79/month or $699/year

| Element | Value |
|---|---|
| Agents | **75** |
| All Pro features | Yes |
| Managed hosting | Yes |
| Daily backups | Yes |
| Support | **Priority** |
| Trial | **14-day, no credit card** |

75+ agents: Contact us.

---

## MSP Tiers (Multi-Tenant — separate page)

All MSP tiers include everything in Pro + multi-tenant management UI + port separation
(agent vs web UI) + webhook templates. Annual pricing is 2 months free (~17% savings).

### MSP Starter — $149/month or $1,490/year

| Element | Value |
|---|---|
| Clients | Up to **10** |
| Host pool | **50** |
| Multi-tenant UI | Yes |
| Port separation | Yes |
| Webhook templates | Yes |
| White-label | Future |

### MSP Growth — $249/month or $2,490/year

| Element | Value |
|---|---|
| Clients | Up to **25** |
| Host pool | **150** |
| Multi-tenant UI | Yes |
| Port separation | Yes |
| Webhook templates | Yes |
| White-label | Future |

### MSP Scale — $399/month or $3,990/year

| Element | Value |
|---|---|
| Clients | Up to **50** |
| Host pool | **400** |
| Multi-tenant UI | Yes |
| Port separation | Yes |
| Webhook templates | Yes |
| White-label | Future |

### MSP Enterprise — Custom

| Element | Value |
|---|---|
| Clients | **50+** |
| Host pool | Custom |
| White-label | **Yes** |
| Pricing | Negotiated |

---

## Conversion Mechanics

### 1. Operations-value ladder
Self-hosted conversion should come from clear operational upgrades, not monitored-system
capacity pressure. Community proves the core monitoring loop, Relay sells convenience,
and Pro sells safe operations, longer history, and team/admin controls.

### 2. AI fix previews (strongest lever)
Free/Relay users see exactly what Patrol found and how to fix it (specific commands), but
must apply manually. Pro users see the same information with an "Apply Fix" button. Every
Patrol finding is a conversion moment.

### 3. Provider setup stays self-managed
AI setup on self-hosted installs should point operators at their own provider connection or
local model endpoint. Paid operations features such as safe remediation
workflows, alert analysis, and longer history remain opt-in extras; they do not
replace the self-managed AI runtime path.

### 4. Relay as optional convenience ($39/yr)
Gives remote-access and mobile users a small paid convenience option without turning core
self-hosted monitoring into a capacity tier.
Remote access + mobile + push notifications + custom URL + 14-day history should make it
an easy purchase for users who want Pulse available outside their LAN.

### 5. Opt-in commercial discovery
Default self-hosted v6 sessions must not present ordinary users with trial, paywall, or
proactive paid-service prompts. Commercial discovery is allowed only when the user explicitly
opens pricing/activation/recovery/support flows, when the session is hosted/cloud, or when an
existing entitlement requires a clear renewal or recovery path.

High-intent product moments should stay useful without becoming sales surfaces:
- Patrol finds a fixable issue -> show the manual fix and BYOK/provider path first; safe remediation execution can
  remain an opt-in Pro capability where commercial prompts are explicitly allowed.
- An alert needs deeper explanation -> keep free investigation context useful; paid alert
  analysis stays an optional, discoverable extra.
- User taps a longer chart range -> explain the local retention state without implying a
  monitoring capacity limit.
- User wants remote access or mobile pairing -> point to Relay only inside the explicit Relay
  setup/commercial handoff, not as a global nudge.

### 6. No self-hosted monitored-system overflow gate
Self-hosted Community users should not need a temporary monitored-system overflow path because
core self-hosted monitoring is unlimited. Onboarding can still surface Relay and Pro value when
users try remote access, push notifications, longer history, alert investigation, or safe remediation workflows.

### 7. Transparent monitored-system ledger
The ledger remains important for inventory truth, hosted/MSP limits, and support. It should
show exactly which top-level systems Pulse sees, which collection path is being used, and what
is included under each system. On self-hosted Community/Relay/Pro, it must not create a false
"X/Y systems" paywall or imply that users need to buy more monitoring room.

### 8. No default upsell cadence
There is no proactive self-hosted upsell cadence in v6 GA. If older compatibility settings
mention prompt reduction, treat them as legacy controls; the v6 default is already quiet unless
the user enters an explicit commercial path or has an entitlement state that needs attention.

### 9. Cloud launches with v6
Not behind a waitlist. Real pricing, real signup. Captures convenience buyers who don't
want to self-host.

---

## v5 Customer Migration

- Existing Pro customers keep their current recurring price until they cancel
- Auto-exchange to v6 license format on binary upgrade
- If subscription lapses and they return, they come back at v6 rates
- Exchange endpoint: `POST /v1/licenses/exchange`
- Once migrated: renewal emails suppressed, legacy JWT disabled

## Self-Hosted Cap Migration

- There is no v6 self-hosted monitored-system cap migration for Community, Relay, Pro, or Pro+
- Existing self-hosted users keep their monitored coverage through the v6 rollout
- Hosted Cloud and MSP capacity limits remain plan-specific license claims, not self-hosted static tier defaults
- The UI may still explain monitored-system identity, but it must not frame self-hosted growth as a capacity upsell

---

## Self-Hosted Trial Policy

Self-hosted v6 GA should not present a trial acquisition path inside the normal
app. Paid self-hosted plans are discoverable from explicit pricing, activation,
recovery, support, or account flows rather than being pushed into ordinary
monitoring, infrastructure setup, AI setup, or Patrol workflows.

Legacy or externally initiated activation plumbing may remain as a compatibility
boundary while existing purchases and support cases are migrated, but it is not a
customer-facing GA funnel.

---

## Pricing Page Layout

### Main pricing page (self-hosted — 3 columns)

```
  Community     Relay           Pro
  $0            $39/yr          $79/yr
  Unlimited     Unlimited       Unlimited
  monitoring    monitoring      monitoring

  [Get Started] [Buy Relay]     [Choose Pro]
```

Below the table:
- "Need managed hosting? → See Cloud plans"
- "Managing clients? → See MSP plans"
- "Need a custom commercial deployment? → Contact us"

### Cloud page (3 columns)

```
  Starter       Power           Max
  $29/mo        $49/mo          $79/mo
  10 agents     30 agents       75 agents
```

### MSP page (3 columns + Enterprise)

```
  Starter       Growth          Scale           Enterprise
  $149/mo       $249/mo         $399/mo         Custom
  10 clients    25 clients      50 clients      50+
```

---

## Canonical User-Facing Copy

### Counted-unit explainer

Use this exact idea wherever pricing, inventory, or hosted/MSP limit enforcement needs to
explain monitored-system identity:

> "Pulse counts monitored systems, not everything underneath them. Each top-level machine or
> cluster counts once, no matter how Pulse collects it. VMs, containers, pods, disks, backups,
> and services under that system are included."

### Plan taglines

- **Community:** Monitor your self-hosted infrastructure for free.
- **Relay:** Monitor from anywhere with remote access, mobile, push, and longer history.
- **Pro:** Pulse does not just watch your infrastructure. It helps operate it.
- **Pro+:** Legacy continuity tier for existing holders.

### Boundary-upgrade copy

- **Community → Relay:** Want Pulse outside your LAN? Upgrade to Relay for secure remote access, mobile, push notifications, and 14-day history.
- **Relay → Pro:** Want Pulse to do more than alert? Upgrade to Pro for root-cause analysis, safe remediation workflows, and 90-day history.
- **Existing self-hosted monitoring:** Your monitored systems keep working. Paid self-hosted tiers add convenience and operations features, not more monitoring room.

---

## Stripe Price IDs

> Updated 2026-04-24 with the final public self-hosted Relay / Pro price IDs.

### Self-Hosted

> 2026-04-24 implementation: the locked Relay and Pro monthly/annual prices now
> exist in live Stripe and are the only self-hosted v6 prices marked
> `public_checkout` in the live license-server plan map. The previous higher
> pre-GA Relay / Pro prices remain non-public compatibility entries only. Pro+
> is a continuity tier, not a public self-hosted checkout column.

- Relay Monthly: `price_1TPmE5BrHBocJIGHdwLp4tTA` ($4.99/mo)
- Relay Annual: `price_1TPmE5BrHBocJIGH7P6JgMHP` ($39/yr)
- Pro Monthly: `price_1TPmE6BrHBocJIGHHaPwluoM` ($8.99/mo)
- Pro Annual: `price_1TPmE6BrHBocJIGHR8bMvjK8` ($79/yr)
- Pro+ renewal/continuity prices: `price_1T51LIBrHBocJIGHkUjg7sgO` ($18/mo), `price_1T51LIBrHBocJIGHvVaoGsGF` ($149/yr), not public checkout

### Cloud (created 2026-02-28)
- Cloud Starter Monthly: `price_1T5kflBrHBocJIGHUqPv1dzV` ($29/mo)
- Cloud Starter Annual: `price_1T5kfmBrHBocJIGHTS3ymKxM` ($249/yr)
- Cloud Starter Founding: `price_1T5kfnBrHBocJIGHATQJr79D` ($19/mo — first 100 signups)
- Cloud Power Monthly: `price_1T5kg2BrHBocJIGHmkoF0zXY` ($49/mo)
- Cloud Power Annual: `price_1T5kg3BrHBocJIGH2EtzKofV` ($449/yr)
- Cloud Max Monthly: `price_1T5kg4BrHBocJIGHHa8Ecqho` ($79/mo)
- Cloud Max Annual: `price_1T5kg5BrHBocJIGH5AIJ4nVc` ($699/yr)

### MSP (created 2026-02-28)
- MSP Starter Monthly: `price_1T5kgTBrHBocJIGHjOs15LI2` ($149/mo)
- MSP Starter Annual: `price_1T5kgUBrHBocJIGHT6PiOn6x` ($1,490/yr)
- MSP Growth Monthly: `price_1T5kgVBrHBocJIGHulNsCTb1` ($249/mo)
- MSP Growth Annual: `price_1T5kgWBrHBocJIGHTuaNjnJ2` ($2,490/yr)
- MSP Scale Monthly: `price_1T5kgWBrHBocJIGHo40iFeRd` ($399/mo)
- MSP Scale Annual: `price_1T5kgXBrHBocJIGHWlOgTyGV` ($3,990/yr)

### Grandfathered (v5)
- v5 Pro Monthly: `price_1ShIsdBrHBocJIGH71yQusLG` ($9/mo)
- v5 Pro Annual: `price_1ShIsnBrHBocJIGHBKkzsZ3T` ($79/yr)

---

## Implementation Slices Required

### Pulse runtime

- [x] Treat self-hosted Community / Relay / Pro and legacy Pro+ continuity defaults as core monitoring included without a monitored-system volume gate
- [x] Preserve grandfathered v5 recurring plans as uncapped continuity states while subscriptions remain active
- [x] Keep hosted Cloud / MSP capacity out of static self-hosted tier defaults and in plan-specific license claims
- [ ] Keep refining monitored-system identity and ledger truth for inventory, hosted/MSP limits, and support workflows

### Frontend

- [x] Remove self-hosted monitored-system cap pressure from billing and pricing surfaces
- [x] Present the public self-hosted ladder as Community / Relay / Pro
- [ ] Keep ledger and inventory language focused on what Pulse monitors, not paid capacity pressure
- [ ] Keep paid prompts out of ordinary self-hosted runtime surfaces; commercial copy belongs in explicit pricing, activation, recovery, hosted, or entitlement-aware paths

### License server / checkout / landing pages (`pulse-pro`)

- [x] Create new self-hosted Stripe prices for Relay / Pro at the locked public bands
- [x] Update plan mappings, checkout flows, and renewal-safe migration logic without disturbing grandfathered v5 continuity
- [x] Keep Pro+ out of the public checkout ladder unless a separate continuity requirement explicitly needs it
- [x] Cut the landing page, checkout copy, and purchase surfaces over to core-monitoring-included language and the new price bands

### Cloud / MSP

- [ ] Keep current Cloud/MSP list pricing unchanged for now unless a separate decision explicitly revises it
- [ ] Continue differentiating Cloud / MSP limits via plan-specific license claims rather than self-hosted static bands

---

## Review Checkpoints

### 30-day post-launch review
- Free → Relay opt-in purchase rate
- Free → Pro opt-in purchase rate
- Relay → Pro upgrade rate
- Which explicit commercial handoffs are used most (pricing, activation, recovery, hosted)
- Support load per tier

### 60-day post-launch review
- Churn by tier
- Revenue per user (actual vs projected)
- Cloud margin analysis
- MSP pipeline health
- Pricing adjustment decisions (if needed)

---

## Revision History

| Date | Change | Author |
|---|---|---|
| 2026-04-29 | Replaced stale unlimited-monitoring phrasing with core-monitoring-included language across active v6 docs and upgrade-return copy so Community does not read like a former capacity upsell. | Codex |
| 2026-04-23 | Removed stale self-hosted monitored-system cap and Pro+ public-checkout language. Reaffirmed Community / Relay / Pro as current public self-hosted tiers, with Pro+ as continuity only and Pro value centered on operations, history, and admin controls. | Codex |
| 2026-03-17 | Re-locked the self-hosted commercial model around monitored systems rather than installed agents. New self-hosted public pricing: Relay $4.99/$39, Pro $8.99/$79, Pro+ $14.99/$129. Added free-tier grace policy and marked the monitored-system counting migration as still required in code. | Codex + Richard |
| 2026-02-25 | Initial v6 pricing structure finalized | Richard + Claude + Codex |
| 2026-02-25 | Changed counting to agents-only model. Only installed Pulse Unified Agents count toward limits. PVE/PBS/PMG/Docker/K8s connections and discovered resources don't count. This makes limits much more generous in practice (5 agents can monitor an entire multi-node cluster). | Richard + Claude |
| 2026-02-26 | Updated Code Changes Required section — marked Pulse core backend and frontend as DONE, narrowed remaining work to license server (pulse-pro) and cloud control plane. | Claude |
| 2026-02-26 | Marked license server work as DONE (Stripe products created, max_nodes→max_agents complete, plan definitions configured, tests added). Only Cloud control plane Stripe products remain. | Claude |
| 2026-02-27 | Frontend gating audit completed. Backend enforcement is correct for all features. Historical frontend UX gaps were identified before the later free-first commercial-surface policy narrowed where upgrade handoffs belong. Details in `feature-capability-audit-2026-02.md` section 9. | Claude |
