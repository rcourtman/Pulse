//go:build linux

package hostmetrics

import (
	"context"
	"math"
	"testing"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

// Exercise the actual dependency's Linux parsers, not the collector's mocked
// wrappers. Do not enumerate usage on mounts: remote filesystems may block.
func TestNativeSamplingLinux(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := gocpu.TimesWithContext(ctx, false)
	if err != nil || len(first) != 1 {
		t.Fatalf("CPU sample unavailable: count=%d err=%v", len(first), err)
	}
	second, err := gocpu.TimesWithContext(ctx, false)
	if err != nil || len(second) != 1 {
		t.Fatalf("second CPU sample unavailable: count=%d err=%v", len(second), err)
	}
	total, busy := cpuBusyTotal(first[0])
	later, _ := cpuBusyTotal(second[0])
	if math.IsNaN(total) || math.IsInf(total, 0) || total <= 0 || busy < 0 || busy > total || later < total {
		t.Fatal("invalid cumulative CPU accounting")
	}
	memory, err := gomem.VirtualMemoryWithContext(ctx)
	if err != nil {
		t.Fatalf("memory sample: %v", err)
	}
	if memory.Total == 0 || memory.Available > memory.Total || memory.Used > memory.Total {
		t.Fatal("invalid memory accounting")
	}
	interfaces, err := gonet.InterfacesWithContext(ctx)
	if err != nil || len(interfaces) == 0 {
		t.Fatalf("interface sample unavailable: count=%d err=%v", len(interfaces), err)
	}
	counters, err := gonet.IOCountersWithContext(ctx, true)
	if err != nil || len(counters) == 0 {
		t.Fatalf("network counters unavailable: count=%d err=%v", len(counters), err)
	}
	names := make(map[string]bool)
	for _, iface := range interfaces {
		names[iface.Name] = true
	}
	matched := false
	for _, counter := range counters {
		matched = matched || names[counter.Name]
	}
	if !matched {
		t.Fatal("network counters have no enumerated interface")
	}
	partitions, err := partitionsVisibleToAgent(ctx, true)
	if err != nil || len(partitions) == 0 {
		t.Fatalf("mount parser unavailable: count=%d err=%v", len(partitions), err)
	}
	for _, partition := range partitions {
		if partition.Mountpoint == "" || partition.Fstype == "" {
			t.Fatal("mount parser lost mountpoint or filesystem")
		}
	}
}
