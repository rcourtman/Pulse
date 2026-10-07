package monitoring

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type issue1595TopologyFixture struct {
	Node        string `json:"node"`
	Instance    string `json:"instance"`
	AgentID     string `json:"agentId"`
	Model       string `json:"model"`
	SizeBytes   int64  `json:"sizeBytes"`
	Controllers []struct {
		ID    string `json:"id"`
		Pool  string `json:"pool"`
		Disks []struct {
			Device         string `json:"device"`
			Target         string `json:"target"`
			Serial         string `json:"serial"`
			ProviderSerial string `json:"providerSerial"`
			Temperature    int    `json:"temperature"`
			ReadBytes      uint64 `json:"readBytes"`
			WriteBytes     uint64 `json:"writeBytes"`
			IOTimeMs       uint64 `json:"ioTimeMs"`
		} `json:"disks"`
	} `json:"controllers"`
}

func loadIssue1595TopologyFixture(t *testing.T) issue1595TopologyFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "issue1595_sas_topology.json"))
	if err != nil {
		t.Fatalf("read issue #1595 fixture: %v", err)
	}
	var fixture issue1595TopologyFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode issue #1595 fixture: %v", err)
	}
	return fixture
}

func TestIssue1595SASTopologySurvivesMergeRegistryAndReadState(t *testing.T) {
	fixture := loadIssue1595TopologyFixture(t)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	nodeID := fixture.Instance + "-" + fixture.Node

	host := models.Host{
		ID:           fixture.AgentID,
		Hostname:     fixture.Node,
		Status:       "online",
		LastSeen:     now,
		LinkedNodeID: nodeID,
	}
	node := models.Node{
		ID:            nodeID,
		Name:          fixture.Node,
		Instance:      fixture.Instance,
		Status:        "online",
		LastSeen:      now,
		LinkedAgentID: fixture.AgentID,
	}

	var providerDisks []models.PhysicalDisk
	expectedBySerial := make(map[string]models.PhysicalDisk)
	for _, controller := range fixture.Controllers {
		for index, disk := range controller.Disks {
			device := "/dev/" + disk.Device
			io := &models.DiskIO{
				Device:     disk.Device,
				ReadBytes:  disk.ReadBytes,
				WriteBytes: disk.WriteBytes,
				ReadOps:    uint64(100 + index),
				WriteOps:   uint64(200 + index),
				ReadTime:   uint64(300 + index),
				WriteTime:  uint64(400 + index),
				IOTime:     disk.IOTimeMs,
			}
			host.DiskIO = append(host.DiskIO, *io)
			host.Sensors.SMART = append(host.Sensors.SMART, models.HostDiskSMART{
				Device:      device,
				Model:       fixture.Model,
				Serial:      disk.Serial,
				Type:        "sas",
				Controller:  controller.ID,
				Target:      disk.Target,
				SizeBytes:   fixture.SizeBytes,
				Temperature: disk.Temperature,
				Health:      "PASSED",
				Pool:        controller.Pool,
				IO:          io,
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("smartctl"),
					Temperature: diskinventory.Available("smartctl"),
					IO:          diskinventory.Available("linux-diskstats"),
					Controller:  diskinventory.Available("linux-sysfs"),
					Pool:        diskinventory.Available("zpool-status"),
				},
			})

			providerDisk := models.PhysicalDisk{
				ID:           fixture.Instance + "-" + fixture.Node + "-" + disk.Device,
				Node:         fixture.Node,
				Instance:     fixture.Instance,
				DevPath:      device,
				Model:        fixture.Model,
				Serial:       disk.ProviderSerial,
				Type:         "unknown",
				Size:         fixture.SizeBytes,
				Health:       "UNKNOWN",
				Wearout:      -1,
				Used:         "ZFS",
				StorageGroup: controller.Pool,
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("pve-api"),
					Temperature: diskinventory.Unsupported("pve-api", "provider does not report disk temperature"),
					IO:          diskinventory.Unsupported("pve-api", "provider does not report per-disk I/O"),
					Controller:  diskinventory.Missing("pve-api", "controller association absent"),
					Pool:        diskinventory.Available("pve-storage"),
				},
				LastChecked: now,
			}
			providerDisk.ExpectedUpdateInterval = 15 * time.Minute
			providerDisks = append(providerDisks, providerDisk)

			want := providerDisk
			want.Serial = disk.Serial
			want.Type = "sas"
			want.Controller = controller.ID
			want.Target = disk.Target
			want.Temperature = disk.Temperature
			want.Health = "PASSED"
			want.IO = io
			expectedBySerial[disk.Serial] = want
		}
	}

	merged := mergeHostAgentSMARTIntoDisks(providerDisks, []models.Node{node}, []models.Host{host})
	if len(merged) != 24 {
		t.Fatalf("merged disk count = %d, want 24", len(merged))
	}
	for _, disk := range merged {
		want, ok := expectedBySerial[disk.Serial]
		if !ok {
			t.Fatalf("provider SAS address survived as stable serial for %s: %q", disk.DevPath, disk.Serial)
		}
		assertIssue1595PhysicalDisk(t, disk, want)
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes:         []models.Node{node},
		Hosts:         []models.Host{host},
		PhysicalDisks: providerDisks,
	})

	resources := registry.ListByType(unifiedresources.ResourceTypePhysicalDisk)
	if len(resources) != 24 {
		t.Fatalf("registry physical disk count = %d, want 24 distinct disks", len(resources))
	}
	for _, resource := range resources {
		if resource.PhysicalDisk == nil {
			t.Fatalf("physical disk resource %q has no physicalDisk metadata", resource.ID)
		}
		_, ok := expectedBySerial[resource.PhysicalDisk.Serial]
		if !ok {
			t.Fatalf("registry serial = %q, want a smartctl serial", resource.PhysicalDisk.Serial)
		}
		if !hasIssue1595Source(resource.Sources, unifiedresources.SourceAgent) ||
			!hasIssue1595Source(resource.Sources, unifiedresources.SourceProxmox) {
			t.Fatalf("disk %q sources = %v, want agent and proxmox", resource.PhysicalDisk.Serial, resource.Sources)
		}
	}

	views := registry.PhysicalDisks()
	if len(views) != 24 {
		t.Fatalf("read-state physical disk count = %d, want 24", len(views))
	}
	for _, view := range views {
		want, ok := expectedBySerial[view.Serial()]
		if !ok {
			t.Fatalf("read-state serial = %q, want smartctl serial", view.Serial())
		}
		if view.MetricResourceID() != want.Serial {
			t.Fatalf("disk %q metrics target = %q, want serial-stable target", want.Serial, view.MetricResourceID())
		}
		readBack := physicalDiskFromReadStateView(view)
		if readBack.ExpectedUpdateInterval != want.ExpectedUpdateInterval {
			t.Fatalf("disk cadence lost during canonical roundtrip: %s != %s", readBack.ExpectedUpdateInterval, want.ExpectedUpdateInterval)
		}
		assertIssue1595PhysicalDisk(t, readBack, want)
		if readBack.Collection == nil ||
			readBack.Collection.Serial.State != diskinventory.FieldAvailable ||
			readBack.Collection.Temperature.State != diskinventory.FieldAvailable ||
			readBack.Collection.IO.State != diskinventory.FieldAvailable ||
			readBack.Collection.Controller.State != diskinventory.FieldAvailable ||
			readBack.Collection.Pool.State != diskinventory.FieldAvailable {
			t.Fatalf("disk %q collection status = %+v", readBack.Serial, readBack.Collection)
		}
	}
}

func TestPhysicalDiskUnavailableEvidenceIsRetainedWithoutPretendingItWasCollected(t *testing.T) {
	previous := models.PhysicalDisk{
		Serial:       "ZR5TESTA0001",
		Controller:   "0000:03:00.0",
		Target:       "6:0:0:0",
		Temperature:  30,
		StorageGroup: "tank-a",
		IO:           &models.DiskIO{Device: "sda", ReadBytes: 1000},
	}
	current := models.PhysicalDisk{
		Collection: &diskinventory.CollectionStatus{
			Serial:      diskinventory.Missing("smartctl", "serial absent from successful response"),
			Temperature: diskinventory.Unavailable("smartctl", "collection deadline exceeded"),
			IO:          diskinventory.Unsupported("controller", "per-member counters unavailable"),
			Controller:  diskinventory.Unavailable("linux-sysfs", "topology lookup failed"),
			Pool:        diskinventory.Unavailable("zpool-status", "command failed"),
		},
	}

	got := preserveUnavailablePhysicalDiskEvidence(current, previous)
	if got.Serial != previous.Serial ||
		got.Controller != previous.Controller ||
		got.Target != previous.Target ||
		got.Temperature != previous.Temperature ||
		got.StorageGroup != previous.StorageGroup ||
		got.IO == nil ||
		got.IO.ReadBytes != previous.IO.ReadBytes {
		t.Fatalf("unavailable evidence was discarded: %+v", got)
	}
	if got.Collection.Temperature.State != diskinventory.FieldUnavailable ||
		got.Collection.IO.State != diskinventory.FieldUnsupported ||
		got.Collection.Serial.State != diskinventory.FieldMissing {
		t.Fatalf("retained values concealed current collection state: %+v", got.Collection)
	}

	got.IO.ReadBytes = 2000
	if previous.IO.ReadBytes != 1000 {
		t.Fatal("retained I/O evidence aliases the previous snapshot")
	}
}

func TestTrustedSMARTSerialPromotionDoesNotRewriteSATAOrNVMeIdentity(t *testing.T) {
	for _, diskType := range []string{"sata", "nvme"} {
		t.Run(diskType, func(t *testing.T) {
			device := "/dev/sda"
			if diskType == "nvme" {
				device = "/dev/nvme0n1"
			}
			disks := []models.PhysicalDisk{{
				ID:      "provider-disk",
				Node:    "node",
				DevPath: device,
				Serial:  "PROVIDER-SERIAL",
				Type:    diskType,
			}}
			hosts := []models.Host{{
				ID: "agent",
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{{
						Device: device,
						Serial: "SMARTCTL-SERIAL",
						Type:   diskType,
						Collection: &diskinventory.CollectionStatus{
							Serial: diskinventory.Available("smartctl"),
						},
					}},
				},
			}}

			got := mergeHostAgentSMARTIntoDisks(
				disks,
				[]models.Node{{Name: "node", LinkedAgentID: "agent"}},
				hosts,
			)[0]
			if got.Serial != "PROVIDER-SERIAL" || got.Type != diskType {
				t.Fatalf("%s identity changed during SAS remediation: %+v", diskType, got)
			}
		})
	}
}

func assertIssue1595PhysicalDisk(t *testing.T, got, want models.PhysicalDisk) {
	t.Helper()
	if got.Serial != want.Serial ||
		got.Type != "sas" ||
		got.Controller != want.Controller ||
		got.Target != want.Target ||
		got.Temperature != want.Temperature ||
		got.StorageGroup != want.StorageGroup ||
		got.IO == nil ||
		got.IO.ReadBytes != want.IO.ReadBytes ||
		got.IO.WriteBytes != want.IO.WriteBytes ||
		got.IO.ReadOps != want.IO.ReadOps ||
		got.IO.WriteOps != want.IO.WriteOps ||
		got.IO.ReadTime != want.IO.ReadTime ||
		got.IO.WriteTime != want.IO.WriteTime ||
		got.IO.IOTime != want.IO.IOTime {
		t.Fatalf("disk %s lost trusted inventory data:\n got  %+v\n want %+v", got.DevPath, got, want)
	}
}

func hasIssue1595Source(sources []unifiedresources.DataSource, want unifiedresources.DataSource) bool {
	for _, source := range sources {
		if source == want {
			return true
		}
	}
	return false
}

// An agent report's SMART row may carry a temperature its collector did not
// collect this time (retained across a standby or failed probe, or kept after
// the agent's reporting lease expired). Only a temperature
// diskinventory.TemperatureCollected accepts becomes a history sample.
func TestWriteHostSMARTMetricsRecordsOnlyCollectedTemperatures(t *testing.T) {
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Monitor{metricsStore: store}

	host := models.Host{ID: "agent-1", Hostname: "node1", Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
		{Device: "sda", Serial: "COLLECTED1", Temperature: 41,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}},
		{Device: "sdb", Serial: "LEGACY1", Temperature: 38},
		{Device: "sdc", Serial: "RETAINED1", Temperature: 36,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}},
		{Device: "sdd", Serial: "SILENT1", Temperature: 44,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", models.HostAgentStoppedReportingReason)}},
		{Device: "sde", Serial: "MISSING1", Temperature: 39,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Missing("smartctl", "temperature attribute absent")}},
	}}}
	now := time.Now()
	m.writeHostSMARTMetrics(host, now)
	store.Flush()

	for _, tc := range []struct {
		disk models.HostDiskSMART
		want int
	}{
		{host.Sensors.SMART[0], 1},
		{host.Sensors.SMART[1], 1},
		{host.Sensors.SMART[2], 0},
		{host.Sensors.SMART[3], 0},
		{host.Sensors.SMART[4], 0},
	} {
		id := unifiedresources.HostSMARTDiskMetricID(host, tc.disk)
		points, err := store.Query("disk", id, "smart_temp", now.Add(-time.Minute), now.Add(time.Minute), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != tc.want {
			t.Fatalf("%s: temperature samples = %d, want %d", tc.disk.Serial, len(points), tc.want)
		}
	}
}

// A controller member without a usable serial or WWN keys its history by its
// source ID, which already names the member behind the shared block path: the
// host agent writes under HostSMARTDiskMetricID, Proxmox under
// PhysicalDiskMetricID, which returns ProxmoxPhysicalDiskSourceID. The metrics
// target a chart reads must be that key, in the live registry and in one
// rehydrated from persisted resources, not the key with the member's topology
// appended a second time, or every sample those writers store goes unread. The
// agent's I/O for a lone member is filed under the member's SMART key. A linked
// node's agent files the I/O of a member it reads no SMART for through the
// Proxmox disk's metrics target, so that I/O must share the member's SMART key.
func TestIdentitylessControllerMembersReadTheirWritersHistory(t *testing.T) {
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	state := models.NewState()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{state: state, resourceStore: adapter, metricsStore: store, rateTracker: NewRateTracker()}

	now := time.Now().UTC().Truncate(time.Second)
	collected := func() *diskinventory.CollectionStatus {
		return &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}
	}
	state.UpdateNodesForInstance("pve", []models.Node{
		{ID: "pve-node1", Name: "node1", Instance: "pve", Status: "online", LastSeen: now},
		{ID: "pve-node2", Name: "node2", Instance: "pve", Status: "online", LastSeen: now, LinkedAgentID: "agent-node2"},
	})
	standalone := models.Host{ID: "host-pve", Hostname: "standalone", MachineID: "machine-standalone", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "sdd", Controller: "ctrl0", Target: "megaraid,3", Type: "sas", Temperature: 33, Collection: collected()},
			{Device: "sdd", Controller: "ctrl0", Target: "megaraid,4", Type: "sas", Temperature: 34, Collection: collected()},
			{Device: "/dev/sde", Controller: "ctrl0", Target: "megaraid,5", Type: "sas", Temperature: 35, Collection: collected()},
		}},
		DiskIO: []models.DiskIO{{Device: "sde", ReadBytes: 1 << 20, WriteBytes: 1 << 20, IOTime: 100}},
	}
	linked := models.Host{ID: "agent-node2", Hostname: "node2", MachineID: "machine-node2", LinkedNodeID: "pve-node2", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "sdc", Controller: "ctrl1", Target: "megaraid,1", Type: "sas", Standby: true, Collection: &diskinventory.CollectionStatus{
				Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}},
		}},
		DiskIO: []models.DiskIO{{Device: "sdc", ReadBytes: 1 << 20, WriteBytes: 1 << 20, IOTime: 100}},
	}
	state.UpsertHost(standalone)
	state.UpsertHost(linked)
	pveMember := models.PhysicalDisk{
		ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve", "node1", "/dev/sdx", "", "megaraid,0"),
		Instance: "pve", Node: "node1", DevPath: "/dev/sdx", Target: "megaraid,0", Type: "sas", Temperature: 40, LastChecked: now,
	}
	pveBlockPath := models.PhysicalDisk{
		ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve", "node2", "/dev/sdc", "", ""),
		Instance: "pve", Node: "node2", DevPath: "/dev/sdc", Serial: "unknown", Type: "sas", LastChecked: now,
	}
	state.UpdatePhysicalDisks("pve", []models.PhysicalDisk{pveMember, pveBlockPath})
	adapter.PopulateFromSnapshot(state.GetSnapshot())

	hosts := []models.Host{standalone, linked}
	for _, host := range hosts {
		m.writeHostPhysicalDiskIOMetrics(host, now.Add(-30*time.Second))
	}
	for _, host := range hosts {
		for i := range host.DiskIO {
			host.DiskIO[i].ReadBytes += 1 << 20
			host.DiskIO[i].WriteBytes += 1 << 20
			host.DiskIO[i].IOTime += 1000
		}
		m.writeHostPhysicalDiskIOMetrics(host, now)
		m.writeHostSMARTMetrics(host, now)
	}
	m.writeSMARTMetrics(pveMember, now)
	store.Flush()

	smartTemp := []string{"smart_temp"}
	diskIO := []string{"diskread", "diskwrite", "disk"}
	want := map[string]struct {
		key     string
		metrics []string
	}{
		"megaraid,3": {unifiedresources.HostSMARTDiskMetricID(standalone, standalone.Sensors.SMART[0]), smartTemp},
		"megaraid,4": {unifiedresources.HostSMARTDiskMetricID(standalone, standalone.Sensors.SMART[1]), smartTemp},
		"megaraid,5": {unifiedresources.HostSMARTDiskMetricID(standalone, standalone.Sensors.SMART[2]), append(smartTemp, diskIO...)},
		"megaraid,0": {unifiedresources.PhysicalDiskMetricID(pveMember), smartTemp},
		"megaraid,1": {unifiedresources.HostSMARTDiskMetricID(linked, linked.Sensors.SMART[0]), diskIO},
	}
	for target, member := range map[string]string{
		"megaraid,3": "host-pve:sdd@ctrl0/megaraid,3",
		"megaraid,5": "host-pve:sde@ctrl0/megaraid,5",
		"megaraid,0": "pve-node1--dev-sdx:sdx@/megaraid,0",
		"megaraid,1": "agent-node2:sdc@ctrl1/megaraid,1",
	} {
		if want[target].key != member {
			t.Fatalf("%s: writer key = %q, want the member scoped once %q", target, want[target].key, member)
		}
	}

	live := unifiedresources.NewRegistry(nil)
	live.IngestSnapshot(state.GetSnapshot())
	payload, err := json.Marshal(live.List())
	if err != nil {
		t.Fatal(err)
	}
	var persisted []unifiedresources.Resource
	if err := json.Unmarshal(payload, &persisted); err != nil {
		t.Fatal(err)
	}
	rehydrated := unifiedresources.NewRegistry(nil)
	rehydrated.IngestResources(persisted)
	for _, tc := range []struct {
		name     string
		registry *unifiedresources.ResourceRegistry
	}{{"live", live}, {"rehydrated", rehydrated}} {
		disks := tc.registry.ListByType(unifiedresources.ResourceTypePhysicalDisk)
		if len(disks) != len(want) {
			t.Fatalf("%s: disks = %d, want %d with the standby member merged into its Proxmox row", tc.name, len(disks), len(want))
		}
		for _, disk := range disks {
			member, ok := want[disk.PhysicalDisk.Target]
			if !ok {
				t.Fatalf("%s: unexpected disk %s at %q target %q", tc.name, disk.ID, disk.PhysicalDisk.DevPath, disk.PhysicalDisk.Target)
			}
			target := tc.registry.MetricsTarget(disk.ID)
			if target == nil || target.ResourceType != "disk" || target.ResourceID != member.key {
				t.Errorf("%s: %s metrics target = %+v, want the writer's disk/%s", tc.name, disk.PhysicalDisk.Target, target, member.key)
				continue
			}
			for _, metric := range member.metrics {
				points, err := store.Query(target.ResourceType, target.ResourceID, metric, now.Add(-time.Minute), now.Add(time.Minute), 0)
				if err != nil {
					t.Fatal(err)
				}
				if len(points) == 0 {
					t.Errorf("%s: %s metrics target %s reads no %s history", tc.name, disk.PhysicalDisk.Target, target.ResourceID, metric)
				}
			}
		}
	}
}

// A dual-ported SAS shelf, cloned VMs with an explicit serial and fixed-serial
// USB bridges report one usable serial on several hosts. The registry keeps a
// disk per host, and each must keep its own agent source target, in the live
// registry and in one rehydrated from its resources, or a disk only the agent
// reports has no metrics target and no chart. Both targets read the series the
// agent's SMART writer files under the serial: disk history follows the drive's
// hardware identity, not the host.
func TestSameSerialAgentDisksOnTwoHostsEachKeepTheirMetricsTarget(t *testing.T) {
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Monitor{metricsStore: store}

	now := time.Now().UTC().Truncate(time.Second)
	const serial = "SHELF-SERIAL-1"
	hosts := []models.Host{
		{ID: "agent-alpha", Hostname: "alpha", MachineID: "machine-alpha", Status: "online", LastSeen: now},
		{ID: "agent-beta", Hostname: "beta", MachineID: "machine-beta", Status: "online", LastSeen: now},
	}
	wantSourceIDs := make(map[string]bool, len(hosts))
	for i := range hosts {
		hosts[i].Sensors.SMART = []models.HostDiskSMART{{Device: "sda", Serial: serial, Model: "Shelf Disk", Type: "sas",
			Health: "PASSED", Temperature: 40 + i, Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}}}
		m.writeHostSMARTMetrics(hosts[i], now)
		wantSourceIDs[unifiedresources.HostSMARTDiskSourceID(hosts[i], hosts[i].Sensors.SMART[0])] = true
	}
	store.Flush()
	if len(wantSourceIDs) != len(hosts) {
		t.Fatalf("agent disk source IDs = %v, want one per host", wantSourceIDs)
	}

	live := unifiedresources.NewRegistry(nil)
	live.IngestSnapshot(models.StateSnapshot{Hosts: hosts})
	rehydrated := unifiedresources.NewRegistry(nil)
	rehydrated.IngestResources(live.List())
	for _, tc := range []struct {
		name     string
		registry *unifiedresources.ResourceRegistry
	}{{"live", live}, {"rehydrated", rehydrated}} {
		disks := tc.registry.ListByType(unifiedresources.ResourceTypePhysicalDisk)
		if len(disks) != len(hosts) {
			t.Fatalf("%s: disks = %d, want one per host", tc.name, len(disks))
		}
		gotSourceIDs := make(map[string]bool, len(disks))
		for _, disk := range disks {
			for _, target := range tc.registry.SourceTargets(disk.ID) {
				if target.Source == unifiedresources.SourceAgent {
					gotSourceIDs[target.SourceID] = true
				}
			}
			target := tc.registry.MetricsTarget(disk.ID)
			if target == nil {
				t.Fatalf("%s: disk %s has no metrics target", tc.name, disk.ID)
			}
			if target.ResourceType != "disk" || target.ResourceID != serial {
				t.Fatalf("%s: disk %s metrics target = %+v, want the serial's disk series", tc.name, disk.ID, *target)
			}
			points, err := store.Query(target.ResourceType, target.ResourceID, "smart_temp", now.Add(-time.Minute), now.Add(time.Minute), 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(points) == 0 {
				t.Fatalf("%s: disk %s metrics target reads no SMART temperature history", tc.name, disk.ID)
			}
		}
		if len(gotSourceIDs) != len(wantSourceIDs) {
			t.Fatalf("%s: agent source targets = %v, want each host's own %v", tc.name, gotSourceIDs, wantSourceIDs)
		}
		for sourceID := range wantSourceIDs {
			if !gotSourceIDs[sourceID] {
				t.Fatalf("%s: agent source targets = %v, want each host's own %v", tc.name, gotSourceIDs, wantSourceIDs)
			}
		}
	}
}

// An Unraid host reports each array disk twice: as a SMART row and as an
// Unraid inventory row. When the SMART row carries no serial, smartctl's
// standby row among them, the disk takes the serial its Unraid row reports,
// and its metrics target reads that serial. The SMART and disk I/O writers must
// file under it too, as must the I/O writer for a disk with no SMART row,
// or the disk's temperature and I/O history never chart.
func TestAgentDiskHistoryFollowsTheSerialItsUnraidRowReports(t *testing.T) {
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &Monitor{metricsStore: store, rateTracker: NewRateTracker()}

	now := time.Now().UTC().Truncate(time.Second)
	collected := func() *diskinventory.CollectionStatus {
		return &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}
	}
	host := models.Host{ID: "host-tower", Hostname: "tower", MachineID: "machine-tower", Status: "online", LastSeen: now}
	host.Sensors.SMART = []models.HostDiskSMART{
		{Device: "sdb", Temperature: 40, Collection: collected()},
		{Device: "/dev/sdc", WWN: "0x5000c500aaaa0001", Temperature: 41, Collection: collected()},
		{Device: "sdd", Standby: true, Collection: &diskinventory.CollectionStatus{
			Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}},
	}
	host.Unraid = &models.HostUnraidStorage{ArrayStarted: true, Disks: []models.HostUnraidDisk{
		{Device: "sdb", Serial: "UNRAID-SERIAL-B", Name: "disk1", Role: "data", Status: "online"},
		{Device: "sdc", Serial: "UNRAID-SERIAL-C", Name: "disk2", Role: "data", Status: "online"},
		{Device: "sdd", Serial: "UNRAID-SERIAL-D", Name: "disk3", Role: "data", Status: "online", SpunDown: true},
		{Device: "sde", Serial: "UNRAID-SERIAL-E", Name: "disk4", Role: "data", Status: "online"},
		{Device: "sdf", Name: "disk5", Role: "data", Status: "online"},
	}}
	want := map[string][]string{
		"sdb":      {"smart_temp", "diskread", "diskwrite", "disk"},
		"/dev/sdc": {"smart_temp", "diskread", "diskwrite", "disk"},
		"sdd":      {"diskread", "diskwrite", "disk"},
		"sde":      {"diskread", "diskwrite", "disk"},
		"sdf":      {"diskread", "diskwrite", "disk"},
	}
	for _, device := range []string{"sdb", "sdc", "sdd", "sde", "sdf"} {
		host.DiskIO = append(host.DiskIO, models.DiskIO{Device: device, ReadBytes: 1 << 20, WriteBytes: 1 << 20, IOTime: 100})
	}
	m.writeHostPhysicalDiskIOMetrics(host, now.Add(-30*time.Second))
	for i := range host.DiskIO {
		host.DiskIO[i].ReadBytes += 1 << 20
		host.DiskIO[i].WriteBytes += 1 << 20
		host.DiskIO[i].IOTime += 1000
	}
	m.writeHostPhysicalDiskIOMetrics(host, now)
	m.writeHostSMARTMetrics(host, now)
	store.Flush()

	live := unifiedresources.NewRegistry(nil)
	live.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{host}})
	rehydrated := unifiedresources.NewRegistry(nil)
	rehydrated.IngestResources(live.List())
	for _, tc := range []struct {
		name     string
		registry *unifiedresources.ResourceRegistry
	}{{"live", live}, {"rehydrated", rehydrated}} {
		disks := tc.registry.ListByType(unifiedresources.ResourceTypePhysicalDisk)
		if len(disks) != len(want) {
			t.Fatalf("%s: disks = %d, want %d", tc.name, len(disks), len(want))
		}
		for _, disk := range disks {
			metricNames, ok := want[disk.PhysicalDisk.DevPath]
			if !ok {
				t.Fatalf("%s: unexpected disk %s at %q", tc.name, disk.ID, disk.PhysicalDisk.DevPath)
			}
			target := tc.registry.MetricsTarget(disk.ID)
			if target == nil {
				t.Fatalf("%s: disk %s has no metrics target", tc.name, disk.PhysicalDisk.DevPath)
			}
			for _, metric := range metricNames {
				points, err := store.Query(target.ResourceType, target.ResourceID, metric, now.Add(-time.Minute), now.Add(time.Minute), 0)
				if err != nil {
					t.Fatal(err)
				}
				if len(points) == 0 {
					t.Errorf("%s: disk %s metrics target %+v reads no %s history", tc.name, disk.PhysicalDisk.DevPath, *target, metric)
				}
			}
		}
	}
}

// A SMART row found only by device path may describe the slot's previous
// occupant. It is refused when its identity contradicts the Proxmox disk's,
// judged only on what both producers report like for like: WWNs for every
// disk, serials only where Proxmox's serial is the drive's own, for NVMe and
// for disks the kernel reaches through libata (vendor "ATA"). A SAS disk's
// Proxmox serial may be a transport address and a USB bridge's is its own.
// Reporter spellings of one identity (udev's underscores, a T10 designator
// ending with the drive serial, the agent's NAA fields, udev's 64-bit NAA 6
// prefix) are not disagreement.
func TestHostAgentSMARTMergeRefusesPathMatchAcrossContradictingIdentity(t *testing.T) {
	type mergeCase struct {
		name  string
		disk  models.PhysicalDisk
		smart models.HostDiskSMART
		// others are further rows on the same path, which make it ambiguous.
		others []models.HostDiskSMART
		merged bool
		serial string
	}
	smartctlSerial := &diskinventory.CollectionStatus{Serial: diskinventory.Available("smartctl")}
	pveDisk := func(device, vendor, serial, wwn, diskType string) models.PhysicalDisk {
		return models.PhysicalDisk{
			ID: "pve1-node1-" + device, Node: "node1", Instance: "pve1", DevPath: "/dev/" + device,
			Vendor: vendor, Serial: serial, WWN: wwn, Type: diskType, Health: "PASSED",
		}
	}
	smartRow := func(device, serial, wwn, diskType string) models.HostDiskSMART {
		return models.HostDiskSMART{
			Device: "/dev/" + device, Serial: serial, WWN: wwn, Type: diskType,
			Health: "FAILED", Temperature: 47, Collection: diskinventory.CloneStatus(smartctlSerial),
		}
	}
	for _, tc := range []mergeCase{
		{
			name:  "SAS replacement beside a stale row with another WWN",
			disk:  pveDisk("sda", "SEAGATE", "5000c500b0000001", "0x5000c500b0000001", "unknown"),
			smart: smartRow("sda", "ZR5OLD0001", "naa.5000c500a0000001", "sas"),
		},
		{
			name:  "SATA replacement beside a stale row with another serial",
			disk:  pveDisk("sdb", "ATA", "WD-NEW0001", "", "hdd"),
			smart: smartRow("sdb", "WD-OLD0001", "", "sata"),
		},
		{
			name:   "same SAS disk whose agent row has no WWN",
			disk:   pveDisk("sdc", "SEAGATE", "5000c500a0000003", "0x5000c500a0000003", "unknown"),
			smart:  smartRow("sdc", "ZR5TESTA0003", "", "sas"),
			merged: true, serial: "ZR5TESTA0003",
		},
		{
			name:   "same SAS disk renamed, agent WWN in smartctl NAA fields",
			disk:   pveDisk("sdd", "SEAGATE", "5000c500a0000004", "0x5000c500a0000004", "unknown"),
			smart:  smartRow("sdx", "ZR5TESTA0004", "5-c50-a0000004", "sas"),
			merged: true, serial: "ZR5TESTA0004",
		},
		{
			name:   "same disk with udev's spelling of the serial",
			disk:   pveDisk("sde", "ATA", "WD-WX_1234", "", "hdd"),
			smart:  smartRow("sde", "WD-WX 1234", "", "sata"),
			merged: true, serial: "WD-WX_1234",
		},
		{
			name:  "NVMe replacement whose serial looks like a SAS address",
			disk:  pveDisk("nvme0n1", "", "50026B7282A0FB69", "", "nvme"),
			smart: smartRow("nvme0n1", "50026B7282A0FB11", "", "nvme"),
		},
		{
			name:  "ambiguous path where only one row contradicts",
			disk:  pveDisk("sdg", "ATA", "WD-NEW0007", "", "hdd"),
			smart: smartRow("sdg", "WD-OLD0007", "", "sata"),
			others: []models.HostDiskSMART{
				smartRow("sdg", "", "", "sata"),
			},
		},
		{
			name:   "same disk in a USB enclosure whose serial udev reports",
			disk:   pveDisk("sdh", "JMicron", "000000000000000F1E2D", "", "usb"),
			smart:  smartRow("sdh", "WD-WX0008", "", "sata"),
			merged: true, serial: "000000000000000F1E2D",
		},
		{
			name:   "same SSD in a USB enclosure Proxmox types ssd",
			disk:   pveDisk("sdj", "ASMT", "00000000000000000009", "", "ssd"),
			smart:  smartRow("sdj", "S3Z9NB0K123456X", "", "sata"),
			merged: true, serial: "00000000000000000009",
		},
		{
			name:   "same disk behind a T10 designator ending with its serial",
			disk:   pveDisk("sdi", "ATA", "ATA_ST4000NM000A-2HZ_ZC1ABCDE", "", "unknown"),
			smart:  smartRow("sdi", "ZC1ABCDE", "", "sata"),
			merged: true, serial: "ATA_ST4000NM000A-2HZ_ZC1ABCDE",
		},
		{
			name:   "same NAA 6 volume with udev's 64-bit WWN",
			disk:   pveDisk("sdf", "HP", "PDNLH0BRH8V0AB", "0x600508b1001c4d5e", "hdd"),
			smart:  smartRow("sdf", "", "naa.600508b1001c4d5e7f8a9b0c1d2e3f40", "scsi"),
			merged: true, serial: "PDNLH0BRH8V0AB",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeHostAgentSMARTIntoDisks(
				[]models.PhysicalDisk{tc.disk},
				[]models.Node{{Name: "node1", LinkedAgentID: "agent-1"}},
				[]models.Host{{ID: "agent-1", Sensors: models.HostSensorSummary{
					SMART: append([]models.HostDiskSMART{tc.smart}, tc.others...),
				}}},
			)[0]
			if !tc.merged {
				if got.Serial != tc.disk.Serial || got.WWN != tc.disk.WWN || got.Type != tc.disk.Type ||
					got.Temperature != 0 || got.Health != "PASSED" {
					t.Fatalf("contradicting row was merged into the disk: %+v", got)
				}
				return
			}
			if got.Serial != tc.serial || got.Temperature != 47 || got.Health != "FAILED" {
				t.Fatalf("same disk's row was not merged: serial %q (want %q), temperature %d, health %q",
					got.Serial, tc.serial, got.Temperature, got.Health)
			}
		})
	}
}

// Proxmox's disk inventory reports a serial or WWN udev cannot read as the
// literal "unknown". The poller records it as unreported rather than as a
// collected serial; the disk's identity keys were already blind to it.
func TestProxmoxUnknownDiskIdentityIsRecordedAsUnreported(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	state := models.NewState()
	state.UpdateNodesForInstance("pve1", []models.Node{{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", LastSeen: time.Now(),
	}})
	alertManager := alerts.NewManager()
	t.Cleanup(alertManager.Stop)
	m := &Monitor{
		state: state, alertManager: alertManager,
		startTime: time.Now().Add(-time.Hour), lastPhysicalDiskPoll: make(map[string]time.Time),
	}
	// A record from before the placeholder was recorded as unreported must
	// not carry it back in as the disk's last known serial.
	state.UpdatePhysicalDisks("pve1", []models.PhysicalDisk{{
		ID:   unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdb", "", ""),
		Node: "node1", Instance: "pve1", DevPath: "/dev/sdb", Serial: "unknown", WWN: "unknown",
		Collection: &diskinventory.CollectionStatus{Serial: diskinventory.Available("proxmox_disks")},
	}})
	client := &slotDiskPVEClient{}
	client.setDisk(proxmox.Disk{
		DevPath: "/dev/sdb", Model: "USB3.0 Bridge", Serial: "unknown", WWN: "unknown",
		Type: "hdd", Health: "PASSED", Wearout: 100, Size: 1000204886016,
	})
	started := time.Now()
	m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, client,
		[]proxmox.Node{{Node: "node1", Status: "online"}}, map[string]string{"node1": "online"}, nil)
	deadline := started.Add(3 * time.Second)
	for disks := state.GetSnapshot().PhysicalDisks; len(disks) != 1 || disks[0].LastChecked.Before(started); disks = state.GetSnapshot().PhysicalDisks {
		if time.Now().After(deadline) {
			t.Fatal("physical disk poll did not land in state")
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := state.GetSnapshot().PhysicalDisks[0]
	if got.Serial != "" || got.WWN != "" {
		t.Fatalf("placeholder identity recorded as reported: serial %q, wwn %q", got.Serial, got.WWN)
	}
	if got.Collection == nil || got.Collection.Serial.State != diskinventory.FieldMissing {
		t.Fatalf("serial collection = %+v, want missing", got.Collection)
	}
	placeholder := got
	placeholder.Serial, placeholder.WWN = "unknown", "unknown"
	if unifiedresources.PhysicalDiskMetricID(got) != unifiedresources.PhysicalDiskMetricID(placeholder) {
		t.Fatalf("metric key moved: %q, was %q",
			unifiedresources.PhysicalDiskMetricID(got), unifiedresources.PhysicalDiskMetricID(placeholder))
	}
}

// The registry joins a Proxmox disk to its linked agent's disk on the same
// path when the agent reports SAS, because Proxmox puts the SAS address in
// the serial (#1595). A WWN naming another disk refuses that join: the agent's
// row may be the slot's previous occupant, retained by a silent agent, and the
// join used to hand the replacement that disk's serial, readings and canonical
// resource.
func TestRegistrySASPathJoinRefusesContradictingWWN(t *testing.T) {
	const replacementWWN = "0x5000c500bbbb0002"
	for _, tc := range []struct {
		name     string
		agentWWN string
		joined   bool
	}{
		{name: "retained row of the previous occupant", agentWWN: "5-c50-aaaa0001"},
		{name: "same disk in the agent's WWN spelling", agentWWN: "5-c50-bbbb0002", joined: true},
		{name: "same disk without an agent WWN", agentWWN: "", joined: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			node := models.Node{
				ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
				LastSeen: now, LinkedAgentID: "agent-1",
			}
			host := models.Host{
				ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1", Status: "online", LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
					Device: "/dev/sdb", Serial: "ZR5AGENT0001", WWN: tc.agentWWN, Type: "sas",
					Health: "FAILED", Temperature: 52,
					Collection: &diskinventory.CollectionStatus{
						Serial:      diskinventory.Available("smartctl"),
						Temperature: diskinventory.Available("smartctl"),
					},
				}}},
			}
			replacement := models.PhysicalDisk{
				ID: "pve1-node1--dev-sdb", Node: "node1", Instance: "pve1", DevPath: "/dev/sdb",
				Serial: strings.TrimPrefix(replacementWWN, "0x"), WWN: replacementWWN, Type: "unknown",
				Health: "PASSED", Wearout: -1, LastChecked: now,
			}
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(models.StateSnapshot{
				Nodes: []models.Node{node}, Hosts: []models.Host{host}, PhysicalDisks: []models.PhysicalDisk{replacement},
			})

			var proxmoxDisk *unifiedresources.Resource
			disks := registry.ListByType(unifiedresources.ResourceTypePhysicalDisk)
			for index := range disks {
				if hasIssue1595Source(disks[index].Sources, unifiedresources.SourceProxmox) {
					proxmoxDisk = &disks[index]
				}
			}
			if proxmoxDisk == nil || proxmoxDisk.PhysicalDisk == nil {
				t.Fatalf("no canonical Proxmox disk among %d disks", len(disks))
			}
			joined := hasIssue1595Source(proxmoxDisk.Sources, unifiedresources.SourceAgent)
			if joined != tc.joined || (tc.joined && len(disks) != 1) || (!tc.joined && len(disks) != 2) {
				t.Fatalf("joined = %v with %d disks, want joined = %v", joined, len(disks), tc.joined)
			}
			if !tc.joined && (proxmoxDisk.PhysicalDisk.Serial == "ZR5AGENT0001" ||
				proxmoxDisk.PhysicalDisk.Temperature != 0 || proxmoxDisk.PhysicalDisk.Health == "FAILED") {
				t.Fatalf("replacement took the retained row's identity or readings: %+v", proxmoxDisk.PhysicalDisk)
			}
		})
	}
}
