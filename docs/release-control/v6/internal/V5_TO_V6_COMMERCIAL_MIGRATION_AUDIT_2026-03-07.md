# Pulse v5 -> v6 Commercial Migration Audit

Date: 2026-03-07
Owner: Pulse v6 release control
Scope: Self-hosted v5 -> v6 commercial and licensing bridge in `pulse`

## Canonical inputs used

- Human source: `docs/release-control/v6/SOURCE_OF_TRUTH.md`
- Machine source: `docs/release-control/v6/status.json`

Locked v6 contract from those sources:

1. Trial authority in v6 is SaaS-controlled. `POST /api/license/trial/start` must initiate hosted signup only.
2. The local runtime may only redeem signed hosted trial activation tokens via `/auth/trial-activate`.
3. v6 may auto-exchange persisted v5 Pro/Lifetime licenses on upgrade startup.
4. v6 may accept valid v5 Pro/Lifetime keys in the activation flow.
5. Paid Pulse Pro v5 recurring customers keep their existing recurring price after migration until they cancel; any return after cancellation uses current v6 pricing.

## Current bridge surface inspected

- Startup auto-exchange: `internal/api/licensing_handlers.go`, `internal/api/licensing_handlers_auto_migrate_test.go`
- Activation flow: `pkg/licensing/service.go`, `internal/api/license_handlers_test.go`, `pkg/licensing/service_activate_test.go`
- Entitlement payload + trial eligibility: `internal/api/subscription_entitlements.go`, `pkg/licensing/entitlement_payload.go`, `internal/api/entitlement_handlers_test.go`
- Hosted trial start and callback: `internal/api/licensing_handlers.go`, `internal/api/trial_handlers_test.go`, `internal/api/hosted_lifecycle_integration_test.go`
- Upgrade UI messaging: `frontend-modern/src/components/Settings/ProLicensePanel.tsx`, `frontend-modern/src/components/Settings/__tests__/ProLicensePanel.test.tsx`, `frontend-modern/src/stores/license.ts`
- Upgrade integration fixture: `tests/migration/v5_full_upgrade_test.go`

## Truth table: incoming v5 commercial state -> required v6 behavior

| Incoming v5 state | Starting persisted state | Expected v6 entitlement result | Expected UI state / message | User action required | Hosted service involved |
|---|---|---|---|---|---|
| Fresh/free v5 install | No `license.enc`, no `activation.enc` | Free / expired entitlement only | Standard free-state upgrade UI; Pro trial CTA allowed | No | No |
| Already on v6 activation model | `activation.enc` present, optional stale `license.enc` | Keep current v6 activation/grant; do not re-exchange legacy file | Active paid state with current plan details; no migration prompt | No | No at startup |
| Paid v5 Pro/Lifetime, exchange succeeds on startup | Valid v5 `license.enc`, no `activation.enc` | Auto-exchange into active v6 activation/grant; preserve grandfathered recurring-price identity and `plan_version`; keep legacy key on disk for downgrade fallback | Paid state is live immediately; if grandfathered, show migrated plan terms and legacy-price continuity | No | Yes, license exchange endpoint |
| Paid v5 Pro/Lifetime, exchange fails transiently | Valid v5 `license.enc`, no `activation.enc`, exchange unavailable/5xx/network | Do not silently collapse to ordinary free/trial-eligible state; mark migration as pending/blocked; preserve legacy key | Explicit migration-needed notice: paid v5 key detected, automatic exchange did not complete, retry activation from this instance; no new-trial CTA | Yes, retry activation or retrieve v6 activation key | Yes, exchange endpoint unavailable |
| Paid v5 Pro/Lifetime, exchange rejected permanently | Valid-looking v5 `license.enc`, no `activation.enc`, exchange returns invalid/expired/unsupported | Do not grant paid entitlements; preserve enough state to explain the failure; do not offer a misleading fresh trial as if no paid key existed | Explicit migration failure notice with invalid/expired/unsupported wording; direct user to activate with current v6 key or correct v5 key | Yes | Yes, exchange endpoint rejects key |
| Manual activation with valid v5 Pro/Lifetime key | User pastes v5 key into v6 panel | Exchange into active v6 activation/grant; persist activation state; preserve legacy key for downgrade fallback | Success message should make it clear the v5 key was migrated to v6 | Yes, one-time manual paste | Yes, exchange endpoint |
| Manual activation with invalid/expired/unsupported v5-like key | User pastes JWT-like legacy key into v6 panel | No entitlement change | Clear error message: not a valid v6 activation key or supported v5 Pro/Lifetime key | Yes | Yes, exchange endpoint |

## Related hosted-trial flows after upgrade

These are not incoming paid-license migration states, but they are part of the same commercial bridge and must stay coherent for upgraded v5 users.

| Post-upgrade state | Starting persisted state | Expected v6 entitlement result | Expected UI state / message | User action required | Hosted service involved |
|---|---|---|---|---|---|
| Free/eligible org starts v6 trial | No active paid state; no prior `trial_started_at` | No immediate local trial minting from `/api/license/trial/start`; response must redirect into hosted signup | User leaves Pulse for hosted signup | Yes | Yes, hosted signup |
| Hosted trial callback succeeds | Signed token + valid initiation token | Lease-backed trial entitlement becomes active; local billing state is lease cache only | `/settings/system-pro?trial=activated` notice and live trial countdown | No further action | Yes, hosted signup + lease redemption |
| Hosted trial callback invalid/replayed/unavailable/ineligible | Invalid or stale callback/token state | No new paid entitlement | Explicit result banner based on `trial` query (`invalid`, `replayed`, `unavailable`, `ineligible`) | Usually yes | Yes |

## Comparison to current implementation

### What is already correct

1. Startup auto-exchange exists for persisted legacy JWT-style licenses and preserves the old key for downgrade fallback.
2. Manual activation accepts v6 activation keys and also exchanges v5 JWT-style keys outside dev mode.
3. Migrated `plan_version` survives into `status` and `entitlements`, and the Pro panel renders migrated plan terms without repricing recurring v5 customers.
4. `POST /api/license/trial/start` does not mint local trial state; it returns `trial_signup_required` with a hosted action URL.
5. `/auth/trial-activate` redeems a signed hosted token, stores lease-backed billing state, and redirects with an explicit result code.

### Highest-risk gaps

1. Auto-exchange failure is not represented as explicit state.
   Current behavior in `internal/api/licensing_handlers.go` logs the exchange failure and keeps running. After that, `svc.Status()` and `GET /api/license/entitlements` collapse to ordinary free-state behavior because there is no machine-readable "migration pending" or "migration failed" contract.

2. A paid v5 migrator can be misclassified as trial-eligible.
   Trial eligibility only checks active v6 license state plus billing state. If a valid persisted v5 paid key fails to exchange and no billing state exists yet, the org becomes `trial_eligible=true` even though the correct contract is "paid migration blocked, retry exchange". This is the largest commercial coherence risk.

3. Upgrade-time UI messaging is not state-driven.
   The only migration-specific frontend notice is a textarea heuristic in `frontend-modern/src/components/Settings/ProLicensePanel.tsx` that treats any three-segment key as "Legacy v5 license detected". There is no startup banner or entitlement-state notice for "persisted v5 paid key detected but exchange failed".

4. Success and failure copy does not distinguish migration outcomes strongly enough.
   Manual migration success returns the generic message `License activated successfully`. That is functional, but it does not confirm that the pasted v5 key was exchanged into the v6 activation model. The failure path is better, but it still depends on a manual paste instead of a detected startup state.

5. The migration test suite is happy-path heavy.
   `tests/migration/v5_full_upgrade_test.go` covers only the startup success case for a persisted v5 Lifetime key. There is no full-upgrade negative-path contract for exchange failure, rejection, or UI/entitlement behavior after failure.

## Exact missing test file paths

Add or extend tests in these exact files:

1. `tests/migration/v5_full_upgrade_test.go`
   Add persisted-v5-paid-license upgrade scenarios where exchange is transiently unavailable and permanently rejected.

2. `internal/api/licensing_handlers_auto_migrate_test.go`
   Add startup auto-exchange negative-path tests proving legacy key preservation plus explicit migration-pending behavior once the new contract exists.

3. `internal/api/entitlement_handlers_test.go`
   Add payload contract tests for the new migration state and for `trial_eligible=false` while a paid v5 migration is pending or failed.

4. `internal/api/license_handlers_test.go`
   Add manual activation tests for legacy exchange rejection classes (expired, unsupported, invalid) and migration-specific success messaging.

5. `frontend-modern/src/components/Settings/__tests__/ProLicensePanel.test.tsx`
   Add UI tests for startup migration-pending / migration-failed notices and suppression of the Pro trial CTA during those states.

6. `frontend-modern/src/stores/__tests__/license.test.ts`
   Add store tests for any new commercial-migration fields surfaced by `GET /api/license/entitlements`.

## Recommended implementation sequence

1. Add an explicit v6-owned migration contract to the entitlements payload.
   Recommended shape: a dedicated field such as `commercial_migration` with `state`, `source`, `reason`, and `recommended_action`. Do not overload `has_migration_gap`; it already means legacy infrastructure connection drift.

2. Persist migration-pending state when startup auto-exchange fails.
   The runtime needs durable state that says: a legacy paid key exists, exchange did not complete, and trial start must be suppressed until the user resolves migration or clears the key intentionally.

3. Make trial eligibility migration-aware.
   `trial_eligible` must be false whenever a paid v5 migration is pending or has failed but remains unresolved.

4. Drive the Pro panel from the new contract.
   Show an explicit upgrade-time notice for startup exchange failure and a clearer success notice for manual v5->v6 migration. Remove reliance on the current "three JWT segments means legacy v5" heuristic for anything beyond a weak input hint.

5. Backfill the negative-path tests listed above.
   The new contract should be locked in both backend payload tests and frontend rendering tests before any broader commercial-path polish.

## Audit verdict

The v5->v6 bridge is implemented for the success path, but it is not yet an explicit v6-owned contract on the failure path. Until v6 can represent "paid v5 migration blocked" as a first-class entitlement/UI state, upgrade safety is incomplete and the commercial path can drift into the wrong offer and the wrong message.
