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
5. Confirmed production still has a concrete live blocker on the post-checkout hosted path:
   - `/opt/pulse-cloud/.env` is missing `CP_TRIAL_ACTIVATION_PRIVATE_KEY`
   - recent control-plane logs show repeated `checkout.session.completed` failures with `tenant <id> container failed health check`
6. Did not generate a runtime login against an existing production tenant:
   - no clearly dedicated internal hosted tenant was identified for a safe rehearsal
   - using an admin-generated magic link against a real customer tenant would amount to impersonating a live customer workspace for release proof
   - because the production signup/provisioning path is already showing live failures, that additional intrusion was not justified

## Outcome

- This is real external production evidence, not localhost rehearsal.
- The live hosted surface is reachable, but the gate still cannot pass honestly because new post-checkout provisioning is already failing on production.
- `cloud-hosted-tier-runtime-readiness` remains pending until:
  - production trial/signup env wiring is corrected
  - checkout-driven provisioning succeeds on the real external service
  - a safe hosted runtime-entry drill can be completed without relying on a real customer workspace
