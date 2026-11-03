package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The node disk bar falls back to a storage when neither the host agent nor
// Proxmox reports a node filesystem. The order below decides which storage the
// operator sees: the canonical system storages first, then guest-root capable
// storages by type, and never shared or content-less ones.
func TestPreferredNodeDiskFallbackRank(t *testing.T) {
	tests := []struct {
		name    string
		storage proxmox.Storage
		want    int
		wantOK  bool
	}{
		{name: "local-zfs", storage: proxmox.Storage{Storage: "local-zfs", Type: "zfspool"}, want: 0, wantOK: true},
		{name: "local-lvm", storage: proxmox.Storage{Storage: "local-lvm", Type: "lvmthin"}, want: 1, wantOK: true},
		{name: "local", storage: proxmox.Storage{Storage: "local", Type: "dir", Path: "/var/lib/vz"}, want: 2, wantOK: true},
		{name: "zfs pool with guest roots", storage: proxmox.Storage{Storage: "tank", Type: "zfspool", Content: "images,rootdir"}, want: 3, wantOK: true},
		{name: "lvm with guest images", storage: proxmox.Storage{Storage: "vg0", Type: "lvmthin", Content: "images"}, want: 4, wantOK: true},
		{name: "default dir path", storage: proxmox.Storage{Storage: "vmdata", Type: "dir", Path: "/var/lib/vz", Content: "rootdir"}, want: 5, wantOK: true},
		{name: "local-prefixed dir", storage: proxmox.Storage{Storage: "local-ssd", Type: "dir", Path: "/mnt/ssd", Content: "images"}, want: 6, wantOK: true},
		{name: "other guest storage", storage: proxmox.Storage{Storage: "fast", Type: "dir", Path: "/mnt/fast", Content: "rootdir"}, want: 7, wantOK: true},
		{name: "shared storage with guest images", storage: proxmox.Storage{Storage: "nfs-vms", Type: "nfs", Content: "images,rootdir", Shared: 1}, wantOK: false},
		{name: "backup-only storage", storage: proxmox.Storage{Storage: "backups", Type: "dir", Path: "/mnt/backups", Content: "backup"}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := preferredNodeDiskFallbackRank(tt.storage)
			if ok != tt.wantOK {
				t.Fatalf("eligible = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("rank = %d, want %d", got, tt.want)
			}
		})
	}
}

// With several storages on a node whose own disk is unreported, the node disk
// bar shows local-zfs rather than a larger or shared storage.
func TestPollPVEInstanceNodeDiskFallbackPrefersCanonicalSystemStorage(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)
	client := &stubPVEClient{
		nodes: []proxmox.Node{{
			Node:   "nuc",
			Status: "online",
			CPU:    0.12,
			MaxCPU: 8,
			Mem:    4 * gib,
			MaxMem: 8 * gib,
			Uptime: 3600,
		}},
		allStorage: []proxmox.Storage{{Storage: "local-zfs"}},
		storageByNode: map[string][]proxmox.Storage{
			"nuc": {
				{Storage: "nfs-vms", Type: "nfs", Content: "images,rootdir", Shared: 1, Total: 5000 * gib, Used: 100 * gib, Available: 4900 * gib},
				{Storage: "local", Type: "dir", Path: "/var/lib/vz", Content: "iso,vztmpl,backup", Total: 2000 * gib, Used: 200 * gib, Available: 1800 * gib},
				{Storage: "local-lvm", Type: "lvmthin", Content: "images,rootdir", Total: 1500 * gib, Used: 150 * gib, Available: 1350 * gib},
				{Storage: "local-zfs", Type: "zfspool", Content: "images,rootdir", Total: 944 * gib, Used: 313 * gib, Available: 631 * gib},
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()
	mon.config.PVEInstances[0].MonitorStorage = true

	mon.pollPVEInstance(context.Background(), "test", client)

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one node in state, got %d", len(snapshot.Nodes))
	}
	disk := snapshot.Nodes[0].Disk
	if disk.Total != int64(944*gib) || disk.Used != int64(313*gib) {
		t.Fatalf("node disk = %+v, want local-zfs totals (944 GiB total, 313 GiB used)", disk)
	}
}

// A shared storage is never a node's own disk, so when it is the only storage
// that holds guest roots the node reports no fallback disk at all.
func TestPollPVEInstanceNodeDiskFallbackIgnoresSharedStorage(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)
	client := &stubPVEClient{
		nodes: []proxmox.Node{{
			Node:   "nuc",
			Status: "online",
			CPU:    0.12,
			MaxCPU: 8,
			Mem:    4 * gib,
			MaxMem: 8 * gib,
			Uptime: 3600,
		}},
		allStorage: []proxmox.Storage{{Storage: "nfs-vms"}},
		storageByNode: map[string][]proxmox.Storage{
			"nuc": {
				{Storage: "nfs-vms", Type: "nfs", Content: "images,rootdir", Shared: 1, Total: 5000 * gib, Used: 100 * gib, Available: 4900 * gib},
				{Storage: "backups", Type: "dir", Path: "/mnt/backups", Content: "backup", Total: 700 * gib, Used: 100 * gib, Available: 600 * gib},
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()
	mon.config.PVEInstances[0].MonitorStorage = true

	mon.pollPVEInstance(context.Background(), "test", client)

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one node in state, got %d", len(snapshot.Nodes))
	}
	if disk := snapshot.Nodes[0].Disk; disk.Total != 0 {
		t.Fatalf("node disk = %+v, want no fallback from shared or backup-only storage", disk)
	}
}
