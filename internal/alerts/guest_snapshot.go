package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

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
	Lock     string

	CPUPercent float64
	MemUsage   float64
	// MemoryUnavailable prevents a missing cache-aware sample from clearing a
	// real alert or starting a false one.
	MemoryUnavailable bool
	DiskUsage         float64
	DiskRead          int64
	DiskWrite         int64
	NetworkIn         int64
	NetworkOut        int64

	Disks  []models.Disk
	Tags   []string
	OnBoot *bool
}

func emptyGuestSnapshot() guestSnapshot {
	return guestSnapshot{}.normalizeCollections()
}

func (g guestSnapshot) normalizeCollections() guestSnapshot {
	if g.Disks == nil {
		g.Disks = []models.Disk{}
	}
	if g.Tags == nil {
		g.Tags = []string{}
	}
	return g
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
		return "system-container"
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
		Kind:              guestKindVM,
		ID:                vm.ID,
		VMID:              vm.VMID,
		Name:              vm.Name,
		Node:              vm.Node,
		Instance:          vm.Instance,
		Status:            vm.Status,
		Lock:              vm.Lock,
		CPUPercent:        unifiedresources.ProxmoxGuestCPUPercent(vm.CPU),
		MemUsage:          vm.Memory.Usage,
		MemoryUnavailable: vm.Memory.UsageUnavailable,
		DiskUsage:         vm.Disk.Usage,
		DiskRead:          vm.DiskRead,
		DiskWrite:         vm.DiskWrite,
		NetworkIn:         vm.NetworkIn,
		NetworkOut:        vm.NetworkOut,
		Disks:             append([]models.Disk(nil), vm.Disks...),
		Tags:              append([]string(nil), vm.Tags...),
		OnBoot:            vm.OnBoot,
	}.normalizeCollections()
}

func guestSnapshotFromContainer(container models.Container) guestSnapshot {
	return guestSnapshot{
		Kind:              guestKindContainer,
		ID:                container.ID,
		VMID:              container.VMID,
		Name:              container.Name,
		Node:              container.Node,
		Instance:          container.Instance,
		Status:            container.Status,
		Lock:              container.Lock,
		CPUPercent:        unifiedresources.ProxmoxGuestCPUPercent(container.CPU),
		MemUsage:          container.Memory.Usage,
		MemoryUnavailable: container.Memory.UsageUnavailable,
		DiskUsage:         container.Disk.Usage,
		DiskRead:          container.DiskRead,
		DiskWrite:         container.DiskWrite,
		NetworkIn:         container.NetworkIn,
		NetworkOut:        container.NetworkOut,
		Disks:             append([]models.Disk(nil), container.Disks...),
		Tags:              append([]string(nil), container.Tags...),
		OnBoot:            container.OnBoot,
	}.normalizeCollections()
}

func guestKindFromType(guestType string) guestKind {
	switch strings.ToLower(strings.TrimSpace(guestType)) {
	case "qemu", "vm":
		return guestKindVM
	case "lxc", "container", "system-container":
		return guestKindContainer
	default:
		return guestKindUnknown
	}
}

func guestSnapshotFromIdentity(resourceID, name, node, instance, guestType, status string) guestSnapshot {
	snapshot := guestSnapshot{
		Kind:     guestKindFromType(guestType),
		ID:       strings.TrimSpace(resourceID),
		Name:     strings.TrimSpace(name),
		Node:     strings.TrimSpace(node),
		Instance: strings.TrimSpace(instance),
		Status:   strings.TrimSpace(status),
	}

	if ident, ok := guestOverrideIdentityFromGuestOrID(nil, resourceID); ok {
		if snapshot.VMID <= 0 {
			snapshot.VMID = ident.vmid
		}
		if snapshot.Instance == "" {
			snapshot.Instance = ident.instance
		}
		if snapshot.Node == "" {
			snapshot.Node = ident.node
		}
	}

	if snapshot.Instance == "" {
		snapshot.Instance = snapshot.Node
	}

	return snapshot.normalizeCollections()
}

func guestSnapshotFromLookup(lookup GuestLookup, fallbackName string) guestSnapshot {
	resourceID := strings.TrimSpace(lookup.ResourceID)
	if resourceID == "" && lookup.Instance != "" && lookup.Node != "" && lookup.VMID > 0 {
		resourceID = BuildGuestKey(lookup.Instance, lookup.Node, lookup.VMID)
	}

	name := strings.TrimSpace(lookup.Name)
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}

	snapshot := guestSnapshotFromIdentity(resourceID, name, lookup.Node, lookup.Instance, lookup.Type, "")
	if snapshot.VMID <= 0 && lookup.VMID > 0 {
		snapshot.VMID = lookup.VMID
	}
	snapshot.Tags = append([]string(nil), lookup.Tags...)
	return snapshot.normalizeCollections()
}

func guestSnapshotFromAlert(alert *Alert, resourceID string) guestSnapshot {
	if alert == nil {
		return guestSnapshotFromIdentity(resourceID, "", "", "", "", "")
	}

	name := metadataStringValue(alert.Metadata, "guestName")
	if name == "" {
		name = alert.ResourceName
	}

	node := metadataStringValue(alert.Metadata, "guestNode")
	if node == "" {
		node = alert.Node
	}

	instance := metadataStringValue(alert.Metadata, "guestInstance")
	if instance == "" {
		instance = alert.Instance
	}

	guestType := metadataStringValue(alert.Metadata, "guestType")
	if guestType == "" {
		guestType = metadataStringValue(alert.Metadata, "resourceType")
	}

	status := metadataStringValue(alert.Metadata, "guestStatus")
	if status == "" {
		status = metadataStringValue(alert.Metadata, "status")
	}

	snapshot := guestSnapshotFromIdentity(resourceID, name, node, instance, guestType, status)
	if snapshot.VMID <= 0 {
		snapshot.VMID = metadataIntValue(alert.Metadata["guestVmid"])
	}
	return snapshot.normalizeCollections()
}

func extractGuestSnapshot(guest any) (guestSnapshot, bool) {
	switch g := guest.(type) {
	case models.VM:
		return guestSnapshotFromVM(g), true
	case *models.VM:
		if g == nil {
			return emptyGuestSnapshot(), false
		}
		return guestSnapshotFromVM(*g), true
	case models.Container:
		return guestSnapshotFromContainer(g), true
	case *models.Container:
		if g == nil {
			return emptyGuestSnapshot(), false
		}
		return guestSnapshotFromContainer(*g), true
	case guestSnapshot:
		return g.normalizeCollections(), true
	case *guestSnapshot:
		if g == nil {
			return emptyGuestSnapshot(), false
		}
		return g.normalizeCollections(), true
	default:
		return emptyGuestSnapshot(), false
	}
}
