# Cloud Paid Contract

## Contract Metadata

```json
{
  "subsystem_id": "cloud-paid",
  "lane": "L3",
  "contract_file": "docs/release-control/v6/subsystems/cloud-paid.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own cloud plan/version semantics, entitlement limits, hosted billing/runtime
agreement, and cloud-specific enforcement rules.

## Canonical Files

1. `pkg/licensing/features.go`
2. `pkg/licensing/billing_state_normalization.go`
3. `pkg/licensing/database_source.go`
4. `pkg/licensing/evaluator.go`
5. `pkg/licensing/models.go`
6. `pkg/licensing/activation_types.go`
7. `pkg/licensing/token_source.go`
8. `pkg/licensing/entitlement_payload.go`
9. `pkg/licensing/hosted_subscription.go`
10. `pkg/licensing/service.go`
11. `pkg/licensing/grant_refresh.go`
12. `pkg/licensing/revocation_poll.go`
13. `pkg/licensing/license_server_client.go`
14. `pkg/licensing/persistence.go`
15. `pkg/licensing/activation_store.go`
16. `pkg/licensing/trial_activation.go`
17. `pkg/licensing/stripe_subscription.go`
18. `internal/cloudcp/entitlements/service.go`
19. `internal/cloudcp/registry/registry.go`
20. `internal/cloudcp/stripe/provisioner.go`
21. `internal/hosted/provisioner.go`
22. `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
23. `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
24. `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
25. `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
26. `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
27. `frontend-modern/src/pages/CloudPricing.tsx`

## Shared Boundaries

1. `internal/api/licensing_bridge.go` shared with `api-contracts`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
2. `internal/api/licensing_handlers.go` shared with `api-contracts`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
3. `internal/api/payments_webhook_handlers.go` shared with `api-contracts`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
4. `internal/api/public_signup_handlers.go` shared with `api-contracts`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.

## Extension Points

1. Add or change limits through `pkg/licensing/`
2. Add or change hosted entitlement issuance through `internal/cloudcp/entitlements/service.go`
3. Add or change control-plane plan storage through `internal/cloudcp/registry/registry.go`
4. Add or change Stripe provisioning plan resolution through `internal/cloudcp/stripe/provisioner.go`
5. Add or change activation/grant lifecycle through `pkg/licensing/service.go`, `pkg/licensing/grant_refresh.go`, and `pkg/licensing/revocation_poll.go`
6. Add or change license-server transport through `pkg/licensing/license_server_client.go`
7. Add or change encrypted activation persistence through `pkg/licensing/persistence.go` and `pkg/licensing/activation_store.go`
8. Add or change hosted trial token semantics through `pkg/licensing/trial_activation.go`
9. Add or change hosted signup provisioning through `internal/hosted/provisioner.go`
10. Add or change hosted billing-admin presentation through `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
11. Add or change organization billing and usage presentation through `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
12. Add or change Pro license activation, trial, and entitlement presentation through `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
13. Add or change paid relay settings and onboarding presentation through `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` and `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
14. Add or change cloud plan presentation through `frontend-modern/src/pages/CloudPricing.tsx`
15. Add contract tests where runtime and pricing need to stay aligned

## Forbidden Paths

1. New ad hoc plan names in runtime or UI
2. Silent aliases between old and new limit keys in live runtime paths
3. Pricing/UI claims that are not enforced by runtime entitlements

## Completion Obligations

1. Update this contract when cloud plan semantics change
2. Update runtime and frontend tests together when plan/limit rules move
3. Add or tighten drift tests when a pricing/runtime mismatch is fixed

## Current State

Cloud paid readiness is materially behind architecture work. The main concern is
contract coherence between pricing, entitlements, and runtime enforcement.
Legacy Cloud plan aliases are now expected to canonicalize to the `cloud_*`
contract not only when Stripe metadata is parsed, but also when persisted plan
versions are consumed at hosted entitlement and workspace-limit enforcement
boundaries.
Persisted billing state is now also part of that canonical boundary: when a
recognized Cloud/MSP plan version is loaded or saved, the stored `plan_version`
must canonicalize and `limits.max_agents` must reconcile to the authoritative
per-plan contract rather than preserving stale ad hoc values.
Hosted entitlement-source loading follows the same rule: `DatabaseSource` must
normalize persisted Cloud/MSP plan aliases and legacy limit keys before runtime
evaluation, but it must not fabricate a canonical `plan_version` from bare
subscription lifecycle state when the stored plan label is absent.
Stripe control-plane fallback paths are also part of the boundary: when
subscription or workspace provisioning logic reuses an already stored
`plan_version`, it must canonicalize that value before persisting tenant,
Stripe-account, or billing-state updates.
Signed hosted entitlement leases are part of the same boundary: lease signing
and verification must canonicalize recognized Cloud plan aliases and reconcile
lease `limits.max_agents` to the authoritative per-plan contract instead of
trusting stale embedded values. They also must not fabricate `plan_version`
from bare `subscription_state` when the signed lease claim label is absent.
The control-plane registry is also canonical: tenant and Stripe-account
`plan_version` rows must canonicalize recognized Cloud aliases on read and
write so stored legacy values cannot re-enter provisioning, entitlement, or
limit-enforcement fallbacks.
JWT-backed entitlement claims are also canonical: when runtime evaluation uses
claim `plan_version` and `limits`, recognized Cloud plan aliases must
canonicalize and `max_agents` must reconcile to the authoritative per-plan
contract instead of trusting stale embedded claim values. When a Cloud/MSP
claim arrives without a recognized plan label, runtime must preserve the
missing/unknown `plan_version` metadata but still fail closed on `max_agents`
instead of drifting to an unlimited tier default.
Activation-grant translation is part of the same boundary: when relay/license
server grants enter the local claims model, Cloud plan keys and lifecycle state
must still resolve through the canonical entitlement claim accessors rather
than becoming a parallel truth path.
The legacy-license exchange transport is part of that same activation boundary:
`pkg/licensing/activation_types.go` and `pkg/licensing/license_server_client.go`
must treat `legacy_license_token` as the canonical v6 request field for
`POST /v1/licenses/exchange`, while accepting `legacy_license_key` only as a
backward-compatible decode alias for older local stubs and historical test
fixtures. Future exchange-path changes must not reintroduce a split contract
where the shared Pulse runtime and the real `pulse-pro/license-server` disagree
on the activation payload shape.
Frontend billing/admin surfaces must not synthesize `plan_version` from
subscription lifecycle state. When a hosted billing record lacks a plan label,
the UI must preserve that absence instead of fabricating values like `active`
or `suspended` into the canonical plan field.
The hosted billing-admin settings surface is now part of the explicit
cloud-paid ownership model as well. Changes to
`frontend-modern/src/components/Settings/BillingAdminPanel.tsx` must carry this
contract and the dedicated billing-admin proof file instead of remaining an
unowned consumer of hosted billing state.
The organization billing settings surface now follows the same rule. Changes
to `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` must
carry this contract and the dedicated organization-billing proof file instead
of remaining an unowned consumer of plan tier, entitlement limits, and
usage-versus-cap presentation.
The Pro license settings surface now follows the same rule as well. Changes to
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` must carry this
contract and the dedicated Pro-license proof file instead of remaining an
unowned consumer of activation, trial eligibility, entitlement capability, and
plan-term presentation.
Paid Pulse Pro v5 grandfathering is now part of that same canonical boundary:
when a recurring v5 customer migrates into v6, billing persistence,
entitlement evaluation, renewal handling, and Pro-license presentation must
preserve the customer's existing recurring price identity instead of silently
rewriting them onto current v6 retail pricing.
That Pro-license presentation rule is explicit UX, not only hidden metadata:
when a migrated recurring v5 plan is active or in grace, the settings surface
must render plan terms and a continuity notice that makes it clear the
existing recurring price remains in force until cancellation.
Cancellation is the explicit boundary for that policy. Once a grandfathered v5
recurring subscription is canceled, any later return must resolve through the
current v6 pricing contract rather than reviving the legacy recurring rate.
The canonical cross-repo manual drill for that boundary is
`docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`.
The paid relay settings and onboarding surfaces are now part of that same
ownership model. Changes to
`frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` and
`frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx` must carry
this contract and the dedicated relay frontend proof files instead of
remaining unowned consumers of relay licensing and onboarding state.
That relay pairing boundary now also includes ephemeral device-token lifecycle:
when the settings surface generates a mobile pairing QR, it must mint a fresh
scoped API token for that pairing attempt, fetch the onboarding payload through
that token context, and tear down superseded or failed pairing tokens instead
of accumulating long-lived hidden credentials.
That same pairing fetch path must stay token-bound instead of ambient-session
bound: the onboarding QR request must carry the freshly minted relay pairing
token explicitly so the returned payload reflects the exact credential that the
mobile device will bootstrap with, not whichever browser session happened to
open the settings page.
The same flow must fail closed if the pairing payload omits the authenticated
relay token state needed by the mobile deep link; pairing UI cannot silently
render a QR that bypasses governed auth ownership.
Relay pairing token presentation is part of that same contract as well: the
settings surface must label those transient credentials distinctly from
long-lived automation tokens so operators can identify and revoke stale mobile
pairing attempts without guessing which credential was created for device
bootstrap.
Relay pairing refresh behavior is part of that lifecycle contract too: a
successful QR refresh must revoke the superseded pairing token once the new
token-backed payload is ready, while a failed refresh must revoke only the new
failed token and keep the previously valid QR visible instead of collapsing the
operator back to an empty pairing state.
Hosted signup provisioning now follows the same rule. Changes to
`internal/api/public_signup_handlers.go` and `internal/hosted/provisioner.go`
must carry this contract and the dedicated hosted-signup provisioning proof
files instead of remaining a split boundary between API handlers and an
unowned hosted runtime helper.
That hosted signup boundary is now also canonical in shape: the public signup
handler owns request validation, trial billing initialization, and magic-link
issuance, while `internal/hosted/provisioner.go` owns the shared org
bootstrap/admin-role assignment and rollback path for hosted signup failures.
Hosted billing-state normalization now follows the same rule: a missing
`plan_version` must remain missing instead of being synthesized from
`subscription_state`, while explicit trial defaults remain explicit.
Legacy MSP plan aliases are input-only compatibility shims. Live runtime
defaults, fallback provisioning, entitlement issuance, and limit/workspace
lookups must resolve to canonical `msp_starter` rather than preserving
`msp_hosted_v1` as an active first-class plan name.
Hosted control-plane plan resolution is now part of the enforced ownership
model: changes to hosted entitlement issuance, control-plane registry
canonicalization, or Stripe provisioning plan resolution must carry this
contract and the path-specific proof files that verify those boundaries.
JWT-backed entitlement claim evaluation and activation-grant translation now
follow the same explicit proof model instead of relying only on the broad cloud
runtime catch-all policy.
Persisted billing-state normalization, hosted database-source loading, Stripe
plan derivation, and the cloud plan/limit tables now follow the same ratchet:
they are expected to move behind path-specific proof routes rather than staying
indistinguishable inside the generic cloud runtime policy.
The runtime entitlement surface now follows the same rule: evaluator/token
source accessors, hosted-subscription validity rules, and frontend entitlement
payload construction should move behind explicit proof routes rather than being
implicitly trusted as part of the catch-all cloud runtime layer.
Cloud/MSP live price IDs are no longer an open fill-in task either. The audit
record `docs/release-control/v6/records/cloud-msp-price-audit-2026-03-13.md`
verified that the 13 canonical Cloud/MSP v6 `price_*` IDs are present in the
governed `pulse-pro` operational docs and license-server env template, and that
each ID resolves to an active live recurring Stripe price object.
Activation service runtime, license-server transport, encrypted activation
persistence, and hosted trial activation now follow the same ratchet. Changes
to `pkg/licensing/service.go`, `pkg/licensing/grant_refresh.go`,
`pkg/licensing/revocation_poll.go`, `pkg/licensing/license_server_client.go`,
`pkg/licensing/persistence.go`, `pkg/licensing/activation_store.go`, and
`pkg/licensing/trial_activation.go` should carry their dedicated proof files
instead of relying only on the generic cloud runtime policy.
The remaining cloud-paid runtime families now follow the same rule as well:
feature/limit primitives, billing and entitlement type shapes, commercial
migration and trial flow, conversion telemetry, host lifecycle tracking, and
public-key/build-mode boundaries should all resolve through explicit proof
routes rather than a package-wide `pkg/licensing/` fallback.
Stripe checkout and subscription webhook persistence now also follows the
canonical Cloud/MSP limit rule: when paid state is granted, billing-state
writes must persist authoritative `limits.max_agents` derived from canonical
plan resolution, and when paid state is revoked they must clear those stored
limits instead of preserving stale paid capacity.
