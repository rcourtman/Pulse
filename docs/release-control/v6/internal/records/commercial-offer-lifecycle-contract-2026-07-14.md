# Commercial Offer And Lifecycle Contract

Date: 2026-07-14
Owner: project owner
Status: approved for implementation

## Decision

Pulse commercial packaging and lifecycle behavior must converge on one
versioned offer contract. The public site, Pulse Account, in-product plan
surfaces, checkout projection, Stripe catalog audit, license-server billing
state, runtime entitlements, and support policy must not maintain independent
commercial definitions.

## Self-Hosted Offer

1. Community is the free local monitoring foundation with 7-day history.
2. Relay is the secure remote-access, Pulse Mobile pairing, push-delivery, and
   14-day-history service.
3. Pro is the Patrol-powered investigation and governed-operations product
   with 90-day history and team/admin controls. Pro includes Relay; a buyer
   never needs simultaneous Relay and Pro subscriptions for one environment.
4. Community, Relay, and Pro solve distinct jobs. They must not be presented
   as a recommended good/better/best ladder.
5. Ordinary self-hosted trial acquisition remains retired.

## License Scope And Support

1. One Relay or Pro subscription covers one owner-operated Pulse environment.
2. Monitored-system and child-resource volume is not metered for self-hosted
   Community, Relay, or Pro.
3. A paid subscription permits three concurrent activations inside that one
   environment for primary, migration, and recovery use. It does not license
   independently operated client environments; that is the MSP product job.
4. Verified administrative ownership transfer is permitted. Resale, sharing,
   and unverified third-party assignment are prohibited.
5. Relay and Pro include standard verified commercial support for billing,
   activation, transfer, configuration, and diagnostics. The public target is
   typically within two business days, without a contractual SLA or a
   priority-support promise.

## Subscription Transition Matrix

| From | To | Effective time | Billing treatment | Entitlement treatment |
|---|---|---|---|---|
| Community | Relay or Pro | After successful checkout | New subscription | Grant the purchased plan atomically |
| Relay | Pro, same cadence | Immediate after explicit quote and successful prorated payment | Prorated upgrade | Pro plus bundled Relay |
| Monthly | Annual, same tier | Immediate after explicit quote and successful prorated payment | Prorated cadence change | Tier unchanged |
| Pro | Relay | Current paid period end | No proration | Remove Pro-only capabilities; retain Relay |
| Annual | Monthly, same tier | Current paid period end | No proration | Tier unchanged |
| Any paid plan | Community by cancellation | Current paid period end | No proration | Paid capabilities end at the paid-through timestamp |

Combined transitions follow the most restrictive rule. Any transition that
reduces capabilities or moves annual to monthly takes effect at renewal.
Customer-visible confirmation must state the effective date and quoted charge
or credit before mutation.

## Cancellation, Grace, And Continuity

1. Voluntary cancellation is scheduled for period end. Paid capabilities end
   at the paid-through timestamp.
2. A seven-day post-term recovery window may support subscription reactivation,
   license retrieval, and account recovery, but it is not a paid-entitlement
   extension and must not grant Relay or Pro capabilities.
3. Involuntary payment failure receives a separate seven-day functional grace
   period. Existing paid capabilities remain available during that grace, then
   fail closed if payment has not recovered.
4. Reversing scheduled cancellation before the paid-through timestamp
   preserves subscription continuity.
5. Grandfathered recurring price continuity survives only while the original
   recurring subscription remains continuous. Completed cancellation or a
   completed tier/cadence change exits the grandfathered contract; later
   re-entry uses current pricing.
6. A full refund or dispute revokes the affected paid entitlement and requires
   explicit reactivation or repurchase.

## Downgrade Preservation

1. Configuration, report definitions, and audit records are not deleted solely
   because a customer downgrades.
2. Pro-only configuration remains stored but inert while the entitlement is
   absent.
3. History and generated artifacts outside the lower tier's active window are
   soft-hidden for 30 days and become purge-eligible after 60 days.
4. Re-upgrade during the soft-hide window restores access without
   reconfiguration.
5. Every capability, tier, cadence, restrictive state, or entitlement change
   increments `license_version` and invalidates obsolete grants atomically.

## Cloud And MSP Availability

1. Cloud is unavailable. Its historical prices, caps, trial, and support labels
   are dormant proposals, not a current customer offer. Cloud cannot reopen
   until card policy, economic unit/caps, support, retention, export,
   cancellation, reactivation, and runtime enforcement pass a governed
   readiness gate.
2. MSP is an assisted preview and provider-hosted by default. Starter, Growth,
   and Scale retain 5, 15, and 40 isolated client-workspace limits at the
   recorded monthly and annual prices. Enterprise remains custom.
3. Public MSP copy may publish the recorded monthly and annual prices, but it
   must not promise immediate self-service checkout or Pulse-hosted fulfillment.
4. Pulse-hosted MSP remains an optional assisted arrangement, not the default
   public delivery model.

## Implementation Boundary

Stripe remains billing truth, but one Pulse-owned transition authority must
apply the authoritative Stripe snapshot atomically to billing contract,
entitlement projection, continuity epoch, transition history, license version,
and grant-revocation outbox state. Checkout, webhook, reconciliation, refunds,
support/admin actions, and future Pulse Account transitions must converge on
that authority. Stripe Customer Portal subscription updates remain disabled;
Pulse-owned plan transitions must not depend on mutable portal configuration.

## Proof Required Before Closure

1. Contract drift proof across public pricing, Pulse Account, in-product plan
   presentation, Stripe product/price descriptions, support policy, and runtime
   entitlements.
2. Complete Community/Relay/Pro and monthly/annual transition matrix proof.
3. Stripe test-mode payment, proration, schedule, cancellation, payment-failure,
   refund, and re-entry proof.
4. Duplicate, reversed, missing, and replayed webhook convergence proof.
5. Restrictive-transition grant version-floor proof in Pulse and Relay.
6. Downgrade soft-hide, restoration, purge-eligibility, and configuration
   preservation proof.
7. Desktop and phone-width buyer/account browser proof.
8. A read-only production Stripe catalog and portal expected-state audit before
   any separately approved external configuration change.

## Implemented Local Evidence

The 2026-07-14 implementation slice now provides the local, non-transactional
foundation required by this contract:

1. `pulse-pro/license-server/v6_commercial_projection.go` owns atomic catalog,
   billing-contract, entitlement, continuity-epoch, license-version, transition
   history, and revocation-outbox projection. Unknown or multi-price snapshots
   fail closed, and checkout, subscription, invoice, cancellation, payment-
   failure, and refund paths converge on that projection.
2. `pulse-pro/license-server/v6_commercial_transitions.go` owns authenticated,
   durable transition quotes. Immediate expansion uses the exact quoted Stripe
   proration timestamp with `pending_if_incomplete`; restrictive changes use a
   renewal-bound subscription schedule without proration. Retry paths reuse
   Stripe idempotency keys and already-created schedules.
3. `pulse-pro/landing-page/manage.html` exposes invoices/payment methods,
   quoted Relay/Pro and cadence changes, period-end cancellation, reactivation,
   and scheduled-change cancellation behind an emailed verification code.
4. `pkg/licensing/subscription_transitions.go`, `pkg/metrics/store.go`, and
   `internal/api/report_schedules.go` persist downgrade timing, keep protected
   configuration and report definitions, restrict out-of-tier access
   immediately, delay physical history cleanup until day 60, block background
   report execution without the current entitlement, and purge generated
   report artifacts without following symlinks after eligibility.
5. Local proof is green for the complete license-server Go suite, 359 Python
   pricing/copy tests, the Pulse licensing/metrics/API suites, and desktop plus
   390-pixel buyer/account browser exercise. These are local implementation and
   rehearsal facts, not a substitute for the required Stripe test-mode and
   production read-only evidence.
6. `pulse-pro/relay-server` now requires the operator revocation authority,
   synchronously drains it before serving, reports stale feed state through
   readiness, disconnects already-connected v6 sessions below a newly applied
   license-version floor, and clears their persisted reconnect credentials.
   The joined Relay proof covers feed consumption through active-session
   teardown. License-server startup also fails closed when v6 is enabled
   without the matching feed credential.

## Remaining Readiness Boundary

The `self-hosted-commercial-transition-coherence` gate remains blocked. No
Stripe test-mode mutation, live catalog/configuration change, purchase,
deployment, or production customer-data access was authorized or performed in
this slice. Before self-service is released, the project still needs the
governed external transition matrix, event-order/reconciliation exercise,
read-only production catalog/portal audit, and a customer-safe Pulse runtime
invalidation path named above. The global operator feed credential must never
be distributed to customer Pulse installations; Pulse convergence requires an
installation-scoped authenticated feed or an equivalently bounded canonical
authority. The local Relay proof raises confidence but does not replace the
required real external Stripe-to-Relay exercise.
