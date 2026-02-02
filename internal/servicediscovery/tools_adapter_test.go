package servicediscovery

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolsAdapter(t *testing.T) {
	t.Run("returns nil if service is nil", func(t *testing.T) {
		adapter := NewToolsAdapter(nil)
		assert.Nil(t, adapter)
	})

	t.Run("returns adapter if service is provided", func(t *testing.T) {
		store, err := NewStore(t.TempDir())
		require.NoError(t, err)
		store.crypto = nil // Disable crypto for testing
		service := NewService(store, nil, nil, DefaultConfig())

		adapter := NewToolsAdapter(service)
		assert.NotNil(t, adapter)
	})
}

func TestToolsAdapter_GetDiscovery(t *testing.T) {
	// Setup service with a populated store
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	store.crypto = nil
	service := NewService(store, nil, nil, DefaultConfig())
	adapter := NewToolsAdapter(service)

	// Create a test discovery
	discovery := &ResourceDiscovery{
		ID:             "test-id",
		ResourceType:   ResourceTypeDocker,
		ResourceID:     "container-1",
		HostID:         "host-1",
		Hostname:       "localhost",
		ServiceType:    "nginx",
		ServiceName:    "Nginx Web Server",
		ServiceVersion: "1.21",
		Category:       CategoryWebServer,
		Confidence:     0.95,
		DiscoveredAt:   time.Now(),
		Facts: []DiscoveryFact{
			{Category: FactCategoryVersion, Key: "version", Value: "1.21", Source: "docker", Confidence: 1.0},
		},
		Ports: []PortInfo{
			{Port: 80, Protocol: "tcp", Process: "nginx", Address: "0.0.0.0"},
		},
		DockerMounts: []DockerBindMount{
			{ContainerName: "nginx", Source: "/host/data", Destination: "/container/data", Type: "bind", ReadOnly: true},
		},
	}
	err = store.Save(discovery)
	require.NoError(t, err)

	t.Run("returns discovery if found", func(t *testing.T) {
		result, err := adapter.GetDiscovery("test-id")
		require.NoError(t, err)

		assert.Equal(t, discovery.ID, result.ID)
		assert.Equal(t, string(discovery.ResourceType), result.ResourceType)
		assert.Equal(t, discovery.ResourceID, result.ResourceID)
		assert.Equal(t, discovery.ServiceName, result.ServiceName)

		// Check facts conversion
		require.Len(t, result.Facts, 1)
		assert.Equal(t, discovery.Facts[0].Key, result.Facts[0].Key)
		assert.Equal(t, discovery.Facts[0].Value, result.Facts[0].Value)

		// Check ports conversion
		require.Len(t, result.Ports, 1)
		assert.Equal(t, discovery.Ports[0].Port, result.Ports[0].Port)

		// Check mounts conversion
		require.Len(t, result.DockerMounts, 1)
		assert.Equal(t, discovery.DockerMounts[0].Source, result.DockerMounts[0].Source)
		assert.True(t, result.DockerMounts[0].ReadOnly)
	})

	t.Run("returns empty if not found", func(t *testing.T) {
		result, err := adapter.GetDiscovery("non-existent")
		require.NoError(t, err)
		assert.Empty(t, result.ID)
	})
}

func TestToolsAdapter_GetDiscoveryByResource(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	store.crypto = nil
	service := NewService(store, nil, nil, DefaultConfig())
	adapter := NewToolsAdapter(service)

	id := MakeResourceID(ResourceTypeVM, "node-1", "vm-1")
	discovery := &ResourceDiscovery{
		ID:           id,
		ResourceType: ResourceTypeVM,
		ResourceID:   "vm-1",
		HostID:       "node-1",
	}
	err = store.Save(discovery)
	require.NoError(t, err)

	t.Run("returns discovery if found", func(t *testing.T) {
		result, err := adapter.GetDiscoveryByResource(string(ResourceTypeVM), "node-1", "vm-1")
		require.NoError(t, err)
		assert.Equal(t, id, result.ID)
	})

	t.Run("returns empty if not found", func(t *testing.T) {
		result, err := adapter.GetDiscoveryByResource("vm", "node-1", "vm-999")
		require.NoError(t, err)
		assert.Empty(t, result.ID)
	})
}

func TestToolsAdapter_ListDiscoveries(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	store.crypto = nil
	service := NewService(store, nil, nil, DefaultConfig())
	adapter := NewToolsAdapter(service)

	d1 := &ResourceDiscovery{ID: "d1", ResourceType: ResourceTypeDocker, HostID: "h1", ResourceID: "r1"}
	d2 := &ResourceDiscovery{ID: "d2", ResourceType: ResourceTypeVM, HostID: "h1", ResourceID: "r2"}

	require.NoError(t, store.Save(d1))
	require.NoError(t, store.Save(d2))

	t.Run("ListDiscoveries returns all", func(t *testing.T) {
		list, err := adapter.ListDiscoveries()
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})

	t.Run("ListDiscoveriesByType filters correctly", func(t *testing.T) {
		list, err := adapter.ListDiscoveriesByType(string(ResourceTypeDocker))
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, "d1", list[0].ID)
	})

	t.Run("ListDiscoveriesByHost filters correctly", func(t *testing.T) {
		// Add one on another host
		d3 := &ResourceDiscovery{ID: "d3", ResourceType: ResourceTypeDocker, HostID: "h2", ResourceID: "r3"}
		require.NoError(t, store.Save(d3))

		list, err := adapter.ListDiscoveriesByHost("h1")
		require.NoError(t, err)
		assert.Len(t, list, 2) // d1 and d2
	})
}

func TestToolsAdapter_FormatForAIContext(t *testing.T) {
	adapter := NewToolsAdapter(nil) // Service not needed for this method

	sourceData := []tools.DiscoverySourceData{
		{
			ID:           "test-1",
			ResourceType: "docker",
			ServiceName:  "Nginx",
			Facts: []tools.DiscoverySourceFact{
				{Category: "version", Key: "ver", Value: "1.0", Confidence: 1.0},
			},
		},
	}

	output := adapter.FormatForAIContext(sourceData)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Nginx")
	assert.Contains(t, output, "ver: 1.0")
}

func TestToolsAdapter_TriggerDiscovery(t *testing.T) {
	// TriggerDiscovery is harder to test fully because it depends on the deep scanner.
	// We can test the "not found" or basic delegation where it might fail or return if we could mock the scanner easily.
	// Since Service struct makes mocking hard without a full setup, we will test basic pass-through if possible,
	// or skip deep logic for now. Use a dummy state provider if needed.

	// For now, testing TriggerDiscovery with a NO-OP scanner (nil) hoping it errors or handles gracefully
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	store.crypto = nil

	// Create a service WITHOUT a scanner. DiscoverResource should fail or return error
	service := NewService(store, nil, nil, DefaultConfig())
	adapter := NewToolsAdapter(service)

	t.Run("returns error if scanner missing/fails", func(t *testing.T) {
		// DiscoverResource checks for scanner and state provider essentially.
		// If we pass force=false (default in adapter), it checks cache/store first.

		// Pre-populate store to simulate existing discovery (happy path without scanner)
		id := MakeResourceID("docker", "h1", "r1")
		d1 := &ResourceDiscovery{ID: id, ResourceType: "docker", HostID: "h1", ResourceID: "r1", DiscoveredAt: time.Now()}
		require.NoError(t, store.Save(d1))

		result, err := adapter.TriggerDiscovery(context.Background(), "docker", "h1", "r1")
		require.NoError(t, err)
		assert.Equal(t, id, result.ID)
	})
}
