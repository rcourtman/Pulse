package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleAgentFleetDiagnosticsReturnsFleetPayload(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	now := time.Now().UTC()
	state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "node-1",
		DisplayName:     "Node One",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.0.0",
	})

	router := &Router{config: &config.Config{}, monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/agents/diagnostics", nil)
	rec := httptest.NewRecorder()

	router.handleAgentFleetDiagnostics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload monitoring.AgentFleetDiagnostics
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostics: %v", err)
	}
	if payload.Agents == nil {
		t.Fatal("expected agents to be a non-nil array")
	}
	if payload.Summary.Total != 1 || len(payload.Agents) != 1 {
		t.Fatalf("expected one agent diagnostic, summary=%+v agents=%+v", payload.Summary, payload.Agents)
	}
	if payload.SchemaVersion != monitoring.AgentFleetDiagnosticsSchemaVersion {
		t.Fatalf("schema version = %d, want %d", payload.SchemaVersion, monitoring.AgentFleetDiagnosticsSchemaVersion)
	}
	if payload.Agents[0].Name != "Node One" {
		t.Fatalf("agent name = %q, want Node One", payload.Agents[0].Name)
	}
	if payload.Agents[0].ConnectionID != "agent:agent-1" || payload.Agents[0].Platform != "linux" {
		t.Fatalf("agent identity = %+v", payload.Agents[0])
	}
	if payload.AgentUpdateTargetVersion != currentAgentTargetVersion() {
		t.Fatalf("agent update target = %q, want %q", payload.AgentUpdateTargetVersion, currentAgentTargetVersion())
	}
}

func TestApplyAgentFleetActionRunnerStateMatchesOnlyBoundTypedRunner(t *testing.T) {
	connectedAt := time.Date(2026, 8, 29, 16, 30, 0, 0, time.UTC)
	expiredAt := connectedAt.Add(-time.Minute)
	diagnostics := monitoring.AgentFleetDiagnostics{Agents: []monitoring.AgentFleetAgentDiagnostic{
		{
			AgentID:  "host-a",
			Hostname: "host-a.example",
			Privilege: &monitoring.AgentFleetDiagnosticPrivilege{
				CommandAuthority: "monitoring-only",
				TypedHelper:      true,
			},
		},
		{AgentID: "host-b", Hostname: "host-b.example"},
		{AgentID: "host-c", Hostname: "host-c.example"},
	}}
	applyAgentFleetActionRunnerState(&diagnostics, []config.APITokenRecord{
		{
			OrgID: "default", Scopes: []string{config.ScopeAgentExec},
			Metadata: map[string]string{
				agenttokens.CredentialKindMetadataKey:       agenttokens.CredentialKindActionRunner,
				agenttokens.ActionCapabilityMetadataKey:     agenttokens.ActionCapabilityTypedV1,
				agenttokens.ActionBindingVersionMetadataKey: agenttokens.ActionBindingVersion,
				"bound_agent_id":                            "host-a",
			},
		},
		{
			OrgID: "default", Scopes: []string{config.ScopeAgentExec}, ExpiresAt: &expiredAt,
			Metadata: map[string]string{
				agenttokens.CredentialKindMetadataKey:       agenttokens.CredentialKindActionRunner,
				agenttokens.ActionCapabilityMetadataKey:     agenttokens.ActionCapabilityTypedV1,
				agenttokens.ActionBindingVersionMetadataKey: agenttokens.ActionBindingVersion,
				"bound_agent_id":                            "host-b",
			},
		},
		{
			OrgID: "default", Scopes: []string{config.ScopeAgentExec},
			Metadata: map[string]string{
				agenttokens.CredentialKindMetadataKey:                agenttokens.CredentialKindActionRunner,
				agenttokens.ActionCapabilityMetadataKey:              agenttokens.ActionCapabilityTypedV1,
				agenttokens.ActionBindingVersionMetadataKey:          agenttokens.ActionBindingVersion,
				agenttokens.ActionRunnerActivationPendingMetadataKey: "true",
				"bound_agent_id": "host-c",
			},
		},
		{OrgID: "other-org", Metadata: map[string]string{agenttokens.CredentialKindMetadataKey: agenttokens.CredentialKindActionRunner, "bound_agent_id": "host-a"}},
	}, []agentexec.ConnectedAgent{
		{AgentID: "host-a", RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1, Version: "6.3.0-linux-amd64", ConnectedAt: connectedAt, OperationReceiptVersion: 1, ActionPreflightVersion: 2, DockerObservationVersion: 2},
		{AgentID: "host-b", RuntimeRole: agentexec.RuntimeRoleLegacyFullTrust, ActionCapability: agentexec.ActionCapabilityTypedV1, Version: "legacy"},
		{AgentID: "host-d", RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1, Version: "other-host"},
	}, "default", connectedAt)

	hostA := diagnostics.Agents[0].Privilege
	if hostA == nil || hostA.CommandAuthority != "monitoring-only" || !hostA.TypedHelper ||
		!hostA.ActionRunnerCredentialIssued || !hostA.ActionRunnerCredentialActive || !hostA.ActionRunnerConnected ||
		hostA.ActionRunnerRuntimeRole != agentexec.RuntimeRoleActionRunner || hostA.ActionRunnerCapability != agentexec.ActionCapabilityTypedV1 ||
		hostA.ActionRunnerBindingVersion != agenttokens.ActionBindingVersion || hostA.ActionRunnerVersion != "6.3.0-linux-amd64" ||
		hostA.ActionRunnerConnectedAt != connectedAt.UnixMilli() || hostA.ActionRunnerReceiptProtocol != 1 ||
		hostA.ActionRunnerPreflightProtocol != 2 || hostA.ActionRunnerDockerObservationProtocol != 2 {
		t.Fatalf("host-a runtime posture = %+v", hostA)
	}
	hostB := diagnostics.Agents[1].Privilege
	if hostB == nil || !hostB.ActionRunnerCredentialIssued || hostB.ActionRunnerCredentialActive || hostB.ActionRunnerConnected {
		t.Fatalf("host-b credential/session posture = %+v", hostB)
	}
	hostC := diagnostics.Agents[2].Privilege
	if hostC == nil || !hostC.ActionRunnerCredentialIssued || hostC.ActionRunnerCredentialActive || hostC.ActionRunnerConnected {
		t.Fatalf("host-c pending credential/session posture = %+v", hostC)
	}
}
