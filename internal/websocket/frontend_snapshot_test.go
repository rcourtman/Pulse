package websocket

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func assertFrontendSnapshotEquivalent(t testing.TB, state models.StateFrontend) {
	t.Helper()
	want, wantErr := buildGenericClientStateSnapshot(state)
	for _, value := range []interface{}{state, &state} {
		got, gotErr := buildClientStateSnapshot(value)
		if (wantErr == nil) != (gotErr == nil) {
			t.Fatalf("%T error differs: generic=%v typed=%v", value, wantErr, gotErr)
		}
		if wantErr == nil && !reflect.DeepEqual(got, want) {
			t.Fatalf("%T snapshot differs from full wire encoding", value)
		}
	}
}

func TestFrontendSnapshotMatchesFullWireEncoding(t *testing.T) {
	state := benchmarkFrontendState(20)
	state.Resources[19].ID = "尾<&\"\\\n"
	state.Resources[19].PlatformData = json.RawMessage(`{"id":"not-the-key","extra":[1,true,null]}`)
	state.ConnectedInfrastructure = []models.ConnectedInfrastructureItemFrontend{{ID: "i-1"}, {ID: "i-2"}}
	state.ActiveAlerts = []models.Alert{{ID: "a-1", Level: "critical"}, {ID: "a-2"}}
	state.CapabilityCatalog = map[string]json.RawMessage{"ref": json.RawMessage(`{"allowed":true}`)}
	state.PolicyCatalog = map[string]json.RawMessage{"ref": json.RawMessage(`{"denied":true}`)}
	state.AISafeSummaryCatalog = map[string]string{"ref": "summary<&>"}
	state.ConnectionHealth = map[string]bool{"lab": false}
	state.PVETagColors = map[string]string{"tag": "#abcdef"}
	state.LastUpdate = 1234
	for name, value := range map[string]models.StateFrontend{
		"populated": state, "zero": {}, "empty": models.EmptyStateFrontend(),
	} {
		t.Run(name, func(t *testing.T) { assertFrontendSnapshotEquivalent(t, value) })
	}
	// Missing and duplicate IDs, bad raw data and unencodable metrics must fail
	// rather than quietly adopt a different resource baseline.
	state.Resources[1].ID = state.Resources[0].ID
	assertFrontendSnapshotEquivalent(t, state)
	state.Resources[1].ID = ""
	assertFrontendSnapshotEquivalent(t, state)
	state.Resources[1].ID = "fixed"
	state.Resources[0].CPU.Current = math.NaN()
	assertFrontendSnapshotEquivalent(t, state)
	state.Resources[0].CPU.Current = 0
	state.Resources[19].PlatformData = json.RawMessage(`{"invalid":`)
	assertFrontendSnapshotEquivalent(t, state)
}

func TestFrontendSnapshotOwnsMutableSource(t *testing.T) {
	state := benchmarkFrontendState(2)
	state.Resources[0].Labels = map[string]string{"label": "original"}
	state.Resources[0].Tags = []string{"original"}
	state.Resources[0].PlatformData = json.RawMessage(`{"value":"original"}`)
	snapshot, err := buildClientStateSnapshot(state)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(snapshot.resources)
	if err != nil {
		t.Fatal(err)
	}
	state.Resources[0].ID = "different"
	state.Resources[0].CPU.Current++
	state.Resources[0].Labels["label"] = "changed"
	state.Resources[0].Tags[0] = "changed"
	state.Resources[0].PlatformData[10] = 'X'
	after, err := json.Marshal(snapshot.resources)
	if err != nil || string(before) != string(after) {
		t.Fatal("snapshot aliases mutable source")
	}
}

func FuzzFrontendSnapshotMatchesFullWireEncoding(f *testing.F) {
	f.Add("id-1", "payload<&>", false, false)
	f.Add("尾\n\\\"", "ID", true, true)
	f.Add("", "", true, false)
	f.Fuzz(func(t *testing.T, id, payload string, duplicate, nilCollections bool) {
		state := benchmarkFrontendState(3)
		state.Resources[2].ID = id
		state.Resources[2].Name = payload
		if duplicate {
			state.Resources[1].ID = id
		}
		if nilCollections {
			state.ActiveAlerts = nil
			state.ConnectedInfrastructure = nil
			state.Metrics = nil
			state.ConnectionHealth = nil
		}
		assertFrontendSnapshotEquivalent(t, state)
	})
}
