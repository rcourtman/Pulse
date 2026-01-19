package websocket

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDispatchToClientsDropsFullClient(t *testing.T) {
	hub := NewHub(nil)
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
		id:   "client-1",
	}

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	client.send <- []byte("filled")

	hub.dispatchToClients([]byte("payload"), "drop")

	if hub.GetClientCount() != 0 {
		t.Fatalf("expected client to be dropped")
	}

	<-client.send
	if _, ok := <-client.send; ok {
		t.Fatal("expected send channel to be closed")
	}
}

func TestRunBroadcastSequencerImmediate(t *testing.T) {
	hub := NewHub(nil)
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
		id:   "client-1",
	}

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	done := make(chan struct{})
	go func() {
		hub.runBroadcastSequencer()
		close(done)
	}()

	hub.broadcastSeq <- Message{
		Type: "alert",
		Data: map[string]string{"id": "a1"},
	}

	select {
	case data := <-client.send:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		if msg.Type != "alert" {
			t.Fatalf("unexpected message type: %s", msg.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected broadcast message")
	}

	close(hub.stopChan)
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("broadcast sequencer did not exit")
	}
}

func TestRunBroadcastSequencerCoalescesRawData(t *testing.T) {
	hub := NewHub(nil)
	hub.coalesceWindow = 5 * time.Millisecond
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
		id:   "client-1",
	}

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	done := make(chan struct{})
	go func() {
		hub.runBroadcastSequencer()
		close(done)
	}()

	hub.broadcastSeq <- Message{Type: "rawData", Data: map[string]string{"value": "first"}}
	hub.broadcastSeq <- Message{Type: "rawData", Data: map[string]string{"value": "second"}}

	select {
	case data := <-client.send:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		payload := msg.Data.(map[string]interface{})
		if payload["value"] != "second" {
			t.Fatalf("expected coalesced value 'second', got %v", payload["value"])
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected coalesced message")
	}

	select {
	case <-client.send:
		t.Fatal("expected only one coalesced message")
	default:
	}

	close(hub.stopChan)
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("broadcast sequencer did not exit")
	}
}
