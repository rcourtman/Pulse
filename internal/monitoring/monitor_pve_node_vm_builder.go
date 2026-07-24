package monitoring

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func (m *Monitor) pollNodeVMsWithClusterResourceBuilder(
	ctx context.Context,
	instanceName string,
	node string,
	vms []proxmox.VM,
	client PVEClientInterface,
	prevVMByID map[string]models.VM,
	vmIDToHostAgent map[string]models.Host,
) ([]models.VM, map[string]struct{}) {
	templateSubjects := make(map[string]struct{})
	resources := make([]indexedClusterResource, 0, len(vms))

	for _, vm := range vms {
		if vm.Template == 1 {
			if key := pveBackupTemplateSubjectKey(instanceName, "qemu", node, vm.VMID); key != "" {
				templateSubjects[key] = struct{}{}
			}
			continue
		}

		guestID := makeGuestID(instanceName, node, vm.VMID)
		resources = append(resources, indexedClusterResource{
			order: len(resources),
			resource: proxmox.ClusterResource{
				ID:         guestID,
				Type:       "qemu",
				Node:       node,
				Pool:       vm.Pool,
				Status:     vm.Status,
				Name:       vm.Name,
				VMID:       vm.VMID,
				CPU:        vm.CPU,
				MaxCPU:     vm.CPUs,
				Mem:        vm.Mem,
				MaxMem:     vm.MaxMem,
				Disk:       vm.Disk,
				MaxDisk:    vm.MaxDisk,
				NetIn:      vm.NetIn,
				NetOut:     vm.NetOut,
				DiskRead:   vm.DiskRead,
				DiskWrite:  vm.DiskWrite,
				Uptime:     vm.Uptime,
				Template:   vm.Template,
				Tags:       vm.Tags,
				IOCounters: vm.IOCounters,
				ObservedAt: vm.ObservedAt,
			},
			guestID: guestID,
		})
	}

	return m.collectClusterVMResources(ctx, instanceName, resources, client, prevVMByID, vmIDToHostAgent), templateSubjects
}
