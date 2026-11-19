package hostmetrics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	gopsutilcommon "github.com/shirou/gopsutil/v4/common"
	godisk "github.com/shirou/gopsutil/v4/disk"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

func TestMountEnumerationContextUsesAgentNamespaceOnNativeLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mount namespaces only")
	}
	t.Setenv("HOST_PROC", "")
	t.Setenv("HOST_PROC_MOUNTINFO", "")

	original := gopsutilcommon.EnvMap{gopsutilcommon.HostSysEnvKey: "/host/sys"}
	ctx := context.WithValue(context.Background(), gopsutilcommon.EnvKey, original)
	got, ok := mountEnumerationContext(ctx).Value(gopsutilcommon.EnvKey).(gopsutilcommon.EnvMap)
	if !ok {
		t.Fatal("mount enumeration context has no gopsutil environment")
	}
	if got[gopsutilcommon.HostProcMountinfo] != "/proc/self/mountinfo" {
		t.Fatalf("mountinfo = %q, want collector namespace", got[gopsutilcommon.HostProcMountinfo])
	}
	if got[gopsutilcommon.HostSysEnvKey] != "/host/sys" {
		t.Fatalf("existing gopsutil context was discarded: %+v", got)
	}
	if _, mutated := original[gopsutilcommon.HostProcMountinfo]; mutated {
		t.Fatal("caller's gopsutil environment was mutated")
	}
}

func TestMountEnumerationContextPreservesHostProcOverride(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mount namespaces only")
	}
	t.Setenv("HOST_PROC", "")
	t.Setenv("HOST_PROC_MOUNTINFO", "")

	original := gopsutilcommon.EnvMap{gopsutilcommon.HostProcEnvKey: "/host/proc"}
	ctx := context.WithValue(context.Background(), gopsutilcommon.EnvKey, original)
	got := mountEnumerationContext(ctx)
	if got != ctx {
		t.Fatal("explicit host proc context was replaced")
	}

	t.Setenv("HOST_PROC_MOUNTINFO", "/host/proc/1/mountinfo")
	plain := context.Background()
	if got := mountEnumerationContext(plain); got != plain {
		t.Fatal("explicit host mountinfo environment was replaced")
	}
}

func TestCollectDiskIO(t *testing.T) {
	ctx := context.Background()

	snapshot, err := Collect(ctx, nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	t.Logf("DiskIO count: %d", len(snapshot.DiskIO))

	if len(snapshot.DiskIO) == 0 {
		t.Error("Expected disk IO data but got none")
	}

	data, _ := json.MarshalIndent(snapshot.DiskIO, "", "  ")
	t.Logf("DiskIO data:\n%s", string(data))
}

// TestCollectDisks_DeviceDeduplication verifies that multiple mount points
// sharing the same device and total bytes are deduplicated (issue #953).
// This prevents Synology NAS and similar systems from over-counting storage.
func TestCollectDisks_DeviceDeduplication(t *testing.T) {
	// Save original function and restore after test
	origPartitions := diskPartitions
	origUsage := diskUsage
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	}()

	// Simulate Synology-like scenario: multiple shared folders on same volume
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/docker", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/photos", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/music", Fstype: "btrfs"},
			{Device: "/dev/sdb1", Mountpoint: "/mnt/backup", Fstype: "ext4"}, // Different device
		}, nil
	}

	// All volume1 mounts report the same 16TB total
	const volume1Total = uint64(16 * 1024 * 1024 * 1024 * 1024) // 16 TB
	const backupTotal = uint64(4 * 1024 * 1024 * 1024 * 1024)   // 4 TB

	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		if path == "/mnt/backup" {
			return &godisk.UsageStat{
				Total:       backupTotal,
				Used:        backupTotal / 2,
				Free:        backupTotal / 2,
				UsedPercent: 50.0,
			}, nil
		}
		// All volume1 paths report the same totals
		return &godisk.UsageStat{
			Total:       volume1Total,
			Used:        volume1Total / 4,
			Free:        volume1Total * 3 / 4,
			UsedPercent: 25.0,
		}, nil
	}

	ctx := context.Background()
	disks := collectDisks(ctx, nil)

	// Should have exactly 2 disks: /volume1 (deduplicated) and /mnt/backup
	if len(disks) != 2 {
		t.Errorf("Expected 2 disks after deduplication, got %d", len(disks))
		for _, d := range disks {
			t.Logf("  - %s (%s): %d bytes", d.Mountpoint, d.Device, d.TotalBytes)
		}
	}

	// Verify the shallowest mountpoint (/volume1) was kept
	foundVolume1 := false
	foundBackup := false
	for _, d := range disks {
		if d.Mountpoint == "/volume1" {
			foundVolume1 = true
		}
		if d.Mountpoint == "/mnt/backup" {
			foundBackup = true
		}
	}

	if !foundVolume1 {
		t.Error("Expected /volume1 to be included (shallowest path)")
	}
	if !foundBackup {
		t.Error("Expected /mnt/backup to be included (different device)")
	}

	// Calculate total - should be 16TB + 4TB = 20TB, not 64TB + 4TB = 68TB
	var total int64
	for _, d := range disks {
		total += d.TotalBytes
	}
	expectedTotal := int64(volume1Total + backupTotal)
	if total != expectedTotal {
		t.Errorf("Total storage should be %d bytes, got %d bytes", expectedTotal, total)
	}
}

func TestCollectDisksExcludesFreeBSDFdescfsBeforeUsage(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "fdescfs", Mountpoint: "/var/run/samba/fd", Fstype: "fdescfs"},
			{Device: "/dev/ada0p2", Mountpoint: "/", Fstype: "ufs"},
		}, nil
	}

	usageCalls := make([]string, 0, 1)
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		usageCalls = append(usageCalls, path)
		return &godisk.UsageStat{
			Total:       100,
			Used:        25,
			Free:        75,
			UsedPercent: 25,
		}, nil
	}

	disks := collectDisks(context.Background(), []string{"/var/run/samba/fd"})
	if len(disks) != 1 || disks[0].Mountpoint != "/" {
		t.Fatalf("excluded fdescfs mount was reported: %+v", disks)
	}
	if len(usageCalls) != 1 || usageCalls[0] != "/" {
		t.Fatalf("disk usage was queried for an excluded fdescfs mount: %v", usageCalls)
	}
}

// A Mac backing up to a network Time Machine destination mounts the backup
// disk image plus one read-only snapshot per retained backup. macOS marks all
// of them nobrowse; reporting them buried the real disks under rows whose
// capacity is the sparse bundle's ceiling, not the destination's space.
func TestCollectDisksSkipsMacOSHiddenMountsBeforeUsage(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	const backupImage = "/Volumes/Backups of RICHARD-MAC-MINI"
	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/disk3s1s1", Mountpoint: "/", Fstype: "apfs", Opts: []string{"ro", "journaled", "local"}},
			{Device: "/dev/disk7s1", Mountpoint: "/Volumes/Development", Fstype: "apfs", Opts: []string{"rw", "nodev", "nosuid", "journaled", "local", "noowners"}},
			{Device: "/dev/disk11s1", Mountpoint: backupImage, Fstype: "apfs", Opts: []string{"rw", "nodev", "nosuid", "nobrowse", "journaled", "local"}},
			{Device: "com.apple.TimeMachine.2026-10-25-122503.backup@/dev/disk11s1", Mountpoint: "/Volumes/.timemachine/5D0A488B/2026-10-25-122503.backup", Fstype: "apfs", Opts: []string{"ro", "nodev", "nosuid", "nobrowse", "journaled", "local"}},
		}, nil
	}

	usageCalls := make([]string, 0, 2)
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		usageCalls = append(usageCalls, path)
		return &godisk.UsageStat{Path: path, Total: 1000, Used: 400, Free: 600, UsedPercent: 40}, nil
	}

	disks := collectDisks(context.Background(), nil)
	got := make([]string, 0, len(disks))
	for _, disk := range disks {
		got = append(got, disk.Mountpoint)
	}
	want := []string{"/", "/Volumes/Development"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reported mounts = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(usageCalls, want) {
		t.Fatalf("disk usage queried for %v, want only %v", usageCalls, want)
	}

	disks = collectDisksWithIncludes(context.Background(), nil, []string{backupImage})
	found := false
	for _, disk := range disks {
		if disk.Mountpoint == backupImage {
			found = true
		}
	}
	if !found {
		t.Fatalf("explicitly included hidden mount was dropped: %+v", disks)
	}
}

func TestCollectDisksIncludesExplicitTmpfsMount(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "log2ram", Mountpoint: "/var/log", Fstype: "tmpfs"},
			{Device: "tmpfs", Mountpoint: "/run", Fstype: "tmpfs"},
		}, nil
	}
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{
			Path:        path,
			Total:       1024,
			Used:        768,
			Free:        256,
			UsedPercent: 75,
		}, nil
	}

	disks := collectDisksWithIncludes(context.Background(), nil, []string{"/var/log"})
	if len(disks) != 1 || disks[0].Mountpoint != "/var/log" || disks[0].Filesystem != "tmpfs" {
		t.Fatalf("explicit tmpfs include was not reported correctly: %+v", disks)
	}
}

func TestCollectDisksKeepsDistinctExplicitTmpfsMountsWithEqualCapacity(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "tmpfs", Mountpoint: "/var/log", Fstype: "tmpfs"},
			{Device: "tmpfs", Mountpoint: "/mnt/ramdisk/plex-transcode", Fstype: "tmpfs"},
		}, nil
	}
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{
			Path:        path,
			Total:       1024,
			Used:        768,
			Free:        256,
			UsedPercent: 75,
		}, nil
	}

	disks := collectDisksWithIncludes(context.Background(), nil, []string{
		"/var/log",
		"/mnt/ramdisk/plex-transcode",
	})
	if len(disks) != 2 {
		t.Fatalf("distinct explicitly included tmpfs mounts were deduplicated: %+v", disks)
	}
	if disks[0].Mountpoint != "/mnt/ramdisk/plex-transcode" || disks[1].Mountpoint != "/var/log" {
		t.Fatalf("explicit tmpfs mounts = %+v", disks)
	}
}

func TestCollectDisksExplicitExcludeWinsOverInclude(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "log2ram", Mountpoint: "/var/log", Fstype: "tmpfs"},
		}, nil
	}
	diskUsage = func(context.Context, string) (*godisk.UsageStat, error) {
		t.Fatal("excluded mount should not be queried")
		return nil, nil
	}

	disks := collectDisksWithIncludes(context.Background(), []string{"/var/log"}, []string{"/var/log"})
	if len(disks) != 0 {
		t.Fatalf("explicit exclusion should win over include: %+v", disks)
	}
}

func TestCollectSplitsReclaimableCache(t *testing.T) {
	origVirtualMemory, origARC := virtualMemory, readZFSARCSize
	t.Cleanup(func() { virtualMemory, readZFSARCSize = origVirtualMemory, origARC })
	readZFSARCSize = func() (uint64, error) { return 0, nil }

	const gib = uint64(1024 * 1024 * 1024)
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total: 16 * gib,
			Used:  6 * gib,
			Free:  4 * gib,
			// Available - Free = 6 GiB of reclaimable buff/cache.
			Available:   10 * gib,
			UsedPercent: 37.5,
		}, nil
	}

	snapshot, err := Collect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if got, want := snapshot.Memory.CacheBytes, int64(6*gib); got != want {
		t.Fatalf("CacheBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.UsedBytes, int64(6*gib); got != want {
		t.Fatalf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.FreeBytes, int64(4*gib); got != want {
		t.Fatalf("FreeBytes = %d, want %d", got, want)
	}
	sum := snapshot.Memory.UsedBytes + snapshot.Memory.CacheBytes + snapshot.Memory.FreeBytes
	if sum > snapshot.Memory.TotalBytes {
		t.Fatalf("used+cache+free = %d exceeds total %d", sum, snapshot.Memory.TotalBytes)
	}
}

func TestCollectDerivesMemoryPressureFromAvailableMemory(t *testing.T) {
	origVirtualMemory, origARC := virtualMemory, readZFSARCSize
	t.Cleanup(func() { virtualMemory, readZFSARCSize = origVirtualMemory, origARC })
	readZFSARCSize = func() (uint64, error) { return 0, nil }

	const gib = uint64(1024 * 1024 * 1024)
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total: 16 * gib,
			// Some Linux/gopsutil paths can report Used as cache-inclusive. The
			// operator-facing pressure value should follow MemAvailable instead.
			Used:        15 * gib,
			Free:        1 * gib,
			Available:   8 * gib,
			UsedPercent: 93.75,
		}, nil
	}

	snapshot, err := Collect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if got, want := snapshot.Memory.UsedBytes, int64(8*gib); got != want {
		t.Fatalf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.CacheBytes, int64(7*gib); got != want {
		t.Fatalf("CacheBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.FreeBytes, int64(1*gib); got != want {
		t.Fatalf("FreeBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.Usage, 50.0; got != want {
		t.Fatalf("Usage = %.2f, want %.2f", got, want)
	}
	sum := snapshot.Memory.UsedBytes + snapshot.Memory.CacheBytes + snapshot.Memory.FreeBytes
	if got, want := sum, snapshot.Memory.TotalBytes; got != want {
		t.Fatalf("used+cache+free = %d, want total %d", got, want)
	}
}

func TestCollectMovesARCFromUsedToCacheWithoutChangingFree(t *testing.T) {
	origVirtualMemory, origARC := virtualMemory, readZFSARCSize
	t.Cleanup(func() { virtualMemory, readZFSARCSize = origVirtualMemory, origARC })
	const gib = 1 << 30
	virtualMemory = func(context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{Total: 16 * gib, Available: 2 * gib, Free: gib, Used: 14 * gib}, nil
	}
	for _, tc := range []struct {
		name string
		arc  uint64
		used int64
	}{
		{name: "ordinary ARC", arc: 12 * gib, used: 2 * gib},
		{name: "ARC cannot reclassify more than used", arc: 20 * gib},
		{name: "unobserved ARC is not invented", used: 14 * gib},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readZFSARCSize = func() (uint64, error) { return tc.arc, nil }
			snapshot, err := Collect(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			memory := snapshot.Memory
			if memory.TotalBytes != 16*gib || memory.UsedBytes != tc.used || memory.CacheBytes != 15*gib-tc.used || memory.FreeBytes != gib ||
				memory.Usage != float64(tc.used)/(16*gib)*100 {
				t.Fatalf("wrong usage/cache/free partition: %+v", memory)
			}
		})
	}
}

// Exercise the real gopsutil mountinfo parser, not a pre-parsed partition
// stub. The reporter's df/service data establish these filesystem types and
// include paths, but not the actual running service's mountinfo or environment.
// This proves the expected path only; it is not an installed-estate reproduction.
func TestIssue1875MountinfoToExplicitDiskCollection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mountinfo only")
	}
	mountinfo := filepath.Join(t.TempDir(), "mountinfo")
	fixture := "31 1 0:31 / /var/log rw - tmpfs log2ram rw,size=524288k\n" +
		"32 1 0:32 / /mnt/ramdisk/plex-transcode rw - tmpfs tmpfs rw,size=524288k\n" +
		"33 1 0:33 / /run rw - tmpfs tmpfs rw,size=524288k\n"
	if err := os.WriteFile(mountinfo, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), gopsutilcommon.EnvKey, gopsutilcommon.EnvMap{
		gopsutilcommon.HostProcMountinfo: mountinfo,
	})
	originalUsage := diskUsage
	t.Cleanup(func() { diskUsage = originalUsage })
	var queried []string
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		queried = append(queried, path)
		return &godisk.UsageStat{Path: path, Total: 1024, Used: 768, Free: 256, UsedPercent: 75}, nil
	}
	for _, tc := range []struct {
		name                   string
		include, exclude, want []string
	}{
		{name: "no runtime include"},
		{name: "PVE single include", include: []string{"/var/log"}, want: []string{"/var/log"}},
		{name: "LXC last assignment only", include: []string{"/mnt/ramdisk/plex-transcode"}, want: []string{"/mnt/ramdisk/plex-transcode"}},
		{name: "LXC combined include", include: []string{"/var/log", "/mnt/ramdisk/plex-transcode"}, want: []string{"/mnt/ramdisk/plex-transcode", "/var/log"}},
		{name: "explicit exclusion wins", include: []string{"/var/log"}, exclude: []string{"/var/log"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queried = nil
			disks := collectDisksWithIncludes(ctx, tc.exclude, tc.include)
			var got []string
			for _, disk := range disks {
				got = append(got, disk.Mountpoint)
				if !disk.ExplicitlyIncluded || disk.Filesystem != "tmpfs" || disk.TotalBytes != 1024 || disk.Usage != 75 {
					t.Fatalf("incorrect capacity/type: %+v", disk)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mounts = %v, want %v", got, tc.want)
			}
			if len(queried) != len(tc.want) {
				t.Fatalf("usage calls = %v, want only selected mounts %v", queried, tc.want)
			}
			for _, path := range queried {
				if path == "/run" {
					t.Fatal("queried automatically filtered /run")
				}
			}
		})
	}
}

// A Proxmox host bridges its physical port into vmbr0, so eno1 has no address
// of its own. It must still be reported, with its link speed, as inventory,
// while traffic stays on the addressed bridge so the host's network rate
// (the sum of every reported counter) does not double.
func TestCollectNetworkReportsAddresslessPhysicalPortsAsInventory(t *testing.T) {
	origInterfaces, origCounters, origLinkInfo := netInterfaces, netIOCounters, readLinkInfo
	t.Cleanup(func() {
		netInterfaces, netIOCounters, readLinkInfo = origInterfaces, origCounters, origLinkInfo
	})

	netInterfaces = func(context.Context) (gonet.InterfaceStatList, error) {
		return gonet.InterfaceStatList{
			{Name: "lo", Flags: []string{"up", "loopback"}, Addrs: gonet.InterfaceAddrList{{Addr: "127.0.0.1/8"}}},
			{Name: "eno1", HardwareAddr: "aa:bb:cc:00:00:01", Flags: []string{"up"}},
			{Name: "enp2s0", HardwareAddr: "aa:bb:cc:00:00:02"},
			{Name: "enp3s0", HardwareAddr: "aa:bb:cc:00:00:03", Flags: []string{"up"}},
			{Name: "vmbr0", HardwareAddr: "aa:bb:cc:00:00:01", Flags: []string{"up"}, Addrs: gonet.InterfaceAddrList{{Addr: "192.0.2.10/24"}}},
			{Name: "veth100i0", Flags: []string{"up"}},
			{Name: "tap101i0", Flags: []string{"up"}},
		}, nil
	}
	netIOCounters = func(context.Context, bool) ([]gonet.IOCountersStat, error) {
		return []gonet.IOCountersStat{
			{Name: "eno1", BytesRecv: 900, BytesSent: 800},
			{Name: "enp3s0", BytesRecv: 300, BytesSent: 200},
			{Name: "vmbr0", BytesRecv: 900, BytesSent: 800},
			{Name: "veth100i0", BytesRecv: 50, BytesSent: 40},
		}, nil
	}
	readLinkInfo = func(name string) linkInfo {
		switch name {
		case "eno1":
			return linkInfo{physical: true, speedMbps: 2500, linkUp: new(true)}
		case "enp2s0":
			return linkInfo{physical: true, linkUp: new(false)}
		case "enp3s0":
			return linkInfo{physical: true, speedMbps: 1000}
		}
		return linkInfo{}
	}

	got := collectNetwork(context.Background())

	names := make([]string, 0, len(got))
	for _, iface := range got {
		names = append(names, iface.Name)
	}
	if want := []string{"eno1", "enp2s0", "enp3s0", "vmbr0"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("interfaces = %v, want %v", names, want)
	}

	eno1, unplugged, unobserved, bridge := got[0], got[1], got[2], got[3]
	if eno1.SpeedMbps == nil || *eno1.SpeedMbps != 2500 || eno1.MAC != "aa:bb:cc:00:00:01" {
		t.Fatalf("eno1 = %+v, want its MAC at 2500 Mb/s", eno1)
	}
	if eno1.RXBytes != 0 || eno1.TXBytes != 0 || len(eno1.Addresses) != 0 {
		t.Fatalf("eno1 = %+v, want inventory without addresses or counters", eno1)
	}
	if unplugged.SpeedMbps != nil {
		t.Fatalf("unplugged port speed = %d, want none", *unplugged.SpeedMbps)
	}
	if eno1.LinkUp == nil || !*eno1.LinkUp {
		t.Fatalf("eno1 link = %v, want up", eno1.LinkUp)
	}
	if unplugged.LinkUp == nil || *unplugged.LinkUp {
		t.Fatalf("unplugged port link = %v, want down", unplugged.LinkUp)
	}
	if unobserved.LinkUp != nil || unobserved.SpeedMbps == nil || *unobserved.SpeedMbps != 1000 || len(unobserved.Addresses) != 0 {
		t.Fatalf("port without carrier evidence = %+v, want unknown carrier at 1000 Mb/s", unobserved)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var wire []map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire[0]["linkUp"] != true || wire[1]["linkUp"] != false {
		t.Fatalf("observed carrier states missing from report: %s", raw)
	}
	for _, idx := range []int{2, 3} {
		if _, ok := wire[idx]["linkUp"]; ok {
			t.Fatalf("unobserved link state sent for %s: %s", got[idx].Name, raw)
		}
	}
	if bridge.LinkUp != nil {
		t.Fatalf("vmbr0 link = %v, want unknown for a software interface", *bridge.LinkUp)
	}
	if bridge.RXBytes != 900 || bridge.TXBytes != 800 || bridge.SpeedMbps != nil {
		t.Fatalf("vmbr0 = %+v, want its counters and no speed", bridge)
	}

	var rx, tx uint64
	for _, iface := range got {
		rx += iface.RXBytes
		tx += iface.TXBytes
	}
	if rx != 900 || tx != 800 {
		t.Fatalf("summed RX/TX = %d/%d, want 900/800 (bridge traffic counted once)", rx, tx)
	}
}

func TestCollectNetworkReportsSpeedOfAnAddressedPhysicalPort(t *testing.T) {
	origInterfaces, origCounters, origLinkInfo := netInterfaces, netIOCounters, readLinkInfo
	t.Cleanup(func() {
		netInterfaces, netIOCounters, readLinkInfo = origInterfaces, origCounters, origLinkInfo
	})

	netInterfaces = func(context.Context) (gonet.InterfaceStatList, error) {
		return gonet.InterfaceStatList{{Name: "eth0", Addrs: gonet.InterfaceAddrList{{Addr: "192.0.2.20/24"}}}}, nil
	}
	netIOCounters = func(context.Context, bool) ([]gonet.IOCountersStat, error) {
		return []gonet.IOCountersStat{{Name: "eth0", BytesRecv: 7, BytesSent: 8}}, nil
	}
	readLinkInfo = func(string) linkInfo { return linkInfo{physical: true, speedMbps: 10000, linkUp: new(true)} }

	got := collectNetwork(context.Background())
	if len(got) != 1 || got[0].SpeedMbps == nil || *got[0].SpeedMbps != 10000 || got[0].RXBytes != 7 {
		t.Fatalf("interfaces = %+v, want eth0 at 10000 Mb/s with its counters", got)
	}
}

func TestReadPhysicalLinkInfoFromSysfs(t *testing.T) {
	root := t.TempDir()
	port := func(name, speed, carrier, flags string, device bool) {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if device {
			if err := os.Mkdir(filepath.Join(dir, "device"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for file, value := range map[string]string{"speed": speed, "carrier": carrier, "flags": flags} {
			if value == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, file), []byte(value+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	port("eno1", "2500", "1", "", true)
	port("enp3s0", "-1", "0", "", true)   // cable out
	port("enp4s0", "1000", "0", "", true) // stale speed left by the driver
	port("enp5s0", "1000", "", "", true)  // carrier file missing, speed still valid
	port("enp6s0", "2500", "unknown", "0x1003", true)
	port("enp7s0", "10000", "", "0x1003", true)
	port("enp8s0", "1000", "", "0x1002", true) // independently observed admin down
	port("enp9s0", "100", "2", "bad", true)    // neither read establishes link state
	port("wlp2s0", "", "1", "", true)          // associated Wi-Fi has no readable speed
	port("veth100i0", "10000", "1", "", false) // software link with an invented speed
	port("vmbr0", "", "1", "", false)
	for _, name := range []string{"enp7s0", "enp8s0"} {
		// A directory yields a native read error, even when the test runs
		// as a privileged guest. Missing files are covered separately.
		if err := os.Mkdir(filepath.Join(root, name, "carrier"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name string
		want linkInfo
	}{
		{"eno1", linkInfo{physical: true, speedMbps: 2500, linkUp: new(true)}},
		{"enp3s0", linkInfo{physical: true, linkUp: new(false)}},
		{"enp4s0", linkInfo{physical: true, linkUp: new(false)}},
		{"enp5s0", linkInfo{physical: true, speedMbps: 1000}},
		{"enp6s0", linkInfo{physical: true, speedMbps: 2500}},
		{"enp7s0", linkInfo{physical: true, speedMbps: 10000}},
		{"enp8s0", linkInfo{physical: true, linkUp: new(false)}},
		{"enp9s0", linkInfo{physical: true, speedMbps: 100}},
		{"wlp2s0", linkInfo{physical: true, linkUp: new(true)}},
		{"veth100i0", linkInfo{}},
		{"vmbr0", linkInfo{}},
		{"missing0", linkInfo{}},
		{"../eno1", linkInfo{}},
		{"..", linkInfo{}},
	}
	for _, tc := range cases {
		if got := readPhysicalLinkInfoFrom(root, tc.name); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
