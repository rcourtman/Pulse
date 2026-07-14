# Self-Hosted Commercial Stripe Remediation Prepared Record

- Date: `2026-07-14`
- Gate: `self-hosted-commercial-transition-coherence`
- Result: `repository preparation complete; production remains blocked`
- Evidence tier: `test-proof`

## Fact

The repository now contains a bounded Stripe commercial-state remediation tool
at `pulse-pro:scripts/remediate_stripe_commercial_state.py`. Its default mode
performs only GET requests and emits a machine-readable plan. Apply mode
requires the exact `APPLY_STRIPE_COMMERCIAL_STATE` confirmation and a Stripe
key whose live/test mode matches the operator's declared expected mode.

The tool derives the public Relay and Pro price IDs from the public pricing
model, validates their amount, cadence, activity, product identity, shared-
product shape, and Stripe mode, then plans at most these operations:

1. align the descriptions of the existing public Pulse Relay and Pulse Pro
   products with the canonical offer;
2. reuse an exact Pulse-owned invoice/payment-method-only Customer Portal
   configuration, or create a new dedicated one if none exists.

It does not contain an apply path for prices, customers, subscriptions,
webhooks, licenses, or runtime configuration. A nonconforming or unrelated
portal configuration is not modified. The dormant new-environment price
creation helper now carries the same canonical Relay and Pro descriptions.

## Offline Proof

`pulse-pro:scripts/tests/test_remediate_stripe_commercial_state.py` proves:

- monthly and annual prices must share one product per tier;
- Relay and Pro must use distinct products;
- catalog or mode drift fails before mutation;
- shared product updates are deduplicated;
- only an exact dedicated portal configuration is reused;
- a nonconforming portal causes safe dedicated creation rather than mutation;
- portal lifecycle mutation features are disabled;
- apply requires the exact confirmation token; and
- the bounded apply path posts only product descriptions and, when required,
  a dedicated portal configuration.

`pulse-pro:scripts/tests/test_create_v6_stripe_prices.py` proves the dormant
creation helper carries the governed descriptions and still excludes Pro+ from
public creation.

## Non-Claim And Remaining Gate

No Stripe mutation, customer-data access, runtime configuration change,
deployment, or production re-audit was performed in this slice. This is not
production proof and does not raise the gate above `test-proof`. The production
blocking record remains authoritative until a separately approved apply and
deployment are followed by a passing GET-only audit. The external Stripe
transition/reconciliation matrix and Relay license-version-floor exercise also
remain required.
