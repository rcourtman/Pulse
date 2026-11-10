package chartapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func chartNodeScopeNodes(enriched bool) []models.Node {
	var nodes []models.Node
	for i := 0; i < 32; i++ {
		node := models.Node{ID: fmt.Sprintf("node-%02d", i), Name: fmt.Sprintf("native-%02d", i), DisplayName: "Different display name", Instance: "site", Status: "online"}
		if enriched {
			node.LoadAverage = []float64{1, 2, 3}
			node.Temperature = &models.Temperature{Available: true, HasCPU: true, Cores: make([]models.CoreTemp, 64), GPU: make([]models.GPUTemp, 2), NVMe: make([]models.NVMeTemp, 8), SMART: make([]models.DiskTemp, 12)}
			for nic := 0; nic < 8; nic++ {
				node.NetworkInterfaces = append(node.NetworkInterfaces, models.HostNetworkInterface{Name: fmt.Sprintf("eth%d", nic), Addresses: []string{"192.0.2.1/24", "2001:db8::1/64"}})
			}
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func chartNodeScopeFixture(tb testing.TB, enriched bool) *monitoring.Monitor {
	tb.Helper()
	state := models.NewState()
	state.Nodes = chartNodeScopeNodes(enriched)
	for i := 0; i < 25; i++ {
		state.VMs = append(state.VMs, models.VM{ID: fmt.Sprintf("vm-%02d", i), VMID: 100 + i, Name: "same-name", Node: fmt.Sprintf("native-%02d", i%2), Instance: "site", Status: "running"})
	}
	for i := 0; i < 20; i++ {
		state.Containers = append(state.Containers, models.Container{ID: fmt.Sprintf("ct-%02d", i), VMID: 200 + i, Name: "same-name", Node: fmt.Sprintf("native-%02d", i%2), Instance: "site", Status: "running"})
	}
	for i := 0; i < 2; i++ {
		host := models.DockerHost{ID: fmt.Sprintf("host-%02d", i), Hostname: fmt.Sprintf("native-%02d", i), Status: "online"}
		for app := 0; app < 10; app++ {
			host.Containers = append(host.Containers, models.DockerContainer{ID: fmt.Sprintf("app-%02d-%02d", i, app), Name: "same-name", State: "running"})
		}
		state.DockerHosts = append(state.DockerHosts, host)
	}
	cluster := models.KubernetesCluster{ID: "cluster", Status: "online"}
	for i := 0; i < 5; i++ {
		cluster.Pods = append(cluster.Pods, models.KubernetesPod{UID: fmt.Sprintf("pod-%02d", i), Name: "same-name", Namespace: "default", NodeName: fmt.Sprintf("native-%02d", i%2), Phase: "Running"})
	}
	state.KubernetesClusters = []models.KubernetesCluster{cluster}
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	history := monitoring.NewMetricsHistory(300, 48*time.Hour)
	read := adapter
	var requests []monitoring.GuestChartRequest
	for _, vm := range read.VMs() {
		_, request, _ := vmChartRequest(vm)
		requests = append(requests, request)
	}
	for _, ct := range read.Containers() {
		_, request, _ := systemContainerChartRequest(ct)
		requests = append(requests, request)
	}
	for _, pod := range read.Pods() {
		requests = append(requests, monitoring.GuestChartRequest{InMemoryKey: kubernetesPodMetricIDFromView(pod)})
	}
	for _, app := range read.DockerContainers() {
		_, request, _ := appContainerChartRequest(app)
		requests = append(requests, request)
	}
	if len(requests) != 70 {
		tb.Fatalf("expected 70 workloads, got %d", len(requests))
	}
	base := time.Now().UTC().Truncate(time.Minute).Add(-240 * time.Minute)
	for resource, request := range requests {
		for _, metric := range workloadSummaryMetricOrder {
			for point := 0; point < 240; point++ {
				history.AddGuestMetric(request.InMemoryKey, metric, float64(1000*resource+point), base.Add(time.Duration(point)*time.Minute))
			}
		}
	}
	monitor := &monitoring.Monitor{}
	for name, value := range map[string]any{"state": state, "resourceStore": monitoring.ResourceStoreInterface(adapter), "metricsHistory": history} {
		field := reflect.ValueOf(monitor).Elem().FieldByName(name)
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
	}
	return monitor
}

var chartNodeScopeBody []byte

func chartNodeScopeBuild(tb testing.TB, monitor *monitoring.Monitor, selected string) {
	tb.Helper()
	service := &Service{}
	var err error
	chartNodeScopeBody, err = service.buildWorkloadChartsResponse(context.Background(), monitor, "24h", selected, 120, 24*time.Hour, 2*time.Hour)
	if err != nil {
		tb.Fatal(err)
	}
}

func BenchmarkWorkloadChartsNodeInventory(b *testing.B) {
	for _, enriched := range []bool{false, true} {
		monitor := chartNodeScopeFixture(b, enriched)
		for _, selected := range []string{"", "node-00", "missing"} {
			b.Run(fmt.Sprintf("enriched=%t/selected=%s", enriched, selected), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					chartNodeScopeBuild(b, monitor, selected)
				}
			})
		}
	}
}

type chartNodeReadState struct {
	unifiedresources.ReadState
	nodes []*unifiedresources.NodeView
	calls int
}

func (r *chartNodeReadState) Nodes() []*unifiedresources.NodeView {
	r.calls++
	return r.nodes
}

func TestWorkloadChartsSelectedNodeKeepsSnapshotIdentity(t *testing.T) {
	view := func(id, source, name, instance string) *unifiedresources.NodeView {
		v := unifiedresources.NewNodeView(&unifiedresources.Resource{
			ID: id, Name: "Different display name",
			Proxmox: &unifiedresources.ProxmoxData{SourceID: source, NodeName: name, Instance: instance},
		})
		return &v
	}
	first := view("canonical", " source ", " native ", " site ")
	duplicate := view("another-canonical", "source", "other-native", "other-site")
	fallback := view(" fallback ", " \t", "fallback-native", "fallback-site")
	empty := view("", "", "", "")
	read := &chartNodeReadState{nodes: []*unifiedresources.NodeView{nil, first, duplicate, fallback, empty}}
	for _, tc := range []struct {
		id   string
		want *unifiedresources.NodeView
	}{
		{"source", first}, {"canonical", nil}, {"another-canonical", nil},
		{"fallback", fallback}, {"missing", nil}, {"Source", nil}, {"native", nil},
	} {
		if got := workloadChartsSelectedNode(read, tc.id); got != tc.want {
			t.Fatalf("ID %q: got %v, want %v", tc.id, got, tc.want)
		}
	}
	if first.NodeName() != "native" || first.Instance() != "site" {
		t.Fatal("fixture must distinguish native name/instance from display metadata")
	}
	read.calls = 0
	if got := workloadChartsSelectedNode(read, ""); got != nil || read.calls != 0 {
		t.Fatal("global requests must not enumerate nodes or match an empty identity")
	}
	if got := workloadChartsSelectedNode(nil, "source"); got != nil {
		t.Fatal("absent read state invented a selected node")
	}
}

// Use the public snapshot conversion as the independent, previous-behavior
// oracle. Selection must preserve its source-ID precedence and native name.
func snapshotChartScopeKeys(monitor *monitoring.Monitor, selected string) map[string]bool {
	var node *models.Node
	for _, candidate := range monitor.NodesSnapshot() {
		if selected != "" && candidate.ID == selected {
			node = &candidate
			break
		}
	}
	match := func(instance, name string) bool {
		return node == nil || (strings.EqualFold(strings.TrimSpace(instance), strings.TrimSpace(node.Instance)) &&
			strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(node.Name)))
	}
	read := monitor.GetUnifiedReadStateOrSnapshot()
	keys := make(map[string]bool)
	for _, vm := range read.VMs() {
		if match(vm.Instance(), vm.Node()) {
			key, _, _ := vmChartRequest(vm)
			keys[key] = true
		}
	}
	for _, ct := range read.Containers() {
		if match(ct.Instance(), ct.Node()) {
			key, _, _ := systemContainerChartRequest(ct)
			keys[key] = true
		}
	}
	for _, pod := range read.Pods() {
		if node == nil || strings.EqualFold(strings.TrimSpace(pod.NodeName()), strings.TrimSpace(node.Name)) {
			keys[kubernetesPodMetricIDFromView(pod)] = true
		}
	}
	hosts := make(map[string]bool)
	for _, host := range read.DockerHosts() {
		hosts[host.ID()] = node == nil || strings.EqualFold(strings.TrimSpace(host.Hostname()), strings.TrimSpace(node.Name)) ||
			strings.EqualFold(strings.TrimSpace(host.Name()), strings.TrimSpace(node.Name))
	}
	for _, app := range read.DockerContainers() {
		if node == nil || hosts[app.ParentID()] {
			key, _, _ := appContainerChartRequest(app)
			keys[key] = true
		}
	}
	return keys
}

func TestHandleWorkloadChartsNodeScopeMatchesSnapshot(t *testing.T) {
	setMockModeForTest(t, false)
	monitor := chartNodeScopeFixture(t, true)
	before := monitor.NodesSnapshot()
	service := NewService(fixedMonitorResolver{monitor: monitor})
	read := func(selected string) (WorkloadChartsResponse, []byte) {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=24h&maxPoints=120&node="+selected, nil)
		recorder := httptest.NewRecorder()
		service.HandleWorkloadCharts(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected chart result: %d %s", recorder.Code, recorder.Body.String())
		}
		var response WorkloadChartsResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response, recorder.Body.Bytes()
	}
	global, _ := read("")
	if len(global.GuestTypes) != 70 || global.Stats.PointCounts.Total != 70*5*120 {
		t.Fatalf("fixture lost workloads or observations: %d workloads, %d points", len(global.GuestTypes), global.Stats.PointCounts.Total)
	}
	// A unified ID is not a second alias when the node has a source ID.
	canonicalID := monitor.GetUnifiedReadStateOrSnapshot().Nodes()[0].ID()
	for _, selected := range []string{"", "node-00", "node-01", "missing", canonicalID} {
		t.Run(selected, func(t *testing.T) {
			wantKeys := snapshotChartScopeKeys(monitor, selected)
			got, payload := read(selected)
			if len(got.GuestTypes) != len(wantKeys) || len(got.ChartData)+len(got.DockerData) != len(wantKeys) {
				t.Fatalf("scope %q: wrong workload membership: got %d, want %d", selected, len(got.GuestTypes), len(wantKeys))
			}
			if selected == "node-00" && len(wantKeys) != 36 {
				t.Fatalf("selected fixture must cover every family, got %d workloads", len(wantKeys))
			}
			var guests, apps int
			for key := range wantKeys {
				if got.GuestTypes[key] != global.GuestTypes[key] {
					t.Fatalf("scope changed family for %q", key)
				}
				series, want := got.ChartData[key], global.ChartData[key]
				if got.GuestTypes[key] == "app-container" {
					series, want = got.DockerData[key], global.DockerData[key]
				}
				if !reflect.DeepEqual(series, want) {
					t.Fatalf("scope changed timestamp, value or empty metric series for %q", key)
				}
				for _, points := range series {
					if got.GuestTypes[key] == "app-container" {
						apps += len(points)
					} else {
						guests += len(points)
					}
				}
			}
			if got.Stats.PointCounts.Guests != guests || got.Stats.PointCounts.DockerContainers != apps ||
				got.Stats.PointCounts.Total != guests+apps || got.Stats.OldestDataTimestamp != global.Stats.OldestDataTimestamp {
				t.Fatal("scope changed point counts or oldest observation")
			}
			_, cached := read(selected)
			if !bytes.Equal(payload, cached) {
				t.Fatal("cache hit changed chart bytes")
			}
		})
	}
	if !reflect.DeepEqual(before, monitor.NodesSnapshot()) {
		t.Fatal("chart selection mutated retained node inventories")
	}
}

func TestWorkloadChartsAllocationsDoNotScaleWithNodeSensors(t *testing.T) {
	// AllocsPerRun counts process-wide allocations, including other tests'
	// asynchronous work. Use this same binary (and its race instrumentation)
	// with only this guard selected, and verify that the child ran it.
	const childEnv = "PULSE_CHART_NODE_ALLOCATION_CHILD"
	if os.Getenv(childEnv) != t.Name() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0],
			"-test.run=^"+regexp.QuoteMeta(t.Name())+"$",
			"-test.count=1", "-test.v", "-test.timeout=20s",
			"-test.cpu="+strconv.Itoa(runtime.GOMAXPROCS(0)))
		cmd.Env = append(os.Environ(), childEnv+"="+t.Name())
		output, err := cmd.CombinedOutput()
		t.Logf("isolated chart node allocation check:\n%s", output)
		if err != nil {
			t.Fatalf("isolated chart node allocation check: %v", err)
		}
		if !strings.Contains(string(output), "--- PASS: "+t.Name()+" ") ||
			!strings.Contains(string(output), "snapshot copy positive control:") {
			t.Fatal("isolated chart node allocation check did not complete the guard and positive control")
		}
		return
	}

	setMockModeForTest(t, false)
	// Keep the same 32-node sensor/network inventories, but no workload
	// series: unrelated map growth and JSON pools in a 42,000-point response
	// cannot establish whether the builder copied node inventories. The full
	// populated handler/benchmark fixtures above still exercise that payload.
	fixture := func(enriched bool) *monitoring.Monitor {
		monitor, state, _ := newTestMonitor(t)
		state.Nodes = chartNodeScopeNodes(enriched)
		syncTestResourceStore(t, monitor, state)
		return monitor
	}
	lean, enriched := fixture(false), fixture(true)
	for _, selected := range []string{"", "node-00", "missing"} {
		plain := testing.AllocsPerRun(5, func() { chartNodeScopeBuild(t, lean, selected) })
		sensors := testing.AllocsPerRun(5, func() { chartNodeScopeBuild(t, enriched, selected) })
		t.Logf("selected=%q: lean=%.0f enriched=%.0f allocations", selected, plain, sensors)
		// Keep the original sample count and +10 budget. The real builder is
		// measured, so reverting it to NodesSnapshot cannot bypass the guard.
		if sensors > plain+10 {
			t.Fatalf("irrelevant node sensor inventory increased chart allocations: lean=%.0f enriched=%.0f", plain, sensors)
		}
	}

	// The original snapshot conversion is a positive control: this fixture
	// must expose sensor copying, rather than pass with empty node views.
	var snapshot []models.Node
	plain := testing.AllocsPerRun(5, func() { snapshot = lean.NodesSnapshot() })
	sensors := testing.AllocsPerRun(5, func() { snapshot = enriched.NodesSnapshot() })
	t.Logf("snapshot copy positive control: lean=%.0f enriched=%.0f allocations", plain, sensors)
	if len(snapshot) != 32 || sensors <= plain+10 {
		t.Fatal("fixture did not expose copying the retained node inventories")
	}
}
