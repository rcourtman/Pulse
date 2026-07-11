package unifiedresources

import "testing"

func TestHostStorageCleanupTargetDiskUsesLongestContainingMount(t *testing.T) {
	disks := []DiskInfo{
		{Mountpoint: "/", Usage: 96},
		{Mountpoint: "/var", Usage: 55},
		{Mountpoint: "/var/cache/apt", Usage: 91},
		{Mountpoint: "/var/cache/apt/archives-other", Usage: 99},
	}
	disk, ok := HostStorageCleanupTargetDisk(disks)
	if !ok || disk.Mountpoint != "/var/cache/apt" {
		t.Fatalf("target disk = %#v ok=%v", disk, ok)
	}
	if _, pressured := HostStorageCleanupPressureDisk(disks); !pressured {
		t.Fatal("expected actual package-cache filesystem to be under pressure")
	}

	disks[2].Usage = 89
	if _, pressured := HostStorageCleanupPressureDisk(disks); pressured {
		t.Fatal("root or sibling pressure must not authorize cleanup on a healthy package-cache filesystem")
	}
}
