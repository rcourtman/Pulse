# Hosted Signup Billing Replay Production Record

- Date: `2026-03-13`
- Gate: `hosted-signup-billing-replay`
- Assertion: `RA2`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Evidence source: production HTTPS surface, admin-key protected control-plane APIs, and control-plane logs

## External Exercise

1. Ran the governed production preflight against the real external service:
   - `DOMAIN=cloud.pulserelay.pro SSH_TARGET=root@pulse-cloud ADMIN_KEY_FILE=/Volumes/Development/pulse/secrets/cloud-cp/admin_key deploy/cloud/preflight-live.sh`
2. Confirmed the real public hosted-signup surface was reachable and contract-valid:
   - `GET /healthz` returned `200`
   - `GET /readyz` returned `200`
   - `GET /signup` returned `200`
   - `GET /cloud/signup` returned `200`
   - `GET /signup/complete` returned `200`
   - `GET /api/public/signup` returned `405`
   - invalid `POST /api/public/signup` returned `400`
   - invalid `POST /api/public/magic-link/request` returned `400`
   - valid-shape `POST /api/public/magic-link/request` returned `200`
3. Confirmed the real admin control surface was reachable:
   - `GET /status` with the production admin key returned `{"version":"dev","total_tenants":16,"healthy":16,"unhealthy":0,"by_state":{"active":16}}`
   - SSH to `root@pulse-cloud` succeeded
4. Confirmed the live production env is not actually ready for a clean hosted checkout/trial replay:
   - `/opt/pulse-cloud/.env` contains `CP_TRIAL_SIGNUP_PRICE_ID`
   - `/opt/pulse-cloud/.env` does not contain `CP_TRIAL_ACTIVATION_PRIVATE_KEY`
   - this is a real env mismatch, not a local-doc artifact
5. Pulled recent production control-plane logs and confirmed repeated real checkout completion failures on the external service:
   - multiple `checkout.session.completed` events in the last 12 hours logged `Stripe webhook processing failed`
   - the concrete failure was `tenant <id> container failed health check`
   - observed failing tenant IDs included `t-KPXEWNB56Z`, `t-P1QD6QHHWK`, `t-N6BEJKY9AW`, `t-SJY27FXC1V`, `t-1GGB3EX439`, `t-5PAARCDCJM`, `t-S5MPK98VM7`, `t-CJTV56H46F`, `t-YBP562AM1E`, `t-JRDF96FXTB`, and `t-0Y0C9NG6PR`
6. Did not initiate a fresh live checkout from production:
   - production is configured for live Stripe
   - the existing external evidence already shows real checkout completion failures
   - creating an additional live billing flow would add customer-visible or finance-visible side effects without improving confidence

## Outcome

- This is real external production evidence, not localhost rehearsal.
- The hosted public signup surface is up, but the real paid provisioning path is not healthy enough to close the gate.
- `hosted-signup-billing-replay` remains pending because production currently shows:
  - missing `CP_TRIAL_ACTIVATION_PRIVATE_KEY` in the control-plane env
  - repeated real `checkout.session.completed` failures ending in tenant container health-check failure
- The gate should only move after the production env is corrected and a fresh external hosted checkout plus replay path succeeds cleanly.
