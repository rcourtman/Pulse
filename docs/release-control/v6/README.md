# Pulse v6 Release Control

This folder is the current active release profile under the evergreen Pulse
release control plane.

The evergreen control-plane files live one level up:

1. `docs/release-control/CONTROL_PLANE.md`
2. `docs/release-control/control_plane.json`
3. `docs/release-control/control_plane.schema.json`

## Start With These Control Files

1. `../CONTROL_PLANE.md` (evergreen governance, active-profile selection, and active-target rules)
2. `../control_plane.json` (machine-readable active profile and active target)
3. `SOURCE_OF_TRUTH.md` (stable human governance, readiness-assertion design rules, and locked decisions)
4. `status.json` (live lane state, readiness derivation rules, the active readiness assertion catalog, lane-to-subsystem ownership, structured evidence references, typed lane/subsystem decision records, and canonical ordered lists)
5. `status.schema.json` (machine-readable contract for the `status.json` shape)
6. `subsystems/registry.json` (machine-readable subsystem ownership, explicit shared-ownership exceptions, and proof routing)
7. `subsystems/registry.schema.json` (machine-readable contract for the subsystem registry shape, shared-ownership declarations, and unordered-list uniqueness)

`status.json` reporting every lane as `target-met` means the tracked v6
repo-hardening work is at target. It does not, by itself, mean Pulse v6 is release-approved while `open_decisions` or `release_gates` remain unresolved.
Use `python3 scripts/release_control/status_audit.py --pretty` for the current derived repo/release readiness summary.
Use `status.json.readiness_assertions` for the active required assertion set, its proof references, and any executable `proof_commands` that readiness assertion guardrails run.
Use `docs/release-control/control_plane.json` for the active profile and active
target that sit above this profile.
Use `python3 scripts/release_control/control_plane_audit.py --check` to enforce
that the active target stays current and does not remain active after its
completion rule is already satisfied.

Supporting governance file:

- `CONSOLIDATION_MAP.md` (legacy-doc demotion and archival map)
- `RETIREMENT_AUDIT_2026-02-27.md` (file-by-file audited retirement decisions)
- `CANONICAL_DEVELOPMENT_PROTOCOL.md` (canonical subsystem development protocol)
- `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` (manual verification runbook for trust-critical release gates)
- `subsystems/*.md` (per-subsystem contracts: truth, shared boundaries, extension points, forbidden paths, completion obligations)

Useful helper tools:

- `python3 scripts/release_control/contract_audit.py --check` for structured subsystem contract metadata, explicit cross-subsystem dependency declarations, shared-boundary declarations, required sections, and canonical path references
  Shared-boundary entries must match the exact registry-derived sentence shape, not freeform prose.
  Local pre-commit runs the v6 machine audits with staged control-file content so validation is based on the actual index content being committed.
  Local pre-commit also blocks partial staging for hook-sensitive governance files under `docs/release-control/v6/`, `scripts/release_control/`, `internal/repoctl/`, `.husky/pre-commit`, and `.github/workflows/canonical-governance.yml`, because those checks still execute or structurally read working-tree content locally.
- `python3 scripts/release_control/status_audit.py --check`
  Validates lane evidence, readiness assertions, release gates, decision records, and derived repo/release readiness.
- `python3 scripts/release_control/readiness_assertion_guard.py --blocking-level repo-ready --proof-type automated`
  Runs the machine-declared proof commands for automated repo-ready assertions straight from `status.json.readiness_assertions`.
- `python3 scripts/release_control/readiness_assertion_guard.py --proof-type hybrid`
  Runs any machine-declared executable proof commands attached to hybrid release assertions, without replacing the linked manual release gates.
- `python3 scripts/release_control/registry_audit.py --check`
- `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...]` for subsystem ownership, proof routing, lane context, relevant decision records, and dependent contract-update obligations

For governed runtime changes, the completion guard also requires any staged
contract update to touch a substantive contract section such as `Purpose`,
`Canonical Files`, `Shared Boundaries`, `Extension Points`, `Forbidden Paths`,
`Completion Obligations`, or `Current State`, not just metadata.
Local pre-commit formatting is intentionally scoped to staged files so unrelated
dirty worktree files are not mutated during commit.
The staged Go formatter updates the git index directly and avoids broad
restaging, so partially staged files do not silently absorb unrelated hunks.
Local pre-commit also blocks any unstaged edits to hook-sensitive governance
files, so working-tree-only governance changes cannot make local validation
disagree with the committed tree.

The old release-control orchestrator and loop tooling are retired. Direct,
repo-aware sessions are now the only supported control-plane execution path.
Post-release work should keep using this same profile until the control plane
promotes a new active target such as stabilization, polish, or a named feature
initiative.

## Active Repo Scope

`status.json.scope.control_plane_repo` is `pulse`, and
`status.json.scope.repo_catalog` is the canonical machine-readable repo map for
the active workspace.

- `pulse`: core desktop/runtime repo and canonical v6 release-control authority
- `pulse-pro`: commercial operations, checkout, license-server, and relay-server
- `pulse-enterprise`: closed-source enterprise and paid runtime features
- `pulse-mobile`: mobile client, relay pairing, approvals, and local auth/state

Ignored for v6 control:

- `pulse-5.1.x`
- `pulse-refactor-streams`

## Lean-Mode Rule

For v6 execution, agents must read `../CONTROL_PLANE.md` and
`../control_plane.json` first, then `SOURCE_OF_TRUTH.md` and `status.json`.
Open other docs only when direct evidence is needed for the active task.

For canonical subsystem work, the next file to open after those two is
`CANONICAL_DEVELOPMENT_PROTOCOL.md`, followed by the relevant file under
`subsystems/`.
