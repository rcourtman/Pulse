# Cloud Hosted Tier Runtime Readiness Production Follow-up Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Dedicated hosted rehearsal tenant: `t-P9ES7BWFBT`
  - Dedicated MSP rehearsal account: `a_msp_prod_fix_20260313111348`
  - Evidence source: live production HTTPS surface, admin-key protected control-plane APIs, and tenant container inspection/logs

## External Follow-up Exercise

1. Reused the dedicated production MSP rehearsal tenant created earlier for the
   MSP gate:
   - tenant id `t-P9ES7BWFBT`
   - display name `Client Alpha Fix 20260313111348`
   - control-plane health `health_check_ok=true`
2. Generated a fresh production admin magic link for the dedicated tenant:
   - `POST https://cloud.pulserelay.pro/admin/magic-link`
   - `email=owner+msp-fix-20260313111348@pulserelay.pro`
   - `tenant_id=t-P9ES7BWFBT`
3. Confirmed the control plane minted the correct tenant handoff target:
   - redirect target was `https://t-P9ES7BWFBT.cloud.pulserelay.pro/auth/cloud-handoff?...`
   - decoded handoff payload carried `"t":"t-P9ES7BWFBT"`
4. Exercised the real tenant handoff path before any runtime repair:
   - `GET /auth/cloud-handoff` on the tenant returned `404`
   - tenant logs recorded the failing request on the live container
5. Inspected the live tenant data dir and immutable mounts on `pulse-cloud`:
   - `/data/tenants/t-P9ES7BWFBT/.cloud_handoff_key` existed
   - `/data/tenants/t-P9ES7BWFBT/secrets/handoff.key` existed
   - `/data/tenants/t-P9ES7BWFBT/billing.json` existed
   - all three files were `root:root 0600`
   - the tenant container was running with `PULSE_HOSTED_MODE=true`
6. Confirmed the runtime bug mechanism:
   - the hosted tenant startup fix had correctly stopped startup-time `chown`
     against immutable files
   - but the control plane was still writing those same immutable files as
     `root:root 0600`
   - the tenant Pulse runtime therefore could not read the handoff key or the
     hosted billing lease even though the files were mounted correctly
7. Applied a narrow live ownership repair only on the dedicated rehearsal
   tenant to validate the diagnosis:
   - `chown 1000:1000 .cloud_handoff_key secrets/handoff.key billing.json`
8. Re-ran the exact same production handoff flow after the ownership repair:
   - `GET /auth/cloud-handoff` now completed and created tenant session cookies
   - final tenant runtime landing page returned `200`
   - tenant logs recorded `Cloud handoff completed, session created`
9. Continued the same runtime drill into hosted post-login surfaces:
   - `GET /api/license/status` still returned free-tier state
   - `GET /api/admin/orgs/default/billing-state` still returned `403 access_denied`
   - the post-handoff tenant cookie was still `pulse_org_id=default`

## Outcome

- This is real external production evidence, not localhost rehearsal.
- It proves a concrete hosted runtime blocker:
  - immutable hosted files were being written with ownership that the tenant
    runtime could not read after the startup `chown` fix
- The repo-side fix belongs in hosted provisioning and hosted billing-state
  rewrite ownership, not in another startup mutation workaround.
- The narrow live ownership repair also proved there is at least one more
  runtime issue after handoff:
  - the tenant session still falls back to `default` org/free-tier behavior
    instead of landing in a coherent hosted paid runtime

## Conclusion

- `cloud-hosted-tier-runtime-readiness` remains pending.
- The gate cannot pass honestly yet because the live hosted runtime still does
  not land in a coherent paid hosted state after sign-in.
- This follow-up record supersedes the earlier assumption that tenant health
  alone plus checkout creation was close enough to runtime readiness.
