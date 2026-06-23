package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

func (r *Router) withExternalAgentActivity(activity string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.recordExternalAgentActivity(req, activity)
		handler(w, req)
	}
}

func (r *Router) withExternalAgentCapabilityActivity(capabilityName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.recordExternalAgentCapabilityActivity(req, capabilityName)
		handler(w, req)
	}
}

func (r *Router) recordExternalAgentCapabilityActivity(req *http.Request, capabilityName string) {
	activity, requiredScope, ok := externalAgentCapabilityActivity(capabilityName)
	if !ok {
		return
	}
	r.recordExternalAgentActivity(req, activity, requiredScope)
}

func externalAgentActivityForCapability(capabilityName string) (string, bool) {
	activity, _, ok := externalAgentCapabilityActivity(capabilityName)
	return activity, ok
}

func externalAgentCapabilityActivity(capabilityName string) (string, string, bool) {
	capabilityName = strings.TrimSpace(capabilityName)
	if capabilityName == "" {
		return "", "", false
	}
	capability, ok := agentcapabilities.FindCapability(agentcapabilities.CanonicalManifest().Capabilities, capabilityName)
	if !ok {
		return "", "", false
	}
	var activity string
	switch strings.TrimSpace(capabilityName) {
	case agentcapabilities.ResourceContextCapabilityName:
		activity = config.ExternalAgentActivityResourceContext
	case agentcapabilities.FleetContextCapabilityName:
		activity = config.ExternalAgentActivityFleetContext
	case agentcapabilities.OperationsLoopStatusCapabilityName:
		activity = config.ExternalAgentActivityFleetContext
	case agentcapabilities.EventSubscriptionCapabilityName:
		activity = config.ExternalAgentActivityEventStream
	case agentcapabilities.ListNodesCapabilityName,
		agentcapabilities.AddNodeCapabilityName,
		agentcapabilities.UpdateNodeCapabilityName,
		agentcapabilities.RemoveNodeCapabilityName,
		agentcapabilities.TestNodeCredentialsCapabilityName,
		agentcapabilities.TestNodeConnectionCapabilityName,
		agentcapabilities.RefreshNodeClusterMembershipCapabilityName,
		agentcapabilities.DiscoverLANCapabilityName:
		activity = config.ExternalAgentActivityProvisioning
	case agentcapabilities.GetOperatorStateCapabilityName,
		agentcapabilities.SetOperatorStateCapabilityName,
		agentcapabilities.ClearOperatorStateCapabilityName:
		activity = config.ExternalAgentActivityOperatorState
	case agentcapabilities.ListFindingsCapabilityName:
		activity = config.ExternalAgentActivityFindingList
	case agentcapabilities.AcknowledgeFindingCapabilityName,
		agentcapabilities.SnoozeFindingCapabilityName,
		agentcapabilities.DismissFindingCapabilityName,
		agentcapabilities.ResolveFindingCapabilityName:
		activity = config.ExternalAgentActivityFindingDecision
	case agentcapabilities.PlanActionCapabilityName:
		activity = config.ExternalAgentActivityActionPlan
	case agentcapabilities.DecideActionCapabilityName:
		activity = config.ExternalAgentActivityActionDecision
	case agentcapabilities.ExecuteActionCapabilityName:
		activity = config.ExternalAgentActivityActionExecute
	default:
		return "", "", false
	}
	requiredScope := strings.TrimSpace(capability.Scope)
	if requiredScope == "" {
		return "", "", false
	}
	return activity, requiredScope, true
}

func (r *Router) recordExternalAgentActivity(req *http.Request, activity string, requiredScopes ...string) {
	if r == nil || req == nil || strings.TrimSpace(activity) == "" {
		return
	}
	token := getAPITokenRecordFromRequest(req)
	if !apiTokenCoversExternalAgentSurface(token, time.Now().UTC(), requiredScopes...) {
		return
	}
	persistence := r.persistenceForOrg(req.Context())
	if persistence == nil {
		return
	}
	if err := persistence.RecordExternalAgentActivity(config.ExternalAgentActivityRecord{
		Timestamp: time.Now().UTC(),
		Surface:   externalAgentActivitySurface(req),
		Activity:  activity,
	}); err != nil {
		log.Debug().Err(err).Str("activity", activity).Msg("Failed to record external agent activity")
	}
}

type agentWorkflowPromptActivityRequest struct {
	Name string `json:"name"`
}

// HandleAgentWorkflowPromptActivity records content-free workflow starter
// usage from agent adapters that render prompts locally, such as pulse-mcp.
func (r *Router) HandleAgentWorkflowPromptActivity(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload agentWorkflowPromptActivityRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4096))
	if err := dec.Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var discard json.RawMessage
	if err := dec.Decode(&discard); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	promptName := strings.TrimSpace(payload.Name)
	if !canonicalWorkflowPromptDeclared(promptName) {
		http.Error(w, "Unknown workflow prompt", http.StatusBadRequest)
		return
	}

	r.recordWorkflowPromptActivity(req, workflowPromptActivitySurface(req), promptName)
	w.WriteHeader(http.StatusNoContent)
}

func canonicalWorkflowPromptDeclared(promptName string) bool {
	promptName = strings.TrimSpace(promptName)
	if promptName == "" {
		return false
	}
	for _, prompt := range agentcapabilities.ManifestPulseWorkflowPrompts(agentcapabilities.CanonicalManifest()) {
		if strings.TrimSpace(prompt.Name) == promptName {
			return true
		}
	}
	return false
}

func (r *Router) recordWorkflowPromptActivity(req *http.Request, surface, promptName string) {
	if r == nil || req == nil || strings.TrimSpace(promptName) == "" {
		return
	}
	token := getAPITokenRecordFromRequest(req)
	if !apiTokenCoversExternalAgentSurface(token, time.Now().UTC(), config.ScopeMonitoringRead) {
		return
	}
	persistence := r.persistenceForOrg(req.Context())
	if persistence == nil {
		return
	}
	if err := persistence.RecordWorkflowPromptActivity(config.WorkflowPromptActivityRecord{
		Timestamp:  time.Now().UTC(),
		Surface:    surface,
		PromptName: promptName,
	}); err != nil {
		log.Debug().Err(err).Str("prompt_name", promptName).Msg("Failed to record workflow prompt activity")
	}
}

func workflowPromptActivitySurface(req *http.Request) string {
	if req == nil {
		return config.WorkflowPromptActivitySurfaceAgentAPI
	}
	switch strings.TrimSpace(req.Header.Get(agentcapabilities.AgentSurfaceHeader)) {
	case agentcapabilities.AgentSurfacePulseMCP:
		return config.WorkflowPromptActivitySurfacePulseMCP
	default:
		return config.WorkflowPromptActivitySurfaceAgentAPI
	}
}

func externalAgentActivitySurface(req *http.Request) string {
	if req == nil {
		return config.ExternalAgentActivitySurfaceAgentAPI
	}
	switch strings.TrimSpace(req.Header.Get(agentcapabilities.AgentSurfaceHeader)) {
	case agentcapabilities.AgentSurfacePulseMCP:
		return config.ExternalAgentActivitySurfacePulseMCP
	default:
		return config.ExternalAgentActivitySurfaceAgentAPI
	}
}

func apiTokenCoversExternalAgentSurface(token *config.APITokenRecord, now time.Time, requiredScopes ...string) bool {
	if token == nil {
		return false
	}
	if token.ExpiresAt != nil && now.After(token.ExpiresAt.UTC()) {
		return false
	}
	if len(requiredScopes) == 0 {
		requiredScopes = agentcapabilities.CanonicalManifest().RequiredScopes
	}
	if len(requiredScopes) == 0 {
		return false
	}
	token = cloneAPITokenRecord(token)
	for _, scope := range requiredScopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return false
		}
		if !token.HasScope(scope) {
			return false
		}
	}
	return true
}

func cloneAPITokenRecord(token *config.APITokenRecord) *config.APITokenRecord {
	if token == nil {
		return nil
	}
	clone := token.Clone()
	return &clone
}
