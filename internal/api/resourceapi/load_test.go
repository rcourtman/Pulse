package resourceapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestLoad_500Node_ConcurrentResources validates the resource query service
// with 500 nodes and 2,500 VMs under 50-way concurrent request load.
func TestLoad_500Node_ConcurrentResources(t *testing.T) {
	if raceEnabled {
		t.Skip("skipping latency test under -race")
	}

	state := buildResourceLoadState(t, 500)
	handlers := NewQueryService(&config.Config{DataPath: t.TempDir()})
	handlers.SetStateProvider(&resourceLoadStateProvider{state: state})

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
	if int(total) != 3000 {
		t.Fatalf("warmup: expected 3000 total resources, got %v", total)
	}

	const concurrency = 50
	const duration = 2 * time.Second
	latencies := make([][]time.Duration, concurrency)
	var errors, totalCount int64
	var ready, workers sync.WaitGroup
	ready.Add(concurrency)

	for index := 0; index < concurrency; index++ {
		workers.Add(1)
		index := index
		latencies[index] = make([]time.Duration, 0, 200)
		go func() {
			defer workers.Done()
			ready.Done()
			ready.Wait()
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				started := time.Now()
				req := httptest.NewRequest(http.MethodGet, "/api/resources?limit=50", nil)
				rec := httptest.NewRecorder()
				handlers.HandleListResources(rec, req)
				if rec.Code != http.StatusOK {
					atomic.AddInt64(&errors, 1)
					continue
				}
				latencies[index] = append(latencies[index], time.Since(started))
				atomic.AddInt64(&totalCount, 1)
			}
		}()
	}
	ready.Wait()
	started := time.Now()
	workers.Wait()
	wallTime := time.Since(started)
	if errors > 0 {
		t.Errorf("got %d error responses", errors)
	}

	all := mergeResourceLoadLatencies(latencies)
	if len(all) == 0 {
		t.Fatal("no successful requests recorded")
	}
	p50 := resourceLoadPercentile(all, 0.50)
	p95 := resourceLoadPercentile(all, 0.95)
	p99 := resourceLoadPercentile(all, 0.99)
	t.Logf("resources 500-node load: %d requests in %v (%.1f rps)", totalCount, wallTime, float64(totalCount)/wallTime.Seconds())
	t.Logf("p50=%v p95=%v p99=%v", p50, p95, p99)

	target := 3 * time.Second
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		target = 4 * time.Second
	}
	if p95 > target {
		resourceLoadOverrun(t, "p95 latency %v exceeds %v budget for 500-node concurrent resources load", p95, target)
	}
	minimum := int64(100)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		minimum = 40
	}
	if totalCount < minimum {
		resourceLoadOverrun(t, "completed only %d requests, expected at least %d for 500-node concurrent resources load", totalCount, minimum)
	}
}

func buildResourceLoadState(t *testing.T, numNodes int) *models.State {
	t.Helper()
	state := models.NewState()
	const numInstances = 10
	basePerInstance, remainder := numNodes/numInstances, numNodes%numInstances
	nodesSoFar := 0
	for instance := 0; instance < numInstances; instance++ {
		instanceName := fmt.Sprintf("pve%d", instance)
		count := basePerInstance
		if instance < remainder {
			count++
		}
		if count == 0 {
			continue
		}
		nodes := make([]models.Node, count)
		for node := range nodes {
			globalIndex := nodesSoFar + node
			nodes[node] = models.Node{
				ID: fmt.Sprintf("%s:node%d", instanceName, node), Name: fmt.Sprintf("node-%d", globalIndex), Instance: instanceName,
				Status: "online", CPU: float64(globalIndex%80+10) / 100,
				Memory: models.Memory{Usage: float64(globalIndex%60 + 20), Total: 64 << 30, Used: 32 << 30},
				Disk:   models.Disk{Usage: float64(globalIndex%40 + 30), Total: 500 << 30, Used: 250 << 30},
			}
		}
		state.UpdateNodesForInstance(instanceName, nodes)
		vms := make([]models.VM, count*5)
		for vm := range vms {
			nodeIndex := vm / 5
			globalIndex := (nodesSoFar+nodeIndex)*5 + vm%5
			vms[vm] = models.VM{
				ID: fmt.Sprintf("%s:node%d:%d", instanceName, nodeIndex, 1000+globalIndex), VMID: 1000 + globalIndex,
				Name: fmt.Sprintf("vm-%d", globalIndex), Node: fmt.Sprintf("node%d", nodeIndex), Instance: instanceName,
				Status: "running", Type: "qemu", CPU: float64(globalIndex%80+10) / 100,
				Memory: models.Memory{Usage: float64(globalIndex%60 + 20), Total: 4 << 30, Used: 2 << 30},
				Disk:   models.Disk{Usage: float64(globalIndex%40 + 30), Total: 50 << 30, Used: 25 << 30},
			}
		}
		state.UpdateVMsForInstance(instanceName, vms)
		nodesSoFar += count
	}
	return state
}

func resourceLoadOverrun(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Errorf(format, args...)
		return
	}
	t.Skipf("%s (host CPU contention can cause this locally; CI enforces the budget)", fmt.Sprintf(format, args...))
}

func mergeResourceLoadLatencies(groups [][]time.Duration) []time.Duration {
	var merged []time.Duration
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return merged
}

func resourceLoadPercentile(durations []time.Duration, percentile float64) time.Duration {
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted)-1)*percentile)]
}

type resourceLoadStateProvider struct{ state *models.State }

func (p *resourceLoadStateProvider) ReadSnapshot() models.StateSnapshot { return p.state.GetSnapshot() }
