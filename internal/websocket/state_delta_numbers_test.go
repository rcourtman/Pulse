package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Do not use float64 to inspect the wire under test: that would reproduce the
// production defect in the oracle and hide a dropped or rounded counter.
func decodeExactDeltaJSON(t testing.TB, encoded []byte) interface{} {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestJSONMergePatchNumberFidelity(t *testing.T) {
	for _, tc := range []struct{ name, previous, current, patch string }{
		{"adjacent-large-counters", `{"id":"r","bytes":9007199254740992}`, `{"id":"r","bytes":9007199254740993}`, `{"id":"r","bytes":9007199254740993}`},
		{"uint64-limit", `{"id":"r","bytes":18446744073709551614}`, `{"id":"r","bytes":18446744073709551615}`, `{"id":"r","bytes":18446744073709551615}`},
		{"negative-int64", `{"id":"r","bytes":-9223372036854775808}`, `{"id":"r","bytes":-9223372036854775807}`, `{"id":"r","bytes":-9223372036854775807}`},
		{"nested-counter", `{"id":"r","io":{"readBytes":9007199254740992,"writeBytes":1}}`, `{"id":"r","io":{"readBytes":9007199254740993,"writeBytes":1}}`, `{"id":"r","io":{"readBytes":9007199254740993}}`},
		{"array-replacement", `{"id":"r","samples":[1,2]}`, `{"id":"r","samples":[9007199254740993,18446744073709551615]}`, `{"id":"r","samples":[9007199254740993,18446744073709551615]}`},
		{"decimal-token", `{"id":"r","ratio":0.1}`, `{"id":"r","ratio":0.123456789012345678901}`, `{"id":"r","ratio":0.123456789012345678901}`},
		{"unchanged-large-counter", `{"id":"r","bytes":9007199254740993,"cpu":1}`, `{"id":"r","bytes":9007199254740993,"cpu":2}`, `{"id":"r","cpu":2}`},
		{"remove-field", `{"id":"r","bytes":9007199254740993,"old":true}`, `{"id":"r","bytes":9007199254740993}`, `{"id":"r","old":null}`},
		{"unchanged", `{"id":"r","bytes":18446744073709551615}`, `{"id":"r","bytes":18446744073709551615}`, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patch, err := createJSONMergePatch(json.RawMessage(tc.previous), json.RawMessage(tc.current))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := decodeExactDeltaJSON(t, patch), decodeExactDeltaJSON(t, []byte(tc.patch)); !reflect.DeepEqual(got, want) {
				t.Fatalf("numeric wire value changed or vanished: got=%s want=%s", patch, tc.patch)
			}
		})
	}
}

func TestJSONMergePatchRejectsTrailingValues(t *testing.T) {
	for _, invalid := range []string{``, `{"id":"r"} {}`, `{"id":"r"} null`, `{"id":"r"} garbage`, `{"value":01}`, `{"value":NaN}`, `{"value":`, `[1,]`} {
		for _, previousInvalid := range []bool{true, false} {
			t.Run(fmt.Sprintf("%q/previous=%t", invalid, previousInvalid), func(t *testing.T) {
				previous, current := json.RawMessage(`{"id":"r"}`), json.RawMessage(`{"id":"r","value":2}`)
				if previousInvalid {
					previous = json.RawMessage(invalid)
				} else {
					current = json.RawMessage(invalid)
				}
				if patch, err := createJSONMergePatch(previous, current); err == nil {
					t.Fatalf("invalid/trailing JSON admitted: %s", patch)
				}
			})
		}
	}
	if _, err := createJSONMergePatch(json.RawMessage(" {\"id\":\"r\"}\n\t"), json.RawMessage(" {\"id\":\"r\",\"value\":2}\n\t")); err != nil {
		t.Fatalf("ordinary trailing whitespace rejected: %v", err)
	}
}

func TestNumericCounterDeltaFollowsProductionQueues(t *testing.T) {
	current := models.EmptyStateFrontend()
	current.Resources = []models.ResourceFrontend{{ID: "r", Type: "docker", Network: &models.ResourceNetworkFrontend{RXBytes: 9007199254740992}, PlatformData: json.RawMessage(`{"blockIO":{"readBytes":18446744073709551614}}`)}}
	hub := NewHub(func(string) interface{} { return current })
	clients := []*Client{
		{hub: hub, id: "first", orgID: "own", send: make(chan []byte, 1)},
		{hub: hub, id: "second", orgID: "own", send: make(chan []byte, 1)},
		{hub: hub, id: "other", orgID: "other", send: make(chan []byte, 1)},
	}
	for _, client := range clients {
		if _, sent, err := client.queueFullState("initialState", current); err != nil || !sent {
			t.Fatalf("initial baseline: sent=%t err=%v", sent, err)
		}
		<-client.send
		hub.clients[client] = true
		if hub.clientsByTenant[client.orgID] == nil {
			hub.clientsByTenant[client.orgID] = make(map[*Client]bool)
		}
		hub.clientsByTenant[client.orgID][client] = true
	}
	otherBaseline := clients[2].stateSnapshot
	current.Resources[0].Network.RXBytes = 9007199254740993
	current.Resources[0].PlatformData = json.RawMessage(`{"blockIO":{"readBytes":18446744073709551615}}`)
	hub.dispatchStateBroadcast(&Message{Type: "rawData", Data: stateBroadcastRequest{orgID: "own"}}, "own")
	for _, client := range clients[:2] {
		select {
		case frame := <-client.send:
			want := `{"type":"rawData","data":{"resourceDelta":{"upserts":[{"id":"r","network":{"rxBytes":9007199254740993},"platformData":{"blockIO":{"readBytes":18446744073709551615}}}]}}}`
			if !reflect.DeepEqual(decodeExactDeltaJSON(t, frame), decodeExactDeltaJSON(t, []byte(want))) {
				t.Errorf("%s received rounded or missing counters: %s", client.id, frame)
			}
		default:
			t.Errorf("%s received no counter delta", client.id)
		}
	}
	select {
	case frame := <-clients[2].send:
		t.Errorf("counter data crossed tenants: %s", frame)
	default:
	}
	if clients[2].stateSnapshot != otherBaseline {
		t.Fatal("unrelated tenant baseline advanced")
	}
	// Repeating unchanged evidence sends nothing; the accepted baseline owns
	// the exact encoded values rather than a rounded synthetic replacement.
	hub.dispatchStateBroadcast(&Message{Type: "rawData", Data: stateBroadcastRequest{orgID: "own"}}, "own")
	for _, client := range clients {
		select {
		case frame := <-client.send:
			t.Errorf("unchanged counter re-sent: %s", frame)
		default:
		}
	}
}

func TestNumericCounterDeltaDoesNotAdvanceRejectedQueue(t *testing.T) {
	initial := models.EmptyStateFrontend()
	initial.Resources = []models.ResourceFrontend{{ID: "r", Network: &models.ResourceNetworkFrontend{RXBytes: 9007199254740992}}}
	client := &Client{send: make(chan []byte, 1)}
	if _, sent, err := client.queueFullState("initialState", initial); err != nil || !sent {
		t.Fatal(err)
	}
	baseline := client.stateSnapshot
	initial.Resources[0].Network.RXBytes++
	current, err := buildClientStateSnapshot(initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, attempted, sent, err := client.queueStateDelta(current); err != nil || !attempted || sent {
		t.Fatalf("full queue: attempted=%t sent=%t err=%v", attempted, sent, err)
	}
	if client.stateSnapshot != baseline {
		t.Fatal("rejected queue changed exact numeric baseline")
	}
	<-client.send
	if _, attempted, sent, err := client.queueStateDelta(current); err != nil || !attempted || !sent {
		t.Fatalf("accepted queue: attempted=%t sent=%t err=%v", attempted, sent, err)
	}
	frame := <-client.send
	if !bytes.Contains(frame, []byte(`"rxBytes":9007199254740993`)) || client.stateSnapshot != current {
		t.Fatalf("accepted frame/baseline lost counter: %s", frame)
	}
}
