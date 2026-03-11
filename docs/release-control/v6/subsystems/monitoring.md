# Monitoring Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "unified-resources"
  ]
}
```

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
3. Add unified supplemental ingest through `internal/monitoring/poll_providers.go`

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

Node export is now derived from canonical `ReadState.Nodes()` instead of
`GetState().Nodes`; `models.Node` is treated as a boundary artifact for that
path.

Host export is now derived from canonical `ReadState.Hosts()` instead of
`GetState().Hosts`; `models.Host` is treated as a boundary artifact for that
path.

Docker host export is now derived from canonical `ReadState.DockerHosts()`
instead of `GetState().DockerHosts`; `models.DockerHost` is treated as a
boundary artifact for that path.

VM and container export are now derived from canonical `ReadState.VMs()` and
`ReadState.Containers()` instead of `GetState().VMs`/`GetState().Containers`;
`models.VM` and `models.Container` are treated as boundary artifacts for those
paths.

PBS instance export is now derived from canonical `ReadState.PBSInstances()`
instead of `GetState().PBSInstances`; `models.PBSInstance` is treated as a
boundary artifact for that path.

Backup-alert guest lookup assembly now derives VM/container identity from
canonical `ReadState` workload views instead of from snapshot-owned guest
arrays, so backup alert resolution follows unified runtime truth when a live
resource registry exists.

Physical-disk refresh/merge logic now derives physical disks, nodes, and linked
host-agent context from canonical `ReadState` before applying NVMe temperature
and SMART merges, so skipped or background disk refresh no longer treats the
snapshot as internal truth for that path.

Backup polling and recovery guest identity assembly now derive workload node,
name, and type context from canonical `ReadState` instead of from
snapshot-owned VM/container arrays, so storage backup polling, guest snapshot
polling, timeout sizing, PBS recovery candidate assembly, and Proxmox recovery
ingest all follow unified runtime truth when a live resource registry exists.

Storage-backup preservation now also derives node-to-storage membership from
canonical `ReadState.StoragePools()` instead of from snapshot-owned storage
arrays, leaving only persisted backup/cache payloads in this path on direct
snapshot state.

Canonical monitoring guardrails now also fail if resource-array access is
reintroduced through `GetState().VMs`/`Containers`/`Nodes`/`Hosts`/`Storage`/
`DockerHosts`/`PBSInstances` helpers, and the subsystem registry now requires
explicit proof-policy coverage for all owned runtime files.
