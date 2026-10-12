package monitoring

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Deliberately ignore cancellation: a consumed command can finish after its
// caller's lifecycle ends. Its reply is not a sample for the replacement.
type blockedGuestMemoryClient struct {
	*stubPVEClient
	entered chan context.Context
	release chan struct{}
	info    proxmox.LinuxMemoryAvailability
	err     error
	calls   atomic.Int32
}

func (c *blockedGuestMemoryClient) GetVMMemoryAvailabilityFromAgent(ctx context.Context, _ string, _ int) (proxmox.LinuxMemoryAvailability, error) {
	c.calls.Add(1)
	c.entered <- ctx
	<-c.release
	return c.info, c.err
}

type guestMemoryReply struct {
	info proxmox.LinuxMemoryAvailability
	err  error
}

func memoryLifecycleSample(available uint64) proxmox.LinuxMemoryAvailability {
	return proxmox.LinuxMemoryAvailability{Total: 8192, Available: available, EffectiveAvailable: available, Free: 1024, Source: "meminfo-available"}
}

func startBlockedGuestMemory(t *testing.T, m *Monitor, instance string, info proxmox.LinuxMemoryAvailability, err error) (*blockedGuestMemoryClient, context.Context, <-chan guestMemoryReply, func()) {
	t.Helper()
	c := &blockedGuestMemoryClient{stubPVEClient: &stubPVEClient{}, entered: make(chan context.Context, 1), release: make(chan struct{}), info: info, err: err}
	done := make(chan guestMemoryReply, 1)
	go func() {
		got, readErr := m.getVMAgentMemoryAvailability(context.Background(), c, instance, "node", 105)
		done <- guestMemoryReply{got, readErr}
	}()
	var once sync.Once
	unblock := func() { once.Do(func() { close(c.release) }) }
	t.Cleanup(unblock)
	select {
	case ctx := <-c.entered:
		return c, ctx, done, unblock
	case <-time.After(5 * time.Second):
		t.Fatal("synthetic memory read did not start")
		return nil, nil, nil, unblock
	}
}

func readGuestMemoryReply(t *testing.T, done <-chan guestMemoryReply) guestMemoryReply {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("synthetic memory read did not return")
		return guestMemoryReply{}
	}
}

func invalidateMemoryLifecycle(m *Monitor, instance, disposition string) {
	switch disposition {
	case "guest":
		m.clearGuestMetadataCache(instance, "node", 105)
	case "connection":
		m.retirePVEInstanceRuntime(instance)
	case "unavailable":
		// A fresh display identity may remain, but cannot retain memory samples.
		m.fetchGuestAgentMetadata(context.Background(), nil, instance, "node", "guest", 105, &proxmox.VMStatus{}, false)
	case "stopped":
		m.buildVMFromClusterResource(context.Background(), instance, proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 105, Status: "stopped", MaxMem: 8192}, nil, "", nil, nil)
	}
}

func TestGuestMemoryLifecycleRejectsLateReplies(t *testing.T) {
	for _, disposition := range []string{"guest", "connection", "unavailable", "stopped"} {
		for _, reply := range []struct {
			name string
			info proxmox.LinuxMemoryAvailability
			err  error
		}{
			{"success", memoryLifecycleSample(4096), nil},
			{"failure", proxmox.LinuxMemoryAvailability{}, errors.New("synthetic failure")},
			{"empty", proxmox.LinuxMemoryAvailability{}, nil},
		} {
			t.Run(disposition+"/"+reply.name, func(t *testing.T) {
				const instance = "memory-lifecycle"
				m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey(instance, "node", 105): metadataObservationFixture(time.Now())}}
				key := guestMemoryCacheKey(instance, "node", 105)
				old, requestCtx, done, unblock := startBlockedGuestMemory(t, m, instance, reply.info, reply.err)
				invalidateMemoryLifecycle(m, instance, disposition)
				select {
				case <-requestCtx.Done():
				default:
					t.Error("invalidation did not cancel the obsolete memory request")
				}
				freshInfo := memoryLifecycleSample(6144)
				fresh := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &freshInfo}
				if got, err := m.getVMAgentMemoryAvailability(context.Background(), fresh, instance, "node", 105); !errors.Is(err, proxmox.ErrGuestAgentDeferred) || got != (proxmox.LinuxMemoryAvailability{}) || fresh.memCalls != 0 {
					t.Error("new lifecycle bypassed the still-outstanding memory owner")
				}
				// A different VM can still get its own sample while 105 is owned.
				if got, err := m.getVMAgentMemoryAvailability(context.Background(), fresh, instance, "node", 106); err != nil || got != freshInfo || fresh.memCalls != 1 {
					t.Fatal("obsolete read blocked independent guest memory")
				}
				unblock()
				got := readGuestMemoryReply(t, done)
				if !errors.Is(got.err, proxmox.ErrGuestAgentDeferred) || got.info != (proxmox.LinuxMemoryAvailability{}) {
					t.Error("obsolete reply was returned as a new guest sample or failure")
				}
				m.rrdCacheMu.RLock()
				_, resurrected := m.vmAgentMemCache[key]
				m.rrdCacheMu.RUnlock()
				if resurrected {
					t.Error("obsolete read recreated successful/negative memory cache state")
				}
				if got, err := m.getVMAgentMemoryAvailability(context.Background(), fresh, instance, "node", 105); err != nil || got != freshInfo || fresh.memCalls != 2 || old.calls.Load() != 1 {
					t.Errorf("normal new-lifecycle memory did not recover: sample=%+v error=%v calls=%d/%d", got, err, fresh.memCalls, old.calls.Load())
				}
			})
		}
	}
}

func TestGuestMemoryLifecycleClearsSuccessfulAndNegativeCaches(t *testing.T) {
	for _, disposition := range []string{"guest", "connection", "unavailable", "stopped"} {
		for _, negative := range []bool{false, true} {
			name := "success"
			if negative {
				name = "failure"
			}
			t.Run(disposition+"/"+name, func(t *testing.T) {
				const instance = "cached-lifecycle"
				key := guestMemoryCacheKey(instance, "node", 105)
				metadata := metadataObservationFixture(time.Now())
				entry := agentMemCacheEntry{negative: negative, info: memoryLifecycleSample(4096), fetchedAt: time.Now()}
				m := &Monitor{vmAgentMemCache: map[string]agentMemCacheEntry{key: entry}, guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey(instance, "node", 105): metadata}}
				invalidateMemoryLifecycle(m, instance, disposition)
				if _, exists := m.vmAgentMemCache[key]; exists {
					t.Error("new lifecycle retained old memory success/backoff")
				}
				if disposition == "unavailable" || disposition == "stopped" {
					if got := m.guestMetadataCache[guestMetadataCacheKey(instance, "node", 105)]; got.osName != metadata.osName || !got.fetchedAt.Equal(metadata.fetchedAt) {
						t.Error("memory invalidation erased useful display/OS command-safety state")
					}
				}
				nextInfo := memoryLifecycleSample(6144)
				client := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &nextInfo}
				if got, err := m.getVMAgentMemoryAvailability(context.Background(), client, instance, "node", 105); err != nil || got != nextInfo || client.memCalls != 1 {
					t.Errorf("next available lifecycle reused obsolete evidence: %+v / %v / calls=%d", got, err, client.memCalls)
				}
			})
		}
	}
}

func TestGuestMemoryLifecycleOwnsOneReadUntilCompletion(t *testing.T) {
	m := &Monitor{}
	info := memoryLifecycleSample(4096)
	old, _, done, unblock := startBlockedGuestMemory(t, m, "coalesced", info, nil)
	next := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &info}
	if got, err := m.getVMAgentMemoryAvailability(context.Background(), next, "coalesced", "node", 105); !errors.Is(err, proxmox.ErrGuestAgentDeferred) || got != (proxmox.LinuxMemoryAvailability{}) || next.memCalls != 0 {
		t.Error("overlapping refresh dispatched another memory read")
	}
	unblock()
	if got := readGuestMemoryReply(t, done); got.err != nil || got.info != info {
		t.Fatal("ordinary owner lost its successful sample")
	}
	if got, err := m.getVMAgentMemoryAvailability(context.Background(), next, "coalesced", "node", 105); err != nil || got != info || next.memCalls != 0 || old.calls.Load() != 1 {
		t.Error("ordinary successful cache caused another guest command")
	}
}

func TestGuestMemoryLifecycleCancelledCallerCannotCreateBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &Monitor{}
	info := memoryLifecycleSample(4096)
	client := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &info}
	if got, err := m.getVMAgentMemoryAvailability(ctx, client, "cancelled", "node", 105); !errors.Is(err, proxmox.ErrGuestAgentDeferred) || got != (proxmox.LinuxMemoryAvailability{}) || client.memCalls != 0 || len(m.vmAgentMemCache) != 0 {
		t.Error("cancelled admission dispatched a command or created evidence/backoff")
	}
}
