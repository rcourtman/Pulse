# Operational Trust: Hardening and Rollout Completion

Date: 2026-07-19
Specification:
`docs/release-control/v6/internal/OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`
Phase: 6, hardening and rollout completion
Candidate: `protection-posture-attention-queue`
Verdict: accepted as a product-grade lane expansion

## Outcome

The six-phase Operational Trust implementation is complete. Pulse now has one
contradiction-resistant path from observation through evidence, operational
lifecycle, protection posture, attached availability, Patrol attention,
notification delivery, governed action, and detector-owned recovery.

The primary runtime has one writable owner for every trust-bearing domain:

1. alerts own operational lifecycle transitions;
2. operational evidence envelopes own freshness, completeness, permissions,
   confidence, source, subject, and correlation;
3. recovery owns provider-aware protection posture;
4. unified resources own canonical identity and relationships;
5. notifications own delivery state linked to an exact lifecycle transition;
6. Patrol owns the bounded attention projection, not lifecycle truth;
7. the action lifecycle owns plan, approval, execution, receipt, audit, and
   postcondition verification;
8. detectors alone own operational resolution.

Stale, partial, denied, failed, ambiguous, expired, unknown, or unavailable
evidence cannot become healthy, protected, calm, or resolved. A successful
provider mutation cannot become operational recovery.

## Implementation Commits

The complete specification is carried by the sequential core commits plus the
Phase 6 cross-repository boundary commits:

| Phase | Repository | Commit | Scope |
| --- | --- | --- | --- |
| 0 | `pulse` | `d43008e52` | alert detection and notification state boundary |
| 1 | `pulse` | `14b5731703` | lifecycle, evidence, and transition linkage |
| 2 | `pulse` | `bff47e8e05` | provider-aware protection posture |
| 3 | `pulse` | `b6a39ec33` | canonical Patrol attention workbench |
| 4 | `pulse` | `80b75f4d6` | availability as a resource facet |
| 5 | `pulse` | `9d880cfd1` | governed restart and verification loop |
| 6 | `pulse` | `f9c44c3d7` | lifecycle writers, evidence/mutation APIs, telemetry, docs, restart/load/accessibility proof |
| 6 | `pulse-mobile` | `3766b81` | canonical attention and approval/verification continuity |
| 6 | `pulse-enterprise` | `ffac3e0` | inert legacy remediation and canonical action ownership boundary |
| 6 | `pulse-pro` | `de1b9aa` | `ai_autofix` commercial entitlement ownership boundary |

Every commit is scoped to its repository. Pre-existing workspace changes remain
outside these commits.

## Required Proof Matrix

### Unit and contract proof

| Requirement | Result and canonical evidence |
| --- | --- |
| Lifecycle transitions, recurrence, and stale collection | Pass. `internal/operationaltrust/contracts_test.go`, `internal/alerts/operational_contract_test.go`, and `internal/alerts/operational_state_writers_test.go` cover every state, retry idempotency, recurrence, suppression expiry, stale/unknown precedence, resolving, restoration, and restart persistence. |
| Evidence freshness, partial permission, provider failure, and expiry | Pass. Operational Trust contract tests, attention evidence handler tests, recovery posture truth-table tests, and retained/expired API tests cover fresh/stale/unknown, partial/denied/unavailable, provider failure, missing retained payload, and typed `410 Gone`. |
| Identity resolution, ambiguity, and provider scoping | Pass. `internal/unifiedresources/availability_link_test.go`, relationship presentation tests, registry tests, and API authorization tests cover attached, standalone, ambiguous rejection, normalized IDs, evidence IDs, and scoped lookup. |
| Protection posture truth table | Pass. `internal/recovery/posture_test.go` covers protected, attention, unprotected, and unknown across freshness, verification, permission, history completeness, provider failure, and snapshot-only cases. |
| Notification grouping, retry, resolve linkage, cancellation, and dead letter | Pass. `internal/notifications/queue_test.go` plus the Phase 1 record cover grouping, exact transition linkage, retry, cancellation, dead letter, processor attachment, and restart during queued retry. |
| Attention projection and stable ordering | Pass. attention contract and performance tests cover one projection, stable order, filters, honest calm, bounded pagination, and 10,000 records. |
| Action eligibility, approval, idempotency, and verification | Pass. action lifecycle, planner, unified-resource, AI, and API suites cover fail-closed offer reasons, entitlement, plan replay, actor/plan binding, approval, exactly-once execution, timeout reconciliation, confirmed/contradicted/inconclusive verification, and detector-only resolution. |
| Supported-client JSON compatibility | Pass. alert, attention, relationship, action-origin, notification, and recovery JSON contract tests prove additive fields and legacy read compatibility. Full mobile tests prove the canonical client migration. |

Full core command:

```text
go test ./internal/operationaltrust ./internal/alerts \
  ./internal/notifications ./internal/recovery \
  ./internal/unifiedresources ./internal/actionlifecycle \
  ./internal/api ./internal/ai -count=1
```

Result: eight of eight packages passed.

Race command:

```text
go test -race ./internal/alerts ./internal/notifications \
  ./internal/actionlifecycle ./internal/operationaltrust -count=1
```

Result: four of four packages passed with no race report.

### Integration proof

| Requirement | Result |
| --- | --- |
| One observation opens one record and one attention item | Pass through alert operational-contract and attention projection integration tests. |
| Repeated observation does not duplicate either | Pass through deterministic transition/evidence IDs, same-state refresh dedupe, and attention projection tests. |
| Acknowledgement is reflected everywhere without resolving | Pass through the end-to-end API mutation/projection test, desktop component test, browser journey, and mobile canonical adapter. |
| Suppressed work leaves the default queue but remains inspectable | Pass through active/suppressed filter tests and the deepest browser lifecycle journey. |
| Source outage becomes stale or unknown, not false resolution | Pass through lifecycle writer tests, attention unavailable/calm tests, and availability browser proof. |
| Recovery evidence changes posture and related attention | Pass through recovery posture/store tests and Phase 2/3 projection tests. |
| Notification audit links to the exact transition | Pass through queue persistence, grouping, retry, and restart tests. |
| Action executes once and closes only after detector recovery | Pass through action lifecycle restart/receipt tests and both verified-action browser journeys. |
| Attached availability does not create a duplicate resource | Pass through unified-resource contract tests and the Phase 4 browser suite. |
| Authorization and tenant boundaries hold across evidence/actions | Pass through attention handler, action planning, route inventory, RBAC, and entitlement tests. |

### Browser proof

Managed current-source runtime:

```text
cd tests/integration
PULSE_E2E_USE_HOT_DEV=1 npm test -- \
  tests/90-operational-trust-protection-posture.spec.ts \
  tests/91-operational-trust-attention-workbench.spec.ts \
  tests/92-operational-trust-availability-facet.spec.ts \
  --project=chromium
```

Result: ten passed, one phone-only case skipped by project annotation.

```text
PULSE_E2E_USE_HOT_DEV=1 npm test -- \
  tests/90-operational-trust-protection-posture.spec.ts \
  tests/91-operational-trust-attention-workbench.spec.ts \
  --project=mobile-chrome
```

Result: seven passed, one desktop-only case skipped by project annotation.

The combined browser matrix proves:

1. normal authenticated monitor-first launch and one navigation to Patrol;
2. healthy/current, attention, stale, unknown, protected, unprotected, and
   unavailable-evidence presentation where applicable across platform,
   protection, availability, and attention surfaces;
3. active work and a current-coverage calm day;
4. acknowledged and suppressed filters plus reversible lifecycle controls;
5. stale collection without a healthy or resolved fallback;
6. deepest evidence, provider limitation, protection, relationship, and
   lifecycle detail;
7. action approval, execution, confirmed verification, contradicted
   verification, and still-open detector truth;
8. navigation and queue count consistency;
9. keyboard activation, screen-reader names, focus restoration after refreshed
   detail, reduced motion, and no document overflow at phone width;
10. default-three evidence with older observations demoted to disclosure.

### Failure proof

| Failure | Proof |
| --- | --- |
| Collector disconnected | stale lifecycle writer and calm/unavailable browser cases |
| Partial permission | evidence envelope and protection posture truth tables |
| Provider rate limit or timeout | typed provider-failure evidence and action timeout reconciliation |
| Notification destination failure and dead letter | notification queue retry/dead-letter tests |
| Ambiguous identity | unified-resource correlation rejection tests |
| Missing recovery history | unknown/unprotected posture truth table |
| Stale successful backup | attention posture truth table and browser drill-down |
| Action provider timeout | durable attempt/late-receipt restart tests |
| Successful provider response with failed postcondition | contradicted action lifecycle and browser journey |
| Restart during lifecycle transition or queued delivery | active-alert store restart and queued-retry restart tests |

### Performance proof

| Requirement | Proof |
| --- | --- |
| Bounded active-count latency | in-memory summary plus active-count mismatch telemetry |
| Bounded Patrol pagination | hard maximum 200 and 10,000-record projection under the five-second shared-CI ceiling |
| Bounded table enrichment | protection and action joins batch at 200 subjects/records |
| No per-row network pattern | browser posture proof asserts one bounded request; summary performs no posture/action reads |
| Stable retained evidence/recovery storage | retention, query-plan, batch, SQLite restart, and recovery-store suites |

## Cross-Repository Compatibility

### Pulse Mobile

The primary mobile findings read is now the canonical
`/api/ai/patrol/attention` list/detail contract. The display adapter preserves
operational record identity, resource, evidence quality, impact, protection,
relationships, next step, action origin, execution, and verification truth.
Acknowledgement uses the item-scoped canonical mutation. The unsafe legacy
one-tap dismiss path is not offered for canonical items because suppression
requires a reason and bounded expiry.

Full proof:

```text
npm run typecheck
CI=1 npm test
```

Result: 243 Jest suites and 2,433 Jest tests passed; 190 Node script tests
passed; 2,623 total tests passed.

The old investigation/dismiss/snooze methods remain only for cached pre-v6
records. They are not used by the primary v6 list/detail path. Removing them
requires the separately governed supported-client retention window, not a
second runtime.

### Pulse Enterprise

The historical remediation store remains readable history. Every authority-
bearing approval, execute, and rollback operation is permanently inert and
returns the typed retired error/HTTP `410 Gone`. The mutation registry guard
prevents executable command remediation from returning. New work uses the
core `/api/actions` lifecycle.

```text
go test ./internal/remediation ./internal/aiautofix ./test -count=1
```

Result: three of three packages passed.

### Pulse Pro

The generated self-hosted catalog is the commercial source for `ai_autofix`:
Community false, Relay false, Pro true. Core offer and plan handlers check that
exact entitlement and prove the missing-entitlement `402` contract. Pro and
Relay do not own lifecycle or action truth.

The current `pulse-pro` working tree had pre-existing uncommitted license email
signature work whose tests did not compile before this documentation-only
slice. That unrelated local state was neither modified nor included. The
Operational Trust commercial-boundary commit changes documentation only; the
required runtime entitlement behavior is covered by the passing core API
contract and the checked-in generated catalog.

No mobile push schema was widened. There is no canonical attention push
producer or consumer today, so adding unused `operational_record_id` fields
would create schema-only compatibility rather than a runtime contract. A
future attention notification transport is a new governed feature, outside
this specification.

## Compatibility and Migration Verdict

Supported compatibility is boundary-only:

- legacy alert payloads are read and normalized into canonical operational
  records; primary writes use lifecycle writers;
- legacy relationship payloads may omit additive IDs; primary projections
  author stable normalized IDs;
- historical enterprise remediation remains read-only and permanently inert;
- mobile cached legacy finding helpers do not feed the primary v6 queue;
- legacy recovery aliases remain input/read compatibility and do not own
  posture;
- notification storage migration preserves exact record/transition links.

No temporary compatibility path remains writable in the primary runtime.
Removal of supported read boundaries requires a separate migration decision
and client-retention evidence.

Upgrade and operator guidance now lives in:

- `docs/OPERATIONAL_TRUST.md`
- `docs/UPGRADE_v6.md`
- `docs/API.md`
- `docs/AI.md`
- `docs/monitoring/PROMETHEUS_METRICS.md`
- `pulse-mobile/OPERATIONAL_TRUST.md`
- `pulse-enterprise/docs/OPERATIONAL_TRUST.md`
- `pulse-pro/docs/OPERATIONAL_TRUST.md`

## Telemetry and Research

Operational Trust ships bounded telemetry for observation-to-open latency,
open-to-notification enqueue latency, evidence state, identity correlation,
protection evaluation/failure, notification delivery, active-count mismatch,
action offer eligibility, and action verification. Labels are enumerated and
never contain raw resource, record, evidence, actor, destination, or provider-
instance IDs.

The metric contract follows primary Prometheus guidance:

- [Metric and label naming](https://prometheus.io/docs/practices/naming/)
- [Histograms and summaries](https://prometheus.io/docs/practices/histograms/)
- [Instrumentation](https://prometheus.io/docs/practices/instrumentation/)
- [Writing client libraries](https://prometheus.io/docs/instrumenting/writing_clientlibs/)

Seconds are the base unit, latency uses histograms, and labels remain bounded.

Production-observed telemetry is not applicable to acceptance of these newly
introduced metrics because the implementation has not yet shipped to a
production release and no existing release gate requires fabricated
post-release samples. Registration, names, label bounds, and observation
behavior are covered by automated tests; the operator alert examples define
the post-release observation path.

## User Lens

Least-expert operator job:

> Show me what needs attention now, why Pulse believes it, what it affects,
> whether I am protected, and the one safe next action. If Pulse does not know,
> say that clearly.

Distance to answer:

- monitor-first launch to queue: one navigation;
- queue to evidence/protection/lifecycle answer: one item selection;
- item to governed mutation: one review handoff.

Default-visible actionability:

| Element | Action | Decision |
| --- | --- | --- |
| active count and filters | choose current work or retained lifecycle state | keep |
| ordered item row | open the next issue | keep |
| evidence/protection/resource summary | judge urgency and trust | keep |
| acknowledge / bounded suppress | make a reversible operator lifecycle decision | keep in selected detail |
| latest three evidence observations | understand current basis | keep in selected detail |
| older evidence, provider forensics, raw timeline | investigate when needed | demote to disclosure |
| policy/audit/receipt detail | review a mutation safely | demote to shared action review |
| generic Assistant prompt or proof strip | no action before selection | cut |

Vocabulary uses `Needs attention`, `Stale or unknown`, `Protection unknown`,
`Return to open`, and `Return to active`; provider and collector internals stay
in detail.

Relevant issue text and every referenced screenshot were inspected:

- [#1244](https://github.com/rcourtman/Pulse/issues/1244) asks for all Patrol
  findings at a glance.
- [#1234](https://github.com/rcourtman/Pulse/issues/1234) asks for centralized,
  reversible management of recommendations and dismissals.
- [#1580](https://github.com/rcourtman/Pulse/issues/1580) shows an acknowledged
  stale backup with a healthy-looking icon and no clear removal path.
- [#1545](https://github.com/rcourtman/Pulse/issues/1545) shows suppressed
  resources continuing to re-notify and controls that did not match the
  operator's mental model.
- [#1592](https://github.com/rcourtman/Pulse/issues/1592) shows missing backup
  coverage despite source evidence.
- [#1582](https://github.com/rcourtman/Pulse/issues/1582) shows availability
  timing and notification timestamps contradicting configuration.
- [#1519](https://github.com/rcourtman/Pulse/issues/1519) reports clock drift
  creating permanent stale/recovery loops.
- [#1496](https://github.com/rcourtman/Pulse/issues/1496) reports unbounded
  unified-resource history growth.

Verdict: `product`. Desktop and phone-width deepest states pass; every
default-visible element supports an operator decision; forensic detail is
available without turning the monitor into an object browser.

## Fourteen Completion Criteria

| # | Criterion | Verdict |
| --- | --- | --- |
| 1 | canonical domain contracts in code and governance | Pass |
| 2 | alert, navigation, Patrol, resource, notification share lifecycle truth | Pass |
| 3 | limited/failed/unknown evidence cannot masquerade as healthy/resolved | Pass |
| 4 | posture is provider-aware, linked, explainable, and batched | Pass |
| 5 | Patrol is a functional active-work and calm-day queue | Pass |
| 6 | availability attaches and shares evidence/lifecycle | Pass |
| 7 | narrow action completes plan/approve/execute/verify | Pass |
| 8 | every required proof passes or has canonical N/A justification | Pass |
| 9 | live Playwright product-quality and user-lens gates pass | Pass |
| 10 | supported API and persisted-data migrations are proven | Pass |
| 11 | subsystem, status, readiness, gate, upgrade, and operator docs agree | Pass |
| 12 | no temporary compatibility remains in the primary runtime | Pass |
| 13 | every implementation slice is scoped and committed | Pass |
| 14 | candidate and coverage gap are normalized through acceptance | Pass |

## Governance Decision

Accept candidate lane `protection-posture-attention-queue`. Its planned
coverage gap remains the typed historical reason for the lane expansion and is
normalized by the accepted candidate plus this completion record. There is no
remaining Operational Trust decision to move outside this specification.

This record satisfies the specification's release gate:

> Pulse v6 is not operationally trustworthy until every customer-facing active
> state, evidence explanation, protection assertion, notification consequence,
> and governed action resolves through one contradiction-free canonical
> lifecycle and evidence model.
