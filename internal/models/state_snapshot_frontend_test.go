package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStateSnapshotToFrontend_UsesConcreteStateContract(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()

	frontend := StateSnapshot{
		Metrics: []Metric{
			{
				Timestamp: now,
				Type:      "cpu",
				ID:        "node-1",
				Values: map[string]interface{}{
					"usage": 42.5,
				},
			},
		},
		Performance: Performance{
			APICallDuration:  map[string]float64{"nodes": 12.5},
			LastPollDuration: 125.5,
			PollingStartTime: now,
			TotalAPICalls:    8,
			FailedAPICalls:   1,
		},
		ConnectionHealth:             map[string]bool{"node-1": true},
		Stats:                        Stats{StartTime: now, Uptime: 60, PollingCycles: 4, WebSocketClients: 2, Version: "v6.0.0"},
		RecentlyResolved:             []ResolvedAlert{},
		LastUpdate:                   now,
		TemperatureMonitoringEnabled: true,
		PVETagColors:                 map[string]string{"production": "#ff0000"},
		PVETagStyles: map[string]PVETagStyle{
			"pve-a": {
				Colors:        map[string]string{"Production": "#00ff00"},
				CaseSensitive: true,
			},
		},
	}.ToFrontend()

	if len(frontend.Metrics) != 1 || frontend.Metrics[0].ID != "node-1" {
		t.Fatalf("expected concrete metrics slice, got %#v", frontend.Metrics)
	}
	if frontend.Performance.TotalAPICalls != 8 {
		t.Fatalf("expected concrete performance payload, got %#v", frontend.Performance)
	}
	if frontend.Stats.Version != "v6.0.0" {
		t.Fatalf("expected concrete stats payload, got %#v", frontend.Stats)
	}
	if frontend.PVETagColors["production"] != "#ff0000" {
		t.Fatalf("expected PVE tag colors to survive frontend conversion, got %#v", frontend.PVETagColors)
	}
	if frontend.PVETagStyles["pve-a"].Colors["Production"] != "#00ff00" || !frontend.PVETagStyles["pve-a"].CaseSensitive {
		t.Fatalf("expected PVE tag styles to survive frontend conversion, got %#v", frontend.PVETagStyles)
	}

	encoded, err := json.Marshal(frontend)
	if err != nil {
		t.Fatalf("marshal frontend state: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if metrics, ok := payload["metrics"].([]any); !ok || len(metrics) != 1 {
		t.Fatalf("expected metrics to serialize as an array, got %T (%v)", payload["metrics"], payload["metrics"])
	}
	if performance, ok := payload["performance"].(map[string]any); !ok || performance["totalApiCalls"] != float64(8) {
		t.Fatalf("expected performance to serialize as an object, got %T (%v)", payload["performance"], payload["performance"])
	}
	if stats, ok := payload["stats"].(map[string]any); !ok || stats["version"] != "v6.0.0" {
		t.Fatalf("expected stats to serialize as an object, got %T (%v)", payload["stats"], payload["stats"])
	}
	if resources, ok := payload["resources"].([]any); !ok || len(resources) != 0 {
		t.Fatalf("expected resources to serialize as an empty array, got %T (%v)", payload["resources"], payload["resources"])
	}
	if connectedInfrastructure, ok := payload["connectedInfrastructure"].([]any); !ok || len(connectedInfrastructure) != 0 {
		t.Fatalf("expected connectedInfrastructure to serialize as an empty array, got %T (%v)", payload["connectedInfrastructure"], payload["connectedInfrastructure"])
	}
	if tagColors, ok := payload["pveTagColors"].(map[string]any); !ok || tagColors["production"] != "#ff0000" {
		t.Fatalf("expected pveTagColors to serialize as an object, got %T (%v)", payload["pveTagColors"], payload["pveTagColors"])
	}
	if tagStyles, ok := payload["pveTagStyles"].(map[string]any); !ok {
		t.Fatalf("expected pveTagStyles to serialize as an object, got %T (%v)", payload["pveTagStyles"], payload["pveTagStyles"])
	} else if pveA, ok := tagStyles["pve-a"].(map[string]any); !ok || pveA["caseSensitive"] != true {
		t.Fatalf("expected pve-a tag style to serialize with caseSensitive=true, got %T (%v)", tagStyles["pve-a"], tagStyles["pve-a"])
	}
}

func TestEmptyStateFrontend_UsesCanonicalCollectionShapes(t *testing.T) {
	frontend := EmptyStateFrontend()

	encoded, err := json.Marshal(frontend)
	if err != nil {
		t.Fatalf("marshal frontend state: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	for _, key := range []string{"activeAlerts", "recentlyResolved", "metrics", "resources", "connectedInfrastructure"} {
		values, ok := payload[key].([]any)
		if !ok {
			t.Fatalf("expected %s to serialize as an array, got %T (%v)", key, payload[key], payload[key])
		}
		if len(values) != 0 {
			t.Fatalf("expected %s to serialize as an empty array, got %v", key, values)
		}
	}

	if connectionHealth, ok := payload["connectionHealth"].(map[string]any); !ok || len(connectionHealth) != 0 {
		t.Fatalf("expected connectionHealth to serialize as an empty object, got %T (%v)", payload["connectionHealth"], payload["connectionHealth"])
	}

	performance, ok := payload["performance"].(map[string]any)
	if !ok {
		t.Fatalf("expected performance to serialize as an object, got %T (%v)", payload["performance"], payload["performance"])
	}
	if apiCallDuration, ok := performance["apiCallDuration"].(map[string]any); !ok || len(apiCallDuration) != 0 {
		t.Fatalf("expected performance.apiCallDuration to serialize as an empty object, got %T (%v)", performance["apiCallDuration"], performance["apiCallDuration"])
	}
}
