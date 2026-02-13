package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// UpdateStatus represents the current status of an update
type UpdateStatus struct {
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// ReleaseInfo represents a GitHub release
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateInfo represents available update information
type UpdateInfo struct {
	Available      bool      `json:"available"`
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	ReleaseNotes   string    `json:"releaseNotes"`
	ReleaseDate    time.Time `json:"releaseDate"`
	DownloadURL    string    `json:"downloadUrl"`
	IsPrerelease   bool      `json:"isPrerelease"`
	Warning        string    `json:"warning,omitempty"`
}

var (
	errGitHubRateLimited = errors.New("GitHub API rate limit exceeded")
	stageDelayOnce       sync.Once
	stageDelayValue      time.Duration
)

// Manager handles update operations
type Manager struct {
	config         *config.Config
	history        *UpdateHistory
	status         UpdateStatus
	statusMu       sync.RWMutex
	updateMu       sync.Mutex
	updateInFlight bool
	checkCache     map[string]*UpdateInfo // keyed by channel
	cacheTime      map[string]time.Time   // keyed by channel
	cacheDuration  time.Duration
	progressChan   chan UpdateStatus
	sseBroadcast   *SSEBroadcaster
}

// ApplyUpdateRequest describes an update request initiated via the API/UI.
type ApplyUpdateRequest struct {
	DownloadURL  string
	Channel      string
	InitiatedBy  InitiatedBy
	InitiatedVia InitiatedVia
	Notes        string
}

// NewManager creates a new update manager
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		config:        cfg,
		checkCache:    make(map[string]*UpdateInfo),
		cacheTime:     make(map[string]time.Time),
		cacheDuration: 5 * time.Minute, // Cache update checks for 5 minutes
		progressChan:  make(chan UpdateStatus, 100),
		sseBroadcast:  NewSSEBroadcaster(),
		status: UpdateStatus{
			Status:    "idle",
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	// Clean up old temp directories from previous failed/killed updates
	go m.cleanupOldTempDirs()

	// Start heartbeat for SSE connections (every 30 seconds)
	go m.sseHeartbeatLoop()

	return m
}

// SetHistory wires an update history sink for recording update progress.
func (m *Manager) SetHistory(history *UpdateHistory) {
	m.history = history
}

// GetProgressChannel returns the channel for update progress
func (m *Manager) GetProgressChannel() <-chan UpdateStatus {
	return m.progressChan
}

// Close closes the progress channel and cleans up resources
func (m *Manager) Close() {
	close(m.progressChan)
	if m.sseBroadcast != nil {
		m.sseBroadcast.Close()
	}
}

// GetSSEBroadcaster returns the SSE broadcaster
func (m *Manager) GetSSEBroadcaster() *SSEBroadcaster {
	return m.sseBroadcast
}

// AddSSEClient adds a new SSE client for update progress streaming
func (m *Manager) AddSSEClient(w http.ResponseWriter, clientID string) *SSEClient {
	if m.sseBroadcast == nil {
		return nil
	}
	return m.sseBroadcast.AddClient(w, clientID)
}

// RemoveSSEClient removes an SSE client
func (m *Manager) RemoveSSEClient(clientID string) {
	if m.sseBroadcast != nil {
		m.sseBroadcast.RemoveClient(clientID)
	}
}

// GetCachedStatus returns the last broadcasted status
func (m *Manager) GetSSECachedStatus() (UpdateStatus, time.Time) {
	if m.sseBroadcast == nil {
		return UpdateStatus{}, time.Time{}
	}
	return m.sseBroadcast.GetCachedStatus()
}

// CheckForUpdates checks GitHub for available updates using saved config channel
func (m *Manager) CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	return m.CheckForUpdatesWithChannel(ctx, "")
}

// CheckForUpdatesWithChannel checks GitHub for available updates with optional channel override
func (m *Manager) CheckForUpdatesWithChannel(ctx context.Context, channel string) (*UpdateInfo, error) {
	// Get current version first to auto-detect channel if needed
	currentInfo, err := GetCurrentVersion()
	if err != nil {
		m.updateStatus("error", 0, "Failed to get current version")
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Track whether an explicit channel override was provided
	explicitChannelProvided := channel != ""

	// Use provided channel, or fall back to config, or auto-detect from current version
	if channel == "" {
		channel = m.config.UpdateChannel
	}
	if channel == "" && currentInfo.Channel != "" {
		// Auto-detect channel from current version (RC users get RC updates)
		channel = currentInfo.Channel
	}
	if channel == "" {
		channel = "stable"
	}

	// Don't use cache when channel is explicitly provided (UI might have changed it)
	// But DO use cache for auto-detected or default channels
	useCache := !explicitChannelProvided

	// Check cache first (only if using saved channel)
	if useCache {
		m.statusMu.RLock()
		cachedInfo, hasCached := m.checkCache[channel]
		cachedTime, hasTime := m.cacheTime[channel]
		if hasCached && hasTime && time.Since(cachedTime) < m.cacheDuration {
			m.statusMu.RUnlock()
			return cachedInfo, nil
		}
		m.statusMu.RUnlock()
	}

	m.updateStatus("checking", 0, "Checking for updates...")

	// Skip update check for source builds
	if currentInfo.IsSourceBuild {
		info := &UpdateInfo{
			Available:      false,
			CurrentVersion: currentInfo.Version,
			LatestVersion:  currentInfo.Version,
		}
		if useCache {
			m.statusMu.Lock()
			m.checkCache[channel] = info
			m.cacheTime[channel] = time.Now()
			m.statusMu.Unlock()
		}
		m.updateStatus("idle", 0, "Updates not available for source builds")
		return info, nil
	}

	// Parse current version first
	currentVer, err := ParseVersion(currentInfo.Version)
	if err != nil {
		m.updateStatus("error", 0, "Invalid current version")
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	// Get latest release from GitHub with specified channel and current version
	release, err := m.getLatestReleaseForChannel(ctx, channel, currentVer)
	if err != nil {
		if errors.Is(err, errGitHubRateLimited) {
			log.Warn().Err(err).Str("channel", channel).Msg("GitHub rate limit encountered while checking for updates")

			if useCache {
				m.statusMu.RLock()
				cachedInfo, hasCached := m.checkCache[channel]
				m.statusMu.RUnlock()
				if hasCached && cachedInfo != nil {
					m.updateStatus("idle", 0, "Using cached update info (GitHub rate limit)")
					return cachedInfo, nil
				}
			}

			info := &UpdateInfo{
				Available:      false,
				CurrentVersion: currentInfo.Version,
				LatestVersion:  currentInfo.Version,
				DownloadURL:    "",
				IsPrerelease:   currentVer.IsPrerelease(),
				Warning:        "Update check temporarily unavailable because GitHub rate limit was reached. Try again in a few minutes.",
			}
			m.updateStatus("idle", 0, "GitHub rate limit reached during update check")
			return info, nil
		}

		// Check if this is a "no releases found" error - handle gracefully
		if strings.Contains(err.Error(), "no releases found") {
			// No releases available for this channel - return "no update available"
			info := &UpdateInfo{
				Available:      false,
				CurrentVersion: currentInfo.Version,
				LatestVersion:  currentInfo.Version,
			}
			if useCache {
				m.statusMu.Lock()
				m.checkCache[channel] = info
				m.cacheTime[channel] = time.Now()
				m.statusMu.Unlock()
			}
			m.updateStatus("idle", 0, fmt.Sprintf("No releases available for %s channel", channel))
			return info, nil
		}
		// For other errors, return the error
		m.updateStatus("error", 0, "Failed to check for updates", err)
		return nil, err
	}

	latestVer, err := ParseVersion(release.TagName)
	if err != nil {
		parseErr := fmt.Errorf("failed to parse latest version: %w", err)
		m.updateStatus("error", 0, "Invalid latest version", parseErr)
		return nil, parseErr
	}

	// Find download URL for current architecture
	downloadURL := ""
	arch := runtime.GOARCH
	// Map Go architecture names to release asset names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"arm":   "armv7",
		"386":   "386",
	}

	targetArch, ok := archMap[arch]
	if !ok {
		targetArch = arch // Use as-is if not in map
	}

	// Look for architecture-specific binary
	targetName := fmt.Sprintf("pulse-%s-linux-%s.tar.gz", release.TagName, targetArch)
	for _, asset := range release.Assets {
		if asset.Name == targetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	// Fallback to any pulse tarball if exact match not found
	if downloadURL == "" {
		for _, asset := range release.Assets {
			if strings.HasPrefix(asset.Name, "pulse-") &&
				strings.Contains(asset.Name, "linux") &&
				strings.HasSuffix(asset.Name, ".tar.gz") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	info := &UpdateInfo{
		Available:      latestVer.IsNewerThan(currentVer),
		CurrentVersion: currentInfo.Version,
		LatestVersion:  strings.TrimPrefix(release.TagName, "v"),
		ReleaseNotes:   release.Body,
		ReleaseDate:    release.PublishedAt,
		DownloadURL:    downloadURL,
		IsPrerelease:   release.Prerelease,
	}

	// Cache the result (only if using saved channel)
	if useCache {
		m.statusMu.Lock()
		m.checkCache[channel] = info
		m.cacheTime[channel] = time.Now()
		m.statusMu.Unlock()
	}

	status := "idle"
	message := "No updates available"
	if info.Available {
		status = "available"
		message = fmt.Sprintf("Update available: %s", info.LatestVersion)
	}
	m.updateStatus(status, 100, message)

	return info, nil
}

// ApplyUpdate downloads and applies an update
func (m *Manager) ApplyUpdate(ctx context.Context, req ApplyUpdateRequest) error {
	// Validate download URL (allow test server URLs when PULSE_UPDATE_SERVER is set)
	if req.DownloadURL == "" {
		return fmt.Errorf("download URL is required")
	}
	if os.Getenv("PULSE_UPDATE_SERVER") == "" {
		if !strings.HasPrefix(req.DownloadURL, "https://github.com/rcourtman/Pulse/releases/download/") {
			return fmt.Errorf("invalid download URL")
		}
	}

	// Check if Docker
	currentInfo, _ := GetCurrentVersion()
	if currentInfo.IsDocker {
		return fmt.Errorf("updates cannot be applied in Docker environment")
	}

	// Check for pre-v4 installation
	if isPreV4Installation() {
		return fmt.Errorf("manual migration required: Pulse v4 is a complete rewrite. Please create a fresh installation. See https://github.com/rcourtman/Pulse/releases/v4.0.0")
	}

	// Ensure only one update runs at a time.
	m.updateMu.Lock()
	if m.updateInFlight {
		m.updateMu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	m.updateInFlight = true
	m.updateMu.Unlock()
	defer func() {
		m.updateMu.Lock()
		m.updateInFlight = false
		m.updateMu.Unlock()
	}()

	m.updateStatus("downloading", 10, "Downloading update...")

	channel := m.resolveChannel(req.Channel, currentInfo)
	targetVersion := inferVersionFromDownloadURL(req.DownloadURL)
	initiatedBy := req.InitiatedBy
	if initiatedBy == "" {
		initiatedBy = InitiatedByUser
	}
	initiatedVia := req.InitiatedVia
	if initiatedVia == "" {
		initiatedVia = InitiatedViaAPI
	}

	start := time.Now()
	eventID := m.createHistoryEntry(ctx, UpdateHistoryEntry{
		Action:         "update",
		Channel:        channel,
		VersionFrom:    currentInfo.Version,
		VersionTo:      targetVersion,
		DeploymentType: currentInfo.DeploymentType,
		InitiatedBy:    initiatedBy,
		InitiatedVia:   initiatedVia,
		Status:         StatusInProgress,
		Notes:          req.Notes,
	})

	var runErr error
	defer func() {
		if eventID == "" {
			return
		}
		status := StatusSuccess
		if runErr != nil {
			status = StatusFailed
		}
		m.completeHistoryEntry(ctx, eventID, status, start, runErr)
	}()

	// Create temp directory in a location we can write to
	// Try multiple locations in order of preference
	var tempDir string
	var err error

	// Try data directory first
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/etc/pulse"
	}

	// Try to create temp dir in data directory
	tempDir, err = os.MkdirTemp(dataDir, "pulse-update-*")
	if err != nil {
		// Fallback to /tmp
		tempDir, err = os.MkdirTemp("/tmp", "pulse-update-*")
		if err != nil {
			// Last resort: current directory
			tempDir, err = os.MkdirTemp(".", "pulse-update-*")
			if err != nil {
				tempErr := fmt.Errorf("failed to create temp directory in any location: %w", err)
				m.updateStatus("error", 10, "Failed to create temp directory", tempErr)
				runErr = tempErr
				return tempErr
			}
		}
	}
	defer os.RemoveAll(tempDir)

	// Download update
	tarballPath := filepath.Join(tempDir, "update.tar.gz")
	downloadBytes, err := m.downloadFile(ctx, req.DownloadURL, tarballPath)
	if err != nil {
		downloadErr := fmt.Errorf("failed to download update: %w", err)
		m.updateStatus("error", 20, "Failed to download update", downloadErr)
		runErr = downloadErr
		return runErr
	}
	if downloadBytes > 0 {
		m.updateHistoryEntry(ctx, eventID, func(entry *UpdateHistoryEntry) {
			entry.DownloadBytes = downloadBytes
		})
	}

	// Verify checksum if available
	m.updateStatus("verifying", 30, "Verifying download...")
	if err := m.verifyChecksum(ctx, req.DownloadURL, tarballPath); err != nil {
		checksumErr := fmt.Errorf("checksum verification failed: %w", err)
		m.updateStatus("error", 30, "Failed to verify update checksum", checksumErr)
		runErr = checksumErr
		return runErr
	}
	log.Info().Msg("Checksum verification passed")

	m.updateStatus("extracting", 40, "Extracting update...")

	// Extract tarball
	extractDir := filepath.Join(tempDir, "extracted")
	if err := m.extractTarball(tarballPath, extractDir); err != nil {
		extractErr := fmt.Errorf("failed to extract update: %w", err)
		m.updateStatus("error", 40, "Failed to extract update", extractErr)
		runErr = extractErr
		return runErr
	}

	m.updateStatus("backing-up", 60, "Creating backup...")

	// Create backup
	backupPath, err := m.createBackup()
	if err != nil {
		backupErr := fmt.Errorf("failed to create backup: %w", err)
		m.updateStatus("error", 60, "Failed to create backup", backupErr)
		runErr = backupErr
		return runErr
	}
	log.Info().Str("backup", backupPath).Msg("Created backup")
	m.updateHistoryEntry(ctx, eventID, func(entry *UpdateHistoryEntry) {
		entry.BackupPath = backupPath
	})

	m.updateStatus("applying", 80, "Applying update...")

	// Apply the update files
	// With the new directory structure (/opt/pulse/bin/), the pulse user has write access
	log.Info().Msg("Applying update files")

	if err := m.applyUpdateFiles(extractDir); err != nil {
		applyErr := fmt.Errorf("failed to apply update: %w", err)
		m.updateStatus("error", 80, "Failed to apply update", applyErr)
		runErr = applyErr
		// Attempt to restore backup
		if restoreErr := m.restoreBackup(backupPath); restoreErr != nil {
			log.Error().Err(restoreErr).Msg("Failed to restore backup")
		}
		return runErr
	}

	m.updateStatus("restarting", 95, "Restarting service...")

	// Schedule a clean exit after a short delay - systemd will restart us
	if !dockerUpdatesAllowed() {
		go func() {
			time.Sleep(2 * time.Second)
			log.Info().Msg("Exiting for restart after update")
			os.Exit(0)
		}()
	} else {
		log.Info().Msg("Skipping process exit after update (mock/CI mode)")
	}

	m.updateStatus("completed", 100, "Update completed, restarting...")
	return nil
}

// GetStatus returns the current update status
func (m *Manager) GetStatus() UpdateStatus {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.status
}

// GetCachedUpdateInfo returns the cached update info without making a network request
// Returns nil if no cached info is available
// Uses the configured or auto-detected channel
func (m *Manager) GetCachedUpdateInfo() *UpdateInfo {
	// Determine which channel to use (same logic as CheckForUpdates)
	channel := m.config.UpdateChannel
	if channel == "" {
		// Try to auto-detect from current version
		currentInfo, err := GetCurrentVersion()
		if err == nil && currentInfo.Channel != "" {
			channel = currentInfo.Channel
		}
	}
	if channel == "" {
		channel = "stable"
	}

	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.checkCache[channel]
}

// getLatestReleaseForChannel fetches the latest release from GitHub for a specific channel
func (m *Manager) getLatestReleaseForChannel(ctx context.Context, channel string, currentVer *Version) (*ReleaseInfo, error) {
	if channel == "" {
		channel = "stable"
	}

	log.Info().
		Str("channel", channel).
		Str("currentVersion", currentVer.String()).
		Bool("isPrerelease", currentVer.IsPrerelease()).
		Msg("Checking for updates")

	// GitHub API URL (can be overridden for testing)
	// Always fetch all releases so we can do version-aware filtering
	baseURL := os.Getenv("PULSE_UPDATE_SERVER")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	url := baseURL + "/repos/rcourtman/Pulse/releases"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pulse-Update-Checker")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Warn().
			Str("channel", channel).
			Str("rateLimitRemaining", resp.Header.Get("X-RateLimit-Remaining")).
			Str("rateLimitReset", resp.Header.Get("X-RateLimit-Reset")).
			Msg("GitHub API rate limit encountered, trying RSS fallback")

		// Try RSS/Atom feed as fallback - doesn't count against rate limits
		if feedRelease, err := m.getLatestReleaseFromFeed(ctx, channel); err == nil {
			log.Info().Str("version", feedRelease.TagName).Msg("Got release info from RSS feed fallback")
			return feedRelease, nil
		}

		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}

		return nil, fmt.Errorf("%w: %s", errGitHubRateLimited, detail)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, detail)
	}

	var releases []ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	// Find latest release based on channel
	// RC channel: return newest release (RC or stable), even if not newer than current
	// Stable channel: return newest stable release, even if not newer than current
	// The caller will determine if it's actually an update by comparing versions
	if channel == "rc" {
		// For RC channel: find newest release (RC or stable)
		// RC users should see both RCs and stable releases
		var newestRC *ReleaseInfo
		var newestStable *ReleaseInfo

		for i := range releases {
			// Skip draft releases
			if releases[i].Draft {
				log.Debug().Str("tag", releases[i].TagName).Msg("Skipping draft release")
				continue
			}

			releaseVer, err := ParseVersion(releases[i].TagName)
			if err != nil {
				log.Debug().Str("tag", releases[i].TagName).Err(err).Msg("Failed to parse release version")
				continue
			}

			if releases[i].Prerelease {
				// Track newest RC
				if newestRC == nil {
					newestRC = &releases[i]
				} else {
					newestRCVer, _ := ParseVersion(newestRC.TagName)
					if releaseVer.IsNewerThan(newestRCVer) {
						newestRC = &releases[i]
					}
				}
			} else {
				// Track newest stable
				if newestStable == nil {
					newestStable = &releases[i]
				} else {
					newestStableVer, _ := ParseVersion(newestStable.TagName)
					if releaseVer.IsNewerThan(newestStableVer) {
						newestStable = &releases[i]
					}
				}
			}
		}

		// Return the highest version among candidates
		// Stable versions are considered higher than RCs (4.22.0 > 4.22.0-rc.3)
		if newestStable != nil && newestRC != nil {
			stableVer, _ := ParseVersion(newestStable.TagName)
			rcVer, _ := ParseVersion(newestRC.TagName)
			if stableVer.IsNewerThan(rcVer) {
				isUpdate := stableVer.IsNewerThan(currentVer)
				if isUpdate {
					log.Info().Str("version", newestStable.TagName).Msg("Found stable update for RC user")
				} else {
					log.Info().Str("version", newestStable.TagName).Msg("On latest stable version")
				}
				return newestStable, nil
			}
			isUpdate := rcVer.IsNewerThan(currentVer)
			if isUpdate {
				log.Info().Str("version", newestRC.TagName).Msg("Found RC update")
			} else {
				log.Info().Str("version", newestRC.TagName).Msg("On latest RC version")
			}
			return newestRC, nil
		} else if newestStable != nil {
			isUpdate := newestStable.TagName != currentVer.String()
			if isUpdate {
				log.Info().Str("version", newestStable.TagName).Msg("Found stable update for RC user")
			} else {
				log.Info().Str("version", newestStable.TagName).Msg("On latest stable version")
			}
			return newestStable, nil
		} else if newestRC != nil {
			isUpdate := newestRC.TagName != currentVer.String()
			if isUpdate {
				log.Info().Str("version", newestRC.TagName).Msg("Found RC update")
			} else {
				log.Info().Str("version", newestRC.TagName).Msg("On latest RC version")
			}
			return newestRC, nil
		}
	} else {
		// For stable channel: find latest non-prerelease
		for i := range releases {
			// Skip draft releases
			if releases[i].Draft {
				log.Debug().Str("tag", releases[i].TagName).Msg("Skipping draft release")
				continue
			}

			if releases[i].Prerelease {
				continue
			}

			releaseVer, err := ParseVersion(releases[i].TagName)
			if err != nil {
				log.Debug().Str("tag", releases[i].TagName).Err(err).Msg("Failed to parse release version")
				continue
			}

			// Found the latest stable release
			isUpdate := releaseVer.IsNewerThan(currentVer)
			if isUpdate {
				log.Info().Str("version", releases[i].TagName).Msg("Found stable update")
			} else {
				log.Info().Str("version", releases[i].TagName).Msg("On latest stable version")
			}
			return &releases[i], nil
		}
	}

	// No releases found at all for this channel
	log.Warn().Str("channel", channel).Msg("No releases found for channel")
	return nil, fmt.Errorf("no releases found for channel %s", channel)
}

func (m *Manager) resolveChannel(requested string, currentInfo *VersionInfo) string {
	if requested != "" {
		return requested
	}
	if m.config != nil && m.config.UpdateChannel != "" {
		return m.config.UpdateChannel
	}
	if currentInfo != nil && currentInfo.Channel != "" {
		return currentInfo.Channel
	}
	return "stable"
}

// getLatestReleaseFromFeed fetches the latest release from GitHub's Atom feed
// This is used as a fallback when the API is rate-limited, as the Atom feed
// doesn't count against API rate limits.
func (m *Manager) getLatestReleaseFromFeed(ctx context.Context, channel string) (*ReleaseInfo, error) {
	feedURL := "https://github.com/rcourtman/Pulse/releases.atom"

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create feed request: %w", err)
	}

	req.Header.Set("User-Agent", "Pulse-Update-Checker")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read feed: %w", err)
	}

	// Parse the Atom feed to extract version tags
	// The feed format includes entries like: <title>Pulse v5.0.0</title>
	// We use simple string parsing rather than a full XML parser for minimal deps
	content := string(body)

	// Find all version tags in the feed (format: "Pulse vX.Y.Z" or "Pulse vX.Y.Z-rc.N")
	versionRegex := regexp.MustCompile(`<title>Pulse (v\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?)</title>`)
	matches := versionRegex.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no version tags found in feed")
	}

	// Filter based on channel
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tagName := match[1]

		// Parse the version to check if it's a prerelease
		ver, err := ParseVersion(tagName)
		if err != nil {
			continue
		}

		isPrerelease := ver.IsPrerelease()

		// For stable channel, skip prereleases
		if channel == "stable" && isPrerelease {
			continue
		}

		// Found a valid release for this channel
		log.Debug().
			Str("tag", tagName).
			Bool("prerelease", isPrerelease).
			Str("channel", channel).
			Msg("Found release from feed")

		return &ReleaseInfo{
			TagName:    tagName,
			Name:       "Pulse " + tagName,
			Prerelease: isPrerelease,
			// Note: Feed doesn't include full release notes or asset info
			// This is just for version checking - actual download still uses known URL patterns
		}, nil
	}

	return nil, fmt.Errorf("no suitable release found for channel %s", channel)
}

func (m *Manager) createHistoryEntry(ctx context.Context, entry UpdateHistoryEntry) string {
	if m.history == nil {
		return ""
	}
	eventID, err := m.history.CreateEntry(ctx, entry)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create update history entry")
		return ""
	}
	return eventID
}

func (m *Manager) updateHistoryEntry(ctx context.Context, eventID string, updateFn func(entry *UpdateHistoryEntry)) {
	if m.history == nil || eventID == "" {
		return
	}
	if err := m.history.UpdateEntry(ctx, eventID, func(e *UpdateHistoryEntry) error {
		updateFn(e)
		return nil
	}); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to update history entry")
	}
}

func (m *Manager) completeHistoryEntry(ctx context.Context, eventID string, status UpdateStatusType, start time.Time, runErr error) {
	if m.history == nil || eventID == "" {
		return
	}
	if err := m.history.UpdateEntry(ctx, eventID, func(e *UpdateHistoryEntry) error {
		e.Status = status
		e.DurationMs = time.Since(start).Milliseconds()
		if runErr != nil {
			e.Error = &UpdateError{
				Message: runErr.Error(),
				Code:    "update_failed",
			}
		} else {
			e.Error = nil
		}
		return nil
	}); err != nil {
		log.Error().Err(err).Str("event_id", eventID).Msg("Failed to finalize history entry")
	}
}

var versionInURLRegex = regexp.MustCompile(`v\d+\.\d+\.\d+(?:-[A-Za-z0-9\.]*\d[A-Za-z0-9\.]*)?`)

func inferVersionFromDownloadURL(downloadURL string) string {
	if downloadURL == "" {
		return ""
	}
	if match := versionInURLRegex.FindString(downloadURL); match != "" {
		return match
	}
	base := filepath.Base(downloadURL)
	if match := versionInURLRegex.FindString(base); match != "" {
		return match
	}
	return ""
}

// downloadFile downloads a file from URL to dest
func (m *Manager) downloadFile(ctx context.Context, url, dest string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	// Copy with progress updates
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return 0, err
	}

	log.Info().Int64("bytes", written).Str("file", dest).Msg("Downloaded file")
	return written, nil
}

// verifyChecksum downloads and verifies the SHA256 checksum of a file
func (m *Manager) verifyChecksum(ctx context.Context, tarballURL, tarballPath string) error {
	// Try to find checksum file URL by deriving from tarball URL
	// Example: pulse-v4.22.0-linux-amd64.tar.gz -> SHA256SUMS or checksums.txt
	baseURL := tarballURL[:strings.LastIndex(tarballURL, "/")+1]

	// Common checksum file names used in GitHub releases
	checksumNames := []string{"SHA256SUMS", "checksums.txt", "SHA256SUMS.txt"}

	var checksumContent string
	var checksumErr error

	// Try each checksum filename
	for _, name := range checksumNames {
		checksumURL := baseURL + name

		req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
		if err != nil {
			continue
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				checksumContent = string(body)
				log.Info().Str("file", name).Msg("Found checksum file")
				break
			}
		}
	}

	if checksumContent == "" {
		return fmt.Errorf("no checksum file found")
	}

	// Parse checksum file to find the hash for our tarball
	tarballName := filepath.Base(tarballURL)
	expectedHash := ""

	for _, line := range strings.Split(checksumContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "hash  filename" or "hash *filename"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			filename := parts[1]
			// Remove leading * if present (indicates binary mode)
			filename = strings.TrimPrefix(filename, "*")

			if filename == tarballName {
				expectedHash = strings.ToLower(hash)
				break
			}
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("checksum not found for %s in checksum file", tarballName)
	}

	// Compute SHA256 of downloaded file
	file, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open tarball for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))

	// Compare hashes
	if actualHash != expectedHash {
		checksumErr = fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		log.Error().
			Str("expected", expectedHash).
			Str("actual", actualHash).
			Msg("Checksum verification failed")
		return checksumErr
	}

	log.Info().
		Str("hash", actualHash).
		Msg("Checksum verified successfully")

	return nil
}

// extractTarball extracts a gzipped tarball
func (m *Manager) extractTarball(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize the path to prevent directory traversal attacks
		cleanName := filepath.Clean(header.Name)

		// Check for path traversal attempts
		if strings.Contains(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("unsafe path in archive: %s", header.Name)
		}

		// Ensure the target path is within the destination directory
		target := filepath.Join(dest, cleanName)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != filepath.Clean(dest) {
			return fmt.Errorf("path escapes destination directory: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

// createBackup creates a backup of the current installation
func (m *Manager) createBackup() (string, error) {
	timestamp := time.Now().Format("20060102-150405")

	// Try to create backup in a writable location
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/etc/pulse"
	}

	backupDir := filepath.Join(dataDir, fmt.Sprintf("backup-%s", timestamp))

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		// Fallback to /tmp if data dir fails
		backupDir = fmt.Sprintf("/tmp/pulse-backup-%s", timestamp)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

	// Backup important directories
	dirsToBackup := []string{"data", "config"}
	pulseDir := os.Getenv("PULSE_INSTALL_DIR")
	if pulseDir == "" {
		pulseDir = "/opt/pulse"
	}

	for _, dir := range dirsToBackup {
		src := filepath.Join(pulseDir, dir)
		dest := filepath.Join(backupDir, dir)

		if _, err := os.Stat(src); err == nil {
			if err := m.copyDirSafe(src, dest); err != nil {
				log.Warn().Str("dir", dir).Err(err).Msg("Failed to backup directory")
			}
		}
	}

	// Backup .env file
	envSrc := filepath.Join(pulseDir, ".env")
	if _, err := os.Stat(envSrc); err == nil {
		envDest := filepath.Join(backupDir, ".env")
		if err := m.copyFileSafe(envSrc, envDest); err != nil {
			log.Warn().Err(err).Msg("Failed to backup .env file")
		}
	}

	// Backup the pulse binary itself
	binaryPath, err := os.Executable()
	if err == nil {
		binaryDest := filepath.Join(backupDir, "pulse")
		if err := m.copyFileSafe(binaryPath, binaryDest); err != nil {
			log.Warn().Err(err).Msg("Failed to backup pulse binary")
		} else {
			log.Info().Str("binary", binaryPath).Msg("Backed up pulse binary")
		}
	}

	// Backup VERSION file if it exists
	versionSrc := filepath.Join(pulseDir, "VERSION")
	if _, err := os.Stat(versionSrc); err == nil {
		versionDest := filepath.Join(backupDir, "VERSION")
		if err := m.copyFileSafe(versionSrc, versionDest); err != nil {
			log.Warn().Err(err).Msg("Failed to backup VERSION file")
		}
	}

	return backupDir, nil
}

// restoreBackup restores from a backup
func (m *Manager) restoreBackup(backupDir string) error {
	pulseDir := "/opt/pulse"

	// Restore directories
	dirsToRestore := []string{"data", "config"}
	for _, dir := range dirsToRestore {
		src := filepath.Join(backupDir, dir)
		dest := filepath.Join(pulseDir, dir)

		if _, err := os.Stat(src); err == nil {
			// Remove existing directory first
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("failed to remove existing %s: %w", dir, err)
			}
			if err := m.copyDirSafe(src, dest); err != nil {
				return fmt.Errorf("failed to restore %s: %w", dir, err)
			}
		}
	}

	// Restore .env
	envSrc := filepath.Join(backupDir, ".env")
	if _, err := os.Stat(envSrc); err == nil {
		envDest := filepath.Join(pulseDir, ".env")
		if err := m.copyFileSafe(envSrc, envDest); err != nil {
			return fmt.Errorf("failed to restore .env: %w", err)
		}
	}

	// Restore the pulse binary if it exists in backup
	binarySrc := filepath.Join(backupDir, "pulse")
	if _, err := os.Stat(binarySrc); err == nil {
		binaryPath, err := os.Executable()
		if err == nil {
			// Create temp copy first, then atomic rename
			tempBinary := binaryPath + ".restored"
			if err := m.copyFileSafe(binarySrc, tempBinary); err != nil {
				return fmt.Errorf("failed to restore pulse binary: %w", err)
			}
			if err := os.Chmod(tempBinary, 0755); err != nil {
				return fmt.Errorf("failed to set binary permissions: %w", err)
			}
			if err := os.Rename(tempBinary, binaryPath); err != nil {
				return fmt.Errorf("failed to replace binary: %w", err)
			}
			log.Info().Str("binary", binaryPath).Msg("Restored pulse binary")
		}
	}

	// Restore VERSION file if it exists in backup
	versionSrc := filepath.Join(backupDir, "VERSION")
	if _, err := os.Stat(versionSrc); err == nil {
		versionDest := filepath.Join(pulseDir, "VERSION")
		if err := m.copyFileSafe(versionSrc, versionDest); err != nil {
			log.Warn().Err(err).Msg("Failed to restore VERSION file")
		}
	}

	return nil
}

// applyUpdateFiles copies update files to the installation directory
func (m *Manager) applyUpdateFiles(extractDir string) error {
	// Check for pulse binary in both old (root) and new (bin/) locations
	pulseBinary := filepath.Join(extractDir, "pulse")
	if _, err := os.Stat(pulseBinary); err != nil {
		// Try new structure with bin/ directory
		pulseBinary = filepath.Join(extractDir, "bin", "pulse")
		if _, err := os.Stat(pulseBinary); err != nil {
			return fmt.Errorf("pulse binary not found in extract (checked both / and /bin/): %w", err)
		}
	}

	// Detect where the current binary is running from
	binaryPath, err := os.Executable()
	if err != nil {
		// Fallback to default location
		binaryPath = "/usr/local/bin/pulse"
	}

	// Copy the pulse binary to a temporary location first, then move atomically
	tempBinary := binaryPath + ".new"
	cmd := exec.Command("cp", pulseBinary, tempBinary)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy pulse binary: %w", err)
	}

	// Make it executable
	if err := os.Chmod(tempBinary, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomically replace the old binary with the new one
	if err := os.Rename(tempBinary, binaryPath); err != nil {
		// If rename fails (cross-device), try mv command
		cmd = exec.Command("mv", "-f", tempBinary, binaryPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to replace pulse binary: %w", err)
		}
	}

	// Frontend is now embedded in the binary as of v4.2.2+
	// No need to copy frontend files separately
	// The new binary contains everything needed

	// Copy VERSION file if it exists (to both locations for compatibility)
	versionSrc := filepath.Join(extractDir, "VERSION")
	if _, err := os.Stat(versionSrc); err == nil {
		// Copy to /opt/pulse
		cmd = exec.Command("cp", versionSrc, "/opt/pulse/VERSION")
		if err := cmd.Run(); err != nil {
			log.Debug().Err(err).Msg("Failed to copy VERSION to /opt/pulse")
		}

		// Copy to binary directory
		binaryDir := filepath.Dir(binaryPath)
		cmd = exec.Command("cp", versionSrc, filepath.Join(binaryDir, "VERSION"))
		if err := cmd.Run(); err != nil {
			log.Warn().Err(err).Msg("Failed to copy VERSION file")
		}
	}

	// Deploy agent installation scripts from tarball
	scriptsDir := filepath.Join(extractDir, "scripts")
	if _, err := os.Stat(scriptsDir); err == nil {
		destScriptsDir := "/opt/pulse/scripts"
		if err := os.MkdirAll(destScriptsDir, 0755); err != nil {
			log.Warn().Err(err).Msg("Failed to create scripts directory")
		} else {
			// List of agent scripts to deploy
			agentScripts := []string{
				"install-docker-agent.sh",
				"install-container-agent.sh",
				"install-host-agent.ps1",
				"uninstall-host-agent.sh",
				"uninstall-host-agent.ps1",
				"install-docker.sh",
				"install.sh",
				"install.ps1",
			}

			deployed := 0
			for _, script := range agentScripts {
				srcPath := filepath.Join(scriptsDir, script)
				if _, err := os.Stat(srcPath); err == nil {
					destPath := filepath.Join(destScriptsDir, script)
					cmd = exec.Command("cp", srcPath, destPath)
					if err := cmd.Run(); err != nil {
						log.Warn().Err(err).Str("script", script).Msg("Failed to copy agent script")
						continue
					}
					if err := os.Chmod(destPath, 0755); err != nil {
						log.Warn().Err(err).Str("script", script).Msg("Failed to set script permissions")
					}
					deployed++
				}
			}
			if deployed > 0 {
				log.Info().Int("count", deployed).Msg("Deployed agent installation scripts")
			}
		}
	}

	// Deploy agent binaries from tarball (for serving to remote hosts)
	binDir := filepath.Join(extractDir, "bin")
	if _, err := os.Stat(binDir); err == nil {
		destBinDir := "/opt/pulse/bin"
		if err := os.MkdirAll(destBinDir, 0755); err != nil {
			log.Warn().Err(err).Msg("Failed to create bin directory")
		} else {
			// Copy agent binaries (pulse-agent-*, pulse-docker-agent-*, pulse-host-agent-*)
			entries, err := os.ReadDir(binDir)
			if err == nil {
				agentBinariesDeployed := 0
				for _, entry := range entries {
					name := entry.Name()
					// Skip the main pulse binary (already handled above) and directories
					if entry.IsDir() || name == "pulse" {
						continue
					}
					// Copy agent binaries
					if strings.HasPrefix(name, "pulse-agent-") ||
						strings.HasPrefix(name, "pulse-docker-agent") ||
						strings.HasPrefix(name, "pulse-host-agent") {
						srcPath := filepath.Join(binDir, name)
						destPath := filepath.Join(destBinDir, name)
						cmd = exec.Command("cp", "-a", srcPath, destPath)
						if err := cmd.Run(); err != nil {
							log.Warn().Err(err).Str("binary", name).Msg("Failed to copy agent binary")
							continue
						}
						// Set executable permission (skip for symlinks)
						if info, err := os.Lstat(destPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
							if err := os.Chmod(destPath, 0755); err != nil {
								log.Warn().Err(err).Str("binary", name).Msg("Failed to set binary permissions")
							}
						}
						agentBinariesDeployed++
					}
				}
				if agentBinariesDeployed > 0 {
					log.Info().Int("count", agentBinariesDeployed).Msg("Deployed agent binaries")
				}
			}
		}
	}

	// Set ownership if /opt/pulse exists
	if _, err := os.Stat("/opt/pulse"); err == nil {
		cmd = exec.Command("chown", "-R", "pulse:pulse", "/opt/pulse")
		if err := cmd.Run(); err != nil {
			log.Warn().Err(err).Msg("Failed to set ownership")
		}
	}

	return nil
}

// updateStatus updates the current status
func (m *Manager) updateStatus(status string, progress int, message string, err ...error) {
	m.statusMu.Lock()
	m.status = UpdateStatus{
		Status:    status,
		Progress:  progress,
		Message:   message,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	// If error provided, sanitize and add to status
	if len(err) > 0 && err[0] != nil {
		m.status.Error = sanitizeError(err[0])
	}
	statusCopy := m.status
	m.statusMu.Unlock()

	// Send to progress channel (non-blocking) for WebSocket compatibility
	select {
	case m.progressChan <- statusCopy:
	default:
	}

	// Broadcast to SSE clients
	if m.sseBroadcast != nil {
		m.sseBroadcast.Broadcast(statusCopy)
	}

	if delay := statusDelayForStage(status); delay > 0 {
		time.Sleep(delay)
	}
}

// sseHeartbeatLoop sends periodic heartbeats to SSE clients
func (m *Manager) sseHeartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if m.sseBroadcast != nil {
			m.sseBroadcast.SendHeartbeat()
		}
	}
}

// sanitizeError removes potentially sensitive information from error messages
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Cap length to prevent extremely long error messages
	maxLen := 500
	if len(errMsg) > maxLen {
		errMsg = errMsg[:maxLen] + "..."
	}

	return errMsg
}

func statusDelayForStage(status string) time.Duration {
	delay := configuredStageDelay()
	if delay == 0 {
		return 0
	}

	switch status {
	case "downloading", "verifying", "extracting", "backing-up", "applying":
		return delay
	default:
		return 0
	}
}

func configuredStageDelay() time.Duration {
	stageDelayOnce.Do(func() {
		value := strings.TrimSpace(os.Getenv("PULSE_UPDATE_STAGE_DELAY_MS"))
		if value == "" {
			return
		}
		ms, err := strconv.Atoi(value)
		if err != nil || ms <= 0 {
			log.Warn().Str("value", value).Msg("Invalid PULSE_UPDATE_STAGE_DELAY_MS, ignoring")
			return
		}
		stageDelayValue = time.Duration(ms) * time.Millisecond
	})

	return stageDelayValue
}

// cleanupOldTempDirs removes old pulse-update-* temp directories from previous runs
func (m *Manager) cleanupOldTempDirs() {
	// Check multiple locations where temp dirs might exist
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/etc/pulse"
	}

	dirsToCheck := []string{"/tmp", dataDir, "."}

	for _, dir := range dirsToCheck {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Directory not accessible, skip
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Check if it matches pulse-update-* pattern
			if !strings.HasPrefix(entry.Name(), "pulse-update-") {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			// Remove directories older than 24 hours
			if time.Since(info.ModTime()) > 24*time.Hour {
				if err := os.RemoveAll(fullPath); err != nil {
					log.Debug().Err(err).Str("path", fullPath).Msg("Failed to cleanup old temp directory")
				} else {
					log.Info().Str("path", fullPath).Msg("Cleaned up old temp directory")
				}
			}
		}
	}
}

// copyFileSafe safely copies a file, skipping symlinks for security
func (m *Manager) copyFileSafe(src, dest string) error {
	// Get file info and check if it's a symlink
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Skip symlinks for security
	if info.Mode()&os.ModeSymlink != 0 {
		log.Warn().Str("file", src).Msg("Skipping symlink during backup/restore")
		return nil
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file with same permissions
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return nil
}

// copyDirSafe recursively copies a directory, skipping symlinks for security
func (m *Manager) copyDirSafe(src, dest string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		// Get file info (using Lstat to detect symlinks)
		info, err := os.Lstat(srcPath)
		if err != nil {
			log.Warn().Str("path", srcPath).Err(err).Msg("Failed to stat file during copy")
			continue
		}

		// Skip symlinks for security
		if info.Mode()&os.ModeSymlink != 0 {
			log.Warn().Str("path", srcPath).Msg("Skipping symlink during backup/restore")
			continue
		}

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := m.copyDirSafe(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := m.copyFileSafe(srcPath, destPath); err != nil {
				log.Warn().Str("file", srcPath).Err(err).Msg("Failed to copy file")
				continue
			}
		}
	}

	return nil
}

// isPreV4Installation checks if this is a pre-v4 (Node.js based) installation
func isPreV4Installation() bool {
	// Check for .env file (used by Node.js version)
	if _, err := os.Stat("/opt/pulse/.env"); err == nil {
		return true
	}

	// Note: pulse-backend.service is used by both v4 and pre-v4, so we can't use it as an indicator
	// Only check for Node.js artifacts which are exclusive to pre-v4

	// Check for Node.js artifacts
	nodeArtifacts := []string{
		"/opt/pulse/package.json",
		"/opt/pulse/node_modules",
		"/opt/pulse/server.js",
		"/opt/pulse/backend",
		"/opt/pulse/frontend",
	}

	for _, artifact := range nodeArtifacts {
		if _, err := os.Stat(artifact); err == nil {
			return true
		}
	}

	return false
}
