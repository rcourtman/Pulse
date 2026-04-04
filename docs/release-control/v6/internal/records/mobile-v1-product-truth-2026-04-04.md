# Mobile v1 Product Truth Record

- Date: `2026-04-04`
- Decision: `mobile-v1-product-truth`
- Scope:
  - `pulse-mobile`
  - `pulse-pro`
  - `pulse`
  - lanes `L5`, `L7`, `L8`

## Decision

Pulse Mobile v1 is the away-from-desk decision surface for Pulse.
It is not a desktop-parity monitoring dashboard squeezed onto a phone.

The primary mobile v1 jobs are:

1. Show what needs operator attention now across paired Pulse instances.
2. Let the operator open and clear findings, pending approvals, and reconnect
   issues away from the desk.
3. Keep instance switching, pairing recovery, and trust posture legible on the
   phone.
4. Hand investigation into Pulse AI without making chat the primary app shell.

The mobile v1 non-goals are:

1. Desktop-feature parity.
2. Deep fleet browsing or broad dashboard recreation on the phone.
3. Marketing the phone app as the place to do all monitoring work.

The shell and public contract follow from that narrower purpose:

1. The primary shell should read as triage-first: `Now`, `Findings`,
   `Instances`, `Approvals`, `Settings`.
2. `Chat` remains contextual and task-aware rather than a primary tab in the
   steady-state shell.
3. Store and landing-page copy must describe away-from-desk triage, approvals,
   reconnect state, and secure remote access rather than promising generic
   real-time monitoring parity.

## Evidence Considered

1. `pulse-mobile/app/(tabs)/index.tsx` already centers the shell around
   `Needs You Now`, current state, quick actions, and safe follow-up rather
   than broad dashboard parity.
2. `pulse-mobile/src/components/dashboard/homeOverviewState.ts`,
   `pulse-mobile/src/components/dashboard/dashboardQuickActionsState.ts`, and
   `pulse-mobile/src/components/dashboard/homeFleetCoverageState.ts` already
   model priority, approvals, findings, reconnect state, and instance
   switching as the primary mobile jobs.
3. `pulse-mobile/store/store.config.json` and `pulse-mobile/store/listing.md`
   had been overpromising phone-first infrastructure monitoring parity instead
   of the narrower v1 job the runtime currently supports.
4. `pulse-pro/landing-page/mobile.html` is the correct public surface to keep
   aligned with the governed mobile v1 purpose.

## Outcome

- Mobile v1 product truth is now explicit.
- The governed mobile shell and public/store copy should align to that truth.
- Public release clearance remains governed separately by
  `pulse-mobile/store/release-readiness.json`; this decision does not lower the
  iOS proof gates that still block general availability.
