# Paid Runtime Build Attribution Alerting

Date: 2026-05-07
Owner: paid-runtime-build-attribution-alerting
Evidence tier: managed-runtime-exercise

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

## Managed Runtime Exercise - 2026-05-08

The remaining managed-runtime proof was exercised against a local v6 license
server and real Pulse runtimes instead of browser-stubbed paid payloads:

- local v6 license server on `127.0.0.1:19080` with an isolated throwaway
  Ed25519 test key and `canary_public_v6_plans.json`;
- public community runtime built from `cmd/pulse`, launched with isolated
  `PULSE_DATA_DIR=/tmp/pulse-community-runtime-proof` and activated with a
  local test Pro license for `runtime-proof-community@test.local`;
- private Pro runtime built from `repos/pulse-enterprise/cmd/pulse-enterprise`,
  launched with isolated `PULSE_DATA_DIR=/tmp/pulse-pro-runtime-proof` and
  activated with a separate local test Pro license for
  `runtime-proof-pro@test.local`;
- the real frontend at `http://127.0.0.1:5173/settings/system/billing/plan`
  proxying to those runtimes for browser-visible Plans proof.

Observed results:

- community runtime entitlement payload reported `tier=pro`,
  `subscription_state=active`, and `runtime.build=community`;
- community runtime `/api/license/runtime-capabilities` reported
  `runtime.build=community` and blocked `agent_profiles`, `ai_alerts`,
  `ai_autofix`, `audit_logging`, `kubernetes_ai`, and `rbac` with
  `paid_runtime_required`;
- Plans showed `Current plan: Pulse Pro`, `Pro runtime missing`, the
  `running the community runtime` warning, and the `Open Pulse Pro downloads`
  handoff;
- license-server admin/customer support JSON recorded the community install with
  `runtime_build=community`, `runtime_status=community`, and per-license
  `installations_runtime={community:1, pro:0, unknown:0}`;
- Pro runtime entitlement and runtime-capability payloads reported
  `runtime.build=pro`, no blocked capabilities, and the Plans value proof showed
  `Pulse Pro runtime` as `Active`;
- license-server admin/customer support JSON recorded the Pro install with
  `runtime_build=pro`, `runtime_status=pro`, and per-license
  `installations_runtime={community:0, pro:1, unknown:0}`.

Proof commands used in the managed exercise:

- `GOCACHE=/Volumes/Development/pulse/repos/pulse/tmp/go-build-cache go build -buildvcs=false -o /tmp/pulse-community-runtime-proof/pulse-community ./cmd/pulse`
- `GOCACHE=/Volumes/Development/pulse/repos/pulse/tmp/go-build-cache go build -buildvcs=false -o /tmp/pulse-pro-runtime-proof/pulse-pro ./cmd/pulse-enterprise` from `repos/pulse-enterprise`
- `curl -fsS -b /tmp/pulse-runtime-proof-community-cookies.txt http://127.0.0.1:7655/api/license/runtime-capabilities`
- `curl -fsS -b /tmp/pulse-runtime-proof-pro-cookies.txt http://127.0.0.1:7655/api/license/runtime-capabilities`
- `curl -fsS 'http://127.0.0.1:19080/v1/admin/customer?email=runtime-proof-community@test.local' -H 'X-API-Token: <local-admin-token>'`
- `curl -fsS 'http://127.0.0.1:19080/v1/admin/customer?email=runtime-proof-pro@test.local' -H 'X-API-Token: <local-admin-token>'`
- Browser DOM proof against `http://127.0.0.1:5173/settings/system/billing/plan`
  for both the community-runtime warning state and the clean Pro-runtime state.

## Result

The release-ready gate is passed. Paid Pro activation can no longer silently look
healthy on a community runtime: the runtime contract reaches Pulse, Pro-only
runtime features are blocked at the runtime-capability boundary, Plans gives the
operator a private-runtime warning and download handoff, and license-server
support telemetry distinguishes community, Pro, and unknown runtime states.
