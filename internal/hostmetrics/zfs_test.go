package hostmetrics

import (
	"context"
	"os"
	"testing"
)

func TestSummarizeZFSPoolsUsesZpoolStats(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"bpool": {Size: 1000, Alloc: 400, Free: 600},
			"tank":  {Size: 5000, Alloc: 2000, Free: 3000},
		}, nil
	}

	datasets := []zfsDatasetUsage{
		{Pool: "tank", Dataset: "tank", Mountpoint: "/mnt/tank", Total: 5000, Used: 2000, Free: 3000},
		{Pool: "tank", Dataset: "tank/home", Mountpoint: "/mnt/tank/home", Total: 4500, Used: 1500, Free: 3000},
		{Pool: "bpool", Dataset: "bpool/ROOT/debian", Mountpoint: "/boot", Total: 800, Used: 200, Free: 600},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}

	if disks[0].Device != "bpool" || disks[1].Device != "tank" {
		t.Fatalf("unexpected disk order: %+v", disks)
	}

	tank := disks[1]
	if tank.Mountpoint != "/mnt/tank" {
		t.Errorf("expected tank mountpoint /mnt/tank, got %s", tank.Mountpoint)
	}
	if tank.TotalBytes != 5000 || tank.UsedBytes != 2000 || tank.FreeBytes != 3000 {
		t.Errorf("unexpected tank capacity %+v", tank)
	}
	if tank.Usage < 39.9 || tank.Usage > 40.1 {
		t.Errorf("expected tank usage ~40%%, got %.2f", tank.Usage)
	}
}

// TestSummarizeZFSPoolsRAIDZCapacity verifies that RAIDZ pools show usable capacity
// (from dataset stats) rather than raw capacity (from zpool list SIZE). Issue #1052.
// The usable total is derived from: zpoolSize * (dsFree / zpoolFree).
// Used is derived from: Total - Free, which captures all pool consumers including zvols.
func TestSummarizeZFSPoolsRAIDZCapacity(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	// Simulate a RAIDZ1 pool with 3 disks:
	// - Raw SIZE from zpool list: 43.6 TB (sum of all disks)
	// - Usable capacity from statfs: ~29 TB (after RAIDZ1 parity overhead)
	// - zpool FREE: 43.593 TB (raw)
	// - dataset Free (statfs): 28.9954 TB (usable)
	// The ratio dsFree/zpoolFree ≈ 0.6653, giving usable total ≈ 29 TB.
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"Main": {Size: 43600000000000, Alloc: 7000000000, Free: 43593000000000},
		}, nil
	}

	datasets := []zfsDatasetUsage{
		{Pool: "Main", Dataset: "Main", Mountpoint: "/mnt/Main", Total: 29000000000000, Used: 4600000000, Free: 28995400000000},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}

	main := disks[0]
	if main.Device != "Main" {
		t.Errorf("expected device Main, got %s", main.Device)
	}

	// Total should be usable capacity (~29 TB), not raw (43.6 TB).
	// Computed as zpoolSize * (dsFree / zpoolFree) = 43.6T * (28.9954T / 43.593T) ≈ 29T.
	// Allow 0.1% tolerance for floating-point rounding.
	if !withinPercent(main.TotalBytes, 29000000000000, 0.1) {
		t.Errorf("expected TotalBytes ~29 TB (usable capacity), got %d (might be using raw capacity)", main.TotalBytes)
	}

	// Free should be dataset free (pool available)
	expectedFree := int64(28995400000000)
	if main.FreeBytes != expectedFree {
		t.Errorf("expected FreeBytes %d, got %d", expectedFree, main.FreeBytes)
	}

	// Used = Total - Free, should be close to actual data usage (~4.6 GB).
	// Small deviation is expected due to the ratio approximation.
	if !withinPercent(main.UsedBytes, 4600000000, 2.0) {
		t.Errorf("expected UsedBytes ~4.6 GB, got %d", main.UsedBytes)
	}

	// Usage should be near 0% (tiny data on a 29 TB pool)
	if main.Usage > 0.1 {
		t.Errorf("expected usage ~0%%, got %.2f%%", main.Usage)
	}
}

// TestSummarizeZFSPoolsMirrorWithZvols verifies that mirror pools correctly
// report total pool usage including zvols (VM disk images on Proxmox).
// Previously, Used only reflected mounted dataset usage, missing zvols entirely.
func TestSummarizeZFSPoolsMirrorWithZvols(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	// Simulate a Proxmox mirror pool (2x 8TB disks):
	// - zpool SIZE: 8 TB (usable, one side of mirror)
	// - zpool ALLOC: 2.5 TB (OS data + VM zvols)
	// - zpool FREE: 5.5 TB
	// The OS root dataset (rpool/ROOT/pve-1) only has 3 GB of files,
	// but zvols under rpool/data hold 2.497 TB of VM disks.
	// statfs on the root dataset sees: Used=3GB, Free=5.5TB (pool available).
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"rpool": {Size: 8000000000000, Alloc: 2500000000000, Free: 5500000000000},
		}, nil
	}

	// statfs on rpool/ROOT/pve-1: only sees 3 GB used by the OS.
	// Free = pool available = 5.5 TB. Total = 3GB + 5.5TB ≈ 5.5TB.
	datasets := []zfsDatasetUsage{
		{Pool: "rpool", Dataset: "rpool/ROOT/pve-1", Mountpoint: "/", Total: 5503000000000, Used: 3000000000, Free: 5500000000000},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}

	rpool := disks[0]

	// Total should be pool usable capacity (8 TB for mirror).
	// Computed as zpoolSize * (dsFree / zpoolFree) = 8T * (5.5T / 5.5T) = 8T.
	if !withinPercent(rpool.TotalBytes, 8000000000000, 0.1) {
		t.Errorf("expected TotalBytes ~8 TB, got %d", rpool.TotalBytes)
	}

	// Free should be pool available (5.5 TB)
	expectedFree := int64(5500000000000)
	if rpool.FreeBytes != expectedFree {
		t.Errorf("expected FreeBytes %d, got %d", expectedFree, rpool.FreeBytes)
	}

	// Used = Total - Free = 8T - 5.5T = 2.5 TB (includes zvols!)
	// Previously this would have been 3 GB (just OS files), missing the zvols entirely.
	if !withinPercent(rpool.UsedBytes, 2500000000000, 0.1) {
		t.Errorf("expected UsedBytes ~2.5 TB (including zvols), got %d (might only be showing OS dataset usage)", rpool.UsedBytes)
	}

	// Usage should be ~31.25% (2.5 TB / 8 TB)
	if rpool.Usage < 30.0 || rpool.Usage > 33.0 {
		t.Errorf("expected usage ~31%%, got %.2f%%", rpool.Usage)
	}
}

// TestSummarizeZFSPoolsRAIDZWithZvols verifies that RAIDZ pools with zvols
// (like Proxmox VM disks) report correct usable used including zvol space.
func TestSummarizeZFSPoolsRAIDZWithZvols(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	// Simulate a RAIDZ1 pool with 4x 10TB disks on Proxmox:
	// - Raw SIZE: 40 TB (sum of disks)
	// - Usable capacity: 30 TB (3/4 for RAIDZ1)
	// - Raw FREE: 26.667 TB
	// - Usable FREE: 20 TB (pool available)
	// - Actual usable used: 10 TB (OS data + VM zvols)
	// - Raw ALLOC: 13.333 TB (includes parity)
	//
	// The root dataset sees: Used=5GB (OS files), Free=20TB (pool available)
	// But 10 TB of usable space is consumed (5GB OS + ~10TB zvols).
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"tank": {Size: 40000000000000, Alloc: 13333000000000, Free: 26667000000000},
		}, nil
	}

	datasets := []zfsDatasetUsage{
		{Pool: "tank", Dataset: "tank/ROOT/pve-1", Mountpoint: "/", Total: 20005000000000, Used: 5000000000, Free: 20000000000000},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}

	tank := disks[0]

	// Total should be ~30 TB usable (not 40 TB raw)
	// zpoolSize * (dsFree / zpoolFree) = 40T * (20T / 26.667T) = 40T * 0.75 = 30T
	if !withinPercent(tank.TotalBytes, 30000000000000, 0.1) {
		t.Errorf("expected TotalBytes ~30 TB (usable), got %d", tank.TotalBytes)
	}

	// Free should be 20 TB (pool available)
	if tank.FreeBytes != 20000000000000 {
		t.Errorf("expected FreeBytes 20 TB, got %d", tank.FreeBytes)
	}

	// Used = 30T - 20T = 10 TB (includes zvols!)
	// Previously would have shown 5 GB (just OS files)
	if !withinPercent(tank.UsedBytes, 10000000000000, 0.1) {
		t.Errorf("expected UsedBytes ~10 TB (including zvols), got %d", tank.UsedBytes)
	}

	// Usage should be ~33.3% (10 TB / 30 TB)
	if tank.Usage < 32.0 || tank.Usage > 35.0 {
		t.Errorf("expected usage ~33%%, got %.2f%%", tank.Usage)
	}
}

func TestSummarizeZFSPoolsFallback(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return nil, context.DeadlineExceeded
	}

	datasets := []zfsDatasetUsage{
		{Pool: "tank", Dataset: "tank", Mountpoint: "/mnt/tank", Total: 5000, Used: 2000, Free: 3000},
		{Pool: "tank", Dataset: "tank/home", Mountpoint: "/mnt/tank/home", Total: 4500, Used: 1500, Free: 3000},
		{Pool: "bpool", Dataset: "bpool/ROOT/debian", Mountpoint: "/boot", Total: 800, Used: 200, Free: 600},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}

	tank := disks[1]
	if tank.TotalBytes != 5000 || tank.UsedBytes != 2000 || tank.FreeBytes != 3000 {
		t.Errorf("tank fallback totals incorrect: %+v", tank)
	}
	if tank.Usage < 39.9 || tank.Usage > 40.1 {
		t.Errorf("expected tank usage ~40%%, got %.2f", tank.Usage)
	}
}

func TestFindZpool(t *testing.T) {
	// This test verifies that findZpool correctly:
	// 1. Prefers common absolute paths
	// 2. Falls back to PATH lookup for non-standard installs

	// We can't easily mock os.Stat, so we just verify the function
	// returns a path (if zpool is installed) or an error (if not)
	path, err := findZpool()

	// On a system without zpool, we expect an error
	if err != nil {
		if path != "" {
			t.Errorf("findZpool() returned path %q with error: %v", path, err)
		}
		// Expected behavior on systems without zpool
		return
	}

	// On a system with zpool, verify the path looks valid
	if path == "" {
		t.Error("findZpool() returned empty path without error")
	}

	// Verify the returned path exists
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("findZpool() returned path %q but os.Stat failed: %v", path, statErr)
	}
}

func TestFindZpoolPrefersCommonPaths(t *testing.T) {
	origCommon := commonZpoolPaths
	origLookPath := zpoolLookPath
	origStat := zpoolStat
	t.Cleanup(func() {
		commonZpoolPaths = origCommon
		zpoolLookPath = origLookPath
		zpoolStat = origStat
	})

	commonZpoolPaths = []string{"/trusted/zpool", "/secondary/zpool"}
	zpoolLookPath = func(file string) (string, error) {
		return "/tmp/attacker/zpool", nil
	}
	zpoolStat = func(name string) (os.FileInfo, error) {
		if name == "/trusted/zpool" {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}

	path, err := findZpool()
	if err != nil {
		t.Fatalf("findZpool() returned error: %v", err)
	}
	if path != "/trusted/zpool" {
		t.Fatalf("findZpool() = %q, want /trusted/zpool", path)
	}
}

func TestFindZpoolRejectsRelativePathLookup(t *testing.T) {
	origCommon := commonZpoolPaths
	origLookPath := zpoolLookPath
	origStat := zpoolStat
	t.Cleanup(func() {
		commonZpoolPaths = origCommon
		zpoolLookPath = origLookPath
		zpoolStat = origStat
	})

	commonZpoolPaths = []string{"/missing/zpool"}
	zpoolLookPath = func(file string) (string, error) {
		return "bin/zpool", nil
	}
	zpoolStat = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	path, err := findZpool()
	if err == nil {
		t.Fatalf("findZpool() expected error for relative path, got path %q", path)
	}
}

func TestCommonZpoolPaths(t *testing.T) {
	// Verify that commonZpoolPaths contains expected paths for TrueNAS
	expectedPaths := []string{
		"/usr/sbin/zpool",       // TrueNAS SCALE, Debian, Ubuntu
		"/sbin/zpool",           // FreeBSD, older Linux
		"/usr/local/sbin/zpool", // FreeBSD ports
	}

	for _, expected := range expectedPaths {
		found := false
		for _, path := range commonZpoolPaths {
			if path == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("commonZpoolPaths missing expected path: %s", expected)
		}
	}
}

func TestZFSPoolFromDevice(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{device: "tank/ROOT/default", want: "tank"},
		{device: "vault", want: "vault"},
		{device: "", want: ""},
	}

	for _, tt := range tests {
		got := zfsPoolFromDevice(tt.device)
		if got != tt.want {
			t.Errorf("zfsPoolFromDevice(%q) = %q, want %q", tt.device, got, tt.want)
		}
	}
}

func TestParseZpoolList(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPools   []string
		wantStats   map[string]zpoolStats
		wantErr     bool
		errContains string
	}{
		{
			name:      "single pool",
			input:     "rpool\t1000000000\t500000000\t500000000\n",
			wantPools: []string{"rpool"},
			wantStats: map[string]zpoolStats{
				"rpool": {Size: 1000000000, Alloc: 500000000, Free: 500000000},
			},
		},
		{
			name: "multiple pools",
			input: "rpool\t1000000000\t500000000\t500000000\n" +
				"tank\t2000000000\t1000000000\t1000000000\n",
			wantPools: []string{"rpool", "tank"},
			wantStats: map[string]zpoolStats{
				"rpool": {Size: 1000000000, Alloc: 500000000, Free: 500000000},
				"tank":  {Size: 2000000000, Alloc: 1000000000, Free: 1000000000},
			},
		},
		{
			name:        "empty output",
			input:       "",
			wantErr:     true,
			errContains: "no usable data",
		},
		{
			name:        "whitespace only",
			input:       "   \n\t\n  ",
			wantErr:     true,
			errContains: "no usable data",
		},
		{
			name:      "pool with zero usage",
			input:     "empty\t1000000000\t0\t1000000000\n",
			wantPools: []string{"empty"},
			wantStats: map[string]zpoolStats{
				"empty": {Size: 1000000000, Alloc: 0, Free: 1000000000},
			},
		},
		{
			name:      "pool fully used",
			input:     "full\t1000000000\t1000000000\t0\n",
			wantPools: []string{"full"},
			wantStats: map[string]zpoolStats{
				"full": {Size: 1000000000, Alloc: 1000000000, Free: 0},
			},
		},
		{
			name:        "skip malformed line too few fields",
			input:       "rpool\t1000\n",
			wantErr:     true,
			errContains: "no usable data",
		},
		{
			name: "skip line with invalid size",
			input: "bad\tnotanumber\t500\t500\n" +
				"good\t1000\t500\t500\n",
			wantPools: []string{"good"},
			wantStats: map[string]zpoolStats{
				"good": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name: "skip line with invalid alloc",
			input: "bad\t1000\tnotanumber\t500\n" +
				"good\t1000\t500\t500\n",
			wantPools: []string{"good"},
			wantStats: map[string]zpoolStats{
				"good": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name: "skip line with invalid free",
			input: "bad\t1000\t500\tnotanumber\n" +
				"good\t1000\t500\t500\n",
			wantPools: []string{"good"},
			wantStats: map[string]zpoolStats{
				"good": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name:      "large values near uint64 max",
			input:     "huge\t18446744073709551615\t9223372036854775808\t9223372036854775807\n",
			wantPools: []string{"huge"},
			wantStats: map[string]zpoolStats{
				"huge": {Size: 18446744073709551615, Alloc: 9223372036854775808, Free: 9223372036854775807},
			},
		},
		{
			name:      "extra fields ignored",
			input:     "pool\t1000\t500\t500\textra\tmore\n",
			wantPools: []string{"pool"},
			wantStats: map[string]zpoolStats{
				"pool": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name:      "pool name with hyphen",
			input:     "my-pool\t1000\t500\t500\n",
			wantPools: []string{"my-pool"},
			wantStats: map[string]zpoolStats{
				"my-pool": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name:      "pool name with underscore",
			input:     "my_pool\t1000\t500\t500\n",
			wantPools: []string{"my_pool"},
			wantStats: map[string]zpoolStats{
				"my_pool": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name: "skip line with unsafe pool name",
			input: "-unsafe\t1000\t500\t500\n" +
				"safe\t1000\t500\t500\n",
			wantPools: []string{"safe"},
			wantStats: map[string]zpoolStats{
				"safe": {Size: 1000, Alloc: 500, Free: 500},
			},
		},
		{
			name: "blank lines interspersed",
			input: "\npool1\t1000\t500\t500\n\n" +
				"pool2\t2000\t1000\t1000\n\n",
			wantPools: []string{"pool1", "pool2"},
			wantStats: map[string]zpoolStats{
				"pool1": {Size: 1000, Alloc: 500, Free: 500},
				"pool2": {Size: 2000, Alloc: 1000, Free: 1000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseZpoolList([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseZpoolList() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsSubstr(err.Error(), tt.errContains) {
					t.Errorf("parseZpoolList() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("parseZpoolList() unexpected error = %v", err)
				return
			}

			for _, pool := range tt.wantPools {
				stat, ok := got[pool]
				if !ok {
					t.Errorf("parseZpoolList() missing pool %q", pool)
					continue
				}
				want := tt.wantStats[pool]
				if stat != want {
					t.Errorf("parseZpoolList() pool %q = %+v, want %+v", pool, stat, want)
				}
			}

			if len(got) != len(tt.wantPools) {
				t.Errorf("parseZpoolList() got %d pools, want %d", len(got), len(tt.wantPools))
			}
		})
	}
}

func TestUniqueZFSPools(t *testing.T) {
	tests := []struct {
		name     string
		datasets []zfsDatasetUsage
		want     []string
	}{
		{
			name:     "empty input",
			datasets: nil,
			want:     nil,
		},
		{
			name:     "empty slice",
			datasets: []zfsDatasetUsage{},
			want:     nil,
		},
		{
			name: "single pool single dataset",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
			},
			want: []string{"tank"},
		},
		{
			name: "single pool multiple datasets",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
				{Pool: "tank", Dataset: "tank/data", Mountpoint: "/tank/data"},
				{Pool: "tank", Dataset: "tank/home", Mountpoint: "/home"},
			},
			want: []string{"tank"},
		},
		{
			name: "multiple pools",
			datasets: []zfsDatasetUsage{
				{Pool: "rpool", Dataset: "rpool", Mountpoint: "/"},
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
				{Pool: "backup", Dataset: "backup", Mountpoint: "/backup"},
			},
			want: []string{"backup", "rpool", "tank"}, // sorted
		},
		{
			name: "skip empty pool names",
			datasets: []zfsDatasetUsage{
				{Pool: "", Dataset: "orphan", Mountpoint: "/orphan"},
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
			},
			want: []string{"tank"},
		},
		{
			name: "skip unsafe pool names",
			datasets: []zfsDatasetUsage{
				{Pool: "-unsafe", Dataset: "unsafe/root", Mountpoint: "/unsafe"},
				{Pool: "bad name", Dataset: "bad/root", Mountpoint: "/bad"},
				{Pool: "safe_pool", Dataset: "safe_pool/root", Mountpoint: "/safe"},
			},
			want: []string{"safe_pool"},
		},
		{
			name: "all empty pool names",
			datasets: []zfsDatasetUsage{
				{Pool: "", Dataset: "orphan1", Mountpoint: "/orphan1"},
				{Pool: "", Dataset: "orphan2", Mountpoint: "/orphan2"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueZFSPools(tt.datasets)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("uniqueZFSPools() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBestZFSMountpoints(t *testing.T) {
	tests := []struct {
		name     string
		datasets []zfsDatasetUsage
		want     map[string]string
	}{
		{
			name:     "empty input",
			datasets: nil,
			want:     map[string]string{},
		},
		{
			name: "single dataset",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
			},
			want: map[string]string{"tank": "/tank"},
		},
		{
			name: "prefer root dataset over child",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank/data", Mountpoint: "/tank/data"},
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
			},
			want: map[string]string{"tank": "/tank"},
		},
		{
			name: "prefer shallower mountpoint",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank/a/b/c", Mountpoint: "/a/b/c/d"},
				{Pool: "tank", Dataset: "tank/x", Mountpoint: "/x"},
			},
			want: map[string]string{"tank": "/x"},
		},
		{
			name: "root mountpoint preferred",
			datasets: []zfsDatasetUsage{
				{Pool: "rpool", Dataset: "rpool/ROOT/ubuntu", Mountpoint: "/"},
				{Pool: "rpool", Dataset: "rpool/home", Mountpoint: "/home"},
			},
			want: map[string]string{"rpool": "/"},
		},
		{
			name: "skip empty pool",
			datasets: []zfsDatasetUsage{
				{Pool: "", Dataset: "orphan", Mountpoint: "/orphan"},
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
			},
			want: map[string]string{"tank": "/tank"},
		},
		{
			name: "skip empty mountpoint",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank", Mountpoint: ""},
				{Pool: "tank", Dataset: "tank/data", Mountpoint: "/data"},
			},
			want: map[string]string{"tank": "/data"},
		},
		{
			name: "multiple pools",
			datasets: []zfsDatasetUsage{
				{Pool: "rpool", Dataset: "rpool", Mountpoint: "/"},
				{Pool: "tank", Dataset: "tank", Mountpoint: "/tank"},
				{Pool: "backup", Dataset: "backup/daily", Mountpoint: "/backup/daily"},
			},
			want: map[string]string{
				"rpool":  "/",
				"tank":   "/tank",
				"backup": "/backup/daily",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestZFSMountpoints(tt.datasets)
			if len(got) != len(tt.want) {
				t.Errorf("bestZFSMountpoints() got %d entries, want %d", len(got), len(tt.want))
			}
			for pool, wantMount := range tt.want {
				gotMount, ok := got[pool]
				if !ok {
					t.Errorf("bestZFSMountpoints() missing pool %q", pool)
					continue
				}
				if gotMount != wantMount {
					t.Errorf("bestZFSMountpoints()[%q] = %q, want %q", pool, gotMount, wantMount)
				}
			}
		})
	}
}

func TestZfsMountpointScore(t *testing.T) {
	tests := []struct {
		name string
		ds   zfsDatasetUsage
		want int
	}{
		{
			name: "root dataset gets score 0",
			ds:   zfsDatasetUsage{Dataset: "tank", Mountpoint: "/tank"},
			want: 0,
		},
		{
			name: "child dataset at root mountpoint",
			ds:   zfsDatasetUsage{Dataset: "rpool/ROOT/ubuntu", Mountpoint: "/"},
			want: 1, // path empty after trim("/"), returns 1
		},
		{
			name: "child dataset shallow path",
			ds:   zfsDatasetUsage{Dataset: "tank/data", Mountpoint: "/data"},
			want: 1, // path="data" has 0 slashes, 1+0=1
		},
		{
			name: "child dataset deep path",
			ds:   zfsDatasetUsage{Dataset: "tank/a/b/c", Mountpoint: "/a/b/c"},
			want: 3, // path="a/b/c" has 2 slashes, 1+2=3
		},
		{
			name: "empty dataset falls through to path scoring",
			ds:   zfsDatasetUsage{Dataset: "", Mountpoint: "/tank"},
			want: 1, // empty dataset passes first check, path="tank" has 0 slashes, 1+0=1
		},
		{
			name: "trailing slash stripped from mountpoint",
			ds:   zfsDatasetUsage{Dataset: "tank/data", Mountpoint: "/data/"},
			want: 1, // path="data" has 0 slashes, 1+0=1
		},
		{
			name: "very deep mountpoint",
			ds:   zfsDatasetUsage{Dataset: "tank/deep", Mountpoint: "/a/b/c/d/e"},
			want: 5, // path="a/b/c/d/e" has 4 slashes, 1+4=5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zfsMountpointScore(tt.ds)
			if got != tt.want {
				t.Errorf("zfsMountpointScore(%+v) = %d, want %d", tt.ds, got, tt.want)
			}
		})
	}
}

func TestZfsPoolFromDeviceExtended(t *testing.T) {
	tests := []struct {
		name   string
		device string
		want   string
	}{
		{
			name:   "whitespace only",
			device: "   ",
			want:   "",
		},
		{
			name:   "pool with leading space",
			device: "  tank/data",
			want:   "tank",
		},
		{
			name:   "pool with trailing space",
			device: "tank/data  ",
			want:   "tank",
		},
		{
			name:   "pool with hyphen",
			device: "my-pool/data",
			want:   "my-pool",
		},
		{
			name:   "pool with underscore",
			device: "my_pool/data",
			want:   "my_pool",
		},
		{
			name:   "deeply nested dataset",
			device: "rpool/ROOT/ubuntu/home/user",
			want:   "rpool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zfsPoolFromDevice(tt.device)
			if got != tt.want {
				t.Errorf("zfsPoolFromDevice(%q) = %q, want %q", tt.device, got, tt.want)
			}
		})
	}
}

func TestNormalizeZFSPoolName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "trimmed valid", input: " tank ", want: "tank", ok: true},
		{name: "empty invalid", input: "   ", ok: false},
		{name: "leading dash invalid", input: "-tank", ok: false},
		{name: "contains slash invalid", input: "tank/root", ok: false},
		{name: "contains spaces invalid", input: "tank root", ok: false},
		{name: "allowed punctuation", input: "pool_1-2.3:4", want: "pool_1-2.3:4", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeZFSPoolName(tt.input)
			if ok != tt.ok {
				t.Fatalf("normalizeZFSPoolName(%q) ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("normalizeZFSPoolName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterValidZFSPoolNames(t *testing.T) {
	input := []string{" tank ", "tank", "-unsafe", "bad name", "safe_pool"}
	got := filterValidZFSPoolNames(input)
	want := []string{"tank", "safe_pool"}
	if !stringSliceEqual(got, want) {
		t.Fatalf("filterValidZFSPoolNames() = %v, want %v", got, want)
	}
}

func TestFetchZpoolStatsRejectsUnsafePoolNames(t *testing.T) {
	stats, err := fetchZpoolStats(context.Background(), []string{"-unsafe", "bad name", "   "})
	if err == nil {
		t.Fatal("fetchZpoolStats() expected error for invalid pool list, got nil")
	}
	if !containsSubstr(err.Error(), "no valid zfs pool names") {
		t.Fatalf("fetchZpoolStats() error = %q, want no valid zfs pool names", err.Error())
	}
	if stats != nil {
		t.Fatalf("fetchZpoolStats() stats = %v, want nil", stats)
	}
}

func TestFetchZpoolStatsRejectsOversizedCommandOutput(t *testing.T) {
	origCommon := commonZpoolPaths
	origStat := zpoolStat
	origRunner := zpoolCommandRunner
	t.Cleanup(func() {
		commonZpoolPaths = origCommon
		zpoolStat = origStat
		zpoolCommandRunner = origRunner
	})

	commonZpoolPaths = []string{"/trusted/zpool"}
	zpoolStat = func(name string) (os.FileInfo, error) {
		if name == "/trusted/zpool" {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}
	zpoolCommandRunner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, errZpoolCommandOutputTooLarge
	}

	stats, err := fetchZpoolStats(context.Background(), []string{"tank"})
	if err == nil {
		t.Fatal("fetchZpoolStats() expected oversized output error, got nil")
	}
	if !containsSubstr(err.Error(), "output exceeded") {
		t.Fatalf("fetchZpoolStats() error = %q, want output exceeded", err.Error())
	}
	if stats != nil {
		t.Fatalf("fetchZpoolStats() stats = %v, want nil", stats)
	}
}

func TestCalculatePercent(t *testing.T) {
	tests := []struct {
		name  string
		total uint64
		used  uint64
		want  float64
	}{
		{
			name:  "zero total returns zero",
			total: 0,
			used:  0,
			want:  0,
		},
		{
			name:  "zero used returns zero percent",
			total: 1000,
			used:  0,
			want:  0,
		},
		{
			name:  "half used",
			total: 1000,
			used:  500,
			want:  50,
		},
		{
			name:  "fully used",
			total: 1000,
			used:  1000,
			want:  100,
		},
		{
			name:  "over 100 percent allowed",
			total: 1000,
			used:  1500,
			want:  150,
		},
		{
			name:  "small percentage",
			total: 1000,
			used:  1,
			want:  0.1,
		},
		{
			name:  "large values",
			total: 10000000000000,
			used:  7500000000000,
			want:  75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePercent(tt.total, tt.used)
			if got != tt.want {
				t.Errorf("calculatePercent(%d, %d) = %v, want %v", tt.total, tt.used, got, tt.want)
			}
		})
	}
}

func TestClampPercent(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{
			name:  "zero",
			value: 0,
			want:  0,
		},
		{
			name:  "negative clamped to zero",
			value: -10,
			want:  0,
		},
		{
			name:  "small negative clamped to zero",
			value: -0.001,
			want:  0,
		},
		{
			name:  "mid range unchanged",
			value: 50,
			want:  50,
		},
		{
			name:  "exactly 100",
			value: 100,
			want:  100,
		},
		{
			name:  "over 100 clamped",
			value: 150,
			want:  100,
		},
		{
			name:  "slightly over 100 clamped",
			value: 100.001,
			want:  100,
		},
		{
			name:  "decimal value unchanged",
			value: 75.5,
			want:  75.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampPercent(tt.value)
			if got != tt.want {
				t.Errorf("clampPercent(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBestZFSPoolDatasets(t *testing.T) {
	tests := []struct {
		name     string
		datasets []zfsDatasetUsage
		want     map[string]zfsDatasetUsage
	}{
		{
			name:     "empty input",
			datasets: nil,
			want:     map[string]zfsDatasetUsage{},
		},
		{
			name: "single dataset",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
			},
			want: map[string]zfsDatasetUsage{
				"tank": {Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
			},
		},
		{
			name: "prefer larger total",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank/small", Total: 500, Used: 250},
				{Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
			},
			want: map[string]zfsDatasetUsage{
				"tank": {Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
			},
		},
		{
			name: "skip empty pool names",
			datasets: []zfsDatasetUsage{
				{Pool: "", Dataset: "orphan", Total: 1000, Used: 500},
				{Pool: "tank", Dataset: "tank", Total: 2000, Used: 1000},
			},
			want: map[string]zfsDatasetUsage{
				"tank": {Pool: "tank", Dataset: "tank", Total: 2000, Used: 1000},
			},
		},
		{
			name: "multiple pools",
			datasets: []zfsDatasetUsage{
				{Pool: "rpool", Dataset: "rpool", Total: 100, Used: 50},
				{Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
				{Pool: "backup", Dataset: "backup", Total: 500, Used: 100},
			},
			want: map[string]zfsDatasetUsage{
				"rpool":  {Pool: "rpool", Dataset: "rpool", Total: 100, Used: 50},
				"tank":   {Pool: "tank", Dataset: "tank", Total: 1000, Used: 500},
				"backup": {Pool: "backup", Dataset: "backup", Total: 500, Used: 100},
			},
		},
		{
			name: "same pool multiple datasets picks largest",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank/a", Total: 300, Used: 100},
				{Pool: "tank", Dataset: "tank/b", Total: 800, Used: 400},
				{Pool: "tank", Dataset: "tank/c", Total: 500, Used: 200},
			},
			want: map[string]zfsDatasetUsage{
				"tank": {Pool: "tank", Dataset: "tank/b", Total: 800, Used: 400},
			},
		},
		{
			name: "equal totals keeps first seen",
			datasets: []zfsDatasetUsage{
				{Pool: "tank", Dataset: "tank/first", Total: 1000, Used: 500},
				{Pool: "tank", Dataset: "tank/second", Total: 1000, Used: 600},
			},
			want: map[string]zfsDatasetUsage{
				"tank": {Pool: "tank", Dataset: "tank/first", Total: 1000, Used: 500},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestZFSPoolDatasets(tt.datasets)
			if len(got) != len(tt.want) {
				t.Errorf("bestZFSPoolDatasets() got %d entries, want %d", len(got), len(tt.want))
			}
			for pool, wantDS := range tt.want {
				gotDS, ok := got[pool]
				if !ok {
					t.Errorf("bestZFSPoolDatasets() missing pool %q", pool)
					continue
				}
				if gotDS != wantDS {
					t.Errorf("bestZFSPoolDatasets()[%q] = %+v, want %+v", pool, gotDS, wantDS)
				}
			}
		})
	}
}

// Helper functions

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// withinPercent checks if got is within pct% of want.
func withinPercent(got, want int64, pct float64) bool {
	if want == 0 {
		return got == 0
	}
	diff := float64(got-want) / float64(want) * 100
	if diff < 0 {
		diff = -diff
	}
	return diff <= pct
}
