# Operational Trust Lifecycle, Evidence, and Notification Linkage Record

Date: 2026-07-19

## Scope

This record closes the Phase 1 foundation slice in
`OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`. It does not claim completion of
protection posture, the Patrol attention workbench, availability attachment,
governed actions, or the full Operational Trust specification.

## Canonical runtime result

1. `internal/operationaltrust/contracts.go` defines the shared typed evidence,
   operational record, lifecycle transition, and notification-link contracts.
2. Evidence distinguishes complete, partial, and unavailable observations;
   confirmed, inferred, and unknown confidence; sufficient, partial, denied,
   and unknown permission posture; provider source; observation and ingestion
   time; bounded payload references; and auditable identity correlation.
3. Alert records carry additive operational records, bounded evidence, the
   latest transition, and a bounded durable transition timeline. Legacy alert
   projections use explicit partial/unknown reasons instead of inventing
   provider certainty.
4. Acknowledgement and unacknowledgement are lifecycle transitions, never
   resolution. Resolution adds distinct recovery evidence and persists the
   resolved record and timeline into alert history.
5. Canonical detector evidence is preserved as typed source evidence. Repeated
   observations do not duplicate transition ids. A recurrence inside the
   existing cooldown window reopens the same stable cause with a
   resolved-to-open detector transition and retains the prior timeline.
6. The notification queue and audit log persist one `NotificationLink` per
   included alert. Grouped delivery retains every record and transition id.
   Queue retry retains the same notification id and links, while queued,
   delivering, retrying, delivered, cancelled, failed, and dead-letter states
   remain delivery consequences rather than operational truth.
7. Resolution notifications link to the recovery-evidence transition.
   Partial cancellation of a grouped firing delivery removes only the
   resolved alert and its corresponding link.
8. Existing queue databases migrate additively with JSON link columns on both
   queue and audit tables. Older alert JSON remains readable, and new fields
   are optional for supported clients.
9. `frontend-modern/src/types/operationalTrust.ts` is the single TypeScript
   projection used by the alert API type instead of re-declaring lifecycle and
   evidence vocabulary in UI features.

## Proof

The completed slice is covered by:

1. deterministic evidence, transition, and notification id tests
2. evidence limitation, permission, freshness, correlation, clone, and
   validation tests
3. alert open, acknowledgement, unacknowledgement, resolution, recurrence,
   JSON compatibility, history persistence, and deep-copy tests
4. grouped notification linkage, retry, delivery, cancellation rewrite,
   database migration, audit persistence, destination identity, DLQ API, and
   query-plan tests
5. focused Go race proof for operational lifecycle and notification linkage
6. frontend TypeScript type-check and formatting proof
7. subsystem registry, contract, and control-plane audits

## Boundary carried forward

Phase 1 supplies typed truth and durable linkage but intentionally adds no new
customer-facing operational-trust presentation. Freshness, completeness,
source explanation, protection posture, Patrol attention projection, and
action eligibility must consume these contracts in their owning later phases.
Notification queue status must never become a navigation count, alert state,
or substitute detector.
