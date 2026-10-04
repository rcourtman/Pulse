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

func testGuestAgentTransportDeferralKeepsLastKnownHistory(t *testing.T) {
	for _, kind := range []string{"lost reply", "redirect", "server error", "gateway error"} {
		t.Run(kind, func(t *testing.T) {
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
						if kind == "server error" || kind == "gateway error" {
							status := http.StatusInternalServerError
							if kind == "gateway error" {
								status = http.StatusBadGateway
							}
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
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
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
				return vm, source
			}
			initial, source := build()
			if source != "guest-agent-meminfo" || initial.Memory.Used != int64(3*mib) || initial.Disk.Used != int64(300*mib) || calls.Load() != 5 {
				t.Fatal("healthy baseline absent")
			}
			memKey, metadataKey := guestMemoryCacheKey("transport", "node", 105), guestMetadataCacheKey("transport", "node", 105)
			memory := m.vmAgentMemCache[memKey]
			memory.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
			m.vmAgentMemCache[memKey] = memory
			metadata := m.guestMetadataCache[metadataKey]
			metadata.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
			m.guestMetadataCache[metadataKey] = metadata
			m.guestMetadataLimiter = make(map[string]time.Time)
			phase.Store(1)
			res.CPU = 0.2
			for i := 0; i < 2; i++ {
				deferred, source := build()
				if deferred.ID != initial.ID || deferred.GuestAgentStatus != "deferred" || deferred.DiskStatusReason != "prev-agent-cooldown" || deferred.Memory.Used != initial.Memory.Used || source != "previous-snapshot" || deferred.Disk.Used != initial.Disk.Used || !reflect.DeepEqual(deferred.NetworkInterfaces, initial.NetworkInterfaces) {
					t.Fatalf("uncertain command lost truthful continuity: source=%s vm=%+v", source, deferred)
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
