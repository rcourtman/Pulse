package monitoring

import (
	"context"
	"errors"
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

// linkedFSInfoClient also answers metadata, the command that keeps guest-agent
// evidence current once a linked guest sends no filesystem read.
type linkedFSInfoClient struct {
	fsInfoPollClient
	metadataCalls int
	metadataFails bool
}

func (c *linkedFSInfoClient) GetVMNetworkInterfaces(context.Context, string, int) ([]proxmox.VMNetworkInterface, error) {
	c.metadataCalls++
	if c.metadataFails {
		return nil, errors.New("metadata unavailable")
	}
	return []proxmox.VMNetworkInterface{{Name: "eth0", HardwareAddr: "02:00:00:00:00:01", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.10", Prefix: 24}}}}, nil
}

func pollLinkedFSInfoGuest(t *testing.T, m *Monitor, client PVEClientInterface, agents map[string]models.Host, prev *models.VM) models.VM {
	t.Helper()
	vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, fsInfoFixtureResource, client, makeGuestID(fsInfoFixtureInstance, "node", 105), agents, prev)
	if !ok {
		t.Fatal("guest disappeared")
	}
	return vm
}

func linkedFSInfoAgent(used int64) map[string]models.Host {
	return map[string]models.Host{makeGuestID(fsInfoFixtureInstance, "node", 105): {
		Status:   "online",
		LastSeen: time.Now().Add(-5 * time.Second),
		Disks:    []models.Disk{{Total: 5000, Used: used, Free: 5000 - used, Usage: float64(used) / 50, Mountpoint: "/", Type: "ext4"}},
	}}
}

func TestLinkedGuestSendsNoFSInfoWhileAgentSuppliesDisks(t *testing.T) {
	t.Run("guest agent to linked agent and back", func(t *testing.T) {
		client := &linkedFSInfoClient{}
		client.used = 400
		m := newFSInfoPollMonitor()

		read := pollLinkedFSInfoGuest(t, m, client, nil, nil)
		if client.fsCalls != 1 || read.DiskObservation.Source != "guest-agent" || read.Disk.Used != 400 {
			t.Fatalf("first poll: calls=%d %+v", client.fsCalls, read)
		}

		agents := linkedFSInfoAgent(1000)
		seen := agents[makeGuestID(fsInfoFixtureInstance, "node", 105)].LastSeen
		linked := pollLinkedFSInfoGuest(t, m, client, agents, &read)
		if client.fsCalls != 1 {
			t.Fatalf("linked poll sent get-fsinfo: %d", client.fsCalls)
		}
		if linked.Disk.Used != 1000 || linked.DiskStatusReason != "" || len(linked.Disks) != 1 || linked.DiskObservation.Source != "agent" || !linked.DiskObservation.ObservedAt.Equal(seen) {
			t.Fatalf("linked poll lost the agent's disks: %+v", linked)
		}
		// A Pulse agent's report says nothing about the QEMU guest agent.
		if linked.GuestAgentEvidence.ObservedAt.Equal(seen) || linked.GuestAgentStatus != "available" {
			t.Fatalf("guest-agent state took the Pulse agent's word: %+v", linked)
		}

		again := pollLinkedFSInfoGuest(t, m, client, agents, &linked)
		if client.fsCalls != 1 || again.Disk.Used != 1000 {
			t.Fatalf("second linked poll: calls=%d %+v", client.fsCalls, again)
		}

		// Still inside the first read's 55 seconds, so only a dropped reply makes
		// this a new read.
		client.used = 700
		before := time.Now()
		back := pollLinkedFSInfoGuest(t, m, client, nil, &again)
		if client.fsCalls != 2 {
			t.Fatalf("losing the agent must read at once: %d", client.fsCalls)
		}
		if back.Disk.Used != 700 || back.DiskStatusReason != "" || back.DiskObservation.Source != "guest-agent" || back.DiskObservation.ObservedAt.Before(before) {
			t.Fatalf("first reading after the agent: %+v", back)
		}
	})

	t.Run("a failed read after the agent keeps no agent reading", func(t *testing.T) {
		client := &linkedFSInfoClient{}
		client.fsErr = errors.New("agent error")
		m := newFSInfoPollMonitor()
		linked := pollLinkedFSInfoGuest(t, m, client, linkedFSInfoAgent(1000), nil)
		if client.fsCalls != 0 || linked.Disk.Used != 1000 {
			t.Fatalf("calls=%d %+v", client.fsCalls, linked)
		}
		lost := pollLinkedFSInfoGuest(t, m, client, nil, &linked)
		if client.fsCalls != 1 {
			t.Fatalf("losing the agent must read at once: %d", client.fsCalls)
		}
		if lost.DiskStatusReason != "agent-error" || lost.Disk.Usage != -1 || len(lost.Disks) != 0 || lost.DiskObservation.Source != "guest-agent" {
			t.Fatalf("an old agent reading stood in for a failed read: %+v", lost)
		}
	})

	t.Run("a poll that reaches no read still drops the earlier reply", func(t *testing.T) {
		client := &linkedFSInfoClient{}
		client.used = 400
		m := newFSInfoPollMonitor()
		read := pollLinkedFSInfoGuest(t, m, client, nil, nil)

		// No status reply and a listed lock: this poll never reaches the read path.
		client.statusErr = errors.New("status unavailable")
		locked := fsInfoFixtureResource
		locked.Lock = "backup"
		id := makeGuestID(fsInfoFixtureInstance, "node", 105)
		linked, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, locked, client, id, linkedFSInfoAgent(1000), &read)
		if !ok || client.fsCalls != 1 || linked.Disk.Used != 1000 {
			t.Fatalf("locked linked poll: calls=%d %+v", client.fsCalls, linked)
		}

		client.statusErr = nil
		client.used = 700
		back := pollLinkedFSInfoGuest(t, m, client, nil, &linked)
		if client.fsCalls != 2 || back.Disk.Used != 700 {
			t.Fatalf("an old reply survived the linked poll: calls=%d %+v", client.fsCalls, back)
		}
	})

	t.Run("memory retention still sees a live guest", func(t *testing.T) {
		client := &linkedFSInfoClient{metadataFails: true}
		client.statusErr = errors.New("status unavailable")
		m := newFSInfoPollMonitor()
		// Past the two-minute carry-forward, so only the healthy-guest path applies.
		m.guestSnapshots = map[string]GuestMemorySnapshot{
			makeGuestSnapshotKey(fsInfoFixtureInstance, "qemu", "node", 105): {
				GuestType:    "qemu",
				Status:       "running",
				MemorySource: "guest-agent-meminfo",
				Memory: models.Memory{Total: 8000, Used: 2000, Free: 6000, Usage: 25, Observation: models.MemoryObservation{
					State: "current", Source: "guest-agent-meminfo", ObservedAt: time.Now().Add(-3 * time.Minute),
				}},
			},
		}
		full := fsInfoFixtureResource
		full.Mem = full.MaxMem
		id := makeGuestID(fsInfoFixtureInstance, "node", 105)
		vm, _, source, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, full, client, id, linkedFSInfoAgent(1000), nil)
		if !ok || client.fsCalls != 0 || vm.Memory.Used != 2000 || source != "previous-snapshot" {
			t.Fatalf("a listing's full memory replaced the retained reading: calls=%d source=%s %+v", client.fsCalls, source, vm.Memory)
		}
	})

	t.Run("an agent reporting no usable disks does not stop the read", func(t *testing.T) {
		id := makeGuestID(fsInfoFixtureInstance, "node", 105)
		for name, host := range map[string]models.Host{
			"no disks":   {Status: "online", LastSeen: time.Now(), Memory: models.Memory{Total: 8000, Used: 2000}},
			"zero total": {Status: "online", LastSeen: time.Now(), Disks: []models.Disk{{Mountpoint: "/"}}},
		} {
			client := &linkedFSInfoClient{}
			client.used = 400
			vm := pollLinkedFSInfoGuest(t, newFSInfoPollMonitor(), client, map[string]models.Host{id: host}, nil)
			if client.fsCalls != 1 || vm.Disk.Used != 400 || vm.DiskObservation.Source != "guest-agent" {
				t.Fatalf("%s: calls=%d %+v", name, client.fsCalls, vm)
			}
		}
	})

	t.Run("no status reply", func(t *testing.T) {
		client := &linkedFSInfoClient{}
		client.used = 400
		client.statusErr = errors.New("status unavailable")
		m := newFSInfoPollMonitor()
		prev := models.VM{Type: "qemu", GuestAgentEvidence: models.GuestAgentEvidence{Explicit: true, ObservedAt: time.Now().Add(-time.Minute)}}

		linked := pollLinkedFSInfoGuest(t, m, client, linkedFSInfoAgent(1000), &prev)
		if client.fsCalls != 0 || linked.Disk.Used != 1000 || linked.DiskStatusReason != "" {
			t.Fatalf("linked fallback: calls=%d %+v", client.fsCalls, linked)
		}
		if client.metadataCalls == 0 || !linked.GuestAgentExpected {
			t.Fatalf("the fallback stopped reading metadata: calls=%d %+v", client.metadataCalls, linked)
		}

		lost := pollLinkedFSInfoGuest(t, m, client, nil, &linked)
		if client.fsCalls != 1 || lost.Disk.Used != 400 || lost.DiskStatusReason != "" {
			t.Fatalf("recent evidence must allow the read: calls=%d %+v", client.fsCalls, lost)
		}

		// With no evidence left nothing justifies a command, and the row says so
		// rather than keeping the agent's reading.
		fresh := &linkedFSInfoClient{metadataFails: true}
		fresh.statusErr = client.statusErr
		expired := linked
		expired.GuestAgentEvidence.ObservedAt = time.Now().Add(-time.Hour)
		noEvidence := pollLinkedFSInfoGuest(t, newFSInfoPollMonitor(), fresh, nil, &expired)
		if fresh.fsCalls != 0 || noEvidence.DiskStatusReason != "no-status" || noEvidence.Disk.Usage != -1 || len(noEvidence.Disks) != 0 {
			t.Fatalf("calls=%d %+v", fresh.fsCalls, noEvidence)
		}
	})

	t.Run("metadata keeps guest-agent evidence current", func(t *testing.T) {
		client := &linkedFSInfoClient{}
		m := newFSInfoPollMonitor()
		agents := linkedFSInfoAgent(1000)
		linked := pollLinkedFSInfoGuest(t, m, client, agents, nil)
		if client.fsCalls != 0 || client.metadataCalls == 0 || !hasRecentGuestAgentEvidence(&linked, time.Now()) {
			t.Fatalf("fsinfo=%d metadata=%d evidence=%+v", client.fsCalls, client.metadataCalls, linked.GuestAgentEvidence)
		}
		// The incident for a guest agent that stops answering still fires.
		client.agentOff = true
		down := pollLinkedFSInfoGuest(t, m, client, agents, &linked)
		if down.GuestAgentStatus != "expected-unreachable" || !down.GuestAgentExpected || down.Disk.Used != 1000 {
			t.Fatalf("a linked guest lost its unreachable state: %+v", down)
		}
	})
}
