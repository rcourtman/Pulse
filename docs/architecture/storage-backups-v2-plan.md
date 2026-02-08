# Storage + Backups V2 Migration Plan

Status: Partially Complete (Storage GA complete; legacy deprecation follow-on pending)
Owner: Pulse core app
Last updated: 2026-02-08

## Execution status (2026-02-08)
- Storage GA execution is complete with `APPROVED` evidence in:
  - `docs/architecture/storage-page-ga-hardening-plan-2026-02.md`
  - `docs/architecture/storage-page-ga-hardening-progress-2026-02.md`
- This document remains the strategic north-star for cross-platform Storage/Backups evolution.
- Remaining open scope is the long-tail deprecation phase (legacy route/shell retirement after stability window).

## Why this exists
Storage and Backups currently carry legacy assumptions from the old Pulse model:
- Storage is still primarily PVE-node-centric.
- Backups is still primarily PBS/PVE-flow-centric.

Infrastructure and Workloads already moved toward source-agnostic resource views. This plan defines how Storage/Backups get to the same standard without breaking current production behavior.

## North Star
Build `StorageV2` and `BackupsV2` as source-agnostic pages where:
- core UI is capability-first, not platform-first
- platform specifics live in adapters only
- legacy pages remain stable until V2 reaches parity

## Non-negotiable rules
1. No platform-specific conditional rendering in V2 page core components.
2. Platform mapping belongs in adapters/view-model builders only.
3. New sources must be addable by adapter extension, not page rewrites.
4. Legacy pages remain untouched except for critical bug fixes during V2 build.
5. Route switch only happens after parity checklist passes.

## Scope
In scope:
- new V2 page shells
- shared source-agnostic domain models
- adapter pipeline from unified resources and legacy fallback
- parity migration from legacy pages to V2

Out of scope:
- removing legacy backend arrays immediately
- changing existing API contracts for external users
- UI polish beyond what is required for functional parity

## Platform expansion map
Current first-class sources:
- Proxmox VE
- Proxmox Backup Server
- Proxmox Mail Gateway

Next targets (near-term):
- Kubernetes (PVCs, CSI snapshots, backup controllers)
- TrueNAS
- Unraid

Future candidates:
- Synology DSM
- VMware vSphere
- Microsoft Hyper-V
- AWS / Azure / GCP
- Docker
- Host Agent

Rule: adding any platform above must require only adapter implementation and capability mapping, not page-level UI rewrites.

## Target architecture

### Shared domain models
Define page-level view models in frontend:
- `StorageRecordV2`
- `BackupRecordV2`

These models expose common fields only (status, capacity, location, time, verification, protection, source metadata) and optional extension metadata blocks.

### Adapter layer
Add dedicated adapter modules:
- `storageV2Adapters/*`
- `backupV2Adapters/*`

Responsibilities:
- ingest from `state.resources` first
- supplement with legacy arrays when resource coverage is missing
- normalize statuses/capabilities consistently
- produce stable IDs for table rows/charts/links

### Page composition
`StorageV2` and `BackupsV2` should consume only:
- V2 view-model hooks
- generic filter state (status/source/capability/search/scope)

They should not directly parse source-specific raw state.

## Phased implementation

### Phase 0: Context lock (this doc)
- Freeze goals, constraints, and acceptance criteria.
- Keep this document updated when tradeoffs change.

### Phase 1: Model + adapter foundation
- Add `StorageRecordV2` and `BackupRecordV2` types.
- Build adapter pipeline with unified-resources-first behavior.
- Add adapter unit tests for:
  - PVE-only data
  - PBS-only data
  - PMG-only data
  - mixed-source data
  - missing unified resources fallback

Exit criteria:
- V2 adapters produce stable outputs for all current supported source combinations.

### Phase 2: V2 page shells
- Create new components:
  - `StorageV2.tsx`
  - `BackupsV2.tsx`
- Implement minimal capability-first UI:
  - common summary cards
  - source-agnostic filter bars
  - baseline tables

Exit criteria:
- pages render correctly with mixed-source mock states
- no direct source-specific branching in core render flow

### Phase 3: Thread in legacy feature parity
- Migrate useful legacy features one by one into V2 via adapter metadata:
  - Storage: grouping, health details, expandable details
  - Backups: charting, verification/protection indicators, host backup handling
- Keep each migrated feature generic in UI vocabulary.

Exit criteria:
- parity checklist mostly complete (see below)

### Phase 4: Side-by-side rollout
- Add route-level gating (feature flag/dev toggle) for old vs V2 pages.
- Run V2 in daily usage while collecting regressions.
- Fix gaps without reintroducing platform coupling.

Exit criteria:
- critical/major parity gaps resolved
- no blocking regressions in navigation/filtering/performance

### Phase 5: Route switch + deprecation — COMPLETE
- Switch `/storage` and `/backups` default routes to V2. ✓
- Keep legacy pages for one release cycle behind fallback route. ✓
- Remove legacy pages after deprecation window. ✓

Exit criteria:
- legacy routes removed ✓ (SB5-05, commit `095b1e82`)
- docs/screenshots updated ✓ (SB5-06 certification)

Execution tracker: `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md`
Packets SB5-00 through SB5-06 all DONE/APPROVED.

## Parity checklist (must pass before default switch)

Storage:
- shows all storage-like entities from all supported sources
- unified filter semantics (status/source/capability/search)
- stable grouping and row identity
- capacity and health summaries correct

Backups:
- shows snapshots/local/remote/host-style artifacts across sources
- consistent verification/protection semantics
- chart and table numbers align
- query params and deep links remain stable

Cross-cutting:
- keyboard/accessibility parity
- no route-level regressions
- performance acceptable at current data scale

## Risk register
1. Mixed-source semantic mismatch (same term, different meaning by source)
Mitigation: central capability normalization map in adapters.

2. Silent fallback masking resource coverage gaps
Mitigation: add debug counters for unified vs legacy contribution in dev mode.

3. Feature parity drift during long migration
Mitigation: require checklist updates in this doc when scope changes.

## Decision log
2026-02-07:
- Decided to build fresh `StorageV2`/`BackupsV2` instead of incremental legacy retrofit.
- Decided adapters must be unified-resource-first with legacy fallback.
- Decided route switch happens only after parity checklist completion.

## Follow-on actions
1. Define a dedicated Phase 5 deprecation packet set for legacy Storage/Backups route/page retirement.
2. Collect one release-cycle stability evidence for V2 default paths before removing legacy fallbacks.
3. Execute legacy route/page removal only after parity checklist and rollback gates are re-validated.
