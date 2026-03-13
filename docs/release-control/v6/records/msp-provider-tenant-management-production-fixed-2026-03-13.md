# MSP Provider Tenant Management Production Fixed Record

- Date: `2026-03-13`
- Gate: `msp-provider-tenant-management`
- Assertion: `RA13`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane data dir: `/data`
  - Patched tenant image: `pulse:hosted-entrypoint-fix-20260313T1118Z`
  - Dedicated rehearsal MSP account: `a_msp_prod_fix_20260313111348`
  - Provider owner session: `owner+msp-fix-20260313111348@pulserelay.pro`
  - Invited provider member: `tech+msp-fix-20260313111348@pulserelay.pro` (`tech`)
  - Rehearsal workspaces:
    - `Client Alpha Fix 20260313111348`
    - `Client Beta Fix 20260313111348`

## Runtime Fix Applied Before Rehearsal

1. Reproduced the hosted tenant boot failure against a real tenant data dir on
   `pulse-cloud`:
   - plain `/etc/pulse` bind mount started healthy
   - the new immutable file mounts (`billing.json`, `handoff.key`,
     `.cloud_handoff_key`) made the same image fail during startup
2. The failure was caused by `docker-entrypoint.sh` still trying to `chown`
   those read-only mounted files during boot.
3. Built a patched tenant image on `pulse-cloud` from the same runtime line:
   - base: `ghcr.io/rcourtman/pulse:cloud-beta`
   - patch: updated `/docker-entrypoint.sh`
   - result tag: `pulse:hosted-entrypoint-fix-20260313T1118Z`
4. Updated `CP_PULSE_IMAGE` in `/opt/pulse-cloud/.env` to the patched image and
   restarted only the control plane.
5. Verified the patched image boots healthy under the immutable mount layout
   before retrying MSP workspace creation.

## Automated Proof Baseline

- `go test ./internal/cloudcp/account ./internal/cloudcp/registry -count=1`
- `go test ./internal/cloudcp/stripe -run 'TestMSPLifecycle_AccountToPortal' -count=1`
- `go test ./internal/cloudcp -run 'TestPublicCloudSignupCheckoutMetadataRejectsMSPPlanForPublicSignup' -count=1`
- `go test ./pkg/licensing -run 'TestMSPPlanAliasCanonicalizationContract' -count=1`
- `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx src/pages/__tests__/CloudPricing.test.tsx`
- Result: pass

## External Rehearsal

1. Seeded a fresh MSP provider account directly in the live control-plane
   registry using the governed rehearsal helper path:
   - `account_id=a_msp_prod_fix_20260313111348`
   - canonical plan mapping `msp_starter`
   - real control-plane bearer session for
     `owner+msp-fix-20260313111348@pulserelay.pro`
2. Ran the governed external rehearsal helper against production:
   - `python3 scripts/release_control/msp_provider_tenant_management_rehearsal.py --base-url https://cloud.pulserelay.pro --account-id a_msp_prod_fix_20260313111348 --bearer-token <session> --timeout 180 --workspace-name 'Client Alpha Fix 20260313111348' --workspace-name 'Client Beta Fix 20260313111348' --member-email 'tech+msp-fix-20260313111348@pulserelay.pro' --member-role tech --public-signup-email 'public-msp-boundary-fix-20260313111348@pulserelay.pro' --public-signup-org-name 'Public MSP Boundary Fix 20260313111348' --public-signup-tier power --report-out /tmp/msp-provider-tenant-management-production-fix-20260313111348.md`
3. Confirmed the full operator workflow now works on the live control plane:
   - `GET /api/accounts/a_msp_prod_fix_20260313111348/tenants` initially returned `200 []`
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/tenants` created
     `t-P9ES7BWFBT` for `Client Alpha Fix 20260313111348`
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/tenants` created
     `t-ZXTM30QX4J` for `Client Beta Fix 20260313111348`
   - both workspaces reported `plan_version='msp_starter'`
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/members` successfully invited
     `tech+msp-fix-20260313111348@pulserelay.pro`
   - `GET /api/accounts/a_msp_prod_fix_20260313111348/members` returned both
     the owner and invited tech member
   - `GET /api/portal/dashboard?account_id=a_msp_prod_fix_20260313111348`
     returned `account.kind="msp"` and `summary.total=2`
   - `GET /api/portal/workspaces/<tenant_id>?account_id=a_msp_prod_fix_20260313111348`
     returned coherent workspace detail for both created tenants
4. Confirmed the MSP/public boundary still fails closed on the same live
   control-plane instance:
   - unauthenticated `POST /api/public/signup` with `tier=power` returned
     `400 {"code":"tier_unavailable", ...}`

## Outcome

- The live MSP operator workflow now works as a real product mode:
  - one provider account can create multiple client workspaces
  - provider membership flow works
  - portal and workspace detail surfaces stay coherent
  - plan handling stays canonical per workspace
  - public individual-cloud signup remains distinct from MSP-only flows

## Conclusion

- This is successful `real-external-e2e` evidence.
- `msp-provider-tenant-management` can be treated as `passed`.
- `RA13` is now backed by real external runtime proof, not pricing-only or
  local rehearsal evidence.
