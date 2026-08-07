# Self-Hosted Commercial Surfaces Revision Record

- Date: `2026-08-07`
- Assertion: `RA5`
- Lane: `interactive (Claude Code, Richard-approved)`
- Result: `pass`
- Supersedes: `records/self-hosted-paid-services-opt-in-surface-2026-04-25.md`

## Decision

Ordinary free self-hosted Pulse v6 sessions see reactive commercial surfaces
by default: paid-feature navigation items are visible with panel-owned inline
gates, gate sections render their upgrade call-to-action, and the Plans &
Billing page is discoverable. One proactive surface exists — a one-shot
business-estate card shown only to authenticated sessions of free installs
whose monitored estate crosses business-scale thresholds (>=5 PVE nodes,
>=10 Docker hosts, or >=3 VMware hosts), permanently dismissible in a single
interaction.

Commercial suppression remains absolute for demo mode and white-label
runtimes (which covers MSP tenant containers via their chained white-label
entitlement), and kiosk sessions inherit suppression through the existing
kiosk mount gate. Monitoring surfaces — dashboards, infrastructure pages,
alerts, incident flows, onboarding, and notification channels — carry no
commercial content. Trial ceremony and hosted handoff flows remain absent.

## Why the April decision is superseded

The 2026-04-25 record optimized for a clean community-first GA, before any
post-GA evidence existed. Measured on 2026-08-07 (production telemetry,
mock-fleet rows excluded): weekly active installs grew from ~330 pre-GA to
9,421, while new paid subscriptions stayed flat (~30-40/month) because no
commercial surface was reachable — `hideUpgrade` was forced on for every
non-hosted session, making upgrade CTAs dead code for the entire self-hosted
fleet, and paid-only navigation hiding made deliberate reach impossible.
671 business-scale estates (which convert at 7.9% vs 0.98% for smaller
installs) were active in the trailing 7 days with no in-product path to any
paid tier. The April record's own boundary ("commercial surfaces remain
available when the user deliberately reaches for them") is better served by
visible-but-gated navigation than by hiding, which concealed the existence
of the capabilities users would deliberately reach for.

## Product Boundary (revised)

- Core self-hosted monitoring stays free and uncapped.
- Reactive surfaces (feature-gate CTAs, paid-feature navigation with inline
  gates, Plans & Billing) are visible to ordinary self-hosted sessions.
- Exactly one proactive commercial surface exists: the one-shot
  business-estate card. It never shows in demo, white-label, kiosk, hosted,
  or paid sessions; never on the first qualifying day; never before the
  GitHub star prompt has been interacted with; and never again after any
  dismissal action.
- The business-estate signal is served only on the authenticated security
  status response (`sessionCapabilities.businessEstate`), never on the
  pre-auth presentation policy, so estate size cannot leak to anonymous
  visitors.
- Demo mode and white-label runtimes hide all commercial surfaces
  (`hideCommercial` and `hideUpgrade` both forced).
- `multi_tenant` organization navigation stays hidden without the feature.
- No trial flows, no hosted handoff prompts, no commercial content in
  monitoring or notification surfaces.

## Proof

- `go test ./internal/api ./pkg/licensing -count=1` — pass (includes new
  `TestContract_SecurityStatusPresentationPolicyShowsUpgradeByDefault`,
  `TestContract_SecurityStatusPresentationPolicySuppressionInputs`,
  `TestContract_BusinessScaleEstateThresholds`; demo-mode suppression pins
  unchanged and passing).
- `npm --prefix frontend-modern run type-check` — pass.
- RA5 frontend proof suite (`npx vitest run` over the pinned files) — pass
  after honest re-pinning of `settingsNavigation.integration.test.tsx` to
  the revised navigation invariant.
- `tests/integration/tests/58-self-hosted-trial-rate-limit-ui.spec.ts`
  re-pinned: paid-only navigation and inline gates visible, trial ceremony
  and hosted handoff still absent.
- Live browser verification recorded in
  `frontend-modern/browser-verification.json` in the landing commit.
