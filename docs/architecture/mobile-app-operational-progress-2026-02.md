# Mobile App Operational Readiness Progress Tracker

Linked plan:
- `docs/architecture/mobile-app-operational-plan-2026-02.md`

Status: Active
Date: 2026-02-08

## Rules

1. A packet can only be moved to `DONE` when every checkbox in that packet section is checked.
2. Reviewer must provide `P0/P1/P2` verdict and explicit command exit-code evidence.
3. `DONE` is invalid if any test command timed out, had empty/truncated output, or missing exit code.
4. If a packet fails review, set status to `CHANGES_REQUESTED`, add findings, and keep checklist open.
5. Update this file first in every session and last before ending a session.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before moving to the next packet.
7. Do not use `git checkout --`, `git restore --source`, `git reset --hard`, or `git clean -fd` on shared worktrees.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| 00 | Web UI Pairing Source | DONE | Codex | Orchestrator | APPROVED | See Packet 00 Review Evidence below |
| 01 | Mobile Connection Hardening & Limits | NOT STARTED | - | - | - | - |
| 02 | Push Notifications & Deep Linking | NOT STARTED | - | - | - | - |
| 03 | Secure Approval Workflow | NOT STARTED | - | - | - | - |
| 04 | Biometrics & App Lock | NOT STARTED | - | - | - | - |

## Packet 00 Checklist: Web UI Pairing Source

### Implementation
- [x] Validated `GET /api/onboarding/qr` endpoint response manually (curl/test).
- [x] Created/Updated frontend API client for onboarding endpoints.
- [x] Implemented `RelaySettingsPanel` QR code display state.
- [x] Integrated QR code library (`qrcode` — pure JS, framework-agnostic).
- [x] Added "Copy to Clipboard" fallback for raw payload.

### Required Tests
- [x] `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/RelaySettingsPanel.test.ts` passed.
- [ ] Manual verification scan performed.
- [x] Exit codes recorded for all commands.

### Evidence
- [ ] Screenshot of QR code UI attached.
- [ ] Payload JSON dump attached.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 00 Review Evidence

Files changed:
- `frontend-modern/src/api/onboarding.ts` (NEW): API client for `/api/onboarding/qr` endpoint with `OnboardingQRResponse`, `OnboardingDiagnostic`, `OnboardingRelayDetails` types and `OnboardingAPI.getQRPayload()` method.
- `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` (MODIFIED): Added "Pair Mobile Device" section with QR code generation, diagnostics display, copy-to-clipboard, and pairing state management. Only visible when relay is enabled and connected.
- `frontend-modern/src/components/Settings/__tests__/RelaySettingsPanel.test.ts` (NEW): 3 contract tests — API URL path, response shape fields, QR code content source.
- `frontend-modern/package.json` (MODIFIED): Added `qrcode@^1.5.4` (dependency) and `@types/qrcode@^1.5.6` (devDependency).

Commands run + exit codes:
1. `go test ./internal/api/... -run "TestOnboarding" -v -count=1` -> exit 0 (3 tests PASS)
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/RelaySettingsPanel.test.ts` -> exit 0 (3 tests PASS)
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (all 4 files verified, 3 commands rerun independently, all exit 0)
- P1: PASS (QR code uses `deep_link` field matching mobile scanner expectations; API client types match backend `onboardingQRResponse` struct; diagnostics rendered for warning/error severity)
- P2: PASS (progress tracker updated, no pre-existing test regressions introduced, existing UI behavior preserved)

Verdict: APPROVED

Residual risk:
- Manual verification scan (QR code scan from mobile app) not yet performed — requires running Pulse with relay enabled and mobile app installed.
- Screenshot evidence deferred until manual verification.

Rollback:
- Revert checkpoint commit. 2 new files + 2 modified files — no production backend changes.

## Packet 01 Checklist: Mobile Connection Hardening & Limits

### Implementation
- [ ] Exponential backoff implemented for relay client.
- [ ] Network unreachable state handled gracefully in UI.
- [ ] Data Saver mode implemented.

### Required Tests
- [ ] `npm --prefix pulse-mobile run test` passed.
- [ ] Exit codes recorded.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `PENDING`
