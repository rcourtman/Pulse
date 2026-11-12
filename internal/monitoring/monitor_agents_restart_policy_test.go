package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// The container state alert reads the restart policy from the model, so a
// report's policy has to reach it in the form the alert compares against.
// A report without one (an older agent) leaves the model's field empty.
func TestApplyDockerReportCarriesContainerRestartPolicy(t *testing.T) {
	monitor := newTestMonitor(t)
	report := supersedeTestReport("agent-policy", "6.9.0", time.Now().UTC())
	report.Containers = []agentsdocker.Container{
		{ID: "c-job", Name: "db-migrate", State: "exited", Status: "Exited (0) 2 minutes ago", RestartPolicy: " On-Failure "},
		{ID: "c-svc", Name: "db", State: "running", Status: "Up 2 minutes", RestartPolicy: "unless-stopped"},
		{ID: "c-old", Name: "legacy", State: "exited", Status: "Exited (0) 2 minutes ago"},
	}

	host, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-policy", CreatedAt: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}

	want := map[string]string{"db-migrate": "on-failure", "db": "unless-stopped", "legacy": ""}
	if len(host.Containers) != len(want) {
		t.Fatalf("got %d containers, want %d", len(host.Containers), len(want))
	}
	for _, container := range host.Containers {
		policy, ok := want[container.Name]
		if !ok {
			t.Fatalf("unexpected container %q", container.Name)
		}
		if container.RestartPolicy != policy {
			t.Fatalf("container %q restart policy = %q, want %q", container.Name, container.RestartPolicy, policy)
		}
	}
}
