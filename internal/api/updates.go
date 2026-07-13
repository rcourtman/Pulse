package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockmode"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

const applyUpdateStartAckTimeout = 250 * time.Millisecond

// UpdateHandlers handles update-related API requests
type UpdateHandlers struct {
	manager           UpdateManager
	history           *updates.UpdateHistory
	registry          *updates.UpdaterRegistry
	statusRateLimits  map[string]time.Time // IP -> last request time
	statusMu          sync.RWMutex
	getConfig         func(context.Context) *config.Config
	getHostsSnapshot  func(context.Context) []models.Host
	getCurrentVersion func() (*updates.VersionInfo, error)
	now               func() time.Time
}

// UpdateManager defines the interface for update management operations
type UpdateManager interface {
	CheckForUpdatesWithChannel(ctx context.Context, channel string) (*updates.UpdateInfo, error)
	ApplyUpdate(ctx context.Context, req updates.ApplyUpdateRequest) error
	RollbackToBackup(ctx context.Context, req updates.RollbackRequest) error
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
	registry.Register("systemd", updates.NewInstallShAdapter())
	registry.Register("proxmoxve", updates.NewInstallShAdapter())
	registry.Register("docker", updates.NewDockerUpdater())
	registry.Register("aur", updates.NewAURUpdater())
	if mockmode.IsEnabled() || strings.EqualFold(os.Getenv("PULSE_ALLOW_DOCKER_UPDATES"), "true") {
		registry.Register("mock", updates.NewMockUpdater())
	}

	h := &UpdateHandlers{
		manager:           manager,
		history:           history,
		registry:          registry,
		statusRateLimits:  make(map[string]time.Time),
		getCurrentVersion: updates.GetCurrentVersion,
		now:               time.Now,
	}

	// Start periodic cleanup of rate limit map
	go h.cleanupRateLimits(cleanupCtx)

	return h
}

// SetUpdateReadinessSources wires runtime state used to attach an upgrade
// readiness verdict to update plans. Nil callbacks leave plans unchanged.
func (h *UpdateHandlers) SetUpdateReadinessSources(
	getConfig func(context.Context) *config.Config,
	getHostsSnapshot func(context.Context) []models.Host,
) {
	if h == nil {
		return
	}
	h.getConfig = getConfig
	h.getHostsSnapshot = getHostsSnapshot
}

// HandleCheckUpdates handles update check requests
func (h *UpdateHandlers) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get channel from query parameter if provided
	channel, err := normalizeRequestedUpdateChannel(r.URL.Query().Get("channel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

// releaseNotesProvider is implemented by update managers that can fetch
// release notes for a specific published version.
type releaseNotesProvider interface {
	GetReleaseNotes(ctx context.Context, version string) (*updates.ReleaseNotesInfo, error)
}

// HandleGetReleaseNotes returns the GitHub release notes for the currently
// running version. Unlike the other update endpoints it is exposed to all
// authenticated users so the UI can show a post-update "What's New" card.
func (h *UpdateHandlers) HandleGetReleaseNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider, ok := h.manager.(releaseNotesProvider)
	if !ok {
		http.Error(w, "Release notes not available", http.StatusNotFound)
		return
	}

	versionInfo, err := h.getCurrentVersion()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get version info for release notes")
		http.Error(w, "Failed to get version info", http.StatusInternalServerError)
		return
	}
	if versionInfo.IsDevelopment || versionInfo.IsSourceBuild {
		http.Error(w, "Release notes not available for this build", http.StatusNotFound)
		return
	}

	notes, err := provider.GetReleaseNotes(r.Context(), versionInfo.Version)
	if err != nil {
		if errors.Is(err, updates.ErrReleaseNotFound) {
			http.Error(w, "Release not found", http.StatusNotFound)
			return
		}
		log.Warn().Err(err).Str("version", versionInfo.Version).Msg("Failed to fetch release notes")
		http.Error(w, "Failed to fetch release notes", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notes); err != nil {
		log.Error().Err(err).Msg("Failed to encode release notes")
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
	defer r.Body.Close()

	var req struct {
		DownloadURL string `json:"downloadUrl"`
		// AllowDowngrade opts in to installing a target at or below the
		// running version; without it the manager rejects downgrades.
		AllowDowngrade bool `json:"allowDowngrade"`
	}

	if err := decodeStrictJSONBody(r.Body, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.DownloadURL = strings.TrimSpace(req.DownloadURL)

	if req.DownloadURL == "" {
		http.Error(w, "Download URL is required", http.StatusBadRequest)
		return
	}

	channel, err := normalizeRequestedUpdateChannel(r.URL.Query().Get("channel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.hasUpdateReadinessSources() {
		targetVersion, err := updates.ValidateApplyTargetVersion(channel, req.DownloadURL)
		if err != nil {
			statusCode, msg := classifyApplyUpdateStartError(err)
			log.Warn().Err(err).Str("download_url", req.DownloadURL).Str("channel", channel).Msg("Update request rejected")
			http.Error(w, msg, statusCode)
			return
		}

		plan, statusCode, msg, err := h.prepareUpdatePlan(r.Context(), targetVersion, channel)
		if err != nil {
			if statusCode >= http.StatusInternalServerError {
				log.Error().Err(err).Str("version", targetVersion).Str("channel", channel).Msg("Failed to prepare update readiness before apply")
			} else {
				log.Warn().Err(err).Str("version", targetVersion).Str("channel", channel).Msg("Update readiness request rejected")
			}
			http.Error(w, msg, statusCode)
			return
		}
		if plan.Readiness != nil && plan.Readiness.Status == updateReadinessBlocked {
			log.Warn().
				Str("version", targetVersion).
				Str("channel", channel).
				Str("summary", plan.Readiness.Summary).
				Msg("Update request blocked by readiness checks")
			http.Error(w, plan.Readiness.Summary, http.StatusConflict)
			return
		}
	}

	applyReq := updates.ApplyUpdateRequest{
		DownloadURL:    req.DownloadURL,
		Channel:        channel,
		InitiatedBy:    updates.InitiatedByUser,
		InitiatedVia:   updates.InitiatedViaUI,
		AllowDowngrade: req.AllowDowngrade,
	}
	result := make(chan error, 1)

	// Start update in background with a new context (not request context which gets canceled)
	go func() {
		result <- h.manager.ApplyUpdate(context.Background(), applyReq)
	}()

	select {
	case err := <-result:
		if err != nil {
			statusCode, msg := classifyApplyUpdateStartError(err)
			if statusCode >= http.StatusInternalServerError {
				log.Error().Err(err).Str("download_url", req.DownloadURL).Str("channel", channel).Msg("Failed to start update")
			} else {
				log.Warn().Err(err).Str("download_url", req.DownloadURL).Str("channel", channel).Msg("Update request rejected")
			}
			http.Error(w, msg, statusCode)
			return
		}
	case <-time.After(applyUpdateStartAckTimeout):
	}

	// Return success immediately
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Update process started",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode update start response")
	}
}

func decodeStrictJSONBody(body io.Reader, dst any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request body must contain a single JSON object")
	}

	return nil
}

func normalizeRequestedUpdateChannel(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	canonical, ok := config.CanonicalUpdateChannel(trimmed)
	if !ok {
		return "", fmt.Errorf("update channel must be 'stable' or 'rc'")
	}
	return canonical, nil
}

func classifyApplyUpdateStartError(err error) (int, string) {
	errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(errMsg, "already in progress"):
		return http.StatusConflict, "Update already in progress"
	case strings.Contains(errMsg, "download url is required"), strings.Contains(errMsg, "invalid download url"):
		return http.StatusBadRequest, "Invalid download URL"
	case strings.Contains(errMsg, "stable channel cannot install prerelease builds"),
		strings.Contains(errMsg, "not newer than the running version"):
		return http.StatusConflict, err.Error()
	case strings.Contains(errMsg, "cannot be applied in docker environment"),
		strings.Contains(errMsg, "manual migration required"),
		strings.Contains(errMsg, "pulse pro updates need an activated license"):
		return http.StatusConflict, err.Error()
	default:
		return http.StatusInternalServerError, "Failed to start update"
	}
}

// HandleRollbackUpdate handles rollback requests: it restores the retained
// backup recorded on the selected update history entry.
func (h *UpdateHandlers) HandleRollbackUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var req struct {
		EventID string `json:"eventId"`
	}

	if err := decodeStrictJSONBody(r.Body, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.EventID = strings.TrimSpace(req.EventID)

	if req.EventID == "" {
		http.Error(w, "Event ID is required", http.StatusBadRequest)
		return
	}

	rollbackReq := updates.RollbackRequest{
		EventID:      req.EventID,
		InitiatedBy:  updates.InitiatedByUser,
		InitiatedVia: updates.InitiatedViaUI,
	}
	result := make(chan error, 1)

	// Start rollback in background with a new context (not request context which gets canceled)
	go func() {
		result <- h.manager.RollbackToBackup(context.Background(), rollbackReq)
	}()

	select {
	case err := <-result:
		if err != nil {
			statusCode, msg := classifyRollbackStartError(err)
			if statusCode >= http.StatusInternalServerError {
				log.Error().Err(err).Str("event_id", req.EventID).Msg("Failed to start rollback")
			} else {
				log.Warn().Err(err).Str("event_id", req.EventID).Msg("Rollback request rejected")
			}
			http.Error(w, msg, statusCode)
			return
		}
	case <-time.After(applyUpdateStartAckTimeout):
	}

	// Return success immediately
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Rollback process started",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode rollback start response")
	}
}

func classifyRollbackStartError(err error) (int, string) {
	errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(errMsg, "already in progress"):
		return http.StatusConflict, "Update already in progress"
	case strings.Contains(errMsg, "event id is required"):
		return http.StatusBadRequest, "Event ID is required"
	case strings.Contains(errMsg, "history entry not found"):
		return http.StatusNotFound, "Update history entry not found"
	case strings.Contains(errMsg, "no retained backup"),
		strings.Contains(errMsg, "backup no longer exists"),
		strings.Contains(errMsg, "not a managed update backup"),
		strings.Contains(errMsg, "cannot be applied in docker environment"):
		return http.StatusConflict, err.Error()
	default:
		return http.StatusInternalServerError, "Failed to start rollback"
	}
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

	// Get version from query
	version := r.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "version parameter required", http.StatusBadRequest)
		return
	}
	channel, err := normalizeRequestedUpdateChannel(r.URL.Query().Get("channel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan, statusCode, msg, err := h.prepareUpdatePlan(r.Context(), version, channel)
	if err != nil {
		if statusCode >= http.StatusInternalServerError {
			log.Error().Err(err).Str("version", version).Str("channel", channel).Msg("Failed to prepare update plan")
		}
		http.Error(w, msg, statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

func (h *UpdateHandlers) prepareUpdatePlan(ctx context.Context, version string, channel string) (updates.UpdatePlan, int, string, error) {
	versionInfo, err := updates.GetCurrentVersion()
	if err != nil {
		return updates.UpdatePlan{}, http.StatusInternalServerError, "Failed to get version info", err
	}

	updater, err := h.registry.Get(versionInfo.DeploymentType)
	if err != nil {
		if plan, ok := fallbackManualUpdatePlan(versionInfo.DeploymentType, version); ok {
			normalizedPlan := h.attachUpdateReadiness(ctx, version, plan.NormalizeCollections())
			return normalizedPlan, 0, "", nil
		}

		return updates.UpdatePlan{}, http.StatusNotFound, "No updater for deployment type", err
	}

	plan, err := updater.PrepareUpdate(ctx, updates.UpdateRequest{
		Version: version,
		Channel: channel,
	})
	if err != nil {
		return updates.UpdatePlan{}, http.StatusInternalServerError, "Failed to prepare update plan", err
	}

	normalizedPlan := h.attachUpdateReadiness(ctx, version, plan.NormalizeCollections())
	return normalizedPlan, 0, "", nil
}

func (h *UpdateHandlers) attachUpdateReadiness(ctx context.Context, version string, plan updates.UpdatePlan) updates.UpdatePlan {
	if h == nil || h.getConfig == nil || h.getHostsSnapshot == nil {
		return plan
	}
	now := time.Now()
	if h.now != nil {
		now = h.now()
	}
	plan.Readiness = buildUpdateReadiness(updateReadinessInputs{
		cfg:           h.getConfig(ctx),
		hosts:         h.getHostsSnapshot(ctx),
		targetVersion: version,
		plan:          plan,
		now:           now,
	})
	return plan.NormalizeCollections()
}

func (h *UpdateHandlers) hasUpdateReadinessSources() bool {
	return h != nil && h.getConfig != nil && h.getHostsSnapshot != nil
}

func fallbackManualUpdatePlan(deploymentType string, version string) (*updates.UpdatePlan, bool) {
	normalized := strings.ToLower(strings.TrimSpace(deploymentType))

	switch normalized {
	case "manual":
		return &updates.UpdatePlan{
			CanAutoUpdate:   false,
			RequiresRoot:    true,
			RollbackSupport: true,
			EstimatedTime:   "5-10 minutes",
			Instructions: []string{
				fmt.Sprintf("Download Pulse %s from the release assets for your platform.", version),
				"Stop the running Pulse service or process.",
				"Back up the current Pulse binary and data directory before replacing the binary.",
				"Install the new binary, then start Pulse again.",
			},
			Prerequisites: []string{
				"Shell access to the host running Pulse",
				"Permission to replace the current Pulse binary",
				"A backup path for rollback if the upgrade must be reverted",
			},
		}, true
	case "development":
		return &updates.UpdatePlan{
			CanAutoUpdate:   false,
			RequiresRoot:    false,
			RollbackSupport: true,
			EstimatedTime:   "5-10 minutes",
			Instructions: []string{
				fmt.Sprintf("Check out or build Pulse %s in your development workspace.", version),
				"Stop the current development instance.",
				"Restart Pulse with the rebuilt binary or release artifact against the existing data directory.",
			},
			Prerequisites: []string{
				"A local development workspace for Pulse",
				"Build tooling for the target version",
				"A backup of the active data directory before replacing the binary",
			},
		}, true
	case "source":
		return &updates.UpdatePlan{
			CanAutoUpdate:   false,
			RequiresRoot:    false,
			RollbackSupport: true,
			EstimatedTime:   "5-15 minutes",
			Instructions: []string{
				fmt.Sprintf("Pull or check out the source for Pulse %s.", version),
				"Build a new Pulse binary from source.",
				"Stop the current instance, replace the binary, and restart Pulse against the existing data directory.",
			},
			Prerequisites: []string{
				"A clean source checkout of Pulse",
				"Go and frontend build tooling required for your environment",
				"A backup of the current binary and data directory for rollback",
			},
		}, true
	default:
		return nil, false
	}
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
