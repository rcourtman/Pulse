# Relay Registration Reconnect Drain Record

- Date: `2026-03-13`
- Gate: `relay-registration-reconnect-drain`
- Evidence tier: `managed-runtime-exercise`
- Environment:
  - Pulse desktop repo: `/Volumes/Development/pulse/repos/pulse`
  - Relay server repo: `/Volumes/Development/pulse/repos/pulse-pro/relay-server`
  - Desktop relay runtime: `internal/relay`
  - Desktop relay onboarding/settings surfaces:
    - `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
    - `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
  - Mobile relay client repo: `/Volumes/Development/pulse/repos/pulse-mobile`

## Managed Runtime Exercise

- `go test ./internal/relay -run TestManagedRuntimeRelayRegistrationReconnectDrain -count=1`
- `python3 scripts/release_control/relay_registration_reconnect_drain_proof.py`

The managed-runtime test builds and launches the real `pulse-pro/relay-server`
binary, then drives it from the real `internal/relay` client with a real local
HTTP backend and a real app-side WebSocket connection. It does not use the
mock relay server used by the unit-level relay tests.

## Exercised Flow

1. Started a real `pulse-pro/relay-server` process with an ephemeral data dir
   and a generated Ed25519 public key for legacy relay license validation.
2. Started the real desktop relay client against that server and waited for
   canonical registration to complete.
3. Opened a real `/ws/app` WebSocket session and proxied a `/api/status`
   request through the relay to confirm the healthy baseline path.
4. Killed the relay server abruptly and restarted it with the same data dir,
   then confirmed the desktop relay client reconnected and proxied traffic
   successfully again.
5. Killed the relay server and restarted it with a fresh data dir, forcing the
   client’s cached session token to become stale. Confirmed the client logged
   `relay session resume rejected, retrying fresh registration`, cleared the
   stale resume path, re-registered cleanly, and proxied traffic successfully
   again.
6. Opened a fresh app-side relay connection, sent an in-flight proxied request
   to a deliberately slow local endpoint, then terminated the relay server
   gracefully to trigger its drain path.
7. Confirmed the desktop relay client logged `Relay server draining, will reconnect`,
   the in-flight local request was cancelled through the relay connection
   context, a replacement relay server process accepted the reconnect, and the
   client returned to `active_channels=0` before a final healthy proxy
   round-trip succeeded.

## Outcome

- Fresh registration succeeded on the real relay server binary.
- Normal reconnect after abrupt relay restart recovered cleanly.
- Stale session resume was rejected and the client fell back to a fresh
  registration path instead of getting trapped in a dead session loop.
- Server drain cancelled in-flight work predictably and the client reconnected
  to a replacement relay server without leaving a stuck active channel behind.
- The relay client remained capable of proxying live traffic after each phase.

## Notes

- This record supersedes the earlier `2026-03-12` relay record for closure
  confidence because it exercises the real relay server binary rather than only
  the mock relay harness plus targeted runtime tests.
- The older `2026-03-12` record still remains useful as lower-level automated
  pressure coverage, but it is no longer the strongest evidence for this gate.
