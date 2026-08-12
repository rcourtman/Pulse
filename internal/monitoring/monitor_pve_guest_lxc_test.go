package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubPVEClientLXCStatus struct {
	stubPVEClient

	containerStatus *proxmox.Container
	statusCalls     int
}

func (s *stubPVEClientLXCStatus) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	s.statusCalls++
	return s.containerStatus, nil
}

func TestMergeContainerRuntimeCounters_PrefersNewerStatusCounters(t *testing.T) {
	t.Parallel()

	current := IOMetrics{
		DiskRead:     0,
		DiskWrite:    8,
		NetworkIn:    12,
		NetworkOut:   0,
		Timestamp:    time.Unix(0, 0),
		SourceUptime: 100,
	}

	merged := mergeContainerRuntimeCounters(current, &proxmox.Container{
		DiskRead:  128,
		DiskWrite: 4,
		NetIn:     10,
		NetOut:    256,
		Uptime:    1,
	})

	if merged.DiskRead != 128 {
		t.Fatalf("expected DiskRead to upgrade from status snapshot, got %d", merged.DiskRead)
	}
	if merged.DiskWrite != 4 {
		t.Fatalf("expected DiskWrite to follow the newer reset counter, got %d", merged.DiskWrite)
	}
	if merged.NetworkIn != 10 {
		t.Fatalf("expected NetworkIn to follow the newer reset counter, got %d", merged.NetworkIn)
	}
	if merged.NetworkOut != 256 {
		t.Fatalf("expected NetworkOut to upgrade from status snapshot, got %d", merged.NetworkOut)
	}
}

func TestMergeContainerRuntimeCounters_OverridesOnlyPresentStatusFields(t *testing.T) {
	t.Parallel()

	listingObservedAt := time.Unix(10, 0)
	current := IOMetrics{
		DiskRead:   8,
		DiskWrite:  16,
		NetworkIn:  32,
		NetworkOut: 64,
		Timestamp:  listingObservedAt,
		ObservedAt: counterObservationTimes(listingObservedAt),
		Presence: models.IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			NetworkIn:  true,
			NetworkOut: true,
		},
		SourceUptime: 100,
	}
	status := &proxmox.Container{
		DiskRead:  0,
		DiskWrite: 999,
		IOCounters: proxmox.IOCounterPresence{
			Explicit: true,
			DiskRead: true,
		},
		ObservedAt: time.Unix(20, 0),
		Uptime:     1,
	}

	merged := mergeContainerRuntimeCounters(current, status)
	if merged.DiskRead != 0 {
		t.Fatalf("explicit status zero was not authoritative: %d", merged.DiskRead)
	}
	if merged.DiskWrite != 16 || merged.NetworkIn != 32 || merged.NetworkOut != 64 {
		t.Fatalf("missing status fields overwrote listing counters: %+v", merged)
	}
	if !merged.Timestamp.Equal(status.ObservedAt) {
		t.Fatalf("timestamp = %v, want status receipt time %v", merged.Timestamp, status.ObservedAt)
	}
	if !merged.ObservedAt.DiskRead.Equal(status.ObservedAt) {
		t.Fatalf("disk-read receipt = %v, want status receipt %v", merged.ObservedAt.DiskRead, status.ObservedAt)
	}
	if !merged.ObservedAt.DiskWrite.Equal(listingObservedAt) ||
		!merged.ObservedAt.NetworkIn.Equal(listingObservedAt) ||
		!merged.ObservedAt.NetworkOut.Equal(listingObservedAt) {
		t.Fatalf("missing status fields lost listing receipt times: %+v", merged.ObservedAt)
	}
}

func TestIssue1613LXCStatusLagDoesNotEraseNewerListingDiskWrite(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{rateTracker: NewRateTracker()}
	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{
			Status:    "running",
			DiskWrite: 0,
			Uptime:    600,
			IOCounters: proxmox.IOCounterPresence{
				Explicit:  true,
				DiskWrite: true,
			},
			ObservedAt: time.Unix(1_700_000_001, 0),
		},
	}
	resource := proxmox.ClusterResource{
		Type:       "lxc",
		Node:       "pve-a",
		Name:       "write-test",
		Status:     "running",
		VMID:       203,
		Uptime:     600,
		MaxMem:     4096,
		Mem:        2048,
		ObservedAt: time.Unix(1_700_000_000, 0),
		IOCounters: proxmox.IOCounterPresence{
			Explicit:  true,
			DiskWrite: true,
		},
	}

	if _, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(), "cluster-a", resource, client, map[int]bool{},
	); !ok {
		t.Fatal("expected first LXC sample")
	}

	resource.DiskWrite = 90 * 1024 * 1024
	resource.ObservedAt = resource.ObservedAt.Add(120 * time.Second)
	client.containerStatus.ObservedAt = resource.ObservedAt.Add(time.Second)

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(), "cluster-a", resource, client, map[int]bool{},
	)
	if !ok {
		t.Fatal("expected second LXC sample")
	}
	if container.DiskWrite <= 0 {
		t.Fatalf("LXC disk write rate = %d, lagging status/current erased the listing counter", container.DiskWrite)
	}
	// The first authoritative zero came from status/current one second after
	// the listing, while the changed counter came from the next listing.
	const want = 90 * 1024 * 1024 / 119
	if container.DiskWrite != want {
		t.Fatalf("LXC disk write rate = %d B/s, want %d B/s", container.DiskWrite, want)
	}
}

func TestBuildContainerFromClusterResource_UsesContainerStatusCountersForRates(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{rateTracker: NewRateTracker()}
	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{
			Status:    "running",
			DiskRead:  4096,
			DiskWrite: 2048,
			NetIn:     1024,
			NetOut:    512,
		},
	}

	resource := proxmox.ClusterResource{
		Type:    "lxc",
		Node:    "pve-a",
		Name:    "cache-ct",
		Status:  "running",
		VMID:    202,
		MaxCPU:  2,
		MaxMem:  4096,
		Mem:     2048,
		MaxDisk: 32 * 1024 * 1024 * 1024,
		Disk:    8 * 1024 * 1024 * 1024,
	}

	if _, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	); !ok {
		t.Fatal("expected first container sample to be built")
	}

	time.Sleep(20 * time.Millisecond)

	client.containerStatus = &proxmox.Container{
		Status:    "running",
		DiskRead:  8192,
		DiskWrite: 4096,
		NetIn:     2048,
		NetOut:    1024,
	}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected second container sample to be built")
	}
	if client.statusCalls < 2 {
		t.Fatalf("expected container status to be queried for running LXC samples, got %d calls", client.statusCalls)
	}
	if container.DiskRead <= 0 {
		t.Fatalf("expected DiskRead rate from container status counters, got %d", container.DiskRead)
	}
	if container.DiskWrite <= 0 {
		t.Fatalf("expected DiskWrite rate from container status counters, got %d", container.DiskWrite)
	}
	if container.NetworkIn <= 0 {
		t.Fatalf("expected NetworkIn rate from container status counters, got %d", container.NetworkIn)
	}
	if container.NetworkOut <= 0 {
		t.Fatalf("expected NetworkOut rate from container status counters, got %d", container.NetworkOut)
	}
}

type stubPVEClientLXCRRD struct {
	stubPVEClient

	lxcRRDCalls int
}

// GetLXCRRDData tracks lookups of the guest RRD endpoint. It is intentionally
// not part of PVEClientInterface anymore: guest rrddata carries only the
// cache-inclusive mem/maxmem columns (#1634), so the LXC memory path must not
// consult it.
func (s *stubPVEClientLXCRRD) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	s.lxcRRDCalls++
	return nil, nil
}

// Issue #1634: real PVE guest RRD responses carry only mem/maxmem, never the
// cache-aware memused/memavailable columns, so running containers use the
// cluster-resources listing value instead of reporting memory as unavailable
// (rendered as 0%). The guest RRD lookup is gone entirely — it could never
// produce memory evidence.
func TestIssue1634LXCMemoryFallsBackToClusterResourcesOnRealRRDShape(t *testing.T) {
	t.Parallel()

	client := &stubPVEClientLXCRRD{}

	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "issue1634-ct",
		Status: "running",
		VMID:   108,
		MaxMem: 8 * 1024 * 1024 * 1024,
		Mem:    335716352,
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if CanonicalMemorySource(memorySource) != "cluster-resources" {
		t.Fatalf("memory source = %q, want cluster-resources fallback", memorySource)
	}
	if container.Memory.UsageUnavailable {
		t.Fatal("running LXC memory marked unavailable despite listing value")
	}
	if container.Memory.Used != int64(resource.Mem) {
		t.Fatalf("memory used = %d, want listing value %d", container.Memory.Used, resource.Mem)
	}
	if container.Memory.Usage <= 0 {
		t.Fatalf("memory usage = %f, want > 0", container.Memory.Usage)
	}
	if !container.Memory.HasKnownUsage() {
		t.Fatal("expected fallback memory to be usable for projections")
	}
	if client.lxcRRDCalls != 0 {
		t.Fatalf("expected no guest RRD lookups for LXC memory, got %d", client.lxcRRDCalls)
	}
}

func TestIssue1634LXCMemoryStaysUnavailableWithoutListingValue(t *testing.T) {
	t.Parallel()

	client := &stubPVEClientLXCRRD{}

	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "issue1634-no-mem",
		Status: "running",
		VMID:   110,
		MaxMem: 512 * 1024 * 1024,
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if CanonicalMemorySource(memorySource) != "unavailable" {
		t.Fatalf("memory source = %q, want unavailable when no evidence exists", memorySource)
	}
	if !container.Memory.UsageUnavailable {
		t.Fatal("expected memory to stay marked unavailable without any usage evidence")
	}
}

func TestIssue1477ConfigOnlyMountsSurviveIntoContainerDisks(t *testing.T) {
	t.Parallel()

	// Stock PVE reports no per-mount usage through the LXC status API, so the
	// only record of an mpX mount is the container config. The API path must
	// still surface it: capacity from size=, usage unknown (-1), and the
	// aggregate-seeded rootfs row (live usage) must survive the merge.
	metadata := parseContainerMountMetadata(map[string]interface{}{
		"rootfs": "local-lvm:vm-106-disk-0,size=59G",
		"mp0":    "tank:subvol-106-disk-1,mp=/srv/archive,size=20G",
	})

	discovered := convertContainerDiskInfo(nil, metadata)
	if len(discovered) != 2 {
		t.Fatalf("discovered disks = %+v, want rootfs + mp0", discovered)
	}

	seededRootfs := models.Disk{
		Total:      63350767616,
		Used:       23530764893,
		Free:       39820002723,
		Usage:      37.14,
		Mountpoint: "/",
		Type:       "rootfs",
	}
	merged := mergeContainerDisksPreservingExisting([]models.Disk{seededRootfs}, discovered)
	if len(merged) != 2 {
		t.Fatalf("merged disks = %+v, want rootfs + mp0", merged)
	}

	var rootfs, archive *models.Disk
	for i := range merged {
		switch merged[i].Mountpoint {
		case "/":
			rootfs = &merged[i]
		case "/srv/archive":
			archive = &merged[i]
		}
	}
	if rootfs == nil || rootfs.Used != seededRootfs.Used || rootfs.Usage != seededRootfs.Usage {
		t.Fatalf("live rootfs row must win the merge, got %+v", rootfs)
	}
	if archive == nil {
		t.Fatal("config-only mount dropped by the merge")
	}
	if archive.Usage != -1 {
		t.Fatalf("config-only mount usage = %f, want the -1 unknown sentinel", archive.Usage)
	}
	if archive.Total != int64(20)*1024*1024*1024 {
		t.Fatalf("config-only mount total = %d, want 20 GiB from size=", archive.Total)
	}
	if archive.Device != "tank:subvol-106-disk-1" || archive.Type != "mp0" {
		t.Fatalf("config-only mount must keep device and mp key, got %+v", archive)
	}
}

// The efficient cluster/resources path must apply the host agent's node-local
// pct df data the same way the per-node fallback path does. Before the #1477
// fix it skipped that enrichment entirely, so cluster-served installs never
// surfaced per-mount usage regardless of a healthy linked agent.
func TestBuildContainerFromClusterResource_AppliesAgentLXCFilesystems(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.state.UpdateNodesForInstance("cluster-a", []models.Node{{
		ID:       "cluster-a-pve-a",
		Name:     "pve-a",
		Instance: "cluster-a",
		Status:   "online",
	}})

	now := time.Now()
	monitor.applyAgentLXCFilesystems("cluster-a-pve-a", "agent-1", &agentshost.ProxmoxLXCInventory{
		Containers: []agentshost.ProxmoxLXCContainer{{
			VMID: 202,
			Name: "cache-ct",
			Disks: []agentshost.Disk{
				{Device: "local:202/vm-202-disk-0.raw", Mountpoint: "/", Type: "rootfs", TotalBytes: 32 << 30, UsedBytes: 8 << 30, FreeBytes: 24 << 30, Usage: 25},
				{Device: "/mnt/tank/cache", Mountpoint: "/data", Type: "mp0", TotalBytes: 100 << 30, UsedBytes: 50 << 30, FreeBytes: 50 << 30, Usage: 50},
			},
		}},
		CollectedAt: now.UTC(),
	}, now, 30)

	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{Status: "running"},
	}
	resource := proxmox.ClusterResource{
		Type:    "lxc",
		Node:    "pve-a",
		Name:    "cache-ct",
		Status:  "running",
		VMID:    202,
		MaxCPU:  2,
		MaxMem:  4096,
		Mem:     2048,
		MaxDisk: 32 << 30,
		Disk:    8 << 30,
	}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected container to be built")
	}
	if len(container.Disks) != 2 {
		t.Fatalf("expected 2 disks from agent pct df enrichment, got %d: %+v", len(container.Disks), container.Disks)
	}
	var dataMount *models.Disk
	for i := range container.Disks {
		if container.Disks[i].Mountpoint == "/data" {
			dataMount = &container.Disks[i]
		}
	}
	if dataMount == nil || dataMount.Total != 100<<30 || dataMount.Used != 50<<30 {
		t.Fatalf("expected /data mount with real usage from agent data, got %+v", container.Disks)
	}
}
