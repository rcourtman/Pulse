package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func nodeDiskManager(t *testing.T, intent bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m, clock := continuityManager(t, intent)
	m.mu.Lock()
	m.config.NodeDefaults = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	if !intent {
		m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0}}
	}
	m.mu.Unlock()
	return m, clock
}

func nodeDiskSample(id string, usage float64) models.Node {
	return models.Node{ID: id, Name: "pve", Instance: "site", Status: "online", Disk: models.Disk{Total: 1000, Used: int64(usage * 10), Usage: usage}}
}

func nodeDiskGap(node models.Node, gap string) models.Node {
	switch gap {
	case "missing":
		node.Disk = models.Disk{}
	case "capacity-only":
		node.Disk = models.Disk{Total: 1000, Used: -1}
	case "no-capacity":
		node.Disk.Total = 0
	case "negative-capacity":
		node.Disk.Total = -1
	case "over-capacity":
		node.Disk.Used = 1001
	case "negative":
		node.Disk.Usage = -1
	case "over-100":
		node.Disk.Usage = 101
	case "NaN":
		node.Disk.Usage = math.NaN()
	case "+Inf":
		node.Disk.Usage = math.Inf(1)
	}
	return node
}

func nodeDiskJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNodeDiskUnknownPreservesOccurrence(t *testing.T) {
	for _, gap := range []string{"missing", "capacity-only", "no-capacity", "negative-capacity", "over-capacity", "negative", "over-100", "NaN", "+Inf"} {
		t.Run(gap, func(t *testing.T) {
			m, clock := nodeDiskManager(t, false)
			node := nodeDiskSample("site-pve", 95)
			m.CheckNode(node)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("known high disk must fire")
			}
			id := m.GetActiveAlerts()[0].ID
			if err := m.AcknowledgeAlert(id, "operator"); err != nil {
				t.Fatal(err)
			}
			before := nodeDiskJSON(t, m.GetActiveAlerts())
			events := len(queryAlertEvents(t, m, eventlog.Filter{}))
			clock.Store(int64(2 * time.Minute))
			for range 3 {
				m.CheckNode(nodeDiskGap(node, gap))
			}
			if !bytes.Equal(before, nodeDiskJSON(t, m.GetActiveAlerts())) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
				t.Fatal("unknown node disk changed occurrence, acknowledgement, trusted evidence or lifecycle")
			}
			m.CheckNode(nodeDiskSample(node.ID, 0))
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeResolved}})) != 1 {
				t.Fatal("measured zero must resolve the original once")
			}
		})
	}
}

func TestNodeDiskUnknownNeverStartsAnIncident(t *testing.T) {
	for _, gap := range []string{"missing", "capacity-only", "no-capacity", "negative-capacity", "over-capacity", "negative", "over-100", "NaN", "+Inf"} {
		t.Run(gap, func(t *testing.T) {
			m, clock := nodeDiskManager(t, false)
			node := nodeDiskGap(nodeDiskSample("site-pve", 95), gap)
			m.CheckNode(node)
			clock.Store(int64(time.Hour))
			m.CheckNode(node)
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
				t.Fatal("unavailable node disk invented an incident")
			}
		})
	}
}

func TestNodeDiskGapRestartsActivation(t *testing.T) {
	for _, intent := range []bool{false, true} {
		for _, gap := range []string{"missing", "no-capacity", "over-capacity"} {
			t.Run(fmt.Sprintf("intent=%v/%s", intent, gap), func(t *testing.T) {
				m, clock := nodeDiskManager(t, intent)
				m.mu.Lock()
				m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 60}}
				m.mu.Unlock()
				node := nodeDiskSample("site-pve", 85)
				other := nodeDiskSample("other-pve", 85)
				m.CheckNode(node)
				m.CheckNode(other)
				incident, exists := continuityIncident(m, node.ID, "disk")
				if !exists || incident.State != reducer.StatePending {
					t.Fatal("control must create timed activation")
				}
				clock.Store(int64(2 * time.Minute))
				m.CheckNode(nodeDiskGap(node, gap))
				if _, exists := continuityIncident(m, node.ID, "disk"); exists {
					t.Fatal("unknown retained timed disk evidence")
				}
				if _, exists := continuityIncident(m, other.ID, "disk"); !exists {
					t.Fatal("gap crossed exact node identity")
				}
				m.CheckNode(node)
				clock.Store(int64(2*time.Minute + 59*time.Second))
				m.CheckNode(node)
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("post-gap run completed early")
				}
				clock.Store(int64(3 * time.Minute))
				m.CheckNode(node)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("full fresh run did not fire")
				}
				if m.ShadowDivergences() != 0 {
					t.Fatal("core and shadow diverged")
				}
			})
		}
	}
}

func TestNodeDiskUnknownHonoursExplicitPolicy(t *testing.T) {
	for _, policy := range []string{"nil-threshold", "zero-threshold", "override", "node-disabled", "all-nodes-disabled"} {
		for _, gap := range []string{"missing", "NaN"} {
			t.Run(policy+"/"+gap, func(t *testing.T) {
				m, _ := nodeDiskManager(t, false)
				node := nodeDiskSample("site-pve", 95)
				m.CheckNode(node)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("known disk control must fire")
				}
				// Direct changes exercise CheckNode's policy handling, not UpdateConfig cleanup.
				m.mu.Lock()
				switch policy {
				case "nil-threshold":
					m.config.NodeDefaults.Disk = nil
				case "zero-threshold":
					m.config.NodeDefaults.Disk = &HysteresisThreshold{}
				case "override":
					m.config.Overrides[node.ID] = ThresholdConfig{Disk: &HysteresisThreshold{}}
				case "node-disabled":
					m.config.Overrides[node.ID] = ThresholdConfig{Disabled: true}
				case "all-nodes-disabled":
					m.config.DisableAllNodes = true
				}
				m.mu.Unlock()
				m.CheckNode(nodeDiskGap(node, gap))
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("unavailable disk overrode explicit alert policy")
				}
			})
		}
	}
}

func TestNodeDiskUnknownLeavesIndependentMetricsAndAgentHandover(t *testing.T) {
	m, _ := nodeDiskManager(t, false)
	m.mu.Lock()
	m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.config.NodeDefaults.Memory = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0, "cpu": 0, "memory": 0}}
	m.mu.Unlock()
	node := nodeDiskSample("site-pve", 95)
	m.CheckNode(node)
	gap := nodeDiskGap(node, "missing")
	gap.CPU = .95
	gap.Memory = models.Memory{Total: 1000, Used: 950, Usage: 95}
	m.CheckNode(gap)
	if len(m.GetActiveAlerts()) != 3 {
		t.Fatal("disk gap masked independently observed CPU or memory")
	}
	// A real linked agent still takes over its disk; absence is not another owner.
	host := hostFilesystemSample("host-node-disk", 95)
	host.LinkedNodeID = node.ID
	m.mu.Lock()
	m.config.AgentDefaults = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.mu.Unlock()
	m.CheckHost(host)
	m.CheckNode(gap)
	if testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, "disk")) {
		t.Fatal("real agent handover left duplicate node disk")
	}
	if !testHasActiveAlert(t, m, canonicalMetricStateID(hostFilesystemID(host), "disk")) {
		t.Fatal("handover lost agent disk incident")
	}
	m.HandleHostRemoved(host)
	m.CheckNode(gap)
	if testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, "disk")) {
		t.Fatal("removing agent invented a node reading")
	}
	m.CheckNode(node)
	if !testHasActiveAlert(t, m, canonicalMetricStateID(node.ID, "disk")) {
		t.Fatal("fresh node disk did not regain ownership")
	}
}

// Export only opt-in synthetic manager frames for the real consuming UI proof.
func TestNodeDiskBrowserProducer(t *testing.T) {
	m, clock := nodeDiskManager(t, false)
	node := nodeDiskSample("site-browser-pve", 95)
	frames := map[string]json.RawMessage{}
	for _, phase := range []string{"breach", "missing", "over-capacity", "recovered"} {
		sample := node
		if phase == "recovered" {
			sample = nodeDiskSample(node.ID, 0)
		} else if phase != "breach" {
			sample = nodeDiskGap(node, phase)
		}
		clock.Add(int64(time.Second))
		m.CheckNode(sample)
		active := m.GetActiveAlerts()
		want := 1
		if phase == "recovered" {
			want = 0
		}
		if len(active) != want {
			t.Fatalf("phase %s: %d active, want %d", phase, len(active), want)
		}
		frames[phase] = nodeDiskJSON(t, active)
	}
	if !bytes.Equal(frames["breach"], frames["missing"]) || !bytes.Equal(frames["breach"], frames["over-capacity"]) {
		t.Fatal("unknown changed exported occurrence")
	}
	if os.Getenv("PULSE_NODE_DISK_FRAMES") == "1" {
		fmt.Printf("NODE_DISK_FRAMES=%s\n", nodeDiskJSON(t, frames))
	}
}
