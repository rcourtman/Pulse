# Business tier launch — 2026-08-20

## Decision

The project owner (Richard Courtman) approved launching the dormant
self-hosted `business` tier for public checkout on 2026-08-20, in an
interactive Claude Code session, with these dials:

- **Price**: $399/year, annual billing only. No monthly business price is
  created or sold.
- **Seats**: Business carries unlimited seats (`max_users` unset), per the
  cloud-paid Extension Point 26 shape. Newly issued Pro subscriptions carry
  `max_users = 3` via new Stripe price ids; every previously issued license
  keeps its original unlimited seat posture because the existing public Pro
  price ids and their plan entries are left untouched (the commercial
  projection reconciler syncs licenses to their plan entry, so the old
  entries must never gain a seat cap).
- **History**: 365-day retention, as already fixed in the tier contract.
- **Support**: next-business-day target, no hard SLA (solo-maintainer
  honesty; Pro/Relay stay at "typically two business days").
- **Features**: identical to Pro, per Extension Point 26 — Business never
  differentiates by gating features.

## Scope — what this does NOT reintroduce

This launch does not reopen any part of the 2026-08-07 commercial-surfaces
cluster reverted under `self-hosted-commercial-surfaces-opt-in-posture`
(resolved 2026-08-08): the opt-in presentation posture stands, the
business-estate card stays removed, the `business_estate` telemetry stays
retired (pinned by `TestTelemetryPing_IgnoresRetiredBusinessEstateField` in
pulse-pro), and no new proactive in-product surface is added. Business
appears only in the public pricing model payload (landing page and any
surface that already renders that payload) and through public checkout.

## Evidence

The 2026-08-07 quantitative rationale was re-baselined on 2026-08-18
against real actives after excluding the phantom-install cohort: 682
business-scale estates (>=5 PVE nodes, >=10 Docker hosts, or >=3 VMware
hosts) active in 7 days, 7.77% paid, versus 1.25% for small estates — a
6.2x conversion multiple; 629 business estates unconverted, including 13
estates with >=20 PVE nodes, all unpaid. Price anchors: Datadog
$180/host/yr, Checkmk ~$1k+/yr entry, PRTG ~$1.9k, versus Pulse's prior
$79/yr ceiling. Purchase attribution is measurable from launch day via the
`v6_checkout_sessions` create ledger (pulse-pro `fb0b180`) and
`checkout_origin` metadata.

## Mechanics

- Stripe: product `prod_V6juOLRVRNBXnf` (Pulse Business), price
  `price_1U6WQuBrHBocJIGH0ThY3XAc` ($399/year). New seat-capped Pro prices
  `price_1U6WQvBrHBocJIGHxMjt6t41` ($79/year) and
  `price_1U6WQvBrHBocJIGHYqXOjFWm` ($8.99/month) on the existing Pro
  product; the prior public Pro price ids stay active for existing
  subscriptions with `public_checkout` flipped off.
- License server: `business` added to `isPublicV6SelfHostedCheckoutTier`,
  annual-only requirement in the public pricing model, Business card and
  seat/grandfathering copy in the pricing payload, dormancy pins flipped
  with reference to this record.
- The runtime side (`TierBusiness` in `pkg/licensing/features.go`) was
  already shipped dormant and honors `business` grants without change.
