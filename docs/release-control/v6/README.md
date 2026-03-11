# Pulse v6 Release Control

This folder is the canonical execution control layer for Pulse v6.

## Start With These Control Files

1. `SOURCE_OF_TRUTH.md` (stable human governance and locked decisions)
2. `status.json` (live lane state, lane-to-subsystem ownership, structured evidence references, typed lane/subsystem decision records, and canonical ordered lists)
3. `status.schema.json` (machine-readable contract for the `status.json` shape)
4. `subsystems/registry.json` (machine-readable subsystem ownership, explicit shared-ownership exceptions, and proof routing)
5. `subsystems/registry.schema.json` (machine-readable contract for the subsystem registry shape, shared-ownership declarations, and unordered-list uniqueness)

Supporting governance file:

- `CONSOLIDATION_MAP.md` (legacy-doc demotion and archival map)
- `RETIREMENT_AUDIT_2026-02-27.md` (file-by-file audited retirement decisions)
- `CANONICAL_DEVELOPMENT_PROTOCOL.md` (canonical subsystem development protocol)
- `subsystems/*.md` (per-subsystem contracts: truth, shared boundaries, extension points, forbidden paths, completion obligations)

Useful helper tools:

- `python3 scripts/release_control/contract_audit.py --check` for structured subsystem contract metadata, explicit cross-subsystem dependency declarations, shared-boundary declarations, required sections, and canonical path references
  Shared-boundary entries must match the exact registry-derived sentence shape, not freeform prose.
  Local pre-commit runs the v6 machine audits with staged control-file content so validation is based on the actual index content being committed.
- `python3 scripts/release_control/status_audit.py --check`
- `python3 scripts/release_control/registry_audit.py --check`
- `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...]` for subsystem ownership, proof routing, lane context, relevant decision records, and dependent contract-update obligations

For governed runtime changes, the completion guard also requires any staged
contract update to touch a substantive contract section such as `Purpose`,
`Canonical Files`, `Shared Boundaries`, `Extension Points`, `Forbidden Paths`,
`Completion Obligations`, or `Current State`, not just metadata.
Local pre-commit formatting is intentionally scoped to staged files so unrelated
dirty worktree files are not mutated during commit.

The old release-control orchestrator and loop tooling are retired. Direct,
repo-aware sessions are now the only supported v6 execution path.

## Active Repo Scope

- `pulse`
- `pulse-pro`
- `pulse-enterprise`
- `pulse-mobile`

Ignored for v6 control:

- `pulse-5.1.x`
- `pulse-refactor-streams`

## Lean-Mode Rule

For v6 execution, agents must read only `SOURCE_OF_TRUTH.md` and `status.json` first.
Open other docs only when direct evidence is needed for the active task.

For canonical subsystem work, the next file to open after those two is
`CANONICAL_DEVELOPMENT_PROTOCOL.md`, followed by the relevant file under
`subsystems/`.
