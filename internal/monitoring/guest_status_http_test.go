package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The independent control-plane observer must not turn an unfinished or
// contradictory status reply into proof of an action's outcome. No action,
// guest-agent operation or native failure is executed by these HTTP fixtures.
func TestGuestStatusHTTPActionObservation(t *testing.T) {
	for _, clientKind := range []string{"direct", "cluster"} {
		for _, kind := range []string{"qemu", "lxc"} {
			for _, rejection := range []string{"prefix", "truncated", "foreign", "conflicting-error"} {
				t.Run(clientKind+"/"+kind+"/"+rejection, func(t *testing.T) {
					var phase, statuses, commands atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch {
						case strings.HasSuffix(r.URL.Path, "/nodes"):
							fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							statuses.Add(1)
							body := `{"data":{"status":"running","uptime":42,"vmid":105}}`
							if phase.Load() == 1 {
								switch rejection {
								case "prefix":
									body += ` {"data":{"status":"stopped"}}`
								case "truncated":
									w.Header().Set("Content-Length", fmt.Sprint(len(body)+100))
								case "foreign":
									body = `{"data":{"status":"running","uptime":42,"vmid":106}}`
								case "conflicting-error":
									body = `{"data":{"status":"running","uptime":42},"errors":{"reason":"fixture-private-detail"}}`
								}
							} else if phase.Load() == 2 {
								body = `{"data":{"status":"stopped","uptime":0}}`
							}
							fmt.Fprint(w, body)
						default:
							commands.Add(1)
							t.Errorf("unexpected non-status operation: %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard}
					var client PVEClientInterface
					if clientKind == "direct" {
						var err error
						client, err = proxmox.NewClient(cfg)
						if err != nil {
							t.Fatal(err)
						}
					} else {
						client = proxmox.NewClusterClient("status-observer", cfg, []string{server.URL}, nil)
					}
					m := &Monitor{pveClients: map[string]PVEClientInterface{"status-observer": client}}
					for step := int32(0); step < 3; step++ {
						phase.Store(step)
						before := statuses.Load()
						observation, err := m.ObserveProxmoxGuest(context.Background(), "status-observer", "node", 105, kind)
						if step == 1 {
							if err == nil || observation != (ProxmoxGuestObservation{}) {
								t.Errorf("unverified action observation: %+v err=%v", observation, err)
							}
							if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
								t.Error("response body escaped into observer error")
							}
						} else {
							state, uptime := "running", uint64(42)
							if step == 2 {
								state, uptime = "stopped", 0
							}
							if err != nil || observation.Status != state || observation.Uptime != uptime || observation.ObservedAt.IsZero() {
								t.Errorf("valid independent observation lost: %+v err=%v", observation, err)
							}
						}
						if statuses.Load()-before != 1 {
							t.Errorf("status admission added retry: %d", statuses.Load()-before)
						}
					}
					if commands.Load() != 0 {
						t.Error("observer added guest operations")
					}
				})
			}
		}
	}
}

type guestStatusHTTPFrame struct {
	Phase         string          `json:"phase"`
	VM            models.VM       `json:"vm"`
	Resources     json.RawMessage `json:"resources"`
	MemoryHistory []MetricPoint   `json:"memoryHistory"`
}

func guestStatusHTTPMonitoringJourney(t *testing.T, clientKind, collector, rejection string) []guestStatusHTTPFrame {
	t.Helper()
	const gib = uint64(1024 * 1024 * 1024)
	var phase, commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			available := uint64(0)
			if phase.Load() > 0 {
				available = 3 * gib
			}
			body := fmt.Sprintf(`{"data":{"status":"running","agent":0,"maxmem":%d,"mem":%d,"meminfo":{"total":%d,"available":%d},"uptime":42}}`, 8*gib, 8*gib, 4*gib, available)
			if phase.Load() == 1 || phase.Load() == 2 {
				switch rejection {
				case "prefix":
					body += ` {"data":null}`
				case "truncated":
					w.Header().Set("Content-Length", fmt.Sprint(len(body)+100))
				case "missing":
					body = `{}`
				case "conflicting-error":
					body = body[:len(body)-1] + `,"errors":{"message":"fixture-private-detail"}}`
				}
			}
			fmt.Fprint(w, body)
		default:
			commands.Add(1)
			t.Errorf("status-only fixture issued extra operation: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard}
	var client PVEClientInterface
	if clientKind == "direct" {
		var err error
		client, err = proxmox.NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		client = proxmox.NewClusterClient("status-integrity", cfg, []string{server.URL}, nil)
	}
	m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
	registry := unifiedresources.NewRegistry(nil)
	id := makeGuestID("status-integrity", "node", 105)
	previous := map[string]models.VM{}
	var frames []guestStatusHTTPFrame
	for step, name := range []string{"accepted-pressure", "unread-status", "repeated-unread", "recovered"} {
		phase.Store(int32(step))
		ctx, cancel := context.WithTimeout(context.Background(), guestPollFixtureHangGuard)
		cycleStart := time.Now()
		res := proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 105, Name: "status-integrity-guest", Status: "running", CPU: .2 + float64(step)/10, MaxMem: 8 * gib, Mem: 8 * gib, MaxDisk: gib}
		var vms []models.VM
		if collector == "cluster" {
			vms = m.collectClusterVMResources(ctx, "status-integrity", []indexedClusterResource{{resource: res, guestID: id}}, client, previous, nil)
		} else {
			vms, _ = m.pollNodeVMsWithClusterResourceBuilder(ctx, "status-integrity", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, CPU: res.CPU, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk}}, client, previous, nil)
		}
		cancel()
		if len(vms) != 1 {
			t.Fatalf("collector lost independently listed guest: %d", len(vms))
		}
		vm := vms[0]
		m.recordGuestMetrics(vms, nil, cycleStart)
		registry.IngestSnapshot(models.StateSnapshot{VMs: vms})
		views := registry.VMs()
		if len(views) != 1 {
			t.Fatal("canonical projection lost guest")
		}
		if vm.CPU != res.CPU || vm.Status != "running" {
			t.Error("status rejection stopped independent guest telemetry")
		}
		switch step {
		case 0:
			if vm.Memory.Usage != 100 || vm.Memory.Observation.State != "current" || vm.Memory.Observation.Source != "available-field" {
				t.Fatalf("initial pressure did not establish evidence: %+v", vm.Memory)
			}
		case 1, 2:
			original := frames[0].VM.Memory
			if vm.Memory.Usage != 100 || vm.Memory.Observation.State != "last-known" || !vm.Memory.Observation.ObservedAt.Equal(original.Observation.ObservedAt) {
				t.Errorf("rejected status invented/renewed healthy memory: %+v", vm.Memory)
			}
			if views[0].MemoryEvidence(time.Now()).PressureKnown {
				t.Error("retained memory became current pressure")
			}
			if len(m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)) != 1 {
				t.Error("rejected status added current memory History")
			}
		case 3:
			if vm.Memory.Usage != 25 || vm.Memory.Observation.State != "current" {
				t.Errorf("valid later status did not recover: %+v", vm.Memory)
			}
		}
		front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: vms}, m.mockModeFence.begin())
		wire, err := json.Marshal(front.Resources)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, guestStatusHTTPFrame{Phase: name, VM: vm, Resources: wire, MemoryHistory: m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)})
		previous[id] = previousVMFromView(views[0])
	}
	if commands.Load() != 0 {
		t.Error("status rejection queued a guest operation")
	}
	history := m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)
	if len(history) != 2 || history[0].Value != 100 || history[1].Value != 25 {
		t.Errorf("History accepted rejected reading or lost recovery: %+v", history)
	}
	return frames
}

func TestGuestStatusHTTPMonitoringContract(t *testing.T) {
	for _, client := range []string{"direct", "cluster"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, rejection := range []string{"prefix", "truncated", "missing", "conflicting-error"} {
				t.Run(client+"/"+collector+"/"+rejection, func(t *testing.T) { guestStatusHTTPMonitoringJourney(t, client, collector, rejection) })
			}
		}
	}
}
