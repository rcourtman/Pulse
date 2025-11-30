package monitoring

import (
	"math"
	"testing"
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
