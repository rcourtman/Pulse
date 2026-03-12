# Upgrade State and Entitlement Preservation Record

- Date: `2026-03-12`
- Gate: `upgrade-state-and-entitlement-preservation`
- Assertions:
  - `RA3`
  - `RA6`
- Environment:
  - Upgrade rehearsal host: `http://127.0.0.1:17655`
  - Starting version: `v5.1.23`
  - Candidate version: `v6.0.0-rc.1`
  - Data directory: `/tmp/pulse-upgrade-rehearsal/data`
  - Strict-mode exchange stub: `http://127.0.0.1:18666`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestHandleActivateLicense_ExchangesLegacyJWTInStrictV6|TestHandleActivateLicense_ClearsCommercialMigrationStateOnNativeActivation|TestHandleActivateLicense_ActivationKeyClearsStaleLegacyPersistence|TestGetTenantComponents_AutoExchangesPersistedLegacyJWT|TestGetTenantComponents_SkipsExchange_WhenActivationStateExists|TestGetTenantComponents_PersistsCommercialMigrationState_WhenAutoExchangeFails|TestRequireLicenseFeature_HostedEntitlementsBlockMissingFeature|TestRequireLicenseFeature_HostedEntitlementsAllowGrantedFeature|TestLicenseGatedEmptyResponse_HostedEntitlementsReturnEmptyArrayWhenLocked|TestHandleGetUpdatePlan|TestHandleGetUpdatePlan_InvalidChannel|TestHandleGetUpdatePlan_PrepareError|TestHandleGetUpdatePlan_ManualFallback' -count=1`
- `go test ./pkg/licensing/... -count=1`
- `go test ./tests/migration -run 'TestV5PaidLicenseUpgrade_CommercialMigrationFailureMatrix|TestV5DataDir_CSRFLegacyMapFormat|TestV5DataDir_CSRFTokenFileContinuity|TestV5DataDir_SessionLegacyMapFormat|TestV5DataDir_SessionTokenContinuity|TestV5DowngradeSafety|TestV5FullUpgradeScenario' -count=1`
- `cd tests/integration && PULSE_BASE_URL=http://127.0.0.1:17655 PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=adminadminadmin PULSE_E2E_SKIP_DOCKER=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 npm test -- tests/11-first-session.spec.ts --project=chromium`
- Result: pass

## Manual Upgrade Exercise

1. Built a real `v5.1.23` server binary from the previous supported line and launched it against a fresh persistent data directory.
2. Completed the normal security/bootstrap flow on v5, persisted admin auth, created an API token, and activated a paid legacy entitlement in the same data directory.
3. Stopped the v5 process without deleting or editing local state.
4. Launched the candidate v6 binary against the exact same data directory in strict entitlement mode, backed by a private exchange stub signed with a matching Ed25519 public key.
5. Observed the persisted legacy entitlement auto-exchange into canonical v6 activation state without prompting for repeated license entry.
6. Confirmed authenticated startup continuity held after upgrade:
   - existing admin auth remained valid
   - existing API token remained valid
   - local session/CSRF continuity stayed intact
7. Confirmed `GET /api/license/status` after upgrade returned:
   - `valid=true`
   - `tier=pro`
   - `plan_version=v5_pro_monthly_grandfathered`
   - `email=upgrade-rehearsal@example.com`
   - `max_agents=10`
8. Confirmed `GET /api/license/entitlements` after upgrade returned active hosted-style paid state with the same grandfathered plan and `max_agents.limit=10`.
9. Ran the first-session browser suite against the upgraded v6 instance and confirmed the app no longer fell into an update-plan error path for non-auto-update deployments.
10. Confirmed the upgraded app loaded first-session and settings surfaces without license re-entry, reset prompts, or paid-surface drift.

## Outcome

- Supported upgrade preserved local state, authenticated continuity, entitlement continuity, and first-session continuity.
- Paid activation did not need to be re-entered after upgrade.
- The persisted v5 entitlement auto-exchanged into canonical v6 state under strict-mode validation.
- First-session surfaces stayed healthy after upgrade once the manual/development update-plan fallback was fixed.

## Notes

- The strict-mode entitlement rehearsal used a private local exchange stub rather than the production hosted service. That keeps the RC proof repeatable while still exercising the real persisted-license auto-exchange path.
- An earlier attempt exposed a real bug where `/api/updates/plan` returned `404` for manual/development deployments and broke the first-session browser suite. That backend path is now fixed and covered by `TestHandleGetUpdatePlan_ManualFallback`.
