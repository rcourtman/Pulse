package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// The probe reason is shown to operators and given to Patrol. The server keeps
// it only for a container the same report calls unhealthy, and only as one of
// the closed codes: whatever an agent sends, no free text arrives there.
func TestApplyDockerReportCarriesTheHealthProbeReason(t *testing.T) {
	monitor := newTestMonitor(t)
	report := supersedeTestReport("agent-probe", "6.9.0", time.Now().UTC())
	report.Containers = []agentsdocker.Container{
		{ID: "c-refused", Name: "paperless", State: "running", Health: "unhealthy",
			HealthProbe: &agentsdocker.ContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 14}},
		{ID: "c-http", Name: "wiki", State: "running", Health: "Unhealthy",
			HealthProbe: &agentsdocker.ContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22, FailingStreak: 3, HTTPStatus: 503}},
		{ID: "c-freetext", Name: "odd", State: "running", Health: "unhealthy",
			HealthProbe: &agentsdocker.ContainerHealthProbe{Reason: "token=abc123 leaked", ExitCode: 9000, FailingStreak: -4, HTTPStatus: 70000}},
		{ID: "c-stale", Name: "recovered", State: "running", Health: "healthy",
			HealthProbe: &agentsdocker.ContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonTimeout, ExitCode: 1}},
		{ID: "c-old-agent", Name: "legacy", State: "running", Health: "unhealthy"},
	}

	host, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-probe", CreatedAt: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}

	want := map[string]*models.DockerContainerHealthProbe{
		"paperless": {Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 14},
		"wiki":      {Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22, FailingStreak: 3, HTTPStatus: 503},
		"odd":       {Reason: agentsdocker.HealthProbeReasonOther, ExitCode: -1},
		"recovered": nil,
		"legacy":    nil,
	}
	if len(host.Containers) != len(want) {
		t.Fatalf("got %d containers, want %d", len(host.Containers), len(want))
	}
	for _, container := range host.Containers {
		probe, ok := want[container.Name]
		if !ok {
			t.Fatalf("unexpected container %q", container.Name)
		}
		switch {
		case probe == nil && container.HealthProbe != nil:
			t.Fatalf("container %q kept a probe %+v, want none", container.Name, container.HealthProbe)
		case probe != nil && (container.HealthProbe == nil || *container.HealthProbe != *probe):
			t.Fatalf("container %q probe = %+v, want %+v", container.Name, container.HealthProbe, probe)
		}
	}
}
