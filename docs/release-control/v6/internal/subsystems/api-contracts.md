# API Contracts

## Contract Metadata

```json
{
  "subsystem_id": "api-contracts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/api-contracts.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "patrol-intelligence"
  ]
}
```

## Purpose

Own canonical runtime payload shapes between backend and frontend.

## Canonical Files

1. `internal/api/contract_test.go`
2. `internal/api/resources.go`
3. `internal/api/alerts.go`
4. `internal/api/activity_audit_handlers.go`
5. `frontend-modern/src/types/api.ts`
6. `frontend-modern/src/api/responseUtils.ts`
7. `frontend-modern/src/components/Settings/APITokenManager.tsx`
8. `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx`
9. `frontend-modern/src/components/Settings/UnifiedAgents.tsx`
10. `frontend-modern/src/utils/agentInstallCommand.ts`
11. `frontend-modern/src/api/nodes.ts`
12. `frontend-modern/src/api/license.ts`
13. `frontend-modern/src/api/monitoredSystemLedger.ts`
14. `frontend-modern/src/api/resources.ts`
15. `frontend-modern/src/api/monitoring.ts`
16. `internal/api/monitored_system_ledger.go`

## Shared Boundaries

1. `frontend-modern/src/api/agentProfiles.ts` shared with `agent-lifecycle`: the agent profiles frontend client is both an agent lifecycle control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/ai.ts` shared with `ai-runtime`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
3. `frontend-modern/src/api/nodes.ts` shared with `agent-lifecycle`: the shared Proxmox node client is both an agent lifecycle setup/install control surface and a canonical API payload contract boundary.
4. `frontend-modern/src/api/notifications.ts` shared with `notifications`: the notifications frontend client is both a notification delivery control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/api/patrol.ts` shared with `ai-runtime`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
6. `frontend-modern/src/api/security.ts` shared with `security-privacy`: the security frontend client is both a security/privacy control surface and a canonical API payload contract boundary.
7. `frontend-modern/src/api/updates.ts` shared with `deployment-installability`: the updates frontend client is both a deployment-installability control surface and a canonical API payload contract boundary.
8. `frontend-modern/src/components/Settings/APITokenManager.tsx` shared with `security-privacy`: the API token settings surface is both a security/privacy control surface and a canonical API payload contract boundary.
9. `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx` shared with `agent-lifecycle`: the infrastructure operations controller is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
10. `frontend-modern/src/components/Settings/UnifiedAgents.tsx` shared with `agent-lifecycle`: the UnifiedAgents module is a compatibility shim for the canonical infrastructure operations controller and remains on the same shared agent lifecycle and API contract boundary while the old module path exists.
11. `frontend-modern/src/utils/agentInstallCommand.ts` shared with `agent-lifecycle`: the shared frontend install-command helper is both an agent lifecycle control surface and a canonical API/install transport contract boundary.
12. `internal/api/agent_install_command_shared.go` shared with `agent-lifecycle`: agent install command assembly is both an agent lifecycle control surface and a canonical API payload contract boundary.
13. `internal/api/ai_handler.go` shared with `ai-runtime`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
14. `internal/api/ai_handlers.go` shared with `ai-runtime`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
15. `internal/api/ai_intelligence_handlers.go` shared with `ai-runtime`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.
16. `internal/api/config_setup_handlers.go` shared with `agent-lifecycle`: auto-register and setup handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
17. `internal/api/licensing_bridge.go` shared with `cloud-paid`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
18. `internal/api/licensing_handlers.go` shared with `cloud-paid`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
19. `internal/api/notifications.go` shared with `notifications`: notification handlers are both a notification delivery control surface and a canonical API payload contract boundary.
20. `internal/api/payments_webhook_handlers.go` shared with `cloud-paid`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
21. `internal/api/public_signup_handlers.go` shared with `cloud-paid`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.
22. `internal/api/resources.go` shared with `unified-resources`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
23. `internal/api/security.go` shared with `security-privacy`: the security handlers are both a security/privacy control surface and a canonical API payload contract boundary.
24. `internal/api/security_tokens.go` shared with `security-privacy`: the security token handlers are both a security/privacy control surface and a canonical API payload contract boundary.
25. `internal/api/slo.go` shared with `performance-and-scalability`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.
26. `internal/api/system_settings.go` shared with `security-privacy`: the system settings telemetry and auth controls are both a security/privacy control surface and a canonical API payload contract boundary.
27. `internal/api/unified_agent.go` shared with `agent-lifecycle`: unified agent download and installer handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
28. `internal/api/updates.go` shared with `deployment-installability`: update handlers are both a deployment-installability control surface and a canonical API payload contract boundary.
## Extension Points

1. Add or change payload fields through handler + contract tests together
2. Update frontend API types in lockstep with backend contract changes
3. Add dedicated contract tests for new stable payloads
4. Route unified resource sensitivity, routing, and `aiSafeSummary` payload changes through `internal/api/resources.go`, `internal/api/contract_test.go`, and the canonical frontend resource consumer proofs together; resource governance metadata must not ship as an API-only or frontend-only heuristic
5. Route unified-resource action, lifecycle, and export audit reads through `internal/api/activity_audit_handlers.go`, `internal/api/router_routes_licensing.go`, and `internal/api/contract_test.go` together so the control-plane execution trail stays on a governed API contract instead of a store-only shape
6. Route dedicated unified-resource timeline and facet-bundle reads through `frontend-modern/src/api/resources.ts`, `internal/api/resources.go`, and `internal/api/contract_test.go` together so the backend facet contract and the frontend client stay aligned on one timeline-first surface, while capability and relationship detail stays backend-owned for AI correlation and change detection
7. Route canonical AI intelligence summary and resource-intelligence reads through `frontend-modern/src/api/ai.ts`, `frontend-modern/src/stores/aiIntelligence.ts`, `frontend-modern/src/pages/AIIntelligence.tsx`, `internal/api/ai_handlers.go`, and `internal/api/contract_test.go` together so the summary card, store state, and backend payload stay aligned on one governed surface, including the canonical recent-changes slice
   while keeping the learning counters backend-only coverage, so the summary page keeps Patrol health and findings primary and renders timeline, correlation, and policy-posture data as secondary investigation context rather than as a separate headline product metric
   and the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx` card, so canonical recent-change timelines stay rendered through one governed frontend card instead of separate page-local list loops
   and the shared `frontend-modern/src/utils/resourceChangePresentation.ts` formatter used by the summary page and resource drawer, so canonical change wording does not drift across surfaces
   and the `/api/ai/intelligence/changes` route plus `internal/api/contract_test.go`, so the canonical recent-changes endpoint stays on the same intelligence facade and contract snapshot instead of bypassing the shared timeline source
   and the canonical policy-posture snapshot derived from unified resources, so sensitivity, routing, and redaction counts stay owned by the same AI summary contract instead of being reconstructed as a page-local governance rollup
   and the resource-intelligence payload carried by the drawer AI card, so the resource-detail surface stays on one canonical intelligence contract instead of introducing a separate detail endpoint
   and the learned-correlation payload loaded into the shared AI intelligence store, so the Patrol intelligence page and the AI summary page consume the same governed correlation slice instead of each page fetching its own copy
   and the shared dashboard-load bundle inside `frontend-modern/src/stores/aiIntelligence.ts`, so the page orchestration stays on the store-owned bundle instead of enumerating the AI fetches inline
   and the shared `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx` card, so the AI summary page renders the governed policy-posture counts while the resource drawer stays on per-resource policy lines instead of carrying duplicate posture UI loops
   and the shared `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx` card, so learned correlations and correlation context stay rendered through one governed frontend card instead of separate page-local list loops
   and the same shared correlation card's ordering and truncation rule, so callers pass raw correlations instead of encoding their own top-N sort behavior
   and the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx` and `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx` cards' infrastructure resource-link default, so the Patrol page, resource drawer, and problem-resource dashboard panels inherit the canonical resource-filter path construction instead of rebuilding infrastructure URLs inline
8. Route frontend API-client parsed error propagation, API-error-status fallback handling, allowed-status handling, custom status-specific error handling, command-trigger success envelope handling, shared response parsing pipelines, missing-resource lookup handling, metadata CRUD routing, stream event consumption, response status, collection normalization, scalar payload coercion, and structured error normalization through canonical shared helpers under `frontend-modern/src/api/`
9. Add or change API token scope, assignment, and revocation presentation through `frontend-modern/src/components/Settings/APITokenManager.tsx`
10. Add or change infrastructure operations token generation, lookup, assignment, and reporting/install presentation through `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx`
11. Keep `internal/api/session_store.go` on a fail-closed auth-persistence boundary: persisted OIDC refresh tokens may only round-trip through encrypted-at-rest session payloads, and any missing-crypto or invalid-ciphertext path must drop the token instead of preserving plaintext-at-rest session state.
12. Keep tenant AI handler wiring on canonical provider ownership: `internal/api/ai_handlers.go` may wire tenant `ReadState` and tenant-scoped unified-resource providers into AI services, but it must not revive tenant snapshot-provider bridges once Patrol can initialize and verify from those canonical providers directly.

## Forbidden Paths

1. Handler-local payload shape drift without a contract test
2. Untracked compatibility aliases becoming permanent runtime contracts
3. Frontend-only payload assumptions that are not owned in backend contracts
4. Frontend API clients inferring canonical HTTP status from `Error.message` text
5. Frontend API clients branching on raw `response.status` checks for governed status handling instead of the shared response-status helpers
6. Frontend API clients parsing governed success or stream payloads with raw `response.json()`, ad hoc `response.text()` + `JSON.parse(...)`, or per-module `JSON.parse(...)` stream decoding instead of the shared response parsing helpers
7. Frontend API clients normalizing nullable or legacy collection payloads with module-local `|| []`, `?? []`, or ad hoc `Array.isArray(...)` fallbacks instead of shared collection helpers
8. Frontend API clients swallowing non-not-found API failures behind broad `catch { return null; }` fallbacks instead of routing only canonical `404` cases through explicit status checks
9. Frontend API clients coercing governed backend payload fields through module-local scalar helper stacks instead of shared scalar coercion helpers
10. Frontend API clients normalizing governed structured error payloads through module-local helper functions instead of shared error normalization helpers
11. Frontend API clients open-coding parsed non-OK response throwing with `throw new Error(await readAPIErrorMessage(...))` instead of the shared response assertion helper
12. Frontend API clients open-coding governed `assertAPIResponseOK(...); parseRequiredJSON(...)` or `parseOptionalJSON(...)` tandems instead of shared response pipeline helpers
13. Frontend API clients open-coding governed `404 => null` response branches for resource lookups instead of shared missing-resource response helpers
14. Agent and guest metadata clients duplicating the same CRUD transport logic instead of using one shared metadata client
15. AI stream clients duplicating SSE reader, timeout, chunk-splitting, and JSON event parsing loops instead of using one shared stream consumer
16. Monitoring delete and idempotent mutate clients open-coding `404`/`204` allowed-status branches instead of using canonical shared allowed-status helpers
17. Governed frontend API clients open-coding `if (!response.ok) { if (isAPIResponseStatus(...)) throw new Error(...) }` status-to-user-message branches instead of using canonical shared custom-status error helpers
18. Monitoring command-trigger clients open-coding `parseOptionalAPIResponse(response, { success: true }, ...)` success-envelope fallbacks instead of using a canonical shared success-envelope helper
19. Governed frontend API clients open-coding `try/catch` wrappers around `apiFetchJSON(...)` just to map `402` or `404` into `[]`, `{ plans: [] }`, or `null` instead of using canonical shared API-error-status fallback helpers

## Completion Obligations

1. Update contract tests when payloads change
2. Update frontend API types in the same slice
3. Route runtime changes through the explicit API-contract proof policies in `registry.json`; default fallback proof routing is not allowed
4. Update this contract when canonical payload ownership changes
5. Keep `/api/resources` policy metadata aligned across backend payload tests and canonical frontend resource consumers whenever sensitivity or routing fields change

## Current State

The API layer already uses contract tests in many places, but every major live
contract should continue moving toward canonical-only runtime shapes.
The unified resource API payload now carries the richer domain facets directly
through the owned backend response: resource objects can expose canonical
`capabilities`, `relationships`, `recentChanges`, and derived `facetCounts`
in addition to policy and identity metadata, so the backend payload contract
stays aligned with the timeline and control-plane model instead of flattening
those fields away. The frontend consumer, however, only preserves the
timeline-first `recentChanges` slice and its counts on the bundle contract.
The same resource contract now also exposes a dedicated
`/api/resources/{id}/timeline` history endpoint and bundled facet reads under
`/api/resources/{id}/facets`, so operators can inspect change history without
depending on a monolithic resource payload.
The `/api/resources` serializer now also refreshes canonical identity and
policy metadata through the shared unified-resource helper before it writes
the payload, so backend and frontend contract tests stay aligned on one
canonical metadata pass instead of consumer-local attach wrappers.
Those history reads now also accept governed `kind`, `sourceType`, and
`sourceAdapter` query filters, and the backend store owns the corresponding
filtered counts, so the timeline contract can narrow by change class and
adapter provenance without inventing a frontend-only relationship slice.
The same facet bundle contract now also returns grouped `recentChangeKinds`
counts by canonical `ChangeKind`, so the shared drawer and summary chips can
show the distribution of restarts, anomalies, state transitions, and other
timeline classes without guessing from the loaded slice.
The same facet bundle contract now also returns grouped
`recentChangeSourceTypes` counts by canonical source type, so the shared
drawer and summary chips can distinguish platform events, pulse diffs,
heuristics, user actions, and agent actions without inventing frontend-local
provenance heuristics.
The same facet bundle contract now also returns grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so the
shared drawer and summary chips can distinguish Docker, Proxmox, TrueNAS, and
ops-helper provenance without inventing frontend-local integration heuristics.
Canonical timeline entries now also preserve correlation context in
`relatedResources`, so the history surface can explain which neighboring
resources moved with restart, anomaly, config, state transition, and
relationship changes instead of only exposing correlation endpoints when the edge
itself changed.
Restart timeline entries are also a first-class contract now: `restart` change
kinds can serialize Docker and Kubernetes restart metadata instead of being
folded into generic state transitions.
Incident-driven anomaly entries are also a first-class contract now:
`metric_anomaly` change kinds can serialize canonical incident rollup changes
instead of being flattened into generic status churn.
For relationship changes, the `from` and `to` fields now summarize the actual
edge(s) rather than only the parent pointer, so the API contract keeps the
relationship transition legible even before the frontend expands the
related-resource chips.
The same relationship and change presenters now also own the state, restart,
incident, and config summary fragments that feed those timeline values, so the
API surface preserves the canonical wording before the frontend renders it.
Invalid `sourceAdapter` values are rejected at the API boundary, so the filter
contract stays aligned with the canonical adapter set rather than silently
falling back to an empty slice.
The same resource-timeline contract now also owns canonical parsing for
`kind`, `sourceType`, and `sourceAdapter` query values, so the HTTP handler
stays thin and the change model remains the source of truth for timeline
filter validation.
The same API contract now also exposes the unified-resource control-plane
history through dedicated enterprise audit reads. The action, lifecycle, and
export history endpoints live in `internal/api/activity_audit_handlers.go` and
`internal/api/router_routes_licensing.go`, and the contract tests now pin their
response shapes so the execution trail remains queryable through the governed
API surface rather than only through the underlying store.
Action-plan stale-plan protection on those audit records now uses the canonical
`resourceVersion`, `policyVersion`, and `planHash` fields only, so the
response contract stays deterministic without extra version baggage.
The same API contract now also owns the dedicated frontend resource facet
client in `frontend-modern/src/api/resources.ts`, which fetches the governed
capability, relationship, and timeline surfaces from `internal/api/resources.go`
instead of teaching the drawer or list views to reconstruct them inline.
The same AI resource-intelligence payload now also carries dependency and
dependent correlation arrays plus correlation evidence, so the drawer can render
canonical correlation context from the shared AI contract instead of inferring it
from the relationship facet payload alone.
The same AI frontend client now also loads `/api/ai/intelligence/correlations`
through the shared `frontend-modern/src/stores/aiIntelligence.ts` store for
the Patrol intelligence page and the AI summary page, so the
learned-correlation list is governed by the same API contract that backs the
resource drawer's correlation evidence instead of being fetched as page-local state.
That correlations route now reads through the canonical AI intelligence
facade first, so the handler and its payload keep the detector behind one
shared access layer instead of routing directly to Patrol-local correlation
state.
That store now also owns the dashboard load bundle used by the Patrol page,
so the page refresh path stays aligned on one store-owned orchestration layer
instead of re-encoding the AI bundle inline.
The AI summary page now also renders the canonical
`frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
card for policy posture, so sensitivity, routing, and redaction counts are
presented through one governed frontend component while the resource drawer
keeps only the per-resource policy lines.
The unified action, lifecycle, and export audit reads now also clamp oversized
`limit` requests to the governed maximum of `1000`, so the control-plane audit
surface stays bounded even when callers ask for arbitrarily large history
pages.
Those relationship and timeline payloads now also carry `lastSeenAt` freshness
and optional metadata through the same owned contract, so the drawer can
preserve provenance without inventing a separate relationship-detail schema.
Relationship-change timeline entries now also use the canonical relationship
summary helper for their compact `from` and `to` wording, so the API keeps the
human-readable edge label aligned with the unified-resource relationship
presenter instead of reconstructing a local type-token summary.
The same `/api/resources/{id}/timeline` filter contract now also routes its
kinds, source types, and source adapters through the shared unified-resource
change-filter parser, so API validation stays owned by the change model rather
than being re-parsed separately in the HTTP handler.
The tenant-scoped unified resource API now also stays on canonical
unified-resource seeds end to end: `internal/api/resources.go`,
`internal/api/router_helpers.go`, and `internal/api/state_provider.go` no
longer treat raw tenant `StateSnapshot` data as a live registry-seeding owner
once `UnifiedResourceSnapshotForTenant` is available.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so non-default org resource list and
facet reads do not fall back to a missing-provider 500 during normal tenant
requests.
The unified agent settings surface now also follows an explicit shared
boundary with agent-lifecycle. Changes to
`frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx` must carry this
contract together with the shared agent-lifecycle contract and the dedicated API
proof files for token generation, agent lookup, and profile assignment, rather
than remaining an unowned consumer of those contract surfaces.
That shared `InfrastructureOperationsController.tsx` boundary must also stay under explicit proof
routing on both sides instead of relying only on generic owned-file coverage on
the API-contract side: token generation, agent lookup, and profile assignment
transport changes must continue to carry the direct
`unified-agent-settings-surface` proof path together with the lifecycle-side
surface proof.
The same shared-boundary rule now applies to `frontend-modern/src/api/agentProfiles.ts`,
`frontend-modern/src/api/nodes.ts`,
`frontend-modern/src/utils/agentInstallCommand.ts`,
`internal/api/agent_install_command_shared.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/unified_agent.go`:
agent install/register/profile control changes must preserve canonical API
payload behavior instead of drifting into subsystem-local transport rules.
That shared `frontend-modern/src/api/agentProfiles.ts` boundary must also stay
under explicit proof routing on both sides instead of remaining a generic
frontend-client match on the API-contract side: assignment, delete, unassign,
and suggestion transport changes must carry the direct profile-client proof
together with the lifecycle-side profile proof.
That shared `frontend-modern/src/api/nodes.ts` boundary must also stay under
explicit proof routing on both sides instead of remaining a generic
frontend-client match on the API-contract side: Proxmox setup-script and
agent-install command transport changes must carry the direct lifecycle/client
proof together with a direct API-contract client proof.
That same rule also applies to the shared update transport surface:
`frontend-modern/src/api/updates.ts` and `internal/api/updates.go` must carry a
direct API-contract proof path instead of relying only on the generic frontend
client or backend payload fallback coverage.
That same rule also applies to the shared security transport surface:
`frontend-modern/src/api/security.ts`, `internal/api/security.go`,
`internal/api/security_tokens.go`, and `internal/api/system_settings.go` must
carry a direct API-contract proof path instead of relying only on the generic
frontend client or backend payload fallback coverage.
That same rule now applies to the shared backend lifecycle install/register
surface as well: `internal/api/agent_install_command_shared.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/unified_agent.go`
must carry a direct API-contract proof path instead of relying only on the
generic `internal/api/` backend payload prefix.
That shared frontend install-command helper must also stay under explicit proof
routing instead of remaining an orphan utility: changes in
`frontend-modern/src/utils/agentInstallCommand.ts` must carry the direct
helper proof path, not rely only on downstream consumer tests to catch
transport drift.
That same backend install-command contract must also normalize trailing slashes
on canonical base URLs before composing installer asset paths or response
payloads, so `/api/agent-install-command` and the governed container-runtime
token response cannot emit `//install.sh` or slash-suffixed `pulseURL`
transport when `PublicURL` or `AgentConnectURL` already ends with `/`.
That same governed container-runtime migration response must also preserve the
canonical lifecycle shell payload shape: `installCommand` in the diagnostics
docker prepare-token response may not emit the stale `--disable-host` alias or
an ad hoc `curl | sudo bash` pipeline, and must instead match the canonical
root-or-sudo wrapped install transport with `--enable-host=false`.
That diagnostics install-command payload must also be assembled through the
shared backend install-command helper in `internal/api/agent_install_command_shared.go`
instead of a handler-local shell formatter, so token omission, plain-HTTP
`--insecure`, and trailing-slash normalization stay under one canonical API
contract surface.
That same diagnostics boundary must also consume the canonical monitoring
memory-source catalog instead of maintaining a second local trust/fallback
classifier. Node, VM, and LXC memory-source aliases must normalize to the same
governed labels and fallback-reason contract before diagnostics memory-source
breakdowns are serialized.
That same diagnostics boundary must also backfill canonical fallback reasons
when a raw snapshot reaches the API layer without one, so
`buildMemorySourceDiagnostics` stays self-consistent even if a caller bypasses
`GetDiagnosticSnapshots()` and hands diagnostics a legacy alias directly.
That shared `InfrastructureOperationsController.tsx` boundary now also preserves copied shell
command payload continuity: any privilege-escalation wrapper applied at the
settings surface must keep the full canonical installer argument list intact
instead of dropping token, profile, or command-execution flags between display
and clipboard transport.
That same shared `InfrastructureOperationsController.tsx` boundary now also consumes the canonical
`connectedInfrastructure` projection from the backend state contract instead of
reconstructing reporting rows by merging raw unified-resource facets and
removed-* arrays in the browser. v6 clients no longer receive those removed-*
arrays at all for this surface; Connected infrastructure row
identity, reporting-surface labels, and ignore/reconnect scope must be owned
by the backend payload contract, with frontend rendering limited to
presentation and operator actions.
That same install-command payload continuity now also applies when auth is
optional: copied install and upgrade commands must omit token arguments
entirely on token-optional Pulse instances rather than serializing a fake
sentinel token into the governed shell or PowerShell payload.
That same shared installer boundary must also stay on one runtime-argument
contract after the command is copied: `scripts/install.sh` may not rebuild
separate service-flag strings for token-bearing and token-file install paths,
and must instead derive persisted `--url`, optional `--token`, feature
toggles, identity flags, and disk-exclude transport from one canonical
installer-owned argument item list.
That same optional-auth contract now extends through the first governed
runtime transport boundary: post-install Unified Agent report requests and
Proxmox auto-register requests must use the canonical `authToken` request
field for one-time setup-token auth instead of any API-token auth header path,
so the canonical API surface does not preserve parallel auth transports or a
second auth meaning for the same field.
The self-hosted commercial entitlement payload now also uses one canonical
counted-unit contract: `max_monitored_systems` is the live runtime and
frontend term, and older `max_agents` or `max_nodes` aliases may be decoded
only at explicit legacy import boundaries. Limit `current` values, add-node
enforcement, auto-register enforcement, deploy-slot enforcement, the
monitored-system ledger endpoint, and TrueNAS/API-backed registration must all
reflect deduped top-level monitored systems rather than agent-only
installation count, and `legacy_connections` / `has_migration_gap` may not
imply that API-backed monitoring sits outside the commercial cap.
That same configured-path contract now also has an explicit shared owner for
manual auth env files: `internal/api/auth_env_path.go` must remain the only
place that derives `.env` from configured runtime paths, and neighboring
handlers like `router.go`, `router_routes_auth_security.go`, and
`security_setup_fix.go` may not reconstruct their own `/etc/pulse/.env`
fallbacks once runtime path authority has been centralized.
That same shared API boundary rule now also applies to notification test
handlers: `internal/api/notifications.go` may decode webhook-test requests and
return the governed response envelope, but notifications-owned service-template
selection, safe header copying, and generic webhook-test payload fallback must
stay in `internal/notifications/` rather than becoming a second API-layer owner
for the same transport contract.
The notifications API boundary also carries the canonical webhook template
shape used by the frontend service chooser: `frontend-modern/src/api/notifications.ts`
must expose the registry's service label, description, and mention-copy
metadata, and it may not invent a second frontend-only service taxonomy for
the chooser.
That same notifications boundary must also canonicalize legacy service-specific
input aliases at ingress instead of leaving them as a live runtime contract:
Pushover `app_token` / `user_token` may be accepted only at config/API/UI input
boundaries, and API responses plus live notification runtime state must carry
only canonical `token` / `user` fields.
That same shared owner now also governs writable auth env target order:
setup, password-change, and auth-status flows must route `.env` writes through
the shared helper instead of open-coding config-path writes plus ad hoc
data-path fallback branches in each handler.
Those shared profile-assignment settings surfaces must also preserve canonical
assignment visibility when an assignment references a profile ID that no longer
resolves in the fetched profile collection: the current payload state must stay
visible to the operator instead of collapsing into an empty/default select
value that misstates the backend assignment.
That same shared install-command boundary must preserve selected Proxmox target
profiles across PowerShell transport: `InfrastructureOperationsController.tsx` must emit
`PULSE_ENABLE_PROXMOX` and `PULSE_PROXMOX_TYPE` when the operator copies a
Windows install command for a Proxmox-targeted flow, and `scripts/install.ps1`
must convert those env vars back into canonical `pulse-agent` service args so
the copied payload does not drift from the governed shell command contract.
That same shared PowerShell install transport must also preserve
operator-selected insecure TLS and command-execution settings: copied Windows
install and upgrade payloads must emit `PULSE_INSECURE_SKIP_VERIFY` and
`PULSE_ENABLE_COMMANDS` when enabled, and copied Windows uninstall payloads
must still emit `PULSE_INSECURE_SKIP_VERIFY` when enabled, so
`scripts/install.ps1` does not silently drop self-signed transport intent on
the Windows path.
That same shared lifecycle transport must also preserve explicit custom CA
selection end to end: copied shell install, upgrade, and uninstall payloads
must pass `--cacert` to both the outer installer download and the governed
installer runtime, while copied Windows install, upgrade, and uninstall
payloads must emit `PULSE_CACERT` and use a PowerShell bootstrap that applies
custom-CA or insecure-TLS certificate handling before `install.ps1` is fetched,
not only after the installer starts executing. That bootstrap must accept the
same PEM/CRT/CER trust input that `scripts/install.ps1` itself accepts, so the
shared command contract does not narrow custom-CA behavior on the first fetch.
That same shell transport contract also applies to the governed setup-completion
install handoff in `SetupCompletionPanel`: when the operator supplies a custom CA path
or opts into insecure/self-signed transport, the shared Unix install builder
must carry those choices through both the outer `curl` fetch and the installer
runtime instead of leaving the first-session onboarding path behind the shared
lifecycle/API contract. For explicit insecure/self-signed mode, that first-hop
fetch must widen to `curl -kfsSL`; preserving `--insecure` only on the later
installer runtime is not sufficient.
That same shared lifecycle/API boundary must also keep setup-script bootstrap
transport under one owned backend shape: `/api/setup-script-url` response
payloads and `/api/setup-script` rerun guidance must derive URL, download URL,
file name, token hint, and env/non-env command variants from one canonical
bootstrap artifact builder instead of duplicating those fields in separate
handler-local payload assembly paths.
That same owned setup-script contract now also covers the rendered shell body:
PVE and PBS script text must come from shared backend render helpers instead of
remaining duplicated inside the setup handler, so the API boundary owns one
artifact contract plus one render path rather than a route-local script engine.
That owned backend shape must itself stay singular: the shared setup artifact
model is the API contract, and handler-local response structs may not mirror
or remap the same `url`, `downloadURL`, `scriptFileName`, command, expiry, and
token metadata in parallel.
That same setup-completion contract must also preserve the canonical agent-connect
URL boundary: first-session install commands must prefer the backend-governed
security status `agentUrl` and only fall back to browser origin when no
canonical agent endpoint exists, while still allowing a local override for
bootstrap cases where the operator needs a different agent-to-Pulse address.
That same shared first-session install contract also applies to Windows
transport: `SetupCompletionPanel` must expose a governed PowerShell install command and
route it through the shared lifecycle helper, so `PULSE_URL`, optional
`PULSE_TOKEN`, insecure/self-signed TLS handling, and `PULSE_CACERT` stay
identical to the Windows install payload contract already enforced in
`InfrastructureOperationsController.tsx`.
That same first-session install boundary must also preserve the shared
optional-auth command contract: the Unix install builder must support omitted
`--token` transport, and `SetupCompletionPanel` may only omit that argument after an
explicit "without token" confirmation when auth is optional, while preserving
the generated token path by default so onboarding does not drift from the
governed settings behavior. After that explicit tokenless confirmation,
repeated wizard copy actions must keep emitting tokenless payloads instead of
silently rotating back to `PULSE_TOKEN` or `--token` transport on the next
rendered command. The same rule applies to wizard-owned background token
rotation: agent-connection polling may not regenerate a token or restore
token-auth payloads while explicit tokenless onboarding is still the active
contract.
That same first-session token contract must also stay coherent across the
setup-completion credential surfaces: once `SetupCompletionPanel` rotates the active install
token, the displayed credential token and downloaded credentials payload must
emit that same current token instead of exporting the stale bootstrap token
while the copied install command already uses a different one. At the same
time, the stable bootstrap admin API token must remain separately visible and
copyable; the setup wizard may not replace the admin credential with the
rotating install token and call that payload contract complete. That same
exported credentials payload must also carry the current agent-install URL and
matching install command contract for both Unix and Windows transport,
including any operator override, instead of serializing only browser-local
login context or Unix-only onboarding while the live setup-completion install
surface has already switched to a different governed endpoint. When explicit
tokenless optional-auth mode is active, the same payload and drawer contract
must report tokenless install mode instead of serializing a misleading current
install token that is no longer part of the active command transport, and the
operator guidance text on the install surface must stop claiming automatic
token rotation after each copy while tokenless transport is active.
That same insecure-TLS contract also applies to installer-owned HTTP traffic:
when `PULSE_INSECURE_SKIP_VERIFY` is set, `scripts/install.ps1` must use the
same relaxed certificate policy for the governed binary download and uninstall
API callback requests instead of preserving `--insecure` only for the later
agent runtime.
That same shared `InfrastructureOperationsController.tsx` boundary must also preserve
platform-canonical uninstall command payloads: copied utility actions for
Windows agents must emit the PowerShell uninstall transport, and uninstall
payloads must only carry real API token secrets rather than token record IDs
when server-side deregistration is requested.
That same uninstall payload rule now also applies to copied Unix shell flows:
`InfrastructureOperationsController.tsx` must never serialize a token record ID into the governed
`--token` argument when building uninstall transport, because the backend
runtime only accepts the raw token secret or no token at all.
The same shared uninstall transport must preserve `PULSE_URL` for token-optional
Windows flows, because `install.ps1` reads its canonical server endpoint from
that environment variable when composing the governed uninstall request.
That same copied uninstall boundary must also preserve the selected agent's
canonical identity when inventory already has it: shell uninstall payloads must
carry `--agent-id`, and PowerShell uninstall payloads must carry
`PULSE_AGENT_ID`, so deregistration targets the intended governed agent record
instead of depending on local fallback files or hostname lookup.
The same identity-preservation contract applies to copied upgrade transport:
shell upgrade payloads must carry `--agent-id` and `--hostname`, and
PowerShell upgrade payloads must carry `PULSE_AGENT_ID` and `PULSE_HOSTNAME`,
so upgrade reruns stay bound to the selected governed inventory record.
That same Unix transport boundary must also preserve shell-safe argument
encoding: copied shell uninstall and upgrade payloads must quote canonical URL,
token, agent ID, and hostname arguments so governed lifecycle commands do not
break or reinterpret inventory values with shell-significant characters.
The same Windows transport boundary must also preserve PowerShell-safe argument
encoding: copied PowerShell uninstall and upgrade payloads must escape
canonical URL, token, agent ID, and hostname values before they enter env
assignments or `irm` command text, and the copied Windows upgrade payload must
quote the resolved script URL so canonical URLs containing spaces remain a
valid PowerShell transport. The same Windows uninstall payload must quote its
resolved script URL too; escaping `PULSE_URL` into env assignments is not
sufficient if the later `install.ps1` invocation can still be split by
PowerShell parsing.
That same install-command boundary must use the identical escaping rules:
copied shell install payloads must quote canonical URL/token arguments, and
copied PowerShell install payloads must escape canonical URL/token values
before they enter env assignments or `irm` transport. The same interactive
Windows install snippet must also export `PULSE_URL` explicitly when copying a
selected canonical agent address, not just the fully qualified `install.ps1`
download URL.
That same shared install payload contract must also normalize trailing slashes
on canonical Pulse URLs before composing installer asset paths, so copied shell
and PowerShell install transport cannot drift onto `//install.sh` or
`//install.ps1` when operators paste a base URL that already ends with `/`.
When a governed token is already selected, that same interactive Windows
install payload must carry `PULSE_TOKEN` too; the copied command may not discard
the chosen credential and regress to a second manual prompt while other
install/uninstall/upgrade payloads stay token-bound.
When no real token has been selected yet, that same interactive Windows payload
must not serialize a placeholder token into `PULSE_TOKEN`; the contract remains
prompt-driven until a governed credential actually exists.
That optional-auth install contract must also remain bidirectional: when Pulse
allows tokenless transport, the settings surface may omit `PULSE_TOKEN` after a
real "without token" confirmation, but it must still preserve a real generated
token if the operator chooses one instead of collapsing optional auth into a
tokenless-only command builder.
That same optional-auth payload rule now also covers backend-generated Proxmox
install responses: when auth is not configured, the canonical
agent-install-command API must omit `token` and `--token` from its payload
instead of implicitly persisting a new API token record and mutating the
server's auth-configured state just to render a backend-driven install
command.
The same uninstall contract applies to hostname fallback identity: shell
payloads must carry `--hostname`, PowerShell payloads must carry
`PULSE_HOSTNAME`, and the uninstall scripts must prefer that explicit hostname
when performing governed `/api/agents/agent/lookup` fallback. That lookup must
fail closed on ambiguous hostname matches: installer-driven recovery may only
resolve a hostname when the match is unique, and display-name or short-hostname
fallbacks must return not found rather than picking an arbitrary agent.
That lookup fallback transport must be canonicalized on both installer paths:
shell and PowerShell uninstall flows must percent-encode the selected hostname
before issuing `/api/agents/agent/lookup`, so API-owned identity recovery does
not depend on raw query interpolation.
The same shell uninstall contract also applies to persisted connection state:
when `scripts/install.sh` receives explicit `--agent-id` or `--hostname`, it
must store those values alongside URL/token in `connection.env` and recover
them before invoking governed uninstall fallback.
The same persisted-identity contract applies to `scripts/install.ps1`: Windows
install and upgrade must store URL, token, agent ID, and hostname continuity in
installer-owned state and reload those values during governed uninstall before
using local fallback files or hostname discovery.
That ProgramData continuity state is scoped to the live installation only:
after governed uninstall succeeds, `scripts/install.ps1` must remove the saved
state so stale agent identity or transport metadata cannot leak into later
removal or reinstall flows.
The same persisted-state contract applies to self-signed transport continuity:
canonical installer-owned uninstall state must retain insecure TLS intent and
reload it during governed offline uninstall, so self-signed Pulse instances do
not lose deregistration reachability after the original clipboard command.
That same persisted shell uninstall state must retain `--cacert` continuity:
`scripts/install.sh` must store and recover the custom CA bundle path from
`connection.env` so governed lookup and uninstall calls continue to trust the
intended Pulse certificate chain offline.
That shell `connection.env` recovery contract is keyed to partial uninstall
context, not only an entirely missing URL/token pair: if any governed uninstall
identity or transport field is absent on the command line, the script must
reload the missing persisted continuity before using API-owned lookup fallback.
Those register/install control surfaces now also carry a canonical host
identity continuity contract: `/api/auto-register` and token reuse must treat
hostname-form and IP-form URLs for the same node as one API-owned identity so
reruns do not fork duplicate runtime records or shadow token payloads.
That canonical `/api/auto-register` payload must also preserve token-action
truth: canonical completion now requires caller-supplied `tokenId` and
`tokenValue`, and the response must stay on the direct-use
`action="use_token"` contract as the only supported completion path.
That same contract must be enforced by first-hop callers too: install and
runtime-side Unified Agent registration clients may not treat a bare 2xx response or a loose
`status` field as success; they must validate the canonical `status`,
`action`, and token/identity response shape.
That same canonical `/api/auto-register` contract must also accept caller-supplied
Proxmox token completion directly on that contract: when a runtime-side Unified Agent or
generated flow already created the canonical token locally, the request may
carry `tokenId` and `tokenValue`, and the response must stay on the direct-use
`action="use_token"` contract as the only supported completion path.
That same runtime transport contract also governs the agent-ingest boundary in
`internal/api/agent_ingest.go` and `internal/api/router*.go`: the primary
request/response surface is the Pulse Unified Agent route family, while
`/api/agents/host/*` stays a compatibility alias and must not leak back into
handler naming, router-owned state, or proof labels as if it were a second
product-facing API surface.
That confirmation marker must survive the legacy setup-script transport too:
script-generated `/api/auto-register` payloads must send `source="script"`,
and canonical callers must send that source explicitly, so later canonical
reruns can distinguish real confirmed credentials from agent-created tokens.
That same `/api/auto-register` request contract must also reject non-canonical
source values outright: only `source="agent"` and `source="script"` are valid,
so the backend does not preserve arbitrary caller labels as accidental API
surface.
That same `/api/auto-register` request contract must also reject non-canonical
node types outright: only `type="pve"` and `type="pbs"` are valid, so the
backend does not complete unsupported runtime labels as fake successful
registrations.
That same `/api/auto-register` request contract must also reject non-canonical
token identities outright: `tokenId` must be a Pulse-managed canonical
identifier in the form `pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>`
matching the requested node type, so the backend does not preserve arbitrary,
cross-type, or non-Pulse-managed token IDs as accidental API surface.
That same caller-supplied token contract must also stay deterministic across
the live registration clients: installer, setup-script, and runtime-side Unified Agent Proxmox
flows must converge on the same Pulse-managed `pulse-<canonical-scope-slug>`
token name for the same Pulse endpoint instead of serializing caller-local
timestamp variants into the canonical `/api/auto-register` payload.
That same deterministic token-name contract also governs backend turnkey
credential setup: the password-based PBS add-node flow and generated
setup-script payloads must derive Pulse-managed token names from the canonical
Pulse endpoint itself rather than request-local `Host` fallbacks, so loopback
or proxy-facing admin requests cannot fork the token scope for the same Pulse
instance.
That same generated setup-script payload must now also opt into the canonical
registration contract explicitly: locally created Proxmox token completions
must send `tokenId` and `tokenValue` as the canonical request shape.
That same request contract must also accept one-time setup-token auth through
`authToken` only, so `/api/auto-register` does not keep a duplicate
`setupCode` payload alias alongside the canonical field.
That same shared discovery transport surface must also keep structured error
ownership in the runtime model: `pkg/discovery` and `internal/discovery` own
`structured_errors`, while `internal/api/config_discovery_handlers.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/config_node_handlers.go`
may derive the deprecated `errors` string list only as a compatibility field
at the API and WebSocket boundary.
That same WebSocket state boundary must also stay tenant-aware by construction:
`internal/websocket` may not keep a separate default-org state getter beside the
tenant-aware state path, and default-org snapshots must flow through the same
`org_id="default"` contract used for non-default organizations.
That same canonical auth contract must also keep its runtime and user-facing
terminology on setup tokens: active `/api/auto-register` auth failures and the
owning handler/proof names may not drift back to setup-code wording after the
payload contract has been canonicalized.
That same first-session security boundary also governs bootstrap-token
persistence and retrieval: the one-time setup secret may remain recoverable
through the supported `pulse bootstrap-token` command, but `.bootstrap_token`
may not remain a raw plaintext secret file on disk. Canonical runtime
persistence must encrypt that token at rest and rewrite any legacy plaintext
bootstrap-token file immediately into the encrypted canonical format on load.
That same setup-token-only contract must also keep missing-token failures
specific: `/api/auto-register` may not answer a missing `authToken` request
with a generic authentication error after the route has been narrowed to the
setup-token flow.
That same canonical request contract must also keep field-validation failures
specific: mismatched `tokenId`/`tokenValue` input may not collapse into
generic missing-field output, and other missing canonical fields must return
explicit `Missing required canonical auto-register fields: ...` guidance.
That same request/validation contract must stay coherent across both entry
points on the canonical runtime surface: the public `/api/auto-register`
handler and the direct canonical handler path may not drift onto different
messages for the same missing-field or token-pair failures.
That same canonical request contract must also require an explicit
`serverName` field from live callers rather than synthesizing node identity
from `host` inside the backend.
That same canonical backend contract must also keep overlap-continuity runtime
messages on canonical `/api/auto-register` wording: the helper/log surface for
resolved-host matches, DHCP continuity matches, and in-place token updates may
not preserve the deleted "secure auto-register" split.
That same canonical runtime path must keep token-completion validation wording
on the canonical contract too: incomplete `tokenId`/`tokenValue` payloads may
not preserve deleted "secure token completion" wording in live handler
messages.
That same canonical request contract also governs runtime-side Unified Agent-initiated
Proxmox completion: callers must fetch and use a one-time setup token in
`authToken` instead of carrying long-lived admin authentication directly on
`/api/auto-register`.
That same canonical caller-supplied completion request shape also governs `scripts/install.sh`:
installer-owned Proxmox auto-registration must submit local token creation
results with `tokenId` and `tokenValue` on the canonical `/api/auto-register` contract instead
of emitting any alternate payload shape.
The unified-agent uninstall command contract must also fail closed on
token-required Pulse instances: copied shell and PowerShell uninstall payloads
must use the same resolved token source as install and upgrade, so required
auth cannot silently collapse into tokenless deregistration transport.
Agent profile assignment payloads now also fail closed on missing profiles:
`POST /api/admin/profiles/assignments` must reject unknown `profile_id`
references with the canonical not-found response instead of writing orphan
assignment rows that no governed UI can represent.
That same not-found assignment contract must propagate through the shared
frontend client path: `frontend-modern/src/api/agentProfiles.ts` must surface
the canonical missing-profile message for 404 assignment responses, and the
settings profile surfaces in `AgentProfilesPanel.tsx` and `InfrastructureOperationsController.tsx`
must treat that message as a resync trigger so stale profile options do not
survive after the backend has already rejected them.
That same shared response contract must also fail closed on malformed list
payloads: the profile-management client may not treat non-array profile or
assignment responses as empty collections, and `AgentProfilesPanel.tsx` /
`InfrastructureOperationsController.tsx` must surface the resulting load failure instead of
flattening it into a fake zero-profile state.
That same shared response contract must also fail closed on malformed
profile-object, suggestion, schema, and validation payloads: the
profile-management client may not accept partial profile objects, malformed
schema definitions, or malformed validation/suggestion bodies as successful
contract responses, and the profile editor plus suggestion modal must surface
those canonical response failures instead of collapsing them into generic
save/delete/schema/validation fallback messaging.
The canonical Proxmox auto-register contract must also preserve legacy DHCP
continuity semantics: when `/api/auto-register` receives the same
canonical node name together with the deterministic Pulse-managed token ID for
that node, it must update the existing PVE or PBS entry in place even if the
host IP has changed, rather than duplicating the node under a second endpoint.
The unified-agent install endpoints now also carry an exact-release fallback
contract: when `/install.sh` or `/install.ps1` cannot be served locally, the
backend must proxy the install script asset from the exact GitHub release that
matches `serverVersion` and must fail closed for dev or unreleased builds
rather than serving branch-tip installer logic.
The `/api/updates/plan` contract must also fail closed without becoming a
transport error on supported non-auto-update deployments: `manual`,
`development`, and `source` runtimes must return an explicit manual update
plan payload instead of `404 No updater for deployment type`, so first-session
and settings surfaces do not treat valid deployment modes as broken update
transport.
Those same install-command payloads now also carry a non-TLS continuity
contract: when Pulse returns a plain `http://` base URL for a generated agent
install command, the command must include `--insecure` so the installed agent
keeps its update path alive on lab or self-hosted targets instead of silently
skipping updater checks after the first install.
The same plain-HTTP continuity rule applies to governed frontend-generated
host install transport too: shared Unix install command builders must append
`--insecure` for `http://` Pulse URLs so setup-completion copies cannot drift from
the lifecycle contract already enforced in the unified settings surface.
That same frontend install-command contract must also fail closed on blank
local overrides: whitespace-only custom Pulse endpoint input in
`InfrastructureOperationsController.tsx` or `SetupCompletionPanel.tsx` may not override the canonical
backend-governed endpoint, and the shared install-command helper must reject
blank base URLs instead of composing installer script paths from an empty
transport root.
That same install-command payload contract also covers backend-generated
Proxmox install responses in `internal/api/agent_install_command_shared.go`:
the `/api/agent-install-command` payload and hosted tenant Proxmox install
payload must emit the same root-or-sudo Unix wrapper contract as the governed
frontend builder, rather than exposing a stale raw `| bash -s --` transport
shape through the API surface.
That same rule applies to the unified settings shell lifecycle copies:
frontend-generated Unix install and upgrade commands must append `--insecure`
for `http://` Pulse URLs automatically, while only the explicit insecure-TLS
toggle may widen curl transport itself to `-k`.
That same unified settings install boundary must also preserve preview/copy
parity: the rendered Linux/macOS/BSD and Windows install snippets in
`InfrastructureOperationsController.tsx` must already reflect the active token contract, custom-CA
transport, insecure/plain-HTTP behavior, install-profile env/flags, and
command-execution mode, rather than showing a stale base command that is only
rewritten at copy time.
The loopback-originated install and setup payloads now also preserve the full
configured `PublicURL` when that URL is the canonical external route, instead
of rewriting only the host and inheriting an `http://` request-local scheme
that would drift the generated command away from the governed public endpoint.
The canonical frontend client contract for Proxmox setup transport now also
applies to `/api/setup-script-url` and `/api/setup-script`: governed settings
surfaces must request quick-setup commands and manual setup-script downloads
through shared `frontend-modern/src/api/nodes.ts` helpers for both `type:"pve"`
and `type:"pbs"`, preserving the runtime-owned bootstrap artifact metadata
instead of open-coding one node type onto raw fetch branches.
That same `/api/setup-script-url` response contract must now also preserve the
canonical bootstrap identity explicitly through returned `type` and normalized
`host`, and the handler must reject missing or unsupported `type`/`host`
input instead of minting open-ended setup tokens with caller-local host
formatting.
That same setup-script-url boundary must keep a strict request shape too: the
handler accepts one canonical JSON object only, and unknown fields or trailing
JSON must fail closed as invalid request shape instead of being ignored as
forward-compatible extras.
That same bootstrap request boundary must also keep `backupPerms` truthful:
the flag is part of the canonical PVE setup contract only, so `/api/setup-script`
and `/api/setup-script-url` must reject it for `type:"pbs"` instead of
silently accepting a transport-level no-op.
That same setup bootstrap contract also keeps host identity explicit across
both routes: `/api/setup-script` and `/api/setup-script-url` must reject
missing `host` input instead of issuing placeholder-host artifacts that only
fail later during execution.
That same request boundary must also keep canonical type and host handling
aligned across both setup routes: `/api/setup-script` may not treat unknown
`type` values as implicit PBS requests, and it must normalize the supplied
host before rendering script text so returned artifacts and rerun URLs preserve
the same canonical node identity as `/api/setup-script-url`.
That same setup bootstrap contract also keeps Pulse identity explicit across
both routes: `/api/setup-script` may not derive `pulse_url` from the request
origin once `/api/setup-script-url` is already returning canonical Pulse URL
metadata, and missing `pulse_url` input must fail closed instead of silently
forking the bootstrap surface onto request-local origin state.
That same canonical bootstrap response shape must also stay enforced by the
shared frontend setup client in `frontend-modern/src/api/nodes.ts`, so
settings-owned quick-setup flows fail closed on malformed `type`, `host`,
`url`, `downloadURL`, `command`, `setupToken`, `tokenHint`, or `expires`
fields instead of passing raw backend JSON deeper into lane-local UI state. That shared client
must validate the returned `setupToken` but may not expose or retain it once
the operator-facing surface only needs the runtime-owned bootstrap artifact
plus masked `tokenHint`.
That frontend bootstrap consumer must also treat `expires` as a live-expiry
field, not merely a positive number, so expired setup-script-url responses are
rejected before quick-setup UI state or copy actions trust the returned setup
token.
That same settings quick-setup surface must consume the canonicalized response
directly: `NodeModal.tsx` must copy the governed token-bearing
`commandWithEnv` field but render `commandWithoutEnv` as the visible preview,
using the guaranteed `expires` value without reintroducing module-local
nullable fallbacks. The same shared surface must
also treat `setupToken` as bootstrap transport data and `tokenHint` as the
operator-facing display field, so the UI does not re-expose the full one-time
token once the copied/downloaded artifact already carries it. That preview
secrecy rule must stay symmetric across both supported Proxmox types, so the
PBS quick-setup branch may not preserve the token-bearing preview after the
PVE branch has moved to the governed `commandWithoutEnv` display contract.
That same quick-setup guidance must also stay truthful after the preview is
masked: copy-success messaging may not tell the operator to paste a token
"shown below" once only `tokenHint` remains visible, and stale raw-token
cleanup paths may not survive in one Proxmox branch after the shared UI state
has moved to hint-only handling.
That same shared frontend setup surface must also trim and validate the
canonical `host` input before invoking `/api/setup-script` downloads, and the
shared `frontend-modern/src/api/nodes.ts` helper must reject empty `host` or
`pulseUrl` inputs instead of serializing whitespace-corrupted query params.
That same `/api/setup-script` payload contract must also stay explicit at the
artifact boundary: successful responses are shell-script downloads with
canonical `text/x-shellscript` content type plus an attachment filename, and
the shared `frontend-modern/src/api/nodes.ts` client must reject malformed
download headers instead of flattening script delivery into an untyped text
blob.
That same setup bootstrap contract must also keep manual download
non-interactive without depending on a separately rendered secret: the
setup-script-url payload must return a token-bearing `downloadURL`, and the
shared frontend client must fetch setup scripts through that field instead of
reusing the plain script `url` that omits the setup token.
That same shared frontend setup surface must also treat
`/api/setup-script-url` as the canonical bootstrap artifact source for the
current host/type/mode: quick-setup copy and manual script download must reuse
the returned `url`, `downloadURL`, `scriptFileName`, `commandWithEnv`,
`tokenHint`, and `expires` until that artifact expires or the operator changes
the endpoint, instead of rebuilding a second download request from lane-local
form state or retaining the raw setup token inside frontend cache state.
That same bootstrap artifact contract must also stay coherent in public-facing
guidance: `docs/API.md` and operator setup guides may not describe
`/api/setup-script-url` as if it only returned a token plus bare URL, and they
may not publish stale `curl -sSL ... | bash` setup examples after the runtime
and settings surfaces have standardized on the returned canonical `command*`
fields.
That same setup-script-url payload contract must also return the canonical
setup-script filename as `scriptFileName`, and the shared settings/bootstrap
consumer may not hardcode separate script names for PVE or PBS once the
runtime-owned filename is available.
That same setup-script-url payload must remain a coherent bootstrap artifact
envelope for all live consumers, not only the frontend: `url`,
`downloadURL`, `scriptFileName`, `command`, `commandWithEnv`,
`commandWithoutEnv`, and masked `tokenHint` are part of the canonical response
shape, and runtime-side Unified Agent/installer consumers must fail closed when those fields
are missing or mismatched instead of silently treating the response as
setup-token-only.
That same consumer contract must also treat `expires` as a live-expiry field,
not merely a populated one: installer and runtime-side Unified Agent callers must reject
bootstrap responses whose returned expiry timestamp is already in the past.
That same setup-script-url auth boundary must stay explicit too: returned
`setupToken` values bootstrap `/api/setup-script` and `/api/auto-register`,
but they do not authenticate the `/api/setup-script-url` request itself once
Pulse auth is configured.
That same setup-script-url payload contract now also fixes the shell transport
it returns: the `command`, `commandWithEnv`, and `commandWithoutEnv` fields
must use shell-quoted `curl -fsSL` fetches assembled through a shared backend
helper rather than a handler-local `curl -sSL` pipeline.
Those returned setup-script command fields must also preserve the governed
root-or-sudo execution contract, including carrying `PULSE_SETUP_TOKEN`
through the sudo path when present instead of assuming direct-root execution.
That same setup-script contract now also covers the generated script text:
operator guidance embedded in `/api/setup-script` responses must keep the same
fail-fast `curl -fsSL` fetch wording for retry and missing-host examples
instead of returning stale `curl -sSL` transport in the script payload.
That embedded guidance must also advertise the same root-or-sudo execution
shape as the API-returned quick-setup command instead of drifting onto a
direct-root-only `| bash` retry path inside the script payload.
That same script-payload guidance must preserve `PULSE_SETUP_TOKEN` across
those retry examples too, so the generated script text does not drop the
non-interactive setup-token contract even when it preserves the shell wrapper.
That same generated-script payload must also hydrate `PULSE_SETUP_TOKEN` from
an embedded setup token before those rerun examples are shown, so canonical
`setup_token`-issued scripts keep the same non-interactive contract on the
next hop instead of silently reverting to a prompt.
That same `/api/setup-script` boundary must keep one token name too: embedded
bootstrap uses only the `setup_token` query, and the rendered setup script body
uses only `PULSE_SETUP_TOKEN` rather than keeping `AUTH_TOKEN` or
`SETUP_AUTH_TOKEN` compatibility aliases alive.
That same generated-script payload must also remove discovered legacy tokens
from the concrete `pve` and `pam` token lists it already enumerated, rather
than iterating an undefined shell variable and silently turning operator-chosen
cleanup into a no-op.
That same generated-script payload must also preserve the canonical encoded
rerun URL contract: embedded `SETUP_SCRIPT_URL` values must carry the exact
selected `host`, `pulse_url`, and `backup_perms` query state instead of
reconstructing a raw query string inside the shell.
That same off-host branch may not advertise a second manual `pveum` token
creation contract either; when the runtime lacks Proxmox host tooling, the
payload must direct operators back to rerun on the host through the canonical
generated command instead of inventing a separate Pulse Settings token-entry
workflow.
That same script payload must also preserve canonical privilege-error wording
for direct execution: the generated runtime may not regress to the stale
"Please run this script as root" string and must instead use the same root
requirement language as the governed retry examples.
That same manual-add payload must also preserve one canonical token placeholder
string when the script cannot echo the secret again from process state, rather
than drifting across neighboring branches with lane-local variants like
"[See above]" or "Check the output above...".
That same payload must also preserve one canonical success-message contract
across generated PVE and PBS scripts, rather than returning node-type-specific
phrasing for the same successful auto-register result.
That same setup-script payload must also discover legacy cleanup candidates
through the canonical Pulse-managed token prefix for the active Pulse URL,
while still matching legacy timestamp-suffixed variants, instead of rebuilding
an IP-derived regex that can drift from `buildPulseMonitorTokenName`.
That same cleanup-discovery contract applies to both generated PVE and PBS
setup-script payloads; node type may not fork onto different legacy token-name
matching rules for the same Pulse-managed token surface.
That same payload must also use exact token-name matching for rerun rotation
detection, rather than broad substring checks over token-list output, so the
canonical managed token contract does not collide with unrelated partial-name
matches.
That same payload must also keep PBS token-copy guidance truthful: the
one-time token banner may only be emitted from the successful token-create
branch, not before the creation result is known.
That same payload must also keep PBS auto-register attempt guidance truthful:
the generated script may only print its attempt banner on the branch that is
actually about to send the registration request, not before token-unavailable
or missing-auth skip handling.
That same payload must also fail closed when token creation output does not
yield a usable token value: the generated script may not continue into prompt
or request assembly with an empty token secret, and must instead stop on the
canonical token-value-unavailable branch before any registration POST is built.
That same setup-script payload must also fail closed on auto-register success
parsing: the generated script may not treat any bare `success` substring as a
successful response, and must instead require an explicit `success:true`
signal before claiming registration succeeded.
That same payload contract must also fail closed on auto-register transport:
the generated script must use fail-fast `curl -fsS` request transport and only
evaluate the response payload after a successful curl exit status, rather than
parsing ambiguous stderr or HTTP-failure output as a valid registration body.
That same setup-script payload must also preserve the canonical auth guidance:
authentication failures in the generated script text must reference the active
Pulse setup-token flow, not stale API-token setup instructions, because the
payload now authenticates auto-register through one-time setup tokens.
That same auth-failure payload must also stay truthful after a request attempt:
once the generated script has already entered the registration-request path,
it may not fall back to a missing-token explanation and must instead report
that the provided setup token was invalid or expired, directing the operator
to fetch a fresh setup token from Pulse Settings → Nodes and rerun. The final
completion/footer path must honor that same auth-failure state instead of
reopening manual completion with the emitted token details.
That same payload must also preserve truthful completion messaging: generated
setup-script text may only announce successful Pulse registration when the
payload's auto-register branch succeeded, and must otherwise describe the
result as manual follow-up using the emitted token details.
That same manual-follow-up payload may not advertise a stale `PULSE_REG_TOKEN`
rerun contract: when auto-register falls back to manual completion, the script
text must direct the operator to Pulse Settings → Nodes with the emitted token
details rather than inventing a second registration-token flow.
That same manual-follow-up payload must also keep its failure-summary text on
that same canonical completion path: the generated script may not fall back to
vague "manual configuration may be needed" wording when it already knows the
operator should finish registration through Pulse Settings → Nodes with the
emitted token details.
That same immediate failure path may not fork into a separate numbered manual
setup list either; it must point directly at the same token-details-below
Settings → Nodes completion contract used by the final manual footer, including
the branch where the registration POST itself fails before a response payload
can be parsed.
That same manual-follow-up payload must also preserve the canonical host value
already carried by the script payload, instead of reverting to a placeholder
host string in the rendered manual-add instructions.
That same host-continuity contract also applies to generated PBS scripts: the
manual-add footer must preserve the canonical `host` payload value instead of
replacing it with a runtime-discovered local IP that may not match the API
contract the caller requested.
That same PBS payload contract must also bind the canonical `host` before any
setup-token gating that can skip auto-registration, so manual fallback output
cannot lose the host URL when the operator does not provide a setup token.
That same host binding must also precede token-creation failure fallback, so
the rendered manual footer still carries the canonical `host` payload even
when the script fails before any auto-register request can be assembled.
If the caller never supplied a canonical `host` at all, the rendered script
must fail closed instead of surfacing placeholder host values as manual
registration targets; it must direct the caller to regenerate the setup script
with a valid host URL.
That same payload must also preserve token-creation failure truth: when
Proxmox token minting fails, the rendered script may not emit placeholder token
details or report token setup completed. It must keep the host binding, skip
auto-register assembly, and tell the caller to rerun after the token-creation
error is fixed.
That same payload must also preserve token-extraction failure truth: if the
returned token output does not yield a usable token secret, the script may not
advertise manual registration as a fallback path from that broken payload and
must instead direct the caller to rerun after the token output issue is fixed.
Rendered completion and manual-detail payload branches must treat only an
extractable token secret as ready; token-create success alone is not enough.
That same rendered PBS payload must also distinguish skipped auto-register
states from attempted request failures, so missing setup-token input or missing
usable token secret cannot surface the generic request-failed-before-success
banner.
That same payload must also preserve canonical manual-completion phrasing
across generated PVE and PBS scripts: both must use the Settings → Nodes
manual-add language instead of diverging onto node-type-specific fallback
headings that imply different completion paths.
That same generated payload may not shorten the earlier auto-register failure
branch back to plain "Pulse Settings" wording either; both the immediate
failure guidance and the final manual footer must preserve the same Settings →
Nodes completion destination.
`/api/charts/workloads-summary` now also has a canonical hot-path invariant:
aggregate workload charts must preserve stable guest counts while batching
store-backed metric reads across workload types, with no payload shape change.
That endpoint now also carries an explicit API p95 budget under the same
store-backed mixed-workload fixture used to verify the batched hot path.
Frontend AI API clients now also normalize `402 Payment Required` responses for
optional paywalled collections into explicit empty states, so Pulse Pro gating
does not become a transport error path during page bootstrap.
That frontend status handling must now route through the shared
`frontend-modern/src/api/responseUtils.ts` status helpers rather than through
message-text heuristics in individual API modules.
Optional not-found response handling in frontend API clients must now also use
those shared response-status helpers rather than open-coded `response.status`
branches in each module.
The same rule now applies to no-content and service-unavailable handling in
governed frontend API clients.
Governed frontend API clients must now also route required and safe success
payload parsing through the shared response parsing helpers rather than through
open-coded `response.json()` calls in each module.
The same rule now applies to optional success payload parsing, including lookup
responses that may legitimately return an empty body but must not use ad hoc
`response.text()` plus `JSON.parse(...)` branches in individual modules.
Investigation and AI chat SSE event payload parsing must now also route through
the shared text-to-JSON helper in `frontend-modern/src/api/responseUtils.ts`
rather than through per-module `JSON.parse(...)` stream decoding.
Nullable or legacy collection payloads in governed frontend API clients must
now also route through shared collection-normalization helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through module-local
`|| []`, `?? []`, or `Array.isArray(...)` fallback branches.
That rule now also covers patrol run history responses so malformed or legacy
run collections collapse through the shared helper instead of per-module
fallback lists.
The `/api/ai/patrol/runs` frontend history clients must now also route their
shared fetch plus run-normalization pipeline through one canonical local helper
in `frontend-modern/src/api/patrol.ts` rather than duplicating the same
endpoint-specific stack across each history variant.
That patrol run-history contract now also treats non-positive or malformed
`limit` query values as defaulted input and caps oversized requests to the
backend maximum, rather than letting invalid caller input widen the history
payload unexpectedly.
The frontend Patrol history clients in `frontend-modern/src/api/patrol.ts`
must mirror that normalization before sending the request: invalid and
non-positive caller input collapses back to the client default of `30`, and
oversized requests clamp to the backend maximum of `100`.
Patrol run detail access for selected-history UX must now resolve a canonical
single-run contract at `/api/ai/patrol/runs/{id}` instead of probing bounded
history pages and hoping the target run is still inside a recent window; the
tool-call trace UI must fetch the selected run by ID, with
`?include=tool_calls` carrying the full trace only when explicitly requested.
Frontend investigation rendering for unified Patrol findings must also key off
finding-level investigation metadata, not only `investigation_session_id`:
the investigation detail endpoint is addressed by finding ID, so findings with
canonical `investigation_status`, `investigation_outcome`, or non-zero
`investigation_attempts` must still surface investigation UI even when the
session ID field is absent or blank.
That same Patrol findings UI contract must keep `fix_queued` approval recovery
actions visible even when no live pending approval remains and
`/api/ai/findings/{id}/investigation` resolves to `null` or omits
`proposed_fix`: queued remediation state cannot collapse into a dead badge with
no user action path.
Patrol run-history serialization and persistence must also preserve full field
parity across API responses and restart boundaries, including
`pmg_checked`, `rejected_findings`, `triage_flags`, `triage_skipped_llm`, and
explicit empty `finding_ids` or `effective_scope_resource_ids` arrays when a
run represents an empty snapshot or an intentionally empty effective scope.
The same patrol run-history contract now also treats
`effective_scope_resource_ids` as the canonical analyzed-resource scope when
present, including when it is an explicit empty array, and frontend snapshot
selection must treat an explicit empty `finding_ids` array as an empty snapshot
rather than falling back to unrelated current findings; a missing
`finding_ids` field must retain its "no snapshot filter available" meaning
rather than being collapsed into an empty snapshot.
That same frontend run-history path must also preserve and expose
`triage_flags` and `triage_skipped_llm` from canonical patrol run records so
deterministic triage-only runs do not collapse into generic "no analysis"
history entries.
Patrol status payloads now also treat quickstart credit state as canonical API
contract data: `/api/ai/patrol/status` must surface
`quickstart_credits_remaining`, `quickstart_credits_total`, and
`using_quickstart` directly from backend runtime state so the frontend can
render Patrol quickstart availability without local heuristics or shadow
derived state.
Patrol mutate endpoints that depend on the background service must also fail
closed with `503 Service Unavailable` when AI service initialization is absent
rather than dereferencing a nil service and crashing before a contract response
is written.
The `/api/recovery/rollups` transport now also carries the same normalized
filter contract as `/api/recovery/points`, `/api/recovery/series`, and
`/api/recovery/facets`: cluster, node, namespace, workload scope,
verification, and free-text query filters must remain coherent across all four
recovery endpoints so the recovery UI cannot render mismatched protected-item
and history views for the same active filter set.
The same rule now also covers optional nested node cluster endpoint collections
so `frontend-modern/src/api/nodes.ts` does not own its own
`Array.isArray(node.clusterEndpoints)` response-shape branch.
Canonical alert incident and bulk-acknowledge result payloads must now also
flow through frontend API clients without no-op per-module wrapper
normalization when the backend shape is already canonical.
Legacy `alert_identifier` compatibility promotion in unified finding and patrol
run payloads must now also route through one shared helper in
`frontend-modern/src/api/responseUtils.ts` rather than duplicated per-module
record wrappers.
AI frontend clients must now also call canonical status helpers and direct
URL-segment encoding behavior without module-local alias wrappers when those
wrappers add no contract value.
The discovery frontend client must now also centralize typed and agent route
construction through dedicated path builders rather than repeating route
templates or trivial collection-path aliases across each endpoint.
Notifications email config parsing and node cluster endpoint normalization must
now also route through shared scalar coercion helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through per-module
string/boolean/number helper stacks.
The same shared scalar coercion rule now also applies to monitoring agent
lookup timestamps so `lastSeen` normalization does not live as a module-local
`typeof`/`Date.parse(...)` branch in `frontend-modern/src/api/monitoring.ts`.
The same scalar-coercion contract now also covers optional Proxmox
`clusterEndpoints` collections in `frontend-modern/src/api/nodes.ts`:
frontend consumers may normalize endpoint fields, but they must not fork the
canonical collection-shape guard or reintroduce legacy `alert_identifier`
field access once camelCase `alertIdentifier` has been promoted by the shared
response helpers.
The same frontend API contract now also governs Proxmox agent-install command
transport in `frontend-modern/src/api/nodes.ts`: the canonical client request
shape for `/api/agent-install-command` must support both `type:"pve"` and
`type:"pbs"` with the same explicit `enableProxmox` flag, so install-command
surfaces do not fork into ad hoc raw POST payloads for different Proxmox node
types. That same shared client boundary must also validate a non-empty
`command` response and keep the raw backend `token` field inside
`frontend-modern/src/api/nodes.ts` rather than leaking it into downstream UI
state. Downstream Proxmox install-command consumers like `NodeModal.tsx` must
then surface those canonical validation errors directly rather than collapsing
one node-type pane back to generic copy-generation failure.
Hosted organization-route gating now also falls under this API payload
boundary: when hosted tenants hit organization membership or billing surfaces
through `internal/api/org_handlers.go` and `internal/api/router.go`, inactive
subscriptions must fail with the canonical hosted `402 subscription_required`
payload instead of reusing the self-hosted `multi_tenant_disabled` contract or
falling through to an untyped transport error.
Hosted signup and magic-link error payload normalization must now also route
through shared structured error normalization helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through module-local
error-shape parsing functions.
Governed frontend API clients must now also route canonical non-OK response
throwing through the shared response assertion helper in
`frontend-modern/src/api/responseUtils.ts` rather than open-coding
`throw new Error(await readAPIErrorMessage(...))` in each module.
The same governed modules must now also route assert-then-parse response
pipelines through shared required/optional response helpers in
`frontend-modern/src/api/responseUtils.ts` rather than repeating
`assertAPIResponseOK(...); parseRequiredJSON(...)` or `parseOptionalJSON(...)`
sequences in each client.
Hosted cloud-handoff and billing-admin payloads are canonical API contracts as
well. The handoff exchange must normalize the verified operator email before
it is written into the browser session and before it is returned in the JSON
success payload so session identity, org membership, and handoff payloads
cannot drift on email casing. Hosted billing-admin reads for non-default orgs
must also project the effective default-org hosted lease when the tenant-local
billing file has not been materialized yet, so admin billing-state payloads
stay coherent with the tenant's active entitlement payload instead of briefly
regressing to local trial/default state.
Canonical missing-resource lookups in governed frontend API clients must now
also route `404 => null` response handling through shared response helpers in
`frontend-modern/src/api/responseUtils.ts` rather than open-coding local
status branches in discovery and monitoring clients.
Agent and guest metadata CRUD clients must now also route through one shared
metadata client in `frontend-modern/src/api/metadataClient.ts` rather than
duplicating the same `get/update/delete/list` transport logic in two files.
AI investigation and chat stream clients must now also route through one shared
SSE JSON event consumer in `frontend-modern/src/api/streaming.ts` rather than
duplicating reader lifecycle, timeout, chunk parsing, and event decoding logic
in each module.
Monitoring delete and idempotent mutate clients must now also route `404`/`204`
success cases through shared allowed-status helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
status-branch stacks in each method.
The docker-runtime and kubernetes-cluster resource clients in
`frontend-modern/src/api/monitoring.ts` must now also route shared delete,
allowed-missing mutation, and display-name transport mechanics through
canonical resource-oriented helpers in that file rather than duplicating the
same fetch-and-assert stacks across runtime and cluster variants.
The same monitoring resource clients must now also route shared no-body
`POST` actions and success-envelope command triggers through canonical
resource-oriented helpers in `frontend-modern/src/api/monitoring.ts` rather
than duplicating identical `POST` transport logic across reenroll and runtime
command endpoints.
Those helpers must stay named and structured in resource terms rather than
reintroducing managed-resource terminology, so the monitoring transport layer
matches the canonical resource model exposed elsewhere in v6.
Those monitoring command helpers must also preserve the canonical frontend
fetch-options contract: governed callers pass string-keyed headers only, and
empty-body success responses normalize through the shared success-envelope
parsing path rather than local `response.ok` branches.
Legacy persisted Unified Agent scope aliases from v5 and early v6 installs
must also canonicalize to the current `agent:*` scope identifiers at the
backend contract boundary, so existing installed agents continue to satisfy
`agent:report`, `agent:config:read`, `agent:manage`, and `agent:enroll`
requirements without manual token replacement after upgrade. That
canonicalization may live only at request-ingress and persistence/migration
boundaries; live token records, runtime scope checks, and API payloads may not
preserve or re-emit `host-agent:*` aliases.
Agent profile delete and unassign clients must now also route canonical `204`
success handling through shared allowed-status helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
`if (!isAPIResponseStatus(response, 204))` branches.
Agent profile suggestion and monitoring display-name mutations must now also
route custom `503` and `404` user-facing error promotion through shared
custom-status error helpers in `frontend-modern/src/api/responseUtils.ts`
instead of open-coding local `if (!response.ok) { if (isAPIResponseStatus(...))
throw new Error(...) }` stacks.
Monitoring command-trigger clients must now also route empty-body
`{ success: true }` fallback behavior through a shared success-envelope helper
in `frontend-modern/src/api/responseUtils.ts` instead of open-coding
`parseOptionalAPIResponse(response, { success: true }, ...)` in each method.
AI chat SSE now also treats interactive `question` events as a canonical API
contract surface: backend and frontend must preserve `session_id`,
`question_id`, and the structured `questions` array without handler-local
rewrites or alternate payload aliases.
That same chat SSE contract must remain request-bound. If the HTTP request
context is canceled or the client disconnects, backend assistant execution
must cancel with the request rather than continuing on a detached background
context until an unrelated timeout expires.
Config-registration API contracts at `/api/auto-register` and
`/api/config/nodes` now also require deterministic automated proof: backend
verification must stub TLS fingerprint capture and Proxmox cluster-detection
probes rather than depending on live network reachability, so canonical
request/response verification reflects contract behavior instead of ambient
lab state.
That same canonical `/api/auto-register` response contract must preserve
node identity on success: `nodeId` must carry the resolved stored node name,
not the raw host URL or requested `serverName`, so registration payloads stay
aligned with fleet-control payload consumers.
That same response contract must also return the rest of the backend-owned
completion identity coherently: `type`, `source`, normalized `host`, and
matching `nodeName` must align with the saved node record so installer and
runtime-side Unified Agent callers do not keep separate local success identities after Pulse has
already canonicalized the node.
That same `/api/auto-register` contract also governs the
`node_auto_registered` WebSocket payload: it must emit the normalized stored
host plus the resolved stored node identity in `name`, `nodeId`, and
`nodeName`, rather than leaking raw request fields that can diverge from the
saved node record, together with the effective token id that was reused or
issued.
AI and agent-profile collection/detail clients must now also route `apiFetchJSON`
`402`/`404` fallback behavior through shared API-error-status fallback helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
`try/catch` wrappers that map those statuses to `[]`, `{ plans: [] }`, or
`null`.
Paywalled Patrol remediation-intelligence responses must also scrub derived
metadata together with the collection itself: when remediation history is
license-locked, `remediations`, `count`, and `stats` must all collapse to an
explicit empty state rather than leaking paid history totals through a partial
payload.
Hosted billing-state payloads now also treat Stripe webhook-backed commercial
state as canonical API contract data: when checkout and subscription webhooks
persist paid state, `plan_version`, `stripe_price_id`, and `limits.max_monitored_systems`
must stay aligned instead of emitting paid-state payloads with an empty limits
map or stale canceled-state carryover.
That same hosted billing API boundary also owns runtime base-path resolution:
`internal/api/payments_webhook_handlers.go` must derive webhook dedupe and
customer-index storage from the shared runtime data-dir helper in
`internal/config/config.go` instead of carrying its own `/etc/pulse` fallback,
so hosted billing API side effects stay aligned with the same configured data
directory used by the rest of the product.
Not-found detail lookups in governed frontend API clients must now also route
through explicit status-based `404` handling rather than through broad
catch-all `null` fallbacks that hide real backend failures.
Session and CSRF persistence compatibility under `internal/api/session_store.go`
and `internal/api/csrf_store.go` now also has an explicit governed migration
proof route: legacy raw-token `sessions.json` and `csrf_tokens.json` files must
load through explicit migration helpers, rewrite immediately into hashed
canonical persistence, and stay covered by
`internal/api/session_store_test.go`, `internal/api/csrf_store_test.go`, plus
`tests/migration/v5_session_db_test.go`, rather than borrowing the generic
backend payload contract proof path.
That same governed auth persistence boundary must also stay owned by the
configured runtime data path instead of hidden package-singleton fallbacks:
session, CSRF, and recovery-token stores may not silently self-initialize on
`/etc/pulse` from first access or lock onto the first caller forever through
`sync.Once`; the configured router data path must remain the canonical owner of
those persistence stores, and reinitializing that data path must replace the
old runtime store rather than leaking prior-path state forward.
That same configured-path rule also applies to runtime auth/config reloads:
`internal/config/watcher.go` may use `PULSE_AUTH_CONFIG_DIR` only as an
explicit override, but otherwise it must watch the resolved runtime
`ConfigPath` / `DataPath` owner. The watcher may not probe `/etc/pulse` or
`/data` and silently override the configured path authority for `.env` and
`api_tokens.json` reloads.
That same configured-path rule also applies to manual auth env writes and
status reads under `internal/api/router.go`,
`internal/api/router_routes_auth_security.go`, and
`internal/api/security_setup_fix.go`: those handlers must resolve `.env`
through the shared auth-path helper instead of rebuilding `/etc/pulse/.env`
fallback logic inline.
That same governed auth persistence rule now also covers recovery-token state
under `internal/api/recovery_tokens.go`: raw recovery secrets may be minted for
one-time operator use, but `recovery_tokens.json` must persist only token
hashes and treat any legacy plaintext-token file as an explicit migration input
that is rewritten immediately into hashed canonical persistence on load instead
of leaving raw recovery secrets on the primary runtime disk path.
That same governed persistence rule also covers `internal/config/persistence.go`
API token metadata handling: `api_tokens.json` may hold only hashed token
records, but a legacy plaintext metadata file may only be migration input.
Canonical runtime persistence must rewrite plaintext API token metadata
immediately into encrypted-at-rest storage on load instead of treating the
unencrypted file as a normal primary path.
That same fail-closed persistence rule also applies to persisted OIDC refresh
tokens in `internal/api/session_store.go`: refresh tokens may only be loaded
from or saved to encrypted-at-rest session payloads, and the runtime must drop
them whenever session-store crypto is unavailable or the stored ciphertext is
not canonically decryptable instead of preserving plaintext-at-rest session
state.
Hosted signup handler payload flow now also follows an explicit shared
boundary: `internal/api/public_signup_handlers.go` owns request/response and
magic-link payload semantics, while `internal/hosted/provisioner.go` owns the
shared org bootstrap and rollback mechanics that the hosted signup handler
invokes.
The API token settings surface now also follows the same explicit ownership
rule. Changes to `frontend-modern/src/components/Settings/APITokenManager.tsx`
must carry this contract and the dedicated API-token management proof file
instead of remaining an unowned consumer of token scope labels, token
assignment visibility, and revoke-state presentation.
That shared `APITokenManager.tsx` boundary must also stay under explicit proof
routing on both sides instead of relying only on broad settings-surface
coverage on the security side: token settings changes must continue to carry
the direct `api-token-management-surface` API-contract proof together with the
security-side surface proof.
That same token surface, together with `frontend-modern/src/api/security.ts`,
`internal/api/security.go`, `internal/api/security_tokens.go`, and
`internal/api/system_settings.go`, now also follows an explicit shared
boundary with `security-privacy` so auth posture, token authority, and
telemetry/privacy control semantics stop borrowing their governance only from
the broader API lane.
The `/api/security/tokens` payload contract now also carries explicit owner
binding: token create/list responses must preserve the originating
`ownerUserId` together with org scope so long-lived automation credentials
cannot appear detached from their intended human identity.
That same governed token contract must fail closed on mutation. Limited-scope
API tokens may only create, rotate, or delete tokens whose effective scopes
are a subset of the caller's own scopes; token-management routes must not let a
settings-capable but narrower token revoke or replace a broader credential.
Those owner-bound credentials now also define the effective authenticated
principal on governed API routes: when token metadata carries `ownerUserId`,
RBAC and audit-facing auth resolution must use that bound user identity rather
than a detached synthetic `token:<id>` subject, while still preserving token
scope and org enforcement.
The onboarding QR payload flow now also carries explicit token-bound auth
semantics: when the frontend requests `/api/onboarding/qr` with a pairing
token, the API client must send that token explicitly so the returned payload
and deep link represent the exact minted pairing credential rather than the
ambient browser session, and the mobile-facing `relay.url`/`relay_url` fields
must normalize the stored relay instance endpoint to the app endpoint
(`/ws/app`) so mobile pairing never receives the instance-only `/ws/instance`
route.
Incoming organization-share payloads now also preserve requested access-role
semantics at the API boundary: `/api/orgs/{id}/shares/incoming` must hide
shares whose `accessRole` exceeds the caller's effective role in the target
organization instead of leaking share metadata that the caller cannot
legitimately accept or use.
Organization membership and authorization payloads now also follow an explicit
live-role contract: `/api/orgs` must list only organizations the caller
currently belongs to, and org-management endpoints must reflect member
promotion or demotion immediately rather than continuing to authorize from
stale owner/admin assumptions after the role change has already been
persisted.
System settings API payloads now also carry an explicit v6 channel contract:
`updateChannel` resolves to `stable` or `rc` with `stable` as the default, and
`autoUpdateEnabled` must serialize as `false` whenever the effective channel is
`rc`, even if stale persisted state or omitted request fields would otherwise
leave unattended updates enabled.
Update API channel selection now also follows that same contract: `/api/updates`
surfaces accept only `stable` or `rc`, reject unsupported channel values at the
HTTP boundary, and must not allow a `stable` installation path to apply a
prerelease tarball even when a caller posts a direct GitHub release URL.
The `/api/resources` and `/api/resources/stats` handlers now also carry a
single-snapshot aggregation invariant: canonical `aggregations.byType` must be
derived from the same registry list snapshot used for that request's response
path, so the contract stays deterministic without paying for duplicate
registry-clone work on the hot path. That same governed resource contract now
also includes backend-derived `policy` and `aiSafeSummary` fields, and list,
detail, and child payloads must source those values from canonical unified
resource metadata rather than from frontend- or AI-local heuristics.
That same resource-handler seed contract must also stay on canonical unified
resource ownership for tenant-scoped requests: once a tenant state provider
implements `UnifiedResourceSnapshotForTenant`, `/api/resources` may not fall
back to raw tenant `StateSnapshot` seeding when that unified seed is empty.
Tenant AI service wiring now follows that same canonical ownership rule:
`internal/api/ai_handlers.go` may provide tenant `ReadState` and
tenant-scoped unified-resource providers, but it must not mint tenant snapshot
provider bridges purely to satisfy Patrol once the Patrol runtime can operate
from those canonical tenant providers directly.
Hosted licensing handlers now also carry a tenant-scoped fallback contract:
when hosted auth handoff preserves a non-default tenant org like `t-...`,
`/api/license/status` and `/api/license/entitlements` must still evaluate the
instance-level hosted billing lease from `default` if that tenant org has no
org-local billing state of its own, rather than failing closed into
`subscription_required` on first entry.
