# Pulse 6.0 — Conversion & Cloud Launch (Guiding Light)

Status: ACTIVE
Owner: Pulse
Date: 2026-02-10
Revision: 3 (final — all open decisions resolved, scope tightened per adversarial review)
Applies to: The next major release (6.0) — the commercial reset

Previous guiding light (W0-W6) is complete. This document replaces it.

## Release Thesis

Pulse has 530,000 downloads and 134 paying customers. That is a 0.03% conversion rate. The product is technically strong but commercially broken. This release fixes that.

The previous release was a product-shape change (unified resources, TrueNAS, mobile, multi-tenant). This release is a **business-shape change**: new tiers, new pricing, a hosted offering, and an in-app experience that makes paying feel inevitable.

The release wins only if these outcomes land together:

1. **Free tier is great but limited** — monitoring is full-featured, AI is constrained to `monitor` autonomy (findings only), mobile requires Pro/Cloud.
2. **Pro feels inevitable** — trial is frictionless, upgrade prompts are contextual, the "aha" moment happens before any paywall.
3. **Cloud exists and is compelling** — hosted Pulse with zero-config onboarding, agent-based connectivity, managed AI that eliminates API key friction, and Stripe-powered billing.
4. **What we sell actually works** — audit logging is real, SSO gating is consistent, paywalls use entitlements not hardcoded links, and the entitlements system actually enforces billing state for hosted tenants.

If any one of these is half-done, the launch undermines trust.

## The Diagnosis (Why Conversion Is 0.03%)

Six root causes, all fixable:

| # | Problem | Evidence | Fix |
|---|---------|----------|-----|
| 1 | **Free gives away the crown jewel** | AI Patrol (BYOK) delivers the core "senior engineer watching your infra" value for $0. Users get findings + investigation + auto-fix for free. | Restrict Community to `monitor` autonomy: patrol runs and produces findings, but no investigation, no remediation, no auto-fix. |
| 2 | **Paywalls are generic and throw away intent** | Frontend hardcodes `pulserelay.pro` links everywhere (e.g. `AIIntelligence.tsx`) instead of using the `upgrade_reasons` deep links that already exist in `upgrade_reasons.go`. | Switch frontend to `/api/license/entitlements` as gating source. Use feature-specific upgrade pages. |
| 3 | **No trial exists** | Trial subscription state plumbing is incomplete: `SubscriptionState()` returns "expired" when `license==nil` even if evaluator has trial state. Users hit a paywall before feeling Pro. | Ship 14-day in-app trial with one-click activation, after fixing subscription state plumbing. |
| 4 | **Strongest conversion feature is invisible** | Relay + mobile (secure remote access without opening ports) is buried. It's the most concrete "I need this" feature for homelabbers. | Make relay onboarding a hero first-run experience with QR pairing wizard. |
| 5 | **Some paid features don't actually work** | Audit logging: `initAuditLoggerIfLicensed` is a stub with TODO comments, never creates a SQLite logger. SSO gating: `features.go` includes `sso` in Free tier but MONETIZATION.md says Pro. | Fix audit logging end-to-end. Audit every feature flag for consistency. |
| 6 | **Hosted entitlements are wired but not enforced** | `HasFeature()` in `license.go:367-368` falls back to `TierHasFeature(TierFree, feature)` when evaluator is set but `license==nil` — ignoring the evaluator entirely. `Status()` returns free-tier features. `SubscriptionState()` returns "expired". The entire hosted billing pipeline is disconnected from runtime gating. | Fix `HasFeature()`, `Status()`, and `SubscriptionState()` to delegate to evaluator when no JWT license is present. |

## Competitive Advantage (The Moat)

Pulse does not win on dashboards (Grafana wins) or breadth (Zabbix wins). Pulse wins on:

1. **"Deploy binary, get root cause analysis"** — Unified agent auto-detects everything + AI Patrol interprets it. No one else has this flow this smooth.
2. **Secure remote access without inbound firewall rules** — Relay + mobile with E2E encryption. No OSS competitor has this.
3. **Actionable intelligence, not just charts** — Patrol findings → investigation → approval → auto-fix → verification. The full operational loop.

Protect these. Everything else is secondary.

## Tier Structure

| | Community (Self-Hosted) | Pro (Self-Hosted) | Cloud (Hosted) | MSP / Enterprise |
|---|---|---|---|---|
| **Price** | Free | $15/mo or $129/yr | $29/mo flat ($19/mo founding member — first 100 signups, locked) | Sales-led |
| **Monitoring** | Full — unlimited nodes, all agents, all dashboards, all platforms | Same | Same | Same + fleet views |
| **AI Patrol** | BYOK, `monitor` autonomy only: findings visible, no investigation, no remediation, no auto-fix | Full: all autonomy levels (approval/assisted/full) + auto-fix (BYOK) | Full + **managed AI** (no API key needed — toggle Patrol on) | Full + managed AI |
| **Retention** | 7 days (existing `long_term_metrics` gate) | 90 days | 90 days | Configurable |
| **Mobile + Relay** | No mobile (relay is Pro/Cloud) | Full mobile + relay access | Built-in (no relay setup) | Built-in |
| **Team Features** | — | RBAC + audit logging + reporting + SSO | Same | Same + multi-tenant + delegated access |
| **Support** | Community | Email | Priority | Dedicated |
| **Trial** | — | 14 days, in-app activation, full Pro capabilities | 14-day free trial at signup | Custom |

### Key Design Decisions

1. **Free monitoring is unlimited.** This preserves the viral loop — 530K downloads came from "install it, it just works." Never gate basic monitoring.
2. **AI gating uses autonomy levels, not new capability keys.** Community is locked to `monitor` autonomy. Pro/Cloud unlock `approval`, `assisted`, and `full`. This reuses the existing autonomy model (`internal/config/ai.go`) and avoids capability-key explosion. No new keys (`ai_patrol_investigate`, `ai_patrol_autofix`) are needed — the existing `ai_autofix` key plus autonomy level checks are sufficient.
3. **Mobile requires Pro/Cloud.** The mobile app is architecturally relay-based (`qrCodeParser.ts` requires `wss://` relay_url). There is no "direct LAN connect" mode and building one is a large project. Rather than promise a "LAN only" mode that doesn't exist, mobile/relay is a Pro/Cloud feature. This makes mobile the hero conversion feature: "Monitor your homelab from your phone, from anywhere, without opening ports."
4. **Managed AI requires Cloud infrastructure to be stable first.** Managed AI (Pulse proxies LLM calls) requires per-tenant budgets, abuse controls, cost attribution, and model routing. These depend on Cloud infrastructure (P2) being solid. Implementation order: deploy Cloud with BYOK → verify stability → ship managed AI with full abuse controls. This is a dependency chain, not a scope cut.
5. **Cloud is additive, not a pivot.** Self-hosted users keep getting everything they had. Cloud captures the convenience segment that would never have self-hosted.
6. **No new capability keys unless they map to a single runtime gate and a single upgrade reason.** The existing keys in `features.go` (`ai_patrol`, `ai_autofix`, `relay`, `long_term_metrics`, etc.) are sufficient. Retention gating continues to use the existing `long_term_metrics` feature key, not a new `max_retention_days` limit key.

## Workstreams

### P0: Trust Foundation (Must Ship — Blocks Everything)

These are bugs and gaps that undermine the ability to sell anything. Fix before all else.

#### P0-0: Fix hosted entitlements enforcement

**Problem:** The hosted entitlement pipeline is wired (billing state → DatabaseSource → Evaluator → Service.SetEvaluator) but the Service methods ignore the evaluator when `s.license == nil`:
- `HasFeature()` at `license.go:367-368`: returns `TierHasFeature(TierFree, feature)` instead of consulting evaluator
- `Status()` at `license.go:486-488`: returns free-tier features without checking evaluator
- `SubscriptionState()` at `license.go:462-463`: returns `"expired"` instead of asking evaluator
- `/api/license/entitlements` handler at `entitlement_handlers.go:89-92`: builds payload from `svc.Status().Features` (tier + Claims.Features), not from evaluator capabilities

Every route that uses `HasFeature()` (which is most feature gates) will behave as Community tier for hosted tenants even when billing state grants Pro/Cloud capabilities. The entitlements endpoint will report different capabilities than what `HasFeature()` actually enforces.

**Scope:**
- `internal/license/license.go` — fix `HasFeature()` to delegate to `evaluator.HasCapability()` when `license==nil` and evaluator is set
- `internal/license/license.go` — fix `SubscriptionState()` to delegate to `evaluator.SubscriptionState()` when `license==nil`
- `internal/license/license.go` — fix `Status()` to populate features/tier/validity from evaluator when `license==nil`
- `internal/api/entitlement_handlers.go` — when evaluator is available, build `EntitlementPayload` from evaluator capabilities/limits/subscription_state rather than from `Status().Features`
- Update the `upgrade_reasons` matrix in `internal/license/conversion/` for any new gates introduced in this release

**Regression protection:** `HasFeature()` is called by every feature gate in the system. A regression here locks users out of features. Add a unit test matrix in `license_test.go` covering all four combinations:
1. `license==nil`, `evaluator==nil` → free tier behavior
2. `license==nil`, `evaluator!=nil` → evaluator drives (hosted path)
3. `license!=nil`, `evaluator==nil` → JWT drives (self-hosted path)
4. `license!=nil`, `evaluator!=nil` → JWT takes precedence (hybrid)

**Exit criteria:**
- A hosted tenant with billing state `{capabilities: ["ai_patrol", "ai_autofix", "relay"]}` has `HasFeature("ai_autofix") == true`
- `/api/license/entitlements` returns capabilities matching evaluator state, not free-tier defaults
- `SubscriptionState()` returns the evaluator's subscription state for hosted tenants
- Parity test: for every capability, `HasFeature(cap)` agrees with the capabilities list in `/api/license/entitlements`
- All four unit test matrix combinations pass

#### P0-1: Make audit logging real and tenant-aware

**Problem:** `initAuditLoggerIfLicensed` in `license_handlers.go:106-140` is a stub — the `auditOnce.Do` block has extensive TODO comments but never creates a SQLite logger. The tenant audit manager in `server.go:116-119` always uses the default factory (console logger). There is no `SQLiteLoggerFactory` for tenants. In a multi-tenant environment, a singleton global logger would interleave or expose cross-tenant audit events.

**Implementation note:** There is an interface mismatch to resolve. `LoggerFactory.CreateLogger(dbPath string)` in `pkg/audit/tenant_logger.go:20` takes a path string, but `NewSQLiteLogger()` in `pkg/audit/sqlite_logger.go:37` takes a `SQLiteLoggerConfig{DataDir: ...}` (a directory, not a file). The `SQLiteLoggerFactory` must bridge this — e.g., `CreateLogger(dbPath)` extracts the directory from the path and passes it as `DataDir` to `NewSQLiteLogger`.

**Decision: create SQLite loggers for all orgs, gate the query/export endpoints behind license.**
Creating the logger unconditionally means audit events are always captured (defense in depth). Unlicensed orgs simply can't query or export them. This is simpler than conditional logger creation and avoids the race where events happen before a license is activated.

**Scope:**
- `pkg/audit/` — implement `SQLiteLoggerFactory` that creates per-tenant SQLite loggers (one DB per org, directory: `<orgDir>/`, DB created by `NewSQLiteLogger` inside that directory)
- `pkg/audit/tenant_logger.go` — replace `DefaultLoggerFactory` (console) with `SQLiteLoggerFactory` as the default when data directory is available
- `internal/api/license_handlers.go` — remove the stub `initAuditLoggerIfLicensed` (no longer needed if factory handles it)
- `pkg/server/server.go` — wire `SQLiteLoggerFactory` into tenant audit manager initialization
- Gate query/export endpoints (`/api/audit/*`) behind `audit_logging` feature key (keep `RequireLicenseFeature` on those routes)
- End-to-end test: write audit events → verify events persisted to SQLite → verify queryable via API → verify HMAC signature → verify export works
- Verify tenant isolation: org A's audit events are not visible to org B

**Exit criteria:** A tenant's audit events are persisted to a per-org SQLite database. Licensed tenants can query and export. Unlicensed tenants' events are captured but not queryable. Tenant isolation is verified.

#### P0-2: Switch frontend to entitlements endpoint (after P0-0)

**Problem:** Frontend gates features via `/api/license/status` and hardcodes `pulserelay.pro` upgrade links. Backend already exposes `/api/license/entitlements` with capabilities, limits, subscription state, and upgrade reasons with deep links (`upgrade_reasons.go`). The frontend doesn't use any of it.

**Prerequisite:** P0-0 must be complete first. The entitlements endpoint must be canonical (derived from evaluator when available) before the frontend can rely on it. Otherwise you'll ship paywalls that disagree with backend enforcement.

**Scope:**
- `frontend-modern/src/` — replace all `license.ts` gating with entitlements consumption
- Replace every hardcoded `pulserelay.pro` link with `upgrade_reasons[].action_url`
- Wire feature-specific upgrade pages: `/pricing?feature=ai_patrol`, `/pricing?feature=relay`, etc.
- Verify: for each feature gate in the frontend, `HasFeature()` on the backend and the entitlements response agree

**Exit criteria:** Every paywall in the UI is contextual (shows what the user tried to do and why it requires Pro) and links to a feature-specific upgrade page, not a generic landing page. No remaining hardcoded `pulserelay.pro` links in frontend source.

#### P0-3: Fix SSO gating consistency

**Problem:** `features.go` includes `FeatureSSO` ("sso") in the Free tier (`TierFeatures[TierFree]`), but MONETIZATION.md and landing page claim SSO is Pro. This creates contradictory upgrade messaging — users see "Upgrade for SSO" but SSO actually works on Free.

**Decision: Basic SSO (OIDC) is free.** It's table stakes in 2026. The code already has it in `TierFeatures[TierFree]` — the bug is that docs/landing page say otherwise. Fix the docs, not the code. Advanced SSO (SAML, multi-provider, role mapping) remains Pro.

**Scope:**
- Audit every feature constant in `internal/license/features.go` against intended tier placement
- Update MONETIZATION.md, landing page, and in-app copy to reflect that basic SSO is free
- Produce a single source-of-truth entitlement matrix (can be a table in this document or a dedicated `ENTITLEMENT_MATRIX.md`)
- Ensure code, docs, landing page, and in-app copy all agree
- Remove any upgrade reasons for features that are actually free (specifically: remove SSO from upgrade prompts)

**Exit criteria:** A single canonical document defines what's free vs. paid, and code enforcement matches exactly. No upgrade prompts for features the user already has.

#### P0-4: Fix hosted signup auth flow

**Problem:** `hosted_signup_handlers.go` collects email + password but the password is thrown away. The handler creates an RBAC entry for the user but never hashes/stores the credential. A user who completes signup gets a success response but cannot log in.

**Decision: Passwordless magic links.**
Cloud users won't have an enterprise IdP, and we don't have (or want) a credential store. Magic links are lower friction than passwords, eliminate the "password thrown away" bug entirely, and are the standard for modern SaaS onboarding.

**Scope:**
- `internal/api/hosted_signup_handlers.go` — remove password field from signup request; collect email only
- Implement magic link flow: signup with email → send email with signed token link → click link → session created → redirect to dashboard
- Token: short-lived (15 min), single-use, signed with HMAC
- For returning users: "Log in" sends a new magic link (same flow)
- Remove the unused password handling code entirely
- Rate limit: max 3 magic link requests per email per hour (prevent email spam)

**Exit criteria:** A user who enters their email at hosted signup receives a magic link within 30 seconds, clicks it, and lands in their tenant dashboard. No password is ever collected or stored.

#### P0-5: Wire trial subscription state end-to-end

**Problem:** Trial state is partially plumbed but not functional:
- `SubscriptionState()` does not read evaluator state when `license==nil` (fixed in P0-0)
- `EntitlementPayload.TrialDaysRemaining` only computes when `subscription_state == "trial"` AND `status.ExpiresAt != nil` — but hosted trial tenants have no `ExpiresAt` in the license sense
- There is no mechanism to start a trial (no endpoint, no state transition)
- `BillingState` struct (`internal/license/entitlements/billing_store.go`) has no `TrialEndsAt` field — schema change required

If we restrict the free tier (P1-3) without offering a trial first, we spike backlash and lose adoption. Trial plumbing is a prerequisite for safe tier restriction.

**Decision: One trial mechanism for both self-hosted and hosted — local billing state.**
- Self-hosted users click "Start Trial" → writes `billing.json` locally → evaluator reads it → `HasFeature()` grants Pro capabilities. No license server roundtrip. No phone-home.
- Hosted users get the same mechanism but billing state is managed server-side.
- If a JWT license is also present, it takes precedence (existing TokenSource override behavior).
- This avoids maintaining two trial paths (JWT + billing state) which doubles testing surface and creates divergent behavior.

**Scope:**
- `internal/license/entitlements/billing_store.go` — add `TrialEndsAt *int64` and `TrialStartedAt *int64` fields to `BillingState` struct
- `internal/license/entitlements/database_source.go` — compute trial expiry from `TrialEndsAt`; when expired, set `SubscriptionState` to `"expired"` and strip Pro capabilities
- Wire trial activation endpoint: `POST /api/license/trial/start` — creates/updates billing state with `subscription_state: "trial"`, `trial_ends_at: now+14d`, `capabilities: [all Pro caps]`
- **Abuse protection on trial endpoint:**
  - 1 trial per org, ever (check `trial_started_at != nil` before allowing)
  - Rate limit: 1 request per org per 24h (prevents retry spam)
  - Tied to org identity (not unauthenticated)
- For self-hosted default org: `POST /api/license/trial/start` writes to `<configDir>/billing.json`. The evaluator in `getTenantComponents()` must also be wired for the default org when trial is active (currently only wired for hosted non-default orgs).
- Ensure `EntitlementPayload` computes `TrialDaysRemaining` from `BillingState.TrialEndsAt` (not from `Status().ExpiresAt`)
- Frontend: trial countdown component that reads `trial_days_remaining` from entitlements

**Exit criteria:** A free user can start a trial with one click. `SubscriptionState()` returns `"trial"`. Entitlements response includes `trial_days_remaining`. Trial expires after 14 days and subscription state reverts to `"expired"`. A second trial attempt is rejected. Self-hosted and hosted use the same mechanism.

#### P0-6: Make conversion telemetry persistent and tenant-aware

**Problem:** Conversion telemetry is a go/no-go launch gate (see Non-Negotiable Contract below) but the current implementation in `recorder.go` hardcodes tenant `"default"` and writes to an in-memory aggregator. Events are lost on restart. You cannot measure conversion if you cannot persist conversion events.

**Scope:**
- `internal/license/conversion/recorder.go` — replace in-memory aggregator with persistent storage (SQLite, one table)
- Make tenant-aware: record `org_id` on every event
- Events to persist: `paywall_viewed`, `trial_started`, `upgrade_clicked`, `checkout_completed`
- Basic query endpoint: `GET /api/admin/conversion-funnel?org_id=...&from=...&to=...`
- Must be operational from day one of launch

**Exit criteria:** Conversion events are persisted across restarts, queryable by org and time range, and include all four event types.

### P1: Conversion Engine (Makes Pro Feel Inevitable)

Requires P0 complete. These items make free users want to pay.

#### P1-1: Ship in-app Pro trial

**Prerequisite:** P0-5 (trial state plumbing) must be complete.

**Scope:**
- Frontend: "Start 14-day Pro trial" button at high-intent moments (relay pairing screen, AI patrol findings, RBAC setup, autonomy settings)
- Trial countdown UI in sidebar/header during trial period (reads `trial_days_remaining` from entitlements)
- Trial expiry: hard-stop (features revert to Community), clear upgrade prompt
- Analytics: emit `trial_started` and `trial_expired` conversion events

**Exit criteria:** A free user can start a trial with one click, experience full Pro for 14 days, and see a clear upgrade prompt when trial expires.

#### P1-2: Make relay onboarding a hero experience

**Scope:**
- Dashboard: prominent "Pair Mobile Device" card in first-run / empty-state
- QR code wizard: scan → connect → "send test push notification" → "check remote connection"
- Gate clearly: "Mobile access requires Pro or Cloud. Start your free 14-day trial to pair your device now."
- Landing page: elevate relay + mobile in messaging (currently buried)

**Exit criteria:** A new user's first encounter with relay is guided, impressive, and creates immediate desire to pay for mobile access. Relay pairing completion rate > 60% of users who open the wizard.

#### P1-3: Restrict AI Patrol in Community tier

**Prerequisite:** P1-1 (trial) should be available before restricting free capabilities, so users have an escape valve.

**Implementation approach:** Gate via autonomy levels, not new capability keys.

**Decision: "reinvestigate" IS investigation and is gated behind Pro.**
Current routes in `router_routes_ai_relay.go:113-130` annotate "viewing and reinvestigation are free." This must change. Reinvestigation triggers the full investigation pipeline (`MaybeInvestigateFinding`). Community gets read-only access to findings: view list, view details, view history. Community does NOT get: investigate, reinvestigate, approve fixes, execute fixes.

**Community can:**
- View all patrol findings (list + detail)
- View finding history and status
- Configure AI provider (BYOK API key)
- See patrol run results

**Community cannot:**
- Set autonomy above `monitor`
- Trigger investigation or reinvestigation
- Approve or reject fix proposals
- Execute auto-fix or remediation plans
- Access autonomous mode

**Scope:**
- `internal/ai/patrol.go` / `internal/config/ai.go` — Community tier locked to `monitor` autonomy level
- Enforce in autonomy settings UI: Community users can only select `monitor`; `approval`/`assisted`/`full` show upgrade prompt
- `MaybeInvestigateFinding()` in `patrol_findings.go` — already checks autonomy level; no change needed if `monitor` prevents investigation
- Autonomy endpoints in `router_routes_ai_relay.go` — gate `approval`/`assisted`/`full` behind Pro license
- **Investigation/reinvestigation routes in `router_routes_ai_relay.go:113-130`** — change annotation from "free" to Pro-gated. Add `RequireLicenseFeature(handlers, "ai_autofix", ...)` or equivalent check on reinvestigation endpoints.
- In-app: when Community user views a finding, show "Upgrade to Pro to investigate and auto-fix this finding"
- Cadence cap: 1 patrol run per hour for Community (existing patrol scheduling can enforce this)

**Existing gates to verify (not new, just confirm they work):**
- `ai_autofix` feature key already gates auto-fix execution — confirm this is Pro-only in `features.go` ✓
- `ai_patrol` feature key is in Free tier (correct — patrol runs are free, outcomes are limited)

**Exit criteria:** Community users see patrol findings but cannot investigate, reinvestigate, or auto-fix. Autonomy settings show `monitor` only with upgrade prompts for higher levels. Reinvestigation endpoints return 402 for Community users. Upgrade path is one click to trial.

### P2: Cloud Platform (Full Hosted Offering)

Cloud is a complete hosted Pulse experience: deploy nothing, monitor everything, with managed AI.

#### P2-1: Deploy hosted Pulse instance

**Scope:**
- DigitalOcean droplet running Pulse with `PULSE_HOSTED_MODE=true`
- Caddy reverse proxy with TLS at `app.pulserelay.pro`
- Daily backups (extend existing backup workflow)
- Monitoring/alerting on the hosted instance itself

**Exit criteria:** `app.pulserelay.pro` serves a running multi-tenant Pulse instance with TLS.

#### P2-2: Connect Stripe → org provisioning

**Scope:**
- License server: new Stripe webhook handler for Cloud checkout
- **Critical: implement Stripe webhook signature verification immediately** — without it, anyone can provision free instances by curling the webhook endpoint
- On `checkout.session.completed`: call hosted signup API → create org → write billing state
- On subscription change/cancel: update billing state → entitlements flow automatically
- **Must handle `customer.subscription.deleted` specifically** — revoke access immediately when a Cloud user churns. This is often missed in "happy path" implementation. Set `subscription_state: "canceled"`, strip Pro capabilities from billing state.
- Stripe Customer Portal for self-service subscription management

**Exit criteria:** User completes Stripe checkout → org is provisioned → user can log in and start monitoring within 60 seconds. Webhook endpoint rejects unsigned requests. Subscription cancellation revokes access within 60 seconds.

#### P2-3: Agent onboarding for Cloud tenants

**Scope:**
- Cloud dashboard: "Add a node" generates a one-liner install script with tenant-specific auth token
- Agent connects outbound to hosted instance (no inbound firewall rules)
- First agent connection triggers initial patrol run + sends first finding

**Exit criteria:** Cloud tenant can add a node to monitoring with a single copy-paste command.

#### P2-4: Managed AI provider for Cloud

**Prerequisite:** P2-1/P2-2/P2-3 must be stable. This is the feature that makes Cloud worth $29/mo — "toggle Patrol on, no API key needed."

**Scope:**
- `internal/ai/` — add `Provider: "pulse-cloud"` that routes LLM calls through hosted infrastructure
- Hosted tenants toggle managed AI on from settings (no API key configuration)
- Use cost-efficient models (Claude Haiku / GPT-4o-mini) for patrol, offer model choice as upsell
- **Abuse controls (must ship with managed AI, not after):**
  - Per-tenant LLM request budget (e.g., 100 patrol runs/day, 20 investigations/day)
  - Hard failure mode: if budget exceeded, disable managed AI for that tenant until next billing cycle
  - Cost attribution: log LLM token usage per tenant for margin analysis
  - Rate limiting: max concurrent LLM calls per tenant
- **Authentication:** Hosted signup must issue a tenant API key (hidden from user) that the backend stores in AI config. This is a new architectural component — design it as a first-class subsystem, not a bolt-on. Without it, the Cloud proxy is an open public relay.
- Gate: validate tenant subscription tier before proxying LLM calls

**Exit criteria:** A Cloud tenant enables managed AI with zero configuration and receives findings/investigations without seeing an API key. Per-tenant budgets are enforced. Margin is measurable per tenant.

### P3: Launch Operations

#### P3-1: Pricing page and landing page update

**Scope:**
- New pricing page reflecting Community / Pro / Cloud tiers (omit MSP/Enterprise from public pricing — keep it sales-led)
- Updated landing page messaging: lead with "secure remote access" and "AI that watches your infra"
- Feature comparison table matching the entitlement matrix exactly
- Founding member pricing ($19/mo for first 100 Cloud signups) prominently featured

**Exit criteria:** Landing page and in-app pricing page are consistent and reflect new tiers.

#### P3-2: Migration path for existing customers

**Scope:**
- 134 existing Pro customers: grandfather at current pricing for 12 months, then migrate to new pricing
- Communication: "You're getting more features at a better price. Nothing is taken away."
- Lifetime license holders (47): continue honoring, no changes

**Exit criteria:** Existing customers are notified, grandfathered, and no one loses access to anything they currently have.

## Out of Scope (Explicit Anti-Creep)

- **White-labeling**: Not implemented, don't promise it.
- **Custom dashboards / Grafana competition**: Pulse's value is answers, not charts.
- **New agent types**: Current coverage (Proxmox, Docker, K8s, TrueNAS, hosts) is sufficient. Deepen, don't widen.
- **Enterprise tier UI/tooling**: Keep MSP/Enterprise sales-led. Don't build enterprise admin UI yet.
- **Per-node metered billing**: Start with flat-rate tiers. Usage-based pricing adds complexity — evolve into it later if needed.
- **Mobile "direct LAN connect" mode**: The mobile app is architecturally relay-based. Building a direct connect mode is a large project. Gate mobile behind Pro/Cloud instead.
- **New capability keys without a 1:1 gate mapping**: Do not introduce `ai_patrol_investigate`, `ai_patrol_autofix`, `relay_remote`, or `max_retention_days` as new keys. Use existing keys (`ai_autofix`, `relay`, `long_term_metrics`) and autonomy-level gating.

## Non-Negotiable Go/No-Go Contract

Do not ship 6.0 unless all of these are true:

1. Every P0 item is complete and verified.
2. `HasFeature()`, `Status()`, and `/api/license/entitlements` all respect evaluator capabilities for hosted tenants.
3. In-app trial works end-to-end (activate → experience Pro → expire → upgrade prompt).
4. Hosted signup → login → add agent → see metrics works end-to-end.
5. Entitlement matrix is consistent across code, docs, landing page, and in-app copy.
6. Relay/mobile onboarding is guided and impressive.
7. Audit logging is persistent, tenant-isolated, queryable, and real — not a stub.
8. Conversion telemetry is persistent, tenant-aware, and can measure free → trial → paid from day one.
9. Every paywall links to a feature-specific upgrade page via `upgrade_reasons`, not a generic landing page.
10. Cloud tenant can enable managed AI with zero configuration and per-tenant budgets are enforced.
11. Stripe webhook signature verification is active and `customer.subscription.deleted` revokes access.

### Quantitative Launch Gates

- **API Latency**: p95 < 200ms for core endpoints.
- **Error Rate**: < 0.1% 5xx on API traffic.
- **Hosted Signup → First Metric**: < 5 minutes.
- **Trial Activation**: < 3 clicks from any paywall.
- **Entitlement Parity**: 0 mismatches between `HasFeature()` and entitlements response across all capability keys.
- **Rollback Trigger**: > 5 P0/P1 tickets in first 24h OR > 1% global error rate → immediate rollback.

## Commercial Architecture

The entitlement primitives, evaluator design, metering pipeline, state machine, frontend contract, and security/revocation architecture from the previous guiding light remain in effect. They are not repeated here — refer to git history for the full B1-B6 specifications.

### Gating Strategy for 6.0

The core principle is: **use existing feature keys and autonomy levels, not new capability keys.**

| Gate | Mechanism | Existing Key | Community | Pro/Cloud |
|------|-----------|-------------|-----------|-----------|
| AI Patrol (run + findings) | Feature key | `ai_patrol` | ✓ (BYOK) | ✓ |
| AI Investigation + Reinvestigation | Autonomy level + route gate | `ai_patrol` + autonomy ≥ `approval` | ✗ (locked to `monitor`; reinvestigate = 402) | ✓ |
| AI Auto-Fix execution | Feature key | `ai_autofix` | ✗ | ✓ |
| Relay / Mobile | Feature key | `relay` | ✗ | ✓ |
| 90-day retention | Feature key | `long_term_metrics` | ✗ | ✓ |
| Audit logging (query/export) | Feature key | `audit_logging` | ✗ (events captured, not queryable) | ✓ |
| RBAC | Feature key | `rbac` | ✗ | ✓ |
| Basic SSO (OIDC) | Feature key | `sso` | ✓ (free) | ✓ |
| Advanced SSO (SAML) | Feature key | `advanced_sso` | ✗ | ✓ |

This avoids introducing any new capability keys. The only behavioral change is restricting Community to `monitor` autonomy and ensuring the autonomy settings UI enforces this.

## Execution Dependency Graph

```
Phase 1: Trust Foundation (P0)
├── P0-0: Hosted entitlements enforcement ← BLOCKS ALL HOSTED WORK
├── P0-1: Audit logging (tenant-aware)
├── P0-2: Frontend → entitlements ← REQUIRES P0-0
├── P0-3: SSO gating consistency
├── P0-4: Hosted signup auth (magic links)
├── P0-5: Trial subscription state plumbing ← REQUIRES P0-0
└── P0-6: Conversion telemetry persistence
         │
Phase 2: Conversion Engine (P1) — requires P0 complete
├── P1-1: In-app trial ← REQUIRES P0-5
├── P1-2: Relay hero onboarding ← REQUIRES P1-1 (trial must work at pairing moment)
└── P1-3: AI Patrol Community limits ← REQUIRES P1-1 (trial as escape valve)
         │
Phase 3: Cloud Platform (P2) — requires P0-0, P0-4 complete
├── P2-1: Deploy hosted instance
├── P2-2: Stripe → org provisioning (with webhook signature verification)
├── P2-3: Agent onboarding for Cloud
└── P2-4: Managed AI provider ← REQUIRES P2-1/2/3 stable + abuse controls
         │
Phase 4: Launch Operations (P3)
├── P3-1: Pricing + landing page
└── P3-2: Customer migration
```

**Critical path:** P0-0 → P0-5 → P1-1 → P1-2/P1-3 (entitlements → trial plumbing → trial UX → conversion features). Everything else can be parallelized around this chain.

## Decision Log

- [x] Entitlement schema approved (previous guiding light).
- [x] Evaluator API approved (previous guiding light).
- [x] Pricing structure: Community (Free) / Pro ($15/mo) / Cloud ($29/mo flat) / MSP (sales-led).
- [x] AI Patrol free tier: `monitor` autonomy only — findings visible, no investigation/auto-fix.
- [x] Mobile/Relay: Pro/Cloud only (no "LAN only" mode — mobile is architecturally relay-based).
- [x] Existing customer migration: 12-month grandfather, then new pricing.
- [x] Gating strategy: use existing feature keys + autonomy levels, no new capability keys.
- [x] Cloud includes managed AI (P2-4). Managed AI depends on Cloud infrastructure (P2-1/2/3) being stable — this is a dependency chain, not a scope cut.
- [x] Telemetry is a P0 (launch gate).
- [x] Hosted launch posture: **Beta/Early Access** with waitlist. Graduate to GA once all P0-P2 items are verified and < 5 P0 tickets are open.
- [x] Cloud auth method: **Passwordless magic links.** Cloud users won't have an IdP, and we don't have a credential store. Magic links are low-friction, no password to throw away, and modern. Implement in P0-4.
- [x] Basic SSO (OIDC): **Free for self-hosted.** It's table stakes in 2026. Update MONETIZATION.md and landing page to match `features.go`. Advanced SSO (SAML/multi-provider) remains Pro.
- [x] Trial policy: **14-day duration, local billing state mechanism, 1 trial per org ever, full Pro capabilities during trial.** Same mechanism for self-hosted and hosted. No license server roundtrip.
- [x] Reinvestigation gating: **Reinvestigate is investigation and requires Pro.** Community gets read-only finding access. Routes in `router_routes_ai_relay.go:113-130` must be updated.
- [x] Audit logging strategy: **Always capture events to SQLite; gate query/export behind license.** Simpler than conditional logger creation.
- [x] Cloud launch pricing: **$29/mo flat with "Founding Member" discount of $19/mo locked for first 100 signups.** Creates urgency and builds early community. Regular price holds at $29/mo.
- [ ] Cloud infrastructure sizing: initial droplet spec and scaling triggers. (Decide during P2-1 implementation.)
- [ ] Cloud AI model selection: default to cost-efficient models, offer choice as upsell. (Decide during P2-4 implementation.)

## Anti-Patterns to Avoid

1. Hardcoding business packaging in route handlers or frontend components.
2. Shipping paywalls that link to a generic landing page instead of feature-specific upgrade pages.
3. Announcing hosted capabilities before signup → login → monitoring works end-to-end.
4. Gating monitoring itself — free monitoring must remain unlimited and excellent.
5. Treating mobile/relay as a secondary feature instead of the primary conversion driver.
6. Measuring success by downloads instead of trial starts and paid conversions.
7. Giving away a year of revenue to migrate existing customers when a grandfather period achieves the same outcome.
8. Building enterprise UI/tooling before having enterprise customers.
9. **Introducing new capability keys that don't map 1:1 to a runtime gate and an upgrade reason.** Every new key creates surface area in features.go, TierFeatures, upgrade_reasons, backend gates, and frontend gates. Use existing keys.
10. **Shipping managed AI without per-tenant budgets and hard failure modes.** $29/mo flat with unlimited LLM proxying will produce negative margins instantly at scale.
11. **Restricting free-tier capabilities without a trial escape valve.** Gate AI investigation behind Pro only after the trial flow is live and tested.
12. **Trusting that `HasFeature()` and the entitlements endpoint agree** without an automated parity test. They have diverged before and will diverge again without enforcement.

## Success Metrics (30 Days Post-Launch)

| Metric | Target | How Measured |
|--------|--------|-------------|
| Paywall viewed → trial started | > 15% | Conversion telemetry (P0-6) |
| Trial started → paid conversion | > 10% of trials | Conversion telemetry + Stripe |
| Relay pairing completion rate | > 60% of wizard opens | Frontend analytics |
| Time to first "aha" (finding or remote connection) | < 10 minutes from install | Backend telemetry |
| Cloud signups (beta/early access) | > 50 in first month | Hosted signup count |
| Cloud managed AI activation | > 80% of Cloud tenants | Backend telemetry |
| Existing customer churn | < 5% | Stripe dashboard |
| P0/P1 support tickets | < 20 in first week | GitHub issues |
| Entitlement parity mismatches | 0 | Automated parity test |
| Managed AI cost per tenant | < $3/mo average | LLM token usage logs (P2-4) |

## Top Execution Risks (Watch List)

These are the highest-probability failure modes identified through adversarial review. Monitor them actively.

**1. P0-0 regression locks users out of features.**
`HasFeature()` is called by every gate in the system. A bug in the evaluator delegation path could return `false` for existing Pro users with JWT licenses. **Mitigation:** Unit test matrix covering all 4 license/evaluator combinations. Entitlement parity launch gate. Deploy behind feature flag if possible.

**2. Trial plumbing scope creep.**
Trial activation touches billing state schema, evaluator behavior, entitlements payload computation, and frontend components. The scope is larger than it looks. **Mitigation:** One trial mechanism only (local billing state). No JWT trial path. No license server changes. Keep plumbing (P0-5) and UX (P1-1) as separate work items — don't conflate them.

**3. Mobile paywall backlash if trial isn't flawless at the relay pairing moment.**
The "no mobile for Community" stance means the first time a user tries to pair their phone, they hit a wall. If the trial button doesn't work perfectly at that exact moment, frustration exceeds conversion. **Mitigation:** Trial must be live and tested before relay onboarding ships. P1-2 (relay) depends on P1-1 (trial) — this is an enforced dependency. QA the specific flow: user taps "Pair Device" → sees "Start 14-day trial" → one click → pairing proceeds. If any step fails, the entire conversion funnel breaks.

**4. Managed AI cost economics.**
$29/mo flat with managed AI means Pulse absorbs LLM costs. A single heavy user with 50+ nodes running patrol every hour could cost more than their subscription. **Mitigation:** Per-tenant budgets with hard caps are mandatory in P2-4. Cost attribution logging from day one. Monitor managed AI cost per tenant against the $3/mo average target. If economics don't work at launch pricing, adjust budgets before adjusting price.

**5. Audit logging cross-tenant data leak.**
The existing audit infrastructure is singleton/global. The refactor to per-tenant SQLite introduces a new isolation boundary. A bug here means Tenant A sees Tenant B's audit events — a trust-destroying outcome for the exact customer segment that pays for compliance features. **Mitigation:** Tenant isolation test suite is a hard exit criterion for P0-1. Test with multiple concurrent tenants writing and querying simultaneously.
