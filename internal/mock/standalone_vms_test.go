package mock

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildFixtureStateIncludesStandaloneVMInventories(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 0
	cfg.DockerHostCount = 0
	cfg.K8sClusterCount = 0
	cfg.GenericHostCount = 4
	cfg.RandomMetrics = false

	state := buildFixtureState(cfg)

	libvirtHosts := 0
	xcpngHosts := 0
	statesSeen := make(map[string]int)

	for _, host := range state.Hosts {
		if host.Libvirt != nil && len(host.Libvirt.Domains) > 0 {
			libvirtHosts++
			if host.Libvirt.CollectedAt.IsZero() {
				t.Fatalf("host %s libvirt inventory missing CollectedAt", host.ID)
			}
			for _, domain := range host.Libvirt.Domains {
				stateKey := strings.ToLower(strings.TrimSpace(domain.State))
				statesSeen[stateKey]++
				if stateKey == "running" {
					if !domain.CPUUsageValid || domain.CPUUsagePercent <= 0 {
						t.Fatalf("running libvirt domain %s missing valid CPU metrics: %+v", domain.Name, domain)
					}
					if !domain.IORatesValid {
						t.Fatalf("running libvirt domain %s missing valid IO rates: %+v", domain.Name, domain)
					}
					used, total, ok := domain.MemoryUsage()
					if !ok || used <= 0 || total <= 0 || used >= total {
						t.Fatalf("running libvirt domain %s expected realistic guest memory usage, got used=%d total=%d ok=%v", domain.Name, used, total, ok)
					}
				}
				if stateKey == "shutoff" {
					used, total, ok := domain.MemoryUsage()
					if !ok || used != 0 || total <= 0 {
						t.Fatalf("shutoff libvirt domain %s expected 0 used against configured total, got used=%d total=%d ok=%v", domain.Name, used, total, ok)
					}
				}
			}
		}
		if host.XCPNG != nil && len(host.XCPNG.VMs) > 0 {
			xcpngHosts++
			if host.XCPNG.CollectedAt.IsZero() {
				t.Fatalf("host %s xcpng inventory missing CollectedAt", host.ID)
			}
			for _, vm := range host.XCPNG.VMs {
				statesSeen[strings.ToLower(strings.TrimSpace(vm.PowerState))]++
			}
		}
	}

	if libvirtHosts != 2 {
		t.Fatalf("expected 2 standalone libvirt hosts, got %d", libvirtHosts)
	}
	if xcpngHosts != 1 {
		t.Fatalf("expected 1 standalone XCP-ng host, got %d", xcpngHosts)
	}
	for _, wantState := range []string{"running", "paused", "shutoff", "crashed", "halted"} {
		if statesSeen[wantState] == 0 {
			t.Fatalf("expected standalone VM fixtures to cover state %q, got states=%v", wantState, statesSeen)
		}
	}
}

func TestUnifiedResourceSnapshotProjectsStandaloneVMsAndHostParenting(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 2
	cfg.DockerHostCount = 1
	cfg.K8sClusterCount = 0
	cfg.GenericHostCount = 4
	cfg.RandomMetrics = false

	now := time.Date(2026, time.November, 23, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, now)
	resources, _ := graph.UnifiedResourceSnapshot()

	agentsByID := make(map[string]unifiedresources.Resource)
	for _, res := range resources {
		if res.Type == unifiedresources.ResourceTypeAgent {
			agentsByID[res.ID] = res
		}
	}

	libvirtCount := 0
	xcpngCount := 0
	runtimeStates := make(map[string]int)
	parentHosts := make(map[string]int)

	for _, res := range resources {
		if res.Type != unifiedresources.ResourceTypeVM || res.VirtualMachine == nil {
			continue
		}
		switch strings.ToLower(res.VirtualMachine.Hypervisor) {
		case "libvirt":
			libvirtCount++
			if res.MetricsTarget == nil || res.MetricsTarget.ResourceType != "vm" || res.MetricsTarget.ResourceID == "" {
				t.Fatalf("libvirt VM %q missing vm metrics target: %+v", res.Name, res.MetricsTarget)
			}
		case "xcp-ng":
			xcpngCount++
		default:
			continue
		}

		if res.ParentID == nil || strings.TrimSpace(*res.ParentID) == "" {
			t.Fatalf("standalone VM %q (%s) missing ParentID", res.Name, res.ID)
		}
		if _, ok := agentsByID[*res.ParentID]; !ok {
			t.Fatalf("standalone VM %q parent %q did not resolve to an agent resource", res.Name, *res.ParentID)
		}
		parentHosts[*res.ParentID]++
		runtimeStates[strings.ToLower(res.VirtualMachine.RuntimeState)]++
	}

	if libvirtCount != 7 {
		t.Fatalf("expected 7 projected libvirt VMs, got %d", libvirtCount)
	}
	if xcpngCount != 2 {
		t.Fatalf("expected 2 projected XCP-ng VMs, got %d", xcpngCount)
	}
	if len(parentHosts) != 3 {
		t.Fatalf("expected standalone VMs parented across 3 hosts, got %d (%v)", len(parentHosts), parentHosts)
	}
	for _, wantState := range []string{"running", "paused", "shutoff", "crashed", "halted"} {
		if runtimeStates[wantState] == 0 {
			t.Fatalf("expected projected standalone VMs to cover runtime state %q, got %v", wantState, runtimeStates)
		}
	}

	// Verify metric personas & memory totals were registered for libvirt VMs.
	for _, host := range graph.State.Hosts {
		if host.Libvirt == nil {
			continue
		}
		for _, domain := range host.Libvirt.Domains {
			metricID := mockLibvirtDomainMetricID(host.ID, domain.ID)
			if got := MemoryTotalForResource("vm", metricID); got <= 0 {
				t.Fatalf("expected MemoryTotalForResource for libvirt VM %q (%s) > 0, got %v", domain.Name, metricID, got)
			}
		}
	}
}

func TestUpdateHostsRefreshesStandaloneVMInventories(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 0
	cfg.DockerHostCount = 0
	cfg.K8sClusterCount = 0
	cfg.GenericHostCount = 4
	cfg.RandomMetrics = true

	state := buildFixtureState(cfg)
	var beforeCPUTime uint64
	for _, host := range state.Hosts {
		if host.Libvirt == nil {
			continue
		}
		for _, domain := range host.Libvirt.Domains {
			if domain.State == "running" {
				beforeCPUTime = domain.CPUTimeNanoseconds
				break
			}
		}
		if beforeCPUTime > 0 {
			break
		}
	}
	if beforeCPUTime == 0 {
		t.Fatal("expected running libvirt domain with initial CPUTimeNanoseconds")
	}

	later := time.Now().Add(5 * time.Minute)
	updateHosts(&state, cfg, later)

	var afterCPUTime uint64
	var collectedAt time.Time
	for _, host := range state.Hosts {
		if host.Libvirt == nil {
			continue
		}
		collectedAt = host.Libvirt.CollectedAt
		for _, domain := range host.Libvirt.Domains {
			if domain.State == "running" {
				afterCPUTime = domain.CPUTimeNanoseconds
				break
			}
		}
		if afterCPUTime > 0 {
			break
		}
	}
	if afterCPUTime <= beforeCPUTime {
		t.Fatalf("expected updateHosts to advance running libvirt CPUTimeNanoseconds (%d -> %d)", beforeCPUTime, afterCPUTime)
	}
	if collectedAt.Before(later.Add(-1 * time.Minute)) {
		t.Fatalf("expected updateHosts to refresh Libvirt.CollectedAt near %v, got %v", later, collectedAt)
	}
}
