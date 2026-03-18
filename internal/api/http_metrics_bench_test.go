package api

import (
	"fmt"
	"testing"
	"time"
)

// Benchmarks for the HTTP metrics hot path — normalizeRoute and
// recordAPIRequest run on every incoming HTTP request via ErrorHandler
// middleware (middleware.go:57–62). These functions must stay fast to
// avoid adding per-request overhead.

// BenchmarkNormalizeRoute measures route normalization with representative
// API paths of varying complexity.
func BenchmarkNormalizeRoute(b *testing.B) {
	paths := []struct {
		name string
		path string
	}{
		{"simple_api", "/api/resources"},
		{"with_numeric_id", "/api/users/12345"},
		{"with_uuid", "/api/nodes/550e8400-e29b-41d4-a716-446655440000"},
		{"deep_path_truncated", "/api/v1/orgs/123/resources/456/metrics"},
		{"with_query_params", "/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:100&metric=cpu&range=1h"},
		{"root", "/"},
		{"metrics_history", "/api/metrics-store/history"},
	}

	for _, p := range paths {
		b.Run(p.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = normalizeRoute(p.path)
			}
		})
	}
}

// BenchmarkNormalizeRoute_Parallel measures normalizeRoute under concurrent
// access matching the production middleware pattern (many goroutines calling
// normalizeRoute simultaneously for different requests). Uses r.URL.Path
// inputs (no query parameters) matching the real middleware call site.
func BenchmarkNormalizeRoute_Parallel(b *testing.B) {
	paths := []string{
		"/api/resources",
		"/api/users/12345",
		"/api/metrics-store/history",
		"/api/nodes/550e8400-e29b-41d4-a716-446655440000/status",
		"/api/alerts/config",
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = normalizeRoute(paths[i%len(paths)])
			i++
		}
	})
}

// BenchmarkRecordAPIRequest measures the full Prometheus recording pipeline
// for a single HTTP request (histogram observe + counter inc + error counter
// conditional). This is the complete per-request cost.
func BenchmarkRecordAPIRequest(b *testing.B) {
	// Ensure metrics are initialized before timing.
	httpMetricsOnce.Do(initHTTPMetrics)

	cases := []struct {
		name   string
		method string
		route  string
		status int
	}{
		{"success_200", "GET", "/api/resources", 200},
		{"not_found_404", "GET", "/api/missing", 404},
		{"server_error_500", "POST", "/api/settings", 500},
		{"created_201", "POST", "/api/alerts", 201},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			elapsed := 5 * time.Millisecond
			// Use unique route per sub-benchmark to avoid label contention
			// affecting other benchmarks in the same process.
			route := fmt.Sprintf("%s/bench/%s", c.route, c.name)
			// Warmup: pre-create Prometheus label series so the benchmark
			// measures steady-state cost, not first-touch allocation.
			recordAPIRequest(c.method, route, c.status, elapsed)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				recordAPIRequest(c.method, route, c.status, elapsed)
			}
		})
	}
}

// BenchmarkRecordAPIRequest_Parallel measures recordAPIRequest under
// concurrent goroutine load, matching the real middleware hot path where
// many HTTP handlers record metrics simultaneously.
func BenchmarkRecordAPIRequest_Parallel(b *testing.B) {
	httpMetricsOnce.Do(initHTTPMetrics)

	elapsed := 5 * time.Millisecond
	routes := []string{
		"/api/resources/bench/parallel/0",
		"/api/metrics-store/history/bench/parallel/1",
		"/api/alerts/bench/parallel/2",
		"/api/settings/bench/parallel/3",
	}
	statuses := []int{200, 200, 404, 200}

	// Warmup: pre-create all Prometheus label series.
	for i, route := range routes {
		recordAPIRequest("GET", route, statuses[i], elapsed)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % len(routes)
			recordAPIRequest("GET", routes[idx], statuses[idx], elapsed)
			i++
		}
	})
}

// BenchmarkNormalizeSegment measures per-segment normalization for the three
// detection paths: numeric ID, UUID, and long token.
func BenchmarkNormalizeSegment(b *testing.B) {
	segments := []struct {
		name string
		seg  string
	}{
		{"numeric_id", "12345"},
		{"uuid", "550e8400-e29b-41d4-a716-446655440000"},
		{"long_token", "abcdefghijklmnopqrstuvwxyz1234567890abcdef"},
		{"short_name", "resources"},
		{"medium_name", "metrics-store"},
	}

	for _, s := range segments {
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = normalizeSegment(s.seg)
			}
		})
	}
}

// BenchmarkFullMiddlewarePath simulates the complete per-request overhead:
// normalizeRoute + recordAPIRequest, as executed by the ErrorHandler middleware.
// Uses r.URL.Path (no query params) matching the real middleware call site.
func BenchmarkFullMiddlewarePath(b *testing.B) {
	httpMetricsOnce.Do(initHTTPMetrics)

	// In the real middleware (middleware.go:57), normalizeRoute receives
	// r.URL.Path which never contains query parameters.
	path := "/api/metrics-store/history"
	elapsed := 5 * time.Millisecond

	// Warmup: ensure Prometheus label series are pre-created so the
	// benchmark measures steady-state cost, not first-touch allocation.
	route := normalizeRoute(path)
	recordAPIRequest("GET", route, 200, elapsed)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		route := normalizeRoute(path)
		recordAPIRequest("GET", route, 200, elapsed)
	}
}
