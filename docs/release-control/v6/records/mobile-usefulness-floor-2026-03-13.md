# Mobile Usefulness Floor Record

- Date: `2026-03-13`
- Decision: `mobile-usefulness-floor`
- Scope:
  - `pulse-mobile`
  - `pulse`
  - lanes `L5`, `L7`, `L8`, `L12`
  - subsystems `frontend-primitives`, `relay-runtime`

## Decision

Pulse Mobile does not need desktop parity to stop blocking v6 RC stabilization.
The v6 mobile usefulness floor is narrower and concrete:

1. A user can keep at least one trusted Pulse instance paired and available
   across relaunches without needing to re-pair on normal reconnect paths.
2. The primary mobile shell exposes current relay/runtime state clearly enough
   that a user can tell whether the active instance is connecting, secured,
   offline, draining, or in error.
3. Stale or revoked mobile access fails closed into a safe disconnected state
   with a recoverable path back to `Add Instance`, rather than leaving the user
   in an ambiguous or partially-authorized session.
4. The mobile app provides useful post-pairing navigation for the v6 RC line:
   Dashboard, Findings, Chat, Approvals, and Settings.
5. Live approval recovery is part of that floor: pending approvals must appear,
   approvals must survive normal reconnect/relaunch behavior, and approval
   actions must converge cleanly to approved or denied terminal states.

Desktop-feature parity, richer operational navigation, and broader mobile
surface expansion remain post-RC and post-GA scope, not v6 RC blockers.

## Evidence Considered

1. `docs/release-control/v6/records/mobile-relay-auth-approvals-2026-03-13.md`
   proves real pairing, persisted relaunch, revoked-credential recovery, and
   live approval visibility and action execution on a physical Android device.
2. `pulse-mobile/src/navigation/routes.ts` defines the active v6 route surface:
   `Dashboard`, `Findings`, `Chat`, `Approvals`, `Settings`, plus instance and
   security entry points.
3. `pulse-mobile/app/(tabs)/_layout.tsx` wires those five primary tabs and
   presents the approval badge in the main shell.
4. `pulse-mobile/src/components/shared/connectionBannerState.ts` and
   `pulse-mobile/src/components/shared/ConnectionBanner.tsx` expose relay-state
   feedback for connecting, securing, draining, offline, and error states.
5. `pulse-mobile/src/stores/mobileAccessState.ts` and instance-scoped clearing
   prove the app has an explicit safe-reset path for stale or revoked access.

## Outcome

- `mobile-usefulness-floor` is resolved.
- Pulse Mobile is judged useful enough for the v6 RC line on the narrower floor
  above.
- Future mobile polish and deeper parity work should be captured as post-RC or
  post-GA targets instead of blocking RC stabilization by default.
