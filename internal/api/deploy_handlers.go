package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
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

	snapshot := h.monitor.GetState()

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

	for _, node := range snapshot.Nodes {
		if !node.IsClusterMember {
			continue
		}
		// Match cluster by name (clusterID in URL = cluster name).
		if node.ClusterName != clusterID {
			continue
		}
		if clusterName == "" {
			clusterName = node.ClusterName
		}

		hasAgent := node.LinkedHostAgentID != ""
		cn := candidateNode{
			NodeID:   node.ID,
			Name:     node.Name,
			IP:       nodeIP(node),
			HasAgent: hasAgent,
		}

		if hasAgent {
			cn.Deployable = false
			cn.Reason = "already_agent"

			// This node has an agent — check if it's a source candidate.
			hostID := node.LinkedHostAgentID
			if connectedAgents[hostID] {
				sourceAgents = append(sourceAgents, sourceAgentInfo{
					AgentID: hostID,
					NodeID:  node.ID,
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

	// Resolve cluster nodes from state.
	snapshot := h.monitor.GetState()

	clusterName := ""
	sourceNodeID := ""
	nodesByID := make(map[string]models.Node)
	for _, node := range snapshot.Nodes {
		if node.ClusterName == clusterID && node.IsClusterMember {
			nodesByID[node.ID] = node
			if clusterName == "" {
				clusterName = node.ClusterName
			}
			if node.LinkedHostAgentID == req.SourceAgentID {
				sourceNodeID = node.ID
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
		ip := nodeIP(node)
		if ip == "" {
			continue // skip nodes without IP
		}

		targetID := generateID("tgt")
		target := &deploy.Target{
			ID:        targetID,
			JobID:     jobID,
			NodeID:    nodeID,
			NodeName:  node.Name,
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
			NodeName: node.Name,
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
		[]string{config.ScopeHostEnroll})
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
	Hostname     string `json:"hostname"`
	FQDN         string `json:"fqdn,omitempty"`
	MachineID    string `json:"machineId,omitempty"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agentVersion"`
	Proxmox      *struct {
		ClusterName string `json:"clusterName,omitempty"`
		NodeName    string `json:"nodeName,omitempty"`
	} `json:"proxmox,omitempty"`
	DeployJobID string `json:"deployJobId,omitempty"`
}

// HandleEnroll processes bootstrap token enrollment from freshly-deployed agents.
// POST /api/agents/host/enroll
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
	runtimeRecord, err := config.NewAPITokenRecord(runtimeRaw,
		fmt.Sprintf("host-agent:%s", req.Hostname),
		[]string{config.ScopeHostReport, config.ScopeHostConfigRead})
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
	resp := map[string]any{
		"hostId":         fmt.Sprintf("host-%s", req.Hostname),
		"runtimeToken":   runtimeRaw,
		"runtimeTokenId": runtimeRecord.ID,
		"reportInterval": "30s",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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

// nodeIP extracts the hostname/IP from a Node's Host URL (e.g. "https://10.0.0.2:8006" → "10.0.0.2").
func nodeIP(node models.Node) string {
	raw := strings.TrimSpace(node.Host)
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
