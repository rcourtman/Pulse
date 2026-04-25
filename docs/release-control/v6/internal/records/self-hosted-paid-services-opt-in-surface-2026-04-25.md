# Self-Hosted Paid Services Opt-In Surface Record

- Date: `2026-04-25`
- Assertion: `RA5`
- Lane: `L2`
- Result: `pass`

## Decision

Ordinary self-hosted Pulse v6 users must not be presented with paid-service
prompts, Pro trial CTAs, plan upsells, or paid-only navigation by default.
Commercial surfaces remain available only when the user deliberately reaches
for them through an explicit handoff/direct route, is running in hosted mode, or
already has paid entitlement/activation/recovery state.

This supersedes the earlier trial-first self-hosted monetization posture for
the normal v6 GA self-hosted app. Trial and checkout plumbing may remain for
support-only or externally initiated flows, but it is not a default in-app user
journey.

## Product Boundary

- Core self-hosted monitoring stays free and uncapped.
- Default Community/self-hosted UI stays quiet about paid services.
- Paid-only feature navigation is hidden unless the instance already has the
  corresponding entitlement or recovery context.
- The billing route remains reachable directly for activation, recovery, and
  explicit commercial handoff.
- Hosted mode can still opt in to promotional/commercial prompts through the
  presentation policy contract.

## Proof

- `npm --prefix frontend-modern test -- --run src/stores/__tests__/sessionPresentationPolicy.test.ts src/stores/__tests__/license.test.ts src/components/Settings/__tests__/ProLicensePanel.test.tsx src/components/Settings/__tests__/settingsNavigation.integration.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/Settings/__tests__/AISettings.test.tsx src/pages/__tests__/AIIntelligence.test.tsx src/components/shared/__tests__/HistoryChart.test.tsx src/components/shared/__tests__/TrialBanner.test.tsx src/components/Settings/__tests__/useReportingPanelState.test.ts src/components/Settings/__tests__/ReportingPanel.test.tsx src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx src/components/Settings/__tests__/AgentProfilesPanel.test.tsx src/features/patrol/__tests__/patrolCommercialBoundary.test.ts`
  - Result: pass, 14 files and 195 tests.
- `npm --prefix frontend-modern run type-check`
  - Result: pass.
- `GOTOOLCHAIN=go1.25.9+auto go test ./internal/api -run 'TestContract_SecurityStatusPresentationPolicyDefaultsHideUpgradeOutsideHosted|TestContract_SecurityStatusIncludesSessionCapabilitiesDemoMode|TestSecurityStatusIncludesDemoModeSessionCapabilities|Test.*Trial|Test.*HostedEntitlementRefresh|TestContract_SelfHostedCommunityEntitlementsJSONSnapshot|TestContract_SelfHostedCommunityRuntimeCapabilitiesJSONSnapshot' -count=1`
  - Result: pass.
- `GOTOOLCHAIN=go1.25.9+auto go test ./internal/cloudcp ./internal/cloudcp/stripe -count=1`
  - Result: pass.
- `PULSE_E2E_USE_LOCAL_BACKEND=1 npm --prefix tests/integration test -- tests/58-self-hosted-trial-rate-limit-ui.spec.ts --project=chromium`
  - Result: pass in Chromium against a managed local backend.

## Follow-Up Copy Audit

The 2026-04-25 copy audit removed lingering trial-first language from the
ordinary self-hosted quickstart reason, settings privacy labels, shared
self-hosted plan presentation helpers, and current commercial strategy docs.
The trial transport remains available for hosted/support handoffs, but default
self-hosted runtime copy now points users to BYOK Patrol or explicit
commercial/entitlement handoff paths instead of asking them to start a trial.

- `npm --prefix frontend-modern test -- src/components/Settings/__tests__/AISettings.test.tsx src/components/Settings/__tests__/GeneralSettingsPanel.guardrails.test.ts src/utils/__tests__/aiSettingsPresentation.test.ts src/utils/__tests__/systemSettingsPresentation.test.ts src/utils/__tests__/relayPresentation.test.ts src/utils/__tests__/selfHostedPlans.test.ts src/utils/__tests__/licensePresentation.test.ts src/api/__tests__/patrol.test.ts src/pages/__tests__/AIIntelligence.test.tsx`
  - Result: pass, 9 files and 86 tests.
- `npm --prefix frontend-modern run type-check`
  - Result: pass.
- `go test ./internal/ai -run 'Test.*Quickstart|TestQuickstart|TestService.*Quickstart' -count=1`
  - Result: pass.
- `go test ./internal/config -count=1`
  - Result: pass.
- `PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 npm --prefix tests/integration test -- tests/58-self-hosted-trial-rate-limit-ui.spec.ts --project=chromium`
  - Result: pass in Chromium against a managed local backend; the proof now also asserts that `Start Trial`, `free 14-day trial`, and `Open hosted handoff` do not appear in the default self-hosted UI.
- `python3 scripts/release_control/documentation_currentness_test.py`
  - Result: pass.
- `python3 scripts/release_control/release_promotion_policy_test.py`
  - Result: pass after regenerating the static blocked RC-to-GA record from current branch facts without creating any release, tag, or promotion artifact.

## Known Unrelated State

- Repository-wide frontend format check still reports pre-existing warnings
  outside this slice.
- `frontendResourceTypeBoundaries.test.ts` still has a pre-existing Recovery
  boundary failure unrelated to the paid-services visibility policy.
