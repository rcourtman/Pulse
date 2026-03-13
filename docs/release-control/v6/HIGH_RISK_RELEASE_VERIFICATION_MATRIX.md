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

Companion drill:

- For cancellation/reactivation pricing continuity, checkout re-entry, and
  Stripe-driven revocation boundaries, run
  `docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`
  and attach the resulting record to the applicable gate evidence.

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
  `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 npm test -- tests/07-trial-signup-return.spec.ts --project=chromium`
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
- Latest exercised record:
  `docs/release-control/v6/records/hosted-signup-billing-replay-2026-03-12.md`
- Block release if:
  Any hosted checkout, org linkage, magic-link, billing-admin, or webhook
  replay path is unconfirmed or inconsistent.

## Gate: `cloud-hosted-tier-runtime-readiness`

- Why this is risky:
  Hosted signup alone is not enough. If the real hosted Pulse tier cannot be
  entered, authenticated, navigated, or administered after provisioning, users
  will pay for a product tier that exists in pricing and billing but not in
  dependable runtime behavior.
- Primary runtime surfaces:
  `internal/cloudcp/...`
  `internal/hosted/...`
  `internal/api/public_signup_handlers.go`
  `internal/api/hosted_org_admin_handlers.go`
  `frontend-modern/src/pages/HostedSignup.tsx`
  `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
  `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
- Automated proof:
  `go test ./internal/cloudcp/... -count=1`
  `go test ./internal/hosted/... -count=1`
  `go test ./internal/api -run 'TestHostedLifecycle|TestHostedOrgAdminHandlers|TestHostedSignupSuccess|TestHostedSignupValidationFailures|TestHostedSignupHostedModeGate|TestHostedSignupRateLimit|TestHostedSignupRateLimit_NoProvisioningSideEffects|TestHostedSignupCleanupOnRBACFailure|TestHostedSignupFailsClosedWithoutPublicURL|TestStripeWebhook_' -count=1`
  `cd frontend-modern && npx vitest run src/pages/__tests__/HostedSignup.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx`
- Manual scenario:
  1. Start from a real hosted Pulse signup or an existing hosted tenant.
  2. Confirm the user can authenticate into the hosted Pulse app and reach a
     working hosted runtime instead of a self-hosted setup or dead-end state.
  3. Confirm hosted billing/admin and organization billing surfaces render
     coherent plan, seat, and entitlement state for the hosted tenant.
  4. Confirm hosted-only admin actions and normal post-signup navigation work
     without self-hosted license prompts or broken hosted assumptions.
- Pass when:
  A real hosted Pulse customer can sign up or sign in, land in a working
  hosted runtime, and use the hosted billing/admin surfaces without self-hosted
  fallbacks or broken post-provisioning behavior.
- Block release if:
  Hosted Pulse can be sold or provisioned but not entered and used as a
  coherent hosted product tier afterward.

## Gate: `commercial-cancellation-reactivation`

- Why this is risky:
  Grandfathered recurring continuity, Stripe cancellation state, entitlement
  revocation, and public checkout re-entry span multiple repos and billing
  boundaries. This is exactly the kind of path that can look correct in unit
  tests while still charging the wrong price or granting the wrong access in a
  real customer journey.
- Primary runtime surfaces:
  `internal/api/payments_webhook_handlers.go`
  `pkg/licensing/...`
  `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
  `pulse-pro/license-server/v6_checkout.go`
  Stripe customer portal / recurring subscription state
- Automated proof:
  `python3 scripts/release_control/commercial_cancellation_reactivation_proof.py`
  Manual command detail remains documented in
  `docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`.
- Manual scenario:
  Execute `CCR-1` through `CCR-7` from
  `docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`
  against a staging-like billing environment and write a dated record under
  `docs/release-control/v6/records/`.
- Pass when:
  Active grandfathered subscribers keep their legacy recurring price while the
  subscription remains continuous, completed cancellation revokes paid access,
  and any later public re-entry lands on current public v6 pricing rather than
  reviving the legacy recurring rate.
- Block release if:
  The scenario is unexercised, a returning canceled customer can re-enter on a
  legacy recurring price, or cancellation/reactivation leaves pricing and
  entitlement state inconsistent across Stripe, Pulse runtime, and customer UI.

## Gate: `documentation-currentness-and-legacy-cleanup`

- Why this is risky:
  Stale release-control or upgrade guidance creates invisible operational
  drift. Agents and humans will follow whatever the docs say is current, even
  when the runtime has already moved on.
- Primary runtime surfaces:
  `docs/release-control/CONTROL_PLANE.md`
  `docs/release-control/control_plane.json`
  `docs/release-control/v6/SOURCE_OF_TRUTH.md`
  `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
  `docs/release-control/v6/README.md`
  `docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`
- Automated proof:
  `python3 scripts/release_control/documentation_currentness_test.py`
- Manual scenario:
  1. Review the active v6 guidance surface used by agents and release work.
  2. Confirm the docs describe the current active target, release phase, and
     canonical workflow rather than superseded guidance.
  3. Confirm any remaining legacy, audit, or historical docs are clearly
     framed as records or reference material instead of current instructions.
  4. Confirm any stale active doc is updated, archived, or removed rather than
     left to drift.
- Pass when:
  Active v6-facing guidance matches the current governed state of the repo, and
  historical docs no longer present themselves as current guidance.
- Block release if:
  Agents or humans can still follow stale v6 guidance, or legacy/historical
  docs remain mixed into the active v6 instruction surface.

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
- Latest exercised record:
  `docs/release-control/v6/records/paid-feature-entitlement-gating-2026-03-12.md`
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
  5. Confirm `V5_MAINTENANCE_SUPPORT_POLICY.md` is still the governing v5
     support policy and record the exact v6 GA date plus the exact v5
     end-of-support date that will ship with the stable or GA announcement.
  6. Confirm the `Release Dry Run` workflow produced an
     `rc-to-ga-rehearsal-summary` artifact and record the run URL in the
     release ticket or rehearsal record.
  7. Confirm the migration gate and other applicable high-risk gates are
     cleared for this same candidate before broad rollout.
- Pass when:
  Stable or GA promotion is a governed handoff from an exercised RC with live
  release-pipeline proof, explicit rollback instructions, and the published v5
  maintenance policy plus exact end-of-support date, with a linked rehearsal
  run URL and dry-run artifact.
- Current blocked record:
  `docs/release-control/v6/records/rc-to-ga-promotion-readiness-blocked-2026-03-13.md`
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
  `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 npm test -- tests/11-first-session.spec.ts --project=chromium`
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
- Latest exercised record:
  `docs/release-control/v6/records/upgrade-state-and-entitlement-preservation-2026-03-12.md`
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
  `go test ./internal/relay -run 'TestClient_E2E_MultiMobileClientRelay|TestClient_AbruptDisconnectCancelsInFlightHandlers|TestClient_AbruptDisconnectMultipleChannelCleanup|TestClient_DrainDuringInFlightData|TestClient_DrainWithMultipleInFlightChannels|TestClientRegister_SessionResumeRejectionClearsCachedSession|TestRunLoop_SessionResumeRejectionFallsBackToFreshRegister' -count=1`
  `go test ./internal/api -run 'TestRelayEndpointsRequireLicenseFeature|TestRelayOnboardingEndpointsRequireLicenseFeature|TestRelayLicenseGatingResponseFormat|TestOnboardingQRPayloadStructure|TestOnboardingValidateSuccessAndFailure|TestOnboardingDeepLinkFormat' -count=1`
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
- Latest exercised record:
  `docs/release-control/v6/records/relay-registration-reconnect-drain-2026-03-12.md`
- Block release if:
  The relay can strand the app/client in resume loops, dead sessions, or lost
  inflight work.

## Gate: `unified-agent-v5-upgrade-continuity`

- Why this is risky:
  The v5-to-v6 unified-agent crossover is where release-asset integrity,
  updater continuity, legacy compatibility routing, and user-visible agent
  inventory can drift apart. Repo-local tests cover most of the mechanics, but
  the real RC path still needs one exercised upgrade from an actual v5 install.
- Primary runtime surfaces:
  `GET /install.sh`
  `GET /install.ps1`
  `GET /api/agent/version`
  `internal/api/unified_agent.go`
  `internal/api/router_routes_registration.go`
  `internal/agentupdate/update.go`
  `internal/hostagent/agent.go`
  `frontend-modern/src/components/Settings/UnifiedAgents.tsx`
  `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
- Automated proof:
  `go test ./internal/api -run 'TestDownloadUnifiedInstallScript|TestDownloadUnifiedInstallScriptPS|TestProxyInstallScriptFromGitHub|TestContract_InstallScriptReleaseAssetURL|TestDownloadUnifiedAgent|TestHostAgentHandlers_LegacyV5ReportUpgradesToSingleCanonicalV6Agent|TestHostAgentEndpointsAcceptLegacyHostAgentReportScopeAlias|TestNormalizeRequestedScopesCanonicalizesLegacyHostAgentAliases|TestContract_APITokenScopeAliasNormalization' -count=1`
  `go test ./internal/agentupdate -run 'TestCheckAndUpdateToFirstHostReportCarriesPreviousVersionOnce|TestUpdateToFirstHostReportCarriesPreviousVersionOnce|TestPerformUpdatePersistsPreviousVersionForNextStart' -count=1`
  `go test ./internal/hostagent -run 'TestNew_CarriesUpdatedFromIntoFirstV6Report|TestAgentSendReport_SetsHeadersAndPostsJSON' -count=1`
- Manual scenario:
  1. Start from a real Pulse v5 install with an already-enrolled unified agent
     and non-empty agent inventory.
  2. Point that install at the candidate v6 RC build and trigger the real
     upgrade path through the release-served installer or updater assets, not a
     repo-local script.
  3. Confirm the fetched install script or update asset resolves to the
     matching v6 RC release asset rather than branch-tip `main` content.
  4. Confirm the upgraded agent reconnects as one canonical v6 unified agent
     identity and does not create a duplicate host or agent resource during the
     crossover.
  5. Confirm the pre-existing installed agent token still reaches the
     canonical `/api/agents/agent/*` v6 endpoints even if its persisted scopes
     originated as legacy `host-agent:*` aliases.
  6. Confirm the first canonical v6 report carries the prior v5 version in
     `updated_from` exactly once.
  7. Confirm a subsequent report clears `updated_from`, and the active-agent
     count shown in settings/billing surfaces still matches runtime
     enforcement after the upgrade.
- Pass when:
  A real v5-installed unified agent upgrades through the candidate v6 RC asset
  path, reconnects as one canonical v6 agent identity, preserves one-shot
  `updated_from` continuity, and leaves user-visible agent counts aligned with
  runtime enforcement.
- Latest exercised record:
  `docs/release-control/v6/records/unified-agent-v5-upgrade-continuity-2026-03-12.md`
- Block release if:
  The RC asset path serves the wrong installer logic, the upgrade creates
  duplicate or orphaned agent identity, `updated_from` continuity is missing or
  repeated, or user-visible agent counts drift from runtime enforcement.

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
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/__tests__/mobileRelayAuthApprovals.rehearsal.test.ts src/utils/__tests__/secureStorage.test.ts src/hooks/__tests__/useRelayLifecycle.test.ts src/hooks/__tests__/approvalActionPolicy.test.ts src/stores/__tests__/instanceStore.test.ts src/stores/__tests__/authStore.test.ts src/stores/__tests__/approvalStore.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/api/__tests__/client.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/hooks/__tests__/useRelay.test.ts src/hooks/__tests__/relayPushRefresh.test.ts src/notifications/__tests__/notificationRouting.test.ts src/stores/__tests__/mobileAccessState.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-enterprise && go test ./internal/aiautofix -run 'TestHandleListApprovals|TestHandleApproveAndExecuteInvestigationFix|TestHandleApprove' -count=1`
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
- Latest exercised record:
  `docs/release-control/v6/records/mobile-relay-auth-approvals-2026-03-13.md`
- Block release if:
  Mobile can keep stale access, lose approval state, or fail to recover from
  reconnect/auth transitions.

## Gate: `multi-tenant-runtime-isolation-and-coherence`

- Why this is risky:
  Multi-tenant support is not just an org settings feature. If tenant
  isolation, tenant-scoped runtime state, or cross-org sharing drifts, Pulse
  will expose the wrong data to the wrong tenant while still looking healthy in
  narrower UI-only checks.
- Primary runtime surfaces:
  `internal/api/org_handlers*.go`
  `internal/api/rbac_handlers*.go`
  `internal/api/resources_tenant_security_test.go`
  `internal/api/router_helpers_more_test.go`
  `internal/api/api_token_org_scope_integration_test.go`
  `internal/monitoring/...`
  `frontend-modern/src/components/Settings/Organization*.tsx`
  `frontend-modern/src/components/Settings/RolesPanel.tsx`
  `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`
  `tests/integration/tests/03-multi-tenant.spec.ts`
- Automated proof:
  `go test ./internal/api -run 'TestOrgHandlers|TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers|TestMultiTenantStateProvider|TestMultiTenantAPITokenRemainsScopedToIssuingOrg' -count=1`
  `go test ./internal/monitoring -run 'TestMultiTenantMonitor' -count=1`
  `go test ./tests/migration -run 'TestV5DataDir_MultiTenantMigration' -count=1`
  `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts`
- Manual scenario:
  1. Enable multi-tenant mode and create at least two organizations with
     different users and roles.
  2. Confirm each user only sees the orgs, resources, and runtime state they
     are explicitly allowed to see.
  3. Confirm role changes and tenant membership changes immediately affect UI
     and API scope.
  4. Confirm tenant-scoped runtime paths do not fall back to default or
     single-tenant state when a non-default org is requested.
  5. Confirm cross-org sharing grants only the intended access and does not
     widen tenant visibility.
- Pass when:
  Multi-tenant Pulse behaves as a coherent tenant-isolated product: org scope,
  RBAC, runtime state, sharing, and migration all stay within the intended
  tenant boundary.
- Block release if:
  A tenant can see or mutate data, runtime state, or shared resources outside
  the intended tenant boundary, or multi-tenant mode still behaves like a
  partially upgraded single-tenant system.

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
  `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/03-multi-tenant.spec.ts --project=chromium`
- Manual scenario:
  1. Add a new user.
  2. Confirm the user only sees the orgs they belong to.
  3. Change member role and confirm the UI and API scope update accordingly.
  4. Confirm self-escalation is blocked.
  5. Confirm cross-org sharing grants only the intended access level.
- Pass when:
  Org membership, RBAC role assignment, and cross-org access all enforce the
  least privilege intended by the UI.
- Latest exercised record:
  `docs/release-control/v6/records/organization-user-scope-and-rbac-2026-03-12.md`
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
  `go test ./internal/api -run 'TestNormalizeRequestedScopesCanonicalizesLegacyHostAgentAliases|TestHostAgentEndpointsAcceptLegacyHostAgentReportScopeAlias|TestContract_APITokenScopeAliasNormalization' -count=1`
  `cd frontend-modern && npx vitest run src/components/Settings/__tests__/APITokenManager.test.tsx src/utils/__tests__/apiClient.org.test.ts src/utils/__tests__/apiTokenPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
  `cd tests/integration && PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MULTI_TENANT_ENABLED=true npm test -- tests/13-api-token-scope.spec.ts --project=chromium`
- Manual scenario:
  1. Generate a token for a specific user.
  2. Confirm the token inherits only the intended user and org scope.
  3. Use the token against read, mutate, and exec paths that should be denied.
  4. Revoke the token and confirm the old token immediately stops working.
  5. Confirm legacy persisted `host-agent:*` token scopes still canonicalize to
     the intended v6 `agent:*` scope checks on installed-agent report and
     config flows.
  6. Confirm scoped agent/API-token flows fail with a clear message when the
     scope is insufficient.
- Pass when:
  Token create, use, read/write/exec scope enforcement, and revocation all
  behave exactly as intended.
- Latest exercised record:
  `docs/release-control/v6/records/api-token-scope-and-assignment-2026-03-12.md`
- Block release if:
  A token can outlive revocation, exceed assigned scope, or detach from the
  intended user/org identity.

## Gate Ownership Rule

Update these machine-visible gate states in `docs/release-control/v6/status.json`
as verification progresses:

1. `pending` means not yet confirmed end to end.
2. `blocked` means the gate is actively failing or cannot yet be exercised.
3. `passed` means both automated proof and the manual scenario are clear.
