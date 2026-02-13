package websocket

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBroadcastStateEnqueuesRawData(t *testing.T) {
	hub := NewHub(nil)
	state := struct {
		DockerHosts []string
	}{DockerHosts: []string{"a", "b"}}

	hub.BroadcastState(state)

	select {
	case msg := <-hub.broadcastSeq:
		if msg.Type != "rawData" {
			t.Fatalf("unexpected message type: %s", msg.Type)
		}
		if !reflect.DeepEqual(msg.Data, state) {
			t.Fatalf("unexpected state payload: %+v", msg.Data)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected broadcastSeq message")
	}
}

func TestBroadcastAlertResolvedAndCustom(t *testing.T) {
	hub := NewHub(nil)

	hub.BroadcastAlertResolved("alert-1")
	select {
	case data := <-hub.broadcast:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		if msg.Type != "alertResolved" {
			t.Fatalf("unexpected type: %s", msg.Type)
		}
		payload := msg.Data.(map[string]interface{})
		if payload["alertId"] != "alert-1" {
			t.Fatalf("unexpected alertId: %v", payload["alertId"])
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected alertResolved broadcast")
	}

	hub.Broadcast(map[string]string{"status": "ok"})
	select {
	case data := <-hub.broadcast:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		if msg.Type != "custom" {
			t.Fatalf("unexpected type: %s", msg.Type)
		}
		if msg.Timestamp == "" {
			t.Fatal("expected timestamp on custom broadcast")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected custom broadcast")
	}
}

func TestSendPingEnqueuesMessage(t *testing.T) {
	hub := NewHub(nil)
	hub.sendPing()

	select {
	case data := <-hub.broadcast:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		if msg.Type != "ping" {
			t.Fatalf("unexpected type: %s", msg.Type)
		}
		payload := msg.Data.(map[string]interface{})
		if _, ok := payload["timestamp"]; !ok {
			t.Fatal("expected ping timestamp")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected ping broadcast")
	}
}

func TestStopClosesChannel(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()

	select {
	case _, ok := <-hub.stopChan:
		if ok {
			t.Fatal("expected stopChan to be closed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected stopChan closure")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()
	hub.Stop()

	select {
	case _, ok := <-hub.stopChan:
		if ok {
			t.Fatal("expected stopChan to be closed")
		}
	default:
		t.Fatal("expected stopChan to be closed after repeated Stop calls")
	}
}

func TestTryRegisterClientReturnsFalseWhenStopped(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()

	done := make(chan bool, 1)
	go func() {
		done <- hub.tryRegisterClient(&Client{
			hub:  hub,
			id:   "stopped-client",
			send: make(chan []byte, 1),
		})
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected tryRegisterClient to reject client during shutdown")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("tryRegisterClient blocked during shutdown")
	}
}

func TestBroadcastStateSkippedWhenStopped(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()

	hub.BroadcastState(map[string]string{"status": "down"})

	select {
	case <-hub.broadcastSeq:
		t.Fatal("expected no broadcastSeq enqueue while hub is stopping")
	default:
	}
}

func TestHandleWebSocketPingPong(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Message{Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			continue
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		if msg.Type == "pong" {
			return
		}
	}

	t.Fatal("expected pong response")
}

func TestHandleWebSocket_ReadLimitExceededClosesConnection(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{
		EnableCompression: true,
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	oversizedPayload, err := json.Marshal(Message{
		Type: "ping",
		Data: strings.Repeat("x", maxWebSocketInboundMessageSize),
	})
	if err != nil {
		t.Fatalf("marshal oversized payload: %v", err)
	}
	if len(oversizedPayload) <= maxWebSocketInboundMessageSize {
		t.Fatalf("test payload must exceed read limit, got %d bytes", len(oversizedPayload))
	}

	if err := conn.WriteMessage(websocket.TextMessage, oversizedPayload); err != nil {
		t.Fatalf("write oversized payload: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
	}

	t.Fatal("expected websocket connection to close after oversized inbound message")
}
