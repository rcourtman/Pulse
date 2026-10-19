package chartapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestHandleWorkloadCharts_RetainedBatchTierChanges(t *testing.T) {
	testutil.SetMockMode(t, false)
	monitor, state, _ := newTestMonitor(t)
	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw, cfg.RetentionMinute = 90*24*time.Hour, 90*24*time.Hour
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.WaitForMaintenance(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	setUnexportedField(t, monitor, "metricsStore", store)
	vms := []models.VM{
		{ID: "tier-vm-a", VMID: 101, Name: "retained-a", Node: "pve1", Instance: "lab", Type: "qemu", Status: "running", CPU: .25},
		{ID: "tier-vm-b", VMID: 102, Name: "retained-b", Node: "pve1", Instance: "lab", Type: "qemu", Status: "running", CPU: .40},
	}
	state.UpdateNodesForInstance("lab", []models.Node{{ID: "lab-pve1", Name: "pve1", Instance: "lab", Status: "online"}})
	state.UpdateVMsForInstance("lab", vms)
	syncTestResourceStore(t, monitor, state)
	base := time.Now().UTC().Truncate(time.Minute).Add(-3 * time.Hour)
	var batch []metrics.WriteMetric
	for r, vm := range vms {
		for _, metric := range []string{"cpu", "memory", "disk", "netin", "netout"} {
			for i := 0; i < 20; i++ {
				batch = append(batch, metrics.WriteMetric{ResourceType: "vm", ResourceID: vm.ID, MetricType: metric,
					Timestamp: base.Add(time.Duration(i) * time.Minute), Tier: metrics.TierMinute, Value: float64(10 + r*20 + i)})
			}
		}
	}
	store.WriteBatchSync(batch)
	payloads := make(map[string]WorkloadChartsResponse)
	read := func(name string, wantPoints int, wantTail float64) {
		t.Helper()
		service := NewService(fixedMonitorResolver{monitor: monitor})
		rec := httptest.NewRecorder()
		service.HandleWorkloadCharts(rec, httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=4h&maxPoints=120", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("chart status %d: %s", rec.Code, rec.Body.String())
		}
		var response WorkloadChartsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		a, b := response.ChartData["lab:pve1:101"]["cpu"], response.ChartData["lab:pve1:102"]["cpu"]
		if len(a) != wantPoints || a[0].Value != 10 || a[len(a)-1].Value != wantTail || len(b) != 20 || b[0].Value != 30 || b[len(b)-1].Value != 49 {
			t.Fatalf("lost history, tail or attribution: a=%+v b=%+v", a, b)
		}
		if response.Stats.PrimarySourceHint != "store_or_memory_fallback" {
			t.Fatalf("wrong source hint: %q", response.Stats.PrimarySourceHint)
		}
		payloads[name] = response
	}
	read("single-tier", 20, 29)
	store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "vm", ResourceID: vms[0].ID, MetricType: "cpu", Timestamp: base.Add(2*time.Hour + 58*time.Minute), Tier: metrics.TierRaw, Value: 71}})
	read("new-tail", 21, 71)
	// Opt-in export contains only these synthetic fixtures. The browser proof
	// consumes the actual endpoint bytes, not a hand-authored chart response.
	if os.Getenv("PULSE_CHART_FIXTURE_EXPORT") == "1" {
		body, err := json.Marshal(map[string]any{"vms": vms, "payloads": payloads})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("CHART_FIXTURE %s", body)
	}
}
