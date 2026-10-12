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

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A complete access/agent-unavailable rejection did not exercise guest-get-osinfo.
// It must not be counted as evidence that this particular command is unsafe.
func testGuestMetadataRejectedOSInfo(t *testing.T) {
	for _, rejection := range []string{"unauthorized", "forbidden", "agent-not-running"} {
		for _, seeded := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached-%t", rejection, seeded), func(t *testing.T) {
				var recovered atomic.Bool
				var osCalls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
						t.Error("fixture authentication changed")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "/config"):
						fmt.Fprint(w, `{"data":{}}`)
					case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
						osCalls.Add(1)
						if recovered.Load() {
							fmt.Fprintf(w, `{"data":{"name":%q,"version":"2025"}}`, bootstrapWindowsOS)
							return
						}
						switch rejection {
						case "unauthorized":
							w.WriteHeader(http.StatusUnauthorized)
							// Even OS-like body text is not a completed OS-info reply.
							fmt.Fprint(w, "unsupported command: guest-get-osinfo")
						case "forbidden":
							w.WriteHeader(http.StatusForbidden)
							fmt.Fprint(w, "Failed to open file '/etc/os-release': No such file or directory")
						default:
							w.WriteHeader(http.StatusInternalServerError)
							fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-get-osinfo' failed - QEMU guest agent is not running"}`)
						}
					case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
						fmt.Fprint(w, `{"data":{"result":[]}}`)
					case strings.HasSuffix(r.URL.Path, "/info"):
						fmt.Fprint(w, `{"data":{"result":{}}}`)
					default:
						t.Errorf("unexpected request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: windowsPollingHangGuard})
				if err != nil {
					t.Fatal(err)
				}
				key := guestMetadataCacheKey("rejection", "node", 105)
				original := guestMetadataCacheEntry{}
				if seeded {
					original = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
					original.osInfoFailureCount = guestAgentOSInfoFailureThreshold - 1
				}
				m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: original}, guestMetadataLimiter: make(map[string]time.Time)}
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				for poll := 1; poll <= guestAgentOSInfoFailureThreshold; poll++ {
					// Represent separate eligible refreshes without waiting for the
					// production backoff. Never clear a QGA command fence or lock.
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					start := time.Now()
					ips, ifaces, name, version, agent, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "rejection", "node", "guest", 105, status, false)
					if deferred || !reflect.DeepEqual(m.guestMetadataCache[key], original) || !reflect.DeepEqual(ips, original.ipAddresses) || !reflect.DeepEqual(ifaces, original.networkInterfaces) || name != original.osName || version != original.osVersion || agent != original.agentVersion {
						t.Errorf("rejected poll %d poisoned OS state or renewed identity: %+v, deferred=%t", poll, m.guestMetadataCache[key], deferred)
					}
					if m.hasRecentGuestMetadataEvidence("rejection", "node", 105, time.Now()) || !m.guestMetadataLimiter[key].After(start) {
						t.Error("rejection became fresh metadata or removed normal backoff")
					}
					before := osCalls.Load()
					m.fetchGuestAgentMetadata(context.Background(), client, "rejection", "node", "guest", 105, status, false)
					if osCalls.Load() != before {
						t.Error("rejection bypassed normal refresh backoff")
					}
				}
				recovered.Store(true)
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				before := time.Now()
				_, _, name, version, _, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "rejection", "node", "guest", 105, status, false)
				entry := m.guestMetadataCache[key]
				if deferred || name != bootstrapWindowsOS || version != "2025" || !entry.windowsGuest || entry.osInfoObservedAt.Before(before) || entry.osInfoFailureCount != 0 || entry.osInfoSkip || osCalls.Load() != guestAgentOSInfoFailureThreshold+1 {
					t.Errorf("ordinary accepted OS reply did not recover: name=%q version=%q calls=%d entry=%+v", name, version, osCalls.Load(), entry)
				}
			})
		}
	}
}

// A stopped QGA can coexist with status/current's configured agent flag. Exercise
// actual HTTP errors, both ordinary collectors and both client paths. No native
// restart, freeze, extra probe, shortened uncertainty fence or ACL change.
func testGuestMetadataStoppedAgentPollingRecovery(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				f := &windowsBootstrapFixture{t: t, osName: bootstrapWindowsOS, linuxAvailable: 6 * bootstrapMiB}
				f.poll.Store(1)
				var available atomic.Bool
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !available.Load() && strings.Contains(r.URL.Path, "/agent/") {
						if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
							t.Error("fixture authentication changed")
							w.WriteHeader(http.StatusForbidden)
							return
						}
						command := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
						f.mu.Lock()
						f.commands = append(f.commands, command)
						f.mu.Unlock()
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprintf(w, `{"data":null,"message":"VM 105 qmp command 'guest-%s' failed - QEMU guest agent is not running"}`, command)
						return
					}
					f.serve(w, r)
				}))
				defer s.Close()
				m := windowsBootstrapMonitor(t)
				m.guestMetadataLimiter = make(map[string]time.Time)
				client := windowsBootstrapClient(t, s.URL, kind)
				key := guestMetadataCacheKey("bootstrap", "node", 105)
				for poll := 0; poll < guestAgentOSInfoFailureThreshold; poll++ {
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					vm := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
					entry := m.guestMetadataCache[key]
					if entry.osInfoFailureCount != 0 || entry.osInfoSkip || !entry.fetchedAt.IsZero() || vm.OSName != "" || vm.Disk.Usage >= 0 || vm.Memory.Observation.Source != "status-mem" || vm.Memory.Observation.State != "current" || vm.CPU != .1 {
						t.Errorf("stopped-agent poll %d became an OS command defect or lost independent counters: %+v, OS=%q disk=%v memory=%+v CPU=%v", poll, entry, vm.OSName, vm.Disk.Usage, vm.Memory.Observation, vm.CPU)
					}
				}
				id := makeGuestID("bootstrap", "node", 105)
				if len(guestHistoryStoredPoints(t, m, "vm", id, "disk")) != 0 {
					t.Error("stopped agent manufactured filesystem History")
				}
				available.Store(true)
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				f.resetTrace()
				vm := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
				checkWindowsBootstrapReading(t, vm, 1)
				if got, want := f.trace(), []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info"}; !reflect.DeepEqual(got, want) {
					t.Errorf("ordinary recovery did not learn Windows before optional Linux reads: %v, want %v", got, want)
				}
				checkBootstrapHistory(t, m, id, "disk", []float64{25})
			})
		}
	}
}
