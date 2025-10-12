package monitoring

import (
	"fmt"
	"sort"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// NodeMemoryRaw captures the raw memory fields returned by Proxmox for a node.
type NodeMemoryRaw struct {
	Total               uint64 `json:"total"`
	Used                uint64 `json:"used"`
	Free                uint64 `json:"free"`
	Available           uint64 `json:"available"`
	Avail               uint64 `json:"avail"`
	Buffers             uint64 `json:"buffers"`
	Cached              uint64 `json:"cached"`
	Shared              uint64 `json:"shared"`
	EffectiveAvailable  uint64 `json:"effectiveAvailable"`
	FallbackTotal       uint64 `json:"fallbackTotal,omitempty"`
	FallbackUsed        uint64 `json:"fallbackUsed,omitempty"`
	FallbackFree        uint64 `json:"fallbackFree,omitempty"`
	FallbackCalculated  bool   `json:"fallbackCalculated,omitempty"`
	ProxmoxMemorySource string `json:"proxmoxMemorySource,omitempty"`
}

// NodeMemorySnapshot records the final node memory calculation alongside the raw data.
type NodeMemorySnapshot struct {
	Instance       string        `json:"instance"`
	Node           string        `json:"node"`
	RetrievedAt    time.Time     `json:"retrievedAt"`
	MemorySource   string        `json:"memorySource"`
	FallbackReason string        `json:"fallbackReason,omitempty"`
	Memory         models.Memory `json:"memory"`
	Raw            NodeMemoryRaw `json:"raw"`
}

// VMMemoryRaw captures both the listing and detailed status memory fields for a VM/CT.
type VMMemoryRaw struct {
	ListingMem       uint64 `json:"listingMem"`
	ListingMaxMem    uint64 `json:"listingMaxmem"`
	StatusMem        uint64 `json:"statusMem,omitempty"`
	StatusFreeMem    uint64 `json:"statusFreemem,omitempty"`
	StatusMaxMem     uint64 `json:"statusMaxmem,omitempty"`
	Balloon          uint64 `json:"balloon,omitempty"`
	BalloonMin       uint64 `json:"balloonMin,omitempty"`
	MemInfoUsed      uint64 `json:"meminfoUsed,omitempty"`
	MemInfoFree      uint64 `json:"meminfoFree,omitempty"`
	MemInfoTotal     uint64 `json:"meminfoTotal,omitempty"`
	MemInfoAvailable uint64 `json:"meminfoAvailable,omitempty"`
	MemInfoBuffers   uint64 `json:"meminfoBuffers,omitempty"`
	MemInfoCached    uint64 `json:"meminfoCached,omitempty"`
	MemInfoShared    uint64 `json:"meminfoShared,omitempty"`
	Agent            int    `json:"agent,omitempty"`
	DerivedFromBall  bool   `json:"derivedFromBalloon,omitempty"`
}

// GuestMemorySnapshot records the memory calculation for a guest (VM/LXC).
type GuestMemorySnapshot struct {
	GuestType    string        `json:"guestType"`
	Instance     string        `json:"instance"`
	Node         string        `json:"node"`
	Name         string        `json:"name"`
	VMID         int           `json:"vmid"`
	Status       string        `json:"status"`
	RetrievedAt  time.Time     `json:"retrievedAt"`
	MemorySource string        `json:"memorySource"`
	Memory       models.Memory `json:"memory"`
	Raw          VMMemoryRaw   `json:"raw"`
	Notes        []string      `json:"notes,omitempty"`
}

// DiagnosticSnapshotSet aggregates the latest node and guest snapshots.
type DiagnosticSnapshotSet struct {
	Nodes  []NodeMemorySnapshot  `json:"nodes"`
	Guests []GuestMemorySnapshot `json:"guests"`
}

func makeNodeSnapshotKey(instance, node string) string {
	return fmt.Sprintf("%s|%s", instance, node)
}

func makeGuestSnapshotKey(instance, guestType, node string, vmid int) string {
	return fmt.Sprintf("%s|%s|%s|%d", instance, guestType, node, vmid)
}

func (m *Monitor) recordNodeSnapshot(instance, node string, snapshot NodeMemorySnapshot) {
	if m == nil {
		return
	}

	snapshot.Instance = instance
	snapshot.Node = node
	if snapshot.RetrievedAt.IsZero() {
		snapshot.RetrievedAt = time.Now()
	}

	m.diagMu.Lock()
	defer m.diagMu.Unlock()
	if m.nodeSnapshots == nil {
		m.nodeSnapshots = make(map[string]NodeMemorySnapshot)
	}
	m.nodeSnapshots[makeNodeSnapshotKey(instance, node)] = snapshot
}

func (m *Monitor) recordGuestSnapshot(instance, guestType, node string, vmid int, snapshot GuestMemorySnapshot) {
	if m == nil {
		return
	}

	snapshot.Instance = instance
	snapshot.GuestType = guestType
	snapshot.Node = node
	snapshot.VMID = vmid
	if snapshot.RetrievedAt.IsZero() {
		snapshot.RetrievedAt = time.Now()
	}

	m.diagMu.Lock()
	defer m.diagMu.Unlock()
	if m.guestSnapshots == nil {
		m.guestSnapshots = make(map[string]GuestMemorySnapshot)
	}
	m.guestSnapshots[makeGuestSnapshotKey(instance, guestType, node, vmid)] = snapshot
}

// GetDiagnosticSnapshots returns copies of the latest node and guest memory snapshots.
func (m *Monitor) GetDiagnosticSnapshots() DiagnosticSnapshotSet {
	result := DiagnosticSnapshotSet{
		Nodes:  []NodeMemorySnapshot{},
		Guests: []GuestMemorySnapshot{},
	}

	if m == nil {
		return result
	}

	m.diagMu.RLock()
	defer m.diagMu.RUnlock()

	if len(m.nodeSnapshots) > 0 {
		result.Nodes = make([]NodeMemorySnapshot, 0, len(m.nodeSnapshots))
		for _, snapshot := range m.nodeSnapshots {
			result.Nodes = append(result.Nodes, snapshot)
		}
		sort.Slice(result.Nodes, func(i, j int) bool {
			if result.Nodes[i].Instance == result.Nodes[j].Instance {
				return result.Nodes[i].Node < result.Nodes[j].Node
			}
			return result.Nodes[i].Instance < result.Nodes[j].Instance
		})
	}

	if len(m.guestSnapshots) > 0 {
		result.Guests = make([]GuestMemorySnapshot, 0, len(m.guestSnapshots))
		for _, snapshot := range m.guestSnapshots {
			result.Guests = append(result.Guests, snapshot)
		}
		sort.Slice(result.Guests, func(i, j int) bool {
			if result.Guests[i].Instance == result.Guests[j].Instance {
				if result.Guests[i].Node == result.Guests[j].Node {
					if result.Guests[i].GuestType == result.Guests[j].GuestType {
						return result.Guests[i].VMID < result.Guests[j].VMID
					}
					return result.Guests[i].GuestType < result.Guests[j].GuestType
				}
				return result.Guests[i].Node < result.Guests[j].Node
			}
			return result.Guests[i].Instance < result.Guests[j].Instance
		})
	}

	return result
}
