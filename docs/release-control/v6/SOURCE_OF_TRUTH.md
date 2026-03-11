# Pulse v6 Source Of Truth

Last updated: 2026-03-11
Status: ACTIVE

This is the canonical human source for v6 execution.

Machine companion:

- `docs/release-control/v6/status.json`

Recent locked release decision:

- 2026-03-11: Monitoring, frontend-primitives, and cloud-paid now ratchet
  toward no-default governance. Their subsystem registry entries require
  explicit path-policy coverage, monitoring/frontend guardrail tests now fail
  on the highest-risk forbidden-path regressions, and hosted billing-state
  normalization now preserves a missing `plan_version` instead of synthesizing
  one from `subscription_state`.
- 2026-03-11: Cloud-paid governance now explicitly owns the hosted control
  plane files that issue entitlements, canonicalize stored plan versions, and
  resolve Stripe provisioning plans. Changes to
  `internal/cloudcp/entitlements/service.go`,
  `internal/cloudcp/registry/registry.go`, and
  `internal/cloudcp/stripe/provisioner.go` now require the cloud-paid contract
  plus their path-specific proof files in the same slice.
- 2026-03-11: Cloud-paid governance now also explicitly owns JWT-backed
  entitlement claim evaluation and the activation grant bridge. Changes to
  `pkg/licensing/models.go` and `pkg/licensing/activation_types.go` now require
  the cloud-paid contract plus their dedicated proof files instead of falling
  through the generic cloud runtime policy.
- 2026-03-11: Cloud-paid governance now also explicitly owns billing-state
  canonicalization, hosted database-source loading, Stripe plan derivation,
  and the cloud plan limit table. Changes to
  `pkg/licensing/billing_state_normalization.go`,
  `pkg/licensing/database_source.go`, `pkg/licensing/features.go`, and
  `pkg/licensing/stripe_subscription.go` now require dedicated proof routes
  instead of relying only on the generic cloud runtime policy.
- 2026-03-11: Cloud-paid governance now also explicitly owns the runtime
  entitlement surface. Changes to `pkg/licensing/evaluator.go`,
  `pkg/licensing/token_source.go`, `pkg/licensing/entitlement_payload.go`, and
  `pkg/licensing/hosted_subscription.go` now require dedicated proof routes,
  and `pkg/licensing/token_source.go` now has direct coverage instead of being
  implicitly trusted through broader package tests.
- 2026-03-11: Cloud/MSP JWT claim handling now fails closed when `plan_version`
  is missing and no explicit `max_agents` limit is present. Runtime still
  preserves the missing `plan_version` metadata, but claim evaluation and
  `Service.Status()` no longer fall through to the unlimited Cloud/MSP tier
  default in that case.
- 2026-03-11: Canonical governance now runs in both local hooks and CI.
  `scripts/release_control/canonical_completion_guard.py` can validate either
  staged changes or an explicit diff file list, `.github/workflows/canonical-governance.yml`
  now runs the guard against PR/push diffs, and `internal/repoctl` now fails if
  `SOURCE_OF_TRUTH.md` and `status.json` drift on update date or source
  precedence.
- 2026-03-11: Canonical development protocol is now part of v6 governance.
  Substantial subsystem work must follow
  `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md` and update the
  relevant contract under `docs/release-control/v6/subsystems/`. Repo guardrail
  tests now fail if the protocol file or required subsystem contracts disappear
  or lose their required sections.
- 2026-03-07: Embedded frontend drift protection is now in place.
  `npm --prefix frontend-modern run build` syncs
  `internal/api/frontend-modern/dist`, and
  `internal/api/frontend_embed_sync_test.go` fails if the embedded copy drifts
  from `frontend-modern/dist`. The disposable upgraded CT now proves pending
  and failed v5 commercial migration states against the native embedded
  frontend with no `PULSE_FRONTEND_DIR` override.
- 2026-03-07: Browser-level proof now exists for unresolved v5 commercial
  migration states in
  `tests/integration/tests/12-v5-commercial-migration.spec.ts`. On the
  disposable upgraded fixture, both pending and failed paid-license migration
  states render the expected Pro settings notice and hide the trial CTA.
- 2026-03-07: v5→v6 commercial migration truth table is now owned in
  `docs/release-control/v6/V5_TO_V6_COMMERCIAL_MIGRATION_AUDIT_2026-03-07.md`.
  V6 persists unresolved paid-license migration state in billing/entitlements,
  blocks trial start while that state is pending or failed, and the migration
  suite now covers startup exchange failure classes that must preserve the v5
  key for downgrade safety. Browser-level upgraded-fixture UI proof remains a
  follow-up, not a release-control claim.
- 2026-03-06: Trial authority for v6 is SaaS-controlled. `POST /api/license/trial/start`
  must initiate hosted signup only and must not mint local trial state directly.
  The local runtime may redeem signed trial activation tokens via `/auth/trial-activate`,
  but local billing state is cache/redeem state, not trial issuance authority.
- 2026-03-06: v5→v6 license migration bridge landed in `pulse`; v6 now
  auto-exchanges persisted v5 Pro/Lifetime licenses on upgrade startup and
  accepts valid v5 Pro/Lifetime keys in the activation flow. Public v6
  migration guidance lives in `docs/UPGRADE_v6.md`, `docs/PULSE_PRO.md`, and
  `docs/releases/RELEASE_NOTES_v6.md`; `CHANGELOG.md` remains deferred until it
  is safe to publish on a public branch.
- 2026-03-05: host-type migration audit completed in
  `docs/release-control/v6/LEGACY_HOST_CLASSIFICATION_2026-03-05.md`;
  release can proceed because remaining `host` references are classified as
  compatibility boundaries, internal shims, or non-resource terminology.

## Scope

Active repositories for v6:

1. `pulse` (`/Volumes/Development/pulse/repos/pulse`)
2. `pulse-pro` (`/Volumes/Development/pulse/repos/pulse-pro`)
3. `pulse-enterprise` (`/Volumes/Development/pulse/repos/pulse-enterprise`)
4. `pulse-mobile` (`/Volumes/Development/pulse/repos/pulse-mobile`)

Ignored for v6 control:

- `pulse-5.1.x`
- `pulse-refactor-streams`

## Release Definition

Pulse v6 is ready when these outcomes land together:

1. Unified resource model is stable and expansion-ready.
2. Product quality feels polished and trustable out of the box.
3. Commercial packaging materially improves conversion and revenue.

## Must-Win Outcomes

1. Strong first-session product value without setup friction.
2. Paid paths feel inevitable via contextual upgrade + trial.
3. Relay/mobile acts as a concrete conversion driver.
4. Entitlements, gating, billing, docs, and UI all agree.
5. Hosted Cloud/MSP offering is operationally credible.

## Non-Negotiable No-Go Rules

1. Do not ship with open trust-critical P0s.
2. Do not ship when paywalls and runtime gates disagree.
3. Do not ship hosted flows that break signup/auth/provision/revocation.
4. Do not keep polishing strong lanes while weak lanes remain behind.

## Development Governance

For canonical subsystem work:

1. Read `docs/release-control/v6/SOURCE_OF_TRUTH.md` and
   `docs/release-control/v6/status.json` first.
2. Then read `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`.
3. Then read the relevant subsystem contract under
   `docs/release-control/v6/subsystems/`.
4. When a canonical path replaces an old path, add or tighten a guardrail so
   the old path cannot silently return.

## Repo Ownership Map

1. `pulse` owns core runtime, frontend, and shared contracts.
2. `pulse-enterprise` owns private paid in-app implementations behind contracts.
3. `pulse-pro` owns monetization infrastructure (license server, relay server, control plane, landing/checkout operations).
4. `pulse-mobile` owns mobile client UX and relay protocol consumption.

Cross-repo contracts that must not drift:

1. Licensing contract (`pulse/pkg/licensing` semantics vs license-server plans and gates).
2. Relay grant contract (license-server issued grants vs relay-server acceptance).
3. Relay protocol contract (`pulse/internal/relay`, `pulse-pro/relay-server`, `pulse-mobile/src/relay` wire compatibility).
4. Pricing contract (architecture pricing docs vs Stripe/checkout wiring in `pulse-pro`).

## Priority Engine

### Lane Catalog

| Lane ID | Lane | Target |
|---|---|---|
| `L1` | Self-hosted release confidence | 9 |
| `L2` | Conversion/commercial readiness | 9 |
| `L3` | Cloud paid readiness | 8 |
| `L4` | Hosted MSP readiness | 6 |
| `L5` | Mobile go-live readiness | 8 |
| `L6` | Architecture coherence | 8 |
| `L7` | Relay infrastructure readiness | 9 |
| `L8` | First-session & UX polish | 8 |
| `L9` | Documentation readiness | 8 |
| `L10` | Performance & scalability | 8 |
| `L11` | v5→v6 migration safety | 8 |
| `L12` | E2E journey coverage | 8 |

### Lane Scoring Rubrics

Each score level describes concrete, verifiable criteria. The orchestrator should
propose a score bump only when **all** criteria for the new level are met.
Items marked **(human)** require final user-facing deployment and cannot be completed
by the automation orchestrator — flag these via `needs_human_decision` when they
are the only remaining blocker for a score level. Most operational/preparation
tasks (Stripe product creation, server config, price ID insertion) are allowed
per the autonomy policy and do NOT require `(human)` tagging.

#### L1 — Self-hosted release confidence

Evidence: `docs/architecture/v6-acceptance-tests.md`

| Score | Criteria |
|-------|----------|
| 7 | Core API tests pass (auth, CSRF, resources, metrics, alerts, license, entitlements). Build green (`go build/test`, `vitest`, `tsc`). Playwright mobile suite green. Known issues documented. |
| 8 | All automatable `[API]` acceptance test items checked. Trust-critical code paths have test coverage (auth lifecycle, license gating, session invalidation, token expiry, CRL/revocation). No untested security-sensitive code paths remain. Code standards tests enforce ratchet ceilings. |
| 9 | All automatable checks pass. Only manual `[UI]`/device checks remain unchecked. Zero known test gaps in security, auth, licensing, or entitlement enforcement paths. Acceptance test sign-off table has PASS for all automatable areas. |

#### L2 — Conversion/commercial readiness

Evidence: `docs/architecture/release-readiness-guiding-light-2026-02.md`, `docs/architecture/conversion-operations-runbook.md`, `V6_LAUNCH_CHECKLIST.md`

| Score | Criteria |
|-------|----------|
| 7 | Pricing strategy locked. Frontend paywall gates complete (all panels gated, upgrade links, trial buttons, paywall tracking). Upgrade metrics telemetry pipeline working (frontend emission, API endpoints, Prometheus counters, SQLite persistence). |
| 8 | Backend-emitted conversion events wired (trial_started, license_activated, license_activation_failed, checkout_started, limit_warning_shown, limit_blocked). Conversion funnel is end-to-end observable. All code-side conversion/commercial work complete — only operational tasks remain (Stripe product creation, landing page deploy, customer comms). |
| 9 | Stripe products created, price IDs filled into configs, launch checklist sections 2-10 executed (Stripe product creation and config updates are allowed prep ops). Backend conversion events end-to-end verifiable. All operational configs and API keys are in pulse-pro (see OPERATIONS.md). **(human)** for final landing page deployment and customer migration comms only. |

#### L3 — Cloud paid readiness

Evidence: `docs/architecture/v6-pricing-and-tiering.md`

| Score | Criteria |
|-------|----------|
| 6 | Self-hosted pricing/entitlements code complete (tier definitions, agent limits, history days, frontend pricing page, license server plan definitions). Frontend gating audit gaps fixed. |
| 7 | Cloud control plane per-tier agent limits enforced (Starter=10, Power=30, Max=75 instead of unlimited). Cloud billing team management API has test coverage. Workspace count safety rails implemented. |
| 8 | All code-side Cloud paid readiness complete. Cloud control plane Stripe products created and wired, team management portal functional. Stripe API keys and deploy access available in pulse-pro (see OPERATIONS.md). |

#### L4 — Hosted MSP readiness

Evidence: `docs/architecture/release-readiness-guiding-light-2026-02.md`

| Score | Criteria |
|-------|----------|
| 5 | Architecture locked. Account model, RBAC, tenant switching, billing model all approved. Exit criteria defined. Dependencies on Cloud control plane (P2-1, P2-2) identified. |
| 6 | MSP portal implementation requires Cloud control plane to be functional (P2-1/P2-2 dependency). Build account/tenant CRUD endpoint scaffolding, integration test skeletons, and wire into existing control plane. Cloud control plane is deployed at cloud.pulserelay.pro — SSH and deploy access available in pulse-pro (see OPERATIONS.md). Flag `needs_human_decision` only if a true external account creation (e.g. third-party vendor signup) is required. |

#### L5 — Mobile go-live readiness

Evidence: `V6_LAUNCH_CHECKLIST.md`, `pulse-mobile/store/listing.md`

| Score | Criteria |
|-------|----------|
| 6 | Store listing metadata complete (name, description, keywords, privacy policy, screenshots). Mobile app feature-gated behind Pro license. Relay protocol wire compatibility verified. |
| 7 | Mobile-specific code hardening complete: error handling for relay disconnects, offline states, biometric auth edge cases tested. Any mobile-side test gaps closed. |
| 8 | All code-side mobile readiness complete. App Store Connect and Google Play Console listings created, builds submitted for review via EAS CLI. Apple Developer account (Team ID `UJD57YVK2B`) and EAS project are configured in pulse-mobile. Flag `needs_human_decision` only if store account creation or manual portal steps are required that cannot be completed via CLI. |

#### L6 — Architecture coherence

Evidence: `docs/architecture/state-read-consolidation-progress-2026-02.md`, `pulse-enterprise/docs/V6_REPO_REALIGNMENT.md`

| Score | Criteria |
|-------|----------|
| 6 | SRC-00 baseline frozen. Code standards ratchet ceilings enforced and tightened to match actual state. Repo realignment Phase 2 substantially complete (licensing extracted, RBAC/Audit/Reporting/SSO migrated to pulse-enterprise). |
| 7 | SRC-01a through SRC-01d complete (typed view accessors, ReadState interface, cached indexes, view layer tests). Repo realignment Phase 1 boundary audit done (paid-domain code identified in public paths). |
| 8 | SRC-02 through SRC-04b complete (parity harness, patrol/tools/API migration to ReadState, enforcement via code standards test with zero GetState() in consumers). Repo realignment Phase 3 governance in place. |

#### L7 — Relay infrastructure readiness

Evidence: `pulse-pro/relay-server/`, `pulse/internal/relay/`, `pulse-mobile/src/relay/`, `pulse/docs/RELAY.md`

| Score | Criteria |
|-------|----------|
| 7 | Relay server deployed and healthy at relay.pulserelay.pro. Wire protocol contract tests pass across all three components (relay-server, pulse client, mobile client). Legacy JWT license validation working. Rate limiting and encryption implemented. |
| 8 | V6 relay grant validation implemented and tested (iss/aud claims, state validation, revocation feed integration). Push notifications (APNs + FCM) functional with rate limiting. Deployment workflow includes canary health check and automatic rollback. 90+ tests per component passing. |
| 9 | Pre-launch relay verification complete: v6 grant validator configured in production (`PULSE_RELAY_LICENSE_SERVER_URL` + `PULSE_RELAY_REVOCATION_FEED_TOKEN`), revocation feed polling confirmed, legacy JWT fallback tested, push notification credentials loaded. End-to-end relay flow verified (Pulse instance → relay server → mobile client). Server credentials and SSH access in pulse-pro OPERATIONS.md. |

#### L8 — First-session & UX polish

Evidence: `frontend/src/pages/setup/`, `frontend/src/pages/settings/`, `tests/integration/`

**SCOPE RESTRICTION:** This lane covers **test coverage, error handling, and verification** only.
The orchestrator must NEVER change visual styling, layouts, colors, spacing, animations, component structure, or any user-visible UI behavior. All visual/UX design decisions are reserved for the project owner. Allowed work: adding E2E tests, adding missing error/loading state checks in existing code, fixing flash-of-content bugs (logic-only, not visual), and running responsive audits (reporting results, not changing CSS).

| Score | Criteria |
|-------|----------|
| 6 | Setup wizard functional (Welcome → Security → Complete). Basic settings panels present. Login flow works with password auth. |
| 7 | Setup wizard polished (auto-token detection, platform-aware commands, credential vault, agent polling). 25+ settings panels organized by category with consistent styling. Paywall gates complete on all Pro/Enterprise features (F1-F5 fixed). SSO/OIDC login flow integrated. E2E Playwright tests cover core flows (bootstrap, login, alerts, node setup). |
| 8 | All empty/loading/error states handled across settings panels (logic guards, not visual changes). Dedicated first-session E2E test (wizard → dashboard → settings discovery). Mobile responsive audit documented (report only — no CSS changes). Theme visual tests passing. Trial activation flow E2E tested. No flash-of-unlocked-content on any gated panel. |

#### L9 — Documentation readiness

Evidence: `docs/`, `docs/releases/RELEASE_NOTES_v6.md`, `README.md`, `pulse-pro/landing-page/`

| Score | Criteria |
|-------|----------|
| 6 | Core docs exist: installation, Docker, Kubernetes, reverse proxy, API reference. README has feature overview and quick start. |
| 7 | V6-specific docs complete: UPGRADE_v6.md migration guide, MIGRATION_UNIFIED_NAV.md route mapping, RELEASE_NOTES_v6.md comprehensive public release notes, PULSE_PRO.md entitlements matrix. 40+ doc files covering all features. Docs index (docs/README.md) well-organized with section groupings. |
| 8 | All v6 docs cross-referenced and consistent. Landing page (pulserelay.pro) v6 pricing integrated with Stripe price IDs filled in. Cloud/MSP pages linked from main navigation. No stale v5-only references in user-facing docs. Advanced topic guides present (AI autonomy, RBAC, audit logging). Stripe price creation and ID insertion are allowed prep ops. **(human)** for final landing page deployment only (user-visible). |

#### L10 — Performance & scalability

Evidence: `frontend-modern/src/components/Dashboard/__tests__/Dashboard.performance.contract.test.tsx`, `tests/integration/02-navigation-perf.spec.ts`, `internal/api/http_metrics.go`, `pkg/metrics/store.go`

| Score | Criteria |
|-------|----------|
| 6 | Frontend virtualization in place (TanStack Virtual, canvasRenderQueue). SQLite optimized (WAL, indexes, busy timeouts). Prometheus HTTP metrics collecting request durations. E2E navigation perf budget gates passing (2.2s median). Performance contract tests for Dashboard windowing (Profiles S/M/L up to 5000 rows). |
| 7 | `/debug/pprof` endpoints added with auth gate (admin-only). Go benchmarks for hot paths (metrics store writes, resource dedup, RBAC lookups). Slow query logging with 100ms threshold. API latency SLOs documented per critical endpoint. Frontend bundle size measured and baselined. |
| 8 | Load test scripts (k6 or similar) for 500-node deployment simulation. Frontend bundle size budget enforced in CI. Multi-tenant stress test (20 orgs concurrent). All Go benchmarks baselined and tracked in CI. Query plan validation for critical SQLite queries (EXPLAIN QUERY PLAN in tests). |

#### L11 — v5→v6 migration safety

Evidence: `internal/config/`, `internal/api/`, `tests/migration/`

Supplementary evidence:

- `tests/integration/tests/12-v5-commercial-migration.spec.ts` proves
  unresolved paid-license migration states in a real browser against an
  upgraded v5 fixture. Pending and failed states render the expected Pro panel
  notice and suppress the trial CTA.
- `internal/api/frontend_embed_sync_test.go` enforces that the embedded
  frontend copy matches `frontend-modern/dist`, and
  `frontend-modern/scripts/sync-embed-dist.mjs` keeps the embed directory in
  sync as part of `npm --prefix frontend-modern run build`.

| Score | Criteria |
|-------|----------|
| 6 | V5 config format documented (ai.enc, nodes.enc, alerts/, .encryption.key structure). V6 binary can start against a v5 data directory without crashing. Basic migration test exists (v5 fixture → v6 startup → health check). |
| 7 | Migration test suite covers: encrypted config roundtrip (ai.enc, nodes.enc survive decryption under v6), alert config forward compatibility, session/CSRF token continuity, database schema auto-migration (SQLite tables created/altered as needed). Edge cases tested: empty data dir, corrupt encryption key, missing optional files. |
| 8 | Full upgrade scenario test: v5 fixture with realistic data (3 nodes, 5 alerts, AI config, metrics history) → v6 binary startup → API health check → verify all resources accessible via API → verify license state preserved → verify no data loss. Downgrade safety documented (what happens if user reverts to v5 binary). Upgrade guide (UPGRADE_v6.md) verified against actual migration test results. |

#### L12 — E2E journey coverage

Evidence: `tests/integration/tests/journeys/`

**PURPOSE:** Prove that v6 user journeys work end-to-end in a real browser. Unit tests and API tests don't catch routing bugs, missing UI gates, broken WebSocket flows, or integration failures between frontend and backend. This lane closes that gap.

| Score | Criteria |
|-------|----------|
| 0 | Lane defined, no orchestrator integration. |
| 1 | Orchestrator plumbing complete (allowlist, lane requirements, runner command). |
| 2 | Stable smoke journeys green: bootstrap → first login → dashboard renders. |
| 3 | TrueNAS node addition → pools/datasets visible. Relay pairing → mobile connection → live data. |
| 4 | Agent install → registration → host visible in UI/API. |
| 5 | SAML SSO → IdP login → role-mapped access. |
| 6 | Audit log → webhook delivery → log viewer. Reporting → scheduled report → export download. |
| 7 | AI patrol → finding → approval → fix → verify resolved (closed-loop). |
| 8 | Full journey set green with stability SLO (>=95% pass rate over 7 consecutive runs). |

**Lane-specific prompt guidance for orchestrator:**

Test authoring:
- Tests live in `tests/integration/tests/journeys/` with naming `XX-journey-name.spec.ts`.
- Use existing helpers from `tests/integration/tests/helpers.ts`: `ensureAuthenticated`, `login`, `apiRequest`, `setMockMode`, `waitForPulseReady`, `maybeCompleteSetupWizard`, etc.
- Follow existing test patterns (see `00-diagnostic.spec.ts` through `11-first-session.spec.ts` for examples).
- Each journey test file must be self-contained and independently runnable.
- `fullyParallel: false` and `workers: 1` are already set in `playwright.config.ts` — do not change these.

Verification commands (CRITICAL — tests fail if run from repo root):
- All Playwright verification commands MUST use: `cd tests/integration && npx playwright test tests/journeys/ --project=chromium --reporter=list`
- The `cd tests/integration &&` prefix is required because `playwright.config.ts` is in `tests/integration/`, not the repo root. Without it, npx resolves a different Playwright version from the global cache and tests fail with "project not found" or version mismatch errors.
- Never provide bare `npx playwright test ...` without the `cd` prefix in `test_evidence.commands`.

Two test environments (use the right one for each journey):

1. **Local dev** (`localhost:7655`) — for UI-centric journeys that work with mock data:
   - Bootstrap → login → dashboard renders (journey 1)
   - RBAC role assignment/enforcement (journey 7)
   - Audit log viewer, reporting export (journey 9, 10)
   - Paywall gates, settings panels, navigation
   - Enable mock mode for deterministic data: `setMockMode(page, true)`
   - Run: `cd tests/integration && npx playwright test tests/journeys/`

2. **Private LXC sandbox** — for journeys that need real infrastructure:
   - Add Proxmox node → VMs appear → metrics flowing (journey 2)
   - Add TrueNAS node → pools/datasets visible (journey 3)
   - Relay pairing → mobile connection → live data (journey 4)
   - Agent install → registration → host visible (journey 5)
   - SAML SSO → IdP login → role-mapped access (journey 6)
   - AI patrol → finding → approval → fix → verify (journey 8)
   - Requires a disposable snapshot-capable sandbox with Pulse and any dependent services already installed
   - Existing automation: `tests/integration/scripts/run-lxc-sandbox-evals.sh` handles rollback → start → tunnel → run
   - Runbook: keep exact host/container identifiers in local ops notes, not in repo-tracked docs
   - Run from workspace root by supplying your local sandbox env: `PVE_HOST=... PVE_CTID=... bash tests/integration/scripts/run-lxc-sandbox-evals.sh`
   - **IMPORTANT**: The sandbox runner currently covers trial/cloud/multi-tenant scenarios only. For new infrastructure journeys (Proxmox, TrueNAS, agent, SAML, patrol), add scenario definitions in `tests/integration/evals/scenarios.json`, add Playwright spec files, and wire the new scenario group into `run-lxc-sandbox-evals.sh`.
   - **SAFETY**: Use only disposable test infrastructure. Do not target shared or production-adjacent hosts from this workflow.

LXC sandbox workflow (snapshot-clean):
- Restore the sandbox from a clean snapshot before each test suite
- Ensure `pulse.service` and any companion services auto-start inside the sandbox
- Establish local SSH tunnels to the sandbox ports needed by the test suite
- Use only dedicated test credentials and bootstrap state
- Each test run should start from identical filesystem state with no cross-run pollution

### Formula

For each lane:

`gap = max(0, target_score - current_score)`

`behind_score = (gap * 4) + criticality + staleness + dependency + blocker_bonus`

Ranges:

- `criticality`: 0-5
- `staleness`: 0-3
- `dependency`: 0-3
- `blocker_bonus`: 8 if launch-blocking item open, else 0

### Selection Rules

1. Recompute all lane scores at session start.
2. Apply floor rule before ranking.
3. Choose top lane by `behind_score`.
4. Execute one smallest complete task that moves lane score or closes a blocker.

Floor rule:

- If any release-critical lane (`L1-L3`, `L7-L12`) is below 6, choose work from below-floor lanes first.

Hard override:

- Any new P0 trust/security/revenue issue takes priority over numeric ranking.

## Session Contract

Every new session starts with:

1. Most behind lane
2. Why behind (with file evidence)
3. Exact next task (single task)
4. Definition of done
5. Risks
6. Time box
7. Escalation check (approval needed or not)

Every session ends with:

1. What changed
2. Proof added
3. Lane score delta
4. Next best task

## Product Review Sweep

### Purpose

Periodic sweep where the orchestrator adopts a user persona, reviews a UI area
through that persona's lens, and reports objective UX defects. Findings feed back
into the implementation loop — objective defects can be auto-fixed (when enabled),
subjective improvements are logged for human approval.

### Personas

| Persona | Focus |
|---------|-------|
| `first_time_user` | Onboarding, discoverability, empty states |
| `infra_admin` | Efficiency, clarity under pressure, actionability |
| `product_designer` | Hierarchy, consistency, error recovery, flows |
| `architect` | Abstractions, naming, complexity exposure |
| `buyer_evaluator` | Value perception, trial experience, upgrade motivation |

Personas rotate each sweep (configurable in `loop.config.json`).

### Scheduling

- Runs every N cycles (default: 12), configurable via `product_review_sweep.every_n_cycles`.
- Priority: assessment > discovery > product review > implementation.
- Skipped during hardening mode.

### Finding Lifecycle

`detected → triaged → (accepted_auto | pending_human | rejected) → queued → in_progress → fixed_pending_verification → verified_closed`

Side paths: `reopened`, `deferred`.

### Tiers

- **auto_fix**: Objective defect in a safe change class (loading/error/empty state, recovery CTA, guard logic). All six auto-fix criteria must be met.
- **human_review**: Everything else — subjective, structural, risky.

### Auto-Fix Rubric (all must be true)

1. Reproducible with evidence (specific code path)
2. User harm is objective (dead-end, missing state, raw error, broken recovery)
3. Fix is in a safe change class
4. No structural UX change (no layout/IA/nav/pricing/copy)
5. Small blast radius (few files, no API contract changes)
6. High confidence

### Anti-Churn Controls

1. **Fingerprint dedup** — same finding cannot be created twice
2. **Area cooldown** — reviewed area skipped for N cycles (default: 8)
3. **Reopen limit** — 3 reopens forces human decision
4. **Auto-fix budget** — max 2 per 10-cycle window
5. **Codex admission gate** — when wired, findings must pass quality review (currently stubbed: findings auto-admit with `gate_pending_not_wired` verdict)
6. **Persona rotation** — different perspective each sweep
7. **Max findings per sweep** — capped at 3, quality over quantity

### Configuration

See `loop.config.json` key `product_review_sweep`. `auto_fix_enabled` is `false` by default — starts read-only. Enable after reviewing finding quality.

### Storage

Findings stored in `tmp/release-control/product-review-findings.json`.

## Autonomy Policy

Allowed without approval:

1. Read-only analysis across active v6 repos.
2. Sacred status updates with evidence.
3. Local test/lint/build verification.
4. Low-risk implementation and hardening with tests.
5. Local git commits (checkpoint commits on the working branch).
6. V6 preparation ops that do NOT affect current users (see below).

Allowed preparation operations (v6 prep that current users never see):

1. Creating Stripe products/prices for v6 tiers (not yet linked from checkout flows).
2. Configuring server env vars for unreleased v6 features (relay v6 grants, revocation feed tokens).
3. SSH to servers to set up v6 infrastructure that is not yet active (new env vars, new config files).
4. Filling in price IDs, API keys, and config values in pulse-pro operations files.
5. Setting up CI/CD pipelines and build tooling for v6 features.
6. Any server-side configuration that existing v5 users cannot see or be affected by.

All operations credentials and SSH access are in `pulse-pro/OPERATIONS.md`.

PROHIBITED — autonomous cycles must NEVER do these (no exceptions):

1. Push to remote repositories (no `git push`, `gh pr create`, or similar).
2. Deploy user-facing pages (no landing page deploys, no marketing site updates).
3. Create releases, tags, or publish artifacts (no `gh release`, `git tag`, `npm publish`).
4. Modify anything current v5 users interact with (no changing active checkout links, live DNS, running service behavior).
5. Send external communications (no emails, no Slack messages, no GitHub issue/PR comments).
6. Trigger auto-updates or notifications to existing users.

Rule: the line is "can current users see or be affected by this change?" If yes, don't do it. If no, proceed.

Requires manual session (not autonomous):

1. Final launch deployment (flipping v6 live for users).
2. Customer migration communications.
3. Irreversible migrations/destructive operations.

## Current Lane Snapshot (Derived Baseline)

| Lane | Current | Status | Evidence |
|---|---|---|---|
| `L1` Self-hosted confidence | 9 | PARTIAL | `docs/architecture/v6-acceptance-tests.md` |
| `L2` Conversion/commercial | 9 | PARTIAL | `docs/architecture/release-readiness-guiding-light-2026-02.md`, `docs/architecture/conversion-operations-runbook.md`, `/Volumes/Development/pulse/repos/pulse-pro/V6_LAUNCH_CHECKLIST.md` |
| `L3` Cloud paid | 7 | PARTIAL | `docs/architecture/v6-pricing-and-tiering.md` |
| `L4` Hosted MSP | 6 | PARTIAL | `docs/architecture/release-readiness-guiding-light-2026-02.md` |
| `L5` Mobile | 7 | PARTIAL | `/Volumes/Development/pulse/repos/pulse-pro/V6_LAUNCH_CHECKLIST.md`, `/Volumes/Development/pulse/repos/pulse-mobile/store/listing.md` |
| `L6` Architecture coherence | 7 | PARTIAL | `docs/architecture/state-read-consolidation-progress-2026-02.md`, `/Volumes/Development/pulse/repos/pulse-enterprise/docs/V6_REPO_REALIGNMENT.md` |
| `L7` Relay infrastructure | 9 | PARTIAL | `pulse-pro/relay-server/`, `pulse/internal/relay/`, `pulse-mobile/src/relay/` |
| `L8` First-session & UX | 8 | PARTIAL | `frontend/src/pages/setup/`, `frontend/src/pages/settings/`, `tests/integration/` |
| `L9` Documentation | 8 | PARTIAL | `docs/`, `docs/releases/RELEASE_NOTES_v6.md`, `README.md`, `pulse-pro/landing-page/` |
| `L10` Performance & scalability | 6 | PARTIAL | `frontend-modern/src/components/Dashboard/__tests__/`, `tests/integration/02-navigation-perf.spec.ts`, `internal/api/http_metrics.go` |
| `L11` v5→v6 migration safety | 8 | PARTIAL | `tests/migration/v5_full_upgrade_test.go`, `tests/migration/v5_to_v6_test.go`, `tests/migration/v5_session_db_test.go`, `docs/UPGRADE_v6.md` |
| `L12` E2E journey coverage | 8 | PARTIAL | `tests/integration/tests/journeys/` |

## Decision Lock (Delegated)

Decision date: 2026-02-27
Decision owner: delegated to Codex by project owner

1. Release policy is staged:
v6 GA is gated by `L1`, `L2`, `L3`, `L5`, `L6`, `L7`, `L8`, `L9`, `L10`, `L11`, `L12`.
`L4` (Hosted MSP full account portal scope) is post-GA track work and is not a GA floor gate.
2. Hosted MSP pricing is locked:
`Starter` = up to 10 workspaces = $149/mo ($1,490/yr),
`Growth` = up to 25 workspaces = $249/mo ($2,490/yr),
`Scale` = up to 50 workspaces = $399/mo ($3,990/yr).
Per-workspace breakpoint anchors are $15 / $10 / $8 monthly.
3. Cloud/MSP Stripe mapping contract is locked:
engineering uses fixed plan keys and env/config slots now;
inserting live Stripe `price_*` values is an operational launch task, not a strategic blocker.

## Remaining Operational Tasks

1. Create Cloud/MSP Stripe prices in Stripe dashboard.
2. Fill concrete Cloud/MSP Stripe `price_*` IDs in:
`pulse-pro` operations docs, launch checklist, and runtime env mappings.

## Legacy Doc Consolidation Policy

Legacy v6 architecture docs are retained for evidence and deep implementation detail,
but they are not primary planning authority for new session task selection.

Rule:

1. Plan from this file + `status.json`.
2. Pull old docs only to verify evidence for the chosen task.
3. If old docs and this file differ, update old docs to match or explicitly archive them.

Consolidation map:

- `docs/release-control/v6/CONSOLIDATION_MAP.md`
- `docs/release-control/v6/RETIREMENT_AUDIT_2026-02-27.md`

## Source Precedence

If conflicts appear, resolve in this order:

1. This file
2. `docs/release-control/v6/status.json`
3. `docs/architecture/release-readiness-guiding-light-2026-02.md` (evidence/spec)
4. `docs/architecture/v6-pricing-and-tiering.md` (evidence/spec)
5. `docs/architecture/ENTITLEMENT_MATRIX.md` (evidence/spec)
6. `docs/architecture/v6-acceptance-tests.md` (evidence/spec)
7. Other supporting docs

## Parallel Execution

The orchestrator supports parallel execution of independent work items via the
`parallel` section in `loop.config.json`.

### Worker Classes

| Worker | Max Concurrent | Cycle Types | Rationale |
|--------|---------------|-------------|-----------|
| `mutating` | 1 | implementation, discovery | Writes git, may update scores |
| `readonly` | 1 | assessment, product review | Read-only, but each runs Claude |
| `sweep` | 1 | regression (`go test`), discovery sweep | CPU-bound subprocesses |

A `claude_semaphore` (default max=2) caps concurrent Claude API calls across
mutating + readonly workers to control spend and rate limits.

**Current phase**: Only sweep-level parallelism is active — sweeps are
dispatched to the sweep pool after the primary cycle completes and run
concurrently with the *next* iteration's primary cycle. Concurrent
mutating + readonly cycles (e.g., implementation + assessment in the same
iteration) require git worktree isolation because assessment/product_review
enforce HEAD-stability checks that would fail if implementation commits
during their run. This is planned for phase 2.

### State Applier (Single-Writer)

A dedicated `StateApplier` thread receives `CycleResult` objects via a queue
and applies them sequentially. Optimistic concurrency via `status_version`
prevents stale results from overwriting newer state.

**Current phase**: The infrastructure is in place but `run_single_cycle` /
`_finalize_cycle` still write `status.json` directly. The applier will be
wired as the exclusive write path in a follow-up change once the concurrent
worker pools are proven stable.

### Runtime Index

Per-run state files live under `tmp/release-control/runtime/runs/<run_id>.json`.
An `index.json` tracks active run IDs. For backward compatibility with
`loopctl.sh`, the most recently active run is projected into the legacy
`tmp/release-control/runtime.json` format.

### Graceful Shutdown

On stop signal (`stop-after-cycle` file or SIGTERM):
1. `scheduler.stop_event` is set (interrupts sleep)
2. No new work is accepted
3. Active runs drain up to `graceful_shutdown_timeout_seconds` (default 120s)
4. `StateApplier` queue is drained
5. Exit 0

### Config

```json
"parallel": {
    "enabled": true,
    "max_claude_concurrent": 2,
    "max_mutating_concurrent": 1,
    "max_readonly_concurrent": 1,
    "max_sweep_concurrent": 1,
    "graceful_shutdown_timeout_seconds": 120
}
```

When `enabled: false`, the orchestrator runs sequentially (identical to the
pre-parallel behaviour).

## Lean-Mode Rule For Agents

For v6 execution, agents must read only:

1. This file
2. `docs/release-control/v6/status.json`

Only open additional docs when needed for direct evidence on the active task.
