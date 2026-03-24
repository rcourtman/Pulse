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

Own Pulse's canonical privacy disclosures, anonymous telemetry boundary, and
the security-facing settings surfaces that expose authentication posture,
token-management visibility, and privacy controls to operators.

## Canonical Files

1. `SECURITY.md`
2. `docs/PRIVACY.md`
3. `frontend-modern/src/api/security.ts`
4. `frontend-modern/src/components/Settings/APITokenManager.tsx`
5. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`
6. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
7. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
8. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`
9. `frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`
10. `frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`
11. `frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`
12. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`
13. `frontend-modern/src/components/Settings/useSystemSettingsState.ts`
14. `frontend-modern/src/utils/apiTokenPresentation.ts`
15. `frontend-modern/src/utils/auditLogPresentation.ts`
16. `frontend-modern/src/utils/auditWebhookPresentation.ts`
17. `frontend-modern/src/utils/securityAuthPresentation.ts`
18. `frontend-modern/src/utils/securityScorePresentation.ts`
19. `internal/api/security.go`
20. `internal/api/security_tokens.go`
21. `internal/api/system_settings.go`
22. `internal/telemetry/telemetry.go`
23. `internal/api/router_routes_auth_security.go`

## Shared Boundaries

1. `frontend-modern/src/api/security.ts` shared with `api-contracts`: the security frontend client is both a security/privacy control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/components/Settings/APITokenManager.tsx` shared with `api-contracts`: the API token settings surface is both a security/privacy control surface and a canonical API payload contract boundary.
3. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` shared with `api-contracts`: the pure API token settings model is both a security/privacy control surface and a canonical API payload contract boundary.
4. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `frontend-primitives`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
5. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `frontend-primitives`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
6. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `frontend-primitives`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
7. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts` shared with `api-contracts`: the API token settings state hook is both a security/privacy control surface and a canonical API payload contract boundary.
8. `frontend-modern/src/utils/apiTokenPresentation.ts` shared with `api-contracts`: the API token presentation helper is both a security/privacy control surface and a canonical API token management boundary.
8. `internal/api/security.go` shared with `api-contracts`: the security handlers are both a security/privacy control surface and a canonical API payload contract boundary.
9. `internal/api/security_tokens.go` shared with `api-contracts`: the security token handlers are both a security/privacy control surface and a canonical API payload contract boundary.
10. `internal/api/system_settings.go` shared with `api-contracts`: the system settings telemetry and auth controls are both a security/privacy control surface and a canonical API payload contract boundary.

## Extension Points

1. Change privacy disclosures or outbound-data guarantees through `docs/PRIVACY.md` and `internal/telemetry/telemetry.go` together.
2. Change security policy, hardening guidance, or supported auth boundaries through `SECURITY.md`.
3. Change telemetry/privacy settings state handling through `frontend-modern/src/components/Settings/useSystemSettingsState.ts`.
4. Change security/auth/token transport behavior through the shared `frontend-modern/src/api/security.ts`, `frontend-modern/src/components/Settings/APITokenManager.tsx`, `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`, `internal/api/security.go`, `internal/api/security_tokens.go`, and `internal/api/system_settings.go` boundary.
5. Change security/privacy settings presentation through the shared `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`, `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`, `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`, `frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`, `frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`, `frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`, `frontend-modern/src/utils/securityAuthPresentation.ts`, `frontend-modern/src/utils/securityScorePresentation.ts`, `frontend-modern/src/utils/auditLogPresentation.ts`, and `frontend-modern/src/utils/auditWebhookPresentation.ts` boundary.

## Forbidden Paths

1. Changing telemetry payload semantics without updating the canonical privacy disclosure.
2. Letting security-facing settings copy or privacy guarantees drift between runtime behavior and the governed docs.
3. Treating API token management, auth posture, or telemetry controls as generic settings-shell polish instead of explicit trust-surface behavior.

## Completion Obligations

1. Update privacy/security docs and the telemetry runtime together when outbound-data behavior changes.
2. Keep shared API-contract proof routing aligned whenever auth, token, or telemetry settings payloads change.
3. Keep shared frontend settings proof routing aligned whenever security/privacy presentation changes.
4. Update this contract whenever a new canonical security, token, auth, or privacy surface becomes part of the governed trust boundary.

## Current State

This subsystem now gives `L14` an explicit governed home for privacy guidance
and telemetry disclosures instead of leaving those trust surfaces as lane-level
evidence with no subsystem ownership.

Security-facing settings remain intentionally shared with `frontend-primitives`
because shell framing and presentation consistency still belong there, but the
meaning of those surfaces now lives here so auth posture, token controls, and
privacy toggles stop borrowing their governance only from adjacent lanes.
That shared settings boundary now also has an explicit split of responsibilities:
`frontend-modern/src/components/Settings/useSystemSettingsState.ts` remains the
canonical owner for telemetry, local-upgrade-metrics, and auth/privacy runtime
state, while `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
stays a presentation boundary and `frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx`
may only assemble props for the shared settings shell. Privacy or telemetry
behavior must not drift into `frontend-modern/src/components/Settings/Settings.tsx`
or the registry hook just because the shell wiring changed.
That shell split now also applies to tab-save coordination: the dedicated
`frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts` owner may
decide which settings tabs participate in shell-level save prompts, but it must
remain pure shell metadata. Telemetry, local-upgrade-metrics, and auth/privacy
state transitions stay canonically owned by
`frontend-modern/src/components/Settings/useSystemSettingsState.ts` and the
backing `frontend-modern/src/stores/systemSettings.ts` trust surface, not by
settings navigation metadata or other frontend-primitives owners.

The security transport surfaces remain intentionally shared with
`api-contracts`: token, auth, and telemetry settings payloads are still API
contracts, but they now also count as first-class security/privacy runtime
behavior that `L14` must govern directly.
That shared token-management boundary now also includes
`frontend-modern/src/utils/apiTokenPresentation.ts`, so API-token load,
generate, and revoke errors stay on one governed customer-facing wording path
instead of drifting back into hook-local notification strings.
That same governed settings trust boundary now also includes
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
That same token-scope boundary now also governs Pulse Mobile relay runtime
credentials: `internal/api/security_tokens.go` must mint only the dedicated
backend-owned `relay:mobile:access` scope for new mobile relay tokens, and the
shared auth/router helpers may expose backward-compatible gates for older
mobile tokens only on the governed mobile runtime routes. Browser callers and
route-local handlers must not recreate wildcard or broad AI-scoped mobile
credentials.
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
