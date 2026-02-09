package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Helper to check if an alert exists with the given ID prefix
func hasAlertWithPrefix(alerts []Alert, prefix string) bool {
	for _, a := range alerts {
		if len(a.ID) >= len(prefix) && a.ID[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestCheckDockerContainerImageUpdate(t *testing.T) {
	// Create a manager for testing
	m := NewManager()

	// Set the default delay hours - NewManager doesn't set this,
	// it's normally set during config loading
	m.mu.Lock()
	m.config.DockerDefaults.UpdateAlertDelayHours = 24 // Default: alert after 24 hours
	m.mu.Unlock()

	hostID := "docker-host-1"
	containerID := "container-abc123"
	resourceID := "docker:" + hostID + "/" + containerID
	containerName := "test-container"
	instanceName := "docker-instance"
	nodeName := "docker-node"

	host := models.DockerHost{ID: hostID, DisplayName: "Test Host"}

	t.Run("no update status - no alert", func(t *testing.T) {
		container := models.DockerContainer{
			ID:           containerID,
			Name:         containerName,
			Image:        "nginx:latest",
			UpdateStatus: nil,
		}

		m.checkDockerContainerImageUpdate(host, container, resourceID, containerName, instanceName, nodeName)

		alerts := m.GetActiveAlerts()
		if hasAlertWithPrefix(alerts, "docker-container-update-"+resourceID) {
			t.Error("Expected no alert when UpdateStatus is nil")
		}
	})

	t.Run("update not available - no alert", func(t *testing.T) {
		container := models.DockerContainer{
			ID:    containerID,
			Name:  containerName,
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: false,
				CurrentDigest:   "sha256:current",
				LatestDigest:    "sha256:current",
				LastChecked:     time.Now(),
			},
		}

		m.checkDockerContainerImageUpdate(host, container, resourceID, containerName, instanceName, nodeName)

		alerts := m.GetActiveAlerts()
		if hasAlertWithPrefix(alerts, "docker-container-update-"+resourceID) {
			t.Error("Expected no alert when update is not available")
		}
	})

	t.Run("update detection error - no alert", func(t *testing.T) {
		container := models.DockerContainer{
			ID:    containerID,
			Name:  containerName,
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: false,
				Error:           "rate limited",
				LastChecked:     time.Now(),
			},
		}

		m.checkDockerContainerImageUpdate(host, container, resourceID, containerName, instanceName, nodeName)

		alerts := m.GetActiveAlerts()
		if hasAlertWithPrefix(alerts, "docker-container-update-"+resourceID) {
			t.Error("Expected no alert when there's an error in update detection")
		}
	})

	t.Run("update available but not yet past threshold - no alert", func(t *testing.T) {
		container := models.DockerContainer{
			ID:    containerID,
			Name:  containerName,
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: true,
				CurrentDigest:   "sha256:olddigest",
				LatestDigest:    "sha256:newdigest",
				LastChecked:     time.Now(),
			},
		}

		// First call - should record firstSeen but not alert yet
		m.checkDockerContainerImageUpdate(host, container, resourceID, containerName, instanceName, nodeName)

		alerts := m.GetActiveAlerts()
		if hasAlertWithPrefix(alerts, "docker-container-update-"+resourceID) {
			t.Error("Expected no alert when update just detected (threshold not reached)")
		}

		// Verify tracking is set
		m.mu.RLock()
		_, tracked := m.dockerUpdateFirstSeen[resourceID]
		trackingKey := dockerUpdateTrackingKey(host, container)
		_, trackedByIdentity := m.dockerUpdateFirstSeenByIdentity[trackingKey]
		m.mu.RUnlock()
		if !tracked {
			t.Error("Expected update to be tracked in dockerUpdateFirstSeen")
		}
		if !trackedByIdentity {
			t.Error("Expected update to be tracked in dockerUpdateFirstSeenByIdentity")
		}
	})

	t.Run("update available and past threshold - creates alert", func(t *testing.T) {
		// Use a fresh resource ID for this test to avoid interference
		testResourceID := "docker:" + hostID + "/container-threshold-test"

		// Set up tracking as if we detected the update 25 hours ago
		m.mu.Lock()
		m.dockerUpdateFirstSeen[testResourceID] = time.Now().Add(-25 * time.Hour)
		m.mu.Unlock()

		container := models.DockerContainer{
			ID:    "container-threshold-test",
			Name:  "threshold-test-container",
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: true,
				CurrentDigest:   "sha256:olddigest",
				LatestDigest:    "sha256:newdigest",
				LastChecked:     time.Now(),
			},
		}

		m.checkDockerContainerImageUpdate(host, container, testResourceID, "threshold-test-container", instanceName, nodeName)

		alerts := m.GetActiveAlerts()
		found := false
		for _, a := range alerts {
			if a.Type == "docker-container-update" {
				found = true
				if a.Level != AlertLevelWarning {
					t.Errorf("Expected level %q, got %q", AlertLevelWarning, a.Level)
				}
				break
			}
		}
		if !found {
			t.Error("Expected alert to be created after threshold")
		}
	})

	t.Run("alert cleared when update applied", func(t *testing.T) {
		// Use a fresh resource ID
		testResourceID := "docker:" + hostID + "/container-clear-test"

		// Set up an existing alert by triggering past threshold
		m.mu.Lock()
		m.dockerUpdateFirstSeen[testResourceID] = time.Now().Add(-25 * time.Hour)
		m.mu.Unlock()

		container := models.DockerContainer{
			ID:    "container-clear-test",
			Name:  "clear-test-container",
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: true,
				CurrentDigest:   "sha256:olddigest",
				LatestDigest:    "sha256:newdigest",
				LastChecked:     time.Now(),
			},
		}

		// First trigger the alert
		m.checkDockerContainerImageUpdate(host, container, testResourceID, "clear-test-container", instanceName, nodeName)

		// Now simulate container being updated
		container.UpdateStatus = &models.DockerContainerUpdateStatus{
			UpdateAvailable: false,
			CurrentDigest:   "sha256:newdigest",
			LatestDigest:    "sha256:newdigest",
			LastChecked:     time.Now(),
		}

		m.checkDockerContainerImageUpdate(host, container, testResourceID, "clear-test-container", instanceName, nodeName)

		// Verify tracking is removed
		m.mu.RLock()
		_, tracked := m.dockerUpdateFirstSeen[testResourceID]
		trackingKey := dockerUpdateTrackingKey(host, container)
		_, trackedByIdentity := m.dockerUpdateFirstSeenByIdentity[trackingKey]
		m.mu.RUnlock()
		if tracked {
			t.Error("Expected tracking to be removed when update is applied")
		}
		if trackedByIdentity {
			t.Error("Expected identity tracking to be removed when update is applied")
		}
	})

	t.Run("disabled by negative delay hours", func(t *testing.T) {
		// Create manager with disabled update alerts
		m2 := NewManager()
		m2.mu.Lock()
		m2.config.DockerDefaults.UpdateAlertDelayHours = -1 // Disable
		m2.mu.Unlock()

		testResourceID := "docker:" + hostID + "/container-disabled-test"

		container := models.DockerContainer{
			ID:    "container-disabled-test",
			Name:  "disabled-test-container",
			Image: "nginx:latest",
			UpdateStatus: &models.DockerContainerUpdateStatus{
				UpdateAvailable: true,
				CurrentDigest:   "sha256:olddigest",
				LatestDigest:    "sha256:newdigest",
				LastChecked:     time.Now(),
			},
		}

		m2.checkDockerContainerImageUpdate(host, container, testResourceID, "disabled-test-container", instanceName, nodeName)

		alerts := m2.GetActiveAlerts()
		for _, a := range alerts {
			if a.Type == "docker-container-update" {
				t.Error("Expected no alert when delay hours is negative (disabled)")
				break
			}
		}
	})
}

func TestCheckDockerContainerImageUpdatePreservesDelayAcrossHostIDChange(t *testing.T) {
	m := NewManager()
	m.mu.Lock()
	m.config.DockerDefaults.UpdateAlertDelayHours = 24
	m.mu.Unlock()

	container := models.DockerContainer{
		ID:    "container-abc123",
		Name:  "web",
		Image: "nginx:latest",
		UpdateStatus: &models.DockerContainerUpdateStatus{
			UpdateAvailable: true,
			CurrentDigest:   "sha256:old",
			LatestDigest:    "sha256:new",
			LastChecked:     time.Now(),
		},
	}

	oldHost := models.DockerHost{
		ID:          "docker-host-old",
		AgentID:     "agent-stable-1",
		DisplayName: "Old Docker Host",
		Hostname:    "docker.local",
	}
	newHost := oldHost
	newHost.ID = "docker-host-new"
	newHost.DisplayName = "New Docker Host"

	oldResourceID := dockerResourceID(oldHost.ID, container.ID)
	m.checkDockerContainerImageUpdate(oldHost, container, oldResourceID, "web", "docker-instance", "docker.local")

	firstSeen := time.Now().Add(-25 * time.Hour)
	trackingKey := dockerUpdateTrackingKey(oldHost, container)
	m.mu.Lock()
	m.dockerUpdateFirstSeen[oldResourceID] = firstSeen
	m.dockerUpdateFirstSeenByIdentity[trackingKey] = firstSeen
	m.mu.Unlock()

	newResourceID := dockerResourceID(newHost.ID, container.ID)
	m.checkDockerContainerImageUpdate(newHost, container, newResourceID, "web", "docker-instance", "docker.local")

	alertID := "docker-container-update-" + newResourceID
	m.mu.RLock()
	alert, hasAlert := m.activeAlerts[alertID]
	resourceFirstSeen, hasResourceTracking := m.dockerUpdateFirstSeen[newResourceID]
	identityFirstSeen, hasIdentityTracking := m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(newHost, container)]
	m.mu.RUnlock()

	if !hasAlert {
		t.Fatalf("Expected update alert %q after host ID change", alertID)
	}
	if !hasResourceTracking {
		t.Fatalf("Expected resource tracking for new host resource ID")
	}
	if !hasIdentityTracking {
		t.Fatalf("Expected identity tracking to persist across host ID change")
	}

	const tolerance = 2 * time.Second
	if delta := alert.StartTime.Sub(firstSeen); delta < -tolerance || delta > tolerance {
		t.Fatalf("Expected alert StartTime near %s, got %s", firstSeen, alert.StartTime)
	}
	if delta := resourceFirstSeen.Sub(firstSeen); delta < -tolerance || delta > tolerance {
		t.Fatalf("Expected resource firstSeen near %s, got %s", firstSeen, resourceFirstSeen)
	}
	if delta := identityFirstSeen.Sub(firstSeen); delta < -tolerance || delta > tolerance {
		t.Fatalf("Expected identity firstSeen near %s, got %s", firstSeen, identityFirstSeen)
	}
}

// Note: Update alerts are now a free feature (no license gating).
// The Pro license gating tests were removed when update alerts became free.

func TestDockerUpdateTrackingCleanup(t *testing.T) {
	m := NewManager()

	hostID := "docker-host-1"
	prefix := "docker:" + hostID + "/"
	host := models.DockerHost{
		ID:       hostID,
		AgentID:  "agent-1",
		Hostname: "docker-host-1.local",
	}
	otherHost := models.DockerHost{
		ID:      "other-host",
		AgentID: "agent-other",
	}
	c1 := models.DockerContainer{ID: "container-1"}
	c2 := models.DockerContainer{ID: "container-2"}
	c3 := models.DockerContainer{ID: "container-3"}
	otherContainer := models.DockerContainer{ID: "container-x"}

	// Add some tracking entries
	m.mu.Lock()
	m.dockerUpdateFirstSeen[prefix+"container-1"] = time.Now()
	m.dockerUpdateFirstSeen[prefix+"container-2"] = time.Now()
	m.dockerUpdateFirstSeen[prefix+"container-3"] = time.Now()
	m.dockerUpdateFirstSeen["docker:other-host/container-x"] = time.Now()
	m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c1)] = time.Now()
	m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c2)] = time.Now()
	m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c3)] = time.Now()
	m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(otherHost, otherContainer)] = time.Now()
	m.mu.Unlock()

	// Create seen maps with only container-1 and container-2
	seen := map[string]struct{}{
		prefix + "container-1": {},
		prefix + "container-2": {},
		// container-3 is removed
	}
	seenUpdateTracking := map[string]struct{}{
		dockerUpdateTrackingKey(host, c1): {},
		dockerUpdateTrackingKey(host, c2): {},
	}

	// Run cleanup
	m.cleanupDockerContainerAlertsWithTracking(host, seen, seenUpdateTracking)

	// Verify container-3 tracking is removed
	m.mu.RLock()
	_, hasC1 := m.dockerUpdateFirstSeen[prefix+"container-1"]
	_, hasC2 := m.dockerUpdateFirstSeen[prefix+"container-2"]
	_, hasC3 := m.dockerUpdateFirstSeen[prefix+"container-3"]
	_, hasOther := m.dockerUpdateFirstSeen["docker:other-host/container-x"]
	_, hasC1ByIdentity := m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c1)]
	_, hasC2ByIdentity := m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c2)]
	_, hasC3ByIdentity := m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(host, c3)]
	_, hasOtherByIdentity := m.dockerUpdateFirstSeenByIdentity[dockerUpdateTrackingKey(otherHost, otherContainer)]
	m.mu.RUnlock()

	if !hasC1 {
		t.Error("Expected container-1 tracking to remain")
	}
	if !hasC2 {
		t.Error("Expected container-2 tracking to remain")
	}
	if hasC3 {
		t.Error("Expected container-3 tracking to be removed")
	}
	if !hasOther {
		t.Error("Expected other host tracking to remain")
	}
	if !hasC1ByIdentity {
		t.Error("Expected container-1 identity tracking to remain")
	}
	if !hasC2ByIdentity {
		t.Error("Expected container-2 identity tracking to remain")
	}
	if hasC3ByIdentity {
		t.Error("Expected container-3 identity tracking to be removed")
	}
	if !hasOtherByIdentity {
		t.Error("Expected other host identity tracking to remain")
	}
}
