# TrueNAS GA Lane Progress Tracker

Linked plan:
- `docs/architecture/truenas-ga-plan-2026-02.md` (authoritative execution spec)

Related lanes:
- `docs/architecture/unified-resource-finalization-progress-2026-02.md` (unified resource model)
- `docs/architecture/storage-page-ga-hardening-progress-2026-02.md` (storage UI, complete)

Status: Active
Date: 2026-02-08

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
10. TrueNAS must remain behind `PULSE_ENABLE_TRUENAS` feature flag until TN-11 certification.

## Packet Board

| Packet | Title | Status | Implementer | Reviewer | Review State | Evidence Link |
|---|---|---|---|---|---|---|
| TN-00 | Scope Freeze + Current-State Audit | DONE | Claude | Claude | APPROVED | TN-00 Review Evidence |
| TN-01 | TrueNAS REST API Client Scaffold | TODO | Codex | Claude | — | — |
| TN-02 | Configuration Model + Encrypted Persistence | TODO | Codex | Claude | — | — |
| TN-03 | Setup API Endpoints (Add/Test/Remove) | TODO | Codex | Claude | — | — |
| TN-04 | Live Provider Upgrade (Fixture -> API Client) | TODO | Codex | Claude | — | — |
| TN-05 | Runtime Registration + Periodic Polling | TODO | Codex | Claude | — | — |
| TN-06 | Frontend Source Badge + Filter Integration | TODO | Codex | Claude | — | — |
| TN-07 | Backend Health/Error State Enrichment | TODO | Codex | Claude | — | — |
| TN-08 | Frontend Health/Error UX Display | TODO | Codex | Claude | — | — |
| TN-09 | Alert + AI Context Compatibility | TODO | Codex | Claude | — | — |
| TN-10 | Integration Test Matrix + E2E Validation | TODO | Codex | Claude | — | — |
| TN-11 | Final Certification + GA Readiness Verdict | TODO | Claude | Claude | — | — |

---

## TN-00 Checklist: Scope Freeze + Current-State Audit

- [x] Current `internal/truenas/` code audited and gaps documented.
- [x] Unified resource integration points verified.
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

### TN-00 Review Evidence

```markdown
Files changed:
- `docs/architecture/truenas-ga-plan-2026-02.md` (new): Full execution spec with 12 packets, risk register, dependency graph, validation baseline, and non-negotiable contracts.
- `docs/architecture/truenas-ga-progress-2026-02.md` (new): Progress tracker with per-packet checklists, checkpoint commit tracking, and review evidence sections.

Audit findings (verified):
1. `internal/truenas/` is fixture-only: 0 `net/http` references, 4 files (types.go, provider.go, fixtures.go, contract_test.go).
2. Provider not wired to runtime: 0 hits for `truenas.NewProvider` or `truenas.NewDefaultProvider` outside the truenas package.
3. Frontend bug: `getUnifiedSourceBadges()` at resourceBadges.ts:91 excludes 'truenas' from filter list.
4. `SourceTrueNAS` registered in unified registry with 120s stale threshold.
5. V2 API source filter recognizes "truenas" (resources_v2.go lines 740-746).
6. Frontend type/badge/platform definitions exist but storage adapters and Infrastructure filter are incomplete.
7. No TrueNAS config model, setup endpoints, or periodic polling infrastructure.

Definition-of-done contracts:
- No new /api/resources dependencies
- No useResourcesAsLegacy usage
- No new LegacyResource contract dependencies
- All data via IngestRecords(SourceTrueNAS, records)
- Feature flag PULSE_ENABLE_TRUENAS gates until TN-11

Dependency gates frozen:
- TN-04 depends on TN-01 (client must exist before provider upgrade)
- TN-05 depends on TN-02, TN-03, TN-04 (config + client + provider before polling)
- TN-08 depends on TN-06, TN-07 (badges + health enrichment before health UX)
- TN-10 depends on TN-05, TN-07, TN-09 (integration tests need full backend)
- TN-11 depends on all

Commands run + exit codes:
1. `go build ./...` -> exit 0
2. `go test ./internal/truenas/... -count=1` -> exit 0 (ok, 0.392s)
3. `grep -r "net/http" internal/truenas/` -> 0 matches (no HTTP client confirmed)
4. `rg "truenas\.NewProvider|truenas\.NewDefaultProvider" internal/` -> 0 matches (not wired to runtime confirmed)
5. `rg "getUnifiedSourceBadges" frontend-modern/src/components/Infrastructure/resourceBadges.ts` -> line 91 excludes 'truenas' (bug confirmed)

Gate checklist:
- P0: PASS (plan/progress docs created, audit verified with explicit command evidence)
- P1: N/A (docs-only packet, no behavioral changes)
- P2: PASS (progress tracker initialized, packet board populated, audit gaps documented)

Verdict: APPROVED

Residual risk:
- None. Docs-only packet.

Rollback:
- Delete both plan and progress docs.
```

---

## TN-01 Checklist: TrueNAS REST API Client Scaffold

- [ ] `Client` struct with HTTP transport, base URL, and API key auth created.
- [ ] Methods implemented: `GetSystemInfo()`, `GetPools()`, `GetDatasets()`, `GetDisks()`, `GetAlerts()`, `TestConnection()`.
- [ ] TrueNAS API v2.0 JSON response parsing into existing types.
- [ ] TLS handling (skip-verify, fingerprint pinning).
- [ ] Unit tests with HTTP test server mocking TrueNAS responses.

### Required Tests

- [ ] `go test ./internal/truenas/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-01 Review Evidence

```markdown
TODO
```

---

## TN-02 Checklist: Configuration Model + Encrypted Persistence

- [ ] `TrueNASConfig` struct defined with required fields.
- [ ] Persistence via `saveJSON[T]()` / `loadSlice[T]()` generics.
- [ ] API key encrypted using existing encryption infrastructure.
- [ ] Config CRUD helpers: Add, Update, Remove, Get, List.
- [ ] Unit tests for serialization and validation.

### Required Tests

- [ ] `go test ./internal/config/... -run "TrueNAS" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-02 Review Evidence

```markdown
TODO
```

---

## TN-03 Checklist: Setup API Endpoints (Add/Test/Remove)

- [ ] `POST /api/truenas/connections` — add connection.
- [ ] `GET /api/truenas/connections` — list connections (redacted keys).
- [ ] `DELETE /api/truenas/connections/{id}` — remove connection.
- [ ] `POST /api/truenas/connections/test` — test connectivity.
- [ ] License feature / feature flag gating.
- [ ] Node limit enforcement on registration.
- [ ] Handler tests.

### Required Tests

- [ ] `go test ./internal/api/... -run "TrueNAS" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-03 Review Evidence

```markdown
TODO
```

---

## TN-04 Checklist: Live Provider Upgrade (Fixture -> API Client)

- [ ] `Fetcher` interface defined: `Fetch(ctx) (*FixtureSnapshot, error)`.
- [ ] `APIFetcher` implemented using `Client`.
- [ ] `FixtureFetcher` wrapping static fixtures.
- [ ] `Provider` updated to accept `Fetcher`.
- [ ] Fetch error handling (last known snapshot, logging).
- [ ] Existing test path preserved via `FixtureFetcher`.

### Required Tests

- [ ] `go test ./internal/truenas/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-04 Review Evidence

```markdown
TODO
```

---

## TN-05 Checklist: Runtime Registration + Periodic Polling

- [ ] `TrueNASPoller` struct managing per-connection providers.
- [ ] Polling loop with configurable interval (default 60s).
- [ ] Runtime connection add/remove handling.
- [ ] Integration with unified resource registry via `IngestRecords`.
- [ ] Staleness handling on fetch failure.
- [ ] Wired into router/monitor lifecycle.

### Required Tests

- [ ] `go test ./internal/monitoring/... -run "TrueNAS" -count=1` -> exit 0
- [ ] `go test ./internal/truenas/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-05 Review Evidence

```markdown
TODO
```

---

## TN-06 Checklist: Frontend Source Badge + Filter Integration

- [ ] TrueNAS added to `getUnifiedSourceBadges()` filter list.
- [ ] TrueNAS resources render correctly in Infrastructure page.
- [ ] TrueNAS pools/datasets appear in Storage page.
- [ ] Test for TrueNAS source badge presence.

### Required Tests

- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [ ] `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/resourceBadges.test.ts src/features/storageBackupsV2/__tests__/storageAdapters.test.ts` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-06 Review Evidence

```markdown
TODO
```

---

## TN-07 Checklist: Backend Health/Error State Enrichment

- [ ] ZFS pool health states exhaustively mapped (ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL).
- [ ] Dataset states mapped (mounted+writable, readonly, unmounted).
- [ ] Scrub status and error counts in metadata.
- [ ] Connection error handling (unreachable → stale).
- [ ] Deterministic tests for all health state transitions.

### Required Tests

- [ ] `go test ./internal/truenas/... -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-07 Review Evidence

```markdown
TODO
```

---

## TN-08 Checklist: Frontend Health/Error UX Display

- [ ] TrueNAS pool/dataset health states render with correct status indicators.
- [ ] Connection error/stale state shows appropriate warning.
- [ ] ZFS-specific labels visible in storage detail view.
- [ ] Tests for degraded and error state display.

### Required Tests

- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0
- [ ] `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-08 Review Evidence

```markdown
TODO
```

---

## TN-09 Checklist: Alert + AI Context Compatibility

- [ ] TrueNAS resources flow through AI resource context provider.
- [ ] Alert rules can reference TrueNAS resource IDs.
- [ ] TrueNAS-native alerts surfaced as unified annotations.
- [ ] Tests for AI infrastructure summary inclusion.
- [ ] Tests for alert targeting.

### Required Tests

- [ ] `go test ./internal/truenas/... -count=1` -> exit 0
- [ ] `go test ./internal/ai/... -run "ResourceContext" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-09 Review Evidence

```markdown
TODO
```

---

## TN-10 Checklist: Integration Test Matrix + E2E Validation

- [ ] Mock TrueNAS HTTP server created.
- [ ] Full lifecycle test: config → poll → ingest → query → verify.
- [ ] Error scenarios tested: connection refused, auth failure, malformed response.
- [ ] Health state transitions tested: healthy → degraded → faulted → recovery.
- [ ] Stale resource handling tested: poll failure → staleness → recovery.

### Required Tests

- [ ] `go test ./internal/truenas/... -count=1 -v` -> exit 0
- [ ] `go test ./internal/api/... -run "TrueNAS" -count=1` -> exit 0
- [ ] `go build ./...` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-10 Review Evidence

```markdown
TODO
```

---

## TN-11 Checklist: Final Certification + GA Readiness Verdict

- [ ] TN-00 through TN-10 are all `DONE` and `APPROVED`.
- [ ] Full milestone validation commands rerun with explicit exit codes.
- [ ] Feature flag GA activation path documented.
- [ ] Rollback runbook recorded.
- [ ] Final readiness verdict recorded (`GO` or `GO_WITH_CONDITIONS`).

### Required Tests

- [ ] `go build ./... && go test ./internal/truenas/... ./internal/api/... ./internal/unifiedresources/... ./internal/ai/... -count=1` -> exit 0
- [ ] `cd frontend-modern && npx vitest run` -> exit 0
- [ ] `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json` -> exit 0

### Review Gates

- [ ] P0 PASS
- [ ] P1 PASS
- [ ] P2 PASS
- [ ] Verdict recorded: `APPROVED`

### TN-11 Review Evidence

```markdown
TODO
```

---

## Checkpoint Commits

- TN-00: TODO
- TN-01: TODO
- TN-02: TODO
- TN-03: TODO
- TN-04: TODO
- TN-05: TODO
- TN-06: TODO
- TN-07: TODO
- TN-08: TODO
- TN-09: TODO
- TN-10: TODO
- TN-11: TODO

## Current Recommended Next Packet

- `TN-01` (TrueNAS REST API Client Scaffold)
