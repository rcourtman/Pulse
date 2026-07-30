package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestRegistryProjectsAgentLibvirtDomainsAsVMResources(t *testing.T) {
	now := time.Now().UTC()
	host := models.Host{
		ID:       "host-1",
		Hostname: "kvm-host",
		Status:   "online",
		LastSeen: now,
		Libvirt: &models.HostLibvirtInventory{Domains: []models.HostLibvirtDomain{{
			ID:                 "domain-a",
			Name:               "app",
			State:              "running",
			VCPUs:              4,
			CPUUsagePercent:    12.5,
			CPUUsageValid:      true,
			MemoryCurrentBytes: 2 << 30,
			MemoryMaximumBytes: 4 << 30,
			NetworkInRate:      100,
			NetworkOutRate:     200,
			DiskReadRate:       300,
			DiskWriteRate:      400,
			IORatesValid:       true,
		}}},
	}

	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{host}})
	vms := registry.ListByType(ResourceTypeVM)
	if len(vms) != 1 {
		t.Fatalf("VM resources = %d, want 1: %+v", len(vms), vms)
	}
	vm := vms[0]
	if vm.Name != "app" ||
		vm.Technology != "libvirt" ||
		vm.Status != StatusOnline ||
		vm.VirtualMachine == nil ||
		vm.VirtualMachine.VCPUs != 4 ||
		vm.VirtualMachine.RuntimeState != "running" {
		t.Fatalf("VM resource = %+v", vm)
	}
	if vm.ParentID == nil || *vm.ParentID == "" {
		t.Fatalf("VM parent = %+v, want host resource", vm.ParentID)
	}
	if vm.Metrics == nil ||
		vm.Metrics.CPU == nil ||
		vm.Metrics.CPU.Percent != 12.5 ||
		vm.Metrics.Memory == nil ||
		vm.Metrics.Memory.Percent != 50 ||
		vm.Metrics.NetIn == nil ||
		vm.Metrics.NetIn.Value != 100 {
		t.Fatalf("VM metrics = %+v", vm.Metrics)
	}
	wantMetricID := "host-1:libvirt:domain-a"
	metricsTarget := BuildMetricsTargetForRegistry(registry, vm.ID)
	if metricsTarget == nil ||
		metricsTarget.ResourceType != "vm" ||
		metricsTarget.ResourceID != wantMetricID {
		t.Fatalf("metrics target = %+v, want vm/%s", metricsTarget, wantMetricID)
	}
}

func TestLibvirtResourceStatusMapping(t *testing.T) {
	tests := map[string]ResourceStatus{
		"running":       StatusOnline,
		"blocked":       StatusOnline,
		"shutoff":       StatusOffline,
		"paused":        StatusWarning,
		"shutting-down": StatusWarning,
		"crashed":       StatusWarning,
		"suspended":     StatusWarning,
		"unknown":       StatusUnknown,
	}
	for state, want := range tests {
		if got := libvirtResourceStatus(state); got != want {
			t.Errorf("libvirtResourceStatus(%q) = %q, want %q", state, got, want)
		}
	}
}
