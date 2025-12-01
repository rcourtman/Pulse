package monitoring

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestClampToInt64(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		expected int64
	}{
		{
			name:     "zero",
			value:    0,
			expected: 0,
		},
		{
			name:     "small value",
			value:    100,
			expected: 100,
		},
		{
			name:     "max int64",
			value:    uint64(math.MaxInt64),
			expected: math.MaxInt64,
		},
		{
			name:     "one below max int64",
			value:    uint64(math.MaxInt64) - 1,
			expected: math.MaxInt64 - 1,
		},
		{
			name:     "one above max int64",
			value:    uint64(math.MaxInt64) + 1,
			expected: math.MaxInt64,
		},
		{
			name:     "max uint64",
			value:    math.MaxUint64,
			expected: math.MaxInt64,
		},
		{
			name:     "typical disk size 1TB",
			value:    1099511627776, // 1 TiB
			expected: 1099511627776,
		},
		{
			name:     "typical disk size 100TB",
			value:    109951162777600, // 100 TiB
			expected: 109951162777600,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := clampToInt64(tc.value)
			if result != tc.expected {
				t.Errorf("clampToInt64(%d) = %d, want %d", tc.value, result, tc.expected)
			}
		})
	}
}

func TestStorageSupportsContainerVolumes(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "rootdir only",
			content:  "rootdir",
			expected: true,
		},
		{
			name:     "images only",
			content:  "images",
			expected: true,
		},
		{
			name:     "subvol only",
			content:  "subvol",
			expected: true,
		},
		{
			name:     "mixed with rootdir",
			content:  "backup,rootdir,vztmpl",
			expected: true,
		},
		{
			name:     "mixed with images",
			content:  "iso,images,snippets",
			expected: true,
		},
		{
			name:     "mixed with subvol",
			content:  "subvol,backup",
			expected: true,
		},
		{
			name:     "uppercase ROOTDIR",
			content:  "ROOTDIR",
			expected: true,
		},
		{
			name:     "uppercase IMAGES",
			content:  "IMAGES",
			expected: true,
		},
		{
			name:     "uppercase SUBVOL",
			content:  "SUBVOL",
			expected: true,
		},
		{
			name:     "mixed case RootDir",
			content:  "RootDir",
			expected: true,
		},
		{
			name:     "no container volumes",
			content:  "backup,iso,vztmpl",
			expected: false,
		},
		{
			name:     "spaces around items",
			content:  " rootdir , backup ",
			expected: true,
		},
		{
			name:     "all three container types",
			content:  "rootdir,images,subvol",
			expected: true,
		},
		{
			name:     "partial match should not match",
			content:  "rootdirx,imagesy",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := storageSupportsContainerVolumes(tc.content)
			if result != tc.expected {
				t.Errorf("storageSupportsContainerVolumes(%q) = %v, want %v", tc.content, result, tc.expected)
			}
		})
	}
}

func TestIsRootVolumeForContainer(t *testing.T) {
	tests := []struct {
		name     string
		volid    string
		vmid     int
		expected bool
	}{
		{
			name:     "empty volid",
			volid:    "",
			vmid:     100,
			expected: false,
		},
		{
			name:     "zero vmid",
			volid:    "local:subvol-100-disk-0",
			vmid:     0,
			expected: false,
		},
		{
			name:     "negative vmid",
			volid:    "local:subvol-100-disk-0",
			vmid:     -1,
			expected: false,
		},
		{
			name:     "subvol disk-0 match",
			volid:    "local:subvol-100-disk-0",
			vmid:     100,
			expected: true,
		},
		{
			name:     "vm disk-0 match",
			volid:    "local:vm-100-disk-0",
			vmid:     100,
			expected: true,
		},
		{
			name:     "subvol disk-1 no match",
			volid:    "local:subvol-100-disk-1",
			vmid:     100,
			expected: false,
		},
		{
			name:     "vm disk-1 no match",
			volid:    "local:vm-100-disk-1",
			vmid:     100,
			expected: false,
		},
		{
			name:     "different vmid no match",
			volid:    "local:subvol-100-disk-0",
			vmid:     101,
			expected: false,
		},
		{
			name:     "with snapshot suffix",
			volid:    "local:subvol-100-disk-0@snapshot1",
			vmid:     100,
			expected: true,
		},
		{
			name:     "uppercase storage",
			volid:    "LOCAL:SUBVOL-100-DISK-0",
			vmid:     100,
			expected: true,
		},
		{
			name:     "mixed case",
			volid:    "Local:SubVol-100-Disk-0",
			vmid:     100,
			expected: true,
		},
		{
			name:     "larger vmid",
			volid:    "zfs-storage:subvol-10234-disk-0",
			vmid:     10234,
			expected: true,
		},
		{
			name:     "vmid substring should not match",
			volid:    "local:subvol-1001-disk-0",
			vmid:     100,
			expected: false,
		},
		{
			name:     "vmid prefix should not match",
			volid:    "local:subvol-10-disk-0",
			vmid:     100,
			expected: false,
		},
		{
			name:     "complex path with subvol",
			volid:    "ceph:subvol-100-disk-0/rootfs",
			vmid:     100,
			expected: true,
		},
		{
			name:     "vm pattern with snapshot",
			volid:    "local-zfs:vm-200-disk-0@autosnap",
			vmid:     200,
			expected: true,
		},
		{
			name:     "unrelated volume format",
			volid:    "local:iso/debian.iso",
			vmid:     100,
			expected: false,
		},
		{
			name:     "backup volume should not match",
			volid:    "backup:vzdump-lxc-100-2024.tar.zst",
			vmid:     100,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isRootVolumeForContainer(tc.volid, tc.vmid)
			if result != tc.expected {
				t.Errorf("isRootVolumeForContainer(%q, %d) = %v, want %v", tc.volid, tc.vmid, result, tc.expected)
			}
		})
	}
}

func TestContainerDiskOverride_Fields(t *testing.T) {
	override := containerDiskOverride{
		Used:  1073741824,  // 1 GiB
		Total: 10737418240, // 10 GiB
	}

	if override.Used != 1073741824 {
		t.Errorf("Used = %d, want 1073741824", override.Used)
	}
	if override.Total != 10737418240 {
		t.Errorf("Total = %d, want 10737418240", override.Total)
	}
}

// stubContainerDiskClient implements PVEClientInterface for testing collectContainerRootUsage
type stubContainerDiskClient struct {
	stubPVEClient
	storages       []proxmox.Storage
	storagesErr    error
	contentByStore map[string][]proxmox.StorageContent
	contentErr     map[string]error
}

func (s *stubContainerDiskClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	if s.storagesErr != nil {
		return nil, s.storagesErr
	}
	return s.storages, nil
}

func (s *stubContainerDiskClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if s.contentErr != nil {
		if err, ok := s.contentErr[storage]; ok {
			return nil, err
		}
	}
	if s.contentByStore != nil {
		return s.contentByStore[storage], nil
	}
	return nil, nil
}

func TestCollectContainerRootUsage(t *testing.T) {
	mon := &Monitor{}

	t.Run("empty vmIDs list returns empty map", func(t *testing.T) {
		client := &stubContainerDiskClient{}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("nil vmIDs list returns empty map", func(t *testing.T) {
		client := &stubContainerDiskClient{}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", nil)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("no storages returns empty map", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("storage does not support container volumes", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "backup-store", Content: "backup,iso", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"backup-store": {{Volid: "backup-store:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (storage doesn't support container volumes), got %d entries", len(result))
		}
	})

	t.Run("GetStorageContent error is handled gracefully", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
				{Storage: "zfs", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"zfs": {{Volid: "zfs:subvol-100-disk-0", VMID: 100, Used: 2048, Size: 8192}},
			},
			contentErr: map[string]error{
				"local": errors.New("storage offline"),
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		// Should continue to next storage and find the container in zfs
		if len(result) != 1 {
			t.Errorf("expected 1 entry from zfs storage, got %d", len(result))
		}
		if override, ok := result[100]; !ok || override.Used != 2048 {
			t.Errorf("expected override for vmid 100 with Used=2048, got %+v", result)
		}
	})

	t.Run("GetStorage error returns empty map", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storagesErr: errors.New("API error"),
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map on GetStorage error, got %d entries", len(result))
		}
	})

	t.Run("matching container root volume found", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		override, ok := result[100]
		if !ok {
			t.Fatal("expected entry for vmid 100")
		}
		if override.Used != 1024 {
			t.Errorf("Used = %d, want 1024", override.Used)
		}
		if override.Total != 4096 {
			t.Errorf("Total = %d, want 4096", override.Total)
		}
	})

	t.Run("volume with Used=0 is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-0", VMID: 100, Used: 0, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (Used=0), got %d entries", len(result))
		}
	})

	t.Run("volume with non-matching VMID is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-200-disk-0", VMID: 200, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (VMID not in list), got %d entries", len(result))
		}
	})

	t.Run("volume with non-root volid is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-1", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (not root disk), got %d entries", len(result))
		}
	})

	t.Run("volume with VMID=0 is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-0", VMID: 0, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (VMID=0), got %d entries", len(result))
		}
	})

	t.Run("multiple containers multiple storages", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 1},
				{Storage: "zfs", Content: "images,rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {
					{Volid: "local:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096},
					{Volid: "local:subvol-101-disk-0", VMID: 101, Used: 2048, Size: 8192},
				},
				"zfs": {
					{Volid: "zfs:subvol-102-disk-0", VMID: 102, Used: 3072, Size: 12288},
					{Volid: "zfs:subvol-103-disk-0", VMID: 103, Used: 4096, Size: 16384},
				},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100, 101, 102, 103})
		if len(result) != 4 {
			t.Fatalf("expected 4 entries, got %d", len(result))
		}
		expected := map[int]containerDiskOverride{
			100: {Used: 1024, Total: 4096},
			101: {Used: 2048, Total: 8192},
			102: {Used: 3072, Total: 12288},
			103: {Used: 4096, Total: 16384},
		}
		for vmid, want := range expected {
			got, ok := result[vmid]
			if !ok {
				t.Errorf("missing entry for vmid %d", vmid)
				continue
			}
			if got.Used != want.Used || got.Total != want.Total {
				t.Errorf("vmid %d: got %+v, want %+v", vmid, got, want)
			}
		}
	})

	t.Run("storage not enabled is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 0, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (storage not enabled), got %d entries", len(result))
		}
	})

	t.Run("storage not active is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "rootdir", Enabled: 1, Active: 0},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (storage not active), got %d entries", len(result))
		}
	})

	t.Run("storage with empty name is skipped", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "", Content: "rootdir", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"": {{Volid: ":subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 0 {
			t.Errorf("expected empty map (storage name empty), got %d entries", len(result))
		}
	})

	t.Run("vm-disk pattern also matches", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "local", Content: "images", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"local": {{Volid: "local:vm-100-disk-0", VMID: 100, Used: 5000, Size: 10000}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 1 {
			t.Fatalf("expected 1 entry (vm-disk pattern), got %d", len(result))
		}
		if result[100].Used != 5000 {
			t.Errorf("Used = %d, want 5000", result[100].Used)
		}
	})

	t.Run("subvol content type works", func(t *testing.T) {
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "zfs", Content: "subvol", Enabled: 1, Active: 1},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"zfs": {{Volid: "zfs:subvol-100-disk-0", VMID: 100, Used: 7777, Size: 9999}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		if len(result) != 1 {
			t.Fatalf("expected 1 entry (subvol content type), got %d", len(result))
		}
		if result[100].Used != 7777 {
			t.Errorf("Used = %d, want 7777", result[100].Used)
		}
	})

	t.Run("PBS storage with Active=0 is queryable for backup content", func(t *testing.T) {
		// PBS storages report Active=0 but should be queryable if they have backup content
		// However, collectContainerRootUsage skips storages that don't support container volumes
		// PBS with "backup" content won't match rootdir/images/subvol
		client := &stubContainerDiskClient{
			storages: []proxmox.Storage{
				{Storage: "pbs", Content: "backup", Type: "pbs", Enabled: 1, Active: 0},
			},
			contentByStore: map[string][]proxmox.StorageContent{
				"pbs": {{Volid: "pbs:subvol-100-disk-0", VMID: 100, Used: 1024, Size: 4096}},
			},
		}
		result := mon.collectContainerRootUsage(context.Background(), client, "node1", []int{100})
		// PBS with only "backup" content doesn't support container volumes
		if len(result) != 0 {
			t.Errorf("expected empty map (PBS backup-only storage), got %d entries", len(result))
		}
	})
}
