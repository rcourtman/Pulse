# Paid Feature Entitlement Gating Record

- Date: `2026-03-12`
- Gate: `paid-feature-entitlement-gating`
- Environment:
  - Managed local backend: `http://127.0.0.1:61153`
  - Managed backend run id: `paid-feature-gate-20260312d`
  - Billing state path: `/Volumes/Development/pulse/repos/pulse/tmp/integration-local-backend/paid-feature-gate-20260312d/data/billing.json`
  - Authenticated user under test: `admin`
  - Runtime proof mode: deterministic local billing-state writes plus live browser/API exercise against the managed backend

## Automated Proof Baseline

- `go test ./internal/api -run 'TestEntitlementHandler_|TestRequireLicenseFeature_HostedEntitlements|TestLicenseGatedEmptyResponse_HostedEntitlements' -count=1`
- `go test ./internal/api -run 'TestMonitoredSystemLedger|TestHandleAddNode_BlocksNewCountedSystemAtLimit|TestHandleAutoRegister_BlocksNewCountedSystemAtLimit|TestTrueNASHandlers_HandleAdd_BlocksNewCountedSystemAtLimit|TestDockerAgentHandlers_HandleReport_BlocksNewMonitoredSystemAtLimit|TestKubernetesAgentHandlers_HandleReport_BlocksNewMonitoredSystemAtLimit|TestContract_EntitlementPayloadMonitoredSystemUsageJSONSnapshot' -count=1`
- `go test ./internal/license/... -count=1`
- `go test ./internal/cloudcp/... -count=1`
- `cd frontend-modern && npx vitest run src/pages/__tests__/AIIntelligence.test.tsx src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/shared/__tests__/MonitoredSystemLimitWarningBanner.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Result: pass

## Manual Exercise

1. Started an isolated managed local backend, then replaced the seeded billing file with a free/community-style state before the first authenticated browser session:
   - `subscription_state=expired`
   - `tier=free`
   - free capabilities resolved to `update_alerts`, `sso`, and `ai_patrol`
   - `trial_eligible=true`
   - `overflow_days_remaining=14`
2. Logged into the live backend as `admin` and confirmed the free surface failed closed:
   - `GET /api/license/entitlements` returned `subscription_state=expired`, `tier=free`, and 12 upgrade reasons.
   - `GET /api/license/status` returned `tier=free` and `max_monitored_systems=5`.
   - `/ai` rendered the live AI surface but kept both `Investigate` and `Auto-fix` disabled, with upgrade links visible.
   - Direct navigation to `/settings/organization/billing` failed closed by redirecting to `/settings` instead of rendering the billing panel.
   - `/settings/system-pro` rendered the activation/upgrade surface, including the activation controls and free-tier trial messaging.
3. Replaced the same billing file with a paid Enterprise-eval state before a fresh authenticated browser session:
   - `subscription_state=active`
   - `plan_version=enterprise_eval`
   - capabilities included `ai_alerts`, `ai_autofix`, `advanced_reporting`, `audit_logging`, `multi_tenant`, `rbac`, `relay`, and related paid features
   - limits were set to `max_monitored_systems=1` and `max_guests=25`
4. Confirmed the paid surface unlocked coherently:
   - `GET /api/license/entitlements` returned `subscription_state=active`, `tier=pro`, and `max_monitored_systems=1`.
   - `GET /api/license/status` returned `tier=pro` and `max_monitored_systems=1`.
   - `/ai` enabled both `Investigate` and `Auto-fix` and no longer showed `Upgrade to Pro`.
   - `/settings/organization/billing` rendered `Billing & Plan` with `Usage vs Plan Limits` and the monitored-system capacity surface.
5. Exercised live monitored-system accounting against that paid state:
   - First authenticated Unified Agent report to `POST /api/agents/agent/report` succeeded with `200`.
   - First authenticated Docker report to `POST /api/agents/docker/report` also succeeded with `200`.
   - `GET /api/license/entitlements` then reported `max_monitored_systems.current=1`, `max_monitored_systems.limit=1`, `state=enforced`, `docker_hosts=1`, and `has_migration_gap=true`.
   - The live upgrade banner rendered the monitored-system cap and showed the upgrade CTA.
   - When legacy-connected resources were also present, the migration guidance kept the same monitored-system term for both the counted limit and the non-counted legacy resources.
6. Confirmed the limit enforces only on new counted monitored systems while existing monitored systems continue:
   - A second new counted monitored system returned `402 license_required` with `feature=max_monitored_systems`.
   - A rereport from the existing host still returned `200`.
   - A second Docker report still returned `200`.
   - Final entitlements stayed at `max_monitored_systems.current=1` while `docker_hosts` increased to `2`.

## Outcome

- Free/community entitlements gated paid AI controls and multi-tenant billing surfaces consistently.
- Paid entitlements unlocked the same surfaces without leaving stale upgrade prompts behind.
- The Pro settings surface matched the active entitlement state on each fresh authenticated session.
- The live `max_monitored_systems` count tracked the canonical monitored-system surface.
- New counted monitored-system enrollment was blocked at the cap, while existing monitored-system reports and Docker reports continued to succeed.
