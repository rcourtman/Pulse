package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestStabilizeGuestLowTrustDiskCarriesForwardPreviousSnapshot(t *testing.T) {
	now := time.Now()
	prev := &models.VM{
		Type:         "qemu",
		Status:       "running",
		LastSeen:     now.Add(-time.Minute),
		AgentVersion: "8.2.0",
		Disk: models.Disk{
			Total: 1000,
			Used:  400,
			Free:  600,
			Usage: 40,
		},
		Disks: []models.Disk{
			{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/", Type: "ext4", Device: "/dev/vda"},
		},
	}

	total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(
		prev,
		"running",
		1000,
		0,
		1000,
		-1,
		nil,
		"no-filesystems",
		false,
		now,
	)

	if total != 1000 || used != 400 || free != 600 || usage != 40 {
		t.Fatalf("unexpected carried-forward disk summary: total=%d used=%d free=%d usage=%.2f", total, used, free, usage)
	}
	if len(disks) != 1 || disks[0].Device != "/dev/vda" {
		t.Fatalf("expected previous disks to be cloned, got %#v", disks)
	}
	if reason != "prev-no-filesystems" {
		t.Fatalf("reason = %q, want prev-no-filesystems", reason)
	}
}

func TestStabilizeGuestLowTrustDiskRejectsPreviouslyCarriedForwardSnapshotWithoutAgentEvidence(t *testing.T) {
	now := time.Now()
	prev := &models.VM{
		Type:             "qemu",
		Status:           "running",
		LastSeen:         now.Add(-time.Minute),
		DiskStatusReason: "prev-no-filesystems",
		Disk: models.Disk{
			Total: 1000,
			Used:  400,
			Free:  600,
			Usage: 40,
		},
		Disks: []models.Disk{
			{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/", Type: "ext4", Device: "/dev/vda"},
		},
	}

	total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(
		prev,
		"running",
		1000,
		0,
		1000,
		-1,
		nil,
		"no-filesystems",
		false,
		now,
	)

	if total != 1000 || used != 0 || free != 1000 || usage != -1 {
		t.Fatalf("expected current low-trust values to remain, got total=%d used=%d free=%d usage=%.2f", total, used, free, usage)
	}
	if disks != nil {
		t.Fatalf("expected no disks to be carried forward, got %#v", disks)
	}
	if reason != "no-filesystems" {
		t.Fatalf("reason = %q, want no-filesystems", reason)
	}
}

func TestStabilizeGuestLowTrustDiskCarriesPreviouslyForwardedSnapshotWithAgentEvidence(t *testing.T) {
	now := time.Now()
	prev := &models.VM{
		Type:             "qemu",
		Status:           "running",
		LastSeen:         now.Add(-time.Minute),
		AgentVersion:     "8.2.0",
		DiskStatusReason: "prev-no-filesystems",
		Disk: models.Disk{
			Total: 1000,
			Used:  400,
			Free:  600,
			Usage: 40,
		},
		Disks: []models.Disk{
			{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/", Type: "ext4", Device: "/dev/vda"},
		},
	}

	total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(
		prev,
		"running",
		1000,
		0,
		1000,
		-1,
		nil,
		"no-filesystems",
		false,
		now,
	)

	if total != 1000 || used != 400 || free != 600 || usage != 40 {
		t.Fatalf("unexpected carried-forward disk summary: total=%d used=%d free=%d usage=%.2f", total, used, free, usage)
	}
	if len(disks) != 1 || disks[0].Device != "/dev/vda" {
		t.Fatalf("expected previous disks to be cloned, got %#v", disks)
	}
	if reason != "prev-no-filesystems" {
		t.Fatalf("reason = %q, want prev-no-filesystems", reason)
	}
}

type failingFSInfoClient struct {
	PVEClientInterface
	err error
}

func (c failingFSInfoClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return nil, c.err
}

func TestUpdateVMDisksFromGuestAgentFSInfoErrorUsesUnavailableSentinel(t *testing.T) {
	// Cluster/resources reports 0 used for QEMU, so a failed agent query must
	// not pass those zeros through as a confident 0% (it renders as
	// "0% (0 B/120 GB)" instead of unavailable + reason in the UI).
	m := &Monitor{}
	res := proxmox.ClusterResource{Node: "pve1", VMID: 100, Name: "vm100", MaxDisk: 128849018880}
	client := failingFSInfoClient{err: errors.New("guest agent request timeout")}

	total, used, free, usage, disks, fromAgent, reason := m.updateVMDisksFromGuestAgentFSInfo(
		context.Background(),
		"test-instance",
		res,
		client,
		uint64(res.MaxDisk),
		0,
		0,
	)

	if usage != -1 {
		t.Fatalf("usage = %v, want -1 sentinel", usage)
	}
	if reason != "agent-timeout" {
		t.Fatalf("reason = %q, want agent-timeout", reason)
	}
	if fromAgent {
		t.Fatal("fromAgent = true, want false")
	}
	if total != uint64(res.MaxDisk) {
		t.Fatalf("total = %d, want allocated size %d", total, res.MaxDisk)
	}
	if used != 0 || free != total {
		t.Fatalf("used/free = %d/%d, want 0/%d", used, free, total)
	}
	if disks != nil {
		t.Fatalf("disks = %v, want nil", disks)
	}
}
