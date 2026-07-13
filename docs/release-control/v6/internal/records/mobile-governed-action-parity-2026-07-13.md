# Pulse Mobile governed action parity — 2026-07-13

## Outcome

Pulse Mobile now preserves the canonical governed-action boundary instead of
compressing permission and execution into one approval gesture. Pending plans
are reviewed and decided first. An approved plan remains visible as active work
until the operator chooses **Run action** and passes a second local
authentication gate. Denial remains non-executing.

## Identity and transport safety

- Mobile reads the durable action inbox, including approved-but-not-run and
  executing actions, rather than the decision-only compatibility queue.
- The displayed plan identity is projected from the canonical `planHash` and is
  required locally before approve, deny, or execute can reach the API.
- Decision and execution requests carry that plan identity. Core rejects a
  presented mismatch with `action_plan_identity_mismatch` before decision
  persistence or executor dispatch while preserving compatibility for clients
  that do not yet present the optional transport field.
- The app no longer models or renders command-shaped approval data. Review is
  based on typed capability, target, preflight, blast radius, safety, rollback,
  verification, and plan identity facts.

## Operator truth

- Approval policy and operational risk remain distinct facts. Admin, operator,
  MFA, no-approval, and dry-run policies are no longer rendered as unknown risk.
- Approval copy states that permission does not run the action. Execution copy
  states that the approved plan will run now and produces a verification result.
- Active counts and cross-source routing include both pending decisions and
  approved plans ready to run, so reload or source switching cannot hide the
  second step.

## Revoked access recovery

Terminal Relay authorization errors and proxied HTTP 401 responses still fail
closed by removing rejected credentials and instance-scoped state. They now
also persist a bounded recovery explanation containing the affected source
name, entitlement-aware guidance when applicable, and a **Pair Again** action.
The explanation survives restart and appears above the main tabs until the
operator acts or dismisses it; repeated terminal signals cannot overwrite the
better source-specific record after credentials have already been removed.

## Proof

- Core action handler tests prove mismatched client plan identities cannot
  record decisions or reach an executor.
- Pulse Mobile typecheck passes.
- All Pulse Mobile application tests pass, including plan projection, separate
  decision/execution API calls, truthful posture, active-inbox projection,
  second-step recovery, and durable revoked-access recovery.
- Release-script tests pass and pin separate decision and execution gestures in
  the iOS and Android native proof drivers.
- The focused Release-arm64 iOS simulator proof executed one XCUITest with zero
  failures in 74.381 seconds. It observed the approval-recorded/not-run state,
  the distinct **Run action** control, the completed execution result, and the
  denial path. The result bundle and summary are retained locally under
  `pulse-mobile/artifacts/proof/mobile-approval-parity-2026-07-13-rerun/`.
- The physical-device readiness probe passes against an unlocked iPad running
  iOS 26.5.2 with Developer Mode and developer disk-image services available.

## Residual

This closes the governed-action parity slice. The broader
`lane-followup:mobile-post-rc-hardening` remains planned because public store
publication is a separate human-triggered release operation and future post-GA
hardening remains deliberately open.
