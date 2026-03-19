# Pulse v6 Bridge Release Foundation Spec

Last updated: 2026-03-17
Status: ACTIVE

## Intent

Pulse v6 has a bridge-release direction from "monitoring product growing
sideways" toward "resource + policy + control platform," but the current
stabilization target stays on the monitoring-and-alerting RC floor until that
direction is proven.

It is not a universal agent sandbox.
It is not the full private operational broker.
It must, however, land the primitives that make the right direction
irreversible: Pulse becomes the infrastructure-specific context, policy, and
action plane that sandboxed agents can use.

The future bridge-release product sentence is:

Pulse v6 becomes resource-centric, policy-aware, and action-ready once the
broader surfaced case is proven.

## Required Outcomes

1. The center of the product is the resource, not the metric.
2. Pulse Agent is the universal collection host for multi-platform adapters.
3. Changes are first-class domain objects, not metric side effects.
4. Sensitivity and routing metadata are part of the model, not later policy
   decoration.
5. Actions use a governed model with capability declaration, approvals, and
   auditability.
6. Fleet governance is a first-class product surface, not an installer detail.

## Must-Have Backend Changes

### 1. Resource-first canonical model

- Every important object must exist as a first-class resource: machine, VM,
  container, storage pool, service, dataset, alert, incident, and agent.
- Each resource must carry canonical identity, parent/child or dependency
  relationships, current health, recent changes, declared capabilities,
  sensitivity classification, and an AI-safe summary surface.
- Platform-specific objects may still exist at adapters or migration
  boundaries, but they must not remain the primary runtime truth.

### 2. Agent envelope and adapter host

- Pulse Agent must own pluggable adapters for host/process, Docker, Proxmox,
  TrueNAS, and generic service collection.
- Adapters must emit one standardized internal envelope family:
  `resource-upsert`, `signal-observed`, `change-detected`, and
  `capability-declared`.
- Adding integrations is not the success criterion by itself; convergence onto
  the canonical envelope is.

### 3. First-class change intelligence

- The backend must own a canonical change/event model instead of treating
  change as inferred UI garnish on top of metrics.
- Changes must support typed facts such as restart, migration, degraded
  storage, config drift, agent upgrade, and snapshot creation.
- Resources, incidents, and timelines must be able to answer `what changed`,
  `what changed first`, and `what else is related` from structured change
  history.

### 4. Policy-aware data governance hooks

- Resources and major signal/change types must support at least
  `public`, `internal`, `sensitive`, and `restricted` classifications.
- The model must carry redaction and routing metadata, including
  local-only versus cloud-eligible handling.
- AI-facing flows must consume AI-safe summaries and redacted payloads by
  contract instead of assuming raw dumps are always allowed to leave the local
  boundary.
- v6 does not need the full enterprise policy engine, but the data model and
  runtime boundaries must make later policy enforcement straightforward.

### 5. Action framework v1

- Resources must declare supported actions and the requirements for each
  action.
- The action model must support `plan`, `dry-run`, `approval-required`, and
  constrained execution of explicitly safe actions.
- Execution must record actor, target resource, parameters, result, and
  approval context in an audit trail.
- v6 should not ship broad autonomous remediation.

### 6. Fleet governance v1

- Fleet state must expose enrollment, liveness, version drift, adapter health,
  config state, credential status, and update status.
- Remote configuration and rollout control may be limited in scope, but the
  fleet itself must become a governed product area rather than a background
  transport assumption.
- The fleet model must line up with resource identity and policy boundaries
  instead of becoming a parallel ad hoc registry.

## Must-Have UI Changes

### 1. Resource-first information architecture

- Primary infrastructure navigation and detail views must lead with resources,
  relationships, health, recent changes, capabilities, and sensitivity.
- Metrics remain important, but they become one aspect of a resource page
  rather than the top-level product ontology.

### 2. Change and timeline visibility

- Every major resource page must surface recent changes and correlated events.
- Incidents and investigations must show cross-resource timelines instead of
  only threshold or metric snapshots.

### 3. Visible policy and routing state

- Operators must be able to see resource sensitivity, whether content is
  local-only or cloud-eligible, and when redaction is applied.
- AI-related views must make it obvious that summaries are policy-shaped and
  not raw unrestricted data egress.

### 4. Governed action UX

- Actions must be shown as declared capabilities on resources.
- The UI must distinguish planning, dry-run, approval waiting, execution, and
  audited completion states.
- Approval surfaces must align with the canonical action model rather than
  remaining a Patrol-only side path.

### 5. Fleet as a product area

- The product must expose an explicit fleet surface for agent enrollment,
  health, drift, adapter status, rollout status, and credential issues.
- Fleet management must feel central to the platform, not buried inside setup.

## Explicit Deferrals

- Broad autonomous remediation.
- Full replacement of native vendor control planes.
- Deep per-vendor feature parity beyond the canonical resource/policy model.
- Heavy chat-first UX as the primary control surface.
- Turning Pulse into the generic sandbox where arbitrary agents execute.
- Full "private operational broker" scope in one release.

## Governance Mapping

- Resource change intelligence: retained as hidden backend
  foundations in the current L6/L13 overlap, while the surfaced lane stays
  deferred until the cross-resource investigation case is proven.
- Policy-aware data governance: the next surfaced target after RC stabilization,
  promoted from the current L6/L14 overlap.
- Action governance and auditability: promoted from the current L6/L14 trust
  surfaces and adjacent approvals work.
- Fleet governance and rollout control: promoted from the current L16 floor.

These promotions are tracked in `docs/release-control/v6/status.json` as
coverage gaps and candidate lanes under the planned target
`v6-product-lane-expansion`.
