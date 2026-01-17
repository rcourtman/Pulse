package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// mockDockerChecker is a test implementation of DockerChecker
type mockDockerChecker struct {
	results map[int]bool // vmid -> hasDocker
	calls   []int        // records vmids that were checked
}

func (m *mockDockerChecker) CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error) {
	m.calls = append(m.calls, vmid)
	if hasDocker, ok := m.results[vmid]; ok {
		return hasDocker, nil
	}
	return false, nil
}

func TestCheckContainersForDocker_NewContainers(t *testing.T) {
	// Create a monitor with state
	state := models.NewState()
	monitor := &Monitor{
		state: state,
	}

	// Create a mock checker
	checker := &mockDockerChecker{
		results: map[int]bool{
			101: true,
			102: false,
			103: true, // This one is stopped, so it shouldn't be checked
		},
	}
	monitor.SetDockerChecker(checker)

	// Input containers - all new (not in state yet)
	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "container-with-docker", Node: "node1", Status: "running"},
		{ID: "ct-2", VMID: 102, Name: "container-without-docker", Node: "node1", Status: "running"},
		{ID: "ct-3", VMID: 103, Name: "stopped-container", Node: "node1", Status: "stopped"},
	}

	// Run the check
	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Verify results
	var ct1, ct2, ct3 *models.Container
	for i := range result {
		switch result[i].ID {
		case "ct-1":
			ct1 = &result[i]
		case "ct-2":
			ct2 = &result[i]
		case "ct-3":
			ct3 = &result[i]
		}
	}

	if ct1 == nil || !ct1.HasDocker {
		t.Errorf("Container 101 should have HasDocker=true")
	}
	if ct1 != nil && ct1.DockerCheckedAt.IsZero() {
		t.Errorf("Container 101 should have DockerCheckedAt set")
	}

	if ct2 == nil || ct2.HasDocker {
		t.Errorf("Container 102 should have HasDocker=false")
	}
	if ct2 != nil && ct2.DockerCheckedAt.IsZero() {
		t.Errorf("Container 102 should have DockerCheckedAt set")
	}

	// Stopped container should not be checked
	if ct3 != nil && !ct3.DockerCheckedAt.IsZero() {
		t.Errorf("Stopped container 103 should not have been checked")
	}

	// Verify only running containers were checked
	if len(checker.calls) != 2 {
		t.Errorf("Expected 2 Docker checks (running containers only), got %d", len(checker.calls))
	}
}

func TestCheckContainersForDocker_PreservesExistingStatus(t *testing.T) {
	// Create a monitor with state containing previously checked containers
	state := models.NewState()
	checkedTime := time.Now().Add(-30 * time.Minute)
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "already-checked",
			Node:            "node1",
			Status:          "running",
			HasDocker:       true,
			DockerCheckedAt: checkedTime,
			Uptime:          3600, // 1 hour uptime
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: false}, // Would return false if checked
	}
	monitor.SetDockerChecker(checker)

	// Same container, still running with same/higher uptime
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "already-checked",
			Node:   "node1",
			Status: "running",
			Uptime: 7200, // 2 hours uptime (no restart)
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should not have been re-checked
	if len(checker.calls) != 0 {
		t.Errorf("Expected 0 checks (should preserve existing status), got %d", len(checker.calls))
	}

	// Should preserve the previous Docker status
	if !result[0].HasDocker {
		t.Errorf("Container should still have cached HasDocker=true")
	}
	if result[0].DockerCheckedAt.IsZero() {
		t.Errorf("Container should have preserved DockerCheckedAt")
	}
}

func TestCheckContainersForDocker_RechecksOnRestart(t *testing.T) {
	// Create a monitor with state containing previously checked container
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "restarted-container",
			Node:            "node1",
			Status:          "running",
			HasDocker:       false,
			DockerCheckedAt: time.Now().Add(-1 * time.Hour),
			Uptime:          3600, // 1 hour uptime
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: true}, // Now has Docker after restart
	}
	monitor.SetDockerChecker(checker)

	// Same container but with reset uptime (restarted)
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "restarted-container",
			Node:   "node1",
			Status: "running",
			Uptime: 60, // Only 1 minute uptime - it restarted
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should have been re-checked because uptime reset
	if len(checker.calls) != 1 {
		t.Errorf("Expected 1 check (container restarted), got %d", len(checker.calls))
	}

	// Should have updated Docker status
	if !result[0].HasDocker {
		t.Errorf("Container should now have HasDocker=true after restart")
	}
}

func TestCheckContainersForDocker_RechecksWhenStarted(t *testing.T) {
	// Create a monitor with state containing previously stopped container
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "was-stopped",
			Node:            "node1",
			Status:          "stopped",
			HasDocker:       false,
			DockerCheckedAt: time.Now().Add(-1 * time.Hour),
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: true},
	}
	monitor.SetDockerChecker(checker)

	// Container is now running
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "was-stopped",
			Node:   "node1",
			Status: "running",
			Uptime: 60,
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should have been checked because it just started
	if len(checker.calls) != 1 {
		t.Errorf("Expected 1 check (container started), got %d", len(checker.calls))
	}

	if !result[0].HasDocker {
		t.Errorf("Container should have HasDocker=true after starting")
	}
}

func TestCheckContainersForDocker_NoCheckerConfigured(t *testing.T) {
	state := models.NewState()
	monitor := &Monitor{
		state: state,
	}
	// No checker configured

	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "test", Node: "node1", Status: "running"},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should return containers unchanged
	if len(result) != 1 {
		t.Errorf("Expected 1 container, got %d", len(result))
	}
	if result[0].HasDocker {
		t.Errorf("Container should not have HasDocker set without checker")
	}
}

func TestAgentDockerChecker_ParsesOutput(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		exitCode   int
		wantDocker bool
		wantErr    bool
	}{
		{
			name:       "docker socket exists",
			stdout:     "yes\n",
			exitCode:   0,
			wantDocker: true,
			wantErr:    false,
		},
		{
			name:       "docker socket does not exist",
			stdout:     "no\n",
			exitCode:   1,
			wantDocker: false,
			wantErr:    false,
		},
		{
			name:       "container not accessible",
			stdout:     "error: CT 101 is locked",
			exitCode:   1,
			wantDocker: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewAgentDockerChecker(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
				return tt.stdout, tt.exitCode, nil
			})

			hasDocker, err := checker.CheckDockerInContainer(context.Background(), "node1", 101)

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if hasDocker != tt.wantDocker {
				t.Errorf("Expected hasDocker=%v, got %v", tt.wantDocker, hasDocker)
			}
		})
	}
}

func TestState_GetContainers(t *testing.T) {
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{ID: "ct-1", VMID: 101, Name: "test1"},
		{ID: "ct-2", VMID: 102, Name: "test2"},
	})

	containers := state.GetContainers()

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}

	// Verify it's a copy (modifying shouldn't affect original)
	containers[0].Name = "modified"
	originalContainers := state.GetContainers()
	if originalContainers[0].Name == "modified" {
		t.Error("GetContainers should return a copy, not the original slice")
	}
}

func TestState_UpdateContainerDockerStatus(t *testing.T) {
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{ID: "ct-1", VMID: 101, Name: "test1"},
	})

	now := time.Now()
	updated := state.UpdateContainerDockerStatus("ct-1", true, now)

	if !updated {
		t.Error("UpdateContainerDockerStatus should return true for existing container")
	}

	containers := state.GetContainers()
	if !containers[0].HasDocker {
		t.Error("Container should have HasDocker=true")
	}
	if containers[0].DockerCheckedAt.IsZero() {
		t.Error("Container should have DockerCheckedAt set")
	}

	// Test non-existent container
	updated = state.UpdateContainerDockerStatus("non-existent", true, now)
	if updated {
		t.Error("UpdateContainerDockerStatus should return false for non-existent container")
	}
}

func TestContainerNeedsDockerCheck(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	tests := []struct {
		name       string
		container  models.Container
		previous   map[string]models.Container
		wantReason string
	}{
		{
			name:       "new container",
			container:  models.Container{ID: "ct-1", Status: "running"},
			previous:   map[string]models.Container{},
			wantReason: "new",
		},
		{
			name:      "first check - never checked before",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 3600},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600},
			},
			wantReason: "first_check",
		},
		{
			name:      "restarted - uptime lower",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 60},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600, DockerCheckedAt: time.Now()},
			},
			wantReason: "restarted",
		},
		{
			name:      "started - was stopped",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 60},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "stopped", DockerCheckedAt: time.Now()},
			},
			wantReason: "started",
		},
		{
			name:      "no check needed - same state",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600, DockerCheckedAt: time.Now()},
			},
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := monitor.containerNeedsDockerCheck(tt.container, tt.previous)
			if reason != tt.wantReason {
				t.Errorf("Expected reason %q, got %q", tt.wantReason, reason)
			}
		})
	}
}
