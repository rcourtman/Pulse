# Release Final Certification Plan (Public Release Gate)

Status: Active
Owner: Pulse
Date: 2026-02-09

Progress tracker:
- `docs/architecture/release-final-certification-progress-2026-02.md`

Dependencies (must be complete first):
- `docs/architecture/release-security-gate-progress-2026-02.md`
- `docs/architecture/release-regression-bug-sweep-progress-2026-02.md`
- `docs/architecture/release-documentation-readiness-progress-2026-02.md`

## Intent

This lane is the final go/no-go certification gate for public release. It ratifies security, quality, and documentation evidence into one release decision.

Primary outcomes:
1. All pre-release gate lanes are complete and approved.
2. Full certification baseline is rerun with explicit exit codes.
3. Residual risks and launch conditions are explicit.
4. Final recommendation is recorded: `GO`, `GO_WITH_CONDITIONS`, or `NO_GO`.

## Non-Negotiable Contracts

1. RFC-01 cannot start until SEC/RGS/DOC lanes are complete.
2. `P0` unresolved items in any dependency lane force `NO_GO`.
3. Evidence must include independent reviewer reruns.
4. No summary-only certification claims.

## Certification Baseline Commands

1. `go build ./...`
2. `go test ./...`
3. `cd frontend-modern && npx vitest run`
4. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
5. `go test ./internal/api/... -run "Security|Tenant|RBAC|Contract|RouteInventory" -count=1`
6. `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation|TrueNAS" -count=1`

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

### RFC-00: Dependency Reconciliation and Scope Freeze

Objective:
- Verify dependency lane completion and freeze final certification scope.

Scope:
- `docs/architecture/release-final-certification-plan-2026-02.md`
- `docs/architecture/release-final-certification-progress-2026-02.md`

Checklist:
1. Verify SEC/RGS/DOC lane statuses and verdicts.
2. Record unresolved findings (if any) and disposition.
3. Freeze final certification command set.

### RFC-01: Full-System Certification Replay

Objective:
- Execute the full release certification baseline with explicit evidence.

Scope (max 4 files):
- `docs/architecture/release-final-certification-progress-2026-02.md`
- Minimal test-only fixes if command gates expose regressions

Checklist:
1. Run all certification baseline commands.
2. Capture command output and exit codes.
3. Fix scoped blockers or classify as blocking residuals.

### RFC-02: Go/No-Go Decision and Rollback Ratification

Objective:
- Produce final public release recommendation and rollback confidence statement.

Scope:
- `docs/architecture/release-final-certification-progress-2026-02.md`
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (only if final posture update required)

Checklist:
1. Verify RFC-00 and RFC-01 are `DONE/APPROVED`.
2. Record final verdict and launch conditions.
3. Record rollback trigger and operator actions.

Exit criteria:
- Final certification verdict is evidence-backed and explicit.
