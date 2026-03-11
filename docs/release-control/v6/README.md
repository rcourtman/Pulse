# Pulse v6 Release Control

This folder is the canonical execution control layer for Pulse v6.

## Use Only These Two Files

1. `SOURCE_OF_TRUTH.md` (canonical human source)
2. `status.json` (canonical machine state)

Supporting governance file:

- `CONSOLIDATION_MAP.md` (legacy-doc demotion and archival map)
- `RETIREMENT_AUDIT_2026-02-27.md` (file-by-file audited retirement decisions)
- `CANONICAL_DEVELOPMENT_PROTOCOL.md` (canonical subsystem development protocol)
- `subsystems/*.md` (per-subsystem contracts: truth, extension points, forbidden paths, completion obligations)

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
