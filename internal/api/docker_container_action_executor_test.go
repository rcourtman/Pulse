package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type fakeDockerActionAgentCommander struct {
	results []*agentexec.CommandResultPayload
	calls   []agentexec.ExecuteCommandPayload
}

func (f *fakeDockerActionAgentCommander) ExecuteCommand(_ context.Context, _ string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	f.calls = append(f.calls, cmd)
	if len(f.results) == 0 {
		return &agentexec.CommandResultPayload{RequestID: cmd.RequestID, Success: true, ExitCode: 0}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestDockerContainerActionExecutorDispatchesPodmanRestartAndVerification(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerActionResource("app-container:api", "podman", "running", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{results: []*agentexec.CommandResultPayload{
		{RequestID: "act_container", Success: true, ExitCode: 0, Stdout: "api restarted"},
		{RequestID: "act_container-verify", Success: true, ExitCode: 0, Stdout: "running true"},
	}}
	executor := newDockerContainerActionExecutor(h, agents)

	result, err := executor.ExecuteAction(context.Background(), dockerContainerActionRecord("act_container", "app-container:api", "restart"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.calls) != 2 {
		t.Fatalf("agent calls = %d, want dispatch and verification", len(agents.calls))
	}
	if got := agents.calls[0].Command; got != "podman restart 'container-123'" {
		t.Fatalf("dispatch command = %q", got)
	}
	if agents.calls[0].ApprovalID != "act_container" || agents.calls[0].Trusted {
		t.Fatalf("dispatch approval/trust = %q/%v", agents.calls[0].ApprovalID, agents.calls[0].Trusted)
	}
	if got := agents.calls[1].Command; got != "podman inspect -f '{{.State.Status}} {{.State.Running}}' 'container-123'" {
		t.Fatalf("verification command = %q", got)
	}
}

func TestDockerContainerActionExecutorFailsWhenCapabilityNoLongerAdvertised(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.Capabilities = nil
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents)

	result, err := executor.ExecuteAction(context.Background(), dockerContainerActionRecord("act_container", "app-container:api", "restart"))
	if err == nil || !strings.Contains(err.Error(), "does not currently advertise restart capability") {
		t.Fatalf("ExecuteAction err = %v, result = %#v", err, result)
	}
	if len(agents.calls) != 0 {
		t.Fatalf("agent calls = %#v, want none", agents.calls)
	}
}

func TestHandleExecuteActionRejectsNeverAutoRemediateBeforeExecutor(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{
				ID:        "vm:42",
				Type:      unified.ResourceTypeVM,
				Name:      "web-42",
				Status:    unified.StatusWarning,
				LastSeen:  now,
				UpdatedAt: now,
				Sources:   []unified.DataSource{unified.SourceProxmox},
				Capabilities: []unified.ResourceCapability{
					{
						Name:                 "restart",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Restart the VM",
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
					},
				},
			},
		},
	})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "should not run"}}
	h.SetActionExecutor(executor)

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-locked",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, planReq)
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved"}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	h.HandleDecideAction(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:42",
		NeverAutoRemediate: true,
		SetAt:              now,
		SetBy:              "operator@example.com",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", plan.ActionID)
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, executeReq)
	if executeRec.Code != http.StatusConflict {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want none", executor.calls)
	}
	audit, ok, err := store.GetActionAudit(plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unified.ActionStateFailed || audit.Result == nil || !strings.HasPrefix(audit.Result.ErrorMessage, "resource_remediation_locked:") {
		t.Fatalf("locked audit = %#v, ok=%v", audit, ok)
	}
}

func dockerContainerActionResource(id, runtime, state string, now time.Time) unified.Resource {
	return unified.Resource{
		ID:         id,
		Type:       unified.ResourceTypeAppContainer,
		Technology: runtime,
		Name:       "api",
		Status:     unified.StatusOnline,
		LastSeen:   now,
		UpdatedAt:  now,
		Sources:    []unified.DataSource{unified.SourceDocker},
		SourceStatus: map[unified.DataSource]unified.SourceStatus{
			unified.SourceDocker: {Status: "online", LastSeen: now},
		},
		Docker: &unified.DockerData{
			AgentID:        "agent-1",
			ContainerID:    "container-123",
			ContainerState: state,
			Runtime:        runtime,
		},
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart this container",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Platform:             runtime,
				InternalHandler:      dockerContainerLifecycleHandler,
			},
		},
	}
}

func dockerContainerActionRecord(actionID, resourceID, operation string) unified.ActionAuditRecord {
	return unified.ActionAuditRecord{
		ID:        actionID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		State:     unified.ActionStateExecuting,
		Request: unified.ActionRequest{
			RequestID:      "req-" + actionID,
			ResourceID:     resourceID,
			CapabilityName: operation,
			Reason:         "test execution",
			RequestedBy:    "agent:oncall-helper",
			Params:         map[string]any{},
		},
		Plan: unified.ActionPlan{
			ActionID:         actionID,
			RequestID:        "req-" + actionID,
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unified.ApprovalAdmin,
			PlannedAt:        time.Now().UTC().Add(-time.Minute),
			ExpiresAt:        time.Now().UTC().Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}
}
