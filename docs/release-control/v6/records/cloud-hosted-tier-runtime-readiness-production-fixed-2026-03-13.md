# Cloud Hosted Tier Runtime Readiness Production Fixed Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane image: `pulse-control-plane:634fa3a66663-20260313T115951Z`
  - Final hosted runtime image: `pulse-runtime:b6800fe1f401-20260313T121150Z-pubkey`
  - Dedicated MSP rehearsal account: `a_msp_prod_fix_20260313111348`
  - Dedicated provider owner session: `owner+msp-fix-20260313111348@pulserelay.pro`
  - Final dedicated hosted rehearsal tenant: `t-2VNHNRHSGT`
  - Evidence source: live production HTTPS surface, admin-key protected control-plane APIs, tenant container inspection, and fresh tenant runtime responses

## Runtime Repair Chain

1. Confirmed the earlier hosted runtime failures were real and distinct:
   - older production rehearsal tenants predated org seeding and returned
     `invalid_org`
   - newer tenants on the earlier runtime image still returned
     `subscription_required`
   - tenant logs showed `hosted entitlement instance host is unavailable`
2. Deployed the current governed control-plane build to production:
   - `pulse-control-plane:634fa3a66663-20260313T115951Z`
3. Confirmed fresh hosted tenants now receive the required runtime artifacts at
   provision time:
   - seeded `orgs/<tenant_id>/org.json`
   - root `billing.json` with active hosted entitlement lease and integrity
4. Identified the remaining live blocker in the tenant runtime build itself:
   - the hosted runtime image did not have the entitlement/trial public key
     embedded, so it could not parse the lease token written by the control
     plane
5. Rebuilt the hosted runtime image from the current `pulse/v6` `HEAD` with
   the matching public key embedded:
   - `pulse-runtime:b6800fe1f401-20260313T121150Z-pubkey`
6. Updated `CP_PULSE_IMAGE` on `pulse-cloud` to that rebuilt runtime image and
   restarted only the control plane so new rehearsal tenants would use it

## External Rehearsal

1. Created a fresh hosted rehearsal tenant under the dedicated live MSP account:
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/tenants`
   - created `t-2VNHNRHSGT`
   - container image confirmed on `pulse-cloud` as
     `pulse-runtime:b6800fe1f401-20260313T121150Z-pubkey`
2. Verified the fresh tenant data dir on `pulse-cloud`:
   - `orgs/t-2VNHNRHSGT/org.json` existed with owner/member seeding
   - `billing.json` contained an active `msp_starter` hosted entitlement lease
     with `max_agents=50`
3. Generated a fresh production admin magic link for the dedicated tenant:
   - `POST https://cloud.pulserelay.pro/admin/magic-link`
   - `email=owner+msp-fix-20260313111348@pulserelay.pro`
   - `tenant_id=t-2VNHNRHSGT`
4. Exercised the real hosted handoff path:
   - control-plane redirect target was
     `https://t-2VNHNRHSGT.cloud.pulserelay.pro/auth/cloud-handoff?...`
   - tenant runtime completed handoff, set tenant cookies, and landed on `/`
     with `200`
5. Confirmed the runtime image now carries the required verification key:
   - tenant logs reported
     `license public key loaded`
   - the prior `hosted entitlement instance host is unavailable` warning was no
     longer the active blocker on the successful path
6. Continued the same session into hosted entitlement surfaces on the fresh
   tenant:
   - `GET /api/license/entitlements` on tenant org `t-2VNHNRHSGT` returned
     `200`
   - payload reported:
     - `subscription_state="active"`
     - `plan_version="msp_starter"`
     - `hosted_mode=true`
     - `valid=true`
     - `limits.max_agents=50`
   - `GET /api/license/entitlements` with `X-Pulse-Org-ID: default` also
     returned the same active hosted entitlement state

## Outcome

- This is successful `real-external-e2e` production evidence.
- A fresh externally provisioned hosted tenant can now:
  - receive the seeded org/runtime artifacts it needs
  - complete hosted magic-link handoff
  - land inside the hosted Pulse app
  - resolve active paid hosted entitlements coherently at both tenant and
    default lease surfaces
- This clears the earlier hosted runtime blockers around immutable ownership,
  tenant org seeding, hosted lease fallback, and missing runtime public-key
  embedding.

## Conclusion

- `cloud-hosted-tier-runtime-readiness` can be treated as `passed`.
- `RA11` is now backed by successful `real-external-e2e` hosted runtime proof.
- `hosted-signup-billing-replay` remains a separate gate and is still required
  for complete hosted commercial confidence.
