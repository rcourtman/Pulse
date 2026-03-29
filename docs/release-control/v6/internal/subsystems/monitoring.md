# Monitoring Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L13",
  "contract_file": "docs/release-control/v6/internal/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
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
3. `internal/monitoring/monitor_discovery_helpers.go`
4. `internal/monitoring/metrics.go`
5. `internal/monitoring/metrics_history.go`
6. `internal/unifiedresources/read_state.go`
7. `internal/unifiedresources/monitor_adapter.go`
8. `internal/unifiedresources/views.go`
9. `internal/monitoring/connected_infrastructure.go`
10. `internal/monitoring/reload.go`
11. `docker-entrypoint.sh`
12. `internal/monitoring/truenas_poller.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add pollers/providers and discovery-provider coordination through `internal/monitoring/poll_providers.go` and `internal/monitoring/monitor_discovery_helpers.go`
2. Add metrics capture or history-retention behavior through `internal/monitoring/metrics.go` and `internal/monitoring/metrics_history.go`
3. Add typed read access through `internal/unifiedresources/views.go`
4. Add unified supplemental ingest through `internal/monitoring/poll_providers.go`
5. Add or change container startup ownership/bootstrap behavior for hosted or managed Pulse runtime mounts through `docker-entrypoint.sh`

## Forbidden Paths

1. New consumer logic built directly on `Monitor.GetState()`
2. New runtime truth living only in `models.StateSnapshot`
3. Snapshot-backed helper paths used where `ReadState` should be authoritative

## Completion Obligations

1. Update this contract when monitoring truth ownership changes
2. Tighten guardrails when `GetState()`-centric paths are removed
3. Keep discovery-provider, metrics-history, and container bootstrap proof routes explicit in `registry.json`
4. Update related read-state or monitor tests when new collector paths land

## Current State

This subsystem now sits under the dedicated core monitoring runtime lane so
discovery, metrics-history correctness, and platform-specific runtime coverage
can be governed as first-class product work instead of staying diluted inside
architecture coherence.

The monitor adapter now also acts as the canonical bridge from live registry
rebuilds and supplemental ingest into the unified-resource timeline. That means
monitoring no longer just materializes state snapshots for consumers; it also
emits durable `ResourceChange` history through the shared resource store so
live monitoring updates and historical inspection stay aligned.
That same ownership now includes alert-lifecycle facts emitted by monitoring.
When an alert is fired, acknowledged, unacknowledged, or resolved for a
canonical resource, the monitoring runtime must write the corresponding durable
resource-history event into the unified-resource change store instead of
leaving that lifecycle only inside alert-scoped incident memory. Incident
timelines may still project those breadcrumbs for operator flow, but the
durable backend truth for alert lifecycle now lives on the canonical resource
timeline.
The monitor-owned incident store wiring must therefore attach the canonical
resource timeline reader whenever the unified monitor adapter is present, so
operator alert timelines and AI incident context project those lifecycle events
from canonical history instead of reading a second monitoring-owned timeline.

The registry proof map now treats provider discovery and metrics history as
their own governed runtime surfaces instead of leaving them folded into a
generic monitoring catch-all. Changes to provider wiring, discovery helpers,
or metrics history retention must stay attached to those explicit proof routes.
Install-wide telemetry counts are also monitoring-owned now. Any telemetry or
reporting surface that claims installation totals must aggregate across the
provisioned tenant set through the reloadable multi-tenant monitor boundary,
not by reading `GetMonitor()`'s default-org compatibility shim.

Consumer packages already use `ReadState`, but the monitoring core still has
dual truth between unified resources and `StateSnapshot`. This is the main
remaining architecture-coherence lane.
The container entrypoint in `docker-entrypoint.sh` now also lives under this
boundary. Hosted or managed tenant bootstrap changes must preserve safe startup
when immutable read-only mounts are layered into `/etc/pulse`; the entrypoint
may not reintroduce ownership mutation against those read-only files during
container boot.

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
Typed unified-resource views also need to present canonical monitoring truth,
not raw ingest formatting. Linked topology accessors exposed through
`internal/unifiedresources/views.go` must trim outer whitespace before
returning linked agent, node, VM, or container IDs so downstream consumers do
not observe `" node-99 "` style drift when the canonical linkage is `node-99`.
Source-owned IDs exposed through those same typed views must also trim outer
whitespace before they reach monitoring consumers, so a docker host, VM, node,
or storage view cannot appear to carry a different source identity just
because the ingest payload wrapped the source ID in spaces.
The same applies to proxmox topology coordinates exposed through typed views:
node, cluster, and instance accessors must return canonical trimmed values so
monitoring consumers do not fork topology grouping or labeling on `" pve-a "`
versus `pve-a`.
That same canonical guest runtime truth now also includes Proxmox pool
membership. The cluster-resource builders and traditional VM/LXC pollers must
carry `pool` through `models.VM` and `models.Container` so reporting and
inventory surfaces consume one canonical guest topology contract instead of
re-deriving pool membership from API-local queries.
Connected infrastructure and monitored-system projections now also use the
shared unified-resource display-name fallback, so the monitoring layer does
not rebuild its own canonical name-or-hostname selection for those surfaces.
Connected infrastructure now also consumes the shared top-level system
resolver from unified resources instead of maintaining an independent
machine/hostname grouping heuristic. Monitoring-owned inventory surfaces must
therefore stay aligned with the monitored-system ledger on one canonical
top-level system identity contract, and that contract must not count friendly
display names as identity.

Storage-backup preservation now also derives node-to-storage membership from
canonical `ReadState.StoragePools()` instead of from snapshot-owned storage
arrays, leaving only persisted backup/cache payloads in this path on direct
snapshot state.

Canonical monitoring guardrails now also fail if resource-array access is
reintroduced through `GetState().VMs`/`Containers`/`Nodes`/`Hosts`/`Storage`/
`DockerHosts`/`PBSInstances` helpers, and the subsystem registry now requires
explicit proof-policy coverage for all owned runtime files.
Memory-source classification now also routes through one canonical runtime
catalog and extracted node resolver under `internal/monitoring/`. Node, VM,
LXC, diagnostics, and
diagnostic-snapshot consumers must normalize aliases such as `avail-field`,
`meminfo-available`, `meminfo-derived`, `meminfo-total-minus-used`, and
`listing-mem` onto the governed canonical labels `available-field`,
`derived-free-buffers-cached`, `derived-total-minus-used`, and
`cluster-resources` before trust or fallback reporting is emitted.
That same catalog owns fallback-reason defaults for governed fallback sources,
so monitoring producers and downstream diagnostics must not fork fallback
classification or reason text through lane-local switch statements.
That same canonicalization boundary must also run when snapshots are recorded,
not only at source selection time: node and guest diagnostic snapshots must
normalize memory-source aliases and backfill default fallback reasons before
logging or persistence, so later diagnostics/reporting cannot diverge just
because one poll path still emitted a compatibility label.
That compatibility boundary also applies to historical snapshot labels that may
still exist in tests, live in-memory state, or pre-canonical diagnostic paths:
legacy aliases such as `rrd-available`, `rrd-data`, `node-status-available`,
`calculated`, and `listing` must normalize onto the governed canonical labels
before snapshots are returned to diagnostics consumers, not only when new
snapshots are first recorded.
The same canonical identity rule now applies when removed host agents are
blocked from re-reporting. `ApplyHostReport` must resolve the final canonical
host identifier for the `(token, machine-id, hostname)` tuple before it checks
`removedHostAgents` or emits the reconnect-blocking error, so removing one
token-bound host cannot poison a different host that shared the same raw
machine identifier.
Node disk-source selection now also routes through one canonical resolver
under `internal/monitoring/`. When a Proxmox node has a linked Pulse host
agent, the node summary must prefer the linked host's canonical disk view over
Proxmox `rootfs` bytes because dataset-level `rootfs` can materially
under-report ZFS-backed node capacity and usage. Proxmox `rootfs` and `/nodes`
disk values remain fallback sources only when no linked host disk truth is
available.
TrueNAS monitoring ownership now also includes provider rebind semantics in
`internal/monitoring/truenas_poller.go`. When a stored TrueNAS connection's
host, auth, TLS, or fingerprint settings change, the poller must replace the
live provider instance instead of keeping stale connection state in memory
until the process restarts.
That same monitoring boundary now also owns live TrueNAS disk temperatures.
`internal/truenas/client.go` and `internal/truenas/provider.go` must ingest
`disk.temperatures` from the TrueNAS API and project those readings into the
canonical physical-disk model and risk path instead of leaving temperature
telemetry agent-only or adding a TrueNAS-local presentation shim.
