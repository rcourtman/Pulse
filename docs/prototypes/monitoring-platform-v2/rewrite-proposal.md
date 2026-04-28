# Monitoring Page Rewrite Proposal

## Status

This is a prototype direction, not a production implementation contract. The real Monitoring page rewrite should wait until the unified resource model can represent the relationships and fields this surface needs without UI-local workarounds.

## Release Gate: No Invented Columns

The Monitoring page must not invent columns, row fields, grouping metadata, or synthetic operational facts in the frontend.

Every visible column must be backed by one of:

- a field already exposed by the canonical unified resource model
- a typed relationship edge exposed by the canonical unified resource model
- a documented metrics-history API contract
- a documented extension to the unified resource model that lands before the UI binds to it

If a proposed column cannot be mapped to one of those sources, it does not ship in the production Monitoring page. It can appear in prototype notes only if it is explicitly marked as a required model extension.

## Required Relationship Contract

`ParentID` remains the primary nesting spine for the default tree view, but it is not enough for Monitoring. The production page needs a queryable typed-edge graph so resources can participate in multiple real relationships without forcing the UI to pick one misleading parent.

The relationship contract should expose closed-enum roles. The initial Monitoring-required enum must include:

- `runs_on`
- `contains`
- `uses_storage`
- `backed_up_by`
- `member_of`
- `replicates_to`
- `depends_on`

Adding a role should be treated as a contract change, not a frontend convention. Both directions need to be queryable, so a backup job can answer "what does this protect?" and a VM can answer "what protects me?" from the same canonical graph.

## Required Model Work Before Rewrite

1. Typed resource relationship edges with the role enum above and reverse lookup support.
2. First-class cluster resources for platform-level state across Proxmox, Kubernetes, vSphere, Ceph, and equivalent cluster-capable sources.
3. Metrics-history API schema for row trends, detail drawers, storage growth, and anomaly context.
4. First-class liveness fields separate from incidental `LastSeen`.
5. Recovery/protection fields for target, last run, verification state, and protected-resource linkage.

## Prototype Field Mapping

The current v2 prototype should be read through this mapping:

- `Resource`: canonical identity display name
- `Kind`: canonical resource type
- `Info`: compact source-backed facet summary, not product narration
- resource state indicator: shown directly before the resource name, derived from first-class liveness plus health state. `On` is green, `Off` is red, and `Issue` is yellow.
- `CPU`, `Memory`, `Disk`: latest resource metrics, with history supplied by the metrics API later
- `Recovery`: canonical recovery/protection posture
- `Signals`: incidents, alerts, or canonical health signals
- `Placement`: `ParentID` plus typed edges such as `runs_on`, `member_of`, or `contains`
- `Target`: typed recovery/replication target via `backed_up_by` or `replicates_to`
- `Last Run`, `Verification`: recovery/protection model fields

## Platform Rule

The rewrite must not bias one platform. Proxmox, Docker, Kubernetes, TrueNAS, vSphere, and future first-class platforms should all project into the same Monitoring primitives: identity, type, status, metrics, typed relationships, liveness, storage, recovery, and health signals.

## Layout Rule

The Monitoring page should keep one platform selection model. Do not split platform choice into duplicate concepts such as workspace and focus.

The default surface should remain a dense, segmented table. Visual grouping should come from canonical resource structure and typed relationships: platform, primary/top-level resource, then operational lanes such as workloads, storage, and recovery. If those relationships are not available from the unified resource model, the production UI should wait for the model contract rather than inventing a frontend-only hierarchy.

When a platform is focused, the primary layout should become relationship-first rather than type-first. A user looking at a host should see the workloads on that host, and any attached storage, network devices, recovery jobs, or protection state should appear under the workload or host they belong to. Type filters can narrow the view, but they must not be the main organizing principle because they separate related resources and force the user to reconstruct the topology mentally.

Each filter state should be treated as a monitoring intent with its own view model. Overview should emphasize topology. Workload views should emphasize runtime units and their attachments. Storage views should emphasize ownership and attachment points. Recovery views should emphasize protected resources and backup/replication policies. Problem views should preserve enough context to explain what is affected. A single generic grouping algorithm is not sufficient.

Users should also be able to compose the density of the page. The Monitoring surface needs layer toggles for top-level resources, workloads, storage, and recovery so an operator can choose "hosts only", "hosts plus workloads", or "hosts plus workloads plus storage" without losing the hierarchy.

The filter controls should be preset-first, not combination-first. Platform selection answers "which estate am I looking at?", Focus answers "what monitoring job am I doing?", and Include answers "which related layers should stay visible for context?" Focus choices should apply sensible layer defaults such as Top Level only, Workloads with owners, Storage with owners and attached workloads, Recovery with protected resources, and Problems with enough context to explain impact. Operators can then adjust Include toggles, but they should not have to know hidden layer dependencies to get a useful view.
