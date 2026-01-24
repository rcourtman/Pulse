package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
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

func TestMarkFailed(t *testing.T) {
	t.Parallel()

	t.Run("sets status to failed", func(t *testing.T) {
		t.Parallel()

		cmd := newDockerHostCommand(DockerCommandTypeStop, "test", dockerCommandDefaultTTL, nil)
		cmd.markFailed("connection lost")

		if cmd.status.Status != DockerCommandStatusFailed {
			t.Errorf("expected status %q, got %q", DockerCommandStatusFailed, cmd.status.Status)
		}
	})

	t.Run("sets FailedAt to current time", func(t *testing.T) {
		t.Parallel()

		cmd := newDockerHostCommand(DockerCommandTypeStop, "test", dockerCommandDefaultTTL, nil)
		before := time.Now().UTC()
		cmd.markFailed("timeout")
		after := time.Now().UTC()

		if cmd.status.FailedAt == nil {
			t.Fatal("expected FailedAt to be set")
		}
		if cmd.status.FailedAt.Before(before) || cmd.status.FailedAt.After(after) {
			t.Errorf("FailedAt %v not in expected range [%v, %v]", *cmd.status.FailedAt, before, after)
		}
	})

	t.Run("sets UpdatedAt to current time", func(t *testing.T) {
		t.Parallel()

		cmd := newDockerHostCommand(DockerCommandTypeStop, "test", dockerCommandDefaultTTL, nil)
		originalUpdatedAt := cmd.status.UpdatedAt
		time.Sleep(time.Millisecond) // Ensure time advances
		before := time.Now().UTC()
		cmd.markFailed("error")
		after := time.Now().UTC()

		if !cmd.status.UpdatedAt.After(originalUpdatedAt) {
			t.Errorf("UpdatedAt should be updated; original=%v, new=%v", originalUpdatedAt, cmd.status.UpdatedAt)
		}
		if cmd.status.UpdatedAt.Before(before) || cmd.status.UpdatedAt.After(after) {
			t.Errorf("UpdatedAt %v not in expected range [%v, %v]", cmd.status.UpdatedAt, before, after)
		}
	})

	t.Run("sets FailureReason to provided reason", func(t *testing.T) {
		t.Parallel()

		cmd := newDockerHostCommand(DockerCommandTypeStop, "test", dockerCommandDefaultTTL, nil)
		reason := "agent unreachable: connection refused"
		cmd.markFailed(reason)

		if cmd.status.FailureReason != reason {
			t.Errorf("expected FailureReason %q, got %q", reason, cmd.status.FailureReason)
		}
	})

	t.Run("accepts empty reason string", func(t *testing.T) {
		t.Parallel()

		cmd := newDockerHostCommand(DockerCommandTypeStop, "test", dockerCommandDefaultTTL, nil)
		cmd.markFailed("")

		if cmd.status.Status != DockerCommandStatusFailed {
			t.Errorf("expected status %q, got %q", DockerCommandStatusFailed, cmd.status.Status)
		}
		if cmd.status.FailureReason != "" {
			t.Errorf("expected empty FailureReason, got %q", cmd.status.FailureReason)
		}
		if cmd.status.FailedAt == nil {
			t.Error("expected FailedAt to be set even with empty reason")
		}
	})
}

func TestQueueDockerStopCommand(t *testing.T) {
	t.Parallel()

	t.Run("empty hostID returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		_, err := monitor.queueDockerStopCommand("")
		if err == nil {
			t.Fatal("expected error for empty hostID")
		}
		if !strings.Contains(err.Error(), "docker host id is required") {
			t.Fatalf("expected 'docker host id is required' error, got %q", err.Error())
		}
	})

	t.Run("whitespace-only hostID returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		_, err := monitor.queueDockerStopCommand("   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only hostID")
		}
		if !strings.Contains(err.Error(), "docker host id is required") {
			t.Fatalf("expected 'docker host id is required' error, got %q", err.Error())
		}
	})

	t.Run("host not found returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		_, err := monitor.queueDockerStopCommand("nonexistent-host")
		if err == nil {
			t.Fatal("expected error for nonexistent host")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected 'not found' error, got %q", err.Error())
		}
	})

	t.Run("existing command in Queued status returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-queued",
			Hostname:    "node-queued",
			DisplayName: "node-queued",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue first command
		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("first queue should succeed: %v", err)
		}

		// Try to queue second command
		_, err = monitor.queueDockerStopCommand(host.ID)
		if err == nil {
			t.Fatal("expected error for existing queued command")
		}
		if !strings.Contains(err.Error(), "already has a command in progress") {
			t.Fatalf("expected 'already has a command in progress' error, got %q", err.Error())
		}
	})

	t.Run("existing command in Dispatched status returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-dispatched",
			Hostname:    "node-dispatched",
			DisplayName: "node-dispatched",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue and dispatch command
		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue should succeed: %v", err)
		}
		monitor.FetchDockerCommandForHost(host.ID) // This marks it as dispatched

		// Try to queue second command
		_, err = monitor.queueDockerStopCommand(host.ID)
		if err == nil {
			t.Fatal("expected error for existing dispatched command")
		}
		if !strings.Contains(err.Error(), "already has a command in progress") {
			t.Fatalf("expected 'already has a command in progress' error, got %q", err.Error())
		}
	})

	t.Run("existing command in Acknowledged status returns error", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-ack",
			Hostname:    "node-ack",
			DisplayName: "node-ack",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue, dispatch, and acknowledge command
		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue should succeed: %v", err)
		}
		monitor.FetchDockerCommandForHost(host.ID)
		_, _, _, err = monitor.AcknowledgeDockerHostCommand(cmdStatus.ID, host.ID, DockerCommandStatusAcknowledged, "ack")
		if err != nil {
			t.Fatalf("acknowledge should succeed: %v", err)
		}

		// Try to queue second command
		_, err = monitor.queueDockerStopCommand(host.ID)
		if err == nil {
			t.Fatal("expected error for existing acknowledged command")
		}
		if !strings.Contains(err.Error(), "already has a command in progress") {
			t.Fatalf("expected 'already has a command in progress' error, got %q", err.Error())
		}
	})

	t.Run("existing completed command allows new command", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-completed",
			Hostname:    "node-completed",
			DisplayName: "node-completed",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue, dispatch, and complete command
		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("first queue should succeed: %v", err)
		}
		monitor.FetchDockerCommandForHost(host.ID)
		_, _, _, err = monitor.AcknowledgeDockerHostCommand(cmdStatus.ID, host.ID, DockerCommandStatusCompleted, "done")
		if err != nil {
			t.Fatalf("complete should succeed: %v", err)
		}

		// Queue new command should succeed
		newStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("second queue should succeed after completion: %v", err)
		}
		if newStatus.Status != DockerCommandStatusQueued {
			t.Fatalf("expected queued status, got %s", newStatus.Status)
		}
	})

	t.Run("existing failed command allows new command", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-failed",
			Hostname:    "node-failed",
			DisplayName: "node-failed",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue, dispatch, and fail command
		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("first queue should succeed: %v", err)
		}
		monitor.FetchDockerCommandForHost(host.ID)
		_, _, _, err = monitor.AcknowledgeDockerHostCommand(cmdStatus.ID, host.ID, DockerCommandStatusFailed, "failed")
		if err != nil {
			t.Fatalf("fail should succeed: %v", err)
		}

		// Queue new command should succeed
		newStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("second queue should succeed after failure: %v", err)
		}
		if newStatus.Status != DockerCommandStatusQueued {
			t.Fatalf("expected queued status, got %s", newStatus.Status)
		}
	})

	t.Run("nil dockerCommands map gets initialized", func(t *testing.T) {
		t.Parallel()

		state := models.NewState()
		monitor := &Monitor{
			state:              state,
			removedDockerHosts: make(map[string]time.Time),
			dockerCommands:     nil, // Explicitly nil
			dockerCommandIndex: make(map[string]string),
		}

		host := models.DockerHost{
			ID:          "host-nil-map",
			Hostname:    "node-nil-map",
			DisplayName: "node-nil-map",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue should succeed: %v", err)
		}

		if monitor.dockerCommands == nil {
			t.Fatal("dockerCommands map should be initialized")
		}
		if _, exists := monitor.dockerCommands[host.ID]; !exists {
			t.Fatal("command should be stored in dockerCommands map")
		}
	})

	t.Run("nil dockerCommandIndex map gets initialized", func(t *testing.T) {
		t.Parallel()

		state := models.NewState()
		monitor := &Monitor{
			state:              state,
			removedDockerHosts: make(map[string]time.Time),
			dockerCommands:     make(map[string]*dockerHostCommand),
			dockerCommandIndex: nil, // Explicitly nil
		}

		host := models.DockerHost{
			ID:          "host-nil-index",
			Hostname:    "node-nil-index",
			DisplayName: "node-nil-index",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue should succeed: %v", err)
		}

		if monitor.dockerCommandIndex == nil {
			t.Fatal("dockerCommandIndex map should be initialized")
		}
		if _, exists := monitor.dockerCommandIndex[cmdStatus.ID]; !exists {
			t.Fatal("command should be indexed in dockerCommandIndex map")
		}
	})

	t.Run("successful queue returns correct status", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-success",
			Hostname:    "node-success",
			DisplayName: "node-success",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue should succeed: %v", err)
		}

		// Verify returned status
		if cmdStatus.ID == "" {
			t.Fatal("command ID should not be empty")
		}
		if cmdStatus.Type != DockerCommandTypeStop {
			t.Fatalf("expected type %q, got %q", DockerCommandTypeStop, cmdStatus.Type)
		}
		if cmdStatus.Status != DockerCommandStatusQueued {
			t.Fatalf("expected status %q, got %q", DockerCommandStatusQueued, cmdStatus.Status)
		}
		if cmdStatus.Message != "Stopping agent" {
			t.Fatalf("expected message 'Stopping agent', got %q", cmdStatus.Message)
		}
		if cmdStatus.CreatedAt.IsZero() {
			t.Fatal("CreatedAt should be set")
		}
		if cmdStatus.UpdatedAt.IsZero() {
			t.Fatal("UpdatedAt should be set")
		}
		if cmdStatus.ExpiresAt == nil {
			t.Fatal("ExpiresAt should be set")
		}

		// Verify state updates
		hostState := findDockerHost(t, monitor, host.ID)
		if !hostState.PendingUninstall {
			t.Fatal("host should be marked as pending uninstall")
		}
		if hostState.Command == nil {
			t.Fatal("host command should be set in state")
		}
		if hostState.Command.ID != cmdStatus.ID {
			t.Fatalf("host command ID mismatch: expected %q, got %q", cmdStatus.ID, hostState.Command.ID)
		}

		// Verify internal maps
		if _, exists := monitor.dockerCommands[host.ID]; !exists {
			t.Fatal("command should be in dockerCommands map")
		}
		if resolvedHost, exists := monitor.dockerCommandIndex[cmdStatus.ID]; !exists || resolvedHost != host.ID {
			t.Fatalf("command index mismatch: expected %q, got %q (exists=%v)", host.ID, resolvedHost, exists)
		}
	})

	t.Run("hostID with leading/trailing whitespace is normalized", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{
			ID:          "host-whitespace",
			Hostname:    "node-whitespace",
			DisplayName: "node-whitespace",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue with whitespace-padded ID
		cmdStatus, err := monitor.queueDockerStopCommand("  host-whitespace  ")
		if err != nil {
			t.Fatalf("queue with whitespace should succeed: %v", err)
		}
		if cmdStatus.Status != DockerCommandStatusQueued {
			t.Fatalf("expected queued status, got %s", cmdStatus.Status)
		}

		// Verify command is stored under normalized ID
		if _, exists := monitor.dockerCommands["host-whitespace"]; !exists {
			t.Fatal("command should be stored under normalized host ID")
		}
	})
}

func TestGetDockerCommandPayload(t *testing.T) {
	t.Parallel()

	t.Run("empty hostID returns nil", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		payload, status := monitor.getDockerCommandPayload("")
		if payload != nil {
			t.Fatalf("expected nil payload for empty hostID, got %v", payload)
		}
		if status != nil {
			t.Fatalf("expected nil status for empty hostID, got %v", status)
		}
	})

	t.Run("whitespace-only hostID returns nil", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		payload, status := monitor.getDockerCommandPayload("   ")
		if payload != nil {
			t.Fatalf("expected nil payload for whitespace hostID, got %v", payload)
		}
		if status != nil {
			t.Fatalf("expected nil status for whitespace hostID, got %v", status)
		}
	})

	t.Run("host not found in dockerCommands returns nil", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		payload, status := monitor.getDockerCommandPayload("nonexistent-host")
		if payload != nil {
			t.Fatalf("expected nil payload for nonexistent host, got %v", payload)
		}
		if status != nil {
			t.Fatalf("expected nil status for nonexistent host, got %v", status)
		}
	})

	t.Run("expired command is cleaned up and returns nil", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		host := models.DockerHost{
			ID:          "host-expired",
			Hostname:    "node-expired",
			DisplayName: "node-expired",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue a command
		cmdStatus, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue stop command: %v", err)
		}

		// Manually expire the command
		monitor.mu.Lock()
		cmd := monitor.dockerCommands[host.ID]
		past := time.Now().Add(-2 * dockerCommandDefaultTTL)
		cmd.status.ExpiresAt = &past
		monitor.mu.Unlock()

		// Fetch should return nil and clean up
		payload, status := monitor.getDockerCommandPayload(host.ID)
		if payload != nil {
			t.Fatalf("expected nil payload for expired command, got %v", payload)
		}
		if status != nil {
			t.Fatalf("expected nil status for expired command, got %v", status)
		}

		// Verify cleanup
		monitor.mu.Lock()
		_, commandExists := monitor.dockerCommands[host.ID]
		_, indexExists := monitor.dockerCommandIndex[cmdStatus.ID]
		monitor.mu.Unlock()

		if commandExists {
			t.Fatal("expected expired command to be removed from dockerCommands")
		}
		if indexExists {
			t.Fatal("expected expired command to be removed from dockerCommandIndex")
		}

		// Verify state was updated with expired status
		hostState := findDockerHost(t, monitor, host.ID)
		if hostState.Command == nil {
			t.Fatal("expected host command to be set in state")
		}
		if hostState.Command.Status != DockerCommandStatusExpired {
			t.Fatalf("expected expired status in state, got %q", hostState.Command.Status)
		}
		if hostState.Command.FailureReason == "" {
			t.Fatal("expected failure reason to be set on expired command")
		}
	})

	t.Run("queued command is marked as dispatched", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		host := models.DockerHost{
			ID:          "host-queued",
			Hostname:    "node-queued",
			DisplayName: "node-queued",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue a command
		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue stop command: %v", err)
		}

		// Verify command is queued
		monitor.mu.Lock()
		cmd := monitor.dockerCommands[host.ID]
		if cmd.status.Status != DockerCommandStatusQueued {
			t.Fatalf("expected queued status before fetch, got %q", cmd.status.Status)
		}
		monitor.mu.Unlock()

		// Fetch payload - should mark as dispatched
		payload, status := monitor.getDockerCommandPayload(host.ID)

		// For stop command, payload is nil but status should be returned
		if status == nil {
			t.Fatal("expected status to be returned")
		}
		if status.Status != DockerCommandStatusDispatched {
			t.Fatalf("expected dispatched status, got %q", status.Status)
		}
		if status.DispatchedAt == nil {
			t.Fatal("expected DispatchedAt to be set")
		}

		// Verify state was updated
		hostState := findDockerHost(t, monitor, host.ID)
		if hostState.Command == nil {
			t.Fatal("expected host command to be set in state")
		}
		if hostState.Command.Status != DockerCommandStatusDispatched {
			t.Fatalf("expected dispatched status in state, got %q", hostState.Command.Status)
		}

		// payload is nil for stop commands (no additional data needed)
		if payload != nil {
			t.Fatalf("expected nil payload for stop command, got %v", payload)
		}
	})

	t.Run("already dispatched command is not re-marked", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		host := models.DockerHost{
			ID:          "host-dispatched",
			Hostname:    "node-dispatched",
			DisplayName: "node-dispatched",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue a command
		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue stop command: %v", err)
		}

		// First fetch - marks as dispatched
		_, status1 := monitor.getDockerCommandPayload(host.ID)
		if status1 == nil {
			t.Fatal("expected status from first fetch")
		}
		firstDispatchedAt := status1.DispatchedAt
		firstUpdatedAt := status1.UpdatedAt

		// Small delay to ensure time would change if re-marked
		time.Sleep(time.Millisecond)

		// Second fetch - should NOT re-mark
		_, status2 := monitor.getDockerCommandPayload(host.ID)
		if status2 == nil {
			t.Fatal("expected status from second fetch")
		}

		// DispatchedAt should be the same
		if status2.DispatchedAt == nil {
			t.Fatal("expected DispatchedAt to still be set")
		}
		if !status2.DispatchedAt.Equal(*firstDispatchedAt) {
			t.Fatalf("DispatchedAt changed: first=%v, second=%v", *firstDispatchedAt, *status2.DispatchedAt)
		}

		// UpdatedAt should be the same (not re-updated)
		if !status2.UpdatedAt.Equal(firstUpdatedAt) {
			t.Fatalf("UpdatedAt changed: first=%v, second=%v", firstUpdatedAt, status2.UpdatedAt)
		}

		// Status should still be dispatched
		if status2.Status != DockerCommandStatusDispatched {
			t.Fatalf("expected dispatched status, got %q", status2.Status)
		}
	})

	t.Run("returns payload and status copy", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		host := models.DockerHost{
			ID:          "host-payload",
			Hostname:    "node-payload",
			DisplayName: "node-payload",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Manually create a command with a payload
		testPayload := map[string]any{
			"action": "test",
			"count":  42,
		}
		cmd := newDockerHostCommand("test-type", "test message", dockerCommandDefaultTTL, testPayload)

		monitor.mu.Lock()
		monitor.dockerCommands[host.ID] = &cmd
		monitor.dockerCommandIndex[cmd.status.ID] = host.ID
		monitor.mu.Unlock()

		// Fetch payload
		payload, status := monitor.getDockerCommandPayload(host.ID)

		if payload == nil {
			t.Fatal("expected payload to be returned")
		}
		if payload["action"] != "test" {
			t.Fatalf("expected action 'test', got %v", payload["action"])
		}
		if payload["count"] != 42 {
			t.Fatalf("expected count 42, got %v", payload["count"])
		}

		if status == nil {
			t.Fatal("expected status to be returned")
		}
		if status.Type != "test-type" {
			t.Fatalf("expected type 'test-type', got %q", status.Type)
		}
		if status.Message != "test message" {
			t.Fatalf("expected message 'test message', got %q", status.Message)
		}

		// Verify it's a copy by modifying the returned status
		status.Message = "modified"
		monitor.mu.Lock()
		originalCmd := monitor.dockerCommands[host.ID]
		if originalCmd.status.Message == "modified" {
			t.Fatal("modifying returned status should not affect original")
		}
		monitor.mu.Unlock()
	})

	t.Run("hostID with whitespace is normalized", func(t *testing.T) {
		t.Parallel()

		monitor := newTestMonitorForCommands(t)

		host := models.DockerHost{
			ID:          "host-whitespace",
			Hostname:    "node-whitespace",
			DisplayName: "node-whitespace",
			Status:      "online",
		}
		monitor.state.UpsertDockerHost(host)

		// Queue a command
		_, err := monitor.queueDockerStopCommand(host.ID)
		if err != nil {
			t.Fatalf("queue stop command: %v", err)
		}

		// Fetch with whitespace-padded ID
		_, status := monitor.getDockerCommandPayload("  host-whitespace  ")
		if status == nil {
			t.Fatal("expected status to be returned for whitespace-padded hostID")
		}
		if status.Status != DockerCommandStatusDispatched {
			t.Fatalf("expected dispatched status, got %q", status.Status)
		}
	})
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

func TestQueueDockerContainerUpdateCommand(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)
	host := models.DockerHost{
		ID:       "host-1",
		Hostname: "node-1",
		Status:   "online",
	}
	monitor.state.UpsertDockerHost(host)

	containerID := "test-container"
	containerName := "my-app"

	cmdStatus, err := monitor.QueueDockerContainerUpdateCommand(host.ID, containerID, containerName)
	if err != nil {
		t.Fatalf("Failed to queue update command: %v", err)
	}

	if !strings.HasSuffix(cmdStatus.ID, ":"+containerID) {
		t.Errorf("Expected command ID to end with :%s, got %s", containerID, cmdStatus.ID)
	}

	if cmdStatus.Type != DockerCommandTypeUpdateContainer {
		t.Errorf("Expected type %s, got %s", DockerCommandTypeUpdateContainer, cmdStatus.Type)
	}
}

func TestQueueDockerCheckUpdatesCommand(t *testing.T) {
	t.Parallel()

	monitor := newTestMonitorForCommands(t)
	host := models.DockerHost{
		ID:          "host-check",
		Hostname:    "node-check",
		DisplayName: "node-check",
		Status:      "online",
	}
	monitor.state.UpsertDockerHost(host)

	t.Run("successfully queue check updates", func(t *testing.T) {
		status, err := monitor.QueueDockerCheckUpdatesCommand(host.ID)
		assert.NoError(t, err)
		assert.Equal(t, DockerCommandTypeCheckUpdates, status.Type)
		assert.Equal(t, DockerCommandStatusQueued, status.Status)
	})

	t.Run("existing command in progress", func(t *testing.T) {
		_, err := monitor.QueueDockerCheckUpdatesCommand(host.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already has a command in progress")
	})

	t.Run("host not found", func(t *testing.T) {
		_, err := monitor.QueueDockerCheckUpdatesCommand("missing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestMarkInProgress(t *testing.T) {
	t.Parallel()

	t.Run("transitions status and updates message", func(t *testing.T) {
		cmd := newDockerHostCommand(DockerCommandTypeCheckUpdates, "queued", dockerCommandDefaultTTL, nil)
		cmd.markInProgress("running now")

		assert.Equal(t, DockerCommandStatusInProgress, cmd.status.Status)
		assert.Equal(t, "running now", cmd.status.Message)
		assert.False(t, cmd.status.UpdatedAt.IsZero())
	})

	t.Run("lifecycle through AcknowledgeDockerHostCommand", func(t *testing.T) {
		monitor := newTestMonitorForCommands(t)
		host := models.DockerHost{ID: "h1", Hostname: "n1"}
		monitor.state.UpsertDockerHost(host)

		status, _ := monitor.QueueDockerCheckUpdatesCommand("h1")

		newStatus, _, _, err := monitor.AcknowledgeDockerHostCommand(status.ID, "h1", DockerCommandStatusInProgress, "in progress message")
		assert.NoError(t, err)
		assert.Equal(t, DockerCommandStatusInProgress, newStatus.Status)
		assert.Equal(t, "in progress message", newStatus.Message)

		// Verify state update
		hostState := findDockerHost(t, monitor, "h1")
		assert.NotNil(t, hostState.Command)
		assert.Equal(t, DockerCommandStatusInProgress, hostState.Command.Status)
	})
}
