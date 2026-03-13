# Cloud Hosted Tier Runtime Readiness Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - Live localhost hosted-mode Pulse instance: `http://127.0.0.1:17771`
  - Persisted data dir: `/Volumes/Development/pulse/repos/pulse/tmp/manual-hosted-runtime-20260313/data`
  - Platform admin: `admin`
  - Hosted tenant created during rehearsal: `fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752`
  - Hosted tenant email: `hosted-rc-20260313@example.com`
  - Revalidation tenant after gate reopen: `fc6c9ffa-f100-46a2-b5e6-349dba526469`
  - Revalidation tenant email: `hosted-rc-rerun-20260313-0942@example.com`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestHostedLifecycle|TestHostedOrgAdminHandlers|TestHostedSignupSuccess|TestHostedSignupValidationFailures|TestHostedSignupHostedModeGate|TestHostedSignupRateLimit|TestHostedSignupRateLimit_NoProvisioningSideEffects|TestHostedSignupCleanupOnRBACFailure|TestHostedSignupFailsClosedWithoutPublicURL|TestStripeWebhook_' -count=1`
- `go test ./internal/cloudcp/... ./internal/hosted/... -count=1`
- `cd frontend-modern && npx vitest run src/pages/__tests__/HostedSignup.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx`
- Result: pass

## Manual Exercise

1. Started a clean localhost Pulse instance on `http://127.0.0.1:17771`, applied Quick Security Setup, confirmed auth and API-token state persisted into `tmp/manual-hosted-runtime-20260313/data/.env` and `api_tokens.json`, then restarted the same instance in hosted mode against that exact data directory.
2. Confirmed the hosted relaunch required auth on privileged surfaces:
   - `GET /api/security/status` returned `requiresAuth=true`, `hasAuthentication=true`, and `apiTokenConfigured=true`
   - anonymous `GET /api/hosted/organizations` returned `401 Authentication required`
   - anonymous `GET /api/admin/orgs/fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752/billing-state` returned `401 Authentication required`
3. Exercised real hosted signup on the live hosted-mode HTTP surface:
   - `POST /api/public/signup` with `hosted-rc-20260313@example.com` and `Hosted RC 20260313`
   - response was `201 Created`
   - returned `org_id=fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752`
   - returned `message="Check your email for a magic link to finish signing in."`
4. Confirmed the public hosted post-signup auth surface remained usable:
   - `POST /api/public/magic-link/request` for `hosted-rc-20260313@example.com` returned `200`
   - payload was `{"success":true,"message":"If that email is registered, you'll receive a magic link shortly."}`
5. Confirmed the platform-admin hosted control surface could see the provisioned tenant on the same live hosted instance:
   - authenticated `GET /api/hosted/organizations` as `admin`
   - returned `200`
   - list included both `default` and `fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752`
   - new tenant summary showed `display_name="Hosted RC 20260313"` and `owner_user_id="hosted-rc-20260313@example.com"`
6. Confirmed hosted billing/admin state for the new tenant was coherent:
   - authenticated `GET /api/admin/orgs/fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752/billing-state`
   - returned `200`
   - `subscription_state=trial`
   - `plan_version=cloud_trial`
   - hosted trial capabilities were populated
7. Confirmed tenant-scoped entitlements land in hosted runtime state instead of a self-hosted fallback:
   - authenticated `GET /api/license/entitlements` with `X-Pulse-Org-ID` and `X-Org-ID` set to `fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752`
   - returned `200`
   - `hosted_mode=true`
   - `valid=true`
   - `subscription_state=trial`
   - `plan_version=cloud_trial`
   - `tier=pro`
   - `upgrade_reasons=[]`

## Revalidation After Gate Reopen

1. Relaunched the real hosted-mode Pulse runtime on `http://127.0.0.1:17771` against the same persisted data directory and confirmed the instance still loaded prior auth and token state.
2. Rechecked the auth boundary on the live runtime:
   - `GET /api/security/status` still returned `requiresAuth=true`, `hasAuthentication=true`, and `apiTokenConfigured=true`
   - anonymous `GET /api/hosted/organizations` still returned `401 Authentication required`
   - anonymous `GET /api/admin/orgs/fa0b5ad9-0bcf-47ba-8104-e6d71f0d3752/billing-state` still returned `401 Authentication required`
3. Exercised a fresh hosted signup on the same live hosted runtime:
   - `POST /api/public/signup` with `hosted-rc-rerun-20260313-0942@example.com` and `Hosted RC Rerun 20260313 0942`
   - response was `201 Created`
   - returned `org_id=fc6c9ffa-f100-46a2-b5e6-349dba526469`
   - returned `message="Check your email for a magic link to finish signing in."`
4. Confirmed the public post-signup auth path still worked:
   - `POST /api/public/magic-link/request` for `hosted-rc-rerun-20260313-0942@example.com` returned `200`
   - payload remained `{"success":true,"message":"If that email is registered, you'll receive a magic link shortly."}`
5. Confirmed the platform-admin hosted control surface saw the newly provisioned tenant:
   - authenticated `GET /api/hosted/organizations` as `admin`
   - returned `200`
   - list included `default`, the original rehearsal tenant, and `fc6c9ffa-f100-46a2-b5e6-349dba526469`
   - new tenant summary showed `display_name="Hosted RC Rerun 20260313 0942"` and `owner_user_id="hosted-rc-rerun-20260313-0942@example.com"`
6. Confirmed hosted billing/admin state for the rerun tenant was still coherent:
   - authenticated `GET /api/admin/orgs/fc6c9ffa-f100-46a2-b5e6-349dba526469/billing-state`
   - returned `200`
   - `subscription_state=trial`
   - `plan_version=cloud_trial`
7. Confirmed tenant-scoped entitlements still landed in hosted runtime state:
   - authenticated `GET /api/license/entitlements` with `X-Pulse-Org-ID` and `X-Org-ID` set to `fc6c9ffa-f100-46a2-b5e6-349dba526469`
   - returned `200`
   - `hosted_mode=true`
   - `valid=true`
   - `subscription_state=trial`
   - `plan_version=cloud_trial`
   - `tier=pro`
   - `upgrade_reasons=[]`
8. Re-ran the governed automated proof bundle after the manual revalidation:
   - `go test ./internal/api -run 'TestHostedLifecycle|TestHostedOrgAdminHandlers|TestHostedSignupSuccess|TestHostedSignupValidationFailures|TestHostedSignupHostedModeGate|TestHostedSignupRateLimit|TestHostedSignupRateLimit_NoProvisioningSideEffects|TestHostedSignupCleanupOnRBACFailure|TestHostedSignupFailsClosedWithoutPublicURL|TestStripeWebhook_' -count=1`
   - `go test ./internal/cloudcp/... ./internal/hosted/... -count=1`
   - `cd frontend-modern && npx vitest run src/pages/__tests__/HostedSignup.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx`
   - Result: pass

## Outcome

- Hosted Pulse can be entered as a real tier on a live hosted-mode runtime, not just provisioned in signup and billing tests.
- Public hosted signup and magic-link request stay functional on the same instance that serves hosted runtime/admin surfaces.
- Hosted billing/admin and tenant-scoped entitlements reflect coherent hosted trial state after provisioning.
- The post-provisioning tenant path lands in hosted entitlements (`hosted_mode=true`, valid trial state) instead of falling back to a self-hosted expired/free posture.
- Privileged hosted admin surfaces remain protected while still functioning correctly for the platform admin.
- Re-exercising the gate after it was reopened produced the same result on the persisted hosted runtime, so the pending status was no longer justified.

## Notes

- This rehearsal intentionally used the real `pulse` binary on a live localhost HTTP surface rather than handler-only tests.
- The initial auth seed was applied before the hosted relaunch so the hosted runtime proof covered persisted auth and runtime continuity, not a one-shot in-memory test harness.
