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
4. Corrected the live production control-plane env mismatch and revalidated preflight:
   - `/opt/pulse-cloud/.env` now contains `CP_TRIAL_SIGNUP_PRICE_ID`
   - `/opt/pulse-cloud/.env` now contains `CP_TRIAL_ACTIVATION_PRIVATE_KEY`
   - `docker compose up -d control-plane` completed cleanly on `root@pulse-cloud`
   - rerunning `deploy/cloud/preflight-live.sh` passed with `failures=0` and `warnings=0`
5. Confirmed the real public hosted-signup surface now reaches live Stripe checkout creation on production:
   - `POST /api/public/signup` with a fresh dedicated test email returned `200`
   - response contained a real `checkout_url=https://checkout.stripe.com/...`
   - response message was `Checkout session created. Continue in Stripe to provision your Pulse Cloud tenant.`
6. Pulled recent production control-plane logs and confirmed there is still unresolved evidence on completed checkout execution from earlier live traffic:
   - multiple `checkout.session.completed` events in the last 12 hours logged `Stripe webhook processing failed`
   - the concrete failure was `tenant <id> container failed health check`
   - observed failing tenant IDs included `t-KPXEWNB56Z`, `t-P1QD6QHHWK`, `t-N6BEJKY9AW`, `t-SJY27FXC1V`, `t-1GGB3EX439`, `t-5PAARCDCJM`, `t-S5MPK98VM7`, `t-CJTV56H46F`, `t-YBP562AM1E`, `t-JRDF96FXTB`, and `t-0Y0C9NG6PR`
7. Did not complete a fresh live checkout on production:
   - production is configured for live Stripe
   - creating a completed live billing flow would introduce real finance-visible side effects
   - the new evidence proves checkout-session creation is healthy again, but not yet successful webhook completion through to a healthy tenant runtime

## Outcome

- This is real external production evidence, not localhost rehearsal.
- The hosted public signup surface is up and can now create a real Stripe checkout session on production.
- `hosted-signup-billing-replay` remains pending because production currently shows:
  - no fresh external proof yet that a completed production checkout now replays cleanly through webhook handling into a healthy provisioned tenant
  - historical real `checkout.session.completed` failures still exist in the production evidence set and have not yet been displaced by a successful completed run
- The gate should only move after a fresh external hosted checkout plus replay path succeeds cleanly on production.
