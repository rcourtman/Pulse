# Commercial Cancellation/Reactivation External E2E 2026-03-13

- Gate: `commercial-cancellation-reactivation`
- Assertions:
  - `RA2`
  - `RA4`
  - `RA7`
  - `RA16`
- Evidence tier: `real-external-e2e`
- Operator date: `2026-03-13`
- Operator: `Codex on local workspace driving real public HTTPS surfaces`

## Topology

- Hosted-mode Pulse runtime:
  - URL: `https://ccr-runtime.cloud.pulserelay.pro`
  - Host: temporary public Traefik-routed container on `pulse-cloud`
  - Stripe webhook ingress used by Pulse: `POST /api/webhooks/stripe`
- Commercial checkout runtime:
  - URL: `https://ccr-checkout.cloud.pulserelay.pro`
  - Runtime: temporary public Traefik-routed `pulse-pro/license-server` container on `pulse-cloud`
  - Stripe mode: `sandbox/test`
- External payment/provider surfaces:
  - Stripe-hosted billing portal
  - Stripe-hosted checkout

## Why This Record Replaces The Older External Narrative

The gate was already marked `passed`, but the existing 2026-03-13 record still described an older local/HTTP staging topology. This replacement records the actual public HTTPS rehearsal that passed:

- the real Pulse hosted-mode runtime was reachable on a public external URL
- the real checkout runtime was reachable on a public external URL
- cancellation/resume used the real Stripe billing portal in the browser
- repurchase used the real Stripe-hosted checkout in the browser
- no API-equivalent fallback was used for the cancellation/resume or repurchase path

## Fixtures

- Monthly grandfathered customer:
  - Email: `ccr-1773408158270@example.com`
  - Stripe customer ID: `cus_U8nGR5FZJgraIQ`
  - Stripe subscription ID: `sub_1TAVfnPZ0VLEY1aVS8AP2UUV`
  - Legacy price ID: `price_1TAVYiPZ0VLEY1aV17RZUocW`
  - Test clock ID: `clock_1TAVfmPZ0VLEY1aVpvkB8r2d`
- Annual grandfathered customer:
  - Email: `ccr-1773408158270@example.com`
  - Stripe customer ID: `cus_U8nG0zDM1XM0qZ`
  - Stripe subscription ID: `sub_1TAVfsPZ0VLEY1aVpiG1wb1B`
  - Legacy price ID: `price_1TAVYjPZ0VLEY1aVZYR0taZL`
  - Test clock ID: `clock_1TAVfrPZ0VLEY1aVFni8FenJ`
- Public v6 checkout plan keys:
  - Monthly: current test-mode Pro monthly v6 plan key configured on the temporary checkout runtime
  - Annual: current test-mode Pro annual v6 plan key configured on the temporary checkout runtime
- Direct legacy rejection key:
  - `price_v5_pro_monthly`

## Commands And Proof

Automated proof floor:

- `python3 scripts/release_control/commercial_cancellation_reactivation_proof.py --json`
- Result: `pass`

Live browser rehearsal:

```bash
cd /Volumes/Development/pulse/repos/pulse/tests/integration
PULSE_E2E_SKIP_DOCKER=1 \
PULSE_BASE_URL=https://ccr-runtime.cloud.pulserelay.pro \
PULSE_CCR_CHECKOUT_BASE_URL=https://ccr-checkout.cloud.pulserelay.pro \
PULSE_CCR_CHECKOUT_RESULT_BASE_URL=https://ccr-checkout.cloud.pulserelay.pro \
PULSE_CCR_WEBHOOK_BASE_URL=https://ccr-runtime.cloud.pulserelay.pro \
PULSE_CCR_WEBHOOK_PATH=/api/webhooks/stripe \
PULSE_CCR_ALLOW_BILLING_STATE_SEED=true \
npm test -- tests/14-commercial-cancellation-reactivation.spec.ts --project=chromium
```

Live browser result:

- `2 passed (2.2m)`

## Executed Scenarios

### `CCR-1` through `CCR-5` Monthly Path

Observed:

- Active grandfathered monthly continuity stayed intact while the legacy subscription remained active.
- Cancel-at-period-end did not rewrite the customer early.
- Resume-before-lapse preserved the same legacy recurring identity.
- Completed cancellation revoked paid state cleanly in Pulse.
- Post-cancel repurchase completed through real Stripe-hosted checkout and the fulfillment returned the current public v6 monthly plan key, not a revived grandfathered plan.

Monthly v6 re-entry evidence:

- Fulfilled checkout session recorded by the checkout runtime:
  - License ID: `lic_04b25c69447890d3237874cdded92f3a`
  - Plan key: current test-mode Pro monthly v6 plan key configured on the temporary checkout runtime
  - Checkout session ID: `cs_test_b1iqGMKvA5bk27C2TaUdxsP7K1YqE6m9ufR7lmmlEge1BVDiaScj8TzHv1`

### `CCR-6` Annual Parity

Observed:

- Annual grandfathered continuity stayed intact while active.
- Completed annual cancellation revoked paid state cleanly.
- Post-cancel repurchase completed through real Stripe-hosted checkout and the fulfillment returned the current public v6 annual plan key, not a revived grandfathered plan.

Annual v6 re-entry evidence:

- Fulfilled checkout session recorded by the checkout runtime:
  - License ID: `lic_1aeed899e926c9200d3cc5c57e9a9fe9`
  - Plan key: current test-mode Pro annual v6 plan key configured on the temporary checkout runtime
  - Checkout session ID: `cs_test_b1uEc0cXgNW9OvsuMD04izn7cqb6MDzGRpfyopCyjZfp5M2IN8tTDc0AS7`

### `CCR-7` Direct Legacy Checkout Rejection

Observed against `https://ccr-checkout.cloud.pulserelay.pro`:

- `POST /v1/checkout/session` with `plan_key=price_v5_pro_monthly` returned `400`
- Response body contained `not a v6 checkout plan`
- No Stripe checkout session was created

## Evidence Captured

- Playwright result bundle:
  - `tests/integration/test-results/junit.xml`
- Playwright browser artifact:
  - `tests/integration/test-results/.playwright-artifacts-0/106d1b678691c13eace08d6de5972835.webm`
- Hosted runtime evidence:
  - runtime login, billing-state seed, and webhook replays observed on `https://ccr-runtime.cloud.pulserelay.pro`
- Checkout runtime evidence:
  - `docker logs pulse-ccr-license` on `pulse-cloud` recorded the fulfilled monthly and annual v6 checkout sessions above

## Outcome

- `pass`
- Monthly continuity boundary: `pass`
- Annual parity boundary: `pass`
- Legacy direct checkout rejection: `pass`

## Release Impact

`commercial-cancellation-reactivation` remains satisfied at the required `real-external-e2e` tier, and the governed record now matches the actual public HTTPS rehearsal that passed on `2026-03-13`.
