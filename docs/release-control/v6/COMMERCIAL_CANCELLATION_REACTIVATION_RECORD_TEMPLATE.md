# Commercial Cancellation/Reactivation Record Template

Use this template when exercising the `commercial-cancellation-reactivation`
release gate.

Save executed records under:

`docs/release-control/v6/records/commercial-cancellation-reactivation-YYYY-MM-DD.md`

---

# Commercial Cancellation/Reactivation Record

- Date: `YYYY-MM-DD`
- Gate: `commercial-cancellation-reactivation`
- Assertions:
  - `RA2`
  - `RA4`
  - `RA7`
- Environment:
  - Billing environment: `...`
  - Pulse runtime URL: `...`
  - pulse-pro checkout origin: `...`
  - Stripe mode: `test` or `staging-equivalent`
  - Operator: `...`

## Fixtures

- Monthly grandfathered customer:
  - Email: `...`
  - Stripe customer ID: `...`
  - Stripe subscription ID: `...`
  - Legacy price ID: `...`
- Annual grandfathered customer:
  - Email: `...`
  - Stripe customer ID: `...`
  - Stripe subscription ID: `...`
  - Legacy price ID: `...`
- Returning post-cancel customer:
  - Email: `...`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestStripeWebhook_SubscriptionDeleted_RevokesCapabilities' -count=1`
- `go test ./tests/migration -run 'TestV5FullUpgradeScenario/PersistedV5RecurringLicenseAutoExchanges' -count=1`
- `npm --prefix frontend-modern test -- src/utils/__tests__/licensePresentation.test.ts src/components/Settings/__tests__/ProLicensePanel.test.tsx`
- `cd /Volumes/Development/pulse/repos/pulse-pro/license-server && go test . -run 'TestHandleCheckoutSessionCreate(_RejectsGrandfatheredPlanKey)?$' -count=1`
- Result: `pass` or `fail`

## Manual Exercise

### `CCR-1` Active Grandfathered Continuity Baseline

1. `...`
2. `...`
3. `...`

Observed:

- Stripe price ID: `...`
- `GET /api/license/entitlements` `plan_version`: `...`
- Settings surface continuity notice: `present` or `absent`

### `CCR-2` Cancel At Period End Without Immediate Drift

1. `...`
2. `...`

Observed:

- Cancel-at-period-end state: `...`
- Legacy price ID still attached: `yes` or `no`
- Entitlement state before period end: `...`

### `CCR-3` Resume Before Lapse

1. `...`
2. `...`

Observed:

- Original subscription preserved: `yes` or `no`
- Legacy price ID preserved: `yes` or `no`
- `plan_version`: `...`

### `CCR-4` Completed Cancellation

1. `...`
2. `...`

Observed:

- Webhook/event IDs: `...`
- Post-cancel entitlement state: `...`
- Paid capabilities revoked: `yes` or `no`
- Continuity notice removed: `yes` or `no`

### `CCR-5` Post-Cancel Repurchase

1. `...`
2. `...`
3. `...`

Observed:

- New subscription ID: `...`
- New price ID: `...`
- New plan key / `plan_version`: `...`
- Grandfathered notice absent on new subscription: `yes` or `no`

### `CCR-6` Annual Parity Spot Check

1. `...`
2. `...`

Observed:

- Annual continuity preserved while active: `yes` or `no`
- Annual cancellation revokes access: `yes` or `no`
- Annual re-entry uses v6 pricing: `yes` or `no`

### `CCR-7` Direct Legacy Checkout Rejection

1. `...`
2. `...`

Observed:

- HTTP status: `...`
- Error body: `...`
- Stripe checkout created: `yes` or `no`

## Outcome

- `pass` or `fail`
- Summary:
  - `...`
  - `...`
  - `...`

## Evidence Captured

- Stripe subscription snapshots: `...`
- Entitlement payload snapshots: `...`
- Settings screenshots: `...`
- Checkout request/response logs: `...`
- Webhook event IDs: `...`

## Follow-Ups

- `none`, or:
  - `...`
  - `...`
