package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
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
	rawToken, record, err := agenttokens.IssueActionRunnerAndPersist(r.config, r.persistence, agenttokens.ActionRunnerIssueOptions{
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

	LogAuditEventForTenant(organizationID, "action_runner_credential_issued", auth.GetUser(req.Context()), GetClientIP(req), req.URL.Path, true, "Issued host-bound typed action runner credential")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(actionRunnerCredentialResponse{
		Token:            rawToken,
		TokenID:          record.ID,
		OrganizationID:   record.OrgID,
		AgentID:          record.Metadata["bound_agent_id"],
		Hostname:         record.Metadata["bound_hostname"],
		RuntimeRole:      record.Metadata[agenttokens.RuntimeRoleMetadataKey],
		ActionCapability: record.Metadata[agenttokens.ActionCapabilityMetadataKey],
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
