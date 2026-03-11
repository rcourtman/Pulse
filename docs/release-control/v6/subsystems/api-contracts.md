# API Contracts

## Purpose

Own canonical runtime payload shapes between backend and frontend.

## Canonical Files

1. `internal/api/contract_test.go`
2. `internal/api/resources.go`
3. `internal/api/alerts.go`
4. `frontend-modern/src/types/api.ts`

## Extension Points

1. Add or change payload fields through handler + contract tests together
2. Update frontend API types in lockstep with backend contract changes
3. Add dedicated contract tests for new stable payloads

## Forbidden Paths

1. Handler-local payload shape drift without a contract test
2. Untracked compatibility aliases becoming permanent runtime contracts
3. Frontend-only payload assumptions that are not owned in backend contracts

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
