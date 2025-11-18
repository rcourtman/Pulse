package hostmetrics

import (
	"context"
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
