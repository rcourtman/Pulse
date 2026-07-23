package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func int64PtrDiskFallback(v int64) *int64 { return &v }

func TestPhysicalDisksFromHostAgentSMART_BuildsDisks(t *testing.T) {
	entries := []models.HostDiskSMART{
		{
			Device:      "sda",
			Model:       "ST18000NM019J",
			Serial:      "ZR5DLAYJ",
			WWN:         "5-c500-da60ca43",
			Type:        "sas",
			SizeBytes:   18000207937536,
			Temperature: 30,
			Health:      "PASSED",
			Pool:        "tank",
			Attributes:  &models.SMARTAttributes{PowerOnHours: int64PtrDiskFallback(327)},
		},
		{
			// Standby entry keeps its identity but carries no health reading.
			Device:  "sdb",
			Model:   "ST18000NM019J",
			Serial:  "ZR5DLXYZ",
			Type:    "sas",
			Standby: true,
		},
		{
			// Entries without a device name cannot form a stable ID.
			Device: "  ",
		},
	}

	disks := physicalDisksFromHostAgentSMART("pve1", "node1", entries)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d: %+v", len(disks), disks)
	}

	first := disks[0]
	if first.ID != "pve1-node1--dev-sda" {
		t.Errorf("unexpected ID %q", first.ID)
	}
	if first.DevPath != "/dev/sda" || first.Node != "node1" || first.Instance != "pve1" {
		t.Errorf("unexpected identity fields: %+v", first)
	}
	if first.Health != "PASSED" || first.Temperature != 30 || first.Size != 18000207937536 {
		t.Errorf("unexpected data fields: %+v", first)
	}
	if first.StorageGroup != "tank" {
		t.Errorf("expected pool carried into StorageGroup, got %q", first.StorageGroup)
	}
	if first.SmartAttributes == nil || first.SmartAttributes.PowerOnHours == nil || *first.SmartAttributes.PowerOnHours != 327 {
		t.Errorf("expected SMART attributes carried over, got %+v", first.SmartAttributes)
	}
	if first.Wearout != -1 {
		t.Errorf("expected wearout unknown (-1) without wear attributes, got %d", first.Wearout)
	}

	second := disks[1]
	if second.DevPath != "/dev/sdb" || second.Health != "UNKNOWN" {
		t.Errorf("expected standby disk listed with UNKNOWN health, got %+v", second)
	}

	if got := physicalDisksFromHostAgentSMART("pve1", "node1", nil); got != nil {
		t.Errorf("expected nil for empty SMART inventory, got %+v", got)
	}
}

type disksErrorPVEClient struct {
	fakeStorageClient
}

func (*disksErrorPVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return nil, fmt.Errorf("596 connection timed out")
}

type permissionDeniedDisksPVEClient struct {
	fakeStorageClient
	called chan struct{}
}

func (client *permissionDeniedDisksPVEClient) GetDisks(context.Context, string) ([]proxmox.Disk, error) {
	close(client.called)
	return nil, fmt.Errorf("403 permission check failed")
}

// A node whose Proxmox disks/list query fails (wide nodes can exceed the API
// window) must still populate its Physical Disks view from the linked host
// agent's SMART inventory instead of silently emptying (#1516).
func TestMaybePollPhysicalDisksAsync_AgentFallbackWhenDiskQueryFails(t *testing.T) {
	m := &Monitor{
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		alertManager:         alerts.NewManager(),
	}

	m.state.UpdateNodesForInstance("pve1", []models.Node{{
		ID:            "pve1-node1",
		Name:          "node1",
		Instance:      "pve1",
		LinkedAgentID: "host-1",
	}})
	m.state.UpsertHost(models.Host{
		ID:       "host-1",
		Hostname: "node1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:      "sda",
				Model:       "ST18000NM019J",
				Serial:      "ZR5DLAYJ",
				Type:        "sas",
				SizeBytes:   18000207937536,
				Temperature: 30,
				Health:      "PASSED",
			}},
		},
	})

	m.maybePollPhysicalDisksAsync(
		context.Background(),
		"pve1",
		&config.PVEInstance{},
		&disksErrorPVEClient{},
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		[]models.Node{{Name: "node1"}},
	)

	deadline := time.Now().Add(3 * time.Second)
	for {
		disks := m.state.GetSnapshot().PhysicalDisks
		if len(disks) == 1 {
			disk := disks[0]
			if disk.DevPath != "/dev/sda" || disk.Node != "node1" || disk.Instance != "pve1" {
				t.Fatalf("unexpected fallback disk identity: %+v", disk)
			}
			if disk.Health != "PASSED" || disk.Temperature != 30 {
				t.Fatalf("unexpected fallback disk data: %+v", disk)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected fallback physical disk from host agent SMART, got %+v", disks)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMaybePollPhysicalDisksAsync_PermissionFailurePreservesPreviousInventory(t *testing.T) {
	m := &Monitor{
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		alertManager:         alerts.NewManager(),
	}
	existing := models.PhysicalDisk{
		ID:          "pve1-node1--dev-sda",
		Node:        "node1",
		Instance:    "pve1",
		DevPath:     "/dev/sda",
		Model:       "CT240BX500SSD1",
		Serial:      "SATA-BOOT-1",
		Type:        "sata",
		Size:        240057409536,
		Health:      "PASSED",
		Wearout:     -1,
		Temperature: 54,
		Used:        "LVM",
		LastChecked: time.Now().Add(-time.Minute),
	}
	m.state.UpdatePhysicalDisks("pve1", []models.PhysicalDisk{existing})
	m.state.UpdateNodesForInstance("pve1", []models.Node{{
		ID:       "pve1-node1",
		Name:     "node1",
		Instance: "pve1",
	}})
	beforePoll := m.state.GetSnapshot().LastUpdate
	client := &permissionDeniedDisksPVEClient{called: make(chan struct{})}

	m.maybePollPhysicalDisksAsync(
		context.Background(),
		"pve1",
		&config.PVEInstance{},
		client,
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		[]models.Node{{Name: "node1", Instance: "pve1"}},
	)

	deadline := time.Now().Add(3 * time.Second)
	select {
	case <-client.called:
	case <-time.After(3 * time.Second):
		t.Fatal("permission-denied disk poll was not attempted")
	}
	for {
		snapshot := m.state.GetSnapshot()
		disks := snapshot.PhysicalDisks
		if snapshot.LastUpdate.After(beforePoll) && len(disks) == 1 && disks[0].Serial == existing.Serial {
			if disks[0].Temperature != 54 || disks[0].Used != "LVM" {
				t.Fatalf("permission failure erased prior disk evidence: %+v", disks[0])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("permission failure did not preserve prior inventory: %+v", disks)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
