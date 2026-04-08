package hypervisor

import (
	"context"
	"testing"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	id          string
	name        string
	provType    ProviderType
	healthy     bool
	closed      bool
	nodes       []Node
	vms         []VM
	connectErr  error
}

func (m *mockProvider) Type() ProviderType                                    { return m.provType }
func (m *mockProvider) ID() string                                            { return m.id }
func (m *mockProvider) Name() string                                          { return m.name }
func (m *mockProvider) Connect(_ context.Context) error                       { return m.connectErr }
func (m *mockProvider) Close() error                                          { m.closed = true; return nil }
func (m *mockProvider) Healthy(_ context.Context) bool                        { return m.healthy }
func (m *mockProvider) GetNodes(_ context.Context) ([]Node, error)            { return m.nodes, nil }
func (m *mockProvider) GetVMs(_ context.Context, _ string) ([]VM, error)      { return m.vms, nil }
func (m *mockProvider) GetContainers(_ context.Context, _ string) ([]Container, error) { return nil, nil }
func (m *mockProvider) GetStorage(_ context.Context, _ string) ([]Storage, error)     { return nil, nil }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "test-1", name: "Test", provType: ProviderProxmox, healthy: true}

	if err := r.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, ok := r.Get("test-1")
	if !ok {
		t.Fatal("expected to find registered provider")
	}
	if got.ID() != "test-1" {
		t.Errorf("expected id test-1, got %s", got.ID())
	}
}

func TestRegistryReplaceProvider(t *testing.T) {
	r := NewRegistry()
	p1 := &mockProvider{id: "test-1", name: "Old", provType: ProviderProxmox}
	p2 := &mockProvider{id: "test-1", name: "New", provType: ProviderProxmox}

	_ = r.Register(p1)
	_ = r.Register(p2)

	if !p1.closed {
		t.Error("expected old provider to be closed on replacement")
	}
	got, _ := r.Get("test-1")
	if got.Name() != "New" {
		t.Errorf("expected new provider, got %s", got.Name())
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "test-1", name: "Test", provType: ProviderVMware}
	_ = r.Register(p)

	r.Remove("test-1")
	if !p.closed {
		t.Error("expected provider to be closed on removal")
	}
	_, ok := r.Get("test-1")
	if ok {
		t.Error("expected provider to be removed")
	}
}

func TestRegistryByType(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{id: "pve-1", provType: ProviderProxmox})
	_ = r.Register(&mockProvider{id: "vmw-1", provType: ProviderVMware})
	_ = r.Register(&mockProvider{id: "pve-2", provType: ProviderProxmox})

	pveProviders := r.ByType(ProviderProxmox)
	if len(pveProviders) != 2 {
		t.Errorf("expected 2 proxmox providers, got %d", len(pveProviders))
	}
	vmwProviders := r.ByType(ProviderVMware)
	if len(vmwProviders) != 1 {
		t.Errorf("expected 1 vmware provider, got %d", len(vmwProviders))
	}
}

func TestRegistryNilProvider(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Error("expected error registering nil provider")
	}
}

func TestRegistryHealthCheck(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{
		id: "healthy-1", name: "Healthy", provType: ProviderProxmox, healthy: true,
		nodes: []Node{{ID: "n1"}}, vms: []VM{{ID: "v1"}, {ID: "v2"}},
	})
	_ = r.Register(&mockProvider{
		id: "unhealthy-1", name: "Down", provType: ProviderVMware, healthy: false,
	})

	results := r.HealthCheck(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var healthyResult, unhealthyResult *ProviderHealth
	for i := range results {
		if results[i].ProviderID == "healthy-1" {
			healthyResult = &results[i]
		} else {
			unhealthyResult = &results[i]
		}
	}

	if !healthyResult.Connected {
		t.Error("expected healthy provider to be connected")
	}
	if healthyResult.NodeCount != 1 {
		t.Errorf("expected 1 node, got %d", healthyResult.NodeCount)
	}
	if healthyResult.VMCount != 2 {
		t.Errorf("expected 2 vms, got %d", healthyResult.VMCount)
	}
	if unhealthyResult.Connected {
		t.Error("expected unhealthy provider to be disconnected")
	}
}
