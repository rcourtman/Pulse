package api

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// This deliberately keeps the original split/join algorithm and classifiers
// independent of the bounded scanner. In particular, UUID-like historically
// allows hex at the dash positions too; this optimization must not tighten it.
func referenceRouteLabel(path string) string {
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	var labels []string
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			continue
		}
		if len(labels) == 5 {
			break
		}
		numeric := true
		for _, r := range seg {
			if r < '0' || r > '9' {
				numeric = false
			}
		}
		uuid := len(seg) == 36
		for i, r := range seg {
			if r == '-' && (i == 8 || i == 13 || i == 18 || i == 23) {
				continue
			}
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				uuid = false
			}
		}
		switch {
		case numeric:
			seg = ":id"
		case uuid:
			seg = ":uuid"
		case len(seg) > 32:
			seg = ":token"
		}
		labels = append(labels, seg)
	}
	if len(labels) == 0 {
		return "/"
	}
	return "/" + strings.Join(labels, "/")
}

func routeLabelCorpus() []string {
	paths := []string{
		"", "/", "//", "?query/only", "/?query/only", "////?query/only",
		"api/resources", "/api///users/123//", "/a/b/c/d/e///ignored/99?query/only",
		"/a/b/c/d/e?query/only", "/a/b/c/d?e/f", "/café/１２３/١٢٣/node\xff",
		"/api/" + strings.Repeat("a", 32), "/api/" + strings.Repeat("é", 16),
		"/api/" + strings.Repeat("a", 33), "/api/" + strings.Repeat("é", 17),
		"/api/" + strings.Repeat("1", 64), "/api/" + strings.Repeat("a", 36),
		"/api/550E8400-E29B-41D4-A716-446655440000/status",
		"/a/b/c/d/e/" + strings.Repeat("discarded/", 2048),
	}
	segments := []string{"", "api", "v1", "123", "000", "-1", "12.3", "café", "node\xff", "１２３", "١٢٣",
		strings.Repeat("a", 32), strings.Repeat("a", 33), strings.Repeat("1", 36), strings.Repeat("a", 36),
		"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-44665544000!"}
	rng := rand.New(rand.NewSource(2576))
	for i := 0; i < 4096; i++ {
		parts := make([]string, rng.Intn(12))
		for j := range parts {
			parts[j] = segments[rng.Intn(len(segments))]
		}
		path := strings.Join(parts, "/")
		if i%2 == 0 {
			path = "/" + path + "/"
		}
		if i%3 == 0 {
			path += "?secret=never/a/label"
		}
		paths = append(paths, path)
	}
	return paths
}

func TestNormalizeRoute_Compatibility(t *testing.T) {
	for _, path := range routeLabelCorpus() {
		if got, want := normalizeRoute(path), referenceRouteLabel(path); got != want {
			t.Fatalf("normalizeRoute(%q) = %q, original = %q", path, got, want)
		}
	}
}

func FuzzNormalizeRouteCompatibility(f *testing.F) {
	for _, path := range routeLabelCorpus()[:20] {
		f.Add(path)
	}
	f.Fuzz(func(t *testing.T, path string) {
		if got, want := normalizeRoute(path), referenceRouteLabel(path); got != want {
			t.Fatalf("normalizeRoute(%q) = %q, original = %q", path, got, want)
		}
	})
}

func TestNormalizeRoute_BoundedAllocations(t *testing.T) {
	if !measureAllocsInIsolation(t) {
		return
	}
	for _, path := range []string{
		"/api/resources", "/api/users/12345", "/api/nodes/550e8400-e29b-41d4-a716-446655440000",
		"/api/v1/orgs/123/resources/456/metrics", "/api/metrics-store/history?secret=never/a/label",
		"/api/metrics-store/history", "/café/１２３/node\xff", "/auth/" + strings.Repeat("a", 64),
		"/a/b/c/d/e/" + strings.Repeat("discarded/", 2048),
	} {
		if allocs := testing.AllocsPerRun(100, func() { normalizeRouteSink = normalizeRoute(path) }); allocs > 1 {
			t.Errorf("normalizeRoute(%q) allocated %v times per call, want at most 1", path, allocs)
		}
	}
	for _, path := range []string{"", "/", "////", "?query/only", "/?query/only"} {
		if allocs := testing.AllocsPerRun(100, func() { normalizeRouteSink = normalizeRoute(path) }); allocs != 0 {
			t.Errorf("root label for %q allocated %v times per call, want 0", path, allocs)
		}
	}
}

func TestNormalizeRoute_ConcurrentAndRetainedLabels(t *testing.T) {
	paths := routeLabelCorpus()[:20]
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			labels := make([]string, 0, 64*len(paths))
			for range 64 {
				for _, path := range paths {
					labels = append(labels, normalizeRoute(path))
				}
			}
			for i, label := range labels {
				if want := referenceRouteLabel(paths[i%len(paths)]); label != want {
					t.Errorf("retained label %q changed; want %q", label, want)
					return
				}
			}
		})
	}
	workers.Wait()
}

func TestErrorHandler_NormalizedRouteMetrics(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)
	cases := []struct {
		path, route, method string
		status              int
	}{
		{"/metricroute-contract/users/", "/metricroute-contract/users/:id", "GET", 200},
		{"/metricroute-contract/auth/", "/metricroute-contract/auth/:token", "POST", 500},
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(apiRequestTotal, apiRequestDuration, apiRequestErrors)
	const requests = 64
	for _, c := range cases {
		status := strconv.Itoa(c.status)
		before := httpCounterValue(t, apiRequestTotal, c.method, c.route, status)
		observations := httpHistogramSampleCount(t, apiRequestDuration, c.method, c.route, status)
		errors := httpCounterValue(t, apiRequestErrors, c.method, c.route, classifyStatus(c.status))
		handler := ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
		}))
		var workers sync.WaitGroup
		for i := range requests {
			workers.Go(func() {
				id := strconv.Itoa(i + 1)
				if c.status == 500 {
					id = strings.Repeat("token", 8) + id
				}
				req := httptest.NewRequest(c.method, c.path+id+"?secret="+id, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != c.status || rec.Header().Get("X-Request-ID") == "" {
					t.Errorf("response lost status or request ID: %d", rec.Code)
				}
			})
		}
		workers.Wait()
		if delta := httpCounterValue(t, apiRequestTotal, c.method, c.route, status) - before; delta != requests {
			t.Errorf("%s request count delta = %v, want %d", c.route, delta, requests)
		}
		if delta := httpHistogramSampleCount(t, apiRequestDuration, c.method, c.route, status) - observations; delta != requests {
			t.Errorf("%s histogram count delta = %v, want %d", c.route, delta, requests)
		}
		wantErrors := float64(0)
		if c.status >= 400 {
			wantErrors = requests
		}
		if delta := httpCounterValue(t, apiRequestErrors, c.method, c.route, classifyStatus(c.status)) - errors; delta != wantErrors {
			t.Errorf("%s error count delta = %v, want %v", c.route, delta, wantErrors)
		}
	}
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	series := 0
	for _, family := range metrics {
		if family.GetName() != "pulse_http_requests_total" {
			continue
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "route" && strings.HasPrefix(label.GetValue(), "/metricroute-contract/") {
					series++
					if label.GetValue() != cases[0].route && label.GetValue() != cases[1].route {
						t.Errorf("unexpected raw-ID/token/query route label %q", label.GetValue())
					}
				}
			}
		}
	}
	if series != len(cases) {
		t.Errorf("%d distinct requests created %d series, want %d", requests*len(cases), series, len(cases))
	}
}

func BenchmarkNormalizeRoute_DiscardedSuffix(b *testing.B) {
	for _, count := range []int{8, 2048} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			path := "/a/b/c/d/e/" + strings.Repeat("discarded/", count)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = normalizeRoute(path)
			}
		})
	}
}
