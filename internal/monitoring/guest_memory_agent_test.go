package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type guestMemoryAgentTestClient struct {
	*stubPVEClient
	vmRRDPoints  []proxmox.GuestRRDPoint
	memAvailable uint64
	memErr       error
	rrdCalls     int
	memCalls     int
}

func (c *guestMemoryAgentTestClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	c.rrdCalls++
	return c.vmRRDPoints, nil
}

func (c *guestMemoryAgentTestClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	c.memCalls++
	if c.memErr != nil {
		return 0, c.memErr
	}
	return c.memAvailable, nil
}

func TestGetVMRRDMetricsCacheKeyIncludesInstance(t *testing.T) {
	t.Parallel()

	giB := float64(1024 * 1024 * 1024)
	mon := &Monitor{vmRRDMemCache: make(map[string]rrdMemCacheEntry)}
	clientA := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		vmRRDPoints:   []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(3 * giB)}},
	}
	clientB := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		vmRRDPoints:   []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(5 * giB)}},
	}

	gotA, err := mon.getVMRRDMetrics(context.Background(), clientA, "pve-a", "node1", 100)
	if err != nil {
		t.Fatalf("getVMRRDMetrics(pve-a) error = %v", err)
	}
	gotB, err := mon.getVMRRDMetrics(context.Background(), clientB, "pve-b", "node1", 100)
	if err != nil {
		t.Fatalf("getVMRRDMetrics(pve-b) error = %v", err)
	}

	if gotA != 3*1024*1024*1024 {
		t.Fatalf("getVMRRDMetrics(pve-a) = %d, want %d", gotA, uint64(3*1024*1024*1024))
	}
	if gotB != 5*1024*1024*1024 {
		t.Fatalf("getVMRRDMetrics(pve-b) = %d, want %d", gotB, uint64(5*1024*1024*1024))
	}
	if clientA.rrdCalls != 1 || clientB.rrdCalls != 1 {
		t.Fatalf("expected isolated cache fetches, got clientA=%d clientB=%d", clientA.rrdCalls, clientB.rrdCalls)
	}
}

func TestGetVMAgentMemAvailableCachesResults(t *testing.T) {
	t.Parallel()

	t.Run("positive cache", func(t *testing.T) {
		mon := &Monitor{vmAgentMemCache: make(map[string]agentMemCacheEntry)}
		client := &guestMemoryAgentTestClient{
			stubPVEClient: &stubPVEClient{},
			memAvailable:  5 * 1024 * 1024 * 1024,
		}

		first, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve-a", "node1", 100)
		if err != nil {
			t.Fatalf("first getVMAgentMemAvailable error = %v", err)
		}
		second, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve-a", "node1", 100)
		if err != nil {
			t.Fatalf("second getVMAgentMemAvailable error = %v", err)
		}

		if first != client.memAvailable || second != client.memAvailable {
			t.Fatalf("cached guest agent memavailable = (%d, %d), want %d", first, second, client.memAvailable)
		}
		if client.memCalls != 1 {
			t.Fatalf("expected positive result to be cached, got %d calls", client.memCalls)
		}
	})

	t.Run("negative cache", func(t *testing.T) {
		mon := &Monitor{vmAgentMemCache: make(map[string]agentMemCacheEntry)}
		client := &guestMemoryAgentTestClient{
			stubPVEClient: &stubPVEClient{},
			memErr:        errors.New("boom"),
		}

		if _, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve-a", "node1", 100); err == nil {
			t.Fatal("expected first guest agent memavailable lookup to fail")
		}
		if _, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve-a", "node1", 100); err == nil {
			t.Fatal("expected cached negative guest agent memavailable lookup to fail")
		}
		if client.memCalls != 1 {
			t.Fatalf("expected negative result to back off, got %d calls", client.memCalls)
		}
	})
}

func TestGetVMAgentMemAvailableRetriesKnownNonWindowsGuestSoonerAfterNegativeCache(t *testing.T) {
	t.Parallel()

	client := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		memAvailable:  2 * 1024 * 1024 * 1024,
	}
	mon := &Monitor{
		guestMetadataCache: map[string]guestMetadataCacheEntry{
			guestMetadataCacheKey("pve1", "node1", 100): {
				osName:    "Ubuntu",
				fetchedAt: time.Now(),
			},
		},
		vmAgentMemCache: map[string]agentMemCacheEntry{
			guestMemoryCacheKey("pve1", "node1", 100): {
				negative:  true,
				fetchedAt: time.Now().Add(-vmAgentMemNegativeKnownGuestTTL - time.Second),
			},
		},
	}

	available, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve1", "node1", 100)
	if err != nil {
		t.Fatalf("getVMAgentMemAvailable() error = %v", err)
	}
	if available != 2*1024*1024*1024 {
		t.Fatalf("getVMAgentMemAvailable() available = %d", available)
	}
	if client.memCalls != 1 {
		t.Fatalf("expected guest-agent meminfo retry, got %d calls", client.memCalls)
	}
}

func TestGetVMAgentMemAvailableKeepsLongNegativeCacheForWindowsGuest(t *testing.T) {
	t.Parallel()

	client := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		memAvailable:  2 * 1024 * 1024 * 1024,
	}
	mon := &Monitor{
		guestMetadataCache: map[string]guestMetadataCacheEntry{
			guestMetadataCacheKey("pve1", "node1", 100): {
				osName:    "Microsoft Windows",
				fetchedAt: time.Now(),
			},
		},
		vmAgentMemCache: map[string]agentMemCacheEntry{
			guestMemoryCacheKey("pve1", "node1", 100): {
				negative:  true,
				fetchedAt: time.Now().Add(-vmAgentMemNegativeKnownGuestTTL - time.Second),
			},
		},
	}

	available, err := mon.getVMAgentMemAvailable(context.Background(), client, "pve1", "node1", 100)
	if err == nil {
		t.Fatal("expected cached negative result for Windows guest")
	}
	if available != 0 {
		t.Fatalf("expected no memavailable result, got %d", available)
	}
	if client.memCalls != 0 {
		t.Fatalf("expected Windows guest negative cache to suppress retry, got %d calls", client.memCalls)
	}
}

func TestResolveGuestStatusMemoryUsesGuestAgentMeminfoFallback(t *testing.T) {
	t.Parallel()

	const giB = uint64(1024 * 1024 * 1024)

	mon := &Monitor{
		vmRRDMemCache:   make(map[string]rrdMemCacheEntry),
		vmAgentMemCache: make(map[string]agentMemCacheEntry),
	}
	client := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		memAvailable:  5 * giB,
	}
	raw := &VMMemoryRaw{}

	memTotal, memUsed, source := mon.resolveGuestStatusMemory(
		context.Background(),
		client,
		"pve-a",
		"vm-100",
		"node1",
		100,
		"pve-a:node1:100",
		&proxmox.VMStatus{
			Agent:  proxmox.VMAgentField{Value: 1},
			MaxMem: 8 * giB,
		},
		nil,
		8*giB,
		"",
		raw,
	)

	if memTotal != 8*giB {
		t.Fatalf("memTotal = %d, want %d", memTotal, uint64(8*giB))
	}
	if memUsed != 3*giB {
		t.Fatalf("memUsed = %d, want %d", memUsed, uint64(3*giB))
	}
	if source != "guest-agent-meminfo" {
		t.Fatalf("source = %q, want guest-agent-meminfo", source)
	}
	if raw.GuestAgentMemAvailable != 5*giB {
		t.Fatalf("raw.GuestAgentMemAvailable = %d, want %d", raw.GuestAgentMemAvailable, uint64(5*giB))
	}
	if client.memCalls != 1 {
		t.Fatalf("expected guest agent meminfo fallback to be queried once, got %d calls", client.memCalls)
	}
}

func TestResolveGuestStatusMemoryPrefersGuestAgentMeminfoForSaturatedStatus(t *testing.T) {
	t.Parallel()

	const giB = uint64(1024 * 1024 * 1024)

	mon := &Monitor{
		vmRRDMemCache:   make(map[string]rrdMemCacheEntry),
		vmAgentMemCache: make(map[string]agentMemCacheEntry),
	}
	client := &guestMemoryAgentTestClient{
		stubPVEClient: &stubPVEClient{},
		vmRRDPoints:   []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(512 * 1024 * 1024))}},
		memAvailable:  4 * giB,
	}
	raw := &VMMemoryRaw{}

	memTotal, memUsed, source := mon.resolveGuestStatusMemory(
		context.Background(),
		client,
		"cluster-a",
		"linux-vm",
		"pve2",
		164,
		"cluster-a:pve2:164",
		&proxmox.VMStatus{
			Agent:  proxmox.VMAgentField{Value: 1},
			MaxMem: 16 * giB,
			Mem:    16*giB + 512*1024*1024,
		},
		nil,
		16*giB,
		"cluster-resources",
		raw,
	)

	if memTotal != 16*giB {
		t.Fatalf("memTotal = %d, want %d", memTotal, uint64(16*giB))
	}
	if memUsed != 12*giB {
		t.Fatalf("memUsed = %d, want %d", memUsed, uint64(12*giB))
	}
	if source != "guest-agent-meminfo" {
		t.Fatalf("source = %q, want guest-agent-meminfo", source)
	}
	if raw.GuestAgentMemAvailable != 4*giB {
		t.Fatalf("raw.GuestAgentMemAvailable = %d, want %d", raw.GuestAgentMemAvailable, uint64(4*giB))
	}
	if client.memCalls != 1 {
		t.Fatalf("expected guest agent meminfo to be queried once, got %d calls", client.memCalls)
	}
	if client.rrdCalls != 0 {
		t.Fatalf("expected saturated guest-agent memory path to skip RRD, got %d RRD calls", client.rrdCalls)
	}
}
