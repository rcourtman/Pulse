package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- cloneResource: top-level isolation ---

func TestCloneResource_Nil(t *testing.T) {
	out := cloneResource(nil)
	if out.ID != "" {
		t.Error("clone of nil should produce zero Resource")
	}
}

func TestCloneResourcePtr_Nil(t *testing.T) {
	if cloneResourcePtr(nil) != nil {
		t.Error("cloneResourcePtr(nil) should return nil")
	}
}

func TestCloneResource_MutateOriginalSlice(t *testing.T) {
	original := &Resource{
		ID:      "r-1",
		Name:    "vm-100",
		Tags:    []string{"prod", "web"},
		Sources: []DataSource{SourceProxmox, SourceAgent},
	}
	cloned := cloneResource(original)

	// Mutate the cloned tags.
	cloned.Tags[0] = "MUTATED"
	if original.Tags[0] == "MUTATED" {
		t.Error("mutating cloned Tags should not affect original")
	}

	// Mutate cloned sources.
	cloned.Sources[0] = SourceDocker
	if original.Sources[0] == SourceDocker {
		t.Error("mutating cloned Sources should not affect original")
	}
}

func TestCloneResource_MutateOriginalIdentity(t *testing.T) {
	original := &Resource{
		ID: "r-1",
		Identity: ResourceIdentity{
			Hostnames:    []string{"host1", "host2"},
			IPAddresses:  []string{"10.0.0.1"},
			MACAddresses: []string{"aa:bb:cc:dd:ee:ff"},
		},
	}
	cloned := cloneResource(original)

	cloned.Identity.Hostnames[0] = "MUTATED"
	if original.Identity.Hostnames[0] == "MUTATED" {
		t.Error("mutating cloned Identity.Hostnames should not affect original")
	}

	cloned.Identity.IPAddresses[0] = "MUTATED"
	if original.Identity.IPAddresses[0] == "MUTATED" {
		t.Error("mutating cloned Identity.IPAddresses should not affect original")
	}
}

func TestCloneResource_MutateOriginalMetrics(t *testing.T) {
	used := int64(500)
	total := int64(1000)
	original := &Resource{
		ID: "r-1",
		Metrics: &ResourceMetrics{
			CPU: &MetricValue{Value: 42, Used: &used, Total: &total},
		},
	}
	cloned := cloneResource(original)

	// Mutate the pointer values on the clone.
	*cloned.Metrics.CPU.Used = 999
	if *original.Metrics.CPU.Used != 500 {
		t.Error("mutating cloned Metrics.CPU.Used should not affect original")
	}

	cloned.Metrics.CPU.Value = 99
	if original.Metrics.CPU.Value == 99 {
		t.Error("mutating cloned Metrics.CPU.Value should not affect original")
	}
}

func TestCloneResource_MutateIncidents(t *testing.T) {
	original := &Resource{
		ID: "r-1",
		Incidents: []ResourceIncident{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "Degraded"},
		},
	}
	cloned := cloneResource(original)

	cloned.Incidents[0].Summary = "MUTATED"
	if original.Incidents[0].Summary == "MUTATED" {
		t.Error("mutating cloned Incidents should not affect original")
	}
}

func TestCloneResource_MutateSourceStatusMap(t *testing.T) {
	original := &Resource{
		ID: "r-1",
		SourceStatus: map[DataSource]SourceStatus{
			SourceProxmox: {Status: "online"},
		},
	}
	cloned := cloneResource(original)

	cloned.SourceStatus[SourceDocker] = SourceStatus{Status: "online"}
	if _, exists := original.SourceStatus[SourceDocker]; exists {
		t.Error("adding to cloned SourceStatus should not affect original")
	}
}

func TestCloneResource_MutateParentID(t *testing.T) {
	parentID := "parent-1"
	original := &Resource{
		ID:       "r-1",
		ParentID: &parentID,
	}
	cloned := cloneResource(original)

	*cloned.ParentID = "MUTATED"
	if *original.ParentID == "MUTATED" {
		t.Error("mutating cloned ParentID should not affect original")
	}
}

func TestCloneResource_MutateParentBySource(t *testing.T) {
	original := &Resource{
		ID: "r-1",
		parentBySource: map[DataSource]string{
			SourceProxmox: "parent-pve",
		},
	}
	cloned := cloneResource(original)

	cloned.parentBySource[SourceAgent] = "parent-agent"
	if _, exists := original.parentBySource[SourceAgent]; exists {
		t.Error("mutating cloned parentBySource should not affect original")
	}
}

func TestCloneResource_MutateVMwareDetailSlices(t *testing.T) {
	createdAt := time.Date(2026, time.March, 30, 18, 15, 0, 0, time.UTC)
	pciSlot := int64(160)
	scsiBus := int64(0)
	scsiUnit := int64(1)
	diskCapacity := int64(107374182400)
	autoUpdateSupported := true
	installAttempts := int64(1)
	toolsVersionNumber := int64(12352)
	guestRebootRequested := true
	original := &Resource{
		ID: "vmware-vm-1",
		VMware: &VMwareData{
			SnapshotTree: []VMwareSnapshotData{{
				Snapshot:  "snapshot-201",
				Name:      "pre-upgrade",
				CreatedAt: &createdAt,
				Children: []VMwareSnapshotData{{
					Snapshot: "snapshot-202",
					Name:     "post-upgrade",
				}},
			}},
			NetworkAdapters: []VMwareNetworkAdapterData{{
				NIC:           "4000",
				Label:         "Network adapter 1",
				NetworkName:   "VM Network",
				MACAddress:    "00:50:56:aa:bb:cc",
				PCISlotNumber: &pciSlot,
			}},
			VirtualDisks: []VMwareVirtualDiskData{{
				Disk:          "2000",
				Label:         "Hard disk 1",
				Type:          "SCSI",
				SCSIBus:       &scsiBus,
				SCSIUnit:      &scsiUnit,
				BackingType:   "VMDK_FILE",
				VMDKFile:      "[nvme-primary] app-01/app-01.vmdk",
				DatastoreName: "nvme-primary",
				CapacityBytes: &diskCapacity,
			}},
			Tools: &VMwareToolsData{
				AutoUpdateSupported:   &autoUpdateSupported,
				InstallAttemptCount:   &installAttempts,
				VersionNumber:         &toolsVersionNumber,
				Version:               "12.4.0",
				VersionStatus:         "CURRENT",
				RunState:              "RUNNING",
				GuestRebootRequested:  &guestRebootRequested,
				GuestRebootComponents: []string{"tools"},
			},
		},
	}

	cloned := cloneResource(original)
	cloned.VMware.SnapshotTree[0].Name = "mutated"
	cloned.VMware.SnapshotTree[0].Children[0].Name = "mutated-child"
	*cloned.VMware.SnapshotTree[0].CreatedAt = createdAt.Add(time.Hour)
	cloned.VMware.NetworkAdapters[0].NetworkName = "mutated-network"
	*cloned.VMware.NetworkAdapters[0].PCISlotNumber = 161
	cloned.VMware.VirtualDisks[0].DatastoreName = "mutated-datastore"
	*cloned.VMware.VirtualDisks[0].SCSIUnit = 2
	*cloned.VMware.VirtualDisks[0].CapacityBytes = 1
	*cloned.VMware.Tools.AutoUpdateSupported = false
	*cloned.VMware.Tools.VersionNumber = 1
	cloned.VMware.Tools.GuestRebootComponents[0] = "mutated"

	if original.VMware.SnapshotTree[0].Name != "pre-upgrade" {
		t.Fatalf("mutating cloned VMware snapshot should not affect original: %+v", original.VMware.SnapshotTree)
	}
	if original.VMware.SnapshotTree[0].Children[0].Name != "post-upgrade" {
		t.Fatalf("mutating cloned VMware child snapshot should not affect original: %+v", original.VMware.SnapshotTree)
	}
	if !original.VMware.SnapshotTree[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("mutating cloned VMware snapshot time should not affect original: %+v", original.VMware.SnapshotTree[0].CreatedAt)
	}
	if original.VMware.NetworkAdapters[0].NetworkName != "VM Network" {
		t.Fatalf("mutating cloned VMware adapter should not affect original: %+v", original.VMware.NetworkAdapters)
	}
	if *original.VMware.NetworkAdapters[0].PCISlotNumber != 160 {
		t.Fatalf("mutating cloned VMware adapter PCI slot should not affect original: %+v", original.VMware.NetworkAdapters[0].PCISlotNumber)
	}
	if original.VMware.VirtualDisks[0].DatastoreName != "nvme-primary" {
		t.Fatalf("mutating cloned VMware disk should not affect original: %+v", original.VMware.VirtualDisks)
	}
	if *original.VMware.VirtualDisks[0].SCSIUnit != 1 {
		t.Fatalf("mutating cloned VMware disk unit should not affect original: %+v", original.VMware.VirtualDisks[0].SCSIUnit)
	}
	if *original.VMware.VirtualDisks[0].CapacityBytes != 107374182400 {
		t.Fatalf("mutating cloned VMware disk capacity should not affect original: %+v", original.VMware.VirtualDisks[0].CapacityBytes)
	}
	if original.VMware.Tools.AutoUpdateSupported == nil || !*original.VMware.Tools.AutoUpdateSupported {
		t.Fatalf("mutating cloned VMware Tools auto-update flag should not affect original: %+v", original.VMware.Tools.AutoUpdateSupported)
	}
	if original.VMware.Tools.VersionNumber == nil || *original.VMware.Tools.VersionNumber != 12352 {
		t.Fatalf("mutating cloned VMware Tools version number should not affect original: %+v", original.VMware.Tools.VersionNumber)
	}
	if original.VMware.Tools.GuestRebootComponents[0] != "tools" {
		t.Fatalf("mutating cloned VMware Tools reboot components should not affect original: %+v", original.VMware.Tools.GuestRebootComponents)
	}
}

// --- cloneProxmoxData ---

func TestCloneProxmoxData_Nil(t *testing.T) {
	if cloneProxmoxData(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneProxmoxData_TemperatureIsolation(t *testing.T) {
	temp := 45.5
	original := &ProxmoxData{
		NodeName:    "node1",
		Temperature: &temp,
		LoadAverage: []float64{1.0, 2.0, 3.0},
	}
	cloned := cloneProxmoxData(original)

	*cloned.Temperature = 99.9
	if *original.Temperature != 45.5 {
		t.Error("mutating cloned Temperature should not affect original")
	}

	cloned.LoadAverage[0] = 99.0
	if original.LoadAverage[0] != 1.0 {
		t.Error("mutating cloned LoadAverage should not affect original")
	}
}

func TestCloneProxmoxData_NetworkInterfaceIsolation(t *testing.T) {
	original := &ProxmoxData{
		NetworkInterfaces: []NetworkInterface{
			{Name: "eth0", Addresses: []string{"10.0.0.1", "10.0.0.2"}},
		},
	}
	cloned := cloneProxmoxData(original)

	cloned.NetworkInterfaces[0].Addresses[0] = "MUTATED"
	if original.NetworkInterfaces[0].Addresses[0] == "MUTATED" {
		t.Error("mutating cloned network interface addresses should not affect original")
	}
}

// --- cloneStorageMeta ---

func TestCloneStorageMeta_Nil(t *testing.T) {
	if cloneStorageMeta(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStorageMeta_SliceIsolation(t *testing.T) {
	original := &StorageMeta{
		ContentTypes:  []string{"images", "backup"},
		Nodes:         []string{"node1", "node2"},
		ConsumerTypes: []string{"vm"},
		TopConsumers: []StorageConsumerMeta{
			{Name: "vm-100", DiskCount: 3},
		},
	}
	cloned := cloneStorageMeta(original)

	cloned.ContentTypes[0] = "MUTATED"
	if original.ContentTypes[0] == "MUTATED" {
		t.Error("mutating cloned ContentTypes should not affect original")
	}

	cloned.Nodes[0] = "MUTATED"
	if original.Nodes[0] == "MUTATED" {
		t.Error("mutating cloned Nodes should not affect original")
	}
}

func TestCloneStorageMeta_RiskIsolation(t *testing.T) {
	original := &StorageMeta{
		Risk: &StorageRisk{
			Level: storagehealth.RiskWarning,
			Reasons: []StorageRiskReason{
				{Code: "test", Summary: "test reason"},
			},
		},
	}
	cloned := cloneStorageMeta(original)

	cloned.Risk.Reasons[0].Summary = "MUTATED"
	if original.Risk.Reasons[0].Summary == "MUTATED" {
		t.Error("mutating cloned Risk.Reasons should not affect original")
	}

	cloned.Risk.Level = storagehealth.RiskCritical
	if original.Risk.Level == storagehealth.RiskCritical {
		t.Error("mutating cloned Risk.Level should not affect original")
	}
}

// --- cloneAgentData ---

func TestCloneAgentData_Nil(t *testing.T) {
	if cloneAgentData(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneAgentData_DeepIsolation(t *testing.T) {
	temp := 55.0
	original := &AgentData{
		AgentID:     "agent-1",
		Temperature: &temp,
		LoadAverage: []float64{0.5, 1.0, 1.5},
		DiskExclude: []string{"/dev/sda"},
		RAID: []HostRAIDMeta{
			{Device: "/dev/md0", Level: "raid1", Risk: &StorageRisk{Level: storagehealth.RiskHealthy}},
		},
	}
	cloned := cloneAgentData(original)

	*cloned.Temperature = 99.9
	if *original.Temperature != 55.0 {
		t.Error("mutating cloned temp should not affect original")
	}

	cloned.DiskExclude[0] = "MUTATED"
	if original.DiskExclude[0] == "MUTATED" {
		t.Error("mutating cloned DiskExclude should not affect original")
	}

	cloned.RAID[0].Risk.Level = storagehealth.RiskCritical
	if original.RAID[0].Risk.Level == storagehealth.RiskCritical {
		t.Error("mutating cloned RAID risk should not affect original")
	}
}

// --- cloneDockerData ---

func TestCloneDockerData_Nil(t *testing.T) {
	if cloneDockerData(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneDockerData_LabelsAndPortsIsolation(t *testing.T) {
	original := &DockerData{
		ContainerID: "abc123",
		Labels:      map[string]string{"env": "prod"},
		Ports: []DockerPortMeta{
			{PrivatePort: 8080, Protocol: "tcp"},
		},
	}
	cloned := cloneDockerData(original)

	cloned.Labels["new"] = "label"
	if _, exists := original.Labels["new"]; exists {
		t.Error("adding to cloned Labels should not affect original")
	}

	cloned.Ports[0].PublicPort = 9999
	if original.Ports[0].PublicPort == 9999 {
		t.Error("mutating cloned Ports should not affect original")
	}
}

// --- primitive clone helpers ---

func TestCloneStringSlice_Nil(t *testing.T) {
	if cloneStringSlice(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStringSlice_Empty(t *testing.T) {
	empty := []string{}
	cloned := cloneStringSlice(empty)
	if cloned == nil {
		t.Error("empty slice should clone to non-nil empty slice")
	}
	if len(cloned) != 0 {
		t.Error("empty slice clone should have length 0")
	}
}

func TestCloneStringSlice_Isolation(t *testing.T) {
	original := []string{"a", "b", "c"}
	cloned := cloneStringSlice(original)
	cloned[0] = "MUTATED"
	if original[0] == "MUTATED" {
		t.Error("mutation should not propagate")
	}
}

func TestCloneStringMap_Nil(t *testing.T) {
	if cloneStringMap(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStringMap_Isolation(t *testing.T) {
	original := map[string]string{"key": "val"}
	cloned := cloneStringMap(original)
	cloned["key"] = "MUTATED"
	if original["key"] == "MUTATED" {
		t.Error("mutation should not propagate")
	}
}

func TestCloneStringPtr_Nil(t *testing.T) {
	if cloneStringPtr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStringPtr_Isolation(t *testing.T) {
	s := "hello"
	cloned := cloneStringPtr(&s)
	*cloned = "MUTATED"
	if s == "MUTATED" {
		t.Error("mutation should not propagate")
	}
}

func TestCloneTimePtr_Nil(t *testing.T) {
	if cloneTimePtr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneTimePtr_Isolation(t *testing.T) {
	t1 := time.Now()
	cloned := cloneTimePtr(&t1)
	*cloned = time.Time{}
	if t1.IsZero() {
		t.Error("mutation should not propagate")
	}
}

func TestCloneBoolPtr_Nil(t *testing.T) {
	if cloneBoolPtr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneBoolPtr_Isolation(t *testing.T) {
	b := true
	cloned := cloneBoolPtr(&b)
	*cloned = false
	if !b {
		t.Error("mutation should not propagate")
	}
}

func TestCloneFloat64Ptr_Nil(t *testing.T) {
	if cloneFloat64Ptr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneFloat64Ptr_Isolation(t *testing.T) {
	f := 3.14
	cloned := cloneFloat64Ptr(&f)
	*cloned = 999.0
	if f != 3.14 {
		t.Error("mutation should not propagate")
	}
}

func TestCloneInt64Ptr_Nil(t *testing.T) {
	if cloneInt64Ptr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneInt64Ptr_Isolation(t *testing.T) {
	i := int64(42)
	cloned := cloneInt64Ptr(&i)
	*cloned = 999
	if i != 42 {
		t.Error("mutation should not propagate")
	}
}

// --- cloneParentBySourceMap ---

func TestCloneParentBySourceMap_NilVsEmpty(t *testing.T) {
	if cloneParentBySourceMap(nil) != nil {
		t.Error("nil should clone to nil")
	}

	empty := map[DataSource]string{}
	cloned := cloneParentBySourceMap(empty)
	if cloned == nil {
		t.Error("empty map should clone to non-nil empty map")
	}
}

// --- cloneTemperature ---

func TestCloneTemperature_Nil(t *testing.T) {
	if cloneTemperature(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneTemperature_SliceIsolation(t *testing.T) {
	original := &models.Temperature{
		Cores: []models.CoreTemp{{Core: 0, Temp: 45.0}},
		GPU:   []models.GPUTemp{{Device: "gpu0", Edge: 55.0}},
		NVMe:  []models.NVMeTemp{{Device: "nvme0", Temp: 35.0}},
		SMART: []models.DiskTemp{{Device: "/dev/sda", Temperature: 30}},
	}
	cloned := cloneTemperature(original)

	cloned.Cores[0].Temp = 99.0
	if original.Cores[0].Temp != 45.0 {
		t.Error("mutating cloned Cores should not affect original")
	}
}

// --- zeroTimeToPtr ---

func TestZeroTimeToPtr_ZeroReturnsNil(t *testing.T) {
	if zeroTimeToPtr(time.Time{}) != nil {
		t.Error("zero time should return nil")
	}
}

func TestZeroTimeToPtr_NonZeroReturnsPtr(t *testing.T) {
	now := time.Now()
	result := zeroTimeToPtr(now)
	if result == nil {
		t.Fatal("non-zero time should return non-nil pointer")
	}
	if !result.Equal(now) {
		t.Error("returned time should equal input")
	}
}
