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

## Shared Boundaries

1. `internal/api/licensing_bridge.go` shared with `cloud-paid`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
2. `internal/api/licensing_handlers.go` shared with `cloud-paid`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
3. `internal/api/payments_webhook_handlers.go` shared with `cloud-paid`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
4. `internal/api/resources.go` shared with `unified-resources`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
5. `internal/api/slo.go` shared with `performance-and-scalability`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.

## Extension Points

1. Add or change payload fields through handler + contract tests together
2. Update frontend API types in lockstep with backend contract changes
3. Add dedicated contract tests for new stable payloads
4. Route frontend API-client error, response status, and shared JSON parsing through `frontend-modern/src/api/responseUtils.ts`

## Forbidden Paths

1. Handler-local payload shape drift without a contract test
2. Untracked compatibility aliases becoming permanent runtime contracts
3. Frontend-only payload assumptions that are not owned in backend contracts
4. Frontend API clients inferring canonical HTTP status from `Error.message` text
5. Frontend API clients branching on raw `response.status` checks for governed status handling instead of the shared response-status helpers
6. Frontend API clients parsing governed success or stream payloads with raw `response.json()`, ad hoc `response.text()` + `JSON.parse(...)`, or per-module `JSON.parse(...)` stream decoding instead of the shared response parsing helpers
7. Frontend API clients normalizing nullable or legacy collection payloads with module-local `|| []`, `?? []`, or ad hoc `Array.isArray(...)` fallbacks instead of shared collection helpers
8. Frontend API clients swallowing non-not-found API failures behind broad `catch { return null; }` fallbacks instead of routing only canonical `404` cases through explicit status checks

## Completion Obligations

1. Update contract tests when payloads change
2. Update frontend API types in the same slice
3. Route runtime changes through the explicit API-contract proof policies in `registry.json`; default fallback proof routing is not allowed
4. Update this contract when canonical payload ownership changes

## Current State

The API layer already uses contract tests in many places, but every major live
contract should continue moving toward canonical-only runtime shapes.
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
Not-found detail lookups in governed frontend API clients must now also route
through explicit status-based `404` handling rather than through broad
catch-all `null` fallbacks that hide real backend failures.
