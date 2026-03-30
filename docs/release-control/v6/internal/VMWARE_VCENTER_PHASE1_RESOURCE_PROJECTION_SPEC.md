# VMware vCenter Phase-1 Resource Projection Spec

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `docs/release-control/v6/internal/VMWARE_VSPHERE_PHASE1_EXECUTION_PLAN.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`

## Intent

This document locks the canonical phase-1 projection contract for
`vmware-vsphere`.

It exists to answer one architecture question before runtime work starts:
what exactly from `vCenter` becomes a shared Pulse resource, what stays
topology-only metadata, and what must not be projected at all in phase 1.

## Governing Rule

Phase-1 VMware support is only valid if all of these stay true:

1. `vCenter` is the only supported phase-1 entry point
2. ingestion is `api-backed`
3. ESXi hosts project as canonical `agent`
4. virtual machines project as canonical `vm`
5. datastores project as canonical `storage`
6. datacenter, cluster, folder, resource pool, and `vCenter` itself remain
   topology or relationship metadata, not top-level Pulse resources
7. `physical-disk`, `system-container`, `app-container`, and recovery
   artifacts remain out of phase 1

## Canonical Source Contract

When runtime work starts, Pulse should add one canonical VMware-backed source
classification alongside the existing shared source enums.

That means:

1. one shared VMware source key, expected to land as `vmware` /
   `SourceVMware`, not separate `vcenter` and `esxi` source families
2. `platformType` should be `vmware-vsphere` on projected VMware-backed
   canonical resources
3. `vCenter` is the connection authority, not a top-level unified resource
4. the saved connection or discovered vCenter identity should scope every
   VMware provider identifier so bare managed-object IDs do not masquerade as
   workspace-global identities

The last point is an implementation inference from the official APIs: the docs
describe host, VM, and datastore identifiers as resource-type identifiers, but
they do not claim those identifiers are globally unique across multiple
distinct `vCenter` environments.

## Projection Table

| VMware object class | Canonical Pulse type | Phase-1 floor | Top-level? | Notes |
| --- | --- | --- | --- | --- |
| `HostSystem` / ESXi host | `agent` | yes | yes | host inventory, runtime state, health, metrics/history |
| `VirtualMachine` | `vm` | yes | yes | workload inventory, runtime state, guest identity when available, snapshot-tree visibility |
| `Datastore` | `storage` | yes | yes | inventory, capacity/free-space, accessibility, relationships |
| `vCenter` | none | no | no | connection and poll authority only |
| `Datacenter` | none | metadata only | no | placement and topology context |
| `ClusterComputeResource` | none | metadata only | no | placement and grouping context |
| `Folder` | none | metadata only | no | inventory placement context |
| `ResourcePool` | none | metadata only | no | workload placement context |
| snapshot tree | none | read-side VM detail only | no | not a recovery artifact |
| alarm definitions / triggered alarms | `alerts` / incident context | yes | n/a | routed through shared alert and timeline surfaces |
| events / tasks | resource timeline / incident context | yes | n/a | supporting context, not new resource types |

## ESXi Host To `agent`

What the APIs clearly support:

1. the vCenter host summary surface exposes a host identifier, name,
   connection state, power state, and in newer API versions `host_uuid`
2. VI JSON host runtime exposes connection state, power state, maintenance
   mode, and health runtime state
3. VI JSON host summaries expose overall alarm status plus storage and CPU
   status summaries

Phase-1 projection rule:

1. one ESXi host from `vCenter` becomes one canonical `agent`
2. the provider-scoped host identifier is the VMware-side primary identity for
   that resource inside the VMware source
3. the canonical `agent` display identity should come from the host name, not
   from cluster or datacenter labels
4. cluster, datacenter, folder, and resource-pool context stay in
   VMware-specific platform metadata or relationships under that canonical
   `agent`

Required secondary identity capture when available:

1. `host_uuid`
2. hostname / DNS name
3. IP addresses when the chosen collection path can read them cleanly
4. any future shared agent-augmentation identity fields only through the
   canonical cross-source merge path, not through a VMware-only dedupe model

Validation note:

The official `Vcenter Host Summary` schema adds `host_uuid` in vSphere API
9.0.0.0. Older supported floors may therefore need fallback identity anchors
from VI JSON host properties. That cross-version identity floor must be proven
before a support claim is made.

## Virtual Machine To `vm`

What the APIs clearly support:

1. `GET /api/vcenter/vm` returns VM identifier, name, power state, CPU count,
   and memory size
2. `GET /api/vcenter/vm/{vm}` returns optional `identity.bios_uuid` and
   `identity.instance_uuid`
3. `GET /api/vcenter/vm/{vm}/guest/identity` returns guest OS family, guest
   hostname, and IP address when VMware Tools is running and has data
4. VI JSON `VirtualMachine.snapshot` returns the current snapshot and snapshot
   tree when snapshots exist

Phase-1 projection rule:

1. one VMware virtual machine becomes one canonical `vm`
2. the provider-scoped VM identifier is the VMware-side primary identity for
   that resource inside the VMware source
3. `instance_uuid` and `bios_uuid` should be captured as secondary identities
   whenever present because they are the cleanest future bridge to cross-source
   merge or assistant reasoning
4. guest hostname and guest IP are optional enrichments, not identity
   requirements, because the guest-identity API can fail when VMware Tools is
   absent or not reporting
5. host placement, folder placement, resource-pool membership, and datastore
   attachments stay as relationships or platform metadata under the canonical
   `vm`

Snapshot boundary:

1. snapshot tree is valid phase-1 workload detail
2. snapshot presence may feed timeline or investigation context
3. snapshot data must not be promoted into shared recovery artifacts, restore
   points, or recovery-supported product wording

## Datastore To `storage`

What the APIs clearly support:

1. `GET /api/vcenter/datastore` returns datastore identifier, name, type,
   free-space, and capacity
2. VI JSON `DatastoreSummary` exposes `accessible`, `multipleHostAccess`,
   `maintenanceMode`, `capacity`, `freeSpace`, and `url`

Phase-1 projection rule:

1. one datastore becomes one canonical `storage`
2. the provider-scoped datastore identifier is the VMware-side primary
   identity for that resource inside the VMware source
3. display identity should come from datastore name
4. datastore type, accessibility, maintenance mode, and multi-host access are
   part of the shared storage-facing truth when the upstream environment
   exposes them
5. host attachments and VM usage stay as relationships under canonical
   `storage`, not as new top-level VMware objects

Validation note:

Datastore inventory and headline capacity are clearly supported from the
official APIs. Exact phase-1 extraction of host mounts and VM-to-datastore
usage needs live validation so Pulse does not promise more placement fidelity
than the chosen collection path can actually deliver.

## Topology And Relationship Rules

These VMware concepts remain metadata or relationships in phase 1:

1. `vCenter`
2. datacenter
3. cluster
4. folder
5. resource pool
6. datastore cluster / storage pod
7. network objects

That means phase-1 VMware work must not add:

1. `esxi-host`
2. `vsphere-vm`
3. `vsphere-datastore`
4. `vsphere-cluster`
5. `vsphere-datacenter`
6. `vsphere-resource-pool`

If a future slice wants one of those to become top-level, it needs a separate
governed admission decision because it would expand the shared Pulse resource
model, not just the VMware adapter.

## Alerts, Incidents, And Timeline

What the APIs clearly support:

1. VI JSON `AlarmManager.GetAlarm` retrieves alarms defined on a managed
   entity
2. VI JSON host summaries expose `overallStatus`
3. VI JSON event and task paths exist for related operational context

Phase-1 alert rule:

1. VMware alarm and health signals may surface only through the shared alert
   and incident model
2. alert-backed investigation must attach to canonical `agent`, `vm`, or
   `storage` resources
3. cluster-, datacenter-, or folder-scoped VMware alarm context may inform the
   incident, but it must not create synthetic top-level VMware incident
   resources in phase 1
4. event and task context belongs in the shared resource timeline or incident
   context, not in a VMware-only history surface
5. direct VMware alarm acknowledgement, reset, or editing is out of scope

## Telemetry And History

What the APIs clearly support:

1. VI JSON `PerformanceManager.QueryPerfComposite` returns composite metrics
   for a host and its first-level child entities and is explicitly documented
   for host-plus-VM usage
2. VM runtime and hardware detail are available through the vCenter VM APIs
3. datastore free-space and capacity state are available from the datastore
   inventory APIs

Phase-1 telemetry rule:

1. ESXi host telemetry must land on the shared `agent` metrics/history path
2. VM telemetry must land on the shared `vm` metrics/history path
3. datastore state or capacity-history signals must land on the shared
   `storage` path
4. phase-1 VMware work must not create a `vmware-host`, `vmware-vm`, or
   `vmware-datastore` history store

Validation note:

`QueryPerfComposite` is clearly documented for host-plus-child collection, but
it is not by itself proof of complete standalone VM history parity across the
whole supported floor. Exact counter coverage, retention shape, and any
required per-VM follow-up queries must be proven on the live support floor
before the product makes a broad telemetry/history claim.

## Explicit Phase-1 Non-Projections

Phase 1 must not project any of the following into shared top-level Pulse
resource types:

1. `physical-disk`
2. `system-container`
3. `app-container`
4. recovery artifact
5. restore point
6. backup job
7. direct ESXi standalone system

That exclusion remains in force even though VMware exposes richer APIs in some
of those areas.

## Discovery Gaps That Still Matter

The architecture is stable, but these points still require live proof:

1. exact cross-version identity floor for ESXi hosts when `host_uuid` is not
   available from the chosen supported version
2. exact relationship extraction path for VM-to-datastore usage and
   datastore-to-host mount fidelity
3. exact alarm-to-canonical-resource attachment rule for cluster- or
   datacenter-scoped alarms
4. exact performance/history counter set that is strong enough for the shared
   Pulse history surfaces without inventing VMware-only paths

If those proofs fail, the support claim must narrow or stop. The runtime must
not compensate by inventing provider-local resource types or sidecar products.

## Primary Source Basis

1. host inventory summary:
   [Vcenter Host Summary](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/data-structures/Vcenter%20Host%20Summary/)
2. host runtime and health:
   [HostRuntimeInfo](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/data-structures/HostRuntimeInfo/)
3. host overall alarm and status summaries:
   [HostListSummary](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/data-structures/HostListSummary/)
4. VM inventory:
   [Vcenter VM list](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/vm/get/)
5. VM identity detail:
   [Vcenter VM get](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/vm/vm/get/)
6. guest identity detail:
   [Vcenter Vm Guest Identity get](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/vm/vm/guest/identity/get/)
7. datastore inventory:
   [Vcenter Datastore list](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/datastore/get/)
8. datastore accessibility and storage summary:
   [DatastoreSummary](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/data-structures/DatastoreSummary/)
9. VM snapshot tree:
   [Virtual Machine Get Snapshot](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/VirtualMachine/moId/snapshot/get/)
10. managed-entity alarms:
    [Alarm Manager Get Alarm](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/AlarmManager/moId/GetAlarm/post/)
11. host-plus-child performance:
    [Performance Manager Query Perf Composite](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/PerformanceManager/moId/QueryPerfComposite/post/)
