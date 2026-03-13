# Cloud Hosted Tier Runtime Readiness Production Follow-up Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane image line: `pulse-control-plane:*owner-seed*` then `pulse-control-plane:*tenant-id*`
  - Dedicated MSP rehearsal account: `a_msp_owner_seed_20260313124930`
  - First fresh canary after owner seeding: `t-XX72QXZD8A`
  - Successful handoff canary after tenant-ID injection: `t-8ME7XXQM7X`
  - Evidence source: live production HTTPS surface, control-plane bearer session, tenant container inspection, and tenant runtime API responses

## External Follow-up Exercise

1. Seeded a dedicated live MSP rehearsal account with two provider owners on the
   production control plane:
   - legacy owner `legacy+msp-owner-seed-20260313124930@pulserelay.pro`
   - active operator session `operator+msp-owner-seed-20260313124930@pulserelay.pro`
2. Deployed the owner-aware hosted provisioning fix to production and created a
   fresh canary workspace:
   - tenant id `t-XX72QXZD8A`
   - inspected `/data/tenants/t-XX72QXZD8A/orgs/t-XX72QXZD8A/org.json`
   - confirmed `ownerUserId=operator+msp-owner-seed-20260313124930@pulserelay.pro`
3. Confirmed that first fresh canary still exposed one remaining hosted-entry
   bug:
   - control-plane handoff creation succeeded
   - tenant entry still failed because runtime org/handoff resolution depended
     on forwarded-host inference instead of a canonical injected tenant ID
4. Deployed the follow-up hosted runtime bootstrap fix to production so fresh
   hosted tenants carry an explicit identity at runtime:
   - tenant env now includes `PULSE_TENANT_ID=<tenant-id>`
   - hosted env also preserves the explicit tenant public URL and hosted mode
     flags for runtime bootstrap
5. Created a second fresh canary after the tenant-ID fix:
   - tenant id `t-8ME7XXQM7X`
   - inspected `/data/tenants/t-8ME7XXQM7X/orgs/t-8ME7XXQM7X/org.json`
   - confirmed `ownerUserId=operator+msp-owner-seed-20260313124930@pulserelay.pro`
   - confirmed the live container env included
     `PULSE_TENANT_ID=t-8ME7XXQM7X`
6. Exercised the real hosted handoff path against the successful canary using
   the production operator session:
   - fetched a real control-plane handoff form for
     `a_msp_owner_seed_20260313124930`
   - exchanged the handoff at
     `POST https://t-8ME7XXQM7X.cloud.pulserelay.pro/api/cloud/handoff/exchange?format=json`
   - response returned `200` with a successful tenant session exchange payload
7. Continued the same tenant session into hosted runtime surfaces that had been
   failing on the stale path:
   - `GET /api/admin/orgs/t-8ME7XXQM7X/billing-state` returned `200`
   - `GET /api/orgs/t-8ME7XXQM7X/members` returned `200`
   - members payload included both the legacy owner and the authenticated
     operator owner, matching the seeded MSP account

## Outcome

- This is successful `real-external-e2e` production evidence.
- The fresh hosted runtime can now be entered through the real cloud handoff
  path without collapsing back to forwarded-host guessing.
- Hosted runtime state stays coherent after entry:
  - tenant ownership remains aligned with the authenticated creator
  - the runtime resolves the tenant-scoped org correctly
  - hosted billing and org-member surfaces respond successfully on the live
    tenant
- This later same-day follow-up supersedes the earlier failed intermediate
  hosted-handoff diagnosis and shows the repaired runtime path on a fresh
  production canary.

## Conclusion

- `cloud-hosted-tier-runtime-readiness` remains correctly `passed`.
- `RA11` is now backed not only by the earlier hosted runtime-entry repair
  record, but also by a later fresh-canary proof that tenant-scoped handoff,
  owner seeding, and hosted post-login surfaces stay coherent on production.
