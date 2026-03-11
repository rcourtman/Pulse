# Pulse v6 Canonical Development Protocol

This protocol exists to make the repo constrained instead of interpretive.
Future work should be difficult to do in the wrong shape.

## Core Rule

Each important concern in Pulse v6 must have:

1. one canonical truth
2. one obvious extension path
3. explicit forbidden paths
4. completion obligations when that truth changes
5. at least one executable guardrail where practical

If a subsystem does not satisfy those five properties, it is drift-prone.

## Required Operating Files

For v6 work, agents must treat these files as the execution entry point:

1. `docs/release-control/v6/SOURCE_OF_TRUTH.md`
2. `docs/release-control/v6/status.json`
3. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
4. the relevant subsystem contract in `docs/release-control/v6/subsystems/`

The first two files answer release priority and current lane state.
The protocol and subsystem contracts answer how work must be done.

## Subsystem Contracts

Each major subsystem contract must define:

1. `Purpose`
2. `Canonical Files`
3. `Extension Points`
4. `Forbidden Paths`
5. `Completion Obligations`
6. `Current State`

Current required subsystem contracts:

1. `docs/release-control/v6/subsystems/alerts.md`
2. `docs/release-control/v6/subsystems/monitoring.md`
3. `docs/release-control/v6/subsystems/unified-resources.md`
4. `docs/release-control/v6/subsystems/cloud-paid.md`
5. `docs/release-control/v6/subsystems/api-contracts.md`
6. `docs/release-control/v6/subsystems/frontend-primitives.md`
7. `docs/release-control/v6/subsystems/performance-and-scalability.md`

If a major subsystem is refactored and does not have one of these files, the
work is incomplete.

## Task Completion Protocol

Every substantial task must finish by checking these questions:

1. Did I change a canonical contract?
   If yes: update the relevant subsystem contract.
2. Did I change lane status or lock a new architectural decision?
   If yes: update `SOURCE_OF_TRUTH.md` and, when materially required, `status.json`.
3. Did I replace an old path with a new canonical path?
   If yes: add or tighten a guardrail so the old path cannot silently return.
4. Did I add a new extension point?
   If yes: record exactly where future work must attach to it.
5. Did I leave compatibility code in runtime paths?
   If yes: either remove it now or explicitly classify it as a boundary-only exception.

This is the minimum update set for canonical work:

1. implementation
2. verification
3. contract update
4. guardrail/ratchet update when an old path is retired

This protocol is enforced at commit time by the canonical completion guard in
`scripts/release_control/canonical_completion_guard.py`.

## Guardrails

Canonical architecture is not considered real until the repo can enforce it.

Preferred guardrail types:

1. code standards tests that ban deprecated paths
2. contract tests that lock payload/wire shapes
3. query-plan and performance tests for hot paths
4. parity tests during migrations
5. drift tests for generated or embedded artifacts

Documentation alone is not sufficient when a rule can be made executable.

The canonical completion guard is the default repo-level enforcement point for
subsystem contract updates.

## How To Extend Pulse

Before adding new behavior, the author must answer:

1. Which subsystem contract owns this change?
2. Which canonical files are allowed to carry the truth?
3. Which extension point should be used?
4. Which paths are forbidden for this kind of change?
5. Which tests or guardrails must change with it?

If those answers are not obvious in under a minute, the subsystem still needs
architectural hardening.

## Boundary Rule

Compatibility is allowed only at explicit boundaries:

1. migration loaders
2. persisted upgrade readers
3. temporary API translation layers with a removal plan

Compatibility is not allowed to become the runtime source of truth.

## Expected End State

Pulse v6 should be:

1. canonical by default
2. extension-oriented instead of patch-oriented
3. enforced by tests and guardrails
4. obvious for both humans and agents to continue correctly
