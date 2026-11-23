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
	prevVMByID map[string]*models.VM,
	vmIDToHostAgent map[string]models.Host,
) ([]models.VM, map[string]struct{}) {
	templateSubjects := make(map[string]struct{})
	resources := make([]proxmox.ClusterResource, 0, len(vms))

	for _, vm := range vms {
		if vm.Template == 1 {
			if key := pveBackupTemplateSubjectKey(instanceName, "qemu", node, vm.VMID); key != "" {
				templateSubjects[key] = struct{}{}
			}
			continue
		}

		resources = append(resources, proxmox.ClusterResource{
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
			Lock:       vm.Lock,
			IOCounters: vm.IOCounters,
			ObservedAt: vm.ObservedAt,
		})
	}

	return m.collectClusterVMResources(ctx, instanceName, clusterGuestInventory{rows: resources}, client, prevVMByID, vmIDToHostAgent), templateSubjects
}
