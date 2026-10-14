package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func testGuestAgentTransportDeferralKeepsLastKnownHistory(t *testing.T, kinds ...string) {
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			preserveOrigins := strings.HasPrefix(kind, "typed ") || strings.HasPrefix(kind, "foreign ")
			const mib = uint64(1024 * 1024)
			var phase, calls, redirected atomic.Int32
			var lost atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 8*mib, 8*mib)
				case strings.Contains(r.URL.Path, "/agent/"):
					calls.Add(1)
					if r.URL.Query().Get("redirect") == "1" {
						redirected.Add(1)
					}
					if phase.Load() == 1 && lost.CompareAndSwap(false, true) {
						switch kind {
						case "malformed success":
							fmt.Fprint(w, `{"data":{"result":`)
							return
						case "ambiguous success":
							fmt.Fprint(w, `{"data":null,"data":{"result":[]}}`)
							return
						case "trailing success":
							fmt.Fprint(w, `{"data":{"result":[]}} {}`)
							return
						case "typed result":
							fmt.Fprint(w, `{"data":{"result":"wrong command id"}}`)
							return
						case "typed content":
							fmt.Fprint(w, `{"data":{"result":[],"content":false}}`)
							return
						case "typed truncated":
							fmt.Fprint(w, `{"data":{"result":[],"truncated":"false"}}`)
							return
						case "foreign file":
							fmt.Fprint(w, `{"data":{"content":"MemTotal: 8192 kB\nMemAvailable: 8192 kB\n","truncated":false}}`)
							return
						case "foreign network":
							fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","ip-addresses":[]}]}}`)
							return
						case "foreign version":
							fmt.Fprint(w, `{"data":{"result":{"version":"1.0","supported_commands":[]}}}`)
							return
						}
						if status := map[string]int{
							"server error":           http.StatusInternalServerError,
							"gateway error":          http.StatusBadGateway,
							"conflict response":      http.StatusConflict,
							"too early response":     http.StatusTooEarly,
							"client closed response": 499,
						}[kind]; status != 0 {
							w.WriteHeader(status)
							fmt.Fprint(w, "upstream unavailable")
							return
						}
						if kind == "redirect" {
							http.Redirect(w, r, r.URL.Path+"?redirect=1", http.StatusTemporaryRedirect)
							return
						}
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						conn.Close()
						return
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "file-read"):
						available := 5120
						if phase.Load() == 1 {
							available = 4096
						}
						json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": fmt.Sprintf("MemTotal: 8192 kB\nMemAvailable: %d kB\n", available)}})
					case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
						fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, 300*mib)
					case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
						fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
					case strings.HasSuffix(r.URL.Path, "get-osinfo"):
						fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
					default:
						fmt.Fprint(w, `{"data":{"result":{"version":"1.0"}}}`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			host := server.URL
			timeout := time.Second
			if strings.HasPrefix(kind, "foreign ") {
				timeout = 15 * time.Second
				host += "/command-reply-fixture/" + strings.ReplaceAll(kind, " ", "-")
			}
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: timeout})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
			if preserveOrigins {
				m.state = models.NewState()
			}
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib, CPU: 0.1}
			identity := "transport:node:105"
			var previous *models.VM
			build := func() (models.VM, string) {
				vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "transport", res, client, identity, nil, previous)
				if !ok {
					t.Fatal("VM disappeared")
				}
				m.recordGuestSnapshot("transport", "qemu", "node", 105, GuestMemorySnapshot{Name: vm.Name, Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
				m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
				previous = &vm
				if preserveOrigins {
					// The safe fixture's actual builder/broadcast output is also
					// available for consuming browser proof. Do not hand-author a
					// fresh/last-known carrier or replace its observation time.
					m.state.UpdateVMs([]models.VM{vm})
					front := m.buildBroadcastFrontendStateFromSnapshot(m.state.GetSnapshot(), m.mockModeFence.begin())
					if len(front.Resources) != 1 {
						t.Fatal("typed-reply producer lost the canonical guest")
					}
					wire, err := json.Marshal(map[string]any{
						"kind": kind, "phase": phase.Load(), "guestCommands": calls.Load(),
						"vm": vm, "resources": front.Resources,
						"history": m.metricsHistory.GetAllGuestMetrics(identity, time.Hour),
					})
					if err != nil {
						t.Fatal(err)
					}
					marker := "TYPED_REPLY_PRODUCER"
					if strings.HasPrefix(kind, "foreign ") {
						marker = "COMMAND_REPLY_PRODUCER"
					}
					t.Logf("%s %s", marker, wire)
				}
				return vm, source
			}
			initial, source := build()
			if source != "guest-agent-meminfo" || initial.Memory.Used != int64(3*mib) || initial.Disk.Used != int64(300*mib) || calls.Load() != 5 {
				t.Fatal("healthy baseline absent")
			}
			memKey, metadataKey := guestMemoryCacheKey("transport", "node", 105), guestMetadataCacheKey("transport", "node", 105)
			memory := m.vmAgentMemCache[memKey]
			metadata := m.guestMetadataCache[metadataKey]
			if !preserveOrigins {
				memory.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
				m.vmAgentMemCache[memKey] = memory
				metadata.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
				m.guestMetadataCache[metadataKey] = metadata
			}
			// Typed-reply controls retain the real healthy read's original cache
			// and observation times. fetchedAt also owns measurement provenance;
			// manually aging it would invalidate an unchanged-origin assertion.
			// The filesystem is always read before these optional cached fields,
			// while the six-reader wire controls exercise fresh uncached clients.
			m.guestMetadataLimiter = make(map[string]time.Time)
			phase.Store(1)
			res.CPU = 0.2
			wantMemorySource := "previous-snapshot"
			if preserveOrigins {
				wantMemorySource = "guest-agent-meminfo" // The untouched warm cache retains its actual origin.
			}
			for i := 0; i < 2; i++ {
				deferred, source := build()
				wantReason := "prev-agent-cooldown"
				if i == 0 {
					// Filesystem now receives the original error, before optional
					// reads observe the resulting shared cooldown.
					wantReason = "prev-agent-completion-unverified"
					if kind == "lost reply" {
						wantReason = "prev-agent-timeout"
					} else if kind == "redirect" {
						wantReason = "prev-agent-redirect"
					}
				}
				if deferred.ID != initial.ID || deferred.GuestAgentStatus != "deferred" || deferred.DiskStatusReason != wantReason || deferred.Memory.Used != initial.Memory.Used || source != wantMemorySource || deferred.Disk.Used != initial.Disk.Used || !reflect.DeepEqual(deferred.NetworkInterfaces, initial.NetworkInterfaces) {
					t.Fatalf("uncertain command lost truthful continuity: source=%s vm=%+v", source, deferred)
				}
				if preserveOrigins && (deferred.Memory.Observation.State != "last-known" || !deferred.Memory.Observation.ObservedAt.Equal(initial.Memory.Observation.ObservedAt)) {
					t.Fatal("typed reply renewed or promoted the original memory observation")
				}
				if !reflect.DeepEqual(m.vmAgentMemCache[memKey], memory) || !reflect.DeepEqual(m.guestMetadataCache[metadataKey], metadata) {
					t.Fatal("uncertain command renewed or replaced cached evidence")
				}
			}
			if calls.Load() != 6 || redirected.Load() != 0 {
				t.Fatalf("commands/redirected after uncertainty = %d/%d, want 6/0", calls.Load(), redirected.Load())
			}
			for metric, want := range map[string]int{"cpu": 3, "memory": 1, "memoryused": 1, "disk": 1} {
				if got := len(m.metricsHistory.GetGuestMetrics(identity, metric, time.Hour)); got != want {
					t.Errorf("%s History points = %d, want %d", metric, got, want)
				}
			}
		})
	}
}
