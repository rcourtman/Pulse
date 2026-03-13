# Upgrade State and Entitlement Preservation Record

- Date: `2026-03-13`
- Gate: `upgrade-state-and-entitlement-preservation`
- Assertions:
  - `RA3`
  - `RA6`
- Evidence tier: `real-external-e2e`

## Automated Proof Baseline

- `go test ./internal/api -run 'TestHandleActivateLicense_ExchangesLegacyJWTInStrictV6|TestHandleActivateLicense_ClearsCommercialMigrationStateOnNativeActivation|TestHandleActivateLicense_ActivationKeyClearsStaleLegacyPersistence|TestGetTenantComponents_AutoExchangesPersistedLegacyJWT|TestGetTenantComponents_SkipsExchange_WhenActivationStateExists|TestGetTenantComponents_PersistsCommercialMigrationState_WhenAutoExchangeFails|TestRequireLicenseFeature_HostedEntitlementsBlockMissingFeature|TestRequireLicenseFeature_HostedEntitlementsAllowGrantedFeature|TestLicenseGatedEmptyResponse_HostedEntitlementsReturnEmptyArrayWhenLocked|TestHandleGetUpdatePlan|TestHandleGetUpdatePlan_InvalidChannel|TestHandleGetUpdatePlan_PrepareError|TestHandleGetUpdatePlan_ManualFallback' -count=1`
- `go test ./pkg/licensing/... -count=1`
- `go test ./tests/migration -run 'TestV5PaidLicenseUpgrade_CommercialMigrationFailureMatrix|TestV5PaidLicenseUpgrade_RealLicenseServerExchange|TestV5DataDir_CSRFLegacyMapFormat|TestV5DataDir_CSRFTokenFileContinuity|TestV5DataDir_SessionLegacyMapFormat|TestV5DataDir_SessionTokenContinuity|TestV5DowngradeSafety|TestV5FullUpgradeScenario' -count=1`
- `cd tests/integration && PULSE_BASE_URL=http://127.0.0.1:17655 PULSE_E2E_USERNAME=admin PULSE_E2E_PASSWORD=adminadminadmin PULSE_E2E_SKIP_DOCKER=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 npm test -- tests/11-first-session.spec.ts --project=chromium`
- Result: pass

## Real External Exchange Proof

`TestV5PaidLicenseUpgrade_RealLicenseServerExchange` now replaces the old strict-mode exchange stub as the closure-strengthening proof for paid upgrade continuity.

It exercises the real sibling external dependency in `pulse-pro` instead of an in-process fake:

1. Builds and starts the real `pulse-pro/license-server` binary with a generated Ed25519 signing key and v5 grandfathered plan definitions.
2. Seeds the license-server data directory with a legacy v5 license record for each supported grandfathered shape:
   - `v5_lifetime_grandfathered`
   - `v5_pro_monthly_grandfathered`
   - `v5_pro_annual_grandfathered`
3. Generates a genuinely signed legacy v5 JWT using the same private key the real license server uses for verification.
4. Persists that legacy JWT into the local Pulse data directory as the pre-upgrade paid state.
5. Starts the v6 license handling path against the real `POST /v1/licenses/exchange` endpoint.
6. Confirms the upgrade result for each case:
   - paid state auto-exchanges on startup without repeated license entry
   - a new canonical v6 `lic_...` activation is persisted
   - the activation state points back to the real license-server base URL
   - grandfathered `plan_version` continuity is preserved
   - `max_agents` continuity is preserved
   - the original legacy JWT remains on disk for downgrade safety

## Managed Runtime Continuity Still Covered

The `2026-03-12` upgrade rehearsal remains relevant supporting evidence for the parts this new proof does not replace:

- local state continuity across the v5 -> v6 binary swap
- first-session continuity
- session / CSRF continuity
- non-paid and paid surface stability after upgrade

That record is now supporting evidence, not the sole closure basis.

## Outcome

- Upgrade continuity is now backed by both:
  - real external exchange against the real `pulse-pro/license-server`
  - managed-runtime first-session and local-state continuity evidence
- The gate no longer depends on a local exchange stub to claim closure confidence.
- This is sufficient to treat `upgrade-state-and-entitlement-preservation` as genuinely meeting its `real-external-e2e` evidence threshold.
