# VMware vCenter Phase-1 Alerts And Assistant Spec

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `docs/release-control/v6/internal/VMWARE_VSPHERE_PHASE1_EXECUTION_PLAN.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`

## Intent

This document locks the canonical phase-1 VMware contract for:

1. alert and incident projection
2. event and task context
3. resource timeline attachment
4. Assistant read exposure
5. Assistant control exclusion

It exists to keep VMware phase 1 on the shared Pulse investigation and AI
paths instead of drifting into a VMware-only incident model or admin plane.

## Governing Rule

If `vmware-vsphere` implementation starts, the phase-1 product claim is:

1. shared alert and incident visibility on canonical VMware-backed resources
2. shared event, task, and history context where the canonical investigation
   model expects it
3. shared Assistant read on canonical VMware-backed resources
4. no VMware-specific alert shell
5. no VMware-specific AI tool family
6. no VMware control exposure in phase 1

## What The Official APIs Clearly Support

Broadcom's official APIs clearly expose enough read-side signal for a
meaningful phase-1 investigation floor:

1. alarm definitions attached to a managed entity via
   `AlarmManager.GetAlarm`
2. triggered alarm state on managed entities such as `VirtualMachine`,
   `HostSystem`, and `Datastore`
3. overall managed-entity health state
4. filtered event retrieval through `EventManager.QueryEvents`
5. recent task context on managed entities such as `VirtualMachine`
6. performance and state context through the same shared APIs already chosen
   for the phase-1 projection floor

This is enough evidence for read-side incident support. It is not evidence for
safe VMware alarm management or control-plane claims.

## Shared Alert And Incident Contract

Phase-1 VMware alert support means:

1. canonical `agent`, `vm`, and `storage` resources may carry VMware-backed
   alarm and health context
2. shared alert and incident surfaces may evaluate, show, and drill into that
   context
3. the operator investigation path stays the same shared Pulse path used for
   other platforms

It does not mean:

1. a VMware-only alarm list
2. a VMware-only incident drawer
3. a VMware-only alarm editor
4. direct alarm acknowledgement or reset against VMware

## Signal Mapping Rules

### Direct Resource Signals

When a VMware signal is already attached to a projected phase-1 resource, the
mapping is straightforward:

1. ESXi-host signal maps to canonical `agent`
2. VM signal maps to canonical `vm`
3. datastore signal maps to canonical `storage`

Those signals may participate directly in shared alert evaluation, incident
detail, related-resource links, and resource timeline context.

### Topology-Scoped Signals

Some VMware signals can originate on objects that are intentionally not
top-level Pulse resources in phase 1, such as:

1. datacenter
2. cluster
3. folder
4. resource pool
5. storage pod

Phase-1 rule:

1. topology-scoped signals may enrich a shared incident only when Pulse can
   attach them honestly to one or more canonical `agent`, `vm`, or `storage`
   investigation paths
2. if the signal cannot be attached honestly, it remains supporting context or
   stays out of the support floor
3. topology objects must not become synthetic top-level VMware incident
   resources just to preserve every upstream signal

## Event, Task, And Timeline Contract

Phase-1 VMware history support is investigation support, not a second history
product.

That means:

1. `EventManager.QueryEvents` may supply filtered operational history
2. recent-task or task-collector paths may supply action context and in-flight
   state
3. that context belongs in shared incident detail and canonical resource
   timelines where those surfaces already exist
4. VMware phase 1 must not ship a provider-only history page, timeline model,
   or event browser as the primary investigation surface

Expected phase-1 use:

1. event context explains why a resource alarm or state transition matters
2. task context explains current or recent operator-visible transitions
3. timeline entries on canonical resources preserve durable investigation
   breadcrumbs instead of leaving VMware as a sidecar timeline

Validation note:

The APIs clearly expose events and tasks, but exact phase-1 retention depth,
query window strategy, and attachment quality still require live proof on the
real supported version floor.

## Assistant Read Contract

Phase-1 Assistant support for VMware is read-only and shared-tool-only.

That means:

1. Assistant reads VMware-backed resources through the shared `pulse_read` and
   `pulse_query` paths
2. Assistant targets canonical `agent`, `vm`, and `storage` resources only
3. Assistant may inspect:
   - canonical resource summary and state
   - relationships and placement metadata already present on the shared model
   - alarm, health, incident, event, and task context available on shared
     paths
   - metrics/history and snapshot-tree visibility where the shared runtime
     already exposes them
4. if the shared tool surface lacks a given VMware read, Pulse should narrow
   the VMware phase-1 claim rather than adding a VMware-only tool

## Assistant Control Exclusion

Phase-1 VMware support must not expose any of the following through Assistant
or operator-facing action surfaces:

1. VM power operations
2. snapshot create, delete, revert, or consolidate
3. guest operations
4. host maintenance mode or lifecycle actions
5. datastore actions
6. cluster administration
7. direct alarm acknowledgement, reset, or editing

Even though VMware exposes APIs for many of those actions, Pulse has not yet
expanded the governed action surface into a general VMware admin plane.

## Runtime Exposure Rule

When runtime work starts:

1. VMware-backed canonical resources may expose read-side context and history
2. VMware-backed canonical resources must not advertise VMware-specific action
   capabilities in shared capability metadata
3. `pulse_control` must not gain VMware verbs in phase 1
4. if another platform already supports a shared control verb, that does not
   automatically make the same verb admissible for VMware

## Proof Obligations

Before VMware can be called supported, proof must show:

1. shared alerts can drill into VMware-backed incidents without a provider-
   local shell
2. canonical resource timelines can carry VMware-backed context without a
   provider-only history model
3. Assistant can inspect VMware-backed canonical `agent`, `vm`, and `storage`
   resources using shared tools only
4. no VMware control capability leaks into tool exposure, resource
   capabilities, or product wording

## Current Blocker

As of 2026-03-30, this workspace still has no recorded live VMware capability
in `/Volumes/Development/pulse/LOCAL_CAPABILITIES.md`.

That means this contract is ready for implementation planning, but its support
claim still depends on live proof of:

1. alarm and health signal quality
2. event and task attachment fidelity
3. Assistant read usefulness on real VMware-backed resources
4. continued absence of VMware control exposure

## Primary Source Basis

1. managed-entity alarm lookup:
   [Alarm Manager Get Alarm](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/AlarmManager/moId/GetAlarm/post/)
2. filtered event retrieval:
   [Event Manager Query Events](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/EventManager/moId/QueryEvents/post/)
3. host and VM triggered alarm state families:
   [Host System APIs](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/virtual-infrastructure/host-system/)
   [Virtual Machine APIs](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/virtual-infrastructure/virtual-machine/)
4. datastore triggered alarm state family:
   [Datastore APIs](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/virtual-infrastructure/datastore/)
5. host-plus-child performance context:
   [Performance Manager Query Perf Composite](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/PerformanceManager/moId/QueryPerfComposite/post/)
6. recent-task context example:
   [Virtual Machine Get Recent Task](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/VirtualMachine/moId/recentTask/get/)
