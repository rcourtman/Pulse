# TrueNAS Rollout Readiness Progress Tracker

Linked plan:
- `docs/architecture/truenas-rollout-readiness-plan-2026-02.md` (authoritative execution spec)

Upstream lane:
- `docs/architecture/truenas-ga-progress-2026-02.md` (TN-00..TN-11, COMPLETE, GO verdict)

Status: Active
Date: 2026-02-09

## Rules

1. A packet can only move to `DONE` when every checkbox in that packet is checked.
2. Reviewer must provide explicit command exit-code evidence.
3. `DONE` is invalid if command output is timed out, missing, truncated without exit code, or replaced by summary-only claims.
4. If review fails, set status to `CHANGES_REQUESTED`, add findings, and keep checkboxes open.
5. Update this file first in each implementation session and last before session end.
6. After every `APPROVED` packet, create a checkpoint commit and record the hash in packet evidence before starting the next packet.
7. Do not use `git checkout --`, `git restore --source`, `git reset --hard`, or `git clean -fd` on shared worktrees.
8. Respect packet subsystem boundaries; do not expand packet scope to adjacent streams.
9. No new `/api/resources` dependencies, `useResourcesAsLegacy` usage, or `LegacyResource` contract dependencies.
10. `PULSE_ENABLE_TRUENAS` default behavior unchanged until TRR-05 verdict.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| TRR-00 | Scope Freeze + TN-11 Evidence Reconciliation | DONE | Claude | Claude | APPROVED | TRR-00 Review Evidence |
| TRR-01 | Operational Runbook (Enable/Disable/Rollback) | DONE | Codex | Claude | APPROVED | TRR-01 Review Evidence |
| TRR-02 | Telemetry + Alert Thresholds | DONE | Codex | Claude | APPROVED | TRR-02 Review Evidence |
| TRR-03 | Canary Rollout Controls | DONE | Codex | Claude | APPROVED | TRR-03 Review Evidence |
| TRR-04 | Soak/Failure-Injection Validation | DONE | Codex | Claude | APPROVED | TRR-04 Review Evidence |
| TRR-05 | Final Rollout Verdict | TODO | Claude | Claude | — | — |

---

## TRR-00 Checklist: Scope Freeze + TN-11 Evidence Reconciliation

- [x] TN-11 GO verdict verified with all 12 checkpoint commits.
- [x] Existing telemetry infrastructure audited for reuse.
- [x] Packet boundaries and dependency gates frozen.
- [x] Definition-of-done contracts recorded.

### Required Tests

- [x] `go build ./...` -> exit 0
- [x] `go test ./internal/truenas/... -count=1` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### TRR-00 Review Evidence

```markdown
Files changed:
- docs/architecture/truenas-rollout-readiness-plan-2026-02.md: Created authoritative execution spec (6 packets, dependency graph, risk register, infrastructure audit)
- docs/architecture/truenas-rollout-readiness-progress-2026-02.md: Created progress tracker with packet board, checklists, review evidence sections

Commands run + exit codes:
1. `git log --oneline` (verified all 12 TN checkpoint commits TN-00..TN-11 exist) -> exit 0
2. `go build ./...` -> exit 0
3. `go test ./internal/truenas/... -count=1` -> exit 0 (0.309s, 1 package)

Gate checklist:
- P0: PASS (build clean, tests green, no compilation errors)
- P1: PASS (TN-11 GO verdict verified, all 12 checkpoint commits confirmed, infrastructure audit completed identifying PollMetrics/MonitorError/CircuitBreaker for reuse and 5 gaps)
- P2: PASS (plan and progress docs created, packet board initialized, dependency graph frozen, definition-of-done contracts recorded)

Verdict: APPROVED

Residual risk:
- None for scope freeze packet.

Rollback:
- Revert plan + progress doc commits.
```

---

## TRR-01 Checklist: Operational Runbook

- [x] Enable path documented with verification steps.
- [x] Disable path documented with staleness verification.
- [x] Kill-switch documented for immediate deactivation.
- [x] Code rollback documented with commit range.
- [x] Data cleanup documented (truenas.enc removal).
- [x] All lifecycle paths have explicit verification commands.

### Required Tests

- [x] `go build ./...` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### TRR-01 Review Evidence

```markdown
Files changed:
- docs/architecture/truenas-operational-runbook.md: Created operational runbook (5 lifecycle sections, verification commands, quick reference table)

Commands run + exit codes:
1. `go build ./...` -> exit 0

Gate checklist:
- P0: PASS (build clean, docs-only packet, no code changes)
- P1: PASS (all 5 lifecycle paths documented with verification: enable, disable, kill-switch, code rollback, data cleanup; reviewer corrected disable path verification — connections API is always registered regardless of feature flag, poller is the gated component)
- P2: PASS (runbook complete, verification steps explicit, quick reference table included)

Verdict: APPROVED

Reviewer corrections applied:
- Disable path: changed expected response from HTTP 404 to "no poll activity in logs" — trueNASHandlers is always initialized (router.go:267), only the poller checks IsFeatureEnabled()

Residual risk:
- None for docs-only packet.

Rollback:
- Delete truenas-operational-runbook.md.
```

---

## TRR-02 Checklist: Telemetry + Alert Thresholds

- [x] PollMetrics wired into TrueNAS poller (poll attempt, success, error, staleness).
- [x] TrueNAS API errors wrapped in MonitorError with correct ErrorType.
- [x] Test verifies metrics emitted on success and failure.
- [x] Alert thresholds documented in operational runbook.

### Required Tests

- [x] `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v` -> exit 0
- [x] `go build ./...` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### TRR-02 Review Evidence

```markdown
Files changed:
- internal/monitoring/truenas_poller.go: Wired getPollMetrics().RecordResult() into pollAll() for both success and error paths; added classifyTrueNASError() wrapping errors in MonitorError with type classification (api/timeout/auth/connection)
- internal/monitoring/truenas_poller_test.go: Added TestTrueNASPollerRecordsMetrics — server fails 3 cycles then recovers, exercising both error and success metrics paths
- docs/architecture/truenas-operational-runbook.md: Appended Alert Thresholds section with 7 metric/condition/severity/action rows

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v` -> exit 0 (5 tests PASS, 0.734s)
3. `go test ./internal/monitoring/... -count=1` -> exit 0 (18.662s)

Gate checklist:
- P0: PASS (build clean, all tests green including full monitoring suite)
- P1: PASS (PollMetrics wired with instance_type="truenas"; error classification covers timeout/auth/connection/api; test exercises failure→recovery path; existing tests unbroken)
- P2: PASS (alert thresholds documented in runbook; 3 files changed within scope)

Verdict: APPROVED

Residual risk:
- classifyTrueNASError uses string matching on error messages — if TrueNAS client error format changes, classification may degrade to "api" fallback (acceptable, no data loss)

Rollback:
- Revert truenas_poller.go to remove metrics wiring; revert test file; remove Alert Thresholds section from runbook.
```

---

## TRR-03 Checklist: Canary Rollout Controls

- [x] Phased rollout strategy documented (internal → opt-in → default-on).
- [x] Abort criteria defined with quantitative thresholds.
- [x] Rollback from each phase documented.
- [x] Monitoring checkpoints at each phase transition documented.

### Required Tests

- [x] `go build ./...` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### TRR-03 Review Evidence

```markdown
Files changed:
- docs/architecture/truenas-operational-runbook.md: Appended Canary Rollout Strategy section with 3 phases, abort criteria, and phase transition checklist

Commands run + exit codes:
1. `go build ./...` -> exit 0 (code unchanged, docs-only packet)

Gate checklist:
- P0: PASS (build clean, docs-only packet, no code changes)
- P1: PASS (3 phases documented: internal/dev 48h → opt-in 1 week → default-on permanent; 5 abort criteria with quantitative thresholds; rollback per phase; 5-item phase transition checklist; metric names reference actual Prometheus labels)
- P2: PASS (section inserted before Alert Thresholds as specified; runbook complete)

Verdict: APPROVED

Residual risk:
- Phase 3 (default-on) requires a code change to the feature flag default — deferred to TRR-05 rollout verdict.

Rollback:
- Remove Canary Rollout Strategy section from runbook.
```

---

## TRR-04 Checklist: Soak/Failure-Injection Validation

- [x] API timeout test: poller continues, metrics record timeout.
- [x] Auth failure test: poller logs error, no crash.
- [x] Stale data recovery test: consecutive failures → staleness → recovery.
- [x] Connection flap test: up → down → up → resources recover.
- [x] Concurrent config change test: no race/panic.

### Required Tests

- [x] `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v` -> exit 0
- [x] `go test ./internal/truenas/... -count=1 -v` -> exit 0
- [x] `go build ./...` -> exit 0

### Review Gates

- [x] P0 PASS
- [x] P1 PASS
- [x] P2 PASS
- [x] Verdict recorded: `APPROVED`

### TRR-04 Review Evidence

```markdown
Files changed:
- internal/monitoring/truenas_poller_test.go: Added 5 failure-injection tests + timeout injection helper

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v` -> exit 0 (10 tests: 5 existing + 5 new, 1.697s)
3. `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -race` -> exit 0 (no races, 2.810s)
4. `go test ./internal/truenas/... -count=1 -v` -> exit 0 (25 pass, 1 skip, 0.228s)

Gate checklist:
- P0: PASS (build clean, all tests green, race detector clean)
- P1: PASS (5 failure scenarios tested: API timeout with recovery, 401 auth failure resilience, stale data recovery cycle, connection flap via 503 toggle, concurrent config change with provider convergence; all use waitForCondition for determinism)
- P2: PASS (1 file changed within scope, all 5 checklist items satisfied)

Verdict: APPROVED

Residual risk:
- Timeout test injects client timeout via internal helper (accesses poller.providers directly) — acceptable for test code, not production path

Rollback:
- Revert failure-injection test additions from truenas_poller_test.go
```

---

## TRR-05 Checklist: Final Rollout Verdict

- [ ] TRR-00 through TRR-04 are all `DONE` and `APPROVED`.
- [ ] Full milestone validation commands rerun with explicit exit codes.
- [ ] Residual risks reconciled from all packets.
- [ ] Final rollout verdict recorded (`GO` / `GO_WITH_CONDITIONS` / `NO_GO`).

### Required Tests

- [ ] `go build ./... && go test ./internal/truenas/... ./internal/monitoring/... -count=1` -> exit 0
- [ ] `go test ./internal/api/... -run "TrueNAS" -count=1` -> exit 0
- [ ] `cd frontend-modern && npx vitest run` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TRR-05 Review Evidence

```markdown
TODO
```

---

## Checkpoint Commits

- TRR-00: `687ecd79`
- TRR-01: `64f6b350`
- TRR-02: `9d25e014`
- TRR-03: `14953c87`
- TRR-04: TODO
- TRR-05: TODO

## Current Recommended Next Packet

- `TRR-01` (Operational Runbook) + `TRR-02` (Telemetry) can run in parallel
