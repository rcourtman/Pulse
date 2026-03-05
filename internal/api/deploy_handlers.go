package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// DeployHandlers provides HTTP handlers for cluster agent deployment.
type DeployHandlers struct {
	store       *deploy.Store
	monitor     *monitoring.Monitor
	execServer  *agentexec.Server
	reservation *deploy.ReservationManager

	// resolvePublicURL derives the Pulse URL for agent reachability checks.
	resolvePublicURL func(req *http.Request) string

	// config and persistence for token minting/validation in enroll flow.
	config      *config.Config
	persistence *config.ConfigPersistence

	// Active preflight SSE subscriptions keyed by preflightID.
	sseMu   sync.Mutex
	sseSubs map[string]*deploySSESub
}

// deploySSESub tracks SSE clients for a single preflight job.
type deploySSESub struct {
	clients map[string]chan []byte // clientID -> event channel
	mu      sync.Mutex
}

// NewDeployHandlers creates a DeployHandlers instance.
func NewDeployHandlers(
	store *deploy.Store,
	monitor *monitoring.Monitor,
	execServer *agentexec.Server,
	reservation *deploy.ReservationManager,
	resolvePublicURL func(req *http.Request) string,
	cfg *config.Config,
	persistence *config.ConfigPersistence,
) *DeployHandlers {
	return &DeployHandlers{
		store:            store,
		monitor:          monitor,
		execServer:       execServer,
		reservation:      reservation,
		resolvePublicURL: resolvePublicURL,
		config:           cfg,
		persistence:      persistence,
		sseSubs:          make(map[string]*deploySSESub),
	}
}

// --- Candidates ---

// candidateNode is the per-node response in the candidates list.
type candidateNode struct {
	NodeID     string `json:"nodeId"`
	Name       string `json:"name"`
	IP         string `json:"ip,omitempty"`
	HasAgent   bool   `json:"hasAgent"`
	Deployable bool   `json:"deployable"`
	Reason     string `json:"reason,omitempty"`
}

// sourceAgentInfo describes a connected agent that can execute SSH to peers.
type sourceAgentInfo struct {
	AgentID string `json:"agentId"`
	NodeID  string `json:"nodeId"`
	Online  bool   `json:"online"`
}

type candidatesResponse struct {
	ClusterID    string            `json:"clusterId"`
	ClusterName  string            `json:"clusterName"`
	SourceAgents []sourceAgentInfo `json:"sourceAgents"`
	Nodes        []candidateNode   `json:"nodes"`
}

// HandleCandidates returns deployment candidate nodes for a cluster.
// GET /api/clusters/{clusterId}/agent-deploy/candidates
func (h *DeployHandlers) HandleCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clusterID := extractClusterID(r.URL.Path, "/api/clusters/", "/agent-deploy/candidates")
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cluster_id", "Cluster ID is required", nil)
		return
	}

	readState := h.monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "state_unavailable", "Resource state is unavailable", nil)
		return
	}

	// Build connected agents set
	connectedAgents := make(map[string]bool)
	for _, agent := range h.execServer.GetConnectedAgents() {
		connectedAgents[agent.AgentID] = true
	}

	var (
		clusterName  string
		nodes        []candidateNode
		sourceAgents []sourceAgentInfo
	)

	for _, node := range readState.Nodes() {
		if node == nil {
			continue
		}
		if !node.IsClusterMember() {
			continue
		}
		// Match cluster by name (clusterID in URL = cluster name).
		if node.ClusterName() != clusterID {
			continue
		}
		if clusterName == "" {
			clusterName = node.ClusterName()
		}

		hasAgent := node.LinkedAgentID() != ""
		cn := candidateNode{
			NodeID:   node.ID(),
			Name:     nodeName(node),
			IP:       nodeIP(node.HostURL()),
			HasAgent: hasAgent,
		}

		if hasAgent {
			cn.Deployable = false
			cn.Reason = "already_agent"

			// This node has an agent — check if it's a source candidate.
			hostID := node.LinkedAgentID()
			if connectedAgents[hostID] {
				sourceAgents = append(sourceAgents, sourceAgentInfo{
					AgentID: hostID,
					NodeID:  node.ID(),
					Online:  true,
				})
			}
		} else {
			cn.Deployable = true
		}

		nodes = append(nodes, cn)
	}

	resp := candidatesResponse{
		ClusterID:    clusterID,
		ClusterName:  clusterName,
		SourceAgents: sourceAgents,
		Nodes:        nodes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Preflight ---

type createPreflightRequest struct {
	SourceAgentID string   `json:"sourceAgentId"`
	TargetNodeIDs []string `json:"targetNodeIds"`
	MaxParallel   int      `json:"maxParallel"`
}

type createPreflightResponse struct {
	PreflightID string `json:"preflightId"`
	Status      string `json:"status"`
	EventsURL   string `json:"eventsUrl"`
}

// HandleCreatePreflight creates a preflight job and dispatches to the source agent.
// POST /api/clusters/{clusterId}/agent-deploy/preflights
func (h *DeployHandlers) HandleCreatePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clusterID := extractClusterID(r.URL.Path, "/api/clusters/", "/agent-deploy/preflights")
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cluster_id", "Cluster ID is required", nil)
		return
	}

	var req createPreflightRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return
	}

	req.SourceAgentID = strings.TrimSpace(req.SourceAgentID)
	if req.SourceAgentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_source_agent", "sourceAgentId is required", nil)
		return
	}
	if len(req.TargetNodeIDs) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "missing_targets", "At least one targetNodeIds entry is required", nil)
		return
	}
	if len(req.TargetNodeIDs) > 100 {
		writeErrorResponse(w, http.StatusBadRequest, "too_many_targets", "Maximum 100 targets per preflight", nil)
		return
	}

	// Verify source agent is connected.
	if !h.execServer.IsAgentConnected(req.SourceAgentID) {
		writeErrorResponse(w, http.StatusConflict, "source_agent_offline", "Source agent is not connected", nil)
		return
	}

	// Resolve cluster nodes from read-state.
	readState := h.monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "state_unavailable", "Resource state is unavailable", nil)
		return
	}

	clusterName := ""
	sourceNodeID := ""
	nodesByID := make(map[string]*unifiedresources.NodeView)
	for _, node := range readState.Nodes() {
		if node == nil {
			continue
		}
		if node.ClusterName() == clusterID && node.IsClusterMember() {
			nodesByID[node.ID()] = node
			if clusterName == "" {
				clusterName = node.ClusterName()
			}
			if node.LinkedAgentID() == req.SourceAgentID {
				sourceNodeID = node.ID()
			}
		}
	}

	if sourceNodeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "source_not_in_cluster",
			"Source agent is not linked to a node in this cluster", nil)
		return
	}

	// Build deploy targets from requested node IDs.
	now := time.Now().UTC()
	jobID := generateID("pf")
	maxParallel := req.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 2
	}
	if maxParallel > 10 {
		maxParallel = 10
	}

	job := &deploy.Job{
		ID:            jobID,
		ClusterID:     clusterID,
		ClusterName:   clusterName,
		SourceAgentID: req.SourceAgentID,
		SourceNodeID:  sourceNodeID,
		OrgID:         resolveTenantOrgID(r),
		Status:        deploy.JobQueued,
		MaxParallel:   maxParallel,
		RetryMax:      0, // preflights don't retry
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	ctx := r.Context()
	if err := h.store.CreateJob(ctx, job); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to create preflight job")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to create preflight job", nil)
		return
	}

	var targets []agentexec.DeployPreflightTarget
	for _, nodeID := range req.TargetNodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		node, ok := nodesByID[nodeID]
		if !ok {
			continue // skip nodes not in cluster
		}
		ip := nodeIP(node.HostURL())
		if ip == "" {
			continue // skip nodes without IP
		}

		targetID := generateID("tgt")
		target := &deploy.Target{
			ID:        targetID,
			JobID:     jobID,
			NodeID:    nodeID,
			NodeName:  nodeName(node),
			NodeIP:    ip,
			Status:    deploy.TargetPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.store.CreateTarget(ctx, target); err != nil {
			log.Error().Err(err).Str("target_id", targetID).Msg("Failed to create preflight target")
			continue
		}
		targets = append(targets, agentexec.DeployPreflightTarget{
			TargetID: targetID,
			NodeName: nodeName(node),
			NodeIP:   ip,
		})
	}

	if len(targets) == 0 {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		writeErrorResponse(w, http.StatusBadRequest, "no_valid_targets",
			"None of the requested nodes are valid deployment targets", nil)
		return
	}

	// Resolve Pulse URL for agent reachability.
	pulseURL := h.resolvePublicURL(r)
	if pulseURL == "" {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		writeErrorResponse(w, http.StatusInternalServerError, "no_pulse_url",
			"Cannot determine Pulse URL for agent reachability", nil)
		return
	}

	requestID := generateID("req")
	payload := agentexec.DeployPreflightPayload{
		RequestID:   requestID,
		JobID:       jobID,
		Targets:     targets,
		PulseURL:    pulseURL,
		MaxParallel: maxParallel,
		Timeout:     120,
	}

	// Transition to running.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobRunning)

	// Append job-created event.
	_ = h.store.AppendEvent(ctx, &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventJobCreated,
		Message:   fmt.Sprintf("Preflight started for %d targets", len(targets)),
		CreatedAt: now,
	})

	// Subscribe to progress before sending command to avoid race.
	progressCh := h.execServer.SubscribeDeployProgress(req.SourceAgentID, jobID, 64)

	// Send command to agent.
	if err := h.execServer.SendDeployPreflight(ctx, req.SourceAgentID, payload); err != nil {
		h.execServer.UnsubscribeDeployProgress(req.SourceAgentID, jobID)
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to send preflight command")
		writeErrorResponse(w, http.StatusInternalServerError, "send_failed",
			"Failed to send preflight command to agent", nil)
		return
	}

	// Start background goroutine to process progress events.
	go h.processPreflightProgress(jobID, req.SourceAgentID, progressCh)

	resp := createPreflightResponse{
		PreflightID: jobID,
		Status:      string(deploy.JobRunning),
		EventsURL:   fmt.Sprintf("/api/agent-deploy/preflights/%s/events", jobID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// HandleGetPreflight returns the current status of a preflight job.
// GET /api/agent-deploy/preflights/{preflightId}
func (h *DeployHandlers) HandleGetPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	preflightID := extractPathSuffix(r.URL.Path, "/api/agent-deploy/preflights/")
	if preflightID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Preflight ID is required", nil)
		return
	}
	// Strip /events suffix if present (shouldn't happen via routing, but be safe).
	preflightID = strings.TrimSuffix(preflightID, "/events")

	job, err := h.store.GetJob(r.Context(), preflightID)
	if err != nil {
		log.Error().Err(err).Str("id", preflightID).Msg("Failed to get preflight job")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get preflight", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Preflight not found", nil)
		return
	}

	// Tenant isolation: verify the job belongs to the caller's org.
	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Preflight not found", nil)
		return
	}

	targets, err := h.store.GetTargetsForJob(r.Context(), preflightID)
	if err != nil {
		log.Error().Err(err).Str("id", preflightID).Msg("Failed to get preflight targets")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get targets", nil)
		return
	}

	resp := struct {
		*deploy.Job
		Targets []deploy.Target `json:"targets"`
	}{
		Job:     job,
		Targets: targets,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandlePreflightEvents streams SSE events for a preflight job.
// GET /api/agent-deploy/preflights/{preflightId}/events
func (h *DeployHandlers) HandlePreflightEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract preflight ID: /api/agent-deploy/preflights/{id}/events
	path := strings.TrimPrefix(r.URL.Path, "/api/agent-deploy/preflights/")
	preflightID := strings.TrimSuffix(path, "/events")
	if preflightID == "" || preflightID == path {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Preflight ID is required", nil)
		return
	}

	// Verify the preflight exists.
	job, err := h.store.GetJob(r.Context(), preflightID)
	if err != nil {
		log.Error().Err(err).Str("id", preflightID).Msg("Failed to get preflight job for SSE")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get preflight", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Preflight not found", nil)
		return
	}

	// Tenant isolation: verify the job belongs to the caller's org.
	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Preflight not found", nil)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorResponse(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported", nil)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Register SSE client.
	clientID := generateID("sse")
	eventCh := h.addSSEClient(preflightID, clientID)
	defer h.removeSSEClient(preflightID, clientID)

	// Send existing events first (replay).
	events, replayErr := h.store.GetEventsForJob(r.Context(), preflightID)
	if replayErr != nil {
		log.Error().Err(replayErr).Str("id", preflightID).Msg("Failed to load events for SSE replay")
		// Send an error event so the client knows replay is incomplete.
		fmt.Fprintf(w, "event: error\ndata: {\"message\":\"failed to load event history\"}\n\n")
	}
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// If job is already terminal, send final status and close.
	if isDeployJobTerminal(job.Status) {
		data, _ := json.Marshal(map[string]string{
			"type":   "job_complete",
			"status": string(job.Status),
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Stream new events.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case eventData, ok := <-eventCh:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// --- Progress processing ---

// processPreflightProgress reads deploy progress events from the agent and
// persists them as deploy events, also broadcasting to SSE clients.
func (h *DeployHandlers) processPreflightProgress(jobID, agentID string, ch <-chan agentexec.DeployProgressPayload) {
	defer h.execServer.UnsubscribeDeployProgress(agentID, jobID)

	ctx := context.Background()

	for progress := range ch {
		// Persist as event.
		evt := &deploy.Event{
			ID:        generateID("evt"),
			JobID:     jobID,
			TargetID:  progress.TargetID,
			Type:      deploy.EventPreflightResult,
			Message:   progress.Message,
			Data:      progress.Data,
			CreatedAt: time.Now().UTC(),
		}
		if err := h.store.AppendEvent(ctx, evt); err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to persist deploy event")
		}

		// Update target status based on progress phase.
		if progress.TargetID != "" {
			h.updateTargetFromProgress(ctx, progress)
		}

		// Broadcast to SSE clients.
		h.broadcastSSE(jobID, evt)

		if progress.Final {
			// Derive final job status from target statuses.
			// For preflights, TargetReady means "passed" (not active).
			finalStatus := derivePreflightJobStatus(ctx, h.store, jobID)
			_ = h.store.UpdateJobStatus(ctx, jobID, finalStatus)

			// Broadcast final status.
			finalEvt := &deploy.Event{
				ID:        generateID("evt"),
				JobID:     jobID,
				Type:      deploy.EventJobStatusChanged,
				Message:   fmt.Sprintf("Preflight completed: %s", finalStatus),
				CreatedAt: time.Now().UTC(),
			}
			_ = h.store.AppendEvent(ctx, finalEvt)
			h.broadcastSSE(jobID, finalEvt)

			// Close SSE channels for this job.
			h.closeSSESub(jobID)
			return
		}
	}

	// Channel closed without final — agent disconnected.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
	finalEvt := &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventError,
		Message:   "Source agent disconnected during preflight",
		CreatedAt: time.Now().UTC(),
	}
	_ = h.store.AppendEvent(ctx, finalEvt)
	h.broadcastSSE(jobID, finalEvt)
	h.closeSSESub(jobID)
}

// updateTargetFromProgress maps progress phases to target status transitions.
func (h *DeployHandlers) updateTargetFromProgress(ctx context.Context, p agentexec.DeployProgressPayload) {
	var newStatus deploy.TargetStatus
	var errMsg string

	switch {
	case p.Phase == agentexec.DeployPhasePreflightComplete && p.Status == agentexec.DeployStepOK:
		newStatus = deploy.TargetReady
	case p.Phase == agentexec.DeployPhasePreflightComplete && p.Status == agentexec.DeployStepFailed:
		newStatus = deploy.TargetFailedPermanent
		errMsg = p.Message
	case p.Phase == agentexec.DeployPhasePreflightComplete && p.Status == agentexec.DeployStepSkipped:
		newStatus = deploy.TargetSkippedAgent
	case p.Phase == agentexec.DeployPhasePreflightSSH && p.Status == agentexec.DeployStepStarted:
		newStatus = deploy.TargetPreflighting
	case p.Phase == agentexec.DeployPhasePreflightSSH && p.Status == agentexec.DeployStepFailed:
		newStatus = deploy.TargetFailedPermanent
		errMsg = p.Message
	case p.Phase == agentexec.DeployPhaseCanceled:
		newStatus = deploy.TargetCanceled
	default:
		return // intermediate step, no status change
	}

	if err := h.store.UpdateTargetStatus(ctx, p.TargetID, newStatus, errMsg); err != nil {
		log.Error().Err(err).
			Str("target_id", p.TargetID).
			Str("new_status", string(newStatus)).
			Msg("Failed to update target status from progress")
	}
}

// --- SSE subscription management ---

func (h *DeployHandlers) addSSEClient(jobID, clientID string) chan []byte {
	h.sseMu.Lock()
	defer h.sseMu.Unlock()

	sub, ok := h.sseSubs[jobID]
	if !ok {
		sub = &deploySSESub{clients: make(map[string]chan []byte)}
		h.sseSubs[jobID] = sub
	}

	ch := make(chan []byte, 64)
	sub.mu.Lock()
	sub.clients[clientID] = ch
	sub.mu.Unlock()
	return ch
}

func (h *DeployHandlers) removeSSEClient(jobID, clientID string) {
	h.sseMu.Lock()
	sub, ok := h.sseSubs[jobID]
	h.sseMu.Unlock()
	if !ok {
		return
	}

	sub.mu.Lock()
	if ch, exists := sub.clients[clientID]; exists {
		close(ch)
		delete(sub.clients, clientID)
	}
	sub.mu.Unlock()
}

func (h *DeployHandlers) broadcastSSE(jobID string, evt *deploy.Event) {
	h.sseMu.Lock()
	sub, ok := h.sseSubs[jobID]
	h.sseMu.Unlock()
	if !ok {
		return
	}

	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	sub.mu.Lock()
	defer sub.mu.Unlock()
	for _, ch := range sub.clients {
		select {
		case ch <- data:
		default:
			// Drop if client is slow.
		}
	}
}

func (h *DeployHandlers) closeSSESub(jobID string) {
	h.sseMu.Lock()
	sub, ok := h.sseSubs[jobID]
	if ok {
		delete(h.sseSubs, jobID)
	}
	h.sseMu.Unlock()

	if !ok {
		return
	}

	sub.mu.Lock()
	for id, ch := range sub.clients {
		close(ch)
		delete(sub.clients, id)
	}
	sub.mu.Unlock()
}

// --- Bootstrap Enrollment ---

// MintBootstrapTokenForTarget creates a single-use bootstrap token for a deploy target.
// Used by the deploy job creation flow to issue per-target tokens.
func (h *DeployHandlers) MintBootstrapTokenForTarget(req deploy.BootstrapTokenRequest) (rawToken string, tokenID string, err error) {
	if req.TTL <= 0 {
		return "", "", fmt.Errorf("TTL must be positive, got %v", req.TTL)
	}

	raw, err := auth.GenerateAPIToken()
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	record, err := config.NewAPITokenRecord(raw,
		fmt.Sprintf("deploy-bootstrap:%s:%s", req.JobID, req.TargetID),
		[]string{config.ScopeAgentEnroll})
	if err != nil {
		return "", "", fmt.Errorf("create token record: %w", err)
	}

	exp := time.Now().UTC().Add(req.TTL)
	record.ExpiresAt = &exp
	record.OrgID = req.OrgID
	record.Metadata = req.BuildMetadata()

	config.Mu.Lock()
	h.config.UpsertAPIToken(*record)
	tokens := make([]config.APITokenRecord, len(h.config.APITokens))
	copy(tokens, h.config.APITokens)
	config.Mu.Unlock()

	if h.persistence != nil {
		if err := h.persistence.SaveAPITokens(tokens); err != nil {
			log.Warn().Err(err).Msg("Failed to persist bootstrap token")
		}
	}

	return raw, record.ID, nil
}

// enrollRequest matches the design doc Section 3 enrollment payload.
type enrollRequest struct {
	Hostname        string `json:"hostname"`
	FQDN            string `json:"fqdn,omitempty"`
	MachineID       string `json:"machineId,omitempty"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	AgentVersion    string `json:"agentVersion"`
	CommandsEnabled bool   `json:"commandsEnabled,omitempty"`
	Proxmox         *struct {
		ClusterName string `json:"clusterName,omitempty"`
		NodeName    string `json:"nodeName,omitempty"`
	} `json:"proxmox,omitempty"`
	DeployJobID string `json:"deployJobId,omitempty"`
}

// HandleEnroll processes bootstrap token enrollment from freshly-deployed agents.
// POST /api/agents/agent/enroll
func (h *DeployHandlers) HandleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decode request body.
	var req enrollRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	if req.Hostname == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_hostname", "hostname is required", nil)
		return
	}

	// 2. Get bootstrap token from context (set by RequireAuth middleware).
	bootstrapToken := getAPITokenRecordFromRequest(r)
	if bootstrapToken == nil {
		writeErrorResponse(w, http.StatusUnauthorized, "no_token", "Bootstrap token required", nil)
		return
	}

	// 3. Validate token binding metadata.
	meta := bootstrapToken.Metadata
	if meta == nil {
		writeErrorResponse(w, http.StatusForbidden, "invalid_token", "Token is not a bootstrap deploy token", nil)
		return
	}
	jobID := meta[deploy.MetaKeyJobID]
	targetID := meta[deploy.MetaKeyTargetID]
	expectedNode := meta[deploy.MetaKeyExpectedNode]

	if jobID == "" || targetID == "" {
		writeErrorResponse(w, http.StatusForbidden, "invalid_token", "Token missing deploy binding", nil)
		return
	}

	// 4. Validate node name binding (if set).
	if expectedNode != "" && req.Hostname != expectedNode {
		proxmoxMatch := false
		if req.Proxmox != nil && req.Proxmox.NodeName == expectedNode {
			proxmoxMatch = true
		}
		if !proxmoxMatch {
			writeErrorResponse(w, http.StatusForbidden, "binding_mismatch",
				fmt.Sprintf("Token bound to node %q, got hostname %q", expectedNode, req.Hostname), nil)
			return
		}
	}

	// 5. Verify deploy target exists and is in correct state.
	ctx := r.Context()
	target, err := h.store.GetTarget(ctx, targetID)
	if err != nil || target == nil {
		writeErrorResponse(w, http.StatusNotFound, "target_not_found", "Deploy target not found", nil)
		return
	}
	if target.Status != deploy.TargetEnrolling && target.Status != deploy.TargetInstalling {
		writeErrorResponse(w, http.StatusConflict, "invalid_target_state",
			fmt.Sprintf("Target is in state %q, expected enrolling or installing", target.Status), nil)
		return
	}

	// 6. Verify target belongs to the job referenced in the token.
	if target.JobID != jobID {
		writeErrorResponse(w, http.StatusForbidden, "binding_mismatch",
			"Token job binding does not match target", nil)
		return
	}

	// 7. Invalidate bootstrap token (single-use) BEFORE minting runtime token.
	// Check return value to prevent concurrent replay.
	config.Mu.Lock()
	removed := h.config.RemoveAPIToken(bootstrapToken.ID)
	tokensAfterRemove := make([]config.APITokenRecord, len(h.config.APITokens))
	copy(tokensAfterRemove, h.config.APITokens)
	config.Mu.Unlock()
	if removed == nil {
		writeErrorResponse(w, http.StatusConflict, "token_already_consumed",
			"Bootstrap token has already been used", nil)
		return
	}
	if h.persistence != nil {
		if err := h.persistence.SaveAPITokens(tokensAfterRemove); err != nil {
			log.Warn().Err(err).Msg("Failed to persist token removal during enroll")
		}
	}

	// 8. Mint runtime token (long-lived, host-bound).
	runtimeRaw, err := auth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate runtime token during enroll")
		writeErrorResponse(w, http.StatusInternalServerError, "token_error", "Failed to generate runtime token", nil)
		return
	}
	runtimeScopes := []string{
		config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeAgentManage,
		config.ScopeDockerReport, config.ScopeKubernetesReport,
	}
	if req.CommandsEnabled {
		runtimeScopes = append(runtimeScopes, config.ScopeAgentExec)
	}
	runtimeRecord, err := config.NewAPITokenRecord(runtimeRaw,
		fmt.Sprintf("agent:%s", req.Hostname),
		runtimeScopes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create runtime token record during enroll")
		writeErrorResponse(w, http.StatusInternalServerError, "token_error", "Failed to create runtime token", nil)
		return
	}
	runtimeRecord.OrgID = bootstrapToken.OrgID
	runtimeRecord.Metadata = map[string]string{
		"bound_hostname": req.Hostname,
		"deploy_job_id":  jobID,
	}

	config.Mu.Lock()
	h.config.UpsertAPIToken(*runtimeRecord)
	tokensAfterMint := make([]config.APITokenRecord, len(h.config.APITokens))
	copy(tokensAfterMint, h.config.APITokens)
	config.Mu.Unlock()
	if h.persistence != nil {
		if err := h.persistence.SaveAPITokens(tokensAfterMint); err != nil {
			log.Warn().Err(err).Msg("Failed to persist runtime token during enroll")
		}
	}

	// 9. Update target status to VERIFYING.
	_ = h.store.UpdateTargetStatus(ctx, targetID, deploy.TargetVerifying, "")

	// 10. Append enroll event.
	enrollEvt := &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		TargetID:  targetID,
		Type:      deploy.EventEnrollComplete,
		Message:   fmt.Sprintf("Agent enrolled from %s", req.Hostname),
		CreatedAt: time.Now().UTC(),
	}
	_ = h.store.AppendEvent(ctx, enrollEvt)

	// 11. Broadcast to SSE clients.
	h.broadcastSSE(jobID, enrollEvt)

	// 12. Return runtime token + config to agent.
	canonicalAgentID := fmt.Sprintf("agent-%s", req.Hostname)
	resp := map[string]any{
		"agentId":        canonicalAgentID,
		"runtimeToken":   runtimeRaw,
		"runtimeTokenId": runtimeRecord.ID,
		"reportInterval": "30s",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// --- Deploy Jobs ---

type createJobRequest struct {
	SourceAgentID string   `json:"sourceAgentId"`
	PreflightID   string   `json:"preflightId"`
	TargetNodeIDs []string `json:"targetNodeIds"`
	Mode          string   `json:"mode"`
	MaxParallel   int      `json:"maxParallel"`
	RetryPolicy   *struct {
		MaxAttempts int `json:"maxAttempts"`
	} `json:"retryPolicy,omitempty"`
}

type createJobSkip struct {
	NodeID string `json:"nodeId"`
	Reason string `json:"reason"`
}

type createJobResponse struct {
	JobID                string          `json:"jobId"`
	AcceptedTargets      []string        `json:"acceptedTargets"`
	SkippedTargets       []createJobSkip `json:"skippedTargets"`
	ReservedLicenseSlots int             `json:"reservedLicenseSlots"`
	EventsURL            string          `json:"eventsUrl"`
}

// HandleCreateJob creates a deploy install job from preflight results.
// POST /api/clusters/{clusterId}/agent-deploy/jobs
func (h *DeployHandlers) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clusterID := extractClusterID(r.URL.Path, "/api/clusters/", "/agent-deploy/jobs")
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cluster_id", "Cluster ID is required", nil)
		return
	}

	var req createJobRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return
	}

	req.SourceAgentID = strings.TrimSpace(req.SourceAgentID)
	if req.SourceAgentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_source_agent", "sourceAgentId is required", nil)
		return
	}
	req.PreflightID = strings.TrimSpace(req.PreflightID)
	if req.PreflightID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_preflight_id", "preflightId is required", nil)
		return
	}
	if len(req.TargetNodeIDs) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "missing_targets", "At least one targetNodeIds entry is required", nil)
		return
	}

	// Verify source agent is connected.
	if !h.execServer.IsAgentConnected(req.SourceAgentID) {
		writeErrorResponse(w, http.StatusConflict, "source_agent_offline", "Source agent is not connected", nil)
		return
	}

	ctx := r.Context()
	orgID := resolveTenantOrgID(r)

	// Verify preflight exists and belongs to same org.
	pfJob, err := h.store.GetJob(ctx, req.PreflightID)
	if err != nil {
		log.Error().Err(err).Str("preflight_id", req.PreflightID).Msg("Failed to get preflight job")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get preflight job", nil)
		return
	}
	if pfJob == nil {
		writeErrorResponse(w, http.StatusNotFound, "preflight_not_found", "Preflight job not found", nil)
		return
	}
	if pfJob.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "preflight_not_found", "Preflight job not found", nil)
		return
	}
	if pfJob.Status != deploy.JobSucceeded && pfJob.Status != deploy.JobPartialSuccess {
		writeErrorResponse(w, http.StatusConflict, "preflight_not_passed",
			fmt.Sprintf("Preflight is in state %q, expected succeeded or partial_success", pfJob.Status), nil)
		return
	}

	// Verify cluster and source agent consistency.
	if pfJob.ClusterID != clusterID {
		writeErrorResponse(w, http.StatusBadRequest, "cluster_mismatch",
			"Preflight cluster does not match request cluster", nil)
		return
	}
	if pfJob.SourceAgentID != req.SourceAgentID {
		writeErrorResponse(w, http.StatusBadRequest, "source_agent_mismatch",
			"Preflight source agent does not match request source agent", nil)
		return
	}

	// Get preflight targets — filter requested nodeIDs against Ready targets.
	pfTargets, err := h.store.GetTargetsForJob(ctx, req.PreflightID)
	if err != nil {
		log.Error().Err(err).Str("preflight_id", req.PreflightID).Msg("Failed to get preflight targets")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get preflight targets", nil)
		return
	}

	// Build lookup of preflight targets by node ID.
	pfTargetByNode := make(map[string]*deploy.Target)
	for i := range pfTargets {
		pfTargetByNode[pfTargets[i].NodeID] = &pfTargets[i]
	}

	// Deduplicate requested nodes while preserving request order.
	seen := make(map[string]bool, len(req.TargetNodeIDs))
	var orderedNodeIDs []string
	for _, nid := range req.TargetNodeIDs {
		nid = strings.TrimSpace(nid)
		if nid != "" && !seen[nid] {
			seen[nid] = true
			orderedNodeIDs = append(orderedNodeIDs, nid)
		}
	}

	// Filter: only accept targets that passed preflight (Ready state).
	// Order is preserved from the request so license truncation is deterministic.
	var acceptedPfTargets []*deploy.Target
	var skipped []createJobSkip
	for _, nodeID := range orderedNodeIDs {
		pfTgt, ok := pfTargetByNode[nodeID]
		if !ok {
			skipped = append(skipped, createJobSkip{NodeID: nodeID, Reason: "not_in_preflight"})
			continue
		}
		if pfTgt.Status != deploy.TargetReady {
			skipped = append(skipped, createJobSkip{NodeID: nodeID, Reason: fmt.Sprintf("preflight_status_%s", pfTgt.Status)})
			continue
		}
		acceptedPfTargets = append(acceptedPfTargets, pfTgt)
	}

	// License slot check.
	maxLimit := maxAgentsLimitForContext(ctx)
	if maxLimit > 0 {
		currentCount := agentCount(h.monitor)
		reservedCount := h.reservation.ReservedForOrg(orgID)
		available := maxLimit - currentCount - reservedCount
		if available < 0 {
			available = 0
		}
		if available < len(acceptedPfTargets) {
			// Accept only what fits; skip the rest.
			for i := available; i < len(acceptedPfTargets); i++ {
				skipped = append(skipped, createJobSkip{
					NodeID: acceptedPfTargets[i].NodeID,
					Reason: "skipped_license",
				})
			}
			acceptedPfTargets = acceptedPfTargets[:available]
		}
	}

	if len(acceptedPfTargets) == 0 {
		writeErrorResponse(w, http.StatusConflict, "no_eligible_targets",
			"No targets are eligible for deployment", nil)
		return
	}

	// Create deploy job.
	now := time.Now().UTC()
	jobID := generateID("dep")
	maxParallel := req.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 2
	}
	if maxParallel > 10 {
		maxParallel = 10
	}

	retryMax := 3
	if req.RetryPolicy != nil && req.RetryPolicy.MaxAttempts > 0 {
		retryMax = req.RetryPolicy.MaxAttempts
	}

	job := &deploy.Job{
		ID:            jobID,
		ClusterID:     clusterID,
		ClusterName:   pfJob.ClusterName,
		SourceAgentID: req.SourceAgentID,
		SourceNodeID:  pfJob.SourceNodeID,
		OrgID:         orgID,
		Status:        deploy.JobQueued,
		MaxParallel:   maxParallel,
		RetryMax:      retryMax,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.store.CreateJob(ctx, job); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to create deploy job")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to create deploy job", nil)
		return
	}

	// Resolve Pulse URL for install commands.
	pulseURL := h.resolvePublicURL(r)
	if pulseURL == "" {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		writeErrorResponse(w, http.StatusInternalServerError, "no_pulse_url",
			"Cannot determine Pulse URL for agent installation", nil)
		return
	}

	// Create targets and mint bootstrap tokens.
	var installTargets []agentexec.DeployInstallTarget
	var acceptedNodeIDs []string
	for _, pfTgt := range acceptedPfTargets {
		targetID := generateID("tgt")
		arch := h.getTargetArchFromPreflight(ctx, req.PreflightID, pfTgt.NodeID)

		target := &deploy.Target{
			ID:        targetID,
			JobID:     jobID,
			NodeID:    pfTgt.NodeID,
			NodeName:  pfTgt.NodeName,
			NodeIP:    pfTgt.NodeIP,
			Arch:      arch,
			Status:    deploy.TargetPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.store.CreateTarget(ctx, target); err != nil {
			log.Error().Err(err).Str("target_id", targetID).Msg("Failed to create deploy target")
			continue
		}

		// Mint bootstrap token for this target.
		rawToken, _, err := h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
			ClusterID:     clusterID,
			NodeID:        pfTgt.NodeID,
			ExpectedNode:  pfTgt.NodeName,
			JobID:         jobID,
			TargetID:      targetID,
			SourceAgentID: req.SourceAgentID,
			OrgID:         orgID,
			TTL:           30 * time.Minute,
		})
		if err != nil {
			log.Error().Err(err).Str("target_id", targetID).Msg("Failed to mint bootstrap token")
			_ = h.store.UpdateTargetStatus(ctx, targetID, deploy.TargetFailedPermanent, "failed to mint bootstrap token")
			continue
		}

		installTargets = append(installTargets, agentexec.DeployInstallTarget{
			TargetID:       targetID,
			NodeName:       pfTgt.NodeName,
			NodeIP:         pfTgt.NodeIP,
			Arch:           arch,
			BootstrapToken: rawToken,
		})
		acceptedNodeIDs = append(acceptedNodeIDs, pfTgt.NodeID)
	}

	if len(installTargets) == 0 {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		writeErrorResponse(w, http.StatusInternalServerError, "target_setup_failed",
			"Failed to set up any deployment targets", nil)
		return
	}

	// Reserve license slots based on actual dispatched target count.
	if err := h.reservation.Reserve(jobID, orgID, len(installTargets), 1*time.Hour); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to reserve license slots")
		// Non-fatal — continue. The reservation is for proactive slot tracking.
	}

	// Transition to running.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobRunning)

	// Append job-created event.
	_ = h.store.AppendEvent(ctx, &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventJobCreated,
		Message:   fmt.Sprintf("Deploy started for %d targets", len(installTargets)),
		CreatedAt: now,
	})

	requestID := generateID("req")
	payload := agentexec.DeployInstallPayload{
		RequestID:   requestID,
		JobID:       jobID,
		Targets:     installTargets,
		PulseURL:    pulseURL,
		MaxParallel: maxParallel,
		Timeout:     300,
	}

	// Subscribe to progress before sending command.
	progressCh := h.execServer.SubscribeDeployProgress(req.SourceAgentID, jobID, 64)

	// Send install command to agent.
	if err := h.execServer.SendDeployInstall(ctx, req.SourceAgentID, payload); err != nil {
		h.execServer.UnsubscribeDeployProgress(req.SourceAgentID, jobID)
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		h.reservation.Release(jobID)
		// Mark pending targets as failed so they're eligible for retry.
		for _, it := range installTargets {
			_ = h.store.UpdateTargetStatus(ctx, it.TargetID, deploy.TargetFailedRetryable, "dispatch failed")
		}
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to send install command")
		writeErrorResponse(w, http.StatusInternalServerError, "send_failed",
			"Failed to send install command to agent", nil)
		return
	}

	// Start background goroutine to process progress events.
	go h.processInstallProgress(jobID, req.SourceAgentID, job.RetryMax, progressCh)

	resp := createJobResponse{
		JobID:                jobID,
		AcceptedTargets:      acceptedNodeIDs,
		SkippedTargets:       skipped,
		ReservedLicenseSlots: len(installTargets),
		EventsURL:            fmt.Sprintf("/api/agent-deploy/jobs/%s/events", jobID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// HandleGetJob returns the current status of a deploy job.
// GET /api/agent-deploy/jobs/{jobId}
func (h *DeployHandlers) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractPathSuffix(r.URL.Path, "/api/agent-deploy/jobs/")
	if jobID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Job ID is required", nil)
		return
	}
	jobID = strings.TrimSuffix(jobID, "/events")

	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get deploy job")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get job", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	targets, err := h.store.GetTargetsForJob(r.Context(), jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get job targets")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get targets", nil)
		return
	}

	resp := struct {
		*deploy.Job
		Targets []deploy.Target `json:"targets"`
	}{
		Job:     job,
		Targets: targets,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleJobEvents streams SSE events for a deploy job.
// GET /api/agent-deploy/jobs/{jobId}/events
func (h *DeployHandlers) HandleJobEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID: /api/agent-deploy/jobs/{id}/events
	path := strings.TrimPrefix(r.URL.Path, "/api/agent-deploy/jobs/")
	jobID := strings.TrimSuffix(path, "/events")
	if jobID == "" || jobID == path {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Job ID is required", nil)
		return
	}

	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get deploy job for SSE")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get job", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorResponse(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported", nil)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientID := generateID("sse")
	eventCh := h.addSSEClient(jobID, clientID)
	defer h.removeSSEClient(jobID, clientID)

	// Replay existing events.
	events, replayErr := h.store.GetEventsForJob(r.Context(), jobID)
	if replayErr != nil {
		log.Error().Err(replayErr).Str("id", jobID).Msg("Failed to load events for SSE replay")
		fmt.Fprintf(w, "event: error\ndata: {\"message\":\"failed to load event history\"}\n\n")
	}
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// If job is terminal, send final and close.
	if isDeployJobTerminal(job.Status) {
		data, _ := json.Marshal(map[string]string{
			"type":   "job_complete",
			"status": string(job.Status),
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Stream new events.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case eventData, ok := <-eventCh:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// HandleCancelJob cancels a running deploy job.
// POST /api/agent-deploy/jobs/{jobId}/cancel
func (h *DeployHandlers) HandleCancelJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID: /api/agent-deploy/jobs/{id}/cancel
	path := strings.TrimPrefix(r.URL.Path, "/api/agent-deploy/jobs/")
	jobID := strings.TrimSuffix(path, "/cancel")
	if jobID == "" || jobID == path {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Job ID is required", nil)
		return
	}

	ctx := r.Context()
	job, err := h.store.GetJob(ctx, jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get deploy job for cancel")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get job", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	if job.Status != deploy.JobRunning {
		writeErrorResponse(w, http.StatusConflict, "not_running",
			fmt.Sprintf("Job is in state %q, only running jobs can be canceled", job.Status), nil)
		return
	}

	// Transition to canceling.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobCanceling)

	// Send cancel to source agent.
	cancelPayload := agentexec.DeployCancelPayload{
		RequestID: generateID("req"),
		JobID:     jobID,
	}
	if err := h.execServer.SendDeployCancel(ctx, job.SourceAgentID, cancelPayload); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to send cancel command")
		// Don't fail the request — the agent may have already disconnected.
		// processInstallProgress will handle the channel close.
	}

	// Append cancel event.
	_ = h.store.AppendEvent(ctx, &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventJobStatusChanged,
		Message:   "Cancel requested",
		CreatedAt: time.Now().UTC(),
	})

	// Return current state.
	job.Status = deploy.JobCanceling
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

type retryJobRequest struct {
	TargetIDs []string `json:"targetIds,omitempty"`
}

// HandleRetryJob retries failed targets in a terminal deploy job.
// POST /api/agent-deploy/jobs/{jobId}/retry
func (h *DeployHandlers) HandleRetryJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID: /api/agent-deploy/jobs/{id}/retry
	path := strings.TrimPrefix(r.URL.Path, "/api/agent-deploy/jobs/")
	jobID := strings.TrimSuffix(path, "/retry")
	if jobID == "" || jobID == path {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Job ID is required", nil)
		return
	}

	var req retryJobRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			// EOF means empty body (e.g. chunked with no data) — treat as "retry all".
			if !errors.Is(err, io.EOF) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
				return
			}
		}
	}

	ctx := r.Context()
	job, err := h.store.GetJob(ctx, jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get deploy job for retry")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get job", nil)
		return
	}
	if job == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	orgID := resolveTenantOrgID(r)
	if job.OrgID != orgID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Job not found", nil)
		return
	}

	if !isDeployJobTerminal(job.Status) {
		writeErrorResponse(w, http.StatusConflict, "not_terminal",
			fmt.Sprintf("Job is in state %q, only terminal jobs can be retried", job.Status), nil)
		return
	}

	// Verify source agent is connected.
	if !h.execServer.IsAgentConnected(job.SourceAgentID) {
		writeErrorResponse(w, http.StatusConflict, "source_agent_offline", "Source agent is not connected", nil)
		return
	}

	// Load targets.
	targets, err := h.store.GetTargetsForJob(ctx, jobID)
	if err != nil {
		log.Error().Err(err).Str("id", jobID).Msg("Failed to get job targets for retry")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to get targets", nil)
		return
	}

	// Filter to retryable failed targets.
	requestedIDs := make(map[string]bool, len(req.TargetIDs))
	for _, id := range req.TargetIDs {
		requestedIDs[strings.TrimSpace(id)] = true
	}

	var retryTargets []deploy.Target
	for _, t := range targets {
		if t.Status != deploy.TargetFailedRetryable && t.Status != deploy.TargetFailedPermanent {
			continue
		}
		if t.Attempts >= job.RetryMax {
			continue
		}
		if len(requestedIDs) > 0 && !requestedIDs[t.ID] {
			continue
		}
		retryTargets = append(retryTargets, t)
	}

	if len(retryTargets) == 0 {
		writeErrorResponse(w, http.StatusConflict, "nothing_to_retry",
			"No eligible targets to retry (all succeeded or exceeded max attempts)", nil)
		return
	}

	// License slot re-check.
	maxLimit := maxAgentsLimitForContext(ctx)
	if maxLimit > 0 {
		currentCount := agentCount(h.monitor)
		reservedCount := h.reservation.ReservedForOrg(orgID)
		available := maxLimit - currentCount - reservedCount
		if available < 0 {
			available = 0
		}
		if available < len(retryTargets) {
			retryTargets = retryTargets[:available]
		}
		if len(retryTargets) == 0 {
			writeErrorResponse(w, http.StatusConflict, "license_limit",
				"No license slots available for retry", nil)
			return
		}
	}

	// Reset targets to pending.
	retryIDs := make([]string, len(retryTargets))
	for i, t := range retryTargets {
		retryIDs[i] = t.ID
	}
	resetCount, err := h.store.ResetTargetsForRetry(ctx, retryIDs)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to reset targets for retry")
		writeErrorResponse(w, http.StatusInternalServerError, "store_error", "Failed to reset targets", nil)
		return
	}
	if resetCount == 0 {
		writeErrorResponse(w, http.StatusConflict, "nothing_to_retry",
			"No targets were in a retryable state", nil)
		return
	}

	// Transition job back to running.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobRunning)

	// Resolve Pulse URL.
	pulseURL := h.resolvePublicURL(r)
	if pulseURL == "" {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		// Mark reset targets back to failed_retryable so they remain retryable.
		for _, id := range retryIDs {
			_ = h.store.UpdateTargetStatus(ctx, id, deploy.TargetFailedRetryable, "dispatch failed: no Pulse URL")
		}
		h.reservation.Release(jobID + "-retry")
		writeErrorResponse(w, http.StatusInternalServerError, "no_pulse_url",
			"Cannot determine Pulse URL for agent installation", nil)
		return
	}

	// Mint fresh bootstrap tokens and build install targets.
	var installTargets []agentexec.DeployInstallTarget
	for _, t := range retryTargets {
		rawToken, _, err := h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
			ClusterID:     job.ClusterID,
			NodeID:        t.NodeID,
			ExpectedNode:  t.NodeName,
			JobID:         jobID,
			TargetID:      t.ID,
			SourceAgentID: job.SourceAgentID,
			OrgID:         orgID,
			TTL:           30 * time.Minute,
		})
		if err != nil {
			log.Error().Err(err).Str("target_id", t.ID).Msg("Failed to mint retry bootstrap token")
			_ = h.store.UpdateTargetStatus(ctx, t.ID, deploy.TargetFailedPermanent, "failed to mint bootstrap token for retry")
			continue
		}

		arch := t.Arch
		if arch == "" {
			arch = "amd64"
		}
		installTargets = append(installTargets, agentexec.DeployInstallTarget{
			TargetID:       t.ID,
			NodeName:       t.NodeName,
			NodeIP:         t.NodeIP,
			Arch:           arch,
			BootstrapToken: rawToken,
		})
	}

	if len(installTargets) == 0 {
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		writeErrorResponse(w, http.StatusInternalServerError, "retry_setup_failed",
			"Failed to set up any retry targets", nil)
		return
	}

	// Reserve license slots based on actual dispatch count (after token minting).
	if err := h.reservation.Reserve(jobID+"-retry", orgID, len(installTargets), 1*time.Hour); err != nil {
		log.Warn().Err(err).Str("job_id", jobID).Msg("Failed to reserve license slots for retry")
	}

	// Append retry event.
	_ = h.store.AppendEvent(ctx, &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventJobStatusChanged,
		Message:   fmt.Sprintf("Retry started for %d targets", len(installTargets)),
		CreatedAt: time.Now().UTC(),
	})

	requestID := generateID("req")
	payload := agentexec.DeployInstallPayload{
		RequestID:   requestID,
		JobID:       jobID,
		Targets:     installTargets,
		PulseURL:    pulseURL,
		MaxParallel: job.MaxParallel,
		Timeout:     300,
	}

	// Subscribe and send.
	progressCh := h.execServer.SubscribeDeployProgress(job.SourceAgentID, jobID, 64)
	if err := h.execServer.SendDeployInstall(ctx, job.SourceAgentID, payload); err != nil {
		h.execServer.UnsubscribeDeployProgress(job.SourceAgentID, jobID)
		_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
		h.reservation.Release(jobID + "-retry")
		// Mark retried targets back to failed so they can be retried again.
		for _, it := range installTargets {
			_ = h.store.UpdateTargetStatus(ctx, it.TargetID, deploy.TargetFailedRetryable, "dispatch failed")
		}
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to send retry install command")
		writeErrorResponse(w, http.StatusInternalServerError, "send_failed",
			"Failed to send retry command to agent", nil)
		return
	}

	go h.processInstallProgress(jobID, job.SourceAgentID, job.RetryMax, progressCh)

	resp := map[string]any{
		"jobId":        jobID,
		"retryTargets": len(installTargets),
		"status":       "running",
		"eventsUrl":    fmt.Sprintf("/api/agent-deploy/jobs/%s/events", jobID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// processInstallProgress reads install progress events from the agent and
// persists them as deploy events, also broadcasting to SSE clients.
func (h *DeployHandlers) processInstallProgress(jobID, agentID string, retryMax int, ch <-chan agentexec.DeployProgressPayload) {
	defer h.execServer.UnsubscribeDeployProgress(agentID, jobID)

	ctx := context.Background()

	for progress := range ch {
		// Persist as event.
		evt := &deploy.Event{
			ID:        generateID("evt"),
			JobID:     jobID,
			TargetID:  progress.TargetID,
			Type:      deploy.EventInstallOutput,
			Message:   progress.Message,
			Data:      progress.Data,
			CreatedAt: time.Now().UTC(),
		}
		if err := h.store.AppendEvent(ctx, evt); err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to persist install event")
		}

		// Update target status based on install phase.
		if progress.TargetID != "" {
			h.updateTargetFromInstallProgress(ctx, progress, retryMax)
		}

		// Broadcast to SSE clients.
		h.broadcastSSE(jobID, evt)

		if progress.Final {
			// Derive final job status from target statuses.
			// Use install-specific derivation that treats failed_retryable as terminal.
			targets, err := h.store.GetTargetsForJob(ctx, jobID)
			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get targets for job status derivation")
				_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
			} else {
				finalStatus := deriveInstallJobStatus(targets)
				_ = h.store.UpdateJobStatus(ctx, jobID, finalStatus)

				// Broadcast final status.
				finalEvt := &deploy.Event{
					ID:        generateID("evt"),
					JobID:     jobID,
					Type:      deploy.EventJobStatusChanged,
					Message:   fmt.Sprintf("Deploy completed: %s", finalStatus),
					CreatedAt: time.Now().UTC(),
				}
				_ = h.store.AppendEvent(ctx, finalEvt)
				h.broadcastSSE(jobID, finalEvt)
			}

			// Release license reservation.
			h.reservation.Release(jobID)
			h.reservation.Release(jobID + "-retry") // in case of retry

			// Close SSE channels.
			h.closeSSESub(jobID)
			return
		}
	}

	// Channel closed without final — agent disconnected.
	_ = h.store.UpdateJobStatus(ctx, jobID, deploy.JobFailed)
	h.reservation.Release(jobID)
	h.reservation.Release(jobID + "-retry")

	finalEvt := &deploy.Event{
		ID:        generateID("evt"),
		JobID:     jobID,
		Type:      deploy.EventError,
		Message:   "Source agent disconnected during install",
		CreatedAt: time.Now().UTC(),
	}
	_ = h.store.AppendEvent(ctx, finalEvt)
	h.broadcastSSE(jobID, finalEvt)
	h.closeSSESub(jobID)
}

// updateTargetFromInstallProgress maps install progress phases to target status transitions.
func (h *DeployHandlers) updateTargetFromInstallProgress(ctx context.Context, p agentexec.DeployProgressPayload, retryMax int) {
	var newStatus deploy.TargetStatus
	var errMsg string

	switch {
	case p.Phase == agentexec.DeployPhaseInstallTransfer && p.Status == agentexec.DeployStepStarted:
		newStatus = deploy.TargetInstalling
	case p.Phase == agentexec.DeployPhaseInstallExecute && p.Status == agentexec.DeployStepFailed:
		// Check attempt count to decide retryable vs permanent.
		target, err := h.store.GetTarget(ctx, p.TargetID)
		if err != nil || target == nil {
			newStatus = deploy.TargetFailedRetryable
		} else if target.Attempts+1 >= retryMax {
			newStatus = deploy.TargetFailedPermanent
		} else {
			newStatus = deploy.TargetFailedRetryable
		}
		errMsg = p.Message
		// Increment attempts.
		_ = h.store.IncrementTargetAttempts(ctx, p.TargetID)
	case p.Phase == agentexec.DeployPhaseInstallEnrollWait && p.Status == agentexec.DeployStepStarted:
		newStatus = deploy.TargetEnrolling
	case p.Phase == agentexec.DeployPhaseInstallEnrollWait && p.Status == agentexec.DeployStepFailed:
		newStatus = deploy.TargetFailedRetryable
		errMsg = p.Message
		_ = h.store.IncrementTargetAttempts(ctx, p.TargetID)
	case p.Phase == agentexec.DeployPhaseInstallComplete && p.Status == agentexec.DeployStepFailed:
		newStatus = deploy.TargetFailedRetryable
		errMsg = p.Message
		_ = h.store.IncrementTargetAttempts(ctx, p.TargetID)
	case p.Phase == agentexec.DeployPhaseInstallComplete && p.Status == agentexec.DeployStepOK:
		// Don't change status here — target remains in 'enrolling' until
		// the enrollment endpoint transitions it to 'succeeded'. The source
		// agent fires this event immediately without waiting for enrollment.
		return
	case p.Phase == agentexec.DeployPhaseCanceled:
		newStatus = deploy.TargetCanceled
	default:
		return // intermediate step, no status change
	}

	if err := h.store.UpdateTargetStatus(ctx, p.TargetID, newStatus, errMsg); err != nil {
		log.Error().Err(err).
			Str("target_id", p.TargetID).
			Str("new_status", string(newStatus)).
			Msg("Failed to update target status from install progress")
	}
}

// getTargetArchFromPreflight extracts the architecture from preflight events for a given node.
func (h *DeployHandlers) getTargetArchFromPreflight(ctx context.Context, preflightJobID string, nodeID string) string {
	events, err := h.store.GetEventsForJob(ctx, preflightJobID)
	if err != nil {
		return "amd64"
	}

	// Get preflight targets to map target IDs to node IDs.
	pfTargets, err := h.store.GetTargetsForJob(ctx, preflightJobID)
	if err != nil {
		return "amd64"
	}

	targetIDForNode := ""
	for _, t := range pfTargets {
		if t.NodeID == nodeID {
			targetIDForNode = t.ID
			// Also check if arch was stored on the target directly.
			if t.Arch != "" {
				return t.Arch
			}
			break
		}
	}

	if targetIDForNode == "" {
		return "amd64"
	}

	// Look through events for preflight_complete with arch data.
	for _, evt := range events {
		if evt.TargetID != targetIDForNode {
			continue
		}
		if evt.Type != deploy.EventPreflightResult || evt.Data == "" {
			continue
		}
		var result agentexec.PreflightResultData
		if err := json.Unmarshal([]byte(evt.Data), &result); err == nil && result.Arch != "" {
			return result.Arch
		}
	}

	return "amd64"
}

// --- Helpers ---

// extractClusterID extracts a cluster ID from a path like /api/clusters/{id}/agent-deploy/...
func extractClusterID(path, prefix, suffix string) string {
	path = strings.TrimPrefix(path, prefix)
	idx := strings.Index(path, suffix)
	if idx < 0 {
		idx = strings.Index(path, "/")
		if idx < 0 {
			return strings.TrimSpace(path)
		}
	}
	return strings.TrimSpace(path[:idx])
}

// extractPathSuffix extracts the part after a prefix, e.g. /api/foo/bar -> bar
func extractPathSuffix(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	// Remove trailing slashes and nested paths.
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// nodeIP extracts the hostname/IP from a node host URL (e.g. "https://10.0.0.2:8006" -> "10.0.0.2").
func nodeIP(hostURL string) string {
	raw := strings.TrimSpace(hostURL)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := parsed.Hostname() // strips port
	return host
}

func nodeName(node *unifiedresources.NodeView) string {
	if node == nil {
		return ""
	}
	if name := strings.TrimSpace(node.NodeName()); name != "" {
		return name
	}
	return strings.TrimSpace(node.Name())
}

// generateID creates a prefixed unique ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// isDeployJobTerminal returns true if the job status is terminal.
func isDeployJobTerminal(s deploy.JobStatus) bool {
	switch s {
	case deploy.JobSucceeded, deploy.JobPartialSuccess, deploy.JobFailed, deploy.JobCanceled:
		return true
	}
	return false
}

// deriveInstallJobStatus computes the final install job status from target statuses.
// Unlike DeriveStatus, this treats TargetFailedRetryable as terminal (the agent
// has finished its work and signaled Final=true, so all targets are settled).
// TargetEnrolling is treated as succeeded because the source agent fires
// install_complete/ok immediately without waiting for async enrollment.
func deriveInstallJobStatus(targets []deploy.Target) deploy.JobStatus {
	if len(targets) == 0 {
		return deploy.JobSucceeded
	}

	var succeeded, failed int
	for _, t := range targets {
		switch t.Status {
		case deploy.TargetSucceeded, deploy.TargetVerifying, deploy.TargetEnrolling:
			// enrolling = install completed, enrollment is async and expected to succeed
			succeeded++
		case deploy.TargetFailedPermanent, deploy.TargetFailedRetryable,
			deploy.TargetSkippedAgent, deploy.TargetSkippedLicense, deploy.TargetCanceled:
			failed++
		// pending/installing — shouldn't happen at Final but treat as incomplete
		default:
			failed++
		}
	}

	total := len(targets)
	if succeeded == total {
		return deploy.JobSucceeded
	}
	if succeeded > 0 {
		return deploy.JobPartialSuccess
	}
	return deploy.JobFailed
}

// derivePreflightJobStatus computes the final job status from target statuses.
// Unlike DeriveStatus, this treats TargetReady as "passed" (preflight success)
// and TargetSkippedAgent as neutral success (not a failure).
func derivePreflightJobStatus(ctx context.Context, store *deploy.Store, jobID string) deploy.JobStatus {
	targets, err := store.GetTargetsForJob(ctx, jobID)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get targets for job status derivation")
		return deploy.JobFailed
	}
	if len(targets) == 0 {
		return deploy.JobSucceeded
	}

	var succeeded, failed int
	for _, t := range targets {
		switch t.Status {
		case deploy.TargetReady, deploy.TargetSucceeded, deploy.TargetSkippedAgent:
			succeeded++
		case deploy.TargetFailedPermanent, deploy.TargetFailedRetryable:
			failed++
		}
	}

	total := len(targets)
	if succeeded == total {
		return deploy.JobSucceeded
	}
	if failed == total {
		return deploy.JobFailed
	}
	if succeeded > 0 {
		return deploy.JobPartialSuccess
	}
	return deploy.JobFailed
}
