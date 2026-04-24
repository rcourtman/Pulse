# Mobile Product Purpose GA Blocker - 2026-04-24

## Classification

- Type: release gate
- Blocking level: release-ready
- Lane: L5 Mobile go-live readiness

## Trigger

During physical iPad proof review on 2026-04-24, the product owner stated that the actual Pulse Mobile experience does not make clear what the app brings to a normal self-hosted Pulse user or what the app is for.

The product owner further clarified that normal self-hosted users are unlikely to think of Pulse Mobile as a control center for approving commands. Pulse is understood first as a monitoring application, so a mobile app that does not lead with monitoring will confuse users even if approval and recovery flows are technically useful.

The product owner then paused the monitoring-first assumption as well: because the web app has already been optimized for mobile, Pulse Mobile may not need to duplicate monitoring for broad self-hosted users. The product role may instead be business or multi-tenant operations, Relay-backed remote instance access, notification continuity, approvals, or another narrower job. At that point the mobile product purpose was recorded as an open decision, not merely a UX polish issue.

## Judgment

The current mobile candidate may be technically release-capable, including physical-device pairing, APNs delivery, tap-through routing, approval actions, reconnect, and store configuration evidence, but that does not make it product GA-ready.

The product direction is now locked: Pulse Mobile v6 GA is a native companion for paired Pulse access, phone-native status, alerts, push/device trust, Relay-backed web dashboard handoff, and safe contextual recovery from notifications or deep links. It is not a miniature web dashboard, not a command-approval console, and not a separate control center. Full monitoring depth remains owned by the mobile-optimized web app; the native app must make it obvious when the user should stay native and when they should open Pulse web through Relay.

Pulse Mobile public rollout remains blocked until the current candidate proves this role on physical-device walkthrough. Simulator proof can validate navigation, copy, and release UI automation, but it cannot clear the hardware trust, push, Relay handoff, and user-comprehension bar required for GA.

## Exit Criteria

- The app has an explicit product role rather than feeling like a thin or unexplained subset of desktop Pulse.
- Status and alerts are visible native value: a paired self-hosted user can quickly see whether the phone has a fresh trusted view, whether anything needs attention, and whether Pulse web should be opened for detail.
- Approval, command, and recovery surfaces are positioned as contextual secondary actions from push, deep links, alerts, and follow-ups rather than as the apparent reason the app exists.
- Relay-backed Open Pulse handoff is first-class: the app should guide users into their real Pulse dashboard when they need full monitoring detail instead of duplicating the dashboard badly.
- Empty and all-clear states still feel useful as phone-native status states, not like dead tabs waiting for approvals or commands.
- First-run, unpaired, paired, empty, alert, approval, recovery, and Open Pulse states make the next useful action obvious to a normal self-hosted operator.
- The first screen after pairing communicates current phone trust/access state and why opening the native app is useful.
- A physical-device walkthrough demonstrates that a user can understand the app purpose and primary jobs without release-team narration.
- Technical readiness evidence remains current on the candidate being promoted.

## Resolved Product Decision

The chosen Pulse Mobile v6 GA role is a native companion for self-hosted and Relay-backed operators:

- Native status, alerts, notification continuity, paired-access health, and safe contextual action stay native.
- Full monitoring depth, investigation, configuration, and dashboard work remain in Pulse web.
- Relay-backed Open Pulse handoff is the bridge between those jobs.
- The app must not present approvals as its primary product purpose.

## Current Evidence

On 2026-04-24, the redesigned candidate passed the iOS simulator release proof with the new Status, Alerts, Open Pulse, Access, and Settings IA:

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:proof:ios:simulator`
- Result: 10 UI tests executed, 4 physical-device-only tests skipped, 0 failures.
- Result bundle: `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/pulse-mobile-ios-proof-UeQIDD/PulseUITests.xcresult`

The same-day physical iPad proof could not be rerun after the redesign because Xcode only reported the paired iPad as unavailable (`tunnelState=unavailable`, last connected at 2026-04-24T20:13:00.000Z). The gate therefore remains blocked on fresh physical-device proof for the current candidate.
