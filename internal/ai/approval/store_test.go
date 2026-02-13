package approval

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
		MaxApprovals:   10,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewStore() returned nil store")
	}
}

func TestNewStoreEmptyDataDir(t *testing.T) {
	_, err := NewStore(StoreConfig{
		DataDir: "",
	})
	if err == nil {
		t.Fatal("NewStore() expected error for empty data dir")
	}
}

func TestNewStore_NonPositiveConfigUsesDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: -1 * time.Minute,
		MaxApprovals:   -10,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if store.defaultTimeout != 5*time.Minute {
		t.Fatalf("defaultTimeout = %v, want %v", store.defaultTimeout, 5*time.Minute)
	}
	if store.maxApprovals != 100 {
		t.Fatalf("maxApprovals = %d, want 100", store.maxApprovals)
	}

	req := &ApprovalRequest{Command: "echo ok"}
	if err := store.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}
	if !req.ExpiresAt.After(req.RequestedAt) {
		t.Fatal("approval expiry should be after request time")
	}
}

func TestCreateApproval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		ExecutionID: "exec-1",
		ToolID:      "tool-1",
		Command:     "systemctl restart nginx",
		TargetType:  "host",
		TargetID:    "host-1",
		TargetName:  "webserver",
		Context:     "Service needs restart due to config change",
	}

	err = store.CreateApproval(req)
	if err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}

	if req.ID == "" {
		t.Error("CreateApproval() did not set ID")
	}
	if req.Status != StatusPending {
		t.Errorf("CreateApproval() status = %v, want %v", req.Status, StatusPending)
	}
	if req.RequestedAt.IsZero() {
		t.Error("CreateApproval() did not set RequestedAt")
	}
	if req.ExpiresAt.IsZero() {
		t.Error("CreateApproval() did not set ExpiresAt")
	}
}

func TestGetApproval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "apt update",
	}
	store.CreateApproval(req)

	got, found := store.GetApproval(req.ID)
	if !found {
		t.Fatal("GetApproval() not found")
	}
	if got.Command != "apt update" {
		t.Errorf("GetApproval() command = %v, want %v", got.Command, "apt update")
	}

	_, found = store.GetApproval("nonexistent")
	if found {
		t.Error("GetApproval() found nonexistent approval")
	}
}

func TestGetPendingApprovals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	// Create multiple approvals
	for i := 0; i < 3; i++ {
		store.CreateApproval(&ApprovalRequest{
			Command: "test command",
		})
	}

	pending := store.GetPendingApprovals()
	if len(pending) != 3 {
		t.Errorf("GetPendingApprovals() count = %v, want %v", len(pending), 3)
	}
}

func TestGetApprovalsByExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-1", Command: "cmd-1"})
	store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-1", Command: "cmd-2"})
	store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-2", Command: "cmd-3"})

	results := store.GetApprovalsByExecution("exec-1")
	if len(results) != 2 {
		t.Fatalf("GetApprovalsByExecution() count = %v, want %v", len(results), 2)
	}
	for _, req := range results {
		if req.ExecutionID != "exec-1" {
			t.Fatalf("GetApprovalsByExecution() returned wrong execution ID: %v", req.ExecutionID)
		}
	}
}

func TestApprove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "systemctl restart nginx",
	}
	store.CreateApproval(req)

	got, err := store.Approve(req.ID, "admin")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if got.Status != StatusApproved {
		t.Errorf("Approve() status = %v, want %v", got.Status, StatusApproved)
	}
	if got.DecidedBy != "admin" {
		t.Errorf("Approve() DecidedBy = %v, want %v", got.DecidedBy, "admin")
	}
	if got.DecidedAt == nil {
		t.Error("Approve() did not set DecidedAt")
	}
}

func TestDeny(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "rm -rf /tmp/data",
	}
	store.CreateApproval(req)

	got, err := store.Deny(req.ID, "admin", "Too dangerous")
	if err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	if got.Status != StatusDenied {
		t.Errorf("Deny() status = %v, want %v", got.Status, StatusDenied)
	}
	if got.DenyReason != "Too dangerous" {
		t.Errorf("Deny() DenyReason = %v, want %v", got.DenyReason, "Too dangerous")
	}
}

func TestGetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	pending := &ApprovalRequest{Command: "pending"}
	store.CreateApproval(pending)

	approved := &ApprovalRequest{Command: "approved"}
	store.CreateApproval(approved)
	if _, err := store.Approve(approved.ID, "admin"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	denied := &ApprovalRequest{Command: "denied"}
	store.CreateApproval(denied)
	if _, err := store.Deny(denied.ID, "admin", "no"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	expired := &ApprovalRequest{
		Command:   "expired",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	store.CreateApproval(expired)
	store.CleanupExpired()

	if err := store.StoreExecution(&ExecutionState{ID: "exec-1"}); err != nil {
		t.Fatalf("StoreExecution() error = %v", err)
	}

	stats := store.GetStats()
	if stats["pending"] != 1 || stats["approved"] != 1 || stats["denied"] != 1 || stats["expired"] != 1 {
		t.Fatalf("GetStats() unexpected approval counts: %+v", stats)
	}
	if stats["executions"] != 1 {
		t.Fatalf("GetStats() executions = %v, want %v", stats["executions"], 1)
	}
}

func TestApproveNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	_, err = store.Approve("nonexistent", "admin")
	if err == nil {
		t.Error("Approve() expected error for nonexistent approval")
	}
}

func TestApproveAlreadyDecided(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "test",
	}
	store.CreateApproval(req)
	_, _ = store.Deny(req.ID, "admin", "reason")

	_, err = store.Approve(req.ID, "admin2")
	if err == nil {
		t.Error("Approve() expected error for already decided approval")
	}
}

func TestExecutionState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state := &ExecutionState{
		ID: "state-1",
		OriginalRequest: map[string]interface{}{
			"message": "test",
		},
		Messages: []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}

	err = store.StoreExecution(state)
	if err != nil {
		t.Fatalf("StoreExecution() error = %v", err)
	}

	got, found := store.GetExecution(state.ID)
	if !found {
		t.Fatal("GetExecution() not found")
	}
	if got.ID != state.ID {
		t.Errorf("GetExecution() ID = %v, want %v", got.ID, state.ID)
	}

	_, found = store.GetExecution("nonexistent")
	if found {
		t.Error("GetExecution() found nonexistent state")
	}
}

func TestDeleteExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state := &ExecutionState{
		ID: "state-1",
	}
	store.StoreExecution(state)

	store.DeleteExecution(state.ID)

	_, found := store.GetExecution(state.ID)
	if found {
		t.Error("DeleteExecution() did not delete state")
	}
}

func TestCleanupExpired(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Millisecond, // Very short timeout
	})

	// Create an approval that will expire immediately
	req := &ApprovalRequest{
		Command: "test",
	}
	store.CreateApproval(req)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	cleaned := store.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("CleanupExpired() cleaned = %v, want %v", cleaned, 1)
	}

	got, found := store.GetApproval(req.ID)
	if !found {
		t.Fatal("Approval should still exist after cleanup")
	}
	if got.Status != StatusExpired {
		t.Errorf("CleanupExpired() status = %v, want %v", got.Status, StatusExpired)
	}
}

func TestStartCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanup(ctx)

	// Give cleanup goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel should stop the cleanup loop
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestAssessRiskLevel(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		targetType string
		want       RiskLevel
	}{
		{
			name:    "high risk rm -rf",
			command: "rm -rf /var/log",
			want:    RiskHigh,
		},
		{
			name:    "high risk dd",
			command: "dd if=/dev/zero of=/dev/sda",
			want:    RiskHigh,
		},
		{
			name:    "high risk apt purge",
			command: "apt purge nginx",
			want:    RiskHigh,
		},
		{
			name:    "medium risk service restart",
			command: "systemctl restart nginx",
			want:    RiskMedium,
		},
		{
			name:    "medium risk docker restart",
			command: "docker restart mycontainer",
			want:    RiskMedium,
		},
		{
			name:    "medium risk apt install",
			command: "apt install htop",
			want:    RiskMedium,
		},
		{
			name:    "low risk diagnostic",
			command: "df -h",
			want:    RiskLow,
		},
		{
			name:    "low risk logs",
			command: "journalctl -u nginx",
			want:    RiskLow,
		},
		{
			name:    "low risk status check",
			command: "systemctl status nginx",
			want:    RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessRiskLevel(tt.command, tt.targetType)
			if got != tt.want {
				t.Errorf("AssessRiskLevel(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and add data
	store1, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
	})

	req := &ApprovalRequest{
		Command:    "test command",
		TargetID:   "host-1",
		TargetName: "webserver",
	}
	store1.CreateApproval(req)
	approvalID := req.ID

	state := &ExecutionState{
		ID: "exec-1",
		OriginalRequest: map[string]interface{}{
			"message": "test",
		},
	}
	store1.StoreExecution(state)

	// Flush debounced writes to disk immediately
	store1.Flush()

	// Create new store from same directory
	store2, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
	})

	// Verify data was loaded
	loadedApproval, found := store2.GetApproval(approvalID)
	if !found {
		t.Fatal("Approval not persisted")
	}
	if loadedApproval.Command != "test command" {
		t.Errorf("Persisted approval command = %v, want %v", loadedApproval.Command, "test command")
	}

	loadedState, found := store2.GetExecution("exec-1")
	if !found {
		t.Fatal("Execution state not persisted")
	}
	if loadedState.ID != "exec-1" {
		t.Errorf("Persisted state ID = %v, want %v", loadedState.ID, "exec-1")
	}
}

func TestMaxApprovals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
		MaxApprovals:   2,
	})

	// Create first two approvals - should succeed
	store.CreateApproval(&ApprovalRequest{Command: "test1"})
	store.CreateApproval(&ApprovalRequest{Command: "test2"})

	// Third should fail
	err = store.CreateApproval(&ApprovalRequest{Command: "test3"})
	if err == nil {
		t.Error("CreateApproval() expected error when max approvals reached")
	}
}

func TestGlobalStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	SetStore(store)
	got := GetStore()

	if got != store {
		t.Error("GetStore() did not return the set store")
	}

	// Reset global store
	SetStore(nil)
}
