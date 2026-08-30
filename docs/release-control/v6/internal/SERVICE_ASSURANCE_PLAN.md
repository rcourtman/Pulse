# Infrastructure-Aware Service Assurance Plan

Last updated: 2026-08-30
Status: ACCEPTED — SLICES A, B, AND C IMPLEMENTED
Governance surfaces:

- `status.json.coverage_gaps.infrastructure-aware-service-assurance`
- `status.json.candidate_lanes.infrastructure-aware-service-assurance`
- `docs/AVAILABILITY_HISTORY_CONTRACT.md`

## Intent

Pulse should extend from monitoring infrastructure state into proving whether
that infrastructure is delivering the services the operator expects.

The target is not a broader clone of an endpoint-monitoring product. Pulse
already knows about hosts, virtual machines, containers, storage, platform
connections, discovered services, maintenance intent, alerts, resource
relationships, and remote agents. Service assurance must combine those facts
with active verification so an operator can see not only that an endpoint
failed, but which service is affected, which infrastructure layer is still
healthy, what else depends on the failing layer, and how strong the available
evidence is.

This direction crosses monitoring, unified resources, alerts, agent execution,
fleet presentation, resource history, and Patrol. It therefore needs one
governed product lane rather than a series of unrelated additions to the
availability target form.

## Product Sentence

Pulse shows whether infrastructure is successfully delivering its services,
identifies the infrastructure layer most likely responsible when delivery
fails, and keeps the evidence attached to the resources operators already
manage.

## Strategic Distinction

The product boundary is deliberate:

- Endpoint-monitoring products primarily answer whether a configured target
  responded.
- Pulse must answer whether a known service is being delivered, which
  infrastructure and dependencies provide it, from where the result was
  observed, and why the current conclusion is trustworthy.

Pulse should not compete on the number of protocol names in an add-monitor
dialog. Protocol support is justified when it strengthens a service-delivery
journey already represented in Pulse.

The defining workflow is:

1. Pulse discovers or already knows the resource delivering a service.
2. Pulse proposes a bounded verification based on that resource and service.
3. The operator confirms the expected behavior and observation location.
4. Pulse records active verification as evidence on the canonical resource.
5. Failures enter the existing incident and attention lifecycle.
6. Pulse correlates the failure with runtime, platform, dependency,
   maintenance, and multi-location evidence.
7. The resource timeline and Patrol explain the sequence without inventing
   evidence or requiring the operator to correlate separate tools manually.

## Current Baseline

Pulse already has the foundations this plan must extend rather than replace:

- Availability targets support ICMP, TCP, HTTP, HTTPS, and UDP probes,
  configurable intervals, timeouts, failure thresholds, HTTPS certificate
  posture, and local or assigned-agent execution.
- Every configured availability check remains a source-owned
  `network-endpoint` resource. Explicit or unambiguous correlation may attach
  the check as an additive facet to another canonical resource without moving
  or duplicating its identity.
- Service discovery can propose HTTP, HTTPS, or TCP verification for known web
  services, databases, brokers, and caches.
- Availability failures and certificate failures already project canonical
  resource incidents into the normal alert lifecycle.
- Canonical resources already carry parent, child, and learned relationships,
  maintenance intent, metrics, incidents, timelines, and evidence envelopes.
- The current availability poller retains only the latest result. The approved
  history contract therefore correctly makes categorical observation history
  and coverage semantics the first delivery dependency.
- On 2026-08-29, allowlisted aggregate telemetry showed 339 of 6,578 active
  persistent installs with 2,449 configured availability targets. Seventy
  installs had at least ten targets. The existing workflow has meaningful
  adoption before this expansion begins.

The present gap is not basic reachability. Pulse does not yet preserve honest
availability history, verify application-level response contracts, represent
one service's combined delivery evidence, synthesize infrastructure-aware
failure explanations, or present those conclusions as one service journey.

## Canonical Product Model

### 1. Resources Remain the Identity

Service assurance is a capability projected onto canonical Pulse resources,
not a second inventory of loosely related monitors.

- A configured verification keeps its source-owned availability target ID and
  history.
- A host, VM, container, application, platform service, or explicitly modeled
  endpoint remains the thing the operator investigates.
- Attached checks appear as evidence and relationships on that resource.
- Multiple verifications may contribute to one resource without merging their
  histories or hiding their individual configuration.
- A new general-purpose `service` resource type is not a prerequisite. Add one
  only if real service identity cannot be represented honestly through the
  existing resource registry, discovered application resources, and typed
  relationships.

### 2. Assurance Is Composed Evidence

One delivered service may have several evidence layers:

1. **Runtime evidence**: the process, container, VM, or appliance reports as
   running.
2. **Platform evidence**: the host, cluster, storage, and management connection
   remain healthy.
3. **Network evidence**: the address and port are reachable from a named
   observation location.
4. **Application evidence**: the response satisfies an operator-approved
   status, content, or structured-data contract.
5. **Dependency evidence**: known proxy, DNS, database, storage, or parent
   resources are healthy or failing.
6. **Freshness evidence**: enough recent observations exist to support the
   conclusion.

The UI must expose the contributing evidence rather than hide it behind an
opaque score. A concise derived state may summarize it, but unknown,
indeterminate, stale, and insufficient-coverage states remain distinct from
healthy and failed.

### 3. Correlation Is Deterministic Before It Is Explanatory

Pulse must derive bounded facts before Patrol narrates them:

- a failed parent or platform connection may explain dependent failures;
- a healthy host plus a failed application contract narrows the failure to the
  service, listener, proxy, or dependency layer;
- disagreement between observation locations indicates a path or site problem
  rather than a universally failed service;
- active maintenance suppresses notification intent through the existing
  policy path while preserving observation history;
- missing evidence reduces confidence and coverage rather than authoring a
  healthy result.

Patrol may summarize these facts, gather adjacent evidence, and recommend the
next investigation. It must not be the source of the service state or the
correlation itself.

### 4. Configuration and Operations Stay Separate

The availability inventory remains the bulk configuration and audit surface.
The normal operating experience belongs on resources, timelines, incidents,
and fleet views:

- **Discover**: Pulse found a service and proposes how to verify it.
- **Resource**: show how delivery is currently being verified.
- **Fleet**: show which services and sites need attention now.
- **Incident**: show the likely failing layer and affected dependants.
- **Timeline**: show what changed before and during the failure.

This avoids making a configuration table the product's main operational
surface.

## Primary Customer Journeys

### Discovered Service to Verified Service

Pulse discovers an application or listening service on a known resource,
proposes an appropriate verification, explains the inferred endpoint and
observation location, and lets the operator approve or edit it. The resulting
verification is attached to the resource and begins producing evidence without
requiring the operator to build a parallel naming hierarchy.

### Failure Isolation

An application verification fails. Pulse shows whether the container or VM is
running, whether its host and storage are healthy, whether the port remains
reachable, whether the response contract failed, and whether other observation
locations reproduce the problem. The incident groups downstream symptoms
beneath the strongest supported cause while keeping each observation visible.

### Fleet Attention

An operator with tens or hundreds of checks sees current attention order,
24-hour state shape, reachable latency, coverage, location, and affected
resource without opening every row. Selecting an item enters the existing
resource investigation path rather than a separate monitor detail universe.

### Expected Activity

An operator declares that a backup, replication, scheduled script, renewal, or
remote site should report within a bounded interval. A missed report becomes
an evidence gap associated with the known job, machine, repository, or site.
Pulse combines that absence with the relevant infrastructure state instead of
exposing only an anonymous heartbeat URL.

## Execution Slices

The slices are ordered so each one is releasable, improves the existing Pulse
journey, and establishes a dependency required by the next slice.

### Slice A: Honest Availability History and Fleet Shape

Owner contract: `docs/AVAILABILITY_HISTORY_CONTRACT.md`

Deliver:

- source-owned categorical observation history for local and assigned-agent
  probes;
- explicit reachable, unreachable, indeterminate, and unknown durations;
- coverage-aware availability and reachable-only latency summaries;
- configuration revision boundaries, retention, and deletion behavior;
- one bounded batch read path with no per-target query loop;
- a compact fleet mode over the existing availability resources, filters, and
  resource detail path.

Exit conditions:

- all release gates in the history contract pass;
- the fleet surface remains usable at 50 targets on laptop and phone widths;
- the UI never presents missing observations as healthy time or labels the
  result as an SLA.

Implementation record (2026-08-30): Slice A is delivered through the
monitoring-owned categorical history store and rollups, server-authored
configuration revisions and remote receipt timeline, the bounded
`/api/availability-history` batch contract, and the URL-owned Availability
fleet presentation. Its release proofs live in the owner contract. Slices D
through H remain ordered future work; acceptance of this product lane does not
imply that deferred breadth is already delivered.

### Slice B: Application Response Contracts

Deliver a bounded HTTP and HTTPS verification contract attached to a known
service or endpoint:

- accepted response status or status range;
- HEAD, GET, and bounded POST where required by a real service journey;
- operator-supplied headers;
- Basic and bearer authentication with secret-safe storage and presentation;
- bounded text assertion;
- bounded JSON field or query assertion;
- test-before-save with a redacted result;
- revisioned history when execution-defining fields change.

The normal setup question is "What proves this service is working?" rather
than "Which monitor type do you want?"

Exit conditions:

- reachability and application correctness are separately visible;
- secrets, request bodies, response bodies, and target credentials never enter
  history, evidence summaries, telemetry, logs, or Patrol prompts;
- failure output is bounded and actionable without persisting arbitrary remote
  content;
- current reachability-only configurations migrate without semantic change.

Implementation record (2026-08-30): Slice B is delivered through one bounded,
explicit HTTP/S contract shared by local and assigned-agent execution. The
saved target owns method, accepted status range, optional POST body, headers,
Basic or bearer authentication, text assertion, and structured JSON field
equality. Write-only values are encrypted at rest, omitted from API and
evidence payloads, and reused only while the endpoint origin is unchanged;
changing scheme, host, or effective port requires explicit credential
re-entry. Current status and unified-resource facets preserve transport
reachability separately from typed application correctness while the overall
result remains the alert and history outcome. Legacy targets without an
explicit contract retain their previous HEAD-with-bounded-GET-fallback
semantics. Slices D through H remain ordered future work.

### Slice C: Discovery-Led Assurance Onboarding

Deliver:

- reviewable verification proposals from existing service discovery;
- one-click creation from the resource surface with an explicit preview of
  endpoint, method, expected behavior, interval, and observation location;
- bulk review for operators onboarding a machine or site with several
  discovered services;
- duplicate detection against existing attached and standalone checks;
- clear provenance for inferred versus operator-entered fields;
- no silent creation of active network checks.

Exit conditions:

- the operator can move from a discovered service to a trustworthy attached
  verification without visiting the generic target form;
- rejected suggestions stay dismissed until the discovery evidence materially
  changes;
- ambiguous identity or endpoint evidence fails to a review state rather than
  attaching to a guessed resource.

Implementation record (2026-08-30): Slice C is delivered through the existing
Discovery suggestion and availability-target contracts. Each bounded
HTTP/HTTPS/TCP proposal carries a normalized evidence fingerprint; dismissals
persist only for that exact evidence and stale disposition writes fail with a
conflict. The canonical resource drawer previews inferred endpoint and
application behavior separately from operator-controlled name, cadence, and
observation location, offers an unsaved test, detects equivalent attached or
standalone endpoints, and creates one explicitly enabled availability target
with the drawer's canonical resource ID only after the operator chooses the
activation action. A machine-scoped queue supports bulk review and
evidence-bound dismiss/restore without bulk activation or guessed resource
attachment. Slices D through H remain ordered future work.

### Slice D: Multi-Location Delivery Evidence

Evolve assigned execution from a remote-check option into named observation
locations while preserving source-owned observation identity.

Deliver:

- operator-visible observation locations derived from local Pulse and eligible
  connected agents;
- one logical service verification observed from one or more selected
  locations without duplicating the service in fleet views;
- per-location state, latency, freshness, and coverage;
- an aggregate conclusion that preserves disagreement instead of flattening
  it;
- explicit agent capability, assignment, lapse, and reassignment behavior;
- bounded comparison between internal and remote paths.

Exit conditions:

- a single-location path failure cannot author a universal service outage;
- stale or disconnected observation locations become unknown evidence;
- agent clock skew, retries, and duplicates cannot rewrite the authoritative
  timeline;
- location names and customer-identifying network details obey the existing
  privacy and redaction policy.

### Slice E: Infrastructure-Aware Incident Synthesis

Deliver deterministic incident grouping and causal narrowing over canonical
relationships:

- group child delivery failures beneath a supported parent, platform, or
  shared-dependency failure;
- distinguish runtime, network path, application response, certificate,
  dependency, and evidence-coverage failure classes;
- propagate maintenance and monitoring intent through existing resource policy
  rather than adding monitor-local suppression state;
- show affected dependants and the observations that support the grouping;
- allow the operator to expand every grouped symptom and challenge the
  inferred cause.

Exit conditions:

- grouping never hides a contradictory healthy or failing observation;
- one infrastructure failure does not produce an unbounded notification storm;
- unsupported causality is labeled as an observation set, not a root cause;
- recovery order and partial recovery remain visible.

### Slice F: Unified Timeline and Patrol Investigation

Deliver:

- service state and reachable-latency history in the canonical resource
  timeline;
- overlays or adjacent events for restarts, host pressure, connection loss,
  maintenance, certificate posture, configuration revision, and relevant
  dependency changes;
- Patrol investigation context built from the same canonical observations and
  deterministic incident synthesis;
- explanations that cite observation time, location, affected resource, and
  evidence freshness;
- no dependency on an external model for ordinary service state, grouping, or
  timeline use.

Exit conditions:

- an operator can reconstruct the failure sequence without manually comparing
  separate pages;
- Patrol distinguishes observed facts, deterministic inference, and suggested
  next steps;
- missing provider access or model failure does not reduce monitoring or
  incident correctness.

### Slice G: Expected Activity Assurance

Deliver expected-activity definitions for known operational work:

- backup and replication jobs;
- scheduled scripts and maintenance tasks;
- certificate renewal or other bounded lifecycle activity;
- agent, remote site, or integration reporting expectations.

A generic authenticated push endpoint may be an execution mechanism, but the
product object is the expected activity and its associated resource.

Exit conditions:

- replay, early, duplicate, late, and missing reports have explicit semantics;
- credentials are scoped to one expected activity and can be rotated or
  revoked independently;
- missed activity is correlated with machine, repository, network, and
  maintenance state where available;
- Pulse does not claim a job succeeded merely because a heartbeat arrived
  unless the configured contract explicitly defines that evidence as success.

### Slice H: Customer and Status Communication

Public or authenticated status presentation is demand-gated and follows the
assurance model rather than preceding it.

Potential deliverables after validation:

- selected-service status views backed by the same coverage-aware history;
- customer or site boundaries for MSP operation;
- operator-authored incident communication distinct from inferred health;
- branded or access-controlled views in the appropriate commercial repo;
- explicit separation between observed availability, internal operational
  health, communicated status, and contractual SLA reporting.

This slice does not turn status-page breadth into a prerequisite for the core
service-assurance journey.

## Delivery Phases

### Phase 1: Credible Replacement for Core Service Checks

Slices A through C form the first product milestone. The operator gets honest
history, application-level verification, and a discovery-led setup path tied
to known resources.

Validation question:

> Can an existing Pulse operator stop maintaining a second availability tool
> for their ordinary web services without making Pulse feel like that tool was
> copied into the sidebar?

### Phase 2: Pulse-Specific Differentiation

Slices D through F form the differentiated milestone. Pulse uses distributed
agents, resource relationships, incidents, timelines, and Patrol to isolate
the failing infrastructure layer and explain the impact.

Validation question:

> Does Pulse reduce the time and manual correlation needed to understand a
> service failure, not merely detect it?

### Phase 3: Operational and Commercial Expansion

Slices G and H add expected activity, customer boundaries, and optional status
communication after the core model is proven.

Validation question:

> Does the assurance model support recurring operational work and MSP customer
> communication without creating a parallel product architecture?

## Packaging Principles

- Core local verification remains part of Pulse's monitoring foundation.
- Availability-history access follows the existing `max_history_days`
  entitlement rather than introducing a separate retention table.
- Assigned external execution continues to use the existing `external_probe`
  entitlement until a broader commercial contract is deliberately approved.
- Data correctness, evidence freshness, incident truth, and recovery state are
  never degraded to create an upgrade prompt.
- Commercial value should come from longer history, multi-location operation,
  fleet and customer boundaries, governed collaboration, and operational
  convenience rather than an arbitrary monitored-service tax.
- MSP pricing, customer tenancy, sales claims, and checkout work belong in
  `pulse-pro` or the appropriate private runtime after the open-source contract
  is stable. They must not fork the underlying assurance semantics.

## Success Measures

### Product Adoption

- active installs with at least one availability check;
- installs with ten or more checks;
- proportion of checks explicitly or confidently attached to a canonical
  resource;
- time from service discovery to first accepted verification;
- proposal acceptance, edit, dismissal, and duplicate-prevention rates;
- use of fleet mode and service-level history by estates with many checks.

### Operational Outcome

- incidents where Pulse identifies the failing layer from deterministic
  evidence;
- reduction in raw symptom notifications after relationship-aware grouping;
- multi-location disagreements correctly classified as partial or path-local;
- time from first failed observation to an operator reaching the affected
  resource and supporting timeline;
- proportion of conclusions with sufficient observation coverage;
- operator corrections of inferred relationships or causes.

### Market Validation

- recruit 10 to 15 operators with roughly 20 to 50 service checks for design
  and qualification use;
- obtain at least five sustained Phase 1 evaluations in real infrastructure
  estates;
- obtain at least three operators or providers who retire a meaningful portion
  of a separate uptime workflow or pay specifically for the fleet assurance
  outcome;
- treat positive commentary without sustained use, replacement behavior, or
  paid commitment as interest rather than validation.

## Non-Goals

- matching another product's monitor-type count;
- introducing a separate service or monitor inventory when canonical resources
  and relationships can represent the truth;
- public status pages before trustworthy history and service evidence exist;
- labeling observed availability as an SLA;
- model-generated health or root cause without deterministic evidence;
- browser scripting, game-server protocols, database-specific query checks,
  or broad synthetic transaction tooling without direct demand;
- auto-enabling network checks from discovery without operator review;
- storing arbitrary response bodies, secrets, request bodies, or customer
  identifiers in history or telemetry;
- creating a monitor-count entitlement for the open-source runtime;
- allowing MSP presentation needs to fork core state or incident semantics.

## Cross-Repo Boundaries

- `pulse` owns probe execution, observation history, canonical resources,
  relationships, incidents, resource and fleet presentation, and local Patrol
  context.
- `pulse-enterprise` may own advanced organization policy, customer-separated
  operation, or enterprise-only administration while consuming the canonical
  Pulse contracts.
- `pulse-mobile` may consume assurance incidents, location disagreement, and
  approval or acknowledgement workflows without defining separate health
  semantics.
- `pulse-pro` owns commercial validation, packaging, MSP sales and account
  flows, hosted entitlement state, and public positioning.

## Governance and Completion

This proposal should become an accepted lane only after the project owner
approves its priority relative to the other available candidate lanes. Lane
acceptance must name the first execution slice, its owning subsystems, and its
proof commands rather than accepting the whole roadmap as one unbounded unit.

Each implementation slice must:

1. extend canonical resources and existing target identity rather than create a
   shadow model;
2. update the owning subsystem contracts when a runtime or API truth changes;
3. preserve unknown and indeterminate evidence semantics end to end;
4. prove privacy, retention, redaction, and entitlement behavior;
5. include desktop and narrow rendered-browser verification for user-visible
   work;
6. record any deferred protocol or commercial breadth as a typed follow-up,
   not an implied part of the completed slice;
7. remove newly superseded internal paths rather than retaining two primary
   implementations.
