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
func TestSummarizeZFSPoolsRAIDZCapacity(t *testing.T) {
	originalQuery := queryZpoolStats
	t.Cleanup(func() { queryZpoolStats = originalQuery })

	// Simulate a RAIDZ1 pool with 3 disks:
	// - Raw SIZE from zpool list: 43.6 TB (sum of all disks)
	// - Usable capacity from statfs: 29 TB (after RAIDZ1 parity overhead)
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"Main": {Size: 43600000000000, Alloc: 962000000, Free: 43599038000000},
		}, nil
	}

	// Dataset stats from statfs reflect usable capacity (29 TB)
	datasets := []zfsDatasetUsage{
		{Pool: "Main", Dataset: "Main", Mountpoint: "/mnt/Main", Total: 29000000000000, Used: 962000000, Free: 28999038000000},
	}

	disks := summarizeZFSPools(context.Background(), datasets)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}

	main := disks[0]
	if main.Device != "Main" {
		t.Errorf("expected device Main, got %s", main.Device)
	}

	// Should use usable capacity (29 TB), not raw capacity (43.6 TB)
	expectedTotal := int64(29000000000000)
	if main.TotalBytes != expectedTotal {
		t.Errorf("expected TotalBytes %d (usable capacity), got %d (might be using raw capacity)", expectedTotal, main.TotalBytes)
	}

	// Used should come from zpool stats (accurate allocation)
	expectedUsed := int64(962000000)
	if main.UsedBytes != expectedUsed {
		t.Errorf("expected UsedBytes %d, got %d", expectedUsed, main.UsedBytes)
	}

	// Free should use dataset stats when we're using dataset Total
	expectedFree := int64(28999038000000)
	if main.FreeBytes != expectedFree {
		t.Errorf("expected FreeBytes %d, got %d", expectedFree, main.FreeBytes)
	}

	// Usage should be calculated against usable capacity
	// 962000000 / 29000000000000 * 100 ≈ 0.003%
	if main.Usage > 0.1 {
		t.Errorf("expected usage ~0%%, got %.2f%% (might be calculated against wrong total)", main.Usage)
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
	// 1. Uses exec.LookPath first (if zpool is in PATH)
	// 2. Falls back to common absolute paths

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
