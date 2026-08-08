# Historical Credential Containment — Open

Recorded: 2026-08-08.

This record contains redacted identity labels and repository metadata only. It
contains no credential values, fragments, prefixes, fingerprints, derived
values, request material, or authentication results.

## Current finding

Historical credential containment is not complete and remains a prerelease
blocker across `pulse` and `pulse-pro`.

The public Pulse inventory has ten stable redacted identities:

- `PBS-01`
- `PBS-02`
- `PULSE-01`
- `PULSE-02`
- `PULSE-03`
- `PULSE-04`
- `PXM-01`
- `PXM-02`
- `SLACK-01`
- `TELEGRAM-01`

Those identities remain reachable through 244 affected tag refs, with 225
attached GitHub Releases. No provider-side death evidence is recorded for any
of them.

The private Pulse Pro containment scope uses five redacted role identities:

- `PRO-CLOUDFLARE-01`
- `PRO-DIGITALOCEAN-01`
- `PRO-LICENSE-ADMIN-01`
- `PRO-RESEND-01`
- `PRO-STRIPE-WEBHOOK-01`

At the reviewed Pulse Pro runbook revision
`cf30c40a8ba96fca363b41af1bda85a64554f16a`, the current Resend and
license-admin roles still matched reachable history. Historical DigitalOcean,
Stripe webhook, and Cloudflare revocation remained unproven.

## Closure rule

The gate stays blocking until every redacted identity has both:

1. authoritative provider or owning-control-plane evidence that the historical
   identity was revoked, expired, decommissioned, or is absent from a fresh
   inventory; and
2. verified replacement deployment evidence, or verified retirement evidence
   when the consumer no longer exists.

A narrative approval, a different current value, a file or secret update
timestamp, repository-secret metadata, or a successful request with a current
credential cannot satisfy either typed record set by itself.

History rewriting is a separate, optional, destructive follow-up after
containment. It is not a prerequisite for provider containment and cannot be
used as a substitute for it.
