# MSP Provider Tenant Management Production Follow-up Record

- Date: `2026-03-13`
- Gate: `msp-provider-tenant-management`
- Assertion: `RA13`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Dedicated MSP rehearsal account: `a_msp_owner_seed_20260313124930`
  - Provider owners:
    - `legacy+msp-owner-seed-20260313124930@pulserelay.pro`
    - `operator+msp-owner-seed-20260313124930@pulserelay.pro`
  - Fresh production proof tenant: `t-8ME7XXQM7X`
  - Evidence source: live production HTTPS surface, control-plane bearer session, tenant registry state, and tenant runtime API responses

## External Follow-up Exercise

1. Seeded a fresh live MSP rehearsal account on production with two distinct
   provider-owner identities so owner selection could be validated against a
   realistic multi-owner account.
2. Exercised fresh workspace provisioning through the authenticated operator
   session after deploying the owner-aware provisioning fix:
   - created fresh tenant canaries under
     `a_msp_owner_seed_20260313124930`
   - inspected the tenant org record on disk for the fresh canaries
3. Confirmed deterministic owner assignment on the successful production canary:
   - `/data/tenants/t-8ME7XXQM7X/orgs/t-8ME7XXQM7X/org.json` contained
     `ownerUserId=operator+msp-owner-seed-20260313124930@pulserelay.pro`
   - the owner did not drift back to the older legacy owner purely because
     that account member already existed
4. Confirmed the tenant runtime received the canonical identity it needs to
   preserve MSP-scoped management after handoff:
   - live container env included `PULSE_TENANT_ID=t-8ME7XXQM7X`
   - the real handoff exchange completed successfully with `200`
5. Continued the same session into provider-managed tenant surfaces:
   - `GET /api/admin/orgs/t-8ME7XXQM7X/billing-state` returned `200`
   - `GET /api/orgs/t-8ME7XXQM7X/members` returned `200`
   - members payload showed both seeded provider owners coherently

## Outcome

- This is successful `real-external-e2e` production evidence.
- MSP workspace provisioning now preserves the authenticated creator as the
  tenant owner instead of relying on unstable account-member ordering.
- The same fresh tenant also proves the MSP operator can enter the provisioned
  workspace, resolve billing state, and enumerate tenant membership without the
  earlier tenant-identity handoff drift.

## Conclusion

- `msp-provider-tenant-management` remains correctly `passed`.
- `RA13` is now backed by fresh production follow-up proof that the repaired
  MSP owner-selection and tenant-identity handoff behavior holds on a new live
  canary, not only on the earlier fixed rehearsal account.
