package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestObserveProxmoxGuestReadsDirectControlPlaneState(t *testing.T) {
	client := &actionObserverPVEClient{
		stubPVEClient: &stubPVEClient{},
		vmStatus:      &proxmox.VMStatus{Status: "running", Uptime: 42},
		ctStatus:      &proxmox.Container{VMID: proxmox.FlexInt(101), Status: "stopped", Uptime: 9},
	}
	monitor := &Monitor{pveClients: map[string]PVEClientInterface{"homelab": client}}

	vm, err := monitor.ObserveProxmoxGuest(context.Background(), "homelab", "node-a", 160, "vm")
	if err != nil {
		t.Fatalf("ObserveProxmoxGuest VM: %v", err)
	}
	if vm.Kind != "vm" || vm.Status != "running" || vm.Uptime != 42 || vm.Instance != "homelab" || vm.Node != "node-a" || vm.VMID != 160 || vm.ObservedAt.IsZero() {
		t.Fatalf("VM observation = %#v", vm)
	}

	ct, err := monitor.ObserveProxmoxGuest(context.Background(), "homelab", "node-a", 101, "lxc")
	if err != nil {
		t.Fatalf("ObserveProxmoxGuest CT: %v", err)
	}
	if ct.Kind != "ct" || ct.Status != "stopped" || ct.Uptime != 9 || ct.VMID != 101 {
		t.Fatalf("CT observation = %#v", ct)
	}
}

func TestObserveProxmoxGuestFailsClosedForMissingClientAndIdentityMismatch(t *testing.T) {
	monitor := &Monitor{pveClients: map[string]PVEClientInterface{}}
	if _, err := monitor.ObserveProxmoxGuest(context.Background(), "missing", "node-a", 160, "vm"); err == nil {
		t.Fatal("missing Proxmox client unexpectedly produced an observation")
	}

	monitor.pveClients["homelab"] = &actionObserverPVEClient{
		stubPVEClient: &stubPVEClient{},
		ctStatus:      &proxmox.Container{VMID: proxmox.FlexInt(999), Status: "running"},
	}
	if _, err := monitor.ObserveProxmoxGuest(context.Background(), "homelab", "node-a", 101, "ct"); err == nil {
		t.Fatal("mismatched Proxmox CT identity unexpectedly produced an observation")
	}
}

type actionObserverPVEClient struct {
	*stubPVEClient
	vmStatus *proxmox.VMStatus
	ctStatus *proxmox.Container
}

func (c *actionObserverPVEClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	return c.vmStatus, nil
}

func (c *actionObserverPVEClient) GetContainerStatus(context.Context, string, int) (*proxmox.Container, error) {
	return c.ctStatus, nil
}
