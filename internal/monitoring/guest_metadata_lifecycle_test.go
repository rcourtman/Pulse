package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A deliberately uncooperative synthetic reader can finish after cancellation.
// Invalidation must exclude its late reply, not assume cancellation proves that
// a consumed guest command has stopped. No real guest or backup is exercised.
type metadataLifecycleClient struct {
	mockPVEClient
	mu       sync.Mutex
	commands []string
	stage    string
	entered  chan context.Context
	resume   chan struct{}
}

func TestGuestMetadataLifecycleCancellationKeepsHTTPFence(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, disposition := range []string{"invalidate", "retire"} {
			for _, phase := range []string{"network-get-interfaces", "get-osinfo", "info"} {
				t.Run(kind+"/"+disposition+"/"+phase, func(t *testing.T) {
					entered, resume := make(chan struct{}, 1), make(chan struct{})
					var unblock sync.Once
					var mu sync.Mutex
					var trace []string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasSuffix(r.URL.Path, "/nodes") {
							fmt.Fprint(w, `{"data":[]}`)
							return
						}
						if strings.Contains(r.URL.Path, "/qemu/105/agent/") {
							mu.Lock()
							trace = append(trace, r.URL.Path)
							mu.Unlock()
							if strings.HasSuffix(r.URL.Path, "/"+phase) {
								// Headers arrive, but the consumed command's body
								// remains incomplete even after its client cancels.
								w.WriteHeader(http.StatusOK)
								w.(http.Flusher).Flush()
								entered <- struct{}{}
								<-resume
								return
							}
						}
						metadataReservationPayload(w, r)
					}))
					defer server.Close()
					defer unblock.Do(func() { close(resume) })
					client := windowsBootstrapClient(t, server.URL, kind)
					const instance = "http-lifecycle"
					key := guestMetadataCacheKey(instance, "node", 105)
					m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))}, guestMetadataSlots: make(chan struct{}, 1)}
					status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
					finished := make(chan struct{})
					var oldName string
					var paused bool
					go func() {
						defer close(finished)
						_, _, oldName, _, _, paused = m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
					}()
					defer func() {
						unblock.Do(func() { close(resume) })
						select {
						case <-finished:
						case <-time.After(5 * time.Second):
							t.Error("cancelled HTTP fetch did not finish")
						}
					}()
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("synthetic HTTP body did not reach selected phase")
					}
					if disposition == "invalidate" {
						m.clearGuestMetadataCache(instance, "node", 105)
					} else {
						m.retirePVEInstanceRuntime(instance)
					}
					select {
					case <-finished:
					case <-time.After(5 * time.Second):
						t.Fatal("obsolete HTTP fetch waited for a consumed command instead of cancelling")
					}
					if !paused || oldName != "" || len(m.guestMetadataSlots) != 0 || !m.guestMetadataLimiter[key].IsZero() {
						t.Error("cancelled fetch published identity/backoff or leaked its slot")
					}
					if _, exists := m.guestMetadataCache[key]; exists {
						t.Error("cancelled fetch resurrected cache state")
					}
					assertMetadataReservationReleased(t, m)
					mu.Lock()
					before := append([]string(nil), trace...)
					mu.Unlock()
					// A new diagnostic client and the new metadata lifecycle must
					// honour the original transport's uncertain-completion fence.
					diagnostic := windowsBootstrapClient(t, server.URL, "client")
					if _, err := diagnostic.GetVMAgentInfo(context.Background(), "node", 105); proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" {
						t.Errorf("cancellation discarded shared uncertainty: %v", err)
					}
					_, _, _, _, _, stillPaused := m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
					if !stillPaused {
						t.Error("new lifecycle bypassed uncertain command completion")
					}
					mu.Lock()
					unchanged := reflect.DeepEqual(trace, before)
					mu.Unlock()
					wantCommands := 1
					if phase == "get-osinfo" {
						wantCommands = 2
					} else if phase == "info" {
						wantCommands = 3
					}
					if !unchanged || len(before) != wantCommands {
						t.Error("obsolete/cooldown fetch sent an additional or repeated command")
					}
					// Independent VM discovery remains useful, without shortening
					// or replaying the cancelled VM's uncertainty fence.
					_, _, name, _, agent, otherPaused := m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "other", 106, status, false)
					if otherPaused || name != "FixtureOS" || agent != "2.0" {
						t.Error("cancelled lifecycle blocked an independent VM")
					}
					unblock.Do(func() { close(resume) })
				})
			}
		}
	}
}

func TestGuestMetadataLifecycleCancelsSlotWait(t *testing.T) {
	key := guestMetadataCacheKey("slot-lifecycle", "node", 105)
	m := &Monitor{guestMetadataSlots: make(chan struct{}, 1)}
	m.guestMetadataSlots <- struct{}{} // A different fetch owns this slot.
	client := &metadataSuppressionClient{os: map[string]interface{}{"name": "CurrentOS"}}
	status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
	finished := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _, _, _, _, paused := m.fetchGuestAgentMetadata(ctx, client, "slot-lifecycle", "node", "guest", 105, status, false)
		finished <- paused
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.guestMetadataLimiterMu.Lock()
		fetch := m.guestMetadataInFlight[key]
		waiting := fetch != nil && fetch.cancel != nil
		m.guestMetadataLimiterMu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("metadata fetch did not reserve its slot-wait lifecycle")
		}
		time.Sleep(time.Millisecond)
	}
	m.clearGuestMetadataCache("slot-lifecycle", "node", 105)
	select {
	case paused := <-finished:
		if !paused {
			t.Error("invalidated slot waiter did not defer")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("invalidation did not cancel slot waiting")
	}
	if len(m.guestMetadataSlots) != 1 || client.networkCalls+client.osCalls+client.versionCalls != 0 || len(m.guestMetadataCache) != 0 || !m.guestMetadataLimiter[key].IsZero() {
		t.Error("invalidated slot waiter consumed another owner's slot, sent a command or published state")
	}
	assertMetadataReservationReleased(t, m)
	<-m.guestMetadataSlots
	_, _, name, _, _, paused := m.fetchGuestAgentMetadata(context.Background(), client, "slot-lifecycle", "node", "guest", 105, status, false)
	if paused || name != "CurrentOS" || len(m.guestMetadataSlots) != 0 {
		t.Error("ordinary discovery did not resume after slot-wait invalidation")
	}
}

func (c *metadataLifecycleClient) command(ctx context.Context, name string) {
	c.mu.Lock()
	c.commands = append(c.commands, name)
	c.mu.Unlock()
	if name == c.stage {
		c.entered <- ctx
		<-c.resume // Intentionally ignore cancellation and deliver a late reply.
	}
}

func (c *metadataLifecycleClient) trace() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.commands...)
}

func (c *metadataLifecycleClient) GetVMNetworkInterfaces(ctx context.Context, _ string, _ int) ([]proxmox.VMNetworkInterface, error) {
	c.command(ctx, "network")
	return []proxmox.VMNetworkInterface{{Name: "eth0", HardwareAddr: "02:00:00:00:00:01", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.20"}}}}, nil
}

func (c *metadataLifecycleClient) GetVMAgentInfo(ctx context.Context, _ string, _ int) (map[string]interface{}, error) {
	c.command(ctx, "os")
	if c.stage == "version-deferred" {
		return nil, errors.New("Failed to open file '/etc/os-release': No such file or directory")
	}
	return map[string]interface{}{"name": "OldOS", "version": "1"}, nil
}

func (c *metadataLifecycleClient) GetVMAgentVersion(ctx context.Context, _ string, _ int) (string, error) {
	stage := "version"
	if c.stage == "version-deferred" {
		stage = c.stage
	}
	c.command(ctx, stage)
	if stage == "version-deferred" {
		return "", proxmox.ErrGuestAgentDeferred
	}
	return "old-agent", nil
}

func TestGuestMetadataLifecycleRejectsLateReplies(t *testing.T) {
	for _, disposition := range []string{"invalidate", "retire"} {
		for _, seeded := range []bool{false, true} {
			for _, stage := range []string{"network", "os", "version", "version-deferred"} {
				t.Run(fmt.Sprintf("%s/cached-%t/%s", disposition, seeded, stage), func(t *testing.T) {
					const instance = "lifecycle"
					key := guestMetadataCacheKey(instance, "node", 105)
					sibling := guestMetadataCacheKey("lifecycle-elsewhere", "node", 105)
					originalSibling := metadataObservationFixture(time.Now())
					future := time.Now().Add(time.Hour)
					m := &Monitor{
						guestMetadataCache:      map[string]guestMetadataCacheEntry{sibling: originalSibling},
						guestMetadataLimiter:    map[string]time.Time{sibling: future},
						guestMetadataLastAccess: map[string]time.Time{sibling: future},
						guestMetadataSlots:      make(chan struct{}, 1),
					}
					if seeded {
						m.guestMetadataCache[key] = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
					}
					client := &metadataLifecycleClient{stage: stage, entered: make(chan context.Context, 1), resume: make(chan struct{})}
					var unblock sync.Once
					defer unblock.Do(func() { close(client.resume) })
					status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
					finished := make(chan struct{})
					var ips, names []string
					var deferred bool
					go func() {
						defer close(finished)
						addresses, interfaces, os, version, agent, paused := m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
						ips, names, deferred = addresses, []string{os, version, agent}, paused
						if len(interfaces) != 0 {
							names = append(names, "interface")
						}
					}()
					defer func() {
						unblock.Do(func() { close(client.resume) })
						select {
						case <-finished:
						case <-time.After(5 * time.Second):
							t.Error("late metadata reader did not finish")
						}
					}()
					var commandCtx context.Context
					select {
					case commandCtx = <-client.entered:
					case <-time.After(5 * time.Second):
						t.Fatal("synthetic reader did not reach selected phase")
					}
					if disposition == "invalidate" {
						m.clearGuestMetadataCache(instance, "node", 105)
					} else {
						m.retirePVEInstanceRuntime(instance)
					}
					select {
					case <-commandCtx.Done():
					default:
						t.Error("invalidation did not cancel the obsolete fetch context")
					}
					if m.tryReserveGuestMetadataFetch(key, time.Now()) || len(m.guestMetadataSlots) != 1 {
						t.Error("invalidation released ownership before the consumed reader returned")
					}
					m.cleanupGuestMetadataCache(time.Now())
					unblock.Do(func() { close(client.resume) })
					select {
					case <-finished:
					case <-time.After(5 * time.Second):
						t.Fatal("obsolete reader did not finish")
					}
					if !deferred || len(ips) != 0 || !reflect.DeepEqual(names, []string{"", "", ""}) {
						t.Error("obsolete fetch returned usable old-lifecycle identity")
					}
					want := []string{"network"}
					if stage != "network" {
						want = append(want, "os")
					}
					if stage == "version" || stage == "version-deferred" {
						want = append(want, stage)
					}
					if got := client.trace(); !reflect.DeepEqual(got, want) {
						t.Errorf("obsolete fetch queued later commands: got %v, want %v", got, want)
					}
					if _, exists := m.guestMetadataCache[key]; exists {
						t.Error("late reply resurrected invalidated metadata or OS-command state")
					}
					if !m.guestMetadataLimiter[key].IsZero() || !m.guestMetadataLastAccess[key].IsZero() || len(m.guestMetadataSlots) != 0 {
						t.Error("obsolete fetch recreated retry/activity state or leaked its slot")
					}
					assertMetadataReservationReleased(t, m)
					if !reflect.DeepEqual(m.guestMetadataCache[sibling], originalSibling) || !m.guestMetadataLimiter[sibling].Equal(future) || !m.guestMetadataLastAccess[sibling].Equal(future) {
						t.Error("lifecycle invalidation crossed instance boundaries")
					}
					// Re-adding a retired instance or restoring an available agent is
					// a new lifecycle; no old suppression or backoff can obstruct it.
					fresh := &metadataSuppressionClient{os: map[string]interface{}{"name": "CurrentOS", "version": "2"}}
					_, _, name, _, agent, paused := m.fetchGuestAgentMetadata(context.Background(), fresh, instance, "node", "guest", 105, status, false)
					if paused || name != "CurrentOS" || agent != "2.0" || fresh.osCalls != 1 || !m.hasRecentGuestMetadataEvidence(instance, "node", 105, time.Now()) {
						t.Error("ordinary new-lifecycle discovery did not resume independently")
					}
					assertMetadataReservationReleased(t, m)
				})
			}
		}
	}
}
