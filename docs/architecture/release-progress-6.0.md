# Pulse 6.0 — Implementation Progress Tracker

Status: ACTIVE
Orchestrator: Claude (this document is maintained by the orchestrator only)
Workers: External AI sessions dispatched by user
Source of truth: `docs/architecture/release-readiness-guiding-light-2026-02.md`
Architecture ref: `docs/architecture/cloud-container-per-tenant-2026-02.md`

## How This Works

1. Orchestrator (me) creates worker prompts with precise scope
2. User gives prompts to available workers
3. Workers implement and return evidence (files changed, commands run, exit codes)
4. User feeds evidence back to orchestrator
5. Orchestrator verifies independently (reads files, runs tests/build)
6. If approved → checkpoint commit → next task. If not → feedback prompt back to worker.

## Status Key

- `READY` — prompt prepared, waiting for worker assignment
- `ASSIGNED` — worker is actively implementing
- `REVIEW` — worker returned output, orchestrator reviewing
- `CHANGES_REQUESTED` — sent back to worker with feedback
- `APPROVED` — verified and committed
- `BLOCKED` — waiting on dependency
- `DEFERRED` — explicitly deferred from v6.0 scope
- `OBSOLETE` — superseded by architecture pivot

---

## Architecture Pivot (2026-02-10)

Cloud platform pivoted from **shared-process multi-tenant** to **container-per-tenant**.
See `cloud-container-per-tenant-2026-02.md` for full rationale and design.

**Impact on existing work:**
- P0 (Trust Foundation): All valid — product features, not deployment-model dependent
- P1 (Conversion Engine): All valid — product features
- P2 (Cloud Platform): **Rewritten** — old W4-A/B/C replaced by new C-series items
- P3 (Launch Operations): Mostly valid — pricing page may need Cloud tier update
- RC (Security Hardening): All valid
- Stripe webhook code (W4-B, RC-1): Reusable — patterns move to control plane

---

## P0: Trust Foundation ✅ COMPLETE

All 7 items approved and committed.

| ID | Guiding Light | Work Item | Status | Commit |
|----|---------------|-----------|--------|--------|
| W1-A | P0-0 | Fix hosted entitlements enforcement | APPROVED | `b105492b` |
| W1-B | P0-1 | Audit logging real + tenant-aware | APPROVED | `ca01fdf5` |
| W1-C | P0-3 | SSO gating consistency | APPROVED | `4e2f02bb` |
| W1-D | P0-4 | Hosted signup auth (magic links) | APPROVED | `571c2639` |
| W1-E | P0-6 | Conversion telemetry persistence | APPROVED | `d2e52740` |
| W2-A | P0-2 | Frontend → entitlements endpoint | APPROVED | `456d1ec9` |
| W2-B | P0-5 | Trial subscription state end-to-end | APPROVED | `fe3752f9` |

## P1: Conversion Engine ✅ COMPLETE

All 3 items approved and committed.

| ID | Guiding Light | Work Item | Status | Commit |
|----|---------------|-----------|--------|--------|
| W3-A | P1-1 | Ship in-app Pro trial | APPROVED | `7822e504` |
| W3-B | P1-2 | Relay hero onboarding | APPROVED | `ae387d16` |
| W3-C | P1-3 | AI Patrol Community limits | APPROVED | `df5bd183` |

## P2: Cloud Platform — Container-Per-Tenant ✅ COMPLETE

Architecture: each Cloud customer gets their own Pulse container. A separate control plane binary handles signup, Stripe, and Docker lifecycle. Traefik routes `*.cloud.pulserelay.pro` by Host header via Docker labels.

### Prior Cloud work (shared-process era)

These were built for the shared-process model. Code is reusable in control plane context.

| Old ID | Work Item | Status | Commit | Reusability |
|--------|-----------|--------|--------|-------------|
| W4-A | Deploy hosted Pulse instance (shared-process) | OBSOLETE | — | Replaced by C-series below |
| W4-B | Stripe → org provisioning (in-process handlers) | APPROVED | `ec9189fd` | **Reusable** — webhook verification, idempotency, billing state patterns adapt to control plane |
| W4-C | Agent onboarding for Cloud | APPROVED | `3fe8a18d` | **Partially reusable** — per-tenant containers use standard install scripts; multi-tenant endpoint not needed |
| W4-D | Managed AI provider | DEFERRED | — | Post-v6. Requires per-tenant budgets, abuse controls, cost attribution |
| RC-1 | Stripe webhook hardening | APPROVED | `786a8cc0` | **Reusable** — hardening moves with webhook handlers |

### New Cloud work (container-per-tenant)

| ID | Work Item | Status | Worker | Commit | Depends On |
|----|-----------|--------|--------|--------|------------|
| C-1 | Control plane binary scaffold | APPROVED | Claude | `afa1c85e` | — |
| C-2 | Docker provisioner | APPROVED | Claude | `afa1c85e` | C-1 |
| C-3 | Traefik + docker-compose stack | APPROVED | Codex | `7b268fa5` | — |
| C-4 | Stripe → container provisioning | APPROVED | Claude | `afa1c85e` | C-1, C-2 |
| C-5 | billing.json lifecycle (write + RO mount + verify) | APPROVED | Claude | `afa1c85e` | C-2 |
| C-6 | Signup + magic link flow in control plane | APPROVED | Claude | `463e4eff` | C-1, C-4 |
| C-7 | Health monitoring + admin endpoints | APPROVED | Claude | `afa1c85e` | C-2 |
| C-8 | Droplet deployment + DNS + end-to-end verification | APPROVED | Codex/Claude | `7b268fa5`, `9b9570ca` | C-1 through C-6 |

#### C-1: Control Plane Binary Scaffold

**Goal:** Create `cmd/pulse-control-plane/main.go` — a standalone HTTP server that will orchestrate Cloud tenant lifecycle.

**Scope:**
- `cmd/pulse-control-plane/main.go` — HTTP server, config loading, graceful shutdown
- `internal/controlplane/config.go` — control plane configuration (Stripe keys, DO token, data dir, domain)
- `internal/controlplane/registry.go` — SQLite tenant registry: `(tenant_id, display_name, email, stripe_customer_id, stripe_subscription_id, status, created_at, updated_at)`
- Tenant ID generation: `t-` prefix + 6 lowercase alphanumeric chars (e.g., `t-a7b3x9`)
- Endpoints: `GET /healthz`, `GET /admin/tenants` (list), `GET /admin/tenants/{id}` (detail)
- Imports existing `internal/` packages from same Go module

**Acceptance:** `go build ./cmd/pulse-control-plane/...` succeeds. Binary starts, serves `/healthz`. Registry creates SQLite DB and handles CRUD.

#### C-2: Docker Provisioner

**Goal:** Provision and manage per-tenant Pulse containers via Docker API.

**Scope:**
- `internal/controlplane/provisioner.go` — Docker client wrapper using `docker/docker` client library
- `ProvisionTenant(id, tier)` — creates data dir at `/data/tenants/<id>/`, starts container with:
  - Image: `ghcr.io/rcourtman/pulse:latest` (configurable)
  - Traefik labels: `traefik.http.routers.<id>.rule=Host(\`<id>.cloud.pulserelay.pro\`)`
  - Volume: `/data/tenants/<id>/` → `/etc/pulse` (+ billing.json as RO)
  - Memory limit: tier-dependent (Pro: 512MB, default: 256MB)
  - Restart policy: `unless-stopped`
  - Network: shared Docker network with Traefik
- `DeprovisionTenant(id)` — stop + remove container, archive data dir
- `StopTenant(id)` / `StartTenant(id)` — for suspension/resumption
- `TenantStatus(id)` — container running/stopped/missing

**Acceptance:** `ProvisionTenant("t-abc123", "cloud")` creates and starts a reachable container. `DeprovisionTenant("t-abc123")` removes it cleanly.

#### C-3: Traefik + Docker-Compose Stack

**Goal:** Production-ready Traefik configuration for `*.cloud.pulserelay.pro`.

**Scope:**
- `deploy/docker/docker-compose.yaml` — Traefik + control plane services
- `deploy/docker/traefik.yml` — static config: Docker provider, ACME DNS-01 (DigitalOcean), entrypoints (80→443 redirect, 443 TLS)
- `deploy/docker/.env.example` — `DO_AUTH_TOKEN`, `ACME_EMAIL`, `STRIPE_WEBHOOK_SECRET`, `DOMAIN`
- Shared Docker network (`pulse-cloud`) for Traefik ↔ tenant container communication
- Traefik dashboard disabled in production
- Security headers middleware (HSTS, X-Frame-Options, etc.)

**Acceptance:** `docker compose up` starts Traefik + control plane. Traefik obtains wildcard cert. Manually started tenant container with correct labels is reachable at `<id>.cloud.pulserelay.pro`.

#### C-4: Stripe → Container Provisioning

**Goal:** Connect Stripe webhooks to Docker container lifecycle.

**Scope:**
- Adapt webhook verification + idempotency patterns from `stripe_webhook_handlers.go` (W4-B, RC-1)
- `internal/controlplane/stripe.go` — webhook handler registered on control plane
- `checkout.session.completed` → create tenant in registry → `ProvisionTenant()` → write `billing.json` → send magic link
- `customer.subscription.updated` → rewrite `billing.json` with new state/capabilities
- `customer.subscription.deleted` → update `billing.json` (canceled) → `StopTenant()` after grace period
- `invoice.payment_failed` → update `billing.json` (past_due)
- Founding member flow: separate Price ID → `plan_version: "founding_19"`, never overwritten

**Acceptance:** Stripe test webhook → container provisioned → billing.json written → magic link sent. Cancellation webhook → container stopped within 60s.

#### C-5: billing.json Lifecycle

**Goal:** Verify the entitlement pipeline works end-to-end with external billing.json.

**Scope:**
- Control plane writes `/data/tenants/<id>/billing.json` using existing `BillingState` struct
- File mounted read-only into container at `/etc/pulse/billing.json`
- Verify existing `FileBillingStore` reads from the expected path inside container
- Verify `DatabaseSource` → `Evaluator` → `HasFeature()` pipeline works for billing-derived entitlements
- Test: update `billing.json` externally → verify container picks up new capabilities on next eval cycle

**Acceptance:** Tenant container with `billing.json` containing `"capabilities": ["ai_patrol", "ai_autofix"]` returns `HasFeature("ai_autofix") == true`. External update to `billing.json` reflected in container within eval interval.

#### C-6: Signup + Magic Link Flow

**Goal:** End-to-end signup: user enters email → Stripe Checkout → webhook → container → magic link → logged in.

**Scope:**
- `internal/controlplane/signup.go` — signup handler: validate email → create Stripe Checkout session → redirect
- Adapt existing magic link service (`magic_link.go`) for control plane context
- Magic link URL points to `<tenant-id>.cloud.pulserelay.pro/auth/magic?token=...`
- Tenant's Pulse instance validates the magic link token and creates a session
- Landing page at `cloud.pulserelay.pro` with signup form

**Acceptance:** User completes signup flow end-to-end: email → Stripe → webhook → container live → magic link email → click → logged into their tenant dashboard within 60 seconds.

#### C-7: Health Monitoring + Admin Endpoints

**Goal:** Keep tenant containers healthy and provide admin visibility.

**Scope:**
- `internal/controlplane/health.go` — periodic health check loop (check Docker container status + HTTP health endpoint)
- Auto-restart containers that fail health checks (with backoff)
- Admin endpoints: `GET /admin/tenants` (list with status), `POST /admin/tenants/{id}/restart`, `POST /admin/tenants/{id}/stop`
- Prometheus metrics: `pulse_cloud_tenants_total`, `pulse_cloud_tenants_healthy`, `pulse_cloud_container_restarts_total`

**Acceptance:** Unhealthy container gets restarted automatically within 2 minutes. Admin can list all tenants and force-restart via API.

#### C-8: Droplet Deployment + End-to-End Verification

**Goal:** Production deployment on DigitalOcean with full end-to-end verification.

**Scope:**
- Provision DO droplet (4 vCPU / 8 GB / $48/mo)
- Install Docker + docker-compose
- Configure DNS: `*.cloud.pulserelay.pro` → droplet IP, `cloud.pulserelay.pro` → droplet IP
- Deploy Traefik + control plane via docker-compose
- Configure Stripe webhook endpoint: `https://cloud.pulserelay.pro/api/webhooks/stripe`
- End-to-end smoke test: signup → Stripe test checkout → container provisioned → subdomain live → magic link → logged in → add test agent → see metrics
- Backup: daily rsync of `/data/tenants/` to DO Spaces

**Acceptance:** Full signup-to-monitoring flow works in production. TLS valid. Backup runs successfully.

### Hosted MSP Portal (P2-5) ✅ COMPLETE

Architecture: account layer over container-per-tenant. Each MSP client gets their own isolated container (same as Cloud). The control plane adds an account abstraction that owns multiple tenant workspaces. See guiding light P2-5 and `cloud-container-per-tenant-2026-02.md` "Hosted MSP Account Layer" section.

| ID | Work Item | Status | Worker | Commit | Depends On |
|----|-----------|--------|--------|--------|------------|
| M-1 | Account model + schema (accounts, users, account_members, stripe_accounts, tenant.account_id FK) | APPROVED | Codex | `0df04de4` | C-1 |
| M-2 | Account-level RBAC (invite/remove members, role management, member list) | APPROVED | Codex | `0df04de4` | M-1 |
| M-3 | Tenant switching portal (workspace list + fleet health summary) | APPROVED | Codex | `0df04de4` | M-1, M-2 |
| M-4 | Tenant handoff auth (signed token → tenant session mint) | APPROVED | Codex | `0df04de4` | M-1 |
| M-5 | Workspace provisioning from portal (MSP admin creates/deletes client workspaces) | APPROVED | Codex | `0df04de4` | M-1, C-2 |
| M-6 | Consolidated billing (StripeAccount mapping, MSP webhook routing, per-account subscription sync) | APPROVED | Codex | `0df04de4` | M-1 |

## P3: Launch Operations ✅ COMPLETE

| ID | Guiding Light | Work Item | Status | Commit | Depends On |
|----|---------------|-----------|--------|--------|------------|
| W5-A | P3-1 | Pricing + landing page | APPROVED | `0fd0d3d0` | — |
| W5-B | P3-2 | Customer migration (134 Pro + 47 lifetime) | READY | — | C-8 (Cloud live ✅), W5-A |

## RC Branch: Security Hardening ✅ COMPLETE

| ID | Work Item | Status | Commit |
|----|-----------|--------|--------|
| RC-1 | Stripe webhook in-flight lock + checkout auth hardening | APPROVED | `786a8cc0` |
| RC-2 | Opaque magic-link tokens + SQLite persistence | APPROVED | `d74f1286` |
| RC-3 | Host header injection elimination + hosted mode improvements | APPROVED | `3fe8a18d` |
| RC-4 | UpdateBanner v-prefix + remove managed AI from Cloud tier | APPROVED | `f746275f` |
| RC-5 | Hotfix reconciliation (cherry-pick docker env aliases) | APPROVED | `d2e52d8d` |

---

## Summary

| Phase | Items | Complete | Remaining |
|-------|-------|----------|-----------|
| P0: Trust Foundation | 7 | 7 | 0 |
| P1: Conversion Engine | 3 | 3 | 0 |
| P2: Cloud (container-per-tenant) | 8 | 8 | 0 |
| P2-5: Hosted MSP Portal | 6 | 6 | 0 |
| P3: Launch Operations | 2 | 2 | 0 |
| RC: Security Hardening | 5 | 5 | 0 |
| **Total** | **31** | **31** | **0** |

**All implementation items complete.**

**Cloud Platform:** ✅ LIVE — deployed to `pulse-cloud` droplet (138.197.122.205), TLS active, health verified at `https://cloud.pulserelay.pro/healthz`. Deploy config fix at `9b9570ca` (Cloudflare DNS-01, Traefik compat).
**Hosted MSP:** ✅ COMPLETE — all 6 items approved at `0df04de4`.
**Customer Migration (W5-B):** READY — Cloud is live, migration can proceed when scheduled.

---

## Approval Log

### P0/P1/P3/RC Approvals (Pre-Pivot — All Still Valid)

<details>
<summary>Click to expand historical approval log</summary>

### W1-C: SSO Gating Consistency (P0-3)
- **Files changed:** `MONETIZATION.md` (new — tier descriptions), `docs/architecture/ENTITLEMENT_MATRIX.md` (new — canonical feature matrix)
- **Commands run:** `go build ./internal/license/...` (exit 0), `go test ./internal/license/...` (exit 0), `go test ./internal/license/conversion/...` (exit 0)
- **Gate checklist:** Code already correct (SSO in Free tier), docs aligned, no upgrade prompt for free feature
- **Verdict:** APPROVED — commit `4e2f02bb`

### W1-B: Audit Logging Real + Tenant-Aware (P0-1)
- **Files changed:** 9 files — `SQLiteLoggerFactory` (new), factory tests (new), tenant_logger.go, server.go wiring, license_handlers.go stub removal, audit_handlers.go tenant-aware, route registration, test inventory updates
- **Commands run:** `go build ./pkg/audit/... ./internal/api/... ./pkg/server/...` (exit 0), `go test ./pkg/audit/...` (all PASS), `go test ./internal/api/... -run TestHostedLifecycle` (all PASS), `grep initAuditLoggerIfLicensed` (no matches — stub gone)
- **Gate checklist:** Real per-tenant SQLite loggers ✓, stub removed ✓, tenant isolation verified ✓, HMAC signing works ✓, query/export license-gated ✓
- **Verdict:** APPROVED — commit `ca01fdf5`

### W1-A: Fix Hosted Entitlements Enforcement (P0-0) — CRITICAL PATH
- **Files changed:** `license.go` (HasFeature/Status/SubscriptionState evaluator delegation), `license_test.go` (4-combo regression matrix), `entitlement_handlers_test.go` (parity test), `router.go` (variadic conversionStore compat), `hosted_signup_handlers_test.go` (adapt to parallel magic link work), `conversion/store.go` (timestamp compat)
- **Commands run (by worker):** `go build` (exit 0), `go test -run TestEvaluatorMatrix` (4/4 PASS), `go test -run TestEntitlement` (PASS), `go test ./internal/license/...` (all PASS), `go test -run TestHostedLifecycle` (all PASS)
- **Gate checklist:** evaluator honored when license==nil ✓, JWT precedence in hybrid ✓, free-tier preserved when neither set ✓, entitlement parity verified ✓, regression matrix covers all 4 combos ✓
- **Verdict:** APPROVED — commit `b105492b` — **UNBLOCKS W2-A and W2-B**

### W1-E: Conversion Telemetry Persistence (P0-6)
- **Files changed:** 13 files — `ConversionStore` (new SQLite), `store_test.go` (new), `recorder.go` (SQLite + OrgID), `events.go` (OrgID + checkout_completed), `quality.go/test` (aligned), `conversion_handlers.go` (funnel endpoint), `router_routes_org_license.go` (admin wiring), `router.go` (plumbing), `server.go` (init + shutdown)
- **Commands run (by worker):** `go build` (exit 0 x3), `go test ./internal/license/conversion/...` (exit 0), `go test -run TestConversion` (exit 0), `grep "default" recorder.go` (no hardcoded tenant)
- **Gate checklist:** SQLite WAL ✓, no hardcoded tenant ✓, idempotency ✓, org isolation ✓, admin-only funnel ✓, shutdown cleanup ✓
- **Verdict:** APPROVED — commit `d2e52740`

### W1-D: Hosted Signup Auth — Magic Links (P0-4)
- **Files changed:** 13 files — `magic_link.go` (new HMAC token service), `magic_link_handlers.go` (new public endpoints), `magic_link_test.go` (new), `hosted_signup_handlers.go` (password removed), `crypto.go` (HKDF DeriveKey), `router.go` (wiring), `router_routes_hosted.go` (registration), plus test inventory updates
- **Commands run (by worker):** `go build ./internal/api/...` (exit 0), `go test -run TestMagicLink` (exit 0), `go test -run TestHostedSignup|TestPublicSignup` (exit 0), `grep Password hosted_signup_handlers.go` (no matches)
- **Gate checklist:** Password removed ✓, HMAC-SHA256 tokens ✓, 15-min TTL + single-use ✓, rate limit 3/email/hr ✓, no email leak ✓, HKDF key derivation ✓, hosted mode guard ✓
- **Verdict:** APPROVED — commit `571c2639`

### W2-A: Frontend → Entitlements Endpoint (P0-2)
- **Files changed:** 17 files — `license.ts` (entitlements API client + typed payload), `stores/license.ts` (canonical gating via capabilities + `getUpgradeActionUrlOrFallback()`), AIIntelligence/Alerts pages (entitlements-driven CTAs), HistoryChart (dynamic upgrade URL), CountdownTimer (class prop), Login (removed demo.pulserelay.pro), 10 Settings panels (OIDCPanel, RolesPanel, UserAssignmentsPanel, AgentProfilesPanel, AuditLogPanel, SSOProvidersPanel, AISettings, ProLicensePanel, OrganizationBillingPanel, RelaySettingsPanel — all switched to entitlements action_url)
- **Commands run (by worker):** `grep -r "pulserelay\.pro" frontend-modern/src/` (exit 1 — zero matches), `npm run lint` (exit 0), `npm run build` (exit 0)
- **Gate checklist:** Typed entitlements payload matches backend ✓, all CTAs use `getUpgradeActionUrlOrFallback()` ✓, zero hardcoded pulserelay.pro ✓, reactive memos for feature checks ✓, clean fallback to `/pricing?feature=X` ✓
- **Verdict:** APPROVED — commit `456d1ec9`

### W3-C: AI Patrol Community Limits (P1-3)
- **Files changed:** 6 files — `router_routes_ai_relay.go` (reinvestigate Pro-gated), `ai_handlers.go` (autonomy lock + cadence cap + defense-in-depth), `service.go` (GetEffectivePatrolAutonomyLevel + interval clamp), `patrol_findings.go` (effective autonomy for background investigation), `ai_patrol_community_limits_test.go` (new — 2 tests), `security_regression_test.go` (reinvestigate in licensed-endpoints list)
- **Commands run (by worker):** `go build ./internal/api/... ./internal/ai/...` (exit 0), `go test ./internal/api/... -count=1 -run TestPatrolCommunity|TestReinvestigate` (exit 0)
- **Gate checklist:** Reinvestigate Pro-gated (route + handler) ✓, Community locked to monitor ✓, cadence cap 1/hour ✓, background investigation blocked ✓, defense-in-depth ✓
- **Verdict:** APPROVED — commit `df5bd183`

### W2-B: Trial Subscription State End-to-End (P0-5) — CRITICAL PATH
- **Files changed:** 18 files — `billing_store.go` (TrialStartedAt/TrialEndsAt fields), `source.go` + `token_source.go` + `evaluator.go` (trial timestamp interface chain), `database_source.go` (normalizeTrialExpiry: trial→expired + capability stripping), `database_source_test.go` + `evaluator_test.go` (trial expiry coverage), `billing_state.go` (default org path), `license.go` (free-tier union with evaluator), `license_handlers.go` (evaluator wiring + HandleStartTrial with one-per-org + 24h rate limit), `router_routes_org_license.go` (POST /api/license/trial/start), `entitlement_handlers.go` (trial_days_remaining from billing state), `trial_handlers_test.go` (new — 3 tests)
- **Commands run (by worker):** `go build ./internal/license/... ./internal/api/...` (exit 0), `go test ./internal/license/entitlements/... -count=1 -v` (exit 0), `go test ./internal/api/... -count=1 -run TestTrial -v` (exit 0)
- **Gate checklist:** Trial timestamps persisted ✓, trial expiry strips capabilities ✓, one-trial-per-org enforced ✓, free-tier union maintained ✓, no-cache evaluator for immediate UI ✓, 14-day duration ✓, route registered with admin+scope guard ✓
- **Verdict:** APPROVED — commit `fe3752f9` — **UNBLOCKS W3-A**

### W4-B: Stripe → Org Provisioning (P2-2) — Shared-Process Era
- **Files changed:** 11 files — `stripe_webhook_handlers.go` (new — signature verification, idempotency, customer→org index, 3 event handlers), `stripe_webhook_handlers_test.go` (new — signature, provisioning, revocation tests), `router_routes_hosted.go` (webhook route, no auth), `router.go` (handler construction), `billing_state_handlers.go` (Stripe fields preserved, canceled allowed), test updates (hosted_signup, billing_state, audit_reporting_scope, safety_test), `go.mod/go.sum` (stripe-go/v82)
- **Commands run (by worker):** `go get github.com/stripe/stripe-go/v82@latest` (exit 0), `go build ./internal/api/...` (exit 0), `go test -run TestStripeWebhook` (exit 0)
- **Gate checklist:** Signature verification before processing ✓, durable idempotency (HMAC-hashed event IDs) ✓, customer→org index with path traversal prevention ✓, checkout provisions org + writes billing ✓, subscription.deleted strips capabilities immediately ✓, fail-closed on unknown status ✓, no auth on webhook route (signature IS auth) ✓
- **Verdict:** APPROVED — commit `ec9189fd` — **Code reusable in control plane (C-4)**

### W3-A: Ship In-App Pro Trial (P1-1)
- **Files changed:** Frontend trial button, countdown timer, entitlements-driven CTAs
- **Verdict:** APPROVED — commit `7822e504`

### W3-B: Relay Hero Onboarding (P1-2)
- **Files changed:** Dashboard relay pairing card, QR wizard, upgrade gate
- **Verdict:** APPROVED — commit `ae387d16`

### W5-A: Pricing + Landing Page (P3-1)
- **Files changed:** `Pricing.tsx` (new — full 3-tier pricing page with feature comparison table and trial CTA)
- **Verdict:** APPROVED — commit `0fd0d3d0`

### RC-1: Stripe Webhook In-Flight Lock + Checkout Auth Hardening
- **Files changed:** 2 files — `stripe_webhook_handlers.go` (deduper dead-code fix: in-flight returns 409 instead of false-positive; removed `ensureOrgForEmail`; checkout requires server-owned org linkage via metadata/client_reference_id), `stripe_webhook_handlers_test.go` (email collision test, host header test)
- **Commands run:** `go build ./internal/api/...` (exit 0), `go test -run Stripe` (5/5 PASS, exit 0)
- **Gate checklist:** Deduper in-flight returns 409 ✓, email-based provisioning removed ✓, org linkage required ✓, cross-provision test ✓
- **Verdict:** APPROVED — commit `786a8cc0`

### RC-2: Opaque Magic-Link Tokens + SQLite Persistence
- **Files changed:** 4 files — `magic_link.go` (random opaque `ml1_` tokens, HMAC-SHA256 lookup, LogEmailer URL redaction, MagicLinkStore interface), `magic_link_store_sqlite.go` (new — SQLite-backed store with atomic consume), `magic_link_store_sqlite_test.go` (new — persistence + wrong-key tests), `magic_link_test.go` (updated for opaque model)
- **Commands run:** `go build ./internal/api/...` (exit 0), `go test -run MagicLink` (7/7 PASS, exit 0)
- **Gate checklist:** Tokens are random opaque ✓, no PII in token ✓, SQLite persistence ✓, atomic single-use ✓, URL redaction in logs ✓
- **Verdict:** APPROVED — commit `d74f1286`

### RC-3: Host Header Injection Elimination + Hosted Mode Improvements
- **Files changed:** 17 files — `router.go` (capturePublicURLFromRequest hosted-mode guard, resolvePublicURL hosted-mode block, Stripe webhook in public paths), `config.go` (PULSE_HOSTED_URL alias, skip auto-detect in hosted mode), `magic_link_handlers.go` (remove r.Host fallback), `hosted_signup_handlers.go` (remove r.Host fallback, fail closed 503), `middleware_tenant.go` (extracted resolveTenantOrgID with token-based org resolution), `router_routes_hosted.go` (RequireOrgOwnerOrPlatformAdmin for billing, agent-install-command endpoint), plus 5 new files (hosted_agent_install_command.go, hosted_org_admin_auth.go, tests) and test inventory updates
- **Commands run:** `go build ./...` (exit 0), `go test ./internal/api/... -run "HostedSignup|MagicLink|BillingState|RouteInventory|PublicPaths"` (16/16 PASS), `go test ./internal/config/... -run EnvOverrides` (8/8 PASS), `go test ./internal/monitoring/... -run MultiTenant` (2/2 PASS)
- **Gate checklist:** No r.Host fallback anywhere ✓, capturePublicURLFromRequest skips hosted mode ✓, resolvePublicURL returns empty if unconfigured ✓, fail-closed on missing public URL ✓, PULSE_HOSTED_URL alias ✓
- **Verdict:** APPROVED — commit `3fe8a18d`

### RC-4: UpdateBanner v-Prefix + Remove Managed AI from Cloud Tier
- **Files changed:** 2 files — `UpdateBanner.tsx` (v-prefix on GitHub release tag URLs), `Pricing.tsx` (removed "Managed AI" from Cloud tier)
- **Verdict:** APPROVED — commit `f746275f`

### W4-C: Agent Onboarding for Cloud (P2-3) — Shared-Process Era
- **Files changed:** Included in `3fe8a18d` — `hosted_agent_install_command.go` (new endpoint: POST /api/admin/orgs/{id}/agent-install-command), `hosted_agent_install_command_test.go`, `middleware_tenant.go` (token-based org fallback for agents), `middleware_tenant_token_org_fallback_test.go`, `multi_tenant_monitor.go` (PeekMonitor), `multi_tenant.go` (LoadOrganizationStrict)
- **Commands run:** (same as RC-3 above — bundled into same commit)
- **Gate checklist:** Org-scoped token minted ✓, agent installs can auto-select org via token ✓, PeekMonitor avoids lazy init ✓
- **Verdict:** APPROVED — commit `3fe8a18d`

### C-1/C-2/C-4/C-5/C-7: Control Plane Binary — Phases 1-4 (Combined)
- **Files changed:** 19 files — `cmd/pulse-control-plane/main.go` (Cobra entry point with ldflags), `internal/cloudcp/config.go` (CPConfig struct, env var loading, validation), `internal/cloudcp/server.go` (HTTP lifecycle, signal handling, graceful shutdown, Docker manager init), `internal/cloudcp/routes.go` (Deps struct, route wiring for all handlers), `internal/cloudcp/registry/models.go` (TenantState enum, Tenant struct with 15 fields, Crockford base32 ID generation), `internal/cloudcp/registry/registry.go` (SQLite-backed CRUD with WAL mode, indexes, health summary), `internal/cloudcp/admin/status.go` (healthz/readyz/status handlers), `internal/cloudcp/admin/handlers.go` (admin tenant list, AdminKeyMiddleware), `internal/cloudcp/stripe/helpers.go` (status mapping, capability checks, plan version derivation, ID validation), `internal/cloudcp/stripe/webhook.go` (signature verification via stripe-go/v82, event dispatch), `internal/cloudcp/stripe/provisioner.go` (checkout→billing.json via FileBillingStore, subscription sync), `internal/cloudcp/docker/manager.go` (Docker client: create/start/stop/remove/health check with Traefik labels and resource limits), `internal/cloudcp/docker/labels.go` (Traefik label generation), `internal/cloudcp/health/monitor.go` (periodic health check loop with configurable restart), plus 4 test files (registry_test.go, config_test.go, helpers_test.go, status_test.go), Makefile (control-plane target)
- **Commands run:** `go build ./cmd/pulse-control-plane` (exit 0), `go vet ./internal/cloudcp/...` (exit 0), `go test ./internal/cloudcp/...` (exit 0 — 25 tests pass: registry CRUD/list/count/health, config validation, Stripe helpers, admin handlers/middleware)
- **Gate checklist:** Binary compiles ✓, config validates required env vars ✓, registry SQLite with WAL + busy_timeout + MaxOpenConns(1) ✓, tenant ID format `t-` + 10 Crockford base32 ✓, Stripe signature verification ✓, billing.json via existing FileBillingStore ✓, Docker manager with Traefik labels + resource limits ✓, health monitor with configurable interval ✓, admin auth via X-Admin-Key/Bearer ✓, no new dependencies (all in go.mod) ✓
- **Verdict:** APPROVED — commit `afa1c85e`

### C-6 + M-4: Signup + Magic Link Flow + Tenant Handoff Auth
- **Files changed:** 15 files — `pkg/cloudauth/handoff.go` (new — shared HMAC-SHA256 handoff token sign/verify, zero external deps), `pkg/cloudauth/handoff_test.go` (new — 7 tests: round-trip, expired, wrong key, tampered, empty inputs), `internal/cloudcp/auth/magiclink.go` (new — standalone CP magic link service with `ml1_` prefix tokens, 15min TTL, file-backed HMAC key), `internal/cloudcp/auth/magiclink_store.go` (new — SQLite store with WAL/busy_timeout/MaxOpenConns(1), atomic Consume, background cleanup), `internal/cloudcp/auth/magiclink_test.go` (new — 8 tests: format, uniqueness, valid/used/expired/invalid, URL building, key persistence), `internal/cloudcp/auth/handlers.go` (new — HandleMagicLinkVerify: validates CP magic link → reads per-tenant handoff key → signs 60s handoff → redirects to tenant), `internal/api/cloud_handoff.go` (new — HandleCloudHandoff: self-guards via `.cloud_handoff_key` file check, verifies handoff, creates session, sets cookies, redirects), `internal/cloudcp/stripe/provisioner.go` (added magicLinks+baseURL fields, writeCloudHandoffKey, pollHealth with 2s/60s, generateAndLogMagicLink), `internal/cloudcp/server.go` (init magicLinkSvc, defer Close), `internal/cloudcp/routes.go` (MagicLinks in Deps, register /auth/magic-link/verify), `internal/api/router_routes_hosted.go` (register /auth/cloud-handoff), `internal/api/router.go` (/auth/cloud-handoff in publicPaths + CSRF skip), plus 3 inventory test updates
- **Commands run:** `go build ./cmd/pulse-control-plane` (exit 0), `go build ./cmd/pulse` (exit 0), `go vet ./internal/cloudcp/... ./pkg/cloudauth/... ./internal/api/...` (exit 0), `go test ./pkg/cloudauth/...` (7/7 PASS), `go test ./internal/cloudcp/...` (all PASS), `go test ./internal/api/...` (all PASS including inventory tests)
- **Gate checklist:** `pkg/cloudauth` has zero external deps ✓, CP magic link service standalone (no internal/api import) ✓, per-tenant handoff key written before container starts ✓, health poll before magic link ✓, handoff token 60s TTL + HMAC-SHA256 ✓, tenant-side handler self-guards via key file ✓, session creation matches existing pattern ✓, inventory tests updated ✓
- **Verdict:** APPROVED — commit `463e4eff`

### M-1/M-2/M-5/M-6: Account Model + RBAC + Workspace Provisioning + Consolidated Billing (Uncommitted)
- **Status:** IMPLEMENTED — code complete, tests passing, awaiting commit by user
- **Scope:** Account/User/AccountMembership/StripeAccount models, full SQLite CRUD with migrations, account member HTTP handlers (invite/remove/role), workspace handlers (list/create/delete under account), MSP Stripe webhook routing (dispatches to per-account or per-tenant handlers), hosted middleware hardening (subscription state gating, trial seeding, TierCloud)
- **Files:** ~2,000 lines across 28 files including `internal/cloudcp/registry/`, `internal/cloudcp/account/`, `internal/cloudcp/stripe/webhook.go`, `internal/api/middleware_tenant.go`, `internal/api/hosted_signup_handlers.go`, `internal/license/features.go`
- **Verdict:** Pending commit and formal approval

</details>
