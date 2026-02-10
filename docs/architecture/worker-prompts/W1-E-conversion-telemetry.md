# Worker Prompt: W1-E — Make Conversion Telemetry Persistent and Tenant-Aware

## Task ID: W1-E (P0-6)

## Goal

Replace the in-memory conversion telemetry recorder with persistent SQLite storage that is tenant-aware. Currently, `internal/license/conversion/recorder.go` hardcodes tenant ID to `"default"` and writes to an in-memory `WindowedAggregator`. Events are lost on restart.

## Current State

`internal/license/conversion/recorder.go`:
- `Recorder` struct wraps a `*metering.WindowedAggregator` (in-memory)
- `Record()` method (lines 20-40) converts `ConversionEvent` to `metering.Event` with `TenantID: "default"` hardcoded (line 30)
- `Snapshot()` method returns in-memory aggregation window
- Events are lost on process restart

## Scope (Exact Files)

### 1. `internal/license/conversion/store.go` (NEW FILE)

Create a SQLite-backed conversion event store:

```go
type ConversionStore struct {
    db *sql.DB
}

type StoredConversionEvent struct {
    ID             int64
    OrgID          string
    EventType      string    // paywall_viewed, trial_started, upgrade_clicked, checkout_completed
    Surface        string    // where in the UI the event originated
    Capability     string    // which feature triggered it
    IdempotencyKey string    // for deduplication
    CreatedAt      time.Time
}
```

**Schema:**
```sql
CREATE TABLE IF NOT EXISTS conversion_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    surface TEXT NOT NULL DEFAULT '',
    capability TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_conversion_events_org_time ON conversion_events(org_id, created_at);
CREATE INDEX IF NOT EXISTS idx_conversion_events_type ON conversion_events(event_type, created_at);
```

**Methods:**
- `NewConversionStore(dbPath string) (*ConversionStore, error)` — opens/creates SQLite DB with WAL mode
- `Record(event StoredConversionEvent) error` — INSERT OR IGNORE (idempotency via unique key)
- `Query(orgID string, from, to time.Time, eventType string) ([]StoredConversionEvent, error)` — filtered query
- `FunnelSummary(orgID string, from, to time.Time) (*FunnelSummary, error)` — aggregate counts per event type
- `Close() error`

```go
type FunnelSummary struct {
    PaywallViewed    int64 `json:"paywall_viewed"`
    TrialStarted     int64 `json:"trial_started"`
    UpgradeClicked   int64 `json:"upgrade_clicked"`
    CheckoutCompleted int64 `json:"checkout_completed"`
    Period           struct {
        From time.Time `json:"from"`
        To   time.Time `json:"to"`
    } `json:"period"`
}
```

### 2. `internal/license/conversion/recorder.go` (MODIFY)

Update `Recorder` to use `ConversionStore` instead of (or in addition to) the in-memory aggregator:

- Add `store *ConversionStore` field
- In `Record()`: write to SQLite store with proper `OrgID` (passed via event or context)
- Make `OrgID` a field on `ConversionEvent` (if not already) — check the struct definition
- Keep the in-memory aggregator as an optional fast-path for real-time dashboards, but SQLite is the source of truth

### 3. `internal/api/conversion_handlers.go` (NEW FILE)

Admin endpoint for querying the conversion funnel:

```go
// GET /api/admin/conversion-funnel?org_id=...&from=...&to=...
func (h *ConversionHandlers) HandleConversionFunnel(w http.ResponseWriter, r *http.Request) {
    // Parse query params
    // Call store.FunnelSummary()
    // Return JSON
}
```

- This endpoint should require admin auth
- `org_id` is optional (if omitted, aggregate across all orgs)
- `from` and `to` are optional (default: last 30 days)

### 4. Wire into router

- Add `ConversionHandlers` to router
- Register `GET /api/admin/conversion-funnel` behind admin auth
- Initialize `ConversionStore` in server startup with appropriate DB path

### 5. Wire into server startup

- In `pkg/server/server.go`, create `ConversionStore` during initialization
- DB path: `<baseDataDir>/conversion.db` (single DB for all tenants — org_id column provides isolation)
- Pass store to `Recorder` constructor
- Ensure `store.Close()` is called during shutdown

### 6. Tests

**`internal/license/conversion/store_test.go`** (NEW):
- Test Record + Query roundtrip
- Test idempotency (same key twice → only one row)
- Test FunnelSummary aggregation
- Test org isolation (record for org-a, query org-b → empty)
- Test time range filtering
- Test concurrent writes (5 goroutines writing simultaneously)

## Constraints

- Do NOT remove the existing `WindowedAggregator` integration — it may be used for real-time metrics. Add SQLite alongside it.
- Do NOT change the `ConversionEvent` type's existing fields — only add `OrgID` if it's missing
- SQLite DB should use WAL mode for concurrent read/write
- The conversion funnel endpoint is admin-only (not tenant-scoped — admins see all orgs)
- Keep the store simple — no complex aggregation pipelines, just counts per event type

## Acceptance Checks

```bash
# Must pass
go build ./internal/license/conversion/...
go build ./internal/api/...
go build ./pkg/server/...
go test ./internal/license/conversion/... -count=1 -v
go test ./internal/api/... -count=1 -run "TestConversion"

# Verify tenant-aware recording
grep -n "default" internal/license/conversion/recorder.go
# Expected: "default" should no longer be hardcoded as TenantID
```

## Expected Return

```
status: done | blocked
files_changed: [list with brief why for each]
commands_run: [command + exit code for each]
summary: [what was done]
blockers: [if any]
```
