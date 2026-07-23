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

func TestMergeHostAgentSMARTIntoDisks_DerivesWearoutFromSMARTAttributes(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Wearout: -1},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	used := 6
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{
						Device: "/dev/sda [megaraid,7]",
						Serial: "SER1",
						Health: "PASSED",
						Attributes: &models.SMARTAttributes{
							PercentageUsed: &used,
						},
					},
				},
			},
		},
	}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Wearout != 94 {
		t.Fatalf("expected wearout derived from PercentageUsed, got %d", result[0].Wearout)
	}
	if result[0].SmartAttributes == nil || result[0].SmartAttributes.PercentageUsed == nil || *result[0].SmartAttributes.PercentageUsed != 6 {
		t.Fatalf("expected merged PercentageUsed, got %+v", result[0].SmartAttributes)
	}
}

func TestMergeHostAgentSMARTIntoDisks_SMARTFailureAndWearoutOverrideCoarsePVEValues(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:      "d1",
		Node:    "pve1",
		Serial:  "SER1",
		Type:    "ssd",
		Health:  "PASSED",
		Wearout: 100,
	}}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	used := 95
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device: "/dev/sda",
				Serial: "SER1",
				Health: "FAILED",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: &used,
				},
			}},
		},
	}}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Health != "FAILED" {
		t.Fatalf("SMART failure was suppressed by PVE health: %+v", result[0])
	}
	if result[0].Wearout != 5 {
		t.Fatalf("SMART wearout did not override contradictory PVE value: %+v", result[0])
	}
}

func TestMergeHostAgentSMARTIntoDisks_DoesNotReplaceFailureWithPassed(t *testing.T) {
	disks := []models.PhysicalDisk{{ID: "d1", Node: "pve1", Serial: "SER1", Health: "FAILED"}}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{Serial: "SER1", Health: "PASSED"}},
		},
	}}

	if got := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)[0].Health; got != "FAILED" {
		t.Fatalf("PASSED replaced an existing failure: %q", got)
	}
}

func TestMergeHostAgentSMARTIntoDisks_DuplicateIdentityRequiresUniqueTopology(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:         "d1",
		Node:       "pve1",
		DevPath:    "/dev/sg2",
		Serial:     "DUPLICATE-SERIAL",
		Controller: "sssraid0",
		Target:     "sssraid,0,2",
		Health:     "UNKNOWN",
	}}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{
					Device:      "/dev/sg2",
					Serial:      "DUPLICATE-SERIAL",
					Controller:  "sssraid0",
					Target:      "sssraid,0,1",
					Temperature: 31,
				},
				{
					Device:      "/dev/sg2",
					Serial:      "DUPLICATE-SERIAL",
					Controller:  "sssraid0",
					Target:      "sssraid,0,2",
					Temperature: 47,
					Health:      "PASSED",
				},
			},
		},
	}}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 47 || result[0].Health != "PASSED" {
		t.Fatalf("topology-specific SMART evidence was not selected: %+v", result[0])
	}

	disks[0].Controller = ""
	disks[0].Target = ""
	disks[0].DevPath = "/dev/unknown"
	result = mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Temperature != 0 || result[0].Health != "UNKNOWN" {
		t.Fatalf("ambiguous SMART identity was guessed: %+v", result[0])
	}
}

func TestMergeHostAgentSMARTIntoDisks_FillsMissingHealthFromHostSMART(t *testing.T) {
	disks := []models.PhysicalDisk{
		{ID: "d1", Node: "pve1", Serial: "SER1", Health: "UNKNOWN"},
	}
	nodes := []models.Node{
		{Name: "pve1", LinkedAgentID: "host-1"},
	}
	hosts := []models.Host{
		{
			ID: "host-1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{Device: "/dev/sda", Serial: "SER1", Health: "FAILED"},
				},
			},
		},
	}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Health != "FAILED" {
		t.Fatalf("expected host SMART health to fill unknown disk health, got %q", result[0].Health)
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
	// No reported fields should return nil.
	if smartAttributesFromUnifiedMeta(&unifiedresources.SMARTMeta{}) != nil {
		t.Error("SMARTMeta with no reported fields should return nil")
	}
}

func TestSmartAttributesFromUnifiedMeta_PopulatesFields(t *testing.T) {
	meta := &unifiedresources.SMARTMeta{
		PowerOnHours:         smartInt64Pointer(12345),
		PowerCycles:          smartInt64Pointer(500),
		ReallocatedSectors:   smartInt64Pointer(0),
		PendingSectors:       smartInt64Pointer(2),
		OfflineUncorrectable: smartInt64Pointer(1),
		UDMACRCErrors:        smartInt64Pointer(3),
		PercentageUsed:       smartIntPointer(15),
		AvailableSpare:       smartIntPointer(85),
		MediaErrors:          smartInt64Pointer(0),
		UnsafeShutdowns:      smartInt64Pointer(10),
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
	if result.ReallocatedSectors == nil || *result.ReallocatedSectors != 0 {
		t.Error("reported zero ReallocatedSectors should survive projection")
	}
	if result.PendingSectors == nil || *result.PendingSectors != 2 {
		t.Error("PendingSectors should be 2")
	}
	if result.PercentageUsed == nil || *result.PercentageUsed != 15 {
		t.Error("PercentageUsed should be 15")
	}
	if result.MediaErrors == nil || *result.MediaErrors != 0 {
		t.Error("reported zero MediaErrors should survive projection")
	}
	if result.UnsafeShutdowns == nil || *result.UnsafeShutdowns != 10 {
		t.Error("UnsafeShutdowns should be 10")
	}
}

func smartIntPointer(value int) *int       { return &value }
func smartInt64Pointer(value int64) *int64 { return &value }

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
	if got != "cluster1 (node1)" {
		t.Errorf("cluster mode should include configured cluster label, got %q", got)
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
	if got != "cluster1 (node1)" {
		t.Errorf("expected cluster-qualified node display name, got %q", got)
	}
}
