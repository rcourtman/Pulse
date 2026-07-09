package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
)

// failingOperatorStateStore simulates an audit store whose operator-state
// lookup errors (e.g. sqlite I/O failure) while every other store surface
// keeps working, so refusal audit records can still be asserted.
type failingOperatorStateStore struct {
	*unifiedresources.MemoryStore
}

func (s *failingOperatorStateStore) GetResourceOperatorState(string) (unifiedresources.ResourceOperatorState, bool, error) {
	return unifiedresources.ResourceOperatorState{}, false, errors.New("simulated operator-state lookup failure")
}

// installApprovedApproval seeds the global approval store with an approved,
// plan-less approval and returns its ID. Plan-less keeps the plan-drift
// check out of the way: these tests exercise the remediation-lock gate's
// human-approved branch, which keys off the approved decision record.
func installApprovedApproval(t *testing.T, id, command, targetType, targetID string) {
	t.Helper()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	req := &approval.ApprovalRequest{
		ID:         id,
		Command:    command,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetID,
		Context:    "test approval",
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Approve(id, "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
}

// TestExecuteCommandWithAuditFailsClosedOnLockLookupErrorWithoutApproval
// covers the autonomous posture (Patrol / assisted autonomy: no human
// approval on the dispatch) when the operator-state lookup errors. The
// broker cannot tell whether the operator set NeverAutoRemediate, so it
// must fail CLOSED — refuse the dispatch, surface "remediation lock state
// unknown", and leave a Failed audit record with the stable
// `remediation_lock_state_unknown:` prefix.
func TestExecuteCommandWithAuditFailsClosedOnLockLookupErrorWithoutApproval(t *testing.T) {
	actionStore := &failingOperatorStateStore{MemoryStore: unifiedresources.NewMemoryStore()}
	agentServer := &mockAgentServer{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	executor.SetAutonomousMode(true)

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-degraded",
		"", // no approval — autonomous dispatch
		false,
		"agent-degraded",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-degraded",
		},
		"pulse_patrol",
		"restart workload service",
	)
	if !errors.Is(err, ErrRemediationLockStateUnknown) {
		t.Fatalf("executeCommandWithAudit error = %v, want ErrRemediationLockStateUnknown", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on fail-closed refusal, got %#v", result)
	}
	agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)

	audits, auditErr := actionStore.GetActionAudits("agent-degraded", time.Time{}, 10)
	if auditErr != nil {
		t.Fatalf("GetActionAudits: %v", auditErr)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 refused audit record, got %d", len(audits))
	}
	refused := audits[0]
	if refused.State != unifiedresources.ActionStateFailed {
		t.Fatalf("audit state = %q, want %q", refused.State, unifiedresources.ActionStateFailed)
	}
	if refused.Result == nil || refused.Result.Success {
		t.Fatalf("expected Result.Success=false, got %#v", refused.Result)
	}
	if !strings.HasPrefix(refused.Result.ErrorMessage, "remediation_lock_state_unknown:") {
		t.Fatalf("expected ErrorMessage to start with remediation_lock_state_unknown:, got %q", refused.Result.ErrorMessage)
	}
}

// TestExecuteCommandWithAuditFailsClosedWithNilStoreWithoutApproval covers
// the degraded deployment where no audit store is wired at all (e.g. the
// sqlite store failed to initialize and the chat service continued with a
// warning). Autonomous write dispatches must fail closed rather than
// silently ignoring an operator lock the broker cannot read.
func TestExecuteCommandWithAuditFailsClosedWithNilStoreWithoutApproval(t *testing.T) {
	agentServer := &mockAgentServer{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer: agentServer,
		// No ActionAuditStore.
	})
	executor.SetAutonomousMode(true)

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-nostore",
		"",
		false,
		"agent-nostore",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-nostore",
		},
		"pulse_patrol",
		"restart workload service",
	)
	if !errors.Is(err, ErrRemediationLockStateUnknown) {
		t.Fatalf("executeCommandWithAudit error = %v, want ErrRemediationLockStateUnknown", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on fail-closed refusal, got %#v", result)
	}
	agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
}

// TestExecuteCommandWithAuditAllowsHumanApprovedDispatchOnLockLookupError
// pins the interactive posture: when the dispatch is backed by an approved
// human decision, an unknown lock state keeps the historical fail-open
// behavior — the operator explicitly signed off on the action, so a
// degraded operator-state lookup does not override their decision.
func TestExecuteCommandWithAuditAllowsHumanApprovedDispatchOnLockLookupError(t *testing.T) {
	actionStore := &failingOperatorStateStore{MemoryStore: unifiedresources.NewMemoryStore()}
	installApprovedApproval(t, "approval-degraded", "systemctl restart workload", "agent", "agent-degraded")

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-degraded", mock.Anything).
		Return(&agentexec.CommandResultPayload{Stdout: "OK", ExitCode: 0}, nil)

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-degraded",
		"approval-degraded",
		true,
		"agent-degraded",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-degraded",
		},
		"pulse_control",
		"restart workload service",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected human-approved dispatch to proceed; got nil result")
	}
	agentServer.AssertCalled(t, "ExecuteCommand", mock.Anything, "agent-degraded", mock.Anything)
}

// TestExecuteCommandWithAuditAllowsHumanApprovedDispatchWithNilStore covers
// the human-approved branch with no audit store wired: same fail-open
// posture as the lookup-error case.
func TestExecuteCommandWithAuditAllowsHumanApprovedDispatchWithNilStore(t *testing.T) {
	installApprovedApproval(t, "approval-nostore", "systemctl restart workload", "agent", "agent-nostore")

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-nostore", mock.Anything).
		Return(&agentexec.CommandResultPayload{Stdout: "OK", ExitCode: 0}, nil)

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer: agentServer,
		// No ActionAuditStore.
	})

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-nostore",
		"approval-nostore",
		true,
		"agent-nostore",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-nostore",
		},
		"pulse_control",
		"restart workload service",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected human-approved dispatch to proceed; got nil result")
	}
	agentServer.AssertCalled(t, "ExecuteCommand", mock.Anything, "agent-nostore", mock.Anything)
}

// TestExecuteNativeActionWithAuditRefusesWhenResourceIsRemediationLocked
// pins the lock gate on the native-provider dispatch path (e.g. TrueNAS
// app start/stop/restart), which previously skipped the remediation lock
// entirely. The operator's NeverAutoRemediate flag must gate native write
// dispatches identically to agent-command dispatches.
func TestExecuteNativeActionWithAuditRefusesWhenResourceIsRemediationLocked(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	if err := actionStore.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        "app-locked",
		NeverAutoRemediate: true,
		SetAt:              time.Now().UTC(),
		SetBy:              "operator:richard",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		ActionAuditStore: actionStore,
	})

	executed := false
	result, err := executor.executeNativeActionWithAudit(
		context.Background(),
		"pulse_control",
		"app-locked",
		"",
		false,
		map[string]any{"action": "restart"},
		"pulse_control",
		"restart app-container",
		func(context.Context) (*unifiedresources.ExecutionResult, error) {
			executed = true
			return &unifiedresources.ExecutionResult{Success: true}, nil
		},
	)
	if !errors.Is(err, unifiedresources.ErrResourceRemediationLocked) {
		t.Fatalf("executeNativeActionWithAudit error = %v, want ErrResourceRemediationLocked", err)
	}
	if executed {
		t.Fatal("native action executed despite operator remediation lock")
	}
	if result == nil || result.Success {
		t.Fatalf("expected failed ExecutionResult, got %#v", result)
	}
	if !strings.HasPrefix(result.ErrorMessage, "resource_remediation_locked:") {
		t.Fatalf("expected ErrorMessage to start with resource_remediation_locked:, got %q", result.ErrorMessage)
	}

	audits, auditErr := actionStore.GetActionAudits("app-locked", time.Time{}, 10)
	if auditErr != nil {
		t.Fatalf("GetActionAudits: %v", auditErr)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 refused audit record, got %d", len(audits))
	}
	if audits[0].State != unifiedresources.ActionStateFailed {
		t.Fatalf("audit state = %q, want %q", audits[0].State, unifiedresources.ActionStateFailed)
	}
}

// TestExecuteNativeActionWithAuditFailsClosedOnUnknownLockStateWithoutApproval
// covers both unknown-state shapes on the native path: nil store and a
// store whose operator-state lookup errors. Autonomous native dispatches
// must fail closed in both.
func TestExecuteNativeActionWithAuditFailsClosedOnUnknownLockStateWithoutApproval(t *testing.T) {
	cases := []struct {
		name  string
		store unifiedresources.ResourceStore
	}{
		{name: "nil store", store: nil},
		{name: "lookup error", store: &failingOperatorStateStore{MemoryStore: unifiedresources.NewMemoryStore()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewPulseToolExecutor(ExecutorConfig{
				ActionAuditStore: tc.store,
			})
			executor.SetAutonomousMode(true)

			executed := false
			result, err := executor.executeNativeActionWithAudit(
				context.Background(),
				"pulse_control",
				"app-degraded",
				"",
				false,
				map[string]any{"action": "restart"},
				"pulse_patrol",
				"restart app-container",
				func(context.Context) (*unifiedresources.ExecutionResult, error) {
					executed = true
					return &unifiedresources.ExecutionResult{Success: true}, nil
				},
			)
			if !errors.Is(err, ErrRemediationLockStateUnknown) {
				t.Fatalf("executeNativeActionWithAudit error = %v, want ErrRemediationLockStateUnknown", err)
			}
			if executed {
				t.Fatal("native action executed despite unknown remediation lock state")
			}
			if result == nil || result.Success {
				t.Fatalf("expected failed ExecutionResult, got %#v", result)
			}
			if !strings.HasPrefix(result.ErrorMessage, "remediation_lock_state_unknown:") {
				t.Fatalf("expected ErrorMessage to start with remediation_lock_state_unknown:, got %q", result.ErrorMessage)
			}
		})
	}
}

// TestExecuteNativeActionWithAuditAllowsHumanApprovedDispatchOnUnknownLockState
// pins the human-approved fail-open branch on the native path.
func TestExecuteNativeActionWithAuditAllowsHumanApprovedDispatchOnUnknownLockState(t *testing.T) {
	actionStore := &failingOperatorStateStore{MemoryStore: unifiedresources.NewMemoryStore()}
	installApprovedApproval(t, "approval-native", "truenas app restart myapp", "app-container", "app-degraded")

	executor := NewPulseToolExecutor(ExecutorConfig{
		ActionAuditStore: actionStore,
	})

	executed := false
	result, err := executor.executeNativeActionWithAudit(
		context.Background(),
		"pulse_control",
		"app-degraded",
		"approval-native",
		true,
		map[string]any{"action": "restart"},
		"pulse_control",
		"restart app-container",
		func(context.Context) (*unifiedresources.ExecutionResult, error) {
			executed = true
			return &unifiedresources.ExecutionResult{Success: true}, nil
		},
	)
	if err != nil {
		t.Fatalf("executeNativeActionWithAudit unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected human-approved native dispatch to proceed")
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful ExecutionResult, got %#v", result)
	}
}
