package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const maxActionRunnerCredentialRequestBytes int64 = 16 << 10

type actionRunnerCredentialRequest struct {
	AgentID  string `json:"agentId"`
	Hostname string `json:"hostname"`
	Name     string `json:"name,omitempty"`
}

type actionRunnerCredentialResponse struct {
	Token            string `json:"token"`
	TokenID          string `json:"tokenId"`
	OrganizationID   string `json:"organizationId"`
	AgentID          string `json:"agentId"`
	Hostname         string `json:"hostname"`
	RuntimeRole      string `json:"runtimeRole"`
	ActionCapability string `json:"actionCapability"`
}

type actionRunnerCredentialSelfRevokeRequest struct {
	AgentID  string `json:"agentId"`
	Hostname string `json:"hostname"`
}

func actionRunnerCredentialRoute(cfg *config.Config, issue, selfRevoke http.HandlerFunc) http.HandlerFunc {
	issue = RequireAdmin(cfg, RequireScope(config.ScopeSettingsWrite, RequireScope(config.ScopeActionsExecute, issue)))
	selfRevoke = RequireAuth(cfg, RequireScope(config.ScopeAgentExec, selfRevoke))
	return func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost:
			issue(w, req)
		case http.MethodDelete:
			selfRevoke(w, req)
		default:
			w.Header().Set("Allow", http.MethodPost+", "+http.MethodDelete)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleIssueActionRunnerCredential creates the separately scoped credential
// consumed by pulse-agent-runner. The route is operator-only; a monitoring
// collector credential cannot mint or upgrade itself into remediation
// authority.
func (r *Router) handleIssueActionRunnerCredential(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r == nil || r.config == nil {
		http.Error(w, "Action runner credential service unavailable", http.StatusServiceUnavailable)
		return
	}

	decoder := json.NewDecoder(io.LimitReader(req.Body, maxActionRunnerCredentialRequestBytes+1))
	decoder.DisallowUnknownFields()
	var payload actionRunnerCredentialRequest
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	organizationID := strings.TrimSpace(GetOrgID(req.Context()))
	canonicalAgentID, canonicalHostname, found := r.resolveActionRunnerHostIdentity(req, payload.AgentID, payload.Hostname)
	if !found {
		http.Error(w, "Canonical monitored host identity not found", http.StatusNotFound)
		return
	}
	issued, err := agenttokens.IssueActionRunnerAndPersistDetailed(r.config, r.persistence, agenttokens.ActionRunnerIssueOptions{
		TokenName:   payload.Name,
		OrgID:       organizationID,
		OwnerUserID: apiTokenOwnerUserIDForRequest(r.config, req),
		AgentID:     canonicalAgentID,
		Hostname:    canonicalHostname,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, agenttokens.ErrRecord) {
			status = http.StatusBadRequest
		}
		http.Error(w, "Failed to issue action runner credential", status)
		return
	}
	record := issued.Record
	for _, replaced := range issued.Replaced {
		r.invalidateActionRunnerRecord(replaced)
	}

	LogAuditEventForTenant(organizationID, "action_runner_credential_issued", auth.GetUser(req.Context()), GetClientIP(req), req.URL.Path, true, "Issued host-bound typed action runner credential")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(actionRunnerCredentialResponse{
		Token:            issued.Token,
		TokenID:          record.ID,
		OrganizationID:   record.OrgID,
		AgentID:          record.Metadata["bound_agent_id"],
		Hostname:         record.Metadata["bound_hostname"],
		RuntimeRole:      record.Metadata[agenttokens.RuntimeRoleMetadataKey],
		ActionCapability: record.Metadata[agenttokens.ActionCapabilityMetadataKey],
	})
}

// handleSelfRevokeActionRunnerCredential lets the separately credentialed
// runner revoke only its own exact tenant/host binding. It cannot select a
// token ID or another host, and browser/session authentication is rejected.
func (r *Router) handleSelfRevokeActionRunnerCredential(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r == nil || r.config == nil {
		http.Error(w, "Action runner credential service unavailable", http.StatusServiceUnavailable)
		return
	}
	caller := getAPITokenRecordFromRequest(req)
	if caller == nil {
		http.Error(w, "Action runner bearer credential required", http.StatusForbidden)
		return
	}
	decoder := json.NewDecoder(io.LimitReader(req.Body, maxActionRunnerCredentialRequestBytes+1))
	decoder.DisallowUnknownFields()
	var payload actionRunnerCredentialSelfRevokeRequest
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	organizationID := strings.TrimSpace(GetOrgID(req.Context()))
	config.Mu.Lock()
	previousTokens := append([]config.APITokenRecord(nil), r.config.APITokens...)
	index := -1
	var removed config.APITokenRecord
	for candidateIndex := range r.config.APITokens {
		candidate := &r.config.APITokens[candidateIndex]
		if candidate.ID != caller.ID {
			continue
		}
		orgs := candidate.GetBoundOrgs()
		if len(orgs) != 1 || strings.TrimSpace(orgs[0]) != organizationID ||
			!agentbinding.EvaluateActionRunner(candidate, payload.AgentID, payload.Hostname).Admit {
			config.Mu.Unlock()
			http.Error(w, "Action runner credential binding mismatch", http.StatusForbidden)
			return
		}
		index = candidateIndex
		removed = candidate.Clone()
		break
	}
	if index < 0 {
		config.Mu.Unlock()
		http.Error(w, "Action runner credential not found", http.StatusUnauthorized)
		return
	}
	r.config.APITokens = append(r.config.APITokens[:index], r.config.APITokens[index+1:]...)
	r.config.SortAPITokens()
	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			r.config.APITokens = previousTokens
			r.config.SortAPITokens()
			config.Mu.Unlock()
			http.Error(w, "Failed to revoke action runner credential", http.StatusInternalServerError)
			return
		}
	}
	config.Mu.Unlock()

	r.invalidateActionRunnerRecord(removed)
	LogAuditEventForTenant(organizationID, "action_runner_credential_revoked", auth.GetUser(req.Context()), GetClientIP(req), req.URL.Path, true, "Revoked exact host-bound typed action runner credential")
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) invalidateActionRunnerRecord(record config.APITokenRecord) bool {
	if r == nil || r.agentExecServer == nil {
		return false
	}
	orgs := record.GetBoundOrgs()
	if len(orgs) != 1 {
		return false
	}
	return r.agentExecServer.InvalidateActionRunnerSession(agentexec.AgentAdmission{
		OrganizationID:   strings.TrimSpace(orgs[0]),
		TokenID:          strings.TrimSpace(record.ID),
		AgentID:          strings.TrimSpace(record.Metadata["bound_agent_id"]),
		Hostname:         strings.TrimSpace(record.Metadata["bound_hostname"]),
		RuntimeRole:      strings.TrimSpace(record.Metadata[agenttokens.RuntimeRoleMetadataKey]),
		ActionCapability: strings.TrimSpace(record.Metadata[agenttokens.ActionCapabilityMetadataKey]),
	})
}

func (r *Router) resolveActionRunnerHostIdentity(req *http.Request, requestedID, requestedHostname string) (string, string, bool) {
	requestedID = strings.TrimSpace(requestedID)
	requestedHostname = strings.TrimSpace(requestedHostname)
	if r == nil || r.unifiedAgentHandlers == nil || req == nil || requestedID == "" || requestedHostname == "" {
		return "", "", false
	}
	monitor := r.unifiedAgentHandlers.getMonitor(req.Context())
	if monitor == nil {
		return "", "", false
	}
	var matchedID, matchedHostname string
	matches := 0
	for _, host := range monitor.GetLiveHostsSnapshot() {
		if strings.TrimSpace(host.ID) != requestedID ||
			!unifiedresources.HostnamesEquivalent(host.Hostname, requestedHostname) ||
			strings.TrimSpace(host.IntegrationSource) != "" || host.IdentityConflict != nil {
			continue
		}
		matchedID = strings.TrimSpace(host.ID)
		matchedHostname = unifiedresources.NormalizeFullHostname(host.Hostname)
		matches++
	}
	if matches != 1 || matchedID == "" || matchedHostname == "" {
		return "", "", false
	}
	return matchedID, matchedHostname, true
}
