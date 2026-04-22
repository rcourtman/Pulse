package models

import "testing"

func TestSummaryDiskPrefersRootMount(t *testing.T) {
	disks := []Disk{
		{Mountpoint: "/boot", Total: 1, Used: 1},
		{Mountpoint: "/", Total: 100, Used: 40, Free: 60, Usage: 40},
		{Mountpoint: "/var", Total: 200, Used: 50},
	}

	got, ok := SummaryDisk(disks)
	if !ok {
		t.Fatal("expected summary disk")
	}
	if got.Mountpoint != "/" {
		t.Fatalf("summary mountpoint = %q, want /", got.Mountpoint)
	}
	if got.Total != 100 || got.Used != 40 {
		t.Fatalf("summary disk = %+v, want root disk", got)
	}
}

func TestSummaryDiskFallsBackToFirstNonZeroDisk(t *testing.T) {
	disks := []Disk{
		{Mountpoint: "", Total: 0},
		{Mountpoint: "/tank", Total: 400, Used: 100, Free: 300, Usage: 25},
		{Mountpoint: "/backup", Total: 800, Used: 200},
	}

	got, ok := SummaryDisk(disks)
	if !ok {
		t.Fatal("expected summary disk")
	}
	if got.Mountpoint != "/tank" {
		t.Fatalf("summary mountpoint = %q, want /tank", got.Mountpoint)
	}
}

func TestSummaryDiskReturnsFalseWhenNoUsableDiskExists(t *testing.T) {
	if _, ok := SummaryDisk(nil); ok {
		t.Fatal("expected no summary disk for nil input")
	}
	if _, ok := SummaryDisk([]Disk{{Mountpoint: "/", Total: 0}}); ok {
		t.Fatal("expected no summary disk for zero-capacity disks")
	}
}

func TestAggregateDiskSumsVisibleDisks(t *testing.T) {
	disks := []Disk{
		{Mountpoint: "/", Total: 100, Used: 40},
		{Mountpoint: "/mnt/datastore/repo-a", Total: 300, Used: 120},
		{Mountpoint: "/mnt/datastore/repo-b", Total: 600, Used: 360},
	}

	got, ok := AggregateDisk(disks)
	if !ok {
		t.Fatal("expected aggregate disk")
	}
	if got.Total != 1000 || got.Used != 520 || got.Free != 480 {
		t.Fatalf("aggregate disk = %+v, want total=1000 used=520 free=480", got)
	}
	if got.Usage != 52 {
		t.Fatalf("aggregate usage = %v, want 52", got.Usage)
	}
}

func TestAggregateDiskIgnoresZeroCapacityEntries(t *testing.T) {
	got, ok := AggregateDisk([]Disk{
		{Mountpoint: "/boot", Total: 0, Used: 0},
		{Mountpoint: "/", Total: 100, Used: 25},
	})
	if !ok {
		t.Fatal("expected aggregate disk")
	}
	if got.Total != 100 || got.Used != 25 || got.Free != 75 {
		t.Fatalf("aggregate disk = %+v, want total=100 used=25 free=75", got)
	}
}

func TestAggregateDiskReturnsFalseWhenNoUsableDiskExists(t *testing.T) {
	if _, ok := AggregateDisk(nil); ok {
		t.Fatal("expected no aggregate disk for nil input")
	}
	if _, ok := AggregateDisk([]Disk{{Mountpoint: "/", Total: 0}}); ok {
		t.Fatal("expected no aggregate disk for zero-capacity disks")
	}
}
