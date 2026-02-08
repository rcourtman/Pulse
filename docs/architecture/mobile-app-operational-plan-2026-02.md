# Mobile App Operational Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/mobile-app-operational-progress-2026-02.md`

## Product Intent

The Pulse Mobile App is an operational companion, not just a dashboard.
It must handle critical alerts, approvals, and secure access reliably, even under adverse network conditions.
The primary goal is "Pocket Monitoring" - immediate awareness and actionability.

## Non-Negotiable Contracts

1. **Secure Pairing Contract**:
   - Pairing requires physical proximity (QR scan) or secure manual entry (rare).
   - Keys generated on device are never transmitted.
   - Initial handshake establishes E2E encryption for all subsequent relay traffic.

2. **Reliability Contract**:
   - App must reconnect automatically after network switch (WiFi <-> Cellular).
   - App must queue critical actions (approvals) if offline and sync when online.
   - Push notifications must deep-link to the correct resource context.

3. **Security Contract**:
   - Biometric lock (FaceID/TouchID) engages immediately upon backgrounding (configurable grace period).
   - Approval workflows require re-authentication or biometric confirmation.
   - Relay connection is mutually authenticated via device-specific keypair.

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: delegated coding agent.
- Reviewer: orchestrator.

A packet can be marked DONE only when:
- all packet tests pass,
- all required evidence is provided,
- reviewer gate checklist passes,
- verdict is `APPROVED`.
- every corresponding checklist item in the progress tracker is checked.

## Required Review Output (for every packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit 0
2. `<command>` -> exit 0

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Global Validation Baseline

Run after every packet unless explicitly waived:

1. `npm --prefix pulse-mobile run test` (Mobile unit tests)
2. `npm --prefix frontend-modern run test` (Web UI tests impacted by pairing changes)
3. `go test ./internal/api/... -run "Relay|Onboarding|Mobile" -v` (Backend integration tests)

## Execution Packets

### Packet 00: Web UI Pairing Source (The "Missing Link")

Objective:
- Enable users to generate a secure pairing QR code from the Web UI "Mobile App" settings panel.

Scope:
- `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` (or new component)
- `frontend-modern/src/api/onboarding.ts` (new)
- `internal/api/router_routes_ai_relay.go` (verify existing endpoint)

Implementation checklist:
1. Verify `GET /api/onboarding/qr` returns correct payload structure.
2. Implement frontend API client method for fetching QR payload.
3. Add "Pair New Device" button to Relay Settings.
4. Render QR code using a standard React QR library (e.g., `qrcode.react`).
5. Ensure QR code contains the full JSON payload expected by mobile scanner.
6. Add "Copy Payload" fallback for manual entry (if supported by mobile).

Tests:
1. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/RelaySettingsPanel.test.ts`
2. Manual verification: Scan generated QR with standard camera app (should verify JSON content).

Evidence:
- Screenshot of Web UI displaying QR code.
- JSON payload dump verification.

Exit criteria:
- Web UI generates valid QR code that matches mobile scanner expectations.

### Packet 01: Mobile Connection Hardening & Limits

Objective:
- Ensure mobile app handles network transitions gracefully and respects data usage limits.

Scope:
- `pulse-mobile/src/relay/client.ts`
- `pulse-mobile/src/context/RelayContext.tsx`
- `internal/api/relay_handlers.go`

Implementation checklist:
1. Implement exponential backoff for relay reconnection.
2. Handle "Network Unreachable" state with clear UI indicator (not crashing).
3. Implement "Data Saver" mode check (skip heavy charts if on metered connection).
4. Display "Relay Data Usage" meter in mobile settings (mock/real).

Tests:
1. `npm --prefix pulse-mobile run test`
2. Simulate network disconnect/reconnect during relay session.

Evidence:
- Screen recording of reconnection flow.

Exit criteria:
- App recovers from 30s offline state without requiring app restart.

### Packet 02: Push Notifications & Deep Linking

Objective:
- Ensure alerts reach the user and tapping them opens the correct screen.

Scope:
- `pulse-mobile/app.config.js` (Expo config)
- `pulse-mobile/src/navigation/`
- `internal/ai/notifications.go` (Push sender)

Implementation checklist:
1. Configure Expo Push Notification credentials (development/preview).
2. Implement notification listener/handler in mobile root.
3. Map notification "category" or "data" to navigation routes (e.g., `pulse://alert/123`).
4. Ensure deep links work from cold start and background state.

Tests:
1. Send test push from backend CLI.
2. Click push -> Verify navigation to target screen.

Evidence:
- Log of received notification payload.

Exit criteria:
- Clicking an "Alert" notification opens the Alert details screen.

### Packet 03: Secure Approval Workflow

Objective:
- Allow admins to approve robust actions (e.g., "Restart Service", "Block IP") from mobile.

Scope:
- `pulse-mobile/src/screens/ApprovalsScreen.tsx`
- `internal/ai/approval_handlers.go`

Implementation checklist:
1. Render pending approvals list.
2. Implement "Approve" and "Deny" actions with biometric re-auth (if available) or confirmation.
3. Send signed approval decision to backend via Relay.
4. Handle "Expired" or "Already Actioned" errors gracefully.

Tests:
1. End-to-end approval flow with mock backend event.

Evidence:
- Screen recording of approval flow.

Exit criteria:
- Approval action successfully triggers backend handler.

### Packet 04: Biometrics & App Lock

Objective:
- Protect sensitive infrastructure data when app is backgrounded.

Scope:
- `pulse-mobile/src/store/authStore.ts`
- `pulse-mobile/src/components/BiometricGate.tsx`

Implementation checklist:
1. Trigger lock screen immediately on `AppState` background -> active transition.
2. Allow configurable grace period (e.g., "Immediately", "1 min", "5 mins").
3. Obscure app content in multitasking switcher (if possible via Expo config).

Tests:
1. Background app -> Open app -> Verify Lock Screen appears.

Evidence:
- Config settings screenshot.

Exit criteria:
- App content is not visible until biometrics/pin pass after backgrounding.

## Milestones

M1 complete when Packet 00 is approved (Pairing Unblocked).
M2 complete when Packets 01-02 are approved (Reliable Communication).
M3 complete when Packets 03-04 are approved (Secure Operations).

## Definition of Done

This initiative is complete only when:
1. A user can pair a new mobile device via QR code in < 30 seconds.
2. The app stays connected/reconnects reliably on 4G/5G/WiFi transitions.
3. Push notifications reliably trigger for P0 alerts.
4. The app locks securely when not in use.
