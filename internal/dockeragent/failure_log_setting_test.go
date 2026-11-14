package dockeragent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rs/zerolog"
)

// The report says whether the host's owner turned on failure logs, so Pulse
// can count how many owners did. The setting itself stays local: nothing the
// server sends can change it.
func TestReportCarriesTheLocalFailureLogsSetting(t *testing.T) {
	swap(t, &hostmetricsCollectWithDiskFilters, func(context.Context, []string, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})
	for _, enabled := range []bool{false, true} {
		agent := &Agent{
			cfg: Config{AgentID: "docker-agent-1", Interval: 30 * time.Second, CollectFailureLogs: enabled},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger: zerolog.Nop(),
		}
		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport() failed: %v", err)
		}
		if report.Agent.FailureLogsEnabled != enabled {
			t.Fatalf("setting %v reported as %v", enabled, report.Agent.FailureLogsEnabled)
		}
		encoded, err := json.Marshal(report.Agent)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(encoded), `"failureLogsEnabled":true`); got != enabled {
			t.Fatalf("setting %v encodes as %s", enabled, encoded)
		}
	}
}
