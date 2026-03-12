package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// TestLoad_500Node_ConcurrentResources validates that the /api/resources
// endpoint sustains acceptable latency under concurrent load with 500 nodes
// and ~2500 VMs in state, simulating a large self-hosted deployment.
//
// The test launches 50 concurrent goroutines each making serial requests over
// a 2-second window. It measures p50/p95/p99 and total throughput.
func TestLoad_500Node_ConcurrentResources(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	state := buildLargeDeploymentState(t, 500)

	cfg := &config.Config{DataPath: t.TempDir()}
	handlers := NewResourceHandlers(cfg)
	handlers.SetStateProvider(&loadTestStateProvider{state: state})

	// Warm the cache with an initial request.
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rec := httptest.NewRecorder()
	handlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("warmup failed: status %d, body: %s", rec.Code, rec.Body.String())
	}
	var warmupResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &warmupResp); err != nil {
		t.Fatalf("warmup unmarshal: %v", err)
	}
	meta, _ := warmupResp["meta"].(map[string]interface{})
	total, _ := meta["total"].(float64)
	// 500 nodes + 2500 VMs = 3000 resources exactly.
	if int(total) != 3000 {
		t.Fatalf("warmup: expected 3000 total resources, got %v", total)
	}
	t.Logf("state populated: %.0f total resources", total)

	const (
		concurrency = 50
		duration    = 2 * time.Second
	)

	var (
		latencies  = make([][]time.Duration, concurrency)
		errors     int64
		totalCount int64
	)

	// Use a barrier to start all goroutines simultaneously, ensuring wall-clock
	// timing starts after goroutine launch overhead.
	var ready sync.WaitGroup
	ready.Add(concurrency)
	var wg sync.WaitGroup

	for g := 0; g < concurrency; g++ {
		wg.Add(1)
		g := g
		latencies[g] = make([]time.Duration, 0, 200)
		go func() {
			defer wg.Done()
			ready.Done()
			ready.Wait() // All goroutines wait here until everyone is launched
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				reqStart := time.Now()
				req := httptest.NewRequest(http.MethodGet, "/api/resources?limit=50", nil)
				rec := httptest.NewRecorder()
				handlers.HandleListResources(rec, req)
				elapsed := time.Since(reqStart)
				if rec.Code != http.StatusOK {
					atomic.AddInt64(&errors, 1)
					continue
				}
				latencies[g] = append(latencies[g], elapsed)
				atomic.AddInt64(&totalCount, 1)
			}
		}()
	}
	ready.Wait()
	start := time.Now()
	wg.Wait()
	wallTime := time.Since(start)

	if errors > 0 {
		t.Errorf("got %d error responses", errors)
	}

	all := mergeLatencies(latencies)
	if len(all) == 0 {
		t.Fatal("no successful requests recorded")
	}

	p50 := percentile(all, 0.50)
	p95 := percentile(all, 0.95)
	p99 := percentile(all, 0.99)
	rps := float64(totalCount) / wallTime.Seconds()

	t.Logf("resources 500-node load: %d requests in %v (%.1f rps)", totalCount, wallTime, rps)
	t.Logf("  p50=%v  p95=%v  p99=%v", p50, p95, p99)

	// At 500-node scale (3000 resources) with 50 concurrent goroutines, the
	// resource handler processes snapshot comparison, filtering, sorting,
	// pagination, and JSON serialization under mutex contention. The p95 budget
	// accounts for this in-process overhead — it catches gross regressions
	// (e.g., O(n²) algorithms, lock contention bugs) without being flaky.
	// Budget set to 3s to accommodate -cpu=1 CI runners.
	if p95 > 3*time.Second {
		t.Errorf("p95 latency %v exceeds 3s budget for 500-node concurrent resources load", p95)
	}
	// Throughput includes in-flight request overrun past the 2s window, so
	// actual wall time may exceed 2s. RPS = completed requests / wall time,
	// which measures sustained throughput under contention (standard approach).
	// Floor is set conservatively for -cpu=1 CI runners (~47 rps observed).
	if rps < 30 {
		t.Errorf("throughput %.1f rps is below minimum 30 rps threshold", rps)
	}
}

// TestLoad_500Node_ConcurrentMetricsHistory validates that /api/metrics-store/history
// sustains acceptable latency when 50 concurrent goroutines query chart data for
// resources across a 500-node deployment.
func TestLoad_500Node_ConcurrentMetricsHistory(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newLoadTestMetricsStore(t)
	const (
		numResources = 100 // 100 VMs with history data (representative subset)
		numPoints    = 200
	)
	metricTypes := []string{"cpu", "memory"}
	ids := seedLoadTestMetrics(t, store, "vm", metricTypes, numResources, numPoints)

	state := models.NewState()
	monitor := &monitoring.Monitor{}
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	// Sanity check: verify the store path is exercised and returns data.
	checkURL := "/api/metrics-store/history?resourceType=vm&resourceId=" + ids[0] + "&metric=cpu&range=1h"
	sanityReq := httptest.NewRequest(http.MethodGet, checkURL, nil)
	sanityRec := httptest.NewRecorder()
	router.handleMetricsHistory(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d, body: %s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp metricsHistoryResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if sanityResp.Source != "store" {
		t.Fatalf("sanity check: expected source=store, got %q", sanityResp.Source)
	}
	if len(sanityResp.Points) == 0 {
		t.Fatal("sanity check: expected non-empty points")
	}

	// Precompute URLs to avoid string building in timed loop.
	urls := make([]string, len(ids))
	for i, id := range ids {
		urls[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=1h"
	}

	const (
		concurrency = 50
		duration    = 2 * time.Second
	)

	var (
		latencies  = make([][]time.Duration, concurrency)
		errors     int64
		totalCount int64
	)

	var ready sync.WaitGroup
	ready.Add(concurrency)
	var wg sync.WaitGroup

	for g := 0; g < concurrency; g++ {
		wg.Add(1)
		g := g
		latencies[g] = make([]time.Duration, 0, 200)
		go func() {
			defer wg.Done()
			ready.Done()
			ready.Wait()
			deadline := time.Now().Add(duration)
			idx := g // Each goroutine starts with a different resource
			for time.Now().Before(deadline) {
				reqStart := time.Now()
				req := httptest.NewRequest(http.MethodGet, urls[idx%len(urls)], nil)
				rec := httptest.NewRecorder()
				router.handleMetricsHistory(rec, req)
				elapsed := time.Since(reqStart)
				if rec.Code != http.StatusOK {
					atomic.AddInt64(&errors, 1)
					continue
				}
				latencies[g] = append(latencies[g], elapsed)
				atomic.AddInt64(&totalCount, 1)
				idx++
			}
		}()
	}
	ready.Wait()
	start := time.Now()
	wg.Wait()
	wallTime := time.Since(start)

	if errors > 0 {
		t.Errorf("got %d error responses", errors)
	}

	all := mergeLatencies(latencies)
	if len(all) == 0 {
		t.Fatal("no successful requests recorded")
	}

	p50 := percentile(all, 0.50)
	p95 := percentile(all, 0.95)
	p99 := percentile(all, 0.99)
	rps := float64(totalCount) / wallTime.Seconds()

	t.Logf("metrics-history 500-node load: %d requests in %v (%.1f rps)", totalCount, wallTime, rps)
	t.Logf("  p50=%v  p95=%v  p99=%v", p50, p95, p99)

	// Under concurrent SQLite reads (WAL mode), p95 should stay under 200ms.
	// This budget accounts for in-process test overhead with 50 goroutines
	// contending on a single SQLite file.
	if p95 > 200*time.Millisecond {
		t.Errorf("p95 latency %v exceeds 200ms budget for concurrent metrics-history load", p95)
	}
	if rps < 500 {
		t.Errorf("throughput %.0f rps is below minimum 500 rps threshold", rps)
	}
}

// TestLoad_500Node_MixedEndpoints simulates a realistic dashboard load pattern:
// concurrent requests hitting /api/resources, /api/metrics-store/history, and
// /api/metrics-store/stats simultaneously, as happens when a user opens a
// dashboard showing a 500-node fleet.
func TestLoad_500Node_MixedEndpoints(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	state := buildLargeDeploymentState(t, 500)

	store := newLoadTestMetricsStore(t)
	metricTypes := []string{"cpu", "memory"}
	ids := seedLoadTestMetrics(t, store, "vm", metricTypes, 100, 200)

	monitor := &monitoring.Monitor{}
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	routerObj := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	resourceHandlers := NewResourceHandlers(cfg)
	resourceHandlers.SetStateProvider(&loadTestStateProvider{state: state})

	// Warm caches.
	warmReq := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	warmRec := httptest.NewRecorder()
	resourceHandlers.HandleListResources(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		t.Fatalf("warmup resources failed: status %d", warmRec.Code)
	}

	// Precompute history URLs.
	historyURLs := make([]string, len(ids))
	for i, id := range ids {
		historyURLs[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=1h"
	}

	const duration = 2 * time.Second

	type endpointResult struct {
		name      string
		latencies []time.Duration
		errors    int64
		count     int64
	}

	var histIdx int64 // test-local counter for round-robin
	var mu sync.Mutex

	// 3 endpoint groups, each with their own concurrency.
	groups := []struct {
		name        string
		concurrency int
		fn          func() int // returns HTTP status code
	}{
		{
			name:        "resources",
			concurrency: 20,
			fn: func() int {
				req := httptest.NewRequest(http.MethodGet, "/api/resources?limit=50", nil)
				rec := httptest.NewRecorder()
				resourceHandlers.HandleListResources(rec, req)
				return rec.Code
			},
		},
		{
			name:        "metrics-history",
			concurrency: 25,
			fn: func() int {
				idx := int(atomic.AddInt64(&histIdx, 1))
				req := httptest.NewRequest(http.MethodGet, historyURLs[idx%len(historyURLs)], nil)
				rec := httptest.NewRecorder()
				routerObj.handleMetricsHistory(rec, req)
				return rec.Code
			},
		},
		{
			name:        "metrics-stats",
			concurrency: 5,
			fn: func() int {
				req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil)
				rec := httptest.NewRecorder()
				routerObj.handleMetricsStoreStats(rec, req)
				return rec.Code
			},
		},
	}

	results := make([]endpointResult, len(groups))
	var ready sync.WaitGroup
	totalWorkers := 0
	for _, grp := range groups {
		totalWorkers += grp.concurrency
	}
	ready.Add(totalWorkers)
	var wg sync.WaitGroup

	for gi, grp := range groups {
		results[gi].name = grp.name
		for c := 0; c < grp.concurrency; c++ {
			wg.Add(1)
			gi := gi
			fn := grp.fn
			go func() {
				defer wg.Done()
				ready.Done()
				ready.Wait()
				deadline := time.Now().Add(duration)
				var localLats []time.Duration
				var localErrs int64
				for time.Now().Before(deadline) {
					reqStart := time.Now()
					code := fn()
					elapsed := time.Since(reqStart)
					if code != http.StatusOK {
						localErrs++
						continue
					}
					localLats = append(localLats, elapsed)
				}
				mu.Lock()
				results[gi].latencies = append(results[gi].latencies, localLats...)
				results[gi].errors += localErrs
				results[gi].count += int64(len(localLats))
				mu.Unlock()
			}()
		}
	}
	ready.Wait()
	start := time.Now()
	wg.Wait()
	wallTime := time.Since(start)

	t.Logf("mixed-endpoint 500-node load test completed in %v", wallTime)

	var totalErrors int64
	for _, r := range results {
		if len(r.latencies) == 0 {
			t.Errorf("[%s] no successful requests", r.name)
			continue
		}
		p50 := percentile(r.latencies, 0.50)
		p95 := percentile(r.latencies, 0.95)
		p99 := percentile(r.latencies, 0.99)
		rps := float64(r.count) / wallTime.Seconds()
		t.Logf("  [%s] %d requests (%.1f rps) p50=%v p95=%v p99=%v errors=%d",
			r.name, r.count, rps, p50, p95, p99, r.errors)
		totalErrors += r.errors
	}

	if totalErrors > 0 {
		t.Errorf("total %d error responses across all endpoints", totalErrors)
	}

	// Under mixed concurrent load (50 goroutines across 3 endpoint types
	// sharing a single SQLite store + in-memory state), p95 per endpoint
	// should stay under 3s. This catches gross regressions (lock inversions,
	// O(n²) serialization) without being flaky on -cpu=1 CI runners.
	for _, r := range results {
		if len(r.latencies) == 0 {
			continue
		}
		p95 := percentile(r.latencies, 0.95)
		if p95 > 3*time.Second {
			t.Errorf("[%s] p95=%v exceeds 3s budget under mixed load", r.name, p95)
		}
	}

	// Minimum request volume per endpoint group — ensures throughput didn't
	// collapse under contention. Counts include in-flight overrun past the
	// nominal 2s window (RPS = count / wallTime, standard approach).
	// The mixed-load metrics-history floor is lower because each request does a
	// real store query plus canonical target resolution while contending with
	// resource and stats endpoints on the same process.
	minCounts := map[string]int64{
		"resources":       50,
		"metrics-history": 30,
		"metrics-stats":   10,
	}
	for _, r := range results {
		if minCount, ok := minCounts[r.name]; ok && r.count < minCount {
			t.Errorf("[%s] completed only %d requests, expected at least %d", r.name, r.count, minCount)
		}
	}
}

// --- Load test helpers ---

// buildLargeDeploymentState creates a State with exactly numNodes nodes and
// 5 VMs per node, simulating a large deployment. Nodes are distributed evenly
// across 10 PVE instances; any remainder is spread one-per-instance to the
// first N instances (where N = numNodes % 10).
func buildLargeDeploymentState(t *testing.T, numNodes int) *models.State {
	t.Helper()
	state := models.NewState()

	const numInstances = 10
	basePerInstance := numNodes / numInstances
	remainder := numNodes % numInstances

	nodesSoFar := 0
	for inst := 0; inst < numInstances; inst++ {
		instanceName := fmt.Sprintf("pve%d", inst)
		count := basePerInstance
		if inst < remainder {
			count++ // Distribute remainder across first N instances
		}
		if count == 0 {
			continue
		}

		nodes := make([]models.Node, count)
		for n := 0; n < count; n++ {
			globalIdx := nodesSoFar + n
			nodes[n] = models.Node{
				ID:       fmt.Sprintf("%s:node%d", instanceName, n),
				Name:     fmt.Sprintf("node-%d", globalIdx),
				Instance: instanceName,
				Status:   "online",
				CPU:      float64(globalIdx%80+10) / 100.0,
				Memory:   models.Memory{Usage: float64(globalIdx%60 + 20), Total: 64 << 30, Used: 32 << 30},
				Disk:     models.Disk{Usage: float64(globalIdx%40 + 30), Total: 500 << 30, Used: 250 << 30},
			}
		}
		state.UpdateNodesForInstance(instanceName, nodes)

		// 5 VMs per node.
		vms := make([]models.VM, count*5)
		for v := range vms {
			nodeIdx := v / 5
			vmGlobalIdx := (nodesSoFar+nodeIdx)*5 + (v % 5)
			vms[v] = models.VM{
				ID:       fmt.Sprintf("%s:node%d:%d", instanceName, nodeIdx, 1000+vmGlobalIdx),
				VMID:     1000 + vmGlobalIdx,
				Name:     fmt.Sprintf("vm-%d", vmGlobalIdx),
				Node:     fmt.Sprintf("node%d", nodeIdx),
				Instance: instanceName,
				Status:   "running",
				Type:     "qemu",
				CPU:      float64(vmGlobalIdx%80+10) / 100.0,
				Memory:   models.Memory{Usage: float64(vmGlobalIdx%60 + 20), Total: 4 << 30, Used: 2 << 30},
				Disk:     models.Disk{Usage: float64(vmGlobalIdx%40 + 30), Total: 50 << 30, Used: 25 << 30},
			}
		}
		state.UpdateVMsForInstance(instanceName, vms)

		nodesSoFar += count
	}

	return state
}

// newLoadTestMetricsStore creates an ephemeral metrics store for load tests.
func newLoadTestMetricsStore(t *testing.T) *metrics.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := metrics.DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "load-test.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 50_000
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// seedLoadTestMetrics writes test metrics data (mirrors seedTestMetrics but
// for load test scale).
func seedLoadTestMetrics(t *testing.T, store *metrics.Store, resourceType string, metricTypes []string, numResources, numPoints int) []string {
	t.Helper()
	base := time.Now().Add(-50 * time.Minute)
	ids := make([]string, numResources)

	batch := make([]metrics.WriteMetric, 0, numResources*numPoints*len(metricTypes))
	for r := 0; r < numResources; r++ {
		id := fmt.Sprintf("%s-load-%d", resourceType, r)
		ids[r] = id
		for _, mt := range metricTypes {
			for p := 0; p < numPoints; p++ {
				batch = append(batch, metrics.WriteMetric{
					ResourceType: resourceType,
					ResourceID:   id,
					MetricType:   mt,
					Value:        float64(p % 100),
					Timestamp:    base.Add(time.Duration(p) * 15 * time.Second),
					Tier:         metrics.TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)
	return ids
}

// mergeLatencies flattens per-goroutine latency slices into a single slice.
func mergeLatencies(perGoroutine [][]time.Duration) []time.Duration {
	total := 0
	for _, s := range perGoroutine {
		total += len(s)
	}
	merged := make([]time.Duration, 0, total)
	for _, s := range perGoroutine {
		merged = append(merged, s...)
	}
	return merged
}

// loadTestStateProvider implements StateProvider for load tests.
type loadTestStateProvider struct {
	state *models.State
}

func (p *loadTestStateProvider) ReadSnapshot() models.StateSnapshot {
	return p.state.GetSnapshot()
}
