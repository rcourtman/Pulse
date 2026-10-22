package monitoring

import (
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

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Learn a command failure through real HTTP clients and ordinary pollers, then
// move its request coordinate. No guest, backup or uncertainty fence is changed.
// Only the test-owned metadata observation/scheduling clock is advanced.
func TestGuestMetadataMigrationKeepsSuppression(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, mode := range []string{"unsupported", "repeated-failure"} {
				t.Run(kind+"/"+collector+"/"+mode, func(t *testing.T) {
					const instance = "site|branch"
					var failing, changed atomic.Bool
					var osCalls, networkCalls, versionCalls, fsCalls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						switch {
						case strings.HasSuffix(r.URL.Path, "/nodes"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.HasSuffix(r.URL.Path, "/config"):
							fmt.Fprint(w, `{"data":{}}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":8388608,"meminfo":{"total":8388608,"available":5242880}}}`)
						case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
							fsCalls.Add(1)
							fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ufs","total-bytes":1048576000,"used-bytes":314572800}]}}`)
						case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
							networkCalls.Add(1)
							address := "192.0.2.20"
							if changed.Load() {
								address = "192.0.2.21"
							}
							fmt.Fprintf(w, `{"data":{"result":[{"name":"vio0","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":%q,"ip-address-type":"ipv4","prefix":24}]}]}}`, address)
						case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
							osCalls.Add(1)
							if !failing.Load() {
								fmt.Fprint(w, `{"data":{"result":{"name":"OpenBSD","version":"7.8"}}}`)
							} else if mode == "unsupported" {
								w.WriteHeader(http.StatusInternalServerError)
								fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-get-osinfo' failed - guest agent command failed: Failed to open file '/etc/os-release': No such file or directory"}`)
							} else {
								w.WriteHeader(http.StatusBadRequest)
								fmt.Fprint(w, `{"data":null,"message":"OS info request failed"}`)
							}
						case strings.HasSuffix(r.URL.Path, "/info"):
							versionCalls.Add(1)
							version := "2.0"
							if changed.Load() {
								version = "2.1"
							}
							fmt.Fprintf(w, `{"data":{"result":{"version":%q}}}`, version)
						default:
							t.Errorf("unexpected migration-fixture request: %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					client := windowsBootstrapClient(t, server.URL, kind)
					m := windowsBootstrapMonitor(t)
					key := guestMetadataCacheKey(instance, "node-a", 105)
					makeDue := func() {
						entry := m.guestMetadataCache[key]
						entry.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
						m.guestMetadataCache[key] = entry
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					}
					projections := make(map[string]json.RawMessage)
					poll := func(node, phase string, round int) models.VM {
						vm := windowsBootstrapPoll(t, m, client, collector, instance, node, 105, round)
						if vm.Node != node || vm.OSName != "OpenBSD" || vm.OSVersion != "7.8" || vm.Disk.Used != 314572800 || vm.DiskStatusReason != "" || vm.Memory.Observation.State != "current" || vm.CPU != float64(round)/10 {
							t.Errorf("migration lost independent telemetry or guest identity at %s", phase)
						}
						front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
						wire, err := json.Marshal(front.Resources)
						if err != nil {
							t.Fatal(err)
						}
						projections[phase] = wire
						return vm
					}
					poll("node-a", "initial", 1)
					osOrigin := m.guestMetadataCache[key].osInfoObservedAt
					failing.Store(true)
					failures := 1
					if mode == "repeated-failure" {
						failures = guestAgentOSInfoFailureThreshold
					}
					for round := 0; round < failures; round++ {
						makeDue()
						poll("node-a", "suppressed", 1)
					}
					before := m.guestMetadataCache[key]
					if !before.osInfoSkip || before.osInfoFailureCount != guestAgentOSInfoFailureThreshold {
						t.Fatal("fixture did not learn OS-command suppression")
					}
					metadataCalls := networkCalls.Load() + versionCalls.Load() + osCalls.Load()
					moved := poll("node-b", "migrated", 2)
					if osCalls.Load() != int32(1+failures) || networkCalls.Load()+versionCalls.Load()+osCalls.Load() != metadataCalls || !reflect.DeepEqual(m.guestMetadataCache[key], before) || moved.AgentVersion != "2.0" {
						t.Error("migration reset suppression, metadata age or refresh scheduling")
					}
					makeDue()
					changed.Store(true)
					refreshed := poll("node-b", "refreshed", 3)
					m.cleanupGuestMetadataCache(time.Now())
					entry := m.guestMetadataCache[key]
					if osCalls.Load() != int32(1+failures) || !entry.osInfoSkip || entry.osInfoObservedAt != osOrigin || refreshed.AgentVersion != "2.1" || !reflect.DeepEqual(refreshed.IPAddresses, []string{"192.0.2.21"}) {
						t.Error("eligible refresh reissued suppressed OS work or lost independent network/version recovery")
					}
					metadataCalls = networkCalls.Load() + versionCalls.Load() + osCalls.Load()
					poll("node-a", "returned", 4)
					if networkCalls.Load()+versionCalls.Load()+osCalls.Load() != metadataCalls {
						t.Error("return migration bypassed the shared refresh schedule")
					}
					// An explicit guest lifecycle invalidation still clears suppression.
					// Change the synthetic capability first; no known-broken native
					// command is retried and no uncertainty fence is shortened.
					m.clearGuestMetadataCache(instance, "node-b", 105)
					if _, exists := m.guestMetadataCache[key]; exists {
						t.Error("migration prevented explicit lifecycle invalidation")
					}
					failing.Store(false)
					poll("node-a", "new-lifecycle", 5)
					if osCalls.Load() != int32(2+failures) || m.guestMetadataCache[key].osInfoSkip || len(m.guestMetadataCache) != 1 {
						t.Error("new lifecycle did not recover or left node-local safety copies")
					}
					t.Logf("OS/network/version/filesystem commands=%d/%d/%d/%d; migration adds no metadata commands; eligible refresh keeps OS suppression", osCalls.Load(), networkCalls.Load(), versionCalls.Load(), fsCalls.Load())
					if os.Getenv("PULSE_METADATA_MIGRATION_PROJECTION") == "1" && kind == "client" && collector == "cluster" && mode == "unsupported" {
						wire, err := json.Marshal(projections)
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("PULSE_METADATA_MIGRATION_PROJECTION=%s", wire)
					}
				})
			}
		}
	}
}
