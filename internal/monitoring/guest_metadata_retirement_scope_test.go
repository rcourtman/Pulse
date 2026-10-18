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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Connection labels are not hostname components. A delimiter in a label must
// not turn retirement into cancellation or loss of a different QGA safety state.
func TestGuestMetadataRetirementOwnershipContract(t *testing.T) {
	for _, instance := range []string{"site", "site|branch", "région | zone"} {
		t.Run("cached/"+instance, func(t *testing.T) {
			at := time.Now().Add(-2 * guestMetadataCacheTTL)
			entry := metadataObservationFixture(at)
			entry.osInfoSkip, entry.osInfoFailureCount = true, guestAgentOSInfoFailureThreshold
			deadline, access := time.Now().Add(time.Hour), time.Now()
			m := &Monitor{
				guestMetadataCache:      make(map[string]guestMetadataCacheEntry),
				guestMetadataLimiter:    make(map[string]time.Time),
				guestMetadataLastAccess: make(map[string]time.Time),
				guestMetadataInFlight:   make(map[string]*guestMetadataFetch),
			}
			contexts := make(map[string]context.Context)
			own := []string{guestMetadataCacheKey(instance, "node-a", 105), guestMetadataCacheKey(instance, "node-b", 106)}
			unrelated := []string{guestMetadataCacheKey(instance+"|other", "node-a", 105), guestMetadataCacheKey(instance+"|other|nested", "node-a", 105), guestMetadataCacheKey(instance+"-other", "node-a", 105), guestMetadataCacheKey(instance+"/other", "node-a", 105), guestMetadataCacheKey(strings.ToUpper(instance), "node-a", 105)}
			for _, key := range append(append([]string{}, own...), unrelated...) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				contexts[key] = ctx
				m.guestMetadataCache[key], m.guestMetadataLimiter[key], m.guestMetadataLastAccess[key] = entry, deadline, access
				m.guestMetadataInFlight[key] = &guestMetadataFetch{cancel: cancel}
			}
			m.retirePVEInstanceRuntime(instance)
			for _, key := range own {
				if _, ok := m.guestMetadataCache[key]; ok || !m.guestMetadataLimiter[key].IsZero() || !m.guestMetadataLastAccess[key].IsZero() {
					t.Errorf("retired connection retained metadata, deadline or activity: %s", key)
				}
				if fetch := m.guestMetadataInFlight[key]; fetch == nil || !fetch.invalidated || contexts[key].Err() == nil {
					t.Errorf("retired connection lost its outstanding owner or did not cancel it: %s", key)
				}
			}
			for _, key := range unrelated {
				if !reflect.DeepEqual(m.guestMetadataCache[key], entry) || m.guestMetadataLimiter[key] != deadline || m.guestMetadataLastAccess[key] != access {
					t.Errorf("retirement erased or renewed another connection's safety/evidence/backoff: %s", key)
				}
				if fetch := m.guestMetadataInFlight[key]; fetch == nil || fetch.invalidated || contexts[key].Err() != nil {
					t.Errorf("retirement invalidated another connection's outstanding read: %s", key)
				}
			}
		})
	}
	t.Run("diagnostic-origin", func(t *testing.T) {
		m := &Monitor{}
		at := time.Now()
		memory := models.Memory{Total: 8192, Used: 1024, Free: 7168, Usage: 12.5}
		for _, instance := range []string{"site", "site|other", "site|other|nested"} {
			m.recordNodeSnapshot(instance, "node-a", NodeMemorySnapshot{RetrievedAt: at, Memory: memory, MemorySource: "status"})
			m.recordGuestSnapshot(instance, "qemu", "node-a", 105, GuestMemorySnapshot{RetrievedAt: at, Memory: memory, MemorySource: "status-mem"})
		}
		before := m.GetDiagnosticSnapshots()
		m.retirePVEInstanceRuntime("site")
		got := m.GetDiagnosticSnapshots()
		if len(got.Nodes) != 2 || len(got.Guests) != 2 {
			t.Errorf("retirement removed another connection's diagnostic origin: nodes=%d guests=%d", len(got.Nodes), len(got.Guests))
		}
		for _, snapshot := range before.Nodes {
			if snapshot.Instance != "site" && !reflect.DeepEqual(m.nodeSnapshots[makeNodeSnapshotKey(snapshot.Instance, snapshot.Node)], snapshot) {
				t.Errorf("unrelated node observation changed: %s", snapshot.Instance)
			}
		}
		for _, snapshot := range before.Guests {
			if snapshot.Instance != "site" && !reflect.DeepEqual(m.guestSnapshots[makeGuestSnapshotKey(snapshot.Instance, snapshot.GuestType, snapshot.Node, snapshot.VMID)], snapshot) {
				t.Errorf("unrelated guest observation changed: %s", snapshot.Instance)
			}
		}
	})
	for _, kind := range []string{"client", "cluster-client"} {
		for _, phase := range []string{"network-get-interfaces", "get-osinfo", "info"} {
			t.Run("in-flight/"+kind+"/"+phase, func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				var commands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/nodes") {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					if strings.Contains(r.URL.Path, "/agent/") {
						commands.Add(1)
						if strings.HasSuffix(r.URL.Path, "/"+phase) {
							w.WriteHeader(http.StatusOK)
							w.(http.Flusher).Flush()
							close(entered)
							<-release
						}
					}
					metadataReservationPayload(w, r)
				}))
				defer server.Close()
				defer unblock()
				client := windowsBootstrapClient(t, server.URL, kind)
				key := guestMetadataCacheKey("site|other", "node", 105)
				m := &Monitor{guestMetadataSlots: make(chan struct{}, 1)}
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				done := make(chan bool, 1)
				go func() {
					_, _, name, _, agent, paused := m.fetchGuestAgentMetadata(context.Background(), client, "site|other", "node", "guest", 105, status, false)
					done <- !paused && name == "FixtureOS" && agent == "2.0"
				}()
				defer func() {
					unblock()
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("owned synthetic HTTP fetch did not finish")
					}
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("selected HTTP response body did not start")
				}
				m.retirePVEInstanceRuntime("site")
				m.guestMetadataLimiterMu.Lock()
				fetch := m.guestMetadataInFlight[key]
				invalidated := fetch == nil || fetch.invalidated
				m.guestMetadataLimiterMu.Unlock()
				if invalidated {
					t.Error("unrelated retirement invalidated the live HTTP owner")
				}
				before := commands.Load()
				m.fetchGuestAgentMetadata(context.Background(), client, "site|other", "node", "guest", 105, status, false)
				if commands.Load() != before {
					t.Error("overlapping refresh bypassed the outstanding owner")
				}
				unblock()
				select {
				case accepted := <-done:
					if !accepted || commands.Load() != 3 || len(m.guestMetadataSlots) != 0 {
						t.Error("unrelated retirement lost the completed metadata, queued work or leaked a slot")
					}
					done <- accepted // Leave the completed result for cleanup's join.
				case <-time.After(5 * time.Second):
					t.Fatal("unrelated fetch did not complete")
				}
				assertMetadataReservationReleased(t, m)
			})
		}
	}
}

// Exercise the actual HTTP clients and both ordinary guest collectors. The
// endpoint represents an already-suppressed unsafe OS command; none of these
// controls sends a native guest command, changes an ACL or clears uncertainty.
func TestGuestMetadataRetirementPollingKeepsSuppression(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, schedule := range []string{"backoff", "eligible-refresh"} {
				t.Run(kind+"/"+collector+"/"+schedule, func(t *testing.T) {
					f := &windowsBootstrapFixture{t: t, osName: bootstrapWindowsOS, linuxAvailable: 6 * bootstrapMiB}
					f.poll.Store(1)
					var osCalls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasSuffix(r.URL.Path, "/get-osinfo") {
							osCalls.Add(1)
							w.WriteHeader(http.StatusInternalServerError)
							fmt.Fprint(w, `{"data":null,"message":"unsupported command: guest-get-osinfo"}`)
							return
						}
						f.serve(w, r)
					}))
					defer server.Close()
					m := windowsBootstrapMonitor(t)
					client := windowsBootstrapClient(t, server.URL, kind)
					const instance = "site|other"
					key := guestMetadataCacheKey(instance, "node", 105)
					original := metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
					original.osName, original.osVersion = "Windows", "2025"
					original.windowsGuest, original.osInfoObservedAt = true, time.Now()
					original.osInfoSkip, original.osInfoFailureCount = true, guestAgentOSInfoFailureThreshold
					m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
					m.guestMetadataLimiter = map[string]time.Time{key: time.Now().Add(time.Hour)}
					first := windowsBootstrapPoll(t, m, client, collector, instance, "node", 105, 1)
					if first.OSName != original.osName || osCalls.Load() != 0 {
						t.Fatal("initial ordinary poll did not use the suppressed identity")
					}
					if schedule == "eligible-refresh" {
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					}
					deadline := m.guestMetadataLimiter[key]
					m.retirePVEInstanceRuntime("site")
					if m.guestMetadataLimiter[key] != deadline {
						t.Error("retirement changed a different connection's refresh schedule")
					}
					f.resetTrace()
					f.poll.Store(2)
					second := windowsBootstrapPoll(t, m, client, collector, instance, "node", 105, 2)
					if osCalls.Load() != 0 || second.OSName != original.osName || second.OSVersion != original.osVersion {
						t.Errorf("retirement reissued a suppressed OS command or lost display identity: calls=%d OS=%q/%q", osCalls.Load(), second.OSName, second.OSVersion)
					}
					entry := m.guestMetadataCache[key]
					if !entry.osInfoSkip || entry.osInfoFailureCount != original.osInfoFailureCount || entry.osInfoObservedAt != original.osInfoObservedAt {
						t.Error("retirement changed command suppression or renewed its original OS receipt")
					}
					if second.CPU != .2 || len(second.Disks) != 2 || second.Memory.Observation.Source != "status-mem" || second.Memory.Observation.State != "current" {
						t.Error("retaining suppression lost independent ordinary filesystem/CPU/memory readings")
					}
					want := []string{"get-fsinfo"}
					if schedule == "eligible-refresh" {
						want = append(want, "network-get-interfaces", "info")
					}
					if got := f.trace(); !reflect.DeepEqual(got, want) {
						t.Errorf("ordinary command trace=%v, want %v", got, want)
					}
					if output := os.Getenv("PULSE_GUEST_RETIREMENT_SURFACE_OUTPUT"); output != "" && kind == "client" && collector == "node" && schedule == "backoff" {
						body, err := json.MarshalIndent(struct{ Before, After models.VM }{first, second}, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(output, body, 0600); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
