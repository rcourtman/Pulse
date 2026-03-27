# MSP Provider Tenant Management Record

- Date: `2026-03-13`
- Gate: `msp-provider-tenant-management`
- Assertion: `RA13`
- Environment:
  - Live localhost Pulse control plane: `http://127.0.0.1:18443`
  - Control-plane data dir: `/Volumes/Development/pulse/repos/pulse/tmp/manual-msp-gate-20260313`
  - Seeded MSP account: `a_mspgate20260313` (`Acme MSP Rehearsal`)
  - Provider owner session: `owner@acmemsp.test`
  - Canonical MSP plan: `msp_starter`
  - Revalidation workspace: `t-1WDFA6HW01` (`Client Three`)
  - Revalidation member: `readonly@acmemsp.test` (`read_only`)

## Automated Proof Baseline

- `go test ./internal/cloudcp/account ./internal/cloudcp/registry -count=1`
- `go test ./internal/cloudcp/stripe -run 'TestMSPLifecycle_AccountToPortal' -count=1`
- `go test ./internal/cloudcp -run 'TestPublicCloudSignupCheckoutMetadataRejectsMSPPlanForPublicSignup' -count=1`
- `go test ./pkg/licensing -run 'TestMSPPlanAliasCanonicalizationContract' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/pages/__tests__/CloudPricing.test.tsx`
- Result: pass

## Manual Exercise

1. Started a live localhost `pulse-control-plane` instance in development mode with:
   - `CP_ALLOW_DOCKERLESS_PROVISIONING=true`
   - `CP_REQUIRE_EMAIL_PROVIDER=false`
   - a valid `CP_TRIAL_ACTIVATION_PRIVATE_KEY`
   - `CP_ADMIN_KEY` and `CP_BASE_URL=http://127.0.0.1:18443`
2. Seeded the local control-plane registry with one MSP account, one owner user, one owner membership, one canonical Stripe account mapping, and a real bearer session token:
   - `account_id=a_mspgate20260313`
   - `account.kind=msp`
   - `plan_version=msp_starter`
   - `subscription_state=active`
3. Confirmed the provider account started with no client workspaces:
   - authenticated `GET /api/accounts/a_mspgate20260313/tenants`
   - returned `200 []`
4. Provisioned two client workspaces through the real account-scoped API:
   - authenticated `POST /api/accounts/a_mspgate20260313/tenants` with `{"display_name":"Client One"}`
   - authenticated `POST /api/accounts/a_mspgate20260313/tenants` with `{"display_name":"Client Two"}`
   - both returned `201`
   - both returned `state=active`
   - both returned `plan_version=msp_starter`
5. Confirmed the provider can view multiple client tenants coherently from one account surface:
   - authenticated `GET /api/accounts/a_mspgate20260313/tenants`
   - returned both `Client One` and `Client Two`
   - both tenants remained attached to `account_id=a_mspgate20260313`
6. Confirmed provider member management works on the same account:
   - authenticated `POST /api/accounts/a_mspgate20260313/members` with `{"email":"tech@acmemsp.test","role":"tech"}`
   - authenticated `GET /api/accounts/a_mspgate20260313/members`
   - member list showed:
     - `owner@acmemsp.test` as `owner`
     - `tech@acmemsp.test` as `tech`
7. Confirmed the provider portal reflects the same multi-tenant account state:
   - authenticated `GET /api/portal/dashboard?account_id=a_mspgate20260313`
   - returned `account.kind="msp"`
   - returned both workspaces in the dashboard summary
   - `summary.total=2`
   - `summary.active=2`
8. Confirmed workspace detail stays account-scoped and coherent:
   - authenticated `GET /api/portal/workspaces/t-0T18WWGENX?account_id=a_mspgate20260313`
   - returned the expected `Client One` workspace under the MSP account with `plan_version=msp_starter`
9. Confirmed the public individual cloud path remained distinct from MSP provisioning semantics on the same live control-plane instance:
   - unauthenticated `GET /cloud/signup` rendered the public “Start Pulse Cloud” page
   - unauthenticated `POST /api/public/signup` did not create or route into MSP provisioning; it failed closed with `400 {"code":"tier_unavailable","message":"The selected plan tier is not currently available"}`

## Revalidation After Evidence-Tier Tightening

1. Reused the persisted localhost control-plane rehearsal on `http://127.0.0.1:18443` with the same MSP account, provider owner identity, and stateless bearer session contract.
2. Reconfirmed the provider account still exposed coherent account-scoped state before mutation:
   - authenticated `GET /api/accounts/a_mspgate20260313/tenants` returned the existing `Client One` and `Client Two` workspaces
   - authenticated `GET /api/accounts/a_mspgate20260313/members` returned `owner@acmemsp.test` as `owner` and `tech@acmemsp.test` as `tech`
   - authenticated `GET /api/portal/dashboard?account_id=a_mspgate20260313` returned `account.kind="msp"` with `summary.total=2` and `summary.active=2`
3. Exercised fresh provider mutation on the same live account surface:
   - authenticated `POST /api/accounts/a_mspgate20260313/tenants` with `{"display_name":"Client Three"}`
   - returned `201`
   - returned `id=t-1WDFA6HW01`
   - returned `state=active`
   - returned `plan_version=msp_starter`
4. Confirmed the new workspace stayed attached to the MSP account instead of drifting into individual-cloud state:
   - authenticated `GET /api/accounts/a_mspgate20260313/tenants` returned `Client One`, `Client Two`, and `Client Three`
   - all three workspaces remained attached to `account_id=a_mspgate20260313`
5. Exercised fresh provider member management on the same account:
   - authenticated `POST /api/accounts/a_mspgate20260313/members` with `{"email":"readonly@acmemsp.test","role":"read_only"}`
   - returned `201`
   - authenticated `GET /api/accounts/a_mspgate20260313/members` returned `owner@acmemsp.test`, `tech@acmemsp.test`, and `readonly@acmemsp.test`
6. Confirmed the provider portal stayed coherent after the new workspace was added:
   - authenticated `GET /api/portal/dashboard?account_id=a_mspgate20260313` returned `summary.total=3` and `summary.active=3`
   - authenticated `GET /api/portal/workspaces/t-1WDFA6HW01?account_id=a_mspgate20260313` returned the expected `Client Three` workspace under the MSP account with `plan_version=msp_starter`
7. Reconfirmed the public individual-cloud path still failed closed instead of collapsing into MSP provisioning:
   - unauthenticated `GET /cloud/signup` still rendered the public cloud signup page
   - unauthenticated `POST /api/public/signup` with `{"email":"public-msp-boundary-20260313@example.com","org_name":"Public MSP Boundary 20260313","tier":"power"}` returned `400 {"code":"tier_unavailable","message":"The selected plan tier is not currently available"}`
8. Re-ran the governed automated proof bundle after the manual revalidation:
   - `go test ./internal/cloudcp/account ./internal/cloudcp/registry -count=1`
   - `go test ./internal/cloudcp/stripe -run 'TestMSPLifecycle_AccountToPortal' -count=1`
   - `go test ./internal/cloudcp -run 'TestPublicCloudSignupCheckoutMetadataRejectsMSPPlanForPublicSignup' -count=1`
   - `go test ./pkg/licensing -run 'TestMSPPlanAliasCanonicalizationContract' -count=1`
   - `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/pages/__tests__/CloudPricing.test.tsx`
   - Result: pass

## Outcome

- One provider account can create, view, and manage multiple client tenants from one live control surface.
- MSP workspace provisioning stays on the canonical MSP plan (`msp_starter`) and does not drift into individual-cloud semantics.
- Provider membership and dashboard visibility remain coherent across the same account and the same set of workspaces.
- The public individual cloud signup surface stays separate from MSP operator provisioning instead of silently collapsing the two modes together.
- The rerun remains a localhost control-plane rehearsal, so under the current evidence-tier policy it strengthens the record but does not honestly clear the gate until a real external E2E exercise exists.

## Notes

- For this localhost rehearsal, the MSP account was seeded directly in the control-plane registry because account creation is normally Stripe-driven and not exposed as a public create-account endpoint.
- The rehearsal intentionally used the live `pulse-control-plane` HTTP surface plus a real bearer session token, not only `httptest` handlers.
