package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestUnraidDiskMountTotal covers the pure adapter unraidDiskMountTotal, which
// resolves a disk's mount path via unraidDiskMountPath (driven by disk role and
// name) and then selects the matching host disk's reported Total bytes.
func TestUnraidDiskMountTotal(t *testing.T) {
	tests := []struct {
		name string
		host models.Host
		disk models.HostUnraidDisk
		want int64
	}{
		{
			name: "zero-value disk resolves to no mount path",
			host: models.Host{},
			disk: models.HostUnraidDisk{},
			want: 0,
		},
		{
			name: "explicit parity role yields no mount path",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "/mnt/disk1", Total: 1000}}},
			disk: models.HostUnraidDisk{Name: "parity1", Role: "parity"},
			want: 0,
		},
		{
			name: "data role matches host disk total",
			host: models.Host{Disks: []models.Disk{
				{Mountpoint: "/mnt/cache", Total: 5000},
				{Mountpoint: "/mnt/disk1", Total: 4000},
			}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 4000,
		},
		{
			name: "cache role matches host disk total",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "/mnt/cache", Total: 3000}}},
			disk: models.HostUnraidDisk{Name: "cache", Role: "cache"},
			want: 3000,
		},
		{
			name: "data role inferred from disk name prefix without explicit role",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "/mnt/disk2", Total: 8000}}},
			disk: models.HostUnraidDisk{Name: "disk2"},
			want: 8000,
		},
		{
			name: "matching mountpoint but zero total is skipped",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "/mnt/disk1", Total: 0}}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 0,
		},
		{
			name: "matching mountpoint but negative total is skipped",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "/mnt/disk1", Total: -5}}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 0,
		},
		{
			name: "non-matching host mountpoints yield zero",
			host: models.Host{Disks: []models.Disk{
				{Mountpoint: "/mnt/cache", Total: 9000},
				{Mountpoint: "/mnt/user", Total: 7000},
			}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 0,
		},
		{
			name: "host mountpoint with surrounding whitespace is trimmed and matched",
			host: models.Host{Disks: []models.Disk{{Mountpoint: "  /mnt/disk1  ", Total: 6000}}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 6000,
		},
		{
			name: "resolved mount path but empty host disks slice",
			host: models.Host{Disks: []models.Disk{}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unraidDiskMountTotal(tt.host, tt.disk)
			if got != tt.want {
				t.Fatalf("unraidDiskMountTotal() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestDockerStorageUsageMeta covers the pure adapter dockerStorageUsageMeta,
// which returns nil for an all-zero bucket and otherwise constructs a
// DockerStorageUsageMeta copying all four bucket fields.
func TestDockerStorageUsageMeta(t *testing.T) {
	t.Run("zero-value bucket returns nil", func(t *testing.T) {
		got := dockerStorageUsageMeta(models.DockerStorageUsageBucket{})
		if got != nil {
			t.Fatalf("expected nil for zero-value bucket, got %+v", got)
		}
	})

	populated := []struct {
		name   string
		bucket models.DockerStorageUsageBucket
	}{
		{name: "only total count set short-circuits nil arm", bucket: models.DockerStorageUsageBucket{TotalCount: 3}},
		{name: "only active count set short-circuits nil arm", bucket: models.DockerStorageUsageBucket{ActiveCount: 2}},
		{name: "only total size set short-circuits nil arm", bucket: models.DockerStorageUsageBucket{TotalSizeBytes: 4096}},
		{name: "only reclaimable set short-circuits nil arm", bucket: models.DockerStorageUsageBucket{ReclaimableBytes: 2048}},
		{name: "fully populated bucket", bucket: models.DockerStorageUsageBucket{
			TotalCount:       12,
			ActiveCount:      7,
			TotalSizeBytes:   9999,
			ReclaimableBytes: 1234,
		}},
	}

	for _, tt := range populated {
		t.Run(tt.name, func(t *testing.T) {
			got := dockerStorageUsageMeta(tt.bucket)
			if got == nil {
				t.Fatalf("expected non-nil meta for non-zero bucket")
			}
			if got.TotalCount != tt.bucket.TotalCount {
				t.Fatalf("TotalCount = %d, want %d", got.TotalCount, tt.bucket.TotalCount)
			}
			if got.ActiveCount != tt.bucket.ActiveCount {
				t.Fatalf("ActiveCount = %d, want %d", got.ActiveCount, tt.bucket.ActiveCount)
			}
			if got.TotalSizeBytes != tt.bucket.TotalSizeBytes {
				t.Fatalf("TotalSizeBytes = %d, want %d", got.TotalSizeBytes, tt.bucket.TotalSizeBytes)
			}
			if got.ReclaimableBytes != tt.bucket.ReclaimableBytes {
				t.Fatalf("ReclaimableBytes = %d, want %d", got.ReclaimableBytes, tt.bucket.ReclaimableBytes)
			}
		})
	}
}
