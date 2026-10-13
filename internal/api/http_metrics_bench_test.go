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

// normalizeRoute is not timed on its own. Since the bounded scanner it takes
// 30-180ns a path on CI runners, and on pull requests that never touched
// route code the BenchmarkNormalizeRoute rows and their parallel variant
// failed 17 of 72 paired comparisons, 6 of those without with_uuid. Go aligns
// functions to 32 bytes, so a change earlier in the binary moves the
// scanner's loops by a cycle per iteration, as it did for the "/" fast path
// and normalizeSegment. Every path shares those loops, so a mixed-path row
// moves almost as far. TestNormalizeRoute_LabelAllocationProfile pins the
// allocation columns, which never moved. BenchmarkFullMiddlewarePath still
// times the normaliser inside the request path, and
// BenchmarkNormalizeRoute_DiscardedSuffix the cost of a long discarded suffix.

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
