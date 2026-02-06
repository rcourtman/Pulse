package investigation

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFixVerifier implements FixVerifier for testing verification flow.
type stubFixVerifier struct {
	resolved bool
	err      error
	called   bool
}

func (s *stubFixVerifier) VerifyFixResolved(ctx context.Context, finding *Finding) (bool, error) {
	s.called = true
	return s.resolved, s.err
}

// TestLifecycle_FindingToFixVerified tests the full investigation lifecycle:
// Finding → Investigation → Fix Execution → Verification (success)
func TestLifecycle_FindingToFixVerified(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:           "f1",
		Key:          "high-cpu",
		Title:        "High CPU on web-01",
		Severity:     "critical",
		Category:     "performance",
		ResourceID:   "vm-100",
		ResourceName: "web-01",
		ResourceType: "vm",
		Description:  "CPU usage at 95%",
		Evidence:     "cpu=95.2%",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "Investigation complete.\nPROPOSED_FIX: systemctl restart app\nTARGET_HOST: web-01",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	executor := &stubCommandExecutor{output: "Service restarted", code: 0}
	verifier := &stubFixVerifier{resolved: true}

	config := DefaultConfig()
	config.VerificationDelay = 0 // Skip sleep in test
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)
	orchestrator.SetCommandExecutor(executor)
	orchestrator.SetFixVerifier(verifier)

	// Execute the full lifecycle
	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	// Verify the investigation reached the terminal state
	inv := store.GetLatestByFinding("f1")
	require.NotNil(t, inv)

	assert.Equal(t, StatusCompleted, inv.Status, "investigation should be completed")
	assert.Equal(t, OutcomeFixVerified, inv.Outcome, "outcome should be fix_verified")
	assert.NotNil(t, inv.ProposedFix, "should have a proposed fix")
	assert.Equal(t, "systemctl restart app", inv.ProposedFix.Commands[0])
	assert.Contains(t, inv.ProposedFix.Rationale, "Verification: Issue confirmed resolved")

	// Verify finding was updated
	assert.True(t, findings.updated, "findings store should have been updated")
	assert.Equal(t, string(OutcomeFixVerified), findings.finding.InvestigationOutcome)
	assert.Equal(t, string(StatusCompleted), findings.finding.InvestigationStatus)
	assert.NotNil(t, findings.finding.LastInvestigatedAt)

	// Verify verifier was actually called
	assert.True(t, verifier.called, "fix verifier should have been called")
}

// TestLifecycle_FindingToFixVerificationFailed tests fix that executes but verification
// determines the issue persists.
func TestLifecycle_FindingToFixVerificationFailed(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f2",
		Key:      "disk-full",
		Title:    "Disk Full",
		Severity: "warning",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "PROPOSED_FIX: echo ok\nTARGET_HOST: local",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	executor := &stubCommandExecutor{output: "done", code: 0}
	verifier := &stubFixVerifier{resolved: false} // Issue still present

	config := DefaultConfig()
	config.VerificationDelay = 0
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)
	orchestrator.SetCommandExecutor(executor)
	orchestrator.SetFixVerifier(verifier)

	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	inv := store.GetLatestByFinding("f2")
	require.NotNil(t, inv)
	assert.Equal(t, OutcomeFixVerificationFailed, inv.Outcome, "outcome should be fix_verification_failed")
	assert.Contains(t, inv.ProposedFix.Rationale, "Issue persists after fix execution")
}

// TestLifecycle_FindingToApproval tests the approval path — when autonomy is "controlled",
// the fix should be queued for approval rather than executed.
func TestLifecycle_FindingToApproval(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f3",
		Key:      "mem-leak",
		Title:    "Memory Leak",
		Severity: "critical",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "PROPOSED_FIX: systemctl restart leaky-app\nTARGET_HOST: db-01",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	approval := &stubApprovalStore{}
	executor := &stubCommandExecutor{output: "ok", code: 0}

	config := DefaultConfig()
	orchestrator := NewOrchestrator(chatService, store, findings, approval, config)
	orchestrator.SetCommandExecutor(executor)

	// Use "controlled" autonomy — should queue for approval, NOT auto-execute
	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "controlled")
	require.NoError(t, err)

	inv := store.GetLatestByFinding("f3")
	require.NotNil(t, inv)

	assert.Equal(t, OutcomeFixQueued, inv.Outcome, "should queue for approval in controlled mode")
	assert.True(t, approval.called, "approval store should have been called")
	assert.NotNil(t, inv.ProposedFix, "fix should be recorded on investigation")
	assert.Equal(t, "systemctl restart leaky-app", inv.ProposedFix.Commands[0])
}

// TestLifecycle_FindingToCannotFix tests the path where investigation determines
// the issue cannot be automatically fixed.
func TestLifecycle_FindingToCannotFix(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f4",
		Key:      "hardware-fail",
		Title:    "Hardware Failure",
		Severity: "critical",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "CANNOT_FIX: Requires physical hardware replacement",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	config := DefaultConfig()
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)

	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	inv := store.GetLatestByFinding("f4")
	require.NotNil(t, inv)
	assert.Equal(t, OutcomeCannotFix, inv.Outcome)
	assert.Equal(t, string(OutcomeCannotFix), findings.finding.InvestigationOutcome)
}

// TestLifecycle_TimeoutSetsOutcome tests that when investigation times out,
// the finding is correctly tagged with OutcomeTimedOut for faster retry.
func TestLifecycle_TimeoutSetsOutcome(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f5",
		Key:      "slow-query",
		Title:    "Slow Queries",
		Severity: "warning",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			// Simulate a timeout by returning DeadlineExceeded
			return context.DeadlineExceeded
		},
	}

	config := DefaultConfig()
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)

	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "investigation failed")

	// Verify the finding was tagged with timed_out outcome
	assert.Equal(t, string(StatusFailed), findings.finding.InvestigationStatus)
	assert.Equal(t, string(OutcomeTimedOut), findings.finding.InvestigationOutcome)
	assert.NotNil(t, findings.finding.LastInvestigatedAt)

	// Verify the investigation session also has the outcome
	inv := store.GetLatestByFinding("f5")
	require.NotNil(t, inv)
	assert.Equal(t, StatusFailed, inv.Status)
	assert.Equal(t, OutcomeTimedOut, inv.Outcome)
}

// TestLifecycle_FixExecutionFails tests the path where fix command returns non-zero.
func TestLifecycle_FixExecutionFails(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f6",
		Key:      "stuck-service",
		Title:    "Service Not Responding",
		Severity: "warning",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "PROPOSED_FIX: systemctl restart stuck-service\nTARGET_HOST: local",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	executor := &stubCommandExecutor{output: "Failed to restart: unit not found", code: 1}

	config := DefaultConfig()
	config.VerificationDelay = 0
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)
	orchestrator.SetCommandExecutor(executor)

	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	inv := store.GetLatestByFinding("f6")
	require.NotNil(t, inv)
	assert.Equal(t, OutcomeFixFailed, inv.Outcome)
	assert.Contains(t, inv.ProposedFix.Rationale, "exit code 1")
}

// TestLifecycle_ReinvestigateAfterTimeout verifies that a finding that timed out
// can be reinvestigated and succeed on the second attempt.
func TestLifecycle_ReinvestigateAfterTimeout(t *testing.T) {
	store := NewStore("")
	finding := &Finding{
		ID:       "f7",
		Key:      "intermittent",
		Title:    "Intermittent Issue",
		Severity: "warning",
	}
	findings := &stubFindingsStore{finding: finding}

	callCount := 0
	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			callCount++
			if callCount == 1 {
				// First attempt: timeout
				return context.DeadlineExceeded
			}
			// Second attempt: success
			payload, _ := json.Marshal(map[string]string{
				"text": "CANNOT_FIX: Transient issue already resolved",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	config := DefaultConfig()
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)

	// First attempt — times out
	err := orchestrator.InvestigateFinding(context.Background(), finding, "full")
	require.Error(t, err)
	assert.Equal(t, string(OutcomeTimedOut), finding.InvestigationOutcome)
	assert.Equal(t, 1, finding.InvestigationAttempts)

	// Simulate cooldown elapsed — reset LastInvestigatedAt
	past := time.Now().Add(-15 * time.Minute)
	finding.LastInvestigatedAt = &past
	finding.InvestigationStatus = "" // Clear so orchestrator accepts it

	// Second attempt — succeeds
	err = orchestrator.InvestigateFinding(context.Background(), finding, "full")
	require.NoError(t, err)
	assert.Equal(t, string(OutcomeCannotFix), finding.InvestigationOutcome)
	assert.Equal(t, 2, finding.InvestigationAttempts)
}

// TestLifecycle_VerificationError tests verification returning an error.
func TestLifecycle_VerificationError(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{
		ID:       "f8",
		Key:      "net-issue",
		Title:    "Network Connectivity",
		Severity: "warning",
	}}

	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{
				"text": "PROPOSED_FIX: ip route flush cache\nTARGET_HOST: local",
			})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	executor := &stubCommandExecutor{output: "ok", code: 0}
	verifier := &stubFixVerifier{err: fmt.Errorf("verification patrol failed: circuit breaker open")}

	config := DefaultConfig()
	config.VerificationDelay = 0
	orchestrator := NewOrchestrator(chatService, store, findings, nil, config)
	orchestrator.SetCommandExecutor(executor)
	orchestrator.SetFixVerifier(verifier)

	err := orchestrator.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	inv := store.GetLatestByFinding("f8")
	require.NotNil(t, inv)
	assert.Equal(t, OutcomeFixVerificationFailed, inv.Outcome)
	assert.Contains(t, inv.ProposedFix.Rationale, "Verification error")
}
