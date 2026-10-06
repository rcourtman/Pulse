package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Ordinary guest construction uses existing agent evidence, never a new QGA
// probe. A backup lock remains a deferral even when independent disks exist.
type linkedDiskPollClient struct {
	stubPVEClient
	fsCalls int
	locked  bool
}

func (c *linkedDiskPollClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	status := &proxmox.VMStatus{MaxMem: 8000, Mem: 2000}
	if c.locked {
		status.Lock = "backup"
		status.Agent = proxmox.VMAgentField{Value: 1}
	}
	return status, nil
}
func (c *linkedDiskPollClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	c.fsCalls++
	return nil, nil
}

func TestLinkedGuestDisksNormalPollLifecycle(t *testing.T) {
	for _, merged := range []bool{false, true} {
		name := "automatic-hint"
		if merged {
			name = "retained-manual-link"
		}
		t.Run(name, func(t *testing.T) {
			seen := time.Now().Add(-5 * time.Second)
			guestID := makeGuestID("pve-a", "node1", 111)
			otherID := makeGuestID("pve-b", "node1", 111)
			store := unifiedresources.NewMemoryStore()
			if merged {
				if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: "vm-test", ResourceB: "agent-test", PrimaryID: "agent-test"}); err != nil {
					t.Fatal(err)
				}
			}
			for _, phase := range []struct {
				name                     string
				status                   string
				seen                     time.Time
				diskUsed                 int64
				empty, withdrawn, unlink bool
				want                     bool
			}{
				{name: "current", status: "online", seen: seen, diskUsed: 620, want: true},
				{name: "expired-despite-live-platform", status: "online", seen: seen.Add(-time.Hour), diskUsed: 620},
				{name: "offline", status: "offline", seen: seen, diskUsed: 620},
				{name: "empty-replacement", status: "online", seen: seen, empty: true},
				{name: "withdrawal", withdrawn: true},
				{name: "missing-link", status: "online", seen: seen, diskUsed: 620, unlink: true},
				{name: "recovery-after-registry-restart", status: "online", seen: seen, diskUsed: 300, want: true},
			} {
				t.Run(phase.name, func(t *testing.T) {
					// Reconstruct from sources plus the durable link on each phase. No old
					// matcher, remembered fallback inventory or revived source is required.
					phaseStore := store
					if phase.unlink {
						phaseStore = unifiedresources.NewMemoryStore()
					}
					registry := unifiedresources.NewRegistry(phaseStore)
					resources := []unifiedresources.Resource{
						{ID: "vm-test", Type: unifiedresources.ResourceTypeVM, Name: "windows", Status: unifiedresources.StatusOnline, LastSeen: time.Now(), Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox}, Proxmox: &unifiedresources.ProxmoxData{SourceID: guestID, Instance: "pve-a", NodeName: "node1", VMID: 111, RuntimeStatus: "running"}},
						{ID: "other-vm", Type: unifiedresources.ResourceTypeVM, Name: "other-windows", Status: unifiedresources.StatusOnline, LastSeen: time.Now(), Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox}, Proxmox: &unifiedresources.ProxmoxData{SourceID: otherID, Instance: "pve-b", NodeName: "node1", VMID: 111, RuntimeStatus: "running"}},
					}
					if !phase.withdrawn {
						disks := []unifiedresources.DiskInfo{{Total: 1000, Used: phase.diskUsed, Free: 1000 - phase.diskUsed, Usage: float64(phase.diskUsed) / 10, Mountpoint: `C:\`, Device: "C:", Filesystem: "ntfs"}}
						if phase.empty {
							disks = nil
						}
						link := guestID
						if phase.unlink {
							link = ""
						}
						resources = append(resources, unifiedresources.Resource{ID: "agent-test", Type: unifiedresources.ResourceTypeAgent, Name: "windows-agent", Status: unifiedresources.ResourceStatus(phase.status), LastSeen: phase.seen, Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent}, Agent: &unifiedresources.AgentData{AgentID: "agent-test", LinkedVMID: link, Stale: phase.status == "offline", Disks: disks}})
					}
					registry.IngestResources(resources)
					mon := &Monitor{state: models.NewState(), rateTracker: NewRateTracker(), config: &config.Config{}, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
					prev := mon.previousGuestContextForInstance("pve-a")
					agent := prev.hostAgentsByVMID[guestID]
					if phase.want && (!agent.LastSeen.Equal(phase.seen) || len(agent.Disks) != 1) {
						t.Fatalf("agent evidence dropped or renewed by platform: %+v", agent)
					}
					for _, locked := range []bool{false, true} {
						client := &linkedDiskPollClient{locked: locked}
						vm, _, _, _, _, ok := mon.buildVMFromClusterResource(context.Background(), "pve-a", proxmox.ClusterResource{Type: "qemu", Status: "running", Node: "node1", VMID: 111, Name: "windows", MaxDisk: 2000, MaxMem: 8000, Mem: 2000}, client, guestID, prev.hostAgentsByVMID, nil)
						if !ok {
							t.Fatal("guest omitted")
						}
						if phase.want {
							if vm.Disk.Total != 1000 || vm.Disk.Used != phase.diskUsed || vm.Disk.Usage != float64(phase.diskUsed)/10 || len(vm.Disks) != 1 || vm.DiskStatusReason != "" {
								t.Fatalf("ordinary poll lost current agent disks: %+v", vm)
							}
						} else if vm.Disk.Usage != -1 || len(vm.Disks) != 0 || vm.DiskStatusReason == "" {
							t.Fatalf("unavailable agent invented disk reading: %+v", vm)
						}
						if client.fsCalls != 0 {
							t.Fatal("fallback issued QGA probe")
						}
						if locked && vm.GuestAgentStatus != "deferred" {
							t.Fatal("independent disks cleared guest-read safety deferral")
						}
						projection := unifiedresources.NewRegistry(nil)
						projection.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
						view := projection.VMs()[0]
						if view.DiskStatusReason() != vm.DiskStatusReason {
							t.Fatal("canonical view disagrees with poll")
						}
						resource, _ := projection.Get(view.ID())
						if phase.want {
							if view.DiskUsed() != vm.Disk.Used || view.DiskPercent() != vm.Disk.Usage {
								t.Fatal("canonical current disk disagrees with poll")
							}
						} else if resource.Metrics != nil && resource.Metrics.Disk != nil {
							t.Fatal("canonical projection invented an unavailable disk metric")
						}
					}
					other := mon.previousGuestContextForInstance("pve-b")
					if _, _, ok := resolveGuestDiskFromLinkedHostAgent(otherID, other.hostAgentsByVMID); ok {
						t.Fatal("same VMID across instances inherited another guest's disk")
					}
				})
			}
		})
	}
}

func TestLinkedGuestDiskInventoryReplacement(t *testing.T) {
	id := makeGuestID("pve-a", "node1", 111)
	registry := unifiedresources.NewRegistry(nil)
	adapter := unifiedresources.NewMonitorAdapter(registry)
	mon := &Monitor{state: models.NewState(), resourceStore: adapter}
	snapshot := models.StateSnapshot{VMs: []models.VM{{ID: id, Instance: "pve-a", Node: "node1", VMID: 111, Name: "windows", Status: "running", LastSeen: time.Now()}}}
	for _, phase := range []struct {
		name      string
		disks     []models.Disk
		withdrawn bool
		want      bool
	}{
		{name: "current", disks: []models.Disk{{Total: 1000, Used: 620, Free: 380, Usage: 62, Mountpoint: `C:\`, Type: "ntfs"}}, want: true},
		{name: "empty-replacement"},
		{name: "recovery", disks: []models.Disk{{Total: 1000, Used: 300, Free: 700, Usage: 30, Mountpoint: `C:\`, Type: "ntfs"}}, want: true},
		{name: "withdrawal", withdrawn: true},
	} {
		t.Run(phase.name, func(t *testing.T) {
			snapshot.Hosts = []models.Host{{ID: "agent", Hostname: "windows-agent", Status: "online", LastSeen: time.Now(), LinkedVMID: id, Disks: phase.disks}}
			if phase.withdrawn {
				snapshot.Hosts = nil
			}
			// The monitor's authoritative snapshot refresh rebuilds its registry;
			// raw registry ingestion is deliberately cumulative, not deletion.
			adapter.PopulateFromSnapshot(snapshot)
			prev := mon.previousGuestContextForInstance("pve-a")
			summary, _, ok := resolveGuestDiskFromLinkedHostAgent(id, prev.hostAgentsByVMID)
			if ok != phase.want {
				t.Fatalf("replacement eligibility=%v want=%v", ok, phase.want)
			}
			if ok && summary.Used != phase.disks[0].Used {
				t.Fatalf("previous inventory survived replacement: %+v", summary)
			}
		})
	}
}
