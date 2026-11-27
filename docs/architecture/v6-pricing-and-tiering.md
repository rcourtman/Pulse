# Pulse v6 Pricing & Tiering (Canonical)

> Consolidation Notice (2026-02-27):
> Primary v6 execution authority is `docs/release-control/v6/internal/SOURCE_OF_TRUTH.md` (+ `docs/release-control/v6/internal/status.json`).
> This file remains the detailed pricing evidence/spec and must stay aligned with the release-control source.

> **Status:** APPROVED. Current commercial contract.
> **Date:** 2026-11-27
> **Replaces:** All previous pricing documents and v5 pricing structure.

This document is the single source of truth for Pulse v6 pricing, tiering, feature
allocation, and conversion mechanics. All code, UI, marketing, and documentation must
align with this document. If there is a conflict with release-control state, the
release-control source wins and this file must be corrected.

---

## Design Principles

1. **Free attracts, paid converts.** The free tier must be good enough to get users in the
   door, while paid tiers must add obvious operational value for serious users.
2. **Gate on action, not information.** AI Patrol on self-hosted installs uses the operator's
   configured provider or local model. We never cap how many times users can run
   Patrol through their own provider. The paid gate is on auto-execution of fixes,
   not on analysis or suggestions.
3. **Distinct jobs, clear bundles.** Community, Pro, and Business are not a
   feature-escalation ladder that gates core monitoring. Community is the
   monitoring foundation, Pro is the operations product for individuals and
   small teams, and Business is the same product sized for larger company teams
   with unlimited named team seats and a next-business-day support target.
   Relay left public checkout on 29 September 2026 and retires with Pulse
   Mobile on 31 March 2027. Do not offer Relay again or describe it as a way
   to reach the web UI.
4. **Simple to understand.** A homelabber should know which product fits the job in
   under 10 seconds.
5. **Product-led public language.** Public marketing describes Pulse, Community,
   Pro, Business, Cloud, and MSP as enduring products rather than release trains. Version identifiers
   belong only in version-sensitive tasks such as release notes, downloads, compatibility,
   migration, support, and implementation metadata. They must not lead homepage, product,
   pricing, or acquisition copy.

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

## Self-Hosted Products

### Community (Free) — $0

| Element | Value |
|---|---|
| Monitoring scope | **Core self-hosted monitoring included** |
| Monitoring | Full (Proxmox, Docker, K8s, agents) |
| Alerts | Threshold-based |
| SSO | OIDC/SAML with multi-provider support |
| Update notifications | Yes |
| Metrics history | **Included** (storage retention decides the range; 90 days by default) |
| AI Patrol | Watch-only findings with the operator's configured provider or local model |
| Patrol governed fixes | **No** (must apply fixes manually) |
| Alert investigation | No |
| Relay | No |
| Push notifications | No |
| Customer-specific Relay URL | No |
| RBAC | No |
| Audit logging | No |
| SAML SSO | Yes |
| Agent profiles | No |
| PDF/CSV reporting | No |

**AI Patrol in free tier:** Users provide their own provider connection
(OpenAI/Anthropic/etc.) or a local model endpoint. No commercial quota applies:
it's their provider, their key or local runtime, and their operating cost. Pulse
applies technical guardrails such as anti-loop protection and rate protection.

In both modes, the paid gate is governed execution through Pulse: free users see
watch-only Patrol findings but apply fixes manually. Pro users can let Patrol
investigate, ask for approval when policy requires it, run governed fixes, verify
outcomes, and record what happened.

### Relay (retired 29 September 2026, existing subscribers only)

Relay left public checkout on **29 September 2026** and retires together with
Pulse Mobile on **31 March 2027**. Relay only ever connected the Pulse Mobile
app. It never provided remote access to the Pulse web UI. Do not offer Relay
for purchase again, and do not describe Relay as a way to reach the web UI.

| Element | Value |
|---|---|
| Public checkout | **No** (retired 29 September 2026, existing subscribers only) |
| Monitoring scope | **Core self-hosted monitoring included** |
| Continuing entitlement | **Pro entitlements** at the subscriber's current Stripe price (`$4.99/month` or `$39/year`) for as long as their subscription continues, including after 31 March 2027 |
| Existing Pulse Mobile pairing and push | Continues for already-paired phones until **31 March 2027** |
| Customer-specific Relay URL | No. Relay uses the standard outbound relay service while it remains active for paired phones until 31 March 2027 |
| Metrics history | Included with Community |
| Patrol governed fixes, investigation, RBAC, Audit, Reporting, Agent profiles | **Yes** for continuing Relay subscribers (projected to Pro entitlements via `relay_retirement.go`) |

**Continuity policy:** Existing Relay subscribers keep their Stripe price and
receive Pro entitlements for as long as their subscription remains continuously
active. Do not tell continuing Relay subscribers to buy Pro to obtain those
entitlements. For remote access to the web UI, users use their own VPN or
tunnel. For phone alerts after 31 March 2027, users add an ntfy, Gotify, or
Pushover destination under **Alerts** and open Pulse in their phone's browser.

### Pro ($8.99/month or $79/year)

| Element | Value |
|---|---|
| Monitoring scope | **Core self-hosted monitoring included** |
| Product job | Patrol-powered investigation and governed operations |
| Named team seats | **3 named team seats** on new subscriptions (subscriptions started before the Business launch on 20 August 2026 keep their original unlimited seat allowance) |
| Support | Typically within two business days (no priority-response SLA) |
| Existing Pulse Mobile pairing | Continues for paired phones until 31 March 2027 |
| Patrol modes | **Yes** (choose how much Patrol can do) |
| Issue investigation | **Yes** |
| Governed fixes | **Yes** (approved execution, safety preflight, rollback, verification) |
| Metrics history | Included with Community |
| RBAC | **Yes** |
| Audit logging | **Yes** |
| SAML SSO | Included with Community |
| Agent profiles | **Yes** |
| PDF/CSV reporting | **Yes** |
| Self-hosted trial acquisition | No. Local trial CTAs are retired for v6 GA |

**Positioning:** Pro is the operations product for self-hosted operators who want Pulse to
move from monitoring into investigation and governed action. The
marketing pitch focuses on two things:
1. "Choose Patrol mode" (how much Patrol can do)
2. "Let Patrol investigate and fix governed issues" (issue investigation, governed fixes, verification)

Metric history was a Pro pillar until 17 November 2026. It is now free on every
plan: free installs already stored the data, and only a minority of paid installs
read past seven days.

Team and admin controls (RBAC, audit logging, reporting, and agent profiles) are
included with Pro.

### Business ($399/year, annual only)

Launched for public checkout on **20 August 2026**
(`docs/release-control/v6/internal/records/business-tier-launch-2026-08-20.md`).
Business runs the exact same feature set as Pro and never differentiates by
gating product features.

| Element | Value |
|---|---|
| Price | **$399/year** (annual billing only, no monthly plan) |
| Monitoring scope | **Core self-hosted monitoring included** |
| Product job | Same product and feature set as Pro, sized for a company team |
| Named team seats | **Unlimited named team seats** (`max_users` unset) |
| Support | **Next-business-day target** (no hard SLA) |
| Patrol modes, investigation, governed fixes, RBAC, Audit, Reporting, Agent profiles | **Identical to Pro** |
| Self-hosted trial acquisition | No |

### Self-hosted license and support scope

- One Pro, Business, or continuing retired Relay subscription covers one
  customer-owned Pulse environment. Customer-owned environments remain
  owner-operated when administered by an appointed IT contractor or MSP.
- Monitored systems and child resources are not metered.
- The subscription permits three concurrent activations inside that environment
  for primary, migration, and recovery use. These slots do not license unrelated
  customer environments.
- Each customer may own a separate Pro or Business subscription and independent
  installation. An IT contractor or MSP may install, configure, monitor, and
  maintain it without changing license ownership or requiring an MSP agreement.
- A provider-owned entitlement across client workspaces requires MSP. One
  standard subscription must not be pooled, resold, or shared across unrelated
  customers.
- Community remains MIT-licensed. Independent customer installations and paid
  administration of the open-source core do not require a commercial or MSP license.
- Verified administrative ownership transfer is supported. Resale, sharing,
  and unverified third-party assignment are prohibited.
- Pro and continuing Relay subscriptions include verified commercial support for
  billing, activation, transfer, configuration, and diagnostics, typically
  within two business days. Business carries a next-business-day support target.
  Neither is a contractual SLA or priority-support commitment.

**Readiness assertion `customer-owned-contractor-administration`, adopted
2026-09-19:** Public terms, pricing copy, and support answers must preserve the
distinction between independent customer-owned installations administered by a
contractor and a provider-owned MSP service. Outside IT support alone must never
trigger an MSP requirement. This clarification adds no new plan, price,
activation slot, seat allowance, or entitlement.

### Pro+ (legacy continuity tier only)

Existing Pro+ entitlements remain supported for continuity, but Pro+ is no longer part of
the public v6 self-hosted ladder because monitored-system volume is not the paid boundary.
Runtime feature access matches Pro, while grandfathered recurring or lifetime continuity can
still preserve self-hosted monitoring and child-resource volume as not metered where applicable.

---

## Cloud Tiers (Hosted — unavailable)

Cloud is not currently offered. The prices and tier shapes below are dormant
commercial proposals retained for implementation planning; they are not a
signup promise, supported capacity contract, trial promise, or current support
commitment. Cloud must not reopen until its economic unit/caps, card policy,
support, retention, export, cancellation, reactivation, and runtime enforcement
pass the governed Cloud reopening gate.

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

## MSP Tiers (Provider-hosted, self-serve for Solo through Scale)

MSP provides a provider-owned entitlement across client workspaces, central
management, and one commercial relationship. It is not required merely because
the same contractor administers several independently customer-owned Community,
Pro, or Business installations.

Pulse MSP is not the shared-process organization model. The canonical MSP route
is provider-hosted: the MSP runs a control plane that creates one isolated Pulse
runtime per client workspace. A signed MSP license sets the plan version and
client workspace cap. Pulse-hosted MSP is an optional request-assisted path
where Pulse operates that provider stack.

Provider-hosted MSP offers a self-serve **60-day, two-workspace evaluation**
(no form, card, or approval) and direct in-app Stripe checkout from **Plan** in
the provider portal for **Solo, Starter, Growth, and Scale** (monthly or
annual). **MSP Enterprise** (40+ workspaces), white-label report branding, and
optional Pulse-hosted MSP remain request-assisted.

### MSP Solo ($69/month or $690/year)

| Element | Value |
|---|---|
| Client workspaces | Up to **3** |
| Provider control plane | Yes, provider-hosted |
| Client runtime isolation | One Pulse runtime/container per client |
| Portal handoff | Yes |
| Webhook templates | Gotify, PagerDuty, Slack, and Generic |
| White-label report branding | Requires `white_label` entitlement |
| Access motion | Self-serve 60-day evaluation and in-app Stripe checkout from **Plan** |

### MSP Starter ($149/month or $1,490/year)

| Element | Value |
|---|---|
| Client workspaces | Up to **5** |
| Provider control plane | Yes, provider-hosted |
| Client runtime isolation | One Pulse runtime/container per client |
| Portal handoff | Yes |
| Webhook templates | Gotify, PagerDuty, Slack, and Generic |
| White-label report branding | Requires `white_label` entitlement |
| Access motion | Self-serve 60-day evaluation and in-app Stripe checkout from **Plan** |

### MSP Growth ($249/month or $2,490/year)

| Element | Value |
|---|---|
| Client workspaces | Up to **15** |
| Provider control plane | Yes, provider-hosted |
| Client runtime isolation | One Pulse runtime/container per client |
| Portal handoff | Yes |
| Webhook templates | Gotify, PagerDuty, Slack, and Generic |
| White-label report branding | Requires `white_label` entitlement |
| Access motion | Self-serve 60-day evaluation and in-app Stripe checkout from **Plan** |

### MSP Scale ($399/month or $3,990/year)

| Element | Value |
|---|---|
| Client workspaces | Up to **40** |
| Provider control plane | Yes, provider-hosted |
| Client runtime isolation | One Pulse runtime/container per client |
| Portal handoff | Yes |
| Webhook templates | Gotify, PagerDuty, Slack, and Generic |
| White-label report branding | Requires `white_label` entitlement |
| Access motion | Self-serve 60-day evaluation and in-app Stripe checkout from **Plan** |

### MSP Enterprise (Custom)

| Element | Value |
|---|---|
| Client workspaces | Custom (40+) |
| Hosting shape | Provider-hosted by default, Pulse-hosted optional by agreement |
| White-label report branding | **Yes**, when granted through `white_label` |
| Pricing | Negotiated, request-assisted |

---

## Conversion Mechanics

### 1. Operations-value ladder
Self-hosted conversion should come from clear operational upgrades, not monitored-system
capacity pressure. Community proves the core monitoring loop, Pro sells safe
operations and team/admin controls, and Business scales Pro for larger company
teams with unlimited seats and a next-business-day support target.

### 2. AI fix previews (strongest lever)
Community users see exactly what Patrol found and how to fix it (specific commands), but
must apply fixes manually. Pro and Business users see the same information with
governed execution. Continuing Relay subscribers also receive Pro entitlements.

### 3. Provider setup stays self-managed
AI setup on self-hosted installs should point operators at their own provider connection or
local model endpoint. Paid operations features such as governed fixes
and issue investigation remain opt-in extras. They do not
replace the self-managed AI runtime path.

### 4. Retired Relay (no longer offered)
Relay left public checkout on 29 September 2026 and retires with Pulse Mobile on
31 March 2027. Do not offer Relay in new conversion flows or describe Relay as a
way to reach the web UI. Existing Relay subscribers keep their price and receive
Pro entitlements for as long as their subscription continues.

### 5. Opt-in commercial discovery
Default self-hosted v6 sessions must not present ordinary users with trial, paywall, or
proactive paid-service prompts. Commercial discovery is allowed only when the user explicitly
opens pricing/activation/recovery/support flows, when the session is hosted/cloud, or when an
existing entitlement requires a clear renewal or recovery path.

High-intent product moments should stay useful without becoming sales surfaces:
- Patrol finds a fixable issue -> show the manual fix and BYOK/provider path first. Governed fix execution can
  remain an opt-in Pro capability where commercial prompts are explicitly allowed.
- An alert needs deeper explanation -> keep free investigation context useful. Paid alert
  analysis stays an optional, discoverable extra.
- User taps a longer chart range -> explain the local retention state without implying a
  monitoring capacity limit.
- User wants remote access to the web UI -> recommend their own VPN or tunnel, never retired Relay.

### 6. No self-hosted monitored-system overflow gate
Self-hosted Community users should not need a temporary monitored-system overflow path because
core self-hosted monitoring is included by default and not metered. Explicit
commercial surfaces explain Pro and Business value for alert investigation,
governed fixes, team controls, and organization seat/support scale.

### 7. Transparent monitored-system ledger
The ledger remains important for inventory truth, hosted/MSP limits, and support. It should
show exactly which top-level systems Pulse sees, which collection path is being used, and what
is included under each system. On self-hosted Community, Pro, Business, and
legacy continuity tiers, it must not create a false "X/Y systems" paywall or
imply that users need to buy more monitoring room.

### 8. No default upsell cadence
There is no proactive self-hosted upsell cadence in v6 GA. If older compatibility settings
mention prompt reduction, treat them as legacy controls. The v6 default is already quiet unless
the user enters an explicit commercial path or has an entitlement state that needs attention.

### 9. Cloud remains unavailable until reopening proof
The public Cloud surface must state that signup is closed. Dormant Cloud prices,
caps, trial language, and implementation paths remain non-public planning
inputs until the governed reopening gate passes.

### 10. Self-service lifecycle contract

| Change | Effective time | Billing treatment |
|---|---|---|
| Community to Pro or Business | After successful checkout | New subscription |
| Monthly to annual, same tier | Immediate after explicit quote and successful payment | Prorated |
| Annual to monthly, same tier | Current paid period end | No proration |
| Continuing Relay billing-cycle change | Moves to the regular Pro price (nobody moves onto a retired Relay price) | Prorated for monthly to annual, period end for annual to monthly |
| Voluntary cancellation | Current paid period end | No proration |

Relay is retired and is neither a target offer nor a downgrade option from Pro.
Combined transitions use the most restrictive timing rule. Voluntary
cancellation has a seven-day recovery-only window after paid entitlement ends.
Payment failure has a separate seven-day functional grace period. Downgrades
preserve configuration, report definitions, audit records, metric history, and
generated reports. Nothing is purged because a plan ended.

---

## v5 Customer Migration

- Existing Pro customers keep their current recurring price until they cancel
- Auto-exchange to v6 license format on binary upgrade
- If subscription lapses and they return, they come back at v6 rates
- Exchange endpoint: `POST /v1/licenses/exchange`
- Once migrated: renewal emails suppressed, legacy JWT disabled

## Self-Hosted Cap Migration

- There is no v6 self-hosted monitored-system capacity migration for Community, Relay, Pro, or Pro+
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

### Main pricing page (self-hosted, 3 columns)

```
  Community     Pro             Business
  $0            $79/yr          $399/yr
  Included      Included        Included
  monitoring    monitoring      monitoring

  [Get Started] [Choose Pro]    [Choose Business]
```

Below the table:
- "Need managed hosting? -> See Cloud plans" (signup currently closed)
- "Managing clients? -> See MSP plans"
- "Need a custom commercial deployment? -> Contact us"

### Cloud page (3 columns, dormant)

```
  Starter       Power           Max
  $29/mo        $49/mo          $79/mo
  10 agents     30 agents       75 agents
```

### MSP page (4 self-serve tiers + Enterprise)

```
  Solo          Starter         Growth          Scale           Enterprise
  $69/mo        $149/mo         $249/mo         $399/mo         Custom
  3 clients     5 clients       15 clients      40 clients      40+ clients
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
- **Pro:** Pulse does not just watch your infrastructure. It helps operate it.
- **Business:** The same product Pro runs, sized for a company team with unlimited named team seats and a next-business-day support target.
- **Retired Relay (existing subscribers only):** Relay is no longer sold. Existing subscribers keep their price and receive Pro features for as long as their subscription continues.
- **Pro+:** Legacy continuity tier for existing holders.

### Boundary-upgrade copy

- **Community -> Pro:** Want Pulse to do more than alert? Upgrade to Pro for hands-on Patrol modes, issue investigation, governed fixes, and verified outcomes.
- **Pro -> Business:** Running Pulse for a larger team? Business keeps the exact Pro feature set with unlimited named team seats and a next-business-day support target.
- **Existing self-hosted monitoring:** Your monitored systems keep working. Paid self-hosted tiers add operations and team scale, not more monitoring room.

---

## Stripe Price IDs

> Updated 2026-11-27 to reflect the 2026-08-20 Business launch and 2026-09-29 Relay retirement.

### Self-Hosted

> Public checkout offers Pro (3 named team seats on new subscriptions) and
> Business (annual-only, unlimited named team seats). Prior pre-Business Pro
> prices and retired Relay prices remain active for continuing subscriptions
> only (`public_checkout` off). Continuing Relay subscriptions are projected to
> Pro entitlements at their existing Stripe price. Pro+ is a continuity tier,
> not a public self-hosted checkout column.

- Pro Monthly (current public checkout, 3 named team seats): `price_1U6WQvBrHBocJIGHYqXOjFWm` ($8.99/mo)
- Pro Annual (current public checkout, 3 named team seats): `price_1U6WQvBrHBocJIGHxMjt6t41` ($79/yr)
- Business Annual (current public checkout, annual only, unlimited named team seats): `price_1U6WQuBrHBocJIGH0ThY3XAc` ($399/yr)
- Pro Monthly (pre-2026-08-20 continuity, not public checkout): `price_1TPmE6BrHBocJIGHHaPwluoM` ($8.99/mo)
- Pro Annual (pre-2026-08-20 continuity, not public checkout): `price_1TPmE6BrHBocJIGHR8bMvjK8` ($79/yr)
- Retired Relay Monthly (existing subscribers only, projected to Pro entitlements, not public checkout): `price_1TPmE5BrHBocJIGHdwLp4tTA` ($4.99/mo)
- Retired Relay Annual (existing subscribers only, projected to Pro entitlements, not public checkout): `price_1TPmE5BrHBocJIGH7P6JgMHP` ($39/yr)
- Pro+ renewal/continuity prices: `price_1T51LIBrHBocJIGHkUjg7sgO` ($18/mo), `price_1T51LIBrHBocJIGHvVaoGsGF` ($149/yr), not public checkout

### Cloud (created 2026-02-28)
- Cloud Starter Monthly: `price_1T5kflBrHBocJIGHUqPv1dzV` ($29/mo)
- Cloud Starter Annual: `price_1T5kfmBrHBocJIGHTS3ymKxM` ($249/yr)
- Cloud Starter Founding: `price_1T5kfnBrHBocJIGHATQJr79D` ($19/mo — first 100 signups)
- Cloud Power Monthly: `price_1T5kg2BrHBocJIGHmkoF0zXY` ($49/mo)
- Cloud Power Annual: `price_1T5kg3BrHBocJIGH2EtzKofV` ($449/yr)
- Cloud Max Monthly: `price_1T5kg4BrHBocJIGHHa8Ecqho` ($79/mo)
- Cloud Max Annual: `price_1T5kg5BrHBocJIGH5AIJ4nVc` ($699/yr)

### MSP
> Provider-hosted MSP Solo, Starter, Growth, and Scale are live self-serve plans
> via **Plan** in the provider portal after the 60-day evaluation. MSP
> Enterprise, white-label branding, and optional Pulse-hosted MSP remain
> request-assisted.

- MSP Solo ($69/mo or $690/yr, up to 3 client workspaces, live provider-portal Stripe checkout)
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

- [x] Treat self-hosted Community / Pro / Business and legacy Relay / Pro+ continuity defaults as core monitoring included without a monitored-system volume gate
- [x] Preserve grandfathered v5 recurring plans as not-metered continuity states while subscriptions remain active
- [x] Keep hosted Cloud / MSP capacity out of static self-hosted tier defaults and in plan-specific license claims
- [ ] Keep refining monitored-system identity and ledger truth for inventory, hosted/MSP limits, and support workflows

### Frontend

- [x] Remove self-hosted monitored-system capacity pressure from billing and pricing surfaces
- [x] Present the public self-hosted plans as Community, Pro, and Business (with Relay retired from public checkout)
- [ ] Keep ledger and inventory language focused on what Pulse monitors, not paid capacity pressure
- [ ] Keep paid prompts out of ordinary self-hosted runtime surfaces. Commercial copy belongs in explicit pricing, activation, recovery, hosted, or entitlement-aware paths

### License server / checkout / landing pages (`pulse-pro`)

- [x] Create self-hosted Stripe prices for Pro (3-seat new checkout) and Business ($399/yr annual-only) while preserving prior Pro and retired Relay continuity
- [x] Update plan mappings, checkout flows, and renewal-safe migration logic without disturbing grandfathered v5 continuity
- [x] Keep Pro+ and retired Relay out of public checkout while projecting continuing Relay subscriptions to Pro entitlements
- [x] Cut the landing page, checkout copy, and purchase surfaces over to core-monitoring-included language and current self-hosted plans

### Cloud / MSP

- [x] Launch self-serve 60-day evaluation and provider-portal Stripe checkout for provider-hosted MSP Solo, Starter, Growth, and Scale
- [ ] Keep MSP Enterprise, white-label branding, and optional Pulse-hosted MSP request-assisted until explicitly expanded
- [ ] Keep current Cloud list pricing dormant until the governed Cloud reopening gate passes
- [ ] Continue differentiating Cloud / MSP limits via plan-specific license claims rather than self-hosted static bands

---

## Review Checkpoints

### 30-day post-launch review
- Community -> Pro and Community -> Business opt-in purchase rate
- Continuing Relay retention and entitlement continuity
- Which explicit commercial handoffs are used most (pricing, activation, recovery, hosted)
- Support load per product

### 60-day post-launch review
- Churn by product
- Revenue per user (actual vs projected)
- Cloud margin analysis
- MSP pipeline health
- Pricing adjustment decisions (if needed)

---

## Revision History

| Date | Change | Author |
|---|---|---|
| 2026-11-27 | Aligned canonical pricing spec with shipped commercial state: recorded Relay retirement (left public checkout 29 September 2026, retires with Pulse Mobile 31 March 2027, app-only rather than web remote access, continuing subscribers keep their price with Pro entitlements), documented the launched Business tier ($399/yr annual-only, unlimited seats, next-business-day support target, 3-seat cap on new Pro checkouts), updated self-service lifecycle rules, and aligned provider-hosted MSP Solo ($69/mo or $690/yr), 60-day evaluation, and self-serve provider-portal checkout. | Pulse Maintainer |
| 2026-09-19 | Clarified that independent customer-owned installations may be administered by contractors without an MSP agreement. Reserved MSP for provider-owned entitlements across client workspaces and preserved the prohibition on pooling standard subscriptions. | Richard |
| 2026-07-10 | Made public marketing product-led rather than release-led. Version identifiers remain available for version-sensitive lifecycle tasks and technical contracts, but no longer frame homepage, product, pricing, or acquisition copy. | Richard |
| 2026-07-10 | Reframed Community, Relay, and Pro as distinct job-based product choices rather than a good/better/best ladder. Removed product recommendations from public pricing while preserving Relay connectivity as a bundled Pro entitlement. | Richard |
| 2026-06-02 | Reconciled MSP pricing evidence with the provider-operated architecture: signed MSP license, Stripe-free provider control plane, isolated Pulse runtime per client, 5/15/40 client workspace caps, and request-assisted access until launch approval. | Richard |
| 2026-04-29 | Replaced stale capacity-style monitoring phrasing with core-monitoring-included language across active v6 docs and upgrade-return copy so Community does not read like a former capacity upsell. | Codex |
| 2026-04-23 | Removed stale self-hosted monitored-system capacity and Pro+ public-checkout language. Reaffirmed Community / Relay / Pro as current public self-hosted tiers, with Pro+ as continuity only and Pro value centered on operations, history, and admin controls. | Codex |
| 2026-03-17 | Re-locked the self-hosted commercial model around monitored systems rather than installed agents. New self-hosted public pricing: Relay $4.99/$39, Pro $8.99/$79, Pro+ $14.99/$129. Added free-tier grace policy and marked the monitored-system counting migration as still required in code. | Codex + Richard |
| 2026-02-25 | Initial v6 pricing structure finalized | Richard + Claude + Codex |
| 2026-02-25 | Changed counting to agents-only model. Only installed Pulse Unified Agents count toward limits. PVE/PBS/PMG/Docker/K8s connections and discovered resources don't count. This makes limits much more generous in practice (5 agents can monitor an entire multi-node cluster). | Richard + Claude |
| 2026-02-26 | Updated Code Changes Required section — marked Pulse core backend and frontend as DONE, narrowed remaining work to license server (pulse-pro) and cloud control plane. | Claude |
| 2026-02-26 | Marked license server work as DONE (Stripe products created, max_nodes→max_agents complete, plan definitions configured, tests added). Only Cloud control plane Stripe products remain. | Claude |
| 2026-02-27 | Frontend gating audit completed. Backend enforcement is correct for all features. Historical frontend UX gaps were identified before the later free-first commercial-surface policy narrowed where upgrade handoffs belong. Details in `feature-capability-audit-2026-02.md` section 9. | Claude |
