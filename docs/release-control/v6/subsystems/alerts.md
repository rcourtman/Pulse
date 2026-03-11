# Alerts Contract

## Purpose

Own alert identity, alert specs, evaluation, persistence semantics, and
notification behavior for live runtime alerts.

## Canonical Files

1. `internal/alerts/specs/types.go`
2. `internal/alerts/specs/evaluator.go`
3. `internal/alerts/canonical_metric.go`
4. `internal/alerts/canonical_lifecycle.go`
5. `internal/alerts/unified_incidents.go`

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`
2. Add typed collector/builders in `internal/alerts/alerts.go`
3. Add identity/persistence updates through canonical alert helpers only

## Forbidden Paths

1. New ad hoc `Check*`-local evaluator logic
2. Reintroducing runtime legacy alert-ID contracts
3. Reintroducing per-family threshold/override merge logic outside the shared path

## Completion Obligations

1. Update alert spec/evaluator tests when a new rule kind is added
2. Update this contract if alert truth or identity rules change
3. Route runtime changes through the explicit alert proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten or add guardrails when an old alert path is removed

## Current State

Canonical alert identity and evaluation are the live runtime model. Remaining
legacy references should exist only in explicit migration or negative test
boundaries.

Frontend alert surfaces and backend alert-support files now require explicit
registry path-policy coverage, so new alert-owned runtime files must be mapped
to a concrete proof route instead of silently inheriting subsystem-default
verification.
