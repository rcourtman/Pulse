# Cloud Hosted Tier Runtime Readiness Production Record

- Date: `2026-03-13`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Evidence source: production HTTPS surface, admin-key protected control-plane APIs, and control-plane logs

## External Exercise

1. Ran the governed production preflight against the real external hosted service:
   - `DOMAIN=cloud.pulserelay.pro SSH_TARGET=root@pulse-cloud ADMIN_KEY_FILE=/Volumes/Development/pulse/secrets/cloud-cp/admin_key deploy/cloud/preflight-live.sh`
2. Confirmed the public hosted entry surfaces are live on production:
   - `GET /healthz` returned `200`
   - `GET /readyz` returned `200`
   - `GET /signup` returned `200`
   - `GET /cloud/signup` returned `200`
   - `GET /signup/complete` returned `200`
3. Confirmed the production control plane currently reports existing hosted tenants as healthy:
   - `GET /status` with the production admin key returned `total_tenants=16`, `healthy=16`, `unhealthy=0`
   - `GET /admin/tenants` returned 16 active hosted tenants on the live service
4. Confirmed the operator path itself was healthy once the real SSH target was used:
   - SSH to `root@pulse-cloud` succeeded
   - compose, digest pinning, and backup/restore guardrails passed
5. Corrected the live production control-plane env mismatch and revalidated preflight:
   - `/opt/pulse-cloud/.env` now contains `CP_TRIAL_ACTIVATION_PRIVATE_KEY`
   - restarted `pulse-cloud-control-plane-1` cleanly via `docker compose up -d control-plane`
   - rerunning `deploy/cloud/preflight-live.sh` passed with `failures=0` and `warnings=0`
6. Confirmed the external hosted entry path now reaches live Stripe checkout creation:
   - `POST /api/public/signup` with a fresh dedicated production test email returned a real `checkout_url=https://checkout.stripe.com/...`
   - this is stronger than the earlier broken-env state because the live public hosted surface now reaches the external billing boundary successfully
7. Did not generate a runtime login against an existing production tenant:
   - no clearly dedicated internal hosted tenant was identified for a safe rehearsal
   - using an admin-generated magic link against a real customer tenant would amount to impersonating a live customer workspace for release proof
   - a fresh production checkout was not completed because it would create real finance-visible side effects on the live Stripe environment

## Outcome

- This is real external production evidence, not localhost rehearsal.
- The live hosted surface is reachable and the hosted signup path now reaches real checkout creation on production.
- The gate still cannot pass honestly because there is not yet fresh external evidence that a completed production checkout leads to a healthy tenant runtime that can actually be entered and used.
- `cloud-hosted-tier-runtime-readiness` remains pending until:
  - checkout-driven provisioning is re-exercised successfully on the real external service
  - a safe hosted runtime-entry drill can be completed without relying on a real customer workspace
