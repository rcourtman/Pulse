package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

// Schema v22 sends how many agent hosts have commands on and how many Docker
// hosts have failure logs on. Counts only: no host names or identifiers.
func TestPingCarriesAgentOptInCounts(t *testing.T) {
	if TelemetrySchemaVersion < 22 {
		t.Fatalf("TelemetrySchemaVersion = %d, want at least 22", TelemetrySchemaVersion)
	}
	ping := applySnapshot(Ping{InstallID: "test-id"}, func() Snapshot {
		return Snapshot{AgentHosts: 4, AgentHostsCommandsEnabled: 1, DockerHosts: 2, DockerHostsFailureLogsEnabled: 1}
	})
	if ping.AgentHostsCommandsEnabled != 1 || ping.DockerHostsFailureLogsEnabled != 1 {
		t.Fatalf("ping counts = %d commands-on, %d failure-logs; want 1 and 1", ping.AgentHostsCommandsEnabled, ping.DockerHostsFailureLogsEnabled)
	}
	encoded, err := json.Marshal(ping)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"agent_hosts_commands_enabled":1`, `"docker_hosts_failure_logs_enabled":1`} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("ping JSON lacks %s: %s", key, encoded)
		}
	}
}
