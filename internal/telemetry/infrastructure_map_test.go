package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

// Schema v24 sends how many browser sessions opened the infrastructure map
// and how many saved it as an image, with the saved view and whether names
// were hidden. Counts only: nothing the map draws.
func TestPingCarriesInfrastructureMapCounts(t *testing.T) {
	if TelemetrySchemaVersion < 24 {
		t.Fatalf("TelemetrySchemaVersion = %d, want at least 24", TelemetrySchemaVersion)
	}
	ping := applySnapshot(Ping{InstallID: "test-id"}, func() Snapshot {
		return Snapshot{
			InfrastructureMapOpenSessions30d:              9,
			InfrastructureMapExportSessions30d:            5,
			InfrastructureMapExportServicesSessions30d:    4,
			InfrastructureMapExportConnectionsSessions30d: 3,
			InfrastructureMapExportHostsSessions30d:       2,
			InfrastructureMapExportNamesHiddenSessions30d: 1,
		}
	})
	encoded, err := json.Marshal(ping)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"infrastructure_map_open_sessions_30d":9`,
		`"infrastructure_map_export_sessions_30d":5`,
		`"infrastructure_map_export_services_sessions_30d":4`,
		`"infrastructure_map_export_connections_sessions_30d":3`,
		`"infrastructure_map_export_hosts_sessions_30d":2`,
		`"infrastructure_map_export_names_hidden_sessions_30d":1`,
	} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("ping JSON lacks %s: %s", key, encoded)
		}
	}
}

// An install that never opened the map still reports the fields, as zeros, so
// the receiver can tell "not used" from "sender too old to say".
func TestPingReportsUnusedInfrastructureMapAsZero(t *testing.T) {
	encoded, err := json.Marshal(applySnapshot(Ping{InstallID: "test-id"}, func() Snapshot { return Snapshot{} }))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"infrastructure_map_open_sessions_30d":0`,
		`"infrastructure_map_export_sessions_30d":0`,
	} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("ping JSON lacks %s: %s", key, encoded)
		}
	}
}
