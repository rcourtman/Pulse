# Self-Hosted Commercial Production Remediation Record

- Date: `2026-07-15`
- Gate: `self-hosted-commercial-transition-coherence`
- Result: `production catalog and portal residual passed`
- Evidence tier: `production-observed`

## Approved Scope

The user approved the exact production plan after a GET-only preview. The
bounded apply was limited to:

1. aligning the existing public Pulse Relay and Pulse Pro Stripe product
   descriptions with the canonical offer;
2. creating a dedicated Customer Portal configuration for invoice history and
   payment-method updates only; and
3. deploying that configuration identifier to the license runtime before a
   read-only re-audit.

No price, customer, subscription, webhook, or license object was changed. No
customer, subscription, webhook, or license record was read.

## Stripe Result

The guarded `pulse-pro:scripts/remediate_stripe_commercial_state.py` apply
converged these existing products:

- `prod_U37aaHIl0MtpS5` (`Pulse Relay`): `Secure remote access and Pulse Mobile
  connectivity for one owner-operated Pulse environment.`
- `prod_U37aCdMDY0V6cK` (`Pulse Pro`): `Patrol-powered operations for one
  owner-operated Pulse environment, including Relay connectivity and Pulse
  Mobile.`

Stripe created dedicated portal configuration
`bpc_1TtNsxBrHBocJIGHTjRvG4Qf`, named `Pulse self-hosted invoices and payment
methods`. It enables `invoice_history` and `payment_method_update`. It disables
`customer_update`, `subscription_cancel`, `subscription_update`, and the hosted
`login_page`.

## Runtime Deployment And Recovery

The successful deployment backed up the production environment to
`/etc/pulse-license/secrets.env.bak.stripe-portal.20260715T081138Z`, added the
portal identifier exactly once as `STRIPE_BILLING_PORTAL_CONFIGURATION_ID`, and
restarted `pulse-license`. Loopback and public `/healthz` returned
`{"status":"ok"}`.

An earlier atomic-writer attempt created
`/etc/pulse-license/secrets.env.bak.stripe-portal.20260715T081025Z` but failed
its key-count postcondition after writing escaped line separators. The command
stopped before service restart, so the malformed configuration was never
loaded. The file was restored from that backup, its original 73-line structure
and shell syntax were verified, and service health was confirmed before the
successful deployment. The first public probe during the intentional recovery
restart returned a transient `502`; loopback and public health passed after the
service became ready.

## Read-Only Postconditions

The post-deploy validator reported:

- `ok: true`;
- `checked_prices: 25`;
- `portal_configuration_checked: true`;
- zero errors;
- the same four governed public Relay and Pro checkout price IDs; and
- only the two already-governed inactive v1 legacy recurring warnings.

A second default remediation plan returned `product_updates: []` and portal
action `reuse` for `bpc_1TtNsxBrHBocJIGHTjRvG4Qf`. The production license
runtime configuration validator also passed.

## Gate Verdict

The production catalog and portal failure recorded on `2026-07-14` is closed.
The release gate remains blocked: a passing production catalog observation does
not replace the governed external Stripe lifecycle transition/event-
reconciliation matrix or the real Relay license-version-floor exercise.
