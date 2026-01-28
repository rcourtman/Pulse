package chat

import (
	"testing"
)

func TestFSM_InitialState(t *testing.T) {
	fsm := NewSessionFSM()
	if fsm.State != StateResolving {
		t.Errorf("Initial state = %s, want %s", fsm.State, StateResolving)
	}
}

func TestFSM_WriteBlockedInResolving(t *testing.T) {
	fsm := NewSessionFSM()

	// Write should be blocked in RESOLVING state
	err := fsm.CanExecuteTool(ToolKindWrite, "pulse_control")
	if err == nil {
		t.Error("Write should be blocked in RESOLVING state")
	}

	fsmErr, ok := err.(*FSMBlockedError)
	if !ok {
		t.Fatalf("Expected FSMBlockedError, got %T", err)
	}
	if !fsmErr.Recoverable {
		t.Error("Error should be recoverable")
	}

	// Read should be allowed
	err = fsm.CanExecuteTool(ToolKindRead, "pulse_metrics")
	if err != nil {
		t.Errorf("Read should be allowed in RESOLVING: %v", err)
	}

	// Resolve should be allowed
	err = fsm.CanExecuteTool(ToolKindResolve, "pulse_query")
	if err != nil {
		t.Errorf("Resolve should be allowed in RESOLVING: %v", err)
	}
}

func TestFSM_WriteCausesVerifying(t *testing.T) {
	fsm := NewSessionFSM()

	// Transition to READING via a resolve
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	if fsm.State != StateReading {
		t.Errorf("State after resolve = %s, want %s", fsm.State, StateReading)
	}

	// Execute a write
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")

	// State should be VERIFYING
	if fsm.State != StateVerifying {
		t.Errorf("State after write = %s, want %s", fsm.State, StateVerifying)
	}

	// Flags should be set correctly
	if !fsm.WroteThisEpisode {
		t.Error("WroteThisEpisode should be true")
	}
	if fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be false after write")
	}
	if fsm.LastWriteTool != "pulse_control" {
		t.Errorf("LastWriteTool = %s, want pulse_control", fsm.LastWriteTool)
	}
}

func TestFSM_FinalAnswerBlockedInVerifying(t *testing.T) {
	fsm := NewSessionFSM()

	// Transition to READING then VERIFYING
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")

	// Final answer should be blocked
	err := fsm.CanFinalAnswer()
	if err == nil {
		t.Error("Final answer should be blocked in VERIFYING without read")
	}

	// A read should clear the block
	fsm.OnToolSuccess(ToolKindRead, "pulse_metrics")

	if !fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be true after read")
	}

	// Final answer should now be allowed
	err = fsm.CanFinalAnswer()
	if err != nil {
		t.Errorf("Final answer should be allowed after verification read: %v", err)
	}
}

func TestFSM_ReadAfterWriteClearsVerification(t *testing.T) {
	fsm := NewSessionFSM()

	// Transition through states
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")

	// Verify state
	if fsm.State != StateVerifying {
		t.Fatalf("State = %s, want %s", fsm.State, StateVerifying)
	}

	// Read should set ReadAfterWrite
	fsm.OnToolSuccess(ToolKindRead, "pulse_metrics")

	if !fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be true")
	}

	// Complete verification transitions back to READING
	fsm.CompleteVerification()

	if fsm.State != StateReading {
		t.Errorf("State after verification = %s, want %s", fsm.State, StateReading)
	}
}

func TestFSM_WriteBlockedInVerifyingWithoutRead(t *testing.T) {
	fsm := NewSessionFSM()

	// Transition to VERIFYING
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")

	// Another write should be blocked in VERIFYING
	err := fsm.CanExecuteTool(ToolKindWrite, "pulse_docker")
	if err == nil {
		t.Error("Write should be blocked in VERIFYING until verification read")
	}

	// Read is allowed
	err = fsm.CanExecuteTool(ToolKindRead, "pulse_metrics")
	if err != nil {
		t.Errorf("Read should be allowed in VERIFYING: %v", err)
	}

	// After read, complete verification
	fsm.OnToolSuccess(ToolKindRead, "pulse_metrics")
	fsm.CompleteVerification()

	// Now write should be allowed
	err = fsm.CanExecuteTool(ToolKindWrite, "pulse_docker")
	if err != nil {
		t.Errorf("Write should be allowed after verification: %v", err)
	}
}

func TestFSM_Reset(t *testing.T) {
	fsm := NewSessionFSM()

	// Build up some state
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")
	fsm.OnToolSuccess(ToolKindRead, "pulse_metrics")

	// Reset
	fsm.Reset()

	// Should be back to initial state
	if fsm.State != StateResolving {
		t.Errorf("State after reset = %s, want %s", fsm.State, StateResolving)
	}
	if fsm.WroteThisEpisode {
		t.Error("WroteThisEpisode should be false after reset")
	}
	if fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be false after reset")
	}
}

func TestFSM_ResetKeepProgress(t *testing.T) {
	fsm := NewSessionFSM()

	// Build up to VERIFYING
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")

	// Reset keeping progress
	fsm.ResetKeepProgress()

	// Should transition from VERIFYING to READING
	if fsm.State != StateReading {
		t.Errorf("State after ResetKeepProgress = %s, want %s", fsm.State, StateReading)
	}
	if fsm.WroteThisEpisode {
		t.Error("WroteThisEpisode should be false after ResetKeepProgress")
	}
}

func TestClassifyToolCall(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		expected ToolKind
	}{
		// Resolve tools
		{"pulse_query", "pulse_query", nil, ToolKindResolve},
		{"pulse_discovery", "pulse_discovery", nil, ToolKindResolve},
		{"pulse_search_resources", "pulse_search_resources", nil, ToolKindResolve},

		// Read tools
		{"pulse_metrics", "pulse_metrics", nil, ToolKindRead},
		{"pulse_storage", "pulse_storage", nil, ToolKindRead},
		{"pulse_kubernetes", "pulse_kubernetes", nil, ToolKindRead},
		{"pulse_pmg", "pulse_pmg", nil, ToolKindRead},
		{"pulse_alerts list", "pulse_alerts", map[string]interface{}{"action": "list"}, ToolKindRead},

		// pulse_read - ALWAYS read, regardless of action (read-only enforced at tool layer)
		{"pulse_read exec", "pulse_read", map[string]interface{}{"action": "exec"}, ToolKindRead},
		{"pulse_read file", "pulse_read", map[string]interface{}{"action": "file"}, ToolKindRead},
		{"pulse_read find", "pulse_read", map[string]interface{}{"action": "find"}, ToolKindRead},
		{"pulse_read tail", "pulse_read", map[string]interface{}{"action": "tail"}, ToolKindRead},
		{"pulse_read logs", "pulse_read", map[string]interface{}{"action": "logs"}, ToolKindRead},
		{"pulse_read no action", "pulse_read", nil, ToolKindRead},

		// Write tools
		{"pulse_control", "pulse_control", nil, ToolKindWrite},
		{"pulse_run_command", "pulse_run_command", nil, ToolKindWrite},
		{"pulse_control_guest", "pulse_control_guest", nil, ToolKindWrite},
		{"pulse_control_docker", "pulse_control_docker", nil, ToolKindWrite},
		{"pulse_alerts resolve", "pulse_alerts", map[string]interface{}{"action": "resolve"}, ToolKindWrite},

		// Docker - depends on action
		{"pulse_docker read", "pulse_docker", map[string]interface{}{"action": "services"}, ToolKindRead},
		{"pulse_docker control", "pulse_docker", map[string]interface{}{"action": "control"}, ToolKindWrite},
		{"pulse_docker update", "pulse_docker", map[string]interface{}{"action": "update"}, ToolKindWrite},

		// File edit - depends on action
		{"pulse_file_edit read", "pulse_file_edit", map[string]interface{}{"action": "read"}, ToolKindRead},
		{"pulse_file_edit write", "pulse_file_edit", map[string]interface{}{"action": "write"}, ToolKindWrite},
		{"pulse_file_edit append", "pulse_file_edit", map[string]interface{}{"action": "append"}, ToolKindWrite},

		// Knowledge - depends on action
		{"pulse_knowledge recall", "pulse_knowledge", map[string]interface{}{"action": "recall"}, ToolKindRead},
		{"pulse_knowledge remember", "pulse_knowledge", map[string]interface{}{"action": "remember"}, ToolKindWrite},

		// Unknown tool defaults to write (security-safe: requires discovery first)
		{"unknown_tool", "some_new_tool", nil, ToolKindWrite},

		// Action parameter fallback
		{"generic restart", "some_tool", map[string]interface{}{"action": "restart"}, ToolKindWrite},
		{"generic stop", "some_tool", map[string]interface{}{"action": "stop"}, ToolKindWrite},
		{"operation delete", "some_tool", map[string]interface{}{"operation": "delete"}, ToolKindWrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyToolCall(tt.toolName, tt.args)
			if got != tt.expected {
				t.Errorf("ClassifyToolCall(%q, %v) = %s, want %s", tt.toolName, tt.args, got, tt.expected)
			}
		})
	}
}

func TestFSM_TransitionFromResolving(t *testing.T) {
	// Test that any read or resolve transitions out of RESOLVING
	tests := []struct {
		name string
		kind ToolKind
	}{
		{"resolve", ToolKindResolve},
		{"read", ToolKindRead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewSessionFSM()
			fsm.OnToolSuccess(tt.kind, "test_tool")

			if fsm.State != StateReading {
				t.Errorf("State after %s = %s, want %s", tt.kind, fsm.State, StateReading)
			}
		})
	}
}

func TestFSM_RecoveryTracking(t *testing.T) {
	fsm := NewSessionFSM()

	// Track a pending recovery
	recoveryID := fsm.TrackPendingRecovery("FSM_BLOCKED", "pulse_control")
	if recoveryID == "" {
		t.Error("TrackPendingRecovery should return a recovery ID")
	}

	// Should have one pending recovery
	if len(fsm.PendingRecoveries) != 1 {
		t.Errorf("Expected 1 pending recovery, got %d", len(fsm.PendingRecoveries))
	}

	// Check recovery success for wrong tool - should return nil
	pr := fsm.CheckRecoverySuccess("pulse_docker")
	if pr != nil {
		t.Error("CheckRecoverySuccess should return nil for different tool")
	}

	// Check recovery success for correct tool - should return the recovery
	pr = fsm.CheckRecoverySuccess("pulse_control")
	if pr == nil {
		t.Error("CheckRecoverySuccess should return the pending recovery")
	}
	if pr.ErrorCode != "FSM_BLOCKED" {
		t.Errorf("ErrorCode = %s, want FSM_BLOCKED", pr.ErrorCode)
	}
	if pr.Tool != "pulse_control" {
		t.Errorf("Tool = %s, want pulse_control", pr.Tool)
	}

	// Should be removed after check
	if len(fsm.PendingRecoveries) != 0 {
		t.Errorf("Expected 0 pending recoveries after success, got %d", len(fsm.PendingRecoveries))
	}
}

func TestFSM_MultipleWritesCauseVerification(t *testing.T) {
	fsm := NewSessionFSM()

	// Get to READING
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")

	// First write
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")
	if fsm.State != StateVerifying {
		t.Fatalf("State after first write = %s, want %s", fsm.State, StateVerifying)
	}

	// Can't do another write in VERIFYING
	err := fsm.CanExecuteTool(ToolKindWrite, "another_write")
	if err == nil {
		t.Error("Should not allow consecutive writes without verification")
	}

	// Read to verify
	fsm.OnToolSuccess(ToolKindRead, "pulse_query")
	fsm.CompleteVerification()

	// Now another write is allowed
	err = fsm.CanExecuteTool(ToolKindWrite, "another_write")
	if err != nil {
		t.Errorf("Should allow write after verification: %v", err)
	}
}

func TestFSM_ReadToolNeverTriggersVerifying(t *testing.T) {
	// This test verifies that pulse_read (classified as ToolKindRead) NEVER
	// triggers VERIFYING state, even when executing commands.
	//
	// This is the fix for the bug where "grep logs" through pulse_control
	// was triggering VERIFYING state because pulse_control is classified as write.

	fsm := NewSessionFSM()

	// Get to READING state
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	if fsm.State != StateReading {
		t.Fatalf("Expected READING after resolve, got %s", fsm.State)
	}

	// Simulate multiple pulse_read calls (all classified as ToolKindRead)
	// None of these should trigger VERIFYING
	readTools := []string{"pulse_read", "pulse_metrics", "pulse_storage"}
	for _, tool := range readTools {
		fsm.OnToolSuccess(ToolKindRead, tool)
		if fsm.State != StateReading {
			t.Errorf("Expected READING after %s, got %s", tool, fsm.State)
		}
		if fsm.WroteThisEpisode {
			t.Errorf("WroteThisEpisode should be false after %s", tool)
		}
	}

	// Verify we can still do unlimited reads without VERIFYING
	for i := 0; i < 10; i++ {
		fsm.OnToolSuccess(ToolKindRead, "pulse_read")
	}
	if fsm.State != StateReading {
		t.Errorf("Expected READING after 10 reads, got %s", fsm.State)
	}

	// Only a WRITE should trigger VERIFYING
	fsm.OnToolSuccess(ToolKindWrite, "pulse_control")
	if fsm.State != StateVerifying {
		t.Errorf("Expected VERIFYING after write, got %s", fsm.State)
	}

	// Now reads should work to clear verification
	fsm.OnToolSuccess(ToolKindRead, "pulse_read")
	if !fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be true after read in VERIFYING")
	}
}

func TestFSM_PulseReadClassification(t *testing.T) {
	// Verify pulse_read is ALWAYS classified as Read regardless of action
	actions := []string{"exec", "file", "find", "tail", "logs", ""}
	for _, action := range actions {
		args := map[string]interface{}{}
		if action != "" {
			args["action"] = action
		}

		kind := ClassifyToolCall("pulse_read", args)
		if kind != ToolKindRead {
			t.Errorf("pulse_read action=%q: expected ToolKindRead, got %s", action, kind)
		}
	}
}

func TestFSM_RegressionJellyfinLogsScenario(t *testing.T) {
	// Regression test for the exact failure scenario from the Jellyfin transcript.
	//
	// BEFORE FIX (broken):
	//   1. User asks "what was last played in jellyfin"
	//   2. Model runs pulse_control type=command to grep logs
	//   3. FSM enters VERIFYING because pulse_control is classified as WRITE
	//   4. Model blocked from running more commands
	//
	// AFTER FIX (working):
	//   1. User asks "what was last played in jellyfin"
	//   2. Model runs pulse_read action=exec to grep logs
	//   3. FSM stays in READING because pulse_read is classified as READ
	//   4. Model can run unlimited read operations

	fsm := NewSessionFSM()

	// Step 1: Discovery (RESOLVING → READING)
	fsm.OnToolSuccess(ToolKindResolve, "pulse_discovery")
	if fsm.State != StateReading {
		t.Fatalf("After discovery: expected READING, got %s", fsm.State)
	}

	// Step 2: List log files with pulse_read exec
	kind := ClassifyToolCall("pulse_read", map[string]interface{}{"action": "exec"})
	if kind != ToolKindRead {
		t.Fatalf("pulse_read exec should be ToolKindRead, got %s", kind)
	}
	fsm.OnToolSuccess(kind, "pulse_read")
	if fsm.State != StateReading {
		t.Errorf("After pulse_read exec: expected READING, got %s", fsm.State)
	}

	// Step 3: Tail log file with pulse_read tail
	kind = ClassifyToolCall("pulse_read", map[string]interface{}{"action": "tail"})
	if kind != ToolKindRead {
		t.Fatalf("pulse_read tail should be ToolKindRead, got %s", kind)
	}
	fsm.OnToolSuccess(kind, "pulse_read")
	if fsm.State != StateReading {
		t.Errorf("After pulse_read tail: expected READING, got %s", fsm.State)
	}

	// Step 4: Read specific log file with pulse_read file
	kind = ClassifyToolCall("pulse_read", map[string]interface{}{"action": "file"})
	if kind != ToolKindRead {
		t.Fatalf("pulse_read file should be ToolKindRead, got %s", kind)
	}
	fsm.OnToolSuccess(kind, "pulse_read")
	if fsm.State != StateReading {
		t.Errorf("After pulse_read file: expected READING, got %s", fsm.State)
	}

	// Verify: we never entered VERIFYING, no write flags set
	if fsm.WroteThisEpisode {
		t.Error("WroteThisEpisode should be false - no writes performed")
	}
	if fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be false - no writes to verify")
	}

	// Contrast: if we had used pulse_control (the old broken path)
	fsmBroken := NewSessionFSM()
	fsmBroken.OnToolSuccess(ToolKindResolve, "pulse_discovery")
	brokenKind := ClassifyToolCall("pulse_control", map[string]interface{}{"type": "command"})
	if brokenKind != ToolKindWrite {
		t.Fatalf("pulse_control command should be ToolKindWrite, got %s", brokenKind)
	}
	fsmBroken.OnToolSuccess(brokenKind, "pulse_control")
	if fsmBroken.State != StateVerifying {
		t.Errorf("pulse_control should trigger VERIFYING, got %s", fsmBroken.State)
	}
}

func TestFSM_PulseControlClassification(t *testing.T) {
	// Verify pulse_control is ALWAYS classified as Write
	// This is important: even "read-like" commands through pulse_control
	// are classified as write, which is why we need pulse_read
	actions := []string{"guest", "command", ""}
	for _, action := range actions {
		args := map[string]interface{}{}
		if action != "" {
			args["type"] = action
		}

		kind := ClassifyToolCall("pulse_control", args)
		if kind != ToolKindWrite {
			t.Errorf("pulse_control type=%q: expected ToolKindWrite, got %s", action, kind)
		}
	}
}

func TestFSM_RegressionWriteReadWriteSequence(t *testing.T) {
	// Regression test for the bug where FSM stayed stuck in VERIFYING after reads.
	//
	// BEFORE FIX (broken):
	//   1. Model does pulse_file_edit action=write → FSM enters VERIFYING
	//   2. Model does pulse_read action=exec → FSM sets ReadAfterWrite=true but stays VERIFYING
	//   3. Model tries pulse_docker action=control → BLOCKED because still in VERIFYING
	//
	// AFTER FIX (working):
	//   1. Model does pulse_file_edit action=write → FSM enters VERIFYING
	//   2. Model does pulse_read action=exec → FSM sets ReadAfterWrite=true AND transitions to READING
	//   3. Model tries pulse_docker action=control → ALLOWED because in READING
	//
	// The fix is calling CompleteVerification() immediately after OnToolSuccess()
	// when we're in VERIFYING and ReadAfterWrite becomes true.

	// Simulate the agentic loop behavior with the fix
	fsm := NewSessionFSM()

	// Step 1: Discovery (RESOLVING → READING)
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")
	if fsm.State != StateReading {
		t.Fatalf("After discovery: expected READING, got %s", fsm.State)
	}

	// Step 2: Write operation (READING → VERIFYING)
	fsm.OnToolSuccess(ToolKindWrite, "pulse_file_edit")
	if fsm.State != StateVerifying {
		t.Fatalf("After write: expected VERIFYING, got %s", fsm.State)
	}
	if fsm.ReadAfterWrite {
		t.Error("ReadAfterWrite should be false immediately after write")
	}

	// Step 3: Read operation in VERIFYING state
	// This simulates what the agentic loop does AFTER THE FIX:
	// - Call OnToolSuccess (sets ReadAfterWrite = true)
	// - Immediately call CompleteVerification if ReadAfterWrite is true
	fsm.OnToolSuccess(ToolKindRead, "pulse_read")
	if !fsm.ReadAfterWrite {
		t.Fatal("ReadAfterWrite should be true after read in VERIFYING")
	}
	// THE FIX: Call CompleteVerification immediately after read success in VERIFYING
	if fsm.State == StateVerifying && fsm.ReadAfterWrite {
		fsm.CompleteVerification()
	}

	// Step 4: Verify we're back in READING, not stuck in VERIFYING
	if fsm.State != StateReading {
		t.Errorf("After read+CompleteVerification: expected READING, got %s", fsm.State)
	}

	// Step 5: Another write should now be allowed
	err := fsm.CanExecuteTool(ToolKindWrite, "pulse_docker")
	if err != nil {
		t.Errorf("Second write should be allowed after read verification: %v", err)
	}
}

func TestFSM_RegressionMultipleReadsAfterWrite(t *testing.T) {
	// Test that multiple reads after a write all work correctly
	// and subsequent writes are still allowed.

	fsm := NewSessionFSM()

	// Get to READING state
	fsm.OnToolSuccess(ToolKindResolve, "pulse_query")

	// First write
	fsm.OnToolSuccess(ToolKindWrite, "pulse_file_edit")
	if fsm.State != StateVerifying {
		t.Fatalf("Expected VERIFYING after write, got %s", fsm.State)
	}

	// Multiple reads in VERIFYING - each should set ReadAfterWrite and trigger completion
	for i := 0; i < 3; i++ {
		fsm.OnToolSuccess(ToolKindRead, "pulse_read")
		// Simulate the agentic loop fix
		if fsm.State == StateVerifying && fsm.ReadAfterWrite {
			fsm.CompleteVerification()
		}
	}

	// Should be in READING after all the reads
	if fsm.State != StateReading {
		t.Errorf("Expected READING after multiple reads, got %s", fsm.State)
	}

	// Second write should work
	err := fsm.CanExecuteTool(ToolKindWrite, "pulse_docker")
	if err != nil {
		t.Errorf("Second write should be allowed: %v", err)
	}

	// Execute the second write
	fsm.OnToolSuccess(ToolKindWrite, "pulse_docker")
	if fsm.State != StateVerifying {
		t.Fatalf("Expected VERIFYING after second write, got %s", fsm.State)
	}

	// Verify the second write, then third write should work
	fsm.OnToolSuccess(ToolKindRead, "pulse_query")
	if fsm.State == StateVerifying && fsm.ReadAfterWrite {
		fsm.CompleteVerification()
	}

	err = fsm.CanExecuteTool(ToolKindWrite, "pulse_control")
	if err != nil {
		t.Errorf("Third write should be allowed: %v", err)
	}
}
