# Pulse v6 Commercial Cancellation/Reactivation E2E Test Plan

Use this plan for the trust-critical commercial boundary between:

1. grandfathered v5 recurring subscription continuity while the subscription is still active
2. paid-state revocation once cancellation or lapse is complete
3. current-public-v6 re-entry after a canceled customer returns

This is a companion drill for `L2`, `L3`, `L11`, and `L12`.
It does not replace repo-local tests. It exists because checkout, billing
portal actions, Stripe webhook state, v6 entitlement evaluation, and customer
UI can all pass in isolation while the real cancellation/reactivation journey
still drifts.

## Governing Policy

The locked v6 pricing rule is:

1. Active recurring Pulse Pro v5 subscribers keep their legacy recurring price while subscription continuity is maintained.
2. Cancellation or lapse is the explicit boundary for that grandfathering.
3. Once cancellation is complete, any later return must enter on current public v6 pricing.
4. The prior grandfathered recurring price must not resume automatically after that break in continuity.

## Scope

In scope:

1. Self-hosted recurring Pulse Pro v5 monthly and annual grandfathered plans.
2. Stripe customer-portal cancellation and resume behavior.
3. Stripe webhook propagation into Pulse v6 billing-state persistence.
4. Pulse v6 entitlement and settings-surface behavior after cancellation state changes.
5. Public checkout re-entry after completed cancellation.

Out of scope:

1. Lifetime licenses.
2. MSP or hosted cloud plan conversion.
3. Manual business-exception pricing overrides.

## Runtime Surfaces

`pulse`:

1. `internal/api/payments_webhook_handlers.go`
2. `internal/api/stripe_webhook_handlers_test.go`
3. `pkg/licensing/...`
4. `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
5. `tests/migration/v5_full_upgrade_test.go`

`pulse-pro`:

1. `license-server/v6_checkout.go`
2. public checkout entrypoint `/v1/checkout/session`
3. Stripe customer portal / `https://pulserelay.pro/manage`
4. Stripe recurring price configuration and webhook delivery

## Automated Proof Bundle

Run these before the manual drill:

`pulse`

1. `go test ./internal/api -run 'TestStripeWebhook_SubscriptionDeleted_RevokesCapabilities' -count=1`
2. `go test ./tests/migration -run 'TestV5FullUpgradeScenario/PersistedV5RecurringLicenseAutoExchanges' -count=1`
3. `npm --prefix frontend-modern test -- src/utils/__tests__/licensePresentation.test.ts src/components/Settings/__tests__/ProLicensePanel.test.tsx`

`pulse-pro/license-server`

1. `go test . -run 'TestHandleCheckoutSessionCreate(_RejectsGrandfatheredPlanKey)?$' -count=1`

If any of those fail, stop. The manual drill should not be used to compensate
for a broken automated floor.

## Environment And Fixtures

Use a staging-like environment with:

1. Stripe test mode or an equivalent non-production billing environment.
2. Working customer portal access.
3. Pulse v6 runtime connected to the same billing/webhook environment.
4. Checkout surface configured with current public v6 price IDs only.
5. Legacy recurring v5 price IDs still present for renewal compatibility.

Seed at least these fixtures outside git:

1. `customer_a_monthly`: migrated active v5 monthly recurring subscriber on `v5_pro_monthly_grandfathered`
2. `customer_b_annual`: migrated active v5 annual recurring subscriber on `v5_pro_annual_grandfathered`
3. `returner_email`: a churned customer identity with no active recurring subscription at test start

Record outside git:

1. Stripe customer IDs
2. subscription IDs
3. exact Stripe price IDs
4. Pulse license IDs and activation IDs where applicable
5. environment URL and execution date

## Scenario Matrix

| ID | Scenario | Primary fixture | Pass focus |
|---|---|---|---|
| `CCR-1` | Active grandfathered continuity baseline | `customer_a_monthly` | Legacy price and v5 plan identity are still intact while active |
| `CCR-2` | Cancel at period end without immediate drift | `customer_a_monthly` | Cancellation intent does not rewrite pricing or entitlements early |
| `CCR-3` | Resume before lapse | `customer_a_monthly` | Same active subscription keeps the same legacy price |
| `CCR-4` | Completed cancellation | `customer_a_monthly` | Paid access is revoked and historical v5 plan identity remains visible as history, not access |
| `CCR-5` | Post-cancel repurchase | `customer_a_monthly` or `returner_email` | Re-entry uses current public v6 pricing, not a revived legacy rate |
| `CCR-6` | Annual parity spot check | `customer_b_annual` | Annual grandfathered path follows the same continuity and re-entry rules |
| `CCR-7` | Direct legacy checkout rejection | synthetic request | Public checkout rejects grandfathered/v5 plan keys before Stripe session creation |

## Execution Steps

### `CCR-1` Active Grandfathered Continuity Baseline

1. Start with an already-migrated active v5 monthly subscriber.
2. Confirm Stripe shows the legacy recurring price ID, not a v6 retail price ID.
3. Open Pulse v6 settings and capture the Pro license panel.
4. Call `GET /api/license/entitlements`.

Pass when:

1. `plan_version` is `v5_pro_monthly_grandfathered`.
2. Entitlements are active.
3. The settings panel shows the grandfathered continuity notice.
4. No checkout or upsell surface claims the customer has already moved to a v6 retail recurring price.

### `CCR-2` Cancel At Period End Without Immediate Drift

1. Use the customer portal to schedule cancellation at period end.
2. Confirm Stripe marks the subscription for cancellation without replacing the legacy recurring price.
3. Refresh Pulse entitlements and the settings surface before the current period ends.

Pass when:

1. The same subscription still carries the legacy recurring price ID.
2. The customer remains entitled until the billing period actually ends.
3. The UI still communicates continuity while the subscription is active.
4. No new v6 subscription object is created just because cancellation was scheduled.

### `CCR-3` Resume Before Lapse

1. Before the current period ends, undo the scheduled cancellation from the portal or equivalent Stripe action.
2. Refresh billing state in Pulse.

Pass when:

1. The original subscription remains the billing object of record.
2. The legacy recurring price ID is unchanged.
3. `plan_version` remains the same grandfathered v5 recurring key.
4. There is no forced checkout or new-subscription re-entry path.

### `CCR-4` Completed Cancellation

1. Let the scheduled cancellation complete naturally or drive the equivalent Stripe test-clock/webhook path.
2. Confirm the cancellation webhook is delivered successfully.
3. Refresh Pulse entitlements and the Pro license settings surface.

Pass when:

1. Paid capabilities are revoked after cancellation is complete.
2. Billing state no longer grants active paid access.
3. Historical plan identity still shows the prior grandfathered plan version for continuity/audit purposes where expected.
4. The recurring-price continuity notice is no longer shown for a canceled or expired state.

### `CCR-5` Post-Cancel Repurchase

1. Starting from a fully canceled/lapsed state, use the public checkout flow as the same returning customer.
2. Complete a new purchase through the public v6 checkout path.
3. Capture the resulting Stripe subscription and Pulse license/entitlement state.

Pass when:

1. The new subscription uses a current public v6 price ID.
2. The new purchase does not reuse the legacy recurring price ID.
3. The resulting Pulse plan/license state resolves to a v6 plan, not `v5_pro_*_grandfathered`.
4. The settings surface does not show a grandfathered v5 continuity notice on the new subscription.

### `CCR-6` Annual Parity Spot Check

Run `CCR-1`, `CCR-4`, and `CCR-5` on the annual grandfathered fixture.

Pass when:

1. Annual grandfathering preserves continuity while active.
2. Annual cancellation revokes paid access after completion.
3. Annual return flow still re-enters on current public v6 pricing.

### `CCR-7` Direct Legacy Checkout Rejection

1. Submit a direct request to `pulse-pro/license-server` checkout creation with a legacy/grandfathered `plan_key`.
2. Confirm the request is rejected before any Stripe checkout session is created.

Pass when:

1. The endpoint returns a client error.
2. The error states the plan is not a v6 checkout plan.
3. No Stripe checkout session is created.

## Evidence To Capture

Capture all of the following outside git or in a dated release-control record:

1. Stripe subscription snapshots before cancellation, after scheduling cancellation, after completion, and after repurchase
2. `GET /api/license/entitlements` payloads for the same checkpoints
3. Pro license settings screenshots for active, scheduled-cancel, canceled, and repurchased states
4. The old and new subscription IDs plus their price IDs
5. The resulting Pulse plan key / `plan_version` at each checkpoint
6. Checkout-response payload or request log proving public re-entry used a v6 plan
7. Any webhook event IDs used to advance the cancellation state

## Failure Rules

Block release or rollout if any of these are observed:

1. An active grandfathered subscriber is silently rewritten to a v6 retail recurring plan without an actual cancel-and-rebuy boundary.
2. A canceled subscriber can repurchase onto a legacy recurring price through public checkout.
3. Cancellation intent causes early entitlement revocation before the paid period actually ends.
4. Completed cancellation leaves paid capabilities active.
5. Resume-before-lapse creates a new subscription or loses the legacy recurring price unexpectedly.
6. Monthly and annual grandfathered paths do not behave the same way on the continuity boundary.

## Recording Results

When this drill is executed for a candidate RC or release:

1. Write a dated record under `docs/release-control/v6/records/`.
2. Link that record from the relevant release ticket.
3. If the exercise materially changes confidence for a release gate, update the matching `status.json.release_gates[*]` entry in the same slice.
