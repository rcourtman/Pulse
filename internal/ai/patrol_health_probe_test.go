package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// On a default agent Patrol cannot read a container's logs or probe output.
// What it can know about an unhealthy container is the reason code the agent
// reports, so the seed has to carry it wherever it names the container: the
// scoped table an alert-woken run reads, and the host line a full run reads.
func TestPatrolSeedSaysWhyAHealthCheckFails(t *testing.T) {
	probe := &models.DockerContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonConnectionRefused, ExitCode: 1, FailingStreak: 14}
	snapshot := models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "docker-host-1", Hostname: "docker01", DisplayName: "docker01", Status: "online",
			Containers: []models.DockerContainer{
				{ID: "c-web", Name: "paperless", State: "running", Health: "unhealthy", HealthProbe: probe},
				{ID: "c-old", Name: "legacy", State: "running", Health: "unhealthy"},
				{ID: "c-ok", Name: "wiki", State: "running", Health: "healthy"},
			},
		}},
	}
	state := newPatrolRuntimeState(snapshot)

	rows := map[string]string{}
	for _, row := range patrolAppContainerRows(state, nil) {
		rows[row.name] = row.healthProbe
	}
	if rows["paperless"] != "health check: connection refused, 14 in a row" || rows["legacy"] != "" || rows["wiki"] != "" {
		t.Fatalf("rows from the agent report = %v", rows)
	}

	// The unified view is the path a running server takes.
	view := func(id, containerID, name, health string, p *models.DockerContainerHealthProbe) *unifiedresources.DockerContainerView {
		v := unifiedresources.NewDockerContainerView(&unifiedresources.Resource{
			ID: id, Name: name, Type: unifiedresources.ResourceTypeAppContainer, Status: unifiedresources.StatusOnline,
			Docker: &unifiedresources.DockerData{HostSourceID: "docker-host-1", ContainerID: containerID, ContainerState: "running", Health: health, HealthProbe: p},
		})
		return &v
	}
	state.readState = &mockReadState{dockerCtrs: []*unifiedresources.DockerContainerView{
		view("app-container-web", "c-web", "paperless", "unhealthy", probe),
		view("app-container-old", "c-old", "legacy", "unhealthy", nil),
	}}
	rows = map[string]string{}
	for _, row := range patrolAppContainerRows(state, nil) {
		rows[row.name] = row.healthProbe
	}
	if rows["paperless"] != "health check: connection refused, 14 in a row" || rows["legacy"] != "" {
		t.Fatalf("rows through the unified view = %v", rows)
	}

	ps := NewPatrolService(nil, nil)
	scoped := map[string]bool{"app-container-web": true, "app-container-old": true}
	seed := ps.seedResourceInventoryState(state, scoped, PatrolConfig{AnalyzeDocker: true}, time.Now(), false, nil)
	if !strings.Contains(seed, "| unhealthy (health check: connection refused, 14 in a row) |") {
		t.Fatalf("the scoped table does not say why paperless is unhealthy:\n%s", seed)
	}
	if !strings.Contains(seed, "| legacy | app-container-old | running | unhealthy |") {
		t.Fatalf("a container with no reported reason should read as before:\n%s", seed)
	}
}

func TestPatrolUnhealthyContainerLine(t *testing.T) {
	if got := patrolUnhealthyContainerLine("docker01", "paperless", "unhealthy", "health check: HTTP 503, 3 in a row"); got != "docker01/paperless: health=unhealthy (health check: HTTP 503, 3 in a row)" {
		t.Fatalf("line = %q", got)
	}
	if got := patrolUnhealthyContainerLine("docker01", "legacy", "unhealthy", ""); got != "docker01/legacy: health=unhealthy" {
		t.Fatalf("line without a reason = %q", got)
	}
}
