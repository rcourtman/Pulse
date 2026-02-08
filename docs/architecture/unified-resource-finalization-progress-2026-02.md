# Unified Resource Finalization Progress Tracker

Linked plan:
- `docs/architecture/unified-resource-finalization-plan-2026-02.md` (authoritative execution spec)

Related lanes:
- `docs/architecture/unified-resource-convergence-phase-2-progress-2026-02.md` (complete predecessor)
- `docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` (active dependency)

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
9. URF-05 cannot execute unless URF-04 gate is explicitly `GO`.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| URF-00 | Scope Freeze and Residual Gap Baseline | TODO | Codex | Claude | — | — |
| URF-01 | Organization Sharing Cutover to Unified Resources API | DONE | Codex | Claude | APPROVED | URF-01 Review Evidence |
| URF-02 | Alerts Runtime Cutover Off Legacy Conversion Hook | TODO | Codex | Claude | — | — |
| URF-03 | AI Chat Runtime Cutover Off Legacy Conversion Hook | TODO | Codex | Claude | — | — |
| URF-04 | SB5 Dependency Gate + Legacy Hook Deletion Readiness | TODO | Claude | Claude | — | — |
| URF-05 | Remove Frontend Runtime `useResourcesAsLegacy` Path | TODO | Codex | Claude | — | — |
| URF-06 | AI Backend Contract Scaffold (Legacy -> Unified) | TODO | Codex | Claude | — | — |
| URF-07 | AI Backend Migration to Unified Provider | TODO | Codex | Claude | — | — |
| URF-08 | Final Certification + V2 Naming Convergence Readiness | TODO | Claude | Claude | — | — |

---

## URF-00 Checklist: Scope Freeze and Residual Gap Baseline

- [ ] Residual gap baseline verified against current code.
- [ ] Definition-of-done grep contracts recorded and approved.
- [ ] Packet boundaries/dependency gates ratified.

### Required Tests

- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-00 Review Evidence

```markdown
TODO
```

---

## URF-01 Checklist: Organization Sharing Cutover to Unified Resources API

- [x] `OrganizationSharingPanel` no longer fetches `/api/resources`.
- [x] Option loading/sorting/validation behavior preserved.
- [x] Regression tests added for quick-pick and manual entry paths.
- [x] Any needed `apiClient` adjustments are scoped and tested.

### Required Tests

- [x] `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/ResourcePicker.test.tsx` -> exit 0
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### URF-01 Review Evidence

```markdown
Files changed:
- `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`: Replaced legacy `/api/resources` fetch with reactive `useResources()` hook. Removed `ResourcesResponse` type, `toResourceOptions` function, and `apiFetchJSON` import. Added `unifiedResourceOptions` memo derived from unified resources. Added `createEffect` for manual-entry auto-expand.
- `frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx` (new): 5 regression tests covering loading skeleton, unified resource option derivation, quick-pick field population, manual type validation, and share creation payload.
- `frontend-modern/src/components/Settings/__tests__/ResourcePicker.test.tsx` (new): 6 regression tests covering resource rendering, type filter buttons, search filtering, toggle selection, max selection limit, and select-all/clear-all.

Commands run + exit codes (reviewer-rerun):
1. `cd frontend-modern && npx vitest run src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/ResourcePicker.test.tsx` -> exit 0 (11 tests passed, 2 files)
2. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
3. `grep '/api/resources' frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` -> 0 matches (cutover confirmed)
4. `grep 'apiFetchJSON' frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx` -> 0 matches (legacy fetch removed)

Gate checklist:
- P0: PASS (files exist with expected edits, both required commands rerun by reviewer with exit 0)
- P1: PASS (resource option derivation, quick-pick, manual entry validation, share creation payload all tested; no legacy conversion behavior introduced)
- P2: PASS (progress tracker updated, packet evidence recorded)

Verdict: APPROVED

Residual risk:
- None. The `apiClient.ts` skip-redirect entry for `/api/resources` (line 402) was not removed per scope constraints — this is harmless configuration metadata and is deferred to URF-08 final certification.

Rollback:
- Revert `OrganizationSharingPanel.tsx` to previous version (restore `apiFetchJSON('/api/resources')` call pattern).
- Delete the two new test files.
```

---

## URF-02 Checklist: Alerts Runtime Cutover Off Legacy Conversion Hook

- [ ] Alerts runtime no longer depends on `useAlertsResources()` legacy conversion output.
- [ ] Override mapping/grouping/display behavior preserved.
- [ ] Compatibility fallback remains bounded and explicit.
- [ ] Tests lock unified-path parity.

### Required Tests

- [ ] `cd frontend-modern && npx vitest run src/pages/__tests__/Alerts.helpers.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-02 Review Evidence

```markdown
TODO
```

---

## URF-03 Checklist: AI Chat Runtime Cutover Off Legacy Conversion Hook

- [ ] AI chat context/mentions no longer depend on `useAIChatResources()` legacy conversion output.
- [ ] Mention IDs and summary semantics remain stable.
- [ ] Tests lock unified-path parity.

### Required Tests

- [ ] `cd frontend-modern && npx vitest run src/components/AI/__tests__/aiChatUtils.test.ts src/stores/__tests__/aiChat.test.ts src/hooks/__tests__/useResources.test.ts` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-03 Review Evidence

```markdown
TODO
```

---

## URF-04 Checklist: SB5 Dependency Gate + Legacy Hook Deletion Readiness

- [ ] SB5 packets required for deletion are `DONE/APPROVED`.
- [ ] Runtime references to legacy storage/backups shells verified.
- [ ] Packet decision recorded as `GO` or `BLOCKED` with evidence.

### Required Tests

- [ ] `rg -n "useResourcesAsLegacy\(" frontend-modern/src` -> reviewed
- [ ] `rg -n "Storage\.tsx|UnifiedBackups\.tsx" docs/architecture/storage-backups-phase-5-legacy-deprecation-progress-2026-02.md` -> reviewed

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED` or `BLOCKED`

### URF-04 Review Evidence

```markdown
TODO
```

---

## URF-05 Checklist: Remove Frontend Runtime `useResourcesAsLegacy` Path

- [ ] Runtime `useResourcesAsLegacy` usages removed.
- [ ] Transitional wrapper exports removed/updated.
- [ ] Alerts/AI imports cleaned up.
- [ ] Tests updated for non-legacy runtime contract.

### Required Tests

- [ ] `cd frontend-modern && npx vitest run src/hooks/__tests__/useResources.test.ts src/pages/__tests__/Alerts.helpers.test.ts src/components/AI/__tests__/aiChatUtils.test.ts` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-05 Review Evidence

```markdown
TODO
```

---

## URF-06 Checklist: AI Backend Contract Scaffold (Legacy -> Unified)

- [ ] Unified-resource-native AI provider contract introduced.
- [ ] Legacy bridge retained for one packet compatibility window.
- [ ] Parity tests added for old/new contract behavior.

### Required Tests

- [ ] `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
- [ ] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-06 Review Evidence

```markdown
TODO
```

---

## URF-07 Checklist: AI Backend Migration to Unified Provider

- [ ] AI unified context path uses unified provider by default.
- [ ] Legacy contract dependency narrowed and documented.
- [ ] Parity tests updated and passing.

### Required Tests

- [ ] `go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0
- [ ] `go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-07 Review Evidence

```markdown
TODO
```

---

## URF-08 Checklist: Final Certification + V2 Naming Convergence Readiness

- [ ] URF-00 through URF-07 are `DONE` and `APPROVED`.
- [ ] Full milestone validation commands rerun with explicit exit codes.
- [ ] Grep completion checks for `/api/resources` and `useResourcesAsLegacy` runtime usage recorded.
- [ ] Final readiness verdict recorded (`READY_FOR_NAMING_CONVERGENCE` or `NOT_READY`).

### Required Tests

- [ ] `cd frontend-modern && npx vitest run && frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [ ] `go build ./... && go test ./internal/api/... -run "ResourcesV2|ResourceHandlers|Websocket" -count=1 && go test ./internal/ai/... -run "ResourceContext|Routing" -count=1` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### URF-08 Review Evidence

```markdown
TODO
```

---

## Checkpoint Commits

- URF-00: TODO
- URF-01: TODO
- URF-02: TODO
- URF-03: TODO
- URF-04: TODO
- URF-05: TODO
- URF-06: TODO
- URF-07: TODO
- URF-08: TODO

## Current Recommended Next Packet

- `URF-02` (Alerts Runtime Cutover Off Legacy Conversion Hook)
