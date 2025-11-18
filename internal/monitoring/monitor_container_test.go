package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestParseContainerMountMetadata(t *testing.T) {
	config := map[string]interface{}{
		"rootfs": "local-lvm:vm-100-disk-0,size=8G",
		"mp0":    "/mnt/pve/media/subvol-100-disk-1,mp=/mnt/media,acl=1",
		"MP1":    "local:100/vm-100-disk-2,mp=/srv/backup",
		"unused": "ignored",
	}

	meta := parseContainerMountMetadata(config)
	if len(meta) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(meta))
	}

	root, ok := meta["rootfs"]
	if !ok {
		t.Fatalf("missing rootfs metadata")
	}
	if root.Mountpoint != "/" {
		t.Fatalf("expected root mountpoint '/', got %q", root.Mountpoint)
	}
	if root.Source != "local-lvm:vm-100-disk-0" {
		t.Fatalf("unexpected root source %q", root.Source)
	}

	mp0, ok := meta["mp0"]
	if !ok {
		t.Fatalf("missing mp0 metadata")
	}
	if mp0.Mountpoint != "/mnt/media" {
		t.Fatalf("expected mp0 mountpoint '/mnt/media', got %q", mp0.Mountpoint)
	}
	if mp0.Source != "/mnt/pve/media/subvol-100-disk-1" {
		t.Fatalf("unexpected mp0 source %q", mp0.Source)
	}

	mp1, ok := meta["mp1"]
	if !ok {
		t.Fatalf("missing mp1 metadata")
	}
	if mp1.Mountpoint != "/srv/backup" {
		t.Fatalf("expected mp1 mountpoint '/srv/backup', got %q", mp1.Mountpoint)
	}
	if mp1.Source != "local:100/vm-100-disk-2" {
		t.Fatalf("unexpected mp1 source %q", mp1.Source)
	}
}

func TestConvertContainerDiskInfoUsesMountMetadata(t *testing.T) {
	status := &proxmox.Container{
		RootFS: "local-lvm:vm-200-disk-0,size=20G",
		DiskInfo: map[string]proxmox.ContainerDiskUsage{
			"rootfs": {Total: 20 * 1024 * 1024 * 1024, Used: 10 * 1024 * 1024 * 1024},
			"mp0":    {Total: 100 * 1024 * 1024 * 1024, Used: 50 * 1024 * 1024 * 1024},
		},
	}

	meta := map[string]containerMountMetadata{
		"rootfs": {Key: "rootfs", Mountpoint: "/", Source: "local-lvm:vm-200-disk-0"},
		"mp0":    {Key: "mp0", Mountpoint: "/mnt/media", Source: "/mnt/pve/media/subvol-200-disk-1"},
	}

	disks := convertContainerDiskInfo(status, meta)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}

	var rootDisk, mpDisk *models.Disk
	for i := range disks {
		if disks[i].Mountpoint == "/" {
			rootDisk = &disks[i]
		}
		if disks[i].Mountpoint == "/mnt/media" {
			mpDisk = &disks[i]
		}
	}

	if rootDisk == nil {
		t.Fatalf("root disk not found in %+v", disks)
	}
	if rootDisk.Device != "local-lvm:vm-200-disk-0" {
		t.Fatalf("unexpected root device %q", rootDisk.Device)
	}
	if rootDisk.Type != "rootfs" {
		t.Fatalf("expected root type rootfs, got %q", rootDisk.Type)
	}

	if mpDisk == nil {
		t.Fatalf("mp0 disk not found in %+v", disks)
	}
	if mpDisk.Device != "/mnt/pve/media/subvol-200-disk-1" {
		t.Fatalf("unexpected mp0 device %q", mpDisk.Device)
	}
	if mpDisk.Type != "mp0" {
		t.Fatalf("expected mp0 type 'mp0', got %q", mpDisk.Type)
	}
}

func TestConvertContainerDiskInfoFallsBackWithoutMetadata(t *testing.T) {
	status := &proxmox.Container{
		RootFS: "local-lvm:vm-300-disk-0,size=16G",
		DiskInfo: map[string]proxmox.ContainerDiskUsage{
			"rootfs": {Total: 16 * 1024 * 1024 * 1024, Used: 8 * 1024 * 1024 * 1024},
			"mp1":    {Total: 5 * 1024 * 1024 * 1024, Used: 1 * 1024 * 1024 * 1024},
		},
	}

	disks := convertContainerDiskInfo(status, nil)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}

	var mpDisk *models.Disk
	for i := range disks {
		if disks[i].Type == "mp1" {
			mpDisk = &disks[i]
		}
	}
	if mpDisk == nil {
		t.Fatalf("mp1 disk not found in %+v", disks)
	}
	if mpDisk.Mountpoint != "mp1" {
		t.Fatalf("expected fallback mountpoint 'mp1', got %q", mpDisk.Mountpoint)
	}
}
