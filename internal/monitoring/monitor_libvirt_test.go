package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestNormalizeAgentLibvirtInventoryDerivesCPUAndIORates(t *testing.T) {
	monitor := &Monitor{rateTracker: NewRateTracker()}
	t0 := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	firstReport := &agentshost.LibvirtInventory{Domains: []agentshost.LibvirtDomain{{
		Name:               "app",
		State:              "running",
		VCPUs:              2,
		CPUTimeNanoseconds: 2_000_000_000,
		MemoryCurrentBytes: 2 << 30,
		MemoryMaximumBytes: 4 << 30,
		NetworkRXBytes:     1000,
		NetworkTXBytes:     2000,
		DiskReadBytes:      3000,
		DiskWriteBytes:     4000,
	}}}
	first := monitor.normalizeAgentLibvirtInventory(firstReport, "host-1", nil, t0)
	if first == nil || len(first.Domains) != 1 {
		t.Fatalf("first inventory = %+v", first)
	}
	if first.Domains[0].CPUUsageValid || first.Domains[0].IORatesValid {
		t.Fatalf("first sample invented rates: %+v", first.Domains[0])
	}

	previous := &models.Host{
		ID:       "host-1",
		LastSeen: t0,
		Libvirt:  first,
	}
	secondReport := &agentshost.LibvirtInventory{Domains: []agentshost.LibvirtDomain{{
		Name:               "app",
		State:              "running",
		VCPUs:              2,
		CPUTimeNanoseconds: 3_000_000_000,
		MemoryCurrentBytes: 5 << 30,
		MemoryMaximumBytes: 4 << 30,
		NetworkRXBytes:     2000,
		NetworkTXBytes:     4000,
		DiskReadBytes:      6000,
		DiskWriteBytes:     8000,
	}}}
	second := monitor.normalizeAgentLibvirtInventory(secondReport, "host-1", previous, t0.Add(10*time.Second))
	if second == nil || len(second.Domains) != 1 {
		t.Fatalf("second inventory = %+v", second)
	}
	got := second.Domains[0]
	if !got.CPUUsageValid || math.Abs(got.CPUUsagePercent-5) > 0.001 {
		t.Fatalf("CPU usage = (%v, %v), want valid 5%%", got.CPUUsagePercent, got.CPUUsageValid)
	}
	if !got.IORatesValid ||
		got.NetworkInRate != 100 ||
		got.NetworkOutRate != 200 ||
		got.DiskReadRate != 300 ||
		got.DiskWriteRate != 400 {
		t.Fatalf("I/O rates = %+v", got)
	}
	if got.MemoryCurrentBytes != 4<<30 {
		t.Fatalf("memory current = %d, want clamped to maximum", got.MemoryCurrentBytes)
	}
}

func TestNormalizeAgentLibvirtInventoryBoundsAndDeduplicates(t *testing.T) {
	monitor := &Monitor{rateTracker: NewRateTracker()}
	report := &agentshost.LibvirtInventory{Domains: []agentshost.LibvirtDomain{
		{Name: " VM ", State: "CRASHED", VCPUs: 5000},
		{Name: "vm", State: "running"},
		{Name: "bad\x00name", State: "running"},
	}}
	got := monitor.normalizeAgentLibvirtInventory(report, "host-1", nil, time.Now())
	if got == nil || len(got.Domains) != 1 {
		t.Fatalf("inventory = %+v, want one domain", got)
	}
	if got.Domains[0].Name != "VM" || got.Domains[0].State != "crashed" || got.Domains[0].VCPUs != 4096 {
		t.Fatalf("normalized domain = %+v", got.Domains[0])
	}
}

func TestNormalizeAgentLibvirtInventoryPreservesLastSuccessfulSampleOnFailure(t *testing.T) {
	collectedAt := time.Now().Add(-time.Minute).UTC()
	previous := &models.Host{Libvirt: &models.HostLibvirtInventory{
		CollectedAt: collectedAt,
		Domains: []models.HostLibvirtDomain{{
			ID:   "domain-a",
			Name: "app",
		}},
	}}
	got := (&Monitor{rateTracker: NewRateTracker()}).normalizeAgentLibvirtInventory(
		nil,
		"host-1",
		previous,
		time.Now(),
	)
	if got == nil || len(got.Domains) != 1 || !got.CollectedAt.Equal(collectedAt) {
		t.Fatalf("preserved inventory = %+v", got)
	}
	got.Domains[0].Name = "mutated"
	if previous.Libvirt.Domains[0].Name != "app" {
		t.Fatal("preserved inventory aliases previous host state")
	}
}
