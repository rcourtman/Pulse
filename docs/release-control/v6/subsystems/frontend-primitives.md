# Frontend Primitives Contract

## Contract Metadata

```json
{
  "subsystem_id": "frontend-primitives",
  "lane": "L8",
  "contract_file": "docs/release-control/v6/subsystems/frontend-primitives.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own reusable frontend primitives and presentational patterns so feature work
extends shared components instead of creating new local variants.

## Canonical Files

1. `frontend-modern/src/components/shared/`
2. `frontend-modern/src/components/shared/PageControls.guardrails.test.ts`
3. `frontend-modern/src/components/shared/TypeColumn.guardrails.test.ts`
4. `frontend-modern/src/features/`

## Extension Points

1. Add shared primitives in `frontend-modern/src/components/shared/`
2. Add feature-specific presentation only when no shared primitive should own it
3. Add guardrail tests when a new shared pattern is introduced

## Forbidden Paths

1. Reinventing table/filter/toggle primitives when a shared version exists
2. Feature-local styling forks of canonical shared components without explicit justification
3. Direct imports that bypass shared presentation helpers where guardrails exist

## Completion Obligations

1. Update guardrail tests when new shared primitives are added
2. Update this contract when a new canonical UI pattern is adopted
3. Remove local forks after the shared primitive is introduced

## Current State

The frontend already has several guardrail tests. The next step is to keep
turning repeated local patterns into explicit shared primitives with hard usage
bounds.

The subsystem registry now also requires explicit proof-policy coverage for all
shared runtime files, and shared-component guardrails fail if raw table
composition is reintroduced in new shared components outside the canonical
allowlist.
