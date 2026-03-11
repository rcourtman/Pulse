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
3. `docs/release-control/v6/status.schema.json`
4. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
5. `docs/release-control/v6/subsystems/registry.json`
6. `docs/release-control/v6/subsystems/registry.schema.json`
7. the relevant subsystem contract in `docs/release-control/v6/subsystems/`
8. `scripts/release_control/subsystem_lookup.py` when ownership or proof routing is not obvious

The first two files answer release priority and current lane state.
`SOURCE_OF_TRUTH.md` owns stable governance, scope, and locked decisions.
`status.json` owns live lane state, lane-to-subsystem ownership, structured
evidence references, and typed lane/subsystem decision records.
`status.schema.json` owns the machine-readable status contract.
`subsystems/registry.schema.json` owns the machine-readable subsystem registry
contract, including explicit shared-runtime ownership declarations for
intentionally multi-owner files.
The protocol, subsystem registry, and subsystem contracts answer how work must
be done.

`scripts/release_control/status_audit.py --check` is the machine audit entry
point for validating live lane evidence references, typed decision records, and
derived evidence health. It also enforces canonical list ordering inside
`status.json` so repo scope, lanes, evidence references, and decision timelines
do not drift into noisy, hand-arranged variants.
`scripts/release_control/registry_audit.py --check` is the machine audit entry
point for validating subsystem ownership, proof routing, registry lane
bindings, canonical ordering for unordered registry lists, and path-policy
reachability under first-match precedence.
`scripts/release_control/contract_audit.py --check` is the machine audit entry
point for validating structured contract metadata, section presence/order,
registry/status linkage, explicit cross-subsystem dependency declarations, and
canonical path references and shared-boundary declarations inside subsystem
contracts.

## Subsystem Contracts

Each major subsystem contract must define:

1. `Contract Metadata`
2. `Purpose`
3. `Canonical Files`
4. `Shared Boundaries`
5. `Extension Points`
6. `Forbidden Paths`
7. `Completion Obligations`
8. `Current State`

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
Each contract must also carry structured metadata that binds the markdown file
to its registry subsystem id, owning lane in `status.json`, and exact declared
cross-subsystem dependencies implied by its canonical-file and extension-point
references.
If a runtime file is intentionally owned by multiple subsystems, that overlap
must be declared explicitly in `registry.json` and mirrored in the contract's
`Shared Boundaries` section; accidental overlap is not an allowed registry
state. Shared-boundary entries must use the exact registry-derived sentence
shape rather than freeform wording.

## Task Completion Protocol

Every substantial task must finish by checking these questions:

1. Did I change a canonical contract?
   If yes: update the relevant subsystem contract.
2. Did I change live lane state, evidence references, or typed operational decision records?
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
Local pre-commit governance checks must evaluate staged v6 control-file content
rather than unstaged working-tree noise.
Local pre-commit must also reject partial staging for hook-sensitive governance
files under `docs/release-control/v6/`, `scripts/release_control/`,
`internal/repoctl/`, `.husky/pre-commit`, and
`.github/workflows/canonical-governance.yml`, because those local checks still
execute or structurally read the working-tree versions.
Local formatter steps must also stay scoped to staged files so the hook does not
mutate unrelated dirty worktree state.
When formatting staged Go files, the formatter must operate on staged blobs in
the git index rather than rewriting the whole repo or restaging whole files.

For runtime subsystem changes, the same commit must now include:

1. the matching subsystem contract update
2. any dependent subsystem contract whose `Canonical Files`, `Shared Boundaries`, or `Extension Points` explicitly reference a touched runtime path
3. at least one matching verification artifact update

A staged contract file only counts when the staged diff changes a substantive
contract section such as `Purpose`, `Canonical Files`, `Shared Boundaries`, `Extension Points`,
`Forbidden Paths`, `Completion Obligations`, or `Current State`. Metadata-only
edits are not sufficient completion proof for runtime changes.

Verification artifacts are subsystem-specific. The allowed proof classes are
defined in `docs/release-control/v6/subsystems/registry.json` and may include
explicit guardrail files, contract tests, benchmark/SLO/query-plan artifacts,
approved test-prefix matches, non-test contract/type files, or same-subsystem
tests only when the registry explicitly allows them.

Cross-subsystem contract dependencies are not advisory. If a touched runtime
path is named in another subsystem contract's `Canonical Files`,
`Shared Boundaries`, or `Extension Points`, the canonical completion guard now requires that dependent
contract to be staged in the same slice.

`status.json` evidence references must use repo-qualified relative paths.
Absolute machine-local paths are forbidden.

When a subsystem defines ordered `path_policies`, each touched runtime file must
satisfy the first matching proof policy for that file. The v6 registry requires
explicit path-policy coverage for every governed subsystem, and default
subsystem verification is no longer a supported governed path. New owned
runtime files must therefore be added to a concrete proof route instead of
inheriting subsystem-default verification.

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

Use `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...]`
to ask the repo which subsystem, contract, proof route, live lane context,
relevant decision records, and dependent contract-update obligations apply to a
file set before editing.

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
