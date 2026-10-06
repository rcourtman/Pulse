package websocket

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestDashboardDispatchLifecycleEquivalence(t *testing.T) {
	state := benchmarkFrontendState(3)
	state.Resources[0].Labels = map[string]string{"team": "ops"}
	state.Resources[0].PlatformData = json.RawMessage(`{"counter":9007199254740993,"inventory":["a","b"]}`)
	state.Resources[0].Health = json.RawMessage(`{"status":"healthy","age":0}`)
	state.Resources[0].CapabilitiesRef = "cap"
	state.CapabilityCatalog = map[string]json.RawMessage{"cap": json.RawMessage(`{"read":true}`)}
	state.PolicyCatalog = map[string]json.RawMessage{"policy": json.RawMessage(`{"write":false}`)}
	state.AISafeSummaryCatalog = map[string]string{"summary": "initial"}
	state.ConnectedInfrastructure = []models.ConnectedInfrastructureItemFrontend{{ID: "infra", LastSeen: 1, Status: "online"}}
	state.ActiveAlerts = []models.Alert{{ID: "alert", ResourceID: state.Resources[0].ID, Level: "warning", Message: "initial"}}
	getter := func(string) interface{} { return state }
	hub := NewHub(getter)
	clients := []*Client{{orgID: "own", send: make(chan []byte, 1)}, {orgID: "own", send: make(chan []byte, 1)}, {orgID: "other", send: make(chan []byte, 1)}}
	for _, client := range clients {
		if _, sent, err := client.queueFullState("initialState", state); err != nil || !sent {
			t.Fatal(err)
		}
		<-client.send
		hub.clients[client] = true
		if hub.clientsByTenant[client.orgID] == nil {
			hub.clientsByTenant[client.orgID] = map[*Client]bool{}
		}
		hub.clientsByTenant[client.orgID][client] = true
	}
	otherBaseline := clients[2].stateSnapshot
	for phase := 0; phase < 11; phase++ {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			before := clients[0].stateSnapshot
			switch phase {
			case 0:
				state.LastUpdate = 9007199254740993
				for i := range state.Resources {
					state.Resources[i].LastSeen = 9007199254740993
				}
				state.ConnectedInfrastructure[0].LastSeen = 9007199254740993
			case 1:
				state.Resources[0].CPU.Current = 32.25
				state.Resources[0].Network = &models.ResourceNetworkFrontend{RXBytes: 9007199254740993}
				state.Resources[0].PlatformData = json.RawMessage(`{"counter":18446744073709551615,"inventory":["c"]}`)
			case 2:
				state.Resources[0].Health = json.RawMessage(`{"status":"unhealthy","age":70}`)
				state.ActiveAlerts[0].Level = "critical"
				// Resource alerts have one canonical owner: ActiveAlerts, not
				// an embedded resource copy. Preserve both arrival and update.
				state.ActiveAlerts = append(state.ActiveAlerts, models.Alert{ID: "live", ResourceID: state.Resources[0].ID, Level: "critical"})
			case 3:
				state.Resources[0].Labels = map[string]string{"team": "changed"}
				state.Resources[0].Tags = []string{"new"}
				state.Resources[0].CustomURL = "https://example.invalid/updated"
				state.Resources[0].ParentID = "changed-owner"
				state.Resources[0].DisplayName = "changed"
				state.Resources[0], state.Resources[1] = state.Resources[1], state.Resources[0]
			case 4:
				state.CapabilityCatalog["cap"] = json.RawMessage(`{"read":false}`)
				state.PolicyCatalog = nil
				state.AISafeSummaryCatalog["summary"] = "changed"
				state.ConnectionHealth = map[string]bool{"source": false}
			case 5:
				state.Resources = state.Resources[:1]
				state.Resources[0].ID = "re-enrolled-identity"
				state.ConnectedInfrastructure = nil
				state.ActiveAlerts = nil
			case 6:
				state.Resources[0].Health = nil
				state.Resources[0].PlatformData = nil
				state.ConnectionHealth = nil
			case 7: // Alert-only changes must reach both connected clients.
				state.ActiveAlerts = []models.Alert{{ID: "recurrence", ResourceID: state.Resources[0].ID, Level: "warning", Message: "arrived", Value: 85}}
			case 8:
				state.ActiveAlerts[0].Level, state.ActiveAlerts[0].Value = "critical", 97
				state.ActiveAlerts[0].Message = "changed"
			case 9:
				state.ActiveAlerts = nil
			case 10: // unchanged evidence must not manufacture a heartbeat
			}
			current, err := buildGenericClientStateSnapshot(state)
			if err != nil {
				t.Fatal(err)
			}
			want, err := referenceBuildClientStateDelta(before, current)
			if err != nil {
				t.Fatal(err)
			}
			// Pin the alert oracle independently of reference/wire equality:
			// both encoders dropping alerts must not make this fixture pass.
			if phase == 2 || phase == 5 || phase >= 7 && phase <= 9 {
				payload, ok := want[activeAlertsDeltaField].(resourceDeltaPayload)
				if !ok {
					t.Fatalf("phase %d lost the active-alert delta: %#v", phase, want)
				}
				switch phase {
				case 2:
					if len(payload.Upserts) != 2 || len(payload.Removed) != 0 {
						t.Fatalf("alert update/insertion delta = %#v", payload)
					}
				case 5:
					if len(payload.Upserts) != 0 || len(payload.Removed) != 2 || !slices.Contains(payload.Removed, "alert") || !slices.Contains(payload.Removed, "live") {
						t.Fatalf("alert removal delta = %#v", payload)
					}
				case 7, 8:
					if len(payload.Upserts) != 1 || len(payload.Removed) != 0 {
						t.Fatalf("alert-only upsert delta = %#v", payload)
					}
					var patch models.Alert
					if err := json.Unmarshal(payload.Upserts[0], &patch); err != nil {
						t.Fatal(err)
					}
					if patch.ID != "recurrence" || patch.Level != state.ActiveAlerts[0].Level || patch.Value != state.ActiveAlerts[0].Value || patch.Message != state.ActiveAlerts[0].Message || phase == 7 && patch.ResourceID != state.Resources[0].ID {
						t.Fatalf("alert-only patch lost occurrence, target, message or changed values: %s", payload.Upserts[0])
					}
				case 9:
					if len(payload.Upserts) != 0 || !reflect.DeepEqual(payload.Removed, []string{"recurrence"}) {
						t.Fatalf("alert-only removal delta = %#v", payload)
					}
				}
				if phase >= 7 && len(want) != 1 {
					t.Fatalf("alert-only change altered another state field: %#v", want)
				}
			}
			encoded, err := json.Marshal(Message{Type: "rawData", Data: want})
			if err != nil {
				t.Fatal(err)
			}
			hub.dispatchStateBroadcast(&Message{Type: "rawData", Data: stateBroadcastRequest{orgID: "own"}}, "own")
			for _, client := range clients[:2] {
				select {
				case frame := <-client.send:
					if len(want) == 0 || !reflect.DeepEqual(decodeExactDeltaJSON(t, frame), decodeExactDeltaJSON(t, encoded)) {
						t.Fatalf("reference/wire mismatch: want=%s got=%s", encoded, frame)
					}
				default:
					if len(want) != 0 {
						t.Fatalf("missing live frame: want=%s", encoded)
					}
				}
			}
			if clients[0].stateSnapshot != clients[1].stateSnapshot {
				t.Fatal("accepted clients diverged baselines")
			}
			select {
			case frame := <-clients[2].send:
				t.Fatalf("cross-tenant data: %s", frame)
			default:
			}
			if clients[2].stateSnapshot != otherBaseline {
				t.Fatal("unrelated tenant baseline changed")
			}
		})
	}
}
