package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func paddedFrontendState(count, padBytes int) models.StateFrontend {
	state := models.EmptyStateFrontend()
	state.LastUpdate = 100
	state.Resources = make([]models.ResourceFrontend, 0, count)
	for i := range count {
		id := fmt.Sprintf("agent:host-%d", i)
		state.Resources = append(state.Resources, models.ResourceFrontend{
			ID:           id,
			Type:         "agent",
			Name:         id,
			DisplayName:  id,
			PlatformData: json.RawMessage(`{"pad":"` + strings.Repeat("x", padBytes) + `"}`),
		})
	}
	return state
}

func decodeStateTooLargeMarker(t *testing.T, raw []byte) stateTooLargePayload {
	t.Helper()
	var message struct {
		Type string               `json:"type"`
		Data stateTooLargePayload `json:"data"`
	}
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("decode marker: %v", err)
	}
	if message.Type != stateTooLargeMessageType {
		t.Fatalf("message type = %q, want %q", message.Type, stateTooLargeMessageType)
	}
	return message.Data
}

func TestOversizedFullStateEntersBaselineFreeRESTRecovery(t *testing.T) {
	state := paddedFrontendState(20, 4*1024)
	client := &Client{send: make(chan []byte, 1)}
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	dataLen, sent, err := client.queueFullState("initialState", state)
	if err != nil || !sent {
		t.Fatalf("queueFullState() bytes=%d sent=%v error=%v", dataLen, sent, err)
	}
	marker := decodeStateTooLargeMarker(t, <-client.send)
	if marker.Supersedes != "initialState" || marker.Bytes != dataLen {
		t.Fatalf("marker = %+v, want initialState and %d bytes", marker, dataLen)
	}
	if marker.MaxBytes != minAdvertisedInboundBytes || marker.HydrateFrom != "/api/state" {
		t.Fatalf("marker negotiation = %+v", marker)
	}
	if client.stateSnapshot != nil || !client.restRecovery {
		t.Fatal("withheld snapshot became a delta baseline")
	}
}

func TestRESTRecoverySendsMarkersWithoutAdvancingDeltaBaseline(t *testing.T) {
	initial := paddedFrontendState(20, 4*1024)
	client := &Client{send: make(chan []byte, 2)}
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)
	if _, sent, err := client.queueFullState("initialState", initial); err != nil || !sent {
		t.Fatalf("queueFullState() sent=%v error=%v", sent, err)
	}
	<-client.send

	current := paddedFrontendState(20, 4*1024)
	current.LastUpdate = 200
	currentSnapshot, err := buildClientStateSnapshot(current)
	if err != nil {
		t.Fatal(err)
	}
	_, attempted, sent, err := client.queueStateDelta(currentSnapshot)
	if err != nil || !attempted || !sent {
		t.Fatalf("queueStateDelta() attempted=%v sent=%v error=%v", attempted, sent, err)
	}
	marker := decodeStateTooLargeMarker(t, <-client.send)
	if marker.Supersedes != "rawData" {
		t.Fatalf("marker supersedes = %q, want rawData", marker.Supersedes)
	}
	if client.stateSnapshot != nil || !client.restRecovery {
		t.Fatal("REST recovery marker advanced the delta baseline")
	}
}

func TestOversizedDeltaInvalidatesPreviouslyDeliveredBaseline(t *testing.T) {
	initial := paddedFrontendState(20, 4*1024)
	client := &Client{send: make(chan []byte, 2)}
	if _, sent, err := client.queueFullState("initialState", initial); err != nil || !sent {
		t.Fatalf("queueFullState() sent=%v error=%v", sent, err)
	}
	<-client.send
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	current := paddedFrontendState(20, 4*1024)
	current.LastUpdate = 200
	for i := range current.Resources {
		current.Resources[i].PlatformData = json.RawMessage(
			`{"pad":"` + strings.Repeat("y", 4*1024) + `"}`,
		)
	}
	currentSnapshot, err := buildClientStateSnapshot(current)
	if err != nil {
		t.Fatal(err)
	}
	dataLen, attempted, sent, err := client.queueStateDelta(currentSnapshot)
	if err != nil || !attempted || !sent {
		t.Fatalf("queueStateDelta() bytes=%d attempted=%v sent=%v error=%v", dataLen, attempted, sent, err)
	}
	marker := decodeStateTooLargeMarker(t, <-client.send)
	if marker.Bytes != dataLen || marker.Supersedes != "rawData" {
		t.Fatalf("marker = %+v, want oversized rawData of %d bytes", marker, dataLen)
	}
	if client.stateSnapshot != nil || !client.restRecovery {
		t.Fatal("oversized delta left an accepted baseline behind")
	}
}

func TestDeliverableFullStateExitsRESTRecovery(t *testing.T) {
	client := &Client{send: make(chan []byte, 2)}
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)
	if _, sent, err := client.queueFullState("initialState", paddedFrontendState(20, 4*1024)); err != nil || !sent {
		t.Fatalf("oversized queueFullState() sent=%v error=%v", sent, err)
	}
	<-client.send

	small := paddedFrontendState(1, 32)
	if _, sent, err := client.queueFullState("rawData", small); err != nil || !sent {
		t.Fatalf("deliverable queueFullState() sent=%v error=%v", sent, err)
	}
	var message Message
	if err := json.Unmarshal(<-client.send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "rawData" {
		t.Fatalf("message type = %q, want rawData", message.Type)
	}
	if client.stateSnapshot == nil || client.restRecovery {
		t.Fatal("delivered full state did not establish the new delta baseline")
	}
}

func TestApplyAdvertisedInboundLimit(t *testing.T) {
	tests := []struct {
		raw  string
		want int64
	}{
		{raw: "8388608", want: 8 * 1024 * 1024},
		{raw: " 8388608 ", want: 8 * 1024 * 1024},
		{raw: ""},
		{raw: "1024"},
		{raw: "not-a-number"},
	}
	for _, test := range tests {
		client := &Client{}
		client.applyAdvertisedInboundLimit(test.raw)
		if got := client.inboundLimitBytes(); got != test.want {
			t.Fatalf("applyAdvertisedInboundLimit(%q) = %d, want %d", test.raw, got, test.want)
		}
	}
}

func TestWebSocketUpgradeNegotiatesLimitBeforeInitialState(t *testing.T) {
	hub := NewHub(func(string) interface{} {
		return paddedFrontendState(20, 4*1024)
	})
	go hub.Run()
	t.Cleanup(hub.Stop)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"?org_id=default&max_message_bytes=65536"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeadersForHTTP(t, server.URL))
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}

	for {
		_, raw, readErr := conn.ReadMessage()
		if readErr != nil {
			t.Fatalf("read negotiated initial state: %v", readErr)
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Type == "welcome" {
			continue
		}
		if envelope.Type != stateTooLargeMessageType {
			t.Fatalf("initial state message type = %q, want %q", envelope.Type, stateTooLargeMessageType)
		}
		marker := decodeStateTooLargeMarker(t, raw)
		if marker.MaxBytes != minAdvertisedInboundBytes {
			t.Fatalf("negotiated maxBytes = %d, want %d", marker.MaxBytes, minAdvertisedInboundBytes)
		}
		return
	}
}
