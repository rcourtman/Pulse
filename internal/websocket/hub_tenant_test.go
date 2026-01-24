package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockStateGetter implements StateGetter interface
type MockStateGetter struct {
	state interface{}
}

func (m *MockStateGetter) GetState() interface{} {
	return m.state
}

// MockTenantStateGetter implements TenantStateGetter interface
type MockTenantStateGetter struct {
	state map[string]interface{}
}

func (m *MockTenantStateGetter) GetState(orgID string) interface{} {
	return m.state[orgID]
}

// MockOrgAuthChecker implements OrgAuthChecker interface
type MockOrgAuthChecker struct {
	called bool
	allow  bool
}

func (m *MockOrgAuthChecker) CanAccessOrg(userID string, token interface{}, orgID string) bool {
	m.called = true
	return m.allow
}

func TestHub_Tenant_Broadcasting(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	defer func() {
		// Stop hub logic if exposed, or just let it die with test
	}()

	// Setup Tenant State Getter
	mockState := &MockTenantStateGetter{
		state: map[string]interface{}{
			"org1": map[string]string{"foo": "bar"},
			"org2": map[string]string{"baz": "qux"},
		},
	}
	hub.SetStateGetterForTenant(mockState.GetState)

	// Test GetTenantClientCount
	assert.Equal(t, 0, hub.GetTenantClientCount("org1"))

	// Test BroadcastStateToTenant (should not panic even with 0 clients)
	hub.BroadcastStateToTenant("org1", map[string]string{"status": "ok"})
	hub.BroadcastStateToTenant("org2", nil)
	hub.BroadcastStateToTenant("missing", nil)

	// Allow async broadcast to process
	time.Sleep(100 * time.Millisecond)
}

func TestHub_Setters_Coverage(t *testing.T) {
	hub := NewHub(nil)

	// Test SetOrgAuthChecker
	checker := &MockOrgAuthChecker{allow: true}
	hub.SetOrgAuthChecker(checker)

	// Verify it was set
	assert.NotNil(t, hub.orgAuthChecker)

	// Trigger the checker
	success := hub.orgAuthChecker.CanAccessOrg("user", "tok", "org")
	assert.True(t, success)
	assert.True(t, checker.called)
}

func TestHub_DispatchToTenantClients(t *testing.T) {
	// This tests the internal logic of iterating clients
	hub := NewHub(nil)

	// Create a mock client
	client := &Client{
		hub:   hub,
		send:  make(chan []byte, 256),
		orgID: "org1",
		// isActive removed
	}

	// Manually register (simulating register channel)
	hub.clients[client] = true
	hub.register <- client

	// Allow registration to process
	go hub.Run()
	time.Sleep(50 * time.Millisecond)

	// Now broadcast to org1 (internal method)
	msg := []byte("test message")
	// dispatchToTenantClients(orgID string, data []byte, dropLog string)
	// But wait, dispatchToTenantClients is private (lowercase). Can we call it?
	// Tests are in package `websocket`, so yes we can access private methods of `hub.go`.
	hub.dispatchToTenantClients("org1", msg, "Dropping test message")

	// Check if client received it
	select {
	case received := <-client.send:
		assert.Equal(t, msg, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client did not receive tenant broadcast")
	}

	// Broadcast to org2 (should not receive)
	hub.dispatchToTenantClients("org2", msg, "Dropping test message")
	select {
	case <-client.send:
		t.Fatal("Client received message for wrong tenant")
	case <-time.After(50 * time.Millisecond):
		// Success
	}
}
