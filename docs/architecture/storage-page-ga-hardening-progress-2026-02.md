# Storage Page GA Hardening Progress Tracker

Linked plan:
- `docs/architecture/storage-page-ga-hardening-plan-2026-02.md`

Status: Active
Date: 2026-02-08

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Reviewer must provide explicit command exit-code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or replaced by summary-only claims.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. Update this file first in each implementation session and last before session end.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before starting the next packet.
7. Do not use `git checkout --`, `git restore --source`, `git reset --hard`, or `git clean -fd` on shared worktrees.
8. Respect packet subsystem boundaries; do not expand packet scope to adjacent streams.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| 00 | Storage Architecture Inventory and Contract Freeze | DONE | Claude | Claude | APPROVED | See Packet 00 Review Evidence |
| 01 | Route/Query Orchestration Extraction | TODO | Codex | Claude | PENDING | See Packet 01 Review Evidence |
| 02 | Shared Storage Domain Semantics Extraction | TODO | Codex | Claude | PENDING | See Packet 02 Review Evidence |
| 03 | Storage V2 View-Model Decomposition | TODO | Codex | Claude | PENDING | See Packet 03 Review Evidence |
| 04 | Storage V2 UX Parity Uplift | TODO | Codex | Claude | PENDING | See Packet 04 Review Evidence |
| 05 | Alert and Deep-Link Parity Hardening | TODO | Codex | Claude | PENDING | See Packet 05 Review Evidence |
| 06 | Adapter Identity and Merge Contract Hardening | TODO | Codex | Claude | PENDING | See Packet 06 Review Evidence |
| 07 | Backend Unified Storage Metadata Enrichment | TODO | Codex | Claude | PENDING | See Packet 07 Review Evidence |
| 08 | Frontend Consumption of Enriched Storage Metadata | TODO | Codex | Claude | PENDING | See Packet 08 Review Evidence |
| 09 | Contract Test Hardening and Monolith Regression Guardrails | TODO | Codex | Claude | PENDING | See Packet 09 Review Evidence |
| 10 | Final Certification and GA Readiness Verdict | TODO | Claude | Claude | PENDING | See Packet 10 Review Evidence |

## Packet 00 Checklist: Storage Architecture Inventory and Contract Freeze

### Discovery
- [x] High-risk storage flows enumerated with file/function anchors.
- [x] SP risk IDs mapped one-to-one to owner packets.
- [x] Rollback notes added for high-severity risks.

### Deliverables
- [x] Plan appendices updated with baseline and risk mapping.
- [x] Progress board initialized and validated.

### Required Tests
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [x] Exit codes recorded for all commands.

### Review Gates
- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### Packet 00 Review Evidence

```
Files changed:
- docs/architecture/storage-page-ga-hardening-plan-2026-02.md: Added Appendix A with 7 high-risk flow anchors (A.1-A.7)
- docs/architecture/storage-page-ga-hardening-progress-2026-02.md: Initialized board, checked Packet 00 items

Commands run + exit codes:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Gate checklist:
- P0: PASS (plan appendices exist with file/function anchors; tsc exit 0 verified)
- P1: N/A (docs-only packet, no behavioral changes)
- P2: PASS (progress tracker updated, risk register complete with packet mappings)

Verdict: APPROVED

Commit:
- PENDING (will be filled after checkpoint commit)

Residual risk:
- Line numbers in anchors may shift as subsequent packets modify files; re-anchor if needed.

Rollback:
- Revert appendix additions to plan doc; reset progress tracker Packet 00 entries to TODO.
```

## Packet 01 Checklist: Route/Query Orchestration Extraction

### Implementation
- [ ] Shared storage route-state hook created.
- [ ] Legacy and V2 storage pages consume shared query sync contract.
- [ ] Query canonicalization and deep-link semantics preserved.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/Storage.routing.test.tsx src/components/Storage/__tests__/StorageV2.test.tsx src/routing/__tests__/resourceLinks.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 01 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 02 Checklist: Shared Storage Domain Semantics Extraction

### Implementation
- [ ] Ceph/ZFS/source/status helper logic extracted to shared module.
- [ ] Duplicate helper definitions removed from both storage shells.
- [ ] New unit tests cover extracted storage domain helpers.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageDomain.test.ts src/components/Storage/__tests__/StorageV2.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 02 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 03 Checklist: Storage V2 View-Model Decomposition

### Implementation
- [ ] View-model hooks created for transform-heavy logic.
- [ ] `StorageV2.tsx` reduced to composition + interaction wiring.
- [ ] Ceph grouping/summaries preserved through hook outputs.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 03 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 04 Checklist: Storage V2 UX Parity Uplift

### Implementation
- [ ] Preview-only framing removed from default storage experience.
- [ ] Persistent filter/sort/view behavior implemented and stable.
- [ ] Empty/loading/disconnected UX parity verified.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/routing/__tests__/platformTabs.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 04 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 05 Checklist: Alert and Deep-Link Parity Hardening

### Implementation
- [ ] Alert style/priority behavior restored in V2 row model.
- [ ] Resource-based deep-link highlight/expand behavior implemented.
- [ ] Reconnect/stale-state messaging remains behaviorally correct.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 05 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 06 Checklist: Adapter Identity and Merge Contract Hardening

### Implementation
- [ ] Canonical storage identity key contract added.
- [ ] Deterministic precedence (`resource` over `legacy`) enforced per logical identity.
- [ ] Duplicate logical records prevented under mixed-origin input.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageAdapters.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 06 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 07 Checklist: Backend Unified Storage Metadata Enrichment

### Implementation
- [ ] Unified storage payload extended with storage-specific metadata.
- [ ] Adapter conversion populates metadata from state snapshot.
- [ ] API response compatibility preserved for existing consumers.

### Required Tests
- [ ] `go test ./internal/unifiedresources/... ./internal/api/... -run "ResourcesV2|storage|Storage|registry" -count=1` passed.
- [ ] `go build ./...` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 07 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 08 Checklist: Frontend Consumption of Enriched Storage Metadata

### Implementation
- [ ] Frontend storage adapters prefer enriched unified metadata.
- [ ] `useResources.asStorage` merge precedence made explicit and deterministic.
- [ ] Duplicate assignment/merge ambiguity removed.

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/features/storageBackupsV2/__tests__/storageAdapters.test.ts src/components/Storage/__tests__/StorageV2.test.tsx` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 08 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 09 Checklist: Contract Test Hardening and Monolith Regression Guardrails

### Implementation
- [ ] Parity contract tests added for route/data/interaction layers.
- [ ] Guardrails added to prevent duplicated storage-domain helpers in page shells.
- [ ] Packet remains storage-domain scoped (no unrelated test rewrites).

### Required Tests
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx src/components/Storage/__tests__/Storage.routing.test.tsx src/features/storageBackupsV2/__tests__/storageAdapters.test.ts` passed.
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 09 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Packet 10 Checklist: Final Certification and GA Readiness Verdict

### Certification
- [ ] All packets 00-09 are `DONE` and `APPROVED`.
- [ ] Checkpoint commit hashes recorded for each approved packet.
- [ ] Residual risks and debt ledger entries reviewed.

### Final Validation Gate
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` passed.
- [ ] `cd frontend-modern && npx vitest run` passed.
- [ ] `go build ./...` passed.
- [ ] Exit codes recorded for all commands.

### Review Gates
- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### Packet 10 Review Evidence

```
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>
2. `<command>` -> exit <code>
3. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Final recommendation:
- GO | GO_WITH_CONDITIONS | NO_GO

Blocking items:
- <id>: <description>

Rollback:
- <steps>
```
