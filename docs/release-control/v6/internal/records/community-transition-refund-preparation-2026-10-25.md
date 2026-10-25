# Community transition refund preparation

25 October 2026. Commercial preparation under L2 and L3. This slice adds an
admin-only calculation of an annual transition refund from proposed announcement
and release dates. It does not activate a programme, change the published offer,
remove runtime gates or authorize payment actions.

## Prepared boundary

`pulse-pro:license-server/v6_commercial_refunds.go` binds the requested
subscription and invoice to a configured annual Pulse price, one complete annual
line, its paid payment intent and exact captured charge. Customer, subscription,
price, invoice, payment and currency must agree throughout. Missing, ambiguous,
prorated, disputed or contradictory evidence stops the preview. A release
outside the selected paid period requires reconciliation.

Amounts use actual captured minor units rather than list prices or converted
currency. The calculation requires at least 30 days between proposed notice and
release, and reports the 60-day claim deadline. Its version identifies the
calculation policy, not an activated customer programme. Both current annual
prices and grandfathered recurring annual prices work without a v6 licence row.
The preview is explicitly unexecutable.

Prior refunds produce only a cash-bounded ceiling and a reconciliation status.
The preview cannot infer whether an earlier refund was a transition remedy,
goodwill or a price adjustment. An invoice balance adjustment similarly
withholds an additional-refund recommendation. The separate recent-first-
purchase remedy is explicitly not evaluated, and monthly payments are out of
scope. Unsupported cohorts have private 422 reasons, distinct from unavailable
provider evidence.

All provider operations are GETs. No subscription, refund, local licence,
commercial projection, revocation event or customer email is changed. Existing
full-refund and support-cancellation paths retain their current semantics.

## Remaining implementation

Keep the public free-feature change out of the automatic release train until
the notice and effective release are coordinated. Community history, remote
probes and ordinary accounts must not regain removed restrictions through old
grants or lapse. Preserve configured physical retention and existing data.
Pro retains supported investigation/actions, custom roles and reporting at
protected prices. The proposed Business checkout pause must be coherent with
public copy and preserve original rights and support promises.

Before offering the annual exit, extend the existing commercial transition
authority with a durable quote bound to the adopted programme, exact invoice
and charge, and the applicable local licence generations. Re-read financial
evidence before execution. A known pre-mutation failure differs from an
uncertain financial outcome. Do not reclaim or automatically retry uncertain
cancellation or refund work.

Confirmed cancellation must terminate paid rights through the canonical local
projection and version-floor path, including matching legacy entitlements.
Partial refund webhooks alone do not perform that termination. Prove event
order, replay, renewal between preview and execution, lack of unintended
credits/invoices, and customer-runtime convergence before activation. Full
recent-first-purchase remedies remain distinct from unused annual time.

These are active residuals in `community-commercial-transition-cutover`.
This record does not close the external commercial transition proof or claim
that the free-feature release or complete refund executor has shipped.

## Verification

The standalone license-server Go suite includes
`v6_commercial_refunds_test.go` for exact-payment admission, grandfathered and
current annual prices, currency preservation, partial/full prior refunds,
integer rounding, malformed evidence, interrupted reads, admin-only admission
and absence of a public route. The source is prepared independently of any
production deployment or provider test-mode mutation.
