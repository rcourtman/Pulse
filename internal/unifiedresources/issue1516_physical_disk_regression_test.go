package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIssue1516SeededControllerMembersKeepStableSourceIDsAcrossRestart(t *testing.T) {
	parentID := "host-canonical"
	seeded := []Resource{
		{
			ID:      parentID,
			Type:    ResourceTypeAgent,
			Name:    "pve-wide",
			Sources: []DataSource{SourceAgent},
			Agent:   &AgentData{AgentID: "host-1", Hostname: "pve-wide"},
		},
		{
			ID:       "disk-member-1",
			Type:     ResourceTypePhysicalDisk,
			Name:     "member-1",
			ParentID: &parentID,
			Sources:  []DataSource{SourceAgent},
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath:    "/dev/sg2",
				Controller: "sssraid0",
				Target:     "sssraid,0,1",
				Wearout:    -1,
			},
		},
		{
			ID:       "disk-member-2",
			Type:     ResourceTypePhysicalDisk,
			Name:     "member-2",
			ParentID: &parentID,
			Sources:  []DataSource{SourceAgent},
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath:    "/dev/sg2",
				Controller: "sssraid0",
				Target:     "sssraid,0,2",
				Wearout:    -1,
			},
		},
	}

	restarted := NewRegistry(nil)
	restarted.IngestResources(seeded)
	seededIDByTarget := make(map[string]string)
	for _, disk := range restarted.ListByType(ResourceTypePhysicalDisk) {
		if disk.PhysicalDisk != nil {
			seededIDByTarget[disk.PhysicalDisk.Target] = disk.ID
		}
	}

	for _, test := range []struct {
		sourceID string
		target   string
	}{
		{"host-1:sg2@sssraid0/sssraid,0,1", "sssraid,0,1"},
		{"host-1:sg2@sssraid0/sssraid,0,2", "sssraid,0,2"},
	} {
		got := restarted.ingest(
			SourceAgent,
			test.sourceID,
			Resource{
				Type:     ResourceTypePhysicalDisk,
				Name:     test.target,
				ParentID: &parentID,
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:    "/dev/sg2",
					Controller: "sssraid0",
					Target:     test.target,
					Wearout:    -1,
				},
			},
			ResourceIdentity{Hostnames: []string{"pve-wide"}},
		)
		if want := seededIDByTarget[test.target]; want == "" || got != want {
			t.Fatalf("restart source %q resolved to %q, want seeded ID %q", test.sourceID, got, want)
		}
	}
	if got := len(restarted.ListByType(ResourceTypePhysicalDisk)); got != 2 {
		t.Fatalf("controller members collapsed or duplicated after restart: got %d disks", got)
	}
}

func TestIssue1516AgentSMARTAndPVEInventoryMergeWithoutContradictingEvidence(t *testing.T) {
	registry := NewRegistry(nil)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	used := 95
	zero := int64(0)
	identity := ResourceIdentity{
		MachineID: "NVME-BOOT-1",
		Hostnames: []string{"pve-a"},
	}

	agentID := registry.ingest(
		SourceAgent,
		"host-1:nvme0n1",
		Resource{
			Type:     ResourceTypePhysicalDisk,
			Name:     "NVMe boot disk",
			Status:   StatusWarning,
			LastSeen: now,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath:     "/dev/nvme0n1",
				Model:       "Samsung 990 PRO",
				Serial:      "NVME-BOOT-1",
				DiskType:    "nvme",
				Health:      "FAILED",
				Wearout:     5,
				Temperature: 44,
				SMART: &SMARTMeta{
					PercentageUsed:     &used,
					ReallocatedSectors: &zero,
				},
			},
		},
		identity,
	)
	pveID := registry.ingest(
		SourceProxmox,
		"cluster-a-pve-a--dev-nvme0n1",
		Resource{
			Type:     ResourceTypePhysicalDisk,
			Name:     "Samsung 990 PRO",
			Status:   StatusOnline,
			LastSeen: now,
			Proxmox: &ProxmoxData{
				Instance: "cluster-a",
				NodeName: "pve-a",
			},
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath:   "/dev/nvme0n1",
				Model:     "Samsung 990 PRO",
				Vendor:    "Samsung",
				Serial:    "NVME-BOOT-1",
				DiskType:  "ssd",
				SizeBytes: 2_000_398_934_016,
				Health:    "PASSED",
				Wearout:   100,
				Used:      "BIOS boot",
			},
		},
		identity,
	)
	if pveID != agentID {
		t.Fatalf("agent and PVE views did not merge: agent=%q pve=%q", agentID, pveID)
	}

	disks := registry.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 || disks[0].PhysicalDisk == nil {
		t.Fatalf("expected one merged physical disk, got %+v", disks)
	}
	disk := disks[0]
	if disk.Proxmox == nil || disk.Proxmox.Instance != "cluster-a" || disk.Proxmox.NodeName != "pve-a" {
		t.Fatalf("PVE node/instance facet was lost: %+v", disk.Proxmox)
	}
	if disk.PhysicalDisk.SizeBytes != 2_000_398_934_016 ||
		disk.PhysicalDisk.Vendor != "Samsung" ||
		disk.PhysicalDisk.Used != "BIOS boot" {
		t.Fatalf("PVE-only boot/size fields were lost: %+v", disk.PhysicalDisk)
	}
	if disk.PhysicalDisk.Health != "FAILED" ||
		disk.PhysicalDisk.Wearout != 5 ||
		disk.PhysicalDisk.Temperature != 44 {
		t.Fatalf("coarse PVE values replaced SMART evidence: %+v", disk.PhysicalDisk)
	}
	if disk.PhysicalDisk.SMART == nil ||
		disk.PhysicalDisk.SMART.ReallocatedSectors == nil ||
		*disk.PhysicalDisk.SMART.ReallocatedSectors != 0 {
		t.Fatalf("reported zero SMART evidence was lost: %+v", disk.PhysicalDisk.SMART)
	}
}

func TestIssue1516ProxmoxSourceIDsSeparateNodesAndControllerMembers(t *testing.T) {
	nodeA := ProxmoxPhysicalDiskSourceID("cluster-a", "pve-a", "/dev/sda", "", "")
	nodeB := ProxmoxPhysicalDiskSourceID("cluster-a", "pve-b", "/dev/sda", "", "")
	member1 := ProxmoxPhysicalDiskSourceID("cluster-a", "pve-a", "/dev/sg2", "sssraid0", "sssraid,0,1")
	member2 := ProxmoxPhysicalDiskSourceID("cluster-a", "pve-a", "/dev/sg2", "sssraid0", "sssraid,0,2")
	for _, pair := range [][2]string{
		{nodeA, nodeB},
		{member1, member2},
		{nodeA, member1},
	} {
		if pair[0] == pair[1] {
			t.Fatalf("distinct physical disks shared source ID %q", pair[0])
		}
	}
}

func TestIssue1516RepeatedSerialAcrossNodesDoesNotCrossAttachMetrics(t *testing.T) {
	registry := NewRegistry(nil)
	for _, node := range []string{"pve-a", "pve-b"} {
		registry.ingest(
			SourceProxmox,
			ProxmoxPhysicalDiskSourceID("cluster-a", node, "/dev/sda", "", ""),
			Resource{
				Type: ResourceTypePhysicalDisk,
				Name: node + "-disk",
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:     "/dev/sda",
					Serial:      "DEFAULT-SERIAL",
					DiskType:    "sata",
					Temperature: map[string]int{"pve-a": 31, "pve-b": 47}[node],
					Wearout:     -1,
				},
			},
			ResourceIdentity{
				Hostnames: []string{node},
			},
		)
	}

	disks := registry.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 2 {
		t.Fatalf("same placeholder serial on different nodes collapsed to %d resource(s): %+v", len(disks), disks)
	}
	temperatureByHost := make(map[string]int)
	for _, disk := range disks {
		if disk.PhysicalDisk == nil || len(disk.Identity.Hostnames) != 1 {
			t.Fatalf("unexpected disk projection: %+v", disk)
		}
		temperatureByHost[disk.Identity.Hostnames[0]] = disk.PhysicalDisk.Temperature
	}
	if temperatureByHost["pve-a"] != 31 || temperatureByHost["pve-b"] != 47 {
		t.Fatalf("disk metrics crossed node boundaries: %+v", temperatureByHost)
	}
	metricA := PhysicalDiskMetricID(models.PhysicalDisk{
		ID:       ProxmoxPhysicalDiskSourceID("cluster-a", "pve-a", "/dev/sda", "", ""),
		Instance: "cluster-a",
		Node:     "pve-a",
		DevPath:  "/dev/sda",
		Serial:   "DEFAULT-SERIAL",
	})
	metricB := PhysicalDiskMetricID(models.PhysicalDisk{
		ID:       ProxmoxPhysicalDiskSourceID("cluster-a", "pve-b", "/dev/sda", "", ""),
		Instance: "cluster-a",
		Node:     "pve-b",
		DevPath:  "/dev/sda",
		Serial:   "DEFAULT-SERIAL",
	})
	if metricA == "" || metricB == "" || metricA == metricB {
		t.Fatalf("placeholder serial shared metric key: pve-a=%q pve-b=%q", metricA, metricB)
	}
}
