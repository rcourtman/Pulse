# Mobile Product Purpose GA Blocker - 2026-04-24

## Classification

- Type: release gate
- Blocking level: release-ready
- Lane: L5 Mobile go-live readiness

## Trigger

During physical iPad proof review on 2026-04-24, the product owner stated that the actual Pulse Mobile experience does not make clear what the app brings to a normal self-hosted Pulse user or what the app is for.

## Judgment

The current mobile candidate may be technically release-capable, including physical-device pairing, APNs delivery, tap-through routing, approval actions, reconnect, and store configuration evidence, but that does not make it product GA-ready.

Pulse Mobile public rollout is blocked until the in-app experience itself explains and proves the companion-app value for self-hosted operators. The target product shape should make the primary jobs obvious without internal explanation: away-from-desk infrastructure alert triage, quick status orientation, safe approval handling, and recovery from notification or deep-link context.

## Exit Criteria

- The app has an explicit product role rather than feeling like a thin or unexplained subset of desktop Pulse.
- First-run, unpaired, paired, empty, alert, approval, and recovery states make the next useful action obvious to a normal self-hosted operator.
- The first screen after pairing communicates current estate state and why opening mobile is useful.
- A physical-device walkthrough demonstrates that a user can understand the app purpose and primary jobs without release-team narration.
- Technical readiness evidence remains current on the candidate being promoted.
