# Pulse v6 Release Promotion Policy

This document defines how Pulse v6 and later releases move from development to
customer-facing availability. It is the release-trust contract for Pulse Pro,
Cloud, and self-hosted production users.

## Goals

1. Stable customers must not become the first test cohort for new changes.
2. Development speed must stay decoupled from customer exposure.
3. Every broad rollout must have explicit validation and rollback rules.

## Channel Contract

1. `stable`
   - Default for new installs.
   - The only recommended channel for paid and production environments.
   - Publishes only non-prerelease tags.
   - The only channel eligible for unattended broad rollout.
2. `rc`
   - Opt-in preview channel for internal use, staging-like environments, and
     explicitly willing preview users.
   - Publishes prerelease tags such as `6.0.0-rc.1`.
   - Must never be the default channel.
   - In v6, `rc` affects manual and in-app update selection; unattended
     systemd auto-updates remain `stable`-only.
3. Source builds
   - Are not a customer-facing release channel.
   - Remain reserved for development, debugging, and branch validation.

## Development Model

1. Use short-lived feature branches and feature flags for incomplete or risky
   work.
2. Do not move directly from "issue fixed" to "all customers updated".
3. Channel promotion is the primary customer-safety boundary.
4. Branch topology may change over time; the `stable` versus `rc` customer
   contract must not.

## RC Rules

1. Every candidate intended for broad customer use must ship to `rc` before it
   is eligible for `stable`.
2. Each RC must have:
   - Targeted automated checks for touched release surfaces.
   - A smoke install on a fresh or staging-like environment.
   - Release notes and a rollback target recorded before publish.
   - At least one live run of the release pipeline for the RC tag itself, not
     only structural workflow validation.
3. Failed RCs are fixed forward and replaced with a new RC. They are never
   promoted as-is to `stable`.

## v5 Maintenance Policy

1. When Pulse v6 reaches `stable`, Pulse v5 immediately enters
   maintenance-only support.
2. The maintenance-only window lasts 90 calendar days from the v6 GA or stable
   release date.
3. During that window, v5 fixes are limited to:
   - critical security issues
   - critical correctness or data-loss issues
   - migration blockers that prevent customers from reaching a safe v6 path
4. v5 will not receive:
   - new features
   - normal bug-fix backports
   - pricing-model exceptions
   - entitlement-model parity work introduced for v6
5. After the 90-day window ends, v5 may continue running for users who choose
   to stay on it, but it is unsupported.
6. The v6 GA announcement must publish the exact v5 end-of-support date
   calculated from the GA publication date.

## Stable Promotion Rules

1. A stable tag must be promoted from a commit that has already been exercised
   as an RC.
2. For v6 GA, do not promote to `stable` until the active control-plane target
   satisfies its `release_ready` completion rule.
3. Every stable promotion requires:
   - Applicable items in `PRE_RELEASE_CHECKLIST.md` complete.
   - Applicable entries in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` cleared.
   - No known unresolved high-severity regressions in touched release surfaces.
   - The previous stable rollback target and exact reinstall command recorded.
   - A live release-pipeline exercise already completed for the promoted RC,
     not only YAML lint or static workflow validation.
   - The locked 90-day v5 maintenance-only policy and exact end-of-support
     notice ready to publish with the promotion.
4. Normal stable promotions require a minimum 72-hour RC soak after the
   candidate is available to internal or staging-like users.
5. Hotfix exception:
   - A shorter soak is allowed only for narrowly scoped fixes to active
     customer harm.
   - The exception and rollback target must be recorded in the release notes or
     release ticket before promotion.

## Rollout Rules

1. Default installs stay on `stable`.
2. Broad customer announcements and unattended updates target `stable` only.
3. `rc` enrollment must be explicit and reversible.
4. Paid production tenants should remain on `stable` unless they are knowingly
   participating in preview validation.

## Rollback Rules

1. Never delete or rewrite shipped tags to hide a bad release; supersede them
   with a newer release and explicit guidance.
2. If an RC is bad, hold it in `rc`, fix forward, and cut the next RC. Do not
   promote it.
3. If a stable release is bad:
   - Pause further promotion or auto-update exposure.
   - Direct affected users to the prior stable pin.
   - Cut and validate a hotfix or rollback release.
4. The previous stable version must remain installable by exact version pin
   until the replacement stable release is trusted.

## Required Release Artifacts

1. Release notes.
2. Rollback target version and exact pin command.
3. Checklist evidence and gate status.
4. Staging or internal validation note.
5. v5 maintenance-only support policy and end-of-support note for the GA cutover.

## Authority

If conflicts appear:

1. `SOURCE_OF_TRUTH.md` owns the locked decision that this policy is mandatory.
2. `status.json` owns whether the decision is open or resolved and whether the
   active target is release-ready.
3. `PRE_RELEASE_CHECKLIST.md` and
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` own execution proof for a
   specific promotion.
