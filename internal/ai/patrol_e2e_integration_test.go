package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Stubs for investigation.ChatService (different from patrol's ChatService) ---

type e2eInvestigationChatService struct {
	executeFunc func(investigation.StreamCallback) error
}

func (s *e2eInvestigationChatService) CreateSession(_ context.Context) (*investigation.Session, error) {
	return &investigation.Session{ID: "inv-session-1"}, nil
}

func (s *e2eInvestigationChatService) ExecuteStream(_ context.Context, _ investigation.ExecuteRequest, callback investigation.StreamCallback) error {
	if s.executeFunc != nil {
		return s.executeFunc(callback)
	}
	return nil
}

func (s *e2eInvestigationChatService) GetMessages(_ context.Context, _ string) ([]investigation.Message, error) {
	return nil, nil
}

func (s *e2eInvestigationChatService) DeleteSession(_ context.Context, _ string) error {
	return nil
}

func (s *e2eInvestigationChatService) ListAvailableTools(_ context.Context, _ string) []string {
	return nil
}

func (s *e2eInvestigationChatService) SetAutonomousMode(_ bool) {}

// e2eCommandExecutor records executions and returns canned results.
type e2eCommandExecutor struct {
	commands []string
	output   string
	code     int
	err      error
}

func (e *e2eCommandExecutor) ExecuteCommand(_ context.Context, command, _ string) (string, int, error) {
	e.commands = append(e.commands, command)
	return e.output, e.code, e.err
}

// e2eApprovalStore records approval requests.
type e2eApprovalStore struct {
	approvals []*investigation.Approval
}

func (s *e2eApprovalStore) Create(a *investigation.Approval) error {
	s.approvals = append(s.approvals, a)
	return nil
}

// e2eFixVerifier returns a canned verification result.
type e2eFixVerifier struct {
	resolved bool
	err      error
	called   bool
}

func (v *e2eFixVerifier) VerifyFixResolved(_ context.Context, _ *investigation.Finding) (bool, error) {
	v.called = true
	return v.resolved, v.err
}

// --- Helper to set up patrol + real orchestrator ---

type e2eSetup struct {
	patrolService   *PatrolService
	invStore        *investigation.Store
	approvalStore   *e2eApprovalStore
	commandExecutor *e2eCommandExecutor
	fixVerifier     *e2eFixVerifier
	invChatService  *e2eInvestigationChatService
}

func newE2ESetup(t *testing.T, autonomyLevel string, invChatExecute func(investigation.StreamCallback) error) *e2eSetup {
	t.Helper()

	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: autonomyLevel,
	}
	svc.provider = &mockProvider{}

	// Patrol-level chat service: creates a critical finding
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	patrolCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			_, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key:          "disk-full",
				Severity:     "critical",
				Category:     "storage",
				ResourceID:   "vm-500",
				ResourceName: "db-server",
				ResourceType: "vm",
				Title:        "Disk almost full on db-server",
				Description:  "Root partition is at 95%",
				Evidence:     "Disk usage: 95%",
			})
			if err != nil {
				return nil, fmt.Errorf("create finding: %w", err)
			}
			return &PatrolStreamResponse{Content: "Found disk full on db-server"}, nil
		},
	}
	svc.SetChatService(patrolCS)

	// Investigation-level services
	invChatSvc := &e2eInvestigationChatService{executeFunc: invChatExecute}
	invStore := investigation.NewStore("")
	approvalSt := &e2eApprovalStore{}
	cmdExec := &e2eCommandExecutor{output: "Cleanup done. 2GB freed.", code: 0}
	verifier := &e2eFixVerifier{resolved: true}

	invConfig := investigation.DefaultConfig()
	invConfig.VerificationDelay = 0 // No sleep in tests
	invConfig.Timeout = 10 * time.Second

	orchestrator := investigation.NewOrchestrator(invChatSvc, invStore, nil, approvalSt, invConfig)
	orchestrator.SetCommandExecutor(cmdExec)
	orchestrator.SetFixVerifier(verifier)

	// Wire via adapter
	adapter := NewInvestigationOrchestratorAdapter(orchestrator)

	// State
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-500", VMID: 500, Name: "db-server", Node: "pve-1", Status: "running",
				CPU: 0.50, Memory: models.Memory{Usage: 60}, Disk: models.Disk{Usage: 95}},
		},
		Nodes: []models.Node{
			{ID: "node/pve-1", Name: "pve-1", Status: "online", CPU: 0.30},
		},
	}
	stateProvider := &patrolTestStateProvider{state: state}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})
	ps.SetInvestigationOrchestrator(adapter)

	return &e2eSetup{
		patrolService:   ps,
		invStore:        invStore,
		approvalStore:   approvalSt,
		commandExecutor: cmdExec,
		fixVerifier:     verifier,
		invChatService:  invChatSvc,
	}
}

// waitForPatrolRun polls until the patrol run history has at least one entry.
func waitForPatrolRun(t *testing.T, ps *PatrolService, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for patrol run to complete")
}

// waitForInvestigation polls the investigation store until a session for the
// given finding reaches a terminal status.
func waitForInvestigation(t *testing.T, store *investigation.Store, findingID string, timeout time.Duration) *investigation.InvestigationSession {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inv := store.GetLatestByFinding(findingID)
		if inv != nil && (inv.Status == investigation.StatusCompleted || inv.Status == investigation.StatusFailed) {
			return inv
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for investigation of finding %s", findingID)
	return nil
}

// findByKey returns the first active finding with the given key.
func findByKey(ps *PatrolService, key string) *Finding {
	for _, f := range ps.findings.GetActive(FindingSeverityCritical) {
		if f.Key == key {
			return f
		}
	}
	for _, f := range ps.findings.GetActive(FindingSeverityWarning) {
		if f.Key == key {
			return f
		}
	}
	return nil
}

// --- End-to-end integration tests ---

// TestE2E_PatrolFindingToApproval exercises the complete lifecycle in "approval" mode:
// Patrol Run → Finding Created → Investigation Triggered → Fix Queued for Approval
func TestE2E_PatrolFindingToApproval(t *testing.T) {
	setup := newE2ESetup(t, "approval", func(cb investigation.StreamCallback) error {
		payload, _ := json.Marshal(map[string]string{
			"text": "Root partition is 95% full. Old log files can be cleaned.\n" +
				"PROPOSED_FIX: find /var/log -name '*.gz' -mtime +30 -delete\n" +
				"TARGET_HOST: db-server",
		})
		cb(investigation.StreamEvent{Type: "content", Data: payload})
		return nil
	})

	// Run patrol
	setup.patrolService.ForcePatrol(context.Background())
	waitForPatrolRun(t, setup.patrolService, 5*time.Second)

	// Verify finding was created
	f := findByKey(setup.patrolService, "disk-full")
	require.NotNil(t, f, "expected finding with key 'disk-full'")
	assert.Equal(t, FindingSeverityCritical, f.Severity)
	assert.Equal(t, "vm-500", f.ResourceID)

	// Wait for the investigation to complete (triggered async by MaybeInvestigateFinding)
	inv := waitForInvestigation(t, setup.invStore, f.ID, 10*time.Second)
	require.NotNil(t, inv)

	// In approval mode, fix should be queued, not auto-executed
	assert.Equal(t, investigation.StatusCompleted, inv.Status)
	assert.Equal(t, investigation.OutcomeFixQueued, inv.Outcome)
	assert.NotNil(t, inv.ProposedFix)
	assert.Equal(t, "find /var/log -name '*.gz' -mtime +30 -delete", inv.ProposedFix.Commands[0])
	assert.Equal(t, "db-server", inv.ProposedFix.TargetHost)

	// Approval should have been created
	require.Len(t, setup.approvalStore.approvals, 1)
	assert.Equal(t, "investigation_fix", setup.approvalStore.approvals[0].Type)
	assert.Equal(t, f.ID, setup.approvalStore.approvals[0].FindingID)

	// Command executor should NOT have been called
	assert.Empty(t, setup.commandExecutor.commands, "no commands should be auto-executed in approval mode")

	// Fix verifier should NOT have been called
	assert.False(t, setup.fixVerifier.called, "fix verifier should not be called when fix is queued")
}

// TestE2E_PatrolFindingToFixVerified exercises the complete lifecycle in "full" mode:
// Patrol Run → Finding Created → Investigation → Fix Executed → Fix Verified
func TestE2E_PatrolFindingToFixVerified(t *testing.T) {
	setup := newE2ESetup(t, "full", func(cb investigation.StreamCallback) error {
		payload, _ := json.Marshal(map[string]string{
			"text": "Root partition is 95% full. Cleaning old logs.\n" +
				"PROPOSED_FIX: find /var/log -name '*.gz' -mtime +30 -delete\n" +
				"TARGET_HOST: db-server",
		})
		cb(investigation.StreamEvent{Type: "content", Data: payload})
		return nil
	})

	// Run patrol
	setup.patrolService.ForcePatrol(context.Background())
	waitForPatrolRun(t, setup.patrolService, 5*time.Second)

	// Verify finding was created
	f := findByKey(setup.patrolService, "disk-full")
	require.NotNil(t, f, "expected finding with key 'disk-full'")

	// Wait for the investigation to complete
	inv := waitForInvestigation(t, setup.invStore, f.ID, 10*time.Second)
	require.NotNil(t, inv)

	// In full mode, fix should be auto-executed and verified
	assert.Equal(t, investigation.StatusCompleted, inv.Status)
	assert.Equal(t, investigation.OutcomeFixVerified, inv.Outcome)
	assert.NotNil(t, inv.ProposedFix)
	assert.Equal(t, "find /var/log -name '*.gz' -mtime +30 -delete", inv.ProposedFix.Commands[0])
	assert.Contains(t, inv.ProposedFix.Rationale, "Verification: Issue confirmed resolved")

	// Command executor should have been called
	require.Len(t, setup.commandExecutor.commands, 1)
	assert.Equal(t, "find /var/log -name '*.gz' -mtime +30 -delete", setup.commandExecutor.commands[0])

	// Approval store should NOT have been used
	assert.Empty(t, setup.approvalStore.approvals, "no approvals should be created in full mode")

	// Fix verifier should have been called
	assert.True(t, setup.fixVerifier.called, "fix verifier should have been called after execution")
}

// TestE2E_PatrolFindingToCannotFix exercises the lifecycle when investigation
// determines the issue cannot be automatically fixed.
func TestE2E_PatrolFindingToCannotFix(t *testing.T) {
	setup := newE2ESetup(t, "full", func(cb investigation.StreamCallback) error {
		payload, _ := json.Marshal(map[string]string{
			"text": "Disk is full but all files are critical data, no safe cleanup possible.\n" +
				"CANNOT_FIX: Requires adding more storage or manual data migration",
		})
		cb(investigation.StreamEvent{Type: "content", Data: payload})
		return nil
	})

	setup.patrolService.ForcePatrol(context.Background())
	waitForPatrolRun(t, setup.patrolService, 5*time.Second)

	f := findByKey(setup.patrolService, "disk-full")
	require.NotNil(t, f)

	inv := waitForInvestigation(t, setup.invStore, f.ID, 10*time.Second)
	require.NotNil(t, inv)

	assert.Equal(t, investigation.StatusCompleted, inv.Status)
	assert.Equal(t, investigation.OutcomeCannotFix, inv.Outcome)
	assert.Nil(t, inv.ProposedFix, "no fix should be proposed for cannot_fix outcome")
	assert.Empty(t, setup.commandExecutor.commands)
	assert.False(t, setup.fixVerifier.called)
}

// TestE2E_PatrolFindingToFixVerificationFailed exercises the lifecycle when
// a fix is executed but verification determines the issue persists.
func TestE2E_PatrolFindingToFixVerificationFailed(t *testing.T) {
	setup := newE2ESetup(t, "full", func(cb investigation.StreamCallback) error {
		payload, _ := json.Marshal(map[string]string{
			"text": "PROPOSED_FIX: find /var/log -name '*.gz' -mtime +30 -delete\nTARGET_HOST: db-server",
		})
		cb(investigation.StreamEvent{Type: "content", Data: payload})
		return nil
	})

	// Fix executes OK but verification says issue persists
	setup.fixVerifier.resolved = false

	setup.patrolService.ForcePatrol(context.Background())
	waitForPatrolRun(t, setup.patrolService, 5*time.Second)

	f := findByKey(setup.patrolService, "disk-full")
	require.NotNil(t, f)

	inv := waitForInvestigation(t, setup.invStore, f.ID, 10*time.Second)
	require.NotNil(t, inv)

	assert.Equal(t, investigation.StatusCompleted, inv.Status)
	assert.Equal(t, investigation.OutcomeFixVerificationFailed, inv.Outcome)
	assert.True(t, setup.fixVerifier.called)
	assert.Contains(t, inv.ProposedFix.Rationale, "Issue persists after fix execution")
}
