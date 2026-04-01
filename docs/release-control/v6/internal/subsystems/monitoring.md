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
4. `internal/monitoring/monitor_polling_node.go`
5. `internal/monitoring/monitor_pve.go`
6. `internal/monitoring/monitor_pve_storage.go`
7. `internal/monitoring/node_disk_sources.go`
8. `internal/monitoring/metrics.go`
9. `internal/monitoring/metrics_history.go`
10. `internal/unifiedresources/read_state.go`
11. `internal/unifiedresources/monitor_adapter.go`
12. `internal/unifiedresources/views.go`
13. `internal/monitoring/connected_infrastructure.go`
14. `internal/monitoring/reload.go`
15. `docker-entrypoint.sh`
16. `internal/monitoring/truenas_poller.go`
17. `internal/monitoring/vmware_poller.go`
18. `internal/dockeragent/swarm.go`
19. `internal/monitoring/guest_memory_sources.go`
20. `internal/monitoring/guest_memory_stability.go`
21. `internal/monitoring/monitor_polling_vm.go`
22. `internal/monitoring/monitor_pve_guest_builders.go`
23. `internal/monitoring/monitor_pve_guest_poll.go`
24. `internal/monitoring/guest_disk_stability.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add pollers/providers and discovery-provider coordination through `internal/monitoring/poll_providers.go` and `internal/monitoring/monitor_discovery_helpers.go`
2. Add metrics capture or history-retention behavior through `internal/monitoring/metrics.go` and `internal/monitoring/metrics_history.go`
3. Add typed read access through `internal/unifiedresources/views.go`
4. Add unified supplemental ingest through `internal/monitoring/poll_providers.go`
5. Add or change container startup ownership/bootstrap behavior for hosted or managed Pulse runtime mounts through `docker-entrypoint.sh`
6. Add or change Docker Swarm manager task/service runtime collection through `internal/dockeragent/swarm.go`

## Forbidden Paths

1. New consumer logic built directly on `Monitor.GetState()`
2. New runtime truth living only in `models.StateSnapshot`
3. Snapshot-backed helper paths used where `ReadState` should be authoritative

## Completion Obligations

1. Update this contract when monitoring truth ownership changes
2. Tighten guardrails when `GetState()`-centric paths are removed
3. Keep discovery-provider, guest-memory trust, metrics-history, Docker Swarm collection, and container bootstrap proof routes explicit in `registry.json`
4. Update related read-state or monitor tests when new collector paths land
5. Keep platform ingestion semantics aligned with
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`: hybrid is a
   declared ingestion mode on an admitted first-class platform, not a license
   to create new platform ids from secondary pollers or optional agent
   augmentation paths.

## Current State

This subsystem now sits under the dedicated core monitoring runtime lane so
discovery, metrics-history correctness, and platform-specific runtime coverage
can be governed as first-class product work instead of staying diluted inside
architecture coherence.
VMware vSphere now also has a locked phase-1 ingestion boundary under this
lane. The admitted direction is vCenter-only in phase 1, and monitoring must
stay API-first through the
official vCenter Automation API plus the Virtual Infrastructure JSON API.
Direct ESXi remains out of phase 1 because the standalone host-agent hierarchy
is materially narrower than the vCenter inventory and the declared support
floor depends on vCenter-backed topology, shared datastore scope, alarm state,
and historical performance access. Any later direct-ESXi work must be admitted
explicitly instead of inheriting vCenter support by implication.
That same VMware monitoring boundary now also includes the canonical telemetry
rule. ESXi host metrics and history belong on the shared `agent` path, VM
metrics and history belong on the shared `vm` path, and datastore
capacity/accessibility history belongs on the shared `storage` path. VMware
phase-1 work must not create `vmware-host`, `vmware-vm`, or
`vmware-datastore` history stores just because the collection APIs differ from
other platforms.
That same VMware monitoring boundary now also includes the source and identity
rule. Runtime collection may authenticate to `vCenter`, call multiple VMware
API families, and gather several object classes, but the emitted state must
still collapse onto one canonical VMware source classification and one
provider-scoped identity model for hosts, VMs, and datastores. Monitoring must
not leak `vcenter` versus `esxi` transport distinctions into downstream
resource identity or source filtering.
That same VMware monitoring boundary now also includes provider ownership. One
saved VMware connection should map to one provider owner and one canonical poll
health record, even if that provider keeps separate authenticated Automation
API and VI JSON clients internally. Connection edits that change host, auth,
TLS, or poll cadence must replace that live provider state instead of leaving
stale VMware sessions resident until restart.
That same provider-owned summary must also serve the shared settings runtime
surface. `internal/monitoring/vmware_poller.go` owns the per-connection poll
summary (`poll` plus `observed`), `POST /api/vmware/connections/{id}/test`
with no edit overlay must refresh that same summary owner, and
`/api/vmware/connections` list reads must consume the poller summary instead of
recomputing or shadowing it inside handler-local runtime state. Internal
sub-second test harness intervals must not leak `intervalSeconds: 0` onto that
operator-facing contract.
That same monitoring boundary now also owns runtime mock rebind continuity for
API-backed supplemental providers. When `/api/system/mock-mode` flips on a
running server, the live TrueNAS and VMware provider bindings must swap to the
mock-backed supplemental records and refresh canonical read-state immediately
instead of waiting for a process restart before shared resource consumers can
see the platform inventory.
That same mock-runtime boundary also owns freshness while demos are running.
The mock update loop must keep provider-backed TrueNAS and VMware records plus
legacy PBS and PMG summaries on current `LastSeen` and health state each tick,
so long-lived infrastructure, workloads, storage, and recovery demos do not
decay into synthetic stale-state warnings while mock mode remains enabled.
That same demo-owned mock boundary also owns chart continuity. Seeded mock
history and runtime mock sampling must be projections of the same canonical
metric timeline, so changing chart ranges feels like zooming one history
window instead of stitching a second live tail onto the end of seeded
sparklines. Monitoring must not let any mock-owned resource receive a
duplicate generic unified-resource writer that appends a divergent recent tail
after the canonical mock sampler has already seeded and extended that series.
The seed path must therefore include the canonical terminal `now` sample on
its tiered timeline and anchor seeded series to the canonical metric model at
that timestamp instead of to mutable state fields, so historical charts match
the exact runtime history that would have been recorded live.
That means seeded history must sample the shared canonical mock runtime metric
function at every historical timestamp for every mock-owned resource class.
Monitoring must not approximate the past from snapshot/current values and then
switch to the canonical sampler only for recent live ticks, because that still
creates a visible seam even when the identities and timestamps are correct.
Seeded history and subsequent live mock writes must also record on the same
canonical chart-time grid. Monitoring must not seed on one wall-clock phase
and append live ticks on another `time.Now()` phase, because the canonical
sampler is dynamic enough that off-grid recent points still look like a
different tail.
Runtime mock tick writers must also sample the canonical metric model at the
recorded chart timestamp instead of copying mutable state fields directly,
because graph refresh cadence and state rounding can otherwise append a recent
tail that looks like a different generator even when the underlying mock
resource identity has not changed. Provider-backed fixture refresh paths must
derive their live host, workload, storage, and disk-history writes from that
same canonical sampler instead of replaying snapshot values. Native polling
lanes and unified sync must not append duplicate mock history once the
canonical mock sampler owns that resource class.
That same ownership rule applies by default whenever mock mode is enabled.
Real client initialization, native pollers, and async agent-origin metric
writers are support-only opt-ins, not the normal demo path, and they must not
append chart-history or persistent metric-store points onto mock-owned
timelines while the canonical mock sampler is active.
That same chart boundary also owns role-shaped realism. Seeded history,
synthetic summary fallbacks, and runtime mock writes must derive their bounds
and curve shape from the same canonical resource-role registry, so database,
cache, backup, web, and storage workloads keep believable long-range behavior
instead of switching from one generic seeded pattern to a different recent
runtime pattern.
That same mock-runtime owner now also owns demo-scenario curation. The
canonical `internal/mock/fixture_graph.go` path may project an authored demo
estate over generic fixture synthesis, but that authored layer must stay
graph-native and runtime-stable so infrastructure, workloads, storage, and
recovery all present the same human-readable platform story instead of a lab
of random names, legacy `mock-cluster` labels, or surface-specific mock
overrides.
That same chart boundary also owns storage-series identity. Monitoring and
`ReadState` consumers must address storage pool and physical-disk history
through the resolved unified-resource metrics target, so seeded history,
runtime writes, storage summary hover selection, and detail charts all extend
one series instead of splitting between canonical resource IDs and
source-native metric IDs.
That same chart boundary also owns provider-backed workload bridging.
Workload-chart consumers may query VM and system-container history through the
resolved unified-resource metrics target, but the emitted series identity must
stay on the canonical workload row ID, so VMware-backed workloads participate
in summary hover and focus without leaking provider-native metric IDs into the
UI contract.
That same summary owner also owns VMware partial-success classification.
Optional VI JSON or Automation enrichment reads that fail after base
host/VM/datastore inventory succeeds must not collapse the whole poll into a
runtime failure. The client should preserve the usable base snapshot, record
degraded enrichment issues on the snapshot, and let the poller publish those
as `observed.degraded` plus summarized issue metadata instead of clearing the
observed contribution or pretending the refresh was fully healthy.
That same poller-owned partial-success model must also keep runtime
observability non-noisy. Repeated polls with the same degraded optional-read
issue classes should not emit a fresh warning every interval; monitoring
should log only when VMware optional enrichment first degrades, materially
changes, or recovers.
That provider ownership now has a concrete phase-1 runtime seam:
`internal/monitoring/vmware_poller.go` must keep VMware inventory on the
shared supplemental-ingest path, declare `SourceVMware` as its owned source,
and cache per-organization, per-connection provider records instead of
projecting VMware through `StateSnapshot`-local host or storage arrays.
`internal/api/router.go` may start and stop that poller as shared runtime
infrastructure, but monitoring still owns the provider lifecycle, source
ownership, and canonical record emission rules for VMware.
That same VMware monitoring boundary now also includes the proof rule for
history depth. `PerformanceManager.QueryPerfComposite` clearly supports
host-plus-child metric collection, but exact VM and datastore history fidelity
still requires live proof on the supported version floor. If that proof does
not hold on the shared history model, the support claim must narrow rather
than falling back to VMware-only history paths.
That same VMware monitoring boundary now also includes the incident-context
rule. VMware event and task reads may support investigation, but they must
feed the shared incident and canonical resource-history paths instead of a
parallel VMware event store or provider-only incident timeline.
That same VMware monitoring boundary also includes the topology-signal rule.
Signals collected from non-projected VMware topology objects such as clusters,
folders, or datacenters may inform investigation only when they can be
attached honestly to canonical `agent`, `vm`, or `storage` resources; the
collector must not solve that ambiguity by creating VMware-only top-level
incident targets.
That same monitoring boundary now also has a concrete detail-enrichment seam.
`internal/vmware/client.go`, `internal/vmware/client_topology.go`, and
`internal/vmware/provider.go` may use the official vCenter Automation API plus
VI JSON `name`, `parent`, `runtime`, `resourcePool`, `datastore`, `host`,
`vm`, and datastore-summary paths to enrich canonical VMware-backed resources
with placement, guest identity, and storage consumer context. That
enrichment remains best-effort provider detail on the shared VMware source: it
must not create a second topology cache, a VMware-only placement store, or a
parallel guest-identity model outside the canonical `agent` / `vm` / `storage`
resource graph.

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
Alert arrays are the explicit freshness exception inside that remaining dual
truth. Monitoring APIs that still serve `StateSnapshot` must project
`ActiveAlerts` and `RecentlyResolved` from the live alert manager at read time
instead of trusting the cached snapshot fields, so externally served alert
counts and recently resolved incidents do not lag behind acknowledgement,
resolve, or clear operations between explicit sync points.
The container entrypoint in `docker-entrypoint.sh` now also lives under this
boundary. Hosted or managed tenant bootstrap changes must preserve safe startup
when immutable read-only mounts are layered into `/etc/pulse`; the entrypoint
may not reintroduce ownership mutation against those read-only files during
container boot.
That same monitoring boundary now also owns Docker Swarm runtime truth at the
collection seam. `internal/dockeragent/swarm.go` is the canonical manager-side
filter for live Swarm services and tasks, so monitoring consumers do not ingest
historical shutdown tasks as if they were still part of the active runtime.

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
That same monitoring-owned disk merge path must also treat host-agent SMART
attributes as canonical fill data for the Proxmox disk view. When a linked
host agent reports SMART health or NVMe `percentage_used` for a physical disk
that Proxmox itself exposes without trustworthy health or wearout, the merge
path in `internal/monitoring/monitor.go` must promote that data into the
canonical physical-disk model and the Proxmox polling runtime in
`internal/monitoring/monitor_pve.go` must evaluate disk alerts only after that
merged disk view exists, so controller-backed disks do not lose health and
endurance coverage between collection and alerting.
That same Proxmox monitoring boundary also owns checked response parsing for
polymorphic numeric fields. Shared client parsers such as
`pkg/proxmox/replication.go` must use the package's checked integer conversion
helpers instead of direct casts, so malformed or oversized Proxmox values do
not overflow into monitoring state.

Backup polling and recovery guest identity assembly now derive workload node,
name, and type context from canonical `ReadState` instead of from
snapshot-owned VM/container arrays, so storage backup polling, guest snapshot
polling, timeout sizing, PBS recovery candidate assembly, and Proxmox recovery
ingest all follow unified runtime truth when a live resource registry exists.
That same monitoring-owned workload boundary now includes canonical app
workloads projected through unified resources, not only VM/LXC-style guests.
Consumers that need runtime workload truth must treat `ReadState.Workloads()`
as the cross-platform workload surface for VMs, system containers, docker
containers, and API-backed app containers such as TrueNAS apps instead of
assuming workload views stop at traditional guest types.
Typed unified-resource views also need to present canonical monitoring truth,
not raw ingest formatting. Linked topology accessors exposed through
`internal/unifiedresources/views.go` must trim outer whitespace before
returning linked agent, node, VM, or container IDs so downstream consumers do
not observe `" node-99 "` style drift when the canonical linkage is `node-99`.
Source-owned IDs exposed through those same typed views must also trim outer
whitespace before they reach monitoring consumers, so a docker host, VM, node,
or storage view cannot appear to carry a different source identity just
because the ingest payload wrapped the source ID in spaces.
That same monitoring-owned Docker ingest path must also preserve persisted
container metadata across routine container recreation. When
`ApplyDockerReport` observes the same canonical docker host reporting a new
runtime container ID under the same normalized container name, monitoring must
copy custom URL, description, tags, and notes metadata onto the new container
ID instead of dropping that operator state on ordinary container replacement.
If multiple prior containers normalize to the same name, the migration must
fail closed and skip the copy rather than guessing between ambiguous sources.
Name normalization for that contract must treat Docker's leading `/` prefix as
presentation noise rather than identity, so routine recreate flows keep
metadata continuity when one report spells the same container as `/app` and a
later report spells it as `app`.
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
That same guest-memory boundary also owns the low-trust Proxmox status-memory
selector. When cache-aware availability is unavailable, the shared selector in
`internal/monitoring/guest_memory_sources.go` must derive `status-freemem`
against the effective balloon total and prefer that fallback over `status-mem`
when Proxmox reports a saturated or materially inconsistent used figure, so
Windows and ballooned guests do not get pinned to false 100% usage samples.
That same guest-memory boundary also owns fallback order and cache scoping for
Proxmox VMs when `MemInfo` is absent. Monitoring must try instance-scoped RRD
`memavailable`, then guest-agent `/proc/meminfo` via the shared Proxmox
client, and only then linked host-agent memory as the final fallback. Both RRD
and guest-agent fallback caches must key on `(instance, node, vmid)` instead
of raw `node/vmid`, so separate Proxmox instances cannot leak stale or foreign
memory evidence into each other just because they reuse the same node name and
VMID.
That same guest-memory boundary also owns stabilization when Proxmox falls
back to low-trust VM full-usage readings. The shared VM polling paths must use
the previous guest diagnostic snapshot, not the resource model, to decide when
one more `previous-snapshot` carry-forward is justified. A live guest-agent
signal is sufficient healthy evidence for that decision even before disk or
network enrichment finishes, and the preserved result must be recorded with an
explicit snapshot note so diagnostics can distinguish deliberate stabilization
from ordinary fallback.
Guest-disk continuity now follows the same canonical rule. The shared VM
polling paths must classify guest-agent disk failures consistently, surface the
resulting disk-status reason on the VM model, and only carry forward previous
disk usage when the last VM snapshot is still recent guest-agent truth rather
than an already carried-forward fallback. That keeps transient guest-agent or
status-call failures from regressing a VM back to misleading allocated-disk
data while still avoiding indefinite replay of stale disk summaries.
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
available. When the runtime must fall back beyond the linked host and `rootfs`
paths, it must treat the raw `/nodes` disk figure as low-confidence and prefer
the canonical local system storage owner instead of whichever mounted storage
is merely present or largest. On multi-storage Proxmox hosts, fallback
selection must rank `local-zfs`, `local-lvm`, `local`, and other non-shared
guest-root storages ahead of backup-only mounts, and storage-derived disk
metrics may override the `/nodes` figure only when that figure is the active
source or node disk truth is otherwise absent.
TrueNAS monitoring ownership now also includes provider rebind semantics in
`internal/monitoring/truenas_poller.go`. When a stored TrueNAS connection's
host, auth, TLS, or fingerprint settings change, the poller must replace the
live provider instance instead of keeping stale connection state in memory
until the process restarts.
That same monitoring boundary now also owns canonical per-connection poll
health and discovered-summary state for the settings platform-connections
surface. `internal/monitoring/truenas_poller.go` must honor each configured
TrueNAS connection's `pollIntervalSeconds`, keep the next poll schedule plus
last success/failure state in one canonical runtime owner, and project the most
recent discovered host/pool/dataset/app/disk/recovery counts there instead of
recomputing settings health panel-by-panel. That same poller-owned summary must
also absorb manual saved-connection test results from the shared
`POST /api/truenas/connections/{id}/test` path, so row-level operator tests in
settings update the canonical last success / last error state instead of
stopping at disconnected toast notifications.
That same runtime owner also defines the feature-default contract for TrueNAS:
the API-backed integration is on by default, and `PULSE_ENABLE_TRUENAS` is an
explicit opt-out switch rather than a required bootstrap toggle.
That same monitoring boundary now also owns live TrueNAS disk temperatures.
`internal/truenas/client.go` and `internal/truenas/provider.go` must ingest
`disk.temperatures` from the TrueNAS API, fall back to modern
`reporting.get_data` `disktemp` when the dedicated endpoint is unavailable, and
project those readings into the canonical physical-disk model and risk path
instead of leaving temperature telemetry agent-only or adding a TrueNAS-local
presentation shim.
That same monitoring boundary also owns SMART-backed TrueNAS disk risk
projection. When TrueNAS raises disk-local SMART alerts such as
`truenas_smart`, `internal/truenas/provider.go` must fold that incident truth
into the canonical physical-disk risk payload instead of leaving SMART failure
state trapped in incident/status-only decorations that storage consumers do
not read.
That same boundary now also owns recent aggregate TrueNAS disk temperature
history. `internal/truenas/client.go` must ingest `disk.temperature_agg`, and
`internal/truenas/provider.go` must project the returned min/avg/max readings
onto the shared `physicalDisk.temperatureAggregate` contract so disk-health
consumers can reuse one canonical metadata shape instead of inventing a
TrueNAS-only history payload.
That same boundary now also owns the canonical disk-history write path for
API-backed disks. `internal/monitoring/monitor.go` must sync non-native
physical-disk resources such as TrueNAS disks into the shared `disk`
metrics-store contract via the existing SMART-temperature writer, so physical
disk charts and disk-health consumers read one history path instead of a
TrueNAS-only temperature cache.
That same TrueNAS monitoring ownership also includes runtime mock continuity.
When `/api/system/mock-mode` changes on a live server, the TrueNAS supplemental
provider must rebind immediately and repopulate the canonical read state so
settings, infrastructure, storage, and other shared consumers see the same
mock-backed inventory without restart.
That same runtime mock ownership now also includes fixture authority. Mock
TrueNAS and VMware inventory plus mock metrics-history seeding must derive from
one shared platform fixture owner in `internal/mock/` so settings payloads,
supplemental ingest, unified read-state, and seeded charts cannot drift from
each other when the v6 runtime runs in mock mode.
That same fixture authority now also includes legacy snapshot-backed platforms.
`internal/monitoring/monitor.go` and
`internal/monitoring/mock_metrics_history.go` must treat the canonical
`internal/mock/fixture_graph.go` runtime graph as the one mock owner for
legacy Proxmox/Docker/Kubernetes/agent/PBS/PMG snapshot state plus
provider-backed TrueNAS and VMware fixtures. Monitoring must not rebuild mock
provider context from standalone defaults, consume partial legacy helper
exports, or mix snapshot state with separate provider fixtures when seeding
read-state or metrics history. The graph and its methods are the canonical
mock runtime API.
That same boundary now also owns native disk-history fallback when Pulse's own
history is shallow. `internal/truenas/client.go`,
`internal/truenas/provider.go`, `internal/monitoring/truenas_poller.go`, and
`internal/monitoring/monitor_metrics.go` must route TrueNAS `disktemp`
reporting history through the shared physical-disk chart path, so canonical
disk charts can render real provider-backed history instead of flat padding
after restarts or immediately after onboarding.
That same monitoring boundary now also owns modern TrueNAS app workload
telemetry. `internal/truenas/client.go`, `internal/truenas/provider.go`, and
`internal/monitoring/monitor.go` must ingest `app.stats` through the official
`/api/current` JSON-RPC websocket transport, project those readings onto the
canonical `app-container` metrics contract, and sync them into the existing
guest metrics-history/store path. Pulse must not add a TrueNAS-only charts
lane for that telemetry.
That same monitoring boundary now also owns connected-infrastructure
projection for API-backed platforms. `internal/monitoring/connected_infrastructure.go`
must project TrueNAS into the canonical connected-infrastructure surface list,
carry TrueNAS hostname/version through the shared top-level system grouping,
and preserve platform-managed surfaces such as `proxmox`, `pbs`, `pmg`, and
`truenas` when host telemetry is ignored. Ignore/remove semantics on that
surface remain machine-scoped and may only strip the local `agent`, `docker`,
and `kubernetes` reporting surfaces from the grouped row.
path or treat API-backed app workloads as second-class compared with native
Docker reports.
That same boundary now also owns native host-history fallback for API-backed
TrueNAS systems. `internal/truenas/client.go`,
`internal/truenas/provider.go`, `internal/monitoring/truenas_poller.go`, and
`internal/monitoring/monitor_metrics.go` must route TrueNAS
`reporting.get_data` system history through the shared `agent` guest-chart
path, so canonical host charts can show real provider-backed CPU, memory,
network, and disk throughput history when Pulse's own local history is still
shallow.
That same monitoring boundary now also owns canonical TrueNAS app control
refresh semantics. `internal/truenas/provider.go` and
`internal/monitoring/truenas_poller.go` must execute native app start/stop
actions through the owned TrueNAS runtime and refresh cached records and
recovery ingest immediately afterward, so assistant-driven app control does
not rely on stale provider state or ad hoc config-local action paths.
That same monitoring boundary now also owns canonical TrueNAS app log reads.
`internal/truenas/client.go`, `internal/truenas/provider.go`, and
`internal/monitoring/truenas_poller.go` must read bounded app-container logs
through the owned `/api/current` JSON-RPC runtime and tenant-scoped poller
selection path, so assistant-driven diagnostics do not depend on the unified
agent or a parallel config-local read path.
That same monitoring boundary now also owns canonical TrueNAS app
configuration reads. `internal/truenas/provider.go` and
`internal/monitoring/truenas_poller.go` must serve API-backed app-container
runtime/config shape through the same tenant-scoped provider snapshot and app
selection path used for control and logs, so assistant config reads do not
fork into a separate ad hoc fetch path or stale config cache.
That same monitoring boundary now also owns API-backed TrueNAS system
telemetry for the top-level NAS host. `internal/truenas/client.go` must ingest
`reporting.realtime` through the official `/api/current` JSON-RPC websocket
transport, `internal/truenas/provider.go` must project those readings onto the
canonical host `AgentData` and shared `ResourceMetrics` contract, and
`internal/monitoring/monitor.go` must sync them into the existing `agent`
metrics-history/store path. Pulse must not add a TrueNAS-only top-level
system charts path or leave TrueNAS host telemetry outside the canonical host
history contract.
That same monitoring boundary now also owns API-backed TrueNAS CPU
temperature. `internal/truenas/client.go` must use the modern
`reporting.get_data` RPC surface to derive current `cputemp` readings in the
same RPC session as system telemetry, and `internal/truenas/provider.go` must
project those readings into the canonical host temperature and host-sensor
contract. Pulse must not treat TrueNAS CPU temperature as an agent-only
capability or invent a TrueNAS-local sensor payload.
Taken together, this is the current monitoring-owned TrueNAS floor: one stored
API connection can surface one canonical top-level system, shared host
telemetry/history, app-container workloads, disk health/history, and
per-connection poll health without requiring the unified agent. The same
poller/provider path also owns assistant-driven app start/stop, logs, and
config refresh for those canonical workloads. Pulse does not promise a
separate TrueNAS runtime model, broader NAS administration, or agent-required
bootstrap at this floor.
That same monitoring boundary now also owns VMware signal enrichment on the
canonical alert timeline. `internal/vmware/client_signals.go`,
`internal/vmware/provider.go`, and `internal/monitoring/monitor_alerts.go`
may collect VI JSON overall status, active alarms, recent tasks, and VM
snapshot counts, but they must project those reads onto shared canonical
resources plus shared alert/resource history metadata instead of persisting a
VMware-only signal cache, event log, or provider-specific incident timeline.
That same monitoring boundary now also owns VMware recent-task and recent-event
breadcrumbs on the shared canonical resource timeline. `internal/vmware/`
provider code plus `internal/monitoring/vmware_poller.go` and
`internal/monitoring/monitor.go` may emit read-only `activity` changes through
the shared supplemental-ingest path, but those entries must land in the same
canonical `resource_changes` store used by every other resource timeline read.
Pulse must not add a VMware-only task/event table, replay log, or provider
history reader just because the VI JSON event surfaces differ from alert and
metrics collection.
That same monitoring boundary now also owns VMware performance telemetry on
the shared chart/history paths. `internal/vmware/client_metrics.go` must use
the VI JSON `PerformanceManager` read surfaces to resolve current-support,
available counters, and current samples from the supported `vCenter` release
floor; `internal/vmware/provider.go` must project ESXi host readings onto
canonical `agent` `ResourceMetrics` and VM readings onto canonical `vm`
`ResourceMetrics`; and `internal/monitoring/monitor.go` must sync those
metrics into the existing shared `agent` and `vm` history stores. Pulse must
not add a VMware-only charts cache, host history model, or VM metrics store
just because vSphere performance collection uses a different API family from
inventory and alarm reads.
That same monitoring boundary now also owns Proxmox guest-agent continuity
when `/status` is transiently missing. Recent guest-agent evidence and the
shared guest metadata cache must keep VM network and identity metadata alive
long enough to survive short Proxmox status failures, while incomplete
guest-agent metadata stays on a short retry cadence instead of freezing
partial VM summary data for minutes.
