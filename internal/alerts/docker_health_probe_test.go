package alerts

import (
	"fmt"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func dockerHealthAlertID(host models.DockerHost) string {
	return fmt.Sprintf("docker-container-health-%s", DockerContainerResourceID(host.ID, host.Containers[0].ID, ""))
}

func unhealthyHost(probe *models.DockerContainerHealthProbe) models.DockerHost {
	return models.DockerHost{
		ID:          "host-health",
		DisplayName: "Docker Host",
		Hostname:    "docker.local",
		Containers: []models.DockerContainer{{
			ID: "web-1", Name: "paperless", State: "running", Status: "Up 2 hours (unhealthy)", Health: "unhealthy", HealthProbe: probe,
		}},
	}
}

// "Health is unhealthy" sends the operator to the host to find out why. When
// the agent reports the reason, the alert says it. The start of the message is
// unchanged, because Patrol and notification rules match on it.
func TestDockerContainerHealthAlertSaysWhyTheCheckFails(t *testing.T) {
	m := newTestManager(t)
	host := unhealthyHost(&models.DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 3})
	m.CheckDockerHost(host)

	alert, ok := testLookupActiveAlert(t, m, dockerHealthAlertID(host))
	if !ok {
		t.Fatal("an unhealthy container raised no health alert")
	}
	const want = "Docker container 'paperless' health is unhealthy (health check: connection refused)"
	if alert.Message != want {
		t.Fatalf("message = %q, want %q", alert.Message, want)
	}
	if alert.Metadata["healthProbeReason"] != agentsdocker.HealthProbeReasonConnectionRefused {
		t.Fatalf("metadata = %v, want the reason code", alert.Metadata)
	}

	// The streak grows on every report. The text must not, or the alert would
	// change every thirty seconds.
	host.Containers[0].HealthProbe.FailingStreak = 40
	m.CheckDockerHost(host)
	alert, ok = testLookupActiveAlert(t, m, dockerHealthAlertID(host))
	if !ok || alert.Message != want {
		t.Fatalf("after a longer streak: found %t, message %q, want %q", ok, alert.Message, want)
	}
}

// An agent too old to report the reason leaves the alert exactly as it was.
func TestDockerContainerHealthAlertWithoutAReasonIsUnchanged(t *testing.T) {
	m := newTestManager(t)
	host := unhealthyHost(nil)
	m.CheckDockerHost(host)

	alert, ok := testLookupActiveAlert(t, m, dockerHealthAlertID(host))
	if !ok {
		t.Fatal("an unhealthy container raised no health alert")
	}
	if alert.Message != "Docker container 'paperless' health is unhealthy" {
		t.Fatalf("message = %q", alert.Message)
	}
	if _, has := alert.Metadata["healthProbeReason"]; has {
		t.Fatalf("metadata carries a reason nobody reported: %v", alert.Metadata)
	}
}
