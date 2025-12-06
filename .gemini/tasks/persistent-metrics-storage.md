# Task: Persistent Metrics Storage for Sparklines

## Problem
Currently, metrics history for sparklines is stored **in-memory only**. When the Pulse backend restarts, all historical metrics are lost. Users expect to see historical trends even after being away for days.

## Goal
Implement SQLite-based persistent metrics storage that:
- Survives backend restarts
- Provides historical data for sparklines/trends view
- Supports configurable retention periods
- Minimizes disk I/O and storage footprint

## Architecture

### Storage Tiers (Data Rollup)
```
┌─────────────────────────────────────────────────────────┐
│ RAW (5s intervals)     → Keep 2 hours     → ~1,440 pts │
│ MINUTE (1min avg)      → Keep 24 hours    → ~1,440 pts │
│ HOURLY (1hr avg)       → Keep 7 days      → ~168 pts   │
│ DAILY (1day avg)       → Keep 90 days     → ~90 pts    │
└─────────────────────────────────────────────────────────┘
```

### Database Schema
```sql
-- Main metrics table (partitioned by time for efficient pruning)
CREATE TABLE metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource_type TEXT NOT NULL,  -- 'node', 'vm', 'container', 'storage'
    resource_id TEXT NOT NULL,
    metric_type TEXT NOT NULL,    -- 'cpu', 'memory', 'disk'
    value REAL NOT NULL,
    timestamp INTEGER NOT NULL,   -- Unix timestamp in seconds
    tier TEXT DEFAULT 'raw'       -- 'raw', 'minute', 'hourly', 'daily'
);

-- Indexes for efficient queries
CREATE INDEX idx_metrics_lookup ON metrics(resource_type, resource_id, metric_type, tier, timestamp);
CREATE INDEX idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX idx_metrics_tier_time ON metrics(tier, timestamp);
```

### Configuration
```yaml
metrics:
  enabled: true
  database_path: "${PULSE_DATA_DIR}/metrics.db"
  retention:
    raw: 2h        # 2 hours of raw data
    minute: 24h    # 24 hours of 1-minute averages
    hourly: 168h   # 7 days of hourly averages
    daily: 2160h   # 90 days of daily averages
  write_buffer: 100    # Buffer size before batch write
  rollup_interval: 5m  # How often to run rollup job
```

## Implementation Steps

### Phase 1: SQLite Foundation ✅ COMPLETED
- [x] Add SQLite dependency (`modernc.org/sqlite` - pure Go, no CGO)
- [x] Create `internal/metrics/store.go` with:
  - `Store` struct
  - `NewStore(config StoreConfig) (*Store, error)`
  - `Close() error`
  - Schema auto-migration on startup

### Phase 2: Write Path ✅ COMPLETED
- [x] Create `Write(resourceType, resourceID, metricType string, value float64, timestamp time.Time)`
- [x] Implement write buffering (batch inserts every 100 records or 5 seconds)
- [x] Integrate with existing `AddGuestMetric`, `AddNodeMetric` calls in monitor.go and monitor_polling.go
- [x] Add graceful shutdown to flush buffer

### Phase 3: Read Path ✅ COMPLETED
- [x] Create `Query(resourceType, resourceID, metricType string, start, end time.Time) ([]MetricPoint, error)`
- [x] Auto-select appropriate tier based on time range:
  - < 2 hours → raw data
  - 2-24 hours → minute data
  - 1-7 days → hourly data
  - 7+ days → daily data
- [x] Add `/api/metrics-store/stats` endpoint for monitoring

### Phase 4: Rollup & Retention ✅ COMPLETED
- [x] Create background rollup job:
  - Runs every 5 minutes
  - Aggregates raw → minute (AVG, MIN, MAX)
  - Aggregates minute → hourly
  - Aggregates hourly → daily
- [x] Create retention pruning job:
  - Runs every hour
  - Deletes data older than configured retention
- [x] Use SQLite transactions for atomic operations

### Phase 5: Integration
- [ ] Add configuration to `system.json` or new `metrics.json`
- [ ] Add Settings UI for metrics retention config
- [ ] Add database file size monitoring
- [ ] Add vacuum/optimize scheduled job (weekly)

## Files to Create/Modify

### New Files
```
internal/metrics/
├── store.go          # MetricsStore implementation
├── store_test.go     # Unit tests
├── rollup.go         # Rollup/aggregation logic
├── retention.go      # Retention/pruning logic
└── config.go         # Metrics configuration
```

### Files to Modify
```
internal/monitoring/monitor.go       # Initialize MetricsStore, call Write()
internal/monitoring/metrics_history.go # Keep in-memory as cache, backed by SQLite
internal/api/router.go               # Update handleCharts to query from store
internal/config/persistence.go       # Add metrics config persistence
```

## API Changes

### `/api/charts` Query Parameters
```
GET /api/charts?range=1h             # Last hour (raw/minute data)
GET /api/charts?range=24h            # Last 24 hours (minute data)
GET /api/charts?range=7d             # Last 7 days (hourly data)
GET /api/charts?range=30d            # Last 30 days (daily data)
GET /api/charts?start=...&end=...    # Custom range
```

### Response Enhancement
```json
{
  "data": { ... },
  "nodeData": { ... },
  "stats": {
    "oldestDataTimestamp": 1699900000000,
    "tier": "hourly",
    "pointCount": 168
  }
}
```

## Performance Considerations

1. **Write Buffering**: Batch inserts to reduce I/O
2. **WAL Mode**: Enable SQLite WAL for concurrent reads/writes
3. **Prepared Statements**: Reuse for repeated queries
4. **Index Strategy**: Composite index on (resource_type, resource_id, metric_type, tier, timestamp)
5. **Connection Pooling**: Single connection with proper locking for SQLite
6. **Memory Mapping**: Use `PRAGMA mmap_size` for faster reads

## Storage Estimates
For a typical Pulse installation (5 nodes, 50 VMs, 20 containers, 10 storage):
- 85 resources × 3 metrics = 255 metric series
- Raw (2h at 5s): ~86,400 rows → ~10 MB
- Minute (24h): ~367,200 rows → ~40 MB
- Hourly (7d): ~42,840 rows → ~5 MB
- Daily (90d): ~22,950 rows → ~3 MB
- **Total: ~60-100 MB** for comprehensive historical data

## Testing Plan
1. Unit tests for store CRUD operations
2. Unit tests for rollup logic
3. Integration tests with mock monitor
4. Performance tests with 100+ resources
5. Restart resilience tests

## Rollout Plan
1. Implement as opt-in feature (disable by default initially)
2. Add migration path from in-memory to SQLite
3. Test in dev environment for 1 week
4. Enable by default in next minor release

## Definition of Done
- [ ] SQLite metrics storage implemented
- [ ] Data survives backend restart
- [ ] Rollup/retention working correctly
- [ ] Charts endpoint serves historical data
- [ ] Documentation updated
- [ ] Settings UI for retention config
- [ ] Performance validated (no noticeable slowdown)
