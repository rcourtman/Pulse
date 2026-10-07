package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
)

func TestConvertHostSensorsToTemperature_Empty(t *testing.T) {
	sensors := models.HostSensorSummary{}
	result := convertHostSensorsToTemperature(sensors, time.Now())
	if result != nil {
		t.Error("expected nil for empty sensors")
	}
}

func TestConvertHostSensorsToTemperature_CPUOnly(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 55.0,
			"cpu_core_0":  50.0,
			"cpu_core_1":  52.0,
		},
	}
	now := time.Now()
	result := convertHostSensorsToTemperature(sensors, now)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Available {
		t.Error("expected Available to be true")
	}
	if !result.HasCPU {
		t.Error("expected HasCPU to be true")
	}
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage 55.0, got %f", result.CPUPackage)
	}
	if len(result.Cores) != 2 {
		t.Errorf("expected 2 cores, got %d", len(result.Cores))
	}
	// Cores should be sorted
	if result.Cores[0].Core != 0 || result.Cores[1].Core != 1 {
		t.Error("cores not sorted correctly")
	}
	if result.CPUMax != 52.0 { // Max of core temps
		t.Errorf("expected CPUMax 52.0, got %f", result.CPUMax)
	}
}

func TestConvertHostSensorsToTemperature_NVMe(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"nvme0":       40.0,
			"nvme1":       42.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasNVMe {
		t.Error("expected HasNVMe to be true")
	}
	if len(result.NVMe) != 2 {
		t.Errorf("expected 2 NVMe devices, got %d", len(result.NVMe))
	}
	// NVMe should be sorted
	if result.NVMe[0].Device != "nvme0" || result.NVMe[1].Device != "nvme1" {
		t.Error("NVMe devices not sorted correctly")
	}
}

func TestConvertHostSensorsToTemperature_GPU(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package":  45.0,
			"gpu_edge":     60.0,
			"gpu_junction": 65.0,
			"gpu_mem":      55.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU, got %d", len(result.GPU))
	}
	if result.GPU[0].Device != "gpu0" {
		t.Errorf("expected device 'gpu0', got %q", result.GPU[0].Device)
	}
	if result.GPU[0].Edge != 60.0 {
		t.Errorf("expected Edge 60.0, got %f", result.GPU[0].Edge)
	}
	if result.GPU[0].Junction != 65.0 {
		t.Errorf("expected Junction 65.0, got %f", result.GPU[0].Junction)
	}
}

func TestConvertHostSensorsToTemperature_GenericGPU(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"gpu_nvidia":  70.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU, got %d", len(result.GPU))
	}
	if result.GPU[0].Device != "nvidia" {
		t.Errorf("expected device 'nvidia', got %q", result.GPU[0].Device)
	}
	if result.GPU[0].Edge != 70.0 {
		t.Errorf("expected Edge 70.0, got %f", result.GPU[0].Edge)
	}
}

func TestConvertHostSensorsToTemperature_NoPackageUsesMaxCore(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_core_0": 50.0,
			"cpu_core_1": 55.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// When no package temp, CPUPackage should use max core temp
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage to be max core temp 55.0, got %f", result.CPUPackage)
	}
}

func TestShouldSkipTemperatureSSHCollection(t *testing.T) {
	t.Run("nil host agent temp does not skip", func(t *testing.T) {
		if shouldSkipTemperatureSSHCollection(nil) {
			t.Fatal("expected nil host agent temp not to skip SSH collection")
		}
	})

	t.Run("host agent temp without recent timestamp does not skip", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 55,
		}
		if shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected CPU-only host agent temp without a recent timestamp not to skip SSH collection")
		}
	})

	t.Run("recent cpu package host agent temp skips", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 55,
			LastUpdate: time.Now(),
		}
		if !shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected recent CPU host agent temp to skip legacy SSH collection")
		}
	})

	t.Run("recent cpu core host agent temp skips", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasCPU:     true,
			Cores:      []models.CoreTemp{{Core: 0, Temp: 51}},
			LastUpdate: time.Now(),
		}
		if !shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected recent CPU core host agent temp to skip legacy SSH collection")
		}
	})

	t.Run("recent nvme host agent temp skips", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasNVMe:    true,
			NVMe:       []models.NVMeTemp{{Device: "nvme0", Temp: 42}},
			LastUpdate: time.Now(),
		}
		if !shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected recent NVMe host agent temp to skip legacy SSH collection")
		}
	})

	t.Run("recent gpu host agent temp skips", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasGPU:     true,
			GPU:        []models.GPUTemp{{Device: "gpu0", Edge: 62}},
			LastUpdate: time.Now(),
		}
		if !shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected recent GPU host agent temp to skip legacy SSH collection")
		}
	})

	t.Run("stale cpu only host agent temp does not skip", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 55,
			LastUpdate: time.Now().Add(-3 * time.Minute),
		}
		if shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected stale CPU-only host agent temp not to skip SSH collection")
		}
	})

	t.Run("unavailable host agent temp does not skip", func(t *testing.T) {
		host := &models.Temperature{
			Available:  false,
			HasCPU:     true,
			CPUPackage: 55,
			LastUpdate: time.Now(),
		}
		if shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected unavailable host agent temp not to skip SSH collection")
		}
	})

	t.Run("host agent smart data skips", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasCPU:     true,
			HasSMART:   true,
			SMART:      []models.DiskTemp{{Device: "/dev/sda", Temperature: 35}},
			LastUpdate: time.Now(),
		}
		if !shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected SMART-capable host agent temp to skip SSH collection")
		}
	})

	t.Run("host agent smart inventory without any temperature does not skip", func(t *testing.T) {
		host := &models.Temperature{
			Available:  true,
			HasSMART:   true,
			SMART:      []models.DiskTemp{{Device: "/dev/sda", Temperature: 0}},
			LastUpdate: time.Now(),
		}
		if shouldSkipTemperatureSSHCollection(host) {
			t.Fatal("expected identity-only SMART rows without a temperature to allow legacy SSH fallback")
		}
	})
}

func TestIsHostAgentTemperatureRecent(t *testing.T) {
	tests := []struct {
		name     string
		lastSeen time.Time
		expected bool
	}{
		{
			name:     "recent - just now",
			lastSeen: time.Now(),
			expected: true,
		},
		{
			name:     "recent - 1 minute ago",
			lastSeen: time.Now().Add(-1 * time.Minute),
			expected: true,
		},
		{
			name:     "stale - 3 minutes ago",
			lastSeen: time.Now().Add(-3 * time.Minute),
			expected: false,
		},
		{
			name:     "stale - 1 hour ago",
			lastSeen: time.Now().Add(-1 * time.Hour),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHostAgentTemperatureRecent(tt.lastSeen)
			if result != tt.expected {
				t.Errorf("isHostAgentTemperatureRecent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMergeTemperatureData_NilInputs(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		result := mergeTemperatureData(nil, nil)
		if result != nil {
			t.Error("expected nil for both nil inputs")
		}
	})

	t.Run("host nil", func(t *testing.T) {
		proxy := &models.Temperature{CPUPackage: 50.0}
		result := mergeTemperatureData(nil, proxy)
		if result != proxy {
			t.Error("expected proxy when host is nil")
		}
	})

	t.Run("proxy nil", func(t *testing.T) {
		host := &models.Temperature{CPUPackage: 55.0}
		result := mergeTemperatureData(host, nil)
		if result != host {
			t.Error("expected host when proxy is nil")
		}
	})
}

func TestMergeTemperatureData_PreservesLegacySensorsFormat(t *testing.T) {
	hostTemp := &models.Temperature{
		CPUPackage: 55.0,
		HasCPU:     true,
		Available:  true,
		LastUpdate: time.Now(),
	}
	proxyTemp := &models.Temperature{
		CPUPackage:          52.0,
		HasCPU:              true,
		Available:           true,
		LegacySensorsFormat: true,
	}

	result := mergeTemperatureData(hostTemp, proxyTemp)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.LegacySensorsFormat {
		t.Error("expected LegacySensorsFormat from proxy to survive the merge")
	}
}

func TestMergeTemperatureData_Merge(t *testing.T) {
	hostTemp := &models.Temperature{
		CPUPackage: 55.0,
		CPUMax:     55.0,
		HasCPU:     true,
		Cores: []models.CoreTemp{
			{Core: 0, Temp: 50.0},
			{Core: 1, Temp: 55.0},
		},
		LastUpdate: time.Now(),
	}

	proxyTemp := &models.Temperature{
		CPUPackage:   52.0,
		CPUMin:       30.0,
		CPUMaxRecord: 60.0,
		HasCPU:       true,
		HasSMART:     true,
		SMART: []models.DiskTemp{
			{Device: "/dev/sda", Temperature: 35},
		},
	}

	result := mergeTemperatureData(hostTemp, proxyTemp)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Host agent CPU takes priority
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage 55.0 from host, got %f", result.CPUPackage)
	}

	// Proxy historical data preserved
	if result.CPUMin != 30.0 {
		t.Errorf("expected CPUMin 30.0 from proxy, got %f", result.CPUMin)
	}

	// SMART data from proxy preserved
	if !result.HasSMART {
		t.Error("expected HasSMART to be true")
	}
	if len(result.SMART) != 1 {
		t.Errorf("expected 1 SMART disk, got %d", len(result.SMART))
	}
}

func TestMergeTemperatureData_FallbackToProxy(t *testing.T) {
	// Host has no CPU data, should fall back to proxy
	hostTemp := &models.Temperature{
		HasGPU: true,
		GPU: []models.GPUTemp{
			{Device: "gpu0", Edge: 70.0},
		},
		LastUpdate: time.Now(),
	}

	proxyTemp := &models.Temperature{
		CPUPackage: 52.0,
		CPUMax:     52.0,
		HasCPU:     true,
		Cores: []models.CoreTemp{
			{Core: 0, Temp: 48.0},
		},
	}

	result := mergeTemperatureData(hostTemp, proxyTemp)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should fall back to proxy CPU data
	if result.CPUPackage != 52.0 {
		t.Errorf("expected CPUPackage 52.0 from proxy fallback, got %f", result.CPUPackage)
	}
	if len(result.Cores) != 1 {
		t.Errorf("expected 1 core from proxy, got %d", len(result.Cores))
	}

	// Host GPU data should be present
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU from host, got %d", len(result.GPU))
	}
}

func TestGetHostAgentTemperature(t *testing.T) {
	m := &Monitor{state: models.NewState()}

	t.Run("no hosts in state", func(t *testing.T) {
		result := m.getHostAgentTemperature("node1")
		assert.Nil(t, result)
	})

	t.Run("match by linked node id", func(t *testing.T) {
		host := models.Host{
			ID:           "host1",
			LinkedNodeID: "node-123",
			Sensors: models.HostSensorSummary{
				TemperatureCelsius: map[string]float64{"cpu_package": 60.0},
			},
			LastSeen: time.Now(),
		}
		m.state.UpsertHost(host)

		result := m.getHostAgentTemperatureForNode(models.Node{ID: "node-123", Name: "different-name"})
		assert.NotNil(t, result)
		assert.Equal(t, 60.0, result.CPUPackage)
	})

	t.Run("match by linked node id with SMART-only sensors", func(t *testing.T) {
		host := models.Host{
			ID:           "host-smart-only",
			LinkedNodeID: "node-smart-only",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sda", Temperature: 36, Health: "PASSED"},
				},
			},
			LastSeen: time.Now(),
		}
		m.state.UpsertHost(host)

		result := m.getHostAgentTemperatureForNode(models.Node{ID: "node-smart-only", Name: "different-name"})
		assert.NotNil(t, result)
		assert.True(t, result.Available)
		assert.True(t, result.HasSMART)
		if assert.Len(t, result.SMART, 1) {
			assert.Equal(t, "/dev/sda", result.SMART[0].Device)
			assert.Equal(t, 36, result.SMART[0].Temperature)
		}
	})

	t.Run("match by hostname fallback", func(t *testing.T) {
		host := models.Host{
			ID:       "host2",
			Hostname: "node2",
			Sensors: models.HostSensorSummary{
				TemperatureCelsius: map[string]float64{"cpu_package": 65.0},
			},
			LastSeen: time.Now(),
		}
		m.state.UpsertHost(host)

		result := m.getHostAgentTemperature("node2")
		assert.NotNil(t, result)
		assert.Equal(t, 65.0, result.CPUPackage)
	})

	t.Run("no matching host", func(t *testing.T) {
		result := m.getHostAgentTemperature("node-missing")
		assert.Nil(t, result)
	})

	t.Run("matching host but no sensor data", func(t *testing.T) {
		host := models.Host{
			ID:       "host3",
			Hostname: "node3",
		}
		m.state.UpsertHost(host)
		result := m.getHostAgentTemperature("node3")
		assert.Nil(t, result)
	})
}

func TestConvertHostSensorsToTemperature_ExtraBranches(t *testing.T) {
	t.Run("SMART disk standby", func(t *testing.T) {
		sensors := models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu_package": 45.0},
			SMART: []models.HostDiskSMART{
				{Device: "/dev/sda [sat]", Temperature: 35, Standby: false},
				{Device: "sdb", Temperature: 0, Standby: true},
			},
		}
		result := convertHostSensorsToTemperature(sensors, time.Now())
		assert.NotNil(t, result)
		assert.Len(t, result.SMART, 1)
		assert.Equal(t, "/dev/sda", result.SMART[0].Device)
	})

	t.Run("SMART-only sensors stay available", func(t *testing.T) {
		sensors := models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "/dev/sdc", Temperature: 37},
			},
		}
		result := convertHostSensorsToTemperature(sensors, time.Now())
		assert.NotNil(t, result)
		assert.True(t, result.Available)
		assert.True(t, result.HasSMART)
		if assert.Len(t, result.SMART, 1) {
			assert.Equal(t, "/dev/sdc", result.SMART[0].Device)
			assert.Equal(t, 37, result.SMART[0].Temperature)
		}
	})

	t.Run("GPU merge into same device", func(t *testing.T) {
		sensors := models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{
				"gpu_edge":     60.0,
				"gpu_junction": 65.0,
			},
		}
		result := convertHostSensorsToTemperature(sensors, time.Now())
		assert.NotNil(t, result)
		assert.Len(t, result.GPU, 1)
		assert.Equal(t, "gpu0", result.GPU[0].Device)
		assert.Equal(t, 60.0, result.GPU[0].Edge)
		assert.Equal(t, 65.0, result.GPU[0].Junction)
	})
}

// readStateMonitor returns a monitor whose unified read state is the registry
// projection of nodes and hosts, as the poller sees it. Nodes are linked back to
// the hosts linked to them, as the agent linker leaves them.
func readStateMonitor(nodes []models.Node, hosts []models.Host) *Monitor {
	for i := range nodes {
		for _, host := range hosts {
			if host.LinkedNodeID == nodes[i].ID {
				nodes[i].LinkedAgentID = host.ID
			}
		}
	}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: nodes, Hosts: hosts})
	return &Monitor{
		state:               models.NewState(),
		resourceStore:       adapter,
		clusterSensorsCache: make(map[string]clusterSensorsCacheEntry),
	}
}

func readStateTestNode(instance, name string) models.Node {
	return models.Node{ID: instance + "-" + name, Instance: instance, Name: name, Status: "online", LastSeen: time.Now()}
}

func reportingTestHost(id, hostname, linkedNodeID string, cpuPackage float64) models.Host {
	host := models.Host{ID: id, Hostname: hostname, Status: "online", LinkedNodeID: linkedNodeID, IntervalSeconds: 30, LastSeen: time.Now()}
	if cpuPackage > 0 {
		host.Sensors = models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": cpuPackage}}
	}
	return host
}

func clusterSensorReport(nodeName string, cpuPackage float64) []agentshost.ClusterNodeSensors {
	return []agentshost.ClusterNodeSensors{{
		NodeName: nodeName,
		Sensors:  agentshost.Sensors{TemperatureCelsius: map[string]float64{"cpu_package": cpuPackage, "cpu_core_0": cpuPackage - 3}},
	}}
}

func TestGetClusterSensorTemperature(t *testing.T) {
	m := readStateMonitor(
		[]models.Node{readStateTestNode("pve1", "node1"), readStateTestNode("pve1", "node2"), readStateTestNode("pve1", "MyNode"), readStateTestNode("pve1", "stale-node")},
		[]models.Host{reportingTestHost("agent-node1", "node1", "pve1-node1", 0), reportingTestHost("agent-unlinked", "elsewhere", "", 0)},
	)
	lookup := func(nodeID, nodeName string) *models.Temperature {
		readState := m.GetUnifiedReadStateOrSnapshot()
		polled := models.Node{ID: nodeID, Name: nodeName}
		slot := m.polledNodeSlot(readState.Hosts(), readState.Nodes(), polled)
		return m.getClusterSensorTemperature(readState.Hosts(), readState.Nodes(), polled, slot)
	}

	t.Run("empty cache returns nil", func(t *testing.T) {
		assert.Nil(t, lookup("pve1-node2", "node2"))
	})

	t.Run("cached data returned", func(t *testing.T) {
		m.applyClusterSensors("agent-node1", clusterSensorReport("node2", 58), time.Now())

		result := lookup("pve1-node2", "node2")
		if assert.NotNil(t, result) {
			assert.Equal(t, 58.0, result.CPUPackage)
			assert.True(t, result.HasCPU)
		}
	})

	t.Run("stale data returns nil", func(t *testing.T) {
		m.applyClusterSensors("agent-node1", clusterSensorReport("stale-node", 50), time.Now().Add(-3*time.Minute)) // older than 2min threshold

		assert.Nil(t, lookup("pve1-stale-node", "stale-node"))
	})

	t.Run("case insensitive lookup", func(t *testing.T) {
		m.applyClusterSensors("agent-node1", clusterSensorReport("mynode", 60), time.Now())

		result := lookup("pve1-MyNode", "MyNode")
		if assert.NotNil(t, result) {
			assert.Equal(t, 60.0, result.CPUPackage)
		}
	})

	t.Run("reading from an unlinked agent serves no node", func(t *testing.T) {
		m.applyClusterSensors("agent-unlinked", clusterSensorReport("node1", 40), time.Now())

		assert.Nil(t, lookup("pve1-node1", "node1"))
	})

	t.Run("node of another connection returns nil", func(t *testing.T) {
		assert.Nil(t, lookup("", "node2"))
		assert.Nil(t, lookup("pve9-node2", "node2"))
	})

	t.Run("node not yet in the read state is scoped by its own connection", func(t *testing.T) {
		m.applyClusterSensors("agent-node1", clusterSensorReport("node9", 47), time.Now())
		readState := m.GetUnifiedReadStateOrSnapshot()
		polled := readStateTestNode("pve1", "node9")
		result := m.getClusterSensorTemperature(readState.Hosts(), readState.Nodes(), polled, m.polledNodeSlot(readState.Hosts(), readState.Nodes(), polled))
		if assert.NotNil(t, result) {
			assert.Equal(t, 47.0, result.CPUPackage)
		}
	})

	t.Run("empty node name returns nil", func(t *testing.T) {
		assert.Nil(t, lookup("pve1-node2", ""))
	})
}

func TestGetHostAgentTemperatureByID_ClusterFallback(t *testing.T) {
	m := readStateMonitor(
		[]models.Node{readStateTestNode("pve1", "sibling"), readStateTestNode("pve1", "orphan-node")},
		[]models.Host{reportingTestHost("agent-sibling", "sibling", "pve1-sibling", 0)},
	)

	// No host agent on the node, but a sibling's agent collected its sensors
	m.applyClusterSensors("agent-sibling", clusterSensorReport("orphan-node", 62), time.Now())

	result := m.getHostAgentTemperatureForNode(models.Node{ID: "pve1-orphan-node", Name: "orphan-node"})
	if assert.NotNil(t, result, "should fall back to cluster sensor cache") {
		assert.Equal(t, 62.0, result.CPUPackage)
	}
}

func TestGetHostAgentTemperatureByID_LocalAgentTakesPriority(t *testing.T) {
	m := readStateMonitor(
		[]models.Node{readStateTestNode("pve1", "sibling"), readStateTestNode("pve1", "shared-node")},
		[]models.Host{
			reportingTestHost("agent-sibling", "sibling", "pve1-sibling", 0),
			reportingTestHost("host-local", "shared-node", "pve1-shared-node", 70), // local agent reports 70
		},
	)
	// Both the local agent and the cluster cache have data for the same node
	m.applyClusterSensors("agent-sibling", clusterSensorReport("shared-node", 55), time.Now()) // cluster cache says 55

	result := m.getHostAgentTemperatureForNode(models.Node{ID: "pve1-shared-node", Name: "shared-node"})
	if assert.NotNil(t, result) {
		assert.Equal(t, 70.0, result.CPUPackage, "local agent data should take priority over cluster cache")
	}
}

// An agent that stops reporting keeps its last sensors in state. Past the
// reporting lease that marks it offline, those sensors must not keep feeding the
// linked node as a live reading; the cluster cache (with its own recency) is
// still consulted.
func TestGetHostAgentTemperatureByID_IgnoresAgentPastReportingLease(t *testing.T) {
	nodes := []models.Node{readStateTestNode("pve1", "silent-node"), readStateTestNode("pve1", "sibling")}
	sibling := reportingTestHost("agent-sibling", "sibling", "pve1-sibling", 0)
	silent := reportingTestHost("host-silent", "silent-node", "pve1-silent-node", 95)
	silent.LastSeen = time.Now().Add(-hostAgentHealthWindow(30) - time.Minute)
	m := readStateMonitor(nodes, []models.Host{silent, sibling})

	assert.Nil(t, m.getHostAgentTemperatureForNode(models.Node{ID: "pve1-silent-node", Name: "silent-node"}),
		"a silent agent's retained sensors must not be presented as a current reading")

	m.applyClusterSensors("agent-sibling", clusterSensorReport("silent-node", 58), time.Now())
	if result := m.getHostAgentTemperatureForNode(models.Node{ID: "pve1-silent-node", Name: "silent-node"}); assert.NotNil(t, result) {
		assert.Equal(t, 58.0, result.CPUPackage, "a recent cluster-cache reading still serves the node")
	}

	reporting := reportingTestHost("host-silent", "silent-node", "pve1-silent-node", 71)
	m.resourceStore.(*unifiedresources.MonitorAdapter).PopulateFromSnapshot(models.StateSnapshot{Nodes: nodes, Hosts: []models.Host{reporting, sibling}})
	if result := m.getHostAgentTemperatureForNode(models.Node{ID: "pve1-silent-node", Name: "silent-node"}); assert.NotNil(t, result) {
		assert.Equal(t, 71.0, result.CPUPackage, "a reporting agent's reading is used again")
	}
}

// When every temperature source returns nothing, a previous reading may be
// carried only inside the carry window and keeps its original timestamp; an
// older one is dropped rather than re-presented as current.
func TestCollectNodeTemperatureCarryIsBoundedByCarryWindow(t *testing.T) {
	m := &Monitor{
		config: &config.Config{TemperatureMonitoringEnabled: true, PVEPollingInterval: 10 * time.Second},
		state:  models.NewState(),
	}
	collectWithPrevious := func(prev *models.Temperature) *models.Temperature {
		node := models.Node{ID: "pve1-node1", Name: "node1", Instance: "pve1"}
		m.collectNodeTemperatureData(
			context.Background(), "pve1", &config.PVEInstance{Name: "pve1"}, proxmox.Node{Node: "node1"},
			&node, []models.Node{{ID: node.ID, Temperature: prev}}, "online",
		)
		return node.Temperature
	}

	assert.Equal(t, 5*time.Minute, m.nodeTemperatureCarryWindow(), "the floor applies at the default polling interval")
	m.config.PVEPollingInterval = 10 * time.Minute
	assert.Equal(t, 20*time.Minute, m.nodeTemperatureCarryWindow(), "the window scales with a slow polling interval")
	m.config.PVEPollingInterval = 10 * time.Second

	recent := time.Now().Add(-time.Minute)
	if carried := collectWithPrevious(&models.Temperature{Available: true, CPUPackage: 92, LastUpdate: recent}); assert.NotNil(t, carried) {
		assert.True(t, carried.Available)
		assert.True(t, carried.LastUpdate.Equal(recent), "a carried reading keeps its original timestamp")
	}

	stale := time.Now().Add(-m.nodeTemperatureCarryWindow() - time.Minute)
	assert.Nil(t, collectWithPrevious(&models.Temperature{Available: true, CPUPackage: 92, LastUpdate: stale}),
		"a reading older than the carry window must not be presented as current")
	assert.Nil(t, collectWithPrevious(&models.Temperature{Available: true, CPUPackage: 92}),
		"a reading with no timestamp cannot be shown to be recent")
}

func TestHostAgentReportCurrent(t *testing.T) {
	now := time.Now()
	assert.False(t, hostAgentReportCurrent(time.Time{}, 30, now), "an agent that never reported has no current reading")
	assert.True(t, hostAgentReportCurrent(now.Add(-hostAgentHealthWindow(30)), 30, now))
	assert.False(t, hostAgentReportCurrent(now.Add(-hostAgentHealthWindow(30)-time.Second), 30, now))
	// A slow agent's lease scales with its declared interval.
	assert.True(t, hostAgentReportCurrent(now.Add(-8*time.Minute), 120, now))
}

func TestGetHostAgentTemperatureByID_UsesUnifiedReadState(t *testing.T) {
	now := time.Now().UTC()
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:           "host-readstate",
				Hostname:     "readstate-node",
				LinkedNodeID: "node-readstate",
				LastSeen:     now,
				Sensors: models.HostSensorSummary{
					TemperatureCelsius: map[string]float64{
						"cpu_package": 71.0,
					},
					SMART: []models.HostDiskSMART{
						{Device: "sda", Temperature: 35, Standby: true},
						{Device: "sdb", Temperature: 37},
					},
				},
			},
		},
	})

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: unifiedresources.NewMonitorAdapter(registry),
	}

	result := m.getHostAgentTemperatureForNode(models.Node{ID: "node-readstate", Name: "ignored"})
	assert.NotNil(t, result)
	assert.Equal(t, 71.0, result.CPUPackage)
	if assert.Len(t, result.SMART, 1) {
		assert.Equal(t, "/dev/sdb", result.SMART[0].Device)
	}
}

func TestMergeTemperatureData_HistoricalOverrides(t *testing.T) {
	t.Run("historical max update", func(t *testing.T) {
		host := &models.Temperature{CPUPackage: 70.0, HasCPU: true, Available: true}
		proxy := &models.Temperature{CPUPackage: 50.0, CPUMaxRecord: 60.0, HasCPU: true}

		result := mergeTemperatureData(host, proxy)
		assert.Equal(t, 70.0, result.CPUMaxRecord)
	})

	t.Run("fallback to proxy GPU and NVMe", func(t *testing.T) {
		host := &models.Temperature{CPUPackage: 50.0, HasCPU: true}
		proxy := &models.Temperature{
			HasGPU:  true,
			GPU:     []models.GPUTemp{{Device: "gpu0", Edge: 55.0}},
			HasNVMe: true,
			NVMe:    []models.NVMeTemp{{Device: "nvme0", Temp: 40.0}},
		}

		result := mergeTemperatureData(host, proxy)
		assert.True(t, result.HasGPU)
		assert.True(t, result.HasNVMe)
		assert.Len(t, result.GPU, 1)
		assert.Len(t, result.NVMe, 1)
	})

	t.Run("proxy SMART temperatures replace host inventory rows", func(t *testing.T) {
		host := &models.Temperature{
			CPUPackage: 50.0,
			HasCPU:     true,
			HasSMART:   true,
			SMART:      []models.DiskTemp{{Device: "/dev/sda", Temperature: 0}},
		}
		proxy := &models.Temperature{
			HasSMART: true,
			SMART:    []models.DiskTemp{{Device: "/dev/sda", Temperature: 39}},
		}

		result := mergeTemperatureData(host, proxy)
		assert.True(t, result.HasSMART)
		if assert.Len(t, result.SMART, 1) {
			assert.Equal(t, 39, result.SMART[0].Temperature)
		}
	})
}
