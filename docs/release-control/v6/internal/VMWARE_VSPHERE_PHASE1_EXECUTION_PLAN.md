# VMware vSphere Phase-1 Execution Plan

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `status.json.resolved_decisions.vmware-vsphere-vcenter-first-admission-model`

## Intent

Pulse now has a locked architecture recommendation for VMware vSphere.
This document turns that recommendation into an executable phase-1 plan so
implementation can start without drifting into a provider-specific island.

## Support Decision

Pulse should add VMware support, but only in one narrow form:

1. first-class platform id: `vmware-vsphere`
2. supported entry point: `vCenter` only
3. primary ingestion model: `api-backed`
4. phase-1 resource projections: canonical `agent`, `vm`, and `storage`
5. phase-1 product stance: read-first
6. assistant control: `read-only`
7. recovery support: `n/a`

Implementation may start next on that basis.
If live validation disproves the minimum privilege bundle, supported version
floor, or declared support-floor coverage, Pulse should stop at governance and
must not widen the support claim.

## Product Sentence

Pulse phase-1 VMware support means a shared platform-connections path to
`vCenter` that projects ESXi hosts, VMs, and datastores into the canonical
Pulse resource model, surfaces alarms and history through the shared product
surfaces, and keeps assistant behavior read-only.

## Why This Is A Separate Lane Slice

This is not just another provider adapter.

It crosses:

- platform onboarding and saved-connection health
- canonical inventory and identity projection
- shared infrastructure, workload, and storage presentation
- shared alert and incident surfaces
- assistant read classification and action boundaries
- support-floor proof before a matrix claim changes

That makes VMware a cross-surface admission problem, not a monitoring-only
implementation detail.

## Phase-1 Scope

Phase 1 should include:

1. `vCenter` onboarding through the shared `Platform connections` workspace
2. host inventory projected as canonical `agent`
3. VM inventory and runtime state projected as canonical `vm`
4. datastore inventory and capacity state projected as canonical `storage`
5. topology metadata for datacenter, cluster, folder, resource-pool, and
   placement relationships under those shared resources
6. vSphere alarm state, overall health, and related event/task history through
   shared alert and incident surfaces
7. snapshot-tree visibility and recovery-adjacent VM context as read-side
   workload detail
8. assistant read on canonical VMware-backed resources through shared read
   paths only

## Explicit Exclusions

Phase 1 should not include:

1. direct `ESXi` onboarding
2. agent-first VMware support
3. `physical-disk`, `system-container`, or `app-container` projections
4. treating vSphere snapshots as Pulse recovery support
5. restore, failover, changed-block recovery, or backup-orchestration claims
6. provider-local VMware pages, top-level resource types, or AI tools
7. assistant or operator control for VM power, snapshot lifecycle, guest
   operations, host maintenance, or cluster administration

## Execution Slices

### 1. Platform Connection Admission

Owners:
- `agent-lifecycle`
- `api-contracts`

Slice contract:
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_ONBOARDING_SPEC.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_API_RUNTIME_SPEC.md`

Deliver:

- `vCenter` added only through the shared platform-connections workspace
- one saved-connection contract for create, update, masked-secret preservation,
  test, health, and contribution summary
- one backend contract for VMware session ownership, provider ownership, dual
  API-family access, and stable failure classification under that same saved
  connection model
- explicit disabled/default behavior, reconnect semantics, and credential
  ownership on the shared platform-connections path

Required proof:

- real `vCenter` connection succeeds through the governed setup path
- auth, TLS, endpoint, and permission failures classify cleanly
- minimum privilege bundle is written down from live validation, not inferred
- supported version floor is written down from live validation, not inferred
- a real `vCenter` capability is recorded in `LOCAL_CAPABILITIES.md` so the
  support-floor proof path is explicit and reusable

### 2. Canonical Projection Floor

Owners:
- `monitoring`
- `unified-resources`

Slice contract:
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_RESOURCE_PROJECTION_SPEC.md`

Deliver:

- API-backed ingest from the official vCenter Automation API plus the Virtual
  Infrastructure JSON API
- one canonical VMware source classification and `platformType:
  vmware-vsphere` on shared resources instead of provider-local source forks
- stable identity and dedupe for ESXi hosts, VMs, and datastores
- shared-resource projection with no provider-local `esxi-host`,
  `vsphere-vm`, or `vsphere-datastore` top-level types
- relationship metadata for cluster, folder, placement, and datastore
  attachment without promoting those objects to new top-level Pulse types

Required proof:

- one real `vCenter` produces canonical `agent`, `vm`, and `storage`
  resources end to end as defined in the projection spec
- topology, identity, and counts stay coherent across infrastructure and
  workload surfaces
- no phase-1 path depends on a unified agent being installed on ESXi or guests

### 3. Alerts, History, And Assistant Read

Owners:
- `alerts`
- `monitoring`
- `ai-runtime`

Slice contract:
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_ALERTS_AND_ASSISTANT_SPEC.md`

Deliver:

- vSphere alarm and health signals routed through shared alert/incident paths
- event/task/performance context available where the shared monitoring and
  history model expects it
- assistant read on canonical VMware-backed resources through shared tools and
  read-state views
- no VMware-specific control verbs, remediation tools, or admin-plane promises

Required proof:

- shared alerts can drill into VMware-backed incidents without a provider-local
  shell
- assistant can inspect canonical VMware-backed resources without special-case
  VMware tools or VMware-specific capability exposure
- assistant control remains read-only in both product wording and runtime
  capability exposure

### 4. Support Claim Ratchet

Owners:
- release-control governance
- owning subsystem contracts above

Companion proof matrix:
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_RECORD_TEMPLATE.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_RESOURCE_PROJECTION_SPEC.md`
- `docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-blocked-2026-03-30.md`

Deliver:

- version floor documented
- minimum privilege bundle documented
- explicit phase-1 exclusions documented in support language
- support matrix changed only after live proof exists
- successful proof record written from the shared VMware proof template
- blocked proof state replaced by a dated successful proof record only after the
  first real `vCenter` pass

Required proof:

- onboarding, inventory, storage, alerts, metrics/history, and assistant read
  all hold on a real `vCenter`
- support wording matches the narrow floor exactly
- the implementation can fail closed without silently inheriting direct-ESXi or
  recovery support claims

## What Must Be True Before Pulse Can Say VMware Is Supported

Pulse should not call VMware supported until all of these are true:

1. `vCenter` onboarding works through the shared platform-connections path
2. the minimum required privilege bundle is validated on a real environment
3. the supported `vCenter` version floor is validated on a real environment
4. ESXi hosts, VMs, and datastores project cleanly into canonical `agent`,
   `vm`, and `storage`
5. alarm, health, and metrics/history signals appear on the shared alert and
   monitoring surfaces
6. assistant read works on those canonical resources without VMware-specific
   tools
7. product wording keeps direct `ESXi`, recovery, and broad control out of the
   support claim

## Known Gaps Requiring Live Validation

These are not reasons to block the planning decision, but they are reasons to
block a support claim until proven:

1. exact minimum privilege bundle for a production-grade read-first floor
2. exact supported `vCenter` version floor
3. signal quality and consistency of alarms, events, and history across that
   version floor
4. whether guest-identity and snapshot-detail fidelity is sufficient in mixed
   VMware Tools coverage environments

## Primary Source Basis

The architecture lock and this execution plan are grounded in the official
VMware/Broadcom APIs:

1. vCenter and inventory/control surface:
   [vCenter VM APIs](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/)
2. detail and topology surface:
   [Virtual Infrastructure JSON API](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/)
3. alarms:
   [AlarmManager GetAlarm](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/AlarmManager/moId/GetAlarm/post/)
4. snapshots:
   [VirtualMachine snapshot API](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/VirtualMachine/moId/snapshot/get/)
5. performance/history:
   [PerformanceManager QueryPerfComposite](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/PerformanceManager/moId/QueryPerfComposite/post/)

## Start Decision

Implementation should start next only as slice 1 of this plan.
If slice 1 cannot prove the minimum privilege bundle and supported version
floor on a real `vCenter`, Pulse should stop and record that gap instead of
moving forward as if VMware were already supportable.
