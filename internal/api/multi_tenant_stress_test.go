package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// TestMultiTenant_ConcurrentAPIStress exercises the full API stack under
// concurrent multi-org load: 20 organizations each with their own metrics
// store and state, hit simultaneously by multiple goroutines across three
// endpoint types (/api/resources, /api/metrics-store/history, /api/metrics-store/stats).
//
// This validates:
//   - Per-tenant SQLite store isolation under concurrent reads across 20 orgs
//   - No cross-tenant data leakage (org A's metrics don't appear in org B's queries)
//   - Handler latency and throughput under multi-org contention (60 goroutines)
func TestMultiTenant_ConcurrentAPIStress(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	const (
		numOrgs            = 20
		workersPerOrg      = 3 // 3 workers per org = 60 total goroutines
		duration           = 2 * time.Second
		metricsPerOrg      = 10 // 10 VMs with metrics per org
		metricsPointsPerVM = 100
		testTimeout        = 30 * time.Second
	)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	// Initialize default org persistence (required by LicenseHandlers).
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init default persistence: %v", err)
	}

	// Build per-org infrastructure: state, metrics store, monitor.
	type orgSetup struct {
		orgID      string
		state      *models.State
		store      *metrics.Store
		monitor    *monitoring.Monitor
		metricIDs  []string
		historyURL string // precomputed URL for first resource
	}

	orgs := make([]orgSetup, numOrgs)
	for i := 0; i < numOrgs; i++ {
		orgID := fmt.Sprintf("stress-org-%03d", i)

		// Create org persistence directory.
		if _, err := mtp.GetPersistence(orgID); err != nil {
			t.Fatalf("failed to init persistence for %s: %v", orgID, err)
		}

		// Build state with a small fleet per org (5 nodes + 25 VMs = 30 resources).
		state := buildMultiTenantState(t, orgID, 5)

		// Create per-tenant metrics store.
		store := newOrgMetricsStore(t, orgID)
		metricTypes := []string{"cpu", "memory"}
		ids := seedOrgMetrics(t, store, orgID, metricTypes, metricsPerOrg, metricsPointsPerVM)

		// Create a monitor with the org's state and store.
		monitor := &monitoring.Monitor{}
		setTestUnexportedField(t, monitor, "state", state)
		setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
		setTestUnexportedField(t, monitor, "metricsStore", store)

		historyURL := "/api/metrics-store/history?resourceType=vm&resourceId=" + ids[0] + "&metric=cpu&range=1h"

		orgs[i] = orgSetup{
			orgID:      orgID,
			state:      state,
			store:      store,
			monitor:    monitor,
			metricIDs:  ids,
			historyURL: historyURL,
		}
	}

	// Build a MultiTenantMonitor stub that returns our pre-built monitors.
	// We use a pre-populated MTM since we don't want real Proxmox connections.
	monitorMap := make(map[string]*monitoring.Monitor, numOrgs)
	for _, org := range orgs {
		monitorMap[org.orgID] = org.monitor
	}
	mtm := newPrePopulatedMTM(monitorMap)

	// Build a default (legacy) monitor for the Router.
	defaultState := models.NewState()
	defaultMonitor := &monitoring.Monitor{}
	setTestUnexportedField(t, defaultMonitor, "state", defaultState)
	setTestUnexportedField(t, defaultMonitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	// Build Router with multi-tenant support.
	router := &Router{
		monitor:         defaultMonitor,
		mtMonitor:       mtm,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	// Build a shared ResourceHandlers with a tenant state provider.
	tenantStates := make(map[string]*models.State, numOrgs)
	for _, org := range orgs {
		tenantStates[org.orgID] = org.state
	}
	sharedResourceCfg := &config.Config{DataPath: t.TempDir()}
	sharedResourceHandlers := NewResourceHandlers(sharedResourceCfg)
	sharedResourceHandlers.SetTenantStateProvider(&multiTenantStateProvider{states: tenantStates})

	// Sanity check: verify each org's metrics store returns data and is isolated.
	for i, org := range orgs {
		ctx := context.WithValue(context.Background(), OrgIDContextKey, org.orgID)
		req := httptest.NewRequest(http.MethodGet, org.historyURL, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		router.handleMetricsHistory(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("org %s sanity check failed: status %d, body: %s", org.orgID, rec.Code, rec.Body.String())
		}
		var resp metricsHistoryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("org %s sanity unmarshal: %v", org.orgID, err)
		}
		if resp.Source != "store" {
			t.Fatalf("org %s: expected source=store, got %q", org.orgID, resp.Source)
		}
		if len(resp.Points) == 0 {
			t.Fatalf("org %s: expected non-empty points", org.orgID)
		}
		// Quick isolation check: query org i with org i+1's resource ID must return 200
		// with zero points. A non-200 or decode failure is also a test failure, since
		// it means we can't prove isolation (the handler should always return 200 with
		// an empty result, not error out).
		if i < numOrgs-1 {
			crossURL := "/api/metrics-store/history?resourceType=vm&resourceId=" + orgs[i+1].metricIDs[0] + "&metric=cpu&range=1h"
			crossReq := httptest.NewRequest(http.MethodGet, crossURL, nil).WithContext(ctx)
			crossRec := httptest.NewRecorder()
			router.handleMetricsHistory(crossRec, crossReq)
			if crossRec.Code != http.StatusOK {
				t.Fatalf("org %s cross-tenant query returned status %d (expected 200 with empty points)",
					org.orgID, crossRec.Code)
			}
			var crossResp metricsHistoryResponse
			if err := json.Unmarshal(crossRec.Body.Bytes(), &crossResp); err != nil {
				t.Fatalf("org %s cross-tenant query unmarshal: %v", org.orgID, err)
			}
			if len(crossResp.Points) > 0 {
				t.Fatalf("ISOLATION VIOLATION: org %s returned %d points for org %s resource %s",
					org.orgID, len(crossResp.Points), orgs[i+1].orgID, orgs[i+1].metricIDs[0])
			}
		}
	}

	// Concurrent stress test: 3 workers per org × 20 orgs = 60 goroutines.
	// Each worker randomly hits one of three endpoints for its assigned org.
	type orgResult struct {
		orgID          string
		resourceLats   []time.Duration
		historyLats    []time.Duration
		statsLats      []time.Duration
		resourceErrors int64
		historyErrors  int64
		statsErrors    int64
		resourceCount  int64
		historyCount   int64
		statsCount     int64
	}

	results := make([]orgResult, numOrgs)
	for i, org := range orgs {
		results[i].orgID = org.orgID
	}

	totalWorkers := numOrgs * workersPerOrg
	var ready sync.WaitGroup
	ready.Add(totalWorkers)
	var wg sync.WaitGroup

	var mu sync.Mutex // Protects per-org result slices.

	for orgIdx := 0; orgIdx < numOrgs; orgIdx++ {
		org := orgs[orgIdx]

		// Precompute history URLs for this org (round-robin across resources).
		historyURLs := make([]string, len(org.metricIDs))
		for j, id := range org.metricIDs {
			historyURLs[j] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=1h"
		}

		for w := 0; w < workersPerOrg; w++ {
			wg.Add(1)
			workerOrgIdx := orgIdx
			workerSeed := int64(orgIdx*1000 + w)
			go func() {
				defer wg.Done()
				ready.Done()
				ready.Wait() // Barrier: all goroutines start simultaneously.

				rng := rand.New(rand.NewSource(workerSeed))
				deadline := time.Now().Add(duration)

				var localResourceLats, localHistoryLats, localStatsLats []time.Duration
				var localResourceErrs, localHistoryErrs, localStatsErrs int64

				for time.Now().Before(deadline) {
					// Randomly choose an endpoint (weighted: 40% resources, 40% history, 20% stats).
					roll := rng.Intn(10)
					ctx := context.WithValue(context.Background(), OrgIDContextKey, org.orgID)

					switch {
					case roll < 4: // Resources
						start := time.Now()
						req := httptest.NewRequest(http.MethodGet, "/api/resources?limit=50", nil).WithContext(ctx)
						rec := httptest.NewRecorder()
						sharedResourceHandlers.HandleListResources(rec, req)
						elapsed := time.Since(start)
						if rec.Code != http.StatusOK {
							localResourceErrs++
						} else {
							localResourceLats = append(localResourceLats, elapsed)
						}

					case roll < 8: // Metrics history
						idx := rng.Intn(len(historyURLs))
						start := time.Now()
						req := httptest.NewRequest(http.MethodGet, historyURLs[idx], nil).WithContext(ctx)
						rec := httptest.NewRecorder()
						router.handleMetricsHistory(rec, req)
						elapsed := time.Since(start)
						if rec.Code != http.StatusOK {
							localHistoryErrs++
						} else {
							localHistoryLats = append(localHistoryLats, elapsed)
						}

					default: // Stats
						start := time.Now()
						req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil).WithContext(ctx)
						rec := httptest.NewRecorder()
						router.handleMetricsStoreStats(rec, req)
						elapsed := time.Since(start)
						if rec.Code != http.StatusOK {
							localStatsErrs++
						} else {
							localStatsLats = append(localStatsLats, elapsed)
						}
					}
				}

				mu.Lock()
				results[workerOrgIdx].resourceLats = append(results[workerOrgIdx].resourceLats, localResourceLats...)
				results[workerOrgIdx].historyLats = append(results[workerOrgIdx].historyLats, localHistoryLats...)
				results[workerOrgIdx].statsLats = append(results[workerOrgIdx].statsLats, localStatsLats...)
				results[workerOrgIdx].resourceErrors += localResourceErrs
				results[workerOrgIdx].historyErrors += localHistoryErrs
				results[workerOrgIdx].statsErrors += localStatsErrs
				results[workerOrgIdx].resourceCount += int64(len(localResourceLats))
				results[workerOrgIdx].historyCount += int64(len(localHistoryLats))
				results[workerOrgIdx].statsCount += int64(len(localStatsLats))
				mu.Unlock()
			}()
		}
	}

	ready.Wait()
	start := time.Now()

	// Timeout guard against deadlocks.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("multi-tenant stress test timed out (possible deadlock)")
	}
	wallTime := time.Since(start)

	// Aggregate results across all orgs.
	var (
		allResourceLats []time.Duration
		allHistoryLats  []time.Duration
		allStatsLats    []time.Duration
		totalErrors     int64
		totalRequests   int64
	)

	for _, r := range results {
		allResourceLats = append(allResourceLats, r.resourceLats...)
		allHistoryLats = append(allHistoryLats, r.historyLats...)
		allStatsLats = append(allStatsLats, r.statsLats...)
		totalErrors += r.resourceErrors + r.historyErrors + r.statsErrors
		totalRequests += r.resourceCount + r.historyCount + r.statsCount
	}

	rps := float64(totalRequests) / wallTime.Seconds()
	t.Logf("multi-tenant stress: %d orgs, %d workers, %d requests in %v (%.1f rps)",
		numOrgs, totalWorkers, totalRequests, wallTime, rps)

	if len(allResourceLats) > 0 {
		t.Logf("  resources: %d requests, p50=%v p95=%v p99=%v",
			len(allResourceLats),
			percentile(allResourceLats, 0.50),
			percentile(allResourceLats, 0.95),
			percentile(allResourceLats, 0.99))
	}
	if len(allHistoryLats) > 0 {
		t.Logf("  metrics-history: %d requests, p50=%v p95=%v p99=%v",
			len(allHistoryLats),
			percentile(allHistoryLats, 0.50),
			percentile(allHistoryLats, 0.95),
			percentile(allHistoryLats, 0.99))
	}
	if len(allStatsLats) > 0 {
		t.Logf("  metrics-stats: %d requests, p50=%v p95=%v p99=%v",
			len(allStatsLats),
			percentile(allStatsLats, 0.50),
			percentile(allStatsLats, 0.95),
			percentile(allStatsLats, 0.99))
	}

	if totalErrors > 0 {
		t.Errorf("total %d error responses across all orgs and endpoints", totalErrors)
	}

	// p95 latency budgets under 20-org concurrent load.
	// These are generous (3s) to accommodate single-CPU CI runners.
	if len(allResourceLats) > 0 {
		p95 := percentile(allResourceLats, 0.95)
		if p95 > 3*time.Second {
			t.Errorf("resources p95=%v exceeds 3s budget under multi-tenant load", p95)
		}
	}
	if len(allHistoryLats) > 0 {
		p95 := percentile(allHistoryLats, 0.95)
		if p95 > 3*time.Second {
			t.Errorf("metrics-history p95=%v exceeds 3s budget under multi-tenant load", p95)
		}
	}
	if len(allStatsLats) > 0 {
		p95 := percentile(allStatsLats, 0.95)
		if p95 > 3*time.Second {
			t.Errorf("metrics-stats p95=%v exceeds 3s budget under multi-tenant load", p95)
		}
	}

	// Minimum throughput: with 60 goroutines over 2s, expect substantial request volume.
	// Set per-endpoint minimums to catch throughput collapse under contention.
	minResources := int64(200)
	minHistory := int64(200)
	minStats := int64(50)
	if int64(len(allResourceLats)) < minResources {
		t.Errorf("resources: %d requests below minimum %d", len(allResourceLats), minResources)
	}
	if int64(len(allHistoryLats)) < minHistory {
		t.Errorf("metrics-history: %d requests below minimum %d", len(allHistoryLats), minHistory)
	}
	if int64(len(allStatsLats)) < minStats {
		t.Errorf("metrics-stats: %d requests below minimum %d", len(allStatsLats), minStats)
	}
}

// TestMultiTenant_TenantIsolation verifies that metrics written to one org's
// store are not visible from another org's store, even under concurrent access.
func TestMultiTenant_TenantIsolation(t *testing.T) {
	suppressTestLogs(t)

	const numOrgs = 5

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init default persistence: %v", err)
	}

	type orgData struct {
		orgID   string
		store   *metrics.Store
		monitor *monitoring.Monitor
		vmID    string
	}

	orgList := make([]orgData, numOrgs)
	for i := 0; i < numOrgs; i++ {
		orgID := fmt.Sprintf("iso-org-%03d", i)
		if _, err := mtp.GetPersistence(orgID); err != nil {
			t.Fatalf("failed to init persistence for %s: %v", orgID, err)
		}

		store := newOrgMetricsStore(t, orgID)
		vmID := fmt.Sprintf("%s-vm-unique-%d", orgID, i)

		// Write a unique metric to each org's store.
		store.WriteBatchSync([]metrics.WriteMetric{{
			ResourceType: "vm",
			ResourceID:   vmID,
			MetricType:   "cpu",
			Value:        float64(i * 10),
			Timestamp:    time.Now().Add(-5 * time.Minute),
			Tier:         metrics.TierRaw,
		}})

		state := models.NewState()
		monitor := &monitoring.Monitor{}
		setTestUnexportedField(t, monitor, "state", state)
		setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
		setTestUnexportedField(t, monitor, "metricsStore", store)

		orgList[i] = orgData{orgID: orgID, store: store, monitor: monitor, vmID: vmID}
	}

	isoMonitorMap := make(map[string]*monitoring.Monitor, numOrgs)
	for _, org := range orgList {
		isoMonitorMap[org.orgID] = org.monitor
	}
	mtm := newPrePopulatedMTM(isoMonitorMap)

	defaultMonitor := &monitoring.Monitor{}
	setTestUnexportedField(t, defaultMonitor, "state", models.NewState())
	setTestUnexportedField(t, defaultMonitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	router := &Router{
		monitor:         defaultMonitor,
		mtMonitor:       mtm,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	// For each org, verify its unique VM is visible, and no other org's VMs are visible.
	var wg sync.WaitGroup
	errCh := make(chan string, numOrgs*numOrgs)

	for i, org := range orgList {
		wg.Add(1)
		go func(idx int, o orgData) {
			defer wg.Done()

			// Own data should be visible.
			ctx := context.WithValue(context.Background(), OrgIDContextKey, o.orgID)
			url := "/api/metrics-store/history?resourceType=vm&resourceId=" + o.vmID + "&metric=cpu&range=1h"
			req := httptest.NewRequest(http.MethodGet, url, nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- fmt.Sprintf("org %s own query failed: status %d", o.orgID, rec.Code)
				return
			}
			var resp metricsHistoryResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				errCh <- fmt.Sprintf("org %s own query unmarshal: %v", o.orgID, err)
				return
			}
			if len(resp.Points) == 0 {
				errCh <- fmt.Sprintf("org %s: own data not found (vmID=%s)", o.orgID, o.vmID)
				return
			}

			// Other orgs' data must NOT be visible. Require 200 with zero points
			// so handler errors don't silently mask isolation failures.
			for j, other := range orgList {
				if j == idx {
					continue
				}
				crossURL := "/api/metrics-store/history?resourceType=vm&resourceId=" + other.vmID + "&metric=cpu&range=1h"
				crossReq := httptest.NewRequest(http.MethodGet, crossURL, nil).WithContext(ctx)
				crossRec := httptest.NewRecorder()
				router.handleMetricsHistory(crossRec, crossReq)
				if crossRec.Code != http.StatusOK {
					errCh <- fmt.Sprintf("org %s cross-tenant query for %s returned status %d (expected 200 with empty points)",
						o.orgID, other.orgID, crossRec.Code)
					continue
				}
				var crossResp metricsHistoryResponse
				if err := json.Unmarshal(crossRec.Body.Bytes(), &crossResp); err != nil {
					errCh <- fmt.Sprintf("org %s cross-tenant query for %s unmarshal: %v", o.orgID, other.orgID, err)
					continue
				}
				if len(crossResp.Points) > 0 {
					errCh <- fmt.Sprintf("ISOLATION VIOLATION: org %s sees data from org %s (vmID=%s, %d points)",
						o.orgID, other.orgID, other.vmID, len(crossResp.Points))
				}
			}
		}(i, org)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("tenant isolation test timed out (possible deadlock)")
	}

	close(errCh)
	for msg := range errCh {
		t.Error(msg)
	}
}

// --- Multi-tenant stress test helpers ---

// buildMultiTenantState creates a State with numNodes nodes and 5 VMs per node,
// with resource IDs prefixed by orgID to make cross-org queries distinguishable.
func buildMultiTenantState(t *testing.T, orgID string, numNodes int) *models.State {
	t.Helper()
	state := models.NewState()
	instanceName := orgID + "-pve"

	nodes := make([]models.Node, numNodes)
	for n := 0; n < numNodes; n++ {
		nodes[n] = models.Node{
			ID:       fmt.Sprintf("%s:node%d", instanceName, n),
			Name:     fmt.Sprintf("%s-node-%d", orgID, n),
			Instance: instanceName,
			Status:   "online",
			CPU:      float64(n%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(n%60 + 20), Total: 64 << 30, Used: 32 << 30},
			Disk:     models.Disk{Usage: float64(n%40 + 30), Total: 500 << 30, Used: 250 << 30},
		}
	}
	state.UpdateNodesForInstance(instanceName, nodes)

	vms := make([]models.VM, numNodes*5)
	for v := range vms {
		nodeIdx := v / 5
		vms[v] = models.VM{
			ID:       fmt.Sprintf("%s:node%d:%d", instanceName, nodeIdx, 1000+v),
			VMID:     1000 + v,
			Name:     fmt.Sprintf("%s-vm-%d", orgID, v),
			Node:     fmt.Sprintf("node%d", nodeIdx),
			Instance: instanceName,
			Status:   "running",
			Type:     "qemu",
			CPU:      float64(v%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(v%60 + 20), Total: 4 << 30, Used: 2 << 30},
			Disk:     models.Disk{Usage: float64(v%40 + 30), Total: 50 << 30, Used: 25 << 30},
		}
	}
	state.UpdateVMsForInstance(instanceName, vms)

	return state
}

// newOrgMetricsStore creates an ephemeral per-org metrics store.
func newOrgMetricsStore(t *testing.T, orgID string) *metrics.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := metrics.DefaultConfig(dir)
	cfg.DBPath = fmt.Sprintf("%s/%s-metrics.db", dir, orgID)
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 10_000
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore for %s: %v", orgID, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// seedOrgMetrics writes test metrics for an org with org-prefixed resource IDs.
func seedOrgMetrics(t *testing.T, store *metrics.Store, orgID string, metricTypes []string, numResources, numPoints int) []string {
	t.Helper()
	base := time.Now().Add(-50 * time.Minute)
	ids := make([]string, numResources)

	batch := make([]metrics.WriteMetric, 0, numResources*numPoints*len(metricTypes))
	for r := 0; r < numResources; r++ {
		id := fmt.Sprintf("%s-vm-%d", orgID, r)
		ids[r] = id
		for _, mt := range metricTypes {
			for p := 0; p < numPoints; p++ {
				batch = append(batch, metrics.WriteMetric{
					ResourceType: "vm",
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

// multiTenantStateProvider implements TenantStateProvider for tests, returning
// per-org state snapshots from a pre-built map.
type multiTenantStateProvider struct {
	states map[string]*models.State
}

func (p *multiTenantStateProvider) UnifiedReadStateForTenant(orgID string) unifiedresources.ReadState {
	return SnapshotReadState(p.GetStateForTenant(orgID))
}

func (p *multiTenantStateProvider) UnifiedResourceSnapshotForTenant(orgID string) ([]unifiedresources.Resource, time.Time) {
	snapshot := p.GetStateForTenant(orgID)
	if snapshot.LastUpdate.IsZero() {
		return nil, time.Time{}
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	return registry.List(), snapshot.LastUpdate
}

func (p *multiTenantStateProvider) GetStateForTenant(orgID string) models.StateSnapshot {
	if s, ok := p.states[orgID]; ok {
		return s.GetSnapshot()
	}
	return models.StateSnapshot{}
}

// newPrePopulatedMTM creates a MultiTenantMonitor with pre-built monitors
// injected via reflection, bypassing the lazy-init config-loading path.
// This allows stress tests to use the real Router.getTenantMonitor path
// without requiring Proxmox connectivity.
func newPrePopulatedMTM(monitors map[string]*monitoring.Monitor) *monitoring.MultiTenantMonitor {
	baseCfg := &config.Config{}
	mtm := monitoring.NewMultiTenantMonitor(baseCfg, nil, nil)
	for orgID, monitor := range monitors {
		setMTMMonitor(mtm, orgID, monitor)
	}
	return mtm
}

// setMTMMonitor pre-populates a monitor in the MultiTenantMonitor's internal map
// so that GetMonitor returns it without attempting real config loading.
func setMTMMonitor(mtm *monitoring.MultiTenantMonitor, orgID string, monitor *monitoring.Monitor) {
	// Use the same reflection trick as setTestUnexportedField, but adapted for
	// map insertion. We access the "monitors" field and insert directly.
	// This avoids GetMonitor's lazy-init path which needs real persistence.
	mtmVal := reflect.ValueOf(mtm).Elem()
	monitorsField := mtmVal.FieldByName("monitors")
	if !monitorsField.IsValid() {
		panic("MultiTenantMonitor.monitors field not found")
	}
	ptr := unsafe.Pointer(monitorsField.UnsafeAddr())
	monitorsMap := (*map[string]*monitoring.Monitor)(ptr)
	(*monitorsMap)[orgID] = monitor
}
