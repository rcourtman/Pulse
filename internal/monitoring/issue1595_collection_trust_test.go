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
