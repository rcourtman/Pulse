package websocket

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// stateWithResources builds a frontend state whose marshalled size is dominated
// by padded platform data, so tests can push a snapshot over a frame limit.
func stateWithResources(t *testing.T, count int, padBytes int) models.StateFrontend {
	t.Helper()
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
			Status:       "online",
			LastSeen:     100,
			CPU:          &models.ResourceMetricFrontend{Current: float64(i % 100)},
			PlatformData: json.RawMessage(`{"pad":"` + strings.Repeat("x", padBytes) + `"}`),
		})
	}
	return state
}

func decodeMessageType(t *testing.T, raw []byte) string {
	t.Helper()
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode queued message: %v", err)
	}
	return envelope.Type
}

// A client that never advertises a limit must keep receiving the full snapshot,
// which is what every pre-negotiation client does.
func TestQueueFullStateSendsSnapshotWhenClientAdvertisesNoLimit(t *testing.T) {
	state := stateWithResources(t, 40, 4*1024)
	client := &Client{send: make(chan []byte, 1)}

	result, err := client.queueFullState("initialState", state)
	if err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	if !result.sent || result.withheld {
		t.Fatalf("queueFullState() sent=%v withheld=%v, want sent without withholding", result.sent, result.withheld)
	}
	if got := decodeMessageType(t, <-client.send); got != "initialState" {
		t.Fatalf("queued message type = %q, want initialState", got)
	}
	if client.stateSnapshot == nil {
		t.Fatal("delta baseline was not established for a delivered snapshot")
	}
}

// Reusing the payload from the baseline build must not change the wire bytes.
func TestQueueFullStateReusesMarshalledPayloadByteForByte(t *testing.T) {
	state := stateWithResources(t, 12, 512)
	client := &Client{send: make(chan []byte, 1)}

	if _, err := client.queueFullState("initialState", state); err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	queued := <-client.send

	want, err := json.Marshal(Message{Type: "initialState", Data: state})
	if err != nil {
		t.Fatalf("reference marshal error = %v", err)
	}
	if string(queued) != string(want) {
		t.Fatalf("queued payload differs from a direct marshal\n got: %d bytes\nwant: %d bytes", len(queued), len(want))
	}
}

// The projection used to skip the second marshal must match the real wire size,
// or the limit check fires at the wrong threshold.
func TestProjectedStateMessageBytesMatchesMarshalledSize(t *testing.T) {
	state := stateWithResources(t, 7, 256)
	_, encoded, err := buildClientStateSnapshot(state)
	if err != nil {
		t.Fatalf("buildClientStateSnapshot() error = %v", err)
	}
	actual, err := json.Marshal(Message{Type: "initialState", Data: state})
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if got := projectedStateMessageBytes("initialState", len(encoded)); got != len(actual) {
		t.Fatalf("projectedStateMessageBytes() = %d, want %d", got, len(actual))
	}
}

// The core of the fix: an oversized snapshot is withheld, a compact marker goes
// out instead, and the delta baseline is still established so later deltas apply
// to the REST snapshot the client hydrates.
func TestQueueFullStateWithholdsSnapshotOverAdvertisedLimit(t *testing.T) {
	state := stateWithResources(t, 60, 4*1024)
	client := &Client{send: make(chan []byte, 1)}
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	result, err := client.queueFullState("initialState", state)
	if err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	if !result.sent || !result.withheld {
		t.Fatalf("queueFullState() sent=%v withheld=%v, want a withheld snapshot with a marker sent", result.sent, result.withheld)
	}

	queued := <-client.send
	if got := decodeMessageType(t, queued); got != stateTooLargeMessageType {
		t.Fatalf("queued message type = %q, want %q", got, stateTooLargeMessageType)
	}
	if int64(len(queued)) > minAdvertisedInboundBytes {
		t.Fatalf("marker is %d bytes, which exceeds the limit it exists to respect", len(queued))
	}

	var marker struct {
		Data stateTooLargePayload `json:"data"`
	}
	if err := json.Unmarshal(queued, &marker); err != nil {
		t.Fatalf("decode marker: %v", err)
	}
	if marker.Data.Supersedes != "initialState" {
		t.Fatalf("marker supersedes = %q, want initialState", marker.Data.Supersedes)
	}
	if marker.Data.ResourceCount != 60 {
		t.Fatalf("marker resourceCount = %d, want 60", marker.Data.ResourceCount)
	}
	if marker.Data.MaxBytes != minAdvertisedInboundBytes {
		t.Fatalf("marker maxBytes = %d, want %d", marker.Data.MaxBytes, minAdvertisedInboundBytes)
	}
	if marker.Data.HydrateFrom != "/api/state" {
		t.Fatalf("marker hydrateFrom = %q, want /api/state", marker.Data.HydrateFrom)
	}

	// Baseline coherence: without this the client is frozen on its REST copy,
	// because queueStateDelta abstains whenever the baseline is nil.
	if client.stateSnapshot == nil {
		t.Fatal("delta baseline was not established for a withheld snapshot")
	}
	if len(client.stateSnapshot.resourceOrder) != 60 {
		t.Fatalf("baseline holds %d resources, want the full withheld snapshot", len(client.stateSnapshot.resourceOrder))
	}
}

// A withheld snapshot must not stop deltas: the client hydrates the same payload
// over REST, so the baseline the server holds is the state the client holds.
func TestDeltasFlowAfterSnapshotIsWithheld(t *testing.T) {
	initial := stateWithResources(t, 60, 4*1024)
	client := &Client{send: make(chan []byte, 2)}
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	if _, err := client.queueFullState("initialState", initial); err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	<-client.send // drain the marker

	current := stateWithResources(t, 60, 4*1024)
	current.LastUpdate = 200
	current.Resources[0].CPU = &models.ResourceMetricFrontend{Current: 99}
	currentSnapshot, _, err := buildClientStateSnapshot(current)
	if err != nil {
		t.Fatalf("buildClientStateSnapshot() error = %v", err)
	}

	_, attempted, sent, err := client.queueStateDelta(currentSnapshot)
	if err != nil || !attempted || !sent {
		t.Fatalf("queueStateDelta() attempted=%v sent=%v error=%v, want a delivered delta", attempted, sent, err)
	}
	if got := decodeMessageType(t, <-client.send); got != "rawData" {
		t.Fatalf("delta message type = %q, want rawData", got)
	}
}

// A delta can outgrow the limit on its own. Sending it anyway would advance the
// server baseline past state the client dropped, desynchronising every later delta.
func TestQueueStateDeltaWithholdsOversizedDelta(t *testing.T) {
	initial := stateWithResources(t, 60, 4*1024)
	client := &Client{send: make(chan []byte, 2)}

	// Establish the baseline while unrestricted, then advertise the limit, so the
	// oversized payload under test is the delta rather than the snapshot.
	if _, err := client.queueFullState("initialState", initial); err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	<-client.send
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	// Change every resource so the delta approaches the size of a full snapshot.
	current := stateWithResources(t, 60, 4*1024)
	for i := range current.Resources {
		current.Resources[i].PlatformData = json.RawMessage(`{"pad":"` + strings.Repeat("y", 4*1024) + `"}`)
	}
	currentSnapshot, _, err := buildClientStateSnapshot(current)
	if err != nil {
		t.Fatalf("buildClientStateSnapshot() error = %v", err)
	}

	_, attempted, sent, err := client.queueStateDelta(currentSnapshot)
	if err != nil || !attempted || !sent {
		t.Fatalf("queueStateDelta() attempted=%v sent=%v error=%v", attempted, sent, err)
	}
	if got := decodeMessageType(t, <-client.send); got != stateTooLargeMessageType {
		t.Fatalf("oversized delta queued %q, want %q", got, stateTooLargeMessageType)
	}
	if client.stateSnapshot != currentSnapshot {
		t.Fatal("baseline did not advance to the state the client will hydrate over REST")
	}
}

func TestApplyAdvertisedInboundLimit(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "records a plausible limit", raw: "8388608", want: 8 * 1024 * 1024},
		{name: "tolerates surrounding whitespace", raw: "  8388608 ", want: 8 * 1024 * 1024},
		{name: "ignores an absent advertisement", raw: "", want: 0},
		{name: "ignores a limit below the floor", raw: "1024", want: 0},
		{name: "ignores a zero limit", raw: "0", want: 0},
		{name: "ignores a negative limit", raw: "-1", want: 0},
		{name: "ignores a non-numeric value", raw: "lots", want: 0},
		{name: "ignores an overflowing value", raw: "99999999999999999999999", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			client.applyAdvertisedInboundLimit(tt.raw)
			if got := client.inboundLimitBytes(); got != tt.want {
				t.Fatalf("inboundLimitBytes() = %d, want %d", got, tt.want)
			}
		})
	}
}

// When the marker cannot be queued the client was never told to hydrate, so the
// baseline must stay unset rather than stranding it on state it never receives.
func TestWithheldSnapshotLeavesBaselineUnsetWhenMarkerCannotQueue(t *testing.T) {
	state := stateWithResources(t, 60, 4*1024)
	client := &Client{send: make(chan []byte)} // unbuffered: safeSend cannot succeed
	client.maxInboundBytes.Store(minAdvertisedInboundBytes)

	result, err := client.queueFullState("initialState", state)
	if err != nil {
		t.Fatalf("queueFullState() error = %v", err)
	}
	if result.sent {
		t.Fatal("queueFullState() reported a send on a channel that cannot accept one")
	}
	if client.stateSnapshot != nil {
		t.Fatal("baseline was established even though the hydrate marker never reached the client")
	}
}
