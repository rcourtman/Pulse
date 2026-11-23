package monitoring

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestClusterGuestCollectionPreservesInventoryOrder(t *testing.T) {
	mon := newTestPVEMonitor("site")
	t.Cleanup(mon.alertManager.Stop)
	t.Cleanup(mon.notificationMgr.Stop)
	mon.guestAgentWorkSlots = make(chan struct{}, 3)
	client := &stubPVEClient{}

	resource := func(kind, node string, id, template int) proxmox.ClusterResource {
		return proxmox.ClusterResource{Type: kind, Node: node, VMID: id, Name: "guest-" + strconv.Itoa(id), Template: template, Status: "stopped"}
	}
	for _, tc := range []struct {
		name      string
		resources []proxmox.ClusterResource
		wantVMs   []string
		wantLXCs  []string
	}{
		{
			name: "mixed with templates and an unrelated resource",
			resources: []proxmox.ClusterResource{
				resource("qemu", "b", 200, 1),
				resource("lxc", "a", 300, 1),
				resource("qemu", "b", 205, 0),
				resource("lxc", "a", 305, 0),
				resource("qemu", "a", 201, 0),
				resource("storage", "b", 500, 0),
				resource("lxc", "b", 301, 0),
				resource("qemu", "a", 202, 1),
				resource("lxc", "b", 302, 1),
				resource("qemu", "b", 203, 0),
			},
			wantVMs:  []string{"site:b:205", "site:a:201", "site:b:203"},
			wantLXCs: []string{"site:a:305", "site:b:301"},
		},
		{
			name:      "only containers",
			resources: []proxmox.ClusterResource{resource("lxc", "a", 304, 0)},
			wantLXCs:  []string{"site:a:304"},
		},
		{
			name:      "only templates",
			resources: []proxmox.ClusterResource{resource("qemu", "b", 200, 1), resource("lxc", "a", 300, 1)},
		},
		{name: "authoritative empty inventory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.resources)
			vms, lxcs := mon.collectGuestsFromClusterResources(context.Background(), "site", tc.resources, client, nil, nil, nil, nil)
			if !slices.Equal(tc.resources, original) {
				t.Fatal("collection changed the source inventory")
			}
			if vms == nil || lxcs == nil {
				t.Fatal("empty inventory collections must be non-nil")
			}
			vmIDs := make([]string, 0, len(vms))
			for _, vm := range vms {
				if vm.Name != "guest-"+strconv.Itoa(vm.VMID) {
					t.Fatalf("VM %s got another source row's name: %q", vm.ID, vm.Name)
				}
				vmIDs = append(vmIDs, vm.ID)
			}
			lxcIDs := make([]string, 0, len(lxcs))
			for _, lxc := range lxcs {
				if lxc.Name != "guest-"+strconv.Itoa(lxc.VMID) {
					t.Fatalf("container %s got another source row's name: %q", lxc.ID, lxc.Name)
				}
				lxcIDs = append(lxcIDs, lxc.ID)
			}
			if !slices.Equal(vmIDs, tc.wantVMs) || !slices.Equal(lxcIDs, tc.wantLXCs) {
				t.Fatalf("ordered QEMU/LXC IDs = %v / %v; want %v / %v", vmIDs, lxcIDs, tc.wantVMs, tc.wantLXCs)
			}
		})
	}
}
