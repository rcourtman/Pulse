# MSP Provider Tenant Management Production Record

- Date: `2026-03-13`
- Gate: `msp-provider-tenant-management`
- Assertion: `RA13`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane data dir: `/data`
  - Dedicated rehearsal MSP account: `a_msp_prod_20260313105601` (`Pulse MSP Rehearsal 20260313105601`)
  - Provider owner session: `owner+msp-prod-20260313105601@pulserelay.pro`
  - Invited provider member: `tech+msp-prod-20260313105601@pulserelay.pro` (`tech`)
  - Intended rehearsal workspaces:
    - `Client Alpha 20260313`
    - `Client Beta 20260313`

## Automated Proof Baseline

- `go test ./internal/cloudcp/account ./internal/cloudcp/registry -count=1`
- `go test ./internal/cloudcp/stripe -run 'TestMSPLifecycle_AccountToPortal' -count=1`
- `go test ./internal/cloudcp -run 'TestPublicCloudSignupCheckoutMetadataRejectsMSPPlanForPublicSignup' -count=1`
- `go test ./pkg/licensing -run 'TestMSPPlanAliasCanonicalizationContract' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/pages/__tests__/CloudPricing.test.tsx`
- Result: pass

## External Rehearsal

1. Created a dedicated MSP rehearsal account directly in the live control-plane registry on `pulse-cloud`:
   - `account_id=a_msp_prod_20260313105601`
   - `account.kind=msp`
   - canonical plan mapping `msp_starter`
   - real control-plane bearer session for `owner+msp-prod-20260313105601@pulserelay.pro`
2. Ran the governed external rehearsal helper against `https://cloud.pulserelay.pro`:
   - `python3 scripts/release_control/msp_provider_tenant_management_rehearsal.py --base-url https://cloud.pulserelay.pro --account-id a_msp_prod_20260313105601 --bearer-token <session> --timeout 180 --workspace-name 'Client Alpha 20260313' --workspace-name 'Client Beta 20260313' --member-email 'tech+msp-prod-20260313105601@pulserelay.pro' --member-role tech --public-signup-email 'public-msp-boundary-20260313105601@pulserelay.pro' --public-signup-org-name 'Public MSP Boundary 20260313105601' --report-out /tmp/msp-provider-tenant-management-production-2026-03-13.md`
3. Confirmed the MSP account surface itself works:
   - `GET /api/accounts/a_msp_prod_20260313105601/tenants` initially returned `200 []`
   - `POST /api/accounts/a_msp_prod_20260313105601/members` successfully invited `tech+msp-prod-20260313105601@pulserelay.pro`
   - `GET /api/accounts/a_msp_prod_20260313105601/members` returned both the owner and invited tech member
   - `GET /api/portal/dashboard?account_id=a_msp_prod_20260313105601` returned `account.kind="msp"` and `summary.total=0`
4. Confirmed the MSP/public boundary still fails closed on the same live control-plane instance:
   - unauthenticated `POST /api/public/signup` with `tier=power` returned `400 {"code":"tier_unavailable", ...}`
5. Attempted real external tenant provisioning twice under the MSP account:
   - `POST /api/accounts/a_msp_prod_20260313105601/tenants` with `{"display_name":"Client Alpha 20260313"}`
   - `POST /api/accounts/a_msp_prod_20260313105601/tenants` with `{"display_name":"Client Beta 20260313"}`
   - both calls eventually failed at the control-plane API boundary with `internal error`

## Runtime Failure Observed

- Control-plane logs on `pulse-cloud` showed the live MSP tenant create path entering real provisioning and starting tenant containers:
  - `t-7JJHNF3HZS` for `Client Alpha 20260313`
  - `t-XW52MAR90K` for `Client Beta 20260313`
- Both provisioning attempts then failed closed on the hosted runtime health check:
  - `tenant t-7JJHNF3HZS container failed health check`
  - `tenant t-XW52MAR90K container failed health check`
- Matching audit events were emitted as:
  - `audit_event=cp_tenant_create`
  - `outcome=failure`
  - `reason=provision_failed`
- The governed rehearsal report ended with:
  - `PASS msp-tenant-list`
  - `FAIL msp-create-workspace:Client Alpha 20260313`
  - `FAIL msp-create-workspace:Client Beta 20260313`
  - `PASS msp-invite-member:tech+msp-prod-20260313105601@pulserelay.pro`
  - `PASS msp-member-list`
  - `PASS msp-portal-dashboard`
  - `PASS public-cloud-boundary`

## Outcome

- The external MSP account model is partially real:
  - dedicated MSP accounts can exist on the live control plane
  - provider membership flows work
  - the portal/dashboard path recognizes the account as `msp`
  - the public individual-cloud signup path remains distinct
- The actual operator promise is still broken:
  - live MSP workspace provisioning does not complete successfully
  - the hosted tenant runtime fails health checks during MSP-driven tenant creation
  - one provider account therefore cannot yet manage multiple client tenants from one real external control surface

## Conclusion

- This is genuine `real-external-e2e` evidence, but it is failing evidence.
- `msp-provider-tenant-management` must remain `pending`.
- The next product work is not more governance text; it is fixing hosted tenant provisioning/health-check behavior on the live MSP tenant-create path.
