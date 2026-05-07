# Paid Runtime Build Attribution Alerting

Date: 2026-05-07
Owner: paid-runtime-build-attribution-alerting
Evidence tier: local-rehearsal

## Decision

Paid-license validity and paid-runtime availability are separate product facts.
Pulse must not silently treat an active Pro license on a public community runtime
as a normal paid install.

For v6 RC5/GA readiness, a paid license that checks in from a non-Pro or unknown
runtime must create an operator-visible support signal:

- the local Pulse Plans surface should warn that the license is active but the
  install is running the public community build when runtime identity says so;
- license-server install telemetry should preserve `runtime_build`, version, and
  deployment type for support triage;
- the admin/support view should distinguish Pro runtime, community runtime, and
  unknown runtime for paid licenses;
- optional outbound email should be delayed and conservative, so a fresh install
  or one transient unknown check-in does not create noise.

This is a support and UX guardrail, not the security boundary for paid features.
The root problem is preventing normal paid users from unknowingly running the
wrong build after activation.

## Current Containment

The RC4 customer Docker path has been published and tested through the private
Pulse Pro image/download broker lane. Customer emails and support guidance now
point paid v6 users to `https://pulserelay.pro/download.html` instead of public
GitHub or public Docker builds for paid-runtime features.

That closes the immediate Oscar/Michael support containment, but it does not
prove that future paid customers will be automatically warned if they install or
continue running the community build with a valid paid key.

## Required Proof Before Passing

- A runtime/build identity contract reaches the local Plans/licensing surface.
- A paid-license plus non-Pro/unknown runtime state renders a clear in-app
  warning and private download-page handoff.
- License-server telemetry/admin support surfaces record and expose the runtime
  classification for paid check-ins.
- Proof covers Docker and direct Linux/LXC paths, including the private Pro
  runtime and the public community runtime negative case.
- Alerting is rate-limited or delayed enough to avoid noisy false positives.

## Result

This record creates a new release-ready gate. It should remain pending until the
product-visible detection, telemetry, support view, and proof path exist.
