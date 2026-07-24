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
Monitoring also owns the distinction between Proxmox VM power state and QEMU
guest-agent reachability: fresh or never-healthy VMs with an enabled but
unavailable guest agent stay `not-running`, while only VMs with recent healthy
guest-agent evidence may become `expected-unreachable`.
Monitoring owns source freshness cadence for Proxmox, PBS, and PMG resources:
the stale threshold is derived from the configured polling interval with a
minimum floor, so API-facing resource status must not degrade merely because a
healthy source is between normal poll cycles.
Proxmox guest enumeration is a generation boundary. VM and LXC collection and
enrichment must finish before one `State.UpdateGuestsForInstance` publication,
so readers never observe a VM-only or LXC-only intermediate snapshot. A failed
online cluster member retains only that member's last coherent guests and their
source-native `{instance}:{node}:{vmid}` IDs; a successful empty member
enumeration is authoritative and removes genuinely deleted guests. Collection
failure remains visible through source freshness/error state and must not be
converted into an authoritative empty inventory.
Host-agent report liveness is server-observed, not agent-clock-observed:
`ApplyHostReport` must stamp `Host.LastSeen`, agent-sourced Ceph cluster
freshness, and host-agent cluster sensor freshness from Pulse receipt time, so
a reporting machine with a slow or fast local clock cannot be ingested as stale
or keep offline/recovery alerts flapping while reports are still arriving.
Report ordering is a separate, source-authored contract. Current agents publish
a process-unique stream plus monotonic sequence, and monitoring must serialize
each host's complete accept-and-apply transition, reject duplicate/older or
retired-stream reports without replacing state or writing metrics/history/
alerts, and still advance receipt-time liveness for every authenticated
arrival. Transport activity must not extend `Host.LastSeen`, the accepted
telemetry lease, or alert recovery when the payload itself is rejected. The
accepted ordering watermark, reporting interval, accepted receipt time,
transport receipt time, and observation time are durable host continuity so a
server restart cannot let a delayed buffered report resurrect an operation
that a newer report already stopped.
Legacy reports without a sequence retain timestamp-based reconnect-burst
protection, but a clock correction after a normal report interval must be
admitted rather than freezing telemetry indefinitely.
Host telemetry also has a reporting lease derived from the agent cadence.
Monitoring must not clear a genuinely active Unraid parity operation or Linux
RAID rebuild during an ordinary polling gap. Once the lease expires, it must
clear transient operation/progress fields from host and canonical resource
projections while retaining static topology/health evidence that remains in
the live last-known host snapshot. After a server restart, durable continuity
must still expire persisted operation alerts on the same accepted-telemetry
lease; missing telemetry then remains visible through the separate confirmed
connectivity lifecycle.
PBS backup snapshot refresh is a bounded monitoring hot path: group-level
snapshot fetches must run through the fixed worker pool in
`internal/monitoring/monitor_backups.go`, reuse cached snapshots on per-group
fetch failures, and must not allocate one goroutine or buffered result slot per
backup group in large PBS datastores. Live PBS backup state is intentionally
bounded: groups are processed newest-first, per-group snapshots are capped to
the newest bounded set of real fetched snapshots, and the per-instance PBS
backup list must not grow without an explicit monitoring-owned limit. Bounding
must never synthesize placeholder backup entries from group metadata: a
placeholder drops verification, size, file, and per-snapshot time data, which
users read as broken discovery and failed verification. PBS backup group cache metadata
must be pruned to the retained group set after a completed poll, while
preserving cache metadata only for groups still observed or intentionally reused
after transient datastore failures. Recovery-point ingestion started by backup
polling must be serialized and coalesced so slow store writes cannot retain one
full backup point batch per poll cycle. Complete authoritative enumerations
coalesce only within the same provider, ID-prefix, and instance scope; distinct
source scopes and non-reconciling event batches remain FIFO so bounding memory
does not discard independent recovery facts.
Removed host-agent reconnect blocks are identity-scoped: matching may use the
canonical host ID or token-qualified machine/hostname continuity, but must never
block a distinct live host by hostname alone.
Proxmox cluster node labels are monitoring-owned identity presentation:
configured cluster labels must project into node display names as
`cluster label (node name)` so duplicate node hostnames across clusters remain
distinguishable, while `models.Node.Name` and `ProxmoxData.NodeName` keep the
raw Proxmox node identity for linking, metrics, URLs, and actions.
Node state aggregation is cluster-identity-scoped beyond presentation: a
hostname match alone must never bind a node to a host agent, and neither a
shared linked-agent identity nor an endpoint merge alias may fold two node
slots into one, when the two sides carry contradicting non-empty cluster
names. A hostname-based agent match is also rejected when the node's endpoint
IP is absent from the candidate agent's reported IPs. Nodes with empty
cluster names keep merging into their cluster view so standalone and
not-yet-classified endpoint views of the same machine still fold together.
Docker and Podman container CPU collection preserves the runtime-native raw
per-core CPU percent, but monitoring-owned history and alert threshold
evaluation use host-capacity-normalized CPU percent when host CPU capacity is
known. Raw runtime CPU remains alert/resource metadata, not the canonical
threshold value.
Docker and Podman container OOM state is runtime-authored evidence, not an
exit-code inference. Current agents must publish Docker inspect's `OOMKilled`
boolean for every inspected container, including explicit `false`; monitoring
must preserve that nullable boolean through report ingest, internal/frontend
models, and unified resources. An absent value means an older or reduced-fidelity
report did not provide the evidence and must remain distinguishable from both
confirmed OOM and confirmed non-OOM state. Exit code 137 proves only SIGKILL and
must not be promoted into OOM truth by monitoring.
Docker host identity collapse must be surfaced, not silently absorbed. Docker
report ingest keys host identity on the agent-reported machine ID in unified
mode, so cloned VMs that still share `/etc/machine-id` fold into one
`models.DockerHost` whose reports alternately overwrite each other (#1584).
Ingest must watch each resolved host identity for identity-field revisits: a
reported hostname (or machine ID) that switches away and returns to a value
already observed inside the monitoring-owned flap window proves two machines
share the identity, while a one-time hostname rename never revisits and must
not be flagged. An active conflict is published as
`models.DockerHost.IdentityConflict` carrying the flapping values so
downstream surfaces can warn, and it must clear on its own once only one
machine keeps reporting for the window. Monitoring must not auto-split the
collapsed identity: the machine ID is the identity key, and the remedy
(regenerating the clone's machine-id) belongs to the operator.
Unified Agent module projection is report-authored and additive. When one
agent sends host and Docker reports for the same machine identity, monitoring
must refresh one canonical machine carrying both facets so the Hosts and
Docker typed views expose the same canonical ID. A deliberately Docker-only
agent sends no host report and must remain absent from the Hosts view rather
than gaining a synthetic host facet from Docker telemetry.
Proxmox read-state rehydration is the inverse boundary: canonical
unified-resource CPU metrics are 0..100 percentages, while legacy
`models.Node.CPU`, `models.VM.CPU`, and `models.Container.CPU` remain Proxmox
0..1 ratio fields. Monitoring-owned read-state conversion must divide canonical
Proxmox node, VM, and LXC CPU percentages before handing them back to legacy
snapshot/current-row paths.
Proxmox guest live state, alerts, and history share one guest CPU-percent
normalizer. Efficient cluster polling and traditional per-node polling must
write VM/LXC history under the Proxmox guest ID in that same 0..100 unit, with
no core-count division and no in-guest host-agent substitution. A linked host
agent may supplement metrics the platform does not provide, but guest CPU and
its `vm` / `system-container` history target remain platform-owned so dashboard,
details, API state, alerts, and history cannot select different authorities.
Proxmox guest disk and network throughput has one cumulative-counter sampling
contract. `diskread`, `diskwrite`, `netin`, and `netout` are cumulative bytes;
the canonical rate is `(current counter - previous counter) / elapsed
observation seconds`, in bytes per second, with no 1024 divisor. Elapsed time
comes from the receipt time stamped immediately after the relevant Proxmox API
response is decoded, not from later guest-agent, filesystem, or metadata
enrichment. Each counter keeps an independent adjacent-sample baseline:
explicitly unchanged counters produce a valid zero, missing/null fields produce
unknown, out-of-order samples produce unknown without moving the baseline, and
a counter decrease caused by restart, reconnect, migration epoch change, or
wrap rebases that counter and produces a valid zero for the reset interval. A
source-uptime rollback rebases the complete counter epoch and leaves the first
post-restart rate unknown, including when a busy guest already surpassed its
pre-restart counter value before the next poll.
First-sample and missing-field unknowns remain internal validity state; the
legacy API/websocket guest number fields stay numeric, while history, unified
metrics, and alerts omit the unknown observation instead of manufacturing
zero. The rate-tracker identity is `(configured PVE instance, guest kind,
VMID)`: it survives node migration, separates QEMU from LXC, and prevents
duplicate configured cluster identities from sharing a concurrent baseline.
Idle and partial samples still refresh tracker liveness.
Proxmox row liveness uses the same cadence-derived threshold as source
freshness (`max(2 * configured poll interval, 60s)`). Node offline grace and
guest preservation must not expire between healthy 60- or 90-second polls, and
must not use a separate fixed 60-second timer.
Tenant monitor enumeration is monitoring-owned runtime topology, not a
reporting source of truth. `MultiTenantMonitor.ListOrganizationIDs` may expose
persisted organization IDs to API-owned background workers, but it must not
initialize monitors, start pollers, or reinterpret tenant IDs as monitored
resource health.
Proxmox physical-disk polling is also a continuity boundary. A failed or
permission-denied `disks/list` call must remain an error so the monitor can use
linked host-agent inventory or retain same-instance, same-node prior evidence;
it must never become a successful empty inventory that removes valid boot or
data disks. SMART enrichment matches serial, WWN, device path, and controller
member topology uniquely and fail-closed, rejects placeholder hardware
identifiers, preserves explicit failure over a later coarse healthy value, and
lets explicit SMART endurance replace contradictory Proxmox wearout. Missing
permission, ambiguous identity, standby, and absent SMART fields remain
neutral rather than borrowing telemetry from another disk.
Negative percentage-used counters remain unknown; values above 100 clamp to
exhausted before deriving remaining life, so invalid or over-limit controller
data cannot wrap into a fabricated healthy value.
Proxmox cluster API polling has one configured connection authority: the
operator-saved `PVEInstance.Host` and its single credential set. Auto-discovered
member/corosync addresses remain ordered failover candidates and direct
reachability evidence; they are not per-node API connections or credentials and
must not randomly displace a healthy configured authority. When the authority
is healthy, recovery checks for unreachable members run bounded and
asynchronously so snapshots, storage content, replication, and other API-only
data do not wait on cluster-private addresses. When no endpoint is healthy,
recovery remains synchronous so a reachable member can restore service.
Periodic cluster discovery refreshes changed member addresses and rebuilds the
failover client. Pulse reachability evidence survives that reconciliation only
when the member's effective dial URL is unchanged; a network move resets the
old result until the new target is checked. Infrastructure Settings presents
one cluster-level API source while retaining member addresses and their
node-local Agent evidence.
Proxmox cluster membership is not the `/nodes` telemetry slice. A quorate,
complete `/cluster/status` response is absence-authoritative; members present
there but missing from `/nodes` remain in `models.State` with their stable
identity and last-known linkage while live CPU/uptime is cleared and
connection state is offline or stale. Failed, incomplete, non-quorate, or
cluster-identity-mismatched membership reads retain the last-known
node/endpoint union, break any pending absence sequence, and never advance
deletion. A member absent from a healthy
authoritative membership read is retired only after two consecutive
confirmations; the first omission remains in durable endpoint configuration so
a monitor restart resets the confirmation window rather than converting
uncertainty into removal. A newly reported member is admitted immediately.
Cluster display names are not global identity: config consolidation requires
overlapping endpoint authority, and different provider instances with the same
cluster/member names stay distinct in node and storage identity.

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
18. `internal/monitoring/monitored_system_usage.go`
19. `internal/dockeragent/swarm.go`
20. `internal/dockeragent/collect.go`
21. `pkg/proxmox/ceph.go`
21a. `pkg/proxmox/cluster_client.go`
21b. `pkg/proxmox/client.go`
21c. `pkg/proxmox/io_counters.go`
22. `pkg/proxmox/zfs.go`
23. `internal/monitoring/guest_memory_sources.go`
24. `internal/monitoring/guest_memory_stability.go`
25. `internal/monitoring/monitor_polling_vm.go`
26. `internal/monitoring/monitor_pve_guest_builders.go`
27. `internal/monitoring/monitor_pve_guest_poll.go`
28. `internal/monitoring/guest_disk_stability.go`
29. `internal/monitoring/mock_metrics_history.go`
30. `internal/monitoring/mock_chart_history.go`
31. `internal/monitoring/availability_poller.go`
32. `internal/monitoring/scheduler.go`
33. `internal/monitoring/docker_detection.go`
34. `internal/monitoring/monitor_polling_containers.go`
35. `internal/mock/fixture_graph.go`
35a. `internal/mock/action_fixtures.go`
35b. `internal/mock/availability_fixtures.go`
36. `internal/dockeragent/docker_client.go`
37. `pkg/agents/docker/report.go`
38. `internal/models/models.go`
38a. `internal/models/proxmox_guest_state.go`
38b. `internal/models/metrics_types.go`
39. `internal/models/models_frontend.go`
40. `internal/models/converters.go`
41. `internal/models/deepcopy.go`
42. `internal/mock/generator.go`
43. `internal/mock/demo_scenarios.go`
44. `internal/kubernetesagent/agent.go`
45. `pkg/agents/kubernetes/report.go`
46. `internal/monitoring/temperature.go`
47. `internal/truenas/client.go`
47a. `internal/truenas/transport.go`
48. `internal/truenas/disk_health.go`
49. `internal/truenas/provider.go`
50. `internal/models/ceph_cluster_identity.go`
51. `internal/truenas/types.go`
52. `internal/monitoring/monitor_alert_sync.go`
53. `internal/monitoring/platform_poller_shared.go`
54. `internal/monitoring/monitor_backups.go`
55. `internal/monitoring/resource_stale_thresholds.go`
56. `internal/monitoring/recovery_ingest.go`
56a. `internal/monitoring/pbs_protection_observation.go`
57. `internal/monitoring/multi_tenant_monitor.go`
58. `internal/monitoring/proxmox_action_observer.go`
59. `internal/monitoring/agent_fleet_doctor.go`
60. `internal/config/host_continuity.go`
61. `internal/monitoring/docker_metadata_migration.go`
62. `internal/monitoring/kubernetes_metadata_migration.go`
63. `internal/monitoring/metadata_stores.go`
63a. `internal/config/docker_metadata.go`
63b. `internal/config/guest_metadata.go`

## Shared Boundaries

1. `internal/config/host_continuity.go` shared with `agent-lifecycle`: the durable host identity, report-order watermark, and removal tombstone journal is jointly owned by agent lifecycle admission and monitoring report continuity.
2. `internal/kubernetesagent/agent.go` shared with `agent-lifecycle`: the Kubernetes native agent runtime is both a monitoring inventory source and an agent lifecycle Pulse control-plane transport client.
3. `internal/models/models.go` shared with `agent-lifecycle`: removed host-agent identity aliases and tombstone state are both agent lifecycle authority and monitoring runtime report state.
4. `internal/monitoring/monitor.go` shared with `agent-lifecycle`: monitor construction owns both monitoring runtime initialization and fail-closed agent lifecycle journal hydration before report admission.
5. `internal/monitoring/monitor_agents.go` shared with `agent-lifecycle`: server-side Unified Agent report, removal, token binding, tombstone expiry, and re-enrollment semantics are jointly owned by agent lifecycle authority and monitoring ingest.
6. `internal/proxmoxidentity/backup_identity.go` shared with `alerts`, `storage-recovery`: Proxmox PBS backup subject identity is a shared runtime boundary for monitoring backup freshness, backup-age alert attribution, and recovery-point guest mapping.
7. `pkg/agents/host/report.go` shared with `agent-lifecycle`: the Unified Agent host report is both an agent lifecycle authored-state contract and a monitoring ingest contract for host maintenance posture.

## Extension Points

1. Add pollers/providers and discovery-provider coordination through `internal/monitoring/poll_providers.go` and `internal/monitoring/monitor_discovery_helpers.go`
   The PVE/PBS/PMG providers share one wiring layer inside
   `internal/monitoring/poll_providers.go`: instance listing, instance
   description, and connection-status publication go through the generic
   `sortedClientNames` / `describeProviderInstances` /
   `providerConnectionStatuses` helpers, and the prefixed PBS/PMG providers
   are built by `newPrefixedPollProvider` from a `prefixedPollProviderSpec`.
   New scheduler-backed platform providers extend those helpers instead of
   re-rolling per-platform copies of the same loops.
   Source freshness thresholds for PVE/PBS/PMG resource ingestion are derived
   through `internal/monitoring/resource_stale_thresholds.go` from the active
   poll interval and passed into the unified-resource adapter. New pollers or
   config paths that change source cadence must update that derivation instead
   of hard-coding stale windows inside registry or API code.
   Periodic out-of-scheduler platform pollers (TrueNAS, VMware) share their
   lifecycle and config-resolution scaffold through
   `internal/monitoring/platform_poller_shared.go`: `startPollerLoop` owns
   the double-start guard, stopped-channel handshake, and sync+poll cadence,
   and `loadActiveInstanceConfigs` owns the "enabled instances keyed by
   trimmed connection ID with defaults applied" active-connection policy. A
   new platform poller of this family must reuse both rather than copying
   the TrueNAS/VMware loop, and traditional PVE guest polling records
   per-guest series through the canonical `recordGuestMetric` helper in
   `internal/monitoring/monitor_pve_guest_helpers.go` instead of inline
   metric writes.
   Discovery config and configured-host IP resolution must stay off the
   monitor lock. `internal/monitoring/monitor_discovery_helpers.go` exposes
   the canonical `discoveryConfigSnapshot()` that discovery providers consume,
   and both it and `getConfiguredHostIPs()` may take a brief `m.mu.RLock` only
   to deep-copy config before releasing it; configured Proxmox/PBS/PMG hostname
   resolution runs through the package-local `lookupConfiguredHostIP` seam
   outside the lock, so slow or blocked DNS cannot stall monitor writers or
   discovery subnet probes. The discovery `IPBlocklist` is the deduplicated
   merge of the operator-configured blocklist and the resolved configured-host
   IPs through `mergeDiscoveryIPBlocklist`, never one silently replacing the
   other, and `Start` / `StartDiscoveryService` must read the snapshot through
   that single helper instead of re-inlining the lock-and-clone path.
2. Add metrics capture or history-retention behavior through `internal/monitoring/metrics.go` and `internal/monitoring/metrics_history.go`
3. Add typed read access through `internal/unifiedresources/views.go`
4. Add unified supplemental ingest through `internal/monitoring/poll_providers.go`
5. Add or change container startup ownership/bootstrap behavior for hosted or managed Pulse runtime mounts through `docker-entrypoint.sh`
6. Add or change Docker Swarm manager service, task, node, secret, or config runtime collection through `internal/dockeragent/swarm.go`
   Swarm node inventory is manager-sourced through the documented nodes API
   when available and falls back to the local `system/info` Swarm node
   metadata when a worker or non-manager runtime cannot list cluster nodes.
   Manager-side list failures are warnings, not host-report failures.
   Swarm secret and config inventory is metadata-only: the collector may
   preserve object id, name, labels, driver/template metadata, and timestamps,
   but it must never copy or serialize secret/config payload bytes from the
   Docker API.
7. Add or change Docker or Podman container stats compatibility and runtime metric semantics through `internal/dockeragent/collect.go`
   Docker / Podman collection now owns native runtime inventory as well as
   container metrics. It may collect image summaries, volume summaries,
   network summaries, Swarm services, Swarm tasks, Swarm nodes, Swarm secrets,
   Swarm configs, and daemon storage-usage buckets from the documented runtime
   API, then publish those
   records through the Docker / Podman module report for unified-resource
   ingestion.
   Swarm service records must preserve documented service update status
   (`UpdateStatus.State`, message, and completion time when reported) so the
   container runtime surface can distinguish stable services from active or
   failed rollouts without inventing frontend-only state.
   Failures in image, volume, network, node, secret, config, or storage-usage
   collection are best-effort warnings and must not make the whole host report
   fail when container/runtime health data is otherwise usable. Podman libpod
   pods remain outside this collector until a libpod-native collector owns that
   API shape.
   Docker's native CPU convention reports 100% per CPU core. Agent reports and
   compatibility APIs may keep that raw value, but canonical monitoring history
   and Docker container CPU alerts must pass through the shared normalized
   capacity helper so an 80% threshold means 80% of the reporting host capacity,
   not 0.8 of one core on a multi-core host.
   Container OOM evidence must come from the inspected runtime state. The report
   wire field is nullable for compatibility with older agents, but a current
   collector must set it to the exact Docker inspect boolean even when false;
   report ingest and model conversion must clone and preserve the pointer so
   concurrent state replacement cannot alter previously accepted evidence.
8. Add or change Proxmox Ceph compatibility payload decoding through `pkg/proxmox/ceph.go`
9. Add or change Proxmox ZFS compatibility payload decoding and vdev-role normalization through `pkg/proxmox/zfs.go`
10. Add or change mock chart synthesis, seeded history continuity, or mock-owned
   chart fallbacks through `internal/monitoring/mock_metrics_history.go` and
   `internal/monitoring/mock_chart_history.go`
11. Honor the per-instance `Disabled` flag on PVE/PBS/PMG at poller client
   init, reconnect, and per-node iteration so disabled connections do not
   drive API calls, scheduler health, or surface ingest. Zero-value
   `Disabled=false` must remain the migration-safe default for existing
   `nodes.json` content; the poller must never create a client or mark an
   instance reachable when `Disabled` is true.
   Source-specific backup snapshot accessors in `internal/monitoring/monitor.go`
   are monitor-state read surfaces, not recovery-store projections. PVE backup
   consumers read `PVEBackupsSnapshot()`, while PBS artifact consumers read
   `PBSBackupsSnapshot()` so PBS size, protection, verification, file, owner,
   datastore, and namespace facts remain the live PBS poller result carried on
   `models.PBSBackup`.
   Proxmox PVE backup and guest snapshot polling in
   `internal/monitoring/monitor_backups.go` must consume the canonical
   `unifiedresources.ReadState` shape for guest, storage, and recovery mapping.
   If a store-backed read-state has not yet been refreshed for the PVE instance
   whose current monitor state already contains guests, backup/snapshot polling
   refreshes the canonical resource store from the current state and continues
   through the read-state interface. It must not fall back to direct legacy
   guest slices as the primary source of truth. Clustered PVE snapshot polling
   must therefore see guests collected earlier in the same cycle before calling
   the Proxmox guest snapshot APIs.
   Guest lookups handed from backup/snapshot polling to alert evaluation must
   preserve the canonical instance, node, VMID, type, live display name, and
   live guest tags from read-state. Monitoring must key snapshot lookups with
   the shared alert guest identity and must not downgrade the handoff to a
   name-only map, because ignored-name prefixes, `pulse-no-alerts`, ignored
   tags, and required-tag filtering are alerts-owned policies that require the
   same guest context as ordinary threshold evaluation.
   PBS backup snapshot refreshes in that same file must stay bounded by the
   package worker-pool constant and stream requests through workers instead of
   creating one goroutine per backup group; per-group API failures may reuse
   cached snapshots, but the polling loop must keep memory proportional to the
   worker count rather than datastore cardinality. Per-group cache timestamps
   must be removed when successful group discovery no longer retains that
   group, and recovery-store ingestion must be a bounded latest-batch pipeline
   rather than an untracked goroutine per poll. Latest-batch replacement applies
   only to complete enumerations with the same provider, ID-prefix, and instance
   scope; distinct scopes and event batches must remain independently queued.
12. Add or change agentless availability monitoring only through the
   poll-provider path. `internal/monitoring/availability_poller.go` owns ICMP,
   TCP, and HTTP probes, provider health, scheduler task construction, and
   supplemental unified-resource records for saved availability targets.
   Failed endpoint probes are observed runtime state for that target; they
   must publish provider health and incidents without dead-lettering the
   scheduler task itself.
13. Add or change broadcast resource projection through
   `internal/monitoring/monitor.go` and monitoring guardrails together.
   `/api/state` and websocket broadcasts must coalesce transient split host
   resources before serialization so a single Proxmox node with a reporting
   host agent remains one hybrid top-level system across rebuild ticks.
   High-frequency monitor ticker, mock-mode, and alert-resolution broadcast
   signals are current-state invalidations. They must call the WebSocket hub's
   lazy current-state broadcast path and let the hub resolve tenant-aware
   frontend state after coalescing; monitor callsites must not build or retain
   full frontend-state snapshots for supersedable broadcast signals.
14. Add or change Proxmox-side LXC Docker detection or inventory through
   `internal/monitoring/docker_detection.go`,
   `internal/monitoring/monitor_pve_guest_poll.go`,
   `internal/monitoring/monitor_polling_containers.go`, and monitoring
   guardrails together. Socket detection may only annotate LXC guests after
   explicit server opt-in. Both the efficient cluster/resources guest poll and
   the traditional per-node container polling path must run
   `CollectProxmoxGuestDockerInventory` after Docker presence detection and
   before updating container state, so the Docker runtime lens does not depend
   on which Proxmox polling path is active. LXC Docker inventory may only emit
   Docker / Podman
   module-compatible reports into `ApplyDockerReport`, must skip guests with a
   linked online guest-local host agent, and must keep the command set to
   minimal read-only Docker summary and aggregate stats collection. The socket
   probe must run its yes/no marker inside the target LXC through `pct exec`;
   host-side `pct` / `lxc-attach` failures are probe errors and must not be
   converted into cached `HasDocker=false` results. Negative Docker detections
   may be rechecked on a short cadence so command enrollment, daemon startup,
   or transient Proxmox access failures do not hide later inventory. Negative
   detections from before the current Docker checker configuration must be
   rechecked after monitor/router startup, so explicit guest Docker inventory
   can repopulate immediately after backend restarts instead of waiting for the
   normal negative-cache cadence.
15. Add or change mock-mode Discovery context through the canonical mock
    fixture graph. Mock Discovery records must be derived from the same authored
    state graph as mock nodes, guests, Docker hosts, containers, and Kubernetes
    workloads, then exposed through API-owned Discovery handlers. Monitoring
    must not create a second frontend-only fixture path for service versions,
    config paths, bind mounts, ports, or suggested URLs.
    Mock Docker runtime inventory must use the same authored Docker host graph
    for images, volumes, networks, engine storage-usage buckets, Swarm
    services, tasks, nodes, secrets, and configs so platform pages and browser
    proof exercise the live report/resource contracts rather than a
    frontend-only demo inventory.
    Mock Kubernetes clusters must likewise keep distinct display names,
    contexts, and server hints when the fixture graph authors multiple
    clusters, so platform Overview rows read as real cluster identities instead
    of duplicated placeholder labels. Recovery-point fixture assertions should
    verify readable `cluster/namespace/object` identity, not rely on one
    hard-coded cluster name.
16. Add or change TrueNAS supplemental inventory only through the native
    TrueNAS provider path and unified-resource projection. TrueNAS apps are
    API-owned application records: `app.query` is the live inventory source on
    the negotiated current transport, with legacy REST allowed only for a
    connection proven to run a release that predates the versioned API. The
    provider may preserve Docker-compatible runtime metadata for shared
    container tooling, but it must also publish the native app identity, state,
    version, update availability, workload containers, ports, images, volumes,
    networks, and stat collection metadata through `TrueNASData.App` on the
    canonical `app-container` resource.
    Monitoring must not rebuild a second Docker-only TrueNAS app inventory or
    make the Docker fallback the source of truth for the TrueNAS platform page.
    TrueNAS child source identities are appliance-local and must be scoped under
    the owning system source key before unified-resource ingest, so common pool,
    dataset, app, VM, share, and disk names from different appliances remain
    distinct. Mock fixture metrics and seeded/live history must use the same
    scoped source keys as the TrueNAS provider metrics targets.
    The owning system source key itself is connection-scoped: the poller
    constructs live providers through `NewLiveProviderForConnection` so
    `systemSourceID` keys the system (and every child scoped under it) by the
    configured connection ID, never by the snapshot-reported hostname, and the
    system's ingest identity carries no machine key (DMI serials are shared by
    DR clones and can be vendor placeholders). Two appliances that report the
    same hostname must remain distinct resources (#1573, #1575). The hostname
    arm of `systemSourceID` exists only for fixture snapshots, which carry no
    connection; mock-mode identities stay hostname-scoped through that arm.
    Regression coverage: `TestTrueNASPollerKeysSystemsByConnection` in
    `internal/monitoring/truenas_poller_test.go` and
    `TestRegistryIngestRecordsKeepsSameHostnameSystemsDistinct` in
    `internal/truenas/contract_test.go`. The canonical-ID migration semantics
    for rows minted under the retired hostname-keyed derivation live in the
    unified-resources contract (record-declared succession, item 27).
    Every live TrueNAS client owns one explicit, immutable transport decision.
    SCALE 25.04 and later must use JSON-RPC 2.0 over a TLS WebSocket at
    `/api/current`; authentication, authorization, TLS, protocol, or method
    failures on that endpoint are authoritative and must never wake the
    deprecated REST bridge. Only an unsupported-endpoint WebSocket handshake
    may trigger a REST `/system/info` version probe, and REST may then be
    selected only for recognized SCALE releases before 25.04 or TrueNAS
    CORE/FreeNAS. Unknown or current versions fail closed. The decision and
    persistent socket belong to one configured client, so reconnects or
    legacy negotiation for one appliance cannot alter another appliance.
    Current API-key authentication uses `auth.login_ex` with the key owner's
    username and `API_KEY_PLAIN`; password authentication uses
    `PASSWORD_PLAIN`. Username-less stored API keys may use the deprecated
    login method only as an upgrade bridge, with explicit remediation when a
    release removes that method. Read calls may reconnect with bounded backoff
    and replay once after a transport failure. Mutating app calls must never
    replay after dispatch because their outcome is ambiguous. Event reads must
    retain the `core.subscribe` ID, call `core.unsubscribe` before reusing the
    session, and discard a socket after a terminal stream read deadline.
    Connection summaries expose only secret-free transport mode, endpoint,
    TLS, authentication mechanism, appliance version, reconnect count, and
    last-error timing diagnostics.
    TrueNAS storage and alert inventory follow the negotiated transport:
    pools use `pool.query`, datasets use `pool.dataset.query`, disks use
    `disk.query`, and alerts use `alert.list`. Inventory readers must only consume fields the
    API actually serves on every supported TrueNAS line (CORE 13 REST-only
    included): `pool.dataset.query` carries no `mounted` field, so a listed
    dataset counts as mounted unless `locked` or an explicit value says
    otherwise; `disk.query` carries no health/status field and its
    `extra.pools` join cannot cross the REST bridge, so per-disk pool
    membership and ZFS member state derive from the vdev topology that
    `pool.query` attaches unconditionally; and `disk.temperatures` is a
    parameterized method the legacy REST bridge only serves as POST, while
    current releases use native JSON-RPC reporting without per-method REST
    fallback. Missing disk telemetry is reported as
    unknown, never as a failure signal. Regression coverage:
    `internal/truenas/client_api_shapes_test.go`. Unhealthy pool state
    from `pool.query` must emit a provider-native `zfs_pool_state` incident on
    the canonical pool resource when `alert.list` does not already provide a
    warning or critical pool alert for that same pool, so pool degradation does
    not depend on the TrueNAS appliance's own email or alert-delivery setup.
    Boot-pool inventory follows the separate `boot.get_state` contract because
    `pool.query` is not a reliable boot-topology source across supported CORE
    and SCALE releases. The client must merge that state into the
    connection-local pool list, preserve the boot-pool role, and derive
    path-only leaf devices without matching pool or disk identities across
    configured appliances.
    Read-only dataset health must retain replication intent from
    `replication.query`: `SET` and `REQUIRE` target roots may normalize
    receive-side read-only datasets and descendants only after the poller maps a
    local/PULL task or uniquely matches a remote PUSH target host to one
    configured connection. Missing or ambiguous remote identity fails closed,
    `IGNORE` never normalizes read-only state, and locked, unmounted, pool
    failure, disk failure, or unavailable state remains fault-bearing.
    TrueNAS VMs and network shares follow the same provider-owned inventory
    boundary: `vm.query` data publishes native `TrueNASData.VM` on canonical
    `vm` resources, while SMB/NFS share data from `sharing.smb.query` and
    `sharing.nfs.query` publishes native `TrueNASData.Share` on canonical
    `network-share` resources parented to the owning dataset or pool when the
    API/path supplies that evidence. TrueNAS protection inventory follows the
    same native-query rule: current connections prefer
    `zfs.resource.snapshot.query`, with `pool.snapshot.query` allowed only as a
    same-transport method-name compatibility path; version-gated legacy
    connections use REST. Replication tasks use `replication.query` on current
    connections.
    TrueNAS system services are also native appliance inventory: `service.query`
    is the preferred source for service name, boot enablement, runtime state,
    and process IDs. Pulse must publish that data through `TrueNASData.Services`
    on the owning top-level TrueNAS system resource instead of inventing a
    generic service resource type or rendering services as Docker/container
    rows.
17. Add or change provider supplemental platform activity through the
    provider-owned supplemental-change path and the canonical mock fixture
    graph together. vSphere task/event activity must be authored by the VMware
    provider or VMware mock fixture graph as `activity` resource changes with
    `platform_event` provenance, then recorded by monitoring's supplemental
    resource-change bridge. Monitoring must not create a frontend-only VMware
    activity fixture or bypass the unified resource-change store.
18. Add or change Kubernetes native API inventory through
    `internal/kubernetesagent/agent.go` and
    `internal/monitoring/kubernetes_agents.go`. The Kubernetes agent may read
    Namespaces, Services, ReplicaSets, StatefulSets, DaemonSets, Jobs,
    CronJobs, Ingresses, EndpointSlices, NetworkPolicies, PersistentVolumes,
    PersistentVolumeClaims, StorageClasses, ConfigMaps, Secrets, ServiceAccounts,
    Roles, ClusterRoles, RoleBindings, ClusterRoleBindings, ResourceQuotas,
    LimitRanges, PodDisruptionBudgets, HorizontalPodAutoscalers, and Events as
    bounded best-effort inventory. RBAC inventory (Roles, ClusterRoles,
    RoleBindings, ClusterRoleBindings) reports summary counts only — rule
    counts, subject counts, subject Kinds, and ClusterRole aggregation labels
    — so Pulse stays a "what permissions exist where" surface, not an RBAC
    enumeration tool. Full PolicyRule contents and individual subject names
    (User / Group / ServiceAccount) remain outside the report contract.
    ConfigMap and Secret payload values must not be collected for inventory.
    Current agents must prefer the Kubernetes metadata-only API path for
    ConfigMap and Secret inventory and mark those rows as metadata-only; older
    agent reports may still carry key names, but Secret values remain outside
    the report contract. Mock/demo Kubernetes ConfigMap and Secret inventory
    must mirror the current metadata-only trust boundary rather than seeding
    payload key names. Mock/demo Kubernetes inventory must also seed
    representative Service rows with ClusterIP, external IP, ServicePort,
    targetPort, nodePort, and selector metadata, plus Ingress, EndpointSlice,
    storage-class, persistent-volume, and persistent-volume-claim rows. They
    must also seed StatefulSet, DaemonSet, Job, and CronJob controller rows with
    their API-native target, current, ready/succeeded, availability, exception,
    service-name, schedule, and timing fields so the native services,
    networking, storage, and workload-controller tabs exercise the same
    report/resource contract as live agents. Deployment inventory must preserve
    Kubernetes object metadata creation time and `status.observedGeneration`
    from the agent report through monitoring models so the frontend can show
    API-native age and generation evidence instead of reconstructing those
    fields locally. Monitoring must preserve those objects as native cluster
    inventory instead of flattening them into pods, deployments, or generic
    networking, storage, configuration, or controller rows.
    Agent Fleet Doctor diagnostics must derive from the current monitoring
    `StateSnapshot`, agent-profile assignments, and profile deployment
    acknowledgements only. `internal/monitoring/agent_fleet_doctor.go` may
    explain liveness, version drift, identity splits, expected telemetry gaps,
    and profile drift, but it must remain read-only and must not become a
    separate collector, repair executor, or replacement for the canonical
    `/api/connections` fleet projection.
19. Add or change unified-resource alert synchronization through
    `internal/monitoring/monitor_alert_sync.go` and the alerts subsystem
    contract together. Monitoring may pass the current unified-resource snapshot
    into the alert manager, but threshold selection, override identity, active
    alert state, and notification delivery remain alerts-owned. The monitoring
    sync bridge must not introduce per-platform evaluator branches.

## Forbidden Paths

1. New consumer logic built directly on `Monitor.GetState()`
2. New runtime truth living only in `models.StateSnapshot`
3. Snapshot-backed helper paths used where `ReadState` should be authoritative

## Completion Obligations

1. Update this contract when monitoring truth ownership changes
2. Tighten guardrails when `GetState()`-centric paths are removed
3. Keep discovery-provider, host-agent ingest, guest-memory trust, metrics-history, storage-risk, Docker/Podman container collection, Docker report/model payloads, Proxmox Ceph and ZFS compatibility, Docker Swarm collection, mock runtime fixtures, and container bootstrap proof routes explicit in `registry.json`
4. Update related read-state or monitor tests when new collector paths land
5. Keep platform ingestion semantics aligned with
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`: hybrid is a
   declared ingestion mode on an admitted first-class platform, not a license
   to create new platform ids from secondary pollers or optional agent
   augmentation paths.
6. Preserve Proxmox storage backing-pool truth through the canonical storage
   poller path. `pkg/proxmox.Storage`, `internal/monitoring/monitor_polling_storage.go`,
   and the attached ZFS health model must carry the provider-reported `pool`
   field through to runtime storage snapshots and use it before name/path
   heuristics when matching ZFS pool health on multi-storage hosts.
   That same Proxmox compatibility boundary also owns top-level ZFS vdev-role
   normalization. Provider payload buckets such as `special`, `log`, `cache`,
   and `spares` may omit a concrete health state; `pkg/proxmox/zfs.go` must
   treat those blank-state grouping rows as role metadata rather than projecting
   operator-visible `UNKNOWN` failures unless the bucket or one of its children
   carries an actual degraded state or error count.
7. Keep Proxmox-side LXC Docker inventory privacy bounded. The monitoring path
   may collect Docker host/runtime summary, container ID/name/image/state/status,
   ports, and aggregate `docker stats` counters, but it must not run
   `docker inspect` or collect guest environment values, mount sources, files,
   container commands, or process details.
8. Keep TrueNAS app inventory native to the TrueNAS API projection. The
   monitoring/provider boundary may expose Docker-compatible fields for
   cross-runtime tooling, but platform-page app rows, source identity, and
   update posture must be carried by the TrueNAS app facet published into
   unified resources.
9. Keep TrueNAS network-share inventory native to the TrueNAS API projection.
   SMB/NFS shares must enter unified resources as `network-share` records with
   the TrueNAS share facet, not as generic storage rows or Docker/container
   compatibility records.
10. Keep the monitor-adapter rebuild lifecycle persisting canonical identity
    pins. `internal/unifiedresources/monitor_adapter.go` calls
    `PersistIdentityPins` after snapshot rebuilds and supplemental-record
    ingestion so canonical host IDs stay stable across restarts (see the
    unified-resources contract's durable identity-pin obligation). Rebuild
    paths added to the adapter must keep that persistence step; ephemeral
    snapshot-bridge adapters stay read-only.

## Current State

### PBS health is one completed-poll outcome

PBS client construction is transport setup, not connectivity evidence. Initial
client creation and retry recreation therefore remain pending until a poll
completes; client-construction failures publish a failed scheduler result
instead of leaving Settings pending while legacy state is disconnected.
`pollPBSInstance` finalizes its dynamic `pollErr` once and uses that outcome for
the scheduler ledger, staleness tracker, poll metrics, connection-health map,
dashboard `PBSInstance`, and legacy PBS alert evaluation. Authentication,
timeout, cancellation, and panic outcomes all publish `offline`/`error`, while
a later success clears the current error and publishes `online`/`healthy`.
Optional node, datastore, namespace, or job collection failures remain partial
data evidence and do not turn a successful version/datastore connectivity
probe into a connection failure.

### Host snapshots carry integration provenance; doctor copy is user-facing

`models.Host.IntegrationSource` mirrors the unified fabric's
`HostView.IntegrationSource()` discriminator on hosts produced by
`hostFromReadStateView` (the source behind `Monitor.HostsSnapshot()`); hosts
built from agent reports leave it empty by construction, so state-side host
records never claim integration provenance. Agent fleet doctor reason
messages are user-facing copy: stale detection now reads "No report has
arrived for 10m 2s. Pulse marks an agent stale after 5m without a report."
with durations humanized by `formatFleetDuration` ("45s", "5m", "10m 2s",
"1h 3m", "2d 4h") instead of Go's `5m0s` form, and the non-online status
reason quotes the reported status without leaking the internal
online/running/healthy vocabulary. The diagnostics subject set is unchanged:
`GetAgentFleetDiagnosticsForTarget` still derives subjects from the state
snapshot (real agents), which now matches what agent-only surfaces show once
integration-backed ledger rows are excluded.

Unified Agent host reports now make module readiness and updater/config
lifecycle evidence monitoring-owned observed state. Monitoring preserves the
last successful one-shot update transition across subsequent reports, forwards
the applied config fingerprint without config values, and records Host,
Docker/Podman, and Kubernetes module failures so API consumers can distinguish
an active process from an initialized monitoring source. The Kubernetes module
uses the shared agent TLS constructor for custom CA and leaf-fingerprint trust;
its Pulse transport must not regress to a Kubernetes-local insecure-only TLS
configuration.
The same host report carries bounded OS package-update posture: supported
manager, pending count, package/version identifiers, inspection time,
reboot-required state, and an operator-safe inspection error. Monitoring owns
normalization and deep-copying of that observed state, and stamps inventory
freshness from server receipt time so a skewed agent clock cannot keep update
authority fresh indefinitely; it does not infer
updates from kernel strings, refresh package indexes, install packages, or
convert the presence of an update into execution authority.
The host report also carries bounded package-cache cleanup posture: supported
provider, reclaimable byte count, fingerprint, inspection time, and an
operator-safe error. Monitoring stamps freshness from server receipt time and
normalizes that scalar evidence without receiving cache entry names or paths.
It does not infer cleanup eligibility, run cache scans server-side, or turn
reclaimable bytes into mutation authority.
The authenticated Unified Agent report also carries `OperationReceiptVersion`
as monitoring-owned runtime ingest metadata. Absence, zero, unknown, and future
versions are unsupported; monitoring must never infer support from an agent or
product version string. Each accepted report replaces the prior value, so an
agent replacement or compatible-to-legacy downgrade immediately removes the
receipt-protocol prerequisite for actionable update and cleanup capabilities.
The raw protocol integer stays internal to agent transport, monitoring ingest,
canonical capability construction, and dispatch readiness. Customer-facing
resource and frontend contracts expose only the derived capabilities. A
compatible report is necessary but never sufficient mutation authority: the
agent execution server's live, authenticated connection recheck remains
authoritative immediately before durable action admission and dispatch.

HTTP availability probes consume the shared explicitly unverified,
parseable-peer-certificate capture
boundary used by connection discovery, so support for operator self-signed
endpoints does not create independent skip-verification configurations.
Direct mock-node generation also clamps its allocation count to the canonical
fixture bound even when called below the normal configuration-normalization
entry point. The backing slice uses that fixed canonical capacity rather than a
request-derived capacity, while the normalized count continues to determine the
generated fixture length.

The monitoring-owned storage metrics runtime must preserve store-backed storage
chart continuity during resolver warm-up. `syncUnifiedStorageMetrics` must
prefer the resolver's canonical storage metrics target when it exists, but must
fall back to the storage resource id instead of dropping the resource when the
resolver has not yet produced a storage target. The fallback is a metrics
continuity path for canonical storage resources, not a second storage identity
or recovery-source model.

That same reloadable multi-tenant monitor boundary also owns instance-wide
notification settings fan-out. `ForEachMonitor` visits every live tenant
monitor so callers can propagate the webhook security allowlist and public
URL to each org's notification manager, and tenant monitors inherit those
persisted settings at creation through the router's monitor initializer, so
an org created after the settings were saved (or after a restart) observes
the same allowlist as the default org.

This subsystem now sits under the dedicated core monitoring runtime lane so
discovery, metrics-history correctness, and platform-specific runtime coverage
can be governed as first-class product work instead of staying diluted inside
architecture coherence.
Standalone host-agent identity continuity is part of that monitoring runtime
contract. `internal/monitoring/monitor_agents.go` must resolve short/FQDN
hostname aliases through the shared unified-resource equivalence rule when it
binds tokens, matches reports, and removes ignored agents, so reconnects and
reloads keep the same canonical host without weakening token uniqueness across
different machines.
Docker / Podman token binding in `internal/monitoring/monitor_agents.go` follows
the same single-agent product boundary: token uniqueness and conflict messages
are about Docker / Podman module reports from `pulse-agent`, not enrollment of a
separate Docker-specific agent product.
That same monitoring boundary now owns agentless availability targets as a
first-class provider, not as a settings-only helper. Saved availability targets
load from the config persistence boundary, schedule through
`InstanceTypeAvailability`, and publish `SourceAvailability`
`network-endpoint` supplemental records for unified-resource consumers. ICMP is
the default low-overhead check, while TCP and HTTP are canonical fallbacks for
devices or runtimes where ICMP is unavailable or the useful signal is a port or
web interface.
Supplemental records carry the saved target's optional `LinkedResourceID`
forward into `AvailabilityData` so the unified-resource registry can correlate
and project the probe facet onto the referenced resource. Every saved target
continues to emit its own `network-endpoint` supplemental record regardless of
that correlation outcome; monitoring never substitutes a matched host or
service identity for the configured check. Monitoring does not perform the
correlation decision itself; it only forwards the link hint for the registry
to resolve. Every completed probe also authors an operational-trust
`EvidenceEnvelope` with provider `availability`, collector
`availability-poller`, the saved target as its provider reference, the exact
observation/ingest times, and a validity window of twice the effective polling
interval. Before the first completed probe, evidence is explicitly partial and
unknown with reason `availability_not_observed`; monitoring must never encode
that state as a confirmed failure or a healthy observation. The registry owns
binding the source envelope to the check resource and cloning a separately
bound envelope for any matched-resource facet projection after correlation.
Availability target kind is monitoring-owned runtime metadata, not a frontend
guess. Saved targets carry the bounded `targetKind` values `machine`, `service`,
and `device`; monitoring must preserve that value in probe status, supplemental
resource availability data, and availability tags. Missing legacy target kinds
default to `service`, and monitoring must not promote a ping, TCP, or HTTP probe
to a machine solely from address shape, protocol, name, or successful reachability.
Mock-mode availability targets must use that same provider vocabulary. The
mock fixture graph may author ping/TCP/HTTP endpoint examples, but monitoring
and API consumers must receive them through `SourceAvailability` supplemental
records and probe-status projections, not through a mock-only monitoring type.
Frontend monitoring consumers should treat those supplemental records as
day-to-day availability evidence. Settings owns saved target management, while
the frontend-primitives-owned Standalone surface may read the same
`network-endpoint` projection to show current reachability, latency, check age,
and failure state without creating another monitoring provider or top-level
availability route.
Mock-mode Discovery context follows the same fixture-graph rule. Demo service
details such as detected version, config/data/log paths, Docker bind mounts,
ports, and suggested web URLs may be authored in mock fixtures, but consumers
must receive them through the normal Discovery API contract rather than through
frontend-only demo data or a monitoring-only side channel.
Demo Discovery fixtures must cover the authored estate broadly enough that the
default drawer experience demonstrates meaningful service context instead of a
majority of unknown placeholder records. Non-HTTP services may publish a clear
no-web-interface diagnostic, but fixtures must still identify the service,
version, category, and useful operator paths through the normal Discovery API.
That same monitoring boundary also owns the escalation callback bridge into the
alerts delivery layer. Monitor-owned escalation handling may still publish
canonical escalation state to websocket consumers, but notification fan-out
must defer quiet-hours and resolved-notification suppression policy to the
alerts manager instead of bypassing that shared routing contract when monitor
plumbs escalations outward. Scheduled escalation delivery must use the
notification manager's explicit escalation path, not normal alert re-send
delivery, so delivery cooldown cannot suppress an escalation level that the
alert manager has already deemed due and the escalation channel target remains
the configured email/webhook/all target for that level.
That same monitoring owner now also governs monitored-system grouping readiness
for settings and support boundaries. A non-nil unified read-state is not
sufficient when provider-owned supplemental inventories such as TrueNAS or
VMware are still settling: monitoring must report the grouping view as
unavailable until every active connection in that provider has reached an
initial baseline and the canonical monitor store has rebuilt at or after that
provider watermark, otherwise previews and support ledgers can freeze against
a transient startup undercount.
That same monitoring boundary also owns the machine-readable unavailable-state
contract for monitored-system usage. `internal/monitoring/monitored_system_usage.go`
must emit canonical reason codes such as
`monitor_state_unavailable`, `supplemental_inventory_unsettled`, and
`supplemental_inventory_rebuild_pending` when usage cannot yet be resolved, so
settings and support surfaces can show verification or recovery state without
inventing their own readiness heuristics or falling back to a fake count.
That same continuity rule applies to canonical unified resource snapshots.
`internal/monitoring/monitor.go` must overlay recent standalone host-agent
continuity records onto `UnifiedResourceSnapshot()` and
`GetUnifiedReadStateOrSnapshot()` results, so first-login and post-restart
Infrastructure views retain the durable agent-backed systems Pulse already
knows about while live reports and supplemental providers catch up.
That same monitoring owner also governs collector payload compatibility at the
shared boundary. Podman container stats must honor Podman's compat payload when
it exposes a direct CPU percentage and otherwise fall back to Podman's
wall-clock delta semantics rather than Docker's multi-core normalization, and
Proxmox Ceph status decoding must accept monitor totals from either
`monmap.num_mons` or the concrete monitor arrays and manager standby entries as
either bare names or structured objects so collector payload variations do not
break the canonical monitoring path. That same compatibility boundary also owns
legacy Unraid raw-status normalization at host-agent ingest: when older agents send
`rawStatus` without the newer normalized `status`, `internal/monitoring/monitor_agents.go`
must derive the canonical disk status before storage-risk assessment runs so
v5 aggregate counters do not override clearly healthy per-disk state during v6
compatibility operation.
Unraid long-running operation state uses the same authoritative transition
boundary. A non-empty active sync action may carry progress; cancellation,
completion, or idle state is represented by an empty action and normalized
zero progress even when an older collector or buffered payload retains a
terminal percentage. Accepted terminal reports must immediately remove
`unraid_sync_active` from host state, canonical storage resources, alerts, and
UI-facing risk projections. Older reports must never restore it after that
transition, while a mere loss of reports waits for the reporting lease and then
clears only the transient operation evidence alongside an explicit connectivity
signal.
The same monitoring compatibility boundary owns Unraid slot filtering and
operator health posture after host-agent ingest. Empty no-present Unraid slots
must be removed before storage-risk assessment so unassigned array capacity is
not reported as missing or disabled media. An Unraid array with assigned data
disks but no configured parity is an attention/warning posture with the
machine-readable `unraid_no_parity` reason, while active parity check/sync
state remains a separate `unraid_sync_active` reason. Realtime resource
broadcasts must preserve canonical identity, discovery target, metrics target,
incident rollups, and raw `agent`/`storage` facet payloads so frontend
infrastructure surfaces can explain degraded/warning rows without falling back
to generic status labels. Storage platform-data payloads built by
`monitorStoragePlatformData` must carry the full `zfsPool` report (scan
activity and per-device states/errors/messages) alongside the flattened
`zfsPoolState`/error scalars whenever canonical `StorageMeta.ZFSPool` is
present, so `/api/state` and websocket consumers can render device-level ZFS
health in parity with the unified-resources read path. That realtime broadcast contract also owns source and
platform identity for storage resources: `internal/monitoring/monitor.go` must
carry the canonical `Resource.Sources` array onto `ResourceFrontend` and
`platformData.sources`, and must derive storage `platformType` from the owning
source/facet instead of treating the `storage` resource type as Proxmox by
default. Appliance presentation details such as Unraid array identity may remain
inside storage metadata, but agent-backed storage must stay canonical
`platformType=agent`. That same broadcast resource contract owns resolved
metrics targets. Monitoring must enrich broadcast/state `ResourceFrontend`
payloads from the active metrics-target read-state before serialization so
`/api/state` and websocket consumers use the same canonical metrics history IDs
as `/api/resources`; storage resources must not fall back to generated
`Resource.ID` values when the unified resource registry can resolve a
source-owned `storage` target.
That same broadcast projection owns aggregate resource disk I/O. When the
canonical unified-resource metrics include `diskRead` or `diskWrite`,
`internal/monitoring/monitor.go` must project those rates into
`ResourceFrontend.diskIO` through the shared resource converter, so
`/api/state` and websocket consumers read disk throughput from the same
freshness-gated resource metrics contract as CPU, memory, disk, and network.
Unraid ingest must preserve the agent's native disk topology fields through the
monitoring model and read-state projection. `internal/monitoring/monitor_agents.go`
and `internal/monitoring/monitor.go` must carry model, transport, filesystem,
native capacity, used/free bytes, temperature, spin state, and read/write/error
counters without requiring a parallel SMART row. Monitoring may normalize legacy
statuses and filter empty slots, but it must not collapse assigned Unraid
array/cache members back to generic host disks or discard native fields before
unified resources builds storage and physical-disk resources.
Host-agent memory ingest carries the reclaimable page-cache split. The host
agent reports `cacheBytes` (gopsutil Available minus Free, with the ZFS ARC
adjustment recomputing free so used + cache + free still covers the total),
and `internal/monitoring/monitor_agents.go` maps it into
`models.Memory.Cache`, clamping inconsistent or older-agent reports so
used + cache never exceeds total. Mock fixtures author the same split for
generic hosts and node-linked host agents, and any mock drift updater must
hold the used + cache + free invariant as sampled usage changes.
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
network inventory belongs on the shared `network` resource path, but phase 1
does not claim VMware network metrics or history. VMware phase-1 work must not
create `vmware-host`, `vmware-vm`, `vmware-datastore`, or `vmware-network`
history stores just because the collection APIs differ from other platforms.
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
Manual saved-connection tests that prove the required VMware API floor but
encounter non-fatal optional signal or performance failures must record a
successful poll attempt together with `observed.degraded`, `issueCount`, and
the summarized upstream diagnostics. They must not erase the same
partial-success evidence that a live inventory poll would publish, while
authentication, TLS, network, and required API-family failures must continue
to increment canonical poll failure state.
That same monitoring boundary now also owns runtime mock rebind continuity for
API-backed supplemental providers. When `/api/system/mock-mode` flips on a
running server, the live TrueNAS and VMware provider bindings must swap to the
mock-backed supplemental records and refresh canonical read-state immediately
instead of waiting for a process restart before shared resource consumers can
see the platform inventory.
That same runtime boundary also owns authorization order for demo toggles.
`internal/monitoring/monitor.go` must not clear alerts, reset runtime state,
or restart discovery until the canonical mock runtime has accepted the
requested mode change; rejected release-demo fixture enables must fail before
any monitoring reset so the live preview does not blank itself on an
unauthorized toggle.
That same monitoring boundary now also owns atomic unified-metric persistence.
When unified resource sync projects agent, VM, app-container, or storage
metrics into persisted history, it must append in-memory history first and
flush the backing store through one `metrics.WriteBatchSync` batch per sync
sweep instead of per-metric async writes, so canonical chart history cannot
race itself into partial persisted windows.
That same chart boundary now also owns long-range in-memory coverage
selection. `internal/monitoring/metrics_history.go` must expose guest and node
coverage spans for the requested metric families, and
`internal/monitoring/monitor_metrics.go` must prefer the in-memory history
when that span already covers the requested chart window before falling back
to SQLite, so long-range chart batches do not pay an unnecessary store round
trip just because the request is larger than the old fixed in-memory
threshold.
Agent and node CPU temperature are part of that shared metrics-history family.
`internal/monitoring/monitor_agents.go` must write the primary host-agent CPU
temperature into both in-memory guest history and the persisted `agent`
metrics-store stream, while `internal/monitoring/metrics_history.go` keeps
`temperature` alongside CPU, memory, disk, and I/O for guest and node history
reads. Mock history must seed the same metric so Proxmox node drawer thermals
exercise the production contract instead of relying on a frontend-only fallback.
Host-agent thermal pressure is not part of that Celsius history stream.
`internal/monitoring/monitor_agents.go`, `internal/monitoring/monitor.go`, and
the shared model conversion helpers must preserve `sensors.thermalState`
through ingest, read-state projection, and frontend conversion, while leaving
`agent.temperature` and `metric=temperature` unset unless a real Celsius value
exists.
Host-agent GPU sensor summaries follow that same descriptive-host-telemetry
path. Monitoring must preserve typed GPU id, name, temperature, utilization,
and VRAM readings from agent reports through models, read-state projection, and
frontend conversion, while still using only real Celsius readings for
`agent.temperature` or `metric=temperature` and without promoting GPU workload
or process inventory into monitoring state.
Host-agent power sensor summaries follow the same descriptive-host-telemetry
path. Monitoring must preserve `sensors.powerWatts` readings from agent reports
through models, read-state projection, and frontend conversion without
promoting wattage into temperature history, resource lifecycle, storage health,
or alert metrics unless a separate governed contract adds that metric.
That same monitoring owner also owns canonical unified-resource publication on
`/api/state` and the websocket `state.resources` hydrate path. Monitoring must
publish those resources from the same canonical unified snapshot that
`/api/resources` seeds in mock and live mode, rather than projecting a second
raw store-only inventory for broadcast. Otherwise cold hydrate and later
registry-backed refreshes can swap the operator-visible infrastructure set
under one running session.
The same state-publication owner also carries Proxmox tag presentation. PVE
polling must fetch datacenter `tag-style` through `/cluster/options`, parse the
color map and `case-sensitive` flag per configured Proxmox instance, and merge
that into `models.State.PVETagStyles` before websocket/API publication. Clearing
a Proxmox color map must replace that instance's stored style with an empty
style and rebuild the legacy aggregate `PVETagColors`; stale colors from a
previous poll must not survive as if they still came from Proxmox.
That websocket publication boundary must also treat an absent hub as an absent
broadcast channel in both direct nil and typed-nil forms. Tenant-scoped
background monitors can start in headless test or maintenance runtimes before a
hub is wired, and state publication must no-op safely instead of dereferencing
a nil `*websocket.Hub` during ticker refresh.
That same headless runtime boundary applies to agent ingest itself. Once
`ApplyHostReport`, `ApplyDockerReport`, or `ApplyKubernetesReport` accepts and
finishes applying a report, monitoring must refresh the canonical monitor
adapter before returning so typed `ReadState`, `/api/resources`, and Patrol see
the accepted host, workload, and cluster truth even when no websocket clients
are connected. Websocket client presence and broadcast hydrate are delivery
concerns; they must not gate canonical agent-report publication.
That same Docker/Podman monitoring boundary now also owns Docker
authorization-plugin posture. `internal/dockeragent/collect.go` must project
`system.Info().Plugins.Authorization` into the canonical agent report,
`internal/monitoring/monitor_agents.go` must preserve that posture on the
shared Docker host model, and `internal/monitoring/docker_commands.go` must
refuse Docker daemon-mutating commands when authorization plugins are
configured until the upstream Moby authz-plugin advisory line has a fixed Go
module release.
Unified-resource Docker / Podman lifecycle capabilities consume that preserved
posture and must fail closed when it blocks mutation; monitoring remains the
runtime truth producer and must not grow a monitor-local start/stop/restart
transport around the governed action executor.
That same collector boundary also owns the maintained engine-client seam.
`internal/dockeragent/docker_client.go`, `internal/dockeragent/collect.go`,
and `internal/dockeragent/swarm.go` must keep Pulse's package-local
`dockerClient` interface as the compatibility layer while the underlying
implementation routes through maintained `github.com/moby/moby/api` and
`github.com/moby/moby/client` modules, so monitoring runtime collection does
not drift back onto the legacy `github.com/docker/docker` Go module line.
That same monitoring owner now also governs restart-safe standalone host
continuity for monitored-system grouping. `internal/monitoring/monitor_agents.go`
must persist recent host identity at report time, and
`internal/monitoring/monitored_system_usage.go` must project that continuity
back into the canonical read state through the unified-resources-owned overlay
path instead of rebuilding registry truth locally. A server restart or v6
upgrade must not briefly forget an already admitted standalone host and
misclassify its next report as a brand-new counted system.
That same standalone-host continuity boundary also owns host snapshot and
connection-list continuity during monitor reloads. `internal/monitoring/monitor.go`
must apply the same continuity overlay when `HostsSnapshot()` resolves its
canonical read state, so settings and other host-list consumers do not blank
previously admitted Pulse Agent rows during a config-driven monitor swap while
fresh reports are still in flight.
That same mock-runtime boundary also owns freshness while demos are running.
The mock update loop must keep provider-backed TrueNAS and VMware records plus
legacy PBS and PMG summaries on current `LastSeen` and health state each tick,
so long-lived infrastructure, workloads, storage, and recovery demos do not
decay into synthetic stale-state warnings while mock mode remains enabled.
That same Proxmox container monitoring boundary now also owns runtime counter
authority when the lower-fidelity container list or cluster-resources payload
and the current-status payload differ. `internal/monitoring/monitor_pve.go`,
`internal/monitoring/monitor_pve_guest_lxc.go`, and
`internal/monitoring/monitor_polling_containers.go` must merge the current
`GetContainerStatus` counters through one canonical `mergeContainerRuntimeCounters`
path before LXC rate calculation. A present status field is newer authority
even when it is zero or lower after a restart; an absent/null status field
retains the listing field and its presence state. The merge must retain the
status response receipt time and reuse the same prefetched status snapshot for
metadata enrichment instead of paying disconnected metric and metadata status
reads that can diverge.
That same Proxmox backup/snapshot boundary owns bounded concurrent guest
snapshot enumeration. `internal/monitoring/monitor_backups.go` must query VM
and LXC snapshot endpoints through one capped worker pool and preserve
previously-known snapshots only for guests that were not successfully polled in
the current cycle, so large or slow clusters do not starve later guests while
transient misses still avoid destructive state churn.
That same guest-monitoring boundary also owns linked host-agent precedence for
Proxmox VMs. When a VM has a live linked Pulse host agent, the canonical disk
inventory and aggregate disk summary must prefer that linked host-agent disk
collection over the narrower QEMU guest-agent filesystem list, so VM overview
surfaces keep the richer inside-guest storage truth instead of silently
regressing to mount-only visibility.
That same mock-runtime boundary also owns update cadence. Demo and preview
environments may slow the configured tick interval to reduce visual churn, but
that cadence must flow through the shared mock update loop and smoothing model
rather than through page-local polling suppression or demo-only frontend
special cases.
That same demo-owned mock boundary also owns chart continuity. Seeded mock
history and runtime mock sampling must be projections of the same canonical
metric timeline, so changing chart ranges feels like zooming one history
window instead of stitching a second live tail onto the end of seeded
sparklines. Monitoring must not let any mock-owned resource receive a
duplicate generic unified-resource writer that appends a divergent recent tail
after the canonical mock sampler has already seeded and extended that series.
That same sampler-owned boundary also owns seeding cost. Historical mock
seeding runs synchronously on monitor startup and in package proofs, so it
must stay deterministic and bounded by fixture size instead of carrying
per-resource pacing sleeps or other wall-clock throttles that can exhaust the
package-level `go test -race -timeout 10m` budget on hosted runners before
canonical mock history has even finished initializing.
The seed path must therefore include the canonical terminal `now` sample on
its tiered timeline and anchor seeded series to the canonical metric model at
that timestamp instead of to mutable state fields, so historical charts match
the exact runtime history that would have been recorded live.
Historical backfills must resolve resource identity, metric bounds, stable
seed, speed, and role once per series, then append that ordered series to
`MetricsHistory` under one series-level mutation. They must not repeat
normalization, hashing, role lookup, locking, retention scans, and capacity
checks independently for every point. Test harnesses must also bound mock seed
duration to the deepest history window they actually prove; Core E2E owns a
seven-day chart contract and must not make every parallel shard build the
production-preview 90-day timeline.
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
Bounded plateau generation must preserve an exact constant when the lower and
upper metric bounds are equal; interpolation and noise must not introduce
floating-point drift into a synthetic flat series.
That same mock chart boundary also owns request-path efficiency. Demo chart
reads must reuse monitor-owned downsampled mock history for the current mock
sampler generation instead of regenerating or re-downsampling the same seeded
timeline on every endpoint hit. When seeded mock history is rebuilt or a live
mock tick advances, monitoring must invalidate that cache so preview charts
stay current without paying repeated per-request synthesis cost.
That same sampler-owned cache contract also covers compact summary reads after
the dashboard overview retirement. When live mock ticks advance, monitoring
must repopulate the canonical 24-hour aggregate `/api/charts/storage-summary`
cache inside the sampler path instead of leaving the first operator request
after each tick to rebuild per-pool mock storage charts on demand.
The same mock sampler path must also prewarm the default Workloads guest-chart
cache through `GetGuestMetricsForChartBatch`, using canonical `ReadState`
workload identities for VMs, system containers, Kubernetes pods, and app
containers so `/api/charts/workloads` and `/api/charts/workloads-summary` do
not rebuild every guest sparkline on the first post-tick request.
That same metrics-hot-path ownership also includes metric-type selection for
compact summary reads. When infrastructure or storage summary routes request
only a subset of canonical chart series,
`internal/monitoring/monitor_metrics.go` must preserve that narrowed metric
set through the batch store fallback path instead of querying every metric type
for each resource and discarding most of the payload afterward.
That same mock-runtime owner now also owns demo-scenario curation.
`internal/mock/fixture_graph.go`, `internal/mock/platform_fixtures.go`, and
`internal/mock/demo_scenarios.go` may project an authored demo estate over
generic fixture synthesis, but that authored layer must stay graph-native and
runtime-stable so infrastructure, workloads, storage, and recovery all present
the same human-readable platform story instead of a lab of random names,
legacy `mock-cluster` labels, or surface-specific mock overrides.
Mock fixture defaults in `internal/mock/generator.go` (the `DefaultConfig`
constant) are also part of that mock-runtime contract. They target a
mature small-to-mid homelab / SMB environment so platform-first pages
exercise table density, sorting, grouping, drawer behavior, and
responsive layout out of the box: 5 Proxmox cluster + standalone nodes
with 6 VMs and 8 LXCs each, 5 Docker/Podman hosts with 14 containers
each, 4 standalone Pulse-managed hosts, and 3 Kubernetes clusters
(Production EU + Staging EU + Development EU; a fourth Edge / k3s
profile is curated in `demo_scenarios.go` and instantiates when
`K8sClusterCount` is bumped) with 5 nodes, 40 pods, 14 deployments
each, and curated native controller inventory so the Kubernetes
platform-page overview tab shows multiple clusters and the
nodes/pods/deployments/controller tabs exercise multi-cluster grouping.
Each Kubernetes cluster carries its own node-name prefix
(`prod-euw1-k8s-*` / `stage-euw1-k8s-*` / `dev-euw1-*`), a distinct
kubelet version, and exactly one degraded scenario — Production EU
runs a NotReady worker (`prod-euw1-k8s-03`), Staging EU runs the
payments-worker CrashLoopBackOff, Development EU runs the
cron-nightly-backfill ImagePullBackOff — so the demo tells three
distinct stories instead of three clones. Each cluster also seeds
curated RBAC inventory (per-namespace Roles + RoleBindings plus an
aggregated ClusterRole / ClusterRoleBinding for `pulse-demo-monitoring`)
so the Kubernetes platform-page Configuration tab exercises the same
RBAC summary-count contract live agents use. The
`TestKubernetesDemoClustersTellDistinctStories` test in
`internal/mock/demo_scenarios_test.go` guards that distribution.
Generic Kubernetes fixture synthesis must also preserve at least one ready,
schedulable node whenever a cluster has nodes. Randomized readiness and cordon
stories may degrade the remaining nodes, but they must not accidentally create
a total outage that erases running-pod metrics or makes the demo and its proof
nondeterministic; explicit curated outage scenarios remain the owner of
cluster-wide unavailability.
Bumps to those defaults must keep the
curated demo scenario's per-node hostname seasoning in
`demo_scenarios.go` aligned (today: pve1..pve6 with regional labels,
shared-fabric storage names, and per-node fallback naming) so the
broadcast and snapshot views render the same human-readable estate
regardless of the configured fixture size. The
monitor-broadcast equivalence test
(`TestMonitorBuildBroadcastFrontendStateUsesCanonicalMockUnifiedResources`)
compares broadcast count against the canonical snapshot count within a
±5% tolerance to absorb the legitimate row drops from
`coalesceBroadcastResources` and `convertResourcesForBroadcast` under
larger fixture sizes; that tolerance does not loosen the rest of the
test's exact-name and exact-identity assertions.
That same chart boundary also owns storage-series identity. Monitoring and
`ReadState` consumers must address storage pool and physical-disk history
through the resolved unified-resource metrics target, so seeded history,
runtime writes, storage summary hover selection, and detail charts all extend
one series instead of splitting between canonical resource IDs and
source-native metric IDs.
Proxmox Ceph pools are part of that same storage-series contract. When Ceph DF
exposes pools, monitoring must project each pool through the shared
`models.CephPoolStorage` helper, write storage history under that pool storage
id, and evaluate alerts through `CheckStorage` so per-pool thresholds, active
alerts, and charts all use the same storage series identity.
Ceph cluster identity is FSID-owned across discovery sources. Proxmox API Ceph
reports are canonical when available, host-agent Ceph reports are the fallback
or supplemental source, and state reconciliation must collapse reports for the
same FSID into one cluster while preserving source aliases for existing pool
thresholds. Host-agent Ceph pool storage ids must not carry `agent:` as their
canonical identity; that prefix remains only an alert/threshold alias for
previously persisted overrides.
That same chart boundary also owns provider-backed workload bridging.
Workload-chart consumers may query VM and system-container history through the
resolved unified-resource metrics target, but the emitted series identity must
stay on the canonical workload row ID, so VMware-backed workloads participate
in summary hover and focus without leaking provider-native metric IDs into the
UI contract.
That same chart boundary also owns Kubernetes mock-history completeness.
Seeded mock history and live mock appends must project Kubernetes clusters,
nodes, pods, and deployments onto the same canonical unified-resource metrics
targets that the registry exposes, instead of seeding only pod timelines and
leaving cluster, node, or deployment charts blank on the demo path. When the
mock sampler records a Kubernetes series, it must write the canonical cluster,
node, pod, or deployment key directly and preserve the same identity across
seeded history, in-memory continuation, and metrics-store fallback reads.
That same summary owner also owns VMware partial-success classification.
Optional VI JSON or Automation enrichment reads that fail after base
host/VM/datastore inventory succeeds must not collapse the whole poll into a
runtime failure. The client should preserve the usable base snapshot, record
degraded enrichment issues on the snapshot, and let the poller publish those
as `observed.degraded` plus summarized issue metadata instead of clearing the
observed contribution or pretending the refresh was fully healthy.
That same VMware inventory floor also owns operator-visible uptime and guest
filesystem usage. `vmware.InventoryMetrics` carries `UptimeSeconds`,
`DiskUsedBytes`, `DiskTotalBytes`, and `DiskPercent` for hosts and VMs so the
canonical `Resource.Uptime` field and `ResourceMetrics.Disk` series populate
on vSphere-backed workloads — without these the workloads table renders
empty "0s" and blank disk cells for every vSphere row. Real collection uses
PerformanceManager `sys.uptime.latest` (host + VM) plus
`sys.osUptime.latest` for VMs (Tools-reported guest OS uptime; preferred
when present), and `GET /api/vcenter/vm/{vm}/guest/local-filesystem`
aggregated across mount points for disk usage. A 503 from that REST
endpoint (Tools not running) is recorded as a non-fatal `unavailable`
enrichment issue rather than failing the poll. Mock fixtures
(`internal/mock/platform_fixtures.go`) must synthesize the same fields per
powered-on VM and drop them for powered-off VMs so the demo estate
exercises the same workload-table contract as live vCenter would.
That same broadcast converter owns the canonical Resource.Uptime
fallback. `monitorUptime` walks platform-specific carve-outs
(`Agent.UptimeSeconds`, `Proxmox.Uptime`, `Docker.UptimeSeconds`,
`Kubernetes.UptimeSeconds`, `PBS.UptimeSeconds`, `PMG.UptimeSeconds`,
`TrueNAS.UptimeSeconds`) before falling back to `Resource.Uptime`. The
vSphere adapter populates only the canonical field for ESXi hosts and
VMs, so without that final fallback the websocket payload would silently
drop uptime for VMware-backed rows even though the REST contract carries
it. Carve-outs still take precedence so existing platforms keep their
prior behavior.
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
attached honestly to canonical `agent`, `vm`, `storage`, or `network`
resources; the collector must not solve that ambiguity by creating VMware-only
top-level incident targets.
That same monitoring boundary now also has a concrete detail-enrichment seam.
`internal/vmware/client.go`, `internal/vmware/client_topology.go`, and
`internal/vmware/provider.go` may use the official vCenter Automation API plus
VI JSON `name`, `parent`, `runtime`, `resourcePool`, `datastore`, `host`,
`vm`, `Network.host`, `Network.vm`, and datastore-summary paths to enrich canonical VMware-backed resources
with placement, guest identity, and storage consumer context. That
enrichment remains best-effort provider detail on the shared VMware source: it
must not create a second topology cache, a VMware-only placement store, or a
parallel guest-identity model outside the canonical `agent` / `vm` /
`storage` / `network` resource graph.

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
Monitor construction is the runtime handoff for metrics-store placement and
aggregation cadence: `internal/monitoring/monitor.go` may pass the resolved
data path, `PULSE_METRICS_DB_PATH`, and `PULSE_METRICS_ROLLUP_INTERVAL` through
to `pkg/metrics`, but the SQLite path normalization, rollup bounds, and write
amplification policy stay owned by the metrics store rather than by a
monitoring-local helper.
Install-wide telemetry counts are also monitoring-owned now. Any telemetry or
reporting surface that claims installation totals must aggregate across the
provisioned tenant set through the reloadable multi-tenant monitor boundary,
not by reading `GetMonitor()`'s default-org compatibility shim.
Those install-wide counts are now the canonical aggregate adoption signal for
outbound usage telemetry: monitoring owns the source counts for agent hosts, Docker
and Kubernetes workloads, storage pools and physical disks, Ceph, network
shares, TrueNAS systems/VMs/apps, VMware hosts/VMs/datastores, availability
targets, and active alerts. Telemetry callers may consume those coarse totals,
but they must not bypass monitoring to read provider-local identifiers or
tenant-local resource names.
That install-wide boundary also owns privacy-bounded outcome aggregation for
telemetry schema v3. It may count alert history entries fired, acknowledged,
or resolved within the existing 30-day local history window and notification
attempt, successful-delivery, and terminal failed/dead-letter totals within the
notification queue's existing seven-day telemetry window, across the
provisioned tenant set. Attempts include retries; a recoverable failed attempt
must not also become a terminal failure. It must consume only content-free
totals from tenant-owned managers and must not export alert
IDs, resource IDs, actors, reasons, destinations, recipients, endpoints,
timestamps, error text, or message content. Notification queue state remains
delivery evidence rather than alert-lifecycle truth, and monitoring must not
infer alert resolution from delivery success or failure.
That same reloadable multi-tenant monitor boundary also owns wiring tenant
identity into per-org notification delivery. When a tenant monitor is
initialized for a non-default org, monitoring installs an org-backed tenant
identity resolver on that org's notification manager so webhook payloads can
stamp the org ID and current display name; the resolver reads the org record
lazily so display-name renames propagate without monitor restarts. The
default org keeps environment-provided identity and must not be overridden
here.

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
That same startup path must avoid recursive ownership mutation of image-owned
runtime directories such as `/app` and `/opt/pulse`; those paths are build-time
artifacts, and copy-up into per-container writable layers is a monitoring and
host-health regression, not a valid runtime repair.
That same monitoring boundary now also owns Docker Swarm runtime truth at the
collection seam. `internal/dockeragent/swarm.go` is the canonical manager-side
filter for live Swarm services and tasks, so monitoring consumers do not ingest
historical shutdown tasks as if they were still part of the active runtime.
Standalone Docker daemons report `Swarm.LocalNodeState=inactive`; that is not
Swarm capability evidence and must be normalized away before agent reports,
monitoring ingest, or unified-resource consumers can surface Swarm roles,
services, tasks, tabs, or alerts.

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
host agent reports SMART health, SMART identity, ZFS pool membership, or NVMe
`percentage_used` for a physical disk that Proxmox itself exposes without
trustworthy health, wearout, model, serial, WWN, type, size, or pool data, the
merge path in `internal/monitoring/monitor.go` must promote that missing data
into the canonical physical-disk model without overwriting provider truth. The
read-state sensor conversion must preserve SMART `SizeBytes` so subsequent
refreshes keep whole-disk capacity evidence available for Proxmox disk merges.
The Proxmox polling runtime in `internal/monitoring/monitor_pve.go` must
evaluate disk alerts only after that merged disk view exists, so
controller-backed disks do not lose health and endurance coverage between
collection and alerting.
Wide SAS inventories use the same trust boundary. The host collector must
fan out per-disk SMART reads with bounded concurrency so a single report
deadline cannot truncate a controller-sized suffix of the inventory, and it
must preserve the Linux controller plus HCTL (or controller-member target)
alongside each reading. When Proxmox exposes a SAS address as `serial` while
smartctl exposes the drive serial, the smartctl serial is the canonical
hardware identity; exact device-path correlation is permitted only inside an
already-linked host/node parent and must fail closed when topology is
ambiguous. Direct SATA, SAS, and NVMe device fallback IDs retain their legacy
shape, while multiple controller members behind one block path add their
controller target to the fallback identity. Per-member I/O must never inherit
an aggregate controller counter.
The same rules apply to SATA and NVMe inventory: direct-disk source IDs keep
their historical shape, controller-member IDs add their member target, and
cross-source correlation is scoped to the canonical parent node. A successful
retry may enrich an earlier smartctl attempt but must not erase earlier model,
serial, failure, or counter evidence. SMART temperature selection accepts only
plausible readings, prefers ATA attribute 194 over 190 when higher-level
temperature fields are invalid, and preserves reported zero counters as known
values while leaving omitted counters unknown.

Disk identity, temperature, I/O, controller association, and pool membership
also carry typed collection state from `pkg/diskinventory`: `available`,
transiently `unavailable`, provider/controller `unsupported`, or unexpectedly
`missing`. Normalization may retain the last known value when the current
observation is not available, but it must preserve the current state and
reason so API and UI consumers do not present retained evidence as freshly
collected. Unified-resource physical-disk round trips must retain named
`StorageGroup` membership rather than degrading it to the generic `Used`
filesystem label.
That same host-agent temperature boundary must prefer a recent linked host-agent
payload over legacy SSH collection once the agent provides any usable CPU, NVMe,
GPU, or SMART temperature reading. `internal/monitoring/monitor_polling_node_helpers.go`
may invoke SSH only when no linked, recent, available host-agent temperature
exists or the agent payload has no usable positive reading. Identity-only or
zero-temperature SMART rows do not count as usable by themselves, but the
runtime must not keep probing legacy SSH solely to augment an otherwise healthy
agent temperature payload with SMART data.
Legacy SSH temperature collection must also use the Pulse sensor-wrapper
contract before falling back to raw lm-sensors output. `internal/monitoring/temperature.go`
must request `/usr/local/sbin/pulse-sensors` when it exists, parse the wrapper
payload as `{sensors, smart}`, preserve backward compatibility with old forced
`sensors -j` keys, and expose SMART disk temperatures through the same
`models.Temperature.SMART` path used by the physical-disk merge.
When the payload arrives in the legacy raw `sensors -j` shape, the parser must
mark it via `models.Temperature.LegacySensorsFormat` and the host-agent merge
in `internal/monitoring/host_agent_temps.go` must preserve that marker, so the
frontend can surface a data-gated outdated-sensor-setup notice instead of
letting SATA/SAS disk temperatures silently stay blank on pre-rc.6 SSH key
setups.
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
That recovery identity adapter must carry the canonical unified-resource
`ResourceID` separately from the provider-native Proxmox `SourceID`. Recovery
mappers consume the canonical ID directly for subject linkage and retain the
source ID only for provider correlation and fallback derivation; a canonical
workload ID must never be passed through source-specific ID generation again.
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
Docker-managed app-container web-interface metadata must use the same
host-plus-normalized-container-name identity (`app-container:<host>:name:<name>`)
as the stable synchronization key. Monitoring must migrate current runtime-key
Docker metadata and legacy app-container guest metadata to that key when a
Docker report is ingested, then prefer that stable guest key when projecting
unified app-container custom URLs. Runtime container IDs remain action and
metric identities, not the persistent URL metadata identity.
When a container retains its runtime ID but changes normalized name, monitoring
must move stable guest and Docker metadata to the new name and remove the
obsolete name key after a successful or already-resolved destination. Rename
migration must snapshot all sources before writing so swaps do not exchange
URLs accidentally, and ambiguous normalized source or target names must fail
closed. A later unrelated container that reuses the old name must not inherit
the renamed container's URL.
Kubernetes pod, Deployment, and Service web-interface metadata uses
`k8s-workload:<cluster>:<kind>:<namespace>:<name>` as its stable logical
identity. Monitoring must migrate a current legacy unified-resource key, plus
the legacy `k8s:<cluster>:pod:<pod-uid>` key for pods, when that resource is
observed. Runtime UIDs remain discovery and metrics coordinates. Every scope
component is required so a URL cannot cross cluster, namespace, or kind
boundaries, and an existing empty stable record is an intentional clear that
must block legacy fallback.
The monitor-owned guest, Docker, and host metadata stores are the live
in-memory authority for projection and migration. API, config export/import,
tenant usage, Assistant URL discovery, and reload paths must share those exact
tenant-scoped store instances rather than opening parallel caches over the
same files.
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
`memavailable`, RRD `memused`, guest-agent `/proc/meminfo` via the shared
Proxmox client, and only then linked host-agent memory. Saturated Linux VM
status is the narrow exception: a queryable guest agent's cache-aware
`/proc/meminfo` evidence is tried before RRD because the saturated status is
the known Proxmox failure mode. Both RRD and guest-agent fallback caches must
key on `(instance, node, vmid)` instead of raw `node/vmid`, so separate
Proxmox instances cannot leak stale or foreign memory evidence into each other
just because they reuse the same node name and VMID.
Linux memory availability must never be inferred from `MemTotal-MemFree`.
Nodes and guests prefer a valid explicit `MemAvailable`/`available` field,
then a complete reclaimable-component estimate, then valid RRD availability or
used evidence. The conservative old-kernel guest-agent estimate is
`MemFree + Buffers + Cached + SReclaimable - Shmem`; it is valid without swap
but not from truncated or total/free-only meminfo. A material `total-used`
gap may remain a lower-trust estimate only when it supplies independent
evidence that the reported used value already excludes cache. Invalid,
overflowed, non-finite, over-total, or conflicting candidates must be rejected
before the next source is considered; an explicitly present zero RRD used or
zero available value remains a valid idle or full-pressure sample rather than
being mistaken for an absent field.
Node RRD fallback caches must key on `(instance, node)`, just as guest RRD and
guest-agent caches key on `(instance, node, vmid)`, so identically named nodes
in different Proxmox instances cannot exchange memory evidence.
Running LXC cluster-resource memory is cache-inclusive and therefore cannot be
used as live usage when both RRD `memavailable` and `memused` are absent.
Unified Linux and Docker agent ingest likewise must not repair a missing used
value from total minus free alone; it may use an explicit used/percentage or
complete free-plus-cache evidence. In every collector, known capacity with no
cache-aware usage is represented by `models.Memory.UsageUnavailable`, the
canonical memory source `unavailable`, and fallback reason
`cache-aware-memory-unavailable`. A recent trusted node or guest snapshot may
be carried across a transient reconnect under the existing bounded
`previous-snapshot` rule; otherwise the unknown state must remain honest.
Unknown memory samples must not append zeroes to in-memory or persistent
history, project a canonical unified-resource memory metric, start or clear a
threshold alert, or render as 0% in product surfaces. Existing active alerts
remain fail-safe until a later trusted sample crosses the clear threshold.
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
host identifier before it checks `removedHostAgents` or emits the
reconnect-blocking error, and removed-host records must carry machine and token
identity so the block is scoped to the retired host. Hostname equivalence may
only participate when it is qualified by the same token and compatible machine
identity; removing one stale duplicate must not poison a different live host
that shares the same hostname or raw machine identifier through a different
token.
Docker host re-identification now shares the same hostname-equivalence rule:
monitoring may treat `qnap` and `qnap.local` as the same host when the token
or machine identity already points at one canonical runtime, but it must not
invent broader short-name collapsing on its own or fork away from the
unified-resource monitored-system contract.
That same Docker host identity boundary also owns token-binding aliases after
a reconnect match. When `ApplyDockerReport` has already matched a report to an
existing canonical Docker host, the token uniqueness guard must accept that
host's stable source ID and previous agent ID as aliases for the current raw
agent ID so container recreation does not reject the same logical host after it
has been matched. This must not weaken the one-token-per-Docker-agent rule for
different hosts.
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
recent discovered host/pool/dataset/app/VM/share/disk/recovery counts there instead of
recomputing settings health panel-by-panel. That same poller-owned summary must
also absorb manual saved-connection test results from the shared
`POST /api/truenas/connections/{id}/test` path, so row-level operator tests in
settings update the canonical last success / last error state instead of
stopping at disconnected toast notifications.
That same runtime owner also defines the feature-default contract for TrueNAS:
the API-backed integration is on by default, and `PULSE_ENABLE_TRUENAS` is an
explicit opt-out switch rather than a required bootstrap toggle.
That same TrueNAS monitoring boundary owns system identity compatibility for
`/system/info`. `internal/truenas/client.go` must tolerate provider-version
drift in non-identity display fields such as `buildtime`, including structured
date/value wrappers, and still preserve the canonical hostname, version,
machine ID, capacity, and poll-health path instead of failing connection tests
or background refreshes during JSON decoding.
That same monitoring boundary now also owns live TrueNAS disk temperatures.
`internal/truenas/client.go` and `internal/truenas/provider.go` must ingest
legacy `disk.temperatures` from the REST API or `reporting.get_data` `disktemp`
from the current JSON-RPC transport, and project those readings into the
canonical physical-disk model and risk path
instead of leaving temperature telemetry agent-only or adding a TrueNAS-local
presentation shim.
That same monitoring boundary also owns SMART-backed TrueNAS disk risk
projection. When TrueNAS raises disk-local SMART alerts such as
`truenas_smart`, `internal/truenas/provider.go` must fold that incident truth
into the canonical physical-disk risk payload instead of leaving SMART failure
state trapped in incident/status-only decorations that storage consumers do
not read.
The same boundary owns TrueNAS `smart_status` normalization. `internal/truenas/client.go`
must parse REST and RPC SMART status separately from native disk state, and
`internal/truenas/disk_health.go` plus `internal/truenas/provider.go` must map
null, empty, missing, unknown, or unavailable SMART telemetry to canonical
`UNKNOWN` health with no replacement-required risk. Explicit SMART failure and
native failure states such as `FAULTED`, `FAILED`, `OFFLINE`, `REMOVED`, and
`UNAVAIL` must continue to produce canonical disk-health risk.
That same boundary owns boot-pool and replication-target storage posture.
`internal/truenas/client.go` must collect `boot.get_state` through the
connection's negotiated transport, use REST only on a version-gated legacy
connection, merge it only within the current configured connection, and use
its vdev leaves to enrich boot-disk pool membership and native ZFS state.
`internal/monitoring/truenas_poller.go` must correlate
`replication.query` intent across providers within the same organization using
local/PULL ownership or a unique configured/observed target-host match for
remote PUSH tasks. The resulting `SET`/`REQUIRE` receive-side read-only posture
is healthy, while ordinary read-only datasets remain warning and locked or
unmounted datasets remain offline. Correlation must fail closed when target
identity is absent or ambiguous, and common pool or dataset names on another
connection are never sufficient identity.
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
`internal/monitoring/mock_metrics_history.go` must treat
`internal/mock/fixture_graph.go`, `internal/mock/platform_fixtures.go`, and
`internal/mock/demo_scenarios.go` as the one canonical mock owner for legacy
Proxmox/Docker/Kubernetes/agent/PBS/PMG snapshot state plus provider-backed
TrueNAS and VMware fixtures. Monitoring must not rebuild mock provider context
from standalone defaults, consume partial legacy helper exports, or mix
snapshot state with separate provider fixtures when seeding read-state or
metrics history. The graph, its platform projections, and its curated demo
scenario layer are the canonical mock runtime API.
Availability mock fixtures belong to that same graph authority: UPS network
cards, MQTT meters, HTTP panels, and controller ping targets must be authored
once in `internal/mock/` and then projected into availability status, unified
resources, and connections payloads from that shared graph.
Governed action mock fixtures follow the same rule. Pending, approved,
executing, completed, rejected, and failed action examples are authored once in
`internal/mock/action_fixtures.go`, reference resources from the graph's
canonical unified-resource snapshot, and are projected by the read-only action
API without writing demo rows into the durable action-audit database.
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
That same monitoring boundary now also owns native TrueNAS VM inventory.
`internal/truenas/client.go` must ingest `vm.query` through the official
`/api/current` JSON-RPC websocket transport, `internal/truenas/provider.go`
must project those rows as canonical `vm` resources under the top-level
TrueNAS appliance, and frontend TrueNAS surfaces must read the typed
`TrueNASData.VM` facet instead of inventing a provider-local VM table contract.
Pulse must not treat TrueNAS VMs as Proxmox guests, Docker containers, or a
separate `truenas-vm` resource type.
That same monitoring boundary now also owns connected-infrastructure
projection for API-backed platforms. `internal/monitoring/connected_infrastructure.go`
must project TrueNAS into the canonical connected-infrastructure surface list,
carry TrueNAS hostname/version through the shared top-level system grouping,
and preserve platform-managed surfaces such as `proxmox`, `pbs`, `pmg`, and
`truenas` when host telemetry is ignored. Ignore/remove semantics on that
surface remain machine-scoped and may only strip the local `agent`, `docker`,
and `kubernetes` reporting surfaces from the grouped row. That same
connected-infrastructure payload now also owns guest-link continuity for host
agents: when an agent is running inside a VM or system container, monitoring
must preserve the canonical linked guest identity on both active and ignored
connected-infrastructure rows instead of forcing settings consumers to infer
guest-backed hosts from labels or hostnames.
path or treat API-backed app workloads as second-class compared with native
Docker reports.
That same boundary now also owns native host-history fallback for API-backed
TrueNAS systems. `internal/truenas/client.go`,
`internal/truenas/provider.go`, `internal/monitoring/truenas_poller.go`, and
`internal/monitoring/monitor_metrics.go` must route TrueNAS
`reporting.get_data` system history through the shared `agent` guest-chart
path, so canonical host charts can show real provider-backed CPU, memory,
network, and disk throughput history when Pulse's own local history is still
shallow. That same guest-chart boundary must treat windows beyond the
in-memory chart threshold as store-backed hot paths: batch helpers may merge
native/provider history afterward, but they must not spend the steady-state
latency budget on full in-memory pre-scans that can never satisfy long-range
coverage, and any caller-supplied metric filters must flow into the shared
batch store query instead of being trimmed only after retrieval.
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
history contract. Host profile fields on `AgentData`, such as `hostProfile`
for Unraid-compatible Pulse Agent hosts, are presentation identity only; typed
read access through `internal/unifiedresources/views.go` must keep
`Platform()` as the normalized runtime platform and expose the profile through
a separate host-profile accessor.
That same monitoring boundary now also owns API-backed TrueNAS CPU
temperature. `internal/truenas/client.go` must use the modern
`reporting.get_data` RPC surface to derive current `cputemp` readings in the
same RPC session as system telemetry, and `internal/truenas/provider.go` must
project those readings into the canonical host temperature and host-sensor
contract. Pulse must not treat TrueNAS CPU temperature as an agent-only
capability or invent a TrueNAS-local sensor payload.
Taken together, this is the current monitoring-owned TrueNAS floor: one stored
API connection can surface one canonical top-level system, shared host
telemetry/history, app-container workloads, native VM workloads, disk
health/history, native network shares, and per-connection poll health plus
observed contribution counts without requiring the unified agent. The same
poller/provider path also owns assistant-driven app start/stop, logs, and
config refresh for canonical app workloads. Pulse does not promise a separate
TrueNAS runtime model, broader NAS administration, or agent-required bootstrap
at this floor.
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
The persisted guest metadata store must also remain the synchronization
boundary for last-known guest identity updates. Store reads and writes must
copy metadata, including slice fields, so asynchronous monitor persistence
cannot expose mutable store pointers to caller goroutines or race with
release-pipeline `-race` backend proofs.
When Proxmox reports saturated VM memory without `meminfo` or `freemem` but
the QEMU guest agent is queryable, the monitoring memory selector must prefer
the guest's own `/proc/meminfo` `MemAvailable` signal before lower-trust
Proxmox RRD or status fallbacks. Guest-agent filesystem payloads from Windows
volume GUID mounts remain part of the same canonical VM disk metric path and
must not be dropped just because system-reserved partitions share a physical
disk with usable volumes.
That same monitoring boundary now also owns physical-disk I/O history as a
first-class canonical metric stream. `internal/monitoring/monitor_agents.go`
must project host per-device I/O counters onto the same SMART-resolved disk
resource id that unified resources expose, `internal/monitoring/metrics_history.go`
must retain `disk`, `diskread`, `diskwrite`, and `smart_temp` on one shared
disk history model, and mock seeding plus live mock ticks in
`internal/monitoring/mock_metrics_history.go` must append to that same disk
timeline instead of creating a second drawer-only or mock-only disk history
path.
That same monitoring-owned disk-health boundary also includes shared storage
risk assessment in `internal/storagehealth/`. When providers or host agents
emit structured storage topology such as Unraid per-disk state, the shared
assessment layer must derive canonical risk and alert severity from that
richer disk topology instead of letting coarser aggregate counters override it
and flap the operator-facing storage alert surface.
That same monitoring-owned storage polling boundary also owns cluster-shared
Proxmox storage status coherence. `internal/monitoring/monitor_polling_storage.go`
must merge shared storage observations across nodes into one cluster-scoped
record whose canonical status remains `available` whenever any reporting node
still has the shared target active; node-local inactive copies may expand node
affinity, but they must not downgrade the cluster record into an offline
projection just because that node won the capacity sample.
That same monitoring-owned Proxmox backup boundary also owns the inventory
readiness signal used by backup orphan alerts. `internal/monitoring/` must
record when PVE VM and container inventory has successfully observed a given
instance and guest type, including template VMIDs that are intentionally
excluded from normal workload resources. Backup alert evaluation may then
receive that scoped signal from monitoring, but alert code must not infer PVE
orphan readiness from recovery rollups alone.
That same Proxmox backup boundary also owns permission-repair guidance for PVE
backup visibility failures. When storage content reads fail with authorization
errors, the monitoring warning must tell operators to grant `/storage`
`PVEDatastoreAdmin` to both the `pulse-monitor@pve` service user and the
configured privilege-separated token when that token id is known.
That same monitoring-owned host-agent ingest boundary now also owns
vendor-managed NAS RAID normalization. `internal/monitoring/monitor_agents.go`
must filter vendor-managed system arrays through the shared
`internal/storagehealth/` rules before host state sync so internal Synology
`md0/md1` and QNAP `md9/md13` volumes do not leak into canonical APIs,
resources, or alert inputs just because those hosts report Linux md arrays
alongside customer-managed storage pools.
That same monitoring runtime boundary also owns logger-safe reload behavior.
`internal/monitoring/reload.go` may refresh runtime config, but it must do so
through the no-logging-init config loader so an in-process monitoring reload
does not reinitialize the global logger while pollers, websocket writers, or
tests are still emitting logs. Runtime context access in the monitor-owned
pollers must likewise route through the monitor's synchronized accessor instead
of reading mutable shared fields directly from concurrent goroutines.
That same monitoring-owned PBS job-health boundary must keep backup task
evidence honest. PBS does not expose a canonical scheduled backup-job
configuration API, so PBS-side backup-family entries may only be labeled as
observed task-history evidence. Scheduled backup compliance for PVE workloads
belongs to a future PVE `/cluster/backup` source. PBS task-history reads must
therefore use a bounded filtered lookback over `/nodes/localhost/tasks` and
surface truncation or permission gaps explicitly instead of treating one recent
unfiltered sample as configured backup-job proof.

Task 09 preserves two APT telemetry clocks at host-agent ingest: `CheckedAt`
is agent-observed time and `ObservedAt` is server-received time. Monitoring
must not overwrite the former with the latter; replay and skew safety consumes
both timestamps downstream.

Monitoring now also exposes a bounded direct Proxmox guest observation for the
governed action verifier. `ObserveProxmoxGuest` resolves the configured
instance client under the monitor lock, reads VM or LXC status and uptime from
the Proxmox API, validates the requested guest identity, and stamps server
observation time. It must not satisfy this contract from cached resource state
or node-agent telemetry: the action layer depends on this read remaining in a
trust domain distinct from the node agent that executes `qm` / `pct`.

Canonical Proxmox VM/LXC typed views also expose the monitoring-owned source
delivery status and timestamp alongside guest power state. Downstream Patrol
transition detection consumes that status instead of inventing a fixed stale
window: a stopped guest can have fresh inventory, while a stale source cannot
authoritatively prove either a stopped transition or recovery.

### Agent fleet diagnostic derivation

Monitoring owns the read-only Agent Fleet Doctor derivation over current host,
Docker, Kubernetes, removed-agent, profile-assignment, and deployment state.
`internal/fleethealth/agent.go` supplies the shared agent connection identity,
heartbeat cutoff, and version-drift vocabulary used by both the monitoring
diagnostic and API connections ledger. Five expected reports must be missed,
with a five-minute minimum, before an agent becomes stale; missing timestamps
remain pending/never-reported rather than silently healthy. Version comparison
uses the canonical agent-update target independently from the running server
build version.

The diagnostic may derive bounded updater and module failure reasons, normalized
platform and network evidence, profile drift, and safe repair-handoff support.
It hashes raw machine IDs, filters malformed interface addresses, and redacts
unbounded error strings before returning evidence. Derivation must not mutate
monitor state, probe providers, enqueue commands, or turn a repair hint into
execution authority. Runtime-family normalization is shared with agent
lifecycle through `platformsupport.ResolveAgentRuntimePlatform`. Explicit
Windows, macOS, and FreeBSD families remain distinct; known unsupported OS
families and missing evidence fail closed; and unmatched non-empty values from
legacy agents resolve to Linux because those releases reported gopsutil
distribution identifiers instead of their compiled GOOS. This intentionally
avoids a duplicated distro allowlist, so long-tail distributions such as
Mageia receive the same Linux repair handoff as Ubuntu or Debian. Unknown
updater states remain explicit warnings, and unverified FreeBSD/pfSense
installer state still fails closed for upgrade-command support.
`internal/fleethealth/agent_test.go` and
`internal/monitoring/agent_fleet_doctor_test.go` are the focused runtime proofs.

Removed-agent rows keep command-scoping identity instead of losing it with the
live record: host and Docker removal capture the agent's last-known reported
platform onto the removed record (`RemovedHostAgent.Platform`,
`RemovedDockerHost.Platform`), and the fleet diagnostic resolves that retained
value through the same runtime-family normalization as live subjects, so
`/api/agents/diagnostics` reports Linux for retained legacy distro identifiers
but no platform for missing or explicitly unsupported evidence. Removed
Kubernetes clusters retain no platform because the cluster report never
carries one; downstream host-side cleanup handoffs must treat an empty platform
as "offer explicitly labeled commands for every family, never one guessed
executable". The retained field is additive and optional on the
serialized removed lists, so snapshots recorded before the field existed load
unchanged with an empty platform.

### Unified Agent destination delivery metrics

The local agent health listener exports
`pulse_agent_destination_configured{module,destination,role}` and
`pulse_agent_destination_delivery_up{module,destination,role}`. Role is bounded
to `primary` or `observer`; destination names come from validated configuration.
Observer delivery failure is visible but does not make the primary authority
unready or merge observer retry state into primary delivery health.
Host, Docker/Podman, and Kubernetes reporters each fan out the already-collected
snapshot without triggering a second collection. Their retry queues and latest
delivery gauges remain per destination, and Kubernetes observer transport uses
the same destination-scoped TLS and explicit plaintext policy as the host and
Docker reporters. Observer acknowledgements never change the canonical
monitoring configuration returned by the primary.

### PBS protection evidence collection

Direct PBS backup enumeration emits two separate storage/recovery inputs:
subject-linked recovery points and one typed provider observation for the
polled PBS instance. A complete poll records complete history with sufficient
permissions; a partially successful poll records partial history and the
appropriate partial or unknown permission posture; total transient failure
records unavailable history; total terminal authorization failure records
denied access. Retained backup points survive failed enumeration, but the new
provider observation immediately prevents those cached points from being
presented as current protection truth.

PBS mapping attaches provider scope and a typed evidence envelope to every
successfully enumerated recovery point. Identity correlation is confirmed only
for direct canonical identity and inferred only for an auditable unique
provider-scoped guest match. Monitoring persists the collection observation
before point reconciliation so completeness and permission failure cannot be
lost behind a successful cached-artifact path. Shared protection semantics stay
in `internal/recovery/`; PBS monitoring owns only this explicit evidence-quality
adapter.

### Alert-intent evidence adapters and UDP outcomes

Monitoring supplies read-only context to the alerts-owned intent resolver. The
operator-state adapter resolves source-native references to one canonical
unified-resource ID before reading durable operator intent. Lookup failure,
ambiguity, absence, or store error yields no suppression context; monitoring
does not synthesize maintenance state.

Backup-aware offline intent consumes a PVE task only when VMID, instance, and
node match and the task is active. `pollBackupTasks` stamps server observation
time. Evidence older than five minutes, more than one minute in the future,
finished, terminal, or missing an observation time fails closed. This
short-lived alert context is separate from PBS protection evidence and from
recovery assurance; it cannot claim that a backup is restorable or authorize a
restore.

Availability probing owns three outcomes: reachable, unreachable, and
indeterminate. UDP response-required mode needs a request and treats timeout or
mismatch as unreachable. Open-or-filtered mode may return indeterminate after
the full response deadline. Indeterminate clears accumulated failure count,
projects warning evidence, and emits no availability incident; it never claims
reachability. `internal/monitoring/availability_udp_test.go`,
`internal/monitoring/monitor_alert_intent_test.go`, and the backup polling
assertion in `internal/monitoring/monitor_full_coverage_test.go` are the focused
proofs.

### Durable host-agent removal admission

Monitoring ingest shares server-side host lifecycle authority with the
agent-lifecycle subsystem. `Monitor.RemoveHostAgent` must persist a tombstone
in the host continuity journal before it revokes an unused token, removes the
live record, unlinks resources, clears connection health, or resolves alerts.
The journal transition is rollback-safe. Persistence failure leaves the live
host present; journal load failure prevents monitor construction; expiry
failure retains the block. Monitoring must never convert an unavailable
security journal into an empty in-memory removal map.

`Monitor.New` hydrates canonical ID plus report-host, agent, machine, hostname,
platform, and token aliases from every non-expired tombstone. Removed entries
are excluded from active standalone-host continuity, monitored-system
projection, and remote-config fallback. The 24-hour expiry is based on the
persisted `removedAt` timestamp and deletes durable state before clearing the
snapshot and cache.

`ApplyHostReport` and removal are ordered by the dedicated host lifecycle
read/write lock. Concurrent reports may proceed together, but deletion waits
for earlier reports and prevents later reports from resurrecting the host.
Only a post-removal token and matching retained machine identity may transition
the tombstone back to active continuity. The canonical host ID survives report
ID or persisted-agent-ID alias changes across Linux/systemd, Docker unified
agents, and Windows MachineGuid identities. Token-plus-hostname disambiguation
continues to keep simultaneous cloned or duplicate machine IDs distinct.

After fresh-token re-enrollment, the old token stays attached to the host's
denied-token lineage even when that token is intentionally shared and remains
valid for another active agent. Monitoring rejects the detached credential
before token binding can manufacture a duplicate host. Manual operator
allowance is the only path that clears that lineage. Focused proofs are
`internal/monitoring/monitor_host_agent_removal_lifecycle_test.go`,
`internal/monitoring/monitor_host_agents_test.go`, and
`internal/api/host_agent_removal_lifecycle_integration_test.go`; the concurrency
proof must also pass under the Go race detector.

### Native pool-health collection and appliance isolation

TrueNAS monitoring preserves the complete native `pool.query` observation
needed by the shared storage-health contract: pool GUID and status detail,
structured scrub or resilver state, pool and vdev read/write/checksum counters,
mirror/RAIDZ/spare topology, path-only leaves, and explicit native
missing/unavailable members. `disk.query` absence alone is not missing-disk
evidence. Unknown fields remain unknown and may not be converted into a failed
device, a recovered pool, or a zero-error observation.

The poller keys system and child source identity by configured connection.
Appliances with matching hostnames, restored pool GUIDs, or matching pool names
remain separate through refresh, cache rebuild, restart, and registry ingest.
Replication-target readonly classification remains a separate native-evidence
step and cannot hide locked or unmounted dataset state.

Ceph monitoring may enter the provider-neutral pool-health envelope only from
the native cluster health state and native health-check map. It preserves check
codes, severity, and summaries in deterministic order. A cluster-level
`HEALTH_WARN` or `HEALTH_ERR` does not identify a failed OSD or disk unless the
provider supplies that more specific evidence.

`internal/truenas/client_api_shapes_test.go`,
`internal/monitoring/truenas_poller_test.go`, and
`internal/monitoring/ceph_test.go` are the focused collection and identity
proofs.
