package models

import (
	"testing"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestDockerContainerHealthProbeWording(t *testing.T) {
	for _, tc := range []struct {
		probe             *DockerContainerHealthProbe
		describe, summary string
	}{
		{nil, "", ""},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 14}, "connection refused", "health check: connection refused, 14 in a row"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22, FailingStreak: 1, HTTPStatus: 503}, "HTTP 503", "health check: HTTP 503"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22}, "HTTP error", "health check: HTTP error"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonTimeout, ExitCode: -1, FailingStreak: 5}, "timed out", "health check: timed out, 5 in a row"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonNameNotResolved, ExitCode: 6}, "name not resolved", "health check: name not resolved"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonTLS, ExitCode: 60}, "TLS error", "health check: TLS error"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonCommandMissing, ExitCode: 127}, "command not found", "health check: command not found"},
		{&DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonOther, ExitCode: 2, FailingStreak: 3}, "exit code 2", "health check: exit code 2, 3 in a row"},
		{&DockerContainerHealthProbe{Reason: "not a code", ExitCode: 7}, "exit code 7", "health check: exit code 7"},
	} {
		if got := tc.probe.Describe(); got != tc.describe {
			t.Errorf("Describe(%+v) = %q, want %q", tc.probe, got, tc.describe)
		}
		if got := tc.probe.Summary(); got != tc.summary {
			t.Errorf("Summary(%+v) = %q, want %q", tc.probe, got, tc.summary)
		}
	}
}

// A copy of a container must not share the probe with the original: the state
// snapshot is read while the monitor writes the next one.
func TestCloneDockerContainerCopiesTheHealthProbe(t *testing.T) {
	original := DockerContainer{ID: "c1", Health: "unhealthy",
		HealthProbe: &DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 2}}
	clone := cloneDockerContainer(original)
	if clone.HealthProbe == nil || clone.HealthProbe == original.HealthProbe || *clone.HealthProbe != *original.HealthProbe {
		t.Fatalf("clone probe = %p %+v, original %p %+v", clone.HealthProbe, clone.HealthProbe, original.HealthProbe, original.HealthProbe)
	}
	clone.HealthProbe.FailingStreak = 99
	if original.HealthProbe.FailingStreak != 2 {
		t.Fatal("changing the copy changed the original")
	}
}
