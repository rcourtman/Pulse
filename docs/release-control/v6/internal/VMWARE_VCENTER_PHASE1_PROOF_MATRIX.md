# VMware vCenter Phase-1 Proof Matrix

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_API_RUNTIME_SPEC.md`
- `docs/release-control/v6/internal/VMWARE_VSPHERE_PHASE1_EXECUTION_PLAN.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_ALERTS_AND_ASSISTANT_SPEC.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_ONBOARDING_SPEC.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_RESOURCE_PROJECTION_SPEC.md`

Use this matrix as the live-proof runbook for the trust-critical boundary
between:

1. a planned VMware architecture
2. an implemented VMware onboarding and projection path
3. an honest claim that VMware is actually supported in Pulse

This matrix does not replace repo-local tests.
It exists because VMware can look correct in isolated API, poller, or UI work
while still failing the real support floor on a live `vCenter`.

## Governing Policy

The locked VMware phase-1 rule is:

1. `vmware-vsphere` is one first-class platform id
2. `vCenter` is the only supported entry point in phase 1
3. ingestion is `api-backed`
4. ESXi hosts project as canonical `agent`
5. VMs project as canonical `vm`
6. datastores project as canonical `storage`
7. alerts and assistant read may be supported
8. recovery stays `n/a`
9. assistant control stays `read-only`

## Scope

In scope:

1. shared platform-connections onboarding to `vCenter`
2. saved-connection test and health summary behavior
3. inventory projection for hosts, VMs, and datastores
4. alarm, event, task, snapshot, and metrics/history visibility where the
   declared floor depends on them
5. assistant read on canonical VMware-backed resources

Out of scope:

1. direct `ESXi`
2. unified-agent-required bootstrap
3. `physical-disk`, `system-container`, or `app-container` support
4. recovery-artifact or restore claims
5. assistant or operator control for VMware actions

## Environment Prerequisites

Before running this matrix, provide a real VMware environment with:

1. one reachable `vCenter`
2. one service account candidate for phase-1 read-first support
3. at least one ESXi host
4. at least one VM
5. at least one datastore
6. at least one recent event or task visible through vCenter history
7. ideally one VM snapshot so snapshot-tree visibility can be proven

Record the non-secret capability metadata first in `LOCAL_CAPABILITIES.md`.
If that record does not exist, the matrix is not runnable.

## Proof Record Contract

Until a successful live proof exists, keep the current blocked record at:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-blocked-2026-03-30.md`

When the first real environment is exercised successfully, replace that blocked
state with a dated proof record at:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-<YYYY-MM-DD>.md`

Use
`docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_RECORD_TEMPLATE.md`
to capture both the `LOCAL_CAPABILITIES.md` entry shape and the successful
proof record shape.

That dated record should summarize:

1. the environment alias and `LOCAL_CAPABILITIES.md` entry used
2. `vCenter` version and build information
3. the exact privilege bundle used for the pass
4. pass/fail notes for `VC-0` through `VC-7`
5. captured evidence for projection, alerts/history, and Assistant read
6. explicit confirmation that direct `ESXi`, recovery support, and control
   stayed out of scope

## Automated Proof Floor

Before calling VMware supported, Pulse should have automated proof for at least:

1. backend API contract coverage for `/api/vmware/connections*`
2. saved-secret preservation and saved-connection retest behavior
3. projection of VMware-backed `agent`, `vm`, and `storage` resources without
   provider-local top-level types
4. alert and history mapping on the shared paths
5. assistant read classification and capability exposure remaining read-only

If the automated floor is missing, the manual drill may inform implementation,
but it must not be used as the sole basis for a support claim.

Current repo-local automated proof already covers part of the read-only
boundary:

1. `tests/integration/tests/38-vmware-ai-chat-mentions.spec.ts` proves
   canonical VMware-backed Assistant mentions stay on shared resource IDs and
   shared Assistant routes.
2. `internal/ai/chat/context_prefetch_additional_test.go` proves VMware-backed
   guest mentions stay read-only in Assistant context summaries.
3. `internal/ai/chat/service_tooling_test.go` and
   `internal/ai/tools/control_resource_test.go` prove shared Assistant/tool
   wording stays capability-gated instead of claiming blanket VM control.
4. `internal/api/route_inventory_test.go` and
   `tests/integration/tests/41-vmware-phase1-exclusion-integrity.spec.ts`
   prove the shipped route and browser surface stay out of direct `ESXi`,
   recovery, and VMware admin-plane territory.

## Scenario Matrix

| ID | Scenario | Pass focus |
| --- | --- | --- |
| `VC-0` | Capability registration | Live `vCenter` environment is recorded in `LOCAL_CAPABILITIES.md` with safe metadata |
| `VC-1` | Draft connection test | New `vCenter` draft validates through the shared setup path with clean failure classification |
| `VC-2` | Saved connection retest | Stored credentials can be re-tested without forcing secret re-entry |
| `VC-3` | Inventory projection floor | ESXi hosts, VMs, and datastores land as canonical `agent`, `vm`, and `storage` |
| `VC-4` | Storage and snapshot visibility | Datastore state and snapshot-tree visibility appear without inflating recovery claims |
| `VC-5` | Alerts and history floor | Shared alerts can surface VMware alarm and history context |
| `VC-6` | Assistant read floor | Assistant can inspect canonical VMware-backed resources without VMware-specific tools |
| `VC-7` | Exclusion integrity | No direct `ESXi`, recovery, or control claim leaks into the shipped behavior or wording |

## Execution Steps

### `VC-0` Capability Registration

1. Record the environment in `LOCAL_CAPABILITIES.md` with safe access notes,
   verification command, and any non-secret environment label.

Pass when:

1. the capability is recorded
2. a future agent can discover that a live VMware proof environment exists
   without reading secrets

### `VC-1` Draft Connection Test

1. Open the shared `Platform connections` workspace.
2. Create a new VMware draft with the phase-1 connection fields.
3. Run the draft test path before saving.
4. Intentionally exercise at least one bad-path case such as bad host, TLS
   mismatch, or insufficient privileges.

Pass when:

1. the draft validates through the shared VMware setup path
2. failures classify clearly instead of collapsing into generic unknown error
3. the path does not require a unified agent or direct `ESXi` routing
4. a green result reflects the declared phase-1 floor rather than one partial
   VMware API-family success

### `VC-2` Saved Connection Retest

1. Save a working VMware connection.
2. Re-test it through the saved-connection test path.
3. Re-test it again with a partial edit payload that preserves unchanged
   secrets.
4. Reload the connection list.

Pass when:

1. the saved-connection test succeeds without re-entering masked secrets
2. edited saved-connection tests can reuse stored secrets server-side
3. the list reflects refreshed last-success or last-error state
4. the saved-connection contract still hides any dual-client runtime detail
   behind one connection health result

### `VC-3` Inventory Projection Floor

1. Let the VMware poller collect from the saved connection.
2. Inspect the infrastructure/workload/storage surfaces or canonical APIs.
3. Compare the observed resources against the governed projection spec.
4. Confirm hosts, VMs, and datastores are visible.

Pass when:

1. ESXi hosts land as canonical `agent`
2. guest workloads land as canonical `vm`
3. datastores land as canonical `storage`
4. no provider-local top-level `esxi-host`, `vsphere-vm`, or equivalent type
   appears
5. topology metadata stays metadata and does not appear as synthetic top-level
   VMware resource types

### `VC-4` Storage And Snapshot Visibility

1. Inspect datastore state through the shared storage surface.
2. Inspect VM detail for snapshot-tree visibility.
3. Confirm any recovery-adjacent wording stays descriptive, not product-claim
   inflation.

Pass when:

1. datastore capacity, free space, and accessibility are visible on the shared
   path
2. snapshot-tree visibility exists when the upstream environment exposes it
3. Pulse does not present VMware as recovery-supported just because snapshots
   are readable

### `VC-5` Alerts And History Floor

1. Inspect shared alerts or incident views after VMware collection.
2. Confirm VMware-backed alarm or health context can be reached.
3. Confirm related event/task context is available where the shared product
   expects it.
4. Confirm topology-scoped alarm context does not appear as a VMware-only
   incident resource.

Pass when:

1. alerts route through shared alert and incident surfaces
2. VMware-backed context is readable there without a provider-local shell
3. the product does not imply VMware-specific alarm management
4. event and task context lands on shared incident or resource-timeline paths,
   not a VMware-only history surface

### `VC-6` Assistant Read Floor

1. Use the shared Assistant read paths against canonical VMware-backed
   resources.
2. Inspect a host, a VM, and a datastore.
3. Confirm shared tool exposure stays read-only.

Pass when:

1. Assistant reads work on canonical `agent`, `vm`, and `storage`
2. there are no VMware-specific tools or verbs required
3. no control capability or VMware-specific action metadata is exposed as part
   of the VMware phase-1 floor

Current automated coverage:

1. `tests/integration/tests/38-vmware-ai-chat-mentions.spec.ts`
2. `internal/ai/chat/context_prefetch_additional_test.go`
3. `internal/ai/chat/service_tooling_test.go`
4. `internal/ai/tools/control_resource_test.go`

### `VC-7` Exclusion Integrity

1. Review shipped wording, route inventory, tool inventory, and visible product
   behavior.
2. Confirm no broader support claim leaked in while implementing the floor.

Pass when:

1. direct `ESXi` is still out of scope
2. recovery support is still `n/a`
3. assistant control is still `read-only`
4. no VMware admin-plane action surface shipped by implication

Current automated coverage:

1. `internal/api/route_inventory_test.go`
2. `tests/integration/tests/41-vmware-phase1-exclusion-integrity.spec.ts`
3. `internal/ai/tools/tools_query_test.go`
4. `internal/ai/tools/control_resource_test.go`

## Evidence To Capture

Capture all of the following outside git or in the dated release-control record:

1. the `LOCAL_CAPABILITIES.md` capability entry
2. `vCenter` version/build information
3. the exact privilege bundle used for the successful proof run
4. connection-list screenshots or payloads showing poll health and observed
   contribution summary
5. canonical resource screenshots or payloads showing `agent`, `vm`, and
   `storage` projection
6. alert or incident screenshots/payloads showing VMware-backed context
7. assistant read transcript or screenshots showing canonical VMware-backed
   resource inspection
8. explicit note that recovery and control stayed out of scope

The expected success-record path is:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-<YYYY-MM-DD>.md`

Use the companion template:

`docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_RECORD_TEMPLATE.md`

## Failure Rules

Do not call VMware supported if any of these are true:

1. no live `vCenter` capability is recorded
2. the privilege bundle is guessed rather than proven
3. the supported version floor is guessed rather than proven
4. VMware inventory lands on provider-local top-level resource types
5. the product wording or runtime behavior implies direct `ESXi` support
6. the product wording or runtime behavior implies recovery support
7. the product wording or runtime behavior exposes VMware control as part of
   phase 1

## Current Blocker

As of 2026-03-30, `VC-0` is not yet satisfied in this workspace.
There is still no recorded VMware capability in
`/Volumes/Development/pulse/LOCAL_CAPABILITIES.md`.

That means the planning and implementation path can continue, but the support
claim remains blocked until a real proof environment is available.

Current blocked record:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-blocked-2026-03-30.md`
