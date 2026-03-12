# Pulse v6 High-Risk Release Verification Matrix

Use this file for the parts of the release that I should not trust from unit
tests alone.

This is the human runbook for `status.json.release_gates`.
A gate is only `passed` when its automated proof still passes and the manual
scenario has been exercised in a staging-like environment with the expected
result.

## How To Use This Matrix

1. Run the automated proof first.
2. Run the manual scenario exactly on the runtime surface named below.
3. Record the environment, date, and result in the release ticket or inline in
   this file.
4. Update the matching `status.json.release_gates[*].status` entry to `passed`
   only after the full gate is clear.
5. Treat every failed or unconfirmed gate as a release blocker.

## Gate: `hosted-signup-billing-replay`

- Why this is risky:
  Hosted signup, magic-link access, org provisioning, checkout, and webhook
  replay are cross-system flows. They can look fine in isolated tests while
  still failing in the real handoff path.
- Primary runtime surfaces:
  `frontend-modern/src/pages/HostedSignup.tsx`
  `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
  `internal/api/public_signup_handlers.go`
  `internal/hosted/...`
  `internal/cloudcp/...`
  `internal/api/stripe_webhook_handlers*.go`
- Automated proof:
  `go test ./internal/api -run 'TestHostedLifecycle|TestHostedSignup' -count=1`
  `go test ./internal/api -run 'TestStripeWebhook_'`
  `go test ./internal/cloudcp/... -count=1`
  `go test ./internal/hosted/... -count=1`
  `cd frontend-modern && npx vitest run src/pages/__tests__/HostedSignup.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx`
- Manual scenario:
  1. Start a hosted signup from the self-hosted trial/upgrade path.
  2. Confirm a missing hosted public URL fails closed before any org or RBAC
     tenant is created.
  3. Confirm the user is sent to hosted checkout instead of receiving a local
     entitlement immediately.
  4. Confirm unresolved org linkage fails closed on webhook handling.
  5. Replay the same webhook after the linked org exists and confirm it
     succeeds.
  6. Confirm billing-admin state reflects the resulting org/subscription state.
- Pass when:
  Hosted signup fails closed before provisioning when required external URL
  config is missing, creates the correct org when enabled, webhook replay is
  fail-closed before linkage and succeeds after linkage, and the UI shows the
  resulting state coherently.
- Block release if:
  Any hosted checkout, org linkage, magic-link, billing-admin, or webhook
  replay path is unconfirmed or inconsistent.

## Gate: `paid-feature-entitlement-gating`

- Why this is risky:
  This is where free-vs-paid drift becomes customer-visible. UI claims, API
  enforcement, entitlements, and agent-allocation accounting all need to
  agree.
- Primary runtime surfaces:
  `GET /api/license/entitlements`
  `internal/api/agent_limit_enforcement.go`
  `internal/api/subscription_entitlements.go`
  `frontend-modern/src/pages/AIIntelligence.tsx`
  `frontend-modern/src/pages/Alerts.tsx`
  `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
  `frontend-modern/src/components/shared/AgentLimitWarningBanner.tsx`
  `internal/cloudcp/entitlements/service.go`
  `pkg/licensing/entitlements.go`
- Automated proof:
  `go test ./internal/api -run 'TestEntitlementHandler_|TestRequireLicenseFeature_HostedEntitlements|TestLicenseGatedEmptyResponse_HostedEntitlements' -count=1`
  `go test ./internal/api -run 'TestAgentCountNilMonitor|TestLegacyConnectionCountsFromReadState|TestLegacyConnectionCountsUsesSnapshotFallback|TestDeployReservedCount|TestHostAgentHandlers_HandleReport_EnforcesMaxAgentsForNewHostsOnly|TestHandleAddNode_NoLimitForConfigRegistration|TestHandleAutoRegister_NoLimitForConfigRegistration|TestDockerAgentHandlers_HandleReport_NoLimitForDockerReports|TestKubernetesAgentHandlers_HandleReport_NoLimitForK8sReports|TestTrueNASHandlers_HandleAdd_NoLimitForTrueNAS|TestBuildEntitlementPayloadWithUsage_CurrentValues' -count=1`
  `go test ./internal/license/... -count=1`
  `go test ./internal/cloudcp/... -count=1`
  `cd frontend-modern && npx vitest run src/pages/__tests__/AIIntelligence.test.tsx src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/shared/__tests__/AgentLimitWarningBanner.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Manual scenario:
  1. Use a free/community entitlement state and confirm paid features are gated.
  2. Use a Pro/Cloud entitlement state and confirm the same surfaces unlock.
  3. Confirm the upgrade path shown in the UI matches the runtime capability.
  4. Confirm alert analysis, AI autonomy, RBAC-only areas, and cloud-only areas
     do not leak access for free users.
  5. Confirm the active-agent count shown in settings and upgrade-warning
     surfaces matches the installed v6 agent count and excludes API-only or
     legacy connection surfaces that are not supposed to consume the cap.
  6. Confirm adding a new v6 agent at limit is blocked while existing agents
     continue to report and non-agent config/API registrations are not counted
     against `max_agents`.
- Pass when:
  Free users are blocked consistently, paid users are admitted consistently,
  active-agent counts and caps stay coherent across UI and runtime, and there
  is no UI/API disagreement.
- Block release if:
  Any feature can be used without entitlement, or any paid user is blocked on a
  correctly granted capability, or agent counts/caps disagree across
  enforcement and user-visible surfaces.

## Gate: `rc-to-ga-promotion-readiness`

- Why this is risky:
  Stable users must not become the first real validation cohort for v6. The
  RC-to-GA handoff is where migration confidence, release automation,
  rollback clarity, and the v5 support policy have to become explicit.
- Primary runtime surfaces:
  `.github/workflows/create-release.yml`
  `.github/workflows/publish-docker.yml`
  `.github/workflows/promote-floating-tags.yml`
  `docs/release-control/v6/PRE_RELEASE_CHECKLIST.md`
  `docs/release-control/v6/RELEASE_PROMOTION_POLICY.md`
  `docs/releases/RELEASE_NOTES_v6.md`
- Automated proof:
  `python3 scripts/release_control/release_promotion_policy_test.py`
- Manual scenario:
  1. Identify the exact RC tag and commit that are being considered for stable
     or GA promotion.
  2. Confirm the candidate commit has already shipped on `rc` through a real
     release-pipeline run, not only workflow lint or static YAML validation.
  3. Confirm the candidate satisfies the minimum 72-hour RC soak or that a
     hotfix exception and reason are recorded explicitly before promotion.
  4. Confirm the previous stable rollback target and exact reinstall or pin
     command are recorded in the release notes or release ticket.
  5. Confirm the v5 maintenance-only support policy and end-of-support window
     are written down and ready to ship with the stable or GA announcement.
  6. Confirm the migration gate and other applicable high-risk gates are
     cleared for this same candidate before broad rollout.
- Pass when:
  Stable or GA promotion is a governed handoff from an exercised RC with live
  release-pipeline proof, explicit rollback instructions, and a published v5
  maintenance policy.
- Block release if:
  Stable users would become the first real validation cohort, the rollback
  target is unclear, or the v5 maintenance-only policy is still undecided.

## Gate: `upgrade-state-and-entitlement-preservation`

- Why this is risky:
  Upgrade pain is trust-breaking and easy to miss when clean-room tests start
  from fresh installs. Paid continuity, onboarding continuity, and local state
  preservation all have to survive a real upgrade path.
- Primary runtime surfaces:
  `pkg/licensing/...`
  `internal/api/license_handlers*.go`
  `internal/api/public_signup_handlers.go`
  `frontend-modern/src/components/SetupWizard/...`
  `frontend-modern/src/components/Settings/...`
- Automated proof:
  `go test ./internal/api -run 'TestHostedLifecycle|TestEntitlementHandler_|TestRequireLicenseFeature_HostedEntitlements' -count=1`
  `go test ./pkg/licensing/... -count=1`
  `cd frontend-modern && npx vitest run src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx src/pages/__tests__/AIIntelligence.test.tsx`
  `npx playwright test tests/integration/tests/11-first-session.spec.ts`
- Manual scenario:
  1. Start from the previous supported Pulse build with non-trivial local state
     and an already-activated paid entitlement.
  2. Upgrade directly to the candidate v6 build without deleting local state.
  3. Confirm the app does not ask for the license again during normal startup.
  4. Confirm first-session and setup surfaces do not reset or regress into
     misleading upgrade prompts.
  5. Confirm paid-only surfaces remain correctly gated after upgrade.
- Pass when:
  Upgrade keeps the user's local state, entitlements, and first-session
  continuity intact without requiring manual repair or repeated activation.
- Block release if:
  Upgrade requires manual cleanup, repeated license entry, or leaves paid and
  non-paid surfaces in an inconsistent state.

## Gate: `relay-registration-reconnect-drain`

- Why this is risky:
  Relay failures are highly visible and often only appear under reconnect,
  eviction, or disconnect pressure.
- Primary runtime surfaces:
  `internal/relay/...`
  `internal/api/router_routes_auth_security.go`
  `internal/api/onboarding_handlers.go`
  `pulse-pro/relay-server/...`
  `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
  `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
  `pulse-mobile/src/relay/...`
- Automated proof:
  `go test ./internal/relay -count=1 -timeout=120s`
  `go test ./internal/api -run 'TestRelay|TestOnboarding' -count=1`
  `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/RelayOnboardingCard.test.tsx src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx src/components/Settings/__tests__/settingsReadOnlyPanels.test.tsx`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
- Manual scenario:
  1. Register a fresh relay client.
  2. Force reconnect after a normal disconnect.
  3. Force stale session resume or server-side eviction and confirm fresh
     registration recovery.
  4. Force abrupt disconnect while work is inflight and confirm drain/recovery
     behavior is sane.
- Pass when:
  Fresh register, reconnect, stale resume recovery, and disconnect/drain all
  behave predictably without hanging or spinning.
- Block release if:
  The relay can strand the app/client in resume loops, dead sessions, or lost
  inflight work.

## Gate: `mobile-relay-auth-approvals`

- Why this is risky:
  Mobile is a separate repo with separate state persistence, auth, and approval
  behavior. It is easy to miss regressions while the desktop/web app looks
  fine.
- Primary runtime surfaces:
  `pulse-mobile/src/stores/authStore.ts`
  `pulse-mobile/src/stores/instanceStore.ts`
  `pulse-mobile/src/stores/approvalStore.ts`
  `pulse-mobile/src/hooks/useRelay.ts`
  `pulse-mobile/src/api/client.ts`
- Automated proof:
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/utils/__tests__/secureStorage.test.ts src/hooks/__tests__/useRelayLifecycle.test.ts src/hooks/__tests__/approvalActionPolicy.test.ts src/stores/__tests__/instanceStore.test.ts src/stores/__tests__/authStore.test.ts src/stores/__tests__/approvalStore.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/api/__tests__/client.test.ts`
- Manual scenario:
  1. Pair the mobile app to a real instance through the relay onboarding path.
  2. Kill and relaunch the app to confirm secure persistence and reconnect.
  3. Confirm approval requests appear, are scoped correctly, and resolve
     cleanly.
  4. Confirm logout, token expiry, or revoked access forces the app back to a
     safe state.
- Pass when:
  Pairing, persistence, reconnect, approvals, and sign-out/revocation behavior
  all work without stale access.
- Block release if:
  Mobile can keep stale access, lose approval state, or fail to recover from
  reconnect/auth transitions.

## Gate: `organization-user-scope-and-rbac`

- Why this is risky:
  Multi-tenant scope mistakes are trust-critical. Wrong member roles or org
  boundaries mean real data exposure.
- Primary runtime surfaces:
  `internal/api/org_handlers*.go`
  `internal/api/rbac_handlers*.go`
  `frontend-modern/src/components/Settings/Organization*.tsx`
  `frontend-modern/src/components/Settings/RolesPanel.tsx`
  `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`
- Automated proof:
  `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers' -count=1`
  `go test ./internal/monitoring -run 'TestMultiTenantMonitor'`
  `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Manual scenario:
  1. Add a new user.
  2. Confirm the user only sees the orgs they belong to.
  3. Change member role and confirm the UI and API scope update accordingly.
  4. Confirm self-escalation is blocked.
  5. Confirm cross-org sharing grants only the intended access level.
- Pass when:
  Org membership, RBAC role assignment, and cross-org access all enforce the
  least privilege intended by the UI.
- Block release if:
  A user can see or mutate data outside assigned org or role scope.

## Gate: `api-token-scope-and-assignment`

- Why this is risky:
  API tokens are long-lived authority. If token identity or scope binding is
  wrong, automated access will bypass user intent.
- Primary runtime surfaces:
  `internal/api/router.go`
  `internal/api/router_routes_auth_security.go`
  `internal/api/security_tokens.go`
  `internal/api/system_settings_telemetry_test.go`
  `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
  `frontend-modern/src/components/Settings/APITokenManager.tsx`
  `frontend-modern/src/utils/apiTokenPresentation.ts`
  `frontend-modern/src/utils/url.ts`
- Automated proof:
  `go test ./internal/api -run 'Test(APIToken|SecurityTokens|SystemSettings|MultiTenant)' -count=1`
  `cd frontend-modern && npx vitest run src/components/Settings/__tests__/APITokenManager.test.tsx src/utils/__tests__/apiClient.org.test.ts src/utils/__tests__/apiTokenPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Manual scenario:
  1. Generate a token for a specific user.
  2. Confirm the token inherits only the intended user and org scope.
  3. Use the token against read, mutate, and exec paths that should be denied.
  4. Revoke the token and confirm the old token immediately stops working.
  5. Confirm scoped agent/API-token flows fail with a clear message when the
     scope is insufficient.
- Pass when:
  Token create, use, read/write/exec scope enforcement, and revocation all
  behave exactly as intended.
- Block release if:
  A token can outlive revocation, exceed assigned scope, or detach from the
  intended user/org identity.

## Gate Ownership Rule

Update these machine-visible gate states in `docs/release-control/v6/status.json`
as verification progresses:

1. `pending` means not yet confirmed end to end.
2. `blocked` means the gate is actively failing or cannot yet be exercised.
3. `passed` means both automated proof and the manual scenario are clear.
