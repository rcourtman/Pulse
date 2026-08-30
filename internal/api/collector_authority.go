package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const maxCollectorAuthorityRequestBytes int64 = 16 << 10

type collectorAuthorityReductionRequest struct {
	AgentID  string `json:"agentId"`
	Hostname string `json:"hostname"`
}

// handleReduceCollectorAuthority is an irreversible, self-service reduction.
// A host credential may remove its own execution and management scopes during
// safe-profile migration, but this route can never add a scope or target
// another token.
func (r *Router) handleReduceCollectorAuthority(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	caller := getAPITokenRecordFromRequest(req)
	if caller == nil || !caller.HasScope(config.ScopeAgentReport) {
		http.Error(w, "Agent bearer credential required", http.StatusForbidden)
		return
	}
	decoder := json.NewDecoder(io.LimitReader(req.Body, maxCollectorAuthorityRequestBytes+1))
	decoder.DisallowUnknownFields()
	var payload collectorAuthorityReductionRequest
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	payload.AgentID = strings.TrimSpace(payload.AgentID)
	payload.Hostname = strings.TrimSpace(payload.Hostname)
	if payload.AgentID == "" || payload.Hostname == "" {
		http.Error(w, "Canonical agent identity required", http.StatusBadRequest)
		return
	}
	if r.persistence == nil {
		http.Error(w, "Collector authority persistence is unavailable", http.StatusServiceUnavailable)
		return
	}

	organizationID := strings.TrimSpace(GetOrgID(req.Context()))
	config.Mu.Lock()
	previous := make([]config.APITokenRecord, len(r.config.APITokens))
	for previousIndex := range r.config.APITokens {
		previous[previousIndex] = r.config.APITokens[previousIndex].Clone()
	}
	index := -1
	var prior config.APITokenRecord
	for candidateIndex := range r.config.APITokens {
		candidate := &r.config.APITokens[candidateIndex]
		if candidate.ID != caller.ID {
			continue
		}
		orgs := candidate.GetBoundOrgs()
		boundAgentID := strings.TrimSpace(candidate.Metadata["bound_agent_id"])
		boundHostname := strings.TrimSpace(candidate.Metadata["bound_hostname"])
		if len(orgs) != 1 || strings.TrimSpace(orgs[0]) != organizationID ||
			boundAgentID == "" || boundAgentID != payload.AgentID ||
			boundHostname == "" || !strings.EqualFold(boundHostname, payload.Hostname) {
			config.Mu.Unlock()
			http.Error(w, "Collector credential binding mismatch", http.StatusForbidden)
			return
		}
		index = candidateIndex
		prior = candidate.Clone()
		break
	}
	if index < 0 {
		config.Mu.Unlock()
		http.Error(w, "Collector credential not found", http.StatusUnauthorized)
		return
	}
	record := &r.config.APITokens[index]
	nextScopes, _, err := internalauth.CanonicalizeRoleScopes(internalauth.RuntimeRoleMonitoringCollector, record.Scopes)
	if err != nil {
		r.config.APITokens = previous
		config.Mu.Unlock()
		http.Error(w, "Collector credential scopes cannot be reduced safely", http.StatusForbidden)
		return
	}
	record.Scopes = nextScopes
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	record.Metadata[agenttokens.CredentialKindMetadataKey] = agenttokens.CredentialKindMonitoringCollector
	record.Metadata[agenttokens.CommandPolicyIntentMetadataKey] = agenttokens.CommandPolicyIntentDisabled
	if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
		r.config.APITokens = previous
		r.config.SortAPITokens()
		config.Mu.Unlock()
		http.Error(w, "Failed to persist collector authority reduction", http.StatusInternalServerError)
		return
	}
	config.Mu.Unlock()

	if r.agentExecServer != nil && prior.HasScope(config.ScopeAgentExec) {
		r.agentExecServer.InvalidateAgentSession(agentexec.AgentAdmission{
			OrganizationID: organizationID,
			TokenID:        prior.ID,
			AgentID:        payload.AgentID,
			Hostname:       payload.Hostname,
			RuntimeRole:    agentexec.RuntimeRoleLegacyFullTrust,
		})
	}
	LogAuditEventForTenant(organizationID, "collector_authority_reduced", caller.Name, GetClientIP(req), req.URL.Path, true, "Reduced collector credential to its exact monitoring scope allowlist")
	w.WriteHeader(http.StatusNoContent)
}
