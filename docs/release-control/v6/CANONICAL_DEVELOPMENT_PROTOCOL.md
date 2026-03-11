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
4. `docs/release-control/v6/subsystems/registry.json`
5. the relevant subsystem contract in `docs/release-control/v6/subsystems/`

The first two files answer release priority and current lane state.
`SOURCE_OF_TRUTH.md` owns stable governance, scope, and locked decisions.
`status.json` owns live lane state, structured evidence references, and open
operational decisions.
The protocol, subsystem registry, and subsystem contracts answer how work must
be done.

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

The machine-readable ownership map for those subsystem contracts lives in
`docs/release-control/v6/subsystems/registry.json`.

## Task Completion Protocol

Every substantial task must finish by checking these questions:

1. Did I change a canonical contract?
   If yes: update the relevant subsystem contract.
2. Did I change live lane state, evidence references, or open operational decisions?
   If yes: update `status.json`.
3. Did I change stable governance, scope, or lock a new architectural/release decision?
   If yes: update `SOURCE_OF_TRUTH.md`.
4. Did I replace an old path with a new canonical path?
   If yes: add or tighten a guardrail so the old path cannot silently return.
5. Did I add a new extension point?
   If yes: record exactly where future work must attach to it.
6. Did I leave compatibility code in runtime paths?
   If yes: either remove it now or explicitly classify it as a boundary-only exception.

This is the minimum update set for canonical work:

1. implementation
2. verification
3. contract update
4. guardrail/ratchet update when an old path is retired

This protocol is enforced locally at commit time by the canonical completion
guard in `scripts/release_control/canonical_completion_guard.py` and by the
governance guardrail tests in `internal/repoctl`.

For runtime subsystem changes, the same commit must now include:

1. the matching subsystem contract update
2. at least one matching verification artifact update

Verification artifacts are subsystem-specific. The allowed proof classes are
defined in `docs/release-control/v6/subsystems/registry.json` and may include
explicit guardrail files, contract tests, benchmark/SLO/query-plan artifacts,
approved test-prefix matches, non-test contract/type files, or same-subsystem
tests only when the registry explicitly allows them.

`status.json` evidence references must use repo-qualified relative paths.
Absolute machine-local paths are forbidden.

When a subsystem defines ordered `path_policies`, each touched runtime file must
satisfy the first matching proof policy for that file. Files that match no
explicit path policy fall back to the subsystem default verification policy.
Subsystems can ratchet further by requiring explicit path-policy coverage for
all owned runtime files, eliminating default fallback for that subsystem.

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
subsystem contract updates and proof-of-change verification updates.
That enforcement must run both in local hooks and in CI; bypassable local-only
governance is not sufficient.

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
