package monitoring

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type guestPublicationMetadataFS struct {
	blocked atomic.Bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (fs *guestPublicationMetadataFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
func (fs *guestPublicationMetadataFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	if fs.blocked.Load() {
		fs.once.Do(func() { close(fs.entered) })
		<-fs.release
	}
	return os.WriteFile(path, data, mode)
}
func (fs *guestPublicationMetadataFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
func (fs *guestPublicationMetadataFS) Remove(path string) error { return os.Remove(path) }
func (fs *guestPublicationMetadataFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
func (fs *guestPublicationMetadataFS) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func TestGuestPollPublicationMigratesMetadataBeforeNewCoordinates(t *testing.T) {
	client := &guestPollOwnershipClient{}
	m := guestPollOwnershipMonitor(t, client)
	fs := &guestPublicationMetadataFS{entered: make(chan struct{}), release: make(chan struct{})}
	store := config.NewGuestMetadataStore(t.TempDir(), fs)
	m.guestMetadataStore = store
	for _, id := range []string{"site-a:old-node:105", "site-a:old-node:106"} {
		if err := store.Set(id, &config.GuestMetadata{
			CustomURL: "https://guest.example", Tags: []string{"production"}, Notes: []string{"operator note"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	m.state.UpdateGuestsForInstance("site-a",
		[]models.VM{{ID: "site-a:old-node:105", Instance: "site-a", Node: "old-node", VMID: 105}},
		[]models.Container{{ID: "site-a:old-node:106", Instance: "site-a", Node: "old-node", VMID: 106}})
	ctx, owner := m.beginPVEGuestPoll(context.Background(), "site-a", client)
	if owner == nil {
		t.Fatal("registered cycle not admitted")
	}
	defer m.finishPVEGuestPoll("site-a", owner)
	// Exercise the former collector effect as well as the publication owner.
	// Both versions reach the same persistence barrier, so the observation
	// proves ordering rather than merely requiring a new helper to be called.
	stagePVEGuestEffect(ctx, func() {
		store.GetWithLegacyMigration("site-a:new-node:105", "site-a", "new-node", 105)
	})
	fs.blocked.Store(true)
	done := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(fs.release) }) }
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("publication did not exit")
		}
		if err := store.Close(time.Second); err != nil {
			t.Error(err)
		}
	})
	go func() {
		m.publishPVEGuests(ctx, "site-a",
			[]models.VM{{ID: "site-a:new-node:105", Instance: "site-a", Node: "new-node", VMID: 105}},
			[]models.Container{{ID: "site-a:new-node:106", Instance: "site-a", Node: "new-node", VMID: 106}})
		close(done)
	}()
	select {
	case <-fs.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("metadata persistence did not start")
	}
	snapshot := m.state.GetSnapshot()
	if len(snapshot.VMs) != 1 || snapshot.VMs[0].Node != "old-node" ||
		len(snapshot.Containers) != 1 || snapshot.Containers[0].Node != "old-node" {
		t.Error("new guest coordinates became visible before metadata migration completed")
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not complete")
	}
	for _, id := range []string{"site-a:new-node:105", "site-a:new-node:106"} {
		meta := store.Get(id)
		if meta == nil || meta.CustomURL != "https://guest.example" ||
			!reflect.DeepEqual(meta.Tags, []string{"production"}) || !reflect.DeepEqual(meta.Notes, []string{"operator note"}) {
			t.Errorf("published guest lost operator metadata: %s %+v", id, meta)
		}
	}
}

func TestGuestPollRejectedPublicationDoesNotMigrateMetadata(t *testing.T) {
	client := &guestPollOwnershipClient{}
	m := guestPollOwnershipMonitor(t, client)
	store := config.NewGuestMetadataStore(t.TempDir(), nil)
	m.guestMetadataStore = store
	if err := store.Set("site-a:old-node:105", &config.GuestMetadata{CustomURL: "https://guest.example"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(time.Second) })
	ctx, owner := m.beginPVEGuestPoll(context.Background(), "site-a", client)
	if owner == nil {
		t.Fatal("registered cycle not admitted")
	}
	defer m.finishPVEGuestPoll("site-a", owner)
	m.replacePVEClient("site-a", &guestPollOwnershipClient{})
	m.publishPVEGuests(ctx, "site-a",
		[]models.VM{{ID: "site-a:new-node:105", Instance: "site-a", Node: "new-node", VMID: 105}}, nil)
	if store.Get("site-a:old-node:105") == nil || store.Get("site-a:new-node:105") != nil {
		t.Fatal("replaced poll owner migrated metadata")
	}
}

// Ignore cancellation deliberately: a late inventory reply cannot prove that
// its connection/runtime still owns publication. No native guest is involved.
type guestPollOwnershipClient struct {
	stubPVEClient
	route   string
	cpu     float64
	empty   bool
	entered chan context.Context
	release chan struct{}
	block   string
	once    sync.Once
	calls   atomic.Int32
}

func (c *guestPollOwnershipClient) wait(ctx context.Context, phase string) {
	if c.block == phase {
		c.once.Do(func() { c.entered <- ctx })
		<-c.release
	}
}

func (c *guestPollOwnershipClient) resources() []proxmox.ClusterResource {
	if c.empty {
		return nil
	}
	return []proxmox.ClusterResource{
		{Type: "qemu", Node: "node", VMID: 105, Name: "vm", Status: "running", CPU: c.cpu, MaxMem: 1000, Mem: 200, MaxDisk: 1000},
		{Type: "lxc", Node: "node", VMID: 106, Name: "ct", Status: "running", CPU: c.cpu, MaxMem: 1000, Mem: 200, MaxDisk: 1000, Disk: 200},
		{Type: "qemu", Node: "node", VMID: 107, Template: 1},
	}
}

func (c *guestPollOwnershipClient) GetClusterResources(ctx context.Context, _ string) ([]proxmox.ClusterResource, error) {
	c.calls.Add(1)
	if c.route == "node" {
		return nil, fmt.Errorf("cluster/resources unavailable")
	}
	c.wait(ctx, "inventory")
	return c.resources(), nil
}

func TestGuestPollOwnershipSingleFlightAndOrdinaryRecovery(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			c := &guestPollOwnershipClient{cpu: .1, block: "inventory", entered: make(chan context.Context, 1), release: make(chan struct{})}
			m := guestPollOwnershipMonitor(t, c)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var once sync.Once
			unblock := func() { once.Do(func() { close(c.release) }) }
			defer unblock()
			done := make(chan error, 1)
			go func() { done <- runGuestOwnershipPoll(m, ctx, c) }()
			select {
			case <-c.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("first inventory did not start")
			}
			if canceled {
				cancel()
			}
			// A repeat cannot issue even an inventory GET while this binding's
			// first cycle still owns the outstanding read.
			if err := runGuestOwnershipPoll(m, context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if c.calls.Load() != 1 {
				t.Error("overlapping/canceled cycles stacked inventory reads")
			}
			unblock()
			waitGuestOwnership(t, done)
			if err := runGuestOwnershipPoll(m, context.Background(), c); err != nil {
				t.Fatal(err)
			}
			got := m.state.GetSnapshot()
			if c.calls.Load() != 2 || len(got.VMs) != 1 || len(got.Containers) != 1 || len(m.pveGuestPolls) != 0 {
				t.Fatal("ordinary completed-cycle recovery lost inventory or leaked ownership")
			}
		})
	}
}

func TestGuestPollOwnershipRetirementWaitsForAdmittedPublication(t *testing.T) {
	c := &guestPollOwnershipClient{}
	m := guestPollOwnershipMonitor(t, c)
	ctx, owner := m.beginPVEGuestPoll(context.Background(), "site-a", c)
	if owner == nil {
		t.Fatal("registered cycle not admitted")
	}
	defer m.finishPVEGuestPoll("site-a", owner)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	stagePVEGuestEffect(ctx, func() {
		// Effects may read the monitor; publication must not hold m.mu.
		if _, ok := m.getPVEClient("site-a"); !ok {
			t.Error("publication lost its connection before its admitted effects")
		}
		close(entered)
		<-release
		m.recordGuestSnapshot("site-a", "qemu", "node", 105, GuestMemorySnapshot{Name: "accepted"})
	})
	done := make(chan error, 1)
	go func() {
		m.publishPVEGuests(ctx, "site-a", []models.VM{{ID: "site-a:node:105", Instance: "site-a", Node: "node", VMID: 105}}, nil)
		done <- nil
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("publication effect did not start or could not re-enter monitor")
	}
	retired := make(chan error, 1)
	go func() { m.retirePVEInstanceRuntime("site-a"); retired <- nil }()
	select {
	case <-retired:
		t.Error("retirement completed midway through an admitted guest batch")
	case <-time.After(30 * time.Millisecond):
	}
	unblock()
	waitGuestOwnership(t, done)
	waitGuestOwnership(t, retired)
	if len(m.state.GetSnapshot().VMs) != 0 || len(m.GetDiagnosticSnapshots().Guests) != 0 {
		t.Error("publication resurrected retired guest state/diagnostics")
	}
}

func TestGuestPollOwnershipDeadlineDoesNotExtendRuntimeAuthority(t *testing.T) {
	for _, expired := range []string{"caller", "runtime"} {
		t.Run(expired, func(t *testing.T) {
			c := &guestPollOwnershipClient{}
			m := guestPollOwnershipMonitor(t, c)
			// Install a live deadline, admit the cycle, then observe its expiry.
			// The test waits on the actual signal, not a favourable timed repeat.
			deadlineCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			parent := context.Background()
			if expired == "runtime" {
				m.setRuntimeContext(deadlineCtx, nil)
			} else {
				parent = deadlineCtx
			}
			ctx, owner := m.beginPVEGuestPoll(parent, "site-a", c)
			if owner == nil {
				t.Fatal("live registered cycle not admitted")
			}
			defer m.finishPVEGuestPoll("site-a", owner)
			stagePVEGuestEffect(ctx, func() { m.recordGuestSnapshot("site-a", "qemu", "node", 105, GuestMemorySnapshot{Name: "collected"}) })
			<-deadlineCtx.Done()
			m.publishPVEGuests(ctx, "site-a", []models.VM{{ID: "site-a:node:105", Instance: "site-a", VMID: 105, Node: "node"}}, nil)
			want := 1
			if expired == "runtime" {
				want = 0
			}
			if len(m.state.GetSnapshot().VMs) != want || len(m.GetDiagnosticSnapshots().Guests) != want {
				t.Error("caller-budget and runtime-authority deadlines were conflated")
			}
			if _, next := m.beginPVEGuestPoll(deadlineCtx, "site-a", c); next != nil {
				t.Error("expired deadline admitted a new collection")
			}
		})
	}
}

func (c *guestPollOwnershipClient) GetVMs(ctx context.Context, _ string) ([]proxmox.VM, error) {
	c.wait(ctx, "inventory")
	var rows []proxmox.VM
	for _, r := range c.resources() {
		if r.Type == "qemu" {
			rows = append(rows, proxmox.VM{VMID: r.VMID, Name: r.Name, Status: r.Status, CPU: r.CPU, MaxMem: r.MaxMem, Mem: r.Mem, MaxDisk: r.MaxDisk, Template: r.Template})
		}
	}
	return rows, nil
}

func (c *guestPollOwnershipClient) GetContainers(context.Context, string) ([]proxmox.Container, error) {
	if c.empty {
		return nil, nil
	}
	return []proxmox.Container{{VMID: 106, Name: "ct", Status: "running", CPU: c.cpu, MaxMem: 1000, Mem: 200, MaxDisk: 1000, Disk: 200}}, nil
}

func (c *guestPollOwnershipClient) GetVMStatus(ctx context.Context, _ string, _ int) (*proxmox.VMStatus, error) {
	c.wait(ctx, "detail")
	return &proxmox.VMStatus{Status: "running", MaxMem: 1000, Mem: 200}, nil
}

func guestPollOwnershipMonitor(t *testing.T, c PVEClientInterface) *Monitor {
	t.Helper()
	m := windowsBootstrapMonitor(t)
	m.pveClients = map[string]PVEClientInterface{"site-a": c}
	m.setRuntimeContext(context.Background(), nil)
	ac := m.alertManager.GetConfig()
	ac.Enabled = true
	ac.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	ac.MetricTimeThresholds = map[string]map[string]int{"guest": {"cpu": 0}}
	m.alertManager.UpdateConfig(ac)
	return m
}

func runGuestOwnershipPoll(m *Monitor, ctx context.Context, c PVEClientInterface) error {
	return m.pollGuestsWithFallback(ctx, "site-a", &config.PVEInstance{MonitorVMs: true, MonitorContainers: true}, c,
		[]proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"})
}

func waitGuestOwnership(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("controlled guest poll did not finish")
	}
}

// Use existing entry points only, so this same assertion runs on the original
// implementation. Identity, diagnostics, History and breach continuity are all
// part of the user's reading, not just the returned builder value.
func TestGuestPollOwnershipRejectsObsoletePublication(t *testing.T) {
	for _, route := range []string{"cluster", "node"} {
		for _, mode := range []string{"retire", "replace", "runtime", "cancel", "cancel-empty"} {
			for _, phase := range []string{"inventory", "detail"} {
				if mode == "cancel-empty" && phase == "detail" {
					continue
				}
				t.Run(route+"/"+mode+"/"+phase, func(t *testing.T) {
					old := &guestPollOwnershipClient{route: route, cpu: .1, block: phase, entered: make(chan context.Context, 1), release: make(chan struct{}), empty: mode == "cancel-empty"}
					m := guestPollOwnershipMonitor(t, old)
					seed := models.VM{ID: "site-b:node:105", Instance: "site-b", Node: "node", VMID: 105, Name: "peer", Type: "qemu", Status: "running", CPU: .3}
					m.state.UpdateVMsForInstance("site-b", []models.VM{seed})
					m.state.UpdateVMsForInstance("site-a", []models.VM{{ID: "site-a:node:105", Instance: "site-a", Node: "node", VMID: 105, Type: "qemu", Status: "running", CPU: .95}})
					m.updatePVEBackupTemplateSubjectsForType("site-a", "qemu", map[string]struct{}{"original": {}})
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var once sync.Once
					unblock := func() { once.Do(func() { close(old.release) }) }
					defer unblock()
					done := make(chan error, 1)
					go func() { done <- runGuestOwnershipPoll(m, ctx, old) }()
					var readCtx context.Context
					select {
					case readCtx = <-old.entered:
					case <-time.After(5 * time.Second):
						t.Fatal("controlled old inventory/detail did not start")
					}
					switch mode {
					case "retire":
						m.retirePVEInstanceRuntime("site-a")
					case "runtime":
						m.setRuntimeContext(context.Background(), nil)
					case "cancel", "cancel-empty":
						cancel()
					}
					fresh := &guestPollOwnershipClient{route: route, cpu: .95}
					if mode != "cancel" && mode != "cancel-empty" {
						// Tests change the map only while the old read is blocked;
						// production replacement uses the publication lifecycle gate.
						m.mu.Lock()
						m.pveClients["site-a"] = fresh
						m.mu.Unlock()
						if err := runGuestOwnershipPoll(m, context.Background(), fresh); err != nil {
							t.Fatal(err)
						}
						// A threshold observation needs two evaluations to become a
						// breach, regardless of its zero duration.
						for _, vm := range m.state.GetSnapshot().VMs {
							if vm.Instance == "site-a" {
								m.checkGuestAlertsForVM("site-a", vm)
							}
						}
					}
					before := m.state.GetSnapshot()
					diagnostics := m.GetDiagnosticSnapshots().Guests
					vmHistory := m.metricsHistory.GetGuestMetrics("site-a:node:105", "cpu", time.Hour)
					ctHistory := m.metricsHistory.GetGuestMetrics("site-a:node:106", "cpu", time.Hour)
					active := m.alertManager.GetActiveAlerts()
					m.mu.RLock()
					ready := fmt.Sprint(m.pveBackupInventoryReady["site-a"])
					templates := fmt.Sprint(m.pveBackupTemplateSubjects["site-a"])
					m.mu.RUnlock()
					unblock()
					waitGuestOwnership(t, done)
					got := m.state.GetSnapshot()
					if !reflect.DeepEqual(got.VMs, before.VMs) || !reflect.DeepEqual(got.Containers, before.Containers) {
						t.Error("obsolete guest poll replaced or erased accepted inventory")
					}
					if !reflect.DeepEqual(m.GetDiagnosticSnapshots().Guests, diagnostics) {
						t.Error("obsolete guest poll replaced accepted diagnostics")
					}
					if !reflect.DeepEqual(m.metricsHistory.GetGuestMetrics("site-a:node:105", "cpu", time.Hour), vmHistory) || !reflect.DeepEqual(m.metricsHistory.GetGuestMetrics("site-a:node:106", "cpu", time.Hour), ctHistory) {
						t.Error("obsolete guest poll appended History")
					}
					if !reflect.DeepEqual(m.alertManager.GetActiveAlerts(), active) {
						t.Error("obsolete guest poll changed accepted alert breaches")
					}
					m.mu.RLock()
					if fmt.Sprint(m.pveBackupInventoryReady["site-a"]) != ready || fmt.Sprint(m.pveBackupTemplateSubjects["site-a"]) != templates {
						t.Error("obsolete guest poll changed backup inventory eligibility")
					}
					m.mu.RUnlock()
					if mode == "retire" || mode == "runtime" {
						if readCtx.Err() == nil {
							t.Error("ended connection/runtime did not cancel its outstanding guest read")
						}
					}
				})
			}
		}
	}
}

// Failed collection still follows canonical membership, but a merged/enrichment
// view cannot relabel another source's metric as the last Proxmox observation.
func TestGuestPollFailedEnumerationRetainsProducerOwnership(t *testing.T) {
	at := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	original := models.VM{ID: "site-a:node:105", Instance: "site-a", Node: "node", VMID: 105, Type: "qemu", CPU: .95, LastSeen: at,
		Memory: models.Memory{Total: 100, Used: 25, Usage: 25, Observation: models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: at.Add(-time.Minute)}},
		Disk:   models.Disk{Total: 100, Used: 40, Usage: 40}, GuestAgentStatus: "deferred"}
	m := &Monitor{state: models.NewState()}
	ghost := original
	ghost.ID = "site-a:node:106"
	ghost.VMID = 106
	m.state.UpdateGuestsForInstance("site-a", []models.VM{original, ghost}, nil)
	merged := original
	merged.CPU = .01
	merged.Memory = models.Memory{Total: 100, Used: 99, Usage: 99, Observation: models.MemoryObservation{State: "current", Source: "agent", ObservedAt: at.Add(time.Minute)}}
	previous := previousGuestContext{vms: []models.VM{merged}}
	vms, containers := m.retainedGuestsForFailedNodes("site-a", map[string]struct{}{"node": {}}, previous)
	if len(vms) != 1 || len(containers) != 0 || !reflect.DeepEqual(vms[0].Memory, original.Memory) || vms[0].CPU != .95 || vms[0].GuestAgentStatus != "deferred" || vms[0].Disk.Used != 40 || !vms[0].LastSeen.Equal(at) {
		t.Fatalf("failed enumeration changed source ownership/membership: %+v", vms)
	}
	// Efficient inventory's node grace must obey the same source boundary:
	// neither a linked reading nor an unselected raw-state guest can leak in.
	graceVMs, graceContainers := m.preserveGuestsForGracePeriod("site-a", nil, previous,
		[]proxmox.Node{{Node: "node", Status: "offline"}}, map[string]string{"node": "online"}, nil, nil)
	if !reflect.DeepEqual(graceVMs, vms) || !reflect.DeepEqual(graceContainers, containers) {
		t.Fatalf("efficient grace changed source ownership/membership: %+v", graceVMs)
	}
	// Effective Online is also the normal healthy state, not just grace. A
	// current online node with accepted empty inventory must remove its guest.
	onlineVMs, onlineContainers := m.preserveGuestsForGracePeriod("site-a", nil, previous,
		[]proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, nil, nil)
	if len(onlineVMs) != 0 || len(onlineContainers) != 0 {
		t.Fatal("online empty inventory was replaced by stale linked/raw state")
	}
}
