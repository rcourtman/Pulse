package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// Telemetry counts how many agents have each local opt-in on, so a change
// that needs one can be judged by how many owners it reaches. Only the counts
// leave the install.
func TestInstallSnapshotCountsAgentOptIns(t *testing.T) {
	agent := func(commands bool) *unifiedresources.HostView {
		view := unifiedresources.NewHostView(&unifiedresources.Resource{Agent: &unifiedresources.AgentData{CommandsEnabled: commands}})
		return &view
	}
	docker := func(failureLogs bool) *unifiedresources.DockerHostView {
		view := unifiedresources.NewDockerHostView(&unifiedresources.Resource{Docker: &unifiedresources.DockerData{FailureLogsEnabled: failureLogs}})
		return &view
	}

	hosts := []*unifiedresources.HostView{agent(true), agent(false), agent(true), nil}
	if got := countAgentHostsWithCommands(hosts); got != 2 {
		t.Fatalf("agent hosts with commands on = %d, want 2", got)
	}
	dockerHosts := []*unifiedresources.DockerHostView{docker(false), docker(true), nil}
	if got := countDockerHostsWithFailureLogs(dockerHosts); got != 1 {
		t.Fatalf("Docker hosts with failure logs on = %d, want 1", got)
	}
	if countAgentHostsWithCommands(nil) != 0 || countDockerHostsWithFailureLogs(nil) != 0 {
		t.Fatal("an install with no agents counted an opt-in")
	}
}

// The Docker host keeps the setting its agent reported.
func TestDockerReportCarriesTheFailureLogsSetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		monitor := newTestMonitor(t)
		complete := true
		host, err := monitor.ApplyDockerReport(agentsdocker.Report{
			Agent: agentsdocker.AgentInfo{ID: "docker-agent-1", Version: "6.9.0", Type: "unified", IntervalSeconds: 30, FailureLogsEnabled: enabled},
			Host: agentsdocker.HostInfo{
				Hostname: "docker01", Name: "docker01", MachineID: "machine-docker01", Runtime: "docker",
			},
			InventoryComplete: &complete,
			Timestamp:         time.Now().UTC(),
		}, nil)
		if err != nil {
			t.Fatalf("apply report: %v", err)
		}
		if host.FailureLogsEnabled != enabled {
			t.Fatalf("setting %v stored as %v", enabled, host.FailureLogsEnabled)
		}
	}
}
