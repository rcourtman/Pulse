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

// A registry rehydrated from a persisted unified snapshot seeds each source
// key's mapping before the next poll arrives. Proxmox keys a disk by its slot,
// so a disk swapped into the slot arrives under the previous disk's key. The
// mapping is refused when the two carry hardware identities naming different
// disks, and nothing else may merge the replacement back into the old disk,
// so it gets its own canonical resource instead of the old disk's ID, serial,
// temperature and failed health. Identity missing on one side keeps the
// mapping, and so does a serial that may not be the drive's own (a SAS
// address, a SCSI designator, a USB bridge's serial) set against the agent's.
func TestRegistrySeededSlotMappingRefusesReplacedDisk(t *testing.T) {
	const slotSourceID = "pve1-node1--dev-sdb"
	// Each kind is the slot's previous disk as Proxmox and the agent reported
	// it; the registry merged the two reports by WWN or by path, unless the
	// case has Proxmox alone report it.
	kinds := map[string]struct{ agentType, vendor, proxmoxType, proxmoxSerial string }{
		"sata":    {agentType: "sata", vendor: "ATA", proxmoxType: "hdd", proxmoxSerial: "ZR5A0001"},
		"sas":     {agentType: "sas", vendor: "SEAGATE", proxmoxType: "hdd", proxmoxSerial: "5000c500aaaa0003"},
		"usb":     {agentType: "usb", vendor: "JMicron", proxmoxType: "usb", proxmoxSerial: "BRIDGE0001"},
		"scsi":    {agentType: "sata", vendor: "SEAGATE", proxmoxType: "hdd", proxmoxSerial: "Z1Z0VPD0001"},
		"usb-ata": {agentType: "sata", vendor: "ATA", proxmoxType: "usb", proxmoxSerial: "BRIDGE0001"},
	}
	for _, tc := range []struct {
		name          string
		kind          string
		proxmoxFirst  bool
		proxmoxOnly   bool
		legacyAgent   bool
		serial        string
		wwn           string
		replacedDrive bool
	}{
		{name: "replacement disk", kind: "sata", serial: "ZR5B0002", wwn: "0x5000c500bbbb0002", replacedDrive: true},
		{name: "replacement disk without a WWN", kind: "sata", serial: "ZR5B0002", wwn: "unknown", replacedDrive: true},
		{name: "same disk", kind: "sata", serial: "ZR5A0001", wwn: "0x5000c500aaaa0001"},
		{name: "same disk without a serial", kind: "sata", serial: "unknown", wwn: "0x5000c500aaaa0001"},
		{name: "SAS disk whose Proxmox serial is a SAS address", kind: "sas", serial: "5000c500aaaa0003", wwn: "0x5000c500aaaa0001"},
		{name: "SAS disk without a Proxmox WWN", kind: "sas", serial: "5000c500aaaa0003", wwn: "unknown"},
		{name: "USB disk whose Proxmox serial is the bridge's", kind: "usb", serial: "BRIDGE0001", wwn: "unknown"},
		{name: "USB bridge serial under Proxmox vendor ATA", kind: "usb-ata", serial: "BRIDGE0001", wwn: "unknown"},
		{name: "disk whose Proxmox serial is a SCSI designator", kind: "scsi", serial: "Z1Z0VPD0001", wwn: "unknown"},
		{name: "SAS replacement disk", kind: "sas", serial: "5000c500bbbb0003", wwn: "0x5000c500bbbb0002", replacedDrive: true},
		{name: "drive swapped in the same USB enclosure", kind: "usb", proxmoxFirst: true,
			serial: "BRIDGE0001", wwn: "0x5000c500bbbb0002", replacedDrive: true},
		{name: "SCSI designator from an agent without collection status", kind: "scsi", legacyAgent: true,
			serial: "Z1Z0VPD0001", wwn: "unknown"},
		{name: "replacement Proxmox alone reports, without a WWN", kind: "scsi", proxmoxOnly: true,
			serial: "Z1Z0VPD0002", wwn: "unknown", replacedDrive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind := kinds[tc.kind]
			now := time.Now()
			node := models.Node{
				ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
				LastSeen: now, LinkedAgentID: "agent-1",
			}
			host := models.Host{
				ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1", Status: "online", LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
					Device: "/dev/sdb", Serial: "ZR5A0001", WWN: "5-c50-aaaa0001", Type: kind.agentType,
					Health: "FAILED", Temperature: 55,
					Collection: &diskinventory.CollectionStatus{
						Serial:      diskinventory.Available("smartctl"),
						Temperature: diskinventory.Available("smartctl"),
					},
				}}},
			}
			// As monitor_pve builds the slot's row from disks/list.
			slotDisk := func(serial, wwn, health string) models.PhysicalDisk {
				disk := models.PhysicalDisk{
					ID: slotSourceID, Node: "node1", Instance: "pve1", DevPath: "/dev/sdb", Vendor: kind.vendor,
					Serial: proxmoxReportedDiskIdentity(serial), WWN: proxmoxReportedDiskIdentity(wwn),
					Type: kind.proxmoxType, Health: health, Wearout: -1, LastChecked: now,
					Collection: &diskinventory.CollectionStatus{
						Serial: diskinventory.Missing("proxmox_disks", "disk serial was not reported"),
					},
				}
				if disk.Serial != "" {
					disk.Collection.Serial = diskinventory.Available("proxmox_disks")
				}
				return disk
			}
			previousDisk := slotDisk(kind.proxmoxSerial, "0x5000c500aaaa0001", "FAILED")
			previous := unifiedresources.NewRegistry(nil)
			if tc.proxmoxFirst {
				previous.IngestSnapshot(models.StateSnapshot{
					Nodes: []models.Node{node}, PhysicalDisks: []models.PhysicalDisk{previousDisk},
				})
			}
			hosts := []models.Host{host}
			if tc.proxmoxOnly {
				hosts = nil
			}
			if tc.legacyAgent {
				host.Sensors.SMART[0].Collection = nil
			}
			previous.IngestSnapshot(models.StateSnapshot{
				Nodes: []models.Node{node}, Hosts: hosts, PhysicalDisks: []models.PhysicalDisk{previousDisk},
			})
			persisted := previous.ListByType(unifiedresources.ResourceTypePhysicalDisk)
			if len(persisted) != 1 || persisted[0].PhysicalDisk == nil ||
				!hasIssue1595Source(persisted[0].Sources, unifiedresources.SourceProxmox) ||
				hasIssue1595Source(persisted[0].Sources, unifiedresources.SourceAgent) == tc.proxmoxOnly {
				t.Fatalf("want one disk merged from Proxmox and the agent (Proxmox alone: %v), got %+v", tc.proxmoxOnly, persisted)
			}
			oldID, oldDisk := persisted[0].ID, *persisted[0].PhysicalDisk

			// Restart: rehydrate from the persisted resources, then poll
			// Proxmox before the agent has reported again. A second restart
			// rehydrates from the result, where both disks may claim the slot.
			current := slotDisk(tc.serial, tc.wwn, "PASSED")
			seed := previous.List()
			slotID := ""
			for restart := 1; restart <= 2; restart++ {
				rehydrated := unifiedresources.NewRegistry(nil)
				rehydrated.IngestResources(seed)
				rehydrated.IngestSnapshot(models.StateSnapshot{
					Nodes: []models.Node{node}, PhysicalDisks: []models.PhysicalDisk{current},
				})
				seed = rehydrated.List()

				var slot *unifiedresources.Resource
				disks := rehydrated.ListByType(unifiedresources.ResourceTypePhysicalDisk)
				for index := range disks {
					for _, target := range rehydrated.SourceTargets(disks[index].ID) {
						if target.Source == unifiedresources.SourceProxmox && target.SourceID == slotSourceID {
							slot = &disks[index]
						}
					}
				}
				if slot == nil || slot.PhysicalDisk == nil {
					t.Fatalf("restart %d: no disk holds the Proxmox slot key among %d disks", restart, len(disks))
				}
				if slotID != "" && slot.ID != slotID {
					t.Fatalf("restart %d: slot moved from %s to %s", restart, slotID, slot.ID)
				}
				slotID = slot.ID
				if !tc.replacedDrive {
					if slot.ID != oldID || len(disks) != 1 {
						t.Fatalf("restart %d: slot disk = %s with %d disks, want the persisted disk %s alone",
							restart, slot.ID, len(disks), oldID)
					}
					continue
				}
				if slot.ID == oldID || len(disks) != 2 {
					t.Fatalf("restart %d: replacement took the persisted disk's canonical ID %s (%d disks)", restart, oldID, len(disks))
				}
				if slot.PhysicalDisk.Serial != current.Serial || slot.PhysicalDisk.WWN != current.WWN ||
					slot.PhysicalDisk.Temperature != 0 || slot.PhysicalDisk.Health != "PASSED" {
					t.Fatalf("restart %d: replacement took the persisted disk's identity or readings: %+v", restart, slot.PhysicalDisk)
				}
				for _, disk := range disks {
					if disk.ID == oldID && (disk.PhysicalDisk == nil ||
						disk.PhysicalDisk.Serial != oldDisk.Serial || disk.PhysicalDisk.WWN != oldDisk.WWN) {
						t.Fatalf("restart %d: persisted disk was rewritten: %+v", restart, disk.PhysicalDisk)
					}
				}
			}
		})
	}
}
