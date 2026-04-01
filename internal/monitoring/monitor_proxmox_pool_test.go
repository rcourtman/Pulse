package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestBuildVMFromClusterResource_PreservesProxmoxPool(t *testing.T) {
	monitor := &Monitor{rateTracker: NewRateTracker()}

	vm, _, _, _, _, ok := monitor.buildVMFromClusterResource(
		context.Background(),
		"cluster-a",
		proxmox.ClusterResource{
			Type:     "qemu",
			Node:     "pve-a",
			Pool:     "prod-vms",
			Name:     "app-vm",
			Status:   "stopped",
			VMID:     101,
			MaxCPU:   4,
			MaxMem:   8192,
			MaxDisk:  1024,
			Template: 0,
		},
		nil,
		"",
		nil,
		nil,
	)
	if !ok {
		t.Fatal("expected VM to be built")
	}
	if vm.Pool != "prod-vms" {
		t.Fatalf("expected VM pool %q, got %+v", "prod-vms", vm)
	}
}

func TestBuildContainerFromClusterResource_PreservesProxmoxPool(t *testing.T) {
	monitor := &Monitor{rateTracker: NewRateTracker()}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		proxmox.ClusterResource{
			Type:     "lxc",
			Node:     "pve-a",
			Pool:     "ops-lxc",
			Name:     "cache-ct",
			Status:   "stopped",
			VMID:     202,
			MaxCPU:   2,
			MaxMem:   4096,
			MaxDisk:  2048,
			Template: 0,
		},
		nil,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected container to be built")
	}
	if container.Pool != "ops-lxc" {
		t.Fatalf("expected container pool %q, got %+v", "ops-lxc", container)
	}
}
