package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

type fakeHostStorageCleanupAgent struct {
	connected bool
	result    *agentexec.HostStorageCleanupResultPayload
	err       error
	agentID   string
	requests  []agentexec.HostStorageCleanupPayload
}

var testHostStorageCleanupFingerprint = "sha256:" + strings.Repeat("c", 64)
var testHostStorageCleanupAfterFingerprint = "sha256:" + strings.Repeat("d", 64)

func (f *fakeHostStorageCleanupAgent) ExecuteHostStorageCleanup(_ context.Context, agentID string, req agentexec.HostStorageCleanupPayload) (*agentexec.HostStorageCleanupResultPayload, error) {
	f.agentID = agentID
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func (f *fakeHostStorageCleanupAgent) IsAgentConnected(string) bool { return f.connected }

func TestHostStorageCleanupActionExecutorDispatchesFingerprintBoundOperation(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{hostStorageCleanupActionResource(now)}})
	agents := verifiedHostStorageCleanupAgent()

	result, err := newHostStorageCleanupActionExecutor(h, agents).ExecuteAction(actionDispatchTestContext(t, "action-cleanup"), hostStorageCleanupActionRecord("action-cleanup"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Ran || !result.Verification.Success {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Output, "536870912 bytes before") || !strings.Contains(result.Output, "528482304 bytes reclaimed") {
		t.Fatalf("output = %q", result.Output)
	}
	if len(agents.requests) != 1 || agents.agentID != "agent-1" {
		t.Fatalf("agent calls = %#v agentID=%q", agents.requests, agents.agentID)
	}
	request := agents.requests[0]
	if request.ActionID != "action-cleanup" || request.Operation != agentexec.HostStorageCleanupOperationPackageCache || request.ExpectedFingerprint != testHostStorageCleanupFingerprint || request.Timeout != 300 {
		t.Fatalf("typed request = %#v", request)
	}
}

func TestHostStorageCleanupActionExecutorDoesNotExposeAgentErrorText(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{hostStorageCleanupActionResource(now)}})
	agents := &fakeHostStorageCleanupAgent{connected: true, result: &agentexec.HostStorageCleanupResultPayload{
		RequestID: "action-cleanup", Verification: agentexec.HostStorageCleanupVerificationFailed,
		Before: agentexec.HostStorageCleanupSnapshot{ReclaimableBytes: 512 * 1024 * 1024},
		After:  agentexec.HostStorageCleanupSnapshot{ReclaimableBytes: 512 * 1024 * 1024},
		Error:  "private repository package path and token",
	}}

	result, err := newHostStorageCleanupActionExecutor(h, agents).ExecuteAction(actionDispatchTestContext(t, "action-cleanup"), hostStorageCleanupActionRecord("action-cleanup"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	encoded := result.Output + " " + result.ErrorMessage + " " + result.Verification.Note + " " + result.Verification.Output
	if strings.Contains(encoded, "private repository") || strings.Contains(encoded, "token") {
		t.Fatalf("executor exposed agent error text: %q", encoded)
	}
}

func TestPatrolFullModeRunsStorageCleanupThroughCanonicalLifecycle(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{hostStorageCleanupActionResource(now)}})
	agents := verifiedHostStorageCleanupAgent()
	h.SetActionExecutor(newRoutedActionExecutor(h, newHostStorageCleanupActionExecutor(h, agents)))
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "agent:host-cleanup",
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled: true, CapabilityNames: []string{hostStorageCleanupCapability},
		},
		SetAt: now, SetBy: "operator@example.com",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "full", FullModeUnlocked: true}, nil
	})
	disposition, err := broker.Submit(context.Background(), aicontracts.ActionProposal{
		ProposalID: "proposal-cleanup", FindingID: "finding-disk-pressure", InvestigationID: "investigation-disk-pressure",
		ResourceID: "agent:host-cleanup", CapabilityName: hostStorageCleanupCapability, Reason: "Reclaim the package cache on the pressured filesystem",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStateCompleted) || disposition.VerificationStatus != string(unified.VerificationVerified) {
		t.Fatalf("disposition = %#v", disposition)
	}
	if len(agents.requests) != 1 || agents.requests[0].Operation != agentexec.HostStorageCleanupOperationPackageCache {
		t.Fatalf("typed agent requests = %#v", agents.requests)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit: found=%v err=%v", found, err)
	}
	if len(audit.Approvals) != 1 || audit.Approvals[0].Actor != patrolActionPolicyActor || audit.Approvals[0].Method != unified.MethodPolicy || audit.Result == nil || audit.VerificationOutcome.Status != unified.VerificationVerified {
		t.Fatalf("audit = %#v", audit)
	}
}

func TestHostStorageCleanupAvailabilityFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name       string
		mutate     func(*unified.Resource)
		connected  bool
		reasonCode string
	}{
		{name: "commands disabled", mutate: func(r *unified.Resource) { r.Agent.CommandsEnabled = false }, connected: true, reasonCode: "host_commands_disabled"},
		{name: "stale inventory", mutate: func(r *unified.Resource) { r.Agent.StorageCleanup.CheckedAt = now.Add(-time.Hour) }, connected: true, reasonCode: "stale_cleanup_inventory"},
		{name: "inventory error", mutate: func(r *unified.Resource) { r.Agent.StorageCleanup.Error = "inspection failed" }, connected: true, reasonCode: "cleanup_inventory_error"},
		{name: "invalid fingerprint", mutate: func(r *unified.Resource) { r.Agent.StorageCleanup.Fingerprint = "sha256:bad" }, connected: true, reasonCode: "invalid_cleanup_inventory"},
		{name: "insufficient bytes", mutate: func(r *unified.Resource) {
			r.Agent.StorageCleanup.ReclaimableBytes = unified.HostStorageCleanupMinReclaimableBytes - 1
		}, connected: true, reasonCode: "insufficient_reclaimable_space"},
		{name: "pressure cleared", mutate: func(r *unified.Resource) { r.Agent.Disks[0].Usage = 80 }, connected: true, reasonCode: "storage_pressure_cleared"},
		{name: "separate var filesystem healthy", mutate: func(r *unified.Resource) {
			r.Agent.Disks = append(r.Agent.Disks, unified.DiskInfo{Mountpoint: "/var", Usage: 50})
		}, connected: true, reasonCode: "storage_pressure_cleared"},
		{name: "disconnected", mutate: func(*unified.Resource) {}, connected: false, reasonCode: "command_agent_disconnected"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := hostStorageCleanupActionResource(now)
			tc.mutate(&resource)
			executor := hostStorageCleanupActionExecutor{agents: &fakeHostStorageCleanupAgent{connected: tc.connected}, now: func() time.Time { return now }}
			readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{CapabilityName: hostStorageCleanupCapability}, resource)
			if readiness.Available || readiness.Name != hostStorageCleanupCapability || readiness.ReasonCode != tc.reasonCode {
				t.Fatalf("readiness = %#v", readiness)
			}
		})
	}
}

func verifiedHostStorageCleanupAgent() *fakeHostStorageCleanupAgent {
	now := time.Now().UTC()
	return &fakeHostStorageCleanupAgent{connected: true, result: &agentexec.HostStorageCleanupResultPayload{
		RequestID: "filled-by-executor", Success: true,
		Before:         agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupFingerprint, ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now.Add(-time.Second)},
		After:          agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupAfterFingerprint, ReclaimableBytes: 8 * 1024 * 1024, CheckedAt: now},
		ReclaimedBytes: 504 * 1024 * 1024,
		Verification:   agentexec.HostStorageCleanupVerificationVerified,
	}}
}

func hostStorageCleanupActionResource(now time.Time) unified.Resource {
	return unified.Resource{
		ID: "agent:host-cleanup", Type: unified.ResourceTypeAgent, Technology: "linux", Name: "host-cleanup",
		Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now,
		Sources: []unified.DataSource{unified.SourceAgent}, SourceStatus: map[unified.DataSource]unified.SourceStatus{unified.SourceAgent: {Status: "online", LastSeen: now}},
		Agent: &unified.AgentData{
			AgentID: "agent-1", Platform: "linux", CommandsEnabled: true,
			Disks:          []unified.DiskInfo{{Mountpoint: "/", Usage: 95}},
			StorageCleanup: &unified.AgentStorageCleanupMeta{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupFingerprint, ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now, ObservedAt: now},
		},
		Capabilities: []unified.ResourceCapability{{
			Name: hostStorageCleanupCapability, Type: unified.CapabilityTypeCommon,
			Description: "Clean the bounded APT package cache", MinimumApprovalLevel: unified.ApprovalAdmin,
			AutoAuthorization: unified.AutoAuthorizeLowRisk, Platform: "linux", InternalHandler: hostStorageCleanupActionHandler,
		}},
	}
}

func hostStorageCleanupActionRecord(actionID string) unified.ActionAuditRecord {
	now := time.Now().UTC()
	return unified.ActionAuditRecord{
		ID: actionID, CreatedAt: now, UpdatedAt: now, State: unified.ActionStateExecuting,
		Request: unified.ActionRequest{
			RequestID: "request-" + actionID, ResourceID: "agent:host-cleanup", CapabilityName: hostStorageCleanupCapability,
			Reason: "Reclaim package cache storage", RequestedBy: "pulse_patrol", Params: map[string]any{},
		},
		Plan: unified.ActionPlan{
			ActionID: actionID, RequestID: "request-" + actionID, Allowed: true, RequiresApproval: true,
			ApprovalPolicy: unified.ApprovalAdmin, PlannedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
			ResourceVersion: "resource:sha256:test", PolicyVersion: "policy:sha256:test", PlanHash: "sha256:test",
		},
	}
}
