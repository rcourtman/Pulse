# Self-Hosted Commercial Surfaces Revision Record

- Date: `2026-08-07`
- Assertion: `RA5`
- Lane: `L2`
- Implementation status: `landed on main after v6.2.0-rc.9`
- Approval status: `not verified`
- Governance result: `blocked pending project-owner decision`
- Supersedes only if approved: `records/self-hosted-paid-services-opt-in-surface-2026-04-25.md`

## Governance Correction

The original record labelled this revision `Richard-approved` and treated an
undocumented telemetry read as material decision evidence. The repository and
git history contain no owner-confirmation artifact, query, snapshot, report URL,
or source identifier for that label or the exact figures. The label is removed.

The 2026-08-08 evidence audit reconstructed nearby aggregate telemetry results,
but it did not reproduce the exact population counts, the monthly subscription
claim, or a longitudinal conversion measure. The reconstructed percentages are
paid-license share in the latest weekly ping cohort, not conversion. See
`self-hosted-commercial-surfaces-evidence-audit-2026-08-08.md`.

This correction does not revert product code. It records the landed behavior
honestly and blocks release readiness until the project owner chooses a product
posture explicitly.

## Quantitative Evidence

- Source: `self-hosted-commercial-surfaces-evidence-audit-2026-08-08.md` (forensic reconstruction, exact rationale not reproduced)
- Query: `self-hosted-commercial-surfaces-evidence-audit-2026-08-08.md#sanitized-reconstruction-query`
- Snapshot: `self-hosted-commercial-surfaces-evidence-audit-2026-08-08.md#sanitized-snapshot`
- Measured at: `2026-08-08T01:47:00Z`

## Implemented Posture Under Review

Ordinary free self-hosted Pulse v6 sessions currently see reactive commercial
surfaces by default. Paid-feature navigation items are visible with panel-owned
inline gates, gate sections render their upgrade call-to-action, and the Plans &
Billing page is discoverable. One proactive surface exists: a one-shot
business-estate card shown only to authenticated sessions of free installs whose
monitored estate crosses the current thresholds of at least 5 PVE nodes, 10
Docker hosts, or 3 VMware hosts. The card is permanently dismissible in a single
interaction.

Commercial suppression remains absolute for demo mode and white-label runtimes,
which covers MSP tenant containers through their chained white-label
entitlement. Kiosk sessions inherit suppression through the existing kiosk mount
gate. Monitoring surfaces, including dashboards, infrastructure pages, alerts,
incident flows, onboarding, and notification channels, carry no commercial
content. Trial ceremony and hosted handoff flows remain absent.

## Evidence Status

The earlier quantitative rationale is not decision-grade evidence:

- A reconstruction at the landing commit cutoff found 9,426 non-development
  weekly active install IDs, not 9,421.
- The same reconstruction found 672 threshold-classified estates, not 671.
- Paid-license share was 7.8869% for the threshold cohort and 0.9824% for the
  smaller cohort. This is a current-state association, not a conversion rate.
- No reproducible source was found for the earlier claim about new paid
  subscriptions per month.
- The `business_estate` stored field first received production rows after the
  original record and was not backfilled, so it cannot be the original source.
- No durable evidence confirms project-owner approval of the reversal.

The historical April decision remains the last verified product direction until
the open owner decision is resolved. The current main implementation differs
from that direction and must not be promoted as approved merely because its
runtime tests pass.

## Current Product Boundary

The following describes the landed code and is not an approval record:

- Core self-hosted monitoring stays free and uncapped.
- Reactive feature-gate calls to action, paid-feature navigation with inline
  gates, and Plans & Billing are visible to ordinary self-hosted sessions.
- The one-shot business-estate card never shows in demo, white-label, kiosk,
  hosted, or paid sessions. It never shows on the first qualifying day, never
  appears before the GitHub star prompt has been handled, and never appears
  again after any dismissal action.
- The business-estate signal is served only on the authenticated security
  status response at `sessionCapabilities.businessEstate`, never on the
  pre-auth presentation policy.
- Demo mode and white-label runtimes force both `hideCommercial` and
  `hideUpgrade`.
- `multi_tenant` organization navigation stays hidden without the feature.
- No trial flows, hosted handoff prompts, or commercial content appear in
  monitoring or notification surfaces.

## Owner Decision Required

The project owner must choose one of these outcomes before the next RC or stable
promotion:

1. **Retain the whole commercial cluster.** Approve the implemented reactive
   navigation, inline gates, discoverable Plans & Billing, one-shot
   business-estate card, schema-v8 classification field, and checkout-source
   attribution. The evidence audit may be used only as aggregate current-state
   context. It does not establish conversion causality.
2. **Revert the whole commercial cluster.** Restore the 2026-04-25 opt-in
   posture, remove the card and its business-estate signal, return paid-feature
   navigation and calls to action to the earlier suppression boundary, remove
   schema-v8 `business_estate`, and remove checkout-source attribution from both
   repositories. The implementation must preserve later unrelated authorization
   and settings fixes while reverting the cluster.
3. **Direct a new narrower posture.** Specify the exact retained and removed
   surfaces. This is a new product decision and does not count as approval of
   either whole-cluster option.

### Whole-cluster commit set

Public `pulse` commits:

- `f0e2243b44b7e40eb6c3af624c4f9337cdf2f4d4`: UI gates, navigation,
  business-estate card, session capability, RA5 revision, and browser proof.
- `6a36b5f8ef34a87d613ff113712be758c7b9ceff`: revised settings navigation test
  pin.
- `5b07bdc3d8eab45ee004c3c71f22fd2f4d318371`: schema-v8 telemetry payload,
  business-estate classifier, privacy copy, and server plumbing.
- `d5bc3e3862fd25f520afe9ecf38b3272f6cd5d93`: in-app checkout-source
  attribution and client handoff contract.
- `06d8bfe65ebf3683a07d9ec7775a50a9872418ce`: held release-note copy.

Private `pulse-pro` dependency commits:

- `884eb4c6147a7ccb7a1c254cb5a9ca57173d7fd0`: stores schema-v8
  `business_estate` pings without backfill.
- `34c0a2b7eee071c2e633efd0edad91ecdc313c07`: accepts, stores, and forwards
  checkout-source attribution.
- `3360c640df62c7125b869ac6a99b7a2e49b73966`: records attribution and deploy
  ordering.

Later settings and authorization fixes overlap files first changed by
`f0e2243b4`. A revert must be conflict-aware and must not blanket-revert those
later fixes.

Later overlapping commits to preserve while implementing any product revert:

- `755a8887874e21d0115e1490b9e69e9501d2a31e`: non-admin settings endpoint
  polling.
- `14a82e7684390c9fe0a4bd900417c4adbb7140b8`: authorization refusal logging.
- `6a958761e84b64dca7052caad855144e57880690`: admin-only System tab
  visibility.
- `27b4d98bc0528a359dc71b819a1b79115354eed8`: viewer workload navigation.
- `6d8a50937672b13fb98f025701a885f3361363e1`: viewer display settings.
- `7a0f410508822afed26c23cd3e715fbeaae3c420`: admin-only panel gates.
- `98ecfb3f10143cdb7b317cb7ab410ae5b2852ab2`: update-status route authority.
- `541c9be7fd764acedb29844506681992faba3a8b`: oversized WebSocket recovery.

## Runtime Proof

These checks prove the landed implementation shape. They do not prove owner
approval or the commercial rationale:

- `go test ./internal/api ./pkg/licensing -count=1`
- `npm --prefix frontend-modern run type-check`
- The RA5 frontend proof suite pinned in the landing commit.
- `tests/integration/tests/58-self-hosted-trial-rate-limit-ui.spec.ts`
- `frontend-modern/browser-verification.json` in the landing commit.

## Held Release Note Copy

Do not paste this copy into a release packet until the owner decision is
resolved. It describes the currently landed implementation only.

### Changed

- Free self-hosted installs show the paid feature pages in Settings with an
  inline explanation on each one, and Plans & Billing is reachable from
  Settings. Monitoring pages, alerts, onboarding, and notification channels
  carry no commercial content.
- An install whose monitored estate reaches the current thresholds may see one
  card asking whether Pulse is running at work. It never appears on the first
  qualifying day, and every button dismisses it permanently.
- Demo mode, kiosk sessions, white-label runtimes, and MSP tenant containers
  continue to show no commercial content.

### Telemetry

- The usage telemetry payload moves to schema 8 and adds `business_estate`, a
  Boolean derived from resource counts already present in the payload. No new
  infrastructure detail is collected. Telemetry remains opt-out through
  `PULSE_TELEMETRY=false` or the Settings toggle.
- A checkout started from inside Pulse records a closed-vocabulary source for
  the surface that started it. It is sent only when the user starts a purchase.

### Release ordering

The private license server decodes the checkout handoff strictly. It must be
deployed before any Pulse build that sends the attribution field, or in-app
upgrade clicks fail closed with the Pulse Account unavailable page. The
constraint is recorded in `pulse-pro` `OPERATIONS.md` under
`Stripe / Checkout attribution`.
