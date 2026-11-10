package chartapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Match the chart SLO's 30 VMs, 20 system containers and 20 app containers:
// five metrics, 240 observations, capped at 120. No store is used here.
func chartBatchFixture(shared bool) []map[string][]monitoring.MetricPoint {
	batch := make([]map[string][]monitoring.MetricPoint, 70)
	for resource := range batch {
		metrics := timelineFixture(240, shared)
		delete(metrics, "memoryused")
		delete(metrics, "diskread")
		delete(metrics, "diskwrite")
		for _, points := range metrics {
			for i := range points {
				points[i].Value += float64(resource * 100)
			}
		}
		batch[resource] = metrics
	}
	return batch
}

func TestChartPointSelectionKeepsWorkloadValuesDetached(t *testing.T) {
	t.Parallel()
	batch := chartBatchFixture(true)
	var selection chartPointSelection
	var previous VMChartData
	var previousJSON []byte
	for resource, source := range batch {
		// Change only an interior timestamp on alternating resources. Equal
		// bounds/count and a different resource's values are not cache keys.
		if resource%2 != 0 {
			source["memory"][100].Timestamp = source["memory"][1].Timestamp
		}
		oldest, wantOldest := int64(math.MaxInt64), int64(math.MaxInt64)
		got := selection.convert(source, &oldest, 120)
		want := referenceConvertMetricsForChart(source, &wantOldest, 120)
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) || oldest != wantOldest || string(gotJSON) != string(wantJSON) {
			t.Fatalf("resource %d reused another timeline or values", resource)
		}
		if previous != nil {
			after, err := json.Marshal(previous)
			if err != nil || string(after) != string(previousJSON) {
				t.Fatal("later resource conversion changed earlier wire points")
			}
		}
		for name, points := range got {
			if len(points) == 0 {
				continue
			}
			first := source[name][0]
			original := points[0]
			points[0] = MetricPoint{Value: -999}
			if source[name][0] != first {
				t.Fatal("wire points alias source observations")
			}
			points[0] = original
			source[name][0].Value = -888
			if points[0] != original {
				t.Fatal("source mutation changed retained wire points")
			}
			source[name][0] = first
		}
		previous, previousJSON = got, gotJSON
	}
}

func convertChartBatch(batch []map[string][]monitoring.MetricPoint, reuse bool) {
	var selection chartPointSelection
	oldest := int64(math.MaxInt64)
	for _, source := range batch {
		if reuse {
			chartTimelineBenchmarkSink = selection.convert(source, &oldest, 120)
		} else {
			chartTimelineBenchmarkSink = convertMetricsForChart(source, &oldest, 120)
		}
	}
}

func TestChartPointSelectionBatchAllocations(t *testing.T) {
	batch := chartBatchFixture(true)
	// The standalone helper retains the immediate-parent's per-workload scope.
	isolated := testing.AllocsPerRun(10, func() { convertChartBatch(batch, false) })
	response := testing.AllocsPerRun(10, func() { convertChartBatch(batch, true) })
	t.Logf("70-workload conversion allocations: isolated=%.0f response=%.0f", isolated, response)
	if response >= isolated {
		t.Fatalf("response-scoped selection must avoid repeated sampling scratch: isolated=%.0f response=%.0f", isolated, response)
	}
}

func BenchmarkConvertMetricsForChartBatch(b *testing.B) {
	for _, shared := range []bool{true, false} {
		batch := chartBatchFixture(shared)
		for _, reuse := range []bool{false, true} {
			b.Run(fmt.Sprintf("shared=%t/reuse=%t", shared, reuse), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					convertChartBatch(batch, reuse)
				}
			})
		}
	}
}

func TestHandleWorkloadChartsBatchSelectionPreservesEveryFamily(t *testing.T) {
	setMockModeForTest(t, false)
	monitor, state, _ := newTestMonitor(t)
	history := monitoring.NewMetricsHistory(300, 5*time.Hour)
	setUnexportedField(t, monitor, "metricsHistory", history)
	state.VMs = []models.VM{{ID: "vm-batch", Name: "same-name", Status: "running"}}
	state.Containers = []models.Container{{ID: "ct-batch", Name: "same-name", Status: "running"}}
	state.DockerHosts = []models.DockerHost{{ID: "host-batch", Status: "online", Containers: []models.DockerContainer{
		{ID: "app-batch", Name: "same-name", State: "running"},
	}}}
	state.KubernetesClusters = []models.KubernetesCluster{{ID: "cluster-batch", Status: "online", Pods: []models.KubernetesPod{
		{UID: "pod-batch", Name: "same-name", Namespace: "default", Phase: "Running"},
	}}}
	syncTestResourceStore(t, monitor, state)
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	type workload struct {
		kind, family, key string
		request           monitoring.GuestChartRequest
	}
	var workloads []workload
	for _, vm := range readState.VMs() {
		key, request, ok := vmChartRequest(vm)
		if !ok {
			t.Fatal("VM fixture has no chart identity")
		}
		workloads = append(workloads, workload{"vm", "vm", key, request})
	}
	for _, ct := range readState.Containers() {
		key, request, ok := systemContainerChartRequest(ct)
		if !ok {
			t.Fatal("system container fixture has no chart identity")
		}
		workloads = append(workloads, workload{"system-container", "container", key, request})
	}
	for _, pod := range readState.Pods() {
		key := kubernetesPodMetricIDFromView(pod)
		workloads = append(workloads, workload{"k8s", "k8s", key, monitoring.GuestChartRequest{InMemoryKey: key, SQLResourceID: key}})
	}
	for _, container := range readState.DockerContainers() {
		key, request, ok := appContainerChartRequest(container)
		if !ok {
			t.Fatal("app container fixture has no chart identity")
		}
		workloads = append(workloads, workload{"app-container", "dockerContainer", key, request})
	}
	if len(workloads) != 4 {
		t.Fatalf("expected all four workload families, got %d", len(workloads))
	}
	base := time.Now().UTC().Truncate(time.Minute).Add(-239 * time.Minute)
	for resource, workload := range workloads {
		for _, name := range workloadSummaryMetricOrder {
			for point := 0; point < 240; point++ {
				history.AddGuestMetric(workload.request.InMemoryKey, name, float64(1000*resource+point), base.Add(time.Duration(point)*time.Minute))
			}
		}
	}
	want := make(map[string]VMChartData)
	wantOldest := time.Now().UnixMilli()
	for _, workload := range workloads {
		metricNames := guestSparklineMetricOrder
		if workload.kind == "k8s" {
			metricNames = workloadSummaryMetricOrder
		} else if workload.kind == "app-container" {
			metricNames = infrastructureSummaryMetricOrder
		}
		raw := monitor.GetGuestMetricsForChartBatch(workload.family, []monitoring.GuestChartRequest{workload.request}, 4*time.Hour, metricNames...)
		want[workload.key] = referenceConvertMetricsForChart(raw[workload.request.SQLResourceID], &wantOldest, 120)
		for _, name := range workloadSummaryMetricOrder {
			if len(want[workload.key][name]) != 120 {
				t.Fatalf("%s/%s fixture must exercise capped history", workload.kind, name)
			}
		}
	}
	service := NewService(fixedMonitorResolver{monitor: monitor})
	request := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=4h&maxPoints=120", nil)
	recorder := httptest.NewRecorder()
	service.HandleWorkloadCharts(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	var got WorkloadChartsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ChartData) != 3 || len(got.DockerData) != 1 || len(got.GuestTypes) != 4 || got.Stats.OldestDataTimestamp != wantOldest {
		t.Fatal("response-scoped selection changed family ownership or oldest metadata")
	}
	for _, workload := range workloads {
		series := got.ChartData[workload.key]
		if workload.kind == "app-container" {
			series = got.DockerData[workload.key]
		}
		if !reflect.DeepEqual(series, want[workload.key]) || got.GuestTypes[workload.key] != workload.kind {
			t.Fatalf("%s history changed observations, values or ownership: metric keys got=%v want=%v, type=%q", workload.kind, mapKeys(series), mapKeys(want[workload.key]), got.GuestTypes[workload.key])
		}
	}
	if got.Stats.PointCounts.Guests != 1800 || got.Stats.PointCounts.DockerContainers != 600 || got.Stats.PointCounts.Total != 2400 {
		t.Fatalf("selected point counts changed: %+v", got.Stats.PointCounts)
	}
	cached := httptest.NewRecorder()
	service.HandleWorkloadCharts(cached, request)
	if cached.Code != http.StatusOK || !bytes.Equal(cached.Body.Bytes(), recorder.Body.Bytes()) {
		t.Fatal("cached response changed the computed chart bytes")
	}
}
