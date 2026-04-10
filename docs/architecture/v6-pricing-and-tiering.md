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
   door, but constrained enough that serious users feel upgrade pressure naturally.
2. **Gate on action, not information.** Self-hosted AI Patrol is BYOK in steady state,
   with one Patrol-only quickstart allowance for activated or trial-backed installs during
   first-run activation. We never cap how many times users can run Patrol through their
   own API keys. The paid gate is on auto-execution of fixes, not on analysis or
   suggestions.
3. **Smooth upgrade ladder.** No large price gaps. Every step up has a clear reason.
4. **Simple to understand.** A homelabber should know which tier is right for them in
   under 10 seconds.

---

## Counted Unit

**Rule:** Pulse sells monitored coverage. The counted unit is a **monitored system**, not
an installed agent.

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
- Agent-backed monitoring and API-backed monitoring consume the same cap
- If the same underlying system is seen by both an agent and an API connection, it counts once
- Deduplication must follow canonical unified-resource identity rather than transport-specific state

**Why this model:** charging by monitored systems matches the product Pulse is actually
becoming. It closes the API loophole, keeps the commercial model honest, and still lets
Pulse include all child resources under the counted system for free.

**Definition (shown on pricing page, add-system UI, and docs):**
> "A counted system is a top-level machine or cluster Pulse actively monitors. Each monitored
> system counts once toward your plan limit, no matter how Pulse collects it. Everything under
> that system — VMs, containers, pods, disks, backups, and services — is included."

**Counting stability:** a monitored system should only begin consuming the cap after it is
stable enough to appear as a durable monitored root. Existing offline systems should release
their slot only after a deliberate grace period, not on transient disconnects.

**Transparent ledger:** in-product UI must show the exact counted systems, their collection
path, and their first-seen / last-seen state so users can understand why they are at X/Y.

**Implementation transition note:** the current runtime still enforces `max_agents` and
agent-backed counting in some paths. That is a transitional compatibility boundary, not the
long-term commercial model. The canonical v6 destination is monitored-system counting.

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
| Monitored systems | **5** |
| Onboarding overflow | +1 monitored system for 14 days (one-time per workspace) |
| Monitoring | Full (Proxmox, Docker, K8s, agents) |
| Alerts | Threshold-based |
| SSO | Basic OIDC |
| Update notifications | Yes |
| Metrics history | **7 days** |
| AI Patrol | Monitor + root-cause summaries + fix suggestions (BYOK by default, with Patrol quickstart on activated or trial-backed installs) |
| AI Quickstart Credits | **25 hosted Patrol runs** (activated or trial-backed installs only; no API key needed, Patrol only, powered by MiniMax 2.5M) |
| AI Auto-Fix | **No** (must apply fixes manually) |
| AI Alert Analysis | No |
| Relay | No |
| Push notifications | No |
| Custom URL | No |
| RBAC | No |
| Audit logging | No |
| SAML SSO | No |
| Agent profiles | No |
| PDF/CSV reporting | No |

**AI Patrol in free tier:** Two modes of operation:

1. **Quickstart Credits (Patrol only, no API key needed):** Activated or trial-backed
   installs with the server-verified installation identity get 25 hosted Patrol runs
   powered by MiniMax 2.5M. This gives users a bounded way to try Patrol before adding a
   provider. Unactivated Community installs start a trial, activate, or use their own API
   key instead. After credits are exhausted, users connect their own API key to continue
   self-hosted Patrol. Quickstart is activation support only: it is not a general hosted
   chat entitlement and it does not replace BYOK for ongoing self-hosted AI use.

2. **BYOK (after credits or by choice):** Users provide their own API key
   (OpenAI/Anthropic/etc.). No commercial quota — it's their API key, their money. Only
   technical guardrails (anti-loop, rate protection).

In both modes, the gate is on AUTO-EXECUTION: free users see the full analysis and fix
suggestions but must apply fixes manually. Pro users click "Apply Fix."

**Quickstart credit economics:**
- Model: MiniMax 2.5M (selected for cost-effectiveness)
- Estimated cost: $0.002–0.01 per Patrol run
- Budget per 10,000 users (all 25 credits used): $500–2,500
- This is customer acquisition cost, not ongoing expense
- Cost can be further reduced with: caching repeated analyses, strict context/token caps,
  model cascade (cheap triage first, deeper analysis only when needed)

### Relay — $4.99/month or $39/year

| Element | Value |
|---|---|
| Monitored systems | **8** |
| Everything in Free | Yes |
| Relay remote access | **Yes** |
| Mobile app access | **Yes** |
| Push notifications | **Yes** |
| Custom URL | **yourlab.pulserelay.pro** |
| Metrics history | **14 days** |
| AI Auto-Fix | No |
| AI Alert Analysis | No |
| RBAC/Audit/SAML | No |
| Reporting | No |

**Positioning:** The low-friction headroom tier. It should feel cheap enough to buy on the
spot when someone is just over the Community boundary and wants remote access, mobile, and
push at the same time.

### Pro — $8.99/month or $79/year

| Element | Value |
|---|---|
| Monitored systems | **15** |
| Everything in Relay | Yes |
| AI Auto-Fix | **Yes** (one-click execution, safety preflight, rollback) |
| AI Alert Analysis | **Yes** |
| Kubernetes AI Analysis | **Yes** |
| Scheduled remediations | **Yes** |
| Execution audit trail | **Yes** |
| Metrics history | **90 days** |
| RBAC | **Yes** |
| Audit logging | **Yes** |
| SAML SSO | **Yes** |
| Agent profiles | **Yes** |
| PDF/CSV reporting | **Yes** |
| Trial | **14-day, no credit card** |

**Positioning:** For serious homelabbers who want AI to manage their infrastructure and
want full history. The marketing pitch focuses on three things:
1. "AI that fixes your infrastructure" (Patrol auto-fix)
2. "Monitor from anywhere" (Relay, inherited)
3. "See your full history" (90-day metrics)

Enterprise features (RBAC, audit, SAML) are included but positioned as bonus value, not
the headline.

### Pro+ — $14.99/month or $129/year

| Element | Value |
|---|---|
| Monitored systems | **50** |
| Everything in Pro | Yes |
| Trial | **14-day, no credit card** |

**Positioning:** For power users with large homelabs (15+ agents). Simple $50/yr bump from
Pro. Anyone with 50+ agents should contact us (likely MSP or business).

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

### 1. Monitored-system cap ladder (5 → 8 → 15 → 50)
Graduated upgrade pressure as infrastructure grows. In-product UI shows "5/5 monitored systems"
with an upgrade CTA before hard block. The free tier must still fit one real small homelab end
to end, while paid tiers make boundary crossings feel easy and fair.

### 2. AI fix previews (strongest lever)
Free/Relay users see exactly what Patrol found and how to fix it (specific commands), but
must apply manually. Pro users see the same information with an "Apply Fix" button. Every
Patrol finding is a conversion moment.

### 3. AI Quickstart Credits (25 hosted Patrol runs)
Activated or trial-backed installs get 25 Patrol runs powered by MiniMax 2.5M with no API
key setup. This gives users one bounded Patrol-first activation path. Unactivated Community
installs activate, start a trial, or connect their own key (BYOK, stays free). Upgrading
later unlocks auto-fix, alert analysis, and other paid operations features; it does not
replace BYOK for self-hosted AI runtime. Cost: ~$0.002–0.01 per run.

### 4. Relay as impulse buy ($39/yr)
Fills the $0 → Pro gap. Relay is not the automation tier; it is the cheap convenience and
headroom tier. Remote access + mobile + push notifications + custom URL + three extra counted
systems should make it an easy purchase for users sitting on the Community boundary.

### 5. Contextual trial triggers
14-day Pro trial offered at moments of maximum desire:
- Patrol finds a fixable issue → "Apply this fix automatically? Start your free trial"
- User hits monitored-system cap → "Need more room? Start your free trial"
- User taps 30-day chart range → "See your full history — start your free trial"
- User tries Relay from free tier → "Monitor from anywhere — upgrade to Relay ($49/yr) or
  start a Pro trial"
- 7+ days of active use → proactive "Experience the full power" nudge

### 6. Onboarding overflow (+1 monitored system for 14 days)
New free users get a temporary 6th counted-system slot for their first 14 days. Prevents hard-wall
frustration during initial setup. One-time per workspace.

### 7. Transparent counted-system ledger
Always visible in the UI: "5/5 monitored systems." Link to the counted-system ledger showing
exactly what's counted, which collection path is being used, and what is included under each
system. Upgrade CTA appears before hard block, not
after.

### 8. Upsell snooze (7-day, not permanent)
Users can snooze upgrade prompts for 7 days. No permanent mute option. Power users who
use the product heavily are highest-potential converters — don't let them opt out forever.

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

## Free-Tier Cap Migration

- Existing free users who end up above the new counted-system cap must not be hard-broken on rollout day
- Existing monitored coverage should continue during a defined grace period
- During grace, Pulse should block only new counted-system additions until the user reduces usage or upgrades
- The UI must explain the new counting model clearly and show the exact systems consuming the cap

---

## Trial System

- **Duration:** 14 days
- **Credit card:** Not required
- **Available on:** Pro, Pro+, Cloud (all tiers)
- **Not available on:** Relay (cheap enough to just buy)
- **Features during trial:** Full Pro capabilities
- **Activation paths:**
  - Self-hosted: POST `/api/license/trial/start` initiates hosted signup and returns a control-plane `action_url`
  - Self-hosted completion: hosted control plane returns a signed token to `/auth/trial-activate`
  - Cloud: Stripe checkout with trial period
- **Restrictions:**
  - One trial per workspace
  - Rate limited: a short per-org retry burst is allowed so operators can
    revisit the hosted handoff; once that burst is exhausted the local runtime
    returns `429 trial_rate_limited` with `Retry-After` backoff metadata
  - Cannot start if already has active Pro license

---

## Pricing Page Layout

### Main pricing page (self-hosted — 4 columns)

```
  Free          Relay           Pro             Pro+
  $0            $39/yr          $79/yr          $129/yr
  5 systems     8 systems       15 systems      50 systems

  [Get Started] [Buy Relay]     [Start Trial]   [Start Trial]
```

Below the table:
- "Need managed hosting? → See Cloud plans"
- "Managing clients? → See MSP plans"
- "Need 50+ systems? → Contact us"

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

Use this exact idea everywhere pricing or cap enforcement is shown:

> "Pulse counts monitored systems, not everything underneath them. Each top-level machine or
> cluster counts once, no matter how Pulse collects it. VMs, containers, pods, disks, backups,
> and services under that system are included."

### Plan taglines

- **Community:** Monitor up to 5 systems for free.
- **Relay:** Get a bit more room and monitor from anywhere.
- **Pro:** Pulse does not just watch your infrastructure. It helps operate it.
- **Pro+:** Everything in Pro, with more room for larger labs.

### Boundary-upgrade copy

- **Community → Relay:** Need a little more room? Upgrade to Relay for 3 extra monitored systems plus remote access, mobile, and push notifications.
- **Relay → Pro:** Want Pulse to do more than alert? Upgrade to Pro for AI investigation, auto-fix, and 90-day history.
- **Free over cap grace:** Your existing monitoring will keep working for now, but new systems will not be added until you remove one or upgrade.

---

## Stripe Price IDs

> Updated 2026-02-28 with all v6 price IDs (Self-Hosted, Cloud, MSP).

### Self-Hosted

> 2026-03-17 decision: the previous self-hosted v6 public prices are superseded.
> New live Stripe prices still need to be created for the locked $4.99 / $8.99 / $14.99
> monthly bands and their annual counterparts before public checkout is cut over.

- Relay Monthly: pending new live Stripe price ($4.99/mo)
- Relay Annual: pending new live Stripe price ($39/yr)
- Pro Monthly: pending new live Stripe price ($8.99/mo)
- Pro Annual: pending new live Stripe price ($79/yr)
- Pro+ Monthly: pending new live Stripe price ($14.99/mo)
- Pro+ Annual: pending new live Stripe price ($129/yr)

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

- [ ] Replace agent-only commercial enforcement with monitored-system counting derived from canonical unified-resource roots
- [ ] Deduplicate agent-backed and API-backed monitoring of the same system into one counted unit
- [ ] Preserve child-resource inclusion semantics (VMs, containers, pods, disks, backups do not count separately)
- [ ] Introduce a migration-safe compatibility boundary from `max_agents` to a canonical counted-system limit
- [ ] Add a free-tier grace path for existing installs that end up above the new cap

### Frontend

- [ ] Rename user-facing language from agents to monitored systems where the copy is commercial rather than technical
- [ ] Replace the installed-agent ledger with a counted-system ledger
- [ ] Update pricing, paywall, and upgrade copy to the locked $0 / $39 / $79 / $129 annual ladder
- [ ] Keep the upgrade pressure points, but make the counted unit obvious and fair

### License server / checkout / landing pages (`pulse-pro`)

- [ ] Create new self-hosted Stripe prices for Relay / Pro / Pro+ at the locked public bands
- [ ] Update plan mappings, checkout flows, and renewal-safe migration logic without disturbing grandfathered v5 continuity
- [ ] Cut the landing page, checkout copy, and purchase surfaces over to monitored-system language and the new price bands

### Cloud / MSP

- [ ] Keep current Cloud/MSP list pricing unchanged for now unless a separate decision explicitly revises it
- [ ] Continue differentiating Cloud / MSP limits via plan-specific license claims rather than self-hosted static bands

---

## Review Checkpoints

### 30-day post-launch review
- Free → Relay conversion rate
- Free → Pro trial start rate
- Trial → paid conversion rate
- Relay → Pro upgrade rate
- Which paywall surfaces fire most (agent cap vs AI fix vs Relay vs history)
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
| 2026-03-17 | Re-locked the self-hosted commercial model around monitored systems rather than installed agents. New self-hosted public pricing: Relay $4.99/$39, Pro $8.99/$79, Pro+ $14.99/$129. Added free-tier grace policy and marked the monitored-system counting migration as still required in code. | Codex + Richard |
| 2026-02-25 | Initial v6 pricing structure finalized | Richard + Claude + Codex |
| 2026-02-25 | Changed counting to agents-only model. Only installed Pulse Unified Agents count toward limits. PVE/PBS/PMG/Docker/K8s connections and discovered resources don't count. This makes limits much more generous in practice (5 agents can monitor an entire multi-node cluster). | Richard + Claude |
| 2026-02-26 | Updated Code Changes Required section — marked Pulse core backend and frontend as DONE, narrowed remaining work to license server (pulse-pro) and cloud control plane. | Claude |
| 2026-02-26 | Marked license server work as DONE (Stripe products created, max_nodes→max_agents complete, plan definitions configured, tests added). Only Cloud control plane Stripe products remain. | Claude |
| 2026-02-27 | Frontend gating audit completed. Backend enforcement is correct for all features. 5 frontend UX gaps identified (ReportingPanel missing license check, several panels missing upgrade links/trial buttons). Details in `feature-capability-audit-2026-02.md` section 9 and `ENTITLEMENT_MATRIX.md` "Frontend Gating Status" section. | Claude |
