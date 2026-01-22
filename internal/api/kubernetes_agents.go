package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog/log"
)

// KubernetesAgentHandlers manages ingest from the Kubernetes agent.
type KubernetesAgentHandlers struct {
	mtMonitor     *monitoring.MultiTenantMonitor
	legacyMonitor *monitoring.Monitor
	wsHub         *websocket.Hub
}

// NewKubernetesAgentHandlers constructs a new Kubernetes agent handler group.
func NewKubernetesAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) *KubernetesAgentHandlers {
	// If mtm is provided, try to populate legacyMonitor from "default" org if not provided
	if m == nil && mtm != nil {
		if mon, err := mtm.GetMonitor("default"); err == nil {
			m = mon
		}
	}
	return &KubernetesAgentHandlers{mtMonitor: mtm, legacyMonitor: m, wsHub: hub}
}

// SetMonitor updates the monitor reference for kubernetes agent handlers.
func (h *KubernetesAgentHandlers) SetMonitor(m *monitoring.Monitor) {
	h.legacyMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor reference
func (h *KubernetesAgentHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.mtMonitor = mtm
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			h.legacyMonitor = m
		}
	}
}

// getMonitor helper
func (h *KubernetesAgentHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	orgID := GetOrgID(ctx)
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m
		}
	}
	return h.legacyMonitor
}

// HandleReport accepts heartbeat payloads from the Kubernetes agent.
func (h *KubernetesAgentHandlers) HandleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	// Limit request body to 2MB to prevent memory exhaustion (pods can be sizable).
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
	defer r.Body.Close()

	var report agentsk8s.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	tokenRecord := getAPITokenRecordFromRequest(r)

	cluster, err := h.getMonitor(r.Context()).ApplyKubernetesReport(report, tokenRecord)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("k8sClusterID", cluster.ID).
		Str("k8sClusterName", cluster.Name).
		Int("nodes", len(cluster.Nodes)).
		Int("pods", len(cluster.Pods)).
		Int("deployments", len(cluster.Deployments)).
		Msg("Kubernetes agent report processed")

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":     true,
		"clusterId":   cluster.ID,
		"nodes":       len(cluster.Nodes),
		"pods":        len(cluster.Pods),
		"deployments": len(cluster.Deployments),
		"lastSeen":    cluster.LastSeen,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes agent response")
	}
}

// HandleClusterActions routes kubernetes cluster management actions based on path and method.
func (h *KubernetesAgentHandlers) HandleClusterActions(w http.ResponseWriter, r *http.Request) {
	// Allow reenroll request
	if strings.HasSuffix(r.URL.Path, "/allow-reenroll") && r.Method == http.MethodPost {
		h.HandleAllowReenroll(w, r)
		return
	}

	// Unhide request
	if strings.HasSuffix(r.URL.Path, "/unhide") && r.Method == http.MethodPut {
		h.HandleUnhideCluster(w, r)
		return
	}

	// Pending uninstall request
	if strings.HasSuffix(r.URL.Path, "/pending-uninstall") && r.Method == http.MethodPut {
		h.HandleMarkPendingUninstall(w, r)
		return
	}

	// Custom display name update request
	if strings.HasSuffix(r.URL.Path, "/display-name") && r.Method == http.MethodPut {
		h.HandleSetCustomDisplayName(w, r)
		return
	}

	// Delete/hide request
	if r.Method == http.MethodDelete {
		h.HandleDeleteCluster(w, r)
		return
	}

	writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
}

// HandleDeleteCluster removes and blocks a cluster from re-enrolling.
func (h *KubernetesAgentHandlers) HandleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/kubernetes/clusters/")
	clusterID := strings.TrimSpace(trimmedPath)
	clusterID = strings.TrimSuffix(clusterID, "/")
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_cluster_id", "Kubernetes cluster ID is required", nil)
		return
	}

	cluster, err := h.getMonitor(r.Context()).RemoveKubernetesCluster(clusterID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "kubernetes_cluster_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"clusterId": cluster.ID,
		"message":   "Kubernetes cluster removed",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes cluster operation response")
	}
}

// HandleAllowReenroll clears the removal block for a cluster to permit future reports.
func (h *KubernetesAgentHandlers) HandleAllowReenroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/kubernetes/clusters/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/allow-reenroll")
	clusterID := strings.TrimSpace(trimmedPath)
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_cluster_id", "Kubernetes cluster ID is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).AllowKubernetesClusterReenroll(clusterID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "kubernetes_cluster_reenroll_failed", err.Error(), nil)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"clusterId": clusterID,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes cluster allow reenroll response")
	}
}

// HandleUnhideCluster unhides a previously hidden kubernetes cluster.
func (h *KubernetesAgentHandlers) HandleUnhideCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/kubernetes/clusters/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/unhide")
	clusterID := strings.TrimSpace(trimmedPath)
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_cluster_id", "Kubernetes cluster ID is required", nil)
		return
	}

	cluster, err := h.getMonitor(r.Context()).UnhideKubernetesCluster(clusterID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "kubernetes_cluster_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"clusterId": cluster.ID,
		"message":   "Kubernetes cluster unhidden",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes cluster unhide response")
	}
}

// HandleMarkPendingUninstall marks a cluster as pending uninstall.
func (h *KubernetesAgentHandlers) HandleMarkPendingUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/kubernetes/clusters/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/pending-uninstall")
	clusterID := strings.TrimSpace(trimmedPath)
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_cluster_id", "Kubernetes cluster ID is required", nil)
		return
	}

	cluster, err := h.getMonitor(r.Context()).MarkKubernetesClusterPendingUninstall(clusterID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "kubernetes_cluster_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"clusterId": cluster.ID,
		"message":   "Kubernetes cluster marked as pending uninstall",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes cluster pending uninstall response")
	}
}

// HandleSetCustomDisplayName updates the custom display name for a kubernetes cluster.
func (h *KubernetesAgentHandlers) HandleSetCustomDisplayName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/kubernetes/clusters/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/display-name")
	clusterID := strings.TrimSpace(trimmedPath)
	if clusterID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_cluster_id", "Kubernetes cluster ID is required", nil)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	customName := strings.TrimSpace(req.DisplayName)

	cluster, err := h.getMonitor(r.Context()).SetKubernetesClusterCustomDisplayName(clusterID, customName)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "kubernetes_cluster_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"clusterId": cluster.ID,
		"message":   "Kubernetes cluster custom display name updated",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize kubernetes cluster custom display name response")
	}
}
