package api

import "time"

// API latency SLO targets per critical endpoint.
//
// These define the maximum acceptable p95 latencies for each endpoint under
// representative benchmark conditions (see slo_bench_test.go). Production
// latencies should stay well below these; violations in benchmarks signal a
// performance regression requiring investigation before merge.
//
// Targets are based on baseline measurements from the existing benchmarks in
// router_bench_test.go and store_bench_test.go, with headroom for CI variance.
//
// Endpoint categories:
//   - Chart data (metrics-store/history): Drives the dashboard sparkline/chart
//     rendering path. Must be fast enough for concurrent multi-resource loads.
//   - Stats/metadata (metrics-store/stats): Lightweight queries for dashboard
//     status indicators.
//   - Resource listing (/api/resources): Primary dashboard data source.
//     Includes cache lookup, filtering, sorting, pagination, and JSON encoding.
const (
	// SLOMetricsHistoryStoreP95 is the p95 target for /api/metrics-store/history
	// when serving from the SQLite store (single metric, 1h window, ~500 points).
	SLOMetricsHistoryStoreP95 = 5 * time.Millisecond

	// SLOMetricsHistoryMemoryP95 is the p95 target for /api/metrics-store/history
	// when falling back to the in-memory MetricsHistory (single metric, 1h window).
	SLOMetricsHistoryMemoryP95 = 2 * time.Millisecond

	// SLOMetricsStoreStatsP95 is the p95 target for /api/metrics-store/stats
	// (lightweight SQLite stat queries).
	SLOMetricsStoreStatsP95 = 3 * time.Millisecond

	// SLOResourcesListP95 is the p95 target for GET /api/resources with ~85
	// resources in state (5 nodes + 50 VMs + 30 containers), default pagination
	// (limit=50 per page), including cache lookup, snapshot comparison, filtering,
	// sorting, pagination, pruning, and JSON encoding.
	SLOResourcesListP95 = 3 * time.Millisecond

	// SLOInfrastructureChartsP95 is the p95 target for GET /api/charts/infrastructure
	// with a store-backed 4h window across nodes, docker hosts, and agents.
	// This is the infrastructure summary sparkline hot path.
	SLOInfrastructureChartsP95 = 45 * time.Millisecond

	// SLOWorkloadChartsP95 is the p95 target for GET /api/charts/workloads
	// with a store-backed 4h window across VMs, system containers, and docker
	// containers. This is the workloads summary sparkline hot path.
	SLOWorkloadChartsP95 = 90 * time.Millisecond

	// SLOWorkloadsSummaryChartsP95 is the p95 target for
	// GET /api/charts/workloads-summary with a store-backed 4h window across
	// VMs, system containers, Kubernetes pods, and docker containers. This
	// aggregate endpoint is latency-sensitive because it powers the top-card
	// workload sparklines and blast-radius summaries without per-workload output.
	SLOWorkloadsSummaryChartsP95 = 90 * time.Millisecond
)
