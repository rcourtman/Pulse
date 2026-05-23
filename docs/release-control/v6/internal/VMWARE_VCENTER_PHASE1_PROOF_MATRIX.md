# VMware vCenter Phase-1 Proof Matrix

Last updated: 2026-05-23
Status: ACTIVE
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
2. an implemented VMware phase-1 floor that may be `first-lab-ready`
3. an honest claim that VMware is actually supported in Pulse

This matrix does not replace repo-local tests.
It exists because VMware can look correct in isolated API, poller, or UI work
while still failing the real support floor on a live `vCenter`.
It also defines when the non-live harness may honestly say `first-lab-ready`
instead of collapsing all progress into the same `not supported` bucket.

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
3. the exact privilege bundle used for the pass, including the privilege
   required for `GET /api/vcenter/vm/{vm}/guest/local-filesystem` (the
   guest filesystem read added for `VC-8`'s operational metric surface)
4. pass/fail notes for `VC-0` through `VC-8`
5. captured evidence for projection, alerts/history, Assistant read, and
   the new uptime / guest disk usage cells on the canonical workload table
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
2. `tests/integration/tests/42-vmware-ai-chat-read-recovery.spec.ts` proves a
   failed VMware `pulse_read action=logs resource_id=...` path can recover onto
   shared `pulse_query` behavior without VMware-local routes.
3. `internal/ai/chat/context_prefetch_additional_test.go` proves VMware-backed
   guest mentions stay read-only in Assistant context summaries.
4. `internal/ai/chat/service_tooling_test.go` and
   `internal/ai/tools/control_resource_test.go` prove shared Assistant/tool
   wording stays capability-gated instead of claiming blanket VM control.
5. `internal/api/route_inventory_test.go` and
   `tests/integration/tests/41-vmware-phase1-exclusion-integrity.spec.ts`
   prove the shipped route and browser surface stay out of direct `ESXi`,
   recovery, and VMware admin-plane territory.

## First-Lab-Ready Checkpoint

Pulse may classify VMware `first-lab-ready` before live proof only when all of
these are true:

1. the governed architecture is locked
2. the bounded phase-1 floor is implemented on shared paths
3. automated non-live proof covers shared onboarding, canonical projection,
   alerts/history, Assistant read, and exclusion integrity
4. the remaining blockers to a support claim are live-only facts rather than
   missing shared product work

`first-lab-ready` means the next proper move is a real `vCenter` run.
It does not mean VMware is supported, marketed, or admitted into the support
matrix.

Current state as of 2026-03-31:

1. VMware meets the governed `first-lab-ready` checkpoint.
2. The next highest-value step is a real `vCenter` proof pass through
   `VC-0` through `VC-7`.
3. Repeating `not supported` without also reporting `first-lab-ready` is no
   longer an accurate description of implementation progress.

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
| `VC-8` | Operational metric surface | Host and VM uptime plus VM guest filesystem usage land on the canonical workload table for VMware-backed resources without silent gaps |

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
2. failures classify clearly instead of collapsing into generic unknown error,
   including `tls`, `network`, `auth`, `permission`, and
   `unsupported_version` where applicable
3. the path does not require a unified agent or direct `ESXi` routing
4. a green result reflects the declared phase-1 floor rather than one partial
   VMware API-family success

Automated non-live proof should continue to cover the shared browser path for
`unsupported_version`, `auth`, `tls`, and `network` classifications so the
first real lab run is not the first operator-visible exercise of those failure
shapes.

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

Automated non-live proof should continue to cover the shared settings shell for
saved-connection auth/runtime failure classification and degraded permission
guidance so the first real lab run does not discover those operator-facing
states for the first time.

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
2. `tests/integration/tests/42-vmware-ai-chat-read-recovery.spec.ts`
3. `internal/ai/chat/context_prefetch_additional_test.go`
4. `internal/ai/chat/service_tooling_test.go`
5. `internal/ai/tools/control_resource_test.go`

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

### `VC-8` Operational Metric Surface

Phase-1's resource projection (`VC-3`) and storage/snapshot visibility
(`VC-4`) prove that VMware-backed entities land as canonical types and that
datastore + snapshot context is readable. They do not, by themselves,
prove that the operational columns the canonical workload table renders for
every other platform — host uptime, VM uptime, VM guest filesystem usage —
actually populate against a real `vCenter`. This scenario locks that gap
explicitly so it cannot slip behind the existing projection pass.

The new code paths under proof here are:

1. PerformanceManager counter additions in `internal/vmware/client_metrics.go`:
   `sys.uptime.latest` for hosts and VMs, and `sys.osUptime.latest` for VMs.
   The mapping prefers guest OS uptime (Tools-reported) and falls back to
   VMX-process uptime when the guest counter is absent.
2. New per-VM REST collector in `internal/vmware/client_topology.go`:
   `GET /api/vcenter/vm/{vm}/guest/local-filesystem`, aggregated across mount
   points into `InventoryMetrics.DiskTotalBytes` / `DiskUsedBytes` /
   `DiskPercent` and projected onto canonical `ResourceMetrics.Disk`.
3. Canonical projection in `internal/vmware/provider.go`: `UptimeSeconds`
   lands on `Resource.Uptime` for hosts and VMs; the disk fields land on
   `metrics.disk` so the shared workload table renders the same shape every
   other platform uses.

Steps:

1. Confirm the candidate vCenter service account can read
   `PerformanceManager.perfCounter` and successfully query
   `sys.uptime.latest` against a sample host and VM.
2. Confirm the same account can issue
   `GET /api/vcenter/vm/{vm}/guest/local-filesystem` against a
   powered-on VM with VMware Tools running, and that the response decodes
   as `map[string]VmGuestLocalFilesystemInfo` (each entry carrying
   `capacity` and `free_space`).
3. Open the vSphere overview page; confirm a powered-on VM row renders a
   real "N days" Uptime cell and a non-empty DISK column with both percent
   and human-readable bytes.
4. Confirm a powered-off VM, a VM with VMware Tools stopped, and a VM the
   service account is not permitted to read for guest operations each
   surface their gap correctly: Uptime / DISK stay blank (not "0" or
   "0 B / 0 B") and the snapshot's enrichment-issues array records a
   non-fatal `unavailable` (Tools missing) or `permission` (insufficient
   privilege) entry rather than failing the entire poll.
5. Confirm `sys.osUptime.latest` is collected when the vCenter is
   configured at statistics level 4 (real-time interval), and that
   the fallback to `sys.uptime.latest` engages cleanly when the level-4
   counter is not exposed.

Pass when:

1. Host and VM `Resource.Uptime` populate against the live vCenter for
   any powered-on entity that vCenter would normally surface uptime for.
2. `metrics.disk` on a powered-on VM with Tools running carries a
   non-zero `Total` and a `Percent` derived from real guest filesystem
   reads, with at least one realistic mount point present.
3. The shared workload table renders these on the vSphere overview without
   the operator having to toggle the Columns menu.
4. Missing-data cases (Tools stopped, VM powered off, privilege gap) flow
   through the existing non-fatal enrichment-issue path with an
   actionable category (`unavailable` for Tools, `permission` for
   privilege) rather than failing the whole poll.
5. The exact privilege required for
   `GET /api/vcenter/vm/{vm}/guest/local-filesystem` is captured in the
   dated proof record so the documented "minimum privilege bundle" can be
   tightened from "guessed" to "verified."

Current automated coverage:

1. `internal/vmware/client_test.go`'s degraded-enrichment test exercises
   the new endpoint's happy path (multi-mount filesystem response) and
   the `unavailableVMGuestInfo` Tools-not-running 503 path.
2. `internal/mock/platform_fixtures_test.go` asserts the synthesized
   uptime + guest disk values flow through the canonical projection for
   powered-on VMs and drop cleanly for powered-off VMs.
3. `frontend-modern/src/hooks/__tests__/useWorkloads.test.ts` asserts
   the `Resource.Uptime` fallback so vSphere uptime lands on
   `WorkloadGuest.uptime` despite vSphere not populating a
   platform-specific carve-out.

Open unknown (track in the dated proof record):

The exact vCenter privilege required for the guest local-filesystem read.
Public documentation does not pin this down, and Pulse's existing
"minimum privilege bundle" entry in
`VMWARE_VCENTER_PHASE1_ONBOARDING_SPEC.md` already names privilege
verification as a live-environment unknown. The first real `VC-8` pass
should capture the exact privilege name so subsequent customer setup
docs can ship it explicitly.

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
