package chat

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func TestNewResolvedContext(t *testing.T) {
	ctx := NewResolvedContext("session-1")

	if ctx.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want %q", ctx.SessionID, "session-1")
	}
	if ctx.ttl != DefaultResolvedContextTTL {
		t.Errorf("ttl = %v, want %v", ctx.ttl, DefaultResolvedContextTTL)
	}
	if ctx.maxEntries != DefaultResolvedContextMaxEntries {
		t.Errorf("maxEntries = %d, want %d", ctx.maxEntries, DefaultResolvedContextMaxEntries)
	}
}

func TestNewResolvedContextWithConfig(t *testing.T) {
	ctx := NewResolvedContextWithConfig("session-1", 10*time.Minute, 100)

	if ctx.ttl != 10*time.Minute {
		t.Errorf("ttl = %v, want %v", ctx.ttl, 10*time.Minute)
	}
	if ctx.maxEntries != 100 {
		t.Errorf("maxEntries = %d, want %d", ctx.maxEntries, 100)
	}
}

func TestResolvedContextAddAndGet(t *testing.T) {
	ctx := NewResolvedContext("session-1")

	// Add a resource
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "abc123",
		Name:        "nginx",
		Aliases:     []string{"nginx", "abc123", "abc123def456"},
		HostName:    "server1",
		Executors: []tools.ExecutorRegistration{{
			ExecutorID: "server1",
			Adapter:    "docker",
			Actions:    []string{"restart", "stop", "start"},
			Priority:   10,
		}},
	})

	// Test GetResource by name
	res, ok := ctx.GetResource("nginx")
	if !ok {
		t.Error("GetResource should find nginx")
	}
	if res.Name != "nginx" {
		t.Errorf("Name = %q, want %q", res.Name, "nginx")
	}

	// Test GetResolvedResourceByAlias
	info, ok := ctx.GetResolvedResourceByAlias("abc123")
	if !ok {
		t.Error("GetResolvedResourceByAlias should find abc123")
	}
	if info.GetProviderUID() != "abc123" {
		t.Errorf("ProviderUID = %q, want %q", info.GetProviderUID(), "abc123")
	}

	// Test GetResolvedResourceByID
	// Note: canonical ID format includes host scope: kind:host:provider_uid
	info, ok = ctx.GetResolvedResourceByID("docker_container:server1:abc123")
	if !ok {
		t.Error("GetResolvedResourceByID should find docker_container:server1:abc123")
	}
	if info.GetKind() != "docker_container" {
		t.Errorf("Kind = %q, want %q", info.GetKind(), "docker_container")
	}
}

func TestResolvedContextLRUEviction(t *testing.T) {
	// Create context with very small max entries
	ctx := NewResolvedContextWithConfig("session-1", 1*time.Hour, 3)

	// Add 5 resources
	for i := 0; i < 5; i++ {
		ctx.AddResolvedResource(tools.ResourceRegistration{
			Kind:        "docker_container",
			ProviderUID: string(rune('a' + i)),
			Name:        string(rune('a' + i)),
			Executors:   []tools.ExecutorRegistration{},
		})
		// Small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Should have evicted to max 3 entries
	stats := ctx.Stats()
	if stats.UniqueResources > 3 {
		t.Errorf("UniqueResources = %d, want <= 3", stats.UniqueResources)
	}

	// First two resources (a, b) should be evicted as LRU
	_, okA := ctx.GetResolvedResourceByID("docker_container:a")
	_, okB := ctx.GetResolvedResourceByID("docker_container:b")
	_, okE := ctx.GetResolvedResourceByID("docker_container:e")

	// Note: exact eviction depends on timing, but 'e' should definitely exist
	if !okE {
		t.Error("Resource 'e' should exist (most recently added)")
	}

	// At least one of the first two should be evicted
	if okA && okB {
		t.Error("At least one of resources 'a' or 'b' should be evicted")
	}
}

func TestResolvedContextTTLExpiry(t *testing.T) {
	// Create context with very short TTL
	ctx := NewResolvedContextWithConfig("session-1", 10*time.Millisecond, 100)

	// Add a resource
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "test123",
		Name:        "test",
		Executors:   []tools.ExecutorRegistration{},
	})

	// Should find it immediately
	_, ok := ctx.GetResolvedResourceByID("docker_container:test123")
	if !ok {
		t.Error("Resource should be found immediately after adding")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Should be expired now
	_, ok = ctx.GetResolvedResourceByID("docker_container:test123")
	if ok {
		t.Error("Resource should be expired after TTL")
	}
}

func TestResolvedContextPinning(t *testing.T) {
	// Create context with very short TTL
	ctx := NewResolvedContextWithConfig("session-1", 10*time.Millisecond, 100)

	// Add and pin a resource
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "pinned123",
		Name:        "pinned",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.PinResource("docker_container:pinned123")

	// Verify it's pinned
	if !ctx.IsPinned("docker_container:pinned123") {
		t.Error("Resource should be pinned")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Pinned resource should still exist
	_, ok := ctx.GetResolvedResourceByID("docker_container:pinned123")
	if !ok {
		t.Error("Pinned resource should survive TTL expiry")
	}

	// Unpin and verify
	ctx.UnpinResource("docker_container:pinned123")
	if ctx.IsPinned("docker_container:pinned123") {
		t.Error("Resource should be unpinned")
	}
}

func TestResolvedContextPinningSurvivesLRU(t *testing.T) {
	// Create context with very small max entries
	ctx := NewResolvedContextWithConfig("session-1", 1*time.Hour, 3)

	// Add and pin first resource
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "first",
		Name:        "first",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.PinResource("docker_container:first")

	// Add more resources to trigger LRU
	for i := 0; i < 5; i++ {
		ctx.AddResolvedResource(tools.ResourceRegistration{
			Kind:        "docker_container",
			ProviderUID: string(rune('a' + i)),
			Name:        string(rune('a' + i)),
			Executors:   []tools.ExecutorRegistration{},
		})
		time.Sleep(1 * time.Millisecond)
	}

	// Pinned resource should still exist
	_, ok := ctx.GetResolvedResourceByID("docker_container:first")
	if !ok {
		t.Error("Pinned resource should survive LRU eviction")
	}
}

func TestResolvedContextClear(t *testing.T) {
	ctx := NewResolvedContext("session-1")

	// Add some resources
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "a",
		Name:        "a",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "b",
		Name:        "b",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.PinResource("docker_container:a")

	// Clear without keeping pinned
	ctx.Clear(false)
	stats := ctx.Stats()
	if stats.UniqueResources != 0 {
		t.Errorf("After Clear(false), UniqueResources = %d, want 0", stats.UniqueResources)
	}

	// Re-add resources
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "a",
		Name:        "a",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "b",
		Name:        "b",
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.PinResource("docker_container:a")

	// Clear keeping pinned
	ctx.Clear(true)
	stats = ctx.Stats()
	if stats.UniqueResources != 1 {
		t.Errorf("After Clear(true), UniqueResources = %d, want 1", stats.UniqueResources)
	}
	_, ok := ctx.GetResolvedResourceByID("docker_container:a")
	if !ok {
		t.Error("Pinned resource 'a' should survive Clear(true)")
	}
}

func TestResolvedContextStats(t *testing.T) {
	ctx := NewResolvedContextWithConfig("session-1", 30*time.Minute, 200)

	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "abc",
		Name:        "nginx",
		Aliases:     []string{"nginx", "web", "abc"},
		Executors:   []tools.ExecutorRegistration{},
	})
	ctx.PinResource("docker_container:abc")

	stats := ctx.Stats()
	if stats.UniqueResources != 1 {
		t.Errorf("UniqueResources = %d, want 1", stats.UniqueResources)
	}
	// Aliases include: nginx, web, abc (3 aliases)
	if stats.TotalAliases < 3 {
		t.Errorf("TotalAliases = %d, want >= 3", stats.TotalAliases)
	}
	if stats.PinnedResources != 1 {
		t.Errorf("PinnedResources = %d, want 1", stats.PinnedResources)
	}
	if stats.MaxEntries != 200 {
		t.Errorf("MaxEntries = %d, want 200", stats.MaxEntries)
	}
	if stats.TTL != 30*time.Minute {
		t.Errorf("TTL = %v, want %v", stats.TTL, 30*time.Minute)
	}
}

func TestResolvedContextAccessUpdatesLRU(t *testing.T) {
	ctx := NewResolvedContextWithConfig("session-1", 1*time.Hour, 3)

	// Add 3 resources
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "oldest",
		Name:        "oldest",
		Executors:   []tools.ExecutorRegistration{},
	})
	time.Sleep(5 * time.Millisecond)

	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "middle",
		Name:        "middle",
		Executors:   []tools.ExecutorRegistration{},
	})
	time.Sleep(5 * time.Millisecond)

	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "newest",
		Name:        "newest",
		Executors:   []tools.ExecutorRegistration{},
	})
	time.Sleep(5 * time.Millisecond)

	// Access 'oldest' to update its LRU time
	ctx.GetResolvedResourceByID("docker_container:oldest")
	time.Sleep(5 * time.Millisecond)

	// Add one more resource to trigger eviction
	ctx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "docker_container",
		ProviderUID: "extra",
		Name:        "extra",
		Executors:   []tools.ExecutorRegistration{},
	})

	// 'oldest' should still exist (recently accessed)
	_, okOldest := ctx.GetResolvedResourceByID("docker_container:oldest")
	if !okOldest {
		t.Error("'oldest' should survive because it was recently accessed")
	}

	// 'middle' should be evicted (was LRU at eviction time)
	_, okMiddle := ctx.GetResolvedResourceByID("docker_container:middle")
	if okMiddle {
		t.Error("'middle' should be evicted as it was LRU")
	}
}
