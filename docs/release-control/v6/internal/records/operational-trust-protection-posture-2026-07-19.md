# Operational Trust Protection Posture Record

Date: 2026-07-19

## Scope

This record closes the Phase 2 protection-posture slice in
`OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`. It does not claim completion of
the Patrol attention workbench, availability attachment, governed actions, or
the full Operational Trust specification.

## User job and evidence

The least-expert plausible user's job is: “Tell me which workloads I can
actually recover, which need attention, and why.”

Relevant issue history shows why artifact presence cannot be the answer:

1. [#1541](https://github.com/rcourtman/Pulse/issues/1541) reports current PBS
   backups displayed with an incorrect unverified conclusion.
2. [#81](https://github.com/rcourtman/Pulse/issues/81) records a token that can
   list datastores but cannot enumerate backup snapshots, proving that partial
   permission must remain visible rather than becoming a healthy empty result.
3. [#1389](https://github.com/rcourtman/Pulse/issues/1389) records VM and
   container backups mis-correlated by numeric id, proving that provider-scoped
   subject identity is part of protection evidence.
4. [#1592](https://github.com/rcourtman/Pulse/issues/1592) and
   [#1437](https://github.com/rcourtman/Pulse/issues/1437) report missing PVE
   backups and snapshots, proving that one provider's visible artifacts do not
   establish complete cross-provider coverage.
5. [#1056](https://github.com/rcourtman/Pulse/issues/1056) and
   [#1162](https://github.com/rcourtman/Pulse/issues/1162) show the operator
   cost of treating retained orphan artifacts as current workload protection.

## Canonical runtime result

1. `internal/recovery/model/posture.go` defines the provider-neutral posture,
   provider-state, policy, query, and provider-observation contracts. The only
   customer-facing states are protected, attention, unprotected, and unknown.
2. `internal/recovery/posture.go` derives posture by canonical resource id and
   provider scope. Protected requires a current subject-linked backup plus
   complete sufficient collection evidence. Attention covers actionable stale,
   failed, partial, or expected-but-unverified evidence. Unprotected requires
   complete evidence of no qualifying backup. Unknown is mandatory for
   identity, permission, history, or collection uncertainty.
3. Backup and snapshot semantics remain distinct. A snapshot alone never
   proves independent recovery, provider job success without a subject-linked
   point never becomes subject protection, and a limitation from an unrelated
   provider cannot erase a confirmed complete PBS recovery.
4. Recovery-point persistence adds provider scope and typed evidence
   additively. Existing databases migrate in place, and legacy points receive
   explicit unknown-quality evidence rather than invented certainty.
5. Provider observations and materialized postures have indexed, bounded,
   retained storage. Requested postures are re-evaluated from current points
   and observations at read time so a reassuring stored row cannot age into a
   lie. Point, observation, and posture retention share the 90-day boundary.
6. PBS is the first explicit provider adapter. Every poll records complete,
   partial, unavailable, or denied collection evidence separately from its
   subject-linked recovery points. Collection evidence is persisted before the
   point batch and reconciliation, so a large point write, timeout, cached
   artifact path, or failed enumeration cannot preserve a healthier claim.
7. `GET /api/recovery/postures` supplies one-resource, bounded batch, paged
   list, and `state=attention` reads under `monitoring:read`. Resource batches
   are capped at 200, return unknown for requested ids without sufficient
   evidence, and include policy plus provider evidence for drill-down.
8. `useProtectionPostures` sorts and deduplicates canonical resource ids,
   fetches one batch per 200 resources, and exposes a keyed read model. Proxmox
   passes exact unified-resource ids and does not parse table keys or derive
   posture from raw PBS, PVE, or guest-snapshot artifacts.
9. The Proxmox Backups coverage table keeps the default monitor compact. Each
   workload has one canonical status and latest restore time; the row
   drill-down contains the plain-language reason, human-readable provider
   evidence quality, and bounded restore artifacts.
10. `docs/API.md` and `docs/RECOVERY.md` document the bounded API, the
    artifact-versus-posture distinction, the four states, and unknown-state
    troubleshooting.

## Failure, migration, retention, and performance proof

The Phase 2 proof covers:

1. the provider-aware derivation truth table, including stale, failed,
   unverified, partial, denied, missing-identity, snapshot-only, complete-empty,
   and mixed-provider cases
2. schema migration and legacy evidence backfill
3. read-time re-evaluation over a deliberately corrupted reassuring
   materialization
4. indexed query plans and bounded 200-resource API batches
5. recovery-point, provider-observation, and materialized-posture retention
6. complete, partial, unavailable, and denied PBS collection mappings
7. provider-observation persistence before a deliberately failed point write
8. API authorization, validation, pagination, attention filtering, missing
   requested resources, and compatibility serialization
9. frontend batch normalization, one-call table integration, state filtering,
   evidence drill-down, type-check, and Proxmox regression tests

## Live product and user-lens proof

The changed surface was exercised against the managed live runtime at
`/proxmox/backups`, through the deepest row expansion, at desktop and 390px
mobile widths.

1. Distance to goal is the existing Proxmox Backups route, the Coverage
   selector, then one workload expansion for the reason and evidence.
2. The live pass found an inherited presentation defect: the explanation and
   provider evidence were initially repeated inside every PBS table cell. That
   detail was demoted to the expansion. The resulting workload row is 33px
   high and the default table remains scannable.
3. Default-visible elements all support the job: the four-state summary
   identifies fleet posture, filters narrow the queue, the table answers per
   workload, and the expansion explains the claim. Raw artifacts are retained
   only as forensic restore evidence.
4. Provider codes were replaced with product vocabulary in the expansion
   (`Proxmox Backup Server`, `Proxmox VE`), and evidence-quality values are
   presented as readable labels.
5. The live dataset honestly remained unknown where no complete provider
   collection observation had yet been persisted, despite current verified PBS
   artifacts. This is the required fail-closed behavior, not a healthy guess.
6. The phone-sized page has no body-level horizontal overflow; wide evidence
   tables scroll inside their bounded table wrappers.
7. The durable Playwright guard proves one bounded posture request, compact
   default presentation, evidence-on-expansion, and mobile containment.

The Phase 2 surface is classified as `product`.

## Boundary carried forward

Phase 2 supplies canonical protection truth but does not invent a work queue.
Phase 3 must project lifecycle records into typed attention items and consume
this posture as one input. Patrol must not parse recovery metadata, raw
artifacts, provider job payloads, or table presentation state to create its own
protection verdict.
