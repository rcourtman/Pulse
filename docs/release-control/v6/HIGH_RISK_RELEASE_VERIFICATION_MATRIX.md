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
  enforcement, and entitlements all need to agree.
- Primary runtime surfaces:
  `GET /api/license/entitlements`
  `frontend-modern/src/pages/AIIntelligence.tsx`
  `frontend-modern/src/pages/Alerts.tsx`
  `frontend-modern/src/components/shared/AgentLimitWarningBanner.tsx`
  `internal/cloudcp/entitlements/service.go`
  `pkg/licensing/entitlements.go`
- Automated proof:
  `go test ./internal/api -run 'TestEntitlementHandler_|TestRequireLicenseFeature_HostedEntitlements|TestLicenseGatedEmptyResponse_HostedEntitlements' -count=1`
  `go test ./internal/license/... -count=1`
  `go test ./internal/cloudcp/... -count=1`
  `cd frontend-modern && npx vitest run src/pages/__tests__/AIIntelligence.test.tsx src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx src/components/shared/__tests__/AgentLimitWarningBanner.test.tsx src/utils/__tests__/licensePresentation.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Manual scenario:
  1. Use a free/community entitlement state and confirm paid features are gated.
  2. Use a Pro/Cloud entitlement state and confirm the same surfaces unlock.
  3. Confirm the upgrade path shown in the UI matches the runtime capability.
  4. Confirm alert analysis, AI autonomy, RBAC-only areas, and cloud-only areas
     do not leak access for free users.
- Pass when:
  Free users are blocked consistently, paid users are admitted consistently, and
  there is no UI/API disagreement.
- Block release if:
  Any feature can be used without entitlement, or any paid user is blocked on a
  correctly granted capability.

## Gate: `relay-registration-reconnect-drain`

- Why this is risky:
  Relay failures are highly visible and often only appear under reconnect,
  eviction, or disconnect pressure.
- Primary runtime surfaces:
  `internal/relay/...`
  `pulse-pro/relay-server/...`
  `pulse-mobile/src/relay/...`
- Automated proof:
  `go test ./internal/relay -count=1 -timeout=120s`
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
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/utils/__tests__/secureStorage.test.ts src/stores/__tests__/instanceStore.test.ts src/stores/__tests__/authStore.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
  `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/stores/__tests__/approvalStore.test.ts`
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
  `go test ./internal/api -run 'TestMultiTenant|TestResourceHandlers_NonDefaultOrg|TestSetMultiTenantMonitor_WiresHandlers'`
  `go test ./internal/monitoring -run 'TestMultiTenantMonitor'`
  `cd frontend-modern && npx vitest run src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
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
  `internal/api/system_settings_telemetry_test.go`
  `frontend-modern/src/components/Settings/SecurityPanel*.tsx`
  `frontend-modern/src/utils/apiTokenPresentation.ts`
  `frontend-modern/src/utils/url.ts`
- Automated proof:
  `go test ./internal/api -run 'TestAPIToken|TestSystemSettings|TestMultiTenant'`
  `cd frontend-modern && npx vitest run src/utils/__tests__/apiClient.org.test.ts src/utils/__tests__/apiTokenPresentation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts`
- Manual scenario:
  1. Generate a token for a specific user.
  2. Confirm the token inherits only the intended user and org scope.
  3. Use the token against read, mutate, and exec paths that should be denied.
  4. Revoke the token and confirm the old token immediately stops working.
  5. Confirm scoped agent/API-token flows fail with a clear message when the
     scope is insufficient.
- Pass when:
  Token create, use, scope enforcement, and revocation all behave exactly as
  intended.
- Block release if:
  A token can outlive revocation, exceed assigned scope, or detach from the
  intended user/org identity.

## Gate Ownership Rule

Update these machine-visible gate states in `docs/release-control/v6/status.json`
as verification progresses:

1. `pending` means not yet confirmed end to end.
2. `blocked` means the gate is actively failing or cannot yet be exercised.
3. `passed` means both automated proof and the manual scenario are clear.
