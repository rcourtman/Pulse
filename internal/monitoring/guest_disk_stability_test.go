package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestStabilizeGuestLowTrustDiskRejectsPreviouslyCarriedForwardSnapshot(t *testing.T) {
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
