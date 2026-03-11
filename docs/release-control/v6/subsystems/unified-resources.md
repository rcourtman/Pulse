# Unified Resources Contract

## Purpose

Own canonical resource identity, type normalization, typed views, and
cross-source deduplication.

## Canonical Files

1. `internal/unifiedresources/types.go`
2. `internal/unifiedresources/views.go`
3. `internal/unifiedresources/read_state.go`
4. `internal/unifiedresources/adapters.go`
5. `internal/unifiedresources/monitor_adapter.go`

## Extension Points

1. Add new resource types and identity fields in `types.go`
2. Add typed accessors and views in `views.go`
3. Add source ingestion/adaptation in the adapter layer only

## Forbidden Paths

1. New ad hoc resource-type aliases outside unified resource normalization
2. New duplicate ID normalization logic outside unified resources
3. Reintroducing legacy runtime resource contracts as live truth

## Completion Obligations

1. Update this contract when canonical resource identity or type rules change
2. Update contract and guardrail tests when a new resource type is added
3. Tighten banned-path tests when a compatibility bridge is removed

## Current State

The unified resource core is strong and canonical, but monitoring and some
frontend/API consumers are still being tightened around it.

Canonical storage metadata now carries runtime `enabled` and `active` flags so
monitoring and API export paths can derive `models.Storage` from unified views
without depending on legacy snapshot ownership.
