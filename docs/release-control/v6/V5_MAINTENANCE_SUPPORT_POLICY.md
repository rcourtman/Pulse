# Pulse v5 Maintenance-Only Support Policy

This document is the canonical support-policy decision for the Pulse v6 GA
cutover.

## Trigger

1. This policy activates on the calendar date that `v6.0.0` first ships on
   the `stable` channel.
2. That publication date is the authoritative v6 GA date for support-policy
   purposes.
3. Before the GA release is published, the release notes must include the
   exact v6 GA date and the exact v5 end-of-support date in `YYYY-MM-DD` form.

## Support Window

1. Pulse v5 enters maintenance-only support immediately on the v6 GA date.
2. The maintenance-only support window lasts 90 calendar days from the v6 GA
   date.
3. The published v5 end-of-support date is authoritative and must match that
   90-day window.

## Eligible v5 Fixes

Only issues that materially threaten existing deployments or paying-customer
continuity qualify for v5 maintenance work:

1. Critical security issues.
2. Critical correctness or data-loss issues.
3. Installer, startup, or updater failures that prevent normal operation.
4. Licensing or billing blockers that wrongly break an existing paying
   customer.
5. Safe migration blockers that prevent customers from reaching a supported v6
   path.

## Out Of Scope For v5

These do not qualify as v5 maintenance work:

1. New features or integrations.
2. Routine bug-fix backports.
3. UI polish, refactors, or parity work with v6.
4. Pricing-model or entitlement-model exceptions created to avoid the v6
   model.

## Release-Line Rules

1. Cut `pulse/v5-maintenance` from the last supported v5 stable point at the
   v6 GA cutover.
2. Ship approved v5 maintenance releases from `pulse/v5-maintenance` only.
3. Keep `main` and the active v6 line focused on v6 and later.
4. Fix on the active v6 line first when practical, then backport the smallest
   safe change to v5 only when the issue qualifies under this policy.

## End Of Support

1. After the published v5 end-of-support date, Pulse v5 is unsupported.
2. After that date, new fixes land only on v6 and later unless I explicitly
   announce an exception.
3. The GA release notice is required to publish the exact v5 end-of-support
   date so customers can plan upgrades before the window closes.

## Required GA Release Notice

The first stable `v6.0.0` release must publish this meaning, with placeholders
replaced by exact dates:

> Pulse v5 entered maintenance-only support on [v6-ga-date]. I will ship only
> critical security, data-loss, licensing or billing blocker, installer or
> updater failure, and safe migration blocker fixes for existing v5 users until
> [v5-eos-date]. After [v5-eos-date], Pulse v5 is end-of-support and new fixes
> land on v6 unless I publish an explicit exception.
