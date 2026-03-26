# Relay Registration Reconnect Drain Record

- Date: `2026-03-12`
- Gate: `relay-registration-reconnect-drain`
- Environment:
  - Desktop relay runtime package: `internal/relay`
  - Desktop API/license and onboarding surfaces: `internal/api`
  - Desktop UI surfaces:
    - `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
    - `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
  - Mobile relay client repo: `/Volumes/Development/pulse/repos/pulse-mobile`

## Automated Proof Baseline

- `go test ./internal/relay -run 'TestClient_E2E_MultiMobileClientRelay|TestClient_AbruptDisconnectCancelsInFlightHandlers|TestClient_AbruptDisconnectMultipleChannelCleanup|TestClient_DrainDuringInFlightData|TestClient_DrainWithMultipleInFlightChannels|TestClientRegister_SessionResumeRejectionClearsCachedSession|TestRunLoop_SessionResumeRejectionFallsBackToFreshRegister' -count=1`
- `go test ./internal/api -run 'TestRelayEndpointsRequireLicenseFeature|TestRelayOnboardingEndpointsRequireLicenseFeature|TestRelayLicenseGatingResponseFormat|TestOnboardingQRPayloadStructure|TestOnboardingValidateSuccessAndFailure|TestOnboardingDeepLinkFormat' -count=1`
- `cd frontend-modern && npx vitest run src/components/Dashboard/__tests__/RelayOnboardingCard.test.tsx src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx src/components/Settings/__tests__/settingsReadOnlyPanels.test.tsx`
- `cd /Volumes/Development/pulse/repos/pulse-mobile && npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts src/relay/__tests__/protocol-contract.test.ts`
- Result: pass

## Exercised Relay Recovery Scenarios

1. Ran the targeted desktop relay runtime suite in verbose mode to capture the exact reconnect, drain, and stale-session behavior rather than treating the gate as a generic green test bucket.
2. Confirmed abrupt disconnect handling stayed bounded:
   - `TestClient_AbruptDisconnectCancelsInFlightHandlers` passed.
   - `TestClient_AbruptDisconnectMultipleChannelCleanup` passed.
3. Confirmed server-drain behavior canceled in-flight work cleanly and recovered registration:
   - `TestClient_DrainDuringInFlightData` logged `Relay server draining, will reconnect`, closed the active relay connection, canceled the in-flight local request with `context canceled`, and re-registered the same instance successfully.
   - `TestClient_DrainWithMultipleInFlightChannels` did the same with two simultaneous channels, canceling both local requests without hanging and then re-registering successfully.
4. Confirmed fresh registration and multi-client relay behavior still held under the same suite:
   - `TestClient_E2E_MultiMobileClientRelay` passed.
5. Confirmed stale-session recovery behaved predictably:
   - `TestClientRegister_SessionResumeRejectionClearsCachedSession` passed.
   - `TestRunLoop_SessionResumeRejectionFallsBackToFreshRegister` logged `relay session resume rejected, retrying fresh registration` and then re-registered the instance successfully instead of looping or stranding the client.
6. Confirmed the surrounding relay product surfaces stayed aligned with that runtime behavior:
   - desktop API/license/onboarding relay checks passed
   - desktop relay onboarding/settings UI checks passed
   - mobile relay client and protocol hardening checks passed in `pulse-mobile`

## Outcome

- Fresh relay registration still succeeds.
- Normal reconnect after disconnect remains healthy.
- Server drain closes active relay sessions without hanging or spinning, cancels in-flight work predictably, and reconnects cleanly.
- Stale session resume falls back to a fresh registration path instead of trapping the client in a dead session loop.
- Desktop API/license gating, onboarding payloads, desktop UI surfaces, and the mobile relay client stay aligned with the same reconnect and registration contract.

## Notes

- This record is grounded in the named relay runtime and client-contract exercises that explicitly force reconnect, drain, abrupt disconnect, and stale-session-resume paths. The verbose relay runtime run was captured on `2026-03-12` and showed the expected reconnect and cancellation messages at the exact pressure points the gate is meant to cover.
