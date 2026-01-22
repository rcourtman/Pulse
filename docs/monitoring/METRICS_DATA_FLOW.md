# Metrics Data Flow (Sparklines vs History)

## Quick summary
- Sparklines/trends toggle: client ring buffer + short-term in-memory server history via `/api/charts` (fast, not durable).
- Guest History tab: persistent SQLite metrics store via `/api/metrics-store/history` (durable, long-range, downsampled).

## Path A: Sparklines ("Trends" toggle)
1. Server polling writes to in-memory history: `monitor.go` -> `metricsHistory.AddGuestMetric/AddNodeMetric`.
2. `/api/charts` (`handleCharts`) reads from `metricsHistory` via `monitor.GetGuestMetrics/GetNodeMetrics`.
3. Client toggles to sparklines: `metricsViewMode.ts` -> `seedFromBackend()` -> `ChartsAPI.getCharts()` -> ring buffer in `metricsHistory.ts`.
4. While in sparklines mode, `metricsSampler.ts` samples websocket state every 30s and appends to the ring buffer; localStorage saves periodically.

## Path B: Guest drawer History tab
1. Server polling writes to SQLite store: `monitor.go` -> `metricsStore.Write(resourceType, ...)`.
2. `/api/metrics-store/history` (`handleMetricsHistory`) queries `metrics.Store` (`Query/QueryAll`) with tiered downsampling and license gating.
3. `GuestDrawer` History charts call `ChartsAPI.getMetricsHistory()` for CPU/memory/disk and ranges `24h/7d/30d/90d`.

## Audit notes / inconsistencies
- In-memory retention is `NewMetricsHistory(1000, 24h)` (`monitor.go`). At 30s samples, 1000 points is ~8.3h, so sparklines now cap at 8h to avoid over-promising.
- Sparkline UI ranges (`15m/1h/4h/8h`) are a subset of `TimeRange` support (`5m/15m/30m/1h/4h/8h/12h/7d`) and differ from History tab ranges (`24h/7d/30d/90d`).
- Sparkline ring buffer keeps 7d locally, but server seeding is effectively ~8h at 30s sampling (1000-point cap); longer spans require staying in sparklines mode without reload.
- Docker resource keys differ: in-memory uses `docker:<id>` (via `handleCharts`), persistent store uses `resourceType=dockerContainer`. Mapping is handled client-side when building metric keys; keep consistent when adding resource types.

## DB-backed `/api/charts` assessment
- Feasible approach: add a `source=metrics-store` param to `/api/charts`, enumerate resources from state, then query `metrics.Store` per resource.
- Cost: `N resources x M metric types` → `N*M` queries + SQLite I/O (single-writer). For large fleets this is likely heavier than the current in-memory path.
- Optimization needed for viability: add a bulk store query keyed by resource type/time range (grouped by `resource_id`, `metric_type`) or cache pre-aggregated slices.
- Recommendation: keep `/api/charts` in-memory for table-wide sparklines; use the metrics-store path for per-resource charts or small, explicit batches.
