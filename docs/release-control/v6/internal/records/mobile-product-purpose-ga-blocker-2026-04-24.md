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

On 2026-04-25, the build 4 source and local proof harness were rechecked without generating a release or store submission:

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:readiness`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:probe:ios:device -- --require-unlocked`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && adb devices -l`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && adb mdns services`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run typecheck`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runInBand`

Results:

- Release-readiness metadata still matches app build 4 and keeps public release blocked because physical Android and iOS evidence is stale after the companion-role redesign.
- TypeScript passed.
- Mobile app tests passed: 131 suites, 883 tests.
- Mobile script tests passed: 109 tests.
- The connected iOS physical-device record still could not clear proof availability: `devicectl` listed the paired iPad as `tunnelState=unavailable`, with the same last connection timestamp from 2026-04-24.
- ADB only listed the unsuitable Android TV emulator. The only discovered wireless ADB service (`192.168.0.119:45873`) refused connection, so no physical Android proof target was reachable.

Later on 2026-04-25 UTC, a physical Android phone became reachable over ADB as `192.168.0.119:40467` (`2510DPC44G`, Android 16) with build 4 installed. The Android launch proof was rerun after correcting the proof helper in `pulse-mobile` commit `cc44408` to recognize Android 16 `ResumedActivity` dumps, capture the UI hierarchy, fail explicitly when Keyguard or NotificationShade covers the app, and ignore non-fatal `uiautomator` `AndroidRuntime` bootstrap logs.

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run test:scripts`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:proof:android -- --serial 192.168.0.119:40467 --output-dir /Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-launch-proof-locked-filtered-20260425`

Results:

- Mobile script tests passed: 112 tests.
- The installed Android package matched build 4 (`versionCode=4`, `versionName=1.0.0`) and the app reached `pro.pulserelay.mobile/.MainActivity` as the resumed foreground activity.
- No Pulse-owned fatal Android runtime lines were detected.
- The proof remained blocked because the phone keyguard or notification shade still covered the app (`deviceLocked=true`). This does not clear the Android physical-device launch proof; unlock the phone and rerun before using this evidence for GA readiness.

On 2026-04-26, the same physical Android phone was manually unlocked and the build 4 Android launch proof passed:

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:proof:android -- --serial 192.168.0.119:40467 --output-dir /Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-launch-proof-unlocked-20260426`

Results:

- The installed Android package still matched build 4 (`versionCode=4`, `versionName=1.0.0`).
- The app reached `pro.pulserelay.mobile/.MainActivity` as the resumed foreground activity.
- The proof captured `deviceLocked=false` and 0 Pulse-owned fatal Android runtime lines.
- Summary: `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-launch-proof-unlocked-20260426/summary.md`
- Screenshot: `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-launch-proof-unlocked-20260426/screenshot.png`

This clears the Android physical-device launch proof only. The mobile GA gate remains blocked until the remaining current-candidate live Android pairing/reconnect/fail-closed/push/approval/instance-switching evidence and the iOS physical-device evidence are rerun, and until the physical-device walkthrough proves the companion role without release-team narration.

Also on 2026-04-26, the build 4 Android diagnostics/supportability proof passed on the same physical phone after `pulse-mobile` commit `cddcb62` hardened the proof harness for both healthy and repair-state diagnostics views:

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run test:scripts`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:proof:android:diagnostics -- --serial 192.168.0.119:40467 --output-dir /Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-diagnostics-20260426-fixed2 --wait-timeout-ms 20000`

Results:

- Mobile script tests passed: 112 tests.
- The installed release build exposed Settings -> Diagnostics on the physical Android phone.
- The stale hosted pairing state rendered the fail-closed supportability path rather than pretending the instance was healthy: `Relay path needs repair`, `Repair in Access`, and `Share Summary`.
- Diagnostics captured non-secret phone view, relay, push, follow-up, paired-access, security, and build sections across separate scroll positions.
- Summary: `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-diagnostics-20260426-fixed2/summary.md`

This adds current Android supportability and repair-path evidence, but it does not clear the live pairing, reconnect, fail-closed revocation, push-routing, approval-action, instance-switching, or iOS physical-device proof requirements.

Also on 2026-04-26, the Android hosted instance-switching proof harness was
hardened in `pulse-mobile` commit `1b48ce6` after the first live run exposed
two proof-script assumptions rather than an app crash:

- `npm run release:proof:android:instance-switching` now has a package script.
- The proof can run with `--reset-app-data` so fresh hosted pairing proof starts
  from a clean app state on the physical phone.
- The proof accepts the real Access summary state where one instance is active
  and another is highlighted, instead of requiring two full instance cards to
  be visible at the same scroll position.
- Mobile script tests passed: 113 tests.

The live hosted proof itself did not complete. The physical Android phone's
previous ADB serial (`192.168.0.119:40467`) disappeared, and the newly
advertised wireless-debugging endpoint (`192.168.0.119:45873`) refused
connection after restarting the local ADB server. The disposable hosted proof
account and tenants were deleted and cleaned from Pulse Cloud before stopping
the attempt. This keeps the proof harness ready for the next reachable physical
Android session, but it does not add live hosted pairing or instance-switching
evidence for GA.

Later on 2026-04-26, the physical Android phone came back over wireless ADB as
`192.168.0.119:45003`, and the build 4 hosted Android proof lane was completed
against a fresh disposable Pulse Cloud proof account and two hosted tenants.

Current Android hosted proof evidence:

- Fresh hosted pairing and instance switching passed from a clean app state:
  `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-instance-switching-20260426b-fixed/summary.md`
- Relaunch reconnect passed with the same active hosted instance restored after
  force-stop and relaunch, and diagnostics showing Relay client active and
  Encrypted API ready:
  `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-reconnect-20260426b-fixed2/summary.md`
- Live approval-actions passed from the Status action surface, with hosted
  approve and deny actions reconciling to the expected backend terminal states:
  `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-approval-actions-20260426b-fixed/summary.md`
- Live FCM push-routing passed through the dedicated relay sender credential,
  with the notification tap opening the current Alert recovery surface:
  `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-push-routing-20260426b-fixed/summary.md`
- Hosted relay-mobile token revocation passed from a single clean pairing,
  with Access failing closed to the empty safe state after the exact proof token
  was deleted and the tenant runtime restarted:
  `/Volumes/Development/pulse/.local-build-cache/pulse-mobile/tmp/android-build4-revoked-access-20260426b-fixed2/summary.md`

The Android proof harness was hardened during this pass to match the current
companion IA and OS behavior:

- Approval proof now enters pending actions from the Status action surface
  instead of the removed public Approvals tab.
- Reconnect proof captures relay readiness, paired access, and follow-up
  diagnostics across separate scroll positions so proof does not depend on one
  viewport containing every row.
- Push-routing proof recognizes the current Alert recovery copy.
- A new hosted Android revoked-access proof pairs one tenant, deletes the exact
  proof relay-mobile token, restarts the tenant runtime, and verifies the empty
  Access state.
- The revoked-access proof pre-grants or dismisses the Android notification
  permission prompt after `pm clear`, so the OS prompt cannot mask pairing or
  revocation state.

Verification:

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run test:scripts`
  passed with 113 script tests.
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm run release:readiness`
  now reports all Android build 4 physical-device evidence as passed and keeps
  public release blocked only on stale iOS physical-device gates.

The source, simulator-era product framing, and Android physical-device lane are
coherent with the resolved companion role, but the mobile public-release gate
remains blocked until the iOS physical-device lane is rerun on build 4.
