package monitoring

import (
	"context"
	"fmt"
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
