# TrueNAS GA Lane Plan (Detailed Execution Spec)

Status: Active
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/truenas-ga-progress-2026-02.md`

Related lanes:
- `docs/architecture/unified-resource-finalization-plan-2026-02.md` (unified resource model)
- `docs/architecture/storage-page-ga-hardening-plan-2026-02.md` (storage UI hardening, complete)
- `docs/architecture/release-readiness-guiding-light-2026-02.md` (W2 exit criteria)

## Intent

Graduate TrueNAS from a fixture-only prototype to a production-grade, unified-resource-native integration. TrueNAS resources must flow through the same unified resource model as Proxmox, Docker, Kubernetes, and Host agents. No legacy coupling is permitted.

Primary outcomes:
1. Real TrueNAS REST API client replaces fixture-only data path.
2. TrueNAS connections are configurable via setup UI with encrypted credential persistence.
3. Periodic discovery populates the unified resource registry with live TrueNAS data.
4. Health, degraded, and error states are deterministic and user-visible.
5. TrueNAS resources participate in alerts and AI context through unified model.
6. Test matrix and rollback runbook are certified before GA.

## Definition of Done

This lane is complete only when all are true:
1. A real HTTP client fetches system, pool, dataset, and disk data from TrueNAS REST API.
2. TrueNAS connections are configurable (add/test/remove) via API with encrypted persistence.
3. Periodic polling ingests live TrueNAS data into the unified resource registry via `IngestRecords`.
4. Frontend Infrastructure and Storage pages display TrueNAS resources with source badges and filters.
5. Health/degraded/error states map deterministically from ZFS pool/dataset states.
6. Alert rules can target TrueNAS resources; TrueNAS-native alerts are surfaced.
7. AI context provider includes TrueNAS resources in infrastructure summaries.
8. Integration tests cover the full lifecycle (config → fetch → ingest → display → alert → AI).
9. No new `/api/resources` dependencies, `useResourcesAsLegacy` usage, or `LegacyResource` contract dependencies.
10. Final certification packet approved with explicit go/no-go verdict.

## Code-Derived Baseline (Current State)

### A. Backend — `internal/truenas/`

1. Fixture-only provider:
- `internal/truenas/provider.go`: `Records()` converts fixture data to `[]unifiedresources.IngestRecord`.
- `internal/truenas/fixtures.go`: Hardcoded static data (3 pools, 5 datasets, 4 disks).
- `internal/truenas/types.go`: Data shapes for `FixtureSnapshot`, `SystemInfo`, `Pool`, `Dataset`, `Disk`.
- Zero HTTP/SSH client code. `grep -r "net/http" internal/truenas/` returns 0 results.

2. Feature flag:
- `PULSE_ENABLE_TRUENAS` env var gates `Provider.Records()` output.
- `IsFeatureEnabled()` / `SetFeatureEnabled()` functions exist.

3. Contract tests:
- `internal/truenas/contract_test.go`: 3 tests covering feature flag gating, registry ingestion, and legacy render type mapping.

### B. Unified resource integration

1. `SourceTrueNAS` registered in registry source map (`internal/unifiedresources/registry.go`).
2. Stale threshold: 120 seconds (same as Docker/PBS/PMG).
3. TrueNAS resources map to canonical `ResourceTypeHost` and `ResourceTypeStorage` — no TrueNAS-specific resource types.
4. `LegacyResourceTypeTrueNAS` exists in `legacy_contract.go` but is never emitted by adapters.
5. `LegacyPlatformTrueNAS` exists but is unused.

### C. API surface

1. `/api/v2/resources` source filter recognizes `"truenas"` (`internal/api/resources_v2.go` lines 740-746).
2. No TrueNAS-specific API endpoints exist.
3. No TrueNAS configuration endpoints exist.

### D. Frontend

1. Type definitions include `'truenas'` in ResourceType and PlatformType unions (`frontend-modern/src/types/resource.ts`).
2. Platform badge defined: `truenas: { label: 'TrueNAS', tone: 'bg-blue-100...' }` (`sourcePlatformBadges.ts`).
3. **BUG**: `getUnifiedSourceBadges()` in `resourceBadges.ts` line 91 does NOT include TrueNAS in the normalized filter list.
4. `ResourceDetailDrawer.tsx` maps TrueNAS resources to host-like discovery targets.
5. Storage backups model lists TrueNAS as `stage: 'next'` platform.

### E. Runtime wiring — NOT CONNECTED

1. No code invokes `truenas.NewProvider()` or `truenas.NewDefaultProvider()` in the main application flow.
2. No periodic polling or refresh mechanism exists.
3. No TrueNAS configuration model or persistence exists.
4. Provider ingestion is test-only.

## Non-Negotiable Contracts

1. Unified-resource-native only:
- All TrueNAS data flows through `IngestRecords(SourceTrueNAS, records)`.
- No new `/api/resources` dependencies.
- No `useResourcesAsLegacy` usage.
- No new `LegacyResource` contract dependencies.
- If a packet requires legacy coupling, it MUST be marked `BLOCKED` with explicit unblock condition.

2. Packet scope contract:
- No packet crosses more than one subsystem boundary (backend, frontend, infra, docs).
- No packet combines abstraction creation + rewiring + deletion in one step.
- Max 3-5 files changed target per packet.

3. Evidence contract:
- No packet is APPROVED without explicit command exit codes.
- Timeout, empty output, truncated output, or missing exit code is an automatic failed gate.

4. Rollback contract:
- Each packet has file-level rollback guidance.
- No destructive git operations.

5. Configuration security contract:
- TrueNAS credentials (API keys) must use encrypted persistence (consistent with existing `ai.enc` / `nodes.enc` patterns).
- No plaintext credential storage.

6. Feature flag contract:
- TrueNAS integration remains behind `PULSE_ENABLE_TRUENAS` feature flag until GA certification.
- Flag gates both backend ingestion and frontend display.

## Risk Register

| ID | Severity | Risk | Mitigation Packet |
|---|---|---|---|
| TN-R001 | High | HTTP client implementation may not handle all TrueNAS API versions/auth methods. | TN-01 (client with version detection) |
| TN-R002 | High | Credential storage pattern mismatch with existing encrypted config. | TN-02 (follow nodes.enc pattern) |
| TN-R003 | High | Periodic polling may overwhelm TrueNAS devices or cause stale data. | TN-05 (configurable interval, staleness) |
| TN-R004 | Medium | Frontend source badge/filter omission causes TrueNAS resources to be invisible. | TN-06 (fix getUnifiedSourceBadges) |
| TN-R005 | Medium | ZFS health states may not map cleanly to unified status model. | TN-07 (explicit mapping table) |
| TN-R006 | Medium | AI context may not surface TrueNAS-specific storage semantics. | TN-09 (verification + enrichment) |
| TN-R007 | Low | Node limit enforcement may not count TrueNAS connections. | TN-03 (enforce on registration) |

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: Codex
- Reviewer: Claude

A packet is `DONE` only when:
1. all packet checkboxes are complete,
2. required commands have explicit exit codes,
3. reviewer gate checklist passes,
4. verdict is `APPROVED`.

## Required Review Output (Every Packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit 0
2. `<command>` -> exit 0

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

## Global Validation Baseline

Run after every backend packet unless explicitly waived:

1. `go build ./...`
2. `go test ./internal/truenas/... -count=1`

When API handlers are touched, additionally run:

3. `go test ./internal/api/... -run "ResourcesV2|TrueNAS" -count=1`

When unified resource model is touched, additionally run:

4. `go test ./internal/unifiedresources/... -count=1`

When frontend is touched, run:

5. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Milestone boundary baselines (TN-06, TN-09, TN-11):

6. `cd frontend-modern && npx vitest run`
7. `go build ./... && go test ./internal/truenas/... ./internal/api/... ./internal/unifiedresources/... -count=1`

## Execution Packets

### TN-00: Scope Freeze + Current-State Audit (Docs-Only)

Objective:
- Freeze the TrueNAS GA gap baseline, packet boundaries, and dependency gates.

Scope:
- `docs/architecture/truenas-ga-plan-2026-02.md`
- `docs/architecture/truenas-ga-progress-2026-02.md`

Implementation checklist:
1. Audit current `internal/truenas/` code and document gaps.
2. Verify unified resource integration points.
3. Freeze packet boundaries and dependency gates.
4. Record definition-of-done contracts.

Required tests:
1. `go build ./...`
2. `go test ./internal/truenas/... -count=1`

Exit criteria:
- Gap baseline and packet gates are reviewer-approved.

### TN-01: TrueNAS REST API Client Scaffold (Backend)

Objective:
- Implement an HTTP client for the TrueNAS REST API (`/api/v2.0/`).

Scope (max 5 files):
1. `internal/truenas/client.go` (new)
2. `internal/truenas/client_test.go` (new)
3. `internal/truenas/types.go` (extend with API response types if needed)

Implementation checklist:
1. Create `Client` struct with HTTP transport, base URL, and API key auth.
2. Implement methods: `GetSystemInfo()`, `GetPools()`, `GetDatasets()`, `GetDisks()`, `GetAlerts()`, `TestConnection()`.
3. Parse TrueNAS API v2.0 JSON responses into existing `truenas.SystemInfo`, `Pool`, `Dataset`, `Disk` types.
4. Handle authentication (API key as Bearer token, basic auth fallback).
5. Handle TLS (skip-verify option for self-signed certs, fingerprint pinning).
6. Add unit tests with HTTP test server mocking TrueNAS responses.

Required tests:
1. `go test ./internal/truenas/... -count=1`
2. `go build ./...`

Exit criteria:
- HTTP client can fetch and parse TrueNAS API responses in unit tests.

### TN-02: Configuration Model + Encrypted Persistence (Backend)

Objective:
- Add a TrueNAS connection configuration model with encrypted credential storage.

Scope (max 5 files):
1. `internal/config/truenas.go` (new)
2. `internal/config/truenas_test.go` (new)
3. `internal/truenas/types.go` (add ConnectionConfig if needed)

Implementation checklist:
1. Define `TrueNASConfig` struct: ID, Name, Host, Port, APIKey (encrypted), TLSSkipVerify, Fingerprint, Enabled, PollIntervalSeconds.
2. Implement persistence using `saveJSON[T]()` / `loadSlice[T]()` generics from `config/persistence.go`.
3. Encrypt API key using existing encryption key infrastructure (consistent with `nodes.enc` pattern).
4. Add config CRUD helpers: Add, Update, Remove, Get, List.
5. Add unit tests for serialization round-trip and validation.

Required tests:
1. `go test ./internal/config/... -run "TrueNAS" -count=1`
2. `go build ./...`

Exit criteria:
- TrueNAS configs persist and load with encrypted credentials.

### TN-03: Setup API Endpoints (Add/Test/Remove Connection) (Backend)

Objective:
- Expose HTTP endpoints for managing TrueNAS connections.

Scope (max 5 files):
1. `internal/api/truenas_handlers.go` (new)
2. `internal/api/truenas_handlers_test.go` (new)
3. `internal/api/router_routes_truenas.go` (new) or addition to existing route file
4. `internal/api/router.go` (wire TrueNAS handlers)

Implementation checklist:
1. `POST /api/truenas/connections` — add new TrueNAS connection (validate + persist).
2. `GET /api/truenas/connections` — list configured connections (redact API keys).
3. `DELETE /api/truenas/connections/{id}` — remove connection.
4. `POST /api/truenas/connections/test` — test connectivity (calls `client.TestConnection()`).
5. Gate all endpoints behind `RequireLicenseFeature` (or feature flag check).
6. Enforce node limit on new connection registration (use `enforceNodeLimitForConfigRegistration` pattern).
7. Add handler tests.

Required tests:
1. `go test ./internal/api/... -run "TrueNAS" -count=1`
2. `go build ./...`

Exit criteria:
- TrueNAS connections can be added, tested, listed, and removed via API.

### TN-04: Live Provider Upgrade (Fixture -> API Client) (Backend)

Objective:
- Upgrade `Provider` to fetch from real TrueNAS API, keeping fixture path for tests/mock mode.

Scope (max 5 files):
1. `internal/truenas/provider.go`
2. `internal/truenas/provider_test.go` (new or extend `contract_test.go`)
3. `internal/truenas/client.go` (minor interface extraction if needed)

Implementation checklist:
1. Add `Fetcher` interface: `Fetch(ctx context.Context) (*FixtureSnapshot, error)`.
2. Implement `APIFetcher` using `Client` to populate `FixtureSnapshot` from live API.
3. Implement `FixtureFetcher` wrapping static fixtures (existing behavior).
4. Update `Provider` to accept `Fetcher`, call `Fetch()` before `Records()`.
5. Handle fetch errors gracefully (return last known snapshot, log warning).
6. Preserve existing test path via `FixtureFetcher`.

Required tests:
1. `go test ./internal/truenas/... -count=1`
2. `go build ./...`

Exit criteria:
- Provider can produce `IngestRecord` from both live API and fixtures.

### TN-05: Runtime Registration + Periodic Polling (Backend)

Objective:
- Wire TrueNAS provider into the application lifecycle with periodic polling.

Scope (max 5 files):
1. `internal/monitoring/truenas_poller.go` (new)
2. `internal/monitoring/truenas_poller_test.go` (new)
3. `internal/api/router.go` (wire poller initialization)

Implementation checklist:
1. Create `TrueNASPoller` struct managing one `Provider` + `APIFetcher` per configured connection.
2. Implement polling loop: configurable interval (default 60s), fetch → Records() → IngestRecords().
3. Handle connection add/remove at runtime (config change listener).
4. Integrate with unified resource registry via `IngestRecords(SourceTrueNAS, records)`.
5. Handle staleness: if fetch fails, mark resources stale (existing registry staleness handles this).
6. Wire poller start/stop into router/monitor lifecycle.

Required tests:
1. `go test ./internal/monitoring/... -run "TrueNAS" -count=1`
2. `go test ./internal/truenas/... -count=1`
3. `go build ./...`

Exit criteria:
- TrueNAS resources appear in unified registry from live polling.

### TN-06: Frontend Source Badge + Filter Integration (Frontend)

Objective:
- Ensure TrueNAS resources are visible and filterable in Infrastructure and Storage pages.

Scope (max 5 files):
1. `frontend-modern/src/components/Infrastructure/resourceBadges.ts`
2. `frontend-modern/src/components/Infrastructure/__tests__/resourceBadges.test.ts` (new or update)
3. `frontend-modern/src/features/storageBackupsV2/storageAdapters.ts` (if TrueNAS storage needs adapter support)
4. `frontend-modern/src/types/resource.ts` (verify/fix type coverage)

Implementation checklist:
1. Add `'truenas'` to `getUnifiedSourceBadges()` normalized filter list (fix existing bug).
2. Verify TrueNAS resources render correctly in Infrastructure resource list.
3. Verify TrueNAS pools/datasets appear in Storage page through existing unified storage adapters.
4. Add test for TrueNAS source badge presence and filter behavior.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run src/components/Infrastructure/__tests__/resourceBadges.test.ts src/features/storageBackupsV2/__tests__/storageAdapters.test.ts`

Exit criteria:
- TrueNAS resources appear with proper badges and are filterable by source.

### TN-07: Backend Health/Error State Enrichment (Backend)

Objective:
- Enrich TrueNAS resource metadata with ZFS-specific health details and connection error handling.

Scope (max 5 files):
1. `internal/truenas/provider.go`
2. `internal/truenas/types.go` (add health detail fields if needed)
3. `internal/truenas/contract_test.go` (extend health mapping tests)
4. `internal/unifiedresources/types.go` (extend `StorageMeta` for ZFS details if not already present)

Implementation checklist:
1. Map ZFS pool health states exhaustively: ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL.
2. Map dataset states: mounted+writable → online, readonly → warning, unmounted → offline.
3. Include scrub status and error counts in storage metadata tags.
4. Handle connection errors: unreachable TrueNAS → all resources marked warning/stale (not offline, to distinguish from intentional shutdown).
5. Add deterministic tests for all health state transitions.

Required tests:
1. `go test ./internal/truenas/... -count=1`
2. `go build ./...`

Exit criteria:
- All ZFS health states map deterministically to unified status model.

### TN-08: Frontend Health/Error UX Display (Frontend)

Objective:
- Display TrueNAS health states and connection errors in the UI.

Scope (max 5 files):
1. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`
2. `frontend-modern/src/components/Storage/StorageV2.tsx`
3. `frontend-modern/src/components/Storage/__tests__/StorageV2.test.tsx`

Implementation checklist:
1. Verify TrueNAS pool/dataset health states render with correct status indicators (healthy/degraded/faulted).
2. Verify connection error state (stale/unreachable) shows appropriate warning in resource detail.
3. Ensure ZFS-specific labels (pool health, scrub status) are visible in storage detail view.
4. Add tests for degraded and error state display.

Required tests:
1. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
2. `cd frontend-modern && npx vitest run src/components/Storage/__tests__/StorageV2.test.tsx`

Exit criteria:
- Health/degraded/error states display correctly for TrueNAS resources.

### TN-09: Alert + AI Context Compatibility (Backend)

Objective:
- Verify and ensure TrueNAS resources work with the alert system and AI context provider.

Scope (max 5 files):
1. `internal/truenas/provider.go` (if alert ingestion from TrueNAS API alerts is needed)
2. `internal/truenas/contract_test.go` (extend with alert/AI flow tests)
3. `internal/ai/resource_context.go` (verify TrueNAS inclusion, minor fix if needed)
4. `internal/ai/resource_context_test.go` (add TrueNAS resource context test)

Implementation checklist:
1. Verify TrueNAS resources flow through AI resource context provider (LegacyAdapter → AI context).
2. Verify alert rules can reference TrueNAS resource IDs and source IDs.
3. Surface TrueNAS-native alerts (from `GET /api/v2.0/alert/list`) as unified alert annotations on resources.
4. Add tests proving TrueNAS resources appear in AI infrastructure summaries.
5. Add tests proving alert targeting works for TrueNAS resource types.

Required tests:
1. `go test ./internal/truenas/... -count=1`
2. `go test ./internal/ai/... -run "ResourceContext" -count=1`
3. `go build ./...`

Exit criteria:
- TrueNAS resources participate in alerts and AI context through unified model.

### TN-10: Integration Test Matrix + E2E Validation (Tests)

Objective:
- Comprehensive integration tests covering the full TrueNAS lifecycle.

Scope (max 5 files):
1. `internal/truenas/integration_test.go` (new)
2. `internal/truenas/mock_server_test.go` (new — HTTP test server mimicking TrueNAS API)
3. `internal/api/truenas_handlers_test.go` (extend with lifecycle tests)

Implementation checklist:
1. Create mock TrueNAS HTTP server returning realistic API responses.
2. Test full lifecycle: configure connection → poll → ingest → query via V2 API → verify resources.
3. Test error scenarios: connection refused, auth failure, malformed response, partial data.
4. Test health state transitions: healthy → degraded → faulted → recovery.
5. Test stale resource handling: poll failure → staleness marking → recovery on next success.

Required tests:
1. `go test ./internal/truenas/... -count=1 -v`
2. `go test ./internal/api/... -run "TrueNAS" -count=1`
3. `go build ./...`

Exit criteria:
- All lifecycle and error scenarios have passing integration tests.

### TN-11: Final Certification + GA Readiness Verdict (Docs)

Objective:
- Certify lane completion with explicit go/no-go evidence.

Scope:
- `docs/architecture/truenas-ga-progress-2026-02.md`

Implementation checklist:
1. Verify TN-00 through TN-10 are all `DONE/APPROVED`.
2. Execute full milestone validation commands with explicit exit codes.
3. Record feature flag status and GA activation path.
4. Produce rollback runbook (per-packet revert instructions).
5. Record final verdict: `GO` or `GO_WITH_CONDITIONS` with explicit blocker list.

Required tests:
1. `go build ./... && go test ./internal/truenas/... ./internal/api/... ./internal/unifiedresources/... ./internal/ai/... -count=1`
2. `cd frontend-modern && npx vitest run`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Final certification approved with explicit GA readiness verdict.

## Dependency Graph

```
TN-00 (scope freeze)
  │
  ├── TN-01 (HTTP client)
  │     │
  │     └── TN-04 (live provider upgrade)
  │           │
  │           └── TN-05 (runtime polling)
  │                 │
  │                 ├── TN-07 (health enrichment)
  │                 │     │
  │                 │     └── TN-08 (health UX)
  │                 │
  │                 └── TN-09 (alert + AI compat)
  │
  ├── TN-02 (config model)
  │     │
  │     └── TN-03 (setup API)
  │           │
  │           └── TN-05 (runtime polling)
  │
  └── TN-06 (frontend badges/filters)
        │
        └── TN-08 (health UX)

TN-10 (integration tests) — depends on TN-05, TN-07, TN-09
TN-11 (certification) — depends on all
```

Critical path: TN-00 → TN-01 → TN-04 → TN-05 → TN-07 → TN-10 → TN-11

Parallel track A: TN-02 → TN-03 (can run parallel to TN-01)
Parallel track B: TN-06 (can run parallel to backend work)

## Explicitly Deferred Beyond TN Lane

1. TrueNAS setup wizard UI (frontend configuration page) — deferred to frontend UX lane.
2. TrueNAS snapshot management (create/delete/rollback snapshots via API) — post-GA feature.
3. TrueNAS replication monitoring — post-GA feature.
4. TrueNAS CORE (FreeBSD) support — SCALE-only for GA.
5. Multi-TrueNAS-instance dashboard aggregation — post-GA.
6. TrueNAS service monitoring (SMB/NFS/iSCSI) — post-GA feature.
