# Monitoring Contract

## Purpose

Own polling, typed collection, runtime state assembly, and canonical monitoring
truth for live infrastructure data.

## Canonical Files

1. `internal/monitoring/monitor.go`
2. `internal/monitoring/poll_providers.go`
3. `internal/unifiedresources/read_state.go`
4. `internal/unifiedresources/monitor_adapter.go`
5. `internal/unifiedresources/views.go`

## Extension Points

1. Add pollers/providers through `internal/monitoring/`
2. Add typed read access through `internal/unifiedresources/views.go`
3. Add unified supplemental ingest through `poll_providers.go`

## Forbidden Paths

1. New consumer logic built directly on `Monitor.GetState()`
2. New runtime truth living only in `models.StateSnapshot`
3. Snapshot-backed helper paths used where `ReadState` should be authoritative

## Completion Obligations

1. Update this contract when monitoring truth ownership changes
2. Tighten guardrails when `GetState()`-centric paths are removed
3. Update related read-state or monitor tests when new collector paths land

## Current State

Consumer packages already use `ReadState`, but the monitoring core still has
dual truth between unified resources and `StateSnapshot`. This is the main
remaining architecture-coherence lane.

Storage export is now derived from canonical `ReadState.StoragePools()`
instead of `GetState().Storage`; `models.Storage` is treated as a boundary
artifact for that path.
