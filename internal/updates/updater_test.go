package updates

import (
	"context"
	"errors"
	"testing"
)

// mockUpdater implements the Updater interface for testing
type mockUpdater struct {
	deploymentType string
	supportsApply  bool
	prepareErr     error
	executeErr     error
	rollbackErr    error
}

func (m *mockUpdater) SupportsApply() bool {
	return m.supportsApply
}

func (m *mockUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	if m.prepareErr != nil {
		return nil, m.prepareErr
	}
	return &UpdatePlan{
		CanAutoUpdate: m.supportsApply,
	}, nil
}

func (m *mockUpdater) Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error {
	return m.executeErr
}

func (m *mockUpdater) Rollback(ctx context.Context, eventID string) error {
	return m.rollbackErr
}

func (m *mockUpdater) GetDeploymentType() string {
	return m.deploymentType
}

func TestNewUpdaterRegistry(t *testing.T) {
	registry := NewUpdaterRegistry()
	if registry == nil {
		t.Fatal("NewUpdaterRegistry() returned nil")
	}
	if registry.updaters == nil {
		t.Fatal("NewUpdaterRegistry() did not initialize updaters map")
	}
}

func TestUpdaterRegistry_Register(t *testing.T) {
	registry := NewUpdaterRegistry()
	updater := &mockUpdater{deploymentType: "docker"}

	registry.Register("docker", updater)

	// Verify the updater was registered
	got, err := registry.Get("docker")
	if err != nil {
		t.Fatalf("Get() after Register() failed: %v", err)
	}
	if got != updater {
		t.Error("Get() returned different updater than registered")
	}
}

func TestUpdaterRegistry_Register_Overwrites(t *testing.T) {
	registry := NewUpdaterRegistry()
	updater1 := &mockUpdater{deploymentType: "docker", supportsApply: false}
	updater2 := &mockUpdater{deploymentType: "docker", supportsApply: true}

	registry.Register("docker", updater1)
	registry.Register("docker", updater2)

	got, err := registry.Get("docker")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got != updater2 {
		t.Error("Register() did not overwrite previous updater")
	}
	if !got.SupportsApply() {
		t.Error("Expected second updater with SupportsApply=true")
	}
}

func TestUpdaterRegistry_Get_NotFound(t *testing.T) {
	registry := NewUpdaterRegistry()

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() should return error for unregistered deployment type")
	}

	expectedMsg := "no updater registered for deployment type: nonexistent"
	if err.Error() != expectedMsg {
		t.Errorf("Get() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestUpdaterRegistry_MultipleDeploymentTypes(t *testing.T) {
	registry := NewUpdaterRegistry()

	dockerUpdater := &mockUpdater{deploymentType: "docker"}
	installshUpdater := &mockUpdater{deploymentType: "install.sh"}
	aurUpdater := &mockUpdater{deploymentType: "aur"}

	registry.Register("docker", dockerUpdater)
	registry.Register("install.sh", installshUpdater)
	registry.Register("aur", aurUpdater)

	tests := []struct {
		deploymentType string
		expectedType   string
	}{
		{"docker", "docker"},
		{"install.sh", "install.sh"},
		{"aur", "aur"},
	}

	for _, tc := range tests {
		t.Run(tc.deploymentType, func(t *testing.T) {
			got, err := registry.Get(tc.deploymentType)
			if err != nil {
				t.Fatalf("Get(%q) failed: %v", tc.deploymentType, err)
			}
			if got.GetDeploymentType() != tc.expectedType {
				t.Errorf("GetDeploymentType() = %q, want %q", got.GetDeploymentType(), tc.expectedType)
			}
		})
	}
}

func TestUpdateRequest_Fields(t *testing.T) {
	req := UpdateRequest{
		Version: "1.2.3",
		Channel: "stable",
		Force:   true,
		DryRun:  false,
	}

	if req.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", req.Version, "1.2.3")
	}
	if req.Channel != "stable" {
		t.Errorf("Channel = %q, want %q", req.Channel, "stable")
	}
	if !req.Force {
		t.Error("Force should be true")
	}
	if req.DryRun {
		t.Error("DryRun should be false")
	}
}

func TestUpdatePlan_Fields(t *testing.T) {
	plan := UpdatePlan{
		CanAutoUpdate:   true,
		Instructions:    []string{"step1", "step2"},
		Prerequisites:   []string{"prereq1"},
		EstimatedTime:   "5 minutes",
		RequiresRoot:    true,
		RollbackSupport: true,
		DownloadURL:     "https://example.com/download",
	}

	if !plan.CanAutoUpdate {
		t.Error("CanAutoUpdate should be true")
	}
	if len(plan.Instructions) != 2 {
		t.Errorf("Instructions length = %d, want 2", len(plan.Instructions))
	}
	if len(plan.Prerequisites) != 1 {
		t.Errorf("Prerequisites length = %d, want 1", len(plan.Prerequisites))
	}
	if plan.EstimatedTime != "5 minutes" {
		t.Errorf("EstimatedTime = %q, want %q", plan.EstimatedTime, "5 minutes")
	}
	if !plan.RequiresRoot {
		t.Error("RequiresRoot should be true")
	}
	if !plan.RollbackSupport {
		t.Error("RollbackSupport should be true")
	}
	if plan.DownloadURL != "https://example.com/download" {
		t.Errorf("DownloadURL = %q, want %q", plan.DownloadURL, "https://example.com/download")
	}
}

func TestUpdateProgress_Fields(t *testing.T) {
	progress := UpdateProgress{
		Stage:      "downloading",
		Progress:   50,
		Message:    "Downloading update...",
		IsComplete: false,
		Error:      "",
	}

	if progress.Stage != "downloading" {
		t.Errorf("Stage = %q, want %q", progress.Stage, "downloading")
	}
	if progress.Progress != 50 {
		t.Errorf("Progress = %d, want 50", progress.Progress)
	}
	if progress.Message != "Downloading update..." {
		t.Errorf("Message = %q, want %q", progress.Message, "Downloading update...")
	}
	if progress.IsComplete {
		t.Error("IsComplete should be false")
	}
	if progress.Error != "" {
		t.Errorf("Error = %q, want empty string", progress.Error)
	}
}

func TestUpdateProgress_WithError(t *testing.T) {
	progress := UpdateProgress{
		Stage:      "failed",
		Progress:   0,
		Message:    "Update failed",
		IsComplete: true,
		Error:      "connection timeout",
	}

	if !progress.IsComplete {
		t.Error("IsComplete should be true for failed state")
	}
	if progress.Error != "connection timeout" {
		t.Errorf("Error = %q, want %q", progress.Error, "connection timeout")
	}
}

func TestMockUpdater_SupportsApply(t *testing.T) {
	tests := []struct {
		name     string
		supports bool
	}{
		{"supports apply", true},
		{"does not support apply", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updater := &mockUpdater{supportsApply: tc.supports}
			if updater.SupportsApply() != tc.supports {
				t.Errorf("SupportsApply() = %v, want %v", updater.SupportsApply(), tc.supports)
			}
		})
	}
}

func TestMockUpdater_PrepareUpdate(t *testing.T) {
	ctx := context.Background()
	req := UpdateRequest{Version: "1.0.0"}

	t.Run("success", func(t *testing.T) {
		updater := &mockUpdater{supportsApply: true}
		plan, err := updater.PrepareUpdate(ctx, req)
		if err != nil {
			t.Fatalf("PrepareUpdate() failed: %v", err)
		}
		if !plan.CanAutoUpdate {
			t.Error("CanAutoUpdate should be true")
		}
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("prepare failed")
		updater := &mockUpdater{prepareErr: expectedErr}
		_, err := updater.PrepareUpdate(ctx, req)
		if err != expectedErr {
			t.Errorf("PrepareUpdate() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestMockUpdater_Execute(t *testing.T) {
	ctx := context.Background()
	req := UpdateRequest{Version: "1.0.0"}
	progressCb := func(p UpdateProgress) {}

	t.Run("success", func(t *testing.T) {
		updater := &mockUpdater{}
		err := updater.Execute(ctx, req, progressCb)
		if err != nil {
			t.Errorf("Execute() error = %v, want nil", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("execute failed")
		updater := &mockUpdater{executeErr: expectedErr}
		err := updater.Execute(ctx, req, progressCb)
		if err != expectedErr {
			t.Errorf("Execute() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestMockUpdater_Rollback(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		updater := &mockUpdater{}
		err := updater.Rollback(ctx, "event-123")
		if err != nil {
			t.Errorf("Rollback() error = %v, want nil", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("rollback failed")
		updater := &mockUpdater{rollbackErr: expectedErr}
		err := updater.Rollback(ctx, "event-123")
		if err != expectedErr {
			t.Errorf("Rollback() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestMockUpdater_GetDeploymentType(t *testing.T) {
	updater := &mockUpdater{deploymentType: "custom-type"}
	if got := updater.GetDeploymentType(); got != "custom-type" {
		t.Errorf("GetDeploymentType() = %q, want %q", got, "custom-type")
	}
}
