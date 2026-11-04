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

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type fakeDockerActionAgentCommander struct {
	results          []*agentexec.CommandResultPayload
	calls            []agentexec.ExecuteCommandPayload
	callAgents       []string
	typedCalls       []agentexec.DockerContainerLifecyclePayload
	typedUpdateCalls []agentexec.DockerContainerUpdatePayload
	connected        map[string]bool
	agentByHost      map[string]string
	queryResult      operationreceipt.QueryResult
	queries          []operationreceipt.Identity
}

type scopedFakeDockerActionAgentCommander struct {
	*fakeDockerActionAgentCommander
	tokenAgents map[string]string
	lastOrg     string
}

type observingDockerActionAgentCommander struct {
	*fakeDockerActionAgentCommander
	observations []agentexec.DockerContainerObservationPayload
}

func (f *observingDockerActionAgentCommander) ObserveDockerContainer(_ context.Context, _ string, req agentexec.DockerContainerObservationPayload) (*agentexec.DockerContainerObservationResultPayload, error) {
	if err := agentexec.BindDockerContainerObservationPayload(&req); err != nil {
		return nil, err
	}
	f.observations = append(f.observations, req)
	return &agentexec.DockerContainerObservationResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, ProtocolVersion: req.ProtocolVersion, RequestDigest: req.RequestDigest,
		Observed: true,
		Snapshot: agentexec.DockerContainerObservationSnapshot{ContainerID: req.ContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthHealthy, StartedAt: time.Now().UTC(), ObservedAt: time.Now().UTC()},
	}, nil
}

func (f *scopedFakeDockerActionAgentCommander) IsAgentConnectedForOrganization(organizationID, agentID string) bool {
	f.lastOrg = organizationID
	return f.IsAgentConnected(agentID)
}

func (f *scopedFakeDockerActionAgentCommander) GetAgentForHostForOrganization(organizationID, hostname string) (string, bool) {
	f.lastOrg = organizationID
	return f.GetAgentForHost(hostname)
}

func (f *scopedFakeDockerActionAgentCommander) GetAgentForTokenForOrganization(organizationID, tokenID string) (string, bool) {
	f.lastOrg = organizationID
	agentID, ok := f.tokenAgents[tokenID]
	return agentID, ok
}

func (f *scopedFakeDockerActionAgentCommander) GetAgentForIdentityForOrganization(organizationID, agentID, hostname string) (string, bool) {
	f.lastOrg = organizationID
	hostAgentID, ok := f.GetAgentForHost(hostname)
	return agentID, ok && hostAgentID == agentID && f.IsAgentConnected(agentID)
}

func dockerActionDispatchContext(t *testing.T, executor dockerContainerActionExecutor, record unified.ActionAuditRecord) context.Context {
	t.Helper()
	attempt, err := unified.NewActionDispatchAttempt(record.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = executor.BindActionDispatch(context.Background(), record, attempt)
	if err != nil {
		t.Fatal(err)
	}
	return actionlifecycle.ContextWithCommittedDispatchAttempt(context.Background(), attempt)
}

func (f *fakeDockerActionAgentCommander) AgentOperationReceiptVersion(string) int { return 1 }

func (f *fakeDockerActionAgentCommander) QueryAgentOperation(_ context.Context, _ string, identity operationreceipt.Identity) (operationreceipt.QueryResult, error) {
	f.queries = append(f.queries, identity)
	return f.queryResult, nil
}

func (f *fakeDockerActionAgentCommander) ExecuteDockerContainerLifecycle(_ context.Context, agentID string, req agentexec.DockerContainerLifecyclePayload) (*agentexec.DockerContainerLifecycleResultPayload, error) {
	f.callAgents = append(f.callAgents, agentID)
	f.typedCalls = append(f.typedCalls, req)
	now := time.Now().UTC()
	return &agentexec.DockerContainerLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, ContainerID: req.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: req.ContainerID, State: req.ExpectedState, Running: true, Health: agentexec.DockerContainerHealthNone, StartedAt: req.ExpectedStartedAt, ObservedAt: now.Add(-time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: req.ContainerID, State: "running", Running: true, Health: agentexec.DockerContainerHealthNone, StartedAt: now, ObservedAt: now},
	}, nil
}

func (f *fakeDockerActionAgentCommander) ExecuteDockerContainerUpdate(_ context.Context, agentID string, req agentexec.DockerContainerUpdatePayload) (*agentexec.DockerContainerUpdateResultPayload, error) {
	f.callAgents = append(f.callAgents, agentID)
	f.typedUpdateCalls = append(f.typedUpdateCalls, req)
	now := time.Now().UTC()
	newID := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	return &agentexec.DockerContainerUpdateResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, ContainerID: req.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		NewContainerID: newID, ContainerName: "api",
		OldImageDigest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		NewImageDigest: "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		BackupCreated:  true, BackupContainer: "api_pulse_backup_20260714_000000",
		After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: newID, State: "running", Running: true, Health: agentexec.DockerContainerHealthNone, StartedAt: now, ObservedAt: now},
	}, nil
}

func (f *fakeDockerActionAgentCommander) ExecuteCommand(_ context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	f.callAgents = append(f.callAgents, agentID)
	f.calls = append(f.calls, cmd)
	if len(f.results) == 0 {
		return &agentexec.CommandResultPayload{RequestID: cmd.RequestID, Success: true, ExitCode: 0}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func (f *fakeDockerActionAgentCommander) GetAgentForHost(hostname string) (string, bool) {
	if f.agentByHost == nil {
		return "", false
	}
	agentID, ok := f.agentByHost[strings.TrimSpace(hostname)]
	return agentID, ok
}

func (f *fakeDockerActionAgentCommander) IsAgentConnected(agentID string) bool {
	if f.connected == nil {
		return true
	}
	return f.connected[agentID]
}

func dockerActionCapabilityNames(capabilities []unified.ResourceCapability) []string {
	names := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		names = append(names, capability.Name)
	}
	return names
}

func dockerActionReadinessByName(readinesses []unified.ResourceActionReadiness, name string) (unified.ResourceActionReadiness, bool) {
	for _, readiness := range readinesses {
		if readiness.Name == name {
			return readiness, true
		}
	}
	return unified.ResourceActionReadiness{}, false
}

func TestDockerCommandSessionResolutionUsesTenantAndRecoversFromStaleReportToken(t *testing.T) {
	agents := &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{
			connected:   map[string]bool{"canonical-agent": true, "stale-agent": true},
			agentByHost: map[string]string{"docker-host": "canonical-agent"},
		},
		tokenAgents: map[string]string{"fresh-token": "canonical-agent"},
	}
	executor := dockerContainerActionExecutor{agents: agents}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "org-b")
	resource := unified.Resource{Docker: &unified.DockerData{
		TokenID:  "fresh-token",
		AgentID:  "stale-agent",
		Hostname: "docker-host",
	}}

	agentID, err := executor.connectedDockerCommandAgentID(ctx, resource)
	if err != nil {
		t.Fatalf("resolve command session: %v", err)
	}
	if agentID != "canonical-agent" || agents.lastOrg != "org-b" {
		t.Fatalf("resolved agent/org = %q/%q, want canonical-agent/org-b", agentID, agents.lastOrg)
	}

	delete(agents.tokenAgents, "fresh-token")
	agents.agentByHost["docker-host"] = "canonical-agent"
	resource.Docker.AgentID = "canonical-agent"
	if agentID, err := executor.connectedDockerCommandAgentID(ctx, resource); err != nil || agentID != "canonical-agent" {
		t.Fatalf("stale token exact-identity fallback = %q, %v; want canonical-agent", agentID, err)
	}

	resource.Docker.AgentID = "stale-agent"
	if agentID, err := executor.connectedDockerCommandAgentID(ctx, resource); err == nil {
		t.Fatalf("stale token unexpectedly crossed report identity to %q", agentID)
	}
}

func TestDockerContainerActionExecutorDispatchesPodmanRestartAndVerification(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerActionResource("app-container:api", "podman", "running", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	record := dockerContainerActionRecord("act_container", "app-container:api", "restart")

	result, err := executor.ExecuteAction(dockerActionDispatchContext(t, executor, record), record)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.typedCalls) != 1 || len(agents.calls) != 0 {
		t.Fatalf("typed/raw agent calls = %d/%d, want one typed dispatch and zero raw commands", len(agents.typedCalls), len(agents.calls))
	}
	if got := agents.typedCalls[0].Operation; got != agentexec.DockerContainerOperationRestart {
		t.Fatalf("typed operation = %q", got)
	}
	if agents.typedCalls[0].Runtime != "podman" {
		t.Fatalf("runtime = %q", agents.typedCalls[0].Runtime)
	}
	if agents.typedCalls[0].RequestID != "act_container.dispatch.1" {
		t.Fatalf("dispatch request identity = %q", agents.typedCalls[0].RequestID)
	}
}

func TestDockerContainerActionExecutorUsesProductionDaemonObservationForIndependentTruth(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{dockerContainerActionResource("app-container:api", "docker", "running", now)},
	})
	agents := &observingDockerActionAgentCommander{fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{}}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	if executor.observer == nil {
		t.Fatal("production constructor did not wire the typed daemon observer")
	}
	record := dockerContainerActionRecord("act_container", "app-container:api", "restart")
	result, err := executor.ExecuteAction(dockerActionDispatchContext(t, executor, record), record)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if len(agents.observations) != 1 || agents.observations[0].ActionID != record.ID {
		t.Fatalf("observations=%#v", agents.observations)
	}
	truth := result.ActionResultV2
	if truth == nil || truth.Execution.Status != unified.ActionExecutionSucceeded || truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceIndependent {
		t.Fatalf("canonical truth=%#v", truth)
	}
	if len(truth.Verification.Evidence) != 1 || truth.Verification.Evidence[0].ObserverTrustDomain == truth.Verification.Evidence[0].ExecutorTrustDomain {
		t.Fatalf("verification evidence=%#v", truth.Verification.Evidence)
	}
}

func TestDockerContainerActionExecutorReachesOwnAgentUnderAcknowledgedID(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.Docker.AgentID = "pinned-agent-id"
	resource.Docker.Hostname = "tower"
	resource.Docker.TokenID = "report-token"
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{
			connected:   map[string]bool{"acknowledged-agent-id": true},
			agentByHost: map[string]string{"tower": "acknowledged-agent-id"},
		},
		tokenAgents: map[string]string{"report-token": "acknowledged-agent-id"},
	}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)

	readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{
		RequestID:      "req-availability",
		ResourceID:     "app-container:api",
		CapabilityName: "restart",
		Reason:         "operator requested restart",
		RequestedBy:    "operator",
	}, resource)
	if !readiness.Available {
		t.Fatalf("CheckActionAvailable readiness = %#v, want available through the agent's own session", readiness)
	}

	record := dockerContainerActionRecord("act_container", "app-container:api", "restart")
	result, err := executor.ExecuteAction(dockerActionDispatchContext(t, executor, record), record)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("result = %#v, want successful execution", result)
	}
	if len(agents.callAgents) != 1 || agents.callAgents[0] != "acknowledged-agent-id" {
		t.Fatalf("call agents = %#v, want one typed dispatch through acknowledged-agent-id", agents.callAgents)
	}
}

// The command server admits one session per hostname per tenant, so a hostname
// lookup finds at most one agent, but that agent can be on another machine (#2681).
func TestDockerCommandSessionOnlyReachesTheResourcesOwnAgent(t *testing.T) {
	const namesake = "namesake-agent"
	tests := []struct {
		name        string
		docker      unified.DockerData
		connected   map[string]bool
		agentByHost map[string]string
		tokenAgents map[string]string
		wantAgent   string
	}{
		{
			name:        "tokenless host with no session of its own stays off a namesake",
			docker:      unified.DockerData{AgentID: "docker-source-1", Hostname: "tower"},
			connected:   map[string]bool{namesake: true},
			agentByHost: map[string]string{"tower": namesake},
		},
		{
			name:        "tokenless host with its own session uses it",
			docker:      unified.DockerData{AgentID: "docker-source-1", Hostname: "tower"},
			connected:   map[string]bool{"docker-source-1": true, namesake: true},
			agentByHost: map[string]string{"tower": namesake},
			wantAgent:   "docker-source-1",
		},
		{
			name:        "token held by a session on another machine is refused",
			docker:      unified.DockerData{TokenID: "shared-token", AgentID: "docker-source-1", Hostname: "tower"},
			connected:   map[string]bool{"other-machine-agent": true},
			agentByHost: map[string]string{"other-machine": "other-machine-agent"},
			tokenAgents: map[string]string{"shared-token": "other-machine-agent"},
		},
		{
			name:        "token session that holds the resource's agent ID survives a rename",
			docker:      unified.DockerData{TokenID: "report-token", AgentID: "agent-1", Hostname: "old-name"},
			connected:   map[string]bool{"agent-1": true},
			agentByHost: map[string]string{"new-name": "agent-1"},
			tokenAgents: map[string]string{"report-token": "agent-1"},
			wantAgent:   "agent-1",
		},
		{
			name:        "token session on the resource's hostname may hold another ID",
			docker:      unified.DockerData{TokenID: "report-token", AgentID: "pinned-agent-id", Hostname: "tower"},
			connected:   map[string]bool{"acknowledged-agent-id": true},
			agentByHost: map[string]string{"tower": "acknowledged-agent-id"},
			tokenAgents: map[string]string{"report-token": "acknowledged-agent-id"},
			wantAgent:   "acknowledged-agent-id",
		},
		{
			name:        "guest discovered host never reaches an agent with the guest's name",
			docker:      unified.DockerData{AgentID: "proxmox-lxc-docker:pve-a:node-a:141", Hostname: "frigate"},
			connected:   map[string]bool{namesake: true},
			agentByHost: map[string]string{"frigate": namesake},
		},
		{
			name:        "guest discovered host is refused even if a session claims its ID",
			docker:      unified.DockerData{TokenID: "claimed-token", AgentID: "proxmox-lxc-docker:pve-a:node-a:141", Hostname: "frigate"},
			connected:   map[string]bool{"proxmox-lxc-docker:pve-a:node-a:141": true},
			agentByHost: map[string]string{"frigate": "proxmox-lxc-docker:pve-a:node-a:141"},
			tokenAgents: map[string]string{"claimed-token": "proxmox-lxc-docker:pve-a:node-a:141"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := dockerContainerActionExecutor{agents: &scopedFakeDockerActionAgentCommander{
				fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{connected: tt.connected, agentByHost: tt.agentByHost},
				tokenAgents:                    tt.tokenAgents,
			}}
			docker := tt.docker
			agentID, err := executor.connectedDockerCommandAgentID(context.Background(), unified.Resource{Docker: &docker})
			if tt.wantAgent == "" {
				if err == nil {
					t.Fatalf("resolved %q, want no command session", agentID)
				}
				return
			}
			if err != nil || agentID != tt.wantAgent {
				t.Fatalf("resolved %q, %v; want %q", agentID, err, tt.wantAgent)
			}
		})
	}
}

func TestDockerContainerActionExecutorDoesNotDispatchToANamesake(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.Docker.AgentID = "docker-source-1"
	resource.Docker.Hostname = "tower"
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{
		connected:   map[string]bool{"namesake-agent": true},
		agentByHost: map[string]string{"tower": "namesake-agent"},
	}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	request := unified.ActionRequest{RequestID: "req-namesake", ResourceID: "app-container:api", CapabilityName: "restart", Reason: "operator requested restart", RequestedBy: "operator"}

	if readiness := executor.CheckActionAvailable(context.Background(), request, resource); readiness.Available || readiness.ReasonCode != "command_agent_disconnected" {
		t.Fatalf("readiness = %#v, want a disconnected agent", readiness)
	}
	record := dockerContainerActionRecord("act_container", "app-container:api", "restart")
	if _, err := executor.BindActionDispatch(context.Background(), record, unified.ActionDispatchAttempt{ID: "act_container.dispatch.1", ActionID: record.ID}); err == nil {
		t.Fatal("BindActionDispatch bound the action to a namesake's session")
	}
	if _, err := executor.ExecuteAction(actionDispatchTestContext(t, record.ID), record); err == nil || !strings.Contains(err.Error(), "is not connected") {
		t.Fatalf("ExecuteAction error = %v, want the disconnected agent, not a later binding check", err)
	}
	if len(agents.callAgents) != 0 || len(agents.typedCalls) != 0 {
		t.Fatalf("agent calls = %#v / %#v, want none", agents.callAgents, agents.typedCalls)
	}
}

func TestDockerContainerActionExecutorNamesTheFixForAGuestDiscoveredHost(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:frigate", "docker", "running", now)
	resource.Docker.AgentID = "proxmox-lxc-docker:pve-a:node-a:141"
	resource.Docker.HostSourceID = resource.Docker.AgentID
	resource.Docker.Hostname = "frigate"
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	executor := newDockerContainerActionExecutor(h, &fakeDockerActionAgentCommander{
		connected:   map[string]bool{"namesake-agent": true},
		agentByHost: map[string]string{"frigate": "namesake-agent"},
	}).(dockerContainerActionExecutor)

	readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{
		RequestID: "req-guest", ResourceID: resource.ID, CapabilityName: "restart", Reason: "operator requested restart", RequestedBy: "operator",
	}, resource)
	if readiness.Available || readiness.ReasonCode != "no_command_agent" || !strings.Contains(readiness.Reason, "Install the Pulse agent inside the guest") {
		t.Fatalf("readiness = %#v, want the guest's own fix named", readiness)
	}

	// Websocket rows carry no readiness, so the plan refusal is where a user reads this.
	h.SetActionExecutor(executor)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-guest-plan",
		"resourceId":"app-container:frigate",
		"capabilityName":"restart",
		"reason":"operator requested restart",
		"requestedBy":"operator"
	}`))
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "Install the Pulse agent inside the guest") {
		t.Fatalf("plan status = %d, body = %s, want a refusal naming the guest's own fix", rec.Code, rec.Body.String())
	}
}

// A real snapshot host, not a seeded resource, proves the token reaches the executor.
func TestDockerContainerActionExecutorRoutesASnapshotContainerByItsHostToken(t *testing.T) {
	now := time.Now().UTC()
	registry := unified.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{DockerHosts: []models.DockerHost{{
		ID: "docker-host-1", AgentID: "pinned-agent-id", Hostname: "tower", TokenID: "report-token", Status: "online", LastSeen: now,
		Containers: []models.DockerContainer{{ID: "0123456789abcdef", Name: "api", State: "running"}},
	}}})
	executor := dockerContainerActionExecutor{agents: &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{
			connected:   map[string]bool{"acknowledged-agent-id": true},
			agentByHost: map[string]string{"tower": "acknowledged-agent-id"},
		},
		tokenAgents: map[string]string{"report-token": "acknowledged-agent-id"},
	}}
	for _, resource := range registry.List() {
		if resource.Type != unified.ResourceTypeAppContainer {
			continue
		}
		agentID, err := executor.connectedDockerCommandAgentID(context.Background(), resource)
		if err != nil || agentID != "acknowledged-agent-id" {
			t.Fatalf("resolved %q, %v; want acknowledged-agent-id", agentID, err)
		}
		return
	}
	t.Fatal("no app container resource was ingested")
}

func TestDockerContainerActionExecutorFailsWhenCapabilityNoLongerAdvertised(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.Capabilities = nil
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_container"), dockerContainerActionRecord("act_container", "app-container:api", "restart"))
	if err == nil || !strings.Contains(err.Error(), "does not currently advertise restart capability") {
		t.Fatalf("ExecuteAction err = %v, result = %#v", err, result)
	}
	if len(agents.calls) != 0 {
		t.Fatalf("agent calls = %#v, want none", agents.calls)
	}
}

func TestDockerContainerActionExecutorAvailabilityRequiresConnectedAgent(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{connected: map[string]bool{"agent-1": false}}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)

	readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{
		RequestID:      "req-availability",
		ResourceID:     "app-container:api",
		CapabilityName: "restart",
		Reason:         "operator requested restart",
		RequestedBy:    "operator",
	}, resource)
	if readiness.Available || readiness.ReasonCode != "command_agent_disconnected" || readiness.Reason != "Docker / Podman command agent is not connected." {
		t.Fatalf("CheckActionAvailable readiness = %#v, want disconnected agent", readiness)
	}
	if len(agents.calls) != 0 {
		t.Fatalf("agent calls = %#v, want none", agents.calls)
	}
}

func TestDockerContainerActionExecutorStaleFixtureIsTripleZero(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.SourceStatus[unified.SourceDocker] = unified.SourceStatus{Status: "stale", LastSeen: now.Add(-time.Hour)}
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: resource.ID, CapabilityName: "restart"}, resource)
	if readiness.Available || readiness.ReasonCode != "stale_inventory" || len(agents.typedCalls) != 0 || len(agents.calls) != 0 {
		t.Fatalf("stale readiness=%#v typed=%d raw=%d", readiness, len(agents.typedCalls), len(agents.calls))
	}
}

func TestDockerContainerActionExecutorCallbackLossReconcilesReceiptWithoutRedispatch(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	startedAt := now.Add(-time.Minute)
	resource.Docker.StartedAt = &startedAt
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	record := dockerContainerActionRecord("act_container", resource.ID, "restart")
	attempt, err := unified.NewActionDispatchAttempt(record.ID, now.Add(-5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = executor.BindActionDispatch(context.Background(), record, attempt)
	if err != nil {
		t.Fatal(err)
	}
	facts := agentexec.DockerContainerLifecycleResultPayload{
		RequestID: attempt.ID, ActionID: record.ID, Operation: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, ContainerID: resource.Docker.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: resource.Docker.ContainerID, State: "running", Running: true, StartedAt: startedAt, ObservedAt: now.Add(-2 * time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: resource.Docker.ContainerID, State: "running", Running: true, StartedAt: now.Add(-time.Second), ObservedAt: now.Add(-time.Second)},
	}
	raw, _ := json.Marshal(facts)
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	agents.queryResult = operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &operationreceipt.Record{Identity: identity, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-4 * time.Second), StartedAt: now.Add(-3 * time.Second), TerminalAt: now, ResultKind: agentexec.DockerContainerLifecycleReceiptKind, ResultVersion: agentexec.DockerContainerLifecycleReceiptVersion, Result: raw}}
	result, _, found, err := executor.ReconcileActionDispatch(context.Background(), record, attempt)
	if err != nil || !found || result == nil || result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded || len(agents.typedCalls) != 0 || len(agents.queries) != 1 {
		t.Fatalf("reconcile result=%#v found=%v err=%v typed=%d queries=%d", result, found, err, len(agents.typedCalls), len(agents.queries))
	}
}

func TestHandlePlanActionRejectsDisconnectedDockerContainerAgent(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerActionResource("app-container:api", "docker", "running", now),
		},
	})
	h.SetActionExecutor(newDockerContainerActionExecutor(h, &fakeDockerActionAgentCommander{
		connected: map[string]bool{"agent-1": false},
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-disconnected-agent",
		"resourceId":"app-container:api",
		"capabilityName":"restart",
		"reason":"operator requested restart",
		"requestedBy":"operator"
	}`))
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("plan status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"action_execution_unavailable"`) ||
		!strings.Contains(rec.Body.String(), `"reason":"Docker / Podman command agent is not connected."`) ||
		!strings.Contains(rec.Body.String(), `"reasonCode":"command_agent_disconnected"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	audits, err := store.GetActionAudits("app-container:api", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("audits = %#v, want none for refused plan", audits)
	}
}

func TestResourceResponsesFilterDisconnectedDockerLifecycleCapabilities(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerActionResource("app-container:api", "docker", "running", now),
		},
	})
	h.SetActionExecutor(newDockerContainerActionExecutor(h, &fakeDockerActionAgentCommander{
		connected: map[string]bool{"agent-1": false},
	}))

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=app-container", nil)
	h.HandleListResources(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	var list ResourcesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("list data len = %d, want 1", len(list.Data))
	}
	if got := dockerActionCapabilityNames(list.Data[0].Capabilities); len(got) != 0 {
		t.Fatalf("list capabilities = %#v, want none", got)
	}
	readiness, ok := dockerActionReadinessByName(list.Data[0].ActionReadiness, "restart")
	if !ok || readiness.Available || readiness.ReasonCode != "command_agent_disconnected" {
		t.Fatalf("list action readiness = %#v, ok=%v; want disconnected restart", list.Data[0].ActionReadiness, ok)
	}

	detailRec := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/resources/app-container:api", nil)
	h.HandleGetResource(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detail unified.Resource
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if got := dockerActionCapabilityNames(detail.Capabilities); len(got) != 0 {
		t.Fatalf("detail capabilities = %#v, want none", got)
	}
	readiness, ok = dockerActionReadinessByName(detail.ActionReadiness, "restart")
	if !ok || readiness.Available || readiness.Reason != "Docker / Podman command agent is not connected." {
		t.Fatalf("detail action readiness = %#v, ok=%v; want disconnected restart", detail.ActionReadiness, ok)
	}
}

func TestHandleExecuteActionRejectsNeverAutoRemediateBeforeExecutor(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
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
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, ""))
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
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
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
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
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
			ContainerID:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
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

func dockerContainerUpdateActionResource(id, runtime string, now time.Time) unified.Resource {
	resource := dockerContainerActionResource(id, runtime, "running", now)
	resource.Docker.Image = "ghcr.io/example/api:latest"
	resource.Docker.UpdateStatus = &unified.DockerUpdateStatusMeta{
		UpdateAvailable: true,
		CurrentDigest:   "sha256:" + strings.Repeat("9", 64),
		LatestDigest:    "sha256:" + strings.Repeat("a", 64),
	}
	resource.Capabilities = append(resource.Capabilities, unified.ResourceCapability{
		Name:                 "update",
		Type:                 unified.CapabilityTypeCommon,
		Description:          "Update this container",
		MinimumApprovalLevel: unified.ApprovalAdmin,
		Platform:             runtime,
		InternalHandler:      dockerContainerUpdateHandler,
	})
	return resource
}

func TestDockerContainerActionExecutorDispatchesTypedUpdate(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerUpdateActionResource("app-container:api", "docker", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	record := dockerContainerActionRecord("act_update", "app-container:api", "update")

	result, err := executor.ExecuteAction(dockerActionDispatchContext(t, executor, record), record)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.typedUpdateCalls) != 1 || len(agents.typedCalls) != 0 || len(agents.calls) != 0 {
		t.Fatalf("update/lifecycle/raw agent calls = %d/%d/%d, want exactly one typed update dispatch", len(agents.typedUpdateCalls), len(agents.typedCalls), len(agents.calls))
	}
	call := agents.typedUpdateCalls[0]
	if call.Operation != agentexec.DockerContainerOperationUpdate || call.ExpectedImageDigest != "sha256:"+strings.Repeat("9", 64) || call.Runtime != "docker" {
		t.Fatalf("typed update payload = %+v", call)
	}
	if result.ActionResultV2 == nil || result.ActionResultV2.Compensation.Support != unified.ActionCompensationDeclared || result.ActionResultV2.Compensation.Status != unified.ActionCompensationNotNeeded {
		t.Fatalf("compensation truth = %+v, want declared/not_needed", result.ActionResultV2)
	}
}

func TestDockerContainerActionExecutorRefusesUpdateThroughLifecycleHandler(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	resource.Docker.Image = "ghcr.io/example/api:latest"
	resource.Capabilities = append(resource.Capabilities, unified.ResourceCapability{
		Name:                 "update",
		Type:                 unified.CapabilityTypeCommon,
		MinimumApprovalLevel: unified.ApprovalAdmin,
		Platform:             "docker",
		InternalHandler:      dockerContainerLifecycleHandler,
	})
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	record := dockerContainerActionRecord("act_update_wrong", "app-container:api", "update")

	if readiness := executor.CheckActionAvailable(context.Background(), record.Request, resource); readiness.Available || readiness.Name != "" {
		t.Fatalf("readiness = %+v, want fail-closed for wrong handler", readiness)
	}
}

func TestDockerContainerActionAvailabilityDetailNamesMissedLookup(t *testing.T) {
	now := time.Now().UTC()
	request := unified.ActionRequest{
		RequestID:      "req-detail",
		ResourceID:     "app-container:api",
		CapabilityName: "restart",
		Reason:         "operator requested restart",
		RequestedBy:    "operator",
	}

	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	executor := newDockerContainerActionExecutor(h, &fakeDockerActionAgentCommander{
		connected: map[string]bool{"agent-1": false},
	}).(dockerContainerActionExecutor)

	readiness := executor.CheckActionAvailable(context.Background(), request, resource)
	if readiness.Available || readiness.ReasonCode != "command_agent_disconnected" ||
		readiness.Reason != "Docker / Podman command agent is not connected." {
		t.Fatalf("identity-fallback readiness = %#v, want disconnected agent", readiness)
	}
	if !strings.Contains(readiness.Detail, `agent "agent-1"`) {
		t.Fatalf("identity-fallback detail = %q, want the missed agent identity named", readiness.Detail)
	}

	tokenResource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	tokenResource.Docker.TokenID = "rotated-token"
	tokenResource.Docker.Hostname = "docker-host"
	tokenExecutor := newDockerContainerActionExecutor(h, &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{
			connected: map[string]bool{"agent-1": true},
		},
		tokenAgents: map[string]string{},
	}).(dockerContainerActionExecutor)

	readiness = tokenExecutor.CheckActionAvailable(context.Background(), request, tokenResource)
	if readiness.Available || readiness.ReasonCode != "command_agent_disconnected" ||
		readiness.Reason != "Docker / Podman command agent is not connected." {
		t.Fatalf("token-named readiness = %#v, want disconnected agent", readiness)
	}
	if !strings.Contains(readiness.Detail, "rotated-token") ||
		!strings.Contains(readiness.Detail, `exact agent "agent-1" and hostname "docker-host" identity`) {
		t.Fatalf("token-named detail = %q, want the stale token and failed exact identity proof named", readiness.Detail)
	}
}

func TestHandlePlanActionRefusalEnvelopeCarriesDetail(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			dockerContainerActionResource("app-container:api", "docker", "running", now),
		},
	})
	h.SetActionExecutor(newDockerContainerActionExecutor(h, &fakeDockerActionAgentCommander{
		connected: map[string]bool{"agent-1": false},
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-refusal-detail",
		"resourceId":"app-container:api",
		"capabilityName":"restart",
		"reason":"operator requested restart",
		"requestedBy":"operator"
	}`))
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("plan status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	var envelope struct {
		Details map[string]string `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode refusal envelope: %v", err)
	}
	if !strings.Contains(envelope.Details["detail"], `agent "agent-1"`) {
		t.Fatalf("envelope detail = %q, want the missed agent identity named", envelope.Details["detail"])
	}
}
