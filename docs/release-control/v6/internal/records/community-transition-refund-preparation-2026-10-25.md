# Community transition refund preparation

25 October 2026. Commercial preparation under L2 and L3. The preview and
subsequent annual quote/apply preparation do not adopt programme dates, deploy
the service, change the published offer, remove runtime gates or authorize
production payment actions.

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

The preview makes provider GETs only and remains unexecutable. Its proposed
dates cannot activate the separate annual exit authority.

The annual exit extends the existing durable commercial quote authority.
Original payment and policy terms bind real legacy/v6 targets through complete
invoice and checkout chains. Only refund exits may omit a primary v6 licence;
normal quote constraints and existing records are preserved by migration.
The programme is absent by default and has no adopted dates.

Permanent local rights with conflicting recurring billing links require
manual reconciliation. Unrelated permanent purchases are preserved.

Cancellation and refund each have a persisted, compare-and-set intent before
their provider request. No lease reclaim resends an uncertain financial call.
An authoritative canceled read, matching legacy termination, canonical v6
projection/version-floor outbox and confirmed progress commit together before
refund intent. Removing current programme configuration after cancellation
does not discard that saved obligation. Pending refunds remain pending.

All application subscription/refund writers share the active-exit fence.
Read-only provider reconciliation remains available without an active
programme. An uncertain cancellation that did not execute can release its
local fence only through an explicit, audited operator attestation of provider
evidence that the request cannot complete. An active read or elapsed time is
insufficient. No financial request is repeated by that resolution.

## Individual requests

Refunds are on request; there is no blanket payout or automatic cancellation.
Publish clear terms and a simple request route so customers need not argue a
complaint. Every new executable annual exit requires a recorded explicit
request, verified canonical support case, exact subscription binding and
required audit. An operator-attested external request creates or references a
canonical general case, retaining opaque evidence references and its original
receipt time without backdating case creation. A generic support contact does
not count as consent.

Requests received from announcement through 60 days after the effective date
are timely. Processing after that deadline does not invalidate an on-time
request. The programme and effective date still govern first execution. Request
withdrawal and persisted cancellation intent are mutually exclusive; once a
request has been dispatched, recovery follows its saved financial obligation.
Existing unrequested quotes cannot start cancellation, while already-dispatched
obligations remain recoverable after upgrade or privacy deletion.

For an eligible first-time buyer in the 30 days before announcement who asks
for the remedy, the agreed amount is every payment on that new subscription in
full, including monthly renewals up to cancellation. This amount rule still
needs its separate original-payment/history proof and implementation; the
annual-only executor does not provide that remedy yet.

## Remaining implementation

Keep the public free-feature change out of the automatic release train until
the notice and effective release are coordinated. Community history, remote
probes and ordinary accounts must not regain removed restrictions through old
grants or lapse. Preserve configured physical retention and existing data.
Pro retains supported investigation/actions, custom roles and reporting at
protected prices. The proposed Business checkout pause must be coherent with
public copy and preserve original rights and support promises.

Before offering the annual exit, adopt the programme and complete explicit
prior-refund/credit, renewal-period and failed/absent-refund reconciliation.
The prepared authority refuses ambiguous cases. A manual replacement needs
provider evidence, exact payment binding and a retained purpose; it must not
turn into an automatic financial retry. Outstanding customer invoices and
pending invoice items require reconciliation because subscription cancellation
can affect customer-wide collection. Keep the recent-first-purchase remedy
separate and bind its exact original payment, rather than reuse email/newest-
payment selection.

Confirmed cancellation must terminate paid rights through the canonical local
projection and version-floor path, including matching legacy entitlements.
Partial refund webhooks alone do not perform that termination. Prove event
order, replay, renewal between preview and execution, lack of unintended
credits/invoices, and customer-runtime convergence before activation. Full
recent-first-purchase remedies remain distinct from unused annual time.

These are active residuals in `community-commercial-transition-cutover`.

Confirmed provider cancellation with an unconfirmed local projection is a
separate manual recovery case. Reconcile changed/missing local rights before
recording cancellation confirmed or authorizing payment; the negative
cancellation resolution cannot apply to an already canceled subscription.
This record does not close the external commercial transition proof or claim
that the free-feature release or complete customer programme has shipped.

## Verification

The standalone license-server Go suite includes
`v6_commercial_refunds_test.go` for exact-payment admission, grandfathered and
current annual prices, currency preservation, partial/full prior refunds,
integer rounding, malformed evidence, interrupted reads, admin-only admission
and absence of a public route. `v6_commercial_refund_exits_test.go` covers
legacy/v6 termination with unrelated purchases preserved, uncertain/pending
provider outcomes, concurrent confirmations, atomic rollback, stale evidence,
operator-attested negative resolution, late partial-refund events, existing
writer fencing and upgrade preservation of ordinary customer quotes.
`v6_commercial_refund_claims_test.go` adds request-only admission, explicit
instruction, canonical v5/v6 identity, verified external intake, atomic
case/request/audit persistence, original receipt deadlines, withdrawal/intent
exclusion, changed ownership/case evidence, privacy deletion and recovery of
already-dispatched historical obligations. The source is prepared independently of production deployment and provider
test-mode mutation.
