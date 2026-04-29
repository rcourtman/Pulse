# Pulse v6.0.0-rc.2 changelog evidence

_Scope note: this appendix maps the draft `v6.0.0-rc.2` changelog to the
current `pulse/v6-release` branch after the shipped `v6.0.0-rc.1` tag. It does
not imply that `rc.2` is published yet._

_Line references below were taken from the reviewed `pulse/v6-release` branch
on 2026-04-16._

## 1. Self-hosted core monitoring is no longer paywalled by monitored-system count

- **Claim:** Community, Relay, and Pro now keep self-hosted core monitoring outside monitored-system metering, while public self-hosted checkout is focused on Relay and Pro rather than monitored-system-cap expansion.
- **Why it is true:** The canonical self-hosted monitored-system limits now set Community, Relay, Pro, Pro+, annual Pro, and Lifetime to `0`, which means no monitored-system volume gate in the current self-hosted model. The shared self-hosted plan definitions present Community / Relay / Pro around included core monitoring plus paid extras, and the license server treats Relay and Pro as the public v6 self-hosted checkout tiers while normalizing self-hosted entitlements away from monitored-system metering.
- **Key files:**
  - `pulse@pulse/v6-release pkg/licensing/features.go:62-74`
  - `pulse@pulse/v6-release frontend-modern/src/utils/selfHostedPlans.ts:40-90`
  - `pulse@pulse/v6-release frontend-modern/src/utils/selfHostedPlans.ts:124-145`
  - `pulse-pro@main license-server/public_pricing.go:226-266`
  - `pulse-pro@main license-server/public_pricing.go:379-390`
  - `pulse-pro@main license-server/v6_schema.go:784-794`
  - `pulse-pro@main license-server/v6_store.go:879-888`
- **Surface:** User-visible, operator-visible, commercial/runtime.
- **Consequence:** `rc.2` should be presented as core monitoring outside the self-hosted paid gate rather than as a higher-cap variant of `rc.1`.
- **Current-branch confidence:** High.

## 2. Paid-customer continuity is explicit and uncapped where promised

- **Claim:** Lifetime and active grandfathered recurring v5 Pulse Pro continuity now remain uncapped, and grandfathered recurring runtime payloads no longer emit capped monitored-system or guest limits.
- **Why it is true:** Lifetime now resolves to an unlimited monitored-system contract in the canonical tier map, the billing-state normalization strips `max_monitored_systems` and `max_guests` for grandfathered recurring plan versions, and the entitlement handler proof now fails if a grandfathered recurring payload still emits those bounded limits.
- **Key files:**
  - `pulse@pulse/v6-release pkg/licensing/features.go:64-70`
  - `pulse@pulse/v6-release pkg/licensing/database_source.go:245-249`
  - `pulse@pulse/v6-release internal/api/entitlement_handlers_test.go:456-494`
  - `pulse-pro@main license-server/v6_schema.go:769-780`
  - `pulse-pro@main license-server/v6_store.go:883-888`
- **Surface:** Operator-visible, commercial/runtime.
- **Consequence:** Existing paid users should no longer be told or shown that the intended continuity policy is capped.
- **Current-branch confidence:** High.

## 3. Billing and account surfaces now describe self-hosted upgrades as plan selection plus paid extras

- **Claim:** The in-product billing shell and Pulse Account handoff now frame self-hosted upgrades around plan selection and paid extras rather than monitored-system-cap expansion.
- **Why it is true:** The local licensing handoff now uses the canonical `self_hosted_plan` feature for self-hosted upgrades, the Pulse Account billing view treats both `self_hosted_plan` and the old `max_monitored_systems` alias as the same no-cap self-hosted comparison path, and the shared self-hosted plan presentation copy leads with free core monitoring plus Relay/Pro differentiated features.
- **Key files:**
  - `pulse@pulse/v6-release internal/api/licensing_handlers.go:2036-2069`
  - `pulse@pulse/v6-release internal/cloudcp/portal/frontend/src/billing_view.ts:198-200`
  - `pulse@pulse/v6-release frontend-modern/src/utils/selfHostedPlans.ts:98-122`
- **Surface:** User-visible, operator-visible.
- **Consequence:** Upgrade flows in `rc.2` should not look like users are buying more room for monitoring volume.
- **Current-branch confidence:** High.

## 4. Finite carry-forward cases now describe an admission freeze rather than a hard blackout

- **Claim:** Where Pulse still needs to describe bounded legacy fallback or continuity states, the runtime now treats those states as “existing monitoring continues, new monitored systems are blocked” rather than implying a hard cap blackout.
- **Why it is true:** Grandfathered recurring payloads no longer emit bounded limits at all, and the remaining billing-state normalization plus UI model work only leaves bounded limits for explicit fallback or continuity-verification cases. The shared self-hosted plan model meanwhile treats the standard current retail path as core monitoring included without monitored-system cap language instead of surfacing stale cap-era affordances.
- **Key files:**
  - `pulse@pulse/v6-release pkg/licensing/database_source.go:245-249`
  - `pulse@pulse/v6-release frontend-modern/src/utils/selfHostedPlans.ts:40-90`
  - `pulse@pulse/v6-release frontend-modern/src/utils/selfHostedPlans.ts:124-145`
- **Surface:** User-visible, operator-visible.
- **Consequence:** Over-plan messaging in `rc.2` should be interpreted as legacy continuity or fallback handling, not as the current self-hosted commercial rule.
- **Current-branch confidence:** Medium-high.

## 5. Two early `rc.1` regressions are fixed directly on the release line

- **Claim:** `pulse-agent --version` now exits cleanly, and Proxmox `PVE` / `PBS` / `PMG` settings routes now stay aligned after reload/remount.
- **Why it is true:** The agent CLI now returns `flag.ErrHelp` directly for the version/help path instead of wrapping it as a unified-config failure, and the settings shell now explicitly synchronizes the selected Proxmox platform from canonical deep-link routes with unit and Playwright proof for `pve`, `pbs`, and `pmg`.
- **Key files:**
  - `pulse@pulse/v6-release cmd/pulse-agent/main.go:103-110`
  - `pulse@pulse/v6-release cmd/pulse-agent/main_test.go:1106-1112`
  - `pulse@pulse/v6-release frontend-modern/src/components/Settings/useSettingsNavigation.ts:137-148`
  - `pulse@pulse/v6-release frontend-modern/src/components/Settings/settingsNavigationModel.ts:242-260`
  - `pulse@pulse/v6-release frontend-modern/src/components/Settings/__tests__/useSettingsNavigation.test.tsx:65-79`
  - `pulse@pulse/v6-release tests/integration/tests/43-platform-mock-runtime.spec.ts:233-278`
- **Surface:** Operator-visible.
- **Consequence:** `rc.2` closes two concrete early-feedback regressions instead of only revising pricing and messaging.
- **Current-branch confidence:** High.
