# Cloud Hosted Tier Runtime Readiness Production Recovered Record

- Date: `2026-03-26`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `passed`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Control-plane image: `pulse-control-plane:pulse-account-8ecea8dbd`
  - Hosted runtime code fix: `pulse` commit `8b91ab7c0`
  - Final tenant runtime image: `pulse-runtime:hosted-handoff-8b91ab7c0`
  - Dedicated MSP rehearsal account: `a_msp_prod_fix_20260313111348`
  - Dedicated provider owner session: `owner+msp-fix-20260313111348@pulserelay.pro`
  - Recovered hosted tenant: `t-EF1B7YCNZE`
  - Evidence source: live production HTTPS surface, admin-key protected control-plane APIs, tenant container inspection, and live hosted tenant logs on `pulse-cloud`

## Runtime Failure That Was Still Real

1. The signed-in `Pulse Account` portal was live and could mint tenant handoff
   tokens, but the real browser `Open` path for `t-EF1B7YCNZE` still failed at
   the tenant runtime:
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/tenants/t-EF1B7YCNZE/handoff` returned `200`
   - `POST https://t-EF1B7YCNZE.cloud.pulserelay.pro/api/cloud/handoff/exchange` returned `500 internal error`
2. The failing tenant container still carried an older hosted runtime env
   contract:
   - `PULSE_HOSTED_MODE=true`
   - `PULSE_MULTI_TENANT_ENABLED=true`
   - `PULSE_PUBLIC_URL=https://t-EF1B7YCNZE.cloud.pulserelay.pro`
   - `PULSE_TENANT_ID` missing
3. The same tenant container still carried the older immutable-billing mount
   contract:
   - `/data/tenants/t-EF1B7YCNZE` bind-mounted to `/etc/pulse`
   - `/data/tenants/t-EF1B7YCNZE/billing.json` separately bind-mounted read-only
     to `/etc/pulse/billing.json`
4. Direct inspection confirmed the handoff failure happened before replay-store
   or browser-session success, and the old tenant never completed the hosted
   browser handoff.

## Canonical Repair Chain

1. Fixed tenant-context recovery at the hosted runtime boundary in `pulse`
   commit `8b91ab7c0`:
   - `internal/api/cloud_handoff_handlers.go`
   - `internal/api/cloud_handoff_handlers_test.go`
   - governed in `internal/api/contract_test.go`,
     `docs/release-control/v6/internal/subsystems/api-contracts.md`,
     `docs/release-control/v6/internal/subsystems/agent-lifecycle.md`, and
     `docs/release-control/v6/internal/subsystems/storage-recovery.md`
2. Rebuilt the hosted tenant runtime image from that exact commit with the same
   release-path public-key embedding model as the current hosted runtime line:
   - `pulse-runtime:hosted-handoff-8b91ab7c0`
3. Updated `CP_PULSE_IMAGE` on `pulse-cloud` to
   `pulse-runtime:hosted-handoff-8b91ab7c0` and restarted only the control
   plane so future hosted tenants inherit the corrected image.
4. Replaced the already-running tenant container `pulse-t-EF1B7YCNZE` onto the
   new runtime image with its existing data, labels, and hosted routing
   contract intact.
5. Proved the compatibility fix against the old broken env shape first:
   - no `PULSE_TENANT_ID`
   - same hosted subdomain
   - same old data directory
   - handoff exchange completed and minted tenant cookies instead of returning
     `500`
6. Then moved the tenant onto the current canonical hosted runtime contract:
   - writable `/etc/pulse` tenant bind
   - only handoff secrets immutable
   - explicit `PULSE_TENANT_ID=t-EF1B7YCNZE`

## External Rehearsal

1. Generated a fresh admin magic link for the recovered tenant:
   - `POST https://cloud.pulserelay.pro/admin/magic-link`
   - `email=owner+msp-fix-20260313111348@pulserelay.pro`
   - `tenant_id=t-EF1B7YCNZE`
2. Confirmed the tenant magic-link verify flow still lands on the hosted tenant
   runtime:
   - final URL `https://t-EF1B7YCNZE.cloud.pulserelay.pro/`
3. Confirmed the signed-in `Pulse Account` portal still works for the MSP
   account and can open the real workspace handoff path:
   - `GET /portal?account_id=a_msp_prod_fix_20260313111348` returned `200`
   - `POST /api/accounts/a_msp_prod_fix_20260313111348/tenants/t-EF1B7YCNZE/handoff` returned `200`
4. Exercised the real tenant exchange path after the runtime fix:
   - `POST https://t-EF1B7YCNZE.cloud.pulserelay.pro/api/cloud/handoff/exchange`
     returned `307`
   - redirect target was `/`
   - tenant cookies were minted:
     - `pulse_session`
     - `pulse_csrf`
     - `pulse_org_id`
5. Continued the same tenant session into runtime entry:
   - `GET https://t-EF1B7YCNZE.cloud.pulserelay.pro/` returned `200 text/html`
   - tenant logs recorded `Cloud handoff completed, session created`
6. Confirmed the final tenant container now carries the corrected hosted env
   contract:
   - `PULSE_HOSTED_MODE=true`
   - `PULSE_MULTI_TENANT_ENABLED=true`
   - `PULSE_TENANT_ID=t-EF1B7YCNZE`
   - `PULSE_PUBLIC_URL=https://t-EF1B7YCNZE.cloud.pulserelay.pro`

## Outcome

- This is successful `real-external-e2e` production evidence.
- The current hosted Pulse tier now has fresh production proof for:
  - hosted auth/session entry through the control plane
  - signed-in `Pulse Account` access for the MSP account surface
  - real workspace `Open` handoff from `Pulse Account`
  - tenant browser-session creation on the hosted runtime
  - working hosted runtime entry on the tenant root path
- The previously blocking `500 internal error` on hosted tenant handoff has been
  removed from the live system.

## Conclusion

- `cloud-hosted-tier-runtime-readiness` can be treated as `passed` again.
- `RA11` is now backed by fresh `real-external-e2e` hosted runtime evidence on
  the current `Pulse Account` control-plane path, not only on older standalone
  magic-link rehearsals.
- Older hosted tenant containers still need explicit recreation to pick up a
  new tenant runtime image, but the live hosted/runtime proof is restored and
  the tenant runtime contract has been corrected on the recovered canary.
