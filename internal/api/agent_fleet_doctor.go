package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

func (r *Router) handleAgentFleetDiagnostics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	monitor, err := r.getMonitor(req)
	if err != nil || monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "monitor_unavailable", "Monitor not available", nil)
		return
	}

	serverVersion := "dev"
	if versionInfo, err := updates.GetCurrentVersion(); err == nil && versionInfo != nil {
		serverVersion = versionInfo.Version
	}
	agentUpdateTargetVersion := currentAgentTargetVersion()
	now := time.Now().UTC()
	diagnostics := monitor.GetAgentFleetDiagnosticsForTarget(serverVersion, agentUpdateTargetVersion, now)
	organizationID := GetOrgID(req.Context())
	connected := []agentexec.ConnectedAgent(nil)
	if r.agentExecServer != nil {
		connected = r.agentExecServer.GetConnectedAgentsForOrganization(organizationID)
	}
	applyAgentFleetActionRunnerState(&diagnostics, actionRunnerCredentialSnapshot(r.config), connected, organizationID, now)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(diagnostics); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent fleet diagnostics")
	}
}

// applyAgentFleetActionRunnerState joins the existing organization-scoped
// command-session inventory to the existing Agent Doctor response. Admission
// has already bound an action runner to one canonical host identity; the
// projection still filters the closed runtime role and capability before
// reporting it as connected.
func actionRunnerCredentialSnapshot(cfg *config.Config) []config.APITokenRecord {
	if cfg == nil {
		return nil
	}
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	records := make([]config.APITokenRecord, len(cfg.APITokens))
	for i := range cfg.APITokens {
		records[i] = cfg.APITokens[i].Clone()
	}
	return records
}

func applyAgentFleetActionRunnerState(
	diagnostics *monitoring.AgentFleetDiagnostics,
	records []config.APITokenRecord,
	connected []agentexec.ConnectedAgent,
	organizationID string,
	now time.Time,
) {
	if diagnostics == nil || len(diagnostics.Agents) == 0 {
		return
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		organizationID = "default"
	}
	issued := make(map[string]config.APITokenRecord, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.OrgID) != organizationID ||
			strings.TrimSpace(record.Metadata[agenttokens.CredentialKindMetadataKey]) != agenttokens.CredentialKindActionRunner {
			continue
		}
		agentID := strings.TrimSpace(record.Metadata["bound_agent_id"])
		if agentID == "" {
			continue
		}
		previous, found := issued[agentID]
		if !found || (!actionRunnerCredentialActive(previous, now) && actionRunnerCredentialActive(record, now)) {
			issued[agentID] = record
		}
	}
	runners := make(map[string]agentexec.ConnectedAgent, len(connected))
	for _, candidate := range connected {
		agentID := strings.TrimSpace(candidate.AgentID)
		if agentID == "" || strings.TrimSpace(candidate.RuntimeRole) != agentexec.RuntimeRoleActionRunner ||
			strings.TrimSpace(candidate.ActionCapability) != agentexec.ActionCapabilityTypedV1 {
			continue
		}
		runners[agentID] = candidate
	}
	for i := range diagnostics.Agents {
		agentID := strings.TrimSpace(diagnostics.Agents[i].AgentID)
		record, credentialIssued := issued[agentID]
		runner, runnerConnected := runners[agentID]
		if !credentialIssued && !runnerConnected {
			continue
		}
		if diagnostics.Agents[i].Privilege == nil {
			diagnostics.Agents[i].Privilege = &monitoring.AgentFleetDiagnosticPrivilege{}
		}
		privilege := diagnostics.Agents[i].Privilege
		if credentialIssued {
			privilege.ActionRunnerCredentialIssued = true
			privilege.ActionRunnerRuntimeRole = strings.TrimSpace(record.Metadata[agenttokens.CredentialKindMetadataKey])
			privilege.ActionRunnerCapability = strings.TrimSpace(record.Metadata[agenttokens.ActionCapabilityMetadataKey])
			privilege.ActionRunnerBindingVersion = strings.TrimSpace(record.Metadata[agenttokens.ActionBindingVersionMetadataKey])
			privilege.ActionRunnerCredentialActive = actionRunnerCredentialActive(record, now)
		}
		if runnerConnected {
			privilege.ActionRunnerConnected = true
			privilege.ActionRunnerRuntimeRole = strings.TrimSpace(runner.RuntimeRole)
			privilege.ActionRunnerCapability = strings.TrimSpace(runner.ActionCapability)
			privilege.ActionRunnerVersion = strings.TrimSpace(runner.Version)
			privilege.ActionRunnerReceiptProtocol = runner.OperationReceiptVersion
			privilege.ActionRunnerPreflightProtocol = runner.ActionPreflightVersion
			privilege.ActionRunnerDockerObservationProtocol = runner.DockerObservationVersion
			if !runner.ConnectedAt.IsZero() {
				privilege.ActionRunnerConnectedAt = runner.ConnectedAt.UTC().UnixMilli()
			}
		}
	}
}

func actionRunnerCredentialActive(record config.APITokenRecord, now time.Time) bool {
	return record.HasScope(config.ScopeAgentExec) &&
		strings.TrimSpace(record.Metadata[agenttokens.ActionRunnerActivationPendingMetadataKey]) != "true" &&
		strings.TrimSpace(record.Metadata[agenttokens.ActionCapabilityMetadataKey]) == agenttokens.ActionCapabilityTypedV1 &&
		strings.TrimSpace(record.Metadata[agenttokens.ActionBindingVersionMetadataKey]) == agenttokens.ActionBindingVersion &&
		(record.ExpiresAt == nil || record.ExpiresAt.After(now))
}
