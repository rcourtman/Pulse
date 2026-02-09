# Next Release Readiness (Guiding Light)

Status: Complete (W0-W6 Lanes Closed)
Owner: Pulse
Date: 2026-02-08
Applies to: Next major release after the 5.1.x hotfix line

Related plans:
- `docs/architecture/unified-resource-finalization-plan-2026-02.md`
- `docs/architecture/unified-resource-finalization-progress-2026-02.md`
- `docs/architecture/mobile-app-operational-plan-2026-02.md`
- `docs/architecture/mobile-app-operational-progress-2026-02.md`
- `docs/architecture/multi-tenant-productization-plan-2026-02.md`
- `docs/architecture/multi-tenant-ga-readiness-progress-2026-02.md`
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md`
- `docs/architecture/monetization-foundation-plan-2026-02.md`
- `docs/architecture/monetization-foundation-progress-2026-02.md`
- `docs/architecture/truenas-ga-plan-2026-02.md`
- `docs/architecture/truenas-ga-progress-2026-02.md`
- `docs/architecture/truenas-rollout-readiness-plan-2026-02.md`
- `docs/architecture/truenas-rollout-readiness-progress-2026-02.md`
- `docs/architecture/multi-tenant-rbac-user-limits-plan-2026-02.md`
- `docs/architecture/multi-tenant-rbac-user-limits-progress-2026-02.md`
- `docs/architecture/multi-tenant-rbac-operations-plan-2026-02.md`
- `docs/architecture/multi-tenant-rbac-operations-progress-2026-02.md`
- `docs/architecture/conversion-readiness-plan-2026-02.md`
- `docs/architecture/conversion-readiness-progress-2026-02.md`
- `docs/architecture/conversion-operations-plan-2026-02.md`
- `docs/architecture/conversion-operations-progress-2026-02.md`
- `docs/architecture/hosted-readiness-plan-2026-02.md`
- `docs/architecture/hosted-readiness-progress-2026-02.md`
- `docs/architecture/hosted-operations-plan-2026-02.md`
- `docs/architecture/hosted-operations-progress-2026-02.md`

## Release Thesis

This release is not an incremental patch. It is a product-shape change.

The release wins only if all four outcomes land together:
1. Unified resource model is the default product language (`Infrastructure`, `Workloads`, `Storage`, `Backups`).
2. Platform breadth expands with production-grade TrueNAS support.
3. Mobile app becomes operationally useful (secure remote access, approvals, notifications).
4. Multi-tenant becomes real and trustworthy for teams, not just a hidden flag.

If one of the four slips, launch positioning and conversion quality both degrade.

## Out of Scope (Explicit Anti-Creep)
- **Custom Reporting Builder**: Use predefined PDF templates only.
- **Advanced White-Labeling**: No custom CSS/Domain for this release.
- **Legacy Agent Support**: Agents < v4.0 must upgrade to connect.
- **Billing Integration**: Stripe/Paddle integration is manual/external for v1. No in-app checkout code.

## North Star Outcomes

1. Product maturity is obvious in the first 10 minutes of use.
2. Free tier remains genuinely useful, but clear daily pain is solved by paid plans.
3. Upgrade intent can be measured and converted inside the app, not only on the website.
4. Post-release support load stays controlled (no fire-drill launch).

## Non-Negotiable Go/No-Go Contract

Do not ship the major release unless all of these are true:
1. Zero P0 regressions open in unified resource navigation and core data paths.
2. Multi-tenant isolation and authorization tests are green for API and websocket flows.
3. Mobile app pairing, lock, push, and approval paths are stable in real-device runs.
4. Commercial gating copy in product, docs, and website match exactly (no contradictory language).
5. Upgrade and rollback runbooks are complete and rehearsal-tested.
6. Observability exists for conversion funnel and critical runtime errors on day 1.

### Quantitative Launch Gates (SLOs)
- **API Latency**: p95 < 200ms for core dash/agent endpoints.
- **Error Rate**: < 0.1% 5xx on API traffic.
- **Mobile Reconnect**: > 99% success rate on network switch.
- **Rollback Trigger**: > 5 P0/P1 tickets in first 24h OR > 1% global error rate -> **Immediate Rollback**.

## Commercial Architecture (Design First, Pricing Later)

Pricing can change. Architecture should not.

### Entitlement primitives

All monetization decisions must be expressible as these primitives:
1. `capabilities` (boolean): example `multi_tenant`, `rbac`, `relay_remote`, `audit_logging`, `connectwise_native`.
2. `limits` (quantitative): example `max_orgs`, `max_agents`, `max_retention_days`, `max_relay_sessions`.
3. `meters_enabled` (usage dimensions): example `active_agents`, `managed_clients`, `relay_bytes_gb`, `notifications_sent`.
4. `plan_version` (immutable commercial contract): preserves grandfathered terms.
5. `subscription_state`: `trial`, `active`, `grace`, `expired`, `suspended` with explicit behavior.

### Core rules

1. Marketing tier names are presentation only.
2. Backend enforcement uses entitlements, not tier string branching.
3. Every gate in API/UI must map to a single canonical entitlement key.
4. Every quantitative gate must have both soft-limit and hard-limit behavior.
5. Every plan change must be backwards-compatible through `plan_version`.
6. **Capability Lifecycle**: Deprecated capabilities must have a defined `sunset_at` date and `replacement_key`. Evaluator logs warnings for use of deprecated keys.

### Current structural bottleneck

Current licensing is centered on hardcoded tier maps in:
- `internal/license/features.go`

This is acceptable for fixed packaging, but slow and brittle for:
1. MSP custom deals
2. launch partner exceptions
3. usage-based evolution
4. temporary promotions/trials

## Monetization-Ready Backend Blueprint

### B1: Entitlement contract in license claims

Add canonical claims payload fields:
1. `capabilities: string[]`
2. `limits: map[string]int64`
3. `meters_enabled: string[]`
4. `plan_version: string`
5. `subscription_state: string`

Compatibility rule:
- If explicit entitlements are absent, derive from legacy tier map for backward compatibility.

### B2: Canonical evaluator

Implement a single evaluator module used by all runtime surfaces:
1. `HasCapability(key)` **with Alias Resolution**:
   - Checks `key` first.
   - Checks `LegacyAliases[key]` if present (backwards compatibility for renamed features in old tokens).
2. `GetLimit(key)`
3. `CheckLimit(key, observedValue)` returning `allowed`, `soft_block`, or `hard_block`
4. `MeterEnabled(key)`

Implementation Requirement:
- Evaluator must interact with an `EntitlementSource` interface, not raw tokens.
- Implementation A: `TokenSource` (Stateless validation of JWT for Self-Hosted).
- Implementation B: `DatabaseSource` (Direct DB lookup for SaaS/Hosted).



**Concurrency & Limits Strategy**:
- Use **Eventual Consistency** for limit checks to prioritize system uptime/throughput.
- Accept temporary overage (e.g., 55/50 agents) during race conditions.
- Rectify via async reconciliation: "You are 5 agents over limit. Please upgrade or remove agents."
- **Do not** use blocking distributed locks for every agent registration.

**Abuse Circuit Breaker (Crucial for Fail-Open)**:
1. **Enrollment Rate Limits**: Hard cap on *new* agent registrations per IP/Org per hour (e.g., max 100/hr), regardless of plan.
2. **Global Safety Cap**: Absolute ceiling for enrollments to prevent scripted DoS during CRL outages.
3. **Degraded Mode**: If CRL is unreachable > 1 hour, block *new* enrollments but keep existing agents active.

All gating layers call this evaluator:
1. API middleware
2. background jobs
3. relay server
4. websocket publishers
5. license/status API responses consumed by frontend

### B3: Metering event pipeline

Emit auditable usage events from backend services:
1. `agent_seen`
2. `org_created`
3. `relay_bytes`
4. `notification_sent`
5. `tenant_switch`

Pipeline requirements:
1. idempotent event writes
2. **Windowed Pre-Aggregation**: High-frequency events (e.g., `agent_seen`) must aggregate in memory/Redis (e.g., `INCR agent:123:checkins`) and flush hourly to DB. **Do not** write per-event rows.
3. **Cardinality Limits**: Strict hard cap on unique meter keys per tenant to prevent attribute explosion attacks.
4. daily aggregation table
5. reconciliation job
6. exportable usage snapshot for billing support disputes

### B4: State machine behavior

`subscription_state` behavior must be deterministic:
1. `trial`: full capabilities as configured, expiry timestamp mandatory.
2. `active`: normal enforcement.
3. `grace`: paid capabilities preserved with warning and countdown.
4. `expired`: fallback capabilities only; no data loss.
5. `suspended`: explicit administrative lock behavior.
6. **Downgrade Policy**: Data exceeding new limits (e.g., retention days) is **soft-hidden** for a set grace period (e.g., 30 days) to allow recovery/re-upgrade, then **hard-deleted**.

### B5: Frontend contract

Frontend should consume a normalized entitlement payload and never infer from tier names.

Required UI payload sections:
1. `capabilities`
2. `limits` + current usage for each surfaced limit
3. `subscription_state`
4. `upgrade_reasons` with user-actionable copy

**Over-Limit UX State Machine**:
- **Warning (Soft Limit)**: Usage > Limit. UI shows amber badge ("55/50 Agents"). Functionality persists.
- **Grace Period**: Usage > Limit for > 72h. UI shows red banner with countdown.
- **Enforcement (Hard Limit)**: Grace expired. *New* resources blocked. Existing resources potentially degraded (read-only).
- **Abuse Lock**: Circuit breaker tripped. All changes blocked. Contact support.

### B6: Security & Trust (Revocation & Safety)

1. **Heartbeat requirement**: High-value entitlements (Pro/Enterprise) must support an optional `online_check` or a synchronized `revocation_list` (CRL).
2. **Purpose**: Handles chargebacks, abuse, or compromised keys before the rigid JWT expiration.
3. **Failure Mode**: **Fail Open** with bounded staleness (e.g., cached CRL valid for 72h). If CRL is unreachable, do not block critical operations. Log the error.
4. **Panic Safety**: The Evaluator must recover from internal panics (e.g., nil pointer in token parsing) and **Fail Open** (allow access) to prevent bricking customer monitoring, while logging a P0 error.

## MSP-First Architecture Requirements

These must exist even before final pricing is decided:

1. Organization model first-class
- Multi-tenant org boundaries are strict and auditable.
- Per-org alert routing and API token scoping.

2. Ingest/control-plane separation mode
- Distinct listener or binding options for agent/check-in traffic and admin UI/API traffic.
- Explicit docs for firewall posture and trust boundaries.

3. Integration model
- Generic webhook + alerts API always available.
- Native PSA integrations (like ConnectWise) are independent capabilities, not core coupling.

4. Commercial isolation
- Ability to issue MSP licenses with custom capability/limit bundles without code changes.
- Ability to grandfather launch partners through `plan_version`.

## Workstreams and Exit Criteria

### W0: Monetization Foundation (New)

Exit criteria:
1. Entitlement primitives are implemented in claims and runtime evaluator.
2. Legacy tier compatibility shim is in place and tested.
3. Metering event schema and daily aggregation are implemented.
4. Frontend reads normalized entitlement payload rather than tier assumptions.

Evidence:
- Contract tests for evaluator and fallback behavior
- Migration notes for legacy licenses

### W1: Unified Resource Completion

Exit criteria:
1. No first-party runtime dependency on legacy `/api/resources` in active paths.
2. Storage/backups legacy compatibility work is complete and verified.
3. Navigation, deep links, and route aliases all resolve to unified surfaces.
4. Documentation and screenshots reflect unified information architecture only.

Evidence:
- Updated progress doc and certification packet references
- Typecheck and targeted frontend/backend contract tests

### W2: TrueNAS GA Quality

Exit criteria:
1. TrueNAS onboarding/setup is documented and testable.
2. Resource discovery and health data are consistent with existing platforms.
3. Error handling and degraded-state behavior are explicit and user-friendly.
4. Alerts and AI context can reason over TrueNAS resources through unified model.

Evidence:
- Connector test matrix
- Known limits documented in release notes and docs

### W3: Mobile App Operational GA

Exit criteria:
1. Pairing works for QR and manual flows.
2. Biometric lock/session protection is validated across app foreground/background cycles.
3. Push notifications and action deep-links are reliable.
4. Approval workflows are secure, auditable, and latency-tolerant.
5. Relay reconnection behavior is stable under network churn.
6. **Metering Visibility**: Mobile UI clearly displays usage of metered features (e.g., Relay Data Usage) if active.

Evidence:
- Test suite pass records
- Real-device smoke checklist (iOS and Android)

### W4: Multi-Tenant GA

Exit criteria:
1. Single-tenant default mode hides all tenant concepts.
2. Multi-tenant mode has strict server-side org binding and role checks.
3. No cross-org data/event leakage in API, websocket, alerts, or AI context.
4. Operational kill-switch and rollback are tested.

Evidence:
- Isolation/security test pack
- Operational runbook with verification steps

### W5: Conversion Readiness

Exit criteria:
1. Source-of-truth entitlement matrix is finalized and published across app/site/docs.
2. Trial entry points exist at high-intent moments (mobile pairing, tenant setup, role setup).
3. In-app upgrade prompts are context-aware and action-oriented.
4. Billing/license lifecycle messages are clear for active, grace, and expired states.

Evidence:
- Entitlement matrix doc
- Funnel telemetry dashboards validated in staging

### W6: Hosted Readiness (If Included in Launch Scope)

Exit criteria:
1. Hosted onboarding and tenant provisioning path is defined.
2. Reliability and support expectations are documented (SLOs, escalation paths).
3. Security baseline and data handling policy are publishable.

Evidence:
- Hosted runbook draft and support workflow

Note:
- If Hosted is not launch-ready, explicitly label it as waitlist/private beta.

## Conversion Instrumentation (Must Exist Before Public Launch)

Track these events from day one:
1. `paywall_viewed` with context (`capability`, `surface`, `tenant_mode`).
2. `trial_started` with trigger surface.
3. `license_activated` and `license_activation_failed`.
4. `upgrade_clicked` and `checkout_started`.
5. `checkout_completed` and refund/cancel reasons.
6. `limit_warning_shown` and `limit_blocked` with limit key and usage values.

Core KPIs:
1. Free to trial rate.
2. Trial to paid conversion.
3. Time-to-value from install to first "aha" event.
4. 30-day retention for paid users.
5. Support ticket rate per 100 active instances after release.

## Launch Readiness Checklist

### Product

- [x] Unified IA complete and default in all key flows
- [x] TrueNAS path production-tested with documented limits
- [x] Mobile remote + push + approvals stable on real devices
- [x] Multi-tenant passes isolation and auth hardening checks

### Commercial Architecture

- [ ] Entitlement evaluator is the only runtime gating path (deferred to final certification)
- [x] Legacy tier fallback behavior is tested and documented
- [ ] Capability-to-UI-copy mapping has no ambiguity (deferred to final certification)
- [x] Soft-limit and hard-limit behavior is implemented and tested
- [ ] Trial mechanics tested end to end (deferred to final certification)

### Operational

- [ ] Upgrade guide and breaking changes doc prepared (deferred to final certification)
- [ ] Rollback plan for server and frontend validated (deferred to final certification)
- [ ] Support macros/templates prepared for top 10 expected questions (deferred to final certification)
- [ ] On-call/release monitoring watchlist prepared (deferred to final certification)

### Evidence

- [ ] Required test suites and smoke runs complete with recorded outputs (deferred to final certification)
- [ ] Open risk register contains only explicitly accepted residual risks (deferred to final certification)
- [ ] Go/No-Go decision recorded with date and rationale (deferred to final certification)

## Execution Dependency Graph

### Phase 1: Architecture Lock (Prerequisite for Everything)

1. Lock entitlement primitives and payload schema.
2. Confirm go/no-go gates and owners.
3. Decision Logs 1-4 resolved.

### Phase 2: Product Hardening & Foundation

1. Close unified resource and TrueNAS critical gaps.
2. Run multi-tenant security/isolation replay.
3. Complete mobile reliability and UX hardening.
4. Implement entitlement evaluator and compatibility shim.

### Phase 3: Commercial Integration (Prerequisite for Public Beta)

1. Wire paywall/upgrade/trial paths to entitlement keys.
2. Validate billing/license lifecycle copy and edge cases.
3. Stand up metering aggregation and dashboards.

### Phase 4: Launch Readiness

1. Execute launch checklist.
2. Run full smoke matrix and rollback drill.
3. Resolve all P0/P1 findings or explicitly de-scope launch claims.


## Decision Log (Keep Updated)

- [x] Entitlement schema approved.
- [x] Evaluator API approved.
- [x] Legacy tier migration plan approved.
- [x] Initial capability and limit matrix approved.
- [x] Hosted launch posture approved (`GA`, `beta`, or `waitlist`).
- [ ] Trial policy approved (duration, triggers, limits). (deferred to final certification)

## Anti-Patterns to Avoid

1. Hardcoding business packaging in route handlers or frontend components.
2. Shipping major IA change with ambiguous paywalls.
3. Announcing hosted capabilities before operational maturity.
4. Treating mobile as marketing-only instead of operationally dependable.
5. Launching multi-tenant without exhaustive isolation confidence.
6. Measuring success only by downloads instead of activation and paid conversion.

## Final Go/No-Go Rubric

Launch only when every section is green:
1. Product quality gate
2. Security/isolation gate
3. Commercial architecture gate
4. Operational readiness gate
5. Measurement gate

If any section is red, delay and fix. This release is the platform reset point and should be treated accordingly.
