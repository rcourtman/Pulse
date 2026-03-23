# Cloud Paid Contract

## Contract Metadata

```json
{
  "subsystem_id": "cloud-paid",
  "lane": "L3",
  "contract_file": "docs/release-control/v6/internal/subsystems/cloud-paid.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
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
18. `pkg/licensing/monitored_system_limit.go`
19. `internal/cloudcp/entitlements/service.go`
20. `internal/cloudcp/registry/registry.go`
21. `internal/cloudcp/account/tenant_handlers.go`
22. `internal/cloudcp/stripe/provisioner.go`
23. `internal/hosted/provisioner.go`
24. `frontend-modern/src/App.tsx`
25. `frontend-modern/src/AppLayout.tsx`
26. `frontend-modern/src/useAppRuntimeState.ts`
27. `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
28. `frontend-modern/src/components/Dashboard/useRelayOnboardingCardState.ts`
29. `frontend-modern/src/components/Commercial/MonitoredSystemDefinitionDisclosure.tsx`
30. `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
31. `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`
32. `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
33. `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`
34. `frontend-modern/src/components/Settings/MonitoredSystemLedgerPanel.tsx`
35. `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
36. `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
37. `frontend-modern/src/components/Settings/CommercialBillingSections.tsx`
38. `frontend-modern/src/components/Settings/SelfHostedCommercialActivationSection.tsx`
39. `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
40. `frontend-modern/src/components/Settings/RelayPairingSection.tsx`
41. `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
42. `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
43. `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
44. `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
45. `frontend-modern/src/pages/CloudPricing.tsx`
46. `frontend-modern/src/pages/PricingV6.tsx`
47. `frontend-modern/src/utils/apiClient.ts`
48. `frontend-modern/src/utils/cloudPlans.ts`
49. `frontend-modern/src/utils/commercialBillingModel.ts`
50. `frontend-modern/src/utils/selfHostedPlans.ts`

## Shared Boundaries

1. `internal/api/licensing_bridge.go` shared with `api-contracts`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
2. `internal/api/licensing_handlers.go` shared with `api-contracts`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
3. `internal/api/payments_webhook_handlers.go` shared with `api-contracts`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
4. `internal/api/public_signup_handlers.go` shared with `api-contracts`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.

## Extension Points

1. Add or change limits through `pkg/licensing/`
2. Add or change hosted entitlement issuance through `internal/cloudcp/entitlements/service.go`
3. Add or change control-plane plan storage through `internal/cloudcp/registry/registry.go`
4. Add or change MSP account-scoped workspace provisioning entry handlers through `internal/cloudcp/account/tenant_handlers.go`
5. Add or change Stripe provisioning plan resolution through `internal/cloudcp/stripe/provisioner.go`
6. Add or change activation/grant lifecycle through `pkg/licensing/service.go`, `pkg/licensing/grant_refresh.go`, and `pkg/licensing/revocation_poll.go`
7. Add or change license-server transport through `pkg/licensing/license_server_client.go`
8. Add or change encrypted activation persistence through `pkg/licensing/persistence.go` and `pkg/licensing/activation_store.go`
9. Add or change hosted trial token semantics through `pkg/licensing/trial_activation.go`
10. Add or change hosted signup provisioning through `internal/hosted/provisioner.go`
11. Add or change hosted billing-admin presentation through `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`, `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`, and `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
12. Add or change shared commercial plan/usage presentation through `frontend-modern/src/components/Settings/CommercialBillingSections.tsx` and `frontend-modern/src/utils/commercialBillingModel.ts`
13. Add or change organization billing and usage presentation through `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`, and `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
14. Add or change self-hosted Pro activation, trial, and entitlement actions through `frontend-modern/src/components/Settings/ProLicensePanel.tsx`, `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`, `frontend-modern/src/components/Settings/SelfHostedCommercialActivationSection.tsx`, and `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
15. Add or change paid relay settings and onboarding presentation through `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`, `frontend-modern/src/components/Settings/RelayPairingSection.tsx`, `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`, `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`, and `frontend-modern/src/components/Dashboard/useRelayOnboardingCardState.ts`
16. Add or change cloud plan presentation through `frontend-modern/src/pages/CloudPricing.tsx`
17. Add contract tests where runtime and pricing need to stay aligned
18. Add or change hosted browser org-context bootstrap through `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, `frontend-modern/src/useAppRuntimeState.ts`, and `frontend-modern/src/utils/apiClient.ts`

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
must canonicalize and `limits.max_monitored_systems` must reconcile to the authoritative
per-plan contract rather than preserving stale ad hoc values.
That same persisted billing boundary now also applies to hosted entitlement
lease secrets: `internal/config/billing_state.go` may keep `EntitlementJWT`
and `EntitlementRefreshToken` in runtime billing state, but `billing.json` may
not persist either value raw. Canonical persistence must keep the hosted lease
JWT and refresh token encrypted at rest, rewrite legacy plaintext billing files
on load, and drop those secrets instead of preserving plaintext-at-rest billing
state if encryption cannot be established. Empty/no-secret billing state may
not auto-create new crypto state just to add integrity metadata; no-key
graceful degradation remains canonical until a real billing secret or real
encryption key exists.
That same hosted billing boundary also owns base-path resolution:
`internal/config/billing_state.go` and
`internal/api/payments_webhook_handlers.go` must derive their base data
directory from the shared runtime data-dir helper in
`internal/config/config.go` instead of each carrying a private `/etc/pulse`
fallback. Hosted billing leases, webhook dedupe state, and customer indexes
must therefore follow the same configured runtime data-dir authority as the
rest of the product.
The same secret-at-rest rule also applies to activation state persistence:
`pkg/licensing/activation_store.go` may keep `InstallationToken` and
`GrantJWT` in runtime activation state, but `activation.enc` may accept
plaintext only as migration input. Once a legacy plaintext activation file can
be read, canonical persistence must rewrite encrypted storage immediately on
load instead of treating plaintext as a steady-state runtime path.
That same rule also applies to the canonical local license state in
`pkg/licensing/persistence.go`: `license.enc` may carry the commercial license
key and grace-period metadata in runtime state, but a legacy plaintext
`license.enc` may only serve as migration input. Once it can be read,
canonical persistence must rewrite encrypted storage immediately on load
instead of treating plaintext licensing state as a valid steady-state path.
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
lease `limits.max_monitored_systems` to the authoritative per-plan contract instead of
trusting stale embedded values. They also must not fabricate `plan_version`
from bare `subscription_state` when the signed lease claim label is absent.
The control-plane registry is also canonical: tenant and Stripe-account
`plan_version` rows must canonicalize recognized Cloud aliases on read and
write so stored legacy values cannot re-enter provisioning, entitlement, or
limit-enforcement fallbacks.
JWT-backed entitlement claims are also canonical: when runtime evaluation uses
claim `plan_version` and `limits`, recognized Cloud plan aliases must
canonicalize and `max_monitored_systems` must reconcile to the authoritative per-plan
contract instead of trusting stale embedded claim values. When a Cloud/MSP
claim arrives without a recognized plan label, runtime must preserve the
missing/unknown `plan_version` metadata but still fail closed on `max_monitored_systems`
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
The self-hosted commercial counted unit is now also locked to monitored
systems rather than agent installs. `max_monitored_systems` is the live
runtime and UI contract, while legacy `max_agents` / `max_nodes` aliases are
decode-only compatibility inputs at the storage or grant boundary. Runtime
enforcement, entitlement payload `current` usage, checkout/activation flows,
and upgrade messaging must all treat the cap as deduped top-level monitored
systems across agent, API, and Kubernetes views.
The monitored-system ledger settings surface now also has to explain those
count decisions. Commercial usage UI may show grouped monitored systems, but
it must render the canonical backend explanation for why one or more
top-level views counted as a single monitored system instead of inventing
support copy or merge heuristics in the frontend.
That billing support surface must also remain readable while mixed-version
clients and servers roll forward: missing explanation payloads may degrade to a
safe generic explanation, but the monitored-system ledger must never fail the
page or hide counted systems because the nested support details are absent.
That same disclosure copy must stay professional and customer-facing. The
settings ledger may expose counting details and included collection paths, but
it must avoid informal debug-style labels or ad hoc wording that makes the
commercial usage surface feel provisional.
Frontend billing/admin surfaces must not synthesize `plan_version` from
subscription lifecycle state. When a hosted billing record lacks a plan label,
the UI must preserve that absence instead of fabricating values like `active`
or `suspended` into the canonical plan field.
The hosted billing-admin settings surface is now part of the explicit
cloud-paid ownership model as well. Changes to
`frontend-modern/src/components/Settings/BillingAdminPanel.tsx` must carry this
contract and the dedicated billing-admin proof file instead of remaining an
unowned consumer of hosted billing state.
That hosted billing-admin owner is now intentionally split by responsibility:
`frontend-modern/src/components/Settings/BillingAdminPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
owns the hosted organization list, billing-state cache, preload, and
subscription update runtime, and
`frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`
owns the tenant grid plus expanded JSON presentation. Future hosted billing
admin changes must extend that split instead of pulling hosted state and
mutation flow back into the panel render shell.
The organization billing settings surface now follows the same rule. Changes
to `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` must
carry this contract and the dedicated organization-billing proof file instead
of remaining an unowned consumer of plan tier, entitlement limits, and
usage-versus-cap presentation.
That same billing surface now also normalizes org scope through
`frontend-modern/src/utils/orgScope.ts` before it builds per-tenant state, so
cache keys and tenant lookups do not keep a local `getOrgID() || 'default'`
fallback in the hosted billing UI.
That hosted organization billing owner is now intentionally split by role:
`frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
owns org-scoped billing/runtime lifecycle and plan/usage model wiring, and
`frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`
owns the loading skeleton. Future billing work must extend that split instead
of pulling org-switch lifecycle and hosted entitlement reads back into the
panel render shell.
The self-hosted and hosted billing surfaces now also share a canonical
commercial page shell and plan/usage model. `ProLicensePanel.tsx` and
`OrganizationBillingPanel.tsx` may still differ in deployment-specific actions
and context, but `CommercialBillingSections.tsx` and
`frontend-modern/src/utils/commercialBillingModel.ts` now own the shared
commercial information architecture. Future billing work must extend that
shared shell/model first instead of letting self-hosted Pulse Pro and hosted
organization billing drift back into parallel local layouts or vocabularies.
Hosted tenant browser bootstrap is part of that same cloud-paid boundary as
well. After control-plane or magic-link handoff, the browser client must
preserve the tenant-scoped `pulse_org_id` context that the server issued
instead of clearing it on first page load or collapsing back to `default`
simply because hosted tenants do not expose self-hosted multi-tenant admin
capabilities. Hosted runtime entry must therefore treat tenant org context as
infrastructure state, not a paid UI toggle that can be discarded during app
bootstrap.
That hosted browser bootstrap owner is now intentionally split by role:
`frontend-modern/src/App.tsx` is the route/provider entry shell,
`frontend-modern/src/useAppRuntimeState.ts` owns authentication, org bootstrap,
theme synchronization, and authenticated runtime startup, and
`frontend-modern/src/AppLayout.tsx` owns authenticated cloud-aware chrome such
as org switching and kiosk-safe navigation. Future hosted browser bootstrap
work must extend that split rather than pulling org bootstrap and app chrome
back into one monolithic route component.
That same route/provider shell must stay page-oriented as well: `App.tsx`
should lazy-load route shells like `frontend-modern/src/pages/Storage.tsx`
and `frontend-modern/src/pages/Operations.tsx`
instead of wiring product-surface components such as
`frontend-modern/src/components/Storage/Storage.tsx` directly into the router,
so hosted bootstrap ownership stays at the app boundary rather than leaking
route concerns back into feature components.
The Pro license settings surface now follows the same rule as well. Changes to
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` must carry this
contract and the dedicated Pro-license proof file instead of remaining an
unowned consumer of activation, trial eligibility, entitlement capability, and
plan-term presentation.
That owned Pro-license boundary is now intentionally split by role:
`frontend-modern/src/components/Settings/ProLicensePanel.tsx` is the settings
shell, `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
owns activation, trial, route-scoped notices, and commercial plan runtime
state, and `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
owns the plan/read-model presentation. Future Pro-license work must extend
that split instead of pulling route state, entitlement derivation, and retry
lifecycle back into the shell component.
That owned presentation boundary includes the settings shell itself: the
top-level Pulse Pro surface must keep its page-shell title and leading
SettingsPanel title aligned so commercial activation, trial, and pricing state
do not present as one surface in navigation and a differently named surface in
the actual paid settings UI.
Paid Pulse Pro v5 grandfathering is now part of that same canonical boundary:
when a recurring v5 customer migrates into v6, billing persistence,
entitlement evaluation, renewal handling, and Pro-license presentation must
preserve the customer's existing recurring price identity instead of silently
rewriting them onto current v6 retail pricing.
That continuity rule cannot depend on webhook metadata being perfect. The
canonical Stripe price-to-plan lookup in `pkg/licensing/features.go` and
`pkg/licensing/stripe_subscription.go` must recognize the still-renewing
grandfathered recurring v5 and legacy v1 Stripe price IDs directly, so a real
`customer.subscription.updated` event that omits `metadata.plan_version` still
resolves to `v5_pro_monthly_grandfathered` or
`v5_pro_annual_grandfathered` instead of falling back to an opaque
`stripe_price:*` marker.
That Pro-license presentation rule is explicit UX, not only hidden metadata:
when a migrated recurring v5 plan is active or in grace, the settings surface
must render plan terms and a continuity notice that makes it clear the
existing recurring price remains in force until cancellation.
The self-hosted commercial presentation on that same surface is now locked to
the monitored-system model as well. `ProLicensePanel.tsx`,
`CommercialBillingSections.tsx`, and
`frontend-modern/src/utils/commercialBillingModel.ts` must present current v6
retail capacity as monitored systems rather than agents for Community, Relay,
Pro, and Pro+, while leaving Cloud/MSP pricing semantics unchanged and
preserving grandfathered v5 continuity copy as an explicit boundary policy.
That same pricing boundary now also owns the shared frontend plan-definition
models. `frontend-modern/src/utils/cloudPlans.ts` and
`frontend-modern/src/utils/selfHostedPlans.ts` are the canonical frontend
sources for cloud and self-hosted plan copy, limits, and comparison metadata,
while `frontend-modern/src/pages/CloudPricing.tsx`,
`frontend-modern/src/pages/HostedSignup.tsx`, `frontend-modern/src/pages/PricingV6.tsx`,
and the self-hosted billing settings surfaces must consume those shared owners
instead of redefining retail plan facts or counted-unit policy locally.
That same counted-unit boundary also owns the disclosure rule for retail copy:
default billing and pricing surfaces should use concise monitored-system copy,
while the full counted-unit definition appears only behind explicit disclosure
such as `View counting rules` instead of sitting as always-visible explanatory
chrome.
Those same billing-facing surfaces must also describe the commercial contract in
customer terms: monitored systems, plan limits, subscription status, and
license status. They must not revive legacy `installed-agent` wording or vague
internal nouns like `allocation` once the monitored-system billing model is the
canonical product truth.
The self-hosted pricing page should therefore stay focused on plan cards,
upgrade paths, and the comparison table rather than rendering a separate
counted-unit explainer card beneath the tier grid.
Hosted commercial pages follow the same presentation rule: `CloudPricing.tsx`
and `HostedSignup.tsx` should describe plan scope and setup in concise product
language, not in implementation-detail terms such as internal workspace
provisioning guarantees, control-plane routing mechanics, or narrated
deployment internals.
Those same hosted surfaces should also prefer neutral plan-selection and
sign-in language such as `Choose Starter` or `sign-in link` rather than
marketing-heavy CTA copy or infrastructure-specific transport jargon.
They should also avoid duplicating shared commercial facts across multiple
adjacent blocks on the same screen. Common Cloud inclusions belong in one
shared area rather than being restated inside every plan card, and hosted
signup should not repeat the selected plan facts across the page header, form
card, and plan summary at the same time.
That contract applies to both plan summary labels and upgrade/paywall copy:
current v6 self-hosted pricing may not drift back to the older `$49/yr Relay`,
`$99/yr Pro`, or monitored-system-count marketing drift that contradicts the
locked Community / Relay / Pro / Pro+ model.
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
That relay settings owner is now intentionally split by role as well:
`frontend-modern/src/components/Settings/RelaySettingsPanel.tsx` is the
settings shell, `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
owns relay config/status polling, trial, and pairing runtime, and
`frontend-modern/src/components/Settings/RelayPairingSection.tsx` owns the QR
pairing surface. Future relay settings work must extend that split instead of
pulling polling and QR-generation lifecycle back into the shell component.
The dashboard relay onboarding surface now follows the same rule:
`frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx` is the
dashboard shell, while
`frontend-modern/src/components/Dashboard/useRelayOnboardingCardState.ts`
owns license readiness, relay status polling, snooze state, and trial start
runtime. Future onboarding changes must extend that split instead of pulling
license and relay runtime back into the card shell.
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
Hosted release builds must also accept the trial-activation public key from
runtime environment when `PULSE_HOSTED_MODE=true`, because hosted tenants
receive that verification key from control-plane deployment rather than from
the embedded self-hosted release asset. Otherwise a hosted tenant can mount a
valid lease and still fail its first hosted trial-activation verification path
solely because the release binary refuses the deployed public-key source.
Legacy MSP plan aliases are input-only compatibility shims. Live runtime
defaults, fallback provisioning, entitlement issuance, and limit/workspace
lookups must resolve to canonical `msp_starter` rather than preserving
`msp_hosted_v1` as an active first-class plan name.
Hosted control-plane plan resolution is now part of the enforced ownership
model: changes to hosted entitlement issuance, control-plane registry
canonicalization, or Stripe provisioning plan resolution must carry this
contract and the path-specific proof files that verify those boundaries.
Hosted tenant container bootstrap is part of that same boundary as well: the
control plane may bind-mount billing and handoff files into `/etc/pulse` as
read-only inputs, but runtime startup ownership repair must treat those paths
as immutable and skip `chown` attempts against them instead of aborting tenant
provisioning.
That same immutable-file boundary now also owns write-time runtime ownership:
control-plane provisioning and later billing-state rewrites must leave
`billing.json`, `secrets/handoff.key`, and `.cloud_handoff_key` readable by
the tenant Pulse runtime user at the moment they are written. Fixing startup
`chown` behavior alone is not enough if the mounted files stay `root:root`
after provisioning, because hosted auth handoff and hosted lease reads will
then fail closed inside an otherwise healthy tenant.
Hosted tenant org bootstrap is part of that same runtime boundary. Cloud and
MSP tenant provisioning must seed a canonical tenant-scoped `org.json` under
`orgs/<tenant-id>/` and leave both the directory tree and file readable by the
hosted runtime user; otherwise hosted magic-link handoff can preserve the
correct tenant org cookie while the tenant API still fails closed with
`invalid_org` because no tenant organization metadata exists on disk.
Hosted tenant runtime env is part of the same contract too: provisioned
containers must carry hosted-safe tenant context such as
`PULSE_TENANT_ID=<tenant-id>`, `PULSE_MULTI_TENANT_ENABLED=true`, and an explicit
`PULSE_PUBLIC_URL=https://<tenant-id>.<base-domain>` so the tenant-scoped org
surface is actually enabled after handoff instead of failing closed under a
paid hosted session.
Hosted MSP workspace org seeding is part of that same boundary too:
`internal/cloudcp/account/tenant_handlers.go` owns the authenticated
account-scoped workspace-create entry path, and
`internal/cloudcp/stripe/provisioner.go` owns the underlying workspace/org
bootstrap. When the control plane provisions a new workspace under an existing
account, that boundary must seed `org.OwnerUserID` from the authenticated
creator email when that actor is known on the request path instead of
inferring a canonical owner from membership query order. If the creator is not
available, fallback owner selection must still be deterministic rather than
depending on newest-row ordering in the registry.
Hosted tenant entitlement evaluation is part of that same boundary too: when a
hosted tenant lands in a tenant-scoped org like `t-...`, the runtime must
still honor the instance-level hosted billing lease in `default` until or
unless an org-local billing state exists, rather than collapsing a freshly
provisioned paid tenant into `subscription_required` on first entry.
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
writes must persist authoritative `limits.max_monitored_systems` derived from canonical
plan resolution, and when paid state is revoked they must clear those stored
limits instead of preserving stale paid capacity.
That same monitored-system entitlement boundary also owns the shared operator
warning copy: the limit banner and migration guidance must present the counted
surface as monitored systems, not drift back into agent-install language while
describing non-counted legacy/API-connected resources.
That same webhook boundary now also owns request-lifetime decoupling for
checkout provisioning: long-running `checkout.session.completed` tenant
provisioning must complete under an explicit background timeout instead of
depending on the inbound Stripe request context surviving long enough for first
boot and health polling.
