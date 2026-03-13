# Commercial Cancellation/Reactivation Record

- Date: `2026-03-12`
- Gate: `commercial-cancellation-reactivation`
- Assertions:
  - `RA2`
  - `RA4`
  - `RA7`
- Environment:
  - Billing environment: `Stripe sandbox (test mode) using local fixture product and prices`
  - Pulse runtime URL: `http://127.0.0.1:18765`
  - pulse-pro checkout origin: `https://pulserelay.pro`
  - Stripe mode: `test`
  - Operator: `Codex on local workspace`

## Fixtures

- Monthly grandfathered customer:
  - Email: `ccr-monthly-20260312@example.com`
  - Stripe customer ID: `cus_U8ZvmRtpV7pUea`
  - Stripe subscription ID: `sub_1TAInIPZ0VLEY1aVsh9hYzkp`
  - Legacy price ID: `price_1TAIjIPZ0VLEY1aVwy8Kh14D`
- Annual grandfathered customer:
  - Email: `ccr-monthly-20260312@example.com`
  - Stripe customer ID: `cus_U8ZvmRtpV7pUea`
  - Stripe subscription ID: `sub_1TAJ5BPZ0VLEY1aVMsM9b1vN`
  - Legacy price ID: `price_1TAIjJPZ0VLEY1aVNS6p8BTN`
- Returning post-cancel customer:
  - Email: `ccr-monthly-20260312@example.com`

Note:

- The annual spot check reused the already-mapped monthly sandbox customer after a confirmed canceled state. This avoided synthetic org-mapping edits in the local hosted-mode Pulse rehearsal while still exercising real Stripe subscriptions and real webhook delivery.

## Automated Proof Baseline

- `python3 scripts/release_control/commercial_cancellation_reactivation_proof.py --json`
- `cd /Volumes/Development/pulse/repos/pulse-pro/license-server && go test . -run 'TestHandleCheckoutSessionCreate(_RejectsGrandfatheredPlanKey)?$' -count=1`
- Result: `pass`

## Manual Exercise

### `CCR-1` Active Grandfathered Continuity Baseline

1. Created a Stripe sandbox test clock and monthly recurring customer fixture.
2. Completed a real hosted Stripe Checkout session on the legacy monthly recurring price.
3. Refreshed the local hosted-mode Pulse settings surface and `GET /api/license/entitlements`.

Observed:

- Stripe price ID: `price_1TAIjIPZ0VLEY1aVwy8Kh14D`
- `GET /api/license/entitlements` `plan_version`: `v5_pro_monthly_grandfathered`
- Settings surface continuity notice: `present`

### `CCR-2` Cancel At Period End Without Immediate Drift

1. Scheduled cancellation at period end using the Stripe subscription API equivalent because the hosted billing-portal page rendered an unusable no-JavaScript shell in headless mode.
2. Refreshed Pulse entitlements and the settings surface before the billing period ended.

Observed:

- Cancel-at-period-end state: `true`
- Legacy price ID still attached: `yes`
- Entitlement state before period end: `active`

### `CCR-3` Resume Before Lapse

1. Removed `cancel_at_period_end` from the same monthly Stripe subscription before the test clock reached the end of the period.
2. Refreshed Pulse billing state from the resulting real `customer.subscription.updated` webhook.

Observed:

- Original subscription preserved: `yes`
- Legacy price ID preserved: `yes`
- `plan_version`: `v5_pro_monthly_grandfathered`

### `CCR-4` Completed Cancellation

1. Re-scheduled cancellation on the same monthly legacy subscription.
2. Advanced the Stripe test clock beyond `current_period_end` and let Stripe deliver the resulting deletion webhook to the local hosted-mode Pulse runtime.

Observed:

- Webhook/event IDs: `evt_1TAIrgPZ0VLEY1aVGB9fCDv5`, `evt_1TAJ3dPZ0VLEY1aVCnQpuvYn`, `evt_1TAJ6HPZ0VLEY1aVmo6HEUis`
- Post-cancel entitlement state: `canceled`
- Paid capabilities revoked: `yes`
- Continuity notice removed: `yes`

### `CCR-5` Post-Cancel Repurchase

1. Starting from a fully canceled monthly state, created a new v6 monthly purchase path for the same returning Stripe customer.
2. Hosted Stripe Checkout itself was flaky in headless mode on the repurchase step, so the open checkout session was expired and replaced with the Stripe subscription-creation API equivalent using the same sandbox customer and saved payment method.
3. Forced a real `customer.subscription.updated` webhook on the new subscription and refreshed Pulse entitlements plus the settings surface.

Observed:

- New subscription ID: `sub_1TAIuOPZ0VLEY1aVj1M6IKdQ`
- New price ID: `price_1TAIjKPZ0VLEY1aV2WLdZbID`
- New plan key / `plan_version`: `pro`
- Grandfathered notice absent on new subscription: `yes`

### `CCR-6` Annual Parity Spot Check

1. Starting from a confirmed canceled state on the same mapped Stripe customer, created a new annual legacy recurring subscription with `plan_version=v5_pro_annual_grandfathered` and drove a real `customer.subscription.updated` webhook into Pulse.
2. Scheduled cancellation, advanced the Stripe test clock beyond the annual period boundary, confirmed Pulse revocation, then created a new v6 annual subscription with `plan_version=pro` and delivered a real `customer.subscription.updated` webhook.

Observed:

- Annual continuity preserved while active: `yes`
- Annual cancellation revokes access: `yes`
- Annual re-entry uses v6 pricing: `yes`

### `CCR-7` Direct Legacy Checkout Rejection

1. Executed the `pulse-pro/license-server` checkout contract proof for `TestHandleCheckoutSessionCreate_RejectsGrandfatheredPlanKey`.
2. Confirmed the handler rejects a grandfathered/v5 `plan_key` before any Stripe checkout session is created.

Observed:

- HTTP status: `400`
- Error body: `contains "not a v6 checkout plan"`
- Stripe checkout created: `no`

## Outcome

- `pass`
- Summary:
  - Active monthly and annual grandfathered recurring subscriptions preserved their legacy price identity while active.
  - Completed cancellation removed paid access and removed the grandfathered continuity notice from the Pulse settings surface.
  - Post-cancel return flows landed on current public v6 monthly and annual prices with `plan_version=pro`, not `v5_pro_*_grandfathered`.

## Evidence Captured

- Stripe subscription snapshots: `/Volumes/Development/pulse/tmp/commercial-cancellation-reactivation-20260312/stripe/`
- Entitlement payload snapshots: `/Volumes/Development/pulse/tmp/commercial-cancellation-reactivation-20260312/stripe/`
- Settings screenshots: `/Volumes/Development/pulse/tmp/commercial-cancellation-reactivation-20260312/screenshots/`
- Checkout request/response logs: `annual-checkout-session.json`, `annual-checkout-session-expired.json`, `monthly-repurchase-checkout-session.json`, `monthly-repurchase-checkout-session-expired.json`
- Webhook event IDs: `evt_1TAIrgPZ0VLEY1aVGB9fCDv5`, `evt_1TAIxzPZ0VLEY1aV84jIeI7F`, `evt_1TAJ3dPZ0VLEY1aVCnQpuvYn`, `evt_1TAJ5GPZ0VLEY1aVVyDywj4k`, `evt_1TAJ6HPZ0VLEY1aVmo6HEUis`, `evt_1TAJ7OPZ0VLEY1aVKY0lwmOI`

## Follow-Ups

- Run the same journey once through a human-operated browser on the staging checkout and Stripe customer-portal UI before GA to remove the headless-browser limitation from the evidence set.
