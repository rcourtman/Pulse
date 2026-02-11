package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

// UpdateHandlers handles update-related API requests
type UpdateHandlers struct {
	manager          UpdateManager
	history          *updates.UpdateHistory
	registry         *updates.UpdaterRegistry
	statusRateLimits map[string]time.Time // IP -> last request time
	statusMu         sync.RWMutex
}

// UpdateManager defines the interface for update management operations
type UpdateManager interface {
	CheckForUpdatesWithChannel(ctx context.Context, channel string) (*updates.UpdateInfo, error)
	ApplyUpdate(ctx context.Context, req updates.ApplyUpdateRequest) error
	GetStatus() updates.UpdateStatus
	GetSSECachedStatus() (updates.UpdateStatus, time.Time)
	AddSSEClient(w http.ResponseWriter, clientID string) *updates.SSEClient
	RemoveSSEClient(clientID string)
}

// NewUpdateHandlers creates new update handlers
func NewUpdateHandlers(manager UpdateManager, history *updates.UpdateHistory) *UpdateHandlers {
	return NewUpdateHandlersWithContext(manager, history, context.Background())
}

// NewUpdateHandlersWithContext creates update handlers with a stoppable cleanup loop.
func NewUpdateHandlersWithContext(manager UpdateManager, history *updates.UpdateHistory, cleanupCtx context.Context) *UpdateHandlers {
	if cleanupCtx == nil {
		cleanupCtx = context.Background()
	}

	// Initialize updater registry
	registry := updates.NewUpdaterRegistry()

	// Register adapters
	registry.Register("systemd", updates.NewInstallShAdapter(history))
	registry.Register("proxmoxve", updates.NewInstallShAdapter(history))
	registry.Register("docker", updates.NewDockerUpdater())
	registry.Register("aur", updates.NewAURUpdater())
	if strings.EqualFold(os.Getenv("PULSE_MOCK_MODE"), "true") || strings.EqualFold(os.Getenv("PULSE_ALLOW_DOCKER_UPDATES"), "true") {
		registry.Register("mock", updates.NewMockUpdater())
	}

	h := &UpdateHandlers{
		manager:          manager,
		history:          history,
		registry:         registry,
		statusRateLimits: make(map[string]time.Time),
	}

	// Start periodic cleanup of rate limit map
	go h.cleanupRateLimits(cleanupCtx)

	return h
}

// HandleCheckUpdates handles update check requests
func (h *UpdateHandlers) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get channel from query parameter if provided
	channel := r.URL.Query().Get("channel")

	info, err := h.manager.CheckForUpdatesWithChannel(ctx, channel)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check for updates")
		http.Error(w, "Failed to check for updates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Error().Err(err).Msg("Failed to encode update info")
	}
}

// HandleApplyUpdate handles update application requests
func (h *UpdateHandlers) HandleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var req struct {
		DownloadURL string `json:"downloadUrl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DownloadURL == "" {
		http.Error(w, "Download URL is required", http.StatusBadRequest)
		return
	}

	// Start update in background with a new context (not request context which gets cancelled)
	go func() {
		ctx := context.Background()
		applyReq := updates.ApplyUpdateRequest{
			DownloadURL:  req.DownloadURL,
			Channel:      r.URL.Query().Get("channel"),
			InitiatedBy:  updates.InitiatedByUser,
			InitiatedVia: updates.InitiatedViaUI,
		}
		if err := h.manager.ApplyUpdate(ctx, applyReq); err != nil {
			log.Error().Err(err).Msg("Failed to apply update")
		}
	}()

	// Return success immediately
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Update process started",
	})
}

// HandleUpdateStatus handles update status requests with rate limiting
func (h *UpdateHandlers) HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract client IP for rate limiting
	clientIP := GetClientIP(r)

	// Check rate limit (5 seconds minimum between requests per client)
	h.statusMu.Lock()
	lastRequest, exists := h.statusRateLimits[clientIP]
	now := time.Now()

	if exists && now.Sub(lastRequest) < 5*time.Second {
		// Rate limited - return cached status
		h.statusMu.Unlock()

		// Get cached status from SSE broadcaster (more recent than manager status)
		cachedStatus, cacheTime := h.manager.GetSSECachedStatus()

		// Add cache headers
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("X-Cache-Time", cacheTime.Format(time.RFC3339))
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(cachedStatus); err != nil {
			log.Error().Err(err).Msg("Failed to encode cached update status")
		}

		log.Debug().
			Str("client_ip", clientIP).
			Dur("time_since_last", now.Sub(lastRequest)).
			Msg("Update status request rate limited, returning cached status")

		return
	}

	// Update last request time
	h.statusRateLimits[clientIP] = now
	h.statusMu.Unlock()

	// Get fresh status
	status := h.manager.GetStatus()

	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode update status")
	}
}

// HandleUpdateStream handles Server-Sent Events streaming of update progress
func (h *UpdateHandlers) HandleUpdateStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Generate client ID
	clientIP := GetClientIP(r)
	clientID := fmt.Sprintf("%s-%d", clientIP, time.Now().UnixNano())

	// Register client with SSE broadcaster
	client := h.manager.AddSSEClient(w, clientID)
	if client == nil {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("client_id", clientID).
		Str("client_ip", clientIP).
		Msg("Update progress SSE stream started")

	// Send initial connection message
	fmt.Fprintf(w, ": connected\n\n")
	client.Flusher.Flush()

	// Wait for client disconnect or context cancellation
	select {
	case <-r.Context().Done():
		log.Info().
			Str("client_id", clientID).
			Msg("Update progress SSE stream closed by client")
	case <-client.Done:
		log.Info().
			Str("client_id", clientID).
			Msg("Update progress SSE stream closed by server")
	}

	// Clean up
	h.manager.RemoveSSEClient(clientID)
}

// cleanupRateLimits periodically cleans up old entries from the rate limit map
func (h *UpdateHandlers) cleanupRateLimits(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.doCleanupRateLimits(time.Now())
		}
	}
}

func (h *UpdateHandlers) doCleanupRateLimits(now time.Time) {
	h.statusMu.Lock()
	defer h.statusMu.Unlock()

	// Remove entries older than 10 minutes
	for ip, lastTime := range h.statusRateLimits {
		if now.Sub(lastTime) > 10*time.Minute {
			delete(h.statusRateLimits, ip)
		}
	}
}

// HandleGetUpdatePlan returns update plan for current deployment
func (h *UpdateHandlers) HandleGetUpdatePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current version info to determine deployment type
	versionInfo, err := updates.GetCurrentVersion()
	if err != nil {
		http.Error(w, "Failed to get version info", http.StatusInternalServerError)
		return
	}

	// Get updater for deployment type
	updater, err := h.registry.Get(versionInfo.DeploymentType)
	if err != nil {
		http.Error(w, "No updater for deployment type", http.StatusNotFound)
		return
	}

	// Get version from query
	version := r.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "version parameter required", http.StatusBadRequest)
		return
	}

	// Prepare update plan
	plan, err := updater.PrepareUpdate(r.Context(), updates.UpdateRequest{
		Version: version,
		Channel: r.URL.Query().Get("channel"),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to prepare update plan")
		http.Error(w, "Failed to prepare update plan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// HandleListUpdateHistory returns update history
func (h *UpdateHandlers) HandleListUpdateHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.history == nil {
		http.Error(w, "Update history not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	filter := updates.HistoryFilter{
		Limit: 50, // Default limit
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = updates.UpdateStatusType(status)
	}

	entries := h.history.ListEntries(filter)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleGetUpdateHistoryEntry returns a specific update history entry
func (h *UpdateHandlers) HandleGetUpdateHistoryEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.history == nil {
		http.Error(w, "Update history not available", http.StatusServiceUnavailable)
		return
	}

	// Get event ID from URL path
	eventID := r.URL.Query().Get("id")
	if eventID == "" {
		http.Error(w, "event ID required", http.StatusBadRequest)
		return
	}

	entry, err := h.history.GetEntry(eventID)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}
