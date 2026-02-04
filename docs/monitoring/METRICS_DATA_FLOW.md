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

## Architecture notes
- In-memory retention is `NewMetricsHistory(1000, 24h)` (`monitor.go`). At 10s polling, 1000 points covers ~2.8h of data.
- `/api/charts` uses a two-tier strategy: ranges ≤ 2h are served from the in-memory buffer; longer ranges (4h, 8h, 24h, 7d, 30d) fall back to the SQLite persistent store with LTTB downsampling to ~500 points per metric.
- Frontend sparkline ring buffer keeps up to 8h locally (`metricsHistory.ts`).
- Docker resource keys differ: in-memory uses `docker:<id>`, persistent store uses `resourceType=dockerContainer`. The `GetGuestMetricsForChart` method maps between these automatically.
- History charts in the guest drawer use `/api/metrics-store/history` (SQLite) for ranges `24h/7d/30d/90d`.
