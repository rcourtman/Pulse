# Cloud Hosted Tier Runtime Readiness Production Billing Follow-up Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Fresh runtime image: `pulse-runtime:hosted-billingfix-20260313T164400Z`
  - Fresh production canary tenant: `t-P62TP8K28Y`
  - Evidence source: live production HTTPS surface, control-plane bearer session, tenant runtime JSON exchange payload, tenant members API, tenant billing-state API, tenant entitlements API, and mounted billing-state inspection on `pulse-cloud`

## External Follow-up Exercise

1. Rebuilt and redeployed the hosted tenant runtime on production with the final
   same-lane fixes applied together:
   - hosted release-mode entitlement-key env override
   - control-plane handoff email normalization
   - hosted tenant billing-state fallback to the effective default-org lease
2. Created a fresh MSP workspace canary through the production control plane:
   - account: `a_ownerseed_20260313T145927`
   - tenant: `t-P62TP8K28Y`
3. Exercised the real hosted handoff exchange in JSON mode on the fresh tenant:
   - `POST /api/cloud/handoff/exchange?format=json`
   - returned `200`
   - payload normalized the runtime session identity to
     `operator-owner+20260313t145927@pulserelay.pro`
4. Continued the same session into the tenant-scoped runtime surfaces that had
   previously drifted apart:
   - `GET /api/orgs/t-P62TP8K28Y/members` returned `200`
   - `GET /api/admin/orgs/t-P62TP8K28Y/billing-state` returned `200`
   - `GET /api/license/entitlements` returned `200`
5. Confirmed those surfaces now agree on the live hosted commercial state:
   - members payload includes the normalized provider-owner identities
   - billing-state payload reports:
     - `subscription_state="active"`
     - `plan_version="msp_starter"`
     - `limits.max_monitored_systems=50`
   - entitlements payload reports the same active hosted MSP entitlement with
     `plan_version="msp_starter"` and `max_monitored_systems=50`
6. Confirmed the mounted root hosted billing record on `pulse-cloud` for the
   fresh canary carries the same active entitlement lease and refresh token
   that the runtime now projects correctly through both billing-state and
   entitlements.

## Outcome

- This is successful `real-external-e2e` production follow-up evidence.
- The fresh hosted runtime can now:
  - normalize handoff identity into the same casing used by seeded org
    membership
  - preserve that session identity through tenant entry
  - resolve the mounted hosted entitlement lease into active runtime state
  - return coherent hosted commercial state from both billing-state and
    entitlements on the same fresh tenant
- This closes the remaining same-lane runtime drift between:
  - handoff session identity
  - hosted entitlement evaluation
  - tenant billing-state admin reads

## Conclusion

- `cloud-hosted-tier-runtime-readiness` remains correctly `passed`.
- `RA11` is now backed by fresh production proof that a newly provisioned MSP
  tenant can complete the full hosted runtime path without divergence between
  membership, billing-state, and entitlement surfaces.
