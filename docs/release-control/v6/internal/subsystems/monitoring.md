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
18. `internal/monitoring/monitored_system_usage.go`
19. `internal/dockeragent/swarm.go`
20. `internal/dockeragent/collect.go`
21. `pkg/proxmox/ceph.go`
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
34. `internal/mock/fixture_graph.go`
35. `internal/dockeragent/docker_client.go`
36. `pkg/agents/docker/report.go`
37. `internal/models/models.go`
38. `internal/models/models_frontend.go`
39. `internal/models/converters.go`
40. `internal/models/deepcopy.go`
41. `internal/mock/generator.go`
42. `internal/mock/demo_scenarios.go`
43. `internal/kubernetesagent/agent.go`
44. `pkg/agents/kubernetes/report.go`
45. `internal/monitoring/temperature.go`
46. `internal/truenas/client.go`
47. `internal/truenas/disk_health.go`
48. `internal/truenas/provider.go`
49. `internal/models/ceph_cluster_identity.go`
50. `internal/truenas/types.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add pollers/providers and discovery-provider coordination through `internal/monitoring/poll_providers.go` and `internal/monitoring/monitor_discovery_helpers.go`
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
   records through the Docker agent report for unified-resource ingestion.
   Swarm service records must preserve documented service update status
   (`UpdateStatus.State`, message, and completion time when reported) so the
   container runtime surface can distinguish stable services from active or
   failed rollouts without inventing frontend-only state.
   Failures in image, volume, network, node, secret, config, or storage-usage
   collection are best-effort warnings and must not make the whole host report
   fail when container/runtime health data is otherwise usable. Podman libpod
   pods remain outside this collector until a libpod-native collector owns that
   API shape.
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
14. Add or change Proxmox-side LXC Docker detection or inventory through
   `internal/monitoring/docker_detection.go`,
   `internal/monitoring/monitor_pve_guest_poll.go`, and monitoring guardrails
   together. Socket detection may only annotate LXC guests after explicit
   server opt-in. LXC Docker inventory may only emit Docker-agent-compatible
   reports into `ApplyDockerReport`, must skip guests with a linked online
   guest-local host agent, and must keep the command set to minimal read-only
   Docker summary and aggregate stats collection.
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
    API-owned application records: `app.query` is the preferred live inventory
    source, with legacy REST allowed only as a compatibility fallback. The
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
    TrueNAS storage and alert inventory follow native query methods first:
    pools use `pool.query`, datasets use `pool.dataset.query`, disks use
    `disk.query` with pool join options, and alerts use `alert.list`, with
    legacy REST allowed only as compatibility fallback.
    TrueNAS VMs and network shares follow the same provider-owned inventory
    boundary: `vm.query` data publishes native `TrueNASData.VM` on canonical
    `vm` resources, while SMB/NFS share data from `sharing.smb.query` and
    `sharing.nfs.query` publishes native `TrueNASData.Share` on canonical
    `network-share` resources parented to the owning dataset or pool when the
    API/path supplies that evidence. TrueNAS protection inventory follows the
    same native-query rule: ZFS snapshots prefer `zfs.resource.snapshot.query`
    with older `pool.snapshot.query`/REST compatibility fallback, and
    replication tasks prefer `replication.query`.
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

## Current State

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
That same monitoring boundary now owns agentless availability targets as a
first-class provider, not as a settings-only helper. Saved availability targets
load from the config persistence boundary, schedule through
`InstanceTypeAvailability`, and publish `SourceAvailability`
`network-endpoint` supplemental records for unified-resource consumers. ICMP is
the default low-overhead check, while TCP and HTTP are canonical fallbacks for
devices or runtimes where ICMP is unavailable or the useful signal is a port or
web interface.
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
plumbs escalations outward.
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
to generic status labels. That realtime broadcast contract also owns source and
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
Unraid ingest must preserve the agent's native disk topology fields through the
monitoring model and read-state projection. `internal/monitoring/monitor_agents.go`
and `internal/monitoring/monitor.go` must carry model, transport, filesystem,
native capacity, used/free bytes, temperature, spin state, and read/write/error
counters without requiring a parallel SMART row. Monitoring may normalize legacy
statuses and filter empty slots, but it must not collapse assigned Unraid
array/cache members back to generic host disks or discard native fields before
unified resources builds storage and physical-disk resources.
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
That same monitoring owner also owns canonical unified-resource publication on
`/api/state` and the websocket `state.resources` hydrate path. Monitoring must
publish those resources from the same canonical unified snapshot that
`/api/resources` seeds in mock and live mode, rather than projecting a second
raw store-only inventory for broadcast. Otherwise cold hydrate and later
registry-backed refreshes can swap the operator-visible infrastructure set
under one running session.
That websocket publication boundary must also treat an absent hub as an absent
broadcast channel in both direct nil and typed-nil forms. Tenant-scoped
background monitors can start in headless test or maintenance runtimes before a
hub is wired, and state publication must no-op safely instead of dereferencing
a nil `*websocket.Hub` during ticker refresh.
That same Docker/Podman monitoring boundary now also owns Docker
authorization-plugin posture. `internal/dockeragent/collect.go` must project
`system.Info().Plugins.Authorization` into the canonical agent report,
`internal/monitoring/monitor_agents.go` must preserve that posture on the
shared Docker host model, and `internal/monitoring/docker_commands.go` must
refuse Docker daemon-mutating commands when authorization plugins are
configured until the upstream Moby authz-plugin advisory line has a fixed Go
module release.
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
recovery when the lower-fidelity container list or cluster-resources payload
reports stale or zero I/O totals. `internal/monitoring/monitor_pve.go`,
`internal/monitoring/monitor_pve_guest_lxc.go`, and
`internal/monitoring/monitor_polling_containers.go` must merge the current
`GetContainerStatus` counters through one canonical `mergeContainerRuntimeCounters`
path before LXC rate calculation and must reuse the same prefetched status
snapshot for metadata enrichment instead of paying disconnected metric and
metadata status reads that can diverge.
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
anonymous telemetry: monitoring owns the source counts for agent hosts, Docker
and Kubernetes workloads, storage pools and physical disks, Ceph, network
shares, TrueNAS systems/VMs/apps, VMware hosts/VMs/datastores, availability
targets, and active alerts. Telemetry callers may consume those coarse totals,
but they must not bypass monitoring to read provider-local identifiers or
tenant-local resource names.

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
host agent reports SMART health or NVMe `percentage_used` for a physical disk
that Proxmox itself exposes without trustworthy health or wearout, the merge
path in `internal/monitoring/monitor.go` must promote that data into the
canonical physical-disk model and the Proxmox polling runtime in
`internal/monitoring/monitor_pve.go` must evaluate disk alerts only after that
merged disk view exists, so controller-backed disks do not lose health and
endurance coverage between collection and alerting.
That same host-agent temperature boundary must not suppress SSH SMART disk
collection just because the agent already reported CPU package or NVMe
temperatures. `internal/monitoring/monitor_polling_node_helpers.go` may skip
SSH only once the host-agent temperature payload already has SMART disk data,
so nodes keep their disk-temperature and SMART augmentation when the host agent
is present but lacks SMART support.
Legacy SSH temperature collection must also use the Pulse sensor-wrapper
contract before falling back to raw lm-sensors output. `internal/monitoring/temperature.go`
must request `/usr/local/sbin/pulse-sensors` when it exists, parse the wrapper
payload as `{sensors, smart}`, preserve backward compatibility with old forced
`sensors -j` keys, and expose SMART disk temperatures through the same
`models.Temperature.SMART` path used by the physical-disk merge.
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
The same boundary owns TrueNAS `smart_status` normalization. `internal/truenas/client.go`
must parse REST and RPC SMART status separately from native disk state, and
`internal/truenas/disk_health.go` plus `internal/truenas/provider.go` must map
null, empty, missing, unknown, or unavailable SMART telemetry to canonical
`UNKNOWN` health with no replacement-required risk. Explicit SMART failure and
native failure states such as `FAULTED`, `FAILED`, `OFFLINE`, `REMOVED`, and
`UNAVAIL` must continue to produce canonical disk-health risk.
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
