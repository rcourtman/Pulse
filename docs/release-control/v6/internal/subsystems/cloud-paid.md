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
10. `pkg/licensing/dev_mode_features.go`
11. `pkg/licensing/service.go`
12. `pkg/licensing/grant_refresh.go`
13. `pkg/licensing/revocation_poll.go`
14. `pkg/licensing/license_server_client.go`
15. `pkg/licensing/persistence.go`
16. `pkg/licensing/activation_store.go`
17. `pkg/licensing/trial_activation.go`
18. `pkg/licensing/stripe_subscription.go`
19. `pkg/licensing/monitored_system_limit.go`
20. `internal/cloudcp/entitlements/service.go`
21. `internal/cloudcp/registry/registry.go`
22. `internal/cloudcp/account/tenant_handlers.go`
23. `internal/cloudcp/portal/handlers.go`
24. `internal/cloudcp/portal/page.go`
25. `internal/cloudcp/routes.go`
26. `internal/cloudcp/stripe/provisioner.go`
27. `internal/hosted/provisioner.go`
28. `frontend-modern/src/App.tsx`
29. `frontend-modern/src/AppLayout.tsx`
30. `frontend-modern/src/useAppRuntimeState.ts`
31. `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`
32. `frontend-modern/src/components/Dashboard/useRelayOnboardingCardState.ts`
33. `frontend-modern/src/components/Commercial/MonitoredSystemDefinitionDisclosure.tsx`
34. `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`
35. `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`
36. `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`
37. `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`
38. `frontend-modern/src/components/Settings/MonitoredSystemLedgerPanel.tsx`
39. `frontend-modern/src/components/Settings/ProLicensePanel.tsx`
40. `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
41. `frontend-modern/src/components/Settings/CommercialBillingSections.tsx`
42. `frontend-modern/src/components/Settings/SelfHostedCommercialActivationSection.tsx`
43. `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
44. `frontend-modern/src/components/Settings/RelayPairingSection.tsx`
45. `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
46. `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
47. `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
48. `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`
49. `frontend-modern/src/pages/CloudPricing.tsx`
50. `frontend-modern/src/pages/PricingV6.tsx`
51. `frontend-modern/src/utils/apiClient.ts`
52. `frontend-modern/src/utils/cloudPlans.ts`
53. `frontend-modern/src/utils/commercialBillingModel.ts`
54. `frontend-modern/src/utils/monitoredSystemPresentation.ts`
55. `frontend-modern/src/utils/selfHostedPlans.ts`
56. `frontend-modern/src/utils/licensePresentation.ts`
57. `frontend-modern/src/utils/upgradePresentation.ts`

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
5. Add or change the hosted account portal API, task-first browser shell, maintained portal frontend/bundle, or account-scoped workspace/access/billing handoff through `internal/cloudcp/portal/` and `internal/cloudcp/routes.go`
6. Add or change Stripe provisioning plan resolution through `internal/cloudcp/stripe/provisioner.go`
7. Add or change activation/grant lifecycle or dev-mode capability widening through `pkg/licensing/dev_mode_features.go`, `pkg/licensing/service.go`, `pkg/licensing/grant_refresh.go`, and `pkg/licensing/revocation_poll.go`
8. Add or change license-server transport through `pkg/licensing/license_server_client.go`
9. Add or change encrypted activation persistence through `pkg/licensing/persistence.go` and `pkg/licensing/activation_store.go`
10. Add or change hosted trial token semantics through `pkg/licensing/trial_activation.go`
11. Add or change hosted signup provisioning through `internal/hosted/provisioner.go`
12. Add or change hosted billing-admin presentation through `frontend-modern/src/components/Settings/BillingAdminPanel.tsx`, `frontend-modern/src/components/Settings/BillingAdminOrganizationsTable.tsx`, and `frontend-modern/src/components/Settings/useBillingAdminPanelState.ts`
13. Add or change shared commercial plan/usage presentation through `frontend-modern/src/components/Settings/CommercialBillingSections.tsx` and `frontend-modern/src/utils/commercialBillingModel.ts`
14. Add or change organization billing and usage presentation through `frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationBillingLoadingState.tsx`, and `frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts`
15. Add or change self-hosted Pro activation, trial, and entitlement actions through `frontend-modern/src/components/Settings/ProLicensePanel.tsx`, `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`, `frontend-modern/src/components/Settings/SelfHostedCommercialActivationSection.tsx`, and `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
16. Add or change monitored-system ledger presentation through `frontend-modern/src/components/Settings/MonitoredSystemLedgerPanel.tsx`, `frontend-modern/src/components/Commercial/MonitoredSystemDefinitionDisclosure.tsx`, and `frontend-modern/src/utils/monitoredSystemPresentation.ts`
17. Add or change paid relay settings and onboarding presentation through `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`, `frontend-modern/src/components/Settings/RelayPairingSection.tsx`, `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`, `frontend-modern/src/components/Dashboard/RelayOnboardingCard.tsx`, and `frontend-modern/src/components/Dashboard/useRelayOnboardingCardState.ts`
18. Add or change cloud plan presentation through `frontend-modern/src/pages/CloudPricing.tsx`
19. Add contract tests where runtime and pricing need to stay aligned
20. Add or change hosted browser org-context bootstrap through `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, `frontend-modern/src/useAppRuntimeState.ts`, and `frontend-modern/src/utils/apiClient.ts`

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
That same monitored-system presentation boundary now also owns the customer-
facing ledger loading and retry copy through
`frontend-modern/src/utils/monitoredSystemPresentation.ts`, so commercial
usage states do not leak back into lifecycle inventory helpers.
That same cloud-paid boundary also owns the shared entitlement and upgrade
presentation helpers through `frontend-modern/src/utils/licensePresentation.ts`
and `frontend-modern/src/utils/upgradePresentation.ts`. Commercial tier labels,
feature minimum-tier messaging, migration/trial notices, billing-admin status
labels, and upgrade CTA styling must extend those helpers instead of being
forked into per-surface status-code branches that drift from backend error
truth. Trial initiation surfaces must preserve backend denial messages when the
request is blocked for migration, active commercial state, or other operational
reasons, and only map the explicit canonical cases (`trial_already_used`,
rate-limit retry) to fixed copy.
That same helper boundary also owns generic settings-paywall CTA labels such as
`Upgrade to Pro` and `Start free trial`; feature panels like
`AIRuntimeControlsSection.tsx` and `RelaySettingsPanel.tsx` must consume the
shared upgrade presentation owner instead of embedding those CTA strings
locally.
The same shared self-hosted commercial presentation boundary also owns the
activation-surface copy for `SelfHostedCommercialActivationSection.tsx`,
including the activation field label/help text, legacy-key exchange notice,
and self-serve trial panel labels, so that activation wording does not drift
separately from the rest of the Pro billing surface.
That same activation boundary also owns the linked legal surface:
`SelfHostedCommercialActivationSection.tsx` must route its Terms-of-Service
link through the shipped `TERMS.md` docs asset instead of sending operators to
GitHub `main`, so the activation trust surface stays version-matched and
available on restricted installs.
Hosted self-serve trial leases are also part of that same contract. A redeemed
hosted trial must carry the canonical Pro capability set and the authoritative
`limits.max_monitored_systems` cap inside the signed lease rather than relying
on downstream runtime fallback to infer a limit from trial state alone.
The top-level authenticated shell is part of that same customer-facing
boundary: cloud-paid trial prompts may appear in owned commercial surfaces, but
the app shell must not force a global, persistent Pro trial nudge that
overrides the primary runtime chrome for every signed-in user.
That same shell framing also owns user-facing prerelease labeling for
rc-channel builds. `frontend-modern/src/AppLayout.tsx` may still key off the
canonical `rc` channel metadata internally, but the visible badge must frame
those builds as a preview/prerelease experience rather than implying a
near-ready release candidate.
The shared trial-start runtime is part of that same cloud-paid boundary.
Commercial relay, onboarding, setup, Pro settings, and shared paywall
surfaces may customize success copy, but they must route hosted handoff,
success-notification, and canonical denial handling through
`frontend-modern/src/utils/trialStartAction.ts` instead of carrying local
`startProTrial()` redirect/error branches that drift from backend commercial
truth.
That same rule also covers the self-hosted pricing page in
`frontend-modern/src/pages/PricingV6.tsx`: pricing CTA state may choose when to
switch from trial to upgrade presentation, but the actual hosted handoff and
backend denial classification must still flow through the shared trial-start
owner instead of parsing raw trial-start status codes inline.
The trial-start rate-limit contract is part of that same boundary. Local
`/api/license/trial/start` retries are allowed as a short human-scale burst so
operators can revisit the hosted handoff without getting locked out for a day,
and both the local app server and hosted trial signup limiters must return the
actual remaining backoff through `Retry-After` and
`details.retry_after_seconds` rather than a coarse full-window guess. Shared
trial-start presentation must treat that backoff as canonical and surface it
consistently instead of flattening every `429` into generic copy.
Hosted tenant organization seeding and hosted handoff role mapping now belong
to the same cloud-paid truth too. `internal/cloudcp/stripe/provisioner.go`
must seed tenant org members from the shared account-role-to-organization-role
mapping, and hosted handoff/runtime consumers must preserve that same mapping
when older workspaces need membership continuity repaired at open time. Cloud,
MSP, and commercial account roles therefore have one canonical translation into
tenant org permissions instead of drifting between provisioning-time seeds and
handoff-time access repair.
For the hosted self-serve flow, that also means the public trial pages and form
posts must render the owned Pulse trial experience with preserved instance/form
state when rate limited, rather than dropping users onto a generic control-plane
`Too Many Requests` response.
The same owned hosted-trial failure experience applies to invalid or expired
verification links. The customer must stay inside the Pulse-owned retry path
with the originating-instance context preserved, rather than landing on a
generic hosted error page that reads like a detached SaaS funnel.
The hosted trial handoff page is part of that same boundary as well. It may
still use a secure hosted Stripe-backed session internally, but the customer
copy must present the flow as starting a trial for the originating Pulse
instance, not as a generic purchase funnel. Recovery-contact fields such as
work email and optional company name must remain clearly secondary to the
instance-bound entitlement handoff. Hosted form-stage issuance conflicts must
also preserve the canonical reason shape: duplicate recovery-email usage must
not be flattened into an organization-level message, and terminal conflicts
must render as owned hosted outcome UX rather than editable inline form state.
Expired or invalid hosted backup-link states must follow that same rule rather
than falling back to a form with missing Pulse initiation context.
Hosted service/configuration failures during verification, hosted checkout
preparation, or checkout-session creation must also render as owned
"temporarily unavailable" outcome UX rather than inline form errors.
The self-hosted activation return notice on `/settings/system-pro` is part of
that same boundary: the `trial` query result is a one-shot handoff outcome and
must be consumed into owned UI state, then removed from the URL rather than
becoming sticky page state across refresh and sharing.
That same shared notice owner must also keep activation-result framing
customer-facing: replayed handoffs should confirm the current entitlement state
rather than reading like a fresh failure, while invalid and unavailable states
should direct the operator back to the secure trial handoff on this instance.
That same hosted owner also applies after Stripe returns to
`/trial-signup/complete`: customer-facing completion failures must stay inside
owned trial UX rather than dropping raw control-plane error strings, and they
may only offer a direct "Start Trial Again" restart path when the original
Pulse return target and initiation binding are still known. Terminal conflicts
such as "trial already used" must not present restart as the recommended next
step.
That same rule applies to the self-hosted Pro settings panel. Trial start
errors in `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
must route through the shared cloud-paid presentation helper so backend denial
reasons, explicit already-used conflicts, and retry-later states stay aligned
with the rest of the commercial surfaces.
redefined inline in settings feature gates, Pro license panels, or trial
upgrade nudges.
That same disclosure copy must stay professional and customer-facing. The
settings ledger may expose counting details and included collection paths, but
it must avoid informal debug-style labels or ad hoc wording that makes the
commercial usage surface feel provisional.
That same governed ledger support surface now also owns backend-authored status
explanation copy beside the monitored-system status label. The frontend details
view may normalize a safe default when that field is absent during mixed-version
rollouts, but it must render the canonical backend explanation when present
instead of inventing page-local wording for what warning, offline, or unknown
means on a counted monitored system.
That same cloud-paid surface must now also render the canonical status reason
list when present, so customers can see exactly which grouped source or
top-level surface degraded and its canonical `reported_at` timestamp rather
than only reading generic status copy beside a fresh aggregate `Last Seen`
value.
That same settings surface must also label the monitored-system signal by its
real meaning. The canonical API shape is now the structured
`latest_included_signal` object, and the normalized frontend client contract
should expose only that canonical object. Retired flat alias fields must not
re-enter the live backend payload or client contract. It represents the freshest
included grouped observation, not a guarantee that every grouped source is
healthy, so the UI must not present it with single-source `Last Seen` wording.
When the canonical object is present, the surface should use its
source/name/type attribution instead of showing an unqualified aggregate
timestamp, and that attribution should stay customer-facing rather than
exposing raw monitored-system type/source slugs in the settings table or
included-surface details. When the row expands to show status reasoning, it
should also restate the freshest included surface and timestamp there so a
degraded reason and a fresher grouped signal remain readable together. When
mixed-version payloads omit that canonical freshest signal entirely, the
settings surface should degrade to a safe customer-facing
fallback instead of an unexplained placeholder glyph.
That same billing support boundary now also owns the shared monitored-system
presentation helper. `frontend-modern/src/utils/monitoredSystemPresentation.ts`
is the canonical owner for monitored-system brief/disclosure copy, ledger
labels, safe fallback summaries, source/type attribution wording, and the
customer-facing monitored-system usage/migration strings reused by the shared
limit-warning banner, so the settings panel, Pro usage section, counting-rules
disclosure, and shared warning-banner model must consume that helper instead
of redefining customer-facing monitored-system copy inline or keeping a
parallel copy in generic self-hosted plan utilities.
That same disclosure surface must not accept arbitrary caller-supplied
monitored-system summary strings as its primary API. When the disclosure needs
to show brief summary copy, it should render the canonical helper-owned brief
summary rather than giving callers a free-form summary prop that can drift from
the governed monitored-system language.
That same helper now also owns the row-status fallback summaries used when
mixed-version payloads omit `status_explanation.summary`. The API client and
settings panel must derive online/warning/offline/unknown fallback text from
that shared owner instead of keeping separate local defaults that can drift.
That same settings owner split now also requires
`frontend-modern/src/components/Settings/MonitoredSystemLedgerPanel.tsx` to
consume the normalized ledger client contract directly. Panel-local fallback
reconstruction for `status_explanation`, `explanation`, or
`latest_included_signal` belongs in
`frontend-modern/src/api/monitoredSystemLedger.ts` instead, so the billing
surface stays a pure render shell over one canonical monitored-system ledger
shape.
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
That same ownership split is explicit in the governed registry as well:
`CommercialBillingSections.tsx` is part of the shared commercial shell/model
surface, while `SelfHostedCommercialActivationSection.tsx` stays on the
self-hosted Pro activation surface with `ProLicensePanel.tsx` rather than
floating as an unowned settings fragment.
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
That same authenticated shell contract now also has to distinguish backend
availability from websocket-stream liveness. When hosted runtime health stays
available during a stream reconnect or renegotiation window,
`frontend-modern/src/useAppRuntimeState.ts` must preserve a canonical
backend-healthy signal and `frontend-modern/src/AppLayout.tsx` must not
advertise the whole paid shell as disconnected or "Reconnecting..." solely
because the live stream is transiently recovering. Future hosted shell work
must keep the top-right connection badge aligned to overall authenticated
runtime availability first, with websocket churn treated as a narrower live
stream status instead of the shell-level truth.
That same hosted shell badge must also stay explicit about degraded-but-usable
paid runtime states. When the backend remains healthy but the live stream is
recovering, the authenticated shell should surface a degraded sync state
instead of collapsing back to the same disconnected wording used for total
runtime loss, so hosted operators can tell the difference between browser/API
usability and live-stream freshness at a glance.
That same authenticated shell contract also applies to every chrome consumer of
runtime health, not only the badge label. `frontend-modern/src/AppLayout.tsx`
must consume the canonical `connectionStatus` contract end-to-end for shell
behavior such as the Pulse logo activity animation rather than retaining
parallel `connected`/`reconnecting` prop assumptions after the runtime model
moves. Future hosted shell changes must update all chrome affordances together
so a status-model refactor cannot leave stale prop calls behind that crash the
authenticated cloud shell at render time.
That same route/provider shell must stay page-oriented as well: `App.tsx`
should lazy-load route shells like `frontend-modern/src/pages/Storage.tsx`
and `frontend-modern/src/pages/Operations.tsx`
instead of wiring product-surface components such as
`frontend-modern/src/components/Storage/Storage.tsx` directly into the router,
so hosted bootstrap ownership stays at the app boundary rather than leaking
route concerns back into feature components.
That same authenticated route shell also owns the canonical post-auth landing
path. `frontend-modern/src/App.tsx` must send `/` through the dashboard page
shell first, so existing operators land on the overview route and first-time
operators hit the governed dashboard empty state that forwards them into
Infrastructure Install, instead of carrying a root-only redirect to the
infrastructure route that bypasses the shared landing contract.
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
That same boundary also owns the shell and section framing copy for the
self-hosted Pro billing surface. `ProLicensePanel.tsx` must consume shared
presentation for its shell title/description, refresh CTA label, and section
headings rather than carrying those commercial labels inline.
The same title should also feed the `system-billing` settings navigation item,
and the same title and description should also feed the `system-billing` route
header metadata so the nav, shell, and route header do not narrate the same
commercial surface differently.
That same shared presentation owner also carries the canonical cross-surface
referral copy used outside the billing shell itself. When infrastructure or
other adjacent settings surfaces need to point operators toward Pulse Pro for
billing, monitored-system limits, or license status, they must consume the
settings-owned referral strings from
`frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
instead of drafting route-local commercial guidance or reaching directly into
generic commercial helpers from hosted settings routes.
That same hosted-settings presentation boundary is explicit about bundle
ownership. `frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
is the canonical settings-shell adapter for self-hosted Pro shell framing and
referral copy, while `frontend-modern/src/utils/licensePresentation.ts`
remains the shared commercial notice/label owner. Hosted settings surfaces
must not import self-hosted billing framing straight from the generic helper
module when doing so would reintroduce top-level bundle-init cycles into
hosted tenant settings routes.
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
That shared ownership also includes display-ready price semantics. Monthly
headline price, founding-rate override, compare-at strike-through copy,
campaign badge copy, and annual summary text must come from the shared
plan-definition owners rather than page-local string parsing or hardcoded
retail amounts inside hosted pricing/signup screens.
The same owner also holds shared hosted commercial copy such as page title,
introductory description, common Cloud inclusions, setup-step guidance,
hosted-signup section headings, and primary hosted-signup CTA labels when
those facts describe the canonical offer rather than one page's local layout.
The same rule applies to self-hosted pricing framing: page title/description,
badge copy, primary CTA labels, and adjacent upsell-link copy on
`PricingV6.tsx` must come from the shared self-hosted plan-definition owner
rather than page-local strings.
The shared license presentation owner also holds self-hosted Pro settings
upsell and trial-ended notice copy for `ProLicensePlanSection.tsx`; that
surface must consume canonical helper notices instead of carrying inline
upgrade copy or local status-tone branches.
That same plan-section boundary must also defer notice resolution to component
runtime. `frontend-modern/src/components/Settings/ProLicensePlanSection.tsx`
may not compute trial-ended or inactive-upsell notices at module scope,
because hosted settings bundles must survive top-level import ordering without
throwing initialization-time `ReferenceError` crashes before the workspace UI
mounts.
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
Loading, empty, and temporary-unavailable states on monitored-system usage
surfaces follow that same rule: they should use calm customer-facing status
language such as usage loading or usage unavailable with a clear retry action,
not raw transport phrasing like `failed to load`.
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
That relay pairing boundary now also includes backend-owned mobile credential
lifecycle: when the settings surface generates a mobile pairing QR, it must ask
the server for a fresh scoped Pulse Mobile relay access token, fetch the
onboarding payload through that token context, and tear down superseded or
failed unused tokens instead of accumulating hidden credentials.
That same pairing fetch path must stay token-bound instead of ambient-session
bound: the onboarding QR request must carry the freshly minted relay pairing
token explicitly so the returned payload reflects the exact credential that the
mobile device will bootstrap with, not whichever browser session happened to
open the settings page.
The same flow must fail closed if the pairing payload omits the authenticated
relay token state needed by the mobile deep link; pairing UI cannot silently
render a QR that bypasses governed auth ownership.
Relay pairing token presentation is part of that same contract as well: the
settings surface must label those Pulse Mobile relay access credentials
distinctly from long-lived automation tokens so operators can identify and
revoke stale mobile devices or abandoned pairing attempts without guessing
which credential was created for mobile runtime access.
That same runtime contract now uses a dedicated backend-owned
`relay:mobile:access` scope, with the server preserving backward-compatible
gates for older mobile tokens that still carry the legacy AI scopes during the
post-RC migration window.
Relay pairing refresh behavior is part of that lifecycle contract too: a
successful QR refresh must revoke the superseded token once the new token-backed
payload is ready only if the superseded token still shows no device use, while
a failed refresh must revoke only the new failed token and keep the previously
valid QR visible instead of collapsing the operator back to an empty pairing
state.
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
Hosted trial bootstrap and hosted entitlement refresh now also own quickstart
credit seeding as part of that same persisted billing boundary. New hosted
trial workspaces and later lease-refresh rewrites must preserve
`quickstart_credits_granted` plus its grant timestamp instead of resetting the
workspace to zero hosted quickstart inventory after signup or entitlement
renewal.
Hosted AI runtime defaults are part of the same boundary as well: when a cloud
tenant falls back to provider defaults, the persisted model identifier must
remain canonical `provider:model` data rather than a bare provider-local alias,
so hosted enterprise runtime startup does not fail before chat or approvals can
initialize.
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
That same shared browser client boundary also owns structured error extraction
for hosted tenant requests. Cloud and MSP surfaces may show failures from
`frontend-modern/src/utils/apiClient.ts`, but they must resolve canonical JSON
`error` / `message` fields before showing UI feedback instead of leaking raw
response payloads while tenant-scoped org headers are still in flight.
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
That same runtime surface must stay internally coherent in local dev mode. If
`pkg/licensing/service.go` widens backend `HasFeature()` checks under
`PULSE_DEV=true` or demo/mock mode, the entitlement payload and any other
frontend-facing capability contract derived from that service must advertise
the same capability set instead of leaving paid surfaces artificially locked
behind stale free-tier capabilities.
That dev widening is still bounded by runtime readiness rather than marketing
intent: `multi_tenant` is not a valid advertised dev capability unless the
process also has `PULSE_MULTI_TENANT_ENABLED=true`, because the org API surface
is intentionally disabled otherwise.
The same runtime-readiness rule also excludes placeholder and plan-marker
entries like `white_label`, `multi_user`, and `unlimited` from dev/demo
entitlement payloads when there is no corresponding operable runtime surface;
those belong in tier metadata and plan semantics, not in live capability
advertising for a free dev shell.
The same distinction applies to customer-facing self-hosted billing
presentation. `frontend-modern/src/components/Settings/useProLicensePanelState.ts`
and `frontend-modern/src/utils/licensePresentation.ts` may label operable
capabilities, but they must not render placeholder or plan-marker entries like
`white_label`, `multi_user`, or `unlimited` in the Pro plan feature list just
because those keys exist in tier metadata.
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
That same paid relay onboarding boundary now also owns QR-token lifecycle on
the supported Pulse Mobile pairing surface. `RelaySettingsPanel.tsx`,
`RelayPairingSection.tsx`, and `useRelaySettingsPanelState.ts` may revoke a
displayed pairing token only when canonical token metadata still shows no
`lastUsedAt`; once a token has been used by a paired device, refreshing or
hiding the QR must preserve that credential instead of treating it as
disposable UI state.
That same paid relay onboarding boundary also owns the operator-facing rollout
posture on the licensed settings surface: relay paywall and pairing copy must
describe supported Pulse Mobile pairing as a normal-use Relay capability, and
must not fall back to staged-beta or coming-soon messaging once the owned
pairing/runtime path is live.
The customer-account surface is now also an explicit cloud-paid ownership
boundary. Pulse already has a real hosted control-plane portal in
`internal/cloudcp/portal/`, account/workspace mutation APIs in
`internal/cloudcp/account/tenant_handlers.go`, and transitional self-hosted
commercial utility pages in `pulse-pro/landing-page/`, but those surfaces do
not yet form one coherent Pulse account product. The canonical future shape is
`docs/release-control/v6/internal/PULSE_ACCOUNT_PORTAL_SPEC.md`: one
authenticated Pulse account shell that unifies Cloud tenants, self-hosted
licenses, billing, recovery, and MSP admin surfaces without creating a
standalone Relay portal. That shell now also owns two product rules
explicitly: `Overview` is one shell-level triage surface across the current
account set rather than a repeated per-account dashboard, and unavailable
top-level `Workspaces` or `Access` tasks must render an explicit unavailable
state instead of blank space. The shell navigation must stay honest to that
same account shape rather than labelling unavailable hosted tasks as though
they are still live. The same shell also owns action-first task
surfaces for `Access`, `Billing`, and `Support`: access mutations must be
permission-honest and roster-led, billing must reduce to one obvious job at a
time with hosted billing first when relevant, support must stay a failed-path
handoff rather than a peer workflow, and phone-width layouts must collapse the
desktop task sidebar into a compact task strip so the active job remains
primary and visibly in-frame when the strip scrolls. The same narrow-screen
shell must also compress account identity into a compact summary strip instead
of repeating a desktop-sized intro block ahead of every task, and lower
workspace job surfaces such as lifecycle review or create-workspace forms must
be revealed when the user opens them. `Workspaces` must also default to the
workspace list plus the real task entry points rather than an idle lifecycle
essay; the lifecycle rail should appear only when a lifecycle or
create-workspace job is active. `Access` follows the same rule: the hosted
roster must be the default state, while invite, role-change, and remove
controls appear only when that exact access job is active, and a hosted
view-only roster must stay a review surface instead of a row-by-row action
table with fake disabled action state. `Billing` follows
the same task-first rule: hosted billing leads when present, the self-hosted
billing, license, refund, and privacy paths reduce to explicit job pickers,
and the active self-hosted billing panel must stay hidden until that exact job
is opened and then be revealed in-frame on narrow layouts. When no hosted
account exists, the billing shell must skip any empty hosted-billing block and
lead directly with the self-hosted job picker. `Support` follows
the same account-shape rule: self-hosted-only accounts reduce to the billing
escalation path and billing-specific handoff packet only, and hosted
workspace/access escalation routes must not render without hosted accounts.
The same shell/runtime boundary must also stay honest to hosted-account
permissions: when the current role is view-only, `Workspaces`, `Access`, and
hosted `Billing` copy must not advertise create, roster-mutation, or hosted
billing actions that the runtime will not allow. Those surfaces must say that
an owner or admin is required instead of implying blocked jobs are live.
`Support` follows that same permission rule: a hosted view-only user may be
sent back to `Workspaces`, `Access`, or `Billing` only as review and
owner/admin handoff paths, not as though the user can perform hosted
lifecycle, access-mutation, or hosted-billing changes directly before
escalation.
`Overview` follows the same account-shape and permission rule: hosted-only
accounts must not mention self-hosted billing utilities by default, and hosted
view-only roles must say when hosted billing still needs owner/admin
authority.
The same rule applies to the compact account-context strip: it must describe
the current user's effective hosted tasks, not restate full access-control and
billing capability when those actions are blocked behind owner/admin roles.
That same owned shell must also keep role labels on product vocabulary:
customer-facing copy may say `Owner`, `Admin`, `Tech`, or `Read-only`, but it
must not leak internal identifiers such as `read_only` or legacy aliases such
as `member`.
That same owned `Overview` surface must also keep `Next action`
permission-honest for hosted view-only users: when no workspace is ready, the
primary route must stay on reviewable `Workspaces` or `Access` surfaces before
any blocked hosted billing or owner/admin-only mutation path.
That same canonical shell/runtime boundary now also owns the bootstrap truth
for when self-hosted commercial history is relevant. Hosted-only accounts must
not render self-hosted license, refund, privacy, or support-escalation copy
unless the portal bootstrap explicitly marks self-hosted commercial history as
relevant to the signed-in account.
Until that
candidate lane lands, new
commercial account work must extend the governed Pulse account shape rather
than spawning additional one-off recovery or billing pages.
