package websocket

import (
	"reflect"
	"testing"
	"time"
)

func TestHubGlobalEventBoundary(t *testing.T) {
	for _, messageType := range []string{
		"discovery_started", "discovery_server_found", "discovery_progress",
		"discovery_update", "discovery_complete", "node_auto_registered",
		"nodes_changed", "node_deleted", "nodes_updated", "alert", "alertResolved",
		"custom", "future_tenant_event",
	} {
		t.Run(messageType, func(t *testing.T) {
			hub := NewHub(nil)
			go hub.Run()
			t.Cleanup(hub.Stop)
			registerTenantTestClient(t, hub, "client-a", "org-a")
			other := registerTenantTestClient(t, hub, "client-b", "org-b")
			hub.BroadcastMessage(Message{
				Type: messageType,
				Data: map[string]string{"orgId": "org-a", "subnet": "192.0.2.0/24"},
			})
			assertNoClientMessage(t, other, 50*time.Millisecond)
		})
	}
}

func TestHubTenantEventDelivery(t *testing.T) {
	for _, orgID := range []string{"org-a", "default", ""} {
		t.Run("org="+orgID, func(t *testing.T) {
			hub := NewHub(nil)
			go hub.Run()
			t.Cleanup(hub.Stop)
			owner := registerTenantTestClient(t, hub, "owner", normalizeOrgID(orgID))
			second := registerTenantTestClient(t, hub, "second", normalizeOrgID(orgID))
			other := registerTenantTestClient(t, hub, "other", "org-b")
			otherOrgID := "default"
			if normalizeOrgID(orgID) == "default" {
				otherOrgID = "org-a"
			}
			otherOrg := registerTenantTestClient(t, hub, "other-org", otherOrgID)
			for _, messageType := range []string{
				"discovery_started", "discovery_server_found", "discovery_progress",
				"discovery_update", "discovery_complete", "node_auto_registered",
				"nodes_changed", "node_deleted", "nodes_updated",
			} {
				want := Message{Type: messageType, Data: map[string]interface{}{"host": "192.0.2.10"}, Timestamp: "event-time"}
				hub.BroadcastMessageToTenant(orgID, want)
				for _, client := range []*Client{owner, second} {
					if got := readClientMessage(t, client, time.Second); !reflect.DeepEqual(got, want) {
						t.Fatalf("event changed: got %#v, want %#v", got, want)
					}
				}
			}
			// Discovery retains its existing custom envelope on the wire.
			for _, messageType := range []string{"discovery_started", "discovery_server_found", "discovery_progress", "discovery_update", "discovery_complete"} {
				hub.BroadcastToTenant(orgID, Message{Type: messageType})
				for _, client := range []*Client{owner, second} {
					got := readClientMessage(t, client, time.Second)
					payload, ok := got.Data.(map[string]interface{})
					if got.Type != "custom" || !ok || payload["type"] != messageType || got.Timestamp == "" {
						t.Fatalf("custom event changed: %#v", got)
					}
				}
			}
			assertNoClientMessage(t, other, 50*time.Millisecond)
			assertNoClientMessage(t, otherOrg, 50*time.Millisecond)
		})
	}
}

func TestHubEmptyOrgEventsBelongToDefaultTenant(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	owner := registerTenantTestClient(t, hub, "legacy", "")
	other := registerTenantTestClient(t, hub, "other", "org-b")
	hub.BroadcastAlertToTenant("", map[string]string{"id": "one"})
	hub.BroadcastAlertResolvedToTenant("", "one")
	hub.BroadcastToTenant("", map[string]string{"host": "192.0.2.10"})
	for _, want := range []string{"alert", "alertResolved", "custom"} {
		if got := readClientMessage(t, owner, time.Second); got.Type != want {
			t.Fatalf("got %q, want %q", got.Type, want)
		}
	}
	assertNoClientMessage(t, other, 50*time.Millisecond)
}

func TestHubInstanceEventsReachAllTenants(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	clients := []*Client{
		registerTenantTestClient(t, hub, "a", "org-a"),
		registerTenantTestClient(t, hub, "b", "org-b"),
		registerTenantTestClient(t, hub, "default", "default"),
	}
	for _, messageType := range []string{"ping", "settingsUpdate", "update:progress"} {
		hub.BroadcastMessage(Message{Type: messageType})
		for _, client := range clients {
			if got := readClientMessage(t, client, time.Second); got.Type != messageType {
				t.Fatalf("got %q, want %q", got.Type, messageType)
			}
		}
	}
}
