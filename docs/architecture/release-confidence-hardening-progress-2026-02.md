# Release Confidence Hardening Progress Tracker

Linked plan:
- `docs/architecture/release-confidence-hardening-plan-2026-02.md`

Status: Active
Date: 2026-02-09

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Every packet must include exact commands and explicit exit code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or replaced by summary-only claims.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. Update this file first in each implementation session and last before session end.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before starting the next packet.
7. Do not use destructive git commands on shared worktrees.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| RC-00 | Scope Freeze + Evidence Discipline | DONE | Claude | Claude | APPROVED | RC-00 section |
| RC-01 | Toolchain Pinning (Go stdlib vuln clear) | READY |  |  |  | RC-01 section |
| RC-02 | Security Re-Scan + Verdict Upgrade | BLOCKED |  |  |  | RC-02 section |
| RC-03 | Hosted Signup Partial Provisioning Cleanup | READY |  |  |  | RC-03 section |
| RC-04 | Frontend Release-Test Hygiene (No network noise) | READY |  |  |  | RC-04 section |
| RC-05 | Full Certification Replay | BLOCKED |  |  |  | RC-05 section |
| RC-06 | Release Artifact + Docker Validation | BLOCKED |  |  |  | RC-06 section |
| RC-07 | Final GO Verdict + Docs Alignment | BLOCKED |  |  |  | RC-07 section |

---

## RC-00 Checklist: Scope Freeze + Evidence Discipline

- [x] Confirm packet dependency graph is correct.
- [x] Confirm frozen command set matches current reality:
- [x] `go build ./...` -> exit 0
- [x] `go test ./... -count=1` -> exit 0 (all packages)
- [x] `cd frontend-modern && npx vitest run` -> exit 0 (80 files, 707 tests)
- [x] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [x] `go test ./internal/api/... -run "Security|Tenant|RBAC|Contract|RouteInventory" -count=1` -> exit 0 (11.3s)
- [x] `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation|TrueNAS" -count=1` -> exit 0 (2.9s, rerun; first attempt saw known timing flake per RFC-01)
- [x] `bash scripts/conformance-smoke.sh` -> exit 0 (6/6 permutations PASS)
- [x] Confirm evidence requirements for each packet are explicit in the plan doc.
- [x] Record verdict: APPROVED/BLOCKED with reasoning.

### Dependency Graph Verification

The plan's dependency graph is confirmed correct:
- RC-00 blocks RC-01, RC-03, RC-04 (independent after scope freeze)
- RC-01 blocks RC-02 (security scan requires pinned toolchain)
- RC-01 + RC-02 + RC-03 + RC-04 all block RC-05 (full replay)
- RC-05 blocks RC-06 (artifact validation)
- RC-06 blocks RC-07 (final verdict)

### Additional Baseline Scans (verified alongside frozen command set)

- `go env GOVERSION` -> `go1.25.7`
- `gitleaks detect --no-git --source .` -> exit 0 (no leaks)
- `govulncheck ./...` -> exit 0 (no vulnerabilities)
- `cd frontend-modern && npm audit --omit=dev` -> exit 0 (0 vulnerabilities)

### Evidence Requirements Audit

Each packet (RC-01 through RC-07) has explicit evidence requirements in the plan doc:
- RC-01: commands + exit codes for go env, go build, go test; diff proof of old version removal
- RC-02: govulncheck, gitleaks, npm audit exit codes; security tracker update
- RC-03: test names, go test exit code
- RC-04: vitest, tsc exit codes
- RC-05: all global commands
- RC-06: validate-release.sh, docker build targets
- RC-07: docs updates with evidence links

### Cross-Lane Observation

The SEC lane (already `GO`) has completed the implementation for RC-01 (Go 1.25.7 toolchain upgrade) and RC-03 (signup cleanup). This work exists in the working tree as uncommitted changes. RC-01 and RC-03 will verify, capture evidence, and checkpoint this existing work.

### Review Gates

- [x] P0 PASS — All 7 frozen commands verified with exit 0. Dependency graph validated.
- [x] P1 PASS — N/A for docs-only scope freeze packet.
- [x] P2 PASS — Tracker updated; plan marked Active; evidence requirements audited.
- [x] Verdict recorded

### RC-00 Review Record

```
Files changed:
- docs/architecture/release-confidence-hardening-progress-2026-02.md: scope freeze, command verification, dependency graph validation
- docs/architecture/release-confidence-hardening-plan-2026-02.md: status changed from Draft to Active

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./... -count=1` -> exit 0
3. `cd frontend-modern && npx vitest run` -> exit 0 (80 files, 707 tests)
4. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
5. `go test ./internal/api/... -run "Security|Tenant|RBAC|Contract|RouteInventory" -count=1` -> exit 0 (11.3s)
6. `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation|TrueNAS" -count=1` -> exit 0 (2.9s)
7. `bash scripts/conformance-smoke.sh` -> exit 0 (6/6 PASS)
8. `gitleaks detect --no-git --source .` -> exit 0
9. `govulncheck ./...` -> exit 0
10. `cd frontend-modern && npm audit --omit=dev` -> exit 0

Gate checklist:
- P0: PASS (all frozen commands verified with exit 0)
- P1: N/A (docs-only packet)
- P2: PASS (tracker accurate, plan active)

Verdict: APPROVED

Commit:
- (pending checkpoint)

Residual risk:
- Known timing flake in TestTrueNASPollerRecordsMetrics (P2, documented in RFC-01). Passes on rerun and in full suite.

Rollback:
- Revert checkpoint commit.
```

Evidence:
- Commands run + exit codes: see review record above (11 commands, all exit 0)
- Notes: SEC lane work for RC-01 and RC-03 already present in working tree. Plan status updated to Active.
- Commit: (pending)

## RC-01 Checklist: Toolchain Pinning (Go stdlib vuln clear)

Blocked by:
- RC-00

- [ ] Identify target Go toolchain patch version (must clear reachable stdlib findings).
- [ ] Update `go.mod` toolchain pin.
- [ ] Update Docker builder image pin.
- [ ] Update CI workflow Go versions.
- [ ] Update installer/build scripts that download or assume Go versions.
- [ ] Update devcontainer Go image pin.
- [ ] `go env GOVERSION` -> expected version.
- [ ] `go build ./...` -> exit 0
- [ ] `go test ./... -count=1` -> exit 0

Evidence:
- Commands run + exit codes:
- Search proof (old version references removed):
- Commit:

## RC-02 Checklist: Security Re-Scan + Verdict Upgrade

Blocked by:
- RC-01

- [ ] `gitleaks detect --no-git --source .` -> exit 0
- [ ] `govulncheck ./...` -> exit 0
- [ ] `cd frontend-modern && npm audit --omit=dev` -> exit 0
- [ ] Update security gate tracker with rerun evidence.
- [ ] If all conditions are resolved, record updated security verdict `GO`.

Evidence:
- Commands run + exit codes:
- Commit:

## RC-03 Checklist: Hosted Signup Partial Provisioning Cleanup

Blocked by:
- RC-00

- [ ] Identify all failure points after tenant directory initialization in `/api/public/signup`.
- [ ] Implement best-effort cleanup:
- [ ] Org directory removal.
- [ ] RBAC manager cleanup (close/remove cache).
- [ ] Add regression tests forcing RBAC failure and asserting cleanup.
- [ ] `go test ./internal/api/... -run "HostedSignup" -count=1` -> exit 0

Evidence:
- Commands run + exit codes:
- Test names added:
- Commit:

## RC-04 Checklist: Frontend Release-Test Hygiene (No network noise)

Blocked by:
- RC-00

- [ ] Identify test-time network calls and noisy warnings.
- [ ] Gate or mock network bootstraps in test mode.
- [ ] `cd frontend-modern && npx vitest run` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

Evidence:
- Commands run + exit codes:
- Commit:

## RC-05 Checklist: Full Certification Replay

Blocked by:
- RC-01
- RC-02
- RC-03
- RC-04

- [ ] `go build ./...` -> exit 0
- [ ] `go test ./... -count=1` -> exit 0
- [ ] `cd frontend-modern && npx vitest run` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [ ] `go test ./internal/api/... -run "Security|Tenant|RBAC|Contract|RouteInventory" -count=1` -> exit 0
- [ ] `go test ./internal/monitoring/... -run "Tenant|Alert|Isolation|TrueNAS" -count=1` -> exit 0
- [ ] `bash scripts/conformance-smoke.sh` -> exit 0

Evidence:
- Commands run + exit codes:
- Flake notes (if any):
- Commit:

## RC-06 Checklist: Release Artifact + Docker Validation

Blocked by:
- RC-05

- [ ] Build release artifacts (local or CI) and capture logs.
- [ ] Run `scripts/validate-release.sh <version> --skip-docker` -> exit 0
- [ ] If Docker available: run full validation (without `--skip-docker`) -> exit 0
- [ ] Verify Docker build targets succeed:
- [ ] `docker build --target runtime .`
- [ ] `docker build --target agent_runtime .`

Evidence:
- Commands run + exit codes:
- Artifact list:
- Commit:

## RC-07 Checklist: Final GO Verdict + Docs Alignment

Blocked by:
- RC-06

- [ ] Ensure security gate lane is `GO` with evidence.
- [ ] Ensure final certification lane is `GO` with evidence.
- [ ] Update final certification tracker to remove obsolete conditions.
- [ ] Confirm residual risks are accurately recorded with owners and dates.

Evidence:
- Files updated:
- Commands run + exit codes:
- Commit:

