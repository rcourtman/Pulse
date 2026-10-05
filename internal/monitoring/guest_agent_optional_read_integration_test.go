package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// These are deliberately synthetic protocol outcomes, not captured responses
// from discussion #2538. Filesystem success is independent of optional Linux
// memory/metadata support, but an uncertain command still stops every later read.
func testGuestAgentOptionalReadOrdering(t *testing.T, withoutStatus bool) {
	for _, clientKind := range []string{"client", "cluster"} {
		for _, tc := range []struct {
			name, failure, os, fs string
			terminal              bool
		}{
			{"windows-unsupported-memory", "file-read", "Microsoft Windows 11", "ntfs", true},
			{"android-unsupported-os", "get-osinfo", "Android", "ext4", true},
			{"uncertain-memory", "file-read", "Microsoft Windows 11", "ntfs", false},
			{"uncertain-network", "network-get-interfaces", "Android", "ext4", false},
			{"uncertain-os", "get-osinfo", "Android", "ext4", false},
			{"uncertain-version", "info", "Microsoft Windows 11", "ntfs", false},
		} {
			if withoutStatus && tc.failure != "get-osinfo" {
				continue // No-status enrichment does not issue a memory fallback.
			}
			t.Run(clientKind+"/"+tc.name, func(t *testing.T) {
				const mib = uint64(1024 * 1024)
				var poll atomic.Int32
				var mu sync.Mutex
				var commands []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
						t.Error("lost configured authentication")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					parts := strings.Split(r.URL.Path, "/")
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						fmt.Fprint(w, `{"data":{}}`)
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						if withoutStatus {
							w.WriteHeader(http.StatusForbidden)
							return
						}
						fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 8*mib, 8*mib)
					case strings.Contains(r.URL.Path, "/agent/"):
						command := parts[len(parts)-1]
						id, _ := strconv.Atoi(parts[len(parts)-3])
						mu.Lock()
						commands = append(commands, fmt.Sprintf("%d/%s", id, command))
						mu.Unlock()
						if id == 105 && command == tc.failure {
							w.WriteHeader(http.StatusInternalServerError)
							if tc.terminal {
								fmt.Fprint(w, "unsupported command: guest-"+command)
							} else {
								// A complete but unexplained 500, even mentioning a
								// missing Linux file, is NOT proof of completion.
								fmt.Fprint(w, "optional guest read failed: /proc/meminfo unavailable")
							}
							return
						}
						switch command {
						case "get-fsinfo":
							json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": []any{map[string]any{"mountpoint": map[bool]string{true: `C:\`, false: "/"}[tc.fs == "ntfs"], "type": tc.fs, "total-bytes": 1000 * mib, "used-bytes": uint64(200+poll.Load()*100) * mib}}}})
						case "file-read":
							if r.URL.Query().Get("file") != "/proc/meminfo" {
								t.Error("unexpected guest file")
							}
							json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": "MemTotal: 8192 kB\nMemAvailable: 5120 kB\n"}})
						case "network-get-interfaces":
							fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
						case "get-osinfo":
							json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"name": tc.os, "version": "fixture"}})
						case "info":
							fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
						default:
							t.Error("unexpected guest command", command)
							http.NotFound(w, r)
						}
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				newClient := func() PVEClientInterface {
					cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}
					if clientKind == "cluster" {
						return proxmox.NewClusterClient("optional", cfg, []string{server.URL}, nil)
					}
					client, err := proxmox.NewClient(cfg)
					if err != nil {
						t.Fatal(err)
					}
					return client
				}
				client := newClient()
				m := guestHistoryObservationMonitor(t)
				m.rateTracker = NewRateTracker()
				m.guestMetadataLimiter = make(map[string]time.Time)
				registry := unifiedresources.NewRegistry(nil)
				m.resourceStore = unifiedresources.NewMonitorAdapter(registry)
				res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib, CPU: .1}
				id := makeGuestID("optional", "node", 105)
				var previous *models.VM
				if withoutStatus {
					previous = &models.VM{ID: id, GuestAgentStatus: "available", Status: "running", LastSeen: time.Now()}
				}
				build := func() models.VM {
					vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "optional", res, client, id, nil, previous)
					if !ok {
						t.Fatal("guest disappeared")
					}
					m.recordGuestSnapshot("optional", "qemu", "node", 105, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
					m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
					registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
					view := registry.VMs()[0]
					canonicalID, resolved := registry.ResolveReferenceID(id)
					if !resolved || view.ID() != canonicalID || view.DiskUsed() != vm.Disk.Used || view.DiskStatusReason() != vm.DiskStatusReason {
						t.Fatalf("filesystem observation changed at the read boundary: id=%q resolved=%q ok=%v used=%d/%d reason=%q/%q", view.ID(), canonicalID, resolved, view.DiskUsed(), vm.Disk.Used, view.DiskStatusReason(), vm.DiskStatusReason)
					}
					encoded, err := json.Marshal(m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}).Resources)
					if err != nil || !strings.Contains(string(encoded), canonicalID) {
						t.Fatal("guest identity missing from JSON read projection")
					}
					next := previousVMFromView(view)
					previous = &next
					return vm
				}
				poll.Store(1)
				initial := build()
				if initial.Disk.Used != int64(300*mib) || initial.Disk.Usage != 30 || initial.DiskStatusReason != "" {
					t.Fatalf("optional %s starved independent filesystem success: used=%d usage=%v reason=%q", tc.failure, initial.Disk.Used, initial.Disk.Usage, initial.DiskStatusReason)
				}
				if !tc.terminal && initial.GuestAgentStatus != "deferred" {
					t.Fatal("successful filesystem concealed later command uncertainty")
				}
				if tc.terminal && initial.GuestAgentStatus == "deferred" {
					t.Fatal("completed unsupported command became a shared pause")
				}
				mu.Lock()
				firstCommands := append([]string(nil), commands...)
				mu.Unlock()
				if firstCommands[0] != "105/get-fsinfo" {
					t.Fatalf("first command = %q, want filesystem before optional reads", firstCommands[0])
				}
				memoryKey := guestMemoryCacheKey("optional", "node", 105)
				originalMemory := m.vmAgentMemCache[memoryKey]
				originalDisk := guestHistoryStoredPoints(t, m, "vm", id, "disk")
				if len(originalDisk) != 1 || originalDisk[0].Value != 30 {
					t.Fatal("initial filesystem did not reach persistent History")
				}
				for i := int32(2); i <= 3; i++ {
					// Distinct persisted timestamps ensure a falsely renewed value
					// cannot hide behind a same-second storage key.
					time.Sleep(1100 * time.Millisecond)
					poll.Store(i)
					client = newClient() // A new diagnostic/cluster client cannot escape admission.
					vm := build()
					if tc.terminal {
						if vm.Disk.Usage != float64(20+i*10) || vm.DiskStatusReason != "" || vm.GuestAgentStatus == "deferred" {
							t.Fatal("completed unsupported reply prevented repeated fresh filesystem polls")
						}
					} else {
						if vm.Disk.Used != initial.Disk.Used || vm.DiskStatusReason != "prev-agent-cooldown" || vm.GuestAgentStatus != "deferred" {
							t.Fatalf("uncertainty lost truthful retained disk: %+v", vm.Disk)
						}
						if !reflect.DeepEqual(m.vmAgentMemCache[memoryKey], originalMemory) || (originalMemory.info.Source != "" && !vm.Memory.Observation.ObservedAt.Equal(initial.Memory.Observation.ObservedAt)) {
							t.Fatal("cooldown renewed original memory evidence")
						}
					}
				}
				if !tc.terminal {
					mu.Lock()
					unchanged := reflect.DeepEqual(commands, firstCommands)
					mu.Unlock()
					if !unchanged || !reflect.DeepEqual(guestHistoryStoredPoints(t, m, "vm", id, "disk"), originalDisk) {
						t.Fatal("cooldown dispatched another command or renewed persistent disk History")
					}
					// A cold collector on the same endpoint shares the live process's
					// guard. This is NOT a new Pulse process or native restart proof.
					cold := &Monitor{rateTracker: NewRateTracker(), guestMetadataLimiter: make(map[string]time.Time)}
					vm, _, _, _, _, _ := cold.buildVMFromClusterResource(context.Background(), "optional", res, newClient(), id, nil, nil)
					if !withoutStatus && (vm.Disk.Usage != -1 || vm.DiskStatusReason != "agent-cooldown" || vm.GuestAgentStatus != "deferred") {
						t.Fatal("cold collector bypassed active cooldown or fabricated a disk")
					}
					mu.Lock()
					unchanged = reflect.DeepEqual(commands, firstCommands)
					mu.Unlock()
					if !unchanged {
						t.Fatal("cold collector sent a command during active cooldown")
					}
				}
				wantDiskPoints := 1
				if tc.terminal {
					wantDiskPoints = 3
				}
				for _, points := range [][]MetricPoint{m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour), m.GetGuestMetricsForChart(id, "vm", id, time.Hour)["disk"]} {
					if len(points) != wantDiskPoints {
						t.Fatalf("disk History/chart has %d points, want %d", len(points), wantDiskPoints)
					}
				}
				if len(guestHistoryStoredPoints(t, m, "vm", id, "disk")) != wantDiskPoints || len(m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)) != 3 {
					t.Fatal("persistent disk or independent CPU history disagreed with polling")
				}
			})
		}
	}
}
