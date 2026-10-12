package monitoring

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
