# Release Documentation Readiness Plan (Pre-Public-Release)

Status: Active
Owner: Pulse
Date: 2026-02-09

Progress tracker:
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Inputs:
- `docs/architecture/release-readiness-guiding-light-2026-02.md`
- `docs/architecture/gap-analysis-2026-02.md`
- `docs/architecture/truenas-operational-runbook.md`
- `docs/architecture/hosted-operational-runbook-2026-02.md`
- `docs/architecture/multi-tenant-operational-runbook.md`

## Intent

This lane ratifies all release-facing documentation so operators and users can run, rollback, and troubleshoot public release safely.

Primary outcomes:
1. Architecture status docs reflect actual lane completion state.
2. Runbooks are reconciled and consistent across W2/W4/W5/W6.
3. Changelog and release notes are coherent and accurate.
4. Deferred risks and launch conditions are explicit.

## Non-Negotiable Contracts

1. No doc claims without concrete evidence links.
2. Status fields must match packet/progress trackers.
3. Contradictory release posture statements are blocking (`P0`).
4. Packet scope is docs-only unless an explicit test/doc mismatch requires a tiny code fix.

## Source-of-Truth Hierarchy (DOC-00)

When documents conflict, the higher-authority source wins:

1. **Progress trackers** (`*-progress-2026-02.md`) — canonical packet status, evidence, and verdicts.
2. **Plans** (`*-plan-2026-02.md`) — canonical scope, checklists, and acceptance criteria.
3. **Summary docs** (guiding-light, gap-analysis, closeout) — derived from trackers; must be reconciled, never authoritative over trackers.

Rule: If a summary doc says "In Progress" but the progress tracker says "COMPLETE", the tracker is correct and the summary must be updated.

## Approved Verdict Vocabulary (DOC-00)

All status and verdict fields across this lane MUST use exactly these terms:

| Term | Meaning |
|---|---|
| `PENDING` | Not started |
| `IN_PROGRESS` | Active work underway |
| `DONE` | Implementer considers work complete; awaiting review |
| `APPROVED` | Reviewer has verified and approved |
| `CHANGES_REQUESTED` | Review failed; rework needed |
| `BLOCKED` | Cannot proceed due to external dependency |
| `COMPLETE` | Lane-level: all packets DONE/APPROVED |
| `LANE_COMPLETE` | Equivalent to COMPLETE (legacy synonym, acceptable) |
| `GO` | Final verdict: ready to ship |
| `GO_WITH_CONDITIONS` | Final verdict: ready to ship with documented conditions |
| `NO_GO` | Final verdict: not ready to ship |

Status fields on progress trackers use: `PENDING`, `IN_PROGRESS`, `DONE`, `APPROVED`, `CHANGES_REQUESTED`, `BLOCKED`.
Lane-level status uses: `COMPLETE`, `LANE_COMPLETE`, `In Progress`.
Final verdicts use: `GO`, `GO_WITH_CONDITIONS`, `NO_GO`.

## Frozen In-Scope File List (DOC-00)

These are the only files this lane may read or modify:

### Tier 1: This lane's own files
- `docs/architecture/release-documentation-readiness-plan-2026-02.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

### Tier 2: Architecture summary docs (DOC-01)
- `docs/architecture/release-readiness-guiding-light-2026-02.md`
- `docs/architecture/gap-analysis-2026-02.md`

### Tier 3: Operational runbooks (DOC-02)
- `docs/architecture/truenas-operational-runbook.md`
- `docs/architecture/hosted-operational-runbook-2026-02.md`
- `docs/architecture/multi-tenant-operational-runbook.md`
- `docs/architecture/conversion-operations-runbook.md`

### Tier 4: Release-facing artifacts (DOC-03)
- `CHANGELOG-DRAFT.md`
- `docs/architecture/program-closeout-certification-plan-2026-02.md`
- `docs/architecture/program-closeout-certification-progress-2026-02.md`

### Tier 5: Read-only reference (for cross-checking, never modified by this lane)
All 27 progress trackers in `docs/architecture/*-progress-2026-02.md` are read-only references for status verification.

## Required Review Output (Every Packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Commit:
- `<short-hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Execution Packets

### DOC-00: Scope Freeze + Source of Truth Map

Objective:
- Freeze documentation scope and define authoritative source files.

Scope:
- `docs/architecture/release-documentation-readiness-plan-2026-02.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Checklist:
1. Define source-of-truth hierarchy (progress trackers > plans > summaries).
2. Define approved terminology and verdict vocabulary.
3. Freeze in-scope docs list.

### DOC-01: Architecture Snapshot Ratification

Objective:
- Bring guiding-light and gap-analysis into full alignment with current tracker state.

Scope (max 4 files):
- `docs/architecture/release-readiness-guiding-light-2026-02.md`
- `docs/architecture/gap-analysis-2026-02.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Checklist:
1. Reconcile W0..W6 status language.
2. Remove stale "in progress" claims for completed lanes.
3. Capture remaining real residuals only.

Required commands:
1. `rg -n "^Status:|PENDING|BLOCKED|IN_PROGRESS|DONE|COMPLETE|LANE_COMPLETE" docs/architecture/*progress*2026-02*.md`
2. `rg -n "W0|W1|W2|W3|W4|W5|W6|Status|In Progress|Partial|Complete" docs/architecture/release-readiness-guiding-light-2026-02.md docs/architecture/gap-analysis-2026-02.md`

### DOC-02: Runbook Consistency and Rollback Accuracy

Objective:
- Ensure runbook procedures and fallback steps are coherent and executable.

Scope (max 6 files):
- `docs/architecture/truenas-operational-runbook.md`
- `docs/architecture/hosted-operational-runbook-2026-02.md`
- `docs/architecture/multi-tenant-operational-runbook.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Checklist:
1. Verify enable/disable/rollback sections are present and consistent.
2. Verify kill-switch commands and decision points.
3. Verify incident severity ladder and response SLA sections.

### DOC-03: Release Notes and Debt Ledger Closeout

Objective:
- Produce final release-facing change summary and deferred work ledger.

Scope (max 5 files):
- `CHANGELOG-DRAFT.md`
- `docs/architecture/program-closeout-certification-plan-2026-02.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Checklist:
1. Consolidate user-visible and operator-visible changes.
2. Consolidate deferred items with severity/owner/date.
3. Validate wording consistency with final lane verdicts.

### DOC-04: Final Documentation Verdict

Objective:
- Issue final docs readiness verdict.

Scope:
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

Checklist:
1. Verify DOC-00..DOC-03 are `DONE/APPROVED`.
2. Re-run source-of-truth consistency checks.
3. Record verdict: `GO` / `GO_WITH_CONDITIONS` / `NO_GO`.
