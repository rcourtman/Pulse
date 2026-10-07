package monitoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
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
		id := unifiedresources.HostSMARTDiskSourceID(host, tc.disk)
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
// source ID, which already names the member behind the shared block path. The
// metrics target that chart and history reads query has to be that key, not
// the key with the member topology appended a second time, or every sample the
// agent and Proxmox writers store goes unread. A SAS member reported under its
// smartctl label merges with the Proxmox row for its block path, and the host
// agent writes that path's I/O through the merged disk's metrics target, so
// its SMART and I/O history have to share that key too.
func TestIdentitylessControllerMemberMetricsTargetReadsWriterHistory(t *testing.T) {
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	state := models.NewState()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{state: state, resourceStore: adapter, metricsStore: store, metricsHistory: NewMetricsHistory(100, time.Hour)}

	now := time.Now()
	state.UpdateNodesForInstance("pve1", []models.Node{
		{ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", LastSeen: now},
		{ID: "pve1-node2", Name: "node2", Instance: "pve1", Status: "online", LastSeen: now, LinkedAgentID: "agent-b"},
	})
	standalone := models.Host{ID: "agent-a", Hostname: "host-a", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "sda", Controller: "ctrl0", Target: "megaraid,0", Temperature: 30},
			{Device: "sda", Controller: "ctrl0", Target: "megaraid,1", Temperature: 31},
		}}}
	linked := models.Host{ID: "agent-b", Hostname: "node2", LinkedNodeID: "pve1-node2", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "sdc [megaraid,3]", Controller: "sdc", Target: "megaraid,3", Type: "sas", Temperature: 33},
		}},
		DiskIO: []models.DiskIO{{Device: "sdc", ReadBytes: 4096}},
	}
	state.UpsertHost(standalone)
	state.UpsertHost(linked)
	pveMember := models.PhysicalDisk{
		ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdb", "ctrl1", "megaraid,2"),
		Instance: "pve1", Node: "node1", DevPath: "/dev/sdb", Controller: "ctrl1", Target: "megaraid,2", Temperature: 40,
	}
	pveBlockPath := models.PhysicalDisk{
		ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node2", "/dev/sdc", "", ""),
		Instance: "pve1", Node: "node2", DevPath: "/dev/sdc", Serial: "unknown",
	}
	state.UpdatePhysicalDisks("pve1", []models.PhysicalDisk{pveMember, pveBlockPath})

	m.writeHostSMARTMetrics(standalone, now)
	m.writeHostSMARTMetrics(linked, now)
	m.writeSMARTMetrics(pveMember, now)
	store.Flush()
	adapter.PopulateFromSnapshot(state.GetSnapshot())

	want := map[string]struct {
		key         string
		temperature float64
		devPath     string
		topology    string
	}{
		"megaraid,0": {"agent-a:sda@ctrl0/megaraid,0", 30, "sda", ":sda@ctrl0/megaraid,0"},
		"megaraid,1": {"agent-a:sda@ctrl0/megaraid,1", 31, "sda", ":sda@ctrl0/megaraid,1"},
		"megaraid,2": {"pve1-node1--dev-sdb:sdb@ctrl1/megaraid,2", 40, "/dev/sdb", ":sdb@ctrl1/megaraid,2"},
		"megaraid,3": {"agent-b:sdc@sdc/megaraid,3", 33, "/dev/sdc", ":sdc@sdc/megaraid,3"},
	}
	disks := m.GetUnifiedReadStateOrSnapshot().PhysicalDisks()
	if len(disks) != len(want) {
		t.Fatalf("physical disks = %d, want %d with the SAS member merged into its Proxmox row", len(disks), len(want))
	}
	for _, disk := range disks {
		member, ok := want[disk.Target()]
		if !ok {
			t.Fatalf("unexpected disk %s at %q target %q", disk.ID(), disk.DevPath(), disk.Target())
		}
		if disk.DevPath() != member.devPath {
			t.Fatalf("%s: device path = %q, want %q", disk.Target(), disk.DevPath(), member.devPath)
		}
		target := adapter.MetricsTargetForResource(disk.ID())
		if target == nil || target.ResourceID != member.key {
			t.Fatalf("%s: metrics target = %+v, want the writer's key %q", disk.Target(), target, member.key)
		}
		points, err := store.Query("disk", target.ResourceID, "smart_temp", now.Add(-time.Minute), now.Add(time.Minute), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != 1 || points[0].Value != member.temperature {
			t.Fatalf("%s: history at the metrics target = %+v, want the %v sample its writer stored",
				disk.Target(), points, member.temperature)
		}
		// A fallback that does not name the member yet, such as the canonical
		// resource ID a view falls back to, still gets the topology.
		meta := &unifiedresources.PhysicalDiskMeta{DevPath: disk.DevPath(), Controller: disk.Controller(), Target: disk.Target()}
		if got := unifiedresources.PhysicalDiskMetaMetricID(meta, disk.ID()); got != disk.ID()+member.topology {
			t.Fatalf("%s: canonical-ID fallback key = %q, want %q", disk.Target(), got, disk.ID()+member.topology)
		}
	}

	ioKey := hostDiskIOMetricResourceID(linked, linked.DiskIO[0], m.proxmoxPhysicalDiskMatchesForLinkedNode(linked.LinkedNodeID))
	if ioKey != want["megaraid,3"].key {
		t.Fatalf("merged SAS member I/O key = %q, want its SMART history key %q", ioKey, want["megaraid,3"].key)
	}
}
