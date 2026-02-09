# Release Confidence Hardening Plan (Packetized Execution Spec)

Status: Active
Owner: Release Orchestrator
Date: 2026-02-09

Progress tracker:
- `docs/architecture/release-confidence-hardening-progress-2026-02.md`

## Product Intent

Make the next major release the most reliable, security-resilient, and operationally controllable release to date, with evidence-backed confidence.

This plan is explicitly designed for parallel execution by multiple workers with strict evidence requirements and bounded packet scopes.

## Non-Negotiable Contracts

1. Evidence contract:
- Every packet must include exact commands and explicit exit codes.
- Summaries without rerunnable commands are invalid for `DONE`.
- If output is truncated, packet is not `DONE` unless exit codes are recorded and artifacts/logs are preserved.

2. Scope contract:
- Packets are narrow and reversible.
- No opportunistic refactors outside a packet’s declared goal.

3. Safety contract:
- No weakening of auth, tenant isolation, RBAC, or websocket isolation paths.
- No “fail open” changes without an explicit threat model and rollback plan.

4. Release readiness contract:
- Security gate must be `GO` (no open P0/P1).
- Final certification must be `GO` with a reproducible validation script or command set.

## How To Use This Plan

1. Pick the next `READY` packet from the progress tracker.
2. Create a branch for the packet.
3. Execute only the work in the packet scope.
4. Capture evidence (commands + exit codes).
5. Open review with links to evidence and the updated progress tracker.

## Packet Dependency Graph

```
RC-00 (scope freeze) ──blocks──► RC-01 (toolchain pin + CI)
RC-01 ──blocks──► RC-02 (security re-scan: govulncheck)

RC-00 ──blocks──► RC-03 (hosted signup cleanup hardening)
RC-00 ──blocks──► RC-04 (frontend test noise + invariants)

RC-01 + RC-02 + RC-03 + RC-04 ──all block──► RC-05 (full certification replay)
RC-05 ──blocks──► RC-06 (artifact + docker validation)
RC-06 ──blocks──► RC-07 (final GO verdict + docs update)
```

## Global Command Set (Used By Multiple Packets)

Backend:
- `go build ./...`
- `go test ./... -count=1`
- `go test ./internal/api/... -run "Security|Tenant|RBAC|Contract|RouteInventory" -count=1`
- `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation|TrueNAS" -count=1`

Frontend:
- `cd frontend-modern && npx vitest run`
- `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Conformance harness:
- `bash scripts/conformance-smoke.sh`

Security scans:
- `gitleaks detect --no-git --source .`
- `govulncheck ./...`
- `cd frontend-modern && npm audit --omit=dev`

## Packet RC-00: Scope Freeze + Roles + Evidence Discipline

Goal:
- Freeze the exact validation command set and the packet dependency rules.
- Assign implementer/reviewer expectations for evidence.

In scope:
- Progress tracker creation and initial packet board.
- Document the exact commands and success criteria.

Out of scope:
- Any code change.

Exit criteria:
- Progress tracker exists with packet board and checklists.
- This plan is marked `Active` only after RC-00 is approved.

Rollback:
- Revert the plan/progress doc commits.

## Packet RC-01: Toolchain Pinning to Clear Known Go Stdlib Vulns

Goal:
- Pin Go toolchain to the vetted patch version that clears reachable stdlib findings.

In scope:
- `go.mod` `toolchain` directive and `go` version line (if needed).
- Docker builder image version pin.
- CI workflow Go versions.
- Installer/build scripts that fetch or assume Go versions.
- Devcontainer toolchain pin.

Out of scope:
- Broad dependency bumps unrelated to the toolchain.

Required checks:
- `go env GOVERSION` matches the pinned version.
- `go mod tidy` produces no unintended dependency drift.
- `go test ./... -count=1` passes.

Evidence required:
- Commands + exit codes for: `go env GOVERSION`, `go build ./...`, `go test ./... -count=1`.
- Diff proof that all references to the old toolchain are updated (search evidence is acceptable).

Rollback:
- Revert commits that change toolchain pins.

## Packet RC-02: Security Re-Scan and Verdict Upgrade

Goal:
- Rerun security scans after RC-01 and verify there are no reachable Go stdlib findings.

In scope:
- Install `govulncheck` in the CI/dev environment if missing.
- Rerun `govulncheck ./...` and capture exit code.
- Rerun `gitleaks` and `npm audit --omit=dev`.
- Update the relevant security tracker sections and record an explicit updated verdict if conditions are fully resolved.

Out of scope:
- Fixing new vulnerabilities by ad hoc edits. If found, open a new packet.

Exit criteria:
- `govulncheck ./...` exits 0.
- `gitleaks` exits 0 (0 leaks).
- `npm audit --omit=dev` exits 0 (0 production vulns).

Rollback:
- Revert docs-only changes if evidence is incorrect.

## Packet RC-03: Hosted Signup Partial Provisioning Cleanup

Goal:
- Ensure `/api/public/signup` cannot leave a partially provisioned org directory or open RBAC DB handles on failure paths.

In scope:
- Implement best-effort cleanup on failure after tenant dir initialization.
- Ensure RBAC manager cache is removed/closed during cleanup.
- Add regression tests that force RBAC failure and assert cleanup.

Out of scope:
- Changes to auth policies, rate limiting policies, or signup UX.

Exit criteria:
- New tests cover failure cleanup.
- `go test ./internal/api/... -run "HostedSignup" -count=1` passes.

Rollback:
- Revert handler and tests.

## Packet RC-04: Frontend Release-Test Hygiene (No Network Noise)

Goal:
- Make frontend baseline tests deterministic and quiet (no implicit network calls, no noisy warnings).

In scope:
- Remove or gate any code paths that attempt real network calls during unit tests.
- If test scaffolding is missing, add it in a minimal, reusable way.

Out of scope:
- Visual/UX changes.

Exit criteria:
- `cd frontend-modern && npx vitest run` passes with no unexpected warnings.
- `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` exits 0.

Rollback:
- Revert frontend changes.

## Packet RC-05: Full Certification Replay (Single Script or Frozen Set)

Goal:
- Execute the frozen certification command set end-to-end and record evidence.

In scope:
- Run the Global Command Set.
- Investigate and fix any true regressions revealed by replay (in a follow-up packet if scope expands).

Exit criteria:
- All commands exit 0.
- If flakes occur, rerun until resolved or quarantined with explicit reasoning and owner.

Rollback:
- N/A (evidence packet; fixes are reverted separately).

## Packet RC-06: Release Artifact + Docker Validation

Goal:
- Verify release artifacts are complete and runnable, and Docker builds succeed with pinned toolchain.

In scope:
- Build release artifacts (local, or CI job) and run `scripts/validate-release.sh`.
- Verify Docker build targets: runtime and agent_runtime.

Exit criteria:
- `scripts/validate-release.sh <version> --skip-docker` passes at minimum.
- If Docker is available, run full validation including download endpoint smoke tests.

Rollback:
- Revert any changes that modify packaging outputs unexpectedly.

## Packet RC-07: Final GO Verdict + Documentation Alignment

Goal:
- Upgrade any remaining `GO_WITH_CONDITIONS` lanes to `GO` once conditions are truly resolved.
- Ensure `release-final-certification-progress-2026-02.md` reflects the updated posture.

In scope:
- Update verdict sections and residual risk tables to match reality.
- Ensure all referenced commands are rerunnable and match the evidence.

Exit criteria:
- Security gate: `GO`.
- Final certification: `GO`.
- Residual risks are P2-only (or explicitly accepted with owners and dates).

Rollback:
- Revert docs-only changes.

