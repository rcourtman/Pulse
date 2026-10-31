package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// These tests record history through the production writers and registry,
// then read the rolling alert window with the identity each alert evaluator
// sends. None of them writes a key the provider maps to, so they tie the
// provider to its writers rather than to its own lookup table.

const metricWindowContainerName = "app"

func useRealMetricWindowMode(t *testing.T) {
	t.Helper()
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
}

// newMetricWindowRegistryMonitor returns a report-capable monitor. With
// persistent set, writers also commit to SQLite so a test can lose the
// in-memory tail the way a restart does.
func newMetricWindowRegistryMonitor(t *testing.T, persistent bool) *Monitor {
	t.Helper()
	m := newTestMonitor(t)
	if persistent {
		m.metricsStore = newChartFallbackTestMonitor(t).metricsStore
	}
	return m
}

func newMetricWindowAdapter() *unifiedresources.MonitorAdapter {
	return unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
}

func dropInMemoryTail(m *Monitor) {
	m.metricsStore.Flush()
	m.metricsHistory.Reset()
}

func readCPUWindow(t *testing.T, m *Monitor, resourceID, resourceType string) ([]alerts.MetricWindowPoint, time.Time) {
	t.Helper()
	end := time.Now()
	points, err := m.metricWindowPoints(alerts.MetricWindowRequest{
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Metric:       "cpu",
		Start:        end.Add(-5 * time.Minute),
		End:          end,
	})
	if err != nil {
		t.Fatalf("metricWindowPoints(%q, %q): %v", resourceID, resourceType, err)
	}
	return points, end
}

// assertOneCPUSample requires exactly the writer's observation. SQLite keeps
// whole seconds, so the lower bound is the second the sample was taken in.
func assertOneCPUSample(t *testing.T, label string, points []alerts.MetricWindowPoint, want float64, takenAfter, readAt time.Time) {
	t.Helper()
	if len(points) != 1 || points[0].Value != want ||
		points[0].Timestamp.Before(takenAfter.Truncate(time.Second)) || points[0].Timestamp.After(readAt) {
		t.Fatalf("%s window = %+v, want one %.2f sample taken after %s", label, points, want, takenAfter.Format(time.RFC3339Nano))
	}
}

func persistenceName(persistent bool) string {
	if persistent {
		return "persistent history after the in-memory tail is lost"
	}
	return "live in-memory tail"
}

func applyMetricWindowDockerReport(t *testing.T, m *Monitor, containerID string, containerCPU, hostCPU float64) models.DockerHost {
	t.Helper()
	host, err := m.ApplyDockerReport(agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "docker-agent", Version: "test", IntervalSeconds: 30},
		Host:       agentsdocker.HostInfo{MachineID: "machine", Hostname: "worker", TotalCPU: 4, CPUUsagePercent: hostCPU},
		Containers: []agentsdocker.Container{{ID: containerID, Name: metricWindowContainerName, State: "running", CPUPercent: containerCPU}},
		Timestamp:  time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if len(host.Containers) != 1 || host.Containers[0].ID != containerID {
		t.Fatalf("report did not record the container: %+v", host.Containers)
	}
	return host
}

// Docker container CPU alerts name their subject docker:<host>/<container>
// with type app-container (alerts/docker.go) and evaluate the factory
// five-minute CPU window. Docker reports full 64-character container IDs;
// the registry binds a container reference only by that durable ID.
func TestMetricWindowPointsReadsDockerContainerHistoryFromAlertReference(t *testing.T) {
	useRealMetricWindowMode(t)
	containerID := strings.Repeat("ab", 32)

	for _, persistent := range []bool{false, true} {
		t.Run(persistenceName(persistent), func(t *testing.T) {
			m := newMetricWindowRegistryMonitor(t, persistent)
			m.SetResourceStore(newMetricWindowAdapter())
			reportedAt := time.Now()
			host := applyMetricWindowDockerReport(t, m, containerID, 380, 0)
			want := models.DockerContainerCPUCapacityPercent(host.Containers[0], host.CPUs)
			if persistent {
				dropInMemoryTail(m)
			}

			reference := alerts.DockerContainerResourceID(host.ID, containerID, metricWindowContainerName)
			points, readAt := readCPUWindow(t, m, reference, "app-container")
			assertOneCPUSample(t, "container CPU", points, want, reportedAt, readAt)
		})
	}

	t.Run("empty history stays unknown", func(t *testing.T) {
		m := newMetricWindowRegistryMonitor(t, false)
		m.SetResourceStore(newMetricWindowAdapter())
		host := applyMetricWindowDockerReport(t, m, containerID, 380, 0)
		m.metricsHistory.Reset()

		reference := alerts.DockerContainerResourceID(host.ID, containerID, metricWindowContainerName)
		if points, _ := readCPUWindow(t, m, reference, "app-container"); len(points) != 0 {
			t.Fatalf("window without history = %+v, want no observations", points)
		}
	})

	t.Run("a shortened container ID does not bind", func(t *testing.T) {
		m := newMetricWindowRegistryMonitor(t, false)
		m.SetResourceStore(newMetricWindowAdapter())
		host := applyMetricWindowDockerReport(t, m, containerID, 380, 0)

		reference := alerts.DockerContainerResourceID(host.ID, containerID[:12], metricWindowContainerName)
		if points, _ := readCPUWindow(t, m, reference, "app-container"); len(points) != 0 {
			t.Fatalf("short container reference read history: %+v", points)
		}
	})

	t.Run("a replaced resource store is resolved afresh", func(t *testing.T) {
		m := newMetricWindowRegistryMonitor(t, false)
		m.SetResourceStore(newMetricWindowAdapter())
		reportedAt := time.Now()
		host := applyMetricWindowDockerReport(t, m, containerID, 380, 0)
		want := models.DockerContainerCPUCapacityPercent(host.Containers[0], host.CPUs)
		reference := alerts.DockerContainerResourceID(host.ID, containerID, metricWindowContainerName)

		// SetResourceStore rebuilds the new adapter from current state.
		m.SetResourceStore(newMetricWindowAdapter())
		points, readAt := readCPUWindow(t, m, reference, "app-container")
		assertOneCPUSample(t, "container CPU after store replacement", points, want, reportedAt, readAt)

		// A store that cannot bind the reference leaves no earlier binding behind.
		m.mu.Lock()
		m.resourceStore = &metricWindowReferenceStore{}
		m.mu.Unlock()
		if points, _ := readCPUWindow(t, m, reference, "app-container"); len(points) != 0 {
			t.Fatalf("window kept a binding from the replaced store: %+v", points)
		}
	})
}

// Host-agent CPU alerts name their subject agent:<host ID> with type agent
// (alerts/host.go hostResourceID).
func TestMetricWindowPointsReadsHostAgentHistoryFromAlertReference(t *testing.T) {
	useRealMetricWindowMode(t)
	for _, persistent := range []bool{false, true} {
		t.Run(persistenceName(persistent), func(t *testing.T) {
			m := newMetricWindowRegistryMonitor(t, persistent)
			m.SetResourceStore(newMetricWindowAdapter())
			reportedAt := time.Now()
			host, err := m.ApplyHostReport(agentshost.Report{
				Agent:     agentshost.AgentInfo{ID: "host-agent", Version: "test", IntervalSeconds: 30},
				Host:      agentshost.HostInfo{ID: "host-agent", MachineID: "machine", Hostname: "worker", Platform: "linux"},
				Metrics:   agentshost.Metrics{CPUUsagePercent: 61},
				Timestamp: time.Now().UTC(),
			}, nil)
			if err != nil || host.ID == "" {
				t.Fatalf("ApplyHostReport: host %q, err %v", host.ID, err)
			}
			if persistent {
				dropInMemoryTail(m)
			}

			points, readAt := readCPUWindow(t, m, "agent:"+host.ID, "agent")
			assertOneCPUSample(t, "host agent CPU", points, host.CPUUsage, reportedAt, readAt)
		})
	}
}

// No alert evaluates a Docker host's CPU window today, but a canonical
// docker-host target must still read the series its report records.
func TestMetricWindowPointsReadsDockerHostHistoryFromCanonicalTarget(t *testing.T) {
	useRealMetricWindowMode(t)
	for _, persistent := range []bool{false, true} {
		t.Run(persistenceName(persistent), func(t *testing.T) {
			m := newMetricWindowRegistryMonitor(t, persistent)
			adapter := newMetricWindowAdapter()
			m.SetResourceStore(adapter)
			reportedAt := time.Now()
			host := applyMetricWindowDockerReport(t, m, strings.Repeat("ef", 32), 0, 37)

			hostResourceID := ""
			for _, resource := range adapter.GetAll() {
				if target := adapter.MetricsTargetForResource(resource.ID); target != nil &&
					target.ResourceType == "docker-host" && target.ResourceID == host.ID {
					hostResourceID = resource.ID
				}
			}
			if hostResourceID == "" {
				t.Fatalf("registry exposes no docker-host target for %q", host.ID)
			}
			if persistent {
				dropInMemoryTail(m)
			}

			points, readAt := readCPUWindow(t, m, hostResourceID, "docker-host")
			assertOneCPUSample(t, "Docker host CPU", points, host.CPUUsage, reportedAt, readAt)
		})
	}
}

// PVE guest and node alerts send the PVE source ID with a display type
// ("VM", "Container", "Node" from alerts unifiedAlertType); the registry's
// own form of an LXC is a canonical system-container target.
func TestMetricWindowPointsReadsPVEGuestAndNodeHistory(t *testing.T) {
	useRealMetricWindowMode(t)
	vm := models.VM{ID: "pve1:node1:100", VMID: 100, Name: "web", Node: "node1", Instance: "pve1"}
	ct := models.Container{ID: "pve1:node1:101", VMID: 101, Name: "cache", Node: "node1", Instance: "pve1"}
	node := models.Node{ID: "pve1-node1", Name: "node1", Instance: "pve1"}

	setup := func(t *testing.T, persistent bool) (*Monitor, *unifiedresources.MonitorAdapter, time.Time) {
		t.Helper()
		m := newMetricWindowRegistryMonitor(t, persistent)
		m.state.UpdateNodesForInstance("pve1", []models.Node{node})
		m.state.UpdateVMsForInstance("pve1", []models.VM{vm})
		m.state.UpdateContainersForInstance("pve1", []models.Container{ct})
		adapter := newMetricWindowAdapter()
		m.SetResourceStore(adapter) // rebuilt from the state above
		takenAt := time.Now()
		m.recordGuestMetric("vm", vm.ID, 41, -1, -1, -1, -1, -1, -1, -1, takenAt)
		m.recordGuestMetric("container", ct.ID, 42, -1, -1, -1, -1, -1, -1, -1, takenAt)
		// The node poller writes inline (monitor_pve_storage.go): node.ID in
		// node history and the node store family.
		m.metricsHistory.AddNodeMetric(node.ID, "cpu", 43, takenAt)
		if persistent {
			m.metricsStore.Write("node", node.ID, "cpu", 43, takenAt)
			dropInMemoryTail(m)
		}
		return m, adapter, takenAt
	}

	cases := []struct {
		name         string
		resourceID   string
		resourceType string
		canonical    unifiedresources.ResourceType
		want         float64
	}{
		{name: "VM alert subject", resourceID: vm.ID, resourceType: "VM", want: 41},
		{name: "LXC alert subject", resourceID: ct.ID, resourceType: "Container", want: 42},
		{name: "canonical system-container target", resourceType: "system-container", canonical: unifiedresources.ResourceTypeSystemContainer, want: 42},
		{name: "node alert subject", resourceID: node.ID, resourceType: "Node", want: 43},
	}
	for _, tc := range cases {
		for _, persistent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s", tc.name, persistenceName(persistent)), func(t *testing.T) {
				m, adapter, takenAt := setup(t, persistent)
				resourceID := tc.resourceID
				if tc.canonical != "" {
					for _, resource := range adapter.GetAll() {
						if resource.Type == tc.canonical {
							resourceID = resource.ID
						}
					}
					if resourceID == "" {
						t.Fatalf("registry has no %s resource", tc.canonical)
					}
				}

				points, readAt := readCPUWindow(t, m, resourceID, tc.resourceType)
				assertOneCPUSample(t, tc.name, points, tc.want, takenAt, readAt)
			})
		}
	}
}

// metricWindowReferenceStore answers target and alert-reference lookups from
// fixed tables, so a test can present each binding shape the registry returns.
type metricWindowReferenceStore struct {
	targets    map[string]*unifiedresources.MetricsTarget
	references map[string]metricWindowReference
}

type metricWindowReference struct {
	binding unifiedresources.AlertResourceReference
	claimed bool
}

func (*metricWindowReferenceStore) ShouldSkipAPIPolling(string) bool              { return false }
func (*metricWindowReferenceStore) GetPollingRecommendations() map[string]float64 { return nil }
func (*metricWindowReferenceStore) GetAll() []unifiedresources.Resource           { return nil }
func (*metricWindowReferenceStore) PopulateFromSnapshot(models.StateSnapshot)     {}
func (s *metricWindowReferenceStore) MetricsTargetForResource(resourceID string) *unifiedresources.MetricsTarget {
	return s.targets[resourceID]
}
func (s *metricWindowReferenceStore) ResolveAlertResourceReference(ref string) (unifiedresources.AlertResourceReference, bool) {
	reference := s.references[ref]
	return reference.binding, reference.claimed
}

// A binding that names another source of a merged resource, a component's
// owner, nothing at all, or the subject itself must leave the subject reading
// its own series.
func TestMetricWindowPointsKeepsSubjectSeriesForUnusableBindings(t *testing.T) {
	useRealMetricWindowMode(t)
	bound := func(resourceType unifiedresources.ResourceType, id string, component bool) metricWindowReference {
		return metricWindowReference{
			binding: unifiedresources.AlertResourceReference{ResourceID: id, ResourceType: resourceType, Component: component},
			claimed: true,
		}
	}
	guest := func(h *MetricsHistory, id string, value float64, at time.Time) {
		h.AddGuestMetric(id, "cpu", value, at)
	}
	cases := []struct {
		name         string
		resourceID   string
		resourceType string
		reference    metricWindowReference
		targets      map[string]*unifiedresources.MetricsTarget
		record       func(*MetricsHistory, string, float64, time.Time)
	}{
		{
			name:         "PVE node merged with its host agent",
			resourceID:   "pve1-node1",
			resourceType: "Node",
			reference:    bound(unifiedresources.ResourceTypeAgent, "machine-1", false),
			targets:      map[string]*unifiedresources.MetricsTarget{"machine-1": {ResourceType: "agent", ResourceID: "host-agent-1"}},
			record:       func(h *MetricsHistory, id string, value float64, at time.Time) { h.AddNodeMetric(id, "cpu", value, at) },
		},
		{
			name:         "host agent merged into the VM it runs in",
			resourceID:   "agent:host-agent-1",
			resourceType: "agent",
			reference:    bound(unifiedresources.ResourceTypeVM, "vm-canonical", false),
			targets:      map[string]*unifiedresources.MetricsTarget{"vm-canonical": {ResourceType: "vm", ResourceID: "pve1:node1:100"}},
			record:       guest,
		},
		{
			name:         "component of another resource",
			resourceID:   "agent:host-agent-1/disk/sda",
			resourceType: "agent",
			reference:    bound(unifiedresources.ResourceTypeAgent, "machine-1", true),
			targets:      map[string]*unifiedresources.MetricsTarget{"machine-1": {ResourceType: "agent", ResourceID: "host-agent-1"}},
			record:       guest,
		},
		{
			name:         "conflicting inventory",
			resourceID:   "docker:host-1/" + strings.Repeat("ab", 32),
			resourceType: "app-container",
			reference:    metricWindowReference{claimed: true},
			record:       guest,
		},
		{
			name:         "unknown subject",
			resourceID:   "docker:host-1/" + strings.Repeat("cd", 32),
			resourceType: "app-container",
			record:       guest,
		},
		{
			name:         "binding to itself",
			resourceID:   "pve1:node1:100",
			resourceType: "VM",
			reference:    bound(unifiedresources.ResourceTypeVM, "pve1:node1:100", false),
			record:       guest,
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			history := NewMetricsHistory(32, time.Hour)
			at := time.Now().Add(-time.Minute)
			value := float64(20 + i)
			tc.record(history, tc.resourceID, value, at)
			store := &metricWindowReferenceStore{
				targets:    tc.targets,
				references: map[string]metricWindowReference{tc.resourceID: tc.reference},
			}
			m := &Monitor{metricsHistory: history, resourceStore: store}

			points, _ := readCPUWindow(t, m, tc.resourceID, tc.resourceType)
			if len(points) != 1 || points[0].Value != value || !points[0].Timestamp.Equal(at) {
				t.Fatalf("window = %+v, want the subject's own %.0f sample", points, value)
			}
		})
	}
}

// A single durable observation has zero time coverage, but it is still an
// observation. The alert evaluator, rather than the reader, rejects shallow
// evidence for a rolling average.
func TestMetricWindowPointsPreservesSinglePersistentObservation(t *testing.T) {
	useRealMetricWindowMode(t)
	m := newChartFallbackTestMonitor(t)
	at := time.Now().Add(-time.Minute).Truncate(time.Second)
	m.recordGuestMetric("vm", "pve1:node1:100", 95, -1, -1, -1, -1, -1, -1, -1, at)
	dropInMemoryTail(m)
	stored, err := m.metricsStore.Query("vm", "pve1:node1:100", "cpu", at.Add(-time.Minute), time.Now(), 0)
	if err != nil || len(stored) != 1 || stored[0].Value != 95 {
		t.Fatalf("writer's durable observation = %+v, err %v", stored, err)
	}
	points, readAt := readCPUWindow(t, m, "pve1:node1:100", "VM")
	assertOneCPUSample(t, "single durable observation", points, 95, at, readAt)
}

// The real writers retain independent node/agent histories even when a
// canonical owner prefers another source. Losing the live tail must preserve
// that same source boundary, including the agent reference's store ID.
func TestMetricWindowPointsKeepsMergedSourceHistoryAfterTailLoss(t *testing.T) {
	useRealMetricWindowMode(t)
	writer := newMetricWindowRegistryMonitor(t, true)
	reportedAt := time.Now()
	host, err := writer.ApplyHostReport(agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: "linked-agent", Version: "test", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: "linked-agent", MachineID: "linked-machine", Hostname: "linked-worker", Platform: "linux"},
		Metrics:   agentshost.Metrics{CPUUsagePercent: 61},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const nodeID, vmID = "pve1-node1", "pve1:node1:100"
	at := time.Now()
	// These are the node poller's inline writer calls, as in the PVE guard.
	writer.metricsHistory.AddNodeMetric(nodeID, "cpu", 43, at)
	writer.metricsStore.Write("node", nodeID, "cpu", 43, at)
	writer.recordGuestMetric("vm", vmID, 99, -1, -1, -1, -1, -1, -1, -1, at)
	dropInMemoryTail(writer)
	agentReference := "agent:" + host.ID
	bindings := &metricWindowReferenceStore{
		targets: map[string]*unifiedresources.MetricsTarget{
			"agent-canonical": {ResourceType: "agent", ResourceID: host.ID},
			"vm-canonical":    {ResourceType: "vm", ResourceID: vmID},
		},
		references: map[string]metricWindowReference{
			nodeID:         {binding: unifiedresources.AlertResourceReference{ResourceID: "agent-canonical", ResourceType: unifiedresources.ResourceTypeAgent}, claimed: true},
			agentReference: {binding: unifiedresources.AlertResourceReference{ResourceID: "vm-canonical", ResourceType: unifiedresources.ResourceTypeVM}, claimed: true},
		},
	}
	provider := &Monitor{metricsHistory: writer.metricsHistory, metricsStore: writer.metricsStore, resourceStore: bindings}
	for _, tc := range []struct {
		name, reference, subjectType, storeType, storeID string
		want                                             float64
	}{
		{"node merged with agent", nodeID, "Node", "node", nodeID, 43},
		{"agent merged into VM", agentReference, "agent", "agent", host.ID, 61},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := writer.metricsStore.Query(tc.storeType, tc.storeID, "cpu", reportedAt.Add(-time.Minute), time.Now(), 0)
			if err != nil || len(stored) != 1 || stored[0].Value != tc.want {
				t.Fatalf("own writer observation = %+v, err %v", stored, err)
			}
			points, readAt := readCPUWindow(t, provider, tc.reference, tc.subjectType)
			assertOneCPUSample(t, "merged source", points, tc.want, reportedAt, readAt)
		})
	}
}

// The factory five-minute CPU rule must be able to fire for a Docker container
// and later recover through the production provider. The alert manager's
// clock is not reachable from this package, so window contents are placed
// relative to the real clock instead: the report path supplies the
// evaluation, the history key is learned from the real writer, and earlier
// samples are added under that key.
func TestFactoryCPUWindowFiresAndRecoversForDockerContainer(t *testing.T) {
	useRealMetricWindowMode(t)
	m := newMetricWindowRegistryMonitor(t, false)
	m.alertManager.Stop()
	m.alertManager = alerts.NewManagerWithDataDir(t.TempDir())
	m.SetResourceStore(newMetricWindowAdapter())

	cfg := m.alertManager.GetConfig()
	cfg.ActivationState = alerts.ActivationActive
	m.alertManager.UpdateConfig(cfg)
	if window := m.alertManager.GetConfig().MetricEvaluationWindows["all"]["cpu"]; window != 300 {
		t.Fatalf("factory CPU window = %ds, want 300", window)
	}
	m.alertManager.SetMetricWindowProvider(m.metricWindowPoints) // as New wires it

	containerID := strings.Repeat("cd", 32)
	host := applyMetricWindowDockerReport(t, m, containerID, 380, 0)
	busy := models.DockerContainerCPUCapacityPercent(host.Containers[0], host.CPUs)
	if busy < 80 {
		t.Fatalf("fixture container CPU = %.1f%%, want above the 80%% factory trigger", busy)
	}
	cpuAlert := func() *alerts.Alert {
		for _, alert := range m.alertManager.GetActiveAlerts() {
			if alert.Type == "cpu" && strings.Contains(alert.ResourceID, containerID) {
				return &alert
			}
		}
		return nil
	}
	reference := alerts.DockerContainerResourceID(host.ID, containerID, metricWindowContainerName)
	policy := m.alertManager.ResolveEffectiveIntentPolicy(reference, "app-container", alerts.MetricAlertIntentSignal("cpu"))
	if policy.GraceSeconds != 5 {
		t.Fatalf("factory CPU grace = %ds, want 5", policy.GraceSeconds)
	}
	proof := func(phase string) {
		t.Helper()
		if os.Getenv("PULSE_METRIC_WINDOW_PROOF") == "" {
			return
		}
		observed := make([]alerts.Alert, 0, 1)
		if alert := cpuAlert(); alert != nil {
			observed = append(observed, *alert)
		}
		data, err := json.Marshal(map[string]any{"phase": phase, "alerts": observed, "windowSeconds": 300, "graceSeconds": policy.GraceSeconds, "hostID": host.ID, "containerID": containerID})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("METRIC_WINDOW_PROOF %s\n", data)
	}
	if alert := cpuAlert(); alert != nil {
		t.Fatalf("CPU alert opened without window history: %+v", alert)
	}
	proof("shallow")

	key := ""
	m.metricsHistory.mu.RLock()
	for candidate, series := range m.metricsHistory.guestMetrics {
		if n := len(series.CPU); n > 0 && series.CPU[n-1].Value == busy {
			key = candidate
		}
	}
	m.metricsHistory.mu.RUnlock()
	if key == "" {
		t.Fatal("the Docker report recorded no container CPU history")
	}
	// Five minutes of history at one level: three samples 90 s apart, then
	// the evaluation's own current value.
	seed := func(value float64) {
		m.metricsHistory.Reset()
		now := time.Now()
		for _, age := range []time.Duration{270 * time.Second, 180 * time.Second, 90 * time.Second} {
			m.metricsHistory.AddGuestMetric(key, "cpu", value, now.Add(-age))
		}
	}

	seed(busy)
	applyMetricWindowDockerReport(t, m, containerID, 380, 0)
	if alert := cpuAlert(); alert != nil {
		t.Fatalf("CPU alert bypassed the factory grace: %+v", alert)
	}
	proof("pending")
	time.Sleep(time.Duration(policy.GraceSeconds)*time.Second + 100*time.Millisecond)
	applyMetricWindowDockerReport(t, m, containerID, 380, 0)
	alert := cpuAlert()
	if alert == nil {
		t.Fatalf("factory CPU window never fired for a container held at %.1f%%", busy)
	}
	samples := 0
	switch count := alert.Metadata["evaluationSampleCount"].(type) {
	case int:
		samples = count
	case float64:
		samples = int(count)
	}
	if alert.Metadata["evaluationMode"] != "rolling_average" || alert.Metadata["evaluationWindowSeconds"] != 300 || samples < 4 {
		t.Fatalf("alert was not raised from the rolling window: %+v", alert.Metadata)
	}
	incidentID, startedAt := alert.ID, alert.StartTime
	proof("active")

	// An idle reading without history is unknown, not recovery.
	m.metricsHistory.Reset()
	applyMetricWindowDockerReport(t, m, containerID, 0, 0)
	if unknown := cpuAlert(); unknown == nil || unknown.ID != incidentID || !unknown.StartTime.Equal(startedAt) {
		t.Fatalf("unknown history changed the incident: %+v", unknown)
	}
	proof("unknown")

	seed(10)
	applyMetricWindowDockerReport(t, m, containerID, 0, 0)
	if alert := cpuAlert(); alert != nil {
		t.Fatalf("CPU alert did not recover after a quiet five-minute window: %+v", alert)
	}
	proof("recovered")
}

// PVE node alerts already had readable live windows on the baseline. Measure
// complete healthy alert passes over those same observations so the extra
// source-reference resolution is compared without changing readiness.
func BenchmarkMetricWindowNodeAlertPass(b *testing.B) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := mock.SetEnabled(previous); err != nil {
			b.Error(err)
		}
	})
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprintf("nodes-%d", count), func(b *testing.B) {
			now := time.Now()
			history := NewMetricsHistory(16, time.Hour)
			nodes := make([]models.Node, count)
			for i := range nodes {
				nodes[i] = models.Node{ID: fmt.Sprintf("bench-node-%d", i), Name: fmt.Sprintf("node-%d", i), Instance: "bench", Status: "online", CPU: 0.1}
				for _, age := range []time.Duration{270 * time.Second, 180 * time.Second, 90 * time.Second, 0} {
					history.AddNodeMetric(nodes[i].ID, "cpu", 0.1, now.Add(-age))
				}
			}
			adapter := newMetricWindowAdapter()
			adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: nodes})
			monitor := &Monitor{metricsHistory: history, resourceStore: adapter}
			manager := alerts.NewManagerWithDataDir(b.TempDir())
			b.Cleanup(manager.Stop)
			cfg := manager.GetConfig()
			cfg.ActivationState = alerts.ActivationActive
			manager.UpdateConfig(cfg)
			windowCalls, readyWindows := 0, 0
			manager.SetMetricWindowProvider(func(request alerts.MetricWindowRequest) ([]alerts.MetricWindowPoint, error) {
				windowCalls++
				points, err := monitor.metricWindowPoints(request)
				if err == nil && len(points) == 4 && points[3].Timestamp.Sub(points[0].Timestamp) >= 240*time.Second {
					readyWindows++
				}
				return points, err
			})
			if binding, claimed := adapter.ResolveAlertResourceReference(nodes[0].ID); !claimed || binding.ResourceID == "" {
				b.Fatalf("benchmark node reference does not bind: %+v", binding)
			}
			// Warm the generation-scoped registry indexes outside the timed pass.
			manager.CheckNode(nodes[0])
			if windowCalls == 0 || readyWindows == 0 {
				b.Fatal("benchmark must evaluate a ready CPU window")
			}
			before := readyWindows
			b.ReportAllocs()
			b.ResetTimer()
			for pass := 0; pass < b.N; pass++ {
				for _, node := range nodes {
					manager.CheckNode(node)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*count), "ns/subject")
			if readyWindows-before != b.N*count {
				b.Fatalf("ready CPU windows = %d, want %d", readyWindows-before, b.N*count)
			}
			if got := manager.GetActiveAlerts(); len(got) != 0 {
				b.Fatalf("healthy benchmark opened incidents: %+v", got)
			}
		})
	}
}
