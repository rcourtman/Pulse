# Hosted Signup Billing Replay Production Fixed Record

- Date: `2026-03-13`
- Gate: `hosted-signup-billing-replay`
- Assertion: `RA2`
- Environment:
  - External control plane: `https://cloud.pulserelay.pro`
  - Remote host: `root@pulse-cloud`
  - Evidence source: production HTTPS surface, production Stripe event replay, control-plane tenant registry, and control-plane logs

## External Exercise

1. Reused the governed production control-plane surface that was already fixed for hosted runtime readiness:
   - `CONTROL_PLANE_IMAGE=pulse-control-plane:634fa3a66663-20260313T115951Z`
   - `CP_PULSE_IMAGE=pulse-runtime:b6800fe1f401-20260313T121150Z-pubkey`
   - `deploy/cloud/preflight-live.sh` remained clean against `cloud.pulserelay.pro`
2. Confirmed the remaining hosted-signup blocker was a real failed production checkout webhook, not a synthetic rehearsal:
   - `stripe_events.stripe_event_id=evt_1TAGtCBrHBocJIGHePETZL11`
   - `event_type=checkout.session.completed`
   - previous `processing_error="tenant t-5PAARCDCJM container failed health check"`
   - corresponding Stripe account already existed for `customer_id=cus_U8Xy7ujZlLnTha`
   - no tenant existed yet for account `a_S3N3VSKK7A`
3. Fetched the exact production Stripe event payload from the live Stripe API using the configured live `STRIPE_API_KEY`.
4. Replayed that exact event through the real production webhook endpoint with a fresh valid signature using the configured live `STRIPE_WEBHOOK_SECRET`:
   - `POST https://cloud.pulserelay.pro/api/stripe/webhook`
   - response was `200 {"received":true}`
5. Confirmed the replay was not a no-op duplicate:
   - `stripe_events.processing_error` for `evt_1TAGtCBrHBocJIGHePETZL11` is now `NULL`
   - `processed_at` advanced to the new replay time
6. Confirmed the replayed production checkout now provisions the tenant successfully:
   - new tenant ID: `t-YSK1GQDZS2`
   - account: `a_S3N3VSKK7A`
   - email: `alfons@fonsie.eu`
   - state: `active`
   - `stripe_customer_id=cus_U8Xy7ujZlLnTha`
   - `stripe_subscription_id=sub_1TAGsrBrHBocJIGHHwpjRMHg`
7. Confirmed the tenant runtime actually started on the fixed hosted image:
   - Docker container: `pulse-t-YSK1GQDZS2`
   - image: `pulse-runtime:b6800fe1f401-20260313T121150Z-pubkey`
8. Confirmed post-replay operator signals in production control-plane logs:
   - `Tenant container started`
   - `Magic link email sent`
   - `Tenant provisioned from checkout`

## Outcome

- This is real external production evidence, not localhost rehearsal.
- The public hosted signup surface had already been proven to create real Stripe checkout sessions on production in `hosted-signup-billing-replay-production-2026-03-13.md`.
- The missing piece was a real completed checkout replay proving that a previously failed production `checkout.session.completed` event can now be retried successfully through the live webhook.
- That replay now succeeds end to end:
  - the exact failed Stripe event was reclaimed for retry
  - webhook processing completed without error
  - a real tenant was provisioned and became active
  - the runtime container started on the fixed hosted image
  - the magic-link email path fired for the newly provisioned tenant
- `hosted-signup-billing-replay` can now be treated as passed with `real-external-e2e` evidence.
- `RA2` is now backed by the real hosted checkout replay path instead of only local rehearsal plus incomplete production checkout creation.

## Notes

- This proof intentionally used a previously failed production Stripe event instead of inventing a new synthetic checkout completion. That makes the evidence stronger: it proves the live replay path can recover a real stuck hosted signup after the runtime fixes.
- The hosted runtime-entry path remains separately governed under `cloud-hosted-tier-runtime-readiness`; this record closes the checkout/webhook replay side of the hosted commercial journey.
