# Self-Hosted Commercial Transition Production Audit Blocked Record

- Date: `2026-07-14`
- Gate: `self-hosted-commercial-transition-coherence`
- Result: `blocked`
- Proof posture: read-only production observation

## Scope And Safety

The production license runtime and Stripe commercial state were inspected
without changing application files, deployment configuration, Stripe objects,
subscriptions, or customer records. The Stripe audit performed only HTTP GET
requests. Secret values remained inside the production host process and were
not printed, copied into the workspace, or recorded in this evidence.

The executed proof used:

1. `bash ./scripts/validate_license_runtime_config.sh`
2. the committed `pulse-pro:scripts/validate_stripe_catalog.py` streamed to the
   production license host through the repo-pinned SSH helper

The SSH helper first required a canonical trust fix: DNS resolution returned
the Tailscale FQDN while the repository intentionally pins the logical
`pulse-license` host identity. `pulse-pro` commit `d5f73b1` adds the explicit
`HostKeyAlias=pulse-license` binding while retaining batch mode, strict host-key
checking, the checked-in ED25519 pin, and no global known-host fallback.

## Observed Facts

1. Production runtime-file validation passed. The remote secrets file,
   entrypoint, and binary were present and syntactically valid, and the
   entrypoint's validation-only path exited successfully.
2. The production Stripe audit resolved all 25 governed price objects.
3. The production runtime environment did not provide a valid
   `STRIPE_BILLING_PORTAL_CONFIGURATION_ID`. The auditor therefore did not
   retrieve or certify a billing portal configuration.
4. Both public Relay prices still resolve to a Stripe product description that
   describes remote access, mobile, push, and a custom URL but does not state
   the governed one-owner-operated-environment boundary.
5. Both public Pro prices still resolve to a Stripe product description that
   describes the earlier AI/auto-fix, history, RBAC, audit, and SAML framing but
   does not state the current Patrol job, the one-owner-operated-environment
   boundary, or that Pro includes Relay and Pulse Mobile.
6. The two legacy v1 recurring prices remain inactive. The auditor reported
   these as the already-governed non-blocking legacy warnings.
7. No production customer, invoice, subscription, payment method, or license
   record was read during this proof.

## Inference

The successful validation-only startup alongside the missing portal
configuration indicates that the deployed entrypoint/config contract has not
yet converged on the repo-owned fail-closed portal requirement. This is an
inference from observed behavior; the remote file contents were not copied or
inspected.

## Blocking Verdict

The read-only production audit is now exercised, but it failed expected state.
The gate must remain blocked. Passing local tests and resolving all price IDs do
not compensate for a missing governed portal configuration or public Stripe
product descriptions that contradict the canonical offer.

## Required Unblock Steps

1. Under a separately approved deployment/configuration change, deploy the
   fail-closed runtime contract and configure the explicit governed Stripe
   Customer Portal configuration.
2. Under a separately approved Stripe catalog change, align the public Relay
   and Pro product descriptions with the canonical offer and Pro bundle.
3. Re-run the same GET-only production audit and require zero blocking errors.
4. Separately complete the Stripe test-mode transition/event-reconciliation
   matrix and the real Relay/license-version-floor exercise. This production
   catalog observation does not replace either end-to-end proof.
