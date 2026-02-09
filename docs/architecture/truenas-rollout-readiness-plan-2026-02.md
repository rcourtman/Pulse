# TrueNAS Rollout Readiness Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-09

Progress tracker:
- `docs/architecture/truenas-rollout-readiness-progress-2026-02.md`

Upstream lane:
- `docs/architecture/truenas-ga-plan-2026-02.md` (feature build, COMPLETE)
- `docs/architecture/truenas-ga-progress-2026-02.md` (TN-00..TN-11 DONE/APPROVED, GO verdict)

Related:
- `docs/architecture/program-closeout-certification-plan-2026-02.md` (program-level closeout)
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (W2 exit criteria)
- `docs/architecture/delegation-review-rubric.md` (orchestration protocol)

## Intent

The TrueNAS GA lane (TN-00..TN-11) certified the feature as code-complete with a GO verdict. This follow-on lane operationalizes the rollout: runbooks, telemetry, canary controls, failure injection, and a final rollout verdict.

Primary outcomes:
1. Operators have a tested enable/disable/rollback runbook for TrueNAS support.
2. Telemetry instruments poll health, API latency, and ingest freshness with alert thresholds.
3. Canary rollout controls exist (flag strategy, abort criteria) for staged activation.
4. Soak/failure-injection tests prove resilience under timeout, auth failure, stale data, and reconnect scenarios.
5. A final evidence-backed rollout verdict is issued (GO / GO_WITH_CONDITIONS / NO_GO).

## Definition of Done

This lane is complete only when all are true:
1. Operational runbook for TrueNAS enable/disable/rollback is written and verified.
2. TrueNAS poller emits Prometheus metrics via existing `PollMetrics` infrastructure.
3. Alert thresholds for poll failures and staleness are documented.
4. Canary rollout strategy is documented with abort criteria.
5. Failure-injection tests validate resilience for all error classes.
6. Final rollout verdict is evidence-backed.

## Non-Negotiable Contracts

1. `PULSE_ENABLE_TRUENAS` default behavior unchanged until TRR-05 verdict.
2. No new `/api/resources` dependencies.
3. No `useResourcesAsLegacy` usage.
4. No destructive git commands.
5. No modifications to active non-TrueNAS lane files.
6. Stage only packet-scoped files.

## Existing Infrastructure Audit

### Available for reuse (no new invention needed)

1. **PollMetrics singleton** (`internal/monitoring/metrics.go`):
   - `pulse_monitor_poll_duration_seconds` (Histogram) — labeled by `instance_type`, `instance`
   - `pulse_monitor_poll_total` (Counter) — with `result` label (success/error)
   - `pulse_monitor_poll_errors_total` (Counter) — with `error_type` label
   - `pulse_monitor_poll_last_success_timestamp` (Gauge)
   - `pulse_monitor_poll_staleness_seconds` (Gauge)

2. **MonitorError** (`internal/errors/errors.go`):
   - Typed errors: `connection`, `auth`, `validation`, `api`, `timeout`
   - Retryability classification
   - HTTP status code mapping

3. **Circuit breaker** (`internal/monitoring/circuit_breaker.go`):
   - Per-instance state machine (closed → open → half-open)
   - Exponential backoff
   - Prometheus metrics: `pulse_scheduler_breaker_state`, `failure_count`, `retry_seconds`

4. **HTTP API metrics** (`internal/api/http_metrics.go`):
   - `pulse_http_request_duration_seconds` — already covers `/api/truenas/*` routes
   - `pulse_http_requests_total`

### Gaps requiring implementation

1. TrueNAS poller does not call `PollMetrics` — needs wiring.
2. TrueNAS API errors are not wrapped in `MonitorError` — needs wrapping.
3. No per-connection circuit breaker in TrueNAS poller.
4. No canary/percentage-based rollout — only binary feature flag.
5. No operational runbook for TrueNAS enable/disable.

## Orchestrator Operating Model

- Implementer: Codex MCP
- Reviewer: Claude (orchestrator)
- Sandbox: `danger-full-access` for all Codex invocations
- Fail closed on all gates

## Required Review Output (Every Packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit <code>

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

## Execution Packets

### TRR-00: Scope Freeze + TN-11 Evidence Reconciliation

Objective:
- Freeze rollout lane scope, reconcile TN-11 evidence, verify dependency gates.

Scope:
- `docs/architecture/truenas-rollout-readiness-plan-2026-02.md` (this file)
- `docs/architecture/truenas-rollout-readiness-progress-2026-02.md`

Implementation checklist:
1. Verify TN-11 GO verdict and all 12 TN checkpoint commits exist.
2. Audit existing telemetry infrastructure for reuse.
3. Freeze packet boundaries and dependency gates.
4. Record TRR definition-of-done contracts.

Required tests:
1. `go build ./...`
2. `go test ./internal/truenas/... -count=1`

Exit criteria:
- Scope frozen, TN-11 evidence reconciled, plan and progress docs created.

### TRR-01: Operational Runbook (Enable/Disable/Kill-Switch/Rollback)

Objective:
- Create a tested operational runbook for TrueNAS rollout lifecycle.

Scope (docs-only, max 2 files):
1. `docs/architecture/truenas-operational-runbook.md` (new)
2. `docs/architecture/truenas-rollout-readiness-progress-2026-02.md` (update)

Implementation checklist:
1. Document enable path: env var → restart → verify resources appear.
2. Document disable path: unset env var → restart → verify resources expire (120s stale).
3. Document kill-switch: immediate deactivation without code rollback.
4. Document code rollback: revert commit range, verify clean build.
5. Document data cleanup: remove `truenas.enc`.
6. Include verification steps for each operation.

Required tests:
1. `go build ./...` (confirms code integrity)

Exit criteria:
- Runbook is complete, verification steps are explicit, and all lifecycle paths are documented.

### TRR-02: Telemetry + Alert Thresholds

Objective:
- Wire TrueNAS poller into existing PollMetrics infrastructure and define alert thresholds.

Scope (max 5 files, backend only):
1. `internal/monitoring/truenas_poller.go` (modify — add metrics calls)
2. `internal/monitoring/truenas_poller_test.go` (extend — verify metrics emitted)
3. `docs/architecture/truenas-operational-runbook.md` (append thresholds section)

Implementation checklist:
1. Wire `PollMetrics.RecordPollAttempt` / `RecordPollSuccess` / `RecordPollError` with `instance_type="truenas"` in `pollAll()`.
2. Wire `PollMetrics.RecordStaleness` for each connection.
3. Wrap TrueNAS API errors in `MonitorError` with correct `ErrorType` classification.
4. Add test verifying metrics are recorded on successful and failed polls.
5. Document alert thresholds in runbook:
   - Poll failure rate > 3 consecutive → WARNING
   - Staleness > 300s → WARNING, > 600s → CRITICAL
   - Auth errors → immediate operator notification

Required tests:
1. `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v`
2. `go build ./...`

Exit criteria:
- TrueNAS poller emits standard poll metrics; alert thresholds documented.

### TRR-03: Canary Rollout Controls

Objective:
- Document canary rollout strategy with flag controls and abort criteria.

Scope (docs-only, max 2 files):
1. `docs/architecture/truenas-operational-runbook.md` (append canary section)
2. `docs/architecture/truenas-rollout-readiness-progress-2026-02.md` (update)

Implementation checklist:
1. Document phased rollout strategy:
   - Phase 1: Internal/dev with `PULSE_ENABLE_TRUENAS=true` (existing).
   - Phase 2: Opt-in early adopters via env var.
   - Phase 3: Default-on with env var opt-out (`PULSE_ENABLE_TRUENAS=false` to disable).
2. Document abort criteria:
   - Poll failure rate > 50% across all connections for > 5 minutes → abort.
   - Any data corruption or cross-platform interference → immediate abort.
   - Memory/CPU regression > 20% attributable to TrueNAS poller → abort.
3. Document rollback from each phase.
4. Document monitoring checkpoints at each phase transition.

Required tests:
1. `go build ./...` (confirms code integrity)

Exit criteria:
- Canary strategy with abort criteria and phase-gate verification documented.

### TRR-04: Soak/Failure-Injection Validation

Objective:
- Validate TrueNAS subsystem resilience under failure conditions.

Scope (max 5 files, backend only):
1. `internal/monitoring/truenas_poller_test.go` (extend — failure injection tests)
2. `internal/truenas/integration_test.go` (extend — if needed)

Implementation checklist:
1. Test: API timeout (slow response > configured timeout) → poller continues, metrics record timeout.
2. Test: Auth failure mid-session (API key revoked) → poller logs error, metrics record auth failure, no crash.
3. Test: Stale data recovery (N consecutive failures → staleness → successful poll → fresh data).
4. Test: Connection flap (server up → down → up) → poller reconnects, resources recover.
5. Test: Concurrent poll with config change (add/remove connection during active poll) → no race/panic.

Required tests:
1. `go test ./internal/monitoring/... -run "TrueNAS" -count=1 -v`
2. `go test ./internal/truenas/... -count=1 -v`
3. `go build ./...`

Exit criteria:
- All failure-injection tests pass; no panics, races, or resource leaks.

### TRR-05: Final Rollout Verdict

Objective:
- Issue final evidence-backed rollout verdict.

Scope:
- `docs/architecture/truenas-rollout-readiness-progress-2026-02.md`

Implementation checklist:
1. Verify TRR-00 through TRR-04 are all DONE/APPROVED.
2. Re-run full milestone validation commands with explicit exit codes.
3. Reconcile residual risks from all packets.
4. Issue final verdict: GO / GO_WITH_CONDITIONS / NO_GO with justification.

Required tests:
1. `go build ./... && go test ./internal/truenas/... ./internal/monitoring/... -count=1`
2. `go test ./internal/api/... -run "TrueNAS" -count=1`
3. `cd frontend-modern && npx vitest run`
4. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Final rollout verdict is evidence-backed and auditable.

## Dependency Graph

```
TRR-00 (scope freeze)
  │
  ├── TRR-01 (operational runbook) ─── docs only
  │     │
  │     └── TRR-03 (canary controls) ─── extends runbook
  │
  └── TRR-02 (telemetry + thresholds) ─── backend code
        │
        └── TRR-04 (failure injection) ─── extends tests

TRR-05 (final verdict) ─── depends on all
```

Critical path: TRR-00 → TRR-02 → TRR-04 → TRR-05
Parallel track: TRR-01 → TRR-03 (docs, can run parallel to backend work)

## Risk Register

| ID | Severity | Risk | Mitigation Packet |
|---|---|---|---|
| TRR-R001 | Medium | PollMetrics wiring may require interface changes to poller | TRR-02 (use existing PollMetrics API) |
| TRR-R002 | Low | MonitorError wrapping may need client.go changes | TRR-02 (wrap at poller level, not client) |
| TRR-R003 | Low | Failure-injection tests may be flaky with real timers | TRR-04 (use test timeouts, not real delays) |
| TRR-R004 | Low | Canary phase 3 (default-on) requires changing feature flag default | TRR-05 (defer to rollout verdict) |
