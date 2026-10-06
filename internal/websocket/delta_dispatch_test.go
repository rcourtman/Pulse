package websocket

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestStateDeltaDispatchSharesOnlyAcceptedBaseline(t *testing.T) {
	before, err := buildClientStateSnapshot(benchmarkFrontendState(2))
	if err != nil {
		t.Fatal(err)
	}
	state := benchmarkFrontendState(2)
	state.Resources[0].LastSeen = 9007199254740993
	current, err := buildClientStateSnapshot(state)
	if err != nil {
		t.Fatal(err)
	}
	cache := make(stateDeltaDispatch)
	first := &Client{stateSnapshot: before, send: make(chan []byte, 1)}
	second := &Client{stateSnapshot: before, send: make(chan []byte, 1)}
	blocked := &Client{stateSnapshot: before, send: make(chan []byte, 1)}
	blocked.send <- []byte("already queued")
	separate, err := buildClientStateSnapshot(benchmarkFrontendState(2))
	if err != nil {
		t.Fatal(err)
	}
	reconnect := &Client{stateSnapshot: separate, send: make(chan []byte, 1)}
	waiting := &Client{send: make(chan []byte, 1)}
	for _, c := range []*Client{first, second, reconnect} {
		if _, attempted, sent, err := c.queueStateDeltaForDispatch(current, cache); err != nil || !attempted || !sent {
			t.Fatalf("queue: %v %v %v", attempted, sent, err)
		}
	}
	a, b := <-first.send, <-second.send
	if &a[0] != &b[0] || !bytes.Contains(a, []byte(`"lastSeen":9007199254740993`)) {
		t.Fatal("accepted baseline did not share exact immutable frame")
	}
	if len(cache) != 2 {
		t.Fatalf("dispatch cached %d baselines, want shared and rehydrated only", len(cache))
	}
	if _, attempted, sent, err := blocked.queueStateDeltaForDispatch(current, cache); err != nil || !attempted || sent || blocked.stateSnapshot != before {
		t.Fatal("failed send advanced baseline")
	}
	if _, attempted, _, err := waiting.queueStateDeltaForDispatch(current, cache); err != nil || attempted || waiting.stateSnapshot != nil {
		t.Fatal("unhydrated client adopted another baseline")
	}
	if _, attempted, sent, err := first.queueStateDeltaForDispatch(current, make(stateDeltaDispatch)); err != nil || attempted || !sent {
		t.Fatal("quiet dispatch re-sent stale frame")
	}
	// Each client still applies its own frame limit before send/adoption.
	limited := &Client{stateSnapshot: before, send: make(chan []byte, 1)}
	limited.maxInboundBytes.Store(1)
	if _, attempted, sent, err := limited.queueStateDeltaForDispatch(current, cache); err != nil || !attempted || !sent {
		t.Fatalf("limit: %v %v %v", attempted, sent, err)
	}
	var marker Message
	if err := json.Unmarshal(<-limited.send, &marker); err != nil || marker.Type != stateTooLargeMessageType || limited.stateSnapshot != nil || !limited.restRecovery {
		t.Fatal("shared frame bypassed REST recovery limit")
	}
}
