package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A cached registry observation is still an asserted condition on each report.
// Its first detection time must not be mistaken for tracking inactivity.
func TestDockerUpdateTrackingSurvivesDailyCleanup(t *testing.T) {
	for _, delay := range []int{24, 48} {
		t.Run((time.Duration(delay) * time.Hour).String(), func(t *testing.T) {
			m := newTestManager(t)
			m.config.DockerDefaults.UpdateAlertDelayHours = delay
			host := models.DockerHost{ID: "host", DisplayName: "host"}
			container := models.DockerContainer{ID: "container", Name: "web", Image: "mongo:7", UpdateStatus: &models.DockerContainerUpdateStatus{UpdateAvailable: true, CurrentDigest: "sha256:old", LatestDigest: "sha256:new", LastChecked: time.Now().Add(-6 * time.Hour)}}
			resource := "docker:host/container"
			key := dockerUpdateTrackingKey(host, container)
			first := time.Now().Add(-25 * time.Hour)
			m.dockerUpdateFirstSeen[resource] = first
			m.dockerUpdateFirstSeenByIdentity[key] = first
			check := func() { m.checkDockerContainerImageUpdate(host, container, resource, "web", "host", "host") }
			check()
			m.cleanupStaleMaps()
			if !m.dockerUpdateFirstSeen[resource].Equal(first) || !m.dockerUpdateFirstSeenByIdentity[key].Equal(first) {
				t.Fatal("daily cleanup discarded an observed pending update's first detection")
			}
			check()
			if delay == 24 {
				alerts := m.GetActiveAlerts()
				if len(alerts) != 1 || !alerts[0].StartTime.Equal(first) {
					t.Fatalf("continuing update lost its original incident: %+v", alerts)
				}
			}
			// Unknown reports must neither resolve nor restart a pending condition.
			container.UpdateStatus = nil
			check()
			m.cleanupStaleMaps()
			if !m.dockerUpdateFirstSeen[resource].Equal(first) {
				t.Fatal("missing status reset pending age")
			}
			container.UpdateStatus = &models.DockerContainerUpdateStatus{Error: "registry unavailable"}
			check()
			m.cleanupStaleMaps()
			if !m.dockerUpdateFirstSeenByIdentity[key].Equal(first) {
				t.Fatal("failed check reset pending age")
			}
			// An affirmative clear remains effective.
			container.UpdateStatus = &models.DockerContainerUpdateStatus{UpdateAvailable: false, CurrentDigest: "sha256:new", LatestDigest: "sha256:new", LastChecked: time.Now()}
			check()
			if len(m.GetActiveAlerts()) != 0 || len(m.dockerUpdateFirstSeen) != 0 || len(m.dockerUpdateFirstSeenByIdentity) != 0 || len(m.dockerUpdateLastObserved) != 0 {
				t.Fatal("affirmative clear did not retire update")
			}
		})
	}
}

func TestDockerUpdateTrackingExpiresOnlyAfterObservationStops(t *testing.T) {
	m := newTestManager(t)
	old := time.Now().Add(-25 * time.Hour)
	for _, key := range []string{"docker:host/container", "docker-update:host/id:container"} {
		m.dockerUpdateLastObserved[key] = old
	}
	m.dockerUpdateFirstSeen["docker:host/container"] = old.Add(-48 * time.Hour)
	m.dockerUpdateFirstSeenByIdentity["docker-update:host/id:container"] = old.Add(-48 * time.Hour)
	m.cleanupStaleMaps()
	if len(m.dockerUpdateFirstSeen) != 0 || len(m.dockerUpdateFirstSeenByIdentity) != 0 || len(m.dockerUpdateLastObserved) != 0 {
		t.Fatal("unobserved tracking was not reclaimed")
	}
}
