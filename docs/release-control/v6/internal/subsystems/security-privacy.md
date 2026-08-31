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

API-token rename is an authenticated token-management mutation: blank labels
fail closed, successful changes are persisted and audited as `token_renamed`,
and neither the plaintext credential nor its stored hash is exposed or
rotated. Rename UI copy must state that connected agents remain unchanged.
Live token-file reloads must serialize their disk snapshot with in-process
rename, scope-update, rotate, create, and revoke mutations under `config.Mu`.
An older watcher snapshot may never overwrite newer runtime and durable token
state; `TestConfigWatcher_ReloadAPITokensDoesNotOverwriteConcurrentMutation`
is the required concurrency proof.

Pulse-minted Unified Agent credentials follow least authority at issuance.
Implicit and ordinary monitoring installs require `agent:report` and
`agent:config:read`; host collectors may additionally receive only
`docker:report` and `kubernetes:report`. The role policy denies wildcard,
operator, action, `agent:exec`, `agent:manage`, and unknown future scopes.
Issuance and runtime admission reject non-canonical role credentials;
persisted-token loading durably subtracts historical excess scopes and rejects
a credential missing its mandatory baseline. Execution scope is added only for
an explicit command-capable installer choice. Local runtime
authority remains an independent ceiling: a credential with `agent:exec`
cannot promote a service installed as `monitoring-only`, and Agent Doctor must
warn when such excess scope remains attached to that runtime.

Privileged host telemetry crosses only the local typed helper boundary owned by
`internal/agenthelper/` and `cmd/pulse-agent-helper/main.go`. That root process
has no network or Pulse credential, admits only the configured collector UID
through Linux peer credentials, and exposes named versioned collection
operations rather than command, path, argument, environment, or shell input.
Protocol decoding is bounded and strict, request deadlines are mandatory, and
audit records contain metadata only. SMART and Proxmox LXC filesystem payloads
remain local collector data and must never be copied into helper audit output.
Container inventory is a bounded helper-owned projection of fixed
Docker/Podman daemon endpoints, never a daemon proxy. Update staging accepts
only an artifact identity and digest from the fixed collector-owned quarantine;
the request cannot select a source, destination, target, URL, command, or
argument. The helper revalidates the quarantine owner, signature, digest, ELF
shape, regular-file identity, symlink resistance, and byte ceiling before
copying into fixed root-owned staging. Activation and rollback then revalidate
the root boundary, perform atomic replacement, and durably bind the transition
identity before changing the root-owned collector binary.
The helper must identify the candidate statically as the Pulse agent Go
command, require both its build metadata and bounded `--version` output to
equal the requested target, and reject a target that does not advance the
installed version. Commit is accepted only from the activating process ID when
the digest of its live `/proc/<pid>/exe` equals the pending artifact digest.
Activation is not final authority: the helper records a bounded pending state
and last-known-good digest before replacement, and exposes an exact typed
commit. Only the replacement collector's local readiness plus an accepted new
authoritative report may request commit. Restart/power-loss recovery and the
deadline watchdog roll back an uncommitted activation, while strict private
handoff parsing and fixed artifact cleanup prevent a collector-selected target
or stale quarantine from widening that boundary. Configuring the helper also
requires a successful versioned health exchange at collector startup and
forbids privileged telemetry fallback into the collector process.
Helper health exposed beyond the local process is a classified status boundary,
not a copy of privileged error output. SMART, Proxmox LXC filesystem, and
container inventory failures share one operation-aware module status, but only
stable transport/provider categories may enter host or Docker reports. Raw
helper messages, request payloads, bearer-shaped values, local paths, and
provider output must be discarded before serialization. A Docker-only status
report explicitly marks inventory incomplete, retains the last complete server
snapshot, and cannot widen the collector into direct rootful socket access or
create a remediation grant.

Remediation credentials belong only to the separately installed
`pulse-agent-runner`. They bind organization, canonical host identity, token
record, runtime role, and `typed_actions.v1`; monitoring credentials cannot be
upgraded in place or accepted on the action session. The runner's closed
protocol permits only the enumerated typed host, Proxmox guest, and container
operations with strict payloads, target binding, deadlines, request digests,
replay protection, cancellation, and durable terminal receipts. Generic
shell/exec, unrestricted `read_file`, deploy, and trusted-origin bypasses are
forbidden. The legacy combined command channel remains a disclosed full-trust
migration boundary only until runner enrollment and live session parity are
qualified; it is not safe-profile authority.
The action runner necessarily has a writable host view for its closed mutation
set, so `ProtectSystem=strict` is not a valid runner sandbox claim. Its separate
credential, no-listener transport, typed admission, target and digest binding,
bounded capabilities, state privacy, and durable receipts are the compensating
boundary; collector and helper filesystem hardening remains independent.
Action-runner issuance and rotation are a two-phase durable host-bound
transition per organization and canonical agent ID. Re-issuance prepares a
ten-minute replacement credential while retaining the prior active record,
returns the new plaintext secret only after persistence succeeds, and removes
older unactivated replacements. Issuance, root-owned runtime configuration,
session admission, typed payloads, and receipts share one bounded
action-identity vocabulary, so an unsafe or unrepresentable host identity is
rejected before any action credential is minted. The pending credential may authenticate one
exact runner transport in a separate bounded pending slot but cannot become
dispatch authority. Pending reconnects replace only that slot; they cannot
close or displace the active predecessor, which remains the dispatch target
until commit. After the runner
durably records its current activation nonce as pending, its authenticated
activation request durably activates the replacement and revokes the
server-recorded predecessor set, then atomically promotes the exact pending
transport under the same token-inventory transaction; displaced transport
cleanup occurs outside both locks and the local activated marker follows that
commit. If the pending transport vanished or was superseded, a compensating
durable save restores both sides of the transition and returns conflict. If the
compensating save itself fails, memory remains aligned with the last known
durable activated inventory and the response is indeterminate rather than a
false success. An initial persistence failure also restores both sides;
an unactivated replacement expires without revoking the predecessor.
Installer recovery stops the replacement and may restore a predecessor only
after a bodyless self-cancellation durably removes the exact pending credential
under the activation transaction lock. Activation conflict or any transport,
TLS, persistence, or admission-tombstone uncertainty retains the new
credential/runtime and surfaces repair-required rather than reinstalling a
secret that the server may already have revoked. Retention is claimed only
after the exact requested replacement bearer is atomically and durably
reinstalled with root-only ownership; failure keeps the runner stopped and
requires re-enrollment without restoring the predecessor.
That invalidation is exact and post-persistence: it matches organization,
token, canonical agent, hostname, runtime role, and typed capability before
closing the session, so a stale rotation cannot evict a replacement. Activation
is idempotent for an already committed exact credential so transport-level
response loss is recoverable without requiring a stale pending-session
snapshot. The
runner may also delete only its own matching record using its bearer
credential; browser sessions and a caller-selected token ID are rejected, and
persistence failure restores the prior inventory. Installer teardown keeps the
secret in a private file/config boundary rather than argv, stops and disables
the runner first, and removes no artifact until self-revocation is durably
confirmed. Runner WS and lifecycle HTTP use HTTPS/WSS with normal/custom CA
trust or exact DER pinning; plaintext accepts only exact `localhost`, `127/8`,
or `::1` literals, never resolver-controlled `*.localhost`, and curl `-k` never
carries the runner bearer. These bearer-bearing lifecycle clients also disable
ambient proxy discovery so an unconfigured `HTTP_PROXY`/`HTTPS_PROXY` cannot
receive the credential.
Safe-profile migration uses the authenticated
`POST /api/agents/collector/reduce-authority` transition. The server accepts
only the caller's exact organization, agent, and canonical hostname; rejects
wildcard, cross-host, and cross-organization credentials; durably removes both
`agent:exec` and `agent:manage`; restores a deep token snapshot on persistence
failure; and then closes only the superseded live collector session. The
installer must not undo that server-side reduction during local rollback and
must strip collector command flags from any restored service definition.

Own Pulse's canonical privacy disclosures, outbound usage-data boundary,
and the security-facing settings surfaces that expose authentication posture,
token-management visibility, and privacy controls to operators. Customer-facing
privacy and Settings surfaces must not present maintainer commercial-event
controls as normal product settings.
Configuration-level validation also keeps Proxmox cluster-node display
overrides presentation-only: valid Unicode is bounded to 128 code points,
control characters are rejected, and tenant deep copies must isolate the
retained identity ledger and aliases. Those labels never become authentication
principals, API-token ownership, credential lookup, TLS identity, routing
authority, audit actor identity, or organization scope.

External-probe mobile notifications are a metadata-minimization boundary.
Only the generated event type, generic operator copy, canonical alert ID,
severity/category, and `view_alert` action may leave the instance through
Relay. Agent names, hostnames, target names, URLs, IP addresses, failure text,
and target lists must remain local. Detection remains behind the existing
Pulse Pro availability entitlement; the push adapter cannot turn Relay access
into availability-probe entitlement.

`cmd/pulse-agent/main.go` treats disk inclusion as explicit local operator
configuration. It may override Pulse's automatic filesystem suppression only;
it must not override a matching operator exclusion or introduce remote
authority to expand the reported filesystem set silently.

Retained Patrol objective briefs and optional context are operator-authored AI
content. They are encrypted at rest in the organization-scoped Pulse data
directory and loading fails closed if decryption fails. Audit and telemetry
records must not contain the brief, optional context, observer artifact,
resource identifiers, or health evidence. Audit records may include the
objective ID; usage telemetry may include only content-free aggregate lifecycle
counts. When Patrol is enabled, applicable active objective text becomes part
of the context sent to the configured AI provider under the same local-provider
and non-local provider-bound resource-policy redaction rules as other Patrol
context. Merely saving an objective makes no outbound model request.
When a configured provider returns an observer proposal through the first-party
Patrol builder, the bounded probe and requirements artifact is persisted only
inside the same encrypted objective document. Public objective reads and later
Patrol prompt seeds omit the artifact, and telemetry/audit surfaces remain
content-free. The proposal is not executable and carries no infrastructure
mutation authority; any future validator or installer must preserve this
confidentiality boundary while enforcing declared secret references rather
than accepting secret values.
The canonical and shipped privacy documents must state this retained-data and
telemetry boundary explicitly and remain byte-for-byte synchronized, so the
promise visible inside the product cannot drift from the repository policy.
Their telemetry table must identify the current schema version and name every
outbound JSON field individually. Grouped prose may explain related aggregates,
but it cannot replace field-level disclosure or make a newly added counter
invisible to operators reviewing exactly what Pulse sends.

## Canonical Files

1. `SECURITY.md`
2. `docs/PRIVACY.md`
3. `frontend-modern/public/docs/PRIVACY.md`
4. `frontend-modern/src/utils/docsLinks.ts`
5. `frontend-modern/src/api/security.ts`
6. `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
7. `frontend-modern/src/components/Settings/APITokenManager.tsx`
7. `frontend-modern/src/components/Settings/APITokenManagerDialogs.tsx`
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
29. `internal/telemetry/service_health.go`
30. `internal/config/workload_history_activity_tally.go`
31. `internal/api/telemetry_workload_history.go`
32. `pkg/server/service_health.go`
33. `pkg/server/telemetry_pulse_intelligence.go`
34. `internal/api/router_routes_auth_security.go`
35. `internal/crypto/crypto.go`
36. `internal/securityutil/secure_storage_dir.go`
37. `internal/cloudcp/auth/magiclink.go`
38. `internal/cloudcp/auth/magiclink_store.go`
39. `pkg/tlsutil/fingerprint.go`
40. `pkg/audit/audit.go`
41. `pkg/audit/async_logger.go`
42. `pkg/audit/sqlite_logger.go`
43. `pkg/audit/signer.go`
44. `pkg/audit/sqlite_factory.go`
45. `pkg/extensions/audit_admin.go`
46. `scripts/telemetry_adoption_report.py`
47. `frontend-modern/src/components/Settings/DataHandlingPanel.tsx`
48. `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts`
49. `internal/api/agent_exec_token_binding.go`
50. `internal/logging/logging.go`
51. `pkg/auth/agent_credentials.go`
52. `pkg/auth/rbac.go`
53. `pkg/auth/rbac_manager.go`
54. `pkg/auth/sqlite_manager.go`
55. `pkg/server/server.go`

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
   Phone layouts may tighten this intro into the shared compact settings frame,
   but scope-reference access, token inventory semantics, and credential-safety
   guidance must remain visible and unchanged.
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
   Existing token rows may open an explicit checkbox checklist for in-place
   scope editing. That editor must initialize from the token's effective live
   scopes, preserve wildcard exclusivity, reject empty or unchanged
   submissions, state that the token value does not rotate, and keep the
   dialog open when the backend rejects the transition.
   Stable in-page anchors for sibling API Access onboarding panels are allowed
   only as navigation into the token creation section; those sibling panels do
   not own token scope derivation or preset contents.
3. `frontend-modern/src/components/Settings/APITokenManagerDialogs.tsx` shared with `api-contracts`: the deferred API token edit and revoke dialogs preserve the shared security/privacy and API contract boundary.
   The dialogs may load on demand after the operator selects their row action.
   This bundle boundary changes neither authorization nor mutation semantics
   and must preserve the explicit revoke confirmation, scope validation, and
   failed-edit state.
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
   At phone widths, the expanded privacy explanation may collapse to a
   localized details link to the same canonical privacy document. The
   telemetry state, environment override, and outbound-data control meaning
   must remain directly visible; density must never imply weaker disclosure or
   a different telemetry default.
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
13. `internal/api/agentbinding/policy.go` shared with `agent-lifecycle`, `api-contracts`: install-token command-channel binding is simultaneously an agent lifecycle admission policy, a canonical API identity contract, and a security boundary.
14. `internal/api/agenttokens/install.go` shared with `agent-lifecycle`, `api-contracts`: agent install-token issuance and persistence are simultaneously an agent lifecycle authority, a canonical API token contract, and a security boundary.
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
18. `pkg/extensions/audit_admin.go` shared with `api-contracts`: the enterprise audit endpoint and canonical store configuration seam is both a security persistence trust boundary and a canonical API extension contract.

## Extension Points

The agent-lifecycle-owned `internal/collectorlifecycle/` package and
`cmd/pulse-agent/collector_lifecycle.go` are the security boundary for
bearer-authenticated safe-profile migration calls. They must preserve direct
transport, redirect rejection, normal/custom CA or exact DER-leaf pin trust,
literal-loopback-only plaintext, and single-open bounded credential-file
validation with an explicit root-or-collector-owner matrix. Installer changes
must not replace this boundary with curl `-k`, plaintext to a non-loopback
host, proxy-aware transport, or a bearer supplied in argv.

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

Patrol attention list, summary, and detail routes require `monitoring:read`.
They expose bounded canonical evidence and resource references only; no route
grants action authority, returns credentials, or turns Assistant explanation
into approval. Canonical record IDs are compared as opaque identities even
when their percent-encoded route representation contains `/`.

Scheduled report management under `/api/admin/reports/schedules` is a
settings/reporting control surface, not a new public data export. It must reuse
the existing reporting feature gate and settings read/write scopes, persist
workspace-local schedule metadata only, and never add cross-tenant report
creation, unauthenticated delivery, raw SMTP secret exposure, or bypasses for
the `white_label` branding entitlement.

1. Change privacy disclosures, usage-data vocabulary, or outbound-data guarantees through `docs/PRIVACY.md`, `frontend-modern/public/docs/PRIVACY.md`, `internal/telemetry/telemetry.go`, and `pkg/server/telemetry_pulse_intelligence.go` together.
   Workload-history adoption telemetry may cross the browser boundary only as
   one of four closed milestones: populated preview, cursor scrub, range
   change, or Details-mode selection. The browser must deduplicate each value
   once per session before calling the authenticated local intake. The intake
   rejects unknown fields and values, and persistence retains only bounded
   UTC-day counts. Guest/user identity, route, selected range, cursor/value,
   timing, browser identity, and raw event streams are forbidden at every
   layer; only rolling 30-day aggregate counts enter the existing opt-out
   startup/daily heartbeat.
   Mock/demo fixture mode is a hard suppression boundary for outbound usage
   telemetry: while `internal/mock.IsMockEnabled()` reports true,
   `internal/telemetry` must not send startup or heartbeat pings — the check
   runs per event so runtime mock toggles take effect immediately — because a
   mock-mode snapshot describes the synthetic fixture fleet rather than a real
   installation.
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
   A non-admin General panel may read the effective `telemetryEnabled` boolean
   from the authenticated `/api/runtime/display` whitelist after the complete
   admin settings request is refused. That fallback must expose no preview
   payload, install ID, environment override, origin, webhook-network, or login
   configuration, and it must preserve an operator-selected `false` rather than
   reverting to the frontend's enabled default. The same response may carry
   the effective Docker-action display boolean because both values are needed
   to render read-only global state; this does not widen settings write access.
   The projection may likewise carry the effective PVE polling cadence in
   seconds so the read-only Monitoring Cadence card stops misreporting the
   Realtime default (issue #1601); that single integer is presentation state,
   not a channel for any other admin-only polling, discovery, or override
   configuration, and consuming it grants no settings write access.
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
   preset ids, privacy disclosures, or security control terminology. The
   Assistant settings header naming external agent (MCP) connectors across
   locales is such product-settings copy: it describes what the page hosts and
   does not alter the scoped-token model, connector token handling, or any
   security/privacy disclosure. `MCP` stays untranslated as a protocol
   identifier. Self-hosted
   Plans & Billing header and navigation localization may share that same catalog
   boundary when it frames Pro setup as choosing Patrol autonomy; it must not
   alter API Access, authentication, privacy, or token-management terminology in
   the same edit.
   Commercial pricing handoff localization may share the same catalog boundary
   only for redirect/manual-link copy and must preserve `Pulse Account`,
   security/privacy disclosures, token names, API field names, route/query keys,
   and purchase-return state exactly.
   The first-session setup completion Pro activation pointer
   (`setup.completion.proActivation.*`) is likewise product-surface copy in the
   shared catalogs: it must stay a factual statement of build edition and
   license state plus the activation action, must not add upgrade-marketing
   language, and must not alter API token, authentication, privacy-disclosure,
   or non-translatable security terms. `Pulse Pro` stays untranslated as a
   product identifier across locales.
   Alerts-owned localization may distinguish detector state, in-product alert
   visibility, and external notification delivery, but it must preserve all
   security/privacy terminology and destination confidentiality. Pausing
   delivery changes no authorization, tenant boundary, destination secret, or
   active-alert evidence access rule.
   Localized active-alert hydration states may distinguish unconfirmed,
   unavailable, and confirmed-empty alert truth, but they remain presentation
   only. Retry must reuse the authenticated active-alert read boundary and must
   not disclose destination configuration, recipient identity, alert evidence,
   or tenant-private failure detail in customer-facing copy.
   Per-alert snooze and resume remain authenticated `monitoring:write`
   operations. Their canonical lifecycle projection may retain the authenticated
   actor and exact expiry needed for auditability, but must not copy destination
   credentials, recipient details, alert evidence, or other tenant-private
   notification configuration into the resource or incident timeline.
6. Change operator-facing telemetry/adoption reporting through `scripts/telemetry_adoption_report.py` together with the privacy disclosure whenever release-identity interpretation changes.
   Release service-health telemetry must come from a bounded loopback probe of
   the listener Pulse actually bound. It may report only whether the API, UI,
   and referenced frontend assets were served, a fixed failure category, and
   the immediately previous normalized release observation. It must not report
   listener addresses, URLs, asset names, response bodies, errors, hostnames,
   customer identity, or account identity. Adoption reporting must interpret
   those direct current/previous observations as release cohorts rather than
   attributing rolling historical counters to the current release.
   The adoption report excludes mock-fixture-fleet-signature rows (120×N
   Kubernetes pods with 7×N VMware hosts, the `internal/mock` template) from
   adoption reads by default and must disclose the excluded row/install
   counts, because versions that predate client-side mock suppression keep
   pinging the synthetic fleet until upgraded; `--include-mock-fleet` remains
   the audit path.
   The existing `known_install_age_bucket` wire field must be presented as
   time since the first schema-v2 lifecycle observation. For an upgraded
   install it is only a lower bound, never original installation age.
   Deployment-method buckets are best-effort current-runtime evidence.
   `container_other` and `binary_other` are unknown fallbacks for many upgraded
   installs and must not be presented as precise original installation
   provenance.
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
9. Change operator-facing Resource Privacy/Data Handling posture through `frontend-modern/src/components/Settings/DataHandlingPanel.tsx` and `frontend-modern/src/components/Settings/dataHandlingPanelModel.ts` together so resource classification, handling-boundary, redaction copy, and the route-backed/hidden-sidebar presentation stay governed as a trust surface. The panel must consume the compact backend-owned policy posture through `frontend-modern/src/hooks/useResourceStats.ts` and `ResourceAPI.getStats()`; it must not hydrate and adapt every unified-resource row merely to display aggregate privacy counts.
10. Change inside-guest runtime collection boundaries through `docs/AGENT_SECURITY.md`, `docs/UNIFIED_AGENT.md`, `cmd/pulse-agent/main.go`, `internal/api/router.go`, and `internal/config/config.go` together. Docker / Podman inventory inside a VM or LXC may come from a guest-local `pulse-agent` module or explicitly reported guest data; LXC Docker inventory may also be collected by a Proxmox host agent only through explicit server opt-in, with optional VMID allowlisting and a minimal summary command set that avoids `docker inspect`, environment, mount, file, command, and process collection. Local Unified Agent Docker / Podman disables must not be reversed by remote profile configuration, and self-test/update preflight that needs the live runtime token must pass it through a short-lived token file rather than argv. The `--enable-docker` help line is part of that operator privacy control, so it must remain "Enable Docker / Podman Agent module" instead of exposing internal collection-module wording. The `--enable-commands` help line and installer disclosure must identify Pulse command execution as disabled by default and required for Patrol actions or the explicit Proxmox LXC Docker inventory path, not as implicit guest access.
    Helper-backed update recovery is part of the same local executable trust
    boundary. On startup, a pending update handoff must be compared with the
    SHA-256 identity of `/proc/self/exe`: the candidate identity may proceed
    toward accepted-report commit, the rollback identity may only acknowledge
    the helper's idempotent rolled-back state and clear the stale handoff, and
    an unrelated executable identity must leave the handoff intact and fail
    closed without mutation.
    Node-local LXC filesystem capacity is a distinct automatic host telemetry
    boundary, not inside-guest command authority. A PVE Unified Agent may use
    bounded `pct list` plus `pct df` only for guests reported running and may
    report only VMID, exact guest name, mount key, volume label, mount path,
    and capacity/usage values. That path must not use `pct exec`, read guest
    file contents, or collect guest processes, environment, commands, or
    container-runtime inventory.
    Agent file logging is local operational state, not a second telemetry path:
    `cmd/pulse-agent/main.go` must use the canonical owner-only rotating sink,
    retain that sink when remote configuration changes log level, and never
    place runtime tokens or enrollment secrets in the service command or log
    output.
    Custom agent state directories share that same credential boundary: the
    directory is owner-only, token, runtime-token, identity, and connection
    files are mode `0600`, and implicit token lookup is confined to the
    resolved directory instead of falling through to another instance's
    default token. Installer health, lookup, and uninstall HTTP calls must feed
    token headers through private curl configuration rather than exposing the
    token in curl process arguments; generated service definitions and
    `connection.env` may contain only protected token-file paths.
    Disk exclusion is an operator-defined collection boundary:
    `--disk-exclude` accepts device names, device paths, and mount-point
    patterns, combines repeated flags with comma-separated environment values,
    and applies before filesystem-usage, disk-I/O, and SMART collection.
    Excluded inventory must not re-enter through linked Proxmox disk health or
    wear alerts, and the CLI help text must state the supported exclusion
    forms.
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
    The Assistant steer sub-route (`POST /api/ai/sessions/{id}/steer`) added
    to the session dispatch is bound by the same rule: it requires
    `ScopeAIChat`, carries conversation text only, cannot approve or bypass
    a pending approval, cannot change the running turn's control level,
    autonomous mode, or model route, rejects Pulse-owned system sessions,
    and its response discloses only `accepted` plus a coarse reason, never
    run internals, provider state, or transcript content.
    The Patrol action-broker and proposal-catalog factory glue wired here is bound by the
    same rule: it may connect the investigation orchestrator to the tenant-bound
    action lifecycle, but it exposes only typed-proposal capture and gives the
    orchestrator no autonomy control, command execution, or command-shaped
    approval path.
    Operator-state mutation callbacks wired here may trigger alert
    reconciliation across active incidents so descendant-scoped maintenance
    takes effect immediately, but they receive only tenant-bound canonical
    resource identity and the already authorized mutation result. They must not
    expose inventory, operator notes, maintenance reasons, credentials, or
    cross-organization state through router callbacks or synchronization events.
    Automatic action authority is a versioned, one-use admission lease rather
    than a reusable approval. Tenant mode/license/unlock, capability safety and
    approval floor, resource allowlist/window/Never state, plan hashes, and the
    action emergency stop are revalidated under one admission coordinator and
    committed with `executing`. Missing or unreadable policy denies dispatch.
    Human approval remains separate, but `NeverAutoRemediate`, plan drift, and
    emergency stop are universal. Emergency stop after `executing` is only
    best-effort cancellation and must never be described as rollback.
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
    Proxmox VM/LXC lifecycle observer wiring is also not mutation authority.
    It may resolve only the current tenant's existing monitor and use that
    monitor's configured Proxmox client for bounded status/uptime reads after a
    governed node-agent action. It must not expose provider credentials or raw
    provider responses, cross tenant or guest identity, call a provider
    mutation API, or classify the executing agent's own trust domain as
    independent evidence.
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
12. Change host registry-credential use for Docker / Podman update detection
    through `cmd/pulse-agent/main.go`,
    `internal/dockeragent/registry_credentials.go`, `docs/UNIFIED_AGENT.md`,
    and `docs/DOCKER.md` together. Update detection may read the host's own
    Docker / Podman credential store (`config.json` auths entries,
    `credsStore`/`credHelpers` credential helpers, and Podman's `auth.json`)
    to authenticate digest checks against private registries, and that read
    is an explicit local operator boundary: resolved credentials are
    presented only to the registry or the token endpoint it names, must
    never be sent to the Pulse server, logged, or embedded in reported check
    errors, and credential helper output must stay out of returned error
    surfaces. Credential helper names must be validated before the agent
    executes `docker-credential-<name> get`, helper execution stays
    time-bounded with capped output, and `--disable-registry-credentials` /
    `PULSE_DISABLE_REGISTRY_CREDENTIALS` must keep detection anonymous-only
    without reading the store or executing helpers. Remote profile
    configuration has no key for this boundary and must not gain one that
    can re-enable credential reads a local operator disabled.

Node connection test telemetry reports two counters over the install-ID
rotation window: `node_test_attempts_30d` and `node_test_failures_30d`. They
count only tests that carried a target and credentials, and they carry counts
alone. Hostnames, addresses, ports, credentials, fingerprints, and error text
must never enter the tally or the outbound ping. The tally is stored as plain
JSON in the config directory precisely because it holds no secret material;
adding any field that identifies a target would invalidate that storage choice
and require the encrypted history path instead. Both fields must stay disclosed
in `docs/PRIVACY.md` and `frontend-modern/public/docs/PRIVACY.md`, which
`TestAllTelemetryFieldsAreDisclosed` enforces.

## Forbidden Paths

1. Changing telemetry payload semantics without updating the canonical privacy disclosure.
2. Letting security-facing settings copy or privacy guarantees drift between runtime behavior and the governed docs.
3. Treating API token management, auth posture, or telemetry controls as generic settings-shell polish instead of explicit trust-surface behavior.

## Completion Obligations

Split-port agent exposure is complete only when its exact allowlist covers
report/config routes, command WebSocket admission, version/server bootstrap,
and the supported installer/download endpoints while every UI and management
route remains unavailable. Registration binds one execution-scoped token to
one organization, agent ID, and hostname; the server stores only the resulting
non-secret admission, revalidates it before visibility or dispatch, and rejects
multi-organization authority, identity drift, replay after migration, revoked
tokens, and path-normalization variants.

1. Update privacy/security docs and the telemetry runtime together when outbound-data behavior changes.
2. Keep shared API-contract proof routing aligned whenever auth, token, or telemetry settings payloads change.
3. Keep shared frontend settings proof routing aligned whenever security/privacy presentation changes.
4. Keep the checked-in telemetry adoption report aligned with the same release-identity rules used by the runtime telemetry payload.
4a. Keep full-history Pulse Intelligence cohort and funnel aggregation linear
    in the number of input rows. Parse and order each install's timestamps once,
    share that analysis across cohort and funnel projections, and preserve
    exact aggregate counts, rates, exclusions, and privacy boundaries.
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
7a. Keep v5 RBAC file import transactional and fail closed, retain source files
    on validation or persistence failure, keep SSO and settings on one
    canonical manager, require the enterprise authorizer to resolve that live
    manager rather than capturing a startup-local store, and prove that moving
    a legacy identity alias cannot union conflicting grants.
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

13. A rejected legacy RBAC import must not destroy the store. The import is
    transactional and leaves the legacy files in place, so a failure leaves the
    database un-migrated rather than half-migrated, which denies access rather
    than granting it. Failing manager construction instead takes the whole org's
    RBAC surface offline including `ResetAdminRole`, the operator's only way
    back, on a conflict an ordinary v5 upgrade can produce. The store therefore
    stays live and records the failure on `SQLiteManager.MigrationError`.
    Management routes still fail closed with `rbac_store_unavailable` through
    the handler manager accessors; recovery reaches the provider directly and is
    deliberately exempt. Regression coverage:
    `TestAdminRecoverySurvivesFailedLegacyRBACMigration` in
    `internal/api/contract_test.go`,
    `TestRBACHandlersReportLegacyMigrationFailure` in
    `internal/api/rbac_handlers_test.go`, and the rejection cases in
    `pkg/auth/sqlite_manager_test.go`.

14. Security posture presentation must never be computed from a payload the
    caller was not authorized to read. `/api/security/status` truncates its
    response by authority and declares which tier it returned through
    `detailLevel`. `frontend-modern/src/utils/securityScorePresentation.ts`
    treats an `authenticated` payload as carrying no posture at all and a
    `public` payload as authoritative for `hasAuthentication` only; only a
    `privileged` payload supports the full assessment. Absent posture fields
    must not be read as disabled controls.
15. Historical credential containment remains release governance, not a prose
    security sign-off. Keep the rc-ready release gate fail-closed until every
    redacted Pulse and Pulse Pro subject has typed provider/control-plane
    closure plus replacement-deployment or verified-retirement evidence at the
    required tier. Keep optional history rewriting separate and nonblocking
    after containment.

## Current State

### Alert-quality telemetry remains aggregate and comparison-safe

Telemetry schema v14 exports only counts and closed buckets. It exports no
resource or destination identity, hostname, alert or rule content, lifecycle
timestamp, actor, reason, error text, or clickstream. Public and frontend-served
privacy disclosures enumerate the fields and explain the deliberately excluded
resolution, maintenance-suppression, and detected-flapping classifications.

The adoption report treats a version's first heartbeat as baseline-only and
uses only consecutive same-version changes for rolling counters. Optional
release comparisons require schema v14 in both cohorts, then match observations
by the latest known-install-age bucket and version heartbeat-count bucket.
Unmatched exposure is reported rather than pooled, and the specific 6.4.0
versus 6.3.2 comparison is rejected as schema-incompatible instead of presenting
legacy alert volume as alert quality.

### Docker update preflight remains read-only across the unified-agent bridge

The `pulse-agent` late-bound Docker updater delegates the typed, read-only
container-update preflight through the same required capability as execution.
The preflight checks runtime identity, container identity, and expected image
digest without mutating the workload; adding it to the bridge restores the
existing approval boundary and grants no new command, inventory, environment,
mount, file, process, or guest-inspection authority. An unavailable Docker /
Podman module continues to fail closed.

### Historical credential containment is prerelease-blocking

The `historical-credential-containment` release gate now owns the cross-repo
containment boundary for the stable redacted Pulse identities and the redacted
Pulse Pro credential roles. Its `owner-action` state blocks `rc_ready`, and the
status audit refuses to make a raw `passed` state effective until both typed
record sets cover every declared subject with production-observed evidence.
The opening sanitized inventory is recorded in
`docs/release-control/v6/internal/records/historical-credential-containment-open-2026-08-08.md`.
History rewrite remains an optional post-containment operation and cannot
satisfy the gate.

### Custom metric source authority is local-only and fail-closed

`pulse-agent` may load site-defined probes only from the absolute path
named by `--custom-sensors-file` or `PULSE_CUSTOM_SENSORS_FILE`. Server
profiles, remote configuration, AI tooling, and command transport cannot author
the executable, REST URL, or arguments. The strict versioned YAML schema accepts
exactly one absolute executable or HTTP(S) URL per metric. Executables use no
shell or arguments; POSIX ownership, permissions, and symlink checks protect the
config, executable, and immediate parent directory, and the executable is
revalidated before each run. REST URLs reject embedded credentials and
fragments, retain standard TLS validation, do not follow redirects, and expose
neither the configured URL nor transport errors that could contain query
credentials in reports. Count, concurrency, timeout, output, response, label,
and error sizes are bounded. Probe stderr is discarded so credentials or query
detail cannot enter reports or alerts; tests in
`internal/hostagent/custom_sensors_test.go` pin unsafe-config rejection and the
stderr, REST-shape, freshness, and response-size boundaries.

### System settings mutation surface no longer accepts dead schedule inputs

The governed system-settings write path in `internal/api/system_settings.go`
and the runtime config in `internal/config/config.go` dropped the
`autoUpdateCheckInterval` / `autoUpdateTime` fields, which were stored but
never consumed by any runtime. Dead accepted-but-unread input is surface the
security review has to reason about for no benefit, so the fields, their
validation, and their persistence were removed together. Legacy keys in a
POSTed settings body are ignored rather than rejected, and a `system.json`
written before the removal still loads cleanly with the legacy keys ignored
(`TestLoad_IgnoresLegacyAutoUpdateScheduleFields` in
`internal/config/config_load_test.go`,
`TestSystemSettingsUpdate_LegacyAutoUpdateFieldsIgnored` in
`internal/api/system_settings_telemetry_test.go`).

### Settings save resets only SSH retry timing, never SSH trust state

The governed system-settings write path in `internal/api/system_settings.go`
now clears the temperature SSH failure backoff on every live tenant monitor
after a successful save (#1638). The reset touches retry *timing* state only —
in-memory per-host backoff windows and the knownhosts keyscan backoff — and
never SSH trust state: it does not remove pinned host keys, does not relax
known-hosts verification, and discloses nothing in the response. It runs
behind the same authenticated write access as the rest of the settings
mutation surface and adds no new accepted input — no request field enables,
disables, or targets it, so there is no new attacker-controllable surface
beyond the already-governed ability to save settings. The worst an authorized
save can do is advance an SSH retry that the poll cycle would have run anyway
once the window expired.
`TestIssue1638SettingsSaveResetsSSHFailureBackoff` in
`internal/api/system_settings_telemetry_test.go` pins the trigger.

### Canonical mutation-plane dependency

Raw command, file-write, arbitrary pod-exec, and legacy remediation authority
are retired before handler or transport execution. Extension aliases cannot
shadow retired names, and transport delivery must name committed lifecycle
authority rather than accepting model-supplied command or rollback text.
Container image updates re-entered the plane as a closed typed operation:
the `docker_container_update` payload carries only an immutable container id,
runtime, and the plan-observed image digest (never a pull reference or
command text), the unified agent binds it through the same durable
operation-receipt admission as the other typed operations, and the process
bridge in `cmd/pulse-agent/main.go` late-binds the Docker module so a missing
module refuses before any mutation.

Unified Agent Pulse transports share one fail-closed TLS policy across host,
Docker/Podman, Kubernetes, remote configuration, commands, and self-update.
Custom CA bundles and SHA-256 leaf-certificate pins are runtime trust inputs,
not monitoring data, and malformed pins must fail during client construction
instead of silently degrading to system roots or blanket skip-verification.
Plaintext transport to a non-local-looking control plane is never implicit:
it requires the operator's explicit `--allow-plaintext-http` consent recorded
once at agent startup, is logged as a warning naming the cleartext token
exposure, and cannot be enabled by the server, by remote configuration, or by
generated install commands.
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

Webhook SSRF classification is an address-reachability decision, not a
byte-pattern decision. The `net.IP` predicates (`IsLoopback`, `IsPrivate`,
`IsLinkLocalUnicast`, `IsMulticast`, `IsUnspecified`) read only the literal
address handed to them, so an IPv6 transition address carries an internal IPv4
destination past every one of them: `64:ff9b::a9fe:a9fe` reaches
169.254.169.254 while reporting itself as ordinary global unicast. Both SSRF
layers -- the webhook URL validator (`pkg/audit/webhook.go`) and the restricted
outbound transport (`pkg/securityutil/outbound_http.go`) -- must therefore
unwrap NAT64 (RFC 6052 well-known and RFC 8215 local-use prefixes), 6to4,
Teredo, ISATAP, IPv4-compatible, and IPv4-translated encodings through the
single shared `securityutil.EmbeddedIPv4Candidates` helper and apply the same
policy to every embedded destination they find. Unwrapping is a shared helper
rather than a per-layer predicate because a bypass that defeats one layer
defeats the other identically, so the two layers must never drift on which
encodings they recognize. The embedded destination inherits the caller's
policy, not a stricter one: a NAT64 address wrapping a permitted public target
stays permitted, and `AllowPrivateIPs`/`AllowLoopback` relax the embedded check
exactly as they relax the outer one.

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
Target-release activity reads must not attribute a rolling counter already
present on an install's first heartbeat after upgrade to the target release.
The first target-version heartbeat in the selected source window is a
non-attributable baseline; activity
requires consecutive heartbeats from the same pseudonymous install on the same
version and is reported as the observed counter increase across those pairs. A
version departure breaks the comparison chain, so returning to the target
version starts with another non-attributable observation. Counter decreases
remain separate because a rolling window or local reset can reduce a value. If
the latest later heartbeat reports a different version, the report must
classify that departure as a semantic-version rollback, a forward transition,
or unclassified development/version drift and must expose aggregate
destination-version counts without install identifiers. Activity from an
install whose latest later heartbeat departed the target must remain separate
from activity on installs still reporting the target version, so a rolled-back
install cannot supply positive evidence for the currently running cohort.
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
The runtime now has one canonical persistent audit owner across Community and
Pro. Enterprise startup may provide directory, signing-key, and explicit
retention settings through `ResolveAuditStoreConfig`, but it must not open a
second database, create a competing signing key, replace the canonical list or
verify handlers, or make storage behavior depend on install history. Store
startup must transactionally normalize legacy `DATETIME`, textual, and mixed
timestamp rows into the schema-v2 integer contract before serving reads. A
malformed legacy row must roll the migration back and surface a sanitized
`audit_store_unavailable` diagnostic rather than deleting, skipping, or
silently returning an empty history.
Canonical reads use stable `(timestamp DESC, id DESC)` ordering, and a paged
read must derive rows plus total from one SQLite snapshot. Async wrapping must
retain persistent-reader and signature-verification capabilities. Retention
`0` is an explicit keep-forever setting, persisted retention is restored at
startup, the configured cleanup cadence is preserved across the enterprise
configuration seam, and cleanup must tolerate concurrent writers without
exposing partial results. Existing core and Pro signing keys remain valid
through upgrade. Every new global and tenant SQLite row must carry a
self-identifying `v2:` HMAC-SHA256 signature over the domain-separated,
length-prefixed persisted tuple (ID, Unix-second timestamp, event type, user,
IP, path, success, and details). Arbitrary string bytes, including pipes,
empty values, Unicode, and newlines, must retain injective field boundaries.
Verification dispatches by the signature envelope: unknown or malformed
versions fail closed, and a v2 signature is never retried against a historical
representation. Unprefixed 64-hex signatures remain explicitly identifiable
as legacy and may verify against the three previously accepted encodings for
read/export compatibility, but that result proves only the historical MAC and
must not be represented as providing v2 boundary integrity. Startup and reads
must not rewrite or re-sign those historical rows.
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
Container-runtime migration tokens must also follow the requested module
boundary. The default host-plus-Docker migration may grant only the bounded
Agent report, configuration-read, Agent-manage, and Docker-report scopes; an
explicit `enableHost:false` workload-only migration must retain only the
Docker-report scope. Neither path may gain command-execution or Kubernetes
report authority implicitly.
That same command-token trust boundary also owns first-use binding for
Proxmox install-command tokens. `internal/api/agent_exec_token_binding.go` may
persist `bound_agent_id`, `bound_hostname`, and `bound_at` only for
Pulse-minted PVE/PBS install-command tokens when the command agent first
registers. Generic unbound `agent:exec` tokens, or tokens already bound to a
different agent, must fail closed so command execution cannot cross hosts
through reusable bearer credentials. Within that boundary the immutable
machine-derived agent ID is the binding identity: a version-2 binding whose
agent ID does not match fails closed even when the hostname matches, while a
token whose agent ID matches exactly may re-bind a drifted `bound_hostname`
in place, because a hostname is an operator-renamable label, not a second
credential. Hostname comparison follows the system-wide short-name vs
fully-qualified equivalence rule rather than ad-hoc case folding. The
accept/reject decision is single-sourced (`evaluateAgentExecBinding`) and the
agent config gate must consume the same decision, so no surface can advertise
command execution that channel admission would refuse.
That binding boundary also covers install tokens that auto-registered a Proxmox
source before their agent ever reached the command channel. The registration
bootstrap writes `bound_hostname` with no `bound_agent_id` and no binding
version (#1644), which is the record shape the legacy pre-v6.1.1 migration
branch exists to repair. Such a token is a clean first use instead: an
equivalent hostname binds the fresh machine-derived agent ID and the current
binding version through the first-use path, an unrelated hostname still fails
closed, and the registration-bound hostname is never overwritten by an
equivalent spelling the agent reports, because the install grant compares
against it.
Pulse-minted install tokens now carry a server-authored command-policy intent
in addition to first-use binding metadata. That intent grants no independent
authority: first-report convergence requires the same shared binding decision
as command-channel admission, and enabled intent is projected only while the
live token still has `agent:exec`. A generic API token cannot opt itself into
this path by holding `agent:exec`, and the applied-agent marker makes the
installer choice one-shot so it cannot override a later administrator disable.
This is the security side of #1728's stale-policy repair: reinstall intent may
replace policy retained from an earlier installation exactly once, without
turning reusable bearer credentials into command credentials.
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
Multi-location availability evidence stays inside the same privacy floor.
Saved configuration and authenticated operator surfaces may use source-owned
location IDs and agent display names to distinguish selected paths, but
categorical history, evidence envelopes, telemetry, incidents, logs, and
Patrol context must not retain or export customer network names, addresses,
agent identity, or location labels. Durable categorical history remains keyed
only by the logical target and its aggregate conclusion; location identity is
not a new telemetry, history, or customer-identity dimension.
Infrastructure incident synthesis remains inside that same boundary. Its alert
correlation envelope may retain only canonical alert/resource IDs, typed
failure class and inference, severity, observation time, and existing evidence
IDs. It must not copy probe addresses, observation-location names, response or
request bodies, headers, credentials, raw provider metadata, or unbounded error
text into alerts, event logs, frontend catalogs, telemetry, or Patrol context.
Localized synthesis labels are product copy only and must not change API token
names, privacy disclosures, authentication terminology, or non-translatable
security identifiers.
That same outbound usage telemetry floor now also permits content-free update
funnel counters derived from local update history inside the same rotating
30-day telemetry window: update attempts, successful updates, failed or
rolled-back updates, and the latest coarse failure category. The category may
identify only the governed class (`download`, `signature`, `checksum`,
`disk_space`, `extract`, `backup`, `apply`, `restart`, `rolled_back`,
`cancelled`, or `unknown`). It must not export raw updater error text,
download URLs, command output, log lines, paths, hostnames, release asset URLs,
checksums, signatures, or operator-entered values.
Schema v13 adds a direct local service-health observation so a process-level
telemetry heartbeat is not mistaken for proof that the installed UI and API
are being served. The runtime probes its bound listener through loopback and
reports only observed/healthy booleans, one fixed failure class (`listener`,
`startup`, `runtime`, `api_connectivity`, `api_status`, `ui_status`,
`frontend_assets`, or `unknown`), a fixed observation cohort, and the
immediately previous normalized release's observed/healthy booleans. No probe
target, listener address, URL, IP address, asset path, response content, raw
error, account, customer, or infrastructure identity may enter the payload or
persisted receiver row. The previous-release fields are direct adjacent-release
observations, not 30-day update counters, and are the only valid basis for a
before/after release-health cohort in the adoption report.
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
approved-action-attempt counts, approved-action-success counts, and the
source-specific Patrol-origin subset of plan, approval request, rejected or
approved decision, approved attempt, and approved success counts. Those fields may measure whether Patrol,
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
Customer-facing legal pages, shipped privacy documentation, in-product copy,
runtime comments, operational notes, and governed contracts must therefore
describe this telemetry as pseudonymous. The stronger anonymity claim is
prohibited; disclosures must instead state the concrete direct identifiers and
content that the allowlisted payload excludes.
The identifier must be resolved at each outbound event, not frozen when the
process starts, so a continuously running installation still rotates after the
30-day window. Each payload also carries a schema version and one UTC build
time so receiver and reporting semantics can be selected explicitly.
Schema v2 may add only privacy-bounded user-base signals: a closed deployment
method, known-age/activation/time-to-first-resource/estate-size buckets,
authentication and current-monitoring booleans, configured-connection count,
and aggregate alert and notification outcome counts from the runtime's existing
30-day and seven-day local retention windows. The lifecycle record kept on the
instance may contain only first observation time, first monitored-resource
milestone time, and highest coarse activation stage. It must not retain or send
names, email addresses, account IDs, locale, URLs, paths, host/resource IDs,
recipients, endpoints, alert content, notification content, prompts, commands,
or an event-level journey/clickstream. Known install age for an upgraded
installation is therefore a lower bound beginning with its first schema-v2
observation, not a reconstructed installation date.
The adoption report must name that bucket as time since the first schema-v2
lifecycle observation without renaming the wire field. Its deployment-method
projection must preserve the closed wire buckets while clearly marking the
signal as best effort. Unknown `container_other` and `binary_other` fallbacks
must not be interpreted as precise original installation provenance.
Schema v3 preserves that field/type allowlist but corrects
`notification_failures_7d` from all unsuccessful attempts to terminal
failed/dead-letter outcomes; `notification_attempts_7d` still includes retry
attempts. Adoption reporting must keep legacy schema-v2 failed-attempt values
separate from schema-v3 terminal-failure values. This semantic correction
must not add destination, recipient, endpoint, error, notification, alert, or
identity content to either the outbound payload or central aggregate.
If monitoring is already populated at that first observation, time-to-first
resource must report a dedicated present-at-first-observation bucket rather
than inventing an under-15-minute activation duration.
The public Go payload, Settings preview TypeScript interface, and Pulse Pro
receiver must remain field/type-equivalent under
`scripts/check_telemetry_schema_parity.py`, except for explicitly named legacy
receiver-only compatibility fields. The receiver JSON struct is the storage
allowlist: unknown input fields are discarded, and inserts are generated from
that allowlist so a declared client field cannot be silently dropped by a
hand-maintained SQL list. Receiver values must clamp counts and validate every
new categorical value against its fixed set before storage. The adoption
report may aggregate those latest-per-install signals and use indexed time
filters plus compressed remote transport, but it must not enrich them with
accounts, request IPs, customer records, or event-level browser data.
The compressed remote-report transport must compile the exact generated Python
program in its unit proof. Its JSON-lines delimiter is escaped at the outer
generator boundary so the program sent over SSH contains a valid `b"\n"`
literal rather than an unterminated bytes literal.
That same telemetry trust boundary must remain operator-inspectable in-product:
the shared system settings surface may preview only the exact runtime payload
Pulse would send, and it must allow an operator to rotate the local telemetry
install ID immediately without waiting for the scheduled 30-day window.
An existing installation's first published schema-v2 upgrade must also receive
a one-time, non-blocking notice that names the coarse payload expansion and
links directly to the exact preview, the disable action, and the governed
privacy disclosure. Fresh installs stay silent because setup already presents
the current disclosure. Acknowledging the notice may persist locally, but it
must not change the operator's telemetry preference by itself.
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
Agent lifecycle state that explicitly requires a private local path uses the
stricter shared `internal/securityutil/private_path_*` contract. It must reject
symlinks and special files before hardening. POSIX paths must grant no group or
other access; Windows paths must disable DACL inheritance and grant access only
to an approved owner identity, LocalSystem, and Administrators. `os.Chmod`
success and Unix-style mode bits are not evidence of Windows privacy.
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
Live `api_tokens.json` reloads and in-process token mutations share that same
inventory authority. `internal/config/watcher.go` must serialize reading and
applying a disk snapshot with rename, scope-update, rotate, create, and revoke
mutations under `config.Mu`; an older snapshot may not be applied after a newer
mutation has updated runtime state and durable storage. The concurrency proof
is `TestConfigWatcher_ReloadAPITokensDoesNotOverwriteConcurrentMutation`.
That same trust boundary also governs API token scope identity: legacy
`host-agent:*` scopes may be accepted only at request-ingress or persistence/
migration boundaries, where they must be rewritten immediately into canonical
`agent:*` scopes. Live token records and runtime scope checks may not keep the
legacy scope names as an active second contract.
In-place token scope mutation belongs to that same boundary.
`PATCH /api/security/tokens/<id>` must require `settings:write`, normalize and
validate the complete replacement scope set, and prevent token-authenticated
callers from controlling a target whose existing authority exceeds their own
or granting authority they do not hold. Successful edits preserve the token
secret and all identity/binding metadata, apply to the next request, and
record both sides of the transition in a `token_updated` audit event without
including a raw secret or hash.
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
The containerized Unified Agent's daemon bridge does not widen that grant. It
accepts only the canonical typed lifecycle payload after command admission,
binds an immutable container id and an allowlisted start/stop/restart operation,
and exposes no raw daemon request, command text, arbitrary name, or general
socket capability to the model or server. It reuses the locally configured
module connection that already collects Docker / Podman inventory; a missing
or mismatched module fails before mutation, and an ambiguous mutating daemon
call is never retried automatically.
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
RBAC persistence follows the same single-owner and fail-closed posture.
`internal/api/access_tenant_provider.go` owns per-organization manager
selection, while `pkg/auth/sqlite_manager.go` is the canonical v6 store and
`internal/api/router.go` binds its default-organization instance to the global
SSO and authorization boundary. The enterprise runtime registers authorization
behavior only: `pulse-enterprise/internal/rbac/rbac.go` must resolve the current
global manager for each authorization check and must not construct, register,
or retain a parallel file-backed manager before the router installs SQLite.
`pkg/server/server.go` must not initialize a parallel file-backed manager.
Legacy `rbac_roles.json` and
`rbac_assignments.json` files are migration inputs only: the complete role and
assignment graph must validate and commit transactionally before either source
is archived. Corrupt JSON, missing role references, inheritance cycles, and
conflicts with newer v6 state must preserve the source files and make RBAC
unavailable with an explicit error rather than silently yielding empty data.
The canonical identity table must retain known local and stable SSO principals
when their role set is empty or a custom role is deleted, without retaining a
permission grant to the deleted role. It may also retain bounded mutable SSO
display-name, email, provider, and last-login metadata for operator
presentation, but those claims never replace the stable provider-scoped
principal used for authorization. Deprovisioning removes the identity and its
assignments, writes the acting administrator to the RBAC changelog, rejects
self-removal, and revokes every persisted session and matching CSRF record for
that principal, including sessions restored after restart. Deprovisioning is
not an IdP denylist: a later login that still passes provider restrictions may
recreate the identity. Enterprise regression coverage in
`pulse-enterprise/internal/rbac/rbac_test.go` must preserve the real startup
order and prove that an OIDC principal assigned after initialization is
authorized from the canonical manager.
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
Operator-created Agent tokens must default to the least authority needed for a
long-lived agent lifecycle: `agent:report`, `agent:config:read`, and
`agent:manage`. The custom scope chooser and in-place editor must expose
`agent:manage`; server-minted bootstrap, mobile-relay, and granular governed-
action scopes remain unavailable as general-purpose operator choices.
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
`PULSE_AGENT_INGEST_PORT` as a dedicated listener for the bounded agent control
plane so operators can place report/config traffic, command WebSocket
admission, version checks, and bootstrap downloads on one network or firewall
boundary, but the option must fail closed at validation: the agent
ingest port stays disabled at `0`, must be a valid `1`-`65535` port, and must
differ from both the frontend port and any HTTP redirect port. When that
listener is active the runtime must serve only `/api/agents/*` plus the exact
agent-owned `/api/agent/ws`, `/api/agent/version`, `/api/server/info`,
`/install.sh`, `/install.ps1`, and `/download/pulse-agent` routes. It must never
expose the web UI or management REST API through that port, so a port reachable
from an untrusted agent network cannot widen into the operator console.
Enabling the dedicated port is additive: the main listener keeps serving the
agent control plane too, so the default single-port deployment and existing
agents are unaffected.

Command admission is a separate security fact from report health. A live
session is owned by one organization, one token identity, one runtime agent ID,
and one hostname. Multi-organization exec tokens are ambiguous and must be
rejected; duplicate IDs remain isolated across organizations but fail closed
inside one organization when hostnames conflict. Legacy hostname-bound tokens
may migrate a synthesized ID to the first observed runtime ID once, after
which both identity fields are immutable. Revoked, expired, re-scoped, or
re-bound token records invalidate an existing socket before connectivity is
reported or work is dispatched. Raw bearer values must not be retained in the
session registry.

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
caller grant. It requires an eligible capability, an effective tenant Patrol
mode that admits the eligibility class, and an absent Never-auto-remediate
lock. An absent or empty per-resource policy inherits those server-owned global
bounds; an enabled per-resource capability limit and recurring window can only
narrow them. Unreadable or malformed state denies automatic execution.
The policy actor/method are server-stamped and cannot be supplied by the model,
enterprise orchestrator, browser, relay, or action-proposal payload.

Plan-time policy provenance follows the same trust boundary. The server owns
the version, authority kinds, source identities/revisions, scope, approval
result, and bounded reason vocabulary. Public payloads cannot provide the
object, and compatibility adapters must strictly reject malformed canonical
JSON rather than interpreting it as legacy. Cross-organization, duplicated,
contradictory, unsupported, and unbounded provenance cannot enter Memory or
SQLite audit truth. The field named `planningAllowed` is deliberately not an
execution grant; only current RBAC/scope checks and Task 04 dispatch policy can
authorize control.

The same server-derived boundary now applies to every public governed action.
Session actors are credential-bound and remain subject to current organization
membership and RBAC; browser-session scope bypass does not bypass those checks.
API-token actors must prove a current durable human owner, matching credential
and organization, current owner membership/role, and an explicitly enumerated
action or compatibility scope. Detached/service tokens cannot approve or
execute as humans, and owner-bound tokens can satisfy only authenticated/admin
floors, never cryptographic step-up. Ordinary session, API-token, mobile-local-
biometric, or UI method labels are not MFA evidence. MFA-required actions fail
closed unless the core verifier validates and single-use consumes evidence
bound to the actor, credential, organization, action, plan, outcome, and
challenge.

Action completion and resource-context projections expose only normalized and
redacted canonical evidence plus legacy compatibility fields derived from that
truth. A present but malformed `ActionResultV2` fails closed to bounded
inconclusive truth; it cannot fall back to legacy success or verified labels.
Independent verification requires durable evidence from a trust domain
distinct from the executor. Raw commands, provider stderr, credentials, and
secret-bearing text do not enter `ActionResultV2` projections. Stable bounded
refusal reason codes such as `resource_remediation_locked:` and `plan_drift:`
may remain visible so authorized consumers can branch without receiving raw
provider output.

### Patrol Autopilot acknowledgement authority

Autopilot acknowledgement and activation stamp the authenticated human session
as an immutable `ActionActor` binding with organization and credential. Public
actor, organization, version, scope, limits, time, and digest fields are
rejected; owner-bound and detached API tokens are both ineligible. Cross-org,
wrong-credential, tampered, stale, expired, revoked, or conflicting evidence
fails closed and cannot change effective full mode. The legacy unlock boolean
is retained only for wire/storage compatibility and is never treated as user
acknowledgement.

This is not an MFA implementation. No acknowledgement label or local browser
state may satisfy an action MFA floor without the existing server-verified,
action-bound cryptographic step-up contract.

Self-hosted commercial transitions authenticate the customer through the
existing emailed manage-code boundary, then bind the quoted catalog identity,
amount, proration timestamp, effective date, license version, continuity epoch,
Stripe customer, subscription, and item to one short-lived hashed quote token.
The apply route accepts that opaque token only and re-verifies authoritative
Stripe ownership and current state before mutation. Downgrade artifact cleanup
must stay within the tenant `reports/generated` directory and skip symlinks so
commercial retention cannot become an arbitrary-file deletion primitive.

Multi-destination Unified Agent configuration preserves authority separation.
The primary token is never reused for an observer, observer tokens are loaded
only from private regular non-symlink files, and observer URLs independently
enforce TLS, CA, fingerprint, and explicit plaintext policy. Observer payload
responses are report acknowledgements only and cannot authorize configuration,
commands, enrollment, or updates. Per-destination Proxmox tokens prevent one
Pulse instance from rotating credentials used by another.
The process-wide plaintext override belongs only to the authoritative primary:
it cannot satisfy an observer's opt-in. Every non-loopback HTTP observer must
declare its own `allowPlaintextHTTP` consent, and that decision must propagate
unchanged through host, Docker, Kubernetes, and observer Proxmox transports.

Operational Trust action offers enforce current plan, approve, and execute
authority before a mutating affordance is returned. Planning repeats those
checks after reloading the selected canonical record and before entering the
shared action lifecycle; execution retains the existing fresh authorization,
plan-hash, policy, and delivery gates. The browser cannot claim a first-party
origin, target, evidence set, handler, actor, or parameters. The server binds
the exact operational record and policy-shaped evidence IDs internally.
Unauthorized, stale, partial, permission-limited, ambiguous, unsupported, or
executor-unready records return no offer and no cross-resource detail.

Agent Fleet Doctor is an authenticated admin `settings:read` projection, not a
support-bundle export or command channel. Its identity evidence may expose only
a one-way machine-ID fingerprint, normalized platform metadata, and validated
IP/interface addresses. Raw machine IDs, credentials, tokens, environment
values, command text, and unbounded updater or module errors must not cross the
API boundary; secret-shaped error detail is reduced to bounded redacted
evidence. A repair entry is descriptive support metadata only and never grants
`settings:write`, local shell, agent execution, or action approval authority.
Unknown platform and unverified installer state fail closed without rendering
an executable upgrade command.

### Operational Trust evidence and mutation authorization

Evidence detail is tenant-scoped through the selected canonical record and
requires `monitoring:read`; lifecycle mutation requires `monitoring:write`.
Unknown record/evidence pairs do not permit cross-resource enumeration, and an
expired referenced envelope returns typed expiry without disclosing another
record. Acknowledge and suppression actors are server-derived. Suppression
accepts only a non-empty reason and bounded future expiry. Action offer and
planning require the `ai_autofix` entitlement in addition to current RBAC,
evidence, resource-capability, executor-readiness, approval, and action-policy
checks.

Guest, Docker, and host metadata reads, writes, and reloads must resolve the
active request tenant's monitor-owned store. A default-tenant cache must never
be reused to satisfy another organization, including Assistant-authored URL
updates, and stable workload identities must retain their full host or cluster,
namespace, kind, and name scope so a saved URL cannot cross a tenant or
resource boundary.

Agent update cache reconciliation on the shared router preserves the existing
credential boundary. `/api/agent/version` is non-cacheable, and current agents
put only the running version plus a non-secret request nonce in its query;
runtime tokens remain in protected request headers and redirects remain
rejected. The target-version query on `/download/pulse-agent` is cache identity,
not trust evidence: checksum, embedded-key signature, self-test, and atomic
replacement validation remain mandatory and fail closed.

### Request-derived absolute URLs share one trusted command-target boundary

`internal/api/router.go` owns one `resolveRequestOrigin` boundary behind
`requestOriginBaseURL`, public-URL capture, and the shared
`resolveConfiguredPublicBaseURL` precedence function consumed by
`Router.resolvePublicURL` and config-owned installer paths.
Direct `Host` values must parse as strict HTTP authorities and reject
userinfo, schemes, path/query/fragment bytes, whitespace and controls,
malformed DNS labels, invalid IPv4/IPv6 bracket forms, and ports outside
1-65535. `X-Forwarded-Proto`, `X-Forwarded-Scheme`, `X-Forwarded-Host`, and
`X-Forwarded-Port` are honored only when the immediate peer matches
`PULSE_TRUSTED_PROXY_CIDRS`, which is empty by default. Invalid forwarded
values are ignored independently rather than being copied or allowed to erase
a valid direct fallback.

This derivation is not authorization evidence, but it is security-sensitive
credential targeting. A validated live origin may outrank only an
auto-detected `PublicURL`; `AgentConnectURL` and explicit `PublicURL` remain
authoritative. Config-owned PVE/PBS install commands and setup-script
artifacts, diagnostics install commands, cluster deploy payloads, hosted
install commands, onboarding payloads, security status, SSO endpoint
responses, and magic links must consume the canonical resolver instead of raw
request headers because several of those payloads pair the resulting URL with
fresh API or bootstrap tokens. The legacy loopback-aware helper that copied
raw `Host` and untrusted forwarded scheme values is retired. Hosted mode
remains fail-closed without a configured external URL. The adversarial
contract proof is
`TestContract_RequestOriginCannotRetargetTokenBearingCommands` in
`internal/api/contract_test.go`, including direct execution of the PVE/PBS
`POST /api/agent-install-command` handler with a newly minted token.

Hosted fail-closed behavior is now enforced at the registered config routes,
not only inside the router-owned resolver. `Router.hostedMode` is immutable
input to `ConfigHandlers`; hosted config-owned consumers validate the selected
`AgentConnectURL` or explicit `PublicURL` through the canonical Pulse URL
validator and never accept an auto-detected URL, direct Host, or forwarded
origin as a substitute. PVE/PBS API-token issuance, setup-token issuance,
diagnostics container-runtime migration-token issuance, and password-driven
PBS token-label creation all occur only after that resolution succeeds.
`TestContract_HostedInstallerOriginsFailClosedAtRouter` proves the config-owned
install/setup boundary, while
`TestContract_HostedDiagnosticsDockerPrepareTokenValidatesOriginBeforeMutation`
proves that missing, auto-detected-only, invalid, or invalid-higher-precedence
hosted configuration returns 503 from the registered diagnostics route without
changing the in-memory or persisted API-token store. Both proofs keep valid
configured URLs authoritative under hostile direct, trusted-forwarded, and
untrusted-forwarded request evidence; the install/setup proof also follows the
returned setup artifact through the public script route so the rendered
token-bearing shell body remains bound to the configured origin.

The returning-user and post-checkout magic-link paths are credential-target
consumers of that same resolver. They must resolve a valid authoritative hosted
URL before `GenerateToken`, so missing, auto-detected-only, invalid, or invalid-
higher-precedence configuration cannot leave an unreachable HMAC token record
and cannot fall through to direct Host or trusted/untrusted forwarded headers.
The public route preserves one generic `200 OK` response for registered and
unknown email while leaving both supported stores empty; Stripe completion
skips only best-effort delivery while preserving billing and event idempotency.
Configured `PublicURL` and higher-precedence `AgentConnectURL` remain valid
targets. Exact in-memory/SQLite state and adversarial header proof lives in
`TestContract_HostedMagicLinkRequestValidatesOriginBeforeMutation` and
`TestStripeWebhook_CheckoutMagicLinkValidatesOriginBeforeMutation`.

### The global security banner no longer scores a truncated status payload

`/api/security/status` in `internal/api/router_routes_auth_security.go` returns
three payload tiers and names the tier it served in `detailLevel`. Only the
`privileged` tier (settings:read) carries `exportProtected`,
`apiTokenConfigured`, `hasHTTPS`, and `publicAccess`. The browser banner used
to compute its score from whatever arrived, so a kiosk API token holding
`monitoring:read` alone received an `authenticated` payload with those fields
absent, read them as disabled controls, and rendered a false "security score
1/5" banner whose "Enable Security" link pointed at a settings page the token
cannot open (#1650).

`shouldShowGlobalSecurityWarning` in
`frontend-modern/src/utils/securityScorePresentation.ts` now takes the served
`detailLevel` and refuses to assert anything the payload does not support. An
`authenticated` payload raises no warning. A `public` payload is authoritative
for `hasAuthentication` only, so an instance with no authentication configured
is still warned about — the case the banner exists for — while a public payload
that does have authentication contributes no invented posture debt. A
`privileged` payload keeps the full assessment unchanged, and a caller passing
the posture directly without a `detailLevel` is still treated as privileged.
`frontend-modern/src/components/SecurityWarning.tsx` carries the field through
from the response. This is presentation authority only; every settings endpoint
already enforces scopes server-side and no backend behaviour changed.
Regression coverage: the `Issue1650` cases in
`frontend-modern/src/utils/__tests__/securityScorePresentation.test.ts` and
`frontend-modern/src/components/__tests__/SecurityWarning.test.tsx`.

### Runtime branding reveals presentation material only after entitlement

The authenticated application header cannot depend on the admin-only full
system-settings response, so `/api/runtime/branding` is deliberately readable
with `monitoring:read`. That wider read authority is safe only because the
payload is a strict allowlist of three presentation fields and the server
fails closed: without the active `white_label` entitlement it returns
`enabled: false` with empty name and logo values. It never exposes
`logoPath`, other settings, environment values, licence records, or storage
locations.

Brand mutations remain behind `settings:write` and the existing report-brand
validation rejects unsupported keys, newlines, oversized base64, malformed
base64, and formats outside PNG/JPEG/GIF. Browser rendering consumes only the
server-filtered runtime payload; hiding controls or checking a client-side
capability is not treated as the authorization boundary.
`internal/api/runtime_branding_test.go` proves the no-entitlement non-leakage
and image normalization.
### Licensed-feature adoption telemetry stays count-only

Schema v6 of the outbound usage payload adds nine licensed-feature adoption
signals so Pro feature usage is measurable at all: `alert_ai_enabled`,
`rbac_custom_roles`, `rbac_user_assignments`, `audit_logging_persistent`,
`audit_events_30d`, `report_schedules`, `report_schedules_enabled`,
`report_schedules_run_30d`, and `agent_profiles`. Every one is a count or a
coarse boolean. Role names, permission sets, usernames, schedule names,
delivery recipients, report scope and contents, profile names, and every audit
event field (type, actor, target, outcome, detail) stay on the install.
`audit_logging_persistent` reports only that a persistent store is active
rather than console logging, and `audit_events_30d` is a retained-row count
inside the same 30-day window used by the other counters, so it cannot be
linked to one pseudonymous identifier indefinitely. Config-sourced signals are
read in `applyLicensedFeatureConfigSnapshot`
(`pkg/server/telemetry_licensed_features.go`); RBAC and audit are read behind
the router in `Router.ApplyLicensedFeatureTelemetrySnapshot`
(`internal/api/telemetry_licensed_features.go`). Both are disclosed in
`docs/PRIVACY.md` and its `frontend-modern/public/docs/` copy, pinned by
`TestAllTelemetryFieldsAreDisclosed`, and the counts are pinned by
`TestApplyLicensedFeatureConfigSnapshot_CountsScheduledReportingAndProfiles`
and `TestApplyLicensedFeatureTelemetrySnapshot_CountsOnlyOperatorAuthoredRBAC`.

### Telemetry RBAC reads never provision an RBAC store

The usage telemetry snapshot runs on a timer against every known org, so it
reads RBAC through `TenantRBACProvider.PeekManager`
(`internal/api/access_tenant_provider.go`), which returns an already-cached
manager and never creates one. Using the provisioning `GetManager` there would
have created a SQLite RBAC store for every org that has never used RBAC, as a
side effect of a background read. A store reporting a migration failure is
skipped rather than counted, so a degraded store cannot silently understate
adoption. `TestTenantRBACProvider_PeekManagerDoesNotProvision` and
`TestTenantRBACProvider_PeekManagerNilProviderIsSafe` in
`internal/api/rbac_tenant_provider_test.go` pin both properties, and
`TestApplyLicensedFeatureTelemetrySnapshot_DoesNotCreateRBACStores` pins it
through the router boundary.
### Audit adoption telemetry counts gated reads, never store presence

Schema v7 replaced `audit_logging_persistent` and `audit_events_30d` with
`audit_reads_30d`. The first two did not discriminate: `pkg/server` installs the
SQLite audit logger on every install for defense in depth and gates only the
read and export endpoints, so the boolean was true on every install that
reported it and the event count measured background write volume, saturating
the receiver clamp on unlicensed community installs. `audit_reads_30d` counts
requests that cleared `RequireLicenseFeature(featureAuditLoggingValue, ...)` on
an audit read or export surface, which requires a human action and so cannot
settle into a constant. The recorder in
`internal/api/telemetry_audit_reads.go` is wrapped inside the licence gate, and
`AuditReadActivityRecord` (`internal/config/audit_read_activity.go`) carries
only a timestamp and an activity class drawn from a fixed allowlist; unknown
classes are dropped rather than stored. Query filters, requested ranges, the
actor, and every audit row read stay on the install.
`TestWithAuditReadActivity_RecordIsContentFree` and
`TestRecordAuditReadActivity_RejectsUnknownActivity` pin both properties.

### Telemetry ingestion matches the released sender while storage stays compatible

The active outbound contract is schema v16. Schema v8 added content-free
approved-action refusal counters for target change, prerequisite failure, and
invalid typed contract so agent-side pre-mutation failures no longer collapse
into `other`. Schema v9 completes that split with a content-free `uncoded`
counter for refusals that carried no machine reason code at all, which is what
every agent older than the typed refusal contract reports. Without it a split
starved by agent rollout is indistinguishable from a broken one, because both
present as `other` absorbing every refusal. Schema v10 adds the Patrol blocked
cause: one fixed machine enum exported only while an enabled Patrol is in the
blocked runtime state, so an install whose Patrol can never run is
distinguishable from one that runs and finds nothing. Blocked-reason text,
provider endpoints, model names, and configuration stay on the install, and an
untyped blocked reason exports nothing rather than free text.
Schema v16 adds four workload-history adoption counts. Each closed activity is
deduplicated once per browser session before the authenticated local intake,
stored only as a bounded UTC-day count, and exported only as a rolling 30-day
total. The contract forbids raw browser events, event-level clickstream data,
guest/user identity, routes, selected ranges, cursor coordinates or values,
interaction timing, and browser identity.
The earlier draft schema-v8 `business_estate` field was reverted and must not
remain in the license server's accepted ping struct merely because a private receiver build
and database migration briefly carried it. Existing deployed databases need no
destructive column migration, while new databases do not recreate the retired
column. Incoming draft-field values are ignored and can never enter adoption
reporting. `TestTelemetryPing_IgnoresRetiredBusinessEstateField` pins that
boundary, while `scripts/check_telemetry_schema_parity.py` requires the public
sender, frontend preview, and active private receiver struct to stay exact apart
from the named receiver-only inputs. `license_tier` and `api_tokens` remain
receiver-derived compatibility inputs, while `deployment_proof` is a
receiver-only marker for synthetic operator verification traffic. None is
accepted as an outbound public heartbeat field.

### Adoption reporting aggregates high-cardinality history in one pass

`scripts/telemetry_adoption_report.py` must select only its explicit reporting
projection instead of copying every receiver column over SSH. That projection
includes the licensed-feature, availability-probe, and updater signals added to
the released schema, while excluding the retired `business_estate` draft.
SQLite reduces remote history to one latest-state row plus compact sufficient
facts per install: first free, first paid, observed signal fields, and signal
fields observed while free before the first paid posture. For an explicitly
selected target release, those compact facts also include the first and latest
target-version heartbeat times, consecutive same-version pair counts, the
first-heartbeat values, and separately accumulated increases and decreases for
the closed activity-counter projection. This preserves
latest-state reporting, first-free/first-paid conversion, and all outcome
cohort membership while allowing inherited first-heartbeat totals,
same-version net changes, and later version departures to remain distinct,
without sending raw heartbeat history over SSH. The local
fallback must analyze each row once and keep only bounded per-install evidence
sets and earliest observation times. It must not regroup rows into per-install
history lists or sort those lists before producing the outcome cohorts and
operations funnel. The production-scale guard exercises 120,000 rows across
12,000 installs, verifies exactly one timestamp parse per row, and keeps the
cohort plus funnel aggregation inside the bounded runtime budget.

Reporting must remain usable while an operator upgrades an older receiver
database. The report inspects `telemetry_pings` before building its explicit
projection, substitutes zero only for known requested fields that are absent,
and emits the absent-field list as a source-schema warning. It must not expand
to unknown columns, hide a missing field without that warning, or reinterpret
an unavailable counter as positive adoption evidence.

### Licensed-feature adoption fields must discriminate

`telemetry.LicensedFeatureAdoptionFields` registers every telemetry field whose
purpose is to measure adoption of a licensed feature, and
`TestLicensedFeatureAdoptionFieldsDiscriminate`
(`pkg/server/telemetry_licensed_features_guard_test.go`) fails if any registered
field is non-zero on an install that uses none of them. The guard builds that
install through the real production snapshot paths and installs a real SQLite
audit logger exactly as `pkg/server` does, because pinning a console logger
would let the guard pass while the outbound payload lied. A field that reads the
same on a used and an unused install measures nothing and is worse than no field
because it looks like data in the fleet aggregate.
`TestRetiredNonDiscriminatingFieldsStayRemoved` additionally pins
`audit_logging_persistent`, `audit_events_30d`, and
`pulse_intelligence_patrol_autofixes_30d` so they cannot return under their old
names.
### Per-tenant resource stores are released on offboarding and shutdown

`ResourceHandlers.getStore` opens a SQLite handle per org and caches it for the
process lifetime. `CloseTenantStore` releases and evicts one org's handle and is
called from `Router.CleanupTenant` alongside the other per-tenant teardown;
`CloseStores`, exposed as `Router.ShutdownResourceStores`, releases all of them.
Without this an offboarded tenant kept its file descriptors and its
`unified_resources.db-wal`/`-shm` files alive and its directory could not be
fully removed. Closed stores are evicted from the cache so a later request opens
a fresh handle rather than using a closed one
(`TestResourceHandlers_CloseTenantStoreReleasesTheHandle`), and both entry points
are idempotent and nil-safe
(`TestResourceHandlers_CloseIsIdempotentAndNilSafe`).
### Report-ack server versions cannot steer agents beyond one validated check

The unified-agent report ack's `serverVersion` echo gives acks a version
channel, but it carries no update authority. Only the authoritative Pulse
destination's ack reaches the updater hook — observer destination acks are
discarded before config parsing
(`TestAgentSendReport_ObserverAckNeverInvokesCallback`) — and a nudged updater
re-fetches the server version itself and runs the existing download, checksum,
and self-test pipeline before swapping binaries, so a stale or spoofed ack
version can at most trigger one extra validated check
(`TestRunLoopRunsCheckOnNudge`).

### Routine authorization refusals log at debug; the rate is what warns

`RequireAuth`, `RequireAdmin` and `RequirePermission` used to emit `log.Warn()`
for every refusal — an unauthenticated caller before login, a non-admin on an
admin route, an RBAC denial — and `internal/api/middleware.go` independently
warned on *every* 4xx. A single refusal therefore produced two warn lines, and a
correctly configured instance could not produce a quiet log. #1601's rc.9
reporter read that stream as an RBAC regression.

Refusals now route through `logAuthDenial`
(`internal/api/auth_denial_signal.go`), which records the refusal at debug and
counts it per caller. Attribution prefers the authenticated username so a
principal is still tracked across rotating source addresses, falling back to
`GetClientIP`. When one caller crosses `authDenialWarnThreshold` refusals inside
`authDenialWindow`, exactly one warn is emitted for that window
(`"Repeated authorization denials from one caller; possible endpoint probing"`),
which is the shape that actually distinguishes probing from a UI mounting a
surface its session cannot read. The tracked set is bounded by
`authDenialMaxTracked` with oldest-window eviction so spoofed forwarded-for
values cannot grow it without bound.

The middleware now warns only on 5xx — a fault this build owns — and logs 4xx at
debug.

**Enforcement is unchanged.** Every route returns the same status to the same
callers; only the log level and the escalation moved. Verified live on a
proxy-auth instance: `/api/connections`, `/api/updates/status` and
`/api/system/settings` still return 403 to a viewer and 200 to an admin, while
an idle non-admin browser session produced zero warn lines across 90 seconds
where it previously produced roughly one per refusal plus a paired
`"Request failed"`.

Pinned by `auth_denial_signal_test.go`: below-threshold refusals stay at debug,
the escalation fires exactly once per window, a closed window re-arms it, a
username aggregates across addresses, separate callers keep separate budgets,
and the tracked set stays bounded.

### Global update polling follows the route's served authority

`/api/updates/status` remains protected by `RequireAdmin` plus
`settings:read`. The authenticated app shell now mounts its five-second
fallback watcher only when `/api/security/status` serves
`settingsCapabilities.systemSettingsRead=true`, the capability derived from
that same admin-and-scope predicate. Authenticated viewers therefore do not
generate a perpetual stream of expected 403 responses. Auth-free installations
remain an explicit exception: `requiresAuth=false` keeps the watcher mounted
because `RequireAdmin` intentionally leaves the update route reachable there.
The frontend behavior is pinned by `App.architecture.test.ts`, and
`TestContract_SecurityStatusSystemSettingsReadTracksSettingsReadScope` pins the
capability against both `/api/system/settings` and `/api/updates/status`.

### Resource policy identity resolution remains tenant scoped

Operator-state reads and writes resolve source-native or superseded resource
references through the requesting tenant's canonical registry before touching
the tenant resource store. Runtime reconciliation may visit every live monitor,
but each alert manager resolves policy through its own tenant-scoped store, so a
matching provider ID in another organization cannot import the mutation. The
existing route scopes and authenticated actor attribution remain unchanged.

### Secret-bearing configuration transfer fails closed before data access

Configuration archives may contain node credentials, notification secrets,
API-token hashes and metadata, OIDC client secrets, and SAML private keys. The
export/import router therefore authorizes the resolved organization before
request-body parsing or persistence access. Hosted mode, enabled persisted or
environment-backed SSO, and an SSO load failure all require authentication;
none may fall through to no-auth recovery. Browser sessions need instance-admin
or tenant-management authority, proxy identities need the configured admin
role, and API tokens preserve organization binding plus the operation-specific
`settings:read` or `settings:write` scope.

No-auth recovery trusts only a direct loopback transport without forwarded
identity headers. `ALLOW_UNPROTECTED_EXPORT` is an export-only exception and is
ineffective once any authentication or hosted mode is active. Security status
reports that effective policy rather than the environment variable alone, and
the root and shipped security guides remain byte-for-byte synchronized. The
router matrix proves that malformed and otherwise valid denied requests read no
body and perform no export, import, config replacement, or runtime reload.

### Packaged server identity outranks inherited image metadata

`pkg/server.Run` binds the version supplied by the executing binary into the
canonical update/version subsystem before runtime initialization. Wrapper
binaries such as Pulse Pro therefore report the same packaged identity through
`/api/version` that they report through `--version`, even when a derived test
container inherits a stale `VERSION` file from its base image. The public
endpoint remains non-secret release metadata; this binding does not expose
inventory, credentials, update selections, or command authority.
`TestRunBindsPackagedVersionIdentity` in `pkg/server/server_test.go` pins the
wrapper-runtime boundary.

### Least-privilege agent install profile boundaries

The least-privilege agent install profile is a security boundary, not a
convenience flag: `install.sh --least-privilege` must keep the service user
non-root with a nologin shell, keep every sudoers grant exact-command and
visudo-validated with the pct grant excluding `pct exec`/`start`/`stop`/
`enter`, refuse `--enable-commands` under the profile, refuse unsupported
platforms instead of silently reverting to root, and drop the LXC-attach
ambient capability grant. `NoNewPrivileges` stays enabled on a grantless
profile and is relaxed only when a sudo grant is active, because NNP blocks
sudo outright; that relaxation is part of the grant's declared cost. The agent-reported privilege profile is
informational: the fleet doctor presents it descriptively and must not treat
a non-root agent as unhealthy on that evidence alone.

### API token creation and revocation are durable credential transitions

Creation may not admit an unreturned secret or evict an older valid token when
persistence fails. Both token-management creation and shared agent
install-token issuance retain the complete pre-creation inventory until the
expanded inventory commits, then restore that inventory and its primary-token
projection before returning an error on failed writes. The generated secret
and record are returned only after a successful commit. The agent-install
failure proof is
`TestIssueAndPersistRollsBackCompleteInventoryWhenPersistenceFails` in
`internal/api/agenttokens/install_test.go`.

Revocation may not create different live and restart-time credential sets.
The token-management API therefore retains a complete pre-mutation inventory
until the reduced inventory is persisted. A failed write restores the prior
tokens and primary-token projection, emits only a failed `token_deleted` audit
event, and returns an error; a successful response identifies a deletion that
will survive restart. Creation rollback, exact multi-token removal, and
revocation persistence-failure rollback are exercised in
`internal/api/security_tokens_lifecycle_test.go`.

Automatic credential cleanup after host-agent, Docker-host, or Kubernetes
cluster removal obeys the same rule. One shared lock protects mutation and
persistence; a failed reduced-inventory write restores all prior records and
the primary-token projection. The resource remains removed, but Pulse keeps
the orphaned credential visibly active so restart cannot silently reverse a
claimed revocation.
`internal/monitoring/monitor_host_agent_removal_lifecycle_test.go` proves both
commit and rollback outcomes.

### Schema v15 separates destination server failures from rejections

Telemetry schema v15 adds only the bounded integer field
`notification_failures_server_error_7d`. It counts terminal notification jobs
whose locally classified response was HTTP 5xx. New schema v15 terminal
failures enter `notification_failures_rejected_7d` only for HTTP 4xx responses
after the dedicated authentication and rate-limit buckets. Retained pre-upgrade
rows can preserve the v5-v14 mixed 4xx/5xx meaning for up to seven days, so
first heartbeats are baseline-only and reports must use consecutive
same-version deltas. No raw error, response, destination, provider,
notification, alert, tenant, account, or infrastructure identity is sent.
Sender, receiver, local preview, adoption report, and the public privacy copies
must remain in lockstep.

### Deploy enrollment never exposes an uncommitted credential

The deploy bootstrap secret is single-use only at the durable API-token commit
boundary. Pulse generates the host-bound runtime credential without admitting
or returning it, then replaces the bootstrap record under one lock and persists
the resulting inventory once. A concurrent replay cannot pass the locked
removal. A failed persistence write restores the complete prior inventory and
primary-token projection, returns an error, and discloses no runtime secret;
failed bootstrap minting likewise returns no credential material. The success,
replay, and forced-write-failure paths are exercised in
`internal/api/deploy_handlers_test.go`.

### Mobile alert notifications are private by construction

Ordinary warning and critical alert pushes contain a generic severity-aware
title and private prompt to open Pulse Mobile; lock-screen payloads do not
include hostnames, resource names, addresses, metric values, thresholds,
provider identifiers, probe targets, or notification credentials. The action
carries only the canonical alert ID required to open the authenticated alert
surface, where current state is re-read rather than trusted from stale push
copy. External-probe outages retain their existing private specialized
message. Relay configuration responses expose the normalized alert-severity
floor but never the Relay private key, and omitted or invalid floors fail open
to `all` rather than silently suppressing warnings. Payload-shape and config
normalization proofs live in `internal/relay/push_test.go` and
`internal/api/relay_hosted_runtime_test.go`.

### Alert correlation stays inside authenticated incident context

The optional alert `correlation` object carries only a bounded shared-system
key, kind, presentation role, and auditable reason. It is available through the
same authenticated alert read surfaces as the resource and instance identity
already present on the alert; it must not be copied into public telemetry,
lock-screen push copy, notification destination configuration, or
unauthenticated status output. The overview uses only the typed role and group
membership and does not render the raw key or reason.

Malformed, unsupported, or incomplete persisted correlation fails open during
alert cloning and is omitted from client payloads. Clients likewise require
the closed `shared-system` kind and a non-empty key before cross-resource
grouping, and they never infer identity from hostnames, message text,
timestamps, or resource-path truncation. `internal/alerts/correlation_test.go`
and the alerts overview state tests pin these disclosure and fail-open
boundaries.

### Typed-helper container inventory never delegates daemon authority

Under the typed-helper profile, the collector admits a direct container
runtime only when the endpoint is a real Unix socket below
`/run/user/<collector-uid>`, owned by that UID, and not a symlink. Admission
occurs before the first daemon API request and is repeated on reconnect.
Rootful, remote, missing, malformed, and replaced candidates are closed and the
collector falls back to the helper's fixed-endpoint summary operation.

The helper request contains no daemon URL, HTTP method, query, container
selector, or mutation argument. Summary mode does not bind lifecycle or update
bridges, and its report labels the reduced authority as
`typed-helper-summary`; helper loss cannot trigger sudo, root execution, or a
broader direct socket fallback.

### Journald priority framing does not alter durable or live log payloads

The systemd-only `PULSE_LOG_JOURNAL_LEVEL_PREFIX=true` opt-in wraps the
process stream with zerolog-aware syslog priority framing so systemd can assign
native journal severity. The prefix is a transport marker, not part of the
structured event: it must be applied before the stderr branch only and systemd
must remove it from `MESSAGE`. The owner-only rotating file sink and the
authenticated in-memory live-log broadcaster continue receiving the original
unprefixed record, preserving their existing payload and access boundaries.

`internal/logging/logging_test.go` pins both the level-to-priority mapping and
that sink isolation. Deployments that do not explicitly opt in—including
containers and interactive terminals—must retain their unprefixed output.
