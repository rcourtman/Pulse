# Worker Prompt: W1-B — Make Audit Logging Real and Tenant-Aware

## Task ID: W1-B (P0-1)

## Goal

Replace the stub audit logging initialization with a real `SQLiteLoggerFactory` that creates per-tenant SQLite audit loggers. Decision: always capture audit events to SQLite; gate query/export endpoints behind the `audit_logging` license feature.

## Current State

1. **`pkg/audit/tenant_logger.go`** — `TenantLoggerManager` exists with a `LoggerFactory` interface:
   ```go
   type LoggerFactory interface {
       CreateLogger(dbPath string) (Logger, error)
   }
   ```
   It has a `DefaultLoggerFactory` that creates `ConsoleLogger` instances (no persistence).

2. **`pkg/audit/sqlite_logger.go`** — `NewSQLiteLogger(cfg SQLiteLoggerConfig)` exists and works. Takes `SQLiteLoggerConfig{DataDir: string, ...}`. Creates `audit.db` inside `DataDir/audit/` directory.

3. **Interface mismatch**: `LoggerFactory.CreateLogger(dbPath string)` takes a path string, but `NewSQLiteLogger` takes a config with `DataDir` (a directory). The factory must bridge this.

4. **`internal/api/license_handlers.go:106-140`** — `initAuditLoggerIfLicensed()` is a stub. The `auditOnce.Do` block has TODO comments but never creates a SQLite logger.

5. **`pkg/server/server.go:116-119`** — `TenantAuditManager` is initialized but always gets the default factory (console logger).

## Scope (Exact Files)

### 1. `pkg/audit/sqlite_factory.go` (NEW FILE)

Create `SQLiteLoggerFactory` that implements `LoggerFactory`:

```go
type SQLiteLoggerFactory struct{}

func (f *SQLiteLoggerFactory) CreateLogger(dbPath string) (Logger, error) {
    // dbPath will be something like "/data/orgs/test-org/audit.db"
    // Extract directory from path for SQLiteLoggerConfig.DataDir
    dataDir := filepath.Dir(dbPath)
    cfg := SQLiteLoggerConfig{
        DataDir: dataDir,
    }
    return NewSQLiteLogger(cfg)
}
```

### 2. `pkg/server/server.go`

Wire `SQLiteLoggerFactory` into `TenantAuditManager` initialization:
- Find where `TenantAuditManager` is created (around line 116-119)
- Replace `DefaultLoggerFactory` with `SQLiteLoggerFactory` when a data directory is available
- The data directory is available from `mtPersistence.BaseDataDir()` (already in scope in server.go)

### 3. `internal/api/license_handlers.go`

- **Remove** the stub `initAuditLoggerIfLicensed()` method (lines 106-140) — it's no longer needed since the factory handles logger creation
- **Remove** the `auditOnce sync.Once` field from `LicenseHandlers` struct (line 25)
- Remove the call to `initAuditLoggerIfLicensed` in `getTenantComponents()` (around line 86)

### 4. Gate query/export endpoints

- Find audit query/export routes (likely in `router.go` or a dedicated audit handler file)
- Ensure they are wrapped with `RequireLicenseFeature(handlers, "audit_logging", ...)`
- If they already are, verify. If not, add the gate.
- The key insight: audit events are ALWAYS captured (defense in depth), but only licensed tenants can READ them

### 5. Tests

**`pkg/audit/sqlite_factory_test.go`** (NEW):
- Test that `SQLiteLoggerFactory.CreateLogger()` creates a working SQLite logger
- Test that events written through the factory-created logger are persisted and queryable
- Test HMAC signing works on factory-created loggers

**Tenant isolation test** (in `pkg/audit/` or `internal/api/`):
- Create two tenant loggers via the factory (org-a, org-b)
- Write events to org-a's logger
- Query org-b's logger — verify org-a's events are NOT visible
- Write events to org-b's logger
- Query each — verify isolation

## Constraints

- Do NOT change the `Logger` interface in `audit.go`
- Do NOT change `NewSQLiteLogger` signature
- Do NOT change the `LoggerFactory` interface (implement it, don't modify it)
- Audit events must be captured for ALL tenants regardless of license status
- Only query/export API endpoints are gated behind `audit_logging` feature
- Per-tenant SQLite DBs go in `<orgDir>/audit/audit.db` (this is what `NewSQLiteLogger` already does when given `DataDir`)
- For the "default" org, the `TenantLoggerManager.GetLogger("default")` already returns the global logger — leave this behavior intact

## Acceptance Checks

```bash
# Must pass
go build ./pkg/audit/...
go build ./internal/api/...
go build ./pkg/server/...
go test ./pkg/audit/... -count=1 -v
go test ./internal/api/... -count=1 -run "TestHostedLifecycle"

# Verify the stub is removed
grep -n "initAuditLoggerIfLicensed" internal/api/license_handlers.go
# Expected: no matches (or only the removal)
```

## Expected Return

```
status: done | blocked
files_changed: [list with brief why for each]
commands_run: [command + exit code for each]
summary: [what was done]
blockers: [if any]
```
