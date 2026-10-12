package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Rate refusal is not a completed OS-command failure. Exercise the real HTTP
// clients and both ordinary builders without a guest, database, backup or restart.
func TestGuestMetadataRateLimitRecoveryContract(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, seeded := range []bool{false, true} {
				for _, tc := range []struct {
					name        string
					status      int
					body        string
					rateRefusal bool
				}{
					{"rate-limit", http.StatusTooManyRequests, "Too many requests", true},
					{"rate-limit-os-like-body", http.StatusTooManyRequests, "unsupported command: guest-get-osinfo", true},
					{"quoted-rate-is-not-wire-rate", http.StatusBadRequest, "API error 429: Too many requests", false},
				} {
					t.Run(fmt.Sprintf("%s/%s/cached-%t/%s", kind, collector, seeded, tc.name), func(t *testing.T) {
						const instance = "rate-recovery"
						var recovered atomic.Bool
						var osCalls, linuxCalls atomic.Int32
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" || r.Method != http.MethodGet {
								t.Error("fixture authentication or read-only method changed")
								w.WriteHeader(http.StatusForbidden)
								return
							}
							w.Header().Set("Content-Type", "application/json")
							switch {
							case strings.HasSuffix(r.URL.Path, "/nodes"):
								fmt.Fprint(w, `{"data":[]}`)
							case strings.HasSuffix(r.URL.Path, "/config"):
								fmt.Fprint(w, `{"data":{}}`)
							case strings.HasSuffix(r.URL.Path, "/status/current"):
								// Independent pressure evidence avoids an unnecessary optional Linux
								// read while the OS endpoint is temporarily refusing requests.
								if recovered.Load() && tc.rateRefusal {
									fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":3145728}}`)
								} else {
									fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":3145728,"meminfo":{"total":8388608,"available":5242880}}}`)
								}
							case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
								fmt.Fprint(w, `{"data":{"result":[{"name":"C:","mountpoint":"C:\\","type":"ntfs","total-bytes":1572864000,"used-bytes":419430400},{"name":"D:","mountpoint":"D:\\","type":"ntfs","total-bytes":524288000,"used-bytes":104857600}]}}`)
							case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
								fmt.Fprint(w, `{"data":{"result":[]}}`)
							case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
								osCalls.Add(1)
								if recovered.Load() {
									fmt.Fprintf(w, `{"data":{"result":{"name":%q,"version":"2025"}}}`, bootstrapWindowsOS)
								} else {
									w.WriteHeader(tc.status)
									fmt.Fprint(w, tc.body)
								}
							case strings.HasSuffix(r.URL.Path, "/info"):
								fmt.Fprint(w, `{"data":{"result":{}}}`)
							case strings.HasSuffix(r.URL.Path, "/file-read"):
								linuxCalls.Add(1)
								w.WriteHeader(http.StatusInternalServerError)
								fmt.Fprint(w, "unsupported command: guest-file-read")
							default:
								t.Errorf("unexpected request: %s", r.URL.Path)
								http.NotFound(w, r)
							}
						}))
						defer server.Close()
						client := windowsBootstrapClient(t, server.URL, kind)
						key := guestMetadataCacheKey(instance, "node", 105)
						original := guestMetadataCacheEntry{}
						if seeded {
							original = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
							original.osInfoFailureCount = guestAgentOSInfoFailureThreshold - 1
						}
						m := &Monitor{
							config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(),
							guestMetadataCache:   map[string]guestMetadataCacheEntry{key: original},
							guestMetadataLimiter: make(map[string]time.Time), guestAgentRetries: 2,
						}
						m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
						t.Cleanup(m.alertManager.Stop)
						m.guestAgentNetworkTimeout, m.guestAgentOSInfoTimeout, m.guestAgentVersionTimeout = windowsPollingHangGuard, windowsPollingHangGuard, windowsPollingHangGuard
						id := makeGuestID(instance, "node", 105)
						var previous models.VM
						poll := func() models.VM {
							var vm models.VM
							if collector == "node" {
								vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), instance, "node", []proxmox.VM{{VMID: 105, Name: "rate-limited-guest", Status: "running", MaxMem: 8 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB, CPU: .1}}, client, map[string]models.VM{id: previous}, nil)
								if len(vms) != 1 {
									t.Fatal("ordinary node poll lost the guest")
								}
								vm = vms[0]
							} else {
								var ok bool
								vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), instance, proxmox.ClusterResource{VMID: 105, Node: "node", Type: "qemu", Name: "rate-limited-guest", Status: "running", MaxMem: 8 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB, CPU: .1}, client, id, nil, &previous)
								if !ok {
									t.Fatal("ordinary cluster builder lost the guest")
								}
							}
							if vm.Disk.Used != int64(500*bootstrapMiB) || len(vm.Disks) != 2 || vm.DiskStatusReason != "" || vm.Memory.Used != int64(3*bootstrapMiB) || vm.CPU != .1 {
								t.Errorf("independent accepted readings were lost: disk=%+v rows=%d memory=%+v CPU=%v", vm.Disk, len(vm.Disks), vm.Memory, vm.CPU)
							}
							previous = vm
							return vm
						}
						rounds := guestAgentOSInfoFailureThreshold
						if !tc.rateRefusal && seeded {
							rounds = 1
						}
						var before models.VM
						for round := 0; round < rounds; round++ {
							// Advance only test-owned metadata scheduling. No lock, command-safety
							// suppression, uncertainty fence or guest command is cleared/replayed.
							m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
							before = poll()
							if tc.rateRefusal && !reflect.DeepEqual(m.guestMetadataCache[key], original) {
								t.Errorf("HTTP429 poisoned OS suppression or renewed identity: %+v, want %+v", m.guestMetadataCache[key], original)
							}
							if m.hasRecentGuestMetadataEvidence(instance, "node", 105, time.Now()) {
								t.Error("refusal became fresh metadata")
							}
							count := osCalls.Load()
							poll()
							if osCalls.Load() != count {
								t.Error("refusal bypassed ordinary refresh backoff")
							}
						}
						recovered.Store(true)
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
						after := poll()
						entry := m.guestMetadataCache[key]
						if tc.rateRefusal {
							if after.OSName != bootstrapWindowsOS || after.OSVersion != "2025" || entry.osInfoSkip || entry.osInfoFailureCount != 0 || !entry.windowsGuest || entry.osInfoObservedAt.IsZero() || osCalls.Load() != int32(rounds+1) || linuxCalls.Load() != 0 {
								t.Errorf("ordinary recovery stayed suppressed or sent Linux memory read: OS=%q version=%q calls=%d Linux=%d entry=%+v", after.OSName, after.OSVersion, osCalls.Load(), linuxCalls.Load(), entry)
							}
						} else if !entry.osInfoSkip || osCalls.Load() != int32(rounds) || after.OSName != original.osName {
							t.Error("unrelated genuine repeated failure suppression was weakened by a quoted 429")
						}
						// The optional, safe source projection is byte-for-byte producer JSON,
						// not a hand-authored browser replacement. No database/native acceptance.
						if os.Getenv("PULSE_TEST_RATE_LIMIT_PROJECTION") == "1" && kind == "client" && collector == "cluster" && !seeded && tc.name == "rate-limit" {
							data, err := json.Marshal(map[string]models.VM{"before": before, "after": after})
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("RATE_LIMIT_PROJECTION %s", data)
						}
					})
				}
			}
		}
	}
}
