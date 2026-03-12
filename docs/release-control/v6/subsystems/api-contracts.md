# API Contracts

## Contract Metadata

```json
{
  "subsystem_id": "api-contracts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/api-contracts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own canonical runtime payload shapes between backend and frontend.

## Canonical Files

1. `internal/api/contract_test.go`
2. `internal/api/resources.go`
3. `internal/api/alerts.go`
4. `frontend-modern/src/types/api.ts`
5. `frontend-modern/src/api/responseUtils.ts`
6. `frontend-modern/src/components/Settings/APITokenManager.tsx`
7. `frontend-modern/src/components/Settings/UnifiedAgents.tsx`

## Shared Boundaries

1. `frontend-modern/src/components/Settings/UnifiedAgents.tsx` shared with `monitoring`: the unified agent settings surface is both a canonical monitoring-truth consumer and an API token, lookup, and assignment contract boundary.
2. `internal/api/licensing_bridge.go` shared with `cloud-paid`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
3. `internal/api/licensing_handlers.go` shared with `cloud-paid`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
4. `internal/api/payments_webhook_handlers.go` shared with `cloud-paid`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
5. `internal/api/public_signup_handlers.go` shared with `cloud-paid`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.
6. `internal/api/resources.go` shared with `unified-resources`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
7. `internal/api/slo.go` shared with `performance-and-scalability`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add or change payload fields through handler + contract tests together
2. Update frontend API types in lockstep with backend contract changes
3. Add dedicated contract tests for new stable payloads
4. Route frontend API-client parsed error propagation, API-error-status fallback handling, allowed-status handling, custom status-specific error handling, command-trigger success envelope handling, shared response parsing pipelines, missing-resource lookup handling, metadata CRUD routing, stream event consumption, response status, collection normalization, scalar payload coercion, and structured error normalization through canonical shared helpers under `frontend-modern/src/api/`
5. Add or change API token scope, assignment, and revocation presentation through `frontend-modern/src/components/Settings/APITokenManager.tsx`
6. Add or change unified agent token generation, lookup, and assignment presentation through `frontend-modern/src/components/Settings/UnifiedAgents.tsx`

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

## Current State

The API layer already uses contract tests in many places, but every major live
contract should continue moving toward canonical-only runtime shapes.
The unified agent settings surface now also follows an explicit shared
boundary with monitoring. Changes to
`frontend-modern/src/components/Settings/UnifiedAgents.tsx` must carry this
contract together with the shared monitoring contract and the dedicated API
proof files for token generation, agent lookup, and profile assignment, rather
than remaining an unowned consumer of those contract surfaces.
The unified-agent install endpoints now also carry an exact-release fallback
contract: when `/install.sh` or `/install.ps1` cannot be served locally, the
backend must proxy the install script asset from the exact GitHub release that
matches `serverVersion` and must fail closed for dev or unreleased builds
rather than serving branch-tip installer logic.
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
The docker-runtime and kubernetes-cluster managed-resource clients in
`frontend-modern/src/api/monitoring.ts` must now also route shared delete,
allowed-missing mutation, and display-name transport mechanics through
canonical local helpers in that file rather than duplicating the same
fetch-and-assert stacks across runtime and cluster variants.
The same monitoring managed-resource clients must now also route shared
no-body `POST` actions and success-envelope command triggers through canonical
local helpers in `frontend-modern/src/api/monitoring.ts` rather than
duplicating identical `POST` transport logic across reenroll and runtime
command endpoints.
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
persist paid state, `plan_version`, `stripe_price_id`, and `limits.max_agents`
must stay aligned instead of emitting paid-state payloads with an empty limits
map or stale canceled-state carryover.
Not-found detail lookups in governed frontend API clients must now also route
through explicit status-based `404` handling rather than through broad
catch-all `null` fallbacks that hide real backend failures.
Session and CSRF persistence compatibility under `internal/api/session_store.go`
and `internal/api/csrf_store.go` now also has an explicit governed migration
proof route: legacy raw-token `sessions.json` and `csrf_tokens.json` files must
load into hashed runtime state and stay covered by
`internal/api/session_store_test.go`, `internal/api/csrf_store_test.go`, plus
`tests/migration/v5_session_db_test.go`, rather than borrowing the generic
backend payload contract proof path.
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
The `/api/security/tokens` payload contract now also carries explicit owner
binding: token create/list responses must preserve the originating
`ownerUserId` together with org scope so long-lived automation credentials
cannot appear detached from their intended human identity.
Those owner-bound credentials now also define the effective authenticated
principal on governed API routes: when token metadata carries `ownerUserId`,
RBAC and audit-facing auth resolution must use that bound user identity rather
than a detached synthetic `token:<id>` subject, while still preserving token
scope and org enforcement.
The onboarding QR payload flow now also carries explicit token-bound auth
semantics: when the frontend requests `/api/onboarding/qr` with a pairing
token, the API client must send that token explicitly so the returned payload
and deep link represent the exact minted pairing credential rather than the
ambient browser session.
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
registry-clone work on the hot path.
