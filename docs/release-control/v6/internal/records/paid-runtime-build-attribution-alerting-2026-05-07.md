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

## Implementation Slice - 2026-05-07

The first product and support guardrail slice now exists:

- active paid self-hosted plans treat any non-Pro or missing runtime identity as
  a private-runtime mismatch;
- the Plans surface renders `Pro runtime missing` plus the private Pulse Pro
  download handoff when a paid install reports community runtime or no runtime;
- activation-success and value-proof copy use the same runtime identity check,
  so a freshly accepted paid key cannot present as fully healthy without the
  private runtime;
- license-server installation responses preserve raw `runtime_build` and add
  normalized `runtime_status` values: `pro`, `community`, or `unknown`;
- admin/customer support JSON and the admin UI expose runtime status per
  installation, plus a per-license runtime-status rollup for support triage.

Local proof added:

- `go test ./internal/api -run 'TestHandleRuntimeCapabilities_CommunityRuntimeBlocksPrivateProCapabilities|TestHandleRuntimeCapabilities_ProRuntimePreservesPrivateProCapabilities'`
- `go test ./...` from `repos/pulse-pro/license-server`
- `npm test -- --run src/utils/__tests__/licensePresentation.test.ts src/components/Settings/__tests__/ProLicensePanel.test.tsx`
- `npm run type-check` from `repos/pulse/frontend-modern`
- `PULSE_BASE_URL=http://127.0.0.1:5173 npx playwright test tests/59-self-hosted-plans-entitlement-summary.spec.ts --project=chromium` from `repos/pulse/tests/integration`
- `GOCACHE=/Volumes/Development/pulse/repos/pulse/tmp/go-build-cache PULSE_E2E_USE_HOT_DEV=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_E2E_HOT_DEV_TAKEOVER=1 npm --prefix tests/integration test -- tests/59-self-hosted-plans-entitlement-summary.spec.ts --project=chromium -g "warns when an active Pro plan reports the public community runtime"` from `repos/pulse`

The Playwright proof exercises the real hot-dev browser surface with a paid Pro
payload that reports the Pro runtime and a negative paid Pro payload with the
runtime identity omitted. The additional browser-stubbed negative proof now also
covers the explicit public community runtime response, including the
private-runtime warning and download handoff in Plans.

## Remaining Proof Before Passing

The release gate remains pending until the negative path is exercised against a
managed runtime rather than only browser-stubbed paid payloads. The remaining
proof should run a valid paid activation against the public community runtime
and confirm:

- Pulse reports `runtime.build=community` in local entitlement/runtime payloads;
- Pro-only runtime capabilities are blocked with `paid_runtime_required`;
- Plans displays the private-runtime warning and download handoff;
- license-server support/admin telemetry records `runtime_status=community`;
- the private Pulse Pro runtime path still reports `runtime_status=pro`.

Direct Linux/LXC coverage should use the same contract with the private archive
and public community archive before this gate is marked `passed`.

## Result

This record creates a new release-ready gate. It should remain pending until the
product-visible detection, telemetry, support view, and proof path exist.
