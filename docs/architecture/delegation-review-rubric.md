# Delegation Review Rubric

Status: Active
Last Updated: 2026-02-08

## Purpose

This rubric defines how orchestration and review must run when delegating implementation packets to Codex.

## Core Rules

1. Fail closed.
- No packet is `APPROVED` unless every required gate passes.

2. Evidence over summaries.
- Implementer summaries are never sufficient on their own.
- Reviewer must inspect changed files and rerun required commands.

3. Exit-code discipline.
- Every required command must include explicit exit code evidence.
- Timeout, empty output, truncated output, or missing exit code is an automatic failed gate.

4. Packet completeness.
- Packet can be marked `DONE` only after all checklist items are checked and verdict is `APPROVED`.

5. Checkpoint commits are mandatory.
- After each approved packet, reviewer/orchestrator must create a checkpoint commit.
- Packet evidence must include the commit hash.
- Next packet must not begin until the checkpoint commit exists.

## Delegation Packet Sizing Protocol (Mandatory)

Do not delegate oversized packets.

Hard limits per packet:
- Max 1 change shape.
- Max 1 subsystem boundary crossed.
- Max 3-5 files touched target.
- Max 2 required validation commands in-packet.

Disallowed packet shape:
- Creating a new abstraction, rewiring integrations, and deleting legacy code in one packet.

Required sequencing for risky work:
1. Step A: discovery + scaffold.
2. Step B: integration/wiring.
3. Step C: tests + cleanup.
4. Step D: deletion/removal only after green build/tests prove no dependency remains.

Stall handling:
- If no concrete progress in ~10 minutes, stop and split into smaller packets.
- If the implementer returns partial output, convert remainder into explicit follow-on packets.

## Git Safety and Parallel Isolation (Mandatory)

1. One orchestrator must map to one branch/worktree.
2. Never use destructive restore/reset commands on shared or unknown changes:
- `git checkout -- <path>`
- `git restore --source ...`
- `git reset --hard`
- `git clean -fd`
3. Stage and commit only packet-scoped files with explicit path lists.
4. If unrelated modified files are detected, do not revert them; proceed with packet-scoped staging only.
5. Any packet approved without a checkpoint commit hash is automatically `CHANGES_REQUESTED`.

## Review Gates

### P0: Execution Integrity
Required:
- Claimed files exist and contain the expected edits.
- Required commands rerun by reviewer.
- All required commands show exit code 0.

Fail conditions:
- Missing file verification.
- Missing reruns.
- Missing/invalid exit-code evidence.

### P1: Behavioral Correctness / Regression Risk
Required:
- High-risk behavior paths validated (authz, isolation, contracts, routing, state transitions as applicable).
- Tests cover changed behavior and meaningful negative paths.

Fail conditions:
- Critical paths untested.
- Isolation or contract risk not explicitly validated.

### P2: Plan/Tracker Compliance
Required:
- Progress tracker updated accurately.
- Packet status matches evidence.
- Residual risk and rollback documented.

Fail conditions:
- Tracker drift.
- `DONE` status without complete checklist/gates.

## Mandatory Review Output Template

```markdown
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
- `<short-hash>` (<message>)

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Approval and Reopen Policy

1. If any gate fails: `CHANGES_REQUESTED`.
2. If blocked by missing context or environment: `BLOCKED`.
3. Reopen any previously approved packet if later evidence invalidates prior gate assumptions.
