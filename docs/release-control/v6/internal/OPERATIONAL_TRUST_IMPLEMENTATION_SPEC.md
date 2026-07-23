# Pulse v6 Operational Trust Implementation Spec

Last updated: 2026-07-19
Status: ACTIVE
Primary governance surface:
- `status.json.candidate_lanes.protection-posture-attention-queue`

Resolved related governed surface:
- `internal/records/operational-trust-availability-resource-facet-2026-07-19.md`

## Intent

This specification turns Pulse's product direction into one executable
contract.

Pulse must become the trusted operations layer for mixed private
infrastructure. It must tell an operator:

1. what needs attention
2. which resource is affected
3. why Pulse believes it
4. what the likely impact is
5. what protection or recovery evidence exists
6. what safe next action is available
7. whether that action worked

The implementation loop is:

`observe -> identify -> detect -> relate -> explain -> act -> verify`

This is a monitor-first product. Platform pages and the dashboard remain the
normal observation surfaces. Patrol is the cross-platform attention queue and
explain-act-verify workbench. Assistant is contextual help for selected
resources, evidence, findings, and actions. It is not the primary navigation
model.

The work is complete only when the runtime, APIs, product surfaces, migrations,
tests, live-browser journeys, and governance evidence satisfy every applicable
criterion in this document. A narrow detector, a polished card, or a passing
unit test is not completion.

## Canonical Root-Fix Decision

The canonical fix is a shared operational-trust model, not UI composition over
unrelated alert, notification, recovery, and Patrol payloads.

The implementation must establish typed shared contracts for:

1. resource identity and relationships
2. evidence provenance, freshness, completeness, and confidence
3. operational lifecycle state
4. protection posture
5. attention items
6. governed actions and verification

Platform adapters may emit provider-specific evidence, but they must not own
parallel lifecycle, posture, or action semantics. Frontend code may present the
shared contracts, but it must not infer canonical trust state from labels,
message text, or loose metadata maps.

## Product Jobs

The least-expert plausible operator should be able to complete these jobs in
their own vocabulary:

1. "Show me what needs my attention."
2. "Tell me what is affected and how serious it is."
3. "Tell me whether the data is current and where it came from."
4. "Tell me whether this workload has a usable recent backup."
5. "Give me the safest next step."
6. "Let me approve or run a supported action."
7. "Show me whether the problem is actually fixed."
8. "When everything is healthy, say so plainly and get out of the way."

The default experience must not require the operator to understand internal
terms such as detector, evidence envelope, rollup, canonical identity,
projection, or verification intent.

## Governing Product Rules

### 1. Evidence before assertion

Every user-visible operational assertion must link to typed evidence. Evidence
must include source, observation time, ingestion time, subject identity, and
completeness. When freshness or completeness is insufficient, the product says
`unknown` or `stale`; it does not silently downgrade those states to `healthy`.

### 2. One lifecycle truth

The active count, navigation badge, alert list, Patrol queue, notification
history, and resource detail must derive from the same canonical lifecycle
truth. They may show different filtered views, but they must not disagree about
whether the same operational item is open, acknowledged, suppressed, stale, or
resolved.

### 3. Identity before correlation

Pulse must resolve observations to canonical resources before it correlates
impact, recovery evidence, availability, or actions. Explicit links win.
Provider-scoped stable identities are next. Unambiguous secondary identity is
allowed only with auditable match evidence. Ambiguous matches remain separate
and visible as unresolved.

### 4. Protection evidence, not a backup engine

Pulse reports protection and recovery evidence from connected providers. It
does not claim to execute, own, or guarantee backup and restore workflows that
belong to PBS, Proxmox, TrueNAS, Kubernetes tooling, or another native control
plane.

### 5. Monitor first, attention second

Platform surfaces answer "what is happening here?" Patrol answers "what needs
me across everything?" The same attention item may be entered from either
surface without becoming two separate records.

### 6. Actions are declared capabilities

An action exists only when the target resource declares the capability and the
policy engine can determine its eligibility. Unknown, stale, incomplete, or
ambiguous evidence must fail closed for mutating actions.

### 7. Verification is part of the action

An execution result is not enough. Every mutating action must define a
postcondition and a bounded verification policy. The UI distinguishes
`executed` from `verified`.

### 8. Calm is a valid state

When nothing needs attention, Patrol shows a short calm-day state with the last
successful evaluation time and any stale or disconnected coverage caveat.
It must not invent proof strips, scores, or decorative activity to make an
empty queue look busy.

## Explicit Non-Goals

This specification does not authorize:

1. a general backup scheduler, retention manager, or restore engine
2. broad autonomous remediation
3. arbitrary shell or vendor API execution
4. a chat-first home screen
5. replacement of native vendor control planes
6. a universal recoverability score that hides missing evidence
7. automatic merging of ambiguous resources
8. a second alert engine inside Patrol
9. replacing a configured availability check's source-owned identity with a
   correlated target or creating more than one check row for one saved target
10. platform-specific lifecycle states in primary APIs
11. hiding stale, partial-permission, or collection-error states
12. MSP multi-tenancy, fleet-wide delegation, or policy inheritance beyond
    existing governed enterprise boundaries

## Canonical Domain Model

The implementation may refine names to match existing packages, but it must
preserve the semantics and typed boundaries below.

### 1. Evidence envelope

Every detector input and every trust-bearing output must be traceable to an
evidence envelope.

```text
EvidenceEnvelope
  id                    stable within its source and observation
  source                provider and collector identity
  subject               canonical resource id or unresolved provider ref
  observedAt            time represented by the source
  ingestedAt            time Pulse accepted the observation
  validUntil            optional source-specific freshness boundary
  completeness          complete | partial | unavailable
  confidence            confirmed | inferred | unknown
  reason                typed reason for partial, unavailable, or inferred
  permissions           sufficient | partial | denied | unknown
  payloadRef            bounded reference to provider detail
  correlation           optional auditable identity match record
```

Required behavior:

1. freshness is evaluated from `observedAt` and source policy, never only from
   frontend wall-clock assumptions
2. partial permission is different from no data
3. provider failure is different from an observed healthy value
4. inferred identity records the matched fields and rule
5. raw provider payloads stay bounded and policy-shaped
6. evidence needed for an open lifecycle item remains available for the
   lifecycle retention window

### 2. Resource identity and relationships

The unified resource registry remains the owner of canonical resource identity.
It must support auditable relationships required by operational reasoning:

```text
ResourceRelationship
  id
  fromResourceId
  toResourceId
  kind                  runs-on | hosted-by | depends-on | stores-on |
                        protected-by | checks | member-of | exposes
  source
  observedAt
  confidence
  evidenceId
```

Identity resolution order:

1. explicit canonical resource id
2. existing canonical provider link
3. stable provider-scoped identity
4. unambiguous normalized secondary identity
5. unresolved provider reference

Secondary identity correlation must:

1. normalize case, address form, and hostname form consistently
2. reject zero-match and multi-match results
3. record the rule and matching values
4. avoid per-row network or database fetches
5. preserve provider scope so bare vendor ids are never workspace-global

### 3. Operational lifecycle

The existing canonical alert lifecycle is the starting point. The completed
contract must represent:

```text
OperationalState
  observing
  open
  acknowledged
  suppressed
  resolving
  resolved
  stale
  unknown
```

These are operational states, not severity levels.

Every lifecycle record must have:

```text
OperationalRecord
  id
  canonicalSpecId
  subjectResourceId
  state
  severity
  firstObservedAt
  lastObservedAt
  stateChangedAt
  resolvedAt
  acknowledgement
  suppression
  evidenceIds
  causeKey
  relatedResourceIds
  impactSummary
  recommendedNextStep
```

Required transition rules:

1. `observing -> open` only through a canonical detector decision
2. acknowledgement never means resolution
3. suppression never rewrites the underlying detector state
4. an absent observation cannot resolve an item when collection is stale,
   partial, failed, or permission-denied
5. resolution records the decisive recovery evidence
6. recurrence after resolution either reopens the same stable cause or creates
   a linked recurrence according to the detector's canonical identity policy
7. transitions are idempotent
8. concurrent reads and writes are race-safe
9. retention preserves a usable timeline after resolution

### 4. Notification linkage

Notification delivery is a consequence of lifecycle transitions, not a second
source of operational truth.

Each queued delivery and audit record must carry:

```text
NotificationLink
  notificationId
  operationalRecordId
  transitionId
  lifecycleState
  causeKey
  destinationId
  deliveryState
  attemptedAt
  completedAt
```

Required behavior:

1. retries preserve one stable event id
2. grouped notifications preserve every included operational and transition id
3. resolve notifications link to the resolving transition
4. cancelled, failed, and dead-letter delivery remain inspectable
5. navigation counts never derive from queue status
6. disabling notification delivery does not disable detection or erase open
   operational records
7. configuration surfaces clearly distinguish detection, in-product alerts,
   and external notification delivery

### 5. Protection posture

Protection posture is a typed, per-subject read model derived from recovery
points and provider job evidence.

```text
ProtectionPosture
  subjectResourceId
  state                 protected | attention | unprotected | unknown
  lastAttemptAt
  lastSuccessfulPointAt
  lastVerifiedAt
  freshness             current | stale | unknown
  verification          verified | unverified | stale | unknown
  coverage              complete | partial | none | unknown
  providerStates[]
  repositoryResourceIds[]
  evidenceIds[]
  explanation
  evaluatedAt
```

`providerStates[]` must preserve provider-specific uncertainty:

```text
ProtectionProviderState
  provider
  source
  scope
  jobState
  historyCompleteness
  permissions
  lastAttemptAt
  lastSuccessAt
  lastVerifiedAt
  evidenceIds[]
```

Derivation rules:

1. `protected` requires a successful recovery point inside the configured
   freshness window and no known provider failure that invalidates the claim
2. `attention` means evidence exists but is stale, failing, unverified where
   verification is expected, or incomplete in a way the operator can address
3. `unprotected` requires sufficient evidence to affirm that no qualifying
   protection exists
4. `unknown` is mandatory when identity, permission, history, or collection
   completeness cannot support a stronger claim
5. snapshots and backups preserve their canonical distinction
6. snapshot presence alone never implies independent recovery
7. provider job success without a subject-linked recovery point may be useful
   evidence, but it must not silently become subject-level protection
8. the posture explanation names the strongest evidence and the limiting
   uncertainty in plain language

The existing recovery-point and protection-rollup models must be migrated or
adapted into this contract without breaking supported clients.

### 6. Attention item

Patrol consumes a shared attention read model. It does not inspect arbitrary
metadata to invent a queue.

```text
AttentionItem
  id
  operationalRecordId
  subjectResourceId
  title
  plainLanguageSummary
  severity
  state
  firstObservedAt
  lastObservedAt
  evidenceFreshness
  evidenceCompleteness
  impact
  protectionPosture
  relatedResources[]
  recommendedNextStep
  availableActions[]
  verificationState
```

Queue rules:

1. default ordering is actionable severity, blast radius, protection concern,
   evidence freshness, then age
2. stale or unknown evidence is visible but never presented as a confirmed
   failure
3. duplicate symptoms sharing a stable cause may group under one item, while
   every member remains inspectable
4. acknowledged and suppressed items are filterable and retain their true
   lifecycle state
5. resolved items leave the active queue and remain in recent history
6. the same item id deep-links from navigation, platform rows, resource
   detail, notifications, and Patrol

### 7. Availability facet

Availability is evidence about a resource, not a parallel inventory taxonomy.

Required ingest behavior:

1. attach by explicit canonical resource link first
2. otherwise attach by one unambiguous normalized address or hostname match
3. retain a standalone `network-endpoint` only for genuinely unowned targets
4. preserve ambiguous targets as unresolved rather than guessing
5. emit a `checks` relationship and evidence envelope
6. use the unified-resource hot path with no per-row fetch

Required presentation:

1. show a compact Availability facet on the owning platform resource row and
   resource detail
2. show protocol identity, target, latest result, latency when relevant,
   freshness, and last observation
3. route failures into the canonical lifecycle and attention model
4. avoid duplicating the target on a generic Machines page once attached

This portion remains linked to the separately governed
`availability-as-resource-facet` candidate and must be claimed in sequence
before its runtime mutation begins.

### 8. Governed action and verification

Available actions are declared by resource capability and evaluated against
policy, evidence, and current lifecycle state.

```text
ActionOffer
  actionId
  targetResourceId
  kind
  mode                  plan | dry-run | execute
  risk                  read-only | low | elevated
  approval              not-required | required | granted | denied
  eligibility           eligible | ineligible | unknown
  reasons[]
  evidenceIds[]
  expectedPostcondition
  verificationPolicy
```

```text
ActionRun
  runId
  actionId
  operationalRecordId
  actor
  requestedAt
  approvedAt
  startedAt
  completedAt
  executionState
  result
  auditRef
  verificationState     pending | verified | failed | inconclusive | timed-out
  verificationEvidenceIds[]
```

Required behavior:

1. planning and dry-run are preferred where the provider supports them
2. elevated actions require explicit approval
3. mutating actions fail closed on stale, incomplete, unknown, or ambiguous
   target evidence unless a narrower canonical policy explicitly allows them
4. action parameters are typed and bounded
5. idempotency keys prevent duplicate execution
6. execution records actor, target, parameters, policy result, provider result,
   and timestamps
7. verification uses fresh post-action evidence and a declared postcondition
8. verification failure or timeout keeps the attention item open
9. the first shipped mutating actions must be narrow, reversible where
   possible, and already supported by the canonical action framework

## Runtime Ownership

The intended ownership boundaries are:

1. `internal/unifiedresources`: canonical identity and relationships
2. `internal/alerts`: detector decisions and operational lifecycle
3. `internal/notifications`: transition-linked delivery and audit
4. `internal/recovery`: recovery points, provider evidence, and protection
   posture derivation
5. `internal/monitoring`: observation collection and availability evidence
6. `internal/ai` or the existing Patrol application owner: attention
   projection, explanation, and supported action proposals
7. `internal/api`: versioned transport and authorization only
8. `frontend-modern`: typed presentation and interaction only

If existing ownership differs, the implementation must update subsystem
contracts before moving logic. It must not create circular dependencies or
place canonical derivation in API handlers.

## API Contract

Exact route names may follow existing conventions, but the completed API must
provide these capabilities through versioned typed payloads.

### Operational records

1. list active records with filters and stable pagination
2. get one record with transition timeline and evidence references
3. acknowledge and unacknowledge
4. suppress with bounded scope and expiry
5. list recent resolved records
6. expose one canonical active-count summary used by all badges

### Evidence

1. get bounded evidence detail authorized for the current operator
2. show source, freshness, completeness, confidence, and reason
3. redact sensitive provider detail by policy
4. return typed unavailable responses when retained detail has expired

### Protection

1. get posture for one canonical resource
2. batch posture for bounded resource tables
3. list attention postures
4. drill into contributing recovery points and provider evidence
5. expose evaluation policy and timestamps

### Patrol

1. list active attention items
2. get item detail
3. list recent resolved history
4. expose calm-day evaluation and coverage state
5. deep-link to the owning resource and source platform

### Actions

1. list offers for an attention item or resource
2. plan or dry-run
3. request and record approval
4. execute with idempotency
5. retrieve execution and verification status
6. cancel only where the provider contract supports safe cancellation

### Compatibility rules

1. existing supported alert and recovery routes remain compatible during
   migration
2. compatibility adapters are read-side boundaries, not parallel truth
3. new typed fields are additive before deprecated fields are removed
4. deprecations include usage evidence, migration tests, and a governed removal
   decision
5. table APIs support bounded batch reads; the frontend must not issue a
   request per row

## Product Surface Contract

### 1. Navigation and dashboard

1. one active attention count comes from the canonical read model
2. label and tooltip distinguish attention items from notification delivery
3. the count never remains non-zero beside an empty active queue
4. the dashboard shows concise actionable summaries and links into Patrol or
   the affected resource
5. monitor pages remain the normal authenticated launch path

### 2. Platform resource rows

1. health, active attention, protection posture, and availability appear only
   when relevant to that resource
2. compact indicators show state and freshness without widening tables into
   consoles
3. row expansion or a drawer holds provider evidence and forensic detail
4. every indicator has a clear operator action or is demoted from the default
   row
5. canonical table alignment helpers remain the sole alignment owner

### 3. Resource detail

The default detail answers, in order:

1. what resource is this
2. what is its current state
3. what needs attention
4. what recently changed
5. what it depends on and what depends on it
6. what protection evidence exists
7. what safe actions are available

Metrics and provider detail remain available without displacing those answers.

### 4. Patrol active queue

The initial view contains:

1. a plain heading and accurate active count
2. concise filters for open, acknowledged, suppressed, stale/unknown, and
   recent resolved
3. an ordered list of attention items
4. resource identity, impact, evidence state, protection context, and next step
5. a clear calm-day state when the list is empty

It must not contain:

1. a generic object browser
2. vanity trust scores
3. a large Assistant prompt before the queue
4. stale findings presented as live
5. read-only replicas of native vendor consoles

### 5. Attention detail

The detail or drawer contains:

1. plain-language finding
2. affected and related resources
3. timeline from first observation through current state
4. evidence source, age, completeness, confidence, and limiting caveats
5. protection posture with provider drill-down
6. recommended next step
7. eligible actions, approval state, and risk
8. execution and verification results
9. links to the owning platform or native tool when Pulse cannot act

### 6. Assistant

Assistant receives selected typed context. It may:

1. explain evidence
2. summarize impact and relationships
3. describe a proposed action or verification result
4. help the operator navigate to supporting detail

Assistant may not:

1. become the source of lifecycle or protection truth
2. infer an action capability absent from the canonical resource
3. bypass approval or evidence eligibility
4. hide uncertainty

## Security, Privacy, and Trust

1. evidence access follows existing tenant, role, and sensitivity boundaries
2. secret values and full credential-bearing payloads are never stored as
   evidence
3. action authorization is checked at offer time and again at execution time
4. approvals bind actor, exact target, action kind, parameters, and expiry
5. evidence and action audit records use stable ids without leaking sensitive
   provider identifiers into public logs
6. API errors do not reveal unavailable resources across authorization
   boundaries
7. notification destination configuration remains protected
8. Assistant receives policy-shaped summaries and references, not unrestricted
   raw provider payloads

## Performance and Retention

1. active-count and Patrol list reads must not scan unbounded history
2. platform tables use bounded batch posture and availability data
3. no per-row fetches
4. lifecycle writes and notification enqueue remain idempotent under retry
5. relationship and evidence indexes support subject, time, source, and active
   state reads
6. retained evidence is sufficient to explain open records and recent
   resolution
7. high-cardinality recovery points remain outside unified-resource inventory
8. retention expiry preserves a typed summary and explicitly reports expired
   detail
9. load proofs cover realistic resource, alert, recovery-point, and queue
   cardinalities

## Telemetry and Product Health

The implementation must expose operational measurements for:

1. observation-to-open latency
2. open-to-notification-enqueue latency
3. notification delivery success, retries, and dead letters
4. stale or incomplete evidence counts by source
5. unresolved or ambiguous identity correlations
6. protection posture distribution and evaluation failures
7. attention queue age and acknowledgement time
8. action offer, approval, execution, and verification outcomes
9. active-count/read-model mismatch detection
10. calm-day evaluation age

Telemetry must not use high-cardinality raw resource ids as metric labels.

## Migration and Rollout

### Phase 0: contract and contradiction containment

1. finalize typed contracts and subsystem ownership
2. inventory every active-count and alert-enabled interpretation
3. make detection, in-product visibility, and external delivery separate
4. add read-model consistency assertions
5. preserve supported payload compatibility

### Phase 1: lifecycle and evidence foundation

1. land evidence envelopes
2. make lifecycle transition ids durable and idempotent
3. link notification audit to transitions
4. surface freshness, completeness, and source
5. migrate loose trust-bearing metadata into typed fields

### Phase 2: protection posture

1. build canonical posture derivation over recovery points and provider job
   evidence
2. add bounded subject and table APIs
3. distinguish protected, attention, unprotected, and unknown
4. prove PBS first without baking PBS semantics into the shared model
5. add provider adapters only with explicit evidence-quality mappings

### Phase 3: Patrol attention workbench

1. project lifecycle records into typed attention items
2. unify active counts and deep links
3. deliver active, acknowledged, suppressed, stale/unknown, resolved, and calm
   states
4. show evidence, impact, relationships, and protection in detail
5. demote Assistant to selected context

### Phase 4: availability attachment

1. claim the `availability-as-resource-facet` governed surface
2. implement explicit and unambiguous correlation
3. attach the facet and relationship
4. remove duplicate primary presentation for attached checks
5. route failures through canonical lifecycle and Patrol

### Phase 5: governed actions and verification

1. select the smallest useful existing declared capabilities
2. implement offer eligibility and evidence gating
3. prove approval, idempotent execution, audit, and postcondition verification
4. ship no action whose failed verification cannot be represented honestly

### Phase 6: hardening and rollout completion

1. compatibility and migration proofs
2. concurrency, failure, retention, and load proofs
3. accessibility and responsive proofs
4. production-observed telemetry where required by governance
5. documentation and upgrade notes
6. remove superseded compatibility paths only after governed evidence

Each phase must leave one coherent runtime path. Feature flags may control
exposure, but they must not create two writable sources of truth.

## Required Proof Matrix

### Unit and contract proof

1. every lifecycle transition, including recurrence and stale collection
2. evidence freshness, partial permission, provider failure, and expiry
3. identity resolution, ambiguous rejection, and provider scoping
4. protection posture truth table
5. notification grouping, retry, resolve linkage, cancellation, and dead letter
6. attention projection and stable ordering
7. action eligibility, approval binding, idempotency, and verification
8. JSON compatibility for supported clients

### Integration proof

1. observation opens one operational record and one attention item
2. repeated observation does not duplicate either
3. acknowledgement is reflected everywhere without resolving
4. suppressed items leave the default queue but remain inspectable
5. source outage produces stale or unknown, not false resolution
6. recovery evidence changes protection posture and the related attention item
7. notification audit links to the exact transition
8. an action executes once and only closes after verified recovery evidence
9. an attached availability target keeps exactly one source-owned check
   resource and one additive facet projection without duplicate incidents
10. authorization and tenant boundaries hold across evidence and actions

### Browser proof

Playwright inspection is mandatory for:

1. monitor-first launch
2. platform row with healthy, attention, stale, unknown, protected,
   unprotected, and unavailable-evidence states as applicable
3. Patrol with active work
4. Patrol calm day
5. acknowledged and suppressed filters
6. stale collection without false reassurance
7. evidence and protection drill-down
8. action approval, execution, verification success, and verification failure
9. navigation count consistency
10. keyboard navigation, focus restoration, screen-reader names, narrow
    viewport, and reduced-motion behavior

Proof must exercise expansions, drawers, empty states, and deepest supported
detail. Source inspection and top-level screenshots are insufficient.

### Failure proof

1. collector disconnected
2. provider partial permission
3. provider rate limit or timeout
4. notification destination failure and dead letter
5. ambiguous identity
6. missing recovery history
7. stale successful backup
8. action provider timeout
9. successful provider response with failed postcondition
10. restart during lifecycle transition or queued delivery

### Performance proof

1. bounded active-count latency
2. bounded Patrol pagination
3. bounded table batch enrichment
4. no per-row network pattern
5. stable memory and database behavior under retained evidence and recovery
   history

## User-Lens Acceptance

Before each customer-facing phase is accepted:

1. state the operator's job in their words
2. exercise the live surface to deepest state
3. count navigation depth and visible elements before the answer
4. identify the action supported by each default-visible element
5. replace or explain unfamiliar vocabulary
6. compare the design with relevant GitHub issue evidence
7. classify each element as keep, demote to expansion, or cut
8. record whether the surface is `prototype` or `product`

No phase closes while its main surface remains prototype-grade.

## Completion Criteria

This specification is fully implemented only when all of the following are
true:

1. every required canonical domain contract exists in code and subsystem
   governance
2. alert, navigation, Patrol, resource, and notification views share one
   lifecycle truth
3. stale, partial, denied, failed, ambiguous, and unknown evidence cannot
   masquerade as healthy or resolved
4. protection posture is provider-aware, subject-linked, explainable, and
   available without per-row fetches
5. Patrol is a functional attention queue with active-work and calm-day product
   states
6. availability attaches to known resources and participates in the same
   evidence and lifecycle model
7. at least one useful narrow mutating action completes the
   plan/approve/execute/verify loop through the canonical action framework
8. every required proof in this document passes or is explicitly marked not
   applicable with a canonical justification
9. live Playwright inspection passes the product quality and user-lens gates
10. supported API and persisted-data migrations are proven
11. subsystem contracts, status evidence, readiness assertions, release gates,
    upgrade documentation, and operator documentation reflect runtime truth
12. no temporary compatibility path remains in the primary runtime
13. all implementation slices are committed with scoped proof and no unrelated
    workspace changes
14. the governing candidate lanes and coverage gaps are resolved or normalized
    to an explicit remaining decision owned outside this specification

Passing one phase does not satisfy the overall goal. The goal closes only after
the full completion criteria pass.

## Governance Mapping

Primary lane and subsystem ownership:

1. L6: alerts, lifecycle, notifications, API coherence, Patrol projection
2. L8: platform and resource-facing product surfaces
3. L13: monitoring, recovery evidence, unified resources, availability
4. L20: action capability, approval, audit, and verification
5. L22: product trust, Assistant boundaries, policy-shaped context
6. L23: proactive operations and Patrol workbench

Primary candidate:

- `protection-posture-attention-queue`

Resolved sequential related candidate:

- `availability-as-resource-facet`, closed by
  `internal/records/operational-trust-availability-resource-facet-2026-07-19.md`

The primary candidate's subsystem mapping must include the canonical owners
that this implementation touches, including alerts, notifications, storage and
recovery, Patrol intelligence, monitoring, unified resources, API contracts,
frontend primitives, action governance, and policy-shaped Assistant context.

## Direction Captured

This specification records a `release_gate`:

> Pulse v6 is not operationally trustworthy until every customer-facing active
> state, evidence explanation, protection assertion, notification consequence,
> and governed action resolves through one contradiction-free canonical
> lifecycle and evidence model.

It also records a `readiness_assertion`:

> A resource may be called protected, healthy, resolved, or action-eligible
> only when current, sufficiently complete, subject-linked evidence supports
> that assertion; otherwise Pulse must display the limiting stale, partial,
> ambiguous, failed, or unknown state.
