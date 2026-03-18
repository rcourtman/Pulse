package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// --- mergeHostAgentSMARTIntoDisks ---

func TestMergeHostAgentSMARTIntoDisks_EmptyInputs(t *testing.T) {
	// All combinations of empty inputs should return disks unchanged
	disks := []models.PhysicalDisk{{ID: "d1"}}
	if got := mergeHostAgentSMARTIntoDisks(nil, nil, nil); got != nil {
		t.Error("nil disks should return nil")
	}
	if got := mergeHostAgentSMARTIntoDisks(disks, nil, nil); len(got) != 1 {
		t.Error("nil nodes/hosts should return disks unchanged")
	}
	if got := mergeHostAgentSMARTIntoDisks(disks, []models.Node{}, nil); len(got) != 1 {
		t.Error("empty nodes should return disks unchanged")
	}
	if got := mergeHostAgentSMARTIntoDisks(disks, []models.Node{{Name: "n1"}}, nil); len(got) != 1 {
		t.Error("nil hosts should return disks unchanged")
	}
}

func TestMergeHostAgentSMARTIntoDisks_MatchByWWN(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", DevPath: "/dev/sda", WWN: "0x5001234", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sda", WWN: "0x5001234", Temperature: 42},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 42 {
		t.Errorf("expected temp 42 via WWN match, got %d", result[0].Temperature)
	}
}

func TestMergeHostAgentSMARTIntoDisks_MatchBySerial(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SERIAL123", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sdb", Serial: "serial123", Temperature: 38},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 38 {
		t.Errorf("expected temp 38 via serial match (case-insensitive), got %d", result[0].Temperature)
	}
}

func TestMergeHostAgentSMARTIntoDisks_MatchByDevPath(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", DevPath: "/dev/nvme0n1", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "nvme0n1", Temperature: 35},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 35 {
		t.Errorf("expected temp 35 via device path match (strip /dev/), got %d", result[0].Temperature)
	}
}

func TestMergeHostAgentSMARTIntoDisks_SkipsStandby(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sda", Serial: "SER1", Temperature: 40, Standby: true},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 0 {
		t.Error("standby disk should not have its temperature merged")
	}
}

func TestMergeHostAgentSMARTIntoDisks_MergesSMARTAttributes(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	attrs := &models.SMARTAttributes{}
	poh := int64(5000)
	attrs.PowerOnHours = &poh
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sda", Serial: "SER1", Temperature: 41, Attributes: attrs},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].SmartAttributes == nil {
		t.Fatal("SMART attributes should be merged")
	}
	if *result[0].SmartAttributes.PowerOnHours != 5000 {
		t.Error("PowerOnHours should be 5000")
	}
}

func TestMergeHostAgentSMARTIntoDisks_NoAgentLink(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Temperature: 0},
	}
	nodes := []models.Node{
		{Name: "pve1"}, // No LinkedAgentID
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Serial: "SER1", Temperature: 40},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 0 {
		t.Error("unlinked nodes should not get host agent temperatures")
	}
}

func TestMergeHostAgentSMARTIntoDisks_PreservesExistingTemp(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Temperature: 50}, // Already has temp
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Serial: "SER1", Temperature: 40},
				},
			},
		},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 50 {
		t.Errorf("existing temperature should be preserved, got %d", result[0].Temperature)
	}
}

func TestMergeHostAgentSMARTIntoDisks_OriginalUnchanged(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Temperature: 0},
	}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	hosts := []models.Host{
		{ID: "host-1", Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{Serial: "SER1", Temperature: 42}},
		}},
	}
	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if disks[0].Temperature != 0 {
		t.Error("original disk slice should not be mutated")
	}
	if result[0].Temperature != 42 {
		t.Error("returned slice should have the merged temp")
	}
}

// --- smartAttributesFromUnifiedMeta ---

func TestSmartAttributesFromUnifiedMeta_Nil(t *testing.T) {
	if smartAttributesFromUnifiedMeta(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestSmartAttributesFromUnifiedMeta_AllZero(t *testing.T) {
	// All-zero fields should return nil (no meaningful data)
	if smartAttributesFromUnifiedMeta(&unifiedresources.SMARTMeta{}) != nil {
		t.Error("all-zero SMARTMeta should return nil")
	}
}

func TestSmartAttributesFromUnifiedMeta_PopulatesFields(t *testing.T) {
	meta := &unifiedresources.SMARTMeta{
		PowerOnHours:         12345,
		PowerCycles:          500,
		ReallocatedSectors:   0,
		PendingSectors:       2,
		OfflineUncorrectable: 1,
		UDMACRCErrors:        3,
		PercentageUsed:       15,
		AvailableSpare:       85,
		MediaErrors:          0,
		UnsafeShutdowns:      10,
	}
	result := smartAttributesFromUnifiedMeta(meta)
	if result == nil {
		t.Fatal("non-zero meta should produce non-nil result")
	}
	if result.PowerOnHours == nil || *result.PowerOnHours != 12345 {
		t.Error("PowerOnHours should be 12345")
	}
	if result.PowerCycles == nil || *result.PowerCycles != 500 {
		t.Error("PowerCycles should be 500")
	}
	if result.ReallocatedSectors != nil {
		t.Error("zero ReallocatedSectors should not create a pointer")
	}
	if result.PendingSectors == nil || *result.PendingSectors != 2 {
		t.Error("PendingSectors should be 2")
	}
	if result.PercentageUsed == nil || *result.PercentageUsed != 15 {
		t.Error("PercentageUsed should be 15")
	}
	if result.MediaErrors != nil {
		t.Error("zero MediaErrors should not create a pointer")
	}
	if result.UnsafeShutdowns == nil || *result.UnsafeShutdowns != 10 {
		t.Error("UnsafeShutdowns should be 10")
	}
}

// --- getNodeDisplayName ---

func TestGetNodeDisplayName_NilInstance(t *testing.T) {
	got := getNodeDisplayName(nil, "pve1")
	if got != "pve1" {
		t.Errorf("expected 'pve1', got %q", got)
	}
}

func TestGetNodeDisplayName_EmptyNodeName(t *testing.T) {
	got := getNodeDisplayName(nil, "")
	if got != "unknown-node" {
		t.Errorf("expected 'unknown-node', got %q", got)
	}
}

func TestGetNodeDisplayName_FriendlyName(t *testing.T) {
	inst := &config.PVEInstance{Name: "My Server"}
	got := getNodeDisplayName(inst, "pve1")
	if got != "My Server" {
		t.Errorf("expected friendly name 'My Server', got %q", got)
	}
}

func TestGetNodeDisplayName_ClusterUsesNodeName(t *testing.T) {
	inst := &config.PVEInstance{Name: "cluster1", IsCluster: true}
	got := getNodeDisplayName(inst, "node1")
	if got != "node1" {
		t.Errorf("cluster mode should use node name, got %q", got)
	}
}

func TestGetNodeDisplayName_ClusterWithEndpointLabel(t *testing.T) {
	inst := &config.PVEInstance{
		Name:      "cluster1",
		IsCluster: true,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "node1", NodeID: "node/node1"},
		},
	}
	got := getNodeDisplayName(inst, "node1")
	// With matching endpoint, should still return node1 (or the endpoint label)
	if got == "" || got == "unknown-node" {
		t.Errorf("expected a valid display name for cluster node, got %q", got)
	}
}
