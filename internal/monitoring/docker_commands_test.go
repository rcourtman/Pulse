package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newTestMonitorForCommands(t *testing.T) *Monitor {
	t.Helper()
	state := models.NewState()
	return &Monitor{
		state:              state,
		removedDockerHosts: make(map[string]time.Time),
		dockerCommands:     make(map[string]*dockerHostCommand),
		dockerCommandIndex: make(map[string]string),
	}
}

func findDockerHost(t *testing.T, m *Monitor, id string) models.DockerHost {
	t.Helper()
	hosts := m.state.GetDockerHosts()
	for _, host := range hosts {
		if host.ID == id {
			return host
		}
	}
	t.Fatalf("docker host %s not found", id)
	return models.DockerHost{}
}

func TestDockerStopCommandLifecycle(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)

	host := models.DockerHost{
		ID:          "host-1",
		Hostname:    "node-1",
		DisplayName: "node-1",
		Status:      "online",
	}
	monitor.state.UpsertDockerHost(host)

	cmdStatus, err := monitor.QueueDockerHostStop(host.ID)
	if err != nil {
		t.Fatalf("queue stop command: %v", err)
	}
	if cmdStatus.Status != DockerCommandStatusQueued {
		t.Fatalf("expected queued status, got %s", cmdStatus.Status)
	}

	hostState := findDockerHost(t, monitor, host.ID)
	if !hostState.PendingUninstall {
		t.Fatalf("expected host to be marked pending uninstall")
	}
	if hostState.Command == nil || hostState.Command.Status != DockerCommandStatusQueued {
		t.Fatalf("expected host command to be stored with queued status")
	}

	payload, fetchedStatus := monitor.FetchDockerCommandForHost(host.ID)
	if payload != nil {
		t.Fatalf("expected no payload, got %v", payload)
	}
	if fetchedStatus == nil {
		t.Fatalf("expected command status from fetch")
	}
	if fetchedStatus.Status != DockerCommandStatusDispatched {
		t.Fatalf("expected dispatched status, got %s", fetchedStatus.Status)
	}

	completedStatus, returnedHostID, shouldRemove, err := monitor.AcknowledgeDockerHostCommand(fetchedStatus.ID, host.ID, DockerCommandStatusCompleted, "done")
	if err != nil {
		t.Fatalf("acknowledge command: %v", err)
	}
	if returnedHostID != host.ID {
		t.Fatalf("expected host id %s, got %s", host.ID, returnedHostID)
	}
	if completedStatus.Status != DockerCommandStatusCompleted {
		t.Fatalf("expected completed status, got %s", completedStatus.Status)
	}
	if !shouldRemove {
		t.Fatalf("expected shouldRemove true for stop command")
	}

	if len(monitor.dockerCommands) != 0 {
		t.Fatalf("expected command map to be cleared")
	}
	if len(monitor.dockerCommandIndex) != 0 {
		t.Fatalf("expected command index to be cleared")
	}

	hostState = findDockerHost(t, monitor, host.ID)
	if hostState.Command == nil || hostState.Command.Status != DockerCommandStatusCompleted {
		t.Fatalf("expected host command status to be completed in state")
	}
	if hostState.Command.Message != "done" {
		t.Fatalf("expected completion message to be propagated")
	}
}

func TestDockerCommandExpiryCleanup(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)

	host := models.DockerHost{
		ID:          "host-expire",
		Hostname:    "expire",
		DisplayName: "expire",
		Status:      "online",
	}
	monitor.state.UpsertDockerHost(host)

	status, err := monitor.QueueDockerHostStop(host.ID)
	if err != nil {
		t.Fatalf("queue stop command: %v", err)
	}
	if status.Status != DockerCommandStatusQueued {
		t.Fatalf("expected queued status, got %s", status.Status)
	}

	monitor.mu.Lock()
	cmd := monitor.dockerCommands[host.ID]
	past := time.Now().Add(-2 * dockerCommandDefaultTTL)
	cmd.status.ExpiresAt = &past
	monitor.mu.Unlock()

	payload, fetched := monitor.FetchDockerCommandForHost(host.ID)
	if payload != nil || fetched != nil {
		t.Fatalf("expected expired command to be removed")
	}

	if len(monitor.dockerCommands) != 0 {
		t.Fatalf("expected command to be removed after expiry")
	}
	if len(monitor.dockerCommandIndex) != 0 {
		t.Fatalf("expected index to be cleared after expiry")
	}

	hostState := findDockerHost(t, monitor, host.ID)
	if hostState.Command == nil || hostState.Command.Status != DockerCommandStatusExpired {
		t.Fatalf("expected state to record expiry status, got %#v", hostState.Command)
	}
	if hostState.Command.FailureReason == "" {
		t.Fatalf("expected failure reason to be set on expiry")
	}
}

func TestAllowDockerHostReenrollClearsState(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)

	host := models.DockerHost{
		ID:               "host-reenroll",
		Hostname:         "reenroll",
		DisplayName:      "reenroll",
		Status:           "online",
		PendingUninstall: true,
	}
	monitor.state.UpsertDockerHost(host)
	monitor.removedDockerHosts[host.ID] = time.Now().Add(-time.Hour)

	status, err := monitor.QueueDockerHostStop(host.ID)
	if err != nil {
		t.Fatalf("queue stop command: %v", err)
	}
	if status.Status != DockerCommandStatusQueued {
		t.Fatalf("expected queued status, got %s", status.Status)
	}

	if err := monitor.AllowDockerHostReenroll(host.ID); err != nil {
		t.Fatalf("allow reenroll: %v", err)
	}

	if _, exists := monitor.removedDockerHosts[host.ID]; exists {
		t.Fatalf("expected host removal block to be cleared")
	}
	if _, exists := monitor.dockerCommands[host.ID]; exists {
		t.Fatalf("expected any queued commands to be cleared")
	}

	hostState := findDockerHost(t, monitor, host.ID)
	if hostState.Command != nil {
		t.Fatalf("expected host command pointer to be cleared")
	}
}

func TestCleanupRemovedDockerHosts(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)
	monitor.removedDockerHosts["old-host"] = time.Now().Add(-2 * removedDockerHostsTTL)
	monitor.removedDockerHosts["fresh-host"] = time.Now()

	now := time.Now()
	monitor.cleanupRemovedDockerHosts(now)

	if _, exists := monitor.removedDockerHosts["old-host"]; exists {
		t.Fatalf("expected old host removal entry to be purged")
	}
	if _, exists := monitor.removedDockerHosts["fresh-host"]; !exists {
		t.Fatalf("expected fresh host removal entry to remain")
	}
}

func TestAllowDockerHostReenrollNoopWhenHostNotBlocked(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)

	host := models.DockerHost{
		ID:          "host-not-blocked",
		Hostname:    "not-blocked",
		DisplayName: "Not Blocked",
		Status:      "online",
	}
	monitor.state.UpsertDockerHost(host)

	if err := monitor.AllowDockerHostReenroll(host.ID); err != nil {
		t.Fatalf("allow reenroll for non-blocked host returned error: %v", err)
	}

	if _, exists := monitor.removedDockerHosts[host.ID]; exists {
		t.Fatalf("non-blocked host should not be added to removal map")
	}

	stateHost := findDockerHost(t, monitor, host.ID)
	if stateHost.ID != host.ID {
		t.Fatalf("expected host to remain in state; got %+v", stateHost)
	}
}
