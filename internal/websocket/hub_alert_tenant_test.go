package websocket

import (
	"encoding/json"
	"testing"
	"time"
)

func registerTenantTestClient(t *testing.T, hub *Hub, id, orgID string) *Client {
	t.Helper()

	client := &Client{
		hub:   hub,
		id:    id,
		orgID: orgID,
		send:  make(chan []byte, 16),
	}

	hub.register <- client

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		_, ok := hub.clients[client]
		hub.mu.RUnlock()
		if ok {
			return client
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("client %s was not registered", id)
	return nil
}

func drainClientMessages(client *Client) {
	for {
		select {
		case <-client.send:
		default:
			return
		}
	}
}

func readClientMessage(t *testing.T, client *Client, timeout time.Duration) Message {
	t.Helper()

	select {
	case data := <-client.send:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for client message")
	}

	return Message{}
}

func assertNoClientMessage(t *testing.T, client *Client, timeout time.Duration) {
	t.Helper()

	select {
	case data := <-client.send:
		t.Fatalf("unexpected message received: %s", string(data))
	case <-time.After(timeout):
	}
}

func TestAlertBroadcastTenantIsolation(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	orgAClient := registerTenantTestClient(t, hub, "client-a", "orgA")
	orgBClient := registerTenantTestClient(t, hub, "client-b", "orgB")
	drainClientMessages(orgAClient)
	drainClientMessages(orgBClient)

	hub.BroadcastAlertToTenant("orgA", map[string]string{"id": "alert-1"})

	msg := readClientMessage(t, orgAClient, 300*time.Millisecond)
	if msg.Type != "alert" {
		t.Fatalf("expected alert type, got %q", msg.Type)
	}
	payload, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", msg.Data)
	}
	if payload["id"] != "alert-1" {
		t.Fatalf("expected alert id alert-1, got %v", payload["id"])
	}

	assertNoClientMessage(t, orgBClient, 200*time.Millisecond)
}

func TestAlertResolvedBroadcastTenantIsolation(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	orgAClient := registerTenantTestClient(t, hub, "client-a", "orgA")
	orgBClient := registerTenantTestClient(t, hub, "client-b", "orgB")
	drainClientMessages(orgAClient)
	drainClientMessages(orgBClient)

	hub.BroadcastAlertResolvedToTenant("orgA", "alert-1")

	msg := readClientMessage(t, orgAClient, 300*time.Millisecond)
	if msg.Type != "alertResolved" {
		t.Fatalf("expected alertResolved type, got %q", msg.Type)
	}
	payload, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", msg.Data)
	}
	if payload["alertId"] != "alert-1" {
		t.Fatalf("expected alertId alert-1, got %v", payload["alertId"])
	}

	assertNoClientMessage(t, orgBClient, 200*time.Millisecond)
}

func TestAlertBroadcastEmptyOrgTargetsDefaultTenantOnly(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	defaultClient := registerTenantTestClient(t, hub, "client-default", "default")
	orgAClient := registerTenantTestClient(t, hub, "client-a", "orgA")
	orgBClient := registerTenantTestClient(t, hub, "client-b", "orgB")
	drainClientMessages(defaultClient)
	drainClientMessages(orgAClient)
	drainClientMessages(orgBClient)

	hub.BroadcastAlertToTenant("", map[string]string{"id": "alert-default"})

	msgDefault := readClientMessage(t, defaultClient, 300*time.Millisecond)
	if msgDefault.Type != "alert" {
		t.Fatalf("expected default tenant alert type, got %q", msgDefault.Type)
	}
	assertNoClientMessage(t, orgAClient, 200*time.Millisecond)
	assertNoClientMessage(t, orgBClient, 200*time.Millisecond)
}

func TestAlertResolvedEmptyOrgTargetsDefaultTenantOnly(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	defaultClient := registerTenantTestClient(t, hub, "client-default", "default")
	orgAClient := registerTenantTestClient(t, hub, "client-a", "orgA")
	orgBClient := registerTenantTestClient(t, hub, "client-b", "orgB")
	drainClientMessages(defaultClient)
	drainClientMessages(orgAClient)
	drainClientMessages(orgBClient)

	hub.BroadcastAlertResolvedToTenant("", "alert-default")

	msgDefault := readClientMessage(t, defaultClient, 300*time.Millisecond)
	if msgDefault.Type != "alertResolved" {
		t.Fatalf("expected default tenant alertResolved type, got %q", msgDefault.Type)
	}
	assertNoClientMessage(t, orgAClient, 200*time.Millisecond)
	assertNoClientMessage(t, orgBClient, 200*time.Millisecond)
}

func TestAlertBroadcastUnknownTenantDoesNotLeak(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	orgAClient := registerTenantTestClient(t, hub, "client-a", "orgA")
	orgBClient := registerTenantTestClient(t, hub, "client-b", "orgB")
	drainClientMessages(orgAClient)
	drainClientMessages(orgBClient)

	hub.BroadcastAlertToTenant("orgC", map[string]string{"id": "alert-unknown-org"})
	assertNoClientMessage(t, orgAClient, 250*time.Millisecond)
	assertNoClientMessage(t, orgBClient, 250*time.Millisecond)

	hub.BroadcastAlertResolvedToTenant("orgC", "alert-unknown-org")
	assertNoClientMessage(t, orgAClient, 250*time.Millisecond)
	assertNoClientMessage(t, orgBClient, 250*time.Millisecond)
}
