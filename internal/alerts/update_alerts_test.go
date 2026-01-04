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
		m.mu.RUnlock()
		if !tracked {
			t.Error("Expected update to be tracked in dockerUpdateFirstSeen")
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
		m.mu.RUnlock()
		if tracked {
			t.Error("Expected tracking to be removed when update is applied")
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

// Note: Update alerts are now a free feature (no license gating).
// The Pro license gating tests were removed when update alerts became free.

func TestDockerUpdateTrackingCleanup(t *testing.T) {
	m := NewManager()

	hostID := "docker-host-1"
	prefix := "docker:" + hostID + "/"

	// Add some tracking entries
	m.mu.Lock()
	m.dockerUpdateFirstSeen[prefix+"container-1"] = time.Now()
	m.dockerUpdateFirstSeen[prefix+"container-2"] = time.Now()
	m.dockerUpdateFirstSeen[prefix+"container-3"] = time.Now()
	m.dockerUpdateFirstSeen["docker:other-host/container-x"] = time.Now()
	m.mu.Unlock()

	// Create host and seen map with only container-1 and container-2
	host := models.DockerHost{ID: hostID}
	seen := map[string]struct{}{
		prefix + "container-1": {},
		prefix + "container-2": {},
		// container-3 is removed
	}

	// Run cleanup
	m.cleanupDockerContainerAlerts(host, seen)

	// Verify container-3 tracking is removed
	m.mu.RLock()
	_, hasC1 := m.dockerUpdateFirstSeen[prefix+"container-1"]
	_, hasC2 := m.dockerUpdateFirstSeen[prefix+"container-2"]
	_, hasC3 := m.dockerUpdateFirstSeen[prefix+"container-3"]
	_, hasOther := m.dockerUpdateFirstSeen["docker:other-host/container-x"]
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
}
