package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestDockerRuntimeContainerMute(t *testing.T) {
	m := newTestManager(t)
	a := models.DockerHost{ID: "lab", Hostname: "docker", AgentID: "source-a"}
	b := models.DockerHost{ID: "lab::2", Hostname: "docker", AgentID: "source-b"}
	c := models.DockerContainer{ID: "web", Name: "web", Image: "web:1", UpdateStatus: &models.DockerContainerUpdateStatus{UpdateAvailable: true, CurrentDigest: "old", LatestDigest: "new"}}
	run := func(h models.DockerHost) {
		id := DockerContainerResourceID(h.ID, c.ID, c.Name)
		m.checkDockerContainerImageUpdate(h, c, id, c.Name, h.Hostname, h.Hostname)
		m.checkDockerContainerRestartLoop(h, c, id, c.Name, h.Hostname, h.Hostname)
	}
	cfg := m.GetConfig()
	cfg.DockerDefaults.UpdateAlertDelayHours = 24
	cfg.DockerDefaults.RestartCount = 3
	cfg.DockerDefaults.RestartWindow = 10
	m.UpdateConfig(cfg)
	run(a)
	run(b)
	aID := DockerContainerResourceID(a.ID, c.ID, c.Name)
	bID := DockerContainerResourceID(b.ID, c.ID, c.Name)
	first := time.Now().Add(-48 * time.Hour)
	_, _, _ = seedDockerUpdateAlert(t, m, a, c, aID, first)
	cfg.DockerMutedContainerHosts = map[string]bool{a.ID: true}
	m.UpdateConfig(cfg)
	run(a)
	if !m.dockerUpdateFirstSeen[aID].IsZero() || !m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(a, c)].IsZero() || m.dockerRestartTracking[aID] != nil {
		t.Fatal("muted runtime retained container detector state")
	}
	if m.dockerUpdateFirstSeen[bID].IsZero() || m.dockerRestartTracking[bID] == nil {
		t.Fatal("other runtime lost detector state")
	}
	for _, id := range []string{DockerHostResourceID(a.ID), DockerServiceResourceID(a.ID, "web", "web"), bID, "agent:lab", "availability:lab"} {
		if muted, _ := m.operatorSuppressionForAlertNoLock(&Alert{ResourceID: id}, time.Now()); muted {
			t.Errorf("unexpected mute for %q", id)
		}
	}
	if muted, _ := m.operatorSuppressionForAlertNoLock(&Alert{ResourceID: DockerContainerResourceID(a.ID, "future", "")}, time.Now()); !muted {
		t.Fatal("new container missed runtime mute")
	}
	for _, alert := range m.GetActiveAlerts() {
		if alert.ResourceID == aID {
			t.Fatal("configuration left muted alert active")
		}
	}
	cfg.DockerMutedContainerHosts = nil
	m.UpdateConfig(cfg)
	run(a)
	if m.dockerUpdateFirstSeen[aID].Before(time.Now().Add(-time.Minute)) {
		t.Fatal("unmute did not start a new delay")
	}
}
