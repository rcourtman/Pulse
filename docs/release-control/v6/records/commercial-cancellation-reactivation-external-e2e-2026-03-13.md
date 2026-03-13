# Commercial Cancellation/Reactivation External E2E 2026-03-13

- Gate: `commercial-cancellation-reactivation`
- Assertion: `RA16`
- Evidence tier: `real-external-e2e`
- Operator date: `2026-03-13`
- Runtime topology:
  - Hosted-mode Pulse runtime: `http://192.168.0.106:17655`
  - Staging commercial checkout runtime: `http://127.0.0.1:18080`
  - External payment/provider surface: Stripe-hosted checkout and Stripe customer portal in sandbox/test mode

## Why This Rehearsal Replaced The Earlier Record

The prior 2026-03-12 record proved the behavior floor, but parts of the repurchase path fell back to Stripe API equivalents because the browser journey was still too brittle. This rehearsal reran the governed path with:

- the real hosted-mode Pulse runtime still carrying the entitlement and webhook state transitions
- the real `pulse-pro/license-server` binary running in Stripe test mode
- real Stripe-hosted checkout and customer-portal browser flows
- no API-equivalent fallback for the repurchase or cancellation/resume journey

That lifts the gate from `managed-runtime-exercise` to `real-external-e2e`.

## Runtime Setup

### Pulse Runtime

- Base URL: `http://192.168.0.106:17655`
- Mode: hosted-mode Pulse runtime for commercial continuity entitlement checks
- Auth: local admin login on the rehearsal runtime
- Stripe webhook ingress used by Pulse runtime:
  `POST /api/webhooks/stripe`

### Commercial Checkout Runtime

- Binary: locally built `pulse-pro/license-server`
- Base URL: `http://127.0.0.1:18080`
- Stripe mode: sandbox/test mode
- Public v6 checkout plan keys:
  - Monthly: current test-mode Pro monthly v6 plan key served by the staging commercial runtime
  - Annual: current test-mode Pro annual v6 plan key served by the staging commercial runtime
- Legacy rejection plan key:
  - `price_v5_pro_monthly`

## Executed Scenarios

### `CCR-1` through `CCR-5` Monthly Path

Executed with Playwright against:

- Pulse runtime: `http://192.168.0.106:17655`
- Checkout/result/landing override: `http://127.0.0.1:18080`

Observed:

- Active grandfathered monthly continuity stayed intact while the legacy subscription remained active.
- Cancel-at-period-end did not rewrite the customer early.
- Resume-before-lapse preserved the same legacy recurring identity.
- Completed cancellation revoked paid state cleanly in Pulse.
- Post-cancel re-entry completed through real Stripe-hosted checkout and landed on the current v6 monthly plan, not a revived grandfathered plan.

### `CCR-6` Annual Parity

Executed in the same topology with the annual public v6 checkout plan key.

Observed:

- Annual grandfathered continuity stayed intact while active.
- Completed annual cancellation revoked paid state cleanly.
- Post-cancel annual re-entry completed through real Stripe-hosted checkout and landed on the current v6 annual plan, not a revived grandfathered plan.

### `CCR-7` Direct Legacy Checkout Rejection

Observed against the staging commercial runtime:

- `POST /v1/checkout/session` with `plan_key=price_v5_pro_monthly`
  returned `400`
- Response body contained `not a v6 checkout plan`
- No Stripe checkout session was created

## Commands And Proof

Automated proof floor:

- `python3 scripts/release_control/commercial_cancellation_reactivation_proof.py --json`

Browser rehearsal:

```bash
cd /Volumes/Development/pulse/repos/pulse/tests/integration
PULSE_E2E_SKIP_DOCKER=1 npx playwright test tests/14-commercial-cancellation-reactivation.spec.ts --project=chromium
```

Required env overrides for the successful external rehearsal:

- `PULSE_COMMERCIAL_BASE_URL=http://192.168.0.106:17655`
- `PULSE_CCR_WEBHOOK_BASE_URL=http://192.168.0.106:17655`
- `PULSE_CCR_WEBHOOK_PATH=/api/webhooks/stripe`
- `PULSE_CCR_ALLOW_BILLING_STATE_SEED=1`
- `PULSE_CCR_CHECKOUT_BASE_URL=http://127.0.0.1:18080`
- `PULSE_CCR_CHECKOUT_RESULT_BASE_URL=http://127.0.0.1:18080`
- `PULSE_CCR_LANDING_BASE_URL=http://127.0.0.1:18080`
- `PULSE_CCR_V6_MONTHLY_PLAN_KEY=<current test-mode monthly v6 plan key>`
- `PULSE_CCR_V6_ANNUAL_PLAN_KEY=<current test-mode annual v6 plan key>`

## Outcome

- `pass`
- Monthly continuity boundary: `pass`
- Annual parity boundary: `pass`
- Legacy direct checkout rejection: `pass`

## Release Impact

`commercial-cancellation-reactivation` is now satisfied at the required `real-external-e2e` tier. `RA16` can derive as passed again from governed evidence rather than from a weaker local/manual closure.
