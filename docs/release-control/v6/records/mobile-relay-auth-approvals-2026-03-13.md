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

## Approval Coverage Note

- The open `pulse` lab backend does not expose live approval management over
  `/api/ai/approvals`; in this workspace build it correctly returned `402
  Approval management requires Pulse Pro` without the enterprise auto-fix
  adapter.
- Approval routing, approval state handling, and approval-action behavior were
  therefore covered by the audited `pulse-mobile` automated proof bundle plus
  the targeted `pulse-enterprise/internal/aiautofix` approval handler tests
  above, while the live manual rehearsal covered the real mobile pairing,
  persistence, reconnect, and revocation boundaries against a real relay
  instance.

## Outcome

- Real mobile pairing now succeeds against a real relay-backed Pulse instance.
- Persisted relaunch and reconnect now succeed without re-pairing.
- Revoked credentials fail closed back to a safe disconnected state.
- The onboarding payload now gives mobile the canonical app endpoint instead of
  the instance-only relay endpoint that previously caused immediate disconnects.
- Approval visibility and action handling remain covered by the governed mobile
  and enterprise proof suites for the enterprise approval runtime.
