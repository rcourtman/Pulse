# Mobile Relay Auth Approvals Record

- Date: `2026-03-13`
- Gate: `mobile-relay-auth-approvals`
- Environment:
  - Pulse workspace repos:
    - `pulse`
    - `pulse-mobile`
    - `pulse-pro`
    - `pulse-enterprise`
  - Lab backend: `http://127.0.0.1:55190`
  - Public URL under test: `http://192.168.0.98:55190`
  - Relay instance endpoint: `wss://127.0.0.1:8443/ws/instance`
  - Mobile onboarding endpoint presented to app: `wss://127.0.0.1:8443/ws/app`
  - Relay instance id: `relay_75c6978012c883dc`
  - Booted simulator device:
    - `Pulse RC iPhone 16`
    - `5ADC45DD-E5D9-4FF9-ADBB-59D1E95171F3`
  - Release simulator app:
    - `/tmp/pulse-mobile-derived-release/Build/Products/Release-iphonesimulator/Pulse.app`

## Automated Proof Baseline

- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/__tests__/mobileRelayAuthApprovals.rehearsal.test.ts src/utils/__tests__/secureStorage.test.ts src/hooks/__tests__/useRelayLifecycle.test.ts src/hooks/__tests__/approvalActionPolicy.test.ts src/stores/__tests__/instanceStore.test.ts src/stores/__tests__/authStore.test.ts src/stores/__tests__/approvalStore.test.ts`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/api/__tests__/client.test.ts`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/hooks/__tests__/useRelay.test.ts src/hooks/__tests__/relayPushRefresh.test.ts src/notifications/__tests__/notificationRouting.test.ts src/stores/__tests__/mobileAccessState.test.ts`
- `cd /Volumes/Development/pulse/repos/pulse-enterprise && go test ./internal/aiautofix -run 'TestHandleListApprovals|TestHandleApproveAndExecuteInvestigationFix|TestHandleApprove' -count=1`
- Result: pass

## Manual Exercise A: Real Pairing Through Relay Onboarding

1. Rebuilt the local Pulse lab backend with the onboarding relay URL normalization fix so `/api/onboarding/qr` emitted `relay_url=wss://127.0.0.1:8443/ws/app` while the desktop relay config remained on the canonical instance endpoint.
2. Verified the live onboarding payload returned:
   - `instance_id=relay_75c6978012c883dc`
   - `relay.url=wss://127.0.0.1:8443/ws/app`
   - no diagnostics
3. Minted a fresh real pairing token through `POST /api/security/tokens`.
4. Fetched the real deep link through `GET /api/onboarding/qr` with that token in `X-API-Token`.
5. Uninstalled the simulator app, reinstalled the Release build, and launched it with the committed simulator launch hook using `SIMCTL_CHILD_PULSE_SIMULATOR_LAUNCH_URL=<deep-link>`.
6. Confirmed the backend moved from `active_channels=0` to `active_channels=1`.
7. Confirmed the Pulse backend logged:
   - `channel opened channel=1`
   - `key exchange completed, channel encrypted channel=1`
8. Captured the paired state screenshot at:
   - `/tmp/pulse-mobile-mobile-relay-auth-approvals-passed.png`

## Manual Exercise B: Persisted Relaunch and Reconnect

1. Terminated the paired simulator app without changing its stored instance.
2. Relaunched the app normally with no deep link.
3. Confirmed the backend returned to `active_channels=1` after relaunch.
4. Confirmed the stored instance stayed available to the app and the channel came back without re-pairing.

## Manual Exercise C: Revocation Fails Closed

1. Deleted the exact pairing token used by the simulator via:
   - `DELETE /api/security/tokens/{token_id}`
2. Terminated the simulator app and relaunched it normally.
3. Confirmed the backend no longer kept a mobile relay channel:
   - `active_channels=0`
4. Confirmed the Pulse backend rejected the reconnect attempt with:
   - `Rejecting channel: invalid auth token`
5. Captured the revoked safe-state screenshot at:
   - `/tmp/pulse-mobile-mobile-relay-auth-approvals-revoked.png`
6. Confirmed the app returned to a safe empty state with `Add Instance` visible instead of preserving stale access.

## Manual Exercise D: Real Android Approval Visibility Through Live Relay

1. Switched from the earlier simulator-only lab backend to the real
   enterprise-capable Pulse test container on `delly`:
   - backend under test: `http://192.168.0.106:7655`
   - live managed relay endpoint: `wss://relay.pulserelay.pro/ws/instance`
   - mobile onboarding endpoint presented to the app:
     `wss://relay.pulserelay.pro/ws/app`
2. Paired a fresh Android device through the real relay onboarding path and
   confirmed the phone reached the live instance as `Instance relay_98`.
3. Verified the backend approval API first returned an empty state:
   - `GET /api/ai/approvals`
   - response stats: `pending=0`, `expired=1`
4. Reloaded the real mobile dev bundle after the mobile initial-hydration fix
   so the live phone was running the new `pulse-mobile` code, not a stale Metro
   session.
5. Opened the real phone approval route and confirmed the empty state rendered
   without the previous perpetual spinner:
   - screenshot: `/tmp/pulse-mobile-approvals-fresh.png`
6. Seeded a fresh pending approval into the enterprise-backed test instance and
   confirmed the backend returned:
   - `GET /api/ai/approvals`
   - response stats: `pending=1`, `expired=0`
7. Reopened the approval route on the physical Android device and confirmed the
   pending approval card rendered live over the real relay channel instead of
   hanging in loading:
   - screenshot: `/tmp/pulse-mobile-approvals-pending.png`

## Manual Exercise E: Real Android Approve and Deny Actions Through Live Relay

1. Kept the same paired Android device and enterprise-backed Pulse test
   container from Exercise D:
   - backend under test: `http://192.168.0.106:7655`
   - live managed relay endpoint: `wss://relay.pulserelay.pro/ws/instance`
   - mobile onboarding endpoint presented to the app:
     `wss://relay.pulserelay.pro/ws/app`
2. Seeded a fresh plain pending approval into the live approval store on the
   test container and restarted `pulse.service` so the running backend reloaded
   it, then confirmed the backend approval API returned:
   - `GET /api/ai/approvals`
   - response stats: `pending=1`, `approved=1`, `denied=0`
3. Opened that approval on the physical Android device and confirmed the detail
   screen now rendered the idle action state correctly before confirmation:
   - both `Approve` and `Deny` buttons were idle
   - the earlier premature `Approving...` state no longer appeared before the
     confirm dialog
4. Triggered `Approve` on-device and confirmed the native `Approve Fix`
   confirmation dialog rendered before the action committed.
5. Confirmed the approval on-device and verified both sides converged to the
   expected approved terminal state:
   - backend `GET /api/ai/approvals` response stats: `pending=0`, `approved=2`,
     `denied=0`
   - mobile detail screen showed `Status: Approved`
   - mobile detail screen rendered the inline success copy:
     `This fix has been approved`
   - screenshot: `/tmp/pulse-after-approve.png`
6. Seeded a second fresh plain pending approval into the same live approval
   store, restarted `pulse.service`, and confirmed the backend returned:
   - `GET /api/ai/approvals`
   - response stats: `pending=1`, `approved=2`, `denied=0`
7. Reopened the approval list on the physical Android device, opened the new
   pending item, and triggered `Deny`.
8. Confirmed the mobile app rendered the dedicated `Deny Fix` sheet with the
   optional reason field before the action committed.
9. Confirmed the denial on-device and verified both sides converged to the
   expected denied terminal state:
   - backend `GET /api/ai/approvals` response stats: `pending=0`, `approved=2`,
     `denied=1`
   - mobile detail screen showed `Status: Denied`
   - mobile detail screen rendered the inline success copy:
     `This fix has been denied`
   - screenshot: `/tmp/pulse-after-deny.png`

## Approval Coverage Note

- Approval routing, approval list visibility, and scoped approval state are now
  covered both by the audited `pulse-mobile` automated proof bundle and by the
  live Android exercise above against the enterprise-backed approval runtime.
- Approval action execution is now covered in three layers:
  - governed `pulse-mobile` approval-action tests
  - governed `pulse-enterprise/internal/aiautofix` approval handler tests
  - the real Android approve and deny executions above against the
    enterprise-backed approval runtime

## Outcome

- Real mobile pairing now succeeds against a real relay-backed Pulse instance.
- Persisted relaunch and reconnect now succeed without re-pairing.
- Revoked credentials fail closed back to a safe disconnected state.
- Real approval list visibility now succeeds on a physical Android device for
  both the empty state and a live pending approval.
- Real approval action execution now succeeds on a physical Android device for
  both `Approve` and `Deny`, with correct confirm-sheet behavior and correct
  approved/denied terminal states instead of hanging or forcing the user back
  through a stale loading loop.
- The onboarding payload now gives mobile the canonical app endpoint instead of
  the instance-only relay endpoint that previously caused immediate disconnects.
- Approval action handling is now covered by both the governed proof suites and
  the live physical-device exercise against the enterprise approval runtime.
