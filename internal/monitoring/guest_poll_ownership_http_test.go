package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Actual single/cluster clients, actual production enumeration and a consumed
// synthetic filesystem response. Cancellation must withdraw the cycle, not
// reinterpret unknown command completion as permission to send another command.
func TestGuestPollOwnershipHTTPWithdrawalPreservesSharedFence(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, route := range []string{"cluster", "node"} {
			for _, withdrawal := range []string{"retire", "runtime"} {
				t.Run(kind+"/"+route+"/"+withdrawal, func(t *testing.T) {
					f := &windowsBootstrapFixture{t: t, osName: bootstrapWindowsOS}
					f.poll.Store(1)
					var vmid, fsCalls atomic.Int32
					vmid.Store(105)
					entered, release, handlerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
					var once sync.Once
					unblock := func() { once.Do(func() { close(release) }) }
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						resource := proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: int(vmid.Load()), Name: "guest", Status: "running", CPU: .2, MaxMem: 8 * bootstrapMiB, Mem: 3 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB}
						switch {
						case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
							if route == "node" {
								http.Error(w, "fixture endpoint unavailable", http.StatusNotFound)
							} else {
								_ = json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.ClusterResource{resource}})
							}
						case strings.HasSuffix(r.URL.Path, "/node/qemu"):
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.VM{{VMID: resource.VMID, Name: resource.Name, Status: resource.Status, CPU: resource.CPU, MaxMem: resource.MaxMem, Mem: resource.Mem, MaxDisk: resource.MaxDisk}}})
						case strings.HasSuffix(r.URL.Path, "/node/lxc"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.Contains(r.URL.Path, "/qemu/105/agent/get-fsinfo"):
							if fsCalls.Add(1) != 1 {
								t.Error("withdrawal cleared consumed filesystem uncertainty")
							}
							w.WriteHeader(http.StatusOK)
							w.(http.Flusher).Flush()
							close(entered)
							<-release // Deliberately ignore the disconnected reader.
							fmt.Fprint(w, `{"data":{"result":[]}}`)
							close(handlerDone)
						default:
							f.serve(w, r)
						}
					}))
					defer server.Close()
					defer unblock()
					old := windowsBootstrapClient(t, server.URL, kind)
					m := guestPollOwnershipMonitor(t, old)
					done := make(chan error, 1)
					go func() { done <- runGuestOwnershipPoll(m, context.Background(), old) }()
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("production filesystem read did not consume the fixture headers")
					}
					if withdrawal == "retire" {
						m.retirePVEInstanceRuntime("site-a")
					} else {
						m.setRuntimeContext(context.Background(), nil)
					}
					waitGuestOwnership(t, done)
					if len(m.state.GetSnapshot().VMs) != 0 || len(m.GetDiagnosticSnapshots().Guests) != 0 || len(m.metricsHistory.GetGuestMetrics("site-a:node:105", "cpu", time.Hour)) != 0 || len(guestHistoryStoredPoints(t, m, "vm", "site-a:node:105", "cpu")) != 0 {
						t.Error("withdrawn HTTP cycle published inventory, diagnostics or either History")
					}
					fresh := windowsBootstrapClient(t, server.URL, kind)
					m.replacePVEClient("site-a", fresh)
					if _, err := fresh.GetVMAgentInfo(context.Background(), "node", 105); proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" {
						t.Errorf("replacement diagnostic lost shared consumed-read fence: %v", err)
					}
					if err := runGuestOwnershipPoll(m, context.Background(), fresh); err != nil {
						t.Fatal(err)
					}
					rows := m.state.GetSnapshot().VMs
					if len(rows) != 1 || rows[0].GuestAgentStatus != "deferred" || rows[0].Memory.Observation.Source != "status-mem" || fsCalls.Load() != 1 {
						t.Error("ordinary cycle lost independent status memory or resent the uncertain filesystem command")
					}
					vmid.Store(106)
					if err := runGuestOwnershipPoll(m, context.Background(), fresh); err != nil {
						t.Fatal(err)
					}
					rows = m.state.GetSnapshot().VMs
					if len(rows) != 1 || rows[0].VMID != 106 || rows[0].OSName != bootstrapWindowsOS || len(rows[0].Disks) != 2 || rows[0].GuestAgentStatus != "available" {
						t.Error("withdrawal blocked independent guest OS/filesystem collection")
					}
					unblock()
					select {
					case <-handlerDone:
					case <-time.After(5 * time.Second):
						t.Fatal("controlled consumed-response handler did not finish")
					}
					if fsCalls.Load() != 1 || len(m.vmAgentMemInFlight) != 0 || len(m.pveGuestPolls) != 0 {
						t.Error("withdrawal leaked owners or replayed the consumed command")
					}
				})
			}
		}
	}
}
