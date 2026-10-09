package alerts

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise the current CheckHost/CheckNode/config entry points, independently
// of the proposal's new registry helpers, so the same controls run on its parent.
func TestReportedAlertOwnershipComposition(t *testing.T) {
	setup := func(t *testing.T) (*Manager, models.Node, models.Host) {
		t.Helper()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.NodeDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.config.NodeDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
		m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
		m.mu.Unlock()
		node, host := testNodeWithHostAgent()
		node.CPU = 0.9
		node.Memory = models.Memory{Total: 100, Used: 90, Free: 10, Usage: 90}
		node.Disk = models.Disk{Total: 100, Used: 95, Free: 5, Usage: 95}
		host.CPUUsage = 50
		host.Memory = node.Memory
		return m, node, host
	}
	for _, kind := range []string{"memory-never-observed", "cpu-nan", "cpu-infinite", "window-not-ready", "summary-disk-nan"} {
		t.Run(kind, func(t *testing.T) {
			m, node, host := setup(t)
			metric := "cpu"
			switch kind {
			case "memory-never-observed":
				metric = "memory"
				host.Memory = models.Memory{Total: 100, UsageUnavailable: true}
			case "cpu-nan":
				host.CPUUsage = math.NaN()
			case "cpu-infinite":
				host.CPUUsage = math.Inf(1)
			case "window-not-ready":
				m.mu.Lock()
				m.config.MetricEvaluationWindows = map[string]map[string]int{"agent": {"cpu": 300}}
				m.mu.Unlock()
				m.SetMetricWindowProvider(func(MetricWindowRequest) ([]MetricWindowPoint, error) { return nil, nil })
			case "summary-disk-nan":
				metric = "disk"
				host.Disks = []models.Disk{{Mountpoint: "/", Device: "/dev/sda1", Total: 100, Usage: math.NaN()}}
			}
			m.CheckHost(host)
			m.CheckNode(node)
			if !testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, metric)) {
				t.Fatalf("node %s lost %s to an agent with no evidence or open run", node.ID, metric)
			}
		})
	}
	t.Run("config-save-without-agent-report", func(t *testing.T) {
		m, node, host := setup(t)
		m.CheckHost(host)
		m.CheckNode(node)
		cfg := m.GetConfig()
		cfg.DisableAllAgents = true
		m.UpdateConfig(cfg)
		m.CheckNode(node)
		m.mu.RLock()
		observed := testCoreHasIncident(m, node.ID, canonicalMetricSpecID(node.ID, "cpu"))
		m.mu.RUnlock()
		if !observed {
			t.Fatal("disabled agent left node CPU unevaluated until another agent report")
		}
	})
	t.Run("direct-disk-disable-without-agent-report", func(t *testing.T) {
		m, node, host := setup(t)
		host.Disks = []models.Disk{{Mountpoint: "/", Device: "/dev/nvme0n1p2", Total: 100, Usage: 50}}
		m.CheckHost(host)
		m.CheckNode(node)
		if testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, "disk")) {
			t.Fatal("control failed to hand summary disk coverage to the agent")
		}
		cfg := m.GetConfig()
		resourceID, _ := hostDiskResourceID(host, host.Disks[0])
		cfg.Overrides = map[string]ThresholdConfig{resourceID: {Disk: &HysteresisThreshold{Trigger: 0}}}
		m.UpdateConfig(cfg)
		m.CheckNode(node)
		m.mu.RLock()
		observed := testCoreHasIncident(m, node.ID, canonicalMetricSpecID(node.ID, "disk"))
		m.mu.RUnlock()
		if !observed {
			t.Fatal("saved direct disk disable left node disk unevaluated until another agent report")
		}
	})
	t.Run("nil-disk-threshold-releases-pending", func(t *testing.T) {
		m, _, host := setup(t)
		host.Disks = []models.Disk{{Mountpoint: "/", Device: "/dev/sda1", Total: 100, Usage: 92}}
		m.mu.Lock()
		m.config.MetricTimeThresholds = map[string]map[string]int{"agent-disk": {"disk": 300}}
		m.mu.Unlock()
		m.CheckHost(host)
		resourceID, _ := hostDiskResourceID(host, host.Disks[0])
		specID := canonicalMetricSpecID(resourceID, "disk")
		m.mu.Lock()
		if !testCoreIsPending(m, resourceID, specID) {
			m.mu.Unlock()
			t.Fatal("control did not create a pending disk run")
		}
		m.config.AgentDefaults.Disk = nil
		m.mu.Unlock()
		m.CheckHost(host)
		m.mu.RLock()
		defer m.mu.RUnlock()
		if testCoreHasIncident(m, resourceID, specID) {
			t.Fatal("nil disk threshold retained a pending disk run")
		}
	})
	for _, policy := range []string{"global", "host", "threshold"} {
		t.Run("pending-release-"+policy, func(t *testing.T) {
			m, _, host := setup(t)
			m.EnableShadowFeed()
			host.Memory = models.Memory{Total: 100, Used: 90, Free: 10, Usage: 90}
			m.mu.Lock()
			m.config.MetricTimeThresholds = map[string]map[string]int{"agent": {"memory": 300}}
			m.mu.Unlock()
			m.CheckHost(host)
			resourceID := hostResourceID(host.ID)
			specID := canonicalMetricSpecID(resourceID, "memory")
			m.mu.Lock()
			if !testCoreIsPending(m, resourceID, specID) {
				m.mu.Unlock()
				t.Fatal("control did not create a pending canonical memory run")
			}
			// Canonical metrics do not feed diagnostic shadow evaluation.
			// Seed the matching diagnostic run explicitly to pin release mirroring.
			m.shadow.state.ApplyMetric(reducer.MetricSignal{ResourceID: resourceID, Key: specID, Value: 90, ObservedAt: time.Now()}, reducer.MetricRule{Trigger: 85, Clear: 80, DelaySeconds: 300})
			for _, state := range m.mirrorStatesNoLock() {
				if incident, ok := state.Incident(resourceID, specID); !ok || string(incident.State) != "pending" {
					m.mu.Unlock()
					t.Fatal("control did not create a pending canonical/shadow memory run")
				}
			}
			switch policy {
			case "global":
				m.config.DisableAllAgents = true
			case "host":
				m.config.Overrides = map[string]ThresholdConfig{host.ID: {Disabled: true}}
			case "threshold":
				m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 0}
			}
			m.mu.Unlock()
			host.Memory = models.Memory{Total: 100, UsageUnavailable: true}
			m.CheckHost(host)
			m.mu.RLock()
			defer m.mu.RUnlock()
			for _, state := range m.mirrorStatesNoLock() {
				if _, ok := state.Incident(resourceID, specID); ok {
					t.Fatal("disabled interval retained a pending canonical/shadow memory run")
				}
			}
		})
	}
	for _, scenario := range []string{"newer-unlink", "newer-cover", "offline", "removed", "config-disabled"} {
		t.Run("in-flight-"+scenario, func(t *testing.T) {
			m, node, host := setup(t)
			m.mu.Lock()
			m.config.MetricEvaluationWindows = map[string]map[string]int{"agent": {"cpu": 60}}
			m.mu.Unlock()
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			var once sync.Once
			defer once.Do(func() { close(release) })
			m.SetMetricWindowProvider(func(r MetricWindowRequest) ([]MetricWindowPoint, error) {
				if r.ResourceID == hostResourceID(host.ID) && r.Metric == "cpu" && calls.Add(1) == 1 {
					close(entered)
					<-release
				}
				return []MetricWindowPoint{{Timestamp: r.Start, Value: 50}, {Timestamp: r.Start.Add(30 * time.Second), Value: 50}}, nil
			})
			older := host
			if scenario == "newer-cover" {
				older.LinkedNodeID = ""
			}
			go func() { defer close(done); m.CheckHost(older) }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("older report did not reach its evaluation window")
			}
			wantCover := false
			switch scenario {
			case "newer-unlink":
				newer := host
				newer.LinkedNodeID = ""
				m.CheckHost(newer)
			case "newer-cover":
				m.CheckHost(host)
				wantCover = true
			case "offline":
				m.HandleHostOffline(host)
			case "removed":
				m.HandleHostRemoved(host)
			case "config-disabled":
				cfg := m.GetConfig()
				cfg.DisableAllAgents = true
				m.UpdateConfig(cfg)
			}
			once.Do(func() { close(release) })
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("older report did not finish")
			}
			if got := m.hasHostAgentForNode(node.ID); got != wantCover {
				t.Fatalf("coverage after %s = %v, want %v", scenario, got, wantCover)
			}
		})
	}
	t.Run("handover-names-metric-owner", func(t *testing.T) {
		m, node, host := setup(t)
		m.CheckNode(node)
		var closed []*ResolvedAlert
		m.SetResolvedAlertCallback(func(a *ResolvedAlert) { closed = append(closed, a) })
		a := host
		a.ID = "agent-a"
		a.DisplayName = "CPU reader"
		a.CPUUsage = 50
		b := host
		b.ID = "agent-b"
		b.DisplayName = "Memory reader"
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			a.ID: {Memory: &HysteresisThreshold{Trigger: 0}, Disk: &HysteresisThreshold{Trigger: 0}},
			b.ID: {CPU: &HysteresisThreshold{Trigger: 0}, Disk: &HysteresisThreshold{Trigger: 0}},
		}
		m.mu.Unlock()
		m.CheckHost(a)
		m.CheckHost(b)
		m.CheckNode(node)
		seen := map[string]bool{}
		for _, a := range closed {
			if a.Alert.ResourceID != node.ID {
				continue
			}
			expected := "agent:agent-a"
			if a.Alert.Type == "memory" {
				expected = "agent:agent-b"
			}
			if a.Alert.Resolution == nil || a.Alert.Resolution.Reason != AlertResolutionMovedToAgent || a.Alert.Resolution.SuccessorResourceID != expected {
				t.Fatalf("%s close named wrong successor: %+v", a.Alert.Type, a.Alert.Resolution)
			}
			seen[a.Alert.Type] = true
		}
		if !seen["cpu"] || !seen["memory"] {
			t.Fatalf("missing node handover callbacks: %v", seen)
		}
		if !testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, "disk")) {
			t.Fatal("node disk lost coverage even though neither agent owns a summary filesystem")
		}
	})
}
