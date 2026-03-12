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
- `go test ./internal/api -run 'TestAgentCountNilMonitor|TestLegacyConnectionCountsFromReadState|TestLegacyConnectionCountsUsesSnapshotFallback|TestDeployReservedCount|TestHostAgentHandlers_HandleReport_EnforcesMaxAgentsForNewHostsOnly|TestHandleAddNode_NoLimitForConfigRegistration|TestHandleAutoRegister_NoLimitForConfigRegistration|TestDockerAgentHandlers_HandleReport_NoLimitForDockerReports|TestKubernetesAgentHandlers_HandleReport_NoLimitForK8sReports|TestTrueNASHandlers_HandleAdd_NoLimitForTrueNAS|TestBuildEntitlementPayloadWithUsage_CurrentValues' -count=1`
- `go test ./internal/license/... -count=1`
- `go test ./internal/cloudcp/... -count=1`
- `cd frontend-modern && npx vitest run src/pages/__tests__/AIIntelligence.test.tsx src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/shared/__tests__/AgentLimitWarningBanner.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
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
   - `GET /api/license/status` returned `tier=free` and `max_agents=5`.
   - `/ai` rendered the live AI surface but kept both `Investigate` and `Auto-fix` disabled, with upgrade links visible.
   - Direct navigation to `/settings/organization/billing` failed closed by redirecting to `/settings` instead of rendering the billing panel.
   - `/settings/system-pro` rendered the activation/upgrade surface, including the activation controls and free-tier trial messaging.
3. Replaced the same billing file with a paid Enterprise-eval state before a fresh authenticated browser session:
   - `subscription_state=active`
   - `plan_version=enterprise_eval`
   - capabilities included `ai_alerts`, `ai_autofix`, `advanced_reporting`, `audit_logging`, `multi_tenant`, `rbac`, `relay`, and related paid features
   - limits were set to `max_agents=1` and `max_guests=25`
4. Confirmed the paid surface unlocked coherently:
   - `GET /api/license/entitlements` returned `subscription_state=active`, `tier=pro`, and `max_agents=1`.
   - `GET /api/license/status` returned `tier=pro` and `max_agents=1`.
   - `/ai` enabled both `Investigate` and `Auto-fix` and no longer showed `Upgrade to Pro`.
   - `/settings/organization/billing` rendered `Billing & Plan` with `Usage vs Plan Limits` and `Agents 0 / 1`.
5. Exercised live agent-limit accounting against that paid state:
   - First authenticated host-agent report to `POST /api/agents/agent/report` succeeded with `200`.
   - First authenticated Docker report to `POST /api/agents/docker/report` also succeeded with `200`.
   - `GET /api/license/entitlements` then reported `max_agents.current=1`, `max_agents.limit=1`, `state=enforced`, `docker_hosts=1`, and `has_migration_gap=true`.
   - The live upgrade banner rendered `v6 Host Agents: 1/1` and showed the upgrade CTA.
6. Confirmed the limit enforces only on new v6 host agents while existing agents and non-host reports continue:
   - A second new host-agent report returned `402 license_required` with `feature=max_agents`.
   - A rereport from the existing host still returned `200`.
   - A second Docker report still returned `200`.
   - Final entitlements stayed at `max_agents.current=1` while `docker_hosts` increased to `2`.

## Outcome

- Free/community entitlements gated paid AI controls and multi-tenant billing surfaces consistently.
- Paid entitlements unlocked the same surfaces without leaving stale upgrade prompts behind.
- The Pro settings surface matched the active entitlement state on each fresh authenticated session.
- The live `max_agents` count tracked only installed v6 host agents.
- New host-agent enrollment was blocked at the cap, while existing host reports and Docker reports continued to succeed.
