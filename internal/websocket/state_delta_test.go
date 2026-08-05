package websocket

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestClientStateDeltaOmitsUnchangedResourcePayload(t *testing.T) {
	previousState := models.EmptyStateFrontend()
	previousState.LastUpdate = 100
	previousState.Resources = []models.ResourceFrontend{{
		ID:           "agent:host-1",
		Type:         "agent",
		Name:         "host-1",
		DisplayName:  "host-1",
		Status:       "online",
		LastSeen:     100,
		CPU:          &models.ResourceMetricFrontend{Current: 40},
		PlatformData: json.RawMessage(`{"static":"` + strings.Repeat("x", 16*1024) + `"}`),
	}}
	currentState := previousState
	currentState.LastUpdate = 200
	currentState.Resources = append([]models.ResourceFrontend(nil), previousState.Resources...)
	currentState.Resources[0].LastSeen = 200
	currentState.Resources[0].CPU = &models.ResourceMetricFrontend{Current: 41}

	previous, err := buildClientStateSnapshot(previousState)
	if err != nil {
		t.Fatal(err)
	}
	current, err := buildClientStateSnapshot(currentState)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := buildClientStateDelta(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(delta)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), strings.Repeat("x", 128)) {
		t.Fatal("state delta repeated unchanged platform data")
	}
	if !strings.Contains(string(encoded), `"current":41`) || !strings.Contains(string(encoded), `"lastSeen":200`) {
		t.Fatalf("state delta omitted changed telemetry: %s", encoded)
	}
	if len(encoded) >= 1024 {
		t.Fatalf("state delta = %d bytes, want a bounded telemetry patch", len(encoded))
	}
}

func TestClientStateDeltaTracksResourceRemovalAndOrder(t *testing.T) {
	previousState := models.EmptyStateFrontend()
	previousState.Resources = []models.ResourceFrontend{
		{ID: "a", Type: "agent", Name: "a", DisplayName: "a"},
		{ID: "b", Type: "agent", Name: "b", DisplayName: "b"},
	}
	currentState := models.EmptyStateFrontend()
	currentState.Resources = []models.ResourceFrontend{
		{ID: "c", Type: "agent", Name: "c", DisplayName: "c"},
		{ID: "a", Type: "agent", Name: "a", DisplayName: "a"},
	}

	previous, err := buildClientStateSnapshot(previousState)
	if err != nil {
		t.Fatal(err)
	}
	current, err := buildClientStateSnapshot(currentState)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := buildClientStateDelta(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	resources, ok := delta[resourceDeltaField].(resourceDeltaPayload)
	if !ok {
		t.Fatalf("resource delta type = %T", delta[resourceDeltaField])
	}
	if len(resources.Removed) != 1 || resources.Removed[0] != "b" {
		t.Fatalf("removed = %#v, want [b]", resources.Removed)
	}
	if len(resources.Order) != 2 || resources.Order[0] != "c" || resources.Order[1] != "a" {
		t.Fatalf("order = %#v, want [c a]", resources.Order)
	}
	if len(resources.Upserts) != 1 || !strings.Contains(string(resources.Upserts[0]), `"id":"c"`) {
		t.Fatalf("upserts = %s, want full resource c", resources.Upserts)
	}
}

func TestClientStateBaselineAdvancesOnlyAfterDeltaIsQueued(t *testing.T) {
	initial := models.EmptyStateFrontend()
	initial.LastUpdate = 100
	initial.Resources = []models.ResourceFrontend{{
		ID: "agent:host-1", Type: "agent", Name: "host-1", DisplayName: "host-1",
		CPU: &models.ResourceMetricFrontend{Current: 10},
	}}
	current := initial
	current.LastUpdate = 200
	current.Resources = append([]models.ResourceFrontend(nil), initial.Resources...)
	current.Resources[0].CPU = &models.ResourceMetricFrontend{Current: 20}

	client := &Client{send: make(chan []byte, 1)}
	if _, sent, err := client.queueFullState("initialState", initial); err != nil || !sent {
		t.Fatalf("queueFullState() sent=%v error=%v", sent, err)
	}
	initialBaseline := client.stateSnapshot
	currentSnapshot, err := buildClientStateSnapshot(current)
	if err != nil {
		t.Fatal(err)
	}

	if _, attempted, sent, err := client.queueStateDelta(currentSnapshot); err != nil || !attempted || sent {
		t.Fatalf("full-channel queueStateDelta() attempted=%v sent=%v error=%v", attempted, sent, err)
	}
	if client.stateSnapshot != initialBaseline {
		t.Fatal("client baseline advanced after the delta queue rejected the payload")
	}

	<-client.send
	if _, attempted, sent, err := client.queueStateDelta(currentSnapshot); err != nil || !attempted || !sent {
		t.Fatalf("retry queueStateDelta() attempted=%v sent=%v error=%v", attempted, sent, err)
	}
	if client.stateSnapshot != currentSnapshot {
		t.Fatal("client baseline did not advance after the delta was queued")
	}
}
