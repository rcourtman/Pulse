package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	missingStatusSite     = "sample-pve"
	missingStatusNode     = "pve-node"
	missingStatusName     = "truenas-vm"
	missingStatusCapacity = 10_000 * 1024 * 1024
)

type missingStatusPoll struct {
	vm                models.VM
	raw               VMMemoryRaw
	alerts            []models.Alert
	listedAt, agentAt time.Time
}

type nilVMStatusClient struct {
	PVEClientInterface
}

func (nilVMStatusClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	return nil, nil
}

// Use the same real link resolution and monitor input as the next poll. The
// PVE HTTP fixture can lose status independently of the Pulse report.
func pollQEMUWithIndependentMemory(t *testing.T, collector, status, agentCase string) missingStatusPoll {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	agentAt, listedAt := now.Add(-25*time.Second), now.Add(-2*time.Second)
	id := makeGuestID(missingStatusSite, missingStatusNode, 105)
	listing := proxmox.ClusterResource{
		Type: "qemu", Status: "running", Node: missingStatusNode, VMID: 105, Name: missingStatusName,
		MaxMem: missingStatusCapacity, Mem: missingStatusCapacity * 92 / 100,
		CPU: .08, MaxCPU: 4, Uptime: 3600, ObservedAt: listedAt,
	}
	memory := models.Memory{
		Total: missingStatusCapacity, Used: missingStatusCapacity * 12 / 100,
		Cache: missingStatusCapacity * 80 / 100, Free: missingStatusCapacity * 8 / 100, Usage: 12,
	}
	switch agentCase {
	case "expired":
		agentAt = now.Add(-time.Hour)
	case "future":
		agentAt = now.Add(time.Hour)
	case "oversized":
		memory.Total *= 2
	case "unavailable":
		memory.UsageUnavailable = true
	case "no-capacity":
		listing.MaxMem, listing.Mem = 0, 0
	case "stopped":
		listing.Status = "stopped"
	case "backup-lock", "backup-lock-unlinked":
		listing.Lock = "backup"
	case "pressure":
		memory.Used, memory.Cache, memory.Free, memory.Usage = missingStatusCapacity*90/100, missingStatusCapacity*2/100, missingStatusCapacity*8/100, 90
	}
	snapshot := models.StateSnapshot{VMs: []models.VM{{
		ID: id, Instance: missingStatusSite, Node: missingStatusNode, VMID: 105,
		Name: missingStatusName, Status: listing.Status, LastSeen: listedAt,
	}}}
	switch agentCase {
	case "ambiguous":
		// Same node name and VMID in a different instance cannot establish a link.
		snapshot.VMs = append(snapshot.VMs, models.VM{
			ID: makeGuestID("other-pve", missingStatusNode, 105), Instance: "other-pve",
			Node: missingStatusNode, VMID: 105, Name: missingStatusName, Status: "running", LastSeen: listedAt,
		})
	case "foreign":
		snapshot.VMs[0].Name = "other-vm"
		snapshot.VMs = append(snapshot.VMs, models.VM{
			ID: makeGuestID("other-pve", missingStatusNode, 105), Instance: "other-pve",
			Node: missingStatusNode, VMID: 105, Name: missingStatusName, Status: "running", LastSeen: listedAt,
		})
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	cfg := &config.Config{}
	mon := &Monitor{config: cfg, state: models.NewState(), rateTracker: NewRateTracker(), resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	const hostname = "truenas-vm.example.test"
	_, link, _ := mon.findLinkedProxmoxEntityWithHints(hostname, "", nil)
	switch agentCase {
	case "ambiguous":
		if link != "" {
			t.Fatalf("ambiguous name unexpectedly linked to %q", link)
		}
	case "foreign":
		if link != makeGuestID("other-pve", missingStatusNode, 105) {
			t.Fatalf("foreign name matched the wrong guest: %q", link)
		}
	default:
		if link != id {
			t.Fatalf("unambiguous VM did not link: %q, want %q", link, id)
		}
	}
	if agentCase != "backup-lock-unlinked" {
		snapshot.Hosts = []models.Host{{
			ID: "in-guest-agent", Hostname: hostname, LinkedVMID: link,
			Status: "online", LastSeen: agentAt, Memory: memory,
		}}
		registry.IngestSnapshot(snapshot)
	}
	prev := mon.previousGuestContextForInstance(missingStatusSite)

	am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(am.Stop)
	alertConfig := am.GetConfig()
	alertConfig.Enabled = true
	alertConfig.ActivationState = alerts.ActivationActive
	alertConfig.GuestDefaults = alerts.ThresholdConfig{Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}}
	alertConfig.TimeThresholds = map[string]int{"guest": 0}
	alertConfig.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 0}}
	am.UpdateConfig(alertConfig)
	mon.alertManager = am

	var statusCalls, guestCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			statusCalls.Add(1)
			switch status {
			case "present":
				fmt.Fprintf(w, `{"data":{"status":"running","agent":0,"maxmem":%d,"mem":%d,"meminfo":{"total":%d,"used":%d,"available":%d}}}`, listing.MaxMem, listing.Mem, listing.MaxMem, listing.Mem, listing.MaxMem-listing.Mem)
			case "empty":
				fmt.Fprint(w, `{"data":null}`)
			default:
				http.Error(w, "fixture detail unavailable", http.StatusServiceUnavailable)
			}
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{"onboot":0}}`)
		case strings.Contains(r.URL.Path, "/agent/"):
			guestCalls.Add(1)
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	native, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard})
	if err != nil {
		t.Fatal(err)
	}
	var client PVEClientInterface = native
	if status == "nil" {
		client = nilVMStatusClient{client}
	}

	var vms []models.VM
	for range 2 { // A second poll also exercises the existing alert duration gate.
		if collector == "node-list" {
			vms, _ = mon.pollNodeVMsWithClusterResourceBuilder(context.Background(), missingStatusSite, missingStatusNode,
				[]proxmox.VM{{VMID: listing.VMID, Name: listing.Name, Status: listing.Status, Mem: listing.Mem,
					MaxMem: listing.MaxMem, CPU: listing.CPU, CPUs: listing.MaxCPU, Uptime: listing.Uptime, Lock: listing.Lock, ObservedAt: listedAt}},
				client, prev.vmsByID, prev.hostAgentsByVMID)
		} else {
			vms = mon.collectClusterVMResources(context.Background(), missingStatusSite,
				[]indexedClusterResource{{resource: listing, guestID: id}}, client, prev.vmsByID, prev.hostAgentsByVMID)
		}
		if len(vms) != 1 {
			t.Fatalf("collector lost the VM: %v", vms)
		}
	}
	wantCalls := int32(2)
	if status == "nil" || listing.Status != "running" {
		wantCalls = 0
	}
	if statusCalls.Load() != wantCalls || guestCalls.Load() != 0 {
		t.Fatalf("detail calls %d, guest commands %d; want %d detail calls and no guest commands", statusCalls.Load(), guestCalls.Load(), wantCalls)
	}
	diag := mon.previousGuestSnapshot(missingStatusSite, "qemu", missingStatusNode, 105)
	if diag == nil {
		t.Fatal("diagnostics lost the VM")
	}
	return missingStatusPoll{vms[0], diag.Raw, mon.ActiveAlertsSnapshot(), listedAt, agentAt}
}

func TestQEMUCurrentLinkedMemoryWithoutDetailedStatus(t *testing.T) {
	for _, collector := range []string{"cluster-resources", "node-list"} {
		for _, status := range []string{"present", "error", "empty", "nil"} {
			t.Run(collector+"/"+status, func(t *testing.T) {
				got := pollQEMUWithIndependentMemory(t, collector, status, "current")
				m := got.vm.Memory
				if m.Observation.State != "current" || m.Observation.Source != "agent" || !m.Observation.ObservedAt.Equal(got.agentAt) ||
					m.Used != missingStatusCapacity*12/100 || m.Total != missingStatusCapacity || m.Cache != missingStatusCapacity*80/100 ||
					m.Free != missingStatusCapacity*8/100 || m.Usage != 12 {
					t.Fatalf("selected memory = %+v; want the agent's 12%%, partition and original time (%s)", m, got.agentAt)
				}
				if got.raw.HostAgentUsed != uint64(m.Used) || got.raw.HostAgentTotal != uint64(m.Total) || len(got.alerts) != 0 {
					t.Fatalf("mismatched diagnostic source or unexpected alert: raw=%+v alerts=%+v", got.raw, got.alerts)
				}
				r := unifiedresources.NewRegistry(nil)
				r.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{got.vm}})
				view := r.VMs()[0]
				evidence := view.MemoryEvidence(time.Now())
				if view.MemoryPercent() != 12 || view.MemoryUsed() != m.Used || !evidence.PressureKnown ||
					evidence.Source != "agent" || !evidence.ObservedAt.Equal(got.agentAt) {
					t.Fatalf("registry disagrees with selected memory: %v%%, evidence=%+v", view.MemoryPercent(), evidence)
				}
			})
		}
	}
}

func TestQEMUNoDetailedStatusKeepsIndependentMemoryGuards(t *testing.T) {
	for _, agentCase := range []string{"expired", "future", "ambiguous", "foreign", "oversized", "unavailable"} {
		t.Run(agentCase, func(t *testing.T) {
			got := pollQEMUWithIndependentMemory(t, "cluster-resources", "error", agentCase)
			m := got.vm.Memory
			if m.Usage != 92 || m.Cache != 0 || m.Observation.Source != "cluster-resources" || !m.Observation.ObservedAt.Equal(got.listedAt) ||
				got.raw.HostAgentTotal != 0 {
				t.Fatalf("ineligible agent changed the existing listing fallback: %+v", m)
			}
		})
	}
	for _, tc := range []struct{ agentCase, source string }{
		{"no-capacity", "unavailable"}, {"stopped", "powered-off"},
		{"backup-lock", "agent"}, {"backup-lock-unlinked", "unavailable"}, {"pressure", "agent"},
	} {
		t.Run(tc.agentCase, func(t *testing.T) {
			got := pollQEMUWithIndependentMemory(t, "cluster-resources", "error", tc.agentCase)
			if got.vm.Memory.Observation.Source != tc.source {
				t.Fatalf("unexpected source for %s: %+v", tc.agentCase, got.vm.Memory)
			}
			if strings.HasPrefix(tc.agentCase, "backup-lock") && got.vm.GuestAgentStatus != "deferred" {
				t.Fatal("an independent memory reading must not change QGA lock deferral")
			}
			if tc.agentCase == "pressure" {
				if len(got.alerts) != 1 || got.alerts[0].Type != "memory" || got.alerts[0].Value != 90 {
					t.Fatalf("real pressure stopped alerting: %+v", got.alerts)
				}
			}
		})
	}
}
