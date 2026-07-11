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
    "agent-lifecycle",
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
29. `pkg/server/telemetry_pulse_intelligence.go`
30. `internal/api/router_routes_auth_security.go`
31. `internal/crypto/crypto.go`
32. `internal/securityutil/secure_storage_dir.go`
33. `internal/cloudcp/auth/magiclink.go`
34. `internal/cloudcp/auth/magiclink_store.go`
35. `pkg/tlsutil/fingerprint.go`
36. `pkg/audit/audit.go`
37. `pkg/audit/async_logger.go`
38. `pkg/audit/sqlite_logger.go`
39. `scripts/telemetry_adoption_report.py`
40. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx`
41. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts`
42. `internal/api/agent_exec_token_binding.go`
43. `internal/logging/logging.go`

## Shared Boundaries

API token scope copy must match runtime authority. `ai:chat` covers Assistant
conversation, model selection, sessions, and knowledge reads only. Knowledge
save/delete/import/clear and explicit governed action approval/execution
require `ai:execute`; relay-mobile chat access does not inherit it. Browser
sessions retain their authenticated product permissions, while token requests
with missing, unknown, or unrelated scopes fail closed.

1. `frontend-modern/src/api/security.ts` shared with `api-contracts`: the security frontend client is both a security/privacy control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/components/Settings/APIAccessPanel.tsx` shared with `frontend-primitives`: the API Access settings intro is both a security/privacy token-management trust surface and a canonical settings-shell presentation boundary.
   Its Docker / Podman token wording must come from
   `frontend-modern/src/utils/apiTokenPresentation.ts` rather than page-local
   copy. The scope-reference action may compose frontend-primitives'
   `ButtonLink` info variant for external docs-link chrome and new-tab safety;
   security-privacy owns the scope trust copy, not the anchor shell.
3. `frontend-modern/src/components/Settings/APITokenManager.tsx` shared with `api-contracts`: the API token settings surface is both a security/privacy control surface and a canonical API payload contract boundary.
   Token-management table rows are security-facing content, but the visual
   table frame and scroll shell belong to `frontend-primitives`
   `PulseDataGrid`; do not add token-surface-local overflow, side-border, or
   negative-margin wrappers around the inventory grid. Scope-reference
   documentation links compose `ExternalTextLink` for shared rel/target safety
   and link chrome.
   API token scope selectors follow the same split: security/privacy owns
   the wildcard, preset, and custom scope semantics, while frontend-primitives
   owns the pressed selector pill chrome through `SelectablePillButton`.
   Full access is a deliberate wildcard choice, not the default empty
   selection. The token creation form must require an explicit scoped preset,
   custom scope, or Full access selection before a credential can be minted.
   Stable in-page anchors for sibling API Access onboarding panels are allowed
   only as navigation into the token creation section; those sibling panels do
   not own token scope derivation or preset contents.
4. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` shared with `api-contracts`: the pure API token settings model is both a security/privacy control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` shared with `frontend-primitives`: the data-handling settings surface is both a security/privacy trust surface and a canonical settings-shell presentation boundary.
6. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` shared with `frontend-primitives`: the data-handling settings model is both a security/privacy posture projection and a canonical settings-shell presentation boundary.
7. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx` shared with `frontend-primitives`: the general settings privacy panel is both a security/privacy control surface and a canonical settings-shell presentation boundary.
   Privacy documentation links compose `ExternalTextLink`; security-privacy
   owns the telemetry/privacy meaning and retention copy. Localized settings
   copy for this surface may route through `frontend-modern/src/i18n/messages.ts`
   and `frontend-modern/src/i18n/policy.ts`, but translation must preserve the
   governed privacy guarantees and leave machine-facing tokens such as
   `PULSE_TELEMETRY`, API fields, config keys, commands, logs, and product or
   source identifiers untranslated.
8. `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx` shared with `frontend-primitives`: the authentication settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
9. `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx` shared with `frontend-primitives`: the security overview settings surface is both a security/privacy control surface and a canonical settings-shell presentation boundary.
   Security guide links compose `ExternalTextLink`; security-privacy owns the
   hardening and proxy-auth guidance semantics.
10. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts` shared with `api-contracts`: the API token settings state hook is both a security/privacy control surface and a canonical API payload contract boundary.
11. `frontend-modern/src/constants/apiScopes.ts` shared with `api-contracts`: the API token scope catalog is both a security/privacy token-management trust surface and a canonical API token payload boundary.
    Scope labels and descriptions are visible security controls. Docker /
    Podman scopes must use the shared source-platform label rather than
    generic `container` copy.
    The `ai:execute` scope must stay labeled and described as Pulse
    Intelligence actions for governed Patrol actions: plans, approvals,
    policy-allowed fixes, verification, and history. Security-facing token
    setup must not present it as generic operations workflow access.
12. `frontend-modern/src/utils/apiTokenPresentation.ts` shared with `api-contracts`: the API token presentation helper is both a security/privacy control surface and a canonical API token management boundary.
    It owns Docker / Podman token copy for API Access, token presets, usage
    summaries, and revoke warnings so security-facing copy does not drift into
    page-local `container runtime` labels.
13. `internal/api/security.go` shared with `api-contracts`: the security handlers are both a security/privacy control surface and a canonical API payload contract boundary.
    SSO session status must distinguish stable identity from presentation:
    `ssoSessionUsername` remains the provider-scoped principal used for
    authorization-sensitive comparisons, while `ssoSessionDisplayName` is
    display/contact metadata for app chrome. Security/privacy surfaces may show
    the display label, but they must not use mutable username, email, or name
    claims as proof of admin, organization owner, token owner, or tenant
    membership.
14. `internal/api/security_tokens.go` shared with `api-contracts`: the security token handlers are both a security/privacy control surface and a canonical API payload contract boundary.
    Pulse Mobile relay token creation is a security token-management surface,
    but it is not a free API-token convenience. After admin and
    `settings:write` authorization, `POST /api/security/tokens/relay-mobile`
    must fail closed with the standard license-required response unless the
    active entitlement includes the paid `relay` feature.
15. `internal/api/system_settings.go` shared with `api-contracts`: the system settings telemetry and auth controls are both a security/privacy control surface and a canonical API payload contract boundary.
    Remote command authorization is also a trust boundary: security-facing
    copy and controls must distinguish desired command policy from applied
    agent runtime truth. `/api/connections` `fleet.commandPolicy` is the
    source for desired, applied, enforcement, and reason; top-level
    `remoteControl` or `commandsEnabled` must not be used to imply that a
    desired server state is already enforced on the agent when the applied
    report is missing or divergent.
    Report branding settings are also a trust-surface payload because they
    can carry operator-authored names and logo material into generated PDFs.
    `reportBranding` updates must validate object shape, supported keys,
    string types, bounded lengths, newline-free values, supported logo formats,
    and valid bounded base64 before persistence. Workspace settings must not
    accept local filesystem `logoPath` values; file-backed logo paths are
    provider-default runtime configuration only. Rendering custom branding
    remains gated by the `white_label` entitlement in the reporting layer, so
    storing a brand setting never becomes a free branding bypass.
16. `internal/cloudcp/auth/magiclink.go` shared with `cloud-paid`: control-plane magic-link HMAC handling is both a Pulse Cloud account-access boundary and a security/privacy token-secrecy boundary.
17. `internal/cloudcp/auth/magiclink_store.go` shared with `cloud-paid`: control-plane magic-link persistence is both a Pulse Cloud account-access boundary and a security/privacy storage-hardening boundary.
## Extension Points

Catalog edits in `frontend-modern/src/i18n/` that add or promote Patrol-trigger
copy (such as an alert's primary "Have Patrol investigate" action) must stay
non-disclosing: the manual Patrol trigger carries resource identity only —
resource ids plus alert identifier/type — and injects no operator briefing,
command, prompt, or remediation payload into the model beyond the existing
scoped-investigation context, so it adds no new disclosure surface and the
existing resource-policy redaction still governs any model-bound context.

Docker and Podman container CPU normalization may expose numeric raw per-core
CPU percent, normalized capacity percent, and reporting host CPU count in
resource or alert metadata. Those fields are operational usage telemetry only;
they must not be expanded into command lines, environment variables, secret
material, or unbounded container inspection output at the API boundary.

Scheduled report management under `/api/admin/reports/schedules` is a
settings/reporting control surface, not a new public data export. It must reuse
the existing reporting feature gate and settings read/write scopes, persist
workspace-local schedule metadata only, and never add cross-tenant report
creation, unauthenticated delivery, raw SMTP secret exposure, or bypasses for
the `white_label` branding entitlement.

1. Change privacy disclosures, usage-data vocabulary, or outbound-data guarantees through `docs/PRIVACY.md`, `frontend-modern/public/docs/PRIVACY.md`, `internal/telemetry/telemetry.go`, and `pkg/server/telemetry_pulse_intelligence.go` together.
   Pulse Intelligence external-agent/MCP telemetry may expose only content-free
   adapter-origin usage and capability-class counters for context, event
   stream, provisioning, operator state, finding, and action requests. It must
   not expose token identity, route parameters, resource IDs, finding text,
   command text, action output, prompts, responses, or request bodies.
   External-agent activity markers may be recorded for narrow tokens that
   satisfy the called manifest capability's own scope, such as
   `monitoring:read` for context reads, but that does not widen token
   permissions or export token identity. The emitted telemetry remains only the
   coarse activity class.
   External-agent/MCP readiness for the operations loop may likewise be true
   only when a single non-expired API token satisfies every scope required by
   the published Pulse MCP operations-loop capability set; readiness must not
   require the full manifest scope set and must not export token identity,
   token name, token counts, or matched scopes.
   The broad external-agent configured signal may remain true for a narrower
   read-only MCP token, but Patrol autonomy completed/resolved loop telemetry no
   longer uses MCP readiness as a value gate; readiness remains optional
   external-agent setup telemetry and settings handoff context.
   Pulse Intelligence guided operations-loop starter telemetry may expose only
   content-free 30-day request counts for the total starter flow and the
   coarse Assistant, Pulse Patrol, Patrol control, legacy Patrol autonomy
   compatibility, legacy Pro activation
   entry-point, and Pulse MCP source surfaces. It must not expose prompt text,
   prompt arguments, resource
   IDs, finding IDs, session IDs, token identity, checkout/account identity,
   request bodies, model output, remediation command text, or
   infrastructure-specific details.
   Pulse Intelligence Patrol control completed-loop telemetry may expose only a
   content-free boolean derived from the Patrol control starter or legacy Patrol
   autonomy/Pro activation entry-point aliases, Patrol issue evidence,
   contextual Assistant or external-agent collaboration, and either a rejected
   governed decision or an approved governed decision with verified outcome
   proof. Pulse Intelligence Patrol control resolved-loop telemetry remains
   stricter: it may expose only a content-free boolean derived from the same
   evidence plus an approved governed decision and verified outcome proof. Paid
   Patrol control completed/resolved loop cohorts may expose only whether the
   current coarse paid-license posture coexists with those same primary Patrol
   control completed/resolved booleans. Legacy Pro activation completed,
   resolved, and paid cohort fields may remain as mirrors for longitudinal
   commercial analysis, but they must not add exact tier, checkout, account,
   license, token, or customer identity. None of these fields may expose prompt text,
   prompt arguments, checkout/account identity, token details, resource IDs,
   finding IDs, session IDs, request bodies, remediation command text, action
   output, or infrastructure-specific details.
   The shared count-only classifier in
   `internal/telemetry.ClassifyPulseIntelligencePatrolControlProof` is the
   privacy boundary for those Patrol control booleans and the native
   `patrolControlValueState` string. The legacy
   `ClassifyPulseIntelligencePatrolAutonomyProof` and
   `ClassifyPulseIntelligenceProActivationProof` wrappers plus
   `patrolAutonomyValueState` and `proActivationValueProofState` aliases may
   remain for metric/storage continuity, but callers may pass aggregate
   evidence counts only, never external-agent readiness, prompt text, request
   bodies, resource/finding identifiers, token metadata, actors, commands, or
   outputs.
   The agent operations-loop status endpoint may mirror that same starter
   evidence and contextual Assistant/external-agent collaboration evidence only
   as aggregate count fields in its content-safe payload; it must not expose the
   underlying workflow-prompt event records, AI prompt or response content,
   Assistant session IDs, external-agent route parameters, surfaces beyond the
   approved coarse categories, token metadata, prompt names, or request context.
   The same endpoint may expose aggregate active Patrol finding counts and let
   active findings or pending approvals outrank historical completed/resolved
   proof in `nextAction`, but that precedence must remain count-only and must
   not expose finding IDs, resource IDs, commands, prompt text, actors, token
   metadata, or remediation output.
2. Change security policy, hardening guidance, or supported auth boundaries through `SECURITY.md`.
3. Change telemetry/privacy settings state handling through `frontend-modern/src/components/Settings/useSystemSettingsState.ts`.
   Relay runtime access through `internal/api/router.go` must stay behind the
   existing protected route and API-token gates. Testable router seams may
   expose relay status to onboarding validation, but they must not broaden
   Pulse Mobile token scopes, bypass the server-minted credential requirement,
   or expose relay secrets beyond the existing public onboarding diagnostics.
4. Change security/auth/token transport behavior through the shared `frontend-modern/src/api/security.ts`, `frontend-modern/src/components/Settings/APITokenManager.tsx`, `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`, `internal/api/security.go`, `internal/api/security_tokens.go`, and `internal/api/system_settings.go` boundary.
   Local username/password verification in `internal/api/auth.go` and
   `internal/api/router.go` must snapshot `AuthUser` and `AuthPass` under
   `config.Mu.RLock()` before comparison. Security/privacy may consume that
   shared auth result, but it must not read mutable credential fields outside
   the shared config lock or hold the lock across password hashing, session
   mutation, SSO provider checks, or response writing.
   Release metadata surfaced through `/api/version` remains outside token,
   auth, and privacy state. Adding or changing `agentUpdateTargetVersion`
   must stay limited to non-secret deployable release identity and must not
   expose agent inventory, scoped update selections, or command authorization
   state.
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
   username/email/display claims may not be written as the session owner. Those
   mutable claims may persist only as display metadata for user-facing chrome.
   Live organization authorization follows the same trust boundary: contact
   email can support display, delivery, or migration, but request access must
   match the authenticated principal against stored `OwnerUserID` or member
   `UserID`.
5. Change security/privacy settings presentation through the shared `frontend-modern/src/components/Settings/APIAccessPanel.tsx`, `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`, `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`, `frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx`, `frontend-modern/src/components/Settings/QuickSecuritySetup.tsx`, `frontend-modern/src/components/Settings/SecurityPostureSummary.tsx`, `frontend-modern/src/components/Settings/SSOProviderTypeIcon.tsx`, `frontend-modern/src/constants/apiScopes.ts`, `frontend-modern/src/utils/apiTokenPresentation.ts`, `frontend-modern/src/utils/securityAuthPresentation.ts`, `frontend-modern/src/utils/securityScorePresentation.ts`, `frontend-modern/src/utils/auditLogPresentation.ts`, `frontend-modern/src/utils/auditWebhookPresentation.ts`, and the localized catalog/policy boundary in `frontend-modern/src/i18n/`. Locale-catalog edits owned by another product surface may share this boundary only if they preserve API token names, token preset ids, privacy disclosures, and non-translatable security terms exactly; changing those security/privacy strings requires the security/privacy owner and tests.
   Pulse Intelligence Provider & Models, Patrol, Assistant, and Service Context
   settings labels may use the same localized catalog boundary, but those edits
   must stay product-settings copy only and must not change token scope names,
   preset ids, privacy disclosures, or security control terminology. Self-hosted
   Plans & Billing header and navigation localization may share that same catalog
   boundary when it frames Pro setup as choosing Patrol autonomy; it must not
   alter API Access, authentication, privacy, or token-management terminology in
   the same edit.
   Commercial pricing handoff localization may share the same catalog boundary
   only for redirect/manual-link copy and must preserve `Pulse Account`,
   security/privacy disclosures, token names, API field names, route/query keys,
   and purchase-return state exactly.
6. Change operator-facing telemetry/adoption reporting through `scripts/telemetry_adoption_report.py` together with the privacy disclosure whenever release-identity interpretation changes.
7. Change data-at-rest encryption-key or control-plane magic-link HMAC key and storage-root hardening semantics through `internal/crypto/crypto.go`, `internal/cloudcp/auth/magiclink.go`, `internal/cloudcp/auth/magiclink_store.go`, and `internal/securityutil/secure_storage_dir.go` together so writable-but-not-owned runtime storage mounts stay supported without weakening file-level secrecy.
   Control-plane portal session lifetime rides on that same service: the auth
   service session TTL is configurable (`CP_SESSION_TTL`, longer
   provider-hosted MSP default) but must stay bounded; non-positive overrides
   are ignored so a misconfigured caller cannot issue never-expiring or
   instantly-expired sessions, and session issuance sites must read the
   service TTL instead of the package constant.
8. Change auth-env password normalization, hosted commercial base URL
   normalization, or shared TLS fingerprint verification defaults through
   `internal/config/config.go`, `internal/config/watcher.go`, and
   `pkg/tlsutil/fingerprint.go` together so startup auth ingestion, live
   auth-env reloads, hosted entitlement refresh origins, and
   pinned-fingerprint TLS clients keep one fail-closed security floor.
9. Change operator-facing Resource Privacy/Data Handling posture through `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` and `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` together so resource classification, handling-boundary, redaction copy, and the route-backed/hidden-sidebar presentation stay governed as a trust surface.
10. Change inside-guest runtime collection boundaries through `docs/AGENT_SECURITY.md`, `docs/UNIFIED_AGENT.md`, `cmd/pulse-agent/main.go`, `internal/api/router.go`, and `internal/config/config.go` together. Docker / Podman inventory inside a VM or LXC may come from a guest-local `pulse-agent` module or explicitly reported guest data; LXC Docker inventory may also be collected by a Proxmox host agent only through explicit server opt-in, with optional VMID allowlisting and a minimal summary command set that avoids `docker inspect`, environment, mount, file, command, and process collection. Local Unified Agent Docker / Podman disables must not be reversed by remote profile configuration, and self-test/update preflight that needs the live runtime token must pass it through a short-lived token file rather than argv. The `--enable-docker` help line is part of that operator privacy control, so it must remain "Enable Docker / Podman Agent module" instead of exposing internal collection-module wording. The `--enable-commands` help line and installer disclosure must identify Pulse command execution as disabled by default and required for Patrol actions or the explicit Proxmox LXC Docker inventory path, not as implicit guest access.
    Agent file logging is local operational state, not a second telemetry path:
    `cmd/pulse-agent/main.go` must use the canonical owner-only rotating sink,
    retain that sink when remote configuration changes log level, and never
    place runtime tokens or enrollment secrets in the service command or log
    output.
    Global resource timeline reads through `/api/resources/timeline` are
    adjacent monitoring-read surfaces, not a privacy bypass. Provider activity
    filters may expose backend-authored task/event metadata, but the endpoint
    must keep normal API auth, resource-policy redaction, and inside-guest
    runtime collection limits intact rather than expanding what collectors are
    allowed to gather.
11. Change Agent context, discovery-readiness, or action-related route wiring
    through `internal/api/router.go` without weakening the existing
    `RequireAuth` and scope checks, resource-policy redaction pass, or
    read-only Agent-context boundary. Router glue may connect providers, but
    it must not become an alternate command path, raw provider-command path,
    config path, environment path, or secret-bearing metadata path.
    The command-authorization bridge wired by `internal/api/router.go` preserves
    that rule: public chat and relay input cannot serialize its org/action
    authorization context, and invalid approvals fail before signing or agent
    dispatch rather than falling through to a route-local trust shortcut.
    The Patrol action-broker and proposal-catalog factory glue wired here is bound by the
    same rule: it may connect the investigation orchestrator to the tenant-bound
    action lifecycle, but it exposes only typed-proposal capture and gives the
    orchestrator no autonomy control, command execution, or command-shaped
    approval path.
    The typed host-update executor wired here is likewise not generic command
    authority. It may dispatch only the closed `install_os_updates` operation,
    bound to the server-observed package inventory fingerprint and canonical
    action approval. Package names, versions, raw APT output, and agent error
    text must not enter model context or action results; reboot remains a
    reported fact rather than an authorized operation.
    The typed storage-cleanup executor is equally closed: only
    `clean_package_cache` may cross the boundary, bound to the server-observed
    fingerprint and canonical approval. The model and API never supply a path,
    command, package selector, or deletion rule. Cache entry names,
    fingerprint, raw APT output, and agent error text remain out of model
    context and terminal action output.
    Router glue may also pass monitor-owned source freshness thresholds into
    unified-resource adapters, but those thresholds are operational cadence
    metadata only. They must not disclose credentials, command output, raw
    provider payloads, tenant-crossing config, or any new resource-policy bypass
    through monitoring-readable API responses.
    The Pro update credential source in router glue hands the activation's
    installation token, instance fingerprint, and license server URL to the
    server updater only. The token travels solely as an Authorization header
    to the activation's normalized license-server base URL; it must never be
    logged, echoed through update payloads, status, or history surfaces, or
    sent to any other host, and the broker's short-lived signed artifact URLs
    are transport only and must not be persisted or exposed.
	    Assistant session rename routing through `PATCH /api/ai/sessions/{id}`
	    stays on that same auth/scope boundary: the route may accept only a
	    user-visible title mutation, must not expose transcript contents,
	    provider-bound model context, tool evidence, approvals, or action state,
	    and must not treat title text as trusted secret-bearing or command-bearing
	    input.
	    Assistant session undo/redo routing through
	    `POST /api/ai/sessions/{id}/undo` and
	    `POST /api/ai/sessions/{id}/redo` stays on that same trust boundary:
	    responses may expose only browser-safe repair metadata such as restored
	    prompt text, removed/restored message counts, and `can_redo`; they must
	    not expose redo-stack internals, provider reasoning, raw tool output,
	    model-only handoff text, approval payload internals, environment data, or
	    command-bearing fix details.

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
5a. Keep localized privacy and telemetry settings copy covered by catalog
    completeness, fallback, and non-translatable-token tests so translated
    surfaces cannot weaken the governed privacy disclosure or turn machine
    identifiers into localized prose.
6. Keep Security Overview and Resource Privacy/Data Handling loading
   placeholders on the shared `SettingsLoadingSkeleton` primitive. This
   subsystem owns the security/privacy posture semantics; frontend-primitives
   owns skeleton animation, fill tokens, and placeholder shell consistency.
6a. Keep API token refresh/loading indicators on the shared `LoadingSpinner`
    primitive. Security/privacy owns the token-management trust copy and
    refresh semantics; frontend-primitives owns spinner shell, tone, and
    accessible status behavior.
6b. Keep API token scope selector pills on the shared `SelectablePillButton`
    primitive. Security/privacy owns scope authority, wildcard behavior, preset
    membership, and custom scope toggles; frontend-primitives owns active and
    inactive pill tone, focus, disabled treatment, and pressed-state wiring.
6c. Keep authentication setup, password-change, and credential-rotation actions
    on the shared `Button` primitive. Security/privacy owns the auth authority,
    setup/rotation semantics, and read-only capability state;
    frontend-primitives owns warning, primary, secondary, focus, disabled, and
    settings-action chrome.
6. Keep the shared storage-directory and secure storage-file hardening helper aligned with the crypto manager plus control-plane magic-link key and store handling whenever runtime data-root ownership assumptions change.
7. Keep auth-env ingestion, hosted commercial base URL validation, and shared
   fingerprint-verifier TLS defaults aligned whenever runtime auth loading,
   hosted entitlement refresh origin handling, or pinned-certificate transport
   behavior changes. Hosted commercial URL overrides must remain absolute
   HTTP(S) URLs, with plain HTTP limited to loopback development origins.
8. Keep the Resource Privacy/Data Handling settings surface neutral and non-commercial: it may show resource policy posture, local-only counts, and redaction coverage, but it must not advertise trials, upgrades, paid plans, or monitoring limits, and it must remain route-backed rather than promoted in the normal Settings sidebar while it is informational only.
9. Keep operator-facing Resource Privacy/Data Handling posture aligned with runtime AI/context enforcement: `local-only` resource details must not be sent to external model prompts, and sensitive free-form alert, tool-result, investigation, handoff context, and any retained legacy managed-model compatibility text must use the shared resource-policy redaction helper before leaving the local trust boundary. Assistant handoffs may surface canonical policy handling guidance and current resource-state summaries for product-originated resources, but that guidance and state are model-only context and must not become disclosure authority. Product-originated Assistant handoff text must also be policy-cleaned before prompt injection, including operator briefings and finding/action context, so raw governed resource identity cannot leak through local-model briefing prose while non-local transport still receives the final provider-bound sanitizer. All provider-bound AI requests to non-local models must use the shared resource-policy sanitizer immediately before transport so later agentic turns cannot bypass the advertised handling posture.
   Native Pulse Assistant provider seams and native tool-adapter names in the
   shared AI/API route wiring are part of that same trust boundary. `MCP`
   remains an external protocol, manifest, and wire-schema term; the in-app
   Assistant `ToolAdapter` family must stay governed by the same sanitizer,
   approval, auth, and action-audit checks as the rest of AI/runtime. Security
   and privacy code must not treat MCP-named native seams as a separate trust
   boundary, and must not bypass provider-bound redaction or approval controls
   because a tool call is replayed through Assistant route wiring.
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
12. Keep inside-guest runtime visibility explicit: Pulse may show Docker /
    Podman workloads from a VM or LXC when a guest-local agent or another
    explicit guest reporting path supplies that inventory. Pulse may
    additionally show LXC Docker workloads from a Proxmox host agent only when
    the server has explicitly enabled LXC Docker inventory collection; that
    path must remain read-only, VMID-allowlistable, and limited to Docker
    host/container summary fields plus aggregate stats, with no
    `docker inspect`, environment, mount, file, command, or process collection.

## Current State

Unified Agent Pulse transports share one fail-closed TLS policy across host,
Docker/Podman, Kubernetes, remote configuration, commands, and self-update.
Custom CA bundles and SHA-256 leaf-certificate pins are runtime trust inputs,
not monitoring data, and malformed pins must fail during client construction
instead of silently degrading to system roots or blanket skip-verification.
Installer persistence may carry the non-secret pin into service arguments, but
must not copy API tokens or certificate material into reports or diagnostics.

The browser-auth boundary now owns one request-derived cookie policy for every
session, CSRF, organization, SAML, magic-link, and cloud-handoff cookie. Secure
requests use Secure cookies and host-prefixed session names; explicitly
supported non-TLS self-hosted requests retain the bounded compatibility path
without allowing individual handlers to choose weaker attributes. Audit
Session cookies pass through a dedicated writer that forces `HttpOnly`; CSRF
and organization cookies use a separately named client-readable writer because
the frontend must read them. The request-derived `Secure` decision remains the
explicit compatibility boundary for supported non-TLS self-hosted deployments,
not a handler-local exception.
Audit backends persist events through `Logger.Record`; realtime projections omit raw
actor/IP identity and redact free-form details, keeping queryable audit storage
distinct from process logs. Certificate discovery and availability probing use
the single `tlsutil.PeerCertificateCaptureTLSConfig` TOFU boundary, while
fingerprint-pinned clients continue to require exact leaf-certificate identity;
the capture helper's `Unverified` name makes the pre-trust boundary explicit to
callers and static analysis.

The multi-tenant authorization boundary now also owns default-org token
scoping. An org-bound API token is a client-scoped credential: it must be
denied implicit access to the default org so a token that leaks from a client
site cannot read the provider's own estate, while authenticated users and
legacy unbound tokens keep default-org access for compatibility. The webhook
SSRF allowlist is the related instance-wide security setting: it must
propagate to every tenant org's notification manager (update, reload, and
tenant-monitor creation), because an allowlist that only the default org
observes silently denies legitimate per-client private webhook targets and
invites per-org security drift.

This subsystem now gives `L14` an explicit governed home for privacy guidance
and telemetry disclosures instead of leaving those trust surfaces as lane-level
evidence with no subsystem ownership.
The per-rule patrol alert-trigger policy is operator-authored input validated at
the API boundary before it reaches persisted AI config: the settings handler
(`internal/api/ai_handlers.go`) rejects any minimum-severity value other than
`warning` or `critical` and canonicalizes the alert-type allowlist (lowercase,
trim, drop blanks, de-duplicate) so untrusted request bodies cannot widen the
alert-driven investigation surface beyond the validated shape.
That same governed home now also owns the single customer-facing "usage data"
vocabulary for outbound usage telemetry. Local commercial activation and
license-recovery runtime records must stay out of ordinary Settings, support
diagnostics, outbound telemetry disclosure copy, and public configuration
reference tables.
Customer-facing telemetry disclosures and telemetry-enabled log copy must
describe the governed AI counters as coarse Patrol, Assistant, and
external-agent usage counters, not as Pulse Intelligence loop-adoption,
activation-loop, operations-loop, or value-proof internals.
That same operator-reporting boundary now also owns reusable latest-install
adoption baselines. `scripts/telemetry_adoption_report.py` must emit
windowed 24h, 72h, and 7d latest-install snapshots that split published
versions from unpublished or development builds, so RC adoption reads stop
depending on ad hoc SQL or one-off local helper scripts.
Pulse Intelligence derived governed-operation booleans must treat content-free MCP /
external-agent capability-class counters as external-agent collaboration
activity, not only the legacy `pulse_intelligence_external_agent_used_30d`
boolean. The `pulse_intelligence_mcp_adapter_used_30d` bit is an adapter-origin
marker for the `pulse-mcp` surface, while the aggregate external-agent
recent-use bit still represents direct HTTP and MCP adapter use together. The
runtime telemetry snapshot, checked-in adoption report, and commercial value
report must agree on that interpretation so class-only MCP usage and
adapter-specific MCP usage still contribute to governed-operation activity,
completed/resolved compatibility metrics, retention, and signal-to-paid proof
without adding prompts,
request bodies, command output, resource IDs, finding IDs, token identity, or
route parameters. Source-specific Pulse Intelligence loop booleans for native
Assistant, external-agent, and `pulse-mcp` adapter operations-loop,
approved-execution, approved-action-success, and resolved-loop stages are
allowed only as content-free 30-day adoption evidence over those same
privacy-safe counters; they must not introduce separate prompt, request,
approval, resource, finding, action-output, or token-identifying payloads.
The checked-in Pulse Intelligence adoption report must expose machine-readable
rate fields beside the privacy-safe counts for cohort and operations-funnel
outcomes: retention, latest-paid posture, observed free-to-paid conversion, and
signal-to-paid conversion. Text output may format those rates for humans, but
JSON consumers must not need to parse prose or recompute denominators to tell
whether Patrol, Assistant, MCP, and governed action usage drives activation,
retention, and paid conversion.
That same report must treat Patrol control as the primary paid value cohort and
operations-funnel stage. Legacy Pro activation telemetry may contribute to
Patrol-control cohorts as a compatibility source and may remain visible as a
legacy entry-point count, but report keys and funnel stages must not present
Pro activation as the first-class product loop.
That same storage hardening boundary now also owns secure regular-file
handling for secret-bearing local trust material and the control-plane
magic-link storage root. `internal/crypto/crypto.go`,
`internal/cloudcp/auth/magiclink.go`, and
`internal/cloudcp/auth/magiclink_store.go` must route encryption keys,
magic-link HMAC keys, and the magic-link SQLite store path through the shared
secure storage helpers so symlink, oversize, and non-regular file paths fail
closed instead of slipping past directory-only hardening.
Kubernetes Secret inventory is part of that same secret-handling boundary.
Agent collectors and unified-resource projections may expose Secret metadata,
type, labels, and data key names for platform inventory, but they must not read,
store, serialize, search, or display Secret data values. Secret inventory
policy metadata must remain `restricted` and `local-only` because names and key
names can still reveal deployment intent.

Security-facing settings remain intentionally shared with `frontend-primitives`
because shell framing and presentation consistency still belong there, but the
meaning of those surfaces now lives here so auth posture, token controls, and
privacy toggles stop borrowing their governance only from adjacent lanes.
That settings presentation boundary also owns trust-sensitive vocabulary around
operator access. `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
and `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` must use
monitoring/workspace wording for tours and read-only token presets instead of
reviving Dashboard-specific labels after the Dashboard route has been retired.
The Resource Privacy/Data Handling settings surface extends that trust boundary
to resource policy posture. It may expose the canonical sensitivity,
handling-boundary, and redaction counts that Pulse already applies to
resources, but it must stay informational, route-backed, hidden from the
normal Settings sidebar, and non-commercial so free/self-hosted operators are
not shown paywall, trial, upgrade, monitoring-limit prompts, or an empty
read-only destination inside a privacy surface.
That posture is now enforced at the AI provider boundary too: non-local model
requests must be sanitized from the same resource-policy metadata that powers
the Data Handling surface. Assistant finding handoffs may hydrate policy
guidance for the handed-over resources from that same metadata, but it remains
read-only model context and cannot authorize raw identifier disclosure. Hosted
quickstart traffic is retired from the Pulse runtime, so privacy governance must
not describe a live public hosted-model proxy for normal self-hosted v6 installs.
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
Telemetry preview, copy, and install-ID rotation controls keep their
security/privacy behavior in that surface, but their button chrome must compose
the frontend-primitives `Button` family instead of carrying privacy-local
compact action shells.
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
Security audit surfaces must also distinguish runtime mismatch from billing
failure. If `runtime-capabilities` blocks `audit_logging` with
`paid_runtime_required`, Audit Log and Audit Webhooks may explain that the
active Pro license needs the private Pulse Pro runtime, but they must not
expose license keys, billing identity, or plan-upgrade copy as part of that
security/privacy feature gate.
Audit-log storage availability is also a security/privacy trust boundary.
The `pkg/audit/` runtime package owns persistent audit-store classification:
transient SQLite busy/locked conditions must be retried and surfaced as
structured `audit_store_busy`, while missing, corrupt, readonly, or
uninitialized audit stores must surface as `audit_store_unavailable`. The
Audit Log settings surface may translate those stable API codes into recovery
copy, but it must not show raw internal server errors or collapse audit-store
state into a generic frontend failure.
The persistent audit reader must also tolerate legacy timestamp encodings that
were previously written into `audit_events.timestamp`, including Unix seconds,
SQLite datetime values, and Go wall-clock strings carrying a monotonic
`m=+...` suffix, so valid historical audit rows cannot make `/api/audit`
return `query_failed`.
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
The API Access tab now hosts an Agent Integrations section
(`frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`)
alongside the existing API Token Manager. The section reads
`/api/agent/capabilities` at mount and renders the declared
agent surface (capabilities grouped by category, stable error
codes, scopes) plus an MCP config snippet generated from the
deployment's own origin so an operator wiring Claude Desktop or
Claude Code sees the right base URL automatically. The section
does NOT introduce a new token-mint flow or auth path: tokens
still flow through the API Token Manager, and the snippet
documents the manifest-derived scopes the agent surface requires.
Pulse Intelligence owns the agent-surface disclosure so the operator sees MCP as
an access path over governed Patrol actions, while API Access owns the scoped
credential minted for that access path. Normal API Access visits keep the token
manager first; `/settings/security/api?tokenPreset=pulse-intelligence-agent#api-token-create`
may open token creation for the external-agent preset, but
`/settings/security/api#external-agent-setup` and legacy
`/settings/security/api#pulse-mcp-setup` links must redirect to the canonical
Pulse Intelligence Assistant setup route instead of placing Agent Integrations
inside the API Access trust surface.

That same token-management boundary also reserves token-owner identity for the
server-authenticated principal. Token-minting helpers must derive
`owner_user_id` from the authenticated session or caller token and reject any
extension metadata that attempts to overwrite that field. This applies beyond
the visible API-token manager: agent install command tokens, deploy bootstrap
tokens, enrollment runtime tokens, container runtime migration tokens, and
first-run/regenerated admin tokens must use the same shared server-side owner
setter rather than carrying owner identity in caller-controlled metadata.
That same command-token trust boundary also owns first-use binding for
Proxmox install-command tokens. `internal/api/agent_exec_token_binding.go` may
persist `bound_agent_id`, `bound_hostname`, and `bound_at` only for
Pulse-minted PVE/PBS install-command tokens when the command agent first
registers. Generic unbound `agent:exec` tokens, or tokens already bound to a
different hostname or agent ID, must fail closed so command execution cannot
cross hosts through reusable bearer credentials.
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
outbound usage telemetry contract and may not be reintroduced without updating this
trust boundary and the governed privacy disclosure together.
That same rule also applies at the license-server ingest and storage boundary:
server-side telemetry rows may preserve the canonical normalized version
identity plus those same coarse booleans, but they must not retain legacy
exact commercial tier or exact API-token count fields as first-class analytics
dimensions just because older clients once sent them.
That same outbound usage telemetry floor now also permits only privacy-safe
aggregate self-hosted adoption counters: counts of monitored platforms,
workloads, storage resources, physical disks, Ceph clusters, network shares,
TrueNAS and VMware resource categories, availability targets, and active
alerts. Those counts may describe scale and feature adoption, but they must not
include hostnames, resource IDs, infrastructure identifiers, credentials,
prompts, chat messages, command text, action output, token values, or personal
information.
That same outbound usage telemetry floor now also permits content-free update
funnel counters derived from local update history inside the same rotating
30-day telemetry window: update attempts, successful updates, failed or
rolled-back updates, and the latest coarse failure category. The category may
identify only the governed class (`download`, `signature`, `checksum`,
`disk_space`, `extract`, `backup`, `apply`, `restart`, `rolled_back`,
`cancelled`, or `unknown`). It must not export raw updater error text,
download URLs, command output, log lines, paths, hostnames, release asset URLs,
checksums, signatures, or operator-entered values.
That same outbound usage telemetry floor now also permits only content-free Pulse
Patrol control and governed Pulse Intelligence operations adoption flags and
counters inside the same rotating 30-day telemetry window:
configured/active/completed/resolved governed-operation and approved-execution
adoption booleans, primary Patrol-control completed/resolved and paid-cohort
adoption booleans, legacy Pro activation mirrors where needed for cohort continuity,
source-specific native
Assistant, external-agent, and `pulse-mcp` adapter operations-loop,
approved-execution, approved-action-success, and resolved-loop adoption
booleans, governed-operation workflow starter request counts for the canonical
`pulse_operations_loop` prompt split by total, native Assistant, first-party
Patrol, primary Patrol-control, legacy Pro entry-point, and Pulse MCP surfaces, Assistant AI call
counts, Assistant governed-context AI call counts, Assistant governed-tool call counts, Patrol AI call counts, Patrol run/
new-finding/investigation/resolved-finding/autofix counts, external-agent/MCP
readiness and recent-use booleans including the adapter-origin `pulse-mcp`
recent-use boolean, action-plan counts, approval-request counts,
rejected-action-decision counts, approved-action-decision counts,
approved-action-attempt counts, and approved-action-success counts. Those fields may measure whether Patrol,
Assistant, external agents, approvals, and governed actions form an adopted
governed operation, whether an operator entered the guided Patrol-control
starter, which source carried the stage, whether the Patrol control journey
reached a terminal approve/reject decision or the stricter
approved-and-verified resolved outcome, whether that path reached approved
action-execution depth, and
whether approved governed actions completed successfully, were rejected before
execution, or coincided with content-free Patrol resolution,
but they must not include tool names, tool inputs, tool outputs, prompts,
responses, chat messages, command text, action output, approval actors,
approval reasons, token values, token counts, resource IDs, finding IDs, or
other local identifiers.
Governed-operation workflow starter telemetry is entry-point evidence only: a
successful starter render may make the coarse active-loop boolean true, but it
must not by itself count as contextual collaboration, approved execution,
verification, resolved finding evidence, or a completed governed operation.
Completed governed-operation telemetry is approve/reject evidence, not a pending
request shortcut: `pulse_intelligence_complete_operations_loop_30d` and the
source-specific operations-loop booleans may be true only when the same
content-free telemetry window contains Patrol issue evidence, contextual
Assistant/MCP/external-agent collaboration, and either a rejected action
decision, an approved action decision, or approved execution evidence. Generic
Patrol runs, Patrol AI calls, action plans, and approval requests may contribute
to activity or governed-action reach, but they must not complete the loop
without issue-backed Patrol evidence and a real decision/outcome signal.
The public privacy disclosure table is the operator-facing inventory for that
same payload. `docs/PRIVACY.md` and
`frontend-modern/public/docs/PRIVACY.md` must name every
`update_*` and `pulse_intelligence_*` field exported by
`internal/telemetry.Ping` using update-funnel, Patrol control, and
governed-operation vocabulary, including source-specific Assistant,
direct external-agent, and `pulse-mcp` governed-operation booleans, workflow
starter counts including primary Patrol control and legacy Pro entry-point counts,
Patrol-control completed/resolved booleans, paid Patrol-control cohort booleans,
legacy Pro activation mirrors, rejected decisions, and
approved-action outcome counts, so runtime telemetry can never grow a Patrol
control or legacy activation signal that is invisible to operators inspecting
outbound usage data.
External-agent/MCP recent-use is derived from content-free authenticated
agent-surface capability activity by a manifest-capable API token, not from
broad API token last-use metadata. The activity class may identify only the
coarse manifest category being used, never resource IDs, finding IDs, node IDs,
request bodies, outputs, token identity, or prompt/chat content.
The `pulse-mcp` adapter may additionally mark requests with a content-free
surface-origin header so telemetry can distinguish adapter use from direct
HTTP agent use without recording the client identity, prompt, request payload,
route parameters, or local resource identifiers.
External-agent/MCP readiness is derived from a non-expired API token that
covers every scope required by the published Pulse MCP operations-loop
capability set. This keeps OpenCode, Claude Code, Claude Desktop, `pulse-mcp`,
and direct HTTP agent setups measurable only when they can run the same
governed loop, without treating generic `ai:chat` tokens as external-agent
readiness and without requiring the operator to grant every manifest scope.
The operations-loop status endpoint may expose only the resulting boolean; it
must not expose token identity, token names, token counts, token last-use
metadata, or the specific matching scopes.
The Pulse Pro license-server telemetry ingest may persist those same
content-free Pulse Intelligence fields only alongside the canonical coarse
`paid_license` posture and received timestamp, so
`scripts/telemetry_adoption_report.py` can summarize Patrol-control and governed-operation adoption,
7-day retention windows, latest paid/free posture, source-window
entry-to-retention cohorts, paid Patrol-control completed/resolved
cohorts, and observed free-to-paid conversion counts without linking telemetry
to customer accounts or storing exact commercial tiers. The report may also
derive or persist a completed governed-operation signal from those same content-free
fields, but completion may only mean observed Patrol
issue evidence plus Assistant governed-context or MCP collaboration activity
plus approved/rejected governed-action decision evidence inside the source window;
approved action success may only mean a content-free successful completion
counter derived from approved action audit state. Neither signal may imply that
Pulse stored prompts, findings, resource identifiers, command payloads,
verification detail, or action outputs to prove that linkage.
The stricter approved-execution loop signal may only mean that the same Patrol
issue evidence and Assistant governed-context or MCP collaboration
signals also coincided with at least one approved governed-action attempt in
the source window. It may not encode action targets, command text, execution
output, verification detail, approver identity, or approval rationale.
The resolved governed-operation signal is stricter again: it may only mean that
Patrol resolved or fix-verified at least one finding in the source window, the
same window had Assistant governed-context/tool or MCP/external-agent
collaboration, and at least one approved governed action completed
successfully. It may not encode finding IDs, resource IDs, fix details,
verification detail, command text, action output, approver identity, or a
causal claim that the approved action directly resolved the finding.
The Patrol control completed-loop status count follows that same content-free telemetry
evidence contract: it may only mean the same content-free window also had
Patrol issue evidence, contextual collaboration, and either a rejected governed
decision or an approved governed decision with verified outcome proof. Legacy
Patrol autonomy and Pro activation completed-loop fields may mirror that value
for compatibility, but must not add checkout/account identity, prompt content,
action identity, resource identity, finding identity, token identity, or a
causality claim.
The Patrol control resolved-loop status count follows that same content-free telemetry
evidence contract: it may only mean the same content-free window also had
Patrol issue evidence, contextual collaboration, an approved governed decision,
and verified outcome proof. It must not require MCP readiness, treat rejected
decisions as resolved proof, or add checkout/account identity, prompt content,
action identity, resource identity, finding identity, or token identity. Both
the status projection and outbound telemetry must derive these Patrol control
completed/resolved values through the shared `internal/telemetry` proof
classifier so privacy-sensitive reporting cannot drift into a richer runtime
event join in one caller.
That same outbound usage telemetry contract also treats `install_id` as a rotating
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
General settings privacy surface. That surface may stay concise, but it must
not claim a stronger privacy posture than the governed docs; if telemetry rows
are retained for a fixed window and IP addresses are not stored rather than
“never seen,” the summary copy must say that plainly.
That same shared trust boundary now also owns the TLS floor used by pinned-
fingerprint runtime clients. `pkg/tlsutil/fingerprint.go` may support
certificate-fingerprint capture and verification for self-signed deployments,
but every mode must still set an explicit minimum TLS version instead of
silently inheriting whatever older protocol floor the host runtime would allow.
The same shared client transport must not leak local infrastructure API
requests through inherited environment proxies: loopback, private, link-local,
CGNAT/Tailscale, mDNS/local, and single-label infrastructure hosts are direct
connections by default, while public endpoints may still honor the operator's
proxy environment. Proxy-bypass changes for this path require targeted TLS
client tests plus adjacent Proxmox, PBS, and PMG client coverage.
That same rule also applies inside shipped security guidance itself:
`SECURITY.md` and the synced `frontend-modern/public/docs/SECURITY.md` copy may
not bounce the operator back to GitHub `main` for section references that the
running build already owns locally. Their Relay security section must also use
the current Relay-and-higher entitlement boundary instead of stale Pro-only
license wording.
Agent-based Proxmox hardening guidance in those same security docs must also
point operators to the current Infrastructure install or upgrade command
surface and to post-report verification on the relevant platform page or
Machines view. It must not revive the retired Settings Agents install-command
route or imply that v6 can prove upgraded-agent state before the agent has
authenticated and reported.
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
SSO action chrome is intentionally not security-owned: add, edit, delete,
test, preview, copy, close, and modal footer controls in
`frontend-modern/src/components/Settings/SSOProvidersPanel.tsx` must compose
the frontend-primitives `Button`, `ActionIconButton`, and `CopyValueButton`
family while security/privacy owns the authority, capability, SAML/OIDC, and
principal-trust semantics behind those controls.
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
Metrics bearer-token transport is also part of this trust boundary:
`internal/config/config.go` owns `PULSE_METRICS_BIND_ADDRESS`, which defaults
the metrics listener to loopback, and the explicit
`PULSE_METRICS_ALLOW_INSECURE_REMOTE` escape hatch. Runtime metrics serving
must reject a configured bearer token on non-loopback plaintext HTTP unless
that override is set, so a UI/API bind address cannot silently widen scrape
credentials to a remote network.
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
Update-readiness checks may inspect loaded API token metadata to determine
whether agent reporting scope exists or has expired, but they must not expose
raw token values, token hashes, or owner metadata in the update plan payload.
Legacy `host-agent:*` continuity must be reported only after the loaded token
record has normalized to canonical `agent:*` scope.
That same token-scope boundary also owns audit-log least privilege: audit
event, verification, summary, export, and unified action/export audit reads
must require the dedicated `audit:read` scope instead of inheriting broader
monitoring or settings-read token access.
The same security boundary now depends on unified action-audit normalization:
persisted action records must identify the requester, resource, capability,
approval policy, preflight dry-run posture, and lifecycle state before they are
read through audit APIs, so audit history cannot silently accept an unscoped or
unplanned execution record.
Assistant handoff context may hydrate those normalized action-audit facts for
review, but that read is still model-only context: it must remain org-scoped,
must not expose raw command text or raw execution output, and must not grant
approval or execution authority.
Scoped Assistant `handoff_actions` from Patrol assessment handoffs may carry
only safe approval/action metadata for model-only refresh, including approval
IDs, action IDs, policy, expiry, dry-run posture, and proposed-fix labels; they
must not expose raw command or execution payloads or become an approval bypass.
Assistant operator briefings generated from Patrol findings follow the same
boundary: they may summarize approval IDs, proposed-fix IDs, risk, destructive
posture, and bounded evidence for model review, but they must not expose raw
command payloads, present Patrol-authored remediation guidance, or convert chat
into approval or execution authority.
Action planning and action decision mutations remain privileged runtime
control surfaces even though the decision endpoint does not execute the
capability. `POST /api/actions/plan` and
`POST /api/actions/{id}/decision` must both require the governed `ai:execute`
scope so API tokens cannot create or approve executable action intent through a
read-only or mobile-only grant. `POST /api/actions/{id}/execute` is governed by
the same `ai:execute` scope because it is the only API-owned handoff from
approved intent into capability execution; missing executors must fail closed
without creating execution lifecycle evidence.
Docker / Podman container lifecycle execution stays under that same privileged
handoff: the executor may use agent command execution only after scope,
approval/policy, stale-plan, operator-lock, source-freshness, and runtime
posture checks pass, and it must record redacted audit and verification facts
instead of exposing raw command text through monitoring-readable surfaces.
Proxmox VM/LXC lifecycle execution is governed by the same privileged action
handoff: `start`, `shutdown`, `reboot`, and `stop` may use a Proxmox node
command agent only after the API action scope, approval/policy, stale-plan,
operator-lock, fresh resource capability, and connected-agent checks pass. Raw
`qm` / `pct` command text and command output must remain action-executor/audit
implementation detail, with monitoring-readable surfaces receiving only
redacted result, verification, and readiness facts.
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
The manifest-derived full-surface preset may keep the internal
`pulse_intelligence_agent` id for route compatibility, but its visible label and
default token name must be `Patrol external agent` so API Access presents the
token as connected-agent access to Patrol work rather than an internal
Pulse Intelligence proof surface. Its description must frame the preset as
scopes for connected agents that use Pulse MCP or HTTP to read context and
request Patrol work, not as generic external-client access.
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
`internal/api/security_status_capabilities.go` must expose public demo posture
through `/api/security/status.presentationPolicy.demoMode`; authenticated
responses may additionally expose `sessionCapabilities.demoMode` as caller
context. Security/privacy consumers must not infer demo state from response
headers, `/api/health`, or hostname heuristics. That authenticated
session-capability contract now also carries the closed-shell assistant
availability fact through
`/api/security/status.sessionCapabilities.assistantEnabled`, so general
settings or security surfaces do not probe `/api/settings/ai` or other
assistant endpoints merely to decide whether dormant assistant chrome may be
opened.
Security status disclosure is tiered by construction: public callers receive
only login/setup discovery, authenticated callers receive their own identity
and capability context, and deployment, network, credential, token-hint,
audit, proxy-configuration, and agent URL details require an admin session or
a `settings:read` API token. Bootstrap paths and container identifiers are not
security-status fields. Initial service restart requires either normal
admin/settings-write authorization or the rate-limited bootstrap token, and
quick security setup rejects structurally unsafe local usernames before it
mutates runtime state or renders `.env` and systemd configuration.
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
That same server-bind config boundary now also owns optional agent-ingest
network isolation. `internal/config/config.go` may accept
`PULSE_AGENT_INGEST_PORT` as a dedicated listener for agent report and
management traffic so operators can place `/api/agents/*` on its own network or
firewall boundary, but the option must fail closed at validation: the agent
ingest port stays disabled at `0`, must be a valid `1`-`65535` port, and must
differ from both the frontend port and any HTTP redirect port. When that
listener is active the runtime must serve only the `/api/agents/*` surface on
it and must never expose the web UI or the rest of the REST API through that
port, so a port reachable from an untrusted agent network cannot widen into the
operator console. Enabling the dedicated port is additive: the main listener
keeps serving agent ingest too, so the default single-port deployment and
existing agents are unaffected.

Locale-catalog additions for shared mobile copy controls remain contract-neutral
to security and privacy only while they preserve every governed token, scope,
privacy disclosure, and API name unchanged. Responsive presentation work may
add localized accessible labels, but it must not rename or weaken security-owned
terms through a mobile-specific catalog variant.

Patrol action authority remains server-derived through the canonical action
lifecycle. Relay-mobile callers may read their pending queue and submit a
decision or execution request only through the existing scoped route checks;
they cannot supply requester identity, origin, approval policy, capability
catalog entries, or verification outcome. Legacy command-shaped investigation
history is never exposed as an executable payload in desktop or mobile review.
Core-owned Patrol policy authorization is additive to that boundary, not a new
caller grant. It requires an eligible capability, an explicit persisted
per-resource capability allowlist (and any configured recurring window), an
effective tenant Patrol mode that admits the eligibility class, and an absent
Never-auto-remediate lock. Missing or unknown state denies automatic execution.
The policy actor/method are server-stamped and cannot be supplied by the model,
enterprise orchestrator, browser, relay, or action-proposal payload.
