package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A janitor must not erase the command-safety history of a guest that is still
// being polled. Empty replies are not useful observations. All endpoints here
// are synthetic; no guest, backup or uncertainty cooldown is manipulated.
func TestGuestMetadataCleanupSuppressionContract(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, route := range []string{"cluster", "node"} {
			for _, seeded := range []bool{false, true} {
				for _, mode := range []string{"unsupported", "repeated-failure"} {
					t.Run(fmt.Sprintf("%s/%s/cached-%t/%s", kind, route, seeded, mode), func(t *testing.T) {
						const instance = "cleanup-suppression"
						const mib = uint64(1024 * 1024)
						var osCalls, networkCalls, versionCalls, fsCalls atomic.Int32
						var healthy atomic.Bool
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Type", "application/json")
							switch {
							case strings.HasSuffix(r.URL.Path, "/config"):
								fmt.Fprint(w, `{"data":{}}`)
							case strings.HasSuffix(r.URL.Path, "/status/current"):
								fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":8388608,"meminfo":{"total":8388608,"available":5242880}}}`)
							case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
								fsCalls.Add(1)
								fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1048576000,"used-bytes":314572800}]}}`)
							case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
								networkCalls.Add(1)
								if healthy.Load() {
									fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:03","ip-addresses":[{"ip-address":"192.0.2.33","ip-address-type":"ipv4","prefix":24}]}]}}`)
								} else {
									fmt.Fprint(w, `{"data":{"result":[]}}`)
								}
							case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
								osCalls.Add(1)
								if mode == "unsupported" {
									w.WriteHeader(http.StatusInternalServerError)
									fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-get-osinfo' failed - guest agent command failed: Failed to open file '/etc/os-release': No such file or directory"}`)
								} else {
									w.WriteHeader(http.StatusBadRequest)
									fmt.Fprint(w, `{"data":null,"message":"OS info request failed"}`)
								}
							case strings.HasSuffix(r.URL.Path, "/info"):
								versionCalls.Add(1)
								if healthy.Load() {
									fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
								} else {
									fmt.Fprint(w, `{"data":{"result":{}}}`)
								}
							default:
								t.Errorf("unexpected cleanup-fixture request %s", r.URL.Path)
								http.NotFound(w, r)
							}
						}))
						defer server.Close()
						client := windowsBootstrapClient(t, server.URL, kind)
						m := windowsBootstrapMonitor(t)
						m.guestMetadataLimiter = make(map[string]time.Time)
						key := guestMetadataCacheKey(instance, "node", 105)
						original := guestMetadataCacheEntry{}
						if seeded {
							original = metadataObservationFixture(time.Now().Add(-3 * guestMetadataCacheTTL))
							m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
						}
						resource := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib}
						build := func() models.VM {
							if route == "cluster" {
								vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), instance, resource, client, makeGuestID(instance, "node", 105), nil, nil)
								if !ok {
									t.Fatal("VM disappeared")
								}
								return vm
							}
							vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), instance, "node", []proxmox.VM{{VMID: 105, Name: "guest", Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib}}, client, nil, nil)
							if len(vms) != 1 {
								t.Fatal("node VM disappeared")
							}
							return vms[0]
						}
						failures := 1
						if mode == "repeated-failure" {
							failures = guestAgentOSInfoFailureThreshold
						}
						for round := 1; round <= failures+1; round++ {
							// Only advance test-owned metadata scheduling. Do not shorten a QGA
							// cooldown or replay any uncertain command.
							m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
							vm := build()
							before := m.guestMetadataCache[key]
							m.cleanupGuestMetadataCache(time.Now())
							entry, exists := m.guestMetadataCache[key]
							expectedCount := guestAgentOSInfoFailureThreshold
							if mode == "repeated-failure" && round < failures {
								expectedCount = round
							}
							if !exists || entry.osInfoFailureCount != expectedCount || entry.osInfoSkip != (expectedCount == guestAgentOSInfoFailureThreshold) {
								t.Errorf("active safety history erased at round %d: exists=%t failures=%d skip=%t", round, exists, entry.osInfoFailureCount, entry.osInfoSkip)
							}
							if !reflect.DeepEqual(entry, before) || !entry.fetchedAt.Equal(original.fetchedAt) || !entry.osInfoObservedAt.Equal(original.osInfoObservedAt) || m.hasRecentGuestMetadataEvidence(instance, "node", 105, time.Now()) {
								t.Error("cleanup changed retained metadata or renewed stale observation evidence")
							}
							if vm.Disk.Used != int64(300*mib) || vm.DiskStatusReason != "" || vm.OSName != original.osName || vm.AgentVersion != original.agentVersion {
								t.Error("safety retention lost independent filesystems or changed last-known identity")
							}
						}
						healthy.Store(true)
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
						resumed := build()
						m.cleanupGuestMetadataCache(time.Now())
						wantPolls := int32(failures + 2)
						if osCalls.Load() != int32(failures) || networkCalls.Load() != wantPolls || versionCalls.Load() != wantPolls || fsCalls.Load() != wantPolls {
							t.Errorf("command counts os/network/version/fs=%d/%d/%d/%d, want %d/%d/%d/%d", osCalls.Load(), networkCalls.Load(), versionCalls.Load(), fsCalls.Load(), failures, wantPolls, wantPolls, wantPolls)
						}
						if resumed.AgentVersion != "2.0" || len(resumed.NetworkInterfaces) != 1 || resumed.NetworkInterfaces[0].Name != "eth0" || !m.hasRecentGuestMetadataEvidence(instance, "node", 105, time.Now()) || !m.guestMetadataCache[key].osInfoSkip {
							t.Error("ordinary network/version recovery did not preserve unsupported OS safety")
						}
					})
				}
			}
		}
	}
}
