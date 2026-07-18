# Operational Trust Alert-State Boundary

Date: 2026-07-18
Lane: L6 alerts and notification coherence
Candidate: `protection-posture-attention-queue`
Specification:
`docs/release-control/v6/internal/OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`

## Outcome

Pulse now preserves the canonical boundary between alert detection, in-product
alert visibility, and external notification delivery.

- `AlertConfig.enabled` owns detector evaluation and the in-product active-alert
  read model.
- `AlertConfig.activationState` remains the compatibility field controlling
  external notification delivery until the specification's typed API migration
  replaces it.
- Pausing external notifications no longer clears websocket alert state, hides
  row alert indicators, blocks alert configuration, or claims that threshold
  evaluation has stopped.
- The Alerts surface names the control **Notifications enabled** or
  **Notifications paused** and explicitly states that Pulse continues detecting
  and showing active alerts while delivery is paused.

This removes the observed contradiction where navigation reported active alerts
while the Alerts page presented monitoring as paused and empty.

## Canonical Ownership

The alerts subsystem contract now owns the frontend state boundary, including
the websocket projection and shared presentation helpers. The matching
`alert-state-ownership-boundary` path policy requires focused store, page,
websocket-resilience, and compatibility-helper tests whenever those files
change.

## Automated Proof

- `npm run type-check` from `frontend-modern`
- `npm --prefix frontend-modern run lint:eslint -- --quiet`
- `npm test -- src/stores/__tests__/alertsActivation.test.ts src/utils/__tests__/alertsActivation.test.ts src/utils/__tests__/alertsActivation.branchcov0712c.test.ts src/pages/__tests__/Alerts.readOnly.test.tsx src/stores/__tests__/websocket-resilience.test.ts`
  - 5 files passed
  - 28 tests passed
- Focused table and surface suite:
  - 14 files passed
  - 128 tests passed
- `npm test -- src/i18n/__tests__/i18n.test.ts`
  - 19 tests passed
- `npm test -- src/utils/__tests__/metricThresholds.test.ts`
  - 34 tests passed
- `go test ./internal/alerts -run 'TestNotificationActivationDoesNotSuppressDetectionOrActiveReadModel' -count=1`
  - proves both `pending_review` and `snoozed` continue detection and active
    read-model visibility while suppressing external delivery
- `go test ./internal/alerts -run 'TestCheckMetricInvokesAICallbackWhenNotificationsSuppressed|TestSmokeUnifiedEvaluatorActiveEndToEnd|TestSmokeAlertDisableGatesUnifiedEvaluation|TestSmokeReenableProducesCorrectAlerts' -count=1`
- `go test ./internal/api -run 'TestActivateAlerts_EnablesNotificationManager' -count=1`
- `python3 scripts/release_control/status_audit.py --pretty`
  - repository governance ready
  - no missing evidence
  - no claim conflicts
- `python3 scripts/release_control/contract_audit.py --check`
  - 16 subsystem contracts
  - no errors
  - no warnings
- `python3 -m unittest subsystem_contracts_test contract_audit_test` from
  `scripts/release_control`
  - 18 tests passed

## Live Browser Proof

The local v6 Alerts surface was exercised in the in-app browser at desktop and
390 by 844 mobile dimensions.

1. With notifications enabled, the navigation badge and Alerts overview both
   showed the same four active alerts.
2. Pausing notifications changed the header to **Notifications paused** while
   the active alerts remained visible.
3. The confirmation message stated that Pulse would keep detecting and showing
   active alerts.
4. Thresholds, Notifications, and Schedule remained available. Opening
   Thresholds showed the complete configuration surface while delivery was
   paused.
5. Notifications were restored before ending the proof.
6. The narrow viewport retained the header control, active-alert content, and
   mobile navigation. A transient hydration frame settled to the same four
   active alerts without a persistent contradictory empty state.
7. No browser console errors or warnings were present.

## Remaining Specification Work

This evidence closes only the contradiction-containment implementation slice.
It does not close the Operational Trust specification. The typed API migration,
read-model consistency assertions, durable lifecycle and evidence model,
protection posture, Patrol attention workbench, availability attachment,
governed action loop, and hardening phases remain governed by the specification.
