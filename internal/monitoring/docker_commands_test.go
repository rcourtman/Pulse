package monitoring

import (
	"strings"
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

func TestDockerCommandHasExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt *time.Time
		checkTime time.Time
		want      bool
	}{
		{
			name:      "nil ExpiresAt returns false",
			expiresAt: nil,
			checkTime: time.Now(),
			want:      false,
		},
		{
			name:      "future expiry returns false",
			expiresAt: func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
			checkTime: time.Now(),
			want:      false,
		},
		{
			name:      "past expiry returns true",
			expiresAt: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			checkTime: time.Now(),
			want:      true,
		},
		{
			name:      "exact expiry time returns false",
			expiresAt: func() *time.Time { t := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC); return &t }(),
			checkTime: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			want:      false, // checkTime.After(expiresAt) is false when equal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &dockerHostCommand{
				status: models.DockerHostCommandStatus{
					ExpiresAt: tt.expiresAt,
				},
			}

			got := cmd.hasExpired(tt.checkTime)
			if got != tt.want {
				t.Errorf("hasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAcknowledgeDockerCommandErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(*Monitor) (commandID, hostID, status, message string)
		expectedError string
	}{
		{
			name: "empty command ID",
			setup: func(m *Monitor) (string, string, string, string) {
				return "", "host-1", DockerCommandStatusCompleted, ""
			},
			expectedError: "command id is required",
		},
		{
			name: "whitespace-only command ID",
			setup: func(m *Monitor) (string, string, string, string) {
				return "   ", "host-1", DockerCommandStatusCompleted, ""
			},
			expectedError: "command id is required",
		},
		{
			name: "command not found in index",
			setup: func(m *Monitor) (string, string, string, string) {
				return "nonexistent-cmd-id", "host-1", DockerCommandStatusCompleted, ""
			},
			expectedError: `docker host command "nonexistent-cmd-id" not found`,
		},
		{
			name: "empty normalized host ID",
			setup: func(m *Monitor) (string, string, string, string) {
				host := models.DockerHost{
					ID:          "host-1",
					Hostname:    "node-1",
					DisplayName: "node-1",
					Status:      "online",
				}
				m.state.UpsertDockerHost(host)

				cmdStatus, err := m.QueueDockerHostStop(host.ID)
				if err != nil {
					t.Fatalf("queue stop command: %v", err)
				}

				return cmdStatus.ID, "   ", DockerCommandStatusCompleted, ""
			},
			expectedError: "docker host id is required",
		},
		{
			name: "command belongs to wrong host",
			setup: func(m *Monitor) (string, string, string, string) {
				host := models.DockerHost{
					ID:          "host-correct",
					Hostname:    "node-correct",
					DisplayName: "node-correct",
					Status:      "online",
				}
				m.state.UpsertDockerHost(host)

				cmdStatus, err := m.QueueDockerHostStop(host.ID)
				if err != nil {
					t.Fatalf("queue stop command: %v", err)
				}

				return cmdStatus.ID, "host-wrong", DockerCommandStatusCompleted, ""
			},
			expectedError: "does not belong to host",
		},
		{
			name: "command not in active map",
			setup: func(m *Monitor) (string, string, string, string) {
				host := models.DockerHost{
					ID:          "host-1",
					Hostname:    "node-1",
					DisplayName: "node-1",
					Status:      "online",
				}
				m.state.UpsertDockerHost(host)

				cmdStatus, err := m.QueueDockerHostStop(host.ID)
				if err != nil {
					t.Fatalf("queue stop command: %v", err)
				}

				m.mu.Lock()
				delete(m.dockerCommands, host.ID)
				m.mu.Unlock()

				return cmdStatus.ID, host.ID, DockerCommandStatusCompleted, ""
			},
			expectedError: "not active",
		},
		{
			name: "invalid command status",
			setup: func(m *Monitor) (string, string, string, string) {
				host := models.DockerHost{
					ID:          "host-1",
					Hostname:    "node-1",
					DisplayName: "node-1",
					Status:      "online",
				}
				m.state.UpsertDockerHost(host)

				cmdStatus, err := m.QueueDockerHostStop(host.ID)
				if err != nil {
					t.Fatalf("queue stop command: %v", err)
				}

				m.FetchDockerCommandForHost(host.ID)

				return cmdStatus.ID, host.ID, "invalid-status", ""
			},
			expectedError: `invalid command status "invalid-status"`,
		},
		{
			name: "empty host ID skips host validation",
			setup: func(m *Monitor) (string, string, string, string) {
				host := models.DockerHost{
					ID:          "host-1",
					Hostname:    "node-1",
					DisplayName: "node-1",
					Status:      "online",
				}
				m.state.UpsertDockerHost(host)

				cmdStatus, err := m.QueueDockerHostStop(host.ID)
				if err != nil {
					t.Fatalf("queue stop command: %v", err)
				}

				m.FetchDockerCommandForHost(host.ID)

				// Empty hostID should skip host validation and succeed
				return cmdStatus.ID, "", DockerCommandStatusCompleted, "done"
			},
			expectedError: "", // No error expected - this is a success case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := newTestMonitorForCommands(t)

			commandID, hostID, status, message := tt.setup(monitor)

			_, _, _, err := monitor.AcknowledgeDockerHostCommand(commandID, hostID, status, message)

			if tt.expectedError == "" {
				// Success case
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			// Error case
			if err == nil {
				t.Fatalf("expected error containing %q, got none", tt.expectedError)
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}
