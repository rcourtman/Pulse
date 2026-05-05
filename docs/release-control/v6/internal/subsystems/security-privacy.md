# Security Privacy Contract

## Contract Metadata

```json
{
  "subsystem_id": "security-privacy",
  "lane": "L14",
  "contract_file": "docs/release-control/v6/internal/subsystems/security-privacy.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
}
```

## Purpose

Own Pulse's canonical privacy disclosures, outbound usage-data boundary,
and the security-facing settings surfaces that expose authentication posture,
token-management visibility, and privacy controls to operators. Customer-facing
privacy and Settings surfaces must not present maintainer commercial-event
controls as normal product settings.

## Canonical Files

1. `SECURITY.md`
2. `docs/PRIVACY.md`
3. `frontend-modern/public/docs/PRIVACY.md`
4. `frontend-modern/src/utils/docsLinks.ts`
5. `frontend-modern/src/api/security.ts`
6. `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
7. `frontend-modern/src/components/Settings/APITokenManager.tsx`
8. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`
9. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
10. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
11. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`
12. `frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`
13. `frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`
14. `frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`
15. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`
16. `frontend-modern/src/components/Settings/useSystemSettingsState.ts`
17. `frontend-modern/src/constants/apiScopes.ts`
18. `frontend-modern/src/utils/apiTokenPresentation.ts`
19. `frontend-modern/src/utils/auditLogPresentation.ts`
20. `frontend-modern/src/utils/auditWebhookPresentation.ts`
21. `frontend-modern/src/utils/securityAuthPresentation.ts`
22. `frontend-modern/src/utils/securityScorePresentation.ts`
23. `internal/api/security.go`
24. `internal/api/security_tokens.go`
25. `internal/api/system_settings.go`
26. `internal/config/config.go`
27. `internal/config/watcher.go`
28. `internal/telemetry/telemetry.go`
29. `internal/api/router_routes_auth_security.go`
30. `internal/crypto/crypto.go`
31. `internal/securityutil/secure_storage_dir.go`
32. `internal/cloudcp/auth/magiclink.go`
33. `internal/cloudcp/auth/magiclink_store.go`
34. `pkg/tlsutil/fingerprint.go`
35. `scripts/telemetry_adoption_report.py`
36. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx`
37. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts`

## Shared Boundaries

1. `frontend-modern/src/api/security.ts` shared with `api-contracts`: the security frontend client is both a security/privacy control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/components/Settings/APIAccessPanel.tsx` shared with `frontend-primitives`: the API Access settings intro is both a security/privacy token-management trust surface and a canonical settings-shell presentation boundary.
   Its Docker / Podman token wording must come from
   `frontend-modern/src/utils/apiTokenPresentation.ts` rather than page-local
   copy.
3. `frontend-modern/src/components/Settings/APITokenManager.tsx` shared with `api-contracts`: the API token settings surface is both a security/privacy control surface and a canonical API payload contract boundary.
   Token-management table rows are security-facing content, but the visual
   table frame and scroll shell belong to `frontend-primitives`
   `PulseDataGrid`; do not add token-surface-local overflow, side-border, or
   negative-margin wrappers around the inventory grid.
4. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` shared with `api-contracts`: the pure API token settings model is both a security/privacy control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` shared with `frontend-primitives`: the data-handling settings surface is both a security/privacy trust surface and a canonical settings-shell presentation boundary.
6. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` shared with `frontend-primitives`: the data-handling settings model is both a security/privacy posture projection and a canonical settings-shell presentation boundary.
7. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `frontend-primitives`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
8. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `frontend-primitives`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
9. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `frontend-primitives`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
10. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts` shared with `api-contracts`: the API token settings state hook is both a security/privacy control surface and a canonical API payload contract boundary.
11. `frontend-modern/src/constants/apiScopes.ts` shared with `api-contracts`: the API token scope catalog is both a security/privacy token-management trust surface and a canonical API token payload boundary.
    Scope labels and descriptions are visible security controls. Docker /
    Podman scopes must use the shared source-platform label rather than
    generic `container` copy.
12. `frontend-modern/src/utils/apiTokenPresentation.ts` shared with `api-contracts`: the API token presentation helper is both a security/privacy control surface and a canonical API token management boundary.
    It owns Docker / Podman token copy for API Access, token presets, usage
    summaries, and revoke warnings so security-facing copy does not drift into
    page-local `container runtime` labels.
13. `internal/api/security.go` shared with `api-contracts`: the security handlers are both a security/privacy control surface and a canonical API payload contract boundary.
14. `internal/api/security_tokens.go` shared with `api-contracts`: the security token handlers are both a security/privacy control surface and a canonical API payload contract boundary.
    Pulse Mobile relay token creation is a security token-management surface,
    but it is not a free API-token convenience. After admin and
    `settings:write` authorization, `POST /api/security/tokens/relay-mobile`
    must fail closed with the standard license-required response unless the
    active entitlement includes the paid `relay` feature.
15. `internal/api/system_settings.go` shared with `api-contracts`: the system settings telemetry and auth controls are both a security/privacy control surface and a canonical API payload contract boundary.
16. `internal/cloudcp/auth/magiclink.go` shared with `cloud-paid`: control-plane magic-link HMAC handling is both a Pulse Cloud account-access boundary and a security/privacy token-secrecy boundary.
17. `internal/cloudcp/auth/magiclink_store.go` shared with `cloud-paid`: control-plane magic-link persistence is both a Pulse Cloud account-access boundary and a security/privacy storage-hardening boundary.

## Extension Points

1. Change privacy disclosures, usage-data vocabulary, or outbound-data guarantees through `docs/PRIVACY.md` and `internal/telemetry/telemetry.go` together.
2. Change security policy, hardening guidance, or supported auth boundaries through `SECURITY.md`.
3. Change telemetry/privacy settings state handling through `frontend-modern/src/components/Settings/useSystemSettingsState.ts`.
4. Change security/auth/token transport behavior through the shared `frontend-modern/src/api/security.ts`, `frontend-modern/src/components/Settings/APITokenManager.tsx`, `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`, `internal/api/security.go`, `internal/api/security_tokens.go`, and `internal/api/system_settings.go` boundary.
   CSRF token-store behavior in `internal/api/csrf_store.go` is part of that
   shared browser-auth trust boundary: parallel stale-token mutations may
   receive distinct bounded replacement tokens for one session, but explicit
   session deletion, password-change invalidation, and logout must invalidate
   every retained CSRF hash for that session.
   Auth and session changes that involve hosted, SSO, or commercial identity
   must also preserve `docs/release-control/v6/internal/IDENTITY_INVARIANTS.md`:
   email is contact metadata once a stable principal exists, and browser
   sessions must bind to the durable principal rather than a delivery address.
   For SSO, the durable principal is the provider-scoped subject, and mutable
   username/email/display claims may not be written as the session owner.
   Live organization authorization follows the same trust boundary: contact
   email can support display, delivery, or migration, but request access must
   match the authenticated principal against stored `OwnerUserID` or member
   `UserID`.
5. Change security/privacy settings presentation through the shared `frontend-modern/src/components/Settings/APIAccessPanel.tsx`, `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`, `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`, `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`, `frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`, `frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`, `frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`, `frontend-modern/src/constants/apiScopes.ts`, `frontend-modern/src/utils/apiTokenPresentation.ts`, `frontend-modern/src/utils/securityAuthPresentation.ts`, `frontend-modern/src/utils/securityScorePresentation.ts`, `frontend-modern/src/utils/auditLogPresentation.ts`, and `frontend-modern/src/utils/auditWebhookPresentation.ts` boundary.
6. Change operator-facing telemetry/adoption reporting through `scripts/telemetry_adoption_report.py` together with the privacy disclosure whenever release-identity interpretation changes.
7. Change data-at-rest encryption-key or control-plane magic-link HMAC key and storage-root hardening semantics through `internal/crypto/crypto.go`, `internal/cloudcp/auth/magiclink.go`, `internal/cloudcp/auth/magiclink_store.go`, and `internal/securityutil/secure_storage_dir.go` together so writable-but-not-owned runtime storage mounts stay supported without weakening file-level secrecy.
8. Change auth-env password normalization, hosted commercial base URL
   normalization, or shared TLS fingerprint verification defaults through
   `internal/config/config.go`, `internal/config/watcher.go`, and
   `pkg/tlsutil/fingerprint.go` together so startup auth ingestion, live
   auth-env reloads, hosted entitlement refresh origins, and
   pinned-fingerprint TLS clients keep one fail-closed security floor.
9. Change operator-facing data-handling posture through `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` and `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` together so resource classification, handling-boundary, and redaction copy stays governed as a trust surface.

## Forbidden Paths

1. Changing telemetry payload semantics without updating the canonical privacy disclosure.
2. Letting security-facing settings copy or privacy guarantees drift between runtime behavior and the governed docs.
3. Treating API token management, auth posture, or telemetry controls as generic settings-shell polish instead of explicit trust-surface behavior.

## Completion Obligations

1. Update privacy/security docs and the telemetry runtime together when outbound-data behavior changes.
2. Keep shared API-contract proof routing aligned whenever auth, token, or telemetry settings payloads change.
3. Keep shared frontend settings proof routing aligned whenever security/privacy presentation changes.
4. Keep the checked-in telemetry adoption report aligned with the same release-identity rules used by the runtime telemetry payload.
5. Update this contract whenever a new canonical security, token, auth, or privacy surface becomes part of the governed trust boundary.
6. Keep the shared storage-directory and secure storage-file hardening helper aligned with the crypto manager plus control-plane magic-link key and store handling whenever runtime data-root ownership assumptions change.
7. Keep auth-env ingestion, hosted commercial base URL validation, and shared
   fingerprint-verifier TLS defaults aligned whenever runtime auth loading,
   hosted entitlement refresh origin handling, or pinned-certificate transport
   behavior changes. Hosted commercial URL overrides must remain absolute
   HTTP(S) URLs, with plain HTTP limited to loopback development origins.
8. Keep the Data Handling settings surface neutral and non-commercial: it may show resource policy posture, local-only counts, and redaction coverage, but it must not advertise trials, upgrades, paid plans, or monitoring limits.
9. Keep operator-facing Data Handling posture aligned with runtime AI/context enforcement: `local-only` resource details must not be sent to external model prompts, and sensitive free-form alert, tool-result, investigation, and any retained legacy managed-model compatibility text must use the shared resource-policy redaction helper before leaving the local trust boundary. All provider-bound AI requests to non-local models must use the shared resource-policy sanitizer immediately before transport so later agentic turns cannot bypass the advertised handling posture.
10. Keep the canonical and frontend-served privacy disclosures aligned with
    the actual AI transport boundary: self-managed installs must describe local
    providers as staying on the operator network, non-local providers as direct
    provider-bound requests from the Pulse instance, and managed-model
    quickstart/trial transport as absent from normal self-hosted v6 GA docs.
    Both disclosures must state that governed resource details use
    resource-policy redaction before non-local model transport.
11. Keep durable identity and email-contact semantics aligned with the
    canonical identity invariant record. Hosted and commercial auth paths must
    use stable Pulse user/account/tenant IDs where they exist; SSO subject
    migration must be explicit and compatible rather than silently substituting
    email or display claims as durable principals.

## Current State

This subsystem now gives `L14` an explicit governed home for privacy guidance
and telemetry disclosures instead of leaving those trust surfaces as lane-level
evidence with no subsystem ownership.
That same governed home now also owns the single customer-facing "usage data"
vocabulary for anonymous outbound telemetry. Local commercial activation and
license-recovery runtime records must stay out of ordinary Settings, support
diagnostics, outbound telemetry disclosure copy, and public configuration
reference tables.
That same operator-reporting boundary now also owns reusable latest-install
adoption baselines. `scripts/telemetry_adoption_report.py` must emit
windowed 24h, 72h, and 7d latest-install snapshots that split published
versions from unpublished or development builds, so RC adoption reads stop
depending on ad hoc SQL or one-off local helper scripts.
That same storage hardening boundary now also owns secure regular-file
handling for secret-bearing local trust material and the control-plane
magic-link storage root. `internal/crypto/crypto.go`,
`internal/cloudcp/auth/magiclink.go`, and
`internal/cloudcp/auth/magiclink_store.go` must route encryption keys,
magic-link HMAC keys, and the magic-link SQLite store path through the shared
secure storage helpers so symlink, oversize, and non-regular file paths fail
closed instead of slipping past directory-only hardening.

Security-facing settings remain intentionally shared with `frontend-primitives`
because shell framing and presentation consistency still belong there, but the
meaning of those surfaces now lives here so auth posture, token controls, and
privacy toggles stop borrowing their governance only from adjacent lanes.
That settings presentation boundary also owns trust-sensitive vocabulary around
operator access. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
and `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` must use
monitoring/workspace wording for tours and read-only token presets instead of
reviving Dashboard-specific labels after the Dashboard route has been retired.
The Data Handling settings surface extends that trust boundary to resource
policy posture. It may expose the canonical sensitivity, handling-boundary,
and redaction counts that Pulse already applies to resources, but it must stay
informational and non-commercial so free/self-hosted operators are not shown
paywall, trial, upgrade, or monitoring-limit prompts inside a privacy surface.
That posture is now enforced at the AI provider boundary too: non-local model
requests must be sanitized from the same resource-policy metadata that powers
the Data Handling surface. Hosted quickstart traffic is retired from the Pulse
runtime, so privacy governance must not describe a live public hosted-model
proxy for normal self-hosted v6 installs.
That shared settings boundary now also has an explicit split of responsibilities:
`frontend-modern/src/components/Settings/useSystemSettingsState.ts` remains the
canonical owner for customer-visible telemetry and auth/privacy runtime state,
while maintainer commercial analytics controls stay out of customer settings
payloads and frontend settings state entirely.
`frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` stays a
customer-facing presentation boundary for outbound telemetry controls and
`frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx` may only
assemble customer-visible props for the shared settings shell. Privacy or telemetry
behavior must not drift into `frontend-modern/src/components/Settings/Settings.tsx`
or the registry hook just because the shell wiring changed.
That shell split now also applies to tab-save coordination: the dedicated
`frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts` owner may
decide which settings tabs participate in shell-level save prompts, but it must
remain pure shell metadata. Telemetry and auth/privacy state transitions stay
canonically owned by `frontend-modern/src/components/Settings/useSystemSettingsState.ts`,
and maintainer analytics state must not be carried by
`frontend-modern/src/stores/systemSettings.ts`, settings navigation metadata, or
other frontend-primitives owners.
Retired local-upgrade-metrics compatibility must not become customer-side or
runtime commercial analytics emission: browser product surfaces must not POST
pricing, checkout, paywall, funnel, or onboarding signals to
`/api/upgrade-metrics/events`; the normal product API must not register
`/api/upgrade-metrics/*` or `/api/admin/upgrade-metrics-funnel`; product startup
must not open or migrate `upgrade_metrics.db`; and customer frontend source must
not keep `upgradeMetrics`, `conversionEvents`, or infrastructure onboarding
metrics wrappers as compatibility imports.

The security transport surfaces remain intentionally shared with
`api-contracts`: token, auth, and telemetry settings payloads are still API
contracts, but they now also count as first-class security/privacy runtime
behavior that `L14` must govern directly.
That same shared auth and forwarded-header trust surface must reject wildcard
proxy trust ranges in `PULSE_TRUSTED_PROXY_CIDRS` at startup, and runtime
client-IP derivation must fail closed instead of trusting forwarded headers if
an invalid wildcard proxy trust range is configured.
That shared settings/auth boundary now also inherits the runtime-versus-
commercial licensing split. Security/privacy settings may consume runtime
capability truth where feature availability matters, but billing identity,
trial posture, and upgrade routing stay on the dedicated commercial boundary,
and public-demo suppression must resolve from the shared `presentationPolicy`
contract instead of security-surface entitlement reads or local demo flags.
Security/privacy feature gates that are suppressed by
`presentationPolicy.hideUpgrade` must also use neutral unavailable-capability
copy: privacy and audit surfaces must not leave `(Pro)`, trial, plan-tier, or
upgrade wording visible after their commercial actions are hidden.
That shared token-management boundary now also includes
`frontend-modern/src/utils/apiTokenPresentation.ts`, so API-token load,
generate, and revoke errors stay on one governed customer-facing wording path
instead of drifting back into hook-local notification strings.
That same API-token presentation helper also owns API token management-location
copy for Settings surfaces. Token reveal and rotation guidance must point
operators to `Settings → API Access` and must not revive legacy
`Security → API tokens` wording.
That same token-management boundary must also treat top-level TrueNAS
appliances as canonical agent-scope resources through the shared agent-facet
helper. Security surfaces may consume compatibility-normalized
`platformType: 'truenas'` resources, but they must not reintroduce a separate
`resource.type === 'truenas'` trust path when calculating token usage,
revocation targets, or operator-facing token ownership.
That same token-management boundary also reserves token-owner identity for the
server-authenticated principal. Token-minting helpers must derive
`owner_user_id` from the authenticated session or caller token and reject any
extension metadata that attempts to overwrite that field. This applies beyond
the visible API-token manager: agent install command tokens, deploy bootstrap
tokens, enrollment runtime tokens, container runtime migration tokens, and
first-run/regenerated admin tokens must use the same shared server-side owner
setter rather than carrying owner identity in caller-controlled metadata.
Telemetry/privacy disclosures now also route through the shipped frontend docs
boundary: `frontend-modern/src/utils/docsLinks.ts` is the canonical frontend
owner for privacy-document URLs, while `frontend-modern/public/docs/PRIVACY.md`
is the version-matched asset served by the running build. Privacy disclosures
must not drift back to GitHub `main` links that can describe a different
revision than the installed runtime.
Relay privacy copy belongs to that same synchronized disclosure boundary: both
the canonical and frontend-served privacy docs must describe Relay outbound use
as secure remote web access, Pulse Mobile pairing for handoff, and push
notifications rather than generic mobile-app monitoring.
That same disclosure boundary now also fixes the telemetry payload floor:
commercial and auth-adjacent telemetry may report only coarse posture signals
such as whether a paid license is active or whether any API tokens exist.
Exact license tiers and exact API-token counts are not part of the canonical
anonymous telemetry contract and may not be reintroduced without updating this
trust boundary and the governed privacy disclosure together.
That same rule also applies at the license-server ingest and storage boundary:
server-side telemetry rows may preserve the canonical normalized version
identity plus those same coarse booleans, but they must not retain legacy
exact commercial tier or exact API-token count fields as first-class analytics
dimensions just because older clients once sent them.
That same anonymous telemetry contract also treats `install_id` as a rotating
pseudonymous identifier, not a lifetime install handle. The runtime may keep a
local rotating UUID so startup and heartbeat pings can still represent an
active installation window, but it may not preserve one stable install
identifier indefinitely or echo that identifier back into routine logs.
That same telemetry trust boundary must remain operator-inspectable in-product:
the shared system settings surface may preview only the exact runtime payload
Pulse would send, and it must allow an operator to rotate the local telemetry
install ID immediately without waiting for the scheduled 30-day window.
That same governed privacy disclosure must also state the current server-side
telemetry retention and handling rules plainly. If the license-server path
retains telemetry rows for a fixed window or uses client IPs transiently for
abuse controls, `docs/PRIVACY.md` and the shipped
`frontend-modern/public/docs/PRIVACY.md` copy must say so explicitly rather
than implying the server stores nothing at all.
That same rule also applies to the short in-product summary on the shared
General settings privacy surface and the whats-new disclosure copy. Those
surfaces may stay concise, but they must not claim a stronger privacy posture
than the governed docs; if telemetry rows are retained for a fixed window and
IP addresses are not stored rather than “never seen,” the summary copy must
say that plainly.
That same shared trust boundary now also owns the TLS floor used by pinned-
fingerprint runtime clients. `pkg/tlsutil/fingerprint.go` may support
certificate-fingerprint capture and verification for self-signed deployments,
but every mode must still set an explicit minimum TLS version instead of
silently inheriting whatever older protocol floor the host runtime would allow.
That same rule also applies inside shipped security guidance itself:
`SECURITY.md` and the synced `frontend-modern/public/docs/SECURITY.md` copy may
not bounce the operator back to GitHub `main` for section references that the
running build already owns locally. Their Relay security section must also use
the current Relay-and-higher entitlement boundary instead of stale Pro-only
license wording.
That same governed settings trust boundary now also includes
`frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`,
`frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`,
`frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`,
`frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`,
`frontend-modern/src/utils/securityAuthPresentation.ts`,
`frontend-modern/src/utils/securityScorePresentation.ts`,
`frontend-modern/src/utils/auditLogPresentation.ts`, and
`frontend-modern/src/utils/auditWebhookPresentation.ts`, so auth bootstrap
copy, security posture scoring, audit-log wording, audit-webhook wording, and
SSO provider-type presentation remain part of the governed security trust
surface instead of floating as unowned settings helpers.
That SSO security surface is not a paid-feature trust boundary. OIDC, SAML,
and multi-provider SSO share the same Community-tier authentication control
plane; security/privacy code may enforce authenticated settings capability
reads and writes, but it must not turn SAML metadata, SAML runtime routes, or
multi-provider administration into an `advanced_sso` paywall.
Audit-log filter option wording is part of that same trust surface: event,
success, and verification filter labels must be sourced from
`frontend-modern/src/utils/auditLogPresentation.ts` and the shared filter-option
label primitive rather than hard-coded title-case strings in
`AuditLogPanel.tsx`.
That same governed security-score presentation boundary also owns the
operator-facing low-score warning copy used by the top-level runtime banner:
`frontend-modern/src/utils/securityScorePresentation.ts` must describe the
actual missing controls surfaced by the current security posture, and it may
only claim the instance is accessible without authentication when
`hasAuthentication` is false. Authenticated local runtimes that are merely
missing HTTPS, API tokens, or protected exports must not reuse the
unauthenticated credential-exposure warning just because the aggregate score
remains below the banner threshold.
That same shared runtime-warning boundary must also keep the global banner
reserved for active exposure states rather than generic setup debt:
`frontend-modern/src/components/SecurityWarning.tsx` and
`frontend-modern/src/utils/securityScorePresentation.ts` may surface an
always-visible app-wide warning when authentication is disabled, export
protection is disabled, or a publicly reachable instance is still serving over
HTTP, but private authenticated runtimes that are only missing optional
hardening controls such as HTTPS on localhost or an API token must route that
guidance through the governed Security Overview posture surfaces instead of
covering the primary app chrome with a persistent warning.
That same governed trust boundary now also owns the runtime contract for
storage-root hardening of at-rest secrets: `internal/crypto/crypto.go` and the
shared `internal/securityutil/secure_storage_dir.go` helper may attempt to
harden storage directories when Pulse owns them, but they must not assume the
process owns the mount root of a writable Kubernetes or container volume.
Mounted storage roots that are writable but not chmod-able must still support
secure startup, while sensitive leaf files such as `.encryption.key` remain
file-hardened at `0600`. The mount root itself must be validated as the real
directory path rather than a symlink or other filesystem object, but its mode
bits are not a fatal startup gate when Kubernetes or another runtime owns that
mount point.
That same Security Overview surface must stay action-oriented once those
low-risk states are demoted out of the global banner:
`frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` and
`frontend-modern/src/utils/securityScorePresentation.ts` must render explicit
next-step hardening actions that point to the owning auth, API-access, or
security-guidance surface rather than dropping operators onto a generic score
without a remediation path.
That same shared security transport boundary must stay under explicit proof
routing on both sides: `frontend-modern/src/api/security.ts`,
`internal/api/security.go`, `internal/api/security_tokens.go`, and
`internal/api/system_settings.go` must continue to carry the direct
`security-api-surface` proof path together with a direct API-contract proof
path instead of borrowing coverage only from broader API fallback rules.
That same shared trust boundary now also owns canonical recovery-token
persistence: `recovery_tokens.go` may mint raw recovery secrets for immediate
operator use, but persisted `recovery_tokens.json` state must store only token
hashes and treat any legacy plaintext-token file as a one-time migration input
that is rewritten immediately into hashed canonical persistence on load.
Direct auth probes on that same boundary must fail closed with an explicit
response: public-network or missing-credential calls into shared `CheckAuth`
must emit the canonical auth-required error, while middleware-owned paths use
shared response capture so setup, recovery, and API-token-specific handlers can
preserve their stricter single response.
That same recovery trust boundary also governs live use of those secrets:
recovery tokens must bind to the generating client IP, may authorize only a
direct-loopback browser recovery session, and must not reopen authentication
through a shared `.auth_recovery` flag that affects every localhost client.
Secret-bearing comparisons on adjacent auth paths such as metrics bearer
validation and local-auth username matching must stay constant-time.
That same persistence rule also governs API token metadata: even though
`api_tokens.json` stores hashed records rather than raw token secrets, a
legacy plaintext metadata file may only serve as migration input. Canonical
runtime persistence must rewrite plaintext API token metadata immediately into
encrypted-at-rest storage on load instead of continuing to run against the
unencrypted file as a normal primary path.
That same trust boundary also governs API token scope identity: legacy
`host-agent:*` scopes may be accepted only at request-ingress or persistence/
migration boundaries, where they must be rewritten immediately into canonical
`agent:*` scopes. Live token records and runtime scope checks may not keep the
legacy scope names as an active second contract.
That same token-scope boundary also owns audit-log least privilege: audit
event, verification, summary, export, and unified action/export audit reads
must require the dedicated `audit:read` scope instead of inheriting broader
monitoring or settings-read token access.
The same security boundary now depends on unified action-audit normalization:
persisted action records must identify the requester, resource, capability,
approval policy, preflight dry-run posture, and lifecycle state before they are
read through audit APIs, so audit history cannot silently accept an unscoped or
unplanned execution record.
Action planning and action decision mutations remain privileged runtime
control surfaces even though the decision endpoint does not execute the
capability. `POST /api/actions/plan` and
`POST /api/actions/{id}/decision` must both require the governed `ai:execute`
scope so API tokens cannot create or approve executable action intent through a
read-only or mobile-only grant. `POST /api/actions/{id}/execute` is governed by
the same `ai:execute` scope because it is the only API-owned handoff from
approved intent into capability execution; missing executors must fail closed
without creating execution lifecycle evidence.
That same token-scope boundary now also governs Pulse Mobile relay runtime
credentials: `internal/api/security_tokens.go` must mint only the dedicated
backend-owned `relay:mobile:access` scope for new mobile relay tokens, and the
shared auth/router helpers may expose backward-compatible gates for older
mobile tokens only on the governed mobile runtime routes enumerated in
`internal/api/relay_mobile_capability.go`. Browser callers and route-local
handlers must not recreate wildcard or broad AI-scoped mobile credentials, and
future route expansion must update that backend-owned inventory explicitly
rather than widening compatibility through ad hoc handler checks.
That same trust rule also applies to AI-owned persisted state under
`internal/config/persistence.go`: findings, usage history, patrol run history,
and chat sessions may use plaintext files only as migration input. Once those
AI persistence owners can read the data, they must rewrite it immediately into
encrypted-at-rest storage instead of keeping plaintext history on the runtime
primary path.
That same persistence rule also applies to shared encrypted-slice config
owners under `internal/config/persistence.go`: TrueNAS instances, agent
profiles, assignments, profile versions, deployment status, change logs, and
other `loadSlice()`-backed data may use plaintext files only as migration
input. The shared loader must rewrite those slices immediately into
encrypted-at-rest storage on load instead of letting plaintext files remain the
runtime primary path.
The same migration-only rule applies to single-object encrypted config owners
in that package as well: email, Apprise, webhook, SSO, and AI config payloads
may accept plaintext files only as upgrade input, and the owning loader must
rewrite canonical encrypted-at-rest storage immediately on load rather than
deferring encryption until some later save path.
That same rule extends to AI guest knowledge under `internal/ai/knowledge/`:
legacy `.json` knowledge files and plaintext `.enc` knowledge files may only
serve as migration input, and the knowledge store must rewrite canonical
encrypted-at-rest storage immediately on load instead of leaving guest
knowledge plaintext on disk until a future note update.
That same trust boundary also applies at store construction time: the AI
knowledge store and the service discovery store may not fail open into
plaintext-at-rest mode when crypto initialization fails. If encryption cannot
be established for those stores, construction must fail closed instead of
quietly persisting runtime state unencrypted.
That same rule also applies to persisted service-discovery records after store
construction: `internal/servicediscovery/store.go` may only accept plaintext
`.enc` discovery files as migration input. Once a discovery record can be
read, canonical persistence must rewrite encrypted-at-rest storage immediately
on load/list/id-scan instead of leaving plaintext discovery metadata or user
secrets on the steady-state runtime path.
That same trust boundary also covers audit-signing key persistence:
`pkg/audit/signer.go` may keep the 32-byte HMAC signing key in runtime memory,
but `.audit-signing.key` may only accept plaintext key material as migration
input. Once a legacy plaintext signing-key file can be read, canonical
persistence must rewrite encrypted-at-rest storage immediately on load instead
of leaving the audit signing root in plaintext on the runtime primary path.
That same fail-closed rule also applies to persisted OIDC refresh tokens in
the session store: if session-store crypto is unavailable or a stored refresh
token cannot be decrypted canonically, the runtime must drop that token
instead of accepting or writing plaintext-at-rest refresh-token state.
That same rule also applies to hosted entitlement lease secrets in
`internal/config/billing_state.go`: `billing.json` may not keep
`entitlement_jwt` or `entitlement_refresh_token` as plaintext-at-rest billing
state. Canonical billing persistence must encrypt both values at rest, rewrite
legacy plaintext billing files on load, and drop those secrets instead of
preserving raw lease state if billing encryption cannot be established.
Billing persistence also may not auto-create a new crypto/key footprint just
to add integrity metadata for empty no-secret billing state; no-key graceful
degradation remains the canonical behavior until a real secret or real key is
present.
That same trust boundary also owns runtime store initialization: session, CSRF,
and recovery-token persistence may not silently self-initialize on a hidden
`/etc/pulse` fallback or remain locked to the first caller through package
`sync.Once` state. The configured router data path must stay the canonical
owner, and reinitializing it must replace the prior runtime store instead of
leaking old-path auth state into the active process.
That same path-ownership rule also governs the shared runtime data-dir helper
under `internal/config/config.go` together with `internal/config/watcher.go`:
`PULSE_AUTH_CONFIG_DIR` may remain an explicit watcher-only override, but the
canonical runtime owner for auth, token, billing, and bootstrap-adjacent disk
state must otherwise come from the resolved `ConfigPath` / `DataPath` owner or
the shared `PULSE_DATA_DIR` fallback. These surfaces may not probe `/etc/pulse`
or `/data` independently and silently override the configured path authority
just because those directories exist on the host.
`PULSE_METRICS_DB_PATH` is the explicit non-secret exception for metrics
history placement only: it may move `metrics.db` to tmpfs or a dedicated mount,
but it must not become a second authority for `.env`, tokens, encrypted
credentials, sessions, bootstrap state, billing state, or other security
persistence. `internal/config/config.go` owns that env parsing so the exception
stays visible at the shared runtime config boundary.
That same auth-env boundary must also fail closed on password normalization:
`internal/config/config.go` and `internal/config/watcher.go` may auto-hash a
plaintext `PULSE_AUTH_PASS`, but they must never preserve a raw plaintext value
in runtime config just because hashing failed. Startup must return an explicit
error, and live `.env` reloads must keep the previous runtime auth password
until a valid replacement is available.
That same rule also governs the auth `.env` file path itself: `router.go`,
`router_routes_auth_security.go`, and `security_setup_fix.go` must derive the
manual-auth env file through the shared auth-path helper instead of
reconstructing `/etc/pulse/.env` locally when `ConfigPath` is empty.
That same shared boundary also owns writable auth-env target order: password
changes and first-session setup may not reintroduce per-handler config-path
writes with private data-path fallback branches, and must instead write `.env`
through the shared auth-env helper contract.
That same first-session trust boundary also owns bootstrap-token persistence:
the one-time setup secret may remain operator-recoverable through the supported
`pulse bootstrap-token` command, but `.bootstrap_token` may not remain a raw
plaintext secret file on disk. Canonical runtime persistence must keep the
token encrypted at rest, and any legacy plaintext bootstrap-token file must be
treated only as migration input that is rewritten immediately into the
encrypted canonical format on load.
Managed first-session proof may reset that boundary only through the dev-only
`/api/security/dev/reset-first-run` route under authenticated
`settings:write`; harnesses may not scrape `.env`, delete persisted token
state, or recreate bootstrap material through lane-local teardown logic.
That same trust rule also applies to persisted relay client secrets:
`internal/config/persistence_relay.go` may only accept plaintext `relay.enc`
files as migration input. Once relay config can be read, canonical runtime
persistence must rewrite encrypted-at-rest storage immediately so
`instance_secret` and relay identity private-key material do not remain on the
steady-state runtime path.
That same migration-only rule also applies to `nodes.enc`: the canonical
infrastructure credential store may carry PVE, PBS, and PMG passwords and
token values, so `LoadNodesConfig()` may not treat legacy plaintext
`nodes.enc` as a steady-state runtime path or as silent data-loss corruption.
If the file still parses as plaintext config, the loader must keep the
credentials in memory and immediately rewrite encrypted-at-rest storage on
load.
That same rule also applies to local commercial activation persistence:
`pkg/licensing/activation_store.go` may keep `InstallationToken` and
`GrantJWT` in runtime activation state, but `activation.enc` may only accept
plaintext as migration input. Once a legacy plaintext activation file can be
read, canonical persistence must rewrite encrypted-at-rest storage immediately
on load.
That same trust boundary also covers the persisted commercial license itself:
`pkg/licensing/persistence.go` may keep the local license key and grace-period
metadata in runtime state, but `license.enc` may only accept plaintext as
migration input. Once a legacy plaintext license file can be read, canonical
persistence must rewrite encrypted-at-rest storage immediately on load instead
of allowing plaintext licensing state to remain on the runtime primary path.
That same shared token-settings boundary must stay under explicit proof routing
on both sides: `frontend-modern/src/components/Settings/APITokenManager.tsx`,
`frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, and
That same security settings presentation boundary also owns deployment-specific
restart guidance after auth changes. When `securityAuthPresentation.ts`
describes the development deployment, it must point at the canonical managed
runtime control surface (`npm run dev:restart` from the repo root), not a
stale `pulse-hot-dev` service name or any lane-local restart folklore.
`frontend-modern/src/components/Settings/useAPITokenManagerState.ts` must
continue to carry the direct `security-settings-surfaces` proof path together
with the API-contract token-management proof instead of borrowing coverage only
from broader settings-shell or API ownership.
That same token-settings surface must also derive presets lazily from the
canonical scope constants. `apiTokenManagerModel.ts` may expose a
`getAPITokenScopePresets()` factory, but it must not freeze preset scope data
at module-load time in a way that can break security settings initialization in
production chunks.
That same revoke/usage surface must also preserve canonical local operator
identity for the runtimes currently bound to a token. When token usage is
attributed to Docker hosts, agents, PBS, PMG, or similar monitored systems,
the security settings UI must keep the local instance name instead of swapping
in governed summary text, so the operator can revoke credentials against the
correct concrete system.
That same governed AI trust boundary also covers unified-resource context
posture derivation: `internal/ai/resource_context_policy_model.go` is now the
canonical owner for the policy-posture summary, local-only count, and
redaction-hint inputs that drive outbound AI context export decisions, so
`resource_context.go` does not duplicate trust-boundary policy assembly inline.
That same shared token-settings boundary now also governs relay pairing token
lifecycle. `internal/api/security_tokens.go`,
`internal/api/router_routes_auth_security.go`, and
`frontend-modern/src/api/security.ts` expose canonical single-token metadata
reads, expose the backend-owned Pulse Mobile relay access token creator, and
the relay pairing UI may revoke a displayed token only when that metadata still
shows no `lastUsedAt`. Refreshing or hiding a QR payload must not delete a
token that an already paired device is actively depending on.
That same auth/security boundary also owns browser session-capability posture:
`internal/api/router_routes_auth_security.go` together with
`internal/api/security_status_capabilities.go` must expose
`/api/security/status.sessionCapabilities.demoMode` as the backend-owned
public-demo posture signal, and security/privacy consumers must not infer demo
state from response headers, `/api/health`, or hostname heuristics. That same
session-capability contract now also carries the closed-shell assistant
availability fact through
`/api/security/status.sessionCapabilities.assistantEnabled`, so general
settings or security surfaces do not probe `/api/settings/ai` or other
assistant endpoints merely to decide whether dormant assistant chrome may be
opened.
That same token-management boundary now also depends on one neutral
app-runtime context owner. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`
may consume websocket-backed revocation fan-out through
`frontend-modern/src/contexts/appRuntime.ts`, but security/privacy authority
stays in the governed API token contract. The hook must not import `@/App` or
borrow root-shell ownership as token-management authority.
That same live auth-env reload boundary also owns watcher lifecycle cleanup:
`internal/config/watcher.go` must not return from `ConfigWatcher.Stop()` while
its fsnotify or polling goroutine can still read debounce or callback state.
Stopping the watcher is the synchronization point that lets tests and runtime
teardown restore auth/config state without racing a background reload.
