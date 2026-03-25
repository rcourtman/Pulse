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
