package models

import (
	"testing"
)

// This file raises BRANCH coverage for the NormalizeCollections methods on
// the NON-frontend (core domain) types defined in models.go.
//
// For every target type we exercise BOTH arms of each nil-collection
// conditional:
//
//   - nil arm:    the collection field is left nil; after NormalizeCollections
//                  it MUST be a non-nil empty slice/map of the right type.
//   - populated:  the collection field is pre-populated; NormalizeCollections
//                  MUST preserve it (these methods do NOT sort or deduplicate).
//                  Where the method recurses into nested elements (Host,
//                  DockerHost, KubernetesCluster, HostCephCluster,
//                  HostCephHealth, Node/VM/Container), the populated arm
//                  supplies a nested element with a nil sub-collection so the
//                  observable side-effect of the recursion (the sub-collection
//                  becoming non-nil) can be asserted, exercising the for-loop
//                  bodies and the `!= nil` guard branches.
//
// No source file or sibling test was modified. The *Frontend variants are
// covered by models_frontend_branchcov0718_test.go and are intentionally NOT
// duplicated here.

func TestCoreNormalizeCollections_BranchCov0718(t *testing.T) {
	// ---------------- Node ----------------
	t.Run("Node_nil_LoadAverage_and_nil_Temperature", func(t *testing.T) {
		n := Node{ID: "n-1"} // LoadAverage nil, Temperature nil
		out := n.NormalizeCollections()
		if out.LoadAverage == nil {
			t.Fatalf("LoadAverage should be non-nil after normalize")
		}
		if len(out.LoadAverage) != 0 {
			t.Fatalf("LoadAverage should be empty, got len=%d", len(out.LoadAverage))
		}
		// Temperature is nil on input -> must remain nil (no fabrication).
		if out.Temperature != nil {
			t.Fatalf("Temperature must remain nil when nil on input")
		}
	})
	t.Run("Node_populated_with_Temperature_recurses", func(t *testing.T) {
		n := Node{
			ID:          "n-1",
			LoadAverage: []float64{0.5, 0.4, 0.3},
			Temperature: &Temperature{}, // CoreCount nil -> recursion observable
		}
		out := n.NormalizeCollections()
		if len(out.LoadAverage) != 3 || out.LoadAverage[2] != 0.3 {
			t.Fatalf("LoadAverage not preserved: %+v", out.LoadAverage)
		}
		if out.Temperature == nil {
			t.Fatalf("Temperature must be preserved when non-nil on input")
		}
	})

	// ---------------- VM ----------------
	t.Run("VM_nil_all_collections", func(t *testing.T) {
		v := VM{ID: "v-1"}
		out := v.NormalizeCollections()
		if out.Disks == nil || out.IPAddresses == nil || out.NetworkInterfaces == nil || out.Tags == nil {
			t.Fatalf("Disks/IPAddresses/NetworkInterfaces/Tags must all be non-nil after normalize")
		}
	})
	t.Run("VM_populated_recurses_NetworkInterfaces", func(t *testing.T) {
		v := VM{
			ID:                "v-1",
			Disks:             []Disk{{Device: "/sda"}},
			IPAddresses:       []string{"10.0.0.1", "10.0.0.1"},        // duplicate on purpose
			NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}}, // IPs nil -> recursion observable
			Tags:              []string{"prod", "prod"},
		}
		out := v.NormalizeCollections()
		if len(out.Disks) != 1 || out.Disks[0].Device != "/sda" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
		if len(out.IPAddresses) != 2 || out.IPAddresses[0] != "10.0.0.1" || out.IPAddresses[1] != "10.0.0.1" {
			t.Fatalf("IPAddresses not preserved (no dedup expected): %+v", out.IPAddresses)
		}
		if len(out.Tags) != 2 || out.Tags[0] != "prod" || out.Tags[1] != "prod" {
			t.Fatalf("Tags not preserved (no dedup expected): %+v", out.Tags)
		}
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eth0" {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if out.NetworkInterfaces[0].Addresses == nil {
			t.Fatalf("nested GuestNetworkInterface.Addresses not normalized (recursion did not run)")
		}
	})

	// ---------------- Container ----------------
	t.Run("Container_nil_all_collections", func(t *testing.T) {
		c := Container{ID: "c-1"}
		out := c.NormalizeCollections()
		if out.Disks == nil || out.IPAddresses == nil || out.NetworkInterfaces == nil || out.Tags == nil {
			t.Fatalf("Disks/IPAddresses/NetworkInterfaces/Tags must all be non-nil after normalize")
		}
	})
	t.Run("Container_populated_recurses_NetworkInterfaces", func(t *testing.T) {
		c := Container{
			ID:                "c-1",
			Disks:             []Disk{{Device: "/data"}},
			IPAddresses:       []string{"10.0.0.2"},
			NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}}, // IPs nil
			Tags:              []string{"env:dev"},
		}
		out := c.NormalizeCollections()
		if len(out.Disks) != 1 || out.Disks[0].Device != "/data" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
		if len(out.IPAddresses) != 1 || out.IPAddresses[0] != "10.0.0.2" {
			t.Fatalf("IPAddresses not preserved: %+v", out.IPAddresses)
		}
		if len(out.Tags) != 1 || out.Tags[0] != "env:dev" {
			t.Fatalf("Tags not preserved: %+v", out.Tags)
		}
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eth0" {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if out.NetworkInterfaces[0].Addresses == nil {
			t.Fatalf("nested GuestNetworkInterface.Addresses not normalized (recursion did not run)")
		}
	})

	// ---------------- Host (aggregator) ----------------
	t.Run("Host_nil_all_collections_and_nil_pointers", func(t *testing.T) {
		h := Host{ID: "h-1"}
		out := h.NormalizeCollections()
		if out.LoadAverage == nil || out.Disks == nil || out.DiskIO == nil ||
			out.NetworkInterfaces == nil || out.RAID == nil || out.Tags == nil || out.DiskExclude == nil {
			t.Fatalf("LoadAverage/Disks/DiskIO/NetworkInterfaces/RAID/Tags/DiskExclude must all be non-nil after normalize")
		}
		// Sensors is a value (not pointer) -> always normalized, even from zero value.
		if out.Sensors.TemperatureCelsius == nil || out.Sensors.GPU == nil || out.Sensors.SMART == nil {
			t.Fatalf("nested HostSensorSummary collections must be normalized unconditionally")
		}
		// Pointer fields nil on input -> must remain nil.
		if out.Unraid != nil || out.Ceph != nil {
			t.Fatalf("Unraid/Ceph must remain nil when nil on input")
		}
	})
	t.Run("Host_populated_recurses_all_nested", func(t *testing.T) {
		h := Host{
			ID:                "h-1",
			LoadAverage:       []float64{1.0, 2.0},
			Disks:             []Disk{{Device: "/"}},
			DiskIO:            []DiskIO{{Device: "sda"}},
			NetworkInterfaces: []HostNetworkInterface{{Name: "eno1"}}, // Addresses nil
			Sensors:           HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 50.0}},
			RAID:              []HostRAIDArray{{Device: "md0"}},       // Devices nil
			Unraid:            &HostUnraidStorage{ArrayStarted: true}, // Disks nil
			Ceph:              &HostCephCluster{FSID: "ceph-fsid"},    // Pools/Services nil, Health/MonMap normalized
			Tags:              []string{"prod", "prod"},
			DiskExclude:       []string{"/dev/sda"},
		}
		out := h.NormalizeCollections()

		if len(out.LoadAverage) != 2 || out.LoadAverage[1] != 2.0 {
			t.Fatalf("LoadAverage not preserved: %+v", out.LoadAverage)
		}
		if len(out.Disks) != 1 || out.Disks[0].Device != "/" {
			t.Fatalf("Disks not preserved")
		}
		if len(out.DiskIO) != 1 || out.DiskIO[0].Device != "sda" {
			t.Fatalf("DiskIO not preserved")
		}
		if len(out.Tags) != 2 || out.Tags[0] != "prod" || out.Tags[1] != "prod" {
			t.Fatalf("Tags not preserved (no dedup expected): %+v", out.Tags)
		}
		if len(out.DiskExclude) != 1 || out.DiskExclude[0] != "/dev/sda" {
			t.Fatalf("DiskExclude not preserved: %+v", out.DiskExclude)
		}
		// Sensors preserved.
		if len(out.Sensors.TemperatureCelsius) != 1 || out.Sensors.TemperatureCelsius["cpu"] != 50.0 {
			t.Fatalf("Sensors.TemperatureCelsius not preserved: %+v", out.Sensors.TemperatureCelsius)
		}
		// NetworkInterfaces recursion.
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eno1" {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if out.NetworkInterfaces[0].Addresses == nil {
			t.Fatalf("nested HostNetworkInterface.Addresses not normalized")
		}
		// RAID recursion.
		if len(out.RAID) != 1 || out.RAID[0].Device != "md0" {
			t.Fatalf("RAID not preserved")
		}
		if out.RAID[0].Devices == nil {
			t.Fatalf("nested HostRAIDArray.Devices not normalized")
		}
		// Unraid pointer recursion.
		if out.Unraid == nil || out.Unraid.Disks == nil {
			t.Fatalf("nested HostUnraidStorage.Disks not normalized")
		}
		// Ceph pointer recursion.
		if out.Ceph == nil {
			t.Fatalf("Ceph must be preserved when non-nil on input")
		}
		if out.Ceph.Pools == nil || out.Ceph.Services == nil {
			t.Fatalf("nested HostCephCluster Pools/Services not normalized")
		}
		if out.Ceph.Health.Checks == nil {
			t.Fatalf("nested HostCephHealth.Checks not normalized")
		}
	})

	// ---------------- HostNetworkInterface ----------------
	t.Run("HostNetworkInterface_nil_Addresses", func(t *testing.T) {
		i := HostNetworkInterface{Name: "eth0"}
		out := i.NormalizeCollections()
		if out.Addresses == nil {
			t.Fatalf("Addresses should be non-nil after normalize")
		}
		if len(out.Addresses) != 0 {
			t.Fatalf("Addresses should be empty, got len=%d", len(out.Addresses))
		}
	})
	t.Run("HostNetworkInterface_populated_preserved", func(t *testing.T) {
		i := HostNetworkInterface{Name: "eth0", Addresses: []string{"10.0.0.5", "10.0.0.5"}}
		out := i.NormalizeCollections()
		if len(out.Addresses) != 2 || out.Addresses[0] != "10.0.0.5" || out.Addresses[1] != "10.0.0.5" {
			t.Fatalf("Addresses not preserved (no dedup expected): %+v", out.Addresses)
		}
	})

	// ---------------- HostSensorSummary ----------------
	t.Run("HostSensorSummary_nil_all_six", func(t *testing.T) {
		s := HostSensorSummary{}
		out := s.NormalizeCollections()
		if out.TemperatureCelsius == nil || out.FanRPM == nil || out.PowerWatts == nil ||
			out.Additional == nil || out.GPU == nil || out.SMART == nil {
			t.Fatalf("all six collection fields must be non-nil after normalize")
		}
	})
	t.Run("HostSensorSummary_populated_preserved", func(t *testing.T) {
		s := HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 65.0},
			FanRPM:             map[string]float64{"fan1": 1500},
			PowerWatts:         map[string]float64{"psu1": 220.5},
			Additional:         map[string]float64{"voltage": 12.0},
			GPU:                []HostGPUSensor{{ID: "gpu0", Name: "nvidia"}},
			SMART:              []HostDiskSMART{{Device: "sda", Temperature: 40}},
		}
		out := s.NormalizeCollections()
		if len(out.TemperatureCelsius) != 1 || out.TemperatureCelsius["cpu"] != 65.0 {
			t.Fatalf("TemperatureCelsius not preserved: %+v", out.TemperatureCelsius)
		}
		if len(out.FanRPM) != 1 || out.FanRPM["fan1"] != 1500 {
			t.Fatalf("FanRPM not preserved: %+v", out.FanRPM)
		}
		if len(out.PowerWatts) != 1 || out.PowerWatts["psu1"] != 220.5 {
			t.Fatalf("PowerWatts not preserved: %+v", out.PowerWatts)
		}
		if len(out.Additional) != 1 || out.Additional["voltage"] != 12.0 {
			t.Fatalf("Additional not preserved: %+v", out.Additional)
		}
		if len(out.GPU) != 1 || out.GPU[0].Name != "nvidia" {
			t.Fatalf("GPU not preserved: %+v", out.GPU)
		}
		if len(out.SMART) != 1 || out.SMART[0].Device != "sda" || out.SMART[0].Temperature != 40 {
			t.Fatalf("SMART not preserved: %+v", out.SMART)
		}
	})

	// ---------------- HostRAIDArray ----------------
	t.Run("HostRAIDArray_nil_Devices", func(t *testing.T) {
		a := HostRAIDArray{Device: "md0", Level: "1"}
		out := a.NormalizeCollections()
		if out.Devices == nil {
			t.Fatalf("Devices should be non-nil after normalize")
		}
		if len(out.Devices) != 0 {
			t.Fatalf("Devices should be empty, got len=%d", len(out.Devices))
		}
	})
	t.Run("HostRAIDArray_populated_preserved", func(t *testing.T) {
		a := HostRAIDArray{
			Device:  "md0",
			Devices: []HostRAIDDevice{{Device: "sda1", State: "in_sync", Slot: 0}},
		}
		out := a.NormalizeCollections()
		if len(out.Devices) != 1 || out.Devices[0].Device != "sda1" || out.Devices[0].Slot != 0 {
			t.Fatalf("Devices not preserved: %+v", out.Devices)
		}
	})

	// ---------------- HostUnraidStorage ----------------
	t.Run("HostUnraidStorage_nil_Disks", func(t *testing.T) {
		s := HostUnraidStorage{ArrayStarted: true}
		out := s.NormalizeCollections()
		if out.Disks == nil {
			t.Fatalf("Disks should be non-nil after normalize")
		}
		if len(out.Disks) != 0 {
			t.Fatalf("Disks should be empty, got len=%d", len(out.Disks))
		}
	})
	t.Run("HostUnraidStorage_populated_preserved", func(t *testing.T) {
		s := HostUnraidStorage{
			Disks: []HostUnraidDisk{{Name: "parity", Role: "parity"}},
		}
		out := s.NormalizeCollections()
		if len(out.Disks) != 1 || out.Disks[0].Name != "parity" || out.Disks[0].Role != "parity" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
	})

	// ---------------- HostCephCluster (aggregator) ----------------
	t.Run("HostCephCluster_nil_Pools_Services", func(t *testing.T) {
		c := HostCephCluster{FSID: "fsid-1"}
		out := c.NormalizeCollections()
		if out.Pools == nil || out.Services == nil {
			t.Fatalf("Pools/Services should be non-nil after normalize")
		}
		// Health and MonMap are normalized unconditionally (value receivers).
		if out.Health.Checks == nil || out.Health.Summary == nil {
			t.Fatalf("Health.Checks/Summary must be normalized unconditionally")
		}
		if out.MonMap.Monitors == nil {
			t.Fatalf("MonMap.Monitors must be normalized unconditionally")
		}
	})
	t.Run("HostCephCluster_populated_recurses", func(t *testing.T) {
		c := HostCephCluster{
			FSID: "fsid-1",
			Health: HostCephHealth{
				Checks: map[string]HostCephCheck{"PG_AVAIL": {Severity: "WARN"}}, // Detail nil
			},
			Services: []HostCephService{{Type: "mon"}}, // Daemons nil
			Pools:    []HostCephPool{{Name: "replicapool"}},
		}
		out := c.NormalizeCollections()
		if len(out.Pools) != 1 || out.Pools[0].Name != "replicapool" {
			t.Fatalf("Pools not preserved: %+v", out.Pools)
		}
		if len(out.Services) != 1 || out.Services[0].Type != "mon" {
			t.Fatalf("Services not preserved: %+v", out.Services)
		}
		// Services recursion.
		if out.Services[0].Daemons == nil {
			t.Fatalf("nested HostCephService.Daemons not normalized")
		}
		// Health recursion over Checks map.
		if len(out.Health.Checks) != 1 {
			t.Fatalf("Health.Checks not preserved: %+v", out.Health.Checks)
		}
		if out.Health.Checks["PG_AVAIL"].Detail == nil {
			t.Fatalf("nested HostCephCheck.Detail not normalized")
		}
	})

	// ---------------- HostCephHealth ----------------
	t.Run("HostCephHealth_nil_Checks_Summary", func(t *testing.T) {
		h := HostCephHealth{Status: "HEALTH_OK"}
		out := h.NormalizeCollections()
		if out.Checks == nil || out.Summary == nil {
			t.Fatalf("Checks/Summary should be non-nil after normalize")
		}
		if len(out.Checks) != 0 || len(out.Summary) != 0 {
			t.Fatalf("Checks/Summary should be empty after normalize")
		}
		if out.Status != "HEALTH_OK" {
			t.Fatalf("Status scalar must be preserved")
		}
	})
	t.Run("HostCephHealth_populated_recurses_checks", func(t *testing.T) {
		h := HostCephHealth{
			Status:  "HEALTH_WARN",
			Checks:  map[string]HostCephCheck{"POOL": {Severity: "WARN"}}, // Detail nil
			Summary: []HostCephHealthSummary{{Severity: "WARNING", Message: "pool near full"}},
		}
		out := h.NormalizeCollections()
		if out.Status != "HEALTH_WARN" {
			t.Fatalf("Status not preserved: %q", out.Status)
		}
		if len(out.Summary) != 1 || out.Summary[0].Message != "pool near full" {
			t.Fatalf("Summary not preserved: %+v", out.Summary)
		}
		if len(out.Checks) != 1 {
			t.Fatalf("Checks not preserved: %+v", out.Checks)
		}
		if out.Checks["POOL"].Severity != "WARN" {
			t.Fatalf("Checks[POOL].Severity not preserved")
		}
		if out.Checks["POOL"].Detail == nil {
			t.Fatalf("nested HostCephCheck.Detail not normalized")
		}
	})

	// ---------------- HostCephCheck ----------------
	t.Run("HostCephCheck_nil_Detail", func(t *testing.T) {
		c := HostCephCheck{Severity: "ERR"}
		out := c.NormalizeCollections()
		if out.Detail == nil {
			t.Fatalf("Detail should be non-nil after normalize")
		}
		if len(out.Detail) != 0 {
			t.Fatalf("Detail should be empty, got len=%d", len(out.Detail))
		}
		if out.Severity != "ERR" {
			t.Fatalf("Severity scalar must be preserved")
		}
	})
	t.Run("HostCephCheck_populated_preserved", func(t *testing.T) {
		c := HostCephCheck{
			Severity: "WARN",
			Detail:   []string{"msg1", "msg1"}, // duplicate on purpose
		}
		out := c.NormalizeCollections()
		if len(out.Detail) != 2 || out.Detail[0] != "msg1" || out.Detail[1] != "msg1" {
			t.Fatalf("Detail not preserved (no dedup expected): %+v", out.Detail)
		}
	})

	// ---------------- HostCephMonitorMap ----------------
	t.Run("HostCephMonitorMap_nil_Monitors", func(t *testing.T) {
		m := HostCephMonitorMap{Epoch: 3}
		out := m.NormalizeCollections()
		if out.Monitors == nil {
			t.Fatalf("Monitors should be non-nil after normalize")
		}
		if len(out.Monitors) != 0 {
			t.Fatalf("Monitors should be empty, got len=%d", len(out.Monitors))
		}
		if out.Epoch != 3 {
			t.Fatalf("Epoch scalar must be preserved")
		}
	})
	t.Run("HostCephMonitorMap_populated_preserved", func(t *testing.T) {
		m := HostCephMonitorMap{
			Monitors: []HostCephMonitor{{Name: "mon.a", Rank: 0, Status: "ok"}},
		}
		out := m.NormalizeCollections()
		if len(out.Monitors) != 1 || out.Monitors[0].Name != "mon.a" || out.Monitors[0].Rank != 0 {
			t.Fatalf("Monitors not preserved: %+v", out.Monitors)
		}
	})

	// ---------------- HostCephService ----------------
	t.Run("HostCephService_nil_Daemons", func(t *testing.T) {
		s := HostCephService{Type: "osd", Running: 3}
		out := s.NormalizeCollections()
		if out.Daemons == nil {
			t.Fatalf("Daemons should be non-nil after normalize")
		}
		if len(out.Daemons) != 0 {
			t.Fatalf("Daemons should be empty, got len=%d", len(out.Daemons))
		}
		if out.Running != 3 {
			t.Fatalf("Running scalar must be preserved")
		}
	})
	t.Run("HostCephService_populated_preserved", func(t *testing.T) {
		s := HostCephService{
			Type:    "mon",
			Daemons: []string{"mon.a", "mon.b"},
		}
		out := s.NormalizeCollections()
		if len(out.Daemons) != 2 || out.Daemons[0] != "mon.a" || out.Daemons[1] != "mon.b" {
			t.Fatalf("Daemons not preserved: %+v", out.Daemons)
		}
	})

	// ---------------- DockerHost (aggregator) ----------------
	t.Run("DockerHost_nil_all_collections", func(t *testing.T) {
		h := DockerHost{ID: "dh-1"}
		out := h.NormalizeCollections()
		if out.LoadAverage == nil || out.Disks == nil || out.NetworkInterfaces == nil ||
			out.Containers == nil || out.Images == nil || out.Volumes == nil || out.Networks == nil ||
			out.Services == nil || out.Tasks == nil || out.Nodes == nil || out.Secrets == nil || out.Configs == nil {
			t.Fatalf("all top-level collection fields must be non-nil after normalize")
		}
		if out.Security != nil || out.IdentityConflict != nil {
			t.Fatalf("Security/IdentityConflict must remain nil when nil on input")
		}
	})
	t.Run("DockerHost_populated_recurses_all_nested", func(t *testing.T) {
		h := DockerHost{
			ID:                "dh-1",
			LoadAverage:       []float64{1.5},
			Disks:             []Disk{{Device: "/dev/sda"}},
			NetworkInterfaces: []HostNetworkInterface{{Name: "eno1"}},         // Addresses nil
			Containers:        []DockerContainer{{ID: "ctr-1"}},               // Ports/Labels/Networks/Mounts nil
			Images:            []DockerImage{{ID: "img-1"}},                   // RepoTags/RepoDigests/Labels nil
			Volumes:           []DockerVolume{{Name: "vol-1"}},                // Labels/Options nil
			Networks:          []DockerNetwork{{ID: "net-1", Name: "bridge"}}, // Subnets/Labels/Options nil
			Services:          []DockerService{{ID: "svc-1"}},                 // Labels/EndpointPorts nil
			Tasks:             []DockerTask{{ID: "task-1"}},
			Nodes:             []DockerNode{{ID: "node-1"}},  // Labels/EngineLabels nil
			Secrets:           []DockerSecret{{ID: "sec-1"}}, // Labels nil
			Configs:           []DockerConfig{{ID: "cfg-1"}}, // Labels nil
			Security:          &DockerHostSecurity{},         // AuthorizationPlugins nil
			IdentityConflict:  &DockerHostIdentityConflict{}, // Hostnames nil
		}
		out := h.NormalizeCollections()

		if len(out.LoadAverage) != 1 || out.LoadAverage[0] != 1.5 {
			t.Fatalf("LoadAverage not preserved: %+v", out.LoadAverage)
		}
		if len(out.Disks) != 1 || out.Disks[0].Device != "/dev/sda" {
			t.Fatalf("Disks not preserved")
		}
		if len(out.Tasks) != 1 || out.Tasks[0].ID != "task-1" {
			t.Fatalf("Tasks not preserved")
		}
		// Recursion observable for each nested element.
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Addresses == nil {
			t.Fatalf("nested HostNetworkInterface.Addresses not normalized")
		}
		c := out.Containers
		if len(c) != 1 || c[0].Ports == nil || c[0].Labels == nil || c[0].Networks == nil || c[0].Mounts == nil {
			t.Fatalf("nested DockerContainer collections not normalized: %+v", c)
		}
		if len(out.Images) != 1 || out.Images[0].RepoTags == nil || out.Images[0].RepoDigests == nil || out.Images[0].Labels == nil {
			t.Fatalf("nested DockerImage collections not normalized")
		}
		if len(out.Volumes) != 1 || out.Volumes[0].Labels == nil || out.Volumes[0].Options == nil {
			t.Fatalf("nested DockerVolume collections not normalized")
		}
		if len(out.Networks) != 1 || out.Networks[0].Subnets == nil || out.Networks[0].Labels == nil || out.Networks[0].Options == nil {
			t.Fatalf("nested DockerNetwork collections not normalized")
		}
		if len(out.Services) != 1 || out.Services[0].Labels == nil || out.Services[0].EndpointPorts == nil {
			t.Fatalf("nested DockerService collections not normalized")
		}
		if len(out.Nodes) != 1 || out.Nodes[0].Labels == nil || out.Nodes[0].EngineLabels == nil {
			t.Fatalf("nested DockerNode collections not normalized")
		}
		if len(out.Secrets) != 1 || out.Secrets[0].Labels == nil {
			t.Fatalf("nested DockerSecret.Labels not normalized")
		}
		if len(out.Configs) != 1 || out.Configs[0].Labels == nil {
			t.Fatalf("nested DockerConfig.Labels not normalized")
		}
		// Pointer recursion.
		if out.Security == nil || out.Security.AuthorizationPlugins == nil {
			t.Fatalf("nested DockerHostSecurity.AuthorizationPlugins not normalized")
		}
		if out.IdentityConflict == nil || out.IdentityConflict.Hostnames == nil {
			t.Fatalf("nested DockerHostIdentityConflict.Hostnames not normalized")
		}
	})

	// ---------------- DockerHostIdentityConflict ----------------
	t.Run("DockerHostIdentityConflict_nil_Hostnames", func(t *testing.T) {
		c := DockerHostIdentityConflict{}
		out := c.NormalizeCollections()
		if out.Hostnames == nil {
			t.Fatalf("Hostnames should be non-nil after normalize")
		}
		if len(out.Hostnames) != 0 {
			t.Fatalf("Hostnames should be empty, got len=%d", len(out.Hostnames))
		}
	})
	t.Run("DockerHostIdentityConflict_populated_preserved", func(t *testing.T) {
		c := DockerHostIdentityConflict{
			Hostnames: []string{"h-1", "h-2", "h-1"}, // duplicate on purpose
		}
		out := c.NormalizeCollections()
		if len(out.Hostnames) != 3 || out.Hostnames[0] != "h-1" || out.Hostnames[2] != "h-1" {
			t.Fatalf("Hostnames not preserved (no dedup expected): %+v", out.Hostnames)
		}
	})

	// ---------------- DockerHostSecurity ----------------
	t.Run("DockerHostSecurity_nil_AuthorizationPlugins", func(t *testing.T) {
		s := DockerHostSecurity{MutatingCommandsBlocked: true}
		out := s.NormalizeCollections()
		if out.AuthorizationPlugins == nil {
			t.Fatalf("AuthorizationPlugins should be non-nil after normalize")
		}
		if len(out.AuthorizationPlugins) != 0 {
			t.Fatalf("AuthorizationPlugins should be empty, got len=%d", len(out.AuthorizationPlugins))
		}
		if !out.MutatingCommandsBlocked {
			t.Fatalf("scalar MutatingCommandsBlocked must be preserved")
		}
	})
	t.Run("DockerHostSecurity_populated_preserved", func(t *testing.T) {
		s := DockerHostSecurity{
			AuthorizationPlugins: []string{"opa", "opa"}, // duplicate on purpose
		}
		out := s.NormalizeCollections()
		if len(out.AuthorizationPlugins) != 2 || out.AuthorizationPlugins[0] != "opa" || out.AuthorizationPlugins[1] != "opa" {
			t.Fatalf("AuthorizationPlugins not preserved (no dedup expected): %+v", out.AuthorizationPlugins)
		}
	})

	// ---------------- DockerContainer ----------------
	t.Run("DockerContainer_nil_all_four", func(t *testing.T) {
		c := DockerContainer{ID: "ctr-1"}
		out := c.NormalizeCollections()
		if out.Ports == nil || out.Labels == nil || out.Networks == nil || out.Mounts == nil {
			t.Fatalf("Ports/Labels/Networks/Mounts must all be non-nil after normalize")
		}
	})
	t.Run("DockerContainer_populated_preserved", func(t *testing.T) {
		c := DockerContainer{
			ID:       "ctr-1",
			Ports:    []DockerContainerPort{{PrivatePort: 80, Protocol: "tcp"}},
			Labels:   map[string]string{"compose.service": "web"},
			Networks: []DockerContainerNetworkLink{{Name: "bridge", IPv4: "172.17.0.2"}},
			Mounts:   []DockerContainerMount{{Type: "bind", Source: "/host"}},
		}
		out := c.NormalizeCollections()
		if len(out.Ports) != 1 || out.Ports[0].PrivatePort != 80 || out.Ports[0].Protocol != "tcp" {
			t.Fatalf("Ports not preserved: %+v", out.Ports)
		}
		if len(out.Labels) != 1 || out.Labels["compose.service"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Networks) != 1 || out.Networks[0].IPv4 != "172.17.0.2" {
			t.Fatalf("Networks not preserved: %+v", out.Networks)
		}
		if len(out.Mounts) != 1 || out.Mounts[0].Source != "/host" {
			t.Fatalf("Mounts not preserved: %+v", out.Mounts)
		}
	})

	// ---------------- DockerImage ----------------
	t.Run("DockerImage_nil_all_three", func(t *testing.T) {
		i := DockerImage{ID: "img-1"}
		out := i.NormalizeCollections()
		if out.RepoTags == nil || out.RepoDigests == nil || out.Labels == nil {
			t.Fatalf("RepoTags/RepoDigests/Labels must all be non-nil after normalize")
		}
	})
	t.Run("DockerImage_populated_preserved", func(t *testing.T) {
		i := DockerImage{
			ID:          "img-1",
			RepoTags:    []string{"nginx:latest", "nginx:latest"},
			RepoDigests: []string{"sha256:abc"},
			Labels:      map[string]string{"maintainer": "team"},
		}
		out := i.NormalizeCollections()
		if len(out.RepoTags) != 2 || out.RepoTags[0] != "nginx:latest" || out.RepoTags[1] != "nginx:latest" {
			t.Fatalf("RepoTags not preserved (no dedup expected): %+v", out.RepoTags)
		}
		if len(out.RepoDigests) != 1 || out.RepoDigests[0] != "sha256:abc" {
			t.Fatalf("RepoDigests not preserved: %+v", out.RepoDigests)
		}
		if len(out.Labels) != 1 || out.Labels["maintainer"] != "team" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- DockerVolume ----------------
	t.Run("DockerVolume_nil_Labels_Options", func(t *testing.T) {
		v := DockerVolume{Name: "vol-1"}
		out := v.NormalizeCollections()
		if out.Labels == nil || out.Options == nil {
			t.Fatalf("Labels/Options should be non-nil after normalize")
		}
		if len(out.Labels) != 0 || len(out.Options) != 0 {
			t.Fatalf("Labels/Options should be empty after normalize")
		}
	})
	t.Run("DockerVolume_populated_preserved", func(t *testing.T) {
		v := DockerVolume{
			Name:    "vol-1",
			Labels:  map[string]string{"env": "prod"},
			Options: map[string]string{"device": "tmpfs"},
		}
		out := v.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["env"] != "prod" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Options) != 1 || out.Options["device"] != "tmpfs" {
			t.Fatalf("Options not preserved: %+v", out.Options)
		}
	})

	// ---------------- DockerNetwork ----------------
	t.Run("DockerNetwork_nil_all_three", func(t *testing.T) {
		n := DockerNetwork{ID: "net-1", Name: "bridge"}
		out := n.NormalizeCollections()
		if out.Subnets == nil || out.Labels == nil || out.Options == nil {
			t.Fatalf("Subnets/Labels/Options must all be non-nil after normalize")
		}
	})
	t.Run("DockerNetwork_populated_preserved", func(t *testing.T) {
		n := DockerNetwork{
			ID:      "net-1",
			Name:    "bridge",
			Subnets: []DockerNetworkSubnet{{Subnet: "172.17.0.0/16", Gateway: "172.17.0.1"}},
			Labels:  map[string]string{"driver": "bridge"},
			Options: map[string]string{"com.docker.network.bridge.enable_icc": "true"},
		}
		out := n.NormalizeCollections()
		if len(out.Subnets) != 1 || out.Subnets[0].Gateway != "172.17.0.1" {
			t.Fatalf("Subnets not preserved: %+v", out.Subnets)
		}
		if len(out.Labels) != 1 || out.Labels["driver"] != "bridge" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Options) != 1 || out.Options["com.docker.network.bridge.enable_icc"] != "true" {
			t.Fatalf("Options not preserved: %+v", out.Options)
		}
	})

	// ---------------- DockerNode (was 60%) ----------------
	t.Run("DockerNode_nil_Labels_EngineLabels", func(t *testing.T) {
		n := DockerNode{ID: "node-1"}
		out := n.NormalizeCollections()
		if out.Labels == nil || out.EngineLabels == nil {
			t.Fatalf("Labels/EngineLabels should be non-nil after normalize")
		}
		if len(out.Labels) != 0 || len(out.EngineLabels) != 0 {
			t.Fatalf("Labels/EngineLabels should be empty after normalize")
		}
	})
	t.Run("DockerNode_populated_preserved", func(t *testing.T) {
		n := DockerNode{
			ID:           "node-1",
			Labels:       map[string]string{"role": "worker"},
			EngineLabels: map[string]string{"foo": "bar"},
		}
		out := n.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["role"] != "worker" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.EngineLabels) != 1 || out.EngineLabels["foo"] != "bar" {
			t.Fatalf("EngineLabels not preserved: %+v", out.EngineLabels)
		}
	})

	// ---------------- KubernetesCluster (aggregator) ----------------
	t.Run("KubernetesCluster_nil_all_slice_fields", func(t *testing.T) {
		c := KubernetesCluster{ID: "k-1"}
		out := c.NormalizeCollections()
		// Assert EVERY normalized slice is non-nil (the nil arm ran for each).
		for label, got := range map[string][]struct{}{
			"Nodes":                    castSlice(out.Nodes),
			"Namespaces":               castSlice(out.Namespaces),
			"Pods":                     castSlice(out.Pods),
			"Deployments":              castSlice(out.Deployments),
			"ReplicaSets":              castSlice(out.ReplicaSets),
			"StatefulSets":             castSlice(out.StatefulSets),
			"DaemonSets":               castSlice(out.DaemonSets),
			"Services":                 castSlice(out.Services),
			"Jobs":                     castSlice(out.Jobs),
			"CronJobs":                 castSlice(out.CronJobs),
			"Ingresses":                castSlice(out.Ingresses),
			"EndpointSlices":           castSlice(out.EndpointSlices),
			"NetworkPolicies":          castSlice(out.NetworkPolicies),
			"PersistentVolumes":        castSlice(out.PersistentVolumes),
			"PersistentVolumeClaims":   castSlice(out.PersistentVolumeClaims),
			"StorageClasses":           castSlice(out.StorageClasses),
			"ConfigMaps":               castSlice(out.ConfigMaps),
			"Secrets":                  castSlice(out.Secrets),
			"ServiceAccounts":          castSlice(out.ServiceAccounts),
			"Roles":                    castSlice(out.Roles),
			"ClusterRoles":             castSlice(out.ClusterRoles),
			"RoleBindings":             castSlice(out.RoleBindings),
			"ClusterRoleBindings":      castSlice(out.ClusterRoleBindings),
			"ResourceQuotas":           castSlice(out.ResourceQuotas),
			"LimitRanges":              castSlice(out.LimitRanges),
			"PodDisruptionBudgets":     castSlice(out.PodDisruptionBudgets),
			"HorizontalPodAutoscalers": castSlice(out.HorizontalPodAutoscalers),
			"Events":                   castSlice(out.Events),
		} {
			if got == nil {
				t.Fatalf("KubernetesCluster.%s should be non-nil after normalize", label)
			}
		}
	})
	t.Run("KubernetesCluster_populated_recurses_every_nested", func(t *testing.T) {
		// Every nested element has nil sub-collections so the parent's
		// per-element recursion is observable (sub-collection -> non-nil).
		c := KubernetesCluster{
			ID:                       "k-1",
			Nodes:                    []KubernetesNode{{UID: "n-1"}},                      // Roles nil
			Namespaces:               []KubernetesNamespace{{UID: "ns-1"}},                // Labels nil
			Pods:                     []KubernetesPod{{UID: "p-1"}},                       // Labels/Containers nil
			Deployments:              []KubernetesDeployment{{UID: "d-1"}},                // Labels nil
			ReplicaSets:              []KubernetesReplicaSet{{UID: "rs-1"}},               // Labels nil
			StatefulSets:             []KubernetesStatefulSet{{UID: "ss-1"}},              // Labels nil
			DaemonSets:               []KubernetesDaemonSet{{UID: "ds-1"}},                // Labels nil
			Services:                 []KubernetesService{{UID: "svc-1"}},                 // ExternalIPs/Ports/Selector/Labels nil
			Jobs:                     []KubernetesJob{{UID: "job-1"}},                     // Labels nil
			CronJobs:                 []KubernetesCronJob{{UID: "cj-1"}},                  // Labels nil
			Ingresses:                []KubernetesIngress{{UID: "ing-1"}},                 // Hosts/Addresses/Labels nil
			EndpointSlices:           []KubernetesEndpointSlice{{UID: "eps-1"}},           // Ports/Labels nil
			NetworkPolicies:          []KubernetesNetworkPolicy{{UID: "np-1"}},            // PolicyTypes/Labels nil
			PersistentVolumes:        []KubernetesPersistentVolume{{UID: "pv-1"}},         // AccessModes/Labels nil
			PersistentVolumeClaims:   []KubernetesPersistentVolumeClaim{{UID: "pvc-1"}},   // AccessModes/Labels nil
			StorageClasses:           []KubernetesStorageClass{{UID: "sc-1"}},             // ParameterKeys/Labels nil
			ConfigMaps:               []KubernetesConfigMap{{UID: "cm-1"}},                // DataKeys/BinaryDataKeys/Labels nil
			Secrets:                  []KubernetesSecret{{UID: "sec-1"}},                  // DataKeys/Labels nil
			ServiceAccounts:          []KubernetesServiceAccount{{UID: "sa-1"}},           // ImagePullSecrets/Labels nil
			Roles:                    []KubernetesRole{{UID: "role-1"}},                   // Labels nil
			ClusterRoles:             []KubernetesClusterRole{{UID: "crole-1"}},           // AggregationLabels/Labels nil
			RoleBindings:             []KubernetesRoleBinding{{UID: "rb-1"}},              // SubjectKinds/Labels nil
			ClusterRoleBindings:      []KubernetesClusterRoleBinding{{UID: "crb-1"}},      // SubjectKinds/Labels nil
			ResourceQuotas:           []KubernetesResourceQuota{{UID: "rq-1"}},            // Hard/Used/Labels nil
			LimitRanges:              []KubernetesLimitRange{{UID: "lr-1"}},               // LimitTypes/Labels nil
			PodDisruptionBudgets:     []KubernetesPodDisruptionBudget{{UID: "pdb-1"}},     // Labels nil
			HorizontalPodAutoscalers: []KubernetesHorizontalPodAutoscaler{{UID: "hpa-1"}}, // MetricTypes/Labels nil
			Events:                   []KubernetesEvent{{UID: "ev-1"}},                    // no NormalizeCollections; nil-check only
		}
		out := c.NormalizeCollections()

		// Each recursing slice preserved length 1 and its nested element's
		// collections were normalized (recursion side-effect observable).
		if len(out.Nodes) != 1 || out.Nodes[0].Roles == nil {
			t.Fatalf("nested KubernetesNode.Roles not normalized")
		}
		if len(out.Namespaces) != 1 || out.Namespaces[0].Labels == nil {
			t.Fatalf("nested KubernetesNamespace.Labels not normalized")
		}
		if len(out.Pods) != 1 || out.Pods[0].Labels == nil || out.Pods[0].Containers == nil {
			t.Fatalf("nested KubernetesPod collections not normalized")
		}
		if len(out.Deployments) != 1 || out.Deployments[0].Labels == nil {
			t.Fatalf("nested KubernetesDeployment.Labels not normalized")
		}
		if len(out.ReplicaSets) != 1 || out.ReplicaSets[0].Labels == nil {
			t.Fatalf("nested KubernetesReplicaSet.Labels not normalized")
		}
		if len(out.StatefulSets) != 1 || out.StatefulSets[0].Labels == nil {
			t.Fatalf("nested KubernetesStatefulSet.Labels not normalized")
		}
		if len(out.DaemonSets) != 1 || out.DaemonSets[0].Labels == nil {
			t.Fatalf("nested KubernetesDaemonSet.Labels not normalized")
		}
		if len(out.Services) != 1 || out.Services[0].ExternalIPs == nil || out.Services[0].Ports == nil ||
			out.Services[0].Selector == nil || out.Services[0].Labels == nil {
			t.Fatalf("nested KubernetesService collections not normalized")
		}
		if len(out.Jobs) != 1 || out.Jobs[0].Labels == nil {
			t.Fatalf("nested KubernetesJob.Labels not normalized")
		}
		if len(out.CronJobs) != 1 || out.CronJobs[0].Labels == nil {
			t.Fatalf("nested KubernetesCronJob.Labels not normalized")
		}
		if len(out.Ingresses) != 1 || out.Ingresses[0].Hosts == nil || out.Ingresses[0].Addresses == nil || out.Ingresses[0].Labels == nil {
			t.Fatalf("nested KubernetesIngress collections not normalized")
		}
		if len(out.EndpointSlices) != 1 || out.EndpointSlices[0].Ports == nil || out.EndpointSlices[0].Labels == nil {
			t.Fatalf("nested KubernetesEndpointSlice collections not normalized")
		}
		if len(out.NetworkPolicies) != 1 || out.NetworkPolicies[0].PolicyTypes == nil || out.NetworkPolicies[0].Labels == nil {
			t.Fatalf("nested KubernetesNetworkPolicy collections not normalized")
		}
		if len(out.PersistentVolumes) != 1 || out.PersistentVolumes[0].AccessModes == nil || out.PersistentVolumes[0].Labels == nil {
			t.Fatalf("nested KubernetesPersistentVolume collections not normalized")
		}
		if len(out.PersistentVolumeClaims) != 1 || out.PersistentVolumeClaims[0].AccessModes == nil || out.PersistentVolumeClaims[0].Labels == nil {
			t.Fatalf("nested KubernetesPersistentVolumeClaim collections not normalized")
		}
		if len(out.StorageClasses) != 1 || out.StorageClasses[0].ParameterKeys == nil || out.StorageClasses[0].Labels == nil {
			t.Fatalf("nested KubernetesStorageClass collections not normalized")
		}
		if len(out.ConfigMaps) != 1 || out.ConfigMaps[0].DataKeys == nil || out.ConfigMaps[0].BinaryDataKeys == nil || out.ConfigMaps[0].Labels == nil {
			t.Fatalf("nested KubernetesConfigMap collections not normalized")
		}
		if len(out.Secrets) != 1 || out.Secrets[0].DataKeys == nil || out.Secrets[0].Labels == nil {
			t.Fatalf("nested KubernetesSecret collections not normalized")
		}
		if len(out.ServiceAccounts) != 1 || out.ServiceAccounts[0].ImagePullSecrets == nil || out.ServiceAccounts[0].Labels == nil {
			t.Fatalf("nested KubernetesServiceAccount collections not normalized")
		}
		if len(out.Roles) != 1 || out.Roles[0].Labels == nil {
			t.Fatalf("nested KubernetesRole.Labels not normalized")
		}
		if len(out.ClusterRoles) != 1 || out.ClusterRoles[0].AggregationLabels == nil || out.ClusterRoles[0].Labels == nil {
			t.Fatalf("nested KubernetesClusterRole collections not normalized")
		}
		if len(out.RoleBindings) != 1 || out.RoleBindings[0].SubjectKinds == nil || out.RoleBindings[0].Labels == nil {
			t.Fatalf("nested KubernetesRoleBinding collections not normalized")
		}
		if len(out.ClusterRoleBindings) != 1 || out.ClusterRoleBindings[0].SubjectKinds == nil || out.ClusterRoleBindings[0].Labels == nil {
			t.Fatalf("nested KubernetesClusterRoleBinding collections not normalized")
		}
		if len(out.ResourceQuotas) != 1 || out.ResourceQuotas[0].Hard == nil || out.ResourceQuotas[0].Used == nil || out.ResourceQuotas[0].Labels == nil {
			t.Fatalf("nested KubernetesResourceQuota collections not normalized")
		}
		if len(out.LimitRanges) != 1 || out.LimitRanges[0].LimitTypes == nil || out.LimitRanges[0].Labels == nil {
			t.Fatalf("nested KubernetesLimitRange collections not normalized")
		}
		if len(out.PodDisruptionBudgets) != 1 || out.PodDisruptionBudgets[0].Labels == nil {
			t.Fatalf("nested KubernetesPodDisruptionBudget.Labels not normalized")
		}
		if len(out.HorizontalPodAutoscalers) != 1 || out.HorizontalPodAutoscalers[0].MetricTypes == nil || out.HorizontalPodAutoscalers[0].Labels == nil {
			t.Fatalf("nested KubernetesHorizontalPodAutoscaler collections not normalized")
		}
		if len(out.Events) != 1 || out.Events[0].UID != "ev-1" {
			t.Fatalf("Events not preserved (no recursion expected)")
		}
	})

	// ---------------- KubernetesNode ----------------
	t.Run("KubernetesNode_nil_Roles", func(t *testing.T) {
		n := KubernetesNode{UID: "n-1"}
		out := n.NormalizeCollections()
		if out.Roles == nil {
			t.Fatalf("Roles should be non-nil after normalize")
		}
	})
	t.Run("KubernetesNode_populated_preserved", func(t *testing.T) {
		n := KubernetesNode{UID: "n-1", Roles: []string{"control-plane", "worker", "control-plane"}}
		out := n.NormalizeCollections()
		if len(out.Roles) != 3 || out.Roles[0] != "control-plane" || out.Roles[2] != "control-plane" {
			t.Fatalf("Roles not preserved (no dedup expected): %+v", out.Roles)
		}
	})

	// ---------------- KubernetesPod ----------------
	t.Run("KubernetesPod_nil_Labels_Containers", func(t *testing.T) {
		p := KubernetesPod{UID: "p-1"}
		out := p.NormalizeCollections()
		if out.Labels == nil || out.Containers == nil {
			t.Fatalf("Labels/Containers should be non-nil after normalize")
		}
	})
	t.Run("KubernetesPod_populated_preserved", func(t *testing.T) {
		p := KubernetesPod{
			UID:        "p-1",
			Labels:     map[string]string{"app": "web"},
			Containers: []KubernetesPodContainer{{Name: "c-1"}},
		}
		out := p.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Containers) != 1 || out.Containers[0].Name != "c-1" {
			t.Fatalf("Containers not preserved: %+v", out.Containers)
		}
	})

	// ---------------- KubernetesDeployment ----------------
	t.Run("KubernetesDeployment_nil_Labels", func(t *testing.T) {
		d := KubernetesDeployment{UID: "d-1"}
		out := d.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesDeployment_populated_preserved", func(t *testing.T) {
		d := KubernetesDeployment{UID: "d-1", Labels: map[string]string{"app": "api"}}
		out := d.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "api" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesReplicaSet ----------------
	t.Run("KubernetesReplicaSet_nil_Labels", func(t *testing.T) {
		r := KubernetesReplicaSet{UID: "rs-1"}
		out := r.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesReplicaSet_populated_preserved", func(t *testing.T) {
		r := KubernetesReplicaSet{UID: "rs-1", Labels: map[string]string{"tier": "backend"}}
		out := r.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["tier"] != "backend" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesNamespace ----------------
	t.Run("KubernetesNamespace_nil_Labels", func(t *testing.T) {
		n := KubernetesNamespace{UID: "ns-1"}
		out := n.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesNamespace_populated_preserved", func(t *testing.T) {
		n := KubernetesNamespace{UID: "ns-1", Labels: map[string]string{"name": "prod"}}
		out := n.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["name"] != "prod" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesStatefulSet ----------------
	t.Run("KubernetesStatefulSet_nil_Labels", func(t *testing.T) {
		s := KubernetesStatefulSet{UID: "ss-1"}
		out := s.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesStatefulSet_populated_preserved", func(t *testing.T) {
		s := KubernetesStatefulSet{UID: "ss-1", Labels: map[string]string{"app": "db"}}
		out := s.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "db" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesDaemonSet ----------------
	t.Run("KubernetesDaemonSet_nil_Labels", func(t *testing.T) {
		d := KubernetesDaemonSet{UID: "ds-1"}
		out := d.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesDaemonSet_populated_preserved", func(t *testing.T) {
		d := KubernetesDaemonSet{UID: "ds-1", Labels: map[string]string{"app": "logs"}}
		out := d.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "logs" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesService ----------------
	t.Run("KubernetesService_nil_all_four", func(t *testing.T) {
		s := KubernetesService{UID: "svc-1"}
		out := s.NormalizeCollections()
		if out.ExternalIPs == nil || out.Ports == nil || out.Selector == nil || out.Labels == nil {
			t.Fatalf("ExternalIPs/Ports/Selector/Labels must all be non-nil after normalize")
		}
	})
	t.Run("KubernetesService_populated_preserved", func(t *testing.T) {
		s := KubernetesService{
			UID:         "svc-1",
			ExternalIPs: []string{"1.2.3.4", "1.2.3.4"},
			Ports:       []KubernetesServicePort{{Port: 443, Protocol: "tcp"}},
			Selector:    map[string]string{"app": "web"},
			Labels:      map[string]string{"app": "web"},
		}
		out := s.NormalizeCollections()
		if len(out.ExternalIPs) != 2 || out.ExternalIPs[0] != "1.2.3.4" || out.ExternalIPs[1] != "1.2.3.4" {
			t.Fatalf("ExternalIPs not preserved (no dedup expected): %+v", out.ExternalIPs)
		}
		if len(out.Ports) != 1 || out.Ports[0].Port != 443 {
			t.Fatalf("Ports not preserved: %+v", out.Ports)
		}
		if len(out.Selector) != 1 || out.Selector["app"] != "web" {
			t.Fatalf("Selector not preserved: %+v", out.Selector)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesJob ----------------
	t.Run("KubernetesJob_nil_Labels", func(t *testing.T) {
		j := KubernetesJob{UID: "job-1"}
		out := j.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesJob_populated_preserved", func(t *testing.T) {
		j := KubernetesJob{UID: "job-1", Labels: map[string]string{"batch": "true"}}
		out := j.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["batch"] != "true" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesCronJob ----------------
	t.Run("KubernetesCronJob_nil_Labels", func(t *testing.T) {
		j := KubernetesCronJob{UID: "cj-1"}
		out := j.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesCronJob_populated_preserved", func(t *testing.T) {
		j := KubernetesCronJob{UID: "cj-1", Labels: map[string]string{"schedule": "hourly"}}
		out := j.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["schedule"] != "hourly" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesIngress ----------------
	t.Run("KubernetesIngress_nil_all_three", func(t *testing.T) {
		i := KubernetesIngress{UID: "ing-1"}
		out := i.NormalizeCollections()
		if out.Hosts == nil || out.Addresses == nil || out.Labels == nil {
			t.Fatalf("Hosts/Addresses/Labels must all be non-nil after normalize")
		}
	})
	t.Run("KubernetesIngress_populated_preserved", func(t *testing.T) {
		i := KubernetesIngress{
			UID:       "ing-1",
			Hosts:     []string{"a.example.com", "a.example.com"},
			Addresses: []string{"10.0.0.1"},
			Labels:    map[string]string{"app": "edge"},
		}
		out := i.NormalizeCollections()
		if len(out.Hosts) != 2 || out.Hosts[0] != "a.example.com" || out.Hosts[1] != "a.example.com" {
			t.Fatalf("Hosts not preserved (no dedup expected): %+v", out.Hosts)
		}
		if len(out.Addresses) != 1 || out.Addresses[0] != "10.0.0.1" {
			t.Fatalf("Addresses not preserved: %+v", out.Addresses)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "edge" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesEndpointSlice ----------------
	t.Run("KubernetesEndpointSlice_nil_Ports_Labels", func(t *testing.T) {
		e := KubernetesEndpointSlice{UID: "eps-1"}
		out := e.NormalizeCollections()
		if out.Ports == nil || out.Labels == nil {
			t.Fatalf("Ports/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesEndpointSlice_populated_preserved", func(t *testing.T) {
		e := KubernetesEndpointSlice{
			UID:    "eps-1",
			Ports:  []KubernetesEndpointPort{{Port: 8080, Protocol: "tcp"}},
			Labels: map[string]string{"kubernetes.io/service-name": "web"},
		}
		out := e.NormalizeCollections()
		if len(out.Ports) != 1 || out.Ports[0].Port != 8080 {
			t.Fatalf("Ports not preserved: %+v", out.Ports)
		}
		if len(out.Labels) != 1 || out.Labels["kubernetes.io/service-name"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesNetworkPolicy ----------------
	t.Run("KubernetesNetworkPolicy_nil_PolicyTypes_Labels", func(t *testing.T) {
		n := KubernetesNetworkPolicy{UID: "np-1"}
		out := n.NormalizeCollections()
		if out.PolicyTypes == nil || out.Labels == nil {
			t.Fatalf("PolicyTypes/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesNetworkPolicy_populated_preserved", func(t *testing.T) {
		n := KubernetesNetworkPolicy{
			UID:         "np-1",
			PolicyTypes: []string{"Ingress", "Egress"},
			Labels:      map[string]string{"app": "secured"},
		}
		out := n.NormalizeCollections()
		if len(out.PolicyTypes) != 2 || out.PolicyTypes[0] != "Ingress" || out.PolicyTypes[1] != "Egress" {
			t.Fatalf("PolicyTypes not preserved: %+v", out.PolicyTypes)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "secured" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesPersistentVolume ----------------
	t.Run("KubernetesPersistentVolume_nil_AccessModes_Labels", func(t *testing.T) {
		p := KubernetesPersistentVolume{UID: "pv-1"}
		out := p.NormalizeCollections()
		if out.AccessModes == nil || out.Labels == nil {
			t.Fatalf("AccessModes/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesPersistentVolume_populated_preserved", func(t *testing.T) {
		p := KubernetesPersistentVolume{
			UID:         "pv-1",
			AccessModes: []string{"ReadWriteOnce", "ReadWriteOnce"},
			Labels:      map[string]string{"app": "data"},
		}
		out := p.NormalizeCollections()
		if len(out.AccessModes) != 2 || out.AccessModes[0] != "ReadWriteOnce" || out.AccessModes[1] != "ReadWriteOnce" {
			t.Fatalf("AccessModes not preserved (no dedup expected): %+v", out.AccessModes)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "data" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesPersistentVolumeClaim ----------------
	t.Run("KubernetesPersistentVolumeClaim_nil_AccessModes_Labels", func(t *testing.T) {
		p := KubernetesPersistentVolumeClaim{UID: "pvc-1"}
		out := p.NormalizeCollections()
		if out.AccessModes == nil || out.Labels == nil {
			t.Fatalf("AccessModes/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesPersistentVolumeClaim_populated_preserved", func(t *testing.T) {
		p := KubernetesPersistentVolumeClaim{
			UID:         "pvc-1",
			AccessModes: []string{"ReadWriteOnce"},
			Labels:      map[string]string{"app": "data"},
		}
		out := p.NormalizeCollections()
		if len(out.AccessModes) != 1 || out.AccessModes[0] != "ReadWriteOnce" {
			t.Fatalf("AccessModes not preserved: %+v", out.AccessModes)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "data" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesStorageClass ----------------
	t.Run("KubernetesStorageClass_nil_ParameterKeys_Labels", func(t *testing.T) {
		s := KubernetesStorageClass{UID: "sc-1"}
		out := s.NormalizeCollections()
		if out.ParameterKeys == nil || out.Labels == nil {
			t.Fatalf("ParameterKeys/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesStorageClass_populated_preserved", func(t *testing.T) {
		s := KubernetesStorageClass{
			UID:           "sc-1",
			ParameterKeys: []string{"type", "fsType"},
			Labels:        map[string]string{"app": "storage"},
		}
		out := s.NormalizeCollections()
		if len(out.ParameterKeys) != 2 || out.ParameterKeys[0] != "type" || out.ParameterKeys[1] != "fsType" {
			t.Fatalf("ParameterKeys not preserved: %+v", out.ParameterKeys)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "storage" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesConfigMap ----------------
	t.Run("KubernetesConfigMap_nil_all_three", func(t *testing.T) {
		c := KubernetesConfigMap{UID: "cm-1"}
		out := c.NormalizeCollections()
		if out.DataKeys == nil || out.BinaryDataKeys == nil || out.Labels == nil {
			t.Fatalf("DataKeys/BinaryDataKeys/Labels must all be non-nil after normalize")
		}
	})
	t.Run("KubernetesConfigMap_populated_preserved", func(t *testing.T) {
		c := KubernetesConfigMap{
			UID:            "cm-1",
			DataKeys:       []string{"config.yaml", "config.yaml"},
			BinaryDataKeys: []string{"cert.pem"},
			Labels:         map[string]string{"app": "conf"},
		}
		out := c.NormalizeCollections()
		if len(out.DataKeys) != 2 || out.DataKeys[0] != "config.yaml" || out.DataKeys[1] != "config.yaml" {
			t.Fatalf("DataKeys not preserved (no dedup expected): %+v", out.DataKeys)
		}
		if len(out.BinaryDataKeys) != 1 || out.BinaryDataKeys[0] != "cert.pem" {
			t.Fatalf("BinaryDataKeys not preserved: %+v", out.BinaryDataKeys)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "conf" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesSecret ----------------
	t.Run("KubernetesSecret_nil_DataKeys_Labels", func(t *testing.T) {
		s := KubernetesSecret{UID: "sec-1"}
		out := s.NormalizeCollections()
		if out.DataKeys == nil || out.Labels == nil {
			t.Fatalf("DataKeys/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesSecret_populated_preserved", func(t *testing.T) {
		s := KubernetesSecret{
			UID:      "sec-1",
			DataKeys: []string{"tls.crt", "tls.key"},
			Labels:   map[string]string{"app": "tls"},
		}
		out := s.NormalizeCollections()
		if len(out.DataKeys) != 2 || out.DataKeys[0] != "tls.crt" || out.DataKeys[1] != "tls.key" {
			t.Fatalf("DataKeys not preserved: %+v", out.DataKeys)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "tls" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesServiceAccount ----------------
	t.Run("KubernetesServiceAccount_nil_ImagePullSecrets_Labels", func(t *testing.T) {
		s := KubernetesServiceAccount{UID: "sa-1"}
		out := s.NormalizeCollections()
		if out.ImagePullSecrets == nil || out.Labels == nil {
			t.Fatalf("ImagePullSecrets/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesServiceAccount_populated_preserved", func(t *testing.T) {
		s := KubernetesServiceAccount{
			UID:              "sa-1",
			ImagePullSecrets: []string{"regcred", "regcred"},
			Labels:           map[string]string{"app": "svc"},
		}
		out := s.NormalizeCollections()
		if len(out.ImagePullSecrets) != 2 || out.ImagePullSecrets[0] != "regcred" || out.ImagePullSecrets[1] != "regcred" {
			t.Fatalf("ImagePullSecrets not preserved (no dedup expected): %+v", out.ImagePullSecrets)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "svc" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesRole ----------------
	t.Run("KubernetesRole_nil_Labels", func(t *testing.T) {
		r := KubernetesRole{UID: "role-1"}
		out := r.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesRole_populated_preserved", func(t *testing.T) {
		r := KubernetesRole{UID: "role-1", Labels: map[string]string{"app": "rbac"}}
		out := r.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "rbac" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesClusterRole ----------------
	t.Run("KubernetesClusterRole_nil_AggregationLabels_Labels", func(t *testing.T) {
		r := KubernetesClusterRole{UID: "crole-1"}
		out := r.NormalizeCollections()
		if out.AggregationLabels == nil || out.Labels == nil {
			t.Fatalf("AggregationLabels/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesClusterRole_populated_preserved", func(t *testing.T) {
		r := KubernetesClusterRole{
			UID:               "crole-1",
			AggregationLabels: map[string]string{"rbac.example.com/aggregate": "true"},
			Labels:            map[string]string{"app": "rbac"},
		}
		out := r.NormalizeCollections()
		if len(out.AggregationLabels) != 1 || out.AggregationLabels["rbac.example.com/aggregate"] != "true" {
			t.Fatalf("AggregationLabels not preserved: %+v", out.AggregationLabels)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "rbac" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesRoleBinding ----------------
	t.Run("KubernetesRoleBinding_nil_SubjectKinds_Labels", func(t *testing.T) {
		b := KubernetesRoleBinding{UID: "rb-1"}
		out := b.NormalizeCollections()
		if out.SubjectKinds == nil || out.Labels == nil {
			t.Fatalf("SubjectKinds/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesRoleBinding_populated_preserved", func(t *testing.T) {
		b := KubernetesRoleBinding{
			UID:          "rb-1",
			SubjectKinds: []string{"User", "ServiceAccount"},
			Labels:       map[string]string{"app": "rbac"},
		}
		out := b.NormalizeCollections()
		if len(out.SubjectKinds) != 2 || out.SubjectKinds[0] != "User" || out.SubjectKinds[1] != "ServiceAccount" {
			t.Fatalf("SubjectKinds not preserved: %+v", out.SubjectKinds)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "rbac" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesClusterRoleBinding ----------------
	t.Run("KubernetesClusterRoleBinding_nil_SubjectKinds_Labels", func(t *testing.T) {
		b := KubernetesClusterRoleBinding{UID: "crb-1"}
		out := b.NormalizeCollections()
		if out.SubjectKinds == nil || out.Labels == nil {
			t.Fatalf("SubjectKinds/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesClusterRoleBinding_populated_preserved", func(t *testing.T) {
		b := KubernetesClusterRoleBinding{
			UID:          "crb-1",
			SubjectKinds: []string{"Group", "Group"},
			Labels:       map[string]string{"app": "rbac"},
		}
		out := b.NormalizeCollections()
		if len(out.SubjectKinds) != 2 || out.SubjectKinds[0] != "Group" || out.SubjectKinds[1] != "Group" {
			t.Fatalf("SubjectKinds not preserved (no dedup expected): %+v", out.SubjectKinds)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "rbac" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesResourceQuota ----------------
	t.Run("KubernetesResourceQuota_nil_Hard_Used_Labels", func(t *testing.T) {
		q := KubernetesResourceQuota{UID: "rq-1"}
		out := q.NormalizeCollections()
		if out.Hard == nil || out.Used == nil || out.Labels == nil {
			t.Fatalf("Hard/Used/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesResourceQuota_populated_preserved", func(t *testing.T) {
		q := KubernetesResourceQuota{
			UID:    "rq-1",
			Hard:   map[string]string{"cpu": "10"},
			Used:   map[string]string{"cpu": "3"},
			Labels: map[string]string{"app": "quota"},
		}
		out := q.NormalizeCollections()
		if len(out.Hard) != 1 || out.Hard["cpu"] != "10" {
			t.Fatalf("Hard not preserved: %+v", out.Hard)
		}
		if len(out.Used) != 1 || out.Used["cpu"] != "3" {
			t.Fatalf("Used not preserved: %+v", out.Used)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "quota" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesLimitRange ----------------
	t.Run("KubernetesLimitRange_nil_LimitTypes_Labels", func(t *testing.T) {
		l := KubernetesLimitRange{UID: "lr-1"}
		out := l.NormalizeCollections()
		if out.LimitTypes == nil || out.Labels == nil {
			t.Fatalf("LimitTypes/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesLimitRange_populated_preserved", func(t *testing.T) {
		l := KubernetesLimitRange{
			UID:        "lr-1",
			LimitTypes: []string{"Container", "Pod"},
			Labels:     map[string]string{"app": "limits"},
		}
		out := l.NormalizeCollections()
		if len(out.LimitTypes) != 2 || out.LimitTypes[0] != "Container" || out.LimitTypes[1] != "Pod" {
			t.Fatalf("LimitTypes not preserved: %+v", out.LimitTypes)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "limits" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesPodDisruptionBudget ----------------
	t.Run("KubernetesPodDisruptionBudget_nil_Labels", func(t *testing.T) {
		p := KubernetesPodDisruptionBudget{UID: "pdb-1"}
		out := p.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesPodDisruptionBudget_populated_preserved", func(t *testing.T) {
		p := KubernetesPodDisruptionBudget{UID: "pdb-1", Labels: map[string]string{"app": "pdb"}}
		out := p.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "pdb" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- KubernetesHorizontalPodAutoscaler ----------------
	t.Run("KubernetesHorizontalPodAutoscaler_nil_MetricTypes_Labels", func(t *testing.T) {
		h := KubernetesHorizontalPodAutoscaler{UID: "hpa-1"}
		out := h.NormalizeCollections()
		if out.MetricTypes == nil || out.Labels == nil {
			t.Fatalf("MetricTypes/Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesHorizontalPodAutoscaler_populated_preserved", func(t *testing.T) {
		h := KubernetesHorizontalPodAutoscaler{
			UID:         "hpa-1",
			MetricTypes: []string{"Resource", "Pods"},
			Labels:      map[string]string{"app": "hpa"},
		}
		out := h.NormalizeCollections()
		if len(out.MetricTypes) != 2 || out.MetricTypes[0] != "Resource" || out.MetricTypes[1] != "Pods" {
			t.Fatalf("MetricTypes not preserved: %+v", out.MetricTypes)
		}
		if len(out.Labels) != 1 || out.Labels["app"] != "hpa" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})
}

// castSlice is a tiny helper that returns the supplied slice as a []struct{}
// (preserving only nil-ness) so the KubernetesCluster nil-arm assertions can
// loop over a heterogeneous set of slice fields with one uniform check. Only
// nil-ness is meaningful; the returned slice contents are not inspected.
func castSlice[T any](s []T) []struct{} {
	if s == nil {
		return nil
	}
	return make([]struct{}, len(s))
}
