package alerts

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

type guestKind uint8

const (
	guestKindUnknown guestKind = iota
	guestKindVM
	guestKindContainer
)

type guestSnapshot struct {
	Kind guestKind

	ID       string
	VMID     int
	Name     string
	Node     string
	Instance string
	Status   string

	CPUPercent float64
	MemUsage   float64
	DiskUsage  float64
	DiskRead   int64
	DiskWrite  int64
	NetworkIn  int64
	NetworkOut int64

	Disks []models.Disk
	Tags  []string
}

func (g guestSnapshot) displayType() string {
	switch g.Kind {
	case guestKindVM:
		return "VM"
	case guestKindContainer:
		return "Container"
	default:
		return "Guest"
	}
}

func (g guestSnapshot) resourceType() string {
	switch g.Kind {
	case guestKindVM:
		return "vm"
	case guestKindContainer:
		return "container"
	default:
		return "guest"
	}
}

func (g guestSnapshot) metrics() guestMetrics {
	return guestMetrics{
		CPU:        g.CPUPercent,
		MemUsage:   g.MemUsage,
		DiskUsage:  g.DiskUsage,
		DiskRead:   g.DiskRead,
		DiskWrite:  g.DiskWrite,
		NetworkIn:  g.NetworkIn,
		NetworkOut: g.NetworkOut,
		Name:       g.Name,
		Node:       g.Node,
		ID:         g.ID,
		Status:     g.Status,
	}
}

func guestSnapshotFromVM(vm models.VM) guestSnapshot {
	return guestSnapshot{
		Kind:       guestKindVM,
		ID:         vm.ID,
		VMID:       vm.VMID,
		Name:       vm.Name,
		Node:       vm.Node,
		Instance:   vm.Instance,
		Status:     vm.Status,
		CPUPercent: vm.CPU * 100,
		MemUsage:   vm.Memory.Usage,
		DiskUsage:  vm.Disk.Usage,
		DiskRead:   vm.DiskRead,
		DiskWrite:  vm.DiskWrite,
		NetworkIn:  vm.NetworkIn,
		NetworkOut: vm.NetworkOut,
		Disks:      append([]models.Disk(nil), vm.Disks...),
		Tags:       append([]string(nil), vm.Tags...),
	}
}

func guestSnapshotFromContainer(container models.Container) guestSnapshot {
	return guestSnapshot{
		Kind:       guestKindContainer,
		ID:         container.ID,
		VMID:       container.VMID,
		Name:       container.Name,
		Node:       container.Node,
		Instance:   container.Instance,
		Status:     container.Status,
		CPUPercent: container.CPU * 100,
		MemUsage:   container.Memory.Usage,
		DiskUsage:  container.Disk.Usage,
		DiskRead:   container.DiskRead,
		DiskWrite:  container.DiskWrite,
		NetworkIn:  container.NetworkIn,
		NetworkOut: container.NetworkOut,
		Disks:      append([]models.Disk(nil), container.Disks...),
		Tags:       append([]string(nil), container.Tags...),
	}
}

func extractGuestSnapshot(guest any) (guestSnapshot, bool) {
	switch g := guest.(type) {
	case models.VM:
		return guestSnapshotFromVM(g), true
	case models.Container:
		return guestSnapshotFromContainer(g), true
	default:
		return guestSnapshot{}, false
	}
}
