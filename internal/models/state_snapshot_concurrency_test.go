package models

import "testing"

func TestGetSnapshotDeepCopiesNestedMutableFields(t *testing.T) {
	state := NewState()

	state.UpdateNodes([]Node{{
		ID:          "node-1",
		Name:        "node-1",
		LoadAverage: []float64{1.0, 2.0, 3.0},
		Temperature: &Temperature{
			Available: true,
			Cores:     []CoreTemp{{Core: 0, Temp: 55.5}},
		},
	}})

	state.UpsertHost(Host{
		ID:       "host-1",
		Hostname: "host-1",
		Sensors: HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 44.0},
		},
	})

	snapshot := state.GetSnapshot()
	snapshot.Nodes[0].LoadAverage[0] = 99
	snapshot.Nodes[0].Temperature.Cores[0].Temp = 99
	snapshot.Hosts[0].Sensors.TemperatureCelsius["cpu"] = 99

	fresh := state.GetSnapshot()
	if got := fresh.Nodes[0].LoadAverage[0]; got != 1.0 {
		t.Fatalf("expected node load average to remain 1.0, got %v", got)
	}
	if got := fresh.Nodes[0].Temperature.Cores[0].Temp; got != 55.5 {
		t.Fatalf("expected node core temp to remain 55.5, got %v", got)
	}
	if got := fresh.Hosts[0].Sensors.TemperatureCelsius["cpu"]; got != 44.0 {
		t.Fatalf("expected host sensor temp to remain 44.0, got %v", got)
	}
}

func TestUpdateNodesCopiesInputData(t *testing.T) {
	state := NewState()

	nodes := []Node{{
		ID:          "node-1",
		Name:        "node-1",
		LoadAverage: []float64{1.0},
		Temperature: &Temperature{Available: true, CPUPackage: 52.0},
	}}

	state.UpdateNodes(nodes)

	// Mutate caller-owned data after update; state must stay unchanged.
	nodes[0].Name = "mutated"
	nodes[0].LoadAverage[0] = 99
	nodes[0].Temperature.CPUPackage = 99

	snapshot := state.GetSnapshot()
	if got := snapshot.Nodes[0].Name; got != "node-1" {
		t.Fatalf("expected stored node name to remain node-1, got %q", got)
	}
	if got := snapshot.Nodes[0].LoadAverage[0]; got != 1.0 {
		t.Fatalf("expected stored load average to remain 1.0, got %v", got)
	}
	if got := snapshot.Nodes[0].Temperature.CPUPackage; got != 52.0 {
		t.Fatalf("expected stored CPU package temp to remain 52.0, got %v", got)
	}
}
